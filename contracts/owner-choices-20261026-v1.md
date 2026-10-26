# Owner choices 2026-10-26 v1 — binding rulings for the code-alignment backlog

Status: FROZEN 2026-10-26 (DESIGN; every gate NOT_RUN). Rationale and sources: `docs/adr/ADR-20261026-owner-choices.md`.
Authority: owner words 「ad-01、Stripe不是支持信用卡、储蓄卡、link收款嘛？ad-03、让商家能够简单配置，快速开店，其他的按照你的结论来」. "结论" = the recommended option of each question in `output/figma-ui-completion-20261025/ADR-CHOICES.md`.
Scope: where a cited contract is silent or says otherwise, this file governs the named point only. Anything kept "read-only" means: history is retained, still readable and exportable, never deleted, no new writes.
No code, SQL or secrets here. Migrations are `02xx` placeholders (integrator assigns).

| Clause | Ruling (binding) | Owning contract touched |
|---|---|---|
| OC-Q1 (AD-02, FIN-01, FIN-02) | Option B. Merchant "own bank account" settings entry is closed (no new bank details can be saved); manual order creation no longer offers `bank_transfer`; storefront `bank_transfer` stays unavailable. Existing bank-transfer orders, proofs, `AWAITING_TRANSFER` states and the `bank_transfer_confirmed_*` report fields/CSV columns are **kept read-only** (still shown, exported, API/CSV shape unchanged). `decide_bank_transfer` stays for orders already awaiting transfer (it needs recent re-auth, merchant-password-auth §16). | merchant-orders-v2, reporting-v2, buyer-checkout-options-v1 |
| OC-Q2 (FIN-03) | Option A: no new merchant-visible 退回/帶回 labels; the settlement page keeps 已撥付 / 待撥付 / 無須撥款. **Conflict flagged:** SA-0 (frozen after the question was written) already specifies `returned_in_minor`/`payout_returned` (stripe-platform-account §6.7–§6.9). Ruling: SA-0/SA-1 backend ledger work is unaffected; merchant UI task SA-2 is held until the owner confirms Q2 in one line. | stripe-platform-account |
| OC-Q3 (FIN-04) | Option A: keep 已請款 / 上期結轉（扣除）/ 應撥淨額 / 已撥付 on merchant settlement pages; Figma returns to current wording. | stripe-platform-account |
| OC-Q4 (LOG-05) | Option A: shipping stays `country_flat` fixed fee + free-shipping threshold. No per-method fee or lead time. | delivery-service-v1 |
| OC-Q5 (LOG-06, CP-03) | Option A: when the buyer picks cash on delivery the checkout footer shows 「到貨需付」 = the already-quoted COD due (including the COD fee) taken from the server quote; the front end never adds money. Order-page 「代收金額」 is replaced by plain wording (exact text needs one owner review; until then current text stays). Submit idempotency unchanged. | buyer-checkout-options-v1, storefront-v2 |
| OC-Q6 (AD-12, CP-05) | Option A: no in-UI Meta/IG disconnect and no 「管理連結」. The OAuth page keeps the note that not ticking IG does not remove an earlier grant. | meta-ads-v1 |
| OC-Q7 (AD-10) | Option A: all "forbidden" states show one sentence 「你目前的角色無法使用此功能，請聯絡店鋪擁有者」 with no role named. Server ACL unchanged. | admin-transport-v1 |
| OC-Q8 (LOG-08) | Option A: no home-delivery address label. Figma marks it 「未納入產品」. No waybill purchase. | taiwan-cvs-logistics-v1 |
| OC-Q9 (SF-01) | Option A: no live-replay entry on the storefront; stays planned. | storefront-v2 |
| OC-Q10 (SF-04) | Option A: a publish whose outcome is unknown shows 「發布結果待確認」 and checks the **original request** (same request identity and version); it never sends a new publish. | storefront-v2 |
| OC-Q11 (SF-07) | Option A: one "store not open yet" message; no 暫停營業 state. | storefront-v2 |
| OC-Q12 (AD-04) | Option B (recommended): add an inbox `unread` filter. Needs a new query (definer, store-scoped, RLS) and a written definition of "unread" before build; until then the filter is not shown. | meta-inbox-v1 |
| OC-Q13 (AD-05) | Option A: the inbox shows no marketing-consent state; consent is never inferred from the reply window. | meta-inbox-v1 |
| OC-Q14 (AD-08) | Option A: the Meta connection-health mail has no off switch. | meta-connection-health-v1 |
| OC-Q15 (CAT-02) | Option A: no per-SKU delivery eligibility; weight/size only, delivery per store settings. | catalog-inventory-v1 |
| OC-Q16 (LOG-04) | Option A: no separate 自配送 method; Figma keeps it 「未開放」. | delivery-service-v1 |
| OC-Q17 (CP-07) | Option A: the `shipped` email stays (only to buyers with an email); manual-fulfilment MD10 now says "no Messenger/IG message". | manual-fulfilment-v1 (edited in place) |
| OC-Q18 (SF-06) | Option A: error page offers only 「重新讀取這個頁面」 (read-only reload) and an offline banner; it never replays a payment or order and promises no input persistence. | storefront-v2 |
| OC-Q19 (AD-11) | Option A: empty states carry explanation text only, no action buttons, one shared style. | docs/design/DESIGN-SPEC.md |
| OC-Q20 (LOG-07) | Option B (recommended): add a "review before recording shipment" step; the final confirm sends the existing command (version check and idempotency key unchanged; no new write permission). | manual-fulfilment-v1 |
| OC-Q21 (ORD-01) | Option A: merchant order labels use 已出貨 and 待付款 everywhere (state codes and filters unchanged); 待付款 never reads as payment confirmed. | merchant-orders-v2 |
| OC-Q22 (CP-06) | Option A: subscription page title stays 「帳單」. | customers-billing-v1 |
| OC-AD01 | Stripe card + Link, see stripe-psp-v1 D6 and stripe-platform-account AD-PF3. | stripe-psp-v1, stripe-refund-v1, stripe-platform-account |
| OC-AD03 | Quick start = 3 steps, new stores only; see merchant-settings-wizard-v1 amendment. | merchant-settings-wizard-v1 |
| OC-T21R01 | Recent re-auth; see merchant-password-auth-v1 §16. | merchant-password-auth-v1 |

## Already decided elsewhere (no change here)
LOG-01, LOG-02, LOG-03, CAT-01, CAT-03, CP-02, AD-07, AD-13, CAT-04: owner words O1–O9 (`output/integrator/tools/logs/owner-decisions-20261026.md`) or current contracts govern. AD-07 and AD-13 stay「未納入產品」and are **kept read-only** (not deleted).

## Gates (all NOT_RUN)
OC01 `python3 scripts/check_packet.py` passes with the new tasks. OC02–OC0x per implementation card (`contracts/tasks.json` OC-* entries): each card states its red-first test; no card may delete history named read-only above.
