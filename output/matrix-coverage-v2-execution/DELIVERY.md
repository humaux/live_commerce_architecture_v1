# MATRIX-COVERAGE-V2 GROUP2 delivery

- task_id: f597a8bc-ce29-4b68-ab31-815dd5a319cb (parent); sub-element codex4-matrix-v2-sub-execution. Issue #45 GROUP2.
- Branch: unit/matrix-coverage-v2-execution. Base_commit: 0bfe36e53acb694f4697521202e24f0790638941. Commit: resolve with `git log -1 unit/matrix-coverage-v2-execution`; source bytes are pinned in source-hashes.json.
- Worktree: /Volumes/data/live_commerce_architecture_v1/.worktrees/matrix-coverage-v2-execution.
- Role: implementation/test author. Actual precise model/reasoning identifiers: UNKNOWN (not exposed to this delegated runtime). No recursive delegation.
- Changed paths: tests/ci/product-editor-matrix-execution.mjs; tests/ci/product-editor-matrix-execution.test.mjs; output/matrix-coverage-v2-execution/**. foundation.yml remains unchanged.
- Summary: literal argv parsing now removes assignment prefixes before identifying node/go; required Node suites refuse --test-only/--test-shard/--test-concurrency plus prior pattern filters; Go refuses --run/--skip/-test.run/-test.skip in space/equals/quoted forms. Real foundation job/step/run binding rejects retained-filename comment/name/echo/disabled/unreachable substitutes. Go registry requires the executable to be go test, rather than incidental echo text.
- Interface: existing matrixExecutionIssues API remains compatible; optional foundationWorkflow string defaults to reading the actual foundation.yml. No runtime product, shared schema or dependency changes.
- Rationale: reuse the existing quote-aware literal argv helper and accept only the repository's current workflow subset; no shell execution, no YAML dependency, no duplicated CI runner. Workflow shape changes fail closed and require review.

## Actual commands and evidence

- pnpm install --frozen-lockfile --ignore-scripts → exit 0: install.log/install.exit (no lockfile change).
- node --test tests/ci/product-editor-matrix-execution.test.mjs → RED exit 1, pass32/fail83: red.log/red.exit, unchanged base implementation. Initial test-authoring SyntaxError is separately retained in test-authoring-syntax.log/.exit and is not the RED proof.
- Same focused command with two added reachability tests → RED exit 1, pass115/fail2: red-reachability.log/.exit.
- Same focused command → final GREEN exit 0, pass117/fail0: green.log/.exit, bound by source-hashes.json.
- Candidate Go executable-prefix regression → focused exit 1, pass110/fail7: candidate-go-prefix-regression.log/.exit. Corrected by reusing assignment-prefix removal for actual GOTOOLCHAIN-prefixed Go command.
- First broad Node attempt edited during execution: invalid for acceptance, stopped exact owned PIDs; test-node-invalid-source-change.log/.json/.exit (child -15, shell 241).
- Premature broad Node start while focused regression was red: stopped exact owned PIDs; test-node-premature-focus-red.log/.json/.exit (child -15, shell 241). Not acceptance.
- bash scripts/dev/test-node.sh → fixed-source exit 1 in155.09s: test-node.log/.json/.exit, unchanged=true. Failures: next-startup stolen-port fixture timed out after10000ms; three playwright-deps installer fixtures timed out/status null or zero retries. GROUP2 remains117/117. No failing test or threshold changed.
- bash scripts/dev/check-gates.sh → exit 1 in611.30s; matrix-execution passed, unrelated TestReviewR1bImportedSinkValuesRealCLI/alias hit CLI deadline then contractdrift package timed out after10m. check-gates.log/.json/.exit; unchanged=true. Both original commands passed in serialized retry; initial failures remain retained.
- git diff --check → exit 0.
- Resource snapshot: resource-observation.log (load25.78/30.90/29.03; command names only).

## Evidence and outstanding work

- E3 for focused Node mutation gate on exact source-hashes.json; author self-check only. Independent review: NOT_RUN by this author.
- Initial full Node/check-gates failed outside this task module; original commands passed in the parent-assigned serialized retry. Independent acceptance remains pending. Full browser/REAL_PG/full foundation/runtime/provider/SANDBOX/LIVE gates: NOT_RUN by this author, per assigned scope and shared resource ownership.
- Production/provider actions: none. No external credentials required; BLOCKED_EXTERNAL: none.
- Integrator to-do: independent review/re-run, serialized Node retry and canonical browser acceptance/CI as appropriate; push and open focused PR. No author push/PR/merge.

Final source hashes: implementation `b39b4377c1f1e9592ab351cfc500279f8eec2482f38764d1702663eeaf072811`; tests `1ee16f0d106bde822990826222ed29a2e3c663cb3182da9a314f7a664c01249e`. No source changes after focused GREEN.

## Final serialized retry (source commit 6f46db7788acad1ee66da11eb6f27b3f738f3c3d)

- bash scripts/dev/test-node.sh → exit 0, 144.138s, completed Node totals {'tests': 1786, 'pass': 1786, 'fail': 0, 'skipped': 0, 'cancelled': 0}: test-node-retry.log/.json/.exit, unchanged=true.
- bash scripts/dev/check-gates.sh → exit 0, 293.167s: check-gates-retry.log/.json/.exit, unchanged=true.
- No executable/test bytes changed since the final focused GREEN. Final receipt commit changes output only; no further retries. Local gates E3 within tested source environment. Browser/REAL_PG/independent review/CI remain NOT_RUN by author. No P0/P1 identified in scoped self-review; independent acceptance belongs to parent.

## Issue #45 audit bypass follow-up (base 8040c36605666a283a7eb048f9f99794b40a27e2)

- Scope remains only `tests/ci/product-editor-matrix-execution.mjs`, `tests/ci/product-editor-matrix-execution.test.mjs`, and this evidence directory.
- Added fail-closed checks for literal `GOFLAGS` selection switches (all existing Go aliases, space and equals forms) on the command and workflow environment; shell control operators in parsed command argv; and `continue-on-error: true` in the foundation job/steps.
- `node --test tests/ci/product-editor-matrix-execution.test.mjs` before implementation → exit 1, 124 tests: pass 118, fail 6. This reproduces GOFLAGS `-run=^$`, Node/Go trailing `|| true`, and job/step `continue-on-error: true`; see `issue45-red.log`.
- Same focused command after implementation → exit 0, 136 tests: pass 136, fail 0; see `issue45-focused-green.log`.
- `git diff --check` → exit 0. Full `bash scripts/dev/test-node.sh` and `bash scripts/dev/check-gates.sh` → NOT_RUN for this follow-up (HOLD pending root's serialized slot). Browser, REAL_PG, provider SANDBOX/LIVE and CI → NOT_RUN.
- Current source SHA256: implementation `754d0855c6dd1bd35022e797059f2f714e3cd3e2a37ff24df66efdfaaff4cd70`; tests `869cf7173ce8b1615391eb03053b4843eda69e65e30ac148ba60b3b74a0cfa93`.
- Parent task: `f597a8bc-ce29-4b68-ab31-815dd5a319cb`; current base at follow-up start: `8040c36605666a283a7eb048f9f99794b40a27e2`; commit for these changes: pending.

## Independent review follow-up (base b7a228829c32fdba05d0ac692c4a65ce4f092e66)

- Added red coverage for a quoted workflow scalar containing multiple GOFLAGS (`"-count=1 -run=^$"`) and expression-valued `continue-on-error: ${{ true }}` at job and runner-step scope. Literal `false` remains accepted.
- `node --test tests/ci/product-editor-matrix-execution.test.mjs` on the unfixed parser → exit 1, 140 tests: pass 136, fail 4; see `review-round-red.log`.
- Same focused command after fix → exit 0, 143 tests: pass 143, fail 0; see `review-round-green.log`. `git diff --check` → exit 0.
- Frozen source SHA256 before serialized full gates: implementation `03726d87ea9610b73931586f87928fcdea76d672adce5d29b0556719ce4e2f0e`; tests `9eab9ab6f9da74e8949c51655ee1933e73ebb88c1a54ff527bb4304c3244620f`.
- Frozen source `c59e43e0a97b813b3e2d4b661d8271f69fe63646` was run serially. `bash scripts/dev/test-node.sh` → exit 0, aggregated TAP totals 1812 tests / 1812 pass / 0 fail / 0 skipped across 30 summaries; receipt: `review-test-node.log`, `.json`, `.exit`.
- Then `bash scripts/dev/check-gates.sh` → exit 0; receipt: `review-check-gates.log`, `.json`, `.exit`. Output includes `check-gates: ok (83 modes, all documented; every tracked test file is run)` and `shard-plan: ok (1272 top-level tests, each in exactly one of 10 groups)`. G-UI5 size warnings remain informational under that gate's existing behavior.
- `full-static-source.before.txt` and `full-static-source.after.txt` show identical HEAD and source SHA256s across both runs. No executable/test source changed while either gate ran.
- Browser, REAL_PG, provider SANDBOX/LIVE and GitHub CI remain NOT_RUN. Independent acceptance remains root-owned; no production/provider actions were performed.
