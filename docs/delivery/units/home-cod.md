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
