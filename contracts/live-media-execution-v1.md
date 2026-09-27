# T08 dedicated MOCK media execution — LME01–08

Status: **PASS_LOCAL_MOCK_EXECUTION_ONLY** (2026-09-27), tested main `4345732`.
Interfaces frozen at `1fbae8a`, clarification `e96c75f`; independent LME01–08 and
root full 623-test PG18/race/vet passed. See [acceptance evidence](../docs/implementation/2026-09-27-live-media-execution-acceptance.md).
Implements executable Start and observation/recovery after [LMP](live-media-plan-v1.md).
This is not Stop, LIVE intake, studio UI, resource reclamation or G06 acceptance.
Reuse the existing operation ledger, River, LKP client and LKM custody. No new
SDK, generic dispatcher branch, queue engine or standalone business ledger.

## Authority, migrations and persistence

Use forward0036 and post-River0007; no edits to historical migrations. Forward
business functions must not require native River relations at fresh-install
definition time. Post0007 replaces the initial-only UPDATE guard/readiness from
post0006 only after the fifth native schema and original guards exist.

Two distinct NOLOGIN application roles: `commerce_media_worker` administers
native `river_media` only; `commerce_media_executor` may execute the five fixed
business functions below, without direct business-table/ciphertext SELECT or
writes. Existing private `commerce_media_writer` owns the SECURITY DEFINER
functions (VOLATILE, fixed `pg_catalog` search_path) and minimal scoped policies.
No EXECUTE via PUBLIC, merchant, ordinary worker, Meta, payment, expiry, buyer,
registrar, writer membership or mixed/SET-reachable combinations at startup.
The two runtime pools are separately admitted and must prove the same physical
database via existing `platform.ValidateSameDatabase`, not DSN comparison.

The private writer needs only column SELECT on `identity.principals(id,active)`,
`identity.memberships(tenant_id,principal_id,active)` and
`identity.store_grants(tenant_id,store_id,principal_id,permission)`, with schema
USAGE. Join the original principal to the already-locked active tenant/store;
use fresh VOLATILE queries after lock waits and after final event writes, never
an old browser token or token-based `resolve_access`. No identity grants go to
the executor or native worker. Add a MEDIA-only UPDATE policy on
`integration.operations` and private-writer column UPDATE grants limited to
`state,generation,lease_mode,lease_until,lease_token_hash,result_code,
provider_reference,updated_at`; do not inherit `commerce_integration_writer`.

Add `live.media_execution_state`, one row per immutable attempt, created lazily
by its first claim, not by rewriting the planner. Composite scope/attempt and
operation FKs preserve exact origin. Fields:

- attempt_id PK; tenant_id, store_id, session_id, authorization_id,
  operation_id UNIQUE, project_id: frozen from the attempt/authorization.
- wire_reserved_at nullable timestamptz and wire_generation nullable bigint:
  both null or both present; once present immutable, exactly one Start reservation.
- egress_id nullable text with LKP ID grammar; once pinned immutable;
  UNIQUE(project_id,egress_id), preventing cross-attempt/tenant resource adoption.
- resource_state `UNOBSERVED|OBSERVED|TERMINAL`; transport_status empty or exact
  LKP enum; started_at_ns, updated_at_ns, ended_at_ns nonnegative bigint defaults0.
- cleanup_required boolean defaultfalse (sticky), escalated_at nullable timestamp,
  escalation_code empty or closed code below; updated_at from DB clock.

Add append-only `live.media_observations`: UUID id, composite scope/attempt/op,
generation, source `START|ROOM|QUERY`, exact project/room/Egress ID and LKP status,
nonnegative provider timestamps, observed_at DB clock and canonical report hash.
UNIQUE(attempt_id,generation,source,report_hash). Hash is computed by SQL from
typed safe values, never trusted caller bytes. A repeated stale completion may
fail; it must never duplicate/change evidence. No raw response, URL, API key,
lease token or provider error message in tables, events, logs or job arguments.

Both tables FORCE RLS. Only private writer can insert/update allowed projection
columns or append observations. No DELETE or identity update; private writer
UPDATE policies/column grants and triggers protect once-pinned fields. Existing
`live.media_attempts` remains immutable; this is its execution projection, not
a second operation/lease ledger. All lease/generation fields remain exclusively
on `integration.operations`.

## Exact SQL boundary

All functions use READ COMMITTED, static errors and the existing global order:
sorted media/destination bindings → tenant/store → session/program → prepared
authorization/attempt → operation → execution projection/observation/event.
Resolve locators without authority, then recheck exact scope/frozen identity
under locks. Repeat DB-clock lease checks after event/observation writes.
Never hold these locks during a provider request.

1. `live.claim_media_operation(p_id uuid,p_job bigint,p_lease_seconds integer,
   p_token bytea) RETURNS TABLE(disposition text,generation bigint,mode text)`.
   Lease 5..300 seconds, 32 random token bytes (persist hash only), exact native
   job ID/kind/queue/args and MEDIA actor/start action/PROVIDER_MOCK linkage.
   Reject generic/payment and wrong-job inputs before mutation. Fresh lease →
   `busy`; persisted terminal resource, undispatched policy block or escalated
   projection → `terminal` or `escalated`, no provider action. Otherwise increment
   generation, persist lease/event and return `claimed` with `dispatch` only if
   no Start wire reservation has EVER been committed; else `reconcile`.
   Every successful call returns exactly one row with no NULL fields. Unclaimed
   outcomes return the persisted generation and empty mode; claimed returns
   its newly persisted generation and `dispatch|reconcile`.
   Dispatch requires current original principal/membership/store live:manage,
   current active tenant/store, exact enabled bindings, unrevoked MOCK authority,
   unchanged session/layout, current start deadline and positive frozen caps.
   This is worker service authorization of the original principal, not reuse of
   an expired browser session token. Recheck current rows after waits. Policy
   denial before any reservation persists BLOCKED_POLICY/event and returns
   terminal; revocation after reservation must not disable observation/recovery.
2. `live.load_media_material(p_id uuid,p_generation bigint,p_token bytea)
   RETURNS jsonb`. Strict lease/mode/identity gate; exact safe JSON keys:
   `tenant_id,store_id,session_id,attempt_id,project_id,endpoint_identity,
   credential_version,material_version,room_name,aspect_ratio,egress_id,mode,
   key_id,nonce_hex,ciphertext_hex`. Numeric versions, strings otherwise;
   egress_id is empty when unknown. Dispatch returns sealed material after a
   fresh eligibility check; reconcile returns empty key/nonce/ciphertext, never
   destination secrets, and only frozen project credential/owned target identity.
   Reads do not reserve a wire call. No decrypted URL is returned by SQL.
3. `live.reserve_media_start(p_id uuid,p_generation bigint,p_token bytea)
   RETURNS void`. Last committed DB gate after Go selects the exact credential
   and decrypts/validates material, immediately before Start I/O. Requires a live
   dispatch lease, current mutable eligibility and no prior reservation. Sets
   wire_reserved_at/wire_generation and a static event atomically. Repeating it
   is an error, not permission to resend. Committed reservation followed by crash
   consumes the one wire allowance; all later claims are reconcile-only.
4. `live.record_media_observation(p_id uuid,p_generation bigint,p_token bytea,
   p_source text,p_egress text,p_room text,p_status text,p_started bigint,
   p_updated bigint,p_ended bigint) RETURNS text` → `observe|terminal|escalated`.
   Requires exact current lease and a committed reservation. START is permitted
   only in its original reserved dispatch generation; ROOM only for reconcile
   with no pinned ID; QUERY only for reconcile with the pinned exact ID. Validate
   frozen room/project identity and the complete closed provider value shape.
   Pin one exact ID atomically or reject a collision; never replace it. Preserve
   late facts even when mutable merchant/binding authority has been revoked.
   Append evidence and project monotonic status in the same transaction, clear
   lease and append a static operation event. Nonterminal observations keep the
   operation UNKNOWN (not SUCCEEDED/finished), resource OBSERVED. Only a terminal
   LKP status with positive ended_at_ns and coherent nonzero timestamps may set
   resource TERMINAL; COMPLETE maps operation SUCCEEDED, FAILED/ABORTED/
   LIMIT_REACHED map FAILED_FINAL. No audience-live/billing claim follows.
5. `live.finish_media_uncertain(p_id uuid,p_generation bigint,p_token bytea,
   p_code text) RETURNS text` → `observe|terminal|escalated`. Closed codes:
   `remote_unknown|not_observed|invalid_observation|credential_unavailable|
   material_invalid|policy_denied`. Under the same lease fence clear lease and
   persist UNKNOWN/event, never invent a provider ID or terminal resource.
   A policy denial before reservation may persist BLOCKED_POLICY; after a
   reservation it must preserve UNKNOWN and sticky cleanup_required.

Only executor gets EXECUTE on these five functions; readiness functions expose
booleans only. Registrar continues to register/revoke but cannot execute work.
Any shared private eligibility helper is mode-sensitive: unreserved dispatch
claim/load/reserve checks all current principal/binding restrictions. Reserved
reconcile claim/load/record checks immutable scope, fenced lease, exact project
and owned target; merchant revocation or disabled binding must not block factual
recovery. Reconcile never permits Start. The helper is not a public bypass.

## Ordering, resource evidence and bounded recovery

Reservation, not claim, is the durable boundary proving that a request may have
escaped. Crash before reservation can safely retry its still-authorized dispatch;
crash afterward must FindByRoom (unknown ID) or Query (pinned ID), never Start.
This refines the older candidate's ambiguous phrase "dispatched": an unreserved
claim has not dispatched. A DB commit with unknown acknowledgement is treated
as uncertain; do NOT call Start when reserve commit success was not observed.

Use all seven exact LKP statuses. STARTING < ACTIVE < ENDING < terminal for the
projection; lower/out-of-order evidence is retained without regression. Nonzero
provider timestamps must be coherent: updated >= started where both present,
ended >= started where both present, updated >= ended where both present.
A terminal enum without a positive ended timestamp remains an observation,
not resource termination. An end timestamp on a nonterminal status is invalid.
Zero started/updated timestamps mean absent, not an extra reason to reject a
positive terminal end. Keep each valid original report. Merge nonzero timestamp
facts only when the candidate projection remains coherent under the same pair
rules; otherwise retain the previous coherent timestamps and unresolved resource
state, return observe and continue exact-target Query. A contradictory partial
merge must not fabricate terminal proof. Evaluate duration on the coherent merged
projection, including start and update facts received in different reports.
Terminal projection never reopens. This synchronous-only increment issues no
new lease once terminal: a report using the closed/stale lease is rejected
without writes and cannot replace identity or terminal facts. Preserving and
escalating contradictory asynchronous/webhook terminal evidence needs a later
authenticated ingress contract; it is not claimed by these five functions.
Empty/ambiguous/malformed discovery is UNKNOWN, not proof that no resource exists.

Revocation after reservation or observed duration >= frozen maximum sets sticky
cleanup_required. Revocation includes loss of original principal/membership/
live:manage, inactive tenant/store, disabled or changed frozen bindings, and
prepared-authorization revocation. Recheck these lifetime conditions on recovery
without rejecting the observation. `start_before` is only a Start admission
deadline: its passage alone is not a lifetime revocation or duration limit.
It does not authorize Stop in this increment. At >=4096 claimed
generations or age >=24h measured from operation.created_at, persist escalation
(`reconcile_exhausted`), clear lease,
leave unresolved resource/UNKNOWN liability intact, return escalated. No silent
resource closure, deletion or replacement. Frozen references stay retained.
These bounds are recovery ceilings, not a promised polling SLA or cost guarantee.
Durably escalated UNKNOWN rows retain their attempt, operation, projection and
evidence even after normal native completed-job retention deletes the job.
Readiness exempts only those rows from mandatory job existence; if the job still
exists its exact linkage is mandatory. Every other unresolved nonterminal row
still requires its exact native job. Historical escalation cannot disable all
future valid planners/workers or silently discharge unresolved liability.

## Go and native River boundary

Package `internal/live`:

```go
type MediaProject struct {
    ProjectID string
    CredentialVersion int64
    Config livekit.Config
    Transport http.RoundTripper
}
func NewMediaClient(ctx context.Context, workerPool, executorPool *pgxpool.Pool,
    keys *livekit.MaterialKeyring, projects []MediaProject, concurrency int,
) (*river.Client[pgx.Tx], error)
```

Constructor validates/copies project entries (1..128, nonzero versions, unique
project/version, exact valid MOCK Config and transport only), builds LKP clients,
validates both authority pools plus same-DB and `live.media_worker_ready()`, and
borrows pools without closing them. Configuration and any container holding
secrets have redacted String/GoString/JSON; no raw error text escapes.
Exact endpoint/project/credential version must match frozen material before use.
No arbitrary dynamic resolver, endpoint fallback, default credential or LIVE flag.

Register existing private `mediaOperationArgs` with a typed worker, only native
`river_media`/`media_mock_v1`, concurrency1..32, fixed 30s lease, 5s DB-call ceiling
and 5s observation snooze. Existing LKP bounds each provider call to10s. Generate
a fresh random 32-byte token per claim. Each SQL mutation commits before the next
network phase. Dispatch: claim → load sealed material → exact project selection
→ LKM Open → reserve/commit → one Start → durable observation/uncertain finish.
Reconcile: claim → metadata-only load → FindByRoom or exact Query → durable record.
No Stop and no implicit HTTP retry. Persisted busy/observe → River JobSnooze;
terminal/escalated → nil only after durable state exists. DB failure/cancellation
returns a sanitized retriable error; it cannot acknowledge an unrecorded outcome.
Provider ACK alone never completes a nonterminal resource's native job.

Add platform Open/ValidateMediaWorkerPool and Open/ValidateMediaExecutorPool,
reuse existing bounded pool and exact-role guards. Existing pool profiles must
also reject new media-role membership/direct function/SET paths. No global
weakening of registrar/ciphertext admission guards to make the worker start.

Post0007 permits native UPDATE lifecycle only, retaining immutable job
id/kind/queue/args/unique key. INSERT still enforces available/attempt0/unfinalized,
DB-clock immediate schedule and exact deferred linkage. Merchant producer keeps
only UPDATE(kind), never state/schedule/delete. Dedicated River lifecycle role
cannot call business execution functions or read ciphertext. Both planning and
worker readiness check exact current owners, schemas, signatures, guards,
FORCE RLS and valid nonterminal job linkage without demanding attempt0/available
for an already-running job. Wrong/missing guard or mixed-role startup fails closed.

## Independent acceptance and stop lines

|Gate|Required causal evidence|
|---|---|
|LME01|Actual executor/River roles, cross-role/direct SQL denials, mixed/SET/direct grants, wrong DB, secret redaction, old ordinary/payment/Meta positive controls|
|LME02|Real PG18 + local TLS + native River executes one Start from LMP, exact material/config/target and durable nonterminal observation; no transaction held over network|
|LME03|Real observed lock wait followed by grant/binding/revoke/deadline change denies reservation and Start; repeated reservation and stale/expired token/generation denied with no partial changes|
|LME04|Accepted Start with lost reply, then revoke and real worker process death/restart: Start count stays1, exact room discovered/pinned once, later exact-ID Query, no stream secret for reconcile|
|LME05|Concurrent workers, reserve commit uncertainty/crash, before-reservation failure, observation/finish rollback: one wire reservation, no false terminal or orphan facts; late external facts survive revoked merchant|
|LME06|Empty/ambiguous/wrong-room/ID collision, out-of-order and duplicate reports, all LKP status/time boundaries; monotonic projection and positive terminal proof|
|LME07|Populated0035 upgrade, queue native fetch/maintenance/rescue isolation, repeated Apply, tampered ACL/guard/readiness fail-closed; valid producer/job linkage survives native state changes|
|LME08|Generation/age exhaustion persists unresolved escalation; credentials/material failure cannot choose fallback or claim resource ended; shutdown releases task-owned pools/processes|

Independent test writer consumes this contract before source is provided; frozen
source and tests are then integrated and root runs real PG18/race/vet. Preserve
red evidence; max two targeted repairs per blocker, no weakening gates. Add runner
`--live-media-execution` with a nonempty LME selector; full T08/G06/LIVE remain open.
Explicitly verify old LMP initial-state assertions at their historical boundary
or refine only UPDATE expectations with dedicated-role negatives, never delete
the initial INSERT or merchant mutation gates to admit native worker lifecycle.
