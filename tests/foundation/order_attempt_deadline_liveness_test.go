// Purpose: REAL_PG liveness through the unmodified fifteen-minute C1 deadline.
// Depends on: real claim admission, actual River consumer/periodic sweep and SQL clock.
// Used by: parent-owned long CI; expected duration 15 minutes, bounded at execute_by+90s.
// Invariants: retryable C2 faults never clear custody or extend intent; real periodic
// expiry releases native admission and permits an explicit next-generation CAS.
package foundation_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/orderattempt"
)

func oaDeadlineFault(t *testing.T, e *lbuEnv, attempt string) (string, func()) {
	t.Helper()
	name := "zz_oa_deadline_" + strings.ReplaceAll(randomUUID(), "-", "")
	fn := pgx.Identifier{"public", name}.Sanitize()
	seq := pgx.Identifier{"public", name + "_hits"}.Sanitize()
	trigger := pgx.Identifier{name}.Sanitize()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := e.p.f.owner.Exec(ctx, "CREATE SEQUENCE "+seq); err != nil {
		t.Fatal(err)
	}
	cleaned := false
	cleanup := func() {
		if cleaned {
			return
		}
		clean, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		for _, statement := range []string{
			"DROP TRIGGER IF EXISTS " + trigger + " ON checkout.orders",
			"DROP FUNCTION IF EXISTS " + fn + "()",
			"DROP SEQUENCE IF EXISTS " + seq,
		} {
			if _, err := e.p.f.owner.Exec(clean, statement); err != nil {
				t.Error("bounded own deadline fault cleanup", err)
				return
			}
		}
		cleaned = true
	}
	t.Cleanup(cleanup)
	// nextval survives transaction rollback, unlike a test table INSERT. Only
	// this test-owned sequence is changed; the exact order INSERT still fails.
	ddl := fmt.Sprintf(`CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
	 BEGIN
	  PERFORM nextval('%s'::regclass);
	  RAISE EXCEPTION 'oa_deadline_retryable_fault' USING ERRCODE='XX000';
	 END $$`, fn, seq)
	if _, err := e.p.f.owner.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	if _, err := e.p.f.owner.Exec(ctx, "REVOKE ALL ON FUNCTION "+fn+"() FROM PUBLIC"); err != nil {
		t.Fatal(err)
	}
	ddl = fmt.Sprintf("CREATE TRIGGER %s AFTER INSERT ON checkout.orders FOR EACH ROW WHEN (NEW.create_attempt_id='%s'::uuid) EXECUTE FUNCTION %s()", trigger, attempt, fn)
	if _, err := e.p.f.owner.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	return seq, cleanup
}

func TestOrderAttemptDeadlineRealClaimPeriodicLiveness(t *testing.T) {
	e := lbuNew(t)
	_, offer, bundle, conversation := e.scenario()
	selection := oaClaimSelection(e, []string{bundle}, []string{offer.ID}, conversation)
	req := oaMaterialRequest(t, e, selection)
	keys := oaServiceKeys(t)
	svc, err := orderattempt.NewService(e.p.f.runtime, keys)
	if err != nil {
		t.Fatal(err)
	}
	a, err := svc.Admit(context.Background(), e.token(), e.store(), req)
	if err != nil || a.State != "PENDING" || a.Replay {
		t.Fatal("real claim C1 admission", err)
	}
	// Opening the actual ciphertext is a positive sentinel, not a backdated
	// storage fixture. The SQL deadline remains immutable throughout this test.
	oaMaterialPlan(t, e, keys, a.AttemptID)
	var deadline, now time.Time
	var custody bool
	err = e.p.f.owner.QueryRow(context.Background(), `SELECT a.execute_by,clock_timestamp(),
	 a.execute_by=a.admitted_at+interval '15 minutes'
	 AND (SELECT count(*)=1 FROM inbox.order_for_buyer r WHERE r.create_attempt_id=a.attempt_id
	  AND r.bundle_id=$2 AND r.request_id=a.attempt_id AND r.state='pending' AND r.order_id IS NULL AND r.expires_at=a.execute_by)
	 AND (SELECT count(*)=1 FROM claims.merchant_origin_grants g WHERE g.create_attempt_id=a.attempt_id
	  AND g.bundle_id=$2 AND g.request_id=a.attempt_id AND g.quote_id IS NULL AND g.consumed_at IS NULL AND g.expires_at=a.execute_by)
	 AND EXISTS(SELECT 1 FROM ops.order_attempt_subjects s WHERE s.attempt_id=a.attempt_id)
	 FROM ops.order_attempts a WHERE a.attempt_id=$1`, a.AttemptID, bundle).Scan(&deadline, &now, &custody)
	if err != nil || !custody || deadline.Sub(now) < 14*time.Minute {
		t.Fatalf("real fifteen-minute admission/grant/reservation sentinel: custody=%v err=%v", custody, err)
	}
	t.Log("long native deadline gate: waiting for unchanged C1 execute_by; maximum remaining duration", deadline.Sub(now)+90*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), deadline.Sub(now)+90*time.Second)
	defer cancel()
	intent := oaSweepIntent(t, e, a.AttemptID)
	job := oaSweepJob(t, e, a.AttemptID)
	seq, removeFault := oaDeadlineFault(t, e, a.AttemptID)
	// Only the original River delivery budget is made one attempt, so the real
	// XX000 failure becomes a discarded orphan for the native periodic repair.
	if _, err = e.p.f.owner.Exec(ctx, `UPDATE river.river_job SET max_attempts=1 WHERE id=$1`, job); err != nil {
		t.Fatal(err)
	}
	pool := miPool(t, e.p.f, "commerce_claims_worker")
	consumer := oaSweepConsumer(t, pool, oaExecutorNew(t, pool, keys))
	first, stopFirst := context.WithTimeout(ctx, 10*time.Second)
	defer stopFirst()
	oaSweepAwait(t, first, func() bool {
		var faulted bool
		query := `SELECT is_called AND last_value>=1 AND EXISTS(SELECT 1 FROM river.river_job
		 WHERE id=$1 AND state='discarded' AND attempt=1 AND coalesce(cardinality(errors),0)=1) FROM ` + seq
		if err := e.p.f.owner.QueryRow(first, query, job).Scan(&faulted); err != nil {
			t.Fatal(err)
		}
		return faulted
	}, "actual consumer did not persist its first retryable SQL failure/discard")
	oaExecutorNoEffects(t, e, a)
	if oaSweepIntent(t, e, a.AttemptID) != intent {
		t.Fatal("retryable SQL fault changed frozen admission or deadline")
	}
	// Require the actual periodic job to repair the discarded delivery and its
	// replacement consumer to reach the same fault. No direct Sweep/Execute call.
	repair, stopRepair := context.WithTimeout(ctx, 75*time.Second)
	defer stopRepair()
	var replacement int64
	oaSweepAwait(t, repair, func() bool {
		query := `UPDATE river.river_job j SET state='pending',finalized_at=NULL
		 FROM ops.order_attempts a WHERE a.attempt_id=$1 AND j.id=a.execution_job_id AND j.id<>$2
		 AND a.state='PENDING' AND j.state='retryable' AND j.attempt>=1 AND coalesce(cardinality(j.errors),0)>=1
		 AND (SELECT is_called AND last_value>=2 FROM ` + seq + `) RETURNING j.id`
		err := e.p.f.owner.QueryRow(repair, query, a.AttemptID, job).Scan(&replacement)
		if err != nil && err != pgx.ErrNoRows {
			t.Fatal(err)
		}
		return err == nil
	}, "real periodic repair and replacement consumer did not repeat the native fault")
	oaExecutorNoEffects(t, e, a)
	if oaSweepIntent(t, e, a.AttemptID) != intent {
		t.Fatal("repair/retry changed frozen plan or deadline")
	}
	// Pending is live but unfetchable. Keep the SQL fault installed and let only
	// the registered periodic sweeper expire the intent, independent of job liveness.
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	var terminal orderattempt.ExecutionReceipt
	for {
		probe, stop := context.WithTimeout(ctx, 3*time.Second)
		var expired, unchanged bool
		err = e.p.f.owner.QueryRow(probe, `SELECT state,receipt,clock_timestamp()>=execute_by,execute_by=$2
		 FROM ops.order_attempts WHERE attempt_id=$1`, a.AttemptID, deadline).Scan(&terminal.State, &terminal.Receipt, &expired, &unchanged)
		stop()
		if err != nil || !unchanged {
			t.Fatal("native deadline observation failed or deadline changed", err)
		}
		if terminal.State != "PENDING" {
			if !expired || terminal.State != "ABORTED" {
				t.Fatal("transient fault settled intent early or created a native order")
			}
			break
		}
		select {
		case <-ctx.Done():
			oaSweepDiagnoseFirst(t, pool)
			t.Fatal("periodic sweep stranded genuine C1 beyond execute_by+90s")
		case <-tick.C:
		}
	}
	oaExecutorAborted(t, e, a, terminal, "EXPIRED")
	oaSweepAwait(t, ctx, func() bool {
		var completed bool
		if err := e.p.f.owner.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM river.river_job
		 WHERE kind='order_attempt_sweep_v1' AND state='completed' AND finalized_at>=$1)`, deadline).Scan(&completed); err != nil {
			t.Fatal(err)
		}
		return completed
	}, "expiry committed but the real periodic job never completed")
	var released bool
	err = e.p.f.owner.QueryRow(ctx, `SELECT
	 (SELECT count(*)=1 FROM inbox.order_for_buyer WHERE create_attempt_id=$1 AND bundle_id=$2
	  AND state='released' AND order_id IS NULL AND expires_at=$3)
	 AND (SELECT count(*)=1 FROM claims.merchant_origin_grants WHERE create_attempt_id=$1 AND bundle_id=$2
	  AND quote_id IS NULL AND consumed_at IS NULL AND expires_at=$3 AND expires_at<=clock_timestamp())
	 AND EXISTS(SELECT 1 FROM river.river_job WHERE id=$4 AND state='pending')
	 AND EXISTS(SELECT 1 FROM river.river_job WHERE kind='order_attempt_sweep_v1' AND state='completed' AND finalized_at>=$3)`,
		a.AttemptID, bundle, deadline, replacement).Scan(&released)
	if err != nil || !released {
		t.Fatal("periodic expiry did not preserve native release/custody proof", err)
	}
	pwStopClient(t, consumer)
	removeFault()
	// Fresh authentication avoids coupling the fifteen-minute gate to a browser
	// session TTL. It is the same admitted principal, and advances CAS explicitly.
	relogin := lcToken(t, e.p.f, e.p.f.principalA)
	req.ExpectedGeneration, req.PreviousAttemptID = a.Generation, &a.AttemptID
	req.TransportKey = "oa-deadline-next-" + randomUUID()
	next, err := svc.Admit(ctx, relogin, e.store(), req)
	if err != nil || next.State != "PENDING" || next.Replay || next.ScopeID != a.ScopeID || next.Generation != a.Generation+1 || next.AttemptID == a.AttemptID {
		t.Fatal("released genuine claim could not be admitted by explicit next CAS", err)
	}
	if _, err = svc.Abort(ctx, relogin, e.store(), next.AttemptID, next.Generation, "PLAN_UNAVAILABLE"); err != nil {
		t.Fatal(err)
	}
	old, err := svc.Abort(ctx, relogin, e.store(), a.AttemptID, a.Generation, "PLAN_UNAVAILABLE")
	if err != nil {
		t.Fatal(err)
	}
	oaExecutorSameReceipt(t, terminal, old)
}
