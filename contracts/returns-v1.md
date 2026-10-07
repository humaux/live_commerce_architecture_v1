# returns-v1 — merchant returns (RMA, minimal) and merchant order cancel

Unit W3-08B (`unit/w3-08b-returns`), migration `0155_returns.sql`. Status: IMPLEMENTED on REAL_PG (evidence class `REAL_PG` with MOCK Stripe fakes; no
LIVE or SANDBOX provider is touched). Supersedes the "no cancel, no restock" text of `stripe-refund-v1.md` R-3 / M-8 (marked RESOLVED there).
Reads: `stripe-refund-v1.md` RD6, `manual-fulfilment-v1.md` §2 / Amendment W3-07B, `taiwan-cvs-logistics-v1.md` §16.8, invariants I02, I03, I05, I13.

## 1. Scope

In: a merchant registers a return of a SHIPPED order, receives it, inspects it (sellable / scrap per line) and closes it; sellable units go back on
sale. A merchant cancels an UNSHIPPED order (unpaid hold, paid card order after a full refund, pay-at-pickup / COD through the existing release).
Out (non-goals): buyer-side return request, return label, exchange / re-ship, partial cancel, automatic refund, lost-parcel claim, multi-parcel.

## 2. Return (RMA)

State machine (`returns.rmas.state`): `REGISTERED -> RECEIVED -> INSPECTED -> CLOSED`; `REGISTERED -> CANCELLED`. Every step carries `Idempotency-Key`
and `expected_version` (CAS; a stale version is `409 version_changed`, a wrong state `409 invalid_state`; the replay of a stored key returns the stored
body BEFORE both checks). Version starts at 1, +1 per transition.

Lines (`returns.rma_lines`) are keyed `(warehouse_id, sku_id)`: an order has no line-id table, its lines are `inventory.reservation_lines`. A request names
`sku_id` and optionally `warehouse_id` (needed only when the order holds the SKU in two warehouses; else `422 ambiguous_line`). Columns:
`qty_registered`, `qty_received`, `qty_restock`, `qty_scrap`; once inspected `qty_restock + qty_scrap = qty_received <= qty_registered` (table CHECKs).

* **Register** (`fulfillment:write`): order must be shipped (`MERCHANT_SHIPPED`, or `PROVIDER_LABEL_CREATED` with a CVS parcel `PICKED_UP`) and returnable
  (`returns.order_returnable`, shared by register, close and the ledger guard): `CONFIRMED` with `collection_state` NULL or `COLLECTED`, or a `cash_on_delivery` order that is
  `COLLECTED` while its `commercial_state` is still `AWAITING_COLLECTION` (COD keeps that state after collection by design); an uncollected COD order is `409 not_returnable`;
  else `409 not_shipped` / `409 not_returnable`. Per line, the sum of `qty_registered` over every
  non-cancelled RMA (CLOSED ones included: returned units cannot be returned again) is `<= reservation_lines.quantity` (the shipped quantity; shipments are
  whole-order) else `422 exceeds_shipped`. The order row lock serializes registrations of one order.
* **Receive** (`fulfillment:write`): every line named exactly once, `0 <= qty_received <= qty_registered`, at least one unit (`422 nothing_received`).
* **Inspect** (`fulfillment:write` AND `inventory:write`): every line once, `qty_restock + qty_scrap = qty_received` (`422 quantities_mismatch`). Records the
  decision only; no stock moves.
* **Close** (`fulfillment:write` AND `inventory:write`): INSPECTED -> CLOSED and the stock write of section 5. Optional `refund_id` must be a
  `payments.stripe_refunds` row of the SAME order (`422 refund_mismatch`); it is only stored on the RMA, nothing in `payments.*` is read for effect or written.
* **Cancel RMA** (`fulfillment:write`): REGISTERED only; frees its quantity.
* Reads (`orders:read`): `GET /orders/{id}/returns`, `GET /returns?state=` (100 newest). Another store's order or RMA is `404`.

## 3. Merchant cancel (`POST /orders/{order_id}/cancel`, `fulfillment:write`)

Body `{expected_state, reason}` (`expected_state` in DRAFT, AWAITING_PAYMENT, CONFIRMED, AWAITING_COLLECTION; the orders table has no version column, so the CAS is the
commercial state the merchant saw; mismatch `409 state_changed`; `reason` 1..240 chars, echoed in the stored response). Decision order (first match wins):

| Current order | Result |
|---|---|
| `CANCELLED` | `409 already_cancelled` |
| `AWAITING_PAYMENT` | `409 payment_in_flight` (a payment attempt exists; the payment worker closes it, never the merchant) |
| `MERCHANT_SHIPPED` / `PROVIDER_LABEL_CREATED` (or a live CVS parcel) | `409 already_shipped` (use the returns path); a CVS create in flight `409 cvs_attempt_in_flight` |
| other `fulfillment_state` (e.g. `PAID_ALLOCATION_FAILED`), `AWAITING_TRANSFER` (accepted as `expected_state`), bank transfer | `422 not_cancellable` (bank transfer ends by expiry or the offline-refund path) |
| `pay_at_pickup` / `cash_on_delivery`, `collection_state=PENDING` | delegates to `inventory.release_pay_at_pickup(cancel)` (taiwan-cvs-logistics §16.8), unchanged |
| `DRAFT` (unpaid hold) | reservation `HELD -> RELEASED`; one `RELEASE` ledger row per line |
| `CONFIRMED` card, unshipped | `409 has_returns` if a live RMA exists; `409 payment_review_open` if an open payment review case exists; **`409 refund_first` unless the refunds not failed/cancelled/rejected (succeeded + in flight) sum to at least the CAPTURED amount**; then order `CANCELLED/CANCELLED`, reservation `COMMITTED -> RELEASED`, one `DEALLOCATE` row per line, and the order's `READY` `fulfillment.payment_work_items` row is DELETED in the same transaction (Amendment 0162 below; a `REVIEW_REQUIRED` work item is never touched — such an order is `422 not_cancellable` before this point anyway) |

The cancel NEVER starts a refund and never writes `payments.*` (I05, I13). The existing notify trigger enqueues the buyer's `cancelled` notice for a
`CONFIRMED -> CANCELLED` transition; no new mail template. Audit `orders.merchant_cancelled`.

**Refund fails after the cancel.** A cancel counts refunds in flight. If one later FAILS the order stays CANCELLED (stock released: the goods never left) and the buyer's money is held:
`GET /orders/cancel-refund-gaps` (`orders:read`) lists the merchant-cancelled card orders whose non-failed refunds are below the captured amount (`{order_id, captured_minor, refunded_minor, gap_minor,
cancelled_at, reason:"cancel_refund_failed"}`); the merchant refunds again on the cancelled order (the refund route has no order-state gate) and the row disappears. The gap list is computed from
`payments.facts` / `payments.stripe_refunds` / `payments.refund_facts` and the `checkout.merchant_cancel` DEALLOCATE ledger row only — it does NOT read the work item, so closing the work item at
cancel time never hides a gap (Amendment 0162).

**Parcel groups (W3-07B).** Lock order is group -> order -> reservation -> balances (the same direction as `begin_parcel_group_shipment`). If the order's group membership changed between the unlocked
read and the order lock, the cancel answers `503 retry_later` before any write (the group lock is never taken after the order lock); the retry succeeds. Cancelling a member of an OPEN group removes it
from `parcel_group_orders` and bumps the group `version`; if one order is left, the group is `DISSOLVED` (the survivor row is deleted so it may ship alone or regroup). The response carries
`parcel_group:{id,state,version}` (or null). A SHIPPED group cannot hold an unshipped order, so `already_shipped` is the only other outcome.

## 4. Idempotency, concurrency, authority

Each command stores `ops.command_results` under its own operation (`returns.rma.register|receive|inspect|close|cancel`, `fulfillment.merchant_cancel`); same key + same canonical request =
the stored body, same key + other request = `409 conflict`. The RMA / order row lock is taken first and the replay is read after it, so concurrent identical requests serialize.
Authority: `identity.resolve_access` before any lock and again after the writes (session, principal, authorization revision); permissions as above. All definers are SECURITY DEFINER,
`search_path=pg_catalog`, owner `commerce_checkout_writer`, EXECUTE `commerce_runtime` only (no worker authority); Go never SELECTs `returns.*`.

## 5. Stock (I03)

Shipping never lowers `on_hand` or `allocated` in this system (a shipped order's units stay allocated). Therefore a sellable returned unit is **released from `allocated`**
(`DEALLOCATE`, `delta_allocated < 0`, `on_hand` unchanged), exactly like the pay-at-pickup restock (§16.8); crediting `on_hand` as well would count the unit twice. Rows:

| Operation | Kind | When | command_key | reason |
|---|---|---|---|---|
| `returns.rma.restock` | DEALLOCATE | close, per line with `qty_restock > 0` | the RMA id | `rma_restock` |
| `checkout.merchant_cancel` | DEALLOCATE | cancel of a paid card order, per line | the order id | `merchant_cancel` |
| `checkout.merchant_cancel` | RELEASE | cancel of an unpaid hold, per line | the order id | `merchant_cancel` |

Scrap writes nothing. A refund writes nothing (RD6). **One invariant for every DEALLOCATE writer** (RMA restock, cancel release, bank-transfer refund restock, pay-at-pickup/COD cancel and restock): the guard refuses any
DEALLOCATE larger than what the ledger still holds allocated for that order line (sum ALLOCATE + DEALLOCATE of that checkout/warehouse/sku), so a second release of the same units cannot create phantom stock even when other orders
hold the SKU. **A shipment void is refused `409 has_returns` while a live (non-cancelled) RMA exists** (`record_manual_shipment` patched in 0155): a return presumes the parcel left, a void says it never did; without it an order could be
re-shipped, cancelled or refunded-with-restock after its units were given back. Provenance guard `inventory.guard_returns_ledger` (BEFORE INSERT, the only way a row with either operation name is accepted):
restock requires a `CLOSED` RMA of that order whose line says exactly the quantity, closed by the writing principal, the order `CONFIRMED` with a `COMMITTED` reservation, and never more than the ledger
still holds allocated for that line; cancel requires the order already `CANCELLED/CANCELLED` and the reservation `RELEASED`, the exact line quantity, for a DEALLOCATE a CAPTURED fact whose amount is covered by
succeeded + in-flight refunds, for a RELEASE no payment attempt. No double restock: the RMA state machine, the row lock plus `inventory.lock_balance`, the ledger unique key `(operation, command_key, warehouse, sku, kind)`
and the guard each refuse it independently. The unique index `ledger_pay_at_pickup_release_once` (one DEALLOCATE per order line) now excludes `returns.rma.restock` (several partial returns of one line).
Operator rule (pre-existing ceiling): `allocated` of shipped orders only shrinks through restocks, and a merchant who adjusts `on_hand` DOWN for goods that already shipped breaks `available = on_hand - reserved - allocated - unavailable`; do not.

## 6. HTTP (merchant routes, `/v1/admin/stores/{store_id}`)

| Route | Permission | Success |
|---|---|---|
| `POST /orders/{order_id}/cancel` `{expected_state, reason}` | `fulfillment:write` | 200 `{order_id, commercial_state:"CANCELLED", released_lines, parcel_group, reason}` |
| `POST /orders/{order_id}/returns` `{reason, lines:[{sku_id, warehouse_id?, quantity}]}` | `fulfillment:write` | 201 RMA |
| `GET /orders/{order_id}/returns` | `orders:read` | 200 `{items:[RMA]}` |
| `GET /returns[?state=]` | `orders:read` | 200 `{items:[RMA]}` |
| `POST /returns/{rma_id}/receive` `{expected_version, lines:[{sku_id, warehouse_id?, qty_received}]}` | `fulfillment:write` | 200 RMA |
| `POST /returns/{rma_id}/inspect` `{expected_version, lines:[{sku_id, warehouse_id?, qty_restock, qty_scrap}]}` | `fulfillment:write` + `inventory:write` | 200 RMA |
| `POST /returns/{rma_id}/close` `{expected_version, refund_id?}` | `fulfillment:write` + `inventory:write` | 200 RMA + `restocked_units` |
| `POST /returns/{rma_id}/cancel` `{expected_version}` | `fulfillment:write` | 200 RMA |
| `GET /orders/cancel-refund-gaps` | `orders:read` | 200 `{items:[{order_id, captured_minor, refunded_minor, gap_minor, cancelled_at, reason}]}` |

Every POST needs exactly one canonical `Idempotency-Key`, no query, strict JSON (unknown or duplicate keys 400). RMA: `{id, order_id, state, version, reason, refund_id, created_at, updated_at, lines:[{warehouse_id, sku_id, qty_registered, qty_received, qty_restock, qty_scrap}]}`.

## 7. Codes

409: `not_shipped`, `not_returnable`, `version_changed`, `invalid_state`, `not_restockable`, `has_returns`, `already_cancelled`, `state_changed`, `payment_in_flight`, `refund_first`, `already_shipped`,
`cvs_attempt_in_flight`, `payment_review_open`, `conflict` (idempotency); `has_returns` also answers a shipment VOID (PUT shipment). 503: `retry_later` (cancel raced a parcel-group change; also deadlocks). 422: `unknown_line`, `ambiguous_line`, `exceeds_shipped`, `exceeds_registered`, `invalid_quantities`, `quantities_mismatch`,
`lines_incomplete`, `nothing_received`, `refund_mismatch`, `not_cancellable`, `invalid_request`. 403 `forbidden`, 404 `not_found` (also another store's id), 401 `unauthorized`, 503 `retry_later` (deadlock) / `unavailable`.

## 7a. Known limits

* A card order shipped to a convenience store and returned to sender unclaimed has no stock path: it is not returnable (an RMA needs the parcel `PICKED_UP`) and not cancellable (a live parcel is `already_shipped`).
  The merchant uses an inventory adjustment; a restock path for returned CVS parcels is a follow-up unit.
* The inspect step records the decision and close writes stock (two calls); the order list has no "has returns" filter (`GET /returns` is the list).

## 8. Gates (`tests/foundation/returns_test.go`, `internal/httpapi/returns_test.go`)

`^TestReturns$` (RT01 full lifecycle + one ledger row, RT02 refusals + cross-store, RT03 permissions, RT07 refund decoupling and `refund_id` link, RT09 replay/drift, concurrent close and partial RMAs, forged ledger rows),
`^TestMerchantCancel$` (RT04 hold, RT05 payment in flight, RT06 refund_first then cancel without auto-refund, shipped, concurrent cancels, RT08 parcel group shrink/dissolve, refused cancel keeps the group, RT10 cancel vs payment start,
COD delegation), `TestReturnRoutesTransportRules`/`TestReturnCodesReachJSONBody` (DB-free router). Pins updated: R2 migration count 77, ACL pins of the eight definers, WAS02.

## Amendment 0162 (unit cancel-closes-work-item): the cancel closes the payment work item

`fulfillment.merchant_cancel_order` (W3-08B) left the captured order's `fulfillment.payment_work_items` row `READY` forever, so a merchant-cancelled paid order projected `work_state=READY`
(`identity.read_merchant_orders` / `_v2` map a missing row to `NONE`; nothing else ever closed it) and the whole store list 503'd until the Go/TS validators were relaxed to accept the combination
(symptom fix, kept as legacy protection). 0162 makes the data right:

* The card-cancel branch DELETEs the order's `READY` work item row in the same transaction (I04), after the refund-coverage gate (`409 refund_first`) has passed. `commerce_checkout_writer` gains
  `DELETE` on `fulfillment.payment_work_items` for this definer only; RLS (`private_writer`, tenant/store GUCs set by `returns.authorize`) still scopes the row.
* At cancel time the money is by definition covered (held >= captured), so no human work remains on the order itself; if a counted in-flight refund later FAILS, `GET /orders/cancel-refund-gaps`
  is the single surface that keeps the money visible as needing work (it is fact-based, §3 above). Rejected alternative: keeping the work item `READY` while a refund is in flight — the common
  flow (refund, then cancel immediately) would then linger `READY` after the refund SUCCEEDS, i.e. the original bug, unless the frozen stripe-refund observation applier were also hooked.
* Forward-only backfill: EVERY `READY` work item of a `CANCELLED` order is closed (integrator ruling, no open-gap exception: an outstanding cancel-refund gap is carried by
  `GET /orders/cancel-refund-gaps`, which never reads the work item, and the new definer already gives that business state `NONE`). `REVIEW_REQUIRED` work items and every row of a
  non-`CANCELLED` order are never touched. The migration block counts the rows before deleting and fails unless the delete closed exactly that many, and it refuses to run under a role
  without `SUPERUSER`/`BYPASSRLS` (the table is FORCE RLS: such a role would see zero rows and the backfill would silently do nothing).
* Projection, work-state vocabulary, ship eligibility (`manual-fulfilment-v1` MD6 requires a `READY` work item of a `CONFIRMED` order — a cancelled order can never ship) and the refund engine
  (`stripe-refund-v1` RF07 replay) are unchanged. Gates: `^TestMerchantCancelClosesWorkItem$` (cancel closes, never-paid NONE, in-flight-then-failed gap stays listed, 0162 backfill closes
  every legacy cancelled `READY` row including open-gap ones and spares a `REVIEW_REQUIRED` row and a `CONFIRMED` order's `READY` row, the backfill block refuses a non-bypass role and fails on a
  suppressed delete), `TestBuyerPaymentCaptureACLAndObservationBinding` (only `commerce_checkout_writer` holds `DELETE` on the work-item table, nobody a wider table privilege), R2 migration count
  pin +1 (87 -> 88; PAY-RM1's 0161 is 87).
