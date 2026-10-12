# matrix-ledger-enforce PR #28 postcondition P1 delivery

- Branch/worktree: `unit/matrix-ledger-enforce`, `.worktrees/matrix-ledger-enforce`. Pull from the unit remote was a no-op at integrator merge **`da5a28b38a0dff16c88738a1e3615ca98d24d621`**; both test-node lists remain.
- Source commits: `8bfedec7e689192fa3d2e80f8e495c55c0f944e9` (gate and native UI assertion), `6533cb5d5064d27123eaeaac481e123aa26946f0` (bind native acceptance to the existing save ACK). Tested frozen source: **6533cb5d**; final packaging changes only evidence/docs.
- Author Codex-4; actual parent model/effort unexposed. Read-only test_worker requested GPT-6.1-sol/medium owns no code paths. Task `5a52a61e-f8b1-48e0-852f-47edfbb499f6`; review thread **4246050140**. Commit only; integrator pushes/replies/resolves.

## Fix and boundaries

The existing gate certified an operated receiver/method/key without requiring a user-visible result assertion. `tests/ci/product-editor-matrix-ledger.test.mjs` now retains the original lexical locator/DOM-read identity checks and traces each callback's **action → direct awaited UI matcher → next action or own observed return**. Each operated action must complete that sequence. Parent/child steps cannot lend each other an assertion; an unused closure, branch-only assertion, precondition, unawaited matcher, assertion after return, or later uncheck outcome cannot certify an earlier check.

The original native confirmation callback was the only real source action without a postcondition: it checked the prompt before `dialog.accept()` and immediately returned a PASS observation. `tests/admin/product-editor.acceptance.ts` now awaits the **same existing `axisSaved` promise**, checks its successful ACK, and asserts the visible Save button is enabled before that receipt. All original later assertions, waits, timeout budgets, selectors and the 77-row case table remain. No product behavior, API, Go, SQL, migration, dependencies or registry layout changed.

This static gate proves supported matcher **presence and execution order**, plus existing action identity. It does not automatically prove semantic equivalence of every matcher receiver/expected value to the action. That is a P2 follow-up (`4279838f`), outside this converged P1 batch; the real browser run executes the assertions. Direct awaited matcher syntax is deliberately required; unsupported helper/branch-only proof shapes fail closed.

## Red → green evidence

`round2-postconditions/red-tests.log` records **7 failed tests, exit 1** against the old policy before implementing the guard. The actual source then also exposed the native accept omission. Transient receiver/read-slice implementation diagnostics are retained explicitly as diagnostics, never acceptance.

The completed suite has **32 PASS / 0 FAIL / 0 SKIP**, including nine counterexamples: removed, pre-action, after-return, unawaited, unused-function, nested-step, unreachable-branch, missing intermediate checked-state, and missing native outcome. The exact mutated source copy is `round2-postconditions/removed-postcondition.source.txt`; it is a byte-bound forensic snapshot, not a registered runnable test.

[commands.json](round2-postconditions/commands.json) binds current source hashes and exact injection/restoration commands. Removing only the mobile product-name outcome leaves its action, synchronous write-count assertion and `{ observed }` receipt intact:

| Exact command | Exit | Evidence |
| --- | ---: | --- |
| `LC_MATRIX_ACTION_MUTATION=removed-postcondition node --test '--test-name-pattern=every actual mobile' tests/ci/product-editor-matrix-ledger.test.mjs` | **1** | `round2-postconditions/injected-copy.log`; missing-outcome rejection |
| `node --test '--test-name-pattern=every actual mobile' tests/ci/product-editor-matrix-ledger.test.mjs` | **0** | `round2-postconditions/restored-copy.log`; original assertions restored |
| `node --test tests/ci/product-editor-matrix-ledger.test.mjs` | **0** | `round2-postconditions/green-focused.log`; 32 tests and real Playwright ESM collection seam |

Independent read-only focused rerun also returned 32/0/0 (`4279838f`); it preceded the extra save-ACK await and is limited to that recorded source. It is not a new K3/CI approval.

## Current native evidence and gates

The original full product-editor mode ran on frozen **6533cb5d**, with no source/hash changes, grep, calibration or relaxed wait. It passed **12 PE/review tests and two mandatory CC12 tests**. The real Go harness validates **231 unique required PASS rows: 77 for each zh-TW/zh-CN/en at 390px**. The raw native ledger has 286 total rows including unrelated suite entries.

Canonical fresh [ledger](browser/product-editor-click-ledger.json), [Playwright log](browser/playwright.log), and [browser manifest](browser/browser-evidence.json) are committed. Their copied bytes and tested source are hash-bound; [SOURCE](round2-postconditions/SOURCE.json) binds 13 source files. The native evidence root is the actual `output/playwright/pr28-postconditions-20261012T0903/catalog-core/20261012T090306.000097000`; mandatory CC12 uses its separate same-mode native directory. No screenshots are a delivery claim. Raw artifact screenshot names remain auxiliary runner paths.

Exact argv/env, timeout, PID, UTC timestamps, exits and log SHA256 are in [browser-run.json](round2-postconditions/browser-run.json) and [local-gates.json](round2-postconditions/local-gates.json):

| Command | Exit | Result |
| --- | ---: | --- |
| `LC_TEST_LOCK_WAIT=1800 LC_BROWSER_ENGINE=chromium LC_BROWSER_EVIDENCE_ROOT="$PWD/output/playwright/pr28-postconditions-20261012T0903" bash scripts/dev/test-local.sh --browser-product-editor` | 0 | `round2-postconditions/browser-product-editor.log`; real production Next/BFF/Go/isolated PG, signed synthetic MOCK identity; 231 required ledger PASS |
| `bash scripts/dev/test-node.sh` | 0 | `round2-postconditions/test-node.log`; final source suites green |
| `go test ./tests/foundation -run '^TestProductEditorMatrixLedger' -count=1 -v` | 0 | `round2-postconditions/ledger-go.log`; both DB-free ledger/evidence tests, corruption controls, no unrelated PG skips |
| `pnpm typecheck:admin` | 0 | `round2-postconditions/typecheck.log`; strict admin TypeScript |
| `bash scripts/dev/check-gates.sh` | 0 | `round2-postconditions/check-gates.log`; final source modes documented, test registration, headers and static ratchets green |

Prior `25bb988f`, pre-ACK `8bfedec7`, and their round1/round2/before-ack records are **historical**, not current-source acceptance. Previous canonical browser bytes are preserved under `round2-postconditions/historical-browser/`; its old manifest retains the original source identity. Current browser manifest supersedes those claims without rewriting past runs.

## Evidence level / CI handoff

**E3: complete in the automated tested environment**, bound to source6533 and artifact hashes. Provider SANDBOX/LIVE, deployment, full foundation, other selected browser modes, and current-head remote CI/K3 are **NOT_RUN** here. CI gates needed: foundation-shards/test-node/check-gates and the current PR planner's affected browser set, including `--browser-product-editor`. The integrator's previously green catalogue-core run at da5 is historical; this batch reruns the requested product-editor mode with mandatory CC12.

No real money/messages/keys/buyer PII or production access. Owned browser/Next/PG fixtures and runner exited; browser mutex released. Foreign `output/ext-agents/`, other worktrees/processes and shared caches were not changed or staged. No push.
