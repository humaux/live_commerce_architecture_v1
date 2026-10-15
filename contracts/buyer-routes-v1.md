# Canonical private buyer route index v1

Scope: the 46 entries served by `buyerhttp.Routes` plus the feed's GET and HEAD.
This index points to the owning contracts for bodies/responses; it does not replace
or amend those contracts. The three explicitly code-derived sections below record
current implementation for owner review, not a runtime acceptance claim.

## 1. Canonical inventory

Each code span contains METHOD, a space and the full path, exactly as
`scripts/dev/contractdrift/contracts.go:94` (`mdRoute`) reads it. In particular,
`buyer-payment-http-v1.md:100` puts a backtick BETWEEN POST and the full path;
the regexp excludes that backtick from the path token, so it never matches that
row. Relative paths also need a recognized `Route base:` directive. The owning
contracts are left unchanged.

Auth legend: B = exactly one private `X-Commerce-Buyer-BFF-Key`; O = exactly one
BFF-attested `X-Commerce-Storefront-Origin`; C = canonical `Authorization: Bearer`
buyer capability. Every row requires B + O. `no C` means Authorization is refused
on public buyer reads (session issuance separately refuses it). K is the
`Idempotency-Key` policy: required or forbidden. Feed is behind the BFF key but
its handler does not inspect Bearer or K; that transport detail is recorded below
rather than imposing a new restriction. These are private Go paths, not public
browser BFF paths. Preview additionally requires its preview token; claim routes
add the claim token as defined in their owning contract.

| Method + full path | Auth / Idempotency-Key | Owning contract file + section (body/response authority) |
| --- | --- | --- |
| `GET /v1/buyer/session` | B + O + C; K forbidden | buyer-http-v1.md §Routes and strict input; §Explicit response projections |
| `POST /v1/buyer/session` | B + O + no C; K forbidden | buyer-http-v1.md §Routes and strict input; §Explicit response projections |
| `DELETE /v1/buyer/session` | B + O + C; K forbidden | buyer-http-v1.md §Routes and strict input; §Explicit response projections |
| `POST /v1/buyer/session/bootstrap` | B + O + C; K forbidden | buyer-session-registration-v1.md §Frozen interface |
| `POST /v1/buyer/session/retire` | B + O + C; K forbidden | buyer-http-v1.md §Authority and lifecycle (retirement amendment at top) |
| `GET /v1/buyer/design/published` | B + O + no C; K forbidden | storefront-v2.md §B-acceptance |
| `GET /v1/buyer/design/preview` | B + O + no C; K forbidden | storefront-v2.md §B-acceptance |
| `GET /v1/buyer/storefront/primary-origin` | B + O + no C; K forbidden | this index §2.2 (code-derived) |
| `GET /v1/buyer/catalog` | B + O + C; K forbidden | buyer-catalog-discovery-v1.md §Private HTTP amendment; §Amendment product-media-v2 |
| `GET /v1/buyer/checkout-options` | B + O + C; K forbidden | buyer-checkout-options-v1.md §Single surface and existing authority; §Stable bounded pagination and query admission |
| `GET /v1/buyer/cart` | B + O + C; K forbidden | buyer-http-v1.md §Routes and strict input; §Explicit response projections |
| `PUT /v1/buyer/cart` | B + O + C; K required | buyer-http-v1.md §Routes and strict input; §Explicit response projections |
| `POST /v1/buyer/quotes` | B + O + C; K required | buyer-http-v1.md §Routes and strict input; §Explicit response projections |
| `GET /v1/buyer/destination` | B + O + C; K forbidden | buyer-current-destination-v1.md (GET); buyer-destination-v1.md §Buyer snapshot (PUT) |
| `PUT /v1/buyer/destination` | B + O + C; K required | buyer-current-destination-v1.md (GET); buyer-destination-v1.md §Buyer snapshot (PUT) |
| `POST /v1/buyer/checkout` | B + O + C; K required | buyer-checkout-v1.md §Internal implementation interface; buyer-http-v1.md §Explicit response projections |
| `GET /v1/buyer/orders` | B + O + C; K forbidden | buyer-order-history-v1.md §Read contract |
| `GET /v1/buyer/claim-link` | B + O + C; K forbidden | live-keyword-claims-v1.md §7.2 B1 |
| `POST /v1/buyer/claim-link/redeem` | B + O + C; K required | live-keyword-claims-v1.md §7.2 B2; §Amendment Direct checkout |
| `GET /v1/buyer/privacy` | B + O + C; K forbidden | this index §2.1 (code-derived); customers-billing-v1.md §5 (privacy overview) |
| `PUT /v1/buyer/consents` | B + O + C; K required | customers-billing-v1.md §5 HTTP and UI surfaces; §0 rulings CD4/D12 |
| `POST /v1/buyer/privacy/export` | B + O + C; K required | customers-billing-v1.md §5 HTTP and UI surfaces; §0 rulings CD6/D5/D12 |
| `POST /v1/buyer/privacy/erasure` | B + O + C; K required | customers-billing-v1.md §5 HTTP and UI surfaces; §0 rulings D10/D11/D12 |
| `POST /v1/buyer/cvs-selections` | B + O + C; K required | taiwan-cvs-logistics-v1.md §5.2 HTTP |
| `POST /v1/buyer/cvs-stores` | B + O + C; K required | taiwan-cvs-logistics-v1.md §16.1 Store source |
| `POST /v1/buyer/cvs-selections/{id}/verify` | B + O + C; K forbidden | taiwan-cvs-logistics-v1.md §5.2 HTTP |
| `GET /v1/buyer/cvs-selections/{id}` | B + O + C; K forbidden | taiwan-cvs-logistics-v1.md §5.2 HTTP |
| `PUT /v1/buyer/orders/{id}/bank-transfer/proof` | B + O + C; K required | this index §2.3 (code-derived); storefront-v2.md §C (business rules) |
| `GET /v1/buyer/orders/{id}/bank-transfer` | B + O + C; K forbidden | storefront-v2.md §C; §E5 (owned bank-transfer read) |
| `POST /v1/buyer/orders/lookup` | B + O + C; K forbidden | storefront-v2.md §E5 Guest order lookup |
| `POST /v1/buyer/orders/link` | B + O + C; K forbidden | storefront-v2.md §G3 Manual order (Link exchange) |
| `GET /v1/buyer/media/s/{image}` | B + O + no C; K forbidden | storefront-v2.md §B-acceptance |
| `GET /v1/buyer/catalog/v2/products` | B + O + no C; K forbidden | storefront-v2.md §A Implemented shapes and acceptance list; catalog-inventory-v1.md §Buyer shapes |
| `GET /v1/buyer/catalog/v2/collections` | B + O + no C; K forbidden | storefront-v2.md §A Implemented shapes and acceptance list; catalog-inventory-v1.md §Buyer shapes |
| `GET /v1/buyer/catalog/v2/products/{slug_or_id}` | B + O + no C; K forbidden | storefront-v2.md §A Implemented shapes and acceptance list; catalog-inventory-v1.md §Buyer shapes |
| `GET /v1/buyer/catalog/v2/collections/{slug}` | B + O + no C; K forbidden | storefront-v2.md §A Implemented shapes and acceptance list; catalog-inventory-v1.md §Buyer shapes |
| `GET /v1/buyer/media/c/{id}/{image}` | B + O + no C; K forbidden | storefront-v2.md §A Implemented shapes and acceptance list |
| `GET /v1/buyer/media/p/{id}/{image}` | B + O + no C; K forbidden | buyer-catalog-discovery-v1.md §Amendment catalog-media; catalog-inventory-v1.md §Buyer shapes |
| `POST /v1/buyer/orders/{id}/payment/prepare` | B + O + C; K required | buyer-payment-http-v1.md §Private routes and projections; stripe-psp-v1.md §9.2; stripe-buyer-ui-v1.md §4 |
| `POST /v1/buyer/orders/{id}/payment/handoff` | B + O + C; K forbidden | buyer-payment-http-v1.md §Private routes and projections; stripe-psp-v1.md §9.2; stripe-buyer-ui-v1.md §4 |
| `POST /v1/buyer/orders/{id}/payment/refresh` | B + O + C; K forbidden | stripe-psp-v1.md §9.2 Buyer routes; stripe-buyer-ui-v1.md §4 |
| `POST /v1/buyer/orders/{id}/payment/cancel` | B + O + C; K forbidden | stripe-psp-v1.md §9.2 Buyer routes; stripe-buyer-ui-v1.md §4 |
| `GET /v1/buyer/orders/{id}/payment` | B + O + C; K forbidden | buyer-payment-http-v1.md §Private routes and projections; stripe-psp-v1.md §9.2; stripe-buyer-ui-v1.md §4 |
| `GET /v1/buyer/quotes/{id}` | B + O + C; K forbidden | buyer-http-v1.md §Routes and strict input; §Explicit response projections |
| `GET /v1/buyer/destinations/{id}` | B + O + C; K forbidden | buyer-http-v1.md §Routes and strict input; §Explicit response projections |
| `GET /v1/buyer/orders/{id}` | B + O + C; K forbidden | buyer-http-v1.md §Routes and strict input; §Explicit response projections |
| `GET /v1/buyer/feeds/meta.csv` | B + O; C/K not inspected | meta-ads-v1.md §7 HTTP (public feed); catalog-inventory-v1.md §Meta catalog feed; internal/attribution/feed.go:100 (GET/HEAD transport) |
| `HEAD /v1/buyer/feeds/meta.csv` | B + O; C/K not inspected | meta-ads-v1.md §7 HTTP (public feed); catalog-inventory-v1.md §Meta catalog feed; internal/attribution/feed.go:100 (GET/HEAD transport) |

## 2. Current code-derived behaviour (owner review)

All three sections are **documented-from-code 2026-10-15**. Source locations refer
to this unit's implementation; subordinate service/type pointers supplement the
handler pointers where the handler delegates. No handler behaviour is changed.

### Shared transport for §2.1–§2.3

- No query (including a bare `?`), Cookie, Origin, X-Tenant-ID, X-Store-ID or
  off-route claim token: 403 `forbidden` (`internal/buyerhttp/handler.go:145`,
  `:311`). Escaped-path aliases: 404 `not_found` (`handler.go:315`). Wrong/missing/
  duplicate or noncanonical BFF key: 401 `unauthorized`; absent/duplicate/empty
  storefront origin: 422 `invalid_request` (`handler.go:319`, `:324`).
- Known path with any method other than its indexed method: 405
  `method_not_allowed`; noncanonical order UUID: 422 `invalid_request`
  (`handler.go:334`, `:338`). K must be absent for the two GETs, and exactly one
  matching `^[A-Za-z0-9_.:-]{8,128}$` for proof PUT; otherwise 422
  `invalid_request` (`handler.go:39`, `:268`, `:342`).
- Privacy/proof require one canonical Bearer; invalid/missing/duplicate Bearer:
  401 `unauthorized`. Primary-origin refuses any Authorization: 403 `forbidden`
  (`handler.go:391`, `:358`, `:365`; `catalogv2.go:52`). Origin resolves before
  dispatch; unpublished/unknown origin is 404 `not_found`, invalid origin 422
  `invalid_request` (`handler.go:372`, `:749`). View-only sessions cannot read
  privacy or submit proof (403 `forbidden`; lookup failure 503 `unavailable`)
  (`handler.go:379`; `internal/buyerhttp/lookup.go:172`, `:191`).
- Shared 10-second context/read/write deadlines (`handler.go:305`); errors use
  the safe JSON envelope, no-store and nosniff via `New`'s middleware
  (`handler.go:75`; `internal/httperror/error.go:25`). Generic mappings: 401
  `unauthorized`, 429 `rate_limited`, 422 `invalid_request`, 403 `forbidden`,
  404 `not_found`, 409 `insufficient_inventory`/`conflict`, otherwise 503
  `unavailable` (`handler.go:749`). These are error mappings, not a claim that
  each delegate can generate every sentinel. Success JSON is 200
  (`handler.go:779`); none of these handlers redirects (`primary.go:33`,
  `privacy.go:55`, `handler.go:666`).

### 2.1 Privacy read — documented-from-code 2026-10-15

The indexed privacy GET takes no request fields; it reads the authenticated
scope through `customers.ReadBuyerPrivacy` and returns
`{consents:{marketing_messages:bool,ads_personalization:bool},erased:bool}`
(`internal/buyerhttp/privacy.go:45`, `:55`; `internal/customers/types.go:80`,
`:177`). It drops the service's `store_name` (`internal/customers/privacy.go:141`).
The handler does **not** call `noBody` or `decodeJSON`: supplied body bytes are
not read, and it enforces no body-size limit (`privacy.go:45`).
Errors delegate to `privacyError` (`privacy.go:49`, `:172`), which maps its
sentinels to 409 `idempotency_conflict`/`erasure_blocked`/`export_too_large` or
410 `erased`; other errors use the shared mapping above. These include the
shared 401/403/404/422/503 paths; a successful 200 is written only after the
context check (`privacy.go:52`). This section records that mapper, without
claiming that all write-only privacy sentinels occur on the read operation.

### 2.2 Primary-origin lookup — documented-from-code 2026-10-15

The indexed primary-origin GET uses only the verified origin. It calls
`noBody`: any positive Content-Length or observed body byte is 422
`invalid_request`, so the allowed body size is zero bytes
(`internal/buyerhttp/primary.go:20`; `handler.go:680`). It calls
`storefrontdomains.PrimaryOrigin` and returns 200 `{primary_origin:string|null}`
(the empty service result becomes null); it returns JSON, not a redirect
(`primary.go:24`, `:29`, `:33`; `internal/storefrontdomains/service.go:209`).
Errors from that call use the shared mapping, including invalid input 422
`invalid_request`, unknown/unpublished origin 404 `not_found` at the preceding
resolver, and unclassified dependency/timeout failures 503 `unavailable`
(`primary.go:26`; `handler.go:372`, `:749`). The shared BFF/Authorization/method/
query/K refusals apply; success inherits no-store (`handler.go:75`;
`internal/httperror/error.go:36`).

### 2.3 Transfer proof — documented-from-code 2026-10-15

The indexed proof PUT takes the canonical order UUID and strict JSON
`{last5:string,amount_minor:int64,paid_at:RFC3339 timestamp}`; the handler decodes
`checkout.ProofInput` then calls `SubmitTransferProof` with server-resolved store,
Bearer and K (`internal/buyerhttp/transfer.go:30`, `:35`, `:39`;
`internal/checkout/transfer.go:61`). Last5 is exactly five ASCII digits; amount is
1..1,000,000,000,000; paid_at must be nonzero and is normalized to UTC microseconds
(`internal/checkout/transfer.go:77`, `:86`, `:139`).

Exactly one Content-Type parsing to application/json is required (415
`json_required` otherwise). Body limit is 64 KiB (Content-Length or observed
excess: 422 `invalid_request`); missing/malformed JSON, explicit null anywhere,
unknown fields, wrong types and trailing JSON values: 400 `invalid_json`
(`internal/buyerhttp/handler.go:37`, `:701`). JSON decode does not independently
require each key; missing fields reach service validation and fail 422
`invalid_request` (`transfer.go:35`; `internal/checkout/transfer.go:139`).

Success is 200 `{order_id,state:"SUBMITTED",proof_count,submitted_at}`; a
malformed SQL projection becomes conflict (`internal/buyerhttp/handler.go:525`,
`:654`, `:666`; `internal/checkout/transfer.go:68`, `:161`). The service uses
buyer scope and the SQL definer for ownership/window/state/idempotency, then
rechecks capability (`internal/buyerhttp/transfer.go:39`;
`internal/checkout/transfer.go:157`). Errors pass through checkout `safeError`
and the shared HTTP mapping: 401 `unauthorized`, 422 `invalid_request`, 404
`not_found`, 409 `conflict` (including a refused state/replay), and dependency/
timeout failures 503 `unavailable`; shared transport/view-only refusals above
also apply (`transfer.go:39`; `internal/checkout/transfer.go:168`;
`internal/checkout/checkout.go:638`; `internal/buyerhttp/handler.go:749`).

The proof service also propagates coded SQL refusals: 422 `not_bank_transfer`,
`transfer_not_open`, `transfer_window_closed`, `invalid_proof` (the service's
explicit list at `internal/checkout/transfer.go:136`; transport at
`internal/buyerhttp/transfer.go:39`, `internal/buyerhttp/cvs.go:62`,
`internal/checkout/checkout.go:659`, `:844`). A coded PT429 is preserved as
429 with Retry-After, and coded PT409 as 409; these are mapper capabilities,
not evidence of a new proof rate limit (`handler.go:388`; `checkout.go:844`).
