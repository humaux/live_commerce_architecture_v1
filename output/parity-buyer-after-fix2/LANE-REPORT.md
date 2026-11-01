# LANE-REPORT PARITY-BUYER-AFTER-FIX2

| finding | class | change |
|---|---|---|
| TestBrowserStorefront: lookup submit label | a | tests/storefront/shop-real-gate.mjs lookupSubmit zh-TW/zh-CN (`查詢訂單` / `查询订单`) |
| TestBrowserStorefront: expired order-link panel | a | tests/storefront/shop-real-gate.mjs expired copy + panel locator (`order-link`) |
| TestBrowserE2EDealLoop: claim action | a | tests/e2e/deal-loop.spec.ts:231 frame label regexp |
| TestBrowserLiveClaimsRealChain: claim copy / not-found | a | tests/admin/claims-ui.spec.ts:449-451 and the not-found assertions (toContainText) |
| TestBrowserReturnsUI | c? (no PR path; admin only) | none; merged origin/r3/integration; see output/parity-buyer-after/FIX2.md. No artifact access, failing assertion unknown |

Merged origin/r3/integration first. No tests run (owner rule).

STATUS: DONE
