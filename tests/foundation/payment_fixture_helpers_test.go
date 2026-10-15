package foundation_test

// Responsibility: paid-order fixtures for read/fulfilment suites, using the existing Stripe
// fake, signed webhook and production worker. Depends on sfl/sst fixtures; never dispatches
// PAYUNi. Workers stop before returning so read-only assertions have a stable baseline.

import "testing"

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

func (q pqFixture) rm2StripePay(t *testing.T) {
	t.Helper()
	if q.stripe == nil {
		t.Fatal("Stripe fixture not initialized")
	}
	session := q.stripe.session(t, q.result.AttemptID)
	if !q.stripe.fake.SetState(session, "complete", "paid") {
		t.Fatal("fake Stripe session missing")
	}
	endpoint, secret := q.stripe.endpoint(t, q.stripeStore)
	if status := q.stripe.deliver(t, endpoint, secret, sflEvent(q.result.AttemptID, session)); status != 200 {
		t.Fatalf("Stripe signed webhook status=%d", status)
	}
	stop := q.stripe.start(t, true)
	defer stop()
	q.stripe.awaitFact(t, q.result.AttemptID, "CAPTURED")
}
