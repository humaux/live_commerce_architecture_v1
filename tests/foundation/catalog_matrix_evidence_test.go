// Purpose: Exercise canonical matrix evidence with isolated byte, source and run-outcome corruptions.
// Depends on: the real evidence validator and canonical receipts; isolated current-table fixtures are MODEL_ONLY.
// Used by: foundation shards; fixture mutations never modify delivered evidence or source files.
package foundation_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Exercise the actual canonical test entry point, including its SKIP/FAIL policy,
// in an isolated checkout. Validator-only calls cannot prove testing.T.Skip.
func TestProductEditorMatrixLedgerEvidenceHistoricalPolicy(t *testing.T) {
	for _, tc := range []struct {
		name, strict, mutation string
		wantExit               int
		want                   string
	}{
		{"matching", "1", "", 0, "--- PASS: TestProductEditorMatrixLedgerEvidence"},
		{"drift-default", "", "source", 0, "--- SKIP: TestProductEditorMatrixLedgerEvidence"},
		{"drift-strict", "1", "source", 1, "--- FAIL: TestProductEditorMatrixLedgerEvidence"},
		{"removed-producer-default", "", "removed-producer", 0, "--- SKIP: TestProductEditorMatrixLedgerEvidence"},
		{"removed-producer-strict", "1", "removed-producer", 1, "--- FAIL: TestProductEditorMatrixLedgerEvidence"},
		{"renamed-producer-default", "", "renamed-producer", 0, "--- SKIP: TestProductEditorMatrixLedgerEvidence"},
		{"renamed-producer-strict", "1", "renamed-producer", 1, "--- FAIL: TestProductEditorMatrixLedgerEvidence"},
		{"tampered-matching", "", "artifact", 1, "--- FAIL: TestProductEditorMatrixLedgerEvidence"},
		{"source-directory-default", "", "source-directory", 1, "--- FAIL: TestProductEditorMatrixLedgerEvidence"},
		{"source-directory-strict", "1", "source-directory", 1, "--- FAIL: TestProductEditorMatrixLedgerEvidence"},
		{"dangling-source-default", "", "dangling-source", 1, "--- FAIL: TestProductEditorMatrixLedgerEvidence"},
		{"dangling-source-strict", "1", "dangling-source", 1, "--- FAIL: TestProductEditorMatrixLedgerEvidence"},
		{"unindexed-outer-matching-default", "", "unindexed-outer-matching", 1, "--- FAIL: TestProductEditorMatrixLedgerEvidence"},
		{"unindexed-outer-matching-strict", "1", "unindexed-outer-matching", 1, "--- FAIL: TestProductEditorMatrixLedgerEvidence"},
		{"unindexed-outer-drift-default", "", "unindexed-outer-drift", 1, "--- FAIL: TestProductEditorMatrixLedgerEvidence"},
		{"unindexed-outer-drift-strict", "1", "unindexed-outer-drift", 1, "--- FAIL: TestProductEditorMatrixLedgerEvidence"},
		{"unindexed-nested-matching-default", "", "unindexed-nested-matching", 1, "--- FAIL: TestProductEditorMatrixLedgerEvidence"},
		{"unindexed-nested-matching-strict", "1", "unindexed-nested-matching", 1, "--- FAIL: TestProductEditorMatrixLedgerEvidence"},
		{"unindexed-nested-drift-default", "", "unindexed-nested-drift", 1, "--- FAIL: TestProductEditorMatrixLedgerEvidence"},
		{"unindexed-nested-drift-strict", "1", "unindexed-nested-drift", 1, "--- FAIL: TestProductEditorMatrixLedgerEvidence"},
		{"deleted-drift-default", "", "deleted", 1, "--- FAIL: TestProductEditorMatrixLedgerEvidence"},
		{"deleted-drift-strict", "1", "deleted", 1, "--- FAIL: TestProductEditorMatrixLedgerEvidence"},
		{"copy-drift-default", "", "copy", 1, "--- FAIL: TestProductEditorMatrixLedgerEvidence"},
		{"copy-drift-strict", "1", "copy", 1, "--- FAIL: TestProductEditorMatrixLedgerEvidence"},
		{"index-drift-default", "", "index", 1, "--- FAIL: TestProductEditorMatrixLedgerEvidence"},
		{"index-drift-strict", "1", "index", 1, "--- FAIL: TestProductEditorMatrixLedgerEvidence"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, canonical := productEditorMatrixEvidenceFixture(t)
			path := "tests/admin/product-editor.acceptance.ts"
			if tc.mutation == "artifact" {
				path = canonical + "/playwright.log"
			}
			if tc.mutation != "" {
				data := append(productEditorMatrixReadFixture(t, root, path), []byte("\n// isolated mutation\n")...)
				if err := os.WriteFile(filepath.Join(root, path), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if strings.HasPrefix(tc.mutation, "unindexed-") {
				extra := filepath.Dir(canonical) + "/unindexed.txt"
				if strings.Contains(tc.mutation, "nested") {
					extra = filepath.Dir(canonical) + "/nested/unindexed.txt"
				}
				if err := os.MkdirAll(filepath.Dir(filepath.Join(root, extra)), 0700); err != nil {
					t.Fatal(err)
				}
				productEditorMatrixWriteFixture(t, root, extra, []byte("unindexed evidence"))
				if strings.HasSuffix(tc.mutation, "-matching") {
					productEditorMatrixWriteFixture(t, root, path, productEditorMatrixReadFixture(t, "../..", path))
				}
			}
			switch tc.mutation {
			case "source-directory", "dangling-source":
				if err := os.Remove(filepath.Join(root, path)); err != nil {
					t.Fatal(err)
				}
				if tc.mutation == "source-directory" {
					if err := os.Mkdir(filepath.Join(root, path), 0700); err != nil {
						t.Fatal(err)
					}
				} else if err := os.Symlink("missing-producer.ts", filepath.Join(root, path)); err != nil {
					t.Fatal(err)
				}
			case "removed-producer":
				if err := os.Remove(filepath.Join(root, path)); err != nil {
					t.Fatal(err)
				}
			case "renamed-producer":
				if err := os.Rename(filepath.Join(root, path), filepath.Join(root, path+".renamed")); err != nil {
					t.Fatal(err)
				}
			case "deleted":
				if err := os.Remove(filepath.Join(root, canonical, "playwright.log")); err != nil {
					t.Fatal(err)
				}
			case "copy":
				manifestPath := canonical + "/browser-evidence.json"
				manifest := productEditorMatrixJSONFixture(t, root, manifestPath)
				manifest["copied_native_bytes"].([]any)[0].(map[string]any)["sha256"] = strings.Repeat("0", 64)
				data, _ := json.Marshal(manifest)
				productEditorMatrixRehashFixture(t, root, canonical, manifestPath, data)
			case "index":
				indexPath := filepath.Dir(canonical) + "/ARTIFACTS.json"
				index := productEditorMatrixJSONFixture(t, root, indexPath)
				index["artifacts_sha256"].(map[string]any)[filepath.Dir(canonical)+"/test-node.json"] = strings.Repeat("0", 64)
				data, _ := json.Marshal(index)
				productEditorMatrixWriteFixture(t, root, indexPath, data)
			}
			binary, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, binary, "-test.run=^TestProductEditorMatrixLedgerEvidence$", "-test.v")
			cmd.Dir = filepath.Join(root, "tests/foundation")
			t.Setenv("LC_MATRIX_EVIDENCE_STRICT", tc.strict)
			output, err := cmd.CombinedOutput()
			code := 0
			if err != nil {
				if exit, ok := err.(*exec.ExitError); ok {
					code = exit.ExitCode()
				} else {
					t.Fatal(err)
				}
			}
			t.Logf("actual canonical child exit=%d:\n%s", code, output)
			if code != tc.wantExit || !strings.Contains(string(output), tc.want) {
				t.Fatalf("want exit %d and %q", tc.wantExit, tc.want)
			}
			if tc.mutation == "source-directory" || tc.mutation == "dangling-source" || strings.HasPrefix(tc.mutation, "unindexed-") || tc.mutation == "deleted" || tc.mutation == "copy" || tc.mutation == "index" {
				if strings.Contains(string(output), "canonical evidence is historical") || strings.Contains(string(output), "--- SKIP:") {
					t.Fatal("corrupted historical package must fail for immutable integrity in both policies")
				}
			}
			if tc.mutation == "source" && !strings.Contains(string(output), "canonical evidence is historical (source drift: "+path+")") {
				t.Fatal("source drift must explicitly identify the historical record and changed file")
			}
			if tc.mutation == "artifact" && (!strings.Contains(string(output), "SHA-256 mismatch") || strings.Contains(string(output), "--- SKIP:")) {
				t.Fatal("matching-source byte tampering must fail, never skip")
			}
		})
	}
}

// Drift may relax comparison with evolved producers, never the immutable package.
func TestProductEditorMatrixLedgerEvidenceDriftIntegrity(t *testing.T) {
	for _, mutation := range []string{"deleted-native", "omitted-native", "copy-hash", "unindexed-copy", "extra-index-bytes", "extra-index-missing", "extra-index-path", "failed-outcome", "duplicate-ledger", "failed-ledger", "missing-claim", "manifest-ledger", "malformed-source", "unindexed-outer", "unindexed-nested", "blank-locale", "missing-locale", "altered-claim"} {
		t.Run(mutation, func(t *testing.T) {
			root, canonical := productEditorMatrixEvidenceFixture(t)
			source := "tests/admin/product-editor.acceptance.ts"
			productEditorMatrixWriteFixture(t, root, source, append(productEditorMatrixReadFixture(t, root, source), '\n'))
			manifestPath, indexPath := canonical+"/browser-evidence.json", filepath.Dir(canonical)+"/ARTIFACTS.json"
			manifest, index := productEditorMatrixJSONFixture(t, root, manifestPath), productEditorMatrixJSONFixture(t, root, indexPath)
			path := canonical + "/playwright.log"
			switch mutation {
			case "unindexed-outer", "unindexed-nested":
				path = filepath.Dir(canonical) + "/unindexed.txt"
				if mutation == "unindexed-nested" {
					path = filepath.Dir(canonical) + "/nested/unindexed.txt"
				}
				if err := os.MkdirAll(filepath.Dir(filepath.Join(root, path)), 0700); err != nil {
					t.Fatal(err)
				}
				productEditorMatrixWriteFixture(t, root, path, []byte("unindexed evidence"))
			case "blank-locale", "missing-locale", "altered-claim":
				path = canonical + "/product-editor-click-ledger.json"
				var rows []productEditorMatrixLedgerRow
				if err := json.Unmarshal(productEditorMatrixReadFixture(t, root, path), &rows); err != nil {
					t.Fatal(err)
				}
				if mutation == "blank-locale" {
					rows[0].Locale = ""
				}
				if mutation == "altered-claim" {
					rows[0].Expected = json.RawMessage(`"unrelated plausible historical claim"`)
				}
				if mutation == "missing-locale" {
					kept := []productEditorMatrixLedgerRow{}
					for _, row := range rows {
						if row.Locale != "en" {
							kept = append(kept, row)
						}
					}
					rows = kept
				}
				data, _ := json.Marshal(rows)
				productEditorMatrixRehashFixture(t, root, canonical, path, data)
			case "deleted-native":
				if err := os.Remove(filepath.Join(root, path)); err != nil {
					t.Fatal(err)
				}
			case "omitted-native":
				if err := os.Remove(filepath.Join(root, path)); err != nil {
					t.Fatal(err)
				}
				delete(index["artifacts_sha256"].(map[string]any), path)
				copies := []any{}
				for _, raw := range manifest["copied_native_bytes"].([]any) {
					if raw.(map[string]any)["canonical"] != path {
						copies = append(copies, raw)
					}
				}
				manifest["copied_native_bytes"] = copies

			case "copy-hash":
				manifest["copied_native_bytes"].([]any)[0].(map[string]any)["sha256"] = strings.Repeat("0", 64)
			case "unindexed-copy":
				delete(index["artifacts_sha256"].(map[string]any), path)
			case "extra-index-bytes", "extra-index-missing", "extra-index-path":
				path = filepath.Dir(canonical) + "/test-node.json"
				data := []byte(`{"exit_code":0}`)
				productEditorMatrixWriteFixture(t, root, path, data)
				index["artifacts_sha256"].(map[string]any)[path] = fmt.Sprintf("%x", sha256.Sum256(data))
				if mutation == "extra-index-bytes" {
					productEditorMatrixWriteFixture(t, root, path, []byte(`{"exit_code":1}`))
				}
				if mutation == "extra-index-missing" {
					if err := os.Remove(filepath.Join(root, path)); err != nil {
						t.Fatal(err)
					}
				}
				if mutation == "extra-index-path" {
					index["artifacts_sha256"].(map[string]any)[filepath.Dir(canonical)+"/../test-node.json"] = fmt.Sprintf("%x", sha256.Sum256(data))
				}
			case "failed-outcome":
				productEditorMatrixRehashFixture(t, root, canonical, canonical+"/pe-last-run.json", []byte(`{"status":"failed","failedTests":[]}`))
			case "duplicate-ledger", "failed-ledger", "missing-claim":
				path = canonical + "/product-editor-click-ledger.json"
				var rows []productEditorMatrixLedgerRow
				if err := json.Unmarshal(productEditorMatrixReadFixture(t, root, path), &rows); err != nil {
					t.Fatal(err)
				}
				for i, row := range rows {
					if strings.HasPrefix(row.Page, "mobile matrix") {
						if mutation == "duplicate-ledger" {
							rows = append(rows, row)
						}
						if mutation == "failed-ledger" {
							rows[i].Actual = "FAIL"
						}
						if mutation == "missing-claim" {
							rows[i].Expected = nil
						}
						break
					}
				}
				data, _ := json.Marshal(rows)
				productEditorMatrixRehashFixture(t, root, canonical, path, data)
			case "manifest-ledger":
				manifest["ledger"].(map[string]any)["required_unique_PASS"] = 1
			case "malformed-source":
				manifest["source_files_sha256"].(map[string]any)[source] = "malformed"
				index["source_files_sha256"].(map[string]any)[source] = "malformed"
				sourcePath := filepath.Dir(canonical) + "/SOURCE.json"
				receipt := productEditorMatrixJSONFixture(t, root, sourcePath)
				receipt["source_files_sha256"].(map[string]any)[source] = "malformed"
				data, _ := json.Marshal(receipt)
				productEditorMatrixWriteFixture(t, root, sourcePath, data)
				index["artifacts_sha256"].(map[string]any)[sourcePath] = fmt.Sprintf("%x", sha256.Sum256(data))
			}
			// Preserve deliberate semantic rehashes; only explicitly edited receipts are rewritten.
			if mutation != "blank-locale" && mutation != "missing-locale" && mutation != "altered-claim" && mutation != "failed-outcome" && mutation != "duplicate-ledger" && mutation != "failed-ledger" && mutation != "missing-claim" {
				data, _ := json.Marshal(manifest)
				productEditorMatrixWriteFixture(t, root, manifestPath, data)
				index["artifacts_sha256"].(map[string]any)[manifestPath] = fmt.Sprintf("%x", sha256.Sum256(data))
				data, _ = json.Marshal(index)
				productEditorMatrixWriteFixture(t, root, indexPath, data)
			}
			err := validateProductEditorMatrixEvidence(root)
			var drift productEditorMatrixEvidenceDrift
			if err == nil || errors.As(err, &drift) {
				t.Fatalf("immutable %s corruption must fail before drift: %v", mutation, err)
			}
			t.Logf("immutable %s rejected: %v", mutation, err)
		})
	}
}

func TestProductEditorMatrixLedgerEvidenceProducerEvolution(t *testing.T) {
	for _, gateDrift := range []bool{false, true} {
		t.Run(fmt.Sprintf("gate-drift-%t", gateDrift), func(t *testing.T) {
			root, _ := productEditorMatrixEvidenceFixture(t)
			gate := "tests/foundation/catalog_matrix_ledger_test.go"
			if gateDrift {
				productEditorMatrixWriteFixture(t, root, gate, append(productEditorMatrixReadFixture(t, root, gate), '\n'))
			}
			original := productEditorMatrixEvidenceSources
			productEditorMatrixEvidenceSources = append(append([]string{}, original...), "tests/fixture/new-producer.ts")
			defer func() { productEditorMatrixEvidenceSources = original }()
			err := validateProductEditorMatrixEvidence(root)
			var drift productEditorMatrixEvidenceDrift
			if gateDrift {
				if !errors.As(err, &drift) || !strings.Contains(err.Error(), "tests/fixture/new-producer.ts") {
					t.Fatalf("new producer on changed declaration must be historical drift: %v", err)
				}
			} else if err == nil || errors.As(err, &drift) {
				t.Fatalf("unchanged declaration missing producer must be malformed: %v", err)
			}
			t.Logf("producer evolution: %v", err)
		})
	}
}

// A changed declaration alone cannot excuse an omitted OLD producer. The
// separately indexed frozen run receipt must corroborate the recorded source set.
func TestProductEditorMatrixLedgerEvidenceRunReceipt(t *testing.T) {
	for _, mutation := range []string{"old-producer-omitted", "old-producer-omitted-everywhere", "missing", "unindexed", "tested-commit", "after-commit", "before-sources", "after-sources", "command", "exit", "missing-exit"} {
		t.Run(mutation, func(t *testing.T) {
			root, canonical := productEditorMatrixEvidenceFixture(t)
			gate := "tests/foundation/catalog_matrix_ledger_test.go"
			productEditorMatrixWriteFixture(t, root, gate, append(productEditorMatrixReadFixture(t, root, gate), []byte("\n// unrelated comment\n")...))
			evidence := filepath.Dir(canonical)
			path := evidence + "/browser-product-editor.json"
			indexPath := evidence + "/ARTIFACTS.json"
			index := productEditorMatrixJSONFixture(t, root, indexPath)
			receipt := productEditorMatrixJSONFixture(t, root, path)
			switch mutation {
			case "old-producer-omitted", "old-producer-omitted-everywhere":
				producer := "tests/admin/product-editor.acceptance.ts"
				for _, boundPath := range []string{canonical + "/browser-evidence.json", evidence + "/SOURCE.json"} {
					bound := productEditorMatrixJSONFixture(t, root, boundPath)
					delete(bound["source_files_sha256"].(map[string]any), producer)
					data, _ := json.Marshal(bound)
					productEditorMatrixWriteFixture(t, root, boundPath, data)
					index["artifacts_sha256"].(map[string]any)[boundPath] = fmt.Sprintf("%x", sha256.Sum256(data))
				}
				delete(index["source_files_sha256"].(map[string]any), producer)
				if mutation == "old-producer-omitted-everywhere" {
					delete(receipt["source_files_sha256"].(map[string]any), producer)
					delete(receipt["source_files_sha256_after"].(map[string]any), producer)
				}
			case "missing":
				if err := os.Remove(filepath.Join(root, path)); err != nil {
					t.Fatal(err)
				}
				delete(index["artifacts_sha256"].(map[string]any), path)
			case "unindexed":
				delete(index["artifacts_sha256"].(map[string]any), path)
			case "tested-commit":
				receipt["tested_source_commit"] = strings.Repeat("0", 40)
			case "after-commit":
				receipt["source_commit_after"] = strings.Repeat("0", 40)
			case "before-sources", "after-sources":
				field := "source_files_sha256"
				if mutation == "after-sources" {
					field += "_after"
				}
				delete(receipt[field].(map[string]any), "tests/admin/product-editor.acceptance.ts")
			case "command":
				receipt["command"] = []string{"bash", "scripts/dev/test-local.sh", "--browser-product-editor", "--grep", "PE12"}
			case "exit":
				receipt["exit_code"] = 1
			case "missing-exit":
				delete(receipt, "exit_code")
			}
			if mutation != "old-producer-omitted" && mutation != "missing" && mutation != "unindexed" {
				data, _ := json.Marshal(receipt)
				productEditorMatrixWriteFixture(t, root, path, data)
				index["artifacts_sha256"].(map[string]any)[path] = fmt.Sprintf("%x", sha256.Sum256(data))
			}
			data, _ := json.Marshal(index)
			productEditorMatrixWriteFixture(t, root, indexPath, data)
			err := validateProductEditorMatrixEvidence(root)
			var drift productEditorMatrixEvidenceDrift
			if err == nil || errors.As(err, &drift) {
				t.Fatalf("frozen run receipt %s must fail before drift: %v", mutation, err)
			}
			t.Logf("run receipt %s rejected: %v", mutation, err)
		})
	}
}

func TestProductEditorMatrixLedgerEvidenceTableEvolution(t *testing.T) {
	for _, change := range []string{"rename-title", "add-case", "remove-case"} {
		t.Run(change, func(t *testing.T) {
			root, _ := productEditorMatrixEvidenceFixture(t)
			gate := "tests/foundation/catalog_matrix_ledger_test.go"
			productEditorMatrixWriteFixture(t, root, gate, append(productEditorMatrixReadFixture(t, root, gate), '\n'))
			original := productEditorMatrixCurrentRunIdentities
			productEditorMatrixCurrentRunIdentities = func(count int) []string {
				identities := original(count)
				switch change {
				case "rename-title":
					identities[0] += " future title"
				case "add-case":
					identities = append(identities, "tests/admin/product-review.acceptance.ts › future case")
				case "remove-case":
					identities = identities[1:]
				}
				return identities
			}
			defer func() { productEditorMatrixCurrentRunIdentities = original }()
			var drift productEditorMatrixEvidenceDrift
			if err := validateProductEditorMatrixEvidence(root); !errors.As(err, &drift) {
				t.Fatalf("frozen title evolution must be typed drift: %v", err)
			}
		})
	}
	for _, change := range []string{"add-locale", "remove-locale", "rename-label"} {
		t.Run(change, func(t *testing.T) {
			root, _ := productEditorMatrixEvidenceFixture(t)
			gate := "tests/foundation/catalog_matrix_ledger_test.go"
			productEditorMatrixWriteFixture(t, root, gate, append(productEditorMatrixReadFixture(t, root, gate), '\n'))
			original := productEditorMatrixLedgerLocales
			locales := map[string]map[string]string{}
			for locale, labels := range original {
				locales[locale] = labels
			}
			switch change {
			case "add-locale":
				locales["ja"] = original["en"]
			case "remove-locale":
				delete(locales, "en")
			case "rename-label":
				labels := map[string]string{}
				for key, value := range original["en"] {
					labels[key] = value
				}
				labels["@price"] = "Future price label"
				locales["en"] = labels
			}
			productEditorMatrixLedgerLocales = locales
			defer func() { productEditorMatrixLedgerLocales = original }()
			var drift productEditorMatrixEvidenceDrift
			if err := validateProductEditorMatrixEvidence(root); !errors.As(err, &drift) {
				t.Fatalf("frozen locale evolution must be typed drift: %v", err)
			}
		})
	}
	root, _ := productEditorMatrixEvidenceFixture(t)
	path := "tests/foundation/testdata/product_editor_matrix_cases.json"
	productEditorMatrixWriteFixture(t, root, path, append(productEditorMatrixReadFixture(t, root, path), '\n'))
	original := productEditorMatrixLedgerCases
	productEditorMatrixLedgerCases = append(append([]productEditorMatrixLedgerCase{}, original...), productEditorMatrixLedgerCase{Page: "mobile matrix future", Row: "new", Control: "new", Action: "click", Expected: "new"})
	defer func() { productEditorMatrixLedgerCases = original }()
	err := validateProductEditorMatrixEvidence(root)
	var drift productEditorMatrixEvidenceDrift
	if !errors.As(err, &drift) {
		t.Fatalf("internally coherent historical ledger must survive table evolution: %v", err)
	}
	t.Logf("table evolution: %v", err)
}

func productEditorMatrixWriteFixture(t *testing.T, root, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, path), data, 0600); err != nil {
		t.Fatal(err)
	}
}
func productEditorMatrixJSONFixture(t *testing.T, root, path string) map[string]any {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal(productEditorMatrixReadFixture(t, root, path), &value); err != nil {
		t.Fatal(err)
	}
	return value
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
	productEditorMatrixEvidenceMembershipControls(t)
	t.Run("content-rehashed-test-node-receipt", func(t *testing.T) {
		root, canonical := productEditorMatrixEvidenceFixture(t)
		path := filepath.Dir(canonical) + "/test-node.json"
		receipt := productEditorMatrixJSONFixture(t, root, path)
		if receipt["exit_code"] != float64(0) {
			t.Fatal("original test-node receipt must retain its recorded success")
		}
		receipt["exit_code"] = 1
		data, _ := json.Marshal(receipt)
		productEditorMatrixRehashFixture(t, root, canonical, path, data)
		gate := "tests/foundation/catalog_matrix_ledger_test.go"
		productEditorMatrixWriteFixture(t, root, gate, append(productEditorMatrixReadFixture(t, root, gate), '\n'))
		var drift productEditorMatrixEvidenceDrift
		if err := validateProductEditorMatrixEvidence(root); err == nil || errors.As(err, &drift) {
			t.Fatalf("rehashed original receipt corruption must fail before drift: %v", err)
		}
	})
	t.Run("changed-checkout-review-keeps-frozen-bytes", func(t *testing.T) {
		checkout := t.TempDir()
		const evidence = "output/matrix-ledger-enforce/historical-evidence"
		index := productEditorMatrixJSONFixture(t, "../..", evidence+"/ARTIFACTS.json")
		packagePaths := []string{evidence + "/ARTIFACTS.json", "output/matrix-ledger-enforce/DELIVERY.md"}
		for path := range index["artifacts_sha256"].(map[string]any) {
			packagePaths = append(packagePaths, path)
		}
		for _, path := range packagePaths {
			if err := os.MkdirAll(filepath.Dir(filepath.Join(checkout, path)), 0700); err != nil {
				t.Fatal(err)
			}
			productEditorMatrixWriteFixture(t, checkout, path, productEditorMatrixReadFixture(t, "../..", path))
		}
		paths := append([]string{}, productEditorMatrixEvidenceSources...)
		manifest := productEditorMatrixJSONFixture(t, "../..", "output/matrix-ledger-enforce/historical-evidence/browser/browser-evidence.json")
		for path := range manifest["informational_source_files_sha256"].(map[string]any) {
			paths = append(paths, path)
		}
		record := productEditorMatrixJSONFixture(t, "../..", "output/matrix-ledger-enforce/historical-evidence/FROZEN-IDENTITIES.json")
		for path := range record["source_snapshots"].(map[string]any) {
			paths = append(paths, path)
		}
		for _, path := range paths {
			if err := os.MkdirAll(filepath.Dir(filepath.Join(checkout, path)), 0700); err != nil {
				t.Fatal(err)
			}
			productEditorMatrixWriteFixture(t, checkout, path, productEditorMatrixReadFixture(t, "../..", path))
		}
		const consumer = "tests/admin/product-review.acceptance.ts"
		changed := append(productEditorMatrixReadFixture(t, checkout, consumer), []byte("\n// future checkout review change\n")...)
		productEditorMatrixWriteFixture(t, checkout, consumer, changed)
		root, canonical := productEditorMatrixEvidenceFixtureFromCheckout(t, checkout)
		frozen := productEditorMatrixJSONFixture(t, root, filepath.Dir(canonical)+"/FROZEN-IDENTITIES.json")
		snapshot := frozen["source_snapshots"].(map[string]any)[consumer].(string)
		original := record["source_snapshots"].(map[string]any)[consumer].(string)
		if !bytes.Equal(productEditorMatrixReadFixture(t, root, snapshot), productEditorMatrixReadFixture(t, "../..", original)) {
			t.Fatal("fixture must preserve original frozen consumer bytes after a checkout edit")
		}
		if !bytes.Equal(productEditorMatrixReadFixture(t, root, consumer), changed) {
			t.Fatal("fixture must retain the changed informational checkout consumer")
		}
		if err := validateProductEditorMatrixEvidence(root); err != nil {
			t.Fatalf("future informational consumer must not invalidate fixed historical fixture: %v", err)
		}
	})
	for _, path := range []string{
		"scripts/dev/test-node.sh", "scripts/dev/test-local.sh", "docs/delivery/GATES.md", "playwright.config.ts",
		"tests/foundation/testdata/product_editor_mobile_matrix_ledger.json",
		"apps/admin/lib/product-editor-copy.ts", "apps/admin/components/ProductDocumentVariants.tsx",
		"apps/admin/components/ProductDocumentForm.tsx", "apps/admin/lib/use-product-document.ts", "apps/admin/lib/product-document.ts",
		// Frozen consumer snapshots corroborate historical run identities; their
		// current checkout bytes are outside SOURCE.json's owned producer set.
		"tests/admin/product-review.acceptance.ts", "tests/admin/product-feedback.acceptance.ts", "tests/admin/catalog-core.spec.ts",
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

// Removing a file and its mutable index entry together must not erase history.
func productEditorMatrixEvidenceMembershipControls(t *testing.T) {
	t.Helper()
	for _, name := range []string{"test-node.json", "frozen/tests/admin/product-editor-matrix-ledger.ts.source.txt"} {
		for _, changed := range []bool{false, true} {
			t.Run(fmt.Sprintf("membership-deleted-%s-drift-%t", name, changed), func(t *testing.T) {
				root, canonical := productEditorMatrixEvidenceFixture(t)
				evidence := filepath.Dir(canonical)
				path := evidence + "/" + name
				if err := os.Remove(filepath.Join(root, path)); err != nil {
					t.Fatal(err)
				}
				indexPath := evidence + "/ARTIFACTS.json"
				index := productEditorMatrixJSONFixture(t, root, indexPath)
				delete(index["artifacts_sha256"].(map[string]any), path)
				data, _ := json.Marshal(index)
				productEditorMatrixWriteFixture(t, root, indexPath, data)
				if changed {
					gate := "tests/foundation/catalog_matrix_ledger_test.go"
					productEditorMatrixWriteFixture(t, root, gate, append(productEditorMatrixReadFixture(t, root, gate), '\n'))
				}
				var drift productEditorMatrixEvidenceDrift
				if err := validateProductEditorMatrixEvidence(root); err == nil || errors.As(err, &drift) {
					t.Fatalf("coordinated package deletion must fail before drift: %v", err)
				}
			})
		}
	}
	for _, name := range []string{"test-node.json", "frozen/tests/admin/product-editor-matrix-ledger.ts.source.txt"} {
		t.Run("membership-all-rehashed-deleted-"+name, func(t *testing.T) {
			root, canonical := productEditorMatrixEvidenceFixture(t)
			evidence := filepath.Dir(canonical)
			path := evidence + "/" + name
			if err := os.Remove(filepath.Join(root, path)); err != nil {
				t.Fatal(err)
			}
			indexPath := evidence + "/ARTIFACTS.json"
			index := productEditorMatrixJSONFixture(t, root, indexPath)
			delete(index["artifacts_sha256"].(map[string]any), path)
			data, _ := json.Marshal(index)
			productEditorMatrixWriteFixture(t, root, indexPath, data)
			membershipPath := evidence + "/FROZEN-MEMBERSHIP.json"
			membership := productEditorMatrixJSONFixture(t, root, membershipPath)
			members := []any{}
			for _, member := range membership["package_members"].([]any) {
				if member != name {
					members = append(members, member)
				}
			}
			membership["package_members"] = members
			data, _ = json.Marshal(membership)
			productEditorMatrixRehashFixture(t, root, canonical, membershipPath, data)
			recordPath := evidence + "/FROZEN-IDENTITIES.json"
			record := productEditorMatrixJSONFixture(t, root, recordPath)
			record["package_membership_sha256"] = fmt.Sprintf("%x", sha256.Sum256(data))
			// Remove auxiliary snapshot references too; stale metadata must not
			// be the reason a fully coordinated historical corruption fails.
			if strings.HasPrefix(name, "frozen/") {
				for _, field := range []string{"source_snapshots", "source_files_sha256", "git_blob_ids"} {
					delete(record[field].(map[string]any), "tests/admin/product-editor-matrix-ledger.ts")
				}
			}
			data, _ = json.Marshal(record)
			productEditorMatrixRehashFixture(t, root, canonical, recordPath, data)
			for _, bound := range []string{canonical + "/browser-evidence.json", evidence + "/SOURCE.json"} {
				value := productEditorMatrixJSONFixture(t, root, bound)
				value["frozen_identity_record_sha256"] = fmt.Sprintf("%x", sha256.Sum256(data))
				bytes, _ := json.Marshal(value)
				productEditorMatrixRehashFixture(t, root, canonical, bound, bytes)
			}
			gate := "tests/foundation/catalog_matrix_ledger_test.go"
			productEditorMatrixWriteFixture(t, root, gate, append(productEditorMatrixReadFixture(t, root, gate), '\n'))
			var drift productEditorMatrixEvidenceDrift
			if err := validateProductEditorMatrixEvidence(root); err == nil || errors.As(err, &drift) {
				t.Fatalf("fully rehashed membership omission must fail before drift: %v", err)
			}
		})
	}
}

func productEditorMatrixEvidenceFixture(t *testing.T) (string, string) {
	t.Helper()
	return productEditorMatrixEvidenceFixtureFromCheckout(t, "../..")
}

func productEditorMatrixEvidenceFixtureFromCheckout(t *testing.T, checkout string) (string, string) {
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
		data := productEditorMatrixReadFixture(t, checkout, path)
		write(path, data)
		sources[path] = hash(data)
	}
	// Isolated corruption fixtures rebind current executable bytes; delivered
	// canonical evidence still must come from an actual run on frozen sources.
	for _, path := range productEditorMatrixEvidenceSources {
		data := productEditorMatrixReadFixture(t, checkout, path)
		write(path, data)
		sources[path] = hash(data)
	}
	manifest["source_files_sha256"] = sources
	if shared, ok := manifest["informational_source_files_sha256"].(map[string]any); ok {
		for path := range shared {
			data := productEditorMatrixReadFixture(t, checkout, path)
			write(path, data)
			shared[path] = hash(data)
		}
	}
	// MODEL_ONLY: these isolated corruption fixtures exercise the validator with
	// the current table, which can evolve beyond the historical real browser run.
	// This modeled ledger never modifies or proves the delivered canonical package.
	rows := []productEditorMatrixLedgerRow{}
	perLocale := map[string]int{}
	for key, claim := range productEditorMatrixLedgerExpected() {
		expected, _ := json.Marshal(claim)
		rows = append(rows, productEditorMatrixLedgerRow{productEditorMatrixLedgerKey: key, Expected: expected, Actual: "PASS"})
		perLocale[key.Locale]++
	}
	sort.Slice(rows, func(i, j int) bool {
		left, _ := json.Marshal(rows[i].productEditorMatrixLedgerKey)
		right, _ := json.Marshal(rows[j].productEditorMatrixLedgerKey)
		return string(left) < string(right)
	})
	ledgerData, _ := json.Marshal(rows)
	manifest["ledger"] = map[string]any{"raw_rows": len(rows), "required_unique_PASS": len(rows), "per_locale": perLocale, "width": 390}
	artifacts := map[string]string{}
	// Copy the entire original package before MODEL_ONLY semantic rebinding.
	// Its complete names stay subject to the same independent oracle as delivery.
	originalEvidence := "output/matrix-ledger-enforce/" + string(dir[1])
	originalIndex := productEditorMatrixJSONFixture(t, "../..", originalEvidence+"/ARTIFACTS.json")
	for path := range originalIndex["artifacts_sha256"].(map[string]any) {
		if strings.HasSuffix(path, "/FROZEN-MEMBERSHIP.json") {
			continue
		}
		data := productEditorMatrixReadFixture(t, checkout, path)
		copyPath := filepath.Dir(canonical) + strings.TrimPrefix(path, originalEvidence)
		write(copyPath, data)
		artifacts[copyPath] = hash(data)
	}
	copies := []map[string]string{}
	for _, entry := range manifest["copied_native_bytes"].([]any) {
		old := entry.(map[string]any)
		name := filepath.Base(old["canonical"].(string))
		path := canonical + "/" + name
		data := productEditorMatrixReadFixture(t, "../..", old["canonical"].(string))
		if name == "product-editor-click-ledger.json" {
			data = ledgerData
		}
		write(path, data)
		artifacts[path] = hash(data)
		copies = append(copies, map[string]string{"canonical": path, "source_native": "fixture-only/" + name, "sha256": hash(data)})
	}
	manifest["copied_native_bytes"] = copies
	// MODEL_ONLY frozen metadata and source snapshots rebind the copied fixture,
	// never native delivered evidence. Its case source remains the sole table.
	frozen := productEditorMatrixJSONFixture(t, "../..", "output/matrix-ledger-enforce/"+string(dir[1])+"/FROZEN-IDENTITIES.json")
	frozen["fixture_evidence_class"] = "MODEL_ONLY"
	frozen["locale_labels"] = productEditorMatrixLedgerLocales
	frozen["test_identities"] = map[string][]string{"playwright.log": productEditorMatrixCurrentRunIdentities(12), "cc12-playwright.log": productEditorMatrixCurrentRunIdentities(2)}
	frozenHashes, snapshots, blobs := map[string]string{}, map[string]string{}, map[string]string{}
	for path := range frozen["source_snapshots"].(map[string]any) {
		data := productEditorMatrixReadFixture(t, checkout, path)
		write(path, data)
		snapshot := filepath.Dir(canonical) + strings.TrimPrefix(frozen["source_snapshots"].(map[string]any)[path].(string), originalEvidence)
		write(snapshot, data)
		snapshots[path], frozenHashes[path], artifacts[snapshot], blobs[path] = snapshot, hash(data), hash(data), productEditorMatrixGitBlob(data)
	}
	frozen["source_files_sha256"], frozen["source_snapshots"], frozen["git_blob_ids"] = frozenHashes, snapshots, blobs
	// MODEL_ONLY membership records this isolated package, including metadata
	// written below. Names only avoid a circular dependency on their digests.
	evidence := filepath.Dir(canonical)
	membershipPath := evidence + "/FROZEN-MEMBERSHIP.json"
	members := []string{"FROZEN-MEMBERSHIP.json"}
	for path := range artifacts {
		members = append(members, strings.TrimPrefix(path, evidence+"/"))
	}
	sort.Strings(members)
	membershipData, _ := json.Marshal(map[string]any{"tested_source_commit": manifest["tested_source_commit"], "package_members": members, "fixture_evidence_class": "MODEL_ONLY"})
	write(membershipPath, membershipData)
	artifacts[membershipPath] = hash(membershipData)
	frozen["package_membership_sha256"] = hash(membershipData)
	frozenData, _ := json.Marshal(frozen)
	frozenPath := filepath.Dir(canonical) + "/FROZEN-IDENTITIES.json"
	write(frozenPath, frozenData)
	artifacts[frozenPath] = hash(frozenData)
	manifest["frozen_identity_record_sha256"] = hash(frozenData)
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	write(canonical+"/browser-evidence.json", data)
	artifacts[canonical+"/browser-evidence.json"] = hash(data)
	source, _ := json.Marshal(map[string]any{"tested_source_commit": manifest["tested_source_commit"], "source_files_sha256": sources, "frozen_identity_record_sha256": hash(frozenData)})
	write("output/matrix-ledger-enforce/fixture/SOURCE.json", source)
	artifacts["output/matrix-ledger-enforce/fixture/SOURCE.json"] = hash(source)
	// MODEL_ONLY rebinds a COPY of the independent frozen run receipt. This is
	// fixture support, never an actual browser acceptance or canonical mutation.
	runReceipt := productEditorMatrixJSONFixture(t, "../..", "output/matrix-ledger-enforce/"+string(dir[1])+"/browser-product-editor.json")
	runReceipt["fixture_evidence_class"] = "MODEL_ONLY"
	runReceipt["tested_source_commit"] = manifest["tested_source_commit"]
	runReceipt["source_commit_after"] = manifest["tested_source_commit"]
	runReceipt["source_files_sha256"] = sources
	runReceipt["source_files_sha256_after"] = sources
	runData, _ := json.Marshal(runReceipt)
	runPath := "output/matrix-ledger-enforce/fixture/browser-product-editor.json"
	write(runPath, runData)
	artifacts[runPath] = hash(runData)
	index, _ := json.Marshal(map[string]any{"tested_source_commit": manifest["tested_source_commit"], "source_files_sha256": sources, "artifacts_sha256": artifacts})
	write("output/matrix-ledger-enforce/fixture/ARTIFACTS.json", index)
	write("output/matrix-ledger-enforce/DELIVERY.md", []byte(fmt.Sprintf("Tested source: **`%s`**\n[SOURCE.json](fixture/SOURCE.json)\n", manifest["tested_source_commit"])))
	return root, canonical
}

// Hash-consistent bytes alone are not a passing browser run. Both native final
// receipts, list-reporter summaries and manifest counts must corroborate it.
// Internal immutable ledger checks are independent of the evolving current table.
// Exact identity/expected-claim coverage still runs whenever producer hashes match.
func productEditorMatrixHistoricalLedger(data []byte) (int, error) {
	var rows []productEditorMatrixLedgerRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return 0, err
	}
	seen := map[productEditorMatrixLedgerKey]bool{}
	perLocale := map[string]int{}
	for i, row := range rows {
		if strings.TrimSpace(row.Page) == "" {
			return 0, fmt.Errorf("ledger row %d has no page identity", i)
		}
		if !strings.HasPrefix(row.Page, "mobile matrix") && !strings.HasPrefix(row.Page, "mobile axis remove") {
			continue
		}
		if seen[row.productEditorMatrixLedgerKey] {
			return 0, fmt.Errorf("duplicate mobile matrix ledger row %d", i)
		}
		var claim string
		if err := json.Unmarshal(row.Expected, &claim); err != nil || strings.TrimSpace(claim) == "" {
			return 0, fmt.Errorf("mobile matrix ledger row %d has missing expected claim", i)
		}
		if row.Actual != "PASS" || row.Width != 390 || strings.TrimSpace(row.Locale) == "" || strings.TrimSpace(row.Control) == "" || strings.TrimSpace(row.Action) == "" {
			return 0, fmt.Errorf("invalid mobile matrix ledger identity/result at row %d", i)
		}
		seen[row.productEditorMatrixLedgerKey] = true
		perLocale[row.Locale]++
	}
	count := 0
	for _, covered := range perLocale {
		if count == 0 {
			count = covered
		}
		if covered == 0 || covered != count {
			return 0, fmt.Errorf("historical matrix ledger requires equal nonempty locale coverage")
		}
	}
	if len(perLocale) == 0 {
		return 0, fmt.Errorf("historical matrix ledger requires nonempty locale coverage")
	}
	return len(seen), nil
}

func validateProductEditorMatrixRunOutcome(manifestData []byte, native map[string][]byte, required int, frozen map[string][]string) error {
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
	}{
		{"pe-last-run.json", "PE_and_review", "playwright.log"},
		{"cc12-last-run.json", "CC12_frozen", "cc12-playwright.log"},
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
		if manifest.Passed[run.field] != len(frozen[run.log]) || len(frozen[run.log]) == 0 {
			return fmt.Errorf("canonical manifest passed count %s must match frozen identities", run.field)
		}
		if err := productEditorMatrixPlaywrightSummary(native[run.log], frozen[run.log]); err != nil {
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
	if manifest.Ledger.Raw != len(rows) || manifest.Ledger.Required != required ||
		manifest.Ledger.Width != 390 || !reflect.DeepEqual(manifest.Ledger.PerLocale, perLocale) {
		return fmt.Errorf("canonical manifest ledger counts must match the exact validated PASS ledger")
	}
	return nil
}

func productEditorMatrixPlaywrightSummary(data []byte, declared []string) error {
	expected := len(declared)
	if expected == 0 {
		return fmt.Errorf("empty declared run identities")
	}
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
	identities := map[string]int{}
	for _, identity := range declared {
		identities[identity]++
	}
	cases := regexp.MustCompile(`(?m)^\s*✓\s+\d+\s+(tests/[^\r\n]+):\d+:\d+ › (.+) \([^()\r\n]*\)\s*$`).FindAllStringSubmatch(text, -1)
	if len(cases) != expected {
		return fmt.Errorf("must report exactly %d passing case identities, got %d", expected, len(cases))
	}
	for _, executed := range cases {
		identity := executed[1] + " › " + executed[2]
		if identities[identity] != 1 {
			return fmt.Errorf("unexpected or duplicate executed case identity: %s", identity)
		}
		delete(identities, identity)
	}
	if len(identities) != 0 {
		return fmt.Errorf("missing %d frozen executed case identities", len(identities))
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

// Identity controls keep summaries, final receipts and all hashes consistent.
// Only the executed consumer file/title multiset may make these runs invalid.
func TestProductEditorMatrixLedgerEvidenceRunIdentity(t *testing.T) {
	for _, mutation := range []string{"frozen-record-missing", "frozen-record-unindexed", "frozen-label", "frozen-test-duplicate", "frozen-test-unbound", "frozen-title-editor-rehashed", "frozen-title-review-rehashed", "frozen-title-feedback-rehashed", "frozen-title-cc12-rehashed", "frozen-blob", "frozen-table-bytes"} {
		t.Run(mutation, func(t *testing.T) {
			root, canonical := productEditorMatrixEvidenceFixture(t)
			gate := "tests/foundation/catalog_matrix_ledger_test.go"
			productEditorMatrixWriteFixture(t, root, gate, append(productEditorMatrixReadFixture(t, root, gate), '\n'))
			path := filepath.Dir(canonical) + "/FROZEN-IDENTITIES.json"
			record := productEditorMatrixJSONFixture(t, root, path)
			switch mutation {
			case "frozen-record-missing":
				if err := os.Remove(filepath.Join(root, path)); err != nil {
					t.Fatal(err)
				}
			case "frozen-record-unindexed":
				indexPath := filepath.Dir(canonical) + "/ARTIFACTS.json"
				index := productEditorMatrixJSONFixture(t, root, indexPath)
				delete(index["artifacts_sha256"].(map[string]any), path)
				data, _ := json.Marshal(index)
				productEditorMatrixWriteFixture(t, root, indexPath, data)
			case "frozen-table-bytes":
				table := "tests/foundation/testdata/product_editor_matrix_cases.json"
				snapshot := record["source_snapshots"].(map[string]any)[table].(string)
				productEditorMatrixRehashFixture(t, root, canonical, snapshot, []byte(`[]`))
			default:
				switch mutation {
				case "frozen-label":
					record["locale_labels"].(map[string]any)["en"].(map[string]any)["@price"] = "invented historical price"
				case "frozen-test-duplicate":
					tests := record["test_identities"].(map[string]any)["playwright.log"].([]any)
					tests[0] = tests[1]
				case "frozen-test-unbound":
					record["test_identities"].(map[string]any)["playwright.log"].([]any)[0] = "tests/fake.spec.ts › invented passing case"
				case "frozen-title-editor-rehashed", "frozen-title-review-rehashed", "frozen-title-feedback-rehashed", "frozen-title-cc12-rehashed":
					log, position := "playwright.log", 0
					switch mutation {
					case "frozen-title-review-rehashed":
						position = 2
					case "frozen-title-feedback-rehashed":
						position = 10
					case "frozen-title-cc12-rehashed":
						log = "cc12-playwright.log"
					}
					tests := record["test_identities"].(map[string]any)[log].([]any)
					original := tests[position].(string)
					parts := strings.SplitN(original, " › ", 2)
					invented := parts[1] + " invented passing title"
					tests[position] = parts[0] + " › " + invented
					logPath := canonical + "/" + log
					logData := strings.Replace(string(productEditorMatrixReadFixture(t, root, logPath)), " › "+parts[1]+" (", " › "+invented+" (", 1)
					if !strings.Contains(logData, invented) {
						t.Fatal("negative control must mutate the native title")
					}
					productEditorMatrixRehashFixture(t, root, canonical, logPath, []byte(logData))
				case "frozen-blob":
					record["git_blob_ids"].(map[string]any)[gate] = strings.Repeat("0", 40)
				}
				// Consistently rebind the extra metadata; frozen source semantics,
				// rather than an old metadata digest, must reject these mutations.
				data, _ := json.Marshal(record)
				productEditorMatrixRehashFixture(t, root, canonical, path, data)
				for _, bound := range []string{canonical + "/browser-evidence.json", filepath.Dir(canonical) + "/SOURCE.json"} {
					value := productEditorMatrixJSONFixture(t, root, bound)
					value["frozen_identity_record_sha256"] = fmt.Sprintf("%x", sha256.Sum256(data))
					bytes, _ := json.Marshal(value)
					productEditorMatrixRehashFixture(t, root, canonical, bound, bytes)
				}
			}
			var drift productEditorMatrixEvidenceDrift
			if err := validateProductEditorMatrixEvidence(root); err == nil || errors.As(err, &drift) {
				t.Fatalf("corrupt frozen evidence must fail before drift: %v", err)
			}
		})
	}
	for _, log := range []string{"playwright.log", "cc12-playwright.log"} {
		for _, mutation := range []string{"wrong-file", "wrong-title", "substituted-case", "drop-case", "duplicate-case", "same-counts-fake-run"} {
			t.Run(log+"/"+mutation, func(t *testing.T) {
				root, canonical := productEditorMatrixEvidenceFixture(t)
				if err := validateProductEditorMatrixEvidence(root); err != nil {
					t.Fatalf("untampered evidence: %v", err)
				}
				path := canonical + "/" + log
				original := string(productEditorMatrixReadFixture(t, root, path))
				lines := regexp.MustCompile(`(?m)^\s*✓[^\r\n]+`).FindAllString(original, -1)
				if len(lines) < 2 {
					t.Fatal("native case mutation anchors missing")
				}
				replacement := lines[0]
				switch mutation {
				case "wrong-file":
					replacement = strings.Replace(replacement, "tests/admin/", "tests/unrelated/", 1)
				case "wrong-title":
					replacement = strings.Replace(replacement, " › ", " › unrelated case: ", 1)
				case "substituted-case":
					replacement = "  ✓  1 tests/admin/catalog-core.spec.ts:1:1 › unrelated substituted case (1s)"
				case "drop-case":
					replacement = ""
				case "duplicate-case":
					replacement = lines[1]
				case "same-counts-fake-run":
					for _, line := range lines {
						original = strings.Replace(original, line, strings.Replace(line, "tests/admin/", "tests/fake/", 1), 1)
					}
					replacement = ""
				}
				data := []byte(original)
				if mutation != "same-counts-fake-run" {
					data = []byte(strings.Replace(original, lines[0], replacement, 1))
				}
				if string(data) == string(productEditorMatrixReadFixture(t, root, path)) {
					t.Fatal("identity mutation did not change bytes")
				}
				productEditorMatrixRehashFixture(t, root, canonical, path, data)
				if err := validateProductEditorMatrixEvidence(root); err == nil {
					t.Fatalf("consistently rehashed %s must reject executed case identity", mutation)
				} else {
					if strings.Contains(err.Error(), "SHA-256") {
						t.Fatalf("must reject identity, not hashes: %v", err)
					}
					t.Logf("rejected rehashed %s: %v", mutation, err)
				}
				gate := "tests/foundation/catalog_matrix_ledger_test.go"
				productEditorMatrixWriteFixture(t, root, gate, append(productEditorMatrixReadFixture(t, root, gate), '\n'))
				var drift productEditorMatrixEvidenceDrift
				if err := validateProductEditorMatrixEvidence(root); err == nil || errors.As(err, &drift) {
					t.Fatalf("corrupted historical identity must fail before drift: %v", err)
				}
			})
		}
	}
}

// Native list metadata is not identity: numeric locations and timings may vary.
func TestProductEditorMatrixLedgerEvidenceRunIdentityMetadata(t *testing.T) {
	root, canonical := productEditorMatrixEvidenceFixture(t)
	for _, log := range []string{"playwright.log", "cc12-playwright.log"} {
		path := canonical + "/" + log
		text := string(productEditorMatrixReadFixture(t, root, path))
		text = regexp.MustCompile(`(?m)(tests/admin/[^\r\n]+):\d+:\d+ › `).ReplaceAllString(text, "${1}:9000:99 › ")
		text = regexp.MustCompile(`(?m) \([\d.]+(?:ms|s|m)\)$`).ReplaceAllString(text, " (999.9s)")
		productEditorMatrixRehashFixture(t, root, canonical, path, []byte(text))
	}
	if err := validateProductEditorMatrixEvidence(root); err != nil {
		t.Fatalf("changed line numbers and durations must preserve identities: %v", err)
	}
}

// Current matching producers keep the exact full case contract. Historical
// checks use the indexed observed record instead of these evolving titles.
var productEditorMatrixCurrentRunIdentities = func(expected int) []string {
	identities := map[string]int{}
	add := func(file, title string) { identities["tests/admin/"+file+" › "+title]++ }
	switch expected {
	case 12:
		add("product-editor.acceptance.ts", "PE12-17 document workflow, matrix, list actions and click ledger")
		add("product-review.acceptance.ts", "review: bulk price 80 plus archive sends exclusive archive entry and persists")
		for _, action := range []string{"create", "copy"} {
			for _, denial := range []string{"client", "401", "403"} {
				add("product-review.acceptance.ts", fmt.Sprintf("review: %s UNKNOWN then %s preserves receipt fence after reload", action, denial))
			}
		}
		for _, action := range []string{"order", "publish"} {
			add("product-review.acceptance.ts", fmt.Sprintf("review: %s UNKNOWN then 403 preserves receipt fence after reload", action))
		}
		for _, width := range []int{1586, 390} {
			add("product-feedback.acceptance.ts", fmt.Sprintf("feedback: identical image-required validation reveals again at %dpx", width))
		}
	case 2:
		for _, variant := range []struct{ viewport, locale string }{{"desktop", "en"}, {"mobile", "zh-TW"}} {
			add("catalog-core.spec.ts", fmt.Sprintf("CC12 %s %s › catalog v2: draft product, 2x3 matrix, prices, compare-at, stock, photo, collection, activate, shopper view, archive a variant, deactivate (%s, %s)", variant.viewport, variant.locale, variant.viewport, variant.locale))
		}
	default:
		return nil
	}
	result := []string{}
	for identity := range identities {
		result = append(result, identity)
	}
	sort.Strings(result)
	return result
}
