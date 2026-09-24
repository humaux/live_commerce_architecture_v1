# Account credential runtime configuration

Scope: internal merchant account intake, not permission to collect money or ship.
The HTTP contract is [merchant-account-http-v1](../../contracts/merchant-account-http-v1.md).
No new dependency or credential service is required for this slice.

## Composition and authority

`cmd/api/main.go` loads identity configuration first, then account configuration,
opens the existing restricted runtime pool, constructs the account service and
injects it through `httpapi.Options`. `cmd/api/accounts.go` uses the existing
`core.Service` and an **insert-only** River client; it never starts a worker.
`accounts.Service` encrypts before writing SQL. The Next store BFF forwards only
the server-side session bearer, scoped target, JSON and command key.

Account routes require identity mode; the legacy shared development fixture
cannot accept credentials. The private Go listener must be a literal loopback
address. The public BFF requires HTTPS except explicitly enabled loopback tests.
Newly onboarded owners receive `integration:read/manage` through migration 0019;
they do not receive `integration:execute`. Existing members are **not** backfilled.
If an existing account lacks a grant, use an authorized role-management workflow;
do not restore access by replaying onboarding or editing the browser token.

## Environment inputs

|Name|Required contract|
|---|---|
|`COMMERCE_ACCOUNTS_ENABLED`|Default disabled; strict existing flag parser. Disabled reads no account secret inputs|
|`COMMERCE_ACCOUNT_ACTIVE_KEY_ID`|Exact active encryption-key ID, matching one supplied entry|
|`COMMERCE_ACCOUNT_KEYS_JSON`|JSON array of 1–16 unique `{id,key_base64}` objects; no unknown/trailing values; canonical standard base64 encoding of 32-byte keys|
|`COMMERCE_ACCOUNT_REPLAY_KEY`|Separate canonical-base64 32-byte HMAC key; must differ from every encryption key|

These are platform encryption/replay keys, **not** a merchant's PAYUNi HashKey or
HashIV. Provision them through a protected deployment environment/secret manager,
never checked-in files, shell arguments, screenshots or logs. Startup rejects
missing or malformed inputs without printing the values. There is no random-key
or plaintext fallback in production. Local tests use unrelated synthetic keys.

## Rotation and recovery

1. Back up encrypted records together with recoverable encryption/replay key
   custody under the deployment operator's authorized policy before production changes.
2. Add the new encryption key to the keyring, retaining all referenced historical
   keys. Change only the active encryption-key ID for new writes.
3. Keep the replay key stable: changing it invalidates permanent command identity
   and requires a separately designed migration, not a normal key rotation.
4. Merchant secret rotation creates a new immutable credential version. It does
   not change provider/account/environment or reopen a disabled method/binding.
5. After a lost network response, read safe metadata before any retry. Reuse the
   same nonsecret command key and require explicit credential re-entry when needed;
   never persist secret bodies in browser storage or an automatic retry journal.

There is deliberately no public decrypt/export/delete/verify endpoint. Do not
remove old key material based solely on the active head: historical payment
operations can reference old credential versions.

## Release checks and remaining deployment work

- Run the full isolated PG/race/vet suite and signed-MOCK-IdP browser chain; inspect
  safe metadata, encrypted versions, CAS/replay and permission revocation evidence.
- The environment-to-factory smoke proves constructor wiring without connecting
  a database or starting workers. A separate browser-tag gate builds the actual
  `cmd/api` executable, starts it from a private environment, saves to isolated PG,
  requires clean SIGTERM exit, then starts a different PID with the same PG/keys.
  It verifies read/list/replay/rotation and independently decrypts both persisted
  versions. This proves local process assembly and restart, **not production
  deployment, real IdP, real provider credentials or production key recovery**.
  See [the dated evidence](2026-09-24-merchant-account-http-acceptance.md).
- A bounded per-process tenant/store limit (60 credential writes/minute, 4096
  active scopes) is not a distributed quota. Add deployment-level anti-abuse,
  body/header limits, TLS, access controls and secret-safe request logging.
- A saved account reports `CONFIGURED_UNVERIFIED`. Provider qualification,
  merchant method enablement, buyer visibility and runtime availability remain
  separate. No sandbox/live provider gate is satisfied by saving configuration.

No production service was restarted or reconfigured by this implementation.
