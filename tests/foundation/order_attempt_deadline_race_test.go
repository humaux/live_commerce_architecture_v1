// Purpose: REAL_PG C2-versus-genuine-execute_by race, including both clock winners.
// Depends on: real C1/Executor/Sweep, native audit trigger and PostgreSQL clock/PID.
// Used by: parent long CI; expected duration fifteen minutes, no deadline rewriting.
// Invariants: unchanged 5s/1s budgets, final clock check after every local effect,
// SKIP LOCKED competing sweep, and zero-effects EXPIRED after complete C2 rollback.
package foundation_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/claims"
	"livecommerce/internal/orderattempt"
)

func oaDeadlineAuditBarrier(t *testing.T, e *lbuEnv, attempt string) string {
	t.Helper()
	name := "zz_oa_clock_" + strings.ReplaceAll(randomUUID(), "-", "")
	fn := pgx.Identifier{"public", name}.Sanitize()
	seq := pgx.Identifier{"public", name + "_pid"}.Sanitize()
	trigger := pgx.Identifier{name}.Sanitize()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := e.p.f.owner.Exec(ctx, "CREATE SEQUENCE "+seq); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		clean, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		for _, statement := range []string{
			"DROP TRIGGER IF EXISTS " + trigger + " ON ops.audit_events",
			"DROP FUNCTION IF EXISTS " + fn + "()",
			"DROP SEQUENCE IF EXISTS " + seq,
		} {
			if _, err := e.p.f.owner.Exec(clean, statement); err != nil {
				t.Error("bounded own clock barrier cleanup", err)
				return
			}
		}
	})
	// AFTER INSERT sees every real C2 effect before final settlement. A sequence
	// survives rollback and reveals only the backend PID after all sentinels pass.
	// No product clock, authority, timeout, trigger or domain writer is replaced.
	ddl := fmt.Sprintf(`CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
	 DECLARE a ops.order_attempts%%ROWTYPE; exact boolean;
	 BEGIN
	  SELECT * INTO a FROM ops.order_attempts WHERE attempt_id=(NEW.details->>'attempt_id')::uuid;
	  IF NOT FOUND OR a.state<>'PENDING' OR a.execute_by<>a.admitted_at+interval '15 minutes'
	   OR clock_timestamp()>=a.execute_by OR a.execute_by-clock_timestamp()>interval '3 seconds'
	   OR current_setting('statement_timeout')<>'5s' OR current_setting('lock_timeout')<>'1s'
	   OR current_setting('idle_in_transaction_session_timeout')<>'5s'
	   OR NOT EXISTS(SELECT 1 FROM ops.order_attempt_execution_fences f WHERE f.attempt_id=a.attempt_id
	    AND f.execution_xid=pg_current_xact_id()) THEN
	   RAISE EXCEPTION 'oa_deadline_audit_clock_or_fence_prerequisite'; END IF;
	  SELECT o.creator_kind='order_attempt' AND o.creator_session_id IS NULL AND o.create_attempt_id=a.attempt_id
	   AND o.owner_id=a.buyer_owner_id AND o.payment_mode='bank_transfer' AND o.commercial_state='AWAITING_TRANSFER'
	   AND r.create_attempt_id=a.attempt_id AND r.state='HELD' AND r.buyer_session_id IS NULL
	   AND p.create_attempt_id=a.attempt_id AND p.link_version=1 AND p.revoked_at IS NULL
	   AND j.kind='checkout_expiry_v1' AND j.args->>'order_id'=o.id::text AND j.state='scheduled'
	   AND (SELECT count(*)=1 FROM checkout.command_results c WHERE c.create_attempt_id=a.attempt_id)
	   AND (SELECT count(*)=1 FROM claims.order_origins c WHERE c.order_id=o.id)
	   AND (SELECT count(*)=1 FROM claims.live_price_uses u WHERE u.order_id=o.id AND u.quantity=2)
	   AND (SELECT count(*)=1 FROM inbox.order_for_buyer b WHERE b.create_attempt_id=a.attempt_id AND b.state='placed' AND b.order_id=o.id)
	   AND (SELECT count(*)=1 FROM claims.merchant_origin_grants g WHERE g.create_attempt_id=a.attempt_id
	    AND g.quote_id=o.quote_id AND g.consumed_at IS NOT NULL AND g.expires_at=a.execute_by)
	   AND (SELECT count(*)=1 FROM inbox.order_payment_notifications n WHERE n.create_attempt_id=a.attempt_id
	    AND n.order_id=o.id AND n.state='VOID' AND n.void_code='capability' AND n.void_stage='before_c3'
	    AND n.operation_id IS NULL AND n.outbound_id IS NULL)
	   AND NOT EXISTS(SELECT 1 FROM integration.operations i WHERE i.request->>'order_id'=o.id::text)
	   AND NOT EXISTS(SELECT 1 FROM river.river_job x WHERE x.kind='external_operation_v1' AND x.xmin=pg_current_xact_id()::xid)
	   AND (SELECT sum(l.delta_reserved)=2 FROM inventory.ledger l WHERE l.create_attempt_id=a.attempt_id AND l.kind='RESERVE')
	  INTO exact FROM checkout.orders o JOIN inventory.reservations r ON r.id=o.id
	   JOIN checkout.order_pay_entries p ON p.order_id=o.id JOIN river_expiry.river_job j ON j.id=o.job_id
	   WHERE o.id=a.attempt_id;
	  IF exact IS DISTINCT FROM true THEN RAISE EXCEPTION 'oa_deadline_audit_native_effect_prerequisite'; END IF;
	  PERFORM setval('%s'::regclass,pg_backend_pid(),true);
	  PERFORM pg_sleep(greatest(0,extract(epoch FROM (a.execute_by+interval '1 second'-clock_timestamp()))));
	  RETURN NEW;
	 END $$`, fn, seq)
	if _, err := e.p.f.owner.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	if _, err := e.p.f.owner.Exec(ctx, "REVOKE ALL ON FUNCTION "+fn+"() FROM PUBLIC"); err != nil {
		t.Fatal(err)
	}
	ddl = fmt.Sprintf("CREATE TRIGGER %s AFTER INSERT ON ops.audit_events FOR EACH ROW WHEN (NEW.action='order.manual_created' AND NEW.details->>'attempt_id'='%s') EXECUTE FUNCTION %s()", trigger, attempt, fn)
	if _, err := e.p.f.owner.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	return seq
}

func TestOrderAttemptDeadlineExecutionWinsBeforeClock(t *testing.T) {
	e := oaExpiryIsolated(t)
	a := oaFenceAdmit(t, e)
	plan, _ := oaMaterialPlan(t, e, oaServiceKeys(t), a.AttemptID)
	executor := oaExecutorNew(t, e.p.f.runtime, oaServiceKeys(t))
	got, err := executor.Execute(context.Background(), e.token(), e.store(), a.AttemptID, a.Generation)
	if err != nil {
		t.Fatal(err)
	}
	oaExecutorCommitted(t, e, a, plan, got, 5)
	var before bool
	if err = e.p.f.owner.QueryRow(context.Background(), `SELECT settled_at<execute_by AND clock_timestamp()<execute_by
	 AND execute_by=admitted_at+interval '15 minutes' FROM ops.order_attempts WHERE attempt_id=$1`, a.AttemptID).Scan(&before); err != nil || !before {
		t.Fatal("positive C2 commit before original SQL deadline", err)
	}
	pool := miPool(t, e.p.f, "commerce_claims_worker")
	result, err := oaExecutorNew(t, pool, oaServiceKeys(t)).Sweep(context.Background())
	if err != nil || result.Scanned != 0 || result.Requeued != 0 || result.Aborted != 0 {
		t.Fatal("sweep must not reconsider the winning terminal C2", result, err)
	}
	ca, err := executor.Execute(context.Background(), e.token(), e.store(), a.AttemptID, a.Generation)
	if err != nil {
		t.Fatal(err)
	}
	oaExecutorSameReceipt(t, got, ca)
}

func TestOrderAttemptDeadlineExecutionCrossesRealClock(t *testing.T) {
	// Sweep is cross-store discovery: reuse the existing isolated PG fixture so
	// exact counters cannot include another test's unfinished admission.
	e := oaExpiryIsolated(t)
	e.grantCreator("live:read", "live:manage", "pricing:read", "pricing:write", "catalog:read", "inbox:reply", "inbox:read", "integration:read", "integration:manage")
	labels, err := claims.NewLabelKey(randomBytes(32))
	if err != nil {
		t.Fatal(err)
	}
	e.h = &lcHarness{cqHarness: e.p.cqHarness, ctx: context.Background(), actor: e.p.f.principalA, token: e.token(), labels: labels}
	_, offer, bundle, conversation := e.scenario()
	mustExec(t, e.p.f.owner, `UPDATE inbox.conversation_state SET last_inbound_at=clock_timestamp() WHERE conversation_id=$1`, conversation)
	selection := oaClaimSelection(e, []string{bundle}, []string{offer.ID}, conversation)
	selection["send_payment_link"] = true
	a := oaAllocationAdmit(t, e, selection)
	oaMaterialPlan(t, e, oaServiceKeys(t), a.AttemptID)
	var deadline, now time.Time
	var exact bool
	err = e.p.f.owner.QueryRow(context.Background(), `SELECT a.execute_by,clock_timestamp(),
	 a.execute_by=a.admitted_at+interval '15 minutes'
	 AND (SELECT count(*)=1 FROM inbox.order_for_buyer b WHERE b.create_attempt_id=a.attempt_id AND b.bundle_id=$2
	  AND b.state='pending' AND b.order_id IS NULL AND b.expires_at=a.execute_by)
	 AND (SELECT count(*)=1 FROM claims.merchant_origin_grants g WHERE g.create_attempt_id=a.attempt_id AND g.bundle_id=$2
	  AND g.quote_id IS NULL AND g.consumed_at IS NULL AND g.expires_at=a.execute_by)
	 AND NOT EXISTS(SELECT 1 FROM integration.bindings WHERE tenant_id=a.tenant_id AND store_id=a.store_id AND provider IN ('facebook','instagram'))
	 FROM ops.order_attempts a WHERE a.attempt_id=$1`, a.AttemptID, bundle).Scan(&deadline, &now, &exact)
	if err != nil || !exact || deadline.Sub(now) < 14*time.Minute {
		t.Fatal("real C1 fifteen-minute native custody/no-binding prerequisite", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), deadline.Sub(now)+90*time.Second)
	defer cancel()
	t.Log("genuine deadline C2 race: expected fifteen minutes; no authority deadline or existing executor budget is changed")
	seq := oaDeadlineAuditBarrier(t, e, a.AttemptID)
	executor := oaExecutorNew(t, e.p.f.runtime, oaServiceKeys(t))
	pool := miPool(t, e.p.f, "commerce_claims_worker")
	sweeper := oaExecutorNew(t, pool, oaServiceKeys(t))
	// Every waiting query is bounded; the actual SQL clock decides when to start.
	for {
		probe, stop := context.WithTimeout(ctx, 3*time.Second)
		err = e.p.f.owner.QueryRow(probe, `SELECT clock_timestamp()`).Scan(&now)
		stop()
		if err != nil {
			t.Fatal(err)
		}
		remaining := deadline.Sub(now) - 2*time.Second
		if remaining <= 0 {
			if deadline.Sub(now) < time.Second {
				t.Fatal("CI missed the bounded predeadline execution window")
			}
			break
		}
		if remaining > time.Second {
			remaining = time.Second
		}
		timer := time.NewTimer(remaining)
		select {
		case <-ctx.Done():
			timer.Stop()
			t.Fatal("genuine SQL deadline observation timed out")
		case <-timer.C:
		}
	}
	type result struct {
		receipt orderattempt.ExecutionReceipt
		err     error
	}
	done := make(chan result, 1)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		got, err := executor.Execute(ctx, e.token(), e.store(), a.AttemptID, a.Generation)
		done <- result{got, err}
	}()
	defer func() {
		cancel()
		select {
		case <-finished:
		case <-time.After(6 * time.Second):
			t.Error("bounded deadline executor failed to stop")
		}
	}()
	var pid int64
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		var called, sleeping, expired bool
		if err = e.p.f.owner.QueryRow(ctx, `SELECT s.last_value,s.is_called,
		 EXISTS(SELECT 1 FROM pg_stat_activity p WHERE p.pid=s.last_value AND p.state='active' AND p.wait_event='PgSleep'),
		 clock_timestamp()>=$1::timestamptz FROM `+seq+` s`, deadline).Scan(&pid, &called, &sleeping, &expired); err != nil {
			t.Fatal(err)
		}
		if called && sleeping && expired {
			break
		}
		select {
		case got := <-done:
			t.Fatal("C2 never reached the positive native audit barrier across the deadline", got.err)
		case <-ctx.Done():
			t.Fatal("late native audit barrier was not observed")
		case <-tick.C:
		}
	}
	// The real worker-only Sweep sees an expired PENDING row, but its first
	// SKIP LOCKED attempt fence must skip the executing C2 without domain work.
	sweepCtx, stopSweep := context.WithTimeout(ctx, 500*time.Millisecond)
	swept, sweepErr := sweeper.Sweep(sweepCtx)
	stopSweep()
	if sweepErr != nil || swept.Scanned != 1 || swept.Requeued != 0 || swept.Aborted != 0 {
		t.Fatal("real expired sweep did not skip the held C2 attempt", swept, sweepErr)
	}
	if err = e.p.f.owner.QueryRow(ctx, `SELECT a.state='PENDING' AND a.execute_by=$2 AND clock_timestamp()>=a.execute_by
	 AND EXISTS(SELECT 1 FROM pg_stat_activity p WHERE p.pid=$3 AND p.state='active' AND p.wait_event='PgSleep')
	 FROM ops.order_attempts a WHERE a.attempt_id=$1`, a.AttemptID, deadline, pid).Scan(&exact); err != nil || !exact {
		t.Fatal("sweep returned without preserving the observed expired active C2 barrier", err)
	}
	var got result
	select {
	case got = <-done:
	case <-ctx.Done():
		t.Fatal("C2 failed to settle after real clock barrier")
	}
	if got.err != nil {
		t.Fatal("late C2 must roll back and seal EXPIRED within its original five-second budget", got.err)
	}
	oaExecutorAborted(t, e, a, got.receipt, "EXPIRED")
	if err = e.p.f.owner.QueryRow(ctx, `SELECT a.execute_by=$2 AND a.settled_at>=a.execute_by
	 AND (SELECT count(*)=1 FROM inbox.order_for_buyer b WHERE b.create_attempt_id=a.attempt_id
	  AND b.bundle_id=$3 AND b.state='released' AND b.order_id IS NULL AND b.expires_at=a.execute_by)
	 AND (SELECT count(*)=1 FROM claims.merchant_origin_grants g WHERE g.create_attempt_id=a.attempt_id
	  AND g.bundle_id=$3 AND g.quote_id IS NULL AND g.consumed_at IS NULL AND g.expires_at=a.execute_by)
	 FROM ops.order_attempts a WHERE a.attempt_id=$1`, a.AttemptID, deadline, bundle).Scan(&exact); err != nil || !exact {
		t.Fatal("cross-deadline rollback failed native custody/deadline restoration", err)
	}
}
