// Command store-admin owns the operator-only storefront domain CLI (R3 unit storefront-publish, ruling SP2/SP5):
// the platform's act of binding a public origin to a store after it has verified ownership and TLS out of band.
// Subcommands:
//   - domain-bind --store <uuid> --origin https://host --evidence <ref> --valid-until <RFC 3339>: makes the origin
//     ACTIVE for the store in one audited call (creates the row, or advances/renews a non-DETACHED one), stamping
//     ownership and TLS verification with the database clock. --valid-until is the TLS certificate notAfter
//     (`openssl x509 -noout -enddate`, docs/runbooks/merchant-onboarding.md) and must lie within 400 days.
//   - domain-suspend --origin https://host: SUSPENDED; the resolver denies the origin on its next request.
//   - domain-detach  --origin https://host: DETACHED for good; the origin is never re-bound (contract).
//   - status --store <uuid>: prints the store's publication state and domain rows (ids, states, versions, origins).
//
// It never prints a DSN, evidence text or driver message (stdout carries ids, versions and origins only, stderr one
// fixed store_admin_* code), never runs in the API or a worker, never publishes a store (the merchant does that in
// Settings: two separate consents) and cannot verify DNS or TLS: --evidence and --valid-until are the operator's
// attestation. The authority check is the SQL EXECUTE grant of commerce_storefront_registrar (the login named by the
// DSN), not this command. Environment: COMMERCE_STORE_REGISTRAR_DATABASE_URL. Used by deploy/scripts/ops-admin.sh
// (compose service store-admin, profile ops) and the storefront-publish gate tests; no service starts it.
//
// ponytail: one store host per deployment today (Caddy serves exactly LC_STORE_HOST, so one ACTIVE origin). Per-store
// hosts need Caddy on_demand_tls with an `ask` endpoint backed by the resolver (internal/domains), not this CLI.
package main
