# Public buyer browser transport v1

Status: DRAFT; independent protocol review before implementation. Builds on
the accepted private buyer HTTP, published-origin and session-registration
contracts. No buyer visual page is approved by this transport document.

## Deployment and authority

- New `apps/storefront` uses the existing Next/React/TypeScript versions, no new
  provider/client framework. Only route handlers and browser coordination code
  are implemented before the user's visual-comp approval. Build and real browser
  tests use production output, not a mocked Next router.
- Default disabled. `COMMERCE_BUYER_WEB_ENABLED=1` requires fixed private API
  origin (literal loopback HTTP or canonical HTTPS), independent canonical
  32-byte BFF key and separate cookie-signing key, plus integer cookie TTL
  60..2592000 seconds. Signing-key changes invalidate old envelopes; no silent
  fallback key/remint. All replicas must share the configured key and TTL.
- Request Host is only an untrusted canonical origin candidate, never tenant
  authority. Construct HTTPS origin, reject ambiguous/duplicate/comma/encoded
  host, noncanonical case/ports/addresses. Every upstream data operation passes
  this candidate to existing Go published-origin resolution; never cache that
  decision or accept browser tenant/store IDs as authority. Ignore Forwarded
  and X-Forwarded-* for authority. API destination is fixed, not Host-derived.
- Test-only explicit loopback mode may use HTTP localhost/127.0.0.1 with a fixed
  synthetic published-origin mapping supplied by test configuration; production
  must reject this mapping. This is fixture authority, not DNS/TLS acceptance.
- All public responses no-store/nosniff, no CORS, no leaked backend headers,
  cookies, tokens or diagnostics. Fixed safe JSON error envelopes. Body 64KiB,
  upstream response 1MiB, bounded deadlines and no automatic fetch redirects.

## Cookie and request context

- One distinct `__Host-commerce_buyer` Secure/HttpOnly/SameSite=Lax/Path=/ cookie,
  no Domain. Bounded canonical HMAC envelope binds version, random256-bit bearer,
  issued-at, absolute expiry and origin. Cookie provenance is not store authority.
  Duplicate/malformed/forged/expired cookies never silently mint a replacement.
- GET status never mints, sets cookies or writes. Returns absent/expired/inactive/
  active plus a non-bearer HMAC context bound to the exact cookie+origin when
  structurally valid. Private GET session distinguishes active from inactive;
  inactive alone does not claim unknown/revoked/expired database cause.
- Mutations require exact same Origin, application/json, bounded strict JSON,
  and matching session-bound `X-Buyer-Context` (also CSRF proof), except first
  prepare with no cookie. A stale tab context fails before any upstream write;
  it is never refreshed transparently while retaining an old cart/order draft.
- Prepare is the ONLY response that sets the cookie. Registration, data calls,
  errors, status and logout never Set-Cookie. Initial prepare requires absent
  cookie. Explicit reset of a valid old envelope first retires the same token
  successfully, then prepares a replacement; failures retain the old cookie.
  Expired envelopes can be explicitly reset, not used for commerce operations.
- Public activation consumes cookie token and calls private bootstrap with `{}`;
  it never returns token to browser JSON/HTML/URL/storage. Response context stays
  stable; effective expiry cannot exceed cookie or returned database expiry.

## Durable retirement and admission

- Existing private DELETE unknown-token204 is NOT retirement proof. Add private
  POST `/v1/buyer/session/retire` (BFF key, published origin, Bearer, `{}`, no
  Idempotency-Key) and issuer service `RetireForTrustedStore(ctx,storeID,token)`.
- Fixed issuer-only SQL `buyer.retire_capability(store,hash,ttl)` serializes
  retirement of the same hash with a transaction advisory lock. If an exact
  store/hash row exists, invoke existing revoke. Otherwise register the same hash
  and revoke it in the SAME transaction. If a concurrent winner was already
  retired/expired, accept only an exact same-store row after the registration
  denial; no cross-store or inactive unknown-token success. A late registration
  must see the retained revoked/expired row and cannot resurrect it. No new table
  or plaintext token storage. Existing DELETE semantics remain unchanged.
- Successful retirement returns 204; same-token retry is safe. Public logout
  calls retirement and leaves the now-inert cookie untouched. Explicit reset
  retires first as above, so abandoned activation cannot undo logout/reset.
- Public new-session registration uses one shared database fixed-minute quota:
  600 new capability rows per store per UTC minute across API instances; same-hash
  replay and existing-row retirement do not consume quota. Add a store/created-at
  index and a small issuer-only wrapper around existing registration, serialized
  by store advisory lock. No extra quota table or process-global shared budget.
  The value is an initial admission ceiling, not a proven production capacity;
  ingress per-client and volumetric controls remain a deployment gate. Reject
  quota with safe429. Private legacy issue is not exposed by the public BFF.

## Browser coordination

- Use one same-origin Web Lock for prepare/activate/reset/logout. Read status
  INSIDE the lock. No implicit SSR/GET mint, lock stealing, keepalive/sendBeacon,
  or Promise.race releasing the lock while a Cookie-changing fetch remains live.
- Before prepare/reset fetch, persist a NONSECRET localStorage pending journal:
  version, random operation ID, baseline cookie context (null for initial), and
  phase. No bearer/auth token in storage and no HMAC derivation from that ID.
  Web Locks/storage unavailable -> fail closed before network mutation.
- On fulfilled response, re-read status under lock and clear the marker only
  when the new valid context differs from baseline. On network uncertainty,
  abort or tab death, keep marker. Later tabs may only query status: valid new
  context proves the sole outstanding prepare's cookie arrived; same/absent
  context means interrupted initialization, NOT permission for a second prepare.
  No timeout is considered proof of network quiescence. Exceptional unrecoverable
  cases explain using a fresh isolated browser context, not silent cart reset.
- Activation/retirement have no cookie writes and are retried with the same
  context/token. Never silently reuse old quote/order/idempotency drafts after a
  context change. The journal is coordination only, never server authorization.

## Commerce proxy and acceptance

Exact allowlist exposes the existing catalog/options, cart, quotes, destination,
checkout and order methods under `/api/buyer/`; no generic URL proxy, legacy issue,
provider operations or merchant authority. Forward only controlled BFF key,
published-origin candidate, cookie-derived Bearer, required Idempotency-Key,
JSON body and the existing exact catalog/options query forms. Public route/method/
UUID/query and context checks happen before upstream calls; private domain rules
remain authoritative. Successful data uses already-safe private projections.

Gate: actual Next production build/typecheck; real Chromium two tabs + dropped/
reordered/aborted/closed prepare and stale-context writes; token absent from JS;
real PG+HTTP+Next cart/quote/home destination/order/readback, two BFF instances,
registration loss/restart and zero duplicate purchase facts; deterministic late
activation versus retirement; shared quota across stores/instances; forged cookie,
cross-origin/host/header/body injection and disabled/misconfigured startup; full
backend race/vet regression and independent source/evidence review. No success
claim for buyer UI, real domain hosting, CVS source, PSP or full SaaS release.

Root owns contract, migration, Go retirement/quota wiring, package/lockfile and
independent real-PG/browser fixture gates. Isolated author owns Next server and
coordinator files only after contract freeze. UI comp work is independent.
