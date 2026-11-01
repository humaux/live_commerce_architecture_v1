# GEMINI-REPORT — PARITY-ORDERS-FIX2 (RELAY 2)

Card: PARITY-ORDERS-FIX2 (PR #362, branch unit/parity-orders)
Head: 9bce2ffc (Failing run 41173061778, jobs 124455723244 gate (bundle:08), 124455723172 gate (bundle:09), 124455723037 gate (bundle:06))
Base: origin/r3/integration (merged acf2ebd2c, base Gates 41107905131)
Integrator Ruling: 2026-10-31 resolves FIX2.md BLOCKED

## Failure Classification and Changes

| Failing job / test | Class | What changed, file:line |
|---|---|---|
| 124455723244 (bundle:08) / `TestBrowserStaffTeam` (`staff-team.spec.ts:175`) | (a) test helper teardown rejection | `tests/admin/staff-team.spec.ts:175-177`: caught promise rejection on `r.headerValue("referer")` during context close to prevent unhandled rejection crash when context teardown races with in-flight requests |
| 124455723172 (bundle:09) / `TestBrowserMerchantOrdersUIRealChain` (`parcel-merge.spec.ts:404`) | (a) signed frame navigation / setup helper rewrite | `tests/admin/parcel-merge.spec.ts:395`: added `await orderList(page)` at the conclusion of the single-order shipment refusal step to return to the order list before the subsequent step's reload with `/parcel-groups` fault injection |
| 124455723037 (bundle:06) / `TestBrowserInboxUIRealChain` | (c) out-of-card | None (`tests/admin/inbox*.spec.ts`). Out-of-card admin inbox suite (#361 PARITY-INBOX-EDITOR); untouched by parity-orders |
| 124449546996 (bundle:09) / `TestBrowserMerchantOrdersUIRealChain` (`parcel-merge.spec.ts:69`) | (a) signed frame navigation / setup helper rewrite | `tests/admin/parcel-merge.spec.ts:68-70`: robust `orderList(page)` to await `order-detail-back.click()` when on standalone detail route or mounted detail view rather than checking non-waiting `count()` immediately after `page.reload()`; lines 226, 235, 315: added `await orderList(page)` after asserting persisted member details |
| 124185561381 (bundle:08) / `TestBrowserOC` (`reauth.spec.ts:188`) | (a) signed frame disclosure | `tests/admin/oc/reauth.spec.ts:188` (reveal orders-more-filters after reload; verified PASS in runs 41170952247 and 41173061778) |
| 124185562324 (bundle:09) / `TestBrowserMerchantOrdersUIRealChain` (`orders-ui.spec.ts:168`) | (b) product root fix | `apps/admin/components/orders/parity.css:32` (scoped mobile selection bar display; verified PASS in runs 41170952247 and 41173061778) |
| 124185562324 (bundle:09) / `TestBrowserMerchantOrdersUIRealChain` (`orders-ui.spec.ts:495`) | (a) signed frame disclosure | `tests/admin/orders-ui.spec.ts:500` (revealOrderFilters before refresh click; verified PASS in runs 41170952247 and 41173061778) |
| 124185562324 (bundle:09) / `TestBrowserMerchantOrdersUIRealChain` (`orders-ui.spec.ts:974`) | (b) product root fix | `apps/admin/components/MerchantOrders.tsx:185` (move localized empty-state container inside orders-list; verified PASS in runs 41170952247 and 41173061778) |
| 124185562324 (bundle:09) / `TestBrowserMerchantOrdersUIRealChain` (`orders-ui.spec.ts:1029`) | (a) signed frame disclosure | `tests/admin/orders-ui.spec.ts:1027,1039,1050` (call `await revealOrderFilters(page)` following `page.reload()` before clicking `orders-reset`; verified PASS in runs 41170952247 and 41173061778) |
| 124185561277 / WebKit buyer CVS (`cvs-picker.mjs:387`) | (c)-candidate | None (`tests/storefront/cvs-picker.mjs:387`). Out-of-card storefront CVS picker (#358 PARITY-CVS); verified PASS in runs 41170952247 and 41173061778 |

STATUS: DONE
