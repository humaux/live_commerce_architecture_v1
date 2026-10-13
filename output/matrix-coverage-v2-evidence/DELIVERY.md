<!-- Purpose: Record MATRIX-COVERAGE-V2 GROUP3 implementation and immutable evidence.
Depends on: assigned foundation validators and DB-free receipts.
Used by: parent integrator and independent reviewer. -->
# MATRIX-COVERAGE-V2 GROUP3 delivery

- task_id: `f597a8bc-ce29-4b68-ab31-815dd5a319cb` (parent); subcanvas `codex4-matrix-v2-sub-evidence`.
- base_commit: `0bfe36e53acb694f4697521202e24f0790638941`; branch `unit/matrix-coverage-v2-evidence`; worktree `.worktrees/matrix-coverage-v2-evidence`.
- Original author: `/root/matrix_v2_evidence` (Carver, `commerce_worker`), model `gpt-6.1-sol`, reasoning `high`, from owner-supplied **observed thread metadata** for thread `01a133fe-95a6-7c52-bddb-3730de0585fd`. All 8 `turn_context` records from `2026-10-13T06:41:31Z` through `2026-10-13T07:53:40Z` report that configuration, and `session_meta.agent_path` identifies this author. This is the owner's trusted observation; this repair did not independently read local session files. Original root model/effort remains **UNKNOWN**.
- New round1review author: `/root/matrix_v2_evidence_review`, model `gpt-6.1-sol`, reasoning `high`, from explicit parent spawn metadata. No recursive delegation or author push.
- Changed paths: `tests/foundation/catalog_matrix_ledger_test.go`, `tests/foundation/catalog_matrix_evidence_test.go`, focused strict-delivery section of `docs/delivery/GATES.md`, `output/matrix-coverage-v2-evidence/**`; root explicitly extended write_paths to `scripts/dev/shard-plan.json` for four g06 registrations, with integrator owning final JSON merge.
- Contract/dependency/migration changes: none. The 14-entry producer declaration and current live-browser exact-ledger interface stay unchanged.

The root cause was the typed drift return preceding native copy, required-file and outcome checks, while artifact hashing covered only the browser subtree. The validator now checks every artifact-index path/byte/hash inside the canonical package, all copy/index memberships, all five required native files and native outcome coherence before classifying source drift. All recorded source digests must remain valid; an absent newly introduced producer is historical only if the receipt-bound gate declaration source differs from the current checkout. An unchanged declaration missing a producer remains malformed. The follow-up P2 control also requires the independently indexed frozen `browser-product-editor.json` receipt to corroborate the recorded old producer set, tested/after commit, exact full-mode command and explicit exit 0 before drift. An unrelated gate comment cannot excuse removing a producer still present in the immutable run receipt.

Historical ledgers retain schema, unique identity, nonempty claim, PASS, locale/390px and receipt/count validation without requiring a future table or Git history. Matching sources retain the exact current table identity/expected-claim gate. Immutable corruption remains fatal under strict and default policies. Isolated mutation fixtures rebind the sole current source table and explicitly synthesize a **MODEL_ONLY** ledger/counts from its expected keys; they also explicitly rebind a copied run receipt marked `fixture_evidence_class=MODEL_ONLY`; they never modify or establish delivered canonical browser evidence. `TestProductEditorMatrixLedger` still independently checks the recorded golden projection.

## Historical commands and receipts (prior implementation; not current repair acceptance)

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
| Historical working-tree-only `git diff --check` (does not validate the delivered PR patch) | 0 | [diff-check.txt](diff-check.txt) |
| `go test ./tests/foundation -run '^TestProductEditorMatrixLedgerEvidenceRunReceipt$' -count=1 -v` on the prior validator | 1 (RED) | [run-receipt-red.json](run-receipt-red.json), [uncompressed log](run-receipt-red.log): all 10 immutable run-receipt defects incorrectly drifted, including unrelated gate comment + omitted old producer |
| `go test ./tests/foundation -run '^TestProductEditorMatrix' -count=1 -v` after independent receipt binding | 0 | **final source** [focused-receipt-final.json](focused-receipt-final.json), [uncompressed log](focused-receipt-final.log): 10 new receipt controls plus existing strict/default policy and genuine new-producer/table evolution |
| `LC_MATRIX_EVIDENCE_STRICT=1 go test ./tests/foundation -run '^TestProductEditorMatrixLedgerEvidence$' -count=1 -v` after receipt binding | 1 (expected historical negative) | [strict-receipt-historical.json](strict-receipt-historical.json), [log](strict-receipt-historical.log): actual canonical full-run receipt validates; source drift remains fatal in strict mode |
| Historical working-tree-only `git diff --check` after receipt binding (does not validate the delivered PR patch) | 0 | [diff-receipt-final.json](diff-receipt-final.json), [log](diff-receipt-final.log) |
| `pnpm install --frozen-lockfile` | 0 | [deps-frozen-install.json](deps-frozen-install.json), [log](deps-frozen-install.log), [deps-state.json](deps-state.json): existing store reused49/downloaded0, lockfile/source unchanged |
| `bash scripts/dev/test-node.sh` on `a7abaefe` | 1 (RED) | [test-node-final.json](test-node-final.json), [full log](test-node-final.log.gz): sole failure is existing shard-plan synthetic +3 test capacity control, 59 existing unplanned +3 =62 >60 |
| `bash scripts/dev/check-gates.sh` on `a7abaefe` | 0 | [check-gates-final.json](check-gates-final.json), [full log](check-gates-final.log.gz): actual plan59 unplanned and1276 top-level names valid; prior to explicit registration |
| `node --test tests/ci/shard-plan.test.mjs` before explicit registration | 1 (RED) | [shard-registration-red.json](shard-registration-red.json), [full log](shard-registration-red.log.gz): same +3 capacity failure |
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

## PR47 round1review repair — CI only, pending acceptance

- task_id: `f597a8bc-ce29-4b68-ab31-815dd5a319cb-sub-evidence-review`.
- base_commit: `8ab590779d9ed0e22c5f39031143e3e93dcf893a`; negative-control commit `f03b76ba`.
- Current repair evidence: **E1 static edits only**; compilation, lint, all Go/Node/browser/PG/gate runs and independent review **NOT_RUN by this author**. GitHub CI and independent acceptance belong to the parent; historical local PASS receipts above do not prove this repair.
- Allowed paths: both foundation matrix validators, this output directory, canonical `historical-evidence` source/manifest/index and necessary frozen snapshots. Workflow/shard-plan/shared contracts untouched by this repair.

4256257030: safe lexical source paths and valid recorded digests precede source reads; actual producer ENOENT becomes typed historical drift. Missing artifacts, dangling source links, source directories and all non-ENOENT errors remain fatal. Existing child-policy test includes removed/renamed producer default SKIP and strict FAIL, plus directory/link hard failures.

4256257042 / 4256257073: [FROZEN-IDENTITIES.json](../matrix-ledger-enforce/historical-evidence/FROZEN-IDENTITIES.json) binds the actual `2e10084b3f5e96548c15953cc40488aef9e1c86d` Git blobs (SHA-256 and Git blob IDs), raw case table, original locale source, matrix recorder and all PE/review/feedback/CC12 consumer sources. Its identities come from the unchanged native list reporter: full12 PE/review +2 CC12, each file/title exactly once and zero skips. SOURCE and manifest bind the new record digest; the full package index binds the record/snapshots. This is added provenance for an existing run, not a fresh browser run. Historical expected claims are checked against the frozen table plus parsed original locale labels, and logs against the recorded identities before any drift return. Current matching sources retain exact current ledger and title contracts. Added rename/add/remove current-title and locale controls use the existing MODEL_ONLY fixture; delivered native bytes are never synthesized from current PASS rows.

4256257059: recursively enumerate every canonical package regular file (outer and nested), excluding only root `ARTIFACTS.json`, reject symlinks/special files and require exact index membership before drift. Child controls cover unindexed outer/nested files under both policies with matching and changed source; frozen record corruption and rehashed fake/duplicate/missing case identities remain fatal under drift.

4256257049: corrected the original author configuration using the owner-supplied observed thread metadata identified above: `gpt-6.1-sol / high`, 8 records in the exact stated UTC interval. Original root configuration remains UNKNOWN; the new review author configuration continues to derive separately from explicit parent spawn metadata.

4256257065: parent owns CI `git diff --check` over the explicit PR base..head and the committed whitespace negative control. No local diff check was run for this repair. Static reading found whitespace in four historical author-local logs; [historical-raw-log-copies.json](historical-raw-log-copies.json) records lossless gzip paths, original byte sizes and SHA-256 values. All original failed/red bytes remain recoverable. Native canonical bytes remain unchanged.

Actual operations this repair: repository/Humaux directed reads, static file edits, `git add` and `git commit` (exit0 for negative-control commit); no test/gate/formatter process executed locally. [round1review-static.json](round1review-static.json) binds current source hashes and static observations. New GitHub CI red/green, delivered-patch whitespace receipt and independent review remain NOT_RUN here; the parent must bind them to the implementation head SHA before completion.

CI-only owner rule supersedes prior local gate instructions. Root adds a PR47-specific Foundation step that runs the full `TestProductEditorMatrix` family and checks the delivered patch with `git diff --check "$PATCH_BASE..$PATCH_HEAD"` using the GitHub event base/head SHAs. CI retains its gofmt patch for static application; no formatter or gate ran locally. Red test-only SHA `f03b76ba5972d403a4db604b4763d48f1c7a31c7`: Foundation `38380064266` failed at formatting before behavioral controls, so it is not claimed as behavior RED. Gates `38380064714` remains the red-stage execution. Implementation `8914d148` plus workflow binding awaiting CI.

GitHub formatter preview `38381067824` produced the exact [format patch receipt](ci-format-38381067824/receipt.json), then stopped before compile/behavior because the formatter diagnostic returned exit1. The raw diagnostic and header-normalized apply copy are retained losslessly as gzip with raw hashes/sizes. Static `git apply -p0` exited0; source hashes were refreshed. No local formatter/check/test ran. Parent workflow commits `fc8b7642` and `f9956bf9` remain preserved; CI behavioral red/green and independent acceptance remain pending.

GitHub CI `38381815228` at `31781cbf42a0c72c4e1432eb161118d03a8feaeb`: full `go test ./tests/foundation -run '^TestProductEditorMatrix' -count=1 -v` **PASS**; the delivered-patch whitespace step correctly failed (exit2) on raw downloaded CI log whitespace. Its injected committed whitespace defect was RED and the corrected probe was GREEN before checking the actual PR patch. The raw logs are retained losslessly as gzip with [byte/hash receipt](ci-raw-log-copies.json), so no whitespace defect is hidden or normalized. The full Node/check-gates steps were not reached in this run; acceptance remains pending the subsequent exact-source CI.

PR47 integration correction `e0a15aa7` keeps SOURCE.json fourteen owned unit producers authoritative for current-checkout drift. Frozen auxiliary consumers corroborate historical native file/title identities through indexed hash-bound snapshots and do not expand checkout binding. Existing SharedSources now covers all three context changes and requires no drift; frozen history corruption controls remain. Test-first `b60e4c57` and the correction await exact-source GitHub CI. No local formatter or test ran.

CI `38383142992` at `b416e482` reported the inert frozen filename `catalog-core.spec.ts.source.txt` as an unregistered executable spec. It is now named `catalog-core-spec.source.txt`; the tested source key remains `tests/admin/catalog-core.spec.ts`. Raw snapshot SHA-256 remains `18db7e6e25ddfc05bdd6b9fecc6f80b3bd47e92f1e31addddf6ff318c21da0d8`, and all native bytes, Git blob IDs and integrity assertions remain unchanged. FROZEN-IDENTITIES path, SOURCE/browser record digest and exact ARTIFACTS membership/digests were rebound; record SHA-256 is `bee23db00d350a23d999078119239f4afc8499df80572e1538b883af0f678e07`. Static file edits only; no local test/check/formatter ran. New exact-source CI and independent acceptance remain pending.

CI Foundation `38383142992` at `b416e482`: the complete focused Matrix Go family and the committed `base..head` whitespace check PASS; check-gates fails because the inert frozen `catalog-core.spec.ts.source.txt` basename still matches the tracked `*.spec.*` inventory. Commit `e47cd674` renames only that snapshot to `catalog-core-spec.source.txt` and rebinds its metadata/index; native/source snapshot bytes and all assertions are unchanged. The RED CI log is preserved losslessly with digest/byte receipt in `ci-raw-log-copies.json`. Current full Node/check-gates acceptance remains pending GitHub CI.

## Current Foundation CI result (complete Gates pending)

GitHub Foundation [38384639292](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38384639292) at branch head `42e47bc756b92cbd194e2ac6da849e75ee2cea22` is **PASS**. The raw downloaded log is preserved losslessly as [ci-green-38384639292.log.gz](ci-green-38384639292.log.gz), with raw/archive SHA-256 and byte count in [ci-raw-log-copies.json](ci-raw-log-copies.json). Actual job/step metadata is in [run.json](ci-green-38384639292/run.json) and [receipt.json](ci-green-38384639292/receipt.json). No local test/check/formatter was executed.

Foundation ran `go test ./tests/foundation -run '^TestProductEditorMatrix' -count=1 -v`, the actual delivered `git diff --check "$PATCH_BASE..$PATCH_HEAD"` check with the committed-whitespace injected RED/repair GREEN, `bash scripts/dev/check-gates.sh` (83 modes, every tracked test file run), and the full Node suites, all exit0. The historical canonical-evidence test takes its explicit source-drift SKIP; all other Matrix tests and injected corruption controls PASS. Complete Gates `38384639948` was canceled after new integrity defects were identified; it is not a full-chain PASS. Current browser and strict fresh canonical delivery are not claimed by this Foundation proof. **E3 for these CI-executed integrity controls; independent acceptance pending.**

## Coordinated corruption follow-up (GitHub-only)

- `fd4e0fb5` Foundation [38388946999](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38388946999) failed compilation on misplaced fixture bindings. This is not behavioral RED. Test-only `36cad088`/`910cec2c` correct fixture placement without implementing the validator.
- At `910cec2c`, Foundation [38389630299](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38389630299) is actual behavioral RED, exit1: RunReceipt, SharedSources and RunIdentity reject-coordinated-corruption assertions fail. Original raw failure logs and run/job metadata are retained with byte/hash receipts.
- `843d9d2e` binds the old producer set to the frozen Go declaration, verifies all54 package members via an independent compiled sorted-name digest, and checks the actual original12+2 file/title identities against the four frozen consumer hashes. Neither current source evolution nor mutable receipt maps define historical success. Full-package isolated fixtures also cover coordinated receipt/index deletion with matching sources and real drift. Native browser bytes remain unchanged; this adds no synthetic browser PASS.
- Root supplied original author provenance has already been corrected in `a881f08c`; owner-observed thread metadata is distinguished from repair-author explicit spawn metadata.
- Current-source GitHub GREEN/full required workflow is PENDING. No local test, check, formatter, runtime, server or database was executed for these edits. Red-stage Gates38388946977/38389630693 were canceled intentionally, never reported PASS.

GitHub Foundation38390303629 at4d4f87d1 passed the focused full Matrix Go step, including the new coordinated-corruption controls. The job supplied format artifact11808197289; root statically applied its exact Go alignment patch (old-path .orig header removed only) with git apply exit0. Raw and apply forms are retained losslessly under ci-format-38390303629 with hashes. No local formatter or check ran. Node/check-gates/full required workflow acceptance remains pending; the next CI validates the formatted source.
