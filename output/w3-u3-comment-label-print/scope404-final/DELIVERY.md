<!-- Purpose: final W3-U3 post-PR36 acceptance, current evidence and explicitly untested boundaries.
Depends on: merged PR30/PR36, frozen source4f0e08e5, original specs and serial local acceptance.
Used by: integrator independent review and PR preparation; not production release approval. -->
# W3-U3 — READY, author E3 (2026-10-10)

Tested source: **4f0e08e5003838d2746ce7dc9c77d1cccf7471e3**, branch unit/w3-u3-comment-label-print.
The final handoff is an evidence-only descendant; apps/, tests/ and scripts/dev/ remain identical to that source.
Author: Codex-1 (root runtime model/effort not exposed). One read-only test_worker, gpt-6.1-sol / medium,
verified existing artifacts and source parity; that is E1 corroboration, **not** independent K3 or rerun evidence.

## Integration and scope

- GH #36 MERGED 2026-10-10T05:12:55Z, merge **287e08aa6c694030333dd00797c0d3bbac15abc6**.
  Git ancestor exit0; own no-conflict merge **22ef3ac2a2b5263c9e2add87a34a03b154953614**.
- Exact BFF revert **e4e48ec581c8be7309521f20dc4497bee9ca08cc** remains an ancestor.
  auth.ts, store catchall and inbox-bff.test.ts equal that trunk baseline; rejected58510545 re-proof is absent.
- Accepted print/stream/CSS/P2 files and both complete browser specs equal0b153c90.
  Workspace SHA256 **3df8c6fa47d7e559ff535ed7f146d3dec307ce4d973f66ab65906216b4d1182b**;
  label SHA256 **c5f1b3f7a685c1fd284cc03416f6b96cdaf509f71f114acb19bf3bf3bd12b156**.
  This is not a claim that all feature files equal trunk: approved label UI remains the unit's diff.
- Single native mode registry retained. No new Go/SQL/DTO/contract repair, product change, test operation,
  waiter, assertion, threshold or harness timeout change in this verification round.
- Owner explicitly approved **this round's local serial browser exception** in reply to call20454e5f.
  Standing GitHub-only heavy-gate policy still applies to other rounds/units. No push/deploy.

## Commands and actual results

Browser prefix: LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-local.sh.
Five actual invocations used one frozen source and ran strictly serially, not five copies of one result.

| Command / run | Exit | Observed evidence |
|---|---:|---|
| bash scripts/dev/test-node.sh |0|1376/1376, 27 summaries, fail/skip/cancel0; node.log, node-counts.log |
| pnpm --filter admin exec tsc --noEmit |0|tsc.log |
| bash scripts/dev/check-gates.sh |0|82 documented modes, inventory1262, headers; gates.log |
| go run ./scripts/dev/contractdrift |0|**errors=0**, warnings362; contractdrift.log |
| --browser-live-console, console-1 |0|23 workspace +9 label cases; native361.72s |
| --browser-live-console, console-2 |0|23 workspace +9 label cases; native348.06s |
| --browser-inbox |0|13 cases; 31 observed ledger rows; native31.08s |
| --browser-click-sweep |0|147 page/viewport/locale units, load failures0;1135 PASS /0 FAIL /31 SKIP;18 journey steps PASS; native3071.10s |
| --browser-visual-lint |0|342/342 screenshots, missing/NOT_RUN0, blocking0; **R5=21/R7=112 warnings retained**; native777.34s |

Static checks ran on22ef; its runtime/test bytes equal4f. They do not substitute for real browser results.
Post-document check-gates and contractdrift are recorded in handoff-gates.log and handoff-contractdrift.log;
their actual exit receipts are indexed in evidence/MANIFEST.md.

Original command logs, PIDs, start/end UTC, source, exits and signals:
local-acceptance/restart-20261010T055908059Z/ (state.json, supervisor.log, five mode logs).
Supervisor34197 finished 2026-10-10T07:30:00.364Z, aggregate exit0; all owned child PIDs ended.
Console retains780s Go timeout; inbox retains540s; sweep/lint retain5400s.
Supervisor go_budget_seconds:1200 inbox metadata is an outer-envelope estimate only, **not** actual Go timeout;
the unchanged registry is authoritative. No inner timeout was increased.

## Two original scope-revocation runs and A3 replay

| Run | Started → ended UTC | Workspace artifact | Labels artifact |
|---|---|---|---|
| console-1 /PID34207 |05:59:08.199 →06:09:58.817|output/playwright/run.20izueha/live-console-4018777712|output/playwright/run.20izueha/live-console-808004645|
| console-2 /PID43721 |06:09:58.870 →06:21:33.802|output/playwright/run.9LPrSopL/live-console-2930805195|output/playwright/run.9LPrSopL/live-console-286510693|

Original LCU2_404 all/private cases pass in **each** run. All four raw JSON observations:
REAL_PG status404, privateRows0, buyerPanel0, sessionRetained=true, postLossReads0.
Grant restoration and original operation/wait/assertion sequence are unmodified.

Both unchanged lost-ACK cases pass: attempts2, sameKey=true, counts **[7,7]**, sameCount=true.
Each label run: **9 files,45 independently observed rows,45 PASS**, not copied expected values.
Native window.print is counted; no physical printer or OS-dialog acceptance is implied.
Latest18 screenshots: zh-TW/zh-CN/en ×1586×992/390×844 ×preview/A4/small.
Selected safe native reports/ledgers/screenshots are copied byte-for-byte into evidence/;
evidence/SHA256SUMS.txt binds the copy, evidence/MANIFEST.md lists full native paths.

Other native artifacts remain in this worktree:
- Inbox: output/playwright/inbox-ui/20261010T062236.231619000/.
- Sweep: output/playwright/run.M88wYZH4/ui-click-sweep/; runner log under click-sweep/20261010T062402.507575000/.
- Lint: output/playwright/run.Wzs7RSvr/ui-visual-audit/20261010T071704Z/.

## Retained history, CI gates and NOT_RUN

Foreground session81620 was **ABORTED by chat interruption**, with no full exit receipt.
Its partial local-acceptance/console-1.log is preserved, not counted as a third PASS or product failure.
Detached bounded supervisor restarted into a fresh directory; no duplicate active mode or foreign PID/lock was killed.
Earlier A3 counts7→8, LCU2_404 timeout reds and rejected BFF evidence remain historical, never relabeled PASS.

Required PR CI remains repository-selected; integrator recomputes pr-modes.mjs on the pushed PR.
Current local results do **not** reduce that plan. Explicit affected modes: live-console, inbox, click-sweep, visual-lint.
Current independent K3 / required PR CI: **NOT_RUN**. Full G07: **NOT_RUN** (no authored migration/GRANT/backend repair).
Physical printer/OS dialog/paper pagination, LIVE Meta or other providers, production/deployment: **NOT_RUN**.
MOCK Graph + REAL_PG + native real-click browser E3 applies only to the tested paths/environment.

Exact coordination task **dbd03f8b-e0f2-4f2e-8ad9-a6f7e03efd0c**, agent codex-w3-u3.
No source/process remains in progress. Foreign output/ext-agents/ is untouched and unstaged.
Commit only; integrator opens the PR and performs independent review/push. READY is not release approval.
