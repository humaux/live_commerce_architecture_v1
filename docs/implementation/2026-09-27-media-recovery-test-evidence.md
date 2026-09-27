# MRR01–04 independent recovery test evidence

Status: **FIRST FOCUSED RUN FAILED / REPAIR PENDING** (2026-09-27). Frozen contract:
`contracts/live-media-recovery-observer-v1.md` at root `598eea4978e8f32f9ee2b998031c2996b762370b`.
Independent test worktree `commerce/media-recovery-tests-20260927` started from that SHA.
The test author commits are `82379c6`, `5166976`, `53a2be0`, `42f95d9`,
`04cb135`. The source author's SQL-only `ffc1f67` was a static planning
snapshot, never an executable acceptance candidate. First fixed source
candidate `d0cb896` was merged without source edits at `44d75be` for one
isolated focused PG18 run. This candidate is not accepted: its SQL has a
`42702` alias ambiguity, and its child-to-parent ACK control pipe conflicts
with the frozen parent-to-child release/EOF contract. The source author is
repairing both in a new fixed SHA.

| Gate | Authored independent checks | Remaining before PASS |
| --- | --- | --- |
| MRR01 | Real local TLS Start accepted before old native worker SIGKILL and lost ACK; supervised parent starts at t0, two child SIGKILL/reaps/restarts retain the same episode/capture/deadline; committed fresh ROOM witness within 90s, original operation/job/max attempts, no duplicate Start or Stop. Separate known-ID QUERY process path. | First run FAIL: all three process tests could not persist an episode due to source `42702`; real 90-second gate NOT_REACHED. Rerun on repaired fixed SHA; verify exact original attempt delta from native scheduling. |
| MRR02 | SQL: known-empty NO_WORK, late unknown-empty overdue, old active lease busy, immutable same-episode replay, prior unfinished fail-whole, capacity+1 fail-whole, all members witnessed yet unknown coverage scope miss, timeout-first sticky, delayed witness attestation, true PG operation-lock interleaving where witness commits before waiting timeout. Process: actual parent-alive 90s provider-fault miss, durable timeout, redacted local alert, no new provider request after deadline. | First run FAIL at source `42702`; actual 90 seconds NOT_REACHED. Add DB/config/readiness/EOF diagnostics and native-child capacity coexistence or cite independent fixed-source tests with actual results. Human alert delivery remains external NOT_RUN. |
| MRR03 | Exact seven signature/result/owner/security-definer/ACL checks; old roles denied entry points; mixed recovery/executor role rejection; wrong generation/token/room; future and expired lease; old public QUERY cleanup guard; observer-only QUERY keeps cleanup_required with zero Stop reservation; stale terminal QUERY cannot override newer active projection; prewire and exhausted native job denied. | First run FAIL at source `42702`, plus one test-only `23514` fixture setup error; all six test-only single-field NULL lease updates were corrected to structurally valid expired leases after the run. Test wrong physical DB, other mixed-role admissions, missing job and INPUT exclusion. |
| MRR04 | Idempotent migration reapply, `media_recovery_ready()`, local process SIGTERM cleanup, bounded lock race, narrow `--live-media-recovery` selector. Core new tests are included in default full foundation selector without a skip marker. | First focused run FAIL. Rerun focused and then unchanged full runner on fixed source; enabled/disabled/internal-child admission and source/provenance review. Original LMR05 35-second gate remains unchanged and separately open. |

The SQL calls with `p_elapsed_ms` values are **counterexamples for the frozen
SQL boundary**. They are not called a virtual-clock proof of 90 seconds. Only
the `RealNinetySecondMiss` and positive supervisor process tests sample elapsed
time from real process launch, and those tests never rewrite `attempted_at`,
the lease or the deadline. SQL-only counterexamples now use a structurally
valid expired old lease; the real-clock process tests never modify it.

Static author checks: `gofmt`, `bash -n scripts/dev/test-local.sh`, and
`git diff --check` exited 0. A compile-only probe on the old base exited 1
because `platform.ValidateMediaRecoveryPool` was absent there; the source
author included it in `d0cb896`. This compile-only result is not an MRR verdict.

First run receipt: `bash scripts/dev/test-local.sh --live-media-recovery` at
integrated HEAD `44d75bef913815e9e2485e342bb1adb8ed6d2eb5` exited **1**.
The package reported **75.835s**, 11 top-level FAIL, 0 PASS, 0 SKIP.
Bounded raw log: `output/mrr-focused-44d75be.log`, SHA-256
`33212559b393a0d33be13486527a4570c10138b1383de12859d7d0c7192e7543`.
Seven SQL gates failed at `live.begin_media_recovery_episode` with `42702`
`x.operation_id` ambiguity; three process gates stopped before episode capture;
the cleanup guard test had a test-only `23514` from clearing only `lease_until`.
The focused runner's cleanup removed its own PG18 fixture. Readback found no
task-owned foundation fixture container; the unrelated protected
`lc-meta-upgrade-9d14f59e966f` remained running. No full runner was invoked.

Pending repaired-source receipt fields: source SHA; exact command and exit;
top-level PASS/FAIL/SKIP; actual 90-second elapsed; process parent/child
exit and reap; cleanup; original LMR05 and full runner separate results.
Existing untracked `output/` contains earlier immutable Studio failure
evidence and is preserved.
