// Purpose: gate exact split mount/handler tables through the real contractdrift CLI.
// Depends on: real Git/CLI helpers and aligned fixtures; no HTTP server or test-only parser.
// Used by: CI only for BUYER-ROUTE-TABLE round 2.
// Invariants: no marker, partial consumption, bypassed guard and unrelated mounts stay unresolved.

package main

import (
	"path/filepath"
	"strings"
	"testing"
)

const exactTableCLISource = `package router
import "net/http"
type entry struct { Method, Path string }
type Table struct{}; var Routes = Table{}
//contractdrift:routes /v1/feed/meta.csv matchesPath allowedMethod exact mount=Routes
var rows = []entry{
 {Method: http.MethodGet, Path: "/v1/feed/meta.csv"},
 {Method: http.MethodHead, Path: "/v1/feed/meta.csv"},
}
func (Table) MatchesPath(path string) bool { return matchesPath(path) }
func matchesPath(path string) bool {
 for _, entry := range rows {
  if entry.Path == path { return true }
 }
 return false
}
func allowedMethod(method string) bool {
 for _, entry := range rows {
  if entry.Method == method { return true }
 }
 return false
}
func New() http.Handler {
 return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  if !allowedMethod(r.Method) { w.WriteHeader(405); return }
  w.WriteHeader(200)
  if r.Method == http.MethodGet { _, _ = w.Write([]byte("feed")) }
 })
}
`
const exactTableMountSource = `package main
import (
 "net/http"
 "example/internal/router"
 _ "example/internal/httpapi"
)
func main() { _ = mountFeed(http.NotFoundHandler(), router.New()) }
func mountFeed(next, feed http.Handler) http.Handler {
 return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request) {
  if !router.Routes.MatchesPath(r.URL.Path) { next.ServeHTTP(w,r); return }
  feed.ServeHTTP(w,r)
 })
}
`

func exactTableCLIFixture(t *testing.T) (string, string) {
	t.Helper()
	root := fixture(t, "aligned")
	trunkFactoryWrite(t, root, "go.mod", "module example\n")
	p := trunkFactoryWrite(t, root, "internal/router/feed.go", exactTableCLISource)
	trunkFactoryWrite(t, root, "cmd/api/main.go", exactTableMountSource)
	// A single code span includes METHOD + full path, matching the canonical index.
	trunkFactoryWrite(t, root, "contracts/feed.md", "# Feed\n`GET /v1/feed/meta.csv`\n`HEAD /v1/feed/meta.csv`\n")
	return root, p
}

func TestGoRouteTableExactSplitResolvesCLI(t *testing.T) {
	root, _ := exactTableCLIFixture(t)
	code, out := goReviewProcessCLI(t, root)
	// RED with only the round-1 recognizer: the fifth marker field is invalid,
	// both feed methods stay contract-only and the exact mount/handler stay opaque.
	if code != 0 || !strings.Contains(out, "SUMMARY routes go=3 contract=3 bff=1") || strings.Contains(out, "UNRESOLVED") {
		t.Fatalf("exact table unresolved: exit%d %s", code, out)
	}
	for _, want := range []string{"GO_ROUTE_TABLE", "GET /v1/feed/meta.csv", "HEAD /v1/feed/meta.csv", "internal/router/feed.go:7", "internal/router/feed.go:8"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q: %s", want, out)
		}
	}
}

func TestGoRouteTableExactInvalidStaysOpaqueCLI(t *testing.T) {
	cases := []struct{ name, old, replacement string }{
		{"no marker", "//contractdrift:routes /v1/feed/meta.csv matchesPath allowedMethod exact mount=Routes\n", ""},
		{"unserved method tail", "range rows {", "range rows[:len(rows)-1] {"},
		{"different path", "Path: \"/v1/feed/meta.csv\"},", "Path: \"/v1/feed/other.csv\"},"},
		{"bypassed method guard", "if !allowedMethod(r.Method)", "if r.Method != http.MethodPost && !allowedMethod(r.Method)"},
		{"other path dispatcher", "w.WriteHeader(200)", "if r.URL.Path == \"/v1/feed/other\" { w.WriteHeader(204); return }; w.WriteHeader(200)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, p := exactTableCLIFixture(t)
			mustWrite(t, p, strings.Replace(exactTableCLISource, tc.old, tc.replacement, 1))
			code, out := goReviewProcessCLI(t, root)
			if code != 1 || !strings.Contains(out, "UNRESOLVED") || strings.Contains(out, "GO_ROUTE_TABLE") {
				t.Fatalf("invalid exact table became known: exit%d %s", code, out)
			}
		})
	}
}

func TestGoRouteTableExactDoesNotResolveUnrelatedMountCLI(t *testing.T) {
	root, _ := exactTableCLIFixture(t)
	mustWrite(t, filepath.Join(root, "cmd/api/main.go"), exactTableMountSource+`
func mountOther(next, child http.Handler) http.Handler {
 return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request) {
  if r.URL.Path == "/v1/feed/unlisted" { child.ServeHTTP(w,r); return }
  next.ServeHTTP(w,r)
 })
}
`)
	code, out := goReviewProcessCLI(t, root)
	if code != 1 || !strings.Contains(out, "UNRESOLVED EXACT /v1/feed/unlisted") {
		t.Fatalf("unrelated mount hidden by exact table: exit%d %s", code, out)
	}
}

func TestGoRouteTableExactRequiresOwningMountCLI(t *testing.T) {
	root, _ := exactTableCLIFixture(t)
	mustWrite(t, filepath.Join(root, "cmd/api/main.go"), strings.Replace(exactTableMountSource,
		"!router.Routes.MatchesPath(r.URL.Path)", "r.URL.Path != \"/v1/feed/meta.csv\"", 1))
	code, out := goReviewProcessCLI(t, root)
	if code != 1 || !strings.Contains(out, "UNRESOLVED EXACT /v1/feed/meta.csv") || strings.Contains(out, "GO_ROUTE_TABLE") {
		t.Fatalf("unconsumed exact mount became known: exit%d %s", code, out)
	}
}
