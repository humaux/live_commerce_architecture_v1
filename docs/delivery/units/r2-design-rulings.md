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
- CVS: Q3 store-to-store default, bulk later; Q5 no COD in v1; Q6 merchant clicks per order; Q8 bulk
  create/print as follow-up contract; Q9 consent line under the chosen store; Q10 manual services also
  use the ECPay map when connected; Q4 run the OK mart probe first, hide OK mart as "coming soon" if it
  fails (no second provider in R2).
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
- CVS Q1/Q2: ECPay logistics requires a Taiwan individual/company — which Taiwan entity/account will
  hold the logistics contract (each merchant's own, or a platform partner account)?
- Stripe live LQ1: first live store + confirm the live Stripe account holder; LQ6 statement descriptor
  text; LQ7 policy pages content owner.
- Billing Q1/Q2: platform billing entity + plan price/currency.
- Ads O5: budget for the owner's own LIVE ad test.

Amendment rules: fix every P0/P1 at the root with the reviewer's text or an equal-or-stronger fix;
verify each against the real code/SQL; rebut wrong findings with evidence; keep the defaults above.
