// Purpose: Exercise canonical matrix evidence with isolated byte, source and run-outcome corruptions.
// Depends on: the real evidence validator and native canonical receipts; no DB, browser or network calls.
// Used by: foundation shards; fixture mutations never modify delivered evidence or source files.
package foundation_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
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
		{"tampered-matching", "", "artifact", 1, "--- FAIL: TestProductEditorMatrixLedgerEvidence"},
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
			if tc.mutation == "source" && !strings.Contains(string(output), "canonical evidence is historical (source drift: "+path+")") {
				t.Fatal("source drift must explicitly identify the historical record and changed file")
			}
			if tc.mutation == "artifact" && (!strings.Contains(string(output), "SHA-256 mismatch") || strings.Contains(string(output), "--- SKIP:")) {
				t.Fatal("matching-source byte tampering must fail, never skip")
			}
		})
	}
}

// TestProductEditorMatrixLedgerEvidenceCorruption exercises the real validator
// with isolated filesystem mutations, never the delivered evidence in place.
func TestProductEditorMatrixLedgerEvidenceCorruption(t *testing.T) {
	for _, mutation := range []string{"playwright-log", "cc12-log", "ledger", "manifest", "source", "gate-source", "case-table"} {
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
			case "case-table":
				// Swapping the recorded table for the current one must not pass silently.
				path, data = productEditorMatrixCasesPath, productEditorMatrixLedgerCasesJSON
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

// #147 CI-10: the "case-table" corruption above stops at the source-hash check. These rebind every digest to a changed recorded
// table, so the ledger-versus-recorded-table comparison itself must reject: a case the ledger never ran, and a ledger row the table no longer declares.
func TestProductEditorMatrixLedgerEvidenceRecordedTableMismatch(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func([]map[string]any) []map[string]any
		want   string
	}{
		{"case-added", func(table []map[string]any) []map[string]any {
			extra := map[string]any{}
			for k, v := range table[0] {
				extra[k] = v
			}
			extra["control"] = "matrix-select-never-ran"
			return append(table, extra)
		}, "mobile matrix ledger is missing"},
		{"case-removed", func(table []map[string]any) []map[string]any { return table[1:] }, "unexpected mobile matrix ledger identity"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, _ := productEditorMatrixEvidenceFixtureWith(t, tc.mutate)
			err := validateProductEditorMatrixEvidence(root)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want a ledger-versus-table rejection containing %q, got %v", tc.want, err)
			}
			if strings.Contains(err.Error(), "SHA-256") {
				t.Fatalf("must be rejected by the table comparison, not by a digest check: %v", err)
			}
		})
	}
}

// productEditorMatrixRecordedBlob reads path at the tested commit and fails closed
// unless its bytes match the SHA-256 the package recorded for it.
func productEditorMatrixRecordedBlob(t *testing.T, commit, path, recorded string) []byte {
	t.Helper()
	data, err := exec.Command("git", "-C", "../..", "show", commit+":"+path).Output()
	if err != nil {
		t.Fatalf("recorded %s at %s unavailable (needs full history): %v", path, commit, err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(data)) != recorded {
		t.Fatalf("recorded %s at %s does not match its pinned SHA-256", path, commit)
	}
	return data
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
	return productEditorMatrixEvidenceFixtureWith(t, nil)
}

// productEditorMatrixEvidenceFixtureWith builds the isolated checkout; mutateCases, when set, replaces the recorded case table
// BEFORE every digest (manifest, SOURCE.json, ARTIFACTS.json) is bound, so the result is hash-consistent and only the
// ledger-versus-recorded-table comparison can reject it.
func productEditorMatrixEvidenceFixtureWith(t *testing.T, mutateCases func([]map[string]any) []map[string]any) (string, string) {
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
	for path, recorded := range manifest["source_files_sha256"].(map[string]any) {
		data := productEditorMatrixReadFixture(t, "../..", path)
		if path == productEditorMatrixCasesPath {
			// The package is compared with its own recorded table, never the current one.
			data = productEditorMatrixRecordedBlob(t, manifest["tested_source_commit"].(string), path, recorded.(string))
			if mutateCases != nil {
				var table []map[string]any
				if err := json.Unmarshal(data, &table); err != nil {
					t.Fatal(err)
				}
				var err error
				if data, err = json.Marshal(mutateCases(table)); err != nil {
					t.Fatal(err)
				}
			}
		}
		write(path, data)
		sources[path] = hash(data)
	}
	// Isolated corruption fixtures rebind current executable bytes; delivered
	// canonical evidence still must come from an actual run on frozen sources.
	for _, path := range []string{"tests/foundation/catalog_matrix_evidence_test.go", "tests/ci/product-editor-matrix-execution.mjs", "tests/ci/product-editor-matrix-execution.test.mjs", "tests/ci/product-editor-matrix-controls.mjs", "tests/ci/product-editor-matrix-controls.test.mjs", "apps/admin/components/ProductBulkFill.tsx"} {
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
func validateProductEditorMatrixRunOutcome(manifestData []byte, native map[string][]byte, required int) error {
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
	if manifest.Ledger.Raw != len(rows) || manifest.Ledger.Required != required ||
		manifest.Ledger.Width != productEditorMatrixLedgerWidth || !reflect.DeepEqual(manifest.Ledger.PerLocale, perLocale) {
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
	// Frozen consumer identities come from catalog-core's registered acceptance
	// cases, corroborated by round4 native list-reporter output. Positions and
	// durations vary; each exact consumer file/title must pass once, no substitutes.
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
		return fmt.Errorf("no frozen case identities for %d cases", expected)
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
