// Purpose: exercise executable-table resolution and conservative rejection through the real CLI.
// Depends on: aligned source fixtures, real Git, goReviewProcessCLI and the stdlib parser.
// Used by: CI contract-drift gates; source fixtures are never run as HTTP servers.
// Invariants: unserved table tails and custom dispatch without a table cannot become KNOWN.

package main

import (
	"path/filepath"
	"strings"
	"testing"
)

const routeTableCLISource = `package router
import ("net/http"; "strings")
type entry struct { Method, Path, kind string }
type route struct { kind string }
type Table struct{}; var Routes = Table{}
//contractdrift:routes /v1/table match allowed mount=Routes
var rows = []entry{
 {Method: http.MethodGet, Path: "/v1/table/one", kind: "one"},
 {Method: http.MethodPost, Path: "/v1/table/{id}", kind: "item"},
}
func (Table) HasPrefix(path string) bool { return strings.HasPrefix(path,"/v1/table") }
func match(path string) route {
 for _, e := range rows {
  if selected, ok := pattern(e,path); ok { return selected }
 }
 return route{}
}
func allowed(kind, method string) bool {
 for _, e := range rows {
  if e.kind == kind && e.Method == method { return true }
 }
 return false
}
func pattern(e entry, path string) (route,bool) {
 if e.Path == path { return route{kind:e.kind},true }
 if strings.HasSuffix(e.Path,"/{id}") {
  id := strings.TrimPrefix(path, strings.TrimSuffix(e.Path,"{id}"))
  if id != path && id != "" && !strings.Contains(id,"/") { return route{kind:e.kind},true }
 }
 return route{},false
}
type handler struct{}
func New() http.Handler { return &handler{} }
func (*handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
 selected := match(r.URL.Path)
 if selected.kind == "" { http.NotFound(w,r); return }
 if !allowed(selected.kind,r.Method) { w.WriteHeader(405); return }
 w.WriteHeader(204)
}
`

const routeTableMountSource = `package main
import (
 "net/http"
 "path"
 "example/internal/router"
 _ "example/internal/httpapi"
)
func main() { _ = mountBuyer(http.NotFoundHandler(),router.New()) }
func mountBuyer(next, child http.Handler) http.Handler {
 return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request) {
  if router.Routes.HasPrefix(r.URL.Path) || router.Routes.HasPrefix(path.Clean(r.URL.Path)) { child.ServeHTTP(w,r); return }
  next.ServeHTTP(w,r)
 })
}
`

func routeTableCLIFixture(t *testing.T) (string, string) {
	t.Helper()
	root := fixture(t, "aligned")
	trunkFactoryWrite(t, root, "go.mod", "module example\n")
	p := trunkFactoryWrite(t, root, "internal/router/routes.go", routeTableCLISource)
	trunkFactoryWrite(t, root, "cmd/api/main.go", routeTableMountSource)
	trunkFactoryWrite(t, root, "contracts/route-table-openapi.json", `{"openapi":"3.1.0","paths":{
 "/v1/table/one":{"get":{}}, "/v1/table/{id}":{"post":{}}
}}`)
	return root, p
}

func TestGoRouteTableResolvesCLI(t *testing.T) {
	root, _ := routeTableCLIFixture(t)
	code, out := goReviewProcessCLI(t, root)
	// RED without scanner support: go remains 1, CONTRACT_ONLY/UNRESOLVED replace
	// these resolved rows and the GO_ROUTE_TABLE provenance note disappears.
	if code != 0 || !strings.Contains(out, "SUMMARY routes go=3 contract=3 bff=1") || strings.Contains(out, "UNRESOLVED") {
		t.Fatalf("consumed table/owning mount unresolved: exit%d %s", code, out)
	}
	for _, want := range []string{"GO_NOT_IN_BFF GET /v1/table/one", "GO_NOT_IN_BFF POST /v1/table/{}", "GO_ROUTE_TABLE", "internal/router/routes.go:9"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q: %s", want, out)
		}
	}
}

func TestGoRouteTableUnservedTailRejectedCLI(t *testing.T) {
	root, p := routeTableCLIFixture(t)
	// The table still advertises the second entry, but the router never visits it.
	// Full-range consumption is statically enforceable; arbitrary helper behavior
	// is explicitly left to the buyer transport's differential runtime gate.
	mustWrite(t, p, strings.Replace(routeTableCLISource, "range rows {", "range rows[:len(rows)-1] {", 1))
	code, out := goReviewProcessCLI(t, root)
	if code != 1 || !strings.Contains(out, "UNRESOLVED") || !strings.Contains(out, "invalid or incompletely consumed contractdrift route table") || strings.Contains(out, "GO_ROUTE_TABLE") {
		t.Fatalf("unserved table entry became known: exit%d %s", code, out)
	}
}

func TestGoCustomDispatchWithoutTableRemainsOpaqueCLI(t *testing.T) {
	root, p := routeTableCLIFixture(t)
	mustWrite(t, p, strings.Replace(routeTableCLISource, "//contractdrift:routes /v1/table match allowed mount=Routes\n", "", 1))
	code, out := goReviewProcessCLI(t, root)
	if code != 1 || !strings.Contains(out, "UNRESOLVED") || !strings.Contains(out, "SUMMARY routes go=1") || strings.Contains(out, "GO_ROUTE_TABLE") {
		t.Fatalf("unmarked custom router ceased being opaque: exit%d %s", code, out)
	}
}

func TestGoRouteTableDoesNotResolveUnrelatedMountCLI(t *testing.T) {
	root, _ := routeTableCLIFixture(t)
	p := filepath.Join(root, "cmd/api/main.go")
	mustWrite(t, p, routeTableMountSource+`
func mountOther(next, child http.Handler) http.Handler {
 return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request) {
  if r.URL.Path == "/v1/table/unlisted" { child.ServeHTTP(w,r); return }
  next.ServeHTTP(w,r)
 })
}
`)
	code, out := goReviewProcessCLI(t, root)
	if code != 1 || !strings.Contains(out, "UNRESOLVED EXACT /v1/table/unlisted") {
		t.Fatalf("table concealed unrelated opaque mount: exit%d %s", code, out)
	}
}

// Reverting only the round-4 path proof makes each negative case lose its
// no-GO_ROUTE_TABLE assertion: both consumers are still reachable.
func TestGoRouteTableExtraPathDispatchRejectedCLI(t *testing.T) {
	cases := []struct{ name, branch string }{
		{"path equality", `if r.URL.Path == "/v1/table/debug" { w.WriteHeader(204); return }`},
		{"raw path equality", `if r.URL.RawPath == "/v1/table/debug" { w.WriteHeader(204); return }`},
		{"request URI equality", `if r.RequestURI == "/v1/table/debug" { w.WriteHeader(204); return }`},
		{"constant equality", `if r.URL.Path == debugPath { w.WriteHeader(204); return }`},
		{"path switch", `switch r.URL.Path { case "/v1/table/debug": w.WriteHeader(204); return }`},
		{"prefix", `if strings.HasPrefix(r.URL.Path, "/v1/table/debug") { w.WriteHeader(204); return }`},
		{"suffix", `if strings.HasSuffix(r.URL.Path, "/debug") { w.WriteHeader(204); return }`},
		{"map lookup", `if map[string]bool{"/v1/table/debug":true}[r.URL.Path] { w.WriteHeader(204); return }`},
		{"transformed path", `if strings.TrimPrefix(r.URL.Path, "/debug") == "/v1/table/one" { w.WriteHeader(204); return }`},
		{"reached helper", `if debugRequest(r) { w.WriteHeader(204); return }`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, p := routeTableCLIFixture(t)
			source := strings.Replace(routeTableCLISource, "selected := match(r.URL.Path)", tc.branch+"\n selected := match(r.URL.Path)", 1)
			mustWrite(t, p, source+`
const debugPath = "/v1/table/debug"
func debugRequest(r *http.Request) bool { return r.URL.Path == debugPath }
`)
			code, out := goReviewProcessCLI(t, root)
			if code != 1 || !strings.Contains(out, "UNRESOLVED") || strings.Contains(out, "GO_ROUTE_TABLE") {
				t.Fatalf("extra path dispatch became table-covered: exit%d %s", code, out)
			}
		})
	}
}

func TestGoRouteTableListedPathPolicyAcceptedCLI(t *testing.T) {
	root, p := routeTableCLIFixture(t)
	source := strings.Replace(routeTableCLISource, "selected := match(r.URL.Path)", `if r.URL.EscapedPath() != r.URL.Path { http.NotFound(w,r); return }
 if r.URL.Path == listedPath { w.Header().Set("X-Listed", "yes") }
 selected := match(r.URL.Path)`, 1)
	mustWrite(t, p, source+`
const listedPath = "/v1/table/one"
`)
	code, out := goReviewProcessCLI(t, root)
	if code != 0 || strings.Contains(out, "UNRESOLVED") || !strings.Contains(out, "GO_ROUTE_TABLE") {
		t.Fatalf("listed path policy unresolved: exit%d %s", code, out)
	}
}

func TestGoRouteTableMutationRejectedCLI(t *testing.T) {
	for _, exact := range []bool{false, true} {
		t.Run(map[bool]string{false: "prefix", true: "exact"}[exact], func(t *testing.T) {
			source := routeTableCLISource
			root, p := routeTableCLIFixture(t)
			if exact {
				root, p = exactTableCLIFixture(t)
				source = exactTableCLISource
			}
			mustWrite(t, p, source+`
func mutate() { rows[0].Method = "DELETE" }
`)
			code, out := goReviewProcessCLI(t, root)
			if code != 1 || !strings.Contains(out, "invalid or incompletely consumed contractdrift route table") || strings.Contains(out, "GO_ROUTE_TABLE") {
				t.Fatalf("owning-package table write became known: exit%d %s", code, out)
			}
		})
	}
}

// This source intentionally uses the old mutable exported slice shape. It is
// rejected even when the write is in the importing package, outside the owner.
func TestGoRouteTableExportedMutationRejectedCLI(t *testing.T) {
	root, p := routeTableCLIFixture(t)
	source := strings.Replace(routeTableCLISource, "type Table struct{}; var Routes = Table{}", "type Table []entry", 1)
	source = strings.Replace(source, " mount=Routes", "", 1)
	source = strings.ReplaceAll(source, "rows", "Routes")
	source = strings.Replace(source, "var Routes = []entry{", "var Routes = Table{", 1)
	mustWrite(t, p, source)
	mustWrite(t, filepath.Join(root, "cmd/api/main.go"), routeTableMountSource+`
func mutateImported() { router.Routes[0].Method = "DELETE" }
`)
	code, out := goReviewProcessCLI(t, root)
	// RED if private rows/read-only facade recognition is reverted: this exported
	// table again resolves despite the importing package's method write.
	if code != 1 || !strings.Contains(out, "UNRESOLVED") || strings.Contains(out, "GO_ROUTE_TABLE") {
		t.Fatalf("imported mutable table became known: exit%d %s", code, out)
	}
}
