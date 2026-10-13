// Purpose: Enforce the product-editor mobile matrix's declared click identities and fail-closed ledger evidence.
// Depends on: the embedded control table, JSON/SHA-256, canonical manifests and checkout bytes; no DB or browser required.
// Used by: TestBrowserCatalogCore after product-editor Playwright, and DB-free corruption controls.
package foundation_test

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
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

func productEditorMatrixLedgerExpected() map[productEditorMatrixLedgerKey]string {
	expected := make(map[productEditorMatrixLedgerKey]string)
	for locale, labels := range productEditorMatrixLedgerLocales {
		for _, required := range productEditorMatrixLedgerCases {
			control, claim := required.Control, required.Expected
			for token, label := range labels {
				control = strings.ReplaceAll(control, token, label)
				claim = strings.ReplaceAll(claim, token, label)
			}
			key := productEditorMatrixLedgerKey{Page: required.Page, Locale: locale, Width: 390, Row: required.Row, Control: control, Action: required.Action}
			if strings.TrimSpace(claim) == "" {
				panic("matrix table requires a non-empty expected claim")
			}
			if _, duplicate := expected[key]; duplicate {
				panic("duplicate shared matrix control identity")
			}
			expected[key] = claim
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

// TestProductEditorMatrixLedgerEvidence verifies the canonical run DELIVERY selects.
func TestProductEditorMatrixLedgerEvidence(t *testing.T) {
	if err := validateProductEditorMatrixEvidence("../.."); err != nil {
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
	// Bind only this unit's executable producers. CI checks the PR merge ref;
	// shared harness/config/product hashes describe the run informationally and
	// must not reject an unrelated trunk change. Required unit sources stay fixed
	// independently of the receipts, so removing an entry cannot exempt drift.
	for _, path := range []string{
		"tests/foundation/catalog_matrix_ledger_test.go",
		"tests/foundation/browser_catalog_core_test.go",
		"tests/admin/product-editor.acceptance.ts",
		"tests/admin/product-editor-matrix-ledger.ts",
		"tests/foundation/testdata/product_editor_matrix_cases.json",
		"tests/ci/product-editor-matrix-ledger.test.mjs",
		"tests/ci/product-editor-matrix-claims.mjs",
		"tests/ci/product-editor-matrix-source.mjs",
	} {
		hash := manifest.Sources[path]
		if hash == "" {
			return fmt.Errorf("canonical source receipt missing tested producer: %s", path)
		}
		data, err := productEditorMatrixEvidenceRead(root, path)
		if err != nil {
			return err
		}
		if err := productEditorMatrixEvidenceHash(path, data, hash); err != nil {
			return err
		}
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
			if err := validateProductEditorMatrixLedger(data); err != nil {
				return fmt.Errorf("canonical browser ledger: %w", err)
			}
		}
	}
	return nil
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

// TestProductEditorMatrixLedgerEvidenceCorruption exercises the real validator
// with isolated filesystem mutations, never the delivered evidence in place.
func TestProductEditorMatrixLedgerEvidenceCorruption(t *testing.T) {
	for _, mutation := range []string{"playwright-log", "cc12-log", "ledger", "manifest", "source", "gate-source"} {
		t.Run(mutation, func(t *testing.T) {
			root, canonical := productEditorMatrixEvidenceFixture(t)
			if err := validateProductEditorMatrixEvidence(root); err != nil {
				t.Fatalf("untampered isolated evidence: %v", err)
			}
			path := canonical + "/playwright.log"
			var data []byte
			switch mutation {
			case "playwright-log":
				data = []byte("arbitrary text is not Playwright evidence\n")
			case "cc12-log":
				path = canonical + "/cc12-playwright.log"
				data = []byte("arbitrary CC12 log\n")
			case "ledger":
				path = canonical + "/product-editor-click-ledger.json"
				data = productEditorMatrixReadFixture(t, root, path)
				data = append(data, '\n') // Valid unchanged row semantics still require exact native bytes.
			case "manifest":
				path = canonical + "/browser-evidence.json"
				var manifest map[string]any
				if err := json.Unmarshal(productEditorMatrixReadFixture(t, root, path), &manifest); err != nil {
					t.Fatal(err)
				}
				manifest["scope"] = "edited while retaining the tested source commit"
				data, _ = json.Marshal(manifest)
			case "source", "gate-source":
				path = "tests/admin/product-editor.acceptance.ts"
				if mutation == "gate-source" {
					path = "tests/foundation/catalog_matrix_ledger_test.go"
				}
				data = append(productEditorMatrixReadFixture(t, root, path), []byte("\n// changed after the recorded run\n")...)
			}
			if err := os.WriteFile(filepath.Join(root, path), data, 0600); err != nil {
				t.Fatal(err)
			}
			if err := validateProductEditorMatrixEvidence(root); err == nil {
				t.Fatalf("%s corruption must reject canonical evidence", mutation)
			} else {
				t.Logf("rejected %s: %v", mutation, err)
			}
		})
	}
}

func productEditorMatrixReadFixture(t *testing.T, root, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, path))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// Shared harness/configuration changes in a PR merge ref do not invalidate the
// unit's source-bound evidence; its executable producers remain fail-closed.
func TestProductEditorMatrixLedgerEvidenceSharedSources(t *testing.T) {
	for _, path := range []string{
		"scripts/dev/test-node.sh", "scripts/dev/test-local.sh", "docs/delivery/GATES.md", "playwright.config.ts",
		"tests/foundation/testdata/product_editor_mobile_matrix_ledger.json",
		"apps/admin/lib/product-editor-copy.ts", "apps/admin/components/ProductDocumentVariants.tsx",
		"apps/admin/components/ProductDocumentForm.tsx", "apps/admin/lib/use-product-document.ts", "apps/admin/lib/product-document.ts",
	} {
		t.Run(path, func(t *testing.T) {
			root, _ := productEditorMatrixEvidenceFixture(t)
			data := append(productEditorMatrixReadFixture(t, root, path), []byte("\n// shared merge-ref change\n")...)
			if err := os.WriteFile(filepath.Join(root, path), data, 0600); err != nil {
				t.Fatal(err)
			}
			if err := validateProductEditorMatrixEvidence(root); err != nil {
				t.Fatalf("informational shared source change must not invalidate unit evidence: %v", err)
			}
		})
	}
}

func productEditorMatrixEvidenceFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	const canonical = "output/matrix-ledger-enforce/fixture/browser"
	delivery := productEditorMatrixReadFixture(t, "../..", "output/matrix-ledger-enforce/DELIVERY.md")
	dir := regexp.MustCompile(`\[SOURCE\.json\]\(([A-Za-z0-9._-]+)/SOURCE\.json\)`).FindSubmatch(delivery)
	if dir == nil {
		t.Fatal("fixture needs DELIVERY's current canonical evidence directory")
	}
	var manifest map[string]any
	if err := json.Unmarshal(productEditorMatrixReadFixture(t, "../..", "output/matrix-ledger-enforce/"+string(dir[1])+"/browser/browser-evidence.json"), &manifest); err != nil {
		t.Fatal(err)
	}
	write := func(path string, data []byte) {
		t.Helper()
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	hash := func(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }
	sources := map[string]string{}
	for path := range manifest["source_files_sha256"].(map[string]any) {
		data := productEditorMatrixReadFixture(t, "../..", path)
		write(path, data)
		sources[path] = hash(data)
	}
	manifest["source_files_sha256"] = sources
	if shared, ok := manifest["informational_source_files_sha256"].(map[string]any); ok {
		for path := range shared {
			data := productEditorMatrixReadFixture(t, "../..", path)
			write(path, data)
			shared[path] = hash(data)
		}
	}
	artifacts := map[string]string{}
	copies := []map[string]string{}
	for _, entry := range manifest["copied_native_bytes"].([]any) {
		old := entry.(map[string]any)
		name := filepath.Base(old["canonical"].(string))
		path := canonical + "/" + name
		data := productEditorMatrixReadFixture(t, "../..", old["canonical"].(string))
		write(path, data)
		artifacts[path] = hash(data)
		copies = append(copies, map[string]string{"canonical": path, "source_native": "fixture-only/" + name, "sha256": hash(data)})
	}
	manifest["copied_native_bytes"] = copies
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	write(canonical+"/browser-evidence.json", data)
	artifacts[canonical+"/browser-evidence.json"] = hash(data)
	source, _ := json.Marshal(map[string]any{"tested_source_commit": manifest["tested_source_commit"], "source_files_sha256": sources})
	write("output/matrix-ledger-enforce/fixture/SOURCE.json", source)
	artifacts["output/matrix-ledger-enforce/fixture/SOURCE.json"] = hash(source)
	index, _ := json.Marshal(map[string]any{"tested_source_commit": manifest["tested_source_commit"], "source_files_sha256": sources, "artifacts_sha256": artifacts})
	write("output/matrix-ledger-enforce/fixture/ARTIFACTS.json", index)
	write("output/matrix-ledger-enforce/DELIVERY.md", []byte(fmt.Sprintf("Tested source: **`%s`**\n[SOURCE.json](fixture/SOURCE.json)\n", manifest["tested_source_commit"])))
	return root, canonical
}
