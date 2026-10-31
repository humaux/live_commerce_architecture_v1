# GEMINI-REPORT — PARITY-ORDERS-FIX2 (RELAY 1)

Card: PARITY-ORDERS-FIX2 (PR #362, branch unit/parity-orders)
Head: e04726f4 (Failing run 41170952247, job 124449546996 gate (bundle:09))
Base: origin/r3/integration (629f42152, base Gates 41107905131)
Integrator Ruling: 2026-10-31 resolves FIX2.md BLOCKED

## Failure Classification and Changes

| Failing job / test | Class | What changed, file:line |
|---|---|---|
| 124449546996 (bundle:09) / `TestBrowserMerchantOrdersUIRealChain` (`parcel-merge.spec.ts:69`) | (a) signed frame navigation / setup helper rewrite | `tests/admin/parcel-merge.spec.ts:68-70`: robust `orderList(page)` to await `order-detail-back.click()` when on standalone detail route or mounted detail view rather than checking non-waiting `count()` immediately after `page.reload()`; lines 226, 235, 315: added `await orderList(page)` after asserting persisted member details |
| 124449546989 (bundle:06) / `TestBrowserInboxUIRealChain` | (c) out-of-card | None (`tests/admin/inbox*.spec.ts`). Out-of-card admin inbox suite (#361 PARITY-INBOX-EDITOR); untouched by parity-orders |
| 124185561381 (bundle:08) / `TestBrowserOC` (`reauth.spec.ts:188`) | (a) signed frame disclosure | `tests/admin/oc/reauth.spec.ts:188` (reveal orders-more-filters after reload; verified PASS in run 41170952247) |
| 124185562324 (bundle:09) / `TestBrowserMerchantOrdersUIRealChain` (`orders-ui.spec.ts:168`) | (b) product root fix | `apps/admin/components/orders/parity.css:32` (scoped mobile selection bar display; verified PASS in run 41170952247) |
| 124185562324 (bundle:09) / `TestBrowserMerchantOrdersUIRealChain` (`orders-ui.spec.ts:495`) | (a) signed frame disclosure | `tests/admin/orders-ui.spec.ts:500` (revealOrderFilters before refresh click; verified PASS in run 41170952247) |
| 124185562324 (bundle:09) / `TestBrowserMerchantOrdersUIRealChain` (`orders-ui.spec.ts:974`) | (b) product root fix | `apps/admin/components/MerchantOrders.tsx:185` (move localized empty-state container inside orders-list; verified PASS in run 41170952247) |
| 124185562324 (bundle:09) / `TestBrowserMerchantOrdersUIRealChain` (`orders-ui.spec.ts:1029`) | (a) signed frame disclosure | `tests/admin/orders-ui.spec.ts:1027,1039,1050` (call `await revealOrderFilters(page)` following `page.reload()` before clicking `orders-reset`; verified PASS in run 41170952247) |
| 124185561277 / WebKit buyer CVS (`cvs-picker.mjs:387`) | (c)-candidate | None (`tests/storefront/cvs-picker.mjs:387`). Out-of-card storefront CVS picker (#358 PARITY-CVS); verified PASS in run 41170952247 |

STATUS: DONE
