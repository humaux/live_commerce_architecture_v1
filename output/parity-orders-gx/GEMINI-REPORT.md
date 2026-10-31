# GEMINI-REPORT — PARITY-ORDERS-FIX2

Card: PARITY-ORDERS-FIX2 (PR #362, branch unit/parity-orders)
Base: origin/r3/integration (629f42152, base Gates 41107905131 GREEN)
Integrator Ruling: 2026-10-31 resolves FIX2.md BLOCKED

## Failure Classification and Changes

| Failing job / test | Class | What changed, file:line |
|---|---|---|
| 124185561381 / TestBrowserOC (`reauth.spec.ts:188`) | (a) signed frame disclosure | `tests/admin/oc/reauth.spec.ts:188` (reveal orders-more-filters after reload to expose export control; verified PASS in run 41111408749 job 124257277338) |
| 124185562324 / TestBrowserMerchantOrdersUIRealChain (`orders-ui.spec.ts:168`) | (b) product root fix | `apps/admin/components/orders/parity.css:32` (scoped mobile selection bar display) |
| 124185562324 / TestBrowserMerchantOrdersUIRealChain (`orders-ui.spec.ts:495`) | (a) signed frame disclosure | `tests/admin/orders-ui.spec.ts:500` (revealOrderFilters before refresh click) |
| 124185562324 / TestBrowserMerchantOrdersUIRealChain (`orders-ui.spec.ts:974`) | (b) product root fix | `apps/admin/components/MerchantOrders.tsx:185` (move localized empty-state container inside orders-list) |
| 124185562324 / TestBrowserMerchantOrdersUIRealChain (`orders-ui.spec.ts:1029`) | (a) signed frame disclosure | `tests/admin/orders-ui.spec.ts:1027,1039,1050` (call `await revealOrderFilters(page)` following `page.reload()` before clicking `orders-reset`) |
| 124185561277 / WebKit buyer CVS (`cvs-picker.mjs:387`) | (c)-candidate | None (`tests/storefront/cvs-picker.mjs:387`). Out-of-card storefront CVS picker (#358 PARITY-CVS); assigned to integrator per ruling 2026-10-31. |

STATUS: DONE
