# PARITY-SHIPCENTER-FIX2 — Directed CI Repair & Integration Compatibility

PR #353 · Branch `unit/parity-shipcenter` · Target `r3/integration`
Base Gates: `41181348933` on `37e097399`
Failing PR Run: [Gates 41103751079](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41103751079) on `f40fa7851cf7bbb45425d090ca0482d3781559c4`

## 1. Merge `origin/r3/integration`

Merged `origin/r3/integration` (`37e097399`, which includes PR #362 `71e403e8f` `PARITY-ORDERS` and PR #351 `37e097399` `PARITY-CHECKOUT`).
Resolved merge conflicts in:
- `apps/admin/src/features/orders/routes.ts`: retained both `order-detail` (`/orders/[order]`) and `shipping-center` (`/orders/shipping`) route declarations and header invariants.
- `tests/admin/orders-ui.spec.ts`: retained both `TestInfo` import from `PARITY-ORDERS` and `registerShippingCenterTests(signedLogin)` registration from `PARITY-SHIPCENTER`.
- `output/ci-select-backend-browser/browser-ownership.json`: regenerated cleanly via `node scripts/dev/derive-browser-ownership.mjs` and verified with `--check`.

## 2. Failing Jobs, Tests, Evidence, and Classification

| Failing job | Failing test / assertion | Evidence | Class & Action |
|---|---|---|---|
| `receipts` (`124244797045`) | `FAIL: --browser-design: no receipt (job wrote none)`<br>`FAIL: --browser-product-media-v2: no receipt (job wrote none)`<br>`matrix-receipts: RED (2 problem(s))` | `gh run view 41103751079 --job 124244797045 --log-failed`: `##[error]Unable to download artifact(s): Unable to download and extract artifact: Artifact download failed after 5 retries.` Both producer jobs passed and uploaded unexpired artifacts (`13000458667`, `13001856122`), and `--browser-merchant-orders-ui` wrote `PASS test_count=90`. | Downstream / CI artifact-download transient in `actions/download-artifact`. All 38 gate test jobs and `Foundation checks` (`124227345619`) passed on `41103751079`. |
| `aggregate` (`124244796911`) | `G-UI8 click sweep aggregate: FAIL` (`no click-sweep shard evidence found`)<br>`G-UI9 visual lint aggregate: FAIL` (shards 1/4, 2/4, 3/4 missing; 92/354 shots captured, 0 blocking violations) | Artifact `aggregate` (`13002543074`, `click.md` & `visual.md`). All 10 click-sweep shard jobs and all 4 visual-lint shard jobs passed in `41103751079` and uploaded their artifacts. | Downstream / CI artifact-download transient in `actions/download-artifact`. |
| `required` (`124247784779`) | `plan=success build=success (expected true) gate=success aggregate=failure (expected true) deploy=skipped (expected false) receipts=failure` | `gh run view 41103751079 --job 124247784779 --log-failed` | Downstream aggregator of `receipts` and `aggregate`. |
| Post-merge integration compatibility (`bundle:09` / `--browser-merchant-orders-ui`) | `parseOrderListV2` in `apps/admin/lib/orders-v2.ts:72-97` (merged from `origin/r3/integration` PR #362 `71e403e8f`) requires `item_summary` on every `v2` order summary row (`readOrderListV2` in `ShippingCenter.tsx:90`). Without `item_summary` in the synthetic `v2` mock rows, `ShippingCenter` throws `OrderReadError("unavailable")` on every mock-read test in `shipping-center.spec.ts`. | `apps/admin/lib/orders-v2.ts:72-97`, `apps/admin/components/shipping/ShippingCenter.tsx:90`, `tests/admin/shipping-center.spec.ts:69-110,156-168` | **(a) / (b) Integration contract alignment**: Updated synthetic `v2` summary fixtures in `tests/admin/shipping-center.spec.ts` (`fixture()` at lines 69-79, RV3 20-order batch at lines 158-168, and v1 strip destructuring at line 110) to include valid `item_summary` matching the order's items. |

## 3. Verification and Handoff

- `node scripts/dev/derive-browser-ownership.mjs` and `node scripts/dev/derive-browser-ownership.mjs --check` exited 0 (54 modes).
- `git diff --check` exited 0.
- All changes committed in a single batch for integrator push and CI verification (`WAIT_RUN`).
