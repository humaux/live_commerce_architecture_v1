# Single test-local mode registry — final local delivery

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

## Real representative modes — final GREEN

All three ran serially on immutable **fd5566d61b1d5b047bddd95fc53111fffe941b92**, after implementation commit **f7b4f5a0e3231cdaeb8f8abc0614005d7d5bc3fc**. Each result JSON pins the same runner/runtime/selector hashes and verifies they remained unchanged. No new source changes followed these runs; this final commit adds delivery evidence only.

| Exact command | Exit | Result / evidence under real/ |
|---|---:|---|
| `bash scripts/dev/test-local.sh --buyer-http` | **0** | REAL_PG backend-only subset; buyer-http.json/log |
| `bash scripts/dev/test-local.sh --browser-picklist` | **0** | Real browser selections/print/CSV/confirmation/UNKNOWN/scope; MOCK upstream; picklist.json/log |
| `LC_SWEEP_SHARD=1/10 bash scripts/dev/test-local.sh --browser-click-sweep` | **0** | Canonical CI invocation of --browser-click-sweep@1/10; click-shard-1-of-10.json/log |
| `pnpm --filter @live-commerce/admin exec tsc --noEmit` | **0** | typecheck.log |
| `bash scripts/dev/check-gates.sh` after evidence packaging | **0** | check-gates-checkpoint-green.log;82 modes documented |

The click-sweep slice reports8 page/viewport/locale units,0 page-load failures,86 control entries (77PASS/0FAIL/9SKIP under the unchanged gate rules), and10 journey steps (10PASS/0FAIL). Shard index1/of10, complete=true,0 known/new failures and0 stale defect entries are retained in the **current** ledger dated2026-10-08, copied from the run backup before tracked baseline artifacts were restored. It proves this slice only, not all10 shards or a global sweep aggregate.

Root independently recomputed the archived before/after command, environment, exit, utility and artifact comparisons for all120 oracle cases; root-equivalence-audit.json binds the archive hash and confirms zero differences. The independent source/metadata review PASS is separate from the actual runtime receipts above.

**E3 local acceptance:** the requested static, selector, command-equivalence and three representative runtime gates are green. Full foundation, all remaining browser modes/other9 sweep shards, full sweep aggregate, GitHub/Linux runtime, physical Safari and provider SANDBOX/LIVE/deployment remain **NOT_RUN**. Test processes/own PG fixture finished normally; own generated tracked screenshots/ledgers were backed up then restored; transient gate output was moved into primary real/generated-untracked. No other worker output/cache was removed. No push; integrator owns cross-review/opening the PR.

Evidence packaging: the original replay scripts are retained losslessly as `.raw.gz` and normalized readable `.txt` views, not tracked `*.test.mjs` files falsely registered as live repo tests. The executed scripts in primary evidence retain their original filenames and byte equality. The initial checkpoint whitespace check reported only two trailing-blank-line findings in these copied scripts; their views now pass without changing the executed originals.
The packaging check correctly rejected evidence names still containing `.test.` even with a `.txt`/`.gz` suffix; the archived assertion script now uses `monotonicity-assertions` filenames. No gate rule was relaxed.
