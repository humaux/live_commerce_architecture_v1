# Unit home-cod — cash on delivery for home delivery (黑猫 / 新竹货到付款) (R5)

Owner decision 2026-10-02: upgrade the pilot first, then close this gap.

Evidence (docs/discovery/2026-10-02-shopline-admin-study.md §5):
- The pilot merchant takes **no online card payments**. Its methods are cash on delivery, 7-11 and FamilyMart pick-up-and-pay, 黑猫宅配货到付款, and 新竹物流货到付款 (常溫).
- Ours supports card, CVS pay_at_pickup and bank_transfer. contracts/payment-methods-v1.md:34 explicitly infers no COD, and r2-design-rulings superseded "no COD in v1" only for CVS.

Read: contracts/taiwan-cvs-logistics-v1.md §16 (the pay_at_pickup collection state machine, which is the model to reuse), contracts/storefront-v2.md §C (bank transfer, free shipping), contracts/manual-fulfilment-v1.md, migrations 0072/0073/0083/0085/0088/0102, internal/checkout (payment modes, cardRefusal pattern), internal/fulfillment (record_collection), the reporting finance CSV, and the admin order and Settings > Delivery pages.

## Decisions
1. **New payment mode `cash_on_delivery`,** valid only on home-delivery services whose merchant setting enables COD.
   - Settings: an enable switch, a per-order amount cap, an optional surcharge in TWD whole dollars, and the carrier label (黑猫 or 新竹; manual fulfilment, no carrier API).
   - Begin creates the order AWAITING_COLLECTION with no online charge. It reuses the pay_at_pickup money rules: a whole-TWD total, stock held as for pay_at_pickup, and the open-orders limit.
2. **Collection state machine shared with pay_at_pickup** (one shared function, not a copy):
   - PENDING → COLLECTED / RETURNED / REFUNDED_OFFLINE / CANCELLED, recorded by the merchant.
   - The 0102-style guard: `collected` only after the order is marked shipped.
   - `returned` restocks exactly once (the 0083 pattern).
3. **Finance:** COD collected count and amount are their own columns (CSV header grows by two). The dashboard and finance pages show them.
4. **Storefront:** checkout offers 「貨到付款」 for eligible home options with the amount and surcharge. Card or transfer stays available where configured. It reads in three locales.

## Gate
- **PG:** mode eligibility (home only, setting on, cap), the whole-TWD rule, the state machine and guards (collected before shipped is refused), restock-once on returned, finance totals, ACL.
- **Browser:** new mode `--browser-home-cod` covering the merchant enabling COD, a buyer ordering with COD, the merchant shipping, recording collected, and checking the finance row. zh-TW and en, desktop and 390px.
- **Regression:** `--browser-checkout-offline`, `--browser-cvs` and ops-polish finance stay green.

## Review rulings (integrator, 2026-10-02) — independent review `REVIEW-home-cod.md` at f4edfc7 (0 P0, 4 P1, 11 P2)
The reviewer's red tests (`TestHomeCodDefect*`) and hardening tests stay as written; the fix makes them green. Split per the routing rule: the backend (Go/SQL) goes to DeepSeek, then everything under `apps/` goes to Codex.

**Backend (DeepSeek), forward migration 0108_home_cod_fixes.sql.** 0107 is unreleased, but a new file keeps the reviewer's evidence reproducible. Renumber at merge if meta-multi-page lands first.
- **P1-1 / P2-7:** the buyer order DTO (order page, order link, guest lookup) exposes `cod_collect_minor` = total + surcharge for COD orders through a buyer-readable projection. Placed and shipped mails print it.
- **P1-2:** the unshipped export appends two columns at the end, `payment_mode` and `collect_minor`. This is a contract amendment: appending is backward compatible, and the B19 precedent applies. The merchant order DTO carries `cod_collect_minor`.
- **P1-3a / P2-9:** `record_manual_shipment` refuses VOID once `collection_state` ≠ PENDING (COD and pay_at_pickup). A tracking-number correction stays allowed but must not change any state.
- **P1-3b:** `collected_at` is set once by `record_collection`, and finance groups COD and pickup cash on it.
  - Backfill existing collected rows from `updated_at`. This is a best-effort estimate; label it in the migration comment.
- **P2-1:** Begin accepts an optional `expected_cod_surcharge_minor`; a mismatch returns 409 `cod_surcharge_changed` so the buyer re-quotes.
- **P2-3:** options omit COD on a row whose total + surcharge exceeds the cap.
- **P2-4:** the carrier label is snapshotted on the order and returned in the options DTO. The manual-shipment carrier enum gains `black_cat` and `hsinchu`.
- **P2-5 (partial):**
  - the merchant `AWAITING_COLLECTION` filter lists only `collection_state = PENDING`;
  - the commercial-state model, shared with pay_at_pickup, is unchanged and recorded as a known limitation: promo and live-price uses are not released when a parcel is returned.
- **P2-6:** COD requires the same recipient-reachability check as pickup and a TW destination.
- **P2-10:** `read_cod_offer` becomes VOLATILE.
- **P2-11:** erasure waits while a shipped-but-uncollected COD or pickup order is in flight.
- **P2-8 (separate COD open cap): deferred.** The pickup knob still bounds COD; logged.

**UI (Codex), after the backend is merged into the unit branch.** Codex owns every home-cod change under `apps/` and reviews the first UI line by line:
- **P1-1:** CodOrderStatus and the order page state 「到貨需付 NT$X（含貨到付款手續費 NT$Y）」.
- **P1-2:** the merchant list, the order detail and the collect dialog show the collect amount.
- **P2-1:** the storefront sends the expected surcharge and shows the re-quote message.
- **P2-2:** the BFF passes through `cash_on_delivery_*` codes, and the limit is not retryable.
- **P2-4:** the carrier label is shown.
- **P2-5:** the buyer headline follows the collection state.
- **Comps:** 04 and 09.
