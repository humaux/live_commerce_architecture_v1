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
- bash scripts/dev/check-gates.sh → exit 1 in611.30s; matrix-execution passed, unrelated TestReviewR1bImportedSinkValuesRealCLI/alias hit CLI deadline then contractdrift package timed out after10m. check-gates.log/.json/.exit; unchanged=true. Node/check-gates retry awaits parent's serialized resource slot.
- git diff --check → exit 0.
- Resource snapshot: resource-observation.log (load25.78/30.90/29.03; command names only).

## Evidence and outstanding work

- E3 for focused Node mutation gate on exact source-hashes.json; author self-check only. Independent review: NOT_RUN by this author.
- Full Node gate and check-gates failed outside this task module; serialized retries/independent acceptance pending. Full browser/REAL_PG/full foundation/runtime/provider/SANDBOX/LIVE gates: NOT_RUN by this author, per assigned scope and shared resource ownership.
- Production/provider actions: none. No external credentials required; BLOCKED_EXTERNAL: none.
- Integrator to-do: independent review/re-run, serialized Node retry and canonical browser acceptance/CI as appropriate; push and open focused PR. No author push/PR/merge.

Final source hashes: implementation `b39b4377c1f1e9592ab351cfc500279f8eec2482f38764d1702663eeaf072811`; tests `1ee16f0d106bde822990826222ed29a2e3c663cb3182da9a314f7a664c01249e`. No source changes after focused GREEN.
