// Purpose: separate startup-only factory validation from handler dependencies through the real CLI.
// Depends on: Go stdlib, reviewCLI and existing Git-backed fixture/write helpers; no service execution.
// Used by: CI-DRIFT-FACTORY CI regression gate and the parent integrator's independent rerun.
// Invariant: unchanged legacy coverage warns; construction, route and mutable dependencies stay touched.
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const goFactorySource = `package main
import (
 "context"
 "errors"
 "net/http"
)
type pool struct { route string }
func main() {
 mux := http.NewServeMux()
 buyer, _, err := buildBuyerWithCVS(context.Background(), &pool{route: "primary"})
 if err != nil { return }
 mux.Handle("/v1/factory", buyer)
}
func openPool() (*pool, error) { return &pool{route: "participant"}, nil }
func buildBuyerWithCVS(ctx context.Context, mainPool *pool) (http.Handler, func(), error) {
 authPool, err := openPool()
 if err != nil { return nil, nil, err }
 workerPool, err := openPool()
 if err != nil { return nil, nil, err }
 participants := []*pool{authPool, workerPool}
 _ = participants
 svc, err := constructBuyer(mainPool)
 if err != nil { return nil, nil, err }
 return mountFactory(factoryMiddleware(svc)), func() {}, nil
}
func ValidateSameDatabase(ctx context.Context, main, participant *pool) error {
 if err := ctx.Err(); err != nil { return err }
 if main == participant { return errors.New("same participant") }
 return nil
}
func checkedPool(ctx context.Context, main, participant *pool) (*pool, error) {
 if err := ValidateSameDatabase(ctx, main, participant); err != nil { return nil, err }
 return participant, nil
}
func configurePool(p *pool, callback func(*pool)) { callback(p) }
func constructBuyer(p *pool) (http.Handler, error) {
 return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  w.Header().Set("X-Factory-Pool", p.route)
 }), nil
}
func factoryMiddleware(next http.Handler) http.Handler {
 return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  w.Header().Set("X-Factory-Middleware", "legacy")
  next.ServeHTTP(w, r)
 })
}
func mountFactory(child http.Handler) http.Handler {
 return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  if r.URL.Path == "/v1/factory" { child.ServeHTTP(w, r); return }
  http.NotFound(w, r)
 })
}
`

const goFactoryStartupProbe = `for _, participant := range participants {
  if err := ValidateSameDatabase(ctx, mainPool, participant); err != nil {
   return nil, nil, err
  }
 }`

func goFactoryLegacy(t *testing.T, call func(string, ...string) (int, string), source string) (string, string) {
	t.Helper()
	root := fixture(t, "aligned")
	p := filepath.Join(root, "cmd/api/main.go")
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, p, source)
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "legacy opaque factory")
	git(t, root, "tag", "-f", "trunk")
	if code, out := call(root); code != 0 || !strings.Contains(out, "WARNING (unresolved) UNRESOLVED EXACT /v1/factory") || !strings.Contains(out, "WARNING (unresolved) UNRESOLVED MOUNT /v1/factory") {
		t.Fatalf("legacy custom factory coverage absent: exit=%d %s", code, out)
	}
	return root, p
}

func TestGoFactoryStartupProbeCLI(t *testing.T) {
	call := reviewCLI(t)
	for _, factoryName := range []string{"buildBuyerWithCVS", "assembleCheckout"} {
		t.Run(factoryName, func(t *testing.T) {
			source := strings.ReplaceAll(goFactorySource, "buildBuyerWithCVS", factoryName)
			root, p := goFactoryLegacy(t, call, source)
			mustWrite(t, p, strings.Replace(source, "_ = participants", goFactoryStartupProbe, 1))
			code, out := call(root)
			if code != 0 || strings.Contains(out, "TOUCHED") || !strings.Contains(out, "WARNING (unresolved) UNRESOLVED EXACT /v1/factory") || !strings.Contains(out, "WARNING (unresolved) UNRESOLVED MOUNT /v1/factory") {
				t.Fatalf("startup-only validation touched unchanged handler dependencies: exit=%d %s", code, out)
			}
		})
	}
}

func TestGoFactoryHandlerDependenciesCLI(t *testing.T) {
	call := reviewCLI(t)
	for _, tc := range []struct{ name, before, after string }{
		{"probe output", "_ = participants", `mainPool, err = checkedPool(ctx, mainPool, authPool)
 if err != nil { return nil, nil, err }`},
		{"route", `r.URL.Path == "/v1/factory"`, `r.URL.Path == "/v1/factory/changed"`},
		{"middleware", `"X-Factory-Middleware", "legacy"`, `"X-Factory-Middleware", "changed"`},
		{"child dependency comment", `"X-Factory-Pool", p.route`, `"X-Factory-Pool", p.route /* changed dependency */`},
		{"pool alias mutation", "_ = participants", `alias := mainPool
 alias.route = "changed"`},
		{"callback mutation", "_ = participants", `configurePool(mainPool, func(p *pool) { p.route = "changed" })`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, p := goFactoryLegacy(t, call, goFactorySource)
			changed := strings.Replace(goFactorySource, tc.before, tc.after, 1)
			if changed == goFactorySource {
				t.Fatal("mutation did not change fixture")
			}
			mustWrite(t, p, changed)
			if code, out := call(root); code != 1 || !strings.Contains(out, "ERROR (TOUCHED) UNRESOLVED") {
				t.Fatalf("handler-affecting %s escaped legacy gate: exit=%d %s", tc.name, code, out)
			}
		})
	}
}

func TestGoFactorySharedStateCLI(t *testing.T) {
	call := reviewCLI(t)
	source := strings.Replace(goFactorySource, "type pool struct", "var factoryLabel = \"legacy\"\ntype pool struct", 1)
	source = strings.Replace(source, `"X-Factory-Pool", p.route`, `"X-Factory-Pool", factoryLabel`, 1)
	root, p := goFactoryLegacy(t, call, source)
	mustWrite(t, p, strings.Replace(source, "_ = participants", `factoryLabel = "changed"`, 1))
	if code, out := call(root); code != 1 || !strings.Contains(out, "ERROR (TOUCHED) UNRESOLVED") {
		t.Fatalf("shared-state startup mutation escaped gate: exit=%d %s", code, out)
	}
}

func TestGoFactoryMuxMutationCLI(t *testing.T) {
	call := reviewCLI(t)
	source := strings.Replace(goFactorySource, "_ = participants", "_ = participants\n childMux := http.NewServeMux()", 1)
	source = strings.Replace(source, "factoryMiddleware(svc)", "factoryMiddleware(childMux)", 1)
	source = strings.Replace(source, "svc, err := constructBuyer(mainPool)", "_, err = constructBuyer(mainPool)", 1)
	root, p := goFactoryLegacy(t, call, source)
	mustWrite(t, p, strings.Replace(source, "childMux := http.NewServeMux()", `childMux := http.NewServeMux()
 alias := childMux
 alias.Handle("/v1/factory", http.NotFoundHandler())`, 1))
	if code, out := call(root); code != 1 || !strings.Contains(out, "ERROR (TOUCHED) UNRESOLVED") {
		t.Fatalf("returned mux alias mutation escaped gate: exit=%d %s", code, out)
	}
}
