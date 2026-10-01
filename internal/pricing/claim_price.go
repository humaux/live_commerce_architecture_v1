// claim_price.go is the pure unit-price rule for live-only (claim-origin) prices, used by
// storefront's Quote path only (contracts/live-keyword-claims-v1.md, amendment "Live tools (R4)" rule 5).
//
// Non-goals: it decides nothing about WHETHER a claim origin is valid (claims.live_prices proves
// binding, link expiry and offer state in SQL); it never reads a client amount; no discount
// stacking, no per-buyer price.

package pricing

// Price rules recorded on a quote line (and therefore on the order snapshot that embeds the quote).
// An absent rule on a snapshot means RuleCatalog.
const (
	RuleCatalog   = "catalog"
	RuleLiveClaim = "live_claim"
)

// maxUnitPriceMinor mirrors the checkout bound on a quote line (migrations/0013: unit_price_minor <= 1e12).
const maxUnitPriceMinor = 1_000_000_000_000

// ResolveUnitPrice returns the unit price to quote and the rule that produced it. A live price
// applies when the caller proved a valid claim origin (livePriceMinor != nil) and the value is inside
// the offer CHECK range; anything else falls back to the catalog price (fail closed: a bad value
// never lowers or raises a price by accident). A live price HIGHER than the catalog price is applied
// as set: the merchant is warned in Studio, the buyer sees it on the claim link, and the quote shows both.
func ResolveUnitPrice(catalogMinor int64, livePriceMinor *int64) (int64, string) {
	if livePriceMinor == nil || *livePriceMinor < 1 || *livePriceMinor > maxUnitPriceMinor {
		return catalogMinor, RuleCatalog
	}
	return *livePriceMinor, RuleLiveClaim
}
