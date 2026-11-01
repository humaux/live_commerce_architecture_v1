# PARITY-BUYER-AFTER FIX2 r3

Run 41251402317 on b2a2350e (origin/r3/integration already merged, "Already up to date").

| Failing job / test | Class | What changed |
|---|---|---|
| gate (--browser-buyer-after-parity) job 124686170665: `payment hand-off page · home` and `· cvs` (missing 前往付款 heading/text/line/control after clicking 前往安全付款頁) | b (fixture): the test's mock BFF `payment/prepare` returned 200 without advancing the payment view, so `stripeHandoff` (apps/storefront/lib/order-payment.ts:359-372) re-read NOT_STARTED/NONE, judged it not `continuable`, closed the tab and returned `aborted`, and `onSubmitted` (OrderPayment.tsx:382) never fired. Product code is correct; the mock now mirrors the server. | tests/storefront/buyer-after-parity.mjs (mock `prepare` route): sets PENDING/READY, cancel_requested=false, AWAITING_PAYMENT like the real prepare. Assertions unchanged. |
| gate (--browser-webkit) job 124686170506: step `payment` = `TestBrowserStripeCheckout/SU07` (tests/storefront/stripe-browser.mjs newOrder:197 `checkout-empty` not visible after clicking `continue-shopping` on a PENDING order; no BFF call follows the click) | UNPROVEN (c?) | Not changed. This PR does not touch the checkout / OrderPayment non-`renderFrame` path or stripe-browser.mjs; base Gates run 41238442379 has no webkit job, so base red/green cannot be shown from logs. Needs one CI run on this head: if it reproduces, evidence is in artifact gate-browser-webkit (playwright/run.DiSgQFQX/webkit/5f6a4fe43273-payment.jsonl). |
| receipts / required | follow-on | fail only because of the two jobs above. |

STATUS: DONE
