# Merchant purchase-entry projection v1

Status: FROZEN_FOR_IMPLEMENTATION, 2026-09-25. Independent preflight on `0a2e896`
has no remaining P0/P1/P2 (memory `8737f904-f447-424e-b0fd-bf0568db1e9c`).
No implementation or gate pass is implied. Baseline `f6c5cdb`. Implements the missing catalog-to-published-
origin connection in [automatic product payment entry](product-payment-entry-v1.md).

## User outcome and non-goals

After saving a product and at least one active priced SKU, the merchant UI will
automatically read its reusable purchase entry and offer copy/open. It must not
require creating a PSP product or fixed-price payment link. Existing product and
SKU commands and their idempotent receipts stay unchanged: a storefront lookup
failure must not roll back a saved product or invite resubmission of its writes.

This read API provides **configuration**, not a network availability assertion.
`configured` means current authorized database facts permit deriving a URL; it
does not prove that the buyer page, DNS/TLS, PSP or shipping is deployed. The
copy/open UI is released only together with the working public product route
and its browser gate. Until then this is backend-only. Do not label it PE01 PASS.

No catalog URL column, second price table, new identity/service, provider call,
domain publication write, stock reservation, buyer session or order is added.

## Domain URL contract

The reusable URL is exactly:

`<verified-published-origin>/<locale>/products/<canonical-product-uuid>`

`locale` is one of `zh-CN`, `zh-TW`, `en`, supplied explicitly by the caller.
No default locale inferred from currency, location, account or IP. No amount,
token, buyer identity, PSP reference, arbitrary hostname or return URL is accepted
or included. Product identity survives price changes; changing locale changes
only the locale segment. SKU is selected on the destination, not embedded yet.

Exactly one eligible domain yields a URL. Zero yields `storefront_unavailable`.
Multiple eligible domains yield `domain_selection_required` with no URL; do not
choose a random/lexical domain as a purported primary. A future explicit primary
domain choice is a separate reviewed upgrade, not another guessed default here.

## Private database reader (migration 0023)

Add `identity.resolve_storefront_origin(p_hash bytea, p_store uuid)`, a bounded
read-only **VOLATILE** SECURITY DEFINER function owned by existing non-login `commerce_auth`,
fixed `search_path=pg_catalog`, fully qualified references, PUBLIC EXECUTE revoked,
EXECUTE granted only to `commerce_runtime`. No application login gains role
membership or direct access to either publication table. Grant SELECT on those
two tables to the existing function owner with explicit owner-only SELECT RLS
policies; preserve FORCE RLS and the existing buyer owner policies. No grants
to buyer issuer/runtime, checkout, worker or merchant identity login.

The function returns exactly one internal row `(state text, origin text)`:
`configured` with canonical origin; `storefront_unavailable` or
`domain_selection_required` with empty origin. It must:

1. Reject invalid 32-byte hash/store inputs and non-READ-COMMITTED transactions.
2. Call existing `identity.resolve_access(hash, store, 'catalog:read')`; map
   unauthorized/not_found/forbidden to sanitized PT401/PT404/PT403 respectively.
   Missing, expired, revoked, wrong-audience or ungranted tokens never enumerate
   another shop's domains. Do not substitute caller-controlled GUCs for this call.
3. Require transaction-local `app.tenant_id`, `app.store_id`, `app.principal_id`
   to match the resolved authorization and store (missing/malformed/mismatch
   denies); the normal caller is inside existing `platform.WithScope`.
4. Sample `t0=clock_timestamp()` once after authorization. Use one coherent
   domain/publication/store/tenant SELECT for only that authorized scope. Require active tenant/store,
   published=true, ACTIVE domain, ownership/TLS verification at or before the
   `t0` and validity strictly after `t0`, just like the existing
   [origin resolver](published-storefront-resolver-v1.md).
5. Apply **all** these eligibility predicates in that SELECT's WHERE **before**
   LIMIT 2. Two rows unconditionally yield ambiguity, even if one expires during
   the read. For a sole row, sample `t1=clock_timestamp()` after the read and
   reject expired/future proof on that same captured row. Do not prune an expired
   candidate and choose another, requery a fallback, or LIMIT raw unfiltered
   domain rows. Truncation must never hide a second eligible domain.
6. Return no domain IDs, evidence references, proof times, internal versions,
   tenant/member data or credentials. No DNS/network request or state mutation.

The role expansion is explicit: the existing auth owner gains publication reads
solely for this token-authenticated function. It does **not** gain publication
writes. Existing buyer issuer resolution remains unchanged. A scoped lookup
index `(tenant_id, store_id, origin)` on the domain table is permitted; no new
table, uniqueness assumption or primary-domain backfill.

## Go and HTTP

Add a small `catalog.ReadPurchaseEntry(ctx, tx, scope, token, productID, locale)`
function. `token` is the already authenticated opaque merchant session received
by the composition layer; hash with SHA256 before the fixed SQL call, never log,
return or persist it. This extra token parameter is deliberate: the new privileged
origin reader cannot trust a fabricated Scope/GUC alone. Do not open a second
pool or transaction; reuse the merchant READ COMMITTED scope and five-second
request deadline.

Use a local error translator, not unchanged `catalog.mapError`: map PT400 to
`command.ErrInvalid`, PT401 to `platform.ErrUnauthorized`, PT403 to
`platform.ErrForbidden`, PT404 to `platform.ErrScopeNotFound`. Non-RC is exactly
PT503 and maps to an explicit unavailable sentinel handled as sanitized HTTP503.
Preserve context cancellation/deadline and sanitize unknown driver errors through
the existing HTTP error path. Existing generic catalog operations retain their
error behavior. The second authorization can differ from WithScope's first one;
revocation/missing permission is not a database500. WithScope does not provide a
final authorization check after the callback and none is presumed here.

Return an explicit DTO with exactly four JSON keys:
`product_id`, `locale`, `state`, `url`. State is one of:

- `configured`: active visible product, at least one active SKU in current store
  currency, and one eligible published origin. URL follows the contract above.
- `product_inactive`: visible product archived, empty URL.
- `no_active_sku`: no current-currency active SKU, empty URL.
- `storefront_unavailable`: no eligible publication/domain, empty URL.
- `domain_selection_required`: multiple eligible domains, empty URL.

Unknown or foreign product is the same not-found error. Read products/SKUs with
ordinary runtime RLS and explicit tenant/store predicates; do not elevate catalog
reads. Existing nonnegative-price invariants remain; zero price is not silently
treated as missing data. URL configuration does not qualify a PSP to collect zero.
Validate every database-returned state/origin combination before concatenation;
unknown or malformed results are sanitized unavailable/database errors, not URLs.
No dynamic SQL, request Host, X-Forwarded-Host or caller-supplied origin fallback.

Register `GET /v1/admin/stores/{store_id}/products/{product_id}/purchase-entry`
under fixed `catalog:read` permission, reusing the existing admin transport.
The only query key is required single nonempty `locale`; reject unknown/duplicate
keys, invalid encoding, semicolons, unsupported locale or raw query >64 bytes.
Reject request bodies and Idempotency-Key. Preserve no-store and safe errors.
The HTTP response projects the four fields explicitly. Unsupported methods never
mutate state. Existing product/SKU response schemas do not change.

Frontend integration after the approved buyer route is built: refresh this
projection after product/SKU/price mutations and selection, retain the saved
command receipt if projection fails, and refresh before copy/open. A projected
URL is not permanent authorization; the buyer route resolves publication and
product status on every request. Archive/unpublish blocks new purchases, not
settlement of a previously agreed order.

## Acceptance gates (all NOT_RUN)

| Gate | Required actual evidence |
| --- | --- |
| MPE01 | Real ordinary merchant PG: own active product/SKU + one verified published origin yields exact locale URL; price update and command replay preserve product identity; zero-priced SKU handled without pretending payment readiness. |
| MPE02 | Unknown/foreign product, archive, no SKU, wrong currency, unpublished, inactive tenant/store, suspended/detached, future/expired proof, two eligible domains; at least three raw domains including ineligible ones cannot hide two eligible ones. Two initially eligible candidates remain ambiguous if one expires between capture and final clock; no arbitrary primary selection. |
| MPE03 | Direct SQL ACL/ownership/FORCE RLS and non-RC; runtime cannot read/write tables; other app roles cannot execute; forged/missing GUC, invalid/revoked/foreign token and missing permission fail; no evidence/credentials in DTO. |
| MPE04 | Real admin HTTP authorization, method/body/query/locale/idempotency boundaries, exact four-key DTO, no-store, canonical URLs; no raw database errors. Causally revoke authorization after WithScope succeeds and before the new reader: exact401/403/404 (as applicable), never500. Direct non-RC produces PT503 and transport maps it to503. |
| MPE05 | Catalog save/replay/price edits plus lookup leave one original product/SKU receipt; lookup does not change inventory/order/payment/session/provider facts; revoked publication is reflected without restart. |
| MPE06 | Focused checks, full real-PG race/vet, independent source/evidence review, dependency notes, code index and Humaux canvas. Buyer page/merchant copy UI and live PSP are NOT covered by this backend gate. |

## Rejected alternatives and upgrade signals

- Persist a derivable URL or price inside another table: duplicated authority.
- Let runtime SELECT publication/proof tables directly or trust GUCs alone in a
  privileged reader: unnecessary exposure and weaker authorization boundary.
- Auto-publish active stores or infer a domain from the admin request: no consent
  or verified domain evidence. Reuse the existing publication admission kernel.
- Return a fixed PSP checkout on catalog save: no final quantity/destination/
  order amount, and couples catalog availability to external payment availability.
- Arbitrarily pick among multiple domains: add an explicit canonical-domain
  decision only when the domain-management workflow is built and verified.
