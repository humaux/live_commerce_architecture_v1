# T08 durable media start intent — LMP01–07

Status: **FROZEN_FOR_LMP_IMPLEMENTATION**, base `f344574`.
Independent bounded preflight found and closed the producer UPDATE(kind) bypass;
separate post-wait authorization queries plus a DB-clock expiry check retain
existing authority helpers. Native wait/revocation gates remain mandatory.
Implements the next part of [the controller](live-media-controller-v1.md), after
accepted [LMA](live-media-authorization-v1.md). This is real local PG persistence
with MOCK authority, not provider I/O or the complete controller. No HTTP route,
worker execution, LIVE eligibility or G06 claim. Do not introduce another ledger,
queue engine, ORM, generic capability framework or dependency.

## Frozen intended Go boundary

Package `internal/live`, new `media_plan.go`:

- `MediaStartInput { SessionID string; AuthorizationID string;
  ExpectedSessionVersion int64 }` with snake_case JSON tags.
- `MediaStartResult { SessionID string; ProgramID string; AttemptID string;
  OperationID string; JobID int64; RoomName string; State string }` with
  `session_id`, `program_id`, `attempt_id`, `operation_id`, `job_id`, `room_name`,
  `state`. State is the frozen receipt value `READY`, not current transport state.
- `NewMediaPlanner(jobs *river.Client[pgx.Tx]) (*MediaPlanner,error)` rejects nil.
- `(*MediaPlanner).PlanStart(ctx,tx,scope,token,key,in) (MediaStartResult,error)`
  runs inside caller-owned READ COMMITTED `platform.WithScope`. Requires current
  `live:manage`, exact transaction scope and authz revision before/after waits and
  replay. Nil context/service/tx, malformed/nonzero UUIDs, nonpositive version or
  invalid existing command key grammar fail. No partial result on error.

Reuse `authorize`, `command.Run` operation `live.media.start`, `command.Audit`
action `live.media.start.planned`. Canonical request is the ordered Go struct
`{PrincipalID, MediaStartInput}` (principal_id then embedded snake_case fields).
The command receipt digest is SHA-256 of that JSON. Exact scoped key/principal/input
replays the same receipt without a new job or rechecking expired eligibility;
current authentication still applies. Changed input/principal conflicts.
Different keys for the same session conflict, even with the same authorization.

The command callback generates a nonzero operation UUID, inserts a native River
job via InsertTx, then calls the fixed SQL planner. Job schema `river_media`,
kind `live_media_operation_v1`, queue `media_mock_v1`, args exactly
`{"operation_id":"<canonical UUID>","version":1}`, no unique key. A client
configured for another schema cannot successfully plan. Native job ID and
business writes commit together; any error requires rollback, as existing
command APIs. No business authorization comes from knowing job/attempt IDs.

## Migration and typed ownership

Forward `0035_live_media_plan.sql`; post-River `0006_live_media_queue.sql`.
Historical files are immutable. `migrations.Apply` adds native `river_media`
through the same pinned upstream migrator. No old jobs are moved. Preparation
adds a false `live.media_plan_ready()` fence; only the atomic post phase makes
it true and grants narrowly scoped producer rights. Repeated Apply preserves
jobs/receipts and does not leak old worker grants into this schema.

`live.media_attempts` columns:

- id uuid PK, tenant_id/store_id/session_id/program_id/authorization_id uuid,
  original_principal_id uuid, start_operation_id uuid, start_key text,
  request_hash bytea(32), environment text fixed MOCK,
  execution_profile text fixed PROVIDER_MOCK, room_name text, created_at timestamptz.
- All required/nonzero IDs; created_at is DB clock. Room is generated from id:
  `lc_` plus the UUID without hyphens, matching LKM; no caller room/endpoint input.
- Unique (tenant_id,store_id,id), (tenant_id,store_id,session_id), authorization_id,
  start_operation_id, room_name and (tenant_id,store_id,start_key).
- Composite FK (tenant,store,session,program) to the actual program, and
  (tenant,store,session,authorization,id) to matching prepared authorization
  (its attempt_id), backed by explicit unique keys on referenced tables.
  Original principal FK to identity.memberships.
- Add integration.operations.media_attempt_id. Mutually exclusive actor CHECK:
  legacy branches retain their previous predicates plus media_attempt_id IS NULL;
  MERCHANT additionally cannot use the reserved livekit.egress.start/stop actions.
  Pre-existing such MERCHANT rows cause a static operator-review migration error,
  never automatic relabelling into trusted MEDIA authority.
  MEDIA_ATTEMPT has non-null media_attempt_id and principal_id, null buyer/payment
  fields, provider livekit, purpose service, action livekit.egress.start.
- Composite operation FK (tenant,store,media_attempt_id) to attempt. Reciprocal
  attempt FK (tenant,store,start_operation_id) to operation is DEFERRABLE INITIALLY
  DEFERRED. An insert-only deferred consistency check matches exact attempt,
  principal, media binding/version/project, job/action/request/hash/key, not merely
  existence in the same scope. Stop operation support is a later reviewed change.
- Operation request is exactly safe JSONB
  `{ "attempt_id": id, "session_id": session_id, "version": 1 }`.
  No ciphertext, key ID, stream URL, evidence or secret in operation/job/audit/DTO.

FORCE RLS and revoke PUBLIC on new tables. Existing private NOLOGIN
commerce_media_writer owns fixed functions and receives minimum grants/policies.
It may INSERT/SELECT attempts but not UPDATE/DELETE their immutable identity.
commerce_runtime receives only tenant/store scoped SELECT on safe attempts and
EXECUTE on planner/readiness. Registrar does not get planner/queue authority.
Ordinary commerce_worker, Meta and buyer/payment roles get no media attempts,
media queue, prepared ciphertext or new function authority.

## Fixed planner and lock semantics

`live.plan_media_start(p_hash bytea,p_store uuid,p_session uuid,p_authorization uuid,
p_expected bigint,p_key text,p_operation uuid,p_job bigint)
RETURNS jsonb`, VOLATILE SECURITY DEFINER, fixed pg_catalog search_path, owned by private
media writer; REVOKE PUBLIC; grant EXECUTE only commerce_runtime.

Requires a 32-byte token hash, valid nonzero IDs, expected>0, positive job,
key grammar and READ COMMITTED. Resolve real merchant session/permission with
identity.resolve_access, compare resolved tenant/principal/revision to transaction
GUC scope (revision rechecked in Go), not caller-supplied ownership. No plaintext
token is persisted. Private writer gets USAGE on identity and only SELECT of
identity.sessions token_hash/principal_id/audience/revoked_at/expires_at, alongside
EXECUTE identity.resolve_access. Sessions currently use ACL isolation, not RLS;
do not grant runtime this table access or add unrelated policies. Resolve access
again in a separate inner SELECT after waits/final domain writes, comparing the
fresh status/tenant/principal/revision with the initial result. A VOLATILE caller
obtains a new snapshot for each inner query; do not reuse a cached record. Native
direct-SQL wait→grant/revision revocation tests must demonstrate this boundary.
Check token expiration using DB wall clock at
final return, not initial statement timestamp; a lock wait cannot extend it.

Read locators, then acquire all media/destination bindings sorted by UUID →
tenant/store → session/program → authorization → attempt/operation → event.
Private functions must not reverse the common registry order. Confirm immutable
locator identities again under locks. Lock authorization to serialize revocation.
Recheck current active scope, live permission, unrevoked MOCK_FIXTURE authority,
exact session/version/layout, enabled current exact binding/version/provider/
asset set, 1–2 destinations and deadline/caps; reuse LMA predicates, not raw JSON.

Require DRAFT and no prior attempt for the session. Persist attempt + MEDIA READY
operation (generation0, no lease) + READY operation event reason media_start_planned,
set program READY, retaining frozen session version. SQL computes its own semantic
hash as SHA-256 of UTF-8 canonical JSONB text containing principal_id, session_id,
authorization_id and expected_session_version from the typed authenticated input.
Attempt and operation share this digest. This is distinct from command.Run's Go
JSON receipt digest; neither is a substitute for live token/authority validation.
Verify exact native job linkage. Recheck wall-clock authority and authentication
after the final writes, so a waited child/event insertion cannot admit expired
Start authority. A failure rolls back all function writes, and caller transaction
rollback removes job/receipt/audit too.

Go command.Run supplies replay before the callback; the SQL path must also fail
closed on repeated/different IDs instead of overwriting any existing attempt.
Known SQL validation/not-found/conflict map to command errors; authorization maps
to platform errors. Static messages must not echo secret/input values. Do not
mask unexpected database failures or turn them into success.

Freeze title/schedule/version and aspect after an attempt exists at database
boundary, including direct SQL by runtime and real concurrent waits. Merely
changing Go UpdateDraft is insufficient. Use VOLATILE BEFORE UPDATE triggers on
session title/scheduled_at/version/updated_at and program aspect_ratio. Once the
row is locked, read exact scoped attempt existence with a fresh inner query and
reject changes after planning. Do not acquire session locks from a program trigger
(that would invert session→program order). No unauthorized program state transition. Extend
program state vocabulary to DRAFT/READY only for this increment; do not label
READY as actual running/live. Existing GetDraft may return READY after planning;
its historic DRAFT-only guarantee applies to unplanned drafts. Original draft
receipt replay remains frozen and does not mutate current program.

## Legacy isolation and River linkage

Restrict runtime operation INSERT to MERCHANT and event INSERT to corresponding
MERCHANT operations. Legacy worker operation/event policies exclude MEDIA.
Legacy claim/complete SQL must reject MEDIA even by known UUID, before any lease,
generation/event change; Go actor filtering alone is not security. Preserve exact
old MERCHANT/BUYER_PAYMENT_QUERY behavior with static forward CREATE OR REPLACE
copies (latest claim=0016, complete=0009), adding MEDIA rejection before mutation
and rechecking after operation lock. Retain owner/signature/search_path/EXECUTE.
Do not rename privileged helpers or derive SQL from mutable function definitions.

Native River producer grants are SELECT/INSERT/UPDATE(kind) plus sequence USAGE
for runtime; no state/args/queue mutation or delete. Reuse the existing Meta lane's
immediate BEFORE INSERT OR UPDATE family guard: fixed kind/queue, canonical args,
and no changed kind/args/queue on an existing row, even when constraints have
already been switched to IMMEDIATE. River's no-op UPDATE(kind) remains valid.
Deferred INSERT constraint
trigger checks final job unchanged from NEW, canonical args with textual version
`1` (not `1.0`), kind, queue, no unique key and exact persisted operation/attempt
job ID. Reject orphan, cross-scope, extra-key, other-schema or rewritten jobs.
No foreign-key retention trap: future River pruning must not delete history or
require retaining completed jobs forever. No worker is granted this queue yet;
future media execution role must be independently admitted and reviewed.
Readiness checks exact trigger enablement, owner/security/search_path/signature,
scope guards and current nonterminal job linkage; missing post phase fails closed.
PlanStart checks this boolean gate before any attempt/job creation. Existing
river/river_meta/river_payment/river_expiry lanes reject the reserved media kind
or queue using a fixed immediate guard, so no forged media job can reach an
ordinary native worker. No MEDIA jobs exist in these lanes at the accepted base;
unexpected pre-existing rows block migration for operator review, never move them.

## Independent acceptance gates

- LMP01: real scoped merchant, valid prepared authority, one/two destinations;
  exact IDs/room/READY result and attempt/operation/event/job/receipt/audit; zero
  provider calls. Wrong-schema River client fails with no partial transaction.
- LMP02: same-key concurrent calls yield one set; changed input/principal conflicts;
  different keys/authorizations for one session produce only one attempt; exact
  authenticated replay after authority expiry/revoke yields frozen receipt and no
  new writes; revoked merchant cannot replay.
- LMP03: missing live permission, forged GUC, cross-store/tenant, stale revision,
  invalid IDs/token hash/key, invalid/expired/revoked authority and changed bindings/
  session version/aspect fail with zero effects. READY forbids new registration.
- LMP04: direct SQL runtime/worker cannot forge MEDIA operation/event, claim,
  complete, read ciphertext, mutate/delete attempt or change frozen draft. Actual
  role positives remain usable; legacy MERCHANT/payment operations regress.
- LMP05: rollback/fault after native job/attempt/event leaves all counts unchanged;
  deferred trigger rejects orphan/rewritten/mismatched jobs including version1.0;
  SET CONSTRAINTS ALL IMMEDIATE then UPDATE(kind) cannot bypass the immediate guard;
  repeated migrations preserve committed media and legacy facts. Missing/disabled
  queue gate fails closed. No existing customer database.
- LMP06: observed PG lock waits then binding/version/revoke/token/start deadline
  changes fail closed; concurrent plan/update gives one valid serialized outcome;
  final-write expiry is causally observed, not a sleep-only assertion.
- LMP07: source diff/security review, focused LMP+LMA+LSP PG18/race, full root
  PG18/race/vet. No skipped positives, no lowered old thresholds. Browser/Cloud/
  controller execution NOT_RUN remains explicit.

Why not a generic dispatcher extension: it would expose media cleanup authority
and credentials to unrelated workers. Reuse the same ledger with typed family
isolation instead. Future upgrade signal: when fenced media claim/material/
observations and bounded Stop are frozen, add the dedicated media worker and
run MLA recovery gates; do not consume these jobs early just to empty a queue.
