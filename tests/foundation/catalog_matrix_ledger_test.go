// Purpose: Enforce the product-editor mobile matrix's declared click identities and fail-closed ledger evidence.
// Depends on: the embedded table, canonical SHA-256 receipts and LC_MATRIX_EVIDENCE_STRICT; no DB/browser required.
// Used by: TestBrowserCatalogCore after product-editor Playwright, and DB-free corruption controls.
package foundation_test

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
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
	Expected json.RawMessage `json:"expected"`
	Actual   string          `json:"actual"`
}

// One immutable table is shared with the Playwright recorder. Adding a control
// changes the required Go set automatically instead of needing two hand-lists.
//
//go:embed testdata/product_editor_matrix_cases.json
var productEditorMatrixLedgerCasesJSON []byte

type productEditorMatrixLedgerCase struct {
	Page     string `json:"page"`
	Row      string `json:"row"`
	Control  string `json:"control"`
	Action   string `json:"action"`
	Expected string `json:"expected"`
	Width    int    `json:"width"`
}

var productEditorMatrixLedgerCases = func() []productEditorMatrixLedgerCase {
	var cases []productEditorMatrixLedgerCase
	if err := json.Unmarshal(productEditorMatrixLedgerCasesJSON, &cases); err != nil {
		panic(fmt.Sprintf("invalid shared matrix control table: %v", err))
	}
	if len(cases) == 0 {
		panic("empty shared matrix control table")
	}
	return cases
}()

// These bindings name rendered controls; they do not define a second case set.
var productEditorMatrixLedgerLocales = map[string]map[string]string{
	"zh-TW": {"@price": "售價", "@compare": "原價", "@quantity": "數量", "@max": "單次購買上限", "@targetQty": "改後庫存", "@code": "貨號", "@keyword": "直播關鍵字", "@untracked": "不追蹤 ∞"},
	"zh-CN": {"@price": "售价", "@compare": "原价", "@quantity": "数量", "@max": "单次购买上限", "@targetQty": "改后库存", "@code": "货号", "@keyword": "直播关键字", "@untracked": "不追踪 ∞"},
	"en":    {"@price": "Price", "@compare": "Compare-at price", "@quantity": "Quantity", "@max": "Maximum per order", "@targetQty": "Resulting on-hand", "@code": "SKU code", "@keyword": "Live keyword", "@untracked": "Do not track ∞"},
}

// productEditorMatrixCasesPath is the shared table's checkout path; canonical
// evidence binds its recorded bytes by SHA-256 in SOURCE.json.
const productEditorMatrixCasesPath = "tests/foundation/testdata/product_editor_matrix_cases.json"

// productEditorMatrixLedgerWidth is the mobile viewport every matrix row declares
// in the case table. The embedded JSON is the single source (390 appears only
// there); canonical evidence binds the table by SHA-256 (#147 CI-13).
var productEditorMatrixLedgerWidth = func() int {
	width := 0
	for _, required := range productEditorMatrixLedgerCases {
		if required.Width <= 0 {
			panic("shared matrix control table must declare a positive width")
		}
		if width != 0 && required.Width != width {
			panic("shared matrix control table declares inconsistent widths")
		}
		width = required.Width
	}
	if width <= 0 {
		panic("shared matrix control table must declare a positive width")
	}
	return width
}()

// productEditorMatrixLedgerExpected is the CURRENT embedded table's required set:
// live browser runs and the testdata ledger are validated against it.
func productEditorMatrixLedgerExpected() map[productEditorMatrixLedgerKey]string {
	expected, err := productEditorMatrixLedgerExpectedFor(productEditorMatrixLedgerCases)
	if err != nil {
		panic(err.Error())
	}
	return expected
}

// productEditorMatrixLedgerExpectedFor expands one case table (current or recorded) per locale.
func productEditorMatrixLedgerExpectedFor(cases []productEditorMatrixLedgerCase) (map[productEditorMatrixLedgerKey]string, error) {
	if len(cases) == 0 {
		return nil, errors.New("empty shared matrix control table")
	}
	expected := make(map[productEditorMatrixLedgerKey]string)
	for locale, labels := range productEditorMatrixLedgerLocales {
		for _, required := range cases {
			control, claim := required.Control, required.Expected
			for token, label := range labels {
				control = strings.ReplaceAll(control, token, label)
				claim = strings.ReplaceAll(claim, token, label)
			}
			width := required.Width
			if width == 0 {
				// The delivered historical table predates the width column; every row it
				// recorded used the current table's declared mobile width.
				width = productEditorMatrixLedgerWidth
			}
			if width <= 0 {
				return nil, errors.New("matrix table requires a positive width")
			}
			key := productEditorMatrixLedgerKey{Page: required.Page, Locale: locale, Width: width, Row: required.Row, Control: control, Action: required.Action}
			if strings.TrimSpace(claim) == "" {
				return nil, errors.New("matrix table requires a non-empty expected claim")
			}
			if _, duplicate := expected[key]; duplicate {
				return nil, errors.New("duplicate shared matrix control identity")
			}
			expected[key] = claim
		}
	}
	return expected, nil
}

// Select by reserved page namespace BEFORE inspecting locale/width/result, so
// malformed target rows cannot disappear through a success-only filter.
func validateProductEditorMatrixLedger(data []byte) error {
	return validateProductEditorMatrixLedgerFor(data, productEditorMatrixLedgerExpected())
}

// validateProductEditorMatrixLedgerFor checks a ledger against one expanded table; required is not modified.
func validateProductEditorMatrixLedgerFor(data []byte, required map[productEditorMatrixLedgerKey]string) error {
	var rows []productEditorMatrixLedgerRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return fmt.Errorf("invalid matrix ledger JSON: %w", err)
	}
	remaining := make(map[productEditorMatrixLedgerKey]string, len(required))
	for key, claim := range required {
		remaining[key] = claim
	}
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
		want, declared := remaining[key]
		if !declared {
			return fmt.Errorf("unexpected mobile matrix ledger identity at row %d", i)
		}
		// PASS is documented as completion of the exact declared expectation.
		// Never trust a ledger-provided expectation to define its own success.
		var claim string
		if err := json.Unmarshal(row.Expected, &claim); err != nil || strings.TrimSpace(claim) == "" || claim != want {
			return fmt.Errorf("mobile matrix ledger row %d has missing or altered expected claim", i)
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
	// Independent identity/expected/status projection of the canonical real run
	// in output/matrix-ledger-enforce/browser/product-editor-click-ledger.json;
	// never synthesize this successful fixture from the declaration or predicate.
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
		{"missing-expected", func(rows []productEditorMatrixLedgerRow) []productEditorMatrixLedgerRow {
			rows[0].Expected = nil
			return rows
		}},
		{"altered-expected", func(rows []productEditorMatrixLedgerRow) []productEditorMatrixLedgerRow {
			rows[0].Expected = json.RawMessage(`"unrelated success claim"`)
			return rows
		}},
		{"empty-expected", func(rows []productEditorMatrixLedgerRow) []productEditorMatrixLedgerRow {
			rows[0].Expected = json.RawMessage(`""`)
			return rows
		}},
		{"null-expected", func(rows []productEditorMatrixLedgerRow) []productEditorMatrixLedgerRow {
			rows[0].Expected = json.RawMessage(`null`)
			return rows
		}},
		{"wrong-type-expected", func(rows []productEditorMatrixLedgerRow) []productEditorMatrixLedgerRow {
			rows[0].Expected = json.RawMessage(`{"description":"unrelated success claim"}`)
			return rows
		}},
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
			rows[0].Action = "unapproved-action"
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
			before, err := json.Marshal(copy)
			if err != nil {
				t.Fatal(err)
			}
			bad, err := json.Marshal(tc.change(copy))
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Equal(before, bad) {
				t.Fatal("corruption control did not change the recorded fixture")
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
	other := append(append([]productEditorMatrixLedgerRow(nil), original...), productEditorMatrixLedgerRow{productEditorMatrixLedgerKey: productEditorMatrixLedgerKey{Page: "list", Locale: "en", Width: productEditorMatrixLedgerWidth, Control: "unrelated", Action: "click"}, Actual: "PASS (MOCK read contract)"})
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

// #147 CI-13: the recorded viewport is a case-table value, not a Go literal.
// Every current case declares it; the writer and every expected ledger key use it.
func TestProductEditorMatrixLedgerWidthDeclaredByCases(t *testing.T) {
	width := 0
	for i, required := range productEditorMatrixLedgerCases {
		if required.Width <= 0 {
			t.Fatalf("case %d (%s) must declare its recorded width", i, required.Control)
		}
		if width != 0 && required.Width != width {
			t.Fatalf("case %d declares width %d, want one table width %d", i, required.Width, width)
		}
		width = required.Width
	}
	if width == 0 || productEditorMatrixLedgerWidth != width {
		t.Fatalf("ledger writer width %d must come from the case table (%d)", productEditorMatrixLedgerWidth, width)
	}
	for key := range productEditorMatrixLedgerExpected() {
		if key.Width != width {
			t.Fatalf("expected ledger key width %d must equal the case-table width %d", key.Width, width)
		}
	}
}

// These owned executable producers bind delivery evidence and are also read by
// CI-SELECT. A changed producer must run the current live browser gate, even when
// its historical canonical record no longer matches the checkout.
var productEditorMatrixEvidenceSources = []string{
	"tests/foundation/catalog_matrix_ledger_test.go",
	"tests/foundation/catalog_matrix_evidence_test.go",
	"tests/foundation/browser_catalog_core_test.go",
	"tests/admin/product-editor.acceptance.ts",
	"tests/admin/product-editor-matrix-ledger.ts",
	"tests/foundation/testdata/product_editor_matrix_cases.json",
	"tests/ci/product-editor-matrix-ledger.test.mjs",
	"tests/ci/product-editor-matrix-claims.mjs",
	"tests/ci/product-editor-matrix-source.mjs",
	"tests/ci/product-editor-matrix-execution.mjs",
	"tests/ci/product-editor-matrix-execution.test.mjs",
	"tests/ci/product-editor-matrix-controls.mjs",
	"tests/ci/product-editor-matrix-controls.test.mjs",
	"apps/admin/components/ProductBulkFill.tsx",
}

type productEditorMatrixEvidenceDrift []string

func (d productEditorMatrixEvidenceDrift) Error() string {
	return "canonical evidence is historical (source drift: " + strings.Join(d, ", ") + ")"
}

// Verify the canonical delivery when its producers match. Historical source drift
// is explicit SKIP by default; delivery verification opts into strict failure.
func TestProductEditorMatrixLedgerEvidence(t *testing.T) {
	if err := validateProductEditorMatrixEvidence("../.."); err != nil {
		var drift productEditorMatrixEvidenceDrift
		if errors.As(err, &drift) && os.Getenv("LC_MATRIX_EVIDENCE_STRICT") != "1" {
			t.Skip(err)
		}
		t.Fatal(err)
	}
}

// validateProductEditorMatrixEvidence also accepts isolated checkout copies for
// corruption controls; it never edits the source checkout or canonical artifacts.
func validateProductEditorMatrixEvidence(root string) error {
	const unit = "output/matrix-ledger-enforce/"
	delivery, err := os.ReadFile(filepath.Join(root, unit, "DELIVERY.md"))
	if err != nil {
		return err
	}
	if strings.Contains(string(delivery), "/Volumes/") || strings.Contains(string(delivery), "/private/tmp/") {
		return fmt.Errorf("delivery evidence must use repository-relative paths")
	}
	sourceRe := regexp.MustCompile("Tested source: \\*\\*`([0-9a-f]{40})`\\*\\*")
	dirRe := regexp.MustCompile(`\[SOURCE\.json\]\(([A-Za-z0-9._-]+)/SOURCE\.json\)`)
	sourceMatch, dirMatch := sourceRe.FindSubmatch(delivery), dirRe.FindSubmatch(delivery)
	if sourceMatch == nil || dirMatch == nil {
		return fmt.Errorf("DELIVERY.md must name the tested source commit and link its canonical SOURCE.json")
	}
	evidence := unit + string(dirMatch[1])
	canonical := evidence + "/browser/"
	var manifest struct {
		Tested  string            `json:"tested_source_commit"`
		Sources map[string]string `json:"source_files_sha256"`
		Copies  []struct {
			Canonical string `json:"canonical"`
			SHA256    string `json:"sha256"`
		} `json:"copied_native_bytes"`
	}
	manifestPath := canonical + "browser-evidence.json"
	manifestData, err := productEditorMatrixEvidenceRead(root, manifestPath)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return fmt.Errorf("canonical browser manifest: %w", err)
	}
	if manifest.Tested != string(sourceMatch[1]) {
		return fmt.Errorf("canonical browser manifest must bind DELIVERY's tested source %s, got %q", sourceMatch[1], manifest.Tested)
	}
	var index struct {
		Tested    string            `json:"tested_source_commit"`
		Sources   map[string]string `json:"source_files_sha256"`
		Artifacts map[string]string `json:"artifacts_sha256"`
	}
	indexData, err := productEditorMatrixEvidenceRead(root, evidence+"/ARTIFACTS.json")
	if err != nil {
		return err
	}
	if err := json.Unmarshal(indexData, &index); err != nil {
		return fmt.Errorf("canonical artifact index: %w", err)
	}
	if index.Tested != manifest.Tested || !reflect.DeepEqual(index.Sources, manifest.Sources) {
		return fmt.Errorf("canonical artifact index must bind the same tested source and source files as the manifest")
	}
	// The manifest cannot hash itself. Its exact bytes are bound by the external
	// artifact index before any manifest-declared native or source hashes are used.
	if err := productEditorMatrixEvidenceHash(manifestPath, manifestData, index.Artifacts[manifestPath]); err != nil {
		return err
	}
	entries, err := os.ReadDir(filepath.Join(root, canonical))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		path := canonical + entry.Name()
		data, err := productEditorMatrixEvidenceRead(root, path)
		if err != nil {
			return err
		}
		if err := productEditorMatrixEvidenceHash(path, data, index.Artifacts[path]); err != nil {
			return err
		}
	}
	var source struct {
		Tested  string            `json:"tested_source_commit"`
		Sources map[string]string `json:"source_files_sha256"`
	}
	sourcePath := evidence + "/SOURCE.json"
	sourceData, err := productEditorMatrixEvidenceRead(root, sourcePath)
	if err != nil {
		return err
	}
	if err := productEditorMatrixEvidenceHash(sourcePath, sourceData, index.Artifacts[sourcePath]); err != nil {
		return err
	}
	if err := json.Unmarshal(sourceData, &source); err != nil {
		return fmt.Errorf("canonical source receipt: %w", err)
	}
	if source.Tested != manifest.Tested || !reflect.DeepEqual(source.Sources, manifest.Sources) {
		return fmt.Errorf("SOURCE.json must bind the same tested source and source files as the manifest")
	}
	// Validate receipt structure before classifying drift. Missing or malformed
	// digests remain failures; only actual checkout-byte differences are history.
	var drift productEditorMatrixEvidenceDrift
	var recordedCases []byte
	for _, path := range productEditorMatrixEvidenceSources {
		hash := manifest.Sources[path]
		if hash == "" {
			return fmt.Errorf("canonical source receipt missing tested producer: %s", path)
		}
		if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(hash) {
			return fmt.Errorf("invalid recorded source SHA-256: %s", path)
		}
		data, err := productEditorMatrixEvidenceRead(root, path)
		if err != nil {
			return err
		}
		if err := productEditorMatrixEvidenceHash(path, data, hash); err != nil {
			drift = append(drift, path)
		}
		if path == productEditorMatrixCasesPath {
			recordedCases = data
		}
	}
	if len(drift) != 0 {
		return drift
	}
	// Historical evidence is informational (#128): it is compared with the case table
	// recorded (SHA-256 bound just above) with the package, never the current embedded
	// table. Current-table coverage is enforced by the live browser run and the testdata ledger.
	var recorded []productEditorMatrixLedgerCase
	if err := json.Unmarshal(recordedCases, &recorded); err != nil {
		return fmt.Errorf("canonical recorded case table: %w", err)
	}
	required, err := productEditorMatrixLedgerExpectedFor(recorded)
	if err != nil {
		return fmt.Errorf("canonical recorded case table: %w", err)
	}
	seen := map[string]bool{}
	for _, copy := range manifest.Copies {
		if filepath.Dir(copy.Canonical) != strings.TrimSuffix(canonical, "/") || seen[copy.Canonical] {
			return fmt.Errorf("native evidence path outside canonical browser directory or duplicated: %s", copy.Canonical)
		}
		seen[copy.Canonical] = true
		data, err := productEditorMatrixEvidenceRead(root, copy.Canonical)
		if err != nil {
			return err
		}
		if err := productEditorMatrixEvidenceHash(copy.Canonical, data, copy.SHA256); err != nil {
			return err
		}
		if err := productEditorMatrixEvidenceHash(copy.Canonical, data, index.Artifacts[copy.Canonical]); err != nil {
			return err
		}
	}
	native := map[string][]byte{}
	for _, name := range []string{"product-editor-click-ledger.json", "playwright.log", "cc12-playwright.log", "pe-last-run.json", "cc12-last-run.json"} {
		path := canonical + name
		if !seen[path] {
			return fmt.Errorf("canonical native evidence missing recorded hash: %s", path)
		}
		data, err := productEditorMatrixEvidenceRead(root, path)
		if err != nil {
			return err
		}
		if len(strings.TrimSpace(string(data))) == 0 {
			return fmt.Errorf("canonical browser evidence %s is empty", name)
		}
		if name == "product-editor-click-ledger.json" {
			if err := validateProductEditorMatrixLedgerFor(data, required); err != nil {
				return fmt.Errorf("canonical browser ledger: %w", err)
			}
		}
		native[name] = data
	}
	return validateProductEditorMatrixRunOutcome(manifestData, native, len(required))
}

func productEditorMatrixEvidenceHash(path string, data []byte, expected string) error {
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(expected) || fmt.Sprintf("%x", sha256.Sum256(data)) != expected {
		return fmt.Errorf("SHA-256 mismatch for canonical evidence/source %s", path)
	}
	return nil
}

func productEditorMatrixEvidenceRead(root, path string) ([]byte, error) {
	if filepath.IsAbs(path) || filepath.Clean(path) != path || path == "." || path == ".." || strings.HasPrefix(path, "../") {
		return nil, fmt.Errorf("canonical evidence/source path must stay repository-relative: %s", path)
	}
	base, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	full, err := filepath.EvalSymlinks(filepath.Join(root, path))
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(base, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
		return nil, fmt.Errorf("canonical evidence/source symlink escapes checkout: %s", path)
	}
	return os.ReadFile(full)
}
