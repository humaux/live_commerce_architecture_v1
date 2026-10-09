// Purpose: reproduce attribute-suppressed diffs and deletion-only unresolved changes through the actual CLI.
// Depends on: compiled CLI, git-backed fixtures and Go stdlib; custom diff drivers are task-local test data.
// Used by: PR33 r1b acceptance; no production API or repository mutation.
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReviewR1BForcedTextDiffCLI(t *testing.T) {
	for _, mode := range []string{"binary", "textconv", "mnemonic", "no-prefix"} {
		t.Run(mode, func(t *testing.T) {
			root := seedFixture(t, "drift")
			marker := filepath.Join(root, "textconv-ran")
			switch mode {
			case "binary":
				mustWrite(t, filepath.Join(root, ".gitattributes"), "*.go -diff\n")
			case "textconv":
				mustWrite(t, filepath.Join(root, ".gitattributes"), "*.go diff=scrub\n")
				script := filepath.Join(root, "scrub.sh")
				mustWrite(t, script, "#!/bin/sh\nprintf ran > '"+marker+"'\nprintf fixed\n")
				if err := os.Chmod(script, 0700); err != nil {
					t.Fatal(err)
				}
				git(t, root, "config", "diff.scrub.textconv", script)
			case "mnemonic":
				git(t, root, "config", "diff.mnemonicPrefix", "true")
			case "no-prefix":
				git(t, root, "config", "diff.noprefix", "true")
			}
			p := filepath.Join(root, "internal/httpapi/routes.go")
			mustWrite(t, p, strings.Replace(readText(t, p), `"/orphan", handler)`, `"/orphan", handler) // edited`, 1))
			if code, out := reviewCLI(t)(root); code != 1 || !strings.Contains(out, "TOUCHED") {
				t.Fatalf("%s suppressed raw changed lines: %d %s", mode, code, out)
			}
			if mode == "textconv" {
				if _, err := os.Stat(marker); !os.IsNotExist(err) {
					t.Fatal("machine diff executed textconv")
				}
			}
		})
	}
}

func TestReviewR1BDeletedUnresolvedCLI(t *testing.T) {
	for _, write := range []bool{false, true} {
		t.Run(map[bool]string{false: "ordinary", true: "write-baseline"}[write], func(t *testing.T) {
			root := fixture(t, "aligned")
			p := filepath.Join(root, "contracts/opaque-openapi.json")
			original := "{\n\"paths\": null,\n\"openapi\": \"3.0.0\"\n}\n"
			mustWrite(t, p, original)
			git(t, root, "add", ".")
			git(t, root, "commit", "-qm", "legacy opaque contract")
			git(t, root, "tag", "-f", "trunk")
			if code, out := cli(root, "-write-baseline"); code != 0 {
				t.Fatal(code, out)
			}
			git(t, root, "add", ".")
			git(t, root, "commit", "-qm", "reviewed baseline")
			git(t, root, "tag", "-f", "trunk")
			call := reviewCLI(t)
			args := []string{}
			if write {
				args = append(args, "-write-baseline")
			}
			baselinePath := filepath.Join(root, "scripts/dev/contractdrift/baseline.json")
			before := readText(t, baselinePath)
			mustWrite(t, p, strings.Replace(original, "\"paths\": null,\n", "", 1))
			if code, out := call(root, args...); code != 1 || !strings.Contains(out, "UNRESOLVED") {
				t.Fatalf("deleted unresolved source accepted: %d %s", code, out)
			}
			if readText(t, baselinePath) != before {
				t.Fatal("failed baseline write changed reviewed bytes")
			}
			mustWrite(t, p, "{\n\"paths\": {},\n\"openapi\": \"3.0.0\"\n}\n")
			if code, out := call(root, args...); code != 0 {
				t.Fatalf("resolved contract blocked: %d %s", code, out)
			}
			mustWrite(t, p, original)
			if err := os.Remove(p); err != nil {
				t.Fatal(err)
			}
			if code, out := call(root, args...); code != 0 {
				t.Fatalf("fully removed unknown blocked: %d %s", code, out)
			}
		})
	}
}

func TestReviewR1BDeletionFixLeavesUnrelatedUnknownCLI(t *testing.T) {
	root := fixture(t, "aligned")
	p := filepath.Join(root, "contracts/opaque-openapi.json")
	original := "{\n\"paths\": {\n\"bad-one\": {\"get\":{}},\n\"bad-two\": {\"get\":{}}\n}\n}\n"
	mustWrite(t, p, original)
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "two legacy independent unknowns")
	git(t, root, "tag", "-f", "trunk")
	if code, out := cli(root, "-write-baseline"); code != 0 {
		t.Fatal(code, out)
	}
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "baseline")
	git(t, root, "tag", "-f", "trunk")
	mustWrite(t, p, strings.Replace(original, "\"bad-one\": {\"get\":{}},\n", "", 1))
	call := reviewCLI(t)
	for _, args := range [][]string{nil, {"-write-baseline"}} {
		if code, out := call(root, args...); code != 0 || !strings.Contains(out, "UNRESOLVED") {
			t.Fatalf("removed unknown tainted untouched sibling: %d %s", code, out)
		}
	}
}
