//go:build browser

// Purpose: W6-U1 genuine Go/PG plus signed HTTPS OIDC browser acceptance; no successful API response mocks.
// Depends on: rpWorld synthetic report corpus, scoped customers Go commands, identity/httpapi, production Next,
// Playwright and LC_W6UI_{REPORTS,CUSTOMERS}_ACCEPTANCE; requires LC_TEST_DATABASE_ALLOWED.
// Used by: GitHub --browser-reports and extended --browser-customers-billing gates; helpers use w6ui only.
// Invariants: I01/I02/I05/I06/I11/I14/I18; report seeds are disclosed MOCK data, not provider/capture acceptance.
package foundation_test

import (
	"context"
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
	"livecommerce/internal/customers"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/identity"
	"livecommerce/internal/identityhttp"
	"livecommerce/internal/oidclogin"
	"livecommerce/internal/platform"
)

type w6uiFixture struct {
	w                                                 *rpWorld
	principal, token, customer, second, note, session string
	actors                                            map[string]string
	mu                                                sync.Mutex
	fault                                             string
	requests                                          []w6uiRequest
}

type w6uiRequest struct {
	Method, Path, Key string
	Status            int
}

// TestW6UIFaultWrapper is DB-free harness calibration: a lost response happens after commit, once, with passthrough retry.
func TestW6UIFaultWrapper(t *testing.T) {
	x := &w6uiFixture{fault: "drop-note"}
	var commits atomic.Int64
	s := httptest.NewServer(w6uiWrap(x, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		commits.Add(1)
		w.Header().Set("X-Actual-Handler", "yes")
		w.WriteHeader(201)
		_, _ = w.Write([]byte("committed"))
	})))
	defer s.Close()
	client := &http.Client{Timeout: time.Second}
	for i := 0; i < 2; i++ {
		r, _ := http.NewRequest("POST", s.URL+"/v1/admin/stores/fixture/customers/fixture/notes", strings.NewReader("fixture"))
		r.Header.Set("Idempotency-Key", "w6ui-fixed-intent")
		response, e := client.Do(r)
		if i == 0 {
			if e == nil {
				_ = response.Body.Close()
				t.Fatal("W6UI-DROP-AFTER-COMMIT: first response was not dropped")
			}
		}
		if i == 1 {
			if e != nil {
				t.Fatal(e)
			}
			_ = response.Body.Close()
			if response.StatusCode != 201 || response.Header.Get("X-Actual-Handler") != "yes" {
				t.Fatal("W6UI-DROP-AFTER-COMMIT: retry not passed to actual handler")
			}
		}
	}
	if commits.Load() != 2 {
		t.Fatalf("W6UI-DROP-AFTER-COMMIT: actual handler commits=%d want2; fault must drop only after first handler invocation", commits.Load())
	}
}

// TestBrowserW6Reports drives every report tab/control through a real signed browser session.
func TestBrowserW6Reports(t *testing.T) { w6uiRun(t, "reports") }

// TestBrowserW6Customers drives tags/notes/filter/CAS/UNKNOWN controls, with independent SQL readback.
func TestBrowserW6Customers(t *testing.T) { w6uiRun(t, "customers") }

func w6uiRun(t *testing.T, mode string) {
	if os.Getenv("LC_W6UI_"+strings.ToUpper(mode)+"_ACCEPTANCE") != "1" || os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" {
		t.Fatal("use the registered isolated W6 browser gate in GitHub CI")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	x := w6uiSeed(t)
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	evidence := brfEvidence(t, root, "w6-"+mode)
	stack, control := w6uiStart(t, ctx, x, evidence)
	env := map[string]string{
		"LC_W6UI_MODE": mode, "LC_W6UI_STORE": x.w.store, "LC_W6UI_CUSTOMER": x.customer,
		"LC_W6UI_SECOND": x.second, "LC_W6UI_SESSION": x.session, "LC_W6UI_PRINCIPAL": x.principal,
		"LC_W6UI_CONTROL_URL": control.URL, "LC_W6UI_CONTROL_KEY": control.key,
	}
	w6uiSpecs(t, ctx, stack, mode, env)
	// The browser ledger proves user clicks; SQL here independently proves persisted facts, never replaces the clicks.
	if mode == "reports" {
		for _, report := range []string{"products", "channels", "funnel", "manual_orders"} {
			if n := countRows(t, x.w.f.owner, `SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action=$2`, x.w.store, "reports.export."+report); n != 4 {
				t.Errorf("%s exports=%d, want exactly four locale/viewport downloads", report, n)
			}
		}
	} else {
		for _, action := range []string{"customers.tag_created", "customers.tag_renamed", "customers.tag_deleted", "customers.tagged", "customers.note_added", "customers.note_edited", "customers.note_deleted"} {
			if n := countRows(t, x.w.f.owner, `SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action=$2`, x.w.store, action); n < 1 {
				t.Errorf("missing persisted %s", action)
			}
		}
		if n := countRows(t, x.w.f.owner, `SELECT count(*) FROM customers.notes WHERE store_id=$1 AND body='w6ui-unknown-once'`, x.w.store); n != 1 {
			t.Errorf("UNKNOWN retry created %d notes, want one", n)
		}
		if n := countRows(t, x.w.f.owner, `SELECT count(*) FROM ops.command_results WHERE store_id=$1 AND operation LIKE 'customers.%' AND response::text LIKE '%w6ui-unknown-once%'`, x.w.store); n != 0 {
			t.Errorf("private body leaked to %d receipts", n)
		}
		if n := countRows(t, x.w.f.owner, `SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND details::text LIKE '%w6ui-%'`, x.w.store); n != 0 {
			t.Errorf("private fixture body leaked to %d audit details", n)
		}
	}
	w6uiAuthorityReadback(t, x)
	x.mu.Lock()
	requests := append([]w6uiRequest(nil), x.requests...)
	x.mu.Unlock()
	// Keep exact test idempotency keys out of evidence: only method/path/status counts are needed here.
	counts := map[string]int{}
	for _, r := range requests {
		counts[fmt.Sprintf("%s %s %d", r.Method, r.Path, r.Status)]++
	}
	b, _ := json.MarshalIndent(counts, "", "  ")
	if err := os.WriteFile(filepath.Join(evidence, "go-http-counts.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	ledger, err := os.ReadFile(filepath.Join(evidence, "w6ui-click-ledger.json"))
	var rows []struct {
		Result string `json:"result"`
	}
	if err != nil || json.Unmarshal(ledger, &rows) != nil || len(rows) == 0 {
		t.Fatalf("missing nonempty click ledger: %v", err)
	}
	for _, row := range rows {
		if row.Result != "pass" {
			t.Errorf("click ledger contains %q", row.Result)
		}
	}
	t.Logf("W6 %s BROWSER (signed MOCK IdP, REAL_PG synthetic data); evidence=%s", mode, evidence)
}

func w6uiSeed(t *testing.T) *w6uiFixture {
	w := rpNewIn(t, "A", "SANDBOX")
	session, _, _ := rpSeedMain(w)
	x := &w6uiFixture{w: w, session: session, actors: map[string]string{}}
	x.principal, x.token = lcPrincipal(t, w.f, w.tenant, []string{w.store}, "store:read", "customers:read", "customers:write", "customers:privacy", "orders:read", "orders:export", "live:read")
	x.actors["owner"] = x.principal
	for role, perms := range map[string][]string{
		"reader": {"store:read", "customers:read", "orders:read"},
		"writer": {"store:read", "customers:read", "customers:write", "orders:read"},
		"none":   {"store:read"},
	} {
		id, _ := lcPrincipal(t, w.f, w.tenant, []string{w.store}, perms...)
		x.actors[role] = id
	}
	// Actual owners/import profiles make the synthetic report customer visible to the production merchant projection.
	x.customer, x.second = randomUUID(), randomUUID()
	for i, owner := range []string{x.customer, x.second} {
		mustExec(t, w.f.owner, `INSERT INTO buyer.owners(tenant_id,store_id,id) VALUES($1,$2,$3)`, w.tenant, w.store, owner)
		mustExec(t, w.f.owner, `INSERT INTO customers.import_profiles(tenant_id,store_id,owner_id,display_name,source) VALUES($1,$2,$3,$4,'shopline_csv')`, w.tenant, w.store, owner, fmt.Sprintf("W6 synthetic customer %d", i+1))
		o := w.order(time.Date(2026, 8, 1, 9, 0, 0, 0, rpTPE), "CONFIRMED", "storefront", "bank_transfer", rpLine{randomUUID(), randomUUID(), "CUSTOMER", "Synthetic historical-scope anchor", 1, 100})
		w.exec(`UPDATE checkout.orders SET owner_id=$1 WHERE id=$2`, owner, o.id)
	}
	mustExec(t, w.f.owner, `INSERT INTO customers.historical_orders(tenant_id,store_id,owner_id,external_order_id,ordered_at,status,total_minor,currency,items_summary) VALUES($1,$2,$3,'W6-ARCHIVE-ONLY',$4,'paid',999999,'TWD','W6-ARCHIVE-ONLY')`, w.tenant, w.store, x.customer, rpAt(10, 9))
	// The 21 catalogue entries exercise the real 20-checkbox cap; writes use the frozen Go/SQL path.
	for i := 1; i <= 21; i++ {
		var tag customers.TagRecord
		err := x.scope(func(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
			var e error
			tag, e = customers.CreateTag(ctx, tx, s, x.token, t04Key("w6ui-seed-tag"), customers.TagInput{Name: fmt.Sprintf("Seed%02d", i), Color: "blue"})
			return e
		})
		if err != nil {
			t.Fatal(err)
		}
		if i == 1 {
			if err = x.setTags(x.customer, []string{tag.ID}); err != nil {
				t.Fatal(err)
			}
		}
	}
	for i := 0; i < 52; i++ {
		var n customers.Note
		err := x.scope(func(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
			var e error
			n, e = customers.AddNote(ctx, tx, s, x.token, t04Key("w6ui-seed-note"), x.customer, customers.NoteInput{Body: fmt.Sprintf("w6ui-seed-note-%02d", i)})
			return e
		})
		if err != nil {
			t.Fatal(err)
		}
		if i == 51 {
			x.note = n.ID
		}
	}
	// An independently denominated USD fact must never merge into the TWD oracle. Disclosed report fixture only.
	o := w.order(rpAt(10, 9), "CONFIRMED", "storefront", "card", rpLine{randomUUID(), randomUUID(), "USD", "USD-only", 1, 700})
	w.capture(o, 700, "SANDBOX", rpAt(10, 10))
	w.exec(`UPDATE checkout.orders SET currency='USD',snapshot=jsonb_set(snapshot,'{quote,currency}','"USD"') WHERE id=$1`, o.id)
	w.exec(`UPDATE checkout.payment_attempts SET currency='USD' WHERE id=$1`, o.attempt)
	w.exec(`UPDATE payments.facts SET currency='USD' WHERE attempt_id=$1`, o.attempt)
	return x
}

func (x *w6uiFixture) scope(fn func(context.Context, pgx.Tx, platform.Scope) error) error {
	ctx := context.Background()
	return platform.WithScope(ctx, x.w.f.runtime, x.token, x.w.store, "store:read", func(tx pgx.Tx, s platform.Scope) error { return fn(ctx, tx, s) })
}

func (x *w6uiFixture) setTags(owner string, ids []string) error {
	return x.scope(func(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
		d, e := customers.Get(ctx, tx, s, x.token, owner)
		if e != nil {
			return e
		}
		// customers.set_owner_tags: actual scoped CAS fixture write, never a substitute for a browser Save click.
		_, e = customers.SetOwnerTags(ctx, tx, s, x.token, t04Key("w6ui-concurrent-tags"), owner, customers.SetTagsInput{TagIDs: ids, Revision: d.TagsRevision})
		return e
	})
}

type w6uiControlServer struct {
	*httptest.Server
	key string
}

func w6uiStart(t *testing.T, ctx context.Context, x *w6uiFixture, evidence string) (*brfStack, *w6uiControlServer) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	_ = listener.Close()
	_, port, _ := net.SplitHostPort(address)
	target, _ := url.Parse("http://" + address)
	// Always HTTPS, including Chromium: new leaf BFFs require Secure cookies. No auth-cookie injection.
	front := httptest.NewTLSServer(&httputil.ReverseProxy{Rewrite: func(r *httputil.ProxyRequest) {
		r.SetURL(target)
		r.Out.Host = r.In.Host
		r.Out.Header.Set("X-Forwarded-Proto", "https")
	}})
	t.Cleanup(front.Close)
	idp := newBrowserIDP(t, front.URL+"/api/auth/callback")
	mustExec(t, x.w.f.owner, `INSERT INTO identity.external_identities(issuer,subject,principal_id) VALUES($1,'browser-subject',$2)`, idp.server.URL, x.principal)
	role := "w6ui_" + strings.ReplaceAll(randomUUID(), "-", "")
	password := randomToken()
	mustExec(t, x.w.f.owner, `CREATE ROLE `+pgx.Identifier{role}.Sanitize()+` LOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE IN ROLE commerce_identity PASSWORD '`+password+`'`)
	t.Cleanup(func() { mustExec(t, x.w.f.owner, `DROP ROLE `+pgx.Identifier{role}.Sanitize()) })
	u, err := url.Parse(x.w.f.databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	u.User = url.UserPassword(role, password)
	authority, err := platform.OpenIdentityPool(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(authority.Close)
	provider, err := oidclogin.New(ctx, oidclogin.Config{Issuer: idp.server.URL, ClientID: browserClientID, RedirectURL: idp.redirect, AllowLoopbackForTests: true})
	if err != nil {
		t.Fatal(err)
	}
	service, err := identity.New(authority, observedBrowserProvider{Provider: provider, t: t}, identity.Policy{ProviderKey: "w6ui", SessionTTL: time.Hour, OnboardingEnabled: true, Currencies: []string{"TWD", "USD"}})
	if err != nil {
		t.Fatal(err)
	}
	bffKey := randomToken()
	private, err := identityhttp.NewHandler(service, bffKey)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/v1/identity/", private)
	mux.Handle("/", httpapi.NewHandler(x.w.f.runtime, httpapi.Options{SessionStoreList: true, PaymentEnvironment: "SANDBOX"}))
	api := httptest.NewServer(w6uiWrap(x, mux))
	t.Cleanup(api.Close)
	control := &w6uiControlServer{key: randomToken()}
	control.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.Header.Get("X-W6UI-Gate-Key") != control.key {
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
		var e error
		switch r.URL.Path {
		case "/actor":
			principal, ok := x.actors[in.Mode]
			if !ok {
				http.Error(w, "invalid", 422)
				return
			}
			_, e = x.w.f.owner.Exec(r.Context(), `UPDATE identity.external_identities SET principal_id=$2 WHERE issuer=$1 AND subject='browser-subject'`, idp.server.URL, principal)
		case "/fault":
			if in.Mode != "drop-note" && in.Mode != "read-503" && in.Mode != "truncate-report" && in.Mode != "tag-read-503" && in.Mode != "notes-read-503" && in.Mode != "" {
				http.Error(w, "invalid", 422)
				return
			}
			x.mu.Lock()
			x.fault = in.Mode
			x.mu.Unlock()
		case "/concurrent-tags":
			var id string
			e = x.w.f.owner.QueryRow(r.Context(), `SELECT id FROM customers.tags WHERE store_id=$1 AND name='Seed02'`, x.w.store).Scan(&id)
			if e == nil {
				e = x.setTags(x.customer, []string{id})
			}
		case "/concurrent-note":
			e = x.scope(func(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
				var version int64
				// The owner pool supplies version only for fixture coordination; mutation runs via customers.EditNote under merchant scope.
				if e := x.w.f.owner.QueryRow(ctx, `SELECT version FROM customers.notes WHERE id=$1`, x.note).Scan(&version); e != nil {
					return e
				}
				_, e := customers.EditNote(ctx, tx, s, x.token, t04Key("w6ui-concurrent-note"), x.customer, x.note, customers.EditNoteInput{Body: "w6ui-concurrent-version", Version: version})
				return e
			})
		case "/deny-write":
			_, e = x.w.f.owner.Exec(r.Context(), `DELETE FROM identity.store_grants WHERE principal_id=$1 AND store_id=$2 AND permission='customers:write'`, x.principal, x.w.store)
		case "/restore-write":
			_, e = x.w.f.owner.Exec(r.Context(), `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,'customers:write') ON CONFLICT DO NOTHING`, x.w.tenant, x.w.store, x.principal)
		case "/state":
			var unknown int
			e = x.w.f.owner.QueryRow(r.Context(), `SELECT count(*) FROM customers.notes WHERE store_id=$1 AND body IN ('w6ui-unknown-once','w6ui-revoked-unknown')`, x.w.store).Scan(&unknown)
			if e == nil {
				x.mu.Lock()
				writes := 0
				for _, q := range x.requests {
					if q.Method == "POST" && strings.HasSuffix(q.Path, "/notes") {
						writes++
					}
				}
				x.mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]int{"unknown_notes": unknown, "note_posts": writes})
				return
			}
		case "/revoke":
			_, e = x.w.f.owner.Exec(r.Context(), `UPDATE identity.sessions SET revoked_at=clock_timestamp() WHERE principal_id=$1`, x.principal)
		default:
			http.NotFound(w, r)
			return
		}
		if e != nil {
			http.Error(w, "fixture failed", 500)
			return
		}
		w.WriteHeader(204)
	}))
	t.Cleanup(control.Close)
	log := browserLog(t, filepath.Join(evidence, "next.log"))
	next := exec.CommandContext(ctx, "node", filepath.Join(root, "apps/admin/.next/standalone/apps/admin/server.js"))
	next.Dir = root
	next.Env = browserEnvironment(map[string]string{"HOSTNAME": "127.0.0.1", "PORT": port, "NODE_ENV": "production", "COMMERCE_IDENTITY_ENABLED": "1", "COMMERCE_IDENTITY_ALLOW_LOOPBACK_TESTS": "1", "COMMERCE_PUBLIC_ORIGIN": front.URL, "COMMERCE_API_ORIGIN": api.URL, "COMMERCE_OIDC_ISSUER": idp.server.URL, "COMMERCE_BFF_KEY": bffKey, "COMMERCE_ONBOARDING_ENABLED": "1", "COMMERCE_ONBOARDING_CURRENCIES": "TWD,USD"})
	next.Stdout, next.Stderr = log, log
	next.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err = next.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = syscall.Kill(-next.Process.Pid, syscall.SIGKILL); _, _ = next.Process.Wait() })
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
			t.Fatalf("HTTPS admin readiness failed; evidence=%s", evidence)
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(100 * time.Millisecond):
		}
	}
	return &brfStack{root: root, evidence: evidence, origin: front.URL, api: api}, control
}

func w6uiAuthorityReadback(t *testing.T, x *w6uiFixture) {
	// Hidden/disabled UI controls do not weaken authority testing. These refused direct requests are backend evidence only.
	reader := lcToken(t, x.w.f, x.actors["reader"])
	for _, route := range []string{"reports/products.csv?from=2026-09-01&to=2026-09-30", "reports/funnel?from=2026-09-01&to=2026-09-30"} {
		r := httptest.NewRequest("GET", "/v1/admin/stores/"+x.w.store+"/"+route, nil)
		r.Header.Set("Authorization", "Bearer "+reader)
		w := httptest.NewRecorder()
		x.w.h.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Errorf("reader authority %s status=%d want403", route, w.Code)
		}
	}
	r := httptest.NewRequest("POST", "/v1/admin/stores/"+x.w.store+"/customers/"+x.customer+"/notes", strings.NewReader(`{"body":"must-never-commit"}`))
	r.Header.Set("Authorization", "Bearer "+reader)
	r.Header.Set("Idempotency-Key", t04Key("w6ui-refused-note"))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	x.w.h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Errorf("read-only note write status=%d want403", w.Code)
	}
	if n := countRows(t, x.w.f.owner, `SELECT count(*) FROM customers.notes WHERE store_id=$1 AND body='must-never-commit'`, x.w.store); n != 0 {
		t.Errorf("refused backend note mutated %d rows", n)
	}
	for _, resource := range []string{"reports/products?from=2026-09-01&to=2026-09-30", "customers/" + x.customer} {
		r := httptest.NewRequest("GET", "/v1/admin/stores/"+x.w.f.storeB+"/"+resource, nil)
		r.Header.Set("Authorization", "Bearer "+reader)
		w := httptest.NewRecorder()
		x.w.h.ServeHTTP(w, r)
		if w.Code != 404 {
			t.Errorf("foreign scope %s=%d want404", resource, w.Code)
		}
	}
	owner := lcToken(t, x.w.f, x.principal)
	for _, days := range []struct {
		to     string
		status int
	}{{"2026-09-30", 200}, {"2026-10-01", 422}} {
		for _, report := range []string{"products", "channels", "funnel", "manual-orders"} {
			r := httptest.NewRequest("GET", "/v1/admin/stores/"+x.w.store+"/reports/"+report+"?from=2026-07-01&to="+days.to, nil)
			r.Header.Set("Authorization", "Bearer "+owner)
			w := httptest.NewRecorder()
			x.w.h.ServeHTTP(w, r)
			if w.Code != days.status {
				t.Errorf("inclusive range %s/%s=%d want%d", report, days.to, w.Code, days.status)
			}
		}
	}
}

func w6uiWrap(x *w6uiFixture, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		x.mu.Lock()
		fault := x.fault
		match := fault == "drop-note" && r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/notes") || (fault == "read-503" || fault == "truncate-report") && r.Method == "GET" && strings.Contains(r.URL.Path, "/reports/") || fault == "tag-read-503" && r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/customers/tags") || fault == "notes-read-503" && r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/notes")
		if match {
			x.fault = ""
		}
		x.mu.Unlock()
		if match && (fault == "read-503" || fault == "tag-read-503" || fault == "notes-read-503") {
			http.Error(w, "fixture transport unavailable", 503)
			return
		}
		recorded := httptest.NewRecorder()
		next.ServeHTTP(recorded, r)
		x.mu.Lock()
		x.requests = append(x.requests, w6uiRequest{r.Method, r.URL.Path, r.Header.Get("Idempotency-Key"), recorded.Code})
		x.mu.Unlock()
		if match && fault == "drop-note" && recorded.Code < 300 {
			// Commit already happened. Drop the actual response once; explicit UI retry must reuse its immutable command key.
			if hj, ok := w.(http.Hijacker); ok {
				if conn, _, e := hj.Hijack(); e == nil {
					_ = conn.Close()
					return
				}
			}
			panic(http.ErrAbortHandler)
		}
		for k, values := range recorded.Header() {
			w.Header()[k] = values
		}
		w.WriteHeader(recorded.Code)
		body := recorded.Body.Bytes()
		if match && fault == "truncate-report" && len(body) > 0 {
			body = body[:len(body)/2]
		}
		_, _ = w.Write(body)
	})
}

func w6uiSpecs(t *testing.T, ctx context.Context, s *brfStack, mode string, env map[string]string) {
	config := filepath.Join(s.evidence, "playwright.w6.config.ts")
	spec := "reports.spec.ts"
	if mode == "customers" {
		spec = "customer-tags.spec.ts"
	}
	body := fmt.Sprintf(`import {defineConfig} from "@playwright/test";
export default defineConfig({testDir:%q,testMatch:%q,workers:1,retries:0,timeout:120000,expect:{timeout:15000},reporter:[["list"]],outputDir:%q,use:{baseURL:%q,headless:true,ignoreHTTPSErrors:true,trace:"retain-on-failure",screenshot:"only-on-failure"}});`, filepath.Join(s.root, "tests/admin"), spec, filepath.Join(s.evidence, "results"), s.origin)
	if e := os.WriteFile(config, []byte(body), 0600); e != nil {
		t.Fatal(e)
	}
	cmd := exec.CommandContext(ctx, "pnpm", "exec", "playwright", "test", "--config", config)
	cmd.Dir = s.root
	env["LC_BROWSER_PUBLIC_ORIGIN"], env["LC_BROWSER_EVIDENCE"] = s.origin, s.evidence
	// CI calibration truncates/drops the actual backend answer; it must fail the named UI contract assertion, not setup.
	if v := os.Getenv("LC_W6UI_CALIBRATION"); v != "" {
		env["LC_W6UI_CALIBRATION"] = v
	}
	cmd.Env = browserEnvironment(env)
	log := browserLog(t, filepath.Join(s.evidence, "playwright.log"))
	cmd.Stdout, cmd.Stderr = log, log
	if e := cmd.Run(); e != nil {
		t.Fatalf("W6 browser gate failed: %v; evidence=%s", e, s.evidence)
	}
}
