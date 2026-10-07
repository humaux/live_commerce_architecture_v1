package foundation_test

// Purpose: the shared fixture that places SEVERAL paid orders for ONE buyer capability at one identical home destination (what the
//   W3-07B parcel-merge browser harness needs for a mergeable pair), plus the focused REAL_PG test that proves it without a browser.
//   Each step fails with its own message so the failing call is visible (the t.Helper chain of the old closure hid it).
// Depends on: pqFixture/bcHarness (bcSetup prepare/begin), pqFixture.start, pcRecord/pcApply, bdSet/bdHome, cqHarness.cart.
// Used by: TestParcelFixtureSameBuyerTwoOrders (REAL_PG) and parcelOrder in browser_merchant_orders_ui_test.go (browser build tag).
// Status: REAL_PG with the MOCK payment capture path of the other pq fixtures.

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/buyer"
	"livecommerce/internal/checkout"
	"livecommerce/internal/storefront"
)

// pfBuyer carries one buyer capability's checkout state (cart, destination, quote versions) across several orders. bcHarness.prepare
// is single-use per capability (its cart and destination are created at version 0), so a second order must edit those rows at
// their live versions, like hcodRehome does for the tcv fixtures.
type pfBuyer struct {
	q       pqFixture
	placed  int
	service int64 // delivery service version every checkout input must carry (prepare hard-codes 1)
}

// pfRestoreHome flips the shared delivery service back to q.delivery's home kind at expectedVersion (a CVS fixture earlier in the
// MOU harness left it as a CVS service) and returns the new version, which pfNewBuyer must be given: begin refuses a stale
// ServiceVersion (checkout.go service.Version != in.ServiceVersion) and a home destination cannot use a CVS service.
func pfRestoreHome(t *testing.T, q pqFixture, expectedVersion int64) int64 {
	t.Helper()
	home := q.delivery
	home.ExpectedVersion = expectedVersion
	svc, err := dsSet(q.cqHarness, t04Key("pf-home-service"), home)
	if err != nil {
		t.Fatalf("restore the home delivery service: %v", err)
	}
	return svc.Version
}

// pfNewBuyer starts a buyer on a COPY of q bound to capability cap (q itself is never mutated); service is the live delivery
// service version (1 when nothing changed it, else the value pfRestoreHome returned).
func pfNewBuyer(q pqFixture, cap buyer.Capability, service int64) *pfBuyer {
	clone := q
	clone.cap = cap
	return &pfBuyer{q: clone, service: service}
}

// order places one order of two units of the first SKU: the first call prepares cart + destination + quote, later calls bump the
// cart and re-select the same bdHome destination at the live versions. paid=false stops at the hold (a DRAFT order never captured).
func (b *pfBuyer) order(t *testing.T, tag string, paid bool) string {
	t.Helper()
	// Quantity 2, never 1: a paid order's total must be whole TWD (a multiple of 100 minor, validPaymentResult) and one unit costs
	// 1250 minor, so a one-unit order cannot start payment ("conflicting request or version").
	items := []storefront.Item{{SKUID: b.q.stock.skus[0].ID, Quantity: 2}}
	if b.placed == 0 {
		b.q.bcHarness.prepare(t, b.q.cap, items)
	} else {
		b.rehome(t, tag, items)
	}
	b.q.bcHarness.input.ServiceVersion = b.service
	b.placed++
	order, err := b.q.bcHarness.begin(t04Key("pf-begin-" + tag))
	if err != nil {
		t.Fatalf("pf[%s] begin: %v", tag, err)
	}
	if !paid {
		return order.OrderID
	}
	b.q.hold = order
	b.q.input.OrderID = order.OrderID
	if b.q.result, err = b.q.start(t04Key("pf-start-" + tag)); err != nil {
		t.Fatalf("pf[%s] start: %v", tag, err)
	}
	report := pcFull(b.q)
	if err = b.q.record(b.q.claim(t), report); err != nil {
		t.Fatalf("pf[%s] record provider report: %v", tag, err)
	}
	if err = pcApply(b.q.worker, b.q.result.AttemptID, pcHash(t, b.q, report)); err != nil {
		t.Fatalf("pf[%s] apply_capture: %v", tag, err)
	}
	return order.OrderID
}

// rehome is the second-order path: bump the cart and re-select the same bdHome destination at their live versions, then quote
// again. prepare cannot be re-run (cart and destination are created at version 0 and conflict once they exist).
func (b *pfBuyer) rehome(t *testing.T, tag string, items []storefront.Item) {
	t.Helper()
	h := &b.q.bcHarness
	c, err := h.cart(t04Key("pf-cart-"+tag), storefront.CartInput{ExpectedVersion: h.input.CartVersion, Items: items})
	if err != nil {
		t.Fatalf("pf[%s] cart bump: %v", tag, err)
	}
	dest := bdHome(c)
	dest.ExpectedVersion = h.destination.Version
	if h.destination, err = bdSet(h.cqHarness, t04Key("pf-dest-"+tag), dest); err != nil {
		t.Fatalf("pf[%s] destination re-select: %v", tag, err)
	}
	h.quote, err = cqBuyer(h.a.runtime, h.cap, func(ctx context.Context, tx pgx.Tx, s buyer.Scope) (storefront.Quote, error) {
		return storefront.CreateQuote(ctx, tx, s, t04Key("pf-quote-"+tag), storefront.QuoteInput{CartVersion: c.Version, MarketID: h.market.ID, Country: "TW", Method: "delivery:" + h.delivery.Code})
	})
	if err != nil {
		t.Fatalf("pf[%s] quote: %v", tag, err)
	}
	h.input = checkout.Input{QuoteID: h.quote.ID, DestinationID: h.destination.ID, CartVersion: c.Version, ServiceVersion: 1, AllocationVersion: 1}
}

// TestParcelFixtureSameBuyerTwoOrders builds the browser harness' mergeable pair without a browser: two paid orders of ONE buyer
// capability at the identical home destination, then proves they are one owner / one destination hash (so one merge suggestion).
func TestParcelFixtureSameBuyerTwoOrders(t *testing.T) {
	q := pqSetup(t)
	// The MOU harness flips the shared delivery service to a CVS kind (version 2) for its pickup fixture BEFORE the parcel stage.
	cvs := q.delivery
	cvs.ExpectedVersion, cvs.DeliveryKind = 1, "cvs_familymart"
	if _, err := dsSet(q.cqHarness, t04Key("pf-cvs-service"), cvs); err != nil {
		t.Fatalf("flip the service to CVS: %v", err)
	}
	b := pfNewBuyer(q, mustIssue(t, q.cqHarness.service, q.f.storeA1), pfRestoreHome(t, q, 2))
	o1, o2 := b.order(t, "a", true), b.order(t, "b", true)
	if o1 == o2 {
		t.Fatalf("both orders have id %s", o1)
	}
	// The harness also hangs an unpaid hold (COD fixture) and one more paid order (CVS fixture) on the same buyer.
	b.order(t, "hold", false)
	b.order(t, "paid-again", true)
	var owners, hashes int
	if err := q.f.owner.QueryRow(context.Background(), `SELECT count(DISTINCT owner_id),count(DISTINCT fulfillment.parcel_destination_hash(snapshot,country)) FROM checkout.orders WHERE id=ANY($1::uuid[]) AND commercial_state='CONFIRMED'`, []string{o1, o2}).Scan(&owners, &hashes); err != nil {
		t.Fatal(err)
	}
	if owners != 1 || hashes != 1 {
		t.Fatalf("two paid orders of the pair: %d owners, %d destination hashes, want 1 and 1", owners, hashes)
	}
}
