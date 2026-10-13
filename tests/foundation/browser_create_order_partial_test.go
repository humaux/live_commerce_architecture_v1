//go:build browser

// Purpose: own the PG fault/readback seam for the drawer's partial-Begin recovery case.
// Depends on: lbuEnv, signed Meta ingress and real ops/checkout/inbox/River tables.
// Used by: runCreateOrderDrawerBrowser; never adds production fault hooks or logs private request material.
// Invariants: I02/I03/I06/I11; terminate only this store/principal's exact scoped advisory waiter.
package foundation_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// drawerPartialConversation prepares a real signed inbound message for the separately owned signed-inbound bundle.
func drawerPartialConversation(t *testing.T, e *lbuEnv, claims *mciEnv, pageID string) string {
	t.Helper()
	raw := []byte(fmt.Sprintf(`{"object":"page","entry":[{"id":%q,"time":123,"messaging":[{"sender":{"id":%q},"recipient":{"id":%q},"timestamp":%d,"message":{"mid":%q,"text":"SYNTHETIC partial invoice"}}]}]}`, pageID, mciDigits(15), pageID, time.Now().UnixMilli(), "m."+randomUUID()))
	event := mcPost(t, claims.page, pageID, raw)
	mcAwait(t, claims.page, event)
	var conversation string
	if err := e.p.f.owner.QueryRow(context.Background(), `SELECT conversation_id::text FROM social.messages WHERE event_id=$1`, event.id).Scan(&conversation); err != nil {
		t.Fatal(err)
	}
	return conversation
}

type drawerPartialFault struct {
	e                              *lbuEnv
	bundle, barrier, function, key string
	mu                             sync.Mutex
	hold                           pgx.Tx
	openSend                       func(string, string, string, []byte, []byte) ([]byte, error)
	auditBase                      map[string]int
	recoveryCount                  int
	evidenceName                   string
	replyRevoked                   bool
}

// newDrawerPartialFault installs a store-scoped BEFORE receipt INSERT barrier, after checkout Begin committed.
func newDrawerPartialFault(t *testing.T, e *lbuEnv, bundle string, openSend func(string, string, string, []byte, []byte) ([]byte, error)) *drawerPartialFault {
	t.Helper()
	f := &drawerPartialFault{e: e, bundle: bundle, openSend: openSend, barrier: "drawer-partial|" + e.store(), function: "lcu3_partial_" + strings.ReplaceAll(e.store(), "-", "")}
	mustExec(t, e.p.f.owner, fmt.Sprintf(`CREATE FUNCTION ops.%s() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NEW.tenant_id='%s'::uuid AND NEW.store_id='%s'::uuid AND NEW.principal_id='%s'::uuid AND NEW.operation='merchanttools.order.manual' THEN
 PERFORM set_config('lock_timeout','0',true); PERFORM pg_advisory_xact_lock(hashtextextended('%s',0)); END IF; RETURN NEW; END $$`, f.function, e.tenant(), e.store(), e.p.f.principalA, f.barrier))
	mustExec(t, e.p.f.owner, fmt.Sprintf(`CREATE TRIGGER %s BEFORE INSERT ON ops.command_results FOR EACH ROW EXECUTE FUNCTION ops.%s()`, f.function, f.function))
	t.Cleanup(func() {
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.hold != nil {
			_ = f.hold.Rollback(context.Background())
			f.hold = nil
		}
		mustExec(t, e.p.f.owner, fmt.Sprintf(`DROP TRIGGER %s ON ops.command_results`, f.function))
		mustExec(t, e.p.f.owner, fmt.Sprintf(`DROP FUNCTION ops.%s()`, f.function))
	})
	return f
}

// handle exposes ephemeral fixture controls only; product A16/A16R still use the real handler and database.
func (f *drawerPartialFault) handle(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != "POST" || !strings.HasPrefix(r.URL.Path, "/__test/drawer-partial/") {
		return false
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Cache-Control", "no-store")
	var input struct {
		Key string `json:"key"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256))
	dec.DisallowUnknownFields()
	if dec.Decode(&input) != nil {
		http.Error(w, "invalid fixture control", 400)
		return true
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	var facts map[string]int
	var err error
	switch strings.TrimPrefix(r.URL.Path, "/__test/drawer-partial/") {
	case "arm":
		if f.hold != nil || f.auditBase != nil || input.Key != "" {
			http.Error(w, "fault already armed", 409)
			return true
		}
		f.auditBase, err = f.audits(ctx)
		if err == nil {
			f.hold, err = f.e.p.f.owner.Begin(ctx)
		}
		if err == nil {
			_, err = f.hold.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, f.barrier)
		}
		if err == nil {
			facts = map[string]int{"armed": 1}
		}
	case "kill":
		if f.hold == nil || !regexp.MustCompile(`^[A-Za-z0-9_-]{8,128}$`).MatchString(input.Key) {
			http.Error(w, "fault not armed", 409)
			return true
		}
		f.key = input.Key
		var pid int
		// Bounded exact waiter lookup is causal synchronization, not a timed guess or product retry.
		for ctx.Err() == nil {
			err = f.e.p.f.owner.QueryRow(ctx, `SELECT pid FROM pg_locks WHERE locktype='advisory' AND NOT granted AND objsubid=1 AND classid=((hashtextextended($1,0)>>32)&4294967295)::oid AND objid=(hashtextextended($1,0)&4294967295)::oid`, f.barrier).Scan(&pid)
			if err == nil {
				break
			}
			if err != pgx.ErrNoRows {
				break
			}
			select {
			case <-ctx.Done():
			case <-time.After(10 * time.Millisecond):
			}
		}
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		if err == nil {
			facts, err = f.facts(ctx, input.Key)
		}
		// Before killing, independently prove the order and Begin exist but all upper money/link writes are uncommitted.
		if err == nil && (facts["begin"] != 1 || facts["orders"] != 1 || facts["manual_source"] != 0 || facts["links"] != 0 || facts["manual_receipts"] != 0 || facts["for_buyer_receipts"] != 0) {
			err = fmt.Errorf("fault missed partial checkpoint")
		}
		var killed bool
		if err == nil {
			err = f.e.p.f.owner.QueryRow(ctx, `SELECT pg_terminate_backend(pid) FROM pg_locks WHERE pid=$2 AND locktype='advisory' AND NOT granted AND objsubid=1 AND classid=((hashtextextended($1,0)>>32)&4294967295)::oid AND objid=(hashtextextended($1,0)&4294967295)::oid`, f.barrier, pid).Scan(&killed)
		}
		if err == nil && !killed {
			err = fmt.Errorf("waiter termination refused")
		}
		if err == nil {
			err = f.hold.Rollback(ctx)
			f.hold = nil
		}
		if err == nil {
			w.Header().Set("Content-Type", "application/json")
			out := map[string]any{"killed": true}
			for k, v := range facts {
				out[k] = v
			}
			_ = json.NewEncoder(w).Encode(out)
			return true
		}
	case "facts":
		if input.Key == "" || input.Key != f.key {
			http.Error(w, "invalid observation", 400)
			return true
		}
		facts, err = f.facts(ctx, input.Key)
	default:
		http.NotFound(w, r)
		return true
	}
	if err != nil {
		http.Error(w, "partial fixture failed", 500)
		return true
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(facts)
	return true
}

func (f *drawerPartialFault) audits(ctx context.Context) (map[string]int, error) {
	out := map[string]int{}
	for label, action := range map[string]string{"manual_audits": "order.manual_created", "for_buyer_audits": "order.for_buyer_created"} {
		var count int
		if err := f.e.p.f.owner.QueryRow(ctx, `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND action=$4`, f.e.tenant(), f.e.store(), f.e.p.f.principalA, action).Scan(&count); err != nil {
			return nil, err
		}
		out[label] = count
	}
	return out, nil
}

// facts binds counts to the exact original merchant key and its independently derived checkout Begin step key.
func (f *drawerPartialFault) facts(ctx context.Context, key string) (map[string]int, error) {
	e := f.e
	sum := sha256.Sum256([]byte("mt|begin|" + key))
	beginKey := "mt-" + hex.EncodeToString(sum[:16])
	out := map[string]int{}
	var order string
	err := e.p.f.owner.QueryRow(ctx, `SELECT response->>'order_id' FROM checkout.command_results WHERE tenant_id=$1 AND store_id=$2 AND operation='checkout.begin' AND idempotency_key=$3`, e.tenant(), e.store(), beginKey).Scan(&order)
	if err != nil {
		return nil, err
	}
	var begin, orders, source, links, validLinks int
	if err = e.p.f.owner.QueryRow(ctx, `SELECT (SELECT count(*) FROM checkout.command_results WHERE tenant_id=$1 AND store_id=$2 AND operation='checkout.begin' AND idempotency_key=$3),count(*),count(*) FILTER(WHERE source='merchant_manual'),(SELECT count(*) FROM checkout.order_links WHERE order_id=$4),(SELECT count(*) FROM checkout.order_links WHERE order_id=$4 AND redeemed_at IS NULL AND expires_at>clock_timestamp()) FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2 AND id=$4`, e.tenant(), e.store(), beginKey, order).Scan(&begin, &orders, &source, &links, &validLinks); err != nil {
		return nil, err
	}
	out["begin"], out["orders"], out["manual_source"], out["links"], out["valid_links"] = begin, orders, source, links, validLinks
	for label, operation := range map[string]string{"bindings": "merchanttools.order.attempt", "manual_receipts": "merchanttools.order.manual", "for_buyer_receipts": "merchanttools.order.for_buyer", "completion_receipts": "merchanttools.order.for_buyer.completed", "recovery_receipts": "merchanttools.order.attempt.recovery", "closures": "merchanttools.order.checkout.closed"} {
		receiptKey := key
		if label == "closures" {
			receiptKey = beginKey
		}
		var count int
		if err = e.p.f.owner.QueryRow(ctx, `SELECT count(*) FROM ops.command_results WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND operation=$4 AND idempotency_key=$5`, e.tenant(), e.store(), e.p.f.principalA, operation, receiptKey).Scan(&count); err != nil {
			return nil, err
		}
		out[label] = count
	}
	for label, state := range map[string]string{"pending_bundles": "pending", "placed_bundles": "placed"} {
		var count int
		if err = e.p.f.owner.QueryRow(ctx, `SELECT count(*) FROM inbox.order_for_buyer WHERE tenant_id=$1 AND store_id=$2 AND bundle_id=$3 AND state=$4 AND (order_id IS NULL OR order_id=$5)`, e.tenant(), e.store(), f.bundle, state, order).Scan(&count); err != nil {
			return nil, err
		}
		out[label] = count
	}
	var operations, secrets, jobs int
	if err = e.p.f.owner.QueryRow(ctx, `SELECT count(*),(SELECT count(*) FROM inbox.send_secrets s JOIN integration.operations op ON op.id=s.operation_id WHERE op.tenant_id=$1 AND op.store_id=$2 AND op.action='meta.dm_send' AND op.request->>'order_id'=$3),count(*) FILTER(WHERE EXISTS(SELECT 1 FROM river.river_job j WHERE j.id=o.job_id AND j.state='available' AND j.args->>'operation_id'=o.id::text)) FROM integration.operations o WHERE o.tenant_id=$1 AND o.store_id=$2 AND o.action='meta.dm_send' AND o.request->>'order_id'=$3`, e.tenant(), e.store(), order).Scan(&operations, &secrets, &jobs); err != nil {
		return nil, err
	}
	out["operations"], out["secrets"], out["jobs"] = operations, secrets, jobs
	out["dm_valid_links"] = 0
	if operations == 1 && secrets == 1 {
		var operation string
		var enc, sealed []byte
		if err = e.p.f.owner.QueryRow(ctx, `SELECT o.id::text,s.enc,s.sealed FROM integration.operations o JOIN inbox.send_secrets s ON s.operation_id=o.id WHERE o.tenant_id=$1 AND o.store_id=$2 AND o.action='meta.dm_send' AND o.request->>'order_id'=$3`, e.tenant(), e.store(), order).Scan(&operation, &enc, &sealed); err != nil {
			return nil, err
		}
		plain, inner := f.openSend(e.tenant(), e.store(), operation, enc, sealed)
		if inner != nil {
			return nil, inner
		}
		var message struct {
			Text string `json:"text"`
		}
		inner = json.Unmarshal(plain, &message)
		clear(plain)
		if inner != nil {
			return nil, inner
		}
		token := regexp.MustCompile(`/en/order-link#o=` + regexp.QuoteMeta(order) + `&t=([A-Za-z0-9_-]+)`).FindStringSubmatch(message.Text)
		message.Text = ""
		if len(token) == 2 {
			hash := sha256.Sum256([]byte(token[1]))
			var valid int
			if err = e.p.f.owner.QueryRow(ctx, `SELECT count(*) FROM checkout.order_links WHERE order_id=$1 AND token_hash=$2 AND redeemed_at IS NULL AND expires_at>clock_timestamp()`, order, hash[:]).Scan(&valid); err != nil {
				return nil, err
			}
			out["dm_valid_links"] = valid
		}
	}
	// Each audit commits with its own receipt; xmin binds it to this attempt even after another fixture places an order.
	for label, operation := range map[string]string{"manual_audits": "merchanttools.order.manual", "for_buyer_audits": "merchanttools.order.for_buyer"} {
		action := "order.manual_created"
		if label == "for_buyer_audits" {
			action = "order.for_buyer_created"
		}
		var count int
		if err = e.p.f.owner.QueryRow(ctx, `SELECT count(*) FROM ops.audit_events a JOIN ops.command_results c ON c.tenant_id=a.tenant_id AND c.store_id=a.store_id AND c.principal_id=a.principal_id AND c.xmin=a.xmin WHERE c.tenant_id=$1 AND c.store_id=$2 AND c.principal_id=$3 AND c.operation=$4 AND c.idempotency_key=$5 AND a.action=$6`, e.tenant(), e.store(), e.p.f.principalA, operation, key, action).Scan(&count); err != nil {
			return nil, err
		}
		out[label] = count
	}
	return out, nil
}

// readback independently verifies every final PG invariant, even though Node also observes these counts.
func (f *drawerPartialFault) readback(t *testing.T, evidence string, complete bool) {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.key == "" {
		return
	} // Earlier existing-case failure never started this fixture; it is NOT_RUN.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	facts, err := f.facts(ctx, f.key)
	if err != nil {
		t.Fatal("partial independent PG observation failed")
	}
	raw, _ := json.Marshal(facts)
	if err = os.WriteFile(filepath.Join(evidence, f.readbackName()), raw, 0600); err != nil {
		t.Fatal(err)
	}
	if !complete {
		return
	}
	// Place's link is superseded once by atomic DM issuance; preserve exact history and one redeemable sealed URL.
	want := map[string]int{"begin": 1, "bindings": 1, "orders": 1, "manual_source": 1, "links": 2, "valid_links": 1, "manual_receipts": 1, "for_buyer_receipts": 1, "completion_receipts": 1, "recovery_receipts": f.recoveryCount, "closures": 0, "manual_audits": 1, "for_buyer_audits": 1, "pending_bundles": 0, "placed_bundles": 1, "operations": 1, "secrets": 1, "jobs": 1, "dm_valid_links": 1}
	for label, count := range want {
		if facts[label] != count {
			t.Fatalf("partial independent PG %s=%d expected=%d; evidence=%s", label, facts[label], count, evidence)
		}
	}
}

// readbackName keeps the two independent case receipts separate, including failing runs.
func (f *drawerPartialFault) readbackName() string {
	if f.evidenceName != "" {
		return f.evidenceName
	}
	return "partial-pg-readback.json"
}
