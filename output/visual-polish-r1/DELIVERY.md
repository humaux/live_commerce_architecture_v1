<!-- Purpose: VISUAL-POLISH-R1 delivery with reproducible red/green gates, source hashes, screenshot provenance and remaining CI scope.
Depends on: integrator commit 582921b8, trunk merges 9d6b2ad6/3cfb1d22/b9f80810, shared formatter/TabStrip, visual capture/verdict helpers, production browser registry and synthetic PG fixtures.
Used by: integrator PR/K3 review and CI; local BROWSER+REAL_PG acceptance is distinct from provider or production acceptance. -->
# VISUAL-POLISH-R1 delivery

## PR #43 round 1 — 4250866065

READY: required supplemental context/setup failures now fail the owning G-UI9 verdict. Branch `unit/visual-polish-r1`, worktree `/Volumes/data/live_commerce_architecture_v1/.worktrees/visual-polish-r1`; tested baseline `b9f808102fa56d9b050aea30cdc5ca2ca5940d5f` (own remote merged first, then trunk `fef79203`). The two source hashes below bind this uncommitted test run to the final commit. Final commit/push/comment receipts are in `r43-4250866065/`; do not resolve review thread 4250866065.

The old supplemental `newContext()` ran outside the protected capture block. Its rejection escaped to the task pool, which only logged it; the successful primary shot satisfied primary-only coverage. The task now protects creation and capture together, records the 1440 failure reason, closes any allocated context and always folds a record into the primary result. Partial context initialization also closes before rethrowing. An absent capture record is an explicit failure. No product UI, primary matrix, lint threshold, Go or registry changes.

Red/green drives the **actual runner's setup, admin task and G-UI9 aggregation**, with synthetic browser/capture transport and real supplemental folding. Before the fix, injecting the required 1440 `newContext` rejection left the actual verdict at **0**, so the new regression failed (`node-red.log`, exit **1**, 10 pass / 2 fail). After the fix the injected verdict is **1**, with `supplemental 1440x900: context-error: injected supplemental newContext rejection`; the regression command passes. Setup rejection, absent capture and normal success/cleanup also pass. This is DB-free fault injection, separate from the full normal browser gate. The initial VM harness omitted `URL`; its exit 1 is preserved as `node-harness-setup.log` and is **not** the regression red evidence.

All logs/exit receipts below are under `output/visual-polish-r1/r43-4250866065/`:

| Command | Exit | Evidence |
| --- | --- | --- |
| `node --test tests/ui/visual-scroll-capture.test.mjs` before source fix | **1** | `node-red.log/.exit`: old actual G-UI9 verdict 0 differs from required 1 |
| `node --experimental-strip-types --test tests/ui/visual-scroll-capture.test.mjs tests/ui/visual-lint-lib.test.mjs tests/ui/sweep-shard-lib.test.mjs` | **0** | `node-green-final.log/.exit`, 40/40; explicit injected verdict/reason diagnostic |
| `bash scripts/dev/test-node.sh` | **0** | `test-node.log/.exit`; optional LiveKit-binary media test NOT_RUN (see below) |
| `bash scripts/dev/test-local.sh --browser-visual-lint` | **0** | `browser-visual-lint.log/.exit`; full unfiltered normal G-UI9 |
| `bash scripts/dev/check-gates.sh` | **0** | `check-gates.log/.exit`, 83 modes / every tracked test registered |
| `pnpm --filter @live-commerce/admin typecheck` | **0** | `typecheck.log/.exit` |
| `bash scripts/dev/test-local.sh --list`; `node --check` both changed MJS files; `git diff --check` | **0** | `mode-list.log`; direct checks |
| `node scripts/dev/pr-modes.mjs origin/r3/integration` | **0** | `pr-modes.json`: 53 selected entries, deploy false; broader CI remains integrator scope |

Current normal browser evidence: [`lint.json`](../playwright/run.rtxlAF0q/ui-visual-audit/20261012T175953Z/lint.json), [`index.json`](../playwright/run.rtxlAF0q/ui-visual-audit/20261012T175953Z/index.json), [`lint.md`](../playwright/run.rtxlAF0q/ui-visual-audit/20261012T175953Z/lint.md). **348/348** primary shots, **NOT_RUN 0**, **blocking 0**; all three locales at 1586/390, **12 required 1440 supplements** with files/SHA256 independently checked, 540 PNG files under `shots/`. R5 21 / R7 119 remain warnings; no threshold changes. `visual-verification.json` preserves these checks. The reused Go sweep harness passing here proves G-UI9, not click-journey acceptance.

Frozen source SHA256 (unchanged during the gate):

| File | SHA256 |
| --- | --- |
| `tests/ui/visual-audit.mjs` | `321065f166a2761467c8712d1f9d8879ed0d3cc515094f14828816e91df143f9` |
| `tests/ui/visual-scroll-capture.test.mjs` | `1cf6d1a8bcb8481c65d19cd69e7bc1e25701067d4041e86f29b9a0a8cc8a58a5` |

Primary: Codex author (deployed model/effort not exposed), owns the two MJS files and this report. Independent read-only explorer `r43_supplemental_review`: gpt-6.1-sol / medium, base `b9f80810`, no recursive delegation, writes or runtime; reviewed the same hashes and found no blocking gaps (E1 source review). Humaux fix records the root and evidence. Code indexing accepted both files but reported 0 indexed files/entities; exact `tests/ui/visual-audit.mjs::main` link was not found (**NOT_LINKED**). Source pointers and stored rationale remain available; no graph success is claimed.

Evidence: **E3 local BROWSER + REAL_PG with MOCK external services**, plus DB-free injected actual-verdict regression. NOT_RUN: broader planner-selected modes (including full foundation / click-sweep / product-editor on this gate-only revision), independent runtime rerun and provider/production acceptance. Full Node explicitly skips `tests/media/r04-input-runner.test.mjs` because `COMMERCE_R04_LIVEKIT_BINARY` is unset. No product Go touched; separate focused Go gate is not applicable. Existing unrelated P2 FOLLOWUPS remain deferred. Push only the authorized unit branch, never force-push, and leave the review thread unresolved.

## Historical savebar/focus delivery — 91b9a6a8

The local savebar/focus repair and all six prior gates passed on merged baseline `3cfb1d22`. Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/visual-polish-r1`; branch: `unit/visual-polish-r1`. The integrator committed the original four fixes as `582921b8` and merged trunk as `9d6b2ad6`. Pre-commit fetch then received trunk `1200178c`; merge `3cfb1d22` preserves all six source hashes. The prior merged-baseline command receipts are below. The owner authorized local product-editor/visual-lint runs, author commit and branch push. The earlier sandbox-only handoff is historical.

Primary: Codex UI worker (deployed model name/effort not exposed). Final independent source review: explorer `visual_focus_diagnosis`, gpt-6.1-sol, medium effort, read-only, same base; no recursive delegation or concurrent code writers. This patch owns the four product components, two product tests and this report. No Go, migration, OpenAPI, lockfile, shared formatter implementation or gate threshold change.

## Result and roots

- Ads still uses shared `money()` through `formatMinor`, so zh-TW TWD shows `NT$`. The original literal `$0 !== NT$0` regression went red before that fix. The final visual corpus includes the actual zh-TW result page with `NT$0`; the integrator also reports meta-ads exit 0 on the supplied base.
- Mobile section navigation reuses the existing TabStrip/localized scroll hint. The original localized save-status phrase occupies its own row.
- **First edit layout shift:** hiding the empty status row changed footer height when dirty copy appeared. The mobile row now reserves the same line while empty. The 375px browser test asserts identical footer height and y before/after the first edit, retains the original focused-field-bottom assertion, and passes.
- **Readiness focus:** the checklist could be clicked while its destination remained disabled during reference/receipt loading. Both checklist instances and section navigation now inherit the actual field lock.
- **Pane reveal:** focus stays inside `.pe-fields`. Directional rounding and a 5px inset retain the existing 2px upload focus ring plus 3px offset inside the clip. The callback regression models fractional scroll rounding; those synthetic DOMRects are boundary-test data, not measurements from a browser trace.
- **Populated-edit focus:** the first media control is a permanently disabled move-back button. The hook selects the first enabled, rendered control rather than leaving focus on the nav. The new callback regression covers enabled/disabled/hidden first controls.
- The ~929px document screenshot was a viewport-bound editor with a separately scrolling field pane. The existing capture repair keeps document screenshots and adds native wheel frames with contiguous coverage/hash receipts; supplemental 1440px shots use the same lint pipeline. Product acceptance separately reaches the final search controls in new/edit views. The form is 798 lines; no architecture allowance was widened.

## Initial repaired baseline 9d6b2ad6: commands and exits

CWD for the following commands is the assigned worktree. Current raw logs and `.exit` files: `output/visual-polish-r1/savebar-r2/`. `final-source.json` binds the six source files to both final browser runs. No source changed during either final browser gate.

| Command/check | Exit | Evidence |
|---|---:|---|
| `node --test --experimental-strip-types tests/admin/product-document.test.ts tests/admin/shell-registry.test.ts` | 1 → 0 | `node-red/green`: reserved-row and real React SSR lock regressions; final focused result 24/24 |
| Actual focus callback, fractional boundary case | 1 → 0 | `focus-rounding-red/green`; no copied production scroll formula |
| Actual focus callback, populated edit | 1 → 0 | `edit-focus-red/green`; final product + shell 24/24 |
| `bash scripts/dev/test-local.sh --browser-product-editor`, initial own rerun | 1 | `browser-product-editor.log`, run.feuvhMAu: readiness focus boundary; frozen CC12 passes |
| Same mode after rounding fix | 1 | `browser-product-editor-green.log`, run.UNO7x4km: original readiness and 375px footer assertions pass; saved media navigation finds disabled first control |
| Same mode, final frozen source | **0** | `browser-product-editor-final.log`, run.NPtQ2K6k: PE12–17 + mandatory frozen CC12 pass, real Go/isolated PG/production Next |
| `bash scripts/dev/test-local.sh --browser-visual-lint` (full, unfiltered) | **0** | `browser-visual-lint.log`, run.PsUFKOdb: 348/348 primary shots; NOT_RUN 0; blocking 0 |
| `bash scripts/dev/test-node.sh`, initial full run | 0 | `test-node.log`; before the final focus refinements |
| Same command, concurrent intermediate rerun | 1 | `test-node-final.log`: unrelated inbox-review.test.ts never-settles subprocess hit its 10s timeout; test/timeout unchanged |
| Same command, final serial rerun | **0** | `test-node-serial.log`; all script batches completed after browser runs |
| `pnpm --filter @live-commerce/admin typecheck` | **0** | `typecheck-ready.log` on final source |
| Strict browser-spec typecheck below | **0** | `spec-typecheck-ready.log` |
| `bash scripts/dev/check-gates.sh` | **0** | `check-gates.log` and final `check-gates-ready.log`: all 83 modes documented, every tracked test run, headers/architecture/backend coverage pass; final contract-drift step 338.629s |
| `node scripts/dev/pr-modes.mjs origin/r3/integration` | 0 | `pr-modes.json`: 53 selected entries, including foundation-shards and 52 browser modes |
| `git fetch origin`; `git merge --no-edit origin/r3/integration` | 0; 0 | Before final checks; merge already up to date. Own remote branch absent (show-ref exit 128), so no nonexistent own-branch merge |

Exact strict spec command:

```sh
pnpm exec tsc --noEmit --strict --skipLibCheck --target ES2023 --module esnext --moduleResolution bundler --esModuleInterop --allowImportingTsExtensions --typeRoots apps/admin/node_modules/@types --types node tests/admin/product-editor.acceptance.ts tests/admin/product-editor-layout.acceptance.ts tests/admin/ads.spec.ts
```

The earlier original-four-fix Node/capture red/green receipts, 72 focused tests, and prior CI before-manifest remain under `/Volumes/data/live_commerce_architecture_v1/output/visual-polish-r1/`. They are historical evidence, not the final source fingerprint.

## Screenshots and coverage

Product gate: `output/playwright/run.7mQWsFR1/catalog-core/20261012T155800.411384000/product-ui-v2/`: 179 screenshots with `screenshots.json` hashes and `click-ledger.json`. New and saved edit views have all three locales at 1440/390, including section/end captures; old widths are retained.

Visual gate: `output/playwright/run.5B8hddOV/ui-visual-audit/20261012T160338Z/`: `lint.json`, `lint.md`, `index.json`, shots/crops. Primary corpus: 348/348 at 1586/390 across zh-TW, zh-CN, en; 540 PNGs in total including 12 affected-page 1440px shots and native inner-pane frames. No missing shot, load failure or state failure. R5 21 and R7 119 instances are warnings; blocking total is 0. Thresholds and assertions are unchanged.

The current visual index records merged baseline `3cfb1d22` because the runs used frozen working changes before author commit; the source hashes below bind the tested changes. Original before corpus: 24 unmodified synthetic CI38207265014 shots at 1586/390; no original 1440 before image exists.

Representative before/after:

- [375px failing SEO before](/Volumes/data/live_commerce_architecture_v1/.worktrees/visual-polish-r1/output/playwright/run.9rtUABaI/catalog-core/20261012T142847.895897000/results/catalog-core-PE12-17-docum-0e9d3-st-actions-and-click-ledger/test-failed-1.png) → [375px SEO above stable footer after](/Volumes/data/live_commerce_architecture_v1/.worktrees/visual-polish-r1/output/playwright/run.7mQWsFR1/catalog-core/20261012T155800.411384000/product-ui-v2/editor-bottom-en-375.png).
- [zh-TW product before](/Volumes/data/live_commerce_architecture_v1/output/visual-polish-r1/before/products-new/zh-TW-390x844.png) → [hint/full status after](/Volumes/data/live_commerce_architecture_v1/.worktrees/visual-polish-r1/output/playwright/run.7mQWsFR1/catalog-core/20261012T155800.411384000/product-ui-v2/editor-zh-TW-390-seo.png) and [saved edit scroll end](/Volumes/data/live_commerce_architecture_v1/.worktrees/visual-polish-r1/output/playwright/run.7mQWsFR1/catalog-core/20261012T155800.411384000/product-ui-v2/edit-zh-TW-390-scroll-end.png).
- [zh-TW ads before](/Volumes/data/live_commerce_architecture_v1/output/visual-polish-r1/before/ads/zh-TW-390x844.png) → [NT$ metrics after](/Volumes/data/live_commerce_architecture_v1/.worktrees/visual-polish-r1/output/playwright/run.5B8hddOV/ui-visual-audit/20261012T160338Z/shots/admin/ads/zh-TW-390x844.png).

Root inspected the actual final 375px SEO, zh-TW status/hint, native-pane and ads images. Independent source review found no concrete new P0/P1/P2; runtime acceptance belongs to the above gates, not that source review.

## Tested source SHA256

| File | SHA256 |
|---|---|
| apps/admin/components/ProductDocument.css | `77f9b59d608f99a2676a26092c9e47129bcaa1d84947cf57497bfae8ca7cb499` |
| apps/admin/components/ProductReadiness.tsx | `c65cfe792f74373f81ffc0039ce81b61723c6eb42b76975b327c1225c94a64ae` |
| apps/admin/components/ProductDocumentForm.tsx | `bc00d59041281de359593f2efb4591e91d42bf7d0b1c91650d51793b542112b2` |
| apps/admin/components/useProductEditorLayout.ts | `374395ff2b0d7e53ecf65ca9a7558e9236b25c8ed934ea178cbf4c00c189615b` |
| tests/admin/product-document.test.ts | `906161daed466ab751cd991bbe5ca1b4fc9785c77123e31c12bff5497a3413dc` |
| tests/admin/product-editor.acceptance.ts | `bec9dbd63fe55c334eddca5e59821fe46f7ea0f8fa44d69c63b351cb04641d7a` |

## CI gates, NOT_RUN and skipped scope

Completed locally: `--browser-product-editor`, full `--browser-visual-lint`, test-node, admin/spec typechecks and check-gates. The integrator's supplied-base meta-ads exit 0 is attributed to the integrator; the author did not rerun that mode after this product-only patch.

The planner's remaining modes, including meta-ads, operations-ads, catalog-core standalone, catalog-media, product-media-v2, click-sweep journeys and full foundation shards, are **NOT_RUN locally on final source** and remain CI scope. A visual-audit handoff through the sweep harness is not a click-journey PASS. Browser fixtures use synthetic data and MOCK identity/payments/carrier/Meta; provider SANDBOX/LIVE, production and deployment are NOT_RUN.

Skipped: pill-versus-tab idiom (owner's design choice), uncertain impressions without a reproducer, and a separate zh-TW-only unreachable-page-height bug (not reproduced: pane scrolling now has real reachability/coverage evidence). No filter redesign, lint threshold adjustment, PR creation or deployment.

Pre-commit fetch exited 0. The own remote branch is still absent, so there is no own-branch merge to perform. The fresh trunk merge exited 0 and created `3cfb1d22` (incoming `1200178c`, comment-label print UI); Node registry and comment-stream pages changed. All current-baseline gates below finished before author commit/push; previous-baseline command results above are retained. The author stages only the six source files and this report. Integrator opens the PR; no rebase/force-push. Raw evidence is kept locally.

## Current merged baseline 3cfb1d22

All final post-merge checks passed with unchanged six-file SHA256 fingerprints. Exact current receipts:

| Command | Exit | Current evidence |
|---|---:|---|
| `bash scripts/dev/test-node.sh` | **0** | `postmerge-test-node.log/.exit`; includes trunk's newly registered comment-label tests |
| `bash scripts/dev/test-local.sh --browser-product-editor` | **0** | `postmerge-browser-product-editor.log/.exit`; run.7mQWsFR1, PE12–17 + frozen CC12 |
| `bash scripts/dev/test-local.sh --browser-visual-lint` | **0** | `postmerge-browser-visual-lint.log/.exit`; run.5B8hddOV, full unfiltered 348/348, NOT_RUN 0, blocking 0 |
| `bash scripts/dev/check-gates.sh` | **0** | `postmerge-check-gates.log/.exit`; registry/headers/architecture/backend coverage |
| `pnpm --filter @live-commerce/admin typecheck` | **0** | `postmerge-typecheck.log/.exit` |
| Strict spec command printed above | **0** | `postmerge-spec-typecheck.log/.exit` |

The two browser/PG modes ran serially. Non-PG check-gates ran beside the final visual capture after the full Node run finished. Source stayed frozen throughout. `postmerge-source.json` holds run/session receipts and the six hashes. Only the fixed, freshly merged baseline was revalidated; later unrelated trunk changes are left to CI. The author fetches and merges any own remote branch again before the final commit, without force-push.

Evidence level: **E3, local BROWSER+REAL_PG with MOCK external services**. PR-wide CI, independent PR review and provider/production acceptance remain outstanding. No PR creation or deployment by the author.
