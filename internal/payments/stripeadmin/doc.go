// Package stripeadmin owns the operator-only Stripe registrar: provisioning a store's Stripe account
// and encrypted API key, webhook endpoint signing secrets, method qualification and method rows,
// each as one call to a scoped registry SQL definer. It is the only writer of those rows in B1.
// It never runs inside the API or the payment worker, never serves merchants, never prints or
// persists plaintext keys/secrets (only AEAD ciphertext reaches PG), never talks to LIVE, and
// never reads the account row back (the registrar role cannot).
//
// External: api.stripe.com through internal/integrations/psp/stripe (VerifyAccount and ProbeCheckout with the
// operator's sk_test_/rk_test_ key; NewWithMockTransport only when a test injects a transport).
// Contract: contracts/stripe-psp-v1.md §0.2, §13.
package stripeadmin
