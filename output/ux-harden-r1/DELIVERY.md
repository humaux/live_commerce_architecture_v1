<!-- Purpose: record UX-HARDEN-R1 scope, provenance and SHA-bound CI-only red/green acceptance.
     Depends on: product-editor/storefront browser gates and GitHub Actions receipts.
     Used by: owner, integrator and independent PR reviewers. -->
# UX-HARDEN-R1 delivery

Status: CI reproduced UXH01–04 on unchanged product source. The five scoped fixes are implemented; green CI and final independent acceptance are pending.

- Branch: `unit/ux-harden-r1`; PR target: `r3/integration`.
- Base after `git fetch origin`: `0bfe36e53acb694f4697521202e24f0790638941`.
- Before the red-test commit, `git fetch origin` succeeded; `origin/unit/ux-harden-r1` did not yet exist (ref lookup exit 128), so there was no own remote branch to merge.
- Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-harden-r1`.
- Author: primary agent, UI implementation/integration role; actual model `gpt-6.1-sol`, reasoning `ultra`. Config top-level model/effort and own session turn context verify these values.
- Behavioral-test delegate: `/root/ux_browser_red`, `test_worker`, actual `gpt-6.1-sol`, reasoning `high`; same assigned unit worktree, exclusive ownership of `tests/admin/product-editor-layout.acceptance.ts` while root reads product source. No recursive delegation.
- Read-only delegate: `/root/ux_shop_static`, `explorer`, actual `gpt-6.1-sol`, reasoning `medium`; same worktree, no write paths.

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
- Green: `--browser-product-editor`, default REAL-stack `--browser-storefront`; PR-required browser universe and foundation checks apply from the existing path planner.
- Foundation checks supply `scripts/dev/test-node.sh` and `scripts/dev/check-gates.sh`; typecheck/build results must be read from CI.

### Red observations and fixes

The red browser run reached the actual editor in zh-TW and reported all four failures before the existing navigation locator eventually timed out. The remaining editor tests were skipped by that failure; this is not an en/zh-CN red receipt.

| Case | CI red observation | Fix awaiting green CI |
| --- | --- | --- |
| UXH01 | nav, group and toggle all received `完成度` instead of their frozen distinct names | Distinct localized navigation, section group and checklist action names; heading retains the checklist name. |
| UXH02 | Section `seo` lost focus across both 900→901 and 901→900 | Always mount the same TabStrip and section controls; CSS changes their orientation. |
| UXH03 | Unrelated draft edit moved manual `scrollLeft` from 196 to 80 | Observe selected-tab attributes and stable node identity; unrelated renders leave manual scroll untouched. |
| UXH04 | Hint row reserved 26.796875px with overflow and 0px without it | Always mount the localized hint row and hide it with `visibility`/`aria-hidden`, retaining its height. |
| Existing storefront G4 | Timing follow-up; no separate deterministic red race is claimed | Wait for `networkidle` before measuring the 390px collection chips, matching the existing R5 check; original geometry assertions remain. |

The selected-section reveal assertion remains unchanged. A read-only independent source review by `/root/ux_shop_static` found no concrete P1/P2 in the five-file fix; that is source review only, not runtime acceptance. TabStrip hint reservation is scoped to TabStrip, leaving TableFrame behavior unchanged.

## NOT_RUN

- Green product-editor/storefront gates, PR-required checks and final independent acceptance are pending; no green or release-readiness claim.
- Local tests, typechecks, builds and servers are intentionally NOT_RUN under the CI-only directive.
- Production, physical-device rotation, vendor SANDBOX/LIVE requests and deployment are outside scope.
