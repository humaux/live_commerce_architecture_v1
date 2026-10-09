//go:build browser

// Purpose: Verify persisted catalog facts after the real admin catalog/product-editor browser workflow.
// Depends on: isolated PostgreSQL foundation fixture, production Next/Go stack and Playwright merchant clicks.
// Used by: --browser-catalog-core and --browser-product-editor gates; all fixture writes remain caller-owned.
package foundation_test

// CC12 (contracts/storefront-v2.md section A acceptance CC12): the catalog v2 admin pages in real Chromium, desktop and 390 px, zh-TW and en.
//
// Stack: an isolated PG 18 (shared foundation fixture), the real private Go API (identity + admin handlers incl. the catalog v2 routes),
// the production admin Next build behind a signed mock OIDC issuer, and the real buyerhttp handler of the published fixture store. The
// Playwright spec (tests/admin/catalog-core.spec.ts) is the merchant: it creates a product as a draft, builds a 2 x 3 option matrix, prices
// it, sets compare-at and stock, uploads a photo, adds it to a new collection, activates it, archives one variant and deactivates it. After
// every step it asks the BUYER catalog v2 HTTP (through the runner-only control listener: Node never holds the BFF key or a database
// credential) what a shopper would see. Labels: BROWSER + REAL_PG; the buyer side is the real handler over HTTP, not the storefront Next
// build (storefront pages are unit storefront-shell). Evidence: output/playwright/catalog-core/<timestamp>/.

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func ccbRequire(t *testing.T) {
	t.Helper()
	if os.Getenv("LC_BROWSER_CATALOG_CORE_ACCEPTANCE") != "1" || os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" {
		t.Fatal("use scripts/dev/test-local.sh --browser-catalog-core")
	}
}

// ccbStartAdmin starts the private Go API (identity + admin routes over the ISOLATED database) and the production admin Next build against
// it; principal is the merchant that the mock IdP subject maps to.
func ccbStartAdmin(t *testing.T, ctx context.Context, f *testFixture, principal, evidence string) *brfStack {
	t.Helper()
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
	mustExec(t, f.owner, `INSERT INTO identity.external_identities(issuer,subject,principal_id) VALUES($1,'browser-subject',$2)`, idp.server.URL, principal)
	// the spec checks the workspace nav through nav-orders; the role-aware nav (0089, apps/admin/lib/team-model.ts) shows it only with orders:read
	// (the browser run owns its fixture cluster, so the grant needs no cleanup)
	mustExec(t, f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,'orders:read') ON CONFLICT DO NOTHING`, f.tenantA, f.storeA1, principal)
	role := "ccb_" + strings.ReplaceAll(randomUUID(), "-", "")
	password := randomToken()
	mustExec(t, f.owner, `CREATE ROLE `+pgx.Identifier{role}.Sanitize()+` LOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE IN ROLE commerce_identity PASSWORD '`+password+`'`)
	t.Cleanup(func() { mustExec(t, f.owner, `DROP ROLE `+pgx.Identifier{role}.Sanitize()) })
	u, err := url.Parse(f.databaseURL)
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
	service, err := identity.New(authority, observedBrowserProvider{Provider: provider, t: t}, identity.Policy{ProviderKey: "browser-catalog-core-v1", SessionTTL: time.Hour, OnboardingEnabled: true, Currencies: []string{"TWD", "USD"}})
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
	api := httptest.NewServer(mux)
	t.Cleanup(api.Close)
	nextLog := browserLog(t, filepath.Join(evidence, "next.log"))
	next := exec.CommandContext(ctx, "node", filepath.Join(root, "apps/admin/.next/standalone/apps/admin/server.js"))
	next.Dir = root
	next.Env = browserEnvironment(map[string]string{"HOSTNAME": "127.0.0.1", "PORT": port, "NODE_ENV": "production", "COMMERCE_IDENTITY_ENABLED": "1", "COMMERCE_IDENTITY_ALLOW_LOOPBACK_TESTS": "1",
		"COMMERCE_PUBLIC_ORIGIN": origin, "COMMERCE_API_ORIGIN": api.URL, "COMMERCE_OIDC_ISSUER": idp.server.URL, "COMMERCE_BFF_KEY": bffKey, "COMMERCE_ONBOARDING_ENABLED": "1", "COMMERCE_ONBOARDING_CURRENCIES": "TWD,USD"})
	next.Stdout, next.Stderr = nextLog, nextLog
	next.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := next.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = syscall.Kill(-next.Process.Pid, syscall.SIGKILL); _, _ = next.Process.Wait() })
	client := &http.Client{Timeout: time.Second}
	for attempt := 0; ; attempt++ {
		if response, err := client.Get(origin + "/api/stores"); err == nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusUnauthorized {
				break
			}
		}
		if attempt > 100 {
			t.Fatalf("admin Next readiness failed; evidence=%s", evidence)
		}
		time.Sleep(100 * time.Millisecond)
	}
	return &brfStack{root: root, evidence: evidence, origin: origin, api: api}
}

func TestBrowserCatalogCore(t *testing.T) {
	ccbRequire(t)
	if os.Getenv("PRODUCT_EDITOR_ACCEPTANCE") == "1" {
		t.Run("PE12-17-TWD", func(t *testing.T) { runCatalogCoreBrowser(t, true) })
	}
	t.Run("CC12-frozen", func(t *testing.T) { runCatalogCoreBrowser(t, false) })
}

func runCatalogCoreBrowser(t *testing.T, productEditor bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 24*time.Minute)
	defer cancel()
	t04Fixture(t)
	h := bhSetup(t) // published fixture store A1 + the real buyerhttp handler
	f := h.f
	tag := "cc" + strings.ToLower(t04Tag())
	store, tenant := f.storeA1, f.tenantA
	principal := f.principalA
	if productEditor {
		// Use the existing isolated TWD tenant, never mutate a store currency
		// beneath its markets/SKUs or change frozen CC12 monetary assertions.
		store, tenant = f.storeB, f.tenantB
		if err := f.owner.QueryRow(ctx, `SELECT principal_id::text FROM identity.sessions WHERE token_hash=$1`, tokenHash(f.tokens["b"])).Scan(&principal); err != nil {
			t.Fatal(err)
		}
		bhPublish(t, h.bcHarness, "https://pe-buyer.example", tenant, store)
		h.origin = "https://pe-buyer.example"
		mustExec(t, f.owner, `INSERT INTO inventory.warehouses(tenant_id,store_id,name) SELECT $1,$2,'PE fixture warehouse' WHERE NOT EXISTS(SELECT 1 FROM inventory.warehouses WHERE tenant_id=$1 AND store_id=$2 AND active)`, tenant, store)
	}
	// Synthetic one-warehouse store for §g target_qty UI. Multi-warehouse refusal is covered separately by PE22 and UI negatives.
	mustExec(t, f.owner, `UPDATE inventory.warehouses SET active=(id=(SELECT id FROM inventory.warehouses WHERE tenant_id=$1 AND store_id=$2 AND active ORDER BY id LIMIT 1)) WHERE tenant_id=$1 AND store_id=$2`, tenant, store)
	t.Cleanup(func() {
		mustExec(t, f.owner, `UPDATE catalog.products SET status='archived' WHERE tenant_id=$1 AND store_id=$2 AND name LIKE $3`, tenant, store, tag+"%")
		mustExec(t, f.owner, `UPDATE catalog.collections SET status='hidden' WHERE tenant_id=$1 AND store_id=$2 AND title LIKE $3`, tenant, store, tag+"%")
	})
	// paging fixture: more than one page (25) of drafts the merchant list must walk through (owner-seeded, synthetic)
	mustExec(t, f.owner, `INSERT INTO catalog.products(tenant_id,store_id,id,name,description,status)
		SELECT $1,$2,gen_random_uuid(),$3||' paging '||lpad(g::text,3,'0'),'seeded for list paging','draft' FROM generate_series(1,60) g`, tenant, store, tag)

	// runner-only control listener: the browser spec asks "what does a shopper see" without holding the BFF key.
	controlKey := randomToken()
	control := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Query().Get("path")
		if r.Header.Get("X-Gate-Key") != controlKey || r.Method != http.MethodGet || !(strings.HasPrefix(path, "/v1/buyer/catalog/v2/") || strings.HasPrefix(path, "/v1/buyer/media/")) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		req, err := http.NewRequest(http.MethodGet, h.server.URL+path, nil)
		if err != nil {
			http.Error(w, "bad path", http.StatusBadRequest)
			return
		}
		req.Header.Set("X-Commerce-Buyer-BFF-Key", h.key)
		req.Header.Set("X-Commerce-Storefront-Origin", h.origin)
		res, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
		if err != nil {
			http.Error(w, "upstream", http.StatusBadGateway)
			return
		}
		defer res.Body.Close()
		data, _ := io.ReadAll(io.LimitReader(res.Body, 4<<20))
		body := ""
		if strings.HasPrefix(res.Header.Get("Content-Type"), "application/json") {
			body = string(data)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"status": res.StatusCode, "contentType": res.Header.Get("Content-Type"), "cacheControl": res.Header.Get("Cache-Control"), "length": len(data), "body": body})
	}))
	t.Cleanup(control.Close)

	root, _ := filepath.Abs("../..")
	evidence := brfEvidence(t, root, "catalog-core")
	adminFixture := *f
	adminFixture.tenantA, adminFixture.storeA1 = tenant, store
	stack := ccbStartAdmin(t, ctx, &adminFixture, principal, evidence)
	peFlag := "0"
	if productEditor {
		peFlag = "1"
	}
	env := map[string]string{"LC_BROWSER_STORE": store, "LC_BROWSER_TAG": tag, "LC_BROWSER_CONTROL": control.URL, "LC_BROWSER_CONTROL_KEY": controlKey, "PRODUCT_EDITOR_ACCEPTANCE": peFlag}
	if d := os.Getenv("LC_BROWSER_DIAGNOSTIC"); d != "" { // diagnostic only, see the spec: never part of the gate
		env["LC_BROWSER_DIAGNOSTIC"] = d
	}
	brfPlaywright(t, ctx, stack, []string{"catalog-core.spec.ts"}, env)
	if productEditor {
		ledgerPath := filepath.Join(evidence, "product-editor-click-ledger.json")
		ledger, err := os.ReadFile(ledgerPath)
		if err != nil {
			t.Fatalf("PE mobile matrix ledger unavailable: %v; evidence=%s", err, evidence)
		}
		if err := validateProductEditorMatrixLedger(ledger); err != nil {
			t.Fatalf("PE mobile matrix ledger rejected: %v; evidence=%s", err, evidence)
		}
		t.Logf("PASS mobile matrix click ledger: %d declared unique PASS identities; evidence=%s", len(productEditorMatrixLedgerExpected()), ledgerPath)
		prefix := "pe" + tag
		// Preserve the original exact four-product fence. Exclude only the
		// three explicitly named mobile matrices and three axis-removal
		// products, each validated below. No broad name-pattern exclusion.
		if n := countRows(t, f.owner, `SELECT count(*) FROM catalog.products WHERE tenant_id=$1 AND store_id=$2 AND name LIKE $3 AND name NOT IN ($4,$5,$6,$7,$8,$9)`, tenant, store, prefix+"%", prefix+" mobile matrix zh-TW", prefix+" mobile matrix zh-CN", prefix+" mobile matrix en", prefix+" mobile axis remove zh-TW", prefix+" mobile axis remove zh-CN", prefix+" mobile axis remove en"); n != 4 {
			t.Fatalf("PE: expected 4 products including the single committed lost-response copy, got %d", n)
		}
		if n := countRows(t, f.owner, `SELECT count(*) FROM catalog.products WHERE tenant_id=$1 AND store_id=$2 AND name LIKE $3 AND name NOT IN ($4,$5,$6)`, tenant, store, prefix+"%", prefix+" mobile axis remove zh-TW", prefix+" mobile axis remove zh-CN", prefix+" mobile axis remove en"); n != 7 {
			t.Fatalf("PE: expected original 4 plus exactly 3 mobile matrices excluding named axis-removal products, got %d products", n)
		}
		if n := countRows(t, f.owner, `SELECT count(*) FROM catalog.products WHERE tenant_id=$1 AND store_id=$2 AND name LIKE $3`, tenant, store, prefix+"%"); n != 10 {
			t.Fatalf("PE: expected original 7 plus exactly 3 axis-removal products, got %d products", n)
		}
		if n := countRows(t, f.owner, `SELECT count(*) FROM catalog.skus s JOIN catalog.products p ON p.id=s.product_id WHERE p.tenant_id=$1 AND p.store_id=$2 AND p.name=$3 AND s.price_minor=8000 AND s.currency='TWD' AND s.inventory_tracked`, tenant, store, prefix+" single"); n != 1 {
			t.Fatal("PE: exact TWD single SKU readback failed")
		}
		if n := countRows(t, f.owner, `SELECT count(*) FROM catalog.skus s JOIN catalog.products p ON p.id=s.product_id WHERE p.tenant_id=$1 AND p.store_id=$2 AND p.name=$3`, tenant, store, prefix+" matrix"); n != 12 {
			t.Fatal("PE: matrix does not have 12 persisted SKUs")
		}
		for _, locale := range []struct{ name, index string }{{"zh-TW", "0"}, {"zh-CN", "1"}, {"en", "2"}} {
			name := prefix + " mobile matrix " + locale.name
			if n := countRows(t, f.owner, `SELECT count(*) FROM catalog.products WHERE tenant_id=$1 AND store_id=$2 AND name=$3`, tenant, store, name); n != 1 {
				t.Fatalf("PE mobile %s: expected exactly one persisted product, got %d", locale.name, n)
			}
			if n := countRows(t, f.owner, `SELECT count(*) FROM catalog.skus s JOIN catalog.products p ON p.id=s.product_id WHERE p.tenant_id=$1 AND p.store_id=$2 AND p.name=$3`, tenant, store, name); n != 2 {
				t.Fatalf("PE mobile %s: expected exactly two persisted SKUs, got %d", locale.name, n)
			}
			for _, row := range []struct {
				option, index  string
				price, compare int64
				tracked        bool
			}{{"White", "0", 8000, 12000, false}, {"Black", "1", 8100, 12100, true}} {
				if n := countRows(t, f.owner, `SELECT count(*) FROM catalog.skus s
					JOIN catalog.products p ON p.id=s.product_id AND p.tenant_id=s.tenant_id AND p.store_id=s.store_id
					WHERE p.tenant_id=$1 AND p.store_id=$2 AND p.name=$3 AND p.status='draft'
					AND s.option_values=ARRAY[$4::text,'S'] AND s.price_minor=$5 AND s.compare_at_minor=$6
					AND s.currency='TWD' AND s.status=CASE WHEN $7::boolean THEN 'active' ELSE 'archived' END AND s.inventory_tracked=$7
					AND (($7::boolean AND s.max_per_order IS NULL AND
					  (SELECT coalesce(sum(b.on_hand),0) FROM inventory.balances b
					   WHERE b.tenant_id=s.tenant_id AND b.store_id=s.store_id AND b.sku_id=s.id)=8)
					 OR (NOT $7::boolean AND s.max_per_order=7))
					AND s.code ~ ('^M[0-9]{10}-' || $8::text || '-' || $9::text || '$')
					AND (($7::boolean AND EXISTS (SELECT 1 FROM live.keyword_library k
					  WHERE k.tenant_id=s.tenant_id AND k.store_id=s.store_id AND k.sku_id=s.id
					  AND k.keyword=split_part(s.code,'-',1) || $8::text || 'E' || $9::text))
					 OR (NOT $7::boolean AND NOT EXISTS (SELECT 1 FROM live.keyword_library k
					  WHERE k.tenant_id=s.tenant_id AND k.store_id=s.store_id AND k.sku_id=s.id)))`,
					tenant, store, name, row.option, row.price, row.compare, row.tracked, locale.index, row.index); n != 1 {
					t.Fatalf("PE mobile %s %s/S: exact edited price/compare/stock/code/keyword/active readback failed (matches=%d)", locale.name, row.option, n)
				}
			}
			name = prefix + " mobile axis remove " + locale.name
			sizeName := map[string]string{"zh-TW": "尺寸", "zh-CN": "尺寸", "en": "Size"}[locale.name]
			if n := countRows(t, f.owner, `SELECT count(*) FROM catalog.products
				WHERE tenant_id=$1 AND store_id=$2 AND name=$3 AND status='draft'
				AND options=jsonb_build_array(jsonb_build_object('name',$4::text,'values',jsonb_build_array('S')))`, tenant, store, name, sizeName); n != 1 {
				t.Fatalf("PE axis remove %s: expected exactly one product with only Size[S], got %d", locale.name, n)
			}
			if n := countRows(t, f.owner, `SELECT count(*) FROM catalog.skus s
				JOIN catalog.products p ON p.id=s.product_id AND p.tenant_id=s.tenant_id AND p.store_id=s.store_id
				WHERE p.tenant_id=$1 AND p.store_id=$2 AND p.name=$3`, tenant, store, name); n != 3 {
				t.Fatalf("PE axis remove %s: expected two archived old SKUs plus one replacement, got %d", locale.name, n)
			}
			for _, row := range []struct {
				option, index            string
				price, compare, quantity int64
			}{{"White", "0", 9000, 13000, 3}, {"Black", "1", 9100, 13100, 4}} {
				if n := countRows(t, f.owner, `SELECT count(*) FROM catalog.skus s
					JOIN catalog.products p ON p.id=s.product_id AND p.tenant_id=s.tenant_id AND p.store_id=s.store_id
					WHERE p.tenant_id=$1 AND p.store_id=$2 AND p.name=$3
					AND s.option_values=ARRAY[$4::text,'S'] AND s.status='archived'
					AND s.price_minor=$5 AND s.compare_at_minor=$6 AND s.currency='TWD'
					AND s.inventory_tracked AND s.max_per_order IS NULL
					AND s.code ~ ('^M[0-9]{10}-A' || $7::text || '-' || $8::text || '$')
					AND (SELECT coalesce(sum(b.on_hand),0) FROM inventory.balances b
					  WHERE b.tenant_id=s.tenant_id AND b.store_id=s.store_id AND b.sku_id=s.id)=$9`,
					tenant, store, name, row.option, row.price, row.compare, locale.index, row.index, row.quantity); n != 1 {
					t.Fatalf("PE axis remove %s %s/S: old SKU not archived with original values intact (matches=%d)", locale.name, row.option, n)
				}
			}
			if n := countRows(t, f.owner, `SELECT count(*) FROM catalog.skus s
				JOIN catalog.products p ON p.id=s.product_id AND p.tenant_id=s.tenant_id AND p.store_id=s.store_id
				WHERE p.tenant_id=$1 AND p.store_id=$2 AND p.name=$3 AND s.status='active'
				AND s.option_values=ARRAY['S'] AND s.price_minor=9500 AND s.compare_at_minor=14500
				AND s.currency='TWD' AND s.inventory_tracked AND s.max_per_order IS NULL
				AND s.code=p.slug AND s.code<>''
				AND (SELECT coalesce(sum(b.on_hand),0) FROM inventory.balances b
				  WHERE b.tenant_id=s.tenant_id AND b.store_id=s.store_id AND b.sku_id=s.id)=9`, tenant, store, name); n != 1 {
				t.Fatalf("PE axis remove %s: exact active Size[S] replacement price/compare/stock/code readback failed (matches=%d)", locale.name, n)
			}
		}
		t.Logf("PASS PE12-17: TWD tenant, original 4 products plus 3 mobile matrices and 3 real axis-removal products with exact archived/replacement SKU readback; evidence=%s", evidence)
		return
	}

	// ---- PG facts after the browser run: the UI drove real, audited, versioned commands ----
	count := func(q string, args ...any) int { return countRows(t, f.owner, q, args...) }
	for _, variant := range []string{"d", "m"} { // desktop/en and mobile/zh-TW run with their own product
		name := tag + variant + "%"
		var product, status, slug string
		if err := f.owner.QueryRow(ctx, `SELECT id::text,status,slug FROM catalog.products WHERE tenant_id=$1 AND store_id=$2 AND name LIKE $3`, tenant, store, name).Scan(&product, &status, &slug); err != nil {
			t.Fatalf("CC12 %s: the product created in the browser is missing: %v; evidence=%s", variant, err, evidence)
		}
		if status != "draft" {
			t.Errorf("CC12 %s: the run ends with the product deactivated (draft), got %s", variant, status)
		}
		if n := count(`SELECT count(*) FROM catalog.skus WHERE product_id=$1 AND status='active'`, product); n != 5 {
			t.Errorf("CC12 %s: %d active variants, want 5 (2 x 3 matrix minus the archived one)", variant, n)
		}
		if n := count(`SELECT count(*) FROM catalog.skus WHERE product_id=$1 AND status='archived'`, product); n != 1 {
			t.Errorf("CC12 %s: %d archived variants, want exactly the one the merchant archived", variant, n)
		}
		if n := count(`SELECT count(*) FROM catalog.skus WHERE product_id=$1 AND cardinality(option_values)=2`, product); n != 6 {
			t.Errorf("CC12 %s: %d variants carry two aligned option values, want all 6", variant, n)
		}
		if n := count(`SELECT count(*) FROM catalog.skus WHERE product_id=$1 AND compare_at_minor IS NOT NULL AND compare_at_minor > price_minor`, product); n != 1 {
			t.Errorf("CC12 %s: %d variants with a compare-at price, want 1", variant, n)
		}
		var onHand int64
		if err := f.owner.QueryRow(ctx, `SELECT coalesce(sum(b.on_hand),0) FROM inventory.balances b JOIN catalog.skus s ON s.tenant_id=b.tenant_id AND s.store_id=b.store_id AND s.id=b.sku_id WHERE s.product_id=$1`, product).Scan(&onHand); err != nil || onHand != 24 {
			t.Errorf("CC12 %s: on-hand over the matrix = %d (%v), want 24 (10+3+0+6+5+0 from the stock dialogs)", variant, onHand, err)
		}
		if n := count(`SELECT count(*) FROM catalog.product_images WHERE product_id=$1`, product); n != 1 {
			t.Errorf("CC12 %s: %d photos, want 1", variant, n)
		}
		if n := count(`SELECT count(*) FROM catalog.collection_products cp JOIN catalog.collections c ON c.id=cp.collection_id AND c.tenant_id=cp.tenant_id AND c.store_id=cp.store_id WHERE cp.product_id=$1 AND c.title LIKE $2`, product, tag+variant+"%"); n != 1 {
			t.Errorf("CC12 %s: the product is in %d of the new collections, want 1", variant, n)
		}
		if n := count(`SELECT count(*) FROM catalog.collection_images i JOIN catalog.collections c ON c.id=i.collection_id AND c.tenant_id=i.tenant_id AND c.store_id=i.store_id WHERE c.title LIKE $1`, tag+variant+"%"); n != 1 {
			t.Errorf("CC12 %s: %d collection images, want 1", variant, n)
		}
		if slug == "" || !strings.HasPrefix(slug, tag+variant) {
			t.Errorf("CC12 %s: the generated slug %q must come from the title", variant, slug)
		}
	}
	cbbrShots(t, evidence, 16, "en", "zh-TW")
	t.Logf("CC12 BROWSER+REAL_PG: admin catalog v2 pages passed (desktop+390px, en+zh-TW); screenshots hashed; evidence=%s", evidence)
}
