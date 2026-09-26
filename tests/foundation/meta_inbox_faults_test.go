package foundation_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func miStorageCounts(t *testing.T, m miTest) []int64 {
	t.Helper()
	var counts []int64
	for _, table := range []string{"meta_inbox.batches", "meta_inbox.events", "meta_inbox.batch_events", "meta_private.raw_bodies", "meta_private.event_bodies", "meta_private.quarantine_bodies", "river_meta.river_job"} {
		counts = append(counts, miCount(t, m.f.owner, `SELECT count(*) FROM `+table))
	}
	return counts
}

// Real writes reach every stage before a test-only trigger aborts them. This
// tests the complete multi-tenant unit of work, not separate single-row mocks.
func TestMetaInboxEveryPersistenceStageRollsBackMixedTenants(t *testing.T) {
	m := miSetup(t)
	ctx := context.Background()
	a, b, unknown := miAsset(), miAsset(), miAsset()
	var principalB string
	if err := m.f.owner.QueryRow(ctx, `SELECT principal_id::text FROM identity.memberships WHERE tenant_id=$1 AND active LIMIT 1`, m.f.tenantB).Scan(&principalB); err != nil {
		t.Fatal(err)
	}
	ba := miBinding(t, m, a, "facebook", m.f.tenantA, m.f.storeA1, m.f.principalA)
	bb := miBinding(t, m, b, "facebook", m.f.tenantB, m.f.storeB, principalB)
	miRoute(t, m, a, m.f.tenantA, m.f.storeA1, ba)
	miRoute(t, m, b, m.f.tenantB, m.f.storeB, bb)
	for _, stage := range []struct{ table, event string }{
		{"meta_inbox.batches", "INSERT"}, {"meta_inbox.events", "INSERT"}, {"meta_inbox.batch_events", "INSERT"},
		{"river_meta.river_job", "INSERT"}, {"meta_private.event_bodies", "INSERT"}, {"meta_private.quarantine_bodies", "INSERT"},
		{"meta_private.raw_bodies", "INSERT"}, {"meta_inbox.batches", "UPDATE OF finalized"},
	} {
		t.Run(stage.table+" "+stage.event, func(t *testing.T) {
			entries := make([]string, 0, 3)
			for _, asset := range []string{a, b, unknown} {
				entries = append(entries, fmt.Sprintf(`{"id":%q,"messaging":[{"sender":{"id":"4"},"recipient":{"id":%q},"message":{"mid":%q,"text":"synthetic private message"}}]}`, asset, asset, randomUUID()))
			}
			raw := []byte(`{"object":"page","entry":[` + strings.Join(entries, ",") + `]}`)
			name := pgx.Identifier{"mi_abort_" + strings.ReplaceAll(randomUUID(), "-", "")}.Sanitize()
			mustExec(t, m.f.owner, `CREATE FUNCTION public.`+name+`() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'synthetic persistence failure' USING ERRCODE='P0001'; END $$`)
			t.Cleanup(func() {
				mustExec(t, m.f.owner, `DROP TRIGGER IF EXISTS `+name+` ON `+stage.table)
				mustExec(t, m.f.owner, `DROP FUNCTION IF EXISTS public.`+name+`() `)
			})
			mustExec(t, m.f.owner, `CREATE TRIGGER `+name+` BEFORE `+stage.event+` ON `+stage.table+` FOR EACH ROW EXECUTE FUNCTION public.`+name+`() `)
			before := miStorageCounts(t, m)
			status, body := miPost(t, m, raw)
			miStatus(t, status, body, http.StatusServiceUnavailable)
			if body != "COMMIT_UNAVAILABLE" || !reflect.DeepEqual(before, miStorageCounts(t, m)) {
				t.Fatal("failed stage retained partial admission or exposed failure details")
			}
			mustExec(t, m.f.owner, `DROP TRIGGER `+name+` ON `+stage.table)
			mustExec(t, m.f.owner, `DROP FUNCTION public.`+name+`() `)
			status, body = miPost(t, m, raw)
			miStatus(t, status, body, http.StatusOK)
			var routed, scopes, quarantined int
			err := m.f.owner.QueryRow(ctx, `SELECT count(*) FILTER(WHERE e.disposition='ROUTED'),count(DISTINCT(e.tenant_id,e.store_id)) FILTER(WHERE e.disposition='ROUTED'),count(*) FILTER(WHERE e.disposition='QUARANTINED')
			 FROM meta_inbox.batches b JOIN meta_inbox.batch_events m ON m.batch_id=b.id JOIN meta_inbox.events e ON e.id=m.event_id WHERE b.body_hash=$1`, miBodyHash(raw)).Scan(&routed, &scopes, &quarantined)
			if err != nil || routed != 2 || scopes != 2 || quarantined != 1 {
				t.Fatalf("mixed tenant admission wrong: routed=%d scopes=%d quarantine=%d err=%v", routed, scopes, quarantined, err)
			}
		})
	}
}

type miLostResponse struct {
	headers http.Header
	status  int
}

func (w *miLostResponse) Header() http.Header    { return w.headers }
func (w *miLostResponse) WriteHeader(status int) { w.status = status }
func (w *miLostResponse) Write([]byte) (int, error) {
	return 0, errors.New("synthetic disconnected response")
}

func TestMetaInboxCommittedResponseLossReplaysHistory(t *testing.T) {
	m := miSetup(t)
	asset := miAsset()
	binding := miBinding(t, m, asset, "facebook", m.f.tenantA, m.f.storeA1, m.f.principalA)
	miRoute(t, m, asset, m.f.tenantA, m.f.storeA1, binding)
	raw := miMessage(asset, randomUUID(), "synthetic response loss")
	r := httptest.NewRequest(http.MethodPost, "/meta-webhook", bytes.NewReader(raw))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Hub-Signature-256", miSignature(raw))
	w := &miLostResponse{headers: make(http.Header)}
	m.handler.ServeHTTP(w, r)
	if w.status != http.StatusOK {
		t.Fatalf("admission before response loss failed: %d", w.status)
	}
	before := miStorageCounts(t, m)
	status, body := miPost(t, m, raw)
	miStatus(t, status, body, http.StatusOK)
	if !reflect.DeepEqual(before, miStorageCounts(t, m)) {
		t.Fatal("retry after lost committed response wrote another admission")
	}
}

// Finalize succeeds while the proof is valid; only the deferred COMMIT check
// can reject this later expiration. Opaque synthetic envelopes isolate the SQL
// fence; separate HTTP/AEAD tests prove real encryption and ACK behavior.
func TestMetaInboxProofExpiresAfterFinalizeBeforeCommit(t *testing.T) {
	m := miSetup(t)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	asset := miAsset()
	binding := miBinding(t, m, asset, "facebook", m.f.tenantA, m.f.storeA1, m.f.principalA)
	route, _ := miRoute(t, m, asset, m.f.tenantA, m.f.storeA1, binding)
	var expires time.Time
	if err := m.f.owner.QueryRow(ctx, `UPDATE meta_inbox.routes SET proof_expires=clock_timestamp()+interval '1 second' WHERE id=$1 RETURNING proof_expires`, route).Scan(&expires); err != nil {
		t.Fatal(err)
	}
	before := miStorageCounts(t, m)
	tx, err := m.ingress.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	var batchID, eventID, class string
	var replay, needsBody bool
	if err := tx.QueryRow(ctx, `SELECT batch_id::text,replay FROM meta_inbox.begin_batch($1,'page',$2,1)`, miApp, miBodyHash([]byte(randomUUID()))).Scan(&batchID, &replay); err != nil || replay {
		t.Fatal("new batch", err)
	}
	if err := tx.QueryRow(ctx, `SELECT event_id::text,needs_body,body_class FROM meta_inbox.prepare_event($1,1,$2,$3,$4,'page_message','',NULL)`, batchID, miBodyHash([]byte(randomUUID())), miBodyHash([]byte(randomUUID())), asset).Scan(&eventID, &needsBody, &class); err != nil || !needsBody || class != "event" {
		t.Fatal("initial routed admission", err)
	}
	var job int64
	if err := tx.QueryRow(ctx, `INSERT INTO river_meta.river_job(kind,queue,args,max_attempts) VALUES('meta_inbox_v1','meta_inbox',jsonb_build_object('event_id',$1::text,'version',1),25) RETURNING id`, eventID).Scan(&job); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `SELECT meta_inbox.complete_event($1,$2,$3,$4,$5,$6)`, batchID, eventID, miKeyID, randomBytes(12), randomBytes(17), job); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `SELECT meta_inbox.complete_batch($1,$2,$3,$4)`, batchID, miKeyID, randomBytes(12), randomBytes(17)); err != nil {
		t.Fatal("proof must remain valid through finalize", err)
	}
	if _, err := tx.Exec(ctx, `SELECT pg_sleep(greatest(extract(epoch FROM $1::timestamptz-clock_timestamp())+0.05,0))`, expires); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); miSQLState(err) != "22023" {
		t.Fatalf("expired final commit state=%s", miSQLState(err))
	}
	if !reflect.DeepEqual(before, miStorageCounts(t, m)) {
		t.Fatal("expired final commit persisted partial state")
	}
}

func TestMetaInboxInactiveScopesCannotRoute(t *testing.T) {
	m := miSetup(t)
	asset := miAsset()
	binding := miBinding(t, m, asset, "facebook", m.f.tenantA, m.f.storeA1, m.f.principalA)
	miRoute(t, m, asset, m.f.tenantA, m.f.storeA1, binding)
	for _, scope := range []struct{ table, id string }{{"control.tenants", m.f.tenantA}, {"control.stores", m.f.storeA1}} {
		t.Run(scope.table, func(t *testing.T) {
			mustExec(t, m.f.owner, `UPDATE `+scope.table+` SET active=false WHERE id=$1`, scope.id)
			defer mustExec(t, m.f.owner, `UPDATE `+scope.table+` SET active=true WHERE id=$1`, scope.id)
			raw := miMessage(asset, randomUUID(), "inactive scope")
			status, body := miPost(t, m, raw)
			miStatus(t, status, body, http.StatusOK)
			if miCount(t, m.f.owner, `SELECT count(*) FROM meta_inbox.batches b JOIN meta_inbox.batch_events m ON m.batch_id=b.id JOIN meta_inbox.events e ON e.id=m.event_id WHERE b.body_hash=$1 AND e.disposition='QUARANTINED' AND e.tenant_id IS NULL AND e.job_id IS NULL`, miBodyHash(raw)) != 1 {
				t.Fatal("inactive scope did not quarantine privately")
			}
		})
	}
}
