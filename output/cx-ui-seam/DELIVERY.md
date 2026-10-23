# #147 UI seam gate delivery

- Role: UI implementer (takeover). Model: `deepseek-flash`; effort: inherited, not exposed.
- Base: `d5418f4c97ea79ddd678131e86ae59fde5bd6052` (`r3/integration`).
- Worktree: `/Volumes/data/live_commerce_architecture_v1/.claude/worktrees/cx-ui-seam`; branch: `fix/ui-seam-gate`; draft PR #169.
- Allowed write paths: `apps/admin/components/UiGalleryTemplates.tsx`, `apps/storefront/tests/upstream-body-cancel.test.mjs`, `tests/ci/gallery-upload-seam.test.mjs`, `scripts/dev/test-node.sh`, `scripts/dev/test-local.sh`, this delivery record.
- Authority: latest task card; review-151-19339f4c6.md P2-1 and review-160-18644c079.md P2-1 (integrator output/issue-107).

## RED checkpoint (CI only, never run locally)

- RED commit: `8178286a5` (new static seam check `tests/ci/gallery-upload-seam.test.mjs` + its `scripts/dev/test-node.sh` wiring; product code unchanged).
- Foundation run **39891070421** → failure. Only failing test: `tests/ci/gallery-upload-seam.test.mjs:32` “gallery upload seam is unreachable without the public build-time flag” (assert at `:24`). The same file's negative guard test passed; every other suite in the step stayed at `ℹ fail 0`. The seam suite was the only `✖` in the run.

## GREEN

- Fix commit: `cdc452f4fa93525b751aa742ffcb0923ec8c75e0`.
- Foundation run **39891665846** → success (all steps).
- Gates run **39891665886**: `gate (--browser-ui-gallery)` job **120213379438** → success (real-click upload hold/release ledger intact with the build flag set).
- Unrelated flake in the same gates run: `gate (--browser-promotions)` job 120213378399 failed at `tests/storefront/promotions-gate.mjs:95` with `page.screenshot: Timeout 30000ms exceeded … waiting for fonts to load` after 7 PASS scenarios; this diff touches no promotions or font path. _RERUN: PENDING_.

## Root cause: the flag must be set at admin BUILD time

- `apps/admin/components/UiGalleryTemplates.tsx:223` now puts the `window.__lcHoldUpload` / `lc-gallery-upload-release` seam only inside `if (process.env.NEXT_PUBLIC_LC_GALLERY_TEST === "1") { … }`, so a production build folds the literal to false and drops the branch.
- `scripts/dev/test-local.sh:140` sets `NEXT_PUBLIC_LC_GALLERY_TEST=1` on the `--browser-ui-gallery` **build** command.
- Answer to the root-cause question: **yes, the flag is set at admin build time in that gate.** `pnpm --filter @live-commerce/admin build` is the only step that can inline `NEXT_PUBLIC_*`; the runner only starts the packaged output — `tests/admin/ui-gallery-runner.mjs:43` spawns `scripts/dev/start-admin.mjs`, whose server path is `apps/admin/.next/standalone/apps/admin/server.js` (`scripts/dev/start-admin.mjs:9`), with `output: "standalone"` set in `apps/admin/next.config.ts:23`. Setting the flag at `next start` would be too late.

## Storefront mock fix

- `apps/storefront/tests/upstream-body-cancel.test.mjs:123,131,139`: the three partial-stream subtests now mock with their own `st.mock` (auto-restored at subtest end) instead of the parent's `t.mock`; the manual `mock.mock.restore()` calls that a throwing assertion used to skip are gone. Every assertion is unchanged.

## Local checks (allowed commands only)

- `node --check` on both `.mjs` files and `bash -n scripts/dev/test-local.sh` → OK.
- `prepush.sh` → `check-headers: OK (base d5418f4c9)` / `prepush: OK`.
- No local test, browser, build, server, PG or Docker run.

## NOT_RUN

- Local tests / browser runs / builds / servers / containers; production, SANDBOX and LIVE actions.
