# Merchant order browser transport v1

2026-09-25 FROZEN after independent preflight of `1a8187e` and the explicit
raw-query/header clarifications below. Separate from pending visual
selection. Consumes accepted [merchant read API](merchant-orders-v1.md); no
changes to Go DTOs, SQL grants, migrations or financial state machines.

## Minimal extension

Reuse `apps/admin/app/api/stores/[store]/[...resource]/route.ts` plus existing
`authConfig`, `sessionToken`, `authenticatedStores`, `callBackend` and `safeError`.
No second proxy/client or dependency. Add only exact GET `/api/stores/{uuid}/orders`
and `/orders/{uuid}` allowlist entries; reject unsupported methods through the
existing policy. Orders always require configured real merchant session transport;
the localhost fixture bearer must never expose order/recipient PII.

Before any upstream request reject a GET body, Transfer-Encoding, Idempotency-Key,
or Content-Length other than literal `0`. Detail rejects any query including bare
`?`. Collection may omit query; otherwise raw query must have only distinct
nonempty `limit`, `cursor`, `state` name=value segments; reject bare ?, empty
segments, malformed escapes, duplicates and unknown keys. Do not normalize a
bad request into a valid one. Use backend bounds: limit canonical integer1..100,
cursor1..1024 ASCII base64url characters `[A-Za-z0-9_-]`, state exactly
all/DRAFT/AWAITING_PAYMENT/CONFIRMED/CANCELLED. Names are literal ASCII, not encoded.
No percent or plus encoding is needed by these fields: reject it, including
`%31`, encoded names and double encoding, instead of decode/re-encode. Go remains
responsible for cursor signature/scope/position validity. Pass original accepted
query through, never construct a tenant or buyer identity.

Auth derives only from the HttpOnly session cookie. A route store is merely a
selector and must be present in authenticatedStores. Browser Authorization,
Cookie, tenant and arbitrary headers must never be forwarded as authority.
Reuse safeError and clearAuthCookies on401. Backend still enforces orders:read
and rechecks permission after reads; do not infer it from catalog access.
Order successes have exact `Cache-Control: private, no-store`; errors retain
safeError/localError's no-store policy. No response/request bodies in logs or
persistent browser storage.

## Gates MBT01–04

1. Positive collection/detail forwards exact allowed path/query with server-side
session token, returns private/no-store; no production traffic. No authConfig and
fixture-only requests cannot call orders even with a matching fake store.
2. Actual Next BFF route strict metadata: duplicate/unknown/empty/malformed query,
detail query, bare ?, encoded name/value/plus/double encoding, GET
body/content-length/transfer/key, bad store/order IDs
and every non-GET method fail without issuing an upstream order request.
3. Wrong/missing/ambiguous cookie, unlisted store, missing orders:read, expired or
revoked session and upstream503 are safe and do not leak response diagnostics;
401 clears cookies. Malicious browser headers never replace server token.
4. Independent tests/source review and root repeats; strictTS/build, existing
admin identity/settings/purchase-entry regressions, and a real Next→Go→PG read
chain with real cookie-derived session (no dev fixture). Verify actual Next
response headers for private/no-store and 401 Set-Cookie clearing. Pure mocked
transport tests alone are insufficient for full acceptance.

Browser *UI* flow, locale/store/session replacement of in-flight detail, responsive
rendering and visual review are subsequent gates, not implied by BFF acceptance.
No overbroad generic-proxy refactor. Keep T11 IN_PROGRESS and global deployment
closed. Root integrator owns this contract; implementation and independent test
authors use non-overlapping paths in isolated worktrees after freeze.
