// Purpose: independent REAL_PG storage-state acceptance for order-attempt-v1 §§2–3.
// Depends: fixture/newFixture, native ops tables/triggers and pgx savepoints.
// Used: OA1 foundation CI; no runtime admission or complete C2 behavior is simulated.
// Invariants: immutable all-state identities, monotone retired scopes and atomic
// removal of every pending subject reference. Superuser creates synthetic storage
// rows only; all native constraints/triggers remain enabled except an explicit,
// transactionally rolled-back fault injection. Runtime ACLs have separate tests.
package foundation_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type oaStateFixture struct {
	tx                    pgx.Tx
	f                     *testFixture
	scope, attempt, owner string
}

func oaStateExec(t *testing.T, tx pgx.Tx, query string, args ...any) {
	t.Helper()
	if _, err := tx.Exec(context.Background(), query, args...); err != nil {
		t.Fatalf("OA1 storage fixture/action: %v", err)
	}
}

func oaStateBegin(t *testing.T) oaStateFixture {
	t.Helper()
	f := fixture(t)
	tx, err := f.owner.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	h := oaStateFixture{tx: tx, f: f, scope: randomUUID(), attempt: randomUUID(), owner: randomUUID()}
	oaStateExec(t, tx, `SELECT set_config('app.tenant_id',$1,true),set_config('app.store_id',$2,true)`, f.tenantA, f.storeA1)
	oaStateExec(t, tx, `INSERT INTO buyer.owners(id,tenant_id,store_id) VALUES($1,$2,$3)`, h.owner, f.tenantA, f.storeA1)
	oaStateExec(t, tx, `INSERT INTO ops.order_attempt_creators(attempt_id,tenant_id,store_id,buyer_owner_id) VALUES($1,$2,$3,$4)`, h.attempt, f.tenantA, f.storeA1, h.owner)
	oaStateExec(t, tx, `INSERT INTO ops.order_attempt_scopes(scope_id,tenant_id,store_id,principal_id,current_generation)
	 VALUES($1,$2,$3,$4,1)`, h.scope, f.tenantA, f.storeA1, f.principalA)
	oaStateExec(t, tx, `INSERT INTO ops.order_attempts(attempt_id,tenant_id,store_id,scope_id,intent_generation,
	 transport_key_digest,request_hash,request_schema_version,buyer_owner_id,admission_provenance,
	 admitted_at,execute_by,state,enqueue_xid)
	 VALUES($1,$2,$3,$4,1,sha256($1::uuid::text::bytea),sha256('synthetic request'::bytea),1,
	 $5,'{}',statement_timestamp(),statement_timestamp()+interval '15 minutes','PENDING',pg_current_xact_id())`,
		h.attempt, f.tenantA, f.storeA1, h.scope, h.owner)
	// Native storage setup now includes the mandatory fresh River INSERT witness.
	// This remains a storage fixture, not proof of authenticated C1 admission.
	oaJobInsert(t, tx, h.attempt)
	oaJobFinish(t, tx, h.attempt)
	for _, kind := range []string{"customer", "buyer_owner", "private_owner"} {
		oaStateExec(t, tx, `INSERT INTO ops.order_attempt_subjects(tenant_id,store_id,subject_kind,subject_id,attempt_id)
		 VALUES($1,$2,$3,$4,$5)`, f.tenantA, f.storeA1, kind, randomUUID(), h.attempt)
	}
	// Positive sentinel validates native FKs/deferred admission before any negative.
	oaStateExec(t, tx, `SET CONSTRAINTS ALL IMMEDIATE`)
	var state string
	var subjects int
	err = tx.QueryRow(context.Background(), `SELECT state,(SELECT count(*) FROM ops.order_attempt_subjects WHERE attempt_id=$1)
	 FROM ops.order_attempts WHERE attempt_id=$1`, h.attempt).Scan(&state, &subjects)
	if err != nil || state != "PENDING" || subjects != 3 {
		t.Fatalf("OA1 native pending/three-subject sentinel: state=%s subjects=%d err=%v", state, subjects, err)
	}
	return h
}

func oaStateReject(t *testing.T, tx pgx.Tx, code, query string, args ...any) {
	t.Helper()
	probe, err := tx.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	_, err = probe.Exec(context.Background(), query, args...)
	rollbackErr := probe.Rollback(context.Background())
	var pgerr *pgconn.PgError
	if !errors.As(err, &pgerr) || pgerr.Code != code || rollbackErr != nil {
		t.Fatalf("OA1 expected SQLSTATE %s: err=%v rollback=%v", code, err, rollbackErr)
	}
}

func oaStateSettle(t *testing.T, tx pgx.Tx, attempt, state string) {
	t.Helper()
	// This opaque COMMITTED pointer is only a storage-row shape fixture. It is
	// expressly not evidence that quote/order/hold/domain C2 effects were created.
	var order any
	if state == "COMMITTED" {
		order = randomUUID()
	}
	oaStateExec(t, tx, `UPDATE ops.order_attempts SET state=$2,order_id=$3::uuid,
	 receipt=jsonb_build_object('state',$2::text,'attempt_id',attempt_id::text,'intent_generation',intent_generation)
	  || CASE WHEN $3::uuid IS NULL THEN '{}'::jsonb ELSE jsonb_build_object('order_id',$3::uuid::text) END,
	 plan_ciphertext=NULL,plan_key_version=NULL,settled_at=clock_timestamp() WHERE attempt_id=$1`, attempt, state, order)
}

func oaStateSettlementOracle(tx pgx.Tx, attempt, wantState string) error {
	var state string
	var subjects int
	var shredded bool
	err := tx.QueryRow(context.Background(), `SELECT state,plan_ciphertext IS NULL AND plan_key_version IS NULL,
	 (SELECT count(*) FROM ops.order_attempt_subjects WHERE attempt_id=$1)
	 FROM ops.order_attempts WHERE attempt_id=$1`, attempt).Scan(&state, &shredded, &subjects)
	if err != nil {
		return err
	}
	if state != wantState || !shredded || subjects != 0 {
		return fmt.Errorf("state=%s want=%s shredded=%v subjects=%d", state, wantState, shredded, subjects)
	}
	return nil
}

func TestOrderAttemptAdmissionStateAllStateUniqueness(t *testing.T) {
	for _, state := range []string{"ABORTED", "COMMITTED"} {
		t.Run(state, func(t *testing.T) {
			h := oaStateBegin(t)
			oaStateSettle(t, h.tx, h.attempt, state)
			if err := oaStateSettlementOracle(h.tx, h.attempt, state); err != nil {
				t.Fatalf("OA1 terminal uniqueness precondition: %v", err)
			}
			for _, reuse := range []string{"key_with_changed_body", "generation_with_fresh_key"} {
				t.Run(reuse, func(t *testing.T) {
					probe, err := h.tx.Begin(context.Background())
					if err != nil {
						t.Fatal(err)
					}
					defer probe.Rollback(context.Background())
					id := randomUUID()
					oaStateExec(t, probe, `INSERT INTO ops.order_attempt_creators(attempt_id,tenant_id,store_id,buyer_owner_id)
					 SELECT $2,tenant_id,store_id,buyer_owner_id FROM ops.order_attempts WHERE attempt_id=$1`, h.attempt, id)
					generation := 1
					reuseKey := false
					if reuse == "key_with_changed_body" {
						generation, reuseKey = 2, true
						oaStateExec(t, probe, `UPDATE ops.order_attempt_scopes SET current_generation=2 WHERE scope_id=$1`, h.scope)
					}
					oaStateReject(t, probe, "23505", `INSERT INTO ops.order_attempts(attempt_id,tenant_id,store_id,scope_id,intent_generation,
					 transport_key_digest,request_hash,request_schema_version,admission_finished,facts_digest,plan_digest,
					 plan_ciphertext,plan_key_version,buyer_owner_id,admission_provenance,admitted_at,execute_by,state)
					 SELECT $2,tenant_id,store_id,scope_id,$3,CASE WHEN $4 THEN transport_key_digest ELSE sha256($2::uuid::text::bytea) END,
					 sha256('different request'::bytea),1,true,facts_digest,plan_digest,decode(repeat('02',40),'hex'),
					 'synthetic_v1',buyer_owner_id,admission_provenance,admitted_at,execute_by,'PENDING'
					 FROM ops.order_attempts WHERE attempt_id=$1`, h.attempt, id, generation, reuseKey)
				})
			}
		})
	}
}

func TestOrderAttemptAdmissionStateImmutableFacts(t *testing.T) {
	h := oaStateBegin(t)
	// Well-formed scalar replacements must hit the identity trigger (PT409),
	// rather than merely fail a value check or a later foreign-key lookup.
	for _, assignment := range []string{
		"buyer_owner_id=gen_random_uuid()",
		"request_hash=sha256('changed request'::bytea)",
		"transport_key_digest=sha256('changed key'::bytea)",
		"facts_digest=sha256('changed facts'::bytea)",
		"plan_digest=sha256('changed plan'::bytea)",
		"plan_ciphertext=decode(repeat('03',40),'hex')",
		"plan_key_version='synthetic_v2'",
		"admission_provenance='{\"changed\":true}'::jsonb",
		"admitted_at=admitted_at+interval '1 minute',execute_by=execute_by+interval '1 minute'",
	} {
		t.Run(assignment, func(t *testing.T) {
			oaStateReject(t, h.tx, "PT409", `UPDATE ops.order_attempts SET `+assignment+` WHERE attempt_id=$1`, h.attempt)
		})
	}
	oaStateSettle(t, h.tx, h.attempt, "ABORTED")
	oaStateReject(t, h.tx, "PT409", `UPDATE ops.order_attempts SET receipt=receipt||'{"changed":true}'::jsonb WHERE attempt_id=$1`, h.attempt)
	oaStateReject(t, h.tx, "PT409", `UPDATE ops.order_attempts SET state='PENDING',receipt=NULL,settled_at=NULL,
	 plan_ciphertext=decode(repeat('01',40),'hex'),plan_key_version='synthetic_v1' WHERE attempt_id=$1`, h.attempt)
}

func TestOrderAttemptAdmissionStateRetiredScope(t *testing.T) {
	h := oaStateBegin(t)
	oaStateReject(t, h.tx, "PT409", `UPDATE ops.order_attempt_scopes SET current_generation=0 WHERE scope_id=$1`, h.scope)
	oaStateSettle(t, h.tx, h.attempt, "ABORTED")
	oaStateExec(t, h.tx, `UPDATE ops.order_attempt_scopes SET retired_at=clock_timestamp() WHERE scope_id=$1`, h.scope)
	for _, assignment := range []string{"current_generation=0", "current_generation=2", "retired_at=NULL", "retired_at=retired_at+interval '1 day'"} {
		t.Run(assignment, func(t *testing.T) {
			oaStateReject(t, h.tx, "PT409", `UPDATE ops.order_attempt_scopes SET `+assignment+` WHERE scope_id=$1`, h.scope)
		})
	}
	var generation int64
	var retired bool
	err := h.tx.QueryRow(context.Background(), `SELECT current_generation,retired_at IS NOT NULL FROM ops.order_attempt_scopes WHERE scope_id=$1`, h.scope).Scan(&generation, &retired)
	if err != nil || generation != 1 || !retired {
		t.Fatalf("OA1 retired denial fence changed: generation=%d retired=%v err=%v", generation, retired, err)
	}
}

func TestOrderAttemptAdmissionStateSubjectSettlement(t *testing.T) {
	for _, state := range []string{"ABORTED", "COMMITTED"} {
		for _, scope := range []string{"validated", "restored_empty", "other_scope"} {
			t.Run(state+"/"+scope, func(t *testing.T) {
				h := oaStateBegin(t)
				probe, err := h.tx.Begin(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				wantTenant, wantStore := h.f.tenantA, h.f.storeA1
				if scope == "restored_empty" {
					wantTenant, wantStore = "", ""
				} else if scope == "other_scope" {
					wantTenant, wantStore = h.f.tenantB, h.f.storeB
				}
				oaStateExec(t, probe, `SELECT set_config('app.tenant_id',$1,true),set_config('app.store_id',$2,true)`, wantTenant, wantStore)
				oaStateSettle(t, probe, h.attempt, state)
				if err := oaStateSettlementOracle(probe, h.attempt, state); err != nil {
					t.Fatalf("OA1 same-Tx cleanup: %v", err)
				}
				var gotTenant, gotStore string
				err = probe.QueryRow(context.Background(), `SELECT current_setting('app.tenant_id',true),current_setting('app.store_id',true)`).Scan(&gotTenant, &gotStore)
				if err != nil || gotTenant != wantTenant || gotStore != wantStore {
					t.Fatalf("OA1 cleanup leaked trigger scope: tenant_restored=%v store_restored=%v err=%v", gotTenant == wantTenant, gotStore == wantStore, err)
				}
				// Rollback must restore both state and all three subject kinds.
				if err := probe.Rollback(context.Background()); err != nil {
					t.Fatal(err)
				}
				var subjects int
				err = h.tx.QueryRow(context.Background(), `SELECT count(*) FROM ops.order_attempt_subjects s
				 JOIN ops.order_attempts a USING(attempt_id) WHERE a.attempt_id=$1 AND a.state='PENDING'`, h.attempt).Scan(&subjects)
				if err != nil || subjects != 3 {
					t.Fatalf("OA1 rollback split state/subject effects: subjects=%d err=%v", subjects, err)
				}
				oaStateSettle(t, h.tx, h.attempt, state)
				for _, kind := range []string{"customer", "buyer_owner", "private_owner"} {
					oaStateReject(t, h.tx, "PT409", `INSERT INTO ops.order_attempt_subjects(tenant_id,store_id,subject_kind,subject_id,attempt_id)
					 VALUES($1,$2,$3,$4,$5)`, h.f.tenantA, h.f.storeA1, kind, randomUUID(), h.attempt)
				}
			})
		}
	}
}

func TestOrderAttemptAdmissionStateCleanupMutation(t *testing.T) {
	h := oaStateBegin(t)
	for _, mutate := range []bool{false, true, false} {
		probe, err := h.tx.Begin(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if mutate {
			// Fault injection on native table, protected by a savepoint and outer
			// rollback; no fixture table/stand-in trigger can hide production behavior.
			oaStateExec(t, probe, `ALTER TABLE ops.order_attempts DISABLE TRIGGER order_attempt_identity_immutable`)
		}
		oaStateSettle(t, probe, h.attempt, "ABORTED")
		oracleErr := oaStateSettlementOracle(probe, h.attempt, "ABORTED")
		if err := probe.Rollback(context.Background()); err != nil {
			t.Fatal(err)
		}
		if (oracleErr != nil) != mutate {
			t.Fatalf("OA1 cleanup oracle sensitivity mutate=%v: %v", mutate, oracleErr)
		}
		t.Logf("OA1 native cleanup mutation=%v rejected=%v (rollback restored trigger)", mutate, oracleErr != nil)
	}
}
