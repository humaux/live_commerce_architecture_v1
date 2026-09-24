# Buyer catalog discovery v1

Status: FROZEN after independent preflight on 2026-09-25 (no open P0/P1/P2).
Implementation accepted at source `286bfa0`: scoped PG/HTTP gates, 345-test full
regression and independent review passed; see the [bounded acceptance evidence](../docs/implementation/2026-09-25-buyer-catalog-discovery-acceptance.md).
This is the first dependency of
[automatic product payment entry](product-payment-entry-v1.md), not that entire
flow or a public product page. No provider call, catalog write or new database
authority is introduced.

## Domain contract

`storefront.ListCatalog(ctx, tx, buyer.Scope, CatalogRequest)` returns
`pagination.Page[CatalogItem]` inside the existing `buyer.WithScope` transaction.
`CatalogRequest` has `ProductID string` and `Page pagination.Request`.
`CatalogItem` JSON keys are exactly
`product_id,sku_id,name,description,sku_code,currency,price_minor`.
The name/description come from the product; other display data from the SKU.
No full merchant DTO embedding, physical/customs fields, tenant/store/owner IDs,
credentials, inventory promises, payment URL or fictional media fields.

- Check buyer transaction scope before and after read. One joined query of
  catalog products/SKUs and the scoped store currency returns only active product
  and active SKU rows with the current shop currency. Reuse migration 0007's
  restricted columns and RLS. No SELECT * or owner/checkout pool fallback.
- Optional ProductID is a canonical UUID. Absent = all active SKUs; provided =
  only that product's SKUs. Missing, foreign, archived or no-active-SKU product
  returns the same empty list; no cross-store existence oracle.
- Pagination defaults to 50 and caps at 100. Limit 0 means default; negatives
  and greater than 100 are invalid. Stable ascending SKU UUID keyset, limit+1;
  concurrent inserts/archives do not imply a snapshot across pages. Terminal
  `next_cursor` is empty; `items` is always an array, including no results.
- Cursor contains version 1, a SHA256 binding digest and the last SKU UUID.
  Digest binds a constant collection name, tenant, store and product filter via
  unambiguous JSON encoding; it is not authentication/signature/encryption.
  A forged position may skip rows but cannot change SQL scope or visibility.
  Do not include raw tenant/store IDs, buyer/session/capability or price in it.
  Reject wrong version/digest, malformed/unknown/null/duplicate fields,
  noncanonical base64url, trailing JSON, invalid UUID and input over 1024 bytes.
  Encoding is canonical compact JSON in version/binding/after field order;
  decode may require exact re-encoding equality, since clients treat it opaque.
  An unsigned position is sufficient: authorization remains the capability and
  RLS; do not invent a new cursor secret or token service.
- Reads produce no cart/quote/order/receipt/event/inventory/PSP write. Prices are
  display data, never a quote or permission to collect that amount. Existing
  quote/checkout revalidates after selection; archive/price edits stay effective.

## Private HTTP amendment

Add exactly `GET /v1/buyer/catalog`. Every existing private BFF, published-origin,
buyer bearer, deadline, no-body, no-store and error-envelope rule remains.
The response is the concrete allowlisted page/item DTO (do not embed a mutable
catalog domain DTO in the HTTP response).

This GET alone permits a query with optional `product_id`, `limit`, `cursor`.
Maximum raw query length 2048 bytes. Each parameter occurs at most once and,
when present, is nonempty. Reject unknown keys, bad percent encoding and
semicolon syntax. Limit is decimal digits only, in 1..100 if present.
A bare `?` remains 403; all existing routes still reject all query strings.
For this GET, malformed allowed-query syntax/value maps to 422 invalid_request.
GET rejects a body and Idempotency-Key; other methods have no new permission.
Keep query admission narrowly tied to the exact canonical path and GET method.

## Acceptance

BCAT01: real isolated PostgreSQL + ordinary buyer role sees only active own-store
SKUs/current prices; archived product/SKU and foreign-store/tenant rows excluded.
BCAT02: filter and limit+1 keyset cover multiple pages without duplicates; cursor
cannot move across store/filter, invalid cursors/limits fail, empty result is [].
BCAT03: merchant change-price/archive readback reflected; reads have zero durable
fact deltas; physical/customs/account/token fields absent by exact-key assertions.
BCAT04: real private HTTP tests for keys/origin/capability/own-store, revoked
capability/unpublished origin, malformed/duplicate/unknown query, body, unsupported
method, no automatic redirects, existing-route query rejection and exact DTO.
BCAT05: focused subset plus full real-PG/race/vet regression and independent review.

No new schema, public BFF, buyer identity bootstrap, provider payment or live
customer environment is included. All BCAT gates remain NOT_RUN until executed.
