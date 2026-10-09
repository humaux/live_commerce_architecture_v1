// Purpose: prove paid A6 catalogue drift creates durable review instead of a failed reconcile with no evidence.
// Depends on: real catalogue edits, checkout/Stripe fixture, CaptureWorker and payment worker runtime, REAL_PG.
// Used by: Stripe SP/SL/refund foundation gates; all provider traffic stays in the MOCK transport.
// Invariants: recorded paid facts never imply fulfillment eligibility; replay and later catalogue repair cannot settle.
package foundation_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	"github.com/riverqueue/river"
	"livecommerce/internal/catalog"
	"livecommerce/internal/payments"
)

// sucWork decodes a persisted job payload into the actual worker's argument type without a stand-in worker.
func sucWork[T river.JobArgs](t *testing.T, worker river.Worker[T], payload any) error {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	var args T
	if err = json.Unmarshal(raw, &args); err != nil {
		t.Fatal(err)
	}
	return worker.Work(context.Background(), &river.Job[T]{Args: args})
}

func TestStripeUntrackedDriftReview(t *testing.T) {
	for _, drift := range []string{"became_tracked", "sku_deleted"} {
		t.Run(drift, func(t *testing.T) {
			e, attempt, sku := sucOrder(t, false, pwIsolatedFixture(t))
			order := e.p.hold.OrderID
			var product string
			var version int64
			if err := e.f.owner.QueryRow(context.Background(), `SELECT p.id::text,p.version FROM catalog.products p JOIN catalog.skus s ON s.product_id=p.id AND s.tenant_id=p.tenant_id AND s.store_id=p.store_id WHERE s.id=$1`, sku).Scan(&product, &version); err != nil {
				t.Fatal(err)
			}
			edit := func(mode string) {
				t.Helper()
				stock := &catalog.DocumentStock{Mode: mode}
				if mode == "untracked" {
					cap := int64(3)
					stock.MaxPerOrder = &cap
				}
				doc, err := k3Edit(t, e.f, t04Key("suc-drift"), product, catalog.ProductDocumentPatch{ExpectedVersion: version, SKUs: &[]catalog.DocumentSKUPatch{{ID: sku, Stock: stock}}})
				if err != nil {
					t.Fatal(err)
				}
				version = doc.Version
			}
			if drift == "became_tracked" {
				edit("tracked")
			} else {
				// Controlled owner deletion represents a historical SKU missing when the payment reconciles.
				slrReplica(t, e.f, `DELETE FROM catalog.skus WHERE id=$1`, sku)
			}
			if n := countRows(t, e.f.owner, `SELECT count(*) FROM inventory.reservation_lines WHERE reservation_id=$1`, order); n != 0 {
				t.Fatal("fixture did not begin with an empty stock plan")
			}
			hash := e.observe(t, attempt, e.sessionReport(t, attempt, "complete", "paid", "succeeded", false))
			worker, err := payments.NewCaptureWorker(context.Background(), e.p.worker)
			if err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 3; i++ {
				if i == 2 && drift == "became_tracked" {
					edit("untracked")
				}
				// nil proves that the real reconcile worker commits and completes, instead of cancel/retry with rolled-back evidence.
				if err = sucWork(t, worker, pcArgs{OperationID: attempt, ReportHash: hex.EncodeToString(hash), Version: 1}); err != nil {
					t.Fatalf("durable paid review/replay %d: %v", i, err)
				}
				if n := countRows(t, e.f.owner, `SELECT count(*) FROM payments.facts f JOIN checkout.payment_attempts a ON a.id=f.attempt_id WHERE f.attempt_id=$1 AND f.kind='CAPTURED' AND f.amount_minor=a.amount_minor AND f.currency=a.currency AND f.source_report_hash=$2`, attempt, hash); n != 1 {
					t.Fatalf("exact captured money facts=%d", n)
				}
				if n := countRows(t, e.f.owner, `SELECT count(*) FROM payments.review_cases WHERE attempt_id=$1 AND reason='PAID_ALLOCATION_FAILED' AND source_report_hash=$2`, attempt, hash); n != 1 {
					t.Fatalf("durable review cases=%d", n)
				}
				if n := countRows(t, e.f.owner, `SELECT count(*) FROM fulfillment.payment_work_items WHERE order_id=$1 AND attempt_id=$2 AND state='REVIEW_REQUIRED'`, order, attempt); n != 1 {
					t.Fatalf("review work items=%d", n)
				}
				var state, reservation string
				if err = e.f.owner.QueryRow(context.Background(), `SELECT o.commercial_state,r.state FROM checkout.orders o JOIN inventory.reservations r ON r.id=o.id WHERE o.id=$1`, order).Scan(&state, &reservation); err != nil || state != "AWAITING_PAYMENT" || reservation != "PAYMENT_PENDING" {
					t.Fatalf("unsafe settlement: %s/%s err=%v", state, reservation, err)
				}
				if n := countRows(t, e.f.owner, `SELECT count(*) FROM inventory.ledger WHERE checkout_id=$1 AND kind IN ('ALLOCATE','RELEASE')`, order); n != 0 {
					t.Fatalf("review moved stock: %d", n)
				}
				if n := countRows(t, e.f.owner, `SELECT count(*) FROM checkout.events WHERE order_id=$1 AND action IN ('checkout.payment_captured','checkout.payment_closed')`, order); n != 0 {
					t.Fatalf("review emitted settlement event: %d", n)
				}
				view, err := e.svc.PaymentView(context.Background(), e.p.cap.Token, e.p.f.storeA1, order)
				if err != nil || view.PaymentState != "REVIEW_REQUIRED" {
					t.Fatalf("payment readback hides review: state=%s err=%v", view.PaymentState, err)
				}
			}
			// Run the real process assembly: the remaining query job must finish from the captured flag without any new HTTP.
			flow := &sflEnv{sstEnv: e.sstEnv, pool: e.p.worker, opts: payments.DefaultQueryWorkerOptions()}
			flow.opts.MockTransport = pqNoNetwork()
			requests := len(e.fake.Requests())
			stop := flow.start(t, true)
			defer stop()
			flow.await(t, "terminal query after durable review", attempt, 20*time.Second, `SELECT j.state='completed' AND o.result_code='stripe_terminal_observed' FROM checkout.payment_attempts a JOIN river_payment.river_job j ON j.id=a.job_id JOIN integration.operations o ON o.id=a.id WHERE a.id=$1`)
			if n := len(e.fake.Requests()); n != requests {
				t.Fatalf("terminal review made new provider calls: %d -> %d", requests, n)
			}
		})
	}
}
