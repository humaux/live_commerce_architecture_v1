// Purpose: independent source-first BFF extractor acceptance cases.
// Depends on: testing and immutable source fixtures; used by CI-DRIFT Go tests.
// Invariant: forwarding grammar, not Go candidates, determines the inventory.
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func bffHas(inv inventory, method, path string) bool {
	for _, r := range inv.Routes {
		if r.Method == method && r.Path == path {
			return true
		}
	}
	return false
}
func TestBFFRootFixtures(t *testing.T) {
	for _, name := range []string{"aligned", "drift"} {
		inv, err := scanBFF(filepath.Join("testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		if !bffHas(inv, "GET", "/v1/admin/stores/{}/widgets/{}") {
			t.Fatalf("%s missing widget: %+v", name, inv)
		}
		if name == "drift" && !bffHas(inv, "POST", "/v1/admin/stores/{}/phantom") {
			t.Fatalf("lost BFF-only addition: %+v", inv)
		}
	}
}
func TestBFFImportsUnknownAndProvenance(t *testing.T) {
	root := t.TempDir()
	leaf := "apps/admin/app/api/stores/[store]/[...resource]/route.ts"
	grammar := "apps/admin/lib/rules.ts"
	files := map[string]string{leaf: `import { grammar as routes } from "@/lib/rules";
const forward=(request,store,path)=> {if(!routes[request.method].test(path))return null;return fetch(` + "`https://go.invalid/v1/admin/stores/${store}/${path}`" + `,{method:request.method})};
export const GET=forward; export const POST=forward;`, grammar: `const id="[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}";
const root="items";
export const grammar={GET:new RegExp(` + "`^${root}/${id}(?:/download\\\\.csv)?$`" + `),POST:new RegExp(dynamicPattern())};`}
	for p, s := range files {
		p = filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(s), 0644); err != nil {
			t.Fatal(err)
		}
	}
	inv, err := scanBFF(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"/v1/admin/stores/{}/items/{}", "/v1/admin/stores/{}/items/{}/download.csv"} {
		if !bffHas(inv, "GET", p) {
			t.Fatalf("missing %s: %+v", p, inv)
		}
	}
	if len(inv.Unresolved) == 0 {
		t.Fatal("dynamic grammar was silently dropped")
	}
	for _, r := range inv.Routes {
		seen := false
		for _, l := range r.Locations {
			if l.File == grammar && l.Line == 1 {
				seen = true
			}
		}
		if !seen {
			t.Fatalf("lost UUID const provenance: %+v", r)
		}
	}
}
func TestBFFRealFamilies(t *testing.T) {
	inv, err := scanBFF("../../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range []struct{ m, p string }{{"GET", "/v1/admin/stores/{}/operations/{}"}, {"POST", "/v1/admin/stores/{}/operations/{}/retry"}, {"GET", "/v1/admin/stores/{}/reports/manual-orders.csv"}, {"POST", "/v1/identity/login/complete"}, {"POST", "/v1/buyer/session/bootstrap"}, {"GET", "/v1/buyer/catalog/v2/products/{}"}, {"GET", "/v1/buyer/storefront/primary-origin"}, {"POST", "/v1/admin/stores/{}/imports/orders/commit"}, {"POST", "/v1/admin/stores/{}/live-sessions/{}/comments/{}/print"}} {
		if !bffHas(inv, r.m, r.p) {
			t.Errorf("missing %s %s", r.m, r.p)
		}
	}
	if bffHas(inv, "POST", "/v1/buyer/session/prepare") {
		t.Fatal("local mint invented upstream")
	}
	t.Logf("routes=%d unresolved=%d", len(inv.Routes), len(inv.Unresolved))
	for _, u := range inv.Unresolved {
		t.Logf("unresolved %+v", u)
	}
}
func TestBFFBuyerMethods(t *testing.T) {
	inv, e := scanBFF("../../..")
	if e != nil {
		t.Fatal(e)
	}
	for _, r := range []struct{ m, p string }{{"GET", "orders/{}/payment"}, {"POST", "orders/{}/payment/prepare"}, {"POST", "orders/{}/payment/handoff"}, {"POST", "orders/{}/payment/refresh"}, {"POST", "orders/{}/payment/cancel"}, {"GET", "orders/{}/bank-transfer"}, {"PUT", "orders/{}/bank-transfer/proof"}, {"GET", "cvs-selections/{}"}, {"POST", "cvs-selections/{}/verify"}, {"GET", "quotes/{}"}, {"GET", "destinations/{}"}, {"GET", "orders/{}"}, {"PUT", "consents"}, {"POST", "privacy/erasure"}} {
		if !bffHas(inv, r.m, "/v1/buyer/"+r.p) {
			t.Errorf("missing %s %s", r.m, r.p)
		}
	}
}

func bffWrite(t *testing.T, root, path, source string) {
	t.Helper()
	file := filepath.Join(root, path)
	if e := os.MkdirAll(filepath.Dir(file), 0755); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(file, []byte(source), 0644); e != nil {
		t.Fatal(e)
	}
}
func TestBFFUnsupportedFormsAndExtraSinks(t *testing.T) {
	leaf := "apps/admin/app/api/stores/[store]/[...resource]/route.ts"
	for _, pattern := range []string{`new RegExp("^items/.*$")`, `new RegExp("items")`, `new RegExp("^items$","i")`, `new RegExp(dynamicPattern())`} {
		t.Run(pattern, func(t *testing.T) {
			root := t.TempDir()
			bffWrite(t, root, leaf, `const routes={GET:`+pattern+`};const forward=(request,store,path)=>fetch(`+"`https://go.invalid/v1/admin/stores/${store}/${path}`"+`,{method:request.method});export const GET=forward;`)
			inv, e := scanBFF(root)
			if e != nil {
				t.Fatal(e)
			}
			if len(inv.Unresolved) == 0 {
				t.Fatalf("unsupported regex silently accepted: %+v", inv)
			}
		})
	}
	root := t.TempDir()
	bffWrite(t, root, leaf, `const routes={GET:/^items$/,HEAD:/^secret$/};
const forward=(request,store,path)=>{fetch("https://go.invalid/v1/admin/stores/fixed/added",{method:"POST"});fetch(dynamicURL(),{method:"GET"});return fetch(`+"`https://go.invalid/v1/admin/stores/${store}/${path}`"+`,{method:request.method})};
export const GET=forward;export const POST=forward;export const HEAD=()=>new Response(null,{status:405});`)
	inv, e := scanBFF(root)
	if e != nil {
		t.Fatal(e)
	}
	if !bffHas(inv, "POST", "/v1/admin/stores/fixed/added") {
		t.Fatal("extra fixed forwarding call was lost")
	}
	if len(inv.Unresolved) == 0 {
		t.Fatal("extra unknown forwarding call was lost")
	}
	if bffHas(inv, "HEAD", "/v1/admin/stores/{}/secret") {
		t.Fatal("local rejection alias became upstream producer")
	}
	guard := t.TempDir()
	bffWrite(t, guard, "apps/admin/proxy.ts", `const allowed={POST:/^phantom$/};export function proxy(request){return allowed[request.method]?.test(request.path)}`)
	inv, e = scanBFF(guard)
	if e != nil {
		t.Fatal(e)
	}
	if len(inv.Routes) > 0 {
		t.Fatal("recognition-only guard became producer")
	}
}
func TestBFFRealSourceMutation(t *testing.T) {
	root := t.TempDir()
	sourceRoot := "../../.."
	// Copy only existing TypeScript sources into this test-owned tree. No source under
	// test changes while concurrent gates may be reading the repository.
	for _, app := range []string{"apps/admin", "apps/storefront"} {
		if e := filepath.WalkDir(filepath.Join(sourceRoot, app), func(path string, d os.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if d.IsDir() {
				if d.Name() == "node_modules" || d.Name() == ".next" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".ts") {
				return nil
			}
			rel, e := filepath.Rel(sourceRoot, path)
			if e != nil {
				return e
			}
			data, e := os.ReadFile(path)
			if e != nil {
				return e
			}
			bffWrite(t, root, rel, string(data))
			return nil
		}); e != nil {
			t.Fatal(e)
		}
	}
	path := "apps/admin/src/features/live/comment-request.ts"
	file := filepath.Join(root, path)
	data, e := os.ReadFile(file)
	if e != nil {
		t.Fatal(e)
	}
	original := string(data)
	changed := strings.Replace(original, "(?:print|private-reply|public-reply)", "(?:ci-drift-added|print|private-reply|public-reply)", 1)
	if changed == original {
		t.Fatal("mutation insertion point no longer exists")
	}
	bffWrite(t, root, path, changed)
	inv, e := scanBFF(root)
	if e != nil {
		t.Fatal(e)
	}
	target := "/v1/admin/stores/{}/live-sessions/{}/comments/{}/ci-drift-added"
	if !bffHas(inv, "POST", target) {
		t.Fatal("imported BFF-only new alternative was lost")
	}
	for _, r := range inv.Routes {
		if r.Path == target {
			found := false
			for _, l := range r.Locations {
				if l.File == path && l.Line <= 8 && l.End >= 8 {
					found = true
				}
			}
			if !found {
				t.Fatalf("lost changed grammar span: %+v", r)
			}
		}
	}
	changed = strings.Replace(changed, "const write = new RegExp(", "const write = new RegExp(dynamicGrammar(),", 1)
	bffWrite(t, root, path, changed)
	inv, e = scanBFF(root)
	if e != nil {
		t.Fatal(e)
	}
	if len(inv.Unresolved) == 0 {
		t.Fatal("unsupported imported grammar was silently dropped")
	}
	bffWrite(t, root, path, original)
	inv, e = scanBFF(root)
	if e != nil {
		t.Fatal(e)
	}
	if bffHas(inv, "POST", target) || len(inv.Unresolved) != 0 {
		t.Fatalf("restore did not return clean inventory: routes=%d unresolved=%d", len(inv.Routes), len(inv.Unresolved))
	}
	transport := "apps/admin/app/api/stores/[store]/[...resource]/route.ts"
	data, e = os.ReadFile(filepath.Join(root, transport))
	if e != nil {
		t.Fatal(e)
	}
	before := string(data)
	after := strings.Replace(before, "const init: RequestInit = { method: request.method };", "const init: RequestInit = { method: \"DELETE\" };", 1)
	if after == before {
		t.Fatal("transport method insertion point moved")
	}
	bffWrite(t, root, transport, after)
	inv, e = scanBFF(root)
	if e != nil {
		t.Fatal(e)
	}
	if bffHas(inv, "GET", bffStorePrefix+"catalog-ledger") || !bffHas(inv, "DELETE", bffStorePrefix+"catalog-ledger") {
		t.Fatal("upstream method mutation was hidden by browser admission")
	}
}
func TestBFFNestedTemplatesAndConcat(t *testing.T) {
	root := t.TempDir()
	path := "apps/admin/app/api/stores/[store]/[...resource]/route.ts"
	bffWrite(t, root, path, "const unrelated=`nested ${kind === 'x' ? `query;${other}` : ''}`;\n"+`const prefix="reports"+"/";const routes={GET:new RegExp("^"+prefix+"(daily|weekly)(?:\\.csv)?$")};const forward=(request,store,path)=>fetch(`+"`https://go.invalid/v1/admin/stores/${store}/${path}`"+`,{method:request.method});export const GET=forward;`)
	inv, e := scanBFF(root)
	if e != nil {
		t.Fatal(e)
	}
	for _, suffix := range []string{"daily", "weekly", "daily.csv", "weekly.csv"} {
		if !bffHas(inv, "GET", bffStorePrefix+"reports/"+suffix) {
			t.Fatalf("nested template disturbed following declaration: %+v", inv)
		}
	}
}
