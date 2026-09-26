# Merchant orders C UI acceptance

Scope: local read-only `/{locale}/orders`, user-approved **C 表格原位展开**.
Buyer B is unchanged. This is not whole-SaaS, production, provider-payment,
shipping, refunds or live-broadcast acceptance.

## Implementation and authority

- Contract: [MOU01–06](../../contracts/merchant-orders-ui-v1.md); existing
  [MOR](../../contracts/merchant-orders-v1.md) and
  [MBT](../../contracts/merchant-orders-bff-v1.md) remain the server boundary.
- Source candidate `e4189ee`, root integration `1c27aec`: authenticated store
  selection, limit10 keyset pages, strict read DTOs, frozen financial/delivery
  details, independent commercial/payment/fulfillment/work states, three locales.
- Detail remains directly below the selected row. Mobile uses the same order
  and detail sequence with scoped item-table scrolling. No raster asset ships.
- Session/generation fencing and synchronous hide/pagehide clearing prevent
  old responses and cached recipient data from resurfacing. No persistent
  order-body cache, browser bearer or additional transaction engine.

## Evidence collected so far

Root `1c27aec`, all commands exited 0:

| Command | Observed result | Log under `/Volumes/data/output/` |
|---|---|---|
| `pnpm run typecheck:admin` | strict TS pass | terminal result |
| `bash scripts/dev/test-local.sh --browser-identity` | 3 top-level PASS, foundation 13.237s | `merchant-orders-c-identity-regression-20260927.log` |
| `bash scripts/dev/test-local.sh --browser-merchant-buyer` | 1 PASS, 6.521s | `merchant-orders-c-buyer-regression-20260927.log` |
| `bash scripts/dev/test-local.sh --browser-merchant-orders-bff` | 1 PASS, 7.758s | `merchant-orders-c-bff-regression-20260927.log` |

Each browser runner builds production Next and uses disposable PG18 plus a
signed MOCK OIDC issuer. It removes its own database fixture at exit.
The emitted production layout retains design-contract seed `549adef8`.

## Issues caught before acceptance

1. Strict DTO validation initially coerced values with `String(...)`; JSON
   arrays/numeric identifiers could pass regex checks. Source candidate requires
   actual strings before regex checks; independent parser negatives are required.
2. The first response fence checked generation before an awaited cookie digest
   but not after. The candidate rechecks generation/abort/visibility after await.
3. Real browser setup exposed fixture assumptions: payment quantities must obey
   the existing minor-unit contract; loopback `__Host-` cookies must be tested
   through browser fetch; two random store IDs cannot imply a fixed default.
4. Real merchant routes have no reachable Entry logout. A narrow common-shell
   logout is required before the cross-tab UI gate can be accepted. The accepted
   contract records this requirement; synthetic API-plus-event is not its proof.

## Pending acceptance

MOU01–06 remain **NOT_ACCEPTED** until the independent real-chain suite,
root replay, final source review, required desktop/mobile screenshots and
independent visual verdict are recorded. Regression results above do not prove
those gates, and must be replayed after any relevant common-shell repair.
