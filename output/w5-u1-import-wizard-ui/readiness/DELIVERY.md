<!-- Purpose: W5 visible readiness current evidence.
Depends on: original main raw SHA256=544d43175b829ec97ca94dfd2b802e5c7561f3071312709d8bb7d2d1bf59058e.
Used by: integrator normal/calibration CI. -->
<!-- Purpose: narrow second-occurrence import signed-helper readiness guard with exact extracted-source red/green.
Depends on: actual signed AST from import-wizard.spec.ts, TypeScript transpiler, native Node VM/test and tiny modeled browser/page.
Used by: root test-node/GATES integration and independent rerun; never real browser/auth/BFF acceptance. -->
# W5 visible readiness guard

- task_id: `a83bad3d-2b4d-4b4a-b598-85f9e4a99cea`; own subcanvas `codex-w5-u1-ui-sub-readiness` (no independent claim/delegation).
- Role/config: existing independent test_worker, configured gpt6.1/high; runtime UNKNOWN.
- Base: `019a09952272c189aacb73b3411ba2a0369154af`, fresh own worktree `/Volumes/data/live_commerce_architecture_v1/.worktrees/w5-u1-readiness-tests`, branch `unit/w5-u1-readiness-tests`.
- Test-only commit: `bd1b0efe89b50157cc03b18e129e4afe2151b2bc`, clean; changed ONLY `tests/admin/import-readiness.test.mjs` (54 lines).
- Implementation: parse the actual spec AST, locate signed FunctionDeclaration, transpile/execute its actual body. Minimal fake browser/page facilities model a closed details-menu sign-out control and visible store selector. Both zh-TW390/en1440 must click sign in before waiting on visible shell-store-selector, visit the actual wizard path, execute the Secure/httpOnly cookie assertion and wizard-visible assertion. Negative Secure=false/httpOnly=false fixtures must reject the cookie assertion. No auth/CSRF/BFF helper is invoked or faked; ctl/step/expect/locator facilities are explicitly helper-MOCK only.

## Actual evidence

Command: `node --test tests/admin/import-readiness.test.mjs`.

- RED baseline: exit1, PASS0/FAIL1/SKIP0, named `MIUI-READY-VISIBLE-STORE: sign-out is inside a closed details menu`; runw5-readiness-node-red PID93839 FINISHED, `red.log` / `red-status.json`. Root notified BEFORE its one-line edit.
- Root patch: own temporary spec switched readiness to shell-store-selector; candidate spec SHA `669f2b3807a66a0d884eada96427a79a76032b6ab3becaebd7e45dbd984b1d21` matches root.
- GREEN candidate: exit0, PASS1/FAIL0/SKIP0, runw5-readiness-node-green PID96375 FINISHED, `green.log` / `green-status.json`. Its one test executes both dimensions and both negative cookie paths; no thresholds or test changed between red/green.
- Baseline spec SHA: `7ad40ef290a5e15710929de77f716f094b2f60ae633bd44d0cc5f1b4fe978f80`.
- Guard SHA: `27be8aa53e9e92e093e3b7eece6660368883dfded9ada83f8dd366238b08d853`.

Other actual commands: `git worktree add -b unit/w5-u1-readiness-tests <path> <base>` exit0 (`readiness-worktree.log`); offline frozen pnpm install exit0 (`readiness-install.log`), no dependency/version/lock changes; `node --check tests/admin/import-readiness.test.mjs` exit0; `git apply --check <root-source.patch>` and apply exit0; staged diff-check exit0; commit exit0; git status/readback exit0/clean. Branch-existence preflight nonexisting ref was expected, not a test failure.

Both Node runs have outer120s process-group timeout; status JSON registers runid/PID/command/start/end/log/exit/wake. Current child monitored terminal completion and messaged root; no unattended process. Only temporary OWN spec copy was restored path-scoped to base after green and never staged/committed. Isolated guard branch therefore remains red without root's one-line helper fix: root must integrate both, then independently rerun/register its shared import Node group/GATES.

Evidence level: narrow E3/MOCK **browser-helper model** tied to exact extracted spec hashes; test-only source E1 until root integration. Not actual Playwright/BFF/authentication or visible device proof. The modeled visibility behavior is an oracle from the reported real failure; Node cannot establish actual Next layout. No PG/Next/browser/Go/build/heavy/CI execution; all NOT_RUN. Existing actual15-case normal browser gate and its named drop/truncate calibrations remain unchanged and still require CI.

No source commit, peer revert, push, PR, release merge, recursive delegation, new dependency, failed-sample deletion or acceptance relaxation. Own worktree and task evidence retained for root; no local servers/processes left running.

Humaux research readback: `afec8b86-7b06-4fe5-ae07-980f22d4f2d4`; own subcanvas updated and all advisory locks released. code_index accepted the MJS path but finished with0 files/0 entities; MJS indexing/linking is NOT_INDEXED, not asserted complete. Node evidence is unaffected.

Evidence: `/Volumes/data/live_commerce_architecture_v1/output/w5-u1-import-wizard-ui/readiness/`.
