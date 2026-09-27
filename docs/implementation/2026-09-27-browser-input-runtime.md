# Browser input custody implementation record — 2026-09-27

Status: **DESIGN_REVIEWED_BIC_FROZEN; implementation and acceptance NOT_RUN**.
Source base `2b52e7d`; frozen contract:
[BIC/BRW](../../contracts/live-browser-input-runtime-v1.md).

## Root cause and boundary

The current original media job completes when Egress is terminal, before it can
retain browser publisher cleanup. The same loss exists in Stop-before-wire,
policy denial, uncertain finish and age/generation exhaustion. Extending only
the happy-path worker would leave those sibling paths unsafe.

Reuse the original operation/job and private MLC custody. Add one input child
and a fresh local-only profile/queue that an old worker cannot consume. Do not
relax historical MOCK profile or reinterpret R04 probe success as product/Cloud
acceptance. The first unit persists/fences liability and jointly controls
completion; token delivery and new-queue worker remain disabled until the next
unit's independent acceptance.

The private profile is deliberately limited to LOCAL_SFU_MOCK_EGRESS. This is
not a generic provider/profile framework. Local reserved input cannot be marked
strict CLOSED because cached/refreshed token revocation is not qualified here.
Held UNKNOWN is visible unresolved work, not success or automatic retry forever.

## Ownership and planned evidence

Read-only source preflight: `/root/media_runtime_source`, main `2b52e7d`.
Read-only browser/test preflight: `/root/media_runtime_tests`, same base.
Independent reviewer: `/root/livekit_protocol_impl`, no writes; final bounded
review found no remaining confirmed P0/P1. Corrections: legacy planner refuses
profile-attached authority, exact native queue guards, full current-admission
check after cached command replay, and finite attempt-anchored input lifetime.
Runtime model/effort of these reused agents is not exposed; no override claimed.
No recursive delegation. Source/test writing begins only after interface freeze,
in the existing separate worktrees; at most two concurrent writers.

Root owns final migration merge (`0040` and post-River `0009`), contract and
task metadata. Independent tests use existing actual PG18/race runner and
`TestLiveMediaExecutionBIC` prefix, not another harness. Acceptance covers BIC01–05
and preserves old assertions. Root reruns accepted source independently.

Preflight receipts: `1288c97f-bfcd-496d-8f06-c6c6f3548bb4`,
`44467d60-d774-4a8f-bb67-cebc15a00453`,
`67e04874-8a94-410a-8b4d-35d0238467d8` (Humaux).

No customer, production database, provider project, token or broadcast changed.
Orders C native visibility MOU03, Studio UI, real Cloud/Egress, BRI04–07 and
full T08/G06 remain separate unfinished acceptance. Fill source/test commit,
commands, exit codes, evidence hashes and cleanup only after actual execution.
