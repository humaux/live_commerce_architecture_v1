# Live broadcast intents v1 — LBI01–06

Base `8ea20ae`; successor to [live planning](live-planning-v1.md). This is a
LOCAL durable control-intent slice, **not operational start/stop or G06**. It
registers no provider route, HTTP handler or worker. No secrets or provider calls.
Actual credential custody, verified asset ownership, provider policy, historical
reconciliation and resource reclamation are required before registering a route.

## Reuse and boundaries

Use `platform.WithScope`, existing live authorization, `command.Run/Audit`, and
`core.Service.Plan` with the existing insert-only `river_external` client. No new
queue, SDK, engine, permission vocabulary or automatic grant. Every returned error
requires caller rollback. RC, exact GUC scope, token/revision/permission checks
before work, after lock waits and before return (including replay) remain required.

A binding is merchant configuration, NOT proof of ownership or eligibility.
Destination providers are exactly `facebook` and `instagram`, consistent with
Meta intake, assets 1–40 ASCII digits. Media binding provider is `livekit`, asset
an opaque project identifier matching `[A-Za-z0-9_-]{1,80}`. Never accept a stream
URL/key, arbitrary room name or Egress ID from merchant input. No PAYUNi credential
fields are repurposed for media secrets.

One destination per provider per session (maximum two). Configuration uses the
existing session version CAS; a media attempt freezes all destinations. Exactly
ONE start attempt per session in this slice, including after failure/UNKNOWN:
changing the command key cannot create another. Restart/recovery is a later
reviewed transition, not deletion or resetting old records.

Program `DRAFT` remains the configuration record, not transport/audience truth.
After an attempt is planned, both UpdateDraft and destination changes reject
(historical identical command replay still returns its frozen receipt). Start/stop
operation states are exposed separately; audience state is always `UNKNOWN`.
Neither queued intent nor provider acceptance proves RUNNING, LIVE or ENDED.

## Frozen Go surface (`internal/live`)

All functions take `(ctx context.Context, tx pgx.Tx, scope platform.Scope,
token string, ...)`. No pool ownership; no generic injected callbacks.

- `DestinationInput { BindingID string; BindingVersion int64 }` (snake_case JSON).
- `Destination { ID, SessionID, BindingID, Provider, ExternalAssetID string;
  BindingVersion, SessionVersion int64; State string }`, JSON `destination_id`,
  `session_id`, `binding_id`, `provider`, `external_asset_id`, `binding_version`,
  `session_version`, `state`. State `CONFIGURED_UNVERIFIED` only.
- `SetDestination(ctx,tx,scope,token,key,sessionID string,expectedVersion int64,
  in DestinationInput) (Destination,error)` requires live:manage AND
  integration:read. Command `live.destination.set` binds actor/session/version/input.
  Locks session then program; denies existing attempt; locks current enabled
  binding/version. Upsert slot by provider, increment session version once, audit
  `live.destination.configured`. No jobs. Same-key replay remains authenticated.
- `StartInput { MediaBindingID string; MediaBindingVersion int64 }` (JSON
  `media_binding_id`, `media_binding_version`).
- `MediaAttempt { ID, SessionID, ProgramID, RoomName, MediaBindingID string;
  MediaBindingVersion, Generation, SessionVersion int64; StartOperationID,
  StopOperationID, StartState, StopState, AudienceState string; CreatedAt time.Time }`.
  JSON `attempt_id` then snake_case. Generation 1; absent stop fields empty strings.
  Never include request, principal, token, provider reference, raw errors or URLs.
- `NewBroadcast(operations *core.Service) (*Broadcast,error)` rejects nil.
- `(*Broadcast).PlanMediaStart(ctx,tx,scope,token,key,sessionID string,
  expectedVersion int64,in StartInput) (MediaAttempt,error)` requires live:manage,
  integration:execute AND integration:read. Command `live.media.start` binds full
  canonical input+actor. Lock session/program, require expected version and no
  attempt, read ordered destinations (at least one), lock all referenced bindings
  in ID order and verify enabled/current frozen version/provider/asset.
  Allocate attempt UUID and server room `lc_` + UUID without hyphens. Call core.Plan
  with media binding, purpose `service`, action `livekit.egress.start`, semantic key
  `live:start:<attemptUUID>`, safe request below. Insert immutable attempt, increment
  session version once and audit `live.media.start_planned`, all in SAME transaction.
- Start request exact fields: `version:1`, `attempt_id`, `session_id`, `program_id`,
  `room_name`, `aspect_ratio`, `destinations` sorted by provider. Each destination:
  `destination_id`, `binding_id`, `binding_version`, `provider`, `external_asset_id`.
- `(*Broadcast).PlanMediaStop(ctx,tx,scope,token,key,attemptID string)
  (MediaAttempt,error)` same three permissions, command `live.media.stop` binds
  actor+attempt. Locks session/program/attempt; reject an existing stop under a
  different key. Start must be `SUCCEEDED` with an Egress reference matching
  `EG_[A-Za-z0-9_-]{1,100}`; a fixed remote ID alone is not enough. Read the start
  operation FOR SHARE while planning. Recheck frozen media binding enabled/version.
  core.Plan uses purpose `service`, action `livekit.egress.stop`, semantic key
  `live:stop:<attemptUUID>`, request exactly `version:1`, `attempt_id`,
  `start_operation_id`, `egress_id`. Persist one stop operation pointer and audit
  `live.media.stop_planned` atomically. Do not increment configuration version.
  Stop intent acceptance is not evidence of remote resource reclamation.
- `GetMediaAttempt(ctx,tx,scope,token,sessionID string) (MediaAttempt,error)` requires
  live:read AND integration:read; one joined read of attempt/start/optional stop,
  no writes. No partial DTO on errors. Recheck auth before return.

## Database and lock contract

Migration 0034 creates scoped `live.destinations` and `live.media_attempts`, forced
RLS, composite FKs to session/program/bindings/operations, no PUBLIC/buyer/worker
access. Runtime may change destination binding/version before an attempt only;
attempt identity, actor, room, media binding and start operation are immutable.
Stop pointer transitions NULL to a fixed ID only, never replaces/unsets it.
DB triggers protect draft/destination configuration after an attempt, including
direct SQL; SQL constraints/trigger checks bind provider/asset and operation tuple,
request attempt identity, frozen binding/version and safe room. Start and stop
operation cannot be borrowed from another attempt, actor, session or tenant.
Runtime can't delete these records. No state-column grants added to programs.

Lock order: outer command → session → program → attempt (if present) → sorted
binding SHARE locks → core.Plan command lock → operation. Internal semantic keys
are server-derived, distinct from outer keys. No network while holding DB locks.
Get is read-only. Existing core lease/generation rules own dispatch; do not create
a second lease mechanism. One attempt's room is only correlation, not proof the
provider enforces idempotency or retains history.

## Gates (initially NOT_RUN)

- LBI01: real PG create both destination slots, CAS/replay/canonical exact DTO;
  invalid provider/asset/stale/disabled/cross-scope bindings rejected; no jobs.
- LBI02: start freezes ordered safe request, pair/attempt/op/job/audits/receipts
  atomic; forced rollback leaves zero new facts; concurrent same-key replay one
  attempt/job, different keys/versions cannot duplicate; no URLs/secrets in payload.
- LBI03: stop requires known successful start+valid reference, one stop only,
  stable operation keys, replay/different-key conflict, UNKNOWN never creates a
  fresh attempt or stop; stops report intent only. Use real core Claim/Complete
  with restricted worker for simulated outcomes (explicitly MOCK provider).
- LBI04: forged GUC/RC/revision, revoked/expired/missing permissions, cross-scope
  IDs rejected including receipt replay; observed PG lock wait across token expiry
  commits no new facts. Get changes no facts and never leaks raw operation data.
- LBI05: FKs/forced RLS/column ACL/immutable identities/stop pointer and frozen
  config constraints tested through ordinary runtime; cannot direct-SQL substitute
  another operation, reset attempt, add/rebind destinations or edit draft post-start.
- LBI06: independent tests + source/security review + root PG/race/vet replay;
  historical LSP still passes; full migration regression. No full G06/provider claim.

## Stop line

Before an actual route exists, every LBI operation stays non-operational. Future
route Check must load the matching immutable attempt and validated scoped
credentials/asset-ownership proof; generic merchant Plan cannot bypass it.
Current [LiveKit reference](https://docs.livekit.io/reference/other/egress/api/)
documents StartEgress and active-only ListEgress. An empty query cannot establish
that an uncertain start never happened. Acknowledged stop isn't terminal cleanup.
UNKNOWN needs query-only/manual resolution, never a fresh start. User permission,
quota, media lifecycle projection, webhook history and cleanup still need delivery.
