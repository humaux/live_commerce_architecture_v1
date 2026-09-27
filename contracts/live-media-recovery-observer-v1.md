# Media restart state check — narrow observer candidate

Status: **ABI_CANDIDATE_REVIEW_REQUIRED / IMPLEMENTATION_NOT_RUN**.
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

The local positive capacity bound is **at most 32** unresolved reserved-wire
PROVIDER_MOCK operations. Before child release, one deterministic `LIMIT 33`
admission scan at first DB availability establishes this bound. If it returns
33, admit **no partial observation set**: report `CAPACITY_EXCEEDED`, a count
lower bound of 33 and incomplete coverage. Retain every original responsibility,
persist a bounded capacity event on the episode scope metadata surface and
emit a redacted supervisor health signal. Do not block the native child or
existing cleanup merely because this observer's runtime capacity is exceeded;
release the normally admitted child with explicitly degraded supervision and
zero observer claims. This is not full-scope recovery success.

At most 32 returned identities form an immutable per-restart candidate set.
Lock-time terminalization becomes an explicit recorded disposition, never a
silently dropped row. Repeated child crashes reuse this set and unfinished
deadlines. A DB outage before membership capture makes coverage degraded even
if a later scan is empty; absence from that later scan cannot prove the t0 set
was empty. This bound is not a production sizing claim.

Success is the parent's authoritative **committed readback** of a newly
qualified ROOM/QUERY observation by t0+90s, not `observed_at` before COMMIT,
process readiness, an old observation or a provider ACK. Sample elapsed time
only **after** the successful read response. The private witness is the trusted
parent's attestation of that prior readback, not a DB timestamp used to guess
commit visibility. It may persist later without moving the readback deadline.
No qualifying readback by the deadline means a miss; a timeout that has already
committed is sticky and wins over any later witness, even if the claimed readback
was timely. That race may conservatively report a miss, never a false PASS.
A supervisor crash before witness persistence is unproven unless the witness
actually committed. A late observation/readback may not be backdated into PASS.

## Private SQL surface to review

These proposed signatures belong only to a dedicated recovery role; no public
HTTP route, raw table access, River mutation, Start/Stop reservation or material
decryption grant. Definitions use the existing writer owner and fixed
`search_path=pg_catalog`. Role/readiness admission is independent of strict
worker readiness and verifies exact signatures/ACLs plus the same physical DB.
All mutations use the existing business lock order, fresh post-wait validation
and the same operation/projection, never a River-row-first trigger.

The recovery membership must be included in the exactly-one-authority check
for **all existing pool types**, not just the new pool. Admission rejects mixed
memberships, owner/SET reachability, direct or PUBLIC excess EXECUTE and raw
table/sequence privileges. All old roles are denied every new private function;
the recovery role is denied the existing executor/registrar functions. Exactly
the seven entry points below (plus a separately frozen read-only readiness probe,
if needed) constitute its application authority.

| Private function | Bounded result and authority |
| --- | --- |
| `begin_media_recovery_episode(uuid,bigint,integer,boolean)` | Episode ID, monotonic elapsed ms, capacity 1..32 and explicit `coverage_known`; atomic capacity+1 admission as above. Returns admitted exact identities/baselines/deadlines or fail-whole capacity/coverage disposition, never a partially successful `has_more` page; unfinished episodes remain unchanged. |
| `claim_recovery_observation(uuid,uuid,bigint,bytea)` | Episode, operation, original job and 32-byte token; returns disposition, generation and nonsecret frozen project/version/endpoint/room/egress target only. Requires reserved wire, eligible profile/state, no escalation, remaining existing recovery ceiling, expired old business lease and no prior qualifying observation; grants only a fixed 30s reconcile lease. |
| `record_recovery_observation(uuid,uuid,bigint,bytea,text,text,text,text,bigint,bigint,bigint)` | Episode, operation, generation, token, ROOM/QUERY source, egress, room, status and three provider times; returns checked/terminal plus observation ID. Internally reuses the private projector and records episode correlation atomically; no Stop reservation. |
| `finish_recovery_observation(uuid,uuid,bigint,bytea,text)` | Fenced failure only, using bounded existing uncertainty reason codes; releases only its own lease and retains UNKNOWN/resource liability. No native retry action or escalation. |
| `read_media_recovery_episode(uuid)` | Minimal committed per-operation episode/correlation status, no credentials or caller-selected target. |
| `witness_media_recovery_episode(uuid,uuid,uuid,bigint)` | Episode, operation, qualifying observation ID and post-readback parent elapsed ms 0..90000; trusted parent attestation, exact correlation required. A prior sticky timeout returns a non-PASS disposition and cannot be removed. |
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

`media_native_job` proves identity only, not retry eligibility. Admission and
claim additionally classify original native job state and attempt budget;
completed, cancelled, discarded or otherwise exhausted work cannot obtain a
recovery observation lease merely because its native row still exists.

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
| MRR02 | DB/config/readiness/provider failure, child exits/repeated restarts, active old lease, fail-whole capacity overflow and delayed initial membership capture retain truthful coverage and one deadline. Exercise timely committed readback with delayed witness persistence, late observation/readback, and timeout-first ordering separately; late recovery never erases sticky timeout. Parent survives the failed child; capacity degradation alone does not block existing native cleanup. |
| MRR03 | Old generation/target/token and foreign-role negative controls; cleanup-required remains true with zero Stop reservation; old executor QUERY guard remains intact; missing/escalated original jobs and pre-wire/INPUT work cannot gain observation or dispatch authority. |
| MRR04 | Additive migration, exact ACL/role/physical-DB admission, enabled/disabled/child modes, bounded cleanup and independent focused plus fixed-tree full regression. Original LMR05 deadline and failure evidence retained. |

All gates **NOT_RUN**. The supervisor admission/child handoff, role matrix,
episode state transitions, capacity behavior and exact return-key schema require
independent review before implementation; this candidate is not yet frozen.

## ABI/runtime appendix — candidate for independent review

This appendix fixes implementer-facing names and outcomes; it does not freeze
the design. All SQL calls are `READ COMMITTED`, fixed `search_path=pg_catalog`,
owner `commerce_media_writer`; only the seven private entry points below are
granted to `commerce_media_recovery`. Bad shape, caller or stale fence raises
`ME400`/`ME409`; a returned disposition is a business result, never an implicit
permission to issue Start/Stop. UUIDs are server-generated or validated UUIDs;
all elapsed values are the trusted parent's monotonic milliseconds since t0.

| SQL signature and exact return columns | Dispositions |
| --- | --- |
| `begin_media_recovery_episode(p_episode uuid,p_elapsed_ms bigint,p_capacity integer,p_coverage_known boolean) RETURNS TABLE(disposition text,episode_id uuid,operation_id uuid,job_id bigint,baseline_generation bigint,deadline_at timestamptz,candidate_count integer,coverage_known boolean)` | One row per captured member (`pending` or `overdue`), or one row with null operation/job/baseline for `empty`/`capacity_exceeded`. `candidate_count=capacity+1` is a lower bound on overflow. `deadline_at` is DB diagnostic only. |
| `claim_recovery_observation(p_episode uuid,p_operation uuid,p_job bigint,p_token bytea) RETURNS TABLE(disposition text,generation bigint,project_id text,credential_version bigint,endpoint_identity text,room_name text,egress_id text)` | `claimed`, `busy`, `already_observed`, `terminal_at_lock`, `native_ineligible`, `ceiling`, `overdue`; target columns are nonnull only for `claimed`, `egress_id` nullable for ROOM. No secret material. |
| `record_recovery_observation(p_episode uuid,p_operation uuid,p_generation bigint,p_token bytea,p_source text,p_egress text,p_room text,p_status text,p_started bigint,p_updated bigint,p_ended bigint) RETURNS TABLE(disposition text,observation_id uuid)` | `checked` or `terminal`, with nonnull exact persisted observation ID; rejected/stale inputs raise. |
| `finish_recovery_observation(p_episode uuid,p_operation uuid,p_generation bigint,p_token bytea,p_code text) RETURNS text` | `released`; stale fence raises. Codes are only `remote_unknown`, `not_observed`, `invalid_observation`, `credential_unavailable`, `material_invalid`. |
| `read_media_recovery_episode(p_episode uuid) RETURNS TABLE(episode_id uuid,scope_status text,coverage_known boolean,candidate_count integer,operation_id uuid,disposition text,baseline_generation bigint,observation_id uuid,observation_source text,observation_generation bigint,witness_elapsed_ms bigint,timeout_at timestamptz,cleanup_required boolean)` | One scope row when there are no members; one row per member otherwise. `scope_status` is `pending`, `empty`, `capacity_exceeded`, `overdue` or `finished`; per-member `disposition` is `pending`, `checked`, `terminal`, `terminal_at_lock`, `native_ineligible`, `ceiling`, `timeout` or `witnessed`. |
| `witness_media_recovery_episode(p_episode uuid,p_operation uuid,p_observation uuid,p_readback_elapsed_ms bigint) RETURNS text` | `witnessed`, `already_witnessed`, `timeout_wins` or `unqualified`; only first two prove that member. |
| `timeout_media_recovery_episode(p_episode uuid,p_elapsed_ms bigint) RETURNS TABLE(disposition text,affected_count integer)` | `timed_out`, `already_timed_out` or `already_finished`; `affected_count` is newly timed-out members, not all historic members. |

`begin` accepts `0<=p_elapsed_ms`, `1<=p_capacity<=32` and nonnull
`p_coverage_known`; the parent passes false if DB was unavailable at any point
before the first membership capture. A repeated begin for the same episode
returns its immutable original result; it never rescans, extends deadlines or
upgrades false coverage. A different episode cannot replace an unfinished one
for the same operation. A resolved prior episode may be superseded in the active
projection only after its history has committed. DB deadline is set once as
`clock_timestamp()+max(0,90000-p_elapsed_ms) ms`; it is not a success clock.
`p_capacity` equals configured bounded observer concurrency (reuse validated
worker concurrency where safe), so all admitted candidates fit one wave.

One additive `live.media_recovery_episode_scope` row is necessary per restart:
`episode_id` PK, `capture_elapsed_ms`, `deadline_at`, `capacity`,
`candidate_count` (at least capacity+1 on overflow), `coverage_known`, `scope_status`,
`capacity_exceeded_at`, `timeout_at` and `created_at`. It has no resource state,
provider target, job, lease or retry authority. `operation_events` requires an
operation ID, so it cannot truthfully represent an empty delayed capture or
fail-whole overflow without attributing the event to an arbitrary operation.
The scope row is the bounded durable event for those cases; it cannot establish
resource recovery. Existing `media_execution_state` adds only active episode
ID, generation high-water, member disposition, qualifying observation ID,
witness elapsed/at and sticky timeout; existing `operation_events` adds nullable
`episode_id`, `episode_event_kind`, `observation_id`, `elapsed_ms`, with unique
`(operation_id,episode_id,episode_event_kind)` for one-shot `admitted`,
`terminal_at_lock`, `qualified`, `witnessed`, `timeout`, `native_ineligible`,
`ceiling`. Do not encode correlation in `reason_code` JSON or overwrite old
history. Event insertion, active projection and scope changes share a transaction.
Per-member transition is `pending -> checked/terminal -> witnessed`, or
`pending/checked/terminal -> timeout`; `terminal_at_lock`, `native_ineligible`
and `ceiling` remain explicit unwitnessed dispositions until timeout. A later
resource observation never changes `timeout` to `witnessed`. Scope `empty`
means no captured at-risk members, not a fresh observation; `coverage_known=false`
or `capacity_exceeded` can never produce full-scope PASS. `finished` requires
all captured members witnessed and known complete coverage.

Admission uses a deterministic `(operation_id)` ordered `LIMIT p_capacity+1`
scan of unresolved reserved-wire PROVIDER_MOCK originals, then the established
binding/scope/operation lock order and post-wait revalidation. The
`capacity+1` row
commits `capacity_exceeded` scope metadata and zero members; it never admits
the first `capacity` members. For <=capacity, persist all exact IDs, original job IDs, generation
high-water and dispositions before child release. An item that becomes terminal
while acquiring its business lock is retained as `terminal_at_lock`, never
silently removed or claimed. A missing/final/exhausted native job is retained
as `native_ineligible`; `media_native_job` checks identity only, so inspect
`river_media.river_job` state, `finalized_at`, `attempt`, `max_attempts` under
the locked operation. `attempt>=max_attempts` or completed/cancelled/discarded
cannot acquire an observer lease. The room is the frozen attempt room; known
egress ID chooses QUERY, missing ID chooses FindByRoom. No pre-wire candidate.

Claim additionally requires membership in the immutable episode, no prior
qualifying observation, no escalation/terminal resource, current original job,
`wire_reserved_at`, generation below the existing 4096/24h ceiling, and
`lease_until<=clock_timestamp()`. `busy` never steals an old lease. Successful
claim increments the same operation generation once, sets existing
`lease_mode='reconcile'`, `state='UNKNOWN'`, a 30s lease and token hash; it does
not touch River, dispatch eligibility, attempt counters or Stop budgets.
`record` calls the latest private projector under that same fence, obtains the
exact inserted ROOM/QUERY observation ID, and marks `qualified` in the same
transaction only if generation exceeds the member high-water. The normal
worker's private projector can mark a qualifying fresh ROOM/QUERY too; old
public QUERY cleanup guard and cleanup-query Stop reservation stay intact.
`finish` only releases its own lease and retains unresolved liability.

Parent obtains a committed `read_media_recovery_episode` response containing
the exact observation correlation, then samples monotonic elapsed. Witness
accepts that parent-only elapsed (`0..90000`) even if its SQL commit is later;
it checks episode, operation, observation ID/source/generation and absence of
sticky timeout under the original operation lock. Identical witness retries
are idempotent; conflicting observation/elapsed fails closed. `timeout` requires
parent elapsed >=90000, checks each member under the original lock, records
sticky misses for unwitnessed members and cannot reverse a witnessed member.
If timeout commits first, a later witness returns `timeout_wins`. If DB is
unavailable at deadline the parent emits the redacted signal and retries only
timeout persistence; it never calls the provider after deadline. A later
observation can improve resource knowledge but not the missed deadline.
Each observer SQL call is bounded to 5s and each provider Query/FindByRoom to
10s; provider I/O is outside locks and both fit the fixed 30s lease. Waiting
for an old 30s business lease counts inside the same 90s parent clock.

Runtime knobs: `MEDIA_RECOVERY_SUPERVISED=1` enables the same-binary parent;
unset/`0` runs legacy, and an invalid value fails closed. Parent captures t0 at
entry before env/pools, owns bounded recovery-role pool and episode IDs, and
launches an internal child with `MEDIA_RECOVERY_INTERNAL_CHILD=1` plus a private
inherited stdin control pipe. Child runs the existing native worker only after
reading one release byte; EOF before release means exit, and EOF afterward
cancels the child context. Parent holds the write end open while child runs and
writes release after admission
commit/readback; on DB outage or capacity overflow it releases the native child
with degraded observation coverage. Parent constructs a sanitized child env:
child never receives recovery-role DSN or
observer token. Parent reaps every exit, retries child with capped 1..5s
backoff, keeps unfinished episode/deadline across crashes, and never schedules
observer provider I/O after t0+90s. SIGTERM forwards to child, waits a bounded
5s then kills/reaps; parent/host death loses monotonic proof and requires
external supervisor plus alert collection. Local durable evidence and redacted
stderr are testable; actual human alert delivery remains NOT_RUN.

Add one read-only `live.media_recovery_ready() RETURNS boolean` probe granted
only to the recovery role. It checks the seven exact signatures, return shapes,
owners, ACLs, fixed search path and RLS/guards; startup separately proves the
same physical DB across recovery, worker and executor pools. Migration order:
additive metadata/function/role first,
upgrade admission's exactly-one-role matrix in every pool, qualify old and new
binaries while supervision remains disabled, then enable supervisor. A mixed
binary or role matrix mismatch prevents the 90s claim. MRR01-04, original LMR05,
provider and alert delivery remain NOT_RUN until independently executed.
