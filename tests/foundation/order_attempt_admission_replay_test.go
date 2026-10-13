// Purpose: real-runtime C1 all-state replay and A1.1 retired-scope non-disclosure.
// Depends on: native admission helpers, River InsertTx, lbu fixtures and pgx.
// Used by: OA1 CI red/green; synthetic sealed bytes test storage, not crypto or C2.
// Invariants: I01/I02/I04/I14; exact replay precedes current placement gates/CAS.
package foundation_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

func oaReplayAdmit(t *testing.T, e *lbuEnv, selection map[string]any) (oaScopeReceipt, string, oaAdmission) {
	t.Helper()
	ctx := context.Background()
	scope := oaAdmissionScope(t, e)
	key := "oa-replay-" + randomUUID()
	tx, err := e.p.f.runtime.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	prepared, err := oaPrepare(tx, e, scope, key, selection)
	if err != nil || prepared.AttemptID == "" || prepared.Replay {
		t.Fatalf("native C1 positive sentinel: %+v err=%v", prepared, err)
	}
	client, err := river.NewClient(riverpgxv5.New(e.p.f.runtime), &river.Config{Schema: "river"})
	if err != nil {
		t.Fatal(err)
	}
	args := json.RawMessage(fmt.Sprintf(`{"attempt_id":%q,"intent_generation":%d}`, prepared.AttemptID, prepared.Generation))
	if _, err := client.InsertTx(ctx, tx, oaRawJob{"order_attempt_v1", args}, &river.InsertOpts{Queue: "claims", Priority: 1}); err != nil {
		t.Fatal(err)
	}
	var finished []byte
	err = tx.QueryRow(ctx, `SELECT ops.finish_order_attempt_admission($1,$2,$3,$4,'synthetic_v1',decode(repeat('01',40),'hex'))`,
		tokenHash(e.token()), prepared.AttemptID, prepared.Generation, tokenHash("synthetic plan")).Scan(&finished)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("native sealed admission and real River job must commit: %v", err)
	}
	return scope, key, prepared
}

func oaReplayRequest(t *testing.T, e *lbuEnv, token, store string, scope oaScopeReceipt, key string, selection map[string]any) (oaAdmission, error) {
	t.Helper()
	raw, err := json.Marshal(selection)
	if err != nil {
		t.Fatal(err)
	}
	var result []byte
	err = e.p.f.runtime.QueryRow(context.Background(), `SELECT ops.prepare_order_attempt_admission($1,$2,$3,$4,$5,$6,$7::jsonb)`,
		tokenHash(token), store, scope.ScopeID, scope.Generation, scope.AttemptID, tokenHash(key), string(raw)).Scan(&result)
	var out oaAdmission
	if err == nil {
		err = json.Unmarshal(result, &out)
	}
	return out, err
}

func oaReplayConflict(t *testing.T, err error, message string) {
	t.Helper()
	var pgerr *pgconn.PgError
	if !errors.As(err, &pgerr) || pgerr.Code != "PT409" || pgerr.Message != message || pgerr.Detail != "" {
		t.Fatalf("non-disclosing %s required, got %v", message, err)
	}
}

func TestOrderAttemptAdmissionReplayPendingAndRetiredTerminal(t *testing.T) {
	e := lbuNew(t)
	selection := oaFactsSelection(e)
	scope, key, prepared := oaReplayAdmit(t, e, selection)
	// Revocation affects new placement but must not erase a durable admission.
	mustExec(t, e.p.f.owner, `DELETE FROM identity.store_grants WHERE store_id=$1 AND principal_id=$2 AND permission='catalog:read'`, e.store(), e.p.f.principalA)
	pending, err := oaReplayRequest(t, e, e.token(), e.store(), scope, key, selection)
	if err != nil || !pending.Replay || pending.AttemptID != prepared.AttemptID || pending.BuyerOwnerID != prepared.BuyerOwnerID || pending.State != "PENDING" {
		t.Fatalf("pending replay revalidated placement or changed owner: %+v err=%v", pending, err)
	}
	// Native owner fixture seals zero execution effects as ABORTED; no fake C2 order.
	mustExec(t, e.p.f.owner, `UPDATE ops.order_attempts SET state='ABORTED',plan_ciphertext=NULL,plan_key_version=NULL,
	 settled_at=clock_timestamp(),receipt=jsonb_build_object('state','ABORTED','attempt_id',attempt_id,'intent_generation',intent_generation,'reason','EXPIRED')
	 WHERE attempt_id=$1`, prepared.AttemptID)
	mustExec(t, e.p.f.owner, `UPDATE ops.order_attempt_scopes SET retired_at=clock_timestamp() WHERE scope_id=$1`, scope.ScopeID)
	terminal, err := oaReplayRequest(t, e, e.token(), e.store(), scope, key, selection)
	if err != nil || !terminal.Replay || terminal.AttemptID != prepared.AttemptID || terminal.ScopeID != scope.ScopeID || terminal.Generation != prepared.Generation || terminal.State != "ABORTED" {
		t.Fatalf("A1.1 retired exact replay failed: %+v err=%v", terminal, err)
	}
	changed := oaFactsSelection(e)
	_, err = oaReplayRequest(t, e, e.token(), e.store(), scope, key, changed)
	oaReplayConflict(t, err, "idempotency_conflict")
	e.grantCreator("catalog:read")
	_, err = oaReplayRequest(t, e, e.token(), e.store(), scope, "new-"+randomUUID(), selection)
	oaReplayConflict(t, err, "intent_conflict")
	var attempts, jobs int
	err = e.p.f.owner.QueryRow(context.Background(), `SELECT
	 (SELECT count(*) FROM ops.order_attempts WHERE scope_id=$1),
	 (SELECT count(*) FROM river.river_job WHERE kind='order_attempt_v1' AND args->>'attempt_id'=$2)`, scope.ScopeID, prepared.AttemptID).Scan(&attempts, &jobs)
	if err != nil || attempts != 1 || jobs != 1 {
		t.Fatalf("replay/new-key conflict created effects: attempts=%d jobs=%d err=%v", attempts, jobs, err)
	}
}

func TestOrderAttemptAdmissionReplayKeyOwnershipAndOriginalCAS(t *testing.T) {
	e := lbuNew(t)
	selection := oaFactsSelection(e)
	scope, key, _ := oaReplayAdmit(t, e, selection)
	_, otherToken := lcPrincipal(t, e.p.f, e.tenant(), []string{e.store()}, "store:read", "orders:read")
	_, err := oaReplayRequest(t, e, otherToken, e.store(), scope, key, selection)
	oaReplayConflict(t, err, "idempotency_conflict")
	otherStore := randomUUID()
	mustExec(t, e.p.f.owner, `INSERT INTO control.stores(tenant_id,id,name,currency) VALUES($1,$2,'OA replay second store','USD')`, e.tenant(), otherStore)
	for _, permission := range []string{"store:read", "orders:read"} {
		mustExec(t, e.p.f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, e.tenant(), otherStore, e.p.f.principalA, permission)
	}
	_, err = oaReplayRequest(t, e, e.token(), otherStore, scope, key, selection)
	oaReplayConflict(t, err, "idempotency_conflict")
	for _, mode := range []string{"scope", "generation", "previous"} {
		wrong := scope
		switch mode {
		case "scope":
			wrong.ScopeID = randomUUID()
		case "generation":
			wrong.Generation++
		case "previous":
			previous := randomUUID()
			wrong.AttemptID = &previous
		}
		_, err = oaReplayRequest(t, e, e.token(), e.store(), wrong, key, selection)
		oaReplayConflict(t, err, "idempotency_conflict")
	}
}
