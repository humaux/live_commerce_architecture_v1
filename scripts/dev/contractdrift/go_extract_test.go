// Purpose: exercise actual Go fixture syntax and independent expected inventories.
// Depends on: stdlib testing/os; never starts HTTP handlers.
// Used by: contractdrift test gate; changing a constant or wrapper must retain provenance.
package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func goFixture(t *testing.T, sources map[string]string) inventory {
	t.Helper()
	root := t.TempDir()
	for file, text := range sources {
		p := filepath.Join(root, file)
		if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	inv, err := scanGo(root)
	if err != nil {
		t.Fatal(err)
	}
	return inv
}
func goRouteKeys(inv inventory) map[string]route {
	out := map[string]route{}
	for _, r := range inv.Routes {
		out[r.Method+" "+r.Path] = r
	}
	return out
}

func TestGoUnknownRangeTailRetainsKnownRoutesAndProvenance(t *testing.T) {
	for _, setup := range []string{
		`paths := append([]string{"/known"}, dynamicPaths()...)`,
		`paths := []string{}; if enabled { paths = []string{"/known"} } else { paths = dynamicPaths() }`,
	} {
		inv := goFixture(t, map[string]string{"internal/httpapi/routes.go": `package httpapi
import "net/http"
func register(mux *http.ServeMux, enabled bool) {
 ` + setup + `
 for _, p := range paths { mux.HandleFunc("GET "+p, handler) }
}
func dynamicPaths() []string { return []string{"/unknown"} }
func handler(w http.ResponseWriter, r *http.Request) {}
`})
		if _, ok := goRouteKeys(inv)["GET /known"]; !ok {
			t.Fatalf("lost known alternative: %+v", inv)
		}
		if len(inv.Unresolved) != 1 {
			t.Fatalf("lost unknown alternative: %+v", inv)
		}
		lines := map[int]bool{}
		for _, ref := range inv.Unresolved[0].Locations {
			lines[ref.Line] = true
		}
		for _, line := range []int{4, 5, 7} {
			if !lines[line] {
				t.Errorf("unknown must retain assignment, registration and dynamic producer line %d: %+v", line, inv.Unresolved[0])
			}
		}
	}
}

func TestGoLexicalAliasesAndReassignment(t *testing.T) {
	inv := goFixture(t, map[string]string{"internal/httpapi/routes.go": `package httpapi
import h "net/http"
const rootPrefix = "/v1/admin/stores/{store_id}"
func register(mux *h.ServeMux) {
 base := rootPrefix
 alias := base+"/widgets/{widget_id}"
 mux.HandleFunc(h.MethodGet+" "+alias, handler)
 { base := "/inner"; mux.HandleFunc("POST "+base, handler) }
 mux.HandleFunc("DELETE "+base+"/widgets/{widget_id}", handler)
 alias = pattern()
 mux.HandleFunc("GET "+alias, handler)
}
`})
	keys := goRouteKeys(inv)
	for _, want := range []string{"GET /v1/admin/stores/{}/widgets/{}", "POST /inner", "DELETE /v1/admin/stores/{}/widgets/{}"} {
		if _, ok := keys[want]; !ok {
			t.Errorf("missing %s: %+v", want, keys)
		}
	}
	if len(inv.Routes) != 3 {
		t.Errorf("stale initializer or missing route: %+v", inv.Routes)
	}
	if len(inv.Unresolved) != 1 || inv.Unresolved[0].Locations[0].Line != 11 {
		t.Errorf("unknown must retain call location: %+v", inv.Unresolved)
	}
	refs := keys["GET /v1/admin/stores/{}/widgets/{}"].Locations
	lines := map[int]bool{}
	for _, ref := range refs {
		lines[ref.Line] = true
	}
	for _, line := range []int{3, 5, 6, 7} {
		if !lines[line] {
			t.Errorf("definition/call provenance missing line %d: %+v", line, refs)
		}
	}
}
func TestGoWrapperFiniteRangesAndConditionalAppend(t *testing.T) {
	inv := goFixture(t, map[string]string{"internal/httpapi/routes.go": `package httpapi
import "net/http"
func register(mux *http.ServeMux, enabled bool) {
 const base = "/v1/admin/stores/{store_id}"
 wrapper(mux, base+"/tools")
 type action struct { name string; handler any }
 actions := []action{{"cancel", nil}}
 if enabled { actions = append(actions, action{"retry", nil}, action{"query", nil}) }
 for _, a := range actions { mux.HandleFunc("POST "+base+"/ops/"+a.name, handler) }
 for _, m := range []string{http.MethodPost,http.MethodPut} { mux.HandleFunc(m+" "+base+"/results", studioRoute("",false,nil)) }
}
func wrapper(mux *http.ServeMux, base string) {
 for _, p := range []struct{slug string}{{"/a"},{"/b"}} { mux.HandleFunc("GET "+base+p.slug, handler) }
 mux.HandleFunc(base, studioRoute("",false,nil))
}
`})
	keys := goRouteKeys(inv)
	for _, suffix := range []string{"cancel", "retry", "query"} {
		if _, ok := keys["POST /v1/admin/stores/{}/ops/"+suffix]; !ok {
			t.Errorf("missing conditional action %s: %+v", suffix, keys)
		}
	}
	for _, suffix := range []string{"a", "b"} {
		r, ok := keys["GET /v1/admin/stores/{}/tools/"+suffix]
		if !ok {
			t.Errorf("missing wrapper %s", suffix)
		}
		found := false
		for _, ref := range r.Locations {
			if ref.Line == 5 {
				found = true
			}
		}
		if !found {
			t.Errorf("missing wrapper caller: %+v", r)
		}
	}
	for _, method := range []string{"POST", "PUT"} {
		if _, ok := keys[method+" /v1/admin/stores/{}/results"]; !ok {
			t.Errorf("suffix-only constructor concealed %s business route: %+v", method, keys)
		}
	}
	if len(inv.Routes) != 7 || len(inv.Unresolved) != 1 || len(inv.Notes) != 1 {
		t.Errorf("unproved constructor must retain business and methodless coverage: %+v", inv)
	}
}
func TestGoProducerDiscoveryMountsAndOpaqueDispatch(t *testing.T) {
	inv := goFixture(t, map[string]string{
		"internal/identityhttp/routes.go": `package identityhttp
import "net/http"
func NewHandler(mux *http.ServeMux) { route(mux,"list");route(mux,"invite") }
func route(mux *http.ServeMux,path string) { mux.HandleFunc("POST /v1/identity/staff/"+path, handler) }
`,
		"cmd/api/main.go": `package main
import "net/http"
func main() { mux:=http.NewServeMux();mux.Handle("/v1/identity/", identity);mux.Handle("/",fallback) }
func mount(next, custom http.Handler) http.Handler { return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request) { if r.URL.Path=="/internal/custom" { custom.ServeHTTP(w,r);return };next.ServeHTTP(w,r) }) }
`,
		"internal/identityhttp/routes_test.go": `package identityhttp
func TestRoutes() { mux.HandleFunc("GET /fake",nil) }
`})
	keys := goRouteKeys(inv)
	if len(keys) != 2 {
		t.Errorf("mount must not prepend or enumerate test routes: %+v", keys)
	}
	if _, ok := keys["POST /v1/identity/staff/list"]; !ok {
		t.Errorf("missing identity binding: %+v", keys)
	}
	markers := map[string]finding{}
	for _, u := range inv.Unresolved {
		markers[u.Method+" "+u.Path] = u
	}
	for _, want := range []string{"EXACT /internal/custom", "MOUNT /v1/identity/", "MOUNT /"} {
		if u, ok := markers[want]; !ok || !strings.Contains(u.Detail, "opaque") {
			t.Errorf("opaque dispatch or real Handle mount absent: %s %+v", want, inv.Unresolved)
		}
	}
	if len(inv.Unresolved) > 0 && inv.Unresolved[0].Locations[0].End <= inv.Unresolved[0].Locations[0].Line {
		t.Log("single-line dispatch has naturally single-line span")
	}
}

func TestGoEmptyRangeMutationAndBranchUnion(t *testing.T) {
	inv := goFixture(t, map[string]string{"internal/httpapi/routes.go": `package httpapi
import "net/http"
func register(mux *http.ServeMux, flag bool, dynamic []string) {
 path:="/v1/a"
 path+="/b"
 mux.HandleFunc("GET "+path,handler)
 for _,suffix:=range []string{} {mux.HandleFunc("POST /empty/"+suffix,handler)}
 if flag {path="/v1/first"} else {path="/v1/second"}
 mux.HandleFunc("POST "+path,handler)
 for range dynamic {path=pattern()}
 mux.HandleFunc("DELETE "+path,handler)
}
`})
	keys := goRouteKeys(inv)
	for _, want := range []string{"GET /v1/a/b", "POST /v1/first", "POST /v1/second"} {
		if _, ok := keys[want]; !ok {
			t.Errorf("missing %s: %+v", want, keys)
		}
	}
	if len(inv.Unresolved) != 1 {
		t.Errorf("unknown range mutations must invalidate initializer: %+v", inv.Unresolved)
	}
	if _, ok := keys["DELETE /v1/a"]; ok {
		t.Errorf("stale route after loop mutation: %+v", keys)
	}
	for key := range keys {
		if strings.Contains(key, "/empty/") {
			t.Errorf("empty static range must not register: %s", key)
		}
	}
}

func TestGoAPIImportClosureExcludesWorkerBridge(t *testing.T) {
	inv := goFixture(t, map[string]string{
		"go.mod": "module example\n",
		"cmd/api/main.go": `package main
import "example/internal/httpapi"
func main(){httpapi.NewHandler()}
`,
		"internal/httpapi/routes.go": `package httpapi
func NewHandler(){mux.HandleFunc("GET /v1/a",handler)}
`,
		"internal/workerbridge/routes.go": `package workerbridge
func Handler(){mux.HandleFunc("POST /internal/worker",handler)}
`,
	})
	keys := goRouteKeys(inv)
	if len(keys) != 1 {
		t.Fatalf("API inventory imported worker-only bridge: %+v", keys)
	}
	if _, ok := keys["GET /v1/a"]; !ok {
		t.Errorf("lost actual producer: %+v", keys)
	}
}

func TestGoOpaqueDependencySpans(t *testing.T) {
	inv := goFixture(t, map[string]string{"internal/custom/routes.go": `package custom
import "net/http"
const prefix="/v1/custom"
func Handler() http.Handler {return http.HandlerFunc(serve)}
func serve(w http.ResponseWriter,r *http.Request) {
 if r.Method!=http.MethodPost || r.URL.Path!=prefix {return}
}
`})
	if len(inv.Unresolved) != 1 {
		t.Fatalf("opaque adapter lost underlying function: %+v", inv.Unresolved)
	}
	lines := map[int]bool{}
	for _, ref := range inv.Unresolved[0].Locations {
		lines[ref.Line] = true
	}
	for _, line := range []int{3, 4, 5} {
		if !lines[line] {
			t.Errorf("opaque dispatch omitted dependency line %d: %+v", line, inv.Unresolved)
		}
	}
}

func TestGoRepositoryInventory(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	if _, err := os.Stat(filepath.Join(root, "internal", "httpapi", "handler.go")); err != nil {
		t.Skip("real repository unavailable")
	}
	inv, err := scanGo(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("real inventory routes=%d unresolved=%d notes=%d", len(inv.Routes), len(inv.Unresolved), len(inv.Notes))
	coverage := map[string]finding{}
	for _, u := range inv.Unresolved {
		if u.Method == "PREFIX" || u.Method == "EXACT" {
			coverage[u.Method+" "+u.Path] = u
		}
	}
	for _, want := range []string{"PREFIX /v1/meta", "PREFIX /v1/stripe", "PREFIX /.well-known/lc-domain-check/", "EXACT /v1/platform/stripe/webhook", "EXACT /internal/tls-ask"} {
		if _, ok := coverage[want]; !ok {
			t.Errorf("missing actual cmd/api coverage guard %s", want)
		}
	}
	for _, pair := range [][2]string{{"PREFIX /v1/meta", "internal/integrations/meta/runtime.go"}, {"PREFIX /v1/stripe", "internal/payments/stripewebhook/handler.go"}, {"EXACT /internal/tls-ask", "internal/tlsask/tlsask.go"}} {
		found := false
		for _, ref := range coverage[pair[0]].Locations {
			if ref.File == pair[1] && ref.End > ref.Line {
				found = true
			}
		}
		if !found {
			t.Errorf("opaque mount %s lacks child dispatch span %s", pair[0], pair[1])
		}
	}

	// Expected rows come independently from AST declarations, not scanGo or the
	// scanner's table evaluator. Each must retain its row location AND the table
	// proof note, replacing the obsolete buyer opaque guard/span expectations.
	keys := goRouteKeys(inv)
	for _, table := range []struct {
		file, facade string
		count        int
	}{
		{"internal/buyerhttp/routes.go", "Routes", 46},
		{"internal/attribution/feed.go", "FeedRoutes", 2},
	} {
		expected := repositoryTableRows(t, root, table.file, table.facade)
		if len(expected) != table.count {
			t.Fatalf("%s rows=%d want%d", table.file, len(expected), table.count)
		}
		for key, line := range expected {
			actual, ok := keys[key]
			if !ok {
				t.Errorf("missing table row %s (%s:%d)", key, table.file, line)
				continue
			}
			rowRef, proof := false, false
			for _, ref := range actual.Locations {
				if ref.File == table.file && ref.Line == line {
					rowRef = true
				}
			}
			for _, note := range inv.Notes {
				if note.Kind == "GO_ROUTE_TABLE" {
					for _, ref := range note.Locations {
						if ref.File == table.file && ref.Line <= line && ref.End >= line {
							proof = true
						}
					}
				}
			}
			if !rowRef || !proof {
				t.Errorf("table row %s lacks row/proof provenance: row=%t proof=%t %+v", key, rowRef, proof, actual)
			}
		}
	}
	for _, u := range inv.Unresolved {
		if strings.HasPrefix(u.Path, "/v1/buyer") {
			t.Errorf("buyer remains unresolved: %+v", u)
		}
		for _, name := range []string{"buildBuyerHandler", "buildBuyerWithCVS", "mountFeed"} {
			if strings.Contains(u.Detail, "opaque Go handler dispatch "+name+";") {
				t.Errorf("buyer factory remains unresolved: %+v", u)
			}
			for _, ref := range u.Locations {
				if ref.File == "cmd/api/buyer.go" && repositoryFunctionContains(t, root, ref, name) {
					t.Errorf("unresolved span on buyer factory %s: %+v", name, u)
				}
			}
		}
	}
	for _, want := range []string{
		"POST /v1/admin/stores/{}/operations/{}/query",
		"POST /v1/admin/stores/{}/operations/{}/retry",
		"GET /v1/admin/stores/{}/reports/products",
		"GET /v1/admin/stores/{}/reports/channels.csv",
		"POST /v1/identity/staff/invite",
		"POST /v1/cvs/ecpay/status/{}",
		"GET /healthz",
	} {
		if _, ok := keys[want]; !ok {
			t.Errorf("actual constructor inventory omitted %s", want)
		}
	}
	for _, method := range []string{"POST", "PUT", "PATCH", "DELETE"} {
		if _, ok := keys[method+" /v1/admin/stores/{}/live-sessions/results"]; ok {
			t.Errorf("explicit 405 fallback appears as business method %s", method)
		}
	}
	if _, ok := keys["POST /internal/v1/comment-page"]; ok {
		t.Error("worker bridge was included on API mux")
	}
	for _, u := range inv.Unresolved {
		if strings.Contains(u.Detail, "registration pattern") {
			t.Errorf("actual finite mux registration still unresolved: %+v", u)
		}
	}
}

func TestGoOpaqueMountMarkersAndChildSpans(t *testing.T) {
	inv := goFixture(t, map[string]string{
		"go.mod": "module example\n",
		"cmd/api/main.go": `package main
import "net/http"
import "strings"
import "example/internal/custom"
func main(){custom.Handler()}
const exact="/v1/feed.csv"
func mountNamespace(next,child http.Handler) http.Handler {return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){if strings.HasPrefix(r.URL.Path,"/v1/custom"){child.ServeHTTP(w,r);return};next.ServeHTTP(w,r)})}
func mountFile(next,child http.Handler) http.Handler {return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){if r.URL.Path!=exact {next.ServeHTTP(w,r);return};child.ServeHTTP(w,r)})}
`,
		"internal/custom/routes.go": `package custom
import "net/http"
func Handler() http.Handler {return http.HandlerFunc(serve)}
func serve(w http.ResponseWriter,r *http.Request){if r.Method==http.MethodPost && r.URL.Path=="/v1/custom/a" {return}}
`,
	})
	markers := map[string]finding{}
	for _, u := range inv.Unresolved {
		if u.Method != "" {
			markers[u.Method+" "+u.Path] = u
		}
	}
	for _, want := range []string{"PREFIX /v1/custom", "EXACT /v1/feed.csv"} {
		u, ok := markers[want]
		if !ok {
			t.Errorf("missing actual mount guard %s: %+v", want, inv.Unresolved)
		}
		found := false
		for _, ref := range u.Locations {
			if ref.File == "internal/custom/routes.go" && ref.Line == 4 {
				found = true
			}
		}
		if !found {
			t.Errorf("mount lost child dispatcher span: %+v", u)
		}
	}
}

func TestGoUnsupportedClosuresAndSwitchAssignmentsStayVisible(t *testing.T) {
	inv := goFixture(t, map[string]string{"internal/httpapi/routes.go": `package httpapi
import "net/http"
func NewHandler(mux *http.ServeMux,mode string) {
 register:=func(){mux.HandleFunc("GET /hidden",handler)}
 register()
 path:="/old"
 switch mode {case "a":path="/first";default:path=pattern()}
 mux.HandleFunc("POST "+path,handler)
}
`})
	if len(inv.Unresolved) != 2 {
		t.Fatalf("unsupported registration scope/unknown switch branch must stay visible: %+v", inv.Unresolved)
	}
	keys := goRouteKeys(inv)
	if _, ok := keys["POST /first"]; !ok {
		t.Errorf("lost known switch alternative: %+v", keys)
	}
	if _, ok := keys["POST /old"]; ok {
		t.Errorf("switch kept stale initializer: %+v", keys)
	}
}

func TestGoImportChangesAndShadows(t *testing.T) {
	inv := goFixture(t, map[string]string{"internal/httpapi/routes.go": `package httpapi
import h "net/http"
func register(mux *h.ServeMux) {
 mux.HandleFunc(h.MethodGet+" /outer",handler)
 h:=struct{MethodGet string}{"POST"}
 mux.HandleFunc(h.MethodGet+" /inner",handler)
}
`})
	keys := goRouteKeys(inv)
	for _, want := range []string{"GET /outer", "POST /inner"} {
		if _, ok := keys[want]; !ok {
			t.Errorf("import alias shadow missing %s: %+v", want, keys)
		}
	}
	refs := keys["GET /outer"].Locations
	found := false
	for _, ref := range refs {
		if ref.Line == 2 {
			found = true
		}
	}
	if !found {
		t.Errorf("missing import provenance: %+v", refs)
	}
	inv = goFixture(t, map[string]string{"internal/httpapi/routes.go": `package httpapi
import h "example/nothttp"
func register(mux any){mux.HandleFunc(h.MethodGet+" /outer",handler)}
`})
	if len(inv.Unresolved) != 1 {
		t.Fatalf("non-http selector assumed standard method: %+v", inv)
	}
	found = false
	for _, ref := range inv.Unresolved[0].Locations {
		if ref.Line == 2 {
			found = true
		}
	}
	if !found {
		t.Errorf("import edit must touch unknown source provenance: %+v", inv.Unresolved)
	}
}

// Source-only oracle: deliberately independent of scanGo/readRouteTable.
func repositoryTableRows(t *testing.T, root, file, facade string) map[string]int {
	t.Helper()
	set := token.NewFileSet()
	files, err := filepath.Glob(filepath.Join(root, filepath.Dir(file), "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	constants := map[string]ast.Expr{}
	var rows *ast.CompositeLit
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(set, path, nil, parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range parsed.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				if gd.Tok == token.CONST && len(vs.Names) == len(vs.Values) {
					for i, name := range vs.Names {
						constants[name.Name] = vs.Values[i]
					}
				}
				marker := ""
				if gd.Doc != nil {
					for _, comment := range gd.Doc.List {
						marker += comment.Text + "\n"
					}
				}
				if gd.Tok == token.VAR && strings.Contains(marker, "contractdrift:routes ") && strings.Contains(marker, "mount="+facade) {
					if filepath.ToSlash(path) != filepath.ToSlash(filepath.Join(root, file)) || rows != nil || len(vs.Values) != 1 {
						t.Fatalf("ambiguous table %s", facade)
					}
					rows, _ = vs.Values[0].(*ast.CompositeLit)
				}
			}
		}
	}
	if rows == nil {
		t.Fatalf("missing table source for %s", facade)
	}
	var text func(ast.Expr, int) string
	text = func(expr ast.Expr, depth int) string {
		if depth > 30 {
			t.Fatal("constant cycle")
		}
		switch node := expr.(type) {
		case *ast.BasicLit:
			value, err := strconv.Unquote(node.Value)
			if err == nil {
				return value
			}
		case *ast.Ident:
			if value, ok := constants[node.Name]; ok {
				return text(value, depth+1)
			}
		case *ast.ParenExpr:
			return text(node.X, depth+1)
		case *ast.BinaryExpr:
			if node.Op == token.ADD {
				return text(node.X, depth+1) + text(node.Y, depth+1)
			}
		case *ast.SelectorExpr:
			// net/http method names are fixed stdlib constants; path constants above
			// are read from the owning package, including their concatenations.
			if id, ok := node.X.(*ast.Ident); ok && id.Name == "http" && strings.HasPrefix(node.Sel.Name, "Method") {
				return strings.ToUpper(strings.TrimPrefix(node.Sel.Name, "Method"))
			}
		}
		t.Fatalf("unsupported table constant %T at %s", expr, set.Position(expr.Pos()))
		return ""
	}
	expected := map[string]int{}
	for _, element := range rows.Elts {
		row, ok := element.(*ast.CompositeLit)
		if !ok {
			t.Fatal("nonliteral table row")
		}
		fields := map[string]string{}
		for _, element := range row.Elts {
			pair, ok := element.(*ast.KeyValueExpr)
			if !ok {
				t.Fatal("unkeyed table field")
			}
			if id, ok := pair.Key.(*ast.Ident); ok && (id.Name == "Method" || id.Name == "Path") {
				fields[id.Name] = text(pair.Value, 0)
			}
		}
		if fields["Method"] == "" || fields["Path"] == "" {
			t.Fatal("missing row method/path")
		}
		key := fields["Method"] + " " + normalizePath(fields["Path"])
		if _, ok := expected[key]; ok {
			t.Fatalf("duplicate row %s", key)
		}
		expected[key] = set.Position(row.Pos()).Line
	}
	return expected
}

func repositoryFunctionContains(t *testing.T, root string, ref location, name string) bool {
	t.Helper()
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, filepath.Join(root, ref.File), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == name {
			return ref.Line <= set.Position(fn.End()).Line && max(ref.Line, ref.End) >= set.Position(fn.Pos()).Line
		}
	}
	return false
}
