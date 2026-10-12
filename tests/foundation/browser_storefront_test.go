//go:build browser

package foundation_test

// TestBrowserStorefront: the buyer storefront shell (apps/storefront) on the REAL stack, SANDBOX tier: production Next build -> real
// buyerhttp handler -> real PG, with the shop built the way a merchant builds it, through the real admin HTTP API (httpapi): 28 active
// products with option axes, compare-at prices, stock, photos, a draft and an archived product, two collections (one with a photo), a
// designed home page saved and published (plus a newer draft behind a preview token), a delivery policy with a free-shipping threshold.
// The storefront publication and the ACTIVE domain come from the migration 0081 definers (merchant publish, operator bind), never from
// owner INSERTs. Run through `bash scripts/dev/test-local.sh --browser-storefront`; the Node side is tests/storefront/shop-real-gate.mjs.
// Not proven here: real DNS/TLS (synthetic CONNECT edge), a payment provider, devices. Owner-pool writes are disclosed fixture setup:
// archiving the shared fixture store's other products so the counts are exact, and the owner readback at the end.
//
// Why a control listener: the gate must flip the merchant publication through the definer (unpublish -> branded closed page ->
// republish) and only Go holds the merchant session; the listener answers nothing else.

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
	"strings"
	"syscall"
	"testing"
	"time"

	"livecommerce/internal/buyerhttp"
	"livecommerce/internal/checkout"
	"livecommerce/internal/httpapi"
)

type sfrVariant struct {
	Values  []string `json:"values"`
	Code    string   `json:"code"`
	Price   int64    `json:"price_minor"`
	Compare int64    `json:"compare_at_minor"` // 0 = none
	Stock   int64    `json:"stock"`            // units on hand: >=6 "in", 1..5 "low", 0 "out"
}

type sfrProduct struct {
	Key         string       `json:"key"`
	ID          string       `json:"id"`
	Slug        string       `json:"slug"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Axes        []v2Axis     `json:"axes"`
	Variants    []sfrVariant `json:"variants"`
	Images      int          `json:"images"`
	Collection  string       `json:"collection"` // collection key or ""
	Status      string       `json:"status"`     // final status: active | draft | archived
}

type sfrCollection struct {
	Key      string `json:"key"`
	ID       string `json:"id"`
	Slug     string `json:"slug"`
	Title    string `json:"title"`
	ImageID  string `json:"image_id"`
	Products int    `json:"active_products"`
}

// sfrFacts is everything the browser gate asserts against (it never reads PG).
type sfrFacts struct {
	Origin       string          `json:"origin"`
	Currency     string          `json:"currency"`
	StoreName    string          `json:"store_name"`
	Products     []sfrProduct    `json:"products"`
	Collections  []sfrCollection `json:"collections"`
	ActiveCount  int             `json:"active_count"`
	Design       map[string]any  `json:"design"`
	PreviewToken string          `json:"preview_token"`
	FreeShipping struct {
		ThresholdMinor int64 `json:"threshold_minor"`
		FeeMinor       int64 `json:"fee_minor"`
	} `json:"free_shipping"`
}

func TestBrowserStorefront(t *testing.T) {
	if os.Getenv("LC_BROWSER_STOREFRONT_ACCEPTANCE") != "1" || os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" {
		t.Fatal("use scripts/dev/test-local.sh --browser-storefront")
	}
	// hpSetup = bcSetup (PG, market + delivery service + allocation, buyer pools) plus a hosted-payment starter, so the process can take card
	// payment and checkout-options offers the home delivery (OP1: a row nobody can pay is not offered). bhSetup minus the owner-seeded publication.
	hp := hpSetup(t)
	b := hp.bcHarness
	h := bhHarness{bcHarness: b, key: brToken(), origin: sfrOrigin}
	handler, err := buyerhttp.New(context.Background(), b.a.issuer, b.a.runtime, b.service, h.key, time.Hour, hp.api.(*checkout.HostedPaymentStarter))
	if err != nil {
		t.Fatal("buyer HTTP constructor failed")
	}
	h.server = httptest.NewServer(handler)
	t.Cleanup(h.server.Close)
	f := h.f
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()
	root, _ := filepath.Abs("../..")
	evidence := brfEvidence(t, root, "storefront")

	// The shared fixture store holds other tests' products: archive them so the shop's counts are exactly the merchant-created ones.
	mustExec(t, f.owner, `UPDATE catalog.products SET status='archived' WHERE tenant_id=$1 AND store_id=$2`, f.tenantA, f.storeA1)

	admin := v2Admin{t: t, h: httpapi.NewHandler(f.runtime), token: f.tokens["a"], store: f.storeA1}
	sdgGrant(t, f, f.tokens["a"], f.storeA1, "integration:read", "integration:manage")
	var warehouse struct{ ID string }
	admin.do("POST", "/warehouses", t04Key("sfr-wh"), map[string]any{"name": "sfr-" + t04Tag()}, 200, &warehouse)

	// ---- catalog, through the merchant API ------------------------------------------------------------------------------------
	facts := sfrFacts{Origin: sfrOrigin, Currency: b.stock.skus[0].Currency, StoreName: "fixture-store-a1"}
	collections := []*sfrCollection{
		{Key: "fragrance", Slug: "home-fragrance", Title: "Home fragrance"},
		{Key: "tableware", Slug: "tableware", Title: "Tableware"},
	}
	byKey := map[string]*sfrCollection{}
	for _, c := range collections {
		var made struct {
			ID      string `json:"id"`
			Version int64  `json:"version"`
		}
		admin.do("POST", "/collections", t04Key("sfr-col-"+c.Key), map[string]any{"title": c.Title, "slug": c.Slug, "description": c.Title + " picks"}, 200, &made)
		c.ID = made.ID
		byKey[c.Key] = c
	}
	money := func(whole int64) int64 { return whole * 100 }
	products := []sfrProduct{
		{Key: "candle", Slug: "cedar-fig-candle", Title: "Cedar Fig Candle", Description: "Hand-poured soy wax with a dry cedar and ripe fig note.\n\nBurns about 40 hours.",
			Axes: []v2Axis{{Name: "Size", Values: []string{"180g", "300g"}}}, Images: 2, Collection: "fragrance", Status: "active", Variants: []sfrVariant{
				{Values: []string{"180g"}, Code: "CAN-180", Price: money(68), Stock: 12}, {Values: []string{"300g"}, Code: "CAN-300", Price: money(98), Stock: 3}}},
		{Key: "scarf", Slug: "wool-knit-scarf", Title: "Wool Knit Scarf", Description: "Light wool-silk blend.",
			Axes: []v2Axis{{Name: "Color", Values: []string{"Oat", "Black", "Blue"}}}, Images: 3, Status: "active", Variants: []sfrVariant{
				{Values: []string{"Oat"}, Code: "SCF-OAT", Price: money(148), Compare: money(198), Stock: 9}, {Values: []string{"Black"}, Code: "SCF-BLK", Price: money(148), Compare: money(198), Stock: 0},
				{Values: []string{"Blue"}, Code: "SCF-BLU", Price: money(148), Compare: money(198), Stock: 4}}},
		{Key: "diffuser", Slug: "reed-diffuser-box", Title: "Reed Diffuser Box", Description: "100ml diffuser with six reeds.", Images: 1, Collection: "fragrance", Status: "active",
			Variants: []sfrVariant{{Code: "DIF-100", Price: money(89), Stock: 0}}},
		{Key: "dripper", Slug: "ceramic-dripper-set", Title: "Ceramic Dripper Set", Description: "Glazed ceramic dripper with a matching server.", Images: 1, Collection: "tableware", Status: "active",
			Variants: []sfrVariant{{Code: "DRP-001", Price: money(128), Stock: 20}}},
	}
	for i := 1; i <= 24; i++ { // fillers: the list must paginate (page size 24), prices spread for sort and filter
		products = append(products, sfrProduct{Key: fmt.Sprintf("item%02d", i), Slug: fmt.Sprintf("item-%02d", i), Title: fmt.Sprintf("Everyday Item %02d", i), Description: "Everyday item.", Images: 1,
			Collection: map[bool]string{true: "tableware", false: ""}[i <= 10], Status: "active", Variants: []sfrVariant{{Code: fmt.Sprintf("ITM-%02d", i), Price: money(10) + int64(i)*150, Stock: 10}}})
	}
	products = append(products,
		sfrProduct{Key: "draft", Slug: "secret-draft-mug", Title: "Secret Draft Mug", Description: "Not for sale yet.", Images: 1, Status: "draft", Variants: []sfrVariant{{Code: "DRF-001", Price: money(30), Stock: 5}}},
		sfrProduct{Key: "retired", Slug: "retired-plate", Title: "Retired Plate", Description: "No longer sold.", Images: 1, Status: "archived", Variants: []sfrVariant{{Code: "RET-001", Price: money(40), Stock: 5}}})
	for i := range products {
		p := &products[i]
		var made struct {
			ID      string `json:"id"`
			Version int64  `json:"version"`
		}
		body := map[string]any{"name": p.Title, "slug": p.Slug, "description": p.Description, "seo_title": p.Title + " | shop", "seo_description": p.Description}
		if p.Status != "draft" {
			body["status"] = "active"
		}
		if len(p.Axes) > 0 {
			body["options"] = p.Axes
		}
		admin.do("POST", "/products", t04Key("sfr-p-"+p.Key), body, 200, &made)
		p.ID = made.ID
		for _, v := range p.Variants {
			sku := map[string]any{"product_id": p.ID, "code": v.Code, "price_minor": v.Price, "weight_grams": 300, "length_mm": 100, "width_mm": 100, "height_mm": 50, "option_values": v.Values}
			if v.Compare > 0 {
				sku["compare_at_minor"] = v.Compare
			}
			var s v2SKU
			admin.do("POST", "/skus", t04Key("sfr-sku-"+v.Code), sku, 200, &s)
			if v.Stock > 0 {
				admin.do("POST", "/inventory/adjustments", t04Key("sfr-adj-"+v.Code), map[string]any{"warehouse_id": warehouse.ID, "sku_id": s.ID, "delta": v.Stock, "expected_version": 0, "reason": "storefront gate"}, 200, nil)
			}
		}
		for n := 0; n < p.Images; n++ {
			// S1 performance uses this exact deterministic unpadded source both before and after.
			photo := v2PNG(t, uint8(10+n+i))
			if os.Getenv("LC_MEDIA_SIZES_PHASE") != "" {
				photo = cmiJPEG(t, 1440, 1800, uint8(10+n+i))
			}
			data, ctype := v2Multipart(t, photo)
			admin.raw("POST", "/products/"+p.ID+"/images", t04Key("sfr-img"), data, ctype, 200, nil)
		}
		if p.Status == "archived" {
			admin.do("PATCH", "/products/"+p.ID, t04Key("sfr-archive"), map[string]any{"expected_version": made.Version, "status": "archived"}, 200, nil)
		}
		if c := byKey[p.Collection]; c != nil && p.Status == "active" {
			c.Products++
		}
	}
	for _, c := range collections { // membership in one call per collection, then the photo (the amended read returns its id)
		var ids []string
		for _, p := range products {
			if p.Collection == c.Key && p.Status == "active" {
				ids = append(ids, p.ID)
			}
		}
		var made struct {
			Version int64 `json:"version"`
		}
		admin.do("GET", "/collections/"+c.ID, "", nil, 200, &made)
		admin.do("PUT", "/collections/"+c.ID+"/products", t04Key("sfr-set-"+c.Key), map[string]any{"expected_version": made.Version, "product_ids": ids}, 200, nil)
	}
	{
		data, ctype := v2Multipart(t, v2PNG(t, 77))
		var img struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(admin.raw("POST", "/collections/"+byKey["fragrance"].ID+"/image", t04Key("sfr-cimg"), data, ctype, 200, nil).Body.Bytes(), &img)
		byKey["fragrance"].ImageID = img.ID
	}
	for _, p := range products {
		if p.Status == "active" {
			facts.ActiveCount++
		}
	}
	facts.Products = products
	for _, c := range collections {
		facts.Collections = append(facts.Collections, *c)
	}

	// ---- delivery policy with a free-shipping threshold (producer: pricing policy, read by checkout-options) -------------------------
	facts.FreeShipping.ThresholdMinor, facts.FreeShipping.FeeMinor = money(200), money(6)
	{
		var version int64
		code := b.delivery.Code
		if err := f.owner.QueryRow(ctx, `SELECT current_version FROM pricing.policy_heads WHERE tenant_id=$1 AND store_id=$2 AND method=$3`, f.tenantA, f.storeA1, "delivery:"+code).Scan(&version); err != nil {
			t.Fatal(err)
		}
		p := b.policy
		fee, threshold := facts.FreeShipping.FeeMinor, facts.FreeShipping.ThresholdMinor
		p.Method, p.ExpectedVersion, p.ShippingMinor, p.Enabled, p.FreeShippingThresholdMinor = "delivery:"+code, version, &fee, true, &threshold
		policy, err := b.setPolicy(p)
		if err != nil {
			t.Fatalf("set delivery policy: %v", err)
		}
		// A delivery service is pinned to a policy version: re-pin it, as the merchant's delivery settings do, or checkout-options would
		// keep reading the old version (and offer no row once the pinned version is no longer the head's).
		service := b.delivery
		service.ExpectedVersion, service.PolicyVersion = 1, policy.Version
		if _, err := dsSet(b.cqHarness, t04Key("sfr-service"), service); err != nil {
			t.Fatalf("re-pin delivery service: %v", err)
		}
	}

	// ---- design: saved + published, then a newer draft behind a preview token ---------------------------------------------------
	designCall := func(method, path string, payload any, want int, out any) {
		t.Helper()
		var data []byte
		if payload != nil {
			data, _ = json.Marshal(payload)
		}
		r := httptest.NewRequest(method, "/v1/admin/stores/"+f.storeA1+"/design"+path, bytes.NewReader(data))
		r.Header.Set("Authorization", "Bearer "+f.tokens["a"])
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		httpapi.NewHandler(f.runtime).ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("design %s %s: %d %s", method, path, w.Code, w.Body.String())
		}
		if out != nil {
			if err := json.Unmarshal(w.Body.Bytes(), out); err != nil {
				t.Fatal(err)
			}
		}
	}
	upload := func(w int) string {
		t.Helper()
		var buf bytes.Buffer
		buf.Write(sdgPNG(t, w))
		data, ctype := v2Multipart(t, buf.Bytes())
		r := httptest.NewRequest("POST", "/v1/admin/stores/"+f.storeA1+"/design/media", bytes.NewReader(data))
		r.Header.Set("Authorization", "Bearer "+f.tokens["a"])
		r.Header.Set("Content-Type", ctype)
		rec := httptest.NewRecorder()
		httpapi.NewHandler(f.runtime).ServeHTTP(rec, r)
		var m struct {
			ID string `json:"id"`
		}
		if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &m) != nil {
			t.Fatalf("design media upload: %d %s", rec.Code, rec.Body.String())
		}
		return m.ID
	}
	hero, about := upload(40), upload(30)
	doc := func(heading string) map[string]any {
		return map[string]any{
			"profile": map[string]any{"name": "Morning Light Goods", "tagline": "Quiet things for ordinary days", "logo_image_id": nil, "favicon_image_id": nil, "accent_color": "#2f6b5a",
				"announcement": "Free delivery over $200 · live every Thursday 8pm",
				"contact": map[string]any{"email": "hello@morninglight.example", "phone": "+886 2 2345 6789", "address": "100 Heping East Road, Taipei",
					"line_url": "https://line.me/R/ti/p/@morninglight", "facebook_url": "https://www.facebook.com/morninglight.example", "instagram_url": "https://www.instagram.com/morninglight.example"}},
			"nav": map[string]any{
				"header": []any{map[string]any{"label": "All products", "kind": "all_products", "target": nil}, map[string]any{"label": "Home fragrance", "kind": "collection", "target": "home-fragrance"},
					map[string]any{"label": "About us", "kind": "page", "target": "about"}},
				"footer": []any{map[string]any{"label": "Shipping", "kind": "page", "target": "shipping"}, map[string]any{"label": "Tableware", "kind": "collection", "target": "tableware"}}},
			"home": map[string]any{"sections": []any{
				map[string]any{"type": "hero", "image_id": hero, "heading": heading, "subheading": "From dripper to napkin, quiet everyday objects.", "cta_label": "Explore", "cta_kind": "all_products", "cta_target": nil},
				map[string]any{"type": "featured_collection", "collection_slug": "home-fragrance", "heading": "Home fragrance", "limit": 8},
				map[string]any{"type": "rich_text", "heading": "How we choose", "body": "Every item is tested for **at least a month**.\n\n- honest materials\n- repairable\n\nSee [about us](https://example.com/about)."},
				map[string]any{"type": "product_grid", "heading": "Just in", "sort": "newest", "limit": 8},
				map[string]any{"type": "image_text", "image_id": about, "heading": "From small Taiwanese workshops", "body": "We visit ceramic, textile and wood studios.", "image_side": "right"}}},
			"pages": []any{map[string]any{"slug": "about", "title": "About us", "body": "A small shop in **Taipei**."}, map[string]any{"slug": "shipping", "title": "Shipping", "body": "Orders ship in **1-3 working days**."}},
		}
	}
	var saved struct {
		Version int64 `json:"version"`
	}
	designCall("PUT", "/draft", map[string]any{"expected_version": 0, "document": doc("Autumn tables, slowly")}, 200, &saved)
	designCall("POST", "/publish", map[string]any{"expected_draft_version": saved.Version}, 200, nil)
	designCall("PUT", "/draft", map[string]any{"expected_version": saved.Version, "document": doc("DRAFT: winter preview")}, 200, &saved)
	var tk struct {
		Token string `json:"token"`
	}
	designCall("POST", "/preview-token", struct{}{}, 200, &tk)
	facts.PreviewToken = tk.Token
	facts.Design = map[string]any{"hero": "Autumn tables, slowly", "draft_hero": "DRAFT: winter preview", "announcement": "Free delivery over $200 · live every Thursday 8pm", "accent": "#2f6b5a",
		"name": "Morning Light Goods", "line_url": "https://line.me/R/ti/p/@morninglight", "about_title": "About us"}
	facts.StoreName = "Morning Light Goods"

	// ---- publication: merchant publish + operator bind through the 0081 definers --------------------------------------------------
	merchantToken := sfiPublishViaDefiners(t, f, f.principalA, sfrOrigin)

	// ---- control listener + the Node gate -------------------------------------------------------------------------------------------
	controlKey := randomToken()
	control := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Gate-Key") != controlKey || r.Method != http.MethodPost {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var err error
		switch r.URL.Path {
		case "/unpublish":
			err = sfiSetPublished(f, merchantToken, false)
		case "/publish":
			err = sfiSetPublished(f, merchantToken, true)
		default:
			http.NotFound(w, r)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(control.Close)
	factsFile := filepath.Join(evidence, "facts.json")
	raw, _ := json.MarshalIndent(facts, "", " ")
	if err := os.WriteFile(factsFile, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	ordersBefore := countRows(t, f.owner, `SELECT (SELECT count(*) FROM checkout.orders WHERE store_id=$1)+(SELECT count(*) FROM inventory.reservations WHERE store_id=$1)`, f.storeA1)
	ledgerBefore := countRows(t, f.owner, `SELECT count(*) FROM inventory.ledger WHERE store_id=$1`, f.storeA1)

	cmd := exec.CommandContext(ctx, "node", "tests/storefront/shop-real-gate.mjs")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	defer func() {
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
	}()
	cmd.Dir = root
	cmd.Env = browserEnvironment(map[string]string{
		"LC_MEDIA_SIZES_PHASE":       os.Getenv("LC_MEDIA_SIZES_PHASE"),
		"LC_MEDIA_SIZES_BASELINE":    os.Getenv("LC_MEDIA_SIZES_BASELINE"),
		"COMMERCE_BUYER_WEB_ENABLED": "1", "COMMERCE_BUYER_API_ORIGIN": h.server.URL, "COMMERCE_BUYER_BFF_KEY": h.key,
		"COMMERCE_BUYER_COOKIE_KEY": brToken(), "COMMERCE_BUYER_SESSION_TTL": "3600",
		"LC_SFR_EVIDENCE": evidence, "LC_SFR_FACTS": factsFile, "LC_SFR_CONTROL": control.URL, "LC_SFR_CONTROL_KEY": controlKey,
	})
	log := browserLog(t, filepath.Join(evidence, "shop-real-gate.log"))
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Run(); err != nil {
		t.Fatalf("storefront real-stack gate failed: %v; evidence=%s", err, evidence)
	}
	out, err := os.ReadFile(filepath.Join(evidence, "shop-real-gate.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "cases=9 ") {
		t.Fatalf("the real-stack gate did not report all 9 cases; evidence=%s", evidence)
	}

	// ---- independent PG readback ----------------------------------------------------------------------------------------------------
	// Browsing, carts and quotes never place an order, hold stock or write the stock ledger (I05/§11.5: only checkout and payment move stock).
	if n := countRows(t, f.owner, `SELECT (SELECT count(*) FROM checkout.orders WHERE store_id=$1)+(SELECT count(*) FROM inventory.reservations WHERE store_id=$1)`, f.storeA1); n != ordersBefore {
		t.Errorf("orders+reservations %d -> %d: the storefront walk-through must not place an order or hold stock", ordersBefore, n)
	}
	if n := countRows(t, f.owner, `SELECT count(*) FROM inventory.ledger WHERE store_id=$1`, f.storeA1); n != ledgerBefore {
		t.Errorf("inventory ledger rows %d -> %d: carts and quotes must not move stock", ledgerBefore, n)
	}
	if n := countRows(t, f.owner, `SELECT count(*) FROM control.storefront_publications WHERE tenant_id=$1 AND store_id=$2 AND published`, f.tenantA, f.storeA1); n != 1 {
		t.Errorf("the gate must leave the store published (%d)", n)
	}
	var published, bound int
	_ = f.owner.QueryRow(ctx, `SELECT (SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='merchant.storefront_published'),
		(SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='operator.domain_bound')`, f.storeA1).Scan(&published, &bound)
	if published < 2 || bound != 1 {
		t.Errorf("audit trail: merchant.storefront_published=%d (want >=2: publish, republish) operator.domain_bound=%d (want 1)", published, bound)
	}
	t.Logf("PASS: real-stack storefront shell; %d active products, %d collections; evidence=%s", facts.ActiveCount, len(facts.Collections), evidence)
}

const sfrOrigin = "https://shop.example"
