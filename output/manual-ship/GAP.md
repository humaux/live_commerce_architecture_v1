# MANUAL-SHIP — STEP 0 gap map (unit/manual-ship)

Base: `unit/manual-ship` == `origin/r3/integration` @ `40da2b9c5` (0 ahead / 0 behind at start).
Scope of this file: what already exists on base for the card's loop, what is missing, and why the
only code change proposed is a browser acceptance test (no contract change, no migration).

## Verdict

The manual-shipping closed loop **already exists end to end on base**, including CVS pickup orders:

| Card requirement | Existing evidence on base |
| --- | --- |
| Merchant moves 待出貨 → 已出貨 by hand | Frozen contract `contracts/manual-fulfilment-v1.md:209-226` (PUT + history + export, one command route); HTTP adapter `internal/httpapi/shipments.go:40-109`; command `internal/merchantorders/shipments.go:187` (`RecordShipment`) and `:223` (`ShipmentHistory`). |
| Carrier + tracking entered manually, no label/carrier API | `internal/merchantorders/shipments.go:187` validates only merchant-supplied fields; SQL command `migrations/0063_manual_fulfilment.sql:284` writes no `integration.operations`/job (contract MD2, `contracts/manual-fulfilment-v1.md:29`). |
| CVS pickup store comes from the order's frozen snapshot | Merchant detail projects the frozen `destination.pickup` and `pickup_source`: `internal/merchantorders/orders.go:109-124`, `:560-565`; admin renders name/code/source at `apps/admin/components/OrderDetailPanel.tsx:286-301`. Buyer page renders the same frozen pickup at `apps/storefront/components/OrderFlow.tsx:772-780`. |
| Order and buyer page show shipped + tracking | Merchant head projection `internal/merchantorders/orders.go:134-135` and admin `OrderShipment` (`apps/admin/components/OrderShipment.tsx:1-20`, form `ShipmentFields` `:80-125`); buyer `shipment` object `contracts/manual-fulfilment-v1.md:228-235` and `apps/storefront/components/ShipmentBlock.tsx:34-60`. |
| Audit row | `migrations/0063_manual_fulfilment.sql:388` inserts `ops.audit_events` action `fulfillment.shipment_recorded`/`_corrected`/`_voided`. |
| Idempotent same-key replay | `migrations/0063_manual_fulfilment.sql:331-335` replays `ops.command_results` (`fulfillment.manual_shipment.record`), same key + different hash → `PT409`; `internal/merchantorders/shipments.go:187` is the only writer. |
| Store-scoped auth | `internal/httpapi/shipments.go:53` (`fulfillment:write`) and `:70-90` (`orders:read` / `orders:export`); `internal/merchantorders/shipments.go:212` re-checks the permission after locks; other-store orders are 404 by the frozen auth pattern (`contracts/manual-fulfilment-v1.md:211`). |
| CVS orders are eligible for manual shipping (no live ECPay attempt) | `migrations/0073_taiwan_cvs_functions.sql:936-947` replaces `manual_shipment_eligible`; it refuses only a non-FAILED/ABANDONED `cvs_shipments` row, so a `buyer_entered` pickup with no ECPay attempt is shippable. |

Therefore: **no contract text change, no migration, no `apps/` change is warranted.**

## What is actually missing: browser acceptance for the CVS-snapshot case

The existing MF07 browser gate ships a **home-delivery** order: `tests/foundation/browser_refund_fulfilment_test.go:283-323`
(`ship := e.payInStore(...)` at `:289`; `payInStore` is `tests/foundation/manual_fulfilment_env_test.go:239-243`).
The CVS browser gate creates a `buyer_entered` order but only checks the merchant's "buyer-entered" label;
it never marks it shipped: `tests/admin/taiwan-cvs.spec.ts:247-250` (`enteredOrder`, label only).
The HTTP CVS fixture is used only for export eligibility, not for a shipment:
`tests/foundation/manual_fulfilment_http_test.go:539-540` (`mfxCVSOrder` for `e4`/`e5`), while the shipped
fixtures `h1`/`h2`/`h3` are home orders (`:221-223`).

So the card's headline case — *a merchant manually ships a CVS order and the buyer sees the shipment* —
has no browser acceptance anywhere on base. That is the single missing item.

## Proposed change (tests only unless CI proves otherwise)

1. `tests/foundation/manual_fulfilment_env_test.go`: a `buyer_entered` CVS order factory that records the
   store through `fulfillment.record_buyer_cvs_store` (the #262/#298 snapshot directory), sets the
   destination from that pickup, and pays through the real capture path (card).
2. `tests/admin/manual-fulfilment.spec.ts`: real clicks on that order — assert the frozen pickup
   name/code and `buyer_entered` source, record carrier + leading-zero tracking, assert
   `MERCHANT_SHIPPED`, and assert persistence after reload.
3. `tests/storefront/shipment-buyer.mjs`: optional pickup-name assertion so the buyer page run for the
   same order proves tracking **and** the snapshot pickup store (3 locales × desktop/mobile).
4. `tests/foundation/browser_refund_fulfilment_test.go`: wire the fixture + env into the existing MF07
   browser test (no new suite, no apps/ edit).

If CI shows this is green, the loop already worked and the test is the only deliverable (card STOP rule).
If it is red, the failing assertion identifies the missing implementation; at most 2 targeted fixes.

## Not changed

- `contracts/` (frozen; the card forbids silent text changes).
- `migrations/` (existing columns/head/audit already suffice; no 0210).
- `apps/` (owner rule: DeepSeek does no UI/visual work).
- Any provider, label purchase or carrier call (contract MD2; status stays `MOCK`).
