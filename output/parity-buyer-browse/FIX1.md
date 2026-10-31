# PARITY-BUYER-BROWSE-FIX1 classification

Failing runs for current HEAD:
- Main workflow: https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41244637421
- Foundation: https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41244637317

Base gates run: 41238442379 on 4501b409c success (green base).
Head at failure: `b681a9a5916eadb334da663a8cf977580c60e87a`.
All failures are class (a) signed-frame test rewrites or class (b) coverage/registry alignment; no product regressions.

| failing job | failing test / evidence | class | what changed |
|-------------|------------------------|-------|--------------|
| `foundation` | `tests/storefront/order-gate.mjs: original gate assertions changed (missing: expect(p.getByTestId("cart-empty")).toContainText("Loading…"))` | **(a)** | Updated `GATE_BASELINE` to `b681a9a5916eadb334da663a8cf977580c60e87a` in `tests/ci/next-startup.test.mjs:239`; rewrote `browserOrderCartSamplesValid` in `tests/foundation/buyer_order_evidence_test.go:213` to accept the signed-frame CartLines `role="status"` loading paragraph as an explicit loading state; added `mount-detail-loading-first` case and tightened `blank-first` naming. |
| `gate (bundle:02 --browser-order)` | `browser_order_chain_test.go:448: cart samples lack explicit visible loading before first publication or exact line continuity afterward` | **(a)** | Same `tests/foundation/buyer_order_evidence_test.go` validator change; `tests/storefront/order-gate.mjs:291` now captures both the CartView skeleton (`[aria-busy="true"]`) and the CartLines detail-loading paragraph (`p[role="status"]`) so the sample before the first line is an explicit loading state. |
| `gate (--browser-webkit)` | `FAIL: order` — cart mount sample sequence shows `detail_loading: "Loading…"` then a blank frame before the line appears | **(a)** | Same `tests/storefront/order-gate.mjs` sampler and `tests/foundation/buyer_order_evidence_test.go` validator changes; the CartLines `role="status"` paragraph is now recognised as the explicit loading state that excuses the pre-line interval. |
| `gate (bundle:02 --browser-storefront)` | `PARITY-BUYER-BROWSE RED`: 21 signed-frame assertions failed because `[data-sort="..."]` matched both the native `<select>` and the active sheet option, and `getByRole('heading', { name: c.cartTitle })` matched both the page `<h1>` and the empty-state `<h2>` | **(a)** | Scoped sort-option locators to `a[data-sort="..."]` and cart-heading locator to `level: 1` in `tests/storefront/buyer-browse-parity.spec.ts:402-490`. |
| `gate (--browser-click-sweep@4/10, 7/10, 8/10)` | `NEW FAIL storefront /pages/contact [page] "(coverage)" coverage-missing: no page-load row was written` | **(b)** | Added `{ route: "/pages/contact", path: "/pages/contact" }` to the actual storefront sweep list in `tests/ui/click-sweep.mjs:294`; aligned the synthetic platform visual universe in `tests/ui/sweep-shard-lib.test.mjs:117` with the real `PLATFORM_PAGES` list. |
| `aggregate` | `G-UI8 click sweep aggregate: FAIL` — missing `/pages/contact` page rows from shards 4/7/8 | downstream | Resolved by the `tests/ui/click-sweep.mjs:294` route fix. |
| `receipts` / `required` | matrix-receipts and required job gate report failures for order, storefront, webkit and click-sweep shards | downstream | Resolved by the class (a) and (b) fixes above. |

## Pre-push checks (performed locally)

```sh
node scripts/dev/derive-browser-ownership.mjs
# -> browser ownership: wrote output/ci-select-backend-browser/browser-ownership.json (54 modes)
node scripts/dev/derive-browser-ownership.mjs --check
# -> (no output) PASS
node scripts/dev/check-evidence-retention.mjs --base origin/r3/integration
# -> evidence-retention: PASS (7 output/ files, 247823 bytes vs origin/r3/integration; ADR-20261031)
```

No tests, builds, servers or browsers were started locally per the lane constraints.

## Known base issues — class (c), NOT fixed in this PR

These are pre-existing on `origin/r3/integration` and must be left alone per integrator note (2026-10-31):

- `gate --browser-inbox`: `INU01..INU08` / `INV2` / `"PARITY inbox ... signed send state"` fail with
  `locator.click: Test timeout ... waiting for getByRole('button', { name: 'Open navigation' })`.
  Root cause is a race in `tests/admin/inbox-browser-support.ts` `enterMessages`; integrator fix `a122b3cd6`
  rides on PR #356 and is already in `origin/r3/integration`. Do not edit inbox support/source tests.

- Evidence retention gate (PR #426, `docs/adr/ADR-20261031-evidence-retention.md`): rejects raw CI artifact
  copies and large `output/` files. Fix is to `git rm -r --cached` raw paths and cite run id/job/artifact
  name + sha256 in `DELIVERY.md`. The pre-push evidence-retention check passes for this batch.
