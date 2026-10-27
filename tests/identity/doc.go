// Purpose: the package comment (PROCESS.md §5: what the package owns and what it never does).
// Depends on: nothing (comment only).
// Used by: go doc, scripts/dev/check-pkgdocs.sh, scripts/dev/depmap.sh (docs/engineering/dependency-map.md).

// Package reauthtests owns the OC-02 recent re-authentication gates (contracts/merchant-password-auth-v1.md §16):
// real-PostgreSQL tests of migration 0201 (authenticated_at backfill, identity.require_recent_auth, the re-auth
// definers and their privileges) and of the internal/identity re-auth flows (files *_test.go). It never runs
// without the disposable lc_foundation_test database (LC_TEST_DATABASE_ALLOWED=1; otherwise every test is a SKIP =
// NOT_RUN, never a PASS) and never asserts the guarded business definers, which belong to OC-03.
package reauthtests
