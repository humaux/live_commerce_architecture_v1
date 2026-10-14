# CI-DRIFT-FN — author record

Status: tests-first revision; implementation and acceptance pending. This file is the author's record, not independent evidence.

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

## Acceptance and NOT_RUN

New revision CI red: pending. Implementation: NOT_RUN. Green 27 strict cases, full integration CI-DRIFT inventory, current-head required checks and independent full review: NOT_RUN. All local tests, formatting tools, builds and application runs: NOT_RUN by owner CI-only directive. No services or local fixtures were started. No production, provider LIVE, deployment or payment work is in scope.

CI must first reproduce the three failures, then the minimal widening fix must make all 27 strict cases pass. Any newly flagged integration routes must be listed rather than baselined. CI runs, checkout hashes, commands and receipts will be recorded after execution. Foundation timeout 90 minutes; gate matrix 150 minutes, measured from actual execution start. Existing ten-minute heartbeat is the wakeup mechanism.
