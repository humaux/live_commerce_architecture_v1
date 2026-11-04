// Purpose: SP11/#468 regression proof for the bounded-poll wake helper (sstWake) shared by the
// Stripe foundation gates.
// Depends on: sflNew/sflEnv.stripeStore/attempt/sflSignal (stripe_flow_test.go, stripe_start_test.go),
// the fixture owner pool, and River v0.40.0's two persisted snooze shapes.
// Used by: the payments gate shards that run TestStripeSP11BusySnoozeIsWakeable; no worker process
// is assembled, so the fixture is a pure owner-side row shape.
// Invariants: sstWake makes every representation of a sleeping payment job due. A bounded poll must
// never re-arm one payment job kind while leaving another asleep in River's short-snooze shape,
// because that asymmetry starves the busy-lease signal job behind the force-woken poller.

package foundation_test

import (
	"context"
	"testing"
)

// TestStripeSP11BusySnoozeIsWakeable pins the harness invariant behind #468 without a worker and
// without a sleep:
//
//   - river.JobSnooze(2 s) — the busy-lease path (internal/payments/stripe_signal.go:115-117) — is
//     persisted by River as state='available' with a future scheduled_at
//     (riverdriver.JobSetStateSnoozedAvailable; 2 s <= the default 5 s scheduler interval);
//   - a successful poll snoozes stripePollDelay (1 min for an open session) and is persisted as
//     state='scheduled'.
//
// sflEnv.await re-arms sleeping payment jobs every 400 ms through sstWake. Before the fix sstWake
// rewrote only 'scheduled'/'retryable', so the 1-minute poller was force-woken on every round while
// the 2 s busy signal job waited for River: the poller re-claimed the operation (generation 42/43 in
// the captured failures) faster than the signal could retry, and the signal starved until the 40 s
// OBSERVED deadline.
func TestStripeSP11BusySnoozeIsWakeable(t *testing.T) {
	e := sflNew(t, nil)
	ctx := context.Background()
	s := e.stripeStore(t)
	res := e.attempt(t, s)
	_, signalJob := e.sflSignal(t, s, res.AttemptID, "BUYER_REFRESH", 0)

	var queryJob int64
	if err := e.f.owner.QueryRow(ctx, `SELECT id FROM river_payment.river_job
	 WHERE kind='payment_query_v1' AND args->>'operation_id'=$1`, res.AttemptID).Scan(&queryJob); err != nil {
		t.Fatalf("query job for the attempt: %v", err)
	}

	// Write each snooze shape exactly as River persists it, instead of driving a worker and sleeping:
	// the short (2 s) busy snooze is 'available' + future scheduled_at and decrements attempt by one;
	// the long (60 s) poll snooze is 'scheduled'.
	mustExec(t, e.f.owner, `UPDATE river_payment.river_job
	 SET state='available',scheduled_at=clock_timestamp()+interval '2 seconds',attempt=greatest(attempt-1,0)
	 WHERE id=$1`, signalJob)
	mustExec(t, e.f.owner, `UPDATE river_payment.river_job
	 SET state='scheduled',scheduled_at=clock_timestamp()+interval '60 seconds'
	 WHERE id=$1`, queryJob)

	sstWake(t, e.f, res.AttemptID)

	due := func(job int64) bool {
		var ok bool
		if err := e.f.owner.QueryRow(ctx, `SELECT state='available' AND scheduled_at<=clock_timestamp()
		 FROM river_payment.river_job WHERE id=$1`, job).Scan(&ok); err != nil {
			t.Fatalf("read job %d: %v", job, err)
		}
		return ok
	}
	if !due(queryJob) {
		t.Fatal("wake helper stopped re-arming the scheduled Stripe poller")
	}
	if !due(signalJob) {
		t.Fatal("wake helper left the busy-lease signal job in River's short-snooze shape " +
			"(available + future scheduled_at): the force-woken poller re-claims every 400 ms round " +
			"while the signal retries only every 2 s, so the signal starves until the OBSERVED deadline (#468)")
	}
}
