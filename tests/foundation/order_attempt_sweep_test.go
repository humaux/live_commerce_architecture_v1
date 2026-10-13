// Purpose: REAL_PG tests-first coverage of native order-attempt worker and sweep recovery.
// Depends on: actual C1, concrete Executor/keyrings, claims login and upstream River.
// Used by: parent-owned OA1 CI; no local execution or substitute consumer is required.
// Invariants: attempt-first repair, one atomic fresh job, unchanged intent, bounded skip,
// deadline settlement without plaintext and actual startup/30-second periodic recovery.
package foundation_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"livecommerce/internal/orderattempt"
)

func oaSweepJob(t *testing.T, e *lbuEnv, attempt string) int64 {
	t.Helper()
	var id int64
	if err := e.p.f.owner.QueryRow(context.Background(), `SELECT execution_job_id FROM ops.order_attempts WHERE attempt_id=$1`, attempt).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func oaSweepJobState(t *testing.T, e *lbuEnv, job int64, state string) {
	t.Helper()
	if state == "missing" {
		mustExec(t, e.p.f.owner, `DELETE FROM river.river_job WHERE id=$1`, job)
		return
	}
	// Change only the lifecycle of this test's real C1 job. The immutable kind,
	// scope-free arguments, queue and identity guard remain active.
	mustExec(t, e.p.f.owner, `UPDATE river.river_job SET state=$2::river.river_job_state,
	 finalized_at=CASE WHEN $2 IN ('completed','discarded','cancelled') THEN clock_timestamp() ELSE NULL END,
	 attempted_at=CASE WHEN $2='running' THEN clock_timestamp() ELSE attempted_at END,
	 attempt=CASE WHEN $2 IN ('running','retryable') THEN max_attempts ELSE attempt END
	 WHERE id=$1`, job, state)
}

func oaSweepIntent(t *testing.T, e *lbuEnv, attempt string) string {
	t.Helper()
	var raw string
	err := e.p.f.owner.QueryRow(context.Background(), `SELECT jsonb_build_object(
	 'attempt_id',attempt_id,'scope_id',scope_id,'intent_generation',intent_generation,
	 'tenant_id',tenant_id,'store_id',store_id,'buyer_owner_id',buyer_owner_id,
	 'transport_key_digest',transport_key_digest,'request_hash',request_hash,'facts_digest',facts_digest,
	 'plan_digest',plan_digest,'plan_ciphertext',plan_ciphertext,'plan_key_version',plan_key_version,
	 'admitted_at',admitted_at,'execute_by',execute_by,'admission_provenance',admission_provenance)::text
	 FROM ops.order_attempts WHERE attempt_id=$1`, attempt).Scan(&raw)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func oaSweepRepair(t *testing.T, e *lbuEnv, a orderattempt.AdmissionReceipt, old int64, intent string) int64 {
	t.Helper()
	var id int64
	var valid bool
	err := e.p.f.owner.QueryRow(context.Background(), `SELECT a.execution_job_id,
	 a.state='PENDING' AND a.enqueue_job_id=a.execution_job_id AND a.enqueue_xid IS NOT NULL
	 AND j.id<>$2 AND j.kind='order_attempt_v1' AND j.queue='claims' AND j.priority=1
	 AND j.unique_key IS NULL AND j.args=jsonb_build_object('attempt_id',a.attempt_id::text,'intent_generation',a.intent_generation)
	 AND j.state IN ('available','scheduled','retryable','pending','running')
	 FROM ops.order_attempts a JOIN river.river_job j ON j.id=a.execution_job_id WHERE a.attempt_id=$1`, a.AttemptID, old).Scan(&id, &valid)
	if err != nil || !valid || id == old {
		t.Fatalf("fresh native repair/pointer linkage missing: valid=%v err=%v", valid, err)
	}
	if oaSweepIntent(t, e, a.AttemptID) != intent {
		t.Fatal("repair changed frozen intent, plan, admission or deadline")
	}
	oaCompositeNoEffects(t, e, a)
	return id
}

func TestOrderAttemptSweepNativeOrphanRepairAndLiveJobs(t *testing.T) {
	for _, state := range []string{"missing", "completed", "discarded", "cancelled", "available", "pending", "scheduled", "retryable", "running"} {
		t.Run(state, func(t *testing.T) {
			e := lbuNew(t)
			a := oaFenceAdmit(t, e)
			old := oaSweepJob(t, e, a.AttemptID)
			oaSweepJobState(t, e, old, state)
			intent := oaSweepIntent(t, e, a.AttemptID)
			pool := miPool(t, e.p.f, "commerce_claims_worker")
			executor := oaExecutorNew(t, pool, oaServiceKeys(t))
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			got, err := executor.Sweep(ctx)
			if err != nil || got.Scanned < 1 || got.Scanned > 100 {
				oaSweepDiagnoseFirst(t, pool)
				t.Fatalf("bounded native discovery/sweep: counts=%+v err=%v", got, err)
			}
			orphan := state == "missing" || state == "completed" || state == "discarded" || state == "cancelled"
			id := old
			if orphan {
				if got.Requeued < 1 {
					t.Fatal("orphan sweep did not report its repair")
				}
				id = oaSweepRepair(t, e, a, old, intent)
			} else if oaSweepJob(t, e, a.AttemptID) != old || oaSweepIntent(t, e, a.AttemptID) != intent {
				t.Fatal("live job was replaced or intent changed; running/retryable at max_attempts remain live")
			}
			if _, err = executor.Sweep(ctx); err != nil {
				oaSweepDiagnoseFirst(t, pool)
				t.Fatal(err)
			}
			if oaSweepJob(t, e, a.AttemptID) != id {
				t.Fatal("repeat sweep inserted a second live execution job")
			}
			oaExecutorNoEffects(t, e, a)
		})
	}
}

func TestOrderAttemptSweepRepairNeverTakesScopeOrPrincipalLock(t *testing.T) {
	e := lbuNew(t)
	a := oaFenceAdmit(t, e)
	old := oaSweepJob(t, e, a.AttemptID)
	oaSweepJobState(t, e, old, "discarded")
	intent := oaSweepIntent(t, e, a.AttemptID)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	holder, err := e.p.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	oaAbortCleanupTx(t, holder)
	defer holder.Rollback(context.Background())
	oaStateExec(t, holder, `SELECT scope_id FROM ops.order_attempt_scopes WHERE scope_id=$1 FOR UPDATE`, a.ScopeID)
	oaStateExec(t, holder, `SELECT id FROM identity.principals WHERE id=$1 FOR UPDATE`, e.p.f.principalA)
	pool := miPool(t, e.p.f, "commerce_claims_worker")
	executor := oaExecutorNew(t, pool, oaServiceKeys(t))
	if _, err = executor.Sweep(ctx); err != nil {
		oaSweepDiagnoseFirst(t, pool)
		t.Fatal("repair reacquired an earlier lock (including implicit scope FK)", err)
	}
	oaSweepRepair(t, e, a, old, intent)
	// A separate later C2 transaction must still execute while the earlier locks
	// are held; do not settle in the repair transaction after its pointer UPDATE.
	plan, _ := oaMaterialPlan(t, e, oaServiceKeys(t), a.AttemptID)
	got, err := executor.Execute(ctx, "", "", a.AttemptID, a.Generation)
	if err != nil {
		t.Fatal("post-repair separate execution transaction", err)
	}
	oaExecutorCommitted(t, e, a, plan, got, 5)
}

func TestOrderAttemptSweepSkipsHeldAttemptWithoutRepair(t *testing.T) {
	e := lbuNew(t)
	a := oaFenceAdmit(t, e)
	old := oaSweepJob(t, e, a.AttemptID)
	oaSweepJobState(t, e, old, "missing")
	intent := oaSweepIntent(t, e, a.AttemptID)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	holder, err := e.p.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	oaAbortCleanupTx(t, holder)
	defer holder.Rollback(context.Background())
	oaStateExec(t, holder, `SELECT attempt_id FROM ops.order_attempts WHERE attempt_id=$1 FOR UPDATE`, a.AttemptID)
	pool := miPool(t, e.p.f, "commerce_claims_worker")
	executor := oaExecutorNew(t, pool, oaServiceKeys(t))
	if _, err = executor.Sweep(ctx); err != nil {
		oaSweepDiagnoseFirst(t, pool)
		t.Fatal("SKIP LOCKED sweep waited or failed on held attempt", err)
	}
	if oaSweepJob(t, e, a.AttemptID) != old || oaSweepIntent(t, e, a.AttemptID) != intent {
		t.Fatal("held attempt was repaired")
	}
	if err = holder.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = executor.Sweep(ctx); err != nil {
		oaSweepDiagnoseFirst(t, pool)
		t.Fatal(err)
	}
	oaSweepRepair(t, e, a, old, intent)
}

func oaSweepExpired(t *testing.T, e *lbuEnv) orderattempt.AdmissionReceipt {
	t.Helper()
	tx, err := e.p.f.owner.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	oaAbortCleanupTx(t, tx)
	a := orderattempt.AdmissionReceipt{AttemptID: randomUUID(), ScopeID: randomUUID(), Generation: 1}
	owner := randomUUID()
	// Past-seeded storage fixture, matching the existing abort deadline test.
	// It is deliberately not a real C1/AEAD positive and disables no trigger.
	oaStateExec(t, tx, `SELECT set_config('app.tenant_id',$1,true),set_config('app.store_id',$2,true)`, e.tenant(), e.store())
	oaStateExec(t, tx, `INSERT INTO buyer.owners(id,tenant_id,store_id) VALUES($1,$2,$3)`, owner, e.tenant(), e.store())
	oaStateExec(t, tx, `INSERT INTO ops.order_attempt_creators(attempt_id,tenant_id,store_id,buyer_owner_id) VALUES($1,$2,$3,$4)`, a.AttemptID, e.tenant(), e.store(), owner)
	oaStateExec(t, tx, `INSERT INTO ops.order_attempt_scopes(scope_id,tenant_id,store_id,principal_id,current_generation) VALUES($1,$2,$3,$4,1)`, a.ScopeID, e.tenant(), e.store(), e.p.f.principalA)
	oaStateExec(t, tx, `INSERT INTO ops.order_attempts(attempt_id,tenant_id,store_id,scope_id,intent_generation,
	 transport_key_digest,request_hash,request_schema_version,buyer_owner_id,admission_provenance,admitted_at,execute_by,state,enqueue_xid)
	 VALUES($1,$2,$3,$4,1,sha256($1::uuid::text::bytea),sha256('synthetic'::bytea),1,$5,
	 jsonb_build_object('principal_id',$6::text),statement_timestamp()-interval '16 minutes',statement_timestamp()-interval '1 minute','PENDING',pg_current_xact_id())`,
		a.AttemptID, e.tenant(), e.store(), a.ScopeID, owner, e.p.f.principalA)
	oaJobInsert(t, tx, a.AttemptID)
	oaJobFinish(t, tx, a.AttemptID)
	oaStateExec(t, tx, `INSERT INTO ops.order_attempt_subjects(tenant_id,store_id,subject_kind,subject_id,attempt_id) VALUES($1,$2,'buyer_owner',$3,$4)`, e.tenant(), e.store(), owner, a.AttemptID)
	if err = tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	return a
}

func TestOrderAttemptSweepExpiredIntentIgnoresJobLiveness(t *testing.T) {
	for _, state := range []string{"available", "running", "discarded", "missing"} {
		t.Run(state, func(t *testing.T) {
			e := lbuNew(t)
			a := oaSweepExpired(t, e)
			old := oaSweepJob(t, e, a.AttemptID)
			oaSweepJobState(t, e, old, state)
			pool := miPool(t, e.p.f, "commerce_claims_worker")
			executor := oaExecutorNew(t, pool, oaServiceKeys(t))
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			counts, err := executor.Sweep(ctx)
			if err != nil || counts.Aborted < 1 {
				oaSweepDiagnoseFirst(t, pool)
				t.Fatalf("expired intent must settle without decrypting invalid old envelope: counts=%+v err=%v", counts, err)
			}
			var raw []byte
			if err = e.p.f.owner.QueryRow(ctx, `SELECT receipt FROM ops.order_attempts WHERE attempt_id=$1`, a.AttemptID).Scan(&raw); err != nil {
				t.Fatal(err)
			}
			oaExecutorAborted(t, e, a, orderattempt.ExecutionReceipt{State: "ABORTED", Receipt: raw}, "EXPIRED")
			if oaSweepJob(t, e, a.AttemptID) != old {
				t.Fatal("expiry minted a replacement job or changed its pointer")
			}
		})
	}
}

func oaSweepConsumer(t *testing.T, pool *pgxpool.Pool, executor *orderattempt.Executor) *river.Client[pgx.Tx] {
	t.Helper()
	workers := river.NewWorkers()
	if err := orderattempt.AddWorkers(workers, executor); err != nil {
		t.Fatal(err)
	}
	if err := orderattempt.AddWorkers(workers, executor); err == nil {
		t.Fatal("duplicate registration silently replaced native workers")
	}
	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Schema: "river", Workers: workers, Queues: map[string]river.QueueConfig{"claims": {MaxWorkers: 4}},
		PeriodicJobs: []*river.PeriodicJob{orderattempt.PeriodicJob()}, RescueStuckJobsAfter: time.Minute,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = client.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pwStopClient(t, client) })
	return client
}

func oaSweepAwait(t *testing.T, ctx context.Context, check func() bool, failure string) {
	t.Helper()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		if check() {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal(failure)
		case <-ticker.C:
		}
	}
}

func TestOrderAttemptSweepRealRiverStartupAndPeriodicOrphanRecovery(t *testing.T) {
	e := lbuNew(t)
	a := oaFenceAdmit(t, e)
	plan, _ := oaMaterialPlan(t, e, oaServiceKeys(t), a.AttemptID)
	old := oaSweepJob(t, e, a.AttemptID)
	// Pending jobs are native-live but not fetchable. Startup sweep must leave it
	// alone; only a later periodic sweep can rescue the injected discarded job.
	oaSweepJobState(t, e, old, "pending")
	var baseline int64
	if err := e.p.f.owner.QueryRow(context.Background(), `SELECT coalesce(max(id),0) FROM river.river_job WHERE kind='order_attempt_sweep_v1'`).Scan(&baseline); err != nil {
		t.Fatal(err)
	}
	pool := miPool(t, e.p.f, "commerce_claims_worker")
	executor := oaExecutorNew(t, pool, oaServiceKeys(t))
	oaSweepConsumer(t, pool, executor)
	defer func() {
		if t.Failed() {
			oaSweepDiagnoseFirst(t, pool)
		}
	}()
	startup, stop := context.WithTimeout(context.Background(), 10*time.Second)
	defer stop()
	var first int64
	var firstAt time.Time
	oaSweepAwait(t, startup, func() bool {
		err := e.p.f.owner.QueryRow(startup, `SELECT id,created_at FROM river.river_job WHERE id>$1
		 AND kind='order_attempt_sweep_v1' AND state='completed' ORDER BY id LIMIT 1`, baseline).Scan(&first, &firstAt)
		if err != nil && err != pgx.ErrNoRows {
			t.Fatal(err)
		}
		return err == nil
	}, "native RunOnStart sweep did not complete")
	oaCompositeNoEffects(t, e, a)
	oaSweepJobState(t, e, old, "discarded")
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	var receipt []byte
	oaSweepAwait(t, ctx, func() bool {
		var state string
		if err := e.p.f.owner.QueryRow(ctx, `SELECT state,receipt FROM ops.order_attempts WHERE attempt_id=$1`, a.AttemptID).Scan(&state, &receipt); err != nil {
			t.Fatal(err)
		}
		return state == "COMMITTED"
	}, "discarded job stranded without native periodic repair and registered executor")
	got := orderattempt.ExecutionReceipt{State: "COMMITTED", Receipt: receipt}
	oaExecutorCommitted(t, e, a, plan, got, 5)
	if oaSweepJob(t, e, a.AttemptID) == old {
		t.Fatal("terminal attempt did not use a repaired execution job")
	}
	var valid bool
	var secondAt time.Time
	err := e.p.f.owner.QueryRow(ctx, `SELECT created_at,queue='claims' AND priority=1 AND args='{}'::jsonb AND unique_key IS NULL
	 FROM river.river_job WHERE kind='order_attempt_sweep_v1' AND id>$1 ORDER BY id LIMIT 1`, first).Scan(&secondAt, &valid)
	if err != nil || !valid || secondAt.Sub(firstAt) < 28*time.Second || secondAt.Sub(firstAt) > 40*time.Second {
		t.Fatalf("native periodic cadence/lane/opaque args: valid=%v interval=%v err=%v", valid, secondAt.Sub(firstAt), err)
	}
}

func TestOrderAttemptSweepExactWorkerOnlyACL(t *testing.T) {
	f := fixture(t)
	var valid bool
	err := f.owner.QueryRow(context.Background(), `SELECT p.prosecdef AND p.proconfig=ARRAY['search_path=pg_catalog']
	 AND pg_get_userbyid(p.proowner)='commerce_order_attempt_owner'
	 AND NOT EXISTS(SELECT 1 FROM aclexplode(coalesce(p.proacl,acldefault('f',p.proowner))) a
	  WHERE a.privilege_type='EXECUTE' AND a.grantee NOT IN (p.proowner,'commerce_claims_worker'::regrole))
	 AND has_function_privilege('commerce_claims_worker',p.oid,'EXECUTE')
	 FROM pg_proc p WHERE p.oid=to_regprocedure('ops.order_attempt_sweep(text,uuid,bigint)')`).Scan(&valid)
	if err != nil || !valid {
		t.Fatalf("one exact coordinator-owned worker-only sweep entry missing: %v", err)
	}
	var raw json.RawMessage
	err = f.runtime.QueryRow(context.Background(), `SELECT ops.order_attempt_sweep('discover',NULL,NULL)`).Scan(&raw)
	oaJobError(t, err, "42501")
}

func oaSweepCall(t *testing.T, tx pgx.Tx, phase string, a orderattempt.AdmissionReceipt) []byte {
	t.Helper()
	var raw []byte
	err := tx.QueryRow(context.Background(), `SELECT ops.order_attempt_sweep($1,$2::uuid,$3::bigint)`, phase, a.AttemptID, a.Generation).Scan(&raw)
	if err != nil {
		t.Fatalf("native sweep %s: %v", phase, err)
	}
	return raw
}

func oaSweepUpdates(t *testing.T, tx pgx.Tx, want int64) {
	t.Helper()
	var updates int64
	// PostgreSQL's per-transaction counter includes trigger writes. Reading its
	// system view needs no business-table grant and observes this physical Tx.
	err := tx.QueryRow(context.Background(), `SELECT n_tup_upd FROM pg_stat_xact_user_tables WHERE relid='ops.order_attempts'::regclass`).Scan(&updates)
	if err != nil || updates != want {
		t.Fatalf("attempt updates in this transaction: got=%d want=%d err=%v", updates, want, err)
	}
}

func TestOrderAttemptSweepNativeRepairOneAttemptUpdate(t *testing.T) {
	e := lbuNew(t)
	a := oaFenceAdmit(t, e)
	old := oaSweepJob(t, e, a.AttemptID)
	oaSweepJobState(t, e, old, "discarded")
	intent := oaSweepIntent(t, e, a.AttemptID)
	pool := miPool(t, e.p.f, "commerce_claims_worker")
	producer, err := river.NewClient(riverpgxv5.New(pool), &river.Config{Schema: "river"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	holder, err := e.p.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	oaAbortCleanupTx(t, holder)
	defer holder.Rollback(context.Background())
	oaStateExec(t, holder, `SELECT scope_id FROM ops.order_attempt_scopes WHERE scope_id=$1 FOR UPDATE`, a.ScopeID)
	oaStateExec(t, holder, `SELECT id FROM identity.principals WHERE id=$1 FOR UPDATE`, e.p.f.principalA)
	for _, finalize := range []bool{false, true} {
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		oaAbortCleanupTx(t, tx)
		oaStateExec(t, tx, `SELECT set_config('lock_timeout','1s',true),set_config('statement_timeout','5s',true)`)
		var prepared struct {
			Action     string `json:"action"`
			AttemptID  string `json:"attempt_id"`
			Generation int64  `json:"intent_generation"`
		}
		if err = json.Unmarshal(oaSweepCall(t, tx, "prepare", a), &prepared); err != nil || prepared.Action != "REQUEUE" || prepared.AttemptID != a.AttemptID || prepared.Generation != a.Generation {
			t.Fatalf("exact native repair disposition: %v", err)
		}
		oaSweepUpdates(t, tx, 0)
		args, err := json.Marshal(map[string]any{"attempt_id": a.AttemptID, "intent_generation": a.Generation})
		if err != nil {
			t.Fatal(err)
		}
		inserted, err := producer.InsertTx(ctx, tx, oaRawJob{"order_attempt_v1", args}, &river.InsertOpts{Queue: "claims", Priority: 1})
		if err != nil {
			t.Fatal("real claims InsertTx must consume only its protected repair witness", err)
		}
		oaSweepUpdates(t, tx, 0)
		if !finalize {
			if err = tx.Commit(ctx); err == nil {
				t.Fatal("fresh repair job committed without exact final attempt pointer")
			}
			if oaSweepJob(t, e, a.AttemptID) != old || oaSweepIntent(t, e, a.AttemptID) != intent {
				t.Fatal("failed reciprocal commit left repair effects")
			}
			var leaked int
			if err = e.p.f.owner.QueryRow(ctx, `SELECT count(*) FROM river.river_job WHERE id=$1`, inserted.Job.ID).Scan(&leaked); err != nil || leaked != 0 {
				t.Fatalf("unfinalized repair job survived rollback: %d %v", leaked, err)
			}
			continue
		}
		var finished struct {
			AttemptID  string `json:"attempt_id"`
			Generation int64  `json:"intent_generation"`
			JobID      int64  `json:"execution_job_id"`
		}
		if err = json.Unmarshal(oaSweepCall(t, tx, "finish", a), &finished); err != nil || finished.AttemptID != a.AttemptID || finished.Generation != a.Generation || finished.JobID != inserted.Job.ID {
			t.Fatalf("finish did not bind exact same-Tx River insertion: %v", err)
		}
		oaSweepUpdates(t, tx, 1)
		if err = tx.Commit(ctx); err != nil {
			t.Fatal("single-update repair must commit with native scope/principal locks held", err)
		}
		oaSweepRepair(t, e, a, old, intent)
	}
}

func TestOrderAttemptSweepDiscoveryIsOpaqueAndOldestBounded(t *testing.T) {
	e := lbuNew(t)
	a := oaFenceAdmit(t, e)
	// Native past-seeded metadata makes the LIMIT observable with >100 candidates.
	// It does not claim C1/AEAD coverage; the target above is admitted by real C1.
	rowsToSeed := make([]map[string]string, 101)
	attempts, scopes, owners, principals := []string{}, []string{}, []string{}, []string{}
	for i := range rowsToSeed {
		attempt, scope, owner, principal := randomUUID(), randomUUID(), randomUUID(), randomUUID()
		rowsToSeed[i] = map[string]string{"attempt": attempt, "scope": scope, "owner": owner, "principal": principal}
		attempts, scopes, owners, principals = append(attempts, attempt), append(scopes, scope), append(owners, owner), append(principals, principal)
	}
	seed, err := json.Marshal(rowsToSeed)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		// Exact owned IDs only; never clear another test's attempt/job backlog.
		mustExec(t, e.p.f.owner, `DELETE FROM river.river_job WHERE kind='order_attempt_v1' AND (args->>'attempt_id')::uuid=ANY($1::uuid[])`, attempts)
		mustExec(t, e.p.f.owner, `DELETE FROM ops.order_attempts WHERE attempt_id=ANY($1::uuid[])`, attempts)
		mustExec(t, e.p.f.owner, `DELETE FROM ops.order_attempt_creators WHERE attempt_id=ANY($1::uuid[])`, attempts)
		mustExec(t, e.p.f.owner, `DELETE FROM ops.order_attempt_scopes WHERE scope_id=ANY($1::uuid[])`, scopes)
		mustExec(t, e.p.f.owner, `DELETE FROM buyer.owners WHERE id=ANY($1::uuid[])`, owners)
		mustExec(t, e.p.f.owner, `DELETE FROM identity.memberships WHERE principal_id=ANY($1::uuid[])`, principals)
		mustExec(t, e.p.f.owner, `DELETE FROM identity.principals WHERE id=ANY($1::uuid[])`, principals)
	})
	tx, err := e.p.f.owner.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	oaAbortCleanupTx(t, tx)
	oaStateExec(t, tx, `INSERT INTO identity.principals(id) SELECT (x->>'principal')::uuid FROM jsonb_array_elements($1::jsonb) x`, string(seed))
	oaStateExec(t, tx, `INSERT INTO identity.memberships(tenant_id,principal_id) SELECT $2,(x->>'principal')::uuid FROM jsonb_array_elements($1::jsonb) x`, string(seed), e.tenant())
	oaStateExec(t, tx, `INSERT INTO buyer.owners(id,tenant_id,store_id) SELECT (x->>'owner')::uuid,$2,$3 FROM jsonb_array_elements($1::jsonb) x`, string(seed), e.tenant(), e.store())
	oaStateExec(t, tx, `INSERT INTO ops.order_attempt_creators(attempt_id,tenant_id,store_id,buyer_owner_id)
	 SELECT (x->>'attempt')::uuid,$2,$3,(x->>'owner')::uuid FROM jsonb_array_elements($1::jsonb) x`, string(seed), e.tenant(), e.store())
	oaStateExec(t, tx, `INSERT INTO ops.order_attempt_scopes(scope_id,tenant_id,store_id,principal_id,current_generation)
	 SELECT (x->>'scope')::uuid,$2,$3,(x->>'principal')::uuid,1 FROM jsonb_array_elements($1::jsonb) x`, string(seed), e.tenant(), e.store())
	oaStateExec(t, tx, `INSERT INTO ops.order_attempts(attempt_id,tenant_id,store_id,scope_id,intent_generation,
	 transport_key_digest,request_hash,request_schema_version,buyer_owner_id,admission_provenance,admitted_at,execute_by,state,enqueue_xid)
	 SELECT (x->>'attempt')::uuid,$2,$3,(x->>'scope')::uuid,1,sha256(convert_to(x->>'attempt','UTF8')),sha256('synthetic'::bytea),1,
	 (x->>'owner')::uuid,jsonb_build_object('principal_id',x->>'principal'),statement_timestamp()-interval '16 minutes'-n*interval '1 millisecond',
	 statement_timestamp()-interval '1 minute'-n*interval '1 millisecond','PENDING',pg_current_xact_id()
	 FROM jsonb_array_elements($1::jsonb) WITH ORDINALITY seeded(x,n)`, string(seed), e.tenant(), e.store())
	oaStateExec(t, tx, `INSERT INTO river.river_job(kind,args,queue,priority,max_attempts)
	 SELECT 'order_attempt_v1',jsonb_build_object('attempt_id',x->>'attempt','intent_generation',1),'claims',1,25
	 FROM jsonb_array_elements($1::jsonb) x`, string(seed))
	oaStateExec(t, tx, `UPDATE ops.order_attempts SET admission_finished=true,facts_digest=sha256('facts'::bytea),plan_digest=sha256('plan'::bytea),
	 plan_ciphertext=decode(repeat('01',40),'hex'),plan_key_version='synthetic_v1',execution_job_id=enqueue_job_id WHERE attempt_id=ANY($1::uuid[])`, attempts)
	if err = tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	pool := miPool(t, e.p.f, "commerce_claims_worker")
	var got, expected []byte
	err = pool.QueryRow(context.Background(), `SELECT ops.order_attempt_sweep('discover',NULL,NULL)`).Scan(&got)
	if err != nil {
		t.Fatal(err)
	}
	err = e.p.f.owner.QueryRow(context.Background(), `SELECT coalesce(jsonb_agg(jsonb_build_object('attempt_id',attempt_id,'intent_generation',intent_generation) ORDER BY admitted_at,attempt_id),'[]'::jsonb)
	 FROM (SELECT attempt_id,intent_generation,admitted_at FROM ops.order_attempts WHERE state='PENDING' AND admission_finished ORDER BY admitted_at,attempt_id LIMIT 100) oldest`).Scan(&expected)
	if err != nil {
		t.Fatal(err)
	}
	var rows []map[string]json.RawMessage
	if err = json.Unmarshal(got, &rows); err != nil || len(rows) != 100 {
		t.Fatalf("bounded discovery array: count=%d err=%v", len(rows), err)
	}
	for _, row := range rows {
		if len(row) != 2 || row["attempt_id"] == nil || row["intent_generation"] == nil {
			t.Fatal("discovery exposed anything beyond opaque identity")
		}
	}
	oaExecutorSameReceipt(t, orderattempt.ExecutionReceipt{Receipt: got}, orderattempt.ExecutionReceipt{Receipt: expected})
	oaCompositeNoEffects(t, e, a)
}
