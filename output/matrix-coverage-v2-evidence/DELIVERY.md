<!-- Purpose: Record MATRIX-COVERAGE-V2 GROUP3 implementation and immutable evidence.
Depends on: assigned foundation validators and DB-free receipts.
Used by: parent integrator and independent reviewer. -->
# MATRIX-COVERAGE-V2 GROUP3 delivery

- task_id: `f597a8bc-ce29-4b68-ab31-815dd5a319cb` (parent); subcanvas `codex4-matrix-v2-sub-evidence`.
- base_commit: `0bfe36e53acb694f4697521202e24f0790638941`; branch `unit/matrix-coverage-v2-evidence`; worktree `.worktrees/matrix-coverage-v2-evidence`.
- Author: `/root/matrix_v2_evidence`, implementation/test role; inherited actual model/reasoning **UNKNOWN** (not exposed). No delegation, push or PR.
- Changed paths: `tests/foundation/catalog_matrix_ledger_test.go`, `tests/foundation/catalog_matrix_evidence_test.go`, focused strict-delivery section of `docs/delivery/GATES.md`, `output/matrix-coverage-v2-evidence/**`; root explicitly extended write_paths to `scripts/dev/shard-plan.json` for four g06 registrations, with integrator owning final JSON merge.
- Contract/dependency/migration changes: none. The 14-entry producer declaration and current live-browser exact-ledger interface stay unchanged.

The root cause was the typed drift return preceding native copy, required-file and outcome checks, while artifact hashing covered only the browser subtree. The validator now checks every artifact-index path/byte/hash inside the canonical package, all copy/index memberships, all five required native files and native outcome coherence before classifying source drift. All recorded source digests must remain valid; an absent newly introduced producer is historical only if the receipt-bound gate declaration source differs from the current checkout. An unchanged declaration missing a producer remains malformed. The follow-up P2 control also requires the independently indexed frozen `browser-product-editor.json` receipt to corroborate the recorded old producer set, tested/after commit, exact full-mode command and explicit exit 0 before drift. An unrelated gate comment cannot excuse removing a producer still present in the immutable run receipt.

Historical ledgers retain schema, unique identity, nonempty claim, PASS, locale/390px and receipt/count validation without requiring a future table or Git history. Matching sources retain the exact current table identity/expected-claim gate. Immutable corruption remains fatal under strict and default policies. Isolated mutation fixtures rebind the sole current source table and explicitly synthesize a **MODEL_ONLY** ledger/counts from its expected keys; they also explicitly rebind a copied run receipt marked `fixture_evidence_class=MODEL_ONLY`; they never modify or establish delivered canonical browser evidence. `TestProductEditorMatrixLedger` still independently checks the recorded golden projection.

## Actual commands and receipts

The runner [run.py](run.py) bounds each command, preserves full logs and binds the validator/docs sources before/after, and adds `scripts/dev/shard-plan.json` from the focused registration red onwards; every recorded run has unchanged source hashes. All focused commands use `-count=1 -v`.

| Command | Exit | Evidence |
| --- | ---: | --- |
| `go test ./tests/foundation -run '^TestProductEditorMatrixLedgerEvidenceDriftIntegrity$' -count=1 -v` | 1 (RED) | [immutable-red.json](immutable-red.json), [log](immutable-red.log): 11 old bypass failures, 2 pre-existing rejection controls |
| `go test ./tests/foundation -run '^TestProductEditorMatrixLedgerEvidenceProducerEvolution$' -count=1 -v` | 1 (RED) | [producer-red.json](producer-red.json), [log](producer-red.log): changed declaration/new producer wrongly hard-failed |
| `go test ./tests/foundation -run '^TestProductEditorMatrixLedgerEvidenceTableEvolution$' -count=1 -v` | 0 (compatibility precheck) | [table-red.json](table-red.json), [log](table-red.log): runner name retained; this was already green and is not claimed as a red |
| `go test ./tests/foundation -run '^TestProductEditorMatrixLedgerEvidenceHistoricalPolicy$' -count=1 -v` | 1 (RED) | [policy-red.json](policy-red.json), [log](policy-red.log): original validator with only run-outcome signature adapter; all 6 new deleted/copy/index strict/default child controls reject the old skip behavior |
| `go test ./tests/foundation -run '^TestProductEditorMatrix' -count=1 -v` | 1 (first repair) | [focused-green.json](focused-green.json), [log](focused-green.log): incorrectly required nonempty row, but page-level controls legitimately have empty row; corrected schema, preserved failure |
| same focused command after schema correction | 0 | [focused-final.json](focused-final.json), [log](focused-final.log) |
| `go test ./tests/foundation -run '^TestProductEditorMatrix' -count=1 -v` after MODEL_ONLY fixture generalization | 0 | prior implementation [focused-model-final.json](focused-model-final.json), [uncompressed log](focused-model-final.log) |
| `LC_MATRIX_EVIDENCE_STRICT=1 go test ./tests/foundation -run '^TestProductEditorMatrixLedgerEvidence$' -count=1 -v` | 1 (expected historical negative) | [strict-historical.json](strict-historical.json), [log](strict-historical.log): immutable original package validates, current edited sources drift; not current-source delivery acceptance |
| `git diff --check` | 0 | [diff-check.txt](diff-check.txt) |
| `go test ./tests/foundation -run '^TestProductEditorMatrixLedgerEvidenceRunReceipt$' -count=1 -v` on the prior validator | 1 (RED) | [run-receipt-red.json](run-receipt-red.json), [uncompressed log](run-receipt-red.log): all 10 immutable run-receipt defects incorrectly drifted, including unrelated gate comment + omitted old producer |
| `go test ./tests/foundation -run '^TestProductEditorMatrix' -count=1 -v` after independent receipt binding | 0 | **final source** [focused-receipt-final.json](focused-receipt-final.json), [uncompressed log](focused-receipt-final.log): 10 new receipt controls plus existing strict/default policy and genuine new-producer/table evolution |
| `LC_MATRIX_EVIDENCE_STRICT=1 go test ./tests/foundation -run '^TestProductEditorMatrixLedgerEvidence$' -count=1 -v` after receipt binding | 1 (expected historical negative) | [strict-receipt-historical.json](strict-receipt-historical.json), [log](strict-receipt-historical.log): actual canonical full-run receipt validates; source drift remains fatal in strict mode |
| `git diff --check` after receipt binding | 0 | [diff-receipt-final.json](diff-receipt-final.json), [log](diff-receipt-final.log) |
| `pnpm install --frozen-lockfile` | 0 | [deps-frozen-install.json](deps-frozen-install.json), [log](deps-frozen-install.log), [deps-state.json](deps-state.json): existing store reused49/downloaded0, lockfile/source unchanged |
| `bash scripts/dev/test-node.sh` on `a7abaefe` | 1 (RED) | [test-node-final.json](test-node-final.json), [full log](test-node-final.log): sole failure is existing shard-plan synthetic +3 test capacity control, 59 existing unplanned +3 =62 >60 |
| `bash scripts/dev/check-gates.sh` on `a7abaefe` | 0 | [check-gates-final.json](check-gates-final.json), [full log](check-gates-final.log): actual plan59 unplanned and1276 top-level names valid; prior to explicit registration |
| `node --test tests/ci/shard-plan.test.mjs` before explicit registration | 1 (RED) | [shard-registration-red.json](shard-registration-red.json), [full log](shard-registration-red.log): same +3 capacity failure |
| `node --test tests/ci/shard-plan.test.mjs` after explicit registration | 0 (7/7) | [shard-registration-green.json](shard-registration-green.json), [full log](shard-registration-green.log), [registry-only-diff.json](registry-only-diff.json) |

Unchanged final validator/docs SHA-256 values are bound in `focused-receipt-final.json`; its checkout base SHA is recorded with dirty-source content hashes, not asserted as a pristine tested commit. The final registry SHA-256 is bound in `shard-registration-green.json`. E3 evidence proves the filesystem validator, test-entrypoint behavior and focused shard registration in the tested DB-free environment; full broad reruns on frozen `67cfc86f` are complete below. It does not prove current UI clicks, provider, production or independent acceptance.

## Delivery checklist and remaining work

`GATES.md` documents the required fresh delivery sequence: freeze executable sources; run original full `--browser-product-editor`; package unchanged native bytes/receipts and the indexed full-browser run receipt with unchanged commit/producer hashes; execute exactly:

```sh
LC_MATRIX_EVIDENCE_STRICT=1 go test ./tests/foundation -run '^TestProductEditorMatrixLedgerEvidence$' -count=1 -v
```

- `bash scripts/dev/test-node.sh`: first run **exit1** above; after focused registry repair, final full rerun **exit0**, see the final serialized receipts below.
- `bash scripts/dev/check-gates.sh`: initial pre-registration run **exit0**; final registry-freeze rerun **exit0**, see the final serialized receipts below.
- Current full browser/PG: **NOT_RUN and not required for unchanged GROUP3 UI**; CI and independent review **NOT_RUN** here, parent owns resources, push and review.
- Strict current-source canonical delivery: **NOT_RUN**; old canonical remains historical, unmodified. The strict historical failure is expected and is not PASS.
- Historical old expected claims are not compared to a future current table during drift; immutable schema/outcomes remain checked. A historical SKIP cannot certify current controls.
- No owned test fixture/process remains after these commands. All failure and red evidence is retained; no foreign files were cleaned.

## Minimal shared registry integration

The first Node run found that the four new GROUP3 Go names left the existing plan at59 unplanned; the unchanged synthetic +3 control then correctly failed at62 >60. Root authorized appending exactly these four names to existing g06. Other groups, existing names, estimates and thresholds are byte/structure preserved; no repack or assertion change. The same tests remain selected by g06. Root owns final shared JSON integration; full broad reruns will resume after the GROUP1 native resource window.

## Final serialized broad gates

Frozen implementation: `67cfc86f2bc1a9a88bad795fcd290f4decc80dbe`; all four source hashes before/after match.

| Exact command | Exit | Receipt |
| --- | ---: | --- |
| `bash scripts/dev/test-node.sh` first final freeze run | 1 | `test-node-registry-final.json/log`, sole pre-existing PR18 WebCrypto child10s deadline failure |
| `node --test --experimental-strip-types tests/admin/inbox-review.test.ts` isolated unchanged reproduction | 0 | `inbox-timeout-isolated.json/log` |
| `bash scripts/dev/test-node.sh` serialized repeat | 0 | `test-node-registry-retry.json/log`, 1690 passed, zero failed/skipped (`final-static-counts.json`) |
| `bash scripts/dev/check-gates.sh` | 0 | `check-gates-registry-final.json/log`, all83 documented modes executed; four new Go names explicitly registered |

The initial unrelated deadline is retained and no timer, assertion, suite or filter was relaxed. Current UI browser and strict fresh delivery remain NOT_RUN for this static evidence-policy group; the original canonical package remains unchanged historical evidence. E3 author-local filesystem/gate validation, independent review and GitHub CI pending.
