// Purpose: reproduce PR33 r1b buyer transport and hidden leaf sinks with the compiled CLI.
// Depends on: real app source snapshots, git-backed fixtures and reviewCLI subprocess helpers.
// Used by: CI-DRIFT package gate; mutations stay in test-owned temporary directories.
package main

import (
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
	for _, app := range []string{"apps/admin", "apps/storefront"} {
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
			if strings.HasSuffix(p, ".ts") {
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
