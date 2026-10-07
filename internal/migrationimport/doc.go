// Purpose: the package comment (PROCESS.md §5: what the package owns and what it never does).
// Depends on: nothing (comment only).
// Used by: go doc, scripts/dev/check-pkgdocs.sh, scripts/dev/depmap.sh (docs/engineering/dependency-map.md).

// Package migrationimport owns the merchant CSV import framework (W5-02B customers, W5-03B historical orders;
// contracts/migration-import-v1.md): coded refusals, the file digest that keys a batch, the preview/commit loop and the
// batch record / results readers over the migrationimport SECURITY DEFINER functions (migrations 0152, 0156).
//
// It never logs, stores or echoes uploaded CSV bytes (only a PG error code or a fixed reason leaves the package),
// never infers a city outside internal/twcity's closed list, and never writes customer data except through the definers.
package migrationimport
