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
