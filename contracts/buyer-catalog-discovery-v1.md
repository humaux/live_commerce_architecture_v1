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
`product_id,sku_id,name,description,sku_code,currency,price_minor,images`
(`images` added by the catalog-media amendment below; it was absent in v1.0).
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

## Amendment catalog-media (R3, 2026-10-01; migrations/0082)

Product photos exist now, so the buyer projections gain them. This amendment changes no scope, cursor, pagination
or permission rule above.

- **Item `images`**: `[{id, width, height}]`, ordered by merchant position (index 0 is the cover), always an array
  (`[]` when the product has none), the same list on every SKU row of one product. `width`/`height` are integers or
  `null` (WebP is not decoded). Only photos of an ACTIVE product of the buyer scope's store appear. Metadata only:
  no bytes, no content type, no size, no position number.
- **Page `store_name`**: the published store's public name (string, never null), a sibling of `items` and
  `next_cursor`, so the page keys are exactly `items,next_cursor,store_name`. It lets the storefront home show the
  shop name without a second route. Source: `control.stores.name` under the existing buyer_read policy
  (column grant `SELECT(name)` to `commerce_buyer_runtime`).
- **Bytes**: `GET /v1/buyer/media/p/{product_id}/{image_id}` (private BFF transport, same BFF key and
  `X-Commerce-Storefront-Origin` rule as the Meta feed). It needs and accepts NO buyer bearer: an `Authorization`
  header, cookie, query string, body or Idempotency-Key is refused (403 / 422). The store is resolved inside the
  database from the verified origin; an unpublished origin, unknown or foreign image, or archived product is the
  same 404 (no existence oracle); a malformed id is 422. Success: the stored bytes with the validated
  `Content-Type` (`image/jpeg|png|webp`), `Cache-Control: public, max-age=86400, immutable` (the id changes
  whenever the content changes), `X-Content-Type-Options: nosniff`, `Content-Security-Policy: default-src 'none';
  sandbox`, `ETag` = SHA-256 of the bytes.
- **Public URL**: the storefront Next app serves `/media/p/{product_id}/{image_id}` on the shop origin through
  that route (`apps/storefront/app/media/p/[productID]/[imageID]/route.ts`); the Meta feed `image_link` is the
  absolute shop origin plus that path for the product's first photo, empty when it has none.
- Acceptance additions: BCAT06 buyer sees images of active products only, in position order, `[]` when none, and
  never another store's photo; BCAT07 media route: published origin + active product returns the exact uploaded
  bytes and headers above, every other case is the same 404; BCAT08 exact keys `items,next_cursor,store_name` and
  item keys including `images`. All NOT_RUN until executed.

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
