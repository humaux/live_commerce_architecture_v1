// Purpose: Exercise canonical matrix evidence with isolated byte, source and run-outcome corruptions.
// Depends on: the real evidence validator and native canonical receipts; no DB, browser or network calls.
// Used by: foundation shards; fixture mutations never modify delivered evidence or source files.
package foundation_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

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
	// Isolated corruption fixtures rebind current executable bytes; delivered
	// canonical evidence still must come from an actual run on frozen sources.
	for _, path := range []string{"tests/foundation/catalog_matrix_evidence_test.go", "tests/ci/product-editor-matrix-execution.mjs", "tests/ci/product-editor-matrix-execution.test.mjs"} {
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

// Hash-consistent bytes alone are not a passing browser run. Both native final
// receipts, list-reporter summaries and manifest counts must corroborate it.
func validateProductEditorMatrixRunOutcome(manifestData []byte, native map[string][]byte) error {
	var manifest struct {
		Passed map[string]int `json:"actual_passed_cases"`
		Ledger struct {
			Raw       int            `json:"raw_rows"`
			Required  int            `json:"required_unique_PASS"`
			PerLocale map[string]int `json:"per_locale"`
			Width     int            `json:"width"`
		} `json:"ledger"`
	}
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return err
	}
	for _, run := range []struct {
		name, field, log string
		expected         int
	}{
		{"pe-last-run.json", "PE_and_review", "playwright.log", 12},
		{"cc12-last-run.json", "CC12_frozen", "cc12-playwright.log", 2},
	} {
		var final struct {
			Status string             `json:"status"`
			Failed *[]json.RawMessage `json:"failedTests"`
		}
		if err := json.Unmarshal(native[run.name], &final); err != nil {
			return fmt.Errorf("canonical run outcome %s: %w", run.name, err)
		}
		if final.Status != "passed" || final.Failed == nil || len(*final.Failed) != 0 {
			return fmt.Errorf("canonical run outcome %s must be passed with explicit empty failedTests", run.name)
		}
		if manifest.Passed[run.field] != run.expected {
			return fmt.Errorf("canonical manifest passed count %s must be %d", run.field, run.expected)
		}
		if err := productEditorMatrixPlaywrightSummary(native[run.log], run.expected); err != nil {
			return fmt.Errorf("canonical run outcome %s: %w", run.log, err)
		}
	}
	if skipped, present := manifest.Passed["skipped"]; !present || skipped != 0 {
		return fmt.Errorf("canonical manifest must record zero skipped cases")
	}
	var rows []productEditorMatrixLedgerRow
	if err := json.Unmarshal(native["product-editor-click-ledger.json"], &rows); err != nil {
		return err
	}
	perLocale := map[string]int{}
	for _, row := range rows {
		if strings.HasPrefix(row.Page, "mobile matrix") || strings.HasPrefix(row.Page, "mobile axis remove") {
			perLocale[row.Locale]++
		}
	}
	if manifest.Ledger.Raw != len(rows) || manifest.Ledger.Required != len(productEditorMatrixLedgerExpected()) ||
		manifest.Ledger.Width != 390 || !reflect.DeepEqual(manifest.Ledger.PerLocale, perLocale) {
		return fmt.Errorf("canonical manifest ledger counts must match the exact validated PASS ledger")
	}
	return nil
}

func productEditorMatrixPlaywrightSummary(data []byte, expected int) error {
	text := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(string(data), "")
	started := regexp.MustCompile(`(?m)^Running (\d+) tests? using \d+ workers?`).FindAllStringSubmatch(text, -1)
	if len(started) != 1 || started[0][1] != strconv.Itoa(expected) {
		return fmt.Errorf("must start exactly %d cases", expected)
	}
	passed := 0
	for _, summary := range regexp.MustCompile(`(?m)^\s*(\d+) (passed|failed|skipped|flaky|interrupted|did not run)\b`).FindAllStringSubmatch(text, -1) {
		count, err := strconv.Atoi(summary[1])
		if err != nil {
			return err
		}
		if summary[2] == "passed" {
			passed++
			if count != expected {
				return fmt.Errorf("must finish exactly %d passed cases, got %d", expected, count)
			}
		} else if count != 0 {
			return fmt.Errorf("unexpected %s cases: %d", summary[2], count)
		}
	}
	if passed != 1 {
		return fmt.Errorf("must contain one completed passing summary")
	}
	return nil
}

// Every corruption below is rehashed through BOTH native receipts and the
// external manifest index: only run semantics, never a stale hash, may reject it.
func TestProductEditorMatrixLedgerEvidenceRunOutcome(t *testing.T) {
	for _, mutation := range []string{"pe-failed", "cc12-failed", "failed-tests", "missing-failed-tests", "null-failed-tests", "log-failed", "cc12-log-failed", "log-skipped", "log-flaky", "log-incomplete", "log-extra-failed", "log-extra-skipped", "log-extra-flaky", "missing-summary", "manifest-passed", "manifest-skipped", "manifest-ledger"} {
		t.Run(mutation, func(t *testing.T) {
			root, canonical := productEditorMatrixEvidenceFixture(t)
			if err := validateProductEditorMatrixEvidence(root); err != nil {
				t.Fatalf("untampered evidence: %v", err)
			}
			path := canonical + "/pe-last-run.json"
			data := []byte(`{"status":"failed","failedTests":["MOCK-final-failure"]}`)
			switch mutation {
			case "cc12-failed":
				path = canonical + "/cc12-last-run.json"
			case "failed-tests":
				data = []byte(`{"status":"passed","failedTests":["MOCK-final-failure"]}`)
			case "missing-failed-tests":
				data = []byte(`{"status":"passed"}`)
			case "null-failed-tests":
				data = []byte(`{"status":"passed","failedTests":null}`)
			case "log-failed", "log-skipped", "log-flaky", "log-incomplete", "cc12-log-failed", "log-extra-failed", "log-extra-skipped", "log-extra-flaky", "missing-summary":
				path = canonical + "/playwright.log"
				old, replacement := "12 passed", "11 passed\n  1 failed"
				if mutation == "cc12-log-failed" {
					path = canonical + "/cc12-playwright.log"
					old, replacement = "2 passed", "1 passed\n  1 failed"
				}
				if mutation == "log-skipped" {
					replacement = "11 passed\n  1 skipped"
				}
				if mutation == "log-flaky" {
					replacement = "11 passed\n  1 flaky"
				}
				if mutation == "log-incomplete" {
					replacement = "11 passed"
				}
				if strings.HasPrefix(mutation, "log-extra-") {
					replacement = "12 passed\n  1 " + strings.TrimPrefix(mutation, "log-extra-")
				}
				if mutation == "missing-summary" {
					replacement = "run did not finish"
				}
				text := string(productEditorMatrixReadFixture(t, root, path))
				if !strings.Contains(text, old) {
					t.Fatal("native summary mutation anchor missing")
				}
				data = []byte(strings.Replace(text, old, replacement, 1))
			case "manifest-passed", "manifest-skipped", "manifest-ledger":
				path = canonical + "/browser-evidence.json"
				var manifest map[string]any
				if err := json.Unmarshal(productEditorMatrixReadFixture(t, root, path), &manifest); err != nil {
					t.Fatal(err)
				}
				if mutation == "manifest-passed" {
					manifest["actual_passed_cases"].(map[string]any)["PE_and_review"] = 99
				}
				if mutation == "manifest-skipped" {
					manifest["actual_passed_cases"].(map[string]any)["skipped"] = 1
				}
				if mutation == "manifest-ledger" {
					manifest["ledger"].(map[string]any)["required_unique_PASS"] = 230
				}
				data, _ = json.Marshal(manifest)
			}
			productEditorMatrixRehashFixture(t, root, canonical, path, data)
			if err := validateProductEditorMatrixEvidence(root); err == nil {
				t.Fatalf("consistently rehashed %s must reject the run outcome", mutation)
			} else {
				if strings.Contains(err.Error(), "SHA-256") {
					t.Fatalf("must reject semantics, not hashes: %v", err)
				}
				t.Logf("rejected rehashed %s: %v", mutation, err)
			}
		})
	}
}

func productEditorMatrixRehashFixture(t *testing.T, root, canonical, path string, data []byte) {
	t.Helper()
	write := func(path string, data []byte) {
		if err := os.WriteFile(filepath.Join(root, path), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	hash := func(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }
	manifestPath := canonical + "/browser-evidence.json"
	var manifest, index map[string]any
	if err := json.Unmarshal(productEditorMatrixReadFixture(t, root, manifestPath), &manifest); err != nil {
		t.Fatal(err)
	}
	indexPath := filepath.Dir(canonical) + "/ARTIFACTS.json"
	if err := json.Unmarshal(productEditorMatrixReadFixture(t, root, indexPath), &index); err != nil {
		t.Fatal(err)
	}
	write(path, data)
	index["artifacts_sha256"].(map[string]any)[path] = hash(data)
	if path != manifestPath {
		for _, entry := range manifest["copied_native_bytes"].([]any) {
			copy := entry.(map[string]any)
			if copy["canonical"] == path {
				copy["sha256"] = hash(data)
			}
		}
		data, _ = json.Marshal(manifest)
		write(manifestPath, data)
		index["artifacts_sha256"].(map[string]any)[manifestPath] = hash(data)
	}
	data, _ = json.Marshal(index)
	write(indexPath, data)
}
