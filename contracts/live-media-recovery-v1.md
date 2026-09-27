# Media restart reconciliation — owner target, policy preflight

Status: **OWNER_TARGET_APPROVED / DESIGN_REVIEW_REQUIRED / IMPLEMENTATION_NOT_RUN**.
Owner selected **90 seconds** on 2026-09-27; Humaux decision
`f0b36444-a4ea-4112-926d-596cfb7fd89f`. This is not deployment approval.

## Approved outcome

After abnormal media-service restart, resume state reconciliation within
90 seconds. This means fresh authoritative state checking, not a guarantee
that an external stream has stopped. If the deadline cannot be met, retain
the original attempt, operation, resource and unresolved responsibility and
raise an actionable alert. An unobserved resource must never become terminal
merely because a timer, process or retry budget ended.

No new Start, replacement operation, third Stop, customer-stream shutdown,
queue duplication or implicit budget reset is authorized by this target.
Existing LME/LMR fencing, exact frozen credentials and terminal evidence remain
mandatory. Website, Meta and platform conversation domains remain untouched.

## Current evidence and scope gap

`NewMediaClient` currently leaves native River rescue timing at its default
one hour; `mediaExecutionWorker.Timeout` is 30 seconds. The existing LMR05
fixture ages its running row by two hours to exercise rescue and Stop-budget
safety. It does **not** establish real wall-clock crash recovery.

The phase diagnostic observed 34.66 seconds after that eligibility shortcut,
with the original 35-second assertion unchanged. The original full regression
failure remains retained. Owner approval of 90 seconds neither clears that
failure nor authorizes changing its deadline as a substitute for a new gate.

## Proposed clock and success contract

The test parent/supervisor captures monotonic t0 immediately before launching
the replacement process. The process also samples monotonic time at its
earliest entry, before configuration and pool validation. Start/preflight,
old-leader expiry/election, native rescue/retry, the provider call and database
commit are included. The `media_worker_ready` log is a phase marker, not t0
and not proof of a leader or successful reconciliation.

Success requires a newly committed, correlated post-restart QUERY/ROOM
observation or coherent terminal proof for the original operation. Old
observations, generic successful GETs, `finish_uncertain`, native state changes
and bare provider acknowledgements do not count. The parent's elapsed time
through authoritative database readback must be <=90 seconds.

The positive local gate uses valid frozen configuration, a writable same-DB
role pair, responsive exact local provider replies, bounded eligible work no
greater than configured concurrency, and no further crash before the first
new observation. It includes real stale leader state and native scan cadence.
Failures, repeated crashes and prior long backoff must be reported as missed
reconciliation targets and take the timeout path, not excluded to report a
false universal success rate. Cloud and overloaded deployment targets require
their own measured capacity gates.

## Proposed native policy and custody

Current executable scope is `PROVIDER_MOCK / media_mock_v1`. Preserve the
existing stricter INPUT-lane guard without activating its future consumer.
Use media-only River `JobTimeout=30s`, matching the worker's existing timeout,
and `RescueStuckJobsAfter=35s`. Do not fork River or change its 30s maintenance
sweep. Keep its default native maximum attempts (25).

For the first three recorded native errors/rescues, cap the native default
retry delay at five seconds; later errors retain native exponential backoff.
The cap does not apply to business Stop budgets, authorize any additional
wire, or replace successful-observation snoozing. Ordinary errors and rescue
share this policy and must both be tested. The positive gate is a measured
target under its stated conditions, not an unproved deterministic upper bound
for arbitrary scheduling, infrastructure or prior error history.

Before enabling that policy, extend the native finalization/deletion guard
to ordinary unresolved MEDIA resource responsibility. Exact original identity
must remain present and nonfinalized until coherent terminal closure or the
existing durable 4096-generation/24h escalation. Native attempt exhaustion
alone cannot remove it or manufacture that escalation. Preserve all existing
INPUT custody restrictions, which may outlive MEDIA escalation. Do not reset
attempt counts, create a replacement job or retry provider side effects from
an exhaustion handler. A exhausted native job may require operator review;
the timeout monitor must surface that stall, not quietly claim recovery.

## Proposed timeout record and observable signal

Reuse the existing execution projection, original operation and operation
events, not a second operation/queue. A bounded restart-observer owned by the
media process records a recovery episode for eligible, unresolved original
operations. On first DB availability, derive the DB-clock deadline from the
already elapsed monotonic startup time; do not restart the 90s clock at commit.
An already persisted unfinished deadline survives process restart unchanged.
The process must not silently make a cross-crash persistence claim for a
crash before the first marker commits; the supervisor's deadline observation
and a dedicated negative test cover that boundary.

Under the existing operation lock, a new correlated observation can resolve
the active episode; racing timeout handling rechecks that fact. If overdue,
append one deduplicated `media_recovery_timeout` operation event and retain
UNKNOWN/resource responsibility. Keep that event after later recovery. The
monitor cannot issue Start/Stop, clear liability, force native scheduling or
reset attempts. Its scanning/batch/rate bounds and private SQL signatures must
be reviewed before implementation, with exact role and readiness admission.

Local alert acceptance is a durable event plus an externally observable,
redacted structured diagnostic from the real process. This is **not** human
notification delivery. The repository has no verified pager/recipient route;
that remains a deployment gate. While DB is unavailable, persistence cannot
be promised: emit a health-failure diagnostic and let the service supervisor
retain the timeout; persist overdue evidence when DB returns. No recipient,
provider credential or external notification is configured by this unit.

Independent preflight reference: `0f4cdbf7-07d5-4001-8e34-f190d8c4b021`.
These parameters and seams still require a separate design/security verdict
and exact SQL/Go interface freeze; they are not implemented or tested.

## Required acceptance boundaries

| Gate | Required evidence before acceptance |
| --- | --- |
| LRC01 | Actual process SIGKILL and restart with original unaged job/lease timestamps; monotonic elapsed time to fresh, persisted reconciliation at most 90s; original identifiers and Stop budget retained |
| LRC02 | Controlled unavailable/slow dependency prevents timely reconciliation; original UNKNOWN liability and one durable alert remain; no false terminal state or replacement side effect |
| LRC03 | Native retry/attempt exhaustion and repeated failures retain or explicitly escalate original responsibility; no silent native-job disappearance or unlimited wire retry |
| LRC04 | Readiness/roles/old-runtime upgrade safety and independent focused plus fixed-tree full regression; original failure evidence remains available |

All four gates are **NOT_RUN**. Exact SQL/Go seams and independent review are
pending; this draft is not a frozen implementation
interface. Local MOCK acceptance cannot establish Cloud, customer hardware,
production monitoring or public-platform live-stream acceptance.
