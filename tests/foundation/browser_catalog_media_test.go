//go:build browser

package foundation_test

// Independent catalog-media browser gate (R3 test author; docs/delivery/units/catalog-media.md CM3-CM6). `TestBrowserCatalogMedia`.
// Run through `bash scripts/dev/test-local.sh --browser-catalog-media` (isolated PG 18, production admin + storefront Next builds).
//
// Stack (MOCK tier): production admin Next + signed MOCK IdP + real BFF -> Go API -> PG for the merchant, the production storefront Next
// behind a disposable TLS/CONNECT edge for the anonymous buyer. Evidence labels: BROWSER, MOCK IdP; no provider, no real DNS/TLS.
// Seeded fixtures (disclosed): four products with two SKUs each and their names/prices through owner SQL. The storefront publication and the
// ACTIVE domain of https://buyer.example are NOT owner-seeded any more: sfiPublishViaDefiners runs the production writers of migration 0081
// (merchant control.set_storefront_published, operator control.operator_bind_domain) like the storefront-publish gate does.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"livecommerce/internal/buyerhttp"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/identity"
	"livecommerce/internal/identityhttp"
	"livecommerce/internal/oidclogin"
)

func TestBrowserCatalogMedia(t *testing.T) {
	if os.Getenv("LC_BROWSER_CATALOG_MEDIA_ACCEPTANCE") != "1" || os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" {
		t.Fatal("use scripts/dev/test-local.sh --browser-catalog-media")
	}
	// bhSetup minus bhPublish: the buyer HTTP service is production code; the admission facts come from the 0081 definers below.
	b := bcSetup(t)
	h := bhHarness{bcHarness: b, key: brToken(), origin: "https://buyer.example"}
	handler, err := buyerhttp.New(context.Background(), b.a.issuer, b.a.runtime, b.service, h.key, time.Hour)
	if err != nil {
		t.Fatal("buyer HTTP constructor failed")
	}
	h.server = httptest.NewServer(handler)
	t.Cleanup(h.server.Close)
	f := h.f
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	root, _ := filepath.Abs("../..")
	evidence := brfEvidence(t, root, "catalog-media")
	files := filepath.Join(evidence, "files")
	if err := os.MkdirAll(files, 0o700); err != nil {
		t.Fatal(err)
	}

	// Four products (zh-TW/en x desktop/mobile), each with a cheap SKU 2 (to be archived) and a dearer SKU 1 (to be repriced).
	type fixture struct {
		ProductID string `json:"product_id"`
		OldName   string `json:"old_name"`
		SKU1      struct {
			ID       string `json:"id"`
			Code     string `json:"code"`
			PriceOld int64  `json:"price_old"`
			PriceNew int64  `json:"price_new"`
		} `json:"sku1"`
		SKU2 struct {
			ID    string `json:"id"`
			Code  string `json:"code"`
			Price int64  `json:"price"`
		} `json:"sku2"`
		PNG string `json:"png"`
		JPG string `json:"jpg"`
	}
	var fixtures []fixture
	hashes := map[string][2]string{} // product -> sha256 of [png, jpg]
	currency := ""
	for i := 0; i < 4; i++ {
		stock := t04CreateStock(t, f, f.tokens["a"], f.storeA1, 5, 5)
		cmiCleanup(t, f, stock.product.ID)
		fx := fixture{ProductID: stock.product.ID, OldName: "CM old product " + stock.product.ID[:8]}
		fx.SKU1.ID, fx.SKU1.Code, fx.SKU1.PriceOld, fx.SKU1.PriceNew = stock.skus[0].ID, stock.skus[0].Code, 2500, 7777+int64(i)
		fx.SKU2.ID, fx.SKU2.Code, fx.SKU2.Price = stock.skus[1].ID, stock.skus[1].Code, 1100
		currency = stock.skus[0].Currency
		mustExec(t, f.owner, `UPDATE catalog.products SET name=$2,description='Synthetic catalog-media gate fixture' WHERE id=$1`, fx.ProductID, fx.OldName)
		mustExec(t, f.owner, `UPDATE catalog.skus SET price_minor=$2 WHERE id=$1`, fx.SKU1.ID, fx.SKU1.PriceOld)
		mustExec(t, f.owner, `UPDATE catalog.skus SET price_minor=$2 WHERE id=$1`, fx.SKU2.ID, fx.SKU2.Price)
		fx.PNG, fx.JPG = "photo-"+hex.EncodeToString([]byte{byte(i)})+".png", "photo-"+hex.EncodeToString([]byte{byte(i)})+".jpg"
		pngBytes, jpgBytes := cmiPNG(t, 40, 30, uint8(10+i)), cmiJPEG(t, 64, 48, uint8(60+i))
		for name, data := range map[string][]byte{fx.PNG: pngBytes, fx.JPG: jpgBytes} {
			if err := os.WriteFile(filepath.Join(files, name), data, 0o600); err != nil {
				t.Fatal(err)
			}
		}
		p, j := sha256.Sum256(pngBytes), sha256.Sum256(jpgBytes)
		hashes[fx.ProductID] = [2]string{hex.EncodeToString(p[:]), hex.EncodeToString(j[:])}
		fixtures = append(fixtures, fx)
	}

	// Merchant principal that can see only this store (production session issuance and permission checks stay in the path).
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	adminOrigin := browserFront(t, listener.Addr().String())
	_, adminPort, _ := net.SplitHostPort(listener.Addr().String())
	_ = listener.Close()
	idp := newBrowserIDP(t, adminOrigin+"/api/auth/callback")
	_, _, authority := identityFixture(t)
	provider, err := oidclogin.New(ctx, oidclogin.Config{Issuer: idp.server.URL, ClientID: browserClientID, RedirectURL: idp.redirect, AllowLoopbackForTests: true})
	if err != nil {
		t.Fatal(err)
	}
	service, err := identity.New(authority, observedBrowserProvider{Provider: provider, t: t}, identity.Policy{ProviderKey: "browser-catalog-media-mock-v1", SessionTTL: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	principal := randomUUID()
	mustExec(t, f.owner, `INSERT INTO identity.principals(id) VALUES($1)`, principal)
	mustExec(t, f.owner, `INSERT INTO identity.memberships(tenant_id,principal_id) VALUES($1,$2)`, f.tenantA, principal)
	mustExec(t, f.owner, `INSERT INTO identity.external_identities(issuer,subject,principal_id) VALUES($1,'browser-subject',$2)`, idp.server.URL, principal)
	mustExec(t, f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission)
		SELECT $1,$2,$3,p FROM unnest(ARRAY['store:read','catalog:read','catalog:write','inventory:read']) p`, f.tenantA, f.storeA1, principal)
	sfiPublishViaDefiners(t, f, principal, h.origin)
	adminKey := randomToken()
	private, err := identityhttp.NewHandler(service, adminKey)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/v1/identity/", private)
	mux.Handle("/", httpapi.NewHandler(f.runtime, httpapi.Options{SessionStoreList: true}))
	api := httptest.NewServer(mux)
	t.Cleanup(api.Close)

	ordersBefore := countRows(t, f.owner, `SELECT count(*) FROM checkout.orders WHERE store_id=$1`, f.storeA1)
	fixturesJSON, _ := json.Marshal(fixtures)
	cmd := exec.CommandContext(ctx, "node", "tests/storefront/catalog-media-gate.mjs")
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
		"LC_CM_EVIDENCE": evidence, "LC_CM_ADMIN_PORT": adminPort, "LC_CM_STORE": f.storeA1, "LC_CM_CURRENCY": currency,
		"LC_CM_FIXTURES": string(fixturesJSON), "LC_CM_FILES": files,
	})
	log := browserLog(t, filepath.Join(evidence, "browser.log"))
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Run(); err != nil {
		t.Fatalf("catalog-media browser gate failed: %v; evidence=%s", err, evidence)
	}

	// ---- independent PostgreSQL readback of what the browser did ---------------------------------------------------------
	raw, err := os.ReadFile(filepath.Join(evidence, "result.json"))
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Cases   int `json:"cases"`
		Results []struct {
			ProductID string   `json:"product_id"`
			Name      string   `json:"name"`
			ImageIDs  []string `json:"image_ids"`
			Locale    string   `json:"locale"`
			Viewport  string   `json:"viewport"`
		} `json:"results"`
	}
	if json.Unmarshal(raw, &result) != nil || len(result.Results) != 4 || result.Cases != 4*5 {
		t.Fatalf("missing exact browser gate results (cases=%d results=%d)", result.Cases, len(result.Results))
	}
	seen := map[string]bool{}
	for i, r := range result.Results {
		fx := fixtures[i]
		seen[r.Locale+"/"+r.Viewport] = true
		var name, status string
		if err := f.owner.QueryRow(ctx, `SELECT name,status FROM catalog.products WHERE id=$1 AND store_id=$2 AND tenant_id=$3`, fx.ProductID, f.storeA1, f.tenantA).Scan(&name, &status); err != nil || name != r.Name || status != "active" {
			t.Errorf("product %d: name=%q status=%q err=%v, want %q active", i, name, status, err, r.Name)
		}
		var p1, p2 int64
		var s1, s2 string
		if err := f.owner.QueryRow(ctx, `SELECT a.price_minor,a.status,b.price_minor,b.status FROM catalog.skus a, catalog.skus b WHERE a.id=$1 AND b.id=$2`, fx.SKU1.ID, fx.SKU2.ID).Scan(&p1, &s1, &p2, &s2); err != nil ||
			p1 != fx.SKU1.PriceNew || s1 != "active" || p2 != fx.SKU2.Price || s2 != "archived" {
			t.Errorf("product %d SKUs: sku1=%d/%s sku2=%d/%s err=%v, want %d/active and %d/archived", i, p1, s1, p2, s2, err, fx.SKU1.PriceNew, fx.SKU2.Price)
		}
		rows, err := f.owner.Query(ctx, `SELECT id::text,position,content_type,encode(sha256,'hex'),encode(sha256(bytes),'hex') FROM catalog.product_images WHERE product_id=$1 ORDER BY position`, fx.ProductID)
		if err != nil {
			t.Fatal(err)
		}
		var got [][4]string
		var ids []string
		for rows.Next() {
			var id, ct, stored, actual string
			var pos int
			if err := rows.Scan(&id, &pos, &ct, &stored, &actual); err != nil {
				t.Fatal(err)
			}
			ids = append(ids, id)
			got = append(got, [4]string{ct, stored, actual, string(rune('0' + pos))})
		}
		rows.Close()
		want := hashes[fx.ProductID]
		if len(got) != 2 || got[0] != [4]string{"image/jpeg", want[1], want[1], "0"} || got[1] != [4]string{"image/png", want[0], want[0], "1"} || ids[0] != r.ImageIDs[0] || ids[1] != r.ImageIDs[1] {
			t.Errorf("product %d image rows %v (ids %v vs browser %v), want JPEG at 0 then PNG at 1 with the uploaded bytes", i, got, ids, r.ImageIDs)
		}
	}
	for _, key := range []string{"zh-TW/desktop", "zh-TW/mobile", "en/desktop", "en/mobile"} {
		if !seen[key] {
			t.Errorf("matrix cell %s did not run", key)
		}
	}
	var shots []struct {
		File, Sha256, Locale, Viewport, Page string
	}
	rawShots, err := os.ReadFile(filepath.Join(evidence, "screenshots.json"))
	if err != nil || json.Unmarshal(rawShots, &shots) != nil || len(shots) != 12 {
		t.Fatalf("screenshot manifest: %d entries, %v (want 12: 3 pages x 4 cells)", len(shots), err)
	}
	for _, s := range shots {
		info, err := os.Stat(filepath.Join(evidence, s.File))
		if err != nil || info.Size() < 2000 || len(s.Sha256) != 64 {
			t.Errorf("screenshot %s missing, tiny or unhashed", s.File)
		}
	}
	if n := countRows(t, f.owner, `SELECT count(*) FROM checkout.orders WHERE store_id=$1`, f.storeA1); n != ordersBefore {
		t.Errorf("browsing created %d orders", n-ordersBefore)
	}
	t.Logf("BROWSER (MOCK IdP) catalog-media gate passed: 4 cells (zh-TW/en x desktop/390px), 20 cases, DB readback exact; evidence=%s", evidence)
}
