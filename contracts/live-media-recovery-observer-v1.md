# Media restart state check — narrow observer candidate

Status: **DESIGN_REVIEW_REQUIRED / IMPLEMENTATION_NOT_RUN**.
Owner target: fresh state reconciliation within **90 seconds**, or retained
unresolved responsibility plus an alert; decision
`f0b36444-a4ea-4112-926d-596cfb7fd89f`. This does not promise stopped resources,
resumed cleanup automation, native queue recovery or production deployment.

This replaces the native-tuning candidate for the MRR unit only. The rejected
draft and failure evidence remain in `live-media-recovery-v1.md`. Scope research
`fbbcf3ef-dfef-4b57-9278-902386101ff9` identifies existing fenced observation
primitives and the two wrappers that cannot be reused unchanged.

## Reuse and limits

Keep River Rescue/Retry/MaxAttempts, native states, original job identity and
Stop budgets unchanged. No pending parking, earlier exhaustion escalation,
replacement job or second operation/queue is introduced. Reuse the original
MEDIA operation lease, frozen project/client, `Query`/`FindByRoom`, and private
`project_media_observation` validator and merger. Preserve every old public
SQL signature, including the cleanup-required QUERY rejection.

Only `PROVIDER_MOCK` attempts with a previously reserved Start wire and retained
exact original job are eligible for a fresh check. Nonterminal `DISPATCHING`
after a lost Start acknowledgement is included, as is `UNKNOWN`. A missing,
mismatched or exhausted/escalated job cannot be presented as recovered; retain
its liability and report the cause. Pre-wire work has no escaped resource and
is not dispatched by the observer. BIC/INPUT has a different custody path and
is not silently covered by this reuse.

One qualifying observation per operation/restart episode is sufficient. It is
not a second perpetual polling worker. A still-active resource with pending
cleanup remains active/unknown with `cleanup_required` retained; the result
must say **state checked**, never **cleanup resumed**. Existing LMR05 failure,
native exhaustion/liveness issues and input activation gates remain separately
open; accepting MRR may not waive them.

## Supervision and clock

Use the same media-worker executable in a supervisor parent and internal worker
child mode. Preserve the existing worker `run` path. Supervised mode is explicitly
enabled; absent/disabled remains the legacy path with no 90s capability claim.
Enabled mode with missing/invalid monitor configuration fails closed and emits
a redacted health signal, never silently falls back to legacy mode. Deployment
must enable and qualify supervision before advertising the 90s capability.

The parent samples monotonic t0 before admission/replacement launch, registers
episodes before releasing the child to work, and remains alive if child config,
readiness, DB startup or execution fails. All admission and old-lease waiting
count. Repeated child failures retain the same unfinished episode and deadline.
Only a later crash after a resolved episode may create a new episode.

On first DB availability, derive the stored DB deadline from the parent's
already elapsed time. A DB outage cannot be called persisted evidence; the
parent emits the deadline diagnostic regardless and writes overdue evidence
when DB returns. A parent/host crash cannot reconstruct a lost monotonic clock:
retain existing overdue/unfinished DB episodes, and require host supervision
and external alert collection as an explicit deployment gate.

Success is the parent's authoritative **committed readback** of a newly
qualified ROOM/QUERY observation by t0+90s, not `observed_at` before COMMIT,
process readiness, an old observation or a provider ACK. Parent readback creates
a durable witness for that observation/episode. No witness by the deadline
irrevocably means a miss; later evidence cannot erase the timeout event. A
supervisor crash before persisting a success witness remains unproven, not PASS.

## Private SQL surface to review

These proposed signatures belong only to a dedicated recovery role; no public
HTTP route, raw table access, River mutation, Start/Stop reservation or material
decryption grant. Definitions use the existing writer owner and fixed
`search_path=pg_catalog`. Role/readiness admission is independent of strict
worker readiness and verifies exact signatures/ACLs plus the same physical DB.
All mutations use the existing business lock order, fresh post-wait validation
and the same operation/projection, never a River-row-first trigger.

| Private function | Bounded result and authority |
| --- | --- |
| `begin_media_recovery_episode(uuid,bigint,integer)` | Episode ID, monotonic elapsed ms and limit 1..32; returns JSON with exact operation/job identities, baseline generation, deadline and `has_more`; unfinished episodes remain unchanged. Excess capacity is explicit, never reported as a complete scan. |
| `claim_recovery_observation(uuid,uuid,bigint,bytea)` | Episode, operation, original job and 32-byte token; returns disposition, generation and nonsecret frozen project/version/endpoint/room/egress target only. Requires reserved wire, eligible profile/state, no escalation, remaining existing recovery ceiling, expired old business lease and no prior qualifying observation; grants only a fixed 30s reconcile lease. |
| `record_recovery_observation(uuid,uuid,bigint,bytea,text,text,text,text,bigint,bigint,bigint)` | Episode, operation, generation, token, ROOM/QUERY source, egress, room, status and three provider times; returns checked/terminal plus observation ID. Internally reuses the private projector and records episode correlation atomically; no Stop reservation. |
| `finish_recovery_observation(uuid,uuid,bigint,bytea,text)` | Fenced failure only, using bounded existing uncertainty reason codes; releases only its own lease and retains UNKNOWN/resource liability. No native retry action or escalation. |
| `read_media_recovery_episode(uuid)` | Minimal committed per-operation episode/correlation status, no credentials or caller-selected target. |
| `witness_media_recovery_episode(uuid,uuid,uuid,bigint)` | Episode, operation, qualifying observation ID and parent elapsed ms 0..90000; durable parent success witness, exact correlation required. It cannot remove an existing timeout. |
| `timeout_media_recovery_episode(uuid,bigint)` | Parent elapsed ms >=90000; under each original operation lock records one sticky episode timeout/event for unwitnessed work, including late qualifying observations. Does not clear active leases, fabricate UNKNOWN over a proven terminal state or issue provider calls. |

Reuse the execution projection for active episode ID, generation high-water,
deadline, qualifying observation ID, witness elapsed/at and sticky timeout at;
retain episode history in existing operation events. Event identity/dedup must
be enforced transactionally per operation+episode, not just a static reason.
Replacing resolved active fields must retain the prior episode event evidence.

The ordinary claim is not suitable unchanged: it can dispatch pre-wire work or
escalate at its ceiling. The new claim may advance the same generation once per
actual observation attempt but cannot reset counters or spend a Start/Stop
budget. An active lease is waited for, never stolen. New generation/token fences
reject delayed results from the former child.

The old public QUERY wrapper rejects nonterminal cleanup-required observations;
the cleanup-query wrapper may reserve a Stop. Neither is repurposed. Only the
new episode-bound wrapper may use the private projector for an observation-only
check while preserving cleanup liability. The central projector can also mark
a valid new normal-worker ROOM/QUERY as qualifying when generation exceeds the
episode high-water; any later transaction error rolls that mark back. Bare STOP
acknowledgements, `finish_uncertain` and native state transitions do not qualify.

## Go/runtime bounds to freeze

Use a bounded recovery-role pool and existing validated project selector, not
merchant-supplied URLs. Provider I/O stays outside DB locks. Recovery code may
call only Query/FindByRoom, never the worker's generic `Work` dispatcher.
Use fixed bounded request/SQL timeouts within the 30s lease; stop scheduling
observation attempts after the 90s episode deadline. Timeout persistence may
retry after DB recovery without performing another provider action. Missing
credentials, uncertain provider replies, lease contention, capacity overflow
and readiness mismatch are diagnostic outcomes, not successful checks.

Local alert evidence is a durable episode event and independently observable
redacted supervisor output. Human notification delivery, parent-of-parent
health detection, Cloud/provider capacity and customer deployment remain NOT_RUN
until their real collection/delivery path is configured and tested.

## Acceptance gates

| Gate | Required evidence |
| --- | --- |
| MRR01 | Real supervisor/child SIGKILL/restart, no timestamp aging, monotonic committed fresh ROOM/QUERY readback <=90s with original attempt/operation/job, unchanged River parameters/attempts and zero observer Start/Stop. Include escaped Start with no Egress ID and ordinary known-ID Query. |
| MRR02 | DB/config/readiness/provider failure, child exits/repeated restarts, active old lease, capacity overflow and post-deadline commit retain one deadline and sticky timeout; late recovery does not erase it. Parent survives the failed child. |
| MRR03 | Old generation/target/token and foreign-role negative controls; cleanup-required remains true with zero Stop reservation; old executor QUERY guard remains intact; missing/escalated original jobs and pre-wire/INPUT work cannot gain observation or dispatch authority. |
| MRR04 | Additive migration, exact ACL/role/physical-DB admission, enabled/disabled/child modes, bounded cleanup and independent focused plus fixed-tree full regression. Original LMR05 deadline and failure evidence retained. |

All gates **NOT_RUN**. The supervisor admission/child handoff, role matrix,
episode state transitions, capacity behavior and exact return-key schema require
independent review before implementation; this candidate is not yet frozen.
