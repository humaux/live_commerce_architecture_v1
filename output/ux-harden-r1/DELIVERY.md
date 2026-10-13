<!-- Purpose: record UX-HARDEN-R1 scope, provenance and SHA-bound CI-only red/green acceptance.
     Depends on: product-editor/storefront browser gates and GitHub Actions receipts.
     Used by: owner, integrator and independent PR reviewers. -->
# UX-HARDEN-R1 delivery

Status: the completed tests-first `a2dfa7e35` round has a green Foundation check and genuine browser RED for UXH06/07 in all three locales and all 33 missing persisted interactions. The recorder repair `dbbd0bdcd` and this revision's TabStrip geometry repair have passed independent E1 source review. New-head CI and final independent acceptance are pending; neither repair has CI green evidence yet. Earlier scoped green receipts remain historical.

- Branch: `unit/ux-harden-r1`; PR target: `r3/integration`.
- Base after `git fetch origin`: `0bfe36e53acb694f4697521202e24f0790638941`.
- Before the red-test commit, `git fetch origin` succeeded; `origin/unit/ux-harden-r1` did not yet exist (ref lookup exit 128), so there was no own remote branch to merge.
- Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-harden-r1`.
- Author: primary agent, UI implementation/integration role; actual model `gpt-6.1-sol`, reasoning `ultra`. Config top-level model/effort and own session turn context verify these values.
- Historical behavioral-test delegate: `/root/ux_browser_red`, `test_worker`, actual `gpt-6.1-sol`, reasoning `high`; unit base `0bfe36e5`, branch `unit/ux-harden-r1`, same root worktree `/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-harden-r1`. It wrote only `tests/admin/product-editor-layout.acceptance.ts`. This was exclusive file ownership in a shared worktree, **not isolated-worktree compliance**; the historical deviation is retained rather than relabeled. No recursive delegation.
- New isolated behavioral-test delegate: `/root/ux_ledger_isolated`, `test_worker`, actual `gpt-6.1-sol`, reasoning `high`; base `b2edee56c279d1da8a21377c8ee0ac2531d76832`, branch `unit/ux-harden-r1-ledger`, worktree `/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-harden-r1-ledger`. Its only write paths are `tests/admin/product-editor-layout.acceptance.ts` and `tests/admin/product-editor.acceptance.ts`; root does not write those files during its assignment. Its commits are reviewed and integrated into the parent branch; no recursive delegation, product writes, local tests or independent push.
- Read-only delegate: `/root/ux_shop_static`, `explorer`, actual `gpt-6.1-sol`, reasoning `medium`; same worktree, no write paths.
- Phase-2 isolated recorder delegate: `/root/ux_ledger_fix_isolated`, `test_worker`, actual `gpt-6.1-sol`, reasoning `high`; base `a2dfa7e355c3e645aa682640aeec0e2edce08449`, branch `unit/ux-harden-r1-ledger`, worktree `/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-harden-r1-ledger`. Only the two acceptance files assigned above were changed; commit `dbbd0bdcd509336da09d34a82d10795d7659c5dc` was independently inspected and integrated by fast-forward. Its own remote branch was absent after fetch (ref lookup 128). No local runtime checks, push or recursive delegation. This is a new actual delegate, not retrospective isolation of the historical shared-worktree writer.
- New CI-receipt reviewer: `/root/ux_red_receipt_audit`, `explorer`, actual `gpt-6.1-sol`, reasoning `medium`; base/head `a2dfa7e355c3e645aa682640aeec0e2edce08449`, branch `unit/ux-harden-r1`, root worktree above. Read-only paths are the new editor CI artifact, the two product-editor acceptance files and `packages/ui/src/Presentation.tsx`; no code write paths or runtime checks. E1 review confirms the intended RED and no invalid fixture preconditions; it does not accept the pending repair.
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

Exact `b2edee56c279d1da8a21377c8ee0ac2531d76832` outcomes:

| CI run | Result and portable receipt |
| --- | --- |
| [Foundation 38414578284](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38414578284), job `115355481247` | **SUCCESS**: the actual TabStrip opt-in SSR test and production toggle localized-label tests pass; check-gates reports 83 modes. [round1-fix-foundation.log](round1-fix-foundation.log). |
| [Gates 38414578750](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38414578750), editor job `115357440849`, artifact `11829846718` | **FAILURE**: English UXH05 still has `scrollLeft=0` and the selected SEO tab outside the frame after 901→390. zh-TW/zh-CN UXH05 pass; UXH01–04 observations pass in all three locales. [round1-fix-editor.log](round1-fix-editor.log). Eleven later editor tests are **NOT_RUN**; frozen CC12 passes separately. No other test mode failed in this completed round. |

Source diagnosis is an E1 hypothesis: inherited `white-space: nowrap` lets the English desktop label overflow the vertical strip, so raw horizontal overflow can remain true before the mobile transition. The owner's subsequent direct ruling includes the clipping-resize P2 in this batch, superseding the earlier transition-only limitation when actual horizontal geometry changes. The unrelated-draft/manual-scroll assertion remains unchanged. The batch also carries the persisted-interaction-ledger and historical delegate-worktree P1s. No local test, script, build or server was executed.

The isolated delegate's first tests-only commit is `3da7b35a57670907589f8b65a35c5f237b5acd5f`; its reviewed correction is `8797b2928677bae5faf57ff7edebbde0cc498d04`, integrated by fast-forward. Static review found that the proposed 600px starting width has spare room in Chinese, so UXH06 uses 500→390 and retains every overflow/fit/visibility assertion. It also guarantees route removal and viewport restoration when the held transport fails. The delegate fetched before each commit; its own remote branch was absent (ref lookup exit 128), so there was no delegate remote to merge. No local runtime checks were run.

The new regression assertions read the saved `product-editor-click-ledger.json` and require 33 real interaction records across the three locales, with explicit expected/actual/pass, viewport and measured metrics. They use a separate `product editor hardening` namespace; the canonical 267-case matrix and its validator remain unchanged. UXH06 selects SEO once at 500px and measures clipping after 390px. UXH07 holds an unchanged real BFF/Go/PG catalog response at an additional 220px viewport, then measures the same selected button as real status counts arrive. The a2 CI receipt below confirms both fixture preconditions and the intended RED. Recording and product repairs are in this batch after collection of the completed CI round; source-only review is E1, not runtime acceptance.

Portable exact-b2 receipts: `round1-fix-editor.log` SHA-256 `49c59d142ab3e5bca82cc382ce73d1a748d3a34cf948708ce5c9be255056cb81`; `round1-fix-foundation.log` SHA-256 `833bb424934b0e33ed97f385d6f0cbc74a420a722f21497f3cddcff3c5169686`. Both are committed under this unit's evidence directory; downloaded scratch artifacts and raw logs are excluded.

### Tests-first revision a2dfa7e35

[Foundation 38432944274](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38432944274), job `115419546421`, completed **SUCCESS** at `2026-10-13T16:49:20Z`. Its PR head is `a2dfa7e355c3e645aa682640aeec0e2edce08449`; the runner actually checked out the PR test merge `4dd9180c78c1ad5eb267889cbfee5283326e4199` against base `221539a7f6634824dbabb5e0f3681a270a3e7b8b`. [round2-test-foundation.log](round2-test-foundation.log) preserves that checkout and the successful node/architecture/gate-map excerpts (SHA-256 `b62b193864e9afa61b6d92626f2f1aa77a35a4d28c2093ac878ac0c5c90eb186`). The receipt is committed with this repair batch.

[Gates 38432944296](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38432944296), product-editor job `115419825933`, executed `2026-10-13T17:13:46Z`–`17:18:07Z` and failed on the same synthetic PR merge `4dd9180c78c1ad5eb267889cbfee5283326e4199`. Artifact `11833242913` contains the actual browser log and saved interaction ledger. [round2-red-editor.log](round2-red-editor.log) preserves the complete browser log at a stable path (SHA-256 `468e9f4e0a4f6e35fe983347d8c745c7221f6b203f39e792db8f8e62a105c7aa`).

- UXH06: all three locales start with an overflowing strip and fully visible selected SEO at 500px; after 390px, the selected tab remains clipped with its old scroll offset.
- UXH07: all three locales start with a visible selected archived tab; the unchanged real response adds counts to the same element, its measured width grows, and it becomes clipped.
- All 33 saved-ledger assertions fail because the corresponding records are absent. UXH05 also remains red in English. UXH01–04 observations remain green; eleven later editor tests are **NOT_RUN**, and frozen CC12 passes separately.

Artifact source hashes: browser `playwright.log` SHA-256 `be5166951773054121a057a6c27ba1c8a6b6e9aead09aded2f47b590a88327bd`; saved `product-editor-click-ledger.json` SHA-256 `85ab79426ee371087e638e0fbffff34c0c0bae728439dbbf7a451e7a26c213c5`. At the bounded `2026-10-13T17:38Z` read, Gates completed with 79 jobs complete: only product-editor and its derived required aggregate failed; all other test modes passed. The repair batch started after this completed-round collection; no duplicate gate was dispatched.

### Round-2 repair batch

The caller-owned recorder now wraps each native UXH interaction with expected/actual/pass, locale, actual viewport and measured metrics. Soft assertion errors determine FAIL from their per-action error delta; hard failures record FAIL and rethrow. All three layout callsites and real asynchronous count growth use the same caller ledger. The saved 33-row requirement, canonical 267-case matrix and original geometry assertions remain unchanged.

The root TabStrip repair compares actual horizontal frame/content widths, content-relative selected position and direct-child widths. It observes current children and removes detached observers, then rechecks selected visibility only when selection or this geometry changes. Manual scroll and shared viewport translation cancel out of the position sample; unrelated equal-geometry draft renders retain scrolling. Both observers disconnect. E1 source review caught a raw offsetLeft sample that lacked an offsetParent invariant; the repair replaces it with a strip-relative rectangle plus scrollLeft.

Read-only `/root/ux_red_receipt_audit` independently reviewed the final geometry and entire `dbbd0bdcd` recorder diff: **E1 source clear**, no concrete P0/P1/P2 or assertion weakening. This is not runtime acceptance. Its result is bound to these SHA-256 hashes:

| Source | SHA-256 |
| --- | --- |
| `packages/ui/src/Presentation.tsx` | `acca7718a3419ab246e699c6bf063001fc8c2e6a13b17e52fe53b2b581ca6bcd` |
| `tests/admin/product-editor-layout.acceptance.ts` | `228fde36ec93c260ba7e2991557fc6d4c3f4339ecd6751c4e64b4cc0fbef0ea6` |
| `tests/admin/product-editor.acceptance.ts` | `ad0d2f913c9c0f28f29fbcee1b973709726252827870ef4f27535a63c876159b` |

Before the root commit, fetch and own-remote merge succeeded; latest trunk `2d97c2027eefaa7df6553a832ffa38aa9fcd9d93` was merged without conflicts. It contains documentation only after the prior CI base. The next push runs automatic PR checks on this new head; no duplicate manual workflow is dispatched. All new-head runtime acceptance remains **NOT_RUN/pending** until the CI receipts exist. No local tests, builds or application runs were performed.

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

- Full editor acceptance and final PR acceptance remain pending after the exact-a2 geometry and missing-ledger RED; no release-readiness claim. Repairs require new exact-head CI evidence and independent acceptance.
- Local tests, typechecks, builds and servers are intentionally NOT_RUN under the CI-only directive.
- Production, physical-device rotation, vendor SANDBOX/LIVE requests and deployment are outside scope.
