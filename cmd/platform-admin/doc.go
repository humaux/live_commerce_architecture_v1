// Command platform-admin owns the operator-only platform console (R3 unit OPS-01B; the console is CLI-only by owner ruling):
// suspend or resume a merchant (tenant) or one store, read their state, and list the operator audit.
// Subcommands (every mutation needs --operator <name> and --ticket <ref>; suspend also needs --reason):
//   - store-suspend  --store <uuid> --reason fraud|non_payment|legal|owner_request|other
//   - store-resume   --store <uuid>
//   - tenant-suspend --tenant <uuid> --reason <code>      (every store of the tenant stops; per-store flags are kept)
//   - tenant-resume  --tenant <uuid>
//   - status --store <uuid> | --tenant <uuid>             (active flags only)
//   - audit [--since <RFC 3339>] [--limit 1..500]         (newest first)
//
// Suspension flips control.stores.active / control.tenants.active (migration 0143): the merchant API, buyer capabilities,
// the published-store resolver, stored-principal sends/ads Checks, automatic claim replies and mail claiming stop at
// once. It deletes nothing, unbinds no PSP/Meta binding, refunds nothing, force-closes no live window and never blocks
// money already in flight. Re-applying the target state prints result "unchanged" (exit 0) and is still audited.
//
// stdout carries ids, flags and audit fields only (one JSON line); stderr one fixed platform_admin_* code. A DSN, a
// driver message or a flag value is never printed. The authority check is the SQL EXECUTE grant of
// commerce_platform_operator (the login named by the DSN), not this command. Environment:
// COMMERCE_PLATFORM_OPERATOR_DATABASE_URL. Used by deploy/scripts/ops-admin.sh (compose service platform-admin, profile
// ops; integrator wiring) and tests/foundation/platform_operator_test.go; no service starts it.
package main
