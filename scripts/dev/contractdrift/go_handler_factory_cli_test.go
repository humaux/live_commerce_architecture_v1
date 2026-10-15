// Purpose: gate returned-handler provenance through the built contractdrift CLI.
// Depends on: existing two-table source fixtures and real Git/CLI helpers.
// Used by: CI for BUYER-ROUTE-TABLE round 3; these fixtures never serve HTTP.
// Invariant: every returned handler must be proven; one opaque alternative poisons the factory.

package main

import (
	"path/filepath"
	"strings"
	"testing"
)

const handlerFactoryCLISource = `package main
import (
 "net/http"
 "example/internal/router"
 "example/internal/feed"
 _ "example/internal/httpapi"
)
func main() { _, _ = buildTop(nil, false) }
func buildTop(fallback http.Handler, disabled bool) (http.Handler, error) {
 h, err := buildComposite(fallback, disabled)
 return h, err
}
func buildComposite(fallback http.Handler, disabled bool) (http.Handler, error) {
 if disabled { return fallback, nil }
 h := router.New()
 return mountFeed(h, withHeader(feed.New())), nil
}
func withHeader(next http.Handler) http.Handler {
 return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  w.Header().Set("X-Factory", "table")
  next.ServeHTTP(w,r)
 })
}
func mountFeed(next, child http.Handler) http.Handler {
 return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  if !feed.Routes.MatchesPath(r.URL.Path) { next.ServeHTTP(w,r); return }
  child.ServeHTTP(w,r)
 })
}
`

func handlerFactoryCLIFixture(t *testing.T, source string) string {
	t.Helper()
	root, _ := routeTableCLIFixture(t)
	trunkFactoryWrite(t, root, "internal/feed/feed.go", strings.Replace(exactTableCLISource, "package router", "package feed", 1))
	trunkFactoryWrite(t, root, "contracts/feed.md", "# Feed\n`GET /v1/feed/meta.csv`\n`HEAD /v1/feed/meta.csv`\n")
	trunkFactoryWrite(t, root, "cmd/api/main.go", source)
	return root
}

func TestGoHandlerFactoryTablesResolveCLI(t *testing.T) {
	root := handlerFactoryCLIFixture(t, handlerFactoryCLISource)
	code, out := goReviewProcessCLI(t, root)
	// RED if only the round-3 scanner change is reverted: both buildTop and
	// buildComposite are opaque, so exit=0 and no-UNRESOLVED assertions fail.
	if code != 0 || !strings.Contains(out, "SUMMARY routes go=5 contract=5 bff=1; errors=0") || strings.Contains(out, "UNRESOLVED") {
		t.Fatalf("two-table factory unresolved: exit%d %s", code, out)
	}
	for _, name := range []string{"buildTop", "buildComposite"} {
		if !strings.Contains(out, "handler factory "+name+" resolved from returned handler provenance") {
			t.Errorf("missing factory proof for %s: %s", name, out)
		}
	}
}

func TestGoHandlerFactoryOpaqueReturnsStayUnresolvedCLI(t *testing.T) {
	cases := []struct{ name, old, replacement string }{
		{"inline path switch", "if disabled { return fallback, nil }", `if disabled { return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
 switch r.URL.Path { case "/v1/hidden": w.WriteHeader(204); default: w.WriteHeader(404) }
 }), nil }`},
		{"unmarked package", "if disabled { return fallback, nil }", "if disabled { return unknown.New(), nil }"},
		{"reassigned handler", "h := router.New()", "h := router.New(); if disabled { h = unknown.New() }"},
		{"shadowed handler", "h := router.New()", "h := router.New(); if disabled { h := unknown.New(); return h, nil }"},
		{"mount extra path", "child.ServeHTTP(w,r)", `if r.URL.Path == "/v1/hidden" { child.ServeHTTP(w,r); return }; child.ServeHTTP(w,r)`},
		{"wrapper hidden helper", `w.Header().Set("X-Factory", "table")`, `if hiddenPath(r) { w.WriteHeader(204); return }`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := strings.Replace(handlerFactoryCLISource, tc.old, tc.replacement, 1)
			if strings.Contains(source, "unknown.New()") {
				source = strings.Replace(source, `"net/http"`, "\"net/http\"\n \"example/internal/unknown\"", 1)
			}
			source += "\nfunc hiddenPath(r *http.Request) bool { return r.URL.Path == \"/v1/hidden\" }\n"
			root := handlerFactoryCLIFixture(t, source)
			trunkFactoryWrite(t, root, "internal/unknown/unknown.go", `package unknown
import "net/http"
func New() http.Handler { return http.NotFoundHandler() }
`)
			code, out := goReviewProcessCLI(t, root)
			if code != 1 || !strings.Contains(out, "opaque Go handler dispatch buildComposite;") ||
				!strings.Contains(out, "opaque Go handler dispatch buildTop;") {
				t.Fatalf("opaque returned alternative hidden: exit%d %s", code, out)
			}
		})
	}
}

func TestGoHandlerFactoryUnresolvedSpansRetainedCLI(t *testing.T) {
	source := strings.Replace(handlerFactoryCLISource, "if disabled { return fallback, nil }", "if disabled { return unknown.New(), nil }", 1)
	source = strings.Replace(source, `"net/http"`, "\"net/http\"\n \"example/internal/unknown\"", 1)
	root := handlerFactoryCLIFixture(t, source)
	trunkFactoryWrite(t, root, "internal/unknown/unknown.go", `package unknown
import "net/http"
func New() http.Handler { return http.NotFoundHandler() }
`)
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "legacy opaque factory")
	git(t, root, "tag", "-f", "trunk")
	// Only a non-route helper inside the factory changes, like PR70's payment
	// builder. The old whole-function dependency span must still make it TOUCHED.
	p := filepath.Join(root, "cmd/api/main.go")
	mustWrite(t, p, strings.Replace(source, "h := router.New()", "h := router.New() // dependency changed", 1))
	code, out := goReviewProcessCLI(t, root)
	if code != 1 || !strings.Contains(out, "ERROR (TOUCHED) UNRESOLVED") || !strings.Contains(out, "opaque Go handler dispatch buildComposite;") {
		t.Fatalf("opaque factory span lost: exit%d %s", code, out)
	}
}

// Reverting only the terminal-leaf fix incorrectly resolves both build factories:
// a valid table guard cannot supply dispatch provenance to an inline 204 leaf.
func TestGoHandlerFactoryInlineTerminalRejectedCLI(t *testing.T) {
	source := strings.Replace(handlerFactoryCLISource, "child.ServeHTTP(w,r)", `http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }).ServeHTTP(w,r)`, 1)
	root := handlerFactoryCLIFixture(t, source)
	code, out := goReviewProcessCLI(t, root)
	if code != 1 || !strings.Contains(out, "opaque Go handler dispatch buildComposite;") ||
		!strings.Contains(out, "opaque Go handler dispatch buildTop;") ||
		strings.Contains(out, "handler factory buildComposite resolved") || strings.Contains(out, "handler factory buildTop resolved") {
		t.Fatalf("inline terminal became table-covered: exit%d %s", code, out)
	}
}

// R5: removing the exception must break the positive; dropping a condition must
// break the corresponding unresolved assertion. Execution/mutations are CI-only.
func TestGoHandlerFactoryRefusalLeavesCLI(t *testing.T) {
	cases := []struct {
		name, leaf, before      string
		only, outside, resolved bool
	}{
		{name: "mountFeed", leaf: `httperror.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { httperror.Write(w, http.StatusUnauthorized, "unauthorized") })).ServeHTTP(w,r)`, resolved: true},
		{name: "http Error literal return", leaf: `http.HandlerFunc(func(w http.ResponseWriter, unused *http.Request) { http.Error(w, "unavailable", 503); return }).ServeHTTP(w,r)`, resolved: true},
		{name: "constant expression", leaf: `http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "denied", 600-199) }).ServeHTTP(w,r)`, resolved: true},
		{name: "len is outside ruling", leaf: `http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "denied", 401 + 0*len([1]*http.Request{nil})) }).ServeHTTP(w,r)`},
		{name: "constant expression below 400", leaf: `http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "ok", 600-400) }).ServeHTTP(w,r)`},
		{name: "shadowed http", before: `http := pretendHTTP{}`, leaf: `http.HandlerFunc(func(w nethttp.ResponseWriter, _ *nethttp.Request) { http.Error(w, "denied", 401) }).ServeHTTP(w,r)`},
		{name: "shadowed httperror", before: `httperror := struct{ Write func(http.ResponseWriter,int,string) }{httperror.Write}`, leaf: `http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { httperror.Write(w, 401, "denied") }).ServeHTTP(w,r)`},
		{name: "unlisted wrapper", leaf: `unlistedWrapper(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "denied", 401) })).ServeHTTP(w,r)`},
		{name: "2xx error writer", leaf: `http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "ok", 204) }).ServeHTTP(w,r)`},
		{name: "3xx error writer", leaf: `http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "redirect", 302) }).ServeHTTP(w,r)`},
		{name: "refusal only", leaf: `http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "denied", 401) }).ServeHTTP(w,r)`, only: true},
		{name: "variable status", before: `status := http.StatusUnauthorized; _ = status`, leaf: `http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "denied", status) }).ServeHTTP(w,r)`},
		{name: "request read", leaf: `http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, r.URL.Path, 401) }).ServeHTTP(w,r)`},
		{name: "request reference constant message", leaf: `http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "denied", 401 + 0*len([1]*http.Request{r})) }).ServeHTTP(w,r)`},
		{name: "second statement", leaf: `http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "denied", 401); w.WriteHeader(500) }).ServeHTTP(w,r)`},
		{name: "dynamic message", before: `message := "denied"; _ = message`, leaf: `http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { http.Error(w, message, 401) }).ServeHTTP(w,r)`},
		{name: "unrecognised writer", leaf: `http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { pretendError(w, "denied", 401) }).ServeHTTP(w,r)`},
		{name: "outside table guard", leaf: `http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "denied", 401) }).ServeHTTP(w,r)`, outside: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			replacement := tc.before + "\nif r.Header.Get(\"X-Key\") == \"\" { " + tc.leaf + "; return }; child.ServeHTTP(w,r)"
			if tc.only {
				replacement = tc.leaf
			} // fallback outside the table branch is insufficient
			source := strings.Replace(handlerFactoryCLISource, "child.ServeHTTP(w,r)", replacement, 1)
			if tc.outside {
				source = strings.Replace(handlerFactoryCLISource, "return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {\n  if !feed", "return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {\n  if r.Header.Get(\"X-Key\") == \"\" { "+tc.leaf+"; return }; if !feed", 1)
			}
			source = strings.Replace(source, `"net/http"`, "\"net/http\"\n \"example/internal/httperror\"", 1)
			source += "\nfunc pretendError(w http.ResponseWriter, message string, status int) { http.Error(w,message,status) }\n"
			source = strings.Replace(source, `"net/http"`, `"net/http"; nethttp "net/http"`, 1)
			source += `
type pretendHTTP struct{}
func (pretendHTTP) HandlerFunc(fn func(nethttp.ResponseWriter,*nethttp.Request)) nethttp.Handler { return nethttp.HandlerFunc(fn) }
func (pretendHTTP) Error(w nethttp.ResponseWriter, message string, status int) { nethttp.Error(w,message,status) }
func unlistedWrapper(next http.Handler) http.Handler { return next }
`
			root := handlerFactoryCLIFixture(t, source)
			trunkFactoryWrite(t, root, "internal/httperror/error.go", `package httperror
import "net/http"
func Middleware(next http.Handler) http.Handler { return next }
func Write(w http.ResponseWriter, status int, message string) { http.Error(w,message,status) }
`)
			code, out := goReviewProcessCLI(t, root)
			if tc.resolved {
				if code != 0 || !strings.Contains(out, "SUMMARY routes go=5 contract=5 bff=1; errors=0") || strings.Contains(out, "UNRESOLVED") {
					t.Fatalf("refusal poisoned table branch: exit%d %s", code, out)
				}
				for _, name := range []string{"buildComposite", "buildTop"} {
					if !strings.Contains(out, "handler factory "+name+" resolved") {
						t.Errorf("missing refusal factory proof %s: %s", name, out)
					}
				}
			} else if code != 1 || !strings.Contains(out, "opaque Go handler dispatch buildComposite;") || !strings.Contains(out, "opaque Go handler dispatch buildTop;") || strings.Contains(out, "handler factory buildComposite resolved") || strings.Contains(out, "handler factory buildTop resolved") {
				t.Fatalf("invalid refusal became table-covered: exit%d %s", code, out)
			}
		})
	}
}
