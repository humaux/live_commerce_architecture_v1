// Command meta-admin owns the operator-only Meta registrar CLI (meta-claims-intake-v1 §7). Its one
// subcommand, page-token, seals a per-store Page access token under the Page-token keyring and calls
// integration.register_meta_page_token. Flags: --tenant --store --principal --binding --provider
// --asset --expected-version --scopes (comma-separated attestation from the token debug output).
//
// It never prints a token, key, DSN or driver message (stdout carries {"version":N} only, stderr one
// fixed code), never runs in the API or a worker, and never calls Meta: it cannot verify the scopes,
// so --scopes is the operator's attestation. The authority check is the SQL EXECUTE grant of
// commerce_meta_registrar (the login named by the DSN), not this command. Environment:
// COMMERCE_META_REGISTRAR_DATABASE_URL, META_PAGE_ACCESS_TOKEN,
// COMMERCE_META_PAGE_TOKEN_ACTIVE_KEY_ID + COMMERCE_META_PAGE_TOKEN_KEYS_JSON. Used by operators and
// the MCI registrar tests; no service starts it.
package main
