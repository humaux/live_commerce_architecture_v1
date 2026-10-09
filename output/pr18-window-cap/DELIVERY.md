<!-- Purpose: record PR18 short-HEAD reconciliation, bounded A8 paging and enabled privacy gate evidence.
Depends on: packet pr18-0ab315ee items 1–3, source 63b53d9c, real hook host and real PG/browser harness.
Used by: integrator independent review and PR18 CI; author E3 does not substitute for independent acceptance. -->
# PR18 complete-window / bounded-list / permission batch

- Branch `unit/lc-u2a-comment-stream`, base **5e3ce199** (integrator's trunk/#19 union); own remote pull was already up to date. No second trunk merge.
- Final tested source **63b53d9c6c57ed43bbd6f99ed9b5dd4b8ac119a0**; evidence-only commit does not change that source.
- Codex-1, no delegates. Runtime model/effort identifier unavailable. Product changes are UI only; no Go/SQL/contracts/registry changes, no push/deploy/Meta mutation. W3-U3 remains paused.

## Packet items

1. **4227899298**: a nonempty periodic HEAD shorter than its actual requested limit with `older_cursor=null` is the complete live window. Remove absent oldest live refs as well as refs above the floor. The allowed conservative `historyLoaded` choice preserves the existing floor rule after explicit older-page loading. Incremental, older and empty pages still never establish deletions. The requested limit is shared with the actual request, not inferred from response length.
2. **4227899287**: A8 merge dedupes by conversation/bundle identity, sorts newest-first by timestamp with deterministic ID tie-breaking, and retains at most the same **1,000** entries as A2. Loaded-history head refreshes retain the cursor only below the cap. Both the old-page command and its control stop at the cap. No new polling/read or storage path.
3. **4227961860**: the hook returns an empty buffer, no selection, `busy=false`, and `privacy.visible=false` when its current `enabled`/`live:read` is false. This also suppresses A8 rows, BuyerPanel and composer in the first downgraded render, before passive cleanup. Exposed select/refresh/older callbacks do nothing while disabled. Existing effect cleanup, request fences and synchronous hide/revocation paths remain intact.

## Red → green

| Command / scenario | Red | Green | Evidence |
|---|---:|---:|---|
| `node --test --experimental-strip-types --test-name-pattern='short complete HEAD\|full or cursor-bearing HEAD\|loaded history\|A8 paged list stays bounded' tests/admin/comment-stream.test.ts tests/admin/comment-stream-hooks.test.ts` | 1: 2 failures, 2 negative cases PASS | 0 | `red.log`, `green.log` |
| Complete-window removal / historical floor / A8 cap / selected composer / empty-head negatives (expanded focused selection) | — | 0: **9/9** | `green.log` |
| `node --test --experimental-strip-types --test-name-pattern='enabled downgrade' tests/admin/comment-stream-hooks.test.ts` | 1: **3/3 fail** | 0: **3/3 PASS** | `enabled-red.log`, `enabled-green.log` |

The enabled tests run the actual production hook and CommentStream. They hold queued passive effects without suppressing render, then inspect the first output after a same-ID staff Store loses only `live:read` (retaining `inbox:read`/reply). A2 includes a selected claim, real BuyerPanel and composer; A8 includes a selected conversation and BuyerPanel. Cleanup effects are then resumed and settled. The A8 cap case clicks older and executes 22 fake-timer head polls, asserting both retained state and rendered `<li>` counts stay bounded; the old code fails at head20.

## Existing native-hidden gate diagnosis

The first full run on the items1/2 checkpoint was **22/23, exit1**, only the existing native-hidden case failed (`browser.log`, `output/playwright/live-console-1417608559/`). Its server arrival counter changed31→32.

A network-readiness fix alone did not resolve it (`native-hidden-focused.log`, exit1). A read-only native-fetch counter then established **visible=2 / hidden=0**, and the document had become **visible** during the asserted hidden interval (`native-hidden-diagnostic.log`, exit1, `native-hidden-diagnostic.json`). Thus this was not an A2 request started in a hidden document. The blank cover was not a stable hidden-state precondition.

The second targeted fixture correction follows the passing INU05 pattern: use a real `/en/settings` cover, await actual network readiness before hiding, and assert trusted visibility events plus the final hidden state. Native fetch is measured without changing its arguments/results. The original **6,500ms interval and strict unchanged server counter** remain; no timeout/retry/threshold relaxation. Focused case passed (**1/1**, exit0; `native-hidden-cover.log`). Final-source full run retains this gate, plus the visible/hidden start-count assertions.

## Final gates (source63b53d9c)

| Command | Exit / count | Evidence |
|---|---|---|
| `bash scripts/dev/test-node.sh` | **0, 1208 tests / 0 failures** | `node-final.log` |
| `pnpm --filter admin exec tsc --noEmit` | **0** | `tsc-final.log` |
| `bash scripts/dev/check-gates.sh` | **0, 82 modes documented** | `gates-final.log` |
| `LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-local.sh --browser-live-console` | **0, 23/23, Go325.76s** | `browser-final.log`, `browser-results.txt` |

Source stayed fixed for browser execution; PG/browser runs are strictly serial through the current heartbeat lock. Browser outer timeout780s. Native fixtures and gate processes are owned by the harness and cleaned on exit. Generated SHA256 evidence binds all6 changed source/test files. Unrelated existing tracked evidence and other agents' untracked directories are preserved.

Full artifact: `output/playwright/live-console-4232163969/`. The final native-hidden document counter/event record is retained as `native-hidden-final.json`; original counter, first-render privacy assertions and existing real grant-revocation case all passed. All owned gate commands exited; no owned server/PG/browser process remains running.

## Evidence limits / CI gates

**MOCK + REAL_PG**, author-local E3, not LIVE Meta or independent E4. The same-store first-render downgrade, cap growth and deleted-oldest cases are Node real-source regressions; a deletion-specific browser scenario remains **NOT_RUN** because current backend ring fixtures only evict by age/cap (follow-up LC-B2-DEL is owned separately). Empty HEAD deliberately remains non-evidence under the prior ruling; loaded-history mode deliberately uses the accepted conservative floor policy.

Independent review/K3 and required PR CI, full foundation, LIVE Meta, production and physical devices are **NOT_RUN** here. Unrelated R04 Node executable remains explicitly skipped by the existing runner when its binary env is absent.

CI gates requested: `--browser-live-console` plus repository-selected required PR gates; no new mode or registry change. Integrator owns push, review replies and merge. Commit and stop.
