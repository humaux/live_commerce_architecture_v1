// Purpose: pin conservative trunk factory provenance and the PR54 review counterexamples.
// Depends on: real reviewCLI, existing Git/fixture/write helpers and Go stdlib; no service runs.
// Used by: CI-DRIFT-FACTORY CI and independent integrator acceptance.
// Invariant: legacy target coverage must exist before every candidate; no missing route can pass.
package main

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const trunkFactorySource = `package main
import (
 "context"
 "errors"
 "net/http"
)
type pool struct { route string }
type config struct { label string }
var globalLabel = "legacy"
func main() {
 mux := http.NewServeMux()
 mux.HandleFunc("GET /v1/admin/stores/{store_id}/widgets/{widget_id}", func(w http.ResponseWriter, r *http.Request) {})
 h, _, err := buildBuyer(context.Background(), &pool{route: "primary"}, &config{label: "legacy"})
 if err != nil { return }
 mux.Handle("/v1/trunk-factory", h)
}
func openPool() (*pool, error) { return &pool{route: "participant"}, nil }
func ValidateSameDatabase(ctx context.Context, first, second *pool) error {
 if err := ctx.Err(); err != nil { return err }
 if first == second { return errors.New("same participant") }
 return nil
}
func buildBuyer(ctx context.Context, mainPool *pool, c *config) (http.Handler, func(), error) {
 participant, err := openPool()
 if err != nil { return nil, nil, err }
 _ = participant // INSERT_PROBE
 h := constructBuyer(mainPool, c)
 return mountTrunkFactory(factoryHeader(h)), func() {}, nil
}
func constructBuyer(p *pool, c *config) http.Handler {
 return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  w.Header().Set("X-Pool", p.route)
  w.Header().Set("X-Config", c.label)
  w.Header().Set("X-Global", globalLabel)
 })
}
func factoryHeader(next http.Handler) http.Handler {
 return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  w.Header().Set("X-Middleware", "legacy")
  next.ServeHTTP(w, r)
 })
}
func mountTrunkFactory(child http.Handler) http.Handler {
 return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  if r.URL.Path == "/v1/trunk-factory" { child.ServeHTTP(w, r); return }
  http.NotFound(w, r)
 })
}
`

const trunkFactoryProbe = `if err := ValidateSameDatabase(ctx, mainPool, participant); err != nil { return nil, nil, err }`

// Syntax checks run in CI with the test, before Git or scanner assertions. These
// sources are scanner input; pgx callbacks/contexts are never executed by this test.
func trunkFactoryWrite(t *testing.T, root, path, source string) string {
	t.Helper()
	if strings.HasSuffix(path, ".go") {
		if _, err := parser.ParseFile(token.NewFileSet(), path, source, parser.AllErrors); err != nil {
			t.Fatalf("invalid fixture %s: %v", path, err)
		}
	}
	p := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, p, source)
	return p
}

func trunkFactoryReplace(t *testing.T, source, old, next string) string {
	t.Helper()
	if strings.Count(source, old) != 1 {
		t.Fatalf("fixture seam %q must occur exactly once", old)
	}
	return mustReplace(t, source, old, next)
}

func trunkFactoryAssert(t *testing.T, call func(string, ...string) (int, string), root string, touched bool, exact string) {
	t.Helper()
	code, out := call(root)
	wantCode, prefix := 0, "WARNING (unresolved) "
	if touched {
		wantCode, prefix = 1, "ERROR (TOUCHED) "
	}
	for _, want := range []string{
		"SUMMARY routes go=1 contract=1 bff=1;",
		prefix + "UNRESOLVED EXACT " + exact + " ",
		prefix + "UNRESOLVED MOUNT /v1/trunk-factory ",
		"opaque Go handler dispatch",
		"cmd/api/main.go:",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("coverage/provenance missing %q: exit=%d %s", want, code, out)
		}
	}
	if code != wantCode || (!touched && strings.Contains(out, "TOUCHED")) {
		t.Errorf("factory gate exit=%d want=%d touched=%t: %s", code, wantCode, touched, out)
	}
	if t.Failed() {
		t.FailNow() // A broken baseline is setup failure, never candidate evidence.
	}
}

func trunkFactoryLegacy(t *testing.T, call func(string, ...string) (int, string), source string, extra map[string]string) string {
	t.Helper()
	root := fixture(t, "aligned")
	trunkFactoryWrite(t, root, "go.mod", "module example\n\ngo 1.22\n")
	trunkFactoryWrite(t, root, "cmd/api/main.go", source)
	for path, body := range extra {
		trunkFactoryWrite(t, root, path, body)
	}
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "legacy factory target with proven scanner coverage")
	git(t, root, "tag", "-f", "trunk")
	trunkFactoryAssert(t, call, root, false, "/v1/trunk-factory")
	return root
}

func TestGoFactoryTrunkMustTouchCLI(t *testing.T) {
	call := reviewCLI(t)
	cases := []struct{ name, probe string }{
		{"conditional mount", `if ValidateSameDatabase(ctx, mainPool, participant) == nil { mux := http.NewServeMux(); mux.Handle("/v1/trunk-factory", constructBuyer(mainPool, c)); return mountTrunkFactory(mux), func() {}, nil }`},
		{"different handler returned early", `if ValidateSameDatabase(ctx, mainPool, participant) != nil { return http.NotFoundHandler(), func() {}, nil }`},
		{"defer", `defer ValidateSameDatabase(ctx, mainPool, participant)`},
		{"go", `go func() { _ = ValidateSameDatabase(ctx, mainPool, participant) }()`},
		{"pool swapped after probe", trunkFactoryProbe + `; mainPool = participant`},
		{"field write", trunkFactoryProbe + `; mainPool.route = "changed"`},
		{"global write", trunkFactoryProbe + `; globalLabel = "changed"`},
		{"struct mutated then passed on", trunkFactoryProbe + `; c.label = "changed"`},
		{"method value", `check := ctx.Err; if err := check(); err != nil { return nil, nil, err }; ` + trunkFactoryProbe},
		{"captured probe error", `err = ValidateSameDatabase(ctx, mainPool, participant); if err != nil { return nil, nil, err }; defer func() { if err != nil { c.label = "changed" } }()`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := trunkFactoryLegacy(t, call, trunkFactorySource, nil)
			candidate := trunkFactoryReplace(t, trunkFactorySource, "_ = participant // INSERT_PROBE", tc.probe)
			trunkFactoryWrite(t, root, "cmd/api/main.go", candidate)
			trunkFactoryAssert(t, call, root, true, "/v1/trunk-factory")
		})
	}
	for _, tc := range []struct{ name, old, next, exact string }{
		{"route edit", `r.URL.Path == "/v1/trunk-factory"`, `r.URL.Path == "/v1/trunk-factory/changed"`, "/v1/trunk-factory/changed"},
		{"middleware dependency", `"X-Middleware", "legacy"`, `"X-Middleware", "changed"`, "/v1/trunk-factory"},
		{"body dependency", `"X-Config", c.label`, `"X-Config", "changed"`, "/v1/trunk-factory"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := trunkFactoryLegacy(t, call, trunkFactorySource, nil)
			trunkFactoryWrite(t, root, "cmd/api/main.go", trunkFactoryReplace(t, trunkFactorySource, tc.old, tc.next))
			trunkFactoryExactTouched(t, call, root, tc.exact)
		})
	}
	for _, alias := range []string{"web", "."} {
		t.Run("probe import "+alias, func(t *testing.T) {
			source := trunkFactoryReplace(t, trunkFactoryOriginSource, "// CALLER_ORIGIN", trunkFactoryOriginCall)
			source = trunkFactoryReplace(t, source, `"example/internal/platform"`, alias+` "example/internal/platform"`)
			prefix := alias + "."
			if alias == "." {
				prefix = ""
			}
			source = mustReplaceAll(t, source, "platform.", prefix)
			root := trunkFactoryLegacy(t, call, source, map[string]string{"internal/platform/runtime.go": trunkFactoryPlatform})
			probe := `if err := ` + prefix + `ValidateSameDatabase(ctx, mainPool, participant); err != nil { return nil, nil, errIdentity }`
			trunkFactoryWrite(t, root, "cmd/api/main.go", trunkFactoryReplace(t, source, "_ = participant // INSERT_PROBE", probe))
			trunkFactoryAssert(t, call, root, true, "/v1/trunk-factory")
		})
	}
}

// This function is copied literally from OA1 source316848601032b434885dd513b11f0a8871ef57e5,
// internal/platform/meta_runtime.go blob76bd790da74f8de474247f080c3bed10288088fa.
// Its calls to pool.Begin and ctx's methods are why unchanged caller origins matter.
const trunkFactoryPlatform = `package platform
import (
 "context"
 "crypto/rand"
 "encoding/binary"
 "errors"
 "time"
 "github.com/jackc/pgx/v5"
 "github.com/jackc/pgx/v5/pgxpool"
)
func ValidateSameDatabase(ctx context.Context, first, second *pgxpool.Pool) (err error) {
	denied := errors.New("database identity unavailable")
	if ctx == nil || first == nil || second == nil {
		return denied
	}
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var key int64
	for key == 0 {
		var entropy [8]byte
		if _, e := rand.Read(entropy[:]); e != nil {
			return denied
		}
		key = int64(binary.BigEndian.Uint64(entropy[:]))
	}
	// Rollback must work after caller cancellation, release the borrowed pool
	// connection, and never silently turn a cleanup failure into startup success.
	cleanup := func(tx pgx.Tx) {
		rollbackCtx, stop := context.WithTimeout(context.Background(), 2*time.Second)
		defer stop()
		if e := tx.Rollback(rollbackCtx); e != nil {
			err = denied
		}
	}
	a, e := first.Begin(bounded)
	if e != nil {
		return denied
	}
	defer cleanup(a)
	var acquired bool
	if e = a.QueryRow(bounded, "SELECT pg_catalog.pg_try_advisory_xact_lock($1::bigint)", key).Scan(&acquired); e != nil || !acquired {
		return denied
	}
	b, e := second.Begin(bounded)
	if e != nil {
		return denied
	}
	defer cleanup(b)
	if e = b.QueryRow(bounded, "SELECT pg_catalog.pg_try_advisory_xact_lock($1::bigint)", key).Scan(&acquired); e != nil || acquired {
		return denied
	}
	return nil
}
`

const trunkFactoryOriginSource = `package main
import (
 "context"
 "errors"
 "net/http"
 "time"
 "example/internal/platform"
 "github.com/jackc/pgx/v5"
 "github.com/jackc/pgx/v5/pgxpool"
)
type config struct { enabled bool; label string }
var errIdentity = errors.New("database identity unavailable")
type routingContext struct { context.Context; c *config }
func (c *routingContext) Deadline() (time.Time, bool) { c.c.label = "context-changed"; return c.Context.Deadline() }
func (c *routingContext) Err() error { c.c.label = "context-changed"; return c.Context.Err() }
func makePool(ctx context.Context, c *config) (*pgxpool.Pool, error) {
 cfg, err := pgxpool.ParseConfig("postgres://fixture.invalid/fixture")
 if err != nil { return nil, err }
 cfg.BeforeAcquire = func(ctx context.Context, conn *pgx.Conn) bool { c.label = "hook-changed"; return true }
 return pgxpool.NewWithConfig(ctx, cfg)
}
func main() {
 c := &config{enabled: true, label: "legacy"}
 mux := http.NewServeMux()
 mux.HandleFunc("GET /v1/admin/stores/{store_id}/widgets/{widget_id}", func(w http.ResponseWriter, r *http.Request) {})
 ctx := context.Background()
 mainPool := &pgxpool.Pool{}
 _ = mainPool
 _ = platform.ValidateSameDatabase
 hooked, err := makePool(ctx, c)
 if err != nil { return }
 _ = hooked
 // CALLER_ORIGIN
}
func buildBuyer(ctx context.Context, c *config, mainPool *pgxpool.Pool) (http.Handler, func(), error) {
 participant := &pgxpool.Pool{}
 _ = participant // INSERT_PROBE
 return mountTrunkFactory(constructBuyer(mainPool, c)), func() {}, nil
}
func constructBuyer(p *pgxpool.Pool, c *config) http.Handler {
 return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  if err := p.Ping(r.Context()); err != nil { http.NotFound(w, r); return }
  w.Header().Set("X-Config", c.label)
 })
}
func mountTrunkFactory(child http.Handler) http.Handler {
 return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  if r.URL.Path == "/v1/trunk-factory" { child.ServeHTTP(w, r); return }
  http.NotFound(w, r)
 })
}
`

const trunkFactoryOriginCall = `h, _, err := buildBuyer(ctx, c, mainPool)
 if err != nil { return }
 mux.Handle("/v1/trunk-factory", h)`

func TestGoFactoryTrunkCallerOriginCLI(t *testing.T) {
	call := reviewCLI(t)
	cases := []struct{ name, caller, extra string }{
		{"pool nested assignment", `if c.enabled { mainPool = hooked; ` + trunkFactoryOriginCall + ` }`, ""},
		{"pool nested short declaration", `if c.enabled { mainPool, err := makePool(ctx, c); if err != nil { return }; ` + trunkFactoryOriginCall + ` }`, ""},
		{"pool range variable", `for _, mainPool := range []*pgxpool.Pool{hooked} { ` + trunkFactoryOriginCall + ` }`, ""},
		{"pool FuncLit parameter", `func(mainPool *pgxpool.Pool) { _, _, err := buildBuyer(ctx, c, mainPool); if err != nil { return } }(hooked)
 ` + trunkFactoryOriginCall, ""},
		{"context nested assignment", `if c.enabled { ctx = &routingContext{Context: ctx, c: c}; ` + trunkFactoryOriginCall + ` }`, ""},
		{"context nested short declaration", `if c.enabled { ctx := &routingContext{Context: ctx, c: c}; ` + trunkFactoryOriginCall + ` }`, ""},
		{"context range variable", `for _, ctx := range []context.Context{&routingContext{Context: ctx, c: c}} { ` + trunkFactoryOriginCall + ` }`, ""},
		{"context FuncLit parameter", `func(ctx context.Context) { _, _, err := buildBuyer(ctx, c, mainPool); if err != nil { return } }(&routingContext{Context: ctx, c: c})
 ` + trunkFactoryOriginCall, ""},
		{"pool function value", `safe, _, err := buildBuyer(ctx, c, mainPool); if err != nil { return }; _ = safe
 run := buildBuyer
 h, _, err := run(ctx, c, hooked); if err != nil { return }; mux.Handle("/v1/trunk-factory", h)`, ""},
		{"context function value", `safe, _, err := buildBuyer(ctx, c, mainPool); if err != nil { return }; _ = safe
 run := buildBuyer
 h, _, err := run(&routingContext{Context: ctx, c: c}, c, mainPool); if err != nil { return }; mux.Handle("/v1/trunk-factory", h)`, ""},
		{"pool GenDecl initializer", trunkFactoryOriginCall, `var cfg0 = &config{enabled: true, label: "legacy"}
var ctx0 = context.Background()
var hooked0, _ = makePool(ctx0, cfg0)
var _ = func() bool { _, _, _ = buildBuyer(ctx0, cfg0, hooked0); return true }()
`},
		{"context GenDecl initializer", trunkFactoryOriginCall, `var cfg0 = &config{enabled: true, label: "legacy"}
var ctx0 = &routingContext{Context: context.Background(), c: cfg0}
var pool0 = &pgxpool.Pool{}
var _ = func() bool { _, _, _ = buildBuyer(ctx0, cfg0, pool0); return true }()
`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// All caller effects and indirect uses predate the diff. The only
			// candidate edit is the probe inside the factory, never caller code.
			source := trunkFactoryReplace(t, trunkFactoryOriginSource, "// CALLER_ORIGIN", tc.caller) + tc.extra
			if tc.extra != "" {
				// Package initialization and the mounted handler share the same
				// configuration, so the hidden callback changes served output.
				source = trunkFactoryReplace(t, source, `c := &config{enabled: true, label: "legacy"}`, `c := cfg0`)
			}
			root := trunkFactoryLegacy(t, call, source, map[string]string{"internal/platform/runtime.go": trunkFactoryPlatform})
			probe := `if err := platform.ValidateSameDatabase(ctx, mainPool, participant); err != nil { return nil, nil, errIdentity }`
			trunkFactoryWrite(t, root, "cmd/api/main.go", trunkFactoryReplace(t, source, "_ = participant // INSERT_PROBE", probe))
			trunkFactoryAssert(t, call, root, true, "/v1/trunk-factory")
		})
	}
}
