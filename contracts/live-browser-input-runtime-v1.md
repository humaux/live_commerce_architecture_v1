# Browser input custody runtime v1

Status: **BIC_KERNEL_IMPLEMENTED_FOCUSED_PASS — clean full acceptance pending**. Source base
`2b52e7d`. Independent bounded review on 2026-09-27 found no remaining confirmed
P0/P1 after queue/profile, replay and lifetime clarifications. This refines
[BRI](live-browser-input-v1.md), not LIVE admission. Root owns migration numbers
and merge; BIC01–05 clean full acceptance is still required. See the
[implementation receipt](../docs/implementation/2026-09-27-browser-input-runtime.md).
BRW token delivery and new-queue consumption remain unimplemented.

## Scope and smallest implementation

The existing attempt, `integration.operations` row and original `river_media`
job remain the only durable execution ledger. Reuse MLC for the exact initiating
login. Do not add another Stop job, queue backend, login/token cache or controller
service. A private child stores input liability, not a second business operation.

Use a **fresh** MOCK authorization/attempt with execution profile
`LOCAL_SFU_MOCK_EGRESS`, on `media_input_mock_v1`. The old `PROVIDER_MOCK` /
`media_mock_v1` route and historical rows keep their meaning. No row migration
or new LIVE registration is authorized. This new local profile may prove real
local browser/SFU input, but its Egress remains a double. It is not Cloud proof.
The distinct queue is necessary because an old worker returns from terminal
Egress without knowing input liability. Both queues use the existing River
schema/kind, and every attempt still has exactly one original job.

Two implementation units, in order:

1. **BIC custody kernel:** fresh profile registration, atomic planning, nonsecret
   grant reservation, same-operation fencing and combined completion. No token
   HTTP response, signer wiring, new queue consumer or real provider calls.
   Actual-role PG tests exercise the kernel directly. An unconsumed new queue
   is safe only while no application path can deliver a publisher token.
2. **BRW product runtime:** consumes that queue, provider input observation,
   final Start gate, bounded cleanup, post-commit token delivery and real
   authenticated browser/SFU acceptance. Its wire-step ABI and HTTP route are
   reviewed separately before implementation; BIC is not permission to expose
   tokens. Studio UI and qualified Cloud revocation remain separate gates.

## BIC frozen storage and entry points

Forward migration `0040_live_media_input_custody.sql` and post-River migration
`0009_live_media_input_custody.sql`. No changes to historical migration files.
Post-River0009 atomically replaces `guard_media_job_family`, deferred
`check_media_job_link`, `media_native_job`, attempt-operation linkage and
readiness. Each accepts only the exact queue/profile pairing; the old queue
remains valid. The guard must permit insertion of the new queue before planner
InsertTx, then the deferred link proves the completed same-transaction attempt.
Update cross-schema `reject_legacy_media_job` coverage for the new queue as well.
Merely adding a queue string in Go cannot pass the current 22023 guards.

`live.prepared_media_input_profiles` is a private immutable one-to-one child of
prepared authorization, with scoped FK and profile fixed to
`LOCAL_SFU_MOCK_EGRESS`. `live.register_media_input_profile(uuid)` is executable
only by `commerce_media_registrar`, owned by `commerce_media_writer`, fixed
`search_path=pg_catalog`. It accepts only an unused, unrevoked, unexpired MOCK
authorization and must serialize against planning using the same authorization
lock. Repeating the same registration is harmless; attaching after an attempt
exists is rejected. No environment/project/credential is supplied by a merchant.
Under that same authorization lock, the legacy `plan_media_start` MUST reject
an authorization that has this profile child, including the rehearsal route;
it cannot consume it into a legacy attempt. Conversely the input planner rejects
an authorization without the child. Test both sequential directions and the
registration/planning interleave, not only the input planner's positive path.

`live.media_input_custody` is one-to-one with the exact scoped attempt and unique
original operation. It references (does not duplicate) MLC. Freeze room, project,
credential version and endpoint identity from the authorization; identity is a
server UUID rendered `lcp_` + 32 lowercase hex digits, independent of worker
generation. Store original session version and database creation time. Freeze
`lifetime_deadline = media_attempts.created_at + make_interval(secs =>
prepared_media_authorizations.max_duration_seconds)` as a finite timestamptz;
waiting for input consumes this lifetime rather than extending it. This input
lifetime is distinct from Egress's provider-observed duration limit. FORCE
RLS on both children; direct access only for the private media writer, no broad
runtime/executor/worker/registrar SELECT or mutation grants.

Mutable custody fields are: `grant_iat`, `grant_exp` (both null before reservation),
`admission_closed_at`, bounded reason code, provider observation metadata,
cleanup/escalation state and timestamps. Identity/scope/profile cannot change;
grant timestamps can change only once from null to finite positive seconds.
No raw bearer, publisher JWT, API secret, stream key or token hash is stored.
Every potentially delivered grant creates liability at reservation, even if the
HTTP response or COMMIT acknowledgement is lost. Never infer it was not delivered.

Frozen SQL public entry points (all SECURITY DEFINER, VOLATILE, pg_catalog path):

|Entry point|Caller and result|
|---|---|
|`plan_media_input_start(bytea,uuid,uuid,uuid,bigint,text,uuid,bigint)`|runtime; same arguments/result as `plan_media_start`, but exact new profile/queue required|
|`reserve_media_input(bytea,uuid,uuid,uuid,bigint)`|runtime; hash/store/session/attempt/expected session version; nonsecret JSON grant spec|
|`claim_media_input_operation(uuid,bigint,integer,bytea)`|executor; operation/job/lease seconds/32-byte key; same disposition/generation/mode columns as legacy claim|
|`close_media_input_admission(uuid,bigint,bytea,text)`|executor; fenced original lease; returns `observe`, `held` or `terminal`|
|`load_media_input_custody(uuid,bigint,bytea)`|executor; fenced nonsecret custody material only|
|`media_input_plan_ready()`|runtime/executor/worker; boolean; validates new-profile schema/linkage/role gates in addition to legacy readiness|

Writers may factor shared private helpers instead of copying whole SQL bodies;
helpers are not granted to callers. Retain every old entry point/signature and
every existing legacy error/authorization assertion. No generic profile router.
Input branches are explicitly bounded to the one new profile. Legacy claim must
reject the new profile; new claim must reject legacy jobs and incorrect queues.

`MediaPlanner.PlanInputStart` accepts the existing `MediaStartInput`, returns
`MediaStartResult`, and inserts the exact new-queue job within the same caller
transaction as the attempt, operation, MLC and input custody. A shared private
planner helper is allowed; `PlanStart` and rehearsal/start must never silently
select or consume the new profile. `MediaPlanner.ReserveInput` accepts session/attempt/
expected-version plus the normal command idempotency key and returns the
nonsecret grant spec. It does NOT call `MintPublisher`. After `command.Run`,
both fresh and replay paths MUST re-run idempotent `reserve_media_input` and
compare its exact returned spec with the receipt, not merely run the MLC login
assertion. Reuse this entry point rather than add an assertion API. It verifies
exact attempt/profile/room/session/version,
current initiating login and Start eligibility, existing unexpired grant and
still-open admission after all waits. Stop/close/dispatch/expiry/revision change
denies return, even with the original successful command key, without deleting
the existing liability. Error returns expose no partially populated grant.
Use command namespaces `live.media.input.start` and `live.media.input.reserve`;
neither may collide with existing `live.media.start` or Stop receipts.

Grant JSON contains exactly `attempt_id`, `room_name`, `publisher_identity`,
`project_id`, `endpoint_identity`, `credential_version`, `issued_at`, `expires_at`.
Go `MediaInputGrant` exposes these nonsecret fields with matching JSON names;
input struct is `MediaInputReserveInput{SessionID, AttemptID,
ExpectedSessionVersion}`. Do not expose login IDs or proof internals.
`load_media_input_custody` returns exactly: `attempt_id`, `operation_id`,
`session_id`, `execution_profile`, `state`, `room_name`, `publisher_identity`,
`project_id`, `endpoint_identity`, `credential_version`, `session_version`,
`issued_at`, `expires_at`, `start_before`, `lifetime_deadline`,
`admission_closed`, `close_reason`, `egress_state`, `wire_reserved`, `held`.
Times are integer Unix seconds; unreserved issued_at/expires_at are JSON null.
Unset close_reason is the empty string; booleans are not stringified.

The frozen endpoint identity remains the prepared authorization's strict Cloud
shaped identifier. BIC does not return a browser connect URL or relax it. BRW
must explicitly bind a test-only local browser/SFU endpoint mapping and prove
it cannot activate in production. This is a future runtime gate, not implicit
permission for the browser to dial the grant's endpoint_identity.

## Admission and lock ordering

Planning creates the input child **before the original transaction commits**.
There is no interval where a worker can dispatch Egress unaware of input gating.
Merchant planning/reservation entries validate READ COMMITTED, IDs, exact scope,
current authz revision, store:read/live:manage, current and frozen initiating-login
expiry, authorization, bindings, session/program and original job. Executor
claim/load/close instead use the original immutable custody and lease fence;
revoked login/permissions/bindings close admission but MUST NOT remove cleanup
authority. No merchant GUC or renewed merchant login is needed for cleanup.
Reservation is permitted only while
Start is still possible, not after Stop, close, dispatch, escalation or expiry.

Follow the existing binding -> tenant/store -> session/program -> authorization
-> attempt -> operation -> execution state order, then the input child. Login
locking/checks retain MLC's ordering; no new inversion with logout/registration.
Never retain a database transaction over provider I/O. Recheck clock and current
authority after blocking writes, including the idempotency receipt path.

First reservation freezes `issued_at=floor(database_clock_seconds)` and
`expires_at=floor(min(now+60s, start deadline, lifetime deadline, current login
expiry, frozen login expiry))`. Reject expiry <= issued_at or already elapsed.
Repeat reservation with either the original or another valid command key returns
the same identity/timestamps, never extends them or creates another job. A
different login of the same principal is not an accepted replay. Changing the
caller-provided expected version is a conflict, not a new grant.

## Combined lifecycle: invariant, not a later repair

Input states are `UNISSUED`, `RESERVED`, `CLOSING`, `UNKNOWN`, `CLOSED`.
Provider-ready evidence is separate from liability; it cannot imply CLOSED.
Admission closure is sticky. Close reasons include merchant Stop, exact login
loss, permission/revision loss, authorization/binding loss, expiry and Egress
terminal/failure. The original operation stays claimable UNKNOWN while reserved
input is not CLOSED, including when Egress never reached wire reservation or is
already TERMINAL. Never set a terminal operation and subsequently reopen it.
Accepted close reasons are exactly `merchant_stop`, `login_lost`,
`permission_lost`, `authorization_lost`, `binding_lost`, `expired`,
`egress_terminal`, `reconcile_exhausted`, `runtime_unavailable`; no free text.
Close returns `terminal` only for combined completion, `held` if automation is
exhausted/unsupported, otherwise `observe`. In this BIC-only implementation an
issued local grant closes admission into UNKNOWN/held; an unissued grant may
close without provider proof. Close releases the operation lease. Load always
requires a current valid lease; a closed/held reply cannot be reused as one.

Completion requires both:

- Egress coherently TERMINAL, or Start is permanently disabled and wire was never
  reserved (uncertain Start reservation is not proof it never reached provider).
- Input is CLOSED. An UNISSUED child can close under the original lock with no
  external obligation; reserved input cannot close from TTL, empty room, Remove
  acknowledgement or self-hosted room deletion.

Current local profile has **no qualified revocation proof**, so reserved input
cannot transition to CLOSED in BIC. It ends in `UNKNOWN` + visible held reason
until later controlled recovery. There is no public force-close/reset function.
Do not manufacture Cloud proof to make a local positive test pass.

Apply joint-completion logic to ALL existing terminal paths: Stop-before-wire,
claim policy denial, observation projection/record, cleanup-query record,
finish-uncertain, and generation/age escalation. Legacy behavior remains exact.
Final outcomes use existing SUCCEEDED/FAILED_FINAL/CANCELLED/BLOCKED_POLICY;
do not add an independent completion ledger. A child with open liability must
never have a terminal operation or a finalized/missing original native job.

New claim always uses existing operation generation/lease fields and the same
32-byte fencing rules (lease 1..30s, never carried across expired/old generation).
Use existing `reconcile` mode/UNKNOWN state for input-only custody, including
pre-Egress and post-Egress. Before BRW exists, claim cannot grant dispatch and
existing `reserve_media_start` must reject this profile. Every cleanup/load/close
entry verifies the exact native job and profile as well as the live fence.

Before any grant is reserved, an eligible UNISSUED input claim returns
`await_admission` with the existing generation (initially 0), empty mode and no
lease. It does not consume a generation or dispatch Egress. A future consumer
must snooze that same native job, not acknowledge completion. After reservation,
a reconcile claim alone does not close admission: a still-authorized prewire
replay returns the identical, unexpired grant until a real closure condition.

At 4096 generations or 24h since operation creation, stop issuing active leases;
close admission and return `held` without finalizing the operation or deleting
the job. No new provider retry is authorized. The future worker MUST snooze held
jobs (60s; no provider calls, no further generation increments), not return nil.
This is bounded automation with explicit unresolved liability, not a hard stop
guarantee during infrastructure/provider outage.
Evaluate proven combined completion before budget exhaustion so an unissued,
permanently stopped attempt is not needlessly left held.

Readiness checks include every input child, even a corrupt terminal operation:
exact native queue/kind/args/job_id, nonfinalized native job while liability open,
private tables/ACL/definers, enabled guards, exact profile linkage. Extend pool
allowlists only for the concrete new function signatures. New functions are not
an old-worker compatibility claim; BIC does not enable a consumer/token route.
For unresolved input, active native states are exactly available, scheduled,
retryable or running, with finalized_at NULL. In addition to readiness detection,
the native lifecycle guard rejects completion/cancellation/discard/delete of an
open-liability job. Owned final cleanup cannot race a new reservation under the
original operation lock. Disposable test teardown may delete parent fixtures as
owner; no runtime force-delete capability is added.

New executor grants deliberately fail the old binary's exact function allowlist.
Deployment therefore requires a tested compatible API/worker build and coordinated
restart before new profile use. BIC is local only, with no deployment authorized;
do not claim zero-downtime compatibility or relax the old allowlist. The new build
must still run the legacy queue unchanged. Missing0040/post0009 fails closed.

## Acceptance and implementation stop lines

|Gate|Required evidence|
|---|---|
|BIC01|Actual runtime/registrar/executor/worker-role PG18 tests: atomic profile/planner linkage, all rollback artifacts zero; cross-tenant/store/login, wrong profile/queue, missing MLC and direct table ACL denial|
|BIC02|Same-grant replay and COMMIT acknowledgement loss; no timestamp/identity extension; fresh post-wait expiry/revocation/revision checks and exact session version|
|BIC03|Original operation/job retained across Stop before any Egress wire, Egress-terminal projection, policy denial/uncertain finish and age/generation exhaustion; every terminal path; never reopen terminal|
|BIC04|Stale/expired/wrong-key fences rejected; issued local input never falsely CLOSED; unissued close can reach joint terminal; readiness detects finalized/missing job and wrong grants|
|BIC05|Legacy MLC/plan/execution/Stop, runtime, Studio/backend/BFF regressions and full PG/race/vet unchanged; no consumer/signer/HTTP activation|
|BRW / BRI04-07|NOT_RUN until separately implemented: provider camera+mic gate, bounded Remove/Delete/query, signed-cookie BFF -> Go/PG -> real local SFU -> separate receiver decoded frames/nonzero audio energy, crash recovery, qualified Cloud and approved Studio UI|

No token can reach a browser before BRW proves the continuous original-job path
and explicit post-commit authenticated response. Local tests use disposable
servers, test-only injected transports and mock Egress; never relax production
endpoint validation or use customer credentials. Existing tests are not weakened
to admit the new profile. Source and tests use separate worktrees; independent
review precedes merge. Record failure logs and at most two targeted repairs per
blocker. Upgrade signal: only qualified LIVE provider intake plus BRI06 may add
strict issued-input CLOSED; no hidden boolean in a merchant request enables it.
