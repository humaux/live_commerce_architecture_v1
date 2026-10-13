<!-- Purpose: record UX-HARDEN-R1 scope, provenance and SHA-bound CI-only red/green acceptance.
     Depends on: product-editor/storefront browser gates and GitHub Actions receipts.
     Used by: owner, integrator and independent PR reviewers. -->
# UX-HARDEN-R1 delivery

Status: PR #49 round 1 root fixes are ready for CI. Genuine CI red evidence covers UXH05 in all three locales and the default TabStrip empty-hint regression. Earlier scoped green receipts remain historical; final current-head gates and independent acceptance are pending.

- Branch: `unit/ux-harden-r1`; PR target: `r3/integration`.
- Base after `git fetch origin`: `0bfe36e53acb694f4697521202e24f0790638941`.
- Before the red-test commit, `git fetch origin` succeeded; `origin/unit/ux-harden-r1` did not yet exist (ref lookup exit 128), so there was no own remote branch to merge.
- Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-harden-r1`.
- Author: primary agent, UI implementation/integration role; actual model `gpt-6.1-sol`, reasoning `ultra`. Config top-level model/effort and own session turn context verify these values.
- Behavioral-test delegate: `/root/ux_browser_red`, `test_worker`, actual `gpt-6.1-sol`, reasoning `high`; same assigned unit worktree, exclusive ownership of `tests/admin/product-editor-layout.acceptance.ts` while root reads product source. No recursive delegation.
- Read-only delegate: `/root/ux_shop_static`, `explorer`, actual `gpt-6.1-sol`, reasoning `medium`; same worktree, no write paths.
- Author write paths: `apps/admin/components/ProductDocumentForm.tsx`, `ProductDocument.css`, `apps/admin/lib/product-editor-copy.ts` (round-1 owner authorization), shared `packages/ui/src/Presentation.tsx`/`Presentation.module.css` (TabStrip only), product-editor browser acceptance and product-document/shell-render tests, `tests/storefront/shop-real-gate.mjs`, and this unit's `output/ux-harden-r1/` evidence. No other module is edited.

## PR #49 round 1

Independent review by Claude API `claude-sonnet-5-5` of `b379b0a8` is **FAIL**. The owner requires all five findings: a visible copy-based toggle test, UXH05 desktop-selected SEO visible after 901→390 resize, `reserveHint` opt-in for the editor only, all navigation labels in the three-locale copy, and a copy-based feedback locator. Final independent acceptance is pending.

[Foundation run 38395845906](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38395845906), exact `b379b0a85bc632da056697f167c234a6f0aff953`, failed before Node units: the architecture gate reports `ProductDocumentForm.tsx: lines 801 > 0`. Red revision `d53311489750d1d5d995d0f539db7f097f717a1d` compacts an unchanged import declaration below the existing 800-line limit, without changing thresholds or runtime behavior. It also adds UXH05 and the actual TabStrip SSR opt-in test before the fixes.

| Exact d533 CI evidence | Result and portable receipt |
| --- | --- |
| [Foundation 38398316297](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38398316297), job `115299111934`, `scripts/dev/test-node.sh` | **RED**: actual SSR renders an empty hidden hint for default callers. [round1-red-hints.log](round1-red-hints.log). Architecture passes; the later stale-toggle test was **NOT_RUN** because the first Node batch stopped. |
| [Gates 38398316937](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38398316937), job `115304672802`, `--browser-product-editor`, artifact `11822806813` | **RED**: UXH05 desktop-selected SEO remains outside the frame after 901→390 in zh-TW, zh-CN and en. [round1-red-resize.log](round1-red-resize.log). The selected state remains intact, but `scrollLeft=0` and `fullyVisible=false`; UXH01–04 still succeed. Later editor cases are **NOT_RUN**; frozen CC12 passes separately. |

The round-1 batch makes hint reservation opt-in (`reserveHint=false` by default; only the editor opts in) and forces selected-tab reveal only on a non-overflowing→overflowing transition. Other resizes and unrelated mutations retain manual scrolling, and both observers disconnect on cleanup. Navigation/group/toggle copy is explicit in the closed three-locale `productEditorCopy`; the heading retains `progress`. Browser locators reference that copy and independently require four non-empty distinct names. The obsolete toggle source assertion is replaced with SSR of the actual production toggle JSX in each locale; it verifies visible text, rather than assuming the heading is its label. The feedback locator also uses the localized copy.

Read-only delegate `/root/ux_shop_static` (`explorer`, `gpt-6.1-sol`, `medium`) reviewed the eight-file round-1 source diff against `d5331148`: E1, no concrete P0/P1/P2 or weakened geometry assertions. The SSR test executes a production JSX fragment; complete visibility remains a browser obligation. This source review is not final runtime acceptance.

Fix CI outcomes are **NOT_RUN** until the new head has its own GitHub receipts. No local test, script, build or server was executed.

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

The selected-section reveal assertion remains unchanged. A read-only independent review by `/root/ux_shop_static` of the full `0bfe36e5..41cd06fd` unit found no concrete P0/P1/P2 or assertion weakening; the later external review nevertheless found the round-1 issues above. The earlier source review is not final acceptance. The hardening sequence runs once per page and locale; the existing reserved-layout assertions continue on each fixture. In the round-1 fix, only the product editor opts into hint reservation; default TabStrip consumers and TableFrame retain their prior layouts.

## NOT_RUN

- PR-required shared TabStrip consumer regressions, foundation checks and final PR acceptance are pending; no release-readiness claim.
- Local tests, typechecks, builds and servers are intentionally NOT_RUN under the CI-only directive.
- Production, physical-device rotation, vendor SANDBOX/LIVE requests and deployment are outside scope.
