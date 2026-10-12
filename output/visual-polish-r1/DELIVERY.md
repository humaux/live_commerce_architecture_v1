<!-- Purpose: portable VISUAL-POLISH-R1 delivery with committed runtime receipts, manifests, retained screenshots and honest omissions.
Depends on: source commits 630e1c86/91b9a6a8, synthetic browser/PG fixtures, evidence/manifest.json and verify-evidence.mjs.
Used by: integrator and independent PR43 review; local BROWSER/REAL_PG evidence remains separate from provider/production acceptance. -->
# VISUAL-POLISH-R1 delivery

## PR43 evidence repair — 4251129163

Branch: `unit/visual-polish-r1`. This batch changes evidence and documentation only. Tested runtime source remains commit `630e1c867e263c56b03f37ff1def3d918921d96c`; the [portable manifest](evidence/manifest.json) binds eight current source files by SHA256 to the earlier gates. No browser PASS is inferred from packaging. Integrator opens/reviews the PR; authorized author branch push uses no force-push. Leave both review threads unresolved.

All evidence linked here lives beneath this directory and is committed. Raw originals were preserved. Text copies normalize repository/worktree machine prefixes to `.` and ephemeral directories to `<TMP>`; trailing whitespace and extra EOF blank lines are removed; the manifest records original logical paths, raw SHA256 and published-view SHA256. Screenshots are copied byte-for-byte. Fixtures are synthetic with MOCK identity/payments/carrier/Meta. No traces, cookies, HTML reports, login material or full GitHub API user profiles are archived.

Retention is explicit: `COMMITTED` means reviewers can open and hash the PNG; `HASH_ONLY` means its original file hash, byte count and geometry are retained, while independent repository-only pixel inspection of that omitted binary is **NOT_RUN**. This is a selected image corpus, not a claim that every screenshot binary is published. All original visual screenshot and warning-crop hashes were checked against local bytes during packaging.

| Run / evidence | Measured result | Published retention |
| --- | --- | --- |
| Latest `run.rtxlAF0q`: [index](evidence/supplemental-630e1c86/visual/index.json), [lint JSON](evidence/supplemental-630e1c86/visual/lint.json), [summary](evidence/supplemental-630e1c86/visual/lint.md), [counts](evidence/supplemental-630e1c86/visual/counts.json), [image manifest](evidence/supplemental-630e1c86/visual/screenshots.json) | **348/348 primary, NOT_RUN 0, blocking 0**; 12 required 1440px supplements; 540 shot PNG hashes | **54 PNG committed**: affected primary pages in all 3 locales at 1586/390, all 12 supplements at viewport 1440×900, and 18 terminal native-pane frames. 486 remaining shot PNGs hash-only. |
| Historical `run.5B8hddOV`: [index](evidence/savebar-91b9a6a8/visual/index.json), [lint JSON](evidence/savebar-91b9a6a8/visual/lint.json), [summary](evidence/savebar-91b9a6a8/visual/lint.md), [counts](evidence/savebar-91b9a6a8/visual/counts.json), [image manifest](evidence/savebar-91b9a6a8/visual/screenshots.json) | **348/348 primary, NOT_RUN 0, blocking 0**; 12 supplements; 540 shot PNG hashes | **17 PNG committed**, including all 12 supplements and zh-TW mobile examples; 523 shot PNGs hash-only. |
| Historical product-editor `run.7mQWsFR1`: [screenshots](evidence/savebar-91b9a6a8/product/screenshots.json), [click ledger](evidence/savebar-91b9a6a8/product/click-ledger.json), [write metadata](evidence/savebar-91b9a6a8/product/write-metadata.json) | 179 screenshot hashes, real PE12–17 and frozen CC12 PASS | **14 product PNG committed**, all 3 locales at 1440/390 plus 375px footer case; 165 PNGs hash-only. |
| Original CI38207265014 [before manifest](evidence/original/before-manifest.json) | 24 historical before hashes at 1586/390 | 2 CI before PNG committed; 22 hash-only. No 1440 before image exists (**NOT_RUN**). One additional original 375px failing screenshot is committed. |

Both full visual runs retain R5 **21** and R7 **119** warnings; blocking total is 0. Thresholds and primary matrix are unchanged. Their index `commit` fields preserve the actual pre-commit merged baselines (`b9f808102f` / `3cfb1d2250`); they are not rewritten to pretend the runtime used a later commit. Source SHA256 binds the frozen working changes to the committed implementation. A 1440×900 viewport can produce a taller full-page PNG.

Repository-only verification: `node output/visual-polish-r1/verify-evidence.mjs --tracked`. It checks every retained file's bytes/hash, eight source hashes, raw/view provenance consistency, unique retained image membership, both 348-entry manifests, all 540 shot hashes per run, 12 supplements, retained supplemental bytes, complete native-pane ranges, Git-index availability and all DELIVERY links. The [verification receipts](verification-green.log) and [exit](verification-green.exit) record this batch. Missing-manifest and missing-required-image faults must fail closed; receipts are listed below.

## Supplemental failure root — 4250866065

`newContext()` for required 1440px evidence used to sit outside the capture catch. A rejection escaped to the pool, leaving a successful primary shot and a G-UI9 exit 0. Creation/setup/capture are now protected together, an explicit supplemental failure is always folded into the primary verdict, and allocated contexts are closed. Missing capture records also fail. No product UI, Go, registry or threshold changes were made in this repair.

The actual runner setup/admin task/aggregation is exercised with synthetic browser transport and real folding. Old injected `newContext` rejection produced verdict 0, so the regression command failed. Fixed injected verdict is **1**, explicitly `supplemental 1440x900: context-error: injected supplemental newContext rejection`. The regression command itself passes. Fault injection is DB-free and separate from the full normal browser gate.

| Command | Actual exit | Committed evidence |
| --- | ---: | --- |
| `node --test tests/ui/visual-scroll-capture.test.mjs`, before source fix | **1** | [red log](evidence/supplemental-630e1c86/receipts/node-red.log), [exit](evidence/supplemental-630e1c86/receipts/node-red.exit): old actual verdict 0 differs from required 1 |
| `node --experimental-strip-types --test tests/ui/visual-scroll-capture.test.mjs tests/ui/visual-lint-lib.test.mjs tests/ui/sweep-shard-lib.test.mjs` | **0** | [40/40 green log](evidence/supplemental-630e1c86/receipts/node-green-final.log), [exit](evidence/supplemental-630e1c86/receipts/node-green-final.exit), explicit injected exit 1 reason |
| `bash scripts/dev/test-node.sh` | **0** | [log](evidence/supplemental-630e1c86/receipts/test-node.log), [exit](evidence/supplemental-630e1c86/receipts/test-node.exit); optional LiveKit test NOT_RUN |
| `bash scripts/dev/test-local.sh --browser-visual-lint` | **0** | [parent log](evidence/supplemental-630e1c86/receipts/browser-visual-lint.log), [exit](evidence/supplemental-630e1c86/receipts/browser-visual-lint.exit), [actual capture-runner log](evidence/supplemental-630e1c86/receipts/visual-runner.log) |
| `bash scripts/dev/check-gates.sh` | **0** | [log](evidence/supplemental-630e1c86/receipts/check-gates.log), [exit](evidence/supplemental-630e1c86/receipts/check-gates.exit): 83 modes |
| `pnpm --filter @live-commerce/admin typecheck` | **0** | [log](evidence/supplemental-630e1c86/receipts/typecheck.log), [exit](evidence/supplemental-630e1c86/receipts/typecheck.exit) |

The [source receipt](evidence/supplemental-630e1c86/receipts/source.json), [coverage verification](evidence/supplemental-630e1c86/receipts/visual-verification.json), [mode plan](evidence/supplemental-630e1c86/receipts/pr-modes.json) and [publication receipt](evidence/supplemental-630e1c86/receipts/publication.json) are retained. Initial VM harness setup failure (missing `URL`) is excluded from the regression-red claim and is not presented as acceptance evidence.

## Historical savebar/focus repair — 91b9a6a8

The mobile dirty-status row reserves space while empty, so the intact localized phrase has its own line without first-edit footer growth. Readiness/navigation obey the actual field lock; focused controls are revealed inside `.pe-fields` using directional rounding and a 5px focus-ring inset. Populated edit navigation skips disabled/hidden first controls. The ~929px document capture used a separate scrolling editor pane; native wheel-frame coverage and real end-of-form navigation verify reachability. Ads money uses shared `money()` through `formatMinor`; mobile tabs reuse the localized TabStrip scroll hint.

Historical red/green logs are retained: [reserved-row/readiness red](evidence/savebar-91b9a6a8/receipts/node-red.log) / [green](evidence/savebar-91b9a6a8/receipts/node-green.log), [fractional-focus red](evidence/savebar-91b9a6a8/receipts/focus-rounding-red.log) / [green](evidence/savebar-91b9a6a8/receipts/focus-rounding-green.log), [populated-edit red](evidence/savebar-91b9a6a8/receipts/edit-focus-red.log) / [green](evidence/savebar-91b9a6a8/receipts/edit-focus-green.log). Original money [red](evidence/original/receipts/ads-money-red.log) / [green](evidence/original/receipts/ads-money-green.log) / [receipt](evidence/original/receipts/ads-money-receipt.json), capture [receipt](evidence/original/receipts/capture-receipt.json), and [72 focused tests](evidence/original/receipts/focused-node.log) are historical source-level evidence, not fresh runtime evidence for this batch.

The final merged-baseline `3cfb1d22` gates and six hashes are in the [source/session receipt](evidence/savebar-91b9a6a8/receipts/postmerge-source.json). Exact commands/exit files:

| Command | Exit | Evidence log and matching `.exit` |
| --- | ---: | --- |
| `bash scripts/dev/test-node.sh` | **0** | [postmerge-test-node.log](evidence/savebar-91b9a6a8/receipts/postmerge-test-node.log) |
| `bash scripts/dev/test-local.sh --browser-product-editor` | **0** | [postmerge-browser-product-editor.log](evidence/savebar-91b9a6a8/receipts/postmerge-browser-product-editor.log): PE12–17 + mandatory frozen CC12 |
| `bash scripts/dev/test-local.sh --browser-visual-lint` | **0** | [postmerge-browser-visual-lint.log](evidence/savebar-91b9a6a8/receipts/postmerge-browser-visual-lint.log), [capture-runner.log](evidence/savebar-91b9a6a8/receipts/visual-runner.log) |
| `bash scripts/dev/check-gates.sh` | **0** | [postmerge-check-gates.log](evidence/savebar-91b9a6a8/receipts/postmerge-check-gates.log) |
| `pnpm --filter @live-commerce/admin typecheck` | **0** | [postmerge-typecheck.log](evidence/savebar-91b9a6a8/receipts/postmerge-typecheck.log) |
| Strict spec command below | **0** | [postmerge-spec-typecheck.log](evidence/savebar-91b9a6a8/receipts/postmerge-spec-typecheck.log) |

```sh
pnpm exec tsc --noEmit --strict --skipLibCheck --target ES2023 --module esnext --moduleResolution bundler --esModuleInterop --allowImportingTsExtensions --typeRoots apps/admin/node_modules/@types --types node tests/admin/product-editor.acceptance.ts tests/admin/product-editor-layout.acceptance.ts tests/admin/ads.spec.ts
```

Representative unmodified before/after pixels:

- [375px failing SEO before](evidence/original/before/seo-en-375-failed.png) → [375px SEO above stable footer](evidence/savebar-91b9a6a8/product/shots/editor-bottom-en-375.png).
- [zh-TW product before](evidence/original/before/products-new/zh-TW-390x844.png) → [hint and full status](evidence/savebar-91b9a6a8/product/shots/editor-zh-TW-390-seo.png) / [saved edit scroll end](evidence/savebar-91b9a6a8/product/shots/edit-zh-TW-390-scroll-end.png).
- [zh-TW ads before](evidence/original/before/ads/zh-TW-390x844.png) → [NT$ after](evidence/savebar-91b9a6a8/visual/shots/admin/ads/zh-TW-390x844.png).
- Latest required supplemental examples: [zh-TW ads 1440](evidence/supplemental-630e1c86/visual/shots/admin/ads/zh-TW-1440x900.png), [en new product 1440](evidence/supplemental-630e1c86/visual/shots/admin/products-new/en-1440x900.png), [zh-CN saved product 1440](evidence/supplemental-630e1c86/visual/shots/admin/products-product/zh-CN-1440x900.png).

## Evidence packaging checks and remaining scope

Packaging red: missing committed manifest → exit **1**, [log](evidence/supplemental-630e1c86/receipts/evidence-red.log), [exit](evidence/supplemental-630e1c86/receipts/evidence-red.exit). Required-image omission calibration: [log](evidence-missing-image-red.log), [exit](evidence-missing-image-red.exit). Provenance-hash corruption calibration: [log](evidence-provenance-red.log), [exit](evidence-provenance-red.exit), expected mismatch exit **1**. Restored repository-only check: [log](verification-green.log), [exit](verification-green.exit). Current relevant Node/typecheck/gate receipts are recorded in [packaging-checks.json](packaging-checks.json).

Evidence: **E3 local BROWSER + REAL_PG with MOCK services**, bound to source hashes and retained committed artifacts; DB-free injected actual-verdict regression is also E3. Packaging-only batch runtime rerun is **NOT_RUN** (tested runtime source unchanged). Full Node's optional `tests/media/r04-input-runner.test.mjs` is NOT_RUN because `COMMERCE_R04_LIVEKIT_BINARY` is unset. Planner-selected broader modes, click-sweep journeys, separate focused Go, independent runtime rerun, provider SANDBOX/LIVE, production and deploy are NOT_RUN for this batch. The reused Go sweep harness proves visual G-UI9 here, not click journeys.

Author: Codex UI worker; deployed model/effort not exposed. Independent read-only evidence audit: `r43_evidence_review`, gpt-6.1-sol / medium, base `630e1c86`, no code writes/runtime or recursive delegation. Source-graph indexing previously found no MJS entity (**NOT_LINKED**); no graph claim is added. Existing independent review P2 FOLLOWUPS stay deferred; pill-versus-tab idiom and uncertain unreproduced visual impressions remain skipped as instructed.
