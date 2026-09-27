# Media initiating-login custody v1

Status: **CANDIDATE — independent review required**. Base `80792e4`.
This is the first persistence prerequisite of [BRI](live-browser-input-v1.md),
not browser admission, input cleanup, LIVE qualification or G06 acceptance.
The existing MOCK planner, attempt, operation, job and response DTO are reused.

## Why this increment exists

The current attempt freezes a principal but not the login that authorized Start.
The worker cannot distinguish logout of that login from logout of another login
belonging to the same person. Current worker eligibility also omits `store:read`
and the authorization revision. Browser tokens must not be issued on this basis.

Persist exact login custody before adding token issuance. No second operation,
queue, token cache or provider SDK is needed. The private child below avoids
exposing login identifiers through the existing runtime-wide attempt SELECT.

## Frozen interface proposal

Forward migration `0039_live_media_login_custody.sql`; integrator owns final merge.

`live.media_login_custody` is an immutable one-to-one child of an attempt:

|Column|Rule|
|---|---|
|`attempt_id uuid`|Primary key; nonzero; existing exact attempt|
|`tenant_id, store_id uuid`|Composite FK to `live.media_attempts(tenant_id,store_id,id)`; delete cascades only when the owned attempt is deleted|
|`login_session_id uuid`|Exact merchant login ID; FK to `identity.sessions(id)`, no deletion cascade|
|`authz_revision bigint`|Positive revision authorized at Start|
|`login_expires_at timestamptz`|Finite expiry snapshot; cannot be extended by a later login-row edit|
|`created_at timestamptz`|Database clock; expiry strictly later|

No bearer, hash, join token, key, endpoint or second resource state is stored.
Enable and FORCE RLS. Only `commerce_media_writer` has SELECT/INSERT policies
and grants; no UPDATE/DELETE grant. Runtime, executor, worker and registrar have
no direct table access. Additional identity SELECT columns are limited to login
ID and membership revision. No new schema owner or runtime configuration.

Private `identity.lock_media_login(p_hash bytea,p_principal uuid)` returns
`TABLE(login_session_id uuid,login_expires_at timestamptz)`. SECURITY DEFINER,
VOLATILE, fixed pg_catalog path; owner `commerce_identity_writer`, EXECUTE only
to `commerce_media_writer`. Reuse the identity writer's existing session UPDATE
privilege for `FOR SHARE`; do not give the media role a new session mutation
grant. Validate hash/ID/READ COMMITTED; select exact merchant login and principal,
lock the session, then check revocation and `clock_timestamp()` expiry after
the wait. Return no hash/secret. No match raises MP401, malformed input MP400.

`live.plan_media_start(bytea,uuid,uuid,uuid,bigint,text,uuid,bigint)` keeps its
signature, result and MOCK restrictions. In its original transaction, obtain
the exact active merchant login by the passed bearer hash and resolved principal;
persist custody with the new attempt, operation and River job. Check current
clock, login, revision and `store:read`/`live:manage` again after all blocking
writes, so a failed check rolls back every artifact. Do not backfill old attempts
by guessing the latest login or accepting a newly supplied login.

`live.assert_media_start_login(p_hash bytea,p_store uuid,p_attempt uuid)` returns
void; SECURITY DEFINER, VOLATILE, fixed `search_path=pg_catalog`, owned by
`commerce_media_writer`, EXECUTE only to `commerce_runtime`. Validate exact
nonzero IDs, 32-byte hash, READ COMMITTED, current resolved scope/GUC/revision,
the attempt's principal/store/tenant and persisted login binding. Check both
current and frozen expiry with `clock_timestamp()` after any waits. Different
active login for the same principal and missing historical custody fail closed.
Use existing `MP400/401/403/404/409` error classes, no identifiers in errors.

`MediaPlanner.PlanStart` calls this guard **after command.Run**, including replay,
and after its final existing authorize check. Replay never reruns the writer,
inserts another custody row, extends expiry, or rebinds to another login.

Private `live.media_login_eligible(p_operation uuid)` returns boolean and has no
runtime/executor/public grant. Join the exact operation/attempt/custody/login;
require active merchant login, matching original principal, current expiry and
frozen expiry, active membership and matching frozen revision, `store:read` and
`live:manage`, with current active tenant/store/principal. Fail false when absent.
Existing `media_dispatch_eligible` additionally requires this helper;
`media_lifetime_revoked` additionally returns true when it is false. Preserve
every existing binding, destination, start-deadline and duration rule.

This is deliberately conservative: any authorization revision change closes the
old attempt, even if permissions were later restored. This only detects transient
revoke-and-regrant if the authority mutation increments the revision. The current
schema has no automatic grant-mutation revision trigger; do not claim detection
of owner SQL delete/reinsert that leaves the revision unchanged. A production
permission-management path must atomically advance revision before LIVE admission.
A different login's
logout must not affect this binding. Another authorized merchant may still Stop;
Stop authority does not transfer publishing custody.

## Concurrency and legacy boundary

Do not add an inverted media/login lock order. Existing ordered binding locks,
tenant/store, live session/program, authorization/attempt and operation/events
remain. Planning takes the exact login FOR SHARE after the ordered binding,
tenant/store/session/program/authorization locks, before inserting the new
attempt. The post-command assertion also takes that login lock; fresh planning
already owns it and replay has no media mutation. Logout only locks the login
then writes its session event, not media rows. Worker eligibility remains a
fresh read, without a new login lock or identity-mutation permission. No provider
request holds any database lock.
New SQL is VOLATILE so final nested reads get a fresh READ COMMITTED snapshot.

A logout committed before the final eligibility snapshot is rejected. A revoke
after the final dispatch gate cannot atomically cancel provider I/O: once wire
reservation exists, the original worker must accept exact provider observations
and clean up, not throw away historical resource ownership. Logout is not a
terminal-proof event. Missing/expired/revoked custody before reservation prevents
Start; after reservation it requires existing bounded reconciliation/Stop.

Pre-upgrade attempts remain unbound. They cannot acquire new Start permission;
already-reserved attempts retain exact cleanup authority, budgets and target.
No attempt, environment, profile, job or key is rewritten or relabeled LIVE.
This increment does not extend the current Egress-terminal short circuit for
browser input: input issuance remains absent until that next unit is implemented.

## Independent acceptance gates

All **NOT_RUN** until executable evidence is recorded; use real isolated PG18,
actual runtime/worker/executor roles, unchanged receipt/operation/job identities.

|Gate|Required evidence|
|---|---|
|MLC01 atomic custody|Fresh Start persists exact login/revision/expiry with attempt+operation+job; rollback leaves none; concurrent same-key and lost-commit-response replay return the same artifacts and expiry; conflicting body rejects|
|MLC02 replay authority|Same-principal second login cannot replay/take over; revoked/expired initiating login, revision change, lost store:read/live:manage reject; different-login logout has no effect; other authorized Stop remains available|
|MLC03 final gates|Observed PG blocking wait followed by logout/expiry/revision change rolls back planning or denies final Start reservation; no provider Start after causal pre-gate revocation; revision mismatch stays denied after permission restoration|
|MLC04 lifetime|After Start reservation, logout/expiry/access loss still accepts exact Egress facts; original persisted operation/job resumes and reaches bounded Stop/reconciliation with no new Start or job; ambiguous outcomes stay UNKNOWN|
|MLC05 ACL/upgrade|Private table/columns and helper ACLs; fixed definer path/owner; malformed/cross-scope calls; historical unbound attempt denies Start but keeps cleanup; browser HTTP DTO contains no login binding/secret|
|MLC06 regression|Media plan/execution/Stop/runtime and Studio regressions; independent fixed-source review and Root rerun; no weakened old assertions, no customer/provider production calls|

Further required units remain explicit: publisher identity and grant reservation,
original-job input observation/cleanup through Egress terminal, private HTTP/BFF
token delivery, actual browser/SFU gate, Cloud revocation qualification, approved
Studio UI and operational outage drill. Passing MLC alone does not pass BRI.
