// Package stripewebhook owns admission of signed Stripe webhook deliveries for one registered
// endpoint: it verifies the signature with that endpoint's decrypted signing secrets and then
// commits receipt, River signal job and reciprocal signal row in one PG transaction before any ACK.
// It never reads a Stripe API key, decides payment/stock state, trusts unsigned event fields for
// tenant/account selection, ACKs early, or retries a provider call (it makes none).
//
// Depends on:
//   - internal/integrations/psp/stripe (api.stripe.com wire package; WebhookVerifier only, no network here)
//   - internal/integrations/accounts (Keyring.OpenStripeWebhook: separate stripe-webhook-v1 custody)
//   - internal/platform (ValidateStripeIngressPool: the ingress role may execute only the three
//     payments.stripe_webhook_* definers and River InsertTx on river_payment)
//   - internal/jobqueue (ForProfile: server-owned payment queue for the endpoint profile)
//   - PG payments.stripe_webhook_material / _prepare / _commit (contracts/stripe-psp-v1.md §0.2, §6.4)
//
// Used by: cmd/api (stripe_webhook.go mounts NewHandler under /v1/stripe/webhook/{endpoint_id}).
package stripewebhook
