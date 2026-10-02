// Package storefrontdomains owns the merchant self-service domain lifecycle and the platform-primary redirect
// (R5 unit store-domains, Decision 3): a merchant requests a custom hostname (REQUESTED + a TXT verification token
// + DNS instructions), the DNS/TLS worker verifies the proof and completes it to ACTIVE, and every non-primary
// ACTIVE origin 301s to the store's primary origin (the merchant domain when ACTIVE, else the platform subdomain).
//
// It never decides a lifecycle rule itself: every rule is in the SQL definers (control.request_merchant_domain,
// control.read_store_domains, control.suspend_merchant_domain, control.detach_merchant_domain,
// control.store_domain_dns_advance, control.store_domain_tls_complete, control.resolve_primary_origin). This package
// validates inputs at the trust boundary, calls exactly one definer per operation, strictly decodes the jsonb result
// and maps SQLSTATE classes to stable sentinels. It never logs a verification token, never resolves an origin for
// buyers (internal/domains), and never grants authority.
//
// Tables are reached only through the definers: control.storefront_domains, ops.audit_events. Origin grammar is
// reused from internal/domains.ValidOrigin; handle/base grammar from internal/storehandles. It calls no external
// service directly: the DNS lookup and TLS probe are seams (verify.go) the worker fills with real net.DefaultResolver
// and crypto/tls; tests inject fakes.
package storefrontdomains
