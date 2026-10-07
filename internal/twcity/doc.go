// Purpose: the package comment (PROCESS.md §5: what the package owns and what it never does).
// Depends on: nothing (comment only).
// Used by: go doc, scripts/dev/check-pkgdocs.sh, scripts/dev/depmap.sh (docs/engineering/dependency-map.md).

// Package twcity owns the closed list of Taiwan's 22 cities and counties, the only "city" the historical-order archive may
// hold (W5-03B, migration-import-v1 §7), and Normalize, which maps a cell to the canonical spelling (臺 not 台).
//
// It is pure and total and never returns part of its input: a street, name or email can never become a city; an unlisted
// cell is dropped. The same list is the CHECK of customers.historical_orders.city (migration 0156; a test compares the two).
package twcity
