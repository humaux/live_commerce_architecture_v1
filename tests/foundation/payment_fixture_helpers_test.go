package foundation_test

// Purpose: paid-order fixtures for read/fulfilment suites, using the existing Stripe
// fake, signed webhook and production worker; never dispatches PAYUNi. Workers stop
// before returning so read-only assertions have a stable baseline.
// Depends on: sfl/sst fixtures (stripe_flow_test.go, stripe_start_test.go), River schema river_payment.
// Used by: payment capture/query, merchant order, parcel, expiry, CAPI and browser suites in this package.
// Invariant: after rm2StripePending the query job is River state 'available' with scheduled_at
// about 5s ahead (create snooze <= River's 5s scheduler interval), so sstWake does not touch it;
// use rm2WakeQuery to make that exact job due.

import (
	"context"
	"testing"
)

func rm2StripePending(t *testing.T) pqFixture {
	t.Helper()
	return rm2StripePendingOn(t, pwIsolatedFixture(t))
}

func rm2StripePendingOn(t *testing.T, f *testFixture) pqFixture {
	t.Helper()
	e := sflNewOn(t, f, nil)
	s := e.stripeStore(t)
	q := pqFixture{psHarness: s.p, schema: "river_payment", stripe: e, stripeStore: s}
	q.rm2StripeStart(t)
	return q
}

func (q *pqFixture) rm2StripeStart(t *testing.T) {
	t.Helper()
	if q.stripe == nil {
		t.Fatal("Stripe fixture not initialized")
	}
	s := q.stripeStore
	s.p = q.psHarness
	stop := q.stripe.start(t, true)
	defer stop()
	q.result, _ = q.stripe.pinned(t, s)
}

// rm2WakeQuery makes the attempt's own payment_query_v1 job due now. River stores a snooze no
// longer than its scheduler interval as 'available' with a future scheduled_at, which
// sstWake (scheduled/retryable only) leaves sleeping.
func (q pqFixture) rm2WakeQuery(t *testing.T) {
	t.Helper()
	tag, err := q.f.owner.Exec(context.Background(), `UPDATE river_payment.river_job SET scheduled_at=clock_timestamp()
	 WHERE id=$1 AND kind='payment_query_v1' AND args->>'operation_id'=$2
	 AND state IN ('available','scheduled','retryable')`, q.result.JobID, q.result.AttemptID)
	if err != nil || tag.RowsAffected() != 1 {
		t.Fatalf("Stripe query job not wakeable: rows=%d err=%v", tag.RowsAffected(), err)
	}
}

func (q pqFixture) rm2StripePay(t *testing.T) {
	t.Helper()
	if q.stripe == nil {
		t.Fatal("Stripe fixture not initialized")
	}
	session := q.stripe.session(t, q.result.AttemptID)
	if !q.stripe.fake.SetState(session, "complete", "paid") {
		t.Fatal("fake Stripe session missing")
	}
	endpoint, secret := q.stripe.paymentEndpoint(t, q.stripeStore)
	if status := q.stripe.deliver(t, endpoint, secret, sflEvent(q.result.AttemptID, session)); status != 200 {
		t.Fatalf("Stripe signed webhook status=%d", status)
	}
	stop := q.stripe.start(t, true)
	defer stop()
	q.stripe.awaitFact(t, q.result.AttemptID, "CAPTURED")
}

// paymentEndpoint registers the store's webhook endpoint once and reuses it: a second endpoint for the
// same connection violates the (tenant, store, connection, profile) unique index (23505), which the
// registrar reports as stripeadmin: rejected.
func (e *sflEnv) paymentEndpoint(t *testing.T, s sstStore) (string, string) {
	t.Helper()
	if h, ok := e.hooks[s.connection]; ok {
		return h[0], h[1]
	}
	id, secret := e.endpoint(t, s)
	if e.hooks == nil {
		e.hooks = map[string][2]string{}
	}
	e.hooks[s.connection] = [2]string{id, secret}
	return id, secret
}
