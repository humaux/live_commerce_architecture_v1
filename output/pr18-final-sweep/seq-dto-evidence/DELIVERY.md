# PR18 seq DTO delivery

- task_id: 088736c6; delegated unit: pr18-seq-dto. Parent retains task/canvas ownership.
- Base: 21c27683fe22475b5ba2aa9954894305820675cf.
- Branch/commit: unit/pr18-seq-dto-only / f0b63101c910580316a618256b71e091221ecd7d.
- Worktree/evidence root: /Volumes/data/live_commerce_architecture_v1/.worktrees/pr18-seq-dto-only/output/pr18-seq-dto/.
- Role: integration implementation. Model: Codex/GPT-6 declared agent identity; exact runtime model id/effort UNKNOWN, inherited from parent.

## Changed paths and behavior

1. internal/integrations/metabridge/client.go: BridgeComment.Seq is *int64, json:"seq" without omitempty.
2. internal/integrations/metareply/comment_poll.go: pageLocked projects the existing private cc.seq into each copied buffer item; normalizeComments/direct Graph history stay nil.
3. internal/live/stream.go: ConsoleComment has the same nullable field; FB copies it.Seq, IG uses env.Seq. Page cursors, SQL and runtime authorization remain unchanged.
4. internal/integrations/metareply/comment_poll_seq_test.go: real ingestion/pageLocked serialization proves distinct arrival positions, dedup stability, independence from timestamps/page cursor and stable previously returned pages.
5. tests/foundation/live_console_comment_seq_test.go: actual authenticated A2 service + real FB bridge and signed IG webhook/encrypted copy seams prove per-item FB/IG numbers and explicit null on Graph history, including refs already present in the buffer.
6. contracts/live-console-v1.md §2.6: required number|null field, authoritative sources, no page-next/created_at synthesis, numeric coverage only in authoritative windows and null history excluded from absence deletion.

No OpenAPI edited: structured scan of all six existing contracts/*openapi*.json finds zero A2 /comments or comment-page paths and zero ConsoleComment/ConsoleStreamPage/BridgeComment schemas (openapi-proof.log, exit 0). No frontend/reconciliation, SQL, migration, GRANT, payment, production, secret or browser edits.

## Actual commands and exits

Commands run in the above worktree; logs are relative to its output/pr18-seq-dto/.

| Command | Exit/result | Evidence |
| --- | --- | --- |
| GOTOOLCHAIN=go1.27.2 go test -race -count=1 -run '^TestConsoleCommentItemSeqWire$' -v ./internal/integrations/metareply | 1; original code omits both item seqs; executable assertions, not compile failure | red-unit.log |
| LC_TEST_LOCK_WAIT=14400 LC_FOCUSED_TIMEOUT=900 bash scripts/dev/test-focused.sh '^TestLiveConsoleLCN02ItemSeqWire$' | 2; duration parse error, zero tests; NOT red evidence | red-pg.log |
| LC_TEST_LOCK_WAIT=14400 LC_FOCUSED_TIMEOUT=900s bash scripts/dev/test-focused.sh '^TestLiveConsoleLCN02ItemSeqWire$' | 1; PASS=0 FAIL=1 SKIP=0; FB bridge/A2/history + IG A2 seq omissions | red-pg-actual.log |
| GOTOOLCHAIN=go1.27.2 go test -race -count=1 -v ./internal/integrations/metabridge ./internal/integrations/metareply ./internal/live | 0; 48 top-level PASS, 0 FAIL, 0 SKIP; metabridge no test files | green-packages.log |
| LC_TEST_LOCK_WAIT=14400 LC_FOCUSED_TIMEOUT=900s bash scripts/dev/test-focused.sh '^TestLiveConsoleLCN(01BufferCapAgeAndCursor\|02\|04NoCommentTextPersisted\|05PrintAndMarks)' | 0; PASS=7 FAIL=0 SKIP=0; new FB/history + IG subtests passed | green-pg.log |
| LC_HEADER_BASE=21c27683 bash scripts/dev/check-gates.sh (fresh worktree) | 1; missing local node dependencies | check-gates.log |
| pnpm install --offline --frozen-lockfile | 0; ignored dependencies only, no lockfile change | install-offline.log |
| LC_HEADER_BASE=21c27683 bash scripts/dev/check-gates.sh (after dependency install) | 0; 82 modes, 1250 top-level shard assignments, headers/format/vet checks | check-gates-with-deps.log |
| gofmt -w on the five changed Go files | 0 | source files |
| git diff --cached --check; git commit with required Co-Authored-By trailer | 0 | commit above |

The PG runner owns a disposable fixture and uses the existing machine-wide heartbeat lock. LC_TEST_LOCK_WAIT=14400 was passed as required; this baseline test-focused.sh itself calls lc_lock_acquire 86400, so that runner's actual wait ceiling is 86400 seconds (no script edited). Every own exec session has completed; no task-owned PG container/process remains. Source snapshot is bound to the commit above and evidence-hashes.log.

## Evidence and unresolved gates

E3 scope: automated local package tests (MOCK) and focused REAL_PG + MOCK Graph only. Implementation used existing types/projections and standard encoding/json; no abstractions/dependencies added (ponytail minimal-change workflow).

NOT_RUN: parent independent review/rerun, GitHub required gates/full foundation, frontend/browser reconciliation, provider SANDBOX/LIVE Meta, production. The parent must combine its separately owned TS parser/fixture changes with this isolated Go+contract commit and apply the repository PR gate registry. No production readiness claim.

Humaux tools are not exposed to this worker: store/canvas/code-index/code-memory-link/rejected enumeration unavailable (UNKNOWN), no new claim made, no shared state written. Parent owns any recording/reconciliation. No P0/P1 found by self-check; independent review is outstanding.

Retention: output evidence and local dependency installation stay in this worktree for parent collection; preserve these logs before archival. No external sends, push, merge or deploy occurred.
