# Automatic product payment entry v1

Status: REQUIREMENT_AND_DESIGN, 2026-09-25. End-to-end automation is NOT_RUN.
This supplements R02/R03/R07 and T04/T05/T11; it does not replace their gates.
Owner request: creating a product link and entering its price must automatically
produce the corresponding payment entry, without creating a PSP link by hand.

## User-visible contract

1. A merchant enters product/SKU, price and currency in the existing catalog.
   Saving an eligible active SKU automatically makes its shareable purchase
   entry available in the product result/list: copy link, open, or share it.
   There is no separate "create Stripe product/payment link" operation for the
   merchant. An unpublished shop or missing price/configuration reports a
   specific setup blocker rather than a working-payment claim.
2. A product entry belongs to the shop's verified published origin. It remains
   stable across price changes and opens the current product/SKU selection.
   It contains neither a buyer capability nor a payable amount. Opening it,
   including a social crawler's GET, never creates an order, reserves stock,
   starts a payment, or sends a message. A share source is not buyer identity.
3. After the buyer confirms SKU, quantity and destination, existing cart,
   Quote and checkout authority determines amount/currency, verifies delivery
   and reserves inventory. The chosen enabled payment method then creates the
   order-specific provider checkout automatically; the buyer does not copy
   account IDs or ask the merchant to generate another link.
4. The merchant's own qualified provider account receives the payment. Never
   use a platform-wide test link or another merchant's account as a fallback.
   A connected operator plugin is not a credential provisioned to this SaaS.
5. Buyer language remains freely selectable (zh-CN, zh-TW, en). The public
   entry and checkout UI require the already-requested visual review before
   implementation and real desktop/mobile acceptance afterwards.

The shareable shop entry and the temporary provider payment URL are different
objects. This design is not a permanent `buy.stripe.com` link for each SKU and
must not be described as one. The customer-facing result is one automated
purchase flow, not an extra task for the merchant.

## Single-source design and lifecycle

- Reuse catalog product/SKU IDs and price versions; derive the public path from
  the published shop route plus product ID (optionally a validated SKU selector).
  Do not create a duplicate SKU/price database merely to store a derivable URL.
  Public browse and buyer/private checkout are separate authorization surfaces.
- Resolve domain ownership/publication for each entry request. Recheck product
  and SKU status; unknown/archived products cannot start a new purchase. Saving
  a catalog item must not itself make an unpublished shop public.
- Share URLs show current prices. Quotes must be revalidated before order
  creation; stale price/configuration requires a new quote and buyer consent.
  A valid already-created order/payment retains its immutable agreed total;
  later catalog edits never silently reprice or refund it. Archiving blocks new
  purchases, not settlement of an existing authorized payment.
- Reuse the order, attempt, integration-operation ledger, River, account binding
  and credential-version authority. Provider I/O occurs outside DB locks. Store
  operation identity durably before dispatch and reconcile ambiguous outcomes.
  A retry uses the original account/environment and idempotency key; no fresh
  key or new attempt until the original outcome is resolved. Keys alone are not
  an indefinite exactly-once guarantee.
- Do not mark a URL ready before its provider response is validated and saved.
  Validate HTTPS/provider host, mode, account, order, amount, currency and expiry;
  never accept arbitrary browser return URLs. Hosted URLs are sensitive and
  excluded from logs, shared caches and the reusable public product entry.
- Provider-side optional quantity changes, shipping selection, discounts, tax
  or currency conversion must not silently alter the accepted order's amount
  or currency. Derive checkout parameters from the order snapshot and verify
  the resulting totals. Use per-session configuration; do not silently change
  a merchant's global PSP settings. Automatic provider capabilities do not
  override the existing merchant-of-record or single-currency order decision.
- A disabled/expired/missing provider qualification is a setup blocker, not a
  simulated payment. SANDBOX and LIVE remain separate configurations and gates.
  Provider session creation errors surface pending/failed/unknown honestly,
  preserving an existing product without displaying a fake payment URL.
- Signed callbacks and authoritative provider reads enter the existing payment
  fact/capture path. Success redirects and `session completed` alone do not prove
  paid funds for asynchronous methods. Duplicates/out-of-order events must not
  create a second order, stock commit, fulfillment job or notification.

## Verified implementation gap at 9acd869

`internal/checkout/payment.go:StartPayment` admits only `payuni_credit`, creates
a query-only pending attempt and does not call a provider. `PaymentResult` has
no hosted URL/form field, and the private buyer HTTP transport has no payment
route. The isolated Stripe HKD10 Payment Link proves operator sandbox access
only, not a server-side adapter, automatic product flow or merchant onboarding.

Existing stock holds last 15 minutes (`checkout.go` and migration 0013).
Stripe Checkout's configured `expires_at` is 30 minutes to 24 hours after
creation. Do NOT pass the 15-minute hold deadline as that parameter, silently
extend all inventory holds, or release stock while a provider can still accept
payment. The Stripe adapter needs an explicit pending-payment deadline,
provider expire/query reconciliation and failure/unknown-state policy before
it is enabled. Pending/unknown attempts retain stock under the existing rule;
they are not a normal expired HELD reservation.

No new Connect configuration, platform fee, payout arrangement, credential,
live payment or customer production setting is authorized by this document.
Preserve the existing merchant-owned-funds decision; qualify each real provider
and account separately when implementing its adapter.

## Delivery order and acceptance gates

| Gate | Evidence required (all currently NOT_RUN for this new whole flow) |
| --- | --- |
| PE01 Automatic entry | Save a priced SKU once; the eligible shop product result exposes a working share entry without manual PSP setup per product. Duplicate save returns the same identity. |
| PE02 Public entry | Known published origin only; unknown/unpublished/cross-shop, archived item and forged selector fail closed. Crawler GET has zero order/stock/provider effects. |
| PE03 Discovery | Buyer finds active products, current prices, markets and eligible delivery options without manually supplied internal IDs. No account/credential/warehouse fields leak. |
| PE04 Exact total | Variants, quantity, delivery and supported tax/discount inputs produce the stored order total; forged browser amounts and stale quotes cannot change it. |
| PE05 Scoped payment | Only the selected merchant account/environment/method can create the checkout; a second merchant and mixed sandbox/live inputs are denied. |
| PE06 Retry and crash | Repeated clicks, timeout after provider creation, process restart and uncertain response reconcile to one logical attempt, without blind fresh-key creation. |
| PE07 Lifecycle | New purchase uses new price; an existing order keeps its amount; archive/domain suspension blocks new purchase. Pending provider session expiration is reconciled before stock release. |
| PE08 Financial facts | Real sandbox paid, failed and pending outcomes plus duplicate/out-of-order signed callbacks; exactly one capture/stock commit and no success-page-only confirmation. |
| PE09 Browser | Approved three-language desktop/mobile product-to-provider-to-order flow, cookie/CSRF/origin/scoped-rate controls and safe lost-response handling. |
| PE10 Isolation/regression | Ordinary-role PG/RLS tests, race/vet, exact DTO assertions, no production changes and independent review. |

Implement in dependency order: buyer-safe discovery -> published public entry
and bootstrap/BFF -> automatic catalog share entry -> provider checkout adapter
and payment HTTP -> sandbox browser/payment/callback acceptance. Existing
transaction gates remain binding; passing discovery alone cannot pass PE01.

## Alternatives not selected

- One reusable fixed-price PSP link per SKU: does not by itself enforce our
  stock reservation, buyer ownership, current shipping/tax, or order truth.
- Call PSP on every catalog save: creates external side effects before buyer
  quantity/destination and final total exist, and makes catalog availability
  depend on a payment outage.
- A second payment engine or shared platform collection account: conflicts with
  existing transaction and merchant isolation decisions.

## Primary references (checked 2026-09-25)

- [Checkout Sessions](https://docs.stripe.com/payments/checkout-sessions)
- [Session creation and expires_at](https://docs.stripe.com/api/checkout/sessions/create)
- [Explicit session expiration](https://docs.stripe.com/api/checkout/sessions/expire)
- [Idempotent requests](https://docs.stripe.com/api/idempotent_requests)
- [Verified fulfillment and asynchronous outcomes](https://docs.stripe.com/checkout/fulfillment)
