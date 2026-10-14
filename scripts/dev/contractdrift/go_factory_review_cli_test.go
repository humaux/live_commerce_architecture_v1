// Purpose: retain PR55 structural counterexamples and exact converted CDF1-3 receipts.
// Depends on: existing real CLI/Git fixture helpers and Go stdlib; scanner stays frozen.
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

// These signatures come from Foundation 38499888354/job 115644072791 at
// 3a3ae4cb9 (merge checkout ca7741b5). Only the complete observed output may skip.
// Unknown output, setup failures and a repaired scanner all remain fatal.
func trunkFactoryReviewKnownMiss(t *testing.T, name, path, exact string, code int, out string) {
	t.Helper()
	wantCode, wantPath, wantExact, id := 0, "cmd/api/main.go", "/v1/trunk-factory", ""
	var want string
	switch name {
	case "CDF4 cross-package middleware":
		id, wantPath = "CDF-4", "internal/mw/mw.go"
		want = `INFO () GO_MOUNT_OR_FALLBACK  /v1/trunk-factory cmd/api/main.go:3, cmd/api/main.go:14, cmd/api/main.go:16, cmd/api/main.go:24 methodless registration; child paths remain absolute unless StripPrefix is explicit
WARNING (unresolved) UNRESOLVED   cmd/api/main.go:18, cmd/api/main.go:24, cmd/api/main.go:31, cmd/api/main.go:38 opaque Go handler dispatch buildBuyer; route/method coverage is unknown
WARNING (unresolved) UNRESOLVED EXACT /v1/trunk-factory cmd/api/main.go:18, cmd/api/main.go:24, cmd/api/main.go:31, cmd/api/main.go:38, cmd/api/main.go:40 opaque Go handler dispatch mountTrunkFactory; route/method coverage is unknown
WARNING (unresolved) UNRESOLVED MOUNT /v1/trunk-factory cmd/api/main.go:3, cmd/api/main.go:14, cmd/api/main.go:16, cmd/api/main.go:18, cmd/api/main.go:24, cmd/api/main.go:31, cmd/api/main.go:38 opaque Go methodless mount; child route identity is unproven
SUMMARY routes go=1 contract=1 bff=1; errors=0 warnings=3 GO_MOUNT_OR_FALLBACK=1 UNRESOLVED=3
`
	case "CDF5 non-path global":
		id = "CDF-5"
		want = `INFO () GO_MOUNT_OR_FALLBACK  /v1/trunk-factory cmd/api/main.go:3, cmd/api/main.go:13, cmd/api/main.go:15, cmd/api/main.go:23 methodless registration; child paths remain absolute unless StripPrefix is explicit
WARNING (unresolved) UNRESOLVED   cmd/api/main.go:17, cmd/api/main.go:23, cmd/api/main.go:30, cmd/api/main.go:37, cmd/api/main.go:43 opaque Go handler dispatch buildBuyer; route/method coverage is unknown
WARNING (unresolved) UNRESOLVED EXACT /v1/trunk-factory cmd/api/main.go:17, cmd/api/main.go:23, cmd/api/main.go:30, cmd/api/main.go:37, cmd/api/main.go:43, cmd/api/main.go:45 opaque Go handler dispatch mountTrunkFactory; route/method coverage is unknown
WARNING (unresolved) UNRESOLVED MOUNT /v1/trunk-factory cmd/api/main.go:3, cmd/api/main.go:13, cmd/api/main.go:15, cmd/api/main.go:17, cmd/api/main.go:23, cmd/api/main.go:30, cmd/api/main.go:37, cmd/api/main.go:43 opaque Go methodless mount; child route identity is unproven
SUMMARY routes go=1 contract=1 bff=1; errors=0 warnings=3 GO_MOUNT_OR_FALLBACK=1 UNRESOLVED=3
`
	case "CDF6 sibling-file route":
		id, wantCode, wantPath, wantExact = "CDF-6", 1, "cmd/api/factory.go", "/v1/trunk-factory/changed"
		want = `INFO () GO_MOUNT_OR_FALLBACK  /v1/trunk-factory cmd/api/factory.go:9, cmd/api/main.go:3, cmd/api/main.go:12, cmd/api/main.go:14 methodless registration; child paths remain absolute unless StripPrefix is explicit
ERROR (TOUCHED) UNRESOLVED   cmd/api/factory.go:3, cmd/api/factory.go:9, cmd/api/factory.go:16, cmd/api/factory.go:23, cmd/api/factory.go:29 opaque Go handler dispatch buildBuyer; route/method coverage is unknown
ERROR (TOUCHED) UNRESOLVED EXACT /v1/trunk-factory/changed cmd/api/factory.go:3, cmd/api/factory.go:9, cmd/api/factory.go:16, cmd/api/factory.go:23, cmd/api/factory.go:29, cmd/api/factory.go:31 opaque Go handler dispatch mountTrunkFactory; route/method coverage is unknown
WARNING (unresolved) UNRESOLVED MOUNT /v1/trunk-factory cmd/api/factory.go:9, cmd/api/main.go:3, cmd/api/main.go:12, cmd/api/main.go:14 opaque Go methodless mount; child route identity is unproven
SUMMARY routes go=1 contract=1 bff=1; errors=2 warnings=1 GO_MOUNT_OR_FALLBACK=1 UNRESOLVED=3
`
	default:
		t.Fatalf("unreviewed expected-fail case %q", name)
	}
	if path != wantPath || exact != wantExact {
		t.Fatalf("expected-fail fixture changed: path=%q exact=%q want path=%q exact=%q", path, exact, wantPath, wantExact)
	}
	if code == 1 && strings.Contains(out, "ERROR (TOUCHED) UNRESOLVED   ") &&
		strings.Contains(out, "ERROR (TOUCHED) UNRESOLVED EXACT "+wantExact+" ") &&
		strings.Contains(out, "ERROR (TOUCHED) UNRESOLVED MOUNT /v1/trunk-factory ") &&
		strings.Contains(out, "SUMMARY routes go=1 contract=1 bff=1; errors=3 warnings=0 GO_MOUNT_OR_FALLBACK=1 UNRESOLVED=3\n") &&
		!strings.Contains(out, "WARNING (") {
		t.Fatalf("UNEXPECTED_PASS %s: retire its xfail in output/ci-drift-fn/FOLLOWUPS.md\n%s", id, out)
	}
	if code != wantCode || out != want {
		t.Fatalf("unrecognized expected-fail signature %s: exit=%d want=%d\nactual:\n%sexpected:\n%s", id, code, wantCode, out, want)
	}
	t.Skipf("EXPECTED_FAIL %s: exact Foundation 38499888354 diagnostic reproduced; see output/ci-drift-fn/FOLLOWUPS.md", id)
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
			trunkFactoryReviewKnownMiss(t, name, path, exact, code, out)
		})
	}
}
