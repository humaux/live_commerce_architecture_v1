# T08 bounded MOCK media Stop — LMR01–06

Status: **FROZEN_FOR_MOCK_IMPLEMENTATION / NOT_IMPLEMENTED** (2026-09-27).
Base `8f43c2f`. This specifies the next increment after
[LME](live-media-execution-v1.md), not a Cloud or production approval.
Independent preflight of draft `45c4ac8` found no confirmed P0/P1; the final
narrow clarifications specify error mapping, nonnegative claim age, latest
reservation observation pointer, receipt states and the post-COMMIT time limit.
Security evidence: Humaux `8149a5ee-cebd-44e5-9358-8f3ccc505337`; independent
testability closeout: `045b84e4-4b62-413b-b296-8bae14e42c67`.
LMR01–06 remain NOT_RUN. Source and independent tests must use this same revision.

## Decision and limits

Reuse the existing attempt, long-lived MEDIA operation, native River job and
fenced lease. That operation remains UNKNOWN until terminal proof or its
existing scheduling escalation ceiling (not necessarily the resource lifetime);
its immutable `livekit.egress.start` action records its origin, not a claim that
Stop is a new Start. A single sticky cleanup intent and its bounded Stop wire
reservations are projected on that same attempt. Events distinguish Start,
Stop, cancellation and resource termination. An escalated operation may have
lost its retained native job: RequestStop returns `escalated` explicitly and
does not promise automatic cleanup in that case. Do not create another operation,
queue, lease, retry engine, credential store or SDK.

The existing [bounded Stop algorithm](live-media-controller-v1.md#bounded-mock-stop-recovery--design-accepted-implementation-not_run)
is preserved: at most two same-ID wire reservations, no replacement Start, no
terminal inference from an ACK, timeout, missing row or exhausted budget.
Only PROVIDER_MOCK attempts are admitted. No HTTP/studio route, LIVE intake,
real provider call, global G06 or media-quality acceptance is included.

## Persistence and migration ownership

Integrator reserves forward `0037_live_media_stop.sql` and post-River
`0008_live_media_stop.sql`. Do not modify previous migrations. Upgrade historical
rows without changing their old columns, leases, observations or provider facts.
New Stop fields default to no intent/no reservations; never backfill a Stop.

Extend `live.media_execution_state` with:

- `claim_started_at timestamptz`: database-clock start of the current successful
  claim, nullable for pre-upgrade claims. This is conservative evidence timing,
  NOT another lease. Generation/token/expiry remain exclusively in operations.
- `stop_requested_at timestamptz`, `stop_requested_by uuid`: nullable together,
  immutable after the first explicit merchant request. Use a scoped membership
  reference for the requesting principal; do not overwrite the original actor.
- `stop_wire_count smallint NOT NULL DEFAULT 0 CHECK (0..2)`;
  `stop_first_reserved_at`, `stop_last_reserved_at timestamptz`;
  `stop_first_generation`, `stop_last_generation bigint`;
  `stop_observation_id uuid`: all null iff count=0, otherwise positive/frozen
  target evidence. Count only increases by one. At count=1 first=last; at count=2
  first reservation remains unchanged, second time is >=first+5s and second
  generation >first generation. Once count=2 all reservation fields are frozen.
  `stop_observation_id` always points to the Query authorizing the MOST RECENT
  reservation (and therefore `stop_last_generation`); the first Query remains
  in immutable observation history when the second replaces this pointer.
- `stop_exhausted_at timestamptz`: sticky, nullable; set once on a subsequent
  nonterminal observation after the second wire, with a static exhaustion event.
  It reports exhausted Stop budget, not a closed resource or resettable budget.

`stop_observation_id` must reference an observation of this exact attempt,
operation, generation, project, room and Egress ID. Preserve FORCE RLS, exact
column grants and identity/monotonic triggers. No direct runtime/executor table
authority, DELETE, arbitrary target override or plaintext secret storage.
Add `STOP` as an observation source; no webhook ingress is introduced.

## Merchant intent: exact SQL and Go boundary

`live.request_media_stop(p_auth_hash bytea,p_store uuid,p_session uuid,
p_attempt uuid) RETURNS jsonb`, owned by the existing private media writer;
EXECUTE only to `commerce_runtime` (besides the owner), never media executor,
native worker, registrar or PUBLIC. READ COMMITTED, fixed `pg_catalog` search
path, current `identity.resolve_access` for `live:manage`, matching transaction
tenant/store/principal/revision GUCs, and a final fresh check after lock/event
waits. Follow the existing sorted-bindings → tenant/store → session/program →
authorization/attempt → operation → projection order.

Resolve exact attempt/session/store from authority, not a caller project or
Egress ID. Permit a current authorized manager, not only the original manager.
The prepared grant may have been revoked: stopping an already owned resource
must not require current permission to Start it. Inactive caller/tenant/store,
wrong scope, unknown attempt or stale token fail without writes.

Lazily create the same execution projection if no worker has claimed yet.
Authenticated repeats return the existing intent without extra Stop budget,
operation, job or intent event. Set `cleanup_required=true` and first requester
facts once. Before any committed Start reservation, cancel the operation,
increment its generation, clear lease and append `media_cancelled_before_start`;
keep resource UNOBSERVED (not a fictitious terminal resource). A worker that
already loaded material then loses this race cannot reserve or send Start.
After a committed Start reservation, do not steal an active worker lease:
persist intent, let the current Start result/recovery pin the exact resource,
then let the same job reclaim it. An already terminal/cancelled or durably
escalated operation is not reopened. The original native job remains unchanged.

Exact result keys: `session_id,attempt_id,operation_id,state`, all strings;
state `requested|cancelled_before_start|terminal|escalated`. An idempotency
receipt describes the accepted command, not a fresh resource-status read.
For new receipt keys, preserve `cancelled_before_start` for that prior explicit
pre-reserve cancellation; return `terminal` for other already-terminal/blocked
operations, `escalated` for unresolved durable escalation, else `requested`.

Add `MediaStopInput{SessionID,AttemptID string}` and matching `MediaStopResult`
in package live. `(*MediaPlanner).RequestStop(ctx context.Context,tx pgx.Tx,
scope platform.Scope,token,key string,in MediaStopInput) (MediaStopResult,error)`
reuses `authorize`, `command.Run` (`live.media.stop`), token hashing, revision
GUC and `command.Audit` (`live.media.stop.requested`). Validate IDs/key before
SQL; reauthorize on replay and after execution. No job enqueue or provider I/O.
Reuse static `MP400/401/403/404/409` errors and `mediaPlanError` for this merchant
entry point. Worker functions retain the existing static `ME400/404/409` family;
no provider/database error text is exposed in command results.

## Worker query, reservation and completion

Keep the five LME SQL signatures, return shapes, sealed-material rules and
claim `dispatch|reconcile` modes. A successful claim sets `claim_started_at`
alongside its lease. Unreserved cancellation is terminal for scheduling, and
Start eligibility/load/reserve must deny explicit cleanup. Existing automatic
cleanup reasons (revocation and observed duration cap) also qualify for Stop.
Never treat `start_before` alone as a lifetime deadline.

One new executor entry point:

`live.record_media_cleanup_query(p_id uuid,p_generation bigint,p_token bytea,
p_egress text,p_room text,p_status text,p_started bigint,p_updated bigint,
p_ended bigint) RETURNS text` → `observe|terminal|escalated|stop_reserved`.

Go calls this ONLY with the immediately completed synchronous exact-ID Query
under this claim; never with Start, room discovery, cached history, a webhook
or a Stop reply. Reuse one private observation projection routine so both public
recording paths enforce identical target/timestamp/monotonic/terminal rules.
Existing `record_media_observation` continues to release its lease. This new
path releases its lease normally too, EXCEPT when atomically reserving Stop.

Inside one transaction, append this Query's exact generated observation ID,
merge coherent facts, refresh cleanup reasons, then decide. A reservation needs:

1. Live reconcile lease, exact token/generation/immutable target and MOCK
   authority; committed Start reservation, pinned Egress ID, sticky cleanup.
2. This same transaction's observation is correlated and nonterminal; no
   higher projected ENDING/terminal evidence. Initial Stop permits STARTING or
   ACTIVE; exceptional second Stop permits ACTIVE only.
3. `claim_started_at` is present and DB clock is between it and
   `claim_started_at + 5 seconds` inclusive, rechecked AFTER
   all lock waits and event writes. This deliberately bounds the WHOLE
   claim→load→Query→record/reserve interval, so it conservatively bounds Query
   age without trusting a caller clock. Slow successful Query still records
   valid evidence but cannot authorize Stop; the next lease queries again.
4. Count=0, or count=1 with >=5s since first reservation AND current generation
   >first reservation generation. The latter proves this claim/Query began
   after the first reservation; no second wire within its original lease.
5. Exact observation identity/source/scope/generation is rechecked with the
   final lease/time/budget gate. Record `media_stop_reserved` and increment the
   persistent count before releasing DB locks or issuing I/O. On gate expiry,
   roll back reservation; never return a stale permission.

Return `stop_reserved` only after successful commit, leaving the current lease
open. Unknown commit acknowledgement means NO wire call. A crash after reserve
consumes its budget. Go immediately calls existing LKP `Client.Stop` once using
the already validated frozen project credential and loaded exact target; no
material decryption, URL mutation, automatic HTTP retry or fallback credential.
Check context cancellation before I/O; do not enqueue a detached wire permission
or retain it for a later Work call. The 5s bound applies at the final database
decision, not a guarantee of elapsed time at the remote wire after COMMIT.
No network request runs while database locks are held.

Stop reply uses `record_media_observation(...,'STOP',...)`: permitted only for
the exact current reconcile generation with a Stop reserved in that generation,
same target and live lease. It releases the lease and follows the existing
terminal-proof rules. Bare ACK, ENDING or a terminal enum without coherent
positive end time is not reclaimed. Failures use the existing closed uncertain
codes and fenced finish function. Future claims always Query before another
Stop; they never Start again.

After count=2 a subsequent nonterminal observation records
`media_stop_budget_exhausted` once and leaves UNKNOWN, evidence and usage
liability intact. Continue bounded Query-only recovery; valid terminal proof
may still close the resource. Existing >=4096 generations or >=24h escalation
eventually closes scheduling, NOT liability. There is never a third wire, new
Stop operation or implicit escalation reset. A separate operator-resolution
workflow is outside this increment.

## Authority and old-runtime protection

Only the sixth exact worker entry point is added to media-executor admission.
The merchant request function is never admitted to that executor. New internal
helpers are owner-only. Extend exact ACL/search_path/owner/signature checks and
cross-domain effective/SET-reachable execution scanner; preserve ordinary,
Meta, payment, expiry and registrar exclusions. Native worker stays River-only.
Both pools still require the existing same-physical-DB proof.

Post0008 readiness must validate Stop columns/constraints/triggers and both new
entry points, and become fail-closed if their authority drifts. Preserve native
job lifecycle/linkage/retention invariants. Old worker binaries must fail their
strict exact-function admission when connected to the expanded executor; mixed
old/new workers must not silently turn new cleanup intents into observation-only
loops. Fresh install and upgrade use the same schema, without blanket grants.

## Independent acceptance gates

Use the existing actual-role PG18, TLS provider double, process crash and
implicit-COMMIT acknowledgement-loss harnesses; no Cloud credentials required.

| Gate | Required evidence |
| --- | --- |
| LMR01 | Authorized/replayed Stop command; current token/GUC fences and lock-wait revocation; cross-tenant/attempt denial; pre-Start cancellation beats loaded worker; one operation/job/intent; old-row upgrade and exact ACL/old-binary denial. |
| LMR02 | First Stop dropped before provider acceptance; later exact ACTIVE Query authorizes one same-ID resend; <=2 wire requests, one accepted EOS, terminal proof closes resource. |
| LMR03 | First Stop accepted but reply lost; ENDING suppresses resend; delayed ACTIVE may produce the bounded duplicate with fixture EOS guarded by Once; <=2 wires and <=1 accepted EOS. |
| LMR04 | Wrong source/target/generation/token, expired lease, stale/slow Query, lock waits past freshness, <5s pacing, STARTING-after-first, ENDING and contradictory terminal evidence cannot reserve; competing workers do not duplicate the reservation. |
| LMR05 | Kill real worker after first/second committed reservation and lose implicit COMMIT acknowledgement; restart conservatively consumes budget, sends no third wire/replacement Start and retains resource liability. |
| LMR06 | Automatic revocation/duration cleanup, frozen unavailable credential, count exhaustion/continued Query, original escalation ceiling, terminal-only release and other River lanes remain isolated; all LME/LMP/LMA/LSP regressions. |

Tests must causally distinguish before-accept loss from accepted/lost-reply;
record wire count and accepted EOS separately. A mock's Once semantics prove
only this local algorithm under that explicit assumption. Real Cloud same-ID
duplicate/fault behavior, end-user UI and full T08/G06 remain NOT_RUN.

## Rejected alternatives and upgrade triggers

- Separate Stop operation/lease: unnecessary here because the existing operation
  already owns the bounded active recovery interval and retains unresolved
  liability afterward. Revisit for independently addressable business commands
  or audited operator recovery after escalation/native-job retention.
- Query forever after a lost Stop: cannot reclaim a request lost before acceptance.
  Generic repeated Stop: cannot meet the bounded same-ID recovery rule.
- Another query-ticket store or second lease: the existing claim timestamp plus
  stricter <=5s whole-round bound suffices for this synchronous-only worker.
  Revisit if slow providers require a separately fenced exact Query ticket.
- Reset budget or grant LIVE based on local TLS success: explicitly forbidden.
