// Purpose: reproduce PR33 r1b buyer transport and hidden leaf sinks with the compiled CLI.
// Depends on: real app source snapshots, git-backed fixtures and reviewCLI subprocess helpers.
// Used by: CI-DRIFT package gate; mutations stay in test-owned temporary directories.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Keep the actual application source shape, including imported helpers, rather than
// a hand-written parser fixture that can omit the transport seam under review.
func bffReviewFixture(t *testing.T, call func(string, ...string) (int, string)) string {
	t.Helper()
	root := fixture(t, "aligned")
	for _, app := range []string{"apps/admin", "apps/storefront", "packages"} {
		if err := filepath.WalkDir(filepath.Join("../../..", app), func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "node_modules" || d.Name() == ".next" {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(p, ".ts") || filepath.Base(p) == "package.json" {
				rel, err := filepath.Rel("../../..", p)
				if err != nil {
					return err
				}
				bffWrite(t, root, rel, readText(t, p))
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "actual app BFF source snapshot")
	git(t, root, "tag", "-f", "trunk")
	if code, out := call(root, "-write-baseline"); code != 0 {
		t.Fatalf("seed actual source baseline: %d %s", code, out)
	}
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "reviewed snapshot baseline")
	git(t, root, "tag", "-f", "trunk")
	if code, out := call(root); code != 0 {
		t.Fatalf("unchanged actual source: %d %s", code, out)
	}
	return root
}

func TestReviewR1bBuyerTransportRealCLI(t *testing.T) {
	call := reviewCLI(t)
	for _, mutation := range []string{"DELETE", "unknownMethod()", "omitted", "deleted function", "deleted fetch"} {
		t.Run(mutation, func(t *testing.T) {
			root := bffReviewFixture(t, call)
			file := "apps/storefront/lib/buyer-server.ts"
			p := filepath.Join(root, file)
			s := readText(t, p)
			before := s
			switch mutation {
			case "omitted":
				s = strings.Replace(s, "\n      method,\n", "", 1)
			case "deleted function":
				start, end := strings.Index(s, "async function upstream("), strings.Index(s, "async function upstreamJSON(")
				if start < 0 || end < start {
					t.Fatal("transport function shape moved")
				}
				s = s[:start] + s[end:]
			case "deleted fetch":
				s = strings.Replace(s, "return await fetch(`${config.api}/v1/buyer/${path}`", "return await removedFetch(`${config.api}/v1/buyer/${path}`", 1)
			default:
				method := mutation
				if method == "DELETE" {
					method = `"DELETE"`
				}
				s = strings.Replace(s, "\n      method,\n", "\n      method: "+method+",\n", 1)
			}
			if s == before {
				t.Fatal("transport mutation did not apply")
			}
			mustWrite(t, p, s)

			code, out := call(root)
			if code != 1 || !strings.Contains(out, "ERROR") || !strings.Contains(out, file+":") ||
				!(strings.Contains(out, "UNRESOLVED") || strings.Contains(out, "BFF_REMOVED")) {
				t.Fatalf("actual buyer transport mutation invisible: exit=%d %s", code, out)
			}
			if mutation == "DELETE" && !strings.Contains(out, "UNRESOLVED") && !strings.Contains(out, "DELETE /v1/buyer/") {
				t.Fatalf("fixed method not reflected: %s", out)
			}
		})
	}
}

func TestReviewR1bSpecializedExtraSinksRealCLI(t *testing.T) {
	call := reviewCLI(t)
	for _, file := range []string{
		"apps/admin/app/api/stores/[store]/reports/[report]/route.ts",
		"apps/admin/app/api/stores/[store]/tools/[...resource]/route.ts",
		"apps/storefront/app/api/buyer/[...path]/route.ts",
		"apps/admin/lib/import-proxy.ts",
		"apps/admin/lib/picklist-proxy.ts",
		"apps/storefront/lib/buyer-server.ts",
	} {
		for _, dynamic := range []bool{false, true} {
			t.Run(file+map[bool]string{false: " fixed POST", true: " dynamic URL"}[dynamic], func(t *testing.T) {
				root := bffReviewFixture(t, call)
				p := filepath.Join(root, file)
				s := readText(t, p)
				url := `"https://go.invalid/v1/__drift_probe"`
				if dynamic {
					url = "dynamicURL()"
				}
				probe := "\nawait fetch(" + url + ", { method: \"POST\" });\n"
				switch {
				case strings.HasSuffix(file, "reports/[report]/route.ts"):
					s = strings.Replace(s, "  const { store, report }", probe+"  const { store, report }", 1)
				case strings.HasSuffix(file, "tools/[...resource]/route.ts"):
					s = strings.Replace(s, "  const { store, resource }", probe+"  const { store, resource }", 1)
				case strings.HasSuffix(file, "import-proxy.ts"):
					s = strings.Replace(s, "  const route=importRoute", probe+"  const route=importRoute", 1)
				case strings.HasSuffix(file, "picklist-proxy.ts"):
					s = strings.Replace(s, "  const response = await callBackend(", probe+"  const response = await callBackend(", 1)
				case strings.HasSuffix(file, "buyer-server.ts"):
					s = strings.Replace(s, "  const outbound = new Headers", probe+"  const outbound = new Headers", 1)
				default:
					s = strings.Replace(s, "export const GET = handleBuyerRequest;", "export async function GET(request, context) {"+probe+"return handleBuyerRequest(request, context); }", 1)
				}
				if s == readText(t, p) {
					t.Fatal("extra sink mutation did not apply")
				}
				mustWrite(t, p, s)
				code, out := call(root)
				want := "BFF_NO_GO POST /v1/__drift_probe"
				if dynamic {
					want = "UNRESOLVED"
				}
				if code != 1 || !strings.Contains(out, "ERROR") || !strings.Contains(out, want) || !strings.Contains(out, file+":") {
					t.Fatalf("extra sink invisible: exit=%d want=%s %s", code, want, out)
				}
			})
		}
	}
}

func TestReviewR1bSinkValuesRealCLI(t *testing.T) {
	call := reviewCLI(t)
	for name, probe := range map[string]string{
		"inline alias": `const hidden=callBackend; await hidden("new-uncontracted",{method:"POST"});`,
		"module alias": `await hidden("new-uncontracted",{method:"POST"});`,
		"wrapper":      `const hidden=(...args)=>callBackend(...args); await hidden("new-uncontracted",{method:"POST"});`,
		"member":       `const hidden={forward:callBackend}; await hidden.forward("new-uncontracted",{method:"POST"});`,
		"spread":       `const hidden=[callBackend]; await invoke(...hidden);`,
		"higher order": `await invoke(callBackend,"new-uncontracted",{method:"POST"});`,
		"member call":  `await callBackend.call(null,"new-uncontracted",{method:"POST"});`,
		"empty sink":   `await fetch();`,
	} {
		t.Run(name, func(t *testing.T) {
			root := bffReviewFixture(t, call)
			file := "apps/admin/app/api/stores/[store]/[...resource]/route.ts"
			p := filepath.Join(root, file)
			s := readText(t, p)
			marker := "async function route(request: Request, context: Context) {"
			if !strings.Contains(s, marker) {
				t.Fatal("real route shape moved")
			}
			s = strings.Replace(s, marker, marker+"\n"+probe+"\n", 1)
			if name == "module alias" {
				s = "const hidden=callBackend;\n" + s
			}
			mustWrite(t, p, s)
			code, out := call(root)
			if code != 1 || !strings.Contains(out, "UNRESOLVED") || !strings.Contains(out, "ERROR") || !strings.Contains(out, file+":") {
				t.Fatalf("sink value invisible: %d %s", code, out)
			}
		})
	}
}

func TestReviewR1bUnreachableSinkValueRealCLI(t *testing.T) {
	call := reviewCLI(t)
	root := bffReviewFixture(t, call)
	p := filepath.Join(root, "apps/storefront/app/api/buyer/[...path]/route.ts")
	s := `import { callBackend } from "../../../../../admin/lib/backend";` + "\n" + readText(t, p) + `\nfunction unused(){const hidden=fetch;return hidden("https://go.invalid/v1/__unused",{method:"POST"});}`
	// The import itself and this private function are not reachable from an export.
	s = strings.Replace(s, `\nfunction unused`, "\nfunction unused", 1)
	mustWrite(t, p, s)
	if code, out := call(root); code != 0 {
		t.Fatalf("unreachable alias became forwarding evidence: %d %s", code, out)
	}
}

func TestReviewR1bImportedSinkValuesRealCLI(t *testing.T) {
	call := reviewCLI(t)
	for _, kind := range []string{"alias", "wrapper", "re-export", "unknown binding", "unused import"} {
		t.Run(kind, func(t *testing.T) {
			root := bffReviewFixture(t, call)
			file := "apps/admin/app/api/stores/[store]/[...resource]/route.ts"
			p := filepath.Join(root, file)
			alias := `import {callBackend} from "./backend"; export const hidden=callBackend;`
			if kind == "wrapper" {
				alias = `import {callBackend} from "./backend"; export function hidden(...args){return callBackend(...args);}`
			}
			if kind == "re-export" {
				bffWrite(t, root, "apps/admin/lib/extra-inner.ts", alias)
				alias = `export {hidden} from "./extra-inner";`
			}
			if kind == "unknown binding" {
				alias = `export const other=1;`
			}
			bffWrite(t, root, "apps/admin/lib/extra.ts", alias)
			s := `import {hidden} from "@/lib/extra";` + "\n" + readText(t, p)
			if kind != "unused import" {
				s = strings.Replace(s, "async function route(request: Request, context: Context) {", "async function route(request: Request, context: Context) {\nawait hidden(\"new-uncontracted\",{method:\"POST\"});\n", 1)
			}
			mustWrite(t, p, s)
			code, out := call(root)
			if kind == "unused import" {
				if code != 0 {
					t.Fatalf("unused imported alias: %d %s", code, out)
				}
				return
			}
			if code != 1 || !strings.Contains(out, "ERROR") || !strings.Contains(out, "UNRESOLVED") || !strings.Contains(out, file+":") {
				t.Fatalf("imported sink binding invisible: %d %s", code, out)
			}
		})
	}
}

func TestReviewR1bAdminProxySinkValuesRealCLI(t *testing.T) {
	call := reviewCLI(t)
	for _, reachable := range []bool{false, true} {
		t.Run(map[bool]string{true: "reachable alias", false: "unreachable alias"}[reachable], func(t *testing.T) {
			root := bffReviewFixture(t, call)
			file := "apps/admin/proxy.ts"
			p := filepath.Join(root, file)
			s := readText(t, p)
			if reachable {
				s = "const hidden=fetch;\n" + strings.Replace(s, "export function proxy(request: NextRequest) {", "export function proxy(request: NextRequest) {\nhidden(\"https://go.invalid/v1/__proxy_probe\",{method:\"POST\"});\n", 1)
			} else {
				s += "\nfunction unused(){const hidden=fetch;return hidden(\"https://go.invalid/v1/__unused\",{method:\"POST\"});}\n"
			}
			mustWrite(t, p, s)
			code, out := call(root)
			if !reachable {
				if code != 0 {
					t.Fatalf("unreachable proxy alias: %d %s", code, out)
				}
				return
			}
			if code != 1 || !strings.Contains(out, "UNRESOLVED") || !strings.Contains(out, "ERROR") || !strings.Contains(out, file+":") {
				t.Fatalf("proxy alias invisible: %d %s", code, out)
			}
		})
	}
}

func TestReviewR1bTypeValueDualBindingRealCLI(t *testing.T) {
	call := reviewCLI(t)
	root := seedFixture(t, "aligned")
	file := "apps/admin/app/api/stores/[store]/[...resource]/route.ts"
	p := filepath.Join(root, file)
	s := readText(t, p)
	s = "type hidden=typeof fetch;\nconst hidden=fetch;\n" + strings.Replace(s, "{if(!routes", "{hidden(\"https://go.invalid/v1/__dual\",{method:\"POST\"});if(!routes", 1)
	mustWrite(t, p, s)
	code, out := call(root)
	if code != 1 || !strings.Contains(out, "UNRESOLVED") || !strings.Contains(out, "ERROR") || !strings.Contains(out, file+":") {
		t.Fatalf("dual type/value runtime alias erased: %d %s", code, out)
	}
}

func TestReviewR1bSharedCyclicSinkGraphRealCLI(t *testing.T) {
	call := reviewCLI(t)
	root := bffReviewFixture(t, call)
	helper := `import {callBackend} from "./backend"; function shared(){const hidden=callBackend;return hidden("new-uncontracted",{method:"POST"});}` + "\n"
	for i := 0; i < 48; i++ {
		helper += fmt.Sprintf("export function a%d(flag){shared();if(flag)a%d(false);}\n", i, (i+1)%48)
	}
	bffWrite(t, root, "apps/admin/lib/shared-cycle.ts", helper)
	file := "apps/admin/app/api/stores/[store]/[...resource]/route.ts"
	p := filepath.Join(root, file)
	s := `import {a0,a24} from "@/lib/shared-cycle";` + "\n" + readText(t, p)
	s = strings.Replace(s, "async function route(request: Request, context: Context) {", "async function route(request: Request, context: Context) {\na0(false);a24(false);\n", 1)
	mustWrite(t, p, s)
	code, out := call(root) // reviewCLI retains its existing 30-second process bound.
	if code != 1 || !strings.Contains(out, "ERROR") || !strings.Contains(out, "UNRESOLVED") || !strings.Contains(out, file+":") || !strings.Contains(out, "shared-cycle.ts:") {
		t.Fatalf("shared/cyclic graph lost hazard: %d %s", code, out)
	}
}

func TestReviewR1bWorkspaceSinkBindingsRealCLI(t *testing.T) {
	call := reviewCLI(t)
	for _, used := range []bool{false, true} {
		t.Run(map[bool]string{true: "reached export", false: "unused import"}[used], func(t *testing.T) {
			root := bffReviewFixture(t, call)
			bffWrite(t, root, "packages/drift-extra/package.json", `{"name":"@fixture/drift-extra","exports":{".":"./src/index.ts"}}`)
			bffWrite(t, root, "packages/drift-extra/src/index.ts", `export {hidden} from "./sink";`)
			bffWrite(t, root, "packages/drift-extra/src/sink.ts", `export const hidden=fetch;`)
			file := "apps/admin/app/api/stores/[store]/[...resource]/route.ts"
			p := filepath.Join(root, file)
			s := `import {hidden} from "@fixture/drift-extra";` + "\n" + readText(t, p)
			if used {
				s = strings.Replace(s, "async function route(request: Request, context: Context) {", "async function route(request: Request, context: Context) {\nhidden(\"https://go.invalid/v1/__workspace\",{method:\"POST\"});\n", 1)
			}
			mustWrite(t, p, s)
			code, out := call(root)
			if !used {
				if code != 0 {
					t.Fatalf("unused workspace import: %d %s", code, out)
				}
				return
			}
			for _, want := range []string{"ERROR", "UNRESOLVED", file + ":", "packages/drift-extra/package.json:", "packages/drift-extra/src/sink.ts:"} {
				if code != 1 || !strings.Contains(out, want) {
					t.Fatalf("workspace binding lost %s: %d %s", want, code, out)
				}
			}
		})
	}
}

func TestReviewR1bWorkspaceManifestRetargetRealCLI(t *testing.T) {
	call := reviewCLI(t)
	root := bffReviewFixture(t, call)
	manifest := "packages/drift-extra/package.json"
	bffWrite(t, root, manifest, `{"name":"@fixture/drift-extra","exports":{".":"./src/first.ts"}}`)
	for _, name := range []string{"first", "second"} {
		bffWrite(t, root, "packages/drift-extra/src/"+name+".ts", `export const hidden=fetch;`)
	}
	file := "apps/admin/app/api/stores/[store]/[...resource]/route.ts"
	p := filepath.Join(root, file)
	s := `import {hidden} from "@fixture/drift-extra";` + "\n" + readText(t, p)
	s = strings.Replace(s, "async function route(request: Request, context: Context) {", "async function route(request: Request, context: Context) {\nhidden(\"https://go.invalid/v1/__workspace\",{method:\"POST\"});\n", 1)
	mustWrite(t, p, s)
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "legacy workspace alias")
	git(t, root, "tag", "-f", "trunk")
	if code, out := call(root); code != 0 {
		t.Fatalf("unchanged legacy hazard: %d %s", code, out)
	}
	mustWrite(t, filepath.Join(root, manifest), `{"name":"@fixture/drift-extra","exports":{".":"./src/second.ts"}}`)
	code, out := call(root)
	if code != 1 || !strings.Contains(out, "ERROR") || !strings.Contains(out, "UNRESOLVED") || !strings.Contains(out, manifest+":") {
		t.Fatalf("manifest-only retarget escaped touched: %d %s", code, out)
	}
}

func TestReviewR1bMerchantTransportProofRealCLI(t *testing.T) {
	call := reviewCLI(t)
	for _, kind := range []string{"fixed override", "dynamic URL", "spread before init", "path reassigned", "config origin retarget", "config property reassigned", "computed config assignment", "config object escape"} {
		t.Run(kind, func(t *testing.T) {
			root := bffReviewFixture(t, call)
			file := "apps/admin/lib/auth.ts"
			p := filepath.Join(root, file)
			s := readText(t, p)
			start := strings.Index(s, "export async function merchantBackend(")
			if start < 0 {
				t.Fatal("merchant seam moved")
			}
			prefix, transport := s[:start], s[start:]
			if kind == "computed config assignment" {
				transport = strings.Replace(transport, "  const headers = new Headers(init.headers);", "  authConfig[\"apiOrigin\"] = \"https://go.invalid/v1/__merchant_probe\";\n  const headers = new Headers(init.headers);", 1)
			} else if kind == "config object escape" {
				transport = strings.Replace(transport, "  const headers = new Headers(init.headers);", "  Object.assign(authConfig, {apiOrigin: \"https://go.invalid/v1/__merchant_probe\"});\n  const headers = new Headers(init.headers);", 1)
			} else if kind == "config property reassigned" {
				transport = strings.Replace(transport, "  const headers = new Headers(init.headers);", "  authConfig.apiOrigin = \"https://go.invalid/v1/__merchant_probe\";\n  const headers = new Headers(init.headers);", 1)
			} else if kind == "config origin retarget" {
				prefix = strings.Replace(prefix, "apiOrigin: exactOrigin(", "apiOrigin: unprovenOrigin(", 1)
			} else if kind == "path reassigned" {
				transport = strings.Replace(transport, "  if (!authConfig)", "  path = \"/v1/__merchant_probe\";\n  if (!authConfig)", 1)
			} else if kind == "dynamic URL" {
				transport = strings.Replace(transport, "fetch(`${authConfig.apiOrigin}${path}`", "fetch(dynamicURL()", 1)
			} else {
				extra := "...init, method: \"DELETE\","
				if kind == "spread before init" {
					extra = "method: \"DELETE\", ...init,"
				}
				transport = strings.Replace(transport, "...init,", extra, 1)
			}
			if prefix+transport == s {
				t.Fatal("merchant mutation did not apply")
			}
			mustWrite(t, p, prefix+transport)
			code, out := call(root)
			if code != 1 || !strings.Contains(out, "UNRESOLVED") || !strings.Contains(out, "ERROR") || !strings.Contains(out, file+":") {
				t.Fatalf("merchant proof accepted unsupported transport: %d %s", code, out)
			}
		})
	}
}

func TestReviewR1bOriginObjectEscapeRealCLI(t *testing.T) {
	call := reviewCLI(t)
	root := bffReviewFixture(t, call)
	file := "apps/admin/lib/auth.ts"
	p := filepath.Join(root, file)
	s := readText(t, p)
	s = strings.Replace(s, "  return url.origin;", "  Object.defineProperty(url, \"origin\", {value: \"https://go.invalid/v1/__merchant_probe\"});\n  return url.origin;", 1)
	mustWrite(t, p, s)
	code, out := call(root)
	if code != 1 || !strings.Contains(out, "UNRESOLVED") || !strings.Contains(out, "ERROR") || !strings.Contains(out, file+":") {
		t.Fatalf("origin object escape accepted as native getter: %d %s", code, out)
	}
}
