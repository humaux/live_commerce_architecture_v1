# product-editor-visual

Source: `ece52ead` (application changes through `dcf12327`), base `0cc63a1d`.
Branch: `unit/product-editor-visual`; worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/product-editor-visual`.
Status: PASS for this scoped UI unit; ready for integrator/owner acceptance. Application/editor browser/static gates and the final serial product-route sweep all passed.

## Scope and implementation

Only `apps/admin/**`, `tests/admin/**`, and this evidence directory are changed. No Go, SQL, packages/ui, dependency/lockfile, tests/ui, shell, production, credential, push, or deployment change.

| Requested defect | Result / source |
| --- | --- |
| 1. Spec labels, values and remove alignment | PASS. One aligned grid; 44px name/value/remove controls; committed values become removable tags. Clipboard newlines/comma and Enter work. IME composition does not commit prematurely. |
| 2. SKU matrix | PASS. Explicit columns, proper variant names, inline tracking checkbox and quantity; 65px single-line rows / 44px stock controls. Mobile scroll is inside the matrix, not the document. |
| 3. Five bulk buttons | PASS. One compact bulk control chooses field, value, selected/all target, all/empty scope and operation. Same local draft writer; prefix fill preserves matrix ordinals for selected rows. |
| 4. Duplicate image readiness | PASS. Images appear only in required items. A shared readiness renderer puts localized Ready/To do beside labels, not tiny distant circles. |
| 5. Phantom price/stock navigation | PASS. Pricing nav exists only with the single-SKU pricing section; variant products link price/stock readiness to the variants section. All rendered nav items were really clicked. |
| 6. Helpers/counters | PASS. Scoped helper size at least 12px; removed tiny bold option hint treatment. |
| 7. Accent | PASS. Editor actions, tracking/active checks, focus/state styling use incumbent shell/action teal tokens. Other shell navigation is unchanged. |
| 8. Unbounded form / empty uploader | PASS. Bounded document, 640px single-line field maximum on desktop; compact empty upload tile and adjacent instructions. Mobile fields332px. |

The `product-document.ts`, `catalog-v2-write.ts`, and `use-product-document.ts` libraries are unchanged from base (`git diff --exit-code`, exit0). Merge-patch writes, exclusive archive entries, image ordering/recovery and receipt fences remain exercised by the frozen tests. No product API or save-path duplication.

## Audit and measurements

Audit ran before application edits: [AUDIT.md](AUDIT.md). Impeccable audit-first guidance selected structural deduplication/alignment, not a new design system. Independent source and screenshot reviews: [REVIEW.md](REVIEW.md).

| Measurement | Before | Final |
| --- | --- | --- |
| Desktop spec inputs | 44px vs50px,51–54px vertical offset | Both44px,0px offset; remove aligned |
| Desktop name field | 1047px in actual baseline | 640px |
| Desktop SKU row / stock group | 109px /88px | 65px /44px |
| Mobile SKU row | 507–525px stacked cards | 65px scrollable table rows |
| Relevant helpers | 10px present | All measured helpers≥12px |
| Body horizontal overflow | 0 in24states | 0 in24states |

Full-page screenshots: **24 before +24 after** in `before/` and `after/`, plus measurements.json in each. Cartesian coverage: new/existing ×empty/three-variant ×zh-TW/zh-CN/en ×1586×992/390×844. Existing empty means valid saved product with no photos/options, not a nameless invalid product. Fixture product values stay literal, not translated. Screenshots preserve the normal fixed save bar at viewport bottom; see review caveat.

`red-alignment.log`: new equal-height assertion against the original measured fixture exits1. `green-geometry.log`: all24 final states and all12 variant states meet the same geometry and overflow expectations, exit0.

## Gates on final source

All commands run from this worktree. Provider interactions are MOCK; browser harnesses use actual Next/Go and isolated REAL_PG. No live merchant data or provider acceptance is implied.

| Command | Exit | Evidence / counts |
| --- | --- | --- |
| `bash scripts/dev/test-node.sh` | 0 | `test-node-final.log`:398 tests pass,0 fail; external LiveKit R04 explicitly NOT_RUN |
| `pnpm --filter admin exec tsc --noEmit` | 0 | `admin-tsc-final.log` |
| `bash scripts/dev/check-gates.sh` | 0 | `check-gates-final.log`:63 documented modes; existing warnings retained |
| `PRODUCT_VISUAL_PHASE=after LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-local.sh --browser-product-editor` | 0 | `browser-final.log`, `playwright-pe-final.log`12/12 and `playwright-cc12-final.log`2/2; Go postflight exact product/SKU readback PASS |
| `LC_TEST_LOCK_WAIT=14400 LC_SWEEP_ONLY=admin LC_SWEEP_PAGES=/products LC_SWEEP_WORKERS=1 bash scripts/dev/test-local.sh --browser-click-sweep` | 0 | `click-sweep-final.log`, `click-sweep/ledger.{json,md}`:9 page/viewport/locale units,0 load failures;195 pass/0 fail/10 skip;0 journey steps in filtered mode. Wrapper's separate public-site runner also passed30 page cases,390 clicks and30 reloads. |
| Scoped Impeccable layout detector on five changed component/CSS files | 0 | `detector-final.json`=[]; this is static advice, not visual acceptance |
| Readback geometry assertions | 0 | `green-geometry.log` |

Official editor mode remains registered, not a new alias. Existing assertions were not deleted or weakened. Old per-field bulk button selectors are replaced with the new open+field selection path. The optional visual phase adds real-click control/persistence checks and screenshot assertions without changing the default acceptance mode.

Additional click evidence: `controls-ledger.json` for all/selected bulk price/compare/quantity/code/keyword, quantity add/subtract, tag paste/removal, SKU cap and persisted reload; all section-nav targets clicked in each of24 final states. Product G-UI8 is deliberately filtered, not a full-platform sweep. Registry routes are list/detail/import; the new editor is additionally entered and exercised by the real-click PE/visual tests.

The ten sweep skips are list dialog Save changes actions: the scanner deliberately exercises cancel rather than saving. They are not counted as passes. Actual saving/persistence is separately verified by PE12–17 and the new editor-control test. The stock and bulk behavior was not accepted based on a sweep's superficial click alone.

## Red runs and directed repairs

- `red-after-1/`: the first final matrix run caught the section observer highlighting Basics after focusing Images. Fixed focused-section precedence; no assertion relaxed.
- `red-final-1/`: Form814 lines tripped the G-UI5 limit; extracted shared readiness rendering, now794. A concurrently launched PE and sweep build in the same worktree also produced two non-reproduced PE/CC12 failures (receipt recovery, image fetch). These were not fixed by changing assertions; all final builds/runs were sequenced. Concurrent build interference is an inference, not proven by an error trace.
- `red-final-2/`: new SKU-cap locator matched Next's hidden route announcer as well as the editor alert. Scoped it to `.pe-document`; exact message, preserved input and row count assertions retained.
- `red-sweep-1/`: default three-worker sweep interleaved a list Copy operation (`r00106`) with an editor's local Remove (`r00107`). Guard recorded no request from Remove, but its global store fingerprint changed. Both units share one fixture and fingerprint; final run uses the supported `LC_SWEEP_WORKERS=1`, with identical controls and assertions. No tests/ui edit.

The independent harness reviewer confirmed workers1 only serializes the same queue and leaves guard/requests/fingerprint/assertions intact. Its green run supports scoped functional acceptance, not a claim that default three-worker fingerprint concurrency has been fixed. That harness concern remains for its owning agent; the precise cross-request timing is inferred from the ledger, not a network trace. Original unfiltered-mode banner text in the wrapper is generic; use the actual scoped ledger counts above.

## Commits

- `bbda1983` — baseline audit/capture harness and red evidence.
- `54f04271` — structural editor fixes, tag/compact bulk controls, regression coverage.
- `dcf12327` — shared readiness renderer to satisfy existing component-size gate.
- `ece52ead` — scope new SKU-cap alert locator to the editor.
- Final evidence-only commit follows these; no source changes after final gates.

Every commit includes `Co-Authored-By: Codex <noreply@openai.com>`.

## Ownership, records and NOT_RUN

Root author: Codex current runtime (exact runtime model/effort not exposed). Humaux task `fbb53cdb-a4a8-4e3b-8b45-799e92046a25`; worktree lock owned by `codex-product-editor-visual`. Independent review roles/models/paths are recorded in REVIEW.md; no subagent had write ownership. Source indexed incrementally and repair memory `333f72b5-5bb8-4133-ab12-4bc07216b972` linked to ProductDocumentForm, ProductDocumentVariants, ProductBulkFill and AxisValues.

NOT_RUN: production/deployment/provider live tests, global all-route G-UI8, full G07/standalone backend suite (UI-only unit), separate layout-lint owned by the other agent, physical mobile/Safari/assistive technology/real IME validation, performance benchmark. Test-node's `tests/media/r04-input-runner.test.mjs` is explicitly NOT_RUN because `COMMERCE_R04_LIVEKIT_BINARY` is unset. Final aesthetic approval remains owner/integrator responsibility; independent screenshot review found no new confirmed blocker.

Housekeeping: prescribed harnesses regenerate historical product/platform/sweep outputs in this worktree. Relevant logs/ledgers/screens are preserved here; those unrelated tracked artifacts are restored to base. Runner-owned browser/Next/PG processes and fixture are cleaned by the runners; no foreign process/lock/directory was removed. Raw build logs retain terminal carriage returns as evidence; source-only diff whitespace check exits0.
