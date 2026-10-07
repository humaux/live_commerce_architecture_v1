// Package csvguard owns the one spreadsheet formula guard shared by every merchant CSV export (merchantorders carrier/order export, reporting).
//
// Purpose: prefix a text cell a spreadsheet could execute with an apostrophe (contract §5.3, A1).
// Depends on: strings only.
// Used by: internal/merchantorders (export.go, carrier_export.go via guardFormula), internal/reporting (report.go cell).
// Invariants: numeric, date and phone cells never start with a trigger after formatting, so they pass unchanged.
package csvguard

import "strings"

// Cell prefixes a cell whose first non-space character is = + - @, or whose first character is TAB, CR or LF, with an apostrophe.
// Pure function, no side effect.
func Cell(cell string) string {
	if cell == "" {
		return cell
	}
	if t := strings.TrimLeft(cell, " \t\r\n"); cell[0] == '\t' || cell[0] == '\r' || cell[0] == '\n' ||
		(t != "" && strings.IndexByte("=+-@", t[0]) >= 0) {
		return "'" + cell
	}
	return cell
}
