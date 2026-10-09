// Purpose: REAL_PG regression for Stripe capture/close of A6 untracked inventory orders.
// Depends on: real catalog/checkout/Stripe registrar commands, MOCK observations, payment-worker apply_capture.
// Used by: focused Stripe gates and foundation; no provider network, live keys or buyer PII.
// Invariants: paid facts are durable and replay-safe; empty/forged tracked reservations fail atomically.
package foundation_test

import (
	"context"
	"testing"

	"livecommerce/internal/catalog"
	"livecommerce/internal/storefront"
)

// sucOrder creates its order through real Begin (never by deleting a valid hold's stock lines).
// Only provider session pins/observations and explicitly named corruption cases use the owner pool.
func sucOrder(t *testing.T, tracked bool, isolated ...*testFixture) (*slrEnv, string, string) {
	t.Helper()
	base := fixture(t)
	if len(isolated) != 0 {
		base = isolated[0]
	}
	p := psSetupItemsOn(t, base, 1)
	cap := int64(3)
	doc, err := k3Doc(t, p.f, t04Key("suc-product"), catalog.ProductDocumentInput{
		Name: "Synthetic untracked", Status: catalog.StatusActive,
		SKUs: []catalog.DocumentSKUInput{{PriceMinor: 2500, Stock: &catalog.DocumentStock{Mode: "untracked", MaxPerOrder: &cap}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	items := []storefront.Item{{SKUID: doc.SKUs[0].ID, Quantity: 2}}
	if tracked {
		items = append(items, storefront.Item{SKUID: p.stock.skus[0].ID, Quantity: 2})
	}
	p.bcHarness.prepare(t, mustIssue(t, p.cqHarness.service, p.f.storeA1), items)
	p.hold, err = p.bcHarness.begin(t04Key("suc-hold"))
	if err != nil {
		t.Fatal(err)
	}
	e := sstNewEnv(t, p.f, pwKeys(t))
	s := e.seed(t, p)
	result, err := s.begin(e.svc, t04Key("suc-payment"), s.input("zh-TW"))
	if err != nil {
		t.Fatal(err)
	}
	env := &slrEnv{sstEnv: e, p: p}
	env.pin(t, result.AttemptID)
	return env, result.AttemptID, doc.SKUs[0].ID
}

func TestStripeUntrackedCaptureAndClose(t *testing.T) {
	for _, mixed := range []bool{false, true} {
		name := "all_untracked"
		if mixed {
			name = "mixed"
		}
		for _, close := range []bool{false, true} {
			outcome := "capture"
			if close {
				outcome = "close"
			}
			t.Run(name+"/"+outcome, func(t *testing.T) {
				e, attempt, sku := sucOrder(t, mixed)
				order := e.p.hold.OrderID
				wantLines := 0
				if mixed {
					wantLines = 1
				}
				if n := countRows(t, e.f.owner, `SELECT count(*) FROM inventory.reservation_lines WHERE reservation_id=$1`, order); n != wantLines {
					t.Fatalf("real Begin lines=%d want %d", n, wantLines)
				}
				status, paymentStatus, piStatus := "complete", "paid", "succeeded"
				fact, orderState, reservationState, action := "CAPTURED", "CONFIRMED", "COMMITTED", "ALLOCATE"
				if close {
					status, paymentStatus, piStatus = "expired", "unpaid", "canceled"
					fact, orderState, reservationState, action = "CLOSED_UNPAID", "CANCELLED", "RELEASED", "RELEASE"
				}
				hash := e.observe(t, attempt, e.sessionReport(t, attempt, status, paymentStatus, piStatus, false))
				for i := 0; i < 2; i++ {
					if err := e.apply(attempt, hash); err != nil {
						t.Fatalf("%s observation/replay %d: %v", fact, i, err)
					}
				}
				if n := countRows(t, e.f.owner, `SELECT count(*) FROM payments.facts WHERE attempt_id=$1 AND kind=$2`, attempt, fact); n != 1 {
					t.Fatalf("%s facts=%d", fact, n)
				}
				var actualOrder, actualReservation string
				if err := e.f.owner.QueryRow(context.Background(), `SELECT o.commercial_state,r.state FROM checkout.orders o JOIN inventory.reservations r ON r.id=o.id WHERE o.id=$1`, order).Scan(&actualOrder, &actualReservation); err != nil || actualOrder != orderState || actualReservation != reservationState {
					t.Fatalf("states=%s/%s want=%s/%s err=%v", actualOrder, actualReservation, orderState, reservationState, err)
				}
				if n := countRows(t, e.f.owner, `SELECT count(*) FROM inventory.ledger WHERE checkout_id=$1 AND kind=$2`, order, action); n != wantLines {
					t.Fatalf("%s ledger=%d want %d", action, n, wantLines)
				}
				if n := countRows(t, e.f.owner, `SELECT count(*) FROM inventory.ledger WHERE sku_id=$1`, sku); n != 0 {
					t.Fatalf("untracked SKU moved stock: %d", n)
				}
				work := 0
				if !close {
					work = 1
				}
				if n := countRows(t, e.f.owner, `SELECT count(*) FROM fulfillment.payment_work_items WHERE order_id=$1 AND state='READY'`, order); n != work {
					t.Fatalf("ready work=%d want %d", n, work)
				}
				if n := countRows(t, e.f.owner, `SELECT count(*) FROM checkout.events WHERE order_id=$1 AND action IN ('checkout.payment_captured','checkout.payment_closed')`, order); n != 1 {
					t.Fatalf("terminal events=%d", n)
				}
			})
		}
	}
}

func TestStripeUntrackedReservationRefusals(t *testing.T) {
	for _, close := range []bool{false, true} {
		outcome := "capture"
		if close {
			outcome = "close"
		}
		for _, corruption := range []string{"missing_tracked_lines", "empty_quote", "foreign_sku", "missing_reservation", "tracked_flipped_with_missing_lines"} {
			t.Run(outcome+"/"+corruption, func(t *testing.T) {
				tracked := corruption == "missing_tracked_lines" || corruption == "tracked_flipped_with_missing_lines"
				e, attempt, _ := sucOrder(t, tracked)
				order := e.p.hold.OrderID
				switch corruption {
				case "missing_tracked_lines", "tracked_flipped_with_missing_lines":
					slrReplica(t, e.f, `DELETE FROM inventory.reservation_lines WHERE reservation_id=$1`, order)
					if corruption == "tracked_flipped_with_missing_lines" {
						slrDisclose(t, "catalog tracking changed after a tracked hold; its RESERVE history must remain authoritative")
						mustExec(t, e.f.owner, `UPDATE catalog.skus SET inventory_tracked=false,max_per_order=3 WHERE id=$1`, e.p.stock.skus[0].ID)
					}
				case "empty_quote":
					slrReplica(t, e.f, `UPDATE checkout.orders SET snapshot=jsonb_set(snapshot,'{quote,lines}','[]'::jsonb) WHERE id=$1`, order)
				case "foreign_sku":
					other := psSetupItemsOn(t, fixture(t), 1)
					cap := int64(3)
					foreign, err := k3Doc(t, other.f, t04Key("suc-foreign"), catalog.ProductDocumentInput{
						Name: "Synthetic foreign untracked", Status: catalog.StatusActive,
						SKUs: []catalog.DocumentSKUInput{{PriceMinor: 2500, Stock: &catalog.DocumentStock{Mode: "untracked", MaxPerOrder: &cap}}},
					})
					if err != nil {
						t.Fatal(err)
					}
					// False tracking in the OTHER store must not authorize this order's empty reservation.
					slrReplica(t, e.f, `UPDATE checkout.orders SET snapshot=jsonb_set(snapshot,'{quote,lines,0,sku_id}',to_jsonb($2::text)) WHERE id=$1`, order, foreign.SKUs[0].ID)
				case "missing_reservation":
					slrReplica(t, e.f, `DELETE FROM inventory.reservations WHERE id=$1`, order)
				}
				status, paymentStatus, piStatus := "complete", "paid", "succeeded"
				if close {
					status, paymentStatus, piStatus = "expired", "unpaid", "canceled"
				}
				hash := e.observe(t, attempt, e.sessionReport(t, attempt, status, paymentStatus, piStatus, false))
				slrWant(t, "corrupt reservation", e.apply(attempt, hash), "PT409", "")
				if n := countRows(t, e.f.owner, `SELECT count(*) FROM payments.facts WHERE attempt_id=$1`, attempt); n != 0 {
					t.Fatalf("refusal wrote %d money facts", n)
				}
				if n := countRows(t, e.f.owner, `SELECT count(*) FROM inventory.ledger WHERE checkout_id=$1 AND kind IN ('ALLOCATE','RELEASE')`, order); n != 0 {
					t.Fatalf("refusal moved stock: %d", n)
				}
				if n := countRows(t, e.f.owner, `SELECT count(*) FROM fulfillment.payment_work_items WHERE order_id=$1`, order); n != 0 {
					t.Fatalf("refusal wrote work: %d", n)
				}
			})
		}
	}
}
