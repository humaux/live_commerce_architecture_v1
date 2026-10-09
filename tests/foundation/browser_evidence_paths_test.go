//go:build browser

// Purpose: check evidence isolation without launching PG, Next or a browser.
// Depends on: shared browser evidence helpers and synthetic filesystem fixtures.
// Used by: browser-tag unit validation; independent acceptance reruns.
// Invariants: explicit roots survive; run writes do not alter historical output.
package foundation_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBrowserEvidencePaths(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "output", "ui-click-sweep", "journeys.json")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o700); err != nil {
		t.Fatal(err)
	}
	original := []byte("historical journey evidence\n")
	if err := os.WriteFile(legacy, original, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LC_BROWSER_EVIDENCE_ROOT", "")
	if got, want := browserEvidenceRoot(root), filepath.Join(root, "output", "playwright"); got != want {
		t.Fatalf("default=%s want=%s", got, want)
	}
	for _, supplied := range []bool{false, true} {
		t.Run(map[bool]string{false: "default", true: "explicit"}[supplied], func(t *testing.T) {
			base := filepath.Join(root, "output", "playwright")
			if supplied {
				base = t.TempDir()
				t.Setenv("LC_BROWSER_EVIDENCE_ROOT", base)
			}
			for _, create := range []func(*testing.T, string, string) string{brfEvidence, adminEvidence} {
				dir := create(t, root, "synthetic")
				rel, err := filepath.Rel(base, dir)
				if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
					t.Fatalf("artifact escaped chosen base: %s", dir)
				}
				if err := os.WriteFile(filepath.Join(dir, "ledger.json"), []byte("{}\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
	if data, err := os.ReadFile(legacy); err != nil || string(data) != string(original) {
		t.Fatalf("historical journey evidence changed: %v", err)
	}
}
