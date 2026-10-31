# GEMINI-REPORT — PARITY-SHIPCENTER-FIX2

Card: PARITY-SHIPCENTER-FIX2 (PR #353, branch `unit/parity-shipcenter`)
Failing run: `41103751079` (head `f40fa7851cf7bbb45425d090ca0482d3781559c4`)
Base: `origin/r3/integration` (`37e097399`, base Gates `41181348933`)

| Failing job / test | Class | What you changed, file:line |
|---|---|---|
| `receipts` (`124244797045`) / `--browser-design`, `--browser-product-media-v2` receipt check | (c) CI artifact-download transient (`Unable to download and extract artifact: Artifact download failed after 5 retries`; producer jobs passed and uploaded artifacts; `--browser-merchant-orders-ui` wrote `PASS test_count=90`) | None in receipt verifier; documented in `output/parity-shipcenter/FIX2.md:20` |
| `aggregate` (`124244796911`) / `G-UI8 click sweep aggregate`, `G-UI9 visual lint aggregate` | (c) CI artifact-download transient (all 10 click-sweep and 4 visual-lint producer shards passed; `actions/download-artifact` missed shard artifacts in aggregate) | None in aggregate verifier; documented in `output/parity-shipcenter/FIX2.md:21` |
| `required` (`124247784779`) / workflow gate summary | (c) downstream failure from `receipts` and `aggregate` (`gate=success`) | None; documented in `output/parity-shipcenter/FIX2.md:22` |
| `origin/r3/integration` merge & `bundle:09` (`--browser-merchant-orders-ui`) / `tests/admin/shipping-center.spec.ts` `v2` list DTO compatibility with PR #362 (`71e403e8f`) | (a) signed `orders-v2` `item_summary` contract alignment on merge | `apps/admin/src/features/orders/routes.ts:1-6` (resolved merge conflict keeping `order-detail` and `shipping-center`); `tests/admin/orders-ui.spec.ts:4-8` (resolved merge conflict keeping `TestInfo` and `registerShippingCenterTests`); `tests/admin/shipping-center.spec.ts:69-79,110,158-168` (added required `item_summary` to synthetic `v2` order summaries after `origin/r3/integration` added strict `parseItemSummary` in `orders-v2.ts`); `output/ci-select-backend-browser/browser-ownership.json:1-56` (regenerated ownership) |

STATUS: DONE
