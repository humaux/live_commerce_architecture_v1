// Purpose: require opaque global namespace sink values to fail closed through the actual CLI.
// Depends on: compiled reviewCLI and real BFF source snapshots in Git-backed temporary trees.
// Used by: PR33 r1b forwarding-value regression gate; no application execution.
package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestReviewR1BGlobalNamespaceSinkRealCLI(t *testing.T) {
	call := reviewCLI(t)
	for _, reachable := range []bool{true, false} {
		t.Run(map[bool]string{true: "reached computed sink", false: "unused computed sink"}[reachable], func(t *testing.T) {
			root := bffReviewFixture(t, call)
			file := "apps/admin/app/api/stores/[store]/[...resource]/route.ts"
			p := filepath.Join(root, file)
			s := readText(t, p)
			probe := `const hidden=globalThis["fetch"]; await hidden("https://go.invalid/v1/__global_probe",{method:"POST"});`
			if reachable {
				marker := "async function route(request: Request, context: Context) {"
				if strings.Count(s, marker) != 1 {
					t.Fatal("actual forwarding export moved")
				}
				s = strings.Replace(s, marker, marker+"\n"+probe+"\n", 1)
			} else {
				s += "\nasync function unusedGlobalSink(){" + probe + "}\n"
			}
			mustWrite(t, p, s)
			code, out := call(root)
			if !reachable {
				if code != 0 {
					t.Fatalf("unused namespace became forwarding: %d %s", code, out)
				}
				return
			}
			if code != 1 || !strings.Contains(out, "ERROR") || !strings.Contains(out, "UNRESOLVED") || !strings.Contains(out, file+":") {
				t.Fatalf("computed sink value escaped: %d %s", code, out)
			}
		})
	}
}
