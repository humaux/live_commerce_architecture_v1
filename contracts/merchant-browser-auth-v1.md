# Merchant browser authentication v1

Status: frozen for local implementation, 2026-09-20. Extends merchant-identity-v1, not an assertion of provider or production readiness. Visual entry flow remains pending user approval.

## Boundaries and dependencies

Browser → Next route handlers (cookie/CSRF terminator) → private Go API. Go composes separate validated runtime and identity pools. Existing identity service owns OIDC, one-use flow consumption, local session issuance and atomic initial-store creation. No new password/session database, tenant authority in the browser, or environmental merchant bearer in production.

Next-native Request/Response/cookies, Web Crypto, Go net/http and existing pgx/OIDC packages suffice. No new dependency. The fixed public origin and the existing provider adapter determine callback/authorization URLs; Host and Forwarded never do.

## Server configuration

- `COMMERCE_IDENTITY_ENABLED`: empty/`0` off; `1` on; other values invalid. Disabled Go wiring must not inspect/connect identity DSN or discover OIDC; no identity routes.
- Enabled: `COMMERCE_IDENTITY_DATABASE_URL`, `COMMERCE_OIDC_ISSUER`, `COMMERCE_OIDC_CLIENT_ID`, optional `COMMERCE_OIDC_CLIENT_SECRET`, `COMMERCE_IDENTITY_PROVIDER_KEY`, `COMMERCE_SESSION_TTL` (Go duration, 5m–24h), `COMMERCE_PUBLIC_ORIGIN` and `COMMERCE_BFF_KEY` required. `COMMERCE_BFF_KEY` is a server-only 32-byte unpadded base64url secret, never a merchant token. Missing/invalid config fails startup.
- `COMMERCE_PUBLIC_ORIGIN`: exact HTTPS origin; no userinfo, non-root path, query or fragment. Only explicit `COMMERCE_IDENTITY_ALLOW_LOOPBACK_TESTS=1` permits HTTP literal loopback/localhost for isolated tests. Derive callback exactly `/api/auth/callback`. No request-selectable provider/redirect URI.
- Enabled identity Go process binds literal loopback only (same-host BFF topology for this slice); non-loopback LISTEN_ADDR fails. A later remote/private-network deployment needs an explicit reviewed transport change, not `0.0.0.0` by accident.
- `COMMERCE_API_ORIGIN`: Next-only fixed HTTPS or HTTP literal loopback origin, no credentials/path/query/fragment. Auth and business API use only this origin, `redirect:error`, bounded timeout and no cache.
- `COMMERCE_ONBOARDING_ENABLED`: default off (`1` enables). `COMMERCE_ONBOARDING_CURRENCIES`: explicit comma-separated uppercase ISO-like three-letter codes; required when enabled. No locale-derived currency.
- Existing `COMMERCE_FIXTURE_*` remains development-only exact loopback and is mutually exclusive with identity mode. A missing/invalid cookie NEVER falls back to fixture when identity is enabled; production never accepts fixture.

## Private Go endpoints

Every `/v1/identity/` request requires exact `X-Commerce-BFF-Key` using constant-time comparison. It rejects browser Origin/Cookie headers, emits no CORS, no-store, and sanitized JSON error envelopes with server-owned request IDs. Shared key is stripped from public requests and supplied only by Next. Bare headers/private bind are complementary boundaries, not a substitute for authorization.

| Route | Request | Success |
| --- | --- | --- |
| POST `/v1/identity/login/start` | JSON `{}` | 200 `{authorization_url,binding,expires_at}` |
| POST `/v1/identity/login/complete` | JSON `{state,binding,code}` | 200 `{token,expires_at}` |
| POST `/v1/identity/initial-store` | cookie-derived bearer; Idempotency-Key; JSON `{tenant_name,store_name,warehouse_name,currency}` | 200 `{tenant_id,store_id,warehouse_id}` for first/replayed request |
| POST `/v1/identity/logout` | cookie-derived bearer; JSON `{}` | 204 after successful revocation |

Strict JSON MIME, 64 KiB max, unknown fields/trailing JSON/null rejected. Query parameters rejected. Service validates content and opaque tokens; HTTP must not accept issuer/subject/principal/permissions/TTL or arbitrary identifiers. Error mapping: invalid 422, unauthorized 401, disabled 403, conflict 409, dependency/unavailable 503. Unsupported method 405, unknown route 404. No raw provider/DB exceptions or tokens in diagnostics. Login completion must never automatically retry a consumed code, including a network timeout.

`GET /v1/admin/stores` uses only merchant bearer (not a BFF-key grant), rejects queries and returns `{items:[{id,name,currency}]}` sorted by UUID. Runtime invokes `identity.list_session_stores(token_hash)`; fixed commerce_auth-owned SECURITY DEFINER, pg_catalog search_path, PUBLIC revoked, runtime EXECUTE only. Derive active merchant principal from live session, then active membership/tenant/store and store:read grant. Invalid/expired/revoked/inactive/buyer principal raises PT401. Empty valid list is 200, never 401. No principal/tenant input. Function limits to 101; >100 is 409 conflict, no silently truncated list. No need for pagination until explicit multi-store/invitation expansion.

## Public Next routes and cookies

- `POST /api/auth/login`: native form URL-encoded body with exactly `locale` in zh-CN/zh-TW/en, <=1024 bytes, no query/unknown/duplicate fields. Exact Origin equals configured public origin. This anonymous endpoint uses exact Origin rather than an existing session CSRF token. Calls private start once, sets login binding cookie, 303 to returned validated authorization URL. GET cannot begin a flow. No client return URL.
- `GET /api/auth/callback`: exactly one bounded `state` and `code`; reject duplicates/unknown params except bounded standard `iss` (if supplied it must equal configured issuer), or fail safely for provider `error`. Require HttpOnly login binding. Always clear login binding on outcome. Call private complete once. On success set session and fresh CSRF cookies, then 303 to `/{locale}/`. On failure redirect only `/{locale}/?auth=failed`; no provider details/codes echoed. Locale comes from allowlisted login cookie metadata, default zh-CN for malformed metadata. Do not destroy an existing valid merchant session on an invalid unbound callback.
- `POST /api/auth/logout`: JSON `{}`, cookie session, exact Origin and CSRF. Revoke first; clear session/CSRF after 204 or authoritative 401 only. Upstream failure is 503, not a successful logout. Already signed-out request may clear stale cookies with a stable unauthorized result.
- `POST /api/onboarding/initial-store`: exact Origin + CSRF + session; bounded strict body and Idempotency-Key, forwards only the four fields. Same key/body recovery is mandatory after unknown results; no auto retry/new key in transport.
- `GET /api/stores`: authenticated store list, no query/body, cookie-derived bearer only. Safe JSON projection, no credentials. Empty means onboarding needed.
- Existing `/api/stores/[store]/[...resource]`: retain allowlist, body/key limits and sanitized response behavior. Select requested store only from authenticated list; invalid/foreign store yields generic 404. Every write requires Origin+CSRF. Never forward browser Authorization, Cookie, tenant, Forwarded, BFF key or correlation authority.

Cookies: `__Host-commerce_login` contains binding + allowlisted locale, <=5m; `__Host-commerce_session` opaque token <=actual DB expiry; both Secure, HttpOnly, SameSite=Lax, Path=/, no Domain. `__Host-commerce_csrf` random 32-byte base64url has same scope/expiry but is readable solely for `X-CSRF-Token`. Secure remains true in loopback tests. Reject ambiguous duplicate authority cookies. Compare CSRF cookie/header in constant time after strict format validation. Session cookie invalid/missing is unauthorized; upstream business 401 clears auth cookies in route handlers. Server-component loaders cannot mutate cookies, return signed-out state honestly; DB remains authority.

backend.ts uses cookie-derived current session and store list; chooses an explicitly requested member store or deterministic first store. Do not expand fixture selection authority. Client command helper reads only CSRF cookie, not authentication secrets. No component/page/CSS/proxy locale behavior changes until visual approval. Existing interface language continues independent of currency/tenant/store.

## Required evidence

1. Default-off routes absent, no identity DB/provider I/O; incomplete enabled config fails, unsafe origin/listen/fixture overlap fails.
2. Missing/foreign Origin, GET login, unknown/duplicate locale/redirect fields cause no login flow.
3. Missing/wrong/expired/replayed binding/state and provider error do not issue session; no retry after uncertain completion.
4. Cookie exact flags/expiry and no secret/provider data leaks; production fixture denied.
5. Missing/mismatched CSRF/Origin blocks all authenticated writes before Go; no header impersonation.
6. Valid vs expired/revoked/buyer/no-membership store lists; inactive and foreign grants invisible, >100 explicit error; runtime still cannot read identity tables.
7. Initial-store allowlist/idempotency/rollback and logout/expiry run on isolated real PG, not mocks alone.
8. Signed mock IdP + real PG + Next/browser protocol flow; exact routes/cookies/navigation, no new visual page approval assumed.
9. Existing ledger/production-denial/i18n gates preserved. Root independently replays combined changes, separate reviewer closes P0/P1.

NOT_RUN remains real IdP/registration/email/MFA, deployment ingress/rate-limit policy, shared session cleanup/retention policy and full SaaS release gates. These are deployment requirements, not implied by a local login protocol pass.
