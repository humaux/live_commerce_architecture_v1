# Single test-local mode registry — implementation checkpoint

- Owner: Codex-4, primary implementer; current runtime model/effort inherited and not independently exposed. Read-only consumer explorer/security reviewer and independent test worker (isolated primary-output oracle directory, no source writes).
- Branch/worktree: unit/ci-pr-modes-coverage, .worktrees/ci-pr-modes-coverage. Base88d3ba1369de0b13c4a9d4ba8051c1540ca9cfd6 (integrator-provided merged trunk). No push.
- Scope: runner, selector, check/release consumers, their source-sensitive tests, GATES.md and this evidence. No app behavior/API/Go/SQL/dependency changes.

## Change and invariant

One native Bash case entry owns each mode's build profile, fixture requirement, preparation and run commands. Adding a mode appends one entry; there is no separate mode usage/allow/build/dispatch list. The82 prior flags plus defaultfoundation are preserved in their original order. Shared99-line PG lifecycle moved byte-for-byte into test-local-runtime.sh, with a new explanatory header. Bash3.2 remains supported.

pr-modes reads the native registry as data and retains the historical usage/dispatch reader for Git revisions preceding the refactor. It never executes a Git-sourced script. Legacy Git display quoting/trim classification remains unioned with raw-path source-aware selection; no required mode/deploy flag is dropped. check-gates and release-gate consume --list; build/Xvfb consumers now test selected actual shell plans rather than obsolete source offsets.

Metadata commands: --list and --dry-run MODE are inert; dry-run prints selected command/environment expressions, not a test PASS. Registry labels are plain standalone labels (indentation allowed); unsupported alias/inline/trailing-comment arms and duplicates fail closed. Independent review found an initial omission bug here; both the original spacing RED and subsequent trailing-comment RED are retained, and final review confirms closure.

## Red, green and equivalence

- Unchanged selector:22PASS/2FAIL. The added-mode fixture depended on the old first usage token and no longer inserted its test mode after trunk changed the prefix. Assertions were retained; fixtures now append one real registry entry.
- Original runner lacks metadata discovery/dry-run:2 new tests RED. New registry/selector suite:28PASS; hidden consumer regression suite44PASS.
- Independent actual Bash3.2 command/environment oracle: **120/120 equivalent** (83 normal entries +37 flag/guard/failure variants), exit0. It executes old and new shells against controlled external-command boundaries, not a metadata-generated expected result. Command argv/order/environment, exit codes and artifact hashes are compared. Only task-root paths and generated process IDs are normalized.
- Identity control120/120 zero differences. Three deliberate changes (Go timeout, build environment, removed earlyexit) each produce RED. V1 correctly found moved guard/cd environment and admission order; fixed, finalv4 zero differences.
- Metadata85/85 checks: exact83 mode list, every normal dry-run plus storefrontMOCK, no effectful command. Sanitized full before/after listings and negative controls in command-equivalence.tar.gz; raw primary evidence stays outsideGit. Hash linkage and exact commands in RECEIPT-v4.json and COVERAGE-v4.json.
- Existing realGit monotonicity matrix **33/33 PASS**, zero API/CLI drops. Original33 path sets and assertion harness unchanged. Transport adaptation only: old CLI/API receives the frozen88d literal registry; new receives its case registry, because the old parser cannot parse the new syntax. Both expose the same prior mode universe. Full outputs and commands under monotonicity/.

## Current gates

| Command | Exit | Evidence |
|---|---:|---|
| node --test tests/ci/pr-modes.test.mjs tests/ci/mode-registry.test.mjs |0|registry-arm-green.log,28tests|
| node --test --experimental-strip-types tests/admin/meta-health-model.test.ts tests/admin/product-media-ui-model.test.ts tests/admin/picklist-model.test.ts |0|consumer-tests.log,44tests|
| bash scripts/dev/test-node.sh |0|node-accepted.json/log,1159tests|
| bash scripts/dev/check-gates.sh |0|check-gates-accepted.json/log,82modes all documented|
| independent Bash oracle +metadata |0|RECEIPT-v4.json,120+85cases|
| realGit compare.py +node --test monotonicity.test.mjs |0|monotonicity/replay.json,33/33|

E3 here is command-boundary MOCK equivalence and static/Node gates, not real fixture/browser acceptance. The oracle tools are retained exactly as executed; their interpreter/shebang is this Mac's /opt/homebrew/bin/python3, not a claimed portable CI harness. Fixtures are task-owned, commands are stubbed, no realcredentials/provider/network are used. Raw synthetic key-shaped values are hashed in committed evidence; raw-to-redacted hashes retained.

## Next green checkpoint

NOT_RUN at this implementation checkpoint: real backend-only --buyer-http; real --browser-picklist; real --browser-click-sweep@1/10 (canonical LC_SWEEP_SHARD=1/10 invocation). These run serially after this commit. Full foundation/all browser modes/providerSANDBOX/LIVE/deployment remain NOT_RUN. Author commits; integrator obtains cross-review, opens PR and pushes.
