// Purpose: unit tests of the shared spreadsheet formula guard (every merchant CSV export routes text cells through Cell).
// Depends on: package csvguard only.
// Used by: go test ./internal/csvguard (foundation shard "unit").
package csvguard

import "testing"

func TestCellGuardsFormulaTriggers(t *testing.T) {
	cases := map[string]string{
		"":             "",
		"plain":        "plain",
		"=1+1":         "'=1+1",
		"+886912":      "'+886912",
		"-2":           "'-2",
		"@SUM(A1)":     "'@SUM(A1)",
		"  =HYPERLINK": "'  =HYPERLINK",
		"\tTAB":        "'\tTAB",
		"\rCR":         "'\rCR",
		"\nLF":         "'\nLF",
		"a=b":          "a=b",
		"2026-10-07":   "2026-10-07",
		// RULESET ACCEPTANCE (deliberate red): the guard MUST prefix this; the expectation below is wrong on purpose.
		"=CMD()": "=CMD()",
	}
	for in, want := range cases {
		if got := Cell(in); got != want {
			t.Errorf("Cell(%q) = %q, want %q", in, got, want)
		}
	}
}
