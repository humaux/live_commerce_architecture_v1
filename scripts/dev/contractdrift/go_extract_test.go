// Purpose: exercise actual Go fixture syntax and independent expected inventories.
// Depends on: stdlib testing/os; never starts HTTP handlers.
// Used by: contractdrift test gate; changing a constant or wrapper must retain provenance.
package main

import (
	"os"
	"path/filepath"
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
	if len(inv.Routes) != 5 || len(inv.Unresolved) != 0 || len(inv.Notes) != 3 {
		t.Errorf("fallbacks are notes, not business routes: %+v", inv)
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
	if len(inv.Unresolved) != 1 || inv.Unresolved[0].Method != "EXACT" || inv.Unresolved[0].Path != "/internal/custom" || !strings.Contains(inv.Unresolved[0].Detail, "opaque") {
		t.Errorf("opaque dispatch absent: %+v", inv.Unresolved)
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
	for _, want := range []string{"PREFIX /v1/buyer", "PREFIX /v1/meta", "PREFIX /v1/stripe", "PREFIX /.well-known/lc-domain-check/", "EXACT /v1/buyer/feeds/meta.csv", "EXACT /v1/platform/stripe/webhook", "EXACT /internal/tls-ask"} {
		if _, ok := coverage[want]; !ok {
			t.Errorf("missing actual cmd/api coverage guard %s", want)
		}
	}
	for _, pair := range [][2]string{{"PREFIX /v1/buyer", "internal/buyerhttp/handler.go"}, {"PREFIX /v1/meta", "internal/integrations/meta/runtime.go"}, {"PREFIX /v1/stripe", "internal/payments/stripewebhook/handler.go"}, {"EXACT /internal/tls-ask", "internal/tlsask/tlsask.go"}} {
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

	keys := goRouteKeys(inv)
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
