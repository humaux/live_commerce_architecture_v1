<!-- Purpose: Record MATRIX-COVERAGE-V2 GROUP3 implementation and immutable evidence.
Depends on: assigned foundation validators and DB-free receipts.
Used by: parent integrator and independent reviewer. -->
# MATRIX-COVERAGE-V2 GROUP3 delivery

- task_id: `f597a8bc-ce29-4b68-ab31-815dd5a319cb` (parent); subcanvas `codex4-matrix-v2-sub-evidence`.
- base_commit: `0bfe36e53acb694f4697521202e24f0790638941`; branch `unit/matrix-coverage-v2-evidence`; worktree `.worktrees/matrix-coverage-v2-evidence`.
- Author: `/root/matrix_v2_evidence`, implementation/test role; inherited actual model/reasoning **UNKNOWN** (not exposed). No delegation, push or PR.
- Changed paths: `tests/foundation/catalog_matrix_ledger_test.go`, `tests/foundation/catalog_matrix_evidence_test.go`, focused strict-delivery section of `docs/delivery/GATES.md`, `output/matrix-coverage-v2-evidence/**`.
- Contract/dependency/migration changes: none. The 14-entry producer declaration and current live-browser exact-ledger interface stay unchanged.

The root cause was the typed drift return preceding native copy, required-file and outcome checks, while artifact hashing covered only the browser subtree. The validator now checks every artifact-index path/byte/hash inside the canonical package, all copy/index memberships, all five required native files and native outcome coherence before classifying source drift. All recorded source digests must remain valid; an absent newly introduced producer is historical only if the receipt-bound gate declaration source differs from the current checkout. An unchanged declaration missing a producer remains malformed.

Historical ledgers retain schema, unique identity, nonempty claim, PASS, locale/390px and receipt/count validation without requiring a future table or Git history. Matching sources retain the exact current table identity/expected-claim gate. Immutable corruption remains fatal under strict and default policies. Isolated mutation fixtures rebind the sole current source table and explicitly synthesize a **MODEL_ONLY** ledger/counts from its expected keys; they never modify or establish delivered canonical browser evidence. `TestProductEditorMatrixLedger` still independently checks the recorded golden projection.

## Actual commands and receipts

The runner [run.py](run.py) bounds each command, preserves full logs and binds all three edited source files before/after; every recorded run has unchanged source hashes. All focused commands use `-count=1 -v`.

| Command | Exit | Evidence |
| --- | ---: | --- |
| `go test ./tests/foundation -run '^TestProductEditorMatrixLedgerEvidenceDriftIntegrity$' -count=1 -v` | 1 (RED) | [immutable-red.json](immutable-red.json), [log](immutable-red.log): 11 old bypass failures, 2 pre-existing rejection controls |
| `go test ./tests/foundation -run '^TestProductEditorMatrixLedgerEvidenceProducerEvolution$' -count=1 -v` | 1 (RED) | [producer-red.json](producer-red.json), [log](producer-red.log): changed declaration/new producer wrongly hard-failed |
| `go test ./tests/foundation -run '^TestProductEditorMatrixLedgerEvidenceTableEvolution$' -count=1 -v` | 0 (compatibility precheck) | [table-red.json](table-red.json), [log](table-red.log): runner name retained; this was already green and is not claimed as a red |
| `go test ./tests/foundation -run '^TestProductEditorMatrixLedgerEvidenceHistoricalPolicy$' -count=1 -v` | 1 (RED) | [policy-red.json](policy-red.json), [log](policy-red.log): original validator with only run-outcome signature adapter; all 6 new deleted/copy/index strict/default child controls reject the old skip behavior |
| `go test ./tests/foundation -run '^TestProductEditorMatrix' -count=1 -v` | 1 (first repair) | [focused-green.json](focused-green.json), [log](focused-green.log): incorrectly required nonempty row, but page-level controls legitimately have empty row; corrected schema, preserved failure |
| same focused command after schema correction | 0 | [focused-final.json](focused-final.json), [log](focused-final.log) |
| `go test ./tests/foundation -run '^TestProductEditorMatrix' -count=1 -v` after MODEL_ONLY fixture generalization | 0 | **final source** [focused-model-final.json](focused-model-final.json), [uncompressed log](focused-model-final.log) |
| `LC_MATRIX_EVIDENCE_STRICT=1 go test ./tests/foundation -run '^TestProductEditorMatrixLedgerEvidence$' -count=1 -v` | 1 (expected historical negative) | [strict-historical.json](strict-historical.json), [log](strict-historical.log): immutable original package validates, current edited sources drift; not current-source delivery acceptance |
| `git diff --check` | 0 | [diff-check.txt](diff-check.txt) |

Final ledger/evidence/GATES SHA-256 values are bound in `focused-model-final.json`; its checkout base SHA is recorded with dirty-source content hashes, not asserted as a pristine tested commit. E3 evidence proves the filesystem validator and test-entrypoint behavior in the tested DB-free environment. It does not prove current UI clicks, provider, production or independent acceptance.

## Delivery checklist and remaining work

`GATES.md` documents the required fresh delivery sequence: freeze executable sources; run original full `--browser-product-editor`; package unchanged native bytes/receipts; execute exactly:

```sh
LC_MATRIX_EVIDENCE_STRICT=1 go test ./tests/foundation -run '^TestProductEditorMatrixLedgerEvidence$' -count=1 -v
```

- `bash scripts/dev/test-node.sh`: **NOT_RUN**, parent ordered HOLD until a serialized broad-gate slot; unrelated child deadlines occurred under competing agents.
- `bash scripts/dev/check-gates.sh`: **NOT_RUN**, same serialized slot requirement.
- Current full browser/PG, CI and independent review: **NOT_RUN** here; parent owns serial resources, push and review.
- Strict current-source canonical delivery: **NOT_RUN**; old canonical remains historical, unmodified. The strict historical failure is expected and is not PASS.
- Historical old expected claims are not compared to a future current table during drift; immutable schema/outcomes remain checked. A historical SKIP cannot certify current controls.
- No owned test fixture/process remains after these commands. All failure and red evidence is retained; no foreign files were cleaned.
