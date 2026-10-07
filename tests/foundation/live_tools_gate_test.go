package foundation_test

// LTG: the INDEPENDENT gate of unit live-tools (R4, migration 0092). Written from the contract only
// (contracts/live-keyword-claims-v1.md, amendment "Live tools (R4)" rules 1-7, contracts/storefront-v2.md section F,
// docs/delivery/units/live-tools.md and the accepted DEVIATIONS), never from internal/claims or internal/storefront.
// tests/foundation/live_tools_test.go is the implementer's author smoke and is not part of this gate.
//
// Tier: REAL_PG + HTTP_PG (the real merchant handler for the library/offer/import/promotion/refund routes, the real
// buyerhttp handler for cart/quote), MOCK PSP for the refund test (stripetest fake through the real capture path). No
// browser here (tests/e2e/live-tools.spec.ts is the browser half). Evidence label of every passing line: REAL_PG.
//
//   LTG01 TestLiveToolsGateLivePriceOnlyThroughClaim   direct cart (domain + HTTP incl. forged body keys), another session's offer
//                                                      on the same SKU, a bundle bound to another buyer, raising the quantity, a
//                                                      price-less offer: only a bound, unexpired, claimed line is live-priced
//   LTG01b TestLiveToolsGateCartReadLivePrice          the buyer cart read (storefront.GetCart, GET /v1/buyer/cart) returns
//                                                      live_unit_price_minor for a live-claim line and nothing for a direct line,
//                                                      another buyer's origin, a raised quantity or an expired link
//   LTG02 TestLiveToolsGateRawCartForgery              raw cart_lines writes as the buyer database role: shape CHECKs, foreign
//                                                      bundle/offer pairs, claim_quantity below AND above the claimed quantity
//   LTG03 TestLiveToolsGateOfferLifecycleAndExpiry     cleared price / link expiry end the price at the next quote; a PAUSED offer does NOT (owner decision
//                                                      2026-10-07: claimants keep it); a quote taken at the live price cannot place an order
//                                                      once the price was cleared (fail closed)
//   LTG04 TestLiveToolsGateSnapshotsAndOrders          price_rule in the quote and order snapshots (card DRAFT and pay_at_pickup),
//                                                      catalog lines stay byte-compatible, later price edits never touch an order
//   LTG05 TestLiveToolsGateLibraryImportCopy           library CRUD, import/copy conflicts as data, never overwrite, price never
//                                                      copied, permissions/tenant isolation, replay, session_full boundary
//   LTG06 TestLiveToolsGateConcurrentOfferCreation     concurrent creates, imports and create-vs-import on one session: no duplicate,
//                                                      no 5xx, library never double-seeded
//   LTG07 TestLiveToolsGatePromotionOnLivePrice        a discount code applies ONCE and on the live price (percent, fixed cap, min
//                                                      subtotal, expiry falls back to catalog), placement through promotions.redeem
//   LTG08 TestLiveToolsGateRefundsCappedAtPaid         a live-priced + discounted order paid through the real capture path refunds at
//                                                      most what was paid (never the catalog price, never above the remainder)
//
// Owner-pool writes (disclosed fixtures): aging a claim link (claims.links.issued_at/expires_at, as the author smoke and the retention gates
// do: no product path ages a link), archiving a SKU for sku_unavailable, and reading result tables. Raw cart_lines forgeries run through
// buyer.WithScope, i.e. as the real commerce_buyer_runtime role with its canonical GUCs (the threat is a compromised application, not a client).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"livecommerce/internal/buyer"
	"livecommerce/internal/checkout"
	"livecommerce/internal/claims"
	"livecommerce/internal/command"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/platform"
	"livecommerce/internal/storefront"
)

const (
	ltgCatalog = int64(30000) // TWD 300.00 per unit (whole dollars: pay-at-pickup and discounts need them)
	ltgLive    = int64(20000) // the live price of the A1 offer
	ltgQty     = int64(2)
)

// ltgEnv is the tcvEnv (store, market, home + pay-at-pickup service, buyer HTTP) plus the claims harness and a merchant handler that mounts the
// claims routes.
type ltgEnv struct {
	*tcvEnv
	h     *lcHarness
	money string // the money SKU, catalog price ltgCatalog
}

func ltgNew(t *testing.T, opts ...tcvOpts) *ltgEnv {
	t.Helper()
	e := tcvNew(t, opts...)
	e.grantCreator("live:read", "live:manage", "pricing:read", "pricing:write", "orders:read", "catalog:read")
	labels, err := claims.NewLabelKey(randomBytes(32))
	if err != nil {
		t.Fatal(err)
	}
	refunds, err := river.NewClient(riverpgxv5.New(e.p.f.runtime), &river.Config{Schema: "river_payment"})
	if err != nil {
		t.Fatal(err)
	}
	// the merchant handler of the gate: claims routes (library/offers/import) + promotions + refunds + CVS settings
	e.merchant = httpapi.NewHandler(e.p.f.runtime, httpapi.Options{CVS: e.cvs, RefundJobs: refunds, Studio: true, ClaimLabels: &labels})
	h := &lcHarness{cqHarness: e.p.cqHarness, ctx: context.Background(), actor: e.p.f.principalA, token: e.token(), labels: labels}
	return &ltgEnv{tcvEnv: e, h: h, money: e.sku(ltgCatalog, 500)}
}

// session creates a draft with an OPEN window and one offer A1 on the money SKU (live price when price > 0).
func (e *ltgEnv) session(keyword string, price, max int64) (string, claims.Offer) {
	e.t.Helper()
	s := e.h.draft(e.t, e.store())
	e.h.open(e.t, s, claims.MatchExact)
	if price > 0 {
		return s, e.h.livePriceOffer(e.t, s, keyword, e.money, max, price)
	}
	return s, e.h.offer(e.t, s, keyword, e.money, max)
}

// claimLink records "<keyword>+<qty>" for a new actor label, closes the window and issues the link.
func (e *ltgEnv) claimLink(session, label, text string) (string, claims.IssuedLink) {
	e.t.Helper()
	c := e.h.accepted(e.t, session, "", label, text)
	l := e.h.link(e.t, session, c.BundleID, 0, false)
	e.h.closeWindow(e.t, session)
	return c.BundleID, l
}

// buyerCap is a buyer on a fresh capability with an empty cart.
func (e *ltgEnv) buyerCap() *tcvBuyer {
	e.t.Helper()
	capability := mustIssue(e.t, e.p.cqHarness.service, e.store())
	h := e.p.bcHarness
	h.cap = capability
	h.input.ServiceVersion, h.input.AllocationVersion = 1, 1
	h.destination = storefront.Destination{} // the harness copy carries another buyer's destination version
	return &tcvBuyer{e: e.tcvEnv, h: h, cap: capability}
}

// redeemed is a fresh buyer that opened the link and applied it to its cart.
func (e *ltgEnv) redeemed(link claims.IssuedLink) *tcvBuyer {
	e.t.Helper()
	b := e.buyerCap()
	pv, err := e.h.preview(b.cap, link.Token)
	if err != nil {
		e.t.Fatalf("preview: %v", err)
	}
	if _, err = e.h.redeem(b.cap, t04Key("ltg-redeem"), link.Token, pv.BundleVersion); err != nil {
		e.t.Fatalf("redeem: %v", err)
	}
	return b
}

// direct is a fresh buyer whose cart holds qty of the money SKU added directly (never through a claim).
func (e *ltgEnv) direct(qty int64) *tcvBuyer {
	e.t.Helper()
	b := e.buyerCap()
	if _, err := e.h.putCart(b.cap, t04Key("ltg-direct"), storefront.CartInput{Items: []storefront.Item{{SKUID: e.money, Quantity: qty}}}); err != nil {
		e.t.Fatalf("direct cart: %v", err)
	}
	return b
}

func (e *ltgEnv) quoteOf(b *tcvBuyer, promo string) (storefront.Quote, error) {
	e.t.Helper()
	cart := e.h.cartOf(e.t, b.cap)
	return cqBuyer(e.p.a.runtime, b.cap, func(ctx context.Context, tx pgx.Tx, s buyer.Scope) (storefront.Quote, error) {
		return storefront.CreateQuote(ctx, tx, s, t04Key("ltg-quote"), storefront.QuoteInput{CartVersion: cart.Version, MarketID: e.p.market.ID, Country: "TW",
			Method: "delivery:" + e.p.delivery.Code, PromoCode: promo})
	})
}

// line is the single quote line of the money SKU.
func (e *ltgEnv) line(b *tcvBuyer, promo string) (storefront.Quote, storefront.QuoteLine) {
	e.t.Helper()
	q, err := e.quoteOf(b, promo)
	if err != nil || len(q.Lines) != 1 {
		e.t.Fatalf("quote: %+v %v", q, err)
	}
	return q, q.Lines[0]
}

// wantCatalog fails unless the line is priced at the catalog price with no live evidence at all.
func (e *ltgEnv) wantCatalog(what string, l storefront.QuoteLine) {
	e.t.Helper()
	if l.UnitPriceMinor != ltgCatalog || l.PriceRule != "" || l.CatalogUnitPriceMinor != 0 || l.ClaimBundleID != "" || l.ClaimOfferID != "" {
		e.t.Fatalf("%s: want catalog price %d with no live evidence, got %+v", what, ltgCatalog, l)
	}
}

func (e *ltgEnv) wantLive(what string, l storefront.QuoteLine, price int64, bundle, offer string) {
	e.t.Helper()
	if l.UnitPriceMinor != price || l.PriceRule != "live_claim" || l.CatalogUnitPriceMinor != ltgCatalog || l.ClaimBundleID != bundle || l.ClaimOfferID != offer {
		e.t.Fatalf("%s: want live price %d (bundle %s offer %s), got %+v", what, price, bundle, offer, l)
	}
}

// cartLive returns the live unit price of the single money-SKU line in b's cart (0 = none). It reads
// the domain cart (storefront.GetCart), the same projection the buyer GET /v1/buyer/cart serves.
func (e *ltgEnv) cartLive(b *tcvBuyer) int64 {
	e.t.Helper()
	cart := e.h.cartOf(e.t, b.cap)
	if len(cart.Items) != 1 || cart.Items[0].SKUID != e.money {
		e.t.Fatalf("cart: want one money line, got %+v", cart.Items)
	}
	return cart.Items[0].LiveUnitPriceMinor
}

// rawCartLine rewrites the buyer's cart line of sku with raw SQL as the buyer database role (DELETE + INSERT: the only writes the role has).
func (e *ltgEnv) rawCartLine(b *tcvBuyer, sku string, qty int64, bundle, offer any, claimQty any) error {
	return buyer.WithScope(context.Background(), e.p.a.runtime, b.cap.Token, e.store(), func(ctx context.Context, tx pgx.Tx, s buyer.Scope) error {
		var cartID string
		if err := tx.QueryRow(ctx, `SELECT id::text FROM storefront.carts WHERE tenant_id=$1 AND store_id=$2 AND owner_id=$3`, s.TenantID, s.StoreID, s.OwnerID).Scan(&cartID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM storefront.cart_lines WHERE tenant_id=$1 AND store_id=$2 AND owner_id=$3 AND cart_id=$4`, s.TenantID, s.StoreID, s.OwnerID, cartID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO storefront.cart_lines(tenant_id,store_id,owner_id,cart_id,sku_id,quantity,claim_bundle_id,claim_offer_id,claim_quantity)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, s.TenantID, s.StoreID, s.OwnerID, cartID, sku, qty, bundle, offer, claimQty)
		return err
	})
}

// ageLink moves a bundle's link into the past (owner fixture) or to expire in d.
func (e *ltgEnv) setLinkExpiry(bundle string, in time.Duration) {
	e.t.Helper()
	// issued_at must stay within the 72 h TTL CHECK (expires_at <= issued_at + 72 h) and before expires_at.
	mustExec(e.t, e.p.f.owner, `UPDATE claims.links SET issued_at=clock_timestamp()-interval '71 hours', expires_at=clock_timestamp()+make_interval(secs=>$2) WHERE bundle_id=$1`, bundle, in.Seconds())
}

func (e *ltgEnv) orderFacts(owner string) (holds, orders int) {
	return e.count(`SELECT count(*) FROM inventory.reservations WHERE buyer_owner_id=$1`, owner), e.count(`SELECT count(*) FROM checkout.orders WHERE owner_id=$1`, owner)
}

// placeHome places a home-delivery CARD order (a DRAFT hold with the frozen quote snapshot) with the given quote.
func (e *ltgEnv) placeHome(b *tcvBuyer, q storefront.Quote) (checkout.Result, error) {
	cart := e.h.cartOf(e.t, b.cap)
	din := bdHome(cart)
	din.ExpectedVersion = b.h.destination.Version
	dest, err := bdSet(b.h.cqHarness, t04Key("ltg-dest"), din)
	if err != nil {
		return checkout.Result{}, fmt.Errorf("destination: %w", err)
	}
	b.h.destination = dest
	in := checkout.Input{QuoteID: q.ID, DestinationID: dest.ID, CartVersion: cart.Version, ServiceVersion: 1, AllocationVersion: 1}
	return e.svc.Begin(context.Background(), b.cap.Token, e.store(), t04Key("ltg-begin"), in)
}

// snapLines parses an order snapshot (jsonb text: spaces after colons) into its quote lines and the quote object.
func snapQuote(t *testing.T, raw []byte) (lines []map[string]any, quote map[string]any) {
	t.Helper()
	var snap struct {
		Quote map[string]any `json:"quote"`
	}
	if err := json.Unmarshal(raw, &snap); err != nil || snap.Quote == nil {
		t.Fatalf("order snapshot: %v %s", err, raw)
	}
	ls, _ := snap.Quote["lines"].([]any)
	for _, l := range ls {
		lines = append(lines, l.(map[string]any))
	}
	return lines, snap.Quote
}

// ---- merchant HTTP helpers -------------------------------------------------------------------------------------------------------------

func (e *ltgEnv) claimsPath(session, tail string) string {
	return "/v1/admin/stores/" + e.store() + "/live-sessions/" + session + "/claims" + tail
}

func (e *ltgEnv) mjson(token, method, path, body string) (int, map[string]any) {
	e.t.Helper()
	key := ""
	if method != http.MethodGet {
		key = t04Key("ltg-http")
	}
	st, out, _ := e.mcall(token, method, path, key, body)
	return st, out
}

func ltgNum(m map[string]any, key string) int64 {
	f, _ := m[key].(float64)
	return int64(f)
}

func (e *ltgEnv) boardOffers(session string) []claims.Offer {
	e.t.Helper()
	return e.h.board(e.t, session).Offers
}

// ---------------------------------------------------------------------------------------------------------------------------------------
// LTG01
// ---------------------------------------------------------------------------------------------------------------------------------------

func TestLiveToolsGateLivePriceOnlyThroughClaim(t *testing.T) {
	e := ltgNew(t)
	s1, o1 := e.session("A1", ltgLive, 5)
	bundle1, l1 := e.claimLink(s1, "amy", "A1+2")
	// A different live session offers the SAME SKU more cheaply; the buyers of session 1 must never see that price.
	s2, o2 := e.session("A1", 15000, 5)
	bundle2, l2 := e.claimLink(s2, "bob", "A1+2")

	// (1) direct cart, domain level and through the real buyer HTTP routes: catalog price, no live evidence.
	d := e.direct(ltgQty)
	_, line := e.line(d, "")
	e.wantCatalog("direct cart (domain)", line)
	hd := e.buyerCap()
	put := hd.req("PUT", "/v1/buyer/cart", t04Key("ltg-cart"), map[string]any{"expected_version": 0, "items": []map[string]any{{"sku_id": e.money, "quantity": ltgQty}}}, nil)
	if put.status != 200 {
		t.Fatalf("HTTP cart: %d %s", put.status, put.body)
	}
	cartV := ltgNum(tcvJSON(t, put.body), "version")
	quote := hd.req("POST", "/v1/buyer/quotes", t04Key("ltg-q"), map[string]any{"cart_version": cartV, "market_id": e.p.market.ID, "country": "TW", "method": "delivery:" + e.p.delivery.Code}, nil)
	if quote.status != 200 {
		t.Fatalf("HTTP quote: %d %s", quote.status, quote.body)
	}
	var wire struct {
		Lines []map[string]any `json:"lines"`
	}
	if err := json.Unmarshal(quote.body, &wire); err != nil || len(wire.Lines) != 1 {
		t.Fatalf("quote body: %v %s", err, quote.body)
	}
	if int64(wire.Lines[0]["unit_price_minor"].(float64)) != ltgCatalog {
		t.Fatalf("HTTP direct cart priced %v", wire.Lines[0]["unit_price_minor"])
	}
	for _, k := range []string{"price_rule", "catalog_unit_price_minor", "claim_bundle_id", "claim_offer_id"} {
		if _, ok := wire.Lines[0][k]; ok {
			t.Fatalf("a catalog line must not carry %q (snapshots stay byte-compatible): %s", k, quote.body)
		}
	}

	// (2) A buyer cannot NAME an origin: extra keys in the cart body are refused (strict decoder) or ignored, and either way the price stays catalog
	// and no claim_* column is written.
	for i, body := range []map[string]any{
		{"expected_version": 0, "items": []map[string]any{{"sku_id": e.money, "quantity": ltgQty, "claim_bundle_id": bundle1, "claim_offer_id": o1.ID, "claim_quantity": 2}}},
		{"expected_version": 0, "items": []map[string]any{{"sku_id": e.money, "quantity": ltgQty}}, "origins": map[string]any{e.money: map[string]any{"BundleID": bundle1, "OfferID": o1.ID, "Quantity": 2}}},
		{"expected_version": 0, "items": []map[string]any{{"sku_id": e.money, "quantity": ltgQty}}, "unit_price_minor": 1},
	} {
		fb := e.buyerCap()
		r := fb.req("PUT", "/v1/buyer/cart", t04Key("ltg-forge"), body, nil)
		if r.status >= 500 {
			t.Fatalf("forged cart body %d answered %d %s", i, r.status, r.body)
		}
		if r.status == 200 {
			if n := e.count(`SELECT count(*) FROM storefront.cart_lines WHERE owner_id=$1 AND claim_bundle_id IS NOT NULL`, fb.cap.Scope.OwnerID); n != 0 {
				t.Fatalf("forged cart body %d wrote a claim origin (%d lines)", i, n)
			}
			_, l := e.line(fb, "")
			e.wantCatalog(fmt.Sprintf("forged cart body %d", i), l)
		}
	}

	// (3) The claimant: live price on the claimed quantity, with full evidence.
	b1 := e.redeemed(l1)
	_, line = e.line(b1, "")
	e.wantLive("claimant of session 1", line, ltgLive, bundle1, o1.ID)
	if line.Amount.SubtotalMinor != ltgQty*ltgLive {
		t.Fatalf("live line subtotal %d", line.Amount.SubtotalMinor)
	}
	// The other session's offer prices only ITS OWN bundle.
	b2 := e.redeemed(l2)
	_, line2 := e.line(b2, "")
	e.wantLive("claimant of session 2", line2, 15000, bundle2, o2.ID)

	// (4) A bundle bound to another buyer: a third buyer can neither redeem nor preview it, and a forged cart origin naming it earns nothing.
	b3 := e.buyerCap()
	if _, err := e.h.redeem(b3.cap, t04Key("ltg-r3"), l1.Token, 1); !errors.Is(err, command.ErrNotFound) {
		t.Fatalf("a bound link redeemed by another buyer: %v", err)
	}
	if _, err := e.h.preview(b3.cap, l1.Token); !errors.Is(err, command.ErrNotFound) {
		t.Fatalf("a bound link previewed by another buyer: %v", err)
	}
	if _, err := e.h.putCart(b3.cap, t04Key("ltg-b3"), storefront.CartInput{Items: []storefront.Item{{SKUID: e.money, Quantity: ltgQty}}}); err != nil {
		t.Fatal(err)
	}
	if err := e.rawCartLine(b3, e.money, ltgQty, bundle1, o1.ID, ltgQty); err != nil {
		t.Fatalf("forgery setup (buyer role INSERT): %v", err)
	}
	_, l3 := e.line(b3, "")
	e.wantCatalog("a forged origin naming another buyer's bundle", l3)

	// (5) Another session's offer on the same SKU, paired with this buyer's own bundle, earns nothing either (offer must belong to the bundle's session
	// and be a line of the bundle).
	if err := e.rawCartLine(b1, e.money, ltgQty, bundle1, o2.ID, ltgQty); err != nil {
		t.Fatal(err)
	}
	_, l := e.line(b1, "")
	e.wantCatalog("own bundle + another session's cheaper offer", l)
	if err := e.rawCartLine(b1, e.money, ltgQty, bundle2, o1.ID, ltgQty); err != nil {
		t.Fatal(err)
	}
	_, l = e.line(b1, "")
	e.wantCatalog("another buyer's bundle + own offer", l)
	if err := e.rawCartLine(b1, e.money, ltgQty, bundle1, o1.ID, ltgQty); err != nil {
		t.Fatal(err)
	}
	_, l = e.line(b1, "")
	e.wantLive("own origin restored", l, ltgLive, bundle1, o1.ID)

	// (6) Raising the quantity above the claim through the real HTTP cart route drops the live price for the WHOLE line, for good.
	cart := e.h.cartOf(t, b1.cap)
	r := b1.req("PUT", "/v1/buyer/cart", t04Key("ltg-raise"), map[string]any{"expected_version": cart.Version, "items": []map[string]any{{"sku_id": e.money, "quantity": ltgQty + 1}}}, nil)
	if r.status != 200 {
		t.Fatalf("raise quantity: %d %s", r.status, r.body)
	}
	_, l = e.line(b1, "")
	e.wantCatalog("quantity above the claim", l)
	cart = e.h.cartOf(t, b1.cap)
	if r = b1.req("PUT", "/v1/buyer/cart", t04Key("ltg-back"), map[string]any{"expected_version": cart.Version, "items": []map[string]any{{"sku_id": e.money, "quantity": ltgQty}}}, nil); r.status != 200 {
		t.Fatalf("back to the claimed quantity: %d %s", r.status, r.body)
	}
	_, l = e.line(b1, "")
	e.wantCatalog("a dropped origin does not come back by lowering the quantity", l)

	// (7) A price-less offer never earns anything, even with a perfect origin.
	s3, o3 := e.session("P1", 0, 5)
	_, l3link := e.claimLink(s3, "cy", "P1+2")
	b4 := e.redeemed(l3link)
	_, l = e.line(b4, "")
	e.wantCatalog("offer without a live price", l)
	if n := e.count(`SELECT count(*) FROM storefront.cart_lines WHERE owner_id=$1 AND claim_offer_id=$2`, b4.cap.Scope.OwnerID, o3.ID); n != 1 {
		t.Fatalf("the origin of a price-less offer is still recorded as evidence of origin (%d)", n)
	}
}

// ---------------------------------------------------------------------------------------------------------------------------------------
// LTG01b
// ---------------------------------------------------------------------------------------------------------------------------------------

// The buyer cart read returns live_unit_price_minor for a line whose claim origin still earns a live
// price, and nothing (0, omitted on the wire) for any other line. It is display-only and uses the same
// evaluator as the Quote, so the cart preview and the quotation agree.
func TestLiveToolsGateCartReadLivePrice(t *testing.T) {
	e := ltgNew(t)
	s1, o1 := e.session("A1", ltgLive, 5)
	bundle1, l1 := e.claimLink(s1, "amy", "A1+2")
	s2, _ := e.session("A1", 15000, 5)
	_, l2 := e.claimLink(s2, "bob", "A1+2")

	// (1) a non-claim line carries no live unit price.
	if got := e.cartLive(e.direct(ltgQty)); got != 0 {
		t.Fatalf("direct cart live unit price = %d, want 0", got)
	}

	// (2) the claimant: the cart read returns the live unit price on the wire (and the domain value).
	b1 := e.redeemed(l1)
	if got := e.cartLive(b1); got != ltgLive {
		t.Fatalf("claimant cart live unit price = %d, want %d", got, ltgLive)
	}
	got := b1.req("GET", "/v1/buyer/cart", "", nil, nil)
	if got.status != 200 {
		t.Fatalf("GET cart: %d %s", got.status, got.body)
	}
	items := tcvJSON(t, got.body)["items"].([]any)
	if len(items) != 1 || int64(items[0].(map[string]any)["live_unit_price_minor"].(float64)) != ltgLive {
		t.Fatalf("GET cart item must carry live_unit_price_minor=%d: %s", ltgLive, got.body)
	}

	// (3) a forged origin naming another buyer's bundle earns nothing on the cart read either.
	b3 := e.buyerCap()
	if _, err := e.h.putCart(b3.cap, t04Key("ltg-crl-b3"), storefront.CartInput{Items: []storefront.Item{{SKUID: e.money, Quantity: ltgQty}}}); err != nil {
		t.Fatal(err)
	}
	if err := e.rawCartLine(b3, e.money, ltgQty, bundle1, o1.ID, ltgQty); err != nil {
		t.Fatalf("forgery setup (buyer role INSERT): %v", err)
	}
	if got := e.cartLive(b3); got != 0 {
		t.Fatalf("another buyer's origin live unit price = %d, want 0", got)
	}

	// (4) quantity above the claim drops the live price for the whole line.
	b4 := e.redeemed(l2)
	if got := e.cartLive(b4); got != 15000 {
		t.Fatalf("session-2 claimant cart live unit price = %d, want 15000", got)
	}
	cart := e.h.cartOf(t, b4.cap)
	if r := b4.req("PUT", "/v1/buyer/cart", t04Key("ltg-crl-raise"), map[string]any{"expected_version": cart.Version, "items": []map[string]any{{"sku_id": e.money, "quantity": ltgQty + 1}}}, nil); r.status != 200 {
		t.Fatalf("raise quantity: %d %s", r.status, r.body)
	}
	if got := e.cartLive(b4); got != 0 {
		t.Fatalf("quantity above the claim live unit price = %d, want 0", got)
	}

	// (5) an expired link shows no live unit price.
	e.setLinkExpiry(bundle1, -time.Minute)
	if got := e.cartLive(b1); got != 0 {
		t.Fatalf("expired link live unit price = %d, want 0", got)
	}
}

// ---------------------------------------------------------------------------------------------------------------------------------------
// LTG02
// ---------------------------------------------------------------------------------------------------------------------------------------

func TestLiveToolsGateRawCartForgery(t *testing.T) {
	e := ltgNew(t)
	s1, o1 := e.session("A1", ltgLive, 5)
	bundle1, l1 := e.claimLink(s1, "amy", "A1+2")
	b := e.redeemed(l1)
	_, l := e.line(b, "")
	e.wantLive("baseline", l, ltgLive, bundle1, o1.ID)

	// Shape CHECK: claim_bundle_id, claim_offer_id, claim_quantity are all NULL or all set, claim_quantity 1..999.
	for name, c := range map[string]struct{ bundle, offer, qty any }{
		"bundle only":           {bundle1, nil, nil},
		"offer only":            {nil, o1.ID, nil},
		"quantity only":         {nil, nil, int64(2)},
		"bundle+offer no qty":   {bundle1, o1.ID, nil},
		"quantity zero":         {bundle1, o1.ID, int64(0)},
		"quantity negative":     {bundle1, o1.ID, int64(-1)},
		"quantity over 999":     {bundle1, o1.ID, int64(1000)},
		"bundle+qty no offer":   {bundle1, nil, int64(2)},
		"offer+qty no bundle":   {nil, o1.ID, int64(2)},
		"bundle null offer set": {nil, o1.ID, int64(2)},
	} {
		err := e.rawCartLine(b, e.money, ltgQty, c.bundle, c.offer, c.qty)
		if err == nil {
			t.Errorf("%s: the database accepted a malformed claim origin", name)
			continue
		}
		requirePGCode(t, err, "23514", name)
	}

	// A nonexistent bundle/offer pair earns nothing (no FK on purpose: the retention purge deletes claim rows), and writes no price.
	if err := e.rawCartLine(b, e.money, ltgQty, randomUUID(), randomUUID(), ltgQty); err != nil {
		t.Fatalf("a well-formed unknown origin must be storable: %v", err)
	}
	_, l = e.line(b, "")
	e.wantCatalog("unknown bundle/offer", l)

	// claim_quantity BELOW the cart quantity (only raw SQL can write it): the whole line pays catalog.
	if err := e.rawCartLine(b, e.money, ltgQty, bundle1, o1.ID, int64(1)); err != nil {
		t.Fatal(err)
	}
	_, l = e.line(b, "")
	e.wantCatalog("claim_quantity 1 < quantity 2", l)

	// A line whose quantity is within a (genuine) claim_quantity but the claimed quantity was 2: a smaller line keeps the live price.
	if err := e.rawCartLine(b, e.money, 1, bundle1, o1.ID, ltgQty); err != nil {
		t.Fatal(err)
	}
	_, l = e.line(b, "")
	e.wantLive("1 unit of a 2-unit claim", l, ltgLive, bundle1, o1.ID)
}

// The money rule says a forged cart origin can never produce a price a legitimate claim would not (migration 0092 header; amendment rule 7: "or by
// raising the quantity above the claimed quantity"). The claim here is for 2 units; a compromised application that writes claim_quantity 99 and
// quantity 99 must still not get 99 units at the live price.
func TestLiveToolsGateForgedClaimQuantityAboveTheClaim(t *testing.T) {
	e := ltgNew(t)
	s1, o1 := e.session("A1", ltgLive, 5)
	bundle1, l1 := e.claimLink(s1, "amy", "A1+2")
	b := e.redeemed(l1)
	if err := e.rawCartLine(b, e.money, 99, bundle1, o1.ID, int64(99)); err != nil {
		t.Fatalf("forgery setup: %v", err)
	}
	_, l := e.line(b, "")
	if l.PriceRule == "live_claim" {
		t.Fatalf("DEFECT: 99 units were live-priced (%d each) on a claim for 2 units: claim_quantity was taken from the cart line, never compared with claims.lines.quantity: %+v", l.UnitPriceMinor, l)
	}
	e.wantCatalog("forged claim_quantity above the claimed quantity", l)
}

// ---------------------------------------------------------------------------------------------------------------------------------------
// LTG03
// ---------------------------------------------------------------------------------------------------------------------------------------

func TestLiveToolsGateOfferLifecycleAndExpiry(t *testing.T) {
	e := ltgNew(t)
	s1, o1 := e.session("A1", ltgLive, 5)
	bundle1, l1 := e.claimLink(s1, "amy", "A1+2")
	b := e.redeemed(l1)
	_, l := e.line(b, "")
	e.wantLive("baseline", l, ltgLive, bundle1, o1.ID)
	patch := func(version int64, body string) (int, map[string]any) {
		return e.mjson(e.token(), "PATCH", e.claimsPath(s1, "/offers/"+o1.ID), fmt.Sprintf(`{"expected_version":%d,%s}`, version, body))
	}

	// Pausing the offer leaves the stored price alone (key absent = unchanged). Owner decision 2026-10-07 ("already-claimed buyers keep the live price,
	// only new claims are refused") REPLACES the former assertion here (catalog price at the next quote): amy's claim line keeps the price it was granted.
	st, out := patch(o1.Version, `"max_quantity_per_claim":5,"active":false`)
	if st != 200 || out["active"] != false || ltgNum(out, "live_price_minor") != ltgLive {
		t.Fatalf("pause: %d %v", st, out)
	}
	v := ltgNum(out, "version")
	_, l = e.line(b, "")
	e.wantLive("paused offer: the claimed line keeps its live price (owner decision 2026-10-07)", l, ltgLive, bundle1, o1.ID)
	// Re-activating changes nothing: the price is evaluated at quote time, not frozen at offer time.
	st, out = patch(v, `"max_quantity_per_claim":5,"active":true`)
	if st != 200 {
		t.Fatalf("reactivate: %d %v", st, out)
	}
	v = ltgNum(out, "version")
	_, l = e.line(b, "")
	e.wantLive("reactivated offer", l, ltgLive, bundle1, o1.ID)

	// Changing the price changes later quotes only: an existing quote keeps its snapshot.
	earlier, _ := e.line(b, "")
	st, out = patch(v, `"max_quantity_per_claim":5,"active":true,"live_price_minor":10000`)
	if st != 200 || ltgNum(out, "live_price_minor") != 10000 {
		t.Fatalf("set price: %d %v", st, out)
	}
	v = ltgNum(out, "version")
	if got, err := cqBuyer(e.p.a.runtime, b.cap, func(ctx context.Context, tx pgx.Tx, s buyer.Scope) (storefront.Quote, error) {
		return storefront.GetQuote(ctx, tx, s, earlier.ID)
	}); err != nil || got.Lines[0].UnitPriceMinor != ltgLive {
		t.Fatalf("the earlier quote must keep its frozen price %d: %+v %v", ltgLive, got.Lines, err)
	}
	_, l = e.line(b, "")
	e.wantLive("edited price", l, 10000, bundle1, o1.ID)

	// 0 clears the price (and the offer then prices at catalog); invalid prices are refused and change nothing.
	for _, bad := range []string{`-5`, `1000000000001`, `null`, `"20000"`, `1.5`} {
		st, _ := patch(v, `"max_quantity_per_claim":5,"active":true,"live_price_minor":`+bad)
		if st != 422 && st != 400 { // out-of-range value: invalid_request 422; wrong JSON type / null: strict decoder invalid_json 400 (contract section 7)
			t.Errorf("live_price_minor %s: want 400/422, got %d", bad, st)
		}
	}
	if o := e.boardOffers(s1)[0]; o.Version != v || o.LivePriceMinor == nil || *o.LivePriceMinor != 10000 {
		t.Fatalf("a refused price changed the offer: %+v", o)
	}
	st, out = patch(v, `"max_quantity_per_claim":5,"active":true,"live_price_minor":0`)
	if st != 200 || out["live_price_minor"] != nil {
		t.Fatalf("clear: %d %v", st, out)
	}
	v = ltgNum(out, "version")
	_, l = e.line(b, "")
	e.wantCatalog("cleared live price", l)
	st, out = patch(v, `"max_quantity_per_claim":5,"active":true,"live_price_minor":20000`)
	if st != 200 {
		t.Fatalf("set again: %d %v", st, out)
	}
	v = ltgNum(out, "version")
	// A stale expected_version never changes the price.
	if st, _ = patch(v-1, `"max_quantity_per_claim":5,"active":true,"live_price_minor":1`); st != 409 {
		t.Fatalf("stale version: %d", st)
	}
	if o := e.boardOffers(s1)[0]; *o.LivePriceMinor != ltgLive {
		t.Fatalf("a stale write changed the price: %+v", o)
	}

	// A quote taken at the live price cannot place an order after the offer stopped being live priced: fail closed, no hold, no order.
	// Owner decision 2026-10-07: a PAUSE no longer ends a live quote (TestLivePriceKeepOnPauseExistingClaims places one while paused), so the merchant
	// removing the live price (0 = clear, here on the paused offer) is what ends it; the assertion is otherwise identical (conflict, no facts).
	q, l := e.line(b, "")
	e.wantLive("quote before the price is cleared", l, ltgLive, bundle1, o1.ID)
	holds0, orders0 := e.orderFacts(b.cap.Scope.OwnerID)
	if st, out = patch(v, `"max_quantity_per_claim":5,"active":false,"live_price_minor":0`); st != 200 || out["live_price_minor"] != nil {
		t.Fatalf("clear the price of the paused offer: %d %v", st, out)
	}
	if _, err := e.placeHome(b, q); !errors.Is(err, command.ErrConflict) {
		t.Fatalf("a live-price quote must fail closed as a conflict once its live price was cleared (an order was placed, or the refusal was an infrastructure error): %v", err)
	}
	if h, o := e.orderFacts(b.cap.Scope.OwnerID); h != holds0 || o != orders0 {
		t.Fatalf("a refused placement left facts: holds %d->%d orders %d->%d", holds0, h, orders0, o)
	}
	// Re-quote prices at the catalog and then places normally.
	q2, l := e.line(b, "")
	e.wantCatalog("re-quote after the live price was cleared", l)
	res, err := e.placeHome(b, q2)
	if err != nil {
		t.Fatalf("catalog placement: %v", err)
	}
	if got := e.count(`SELECT total_minor FROM checkout.orders WHERE id=$1`, res.OrderID); int64(got) != ltgQty*ltgCatalog {
		t.Fatalf("order total %d, want the catalog total %d", got, ltgQty*ltgCatalog)
	}
}

func TestLiveToolsGateLinkExpiry(t *testing.T) {
	e := ltgNew(t)
	s1, o1 := e.session("A1", ltgLive, 5)
	bundle1, l1 := e.claimLink(s1, "amy", "A1+2")
	b := e.redeemed(l1)
	// The price lives exactly as long as the link: valid for a few more seconds, then gone at the next quote with no other change.
	e.setLinkExpiry(bundle1, 4*time.Second)
	q, l := e.line(b, "")
	e.wantLive("link about to expire", l, ltgLive, bundle1, o1.ID)
	if _, err := checkoutRevalidate(e.h.cqHarness, b.cap, q.ID, e.h.cartOf(t, b.cap).Version); err != nil {
		t.Fatalf("revalidate while the link is valid: %v", err)
	}
	time.Sleep(4500 * time.Millisecond)
	_, l = e.line(b, "")
	e.wantCatalog("after the link expired", l)
	// The earlier live quote is dead: revalidation fails closed (conflict) and checkout places nothing.
	if _, err := checkoutRevalidate(e.h.cqHarness, b.cap, q.ID, e.h.cartOf(t, b.cap).Version); !errors.Is(err, command.ErrConflict) {
		t.Fatalf("revalidate of a live quote after expiry must be a conflict, got %v", err)
	}
	holds0, orders0 := e.orderFacts(b.cap.Scope.OwnerID)
	if _, err := e.placeHome(b, q); !errors.Is(err, command.ErrConflict) {
		t.Fatalf("a live-price quote must fail closed as a conflict once the link expired: %v", err)
	}
	if h, o := e.orderFacts(b.cap.Scope.OwnerID); h != holds0 || o != orders0 {
		t.Fatalf("a refused placement left facts: holds %d->%d orders %d->%d", holds0, h, orders0, o)
	}
	// The buyer-visible link display agrees: an expired link shows no live price (or nothing at all).
	if pv, err := e.h.preview(b.cap, l1.Token); err == nil {
		for _, pl := range pv.Lines {
			if pl.PriceRule != "" || pl.UnitPriceMinor == ltgLive {
				t.Fatalf("an expired link still advertises the live price: %+v", pl)
			}
		}
	}
	// A link rotated by the merchant is a NEW link with a new expiry: the claim earns the price again while it is valid.
	e.h.link(t, s1, bundle1, 1, false)
	_, l = e.line(b, "")
	e.wantLive("claim with a freshly issued link", l, ltgLive, bundle1, o1.ID)
}

// ---------------------------------------------------------------------------------------------------------------------------------------
// LTG03b: the checkout pool must be able to place the order of a claim-origin cart (amendment rule 5: CreateQuote AND RevalidateQuote are the only
// price code path, and checkout.Begin runs RevalidateQuote on the checkout runtime pool, a different database login than the buyer pool).
// ---------------------------------------------------------------------------------------------------------------------------------------

func TestLiveToolsGateCheckoutPlacesClaimOriginCart(t *testing.T) {
	e := ltgNew(t)
	// A claim for a price-less offer: catalog price, but the cart line still carries its claim origin.
	s1, _ := e.session("P1", 0, 5)
	_, l1 := e.claimLink(s1, "amy", "P1+2")
	b := e.redeemed(l1)
	q, l := e.line(b, "")
	e.wantCatalog("claim of a price-less offer", l)
	res, err := e.placeHome(b, q)
	if err != nil {
		t.Fatalf("DEFECT D1: checkout.Begin of a claim-origin cart failed on the checkout runtime pool: %v", err)
	}
	if got := e.count(`SELECT total_minor FROM checkout.orders WHERE id=$1`, res.OrderID); int64(got) != ltgQty*ltgCatalog {
		t.Fatalf("order total %d", got)
	}
	// And the live-priced claim of the same SKU in a second session places at the live total.
	s2, o2 := e.session("A1", ltgLive, 5)
	bundle2, l2 := e.claimLink(s2, "bob", "A1+2")
	b2 := e.redeemed(l2)
	q2, l := e.line(b2, "")
	e.wantLive("live claim", l, ltgLive, bundle2, o2.ID)
	res2, err := e.placeHome(b2, q2)
	if err != nil {
		t.Fatalf("DEFECT D1: placing a live-priced order failed: %v", err)
	}
	if got := e.count(`SELECT total_minor FROM checkout.orders WHERE id=$1`, res2.OrderID); int64(got) != q2.Amount.TotalMinor || int64(got) != ltgQty*ltgLive+q2.Amount.ShippingMinor+q2.Amount.TaxMinor {
		t.Fatalf("live order total %d (quote %d)", got, q2.Amount.TotalMinor)
	}
}

// ---------------------------------------------------------------------------------------------------------------------------------------
// LTG04
// ---------------------------------------------------------------------------------------------------------------------------------------

func TestLiveToolsGateSnapshotsAndOrders(t *testing.T) {
	e := ltgNew(t)
	s1, o1 := e.session("A1", ltgLive, 5)
	bundle1, l1 := e.claimLink(s1, "amy", "A1+2")
	b := e.redeemed(l1)
	q, l := e.line(b, "")
	e.wantLive("claim quote", l, ltgLive, bundle1, o1.ID)

	// The buyer link display shows the live price (the claim page); the accepted deviation keeps the HTTP projection's key set closed.
	pv, err := e.h.preview(mustIssue(t, e.p.cqHarness.service, e.store()), l1.Token)
	if err == nil {
		t.Fatalf("a bound link previewed by a stranger: %+v", pv)
	}
	if pv, err = e.h.preview(b.cap, l1.Token); err != nil || len(pv.Lines) != 1 || pv.Lines[0].UnitPriceMinor != ltgLive {
		t.Fatalf("claimant preview: %+v %v", pv, err)
	}

	// Card order: the DRAFT hold carries the frozen quote; the order total is the live total.
	res, err := e.placeHome(b, q)
	if err != nil {
		t.Fatalf("place a live-priced order: %v", err)
	}
	var total int64
	var snapshot []byte
	if err = e.p.f.owner.QueryRow(context.Background(), `SELECT total_minor,snapshot FROM checkout.orders WHERE id=$1`, res.OrderID).Scan(&total, &snapshot); err != nil {
		t.Fatal(err)
	}
	if total != q.Amount.TotalMinor || total != ltgQty*ltgLive+q.Amount.ShippingMinor+q.Amount.TaxMinor {
		t.Fatalf("order total %d, quote total %d (live subtotal %d)", total, q.Amount.TotalMinor, ltgQty*ltgLive)
	}
	var snap struct {
		Quote struct {
			Lines []map[string]any `json:"lines"`
		} `json:"quote"`
	}
	if err = json.Unmarshal(snapshot, &snap); err != nil || len(snap.Quote.Lines) != 1 {
		t.Fatalf("order snapshot: %v %s", err, snapshot)
	}
	sl := snap.Quote.Lines[0]
	if sl["price_rule"] != "live_claim" || int64(sl["unit_price_minor"].(float64)) != ltgLive || int64(sl["catalog_unit_price_minor"].(float64)) != ltgCatalog ||
		sl["claim_bundle_id"] != bundle1 || sl["claim_offer_id"] != o1.ID {
		t.Fatalf("the order snapshot does not record which price rule applied: %v", sl)
	}
	// The buyer sees the same through the real HTTP order route (the page shows line = unit x quantity).
	got := b.req("GET", "/v1/buyer/orders/"+res.OrderID, "", nil, nil)
	if got.status != 200 || !strings.Contains(string(got.body), fmt.Sprintf(`"unit_price_minor":%d`, ltgLive)) {
		t.Fatalf("buyer order read: %d %s", got.status, got.body)
	}
	// Later merchant edits (price, pause, even link expiry) never touch an existing order.
	e.setLinkExpiry(bundle1, time.Second) // the order is frozen regardless of what happens to the link
	st, out := e.mjson(e.token(), "PATCH", e.claimsPath(s1, "/offers/"+o1.ID), fmt.Sprintf(`{"expected_version":%d,"max_quantity_per_claim":5,"active":false,"live_price_minor":1}`, o1.Version))
	if st != 200 {
		t.Fatalf("edit offer after the order: %d %v", st, out)
	}
	var total2 int64
	var snapshot2 []byte
	if err = e.p.f.owner.QueryRow(context.Background(), `SELECT total_minor,snapshot FROM checkout.orders WHERE id=$1`, res.OrderID).Scan(&total2, &snapshot2); err != nil || total2 != total || string(snapshot2) != string(snapshot) {
		t.Fatalf("an offer edit changed a placed order: %d -> %d", total, total2)
	}
	order, err := e.svc.Get(context.Background(), b.cap.Token, e.store(), res.OrderID)
	if err != nil || order.Snapshot.Quote.Lines[0].PriceRule != "live_claim" || order.Snapshot.Quote.Lines[0].UnitPriceMinor != ltgLive {
		t.Fatalf("service readback of the snapshot: %v %+v", err, order.Snapshot.Quote.Lines)
	}

	// The same SKU bought directly: normal price, and its snapshot is byte-compatible (no price_rule key).
	d := e.direct(ltgQty)
	dq, dl := e.line(d, "")
	e.wantCatalog("direct purchase of the same SKU", dl)
	dres, err := e.placeHome(d, dq)
	if err != nil {
		t.Fatal(err)
	}
	var dtotal int64
	var dsnap []byte
	if err = e.p.f.owner.QueryRow(context.Background(), `SELECT total_minor,snapshot FROM checkout.orders WHERE id=$1`, dres.OrderID).Scan(&dtotal, &dsnap); err != nil {
		t.Fatal(err)
	}
	if dtotal != ltgQty*ltgCatalog+dq.Amount.ShippingMinor+dq.Amount.TaxMinor {
		t.Fatalf("direct order total %d", dtotal)
	}
	for _, key := range []string{"price_rule", "catalog_unit_price_minor", "claim_bundle_id", "claim_offer_id"} {
		if strings.Contains(string(dsnap), key) {
			t.Fatalf("the catalog order snapshot carries %q: %s", key, dsnap)
		}
	}

	// pay_at_pickup: a CONFIRMED order at the live total (the amount the collector will charge), snapshot carries the rule.
	e.cvsSettings(tcvAllChains, true, "20000", 500)
	code, _, _ := e.service("cvs_711", "MANUAL", 0)
	s2, o2 := e.session("B1", 12000, 5)
	bundle2, l2 := e.claimLink(s2, "bob", "B1+2")
	pb := e.redeemed(l2)
	pb.h.input.CartVersion = e.h.cartOf(t, pb.cap).Version
	pickup := e.tppEntered(pb, code)
	dest, err := pb.destination("cvs_711", pickup, tppName, tppPhone)
	if err != nil {
		t.Fatal(err)
	}
	pq, err := pb.quoteCodeFor(code, "")
	if err != nil {
		t.Fatalf("pay-at-pickup quote: %v", err)
	}
	e.wantLive("pay-at-pickup quote", pq.Lines[0], 12000, bundle2, o2.ID)
	pres, err := pb.begin(dest, pq, e.svcVer[code], "pay_at_pickup")
	if err != nil {
		t.Fatalf("pay-at-pickup placement: %v", err)
	}
	var ptotal int64
	var psnap []byte
	var state, mode string
	if err = e.p.f.owner.QueryRow(context.Background(), `SELECT total_minor,snapshot,commercial_state,payment_mode FROM checkout.orders WHERE id=$1`, pres.OrderID).Scan(&ptotal, &psnap, &state, &mode); err != nil {
		t.Fatal(err)
	}
	if ptotal != ltgQty*12000+pq.Amount.ShippingMinor+pq.Amount.TaxMinor || ptotal != pq.Amount.TotalMinor || state != "CONFIRMED" || mode != "pay_at_pickup" {
		t.Fatalf("pay-at-pickup order: total %d (quote %d) %s/%s", ptotal, pq.Amount.TotalMinor, state, mode)
	}
	if pl, _ := snapQuote(t, psnap); len(pl) != 1 || pl[0]["price_rule"] != "live_claim" || int64(pl[0]["unit_price_minor"].(float64)) != 12000 {
		t.Fatalf("pay-at-pickup snapshot lacks the price rule: %s", psnap)
	}
}

// ---------------------------------------------------------------------------------------------------------------------------------------
// LTG05
// ---------------------------------------------------------------------------------------------------------------------------------------

func TestLiveToolsGateLibraryImportCopy(t *testing.T) {
	e := ltgNew(t)
	f := e.p.f
	sku2 := e.sku(45000, 100)
	sku3 := e.sku(52000, 100)
	anchor := e.h.draft(t, e.store()) // any session of the store carries the library routes (accepted deviation 2)
	lib := func(tail string) string { return e.claimsPath(anchor, "/library"+tail) }

	// ---- library CRUD ----
	st, out := e.mjson(e.token(), "PUT", lib("/"+e.money), `{"keyword":"ｌａ１","expected_version":0}`) // full-width input is canonicalised
	if st != 200 || out["keyword"] != "LA1" || ltgNum(out, "version") != 1 || ltgNum(out, "sku_price_minor") != ltgCatalog {
		t.Fatalf("create library row: %d %v", st, out)
	}
	if st, out = e.mjson(e.token(), "PUT", lib("/"+sku2), `{"keyword":"LA1","expected_version":0}`); st != 409 {
		t.Fatalf("a keyword is unique per store: %d %v", st, out)
	}
	if st, _ = e.mjson(e.token(), "PUT", lib("/"+e.money), `{"keyword":"LA9","expected_version":0}`); st != 409 {
		t.Fatalf("version 0 on an existing row must conflict: %d", st)
	}
	if st, _ = e.mjson(e.token(), "PUT", lib("/"+e.money), `{"keyword":"BAD KEY!","expected_version":1}`); st != 422 {
		t.Fatalf("invalid keyword: %d", st)
	}
	if st, _ = e.mjson(e.token(), "PUT", lib("/"+randomUUID()), `{"keyword":"ZZ1","expected_version":0}`); st != 404 && st != 409 {
		t.Fatalf("unknown SKU: %d", st)
	}
	if st, out = e.mjson(e.token(), "PUT", lib("/"+sku2), `{"keyword":"LA2","expected_version":0}`); st != 200 {
		t.Fatalf("second row: %d %v", st, out)
	}
	if st, out = e.mjson(e.token(), "PUT", lib("/"+sku3), `{"keyword":"LA3","expected_version":0}`); st != 200 {
		t.Fatalf("third row: %d %v", st, out)
	}
	if st, out = e.mjson(e.token(), "PUT", lib("/"+e.money), `{"keyword":"LA1X","expected_version":1}`); st != 200 || out["keyword"] != "LA1X" || ltgNum(out, "version") != 2 {
		t.Fatalf("rename: %d %v", st, out)
	}
	if st, out = e.mjson(e.token(), "GET", lib(""), ""); st != 200 {
		t.Fatalf("list: %d %v", st, out)
	} else if entries, _ := out["entries"].([]any); len(entries) != 3 {
		t.Fatalf("list returned %v", out)
	}
	// the library is a template: no offer, no claim effect
	if n := e.count(`SELECT count(*) FROM live.offers WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store()); n != 0 {
		t.Fatalf("a library row created %d offers", n)
	}

	// ---- permissions and isolation ----
	reader, _ := e.member("live:read")
	nolive, _ := e.member("catalog:read")
	_, foreign := lcPrincipal(t, f, f.tenantB, []string{f.storeB}, "store:read", "live:read", "live:manage")
	if st, _ = e.mjson(reader, "GET", lib(""), ""); st != 200 {
		t.Fatalf("live:read lists the library: %d", st)
	}
	if st, _ = e.mjson(reader, "PUT", lib("/"+sku2), `{"keyword":"LA2","expected_version":1}`); st != 403 {
		t.Fatalf("live:read must not write the library: %d", st)
	}
	if st, _ = e.mjson(reader, "POST", e.claimsPath(anchor, "/offer-import"), `{"source":"library"}`); st != 403 {
		t.Fatalf("live:read must not import: %d", st)
	}
	if st, _ = e.mjson(nolive, "GET", lib(""), ""); st != 403 {
		t.Fatalf("without live:read: %d", st)
	}
	if st, _ = e.mjson(foreign, "GET", lib(""), ""); st != 404 {
		t.Fatalf("another tenant reads a store it cannot see: %d", st)
	}
	if st, _ = e.mjson(foreign, "PUT", lib("/"+sku2), `{"keyword":"LA2","expected_version":1}`); st != 404 {
		t.Fatalf("another tenant writes the library: %d", st)
	}
	if st, _ = e.mjson(e.token(), "GET", e.claimsPath(randomUUID(), "/library"), ""); st != 404 {
		t.Fatalf("the carrier session must exist in the store: %d", st)
	}
	if st, _ = e.mjson("", "GET", lib(""), ""); st != 401 {
		t.Fatalf("anonymous: %d", st)
	}
	// strict body: unknown keys and a missing version are refused
	if st, _ = e.mjson(e.token(), "PUT", lib("/"+sku2), `{"keyword":"LA2","expected_version":1,"live_price_minor":5}`); st != 400 && st != 422 {
		t.Fatalf("unknown key on the library write: %d", st)
	}
	if st, _ = e.mjson(e.token(), "PUT", lib("/"+sku2), `{"keyword":"LA2"}`); st != 400 && st != 422 {
		t.Fatalf("missing expected_version: %d", st)
	}

	// ---- import "library" into a session that already has conflicting offers ----
	s2 := e.h.draft(t, e.store())
	pre := e.h.livePriceOffer(t, s2, "LA2", e.money, 4, 600) // keyword LA2 is taken by ANOTHER SKU than the library's LA2 (sku2); sku0 already has an active offer
	st, out = e.mjson(e.token(), "POST", e.claimsPath(s2, "/offer-import"), `{"source":"library"}`)
	if st != 200 {
		t.Fatalf("conflicting import is data, not an error: %d %v", st, out)
	}
	conflicts := map[string]string{}
	for _, c := range out["conflicts"].([]any) {
		m := c.(map[string]any)
		conflicts[m["keyword"].(string)] = m["reason"].(string)
	}
	created := out["created"].([]any)
	if len(created) != 1 || created[0].(map[string]any)["keyword"] != "LA3" {
		t.Fatalf("only LA3 can be created: %v", out)
	}
	if conflicts["LA1X"] != "sku_taken" || conflicts["LA2"] != "keyword_taken" || len(conflicts) != 2 {
		t.Fatalf("conflict reasons: %v", conflicts)
	}
	if o := created[0].(map[string]any); ltgNum(o, "max_quantity_per_claim") != 3 || o["live_price_minor"] != nil || o["active"] != true { // accepted deviation 5
		t.Fatalf("seeded offer: %v", o)
	}
	for _, o := range e.boardOffers(s2) {
		if o.ID == pre.ID && (o.Version != pre.Version || o.LivePriceMinor == nil || *o.LivePriceMinor != 600 || o.MaxQuantityPerClaim != 4) {
			t.Fatalf("import overwrote an existing offer: %+v", o)
		}
	}

	// ---- import into an empty session: all created, no price; replay returns the same result; a NEW import creates nothing ----
	s3 := e.h.draft(t, e.store())
	key := t04Key("ltg-imp")
	body := `{"source":"library"}`
	st1, raw1 := func() (int, []byte) {
		st, _, raw := e.mcall(e.token(), "POST", e.claimsPath(s3, "/offer-import"), key, body)
		return st, raw
	}()
	st2, raw2 := func() (int, []byte) {
		st, _, raw := e.mcall(e.token(), "POST", e.claimsPath(s3, "/offer-import"), key, body)
		return st, raw
	}()
	if st1 != 200 || st2 != 200 {
		t.Fatalf("import / replay: %d %d %s", st1, st2, raw1)
	}
	var r1, r2 struct {
		Created []map[string]any `json:"created"`
	}
	_ = json.Unmarshal(raw1, &r1)
	_ = json.Unmarshal(raw2, &r2)
	if len(r1.Created) != 3 || len(r2.Created) != 3 {
		t.Fatalf("created %d / replay %d, want 3 each: %s", len(r1.Created), len(r2.Created), raw1)
	}
	for i := range r1.Created {
		if r1.Created[i]["offer_id"] != r2.Created[i]["offer_id"] || r1.Created[i]["live_price_minor"] != nil {
			t.Fatalf("replay differs or seeded a price: %v vs %v", r1.Created[i], r2.Created[i])
		}
	}
	if n := e.count(`SELECT count(*) FROM live.offers WHERE session_id=$1`, s3); n != 3 {
		t.Fatalf("replay duplicated offers: %d", n)
	}
	if st, _, _ = e.mcall(e.token(), "POST", e.claimsPath(s3, "/offer-import"), key, `{"source":"library","from_session_id":"`+s2+`"}`); st != 409 && st != 422 {
		t.Fatalf("same key, different body must be refused: %d", st)
	}
	st, out = e.mjson(e.token(), "POST", e.claimsPath(s3, "/offer-import"), body)
	if st != 200 || len(out["created"].([]any)) != 0 || len(out["conflicts"].([]any)) != 3 {
		t.Fatalf("re-import: %d %v", st, out)
	}
	for _, c := range out["conflicts"].([]any) {
		if r := c.(map[string]any)["reason"]; r != "keyword_taken" && r != "already_present" { // contract: keyword_taken for any SKU; the implementation refines it to already_present (DEFECTS.md, doc divergence)
			t.Fatalf("re-import conflict reason %v", r)
		}
	}

	// ---- copy from session: active offers only, price never copied, source untouched ----
	st, out = e.mjson(e.token(), "PATCH", e.claimsPath(s3, "/offers/"+r1.Created[0]["offer_id"].(string)), fmt.Sprintf(`{"expected_version":%d,"max_quantity_per_claim":2,"active":false,"live_price_minor":7000}`, int64(r1.Created[0]["version"].(float64))))
	if st != 200 {
		t.Fatalf("deactivate source offer: %d %v", st, out)
	}
	s4 := e.h.draft(t, e.store())
	st, out = e.mjson(e.token(), "POST", e.claimsPath(s4, "/offer-import"), `{"source":"session","from_session_id":"`+s3+`"}`)
	if st != 200 || len(out["created"].([]any)) != 2 {
		t.Fatalf("copy of the 2 active offers: %d %v", st, out)
	}
	for _, c := range out["created"].([]any) {
		if c.(map[string]any)["live_price_minor"] != nil {
			t.Fatalf("a live price was copied between sessions: %v", c)
		}
	}
	if st, _ = e.mjson(e.token(), "POST", e.claimsPath(s4, "/offer-import"), `{"source":"session","from_session_id":"`+s4+`"}`); st != 422 {
		t.Fatalf("copy from itself: %d", st)
	}
	if st, _ = e.mjson(e.token(), "POST", e.claimsPath(s4, "/offer-import"), `{"source":"session"}`); st != 422 {
		t.Fatalf("copy without a source session: %d", st)
	}
	if st, _ = e.mjson(e.token(), "POST", e.claimsPath(s4, "/offer-import"), `{"source":"library","from_session_id":"`+s3+`"}`); st != 422 {
		t.Fatalf("library with a source session: %d", st)
	}
	if st, _ = e.mjson(e.token(), "POST", e.claimsPath(s4, "/offer-import"), `{"source":"nope"}`); st != 422 {
		t.Fatalf("unknown source: %d", st)
	}
	if st, _ = e.mjson(e.token(), "POST", e.claimsPath(s4, "/offer-import"), `{"source":"session","from_session_id":"`+randomUUID()+`"}`); st != 404 {
		t.Fatalf("unknown source session: %d", st)
	}
	if st, _ = e.mjson(foreign, "POST", e.claimsPath(s4, "/offer-import"), `{"source":"library"}`); st != 404 {
		t.Fatalf("another tenant imports into this session: %d", st)
	}
	// the source session still holds its price on the offer we deactivated (copy never mutates the source)
	if o := func() claims.Offer {
		for _, o := range e.boardOffers(s3) {
			if o.ID == r1.Created[0]["offer_id"] {
				return o
			}
		}
		return claims.Offer{}
	}(); o.LivePriceMinor == nil || *o.LivePriceMinor != 7000 || o.Active {
		t.Fatalf("copy changed the source offer: %+v", o)
	}

	// ---- sku_unavailable: an archived SKU in the library is reported, never imported ----
	mustExec(t, f.owner, `UPDATE catalog.skus SET status='archived' WHERE id=$1`, sku3)
	s5 := e.h.draft(t, e.store())
	st, out = e.mjson(e.token(), "POST", e.claimsPath(s5, "/offer-import"), `{"source":"library"}`)
	if st != 200 || len(out["created"].([]any)) != 2 {
		t.Fatalf("import with an archived SKU: %d %v", st, out)
	}
	cf := out["conflicts"].([]any)
	if len(cf) != 1 || cf[0].(map[string]any)["reason"] != "sku_unavailable" || cf[0].(map[string]any)["sku_id"] != sku3 {
		t.Fatalf("archived SKU conflict: %v", cf)
	}
	mustExec(t, f.owner, `UPDATE catalog.skus SET status='active' WHERE id=$1`, sku3)

	// ---- session_full: 199 offers + 2 library rows = 1 created, 1 session_full ----
	many := lcSKUs(t, f, e.tenant(), e.store(), "TWD", 199)
	full := e.h.draft(t, e.store())
	for i, sku := range many {
		e.h.offer(t, full, fmt.Sprintf("F%03d", i), sku, 1)
	}
	st, out = e.mjson(e.token(), "POST", e.claimsPath(full, "/offer-import"), `{"source":"library"}`)
	if st != 200 || len(out["created"].([]any)) != 1 || len(out["conflicts"].([]any)) != 2 {
		t.Fatalf("import into a nearly full session (199 offers, 3 library rows): %d created=%d conflicts=%v", st, len(out["created"].([]any)), out["conflicts"])
	}
	reasons := map[string]int{}
	for _, c := range out["conflicts"].([]any) {
		reasons[c.(map[string]any)["reason"].(string)]++
	}
	if reasons["session_full"] != 2 {
		t.Fatalf("session_full not reported: %v", reasons)
	}
	if n := e.count(`SELECT count(*) FROM live.offers WHERE session_id=$1`, full); n != 200 {
		t.Fatalf("the cap is 200 offers, session holds %d", n)
	}

	// ---- audit: one import = one audit row ----
	if n := e.count(`SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action='live.claim.offers.imported'`, e.tenant(), e.store()); n < 4 {
		t.Fatalf("import audit rows: %d", n)
	}

	// ---- removing a library row: needs the current version, then it is gone ----
	if st, _ = e.mjson(e.token(), "PUT", lib("/"+sku2), `{"keyword":"","expected_version":99}`); st != 409 {
		t.Fatalf("stale remove: %d", st)
	}
	if st, out = e.mjson(e.token(), "PUT", lib("/"+sku2), `{"keyword":"","expected_version":1}`); st != 200 || out["keyword"] != "" && out["keyword"] != nil {
		t.Fatalf("remove: %d %v", st, out)
	}
	if n := e.count(`SELECT count(*) FROM live.keyword_library WHERE store_id=$1 AND sku_id=$2`, e.store(), sku2); n != 0 {
		t.Fatalf("library row survived its removal")
	}
}

// ---------------------------------------------------------------------------------------------------------------------------------------
// LTG06
// ---------------------------------------------------------------------------------------------------------------------------------------

func TestLiveToolsGateConcurrentOfferCreation(t *testing.T) {
	e := ltgNew(t)
	skus := lcSKUs(t, e.p.f, e.tenant(), e.store(), "TWD", 6)
	const racers = 8

	statuses := func(fn func(i int) (int, map[string]any)) (ok, conflict, other int, bodies []map[string]any) {
		var mu sync.Mutex
		var wg sync.WaitGroup
		start := make(chan struct{})
		for i := 0; i < racers; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				st, out := fn(i)
				mu.Lock()
				defer mu.Unlock()
				switch {
				case st == 200:
					ok++
					bodies = append(bodies, out)
				case st == 409:
					conflict++
				default:
					other++
					t.Errorf("racer %d answered %d %v", i, st, out)
				}
			}(i)
		}
		close(start)
		wg.Wait()
		return
	}

	// (a) same keyword, same SKU, same session: exactly one offer wins, everybody else is a plain 409.
	sa := e.h.draft(t, e.store())
	ok, conflict, other, _ := statuses(func(i int) (int, map[string]any) {
		return e.mjson(e.token(), "POST", e.claimsPath(sa, "/offers"), fmt.Sprintf(`{"keyword":"RACE","sku_id":%q,"max_quantity_per_claim":2,"live_price_minor":%d}`, skus[0], 1000+i))
	})
	if ok != 1 || conflict != racers-1 || other != 0 {
		t.Fatalf("same keyword race: %d ok %d conflict %d other", ok, conflict, other)
	}
	if n := e.count(`SELECT count(*) FROM live.offers WHERE session_id=$1`, sa); n != 1 {
		t.Fatalf("race left %d offers", n)
	}

	// (b) different keywords, same SKU: still one active offer per SKU in a session.
	sb := e.h.draft(t, e.store())
	ok, conflict, other, _ = statuses(func(i int) (int, map[string]any) {
		return e.mjson(e.token(), "POST", e.claimsPath(sb, "/offers"), fmt.Sprintf(`{"keyword":"SK%d","sku_id":%q,"max_quantity_per_claim":2}`, i, skus[1]))
	})
	if ok != 1 || conflict != racers-1 || other != 0 {
		t.Fatalf("same SKU race: %d ok %d conflict %d other", ok, conflict, other)
	}

	// (c) concurrent identical imports under DIFFERENT keys: the library is seeded exactly once, no duplicate keyword, no 5xx.
	for i, sku := range skus[2:5] {
		if st, out := e.mjson(e.token(), "PUT", e.claimsPath(sa, "/library/"+sku), fmt.Sprintf(`{"keyword":"IM%d","expected_version":0}`, i)); st != 200 {
			t.Fatalf("library row %d: %d %v", i, st, out)
		}
	}
	sc := e.h.draft(t, e.store())
	var created sync.Map
	ok, conflict, other, bodies := statuses(func(i int) (int, map[string]any) {
		st, out := e.mjson(e.token(), "POST", e.claimsPath(sc, "/offer-import"), `{"source":"library"}`)
		if st == 200 {
			for _, c := range out["created"].([]any) {
				if _, dup := created.LoadOrStore(c.(map[string]any)["keyword"], i); dup {
					t.Errorf("keyword %v created by two concurrent imports", c.(map[string]any)["keyword"])
				}
			}
		}
		return st, out
	})
	if other != 0 || conflict != 0 || ok != racers { // contract rule 2: conflicts are DATA (HTTP 200), a race loser is never a 409
		t.Fatalf("concurrent imports: ok=%d conflict(409)=%d other=%d, want every import to answer 200", ok, conflict, other)
	}
	total := 0
	for _, b := range bodies {
		total += len(b["created"].([]any))
	}
	if total != 3 || e.count(`SELECT count(*) FROM live.offers WHERE session_id=$1`, sc) != 3 {
		t.Fatalf("library seeded %d times (offers %d), want exactly 3", total, e.count(`SELECT count(*) FROM live.offers WHERE session_id=$1`, sc))
	}
	var kws []string
	rows, _ := e.p.f.owner.Query(context.Background(), `SELECT keyword FROM live.offers WHERE session_id=$1 ORDER BY keyword`, sc)
	for rows.Next() {
		var k string
		_ = rows.Scan(&k)
		kws = append(kws, k)
	}
	rows.Close()
	sort.Strings(kws)
	if strings.Join(kws, ",") != "IM0,IM1,IM2" {
		t.Fatalf("seeded keywords %v", kws)
	}

	// (d) a manual create racing an import on the same keyword/SKU: exactly one offer for IM0, the loser is a conflict datum or 409, never a 5xx.
	sd := e.h.draft(t, e.store())
	var wg sync.WaitGroup
	var stCreate int
	var importOut map[string]any
	var stImport int
	start := make(chan struct{})
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		stCreate, _ = e.mjson(e.token(), "POST", e.claimsPath(sd, "/offers"), fmt.Sprintf(`{"keyword":"IM0","sku_id":%q,"max_quantity_per_claim":2,"live_price_minor":9000}`, skus[2]))
	}()
	go func() {
		defer wg.Done()
		<-start
		stImport, importOut = e.mjson(e.token(), "POST", e.claimsPath(sd, "/offer-import"), `{"source":"library"}`)
	}()
	close(start)
	wg.Wait()
	if (stCreate != 200 && stCreate != 409) || stImport != 200 {
		t.Fatalf("create %d import %d %v", stCreate, stImport, importOut)
	}
	if n := e.count(`SELECT count(*) FROM live.offers WHERE session_id=$1 AND (keyword='IM0' OR sku_id=$2)`, sd, skus[2]); n != 1 {
		t.Fatalf("IM0/sku race left %d offers", n)
	}
	if n := e.count(`SELECT count(*) FROM live.offers WHERE session_id=$1`, sd); n != 3 {
		t.Fatalf("session holds %d offers after create+import, want 3 (IM0 once, IM1, IM2)", n)
	}
	// if the manual create won, its live price is intact (the import never overwrites it)
	if stCreate == 200 {
		if n := e.count(`SELECT count(*) FROM live.offers WHERE session_id=$1 AND keyword='IM0' AND live_price_minor=9000`, sd); n != 1 {
			t.Fatalf("the import overwrote the manual offer's price")
		}
	}
}

// ---------------------------------------------------------------------------------------------------------------------------------------
// LTG09
// ---------------------------------------------------------------------------------------------------------------------------------------

// Database role boundaries of migration 0092 (amendment rules 1, 3, 4, 5): the buyer role reads no live.* table and writes no price, the merchant role
// cannot call the buyer price functions, and the library is invisible across tenants even to a raw SELECT under another tenant's GUCs.
func TestLiveToolsGateRoleBoundaries(t *testing.T) {
	e := ltgNew(t)
	f := e.p.f
	ctx := context.Background()
	s1, _ := e.session("A1", ltgLive, 5)
	_, l1 := e.claimLink(s1, "amy", "A1+2")
	b := e.redeemed(l1)
	if st, out := e.mjson(e.token(), "PUT", e.claimsPath(s1, "/library/"+e.money), `{"keyword":"BND1","expected_version":0}`); st != 200 {
		t.Fatalf("library row: %d %v", st, out)
	}

	// buyer role: no read of the library or of an offer's price, no write of a price or of the library, SQLSTATE 42501 each
	for name, q := range map[string]string{
		"read library":      `SELECT count(*) FROM live.keyword_library`,
		"read offer price":  `SELECT live_price_minor FROM live.offers LIMIT 1`,
		"write offer price": `UPDATE live.offers SET live_price_minor=1`,
		"write library":     `DELETE FROM live.keyword_library`,
	} {
		err := buyer.WithScope(ctx, e.p.a.runtime, b.cap.Token, e.store(), func(c context.Context, tx pgx.Tx, s buyer.Scope) error {
			_, e := tx.Exec(c, q)
			return e
		})
		requirePGCode(t, err, "42501", "buyer role: "+name)
	}
	// merchant role: the buyer-only display function is refused
	err := e.h.do(e.token(), e.store(), func(tx pgx.Tx, s platform.Scope) error {
		_, e := tx.Exec(ctx, `SELECT * FROM claims.preview_live_prices(decode(repeat('00',32),'hex'))`)
		return e
	})
	requirePGCode(t, err, "42501", "merchant EXECUTE on claims.preview_live_prices")
	// RLS: under another tenant's scope the library is empty, even for a raw SELECT; the owner store sees its one row
	_, otherToken := lcPrincipal(t, f, f.tenantB, []string{f.storeB}, "store:read", "live:read", "live:manage")
	var seen int
	if err = platform.WithScope(ctx, f.runtime, otherToken, f.storeB, "store:read", func(tx pgx.Tx, s platform.Scope) error {
		return tx.QueryRow(ctx, `SELECT count(*) FROM live.keyword_library`).Scan(&seen)
	}); err != nil || seen != 0 {
		t.Fatalf("another tenant sees %d library rows (%v)", seen, err)
	}
	var mine int
	if err = e.h.do(e.token(), e.store(), func(tx pgx.Tx, s platform.Scope) error {
		return tx.QueryRow(ctx, `SELECT count(*) FROM live.keyword_library`).Scan(&mine)
	}); err != nil || mine != 1 {
		t.Fatalf("the owner store sees %d library rows (%v), want 1", mine, err)
	}
}

// ---------------------------------------------------------------------------------------------------------------------------------------
// LTG07
// ---------------------------------------------------------------------------------------------------------------------------------------

func TestLiveToolsGatePromotionOnLivePrice(t *testing.T) {
	e := ltgNew(t)
	s1, o1 := e.session("A1", ltgLive, 5)
	bundle1, l1 := e.claimLink(s1, "amy", "A1+2")
	live := e.redeemed(l1)
	direct := e.direct(ltgQty)
	const liveSub, catSub = ltgQty * ltgLive, ltgQty * ltgCatalog // 40000 and 60000

	e.proMust("SAVE10", nil) // percent 10
	q, l := e.line(live, " save10 ")
	e.wantLive("live line with a code", l, ltgLive, bundle1, o1.ID)
	if q.Amount.SubtotalMinor != liveSub || q.Amount.DiscountMinor != 4000 || l.Amount.DiscountMinor != 4000 { // 10 % of the LIVE subtotal, once
		t.Fatalf("live + SAVE10: subtotal %d discount %d line discount %d, want %d / 4000", q.Amount.SubtotalMinor, q.Amount.DiscountMinor, l.Amount.DiscountMinor, liveSub)
	}
	if q.Amount.TotalMinor != liveSub-4000+q.Amount.ShippingMinor+q.Amount.TaxMinor {
		t.Fatalf("total %d is not subtotal - discount + shipping + tax: %+v", q.Amount.TotalMinor, q.Amount)
	}
	if q.Promotion == nil || q.Promotion.Code != "SAVE10" {
		t.Fatalf("snapshot promotion: %+v", q.Promotion)
	}
	// The same code on the SAME SKU bought directly discounts the catalog subtotal (the contrast that proves the live price is the base).
	dq, dl := e.line(direct, "SAVE10")
	e.wantCatalog("direct + SAVE10", dl)
	if dq.Amount.SubtotalMinor != catSub || dq.Amount.DiscountMinor != 6000 {
		t.Fatalf("direct + SAVE10: subtotal %d discount %d", dq.Amount.SubtotalMinor, dq.Amount.DiscountMinor)
	}
	if q.Amount.TotalMinor >= dq.Amount.TotalMinor {
		t.Fatalf("the live buyer must pay less than the direct buyer: %d vs %d", q.Amount.TotalMinor, dq.Amount.TotalMinor)
	}

	// A fixed code is capped at the LIVE merchandise subtotal: goods never go below zero, and the discount is not capped at the catalog subtotal.
	e.proMust("BIGFIX", map[string]any{"kind": "fixed", "percent": nil, "fixed_minor": 99999900})
	fq, _ := e.line(live, "bigfix")
	if fq.Amount.DiscountMinor != liveSub || fq.Amount.TotalMinor != fq.Amount.ShippingMinor+fq.Amount.TaxMinor {
		t.Fatalf("fixed code on the live subtotal: discount %d total %d (live subtotal %d)", fq.Amount.DiscountMinor, fq.Amount.TotalMinor, liveSub)
	}
	// min_subtotal compares the PRE-discount subtotal that is actually quoted: a code needing 50000 refuses the live cart (40000) and accepts the direct one (60000).
	e.proMust("MIN50", map[string]any{"min_subtotal_minor": 50000})
	if _, err := e.quoteOf(live, "min50"); err == nil || proCode(err) != "promo_min_subtotal" {
		t.Fatalf("min_subtotal on a live cart below the minimum: %v", err)
	}
	if _, err := e.quoteOf(direct, "min50"); err != nil {
		t.Fatalf("min_subtotal on the catalog cart above the minimum: %v", err)
	}

	// Placement: promotions.redeem accepts the live-priced snapshot (no promo_changed), the order total is the discounted LIVE total, the redemption row exists once.
	id, _ := e.proMust("ORDER10", map[string]any{"total_limit": 5})
	oq, _ := e.line(live, "order10")
	res, err := e.placeHome(live, oq)
	if err != nil {
		t.Fatalf("placement of a live-priced + discounted quote: %v", err)
	}
	var total int64
	var snapshot []byte
	if err = e.p.f.owner.QueryRow(context.Background(), `SELECT total_minor,snapshot FROM checkout.orders WHERE id=$1`, res.OrderID).Scan(&total, &snapshot); err != nil {
		t.Fatal(err)
	}
	sl, sq := snapQuote(t, snapshot)
	if total != oq.Amount.TotalMinor || oq.Amount.DiscountMinor != 4000 || len(sl) != 1 || sl[0]["price_rule"] != "live_claim" || sq["promotion"] == nil {
		t.Fatalf("order total %d (quote %d, discount %d); snapshot must carry both the live rule and the promotion: %s", total, oq.Amount.TotalMinor, oq.Amount.DiscountMinor, snapshot)
	}
	if n := e.proUsed(id); n != 1 {
		t.Fatalf("redemptions %d, want exactly 1 (a discount is never applied twice)", n)
	}
	if n := e.count(`SELECT count(*) FROM promotions.redemptions WHERE order_id=$1`, res.OrderID); n != 1 {
		t.Fatalf("redemption rows for the order: %d", n)
	}

	// Once the link expires the same code discounts the catalog price: the live price is gone, the code still applies once.
	e.setLinkExpiry(bundle1, time.Second)
	time.Sleep(1500 * time.Millisecond)
	xq, xl := e.line(live, "save10")
	e.wantCatalog("expired link with a code", xl)
	if xq.Amount.SubtotalMinor != catSub || xq.Amount.DiscountMinor != 6000 {
		t.Fatalf("expired + SAVE10: subtotal %d discount %d", xq.Amount.SubtotalMinor, xq.Amount.DiscountMinor)
	}
	// A quote priced live with a code cannot be placed after expiry (fail closed; the code's use is not burnt).
	e.setLinkExpiry(bundle1, 2*time.Second)
	s2q, _ := e.line(live, "order10")
	time.Sleep(2500 * time.Millisecond)
	before := e.proUsed(id)
	if _, err := e.placeHome(live, s2q); !errors.Is(err, command.ErrConflict) {
		t.Fatalf("a live-priced, discounted quote must fail closed as a conflict after the link expired: %v", err)
	}
	if e.proUsed(id) != before {
		t.Fatal("a refused placement burnt a code use")
	}
}

// ---------------------------------------------------------------------------------------------------------------------------------------
// LTG08
// ---------------------------------------------------------------------------------------------------------------------------------------

func TestLiveToolsGateRefundsCappedAtPaid(t *testing.T) {
	e := ltgNew(t, tcvOpts{stripe: true})
	e.r.startWorker(t)
	s1, o1 := e.session("A1", ltgLive, 5)
	bundle1, l1 := e.claimLink(s1, "amy", "A1+2")
	b := e.redeemed(l1)
	e.proMust("SAVE10", nil)
	q, l := e.line(b, "save10")
	e.wantLive("quote", l, ltgLive, bundle1, o1.ID)
	const paid = ltgQty*ltgLive - 4000 // 36000 with a zero delivery fee and no tax
	if q.Amount.TotalMinor != paid {
		t.Fatalf("quote total %d, want %d (live subtotal 40000 - 10%%)", q.Amount.TotalMinor, paid)
	}
	hold, err := e.placeHome(b, q)
	if err != nil {
		t.Fatalf("hold: %v", err)
	}
	o := e.payHold(hold, b)
	if o.captured != paid {
		t.Fatalf("captured %d, want the live+discounted total %d (not the catalog total %d)", o.captured, paid, ltgQty*ltgCatalog)
	}
	if got := e.r.refundable(t, o); got != paid {
		t.Fatalf("refundable %d, want what was paid %d", got, paid)
	}
	// Refunding "what the catalog price would have been" is above what was paid: refused, nothing stored.
	st, out := e.r.request(o, o.token(), t04Key("ltg-rf-cat"), rfxBody(ltgQty*ltgCatalog, "requested_by_customer", paid))
	if st != 422 || out["code"] != "exceeds_refundable" {
		t.Fatalf("refund of the catalog total: %d %v", st, out)
	}
	if st, out = e.r.request(o, o.token(), t04Key("ltg-rf-over"), rfxBody(paid+100, "requested_by_customer", paid)); st != 422 || out["code"] != "exceeds_refundable" {
		t.Fatalf("refund one dollar over paid: %d %v", st, out)
	}
	if n := e.count(`SELECT count(*) FROM payments.stripe_refunds WHERE attempt_id=$1`, o.attempt); n != 0 {
		t.Fatalf("a refused refund stored %d rows", n)
	}
	// Partial then exact remainder succeed; the remainder +1 dollar and anything after zero are refused.
	first := e.r.mustRefund(t, o, 10000, "requested_by_customer")
	_ = first
	if got := e.r.refundable(t, o); got != paid-10000 {
		t.Fatalf("refundable after a partial %d, want %d", got, paid-10000)
	}
	if st, out = e.r.request(o, o.token(), t04Key("ltg-rf-rem"), rfxBody(paid-10000+100, "requested_by_customer", paid-10000)); st != 422 || out["code"] != "exceeds_refundable" {
		t.Fatalf("refund above the remainder: %d %v", st, out)
	}
	e.r.mustRefund(t, o, paid-10000, "requested_by_customer")
	if got := e.r.refundable(t, o); got != 0 {
		t.Fatalf("fully refunded order still refundable: %d", got)
	}
	if st, out = e.r.request(o, o.token(), t04Key("ltg-rf-zero"), rfxBody(100, "requested_by_customer", 0)); st != 422 {
		t.Fatalf("refund after the full amount: %d %v", st, out)
	}
	var sum int64
	if err = e.p.f.owner.QueryRow(context.Background(), `SELECT coalesce(sum(amount_minor),0)::bigint FROM payments.stripe_refunds WHERE attempt_id=$1`, o.attempt).Scan(&sum); err != nil || sum != paid {
		t.Fatalf("requested refunds sum to %d (%v), want exactly the paid amount %d", sum, err, paid)
	}
}
