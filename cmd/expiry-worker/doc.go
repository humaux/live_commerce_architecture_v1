// Command expiry-worker owns the process that runs the checkout-expiry River queue
// (jobqueue.CheckoutExpiry): it turns due unpaid checkout holds into released stock through
// internal/checkout.ExpiryWorker, using its own worker DB login.
//
// It also hosts the buyer / merchant notification mail loop (internal/notify, contracts/storefront-v2.md
// §E) when COMMERCE_BUYER_MAIL_ENABLED=1: it then needs the SMTP variables of the login-code mail
// (COMMERCE_SMTP_HOST, _USERNAME, _PASSWORD[_FILE], COMMERCE_MAIL_FROM, COMMERCE_MAIL_DAILY_CAP) and
// calls that SMTP host only.
//
// It never accepts or reconciles payment, never calls a payment provider, never serves HTTP, and never
// starts unless COMMERCE_EXPIRY_WORKER_ENABLED=1 (COMMERCE_EXPIRY_WORKER_DATABASE_URL required,
// COMMERCE_EXPIRY_WORKER_CONCURRENCY 1..16). Without COMMERCE_BUYER_MAIL_ENABLED it calls no external host.
package main
