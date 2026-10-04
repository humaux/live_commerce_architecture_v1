//go:build browser

package foundation_test

// meta-connect browser gate (contract meta-claims-intake-v1 "Merchant connect (R4)"; docs/delivery/units/meta-connect.md "Done when"),
// label BROWSER, Meta = MOCK. Real browser against an isolated PG, the real private Go API (identity + admin routes with the
// metaconnect service over the real metaoauth Graph against tests/metaconnect/fakegraph on loopback), the production admin Next build
// and a signed mock IdP. The Facebook Login for Business dialog (https://www.facebook.com/...) is answered INSIDE the browser by
// page.route (a 302 to the app's own /api/meta/callback); the code exchange, /me/permissions, /me/accounts and subscribed_apps are answered
// by the fake Graph from the API process. Node never receives a database credential or a Meta token; a runner-only control listener
// registers OAuth codes (with a programmable permission/Page profile) and reads the fake's facts.
//
// Spec: tests/admin/meta-connect.spec.ts (Playwright; en + zh-TW + zh-CN, desktop 1586x992 + 390 px, screenshots hashed). Evidence:
// output/playwright/meta-connect/<timestamp>/. Not proven (MOCK): Meta's real dialog, /me/accounts with the user-token configuration,
// subscribed_apps, App Review, the webhook -> claim -> reply chain (that is the PG gate TestMetaConnectFlow).

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"livecommerce/internal/httpapi"
	metaoauth "livecommerce/internal/integrations/meta/oauth"
	"livecommerce/internal/integrations/meta/pagetoken"
	"livecommerce/internal/metaconnect"
	"livecommerce/tests/metaconnect/fakegraph"
)

func TestBrowserMetaConnect(t *testing.T) {
	if os.Getenv("LC_BROWSER_META_CONNECT_ACCEPTANCE") != "1" || os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" {
		t.Fatal("use scripts/dev/test-local.sh --browser-meta-connect")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 14*time.Minute)
	defer cancel()
	f := fixture(t)
	store := cbxStore(t, f, f.tenantA)
	principal, _ := lcPrincipal(t, f, f.tenantA, []string{store}, "store:read", "integration:manage", "integration:read")

	fake := fakegraph.New()
	t.Cleanup(fake.Close)
	fake.ExpectApp(miApp, miSecret, mcnRedirect)
	graph, err := metaoauth.NewGraph(fake.URL(), mcnVersion, nil)
	if err != nil {
		t.Fatal(err)
	}
	seal, open := mcnPageRing(t)
	mcnRunUnsubscriber(t, mcnUnsubscriber(t, f, open, fake.URL())) // the claims-worker's disconnect job (D2); the API itself never unsubscribes
	jobs, err := newInsertOnlyClient(f)
	if err != nil {
		t.Fatal(err)
	}
	svc, err := metaconnect.New(metaconnect.Config{Graph: graph, App: metaoauth.App{ID: miApp, RedirectURI: mcnRedirect, Secret: []byte(miSecret)},
		ConfigID: mcnConfig, GraphVersion: mcnVersion, PageAppID: miApp, IGAppID: miApp, StateKey: metaconnect.StateKeyFor([]byte(miSecret)), Seal: seal}, jobs)
	if err != nil {
		t.Fatal(err)
	}

	pageA, pageB, pageC := mcnPage("Browser Page A", true), mcnPage("Browser Page B", false), mcnPage("Browser Page C", false)
	var mu sync.Mutex
	var secrets []string
	var codes int
	noMessaging := []string{}
	for _, p := range mcnFullPerms {
		if p != "pages_messaging" {
			noMessaging = append(noMessaging, p)
		}
	}
	profiles := map[string]fakegraph.User{
		"full":   {Permissions: mcnFullPerms, Pages: []fakegraph.Page{pageA, pageB}},
		"nomsg":  {Permissions: noMessaging, Pages: []fakegraph.Page{pageC}},
		"fbonly": {Permissions: mcnFullPerms, Pages: []fakegraph.Page{pageB}},
	}
	controlKey := randomToken()
	control := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Gate-Key") != controlKey {
			http.Error(w, "forbidden", 403)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/oauth/code": // a fresh single-use code for the requested permission/Page profile
			user, ok := profiles[r.URL.Query().Get("profile")]
			if !ok {
				http.NotFound(w, r)
				return
			}
			mu.Lock()
			codes++
			code := fmt.Sprintf("SYNTH-CODE-%d-%s", codes, t04Tag())
			short, long := fake.AddCode(code, user)
			secrets = append(secrets, short, long)
			mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]string{"code": code})
		case "/facts":
			_ = json.NewEncoder(w).Encode(map[string]any{"subscribed": map[string]bool{"A": fake.Subscribed(pageA.ID), "B": fake.Subscribed(pageB.ID), "C": fake.Subscribed(pageC.ID)},
				"exchanges": fake.Count("GET", "oauth/access_token")})
		case "/secrets":
			mu.Lock()
			all := append([]string{miSecret, pageA.Token, pageB.Token, pageC.Token}, secrets...)
			mu.Unlock()
			_ = json.NewEncoder(w).Encode(all)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(control.Close)

	root, _ := filepath.Abs("../..")
	evidence := brfEvidence(t, root, "meta-connect")
	stack := mabStartAdmin(t, ctx, f, principal, evidence, httpapi.Options{SessionStoreList: true, MetaConnect: svc})
	env := map[string]string{
		"LC_BROWSER_STORE": store, "LC_BROWSER_CONTROL": control.URL, "LC_BROWSER_CONTROL_KEY": controlKey, "LC_BROWSER_APP_ID": miApp,
		"LC_BROWSER_CONFIG_ID": mcnConfig, "LC_BROWSER_REDIRECT": mcnRedirect, "LC_BROWSER_GRAPH_VERSION": mcnVersion,
		"LC_BROWSER_PAGE_A": pageA.ID, "LC_BROWSER_PAGE_A_NAME": pageA.Name, "LC_BROWSER_IG_A": pageA.IGID, "LC_BROWSER_IG_A_USER": pageA.IGName,
		"LC_BROWSER_PAGE_B": pageB.ID, "LC_BROWSER_PAGE_B_NAME": pageB.Name, "LC_BROWSER_PAGE_C": pageC.ID, "LC_BROWSER_PAGE_C_NAME": pageC.Name,
	}
	brfPlaywright(t, ctx, stack, []string{"meta-connect.spec.ts"}, env)

	// ---- PG facts after the UI drove the real commands ----
	count := func(q string, args ...any) int { return countRows(t, f.owner, q, args...) }
	// The flow ends connected to Page B (Facebook only): A and its Instagram account were disconnected, C was never bindable.
	if n := count(`SELECT count(*) FROM integration.bindings WHERE store_id=$1 AND provider='facebook' AND external_asset_id=$2 AND enabled`, store, pageB.ID); n != 1 {
		t.Errorf("%d enabled facebook bindings for Page B, want 1; evidence=%s", n, evidence)
	}
	if n := count(`SELECT count(*) FROM integration.bindings WHERE store_id=$1 AND external_asset_id=ANY($2) AND enabled`, store, []string{pageA.ID, pageA.IGID, pageC.ID}); n != 0 {
		t.Errorf("%d enabled bindings for the disconnected Page A / its Instagram / the refused Page C", n)
	}
	if n := count(`SELECT count(*) FROM integration.bindings WHERE store_id=$1 AND external_asset_id=ANY($2)`, store, []string{pageC.ID}); n != 0 {
		t.Errorf("the refused Page C left %d bindings", n)
	}
	if n := count(`SELECT count(*) FROM meta_inbox.routes WHERE store_id=$1 AND enabled`, store); n != 1 {
		t.Errorf("%d enabled routes, want exactly Page B's", n)
	}
	if n := count(`SELECT count(*) FROM integration.meta_page_heads h JOIN integration.bindings b ON b.id=h.binding_id WHERE b.store_id=$1`, store); n != 1 {
		t.Errorf("%d credential heads, want exactly Page B's (A and its Instagram were destroyed on disconnect)", n)
	}
	var keyID string
	var nonce, ct []byte
	var binding string
	if err := f.owner.QueryRow(ctx, `SELECT c.key_id,c.nonce,c.ciphertext,c.binding_id::text FROM integration.meta_page_credentials c WHERE c.store_id=$1 AND c.asset_id=$2 AND c.version=1`, store, pageB.ID).Scan(&keyID, &nonce, &ct, &binding); err != nil {
		t.Fatalf("Page B credential: %v", err)
	}
	if plain, err := open.Open(pagetoken.Scope{TenantID: f.tenantA, StoreID: store, BindingID: binding, Provider: "facebook", AssetID: pageB.ID, Version: 1}, keyID, nonce, ct); err != nil || string(plain) != pageB.Token {
		t.Errorf("Page B credential does not open (with the claims-worker's private ring) to its Page token: %v", err)
	}
	for action, min := range map[string]int{"meta.connect.started": 4, "meta.connect.callback": 3, "meta.connect.page_connected": 2, "meta.connect.disconnected": 1} {
		if n := count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action=$2 AND principal_id=$3`, store, action, principal); n < min {
			t.Errorf("audit %s: %d rows, want >= %d", action, n, min)
		}
	}
	mcnAwaitUnsubscribeDrain(t, f, store)
	if fake.Subscribed(pageA.ID) || fake.Subscribed(pageC.ID) || !fake.Subscribed(pageB.ID) {
		t.Error("final subscriptions: A was disconnected (the claims-worker job unsubscribed it), B is connected, C never was")
	}
	if n := fake.Count("DELETE", "/subscribed_apps"); n != 1 {
		t.Errorf("%d Graph unsubscribe calls, want exactly one (Page A)", n)
	}
	if n := count(`SELECT count(*) FROM integration.meta_unsubscribe_jobs WHERE store_id=$1 AND (ciphertext IS NOT NULL OR nonce IS NOT NULL)`, store); n != 0 {
		t.Errorf("%d unsubscribe jobs still hold a sealed token", n)
	}
	for _, r := range fake.Requests() {
		if r.HasQueryToken || (r.HasClientSecret && r.Path != "oauth/access_token") {
			t.Errorf("secret in a URL: %+v", r)
		}
	}
	mu.Lock()
	needles := append([]string{miSecret, pageA.Token, pageB.Token, pageC.Token}, secrets...)
	mu.Unlock()
	for _, table := range []string{"integration.meta_connect_states", "integration.meta_connections", "integration.meta_page_credentials", "integration.meta_unsubscribe_jobs", "integration.bindings", "ops.audit_events", "ops.command_results", "meta_inbox.routes"} {
		for _, needle := range needles {
			if n := count(`SELECT count(*) FROM `+table+` t WHERE t::text LIKE '%'||$1||'%'`, needle); n != 0 {
				t.Errorf("a token or secret is stored in %s", table)
			}
		}
	}
	if raw, err := os.ReadFile(filepath.Join(evidence, "next.log")); err == nil {
		for _, needle := range needles {
			if bytes.Contains(raw, []byte(needle)) || strings.Contains(string(raw), "code=SYNTH-CODE") {
				t.Errorf("a token, secret or OAuth code appears in the admin Next log")
			}
		}
	}
	brfShots(t, evidence, 6)
	t.Logf("meta-connect BROWSER (Meta = MOCK) passed; evidence=%s", evidence)
}
