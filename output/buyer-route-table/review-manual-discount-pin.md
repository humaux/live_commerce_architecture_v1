Claude Opus 5.5 (integrator, independent of the Codex author): buyerhttp.Routes pin for FEAT-MANUAL-DISCOUNT (PR #394)
APPROVE

The closure was read against base r3/integration at c52b65283.
- The only change inside the buyerhttp package is internal/buyerhttp/projections.go (+6/-5):
  - the buyer order quote DTO gains `Adjustments []pricing.Adjustment json:"adjustments,omitempty"`;
  - projectOrder copies quote.Adjustments into it.
- pricing.Adjustment serialises exactly {kind, currency, name, amount_minor}. That is the merchant-entered discount name and amount, intended for the buyer. It carries no staff identity, no internal reason code and no authorization row id. The storefront parser (apps/storefront/lib/purchase.ts:233) accepts exactly these four keys.
- The functions/declarations lists show no delta; only the closure body hash changed.
  - Old: 31499ffb3ce0f159bcacb0f380bcbfe7fb94c220f5b91231d91e5edd840fbbd7, the #396 reviewed base.
  - New: a816f2699027fc26ca5fa3b7c87258e8b809ebb3c97e24baff797f88b9e9ba30.
- No route, method, URL, authentication, tenant/store scope or error-envelope logic in the closure changed.
- Non-blocking note, kept in the P2 batch: the discount name has no erasure path (Opus review-394-99abe2610.md, P2 2).
