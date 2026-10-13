// Purpose: challenge startup admission proof with renamed helpers and hidden routing effects through the real CLI.
// Depends on: reviewCLI, frozen OA1 source constants, Git fixture helpers and stdlib; PGx fixture source is only parsed.
// Used by: CI-DRIFT-FACTORY CI regression and independent parent acceptance.
// Invariant: names cannot grant admission; imported helper refs and captured/global writes must remain touched.
package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestGoFactoryOA1RenamedAdmissionCLI(t *testing.T) {
	call := reviewCLI(t)
	source := strings.ReplaceAll(goFactoryOA1Source, "buildBuyerWithCVS", "assembleCheckout")
	platform := strings.ReplaceAll(goFactoryOA1Platform, "ValidateSameDatabase", "CheckPoolIdentity")
	probe := strings.ReplaceAll(goFactoryOA1Probe, "ValidateSameDatabase", "CheckPoolIdentity")
	root, p := goFactoryOA1Legacy(t, call, source, platform, nil)
	mustWrite(t, p, strings.Replace(source, "_ = mainPool", probe, 1))
	if code, out := call(root); code != 0 || strings.Contains(out, "TOUCHED") || !strings.Contains(out, "SUMMARY routes go=1 contract=1 bff=1;") || !strings.Contains(out, "WARNING (unresolved) UNRESOLVED EXACT /v1/factory-oa1") || !strings.Contains(out, "WARNING (unresolved) UNRESOLVED MOUNT /v1/factory-oa1") {
		t.Fatalf("renamed startup-only admission changed coverage: exit=%d %s", code, out)
	}
}

func TestGoFactoryOA1AdmissionHelperReferencesCLI(t *testing.T) {
	call := reviewCLI(t)
	for _, tc := range []struct{ name, before, after string }{
		{"comment", "\tdenied := errors.New", "\t// Same advisory SQL; helper provenance must remain tracked.\n\tdenied := errors.New"},
		{"statement", "\treturn nil\n}", "\t_ = key\n\treturn nil\n}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, p := goFactoryOA1Legacy(t, call, goFactoryOA1Source, goFactoryOA1Platform, nil)
			mustWrite(t, p, strings.Replace(goFactoryOA1Source, "_ = mainPool", goFactoryOA1Probe, 1))
			if code, out := call(root); code != 0 || strings.Contains(out, "TOUCHED") || !strings.Contains(out, "SUMMARY routes go=1 contract=1 bff=1;") || !strings.Contains(out, "WARNING (unresolved) UNRESOLVED EXACT /v1/factory-oa1") || !strings.Contains(out, "WARNING (unresolved) UNRESOLVED MOUNT /v1/factory-oa1") {
				t.Fatalf("startup probe must be accepted before helper-only change: exit=%d %s", code, out)
			}
			git(t, root, "add", ".")
			git(t, root, "commit", "-qm", "accepted OA1 admission baseline")
			git(t, root, "tag", "-f", "trunk")
			platform := strings.Replace(goFactoryOA1Platform, tc.before, tc.after, 1)
			mustWrite(t, filepath.Join(root, "internal/platform/runtime.go"), platform)
			if code, out := call(root); code != 1 || !strings.Contains(out, "ERROR (TOUCHED) UNRESOLVED") || !strings.Contains(out, "SUMMARY routes go=1 contract=1 bff=1;") || !strings.Contains(out, "UNRESOLVED EXACT /v1/factory-oa1") || !strings.Contains(out, "UNRESOLVED MOUNT /v1/factory-oa1") {
				t.Fatalf("imported admission helper body edit lost provenance: exit=%d %s", code, out)
			}
		})
	}
}

func TestGoFactoryOA1RealPoolCallbackOriginCLI(t *testing.T) {
	call := reviewCLI(t)
	source := strings.Replace(goFactoryOA1Source, `"github.com/jackc/pgx/v5/pgxpool"`, `"github.com/jackc/pgx/v5"
 "github.com/jackc/pgx/v5/pgxpool"`, 1)
	source = strings.Replace(source, "h, _, _, err := buildBuyerWithCVS(context.Background(), c, &pgxpool.Pool{})", `ctx := context.Background()
 mainPool, err := makePool(ctx, c)
 if err != nil { return }
 h, _, _, err := buildBuyerWithCVS(ctx, c, mainPool)`, 1)
	// The real PGx constructor and its captured write exist before the sole probe-addition diff.
	source += `
func makePool(ctx context.Context, c *buyerConfig) (*pgxpool.Pool, error) {
 cfg, err := pgxpool.ParseConfig("fixture")
 if err != nil { return nil, err }
 cfg.BeforeAcquire = func(context.Context, *pgx.Conn) bool {
  c.bffKey = "changed"
  return true
 }
 return pgxpool.NewWithConfig(ctx, cfg)
}
`
	root, p := goFactoryOA1Legacy(t, call, source, goFactoryOA1Platform, nil)
	mustWrite(t, p, strings.Replace(source, "_ = mainPool", goFactoryOA1Probe, 1))
	if code, out := call(root); code != 1 || !strings.Contains(out, "ERROR (TOUCHED) UNRESOLVED") || !strings.Contains(out, "SUMMARY routes go=1 contract=1 bff=1;") || !strings.Contains(out, "UNRESOLVED EXACT /v1/factory-oa1") || !strings.Contains(out, "UNRESOLVED MOUNT /v1/factory-oa1") {
		t.Fatalf("real pool constructor callback writing buyer configuration escaped gate: exit=%d %s", code, out)
	}
}

func TestGoFactoryOA1SharedEntropyCLI(t *testing.T) {
	call := reviewCLI(t)
	// The helper's rand.Read target is already global in trunk; only the call is added later.
	platform := strings.Replace(goFactoryOA1Platform, "func ValidateSameDatabase", "var entropy [8]byte\nfunc ValidateSameDatabase", 1)
	platform = strings.Replace(platform, "\t\tvar entropy [8]byte\n", "", 1)
	root, p := goFactoryOA1Legacy(t, call, goFactoryOA1Source, platform, nil)
	mustWrite(t, p, strings.Replace(goFactoryOA1Source, "_ = mainPool", goFactoryOA1Probe, 1))
	if code, out := call(root); code != 1 || !strings.Contains(out, "ERROR (TOUCHED) UNRESOLVED") || !strings.Contains(out, "SUMMARY routes go=1 contract=1 bff=1;") || !strings.Contains(out, "UNRESOLVED EXACT /v1/factory-oa1") || !strings.Contains(out, "UNRESOLVED MOUNT /v1/factory-oa1") {
		t.Fatalf("admission probe writing shared entropy escaped gate: exit=%d %s", code, out)
	}
}

func TestGoFactoryOA1SuccessContinuationCloseCLI(t *testing.T) {
	call := reviewCLI(t)
	// The handler already depends on this pool before the probe addition.
	source := strings.Replace(goFactoryOA1Source, "newBuyer(runtime, c)", "newBuyer(mainPool, c)", 1)
	source = strings.Replace(source, `w.Header().Set("X-Factory-Configuration", c.bffKey)`, `if err := pool.Ping(r.Context()); err != nil { http.NotFound(w, r); return }
  w.Header().Set("X-Factory-Configuration", c.bffKey)`, 1)
	root, p := goFactoryOA1Legacy(t, call, source, goFactoryOA1Platform, nil)
	// Closing after the loop affects the successful handler path, not an error-only return.
	probe := strings.TrimSuffix(goFactoryOA1Probe, "\t}") + "\t\tmainPool.Close()\n\t}"
	mustWrite(t, p, strings.Replace(source, "_ = mainPool", probe, 1))
	if code, out := call(root); code != 1 || !strings.Contains(out, "ERROR (TOUCHED) UNRESOLVED") || !strings.Contains(out, "SUMMARY routes go=1 contract=1 bff=1;") || !strings.Contains(out, "UNRESOLVED EXACT /v1/factory-oa1") || !strings.Contains(out, "UNRESOLVED MOUNT /v1/factory-oa1") {
		t.Fatalf("success-path pool close escaped admission gate: exit=%d %s", code, out)
	}
}

func TestGoFactoryOA1MutableErrorSentinelCLI(t *testing.T) {
	call := reviewCLI(t)
	// The initializer is nonnil, but this unchanged caller assignment invalidates that proof.
	source := strings.Replace(goFactoryOA1Source, "mux := http.NewServeMux()", "errBuyerConfig = nil\n mux := http.NewServeMux()", 1)
	root, p := goFactoryOA1Legacy(t, call, source, goFactoryOA1Platform, nil)
	mustWrite(t, p, strings.Replace(source, "_ = mainPool", goFactoryOA1Probe, 1))
	if code, out := call(root); code != 1 || !strings.Contains(out, "ERROR (TOUCHED) UNRESOLVED") || !strings.Contains(out, "SUMMARY routes go=1 contract=1 bff=1;") || !strings.Contains(out, "UNRESOLVED EXACT /v1/factory-oa1") || !strings.Contains(out, "UNRESOLVED MOUNT /v1/factory-oa1") {
		t.Fatalf("mutable nil error sentinel granted admission exclusion: exit=%d %s", code, out)
	}
}
