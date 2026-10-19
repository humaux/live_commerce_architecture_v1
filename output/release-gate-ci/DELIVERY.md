<!-- Purpose: SHA-bound evidence for CI efficiency optimization only. -->
<!-- Depends on: current shard planner, original GitHub run/job receipts and owner CI-only authorization. -->
<!-- Used by: integrator and independent reviewer; not a second requirements or gate catalogue. -->
# CI efficiency: measured shard refresh — READY

- Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/release-gate-ci`
- Branch: `unit/release-gate-ci`; base: `c077226e7a6c1511069cac0c8df57212e0c702d8`.
- Role: primary CI implementer. Actual runtime model/effort identifiers are not exposed (UNKNOWN, not inferred). Two independent read-only roles reviewed source/coverage and original CI timings; no guessed runtime label is used as acceptance evidence.
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

| Test | Job | Observed seconds | Baseline scheduler weight |
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
- Temporary calibration code and its `actions:read` permission have been removed: the final workflow is identical to the base. Read-only independent static review found no P0/P1/P2 in the generated patch, coverage, or regression; terminal runtime acceptance is recorded below.
- Earlier static review corrected the missing full-inventory validation and Node 24 default reporter mismatch before dispatch. TAP is explicit ([Node 24 official reporter documentation](https://nodejs.org/download/release/v24.15.0/docs/api/test.html#test-reporters)).

## Final acceptance (E3)

- Frozen executable source: `a8814b1f751ae468f4e02e2760e81c16977a946a`; [PR #78](https://github.com/humaux/live_commerce_architecture_v1/pull/78), base `c077226e7a6c1511069cac0c8df57212e0c702d8`. Actual PR execution tree: **`26b25e2bd870362fc0747470423ed82105f54985`**, not the source head. Raw checkout log `final-38935079717/checkout-job-117088737196.log:462–465` and `merge-commit.json` bind its two parents to that exact base/head.
- [Gates 38935079717](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38935079717): **SUCCESS**, 2026-10-16T16:50:35–17:29:00Z, attempt 1; **79 SUCCESS / 0 FAILURE / 1 SKIPPED (deploy)**. All unit + ten foundation groups and all selected browser modes succeeded; `required` and `aggregate` succeeded. Every Gate step has exit 0. No final manual dispatch or duplicate run; the sole calibration dispatch is above. The 19:20:35Z deadline was not reached.
- [Foundation 38935079444](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38935079444): **SUCCESS**, job `117088425253`, 16:50:38–16:56:22Z. Secret scan, depmap, package/gate docs, Node suites and reachable-vulnerability scan succeeded (exit 0). Its PG/race/vet and pinned-image steps are **SKIPPED** by the existing PR workflow; PG acceptance comes from the separate Gates shards, not from this job. Raw receipt/log: `final-38935079717/foundation-{run,jobs}.json`, `foundation-workflow.log`.
- Ten PG shard logs contain **1512 top-level PASS / 0 FAIL / 10 SKIP**, including one passing `FuzzOrderAttemptModel` seed-corpus entry. Thus 1522 observed results = **1521 planned `Test*` names + one fuzz entry**, with identical before/after name multisets. `shard-plan.mjs` intentionally enumerates and ingests `Test*`; its unchanged catch-all also runs the fuzz seeds exactly once. The ten existing skips are recorded below, not counted as tested provider success.
- Click aggregate: **150/150 page units**, 1164 control clicks, 18 journey steps, **0 failing rows** (`aggregate/click.md`). Visual aggregate: **348/348 shots**, NOT_RUN 0, **0 blocking violations** (`aggregate/visual.md`). These are unchanged gate verdicts, not an expansion of scope.
- Raw run/job/artifact receipts: `final-38935079717/{run,jobs,artifacts}.json`; the full 77-artifact inventory includes unexpired IDs/digests. The eleven foundation artifacts and aggregate were downloaded once to this directory. Original baseline run/jobs are `source-receipts/{run,jobs}-38808160050.json`; complete ten source logs and calibration red/green/patch remain retained.

## Measured before/after: same eleven foundation modes

Authority: original GitHub jobs/steps for **38808160050@c077 → 38935079717@a881 (synthetic merge above)**. All values below are seconds. `Gate` is the step named `Gate`; `job` is job start→end; `Q` is run creation→job start, **including plan/dependency wait**, not a pure runner-queue metric. Membership moved between groups; comparisons use their entire unchanged union, not an assertion that each group kept its members.

| Mode | Baseline job ID | Gate / job / Q | Final job ID | Gate / job / Q |
| --- | ---: | ---: | ---: | ---: |
| g01 | 116675000695 | 1447 / 1493 / 232 | 117088737681 | 1030 / 1070 / 252 |
| g02 | 116675000787 | 1508 / 1557 / 288 | 117088737133 | 990 / 1033 / 46 |
| g03 | 116675000905 | 792 / 834 / 1149 | 117088738373 | 1033 / 1072 / 1112 |
| g04 | 116675001002 | 654 / 700 / 1139 | 117088737228 | 975 / 1018 / 233 |
| g05 | 116675001004 | 580 / 628 / 1142 | 117088737512 | 1024 / 1064 / 269 |
| g06 | 116675000850 | 1600 / 1654 / 760 | 117088737002 | 753 / 796 / 46 |
| g07 | 116675001047 | 903 / 950 / 1585 | 117088738418 | 967 / 1010 / 1236 |
| g08 | 116675000928 | 687 / 737 / 1290 | 117088736764 | 975 / 1012 / 46 |
| g09 | 116675001003 | 631 / 681 / 1020 | 117088737465 | 977 / 1014 / 1253 |
| g10 | 116675001334 | 512 / 554 / 1447 | 117088737196 | 1188 / 1233 / 460 |
| unit | 116675000965 | 455 / 503 / 1273 | 117088737300 | 682 / 729 / 265 |
| Sum | | **9769 / 10291 / 11325** | | **10594 / 11051 / 5218** |

- **Longest Gate execution: 1600s → 1188s (26m40s → 19m48s), −25.75%.** Longest job: 1654s → 1233s, −25.45%. This is the observed long-tail improvement, not the generated 905-second estimate.
- **Cumulative job minutes: 171.52 → 184.18, +7.39%.** Gate minutes: 162.82 → 176.57, +8.45%. This run does **not** demonstrate cost/total-compute savings. Timestamps are not GitHub billed-minute rounding.
- Q sum 11325s → 5218s is recorded separately; different runner scheduling and the final run's parallel browser load prevent attributing it to this patch. Final whole-run wall 38m25s vs scheduled baseline 42m38s is context only, **not causal speed evidence** (different triggers/scheduling).
- Single before/after sample: no repeated-run confidence interval or guaranteed future percentage. Existing real 900-second clocks and assertions remain intact. No additional optimization is claimed.

## Independent review / delivery boundary

- Final-source static review: **E1, no P0/P1/P2**, exact `c077226e..a8814b1f751ae468f4e02e2760e81c16977a946a`. Independent timing auditor separately fetched original GitHub metadata and checked the eleven-row arithmetic and synthetic merge provenance; no implementation-author self-report is used as the receipt.
- Final independent receipt/document review: **PASS, no P0/P1/P2** (read-only source/coverage reviewer, 2026-10-16). The reviewer checked original downloaded receipts, unchanged 1522-result multiset, skip catalogue, synthetic merge, aggregates and timing arithmetic; a second independent timing auditor independently fetched job/step metadata. This is **E4 evidence review**, not production verification. The sole wording cleanup was to replace a stale calibration-stage “pending” sentence with the terminal reference. No tests or source changes were needed after the frozen green SHA.
- The final evidence-only commit is intentionally kept **local** after the already-green source head. Its contents are raw downloads and this document, not executable changes. It is **not** a new tested source SHA. PR #78 stays at a881; the final PR body carries the receipt so documentation does not trigger another 80-job matrix solely to restate a completed run. No merge/deployment is authorized or performed.

## NOT_RUN / limitations

- All local tests/validation/generation/servers/containers: **NOT_RUN (owner CI-only rule)**.
- Existing top-level skips, unchanged before/after: `TestCustomersBillingCB10Sandbox`, `TestMailPA12SMTPProbe`, `TestMetaAdsSandboxS1/S2/S3/S4`, `TestMetaClaimsMCI11LiveReadOnlyProbes`, `TestStripeRF10Sandbox`, `TestStripeSL08RestrictedKey`, `TestProductEditorMatrixLedgerEvidence`. Provider SANDBOX/LIVE and SMTP secrets were not supplied. The matrix-ledger evidence check is a conditional existing skip, not an external-provider PASS.
- Deploy job: **SKIPPED**; smoke/full-secret release acceptance, SANDBOX/LIVE calls, production and deployment: **NOT_RUN / out of scope**. This timing refresh does not claim a new R1 release workflow or complete deployment readiness.
- No product Go/SQL, contracts, provider credentials, locks, dependency manifests, workflow implementation, original test assertion, timeout or drift threshold changed. Measured timing inputs, generated grouping and the new missing-weight regression are the only executable diff.
