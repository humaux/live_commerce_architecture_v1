// Purpose: PR20 per-line reservation and catalogue-drift counterexamples for paid and unpaid Stripe observations.
// Depends on: real Begin/catalogue/Stripe capture/refund HTTP commands, immutable quote and initial RESERVE ledger, REAL_PG.
// Used by: Stripe focused/foundation gates; MOCK provider only, no live money or buyer PII.
package foundation_test

import (
	"context"
	"encoding/hex"
	"testing"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"livecommerce/internal/catalog"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/payments"
)

// sucDrift changes the catalogue through its actual document command, or explicitly simulates a historical missing SKU.
func sucDrift(t *testing.T, e *slrEnv, sku, drift string) {
	t.Helper()
	if drift == "deleted" {
		slrReplica(t, e.f, `DELETE FROM catalog.skus WHERE id=$1`, sku)
		return
	}
	var product string
	var version int64
	if err := e.f.owner.QueryRow(context.Background(), `SELECT p.id::text,p.version FROM catalog.products p JOIN catalog.skus s ON s.product_id=p.id AND s.store_id=p.store_id AND s.tenant_id=p.tenant_id WHERE s.id=$1`, sku).Scan(&product, &version); err != nil {
		t.Fatal(err)
	}
	_, err := k3Edit(t, e.f, t04Key("suc-r1-drift"), product, catalog.ProductDocumentPatch{ExpectedVersion: version, SKUs: &[]catalog.DocumentSKUPatch{{ID: sku, Stock: &catalog.DocumentStock{Mode: "tracked"}}}})
	if err != nil {
		t.Fatal(err)
	}
}
func sucReviewState(t *testing.T, e *slrEnv, attempt string) {
	t.Helper()
	var commercial, fulfillment, reservation, work string
	err := e.f.owner.QueryRow(context.Background(), `SELECT o.commercial_state,o.fulfillment_state,r.state,w.state FROM checkout.orders o JOIN inventory.reservations r ON r.id=o.id JOIN fulfillment.payment_work_items w ON w.order_id=o.id WHERE o.id=$1`, e.p.hold.OrderID).Scan(&commercial, &fulfillment, &reservation, &work)
	if err != nil || commercial != "AWAITING_PAYMENT" || fulfillment != "PAID_ALLOCATION_FAILED" || reservation != "PAYMENT_PENDING" || work != "REVIEW_REQUIRED" {
		t.Fatalf("allocation review states=%s/%s/%s/%s err=%v", commercial, fulfillment, reservation, work, err)
	}
	if countRows(t, e.f.owner, `SELECT count(*) FROM payments.facts WHERE attempt_id=$1 AND kind='CAPTURED'`, attempt) != 1 {
		t.Fatal("capture fact missing/duplicated")
	}
	if countRows(t, e.f.owner, `SELECT count(*) FROM payments.review_cases WHERE attempt_id=$1 AND reason='PAID_ALLOCATION_FAILED'`, attempt) != 1 {
		t.Fatal("allocation review missing/duplicated")
	}
	if countRows(t, e.f.owner, `SELECT count(*) FROM inventory.ledger WHERE checkout_id=$1 AND kind IN ('ALLOCATE','RELEASE')`, e.p.hold.OrderID) != 0 {
		t.Fatal("review moved any tracked stock")
	}
	if countRows(t, e.f.owner, `SELECT count(*) FROM checkout.events WHERE order_id=$1 AND action IN ('checkout.payment_captured','checkout.payment_closed')`, e.p.hold.OrderID) != 0 {
		t.Fatal("review emitted settlement event")
	}
}
func TestStripeUntrackedMixedDrift(t *testing.T) {
	for _, drift := range []string{"tracked", "deleted"} {
		t.Run(drift, func(t *testing.T) {
			e, attempt, sku := sucOrder(t, true)
			sucDrift(t, e, sku, drift)
			hash := e.observe(t, attempt, e.sessionReport(t, attempt, "complete", "paid", "succeeded", false))
			for i := 0; i < 2; i++ {
				if err := e.apply(attempt, hash); err != nil {
					t.Fatal(err)
				}
				sucReviewState(t, e, attempt)
			}
		})
	}
}
func TestStripeUntrackedFulfillmentState(t *testing.T) {
	e, attempt, sku := sucOrder(t, false)
	sucDrift(t, e, sku, "tracked")
	hash := e.observe(t, attempt, e.sessionReport(t, attempt, "complete", "paid", "succeeded", false))
	if err := e.apply(attempt, hash); err != nil {
		t.Fatal(err)
	}
	sucReviewState(t, e, attempt)
}
func TestStripeUntrackedCloseAfterDrift(t *testing.T) {
	for _, mixed := range []bool{false, true} {
		for _, drift := range []string{"tracked", "deleted"} {
			name := drift
			if mixed {
				name = "mixed_" + name
			}
			t.Run(name, func(t *testing.T) {
				e, attempt, sku := sucOrder(t, mixed, pwIsolatedFixture(t))
				sucDrift(t, e, sku, drift)
				hash := e.observe(t, attempt, e.sessionReport(t, attempt, "expired", "unpaid", "canceled", false))
				worker, err := payments.NewCaptureWorker(context.Background(), e.p.worker)
				if err != nil {
					t.Fatal(err)
				}
				for i := 0; i < 2; i++ {
					if err = sucWork(t, worker, pcArgs{OperationID: attempt, ReportHash: hex.EncodeToString(hash), Version: 1}); err != nil {
						t.Fatalf("close reconcile did not complete: %v", err)
					}
				}
				if countRows(t, e.f.owner, `SELECT count(*) FROM payments.facts WHERE attempt_id=$1 AND kind='CLOSED_UNPAID'`, attempt) != 1 {
					t.Fatal("close fact missing/duplicated")
				}
				var commercial, fulfillment, reservation string
				if err = e.f.owner.QueryRow(context.Background(), `SELECT o.commercial_state,o.fulfillment_state,r.state FROM checkout.orders o JOIN inventory.reservations r ON r.id=o.id WHERE o.id=$1`, e.p.hold.OrderID).Scan(&commercial, &fulfillment, &reservation); err != nil || commercial != "CANCELLED" || fulfillment != "CANCELLED" || reservation != "RELEASED" {
					t.Fatalf("close states=%s/%s/%s err=%v", commercial, fulfillment, reservation, err)
				}
				want := 0
				if mixed {
					want = 1
				}
				if countRows(t, e.f.owner, `SELECT count(*) FROM inventory.ledger WHERE checkout_id=$1 AND kind='RELEASE'`, e.p.hold.OrderID) != want {
					t.Fatal("close must release only original tracked stock")
				}
				flow := &sflEnv{sstEnv: e.sstEnv, pool: e.p.worker, opts: payments.DefaultQueryWorkerOptions()}
				flow.opts.MockTransport = pqNoNetwork()
				requests := len(e.fake.Requests())
				stop := flow.start(t, true)
				defer stop()
				flow.await(t, "closed drift query finishes", attempt, 20*time.Second, `SELECT j.state='completed' AND o.result_code='stripe_terminal_observed' FROM checkout.payment_attempts a JOIN river_payment.river_job j ON j.id=a.job_id JOIN integration.operations o ON o.id=a.id WHERE a.id=$1`)
				if len(e.fake.Requests()) != requests {
					t.Fatal("terminal closed order called provider again")
				}
			})
		}
	}
}
func TestStripeUntrackedPresentmentDriftRefund(t *testing.T) {
	for _, mixed := range []bool{false, true} {
		name := "all_untracked"
		if mixed {
			name = "mixed"
		}
		t.Run(name, func(t *testing.T) {
			e, attempt, sku := sucOrder(t, mixed)
			sucDrift(t, e, sku, "tracked")
			report := e.sessionReport(t, attempt, "complete", "paid", "succeeded", false)
			report["PresentmentCurrency"] = "USD"
			report["CurrencyConversion"] = true
			hash := e.observe(t, attempt, report)
			if err := e.apply(attempt, hash); err != nil {
				t.Fatal(err)
			}
			if countRows(t, e.f.owner, `SELECT count(*) FROM payments.review_cases WHERE attempt_id=$1 AND reason IN ('PROVIDER_PRESENTMENT_DRIFT','PAID_ALLOCATION_FAILED')`, attempt) != 2 {
				t.Fatal("both reviews must coexist before an early return")
			}
			sucReviewState(t, e, attempt)
			jobs, err := river.NewClient(riverpgxv5.New(e.f.runtime), &river.Config{Schema: "river_payment"})
			if err != nil {
				t.Fatal(err)
			}
			r := &rfxEnv{sflEnv: &sflEnv{sstEnv: e.sstEnv}, jobs: jobs, handler: httpapi.NewHandler(e.f.runtime, httpapi.Options{RefundJobs: jobs})}
			o := rfxOrder{s: sstStore{p: e.p}, attempt: attempt, order: e.p.hold.OrderID}
			r.grant(t, o, "orders:read", "payments:refund")
			var captured int64
			if err = e.f.owner.QueryRow(context.Background(), `SELECT amount_minor FROM checkout.payment_attempts WHERE id=$1`, attempt).Scan(&captured); err != nil {
				t.Fatal(err)
			}
			status, out := r.request(o, o.token(), t04Key("suc-r1-partial"), rfxBody(100, "requested_by_customer", captured))
			if status != 422 || srqCode(out) != "refund_blocked_review" {
				t.Fatalf("partial refund=%d/%v", status, out)
			}
			status, out = r.request(o, o.token(), t04Key("suc-r1-full"), rfxBody(captured, "requested_by_customer", captured))
			if status != 201 {
				t.Fatalf("full-remaining refund=%d/%v", status, out)
			}
			if err = e.apply(attempt, hash); err != nil {
				t.Fatal(err)
			}
			sucReviewState(t, e, attempt)
		})
	}
}

// §4.4 applies to later reports even when the catalogue has changed since a successful capture.
func TestStripeUntrackedPostCaptureDriftImmutable(t *testing.T) {
	e, attempt, sku := sucOrder(t, false)
	report := e.sessionReport(t, attempt, "complete", "paid", "succeeded", false)
	hash := e.observe(t, attempt, report)
	if err := e.apply(attempt, hash); err != nil {
		t.Fatal(err)
	}
	read := func() string {
		t.Helper()
		var state string
		if err := e.f.owner.QueryRow(context.Background(), `SELECT jsonb_build_array(o.commercial_state,o.fulfillment_state,o.updated_at,r.state,w.state)::text FROM checkout.orders o JOIN inventory.reservations r ON r.id=o.id JOIN fulfillment.payment_work_items w ON w.order_id=o.id WHERE o.id=$1`, e.p.hold.OrderID).Scan(&state); err != nil {
			t.Fatal(err)
		}
		return state
	}
	before := read()
	sucDrift(t, e, sku, "tracked")
	report["PresentmentCurrency"] = "USD"
	report["CurrencyConversion"] = true
	hash = e.observe(t, attempt, report)
	if err := e.apply(attempt, hash); err != nil {
		t.Fatal(err)
	}
	if after := read(); after != before {
		t.Fatalf("later review rewrote settled aggregate: %s -> %s", before, after)
	}
	if countRows(t, e.f.owner, `SELECT count(*) FROM payments.review_cases WHERE attempt_id=$1 AND reason='PROVIDER_PRESENTMENT_DRIFT'`, attempt) != 1 {
		t.Fatal("late presentment evidence missing")
	}
	if countRows(t, e.f.owner, `SELECT count(*) FROM payments.review_cases WHERE attempt_id=$1 AND reason='PAID_ALLOCATION_FAILED'`, attempt) != 0 {
		t.Fatal("later catalogue change retroactively failed a settled allocation")
	}
}
