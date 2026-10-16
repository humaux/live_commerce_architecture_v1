<!-- Purpose: SHA-bound evidence for CI efficiency optimization only. -->
<!-- Depends on: current shard planner, original GitHub run/job receipts and owner CI-only authorization. -->
<!-- Used by: integrator and independent reviewer; not a second requirements or gate catalogue. -->
# CI efficiency: measured shard refresh (final matrix pending)

- Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/release-gate-ci`
- Branch: `unit/release-gate-ci`; base: `c077226e7a6c1511069cac0c8df57212e0c702d8`.
- Role: primary CI implementer. Runtime model/effort identifiers are not exposed (UNKNOWN, not inferred). Two read-only audits used explicitly selected `gpt-6.1-sol` / `medium`.
- Requirement authority: latest owner messages, “不要开发整个项目，对整个项目跑ci的效率和速度进行优化。” and “遵从唯一权威性原则”. Prior RELEASE-GATE-CI permission covers this unit branch/PR/CI dispatch only, not product work, trunk merges or deployment.
- Development authority: `scripts/dev/shard-plan.mjs`, generated `shard-plan.json`, `shard-timing.json` and existing gate coverage checks at the base SHA. Agent reports are leads only; the original receipts below were read directly.
- Local execution: **NOT_RUN by owner rule**. No local tests, linters, formatters, servers or containers. Git and GitHub receipt operations only.

## Baseline (not an optimized result)

| Original run | Source / scope | Wall time | Cumulative job minutes |
| --- | --- | ---: | ---: |
| [38825835230](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38825835230) | c077226e, full scheduled universe, 80 jobs | 42m38s | 602.72 |
| [38808160050](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38808160050) | same SHA, foundation-only push, 15 jobs | 42m32s | 171.82 |

Different mode sets are **not** a cost before/after comparison. Job minutes are derived from original per-job start/end timestamps, not billed-minute claims. Queue time and execution time must be compared separately.

Direct raw log observations from run 38808160050:

| Test | Job | Observed seconds | Current scheduler weight |
| --- | --- | ---: | ---: |
| TestOrderAttemptLocalKeyMissingExpiresAtRealDeadline | 116675000695 | 904.35 | missing → default 5 |
| TestOrderAttemptDeadlineRealClaimPeriodicLiveness | 116675000787 | 904.57 | missing → default 5 |
| TestOrderAttemptDeadlineExecutionCrossesRealClock | 116675000850 | 904.78 | missing → default 5 |

These land in g01/g02/g06. Their Gate steps take ~24–27 minutes while other foundation groups take ~9–11 minutes. A real ~905-second test is an irreducible floor here; its wait and assertions are not changed.

## Smallest implementation / causal CI plan

1. Add a regression requiring those existing 15-minute clock tests to have measured weights and non-stale generated estimates. Old timing data must produce exactly that one failed Node test.
2. A **temporary**, explicit-dispatch-only calibration in existing `gates.yml` reads complete successful run 38808160050 artifacts. It validates the exact source SHA and unchanged product/test source, runs existing `--ingest`, `--write`, `--check` on GitHub, then reruns the regression and uploads the generated patch plus red/green logs.
3. Download/apply that CI-produced patch, remove the temporary workflow delta, and commit the normal timing/plan refresh. Keep ten groups, all test names, the catch-all, exact-once coverage and all original thresholds/timeouts.
4. Final GitHub run: full foundation matrix plus normal PR checks. Compare the same foundation mode set against the baseline, reporting longest execution and queue time separately. No predicted speed-up is a PASS claim.

## Calibration receipts (E3 for data generation, not final speed acceptance)

- [Run 38932747569](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38932747569), exact source `e7afff8e4dff563ba5c2c0c2d2674a684000a287`, completed **success** at 2026-10-16T16:42:07Z. `modes=["shard:unit"]`, `extra_env=LC_SHARD_TIMING_RUN=38808160050`; gate job [117080677379](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38932747569/job/117080677379). No duplicate dispatch.
- Original artifact: `gate-shard_unit`, id `12060197948`, Actions digest `sha256:37bd24b2ee1cdc75c9698d9945e545a153516c49605099b4e8057dd4be7064ed` (not expired when read).
- `node --test --test-reporter=tap tests/ci/shard-plan.test.mjs`: before refresh **exit 1**, 7 PASS / 1 FAIL / 0 skipped. The sole failure is the new real-clock regression, caused by an undefined measured weight, not a setup error. After refresh **exit 0**, 8 PASS / 0 FAIL / 0 skipped. Original downloaded logs: `calibration-38932747569/ci-gates/shard-timing-{red,green}.log`.
- The existing generator ran on GitHub: `--ingest` of all ten original successful shard logs; `--write`; `--check` **exit 0**, **1521 top-level tests, exactly once in ten groups**. Before ingest, the calibration required successful package/test terminators, no duplicate names within/across logs, and equality with the current compiled foundation inventory. Source run is successful attempt 1 at `c077226e7a6c1511069cac0c8df57212e0c702d8`, and product/foundation source equality was checked by CI.
- CI-produced `calibration-38932747569/ci-gates/shard-timing-refresh.patch` was applied mechanically, not regenerated locally. All ten original logs are retained in `source-receipts/full/gate-shard_gNN/ci-gates/shard_gNN.log`; provenance is `calibration-38932747569/ci-gates/timing-source.json` and the original Actions artifacts. These downloads are retained evidence, not new source modules.
- Generated estimates are **905 / 905 / 904 / 794 / 794 / 794 / 794 / 794 / 794 / 794 seconds**, with the three real-clock tests in singleton groups and one catch-all. Estimates are scheduling inputs, **not measured optimized runtimes**. No original test, assertion, timeout or drift limit changed.
- Temporary calibration code and its `actions:read` permission have been removed: the final workflow is identical to the base. Read-only independent static review found no P0/P1/P2 in the generated patch, coverage, or regression; final runtime acceptance remains pending.
- Earlier static review corrected the missing full-inventory validation and Node 24 default reporter mismatch before dispatch. TAP is explicit ([Node 24 official reporter documentation](https://nodejs.org/download/release/v24.15.0/docs/api/test.html#test-reporters)).

## Final acceptance / NOT_RUN

- Final source SHA / PR / matrix run: **pending next source commit**. Normal PR checks will supply the ten foundation groups and unit shard together with the fixed required checks; do not duplicate that same matrix in an extra dispatch.
- Optimized execution, queue time and cumulative job-minute comparison: **NOT_RUN** until the final source matrix terminates.
- Independent final-SHA review: **NOT_RUN** (the applied source diff has received a read-only static review, not final runtime acceptance).
- Product changes, new release framework, SANDBOX/LIVE provider calls, production and deployment: **not in scope**.
- No product Go/SQL, contracts, provider credentials, locks or dependency manifests changed.
