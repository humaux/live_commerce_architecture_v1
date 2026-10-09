// Purpose: adversarial ratchet/contract tests from the independent review counterexamples.
// Depends on: actual run/scanContracts/compare, temporary git fixture helpers and Go stdlib.
// Used by: CI-DRIFT local and check-gates acceptance; no stand-in parser.
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func seedFixture(t *testing.T, name string) string {
	t.Helper()
	root := fixture(t, name)
	code, out := cli(root, "-write-baseline")
	if code != 0 {
		t.Fatalf("seed %d %s", code, out)
	}
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "reviewed baseline")
	git(t, root, "tag", "-f", "trunk")
	return root
}
func mustWrite(t *testing.T, p, s string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(s), 0600); err != nil {
		t.Fatal(err)
	}
}
func readText(t *testing.T, p string) string {
	t.Helper()
	b, e := os.ReadFile(p)
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}
func TestInitialSeedCannotGrandfatherNewDeletion(t *testing.T) {
	root := fixture(t, "aligned")
	p := filepath.Join(root, "internal/httpapi/routes.go")
	s := readText(t, p)
	s = strings.Replace(s, `mux.HandleFunc(http.MethodGet+" "+(alias),handler)`, "", 1)
	s = strings.Replace(s, `mux.HandleFunc(http.MethodGet+" "+(alias), handler)`, "", 1)
	s = strings.Replace(s, `mux.HandleFunc(http.MethodGet+" "+(alias),handler)`, "", 1)
	// gofmt inserts spaces around +: mutate the exact complete registration line instead of rewriting a helper.
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if strings.Contains(line, "mux.HandleFunc") {
			lines[i] = ""
		}
	}
	mustWrite(t, p, strings.Join(lines, "\n"))
	code, out := cli(root, "-write-baseline")
	if code != 1 || !strings.Contains(out, "NEW") {
		t.Fatalf("new deletion seeded: %d %s", code, out)
	}
	if _, e := os.Stat(filepath.Join(root, "scripts/dev/contractdrift/baseline.json")); !os.IsNotExist(e) {
		t.Fatal("failed initial seed wrote baseline")
	}
}
func TestDeletedBaselineCannotReseedNewDrift(t *testing.T) {
	root := seedFixture(t, "aligned")
	bp := filepath.Join(root, "scripts/dev/contractdrift/baseline.json")
	if e := os.Remove(bp); e != nil {
		t.Fatal(e)
	}
	p := filepath.Join(root, "internal/httpapi/routes.go")
	lines := strings.Split(readText(t, p), "\n")
	for i, line := range lines {
		if strings.Contains(line, "mux.HandleFunc") {
			lines[i] = ""
		}
	}
	mustWrite(t, p, strings.Join(lines, "\n"))
	code, out := cli(root, "-write-baseline")
	if code != 1 || !strings.Contains(out, "NEW") {
		t.Fatalf("reseed widened reviewed baseline: %d %s", code, out)
	}
	if _, e := os.Stat(bp); !os.IsNotExist(e) {
		t.Fatal("failed reseed wrote baseline")
	}
}
func TestBaseDirectiveAndShiftedDeletionProvenance(t *testing.T) {
	root := fixture(t, "aligned")
	p := filepath.Join(root, "contracts/test.md")
	mustWrite(t, p, "Route base: /v1/admin/stores/{store_id}\n`POST .../ghost`\n`POST .../ghost`\n")
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "legacy duplicate relative contract")
	git(t, root, "tag", "-f", "trunk")
	code, out := cli(root, "-write-baseline")
	if code != 0 {
		t.Fatal(code, out)
	}
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "baseline")
	git(t, root, "tag", "-f", "trunk")
	original := readText(t, p)
	mustWrite(t, p, strings.Replace(original, "{store_id}", "{store}", 1))
	code, out = cli(root)
	if code != 1 || !strings.Contains(out, "TOUCHED") {
		t.Fatalf("changed normalized base ignored: %d %s", code, out)
	}
	mustWrite(t, p, original)
	mustWrite(t, p, strings.Repeat("non-route text\n", 100)+original)
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "move route lines without editing routes")
	git(t, root, "tag", "-f", "trunk")
	code, out = cli(root)
	if code != 0 {
		t.Fatalf("line shifts are not source edits: %d %s", code, out)
	}
	moved := readText(t, p)
	mustWrite(t, p, strings.Replace(moved, "`POST .../ghost`\n", "", 1))
	code, out = cli(root)
	if code != 1 || !strings.Contains(out, "TOUCHED") {
		t.Fatalf("deleted duplicate after line shift missed: %d %s", code, out)
	}
}
func TestOpaqueMethodsNeverBecomeFalseMissing(t *testing.T) {
	ref := location{File: "opaque.go", Line: 1}
	g := inventory{Routes: []route{{"GET", "/v1/x", []location{ref}}}, Unresolved: []finding{{Kind: "UNRESOLVED", Method: "PREFIX", Path: "/v1/", Locations: []location{ref}}}}
	d := inventory{Routes: []route{{"GET", "/v1/x", []location{ref}}, {"POST", "/v1/x", []location{ref}}}}
	b := inventory{Routes: []route{{"POST", "/v1/x", []location{ref}}}}
	for _, f := range compare(g, d, b) {
		if f.Kind == "METHOD_MISMATCH" || f.Kind == "CONTRACT_ONLY" || f.Kind == "BFF_NO_GO" {
			t.Fatalf("opaque absence asserted as %s", f.Kind)
		}
	}
}
func TestBaselineGrowthMissingBaseAndShrink(t *testing.T) {
	root := seedFixture(t, "drift")
	bp := filepath.Join(root, "scripts/dev/contractdrift/baseline.json")
	original := readText(t, bp)
	b, err := decodeBaseline([]byte(original))
	if err != nil {
		t.Fatal(err)
	}
	b.Entries = append(b.Entries, finding{Kind: "GO_ONLY", Method: "GET", Path: "/v1/added", Locations: []location{{File: "fake.go", Line: 1}}})
	if err := writeBaseline(bp, b); err != nil {
		t.Fatal(err)
	}
	code, out := cli(root)
	if code != 1 || !strings.Contains(out, "BASELINE_ADDED") {
		t.Fatalf("baseline growth allowed: %d %s", code, out)
	}
	mustWrite(t, bp, original)
	code, out = cli(root, "-base", "no-such-ref")
	if code != 1 || !strings.Contains(out, "merge-base") {
		t.Fatalf("missing base guessed: %d %s", code, out)
	}
	p := filepath.Join(root, "internal/httpapi/routes.go")
	lines := strings.Split(readText(t, p), "\n")
	for i, line := range lines {
		if strings.Contains(line, "/orphan") {
			lines[i] = ""
		}
	}
	mustWrite(t, p, strings.Join(lines, "\n"))
	code, out = cli(root, "-write-baseline")
	if code != 0 {
		t.Fatalf("legal shrink rejected: %d %s", code, out)
	}
	code, out = cli(root)
	if code != 0 || strings.Contains(out, "STALE") {
		t.Fatalf("shrunk baseline failed: %d %s", code, out)
	}
}
func TestMalformedJSONIsUnresolved(t *testing.T) {
	root := fixture(t, "aligned")
	p := filepath.Join(root, "contracts/test-openapi.json")
	for _, s := range []string{`{"paths":null}`, `{"paths":{"/v1/x":{"get":42}}}`, `{"paths":{"/v1/x":{"$ref":"#/x"}}}`, `{"paths":{},"paths":{}}`} {
		mustWrite(t, p, s)
		inv, e := scanContracts(root)
		if e != nil {
			t.Fatal(e)
		}
		if len(inv.Unresolved) == 0 {
			t.Errorf("silently dropped %s", s)
		}
	}
	if _, e := decodeBaseline([]byte(`{"version":1,"seed":"x","entries":[]} {}`)); e == nil {
		t.Fatal("trailing baseline JSON accepted")
	}
}

func TestSnapshotKeepsModuleScope(t *testing.T) {
	root := fixture(t, "aligned")
	mustWrite(t, filepath.Join(root, "go.mod"), "module example\n\ngo 1.22\n")
	if err := os.MkdirAll(filepath.Join(root, "cmd/api"), 0700); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(root, "cmd/api/main.go"), "package main\nimport _ \"example/internal/httpapi\"\nfunc main(){}\n")
	p := filepath.Join(root, "internal/httpapi/routes.go")
	original := readText(t, p)
	mustWrite(t, p, strings.Replace(original, "base := rootPrefix", "base := rootPrefix\n mux.HandleFunc(\"GET /v1/a\",handler)", 1))
	if err := os.MkdirAll(filepath.Join(root, "internal/workerbridge"), 0700); err != nil {
		t.Fatal(err)
	}
	worker := filepath.Join(root, "internal/workerbridge/worker.go")
	w := "package workerbridge\nimport \"net/http\"\nfunc bridge(mux *http.ServeMux){mux.HandleFunc(\"GET /v1/a\",func(http.ResponseWriter,*http.Request){})}\n"
	mustWrite(t, worker, w)
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "modular source with unreferenced worker")
	git(t, root, "tag", "-f", "trunk")
	code, out := cli(root, "-write-baseline")
	if code != 0 {
		t.Fatalf("seed %d %s", code, out)
	}
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "baseline")
	git(t, root, "tag", "-f", "trunk")
	mustWrite(t, worker, strings.Replace(w, "})}", "})} // worker only", 1))
	code, out = cli(root)
	if code != 0 {
		t.Fatalf("unreferenced worker touched API legacy: %d %s", code, out)
	}
}
func TestMultilineUnresolvedJSONValueIsTouched(t *testing.T) {
	for _, body := range []string{"{\n\"paths\": {\n\"/v1/unresolved\": {\n\"$ref\":\n\"#/components/X\"\n}\n}\n}\n", "{\n\"paths\":\nnull\n}\n"} {
		root := fixture(t, "aligned")
		p := filepath.Join(root, "contracts/test-openapi.json")
		mustWrite(t, p, body)
		git(t, root, "add", ".")
		git(t, root, "commit", "-qm", "legacy unresolved JSON")
		git(t, root, "tag", "-f", "trunk")
		code, out := cli(root)
		if code != 0 {
			t.Fatalf("legacy unresolved %d %s", code, out)
		}
		changed := strings.Replace(body, "#/components/X", "#/components/Y", 1)
		if changed == body {
			changed = strings.Replace(body, "null", "42", 1)
		}
		mustWrite(t, p, changed)
		code, out = cli(root)
		if code != 1 || !strings.Contains(out, "TOUCHED") || !strings.Contains(out, "UNRESOLVED") {
			t.Fatalf("changed value not fenced: %d %s", code, out)
		}
	}
}

func TestInitialLoadedBaselineCannotHideNewDrift(t *testing.T) {
	root := fixture(t, "aligned")
	p := filepath.Join(root, "internal/httpapi/routes.go")
	lines := strings.Split(readText(t, p), "\n")
	for i, line := range lines {
		if strings.Contains(line, "mux.HandleFunc") {
			lines[i] = ""
		}
	}
	mustWrite(t, p, strings.Join(lines, "\n"))
	g, err := scanGo(root)
	if err != nil {
		t.Fatal(err)
	}
	d, err := scanContracts(root)
	if err != nil {
		t.Fatal(err)
	}
	f, err := scanBFF(root)
	if err != nil {
		t.Fatal(err)
	}
	b := baseline{Version: 1, Seed: "handwritten", Entries: []finding{}}
	for _, finding := range compare(g, d, f) {
		if mismatchKinds[finding.Kind] {
			b.Entries = append(b.Entries, finding)
		}
	}
	bp := filepath.Join(root, "scripts/dev/contractdrift/baseline.json")
	if err := writeBaseline(bp, b); err != nil {
		t.Fatal(err)
	}
	before := readText(t, bp)
	code, out := cli(root)
	if code != 1 || !strings.Contains(out, "BASELINE_ADDED") {
		t.Fatalf("initial normal-mode grandfathering bypass: %d %s", code, out)
	}
	if readText(t, bp) != before {
		t.Fatal("normal gate rewrote baseline")
	}
}
