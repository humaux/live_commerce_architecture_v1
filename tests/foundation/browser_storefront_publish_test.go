//go:build browser

// Purpose: drive the real publish/unpublish/browser path with an owned stable admin fixture origin.
// Depends on: browserAdminRelay, signed mock IdP, admin/storefront Next, Go services and disposable PostgreSQL.
// Used by: --browser-storefront-publish; no production host or provider mutation.
package foundation_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/buyerhttp"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/identity"
	"livecommerce/internal/identityhttp"
	"livecommerce/internal/oidclogin"
)

// R3 storefront-publish KEY acceptance gate (BROWSER; MOCK: signed MOCK IdP, synthetic TLS/CONNECT edge for the buyer host,
// the operator's ownership/TLS evidence is a reference string nobody verifies here).
//
// Production shape: NO owner-seeded storefront publication or domain row exists anywhere in this database (asserted
// before the browsers start and again at the end through the audit trail). The merchant publishes with the Settings
// card in a real browser (zh-TW + en, desktop + 390px), the platform operator binds the origin by running the built
// cmd/store-admin executable on the lc_store_registrar-shaped login, and an anonymous buyer browser then opens the
// product page on that origin; unpublish / suspend / detach turn it back into the not-found page. Product and SKU are
// created by the merchant in the UI; nothing is mocked in the admin Next -> Go -> PG path.
func TestBrowserStorefrontPublish(t *testing.T) {
	if os.Getenv("LC_BROWSER_STOREFRONT_PUBLISH_ACCEPTANCE") != "1" || os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" {
		t.Fatal("use scripts/dev/test-local.sh --browser-storefront-publish")
	}
	// bhSetup minus bhPublish: the buyer HTTP service is production code; admission facts are NOT seeded.
	b := bcSetup(t)
	h := bhHarness{bcHarness: b, key: brToken(), origin: "https://buyer.example"}
	handler, err := buyerhttp.New(context.Background(), b.a.issuer, b.a.runtime, b.service, h.key, time.Hour)
	if err != nil {
		t.Fatal("buyer HTTP constructor failed")
	}
	h.server = httptest.NewServer(handler)
	t.Cleanup(h.server.Close)
	owner := h.f.owner
	if n := countRows(t, owner, `SELECT (SELECT count(*) FROM control.storefront_publications)+(SELECT count(*) FROM control.storefront_domains)`); n != 0 {
		t.Fatalf("%d storefront publication/domain rows exist before the gate: the production-shape gate must start with none", n)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second)
	defer cancel()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := os.MkdirTemp(filepath.Join(root, "output/playwright"), "storefront-publish-")
	if err != nil {
		t.Fatal(err)
	}
	// The operator executable, exactly as ops-admin.sh runs it in the `ops` container.
	storeAdmin := filepath.Join(evidence, "store-admin")
	build := exec.CommandContext(ctx, "go", "build", "-o", storeAdmin, "./cmd/store-admin")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build cmd/store-admin: %v\n%s", err, out)
	}
	registrarDSN := miRole(t, h.f, "commerce_storefront_registrar") // lc_store_registrar grant shape (inherit_noset)

	admin := newBrowserAdminRelay(t)
	adminOrigin := admin.origin
	idp := newBrowserIDP(t, adminOrigin+"/api/auth/callback")
	_, _, authority := identityFixture(t)
	provider, err := oidclogin.New(ctx, oidclogin.Config{Issuer: idp.server.URL, ClientID: browserClientID, RedirectURL: idp.redirect, AllowLoopbackForTests: true})
	if err != nil {
		t.Fatal(err)
	}
	service, err := identity.New(authority, observedBrowserProvider{Provider: provider, t: t}, identity.Policy{ProviderKey: "browser-storefront-publish-mock-v1", SessionTTL: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	// A dedicated merchant principal that created this store (identity.initial_stores: the operator audit attribution) and
	// holds exactly what the product UI and the publication card need.
	principal := randomUUID()
	mustExec(t, owner, `INSERT INTO identity.principals(id) VALUES($1)`, principal)
	mustExec(t, owner, `INSERT INTO identity.memberships(tenant_id,principal_id) VALUES($1,$2)`, h.f.tenantA, principal)
	mustExec(t, owner, `INSERT INTO identity.external_identities(issuer,subject,principal_id) VALUES($1,'browser-subject',$2)`, idp.server.URL, principal)
	mustExec(t, owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission)
		SELECT $1,$2,$3,p FROM unnest(ARRAY['store:read','catalog:read','catalog:write','inventory:read','integration:read','integration:manage']) p`, h.f.tenantA, h.f.storeA1, principal)
	var warehouse string
	if err := owner.QueryRow(ctx, `SELECT id::text FROM inventory.warehouses WHERE tenant_id=$1 AND store_id=$2 LIMIT 1`, h.f.tenantA, h.f.storeA1).Scan(&warehouse); err != nil {
		t.Fatal(err)
	}
	mustExec(t, owner, `INSERT INTO identity.initial_stores(principal_id,idempotency_key,request_hash,tenant_id,store_id,warehouse_id)
		VALUES($1,$2,decode(repeat('cd',32),'hex'),$3,$4,$5)`, principal, "sp-browser-"+randomUUID()[:12], h.f.tenantA, h.f.storeA1, warehouse)
	t.Cleanup(func() {
		for _, q := range []string{`DELETE FROM control.storefront_domains WHERE tenant_id=$1`, `DELETE FROM control.storefront_publications WHERE tenant_id=$1`,
			`DELETE FROM ops.audit_events WHERE tenant_id=$1 AND action LIKE ANY(ARRAY['merchant.storefront%','operator.domain%'])`} {
			_, _ = owner.Exec(context.Background(), q, h.f.tenantA)
		}
		_, _ = owner.Exec(context.Background(), `DELETE FROM identity.initial_stores WHERE principal_id=$1`, principal)
	})
	adminKey := randomToken()
	private, err := identityhttp.NewHandler(service, adminKey)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/v1/identity/", private)
	mux.Handle("/", httpapi.NewHandler(h.f.runtime, httpapi.Options{SessionStoreList: true}))
	api := httptest.NewServer(mux)
	t.Cleanup(api.Close)

	// Runner-only controls (random key, ephemeral loopback): read-only DB facts and the operator executable.
	type dbFacts struct {
		Publications []map[string]any `json:"publications"`
		Domains      []map[string]any `json:"domains"`
		Audits       []string         `json:"audits"`
		Orders       int              `json:"orders"`
	}
	facts := func() dbFacts {
		out := dbFacts{Publications: []map[string]any{}, Domains: []map[string]any{}} // JSON [] not null
		prow, err := owner.Query(ctx, `SELECT published,version FROM control.storefront_publications WHERE tenant_id=$1 AND store_id=$2`, h.f.tenantA, h.f.storeA1)
		if err != nil {
			t.Fatal(err)
		}
		for prow.Next() {
			var p bool
			var v int64
			_ = prow.Scan(&p, &v)
			out.Publications = append(out.Publications, map[string]any{"published": p, "version": v})
		}
		prow.Close()
		drow, err := owner.Query(ctx, `SELECT origin,state,version FROM control.storefront_domains WHERE tenant_id=$1 AND store_id=$2 ORDER BY origin`, h.f.tenantA, h.f.storeA1)
		if err != nil {
			t.Fatal(err)
		}
		for drow.Next() {
			var o, s string
			var v int64
			_ = drow.Scan(&o, &s, &v)
			out.Domains = append(out.Domains, map[string]any{"origin": o, "state": s, "version": v})
		}
		drow.Close()
		out.Audits = spAuditActions(t, owner, h.f.storeA1)
		out.Orders = countRows(t, owner, `SELECT (SELECT count(*) FROM checkout.orders)+(SELECT count(*) FROM inventory.reservations)`)
		return out
	}
	controlKey := randomToken()
	control := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Gate-Key") != controlKey {
			http.Error(w, "forbidden", 403)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "POST" && r.URL.Path == "/admin-upstream":
			admin.connect(w, r) // Runner-only, behind the same X-Gate-Key guard; no product route.
		case r.Method == "GET" && r.URL.Path == "/db":
			_ = json.NewEncoder(w).Encode(facts())
		case r.Method == "POST" && r.URL.Path == "/operator":
			var in struct {
				Args []string `json:"args"`
			}
			if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<14)).Decode(&in) != nil || len(in.Args) == 0 {
				http.Error(w, "bad request", 400)
				return
			}
			switch in.Args[0] {
			case "domain-bind", "domain-suspend", "domain-detach", "status": // the four ops-admin.sh sub-commands, nothing else
			default:
				http.Error(w, "not an operator command", 400)
				return
			}
			cmd := exec.CommandContext(r.Context(), storeAdmin, in.Args...)
			cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "COMMERCE_STORE_REGISTRAR_DATABASE_URL=" + registrarDSN}
			var so, se bytes.Buffer
			cmd.Stdout, cmd.Stderr = &so, &se
			code := 0
			if err := cmd.Run(); err != nil {
				code = -1
				if ee, ok := err.(*exec.ExitError); ok {
					code = ee.ExitCode()
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"exit": code, "stdout": so.String(), "stderr": se.String()})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(control.Close)

	cmd := exec.CommandContext(ctx, "node", "tests/storefront/storefront-publish-gate.mjs")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	defer func() {
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
	}()
	cmd.Dir = root
	cmd.Env = browserEnvironment(map[string]string{
		"COMMERCE_IDENTITY_ENABLED": "1", "COMMERCE_IDENTITY_ALLOW_LOOPBACK_TESTS": "1",
		"COMMERCE_PUBLIC_ORIGIN": adminOrigin, "COMMERCE_API_ORIGIN": api.URL,
		"COMMERCE_OIDC_ISSUER": idp.server.URL, "COMMERCE_BFF_KEY": adminKey,
		"COMMERCE_BUYER_WEB_ENABLED": "1", "COMMERCE_BUYER_DEMO_LABEL": "1",
		"COMMERCE_BUYER_API_ORIGIN": h.server.URL, "COMMERCE_BUYER_BFF_KEY": h.key,
		"COMMERCE_BUYER_COOKIE_KEY": brToken(), "COMMERCE_BUYER_SESSION_TTL": "3600",
		"LC_JOINT_EVIDENCE": evidence, "LC_JOINT_STORE": h.f.storeA1,
		"LC_JOINT_CONTROL": control.URL, "LC_JOINT_CONTROL_KEY": controlKey,
	})
	log := browserLog(t, filepath.Join(evidence, "browser.log"))
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Run(); err != nil {
		t.Fatalf("storefront publish browser gate failed; evidence=%s", evidence)
	}
	data, err := os.ReadFile(filepath.Join(evidence, "result.json"))
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Cases     int      `json:"cases"`
		Locales   []string `json:"locales"`
		Viewports []string `json:"viewports"`
		Audit     []string `json:"audit_expected"`
		Origin    string   `json:"origin"`
	}
	if json.Unmarshal(data, &result) != nil || result.Cases < 20 || !reflect.DeepEqual(result.Locales, []string{"en", "zh-TW"}) ||
		!reflect.DeepEqual(result.Viewports, []string{"desktop", "390px"}) || result.Origin != "https://buyer.example" {
		t.Fatalf("missing exact browser gate results: %s", data)
	}
	// Independent PostgreSQL readback of what the browsers and the operator executable did.
	after := facts()
	if fmt.Sprint(after.Audits) != fmt.Sprint(result.Audit) {
		t.Fatalf("audit trail in PG differs from the actions the gate drove:\n got %v\nwant %v", after.Audits, result.Audit)
	}
	if len(after.Publications) != 1 || after.Publications[0]["published"] != false || after.Publications[0]["version"].(int64) != 8 {
		t.Fatalf("final publication row: %v", after.Publications)
	}
	if len(after.Domains) != 1 || after.Domains[0]["origin"] != "https://buyer.example" || after.Domains[0]["state"] != "ACTIVE" {
		t.Fatalf("final domain rows: %v", after.Domains)
	}
	if after.Orders != 0 {
		t.Fatal("viewing the storefront created orders or reservations")
	}
	// Every row came from the writers: each state change has its audit row, attributed to the store's creating principal.
	if n := countRows(t, owner, `SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND principal_id=$2 AND action LIKE ANY(ARRAY['merchant.storefront%','operator.domain%'])`, h.f.storeA1, principal); n != len(result.Audit) {
		t.Fatalf("audit rows attributed to the creating principal = %d, want %d", n, len(result.Audit))
	}
	var evidenceRef string
	var until time.Time
	if err := owner.QueryRow(ctx, `SELECT evidence_ref,valid_until FROM control.storefront_domains WHERE store_id=$1`, h.f.storeA1).Scan(&evidenceRef, &until); err != nil || evidenceRef == "" || until.Before(time.Now().Add(24*time.Hour)) {
		t.Fatalf("domain proof readback: %q %v %v", evidenceRef, until, err)
	}
	if countRows(t, owner, `SELECT count(*) FROM identity.sessions WHERE principal_id=$1 AND audience='merchant'`, principal) < 1 {
		t.Fatal("no real merchant session")
	}
	idp.mu.Lock()
	exchanges := idp.exchanges
	idp.mu.Unlock()
	if exchanges < 1 {
		t.Fatal("expected a real signed MOCK IdP exchange")
	}
	t.Logf("PASS: merchant Settings card (en/zh-TW, desktop/390px) + store-admin executable + anonymous buyer browser on https://buyer.example, no owner-seeded publication/domain rows; cases=%d; evidence=%s", result.Cases, evidence)
}

// spAuditActions lists the storefront audit actions of one store in commit order.
func spAuditActions(t *testing.T, owner interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, store string) []string {
	t.Helper()
	rows, err := owner.Query(context.Background(), `SELECT action FROM ops.audit_events WHERE store_id=$1 AND action LIKE ANY(ARRAY['merchant.storefront%','operator.domain%']) ORDER BY created_at,id`, store)
	if err != nil {
		t.Fatal(err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	return out
}
