# T03 merchant identity — bounded backend contract

Status: implementation contract, not a claim of production IdP or browser-login acceptance.

## Authority and scope

- A configured, verified OIDC provider authenticates `(issuer, subject)`. Email, Meta OAuth, browser tenant headers and fixture bearer tokens never create or merge merchant identity.
- `commerce_identity` is a dedicated trusted authentication-service group, not a schema owner, superuser, RLS bypass role or business API role. Its pool must never be passed to domain handlers. It has only schema USAGE and EXECUTE on fixed login/bootstrap/logout functions, not direct table access. A separate non-login `commerce_identity_writer` owns those SECURITY DEFINER functions with fixed `pg_catalog` search paths and PUBLIC execution revoked. No function takes an arbitrary principal/tenant/permission target. A credential compromise can still impersonate a provider subject (this is an identity authority); it cannot directly mutate arbitrary tenant permissions or forge arbitrary audit rows. Production must provision a distinct login/secret and endpoint rate limits.
- Existing `commerce_auth` remains a read-only SECURITY DEFINER owner. Existing `commerce_runtime` cannot read identity mappings/flows or write sessions, memberships or grants. Both pool constructors reject cross-membership and privilege escalation.
- This slice handles only merchant identity. Buyer, support, platform audiences remain separate and are not silently enabled.

## Login state machine

1. Start generates independent 256-bit state, browser binding, nonce and PKCE verifier. A five-minute flow stores state/binding hashes, nonce/verifier and a deployment-controlled provider key. Browser binding is destined for a Secure/HttpOnly, SameSite=Lax cookie, not client storage or a URL.
2. Callback requires exact state plus binding, the same configured provider key, a nonempty code and an unexpired flow. Atomic deletion is committed **before** exchanging the code. Failed or ambiguous exchange/commit requires restarting login; no consumed code is retried.
3. OIDC verifies signature, issuer, audience, expiry, nonce, subject and PKCE. Callback/issuer are HTTPS except explicit loopback test mode. Redirect URI is fixed in configuration, never taken from a callback parameter.
4. Mapping + active principal + opaque merchant session + session-issued audit share one transaction. Only token hashes persist; provider access/refresh/ID tokens are not retained. Session TTL is explicit policy (5 minutes–24 hours), not inferred from provider tokens.
5. Logout revokes the one current token and appends one audit record atomically. Repeated logout is harmless. No session or customer data is deleted.

## First-store transaction

- Onboarding is disabled unless explicitly enabled by deployment policy. Supported currencies must be supplied by policy; this implementation does not decide market, tax, plan, payment provider or logistics carrier.
- Inputs: idempotency key (8–128 ASCII safe characters), tenant/store/initial warehouse names (1–120 Unicode characters), configured allowed currency. No tenant ID, principal ID, owner permissions or token duration are accepted from the client.
- A valid, active, unrevoked merchant session is required. Lock session and principal, then create tenant, store, membership, the fixed existing owner permission set, the user-named initial warehouse, onboarding receipt and `merchant.store_created` audit in one transaction.
- One initial-store receipt per principal. Same key + canonical payload returns the original IDs; changed key or payload conflicts. Concurrent requests serialize on the principal. Failed transactions leave no partial tenant/store/grants/warehouse/audit.
- This is initial-store creation only, not arbitrary extra stores, invitations, ownership transfers or merchant lifecycle management.

## Store-domain handle amendment (owner ruling 2026-10-03)

- Migration 0106 is unpublished and defines the automatic handle as a random eight-digit number matching `^[1-9][0-9]{7}$`, independent of the display name and UUID. It is a public address identifier, not an authentication secret or an authorization boundary.
- Both the new-store trigger and existing-store backfill call `control.assign_store_handle(text,uuid)`. Its legacy internal arguments remain for compatibility but no longer influence the number. Name-to-slug generation is removed.
- Allocation tries at most 50 candidates, excluding reserved/invalid handles, existing store handles and handles still claimed by a serving platform domain (`store_handle_taken`). A transaction-scoped candidate lock serializes automatic assignment; the unique handle index remains the concurrent uniqueness backstop. Exhaustion raises `PT409` and rolls back creation.
- The suggestion SQL function, Go endpoint `/v1/identity/handle-suggest`, BFF endpoint `/api/onboarding/handle-suggest`, and frontend client are removed. Registration displays the localized automatic-assignment explanation, with no address preview. After success it displays the actual `handle` and `storefront_origin` returned by the server. The configured base domain, not client input, determines the address (production: `https://<number>.xgdwm.com`).
- `store-admin handle-set` remains an optional operator tool with its existing grammar, reservation, occupancy and publication restrictions. Custom domains, Caddy, TLS admission and domain verification are unchanged; replayed receipts may reflect an operator-assigned handle.
- Acceptance: 50 concurrent creations produce distinct eight-digit numbers; pre-0106 stores receive numeric backfill; occupied handles and still-serving platform domains are skipped; 50 occupied candidates raise `PT409`; the retired Go route returns 404/405 and its BFF file is absent; real-click onboarding displays the allocated address. Migration changes require full G07 in addition to focused and browser gates.

## Acceptance and stop lines

Required: signed mock IdP positive and negative tests; real PG18 single-use flow and concurrency tests; active/revoked/expired/audience session checks; literal identity mapping (no email merge); idempotent/concurrent onboarding and failure rollback; cross-tenant 401/403/404 and runtime/auth role separation. Root replays tests independently.

Still required before T03/global G01/G02/G11 PASS: approved login/onboarding visual comps; production BFF cookie/CSRF integration; real browser login→store→ledger→logout; chosen IdP/registration policy, deployment secret provisioning, rate limiting and lifecycle/retention operations. Local signed mock evidence must remain labelled `PROVIDER_MOCK`.

## Dependencies and simplicity decision

Reuse PostgreSQL transactions/constraints and the existing merchant session resolver. Use maintained `coreos/go-oidc/v3` for JOSE/OIDC and `x/oauth2` for code/PKCE exchange; do not write a JWT verifier or password store. No extra cache/session service is introduced. The provider interface is a testing seam for signed mock and failure injection, not a pluggable marketplace.

## Amendment W6-01B: permission `customers:write` (migration 0139)

`identity.store_grants.permission` gains `customers:write` (add/rename/delete customer tags, set a customer's tags, add and
edit notes; see `contracts/customers-billing-v1.md` Amendment W6-01B). The owner and admin bundles (derived from the live
permission catalogue) hold it; existing owner/admin staff are backfilled; viewer, live_operator and fulfilment do not.

Status of the role matrix (integrator note, W6-01B review): the unit brief listed an "owner/admin + 客服包" bundle, but no customer-service role exists
yet, so `customers:write` is owner/admin only for now. The note rule "author, or a `customers:privacy` holder, may edit/delete a note" is reserved for the
day a non-privacy role (e.g. a customer-service bundle) receives `customers:write`; until then every writer is also a privacy holder.

## Amendment OPS-01B: suspended store or tenant (migration 0143)
`identity.resolve_access` is unchanged: it already joins `control.stores.active` and `control.tenants.active`, so a suspended store or tenant yields the same "no access" outcome as an unknown scope (the routes' existing refusal; there is no distinct `store_suspended` code, and none is planned for v1). Suspension and resume are performed only by the platform operator CLI, see `contracts/platform-operator-v1.md`.

## Amendment OPS-02B: platform support grant (migration 0153)
`identity.resolve_access` (signature, owner `commerce_auth`, EXECUTE unchanged) gains one branch after the regular lookup finds nothing: the principal has **no** `store_grants` row for the store and holds a live `identity.support_grants` row (unrevoked, `expires_at > clock_timestamp()`, store and tenant active) whose `permissions` include the requested `:read` permission. It returns `ok` with `authz_revision = -1`. `memberships.authz_revision` is always > 0, so a negative revision identifies a support scope; `internal/platform` then runs the transaction read-only and writes a throttled `support.used` audit row. `identity.list_session_stores` lists such stores with role `support` and the grant's permissions. The regular branch is unchanged. Details and operator surface: `contracts/platform-operator-v1.md` §7.
