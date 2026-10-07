// Purpose: own expiry jobs and independently opted-in notification mail loops.
// Depends on: checkout/jobqueue/notify/platform, worker DSN and SMTP/COMMERCE mail flags.
// Used by: expiry-worker executable in deployment and process/configuration tests.
// Command expiry-worker owns the process that runs the checkout-expiry River queue
// (jobqueue.CheckoutExpiry): it turns due unpaid checkout holds into released stock through
// internal/checkout.ExpiryWorker, using its own worker DB login.
//
// It hosts the buyer notification mail loop (internal/notify, contracts/storefront-v2.md
// §E) when COMMERCE_BUYER_MAIL_ENABLED=1. Either mail opt-in needs the SMTP variables of the login-code mail
// (COMMERCE_SMTP_HOST, _USERNAME, _PASSWORD[_FILE], COMMERCE_MAIL_FROM, COMMERCE_MAIL_DAILY_CAP) and
// calls that SMTP host only.
//
// With COMMERCE_MERCHANT_ALERT_MAIL=1, it independently drains notify.merchant_alerts (the meta
// connection-health owner mail, contract meta-connection-health-v1 §5.2) through the same SMTP account; that
// mail's reconnect link is exactly COMMERCE_ADMIN_ORIGIN (never a storefront or Meta URL, §10). An origin
// alone does not opt in; both mail flags default off and share SMTP configuration when either is enabled.
//
// It never accepts or reconciles payment, never calls a payment provider, never serves HTTP, and never
// starts unless COMMERCE_EXPIRY_WORKER_ENABLED=1 (COMMERCE_EXPIRY_WORKER_DATABASE_URL required,
// COMMERCE_EXPIRY_WORKER_CONCURRENCY 1..16). With both mail flags off it calls no external host.
package main
