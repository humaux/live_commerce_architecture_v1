<!-- Purpose: Record issue45 GROUP2 actual source-bound gates and historical attempts.
Depends on: execution parser and existing Node/CI registries.
Used by: integrator and independent review; only observed gate results are acceptance. -->
# MATRIX-COVERAGE-V2 GROUP2 delivery

- Current task: PR46 positive execution receipts. task_id `f597a8bc-ce29-4b68-ab31-815dd5a319cb`; base_commit `f5339cbfbac632473c0bdb2c4823b557689f0fb7`; dependency commit `8ab590779d9ed0e22c5f39031143e3e93dcf893a`.
- Branch/worktree: `unit/matrix-coverage-v2-execution`, `/Volumes/data/live_commerce_architecture_v1/.worktrees/matrix-coverage-v2-execution`. `pr46-source-hashes.json` records the final precommit working-tree file hashes; its `precommit_parent` is `dc2304d2a8dab20941ec7d09fc2bb54e2f5b8360` and the implementation commit is `20c5c1a9b9b93d7e1cfe14c5aede3f7f9ffb7bdb`.
- Current result: source edits and evidence inventory are prepared. Latest local focused tests and all local runtime/static gates are `NOT_RUN` after owner restricted test execution to GitHub CI. Prior local test receipts below are historical and do not validate this source snapshot. Current acceptance depends on root-run CI and independent review.
- Current implementation: positive Node/Go execution receipts bind exact actual outcomes to the workflow run, source, mode and shard; independent verification re-reads raw events. A source-drift-only historical Go skip requires a same-run verified browser job with nested PE12-17-TWD and CC12-frozen passes. The `required` job checks every result consumer, with only artifact download allowed to continue on error, and manual partial runs receive a separate check name.
- Source and runtime risks: CI source-bound inventory / receipt correctness is pending GitHub CI; browser/REAL_PG, provider SANDBOX/LIVE and production operations are `NOT_RUN`. No external credentials or live actions were used.

## Historical issue45/earlier PR46 receipts (superseded; retained for audit only)

The following commands and outcomes belong to earlier source snapshots. They are not evidence for the current PR46 source tree.

- Prior branch base, paths, tests and source hashes below are historical. The current complete changed-path list and current-snapshot evidence are recorded in “PR46 positive execution receipts” near the end of this file.

## Actual commands and evidence

- pnpm install --frozen-lockfile --ignore-scripts → exit 0: install.log/install.exit (no lockfile change).
- node --test tests/ci/product-editor-matrix-execution.test.mjs → RED exit 1, pass32/fail83: red.log/red.exit, unchanged base implementation. Initial test-authoring SyntaxError is separately retained in test-authoring-syntax.log/.exit and is not the RED proof.
- Same focused command with two added reachability tests → RED exit 1, pass115/fail2: red-reachability.log/.exit.
- Same focused command → final GREEN exit 0, pass117/fail0: green.log/.exit, bound by historical-source-hashes.json.
- Candidate Go executable-prefix regression → focused exit 1, pass110/fail7: candidate-go-prefix-regression.log/.exit. Corrected by reusing assignment-prefix removal for actual GOTOOLCHAIN-prefixed Go command.
- First broad Node attempt edited during execution: invalid for acceptance, stopped exact owned PIDs; test-node-invalid-source-change.log/.json/.exit (child -15, shell 241).
- Premature broad Node start while focused regression was red: stopped exact owned PIDs; test-node-premature-focus-red.log/.json/.exit (child -15, shell 241). Not acceptance.
- bash scripts/dev/test-node.sh → fixed-source exit 1 in155.09s: test-node.log/.json/.exit, unchanged=true. Failures: next-startup stolen-port fixture timed out after10000ms; three playwright-deps installer fixtures timed out/status null or zero retries. GROUP2 remains117/117. No failing test or threshold changed.
- bash scripts/dev/check-gates.sh → exit 1 in611.30s; matrix-execution passed, unrelated TestReviewR1bImportedSinkValuesRealCLI/alias hit CLI deadline then contractdrift package timed out after10m. check-gates.log/.json/.exit; unchanged=true. Both original commands passed in serialized retry; initial failures remain retained.
- git diff --check → exit 0.
- Resource snapshot: resource-observation.log (load25.78/30.90/29.03; command names only).

## Evidence and outstanding work

- E3 for focused Node mutation gate on exact historical-source-hashes.json; author self-check only. Independent review: NOT_RUN by this author.
- Initial full Node/check-gates failed outside this task module; original commands passed in the parent-assigned serialized retry. Independent acceptance remains pending. Full browser/REAL_PG/full foundation/runtime/provider/SANDBOX/LIVE gates: NOT_RUN by this author, per assigned scope and shared resource ownership.
- Production/provider actions: none. No external credentials required; BLOCKED_EXTERNAL: none.
- Integrator to-do: independent review/re-run, serialized Node retry and canonical browser acceptance/CI as appropriate; push and open focused PR. No author push/PR/merge.

Final source hashes: implementation `b39b4377c1f1e9592ab351cfc500279f8eec2482f38764d1702663eeaf072811`; tests `1ee16f0d106bde822990826222ed29a2e3c663cb3182da9a314f7a664c01249e`. No source changes after focused GREEN.

## PR46 positive execution receipts (current worktree iteration)

- task_id: `f597a8bc-ce29-4b68-ab31-815dd5a319cb`; base_commit: `f5339cbfbac632473c0bdb2c4823b557689f0fb7`; dependency commit: `8ab590779d9ed0e22c5f39031143e3e93dcf893a`.
- Source snapshot before commit: `dc2304d2a8dab20941ec7d09fc2bb54e2f5b8360`; final commit SHA is reported by the owner after commit.
- Changed paths: `.github/workflows/foundation.yml`, `.github/workflows/gates.yml`, `scripts/dev/ci-plan.mjs`, `scripts/dev/test-local.sh`, `scripts/dev/test-node.sh`, `scripts/dev/matrix-receipt-verify.mjs`, `scripts/dev/matrix-receipt.mjs`, `scripts/dev/matrix-test-inventory.json`, `scripts/dev/matrix-test-reporter.mjs`, `tests/ci/ci-plan.test.mjs`, `tests/ci/matrix-receipt.test.mjs`, `tests/ci/pr-modes.test.mjs`, `tests/ci/product-editor-matrix-execution.mjs`, `tests/ci/product-editor-matrix-execution.test.mjs`, `tests/foundation/catalog_matrix_ledger_test.go`, and this output directory.
- Implementation: Node and Go runner receipts capture actual test events and bind source, workflow mode, run ID, shard and command. The required verifier recomputes from raw event files and checks exact Node identities, the dynamically discovered Go family, and the narrowly allowed historical ledger-evidence skip against actual `SOURCE.json` drift. A historical skip requires the same-run `--browser-product-editor` job with passing parent, nested PE12-17-TWD and CC12-frozen events. The planner adds that browser mode only when the bound source facts show real drift. The required workflow job uses a distinct `manual` check name for partial workflow-dispatch runs. Its PLAN/GATE/AGG/DEPLOY result consumer and receipt verifier are mandatory; only the artifact-download step may continue on error, since missing evidence independently fails both consumers.
- Latest static review fix: the execution parser inspects every step in the `required` job for `continue-on-error`; only the artifact-download action may retain the approved `true`. The mutation test covers an unrelated setup step, the result aggregator as both ordinary and first-key YAML forms, the receipt verifier, and job scope. This edit has no post-edit local test result by owner directive.
- Focused Node tests, full Node, check-gates, Go, and browser execution after the latest source edits: `NOT_RUN` by this worker under the owner's instruction that GitHub CI is the only test runner. Earlier local green logs in this directory predate the current final source snapshot and are not evidence for it.
- Actual non-test commands in this iteration: `git status --short && git rev-parse HEAD && git diff --stat ...` → exit 0; two `shasum -a 256 ...` source-hash commands → exit 0; `git apply_patch` source/evidence edits completed. No tests or runtime gates were started after the stop instruction.
- Current source hash manifest: `pr46-source-hashes.json` (source files are hashed; `precommit_parent` identifies the parent of the precommit source snapshot). Focused historical red/green and receipt fixtures remain under `probes/`; their older green receipts do not cover the latest final source edits.
- Evidence level: E1 source/inventory/hash binding only for this latest iteration. E3 current-source test and CI receipts: `NOT_RUN`; root owns GitHub CI, independent review and any serialized browser acceptance. No production/provider action was performed.

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

## GitHub-only validation boundary

Owner directive on 2026-10-13 requires all tests and runs on GitHub CI. Current implementation is `20c5c1a9b9b93d7e1cfe14c5aede3f7f9ffb7bdb`; source has not been executed locally after this directive. CI run IDs and actual receipts will be added after the push. The exploratory reporter source is retained as inert `probes/custom-reporter.source.txt`, so it cannot masquerade as a newly registered runnable suite. In-test negative controls cover filtered/missing Node cases, Go listing, missing receipts after suppressed failures, exact historical skip conditions, wrong-run browser proof, and mandatory result-consumer failure propagation. The previous successful runs `38363915256` and `38363915641` belong to the old static-only head `f5339cbf`, not this redesign.

PR46 CI red: Foundation `38379329051`, Gates `38379329348` at `226ee182`. Actual Node receipt recorded 223 PASS / 1 FAIL: the continuation mutation targeted the first setup-node outside `required`; it now targets the actual required job. The source inventory omitted two parameterized names; it now includes all 224 actual registered cases, with hashes refreshed on current bytes. Required receipt discovery excludes unrelated manifests/empty exploratory JSON; its injected irrelevant-JSON control executes in CI. Original raw failure events: `ci-red-38379329348/node.json.events`. `source-hashes.json` is a current source-only record; original local receipts remain bound to `historical-source-hashes.json`. No local test was executed. Fix CI pending.

Independent E1 static check identified that a commented shell result predicate could remain as text. The pinned receipt action now independently checks PLAN/GATE/AGG/DEPLOY outcomes in JavaScript even if that shell check is commented; an in-test control executes the real action with each failed outcome and a passing receipt child. Foundation uploads raw current-run receipts even on failure. These changes await CI; no local tests/checks ran.

CI Foundation `38380816525` at source `658613a06a171d3d1cc85e7283f8989a2b19fe3c` **PASS**, including Node suites and check-gates. Complete Gates `38380816987` remains pending, so no runtime E3 closure is claimed from Foundation alone. Static cross-PR review found that the independent drift parser must also classify a genuinely removed SOURCE-bound producer as historical drift, matching PR47 Go policy. It now checks lexical paths and every symlink segment, accepts only ENOENT as drift, and rejects dangling links/directories/invalid digests. CI injected fixtures cover matching-source, deleted-source, unsafe paths, dangling links and directories without modifying checkout sources. This final alignment needs its own CI run; no local tests ran.

CI Foundation `38382999786` at `754f7753` failed one regression cleanup: after correctly refusing a directory as a bound producer, the fixture attempted non-recursive removal of that directory. The cleanup now removes that fixture directory recursively; every refusal and exact historical drift assertion remains unchanged. The raw RED log and earlier Foundation PASS `38380816525` are preserved losslessly with SHA-256/byte counts in `ci-raw-log-copies.json`. Fix acceptance awaits GitHub CI; no local test/check was run.

## Current Foundation CI result (complete Gates pending)

GitHub Foundation [38384409283](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38384409283) at branch head `e5bffe018a5309c8df7da8f565cf715b158dfcce` is **PASS**. The raw downloaded log is preserved losslessly as [ci-green-38384409283.log.gz](ci-green-38384409283.log.gz), with raw/archive SHA-256 and byte count in [ci-raw-log-copies.json](ci-raw-log-copies.json). Actual job/step metadata is in [run.json](ci-green-38384409283/run.json) and [receipt.json](ci-green-38384409283/receipt.json). No local test/check/formatter was executed.

Foundation ran the full Node suites and `bash scripts/dev/check-gates.sh`, both exit0, and the mandatory Matrix receipt verifier passed. The actual [Node receipt](ci-green-38384409283/node.json) and [raw reporter events](ci-green-38384409283/node.json.events) bind the three Matrix suites and all224 actual passing identities to this run; the checkout is CI merge commit `7bdc0f15615128232be975001ff6d8c98803cf9e`. In-test suppressed Node execution, Go-list, commented result-consumer and historical-skip negative controls passed. Complete Gates `38384409603` was canceled when the new mandatory-identity defect was found; no full-chain PASS is claimed. **E3 for Foundation only; not READY for the runtime chain.**

## Follow-up source and CI RED records

- Test-first `ef61b28d`: Foundation [38387315042](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38387315042) RED, exit1, on both mandatory canonical-evidence test omission controls. Fix `ff6a4177` keeps `TestProductEditorMatrixLedgerEvidence` mandatory in discovery and outcome parsing; both controls PASS in the next run.
- Test-first `dbf0b956` and `ef0c9175`: Foundation [38388813827](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38388813827) RED, exit1, on flexible valid Go test signatures and the new Foundation browser-verifier interface. Actual verifier-condition corruption controls already pass as in-test injected failures; no separate pre-fix RED is claimed for them. Failed raw logs and API metadata are retained.
- `e0adf231` adds a conditionally executed original `--browser-product-editor` Foundation step after independently verified historical Go skip. The always consumer requires same-run/current-source browser proof; full Go PASS requires no browser. The workflow budget is explicitly120minutes for existing4500s Go plus original1700s browser bounds; test waits and assertions are unchanged.
- All generated receipt probes now use an owned system temporary directory with `test.after` cleanup. Committed historical probe receipts are never modified by the tests. Root additionally accepts valid named/unnamed/multiline Go signatures and validates actual comment-free always conditions on existing receipt consumers.
- Policy deviation: the execution worker accidentally ran local `git diff --check` once (exit0), reported it immediately, and stopped all further local checks. It is not counted as acceptance; no local test suite, formatter, browser, server or database was run. Root validation remains GitHub-only.
- Current-source GitHub GREEN and the complete required Node/Go/browser receipt chain remain PENDING. Red-stage Gates38387315281/38388814040 were intentionally canceled, not passed.

Final follow-up source uses the PR47 dependency through24f64579 (including the historical frozen-claim/package/title validator) and the GitHub-supplied formatter patch. Current source-only manifests were refreshed after that merge. The exact required Matrix suite inventory is227 cases across3 files; Foundation receipt probes are temporary and historical committed probe files remain unchanged. GitHub validation of this final merged source is pending.

Foundation38390763488 at5b4c19a3 is RED on the existing historical-skip scheduling assertion: the verified historical_skip payload is a record, but the new detector compared it to boolean true. Root changes both scheduling and browser-receipt selection to check the independently validated record.ok === true. Every red assertion stays unchanged. The raw failure log is retained with digest receipt; no local tests/checks ran. Superseded Gates38390763776 is canceled rather than counted as PASS.

Foundation38391591681 atf754dd58 is RED on the next existing assertion: the missing-browser diagnostic must explicitly identify the same-run browser receipt. The verifier already rejects the skipped browser step correctly; its message now preserves that receipt requirement and the browser outcome. Tests and receipt validation are unchanged. Raw CI RED is retained losslessly with hashes; GitHub-only revalidation remains pending.

Foundation38392805174 atf9a57c52 is RED on the existing NODE_OPTIONS-cleared negative control. The historical-skip interface assertions now pass. Adding the browser-need detector introduced a second receipt invocation, which the unchanged mutation targets first; the static positive binding previously checked only the final consumer. The same clean Node wrapper is now required at every receipt invocation, preserving the original mutation and assertion. Actual reporter counted226 passes/one failure across3requiredsuitefiles and correctly failed the command and final consumer. RawRED retained; no localtest/check was run.

## Review receipt controls: tests-stage inventory correction

Tests-only `17373fe8` produced actual CI RED [38396674322](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38396674322), Node exit1. The manual foundation planner and required manual-foundation refusal reached the intended behavior. The new Gate-step receipt control was masked by a stale source-bound Node case inventory; this is a **setup RED**, not proof of that semantic bypass. The inventory now includes the one new execution-suite identity (155 total) and the exact committed test source hash. Assertions remain unchanged. A new GitHub CI run must demonstrate the isolated Gate-step causal RED before the implementation is committed. Native logs and API metadata are retained in [ci-red-38396674322](ci-red-38396674322/run.json). No local tests/checks/formatters were run.

Prior Foundation [38394271564](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38394271564) actually passed 227 required Matrix Node cases at `a24a98d0`; this predates the new tests and is not their acceptance. Its exact receipt is [ci-green-38394271564/receipt.json](ci-green-38394271564/receipt.json). Prior full Gates [38394272172](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38394272172) was superseded/canceled, with one unrelated promotions false-positive preserved in [ci-promotions-38394272172/receipt.json](ci-promotions-38394272172/receipt.json): the forbidden amount regex matched synthetic SKU `BRP-6cdb58d5e960` while the real amount stayed NT$1,060 and discount stayed zero. The promotions script is identical to trunk (blob `4564baa610682b7b693f5efb6f88ce095a21bf27`); no out-of-scope product/test change was made.

## Positive execution proof: implementation after causal GitHub RED

After inventory-only `98506095`, Foundation [38398298494](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38398298494) failed exactly the four intended behavioral assertions: manual foundation planning, omitted ordinary Gate-step evidence, missing Go race instrumentation, and manual foundation historical-skip bypass. The existing receipt positive fixtures passed. The omitted Gate receipt reached the old verifier's success message and exit0, establishing causal RED. [Native log/API and receipt](ci-red-38398298494/receipt.json) are retained.

Implementation `d768f45b` records the actual named Gate step outcome in an always-run capture step and requires one same-run/current-source successful record per selected mode. The verifier checks actual Go argv for active race instrumentation. Manual `foundation` produces exact Node and Go evidence; validated historical skips also require the original same-run product-editor browser proof. Regression assertions remain strict, and negative controls include wrong source/run, skipped Gate outcome and disabled race instrumentation. PR47 dependency `cf64271f` is merged to retain the independently pinned 64-member immutable evidence oracle and frozen-fixture behavior; PR47 Foundation acceptance is separate from this merged source.

Readonly WIP audit found no concrete P0/P1/data-integrity P2 (E1 only). Its initial Foundation-wrapper finding was retracted after locating the direct wrapper at test-node.sh:65; no source change was made for that false positive. Current GitHub GREEN/full receipt acceptance is **PENDING**. Root and this implementation worker ran no local tests/checks/formatters or application/PG runtime. Source hashes are refreshed in [pr46-source-hashes.json](pr46-source-hashes.json); source freeze precedes the next GitHub run.
