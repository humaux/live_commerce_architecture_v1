# PR8 K3 UNKNOWN authority delivery

- Task: `c975698a-2c85-46b3-a0ca-1d5826472bac`; delegated canvas key `c975698a-sub-k3`.
- Base commit: `e376e52c7de4645ed51547f603f0240c4e205aab`.
- Branch: `codex/lc-u2b-unknown-authority`; this delivery and source are committed together; final head is returned to the integrator.
- Worktree: `.worktrees/lc-u2b-unknown-authority`.
- Role: `commerce_worker`; dispatched model `gpt-6.1-sol`, reasoning `high` (parent dispatch confirmed); backend runtime revision UNKNOWN.
- Changed paths: `internal/inbox/{service,read,send_read}.go`, NEW `tests/foundation/inbox_unknown_authority_test.go`, `output/lc-u2b-inbox-page/k3-authority-*` only.

A9 adds `has_unknown_outbound: boolean | null` without omitting null. Every page reads the existing scoped `inbox.read_outbound(conversation, 50)` once. Row count and recognized operation state are checked before the inbound time floor or decryption. Any observed UNKNOWN yields true; only an exhaustive scan of fewer than 50 rows with all states recognized yields false; a full scan without UNKNOWN or a missing/unrecognized operation state yields null. First-page display limits and the inbound time floor remain unchanged; older pages still display only inbound items.

Interface: additive response field authorized by the parent owner's K3 direction. Migration/OpenAPI/permissions remain frozen. No direct runtime table grant/query was added. No old test or assertion was edited.

## Actual commands and evidence

All log paths below are relative to `output/lc-u2b-inbox-page/`. JSON run records retain argv, exit codes, start time, elapsed time and timeout. Source hashes, rather than the uncommitted HEAD at run time, bind the tested files.

| Command | Exit | Evidence |
| --- | --- | --- |
| `bash scripts/dev/test-focused.sh '^TestInboxUnknownAuthority$'` on unchanged base production files | 1 | `k3-authority-red.log`, `k3-authority-red-run.json`, `k3-authority-red-source.sha256` |
| `bash scripts/dev/test-focused.sh '^Test(InboxUnknownAuthority\|LiveConsoleSendLCN11Unknown\|LiveConsoleInboxCrossStoreIsolation\|LiveConsoleSendMigration0128ExactACL)$'` | 0 | `k3-authority-green.log`, `k3-authority-green-run.json`, `k3-authority-green-source.sha256` |
| `GOTOOLCHAIN=go1.27.1 NODE_PATH=<parent inbox worktree>/node_modules bash scripts/dev/check-gates.sh` | 0 | `k3-authority-gates.log`, `k3-authority-gates-run.json` |
| `shasum -a 256 -c output/lc-u2b-inbox-page/k3-authority-green-source.sha256` | 0 | Four source/test hashes verified before commit |
| `git diff --cached --check` | 0 | No whitespace errors before commit |

Focused PG uses PostgreSQL 18.6, Go 1.27.1, `-race -count=1`; supervisor timeout 900 seconds plus 5-second termination grace. GREEN: 4 top-level tests, 9 new subcases; original UNKNOWN 5xx/garbled/hang, cross-store isolation and exact migration 0128 ACL tests also pass. New cases prove null for old UNKNOWN outside 51 newer outbound, true for UNKNOWN below 51 newer inbound, explicit false for empty/exhaustive healthy, null for missing operation and exactly 50 healthy, true for UNKNOWN in full scan, per-page refresh and cross-store/cross-tenant 404.

Static supervisor timeout 300 seconds plus 5-second grace. Gate reports 79 modes and 1220 foundation tests, includes header ratchet and normal/browser-tag Go vet. New test lands in the existing catch-all shard. Parent dependencies are read through NODE_PATH; no dependency installation or lockfile changes.

Initial failed attempts are preserved and excluded from valid bug-red evidence: `k3-authority-red-harness*` (synthetic NULL window and fixture UUID cast), `k3-authority-red-header-expectation*` (existing header is `private, no-store`), `k3-authority-gates-missing-deps*` (fresh worktree lacked TypeScript compiler alias). Fixes corrected setup/expectations only, with no weakening of authority criteria.

## Evidence boundary and unresolved work

- Recorded E3: automated MOCK/REAL_PG checks bound to file hashes. Independent acceptance is pending parent review/re-run; author is not the sole approver.
- Intentional limitation: >=50 healthy outbound messages return null and keep sending unavailable until an authorized unbounded scoped database fact exists. Null also covers missing/unrecognized operation states. This trades availability for fail-closed authority without inventing permission.
- UI integration, refresh/hide recovery, browser click acceptance, full foundation suite and all affected browser modes: NOT_RUN in this worktree; integrator owns those gates.
- Real provider SANDBOX/LIVE, production, push and deployment: NOT_RUN.
- All owned supervisors exited; focused PostgreSQL containers removed by harness cleanup. Shared caches, unrelated containers and other workers' files were untouched.
- Humaux record: `[fix] LC-U2b PR8 K3 nullable outbound UNKNOWN authority`, original record `e35a1578-d5a2-48f0-971b-4b978497f649`; own subcanvas updated under lock. Four files submitted to code index; code rationale linked at handoff.

Integrator to-do: cherry-pick the returned commit, independently review/re-run the focused regex, consume the nullable field with `=== false` as send authority and preserve latched UNKNOWN, then run the shared UI/CI acceptance gates. No migration, dependency or privilege merge is requested.
