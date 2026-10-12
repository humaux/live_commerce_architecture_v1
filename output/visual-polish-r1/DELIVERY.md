<!-- Purpose: VISUAL-POLISH-R1 source handoff with measured gates, screenshot provenance and outstanding visual acceptance.
Depends on: owner base64ba8319, K2.8 findings over CI38207265014, shared format/UI and existing browser registry.
Used by: integrator commit/K3 review/CI; this report does not claim unrun browser or screenshot acceptance. -->
# VISUAL-POLISH-R1 delivery

READY for the integrator to commit the files. Full visual acceptance remains pending. Branch `unit/visual-polish-r1`, supplied base `64ba8319`; no author Git operation, push or PR. Primary Codex ui_worker; one ui_worker owned ads, one test_worker owned capture/tests, explorer and independent source reviews were read-only. Models/effort inherited; deployed variant not exposed. All writes used disjoint paths in the owner-provided worktree because Git metadata is read-only; no recursive delegation.

## Changes and causes

1. **Ads money:** `AdsResults` already called `formatMinor`, but `ads-model.ts` owned another Intl currency formatter. That produces bare `$0` in zh-TW. `formatMinor` now delegates shared `money(locale,currency,minor)`, preserving x100 minor units and the existing fallback. Literal `$0` vs `NT$0` RED preceded the fix. All3locale/TWD-cent/whole/USD/HKD vectors pass. The existing ads browser matrix now includes1440/390 and asserts the TWD symbol before screenshots.
2. **Mobile product sections:** reuse the same `TabStrip` and localized `presentationCopy` as orders, only at the existing <=900px breakpoint. Native scroll/hint lives inside the old navigation landmark; desktop buttons, focus/aria-controls and pane behavior stay intact. No new UI component. Shared active-tab scrolling follows the existing current section.
3. **Save status:** the <=680px `max-width:90px` cap broke the unchanged localized phrase. The status now occupies its own full row; empty status is hidden, actions have their own wrapping row. No copy meaning changed. Structural regressions RED8/2 -> GREEN10/0; these prove source constraints, not rendered pixels.
4. **Full-form capture:** the production editor deliberately reserves viewport space and scrolls `.pe-fields`. Document fullPage capture does not reveal this pane's remaining content. CI images are929px (TW/CN) and950px (en); en is also a current-pane capture, contrary to the review's complete-form hypothesis. Keep this intentional product layout. Visual-audit now adds bounded native-wheel viewport frames covering the entire visible pane, including tall section interiors, with hashes/geometry/contiguous ranges in index.json. Frozen/document-only motion, gaps, frame limits or inaccessible end fail required state coverage. It also reuses the full capture/lint pipeline at supplemental1440x900 for affected pages/all locales; original1586/390 primary matrix/counts and lint thresholds remain unchanged. Supplemental blockers/failures contribute to the owning verdict. Inner frames are screenshot coverage, not per-frame lint passes.

Real product acceptance adds1440 without dropping old widths, six edit locale/width variants without extra writes, visible hint/status-row geometry, native scrolling to the final search control and per-section/end screenshots. The form remains797lines; no architecture allowance was widened.

## Actual commands and exits

All raw logs/status/hashes are in **main checkout** `/Volumes/data/live_commerce_architecture_v1/output/visual-polish-r1/`. `SOURCE.json` binds current source hashes and exact argv. No source changed during final gates.

| Command | Exit | Evidence/result |
|---|---:|---|
| Money-focused Node test on unchanged formatter | 1 -> 0 | `ads-money-red.log` literal `$0 !== NT$0`; `ads-money-green.log`24/24 |
| `node --test --experimental-strip-types tests/admin/product-document.test.ts` | 1 -> 0 | `product-mobile-red/green`:8 pass2 fail ->10 pass0 fail; structural only |
| Capture baseline and supplemental guards | 1 -> 0 | `capture-red`, `capture-supplement-red` (when present), `capture-green`, `capture-receipt.json`; old single-shot strategy and missing1440 verdict fail; helper regression38/38 |
| Focused Node money/product/capture/old lint/shard suites | 0 | `focused-node.status.json` exact argv;72/72, no skip |
| `pnpm --filter @live-commerce/admin typecheck` | 0 | `admin-types.log/status.json` |
| Strict `pnpm exec tsc` on changed browser specs/helpers | 0 | `spec-types-green.status.json` exact argv |
| `bash scripts/dev/test-node.sh` | 1 | `test-node.log`: first batch452 pass/3 fail (team-bff listenEPERM, two deploy-prep Docker socket refusals). Later script batches NOT_RUN; never a full Node PASS |
| `bash scripts/dev/check-gates.sh` | 0 | `check-gates-retry.status.json`: bounded300s run exited0 after252.56s. Initial120s wrapper had interrupted/missing completion status after cold Go check; its log retained, not counted as PASS |
| `bash scripts/dev/test-local.sh --list` | 0 | `mode-list.log` |
| Seven affected browser mode `--dry-run` commands | 0 each | `browser-gates.json` and mode-dry-run logs; metadata only, no browser/PG/build |
| Current `planPr` API with explicit working sources | 0 | `pr-mode-plan.json`:52 browser modes selected; no Git source lookup. Broad plan is preserved for CI |
| Impeccable detector on modified production targets | 0 | `impeccable-detector.log`:[]; one bounded detector pass, not visual proof |
| `pnpm install --offline --frozen-lockfile` | 0 | Initial worktree lacked node_modules/tsc. Reused49 cached packages,0 downloads; setup-only initial tsc254 retained in `spec-types.log`, not acceptance RED |
| Docker info / loopback listener preflights | 1 / 1 | `runtime-docker.log` socket denied; `runtime-listen.log` EPERM |

## Screenshots and visual limits

Before evidence: `before-manifest.json` contains24 copied, unmodified synthetic CI screenshots from run38207265014, all3locales at1586/390 for ads, new, new-variants and edit. They are prior-CI evidence, not a fresh base64ba8319 retake. No1440 before image exists. Representative files:

- [Ads zh-TW390 before](/Volumes/data/live_commerce_architecture_v1/output/visual-polish-r1/before/ads/zh-TW-390x844.png)
- [Product new zh-TW390 before](/Volumes/data/live_commerce_architecture_v1/output/visual-polish-r1/before/products-new/zh-TW-390x844.png)
- [Product variants/save status zh-TW390 before](/Volumes/data/live_commerce_architecture_v1/output/visual-polish-r1/before/products-new-variants/zh-TW-390x844.png)
- [Product edit en390 before](/Volumes/data/live_commerce_architecture_v1/output/visual-polish-r1/before/products-product/en-390x844.png)

**After screenshots: NOT_RUN.** Docker and TCP listen restrictions prevent the real seeded Go/Next browser stack. No synthetic after image or resized-before image was substituted. CI after artifacts must include ads and product new/edit at all3locales1440/390, section/end captures and native inner-scroll frames. Visual index labels supplemental viewport/frame evidence separately rather than pretending it is one ordinary full-page image. Independent source audits found no concrete new P1/P2; runtime wheel/geometry/focus/axes remain NOT_RUN.

## CI gates / integrator action

Run these modes serially per localPG policy or on independent GitHub runners; the author did not start futile Next builds before the denied fixture. Normal runtime exits are **NOT_RUN**, not0:

```sh
bash scripts/dev/test-local.sh --browser-meta-ads
bash scripts/dev/test-local.sh --browser-operations-ads
bash scripts/dev/test-local.sh --browser-product-editor
bash scripts/dev/test-local.sh --browser-catalog-core
bash scripts/dev/test-local.sh --browser-catalog-media
bash scripts/dev/test-local.sh --browser-product-media-v2
LC_SWEEP_PAGES='/ads,/products/new,/products/[product]' bash scripts/dev/test-local.sh --browser-visual-lint
```

The current PR planner also selects52 modes for these UI paths (`pr-mode-plan.json`); broader required CI selection is not reduced by this focused list. Re-run fullNode in a capable environment. The current browser assertions retain old locale/width coverage and explicitly add1440; visual-lint keeps default1586/390 and supplements affected pages1440. Source E3 is bounded to automated focused tests; full BROWSER/REAL_PG visual acceptance, after screenshots, providerSANDBOX/LIVE and deployment are NOT_RUN. Integrator commits/reviews/pushes; no author PR or push.

## Skipped / not reproduced

- Pill-versus-tab filter idiom: skipped by owner as a design choice; no filter redesign.
- A zh-TW-only product page height/unreachable-form bug: not reproduced. Source and all locale screenshots identify intentional pane scrolling plus insufficient capture. Device/native real-scroll confirmation remains NOT_RUN until CI.
- Other uncertain review impressions without a reproducer: no changes. Duplicate new/variants/edit truncation findings share the same capture root cause.

Modified source list and current hashes: `SOURCE.json` (12 source/test/script files) plus this DELIVERY.md. No Go, migration, OpenAPI, package lock or shared formatter implementation edit. All owned final wrappers exited; no CI/deploy/production action or background test was left running.
