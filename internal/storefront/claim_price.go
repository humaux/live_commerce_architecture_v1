// claim_price.go overlays live-only (claim-origin) prices onto the catalog-priced quote lines.
// It is the ONLY place a unit price differs from catalog.skus.price_minor, and both CreateQuote
// and RevalidateQuote call it, so a quote and its revalidation can never disagree about the rule.
//
// Non-goals: it never decides whether a claim is valid (claims.live_prices does, in SQL, at call
// time), never writes, never accepts a price from a client, never touches stock.

package storefront

import (
	"context"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/buyer"
	"livecommerce/internal/pricing" // ResolveUnitPrice: the pure rule and the rule names.
)

// applyLivePrices rewrites lines (as returned by lockCatalog, catalog-priced) whose cart line has a
// claim origin that claims.live_prices still honours. It only considers origins whose quantity
// is within the claimed quantity (the SQL below), so a buyer who raised a claimed line above
// its claim pays the catalog price for the whole line.
func applyLivePrices(ctx context.Context, tx pgx.Tx, s buyer.Scope, cartID string, lines []QuoteLine) error {
	// storefront.cart_lines (own table): origins of the lines in this cart.
	rows, err := tx.Query(ctx, `SELECT sku_id::text,claim_bundle_id::text,claim_offer_id::text,quantity FROM storefront.cart_lines
		WHERE tenant_id=$1 AND store_id=$2 AND owner_id=$3 AND cart_id=$4 AND claim_bundle_id IS NOT NULL AND quantity<=claim_quantity
		ORDER BY sku_id`, s.TenantID, s.StoreID, s.OwnerID, cartID)
	if err != nil {
		return err
	}
	var skus, bundles, offers []string
	var quantities []int64
	for rows.Next() {
		var sku, bundle, offer string
		var quantity int64
		if err = rows.Scan(&sku, &bundle, &offer, &quantity); err != nil {
			rows.Close()
			return err
		}
		skus, bundles, offers, quantities = append(skus, sku), append(bundles, bundle), append(offers, offer), append(quantities, quantity)
	}
	rows.Close()
	if err = rows.Err(); err != nil || len(skus) == 0 {
		return err
	}
	// claims.live_prices (definer, EXECUTE commerce_buyer_runtime + commerce_checkout_runtime: Quote and checkout.Begin's
	// RevalidateQuote both land here; migrations/0092, 0103): returns a row only for origins bound to this buyer with an
	// unexpired link, a claim line covering the cart quantity (0103: the SQL, not claim_quantity, is the authority) and an
	// active priced offer of that session.
	priced, err := tx.Query(ctx, `SELECT sku_id::text,bundle_id::text,offer_id::text,live_price_minor
		FROM claims.live_prices($1::uuid[],$2::uuid[],$3::uuid[],$4::bigint[])`, bundles, offers, skus, quantities)
	if err != nil {
		return err
	}
	defer priced.Close()
	type live struct {
		bundle, offer string
		price         int64
	}
	bySKU := map[string]live{}
	for priced.Next() {
		var sku string
		var l live
		if err = priced.Scan(&sku, &l.bundle, &l.offer, &l.price); err != nil {
			return err
		}
		bySKU[sku] = l
	}
	if err = priced.Err(); err != nil {
		return err
	}
	for i := range lines {
		l, ok := bySKU[lines[i].SKUID]
		if !ok {
			continue
		}
		catalog := lines[i].UnitPriceMinor
		// I05: the applied unit price comes from ResolveUnitPrice over the DB-proven live price only.
		unit, rule := pricing.ResolveUnitPrice(catalog, &l.price)
		if rule != pricing.RuleLiveClaim {
			continue
		}
		lines[i].UnitPriceMinor, lines[i].PriceRule = unit, rule
		lines[i].CatalogUnitPriceMinor, lines[i].ClaimBundleID, lines[i].ClaimOfferID = catalog, l.bundle, l.offer
	}
	return nil
}
