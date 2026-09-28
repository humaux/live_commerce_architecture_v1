// Package stripeadmin owns the operator-only Stripe registrar: provisioning a store's Stripe account
// and encrypted API key, webhook endpoint signing secrets, method qualification and method rows,
// each as one call to a scoped registry SQL definer. It is the only writer of those rows in B1.
// It never runs inside the API or the payment worker, never serves merchants, never prints or
// persists plaintext keys/secrets (only AEAD ciphertext reaches PG), never talks to LIVE, and
// never reads the account row back (the registrar role cannot).
//
// Depends on:
//   - internal/platform (OpenStripeRegistrarPool: login may execute only the five registry definers)
//   - internal/integrations/accounts (Keyring.SealStripeAPI / SealStripeWebhook: separate AAD per purpose)
//   - internal/integrations/psp/stripe (VerifyAccount and ProbeCheckout call api.stripe.com with the
//     operator's sk_test_/rk_test_ key; NewWithMockTransport only when a test injects a transport)
//   - PG integration.register_stripe_account, integration.rotate_stripe_key,
//     payments.set_stripe_webhook_endpoint, payments.qualify_stripe_method, payments.set_stripe_method
//     (contracts/stripe-psp-v1.md §0.2, §13)
//
// Used by: cmd/stripe-admin (thin CLI) and the SP21 tests.
package stripeadmin
