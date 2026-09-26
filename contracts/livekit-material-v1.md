# LiveKit sealed media material v1 — LKM01–05

Status: FROZEN_FOR_IMPLEMENTATION after independent preflight (no open P0/P1).
Required custody dependency of the durable
media controller, not an entitlement, dispatch job, database resolver or new vault
framework. It prevents destination stream keys from entering operation JSON and
keeps MOCK material unusable with a LIVE client. Only local deterministic/crypto
tests may run in this slice; no provider call or production configuration.

## Scope and minimal reuse

Implement `internal/integrations/livekit/material.go` in the existing package.
Reuse `Client.ready`, `Client.validStart`, existing redacted `StartInput`, the
standard library AES-256-GCM and the established copied-key pattern in
`accounts/crypto.go`. Do not import PAYUNi credentials, read environment/files,
create a generic credential framework, expose project API credentials or add a
module dependency. The caller/SQL resolver remains responsible for authorization,
the authenticity of destination material and exact trusted project credentials.

## Frozen candidate Go surface

```go
var ErrMaterial = errors.New("livekit: material unavailable")
type MaterialScope struct {
    TenantID, StoreID, SessionID, AttemptID, ProjectID string
    CredentialVersion, MaterialVersion int64
}
type SealedMaterial struct { KeyID string; Nonce, Ciphertext []byte }
type MaterialKeyring struct { /* private copied key material */ }
func NewMaterialKeyring(activeID string, keys map[string][]byte) (*MaterialKeyring,error)
func (*MaterialKeyring) Seal(scope MaterialScope, client *Client, in StartInput) (SealedMaterial,error)
func (*MaterialKeyring) Open(scope MaterialScope, client *Client, sealed SealedMaterial) (StartInput,error)
```

All failures return exactly ErrMaterial and a zero output (nil keyring for New).
No underlying error, panic, URL, key, ciphertext or validation reason escapes.
All three operations perform zero network I/O. Public scope contains identifiers
only; MaterialKeyring and SealedMaterial implement redacted String/GoString/
MarshalJSON, matching existing secret-bearing types. Explicit fields of the
envelope remain available to the persistence layer; MarshalJSON is deliberately
not a persistence format. No public accessor returns encryption keys.

## Validation, encryption and wire details

- Keyring accepts 1–16 key IDs, each `[A-Za-z0-9_-]{1,64}`, each exactly 32 bytes,
  with activeID present. Own copies of the map and every key; reject nil/empty,
  missing active key or invalid member. A nil or zero-value keyring must return
  ErrMaterial from Seal/Open. Concurrent calls on a constructed ring are safe.
- Scope tenant/store/session/attempt are canonical lowercase UUID strings
  `8-4-4-4-12` hex (not the all-zero UUID). ProjectID is `[A-Za-z0-9_-]{1,80}`;
  both versions are positive. Scope is required on every call, including Open.
- Client must be initialized. Room is derived from the attempt as `lc_` plus its
  UUID without hyphens; input must match it exactly. Use existing validStart for
  aspect, destination-count, duplicate URLs, URL bounds and trusted host allowlist.
  Never infer tenant or project from ciphertext, room syntax or an arbitrary URL.
- Private plaintext JSON has exactly `room_name`, `aspect_ratio`, `stream_urls`.
  Do not json.Marshal StartInput itself: its public marshaler is redacted. Bound
  plaintext to 16 KiB; use a private wire struct and copy slices on return.
- AES-256-GCM with standard 12-byte random nonce from crypto/rand, fresh per Seal.
  Ciphertext includes the 16-byte tag. Envelope format 1 is bound in AAD; AAD also
  contains domain `livecommerce.livekit.media`, key ID, every scope field,
  client environment and canonical Cloud endpoint (strip the optional final `/`).
  AAD must be deterministic, using a fixed struct serialized as JSON (not map).
  Exact JSON field order/names: `domain`, `format`, `key_id`, `tenant_id`,
  `store_id`, `session_id`, `attempt_id`, `project_id`, `credential_version`,
  `material_version`, `environment`, `endpoint`. `format` and both versions are
  JSON integers; all other fields are strings. `format` is exactly 1. Standard
  encoding/json compact output with its default escaping is the AAD wire format.
  A different tenant/store/session/attempt/project/version, MOCK/LIVE environment,
  endpoint or key ID must fail authentication. No project API secret is included
  in plaintext/envelope; the future resolver pins credentials by trusted version.
- Open accepts a known key ID, exactly 12 nonce bytes and ciphertext length
  16..16400 before allocating/decrypting. Authenticate first; then require the
  exact private JSON object shape (all 3 fields, no unknown fields, duplicates,
  aliases, wrong types, trailing value or malformed UTF-8). Reuse decodeObject's
  strict duplicate/depth checks, then check exact fields and scalar/array types.
  Validate decoded StartInput again against scope and client's current allowlist.
  Authentication does not make malformed or wrongly scoped plaintext valid.
- Key rotation: a new ring with new active key plus retained old key opens the old
  envelope; new Seal uses the new ID. Removing an old key makes it unavailable.
  Adding a same-byte alias ID must not allow swapping envelope KeyID (AAD binds it).
- No caches, background jobs, side effects, implicit authority or credential
  lookup. It is valid to store ciphertext longer than the authorization lifetime;
  the future SQL resolver must revalidate dispatch/cleanup eligibility and lease.

## Acceptance gates

|Gate|Required evidence|
|---|---|
|LKM01|Two URL/aspect round trips, attempt-derived room, fresh random nonce and ciphertext, exact zero network calls, standard AES-GCM interoperability with independently built AAD/plaintext|
|LKM02|Every scope field/environment/endpoint/version/key-ID swap, wrong key, nonce/tag/ciphertext mutation, unknown/retired key fails with exact static error and zero output|
|LKM03|Authenticated malformed plaintext (duplicate/unknown/missing/wrong-type/trailing/UTF-8) fails; valid AES-GCM ciphertext for oversized plaintext is rejected by the pre-decrypt 16400-byte envelope cap; invalid room/aspect/URL/allowlist cannot be sealed or opened|
|LKM04|Nil/zero/invalid keyring/client, constructor map/key mutation, returned-envelope/input slice ownership, parallel/race use, key rotation and redacted fmt/JSON with canaries|
|LKM05|Independent author/tests, root race/vet and existing LKP regression, source/contract/dependency/memory trace; no claim of durable ownership or real Cloud/G06|

Durable continuation: trusted registrar/custody persistence and private leased
resolver; typed media actor/worker with direct SQL role isolation; current Start
eligibility versus frozen resource cleanup; then MLA01–08 and real browser/media
acceptance. Sealing a value by itself never makes it authorized for dispatch.
