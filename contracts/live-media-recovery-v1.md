# Media restart reconciliation — owner target, policy preflight

Status: **OWNER_TARGET_APPROVED / POLICY_DRAFT / IMPLEMENTATION_NOT_RUN**.
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

## Policy decisions still required before freeze

Independent preflight must specify the precise restart clock origin and
observation endpoint; healthy-dependency/load assumptions; native rescue and
retry bounds; ordinary-error and repeated-crash behavior; original-job attempt
exhaustion; durable alert deduplication and recovery; and upgrade/readiness
fences. Startup/preflight latency must not be hidden by starting a clock late.

Reuse the existing original native job and operation/event records where
possible. Do not choose unconditional five-second retry before proving that
native attempt exhaustion cannot abandon unresolved responsibility. Alert
delivery channels and credentials must not be invented or contacted.

## Required acceptance boundaries

| Gate | Required evidence before acceptance |
| --- | --- |
| LRC01 | Actual process SIGKILL and restart with original unaged job/lease timestamps; monotonic elapsed time to fresh, persisted reconciliation at most 90s; original identifiers and Stop budget retained |
| LRC02 | Controlled unavailable/slow dependency prevents timely reconciliation; original UNKNOWN liability and one durable alert remain; no false terminal state or replacement side effect |
| LRC03 | Native retry/attempt exhaustion and repeated failures retain or explicitly escalate original responsibility; no silent native-job disappearance or unlimited wire retry |
| LRC04 | Readiness/roles/old-runtime upgrade safety and independent focused plus fixed-tree full regression; original failure evidence remains available |

All four gates are **NOT_RUN**. Exact scenarios and policy parameters are
pending independent preflight; this draft is not a frozen implementation
interface. Local MOCK acceptance cannot establish Cloud, customer hardware,
production monitoring or public-platform live-stream acceptance.
