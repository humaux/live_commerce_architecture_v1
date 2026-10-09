<!-- Purpose: PR18 round-two scoped404 privacy fix, causal evidence and integrator handoff.
Depends on: comment4226524077, existing inbox privacy/fence semantics and real scoped Go/PG reads.
Used by: independent review and PR18 CI; author E3 evidence only, no production claim. -->
# PR #18 round 2 — privacy P1 only

- Branch `unit/lc-u2a-comment-stream`; fetched/merged own remote and trunk, both already up to date at `89499013`.
- Tested source **`e0aa3960e446f0b4337ede7b37e7f3b32680ca74`**; final delivery commit is evidence-only.
- Codex-1; runtime model ID unavailable; no delegates. **The three round-two P2s are deferred and untouched.**

## Root fix

The comment loaders recognized401/403 as authority loss but retained private state on scoped404. Match the standalone inbox: A2, the A8 head loader and A8 history loader now invoke the existing `privacy.expire()` on404 too. That path immediately clears the A2 buffer, selection and related A8 state, invalidates in-flight tickets, prevents polling/focus revival, and unmounts BuyerPanel. No new recovery/writer, product Go, migration, schema or permission change.

## Red → green

- Actual-source Node counterexamples for A2/A8/history404: **3 fail →3 pass** (`node-red.log`, `node-green.log`). They preload names/text and BuyerPanel, assert private state is actually empty (not merely hidden), then advance90s and focus without permitting new reads.
- Browser red on old product: real signed login, real PG `store:read` grant removal for this fixture's principal/store, unchanged session, actual BFF404; both `all` and `private` views remained enabled/private. **2 browser failures, exit1** (`browser-red.log`, `evidence/browser-red.log`). No response interception or DOM mutation.
- The fixture extension is **Go TEST only**. Protected loopback setup deletes/restores one scoped synthetic grant row; a `finally` restores it even on assertion failure. The product permission path is unchanged.
- Final real-click tests confirm scoped404 clears comment rows, conversation rows, comment author/text and BuyerPanel; the signed session cookie is retained. After restoring the grant, the expired page stays locked with0 further reads over two A2 intervals; an explicit reload authorizes fresh data. Safe records contain counts/booleans only (`grant-revocation-all.json`, `grant-revocation-private.json`).

## Final gates on e0aa3960

| Command | Exit | Evidence |
|---|---:|---|
| `node --test --experimental-strip-types --test-name-pattern='PR18 R2 scoped404' tests/admin/comment-stream-hooks.test.ts` | 0 | `node-green.log`,3/3 |
| `bash scripts/dev/test-node.sh` | 0 | `node-final.log`,1179 tests,0 failures |
| `pnpm --filter admin exec tsc --noEmit` | 0 | `tsc.log` |
| `bash scripts/dev/check-gates.sh` | 0 | `gates.log`,82 modes/all documented |
| `LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-local.sh --browser-live-console` | 0 | `browser-live-console.log`,**22/22** (original20 plus two grant-revocation cases) |
| `LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-local.sh --browser-inbox` | 0 | `browser-inbox.log`,**13/13** |

Focused browser red command: `LC_TEST_LOCK_WAIT=14400 LC_FOCUSED_TAGS=browser LC_BROWSER_LIVE_CONSOLE_ACCEPTANCE=1 LC_BROWSER_CONSOLE_GREP=LCU2_404 bash scripts/dev/test-focused.sh '^TestBrowserLiveConsoleRealChain$'` →exit1; this red run is not full-mode acceptance.

All PG/browser runs were strictly serial and source remained fixed. `SOURCE-SHA256.txt` binds the five changed files. Original artifacts: `output/playwright/live-console-2374756194` (red), `live-console-289495699` (green), and `inbox-ui/20261009T043311.975658000`. Only safe excerpts were committed. Existing tracked `output/` files were not modified by these runs (`git diff --exit-code -- output` =0), so no restore was necessary; prior local CI downloads remain untouched and unstaged.

## Handoff

Requested local batch complete at **E3 (MOCK Graph + real Next/Go/PG)**. All owned commands exited; no push/deploy or real customer/provider operation.

NOT_RUN: required PR CI and independent review on the new head, full foundation/G07, LIVE Meta/production. Unrelated R04 media runner remains explicitly NOT_RUN because its binary env is unset. Three P2s remain FOLLOWUPS.

W3-U3 was safely checkpointed at **`2a3a7864a2a3bd806cc27e9ff67d00fa213c8884`** before this priority change: approved React host/isolation tests, original146 assertions and40-test order unchanged, Node/type/gates green; its full live-console+label run is still pending. Resume it after this handoff, then settings-followups.
