<!-- Purpose: final W3-U3 author delivery after correcting diagnostic ownership and resolving the scope-read race.
Depends on: source58510545 merged with trunk729afff9 asd8744793, real Request/auth seams and serial browser/PG gates.
Used by: integrator K3 and PR preparation; local E3 is not production or independent acceptance. -->
# W3-U3 — READY (author E3)

**SUPERSEDED, not current READY:** integrator's 2026-10-10 LCN03 ruling moves the fix to Go SCOPE-404.
58510545 was exactly reverted in e4e48ec5. These logs/hashes remain historical candidate evidence;
current status and retained test design are in `../scope404-revert/DELIVERY.md`.

Branch `unit/w3-u3-comment-label-print`; tested source **d8744793a03a64281d4481d2bbe90ba5f2b2e98d**.
Fix **58510545**, trunk **729afff93aaedda81faf1d1badf44ac3bea9cd85**. Evidence-only delivery commit follows this source.
No push, merge into release, deployment, provider mutation, secrets or real customer data.

## Ownership correction

Prior wording attributing a diagnostic ruling to the owner/LC-U2a is withdrawn. Coordination task
**8f1bb58f-e09d-4ac0-a0b2-2b1b66fb8e17** was canceled as **误交接**: unchanged main-trunk
`b1bfbeb3` passes the same full gate (`trunk-control.log`, original main-checkout evidence
`output/integrator/triage/trunk-b1bfbeb3-live-console.log`). Integrator supplied two more green729afff9 controls.
W3-U3 diagnosed its own failure; no dependency or diagnostic responsibility remains assigned to LC-U2a.

## Root, not a polling workaround

The unchanged original ALL case failed in `hook-diagnostic.log` (workspace22/23, labels9/9):

1. BFF initially admitted the authenticated store. A13 then crossed the real PG grant revocation and answered403.
2. `hook-red/comment-scope-…c542…json`: network A13 starts463ms, ends403493ms. Its separate document-clock
   event sequence is child expiry → parent expiry → clear50rows → buyer removal → A2 cleanup/blocked.
   These two clocks are not conflated. Page stayed visible, cookie was present, and no A3 print request occurred.
3. Privacy clearing was correct. No later A2/A8 read was permitted. The original test waiter only accepts404,
   so it could never receive its awaited event after the403 had terminally cleared the scope.

Cause is the gap between `authenticatedStores` admission and the upstream read's authorization/definer check;
SQL0165 `inbox.buyer_panel` rechecks `identity.principal_holds`, and inbox transport maps PT403 to403.
We did not keep private data, delay expiry, revive denied polling, change the waiter, or blame LC-U2a.

Owner approved temporary safety-only hook observations and then the shared-BFF/seam repair. Both are direct
asynchronous approvals in this thread; no Go/SQL/DTO/contract change was authorised or made.

## Minimal repair and preserved boundaries

`apps/admin/app/api/stores/[store]/[...resource]/route.ts` reconciles only a validated private GET/403 forbidden.
`auth.ts` accepts an optional signal for the server-authorised store read; only explicit GET composes that
signal with its existing timeout. All command timeouts, bodies, keys, CSRF and replay semantics are unchanged.

| Observation | Result |
|---|---|
| Fresh same-bearer complete valid store list confirms target missing |404 not_found, no logout cookie deletion |
| Store still present; genuine permission denial |403 forbidden |
| Proof fails, times out, is canceled or malformed |Original403, never a fabricated missing scope |
| Upstream403 has unknown/non-contract code |Existing503 safe error, not a new accepted code |
| POST/write denial |Unchanged403; no proof or replay |
| Healthy GET |One admission and one private read, no extra scope request |

Proof gets **at most500ms**, and only the remaining part of **7s from BFF entry**, below the client's8s ceiling.
Caller cancellation reaches the proof. The fresh request never replays the private read or forwards browser
cookies/BFF key. An independent read-only source reviewer identified the original6s proof-delay P1; it was
fixed before final acceptance, with three additional red→green time/cancellation cases.

Temporary product hooks and workspace observations were removed byte-for-byte. `final-spec-parity.log`
and `FINAL-SOURCE-SHA256.txt` bind both hooks, the entire original workspace spec and original label spec.
The latter remains `c5f1b3f7a685c1fd284cc03416f6b96cdaf509f71f114acb19bf3bf3bd12b156`;
workspace spec remains `3df8c6fa47d7e559ff535ed7f146d3dec307ce4d973f66ab65906216b4d1182b`.
All411 original assertion/wait calls also stayed identical during observation (`unchanged-assertions.log`).

## Actual commands / exits

All final runs used d8744793; source was frozen and PG/browser modes were strictly serial with the current heartbeat lock.

| Command | Exit | Observed result / file |
|---|---:|---|
| `LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-local.sh --browser-live-console` |0|Workspace23/23 +labels9/9;357.30s; `final-browser.log` |
| `LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-local.sh --browser-inbox` |0|13/13;35.62s; `final-inbox.log`, `final-inbox-results.txt` |
| `bash scripts/dev/test-node.sh` |0|1390 executions,1390 PASS,0 FAIL; `acceptance-node.log` |
| `pnpm --filter admin exec tsc --noEmit` |0|`acceptance-tsc.log` |
| `bash scripts/dev/check-gates.sh` |0|82 modes all documented;1256 Go inventory; header ratchet PASS; `acceptance-gates.log` |
| `node --test --experimental-strip-types tests/admin/inbox-bff.test.ts` |0|27/27 real-handler seam cases,14 new; `bff-full-green.log` |
| `node --test --experimental-strip-types --test-name-pattern='W3U3' tests/admin/inbox-bff.test.ts` |0|13/13 intermediate focused cases, `bff-budget-green.log`; final14 covered by full27-case run |
| `bash scripts/dev/test-local.sh --list` |0|`final-modes.txt`, registry preserved |
| `node scripts/dev/pr-modes.mjs origin/r3/integration HEAD` |0|53 selected modes; `final-pr-modes.json` |
| `git diff --quiet baab02a1 -- <original specs and hooks>` |0|Byte-identical; `final-spec-parity.log` |

Red evidence is retained, not skipped or relaxed:

- Required **two unchanged** baab02a1 runs: `run-1.log` exit0 (23+9), `run-2.log` exit1 (22+9/private wait timeout).
- `bff-red.log`: exit1,8 failing/2 protective passing new seam cases on pre-fix source; notably403≠404.
- `bff-budget-red.log`: exit1,3 failing new cases on the unbounded-proof intermediate fix
  (stalled proof6.15s, cancellation6.0s, late-budget redundant proof).
- `hook-diagnostic.log`: exit1, captured ALL403 expiry chain; original operations/wait/assert/timeout unchanged.
- Two earlier read-only-observed full modes passed but were **not claimed as a root fix**:
  `diagnostic-full.log` exit0 (23+9), `diagnostic-full-2.log` exit0 (23+9).
- `diagnostic.log` filtered LCU2_404 workspace2/2 passed, but labels had zero selected cases and wrapper exit1:
  **diagnostic only**, not full acceptance.
- The deadline branch uses a mocked monotonic clock, not evidence of an actual7s elapsed interval.
  Stall/abort cancellation uses the real HTTP edge and awaits connection closure; each new timing case has a10s outer timeout.

## Current E3 print / privacy evidence

`final-evidence/`: **18 screenshots** (zh-TW/zh-CN/en ×390/1586 ×preview/A4/small) and **9 case ledgers**,
**45 observed rows /45 PASS**. Actual values come from DOM/request counts, geometry and reload reads, not copied expectations.
The unchanged lost-ACK case records `sameKey:true, counts:[7,7], sameCount:true, attempts:2, printCalls:2`.
Both `grant-revocation-{all,private}.json` record real404, privateRows0, buyerPanel0, sessionRetained:true,
postLossReads0. Native print invocation is intercepted; physical printer success is not claimed.

Full ignored runtime directories: `output/playwright/run.QxGoOlvf/live-console-1405013660` (workspace),
`live-console-4063377645` (labels); inbox `output/playwright/inbox-ui/20261009T191831.950581000`.
Selected safe evidence is committed; private automatic artifacts remain governed by the existing guard.
Author viewed current zh-TW390 and en1586 previews; this is self-QA, not independent visual review.

## Roles, indexing and remaining gates

Primary author Codex-1; actual runtime model/effort unavailable. Read-only reviewers: w3_merge_audit
gpt-6.1-sol/medium; w3_fixture_lifecycle explorer and w3_bff_scope_review security reviewer
gpt-6.1-sol/high. Source review E1 found no further blocking P0/P1 after the budget fix; it did not rerun gates
and is not a substitute for integrator K3. Existing framework/state/authorization seams were reused, no new UI library.

Humaux code index:3 files/78 entities, done; `authenticatedStores` linked to fix memory
`cb978fb2-7702-4781-8280-617cab93f842`. The index lock was released after indexing.

### CI gates / NOT_RUN

- Current-head non-author K3 and GitHub required PR checks: **NOT_RUN**, integrator next; CI priority#29/#33, no author push.
- Required mode selection (53 exact modes) is `final-pr-modes.json`; local live-console/inbox results above do
  not substitute for that CI set. Includes `--browser-live-console`, `--browser-inbox`, `--browser-click-sweep`,
  `--browser-visual-lint`; fresh global click-sweep/visual-lint **NOT_RUN locally**.
- Full foundation/G07: **NOT_RUN in this UI/BFF batch**; no migration, SQL or product-Go change.
- Physical printer/OS dialog/hardware pagination, LIVE Meta/production: **NOT_RUN**.
- No foreign `output/ext-agents/` staged or modified. All owned gate sessions, fixtures and servers ended before handoff.

**READY = tested author E3 handoff, not merged/released/E4.** Integrator performs K3, pushes after the priority PRs,
and opens/accepts the PR. Prior failures and the A3 historical7→8 red are retained.
