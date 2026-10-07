// Purpose: the package comment (PROCESS.md §5: what the package owns and what it never does).
// Depends on: nothing (comment only).
// Used by: go doc, scripts/dev/check-pkgdocs.sh, scripts/dev/depmap.sh (docs/engineering/dependency-map.md).

// Package settlement owns the operator CSV of one platform-Stripe settlement statement (contract §6.5
// settlement-export): UTF-8 BOM, formula-guarded text cells (internal/csvguard), a totals block, written once with mode 0600.
//
// It never includes a buyer PII column (an order number is the only order identifier), never overwrites or follows a
// symlink at the target path (O_EXCL), and never moves money: it only renders a statement the stripeadmin path already read.
package settlement
