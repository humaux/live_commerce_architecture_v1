# CI-DRIFT-FN — author record

Status: original three local fixtures pass CI after the widening change; adversarial review follow-up is in progress. Scanner source is frozen. Three structural counter-examples require their own CI receipts and exact expected-fail signatures. Required current-head checks and integrator delta acceptance remain pending. This file is the author's record, not independent evidence.

Owner scope: close CDF-1 (route edit parent MOUNT attribution), CDF-2 (middleware dependency), and CDF-3 (handler body dependency) only by widening detection. The whole-factory-span rule stays intact. False positives are acceptable; no existing case may be weakened. No baseline TOUCHED entries will be added.

## Provenance

| Participant | Role | Assigned model / reasoning | Base | Worktree / branch | Write paths |
| --- | --- | --- | --- | --- | --- |
| Author `/root` | integration | gpt-6.1-sol / ultra | e7421a10bda4046a9c151ac8babc2f61672a3ae3 | `/Volumes/data/live_commerce_architecture_v1/.worktrees/ci-drift-fn`, `unit/ci-drift-fn` | `scripts/dev/contractdrift/**`, `output/ci-drift-fn/**`; CDF cleanup in `output/ci-drift-factory/FOLLOWUPS.md` |
| `/root/fn_widening_audit` | read-only explorer | gpt-6.1-sol / high | e7421a10bda4046a9c151ac8babc2f61672a3ae3 | same worktree, read-only; no branch writes | no repository writes |
| `/root/fn_review_regressions` | test_worker | gpt-6.1-sol / high | b8fc5d267431d243e6d1bdde3f3bd432bb736836 | `/Volumes/data/live_commerce_architecture_v1/.worktrees/ci-drift-fn-regressions`, `unit/ci-drift-fn-regressions` | `scripts/dev/contractdrift/go_factory_trunk_cli_test.go`, `scripts/dev/contractdrift/go_factory_review_cli_test.go` only |

Model and reasoning are the actual parent assignment, not independently verified provider telemetry. No writer shares this worktree.

## Tests-first change

The initial tests-first revision removed the exact expected-failure helper and routed the three existing candidate fixtures through strict TOUCHED assertions. Fixture text, baseline coverage assertions, route counts, provenance checks and failure exit requirements were unchanged. Scanner implementation was unchanged in that revision. The subsequent review follow-up makes these three candidates compare the complete five-line CLI output and exit code, including the generic dispatcher, MOUNT, EXACT and errors=3/warnings=0 totals. The other 24 cases are unchanged.

PR54 merged as e7421a10 after owner independent PASS at 2174ea14. Historical valid red: Foundation [38487723575](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38487723575), Gates [38487723574](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38487723574), on b02134b8; portable receipt [trunk-candidate-red.log](../ci-drift-factory/evidence/trunk-candidate-red.log). This is historical evidence, not this unit's current CI result.

## CI red and widening change

Foundation [38497057944](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38497057944), job 115635121440, failed on tests-first head `4e63b465dfdd0d86cc4d8d1f32bd66622416b058`. Actual checkout `4afe1cc728cd20aa96844332f1973282c500e3bb` merges that head into base `e7421a10bda4046a9c151ac8babc2f61672a3ae3`. The three failures match CDF-1..3: route edit rejects EXACT but misses MOUNT; middleware and handler-body dependency edits exit 0. See [portable red receipt](evidence/red-foundation.log). The non-verbose log lists the three failures; individual green lines for the other 24 cases remain pending.

The implementation changes only `go_opaque.go`: retain the whole span of every dependency already visited by the existing traversal, then propagate existing opaque child spans to MOUNT findings as well as EXACT/PREFIX. The whole factory span, traversal edges, routing dispatch guard, global path references, finding identities and baseline logic remain. Removed `signal` bookkeeping no longer controls whether a visited body is retained. This widens provenance and can conservatively flag sibling mounts; it adds no route-coverage inference or probe exclusion.

Read-only `/root/fn_widening_audit` reviewed source blob `eaff94abe4f2d781c9f98e9e714fa0a087cc7f52` and reported no actionable P0/P1 or narrowing. This is E1 source review, not runtime or integrator acceptance.

The unchanged-scanner tests-first plan job 115635121664 in Gates [38497057966](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38497057966) completed its full inventory with `errors=0` (Go 280 / contract 148 / BFF 301). That proves an empty actual TOUCHED set for the tests-first revision only. The fixed scanner's full integration inventory and every newly TOUCHED route must be reported from its own CI receipt; no TOUCHED baseline is permitted.

## Fixed-scanner CI and full integration inventory

Foundation [38498598779 / job 115639941947](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38498598779/job/115639941947) succeeded on head `b8fc5d267431d243e6d1bdde3f3bd432bb736836`, actual execution 01:59:27–02:04:27 UTC on 2026-10-14. The contractdrift package passed in 126.141 seconds without expected-fail entries for the original three cases. This non-verbose result does not expose individual green test rows.

The actual checkout was `71cfaea3f789f4c9f40923c47a07ff35e69d7e72`, merging b8fc into `e7421a10bda4046a9c151ac8babc2f61672a3ae3`. A fresh origin fetch at 02:06 UTC still resolved `r3/integration` to that base. The full unfiltered scanner inventory in Foundation, independently corroborated by Gates [38498598892 / plan 115640735728](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38498598892/job/115640735728), ended with:

```text
SUMMARY routes go=280 contract=148 bff=301; errors=0 warnings=362 CONTRACT_ONLY=33 GO_405_FALLBACK=4 GO_MOUNT_OR_FALLBACK=89 GO_NOT_IN_BFF=16 GO_ONLY=200 METHOD_MISMATCH=4 UNRESOLVED=125
```

Every newly reported ERROR row: **none**. Every newly reported TOUCHED row: **none**. [Full portable inventory receipt](evidence/fix-b8fc-integration-inventory.log), SHA256 `364acef64f2376edb9121278113b2c080b6bd092c079761ab0733eab4bb272a3`. The receipt includes every scanner row through SUMMARY, not a sampled or capped excerpt. No baseline was added or modified. This source/output-only PR produces no actual newly touched API routes in that snapshot; the result does not quantify potential future dependency-span expansion or certify general scanner soundness. A later revision needs its own exact-head inventory result.

Scanner blob: `eaff94abe4f2d781c9f98e9e714fa0a087cc7f52`; SHA256 `23ed435aec460e5ce54ccc0de49dd5d154818fcb03f53225b0d5be8ba33d6a00`. Foundation raw-log hash: `74060a4eb95d1c31cb1724ae941f98dd1fd0eab01e2d7a344634bb5d7980f950`. Plan raw-log hash: `b7fd65915a212aa4e90c62d49523ff8e1829059eef1dbc66d666c020e558add8`.

## Adversarial review follow-up

The Opus review at b8fc found no narrowing, but identified cross-package middleware, non-path globals and split-file mount attribution limits. The owner explicitly deferred scanner redesign. Scanner source remains exactly b8fc; the isolated test worker's `edb31f108c0456a1e55a33f76ae581485ee34c89` adds strict CDF-4..6 fixtures first and exact CDF-1..3 assertions. Tests and fixtures have not been executed locally. CI must first establish the new failures with valid baselines; only then may exact output/exit signatures become expected-fail entries. UNEXPECTED_PASS and any unknown or setup failure must fail hard.

The old three local-fixture FOLLOWUPS rows are retired at their source. New open CDF-4..6 entries live in [FOLLOWUPS.md](FOLLOWUPS.md). Separate research coord task `4a0c4279-fe4f-4cce-b426-9557bbd086a4` tracks the structural design limits; it authorizes research only, with no scanner patch in this PR.

## Acceptance and NOT_RUN

The original red and b8fc package green/full inventory are CI evidence tied to those source hashes. New exact-output assertions, new structural fixtures, eventual exact expected-fail signatures and new current-head required checks: **NOT_RUN** until their own CI executes. The b8fc full Gates round was still pending when its package/inventory receipt was preserved. The author's E1 source review and the Opus adversarial findings do not substitute for integrator delta approval.

All local tests, formatting tools, builds and application runs: **NOT_RUN** by owner CI-only directive. No services or local fixtures were started. No production, provider LIVE, deployment or payment work is in scope. No baseline or timeout/threshold changes are permitted. Foundation timeout is 90 minutes; matrix jobs 150, plan/aggregate 10, required 5 and browser installation 15 minutes, measured from actual execution start. The existing ten-minute heartbeat registers runs and wakes this unit; no duplicate workflow is dispatched.
