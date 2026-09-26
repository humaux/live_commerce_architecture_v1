# T08 media authority and resource lifetime decision

Status: **DESIGN / implementation contract pending independent review**.
This decision resolves the direction of the preliminary LBI proposal; it does
not register a route, provision credentials or authorize a real broadcast.
Source inventory: main `40335e2`, Humaux `acdaf428-4d35-46cc-b68b-7ab0afd92cf3`.

## Decision and evidence

Separate **permission to start a new broadcast** from **authority to observe and
reclaim an already-created platform resource**. Both require persisted proof;
neither is inferred from an enabled merchant binding or a well-formed room name.

|Boundary|Decision|Existing implementation to reuse|
|---|---|---|
|Media project|Platform-operated LiveKit project; deployment injects a versioned secret reference, never a merchant-supplied endpoint/key|Validated `livekit.New`; immutable attempt must freeze project identity and credential reference|
|Destination eligibility|Trusted official asset authorization/eligibility and version/epoch/expiry are required before Start|Meta owner/routing checks inform the pattern, but `0028_meta_inbox.sql` inbound route/proof hash is **not** an outbound live grant|
|Stream secrets|Separate encrypted media records, key purpose and AAD bound to tenant/store/destination/attempt/version|`accounts/crypto.go` AES-GCM/copy/redaction pattern; not PAYUNi HashKey/HashIV types, keys, columns or permissions|
|New Start|Current merchant permission, destination entitlement, budget and frozen versions checked before planning and immediately before dispatch|Existing scoped transaction, command receipt, audit and River same-transaction operation intent|
|Query/Stop existing resource|Attempt-scoped cleanup authority remains after merchant start entitlement is disabled or changes|Existing operation generation/lease; new narrow persisted ownership check, not a second scheduler|
|Unknown Start|One Start maximum; use `FindByRoom` on the attempt's server-generated persisted room|LKP07 candidate observation must be validated and adopted exactly once by the durable controller|
|Status|Operation acknowledgement, media transport status and audience-live evidence remain separate|Existing architecture section 9; no green LIVE from a queued intent, preview or Egress acknowledgement|

This choice is narrower than a general credential vault or generic policy engine.
The current `accounts` implementation is PAYUNi-specific. Copy its established
cryptographic boundary where necessary; extract a shared primitive only if the
actual media implementation proves useful common code, with both regressions run.

## Why the current dispatcher cannot simply be wired up

`integration.claim_operation` in `0008_external_operations.sql` treats a changed
binding as STALE_BINDING before dispatch, or UNKNOWN/blocked_binding if dispatch
may have occurred. `core.Dispatcher.finalDispatchGate` independently checks that
the current binding is enabled and has the frozen version. These are appropriate
defaults for ordinary operations but also prevent media reconciliation/cleanup
after revocation. A route callback alone cannot override the earlier SQL claim.

The reviewed durable-controller contract must therefore extend **both** claim and
final-dispatch checks with an exact media-ownership predicate. Ordinary providers
retain their existing behavior. The exception is eligible only when an immutable
attempt joins the exact tenant/store/session/project/room, frozen binding and
operation IDs. Allowed modes are an owned query/stop operation, or **query-only
reconciliation of that same already-dispatched Start operation** after its result
became uncertain. The latter remains a Start action in the ledger but must have
`lease_mode=reconcile`; its callback can only FindByRoom/Query, never Start. A
READY Start or any `lease_mode=dispatch` Start cannot use the cleanup exception.
This distinction must hold at SQL claim, final gate and route callback, including
an expired DISPATCHING lease recovered after a crash. Start authorization is still
checked separately. Generic `core.Plan` calls cannot mint the trusted ownership
record, substitute a request payload or acquire cleanup authority by choosing an
action name. Runtime and worker SQL privileges must enforce this distinction.

This is not permission to add an `ignore_binding`, `allow_revoked` or caller-fed
boolean, re-enable a merchant binding, use a replacement account, or release a
generation fence. Freeze the SQL privilege/call graph and lock order before code.
Use the existing lease/token/generation and River transaction rather than a second
media queue, scheduler or lease. No provider call holds an open business lock.

Cleanup **initiation** is a separate authorization boundary. A merchant's Stop
request still requires current authenticated membership, scope and explicit stop
permission; historical ownership is not a bearer capability for a revoked user.
Automatic expiry/revocation cleanup is initiated only by a scoped system-owned
controller (or an explicitly authorized operator), with a durable reason and audit
identity. The worker may execute that exact owned cleanup after merchant access
ends, but may not accept an arbitrary user target. Current core operations/read
queries are MERCHANT-only; the next contract must define the narrow authenticated
system-initiation and audit representation, not impersonate the former merchant
or broadly relax actor filters. That extension is not implemented here.

## Credentials, adoption and terminal evidence

- Deployment supplies platform project credentials only through the secret
  boundary. Attempts persist a version/reference, not plaintext. Rotation must
  not silently change the project or make an unresolved attempt target a new one.
- The credential resolver checks exact owned attempt/operation/generation/lease
  and expiry before returning a short-lived in-process client input. No getter
  returns secrets to browsers, logs, generic operation request JSON or audit rows.
- Destination stream URLs/keys must originate from an authorized official path.
  Manual Instagram Live Producer input, if supported, needs its own reviewed
  authenticated intake and platform-confirmation boundary; syntax alone is not
  ownership. Never accept social account passwords or private API paths.
- `FindByRoom` returns a candidate, not authorization. Under the existing
  generation fence, adopt only one exact valid observation from the attempt's
  project and globally unique server-generated room. Conflicting IDs or rows
  remain UNKNOWN; do not choose the first result. No merchant-selected Egress ID.
- An empty provider list does not prove Start never happened. Stop acknowledgement
  does not prove termination. Provider terminal evidence and durable history are
  required before marking resource cleanup complete or closing usage accounting.
- Real provider credential revocation can make even cleanup impossible. Preserve
  UNKNOWN, an operator escalation and unresolved usage liability; do not report
  a successful stop. Retention of a credential reference is not a claim that the
  external provider will continue honoring it.

## Acceptance gates before operational route registration

All gates below are **NOT_RUN** for the durable controller; LKP07 tests alone do
not close them. Use isolated PostgreSQL 18 plus local TLS provider fixtures first.

|Gate|Required falsifiable evidence|
|---|---|
|MLA01|Cross-tenant/store/session/project, forged generic Plan and direct role SQL cannot create or use trusted attempts; membership/permission-revoked merchant HTTP Stop is denied with no job/call, while scoped system cleanup of the exact owned resource remains possible and auditable|
|MLA02|Revoke/rotate while waiting for claim or final gate prevents a new Start, while exact previously-owned Query/Stop still reaches the original resource only|
|MLA03|Provider accepts Start then loses response, binding is then revoked, process crashes/restarts; exact Start operation enters reconcile-only mode, does one room lookup/adoption, Start count stays 1. Repeat for expired DISPATCHING lease; concurrent/stale worker, READY Start and generic Plan cannot reuse the exception or issue a second Start|
|MLA04|Stop after revocation/rotation is deduplicated and fenced; target substitution, stale completion and arbitrary merchant Egress ID are rejected|
|MLA05|AAD/key/version swapping and replay fail; no plaintext in operation/audit/API/logs; workers lack direct ciphertext read; provider-revoked credentials remain unresolved|
|MLA06|Empty/ambiguous query, delayed or duplicate/out-of-order webhook and Stop ACK never fabricate RUNNING/LIVE/ENDED; terminal history controls cleanup/accounting|
|MLA07|Transaction rollback leaves no orphan attempt/operation/job; restart resumes query/reclamation; existing core and payment/Meta binding regressions remain unchanged|
|MLA08|Documented real authorized Cloud/media test plus browser lifecycle verifies media/output/cleanup, cost boundaries and audience-state distinction; only then evaluate G06|

## Rejected paths and upgrade signals

- **Rejected: executable LOCAL-only intents now, provider adapter later.** A later
  route could activate unverified old work. Continue with protocol and trusted
  durable records, not a dormant queue with unclear future authority.
- **Rejected: binding enabled means asset ownership or live eligibility.** The
  current merchant registration validates configuration, not provider proof.
- **Rejected: reuse payment credentials or bypass all revoked-binding checks.**
  Wrong secret domain and unbounded privilege exception; fix the exact lifecycle
  predicate instead of defeating the safety gate.
- **Rejected: automatic second Start after empty lookup, replacement project, or
  private Instagram API.** None establishes what happened to the original resource.

Known remaining work: trusted destination proof/intake, concrete media credential
resolver, immutable attempt migration and narrow dispatcher SQL extension, durable
status/webhooks/cleanup and browser controls. No new production permission follows
from this document. Additional media providers or merchant-owned LiveKit projects
require a new custody/ownership decision, not a permissive string configuration.
