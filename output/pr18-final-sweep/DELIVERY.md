<!-- Purpose: hand off completed client-only PR18 FINAL items1/4 and the unapproved A2 seq DTO boundary.
Depends on: pr18-867f650b-sweep, tested client source1d00ae16 and current A2 Go/TS DTOs.
Used by: integrator scope ruling and resumption of the remaining FINAL batch; not a full-unit PASS. -->
# PR18 FINAL sweep — IG P1 fixed, items2/3 awaiting DTO ruling

## Latest update: TOP-PRIORITY P1 4227895537

- Tested source **34b01cabb229308c4f4427c51f8d136a81d989c7**, on the prior checkpoint82999818. It retains client fixes1/4 below. No Go/DTO flag or schema change.
- `applyCommentPage` infers HEAD absence only when `reconcileHead && !older && source_platform === "facebook" && nonempty`. IG/unknown-source pages only merge. This uses the **existing** parsed server platform field, not an added authority flag or a client guess.
- FB cursorless bridge traversal is newest-by-arrival-seq: `comment_poll.go:626,634,729`; IG defaults to afterSeq0 and SQL `(received_at,event_id) ASC LIMIT`: `stream.go:261` and `0123_live_console_comments.sql:282`. A created_at display sort cannot turn earliest IG rows into an authoritative newest window.
- Two regressions failed on82999818, then passed: model100-row IG/unknown-authority case, and actual CommentStream hook/composer100-row case. The latter loads1–50 then after_seq50→51–100, selects75 and types an unsent private draft; three cycles of four3s ticks reread earliest1–50, retain all100/selection/draft, keep incremental continuation100 and send **zero** commands. No timeout/retry/threshold change.
- Existing real-source **Facebook** deletion and selected-composer regressions still pass. The model's older synthetic FB deletion fixtures now explicitly include the real response's platform field; their expected ref lists and assertions are unchanged.
- Read-only non-author source review of82999818..34b01cab found no introducedP0/P1, confirmed real FB/IG call-chain semantics and noGo/DTO change. It did not independently execute tests; this is E1 review, not K3/merge acceptance.

| Final-source command | Exit / count | Evidence |
|---|---|---|
| `node --test --experimental-strip-types --test-name-pattern='PR18 P1 IG' tests/admin/comment-stream.test.ts tests/admin/comment-stream-hooks.test.ts` (pre-fix) | **1,2 intended failures** | `red-ig-p1.log` |
| Expanded P1/FB-deletion/items1/4 selection after fix | **0,9/9** | `green-ig-p1.log` |
| `bash scripts/dev/test-node.sh` | **0,1212/1212** | `node-ig-p1.log` |
| `pnpm --filter admin exec tsc --noEmit` | **0** | `tsc-ig-p1.log` |
| `bash scripts/dev/check-gates.sh` | **0,82 documented modes** | `gates-ig-p1.log` |
| `LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-local.sh --browser-live-console` | **0,23/23; Go328.92s** | `browser-ig-p1.log`, `browser-ig-p1-results.txt` |

Browser source stayed fixed at34b01cab; current heartbeat lock, one browser/PG mode only, outer timeout780s. Only evidence files were edited during execution. Source SHA256 is retained in `source-ig-p1.sha256`. Artifact: `output/playwright/live-console-4290512843/`; final safe native-hidden counters/events retained in `native-hidden-ig-p1.json`. All owned gate/server processes exited. The IG100 regression is actual production-source Node hook/composer MOCK, not a new REAL_PG Instagram fixture; existing full browser suite remains MOCK Graph + real Next/Go/PG. New IG-specific browser seed is NOT_RUN (no Go fixture change authorised or made).

Post-fix3 production TS files were incrementally indexed (27 entities); `applyCommentPage` was linked to the current P1 problem record `f2ba2568-00d4-45b2-aa12-5011631d3b1c` so the provider-semantic reason is attached to the code.

**Still open:** original items2/3 (bounded whole-window reset and FB seq-vs-created_at eviction) and their tests; per-item seq approval request remains unanswered. No automatic IG reset was added. A future approved IG reset must follow the fresh earliest page's continuation to rebuild, not preserve an old cursor100 beside a fresh earliest50 buffer. Existing pending scope and NOT_RUN below remain; this update is not completion of all FINAL findings or approval to merge.

The integrator-declared unrelated5e3ce199 CI failures (webkit payment/promotions/storefront/g10) are not altered or attributed to this unit.

## Prior items1/4 checkpoint (historical evidence)

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

**Prior1d00ae16 checkpoint: E3, MOCK real-source Node only for items1/4.** Its browser gate was NOT_RUN. The newer34b01cab P1 checkpoint now has a full browser PASS as recorded above; it still is **not** completion of all FINAL findings or permission to merge. New independent K3/PR CI, Go/PG seq projection tests, full foundation and LIVE/production remain NOT_RUN. No owned background gates/servers remain.

CI gate remains `--browser-live-console` plus repository-selected required PR gates. Commit and stop, no push. Resume the same branch after approval; preserve this source/evidence and replace this partial delivery with the final gate record.
