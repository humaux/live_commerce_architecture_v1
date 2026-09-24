# Buyer order history and explicit continuation

Baseline `1b35842`; protocol `b8aa993`; frontend/index `d68845f`; delayed-load
and storage fences `ddbcf21` / `e322f2b`; backend `75e77cb` (author `4954573`).
Status: source and regression gates passed; new browser/visual gate in progress.
This is not payment, expired-account recovery, deployment or whole-SaaS release.

## Why this shape

Reuse the existing owner cart and `cart.set` CAS/permanent receipt. A new cart
table, deleting carts, or clearing only a browser pointer would either break
historical references or let a stale checkout become a new purchase silently.
The buyer explicitly continues; earlier orders, inventory holds and immutable
quote/destination snapshots are unchanged. A newer server cart is preserved.

Dependencies and maintenance boundaries:

- `internal/checkout/orders.go` owns compact SQL history through the existing
  checkout pool, `buyer.WithScope`, forced owner RLS and final capability check.
  Its canonical `(created_at,id)` cursor binds tenant/store/owner/collection;
  it is never authentication. `0024_buyer_order_history.sql` adds the same-sort
  index only; no role grants or schema/identity redesign.
- `internal/buyerhttp` projects eight summary fields, no recipient/destination
  or worker metadata. Owned detail exposes its original cart ID/version.
- `apps/storefront/lib/buyer-server.ts` adds the strict `orders` read route;
  only limit/cursor are accepted. Existing cookie/context, origin resolution,
  no-store and response limits remain the transport boundary.
- `purchase.ts` adds the non-PII `next-cart` journal under the existing Web Lock.
  Unknown checkout blocks continuation. Empty-cart PUT retries its original
  CAS/key; the server's current cart must be newer before pointer/journal removal.
  A no-op/failed storage removal is not success. SQL history outlives the locator.
- `ProductPurchase` consumes that protocol and fences late order GETs against
  the current locator/epoch. `OrderHistory` is read-only; opening old A cannot
  change B's checkout locator. `history-copy.ts` carries all three locales.

The frozen behavior and negative gates are in
[buyer-order-history-v1](../../contracts/buyer-order-history-v1.md).

## Root evidence already obtained

| Gate | Observed result | Evidence |
| --- | --- | --- |
| Client and strict TypeScript | 34 Node tests PASS; typecheck exit 0 | `/Volumes/data/output/buyer-history-node-final.log`, `buyer-history-typecheck-final.log` |
| Full Go/PG/race/vet | 382 top-level tests PASS; foundation 147.280 s; includes migration 0024 | `/Volumes/data/output/buyer-history-full-regression.log` |
| Existing buyer browser | 13 cases PASS, foundation 8.496 s | `/Volumes/data/output/buyer-history-existing-browser-regression.log`; `output/playwright/buyer-real-2965457144` |
| Existing address/order browser | 15 cases, 6 buyers PASS; exactly one order/hold/job/receipt/reserve each; foundation 13.521 s | `/Volumes/data/output/buyer-history-existing-order-regression.log`; `output/playwright/buyer-order-2768957055` |
| Independent initial-load race | P2 resolved; old source negative control fails; fixed empty/B locator paths pass SOURCE/MOCK only | Humaux `e2d462b8-ba0f-411b-8c85-a3c37fe43394` |
| Style detector | One call, incumbent Arial warning only; approved Operate world retained | `/Volumes/data/output/buyer-history-detector.json` |

Tests use synthetic local HTTPS edges and disposable PostgreSQL. No customer
platform changes, charges, cancellations or production writes occurred. The
original failed backend assertion log is retained: it prohibited the newly
approved public cart provenance; the corrected test asserts exact original
cart values and still forbids internal metadata.

## Remaining acceptance and scope

- Independent new real-browser A/B purchases, history and delayed-GET race gate.
- Review the built history at desktop/mobile widths, then record the local
  extension without changing global DESIGN.md or the approved B composition.
- Secure expired-guest identity recovery remains required. Reset issues a new
  owner; order IDs, recipient names/phones and local hints cannot prove ownership.
- Hosted per-order payment, trusted CVS/carrier mapping, real DNS/TLS deployment,
  performance and complete SaaS release gates remain separate open work.
