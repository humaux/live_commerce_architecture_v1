# GEMINI-REPORT — PARITY-GROWTH-FIX2 R1 (PR #366)

Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/parity-growth`
Branch: `unit/parity-growth`
Base: `3c8997eb3e9c269c6667ab62a09d0e90d56da529`
CI Run: `41084050704` (PR #366 head `4fdf52f5`)

| Failing job / test | Class (a/b/c) | What changed (`file:line`) |
|---|---|---|
| `124178338229` (`gate (bundle:09)` / `--browser-operations-ads`) → `TestBrowserOperationsAds` (`getByTestId('ads-connections')` expected substring `"9132911371291"` at `tests/admin/operations-ads.spec.ts:189:1`) | **a** | `apps/admin/components/Ads.tsx:362`: Added `data-testid="ads-assets"` to the asset authorization navigation action (`g.assets`). `tests/admin/operations-ads.spec.ts:27-35,182-197,229,254-265,279-285`: Rewrote older assertions to navigate via real user click (`openAssets(page)`) to reach the signed Figma frame `5:17516` (Asset Authorization) screen where account connections, unbind controls, and feed copy reside, including after reloads. Preserved all original assertions, permissions checks, feed copying, unbind lifecycles, and visual audits. Regenerated `output/ci-select-backend-browser/browser-ownership.json:137`. Untracked raw CI artifact copies and bulk Figma captures per ADR-20261031. |
| `124178338822` (`gate (bundle:07)` / `--browser-cvs`) → `TestBrowserTaiwanCvs/WebKit` (`getByTestId('cvs-candidate-2656')` expected visible at `tests/storefront/cvs-picker.mjs:387:34`) | **b** | `tests/storefront/cvs-picker.mjs:383-385`: Settle brand switch quotation request (`POST /api/buyer/quotes`) before filling store code in `exerciseCvsPicker`. Previously, on WebKit the asynchronous quote response and subsequent re-render/remount arrived after `query.fill("2656")`, clearing the input and causing `cvs-candidate-2656` lookup to fail. The helper now waits for `await quoteWrite` and `aria-pressed="true"` before typing the store code. |
| `124208027681` (`receipts`), `124209655416` (`required`) | Cascade | Downstream dependency guards; fail strictly because of `bundle:09` and `bundle:07` above. Resolves automatically once upstream gates pass. |

STATUS: DONE
