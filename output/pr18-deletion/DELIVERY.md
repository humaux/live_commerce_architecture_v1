<!-- Purpose: PR18 thread4227227603 client deletion-window reconciliation and acceptance evidence.
Depends on: packet pr18-1c43bfe4-thread-4227227603, live-console-v1 section2.2 and existing10s HEAD refresh.
Used by: integrator K3 pre-review and PR18 CI; author-local evidence with explicit browser deletion limitation. -->
# PR18 deleted-comment privacy fix

- Branch `unit/lc-u2a-comment-stream`; fetched/merged own remote, already current at **`1c43bfe4`**. Retained integrator host settle fix07cc90e5 and BuyerPanel-local404 ruling.
- Tested source **`97afbe2c9cfefdd838038d22d61a5216d8b0a0ff`**. Final evidence commit changes no product/test source.
- Codex-1, GPT-6 family (exact runtime identifier/effort unavailable), no delegates. Own worktree only. No push, deployment or review-thread resolution. W3-U3 remains paused.

## Fix

`applyCommentPage` now has an explicit internal `reconcileHead` option, used **only** at the existing periodic10s HEAD read in `use-comment-stream`. It reuses the chronological comparator for the oldest head tuple. On an eligible non-empty head page, retain buffered rows below that floor and refs present in the head; remove absent refs at or above it. Reset/epoch handling, ref dedupe, newest1000 cap and live/history cursor behavior remain in the existing path. Incremental reads, history reads and empty heads infer no deletion. There is no new polling request or cadence.

After eligible HEAD reconciliation, a functional selection update clears an evicted selected ref. The same render removes its comment and unmounts CommentReply (and its comment-derived buyer selection), rather than only hiding stale text. A pending per-comment public guard remains in sessionStorage so deletion cannot permit a duplicate uncertain send. No contract, Go product, SQL, schema or permission change.

## Red → green

Actual-source Node model/component cases: **3 FAIL +3 negative PASS on old code →6 PASS** (`red.log`, `green.log`).

1. Missing ref within the head window removed.
2. Older paged rows below the floor retained.
3. Incremental/older pages do not remove absent refs, even with an accidental head flag on an older read.
4. Empty head retains all buffered rows.
5. Equal timestamp refs and equivalent timezone instants use the same tuple ordering as the buffer.
6. Actual CommentStream + CommentReply under fake clock: queued public reply → incremental reads retain the row → periodic HEAD omits it → row, selection and composer removed; older history and the durable public guard retained. No extra head reads before the existing cadence.

## Final gates

| Command | Exit | Evidence |
|---|---:|---|
| `node --test --experimental-strip-types --test-name-pattern='PR18 (deletion\|empty deletion)' tests/admin/comment-stream.test.ts tests/admin/comment-stream-hooks.test.ts` — old code | 1 | `red.log`,3 intended failures |
| Same after fix | 0 | `green.log`,6/6 |
| `bash scripts/dev/test-node.sh` | 0 | `node.log`,**1198 tests /0 failures** |
| `pnpm --filter admin exec tsc --noEmit` | 0 | `tsc.log` |
| `bash scripts/dev/check-gates.sh` | 0 | `gates.log`,82 modes documented |
| `LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-local.sh --browser-live-console` | 0 | `browser.log`,**23/23**,Go333.34s |

Full browser evidence: `output/playwright/live-console-2514376323/`. This is the existing real-click control/privacy/reply suite with real Next/Go/PG + MOCK Graph. It proves no regression; it does **not** exercise a synthetic user deletion. Source remained fixed during tests; PG/browser modes serial. `SOURCE-SHA256.txt` binds the four changed files.

## Deletion browser NOT_RUN and backend follow-up

The packet permits NOT_RUN if the fixture cannot evict one synthetic comment. The current fixture cannot drive a recent, same-epoch deletion from MOCK Graph into the real ring: `internal/integrations/metareply/comment_poll.go:620` skips existing refs; `:644` eviction only checks age and capacity; its sole `delete(s.byRef,…)` is`:649`. A Graph list omitting a cached id therefore does not immediately evict it. Replacing the worker via the existing reset fixture increments the epoch and exercises RESET, not this HEAD-window path.

This also differs from contract section2.2 (`contracts/live-console-v1.md:126`), which specifies a60s batch id check and missing-id eviction. Source was inspected read-only; no backend workaround or fake product response was introduced. **Integrator backend follow-up is needed to supply that eviction behavior/seam before the deletion-specific real-browser path or actual Meta deletion can be accepted.** The client honors an absent ref once an eligible head reports it; its6 Node cases test that behavior directly.

## Handoff

E3 for the scoped client fix and requested regression gates. NOT_RUN: deletion-specific browser case (reason above), new independent K3/required PR CI, LIVE Meta/production, full foundation. Unrelated R04 runner remains an explicit test-node skip because its binary env is unset.

Existing tracked output evidence was not overwritten; untracked ext-agent material and prior CI downloads were untouched. All owned commands exited. Commit and stop; integrator owns push, thread replies and independent review. CI gates: `--browser-live-console` plus repository-selected required PR checks.
