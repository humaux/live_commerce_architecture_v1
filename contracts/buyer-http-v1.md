# Buyer private HTTP transport v1

Status: FROZEN after independent preflight of 0de251e (no open P0/P1/P2). This is the private
Go boundary for the future storefront BFF, not a publicly exposed checkout.
Existing buyer, storefront, fulfillment and checkout contracts remain authoritative.

## Authority and lifecycle

- `buyerhttp.New(ctx, issuerPool, buyerPool, checkoutService, bffKey, ttl)` returns
  `(http.Handler, error)`. It validates the issuer and buyer runtime authorities,
  creates the existing domain resolver and buyer issuer, and borrows all pools.
  The supplied checkout service is created by `checkout.New`; no pool ownership
  or queue worker moves into the handler. No new dependencies or migrations.
- Mount `/v1/buyer/` only when `COMMERCE_BUYER_ENABLED=1` (existing flag grammar).
  Disabled mode reads
  only that flag. Enabled mode requires a literal loopback listener, separate
  `COMMERCE_BUYER_BFF_KEY` (canonical 32-byte raw base64url), three nonempty DSNs
  (`COMMERCE_BUYER_ISSUER_DATABASE_URL`, `COMMERCE_BUYER_DATABASE_URL`,
  `COMMERCE_CHECKOUT_DATABASE_URL`) and `COMMERCE_BUYER_SESSION_TTL` (whole
  seconds, 60 seconds through 30 days). Reject reuse of the configured merchant
  `COMMERCE_BFF_KEY`. Partial startup closes every newly opened pool.
- Every request requires exactly one `X-Commerce-Buyer-BFF-Key` matching the
  separate configured key in constant time and one `X-Commerce-Storefront-Origin`.
  Reject Cookie, Origin, query strings (including a bare `?`), and any client
  scope headers `X-Tenant-ID` / `X-Store-ID`. Never derive scope from Host or
  forwarded headers. The future BFF must construct, not forward, these headers.
- Resolve the exact published origin on **every** admitted request, including
  receipt replays and logout. Use only its returned StoreID. Domain resolution
  is an admission snapshot, not a lifetime lock on publication. No route cache.
- Except POST session, require exactly one canonical `Authorization: Bearer`
  buyer capability. Reject any Authorization on POST session. No merchant-token
  fallback. Existing database owner/store checks run for every data operation.
- Entire request deadline is 10 seconds; existing shorter database deadlines
  remain. Return no partial success on a failed/expired transaction.
- Revocation is idempotent for a syntactically valid unknown/revoked capability
  on an eligible origin. Unpublished origin denies logout too; future browser
  cookie clearing is local cleanup, not proof of server-side revocation.
- Session issue returns a new independent anonymous owner each time. It is NOT
  idempotent and must not be automatically retried or raced by the future BFF.
  No identity/cart merge is implied. The token response is private BFF-only;
  browser cookie, CSRF, trusted-origin forwarding, per-tenant rate limits and
  bootstrap concurrency remain prerequisites before public exposure.

## Routes and strict input

Successful responses use 200 (DELETE session uses 204). There are no payment,
provider side-effect, catalog-discovery or public domain-management routes here.

| Method and path | Input | Operation / response |
| --- | --- | --- |
| POST /v1/buyer/session | `{}` | Issue; private `{token,expires_at}` only |
| GET /v1/buyer/session | no body | Resolve capability; `{authenticated:true}` |
| DELETE /v1/buyer/session | no body | Revoke; empty 204 |
| GET /v1/buyer/cart | no body | GetCart / cart projection |
| PUT /v1/buyer/cart | storefront.CartInput | SetCart / cart projection |
| POST /v1/buyer/quotes | storefront.QuoteInput | CreateQuote / quote projection |
| GET /v1/buyer/quotes/{id} | no body | GetQuote / quote projection |
| PUT /v1/buyer/destination | storefront.DestinationInput | SetDestination / destination projection |
| GET /v1/buyer/destinations/{id} | no body | GetDestination / destination projection |
| POST /v1/buyer/checkout | checkout.Input | Begin / receipt projection |
| GET /v1/buyer/orders/{id} | no body | Get / order projection |

Writes other than session require one `Idempotency-Key`, matching existing
`^[A-Za-z0-9_.:-]{8,128}$`. Session and reads reject Idempotency-Key rather than
suggesting replay support. JSON input is bounded to 64 KiB, application/json,
unknown fields/null at ANY depth/trailing values rejected (optional fields may
be omitted, but explicit null is not accepted). No-body routes reject any bytes.
Path IDs use the existing canonical UUID validator. Unknown routes return 404;
known routes with unsupported methods (including HEAD/OPTIONS) return 405.
No automatic redirects or HTML error bodies. Generic errors use httperror with
request ID, no-store and nosniff, never SQL errors, tokens, addresses or DSNs.
Mapping: malformed input 422 invalid_request; JSON 400 invalid_json / 415
json_required; missing/invalid buyer or BFF credential 401 unauthorized; forbidden
browser/scope headers 403 forbidden; unpublished/unknown origin and missing owned
resource 404 not_found; conflict 409; insufficient inventory 409
insufficient_inventory; timeout/database/unclassified errors 503 unavailable.
All POST session errors, including resolver failures, use the shared
`httperror.WriteNonRetryable` envelope so `retryable:false`; other routes retain
the existing shared envelope semantics. A lost session issuance response remains
unknown and must not be automatically retried even without an HTTP response.

## Explicit response projections (frozen function names)

All are concrete allowlisted structs, with no embedded domain DTO or raw map.
`projectCart(storefront.Cart)`, `projectQuote(storefront.Quote)`,
`projectDestination(storefront.Destination)`, `projectCheckout(checkout.Result)`,
`projectOrder(checkout.Order)` return private-package concrete values. Projection
tests assert exact recursive JSON keys and preservation, not just denylist text.

- Cart: `id,currency,version,items[]` with `sku_id,quantity`; empty items is `[]`.
- Quote workflow: `id,cart_id,cart_version,market_id,country,method,currency,
  created_at,expires_at,lines[],amount`. Lines: `sku_id,code,name,description,
  quantity,unit_price_minor,amount`. Line amount: `subtotal_minor,discount_minor,
  tax_minor,total_minor`. Quote amount: `subtotal_minor,discount_minor,
  shipping_minor,shipping_tax_minor,tax_minor,total_minor`. Empty lines is `[]`.
  Market/country/method come from its existing immutable policy snapshot.
- Destination workflow: `id,version,cart_id,cart_version,kind,country,
  recipient_name,phone,home_address,selected_at,expires_at`; optional `pickup`.
  Home address: `region,city,postal_code,line1,line2`. Pickup: `id,kind,namespace,
  code,name,address,country` only. No source evidence, actor or verification claim.
- Checkout receipt: `order_id,hold_expires_at`. This is a historical command
  receipt, not the current reservation/payment state. GET order is authoritative.
- Order: `order_id,commercial_state,fulfillment_state,snapshot`; optional
  `hold_expires_at` only for DRAFT. Snapshot has `quote,destination,service`.
  Order quote display: `currency,lines,amount` with the same safe line/totals
  projection; no quote/cart/market/version IDs. Destination display:
  `kind,country,recipient_name,phone,home_address` plus optional pickup display
  (`kind,namespace,code,name,address,country`, no ID). Service display:
  `code,name_hans,name_hant,name_en,delivery_kind,mode` only.
- Never serialize raw checkout.Order/Result/Snapshot, allocation, warehouse IDs,
  binding IDs/versions, generation, reservation/job IDs or owner/tenant IDs.
  Workflow opaque IDs/CAS versions are intentionally retained where subsequent
  commands require them; they are not authentication credentials. Service and
  allocation version discovery remains a separate prerequisite for browser UI.

## Acceptance gate and exclusions

1. Unit/race: strict transport parsing, auth/header/method/route/body rejection,
   closed configuration and exact DTO allowlists, all three locale-independent.
2. Real isolated PG through HTTP: issue -> cart -> quote -> destination -> begin
   -> order; replay same receipt and one order/hold/job, differing payload conflict;
   owner/store isolation, merchant token rejection, revoked/expired token denial,
   origin unpublish/rebind denial and sanitized failures; no customer data.
3. Independent code review, root targeted run and full race/vet regression.
4. Public storefront BFF/browser/real DNS-TLS/provisioning/live payment/shipping
   are NOT_RUN. Mock DB-free HTTP tests alone cannot claim transaction acceptance.

## Implementation ownership

Integrator owns this contract, cmd/api wiring, platform validator, shared
httperror.WriteNonRetryable helper (preserving existing Write behavior), PG tests,
test runner and acceptance docs. HTTP author owns handler.go/handler_test.go;
projection author owns projections.go/projections_test.go. Each writer uses an
isolated worktree. Independent review must approve the contract before writers
implement it, then review the combined diff and evidence before acceptance.
