# PARITY-BUYER-AFTER-FIX2 (PR #444, run 41244447379)

Evidence limits: Playwright output sits in CI artifacts, which this lane cannot download (tool allowlist). Each row is derived from the failing
job log (job id, Go test line) plus a static read of the spec the Go test drives against the PR diff. Nothing was run locally.

| job (id) | failing test | spec driven | cause | class | fix |
|---|---|---|---|---|---|
| 124665648696 | TestBrowserStorefront (browser_storefront_test.go:393, exit 1) | tests/storefront/shop-real-gate.mjs `checkBuyerParityStates` | lookup button label is now `查詢訂單` / `查询订单` (signed frame 26:708 default state); the old spec clicked `查詢` exact | a | `lookupSubmit` zh-TW / zh-CN updated (en unchanged: `Find order`) |
| 124665648696 | same | same, `expired` case | order-link refusal is drawn as frames 26:746 / 26:1028 (h1 + h2 + body + link), no `section[data-variant="expired"]` | a | expired copy = frame title/body, panel = `order-link` main, same real click on the lookup link |
| 124665648625 | TestBrowserE2EDealLoop | tests/e2e/deal-loop.spec.ts:231 | claim button `直接結帳` replaced by the single frame action `前往結帳（共 N 件）` | a | button located by the frame label regexp |
| 124665648897 | TestBrowserLiveClaimsRealChain | tests/admin/claims-ui.spec.ts | h1 / price note / action label / not-found text changed to frames 4:12347, 66:1746 (not-found now holds body + note + cart link, so exact `toHaveText` cannot hold) | a | copies table set to the frame strings; not-found uses `toContainText` of the frame body |
| 124665648881 | TestBrowserReturnsUI | tests/admin/returns-ui.spec.ts (admin only) | the PR diff touches no admin, Go or returns file; the harness seeds through merchant APIs; no buyer page is walked | c? | none; merged newer `origin/r3/integration`. If red again: BLOCKED with the run's evidence file |
| receipts / required / aggregate | | | fail only because of the jobs above | - | - |
