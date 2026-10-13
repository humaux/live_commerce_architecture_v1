// Purpose: reproduce OA1's imported PGx startup probe with frozen source through the real CLI.
// Depends on: reviewCLI, Git fixture helpers and stdlib; fixture PGx source is parsed, never executed.
// Used by: CI-DRIFT-FACTORY CI regression and independent parent acceptance.
// Invariant: only adding a proven startup probe is safe; cleanup and foreign receiver effects stay touched.
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Frozen source: 316848601032b434885dd513b11f0a8871ef57e5.
// internal/platform/meta_runtime.go blob 76bd790da74f8de474247f080c3bed10288088fa;
// cmd/api/buyer.go blob d78307d1eb1e80a6f5740ecae4be0064bf28c5dd.
// ValidateSameDatabase below is the complete literal function from that source.
const goFactoryOA1Platform = `package platform
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
func OpenBuyerIssuerPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) { return &pgxpool.Pool{}, nil }
func OpenBuyerPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) { return &pgxpool.Pool{}, nil }
func OpenCheckoutPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) { return &pgxpool.Pool{}, nil }
`

const goFactoryOA1Source = `package main
import (
 "context"
 "errors"
 "net/http"
 "github.com/jackc/pgx/v5/pgxpool"
 _ "example/internal/httpapi"
 "example/internal/platform"
)
type buyerConfig struct { enabled bool; bffKey string }
type cvsParts struct{}
var errBuyerConfig = errors.New("invalid buyer configuration")
func main() {
 c := &buyerConfig{enabled: true, bffKey: "fixture"}
 mux := http.NewServeMux()
 h, _, _, err := buildBuyerWithCVS(context.Background(), c, &pgxpool.Pool{})
 if err != nil { return }
 mux.Handle("/v1/factory-oa1", h)
}
func buildBuyerWithCVS(ctx context.Context, c *buyerConfig, mainPool *pgxpool.Pool) (http.Handler, cvsParts, func(), error) {
 if !c.enabled { return nil, cvsParts{}, func() {}, nil }
 issuer, err := platform.OpenBuyerIssuerPool(ctx, "issuer")
 if err != nil { return nil, cvsParts{}, nil, errBuyerConfig }
 runtime, err := platform.OpenBuyerPool(ctx, "runtime")
 if err != nil { issuer.Close(); return nil, cvsParts{}, nil, errBuyerConfig }
 checkoutPool, err := platform.OpenCheckoutPool(ctx, "checkout")
 if err != nil { runtime.Close(); issuer.Close(); return nil, cvsParts{}, nil, errBuyerConfig }
 var hostedPool *pgxpool.Pool
 closePools := func() {
  if hostedPool != nil { hostedPool.Close() }
  checkoutPool.Close()
  runtime.Close()
  issuer.Close()
 }
 _ = mainPool
 h, err := newBuyer(runtime, c)
 if err != nil { closePools(); return nil, cvsParts{}, nil, errBuyerConfig }
 return mountOA1(h), cvsParts{}, closePools, nil
}
func newBuyer(pool *pgxpool.Pool, c *buyerConfig) (http.Handler, error) {
 return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  w.Header().Set("X-Factory-Configuration", c.bffKey)
 }), nil
}
func mountOA1(child http.Handler) http.Handler {
 return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  if r.URL.Path == "/v1/factory-oa1" { child.ServeHTTP(w, r); return }
  http.NotFound(w, r)
 })
}
`

// Literal call block from cmd/api/buyer.go at the frozen source SHA above.
const goFactoryOA1Probe = `if mainPool != nil {
		for _, participant := range []*pgxpool.Pool{issuer, runtime, checkoutPool} {
			if err = platform.ValidateSameDatabase(ctx, mainPool, participant); err != nil {
				closePools()
				return nil, cvsParts{}, nil, errBuyerConfig
			}
		}
	}`

func goFactoryOA1Legacy(t *testing.T, call func(string, ...string) (int, string), source, platform string, extra map[string]string) (string, string) {
	t.Helper()
	root := fixture(t, "aligned")
	files := map[string]string{"go.mod": "module example\n\ngo 1.22\n"}
	files["cmd/api/main.go"] = source
	files["internal/platform/runtime.go"] = platform
	for path, body := range extra {
		files[path] = body
	}
	for path, body := range files {
		p := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			t.Fatal(err)
		}
		mustWrite(t, p, body)
	}
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "frozen OA1 probe and legacy factory")
	git(t, root, "tag", "-f", "trunk")
	if code, out := call(root); code != 0 || !strings.Contains(out, "SUMMARY routes go=1 contract=1 bff=1;") || !strings.Contains(out, "WARNING (unresolved) UNRESOLVED EXACT /v1/factory-oa1") || !strings.Contains(out, "WARNING (unresolved) UNRESOLVED MOUNT /v1/factory-oa1") {
		t.Fatalf("OA1 legacy coverage or reachable aligned routes lost: exit=%d %s", code, out)
	}
	return root, filepath.Join(root, "cmd/api/main.go")
}

func TestGoFactoryOA1FrozenDatabaseProbeCLI(t *testing.T) {
	call := reviewCLI(t)
	root, p := goFactoryOA1Legacy(t, call, goFactoryOA1Source, goFactoryOA1Platform, nil)
	mustWrite(t, p, strings.Replace(goFactoryOA1Source, "_ = mainPool", goFactoryOA1Probe, 1))
	if code, out := call(root); code != 0 || strings.Contains(out, "TOUCHED") || !strings.Contains(out, "SUMMARY routes go=1 contract=1 bff=1;") || !strings.Contains(out, "WARNING (unresolved) UNRESOLVED EXACT /v1/factory-oa1") || !strings.Contains(out, "WARNING (unresolved) UNRESOLVED MOUNT /v1/factory-oa1") {
		t.Fatalf("frozen OA1 startup probe touched stable handler dependencies: exit=%d %s", code, out)
	}
}

func TestGoFactoryOA1CleanupConfigurationCLI(t *testing.T) {
	call := reviewCLI(t)
	// The captured caller configuration write predates the probe call addition.
	source := strings.Replace(goFactoryOA1Source, "closePools := func() {", "closePools := func() {\n c.bffKey = \"changed\"", 1)
	root, p := goFactoryOA1Legacy(t, call, source, goFactoryOA1Platform, nil)
	mustWrite(t, p, strings.Replace(source, "_ = mainPool", goFactoryOA1Probe, 1))
	if code, out := call(root); code != 1 || !strings.Contains(out, "ERROR (TOUCHED) UNRESOLVED") {
		t.Fatalf("admission cleanup writing caller configuration escaped gate: exit=%d %s", code, out)
	}
}

func TestGoFactoryOA1ShadowedForeignPoolCLI(t *testing.T) {
	call := reviewCLI(t)
	foreign := `package foreignpool
import (
 "context"
 "github.com/jackc/pgx/v5"
)
type Pool struct { OnBegin func() }
func (p *Pool) Begin(ctx context.Context) (pgx.Tx, error) { p.OnBegin(); return nil, nil }
func (p *Pool) Close() {}
`
	// Identical selector/type spelling must not confer the real PGx effect contract.
	shadow := `pgxpool "example/internal/foreignpool"`
	source := strings.Replace(goFactoryOA1Source, `"github.com/jackc/pgx/v5/pgxpool"`, shadow, 1)
	source = strings.Replace(source, "&pgxpool.Pool{})", `&pgxpool.Pool{OnBegin: func() { c.bffKey = "changed" }})`, 1)
	platform := strings.Replace(goFactoryOA1Platform, `"github.com/jackc/pgx/v5/pgxpool"`, shadow, 1)
	root, p := goFactoryOA1Legacy(t, call, source, platform, map[string]string{"internal/foreignpool/pool.go": foreign})
	mustWrite(t, p, strings.Replace(source, "_ = mainPool", goFactoryOA1Probe, 1))
	if code, out := call(root); code != 1 || !strings.Contains(out, "ERROR (TOUCHED) UNRESOLVED") {
		t.Fatalf("shadowed foreign pool callback escaped admission gate: exit=%d %s", code, out)
	}
}
