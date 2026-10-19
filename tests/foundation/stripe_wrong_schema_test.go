package foundation_test

// Purpose: prove the live Go payment producer (Stripe start, internal/checkout/stripe.go beginStripe ->
// jobs.InsertTx) fails closed when its River client is miswired to the expiry family schema, and writes
// nothing to business facts or any River lane. Replaces the retired PAYUNi starter's wrong-schema case
// (review output/pay-rm2-p2a/review-0617d49ef.md, NOT_COVERED P2).
// Depends on: pwIsolatedFixture (own PG container, so whole-lane snapshots are exact), psSetupItemsOn,
// sstNewEnv/seed/begin/sstFacts (stripe_start_test.go), miIsoRows (meta_isolation_test.go), the
// river_expiry family guard (migrations post_river 0005 checkout.guard_expiry_job_family).
// Used by: the default foundation shards (no build tag).
// Invariant: the same store, order and key then succeed on the correctly wired service, so the refusal
// is caused by the schema alone, never by fixture state.

import (
	"context"
	"testing"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"livecommerce/internal/checkout"
)

func TestStripeStartWrongRiverSchemaFailsClosed(t *testing.T) {
	p := psSetupItemsOn(t, pwIsolatedFixture(t), 1)
	e := sstNewEnv(t, p.f, sstKeyring(t, "sst_api", randomBytes(32)))
	s := e.seed(t, p)
	lanes := func() [3]string {
		return [3]string{miIsoRows(t, p.f, "river_payment.river_job", ""), miIsoRows(t, p.f, "river_expiry.river_job", ""), miIsoRows(t, p.f, "river.river_job", "")}
	}
	jobs, err := river.NewClient(riverpgxv5.New(e.hosted), &river.Config{Schema: "river_expiry"})
	if err != nil {
		t.Fatal(err)
	}
	miswired, err := checkout.NewHostedPaymentService(context.Background(), e.hosted, jobs, "PROVIDER_MOCK", e.keys,
		checkout.HostedProviders{PAYUNi: &e.pcfg, Stripe: &e.scfg})
	if err != nil {
		t.Fatal(err)
	}
	key, in := t04Key("sst-wrong-schema"), s.input("zh-TW")
	beforeFacts, beforeLanes := sstFacts(t, p), lanes()
	// The redacted database error, not a PG detail (SQLSTATE, schema or job kind) and not success.
	if out, err := s.begin(miswired, key, in); err == nil || err.Error() != "checkout database unavailable" || out != (checkout.PaymentResult{}) {
		t.Fatalf("Stripe start with river_expiry client did not fail closed: out=%+v err=%v", out, err)
	}
	if got := sstFacts(t, p); got != beforeFacts {
		t.Fatalf("miswired Stripe start changed facts %v -> %v", beforeFacts, got)
	}
	if got := lanes(); got != beforeLanes {
		t.Fatal("miswired Stripe start changed a River lane")
	}
	// Control: the rolled-back attempt left no receipt, so the same key on the production wiring
	// (river_payment) starts once and its job lands in the payment lane only.
	out, err := s.begin(e.svc, key, in)
	if err != nil || out.AttemptID == "" || out.JobID < 1 {
		t.Fatalf("correctly wired Stripe start after refusal: out=%+v err=%v", out, err)
	}
	after := lanes()
	if after[0] == beforeLanes[0] || after[1] != beforeLanes[1] || after[2] != beforeLanes[2] {
		t.Fatal("correct start did not enqueue exactly in river_payment")
	}
	if n := countRows(t, p.f.owner, `SELECT count(*) FROM river_payment.river_job WHERE id=$1 AND kind='payment_query_v1'`, out.JobID); n != 1 {
		t.Fatalf("correct start job %d not in river_payment as payment_query_v1 (rows=%d)", out.JobID, n)
	}
}
