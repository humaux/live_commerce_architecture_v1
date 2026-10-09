// Purpose: Enforce the product-editor mobile matrix's declared click identities and fail-closed ledger evidence.
// Depends on: encoding/json, standard-library I/O and the synthetic PR16 real-click identity fixture; no DB or browser required.
// Used by: TestBrowserCatalogCore after product-editor Playwright, and DB-free corruption controls.
package foundation_test

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

type productEditorMatrixLedgerKey struct {
	Page    string `json:"page"`
	Locale  string `json:"locale"`
	Width   int    `json:"width"`
	Row     string `json:"row"`
	Control string `json:"control"`
	Action  string `json:"action"`
}

type productEditorMatrixLedgerRow struct {
	productEditorMatrixLedgerKey
	Actual string `json:"actual"`
}

// Required identities from product-editor.acceptance.ts mobile matrix/axis-removal
// contract: a single table, expanded across the three frozen locale copy bindings.
// Cardinality comes from this table, never a ledger-length magic number.
var productEditorMatrixLedgerCases = []struct{ page, row, control, action string }{
	{"mobile matrix new", "", "axis-add/axis-name-0/axis-values-0", "click fill Enter"},
	{"mobile matrix new", "", "axis-add/axis-name-1/axis-values-1", "click fill Enter"},
	{"mobile matrix new", "White / S", "matrix-select-0", "check uncheck"},
	{"mobile matrix new", "White / S", "@untracked", "check uncheck; choose mode"},
	{"mobile matrix new", "White / S", "@price", "fill"},
	{"mobile matrix new", "White / S", "@compare", "fill"},
	{"mobile matrix new", "White / S", "@quantity", "fill"},
	{"mobile matrix new", "White / S", "@code", "fill"},
	{"mobile matrix new", "White / S", "@keyword", "fill"},
	{"mobile matrix new", "White / S", "matrix-active-0", "uncheck check"},
	{"mobile matrix new", "Black / S", "matrix-select-1", "check uncheck"},
	{"mobile matrix new", "Black / S", "@untracked", "check uncheck; choose mode"},
	{"mobile matrix new", "Black / S", "@price", "fill"},
	{"mobile matrix new", "Black / S", "@compare", "fill"},
	{"mobile matrix new", "Black / S", "@max", "fill"},
	{"mobile matrix new", "Black / S", "@code", "fill"},
	{"mobile matrix new", "Black / S", "@keyword", "fill"},
	{"mobile matrix new", "Black / S", "matrix-active-1", "uncheck check"},
	{"mobile matrix new", "", "saved axis-0", "UI save, editor reopen/reload"},
	{"mobile matrix new", "", "saved axis-1", "UI save, editor reopen/reload"},
	{"mobile matrix new", "White / S", "saved row readback", "UI save, editor reopen/reload, read visible fields"},
	{"mobile matrix new", "Black / S", "saved row readback", "UI save, editor reopen/reload, read visible fields"},
	{"mobile matrix new", "", "product-create", "click reload"},
	{"mobile matrix edit", "White / S", "matrix-select-0", "check uncheck"},
	{"mobile matrix edit", "White / S", "@untracked", "check uncheck; choose mode"},
	{"mobile matrix edit", "White / S", "@price", "fill"},
	{"mobile matrix edit", "White / S", "@compare", "fill"},
	{"mobile matrix edit", "White / S", "@max", "fill"},
	{"mobile matrix edit", "White / S", "@code", "verify existing SKU code disabled"},
	{"mobile matrix edit", "White / S", "@keyword", "fill"},
	{"mobile matrix edit", "White / S", "matrix-active-0", "uncheck check"},
	{"mobile matrix edit", "Black / S", "matrix-select-1", "check uncheck"},
	{"mobile matrix edit", "Black / S", "@untracked", "check uncheck; choose mode"},
	{"mobile matrix edit", "Black / S", "@price", "fill"},
	{"mobile matrix edit", "Black / S", "@compare", "fill"},
	{"mobile matrix edit", "Black / S", "@targetQty", "fill"},
	{"mobile matrix edit", "Black / S", "@code", "verify existing SKU code disabled"},
	{"mobile matrix edit", "Black / S", "@keyword", "fill"},
	{"mobile matrix edit", "Black / S", "matrix-active-1", "uncheck check"},
	{"mobile matrix edit", "", "saved axis-0", "UI save, editor reopen/reload"},
	{"mobile matrix edit", "", "saved axis-1", "UI save, editor reopen/reload"},
	{"mobile matrix edit", "White / S", "saved row readback", "UI save, editor reopen/reload, read visible fields"},
	{"mobile matrix edit", "Black / S", "saved row readback", "UI save, editor reopen/reload, read visible fields"},
	{"mobile matrix edit", "", "product-save", "click reload"},
	{"mobile matrix edit", "", "matrix-active-0/product-save", "uncheck White, click save, reload"},
	{"mobile axis remove", "", ".pe-axis-remove (Color axis 1)", "real click"},
	{"mobile axis remove", "", "product-save", "click, confirm archive, reload"},
}

// These bindings name rendered controls; they do not define a second case set.
var productEditorMatrixLedgerLocales = map[string]map[string]string{
	"zh-TW": {"@price": "售價", "@compare": "原價", "@quantity": "數量", "@max": "單次購買上限", "@targetQty": "改後庫存", "@code": "貨號", "@keyword": "直播關鍵字", "@untracked": "不追蹤 ∞"},
	"zh-CN": {"@price": "售价", "@compare": "原价", "@quantity": "数量", "@max": "单次购买上限", "@targetQty": "改后库存", "@code": "货号", "@keyword": "直播关键字", "@untracked": "不追踪 ∞"},
	"en":    {"@price": "Price", "@compare": "Compare-at price", "@quantity": "Quantity", "@max": "Maximum per order", "@targetQty": "Resulting on-hand", "@code": "SKU code", "@keyword": "Live keyword", "@untracked": "Do not track ∞"},
}

func productEditorMatrixLedgerExpected() map[productEditorMatrixLedgerKey]bool {
	expected := make(map[productEditorMatrixLedgerKey]bool)
	for locale, labels := range productEditorMatrixLedgerLocales {
		for _, required := range productEditorMatrixLedgerCases {
			control := required.control
			if label, localized := labels[control]; localized {
				control = label
			}
			expected[productEditorMatrixLedgerKey{Page: required.page, Locale: locale, Width: 390, Row: required.row, Control: control, Action: required.action}] = true
		}
	}
	return expected
}

// Select by reserved page namespace BEFORE inspecting locale/width/result, so
// malformed target rows cannot disappear through a success-only filter.
func validateProductEditorMatrixLedger(data []byte) error {
	var rows []productEditorMatrixLedgerRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return fmt.Errorf("invalid matrix ledger JSON: %w", err)
	}
	remaining := productEditorMatrixLedgerExpected()
	seen := make(map[productEditorMatrixLedgerKey]bool)
	for i, row := range rows {
		if row.Page == "" {
			return fmt.Errorf("ledger row %d has no page identity", i)
		}
		if !strings.HasPrefix(row.Page, "mobile matrix") && !strings.HasPrefix(row.Page, "mobile axis remove") {
			continue
		}
		key := row.productEditorMatrixLedgerKey
		if seen[key] {
			return fmt.Errorf("duplicate mobile matrix ledger row %d", i)
		}
		if !remaining[key] {
			return fmt.Errorf("unexpected mobile matrix ledger identity at row %d", i)
		}
		if row.Actual != "PASS" {
			return fmt.Errorf("mobile matrix ledger row %d is not PASS", i)
		}
		seen[key] = true
		delete(remaining, key)
	}
	if len(remaining) != 0 {
		return fmt.Errorf("mobile matrix ledger is missing %d required rows", len(remaining))
	}
	return nil
}

// TestProductEditorMatrixLedger checks independently recorded real-click identities,
// then corrupts fixture copies without a browser, database write or source mutation.
func TestProductEditorMatrixLedger(t *testing.T) {
	// Projected identity/status fields from PR16's actual 196-row evidence ledger:
	// output/pr16-r4/evidence/green/click-ledger.json SHA256 fa17ff01531b6d80a253dfb8acdcc6c7814df4693571f35a4100fdb6a15c7116.
	data, err := os.ReadFile("testdata/product_editor_mobile_matrix_ledger.json")
	if err != nil {
		t.Fatal(err)
	}
	var original []productEditorMatrixLedgerRow
	if err := json.Unmarshal(data, &original); err != nil {
		t.Fatal(err)
	}
	if err := validateProductEditorMatrixLedger(data); err != nil {
		t.Fatalf("recorded complete ledger: %v", err)
	}
	for _, tc := range []struct {
		name   string
		change func([]productEditorMatrixLedgerRow) []productEditorMatrixLedgerRow
	}{
		{"missing-matrix-click", func(rows []productEditorMatrixLedgerRow) []productEditorMatrixLedgerRow { return rows[1:] }},
		{"missing-axis-remove", func(rows []productEditorMatrixLedgerRow) []productEditorMatrixLedgerRow {
			for i, row := range rows {
				if row.Page == "mobile axis remove" {
					return append(rows[:i], rows[i+1:]...)
				}
			}
			t.Fatal("fixture lacks axis removal")
			return rows
		}},
		{"failed-row", func(rows []productEditorMatrixLedgerRow) []productEditorMatrixLedgerRow {
			rows[0].Actual = "FAIL"
			return rows
		}},
		{"missing-PASS", func(rows []productEditorMatrixLedgerRow) []productEditorMatrixLedgerRow {
			rows[0].Actual = ""
			return rows
		}},
		{"PASS-prefix-is-not-PASS", func(rows []productEditorMatrixLedgerRow) []productEditorMatrixLedgerRow {
			rows[0].Actual = "PASS (MOCK)"
			return rows
		}},
		{"duplicate", func(rows []productEditorMatrixLedgerRow) []productEditorMatrixLedgerRow { return append(rows, rows[0]) }},
		{"unexpected-extra-control", func(rows []productEditorMatrixLedgerRow) []productEditorMatrixLedgerRow {
			extra := rows[0]
			extra.Control = "unapproved-control"
			return append(rows, extra)
		}},
		{"wrong-locale", func(rows []productEditorMatrixLedgerRow) []productEditorMatrixLedgerRow {
			rows[0].Locale = "ja"
			return rows
		}},
		{"wrong-width", func(rows []productEditorMatrixLedgerRow) []productEditorMatrixLedgerRow {
			rows[0].Width = 1440
			return rows
		}},
		{"wrong-action", func(rows []productEditorMatrixLedgerRow) []productEditorMatrixLedgerRow {
			rows[0].Action = "fill"
			return rows
		}},
		{"wrong-top-level-row", func(rows []productEditorMatrixLedgerRow) []productEditorMatrixLedgerRow {
			rows[0].Row = "White / S"
			return rows
		}},
		{"unknown-target-page", func(rows []productEditorMatrixLedgerRow) []productEditorMatrixLedgerRow {
			rows[0].Page = "mobile matrix unknown"
			return rows
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			copy := append([]productEditorMatrixLedgerRow(nil), original...)
			bad, err := json.Marshal(tc.change(copy))
			if err != nil {
				t.Fatal(err)
			}
			if err := validateProductEditorMatrixLedger(bad); err == nil {
				t.Fatal("corrupted ledger admitted by the Go gate")
			}
		})
	}
	for name, bad := range map[string][]byte{"empty": []byte("[]"), "null": []byte("null"), "invalid-json": []byte("["), "object-instead-of-ledger": []byte("{}")} {
		t.Run(name, func(t *testing.T) {
			if validateProductEditorMatrixLedger(bad) == nil {
				t.Fatal("malformed/empty ledger admitted")
			}
		})
	}
	// Real product-editor ledgers also contain generic form/list clicks. Those are
	// outside this matrix contract, including their distinct MOCK result labels.
	other := append(append([]productEditorMatrixLedgerRow(nil), original...), productEditorMatrixLedgerRow{productEditorMatrixLedgerKey: productEditorMatrixLedgerKey{Page: "list", Locale: "en", Width: 390, Control: "unrelated", Action: "click"}, Actual: "PASS (MOCK read contract)"})
	mixed, _ := json.Marshal(other)
	if err := validateProductEditorMatrixLedger(mixed); err != nil {
		t.Fatalf("unrelated legitimate ledger row: %v", err)
	}
	for i, j := 0, len(original)-1; i < j; i, j = i+1, j-1 {
		original[i], original[j] = original[j], original[i]
	}
	reordered, _ := json.Marshal(original)
	if err := validateProductEditorMatrixLedger(reordered); err != nil {
		t.Fatalf("set equality must not invent an ordering constraint: %v", err)
	}
}
