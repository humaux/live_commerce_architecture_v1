# T08 durable media controller — implementation contract candidate

Status: **PARTIALLY FROZEN; full controller NOT FROZEN**. The authorization
registry increment is specified separately in
[LMA01–05](live-media-authorization-v1.md); durable MOCK start-intent roles,
attempt identity and native queue linkage are implemented separately in
[LMP01–07](live-media-plan-v1.md). Execution leases, observation/material
resolution and full lifecycle below are still a candidate, not enabled by LMP.
The bounded MOCK Stop recovery below
has independent design review; it is not yet implemented. This supersedes the implementation
direction of the old non-executable `live-broadcast-v1.md`; it does not activate
that draft. Media credentials/official destination eligibility are not supplied
by this contract. Existing LKP and LKM components remain separately gated.

## Fixed integration choices

One PostgreSQL truth, `integration.operations` ledger and core generation/lease
semantics; River handles the durable jobs. Use fixed job kind
`live_media_operation_v1` and typed worker in `internal/live`. Do not put a lease
token in generic DispatchRequest or let `core.Dispatcher` accept MEDIA actors.
Separate media River schema/role ACLs may isolate job administration; they are
not a second queue engine or a new business ledger. No live provider call in a
database transaction. All SQL below uses fixed search_path and explicit grants.

## Storage and authority to freeze before implementation

1. **Prepared media authorization**: immutable ID, tenant/store/session/attempt
   IDs, environment (MOCK or LIVE), platform project/credential version and
   endpoint identity; media binding/version; ordered FB/IG destination
   bindings/versions/assets; evidence type/hash/issuer, start deadline, explicit
   duration/budget cap; LKM scope/material version/key ID/nonce/ciphertext.
   Store only encrypted destination URLs. Material AAD binds the exact attempt.
   Revocation is append/audited metadata; no mutation of historical identity,
   destination list or encrypted content. MOCK evidence cannot become LIVE.
2. **Media attempt**: one per session for this release, unique server-derived room,
   prepared authorization reference, immutable original principal, start operation
   ID, optional once-pinned stop operation ID, optional once-pinned Egress ID;
   stop-request reason/initiator, transport observation and unresolved/terminal
   resource state. One Start maximum, even after a different key or crash.
   A project/Egress ID cannot be assigned to two attempts or tenants.
3. **Observation history**: exact attempt/project/room/Egress identity, operation
   ID, generation, observation source/status/timestamps and canonical report hash.
   Immutable/deduplicated. Provider error text and URLs are excluded. Out-of-order
   observations cannot reopen a terminal resource or replace a pinned ID.
4. **Operation family**: `MEDIA_ATTEMPT`, fixed `livekit.egress.start` or
   `livekit.egress.stop`, purpose service, exact immutable attempt FK. Mutually
   exclusive with MERCHANT and BUYER_PAYMENT_QUERY fields. Request contains safe
   identifiers only; no stream key, client-supplied target or credential material.
   Generic runtime INSERT policy must explicitly require actor_kind=MERCHANT.

The concrete migration must specify all columns/checks/composite foreign keys,
RLS, queue-ID linkage and privileges; this candidate is not permission to invent
them independently in source and tests. Current generic job and receipt schemas
must retain legacy semantics and existing migrations remain unchanged.

## Roles and fixed SQL entry points

- `commerce_media_registrar`: trusted, separately provisioned server-side
  identity. Only this path may persist a prepared authorization, following an
  official authenticated asset/eligibility check (or a separately reviewed manual
  Instagram Live Producer intake). An evidence hash is not proof by itself.
  No merchant role may make a binding authorized by inserting this record.
- `commerce_media_worker`: exact narrow worker authority, distinct from ordinary
  commerce_worker/runtime/Meta/payment roles; no direct secret-table writes or
  ciphertext SELECT. It can claim, resolve and report only MEDIA operations.
- Private NOLOGIN writer/definer owns the fixed entry points. No public dynamic
  function/action/family names or caller-fed bypass switches. Startup verifies
  real role memberships/privileges and same physical database, following existing
  platform guards, not DSN equality.
- Generic public core Claim/Complete must refuse MEDIA operations even when an
  ordinary worker knows their UUID. Media wrappers must refuse ordinary/payment
  operations. One possible minimal implementation is private shared core helpers
  plus fixed family wrappers with restricted EXECUTE; freeze the actual ACL/call
  graph before coding. No security depends on the Go worker kind alone.

Required fixed operations (names/signatures still pending SQL preflight):
register prepared authorization; merchant plan start; merchant request stop;
scoped system cleanup request; claim media operation; load leased media material;
record observation; finish uncertain/policy outcome; safe merchant read projection.
Only plan functions insert the operation and River job in the same transaction.
Use existing authenticated command receipt/audit conventions. No detached enqueue.

## Claim/final-gate matrix

|Operation and mode|Authority required|Provider action allowed|
|---|---|---|
|Start READY → dispatch|Current merchant membership/scope/live permission; still-enabled exact frozen bindings; unrevoked official authorization, start deadline, budget; exact new attempt/job linkage|Exactly one Start|
|Start already dispatched, UNKNOWN or expired DISPATCHING → reconcile|Immutable exact attempt/project/room/operation ownership and a fresh fenced lease; may survive merchant/destination binding revoke|FindByRoom while ID unknown, otherwise Query; never Start|
|Stop dispatch|Durable stop request from currently authorized scoped merchant or scoped system cleanup policy; exact owned attempt with pinned Egress ID|Exactly one Stop|
|Stop UNKNOWN or expired DISPATCHING → reconcile|Same frozen owned target and fresh fenced lease|Query by default; only the bounded same-operation reservation below can authorize a second Stop|
|Terminal operation/resource|Persisted correlated terminal history|No provider effect|

The final material resolver repeats actor/action/mode, tenant/store/session/
attempt/project/room/operation/profile, generation/token/deadline and identity
checks after row-lock waits, immediately before releasing material. For Start
dispatch it additionally rechecks mutable eligibility. Query/Stop only need the
frozen project credential reference and target, not decrypted stream URLs.
The worker decrypts Start material using LKM and validates it again before I/O.

The final eligibility check defines ordering: revocation committed before that
check prevents a Start. Revocation after the check can race a remote call; do not
hold a business transaction across the network to hide this. Detect it on the
next observation/recovery pass and request cleanup. Preserve that late external
fact even if the initiating merchant has since lost access.

## Start, observation, stop and cleanup lifecycle

- Start planning freezes the existing draft program/layout and destination set.
  Subsequent draft/destination mutation is denied; command replay remains subject
  to current authentication. No operation exists before a trusted prepared
  authorization passes current eligibility, version, scope and budget checks.
- Persist immutable room/attempt and job before any call. Lost reply, process
  death, provider timeout and database completion failure keep uncertainty.
  Reconcile the original room, adopt one exact candidate under the live lease
  exactly once. Empty/ambiguous results remain UNKNOWN, never another Start.
- Successful Start acknowledgement is not a terminal job or audience-live fact.
  Continue observing the owned resource. Keep command outcome, transport status,
  per-destination/audience evidence and resource reclamation separate.
- An authorized Stop request is durable even if Egress ID is not known yet.
  Record the reason; discovery must finish first. Once the original ID is pinned,
  create at most one Stop operation/job transactionally from that request. An
  ordinary generic Plan cannot create an owned Stop, even with matching JSON.
- System duration expiry or revoked eligibility creates a scoped audited cleanup
  request without impersonating the old merchant. An unauthorized merchant HTTP
  request is still denied; old ownership is not a bearer capability.
- Stop ACK keeps monitoring; only correlated provider terminal status/timestamps
  and durable history may close resource lifetime. Egress failure can terminate
  resource usage without making the broadcast successful. A later contradictory
  observation is retained/escalated, not allowed to reopen or retarget the resource.
- Bounded retries/generations/age must retain unresolved state and an operator
  escalation if credentials are externally revoked or provider cannot be reached.
  Do not close liability, delete the attempt or create replacement resources to
  make a queue look healthy. Pin historical credential refs through this lifetime.

## Concurrency and SQL constraints still to settle

The global media lock order is all referenced bindings (media and destinations)
in sorted UUID order, then tenant/store, session/program, authorization/attempt,
operations and events. The registry and later planner/worker must share it.
Read locators without authority, then validate them under locks. Shared core
helpers lock binding → operation; media wrappers must not invert this order.
No external call holds these locks. All lease-dependent writes repeat expiry
checks after waits/event writes, matching existing payment finalization practice.

Queue routing/profile checks are database-enforced, including exact native job
ID/kind/args linkage and no orphan jobs on rollback. A worker profile cannot consume
MOCK work as LIVE by changing configuration. Historical rows and queue state
cannot be adopted into live authority by enabling a future provider route.

## Gates and remaining freeze blockers

Use [MLA01–08](../docs/implementation/2026-09-27-media-authority-lifetime.md), with
real isolated PG18/actual-role SQL tests plus TLS provider doubles. In particular,
the combined lost-Start + revoke + restart test must prove Start count stays 1,
and ordinary-role direct claim/complete/material SQL must fail with no state
change. Transaction rollback, no orphan job, wrong tenant/profile/family, stale
lease/token, concurrent workers and queue restart are required negative cases.

Before changing production code: settle the actual migration/role wrapper and
same-DB startup checks, freeze Go/SQL function signatures and status mapping,
define trusted official authorization intake and operator cleanup access, review
provider observation terminal rules and test fixtures. LKM custody is an executable
dependency, not closure of any of these controller gates. Real provider/browser
acceptance and the complete SaaS deployment objective remain open.

### Bounded MOCK Stop recovery — design accepted, implementation NOT_RUN

An unconditional query-only UNKNOWN Stop rule is conservative, but is **not a complete
reclamation algorithm**: if the request was lost before acceptance, the same
owned resource can remain ACTIVE. Do not freeze or implement that rule as an
indefinite successful-cleanup loop. Never convert timeout, empty list or Stop ACK
into resource termination. One logical Stop operation is not, by itself, proof
that every additional wire Stop request is either safe or forbidden.

The local algorithm uses one logical Stop operation and at most two wire Stop
reservations. Persist `wire_count=1` before the initial call; a crash after this
reservation consumes the budget conservatively. On uncertainty, Query the exact
pinned project/room/Egress ID. Never issue a replacement Start.

The one exceptional second Stop requires a synchronous exact-ID Query made under
the current fenced lease after the first wire reservation. Cached observations,
webhooks and history do not qualify. In the same transaction that reserves
`wire_count=2`, recheck the exact observation ID/source/target, current generation,
token and lease expiry, DB-clock observation age <=5 seconds, >=5 seconds since
the previous reservation, and absence of newer ENDING or terminal evidence. Repeat
the final material gate after lock waits. Use the same operation and target; no
new operation or generic retry flag. STARTING, ENDING, empty, malformed, error and
terminal observations never authorize a resend. No third wire reservation.

Five seconds is pacing, not proof the first request finished: the client request
ceiling is ten seconds and delayed ACTIVE observations can race accepted stops.
Tests must distinguish loss before acceptance, acceptance followed by lost reply,
delayed ACTIVE, concurrent workers, expired leases and crash after reservation.
The local fixture must demonstrate <=2 wires and <=1 accepted EOS. Exhaustion
stays unresolved/escalated with usage liability until correlated terminal proof.

This freezes the MOCK algorithm only. LIVE intake/activation requires a separate
authorized Cloud same-ID duplicate/fault gate; the open-source handler's
`eosSent.Once` does not establish a universal Cloud contract. Independent review:
Humaux `54f9c14a-72eb-4bf8-af27-b45acf5c741e`; fixed-source research
`fda346dc-4f4d-4712-bb42-31b11635853c`.

Read-only official evidence checked 2026-09-27:

- [Egress API](https://docs.livekit.io/reference/other/egress/api/#stopegress)
  describes stopping an active egress but supplies no general retry guarantee.
- [Server StopEgress source](https://github.com/livekit/livekit/blob/93f5b3eb615c792b8749b0532ff9855d9fd88418/pkg/service/egress.go#L371-L409)
  targets an Egress ID, looks up status on an RPC error, and can return an error
  even for a non-active resource. This fixed upstream snapshot is supporting
  evidence, not proof of a particular Cloud deployment's behavior.
- [ListEgress documentation](https://docs.livekit.io/reference/other/egress/api/#listegress)
  and [Python API reference](https://docs.livekit.io/reference/python/livekit/api/egress_service.html)
  differ on completed-record retention. The controller cannot treat an empty
  result as terminal evidence or assume indefinite recoverability.

Source recheck at `3cb9868` confirms that `Client.Start` and the frozen LKP
contract **already use StartEgress**, with template/preset/outputs and independent
exact-path/body tests. No API-method migration is needed. The earlier note
suggesting the implementation still used deprecated StartRoomCompositeEgress
was incorrect. These local tests still do not prove real Cloud/media behavior;
provider acceptance remains a separate gate.
