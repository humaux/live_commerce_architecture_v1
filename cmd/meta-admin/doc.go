// Command meta-admin owns the operator-only Meta registrar CLI (meta-claims-intake-v1 §7, R1 ruling F2).
// Subcommands:
//   - route: creates or reuses the store's facebook/instagram binding (integration.register_meta_binding)
//     and activates the webhook route (meta_inbox.activate_route) in one transaction. Flags: --tenant
//     --store --principal --app --object page|instagram --asset --proof (sha256 hex of the operator's
//     ownership evidence) --proof-expires (RFC 3339) --expected-epoch (0 creates). Prints binding and route
//     ids/versions.
//   - route-disable: --route --expected-epoch; meta_inbox.disable_route.
//   - page-token: seals a per-store Page access token under the Page-token keyring and calls
//     integration.register_meta_page_token. Flags: --tenant --store --principal --binding --provider
//     --asset --expected-version --scopes (comma-separated attestation from the token debug output).
//
// It never prints a token, key, DSN or driver message (stdout carries ids and versions only, stderr one
// fixed code), never runs in the API or a worker, and never calls Meta: it cannot verify the scopes,
// so --scopes and --proof are the operator's attestation. The authority check is the SQL EXECUTE grant of
// commerce_meta_registrar (the login named by the DSN), not this command. Environment:
// COMMERCE_META_REGISTRAR_DATABASE_URL; page-token also META_PAGE_ACCESS_TOKEN,
// COMMERCE_META_PAGE_TOKEN_ACTIVE_KEY_ID + COMMERCE_META_PAGE_TOKEN_KEYS_JSON. Used by operators and
// the MCI registrar tests; no service starts it.
package main
