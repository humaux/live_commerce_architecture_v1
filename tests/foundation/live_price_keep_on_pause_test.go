// Purpose: REAL_PG gate of owner decision 2026-10-07 "already-claimed buyers keep the live price when a live offer is paused, only new claims are refused"
//   (contract amendment "Live price kept on pause"): cases (a) to (g) of the unit brief plus the one-claim-per-SKU rule that keeps typo recovery working.
// Depends on: the ltg harness (live_tools_gate_test.go: ltgNew, session, claimLink, redeemed, line, wantLive, wantCatalog, placeHome, held, reCart, mjson),
//   lcHarness preview/redeem/claim; claims.live_prices, claims.preview_live_prices, claims.consume_live_prices (migration 0158 changed the first two).
// Used by: scripts/dev/test-focused.sh '^TestLivePriceKeepOnPause'; CI foundation suite.
// Invariants: live-keyword-claims-v1 amendment "Live price kept on pause" rules 1 to 6; "Live tools (R4)" rule 7 (cap per claim line, expiry, no direct-cart path).
// Status: REAL_PG, MOCK (no PSP, no Meta).

package foundation_test

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"livecommerce/internal/claims"
	"livecommerce/internal/command"
)

// expireLinks moves every link of the bundle into the past inside the 72 h TTL CHECK (owner fixture).
func (e *ltgEnv) expireLinks(bundle string) {
	e.t.Helper()
	mustExec(e.t, e.p.f.owner, `UPDATE claims.links SET issued_at=clock_timestamp()-interval '80 hours',expires_at=clock_timestamp()-interval '10 hours' WHERE bundle_id=$1`, bundle)
}

// setOfferActive drives the real merchant PATCH route (M4) and return the new offer version.
func (e *ltgEnv) setOfferActive(session string, o claims.Offer, version int64, active bool) int64 {
	e.t.Helper()
	st, out := e.mjson(e.token(), "PATCH", e.claimsPath(session, "/offers/"+o.ID), fmt.Sprintf(`{"expected_version":%d,"max_quantity_per_claim":5,"active":%t}`, version, active))
	if st != 200 || out["active"] != active {
		e.t.Fatalf("set offer active=%t: %d %v", active, st, out)
	}
	return ltgNum(out, "version")
}

// TestLivePriceKeepOnPauseExistingClaims is the money gate: after the merchant pauses the offer, claimants keep exactly the live price and quantity they
// were granted (quote, link display, redeem, checkout), and nobody else gains anything.
func TestLivePriceKeepOnPauseExistingClaims(t *testing.T) {
	e := ltgNew(t)
	s1, o1 := e.session("A1", ltgLive, 5)
	// Claimants before the pause (every claim needs an OPEN window; claimLink closes it again).
	amyBundle, amyLink := e.claimLink(s1, "amy", "A1+2") // opened: the line is already in her cart
	amy := e.redeemed(amyLink)
	e.h.open(t, s1, claims.MatchExact)
	danBundle, danLink := e.claimLink(s1, "dan", "A1+2") // opened: will place an order while the offer is paused
	dan := e.redeemed(danLink)
	e.h.open(t, s1, claims.MatchExact)
	bobBundle, bobLink := e.claimLink(s1, "bob", "A1+1") // holds a link and has NOT opened it
	e.h.open(t, s1, claims.MatchExact)
	eveBundle, eveLink := e.claimLink(s1, "eve", "A1+1") // unopened link that will expire
	e.h.open(t, s1, claims.MatchExact)
	gusBundle, gusLink := e.claimLink(s1, "gus", "A1+1") // opened, link will expire
	gus := e.redeemed(gusLink)
	_, l := e.line(amy, "")
	e.wantLive("baseline before the pause", l, ltgLive, amyBundle, o1.ID)

	// The merchant pauses the offer (the live price on the offer is untouched).
	v := e.setOfferActive(s1, o1, o1.Version, false)

	// (a) NEW claims on the paused offer are refused, from a new buyer and from an existing claimant (her claimed quantity stays 2).
	e.h.open(t, s1, claims.MatchExact)
	if r := e.h.claim(t, s1, claims.ManualClaimInput{ActorLabel: "newbie", Text: "A1+1"}); r.Outcome != claims.OutcomeRejected || r.Reason != claims.ReasonOfferInactive || r.BundleID != "" {
		t.Fatalf("(a) a new claim on a paused offer must be refused as OFFER_INACTIVE: %+v", r)
	}
	if r := e.h.claim(t, s1, claims.ManualClaimInput{BundleID: amyBundle, Text: "A1+4"}); r.Outcome != claims.OutcomeRejected || r.Reason != claims.ReasonOfferInactive {
		t.Fatalf("(a) an existing claimant cannot raise her claim on a paused offer: %+v", r)
	}
	e.h.closeWindow(t, s1)
	if q := e.count(`SELECT quantity FROM claims.lines WHERE bundle_id=$1 AND offer_id=$2`, amyBundle, o1.ID); q != 2 {
		t.Fatalf("(a) amy's claimed quantity changed to %d", q)
	}
	if n := e.count(`SELECT count(*) FROM claims.bundles WHERE session_id=$1`, s1); n != 5 {
		t.Fatalf("(a) the refused claim created a bundle: %d bundles, want 5", n)
	}

	// (b) amy, who has the line in her cart, is quoted the live price (exact price, exact quantity, the granted origin).
	_, lAmy := e.line(amy, "")
	e.wantLive("(b) quote of a claimed line while the offer is paused", lAmy, ltgLive, amyBundle, o1.ID)
	if lAmy.Quantity != 2 {
		t.Fatalf("(b) quote quantity %d, want 2", lAmy.Quantity)
	}
	// ... and her link display still shows the live price next to the catalog price.
	pv, err := e.h.preview(amy.cap, amyLink.Token)
	if err != nil || len(pv.Lines) != 1 || pv.Lines[0].UnitPriceMinor != ltgLive || pv.Lines[0].PriceRule != "live_claim" || pv.Lines[0].CatalogUnitPriceMinor != ltgCatalog {
		t.Fatalf("(b) preview of a claimed line while paused: %+v %v", pv, err)
	}

	// Checkout begin: dan's live-priced quote places at the live price, the ledger records the claimed units, and the same claim cannot be used again.
	qDan, lDan := e.line(dan, "")
	e.wantLive("dan quote while paused", lDan, ltgLive, danBundle, o1.ID)
	res, err := e.placeHome(dan, qDan)
	if err != nil {
		t.Fatalf("(b) a live quote of a paused offer must place: %v", err)
	}
	if got := e.total(res.OrderID); got != qDan.Amount.TotalMinor || got != ltgQty*ltgLive+qDan.Amount.ShippingMinor+qDan.Amount.TaxMinor {
		t.Fatalf("(b) order total %d, want 2 x live price %d plus shipping/tax (quote total %d)", got, ltgLive, qDan.Amount.TotalMinor)
	}
	if qty, rows := e.held(danBundle, o1.ID); qty != int(ltgQty) || rows != 1 {
		t.Fatalf("(b) ledger for the placed order: quantity %d rows %d, want 2 / 1", qty, rows)
	}
	e.reCart(dan, ltgQty) // the cap is per claim line across all orders: the claimed units are used up
	_, lDan = e.line(dan, "")
	e.wantCatalog("(d) a second order of an already-ordered claim line", lDan)

	// (c) bob opens his UNOPENED link while the offer is paused: the line is applied (not skipped as offer_inactive) and priced live.
	bob := e.buyerCap()
	pvBob, err := e.h.preview(bob.cap, bobLink.Token)
	if err != nil || len(pvBob.Lines) != 1 || !pvBob.Lines[0].Pending || !pvBob.Lines[0].Available || pvBob.Lines[0].UnitPriceMinor != ltgLive || pvBob.Lines[0].PriceRule != "live_claim" {
		t.Fatalf("(c) preview of an unopened link of a paused offer: %+v %v", pvBob, err)
	}
	redeemed, err := e.h.redeem(bob.cap, t04Key("kop-bob"), bobLink.Token, pvBob.BundleVersion)
	if err != nil || len(redeemed.Skipped) != 0 || !reflect.DeepEqual(lcItems(redeemed.Applied), map[string]int64{e.money: 1}) || !reflect.DeepEqual(lcItems(redeemed.Cart.Items), map[string]int64{e.money: 1}) {
		t.Fatalf("(c) redeem of an unopened link of a paused offer: %+v %v", redeemed, err)
	}
	qBob, lBob := e.line(bob, "")
	e.wantLive("(c) bob quote after redeeming while paused", lBob, ltgLive, bobBundle, o1.ID)
	if lBob.Quantity != 1 {
		t.Fatalf("(c) bob quantity %d, want 1", lBob.Quantity)
	}
	resBob, err := e.placeHome(bob, qBob)
	if err != nil || e.total(resBob.OrderID) != qBob.Amount.TotalMinor || e.total(resBob.OrderID) != ltgLive+qBob.Amount.ShippingMinor+qBob.Amount.TaxMinor {
		t.Fatalf("(c) bob's order at the live price: %v total %d want %d plus shipping/tax", err, e.total(resBob.OrderID), ltgLive)
	}
	if qty, rows := e.held(bobBundle, o1.ID); qty != 1 || rows != 1 {
		t.Fatalf("(c) ledger for bob's order: quantity %d rows %d, want 1 / 1", qty, rows)
	}

	// (d) more than the claimed quantity is never at the live price: through the cart API (origin dropped) and through a forged cart row.
	e.reCart(amy, ltgQty+1)
	_, l = e.line(amy, "")
	e.wantCatalog("(d) 3 units of a 2-unit claim through the cart", l)
	if err := e.rawCartLine(amy, e.money, ltgQty+1, amyBundle, o1.ID, ltgQty+1); err != nil {
		t.Fatalf("(d) forgery setup: %v", err)
	}
	_, l = e.line(amy, "")
	e.wantCatalog("(d) forged claim_quantity 3 on a 2-unit claim of a paused offer", l)
	if err := e.rawCartLine(amy, e.money, ltgQty, amyBundle, o1.ID, ltgQty); err != nil { // back to exactly the claim
		t.Fatalf("(d) restore: %v", err)
	}
	_, l = e.line(amy, "")
	e.wantLive("(d) exactly the claimed quantity is live again", l, ltgLive, amyBundle, o1.ID)

	// (e) expiry is not extended by the pause: an opened claim and an unopened link both end with their link.
	e.expireLinks(gusBundle)
	_, l = e.line(gus, "")
	e.wantCatalog("(e) opened claim after its link expired (offer paused)", l)
	e.expireLinks(eveBundle)
	eve := e.buyerCap()
	if _, err := e.h.preview(eve.cap, eveLink.Token); !errors.Is(err, command.ErrNotFound) {
		t.Fatalf("(e) preview of an expired link of a paused offer: %v", err)
	}
	if _, err := e.h.redeem(eve.cap, t04Key("kop-eve"), eveLink.Token, 1); !errors.Is(err, command.ErrNotFound) {
		t.Fatalf("(e) redeem of an expired link of a paused offer: %v", err)
	}

	// (g) a buyer without a claim: the catalog price through the cart, and a forged origin on amy's bundle (bound to amy) earns nothing.
	frank := e.direct(ltgQty)
	_, l = e.line(frank, "")
	e.wantCatalog("(g) direct purchase of the paused offer's SKU", l)
	if err := e.rawCartLine(frank, e.money, ltgQty, amyBundle, o1.ID, ltgQty); err != nil {
		t.Fatalf("(g) forgery setup: %v", err)
	}
	_, l = e.line(frank, "")
	e.wantCatalog("(g) another buyer's claim bundle on a paused offer", l)

	// (f) reactivation changes nothing for the claimants (same price, quantity, origin) and they keep it afterwards.
	_, before := e.line(amy, "")
	e.setOfferActive(s1, o1, v, true)
	_, after := e.line(amy, "")
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("(f) reactivation changed amy's quote line: %+v -> %+v", before, after)
	}
	e.wantLive("(f) amy after reactivation", after, ltgLive, amyBundle, o1.ID)
	_, l = e.line(frank, "")
	e.wantCatalog("(f) a buyer without a claim after reactivation", l)
}

// TestLivePriceKeepOnPauseClearedPriceStillEndsIt: only the merchant removing the live price (not the pause) takes it away: the price is evaluated at quote
// time (rule 3), and a quote taken at the live price fails closed at placement once the price was cleared (the fail-closed half of LTG03).
func TestLivePriceKeepOnPauseClearedPriceStillEndsIt(t *testing.T) {
	e := ltgNew(t)
	s1, o1 := e.session("A1", ltgLive, 5)
	bundle, link := e.claimLink(s1, "amy", "A1+2")
	amy := e.redeemed(link)
	v := e.setOfferActive(s1, o1, o1.Version, false)
	q, l := e.line(amy, "")
	e.wantLive("paused offer keeps the price", l, ltgLive, bundle, o1.ID)
	// Clear the price on the paused offer (PATCH: 0 = clear).
	st, out := e.mjson(e.token(), "PATCH", e.claimsPath(s1, "/offers/"+o1.ID), fmt.Sprintf(`{"expected_version":%d,"max_quantity_per_claim":5,"active":false,"live_price_minor":0}`, v))
	if st != 200 || out["live_price_minor"] != nil {
		t.Fatalf("clear the price of the paused offer: %d %v", st, out)
	}
	if _, err := e.placeHome(amy, q); !errors.Is(err, command.ErrConflict) {
		t.Fatalf("a live quote must fail closed once the merchant cleared the live price: %v", err)
	}
	_, l = e.line(amy, "")
	e.wantCatalog("cleared live price on a paused offer", l)
}

// TestLivePriceKeepOnPauseOneClaimPerSKU: the cart carries one claim origin per SKU. Typo recovery (a paused offer and its active replacement on one SKU,
// both claimed by one buyer) must never fail the redeem: the active offer's line is applied and priced, the paused one stays pending and is skipped.
func TestLivePriceKeepOnPauseOneClaimPerSKU(t *testing.T) {
	e := ltgNew(t)
	const typoPrice = int64(17000)
	s, typo := e.session("A11", typoPrice, 5)
	first := e.h.accepted(t, s, "", "kim", "A11+2")
	e.h.setOffer(t, s, typo, 5, false)
	fixed := e.h.livePriceOffer(t, s, "A1", e.money, 5, ltgLive)
	second := e.h.accepted(t, s, first.BundleID, "", "A1+3")
	if second.BundleID != first.BundleID || second.OfferID != fixed.ID {
		t.Fatalf("setup: %+v %+v", first, second)
	}
	link := e.h.link(t, s, first.BundleID, 0, false)
	e.h.closeWindow(t, s)

	kim := e.buyerCap()
	pv, err := e.h.preview(kim.cap, link.Token)
	if err != nil || len(pv.Lines) != 2 || pv.Lines[0].Keyword != "A1" || pv.Lines[1].Keyword != "A11" {
		t.Fatalf("preview: %+v %v", pv, err)
	}
	if !pv.Lines[0].Available || pv.Lines[1].Available {
		t.Fatalf("only the active replacement is available, the superseded paused line is not: %+v", pv.Lines)
	}
	redeemed, err := e.h.redeem(kim.cap, t04Key("kop-kim"), link.Token, pv.BundleVersion)
	if err != nil || !reflect.DeepEqual(lcItems(redeemed.Applied), map[string]int64{e.money: 3}) ||
		!reflect.DeepEqual(lcSkipped(redeemed.Skipped), []claims.Skipped{{SKUID: e.money, Reason: "offer_inactive"}}) {
		t.Fatalf("redeem of a bundle holding a paused and an active offer on one SKU: %+v %v", redeemed, err)
	}
	_, l := e.line(kim, "")
	e.wantLive("the active replacement's claim is priced", l, ltgLive, first.BundleID, fixed.ID)
	if l.Quantity != 3 {
		t.Fatalf("quantity %d, want 3", l.Quantity)
	}

	// Degenerate: both offers of the SKU are paused and the buyer claimed both: exactly one line is applied (the lowest offer id), never a conflict.
	s2, typo2 := e.session("A11", typoPrice, 5)
	f2 := e.h.accepted(t, s2, "", "lee", "A11+2")
	e.h.setOffer(t, s2, typo2, 5, false)
	fixed2 := e.h.livePriceOffer(t, s2, "A1", e.money, 5, ltgLive)
	e.h.accepted(t, s2, f2.BundleID, "", "A1+3")
	e.h.setOffer(t, s2, fixed2, 5, false)
	link2 := e.h.link(t, s2, f2.BundleID, 0, false)
	e.h.closeWindow(t, s2)
	lee := e.buyerCap()
	pv2, err := e.h.preview(lee.cap, link2.Token)
	if err != nil {
		t.Fatal(err)
	}
	r2, err := e.h.redeem(lee.cap, t04Key("kop-lee"), link2.Token, pv2.BundleVersion)
	if err != nil || len(r2.Applied) != 1 || len(r2.Skipped) != 1 || r2.Skipped[0].Reason != "offer_inactive" {
		t.Fatalf("redeem with two paused offers on one SKU must apply exactly one line: %+v %v", r2, err)
	}
	wantQty, wantOffer, wantPrice := int64(2), typo2.ID, typoPrice
	if fixed2.ID < typo2.ID {
		wantQty, wantOffer, wantPrice = 3, fixed2.ID, ltgLive
	}
	if r2.Applied[0].Quantity != wantQty {
		t.Fatalf("the line with the lowest offer id must win: applied %+v want quantity %d", r2.Applied, wantQty)
	}
	_, l = e.line(lee, "")
	e.wantLive("degenerate case: the winning paused line keeps its own granted price", l, wantPrice, f2.BundleID, wantOffer)
}

// TestLiveConsoleOrderForBuyerPausedOfferKeepsLivePrice: the merchant order-for-buyer prefill (claims.for_buyer_lines, LC-B6) follows the same rule. A bundle holds
// a claim on a PAUSED offer (A1, 2 units at 17000) and on its active replacement on the same SKU (B1, 3 units at the live price): the merchant's order for 2 units
// takes the ACTIVE offer's line (the same winner as the buyer's redeem), not the paused one that sorts first by keyword; and a bundle whose only claim is on a paused
// offer is still live-priced (it used to fall back to the catalog price with live_price "not_applied").
func TestLiveConsoleOrderForBuyerPausedOfferKeepsLivePrice(t *testing.T) {
	e := lbuNew(t)
	token := e.token()

	// (1) only a paused offer: the claim is honoured.
	s1, offer, bundle, conv := e.scenario()
	e.setOfferActive(s1, offer, offer.Version, false)
	st, out := e.post(token, t04Key("kop-lbu-paused"), e.body(2, []string{bundle}, conv, false))
	if st != 201 || out["live_price"] != "applied" || out["live_price_reason"] != "" {
		t.Fatalf("order for a buyer whose claim is on a paused offer: %d %v", st, out)
	}
	order := lbuStr(out, "order_id")
	if p, r := e.unit(order); p != ltgLive || r != "live_claim" {
		t.Fatalf("paused offer: unit price %d rule %q, want %d live_claim", p, r, ltgLive)
	}
	if qty, rows := e.held(bundle, offer.ID); qty != 2 || rows != 1 {
		t.Fatalf("ledger of the paused offer's claim: held %d rows %d", qty, rows)
	}

	// (2) a paused and an active offer on one SKU in one bundle: the active offer's line wins.
	const pausedPrice = int64(17000)
	s2, paused := e.session("A1", pausedPrice, 5)
	b2 := e.h.accepted(t, s2, "", "kim-"+t04Tag(), "A1+2").BundleID
	e.h.setOffer(t, s2, paused, 5, false)
	active := e.h.livePriceOffer(t, s2, "B1", e.money, 5, ltgLive)
	e.h.accepted(t, s2, b2, "", "B1+3")
	e.h.closeWindow(t, s2)
	conv2 := lcConversation(t, e.p.f, e.tenant(), e.store(), "page")
	e.linkPeer(b2, conv2)
	st, out = e.post(token, t04Key("kop-lbu-two"), e.body(2, []string{b2}, conv2, false))
	if st != 201 || out["live_price"] != "applied" {
		t.Fatalf("order with a paused and an active offer on one SKU: %d %v", st, out)
	}
	if p, r := e.unit(lbuStr(out, "order_id")); p != ltgLive || r != "live_claim" {
		t.Fatalf("two offers on one SKU: unit price %d rule %q, want the ACTIVE offer's %d", p, r, ltgLive)
	}
	if qty, _ := e.held(b2, active.ID); qty != 2 {
		t.Fatalf("the active offer's claim must hold the 2 units: %d", qty)
	}
	if qty, rows := e.held(b2, paused.ID); qty != 0 || rows != 0 {
		t.Fatalf("the paused offer's claim must stay untouched: %d rows %d", qty, rows)
	}
}
