// Purpose: reproduce PR33 BFF deletion and diff-content attribution bypasses through the real CLI process.
// Depends on: Go stdlib subprocess/context, actual compiled contractdrift and git-backed fixture helpers.
// Used by: CI-DRIFT package gate; no substituted parser or production service.
package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func reviewCLI(t *testing.T) func(string, ...string) (int, string) {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "contractdrift")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build real CLI: %v %s", err, out)
	}
	return func(root string, args ...string) (int, string) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, bin, append([]string{"-root", root, "-base", "trunk"}, args...)...)
		out, err := cmd.CombinedOutput()
		if ctx.Err() != nil {
			t.Fatalf("CLI deadline: %v", ctx.Err())
		}
		if err == nil {
			return 0, string(out)
		}
		if exit, ok := err.(*exec.ExitError); ok {
			return exit.ExitCode(), string(out)
		}
		t.Fatalf("real CLI: %v", err)
		return -1, string(out)
	}
}

func TestReviewBFFRemovedRealCLI(t *testing.T) {
	root := seedFixture(t, "aligned")
	call := reviewCLI(t)
	p := filepath.Join(root, "apps/admin/app/api/stores/[store]/[...resource]/route.ts")
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	code, out := call(root)
	if code != 1 || !strings.Contains(out, "ERROR") || !strings.Contains(out, "BFF_REMOVED") || !strings.Contains(out, "route.ts:") {
		t.Fatalf("deleted BFF ignored: exit=%d %s", code, out)
	}
	before := readText(t, filepath.Join(root, "scripts/dev/contractdrift/baseline.json"))
	code, out = call(root, "-write-baseline")
	if code != 1 || !strings.Contains(out, "BFF_REMOVED") {
		t.Fatalf("deleted mapping grandfathered: %d %s", code, out)
	}
	if readText(t, filepath.Join(root, "scripts/dev/contractdrift/baseline.json")) != before {
		t.Fatal("failed write modified baseline")
	}
}

func TestReviewBFFRetargetRealCLI(t *testing.T) {
	root := fixture(t, "aligned")
	p := filepath.Join(root, "internal/httpapi/routes.go")
	s := readText(t, p)
	s = strings.Replace(s, "\n}\nfunc handler", "\n mux.HandleFunc(\"GET \"+base+\"/replacement/{id}\",handler)\n}\nfunc handler", 1)
	mustWrite(t, p, s)
	p = filepath.Join(root, "contracts/test.md")
	mustWrite(t, p, readText(t, p)+"\n`GET .../replacement/{id}`\n")
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "backend-only route exists before BFF retarget")
	git(t, root, "tag", "-f", "trunk")
	if code, out := cli(root, "-write-baseline"); code != 0 {
		t.Fatal(code, out)
	}
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "reviewed baseline")
	git(t, root, "tag", "-f", "trunk")
	call := reviewCLI(t)
	if code, out := call(root); code != 0 {
		t.Fatalf("backend-only route must stay allowed: %d %s", code, out)
	}
	p = filepath.Join(root, "apps/admin/app/api/stores/[store]/[...resource]/route.ts")
	mustWrite(t, p, strings.ReplaceAll(readText(t, p), "widgets/", "replacement/"))
	if code, out := call(root); code != 1 || !strings.Contains(out, "BFF_REMOVED") {
		t.Fatalf("retargeting old BFF ignored: %d %s", code, out)
	}
}

func TestReviewBFFDuplicateProducerAndJointRetirement(t *testing.T) {
	for _, joint := range []bool{false, true} {
		t.Run(map[bool]string{false: "duplicate producer", true: "joint retirement"}[joint], func(t *testing.T) {
			root := fixture(t, "aligned")
			p := filepath.Join(root, "apps/admin/app/api/stores/[store]/[...resource]/route.ts")
			if !joint {
				copy := filepath.Join(root, "apps/admin/app/api/v2/stores/[store]/[...resource]/route.ts")
				if err := os.MkdirAll(filepath.Dir(copy), 0700); err != nil {
					t.Fatal(err)
				}
				mustWrite(t, copy, readText(t, p))
				git(t, root, "add", ".")
				git(t, root, "commit", "-qm", "two real BFF producers")
				git(t, root, "tag", "-f", "trunk")
			}
			if code, out := cli(root, "-write-baseline"); code != 0 {
				t.Fatal(code, out)
			}
			git(t, root, "add", ".")
			git(t, root, "commit", "-qm", "reviewed baseline")
			git(t, root, "tag", "-f", "trunk")
			if err := os.Remove(p); err != nil {
				t.Fatal(err)
			}
			if joint {
				for _, file := range []string{"internal/httpapi/routes.go", "contracts/test.md", "contracts/test-openapi.json"} {
					if err := os.Remove(filepath.Join(root, file)); err != nil {
						t.Fatal(err)
					}
				}
			}
			code, out := reviewCLI(t)(root)
			if joint && code != 0 {
				t.Fatalf("full joint retirement blocked: %d %s", code, out)
			}
			if !joint && (code != 1 || !strings.Contains(out, "BFF_REMOVED") || !strings.Contains(out, "bff=1")) {
				t.Fatalf("duplicate aggregate hid removal: %d %s", code, out)
			}
		})
	}
}

func TestReviewDiffHunkHeadersRealCLI(t *testing.T) {
	root := fixture(t, "drift")
	p := filepath.Join(root, "internal/httpapi/routes.go")
	s := readText(t, p)
	s = strings.Replace(s, "\tmux.HandleFunc(\"GET \"+base+\"/orphan\", handler)", "/*\n-- a/not-the-source.go\n*/\n\tmux.HandleFunc(\"GET \"+base+\"/orphan\", handler)", 1)
	mustWrite(t, p, s)
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "legacy source comment")
	git(t, root, "tag", "-f", "trunk")
	if code, out := cli(root, "-write-baseline"); code != 0 {
		t.Fatal(code, out)
	}
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "reviewed baseline")
	git(t, root, "tag", "-f", "trunk")
	s = strings.Replace(s, "-- a/not-the-source.go", "++ b/not-the-source.go", 1)
	s = strings.Replace(s, "base+\"/orphan\", handler", "base+\"/orphan\", otherHandler", 1)
	mustWrite(t, p, s)
	call := reviewCLI(t)
	if code, out := call(root); code != 1 || !strings.Contains(out, "TOUCHED") || !strings.Contains(out, "internal/httpapi/routes.go:") {
		t.Fatalf("hunk content hijacked file context: %d %s", code, out)
	}
}
