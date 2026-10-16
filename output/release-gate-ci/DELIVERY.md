<!-- Purpose: SHA-bound evidence for CI efficiency optimization only. -->
<!-- Depends on: current shard planner, original GitHub run/job receipts and owner CI-only authorization. -->
<!-- Used by: integrator and independent reviewer; not a second requirements or gate catalogue. -->
# CI efficiency: measured shard refresh (in progress)

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

## Current receipts / NOT_RUN

- Calibration dispatch: [38932747569](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38932747569), source `e7afff8e4dff563ba5c2c0c2d2674a684000a287`, `modes=["shard:unit"]`, `extra_env=LC_SHARD_TIMING_RUN=38808160050`, started 2026-10-16T16:32:25Z. **IN_PROGRESS**, not a PASS; job timeout 150 minutes. Read-only static review found and corrected the missing full-inventory validation and Node 24 default reporter mismatch before dispatch. TAP is explicit ([Node 24 official reporter documentation](https://nodejs.org/download/release/v24.15.0/docs/api/test.html#test-reporters)).
- Optimized matrix: **NOT_RUN**.
- Independent final review: **NOT_RUN**.
- Product changes, new release framework, SANDBOX/LIVE provider calls, production and deployment: **not in scope**.
- No product Go/SQL, contracts, provider credentials, locks or dependency manifests changed.
