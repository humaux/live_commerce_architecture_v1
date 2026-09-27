# R04 browser input authority and lifetime v1

Status: **DESIGN_CANDIDATE**. Base `dd9af5a`; no product admission route, LIVE
registrar, Cloud call, migration or Studio UI is enabled by this document.
The accepted [local transport probe](../docs/implementation/2026-09-27-r04-local-input-acceptance.md)
does not satisfy these product gates. T08 remains IN_PROGRESS; G06 remains
NOT_RUN_PRODUCT. Review this boundary before freezing SQL/API signatures.

## Product decision and reuse

1. Before an explicit start, camera/microphone preview is local `getUserMedia`
   only. It creates no provider room, Egress or billable preview session. Display
   it as **local capture**, never server output or audience-live evidence.
2. After explicit start authorization, persist the attempt, its fixed room and
   the existing `integration.operations`/River job in one transaction. Browser
   admission uses that attempt; a separate pre-start room or Stop ledger is not
   needed. A join grant is not a destination credential or permission to Start.
3. The same typed media controller must wait for the admitted publisher's
   observed camera AND microphone tracks before its once-only Egress Start.
   This wait consumes the existing start deadline and does not reserve a Start
   wire call. Stop, expiry or access loss committed before the final dispatch gate
   must prevent Start and still reclaim issued input capabilities/room. A revoke
   racing after that gate cannot be made atomic with provider I/O: retain the
   reserved attempt as possibly dispatched and perform exact query/cleanup,
   never claim zero provider calls merely because revoke committed first on a
   different connection.
4. Reuse merchant `WithScope`/`RequirePermission`, command receipts, actual-role
   PG tests, the existing attempt generation/lease and owned cleanup authority.
   Extend these deliberately; do not bypass generic dispatch admission or create
   a second transaction engine, polling service or queue framework.

The first browser version has one publisher and its initiating merchant login
session. Other authorized merchants may observe safe status or request Stop;
they cannot silently take over that publisher. Logout of another login session
does not stop this input. Publisher transfer, screen sharing, audio-only start,
cohosts, OBS and a server-output viewer grant require separate explicit contracts.
They are not implied by a camera/microphone token. These are upgrade boundaries,
not a claim that the broader R04 requirement is finished.

## Existing boundaries that MUST survive

- `live.prepared_media_authorizations`, `live.media_attempts`, the executable
  worker and Studio projection at the base revision are **MOCK-only**. Never
  rewrite old environment/profile rows, reuse their jobs, or relabel their
  evidence as LIVE. A new Cloud attempt requires independently qualified LIVE
  authorization, a dedicated validated execution profile and fresh identifiers.
- `livekit.Client` currently manages Egress, not browser admission. Its HTTP
  target allowlist, bounded responses, redacted diagnostics and no-redirect/
  no-proxy controls must remain. Shared signing/transport code may be reused;
  no new server SDK is justified merely to mint a small HS256 JWT.
- `identity.resolve_access` denies future HTTP calls after revocation. It does
  **not** disconnect an already connected provider participant. HTTP auth and
  provider resource custody therefore require separate checks.
- Egress terminal proof alone currently concerns Egress. Once an attempt has
  input custody, total resource closure requires **both** Egress closure (or
  proven no Start dispatch) and input closure. A successful Stop HTTP response,
  an empty room once, JWT expiration or tab close is not full closure.

## Authority persisted before delivering any join token

Use an attempt-owned input record, not a new generic operation/Stop log. Exact
SQL column/function names and role grants remain to be frozen and independently
reviewed before implementation. Required immutable identity is:

- tenant, store, live session, program and frozen session version;
- attempt and original media operation IDs; deployment execution profile;
- frozen provider project, endpoint and credential version (never merchant URLs);
- initiating principal, **merchant login session ID**, authorization revision;
- server-generated input incarnation and unique publisher identity;
- issued-at/not-before, join expiry, start deadline and overall lifetime deadline.

The input incarnation is independent of the worker's fencing generation. A
worker restart does not invent a new publisher identity. Identity and room are
never reused for another attempt. The first version issues one fixed publisher
identity per attempt, with no implicit new grant after its admission closes.
Every identity that could have received a token remains queryable for cleanup,
including a response lost after commit and a participant that has left already.

The merchant submits only the exact attempt ID, expected session version and
idempotency key; never room, identity, grants, deadlines, host, project or secret.
At reservation and again after relevant lock waits, require current merchant
session, tenant/store membership, `store:read` and `live:manage`, exact frozen
attempt ownership/profile, eligible authority and unexpired deadlines. Replay
must repeat current access checks; an old receipt is not a bearer capability.

Persist the receipt/identity and cleanup obligation atomically **before** a
token can leave the server. A rollback emits no token. A commit-ack loss can only
replay the original identity/timestamps after current checks, never mint another
identity or lengthen a deadline. Do not store raw join JWTs or provider secrets in
command receipts/audit/SQL. Signing uses the pinned deployment credential version
and fixed claims. A delivery that races a later revoke remains in cleanup scope;
do not pretend that a post-transaction HTTP check creates atomic revocation.

The current binding → tenant/store → session/program → authorization/attempt →
operation/event lock order remains. The later SQL review must place login/input
locks without deadlocking logout or grant changes and prove linearization under
real PG contention; no provider network call holds database locks.

## Browser grant and HTTP custody

- HS256; `iss` is the configured key; `sub` the fixed publisher identity; exact
  room; finite fixed `iat`, `nbf` and `exp`. Join window is at most 60 seconds and
  cannot extend past the frozen start/lifetime/login-session deadlines.
- `roomJoin=true`, `canPublish=true`, sources exactly `camera,microphone`;
  `canSubscribe=false`, `canPublishData=false`, `canUpdateOwnMetadata=false`.
  No roomCreate, roomList, roomAdmin, roomRecord, ingressAdmin, agent, destination
  room, metadata, arbitrary attributes or screen-share permission.
- A grant authorizes sources, not hardware provenance: a malicious client could
  publish synthetic camera-labelled content. Do not claim hardware attestation.
- Authenticated same-origin POST via the existing private BFF; exact allowlist,
  strict request parsing, CSRF/origin checks, no-store, no credentialed CORS to
  third parties, bounded request/response. Final API path/DTO are not frozen yet.
- Token only in the HTTPS response body to the authorized browser and in browser
  memory. No query strings, localStorage, telemetry, URLs, traces or screenshots.
  An explicit transport DTO may serialize the token; debug/error/receipt DTOs
  must redact it. API/SDK keys never cross the BFF.
- Local tracks stop on user Stop, logout, component teardown and terminal input
  state; this is UI hygiene, **not** the authoritative reclaim mechanism.

LiveKit refreshes connected participants' tokens. The initial 60-second expiry
therefore does not enforce session lifetime or prevent all later rejoin. Source:
[official token lifecycle](https://docs.livekit.io/frontends/reference/tokens-grants/).

## Durable lifetime and truthful closure

Input status is separate from program/audience status: `UNISSUED`, `ADMITTED`,
`OBSERVED`, `CLOSING`, `CLOSED`, or `UNKNOWN`. Worker observations are fenced;
replies from an expired generation cannot reopen or close a newer state. Never
derive OBSERVED from the browser saying that it published.

The original media operation/job must remain reconcilable while it owns an
issued input identity, including when Egress was never started or is terminal.
Do not let existing Egress-only terminal handling release this obligation. The
following are irreversible close intent for the original attempt:

- authorized merchant Stop or explicit publisher disconnect;
- initiating login logout/expiry, loss of current publishing eligibility;
- exhausted start deadline or frozen duration cap;
- a revoked media authorization or controller-detected terminal media failure.

No browser heartbeat is required to make these rules true. The same durable
controller rechecks the persisted login/permissions/deadlines and resumes cleanup
after API/worker restart. Notifications/webhooks may wake it but cannot be the
only mechanism. No raw merchant bearer token is retained for system cleanup.
Historical resource ownership permits only exact observation/reclaim, not new
admission, a new Egress Start or a caller-selected target after access loss.

Close algorithm (provider operations outside DB locks, each reserved/fenced and
bounded before call):

1. Close admission durably; no token renewal, replay delivery or new incarnation.
2. Revoke/remove **every recorded publisher identity** in the fixed room/project,
   including issued-but-never-seen and already-left identities. Use an explicit
   current `revoke_token_ts`; do not rely on the provider's default cutoff.
3. Delete only that attempt's room and read back room/participant absence. This
   is separate from Egress query/Stop and does not stand in for Egress terminal
   proof. Partial cleanup records which resources remain uncertain.
4. Strict CLOSED also requires a qualified Cloud revocation profile: original
   and SDK-refreshed cached tokens cannot rejoin after cleanup, including clock
   boundary, left-participant and missing-room cases. Gate BRI06 qualifies the
   profile; individual calls still need successful correlated acknowledgements.
   Do not infer a successful revoke from any unexpected not-found response.

Retries observe the same target first, use an explicit fresh cutoff (the API
accepts only a short clock window), and retain the original identities. Calls
are bounded by a frozen cleanup budget. Exhaustion, invalid cutoff/clock, lost
response or an ambiguous provider result means UNKNOWN plus an operator item;
never CLOSED or a silently new room. The exact budget, polling intervals,
reservation SQL and post-escalation operator recovery remain implementation
freeze items. Existing two-Stop Egress budget is not automatically a room budget.

Provider scope is critical. Official [RemoveParticipant semantics](https://docs.livekit.io/intro/basics/rooms-participants-tracks/participants/)
document Cloud token revocation for an already-left participant with explicit
cutoff. They do not establish the room-already-missing or same-second `nbf`
boundary. A future cutoff must not be assumed valid. Self-hosted removal does
not invalidate cached JWTs. DeleteRoom and empty-room timeout alone do not prove
revocation. No local self-host PASS can replace BRI06.

If all controllers are unavailable or the provider cannot be reached, this
design has **no proven hard kill cap** for a connected participant. It records
overdue/UNKNOWN liability and resumes exact cleanup when service recovers.
Production admission additionally requires an operated cleanup SLO, escalation
owner and independent outage drill; where an unconditional cap is a business
requirement, a provider-enforced cap must be demonstrated before enabling LIVE.
Do not promise safety from token TTL, user honesty, empty_timeout or webhook
delivery. [Webhooks are not guaranteed](https://docs.livekit.io/intro/basics/rooms-participants-tracks/webhooks-events/).

## Acceptance gates (all product gates NOT_RUN at this revision)

|Gate|Required executable evidence|
|---|---|
|BRI01 authority|Real isolated PG/actual roles: positive exact owner; wrong tenant/store/attempt/session/profile, missing grants, revoked/expired login and old revision rejected; command replay does not bypass current auth.|
|BRI02 reservation and concurrency|Concurrent same-key requests yield one identity/fixed expiry; key/body conflict; rollback and COMMIT-ack loss; grant vs Stop/logout contention; stale worker generation; no raw JWT/secret in DB/log/receipt.|
|BRI03 wire and grant|Independently decoded/verified JWT has exact least grants and bounded times; malformed scopes never sign/call; target response mismatch, oversized/malformed body, redirect, network and provider errors do not claim observation/revoke.|
|BRI04 product media|Actual browser → authenticated BFF → Go/PG → disposable local SFU with camera+mic to independent receiver. Server observes the exact admitted publisher, camera+mic; Egress Start cannot precede this or occur twice. Causal revoke before the final gate prevents Start; revoke after reservation remains possibly dispatched and is reconciled/cleaned. Local preview alone makes no provider call.|
|BRI05 lifetime and crash|Logout/access loss/expiry/Stop before and after join; publisher tab crash; API/worker crash at reservation, before wire, after reply and completion-ack loss. The original operation survives Egress terminal until input closure; uncertain cleanup stays UNKNOWN. Local nonrevoking rejoin is recorded as a limitation, never a strict PASS.|
|BRI06 Cloud revocation|Separately approved disposable Cloud project: initial and SDK-refreshed tokens; immediate/same-second removal, already-left and room-already-missing, delete/recreate attempts, clock skew, lost replies, exhausted budget and credential-version loss. Require strict rejection without replacement room/identity. No customer resources.|
|BRI07 UI and operations|Approved Studio comp; desktop/390px and zh-CN/zh-TW/en; explicit device permissions, local/server/output status distinction, camera denied/no mic, replay/error/reconnect, visible UNKNOWN and operator escalation. Cloud outage drill and maintained dependency/runbook evidence.|

## Implementation order and handoff

1. Independently review this lifecycle/authority decision and Cloud limits. Freeze
   the smallest input JWT/RoomService wire profile; reuse standard-library
   transport/crypto and add independent protocol tests.
2. Freeze exact LIVE intake, SQL functions/role separation, one-operation lifetime
   extension and HTTP DTO, including budget/SLO constants. Schema is integrator
   owned. Source/test writers then work in separate worktrees against that freeze.
3. Pass BRI01–05 locally with the real product chain. Keep BRI06 pending until
   disposable Cloud scope/credentials/qualification exist. Do not activate LIVE
   as a side effect of a green mock/local test.
4. Implement the Studio surface only after its own visual choice; the merchant
   orders C decision is not Studio approval. Pass BRI07 and all required global
   regression/release gates before claiming deployable SaaS completion.

Rejected: pre-start billable preview rooms (unneeded extra lifetime), a second
input/Stop job ledger, raw-token receipt storage, reclassifying MOCK as Cloud,
JWT TTL as revocation, and DeleteRoom as proof that cached tokens are unusable.
No dependency additions or running service changes are authorized by this file.
