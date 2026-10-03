# Orders v2 — read-only R5 A3

Scope: additive `GET /v1/admin/stores/{store}/orders?view=v2` (same scoped BFF),
and read-only `POST /v1/admin/stores/{store}/orders/search?view=v2` for private search.
The unversioned list and detail remain v1. No money, inventory, shipment or state
writes change. Owner brief supersedes v1's no-search/no-count UI limitation.

## Request / authority

`limit` 1..100, `cursor` as before. `state` accepts legacy states plus `active`
(the v2/UI default: excludes DRAFT); explicit `all` includes drafts for historical
inspection. `bucket`: all (default), unpaid, transfer_review, ready_to_ship,
ready_to_consign, shipped, completed, cancelled. `q`: trimmed 1..80 characters,
literal case-insensitive substring for order reference/recipient/frozen SKU,
exact phone suffix only when exactly four ASCII digits, exact current tracking
number (manual current SHIPPED head; CVS highest attempt's shipment_no or
provider_logistics_id while CREATED/AT_DC/AT_STORE/PICKED_UP/UNCLAIMED).
No wildcard syntax. `payment_mode`: card/pay_at_pickup/bank_transfer/
cash_on_delivery; `delivery`: home/cvs_711/cvs_familymart/cvs_hilife/cvs_okmart;
`session_id`: canonical UUID; `from`, `to`: inclusive ISO calendar dates in
Asia/Taipei. Empty, duplicate, unknown, malformed and inverted fields fail 422.
All filters (including bucket) are bound into the scoped keyset cursor digest.
GET has no body; q is forbidden in URLs. Private search uses POST with the sole
JSON field `{q: string}` (1 KiB bound), other filters in its query. It executes
the same SQL reader and creates no command/job/effect. No Idempotency-Key on
either read. BFF POST requires the normal origin/CSRF checks. Search text lives
only in mounted UI memory; refresh clears it, while non-sensitive filters
persist in the URL. Never log request bodies. Tenant/store/principal only from auth;
the same fresh final SQL revocation fence as v1 applies, even for an empty result.

## Response

`{items, next_cursor, total, counts, sessions}`. Items extend the existing strict
summary with `order_number`, `recipient_masked`, `delivery_kind`, `live_sessions`.
Order number is the collision-free readable `LC-` + complete uppercase UUID hex,
not a fabricated short/sequence number. Recipient exposes only the first Unicode
character plus `***`; no phone/address/snapshot is returned in list responses.
Legacy missing/blank recipients use `—`; unrecognized delivery projections use
`delivery_kind: "unknown"` with a localized UI label. These are display-only row
fallbacks, not new accepted filter values. Transaction and authority validation
remain strict, and valid rows/counts are not discarded because of a display gap.
`live_sessions` and `sessions` contain `{id,name}`. Sessions are only those proved
by this store's orders' durable live-price-use -> bundle -> session chain, not
ad/visitor attribution. Multiple sessions per order are retained. Missing proof
leaves existing storefront/merchant_manual source intact; the UI explains scope.

`total` is the SQL count after all filters including bucket, before cursor/limit;
it stays stable across pages absent actual data changes. Counts has all eight bucket keys and
is calculated before bucket/cursor but after other filters, in the same snapshot
as the rows. Counts are overlapping task queues, not a sum of exclusive states.
The sessions picker contains store-scoped order-linked sessions, independent of
the selected filters, and never exposes actors. It is bounded to 100 entries;
old selected session IDs remain valid even if not among the latest 100 choices.

## Bucket predicates (read-only, no new state machine)

- unpaid: non-draft/non-cancelled online-card orders not commercially CONFIRMED,
  bank transfers AWAITING_TRANSFER without a submitted proof, or COD/PAP with
  collection_state=PENDING. These task queues deliberately overlap shipping.
- transfer_review: AWAITING_TRANSFER with the current bank transfer SUBMITTED.
- ready_to_ship: CONFIRMED/AWAITING_COLLECTION + MANUAL_UNASSIGNED and the
  existing `manual_shipment_eligible` authority predicate.
- ready_to_consign: PROVIDER_LABEL_CREATED with current CVS state CREATED.
- shipped: MERCHANT_SHIPPED or current CVS AT_DC/AT_STORE/PICKED_UP (shipment is
  a fact and remains true after completion).
- completed: paid-and-picked-up CVS, or COLLECTED COD/PAP. Manual prepaid home
  shipments have no delivery confirmation and are not inferred complete.
- cancelled: commercial CANCELLED, preserving allocation-failure detail.

## Gates

10k synthetic rows on real isolated PG; last4 and current tracking queries each
<1s (10 measured HTTP reads), exact SQL counts, same-tenant and cross-tenant
isolation, scoped/filter-bound cursors, invalid requests, no full list PII,
read-only fingerprints, role ACLs and a blocked-query revocation fence. Record
pre-implementation RED then GREEN. Preserve MOU01–06 assertions; legacy draft
inspection explicitly selects all rather than silently relying on defaults.
The reader is created with `jit=off` (one statement costed above PG's JIT thresholds; JIT compile was
0.3-0.7 s of every call, the cause of the intermittent >1 s on the 10k gate).
Add real-click MOU07, all controls, refresh persistence, three locales at
390/1586, screenshot/click ledger. Run requested node/tsc/gates/depmap/focused PG.
