<!-- Purpose: hand off completed client-only PR18 FINAL items1/4 and the unapproved A2 seq DTO boundary.
Depends on: pr18-867f650b-sweep, tested client source1d00ae16 and current A2 Go/TS DTOs.
Used by: integrator scope ruling and resumption of the remaining FINAL batch; not a full-unit PASS. -->
# PR18 FINAL sweep — partial checkpoint, awaiting DTO ruling

- Branch `unit/lc-u2a-comment-stream`, base867f650b, tested source **1d00ae16**. No fetch/rebase/merge/push/deploy or Go/DTO change in this checkpoint. Unrelated untracked output preserved.
- Codex-1 wrote the code. Two read-only explorers inspected receipt callers and seq/cursor semantics; their turns were interrupted by the scope-question UI, so their findings are source pointers, not independent acceptance. No delegated writes.

## Implemented

**Item1**: PUBLIC unresolved key now uses store/session/ref only; the coarse session receipt still includes the CSRF boundary. The actual CommentReceipt regression arms before login rotation, remains blocked on a new boundary, proves store/session/ref isolation, and clears the shared guard only through the existing terminal/verified API. No text/author/cookie is written to storage.

**Item4**: an older-page `invalid_cursor` drops only `older`; buffer, selected ref and incremental cursor remain. Normal polling resumes through that same valid live cursor. A stale older callback cannot issue another pagination read. Invalid incremental cursors still clear normally; authority loss paths are unchanged.

## Gates / red→green

| Command | Exit / count | Evidence |
|---|---|---|
| `node --test --experimental-strip-types --test-name-pattern='PR18 final' tests/admin/comment-stream-hooks.test.ts` (pre-fix) | **1, 2 intended failures** | `red-client-final.log` |
| Same command after fix | **0, 2/2** | `green-client.log` |
| `bash scripts/dev/test-node.sh` | **0, 1210/1210** | `node.log` |
| `pnpm --filter admin exec tsc --noEmit` | **0** | `tsc.log` |
| `bash scripts/dev/check-gates.sh` | **0,82 documented modes** | `gates.log` |

`red-client.log` is retained as exploration history: a strip-only model test could not load the existing parameter-property class (`ERR_UNSUPPORTED_TYPESCRIPT_SYNTAX`). Moving that regression to the existing transpiled hook host, without changing the class syntax/runtime, produced the two causal reds above.

## Items2/3 — NOT_IMPLEMENTED, approval requested

The actual A2 item has **no seq**: `internal/live/stream.go:89` ConsoleComment and `internal/integrations/metabridge/client.go:56` BridgeComment. Page `next.seq` is only a page endpoint; assigning it to every row would fabricate sequence membership. The TS parser is exact and currently rejects an extra item field (`comment-model.ts:190`).

The smallest proposed addition is **`seq: number | null`**, Go `*int64` without omitempty, within the existing page epoch:

- Facebook buffered rows expose the existing private `consoleComment.seq` (`internal/integrations/metareply/comment_poll.go:141,626–627`) through BridgeComment and the A2 projection (`internal/live/stream.go:231`).
- Instagram exposes the existing decrypted event envelope `env.Seq` (`internal/live/stream.go:279,296–301`).
- Direct Graph history/backfill has no buffer sequence and must return **null**, not an invented timestamp-derived/page-endpoint number.
- Synchronize the exact TS parser/type and affected real projection/fixture tests. No SQL, GRANT, route, message-write or provider mutation is proposed.

Integrator approval is required **before** any of these Go/DTO changes. The async approval question was sent; no answer had arrived at this checkpoint. Items2/3 remain open, including their red→green tests.

The Instagram HEAD differs from Facebook: it starts from `afterSeq=0` and advances through ascending event envelopes (`stream.go:260–303`), so it is not a newest50-by-seq view. Reconciliation must use the actual returned numeric sequence coverage, never a created_at floor. Bounded whole-live-window refresh (N≤5min) and explicit manual refresh/history handling will be implemented after this contract ruling; no unsafe substitute was shipped.

## Evidence / NOT_RUN

**E3, MOCK real-source Node only for items1/4**. This is **not** completion of the four-item batch and not permission to merge. `--browser-live-console` is **NOT_RUN** on this checkpoint, deferred until final source after the DTO ruling to avoid presenting a partial-source run as final acceptance. New independent review/PR CI, Go/PG projection tests, full foundation and LIVE/production are NOT_RUN. No owned background gates/servers remain.

CI gate remains `--browser-live-console` plus repository-selected required PR gates. Commit and stop, no push. Resume the same branch after approval; preserve this source/evidence and replace this partial delivery with the final gate record.
