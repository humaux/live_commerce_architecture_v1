// Purpose: REAL_PG Stripe capture/close proof at the persisted quantity CHECK boundary.
// Depends on: actual Begin/catalogue/payment commands, 0167 apply_capture, scoped immutable quote, MOCK observations.
// Used by: Stripe focused/foundation gates; synthetic historical rows only, no provider calls or live money.
// Invariants: 1..1000000000 is accepted; larger inserted quantities fail CHECK; no untracked stock movement.
package foundation_test

import (
	"context"
	"testing"

	"livecommerce/internal/catalog"
	"livecommerce/internal/storefront"
)

func TestStripeUntrackedQuantityBoundary(t *testing.T) {
	for _, close := range []bool{false, true} {
		name := "capture"
		if close {
			name = "close"
		}
		t.Run(name, func(t *testing.T) {
			p := psSetupItemsOn(t, fixture(t), 1)
			cap := int64(3)
			doc, err := k3Doc(t, p.f, t04Key("suc-qty"), catalog.ProductDocumentInput{
				Name: "Synthetic historical quantity", Status: catalog.StatusActive,
				SKUs: []catalog.DocumentSKUInput{
					{Code: "free", PriceMinor: 0, Stock: &catalog.DocumentStock{Mode: "untracked", MaxPerOrder: &cap}},
					{Code: "paid", PriceMinor: 2500, Stock: &catalog.DocumentStock{Mode: "untracked", MaxPerOrder: &cap}},
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			free := doc.SKUs[0].ID
			p.bcHarness.prepare(t, mustIssue(t, p.cqHarness.service, p.f.storeA1), []storefront.Item{{SKUID: free, Quantity: 2}, {SKUID: doc.SKUs[1].ID, Quantity: 2}})
			p.hold, err = p.bcHarness.begin(t04Key("suc-qty-hold"))
			if err != nil {
				t.Fatal(err)
			}
			// Historical-schema fixture: modern A6 caps untracked Begin at 999. Seed the persisted
			// CHECK boundary explicitly, preserving zero line total, full quote equality and money.
			// Replica mode bypasses only immutable-row triggers; all relational CHECKs remain active.
			seedQuantity := func(value string) {
				slrReplica(t, p.f, `UPDATE storefront.quotes q SET snapshot=jsonb_set(q.snapshot,'{lines}',
			 (SELECT jsonb_agg(CASE WHEN x.line->>'sku_id'=$2 THEN jsonb_set(x.line,'{quantity}',$3::jsonb) ELSE x.line END ORDER BY x.n)
			 FROM jsonb_array_elements(q.snapshot->'lines') WITH ORDINALITY x(line,n)))
			 FROM checkout.orders o WHERE o.id=$1 AND q.id=o.quote_id AND q.tenant_id=o.tenant_id AND q.store_id=o.store_id AND q.owner_id=o.owner_id`, p.hold.OrderID, free, value)
				slrReplica(t, p.f, `UPDATE checkout.orders o SET snapshot=jsonb_set(o.snapshot,'{quote}',q.snapshot)
			 FROM storefront.quotes q WHERE o.id=$1 AND q.id=o.quote_id AND q.tenant_id=o.tenant_id AND q.store_id=o.store_id AND q.owner_id=o.owner_id`, p.hold.OrderID)
			}
			seedQuantity("1000000000")
			// Real INSERT (not a disabled CHECK or updated mock) proves the schema boundary.
			ctx := context.Background()
			tx, err := p.f.owner.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			var cart string
			if err = tx.QueryRow(ctx, `DELETE FROM storefront.cart_lines WHERE sku_id=$1 RETURNING cart_id::text`, free).Scan(&cart); err != nil {
				t.Fatal(err)
			}
			insert := `INSERT INTO storefront.cart_lines(tenant_id,store_id,owner_id,cart_id,sku_id,quantity)
			 SELECT tenant_id,store_id,owner_id,id,$2::uuid,$3 FROM storefront.carts WHERE id=$1`
			if _, err = tx.Exec(ctx, insert, cart, free, int64(1000000000)); err != nil {
				t.Fatalf("CHECK rejected inclusive upper bound: %v", err)
			}
			if _, err = tx.Exec(ctx, `DELETE FROM storefront.cart_lines WHERE sku_id=$1`, free); err != nil {
				t.Fatal(err)
			}
			_, err = tx.Exec(ctx, insert, cart, free, int64(1000000001))
			slrWant(t, "quantity above CHECK", err, "23514", "")
			if err = tx.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
			e := sstNewEnv(t, p.f, pwKeys(t))
			s := e.seed(t, p)
			result, err := s.begin(e.svc, t04Key("suc-qty-payment"), s.input("zh-TW"))
			if err != nil {
				t.Fatal(err)
			}
			env := &slrEnv{sstEnv: e, p: p}
			env.pin(t, result.AttemptID)
			status, payment, pi, fact, commercial, reservation := "complete", "paid", "succeeded", "CAPTURED", "CONFIRMED", "COMMITTED"
			if close {
				status, payment, pi, fact, commercial, reservation = "expired", "unpaid", "canceled", "CLOSED_UNPAID", "CANCELLED", "RELEASED"
			}
			hash := env.observe(t, result.AttemptID, env.sessionReport(t, result.AttemptID, status, payment, pi, false))
			for _, invalid := range []string{"0", "-1", "1000000001", "9999999999", "9223372036854775808", `"not-a-number"`, "null"} {
				seedQuantity(invalid)
				slrWant(t, "invalid quantity "+invalid, env.apply(result.AttemptID, hash), "PT409", "")
				if countRows(t, p.f.owner, `SELECT count(*) FROM payments.facts WHERE attempt_id=$1`, result.AttemptID) != 0 {
					t.Fatal("quantity refusal wrote a money fact")
				}
			}
			seedQuantity("1000000000")
			for i := 0; i < 2; i++ {
				if err = env.apply(result.AttemptID, hash); err != nil {
					t.Fatalf("quantity 1000000000 %s/replay %d: %v", name, i, err)
				}
			}
			if countRows(t, p.f.owner, `SELECT count(*) FROM payments.facts WHERE attempt_id=$1 AND kind=$2`, result.AttemptID, fact) != 1 {
				t.Fatal("terminal fact missing/duplicated")
			}
			var gotOrder, gotReservation string
			if err = p.f.owner.QueryRow(ctx, `SELECT o.commercial_state,r.state FROM checkout.orders o JOIN inventory.reservations r ON r.id=o.id WHERE o.id=$1`, p.hold.OrderID).Scan(&gotOrder, &gotReservation); err != nil || gotOrder != commercial || gotReservation != reservation {
				t.Fatalf("terminal states=%s/%s err=%v", gotOrder, gotReservation, err)
			}
			if countRows(t, p.f.owner, `SELECT count(*) FROM inventory.ledger WHERE checkout_id=$1`, p.hold.OrderID) != 0 {
				t.Fatal("untracked boundary moved stock")
			}
			if countRows(t, p.f.owner, `SELECT count(*) FROM payments.review_cases WHERE attempt_id=$1`, result.AttemptID) != 0 {
				t.Fatal("valid boundary became a review")
			}
		})
	}
}
