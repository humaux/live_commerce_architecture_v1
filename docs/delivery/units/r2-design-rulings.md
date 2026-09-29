# R2 design integrator rulings (2026-09-29) — binding for the amendment round of
# merchant-password-auth-v1, taiwan-cvs-logistics-v1, meta-ads-v1, stripe-live-enable-v1,
# customers-billing-v1

Owner decisions already given (override any draft text):
- O-A Merchant login = email + password + emailed code; OIDC optional (supersedes merchant-identity-v1's
  "no password store" line for password principals).
- O-B Outbound email = generic SMTP adapter (Go stdlib net/smtp, implicit TLS 465, no new dependency);
  first configuration = QQ mailbox SMTP (smtp.qq.com:465) with an SMTP authorization code from env/secret
  file; Tencent Exmail (smtp.exmail.qq.com:465, sender @xgdwm.com + SPF/DKIM/DMARC on Cloudflare) is the
  recommended upgrade. Resend is NOT the default. Idempotency: SMTP has no idempotency key — a send is
  attempted once after commit; on an uncertain result the user presses resend (new code). Rate limits
  must fit a personal-mailbox daily cap (treat as UNKNOWN until measured; global cap configurable,
  default 200/day, fail closed with a clear 503 when exhausted).
- O-C Meta app = 「大梦」 app 4291253377792879, to be attached to 香港大碗貿易有限公司's Business
  Portfolio (not Daerdo's 1299693313226979). Business verification and App Review are owner steps.
- O-D Production is operated by Claude on the owner's server; the owner supplies secrets as files;
  secrets never go through chat.

Defaults the integrator accepts now (owner may revise before go-live; list them in the contract's
"Integrator rulings"):
- Auth: Q2 min password 12; Q3 HIBP k-anonymity check on, fail-open; Q4 code on every login, no
  remember-browser; Q5 reset signs in + revokes other sessions; Q6 admin host DNS-only (no CF proxy)
  until a proxy review; Q7 open sign-up (store creation still behind COMMERCE_ONBOARDING_ENABLED);
  Q8 180 d audit / 2 d throttle; Q9 6-digit code. R-1 accept golang.org/x/crypto (argon2id only);
  R-2 synchronous send after commit; R-3 privilege-controlled tables like 0004 (but fix the owner-role
  P1); R-4 OIDC optional when password login enabled; R-5 X-Commerce-Client-IP from Caddy only after the
  BFF key check; R-6 0071 released.
- CVS: Q3 store-to-store default, bulk later; Q6 merchant clicks per order; Q8 bulk
  create/print as follow-up contract; Q9 consent line under the chosen store; Q10 manual services also
  use the ECPay map when connected; Q4 run the OK mart probe first, hide OK mart as "coming soon" if it
  fails (no second provider in R2).
- CVS owner clarification (2026-09-30, supersedes Q5 "no COD in v1"): the chain is
  buyer picks a store by number/name -> the store's full address is backfilled into checkout ->
  merchant ships with that address -> for pay-at-pickup the carrier collects from the buyer.
  C1 Store source, two modes per store: (a) ECPAY_MAP when the store has an ECPay logistics connection
     (TD3 unchanged: e-map return + GetStoreList verification, address from the directory row);
     (b) MANUAL when it has none: buyer enters chain + store number + store name + store address, with
     links to each chain's official store search; server validates the code format per chain; the
     snapshot is labelled source=buyer_entered (never "verified"); the merchant sees that label at
     shipment. No scraping, no chain private endpoints (AGENTS.md).
  C2 Pay-at-pickup (超商取貨付款) is an order payment mode, not a Stripe payment: the order is created
     with payment_mode=pay_at_pickup, stock follows the normal HELD->COMMITTED path (§11.5) at order
     placement, no Stripe session. The platform never holds the money: with an ECPay connection the
     create sends IsCollection=Y + CollectionAmount=order total (I05: server-computed, TWD integer
     1..20000, F17); without one the merchant collects through their own channel (e.g. 7-11 賣貨便 /
     全家 好賣+). Merchant marks "collected" (audited) or the ECPay pickup status (2067/3022) does it.
     Unclaimed/returned (2074/3020) -> order returned, stock handling via the existing refund/return
     path, no money movement. Refunds on pay-at-pickup orders are offline records only.
  C3 Pickup-and-pay needs recipient real name (as on ID) + TW mobile (09xxxxxxxx) in the destination
     snapshot; buyer PII rules unchanged (no fixtures/logs).
  C4 Merchant-selectable per store: which chains, pay-at-pickup on/off, pay-at-pickup max amount.
- Ads: O4 per-store ceiling NT$0 (off) until the operator sets it; O6 purchase-events disclosure,
  unchecked by default; O7 test dataset in the 香港大碗 business; O8 automatic placements; O3 one App
  Review submission after sandbox gates. O2 is replaced by O-C (香港大碗 verification).
- Stripe live: LQ3 canary cap 2x currency minimum, per-order max NT$20,000 for 30 days; LQ4 Radar CVC
  rule on; LQ5 production LIVE-only with a separate staging deployment for SANDBOX; LQ8 approval
  reference format as drafted; LQ2 secrets typed by Claude on the server from owner-supplied files
  (O-D), never pasted in chat.
- Customers/billing: Q3 per store; Q4 pilot stores unrestricted (UNBILLED); Q5 limits shown, not
  blocking; Q6 arrears restricts new claim windows + new ads only; Q7 'unpaid' after retries; Q8 platform
  standard notice, no automatic purge until periods set; Q9 instructions page for Meta data deletion;
  Q10 buyer self-service erasure with typed confirmation; Q11 UTC+8 finance day; Q12 consent channels
  Messenger/IG DM + Meta ads personalization only.

Genuine owner inputs (keep as blocking owner questions with a default; the integrator will ask them):
- CVS Q1/Q2: ECPay logistics requires a Taiwan individual/company. Default (no answer needed to build):
  each merchant's own ECPay account (BYO, TD2); stores without one run MANUAL (C1b).
- Stripe live LQ1: first live store + confirm the live Stripe account holder; LQ6 statement descriptor
  text; LQ7 policy pages content owner.
- Billing Q1/Q2: platform billing entity + plan price/currency.
- Ads O5: budget for the owner's own LIVE ad test.

Amendment rules: fix every P0/P1 at the root with the reviewer's text or an equal-or-stronger fix;
verify each against the real code/SQL; rebut wrong findings with evidence; keep the defaults above.

## Round-3 integrator rulings (2026-09-30) — after two amend rounds (AGENTS.md: escalate, don't loop)
Findings: output/contract-review/r2-round3.json. stripe-live PASS_WITH_P2; four still BLOCK on P1.
- X1 Every round-3 P1 fix is accepted with the reviewer's exact text (auth ip-mail-unauth rows; CVS
  plan_cvs_create / apply_cvs_create_result / hooks-host routing; ads pause-vs-activate) and each
  contract applies its cheap round-3 P2s. No further full review round: the fixes are text given by the
  reviewer; only new material (CVS §16) gets one verification review.
- X2 Ads reconcile-mode Check: take the additive option — `DispatchRequest.Mode` ("dispatch"|"reconcile")
  set by the dispatcher; ads/CAPI Check returns nil in reconcile mode; existing routes ignore it (no
  behaviour change). external-dispatcher-v1 gets a one-line amendment note.
- X3 Auth: R-2 deviation (sign-up/reset send detached after 202, login synchronous) accepted — it closes
  the timing oracle. §12 Q2 residual distributed lockout: default accepted (operator unlock runbook;
  CAPTCHA contract on first incident).
- X4 Billing P1: the claims production-mount purge blocker stays in force. Waiver W1: the pilot (owner's
  own store only) may run claims in production until U08 (retention purge + actor-level deletion of
  bundles.actor_key, bindings, claims.links hashes, claims.meta_intake, social.*) lands; U08 is an R2
  unit and must land before a second merchant is onboarded.
- X5 CVS C1–C4 go into the same contract as §16 (not a separate contract), before 0072 freezes.
- X6 After X1/X5, contracts are FROZEN at "v1 FROZEN 2026-09-30"; remaining owner questions keep
  their defaults and never block implementation.
- X7 Ads: a paused draft never re-activates. Resume = copy into a new draft (new id, fresh approval,
  preflight and allowance). Applied in meta-ads-v1 §6.3.
- X8 CVS pay-at-pickup cancel/restock (supersedes §16.4/§16.7 "allocation stays until the cancel/restock
  contract"): without it anonymous junk orders lock stock and the max_open slots with no way out.
  Add to taiwan-cvs-logistics-v1 §16.8: merchant (fulfillment:write) cancels a pay-at-pickup order that
  is PENDING collection and not handed to a provider (no ECPay create SUCCEEDED/UNKNOWN), and marks a
  RETURNED order "restocked"; both release the order's COMMITTED allocation through one audited
  inventory definer (§11.5 evidence: order id + collection_state + actor), idempotent, no money rows.
  Card orders keep the existing no-cancel rule (stripe-refund-v1 RD6).
- X9 The seven CVS verification P2s (TCV11 vs §16.1, MG01/MG02 undefined, buyer-entered rows readable
  store-wide, kill-switch mode mismatch, BUYER/ALLOCATE ledger CHECK, start_stripe_payment citation,
  EXECUTE grants vs the checkout pool) are fixed in the contract before implementation.
