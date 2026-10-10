// Purpose: drive the actual CI-DRIFT CLI over small source trees, including real git changed-line seams.
// Depends on: Go stdlib testing, fixture files, git and run; no fake parser or dependency.
// Used by: check-gates.sh and the unit red/green gate.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T, name string) string {
	t.Helper()
	root := t.TempDir()
	err := filepath.WalkDir(filepath.Join("testdata", name), func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(filepath.Join("testdata", name), path)
		if err != nil {
			return err
		}
		dst := filepath.Join(root, rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0700)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, b, 0600)
	})
	if err != nil {
		t.Fatal(err)
	}
	git(t, root, "init", "-q")
	git(t, root, "config", "user.name", "CI fixture")
	git(t, root, "config", "user.email", "ci@example.invalid")
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "fixture base")
	git(t, root, "tag", "trunk")
	return root
}
func git(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %s %v", args, b, err)
	}
}
func cli(root string, args ...string) (int, string) {
	var out bytes.Buffer
	code := run(append([]string{"-root", root, "-base", "trunk"}, args...), &out, &out)
	return code, out.String()
}

func TestThreeWayKinds(t *testing.T) {
	root := fixture(t, "drift")
	code, out := cli(root)
	if code != 1 {
		t.Fatalf("new drift exit=%d, want1: %s", code, out)
	}
	for _, want := range []string{"GO_ONLY GET /v1/admin/stores/{}/orphan", "CONTRACT_ONLY POST /v1/admin/stores/{}/ghost", "BFF_NO_GO POST /v1/admin/stores/{}/phantom", "METHOD_MISMATCH", "internal/httpapi/routes.go:", "contracts/test-openapi.json:", "route.ts:"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %s", want, out)
		}
	}
}
func TestASTAndParameterNormalization(t *testing.T) {
	root := fixture(t, "aligned")
	code, out := cli(root)
	if code != 0 || !strings.Contains(out, "SUMMARY routes go=1 contract=1 bff=1") {
		t.Fatalf("aligned concat/alias/parameters: exit%d %s", code, out)
	}
	if strings.Contains(out, "GO_ONLY") || strings.Contains(out, "BFF_NO_GO") || strings.Contains(out, "UNRESOLVED") {
		t.Fatalf("same route across parameter dialects disagrees: %s", out)
	}
}
func TestLegacyTouchedAndStaleRatchet(t *testing.T) {
	root := fixture(t, "drift")
	code, out := cli(root, "-write-baseline")
	if code != 0 {
		t.Fatalf("seed: %d %s", code, out)
	}
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "initial reviewed baseline")
	git(t, root, "tag", "-f", "trunk")
	code, out = cli(root)
	if code != 0 || !strings.Contains(out, "WARNING (legacy)") {
		t.Fatalf("untouched legacy: %d %s", code, out)
	}
	p := filepath.Join(root, "internal/httpapi/routes.go")
	original, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	touched := strings.Replace(string(original), `"/orphan", handler)`, `"/orphan", handler) // edited`, 1)
	if touched == string(original) {
		t.Fatal("mutation anchor missing")
	}
	if err := os.WriteFile(p, []byte(touched), 0600); err != nil {
		t.Fatal(err)
	}
	code, out = cli(root)
	if code != 1 || !strings.Contains(out, "TOUCHED") {
		t.Fatalf("working-tree touched legacy: %d %s", code, out)
	}
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "touch legacy route")
	code, out = cli(root)
	if code != 1 || !strings.Contains(out, "TOUCHED") {
		t.Fatalf("committed touched legacy: %d %s", code, out)
	}
	git(t, root, "reset", "--hard", "trunk")
	removed := strings.Replace(string(original), `mux.HandleFunc("GET "+base+"/orphan", handler)`, "", 1)
	if removed == string(original) {
		t.Fatal("removal anchor missing")
	}
	if err := os.WriteFile(p, []byte(removed), 0600); err != nil {
		t.Fatal(err)
	}
	code, out = cli(root)
	if code != 1 || !strings.Contains(out, "STALE") {
		t.Fatalf("removed baseline drift: %d %s", code, out)
	}
}
func TestUnresolvedNeverSilent(t *testing.T) {
	root := fixture(t, "unresolved")
	code, out := cli(root)
	if code != 0 || !strings.Contains(out, "UNRESOLVED") || !strings.Contains(out, "internal/httpapi/routes.go:") {
		t.Fatalf("legacy unresolved: %d %s", code, out)
	}
	p := filepath.Join(root, "internal/httpapi/routes.go")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	b = []byte(strings.Replace(string(b), "pattern()", "otherPattern()", 1))
	if err := os.WriteFile(p, b, 0600); err != nil {
		t.Fatal(err)
	}
	code, out = cli(root)
	if code != 1 || !strings.Contains(out, "UNRESOLVED") {
		t.Fatalf("changed unresolved: %d %s", code, out)
	}
}
