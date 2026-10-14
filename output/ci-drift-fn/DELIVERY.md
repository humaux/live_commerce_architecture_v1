# CI-DRIFT-FN — author record

Status: genuine CI red reproduced; widening implementation awaiting CI green and independent acceptance. This file is the author's record, not independent evidence.

Owner scope: close CDF-1 (route edit parent MOUNT attribution), CDF-2 (middleware dependency), and CDF-3 (handler body dependency) only by widening detection. The whole-factory-span rule stays intact. False positives are acceptable; no existing case may be weakened. No baseline TOUCHED entries will be added.

## Provenance

| Participant | Role | Assigned model / reasoning | Base | Worktree / branch | Write paths |
| --- | --- | --- | --- | --- | --- |
| Author `/root` | integration | gpt-6.1-sol / ultra | e7421a10bda4046a9c151ac8babc2f61672a3ae3 | `/Volumes/data/live_commerce_architecture_v1/.worktrees/ci-drift-fn`, `unit/ci-drift-fn` | `scripts/dev/contractdrift/**`, `output/ci-drift-fn/**`; CDF cleanup in `output/ci-drift-factory/FOLLOWUPS.md` |
| `/root/fn_widening_audit` | read-only explorer | gpt-6.1-sol / high | e7421a10bda4046a9c151ac8babc2f61672a3ae3 | same worktree, read-only; no branch writes | no repository writes |

Model and reasoning are the actual parent assignment, not independently verified provider telemetry. No writer shares this worktree.

## Tests-first change

Remove the exact expected-failure helper and route the three existing candidate fixtures through the same strict TOUCHED assertions as the other 24 cases. Fixture text, baseline coverage assertions, route counts, provenance checks and failure exit requirements are unchanged. Scanner implementation is unchanged in this revision. CDF FOLLOWUPS remain open until actual fix evidence exists.

PR54 merged as e7421a10 after owner independent PASS at 2174ea14. Historical valid red: Foundation [38487723575](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38487723575), Gates [38487723574](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38487723574), on b02134b8; portable receipt [trunk-candidate-red.log](../ci-drift-factory/evidence/trunk-candidate-red.log). This is historical evidence, not this unit's current CI result.

## CI red and widening change

Foundation [38497057944](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38497057944), job 115635121440, failed on tests-first head `4e63b465dfdd0d86cc4d8d1f32bd66622416b058`. Actual checkout `4afe1cc728cd20aa96844332f1973282c500e3bb` merges that head into base `e7421a10bda4046a9c151ac8babc2f61672a3ae3`. The three failures match CDF-1..3: route edit rejects EXACT but misses MOUNT; middleware and handler-body dependency edits exit 0. See [portable red receipt](evidence/red-foundation.log). The non-verbose log lists the three failures; individual green lines for the other 24 cases remain pending.

The implementation changes only `go_opaque.go`: retain the whole span of every dependency already visited by the existing traversal, then propagate existing opaque child spans to MOUNT findings as well as EXACT/PREFIX. The whole factory span, traversal edges, routing dispatch guard, global path references, finding identities and baseline logic remain. Removed `signal` bookkeeping no longer controls whether a visited body is retained. This widens provenance and can conservatively flag sibling mounts; it adds no route-coverage inference or probe exclusion.

Read-only `/root/fn_widening_audit` reviewed source blob `eaff94abe4f2d781c9f98e9e714fa0a087cc7f52` and reported no actionable P0/P1 or narrowing. This is E1 source review, not runtime or integrator acceptance.

The unchanged-scanner tests-first plan job 115635121664 in Gates [38497057966](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38497057966) completed its full inventory with `errors=0` (Go 280 / contract 148 / BFF 301). That proves an empty actual TOUCHED set for the tests-first revision only. The fixed scanner's full integration inventory and every newly TOUCHED route must be reported from its own CI receipt; no TOUCHED baseline is permitted.

## Acceptance and NOT_RUN

New revision CI red: reproduced as above. Fixed implementation runtime: NOT_RUN. Green 27 strict cases, fixed-scanner full integration CI-DRIFT inventory, current-head required checks and independent full review: NOT_RUN. CDF FOLLOWUPS remain open until green proof. All local tests, formatting tools, builds and application runs: NOT_RUN by owner CI-only directive. No services or local fixtures were started. No production, provider LIVE, deployment or payment work is in scope.

CI must first reproduce the three failures, then the minimal widening fix must make all 27 strict cases pass. Any newly flagged integration routes must be listed rather than baselined. CI runs, checkout hashes, commands and receipts will be recorded after execution. Foundation timeout 90 minutes; gate matrix 150 minutes, measured from actual execution start. Existing ten-minute heartbeat is the wakeup mechanism.
