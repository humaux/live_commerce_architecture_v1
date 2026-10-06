//go:build browser

package foundation_test

// Purpose: independent PM-U real-upload browser acceptance (docs/delivery/units/product-media-v2.md UI+Acceptance;
//   contracts/catalog-inventory-v1.md "Amendment — product-media-v2"). The merchant uploads 4 main + 2 option-value +
//   6 detail images through the production admin UI with real clicks/file choosers (one tall 750x4000 detail, one
//   > 2 MiB EXIF-orientation-6 JPEG the UI must downsize, not refuse), publishes, and buyers on zh-TW@1440 and en@390
//   see 4 main thumbnails, the variant image swap, the lazy detail stack without horizontal overflow, the option
//   image in the cart line and reach the delivery/payment step without placing an order. A separate 9-image fixture
//   is seeded in POST-migration layout (4 main + 5 detail) and only rendered; the actual upgrade is proven by
//   TestPMv2MigrationNineImages (REAL_PG), not here. PostgreSQL readback after the browsers is the authority.
// Depends on: buyer_checkout/buyer_http harnesses (bcSetup, bhHarness), browser_identity_chain helpers (newBrowserIDP,
//   browserFront, browserEnvironment, browserLog), browser_live_claims helpers (connectProxy, waitReady,
//   freeLoopbackPort), browser_storefront_fixture (sfiPublishViaDefiners), catalog_media generators (cmiPNG/cmiJPEG),
//   browser_product_media_v2_fixture (pmv2EXIF6, pmv2NoisyPhoto, pmv2Fit), production admin/storefront Next builds.
// Used by: scripts/dev/test-local.sh --browser-product-media-v2 (wired by the integrator; sets
//   LC_BROWSER_PRODUCT_MEDIA_V2_ACCEPTANCE=1). Drives tests/admin/product-media-v2.spec.ts (LC_BROWSER_SUITE
//   "product-media-v2" in playwright.config.ts is integrator wiring too).
// Evidence class: BROWSER + REAL_PG; MOCK signed IdP and a synthetic buyer.example TLS/CONNECT edge; no provider.
// LC_PM_UI_INJECT_FAULT=missing-detail is the owned calibration fault: the manifest then names a detail file the
// fixture deliberately does not write, so the spec must fail; the normal run must pass. GitHub runs red then green.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/buyerhttp"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/identity"
	"livecommerce/internal/identityhttp"
	"livecommerce/internal/inventory"
	"livecommerce/internal/oidclogin"
	"livecommerce/internal/platform"
)

// pmv2File is one generated upload source: name on disk, source pixel dimensions, content hash and size. The stored
// row must preserve these exact bytes for fitting originals or contain a resized JPEG/PNG for oversized sources.
// The >2 MiB EXIF photo may ONLY arrive normalized, and that is asserted strictly.
type pmv2File struct {
	Name    string `json:"name"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	Bytes   int    `json:"bytes"`
	SHA256  string `json:"sha256"`
	FitW    int    `json:"fit_width"`  // pmv2Fit of the DISPLAY dimensions (EXIF applied), i.e. the normalized upload's dims
	FitH    int    `json:"fit_height"` // for non-EXIF sources Fit == source when the longest side is <= 2000
	EXIF    bool   `json:"exif"`       // true: buffer is landscape but the displayed photo is portrait; passthrough impossible (> 2 MiB)
	Missing bool   `json:"missing"`    // calibration fault LC_PM_UI_INJECT_FAULT=missing-detail: manifest names a file that is not on disk
}

// pmv2Manifest is the fixture contract handed to the Playwright spec through LC_PM_MANIFEST.
type pmv2Manifest struct {
	Store    string              `json:"store"`
	Product  pmv2ManifestProduct `json:"product"`
	Migrated pmv2ManifestProduct `json:"migrated"`
	Files    struct {
		Main   []pmv2File          `json:"main"`
		Detail []pmv2File          `json:"detail"`
		SKU    map[string]pmv2File `json:"sku"` // option value ("Red"/"Blue") -> file
	} `json:"files"`
}

type pmv2ManifestProduct struct {
	ID     string            `json:"id"`
	Slug   string            `json:"slug"`
	Name   string            `json:"name"`
	SKUs   map[string]string `json:"skus"`   // option value -> sku id (product under test)
	Main   []string          `json:"main"`   // migrated fixture only: seeded main image ids in order
	Detail []string          `json:"detail"` // migrated fixture only: seeded detail image ids in order
}

func TestBrowserProductMediaV2RealUpload(t *testing.T) {
	if os.Getenv("LC_BROWSER_PRODUCT_MEDIA_V2_ACCEPTANCE") != "1" || os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" {
		t.Fatal("use scripts/dev/test-local.sh --browser-product-media-v2")
	}
	// bhSetup minus bhPublish: the buyer HTTP service is production code; the publication and the ACTIVE domain are
	// written below through the migration 0081 definers (merchant publish + operator bind), like the storefront gates.
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
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	evidence := brfEvidence(t, root, "product-media-v2")
	files := filepath.Join(evidence, "files")
	if err := os.MkdirAll(files, 0o700); err != nil {
		t.Fatal(err)
	}
	fault := os.Getenv("LC_PM_UI_INJECT_FAULT")

	// ---- product under test: DRAFT, one option axis Color with Red/Blue, real stock in the existing warehouse ------
	product := pmv2ManifestProduct{ID: randomUUID(), Slug: "pmv2-" + randomUUID()[:8], Name: "PM-U media gate product", SKUs: map[string]string{}}
	mustExec(t, f.owner, `INSERT INTO catalog.products(tenant_id,store_id,id,name,description,status,slug,options)
		VALUES($1,$2,$3,$4,'Synthetic PM-U browser gate fixture; published through the admin UI','draft',$5,$6::jsonb)`,
		f.tenantA, f.storeA1, product.ID, product.Name, product.Slug, `[{"name":"Color","values":["Red","Blue"]}]`)
	for _, value := range []string{"Red", "Blue"} {
		sku := randomUUID()
		mustExec(t, f.owner, `INSERT INTO catalog.skus(tenant_id,store_id,id,product_id,code,status,currency,price_minor,option_values,weight_grams,length_mm,width_mm,height_mm,origin_country,customs_name,hs_candidate)
			VALUES($1,$2,$3,$4,$5,'active',(SELECT currency FROM control.stores WHERE id=$2),1290,$6,100,10,20,30,'US','synthetic fixture','851840')`,
			f.tenantA, f.storeA1, sku, product.ID, "PMV2-"+value+"-"+sku[:6], []string{value})
		if _, err = t04Scoped(ctx, f, f.tokens["a"], f.storeA1, "inventory:write", func(tx pgx.Tx, scope platform.Scope) (inventory.Balance, error) {
			return inventory.AdjustOnHand(ctx, tx, scope, t04Key("pmv2-stock"), inventory.Adjustment{
				WarehouseID: b.stock.warehouse.ID, SKUID: sku, Delta: 10, ExpectedVersion: 0, Reason: "PM-U browser gate fixture stock",
			})
		}); err != nil {
			t.Fatalf("stock for %s: %v", value, err)
		}
		product.SKUs[value] = sku
	}
	// A bank transfer offer makes the checkout payment step reachable; synthetic coordinates, owner-pool fixture only.
	mustExec(t, f.owner, `INSERT INTO checkout.bank_transfer_settings(tenant_id,store_id,enabled,allow_cvs,bank_name,branch,account_name,account_number,window_hours,version,updated_at)
		VALUES($1,$2,true,false,'Synthetic Bank','Synthetic Branch','Synthetic Shop','000-000-0000',6,1,clock_timestamp())
		ON CONFLICT(tenant_id,store_id) DO UPDATE SET enabled=true,allow_cvs=false,window_hours=6,updated_at=clock_timestamp()`, f.tenantA, f.storeA1)
	t.Cleanup(func() {
		mustExec(t, f.owner, `DELETE FROM checkout.bank_transfer_settings WHERE tenant_id=$1 AND store_id=$2`, f.tenantA, f.storeA1)
	})

	// ---- migrated fixture: a 9-image product in POST-migration layout (4 main + 5 detail). The migration itself
	// (the same rows before 0149 as one 0..8 gallery) is proven by TestPMv2MigrationNineImages; here the browser only
	// proves that this layout renders. ---------------------------------------------------------------
	migrated := pmv2ManifestProduct{ID: randomUUID(), Slug: "pmv2-migrated-" + randomUUID()[:8], Name: "PM-U migrated nine-image product"}
	mustExec(t, f.owner, `INSERT INTO catalog.products(tenant_id,store_id,id,name,description,status,slug)
		VALUES($1,$2,$3,$4,'Synthetic post-migration layout fixture (9 images: 4 main + 5 detail)','active',$5)`,
		f.tenantA, f.storeA1, migrated.ID, migrated.Name, migrated.Slug)
	migratedSKU := randomUUID()
	mustExec(t, f.owner, `INSERT INTO catalog.skus(tenant_id,store_id,id,product_id,code,status,currency,price_minor)
		VALUES($1,$2,$3,$4,$5,'active',(SELECT currency FROM control.stores WHERE id=$2),990)`,
		f.tenantA, f.storeA1, migratedSKU, migrated.ID, "PMV2-MIG-"+migratedSKU[:6])
	seedMigrated := func(role string, n int) []string {
		ids := []string{}
		for pos := 0; pos < n; pos++ {
			w, h := 640+pos*40, 480+pos*30
			if role == "detail" {
				w, h = 800, 500+pos*200 // detail stack: taller images, all inside the 6x ratio
			}
			data := cmiJPEG(t, w, h, uint8(170+pos))
			id := randomUUID()
			sum := sha256.Sum256(data)
			mustExec(t, f.owner, `INSERT INTO catalog.product_images(tenant_id,store_id,product_id,id,role,position,content_type,bytes,sha256,width,height)
				VALUES($1,$2,$3,$4,$5,$6,'image/jpeg',$7,$8,$9,$10)`,
				f.tenantA, f.storeA1, migrated.ID, id, role, pos, data, sum[:], w, h)
			ids = append(ids, id)
		}
		return ids
	}
	migrated.Main = seedMigrated("main", 4)
	migrated.Detail = seedMigrated("detail", 5)
	// Owned inventory/cart/quote references survive through readback. The test-local gate removes its isolated
	// PostgreSQL container at exit; deleting products/SKUs here would violate their child foreign keys.

	// ---- generated upload sources (valid JPEG/PNG, never lookalikes) ---------------------------------------------
	manifest := pmv2Manifest{Store: f.storeA1, Product: product, Migrated: migrated}
	manifest.Files.SKU = map[string]pmv2File{}
	write := func(name string, data []byte, w, h int, displayW, displayH int, exif bool) pmv2File {
		t.Helper()
		if fault != "missing-detail" || name != "detail-6.jpg" { // calibration fault: named but never written
			if err := os.WriteFile(filepath.Join(files, name), data, 0o600); err != nil {
				t.Fatal(err)
			}
		}
		sum := sha256.Sum256(data)
		fw, fh := pmv2Fit(displayW, displayH)
		return pmv2File{Name: name, Width: w, Height: h, Bytes: len(data), SHA256: hex.EncodeToString(sum[:]), FitW: fw, FitH: fh, EXIF: exif, Missing: fault == "missing-detail" && name == "detail-6.jpg"}
	}
	manifest.Files.Main = []pmv2File{
		write("main-1.jpg", cmiJPEG(t, 800, 800, 21), 800, 800, 800, 800, false),
		write("main-2.png", pmv2TransparentPNG(t, 640, 640), 640, 640, 640, 640, false),
		write("main-3.jpg", cmiJPEG(t, 700, 900, 23), 700, 900, 700, 900, false),
		write("main-4.png", pmv2TransparentPNG(t, 3000, 2000), 3000, 2000, 3000, 2000, false),
	}
	photo := pmv2NoisyPhoto(t, 3200, 2400, 31) // buffer 3200x2400, EXIF 6 => displayed 2400x3200 portrait
	manifest.Files.Detail = []pmv2File{
		write("detail-1.jpg", cmiJPEG(t, 750, 4000, 32), 750, 4000, 750, 4000, false), // tall detail: ratio 5.33 <= 6
		write("detail-2-exif.jpg", pmv2EXIF6(t, photo), 3200, 2400, 2400, 3200, true),
		write("detail-3.png", cmiPNG(t, 800, 600, 33), 800, 600, 800, 600, false),
		write("detail-4.jpg", cmiJPEG(t, 800, 800, 34), 800, 800, 800, 800, false),
		write("detail-5.png", cmiPNG(t, 700, 1200, 35), 700, 1200, 700, 1200, false),
		write("detail-6.jpg", cmiJPEG(t, 640, 480, 36), 640, 480, 640, 480, false),
	}
	manifest.Files.SKU["Red"] = write("option-red.png", cmiPNG(t, 400, 400, 41), 400, 400, 400, 400, false)
	manifest.Files.SKU["Blue"] = write("option-blue.jpg", cmiJPEG(t, 500, 500, 42), 500, 500, 500, 500, false)

	// ---- merchant principal (signed MOCK IdP), admin API, publication through the 0081 definers -------------------
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
	service, err := identity.New(authority, observedBrowserProvider{Provider: provider, t: t}, identity.Policy{ProviderKey: "browser-product-media-v2-mock-v1", SessionTTL: time.Hour})
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

	// ---- production Next servers (admin standalone, storefront next start) --------------------------------------
	adminLog := browserLog(t, filepath.Join(evidence, "admin-next.log"))
	admin := exec.CommandContext(ctx, "node", filepath.Join(root, "apps/admin/.next/standalone/apps/admin/server.js"))
	admin.Dir = root
	admin.Env = browserEnvironment(map[string]string{"HOSTNAME": "127.0.0.1", "PORT": adminPort, "NODE_ENV": "production",
		"NEXT_TELEMETRY_DISABLED": "1", "COMMERCE_IDENTITY_ENABLED": "1", "COMMERCE_IDENTITY_ALLOW_LOOPBACK_TESTS": "1",
		"COMMERCE_PUBLIC_ORIGIN": adminOrigin, "COMMERCE_API_ORIGIN": api.URL, "COMMERCE_OIDC_ISSUER": idp.server.URL, "COMMERCE_BFF_KEY": adminKey})
	admin.Stdout, admin.Stderr = adminLog, adminLog
	storefrontPort := freeLoopbackPort(t)
	storefrontLog := browserLog(t, filepath.Join(evidence, "storefront-next.log"))
	storefront := exec.CommandContext(ctx, "node", filepath.Join(root, "apps/storefront/node_modules/next/dist/bin/next"),
		"start", "--hostname", "127.0.0.1", "--port", storefrontPort)
	storefront.Dir = filepath.Join(root, "apps/storefront")
	storefront.Env = browserEnvironment(map[string]string{"NODE_ENV": "production", "NEXT_TELEMETRY_DISABLED": "1",
		"COMMERCE_BUYER_WEB_ENABLED": "1", "COMMERCE_BUYER_DEMO_LABEL": "1", "COMMERCE_BUYER_API_ORIGIN": h.server.URL,
		"COMMERCE_BUYER_BFF_KEY": h.key, "COMMERCE_BUYER_COOKIE_KEY": brToken(), "COMMERCE_BUYER_SESSION_TTL": "3600"})
	storefront.Stdout, storefront.Stderr = storefrontLog, storefrontLog
	for _, process := range []*exec.Cmd{admin, storefront} {
		process.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		if err := process.Start(); err != nil {
			t.Fatal(err)
		}
		owned := process
		t.Cleanup(func() {
			_ = syscall.Kill(-owned.Process.Pid, syscall.SIGKILL)
			_ = owned.Wait()
		})
	}
	waitReady(ctx, t, adminOrigin+"/api/stores", "", http.StatusUnauthorized, evidence)
	waitReady(ctx, t, "http://127.0.0.1:"+storefrontPort+"/api/buyer/session", "buyer.example", http.StatusOK, evidence)

	// buyer.example: disposable TLS edge + single-authority CONNECT proxy (the live-claims gate shape).
	target, err := url.Parse("http://127.0.0.1:" + storefrontPort)
	if err != nil {
		t.Fatal(err)
	}
	edge := httptest.NewTLSServer(httputil.NewSingleHostReverseProxy(target))
	t.Cleanup(edge.Close)
	proxy := connectProxy(t, "buyer.example:443", edge.Listener.Addr().String())

	ordersBefore := countRows(t, f.owner, `SELECT count(*) FROM checkout.orders WHERE store_id=$1`, f.storeA1)
	reservationsBefore := countRows(t, f.owner, `SELECT count(*) FROM inventory.reservations WHERE store_id=$1`, f.storeA1)
	manifestJSON, _ := json.Marshal(manifest)
	playwrightLog := browserLog(t, filepath.Join(evidence, "playwright.log"))
	browser := exec.CommandContext(ctx, "pnpm", "exec", "playwright", "test", "tests/admin/product-media-v2.spec.ts",
		"--reporter=list", "--output="+filepath.Join(evidence, "results"))
	browser.Dir = root
	browser.Env = browserEnvironment(map[string]string{
		"LC_BROWSER_SUITE": "product-media-v2", "LC_BROWSER_PUBLIC_ORIGIN": adminOrigin, "LC_BROWSER_EVIDENCE": evidence,
		"LC_PM_PROXY": proxy, "LC_PM_BUYER_ORIGIN": h.origin, "LC_PM_FILES": files, "LC_PM_MANIFEST": string(manifestJSON),
	})
	browser.Stdout, browser.Stderr = playwrightLog, playwrightLog
	runErr := browser.Run()
	if fault == "missing-detail" {
		rawLog, _ := os.ReadFile(filepath.Join(evidence, "playwright.log"))
		if runErr == nil || !strings.Contains(string(rawLog), "PM_CALIBRATION_DETAIL_COUNT: five real uploads; six required") {
			t.Fatalf("calibration did not reach its intended detail-count failure: %v; evidence=%s", runErr, evidence)
		}
		// Calibration is a failing gate, never a PASS credited to an unrelated browser failure.
		t.Fatalf("calibration RED: omitted detail image caught by six-image assertion; evidence=%s", evidence)
	}

	if runErr != nil {
		t.Fatalf("product-media-v2 browser gate failed: %v; evidence=%s", runErr, evidence)
	}

	// ---- independent PostgreSQL readback of what the browser caused ----------------------------------------------
	var result struct {
		Cases  int `json:"cases"`
		Upload []struct {
			Role        string `json:"role"`
			OptionValue string `json:"option_value"`
			Name        string `json:"name"`
			ID          string `json:"id"`
			ContentType string `json:"content_type"`
			SizeBytes   int    `json:"size_bytes"`
			Width       int    `json:"width"`
			Height      int    `json:"height"`
		} `json:"uploads"`
		Buyer []struct {
			Locale   string `json:"locale"`
			Viewport string `json:"viewport"`
		} `json:"buyer"`
	}
	raw, err := os.ReadFile(filepath.Join(evidence, "result.json"))
	if err != nil || json.Unmarshal(raw, &result) != nil {
		t.Fatalf("missing browser result: %v; evidence=%s", err, evidence)
	}
	if len(result.Upload) != 12 || result.Cases < 30 {
		t.Fatalf("uploads=%d cases=%d, want 12 uploads and >= 30 click-ledger cases", len(result.Upload), result.Cases)
	}
	seenCell := map[string]bool{}
	for _, bcell := range result.Buyer {
		seenCell[bcell.Locale+"/"+bcell.Viewport] = true
	}
	for _, cell := range []string{"zh-TW/1440", "zh-TW/390", "en/1440", "en/390"} {
		if !seenCell[cell] {
			t.Errorf("buyer matrix cell %s did not run", cell)
		}
	}
	browserIDs := map[string]pmv2File{} // image id -> source file, as the browser observed them
	manifestByName := map[string]pmv2File{}
	for _, list := range [][]pmv2File{manifest.Files.Main, manifest.Files.Detail} {
		for _, mf := range list {
			manifestByName[mf.Name] = mf
		}
	}
	for _, mf := range manifest.Files.SKU {
		manifestByName[mf.Name] = mf
	}
	for _, up := range result.Upload {
		mf, ok := manifestByName[up.Name]
		if !ok {
			t.Fatalf("browser uploaded an unplanned file %q", up.Name)
		}
		browserIDs[up.ID] = mf
		if mf.EXIF {
			// The > 2 MiB EXIF photo may only arrive normalized: JPEG, under the cap, portrait (rotation applied).
			if up.ContentType != "image/jpeg" || up.SizeBytes > 2<<20 || up.Height <= up.Width || up.Height > 2000 {
				t.Errorf("EXIF photo stored as %s %d bytes %dx%d: must be a rotated (portrait) JPEG <= 2 MiB", up.ContentType, up.SizeBytes, up.Width, up.Height)
			}
			continue
		}
		if up.Width != mf.FitW || up.Height != mf.FitH {
			t.Errorf("%s stored at %dx%d, want the fitted %dx%d (sources <= 2000 px never change dimensions)", up.Name, up.Width, up.Height, mf.FitW, mf.FitH)
		}
		if up.SizeBytes > 2<<20 {
			t.Errorf("%s stored %d bytes > 2 MiB", up.Name, up.SizeBytes)
		}
	}
	type storedRow struct {
		id, role, contentType, sha string
		position, size             int
		width, height              *int
	}
	stored := func(productID string) []storedRow {
		rows, err := f.owner.Query(ctx, `SELECT id::text,role,position,content_type,octet_length(bytes),width,height,encode(sha256,'hex') FROM catalog.product_images WHERE product_id=$1 ORDER BY role,position`, productID)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []storedRow
		for rows.Next() {
			var r storedRow
			if err := rows.Scan(&r.id, &r.role, &r.position, &r.contentType, &r.size, &r.width, &r.height, &r.sha); err != nil {
				t.Fatal(err)
			}
			out = append(out, r)
		}
		return out
	}
	rows := stored(product.ID)
	counts := map[string]int{}
	positions := map[string]int{}
	for _, r := range rows {
		counts[r.role]++
		if r.position != positions[r.role] {
			t.Errorf("%s position %d, want contiguous %d", r.role, r.position, positions[r.role])
		}
		positions[r.role]++
		mf, ok := browserIDs[r.id]
		if !ok {
			t.Errorf("stored %s image %s was never seen in a browser upload response", r.role, r.id)
			continue
		}
		if mf.Bytes <= 2<<20 && max(mf.Width, mf.Height) <= 2000 {
			if r.size != mf.Bytes || r.sha != mf.SHA256 {
				t.Errorf("%s: fitting original bytes/format were changed", mf.Name)
			}
			wantType := "image/jpeg"
			if strings.HasSuffix(mf.Name, ".png") {
				wantType = "image/png"
			}
			if r.contentType != wantType {
				t.Errorf("%s: content type %s, want preserved %s", mf.Name, r.contentType, wantType)
			}
		}
		// Normalized by the client: the stored bytes must decode to the dims the browser reported.
		var data []byte
		if err := f.owner.QueryRow(ctx, `SELECT bytes FROM catalog.product_images WHERE id=$1`, r.id).Scan(&data); err != nil {
			t.Fatal(err)
		}
		cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			t.Errorf("%s: stored bytes are not a decodable raster: %v", mf.Name, err)
			continue
		}
		if r.contentType != "image/"+format {
			t.Errorf("%s: declared format differs from decoded %s", mf.Name, format)
		}
		if mf.Name == "main-2.png" || mf.Name == "main-4.png" {
			decoded, _, decodeErr := image.Decode(bytes.NewReader(data))
			if decodeErr != nil {
				t.Fatal(decodeErr)
			}
			_, _, _, alpha := decoded.At(0, 0).RGBA()
			if alpha != 0 {
				t.Errorf("%s: transparent PNG corner was flattened", mf.Name)
			}
			_, _, _, centerAlpha := decoded.At(cfg.Width/2, cfg.Height/2).RGBA()
			if centerAlpha < 127*257 || centerAlpha > 129*257 {
				t.Errorf("%s: half-transparent center alpha %d was changed", mf.Name, centerAlpha)
			}
		}
		if r.width == nil || r.height == nil || *r.width != cfg.Width || *r.height != cfg.Height {
			t.Errorf("%s: width/height columns %v x %v disagree with the decoded %d x %d", mf.Name, r.width, r.height, cfg.Width, cfg.Height)
		}
		if mf.EXIF {
			if cfg.Height <= cfg.Width || cfg.Height > 2000 {
				t.Errorf("%s: stored %dx%d, want a portrait rotation with the longest side <= 2000", mf.Name, cfg.Width, cfg.Height)
			}
		} else if cfg.Width != mf.FitW || cfg.Height != mf.FitH {
			t.Errorf("%s: stored %dx%d, want %dx%d", mf.Name, cfg.Width, cfg.Height, mf.FitW, mf.FitH)
		}
	}
	if counts["main"] != 4 || counts["detail"] != 6 || counts["sku"] != 2 {
		t.Errorf("role counts main=%d detail=%d sku=%d, want 4/6/2", counts["main"], counts["detail"], counts["sku"])
	}
	// The option-value links and the default image axis.
	links := map[string]string{}
	linkRows, err := f.owner.Query(ctx, `SELECT option_name,option_value,image_id::text FROM catalog.product_option_images WHERE product_id=$1`, product.ID)
	if err != nil {
		t.Fatal(err)
	}
	for linkRows.Next() {
		var name, value, id string
		if err := linkRows.Scan(&name, &value, &id); err != nil {
			t.Fatal(err)
		}
		if name != "Color" {
			t.Errorf("link on axis %q, want Color", name)
		}
		links[value] = id
	}
	linkRows.Close()
	for value := range product.SKUs {
		linkID, ok := links[value]
		if !ok {
			t.Errorf("no option image link for %s", value)
			continue
		}
		mf, ok := browserIDs[linkID]
		if !ok || manifest.Files.SKU[value].Name != mf.Name {
			t.Errorf("option %s links image %s (%v), want the browser-uploaded %s file", value, linkID, ok, manifest.Files.SKU[value].Name)
		}
	}
	var axis *string
	var status string
	if err := f.owner.QueryRow(ctx, `SELECT image_axis,status FROM catalog.products WHERE id=$1`, product.ID).Scan(&axis, &status); err != nil {
		t.Fatal(err)
	}
	if axis != nil {
		t.Errorf("image_axis=%q, want NULL (the default first axis; the merchant never changed it)", *axis)
	}
	if status != "active" {
		t.Errorf("product status=%q after the UI publish, want active", status)
	}
	// The migrated fixture is read-only for the browsers: rows unchanged, still 4 main + 5 detail in order.
	mig := stored(migrated.ID)
	var migMain, migDetail []string
	for _, r := range mig {
		if r.role == "main" {
			migMain = append(migMain, r.id)
		}
		if r.role == "detail" {
			migDetail = append(migDetail, r.id)
		}
	}
	if strings.Join(migMain, ",") != strings.Join(migrated.Main, ",") || strings.Join(migDetail, ",") != strings.Join(migrated.Detail, ",") {
		t.Errorf("migrated fixture rows changed under browsing: %+v", mig)
	}
	if n := countRows(t, f.owner, `SELECT count(*) FROM checkout.orders WHERE store_id=$1`, f.storeA1); n != ordersBefore {
		t.Errorf("the checkout reach placed %d orders (none allowed: the gate stops at the payment step)", n-ordersBefore)
	}
	if n := countRows(t, f.owner, `SELECT count(*) FROM inventory.reservations WHERE store_id=$1`, f.storeA1); n != reservationsBefore {
		t.Errorf("the checkout reach created %d reservations (none allowed)", n-reservationsBefore)
	}
	// Click ledger: every asserted control recorded with pass=true (a failed expect aborts the spec before its entry,
	// so a short/absent ledger or a run error is itself the failure signal; here every recorded entry must be a pass).
	var ledger []struct {
		Page, Control, Action, Expect, Actual string
		Pass                                  bool
	}
	rawLedger, err := os.ReadFile(filepath.Join(evidence, "click-ledger.json"))
	if err != nil || json.Unmarshal(rawLedger, &ledger) != nil || len(ledger) != result.Cases {
		t.Fatalf("click ledger missing or does not match the case count (%d cases): %v; evidence=%s", result.Cases, err, evidence)
	}
	for _, entry := range ledger {
		if !entry.Pass {
			t.Errorf("click ledger failure: %+v", entry)
		}
	}
	idp.mu.Lock()
	exchanges := idp.exchanges
	idp.mu.Unlock()
	if exchanges < 1 {
		t.Fatal("expected a real signed MOCK IdP exchange for the merchant")
	}
	t.Logf("BROWSER (MOCK IdP) product-media-v2 gate passed: 12 real uploads (4 main, 6 detail incl. 750x4000 tall and a downsized > 2 MiB EXIF photo, 2 option), publish, buyer zh-TW/en at1440/390, migrated 9-image layout; evidence=%s", evidence)
}
