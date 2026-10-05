// Command expiry-worker owns the process that runs the checkout-expiry River queue
// (jobqueue.CheckoutExpiry): it turns due unpaid checkout holds into released stock through
// internal/checkout.ExpiryWorker, using its own worker DB login.
//
// It also hosts the buyer / merchant notification mail loop (internal/notify, contracts/storefront-v2.md
// §E) when COMMERCE_BUYER_MAIL_ENABLED=1: it then needs the SMTP variables of the login-code mail
// (COMMERCE_SMTP_HOST, _USERNAME, _PASSWORD[_FILE], COMMERCE_MAIL_FROM, COMMERCE_MAIL_DAILY_CAP) and
// calls that SMTP host only.
//
// When COMMERCE_ADMIN_ORIGIN is also set (https), it additionally drains notify.merchant_alerts (the meta
// connection-health owner mail, contract meta-connection-health-v1 §5.2) through the same SMTP account; that
// mail's reconnect link is exactly COMMERCE_ADMIN_ORIGIN (never a storefront or Meta URL, §10). Unset = the
// merchant-alert loop stays off.
//
// It never accepts or reconciles payment, never calls a payment provider, never serves HTTP, and never
// starts unless COMMERCE_EXPIRY_WORKER_ENABLED=1 (COMMERCE_EXPIRY_WORKER_DATABASE_URL required,
// COMMERCE_EXPIRY_WORKER_CONCURRENCY 1..16). Without COMMERCE_BUYER_MAIL_ENABLED it calls no external host.
package main
