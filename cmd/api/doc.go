// Command api owns the API process assembly: it loads each feature's configuration (identity,
// accounts, buyer and hosted payment, Meta webhooks, Stripe webhooks, Studio, claims, merchant
// refunds), opens the scoped DB pools, builds the handlers and mounts them on one listener. Feature
// files (buyer.go, buyer_payment.go, stripe_webhook.go, merchant_refund.go, ...) each own one
// feature's config and wiring only.
//
// It never starts a worker or dispatches a provider call (workers are cmd/*-worker; the API only
// inserts River jobs and admits signed webhooks), never reads STRIPE_* secrets, and holds no
// business rule: routes and rules live in internal/httpapi and the domain packages. External
// services: none called at request time except the OIDC issuer during identity login
// (internal/oidclogin).
package main
