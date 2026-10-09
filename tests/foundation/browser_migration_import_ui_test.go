//go:build browser

// Purpose: independent W5-U1 signed HTTPS browser import acceptance, actual Go/PG writes and bounded transport faults.
// Depends on: foundation fixture/lcPrincipal, identity/httpapi, synthetic CSV, production Next, Playwright;
// LC_MIUI_ACCEPTANCE and LC_TEST_DATABASE_ALLOWED. No successful import DTO mocks or injected session cookies.
// Used by: GitHub --browser-migration-import; miui helpers are isolated from W6 and existing import fixtures.
// Invariants: I01/I02/I05/I06/I09/I11/I18; pure archive import, no provider or production acceptance.
package foundation_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/identity"
	"livecommerce/internal/identityhttp"
	"livecommerce/internal/oidclogin"
	"livecommerce/internal/platform"
)

type miuiWorld struct {
	t                       *testing.T
	f                       *testFixture
	store, principal, token string
	h                       http.Handler
	actors                  map[string]string
	mu                      sync.Mutex
	fault                   string
	requests                []miuiRequest
}

type miuiRequest struct {
	Method, Path, SHA, Mapping, Expected, ContentType string
	Bytes, Status                                     int
	HasKey                                            bool
}

// TestBrowserMigrationImport accepts both file flows by genuine clicks and independently checks persisted Go/PG facts.
func TestBrowserMigrationImport(t *testing.T) {
	if os.Getenv("LC_MIUI_ACCEPTANCE") != "1" || os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" {
		t.Fatal("use the isolated --browser-migration-import GitHub gate")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 24*time.Minute)
	defer cancel()
	x := miuiNew(t)
	beforeMoney := miuiMoney(t, x)
	beforeReports := miuiReports(t, x)
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	evidence := brfEvidence(t, root, "migration-import")
	origin, control, key := miuiStart(t, ctx, x, evidence)
	miuiSpec(t, ctx, root, evidence, origin, control, key, x.store)
	// Revocation acceptance invalidates the original fixture bearer. New readback authority does not revive the browser's pending payload.
	x.token = lcToken(t, x.f, x.principal)
	after := miuiState(t, x)
	if after.Counts["consents"] != 0 || after.Counts["capabilities"] != 0 {
		t.Errorf("import invented consent/capability: counts=%v", after.Counts)
	}
	if after.Counts["history"] < 51 || after.Counts["profiles"] < 8 {
		t.Errorf("browser did not persist the expected import corpus: counts=%v", after.Counts)
	}
	afterMoney, afterReports := miuiMoney(t, x), miuiReports(t, x)
	for table, n := range beforeMoney {
		if got := afterMoney[table]; got != n {
			t.Errorf("archive changed %s count %d -> %d", table, n, got)
		}
	}
	for report, before := range beforeReports {
		if got := afterReports[report]; got != before {
			t.Errorf("archive changed %s report", report)
		}
	}
	// Uploaded names/phones/addresses remain only in their allowed profile/history fields, never batch/receipt/audit bodies.
	for _, table := range []string{"migrationimport.batches", "ops.command_results", "ops.audit_events"} {
		q := `SELECT count(*) FROM ` + table + ` z WHERE z.store_id=$1 AND (row_to_json(z)::text LIKE '%MIUI-NAME-%' OR row_to_json(z)::text LIKE '%MIUI-NOTE-%' OR row_to_json(z)::text LIKE '%MIUI-CITY-DROP-%' OR row_to_json(z)::text LIKE '%@invalid.test%')`
		if n := countRows(t, x.f.owner, q, x.store); n != 0 {
			t.Errorf("private cells leaked into %s (%d rows)", table, n)
		}
	}
	miuiAuthority(t, x)
	b, _ := json.MarshalIndent(after, "", "  ")
	if err = os.WriteFile(filepath.Join(evidence, "pg-readback.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	ledger, err := os.ReadFile(filepath.Join(evidence, "miui-click-ledger.json"))
	var rows []struct {
		Result string `json:"result"`
	}
	if err != nil || json.Unmarshal(ledger, &rows) != nil || len(rows) == 0 {
		t.Fatalf("missing nonempty click ledger: %v", err)
	}
	for _, r := range rows {
		if r.Result != "pass" {
			t.Errorf("click ledger has result %q", r.Result)
		}
	}
	t.Logf("W5 BROWSER (signed MOCK OIDC, REAL_PG synthetic CSV); evidence=%s", evidence)
}

func miuiNew(t *testing.T) *miuiWorld {
	f := fixture(t)
	x := &miuiWorld{t: t, f: f, store: randomUUID(), actors: map[string]string{}}
	mustExec(t, f.owner, `INSERT INTO control.stores(tenant_id,id,name,currency) VALUES($1,$2,'MIUI synthetic store','TWD')`, f.tenantA, x.store)
	x.principal, x.token = lcPrincipal(t, f, f.tenantA, []string{x.store}, "store:read", "customers:read", "customers:privacy", "customers:write", "orders:read", "orders:export", "live:read")
	x.actors["owner"] = x.principal
	x.actors["reader"], _ = lcPrincipal(t, f, f.tenantA, []string{x.store}, "store:read", "customers:read", "orders:read")
	x.h = httpapi.NewHandler(f.runtime, httpapi.Options{SessionStoreList: true, PaymentEnvironment: "SANDBOX"})
	// This predetermined imported-only customer enables a genuine stale-count erasure fault. It is setup, not UI evidence.
	status, _ := miuiCall(x, "POST", "imports/customers/commit?expected_apply_rows=1", x.token, []byte("customer_id,name\nMIUI-STALE-B,MIUI-NAME-STALE-B\n"), "text/csv")
	if status != 200 {
		t.Fatalf("scoped stale-customer setup status=%d", status)
	}
	// Store-scoped cleanup only. Failed runs retain their fixture until the owning dev script captures/disposes its isolated DB.
	t.Cleanup(func() {
		if t.Failed() {
			return
		}
		for _, table := range []string{"customers.historical_orders", "customers.owner_tags", "customers.notes", "migrationimport.batches", "migrationimport.external_ids", "customers.import_profiles"} {
			mustExec(t, f.owner, `DELETE FROM `+table+` WHERE tenant_id=$1 AND store_id=$2`, f.tenantA, x.store)
		}
	})
	return x
}

func miuiCall(x *miuiWorld, method, resource, token string, body []byte, contentType string) (int, []byte) {
	var source io.Reader
	if len(body) > 0 {
		source = bytes.NewReader(body)
	}
	r := httptest.NewRequest(method, "/v1/admin/stores/"+x.store+"/"+resource, source)
	r.Header.Set("Authorization", "Bearer "+token)
	if contentType != "" {
		r.Header.Set("Content-Type", contentType)
	}
	if strings.HasSuffix(resource, "/erasure") {
		r.Header.Set("Idempotency-Key", t04Key("miui-scoped-erasure"))
	}
	w := httptest.NewRecorder()
	x.h.ServeHTTP(w, r)
	return w.Code, w.Body.Bytes()
}

type miuiSnapshot struct {
	Counts   map[string]int    `json:"counts"`
	Owners   map[string]string `json:"owners"`
	Batches  []miuiBatch       `json:"batches"`
	Requests []miuiRequest     `json:"requests"`
}
type miuiBatch struct {
	SHA, Kind, ID            string
	Created, Updated, Failed int
}

func miuiState(t *testing.T, x *miuiWorld) miuiSnapshot {
	t.Helper()
	out := miuiSnapshot{Counts: map[string]int{}, Owners: map[string]string{}}
	for name, table := range map[string]string{"owners": "buyer.owners", "profiles": "customers.import_profiles", "external": "migrationimport.external_ids", "history": "customers.historical_orders", "batches": "migrationimport.batches", "consents": "customers.consent_events", "capabilities": "buyer.capability_sessions", "receipts": "ops.command_results", "audits": "ops.audit_events"} {
		out.Counts[name] = countRows(t, x.f.owner, `SELECT count(*) FROM `+table+` WHERE tenant_id=$1 AND store_id=$2`, x.f.tenantA, x.store)
	}
	out.Counts["city_dropped"] = countRows(t, x.f.owner, `SELECT count(*) FROM customers.historical_orders WHERE store_id=$1 AND external_order_id LIKE 'ORDER-%-H2' AND city IS NULL`, x.store)
	for _, field := range []string{"phone_e164", "email"} {
		out.Counts["update_"+field+"_present"] = countRows(t, x.f.owner, `SELECT count(*) FROM customers.import_profiles p JOIN migrationimport.external_ids e ON e.internal_id=p.owner_id AND e.store_id=p.store_id AND e.kind='customers' WHERE p.store_id=$1 AND e.external_id='UPDATE-A' AND p.`+field+` IS NOT NULL`, x.store)
	}
	rows, e := x.f.owner.Query(context.Background(), `SELECT external_id,internal_id::text FROM migrationimport.external_ids WHERE tenant_id=$1 AND store_id=$2 AND kind='customers'`, x.f.tenantA, x.store)
	if e != nil {
		t.Fatal(e)
	}
	for rows.Next() {
		var ext, id string
		if e = rows.Scan(&ext, &id); e != nil {
			t.Fatal(e)
		}
		out.Owners[ext] = id
	}
	rows.Close()
	rows, e = x.f.owner.Query(context.Background(), `SELECT encode(file_sha256,'hex'),kind,id::text,applied,updated,failed FROM migrationimport.batches WHERE tenant_id=$1 AND store_id=$2 ORDER BY created_at,id`, x.f.tenantA, x.store)
	if e != nil {
		t.Fatal(e)
	}
	for rows.Next() {
		var b miuiBatch
		if e = rows.Scan(&b.SHA, &b.Kind, &b.ID, &b.Created, &b.Updated, &b.Failed); e != nil {
			t.Fatal(e)
		}
		out.Batches = append(out.Batches, b)
	}
	rows.Close()
	x.mu.Lock()
	out.Requests = append([]miuiRequest(nil), x.requests...)
	x.mu.Unlock()
	return out
}

func miuiMoney(t *testing.T, x *miuiWorld) map[string]int {
	t.Helper()
	out := map[string]int{}
	rows, e := x.f.owner.Query(context.Background(), `SELECT t.table_schema,t.table_name FROM information_schema.tables t WHERE t.table_type='BASE TABLE' AND t.table_schema IN ('checkout','payments','inventory','catalog','billing','ads') AND EXISTS(SELECT 1 FROM information_schema.columns c WHERE c.table_schema=t.table_schema AND c.table_name=t.table_name AND c.column_name='store_id')`)
	if e != nil {
		t.Fatal(e)
	}
	var tables []pgx.Identifier
	for rows.Next() {
		var schema, table string
		if e = rows.Scan(&schema, &table); e != nil {
			t.Fatal(e)
		}
		tables = append(tables, pgx.Identifier{schema, table})
	}
	rows.Close()
	for _, table := range tables {
		out[table.Sanitize()] = countRows(t, x.f.owner, `SELECT count(*) FROM `+table.Sanitize()+` WHERE store_id=$1`, x.store)
	}
	return out
}

func miuiReports(t *testing.T, x *miuiWorld) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, r := range []string{"finance/summary", "reports/products", "reports/channels", "reports/funnel", "reports/manual-orders"} {
		status, body := miuiCall(x, "GET", r+"?from=2026-03-01&to=2026-04-30", x.token, nil, "")
		if status != 200 {
			t.Fatalf("report baseline %s=%d", r, status)
		}
		out[r] = string(body)
	}
	return out
}

func miuiAuthority(t *testing.T, x *miuiWorld) {
	token := lcToken(t, x.f, x.actors["reader"])
	status, _ := miuiCall(x, "POST", "imports/customers/preview", token, []byte("customer_id,name\nREFUSED,No Write\n"), "text/csv")
	if status != 403 {
		t.Errorf("reader preview=%d want403", status)
	}
	status, _ = miuiCall(x, "GET", "imports/"+randomUUID()+"/results.csv?only=failed", x.token, nil, "")
	if status != 404 {
		t.Errorf("unknown batch=%d want404", status)
	}
}

func miuiStart(t *testing.T, ctx context.Context, x *miuiWorld, evidence string) (string, string, string) {
	root, e := filepath.Abs("../..")
	if e != nil {
		t.Fatal(e)
	}
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	address := l.Addr().String()
	_ = l.Close()
	_, port, _ := net.SplitHostPort(address)
	target, _ := url.Parse("http://" + address)
	front := httptest.NewTLSServer(&httputil.ReverseProxy{Rewrite: func(r *httputil.ProxyRequest) {
		r.SetURL(target)
		r.Out.Host = r.In.Host
		r.Out.Header.Set("X-Forwarded-Proto", "https")
	}})
	t.Cleanup(front.Close)
	idp := newBrowserIDP(t, front.URL+"/api/auth/callback")
	mustExec(t, x.f.owner, `INSERT INTO identity.external_identities(issuer,subject,principal_id) VALUES($1,'browser-subject',$2)`, idp.server.URL, x.principal)
	role := "miui_" + strings.ReplaceAll(randomUUID(), "-", "")
	sentinel := randomToken()
	mustExec(t, x.f.owner, `CREATE ROLE `+pgx.Identifier{role}.Sanitize()+` LOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE IN ROLE commerce_identity PASSWORD '`+sentinel+`'`)
	t.Cleanup(func() { mustExec(t, x.f.owner, `DROP ROLE `+pgx.Identifier{role}.Sanitize()) })
	u, e := url.Parse(x.f.databaseURL)
	if e != nil {
		t.Fatal(e)
	}
	u.User = url.UserPassword(role, sentinel)
	authority, e := platform.OpenIdentityPool(ctx, u.String())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(authority.Close)
	provider, e := oidclogin.New(ctx, oidclogin.Config{Issuer: idp.server.URL, ClientID: browserClientID, RedirectURL: idp.redirect, AllowLoopbackForTests: true})
	if e != nil {
		t.Fatal(e)
	}
	service, e := identity.New(authority, observedBrowserProvider{Provider: provider, t: t}, identity.Policy{ProviderKey: "miui", SessionTTL: time.Hour, OnboardingEnabled: true, Currencies: []string{"TWD"}})
	if e != nil {
		t.Fatal(e)
	}
	bffKey := randomToken()
	private, e := identityhttp.NewHandler(service, bffKey)
	if e != nil {
		t.Fatal(e)
	}
	mux := http.NewServeMux()
	mux.Handle("/v1/identity/", private)
	mux.Handle("/", x.h)
	api := httptest.NewServer(miuiWrap(x, mux))
	t.Cleanup(api.Close)
	controlKey := randomToken()
	control := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.Header.Get("X-MIUI-Gate-Key") != controlKey {
			http.Error(w, "forbidden", 403)
			return
		}
		var in struct {
			Mode string `json:"mode"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, 256)
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if dec.Decode(&in) != nil || dec.Decode(&struct{}{}) != io.EOF {
			http.Error(w, "invalid", 422)
			return
		}
		switch r.URL.Path {
		case "/state":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(miuiState(t, x))
			return
		case "/actor":
			id, ok := x.actors[in.Mode]
			if !ok {
				http.Error(w, "invalid", 422)
				return
			}
			if _, e = x.f.owner.Exec(r.Context(), `UPDATE identity.external_identities SET principal_id=$2 WHERE issuer=$1 AND subject='browser-subject'`, idp.server.URL, id); e != nil {
				http.Error(w, "fixture failed", 500)
				return
			}
		case "/fault":
			if in.Mode != "drop-customers" && in.Mode != "drop-orders" && in.Mode != "truncate-preview" && in.Mode != "history-503" && in.Mode != "" {
				http.Error(w, "invalid", 422)
				return
			}
			x.mu.Lock()
			x.fault = in.Mode
			x.mu.Unlock()
		case "/erase-stale":
			external := "MIUI-STALE-B"
			if in.Mode == "orders" {
				external = "ORDER-STALE-A"
			} else if in.Mode != "" {
				http.Error(w, "invalid", 422)
				return
			}
			var owner string
			if e = x.f.owner.QueryRow(r.Context(), `SELECT internal_id::text FROM migrationimport.external_ids WHERE store_id=$1 AND kind='customers' AND external_id=$2`, x.store, external).Scan(&owner); e != nil {
				http.Error(w, "fixture failed", 500)
				return
			}
			status, _ := miuiCall(x, "POST", "customers/"+owner+"/erasure", x.token, []byte(`{"confirm":"ERASE"}`), "application/json")
			if status != 200 {
				http.Error(w, "fixture failed", 500)
				return
			}
		case "/revoke":
			if _, e = x.f.owner.Exec(r.Context(), `UPDATE identity.sessions SET revoked_at=clock_timestamp() WHERE principal_id=$1`, x.principal); e != nil {
				http.Error(w, "fixture failed", 500)
				return
			}
		default:
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(204)
	}))
	t.Cleanup(control.Close)
	log := browserLog(t, filepath.Join(evidence, "next.log"))
	next := exec.CommandContext(ctx, "node", filepath.Join(root, "apps/admin/.next/standalone/apps/admin/server.js"))
	next.Dir = root
	next.Env = browserEnvironment(map[string]string{"HOSTNAME": "127.0.0.1", "PORT": port, "NODE_ENV": "production", "COMMERCE_IDENTITY_ENABLED": "1", "COMMERCE_IDENTITY_ALLOW_LOOPBACK_TESTS": "1", "COMMERCE_PUBLIC_ORIGIN": front.URL, "COMMERCE_API_ORIGIN": api.URL, "COMMERCE_OIDC_ISSUER": idp.server.URL, "COMMERCE_BFF_KEY": bffKey, "COMMERCE_ONBOARDING_ENABLED": "1", "COMMERCE_ONBOARDING_CURRENCIES": "TWD"})
	next.Stdout, next.Stderr = log, log
	next.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if e = next.Start(); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = syscall.Kill(-next.Process.Pid, syscall.SIGKILL); _, _ = next.Process.Wait() })
	processRecord, _ := json.MarshalIndent(map[string]any{"run_id": "miui-next", "pid": next.Process.Pid, "log": filepath.Join(evidence, "next.log"), "timeout_seconds": 24 * 60, "success": "HTTPS /api/stores readiness401, then browser gate exits0", "failure": "readiness failure, child exit, context deadline or browser gate failure", "handoff": "owning Go test and CI mode runner", "wake": "Go gate exit is consumed by CI; t.Cleanup stops owned process group"}, "", "  ")
	if e = os.WriteFile(filepath.Join(evidence, "next-process.json"), processRecord, 0600); e != nil {
		t.Fatal(e)
	}
	client := front.Client()
	client.Timeout = time.Second
	for attempt := 0; ; attempt++ {
		if response, e := client.Get(front.URL + "/api/stores"); e == nil {
			_ = response.Body.Close()
			if response.StatusCode == 401 {
				break
			}
		}
		if attempt > 100 {
			t.Fatalf("HTTPS Next readiness failed; evidence=%s", evidence)
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(100 * time.Millisecond):
		}
	}
	return front.URL, control.URL, controlKey
}

func miuiWrap(x *miuiWorld, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		x.mu.Lock()
		fault := x.fault
		match := (fault == "drop-customers" && r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/imports/customers/commit")) || (fault == "drop-orders" && r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/imports/orders/commit")) || (fault == "truncate-preview" && r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/preview")) || (fault == "history-503" && r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/historical-orders"))
		if match {
			x.fault = ""
		}
		x.mu.Unlock()
		if match && fault == "history-503" {
			http.Error(w, "fixture unavailable", 503)
			return
		}
		var data []byte
		if r.Method == "POST" && strings.Contains(r.URL.Path, "/imports/") {
			data, _ = io.ReadAll(io.LimitReader(r.Body, (2<<20)+1))
			_ = r.Body.Close()
			r.Body = io.NopCloser(bytes.NewReader(data))
		}
		recorded := httptest.NewRecorder()
		next.ServeHTTP(recorded, r)
		if r.Method == "POST" && strings.Contains(r.URL.Path, "/imports/") {
			sum := sha256.Sum256(data)
			x.mu.Lock()
			x.requests = append(x.requests, miuiRequest{Method: r.Method, Path: r.URL.Path, SHA: hex.EncodeToString(sum[:]), Mapping: r.URL.Query().Get("mapping"), Expected: r.URL.Query().Get("expected_apply_rows"), ContentType: r.Header.Get("Content-Type"), Bytes: len(data), Status: recorded.Code, HasKey: r.Header.Get("Idempotency-Key") != ""})
			x.mu.Unlock()
		}
		if match && (fault == "drop-customers" || fault == "drop-orders") && recorded.Code == 200 {
			if h, ok := w.(http.Hijacker); ok {
				if c, _, e := h.Hijack(); e == nil {
					_ = c.Close()
					return
				}
			}
			panic(http.ErrAbortHandler)
		}
		for k, v := range recorded.Header() {
			w.Header()[k] = v
		}
		w.WriteHeader(recorded.Code)
		body := recorded.Body.Bytes()
		if match && fault == "truncate-preview" && len(body) > 0 {
			body = body[:len(body)/2]
		}
		_, _ = w.Write(body)
	})
}

// TestMiuiDropWrapper calibrates bounded lost response AFTER handler completion, without PostgreSQL or a browser.
func TestMiuiDropWrapper(t *testing.T) {
	x := &miuiWorld{fault: "drop-customers"}
	var calls atomic.Int64
	s := httptest.NewServer(miuiWrap(x, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("X-Actual-Handler", "yes")
		w.WriteHeader(200)
		_, _ = w.Write([]byte("committed"))
	})))
	defer s.Close()
	client := &http.Client{Timeout: time.Second}
	for i := 0; i < 2; i++ {
		r, _ := http.NewRequest("POST", s.URL+"/v1/admin/stores/fixture/imports/customers/commit", strings.NewReader("fixture"))
		response, e := client.Do(r)
		if i == 0 && e == nil {
			_ = response.Body.Close()
			t.Fatal("MIUI-DROP-AFTER-COMMIT: first response not dropped")
		}
		if i == 1 {
			if e != nil {
				t.Fatal(e)
			}
			_ = response.Body.Close()
			if response.Header.Get("X-Actual-Handler") != "yes" {
				t.Fatal("MIUI-DROP-AFTER-COMMIT: retry did not reach handler")
			}
		}
	}
	if calls.Load() != 2 {
		t.Fatalf("MIUI-DROP-AFTER-COMMIT: handler invocations=%d want2", calls.Load())
	}
}

func miuiSpec(t *testing.T, ctx context.Context, root, evidence, origin, control, key, store string) {
	config := filepath.Join(evidence, "playwright.miui.config.ts")
	body := fmt.Sprintf(`import {defineConfig} from "@playwright/test";export default defineConfig({testDir:%q,testMatch:"import-wizard.spec.ts",workers:1,retries:0,timeout:180000,expect:{timeout:15000},reporter:[["list"]],outputDir:%q,use:{baseURL:%q,headless:true,ignoreHTTPSErrors:true,trace:"retain-on-failure",screenshot:"only-on-failure"}});`, filepath.Join(root, "tests/admin"), filepath.Join(evidence, "results"), origin)
	if e := os.WriteFile(config, []byte(body), 0600); e != nil {
		t.Fatal(e)
	}
	env := map[string]string{"LC_BROWSER_PUBLIC_ORIGIN": origin, "LC_BROWSER_EVIDENCE": evidence, "LC_MIUI_STORE": store, "LC_MIUI_CONTROL_URL": control, "LC_MIUI_CONTROL_KEY": key}
	if v := os.Getenv("LC_MIUI_CALIBRATION"); v != "" {
		env["LC_MIUI_CALIBRATION"] = v
	}
	cmd := exec.CommandContext(ctx, "pnpm", "exec", "playwright", "test", "--config", config)
	cmd.Dir = root
	cmd.Env = browserEnvironment(env)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	// Bound the whole owned browser process group, including the driver/browser descendants, when the context expires.
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	log := browserLog(t, filepath.Join(evidence, "playwright.log"))
	cmd.Stdout, cmd.Stderr = log, log
	if e := cmd.Start(); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) })
	processRecord, _ := json.MarshalIndent(map[string]any{"run_id": "miui-playwright", "pid": cmd.Process.Pid, "log": filepath.Join(evidence, "playwright.log"), "timeout_seconds": 24 * 60, "success": "Playwright exit0 with nonempty passing click ledger", "failure": "nonzero exit, FAIL or context deadline", "handoff": "owning Go test and CI mode runner", "wake": "synchronous Go Wait returns and CI collects gate result"}, "", "  ")
	if e := os.WriteFile(filepath.Join(evidence, "playwright-process.json"), processRecord, 0600); e != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatal(e)
	}
	if e := cmd.Wait(); e != nil {
		t.Fatalf("migration-import browser gate failed: %v; evidence=%s", e, evidence)
	}
}
