# PARITY-GROWTH-FIX2 — Failure Classification and Resolution Report

Status: Directed fix completed locally, ready for CI run.
Date: 2026-10-31 Asia/Taipei. PR #366 (`unit/parity-growth` -> `r3/integration`).
Failing CI Run: 41084050704 (attempt 1) on head `4fdf52f5`
Latest base Gates: 41204331038 on 3c8997eb3 (success), merged through `faf6502b5`

## Failing Jobs and Failure Classification Table

| Failing Job | Failing Test / Assertion | Evidence | Class | Resolution |
|---|---|---|---|---|
| 124178338229 (`gate (bundle:09)` / `--browser-operations-ads`) | `TestBrowserOperationsAds`: `getByTestId('ads-connections')` expected substring `"9132911371291"`, timed out after 10000ms at `tests/admin/operations-ads.spec.ts:189:1` | Run 41084050704, Job 124178338229, artifact `gate-browser-operations-ads` (run `SF21V9JR`), Playwright log lines 32–45 | (a) Older test asserts UI the signed Figma frame intentionally replaced | Signed Figma frame `5:17516` (Asset Authorization) moved the account connection controls from the root list screen to their dedicated screen reachable via the `g.assets` action ("View ad account connection" / "查看廣告帳號連結" / "查看广告账户连接"). Added `data-testid="ads-assets"` to `apps/admin/components/Ads.tsx:362`. Rewrote `tests/admin/operations-ads.spec.ts:27-35,182-197,229,254-265,279-285` to navigate using real click via `openAssets(page)` before asserting ad connection details and unbind controls, and after reloads. All original assertions, permissions checks, feed copying, unbind lifecycles, and visual audits preserved. |
| 124178338822 (`gate (bundle:07)` / `--browser-cvs`) | `TestBrowserTaiwanCvs/WebKit`: `getByTestId('cvs-candidate-2656')` expected visible within 5000ms, timed out at `exerciseCvsPicker (tests/storefront/cvs-picker.mjs:387:34)` | Run 41084050704, Job 124178338822, artifact `gate-browser-cvs` (run `GN1qYdQ4`), log line 277 | (b) Shared helper race condition on WebKit | In `tests/storefront/cvs-picker.mjs:383-385`, when switching CVS brands in `exerciseCvsPicker`, wait for the brand-switch quote response (`POST /api/buyer/quotes`) before filling the store code. On WebKit, an in-flight quote resolution from the preceding brand click arrived during `query.fill("2656")`, triggering a React re-render of the picker and wiping the typed code. Waiting for `quoteWrite` and `aria-pressed="true"` eliminates the race without loosening timeouts or thresholds. |
| 124208027681 (`receipts`) | Dependency failure | `matrix-receipts`: failed due to `browser-operations-ads` and `browser-cvs` | Cascade | Will resolve once dependent browser gates pass. |
| 124209655416 (`required`) | Dependency failure | `required`: blocked by `gate` and `receipts` | Cascade | Will resolve once dependent browser gates pass. |

## Step 1 Integration Merge
- Base `origin/r3/integration` merged into `unit/parity-growth`.
- Merge commit: `81e9d425c` (bringing latest base commit `faf6502b5`).
- Clean merge of `output/ci-select-backend-browser/browser-ownership.json`.

## Evidence Retention Compliance (ADR-20261031)
- Removed raw CI artifact copies and bulk Figma screenshots from tracking (`git rm -r --cached output/parity-growth/relay2-failures output/parity-growth/figma`).
- Frame references are preserved in `output/parity-growth/frame-manifest.json` with node IDs and sha256 hashes.
- `node scripts/dev/check-evidence-retention.mjs --base origin/r3/integration` passes (34 output/ files, 652587 bytes vs origin/r3/integration; ADR-20261031 budget well below 4 MiB).
- Browser ownership regenerated with `node scripts/dev/derive-browser-ownership.mjs` and verified with `--check`.
