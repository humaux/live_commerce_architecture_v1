# LiveKit browser input wire profile v1 — LKI01–05

Status: **FROZEN_FOR_PROTOCOL_IMPLEMENTATION**. Base `3d72d58`; independent
preflight of `84280e9` found no confirmed P0/P1 (Humaux title: LiveKit input wire
LKI01–05 independent preflight). No implementation or test pass is claimed.
This defines only
the provider boundary needed by [browser input lifetime](live-browser-input-v1.md),
not merchant authorization, PG custody, HTTP routes, runtime configuration, Cloud
qualification or Studio. No production call or new dependency is part of this slice.

## Reuse and fixed scope

Extend `internal/integrations/livekit.Client`; preserve `New`, Config validation,
nonempty StreamHosts, LIVE exact Cloud HTTPS host, no injected LIVE transport,
MOCK explicit test transport, private no-proxy/no-redirect client and 10s timeout.
This is the same output-qualified project/attempt client, not a second room-only
configuration. Do not add public generic RPC/grant helpers or a server SDK.
Sharing private bounded HTTP exchange and HS256 signing code is permitted if
all old Egress wire/secret/error tests stay unchanged and pass.

Primary sources, checked 2026-09-27:
[RoomService](https://docs.livekit.io/reference/other/roomservice-api/),
[tokens](https://docs.livekit.io/frontends/reference/tokens-grants/),
[explicit revocation](https://docs.livekit.io/intro/basics/rooms-participants-tracks/participants/).
RoomService POST paths are `/twirp/livekit.RoomService/<fixed method>`. No endpoint
is supplied by a merchant. Requests below contain only the listed fields.

## Frozen Go surface

```go
type PublisherGrant struct {
    RoomName, Identity string
    IssuedAt, ExpiresAt int64 // Unix seconds, frozen by the future PG authority
}
type PublisherToken struct { /* unexported secret */ }
func (PublisherToken) Bearer() string
func (*Client) MintPublisher(PublisherGrant) (PublisherToken, error)

type InputTarget struct { RoomName, Identity string }
type InputObservation struct {
    RoomName, Identity, ParticipantID, State string
    CameraPublished, CameraMuted bool
    MicrophonePublished, MicrophoneMuted bool
}
type InputRoomObservation struct { RoomName, RoomID string }
func (*Client) ObserveInput(context.Context, InputTarget) (InputObservation, error)
func (*Client) RemoveInput(context.Context, InputTarget, int64) error // explicit cutoff
func (*Client) DeleteInputRoom(context.Context, string) error
func (*Client) ObserveInputRoom(context.Context, string) (InputRoomObservation, error)
```

All callers must load scope and identities from the trusted persisted attempt.
Syntactic validation is NOT merchant permission, session validity, ownership,
revocation, start eligibility or evidence of a LIVE project. No package env/file
loading, tokens in errors, retries or automatic room creation API.

`RoomName`: existing `lc_` plus32 lowercase hex. `Identity`: `lcp_` plus32 lowercase
hex, generated once by the future server authority, never merchant-selected.
Nil/uninitialized client, nil context, malformed target/time rejects with
`ErrInvalid` before network/signing. Errors are existing static errors only.

## Publisher signing

`MintPublisher` does no network I/O. Require 0 < IssuedAt <= current Unix time,
ExpiresAt > current Unix time, and 0 < ExpiresAt-IssuedAt <= 60, with overflow-safe
checks. Fixed input produces the same token while still valid; there is no new
timestamp or deadline on replay. Expired/future/overflow/overlong input fails.

JWT header exactly `alg=HS256,typ=JWT`; claims exactly `iss`, `sub`, `iat`, `nbf`,
`exp`, `video`. `iss=config.APIKey`, `sub=Identity`, `iat=nbf=IssuedAt`, `exp=ExpiresAt`.
Video exactly: room=RoomName, roomJoin=true, canPublish=true,
canPublishSources=["camera","microphone"], canSubscribe=false,
canPublishData=false, canUpdateOwnMetadata=false. No other grants or metadata.

PublisherToken `String`, `GoString` and `MarshalJSON` are constant redacted output.
The only deliberate secret exit is `Bearer()` for an explicit future HTTPS DTO;
zero value returns empty. No accidental credential getter is added to Client.
The token cannot prove actual physical camera/microphone provenance.

## Room management signing and observation

Management JWTs are internal, not PublisherTokens: same existing HS256 header and
issuer/iat/nbf/iat+60 shape, no sub. Only the listed video grant plus exact room:

|Method|POST JSON|Management grant|Result|
|---|---|---|---|
|ObserveInput → GetParticipant|room, identity|roomAdmin=true|Exact participant projection below|
|RemoveInput → RemoveParticipant|room, identity, revoke_token_ts|roomAdmin=true|nil acknowledges only valid HTTP200 empty JSON object|
|DeleteInputRoom → DeleteRoom|room|roomCreate=true|nil acknowledges only valid HTTP200 empty JSON object|
|ObserveInputRoom → ListRooms|names=[room]|roomList=true|Exact room projection or ErrNotObserved on empty list|

Use one grant only, never roomRecord or roomJoin for management. Provider grants
for list/create may be project-wide despite a room claim: **server-owned target
validation and fixed body are still required**, not proof of provider room scoping.
`revoke_token_ts` is serialized as a proto int64 decimal string. Require positive,
not in the future and no more than30 seconds before the client clock at call
validation. This avoids sending a cached cutoff; server clock skew can still fail.
The provider's same-second/clock/missing-room limits are NOT solved here.

Strict JSON uses the existing <=64KiB decompressed, UTF-8, single object, no
duplicates (including ignored nested fields), <=32 depth parser. No raw body,
metadata, participant name, TURN secret, error or token escapes the package.

ObserveInput: require identity exactly equals requested identity; valid `PA_`
SID (1–100 ASCII alnum/underscore/hyphen after prefix). State accepts canonical
JOINING/JOINED/ACTIVE/DISCONNECTED or integer0..3, omitted defaults JOINING;
null/unknown/stringified numeric fails. Tracks omitted or [] means none; null or
nonarray fails. At most two tracks, each unique `TR_` SID (same suffix grammar),
unique source CAMERA or MICROPHONE, and matching VIDEO or AUDIO type. Accept
canonical enum names or numbers (CAMERA1/MICROPHONE2; AUDIO0/VIDEO1). Omitted type
defaults AUDIO per proto3; omitted/unknown source cannot establish the declared
source and fails this bounded profile. Muted is bool, defaults false, null fails.
Return published and muted independently; muted is not absence, and this adapter
does not make the product's readiness decision or forcibly unmute anyone.
Unknown fields are ignored after strict structural parsing. Metadata is not a
media measurement: even ACTIVE+tracks is not decoded frames/audio energy.

ObserveInputRoom: `rooms` omitted or [] yields ErrNotObserved; null/nonarray fails.
Exactly one row otherwise, its name must equal requested room and SID valid
`RM_` (same suffix grammar). Multiple/duplicate/unrelated rows fail. Return only
name/SID. This reports absence **at query time**, not durable revocation, absence
of hidden participants in another room, Egress terminal or resource lifetime.

All non200 responses (including not_found), cancelled/failed transports,
redirects, malformed/oversized/ambiguous results return ErrUnavailable for reads,
ErrUnknown for remove/delete. Nil error for a destructive RPC is ACK only, not
proof cached tokens cannot rejoin or total resource closure. No retry; higher
controller must reserve/fence and decide bounded exact-target recovery.

## Independent acceptance and stop line

- LKI01: independently verify publisher HS256/exact claims, deterministic fixed
  valid input, expired/future/overflow/TTL negatives, nil client, zero value and
  formatting/JSON redaction. Signing produces zero network calls.
- LKI02: real local TLS double captures exact four RPC paths/JSON/management JWT
  and one-call count. Check target syntax/cutoff validation before I/O; never
  relax New's existing transport/config invariants to make the test pass.
- LKI03: exact participant/room correlation, enums/defaults/mute/source/type,
  arrays and max counts, strict JSON/duplicates/UTF8/oversize/gzip, safe unknown
  fields; all failure outputs zero and errors never echo supplied secret text.
- LKI04: real drop/hang/context cancellation, redirect to another origin and
  4xx/5xx/malformed200; no automatic repeat, no token to redirected destination,
  and not_found never quietly counted as revoke success. Retain failing evidence.
- LKI05: independent review, source + independent tests in separate worktrees;
  root `go test -race ./internal/integrations/livekit` and `go vet` plus impacted
  Egress/live regressions. Call/dependency notes and Humaux rationale. No PG,
  browser, real SFU or real Cloud claim follows from a TLS-double pass.

Before any product route: implement the parent authority/lifetime contract and
BRI gates. Explicit-cutoff ACK + empty room is insufficient without qualified
Cloud cached-token rejoin tests. Upgrades expanding enum/source/track limits or
changing provider grants require source review and these gates again, not a
silent liberal decoder. No ListParticipants, UpdateParticipant, CreateRoom,
generic token endpoint, cohost/viewer grant or public configuration surface here.
