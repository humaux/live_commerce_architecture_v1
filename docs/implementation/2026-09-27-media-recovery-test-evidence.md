# MRR01–04 independent recovery test evidence

Status: **FOUR FOCUSED RUNS; FOURTH EARLY-STOPPED ON TEST ASSERTION** (2026-09-27). Frozen contract:
`contracts/live-media-recovery-observer-v1.md` at root `598eea4978e8f32f9ee2b998031c2996b762370b`.
Independent test worktree `commerce/media-recovery-tests-20260927` started from that SHA.
The test author commits are `82379c6`, `5166976`, `53a2be0`, `42f95d9`,
`04cb135`. The source author's SQL-only `ffc1f67` was a static planning
snapshot, never an executable acceptance candidate. First fixed source
candidate `d0cb896` was merged without source edits at `44d75be` for one
isolated focused PG18 run. This candidate is not accepted: its SQL has a
`42702` alias ambiguity, and its child-to-parent ACK control pipe conflicts
with the frozen parent-to-child release/EOF contract. The source author is
repairing both in a new fixed SHA. That repair `b90d191` was merged at
`804065e` for the second run; it removed `42702` but exposed admitted-event
visibility failures under forced RLS.

| Gate | Authored independent checks | Remaining before PASS |
| --- | --- | --- |
| MRR01 | Real local TLS Start accepted before old native worker SIGKILL and lost ACK; supervised parent starts at t0, two child SIGKILL/reaps/restarts retain the same episode/capture/deadline; committed fresh ROOM witness within 90s, original operation/job/max attempts, no duplicate Start or Stop. Separate known-ID QUERY process path. | Second run FAIL: positive episode persisted but ROOM request absent at 43.41s; known-ID QUERY readback absent at 92.61s. Source admitted-event visibility repair pending. Original attempt delta still NOT_RUN. |
| MRR02 | SQL: known-empty NO_WORK, late unknown-empty overdue, old active lease busy, immutable same-episode replay, prior unfinished fail-whole, capacity+1 fail-whole, all members witnessed yet unknown coverage scope miss, timeout-first sticky, delayed witness attestation, true PG operation-lock interleaving where witness commits before waiting timeout. Added two-member owner/media-writer event visibility and DROP/partial-policy failclosed replay/read/timeout, prior-unfinished non-overwrite, malformed DSN/wrong-role pool. Process: actual parent-alive 90s provider-fault miss, durable timeout, redacted local alert, no new provider request after deadline. | Second run SQL member gates FAIL with source `ME409`; real negative ran 1m35s but outer Go harness hit 240s across the suite, so actual 90-second verdict NOT_COMPLETED. Focused harness 360s authorized; frozen 90s unchanged. New short cases NOT_RUN. Native capacity coexistence, DB startup delay and late real readback remain holes. Human alert delivery external NOT_RUN. |
| MRR03 | Exact seven signature/result/owner/security-definer/ACL checks; old roles denied entry points; mixed recovery/executor role rejection; direct observer EXECUTE grants make old runtime/buyer/meta worker pools inadmissible; same recovery login connected to the task-owned PG18 container's distinct `postgres` database is refused; wrong generation/token/room; future, legally cleared NULL, and structurally intact expired lease; old public QUERY cleanup guard; observer-only QUERY keeps cleanup_required with zero Stop reservation; stale terminal QUERY cannot override newer active projection; prewire and exhausted native job denied; escalated nonterminal original with still-eligible River job remains captured as ceiling and times out without observer target/provider call. Added synthetic owner-only INPUT wire-flag exclusion and wrong original job ID denial. | Second run wrong physical DB PASS and prewire subtest PASS; runtime direct-grant subtest PASS. Buyer fixture had `42501` text-signature lookup without schema USAGE; meta fixture's clean pool was rejected due SET-capable membership. Owner-resolved OID and meta SET-false fixture repairs authored but NOT_RUN. INPUT/wrong-job NOT_RUN. Remaining member gates failed `ME409`. |
| MRR04 | Idempotent migration reapply, exact readiness including extra PUBLIC/direct-writer SELECT policy and BYPASSRLS definer rejection, local process SIGTERM cleanup, internal-child release then EOF/reap, bounded lock race, narrow `--live-media-recovery` selector. Core new tests are included in default full foundation selector without a skip marker. | Second focused run FAIL; new policy/EOF cases authored but NOT_RUN. Enabled/disabled modes and full regression still NOT_RUN. Original LMR05 35-second gate remains unchanged and separately open. |

The SQL calls with `p_elapsed_ms` values are **counterexamples for the frozen
SQL boundary**. They are not called a virtual-clock proof of 90 seconds. Only
the `RealNinetySecondMiss` and positive supervisor process tests sample elapsed
time from real process launch, and those tests never rewrite `attempted_at`,
the lease or the deadline. SQL-only counterexamples now use a structurally
valid expired old lease; the real-clock process tests never modify it.
One SQL gate uses the existing native `finish_media_uncertain` function to
clear the complete lease tuple to NULL; a future lease remains busy. The
escalated ceiling gate is another SQL-only fixture and leaves the original
River attempt below its max; neither alters the real process clock.

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

Second run receipt: fixed source `b90d1914602d09e5dbca21ec6b2082e9ce784dd5`
merged with test SHA `2754509` at `804065e302a1f53de08920d7c91a27d8b5e3a5bb`.
The same focused command exited **1**. The package reported **241.417s**;
11 top-level tests failed, one passed, and the final real-90 test was aborted
by Go harness `-timeout=240s` after 1m35s. Raw log
`output/mrr-focused-804065e.log`, SHA-256
`2e32c15b280371987388ee22252774aa345288f646019eed76e2137348c530d7`
(213 lines). SQL `42702` disappeared, but repeated `ME409` membership failure
blocked member gates. The media writer could insert admitted events but could
not SELECT them under forced RLS; source repair is pending. A test-only
direct-grant check looked up a function by text signature from the buyer
role without `live` schema USAGE and got `42501`; the meta role's clean pool
was rejected before its grant because the synthetic login had SET-capable
membership. The fixture now resolves the function OID with the owner first
and configures meta membership SET-false; both repairs remain NOT_RUN. The
240s was an outer suite
budget, not the 90-second supervisor deadline; only this focused selector was
authorized to increase to 360s. Runner cleanup removed the task-owned PG18
fixture; only the unrelated protected container remained. No full run.

After the second run, the focused selector's outer Go budget changed from
240s to 360s and gained `-failfast` so a new SQL/readiness failure does not
waste the remaining real-clock process cases. This changes neither the frozen
90-second deadline nor the default full-suite budget. A fixed test batch adds
owner-resolved function-OID grants, first-test positive readiness, policy
DROP/partial/extra permissive RLS and BYPASSRLS counterexamples, INPUT-profile
and wrong-job denial, malformed DSN/wrong-role pool admission, and post-release
internal-child EOF/reap. `gofmt`, `bash -n`, `git diff --check`, and
`go test -c` exited 0 after the final BYPASSRLS addition. All new runtime
assertions are **NOT_RUN** until the next
fixed source snapshot and isolated PG18 window.

Third run receipt: fixed source `86b641a909277223fa630022cd4d0df6f7d22ad8`
merged with test `596ab4f` at `952e57a02ac2800e06ace3771bd23bde4c63dfb4`.
The same focused command with `-failfast -timeout=360s` exited **1** after
**3.857s** package time. Two top-level tests passed: SQL scope/clock/witness
with positive readiness, and authority/fence checks. One failed:
`OldPoolsRejectDirectObserverGrant/meta` rejected its *clean* meta worker pool
before the direct grant was installed. The synthetic login inherited a
SET-capable membership from `lmaLogin`; meta-worker admission requires an
INHERIT-only, SET-false membership. Runtime and buyer subcases passed. This
is a test fixture baseline error, not evidence of an observer-role product
regression. All later tests, including the real 90-second process gate, are
**NOT_RUN** due to failfast. Raw log `output/mrr-focused-952e57a.log`, 16
lines, SHA-256 `37364c9028898ddd1b921179e6134ec30648f1b5f0e721d124a89bcf365cc94d`.
The runner cleaned its own PG18 fixture; only unrelated protected
`lc-meta-upgrade-9d14f59e966f` remained. No source change followed this
result; the meta synthetic-login membership repair was authorized separately.

Fourth run receipt: source remained `86b641a`, with the meta fixture repair
at test HEAD `419feb817f6082114dae23bc56fba06ae1034564`. Focused command
exited **1**; package **18.563s**. Thirteen top-level tests passed, including
SQL scope/readiness, exact ABI/ACL and old-role grants, wrong physical DB,
config rejection, two-member RLS DROP/partial/overgrant/BYPASSRLS, timeout
ordering/capacity/concurrency, QUERY cleanup/Stop budget, escalated ceiling,
INPUT exclusion, and wrong original job. The next test,
`MRR04InternalChildEOF`, received a release byte, reached native readiness,
then exited and was reaped after post-release EOF, but its final assertion
incorrectly required zero requests to the configured provider TLS endpoint.
One request came from the original READY job run by the **native child**;
that is not an observer side effect. Later three process tests, including
real 90 seconds, remain **NOT_RUN** under failfast. Raw log
`output/mrr-focused-419feb8.log`, 48 lines, SHA-256
`824cd185bd75f1ff88a155a0e76bbbdcd1e1ba3d9a638421eea8150e94d893a5`.
Task-owned PG18 fixture cleaned; unrelated protected container retained.
No source change followed this result. The root-approved test-only correction
retains the configured TLS request count as a diagnostic but removes the
invalid zero-I/O assertion from this **native-child** EOF gate. Observer-only
zero Start/Stop remains asserted by the MRR01 supervisor process gates.

Pending repaired-source receipt fields: source SHA; exact command and exit;
top-level PASS/FAIL/SKIP; actual 90-second elapsed; process parent/child
exit and reap; cleanup; original LMR05 and full runner separate results.
Existing untracked `output/` contains earlier immutable Studio failure
evidence and is preserved.
