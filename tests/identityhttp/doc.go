// Purpose: the package comment (PROCESS.md §5: what the package owns and what it never does).
// Depends on: nothing (comment only).
// Used by: go doc, scripts/dev/check-pkgdocs.sh, scripts/dev/depmap.sh (docs/engineering/dependency-map.md).

// Package reauthhttptests owns the OC-15 recent re-authentication HTTP gates (contracts/merchant-password-auth-v1.md §16):
// the re-auth routes of internal/identityhttp (password start/complete, OIDC start, the re-auth branch of login/complete),
// the 403 reauth_required / owner_role_disabled transport of the domains it asserts (staff invite, set-role, remove and
// accept; orders CSV, finance CSV, products report CSV, customer erasure, bank-transfer confirm and refund-offline, platform
// Stripe settings and billing checkout through internal/httpapi), and the httperror code table (files *_test.go). Its real-PostgreSQL tests never run without the disposable
// lc_foundation_test database (LC_TEST_DATABASE_ALLOWED=1; otherwise they SKIP = NOT_RUN, never a PASS).
package reauthhttptests
