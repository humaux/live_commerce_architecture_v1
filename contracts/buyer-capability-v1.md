# Buyer cart capability v1 — internal authority kernel

Status: FROZEN interface for local implementation; NOT_PUBLIC, not checkout acceptance.
Baseline: `0ccd08a`. This is T03 buyer authority, a prerequisite for T10/T11.

## Boundary

Merchant memberships, anonymous storefront owners, verified customers, Meta
identities and platform support are different authorities. An anonymous cart
capability proves possession only, not a person's identity or email. It creates
no merchant principal/membership and performs no identity linking.

No HTTP route, cookie, cart, Quote, order, payment or customer production write is
introduced here. `IssueForTrustedStore` is a trusted internal issuer, NOT a public
store-ID endpoint. Before public mounting, a server-owned published-domain/store
resolver, CSRF/cookie policy, rate limiting and browser acceptance are required.
An active store is not necessarily published. Issuance retries create independent
anonymous owners; no automatic recovery/merge or exactly-once issuance is claimed.

## Frozen SQL / Go surface

- `commerce_buyer_runtime`: EXECUTE only `buyer.resolve_scope(bytea,uuid)`.
- `commerce_buyer_issuer`: EXECUTE only `buyer.issue_capability(uuid,bytea,bigint)`
  and `buyer.revoke_capability(bytea,uuid)`.
- `commerce_buyer_writer`: non-login, fixed SECURITY DEFINER function owner, no
  grants to either login authority. All functions pin `search_path=pg_catalog`
  and qualify relations; PUBLIC has no schema/function access.
- `platform.OpenBuyerPool(ctx,dsn)` and `OpenBuyerIssuerPool(ctx,dsn)` extend the
  existing startup role check. Runtime, merchant issuer, buyer runtime and buyer
  issuer membership must be mutually exclusive, including indirect membership.
- `internal/buyer.Scope { TenantID, StoreID, OwnerID, SessionID string }` is a
  different type from merchant `platform.Scope`.
- `buyer.New(issuerPool, ttl)` returns `(*Service,error)`; ttl must be a whole
  number of seconds between 60 seconds and 30 days. No hidden/default TTL.
- `Service.IssueForTrustedStore(ctx, storeID)` returns `(Capability,error)`.
  `Capability { Token string [json:"-"], Scope Scope, ExpiresAt time.Time }`.
  A new 32-byte cryptographically random, strict base64url token is minted in Go;
  only SHA-256 is persisted. Store ID must be a canonical lowercase UUID. Tenant,
  owner and session IDs are DB-derived, never caller-supplied.
- SQL issue returns `(tenant_id,store_id,owner_id,session_id,expires_at)`; it locks
  the tenant/store, checks active state, and atomically inserts owner, session and
  one issued event. Invalid input -> PT400; inactive/missing store -> PT401.
- `buyer.WithScope(ctx,buyerPool,token,storeID,fn)` accepts
  `fn(context.Context,pgx.Tx,buyer.Scope) error`. It validates input before SQL,
  resolves the opaque capability, and runs one bounded transaction. Resolve
  returns `(tenant_id,store_id,owner_id,session_id)` or zero rows. It locks the
  session/owner/store/tenant FOR SHARE, then checks active/revoked and DB clock
  expiry. A request authorized before revocation may finish; revocation waits
  for that transaction. New requests after revocation commits fail closed.
- Scope sets transaction-local `app.tenant_id`, `app.store_id`, `app.buyer_id`,
  `app.buyer_session_id`, and clears `app.principal_id`. Error, panic, timeout and
  cancellation roll back with an independent bounded cleanup context. The
  callback receives the bounded context. No buyer table DML/SELECT is granted to
  buyer runtime yet; owner RLS for future cart resources is a separate gate.
- `Service.Revoke(ctx,token,storeID) error` revokes only that hash/store, records
  one event, and is idempotent/no-op for unknown, expired or already-revoked
  well-formed credentials. Wrong store cannot revoke another store's capability.
- `buyer.ErrInvalid` and `buyer.ErrUnauthorized` are stable sentinel errors.
  Do not expose token, hash, DSN, DB details or internal IDs in public errors.

## Persistence

Migration `0006_buyer_capability.sql` only, no edits to earlier checksums.
`buyer.owners` binds `(tenant_id,store_id,id)` to the real store; `active` can
disable an owner. `buyer.capability_sessions` binds its tenant/store/owner via
composite FK, with unique 32-byte token hash, expiry and revocation.
`buyer.capability_events` records only issued/revoked, unique per session/action,
with composite FK preserving the exact owner/store. No raw tokens or PII.
All private buyer tables use FORCE RLS and only the non-login writer's explicit
policies. No runtime/issuer table DML, no UPDATE/DELETE event grants.

## Acceptance gate (real isolated PostgreSQL, not model-only)

1. Valid issue/resolve/revoke; hash-only storage; no merchant membership created;
   distinct repeated issuance; single event for repeated revoke.
2. Wrong tenant/store, another owner's token, merchant token, malformed token,
   inactive tenant/store/owner, expired/revoked session all fail without callback.
   Another owner's valid capability resolves only its own scope, never caller IDs.
3. Buyer runtime cannot mint/revoke, read credential tables, mutate merchant data
   or use merchant resolver. Issuer cannot read tables/use merchant functions.
   Merchant roles cannot execute buyer functions. Mixed/privileged/owner logins
   are rejected at startup by all pool constructors.
4. FK violations fail; no raw tokens in audit/JSON/errors. Issuance event failure
   rolls back owner/session. Transaction callback error/panic/cancel rolls back;
   no scope survives connection reuse.
5. Concurrent resolve/revoke ordering, expiry checked after lock waits, and
   inactive-state changes cannot race issuance/resolution.
6. Root replays full existing Go/PG race suite and vet; independent P0/P1 review
   must close before merge. No public checkout, real PSP or G03/G05 PASS claim.
