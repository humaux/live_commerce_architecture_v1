// Purpose: reproduce PR33 Go extraction/path review counterexamples through the actual OS CLI.
// Depends on: Go stdlib, go build, git and existing source fixture helpers; no parser stand-in.
// Used by: contractdrift review acceptance and the parent integrator's independent rerun.
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

func goReviewProcessCLI(t *testing.T, root string, args ...string) (int, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	binary := filepath.Join(t.TempDir(), "contractdrift")
	if out, err := exec.CommandContext(ctx, "go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build real CLI: %v %s", err, out)
	}
	cmd := exec.CommandContext(ctx, binary, append([]string{"-root", root, "-base", "trunk"}, args...)...)
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("CLI timeout: %v %s", ctx.Err(), out)
	}
	if err == nil {
		return 0, string(out)
	}
	if exit, ok := err.(*exec.ExitError); ok {
		return exit.ExitCode(), string(out)
	}
	t.Fatalf("start real CLI: %v", err)
	return -1, string(out)
}

func TestGoReviewMethodlessHandleCLI(t *testing.T) {
	root := fixture(t, "aligned")
	p := filepath.Join(root, "internal/httpapi/routes.go")
	s := readText(t, p) + `
func mount(mux *http.ServeMux) {
 child := childHandler()
 mux.Handle("/v1/review_mount", child)
}
func childHandler() http.Handler { return http.NotFoundHandler() }
`
	mustWrite(t, p, s)
	code, out := goReviewProcessCLI(t, root)
	if code != 1 || !strings.Contains(out, "ERROR (TOUCHED) UNRESOLVED MOUNT /v1/review_mount") {
		t.Fatalf("new methodless Handle escaped gate: exit%d %s", code, out)
	}
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "legacy opaque mount")
	git(t, root, "tag", "-f", "trunk")
	code, out = goReviewProcessCLI(t, root)
	if code != 0 || !strings.Contains(out, "WARNING (unresolved) UNRESOLVED MOUNT /v1/review_mount") {
		t.Fatalf("legacy mount did not remain a warning: exit%d %s", code, out)
	}
	mustWrite(t, p, strings.Replace(s, `return http.NotFoundHandler()`, `return http.NotFoundHandler() /* changed child */`, 1))
	code, out = goReviewProcessCLI(t, root)
	if code != 1 || !strings.Contains(out, "ERROR (TOUCHED) UNRESOLVED MOUNT /v1/review_mount") {
		t.Fatalf("touched handler dependency escaped mount gate: exit%d %s", code, out)
	}
	mustWrite(t, p, strings.Replace(s, `mux.Handle("/v1/review_mount", child)`, `mux.Handle("/v1/review_mount", child) // touched`, 1))
	code, out = goReviewProcessCLI(t, root)
	if code != 1 || !strings.Contains(out, "ERROR (TOUCHED) UNRESOLVED MOUNT /v1/review_mount") {
		t.Fatalf("touched methodless Handle escaped gate: exit%d %s", code, out)
	}
}

func TestGoReviewRouteSuffixCLI(t *testing.T) {
	root := fixture(t, "aligned")
	p := filepath.Join(root, "internal/httpapi/routes.go")
	mustWrite(t, p, readText(t, p)+`
func registerReview(mux *http.ServeMux) { mux.HandleFunc("GET /v1/review_suffix", makeRoute("")) }
func makeRoute(name string) http.HandlerFunc { return handler }
`)
	code, out := goReviewProcessCLI(t, root)
	if code != 1 || !strings.Contains(out, "GO_ONLY GET /v1/review_suffix") || strings.Contains(out, "GO_405_FALLBACK GET /v1/review_suffix") {
		t.Fatalf("constructor name concealed business route: exit%d %s", code, out)
	}
}

func TestGoReviewFactory405CLI(t *testing.T) {
	root := fixture(t, "aligned")
	mustWrite(t, filepath.Join(root, "go.mod"), "module example\n")
	p := filepath.Join(root, "internal/httpapi/routes.go")
	s := strings.Replace(readText(t, p), `import "net/http"`, "import \"net/http\"\nimport \"example/internal/response\"", 1) + `
func registerReview(mux *http.ServeMux) {
 mux.HandleFunc("POST /v1/review_factory", unrelatedName("", handler))
 mux.HandleFunc("/v1/review_factory_methodless", unrelatedName("", handler))
}
func unrelatedName(method string, next http.HandlerFunc) http.HandlerFunc {
 return func(w http.ResponseWriter, r *http.Request) {
  w.Header().Set("Cache-Control", "private, no-store")
  if r.Method != method { respondReview(w, http.StatusMethodNotAllowed); return }
  next(w, r)
 }
}
func respondReview(w http.ResponseWriter, status int) { response.Write(w, status) }
`
	mustWrite(t, p, s)
	helper := filepath.Join(root, "internal/response/writer.go")
	if err := os.MkdirAll(filepath.Dir(helper), 0700); err != nil {
		t.Fatal(err)
	}
	writer := `package response
import "net/http"
func Write(w http.ResponseWriter, status int) { write(w, status) }
func write(w http.ResponseWriter, status int) { w.Header().Set("Content-Type", "application/json"); w.WriteHeader(status) }
`
	mustWrite(t, helper, writer)
	code, out := goReviewProcessCLI(t, root, "-json")
	if code != 0 || !strings.Contains(out, "GO_405_FALLBACK") || !strings.Contains(out, "internal/response/writer.go") {
		t.Fatalf("actual factory/responder proof missing: exit%d %s", code, out)
	}
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "proven legacy 405 factory")
	git(t, root, "tag", "-f", "trunk")
	mustWrite(t, helper, strings.Replace(writer, "w.WriteHeader(status)", "w.WriteHeader(200)", 1))
	code, out = goReviewProcessCLI(t, root)
	if code != 1 || !strings.Contains(out, "GO_ONLY POST /v1/review_factory") || !strings.Contains(out, "ERROR (TOUCHED) UNRESOLVED EXACT /v1/review_factory_methodless") {
		t.Fatalf("changed imported writer escaped gate: exit%d %s", code, out)
	}
	t.Run("header argument writes first", func(t *testing.T) {
		bad := strings.Replace(writer, `"application/json"`, `sideEffect(w)`, 1) + `
func sideEffect(w http.ResponseWriter) string { w.WriteHeader(200); return "application/json" }
`
		mustWrite(t, helper, bad)
		code, out := goReviewProcessCLI(t, root)
		if code != 1 || !strings.Contains(out, "GO_ONLY POST /v1/review_factory") {
			t.Fatalf("earlier implicit status write escaped proof: exit%d %s", code, out)
		}
	})
	t.Run("status reassigned", func(t *testing.T) {
		mustWrite(t, helper, strings.Replace(writer, "w.WriteHeader(status)", "status = 200; w.WriteHeader(status)", 1))
		code, out := goReviewProcessCLI(t, root)
		if code != 1 || !strings.Contains(out, "GO_ONLY POST /v1/review_factory") {
			t.Fatalf("reassigned status escaped proof: exit%d %s", code, out)
		}
	})
}

func TestGoReviewMethodValueAliasCLI(t *testing.T) {
	for _, body := range []string{
		`h := mux.HandleFunc; alias := h; alias("GET /v1/review_alias", handler)`,
		`h := mux.HandleFunc; bindReview(h, "GET /v1/review_alias", handler)`,
		`h := mux.Handle; h("/v1/review_alias", http.NotFoundHandler())`,
		`child := http.NewServeMux(); receiver := child; h := receiver.HandleFunc; h("GET /v1/review_alias", handler)`,
	} {
		t.Run(body, func(t *testing.T) {
			root := fixture(t, "aligned")
			p := filepath.Join(root, "internal/httpapi/routes.go")
			mustWrite(t, p, readText(t, p)+"\nfunc registerReview(mux *http.ServeMux) { "+body+" }\n"+`
func bindReview(bind func(string, http.HandlerFunc), path string, handler http.HandlerFunc) { bind(path, handler) }
`)
			code, out := goReviewProcessCLI(t, root)
			if code != 1 || !strings.Contains(out, "ERROR (TOUCHED) UNRESOLVED") || !strings.Contains(out, "/v1/review_alias") {
				t.Fatalf("method value registration escaped gate: exit%d %s", code, out)
			}
		})
	}
}

func TestGoReview405ProofCLI(t *testing.T) {
	root := fixture(t, "aligned")
	p := filepath.Join(root, "internal/httpapi/routes.go")
	s := readText(t, p) + `
func registerReview(mux *http.ServeMux) {
 mux.HandleFunc("POST /v1/review_numeric", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(405) })
 mux.HandleFunc("PUT /v1/review_symbolic", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusMethodNotAllowed) })
}
`
	mustWrite(t, p, s)
	code, out := goReviewProcessCLI(t, root)
	if code != 0 || !strings.Contains(out, "GO_405_FALLBACK POST /v1/review_numeric") || !strings.Contains(out, "GO_405_FALLBACK PUT /v1/review_symbolic") {
		t.Fatalf("actual 405 bodies were not preserved: exit%d %s", code, out)
	}
	mustWrite(t, p, strings.Replace(s, `w.WriteHeader(http.StatusMethodNotAllowed)`, `other.WriteHeader(http.StatusMethodNotAllowed)`, 1))
	code, out = goReviewProcessCLI(t, root)
	if code != 1 || !strings.Contains(out, "GO_ONLY PUT /v1/review_symbolic") {
		t.Fatalf("unrelated writer was mistaken for response fallback: exit%d %s", code, out)
	}
}

func TestGoReviewColonParameterCLI(t *testing.T) {
	root := fixture(t, "aligned")
	p := filepath.Join(root, "internal/httpapi/routes.go")
	mustWrite(t, p, readText(t, p)+`
func registerReview(mux *http.ServeMux) {
 mux.HandleFunc("GET /v1/review/:id", handler)
 mux.HandleFunc("GET /v1/review/b:c", handler)
 mux.HandleFunc("GET /v1/review/:id:tail", handler)
}
`)
	mustWrite(t, filepath.Join(root, "contracts/review.md"), "`GET /v1/review/{key}`\n`GET /v1/review/b:c`\n`GET /v1/review/:id:tail`\n")
	code, out := goReviewProcessCLI(t, root)
	if code != 0 || !strings.Contains(out, "SUMMARY routes go=4 contract=4 bff=1") {
		t.Fatalf("whole-segment colon normalization disagreed: exit%d %s", code, out)
	}
}

func TestGoReviewLiteralColonCLI(t *testing.T) {
	root := fixture(t, "aligned")
	p := filepath.Join(root, "internal/httpapi/routes.go")
	mustWrite(t, p, readText(t, p)+`
func registerReview(mux *http.ServeMux) { mux.HandleFunc("GET /v1/a/b:c", handler) }
`)
	mustWrite(t, filepath.Join(root, "contracts/review.md"), "`GET /v1/a/b:d`\n")
	code, out := goReviewProcessCLI(t, root)
	if code != 1 || !strings.Contains(out, "GO_ONLY GET /v1/a/b:c") || !strings.Contains(out, "CONTRACT_ONLY GET /v1/a/b:d") {
		t.Fatalf("literal colon paths collapsed: exit%d %s", code, out)
	}
}
