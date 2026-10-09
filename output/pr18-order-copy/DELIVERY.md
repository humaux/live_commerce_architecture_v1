<!-- Purpose: final combined PR18 packet delivery, including ordering/cap and facts-copy addendum.
Depends on: packet pr18-5f888bca-thread-4226683270 sections1-4 and live-console-v1.
Used by: integrator K3 pre-review and PR CI; author-local E3, not independent merge approval. -->
# PR18 combined batch — final local gates PASS

- Branch `unit/lc-u2a-comment-stream`, worktree of the same name. Base packet `5f888bca`.
- Source **`cc2db2c5f05db2e102cb137fa763a26595af93b2`** includes public guard `0814e97e` and addendum fixes. Final evidence commit changes no runtime/test source.
- Codex-1; actual runtime model identifier unavailable; no delegates. No push/deploy/thread resolution. W3-U3 remains paused.

## All three packet threads in this batch

| Thread | Fix and evidence |
|---|---|
| 4226683270 | Per-comment non-terminal public-reply boolean guard survives queued/UNKNOWN, unmount/reload and31s; other comments usable. Existing coarse UNKNOWN retained. Real worker MOCK Graph5xx regression passes in the final full browser run. Detailed earlier red/green evidence: `../pr18-public-guard/DELIVERY.md`. |
| 4226831239 | `applyCommentPage` ref-dedupe then ascending `(created_at instant, ref)` ordering, documented in its header. Both normal and historical merges keep the newest1000 via oldest eviction. Existing full-history stop and cursor assertions unchanged. `CommentStream` renders that array without a second ordering pass. Tests cover newest-first FB vs oldest-first IG, newer poll, older page, duplicate freshness, equal instants with different timezone spellings, and both cap directions. Each fixture page is at most100 rows. |
| 4226831247 | Composer maps the marks-only `facts_unavailable` reason to the existing `comment_facts_unavailable` copy before rendering. Actual-source component tests in zh-TW/zh-CN/en assert exact actionable text and disabled controls. No new translations/API codes. |

Only two product files changed in this addendum: `CommentReply.tsx` and `comment-model.ts`. Frontend-architect reuse principle: keep the existing buffer/transport boundary and existing copy. No new library, Go product change, migration or schema. Prior public-guard Go changes are test-fixture-only.

## Final-source gates

| Command | Exit | Evidence |
|---|---:|---|
| `node --test --experimental-strip-types --test-name-pattern='PR18 (mixed\|cap keeps\|facts_unavailable)' tests/admin/comment-stream.test.ts tests/admin/comment-stream-hooks.test.ts` — old logic, final tests | 1 | `red-final-tests.log`,6/6 fail |
| Same command after restoring fixes | 0 | `green.log`,6/6 pass |
| `bash scripts/dev/test-node.sh` | 0 | `node.log`,**1191 tests,0 failures** |
| `pnpm --filter admin exec tsc --noEmit` | 0 | `tsc.log` |
| `bash scripts/dev/check-gates.sh` | 0 | `gates.log`,82 modes/all documented |
| `LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-local.sh --browser-live-console` | 0 | `browser.log`,**23/23**,Go317.35s |

Browser evidence: `output/playwright/live-console-578395423/`; `browser-results.txt` preserves its safe result. Includes existing three-locale real clicks, the new queued→real-worker UNKNOWN→selection/reload→31s guard test and both scoped404 privacy cases. PG/browser runs serial, source fixed throughout. `SOURCE-SHA256.txt` binds changed files.

Historical failure is preserved: previous source0814e97e full run was22/23 (scoped404 private-view wait timeout), and its isolated diagnostic was2/2. Logs remain under `../pr18-public-guard/`. The new full run passes; **no claim that ordering fixes the earlier timeout**, and no old assertion/timeout/retry policy was changed to obtain green.

## Handoff / boundaries

E3: requested local gates on final source passed (Node actual-source + Next/Go/PG with MOCK Graph). Independent K3 and required PR CI on the new head remain **NOT_RUN**, as do LIVE Meta/production and full foundation. Existing unrelated R04 runner is explicitly skipped by test-node because its binary env is unset.

Tracked pre-existing output evidence was not overwritten. Prior untracked CI downloads remain untouched. All owned commands have exited; no lock deletion or other agent process interruption. Integrator writes thread replies and runs K3/CI. CI gate: `--browser-live-console`, plus repository-selected required PR gates. Commit and stop; do not resume W3 in this batch.
