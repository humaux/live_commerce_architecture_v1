# matrix-ledger-enforce — PR28 round-4

- Branch: `unit/matrix-ledger-enforce`; starting head `c81533c26a067422141dcb9d3a0f775d09078027`.
- `git fetch origin && git merge origin/r3/integration` → exit 0, already up to date at merged trunk `1200178c5ffe3decaee09077e2bb8b7f59ea715c`.
- Tested source: **`2d9edcab7392cd886af975979f5c551745accfbc`**, recorded in [SOURCE.json](round4/SOURCE.json). Eleven unit executable producers are checkout-bound; twelve shared context hashes remain informational. The final handoff commit changes receipts/documentation only.
- Author: Codex-4 `/root`, exact parent runtime variant/effort not exposed. Isolated JS worker: `test_worker`, `gpt-6.1-sol`, medium, detached worktree `.worktrees/pr28-r4-js-proof`. Read-only execution/source reviewer: `explorer`, `gpt-6.1-sol`, medium. [Task/worker proof](round4/PROOF.json).
- Scope: matrix evidence/outcome and execution gates. Application, fixture, spec, waits and browser assertions are unchanged. Foreign `output/ext-agents/` is excluded.

## Fixes and red → green

P1-a: byte/hash integrity did not establish a passing run. After all existing hashes and exact ledger identities validate, both native final receipts must explicitly say `passed` with `failedTests=[]`; the PE/CC12 logs must start and finish exactly 12/2 cases with zero failed/skipped/flaky/interrupted/not-run outcomes. Manifest passed counts, skipped=0, raw rows, exact required PASS count and per-locale totals must agree with the native results and validated ledger. The logical outcome/fixture module keeps each Go file below 600 lines.

[Outcome RED](round4/outcome-red.json) → exit 1: all 13 initial consistently rehashed corruptions were accepted by the old gate. [Current-source GREEN](round4/outcome-final-green.json) → exit 0: 17 rehashed outcome controls reject semantic corruption; six original byte/source corruptions still reject and ten informational shared-source changes still pass. Exact command: `go test ./tests/foundation -run '^TestProductEditorMatrixLedgerEvidence(RunOutcome|SharedSources|Corruption)$' -count=1 -v`. The extra controls retain 12 passing cases while appending a failure/skip/flaky outcome, or remove the completion summary. Both native copy hashes and the external manifest index are refreshed in each isolated corruption fixture, so a SHA mismatch cannot satisfy these tests.

P1-b: existing `check-gates` coverage mistook the Node filename in a Go source-hash list for execution. [Node-removal before](round4/binding-before-red.json) and [real CI-skip insertion before](round4/go-binding-before-red.json) both incorrectly exited 0 (acceptance RED=1). The new independent execution check requires active `node --test` argv entries, the actual balanced CI planner dispatch and real foundation selector forwarding; shared runner hashes remain informational. On the same real mutations [Node removal now](round4/binding-after-red.json) and [CI-skip insertion now](round4/go-binding-after-red.json), `bash scripts/dev/check-gates.sh` exits 1 with the specific binding error; originals were restored byte-for-byte. [Binding unit RED](round4/binding-unit-red.json) → 1; [GREEN](round4/binding-unit-green.json) → 0, six tests.

P2: unary signatures now retain prefix/postfix operators, distinguishing `!/~ /+`, `++/--` and postfix updates while preserving harmless parentheses. Presence of `CI` forbids either calibration hook, including empty values, before source mutation/snapshot or collection. [Isolated JS RED](round4/js-proof/red.log) → 1 (12 failures); [GREEN](round4/js-proof/green.log) → 0 (12 passed); [parent independent focused run](round4/js-parent-green.json) → 0. Genuine red/green logs are separate from the honest initial child-harness errors/recursive run, which are preserved as history. [Baseline source](round4/baseline-claims.source.txt) is inert, not a runnable snapshot.

## Current actual gates

Original full browser mode: 12 PE/review + 2 frozen CC12 cases, zero skips; both last-run receipts are passed with empty failedTests. Matrix ledger: 286 raw rows, 231 unique required PASS identities, 77 per locale (`zh-TW`, `zh-CN`, `en`) at 390px. All frozen unit/context hashes are unchanged before/after these runs. [Canonical manifest](round4/browser/browser-evidence.json); five native files are copied byte-for-byte, with the external [artifact index](round4/ARTIFACTS.json). No screenshot claim is made.

| Command | Exit | Evidence |
| --- | --- | --- |
| `bash scripts/dev/test-local.sh --browser-product-editor` | 0 | [receipt](round4/browser-run.json) |
| `bash scripts/dev/test-node.sh` | 0 | [receipt](round4/test-node.json) |
| `pnpm typecheck:admin` | 0 | [receipt](round4/typecheck.json) |
| `go vet -tags browser ./tests/foundation` | 0 | [receipt](round4/go-vet.json) |
| `bash scripts/dev/check-gates.sh` | 0 | [frozen-source receipt](round4/check-gates.json), [final staged-artifact receipt](round4/staged-check-gates.json) |
| `go test ./tests/foundation -run '^TestProductEditorMatrixLedger' -count=1 -v` | 0 | [receipt](round4/go-ledger.json), exact ledger, actual canonical evidence, corruption/shared/outcome controls |

E3 actual local acceptance and independent deterministic artifact acceptance passed. [Independent receipt](round4/audit/command-receipt.json): 33 checks verify the 64-entry index as audited, all five unchanged native copies, 11 hard source hashes, exact outcomes and required PASS identities. The actual independent command `go test ./tests/foundation -run '^TestProductEditorMatrixLedgerEvidence($|RunOutcome$)' -count=1 -v` exited 0. This does not claim an independent full browser/PG run or new external review/CI approval. The final handoff commit contains evidence/documentation only. After the audit receipts and metadata were staged, the same actual evidence/outcome Go target exited 0 again ([final receipt](round4/final-evidence.json)); final staged `check-gates` also exited 0. `check-gates` documents all 83 modes and reports contract drift errors=0.

## Limits and history

- MOCK identity/isolated REAL_PG/browser execution only; no provider, production, deployment, PR merge or thread-resolution acceptance.
- CI and a new external independent review: NOT_RUN in this local batch. The read-only source audit (`de8791f4-0dc5-465e-973a-ce8916253d8e`) is E1, not K3/CI approval.
- `test-node.sh`: optional media `r04-input-runner` is honestly NOT_RUN because the pinned binary is unset; the required Node suites pass. Structural test titles marked Go NOT_RUN do not claim a runtime gate.
- One intermediate outcome fixture build error and initial JS child-harness failures are retained; they are not counted as successful red acceptance evidence.
- Accepted FOLLOWUPS remain unchanged: source-list derivation/per-path expansion, self-attesting receipts and merge-ref product-context drift. Execution content checks support the current bounded runner shapes, not arbitrary shell control flow; actual required CI browser execution remains necessary.
- Earlier `merge-ref/`, round-1 and round-2 runs describe their own historical source; they do not prove this changed source. Only `round4/` is designated canonical here.
