//go:build browser

package foundation_test

// browser_meta_connect_gate_test.go: runner of the INDEPENDENT meta-connect browser gate MCG10 (spec tests/admin/meta-connect-gate.spec.ts;
// contract meta-claims-intake-v1 "Merchant connect (R4)", unit brief "Done when"). Label BROWSER, Meta = MOCK. Real Chromium (desktop
// 1586x992 + 390x844, zh-TW + en) against an isolated PG, the real private Go API (identity + admin routes: metaconnect service over the real
// metaoauth Graph against tests/metaconnect/fakegraph on loopback, Studio planning + keyword claims) and the production admin Next build with a
// signed mock IdP. The Facebook Login for Business dialog is answered INSIDE the browser by page.route (a 302 to the app's own callback).
// Unlike TestBrowserMetaConnect (implementer) this gate also drives the Studio claim-source picker, the four locale x viewport combinations,
// state replay, hostile cookies/queries against the callback and refusals in zh-TW at 390 px, and reads PG afterwards.
// Run: bash scripts/dev/test-local.sh --browser-meta-connect

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

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/claims"
	"livecommerce/internal/httpapi"
	metaoauth "livecommerce/internal/integrations/meta/oauth"
	"livecommerce/internal/live"
	"livecommerce/internal/metaconnect"
	"livecommerce/internal/platform"
	"livecommerce/tests/metaconnect/fakegraph"
)

func TestBrowserMetaConnectGate(t *testing.T) {
	if os.Getenv("LC_BROWSER_META_CONNECT_ACCEPTANCE") != "1" || os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" {
		t.Fatal("use scripts/dev/test-local.sh --browser-meta-connect")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	f := fixture(t)
	store := cbxStore(t, f, f.tenantA)
	principal, fixtureToken := lcPrincipal(t, f, f.tenantA, []string{store}, "store:read", "integration:manage", "integration:read",
		"live:read", "live:manage", "integration:execute", "catalog:read", "inventory:read", "orders:read") // orders:read: the role-aware nav shows nav-orders, which the spec waits on

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

	// One Studio scene (draft) of the store: the claim-source picker lives on its claims page.
	var draft live.Draft
	if err := platform.WithScope(ctx, f.runtime, fixtureToken, store, "live:manage", func(tx pgx.Tx, s platform.Scope) (err error) {
		draft, err = live.CreateDraft(ctx, tx, s, fixtureToken, t04Key("mcg-draft"), live.DraftInput{Title: "MCG10 scene " + t04Tag(), AspectRatio: "9:16"})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	labels, err := claims.NewLabelKey(randomBytes(32))
	if err != nil {
		t.Fatal(err)
	}

	// 渠道倉-香港 style names (CJK) on purpose: the card, the pick list and the Studio picker must carry them intact.
	pageA := mcnPage("渠道倉-香港", true)
	pageB := mcnPage("渠道倉-澳門", false)
	pageC := mcnPage("渠道倉-缺權限", false)
	pageD := mcnPage("渠道倉-缺任務", false)
	pageD.Tasks = mcgTasksWithout("MODERATE")
	var mu sync.Mutex
	var secrets []string
	var codes int
	profiles := map[string]fakegraph.User{
		"full":   {Permissions: mcnFullPerms, Pages: []fakegraph.Page{pageA, pageB}},
		"nomsg":  {Permissions: mcgWithout("pages_messaging"), Pages: []fakegraph.Page{pageC}},
		"notask": {Permissions: mcnFullPerms, Pages: []fakegraph.Page{pageD}},
		"fbonly": {Permissions: mcnFullPerms, Pages: []fakegraph.Page{pageB}},
	}
	controlKey := randomToken()
	pageIDs := map[string]string{"A": pageA.ID, "B": pageB.ID, "C": pageC.ID, "D": pageD.ID}
	owned := func(q string, args ...any) int { return countRows(t, f.owner, q, args...) }
	control := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Gate-Key") != controlKey {
			http.Error(w, "forbidden", 403)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/oauth/code":
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
			sub := map[string]bool{}
			for k, id := range pageIDs {
				sub[k] = fake.Subscribed(id)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"subscribed": sub,
				"bindings":    owned(`SELECT count(*) FROM integration.bindings WHERE store_id=$1 AND provider IN ('facebook','instagram') AND enabled`, store),
				"routes":      owned(`SELECT count(*) FROM meta_inbox.routes WHERE store_id=$1 AND enabled`, store),
				"heads":       owned(`SELECT count(*) FROM integration.meta_page_heads h JOIN integration.bindings b ON b.id=h.binding_id WHERE b.store_id=$1`, store),
				"connections": owned(`SELECT count(*) FROM integration.meta_connections WHERE store_id=$1`, store)})
		case "/secrets":
			mu.Lock()
			all := append([]string{miSecret, pageA.Token, pageB.Token, pageC.Token, pageD.Token}, secrets...)
			mu.Unlock()
			_ = json.NewEncoder(w).Encode(all)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(control.Close)

	root, _ := filepath.Abs("../..")
	evidence := brfEvidence(t, root, "meta-connect-gate")
	stack := mabStartAdmin(t, ctx, f, principal, evidence, httpapi.Options{SessionStoreList: true, Studio: true, ClaimLabels: &labels, MetaConnect: svc})
	env := map[string]string{
		"LC_BROWSER_STORE": store, "LC_BROWSER_SCENE": draft.ID, "LC_BROWSER_CONTROL": control.URL, "LC_BROWSER_CONTROL_KEY": controlKey,
		"LC_BROWSER_PAGE_A": pageA.ID, "LC_BROWSER_PAGE_A_NAME": pageA.Name, "LC_BROWSER_IG_A": pageA.IGID, "LC_BROWSER_IG_A_USER": pageA.IGName,
		"LC_BROWSER_PAGE_B": pageB.ID, "LC_BROWSER_PAGE_B_NAME": pageB.Name, "LC_BROWSER_PAGE_C": pageC.ID, "LC_BROWSER_PAGE_C_NAME": pageC.Name,
		"LC_BROWSER_PAGE_D": pageD.ID, "LC_BROWSER_PAGE_D_NAME": pageD.Name,
	}
	brfPlaywright(t, ctx, stack, []string{"meta-connect-gate.spec.ts"}, env)

	// ---- PG facts after the UI drove the real commands ----
	if n := owned(`SELECT count(*) FROM integration.bindings WHERE store_id=$1 AND provider IN ('facebook','instagram') AND enabled`, store); n != 0 {
		t.Errorf("%d enabled Meta bindings after the gate; every flow ends disconnected; evidence=%s", n, evidence)
	}
	if n := owned(`SELECT count(*) FROM integration.meta_page_credentials c WHERE c.store_id=$1`, store); n != 0 {
		t.Errorf("%d sealed credentials left for the store after disconnect", n)
	}
	// R5 meta-multi-page: the spec now connects Page B next to A and then disconnects B (brief Gate), so B keeps a disabled
	// binding row as history; the never-picked / refused Pages C and D must still have no binding row at all, and B must have
	// no enabled binding (the enabled-count check above) and no sealed credential (checked here per Page).
	if n := owned(`SELECT count(*) FROM integration.bindings WHERE store_id=$1 AND external_asset_id=ANY($2)`, store, []string{pageC.ID, pageD.ID}); n != 0 {
		t.Errorf("the unpicked / refused Pages left %d bindings", n)
	}
	if n := owned(`SELECT count(*) FROM integration.bindings WHERE store_id=$1 AND external_asset_id=$2 AND enabled`, store, pageB.ID); n != 0 {
		t.Errorf("disconnected Page B still has %d enabled bindings", n)
	}
	for action, min := range map[string]int{"meta.connect.started": 9, "meta.connect.callback": 7, "meta.connect.page_connected": 5, "meta.connect.disconnected": 5} {
		if n := owned(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action=$2 AND principal_id=$3`, store, action, principal); n < min {
			t.Errorf("audit %s: %d rows, want >= %d", action, n, min)
		}
	}
	mcnAwaitUnsubscribeDrain(t, f, store)
	if fake.Count("DELETE", "/subscribed_apps") < 1 {
		t.Error("no Graph unsubscribe call: disconnect must hand the Page to the claims-worker's unsubscribe job")
	}
	if n := owned(`SELECT count(*) FROM integration.meta_unsubscribe_jobs WHERE store_id=$1 AND (ciphertext IS NOT NULL OR nonce IS NOT NULL)`, store); n != 0 {
		t.Errorf("%d unsubscribe jobs still hold a sealed token", n)
	}
	if n := owned(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='meta.connect.unsubscribed'`, store); n < 1 {
		t.Error("no audited unsubscribe")
	}
	for _, r := range fake.Requests() {
		if r.HasQueryToken || (r.HasClientSecret && r.Path != "oauth/access_token") {
			t.Errorf("secret in a URL: %+v", r)
		}
	}
	_ = open // the private ring is only ever the claims-worker's; the browser gate never opens a token
	mu.Lock()
	needles := append([]string{miSecret, pageA.Token, pageB.Token, pageC.Token, pageD.Token}, secrets...)
	mu.Unlock()
	for _, table := range []string{"integration.meta_connect_states", "integration.meta_connections", "integration.meta_page_credentials", "integration.bindings",
		"ops.audit_events", "ops.command_results", "meta_inbox.routes", "live.sessions"} {
		for _, needle := range needles {
			if n := owned(`SELECT count(*) FROM `+table+` t WHERE t::text LIKE '%'||$1||'%'`, needle); n != 0 {
				t.Errorf("a token or secret is stored in %s", table)
			}
		}
	}
	if raw, err := os.ReadFile(filepath.Join(evidence, "next.log")); err == nil {
		for _, needle := range needles {
			if bytes.Contains(raw, []byte(needle)) {
				t.Errorf("a token or secret appears in the admin Next log")
			}
		}
		if strings.Contains(string(raw), "code=SYNTH-CODE") {
			t.Errorf("an OAuth code appears in the admin Next log")
		}
	}
	// screenshot manifest: this gate promises en + zh-TW on desktop and 390 px
	raw, err := os.ReadFile(filepath.Join(evidence, "screenshots.json"))
	if err != nil {
		t.Fatalf("screenshot hash manifest missing: %v; evidence=%s", err, evidence)
	}
	var shots []struct{ File, Sha256, Locale, Viewport string }
	if err := json.Unmarshal(raw, &shots); err != nil || len(shots) < 18 { // the spec takes 18: pick x2 (mobile), card-connected x4, card-disconnected x4, studio-with-page x4, studio-no-page, refused x3
		t.Fatalf("screenshot manifest: %d entries (want >= 18): %v", len(shots), err)
	}
	seen := map[string]bool{}
	for _, s := range shots {
		if len(s.Sha256) != 64 {
			t.Fatalf("screenshot %s is not hashed", s.File)
		}
		seen[s.Locale+"/"+s.Viewport] = true
	}
	for _, combo := range []string{"en/desktop", "en/mobile", "zh-TW/desktop", "zh-TW/mobile"} {
		if !seen[combo] {
			t.Errorf("no screenshot for %s", combo)
		}
	}
	t.Logf("meta-connect independent BROWSER gate (Meta = MOCK) passed; evidence=%s", evidence)
}
