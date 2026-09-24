# Merchant delivery/payment method HTTP v1

2026-09-24. Frozen transport slice of [merchant service settings](merchant-service-settings-v1.md).
Reuses `httpapi.scoped/bodyRoute`, merchant sessions, domain command receipts,
CAS revisions and the existing SQL authority. No new engine, schema or provider call.

## Routes

Base: `/v1/admin/stores/{store_id}/markets/{market_id}/countries/{country}`.

|Method and suffix|Domain|Permission|
|---|---|---|
|GET `/delivery-services/{code}`|`fulfillment.GetService`|`integration:read`|
|PUT `/delivery-services/{code}`|`fulfillment.SetService`|`integration:manage`|
|GET `/payment-methods/{code}`|`payments.GetMethod`|`integration:read`|
|PUT `/payment-methods/{code}`|`payments.SetMethod`|`integration:manage`|
|POST `/payment-methods/{code}/inspect`|`payments.InspectMethod`|`integration:read`|

All routes use the current authenticated merchant scope. Cookies, Host and caller
tenant/principal headers are not authority. Domain functions recheck permission.
All query strings are rejected (422), including an empty trailing `?`; these are
exact-resource operations, not lists. Lists and provider capability discovery are
separate, not implicitly implemented by these five endpoints.

PUT bodies are the complete existing `fulfillment.ServiceInput` or
`payments.MethodInput` JSON objects, not partial patches. `market_id`, `country`
and `code` must exactly equal the path; conflicting or omitted targets return
422 instead of silently overwriting the target. `expected_version=0` creates;
subsequent revisions require the current version. `Idempotency-Key` is required
by the existing domain command contract: an identical retry returns the original
response; a reused key with a different body returns 409. Successful PUT/GET
returns 200 and the domain DTO, only after the transaction commits.

Inspect accepts the existing `payments.CheckInput`, with the same path identity
check. Its amount is a diagnostic input only, never checkout authority. It creates
no receipt, revision, audit event, payment, provider call or job. It requires no
idempotency key. `available=false` with bounded reasons remains the truthful
result while payment orchestration/provider admission is absent.

Reuse the shared 64 KiB JSON bound, unknown-field/trailing-value rejection,
five-second transaction context, no-store response, generated request ID and
safe error envelope. No credentials are accepted or returned by these routes.
Credential connection/rotation HTTP, a settings UI, production secret loading,
buyer checkout UI and external merchant qualification remain separate work.

## No capability upgrade

Do not remove the disabled-only payment-method constraint or enable API logistics.
Manual delivery enable/visibility changes remain independent of payment methods.
Do not equate the old platform's “自主模式” label with manual fulfillment.
This slice makes existing configuration accessible to an authenticated admin
client; it does not claim automatic collection, shipment creation or settlement.

## Acceptance gates

- H01: real PG + actual HTTP handler saves and GET reads the same full three-language
  configuration; enabling/hiding manual delivery and payment draft visibility are
  independent. Restart/refresh uses the database, not handler memory.
- H02: permanent identical retry, changed-body conflict, stale version, and two
  concurrent updates prove exactly one revision/audit/receipt per accepted command.
- H03: unauthenticated 401, readable-store read-only identity write 403, foreign
  store 404, revoked permission, forged scope/body target all reject without writes.
- H04: inspect is read-only and unavailable; payment enable and API-logistics enable
  stay rejected; no provider operation or River job is created.
- H05: wrong media, unknown/trailing/malformed/oversized JSON, query parameters,
  unsupported methods and routes use safe correlated errors and do not echo input.
- H06: existing full PG/race/vet regressions pass. This unit has no UI change;
  browser/product/provider sandbox/live gates are NOT_RUN, not inferred from HTTP tests.

Root owns this contract and independent PG acceptance. One isolated commerce
worker owns only `internal/httpapi/settings.go`, its unit tests, and the single
registration call in `handler.go`; no shared domain/schema/lockfile modifications.
