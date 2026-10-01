// Live tools (R4) author smoke (commerce_worker, REAL_PG, MOCK manual ingress; no provider):
// proves migrations/0092_live_tools.sql applies and that the live-only price is reachable ONLY through
// a bound, unexpired claim origin (amendment "Live tools (R4)" of contracts/live-keyword-claims-v1.md),
// and that the keyword library / import / copy never overwrite. It is NOT the independent gate: the
// test_worker writes those from the contract. Isolation: the lcSetup harness (own principal, own
// sessions purged on cleanup; the one-OPEN-window-per-store rule is respected by closing windows).

package foundation_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/buyer"
	"livecommerce/internal/claims"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
	"livecommerce/internal/storefront"
)

func (h *lcHarness) quoteFor(c buyer.Capability, key string, version int64) (storefront.Quote, error) {
	return cqBuyer(h.a.runtime, c, func(ctx context.Context, tx pgx.Tx, s buyer.Scope) (storefront.Quote, error) {
		return storefront.CreateQuote(ctx, tx, s, key, storefront.QuoteInput{CartVersion: version, MarketID: h.market.ID, Country: "TW", Method: "cvs_711"})
	})
}

// priced creates a quote for the capability's current cart and returns its single line.
func (h *lcHarness) priced(t *testing.T, c buyer.Capability) (storefront.Quote, storefront.QuoteLine) {
	t.Helper()
	cart := h.cartOf(t, c)
	q, err := h.quoteFor(c, t04Key("lt-quote"), cart.Version)
	if err != nil || len(q.Lines) != 1 {
		t.Fatalf("quote: %+v %v", q, err)
	}
	return q, q.Lines[0]
}

func (h *lcHarness) livePriceOffer(t *testing.T, session, keyword, sku string, max, price int64) claims.Offer {
	t.Helper()
	p := price
	o, err := h.createOffer(h.token, h.f.storeA1, t04Key("lt-offer"), session, claims.OfferInput{Keyword: keyword, SKUID: sku, MaxQuantityPerClaim: max, LivePriceMinor: &p})
	if err != nil {
		t.Fatalf("CreateOffer %s with live price: %v", keyword, err)
	}
	if o.LivePriceMinor == nil || *o.LivePriceMinor != price {
		t.Fatalf("offer live price not stored: %+v", o)
	}
	return o
}

func TestLiveToolsLivePriceOnlyThroughClaimOrigin(t *testing.T) {
	h := lcSetup(t)
	f := h.f
	ctx := context.Background()
	var ledger int
	if err := f.owner.QueryRow(ctx, `SELECT count(*) FROM public.lc_schema_migrations WHERE version='0092_live_tools.sql'`).Scan(&ledger); err != nil || ledger != 1 {
		t.Fatalf("0092 not recorded exactly once: %d %v", ledger, err)
	}
	sku0, sku1 := h.stock.skus[0].ID, h.stock.skus[1].ID
	const live, catalogPrice = int64(700), int64(1250)

	// Session 1: A1 -> sku0 at live price 700, B1 -> sku1 with no live price.
	s1 := h.draft(t, f.storeA1)
	h.open(t, s1, claims.MatchExact)
	offer := h.livePriceOffer(t, s1, "A1", sku0, 5, live)
	h.offer(t, s1, "B1", sku1, 5)
	if offer.SKUPriceMinor != catalogPrice || offer.Currency == "" {
		t.Fatalf("offer display fields: %+v", offer)
	}
	first := h.accepted(t, s1, "", "amy", "A1+2")
	h.accepted(t, s1, first.BundleID, "", "B1")
	l1 := h.link(t, s1, first.BundleID, 0, false)
	h.closeWindow(t, s1)

	buyer1 := h.cap
	buyer2 := mustIssue(t, h.service, f.storeA1)

	// (1) The link display shows the live price on the priced line only.
	pv, err := h.preview(buyer1, l1.Token)
	if err != nil || len(pv.Lines) != 2 {
		t.Fatalf("preview: %+v %v", pv, err)
	}
	for _, l := range pv.Lines {
		switch l.Keyword {
		case "A1":
			if l.UnitPriceMinor != live || l.CatalogUnitPriceMinor != catalogPrice || l.PriceRule != "live_claim" {
				t.Fatalf("A1 preview: %+v", l)
			}
		case "B1":
			if l.UnitPriceMinor != catalogPrice || l.PriceRule != "" {
				t.Fatalf("B1 preview: %+v", l)
			}
		}
	}

	// (2) A direct cart (no claim) never gets the live price.
	if _, err := h.putCart(buyer2, t04Key("lt-direct"), storefront.CartInput{ExpectedVersion: 0, Items: []storefront.Item{{SKUID: sku0, Quantity: 2}}}); err != nil {
		t.Fatal(err)
	}
	if _, line := h.priced(t, buyer2); line.UnitPriceMinor != catalogPrice || line.PriceRule != "" {
		t.Fatalf("direct cart priced at %+v", line)
	}
	// A second buyer cannot use buyer 1's link at all (bound to buyer 1 after redeem) and cannot reach its price.
	red, err := h.redeem(buyer1, t04Key("lt-redeem"), l1.Token, pv.BundleVersion)
	if err != nil || len(red.Applied) != 2 {
		t.Fatalf("redeem: %+v %v", red, err)
	}
	if _, err := h.redeem(buyer2, t04Key("lt-redeem2"), l1.Token, pv.BundleVersion); !errors.Is(err, command.ErrNotFound) {
		t.Fatalf("other buyer redeemed a bound link: %v", err)
	}

	// (3) The redeemed cart: A1 at the live price, B1 (no live price) at catalog. Quote evidence is complete.
	cart := h.cartOf(t, buyer1)
	q1, err := h.quoteFor(buyer1, t04Key("lt-quote-claim"), cart.Version)
	if err != nil || len(q1.Lines) != 2 {
		t.Fatalf("claim quote: %+v %v", q1, err)
	}
	byCode := map[string]storefront.QuoteLine{}
	for _, l := range q1.Lines {
		byCode[l.SKUID] = l
	}
	a, b := byCode[sku0], byCode[sku1]
	if a.UnitPriceMinor != live || a.PriceRule != "live_claim" || a.CatalogUnitPriceMinor != catalogPrice || a.ClaimOfferID != offer.ID || a.ClaimBundleID != first.BundleID || a.Amount.SubtotalMinor != 2*live {
		t.Fatalf("claimed line: %+v", a)
	}
	if b.UnitPriceMinor != catalogPrice || b.PriceRule != "" || b.ClaimBundleID != "" {
		t.Fatalf("unpriced offer line: %+v", b)
	}
	if _, err := checkoutRevalidate(h.cqHarness, buyer1, q1.ID, cart.Version); err != nil {
		t.Fatalf("revalidate of a live-priced quote: %v", err)
	}

	// (4) Quantity: lowering keeps the price; raising above the claim drops the whole line to catalog.
	cart, err = h.putCart(buyer1, t04Key("lt-lower"), storefront.CartInput{ExpectedVersion: cart.Version, Items: []storefront.Item{{SKUID: sku0, Quantity: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, line := h.priced(t, buyer1); line.UnitPriceMinor != live {
		t.Fatalf("lowered quantity lost the live price: %+v", line)
	}
	cart, err = h.putCart(buyer1, t04Key("lt-raise"), storefront.CartInput{ExpectedVersion: cart.Version, Items: []storefront.Item{{SKUID: sku0, Quantity: 3}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, line := h.priced(t, buyer1); line.UnitPriceMinor != catalogPrice || line.PriceRule != "" {
		t.Fatalf("quantity above the claim kept the live price: %+v", line)
	}
	// Dropping an origin is permanent: back at the claimed quantity it stays catalog until a new redeem.
	cart, err = h.putCart(buyer1, t04Key("lt-back"), storefront.CartInput{ExpectedVersion: cart.Version, Items: []storefront.Item{{SKUID: sku0, Quantity: 2}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, line := h.priced(t, buyer1); line.PriceRule != "" {
		t.Fatalf("dropped origin came back: %+v", line)
	}
}

func TestLiveToolsExpiryOffersAndForgedOrigin(t *testing.T) {
	h := lcSetup(t)
	f := h.f
	ctx := context.Background()
	sku0 := h.stock.skus[0].ID
	const live, catalogPrice = int64(700), int64(1250)

	// Buyer 1 holds a live-priced claim line on session 1; buyer 2 on session 2's offer for the SAME SKU at 500.
	s1 := h.draft(t, f.storeA1)
	h.open(t, s1, claims.MatchExact)
	o1 := h.livePriceOffer(t, s1, "A1", sku0, 5, live)
	c1 := h.accepted(t, s1, "", "amy", "A1+2")
	l1 := h.link(t, s1, c1.BundleID, 0, false)
	h.closeWindow(t, s1)
	s2 := h.draft(t, f.storeA1)
	h.open(t, s2, claims.MatchExact)
	h.livePriceOffer(t, s2, "A1", sku0, 5, 500)
	c2 := h.accepted(t, s2, "", "bob", "A1+2")
	l2 := h.link(t, s2, c2.BundleID, 0, false)
	h.closeWindow(t, s2)

	buyer1, buyer2 := h.cap, mustIssue(t, h.service, f.storeA1)
	pv1, _ := h.preview(buyer1, l1.Token)
	pv2, _ := h.preview(buyer2, l2.Token)
	if _, err := h.redeem(buyer1, t04Key("lt-r1"), l1.Token, pv1.BundleVersion); err != nil {
		t.Fatal(err)
	}
	if _, err := h.redeem(buyer2, t04Key("lt-r2"), l2.Token, pv2.BundleVersion); err != nil {
		t.Fatal(err)
	}
	// Another session's offer on the same SKU never reprices a different bundle's line.
	if _, line := h.priced(t, buyer1); line.UnitPriceMinor != live || line.ClaimOfferID != o1.ID {
		t.Fatalf("buyer 1 line: %+v", line)
	}
	if _, line := h.priced(t, buyer2); line.UnitPriceMinor != 500 {
		t.Fatalf("buyer 2 line: %+v", line)
	}

	// Forged origin: buyer 1 writes buyer 2's bundle/offer onto its own cart line with raw SQL (a compromised app).
	// claims.live_prices proves binding, so buyer 1 still pays the price of ITS OWN valid origin only if it keeps it;
	// pointing at the foreign bundle must yield the catalog price.
	err := buyer.WithScope(ctx, h.a.runtime, buyer1.Token, f.storeA1, func(c context.Context, tx pgx.Tx, s buyer.Scope) error {
		tag, e := tx.Exec(c, `UPDATE storefront.cart_lines SET claim_bundle_id=$1,claim_offer_id=$2 WHERE tenant_id=$3 AND store_id=$4 AND owner_id=$5`,
			c2.BundleID, claimOfferOf(t, f, c2.BundleID), s.TenantID, s.StoreID, s.OwnerID)
		if e == nil && tag.RowsAffected() == 0 {
			e = errors.New("no cart line updated")
		}
		return e
	})
	if err != nil {
		// The buyer role has no UPDATE on cart_lines (DELETE+INSERT is the only write): then the forgery must go through INSERT.
		t.Logf("raw UPDATE refused (%v); forging through DELETE+INSERT instead", err)
		err = buyer.WithScope(ctx, h.a.runtime, buyer1.Token, f.storeA1, func(c context.Context, tx pgx.Tx, s buyer.Scope) error {
			if _, e := tx.Exec(c, `DELETE FROM storefront.cart_lines WHERE tenant_id=$1 AND store_id=$2 AND owner_id=$3`, s.TenantID, s.StoreID, s.OwnerID); e != nil {
				return e
			}
			var cartID string
			if e := tx.QueryRow(c, `SELECT id::text FROM storefront.carts WHERE tenant_id=$1 AND store_id=$2 AND owner_id=$3`, s.TenantID, s.StoreID, s.OwnerID).Scan(&cartID); e != nil {
				return e
			}
			_, e := tx.Exec(c, `INSERT INTO storefront.cart_lines(tenant_id,store_id,owner_id,cart_id,sku_id,quantity,claim_bundle_id,claim_offer_id,claim_quantity)
				VALUES($1,$2,$3,$4,$5,2,$6,$7,2)`, s.TenantID, s.StoreID, s.OwnerID, cartID, sku0, c2.BundleID, claimOfferOf(t, f, c2.BundleID))
			return e
		})
	}
	if err != nil {
		t.Fatalf("forgery setup: %v", err)
	}
	if _, line := h.priced(t, buyer1); line.UnitPriceMinor != catalogPrice || line.PriceRule != "" {
		t.Fatalf("forged origin (another buyer's bundle) earned a live price: %+v", line)
	}

	// Restore buyer 1's own origin through a fresh redeem path: re-issue is not needed, the SQL row is rewritten as the app would.
	mustExec(t, f.owner, `UPDATE storefront.cart_lines SET claim_bundle_id=$1,claim_offer_id=$2 WHERE sku_id=$3 AND claim_bundle_id=$4`, c1.BundleID, o1.ID, sku0, c2.BundleID)
	q, line := h.priced(t, buyer1)
	if line.UnitPriceMinor != live {
		t.Fatalf("own origin not honoured after restore: %+v", line)
	}
	cart := h.cartOf(t, buyer1)
	// Defence in depth: a cart row claiming fewer units than it holds (only raw SQL can write one; SetCart drops the origin)
	// pays the catalog price for the whole line.
	mustExec(t, f.owner, `UPDATE storefront.cart_lines SET claim_quantity=1 WHERE claim_bundle_id=$1`, c1.BundleID)
	if _, line := h.priced(t, buyer1); line.UnitPriceMinor != catalogPrice || line.PriceRule != "" {
		t.Fatalf("line above its claimed quantity kept the live price: %+v", line)
	}
	mustExec(t, f.owner, `UPDATE storefront.cart_lines SET claim_quantity=2 WHERE claim_bundle_id=$1`, c1.BundleID)

	// Offer deactivation and live-price clearing both end the live price (evaluated at Quote time).
	paused, err := h.updateOffer(h.token, f.storeA1, t04Key("lt-pause"), s1, o1.ID, claims.OfferUpdate{ExpectedVersion: o1.Version, MaxQuantityPerClaim: 5, Active: false})
	if err != nil || paused.LivePriceMinor == nil || *paused.LivePriceMinor != live {
		t.Fatalf("pause must leave the price untouched when the key is absent: %+v %v", paused, err)
	}
	if _, line := h.priced(t, buyer1); line.PriceRule != "" || line.UnitPriceMinor != catalogPrice {
		t.Fatalf("inactive offer kept the live price: %+v", line)
	}
	zero := int64(0)
	cleared, err := h.updateOffer(h.token, f.storeA1, t04Key("lt-clear"), s1, o1.ID, claims.OfferUpdate{ExpectedVersion: paused.Version, MaxQuantityPerClaim: 5, Active: true, LivePriceMinor: &zero})
	if err != nil || cleared.LivePriceMinor != nil {
		t.Fatalf("0 must clear the live price: %+v %v", cleared, err)
	}
	price := live
	set, err := h.updateOffer(h.token, f.storeA1, t04Key("lt-set"), s1, o1.ID, claims.OfferUpdate{ExpectedVersion: cleared.Version, MaxQuantityPerClaim: 5, Active: true, LivePriceMinor: &price})
	if err != nil || set.LivePriceMinor == nil || *set.LivePriceMinor != live {
		t.Fatalf("setting the price: %+v %v", set, err)
	}
	// A negative or oversized price is invalid, never stored.
	for _, bad := range []int64{-1, 1_000_000_000_001} {
		v := bad
		if _, err := h.updateOffer(h.token, f.storeA1, t04Key("lt-bad"), s1, o1.ID, claims.OfferUpdate{ExpectedVersion: set.Version, MaxQuantityPerClaim: 5, Active: true, LivePriceMinor: &v}); !errors.Is(err, command.ErrInvalid) {
			t.Fatalf("live price %d: %v", bad, err)
		}
	}

	// A quote taken at the live price is revalidated only while the origin still holds: expire the link in the DB.
	q, line = h.priced(t, buyer1)
	if line.UnitPriceMinor != live {
		t.Fatalf("live price missing before expiry: %+v", line)
	}
	cart = h.cartOf(t, buyer1)
	if _, err := checkoutRevalidate(h.cqHarness, buyer1, q.ID, cart.Version); err != nil {
		t.Fatalf("revalidate before expiry: %v", err)
	}
	mustExec(t, f.owner, `UPDATE claims.links SET issued_at=clock_timestamp()-interval '80 hours',expires_at=clock_timestamp()-interval '10 hours' WHERE bundle_id=$1`, c1.BundleID)
	if _, line := h.priced(t, buyer1); line.UnitPriceMinor != catalogPrice || line.PriceRule != "" {
		t.Fatalf("expired link still earned the live price: %+v", line)
	}
	if _, err := checkoutRevalidate(h.cqHarness, buyer1, q.ID, cart.Version); !errors.Is(err, command.ErrConflict) {
		t.Fatalf("revalidate after expiry must fail closed (conflict), got %v", err)
	}
	// Merchant roles can never call the buyer price function (EXECUTE is granted to the buyer runtime only).
	err = h.do(h.token, f.storeA1, func(tx pgx.Tx, s platform.Scope) error {
		_, e := tx.Exec(ctx, `SELECT * FROM claims.live_prices(ARRAY[$1]::uuid[],ARRAY[$2]::uuid[],ARRAY[$3]::uuid[])`, c1.BundleID, o1.ID, sku0)
		return e
	})
	requirePGCode(t, err, "42501", "merchant EXECUTE on claims.live_prices")
}

func claimOfferOf(t *testing.T, f *testFixture, bundle string) string {
	t.Helper()
	var id string
	if err := f.owner.QueryRow(context.Background(), `SELECT offer_id::text FROM claims.lines WHERE bundle_id=$1 LIMIT 1`, bundle).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestLiveToolsLibraryImportAndCopy(t *testing.T) {
	h := lcSetup(t)
	f := h.f
	sku0, sku1 := h.stock.skus[0].ID, h.stock.skus[1].ID
	s1 := h.draft(t, f.storeA1)
	set := func(key, session, sku, keyword string, version int64) (out claims.LibraryEntry, err error) {
		err = h.do(h.token, f.storeA1, func(tx pgx.Tx, s platform.Scope) (e error) {
			out, e = claims.SetLibraryKeyword(h.ctx, tx, s, h.token, key, session, sku, claims.LibraryInput{Keyword: keyword, ExpectedVersion: version})
			return e
		})
		return out, err
	}
	imp := func(key, session string, in claims.ImportInput) (out claims.ImportResult, err error) {
		err = h.do(h.token, f.storeA1, func(tx pgx.Tx, s platform.Scope) (e error) {
			out, e = claims.ImportOffers(h.ctx, tx, s, h.token, key, session, in)
			return e
		})
		return out, err
	}

	e0, err := set(t04Key("lt-lib"), s1, sku0, "ｌ１", 0) // full-width, normalised like offers
	if err != nil || e0.Keyword != "L1" || e0.Version != 1 || e0.SKUPriceMinor != 1250 {
		t.Fatalf("library create: %+v %v", e0, err)
	}
	if _, err := set(t04Key("lt-lib"), s1, sku1, "L1", 0); !errors.Is(err, command.ErrConflict) {
		t.Fatalf("keyword unique per store: %v", err)
	}
	if _, err := set(t04Key("lt-lib"), s1, sku0, "L9", 0); !errors.Is(err, command.ErrConflict) {
		t.Fatalf("stale/absent version must conflict: %v", err)
	}
	if _, err := set(t04Key("lt-lib"), s1, sku0, "L!", 1); !errors.Is(err, command.ErrInvalid) {
		t.Fatalf("invalid keyword: %v", err)
	}
	e1, err := set(t04Key("lt-lib"), s1, sku1, "L2", 0)
	if err != nil {
		t.Fatal(err)
	}
	renamed, err := set(t04Key("lt-lib"), s1, sku0, "L1X", e0.Version)
	if err != nil || renamed.Keyword != "L1X" || renamed.Version != 2 {
		t.Fatalf("rename: %+v %v", renamed, err)
	}
	var list []claims.LibraryEntry
	if err := h.do(h.token, f.storeA1, func(tx pgx.Tx, s platform.Scope) (e error) {
		list, e = claims.ListLibrary(h.ctx, tx, s, h.token, s1)
		return e
	}); err != nil || len(list) < 2 {
		t.Fatalf("list: %+v %v", list, err)
	}

	// Target session 2 already holds keyword L2 on sku0 (active): L1X(sku0) -> sku_taken, L2(sku1) -> keyword_taken.
	s2 := h.draft(t, f.storeA1)
	pre := h.livePriceOffer(t, s2, "L2", sku0, 4, 600)
	res, err := imp(t04Key("lt-imp"), s2, claims.ImportInput{Source: "library"})
	if err != nil || len(res.Created) != 0 || len(res.Conflicts) != 2 {
		t.Fatalf("import into a conflicting session: %+v %v", res, err)
	}
	reasons := map[string]string{}
	for _, c := range res.Conflicts {
		reasons[c.Keyword] = c.Reason
	}
	if reasons["L1X"] != claims.ConflictSKUTaken || reasons["L2"] != claims.ConflictKeywordTaken {
		t.Fatalf("conflict reasons: %+v", reasons)
	}
	// Nothing was overwritten.
	if b := h.board(t, s2); len(b.Offers) != 1 || b.Offers[0].ID != pre.ID || b.Offers[0].Version != pre.Version || b.Offers[0].LivePriceMinor == nil || *b.Offers[0].LivePriceMinor != 600 {
		t.Fatalf("import changed an existing offer: %+v", b.Offers)
	}

	// Empty session 3: both library keywords are created (limit 3, never a live price); a replay returns the same result, an
	// import with a new key reports already_present and creates nothing.
	s3 := h.draft(t, f.storeA1)
	key := t04Key("lt-imp3")
	res, err = imp(key, s3, claims.ImportInput{Source: "library"})
	if err != nil || len(res.Created) != 2 || len(res.Conflicts) != 0 {
		t.Fatalf("import into an empty session: %+v %v", res, err)
	}
	for _, o := range res.Created {
		if o.MaxQuantityPerClaim != 3 || o.LivePriceMinor != nil || !o.Active {
			t.Fatalf("seeded offer: %+v", o)
		}
	}
	if replay, err := imp(key, s3, claims.ImportInput{Source: "library"}); err != nil || len(replay.Created) != 2 {
		t.Fatalf("replay: %+v %v", replay, err)
	}
	again, err := imp(t04Key("lt-imp3b"), s3, claims.ImportInput{Source: "library"})
	if err != nil || len(again.Created) != 0 || len(again.Conflicts) != 2 || again.Conflicts[0].Reason != claims.ConflictAlreadyPresent {
		t.Fatalf("re-import: %+v %v", again, err)
	}

	// Copy from session 2 (active offer L2/sku0/max 4, live price 600) into empty session 4: copied without the price.
	s4 := h.draft(t, f.storeA1)
	cp, err := imp(t04Key("lt-copy"), s4, claims.ImportInput{Source: "session", FromSessionID: s2})
	if err != nil || len(cp.Created) != 1 || cp.Created[0].Keyword != "L2" || cp.Created[0].MaxQuantityPerClaim != 4 || cp.Created[0].LivePriceMinor != nil {
		t.Fatalf("copy: %+v %v", cp, err)
	}
	// Source validation.
	for _, bad := range []claims.ImportInput{{Source: "session", FromSessionID: s4}, {Source: "session"}, {Source: "library", FromSessionID: s2}, {Source: "nope"}} {
		if _, err := imp(t04Key("lt-bad"), s4, bad); !errors.Is(err, command.ErrInvalid) {
			t.Fatalf("import %+v: %v", bad, err)
		}
	}
	if _, err := imp(t04Key("lt-bad"), s4, claims.ImportInput{Source: "session", FromSessionID: randomUUID()}); !errors.Is(err, command.ErrNotFound) {
		t.Fatalf("unknown source session: %v", err)
	}

	// Removal needs the current version; then the row is gone.
	if _, err := set(t04Key("lt-rm"), s1, sku1, "", e1.Version+5); !errors.Is(err, command.ErrConflict) {
		t.Fatalf("stale remove: %v", err)
	}
	gone, err := set(t04Key("lt-rm"), s1, sku1, "", e1.Version)
	if err != nil || gone.Version != 0 || gone.Keyword != "" {
		t.Fatalf("remove: %+v %v", gone, err)
	}
	if _, err := set(t04Key("lt-rm2"), s1, sku1, "", 0); !errors.Is(err, command.ErrNotFound) {
		t.Fatalf("removing an absent entry: %v", err)
	}
	// Library SKU FKs and CHECKs hold at the database level too.
	_, err = f.owner.Exec(context.Background(), `INSERT INTO live.keyword_library(tenant_id,store_id,sku_id,keyword,principal_id) VALUES($1,$2,$3,'bad key',$4)`, f.tenantA, f.storeA1, sku1, h.actor)
	requirePGCode(t, err, "23514", "library keyword CHECK")
	_, err = f.owner.Exec(context.Background(), `UPDATE live.offers SET live_price_minor=0 WHERE id=$1`, pre.ID)
	requirePGCode(t, err, "23514", "live price CHECK >= 1")
	_ = time.Now
}
