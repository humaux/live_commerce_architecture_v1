# PR18 IG scan DTO delivery

- task_id: 088736c6; delegated unit: pr18-ig-scan-dto; own canvas: 088736c6-sub-2 (parent unchanged).
- base_commit: 680aa30477747e32f494ca91d515f97c925d085c.
- head_commit: b2c0b48cd455754037c5d3fbe2763374692f9ca3; branch: unit/pr18-ig-scan-dto.
- worktree: /Volumes/data/live_commerce_architecture_v1/.worktrees/pr18-ig-scan-dto.
- role: integration implementation. Model: Codex declared GPT-6 identity; exact runtime model and inherited effort UNKNOWN.

## Exact changed paths

- internal/live/stream.go: required scan_exhausted bool JSON field, no omitempty; successful IG raw rows.Scan increments count and advances env.Seq before decrypt/media filtering; EOF proof is scanned<q.Limit after successful query. FB default false is emitted explicitly.
- contracts/live-console-v1.md §2.6: required envelope field, raw-scan/next semantics, conservative false means no EOF proof, query snapshot versus future arrivals, ordinary IG HEAD merge unchanged.
- internal/live/stream_scan_test.go: serialized zero/default producer must contain scan_exhausted:false; compiles and fails against original DTO.
- tests/foundation/live_console_comment_scan_test.go: real scope/A2 service + signed webhook/consumer/PG gates for all-filtered raw full page, deleted final row 100→99, empty raw query after99, and actual FB explicit false. Uses existing helpers and one reused real consumer pool for100rows.

No frontend, SQL source, migrations, GRANT, routes, producer side effects, dependencies, OpenAPI, push, merge or deploy edits/actions. Only this test's synthetic fixture ciphertext and final social.comment_events row were mutated under the existing owner-pool fixture mechanism.

## Actual commands, exits and evidence

Evidence paths below are relative to output/pr18-ig-scan-dto in this worktree. Source files were unchanged between green runs and the head commit; evidence-hashes.log binds source/log hashes to that commit.

| Command | Exit / observed result | Log |
| --- | --- | --- |
| GOTOOLCHAIN=go1.27.2 go test -race -count=1 -run '^TestConsoleStreamPageScanExhaustedWire$' -v ./internal/live (original production code) | 1; scan_exhausted omitted | red-unit.log |
| LC_TEST_LOCK_WAIT=14400 LC_FOCUSED_TIMEOUT=900s bash scripts/dev/test-focused.sh '^TestLiveConsoleLCN02ScanExhausted$' (first) | 1; missing field + all-filtered cursor stall; 100row fixture hit SQLSTATE53300 | red-pg.log |
| Same PG RED command after new-test-only pool reuse correction, production code unchanged | 1; PASS0 FAIL1 SKIP0; all three subcases reach intended assertions, including100→99/after99 | red-pg-actual.log |
| GOTOOLCHAIN=go1.27.2 go test -race -count=1 -v ./internal/live | 0; 8 top-level PASS,0 FAIL/SKIP | green-package.log |
| LC_TEST_LOCK_WAIT=14400 LC_FOCUSED_TIMEOUT=900s bash scripts/dev/test-focused.sh '^TestLiveConsoleLCN02(ScanExhausted\|ItemSeqWire)$' | 0; PASS2 FAIL0 SKIP0; five scan/seq subcases PASS | green-pg.log |
| pnpm install --offline --frozen-lockfile | 0; ignored dependencies only, no manifest or lockfile changes | install-offline.log |
| LC_HEADER_BASE=680aa304 bash scripts/dev/check-gates.sh | 0;82 modes,1251 top-level tests mapped exactly once; headers/format/vet checks passed | check-gates.log |
| gofmt -w on three changed Go files; git diff --cached --check; git commit with required Co-Authored-By trailer | 0 | committed source |
| git diff --exit-code 21c27683 680aa304 -- six existing contracts/*openapi*.json files | 0; unchanged specs | openapi-unchanged-proof.log |

Prior structured A2 OpenAPI absence proof remains at /Volumes/data/live_commerce_architecture_v1/.worktrees/pr18-seq-dto-only/output/pr18-seq-dto/openapi-proof.log (six JSON specs,0 A2 path/schema matches). None of those specs changed through this base or this worker commit.

## JSON proof and scope

Actual serialization assertions (map[string]json.RawMessage, not compile-only field existence):

- Full raw2 rows, one other-media and one undecryptable: items=[], scan_exhausted:false, next.seq=actual highest raw envelope seq.
- Full raw100 valid rows: scan_exhausted:false. Delete only synthetic last row:99 valid items, scan_exhausted:true, next.seq=actual row99 sequence.
- Empty raw query after row99: items=[], scan_exhausted:true, next.seq unchanged at row99.
- Actual FB buffer A2 response: explicit scan_exhausted:false.
- Prior FB/IG per-item seq numbers and Graph history null regressions remain green.

E3 evidence is limited to local MOCK + REAL_PG automated tests. false means no EOF proof, not has-more. Exhaustion describes the completed raw query view, not future arrivals; ordinary earliest-window IG HEAD merge behavior remains the client's existing behavior.

## Recording, process cleanup and remaining gates

- Humaux fix title: [fix] PR18 IG raw scan exhaustion DTO b2c0b48c; memory_id:3da42695-8067-4a4a-b159-9032dcaa2fb0 (store accepted; exact-title grep confirmed). Own canvas updated under acquired/released lock; no new task claim or parent canvas writes.
- code_index incremental completed3files/47entities/0rejected; code_memory_link linked instagramPage to the fix. First qualified-name lookup was not found; bare indexed function name succeeded. This is an index naming issue, not a test failure.
- All own exec sessions ended; disposable test PG containers and heartbeat lock released. No task-owned browser/full-suite run started. LC_TEST_LOCK_WAIT=14400 was passed; unchanged test-focused.sh itself calls lc_lock_acquire86400.
- NOT_RUN: parent independent review/rerun/integration, required GitHub CI/full foundation, browser/client rebuild verification, provider SANDBOX/LIVE, production. No independent acceptance or production readiness claim.
- Parent to combine this isolated Go+contract commit with its TS parser/fixture amendment via cherry-pick --no-commit; keep TS rebuild implementation separate. Preserve untracked evidence before archiving the worker worktree.
