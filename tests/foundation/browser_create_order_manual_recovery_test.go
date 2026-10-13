//go:build browser

// Purpose: own independent PG observations and a manual-only partial-Begin fault for recovery UI acceptance.
// Depends on: drawerPartialFault's bounded exact waiter protocol and the owned lbuEnv store/principal.
// Used by: runCreateOrderDrawerBrowser; fixture routes never substitute product HTTP responses.
// Invariants: I02/I03/I06/I11; only counts are persisted; original keys remain private runtime observations.
package foundation_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

type manualRecoveryControl struct {
	*drawerPartialFault
	keys []string
}

// newManualRecoveryControl reuses the real PG fault protocol with a separate owned trigger and advisory barrier.
func newManualRecoveryControl(t *testing.T, e *lbuEnv, unusedBundle string) *manualRecoveryControl {
	t.Helper()
	f := &drawerPartialFault{e: e, bundle: unusedBundle, barrier: "manual-partial|" + e.store(), function: "lcu3_manual_" + strings.ReplaceAll(e.store(), "-", "")}
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
	return &manualRecoveryControl{drawerPartialFault: f}
}

func (f *manualRecoveryControl) handleManual(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != "POST" || !strings.HasPrefix(r.URL.Path, "/__test/manual-recovery/") {
		return false
	}
	if r.URL.Path != "/__test/manual-recovery/request-facts" {
		// Adapt only the ephemeral fixture URL to reuse its bounded arm/kill/facts protocol.
		cloned := r.Clone(r.Context())
		cloned.URL.Path = strings.Replace(r.URL.Path, "/__test/manual-recovery/", "/__test/drawer-partial/", 1)
		return f.drawerPartialFault.handle(w, cloned)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	var input struct {
		Keys []string `json:"keys"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
	dec.DisallowUnknownFields()
	if dec.Decode(&input) != nil || len(input.Keys) < 1 || len(input.Keys) > 2 {
		http.Error(w, "invalid observation", 400)
		return true
	}
	for _, key := range input.Keys {
		if !regexp.MustCompile(`^[A-Za-z0-9_-]{8,128}$`).MatchString(key) {
			http.Error(w, "invalid observation", 400)
			return true
		}
	}
	var orders, receipts int
	e := f.e
	err := e.p.f.owner.QueryRow(r.Context(), `SELECT count(DISTINCT o.id),count(*) FROM ops.command_results c
 JOIN checkout.orders o ON o.id=(c.response->>'order_id')::uuid AND o.tenant_id=c.tenant_id AND o.store_id=c.store_id
 WHERE c.tenant_id=$1 AND c.store_id=$2 AND c.principal_id=$3 AND c.operation='merchanttools.order.manual' AND c.idempotency_key=ANY($4::text[])`, e.tenant(), e.store(), e.p.f.principalA, input.Keys).Scan(&orders, &receipts)
	if err != nil {
		http.Error(w, "manual observation failed", 500)
		return true
	}
	for _, key := range input.Keys {
		found := false
		for _, old := range f.keys {
			if old == key {
				found = true
			}
		}
		if !found {
			f.keys = append(f.keys, key)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]int{"orders": orders, "receipts": receipts})
	return true
}

// readback preserves failed-run evidence and independently checks both added manual orders and the exact partial receipts.
func (f *manualRecoveryControl) readbackManual(t *testing.T, evidence string, complete bool) []string {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var orders, receipts, seals int
	e := f.e
	err := e.p.f.owner.QueryRow(ctx, `SELECT count(DISTINCT o.id),count(*) FROM ops.command_results c
 JOIN checkout.orders o ON o.id=(c.response->>'order_id')::uuid AND o.tenant_id=c.tenant_id AND o.store_id=c.store_id
 WHERE c.tenant_id=$1 AND c.store_id=$2 AND c.principal_id=$3 AND c.operation='merchanttools.order.manual' AND c.idempotency_key=ANY($4::text[])`, e.tenant(), e.store(), e.p.f.principalA, f.keys).Scan(&orders, &receipts)
	if err != nil {
		t.Fatal("manual independent PG observation failed")
	}
	err = e.p.f.owner.QueryRow(ctx, `SELECT count(*) FROM ops.command_results WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND operation='merchanttools.order.attempt.recovery' AND idempotency_key=ANY($4::text[]) AND response='{"state":"not_created"}'::jsonb`, e.tenant(), e.store(), e.p.f.principalA, f.keys).Scan(&seals)
	if err != nil {
		t.Fatal("manual absence seal observation failed")
	}
	out := map[string]any{"orders": orders, "receipts": receipts, "not_created_seals": seals, "observed_keys": len(f.keys)}
	if f.key != "" {
		facts, err := f.facts(ctx, f.key)
		if err != nil {
			t.Fatal("manual partial independent PG observation failed")
		}
		out["partial"] = facts
		if complete {
			want := map[string]int{"begin": 1, "bindings": 1, "orders": 1, "manual_source": 1, "links": 1, "valid_links": 1, "manual_receipts": 1, "for_buyer_receipts": 0, "completion_receipts": 0, "recovery_receipts": 0, "closures": 0, "manual_audits": 1, "for_buyer_audits": 0, "pending_bundles": 0, "placed_bundles": 0, "operations": 0, "secrets": 0, "jobs": 0, "dm_valid_links": 0}
			for label, n := range want {
				if facts[label] != n {
					t.Fatalf("manual partial %s=%d expected=%d", label, facts[label], n)
				}
			}
		}
	}
	raw, _ := json.Marshal(out)
	if err = os.WriteFile(filepath.Join(evidence, "manual-recovery-pg-readback.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	if complete && (orders != 2 || receipts != 2 || seals != 1 || len(f.keys) != 3) {
		t.Fatalf("manual independent PG orders=%d receipts=%d seals=%d observedkeys=%d", orders, receipts, seals, len(f.keys))
	}
	return append([]string(nil), f.keys...)
}
