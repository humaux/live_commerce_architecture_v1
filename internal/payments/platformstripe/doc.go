// Purpose: package documentation of the merchant-facing platform-Stripe service (see service.go).
// Depends on: nothing at runtime (documentation only).
// Used by: go doc, docs/engineering/dependency-map.md.

// Package platformstripe owns the merchant-facing side of platform Stripe card payments
// (contracts/stripe-platform-account-v1.md §3.3): read the store's state and enable or disable card payments through the
// designated platform Stripe account. Each call is one commerce_runtime transaction around one SQL definer
// (payments.read_platform_stripe / payments.set_platform_stripe) that decides every rule: billing:manage, the operator
// allowlist (AD-PF2), platform state, terms, descriptor suffix, own-account refusal, CAS.
// It never calls Stripe, never sees an account id, key, approval or tenant id from the request (scope comes from the
// authenticated token), never takes an amount, and returns only coded refusals, never a driver message.
// Depends on: internal/platform (scope transaction), pgx. Used by: internal/httpapi (payment_card.go).
package platformstripe
