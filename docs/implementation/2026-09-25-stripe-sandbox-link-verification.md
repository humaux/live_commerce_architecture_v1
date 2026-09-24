# Stripe sandbox link — scoped verification

Date: 2026-09-25 Asia/Shanghai. Status: CREATED_API_READBACK_AND_CHECKOUT_DISPLAY_PASS.
No payment has been submitted by the agents. This is an operator-plugin check,
not a deployed Stripe adapter, webhook or SaaS order-payment acceptance.

## Authorized target

- Account: 香港大碗貿易有限公司 沙盒, `acct_1UJDb0RusP6Wwj7e`.
- API environment: `livemode=false`, freshly verified from the connected-account
  list after the user selected sandbox instead of the initially connected live account.
- User explicitly approved one HKD10 test Payment Link. No live account data,
  charges, refunds or configuration changes are in scope.
- Link: [open sandbox checkout](https://buy.stripe.com/test_fZubJ31O59xa3tFerdbsc00).
- Link ID: `plink_1UJGJARusP6Wwj7e0USrGyRm`.
- Generated price: `price_1UJGJARusP6Wwj7eka5FWyx6`.
- Generated test product: `prod_VJu75vyVMjAEeY`.
- Trace metadata: `project=live-commerce`, `test_case=LC-STRIPE-20260925-001`,
  `purpose=sandbox_link_verification`.

## API evidence

1. Initial `GET /v1/payment_links?limit=5`: `data=[]`, `has_more=false`.
2. First create returned `INVALID_ARGUMENT`: account-default Managed Payments
   conflicts with `custom_text`. An immediate list read still returned no links.
3. The corrected request set `managed_payments.enabled=false` on this link only.
   Global account settings were not changed. One create succeeded.
4. Independent list and link-line-items reads confirmed the exact persisted ID,
   `active=true`, `livemode=false`, `currency=hkd`, one-time price1000 minor units,
   quantity1, subtotal/total1000, tax0, discount0, no adjustable quantity.
5. The link has a one-completed-session limit; readback count was0. Promotion
   codes and automatic tax are disabled. Test-only messaging and hosted
   confirmation are configured; no production redirect or fulfillment is wired.

This MCP execution surface does not expose a custom idempotency header. We did
not claim idempotency-key protection or blindly retry an uncertain create. The
first request explicitly failed validation and a list read preceded correction.
The link/product/price remain available for the authorized test; none was deleted.

## Independent browser evidence

A fresh isolated Playwright CLI session opened the hosted checkout and selected
HKD for display. It showed the merchant's 沙盒 label, the test product and
HK$10.00. No contact/card fields were filled; no payment or wallet action was
taken. The isolated browser was closed without affecting the user's browser.

The page also offered a USD converted amount. Therefore the proven base price
and selected display are HKD10, not a guarantee that every visitor is forced to
pay in HKD. Future application reconciliation must explicitly handle or constrain
presentment currency; the merchant's adaptive-pricing defaults were not changed.

Screenshot (inspected by both browser agent and root):
`output/playwright/stripe-sandbox-20260925/.playwright-cli/page-2026-09-24T17-22-59-983Z.png`.
SHA256 `6e1964bf05534922d454aafc554f3a464f0a28a5980f03d4814a20fc8a392d40`.
Humaux browser record: `50d82ff3-55fa-4c2f-bd4c-770d1b2b1ae5`.

After the browser check, a fresh API list read confirmed the same link still
`active=true`, `livemode=false`, with completed-session count0 / limit1. This
does not claim that opening the page created no Checkout Session.

## Remaining acceptance

Browser display verification is separate from API readback. Test-card payment,
signed webhook receipt/replay, delayed-payment handling, SaaS order
mapping, amount/currency/account checks, reconciliation and live qualification
remain NOT_RUN. Opening the hosted page can create a Checkout Session; this is
not evidence of a payment or a completed session.

For the actual application, a successful landing page must not be the fulfillment
authority. Stripe's [fulfillment guide](https://docs.stripe.com/checkout/fulfillment)
requires payment-state checks and webhook handling. The Codex Stripe plugin's
authorization is not a server-side application credential or tenant onboarding.

Humaux creation/root-cause record: `48fb1b93-e27d-4bfc-b583-650df443ca31`.
