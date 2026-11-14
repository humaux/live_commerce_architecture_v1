// Purpose: retain PR55 structural counterexamples and exact converted CDF1-3 receipts.
// Depends on: existing real CLI/Git fixture helpers and Go stdlib; historical receipts stay immutable.
// Used by: GitHub CI and independent CI-DRIFT review.
// Invariant: baseline coverage is mandatory; every strict candidate logs its full output.
package main

import (
	"strings"
	"testing"
)

// The expected locations follow the unchanged fixture's source lines. Deliberately
// compare the entire CLI response, including the generic dispatcher and totals.
func trunkFactoryExactTouched(t *testing.T, call func(string, ...string) (int, string), root, exact string) {
	t.Helper()
	code, out := call(root)
	t.Logf("STRICT_CANDIDATE exit=%d\n%s", code, out)
	want := strings.Join([]string{
		"INFO () GO_MOUNT_OR_FALLBACK  /v1/trunk-factory cmd/api/main.go:3, cmd/api/main.go:13, cmd/api/main.go:15, cmd/api/main.go:23 methodless registration; child paths remain absolute unless StripPrefix is explicit",
		"ERROR (TOUCHED) UNRESOLVED   cmd/api/main.go:17, cmd/api/main.go:23, cmd/api/main.go:30, cmd/api/main.go:37, cmd/api/main.go:43 opaque Go handler dispatch buildBuyer; route/method coverage is unknown",
		"ERROR (TOUCHED) UNRESOLVED EXACT " + exact + " cmd/api/main.go:17, cmd/api/main.go:23, cmd/api/main.go:30, cmd/api/main.go:37, cmd/api/main.go:43, cmd/api/main.go:45 opaque Go handler dispatch mountTrunkFactory; route/method coverage is unknown",
		"ERROR (TOUCHED) UNRESOLVED MOUNT /v1/trunk-factory cmd/api/main.go:3, cmd/api/main.go:13, cmd/api/main.go:15, cmd/api/main.go:17, cmd/api/main.go:23, cmd/api/main.go:30, cmd/api/main.go:37, cmd/api/main.go:43 opaque Go methodless mount; child route identity is unproven",
		"SUMMARY routes go=1 contract=1 bff=1; errors=3 warnings=0 GO_MOUNT_OR_FALLBACK=1 UNRESOLVED=3",
	}, "\n") + "\n"
	if code != 1 || out != want {
		t.Fatalf("strict candidate signature mismatch: exit=%d want=1\nactual:\n%sexpected:\n%s", code, out, want)
	}
}

// Owner unfreeze 2026-11-14: all three historical misses are now required.
// Historical exact failure receipts remain in output/ci-drift-fn.
func trunkFactoryReviewTouched(t *testing.T, name, path, exact string, code int, out string) {
	t.Helper()
	provenance := map[string]string{
		"CDF4 cross-package middleware": "internal/mw/mw.go:3",
		"CDF5 non-path global":          "cmd/api/main.go:9",
		"CDF6 sibling-file route":       "cmd/api/factory.go:29",
	}[name]
	if provenance == "" || !strings.HasPrefix(provenance, path+":") {
		t.Fatalf("invalid strict fixture %q %q", name, path)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	prefixes := []string{
		"INFO () GO_MOUNT_OR_FALLBACK  /v1/trunk-factory ",
		"ERROR (TOUCHED) UNRESOLVED   ",
		"ERROR (TOUCHED) UNRESOLVED EXACT " + exact + " ",
		"ERROR (TOUCHED) UNRESOLVED MOUNT /v1/trunk-factory ",
	}
	if code != 1 || len(lines) != 5 || lines[4] != "SUMMARY routes go=1 contract=1 bff=1; errors=3 warnings=0 GO_MOUNT_OR_FALLBACK=1 UNRESOLVED=3" {
		t.Fatalf("strict CDF signature mismatch: exit=%d want=1, three errors and zero warnings\n%s", code, out)
	}
	for i, prefix := range prefixes {
		if !strings.HasPrefix(lines[i], prefix) {
			t.Errorf("diagnostic %d missing %q: %s", i, prefix, lines[i])
		}
		if i > 0 && !strings.Contains(lines[i], provenance+" ") && !strings.Contains(lines[i], provenance+",") {
			t.Errorf("diagnostic %d missing changed dependency provenance %s: %s", i, provenance, lines[i])
		}
	}
}

func TestGoFactoryReviewStructuralCLI(t *testing.T) {
	call := reviewCLI(t)
	for _, name := range []string{"CDF4 cross-package middleware", "CDF5 non-path global", "CDF6 sibling-file route"} {
		t.Run(name, func(t *testing.T) {
			source := trunkFactorySource
			extra := map[string]string{}
			path, exact := "cmd/api/main.go", "/v1/trunk-factory"
			var candidate string
			switch name {
			case "CDF4 cross-package middleware":
				const middleware = `package mw
import "net/http"
func WithHeader(next http.Handler) http.Handler {
 return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  w.Header().Set("X-Middleware", "legacy")
  next.ServeHTTP(w, r)
 })
}
`
				source = trunkFactoryReplace(t, source, `"net/http"`, "\"net/http\"\n \"example/internal/mw\"")
				source = trunkFactoryReplace(t, source, "factoryHeader(h)", "mw.WithHeader(h)")
				start := strings.Index(source, "func factoryHeader(")
				end := strings.Index(source, "func mountTrunkFactory(")
				if start < 0 || end <= start {
					t.Fatal("cross-package fixture seam missing")
				}
				source = source[:start] + source[end:]
				path = "internal/mw/mw.go"
				extra[path] = middleware
				candidate = trunkFactoryReplace(t, middleware, `"X-Middleware", "legacy"`, `"X-Middleware", "changed"`)
			case "CDF5 non-path global":
				candidate = trunkFactoryReplace(t, source, `var globalLabel = "legacy"`, `var globalLabel = "changed"`)
			case "CDF6 sibling-file route":
				i := strings.Index(source, "func openPool(")
				if i < 0 {
					t.Fatal("sibling-file fixture seam missing")
				}
				path = "cmd/api/factory.go"
				extra[path] = "package main\nimport (\"context\"; \"errors\"; \"net/http\")\n" + source[i:]
				source = source[:i]
				source = trunkFactoryReplace(t, source, " \"errors\"\n", "")
				exact += "/changed"
				candidate = trunkFactoryReplace(t, extra[path], `r.URL.Path == "/v1/trunk-factory"`, `r.URL.Path == "/v1/trunk-factory/changed"`)
			default:
				t.Fatalf("unknown structural regression %q", name)
			}
			root := trunkFactoryLegacy(t, call, source, extra)
			trunkFactoryWrite(t, root, path, candidate)
			code, out := call(root)
			t.Logf("STRICT_CANDIDATE %s path=%s exit=%d\n%s", name, path, code, out)
			trunkFactoryReviewTouched(t, name, path, exact, code, out)
		})
	}
}
