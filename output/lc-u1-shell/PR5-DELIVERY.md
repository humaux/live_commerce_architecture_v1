<!-- Purpose: Deliver the PR5 list-refresh/wait-budget fix and trunk conflict resolution without claiming unrun browser acceptance.
Depends on: live-console-v1 §2.6, source08a560c2, origin35abffca, prior CI37612856598 and owner commit-only instruction.
Used by: Integrator pushing PR5 and rerunning required CI; no production deployment. -->
# LC-U1 PR #5 — explicit list reads, bounded waits

- Branch/worktree: `unit/lc-u1-shell`, `/Volumes/data/live_commerce_architecture_v1/.worktrees/lc-u1-shell`.
- PR: https://github.com/luogangan7-lgtm/live_commerce_architecture_v1/pull/5 (base `r3/integration`).
- Merge **`a97f013c`** includes `origin/r3/integration` **35abffca**; no unresolved index entries. Both LC-U1 and W6-U2 BFF routes/imports, browser modes/build selectors and Node tests retained.
- Source **`08a560c2435bd45196870acfafe030577357f495`**. Final evidence-only commit preserves hashes below.
- Parent model not exposed; one inherited-model read-only explorer, explicit medium reasoning, no recursive delegation or other code writer.

## Root cause and fix

CI37612856598 reported the six real-click cases waiting at the old spec line185 for a list503 that cannot arrive: the contract-compliant review repair stopped automatic list polling. The spec armed a waiter, then changed the fixture, but took no action that reads the list.

1. Keep **only A1 periodic**. `LiveConsole`'s existing Refresh explicitly calls an optional parent list callback; `LiveWorkspace` passes `view.refresh`. Mount/manual refresh/command completion can read the list; no timer or new fetch writer added. Existing retained scene and component key remain, so transient list clear/503 does not discard the UNKNOWN owner. Auth denial concealment remains unchanged.
2. Spec first finishes `list_unavailable`, then `Promise.all` registers a **GET/exact pathname/status503** waiter and **actually clicks Refresh**. Checks retry visible and no new receipt. After `list_restore`, the same actual action waits for200 and checks the selected option's real title, retry still visible, A1 reads continue and receipt count stays unchanged. Existing same key/body/single-effect assertions are intact.
3. All explicit response waiters have **15,000ms** timeouts, well below110s test timeout. Popup and logout-URL waits are also bounded. No assertion, test timeout or harness700s budget relaxed.

## Red → green and checks

New checks execute actual JSX via React SSR, capture the real Refresh handler, check parent callback forwarding/supplied-scene retention, and AST-check every response/event/navigation waiter budget. They do not represent real-browser transition/hydration proof.

| Command | Exit / scope | Evidence |
| --- | --- | --- |
| `node --test --experimental-strip-types tests/admin/live-console-render.test.ts` before product/spec fix | **1**, 3 missing-wiring/unbounded-wait failures | `pr5-refresh-red.log` |
| Same command after fix | **0**, 4 PASS | `pr5-refresh-green.log` |
| `bash scripts/dev/test-node.sh` | **0**, **663 PASS / 0 FAIL**, summed runners | `pr5-node.log` |
| `pnpm --filter admin exec tsc --noEmit` | **0** | `pr5-tsc.log` |
| `LC_HEADER_BASE=35abffca bash scripts/dev/check-gates.sh` | **0**, **79 modes**, header ratchet | `pr5-check-gates.log` |
| `git diff --check`; `git ls-files -u`; ancestor check for origin | **0**, clean/no unresolved entries, ancestor present | author receipts |
| Existing-target Impeccable detector | **0**, `[]` | `pr5-ui-detector.json` |

Independent read-only source review found no P0/P1 in the four-file fix or union merge. No independent runtime rerun claimed. Local evidence **E3** for the above automated scoped checks, not full PR acceptance.

## Handoff / NOT_RUN

- Integrator pushes the delivered SHA to PR5; PR CI reruns. At least rerun **--browser-live-console** and the standing admin-shell/live-claims/studio-bff/studio-ui/click-sweep/visual-lint matrix; preserve W6-U2 operation gates. All focus/calibration flags unset.
- New-source browser/PG/full foundation/Linux/visual screenshots/provider SANDBOX/LIVE **NOT_RUN locally**. Optional R04 input runner **NOT_RUN** (`COMMERCE_R04_LIVEKIT_BINARY` unset), not counted as passing.
- No push, PR merge, deploy, live key/provider action, test-local/browser batch or production Go/SQL change. Local checks finished; no running background gate remains. Integrator owns CI dispatch/result notification; W2-U2 remains queued until acceptance.
- New PR CI run id is not yet known (no dispatch by this unit). `gh run view 37612856598` confirms head1fe069cc and **in_progress** at this check; the integrator reported its live-console mode failure. Do not treat that as a whole-run terminal verdict or a new-source result. Integrator owns this existing run; this unit did not cancel it.

| Source file | SHA256 |
| --- | --- |
| LiveConsole.tsx | `1c52a76c79732afabaf1f141904fa1c132c446967553cdd65231bcd637b2b2bb` |
| LiveWorkspace.tsx | `801e801825774a2d34f6d6c9f4d4844b90d7b7876f5c81f5d67f116ca36cfe19` |
| live-console.spec.ts | `a8df96e25ed6ab8cc16a9c4153f2a10fc5b7ba028ff17635015cd0143566d506` |
| live-console-render.test.ts | `040ba887e84c9059204cea2b408781828bfabc4de03904ece7c50d78ad481b3f` |
