# MRR01–04 independent recovery test evidence

Status: **TEST_AUTHORED / RUNTIME_NOT_RUN** (2026-09-27). Frozen contract:
`contracts/live-media-recovery-observer-v1.md` at root `598eea4978e8f32f9ee2b998031c2996b762370b`.
Independent test worktree `commerce/media-recovery-tests-20260927` started from that SHA.
The author commits are `82379c6`, `5166976`, `53a2be0`, `42f95d9`. The source
author's SQL-only `ffc1f67` is a static planning snapshot with review findings;
it has not been used as an executable acceptance candidate. Fixed integrated
source SHA: **PENDING**. No PG18 fixture or provider was run by this test worker.

| Gate | Authored independent checks | Remaining before PASS |
| --- | --- | --- |
| MRR01 | Real local TLS Start accepted before old native worker SIGKILL and lost ACK; supervised parent starts at t0, two child SIGKILL/reaps/restarts retain the same episode/capture/deadline; committed fresh ROOM witness within 90s, original operation/job/max attempts, no duplicate Start or Stop. Separate known-ID QUERY process path. | Execute on fixed source; verify exact original attempt delta from native scheduling. |
| MRR02 | SQL: known-empty NO_WORK, late unknown-empty overdue, old active lease busy, immutable same-episode replay, prior unfinished fail-whole, capacity+1 fail-whole, all members witnessed yet unknown coverage scope miss, timeout-first sticky, delayed witness attestation, true PG operation-lock interleaving where witness commits before waiting timeout. Process: actual parent-alive 90s provider-fault miss, durable timeout, redacted local alert, no new provider request after deadline. | Execute; add DB/config/readiness/EOF diagnostics and native-child capacity coexistence or cite independent fixed-source tests with actual results. Human alert delivery remains external NOT_RUN. |
| MRR03 | Exact seven signature/result/owner/security-definer/ACL checks; old roles denied entry points; mixed recovery/executor role rejection; wrong generation/token/room; future and NULL lease; old public QUERY cleanup guard; observer-only QUERY keeps cleanup_required with zero Stop reservation; stale terminal QUERY cannot override newer active projection; prewire and exhausted native job denied. | Execute; test wrong physical DB, other mixed-role admissions, missing job and INPUT exclusion. |
| MRR04 | Idempotent migration reapply, `media_recovery_ready()`, local process SIGTERM cleanup, bounded lock race, narrow `--live-media-recovery` selector. Core new tests are included in default full foundation selector without a skip marker. | Execute focused and unchanged full runner on fixed source; enabled/disabled/internal-child admission and source/provenance review. Original LMR05 35-second gate remains unchanged and separately open. |

The SQL calls with `p_elapsed_ms` values are **counterexamples for the frozen
SQL boundary**. They are not called a virtual-clock proof of 90 seconds. Only
the `RealNinetySecondMiss` and positive supervisor process tests sample elapsed
time from real process launch, and those tests never rewrite `attempted_at`,
the lease or the deadline.

Static author checks: `gofmt`, `bash -n scripts/dev/test-local.sh`, and
`git diff --check` exited 0. A compile-only probe on the old base exited 1
because `platform.ValidateMediaRecoveryPool` is absent there; the source author
confirmed the exported method is part of the pending Go implementation. This
is **NOT_RUN** for MRR acceptance, not a product failure verdict.

Pending receipt fields: integrated source SHA; exact command and exit code;
isolated PG18 container label; log path and SHA-256; top-level PASS/FAIL/SKIP;
90-second measured elapsed; process parent/child exit and reap; fixture cleanup;
original LMR05 and full runner separate results. Existing untracked `output/`
contains earlier immutable Studio failure evidence and is preserved.
