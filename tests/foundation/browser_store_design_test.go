//go:build browser

package foundation_test

// SDB (unit store-design, contracts/storefront-v2.md section B; independent gate, evidence label BROWSER, IdP = MOCK).
//
// Stack: isolated PG 18 (the foundation fixture), the real private Go API (httpapi: identity + admin routes incl. design/*),
// the production admin Next build behind a signed mock IdP, and Playwright Chromium (tests/admin/design.spec.ts). One real
// merchant journey runs in each of four (locale x viewport) contexts - en / zh-TW, desktop 1586x992 / 390 px - each in its OWN
// store: edit profile, upload a logo, add hero + product_grid + rich_text sections and a page, save the draft, unsaved-changes
// guard, publish v1, edit, publish v2, roll back to v1 (versions v1, v2, v3 = rollback of v1), reload. A fifth test block
// probes the BFF directly (allowlist, no query, 260 KiB cap, CSRF/key, client-sent identity headers) while the Go-side proxy
// counts what actually reached the API. After the browser run this test checks the PG facts of every journey store.
// Evidence: output/playwright/store-design/<timestamp>/ (screenshots hashed in screenshots.json, next.log, playwright.log).
// Not covered here (other units / NOT_RUN): the storefront rendering of the document (storefront-shell) and the Preview tab
// opening on a real storefront origin.

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"livecommerce/internal/httpapi"
	"livecommerce/internal/identity"
	"livecommerce/internal/identityhttp"
	"livecommerce/internal/oidclogin"
)

type sdbCall struct {
	Method   string `json:"method"`
	Path     string `json:"path"`
	Length   int64  `json:"length"`
	Leaked   bool   `json:"leaked"`
	LeakInfo string `json:"leak_info,omitempty"`
}

func TestBrowserStoreDesign(t *testing.T) {
	if os.Getenv("LC_BROWSER_STORE_DESIGN_ACCEPTANCE") != "1" || os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" {
		t.Fatal("use scripts/dev/test-local.sh --browser-design")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	f := fixture(t)
	sdgReset(t, f)

	type journey struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Locale   string `json:"locale"`
		Viewport string `json:"viewport"`
	}
	var journeys []journey
	for _, c := range []struct{ locale, viewport string }{{"en", "desktop"}, {"zh-TW", "desktop"}, {"en", "mobile"}, {"zh-TW", "mobile"}} {
		j := journey{ID: randomUUID(), Name: "Journey " + c.locale + " " + c.viewport, Locale: c.locale, Viewport: c.viewport}
		mustExec(t, f.owner, `INSERT INTO control.stores(tenant_id,id,name,currency) VALUES($1,$2,$3,'TWD')`, f.tenantA, j.ID, j.Name)
		for _, p := range []string{"store:read", "integration:read", "integration:manage"} {
			mustExec(t, f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,$4)`, f.tenantA, j.ID, f.principalA, p)
		}
		journeys = append(journeys, j)
	}
	bffStore := randomUUID()
	mustExec(t, f.owner, `INSERT INTO control.stores(tenant_id,id,name,currency) VALUES($1,$2,'BFF probes','TWD')`, f.tenantA, bffStore)
	for _, p := range []string{"store:read", "integration:read", "integration:manage"} {
		mustExec(t, f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,$4)`, f.tenantA, bffStore, f.principalA, p)
	}
	// the merchant of the journeys is also a plain store:read member of A1 (no design permission): the page must refuse it
	sdgGrant(t, f, f.tokens["a"], f.storeA1, "integration:read") // read-only on A1: view works, edit controls must fail server-side
	roStore := f.storeA1

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
	origin := "http://" + address
	idp := newBrowserIDP(t, origin+"/api/auth/callback")
	mustExec(t, f.owner, `INSERT INTO identity.external_identities(issuer,subject,principal_id) VALUES($1,'browser-subject',$2)`, idp.server.URL, f.principalA)
	_, _, authority := identityFixture(t)
	provider, err := oidclogin.New(ctx, oidclogin.Config{Issuer: idp.server.URL, ClientID: browserClientID, RedirectURL: idp.redirect, AllowLoopbackForTests: true})
	if err != nil {
		t.Fatal(err)
	}
	service, err := identity.New(authority, observedBrowserProvider{Provider: provider, t: t}, identity.Policy{ProviderKey: "browser-store-design-v1", SessionTTL: time.Hour, OnboardingEnabled: true, Currencies: []string{"TWD", "USD"}})
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
	mux.Handle("/", httpapi.NewHandler(f.runtime, httpapi.Options{SessionStoreList: true}))
	var mu sync.Mutex
	var calls []sdbCall
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/__test/design-calls" {
			mu.Lock()
			defer mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(calls)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/v1/admin/stores/") && strings.Contains(r.URL.Path, "/design") {
			c := sdbCall{Method: r.Method, Path: r.URL.Path, Length: r.ContentLength}
			for _, h := range []string{"Cookie", "X-Tenant-ID", "X-Store-ID", "X-Forwarded-Host", "Forwarded"} {
				if r.Header.Get(h) != "" {
					c.Leaked, c.LeakInfo = true, h
				}
			}
			mu.Lock()
			calls = append(calls, c)
			mu.Unlock()
		}
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(api.Close)

	evidence := filepath.Join(root, "output/playwright/store-design", time.Now().UTC().Format("20060102T150405.000000000"))
	if err = os.MkdirAll(evidence, 0700); err != nil {
		t.Fatal(err)
	}
	nextLog := browserLog(t, filepath.Join(evidence, "next.log"))
	next := exec.CommandContext(ctx, "node", filepath.Join(root, "apps/admin/.next/standalone/apps/admin/server.js"))
	next.Dir = root
	next.Env = browserEnvironment(map[string]string{"HOSTNAME": "127.0.0.1", "PORT": port, "NODE_ENV": "production", "COMMERCE_IDENTITY_ENABLED": "1", "COMMERCE_IDENTITY_ALLOW_LOOPBACK_TESTS": "1",
		"COMMERCE_PUBLIC_ORIGIN": origin, "COMMERCE_API_ORIGIN": api.URL, "COMMERCE_OIDC_ISSUER": idp.server.URL, "COMMERCE_BFF_KEY": bffKey, "COMMERCE_ONBOARDING_ENABLED": "1", "COMMERCE_ONBOARDING_CURRENCIES": "TWD,USD"})
	next.Stdout, next.Stderr = nextLog, nextLog
	if err = next.Start(); err != nil {
		t.Fatal(err)
	}
	nextDone := make(chan error, 1)
	go func() { nextDone <- next.Wait() }()
	t.Cleanup(func() {
		_ = next.Process.Kill()
		select {
		case <-nextDone:
		case <-time.After(5 * time.Second):
			t.Error("owned Next process did not stop")
		}
	})
	client := &http.Client{Timeout: time.Second}
	ready := false
	for attempt := 0; attempt < 150 && !ready; attempt++ {
		if response, e := client.Get(origin + "/api/stores"); e == nil {
			_ = response.Body.Close()
			ready = response.StatusCode == http.StatusUnauthorized
		}
		if !ready {
			select {
			case <-ctx.Done():
				t.Fatal("Next readiness deadline")
			case <-time.After(100 * time.Millisecond):
			}
		}
	}
	if !ready {
		t.Fatalf("Next readiness failed; evidence=%s", evidence)
	}

	stores, _ := json.Marshal(journeys)
	playwrightLog := browserLog(t, filepath.Join(evidence, "playwright.log"))
	browser := exec.CommandContext(ctx, "pnpm", "exec", "playwright", "test", "tests/admin/design.spec.ts", "--reporter=list", "--output="+filepath.Join(evidence, "results"))
	browser.Dir = root
	browser.Env = browserEnvironment(map[string]string{"LC_BROWSER_SUITE": "store-design", "LC_BROWSER_PUBLIC_ORIGIN": origin, "LC_BROWSER_API_ORIGIN": api.URL, "LC_BROWSER_EVIDENCE": evidence,
		"LC_BROWSER_DESIGN_STORES": string(stores), "LC_BROWSER_READONLY_STORE": roStore, "LC_BROWSER_BFF_STORE": bffStore, "LC_BROWSER_FOREIGN_STORES": f.storeB + "," + f.storeA2})
	browser.Stdout, browser.Stderr = playwrightLog, playwrightLog
	if err = browser.Run(); err != nil {
		// keep going: the PG facts below are independent evidence (a failed Playwright test must not hide a PG regression)
		t.Errorf("SDB browser gate failed: %v; evidence=%s", err, evidence)
	}

	// ---- PG facts of every journey store (the browser claims "versions v1, v2, v3"; the database must agree) ----
	for _, j := range journeys {
		type row struct {
			Version, Source int64
			Kind, Doc       string
		}
		rows, err := f.owner.Query(ctx, `SELECT version,source_version,kind,document::text FROM design.published_versions WHERE store_id=$1 ORDER BY version`, j.ID)
		if err != nil {
			t.Fatal(err)
		}
		var hist []row
		for rows.Next() {
			var r row
			if err := rows.Scan(&r.Version, &r.Source, &r.Kind, &r.Doc); err != nil {
				t.Fatal(err)
			}
			hist = append(hist, r)
		}
		rows.Close()
		if len(hist) != 3 || hist[0].Kind != "publish" || hist[1].Kind != "publish" || hist[2].Kind != "rollback" || hist[2].Source != 1 || hist[0].Source != 1 || hist[1].Source != 2 {
			t.Fatalf("%s: history %+v, want publish(v1 of draft 1), publish(v2 of draft 2), rollback of v1", j.Name, hist)
		}
		var d1, d2, d3 map[string]any
		_ = json.Unmarshal([]byte(hist[0].Doc), &d1)
		_ = json.Unmarshal([]byte(hist[1].Doc), &d2)
		_ = json.Unmarshal([]byte(hist[2].Doc), &d3)
		if !reflect.DeepEqual(d1, d3) || reflect.DeepEqual(d1, d2) {
			t.Fatalf("%s: v3 must equal v1 and differ from v2", j.Name)
		}
		profile := d1["profile"].(map[string]any)
		if profile["name"] != "Gate Shop "+j.Locale+" "+j.Viewport || profile["accent_color"] != "#ff6600" || profile["logo_image_id"] == nil {
			t.Fatalf("%s: v1 profile %v", j.Name, profile)
		}
		if d2["profile"].(map[string]any)["name"] != "Gate Shop "+j.Locale+" "+j.Viewport+" two" {
			t.Fatalf("%s: v2 profile %v", j.Name, d2["profile"])
		}
		var types []string
		for _, s := range d1["home"].(map[string]any)["sections"].([]any) {
			types = append(types, s.(map[string]any)["type"].(string))
		}
		if !reflect.DeepEqual(types, []string{"hero", "product_grid", "rich_text"}) {
			t.Fatalf("%s: sections %v", j.Name, types)
		}
		pages := d1["pages"].([]any)
		if len(pages) != 1 || pages[0].(map[string]any)["slug"] != "about" || strings.Contains(hist[0].Doc, "<") {
			t.Fatalf("%s: pages %v / HTML in the stored document", j.Name, pages)
		}
		var draftVersion int64
		var draftDoc string
		if err := f.owner.QueryRow(ctx, `SELECT version,document::text FROM design.documents WHERE store_id=$1`, j.ID).Scan(&draftVersion, &draftDoc); err != nil || draftVersion != 2 || !strings.Contains(draftDoc, "two") {
			t.Fatalf("%s: the rollback must leave the draft at v2 (got v%d, err=%v)", j.Name, draftVersion, err)
		}
		if n := countRows(t, f.owner, `SELECT count(*) FROM design.store_media WHERE store_id=$1`, j.ID); n < 1 {
			t.Fatalf("%s: no media row", j.Name)
		}
		if n := countRows(t, f.owner, `SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action IN ('design.published','design.rolled_back')`, j.ID); n != 3 {
			t.Fatalf("%s: %d audit rows, want 3", j.Name, n)
		}
	}
	// nothing leaked into the stores the journeys must not touch
	if n := countRows(t, f.owner, `SELECT count(*) FROM design.documents WHERE store_id IN ($1,$2,$3)`, f.storeA1, f.storeA2, f.storeB); n != 0 {
		t.Fatalf("%d design rows appeared in stores outside the journeys (read-only / foreign stores must stay empty)", n)
	}
	// the Go API saw no cookie, tenant/store header or forwarded host from the browser through the BFF
	mu.Lock()
	for _, c := range calls {
		if c.Leaked {
			t.Fatalf("BFF forwarded %s to Go on %s %s", c.LeakInfo, c.Method, c.Path)
		}
	}
	mu.Unlock()
	shots, err := os.ReadFile(filepath.Join(evidence, "screenshots.json"))
	if err != nil {
		t.Fatalf("screenshot manifest missing: %v; evidence=%s", err, evidence)
	}
	var manifest []struct{ File, Sha256, Locale, Viewport string }
	if err := json.Unmarshal(shots, &manifest); err != nil || len(manifest) < 4*5 {
		t.Fatalf("screenshot manifest has %d entries (%v)", len(manifest), err)
	}
	seen := map[string]bool{}
	for _, m := range manifest {
		if len(m.Sha256) != sha256.Size*2 {
			t.Fatalf("screenshot %s not hashed", m.File)
		}
		seen[m.Locale+"/"+m.Viewport] = true
	}
	for _, j := range journeys {
		if !seen[j.Locale+"/"+j.Viewport] {
			t.Fatalf("no screenshot for %s/%s", j.Locale, j.Viewport)
		}
	}
	t.Logf("SDB: %d journeys, PG facts, BFF probes and %d hashed screenshots checked; evidence=%s", len(journeys), len(manifest), evidence)
}
