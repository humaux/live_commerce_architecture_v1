# Browser input worker and delivery v1 (BRW)

Status: **DESIGN_ABI_FROZEN — SQL_EXECUTOR_SUBSET_PASS; full product gates pending**.
Design base `5a543e7`, independently closed at `f518c77` on 2026-09-27
(Humaux review `7ca5ab59-6fc3-4d40-ab06-31bf01c9dfed`).
Refines [BIC](live-browser-input-runtime-v1.md), [BRI](live-browser-input-v1.md)
and [LKI](livekit-input-protocol-v1.md). The wire ABI is frozen for implementation,
not accepted as running SQL/HTTP/worker behavior. No token route or new consumer is enabled by
this document. All Cloud, customer hardware and production gates remain NOT_RUN.

The isolated `NewBrowserInputRuntime` constructor is implemented at `a389f9b`,
independently tested and accepted CONFIG_ONLY_LOCAL_UNIT. Root's five-package
race/vet gate: 79 PASS / 0 FAIL / 0 SKIP; no route/job/provider activation.
See [configuration receipt](../docs/implementation/2026-09-27-browser-input-config.md).
The local original-job executor is implemented at `c5160de`; fifteen scoped
PG/worker tests pass. [Worker validation](../docs/implementation/2026-09-27-browser-input-worker.md)
records failures, the NULL-allowlist correction and source-level full709 PASS.
Later test-only fault cases pass separately at `b446e49`, together with old BIC6;
they are not part of full709. Complete BRW01–04 coverage is still pending.
This is not acceptance of the remaining HTTP/browser/recovery product gates.

## One execution owner, two independent liabilities

Reuse the original attempt, operation, River kind/job, BIC input custody, MLC
login custody, LKI client and Egress projector. No second Stop job, token cache,
queue engine, provider router or new service. Root allocates only forward SQL
migrations at implementation time; historical migrations do not change.

`LOCAL_SFU_MOCK_EGRESS` is still MOCK output, with a separately pinned local SFU
input connection. The legacy queue/profile and ordinary Studio rehearsal routes
keep their current behavior. An old binary never consumes the input queue.

The operation can finish only under BIC's joint completion rule. Input cleanup
exhaustion does NOT suppress outstanding Egress query/Stop responsibility.
Egress terminal does NOT suppress input cleanup. The original job cannot
complete, be cancelled, discarded, deleted or replaced while either liability
is unresolved. No new generation at held idle; snooze the original job 60s.

The native job guard must enforce the joint predicate, not merely input
`state != CLOSED`: an input-profile attempt remains protected unless input is
CLOSED AND either Egress is coherently TERMINAL or Start has been permanently
disabled without any wire reservation. Apply this to all native completion,
cancel/discard/delete/rescue paths and preserve the existing MRR batch-safe
pending/hold behavior. Missing or contradictory custody/projection fails closed;
absence of a child row is not terminal proof. Legacy guards remain unchanged.

## Deadline correction when activating BIC

The BIC-only claim currently checks `start_before` and dispatch eligibility on
every claim. BRW must distinguish pre-wire admission from post-wire lifetime:

- Before the single Start reservation: enforce `start_before`, BIC lifetime,
  exact login, current authorization/permissions, scope and binding versions.
- After reservation (including lost Start reply): never permit a new Start;
  `start_before` alone no longer closes a running input. Enforce fixed BIC
  lifetime, MLC login loss, current lifetime eligibility and sticky Stop/revoke.
- Initial grant expiry is not a connected participant lifetime limit. Never
  renew its frozen grant or rely on JWT TTL/empty-room timeout for cleanup.
- Admission closure is sticky. Keep BIC's issued state UNKNOWN; additional
  cleanup progress is not proof of CLOSED. No migration reopens old UNKNOWN
  custody or silently schedules provider calls for historical held attempts.

Activation is restricted to fresh, explicitly registered BRW attempts. Add an
immutable private runtime-version marker at planning, under the existing
authorization lock. BIC rows without this marker retain kernel-only behavior
and cannot emit tokens or reach provider calls. Marker selection is deployment
controlled, never a merchant request field. No adoption of old held attempts.

Concrete marker ABI: registrar-only
`live.register_media_input_runtime_profile(uuid authorization_id) RETURNS void`
registers a private immutable child `live.prepared_media_input_runtime_profiles`
(authorization_id primary key, scoped FK to BIC prepared input profile,
runtime_version fixed 1). It uses the same authorization lock as BIC planning,
requires unused/unrevoked/unexpired MOCK authority and rejects registration
after any attempt. `plan_media_input_start` snapshots runtime_version into a
new immutable input-custody column (default 0 for all old rows). Existing BIC
callers may still plan a marker-0 kernel attempt; only version 1 enters BRW.

`live.load_media_input_runtime_profile(bytea,uuid,uuid,uuid,bigint) RETURNS jsonb`
is runtime-only (login hash, store, session, authorization, expected version).
It repeats exact scoped access/current revision/eligibility checks and returns
only `project_id`, `endpoint_identity`, `credential_version`, `runtime_version`.
Go `MediaPlanner.PlanBrowserInputStart` takes the same arguments/result as
`PlanInputStart` plus a nonnil `*BrowserInputRuntime`; inside the caller's
transaction it checks that profile against the runtime mapping and reuses
`PlanInputStart`. No second planner job or receipt. Token delivery repeats this
marked-attempt check, including replay; kernel-only receipts are never upgraded.

## SQL authority and wire ABI

All listed functions are SECURITY DEFINER, VOLATILE, fixed `pg_catalog` search
path, READ COMMITTED only, owned by the existing private media writer. Public
and runtime/worker table access remains denied, FORCE RLS remains enabled.
Only the media executor can invoke provider-step functions; runtime invokes
the existing planner/reserve functions. Readiness checks exact signatures,
owners, ACLs, profile/queue linkage and current native-job guard.

Every provider step requires the original operation/job, runtime marker,
generation, SHA-256 of a 32-byte lease token and an unexpired 1–30s lease.
Use existing lock order, including MLC's login locks; no provider I/O inside
transactions. Recheck clock, authority and lease after blocking writes. A lost
reservation COMMIT ACK never authorizes I/O. A lost result ACK consumes the
reservation and never repeats that same wire step.

### Observation and single Start

Add executor-only
`claim_browser_input_operation(uuid,bigint,integer,bytea)` with the existing
`(disposition text,generation bigint,mode text)` result. It checks the original
input queue/job and immutable marker BEFORE granting any provider-capable lease.
Marker 0 returns `kernel_only`, current generation, empty mode without changing
lease/generation; worker snoozes 60s and performs no provider I/O. Marker 1 uses
the corrected lifetime/independent-work algorithm and normal busy/claimed/
terminal/held dispositions. An eligible runtime-1 UNISSUED attempt returns
`await_admission`, current generation, empty mode without granting a lease,
incrementing generation or performing provider I/O. Existing `claim_media_input_operation` retains
marker-0 BIC behavior but rejects marker 1, preventing old semantics from
dispatching or prematurely closing runtime attempts. This is a distinct ABI,
not an inferred marker from custody state or the unchanged load DTO.

Continue `load_media_input_custody(operation, generation, lease_key)` for the
fenced nonsecret BIC target; its exact DTO remains unchanged. Add:

```sql
live.reserve_media_input_start(
  uuid, bigint, bytea, -- original operation, generation, lease key
  text, text, text, text, -- room, publisher identity, participant SID, state
  boolean, boolean, boolean, boolean -- camera present/muted, mic present/muted
) RETURNS jsonb;
live.load_media_input_material(uuid,bigint,bytea) RETURNS jsonb;
live.next_media_input_turn(uuid,bigint,bytea) RETURNS text;
live.finish_media_input_turn(uuid,bigint,bytea,text) RETURNS text;
```

Worker first performs exact LKI `ObserveInput` using the loaded target. The
same claim's final Start reservation accepts only JOINED/ACTIVE, valid `PA_`
SID, both camera and microphone present and unmuted, exact room/identity,
open RESERVED custody and no previous `wire_reserved_at`. Missing/not-ready
input releases the lease and snoozes 5s, bounded by start/lifetime/generation
limits; it never authorizes Start. A fresh observation is made on each turn.

Final reservation atomically stores bounded participant observation metadata
and DB observation time, sets `wire_reserved_at`/`wire_generation` once, and
changes this same operation/lease from UNKNOWN/reconcile to DISPATCHING/dispatch.
It returns exactly the legacy 15-key `mediaLoaded` material DTO with dispatch
mode and the pinned sealed material. The worker validates/unseals it and sends
at most one Start after successful COMMIT. Failure to unseal consumes the
reservation conservatively; never clear it to retry Start.

Stop/revoke committed before final reservation prevents Start. A concurrent
Stop/revoke after it leaves possibly-dispatched responsibility; use existing
FindByRoom/Query, two-Stop budget and projections. No promise of atomic database
and provider cancellation. Metadata is not decoded video/audio evidence.

`load_media_input_material` returns the same 15-key DTO in reconcile mode only
after a wire reservation, with empty key/nonce/ciphertext. It permits exact
cleanup after loss of merchant access, not fresh dispatch. Never query Egress
for a never-wire attempt or re-query terminal Egress just to make progress.

`finish_media_input_turn` releases the lease and returns `observe`, `held` or
`terminal` using joint responsibility, including pre-wire and post-terminal
input paths that legacy `finish_media_uncertain` cannot cover. Its reason
allowlist: input_not_ready, input_unknown, credential_unavailable,
material_invalid, cleanup_unknown. No raw provider errors or secrets stored.
It never makes UNKNOWN terminal from an error or elapsed time.

### Bounded cleanup reservations

Add one private bounded child table of the existing input custody, NOT another
operation/job. Key `(attempt_id, ordinal)`; ordinal 1..8, fixed operation kind
derived below, original operation/generation, DB reservation timestamp,
result `PENDING|ACK|PRESENT|ABSENT|UNKNOWN`, optional bounded participant/room SID
and DB result timestamp. No raw JSON, JWT, headers, names or provider errors.
Identity/project/credential are read from immutable BIC, never caller supplied.
Rows are append-once; only that reservation's current fenced generation may
record a result once. Reservation/result ACK loss cannot free or refund budget.

```sql
live.reserve_media_input_cleanup(uuid,bigint,bytea) RETURNS jsonb;
live.record_media_input_cleanup(uuid,bigint,bytea,integer,text,text) RETURNS text;
```

Reserve determines the next ordinal (caller cannot choose step/target) and
returns exact keys `ordinal`, `action`, `room_name`, `publisher_identity`,
`project_id`, `endpoint_identity`, `credential_version`, `revoke_before`.
`revoke_before` is DB Unix seconds only for REMOVE; null otherwise. Require
provider clock validation immediately before Remove; stale cutoff consumes
the reservation and records UNKNOWN. No detached/background retry.

|Ordinal|Action|Accepted result class|
|---|---|---|
|1|REMOVE|ACK or UNKNOWN|
|2|DELETE_ROOM|ACK or UNKNOWN|
|3|READ_PARTICIPANT|PRESENT or UNKNOWN|
|4|READ_ROOM|PRESENT, ABSENT or UNKNOWN|
|5|REMOVE|ACK or UNKNOWN|
|6|DELETE_ROOM|ACK or UNKNOWN|
|7|READ_PARTICIPANT|PRESENT or UNKNOWN|
|8|READ_ROOM|PRESENT, ABSENT or UNKNOWN|

The first round always permits exact reads after uncertain mutations. The
second round requires freshly correlated PRESENT results for both reads 3/4;
unknown/not-found does not authorize a destructive retry. Non-200 participant
not-found remains UNKNOWN under LKI. Room ABSENT is a snapshot, not revocation.
This deliberately conservative first version may hold even when the room is
still present but its publisher is not observable; it does not claim a second
Delete will always be attempted. Independent per-resource retry is deferred
until evidence shows that the extra state machine is needed. Remaining room
liability must be visible in the same operator projection, never cleared.
Before mutation retry, required reads must be at most 30s old; otherwise hold,
not silently add queries or new budget. All unfinished PENDING rows at expired
lease count as consumed UNKNOWN; old replies cannot mutate newer state.

Maximum: two Remove, two Delete and four reads. Freeze cleanup deadline to
first sticky admission-close time +180s, not first successful worker execution;
never extend on restart. One input-provider call per claim, with at most 10s
I/O; independent original Egress work may use subsequent turns. Exhaustion,
deadline, missing credentials or an unsafe retry precondition records an
operator-visible UNKNOWN/held input responsibility. This is a bounded effort,
NOT a promise that all eight calls happen or a hard remote kill cap.

Worker alternates eligible input-cleanup and outstanding Egress reconcile
turns using input-custody `last_runtime_generation` and `last_runtime_turn`,
not process memory. `next_media_input_turn` selects `INPUT_OBSERVE`, `EGRESS`,
`INPUT_CLEANUP` or `HELD` under the same lease and persists its choice before
wire I/O. Repeated selection in that generation returns that exact choice.
Each eligible side gets a reserved opportunity before the other repeats;
a terminal/held side is skipped. Input hold does not
cancel outstanding Egress; Egress hold does not cancel remaining input cleanup.
When both have no automatic work, snooze 60s without new generations/provider
calls. No automatic reset of budgets; operator recovery requires a later
separately reviewed exact-resource protocol. No force-close endpoint.
Persist input hold separately (`input_cleanup_held_at`, bounded reason) from
the existing Egress escalation fields. This independence applies within the
original operation's global 4096-generation/24h budget; that global limit
holds BOTH liabilities, retains the original job and emits an operator item.
Do not promise continued automatic cleanup past that cap. Claim checks shared
hold before incrementing generation; new code cannot translate BIC's generic
`state=UNKNOWN` into an unconditional per-input hold.

## Deployment binding and HTTP seam

Use explicit, disabled-by-default local input configuration keyed by exact
project + credential version + existing Cloud-shaped endpoint identity. It
contains the LKI client transport to disposable local SFU and a separately
validated browser connect URL. The Egress client remains its configured MOCK
double. Never treat the frozen endpoint identity as a browser URL or relax
`livekit.New` production URL checks. Separate SFU and Egress credentials/clients
are required when their test services use different keys.

Local binding accepts only loopback test URLs and explicit local-run mode.
Reuse `LoadWorkerProjects`' existing loopback TLS transport/CA parser for the
SFU management client, via a disposable fixture TLS proxy to native SFU. Do not
add URL-rewriting HTTP transport or change `Client.New`. Browser connects to a
separately validated `ws`/`wss` URL with canonical 127.0.0.1 or [::1], explicit
1..65535 port, no path other than empty or `/`, no credentials/query/fragment.
The selected factory is `live.NewBrowserInputRuntime([]BrowserInputProject)`;
each project contains exact ProjectID/CredentialVersion, independent SFU Config
and Transport and BrowserURL, and redacts all formatted/JSON output. Existing
Egress MediaProject and credentials are not repurposed as input credentials.
Runtime construction alone does not register a route or consume a queue.

Frozen configuration-only Go ABI, in package `live`:

```go
type BrowserInputProject struct {
    ProjectID string
    CredentialVersion int64
    Config livekit.Config
    Transport http.RoundTripper
    BrowserURL string
}
type BrowserInputRuntime struct { /* private immutable keyed map */ }
func NewBrowserInputRuntime([]BrowserInputProject) (*BrowserInputRuntime, error)
```

The worker consumes only `media_input_mock_v1`. Its explicit factory is
`NewBrowserInputMediaClient(context.Context, *pgxpool.Pool, *pgxpool.Pool,
*livekit.MaterialKeyring, []MediaProject, *BrowserInputRuntime, int)` returning
`(*river.Client[pgx.Tx], error)`; worker pool precedes executor pool.
`MediaPlanner.PlanBrowserInputStart` retains `PlanInputStart` arguments plus a
final `*BrowserInputRuntime`. Construction checks
`live.media_browser_input_worker_ready() RETURNS boolean`; it does not itself
start consumption or activate any HTTP route.

Require 1..128 projects, existing project-ID grammar, positive version, unique
project/version key, Config.Environment exactly MOCK and `livekit.New`'s strict
config/non-nil transport validation. BrowserURL must exactly match the canonical
loopback URL grammar above (maximum 256 bytes). No environment loading, default
transport, DNS lookup, signing, HTTP call or runtime side effect in constructor.
Use existing `ErrMediaConfig` only; failure returns nil, never a partial map.
Copy map/string configuration; reuse LKI's defensive stream-host copy. Both
new types implement constant redacted String, GoString and MarshalJSON. Store
only private exact endpoint/client/browser URL bindings, reuse mediaProjectKey.
Do not add speculative exported lookup/mint/lifecycle methods in this unit.
Injected RoundTripper is trusted fixture configuration: its route cannot be
proven by inspecting an interface. This factory does NOT attest transport
loopback or production safety. Real fixture must use the pinned TLS parser,
and the production command wiring remains nil as required below.

For this local unit `cmd/api` and production startup keep the input runtime nil;
no environment flag silently enables it. The isolated authenticated browser
fixture injects it through explicit typed options. Generic runtime mode strings
are not proof of a nonproduction environment. Production wiring is a separate
reviewed change after qualifications; default/nil constructors preserve the old
runtime and expose no token endpoint. Merchant payload cannot
select either transport. A readiness check must reject missing/duplicate/
mismatched mappings before input planning or token delivery. Cloud support
requires BRI06 and a separate profile; local mapping is never a Cloud fallback.

Two new exact same-origin POST actions, authenticated cookie BFF -> Go:

- `live-sessions/{session_id}/input/start`: body exactly `authorization_id` and
  `expected_session_version`; uses `PlanBrowserInputStart` (including replay's
  marker/mapping checks) and existing safe receipt, never raw `PlanInputStart`.
- `live-sessions/{session_id}/input/token`: body exactly `attempt_id` and
  `expected_session_version`; uses `ReserveInput`, then signs **after** successful
  `platform.WithScope` COMMIT. Never sign inside its transaction callback.

Go prefix stays `/v1/admin/stores/{store_id}/`; BFF stays
`/api/stores/{store}/`. Idempotency-Key required, strict body parser and exact
allowlist, no query, no credentialed cross-origin CORS, existing Origin/CSRF
and signed-cookie store membership; no fixture-session shortcut on these routes.
The existing rehearsal/stop action remains the sole merchant Stop, including
input attempts; input disconnect UI invokes it rather than adding a second
server Stop API. No token endpoint enabled before the original-job worker,
readiness and lifetime gates pass.

Add exact GET `live-sessions/{session_id}/input` (no query/body/key), backed by
runtime-only `live.read_studio_input(bytea,uuid,uuid) RETURNS jsonb` under the
existing scoped `live:read`/revision checks. Return null for no input attempt,
otherwise exactly `attempt_id`, `state`, `admission_closed`, `close_reason`,
`cleanup_held`, `can_stop`, `updated_at`; no credential mapping or publisher JWT.
`can_stop` additionally requires current `live:manage` and open joint liability,
not Egress nonterminal alone. Forward `read_studio_media` to recognize marked
input attempts while preserving its existing strict output DTO. Kernel-only
attempts remain rejected there. Legacy prepared-candidate selection excludes
all input-profile authorizations (including kernel-only marker 0), which the
input controls select explicitly through the separate reader below.
UI must use input `can_stop` when a marked input exists; Egress terminal must not
disable needed Stop. This read projection is operator-readable evidence, not
proof a human alert was delivered; its visible controls are still a BRI07 gate.

GET `live-sessions/{session_id}/input/prepared` has the same exact-route,
HTTPS, signed-session, no-query/body/key and private no-store boundary. Return
null or the existing five-field StudioPrepared DTO: `authorization_id`,
`session_version`, `start_before`, `environment`, `destinations`. Runtime-only
`live.read_studio_input_prepared(bytea,uuid,uuid)` selects an unused runtime-1
input authorization for the current DRAFT/program/version/aspect, before its
deadline, without revocation and with current destination bindings. Its private
SQL envelope is exactly `prepared`, `project_id`, `credential_version`,
`endpoint_identity`. Go verifies the immutable runtime project mapping and
endpoint, strips those three mapping fields, and rechecks the draft snapshot.
Missing mapping fails closed; this advisory selection never replaces Start's
locked checks. Both input GET responses are capped at 8192 bytes in Go/BFF.
Initial and final read authority/revision/session-expiry checks apply. INPUT
`cleanup_held` is independent from Egress escalation; a held input or revoked
authorization does not remove a current manager's cleanup Stop permission.

Response is an explicit private no-store bounded DTO with exactly `attempt_id`,
`room_name`, `publisher_identity`, `url`, `token`, `expires_at`. Token is the
only deliberate secret response, never part of receipt/database/log/error,
URL, telemetry, storage, trace or screenshot. Do not return API keys/secrets or
redirect responses. Body <=8192 bytes; BFF enforces the same ceiling. Replays
recheck current admission and fixed grant; a later revoke races into existing
liability, not an atomic-delivery guarantee. Commit ACK loss returns no token.
BRW05/06 fixtures must use an HTTPS authenticated BFF origin and a browser-
accepted SFU WebSocket transport. A convenient HTTP localhost fixture does not
satisfy BRI's HTTPS token-delivery gate; record it as NOT_RUN for that gate,
not an implicit exception or a production transport relaxation.

## Independent acceptance before activation

|Gate|Required evidence (not a specification-model substitute)|
|---|---|
|BRW01|Actual-role PG ACL/RLS, original job/profile/marker fencing, rollback/ACK loss; old BIC/legacy cannot access new wire functions.|
|BRW02|Dual-track final gate, wrong/muted/missing publisher rejection; concurrent Stop/revoke before/after reservation, stale generation, exactly one Start.|
|BRW03|Per-step reservation/result/drop/crash; <=2 Remove/2 Delete/4 reads; strict target/cutoff, 180s deadline, no third mutation, no false CLOSED.|
|BRW04|Original job survives Stop/logout/access loss/expiry/API or worker crash, before wire/after reply/Egress terminal; independent input/Egress holds do not starve the other.|
|BRW05|Signed cookie BFF -> Go -> actual PG -> postcommit response, private transport, exact DTO, secret scan; failed commit and revoked replay return no token.|
|BRW06|Actual browser -> product token -> fixed local SFU -> separate receiver decoded frames and nonzero audio meeting existing RLI thresholds; worker observes camera+mic before Start.|
|BRW07|90s abnormal-process recovery measured from process launch with native original job and real clocks; existing MRR pass does not automatically cover this new queue.|
|BRW08|Independent review, targeted legacy+BIC+MLC+MRR/Studio regression, clean full PG/race/vet, bounded dependency/evidence cleanup.|

BRI06 Cloud revocation, BRI07 approved input controls across locales/viewports,
real output/audience, human-delivered alerts and production rollout remain
separate pending gates. The existing Studio B layout is reusable; that approval
does not by itself approve newly added camera/mic/UNKNOWN controls.
