# PARITY-GROWTH-FIX2 — Failure Classification and Resolution Report

Status: Directed fix completed locally, ready for CI run.
Date: 2026-10-31 Asia/Taipei. PR #366 (`unit/parity-growth` -> `r3/integration`).
Failing CI Run: 41084050704 (attempt 1)

## Failing Jobs and Failure Classification Table

| Failing Job | Failing Test / Assertion | Evidence | Class | Resolution |
|---|---|---|---|---|
| 124178338229 (`--browser-operations-ads`) | `TestBrowserOperationsAds`: `getByTestId('ads-connections')` expected substring `"9132911371291"`, timed out after 10000ms at `tests/admin/operations-ads.spec.ts:189:1` | Run 41084050704, Job 124178338229, artifact `gate-browser-operations-ads` (run `SF21V9JR`), Playwright log lines 32–45 | (a) Older test asserts UI the signed Figma frame intentionally replaced | Signed Figma frame `5:17516` (Asset Authorization) moved the account connection controls from the root list screen to their dedicated screen reachable via the `g.assets` action ("View ad account connection" / "查看廣告帳號連結" / "查看广告账户连接"). Added `data-testid="ads-assets"` to `apps/admin/components/Ads.tsx:362`. Rewrote `tests/admin/operations-ads.spec.ts` to navigate using real click via `openAssets(page)` before asserting ad connection details and unbind controls, and after reloads. All original assertions, permissions checks, feed copying, unbind lifecycles, and visual audits preserved. |
| 124178338822 (`--browser-cvs`) | `TestBrowserTaiwanCvs/WebKit`: `getByTestId('cvs-candidate-2656')` expected visible within 5000ms, timed out at `exerciseCvsPicker (tests/storefront/cvs-picker.mjs:387:34)` | Run 41084050704, Job 124178338822, artifact `gate-browser-cvs` (run `GN1qYdQ4`), log line 277 | (c) Candidate base issue / outside card scope | Storefront CVS picker is outside PR #366 write paths (PR #366 is strictly `apps/admin` growth lane; zero diff in `apps/storefront/` or `tests/storefront/cvs*`). Identical intermittent WebKit candidate timeout at line 387 of `cvs-picker.mjs` was ruled outside card in PR #362. Furthermore, PR #351 (`37e097399`) on `r3/integration` merged CVS picker alignment, which was merged into this branch in Step 1. |
| 124214325754 (`receipts`) | Dependency failure | `matrix-receipts`: failed due to `browser-operations-ads` and `browser-cvs` | Dependency failure | Will resolve once dependent browser gates pass. |
| 124214689095 (`required`) | Dependency failure | `required`: blocked by `gate` and `receipts` | Dependency failure | Will resolve once dependent browser gates pass. |

## Step 1 Integration Merge
- Base `origin/r3/integration` (`3c8997eb3`) merged into `unit/parity-growth`.
- Merge commit: `b61a82628940d702ff97657a08cf652f02806abd`.
- Resolved conflicts in `output/ci-select-backend-browser/browser-ownership.json` and `packages/i18n/src/index.ts`.

## Evidence Retention Compliance (ADR-20261031)
- Removed raw CI artifact copies and bulk Figma screenshots from tracking (`git rm -r --cached output/parity-growth/relay2-failures output/parity-growth/figma`).
- Frame references are preserved in `output/parity-growth/frame-manifest.json` with node IDs and sha256 hashes.
- `node scripts/dev/check-evidence-retention.mjs --base origin/r3/integration` passes with budget well below 4 MiB.
- Browser ownership regenerated with `node scripts/dev/derive-browser-ownership.mjs` and verified with `--check`.
