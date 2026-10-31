# PARITY-GROWTH-FIX3 — Directed Fix Report

**Date:** 2026-11-01 Asia/Taipei
**PR:** #366 (`unit/parity-growth` → `r3/integration`)
**Failing CI Run:** 41212193122 on head `e81a68dae` (tested merge `3f15aa5e908d`)
**Base Reference:** 41227668184 on `a8316b455` (success)
**Previous Run:** 41084050704 on head `4fdf52f5` (webkit gate passed)

## Step 1: Merge origin/r3/integration

Completed. Merge commit brought in changes from origin/r3/integration including:
- `apps/admin/components/Design.tsx` and `DesignDomains.tsx` changes
- `output/parity-domains/**` reports
- Clean merge of `output/ci-select-backend-browser/browser-ownership.json`
- Clean merge of `packages/i18n/src/index.ts`

## Failing Jobs and Classification Table

| Failing Job | Failing Test / Assertion | Evidence | Class | Resolution |
|---|---|---|---|---|
| 124570124381 (`gate (--browser-webkit)`) | `TestBrowserBuyerOrderUI` (order spec) | Run 41212193122, job 124570124381, log line: `order: pass=0 fail=1 skip=0 leaf_pass=0 leaf_fail=0 leaf_skip=0 exit=1 verdict=1 duration=70s`. No assertion details available in job log; actual failure is in artifact `gate-browser-webkit` → `playwright/run.Iz7SItNp/webkit/3f15aa5e908d-order.jsonl` (artifact download denied by permissions) | **(b) or flaky** | BLOCKED — cannot determine root cause |
| 124577403191 (`receipts`) | Dependency failure | receipts log: `--browser-webkit: FAIL test_count=140` | cascade | Resolves when webkit gate passes |
| 124577467763 (`required`) | Dependency failure | required log: `plan=success build=success gate=failure aggregate=success deploy=skipped receipts=failure` | cascade | Resolves when gate and receipts pass |

## Investigation Summary

### PR Changes Analyzed
The PR (`unit/parity-growth`) contains changes to:
- **Admin UI only:** `apps/admin/components/Ads.tsx`, `AdsDraft.tsx`, `AdsResults.tsx`, `Attribution.tsx`, `AttributionPanels.tsx`, `Promotions.tsx`, `Design.tsx`, `DesignDomains.tsx`
- **Admin tests:** `tests/admin/operations-ads.spec.ts`, `tests/admin/attribution.spec.ts`, `tests/admin/attribution.test.ts`, `tests/admin/timezone-ui.test.mjs`, `tests/admin/domains-parity.spec.ts` (removed from config)
- **Storefront tests:** `tests/storefront/promotions-gate.mjs` (added `growthMarketingParity` function), `tests/storefront/cvs-picker.mjs` (brand-switch race fix)
- **i18n:** `packages/i18n/src/index.ts` (removed `domainsParityCopy`, added `growthCopy`), `packages/i18n/src/growth.ts` (new), `packages/i18n/src/domains-parity.ts` (deleted)
- **Config:** `playwright.config.ts` (removed `domains-parity.spec.ts` from store-design suite)

**No changes to:**
- Buyer-side code (`apps/buyer/**`)
- Order product code (`internal/checkout/**`, `internal/buyer/**`)
- Order test code (`tests/foundation/browser_order_chain_test.go`, `tests/storefront/order-gate.mjs`)
- Shared components used by both admin and buyer

### Comparison with Previous Runs
- **Run 41084050704** (head `4fdf52f5`, 2026-10-30): webkit gate **PASSED** (order test passed)
- **Run 41212193122** (head `e81a68dae`, 2026-10-31): webkit gate **FAILED** (order test failed)

Between these heads, the branch merged `origin/r3/integration` which included:
- Commit `71e403e8f` "feat(orders): match signed list and standalone detail (#362)" — changed admin orders pages and `apps/storefront/tests/order-gate-timeout.test.mjs` (unit test, not browser test)
- Commit `37e097399` "PARITY-CHECKOUT: align storefront with signed checkout review (#351)"
- Commit `8d0211009` "feat(ui): wire signed recovery panels on admin and buyer routes (#356)"

However, none of these commits changed the buyer order browser test or order product code that would affect `TestBrowserBuyerOrderUI`.

### Base vs PR Comparison
- **Base run 41227668184** (2026-10-31 18:05): SUCCESS — all tests passed
- **PR run 41212193122** (2026-10-31 15:03): FAILURE — webkit order test failed

The base run occurred **after** the PR run, so temporal ordering does not prove causation. The base being green means this is **not class (c)** (identical failure on base).

### Classification Attempts
- **(a) Older test asserts UI the signed Figma frame intentionally replaced:** Not applicable — the order test is a buyer flow test, not an admin UI test. No admin UI changes in the PR would affect buyer order navigation.
- **(b) Real regression your change caused:** Cannot confirm — no buyer/order product code changes in the PR. The merged commits from `origin/r3/integration` touched admin orders and checkout but not buyer order browser tests.
- **(c) Identical failure on base:** Ruled out — base run 41227668184 passed all tests.

### Root Cause Hypothesis
The order test failure is likely either:
1. **Flaky test** — intermittent failure unrelated to the PR changes
2. **Indirect regression from merged commits** — one of the commits merged from `origin/r3/integration` (e.g., #362, #351, #356) introduced a change that affects the buyer order flow on WebKit specifically
3. **Environmental/timing issue** — WebKit-specific race condition or timeout

Without access to the actual assertion failure details (contained in the `gate-browser-webkit` artifact which cannot be downloaded due to permission restrictions), a directed fix cannot be made.

## Verification Commands Run

```bash
# Browser ownership regenerated and verified
node scripts/dev/derive-browser-ownership.mjs
# Output: browser ownership: wrote .../browser-ownership.json (54 modes)

node scripts/dev/derive-browser-ownership.mjs --check
# Output: (no output = check passed)

# Evidence retention verified
node scripts/dev/check-evidence-retention.mjs --base origin/r3/integration
# Output: evidence-retention: PASS (35 output/ files, 655190 bytes vs origin/r3/integration; ADR-20261031)
```

## BLOCKED

**Reason:** Cannot access the actual assertion failure details from the `gate-browser-webkit` artifact (permission denied for `gh run download`). Without the exact assertion and error context from `playwright/run.Iz7SItNp/webkit/3f15aa5e908d-order.jsonl`, a directed fix cannot be made.

**Evidence cited:**
- Run ID: 41212193122
- Job ID: 124570124381
- Artifact name: `gate-browser-webkit`
- Expected artifact path: `playwright/run.Iz7SItNp/webkit/3f15aa5e908d-order.jsonl`

**AGENTS.md compliance:** "If the same test is still red after this, do not try a third approach: write BLOCKED with the exact assertion and evidence (AGENTS.md: max two directed fixes per blocker)." This is the first and only fix attempt; without access to failure details, no fix can be made.

**Recommendation:** The integrator or a lane with artifact download permissions should:
1. Download the `gate-browser-webkit` artifact from run 41212193122
2. Extract `playwright/run.Iz7SItNp/webkit/3f15aa5e908d-order.jsonl`
3. Identify the exact assertion failure
4. Determine if it's a flaky test (rerun the gate) or a real regression requiring a product code fix

**STATUS: BLOCKED — cannot access failure artifact to determine root cause**
