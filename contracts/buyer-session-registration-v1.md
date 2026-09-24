# Retry-safe private buyer session registration v1

Status: FROZEN after independent preflight of 55f5840 (no open P0/P1/P2). This is a
dependency of the public buyer BFF, not a public login or a browser release.
Implementation SR01–SR06: PASS_BOUNDED_PRIVATE_REGISTRATION at source `669e148`;
[real HTTP/PG and 361-test acceptance](../docs/implementation/2026-09-25-buyer-session-registration-acceptance.md).
Public browser bootstrap and automatic payment entry remain NOT_RUN.

## Reason and reuse

The existing private POST `/v1/buyer/session` generates a new random capability
and owner on each request. Its response can be lost after commit. A trusted BFF
that already possesses a cryptographically random capability needs a repeatable
registration operation, rather than another identity system or plaintext token
storage. Reuse `buyer.capability_sessions.token_hash` uniqueness, existing
owner/session/event tables, scope locks, expiry and published-origin admission.

## Frozen interface

- `buyer.Service.RegisterForTrustedStore(ctx, storeID, token) (Capability, error)`
  accepts only the existing canonical 32-byte raw-base64url token and UUID rules.
  The caller is a trusted server, never arbitrary browser JSON. It stores only
  SHA256(token), using the existing issuer pool and five-second deadline. Returned
  capability includes the supplied token for the internal caller only.
- New private `POST /v1/buyer/session/bootstrap` requires the existing independent
  BFF key, exact published origin and exactly one canonical Bearer token. Body
  must be `{}`; query, browser/scope headers and `Idempotency-Key` are rejected.
  Success is exactly `{authenticated:true,expires_at:<timestamp>}`, no token,
  owner, tenant or store identifier. Only POST is allowed. Common safe errors and
  ten-second HTTP deadline apply. Unlike old issuance, transient failures can be
  retried with the **same** token; never generate a replacement automatically.
- Existing POST `/v1/buyer/session` keeps its non-idempotent behavior and
  `retryable:false` errors. All other routes keep their current contract.
- New `buyer.register_capability(uuid,bytea,bigint)` is SECURITY DEFINER with
  fixed `pg_catalog` search path, owned by `commerce_buyer_writer`, PUBLIC
  EXECUTE revoked, issuer-only EXECUTE. No runtime/merchant table or function
  privilege is added, and no table/column/index/dependency is introduced.

## Transaction rules

1. Validate non-null store, 32-byte hash and TTL in the existing 60s..30d range.
2. If the hash is new, call existing `buyer.issue_capability` inside a PL/pgSQL
   exception subtransaction. A concurrent token-hash uniqueness conflict rolls
   back the losing owner/session/event before replay. Catch only the exact
   token-hash constraint, schema and table; unrelated uniqueness errors propagate
   unchanged. READ COMMITTED/default volatile SQL sees the committed winner after
   waiting. Stronger isolation failures fail safely, without an internal retry loop.
3. For an existing hash, require the exact same store and resolve via existing
   `buyer.resolve_scope`: active tenant/store/owner, live nonrevoked session and
   expiry evaluated after tenant -> store -> owner -> session locks. Return the
   original scope and original expiry. No TTL extension or second issued event.
4. Unknown/inactive store, cross-store hash reuse, revoked/expired capability and
   disabled owner deny without creating a replacement. Errors do not expose
   whether the token exists at another store. Concurrent rollback must leave no
   orphan owner; database uniqueness, not process memory, handles multiple API
   instances. An already admitted operation may finish before revoke commits;
   replay after revoke commits fails.
5. Exact published origin is resolved for every HTTP request, including replay.
   Preserve the accepted admission snapshot semantics: requests admitted before
   unpublish/rebind may finish within their deadline; later admission fails or
   resolves to the new store (where an old token cannot be registered).

## Explicit exclusions and next boundary

Registration is not browser bootstrap by itself. Public host-only Secure
HttpOnly cookies, origin/CSRF checks, per-scope rate limiting, initial cookie
delivery/loss, multi-tab initialization/reset/logout serialization and stale-tab
context protection still require a separate reviewed contract and real browser
gates. No implicit minting from SSR/GET, bearer token in browser JSON/storage,
browser-chosen deterministic login nonce, HMAC key lifecycle or identity/cart
merge is introduced here. A two-phase cookie alone does not resolve first-visit
multi-tab races. Do not expose the private API until that boundary is accepted.

## Acceptance gates

| Gate | Evidence required |
| --- | --- |
| SR01 | Real PG HTTP first registration, discard response, same-token retry preserves scope, expiry, cart and exactly one owner/session/event delta |
| SR02 | Two independent handler/service instances concurrently register one token; all results equal, one owner/session/event; separate tokens create separate owners. Also hold a winner's insert uncommitted, observe the loser waiting on its unique conflict, then commit and prove loser-owner rollback; goroutine timing alone is insufficient |
| SR03 | Expired/revoked/inactive tenant/store/owner and cross-store/hash reuse deny without new facts or expiry refresh; revoke race resolves with existing locks |
| SR04 | Invalid/null/hash/TTL SQL inputs, runtime/merchant/PUBLIC EXECUTE denied, issuer direct tables denied; canonical Go token validation |
| SR05 | Exact HTTP path/method/body/key/header checks; safe exact response; legacy issuance/error semantics unchanged; current published-origin checks enforced on replay |
| SR06 | Root targeted real-PG run and full race/vet regression exit 0; independent review; no customer/provider writes |

Integrator owns this contract, migration, real-PG gates and acceptance records.
Go author owns a new buyer registration Go file, handler wiring and focused unit
tests in an isolated worktree after freeze. No shared schema or dependency edits
by the author. Public browser and actual payment gates remain NOT_RUN.
