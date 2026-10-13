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

func TestGoFactoryOA1CustomContextCLI(t *testing.T) {
	call := reviewCLI(t)
	source := strings.Replace(goFactoryOA1Source, "buildBuyerWithCVS(context.Background(), c,", "buildBuyerWithCVS(&routingContext{Context: context.Background(), c: c}, c,", 1)
	source += `
type routingContext struct { context.Context; c *buyerConfig }
func (ctx *routingContext) Err() error { ctx.c.bffKey = "changed"; return ctx.Context.Err() }
`
	// Context's interface type does not prove its method bodies are effect-free.
	platform := strings.Replace(goFactoryOA1Platform, "bounded, cancel := context.WithTimeout", "if ctx.Err() != nil { return denied }\n\tbounded, cancel := context.WithTimeout", 1)
	root, p := goFactoryOA1Legacy(t, call, source, platform, nil)
	mustWrite(t, p, strings.Replace(source, "_ = mainPool", goFactoryOA1Probe, 1))
	if code, out := call(root); code != 1 || !strings.Contains(out, "ERROR (TOUCHED) UNRESOLVED") || !strings.Contains(out, "SUMMARY routes go=1 contract=1 bff=1;") || !strings.Contains(out, "UNRESOLVED EXACT /v1/factory-oa1") || !strings.Contains(out, "UNRESOLVED MOUNT /v1/factory-oa1") {
		t.Fatalf("custom context method writing buyer configuration escaped gate: exit=%d %s", code, out)
	}
}

func TestGoFactoryOA1CustomScanDestinationCLI(t *testing.T) {
	call := reviewCLI(t)
	source := strings.Replace(goFactoryOA1Source, "mux := http.NewServeMux()", "platform.OnScan = func() { c.bffKey = \"changed\" }\n mux := http.NewServeMux()", 1)
	// The ordinary bool remains separate from this fresh destination's sql.Scanner hook.
	platform := strings.Replace(goFactoryOA1Platform, "var acquired bool", `var acquired bool
	destination := routingScan{}
	if e = a.QueryRow(bounded, "SELECT pg_catalog.pg_try_advisory_xact_lock($1::bigint)", key).Scan(&destination); e != nil { return denied }`, 1)
	platform += `
var OnScan func()
type routingScan struct{}
func (*routingScan) Scan(src any) error { OnScan(); return nil }
`
	root, p := goFactoryOA1Legacy(t, call, source, platform, nil)
	mustWrite(t, p, strings.Replace(source, "_ = mainPool", goFactoryOA1Probe, 1))
	if code, out := call(root); code != 1 || !strings.Contains(out, "ERROR (TOUCHED) UNRESOLVED") || !strings.Contains(out, "SUMMARY routes go=1 contract=1 bff=1;") || !strings.Contains(out, "UNRESOLVED EXACT /v1/factory-oa1") || !strings.Contains(out, "UNRESOLVED MOUNT /v1/factory-oa1") {
		t.Fatalf("fresh custom scan destination callback escaped gate: exit=%d %s", code, out)
	}
}

func TestGoFactoryOA1NestedPoolReturnCLI(t *testing.T) {
	call := reviewCLI(t)
	source := strings.Replace(goFactoryOA1Source, `"github.com/jackc/pgx/v5/pgxpool"`, `"github.com/jackc/pgx/v5"
 "github.com/jackc/pgx/v5/pgxpool"`, 1)
	source = strings.Replace(source, "h, _, _, err := buildBuyerWithCVS(context.Background(), c, &pgxpool.Pool{})", `ctx := context.Background()
 seed, err := callbackPool(ctx, c)
 if err != nil { return }
 sharedPool = seed
 mainPool, err := makePool(ctx, c)
 if err != nil { return }
 h, _, _, err := buildBuyerWithCVS(ctx, c, mainPool)`, 1)
	// A safe final return cannot erase the nested return of a callback-configured global pool.
	source += `
var sharedPool *pgxpool.Pool
func callbackPool(ctx context.Context, c *buyerConfig) (*pgxpool.Pool, error) {
 cfg, err := pgxpool.ParseConfig("fixture")
 if err != nil { return nil, err }
 cfg.BeforeAcquire = func(context.Context, *pgx.Conn) bool { c.bffKey = "changed"; return true }
 return pgxpool.NewWithConfig(ctx, cfg)
}
func makePool(ctx context.Context, c *buyerConfig) (*pgxpool.Pool, error) {
 if c.enabled { if c.bffKey != "" { return sharedPool, nil } }
 cfg, err := pgxpool.ParseConfig("fixture")
 if err != nil { return nil, err }
 return pgxpool.NewWithConfig(ctx, cfg)
}
`
	root, p := goFactoryOA1Legacy(t, call, source, goFactoryOA1Platform, nil)
	mustWrite(t, p, strings.Replace(source, "_ = mainPool", goFactoryOA1Probe, 1))
	if code, out := call(root); code != 1 || !strings.Contains(out, "ERROR (TOUCHED) UNRESOLVED") || !strings.Contains(out, "SUMMARY routes go=1 contract=1 bff=1;") || !strings.Contains(out, "UNRESOLVED EXACT /v1/factory-oa1") || !strings.Contains(out, "UNRESOLVED MOUNT /v1/factory-oa1") {
		t.Fatalf("nested callback-configured pool return escaped gate: exit=%d %s", code, out)
	}
}

func TestGoFactoryOA1DeferredErrorClearCLI(t *testing.T) {
	call := reviewCLI(t)
	source := strings.Replace(goFactoryOA1Source, "newBuyer(runtime, c)", "newBuyer(mainPool, c)", 1)
	source = strings.Replace(source, `w.Header().Set("X-Factory-Configuration", c.bffKey)`, `if err := pool.Ping(r.Context()); err != nil { http.NotFound(w, r); return }
  w.Header().Set("X-Factory-Configuration", c.bffKey)`, 1)
	platform := goFactoryOA1Platform + `
func CloseThenClearError(ctx context.Context, first, second *pgxpool.Pool) (err error) {
 clear := func() { err = nil }
 defer clear()
 if first != nil { first.Close(); return errors.New("startup pool closed") }
 return nil
}
`
	root, p := goFactoryOA1Legacy(t, call, source, platform, nil)
	probe := strings.ReplaceAll(goFactoryOA1Probe, "ValidateSameDatabase", "CloseThenClearError")
	mustWrite(t, p, strings.Replace(source, "_ = mainPool", probe, 1))
	if code, out := call(root); code != 1 || !strings.Contains(out, "ERROR (TOUCHED) UNRESOLVED") || !strings.Contains(out, "SUMMARY routes go=1 contract=1 bff=1;") || !strings.Contains(out, "UNRESOLVED EXACT /v1/factory-oa1") || !strings.Contains(out, "UNRESOLVED MOUNT /v1/factory-oa1") {
		t.Fatalf("deferred named error clear hid successful pool close: exit=%d %s", code, out)
	}
}

func TestGoFactoryOA1CrossFileBoolScannerCLI(t *testing.T) {
	call := reviewCLI(t)
	source := strings.Replace(goFactoryOA1Source, "mux := http.NewServeMux()", "platform.OnScan = func() { c.bffKey = \"changed\" }\n mux := http.NewServeMux()", 1)
	// bool resolves across files to a custom scanner; acquired uses admitted builtin int64.
	platform := strings.Replace(goFactoryOA1Platform, "var acquired bool", `var acquired int64
	var destination bool
	if e = a.QueryRow(bounded, "SELECT pg_catalog.pg_try_advisory_xact_lock($1::bigint)", key).Scan(&destination); e != nil { return denied }`, 1)
	platform = strings.Replace(platform, "!acquired", "acquired == 0", 1)
	platform = strings.Replace(platform, "|| acquired {", "|| acquired != 0 {", 1)
	crossFile := `package platform
type bool struct{}
var OnScan func()
func (*bool) Scan(src any) error { OnScan(); return nil }
`
	root, p := goFactoryOA1Legacy(t, call, source, platform, map[string]string{"internal/platform/scanner.go": crossFile})
	mustWrite(t, p, strings.Replace(source, "_ = mainPool", goFactoryOA1Probe, 1))
	if code, out := call(root); code != 1 || !strings.Contains(out, "ERROR (TOUCHED) UNRESOLVED") || !strings.Contains(out, "SUMMARY routes go=1 contract=1 bff=1;") || !strings.Contains(out, "UNRESOLVED EXACT /v1/factory-oa1") || !strings.Contains(out, "UNRESOLVED MOUNT /v1/factory-oa1") {
		t.Fatalf("cross-file bool scanner callback escaped gate: exit=%d %s", code, out)
	}
}

func TestGoFactoryOA1InitializerClearedErrorCLI(t *testing.T) {
	call := reviewCLI(t)
	source := goFactoryOA1Source + `
var resetBuyerError = func() bool { errBuyerConfig = nil; return true }()
`
	root, p := goFactoryOA1Legacy(t, call, source, goFactoryOA1Platform, nil)
	mustWrite(t, p, strings.Replace(source, "_ = mainPool", goFactoryOA1Probe, 1))
	if code, out := call(root); code != 1 || !strings.Contains(out, "ERROR (TOUCHED) UNRESOLVED") || !strings.Contains(out, "SUMMARY routes go=1 contract=1 bff=1;") || !strings.Contains(out, "UNRESOLVED EXACT /v1/factory-oa1") || !strings.Contains(out, "UNRESOLVED MOUNT /v1/factory-oa1") {
		t.Fatalf("package initializer clearing error sentinel granted admission exclusion: exit=%d %s", code, out)
	}
}

func TestGoFactoryOA1ActualPoolConstructorCLI(t *testing.T) {
	call := reviewCLI(t)
	source := strings.Replace(goFactoryOA1Source, "h, _, _, err := buildBuyerWithCVS(context.Background(), c, &pgxpool.Pool{})", `ctx := context.Background()
 mainPool, err := platform.OpenPool(ctx, "fixture")
 if err != nil { return }
 h, _, _, err := buildBuyerWithCVS(ctx, c, mainPool)`, 1)
	platform := strings.Replace(goFactoryOA1Platform, `"errors"`, `"errors"
 "fmt"
 "strings"`, 1)
	platform = strings.Replace(platform, "return &pgxpool.Pool{}, nil", `return openPool(ctx, dsn, "buyer_issuer")`, 1)
	platform = strings.Replace(platform, "return &pgxpool.Pool{}, nil", `return openPool(ctx, dsn, "buyer_runtime")`, 1)
	platform = strings.Replace(platform, "return &pgxpool.Pool{}, nil", `return openPool(ctx, dsn, "checkout_runtime")`, 1)
	// Literal openPool from root platform.go blob 4b5dc1b531cefc9a04c6dd00f72221df37da9a57.
	// Authority validation is a parsed-only stub; this gate proves constructor compatibility, not PG authority.
	platform += `
const startupTimeout = 2 * time.Second
func OpenPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) { return openPool(ctx, dsn, "runtime") }
func validatePoolAuthority(ctx context.Context, pool *pgxpool.Pool, authority string) error { return nil }
func openPool(ctx context.Context, dsn string, authority string) (*pgxpool.Pool, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, errors.New("database url required")
	}
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse database config: %w", err)
	}
	config.MaxConns = 8

	startup, cancel := context.WithTimeout(ctx, startupTimeout)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(startup, config)
	if err != nil {
		return nil, fmt.Errorf("open database pool: %w", err)
	}
	if err := pool.Ping(startup); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database unavailable: %w", err)
	}
	if err := validatePoolAuthority(startup, pool, authority); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}
`
	root, p := goFactoryOA1Legacy(t, call, source, platform, nil)
	mustWrite(t, p, strings.Replace(source, "_ = mainPool", goFactoryOA1Probe, 1))
	if code, out := call(root); code != 0 || strings.Contains(out, "TOUCHED") || !strings.Contains(out, "SUMMARY routes go=1 contract=1 bff=1;") || !strings.Contains(out, "WARNING (unresolved) UNRESOLVED EXACT /v1/factory-oa1") || !strings.Contains(out, "WARNING (unresolved) UNRESOLVED MOUNT /v1/factory-oa1") {
		t.Fatalf("actual pool constructor startup probe changed coverage: exit=%d %s", code, out)
	}
}

func TestGoFactoryOA1NamedFactoryErrorCLI(t *testing.T) {
	call := reviewCLI(t)
	source := strings.Replace(goFactoryOA1Source, "(http.Handler, cvsParts, func(), error) {", `(handler http.Handler, parts cvsParts, cleanup func(), err error) {
 clear := func() { err = nil }
 defer clear()`, 1)
	root, p := goFactoryOA1Legacy(t, call, source, goFactoryOA1Platform, nil)
	mustWrite(t, p, strings.Replace(source, "_ = mainPool", goFactoryOA1Probe, 1))
	if code, out := call(root); code != 1 || !strings.Contains(out, "ERROR (TOUCHED) UNRESOLVED") || !strings.Contains(out, "SUMMARY routes go=1 contract=1 bff=1;") || !strings.Contains(out, "UNRESOLVED EXACT /v1/factory-oa1") || !strings.Contains(out, "UNRESOLVED MOUNT /v1/factory-oa1") {
		t.Fatalf("named factory error cleared by deferred closure granted admission exclusion: exit=%d %s", code, out)
	}
}
