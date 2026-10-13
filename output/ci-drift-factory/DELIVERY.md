# CI-DRIFT-FACTORY — regression tests only

作者自述记录。Independent acceptance of the final tests-only revision is pending; earlier source reviews do not accept this revision.

## Owner decision and scope

The owner selected option (a) on 2026-10-14: Codex-3 moves OA-1's database probe into a separate DSN-level preflight called from main, leaving the buyer factory body unchanged. That product change belongs to OA-1 / PR #53 and is not implemented in this worktree.

PR #54 withdraws **all scanner implementation changes**. go_eval.go and go_opaque.go are restored from origin/r3/integration; the added go_admission_effects.go and go_factory_refs.go are removed. No existing trunk test, route, baseline, threshold, dependency or workflow is changed. The earlier tests requiring startup-probe exclusion are retired with the rejected implementation. Their historical failure receipts below remain historical evidence only.

The retained regression tests exercise the actual scanner CLI against Git-backed fixtures. Required cases are conditional mounts, an early different handler, defer/go calls, a swapped pool after a probe, field/global writes, a mutated struct passed onward, method values, aliased/dot imports, nested reassignment/shadowing of mainPool and ctx, and factory calls through function values or package-level initializers. Every case initially requires **TOUCHED**. A real miss on the unchanged scanner will be recorded as an explicit expected-fail and unit FOLLOWUPS entry after GitHub CI confirms it; fixture/setup errors are not expected failures.

The isolated test commit is `0e16475b6fc5ddb05d65db3908f68393e0c43ae8`, file blob `112edffbfa8d9fdd219dfe66252b10e34d34b374`: 27 active subcases in go_factory_trunk_cli_test.go. Each first proves legacy EXACT and MOUNT coverage, then requires candidate TOUCHED for both. Fixtures are parsed in the CI test before scanner assertions. The caller-origin fixtures keep the unsafe caller in the baseline and add only the factory probe in the candidate.

## Provenance

Original branch base: `256e9740ced9c5de05a83f58eb9f4c79a231b7b2`. Tests-only revision also includes trunk `5497e55c3a0ad8ead2a4a8c30c5500c73af0c5c5`, merged as `6556251fac0c8390b9a4c9e4d14ff3ba1b209603`.

Author worktree: /Volumes/data/live_commerce_architecture_v1/.worktrees/ci-drift-factory; branch unit/ci-drift-factory. Author role integration, model gpt-6.1-sol, reasoning ultra (root configuration checked). Allowed paths: scripts/dev/contractdrift/** for this rollback and new tests, output/ci-drift-factory/** for receipts and author record. Scanner source must have no remaining diff from trunk.

| Delegate | Role / model / reasoning | Actual worktree / branch / base / writes |
| --- | --- | --- |
| factory_trunk_regressions | test_worker / gpt-6.1-sol / high | /Volumes/data/live_commerce_architecture_v1/.worktrees/ci-drift-factory-regressions; unit/ci-drift-factory-regressions; base 6556251fac0c8390b9a4c9e4d14ff3ba1b209603; only new go_factory_trunk_cli_test.go |
| Historical drift_factory_boundary | explorer / gpt-6.1-sol / high | Root worktree; original base above; read-only |
| Historical drift_factory_tests | test_worker / gpt-6.1-sol / high | /Volumes/data/live_commerce_architecture_v1/.worktrees/ci-drift-factory-tests; unit/ci-drift-factory-tests; original base then bfcfa5eb; only the two now-retired CLI test files |
| Historical factory_effect_audit | explorer / gpt-6.1-sol / high | Root worktree at 7b652f59 and source follow-ups; read-only |
| Tests-only factory_effect_audit | explorer / gpt-6.1-sol / high | Root worktree at test commit 0e16475b6; read-only static audit; Humaux research 0a185907-1730-4af4-8472-eb4d0f8f695e |
| Historical factory_boundary_impl | integration_worker / gpt-6.1-sol / high | /Volumes/data/live_commerce_architecture_v1/.worktrees/ci-drift-factory-effects; unit/ci-drift-factory-effects; base 7b652f59; only the four now-withdrawn scanner source changes |
| Historical factory_counter_tests | test_worker / gpt-6.1-sol / high | Tests worktree / branch above; base 7b652f59; only now-retired go_factory_effect_cli_test.go |
| Historical factory_round1_review_audit | explorer / gpt-6.1-sol / high | Root worktree at 42f29ae0; read-only |

Delegate model/effort records describe explicit dispatch configurations, not independent provider telemetry. Writers used isolated worktrees. Historical implementation work is withdrawn, not described as accepted.

## Historical CI receipts — not final acceptance

| Revision / run | Observed result and portable receipt |
| --- | --- |
| OA-1 316848601032b434885dd513b11f0a8871ef57e5, [Gates 38475612050 / job 115565420528](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38475612050/job/115565420528) | 71 TOUCHED findings, exit 1, after a probe was added inside the buyer factory. [Receipt](evidence/oa1-plan.log). This is the historical repro motivating the separate preflight. |
| Retired tests-first bfcfa5eb, [Foundation 38477346748 / job 115571192630](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38477346748/job/115571192630) | RED: startup-probe exclusion expectations failed; toy middleware/child-body cases also missed. [Receipt](evidence/red-factory.log). Those exclusion expectations are no longer this PR's contract. |
| Retired frozen-probe tests 7b652f59, [Foundation 38478824210 / job 115576095619](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38478824210/job/115576095619) | RED: the added frozen probe produced errors 3. [Receipt](evidence/red-frozen-oa1.log). |
| Withdrawn scanner at 42f29ae0 | Independent adversarial review by claude-opus-5-5 found P1 false negatives in caller-origin name binding and indirect/initializer call enumeration. Owner rejected further analyzer hardening; restoring trunk removes this implementation from the PR. This is not acceptance of the final tests. |

## Current acceptance

| Check | Result |
| --- | --- |
| Scanner source equals current trunk | E1: effective non-test scanner diff against trunk 5497e55c is empty after integration; all existing trunk tests are unchanged |
| Independent scoped static review | E1: no scoped P0/P1 at test commit 0e16475b6; test file SHA256 d478e83bd4cbc97f92241b7f14dad9e282101f9c11cb85631a53c020941332b9. Runtime and final integrator acceptance remain pending |
| New must-be-TOUCHED regressions on unchanged scanner | NOT_RUN — GitHub CI required |
| Existing complete CI-DRIFT package / Foundation / required Gates | NOT_RUN on final tests-only SHA |
| Confirmed existing scanner false negatives | NOT_RUN — no speculative expected-fail entries |
| Independent final tests-only review | NOT_RUN; integrator reviews after push |

Every local test, build and application run is **NOT_RUN** under the owner's CI-only directive. No local Go, gofmt, Node, pnpm, scripts, Playwright, server or PostgreSQL was executed. Product runtime, provider SANDBOX/LIVE, buyer data and deployment are outside this unit. CI run IDs and exact source bindings will replace pending entries only after actual execution.
