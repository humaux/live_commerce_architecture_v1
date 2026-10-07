<!-- Purpose: current PM-U click-sweep repair and evidence boundaries.
Depends on: CIce13e6fa, current integration0617bc40, repair source-manifest and main evidence logs.
Used by: integrator review/GitHub rerun; current browser acceptance remains pending. -->
# PM-U product-media-v2 UI — remaining click-sweep repair

- Branch `unit/product-media-v2-ui`. CI baseline `ce13e6fa9ae79fef90fbc88db8844fd62006ec81` fast-forwarded, then current trunk `0617bc40f4a50e84bd57d6eed00e87167582ca9a` merged as `5d0b24482c2c2693465a0b680c00bd7eda3a6aa0` before checks. Final SHA is the commit containing this file; main `commit-receipt.json` records it.
- Repair source binding `421c22bd0a070c832b83e6d7d0a0547d626f95f40eac0286087ce9d1c43a354e` covers six changed source/test files in `ci-37569737876/source-manifest.json`. Prior1778805b delivery/manifest/runtime evidence retained.
- Main evidence root `/Volumes/data/live_commerce_architecture_v1/output/product-media-v2-ui/ci-37569737876/`.
- Author Codex GPT-6; actual configured model/effort not exposed. Two separate read-only reviews: `codex-product-media-v2-ui-sub-zero-main-review` (migration/backend and final hint), `codex-product-media-v2-ui-sub-j1-readonly` (PNG/driver and final protocol). No recursive delegation or child source edits.

## Changes and root causes

ACTIVE products with zero main images are a legal persisted state. Migration0149 moves existing image rows without creating missing images or changing product status. The actual catalog writer, image move/delete and buyer projection allow this state; the sweep's primary ACTIVE/no-photo product is valid coverage. The fixture is preserved.

`ProductDocumentForm.tsx` renders the existing complete repair text as a plain inline hint with polite announcement and an ID linked from the status field. It does not label a successful page load as an alert/error. Active save/publish restrictions and enabled upload/unpublish repair remain unchanged. The real-upload spec now also requires the hint visible and without an error/status role.

J1's old input/row IDs still exist; the failure is not a selector rename. Its valid70-byte PNG has correct CRCs and a complete pixel stream. The old direct setInputFiles ran after navigation without waiting for canonical media readiness. Manager legitimately mounts a disabled input and ignores upload events while loading. CI had no upload network trace, so the exact event-time state remains UNKNOWN; the source-permitted race and missing driver guarantee are established.

The new small `product-media-journey.mjs` driver waits for the input enabled, clicks the actual input/label and answers a real file chooser. It observes the matching POST for the current store/product with role=main, requires the frozen200 acknowledgement and valid image UUID, then checks the canonical main tile. Main1/option0/detail0 assertions replace a global gallery count. J1 retains activation/save/reload and now binds the same image ID after reload and on the buyer page, including read-only decode verification. The driver makes no direct API write or DOM mutation. The original PNG, fixture, sweep matrix, classifier, thresholds and known-defect registry are unchanged.

## Local commands / exits

| Command | Exit | Evidence |
|---|---:|---|
| `node --test --experimental-strip-types tests/admin/product-media-ui-model.test.ts tests/ui/click-sweep-lib.test.mjs` before/after |1→0| `red.log`20PASS/4FAIL → `green-final.log`24PASS/0FAIL; actual form hint JSX and driver readiness/role/failure negatives |
| `bash scripts/dev/test-node.sh` |0| `node-final.log`516PASS/0FAIL/0SKIP; optional pinned R04 binary explicitly NOT_RUN |
| `pnpm run typecheck:admin` |0| `tsc-admin-final.log` |
| `pnpm exec tsc --noEmit --target ES2022 --module esnext --moduleResolution bundler --strict --skipLibCheck --esModuleInterop --allowImportingTsExtensions --types node --typeRoots apps/admin/node_modules/@types tests/admin/product-media-ui-model.test.ts tests/admin/product-media-v2.spec.ts` |0| `tsc-tests.log` |
| `pnpm exec tsc --noEmit --allowJs --checkJs --strict --target ES2023 --module ESNext --moduleResolution bundler --skipLibCheck --types node --typeRoots apps/admin/node_modules/@types tests/ui/product-media-journey.mjs` |0| `tsc-journey.log` |
| `node --check tests/ui/product-media-journey.mjs`; same for click-sweep.mjs / click-sweep-lib.test.mjs |0 each| syntax checks; no browser start |
| `bash scripts/dev/check-gates.sh`; `git diff --check` |0 each| `check-gates.log`:76 modes, header ratchet/architecture/vet/format |
| Source hash verification |0| six files match the frozen manifest |

The initial protocol red log was archived before correcting its test-fixture response status to the frozen200; `red.log` above is the valid unchanged-driver red. No failing suite, guard or fixture was removed. Local E3 applies only to MODEL_ONLY Node regressions; tsc/static checks are E1. Actual-JSX tests render the hint fragment, not the full browser form. Page/check doubles exercise driver sequencing and rejection, not browser decoding/uploads.

Independent source review reports no concrete P0/P1 blocker or weakening. Review artifacts `zero-main-source-review.md` and `j1-source-review.md` contain anchors, hashes and scope limits. Source indexing was submitted and the ProductDocumentForm reason link succeeded in Humaux. The new MJS helper entity was still absent after a targeted re-index, so its code-memory link is NOT_DONE; source hashes, the fix memory and review preserve its rationale.

## Prior GitHub evidence (ce13e6fa only)

Run37569737876: product-media-v2 normal, product-editor, catalog-core, catalog-media, storefront and visual-lint GREEN. Click-sweep RED has exactly4 new failures: three zero-main page loads (`r00248`, `r00354`, `r00430`) and J1 cover step (`r01144`). Full gate5 artifact and its ledger/log/screenshots are retained under `artifacts/gate-5/`.

Fault run37569741446: expected RED for `PM_CALIBRATION_DETAIL_COUNT: five real uploads; six required`. GitHub failed log records the Go `calibration RED: omitted detail image caught by six-image assertion`; that branch requires the exact marker before declaring calibration RED. This is a valid historical new-gate calibration record. It does not prove the new repair SHA green.

## CI gates — integrator runs on GitHub

- `bash scripts/dev/test-local.sh --browser-click-sweep` — required full sweep/J1 verification on this SHA.
- `bash scripts/dev/test-local.sh --browser-product-media-v2` — full4main/2option/6detail and zero-main repair scenario with the retained new hint assertions.
- `bash scripts/dev/test-local.sh --browser-product-editor` — editor regression.
- `bash scripts/dev/test-local.sh --browser-visual-lint` — inline-hint layout regression.

Catalog-core/media/storefront were green atce13 and their media runtime was not changed in this repair. Integrator may include those modes in the final release run. The existing fault injection remains available; its valid ce13 calibration record is retained.

## NOT_RUN / limits / queue

Current-SHA browser/click-sweep/visual/real-upload, production builds, PostgreSQL/full foundation, Linux/Xvfb and live services are NOT_RUN locally this turn, as instructed. R04 media binary absent. No task-owned browser/server/container was started. No push or release merge; only authorized unit synchronization with integration.

J1 checks one main cover and absence of option/detail uploads; it does not replace the full media gate. The Node driver test does not exercise its hidden-input label branch or real chooser/canvas. Current GitHub acceptance is required before PM-U is fully green and W6-U1 may start. W4-U1 belongs to Kimi. The queued pipeline remains W6-U1 → W3-U4 (after owner-confirmed W3-U1b merge) → W5-U1.
