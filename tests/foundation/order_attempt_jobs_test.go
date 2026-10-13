// Purpose: native OA1 River job identity, private ACL and reciprocal commit gates.
// Depends: migrated PG, existing real runtime/worker logins and River InsertTx.
// Used: OA1 CI; local execution is intentionally not part of this task.
// Invariants: only a fresh protected enqueue witness permits an execution job;
// malformed/cross-lane jobs never grant admission. Synthetic storage fixtures
// below prove native constraints only, not authenticated C1 or complete C2.
package foundation_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

type oaRawJob struct {
	kind string
	args json.RawMessage
}

func (j oaRawJob) Kind() string { return j.kind }
func (j oaRawJob) MarshalJSON() ([]byte, error) { return j.args, nil }

func oaJobError(t *testing.T, err error, code string) {
	t.Helper()
	var p *pgconn.PgError
	if !errors.As(err, &p) || p.Code != code {
		t.Fatalf("want SQLSTATE %s, got %v", code, err)
	}
}

func TestOrderAttemptJobsPrivateGuardACL(t *testing.T) {
	f := fixture(t)
	for _, signature := range []string{
		"ops.guard_order_attempt_job()",
		"ops.check_order_attempt_job_link()",
		"ops.check_order_attempt_enqueue_link()",
		"ops.order_attempt_job_matches(bigint,uuid,bigint)",
	} {
		var valid bool
		err := f.owner.QueryRow(context.Background(), `SELECT p.prosecdef AND p.proconfig=ARRAY['search_path=pg_catalog']
		 AND NOT EXISTS(SELECT 1 FROM aclexplode(coalesce(p.proacl,acldefault('f',p.proowner))) a WHERE a.grantee=0)
		 AND NOT EXISTS(SELECT 1 FROM unnest(ARRAY['commerce_runtime','commerce_claims_worker','commerce_ads_worker',
		 'commerce_payment_worker','commerce_expiry_worker','commerce_worker']) r(name) WHERE has_function_privilege(r.name,p.oid,'EXECUTE'))
		 FROM pg_proc p WHERE p.oid=to_regprocedure($1)`, signature).Scan(&valid)
		if err != nil || !valid { t.Fatalf("private guard %s: valid=%v err=%v", signature, valid, err) }
	}
}

func TestOrderAttemptJobsRealRuntimeInsertTxRefusals(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	client, err := river.NewClient(riverpgxv5.New(f.runtime), &river.Config{Schema: "river"})
	if err != nil { t.Fatal(err) }
	valid := fmt.Sprintf(`{"attempt_id":%q,"intent_generation":1}`, randomUUID())
	for _, tc := range []struct { name, kind, args, queue string; priority int }{
		{"no_witness", "order_attempt_v1", valid, "claims", 1},
		{"extra_pii", "order_attempt_v1", valid[:len(valid)-1]+`,"contact":"synthetic"}`, "claims", 1},
		{"string_generation", "order_attempt_v1", fmt.Sprintf(`{"attempt_id":%q,"intent_generation":"1"}`, randomUUID()), "claims", 1},
		{"zero_generation", "order_attempt_v1", fmt.Sprintf(`{"attempt_id":%q,"intent_generation":0}`, randomUUID()), "claims", 1},
		{"bad_uuid", "order_attempt_v1", `{"attempt_id":"invalid","intent_generation":1}`, "claims", 1},
		{"wrong_queue", "order_attempt_v1", valid, "default", 1},
		{"wrong_priority", "order_attempt_v1", valid, "claims", 2},
		{"runtime_sweep", "order_attempt_sweep_v1", `{}`, "claims", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := f.runtime.Begin(ctx)
			if err != nil { t.Fatal(err) }
			defer tx.Rollback(ctx)
			if _, err = tx.Exec(ctx, `SELECT set_config('app.tenant_id',$1,true),set_config('app.store_id',$2,true)`, f.tenantA, f.storeA1); err != nil { t.Fatal(err) }
			_, err = client.InsertTx(ctx, tx, oaRawJob{tc.kind, json.RawMessage(tc.args)}, &river.InsertOpts{Queue: tc.queue, Priority: tc.priority})
			oaJobError(t, err, "22023")
		})
	}
}

func TestOrderAttemptJobsSweepUsesClaimsLoginOnly(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	for _, role := range []string{waClaims, waAds} {
		t.Run(role, func(t *testing.T) {
			pool, err := pgxpool.New(ctx, bcRole(t, f, role))
			if err != nil { t.Fatal(err) }
			defer pool.Close()
			client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{Schema: "river"})
			if err != nil { t.Fatal(err) }
			tx, err := pool.Begin(ctx)
			if err != nil { t.Fatal(err) }
			defer tx.Rollback(ctx)
			job, err := client.InsertTx(ctx, tx, oaRawJob{"order_attempt_sweep_v1", json.RawMessage(`{}`)}, &river.InsertOpts{Queue: "claims", Priority: 1})
			if role != waClaims { oaJobError(t, err, "22023"); return }
			if err != nil || job.Job.ID <= 0 { t.Fatalf("claims sweep positive sentinel: %v", err) }
			if _, err = tx.Exec(ctx, `SET CONSTRAINTS ALL IMMEDIATE`); err != nil { t.Fatal(err) }
			_, err = tx.Exec(ctx, `UPDATE river.river_job SET kind='order_attempt_v1' WHERE id=$1`, job.Job.ID)
			oaJobError(t, err, "22023")
		})
	}
}

// A native storage fixture: superuser inserts synthetic domain rows, without any
// fake SECURITY DEFINER C1 entry or additional grants to application roles.
func oaJobCandidate(t *testing.T) (*testFixture, pgx.Tx, string) {
	t.Helper()
	f := fixture(t)
	tx, err := f.owner.Begin(context.Background())
	if err != nil { t.Fatal(err) }
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	attempt, owner, scope := randomUUID(), randomUUID(), randomUUID()
	oaStateExec(t, tx, `INSERT INTO buyer.owners(id,tenant_id,store_id) VALUES($1,$2,$3)`, owner, f.tenantA, f.storeA1)
	oaStateExec(t, tx, `INSERT INTO ops.order_attempt_creators(attempt_id,tenant_id,store_id,buyer_owner_id) VALUES($1,$2,$3,$4)`, attempt, f.tenantA, f.storeA1, owner)
	oaStateExec(t, tx, `INSERT INTO ops.order_attempt_scopes(scope_id,tenant_id,store_id,principal_id,current_generation) VALUES($1,$2,$3,$4,1)`, scope, f.tenantA, f.storeA1, f.principalA)
	oaStateExec(t, tx, `INSERT INTO ops.order_attempts(attempt_id,tenant_id,store_id,scope_id,intent_generation,
	 transport_key_digest,request_hash,request_schema_version,buyer_owner_id,admission_provenance,admitted_at,execute_by,state,enqueue_xid)
	 VALUES($1,$2,$3,$4,1,sha256($1::uuid::text::bytea),sha256('synthetic'::bytea),1,$5,'{}',
	 statement_timestamp(),statement_timestamp()+interval '15 minutes','PENDING',pg_current_xact_id())`, attempt, f.tenantA, f.storeA1, scope, owner)
	// Simulate the caller context restored by the real C1 entry, without pretending
	// the synthetic fixture itself authenticates a merchant.
	oaStateExec(t, tx, `SELECT set_config('app.tenant_id','',true),set_config('app.store_id','',true)`)
	return f, tx, attempt
}

func oaJobInsert(t *testing.T, tx pgx.Tx, attempt string) int64 {
	t.Helper()
	var job int64
	err := tx.QueryRow(context.Background(), `INSERT INTO river.river_job(kind,args,queue,priority,max_attempts)
	 VALUES('order_attempt_v1',jsonb_build_object('attempt_id',$1::text,'intent_generation',1),'claims',1,25) RETURNING id`, attempt).Scan(&job)
	if err != nil { t.Fatal(err) }
	return job
}

func oaJobFinish(t *testing.T, tx pgx.Tx, attempt string) {
	t.Helper()
	oaStateExec(t, tx, `UPDATE ops.order_attempts SET admission_finished=true,
	 facts_digest=sha256('facts'::bytea),plan_digest=sha256('plan'::bytea),plan_ciphertext=decode(repeat('01',40),'hex'),
	 plan_key_version='synthetic_v1',execution_job_id=enqueue_job_id WHERE attempt_id=$1`, attempt)
}

func TestOrderAttemptJobsNativeReciprocalCommit(t *testing.T) {
	for _, fault := range []string{"none", "missing_job", "deleted_job", "unfinished", "wrong_pointer", "duplicate_job", "stale_witness", "wrong_generation"} {
		t.Run(fault, func(t *testing.T) {
			_, tx, attempt := oaJobCandidate(t)
			ctx := context.Background()
			if fault == "stale_witness" {
				oaStateExec(t, tx, `UPDATE ops.order_attempts SET enqueue_xid='1'::xid8 WHERE attempt_id=$1`, attempt)
				_, err := tx.Exec(ctx, `INSERT INTO river.river_job(kind,args,queue,priority,max_attempts) VALUES('order_attempt_v1',jsonb_build_object('attempt_id',$1::text,'intent_generation',1),'claims',1,25)`, attempt)
				oaJobError(t, err, "22023"); return
			}
			if fault == "wrong_generation" {
				_, err := tx.Exec(ctx, `INSERT INTO river.river_job(kind,args,queue,priority,max_attempts) VALUES('order_attempt_v1',jsonb_build_object('attempt_id',$1::text,'intent_generation',2),'claims',1,25)`, attempt)
				oaJobError(t, err, "22023"); return
			}
			var job int64
			if fault != "missing_job" { job = oaJobInsert(t, tx, attempt) }
			if fault == "duplicate_job" {
				_, err := tx.Exec(ctx, `INSERT INTO river.river_job(kind,args,queue,priority,max_attempts) VALUES('order_attempt_v1',jsonb_build_object('attempt_id',$1::text,'intent_generation',1),'claims',1,25)`, attempt)
				oaJobError(t, err, "22023"); return
			}
			if fault != "unfinished" { oaJobFinish(t, tx, attempt) }
			if fault == "deleted_job" { oaStateExec(t, tx, `DELETE FROM river.river_job WHERE id=$1`, job) }
			if fault == "wrong_pointer" { oaStateExec(t, tx, `UPDATE ops.order_attempts SET execution_job_id=$2 WHERE attempt_id=$1`, attempt, job+1) }
			_, err := tx.Exec(ctx, `SET CONSTRAINTS ALL IMMEDIATE`)
			if fault != "none" { oaJobError(t, err, "23514"); return }
			if err != nil { t.Fatalf("native final-row reciprocal sentinel: %v", err) }
			var tenant, store string
			if err = tx.QueryRow(ctx, `SELECT current_setting('app.tenant_id',true),current_setting('app.store_id',true)`).Scan(&tenant, &store); err != nil || tenant != "" || store != "" { t.Fatalf("guard leaked scoped context: %q/%q %v", tenant, store, err) }
		})
	}
}
