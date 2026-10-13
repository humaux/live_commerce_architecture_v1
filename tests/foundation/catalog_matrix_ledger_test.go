// Purpose: Enforce the product-editor mobile matrix's declared click identities and fail-closed ledger evidence.
// Depends on: the embedded table, canonical SHA-256 receipts and LC_MATRIX_EVIDENCE_STRICT; no DB/browser required.
// Used by: TestBrowserCatalogCore after product-editor Playwright, and DB-free corruption controls.
package foundation_test

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
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
	return productEditorMatrixExpected(productEditorMatrixLedgerCases, productEditorMatrixLedgerLocales)
}

func productEditorMatrixExpected(cases []productEditorMatrixLedgerCase, locales map[string]map[string]string) map[productEditorMatrixLedgerKey]string {
	expected := make(map[productEditorMatrixLedgerKey]string)
	for locale, labels := range locales {
		for _, required := range cases {
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
	return productEditorMatrixValidateExpectedLedger(data, productEditorMatrixLedgerExpected())
}

func productEditorMatrixValidateExpectedLedger(data []byte, remaining map[productEditorMatrixLedgerKey]string) error {
	var rows []productEditorMatrixLedgerRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return fmt.Errorf("invalid matrix ledger JSON: %w", err)
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
	"tests/ci/matrix-receipt.test.mjs",
	"tests/ci/ci-plan.test.mjs",
	"tests/ci/pr-modes.test.mjs",
	"scripts/dev/matrix-receipt.mjs",
	"scripts/dev/matrix-receipt-verify.mjs",
	"scripts/dev/matrix-test-reporter.mjs",
	"scripts/dev/matrix-test-inventory.json",
	"scripts/dev/ci-plan.mjs",
	"scripts/dev/pr-modes.mjs",
	"scripts/dev/test-node.sh",
	"scripts/dev/test-local.sh",
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
		Tested     string            `json:"tested_source_commit"`
		Sources    map[string]string `json:"source_files_sha256"`
		FrozenHash string            `json:"frozen_identity_record_sha256"`
		Copies     []struct {
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
	// Every indexed artifact, including Node/policy/CI receipts outside browser/,
	// belongs to this immutable package and must validate before a historical skip.
	paths := make([]string, 0, len(index.Artifacts))
	for path := range index.Artifacts {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		if !strings.HasPrefix(path, evidence+"/") {
			return fmt.Errorf("indexed evidence path outside canonical package: %s", path)
		}
		data, err := productEditorMatrixEvidenceRead(root, path)
		if err != nil {
			return err
		}
		if err := productEditorMatrixEvidenceHash(path, data, index.Artifacts[path]); err != nil {
			return err
		}
	}
	// The entire package is closed, including nested and outer receipts. No
	// symlinks/special files may hide unindexed bytes before a historical skip.
	if err := filepath.WalkDir(filepath.Join(root, evidence), func(full string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("canonical package symlink: %s", full)
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("canonical package non-regular artifact: %s", full)
		}
		path, err := filepath.Rel(root, full)
		if err != nil {
			return err
		}
		path = filepath.ToSlash(path)
		if path == evidence+"/ARTIFACTS.json" {
			return nil
		}
		if _, indexed := index.Artifacts[path]; !indexed {
			return fmt.Errorf("unindexed canonical package artifact: %s", path)
		}
		return nil
	}); err != nil {
		return err
	}
	var source struct {
		Tested     string            `json:"tested_source_commit"`
		Sources    map[string]string `json:"source_files_sha256"`
		FrozenHash string            `json:"frozen_identity_record_sha256"`
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
	if source.Tested != manifest.Tested || !reflect.DeepEqual(source.Sources, manifest.Sources) || source.FrozenHash != manifest.FrozenHash {
		return fmt.Errorf("SOURCE.json must bind the same tested source and source files as the manifest")
	}
	// The separately indexed frozen run receipt corroborates the OLD producer
	// set. A changed gate comment cannot excuse dropping an already tested source.
	runPath := evidence + "/browser-product-editor.json"
	runData, err := productEditorMatrixEvidenceRead(root, runPath)
	if err != nil {
		return err
	}
	if err := productEditorMatrixEvidenceHash(runPath, runData, index.Artifacts[runPath]); err != nil {
		return err
	}
	var run struct {
		Command []string          `json:"command"`
		Exit    *int              `json:"exit_code"`
		Tested  string            `json:"tested_source_commit"`
		After   string            `json:"source_commit_after"`
		Sources map[string]string `json:"source_files_sha256"`
		Final   map[string]string `json:"source_files_sha256_after"`
	}
	if err := json.Unmarshal(runData, &run); err != nil {
		return fmt.Errorf("canonical full browser run receipt: %w", err)
	}
	if !reflect.DeepEqual(run.Command, []string{"bash", "scripts/dev/test-local.sh", "--browser-product-editor"}) || run.Exit == nil || *run.Exit != 0 {
		return fmt.Errorf("canonical browser run receipt must record the original full mode and exit 0")
	}
	if run.Tested != manifest.Tested || run.After != manifest.Tested || !reflect.DeepEqual(run.Sources, manifest.Sources) || !reflect.DeepEqual(run.Final, manifest.Sources) {
		return fmt.Errorf("canonical browser run receipt must bind the same tested commit and unchanged producer hashes as the manifest")
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
		native[name] = data
	}
	frozen, err := productEditorMatrixFrozenEvidence(root, evidence, manifest.Tested, manifest.FrozenHash, manifest.Sources, index.Artifacts)
	if err != nil {
		return err
	}
	// Historical records cannot be compared to a future control table. Their
	// immutable ledger must still be internally coherent and corroborate outcomes.
	required, err := productEditorMatrixHistoricalLedger(native["product-editor-click-ledger.json"])
	if err != nil {
		return fmt.Errorf("canonical browser ledger: %w", err)
	}
	if err := productEditorMatrixValidateExpectedLedger(native["product-editor-click-ledger.json"], productEditorMatrixExpected(frozen.Cases, frozen.Locales)); err != nil {
		return fmt.Errorf("frozen historical claims: %w", err)
	}
	if err := validateProductEditorMatrixRunOutcome(manifestData, native, required, frozen.Tests); err != nil {
		return err
	}
	// Validate ALL recorded digests first: an absent newly introduced path can be
	// history, but a present malformed digest is always a broken receipt.
	const gate = "tests/foundation/catalog_matrix_ledger_test.go"
	const table = "tests/foundation/testdata/product_editor_matrix_cases.json"
	for _, path := range []string{gate, table} {
		if manifest.Sources[path] == "" {
			return fmt.Errorf("canonical source receipt missing tested producer: %s", path)
		}
	}
	var drift productEditorMatrixEvidenceDrift
	// SOURCE.json's corroborated owned producer set defines checkout drift.
	// Frozen consumers prove historical identities above without expanding it.
	sourcePaths := make([]string, 0, len(manifest.Sources))
	for path := range manifest.Sources {
		sourcePaths = append(sourcePaths, path)
	}
	sort.Strings(sourcePaths)
	gateChanged := false
	for _, path := range sourcePaths {
		hash := manifest.Sources[path]
		if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(hash) {
			return fmt.Errorf("invalid recorded source SHA-256: %s", path)
		}
		data, err := productEditorMatrixEvidenceSourceRead(root, path)
		if err != nil {
			if !os.IsNotExist(err) {
				return err
			}
			drift = append(drift, path)
			if path == gate {
				gateChanged = true
			}
			continue
		}
		if err := productEditorMatrixEvidenceHash(path, data, hash); err != nil {
			drift = append(drift, path)
			if path == gate {
				gateChanged = true
			}
		}
	}
	for _, path := range productEditorMatrixEvidenceSources {
		if _, present := manifest.Sources[path]; present {
			continue
		}
		// The independent run receipt already corroborates the recorded OLD set.
		// Only an evolved producer declaration permits a new unrecorded producer.
		// The old package has no Git-history dependency (e.g. squash/shallow CI).
		if !gateChanged {
			return fmt.Errorf("canonical source receipt missing tested producer: %s", path)
		}
		drift = append(drift, path)
	}
	if len(drift) != 0 {
		return drift
	}
	for log, expected := range map[string]int{"playwright.log": 12, "cc12-playwright.log": 2} {
		if err := productEditorMatrixPlaywrightSummary(native[log], productEditorMatrixCurrentRunIdentities(expected)); err != nil {
			return fmt.Errorf("current run identities: %w", err)
		}
	}
	return validateProductEditorMatrixLedger(native["product-editor-click-ledger.json"])
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

// Missing actual producers are drift only after lexical validation. Symlinks
// (including dangling ones) and non-ENOENT I/O failures remain hard errors.
func productEditorMatrixEvidenceSourceRead(root, path string) ([]byte, error) {
	if filepath.IsAbs(path) || filepath.Clean(path) != path || path == "." || path == ".." || strings.HasPrefix(path, "../") {
		return nil, fmt.Errorf("canonical source path must stay repository-relative: %s", path)
	}
	full := root
	for _, part := range strings.Split(path, "/") {
		full = filepath.Join(full, part)
		info, err := os.Lstat(full)
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("canonical source symlink: %s", path)
		}
	}
	return os.ReadFile(full)
}

type productEditorMatrixFrozenRecord struct {
	Tested         string                          `json:"tested_source_commit"`
	Snapshots      map[string]string               `json:"source_snapshots"`
	Hashes         map[string]string               `json:"source_files_sha256"`
	Blobs          map[string]string               `json:"git_blob_ids"`
	Locales        map[string]map[string]string    `json:"locale_labels"`
	Tests          map[string][]string             `json:"test_identities"`
	Cases          []productEditorMatrixLedgerCase `json:"-"`
	MembershipHash string                          `json:"package_membership_sha256"`
}

// New metadata is derived from the tested Git objects and observed native logs;
// raw snapshots let validation work without a deep Git checkout or current labels.
func productEditorMatrixFrozenEvidence(root, evidence, tested, frozenHash string, sources, artifacts map[string]string) (productEditorMatrixFrozenRecord, error) {
	var record productEditorMatrixFrozenRecord
	path := evidence + "/FROZEN-IDENTITIES.json"
	data, err := productEditorMatrixEvidenceRead(root, path)
	if err != nil {
		return record, err
	}
	if err := productEditorMatrixEvidenceHash(path, data, artifacts[path]); err != nil {
		return record, err
	}
	if err := productEditorMatrixEvidenceHash(path, data, frozenHash); err != nil {
		return record, err
	}
	if err := json.Unmarshal(data, &record); err != nil {
		return record, err
	}
	if record.Tested != tested || len(record.Snapshots) == 0 || len(record.Snapshots) != len(record.Hashes) || len(record.Snapshots) != len(record.Blobs) {
		return record, fmt.Errorf("frozen identity record must bind tested source and snapshots")
	}
	if err := productEditorMatrixFrozenMembership(root, evidence, tested, record.MembershipHash, artifacts); err != nil {
		return record, err
	}
	frozen := map[string][]byte{}
	for source, snapshot := range record.Snapshots {
		if !strings.HasPrefix(snapshot, evidence+"/frozen/") {
			return record, fmt.Errorf("frozen snapshot outside package: %s", snapshot)
		}
		data, err := productEditorMatrixEvidenceRead(root, snapshot)
		if err != nil {
			return record, err
		}
		if err := productEditorMatrixEvidenceHash(snapshot, data, artifacts[snapshot]); err != nil {
			return record, err
		}
		if err := productEditorMatrixEvidenceHash(source, data, record.Hashes[source]); err != nil {
			return record, err
		}
		if record.Blobs[source] != productEditorMatrixGitBlob(data) {
			return record, fmt.Errorf("frozen snapshot Git blob mismatch: %s", source)
		}
		if hash, present := sources[source]; present && hash != record.Hashes[source] {
			return record, fmt.Errorf("frozen snapshot disagrees with tested producer: %s", source)
		}
		frozen[source] = data
	}
	const table = "tests/foundation/testdata/product_editor_matrix_cases.json"
	const gate = "tests/foundation/catalog_matrix_ledger_test.go"
	if err := json.Unmarshal(frozen[table], &record.Cases); err != nil || len(record.Cases) == 0 {
		return record, fmt.Errorf("frozen case table missing or malformed")
	}
	producers, err := productEditorMatrixFrozenProducers(frozen[gate])
	if err != nil {
		return record, err
	}
	if len(producers) != len(sources) {
		return record, fmt.Errorf("tested producer set disagrees with frozen declaration")
	}
	for _, producer := range producers {
		if _, present := sources[producer]; !present {
			return record, fmt.Errorf("tested source receipt omitted frozen producer: %s", producer)
		}
	}
	locales, err := productEditorMatrixFrozenLocales(frozen[gate])
	if err != nil {
		return record, err
	}
	if !reflect.DeepEqual(locales, record.Locales) {
		return record, fmt.Errorf("frozen locale labels disagree with tested producer")
	}
	// Validate the untrusted table before using the current declaration builder,
	// whose panic contract is appropriate only for compiled trusted data.
	keys := map[productEditorMatrixLedgerKey]bool{}
	for locale, labels := range record.Locales {
		for _, item := range record.Cases {
			control, claim := item.Control, item.Expected
			for token, label := range labels {
				control = strings.ReplaceAll(control, token, label)
				claim = strings.ReplaceAll(claim, token, label)
			}
			key := productEditorMatrixLedgerKey{Page: item.Page, Locale: locale, Width: 390, Row: item.Row, Control: control, Action: item.Action}
			if strings.TrimSpace(claim) == "" || strings.TrimSpace(control) == "" || strings.TrimSpace(item.Action) == "" || (!strings.HasPrefix(item.Page, "mobile matrix") && !strings.HasPrefix(item.Page, "mobile axis remove")) || keys[key] {
				return record, fmt.Errorf("invalid or duplicate frozen table identity")
			}
			keys[key] = true
		}
	}
	for log, identities := range record.Tests {
		if log != "playwright.log" && log != "cc12-playwright.log" {
			return record, fmt.Errorf("unexpected frozen run: %s", log)
		}
		if len(identities) == 0 {
			return record, fmt.Errorf("empty frozen run identities: %s", log)
		}
		seen := map[string]bool{}
		for _, identity := range identities {
			parts := strings.SplitN(identity, " › ", 2)
			if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" || len(frozen[parts[0]]) == 0 || seen[identity] {
				return record, fmt.Errorf("invalid, duplicate or unbound frozen test identity: %s", identity)
			}
			seen[identity] = true
		}
	}
	if len(record.Tests) != 2 {
		return record, fmt.Errorf("frozen record must bind both full browser runs")
	}
	if err := productEditorMatrixFrozenRunOracle(frozen, record.Tests); err != nil {
		return record, err
	}
	return record, nil
}

// The names-only receipt is independent of ARTIFACTS and includes itself; no
// digest is stored inside itself, so membership cannot create a self-hash cycle.
func productEditorMatrixFrozenMembership(root, evidence, tested, hash string, artifacts map[string]string) error {
	path := evidence + "/FROZEN-MEMBERSHIP.json"
	data, err := productEditorMatrixEvidenceRead(root, path)
	if err != nil {
		return err
	}
	if err := productEditorMatrixEvidenceHash(path, data, artifacts[path]); err != nil {
		return err
	}
	if err := productEditorMatrixEvidenceHash(path, data, hash); err != nil {
		return err
	}
	var membership struct {
		Tested  string   `json:"tested_source_commit"`
		Members []string `json:"package_members"`
	}
	if err := json.Unmarshal(data, &membership); err != nil {
		return err
	}
	if membership.Tested != tested || len(membership.Members) == 0 || len(membership.Members) != len(artifacts) {
		return fmt.Errorf("canonical index must retain the entire frozen package membership")
	}
	seen := map[string]bool{}
	for _, member := range membership.Members {
		if member == "ARTIFACTS.json" || filepath.IsAbs(member) || filepath.ToSlash(filepath.Clean(member)) != member || member == "." || strings.HasPrefix(member, "../") || seen[member] {
			return fmt.Errorf("invalid or duplicate frozen package member: %s", member)
		}
		seen[member] = true
		if _, present := artifacts[evidence+"/"+member]; !present {
			return fmt.Errorf("canonical index omitted frozen package member: %s", member)
		}
	}
	// An independently reviewed original package-name digest prevents coordinated
	// rewrites of membership, identity record, SOURCE, manifest and index together.
	// New package versions need an explicit reviewed oracle, just like consumers.
	members := append([]string{}, membership.Members...)
	sort.Strings(members)
	const originalMembers = "ec82c6dcbd9f3804791239c358cac0e592c6d9d9bee1b2515d35c40478a8fdff"
	if fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(members, "\n")))) != originalMembers {
		return fmt.Errorf("frozen package membership disagrees with original package oracle")
	}
	if !seen["FROZEN-MEMBERSHIP.json"] || !seen["FROZEN-IDENTITIES.json"] {
		return fmt.Errorf("frozen membership must retain its provenance records")
	}
	return nil
}

// Only the declaration at the tested source defines OLD producers. Current
// declarations can evolve without expanding the fourteen historical bindings.
func productEditorMatrixFrozenProducers(data []byte) ([]string, error) {
	file, err := parser.ParseFile(token.NewFileSet(), "frozen.go", data, 0)
	if err != nil {
		return nil, err
	}
	var producers []string
	seen := map[string]bool{}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok || len(value.Names) != 1 || value.Names[0].Name != "productEditorMatrixEvidenceSources" || len(value.Values) != 1 {
				continue
			}
			literal, ok := value.Values[0].(*ast.CompositeLit)
			if !ok || producers != nil {
				return nil, fmt.Errorf("frozen producer declaration must be one literal")
			}
			producers = []string{}
			for _, element := range literal.Elts {
				item, ok := element.(*ast.BasicLit)
				if !ok || item.Kind != token.STRING {
					return nil, fmt.Errorf("frozen producer must be a literal path")
				}
				path, err := strconv.Unquote(item.Value)
				if err != nil || path == "" || filepath.IsAbs(path) || filepath.ToSlash(filepath.Clean(path)) != path || strings.HasPrefix(path, "../") || seen[path] {
					return nil, fmt.Errorf("invalid or duplicate frozen producer")
				}
				seen[path] = true
				producers = append(producers, path)
			}
		}
	}
	if len(producers) == 0 {
		return nil, fmt.Errorf("missing frozen producer declaration")
	}
	return producers, nil
}

// Independent original identity oracle for the four exact consumer Git blobs at
// 2e10084b3f5e96548c15953cc40488aef9e1c86d. Titles/domains were read from those
// declarations and corroborated by the unchanged native logs. This deliberately
// fails closed for a different consumer version, which needs a reviewed oracle;
// it never calls the current-title function or generates new browser evidence.
func productEditorMatrixFrozenRunOracle(frozen map[string][]byte, tests map[string][]string) error {
	consumerHashes := map[string]string{
		"tests/admin/product-editor.acceptance.ts":   "42c62c527424bc0d611c29f15ca9570330ee8a7fb2f4aeb2a5f9cbd3c8f44e69",
		"tests/admin/product-review.acceptance.ts":   "e9fb97fb23762ffede5ee3b755e88c88263bb5d458075b05b214e44766f077a4",
		"tests/admin/product-feedback.acceptance.ts": "ac098dafcd7f782901c87523878a37768cf6530a024dd1bf75999a908219a173",
		"tests/admin/catalog-core.spec.ts":           "18db7e6e25ddfc05bdd6b9fecc6f80b3bd47e92f1e31addddf6ff318c21da0d8",
	}
	for path, hash := range consumerHashes {
		if err := productEditorMatrixEvidenceHash(path, frozen[path], hash); err != nil {
			return fmt.Errorf("unsupported frozen consumer oracle: %w", err)
		}
	}
	want := map[string][]string{
		"playwright.log": {
			"tests/admin/product-editor.acceptance.ts › PE12-17 document workflow, matrix, list actions and click ledger",
			"tests/admin/product-review.acceptance.ts › review: bulk price 80 plus archive sends exclusive archive entry and persists",
			"tests/admin/product-review.acceptance.ts › review: create UNKNOWN then client preserves receipt fence after reload",
			"tests/admin/product-review.acceptance.ts › review: create UNKNOWN then 401 preserves receipt fence after reload",
			"tests/admin/product-review.acceptance.ts › review: create UNKNOWN then 403 preserves receipt fence after reload",
			"tests/admin/product-review.acceptance.ts › review: copy UNKNOWN then client preserves receipt fence after reload",
			"tests/admin/product-review.acceptance.ts › review: copy UNKNOWN then 401 preserves receipt fence after reload",
			"tests/admin/product-review.acceptance.ts › review: copy UNKNOWN then 403 preserves receipt fence after reload",
			"tests/admin/product-review.acceptance.ts › review: order UNKNOWN then 403 preserves receipt fence after reload",
			"tests/admin/product-review.acceptance.ts › review: publish UNKNOWN then 403 preserves receipt fence after reload",
			"tests/admin/product-feedback.acceptance.ts › feedback: identical image-required validation reveals again at 1586px",
			"tests/admin/product-feedback.acceptance.ts › feedback: identical image-required validation reveals again at 390px",
		},
		"cc12-playwright.log": {
			"tests/admin/catalog-core.spec.ts › CC12 desktop en › catalog v2: draft product, 2x3 matrix, prices, compare-at, stock, photo, collection, activate, shopper view, archive a variant, deactivate (desktop, en)",
			"tests/admin/catalog-core.spec.ts › CC12 mobile zh-TW › catalog v2: draft product, 2x3 matrix, prices, compare-at, stock, photo, collection, activate, shopper view, archive a variant, deactivate (mobile, zh-TW)",
		},
	}
	for log, identities := range want {
		actual := append([]string{}, tests[log]...)
		sort.Strings(identities)
		sort.Strings(actual)
		if !reflect.DeepEqual(identities, actual) {
			return fmt.Errorf("frozen run titles disagree with original consumer declarations: %s", log)
		}
	}
	return nil
}

func productEditorMatrixFrozenLocales(data []byte) (map[string]map[string]string, error) {
	file, err := parser.ParseFile(token.NewFileSet(), "frozen.go", data, 0)
	if err != nil {
		return nil, err
	}
	locales := map[string]map[string]string{}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok || len(value.Names) != 1 || value.Names[0].Name != "productEditorMatrixLedgerLocales" || len(value.Values) != 1 {
				continue
			}
			outer, ok := value.Values[0].(*ast.CompositeLit)
			if !ok {
				return nil, fmt.Errorf("frozen locale bindings must be literal")
			}
			for _, item := range outer.Elts {
				pair, ok := item.(*ast.KeyValueExpr)
				if !ok {
					return nil, fmt.Errorf("frozen locale entry must be literal")
				}
				key, ok := pair.Key.(*ast.BasicLit)
				if !ok {
					return nil, fmt.Errorf("frozen locale key must be literal")
				}
				locale, err := strconv.Unquote(key.Value)
				if err != nil || strings.TrimSpace(locale) == "" || locales[locale] != nil {
					return nil, fmt.Errorf("invalid frozen locale key")
				}
				labels, ok := pair.Value.(*ast.CompositeLit)
				if !ok {
					return nil, fmt.Errorf("frozen labels must be literal")
				}
				locales[locale] = map[string]string{}
				for _, item := range labels.Elts {
					pair, ok := item.(*ast.KeyValueExpr)
					if !ok {
						return nil, fmt.Errorf("frozen label must be literal")
					}
					key, keyOK := pair.Key.(*ast.BasicLit)
					value, valueOK := pair.Value.(*ast.BasicLit)
					if !keyOK || !valueOK {
						return nil, fmt.Errorf("frozen label operands must be literal")
					}
					name, e1 := strconv.Unquote(key.Value)
					label, e2 := strconv.Unquote(value.Value)
					if e1 != nil || e2 != nil || name == "" || label == "" || locales[locale][name] != "" {
						return nil, fmt.Errorf("invalid frozen label")
					}
					locales[locale][name] = label
				}
			}
		}
	}
	if len(locales) == 0 {
		return nil, fmt.Errorf("missing frozen locale bindings")
	}
	return locales, nil
}

// Git object identity is provenance; SHA-256 above remains the integrity digest.
func productEditorMatrixGitBlob(data []byte) string {
	object := append([]byte(fmt.Sprintf("blob %d\x00", len(data))), data...)
	return fmt.Sprintf("%x", sha1.Sum(object))
}
