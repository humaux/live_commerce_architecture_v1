# LiveKit Egress protocol v1 — LKP01–06

Status: CONTRACT_CANDIDATE pending independent preflight. Base `41d0a6a`.
This is the real provider-wire component for T08, tested against an isolated
HTTP provider double. It is not a local intent queue, broadcast orchestrator,
merchant endpoint, provider SANDBOX/LIVE acceptance or global G06 approval.
No route, jobs, migration, customer broadcast or live credentials are enabled.

## Why this dependency comes before LBI

The preliminary [LBI](live-broadcast-v1.md) cannot safely dispatch: generic binding
configuration is not media/Meta entitlement, old LOCAL intents could become live
after a route addition, and binding revocation invalidates the proposed stop path.
Keep LBI non-implementable until reviewed lifetime/credential policy exists.
Do not create doomed READY jobs or dormant start operations in this increment.
Build the provider protocol actually needed by the later authorized controller.

Reuse the existing Go stdlib HTTP/HMAC/JSON primitives and the bounded,
no-redirect/redacted-client pattern from PAYUNi; do not share PSP configuration,
introduce a second queue, add a new SDK, or generalize a cross-provider framework.
No new module dependency. Runtime assembly will supply environment-owned secrets;
this package must never load files, env or merchant input on its own.

## Primary-source baseline (checked 2026-09-27)

- [Official API](https://docs.livekit.io/reference/other/egress/api/): POST JSON
  `/twirp/livekit.Egress/{StartEgress,ListEgress,StopEgress}`; Bearer JWT with
  `video.roomRecord`. StartEgress is available on Cloud; self-host requires
  server 1.13.5+. Old source-specific start APIs are deprecated.
- [Protocol definitions](https://github.com/livekit/protocol/blob/main/protobufs/livekit_egress.proto):
  StartEgress has room_name, template, outputs; one stream output holds RTMP
  URLs. List supports room_name, egress_id, active and pagination; EgressInfo
  contains secret-bearing original requests, stream URLs and raw error/details.
- [Token grants](https://docs.livekit.io/frontends/reference/tokens-grants/):
  HS256 server token, API key issuer, short lifetime, roomRecord only here.

The prose List documentation says active-only; proto exposes active=false.
Therefore neither an empty list nor not_found proves the start never happened
or resources stopped. This client reports only an observation, never no-effect.
No provider idempotency guarantee is assumed for StartEgress.

## Frozen package surface

Package `livecommerce/internal/integrations/livekit`.

`Config { Environment, Endpoint, APIKey, APISecret string; StreamHosts []string }`.
Environment exactly MOCK or LIVE. Endpoint exactly HTTPS and a single valid
lowercase DNS label before `.livekit.cloud`, no userinfo/port/query/fragment;
only empty path or `/` accepted. This increment supports the selected Cloud
baseline, not arbitrary self-host endpoints. Keys: 1–128 ASCII alnum/underscore/
hyphen; secret 32–256 printable non-whitespace ASCII. StreamHosts is a nonempty
unique list of at most16 exact lowercase DNS names, no wildcard, IP literal,
localhost or single-label hosts. These are trusted deployment-approved ingest
hosts, not a claim that a merchant owns/qualifies for a destination.

`New(config Config, transport ...http.RoundTripper) (*Client,error)`.
MOCK requires exactly one nonnil transport; LIVE forbids injection and uses a
new private TLS1.2+ transport with no ambient proxy and keepalives disabled.
Copy config slices so caller mutation cannot change the allowlist.
Fixed 10s request ceiling; context cancellation honored; no redirects/retries.
The HTTP method is always POST, the hostname is frozen, paths constant, and no
Idempotency-Key header implying unsupported remote idempotency is supplied.

`StartInput { RoomName, AspectRatio string; StreamURLs []string }`.
RoomName exactly `lc_` plus32 lowercase hex characters (server attempt identity);
aspect exactly16:9 or9:16. One or two unique stream URLs, each1–4096 bytes,
ASCII without control/space, `rtmps://`, exact configured hostname, no userinfo,
fragment or non443 port, nonempty path. Query strings are allowed because real
provider stream keys may use them; the entire URL remains secret. No arbitrary
web/custom template URLs, storage, recordings, extra webhook configs or layout.

`Target { RoomName, EgressID string }`; EgressID `EG_` plus1–100 ASCII alnum,
underscore or hyphen. The controller must supply the trusted attempt's room/ID;
client-side syntax checks cannot prove ownership before a remote Stop call.

`Observation { EgressID, RoomName, Status string; StartedAtNS, UpdatedAtNS,
EndedAtNS int64 }` is the ONLY returned provider data. It contains no request,
URL, token, raw error, details or stream result. It describes transport, not
audience LIVE, billing closure, per-destination status or resource cleanup.
The later webhook/lifecycle component must supply those separate projections.

- `(*Client).Start(ctx, StartInput) (Observation,error)`: validate completely
  before network; one StartEgress POST with template.layout=`speaker`, preset
  `H264_720P_30` or `PORTRAIT_H264_720P_30`, outputs containing one stream
  `{protocol:"RTMP",urls:[...]}`. Match returned room to input; validate ID.
- `(*Client).Query(ctx, Target) (Observation,error)`: one ListEgress POST with
  room_name, egress_id, active=false. Require at most one exact room+ID match;
  duplicates, unrelated rows or nonempty next_page_token are not authoritative.
  Empty/omitted items is `ErrNotObserved`, NOT proof of absence or completion.
- `(*Client).Stop(ctx, Target) (Observation,error)`: one StopEgress POST with
  egress_id; require exact room and ID in response. Never infer ended from ACK.

JWT: mint per request with HS256; header alg/typ, iss=config.APIKey,
iat/nbf=current Unix seconds, exp=iat+60; video contains only roomRecord=true
and room=<validated room>. No token getter or token/credential output.
Config, Client and StartInput implement redacted String, GoString, MarshalJSON;
private wire structs are separate so redaction cannot corrupt actual requests.

## Failure and decoding contract

Static sentinel errors only: ErrInvalid, ErrUnknown, ErrUnavailable,
ErrNotObserved. Never wrap HTTP/provider/parser errors with attacker text.
Invalid config/input or nil client/context is ErrInvalid without I/O. Once a
Start/Stop RPC is attempted, any network, cancellation, redirect, non200,
malformed/oversized response or correlation failure is ErrUnknown, no partial
Observation and no retry. Query failures are ErrUnavailable; a valid empty
observation alone is ErrNotObserved. Pre-cancelled valid calls may return the
same conservative operation-specific failure without sending a request.

Response body ≤64KiB after decompression, valid UTF-8, exactly one JSON object,
no trailing value, duplicate object keys or >32 nesting levels. Ignore unknown
provider fields without returning them. Known proto fields accept snake_case or
lowerCamelCase, but both aliases together fail. Known enum accepts canonical
name or integer0..6; status omitted defaults to STARTING per proto3, null or
unknown values fail. int64 timestamp fields may be decimal strings or integer
JSON numbers, zero if omitted; reject null, negative, overflow/fraction/exponent.
All known fields must have exact scalar types, no coercion or partial DTO.

## Acceptance gates

- LKP01: capture exact Start/Query/Stop method/path/JSON/auth at local HTTP/TLS
  provider; verify JWT independently with stdlib HMAC and wall-clock lifetime,
  no extra grants; horizontal/portrait and both destinations; one call each.
- LKP02: every validation family rejects before I/O; constructor clones hosts;
  MOCK cannot silently use real network, LIVE cannot inject transport; trusted
  target requirement and all secret-bearing structs are documented.
- LKP03: exact correlation; enum/string-number/camelCase/default-zero parsing;
  malformed/type/duplicate/alias/depth/UTF8/oversize/trailing negatives; attacker
  secrets in provider errors/request/streams never appear in observation/error.
- LKP04: real canceled/hanging request, dropped reply, 3xx to second origin,
  4xx/5xx and malformed success yield conservative error; no automatic repeated
  Start or Stop. Query empty/not_found never claims stopped/no-effect.
- LKP05: independent tests and source/security review; root race/vet and existing
  core/live unit regressions, no unresolved P0/P1; preserve failing evidence.
- LKP06: dependency/caller notes, upgrade signal, root evidence, graph links and
  Humaux trace. No PG schema changes or new browser UI in this increment; real
  Cloud, duration/billing, credentials, media ingestion and G06 remain NOT_RUN.

## Integration stop line and upgrades

No production dispatcher route is registered. Later assembly must own tenant/
store/attempt entitlement, encrypted credentials/stream secrets, start outbox,
one-attempt rule, query-only UNKNOWN recovery, webhook history, and an explicit
revoked-binding resource-stop policy. Core binding configuration alone is never
enough. Historical LOCAL intents must remain permanently ineligible for later
route activation. A new authorized operation is required under that contract.

Room correlation and timeout do not manufacture exactly-once remote execution.
No automatic start retry, no stop against a merchant-provided ID, no conclusion
from an empty list. A self-hosted deployment, recordings, dynamic layouts or
per-stream projection needs a separately reviewed extension, not permissive
URL/JSON passthrough. Provider wire drift requires re-running LKP and actual
Cloud acceptance; unit fixtures alone cannot approve new protocol behavior.
