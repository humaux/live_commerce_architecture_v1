<!-- Purpose: PR18 thread4226683270 non-terminal public-reply guard and honest gate evidence.
Depends on: integrator packet pr18-5f888bca-thread-4226683270.md and live-console-v1 section4.4.
Used by: integrator K3 pre-review and PR18 CI; author evidence, not merge approval. -->
# PR18 queued public guard

- Base `5f888bca`; fetched/merged `origin/unit/lc-u2a-comment-stream` (already up to date).
- Tested source `0814e97edf7882a3adfee5b1615714709dd91b08`; final delivery commit is evidence only.
- Codex-1, no delegates. Runtime model ID unavailable. Worktree `lc-u2a-comment-stream` only. W3 remains checkpointed.
- **Fix implemented; full browser gate is NOT all green.** No push or deployment.

## Root cause and fix

An acknowledged `queued` operation is not terminal. Clearing the coarse receipt lost all public-send evidence on unmount because A2 returns only public counts. Following preferred option(a), reuse `CommentReceipt` with an optional public-ref key: store + live session + authentication boundary + encoded comment ref. Store only `"1"`, never buyer name, reply text, credentials or operation payload. SessionStorage is tab-scoped; this is not a new cross-tab/server dedupe guarantee.

Arm before POST. Keep the per-comment flag for queued/unknown and transport uncertainty. Clear only on a definite terminal response (sent/failed/blocked), a definite command refusal, or the existing explicit merchant verification confirmation. No timer clears it. Other comments remain usable after queued; existing coarse UNKNOWN protection remains. A2 cannot establish the eventual terminal result of an acknowledged public operation, so after reload manual verification is deliberately required. No automatic resend, new library, API or migration.

Product edits: `CommentReply.tsx`, `comment-receipt.ts`. Go changes are TEST fixtures only: existing MOCK Graph5xx fault plus scoped UNKNOWN count. Real planner/worker writes the operation state; no fake UI/POST response and no product Go/SQL changes. Other review threads untouched.

## Red → green and click ledger

- Actual-source Node guard cases on old product: 2 FAIL, exit1 (`red.log`); missing durable guard. Same two then pass (`green.log`); plus four terminal/refusal cases, 6/6 (`focused-green.log`). Original cross-comment assertion remains: only coarse UNKNOWN blocks other comments. Storage-count expectations intentionally change to include the newly approved per-comment flag.
- Node: queued/UNKNOWN → dispose → other comment → original → dispose/reload → fake clock +31s → still disabled, no second POST → explicit verification clears flags. At every POST the flag is already persisted; terminal/refusal cases clear it.
- Browser new case: real click public mode/send → ACK queued → real worker receives MOCK Graph5xx → scoped SQL UNKNOWN count increases; other comment public composer is enabled; return/reload and wait31 real seconds → original disabled, request count unchanged; click Verified/accept dialog → composer enabled without automatic send. This case passed in the full run (only failing case was the older scoped404-private test).

| Control | Real action and assertion | Result |
|---|---|---|
| Public reply / Send | Typed synthetic text; ACK queued; worker UNKNOWN | PASS |
| Other comment / Public reply | Selected a different comment and typed; Send enabled | PASS |
| Original comment / reload | After31s Send disabled; no duplicate POST | PASS |
| Verified | Click + native confirmation; Send enabled, no automatic POST | PASS |

## Commands and exits

| Command | Exit | Evidence |
|---|---:|---|
| `node --test --experimental-strip-types --test-name-pattern='public receipt has' tests/admin/comment-stream-hooks.test.ts` (red) | 1 | `red.log`,2 FAIL |
| Same (green) | 0 | `green.log`,2 PASS |
| `node --test --experimental-strip-types --test-name-pattern='public (receipt\|guard)' tests/admin/comment-stream-hooks.test.ts` | 0 | `focused-green.log`,6 PASS |
| `bash scripts/dev/test-node.sh` | 0 | `node.log`,1185 tests/0 failures |
| `pnpm --filter admin exec tsc --noEmit` | 0 | `tsc.log` |
| `bash scripts/dev/check-gates.sh` | 0 | `gates.log`,82 modes/all documented |
| `LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-local.sh --browser-live-console` | **1** | `browser.log`,**22 PASS / 1 FAIL**,331.23s |
| `LC_TEST_LOCK_WAIT=14400 LC_FOCUSED_TAGS=browser LC_BROWSER_LIVE_CONSOLE_ACCEPTANCE=1 LC_BROWSER_CONSOLE_GREP=LCU2_404 bash scripts/dev/test-focused.sh '^TestBrowserLiveConsoleRealChain$'` | 0 | `scoped404-diagnostic.log`,2 browser cases; diagnostic only |

Full run evidence: `output/playwright/live-console-4154192057/`. Sole failure: existing `LCU2_404 real store grant revoked clears private view and selected buyer`, spec195:48 / wait at207, waiting15s for a scoped404 response. Node/type/gates and new real worker regression pass. The two unchanged scoped404 tests passed alone in `output/playwright/live-console-396746924/` (28.15s). **The isolation result does not erase or replace the full-run failure; its root cause is not established.** No timeout, retry setting or old assertion changed. Integrator must triage/recheck this before claiming full acceptance.

## Handoff

E3 for the new guard regression (actual source Node; real Next/Go/PG + MOCK Graph browser). Full-mode acceptance remains RED; independent K3/PR CI on new source NOT_RUN. No LIVE Meta, production or full foundation run. All owned commands exited, PG runs serial; existing tracked `output/` remained unchanged and prior untracked CI artifacts were left alone. No cleanup of other agents' work, push or deployment.

CI gates: `--browser-live-console`; normal required PR gates selected by the repository. Please retain and investigate the scoped404 full-run failure instead of describing this batch as all-green. Commit-and-stop; W3-U3 remains queued.
