# Buyer checkout options v1

Status: FROZEN after independent preflight (no open P0/P1/P2), 2026-09-25.
Implementation `83e5ed6` accepted: CO01–CO05 bounded private HTTP/PG gates,
353-test full regression and independent review passed. [Evidence and limits](../docs/implementation/2026-09-25-buyer-checkout-options-acceptance.md).
This joins the existing
market, pricing and fulfillment configuration into buyer-safe choices. It is a
dependency of automatic product payment entry, not payment or carrier acceptance.

## Single surface and existing authority

Add `checkout.Service.ListOptions(ctx, token, storeID, OptionsRequest)` returning
`pagination.Page[Option]`, and private `GET /v1/buyer/checkout-options`.
Use the existing validated checkout-runtime pool and `buyer.WithScope`, as the
existing order `Get` does. No new pool, role, grant, migration or dependency.
Do not call admin discovery functions with fabricated merchant credentials.
Check scope before SQL and recheck the capability after reading. Translate errors
through existing checkout safeError; do not expose SQL, tokens or internal data.

OptionsRequest has optional MarketID/Country and pagination.Request. Validate
canonical UUID, uppercase two-letter country, limit default50/max100 and bounded
cursor. Query uses authenticated tenant/store only; foreign, absent, inactive or
ineligible configurations all produce the same empty page, never ownership clues.
Empty items serialize as `[]`, terminal next_cursor as `""`.

The exact concrete row fields are:
`market_id,market_code,market_name,country,currency,delivery_code,method,service_version,allocation_version,delivery_kind,mode,name_hans,name_hant,name_en,sort_order`.
Use a separate explicit HTTP projection; no embedded merchant DTO. Market labels
are current saved data, not invented translations. Delivery labels preserve the
merchant's three saved languages. `method` is the actual pricing policy method;
versions are the current service and allocation revisions used by checkout.
Do not expose warehouse IDs/counts, actors, binding IDs, credential versions,
provider accounts, configuration references, addresses, stock or payment URLs.

## Effective configuration, not a reservation

One parameterized ordinary-role SELECT joins these existing authoritative rows
using full tenant/store/natural keys:

- Active market and current store currency, equal to service/policy currency.
- Service head's current version: enabled, visible, MANUAL, no binding; permitted
  kinds home/cvs_711/cvs_familymart, with CVS only for TW.
- Policy head's current enabled version matching service.policy_method and
  service.policy_version, country and currency. Legacy bare shipping policy keys
  without a delivery service are not exposed: Begin cannot consume them.
- Allocation head's current revision, 1..16 warehouse_count, exact matching
  number of child rows and all selected warehouses active in the same store.
  Do NOT require allocation.service_version == current service.version: that
  saved value is provenance/write CAS, intentionally survives rename/fee changes
  (delivery-allocation-v1). Empty/cleared or inactive-warehouse configs disappear.

Use one statement snapshot and no FOR UPDATE/SHARE or inventory-lock functions.
No writes, audit events, jobs, stock reservations or provider calls. Lack of SKU
stock is NOT assessed here. A row is selectable configuration, never an
`available=true`, final fee/tax, carrier verification, or fulfillment guarantee.
CreateQuote and Begin must still revalidate prices, config, destination and stock.
Stale versions received between reads are normal conflicts, not silently upgraded.

CVS discovery supplies a delivery choice, NOT a trusted pickup_id. The separate
pickup-source/selection flow remains required before CVS destination creation;
this increment must not claim end-to-end CVS checkout or invent an attestation.

## Payment modes follow what can be paid (ops-polish OP1, amends this contract)

`card` is listed in `payment_modes` (and a home row is offered at all) only while the process
has a hosted buyer payment service (cmd/api `COMMERCE_BUYER_PAYMENT_ENABLED`); a CVS row then
offers `pay_at_pickup` alone when the store enabled it, and a row with no mode is not offered.
`checkout.Service.Begin` refuses `payment_mode=card` with coded 422 `card_unavailable` when the
same condition holds (after the receipt lookup, so an exact replay still returns its order).
The per-store method check stays order-bound in `checkout.hosted_payment_view*`; this rule is
the process-level gate and is not a per-store guarantee.

## Bank transfer joins the payment modes (checkout-offline, amends this contract; storefront-v2 §C)

When the store enabled bank transfer (`checkout.bank_transfer_settings`), `bank_transfer` is listed in `payment_modes` of every home row
and, with `allow_cvs`, of every CVS row, together with `transfer_window_hours` (6..168, the hold the order will keep). A home row
carries `payment_modes` only in that case (absent = card only, as before) and a home row is then offered even without card payment.
`pay_at_pickup` is never listed on a home row. `checkout.Service.Begin` places the order `AWAITING_TRANSFER`; SQL refuses a disabled mode
with coded 422 `bank_transfer_unavailable` (zero holds). Bank details are never part of the options: they appear only on the buyer's own
order (`GET /v1/buyer/orders/{id}/bank-transfer`).

## Stable bounded pagination and query admission

Keyset order is `(market_id UUID, country COLLATE C, delivery_code COLLATE C)`
ascending; use limit+1 then trim, cursor at the last included row. Optional
market_id/country filters are usable separately or together. sort_order is only
display metadata, not a cursor key; consumers finish the relevant filtered pages
before applying presentation ordering. Concurrent configuration edits can change
eligibility; cursors are not immutable snapshots or authority.

Cursor is strict canonical base64url JSON containing version1, SHA256 binding
of unambiguous collection+authenticated tenant/store+both filters, and the three
last-position fields. Max1024 encoded bytes; reject unknown/duplicate/missing/null
fields, wrong types/version/binding, noncanonical encodings/UUID/country/code.
Digest binds position only, not a signature: forging position can skip results,
never change the scoped SELECT. No raw tenant/store IDs or bearer in the cursor.
Do not reuse admin pagination.Encode, which contains raw tenant/store fields.

Only this exact canonical GET joins catalog in the narrow query allowlist.
Max raw query2048; exact keys market_id,country,limit,cursor once/nonempty;
reject bad escaping, semicolons, empty separators/keys, unknowns and invalid
values. Decimal limit1..100 only. Bare `?` remains forbidden. Old routes retain
all query restrictions. GET body and Idempotency-Key forbidden; POST405,
trailing path404; existing private BFF secret, exact published-origin resolution,
buyer capability, forbidden Cookie/Origin/scope headers, deadline, no-store and
safe error behavior remain. Browser input never chooses a tenant/store.

## Acceptance gates (bounded PASS; evidence above)

- CO01 real private HTTP + PostgreSQL ordinary roles: rows/three languages/exact
  fields, multi-page stable traversal, each optional filter, foreign scope empty,
  scope/filter/collection cursor replay rejected and positive cross-store control.
- CO02 current eligibility readback: disable/hide/API draft, policy advance then
  matching service revision, inactive market/current-currency mismatch, empty
  allocation, inactive warehouse and newer service with historical allocation
  provenance. Preserve old fixtures' empty negative controls with fresh IDs.
- CO03 consume returned market/country/method/version fields through real HTTP
  cart -> Quote -> home destination -> Begin -> Get. No copied known config IDs
  in the consumer; no PSP call. Count reads before/after: no purchase facts/jobs.
- CO04 strict input/cursor/body/idempotency/method and auth/publication/revocation
  negatives; old catalog/query boundary regression and unit exact projection.
- CO05 full real PG/race/vet, independent source/evidence review, code graph and
  acceptance record. Public browser/PSP/CVS source gates explicitly NOT_RUN.

## Free-shipping threshold joins every row (storefront-integration, amends this contract; storefront-v2 §C)

Every option row carries `free_shipping_threshold_minor`: the row's delivery policy threshold in minor units (`pricing.policy_versions`, migration
0088), or `null` when the policy has none or the threshold is 0 (always free: nothing to hint). The key is always present. It is a display hint
for the cart and delivery step ("add X more for free delivery"); `pricing.Calculate` in the server quote stays the only authority on shipping,
so a stale hint can mislead wording, never the amount charged (I05).
