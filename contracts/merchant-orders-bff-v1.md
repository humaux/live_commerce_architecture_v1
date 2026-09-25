# Merchant order browser transport v1

2026-09-25 DRAFT, requires independent preflight. Separate from pending visual
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
cursor nonempty max1024, state exactly all/DRAFT/AWAITING_PAYMENT/CONFIRMED/CANCELLED.
Pass original accepted query through, never construct a tenant or buyer identity.

Auth derives only from the HttpOnly session cookie. A route store is merely a
selector and must be present in authenticatedStores. Browser Authorization,
Cookie, tenant and arbitrary headers must never be forwarded as authority.
Reuse safeError and clearAuthCookies on401. Backend still enforces orders:read
and rechecks permission after reads; do not infer it from catalog access.
Successful and error replies remain no-store; add private to order success if
needed. No response/request bodies in logs or persistent browser storage.

## Gates MBT01–04

1. Positive collection/detail forwards exact allowed path/query with server-side
session token, returns private/no-store; no production traffic. No authConfig and
fixture-only requests cannot call orders even with a matching fake store.
2. Actual Next BFF route strict metadata: duplicate/unknown/empty/malformed query,
detail query, bare ?, GET body/content-length/transfer/key, bad store/order IDs
and every non-GET method fail without issuing an upstream order request.
3. Wrong/missing/ambiguous cookie, unlisted store, missing orders:read, expired or
revoked session and upstream503 are safe and do not leak response diagnostics;
401 clears cookies. Malicious browser headers never replace server token.
4. Independent tests/source review and root repeats; strictTS/build, existing
admin identity/settings/purchase-entry regressions, and a real Next→Go→PG read
chain. Pure mocked transport tests alone are insufficient for full acceptance.

Browser *UI* flow, locale/store/session replacement of in-flight detail, responsive
rendering and visual review are subsequent gates, not implied by BFF acceptance.
No overbroad generic-proxy refactor. Keep T11 IN_PROGRESS and global deployment
closed. Root integrator owns this contract; implementation and independent test
authors use non-overlapping paths in isolated worktrees after freeze.
