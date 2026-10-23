# #147 UI seam gate delivery

- Role: UI implementer (takeover). Model: `deepseek-flash`; effort: inherited, not exposed.
- Base: `d5418f4c97ea79ddd678131e86ae59fde5bd6052` (`r3/integration`).
- Worktree: `/Volumes/data/live_commerce_architecture_v1/.claude/worktrees/cx-ui-seam`; branch: `fix/ui-seam-gate`; draft PR #169.
- Authority: latest task card; review-151-19339f4c6.md P2-1, review-160-18644c079.md P2-1, review-169-530c8bf3e.md P1-a/P1-b (integrator output/issue-107).

## Round 2 — review-169-530c8bf3e (P1-a: unset flag does not fold; P1-b: visitor misses spellings)

- Answer to the root-cause question: **round 1 did not verify elimination in an emitted bundle; it assumed it.** Its own record said `NOT_RUN: Local tests / browser runs / builds` (this file, round 1 below), and the review records `emitted bundles were NOT_RUN`. The claim that "a production build folds it to false" was an inference from the component source; with the flag unset Next defines no `NEXT_PUBLIC_*` key at all (`next/dist/lib/static-env.js:42-55` skips null values), so the seam strings stayed in client JS. This round proves it with an artifact gate instead of an inference (RED/GREEN below).
- P1-a fix: `apps/admin/next.config.ts:24-31` adds a `compiler.define` for `process.env.NEXT_PUBLIC_LC_GALLERY_TEST` = `"0"` **only when the variable is undefined**; the `--browser-ui-gallery` build still passes `1` (`scripts/dev/test-local.sh:140`) and Next rejects a define for an already-defined key, so the override is untouched. `compiler.define` (not an `env: {}` block) keeps the existing PS1 source invariant (`tests/admin/platform-site.test.ts:229`).
- P1-a check: `scripts/dev/check-gallery-bundle.mjs` scans executable JS under `apps/admin/.next/static`, fails on either marker (`--expect=absent`) and requires them (`--expect=present`, positive control). Wired into the production-shaped build every `lc_build=admin|both` gate already runs (`scripts/dev/test-local.sh:1677`) and into the flag-enabled gallery build (`:142`); no new build step.
- P1-b fix: `tests/ci/gallery-upload-seam.test.mjs` now scans every non-test TS/JS file under `apps/admin` (378 files), recognises Identifier, StringLiteral, NoSubstitutionTemplateLiteral and literal element access, requires **every** occurrence to sit inside the literal `NEXT_PUBLIC_LC_GALLERY_TEST === "1"` enabled branch, and rejects computed global property names plus fragment-built template literal/binary names. Fixtures cover element access, template literal, template fragments, concatenation, computed global name, element-access flag read, alias guard, else branch, marker in another module, and a nested-in-guard legal case. Fully indirect names (`String.fromCharCode`) are documented UNSUPPORTED and are not used here.

### RED (CI only, never local)

- P1-a RED commit `f733c74f4`. Gates dispatch run **39905758562**, job `gate (--browser-inbox)` **120257406351** → failure at `scripts/dev/check-gallery-bundle.mjs`: `FAIL 3 gallery seam marker(s) in 62 emitted client JS files` (`chunks/0-g_hxv9dlyc_.js:1` at 05:30:02Z). This is the proof the unset flag left the seam in the ordinary build.
- P1-b RED, same commit. Foundation run **39904514657** (job 120253263320) → failure; only failing test `literal element access and template-literal spellings are rejected outside the guarded branch` at `tests/ci/gallery-upload-seam.test.mjs:45`; the old visitor missed both shapes.

### GREEN

- P1-a GREEN, commit `eec94ed08` (product code identical on the later heads below): Gates dispatch run **39906706809**, job `gate (--browser-studio-bff)` **120260401819** → `check-gallery-bundle: OK expect=absent files=62 markers=0`. Positive control run **39906717317**, job `gate (--browser-ui-gallery)` **120260706901** → `check-gallery-bundle: OK expect=present files=62 markers=3` plus `ℹ pass 38 / ℹ fail 0` (real-click gallery ledger intact with the flag set).
- P1-a GREEN re-confirmed on `0f1eab73c`: Gates dispatch run **39910072516**, job `gate (--browser-studio-bff)` **120271025713** → `OK expect=absent files=62 markers=0`.
- P1-b GREEN, commit `d4257b258`: Foundation run **39910802854** → success. The step log shows `✔ every gallery seam marker in apps/admin production source sits inside the literal flag branch`, `✔ a seam moved into another production module is still found`, and `ℹ fail 0` throughout `test-node.sh`.
- No existing test, assertion, timeout, retry or fixture was weakened. The first GREEN attempt used an `env: {}` block and was rejected by the existing PS1 test (Foundation run **39905930455**, `tests/admin/platform-site.test.ts:229`); the production code was changed instead of the test. Two failure/rejection rounds in the new scanner were fixed inside the new test only (runs **39909037483**, **39910068331**).

## Round 1 (historical)

### RED checkpoint (CI only, never run locally)

- RED commit: `8178286a5` (new static seam check `tests/ci/gallery-upload-seam.test.mjs` + its `scripts/dev/test-node.sh` wiring; product code unchanged).
- Foundation run **39891070421** → failure. Only failing test: `tests/ci/gallery-upload-seam.test.mjs:32` “gallery upload seam is unreachable without the public build-time flag” (assert at `:24`). The same file's negative guard test passed; every other suite in the step stayed at `ℹ fail 0`. The seam suite was the only `✖` in the run.

### GREEN

- Fix commit: `cdc452f4fa93525b751aa742ffcb0923ec8c75e0`.
- Foundation run **39891665846** → success (all steps).
- Gates run **39891665886**: `gate (--browser-ui-gallery)` job **120213379438** → success (real-click upload hold/release ledger intact with the build flag set).
- Unrelated flake in the same gates run: `gate (--browser-promotions)` job 120213378399 failed at `tests/storefront/promotions-gate.mjs:95` with `page.screenshot: Timeout 30000ms exceeded … waiting for fonts to load` after 7 PASS scenarios; this diff touches no promotions or font path. _RERUN: PENDING_.

### Round 1 root cause: the flag must be set at admin BUILD time

- `apps/admin/components/UiGalleryTemplates.tsx:223` puts the `window.__lcHoldUpload` / `lc-gallery-upload-release` seam only inside `if (process.env.NEXT_PUBLIC_LC_GALLERY_TEST === "1") { … }`.
- `scripts/dev/test-local.sh:140` sets `NEXT_PUBLIC_LC_GALLERY_TEST=1` on the `--browser-ui-gallery` **build** command.
- `pnpm --filter @live-commerce/admin build` is the only step that can inline `NEXT_PUBLIC_*`; the runner only starts the packaged output — `tests/admin/ui-gallery-runner.mjs:43` spawns `scripts/dev/start-admin.mjs`, whose server path is `apps/admin/.next/standalone/apps/admin/server.js` (`scripts/dev/start-admin.mjs:9`), with `output: "standalone"` set in `apps/admin/next.config.ts`. Setting the flag at `next start` would be too late.

## Storefront mock fix

- `apps/storefront/tests/upstream-body-cancel.test.mjs:123,131,139`: the three partial-stream subtests now mock with their own `st.mock` (auto-restored at subtest end) instead of the parent's `t.mock`; the manual `mock.mock.restore()` calls that a throwing assertion used to skip are gone. Every assertion is unchanged.

## Local checks (allowed commands only)

- `node --check` on the `.mjs` files and `apps/admin/next.config.ts`, `bash -n` on the shell entrypoints → OK.
- `prepush.sh` → `check-headers: OK (base d5418f4c9)` / `prepush: OK`.
- No local test, browser, build, server, PG or Docker run.

## NOT_RUN

- Local tests / browser runs / builds / servers / containers; production, SANDBOX and LIVE actions.
- Full PR Gates matrix on the final head: still running at hand-off (the focused `--browser-studio-bff` and `--browser-ui-gallery` dispatches above are the same-sha evidence for this change).
