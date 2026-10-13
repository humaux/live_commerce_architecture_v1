<!-- Purpose: record UX-HARDEN-R1 scope, provenance and SHA-bound CI-only red/green acceptance.
     Depends on: product-editor/storefront browser gates and GitHub Actions receipts.
     Used by: owner, integrator and independent PR reviewers. -->
# UX-HARDEN-R1 delivery

Status: scoped GitHub CI is green: UXH01–04, complete PE12–17/review/feedback acceptance, frozen CC12, and the REAL-stack storefront gate. PR-required shared-consumer regressions and final PR acceptance remain pending.

- Branch: `unit/ux-harden-r1`; PR target: `r3/integration`.
- Base after `git fetch origin`: `0bfe36e53acb694f4697521202e24f0790638941`.
- Before the red-test commit, `git fetch origin` succeeded; `origin/unit/ux-harden-r1` did not yet exist (ref lookup exit 128), so there was no own remote branch to merge.
- Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-harden-r1`.
- Author: primary agent, UI implementation/integration role; actual model `gpt-6.1-sol`, reasoning `ultra`. Config top-level model/effort and own session turn context verify these values.
- Behavioral-test delegate: `/root/ux_browser_red`, `test_worker`, actual `gpt-6.1-sol`, reasoning `high`; same assigned unit worktree, exclusive ownership of `tests/admin/product-editor-layout.acceptance.ts` while root reads product source. No recursive delegation.
- Read-only delegate: `/root/ux_shop_static`, `explorer`, actual `gpt-6.1-sol`, reasoning `medium`; same worktree, no write paths.
- Author write paths: `apps/admin/components/ProductDocumentForm.tsx`, `ProductDocument.css`, shared `packages/ui/src/Presentation.tsx`/`Presentation.module.css` (TabStrip only), the three touched product-editor browser acceptance files, `tests/storefront/shop-real-gate.mjs`, and this unit's `output/ux-harden-r1/` evidence. No other module is edited.

## Scope and acceptance

1. Distinct localized names for section navigation, section group, readiness toggle and heading.
2. Focus remains on the same section button when resizing across 900px in either direction.
3. Manual section-strip scrolling survives unrelated draft renders; changing active section reveals the selected button.
4. Overflow hint visibility changes retain the same reserved height.
5. The real storefront chip check waits for layout/loading to settle, retaining its no-horizontal-scroll and page-overflow assertions.

Behavior is measured in the existing product-editor browser acceptance; no source-regex proof. Ads money formatting is outside this unit. No API, Go, migration or transaction changes.

## CI gates

All executions are GitHub-only under the owner's 2026-10-13 rule. No local tests, typechecks, builds, dev servers or database containers are run.

- Red: [Gates run 38386230504](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38386230504), head `3feaf4967c7bcdce8ead732cef5b32083b9898ec`, completed **FAILURE**. The `--browser-product-editor` Gate step failed; its browser log is preserved in [red-product-editor.log](red-product-editor.log), copied from artifact `gate-browser-product-editor` (artifact ID `11807208320`).
- Green receipts are listed below. PR-required browser universe and foundation checks apply from the existing path planner; they are still pending.
- Foundation checks supply `scripts/dev/test-node.sh` and `scripts/dev/check-gates.sh`; typecheck/build results must be read from CI.

### First fix run

[Gates run 38390368168](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38390368168), head `bc9e9310c33ac4d0f1b0cf831ff5a49eaacdaf19`:

- `--browser-storefront`: Gate **SUCCESS**, default REAL application/PG stack (no vendor LIVE claim). [fix1-storefront.log](fix1-storefront.log) preserves SFR01–09 success from artifact `11808812314`.
- `--browser-product-editor`: Gate **FAILURE**. [fix1-product-editor.log](fix1-product-editor.log), copied from artifact `11808878094`, records successful UXH01–04 measurements in zh-TW, zh-CN and en. Both resize directions retain focus; manual scroll stays 196/196, 182/182 and 501/501 respectively; the hint row remains 26.796875px at all three measured widths.
- The editor suite reports 10 passed, 1 failed and 1 not run. The failure is `product-feedback.acceptance.ts:62`, which still locates navigation by the former shared name `Readiness`. Its independent navigation expectation now uses `Product editor navigation`; both Publish clicks, reveal/focus checks and mutation fencing remain unchanged. This run is not a whole-editor PASS.

### Complete scoped green

[Gates run 38393003430](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38393003430), exact head `41cd06fd01177447fe1a137529930b18d2408324`, completed **SUCCESS**, including its required aggregate. Artifact `11809409641` provides these portable copies:

- [green-product-editor.log](green-product-editor.log): **12 passed**, including both repeated-validation widths. UXH01–04 succeed in zh-TW, zh-CN and en with the original assertions.
- [green-catalog-frozen.log](green-catalog-frozen.log): mandatory frozen CC12 **2 passed**.
- [green-product-editor-gate.log](green-product-editor-gate.log): `TestBrowserCatalogCore`, `PE12-17-TWD` and `CC12-frozen` PASS; production admin Next build completes its TypeScript validation. Identity is MOCK; Go/PG are real isolated test dependencies. No provider or deployment acceptance is claimed.

The green head changes only the obsolete feedback locator and evidence after `bc9e9310`; its storefront/product/shared-TabStrip code is unchanged from the SFR01–09 green receipt in run `38390368168`. [tested-source-blobs.txt](tested-source-blobs.txt) records the exact Git blob IDs of all eight scoped product/test source files at the green head. Later evidence-only commits do not change those source hashes; required PR checks must still bind the actual PR head.

| GitHub execution | Run | Outcome |
| --- | --- | --- |
| `bash scripts/dev/test-local.sh --browser-product-editor` on unchanged product + new assertions | `38386230504` | RED: UXH01–04 reproduced |
| `bash scripts/dev/test-local.sh --browser-product-editor` after product fixes | `38390368168` | RED: obsolete feedback navigation locator; UXH01–04 all locales succeed |
| `bash scripts/dev/test-local.sh --browser-storefront` on product fixes | `38390368168` | SUCCESS: default REAL-stack SFR01–09 |
| `bash scripts/dev/test-local.sh --browser-product-editor` after locator correction | `38393003430` | SUCCESS: 12 editor tests + 2 frozen CC12 |

These commands ran on GitHub runners, not on the host. The artifact logs above preserve their actual test results; no local exit code is used as acceptance evidence.

### Red observations and fixes

The red browser run reached the actual editor in zh-TW and reported all four failures before the existing navigation locator eventually timed out. The remaining editor tests were skipped by that failure; this is not an en/zh-CN red receipt.

| Case | CI red observation | Fix verified by scoped green CI |
| --- | --- | --- |
| UXH01 | nav, group and toggle all received `完成度` instead of their frozen distinct names | Distinct localized navigation, section group and checklist action names; heading retains the checklist name. |
| UXH02 | Section `seo` lost focus across both 900→901 and 901→900 | Always mount the same TabStrip and section controls; CSS changes their orientation. |
| UXH03 | Unrelated draft edit moved manual `scrollLeft` from 196 to 80 | Observe selected-tab attributes and stable node identity; unrelated renders leave manual scroll untouched. |
| UXH04 | Hint row reserved 26.796875px with overflow and 0px without it | Always mount the localized hint row and hide it with `visibility`/`aria-hidden`, retaining its height. |
| Existing storefront G4 | Timing follow-up; no separate deterministic red race is claimed | Wait for `networkidle` before measuring the 390px collection chips, matching the existing R5 check; original geometry assertions remain. |

The selected-section reveal assertion remains unchanged. A read-only independent review by `/root/ux_shop_static` of the full `0bfe36e5..41cd06fd` unit found no concrete P0/P1/P2 or assertion weakening. This is independent source review, not production verification or a substitute for required PR checks. The hardening sequence runs once per page and locale; the existing reserved-layout assertions continue on each fixture. TabStrip hint reservation is scoped to TabStrip, leaving TableFrame behavior unchanged.

## NOT_RUN

- PR-required shared TabStrip consumer regressions, foundation checks and final PR acceptance are pending; no release-readiness claim.
- Local tests, typechecks, builds and servers are intentionally NOT_RUN under the CI-only directive.
- Production, physical-device rotation, vendor SANDBOX/LIVE requests and deployment are outside scope.
