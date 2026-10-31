# PARITY-CATALOG-PEOPLE-FIX2 — GEMINI REPORT (RELAY 3)

## Directed Fix Summary

This directed fix resolves the CI failures on PR #363 head `0c7f9b37d` (Gates run `41173763994`).
Three root causes were identified and fixed in a single batch:
1. Bundle 03 (`124458076046`): In `tests/admin/product-review.acceptance.ts:226`, after page reload, the copy action lands on the list page where the recovery fence disables `products-copy-entry` (not `products-copy-selected` which only exists in the confirmation modal).
2. Bundle 06 (`124458077108`): In `tests/admin/inbox-browser-support.ts:77`, `enterMessages` had a race condition where `if (!await nav.isVisible())` evaluated before desktop layout settled, erroneously triggering mobile `Open navigation` (absent on desktop) and timing out.
3. Click-Sweep (`124458077587`): In `tests/ui/click-sweep.mjs:444-447`, journey J1 attempted to fill the unsigned `products-search` input which was removed by the signed Figma design. Replaced with clicking the signed `products-tab-active` tab.

Browser ownership was regenerated and verified with `--check`.

## Failure Classification and Changes

| Failing Job / Test | Class | What Changed (File:Line) |
|---|---|---|
| `124458076046` / `gate (bundle:03)` / `TestBrowserCatalogCore/PE12-17-TWD` (`catalog-core.spec.ts >> review: copy UNKNOWN then client/401/403 preserves receipt fence after reload`) | (a) Older test asserts UI that signed Figma frame intentionally relocated | `tests/admin/product-review.acceptance.ts:226`: assert `action === "copy" ? page.getByTestId("products-copy-entry") : trigger()` is disabled after reload, matching `product-editor.acceptance.ts:2020` |
| `124458077108` / `gate (bundle:06)` / `TestBrowserInboxUIRealChain` (`tests/admin/inbox-ui.spec.ts`, tests 8, 10, 11, 14, 17, 18) | (b) Regression in shared browser test helper `enterMessages` | `tests/admin/inbox-browser-support.ts:77-81`: replace instantaneous `!await nav.isVisible()` with `try { await expect(nav).toBeVisible({ timeout: 500 }); } catch { await page.getByRole("button", { name: "Open navigation", exact: true }).click(); }` |
| `124458077587` / `gate (--browser-click-sweep@4/10)` / `journey J1 product editor -> storefront desktop/zh-TW [journey] "the merchant list shows the product (search + click)"` | (a) Older test asserts UI that signed Figma frame intentionally removed | `tests/ui/click-sweep.mjs:444-447`: replace obsolete `products-search` fill/submit with real status tab click `await page.getByTestId("products-tab-active").click()` |
| `124459434850` / `receipts` | Dependent aggregate failure | Fails strictly due to bundles 03, 06, and click-sweep@4/10 |
| `124459711685` / `required` | Dependent aggregate failure | Fails strictly due to the above failing jobs |
| `124458078696` / `aggregate` | Dependent aggregate failure | Fails strictly due to the above failing jobs |

STATUS: DONE
