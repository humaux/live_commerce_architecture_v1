// Purpose: reject ambiguous claim origins when several claim rows collapse onto one requested SKU.
// Depends on: the pure pickOrigins mapper and claims.ForBuyerLine; Quote remains the only price authority.
// Used by: CI merchanttools unit tests; REAL_PG/browser gates separately exercise claim grants and actual quotes.
// Invariants: merged quantity never inherits an arbitrarily selected bundle/offer; a single eligible origin still applies.
package merchanttools

import (
	"testing"

	"livecommerce/internal/claims"
)

func TestPickOriginsAmbiguousSKUUsesCatalog(t *testing.T) {
	live := int64(100)
	first := claims.ForBuyerLine{BundleID: "bundle-a", OfferID: "offer-a", SKUID: "sku-a", Keyword: "A1", Quantity: 4, LivePriceMinor: &live, LiveRemaining: 4}
	second := claims.ForBuyerLine{BundleID: "bundle-b", OfferID: "offer-b", SKUID: "sku-a", Keyword: "B1", Quantity: 2, LivePriceMinor: &live, LiveRemaining: 2}
	covers := second
	covers.Quantity, covers.LiveRemaining = 4, 4
	inactive := second
	inactive.LivePriceMinor = nil
	// The requested quantity is already merged by SKU. One claim covers all four units, but that does not identify their origin.
	items := []ManualItem{{SKUID: "sku-a", Quantity: 4}}
	for _, c := range []struct {
		name   string
		second claims.ForBuyerLine
	}{
		{"only one claim covers the merged quantity", second},
		{"both claims cover the merged quantity", covers},
		{"other claim has no live price", inactive},
	} {
		t.Run(c.name, func(t *testing.T) {
			for _, lines := range [][]claims.ForBuyerLine{{first, c.second}, {c.second, first}} {
				origins, skipped := pickOrigins(items, lines)
				if len(origins) != 0 || skipped != 1 {
					t.Fatalf("ambiguous merged SKU must stay at catalog price in either row order: origins=%d skipped=%d want=0/1", len(origins), skipped)
				}
			}
		})
	}
}

func TestPickOriginsUnambiguousSKUStillApplies(t *testing.T) {
	live := int64(100)
	eligible := claims.ForBuyerLine{BundleID: "bundle-a", OfferID: "offer-a", SKUID: "sku-a", Quantity: 4, LivePriceMinor: &live, LiveRemaining: 4}
	other := claims.ForBuyerLine{BundleID: "bundle-b", OfferID: "offer-b", SKUID: "sku-b", Quantity: 1, LivePriceMinor: &live, LiveRemaining: 1}
	origins, skipped := pickOrigins([]ManualItem{{SKUID: "sku-a", Quantity: 4}, {SKUID: "sku-unclaimed", Quantity: 1}}, []claims.ForBuyerLine{eligible, other})
	got, exists := origins["sku-a"]
	if skipped != 0 || len(origins) != 1 || !exists || got.BundleID != eligible.BundleID || got.OfferID != eligible.OfferID || got.Quantity != eligible.Quantity {
		t.Fatalf("one eligible origin must be preserved without counting an unclaimed SKU as skipped: origins=%d skipped=%d matched=%v", len(origins), skipped, exists)
	}
}
