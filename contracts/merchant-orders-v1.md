# Merchant order reads v1

Frozen after independent preflight of `a8aba0c`, 2026-09-25 (research
`dee6530d-791e-4647-b320-57f420413b6f`, decision
`9ed17103-ff27-43a3-9c4d-d593babd58dc`). Implemented and accepted as a bounded
local backend increment at `25d4303`; [MOR01–06 evidence](../docs/implementation/2026-09-25-merchant-orders-acceptance.md).
This delivery is an authenticated merchant list/detail read, not shipment creation,
manual payment confirmation, refunds, or a replacement for buyer capability APIs.
The admin UI remains a subsequent consumer of this frozen transport contract.

## Authority and forward migration

Add `orders:read` to the existing grant vocabulary in a new migration 0027.
New initial-store creators receive it in the existing onboarding transaction,
preserving the latest 0019 function's locks, replay behavior and other grants.
Do not backfill existing memberships or regrant on onboarding replay. Existing
stores require an explicitly authorized permission grant during rollout; neither
catalog/inventory permission nor historical creator identity implies this right.

Reuse `commerce_auth` and the existing `identity.resolve_access` pattern. Add one
VOLATILE SECURITY DEFINER function, owned by that NOLOGIN role with fixed
`search_path=pg_catalog`, PUBLIC revoked and EXECUTE only to `commerce_runtime`:

`identity.read_merchant_orders(hash bytea, store uuid, order_id uuid,
 row_limit integer, after_created_at timestamptz, after_id uuid, state text)
 RETURNS jsonb`.

`order_id IS NULL` means the bounded list; otherwise detail requires row_limit=1,
no cursor and state='all'. List row_limit is 1..101 (service requests limit+1).
State is exactly all/DRAFT/AWAITING_PAYMENT/CONFIRMED/CANCELLED. Both cursor parts
must be absent or present together; detail returns an array containing one row,
or PT404. No dynamic SQL, writes, row locks or second domain state machine.

Only grant commerce_auth the columns actually used on checkout.orders (including
its frozen snapshot), checkout.payment_attempts, payments.facts/review_cases and
fulfillment.payment_work_items; add SELECT policies for this private reader.
Grant that owner USAGE on exactly checkout, payments and fulfillment schemas.
Preserve the existing 0018 runtime grants on facts/review/work; this delivery's
no-direct-access boundary is checkout orders/attempts, not a retroactive revocation.
Do not grant commerce_runtime direct checkout schema/table access. The function
independently verifies token/store/orders:read, matches resolved tenant/store/
principal against transaction GUCs as text, and rejects non-READ-COMMITTED calls.
Use one data SELECT statement for the whole page/detail: orders, attempt,
financial facts, review/work and frozen detail projection share one snapshot.
Do not assemble them with successive inner queries that could straddle capture.
After its data query, use a direct auth SELECT inside this VOLATILE function at
READ COMMITTED (fresh internal statement snapshot), checking merchant session,
principal, tenant/store/membership active, store:read, orders:read and the same
principal/tenant/revision, with session expiry against `clock_timestamp()`.
Do NOT reuse `resolve_access` for this final SQL fence: it is STABLE and uses
`statement_timestamp()`, so a nested second call can retain pre-wait authority.
Preserve existing distinctions: invalid session/principal -> PT401; invisible or
inactive scope/store:read -> PT404; missing orders:read or changed revision ->
PT403. Do not globally change legacy auth behavior in this delivery. Go also
checks RequirePermission against its original Scope after the read.
The fresh final SQL auth fence runs before any empty-list/detail-not-found branch
or return, so revoked callers cannot distinguish existing and missing orders.
No fabricated Scope/GUC, wrong-store token or raw API header is authority.

Add an index on checkout.orders(tenant_id,store_id,created_at DESC,id DESC).
Do not alter buyer RLS, existing payment/stock transitions or old migrations.

## Service and HTTP contract

Reuse platform scoped transactions, error envelope, pagination.Page and token
transport. Add internal/merchantorders (not a second checkout service):
`List(ctx,tx,scope,token,ListRequest) (pagination.Page[Summary],error)` and
`Get(ctx,tx,scope,token,orderID) (Detail,error)`.

- GET `/v1/admin/stores/{store_id}/orders`: optional `limit` (default50, 1..100),
  `cursor` (max1024), `state` (default all). Keyset order created_at DESC,id DESC.
  Collection merchant-orders uses existing pagination envelope bound to exact
  tenant/store/state. Two keys are canonical UTC microsecond timestamp then UUID;
  explicitly extend the existing collection/key validator for this pair (the
  current UUID-only validator cannot accept this cursor without that change).
  malformed position, cross-store/state and other collection cursors are rejected.
  Newer insertions do not restart an existing page; state changes may change
  membership between pages, so this is not a point-in-time export.
- GET `/v1/admin/stores/{store_id}/orders/{order_id}`: canonical UUID, no query.
- GET only (HEAD rejected). Reject bodies, Transfer-Encoding, Idempotency-Key,
  unknown/duplicate/malformed query fields, trailing bare `?`. No cache/storage
  of address responses; use existing private/no-store response headers.
- Empty list is items:[],next_cursor:"". Missing or other-store order is the same
  not_found envelope. Unauthorized/forbidden remain distinct per existing policy.
  Database/malformed projection fails closed with sanitized unavailable error.

Summary has exactly order_id,created_at,updated_at,currency,total_minor,
commercial_state,fulfillment_state,payment_state,test_mode,work_state.
Fulfillment states are MANUAL_UNASSIGNED, CANCELLED and PAID_ALLOCATION_FAILED;
preserve the latter as a separate allocation failure, not ordinary fulfillment.
No buyer owner/session/cart IDs, address, phone, raw snapshot, PSP reference,
connection/credential/attempt IDs, keys, hosted form or reconciliation payload.

Payment state uses the same precedence as existing buyer payment view:
NOT_STARTED (no attempt), REVIEW_REQUIRED (sticky review exists), CAPTURED,
AUTHORIZED, else PENDING. Facts match the original attempt's tenant/store,
amount/currency/connection/profile/environment. Do not infer paid from commercial
state, an HTTP result, browser return, or work existence. test_mode is false if
no attempt, otherwise profile/environment other than LIVE; false with NOT_STARTED
does not mean live payment is enabled. work_state is NONE/READY/REVIEW_REQUIRED
from durable payment_work_items, separately from commercial/fulfillment state.

Detail extends Summary with country,service_code,items,totals,destination:
- items in frozen quote order: sku_id,code,name,quantity,unit_price_minor,amount;
  amount is the existing pricing.LineAmount (subtotal/discount/tax/total_minor).
- totals: subtotal_minor,discount_minor,shipping_minor,shipping_tax_minor,
  tax_minor,total_minor (no duplicate pricing.Calculation.lines array).
- destination: kind,country,recipient_name,phone,home_address (existing five
  address strings),pickup (null or kind,namespace,code,name,address,
  verification_kind). Values are frozen at checkout; a current catalog edit,
  address selection, store rename or pickup revocation never rewrites history.
  Pickup codes remain strings, including leading zeroes. No evidence refs or
  private verification documents. Merchant attestation is not official carrier
  eligibility; stale snapshot dates do not turn this read into admission.

Use strict bounded typed JSON decoding (list <=128KiB, detail <=256KiB), canonical
IDs/times, known state enums and existing integer/money bounds. A missing,
malformed or contradictory projection fails the whole read, not partial success.
Render these strings as text in the future UI; no raw HTML or automatic exports.

## Acceptance gates (MOR01–06)

1. New permission valid; fresh real onboarding grants it once. Replay does not
   restore a removed grant; preexisting principals are not auto-elevated.
   Exact private function ownership/ACL/search_path and minimal column grants;
   required owner schema USAGE and no new runtime checkout USAGE/SELECT;
   buyer/worker/identity/hosted/PUBLIC callers cannot execute or read new data.
2. Real two-tenant, same-tenant two-store and two-owner order fixtures. Merchant
   sees all authorized-store buyers and no other store. No-address summary keys
   exact; forged Scope/GUC, wrong token/permission/audience/session/revision fail.
3. Real HTTP list/detail; finite states; stable equal-time UUID tie keyset;
   limit+1, cursor/filter/store mismatch, duplicates/unknown/bodies/HEAD rejected.
   Missing detail is indistinguishable from other-store; safe errors/no secrets.
4. Real DRAFT/unpaid, AWAITING_PAYMENT/pending, authorized, captured READY,
   sticky review and cancelled expired states via existing business/worker APIs.
   Include real PAID_ALLOCATION_FAILED with review work and separate states.
   History retains original items/totals/recipient/pickup leading zeroes after
   mutable catalog/destination edits. No manual provider-result fabrication.
5. Real blocked data-query witness (pg_stat_activity/pg_blocking_pids), then
   session expiry/revocation, grant removal, tenant/store/membership disable or
   revision change before release: final fresh SQL auth rejects without returning
   data, including direct function invocation without the Go final fence.
   Include missing/other-store detail and empty-list cases after revocation.
   Read counts leave orders/attempts/facts/stock/receipts/events/queue unchanged.
6. Independent source/security and independent PG tests, root repeat focused
   plus full Go/PG/race/vet; docs/dependencies/graph/fixture cleanup. Existing
   buyer and worker gates stay intact. No production/customer/provider writes.

## Limits and rejected alternatives

No merchant UI claim in the backend gate; real page, mobile, locale and session
switch tests are required when the UI is added. No shipment/label, fulfillment
state write, manual paid flag, customer export, search service or data warehouse.
Dedicated permission + one safe projection avoids exposing raw checkout rows or
reusing a buyer token. No new database, role service or cross-module HTTP call.
Upgrade signals: real order volume/query plans require measured index tuning;
multi-attempt/refund features must deliberately extend the read-state contract.

## Amendment: cash-on-delivery fields (home-cod R5, migration 0107; amends this contract)

The merchant must see the cash a cash_on_delivery order collects (owner ruling, home-cod P1-2), so the safe projection and the
`commerce_auth` column list grow by exactly the fields below; nothing else of the COD data model is exposed.

- Summary/detail keys, always present, JSON `null` unless `payment_mode='cash_on_delivery'`: `cod_surcharge_minor` (the placement-time
  whole-TWD surcharge, 0..100000, multiple of 100, never folded into `total_minor`) and `cod_collect_minor` (= `total_minor` +
  `cod_surcharge_minor`, the cash due on delivery). The strict Go validator refuses a COD row whose collect amount is not that sum.
  The summary key set is therefore the earlier list plus these two (`collection_state`, `payment_mode`, `pickup_source` and
  `refunded_minor`/`refund_pending_minor` were added by 0073/0063; `source` by 0094 on the HTTP row).
- `commerce_auth` SELECT on `checkout.orders` gains exactly `cod_surcharge_minor` (the two keys above and the finance COD column) and
  `collected_at` (the Asia/Taipei finance day of collected COD and pay-at-pickup cash in `identity.read_finance_summary`, which this
  role owns; it is set once by `fulfillment.record_collection` and never read for a merchant DTO key). `cod_carrier` is NOT granted:
  the merchant DTO carries no carrier key and no `commerce_auth` function reads it. Least privilege: any further column needs a new amendment.

## Amendment: orders-v2 domain read boundary (unpublished 0110, ACL ruling)

The authenticated v2 reader retains its initial and final `orders:read` fences.
Its live placement provenance and CVS tracking searches use three internal read
helpers, following `checkout.has_inflight_collection` (CB03), not cross-domain
table grants or new RLS policies for `commerce_auth`:

- `claims.order_live_sources(tenant, store, order_ids uuid[])`: distinct order/session IDs,
  owned by `commerce_claims_writer`. No actors, claim lines, bundle owners or quantities.
- `live.order_session_labels(tenant, store, session_ids uuid[])`: ID, title and creation
  time (for the existing latest-100 session choices), owned by `commerce_media_writer`.
- `fulfillment.order_cvs_tracking(tenant, store, order_ids uuid[])`: latest attempt's
  order ID, state, shipment number and provider tracking ID, owned by `commerce_checkout_writer`.

Each is STABLE, SECURITY DEFINER, `search_path=pg_catalog`, with explicit tenant/store
and requested-ID predicates; PUBLIC is revoked and only `commerce_auth` receives
EXECUTE. Runtime and buyer logins cannot call them. Empty/null ID arrays return no rows.
The authenticated reader supplies server-derived scope and batches IDs per statement.
Existing pre-0110 customer/CVS metadata grants remain unchanged; no additional direct
access to the consumption ledger, session table or CVS tracking columns is allowed.
KC03 explicitly enumerates the new claims helper, including its owner's implicit
EXECUTE; its frozen table/column ACLs, LPC06 and TCV02 remain unchanged.
