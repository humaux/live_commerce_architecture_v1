package foundation_test

// catalog_core_gate_test.go: INDEPENDENT acceptance gates CC02-CC11 of unit catalog-core (contracts/storefront-v2.md
// section A, "Acceptance" list). Written from the contract by a test author who is not the implementer; it shares only
// the repo's test harness (bhSetup/lcPrincipal), not the author's smoke (catalog_v2_smoke_test.go).
//
// Evidence label: REAL_PG (isolated disposable PostgreSQL 18, real httpapi merchant handler, real buyerhttp handler, real
// roles/RLS), synthetic published origin. CC01 (populated 0086 upgrade) is catalog_core_upgrade_test.go, CC12 (browser) is
// browser_catalog_core_test.go + tests/admin/catalog-core.spec.ts. Every product carries a unique tag in its title and every
// buyer read filters on it, because the shared fixture store holds other tests' rows.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/claims"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/inventory"
	"livecommerce/internal/live"
	"livecommerce/internal/platform"
	"livecommerce/internal/storefront"
)

// ---- merchant HTTP client (real handler, bearer token, path-scoped store) ------------------------------------------

type ccAdmin struct {
	t     *testing.T
	h     http.Handler
	token string
	store string
}

func (a ccAdmin) raw(method, path, key string, body []byte, ctype string, edit func(*http.Request)) (int, []byte, http.Header) {
	a.t.Helper()
	r := httptest.NewRequest(method, "/v1/admin/stores/"+a.store+path, bytes.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+a.token)
	if ctype != "" {
		r.Header.Set("Content-Type", ctype)
	}
	if key != "" {
		r.Header.Set("Idempotency-Key", key)
	}
	if edit != nil {
		edit(r)
	}
	w := httptest.NewRecorder()
	a.h.ServeHTTP(w, r)
	return w.Code, w.Body.Bytes(), w.Header()
}

func (a ccAdmin) call(method, path, key string, payload any) (int, []byte) {
	a.t.Helper()
	var body []byte
	ctype := ""
	if payload != nil {
		var err error
		if body, err = json.Marshal(payload); err != nil {
			a.t.Fatal(err)
		}
		ctype = "application/json"
	}
	status, out, _ := a.raw(method, path, key, body, ctype, nil)
	return status, out
}

// ok expects 200 and decodes into out (nil = ignore).
func (a ccAdmin) ok(method, path, key string, payload any, out any) {
	a.t.Helper()
	status, body := a.call(method, path, key, payload)
	if status != 200 {
		a.t.Fatalf("%s %s: status=%d want 200 body=%s", method, path, status, body)
	}
	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			a.t.Fatalf("%s %s: decode: %v body=%s", method, path, err, body)
		}
	}
}

// refuse expects exactly this refusal status.
func (a ccAdmin) refuse(want int, method, path, key string, payload any) {
	a.t.Helper()
	if status, body := a.call(method, path, key, payload); status != want {
		a.t.Fatalf("%s %s %v: status=%d want %d body=%s", method, path, payload, status, want, body)
	}
}

type ccAxis struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
}

type ccProduct struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Slug           string   `json:"slug"`
	Status         string   `json:"status"`
	SEOTitle       string   `json:"seo_title"`
	SEODescription string   `json:"seo_description"`
	Version        int64    `json:"version"`
	Options        []ccAxis `json:"options"`
}

type ccSKU struct {
	ID           string   `json:"id"`
	Code         string   `json:"code"`
	Title        string   `json:"title"`
	Status       string   `json:"status"`
	PriceMinor   int64    `json:"price_minor"`
	Compare      *int64   `json:"compare_at_minor"`
	OptionValues []string `json:"option_values"`
	Version      int64    `json:"version"`
}

type ccCollection struct {
	ID           string  `json:"id"`
	Slug         string  `json:"slug"`
	Title        string  `json:"title"`
	Description  string  `json:"description"`
	SortMode     string  `json:"sort_mode"`
	Status       string  `json:"status"`
	ImageID      *string `json:"image_id"`
	ProductCount int     `json:"product_count"`
	Version      int64   `json:"version"`
	Products     []struct {
		ProductID string `json:"product_id"`
		Position  int    `json:"position"`
	} `json:"products"`
}

// ---- buyer reads --------------------------------------------------------------------------------------------------

type ccCard struct {
	ID           string  `json:"id"`
	Slug         string  `json:"slug"`
	Title        string  `json:"title"`
	PriceMin     int64   `json:"price_min_minor"`
	PriceMax     int64   `json:"price_max_minor"`
	CompareMin   *int64  `json:"compare_at_min_minor"`
	CoverImageID *string `json:"cover_image_id"`
	InStock      bool    `json:"in_stock"`
}

type ccList struct {
	Store struct {
		Name     string `json:"name"`
		Currency string `json:"currency"`
	} `json:"store"`
	Products []ccCard `json:"products"`
	Next     *string  `json:"next"`
}

type ccVariant struct {
	SKUID        string   `json:"sku_id"`
	Title        string   `json:"title"`
	OptionValues []string `json:"option_values"`
	Price        int64    `json:"price_minor"`
	Compare      *int64   `json:"compare_at_minor"`
	Stock        string   `json:"stock"`
}

type ccDetail struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Description string `json:"description"`
	SEO         struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	} `json:"seo"`
	Images []struct {
		ID     string `json:"id"`
		Width  *int   `json:"width"`
		Height *int   `json:"height"`
	} `json:"images"`
	Options     []ccAxis    `json:"options"`
	Variants    []ccVariant `json:"variants"`
	Collections []struct {
		Slug  string `json:"slug"`
		Title string `json:"title"`
	} `json:"collections"`
}

type ccEnv struct {
	t    *testing.T
	h    bhHarness
	a    ccAdmin
	tag  string // "cc" + 12 hex, lower case: in every title, slug and code this test creates
	wh1  string
	wh2  string
	seq  atomic.Int64
	srv  http.Handler
	live *ccLive
}

type ccLive struct{ token string }

func ccNew(t *testing.T) *ccEnv {
	t.Helper()
	t04Fixture(t) // grants catalog/inventory permissions to the fixture merchant tokens once
	h := bhSetup(t)
	e := &ccEnv{t: t, h: h, tag: "cc" + strings.ToLower(t04Tag())}
	e.srv = httpapi.NewHandler(h.f.runtime)
	e.a = ccAdmin{t: t, h: e.srv, token: h.f.tokens["a"], store: h.f.storeA1}
	var w1, w2 struct {
		ID string `json:"id"`
	}
	e.a.ok("POST", "/warehouses", e.key("wh1"), map[string]any{"name": e.tag + " wh1"}, &w1)
	e.a.ok("POST", "/warehouses", e.key("wh2"), map[string]any{"name": e.tag + " wh2"}, &w2)
	e.wh1, e.wh2 = w1.ID, w2.ID
	t.Cleanup(func() {
		// The shared fixture stores are reused by later gates: leave nothing of this test buyer-visible.
		mustExec(t, h.f.owner, `UPDATE catalog.products SET status='archived' WHERE name LIKE $1`, e.tag+"%")
		mustExec(t, h.f.owner, `UPDATE catalog.collections SET status='hidden' WHERE slug LIKE $1`, e.tag+"%")
	})
	return e
}

func (e *ccEnv) key(p string) string {
	return fmt.Sprintf("%s-%s-%d", p, e.tag, e.seq.Add(1))
}
func (e *ccEnv) name(s string) string { return e.tag + " " + s }

// product creates a product through the merchant route. status "" sends no status (the contract default is draft).
func (e *ccEnv) product(title string, extra map[string]any) ccProduct {
	e.t.Helper()
	body := map[string]any{"name": e.name(title), "description": "desc of " + title}
	for k, v := range extra {
		body[k] = v
	}
	var p ccProduct
	e.a.ok("POST", "/products", e.key("p"), body, &p)
	return p
}

func (e *ccEnv) activate(p *ccProduct) {
	e.t.Helper()
	e.setStatus(p, "active")
}

func (e *ccEnv) setStatus(p *ccProduct, status string) {
	e.t.Helper()
	e.a.ok("PATCH", "/products/"+p.ID, e.key("st"), map[string]any{"expected_version": p.Version, "status": status}, p)
	if p.Status != status {
		e.t.Fatalf("status %s != %s", p.Status, status)
	}
}

func (e *ccEnv) sku(productID, code string, price int64, values []string) ccSKU {
	e.t.Helper()
	body := map[string]any{"product_id": productID, "code": strings.ToUpper(e.tag) + "-" + code, "price_minor": price}
	if values != nil {
		body["option_values"] = values
	}
	var s ccSKU
	e.a.ok("POST", "/skus", e.key("sku"), body, &s)
	return s
}

func (e *ccEnv) stock(skuID, warehouse string, delta int64) {
	e.t.Helper()
	e.a.ok("POST", "/inventory/adjustments", e.key("adj"), map[string]any{"warehouse_id": warehouse, "sku_id": skuID, "delta": delta, "expected_version": ccBalanceVersion(e, warehouse, skuID), "reason": "cc gate"}, nil)
}

// ccBalanceVersion reads the balance version (0 = no balance yet) straight from PG: the adjust route is optimistic.
func ccBalanceVersion(e *ccEnv, warehouse, sku string) int64 {
	var v int64
	if err := e.h.f.owner.QueryRow(context.Background(), `SELECT coalesce((SELECT version FROM inventory.balances WHERE warehouse_id=$1 AND sku_id=$2),0)`, warehouse, sku).Scan(&v); err != nil {
		e.t.Fatal(err)
	}
	return v
}

// simple creates an active-able product with one axis-less SKU at price (stock: units in warehouse 1).
func (e *ccEnv) simple(title string, price int64, units int64, extra map[string]any) (ccProduct, ccSKU) {
	e.t.Helper()
	p := e.product(title, extra)
	s := e.sku(p.ID, "S"+fmt.Sprint(e.seq.Load()), price, nil)
	if units > 0 {
		e.stock(s.ID, e.wh1, units)
	}
	return p, s
}

func (e *ccEnv) collection(title string, extra map[string]any, members ...string) ccCollection {
	e.t.Helper()
	body := map[string]any{"title": e.name(title)}
	for k, v := range extra {
		body[k] = v
	}
	var c ccCollection
	e.a.ok("POST", "/collections", e.key("col"), body, &c)
	if members != nil {
		e.a.ok("PUT", "/collections/"+c.ID+"/products", e.key("mem"), map[string]any{"expected_version": c.Version, "product_ids": members}, &c)
	}
	return c
}

func (e *ccEnv) buyer(path string) bhResponse {
	e.t.Helper()
	return e.h.request(e.t, "GET", path, "", "", nil, nil)
}

func (e *ccEnv) list(query string) ccList {
	e.t.Helper()
	r := e.buyer("/v1/buyer/catalog/v2/products?" + query)
	if r.status != 200 {
		e.t.Fatalf("buyer list ?%s: status=%d body=%s", query, r.status, r.body)
	}
	var l ccList
	if err := json.Unmarshal(r.body, &l); err != nil {
		e.t.Fatal(err)
	}
	return l
}

func (e *ccEnv) detail(key string) (ccDetail, int) {
	e.t.Helper()
	r := e.buyer("/v1/buyer/catalog/v2/products/" + key)
	var d ccDetail
	if r.status == 200 {
		if err := json.Unmarshal(r.body, &d); err != nil {
			e.t.Fatal(err)
		}
	}
	return d, r.status
}

func ccIDs(l ccList) []string {
	out := []string{}
	for _, c := range l.Products {
		out = append(out, c.ID)
	}
	return out
}

func ccHas(l ccList, id string) bool { return slices.Contains(ccIDs(l), id) }

// ccMedia GETs buyer media bytes (the media routes carry immutable caching, so they cannot use bhHarness.request).
func (e *ccEnv) media(kind, owner, image string) (int, []byte, http.Header) {
	e.t.Helper()
	req, _ := http.NewRequest("GET", e.h.server.URL+"/v1/buyer/media/"+kind+"/"+owner+"/"+image, nil)
	req.Header.Set("X-Commerce-Buyer-BFF-Key", e.h.key)
	req.Header.Set("X-Commerce-Storefront-Origin", e.h.origin)
	res, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	return res.StatusCode, data, res.Header
}

func ccPNG(shade uint8) []byte {
	img := image.NewRGBA(image.Rect(0, 0, 3, 2))
	img.Set(0, 0, color.RGBA{shade, 7, 9, 255})
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func ccMultipart(data []byte, filename string) ([]byte, string) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	hdr := textproto.MIMEHeader{}
	hdr.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename=%q`, filename))
	hdr.Set("Content-Type", "application/octet-stream") // the declared type is never trusted: bytes are sniffed
	part, _ := w.CreatePart(hdr)
	_, _ = part.Write(data)
	_ = w.Close()
	return buf.Bytes(), w.FormDataContentType()
}

func (e *ccEnv) upload(path string, data []byte, key string) (int, []byte) {
	e.t.Helper()
	body, ctype := ccMultipart(data, "x.bin")
	status, out, _ := e.a.raw("POST", path, key, body, ctype, nil)
	return status, out
}

// ccNormalise drops the per-request id so two 404 envelopes can be compared byte for byte.
func ccNormalise(t *testing.T, raw []byte) string {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("not json: %s", raw)
	}
	delete(m, "request_id")
	out, _ := json.Marshal(m)
	return string(out)
}

func ccKeys(raw map[string]json.RawMessage) []string {
	out := make([]string, 0, len(raw))
	for k := range raw {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// ---- CC02 draft lifecycle ------------------------------------------------------------------------------------------

// ccBuyerSession issues a buyer capability for the fixture store (the cart is a buyer, bearer-token route).
func (e *ccEnv) buyerToken() string {
	e.t.Helper()
	c := mustIssue(e.t, e.h.bcHarness.cqHarness.service, e.h.f.storeA1)
	return c.Token
}

func (e *ccEnv) cartPut(token, skuID string) int {
	e.t.Helper()
	r := e.h.request(e.t, "PUT", "/v1/buyer/cart", token, e.key("cart"), storefront.CartInput{Items: []storefront.Item{{SKUID: skuID, Quantity: 1}}}, nil)
	return r.status
}

// offer tries to open a live claim offer on the SKU through the real claims package (a draft SKU is not claimable).
func (e *ccEnv) offerErr(skuID string) error {
	e.t.Helper()
	if e.live == nil {
		_, token := lcPrincipal(e.t, e.h.f, e.h.f.tenantA, []string{e.h.f.storeA1}, "store:read", "live:read", "live:manage")
		e.live = &ccLive{token: token}
	}
	ctx := context.Background()
	d, err := t04Scoped(ctx, e.h.f, e.live.token, e.h.f.storeA1, "store:read", func(tx pgx.Tx, s platform.Scope) (live.Draft, error) {
		return live.CreateDraft(ctx, tx, s, e.live.token, e.key("draft"), live.DraftInput{Title: "cc gate " + e.tag, AspectRatio: "9:16"})
	})
	if err != nil {
		e.t.Fatalf("live draft: %v", err)
	}
	_, err = t04Scoped(ctx, e.h.f, e.live.token, e.h.f.storeA1, "store:read", func(tx pgx.Tx, s platform.Scope) (claims.Offer, error) {
		return claims.CreateOffer(ctx, tx, s, e.live.token, e.key("offer"), d.ID, claims.OfferInput{Keyword: "A1", SKUID: skuID, MaxQuantityPerClaim: 2})
	})
	return err
}

func (e *ccEnv) feedHas(skuID string) bool {
	e.t.Helper()
	var n int
	if err := e.h.f.owner.QueryRow(context.Background(), `SELECT count(*) FROM ads.feed_rows($1) f WHERE f.id=$2`, e.h.origin, skuID).Scan(&n); err != nil {
		e.t.Fatalf("feed rows: %v", err)
	}
	return n > 0
}

func (e *ccEnv) purchaseEntry(productID string) (int, string, string) {
	e.t.Helper()
	status, body := e.a.call("GET", "/products/"+productID+"/purchase-entry?locale=en", "", nil)
	var out struct {
		State string `json:"state"`
		URL   string `json:"url"`
	}
	_ = json.Unmarshal(body, &out)
	return status, out.State, out.URL
}

// surface reports where a product is buyer-reachable. Every field must be false for a draft or archived product.
type ccSurface struct {
	list, byID, bySlug, collection, collectionCount, media, legacy, cart, entry, feed, offer bool
}

func (e *ccEnv) surface(p ccProduct, sku ccSKU, img string, col ccCollection) ccSurface {
	e.t.Helper()
	var s ccSurface
	s.list = ccHas(e.list("q="+url.QueryEscape(e.tag)), p.ID)
	_, st := e.detail(p.ID)
	s.byID = st == 200
	_, st = e.detail(p.Slug)
	s.bySlug = st == 200
	s.collection = ccHas(e.list("collection="+col.Slug), p.ID)
	cols := bhRead[struct {
		Collections []struct {
			Slug         string `json:"slug"`
			ProductCount int    `json:"product_count"`
		} `json:"collections"`
	}](e.t, e.buyer("/v1/buyer/catalog/v2/collections"), 200)
	for _, c := range cols.Collections {
		if c.Slug == col.Slug && c.ProductCount > 0 {
			s.collectionCount = true
		}
	}
	status, _, _ := e.media("p", p.ID, img)
	s.media = status == 200
	legacy := bhRead[struct {
		Items []struct {
			SKUID string `json:"sku_id"`
		} `json:"items"`
	}](e.t, e.h.request(e.t, "GET", "/v1/buyer/catalog?product_id="+p.ID, e.h.cap.Token, "", nil, nil), 200)
	for _, it := range legacy.Items {
		if it.SKUID == sku.ID {
			s.legacy = true
		}
	}
	s.cart = e.cartPut(e.buyerToken(), sku.ID) == 200
	pe, state, link := e.purchaseEntry(p.ID)
	s.entry = pe == 200 && state == "configured" && link != ""
	s.feed = e.feedHas(sku.ID)
	s.offer = e.offerErr(sku.ID) == nil
	return s
}

func TestCatalogCoreCC02DraftLifecycle(t *testing.T) {
	e := ccNew(t)
	p, sku := e.simple("Draft Tee", 1200, 10, nil)
	if p.Status != "draft" || p.Version != 1 {
		t.Fatalf("a product created without status must be a draft: %+v", p)
	}
	if st, _ := e.a.call("GET", "/products/"+p.ID, "", nil); st != 200 {
		t.Fatalf("merchant must read its own draft: %d", st)
	}
	status, body := e.upload("/products/"+p.ID+"/images", ccPNG(11), e.key("img"))
	if status != 200 {
		t.Fatalf("photo upload on a draft: %d %s", status, body)
	}
	var img struct {
		ID     string `json:"id"`
		Images []struct {
			ID string `json:"id"`
		} `json:"images"`
	}
	_ = json.Unmarshal(body, &img)
	imageID := img.ID
	if imageID == "" && len(img.Images) > 0 {
		imageID = img.Images[len(img.Images)-1].ID
	}
	if imageID == "" {
		t.Fatalf("no image id in upload answer: %s", body)
	}
	col := e.collection("Drafty", nil, p.ID)

	assertNone := func(label string) {
		t.Helper()
		got := e.surface(p, sku, imageID, col)
		if got != (ccSurface{}) {
			t.Fatalf("%s product is buyer-reachable: %+v", label, got)
		}
		// 404 identical for a draft, an unknown id and an unknown slug (no probing of merchant drafts).
		unknown := e.buyer("/v1/buyer/catalog/v2/products/" + randomUUID())
		for _, key := range []string{p.ID, p.Slug} {
			r := e.buyer("/v1/buyer/catalog/v2/products/" + key)
			if r.status != 404 || ccNormalise(t, r.body) != ccNormalise(t, unknown.body) {
				t.Fatalf("%s detail by %q must be the same 404 as an unknown key: %d %s vs %s", label, key, r.status, r.body, unknown.body)
			}
		}
	}
	assertNone("draft")

	// Publish: every surface opens.
	e.activate(&p)
	got := e.surface(p, sku, imageID, col)
	want := ccSurface{list: true, byID: true, bySlug: true, collection: true, collectionCount: true, media: true, legacy: true, cart: true, entry: true, feed: true, offer: true}
	if got != want {
		t.Fatalf("active product must be reachable everywhere: got %+v want %+v", got, want)
	}
	// the detail carries the photo as {id,width,height} (contract A), sniffed from the bytes (the fixture PNG is 3x2)
	var top map[string]json.RawMessage
	_ = json.Unmarshal(e.buyer("/v1/buyer/catalog/v2/products/"+p.ID).body, &top)
	var imgs []map[string]json.RawMessage
	_ = json.Unmarshal(top["images"], &imgs)
	if len(imgs) != 1 || !reflect.DeepEqual(ccKeys(imgs[0]), []string{"height", "id", "width"}) {
		t.Fatalf("detail images must be [{id,width,height}]: %s", top["images"])
	}
	if det, _ := e.detail(p.ID); len(det.Images) != 1 || det.Images[0].ID != imageID || det.Images[0].Width == nil || *det.Images[0].Width != 3 || det.Images[0].Height == nil || *det.Images[0].Height != 2 {
		t.Fatalf("detail image id/width/height: %+v", det.Images)
	}
	// Back to draft hides it again; archived too; re-activation brings it back.
	e.setStatus(&p, "draft")
	assertNone("re-drafted")
	e.setStatus(&p, "archived")
	assertNone("archived")
	e.setStatus(&p, "active")
	if got := e.surface(p, sku, imageID, col); got != want {
		t.Fatalf("re-activated product: got %+v want %+v", got, want)
	}
	// An archived product cannot be created directly (create accepts draft|active only) and an unknown status is refused.
	e.a.refuse(422, "POST", "/products", e.key("bad"), map[string]any{"name": e.name("x"), "status": "archived"})
	e.a.refuse(422, "PATCH", "/products/"+p.ID, e.key("bad"), map[string]any{"expected_version": p.Version, "status": "deleted"})
}

// ---- CC03 slugs ---------------------------------------------------------------------------------------------------

func TestCatalogCoreCC03SlugRules(t *testing.T) {
	e := ccNew(t)
	base := e.product("Linen Shirt", nil)
	if base.Slug != e.tag+"-linen-shirt" {
		t.Fatalf("generated slug %q want %q", base.Slug, e.tag+"-linen-shirt")
	}
	second := e.product("Linen Shirt", nil)
	third := e.product("Linen Shirt", nil)
	if second.Slug != base.Slug+"-2" || third.Slug != base.Slug+"-3" {
		t.Fatalf("collisions must be suffixed -2, -3: %q %q", second.Slug, third.Slug)
	}
	// A title with no ASCII letter or digit falls back to the id prefix (no pinyin).
	zh := e.product("", map[string]any{"name": "夏季洋裝"})
	if zh.Slug == "" || !strings.HasPrefix(strings.ReplaceAll(zh.ID, "-", ""), zh.Slug) {
		t.Fatalf("no-ASCII title must fall back to the id prefix, got %q for %s", zh.Slug, zh.ID)
	}
	// Explicit slug: kept; taken = 409; invalid shapes = 422.
	own := e.product("Explicit", map[string]any{"slug": e.tag + "-own-handle"})
	if own.Slug != e.tag+"-own-handle" {
		t.Fatalf("explicit slug not kept: %q", own.Slug)
	}
	e.a.refuse(409, "POST", "/products", e.key("p"), map[string]any{"name": e.name("y"), "slug": own.Slug})
	long := strings.Repeat("a", 80)
	var atMax ccProduct
	e.a.ok("POST", "/products", e.key("p"), map[string]any{"name": e.name("max"), "slug": long[:80-len(e.tag)-1] + "-" + e.tag}, &atMax)
	for _, bad := range []string{"Bad Slug", "UPPER", "under_score", "-lead", "trail-", "dou--ble", "a/b", "", strings.Repeat("a", 81), randomUUID(), "café"} {
		if bad == "" {
			continue // an empty slug means "generate"
		}
		e.a.refuse(422, "POST", "/products", e.key("p"), map[string]any{"name": e.name("z"), "slug": bad})
	}
	e.a.refuse(422, "PATCH", "/products/"+own.ID, e.key("p"), map[string]any{"expected_version": own.Version, "slug": randomUUID()})
	e.a.refuse(409, "PATCH", "/products/"+own.ID, e.key("p"), map[string]any{"expected_version": own.Version, "slug": base.Slug})
	var renamed ccProduct
	e.a.ok("PATCH", "/products/"+own.ID, e.key("p"), map[string]any{"expected_version": own.Version, "slug": e.tag + "-renamed"}, &renamed)
	if renamed.Slug != e.tag+"-renamed" || renamed.Version != own.Version+1 {
		t.Fatalf("slug patch: %+v", renamed)
	}

	// SEO limits: 70 / 160 characters.
	var seo ccProduct
	e.a.ok("PATCH", "/products/"+base.ID, e.key("p"), map[string]any{"expected_version": base.Version, "seo_title": strings.Repeat("t", 70), "seo_description": strings.Repeat("d", 160)}, &seo)
	e.a.refuse(422, "PATCH", "/products/"+base.ID, e.key("p"), map[string]any{"expected_version": seo.Version, "seo_title": strings.Repeat("t", 71)})
	e.a.refuse(422, "PATCH", "/products/"+base.ID, e.key("p"), map[string]any{"expected_version": seo.Version, "seo_description": strings.Repeat("d", 161)})

	// Buyer: by slug and by id the same product; unknown slug 404; another store's slug is 404 here.
	s1 := e.sku(base.ID, "A", 1000, nil)
	_ = s1
	e.activate(&seo)
	byID, st1 := e.detail(base.ID)
	bySlug, st2 := e.detail(base.Slug)
	if st1 != 200 || st2 != 200 || !reflect.DeepEqual(byID, bySlug) || byID.Slug != base.Slug || byID.SEO.Title != strings.Repeat("t", 70) || byID.SEO.Description != strings.Repeat("d", 160) {
		t.Fatalf("detail by id vs slug: %d %d %+v %+v", st1, st2, byID, bySlug)
	}
	if _, st := e.detail("no-such-" + e.tag); st != 404 {
		t.Fatalf("unknown slug: %d", st)
	}
	// Uniqueness is per store: the other tenant's store may use the same slug, and its product is not served here.
	other := ccAdmin{t: t, h: e.srv, token: e.h.f.tokens["b"], store: e.h.f.storeB}
	var foreign ccProduct
	other.ok("POST", "/products", e.key("p"), map[string]any{"name": e.name("Foreign"), "slug": e.tag + "-foreign-only", "status": "active"}, &foreign)
	var twin ccProduct
	other.ok("POST", "/products", e.key("p"), map[string]any{"name": e.name("Twin"), "slug": base.Slug}, &twin)
	if twin.Slug != base.Slug {
		t.Fatalf("slug uniqueness must be per store: %+v", twin)
	}
	other.ok("POST", "/skus", e.key("sku"), map[string]any{"product_id": foreign.ID, "code": strings.ToUpper(e.tag) + "-F", "price_minor": 100}, nil)
	for _, key := range []string{foreign.Slug, foreign.ID} {
		if _, st := e.detail(key); st != 404 {
			t.Fatalf("another store's product %q must be 404 for this store: %d", key, st)
		}
	}
	if byID2, _ := e.detail(base.Slug); byID2.ID != base.ID {
		t.Fatalf("the same slug in another store must not shadow this store's product: %+v", byID2)
	}
}

// ---- CC04 options and variants ------------------------------------------------------------------------------------

func TestCatalogCoreCC04OptionsAndVariants(t *testing.T) {
	e := ccNew(t)
	p := e.product("Matrix", nil)
	patch := func(want int, axes []map[string]any) {
		t.Helper()
		e.a.refuse(want, "PATCH", "/products/"+p.ID, e.key("ax"), map[string]any{"expected_version": p.Version, "options": axes})
	}
	vals := func(n int) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = fmt.Sprintf("v%02d", i)
		}
		return out
	}
	ax := func(name string, v []string) map[string]any { return map[string]any{"name": name, "values": v} }
	// validation
	patch(422, []map[string]any{ax("a", vals(1)), ax("b", vals(1)), ax("c", vals(1)), ax("d", vals(1))}) // > 3 axes
	patch(422, []map[string]any{ax("a", []string{})})                                                    // no values
	patch(422, []map[string]any{ax("a", vals(51))})                                                      // > 50 values
	patch(422, []map[string]any{ax(strings.Repeat("n", 31), vals(1))})                                   // name > 30
	patch(422, []map[string]any{ax("a", []string{strings.Repeat("v", 41)})})                             // value > 40
	patch(422, []map[string]any{ax("", vals(1))})                                                        // empty name
	patch(422, []map[string]any{ax("a", []string{"x", "x"})})                                            // duplicate value
	patch(422, []map[string]any{ax("a", vals(1)), ax("a", vals(1))})                                     // duplicate axis name
	e.a.ok("PATCH", "/products/"+p.ID, e.key("ax"), map[string]any{"expected_version": p.Version, "options": []map[string]any{ax(strings.Repeat("n", 30), vals(50)), ax("b", []string{strings.Repeat("v", 40)})}}, &p)
	if len(p.Options) != 2 || len(p.Options[0].Values) != 50 {
		t.Fatalf("boundary axes (30-char name, 50 values, 40-char value) must be accepted: %+v", p.Options)
	}
	e.a.ok("PATCH", "/products/"+p.ID, e.key("ax"), map[string]any{"expected_version": p.Version, "options": []map[string]any{}}, &p)
	if p.Options == nil || len(p.Options) != 0 {
		t.Fatalf("clearing the axes must give an empty (non-null) list: %+v", p.Options)
	}

	// 3 axes, 2x2x2 matrix through the API
	e.a.ok("PATCH", "/products/"+p.ID, e.key("ax"), map[string]any{"expected_version": p.Version, "options": []map[string]any{
		ax("Color", []string{"Red", "Blue"}), ax("Size", []string{"S", "M"}), ax("Fit", []string{"Slim", "Loose"})}}, &p)
	if len(p.Options) != 3 {
		t.Fatalf("3 axes: %+v", p.Options)
	}
	var skus []ccSKU
	for _, c := range []string{"Red", "Blue"} {
		for _, s := range []string{"S", "M"} {
			for _, f := range []string{"Slim", "Loose"} {
				skus = append(skus, e.sku(p.ID, c+s+f, 1000, []string{c, s, f}))
			}
		}
	}
	if len(skus) != 8 || skus[0].Title != "Red / S / Slim" || !reflect.DeepEqual(skus[0].OptionValues, []string{"Red", "S", "Slim"}) {
		t.Fatalf("derived title is the values joined by ' / ': %+v", skus[0])
	}
	// alignment: arity, value not on axis, missing values
	mk := func(want int, code string, values []string) {
		t.Helper()
		body := map[string]any{"product_id": p.ID, "code": strings.ToUpper(e.tag) + "-" + code, "price_minor": 1}
		if values != nil {
			body["option_values"] = values
		}
		e.a.refuse(want, "POST", "/skus", e.key("sku"), body)
	}
	mk(422, "ARITY2", []string{"Red", "S"})
	mk(422, "ARITY4", []string{"Red", "S", "Slim", "x"})
	mk(422, "OFF", []string{"Green", "S", "Slim"})
	mk(422, "OFFPOS", []string{"S", "Red", "Slim"}) // values in the wrong axis positions
	mk(422, "NONE", nil)
	mk(409, "DUP", []string{"Red", "S", "Slim"})
	// a duplicate is re-creatable once the first variant is archived
	e.a.ok("POST", "/skus/"+skus[0].ID+"/archive", e.key("arch"), map[string]any{"expected_version": skus[0].Version}, nil)
	again := e.sku(p.ID, "DUP2", 1100, []string{"Red", "S", "Slim"})
	if again.ID == skus[0].ID || again.PriceMinor != 1100 {
		t.Fatalf("re-created variant: %+v", again)
	}
	// axes changes that would strand an active SKU are 409; compatible changes are fine
	patchExp := func(want int, axes []map[string]any) {
		t.Helper()
		e.a.refuse(want, "PATCH", "/products/"+p.ID, e.key("ax"), map[string]any{"expected_version": p.Version, "options": axes})
	}
	patchExp(409, []map[string]any{ax("Color", []string{"Red"}), ax("Size", []string{"S", "M"}), ax("Fit", []string{"Slim", "Loose"})}) // drops Blue (in use)
	patchExp(409, []map[string]any{ax("Color", []string{"Red", "Blue"}), ax("Size", []string{"S", "M"})})                               // drops an axis
	patchExp(409, []map[string]any{ax("Color", []string{"Red", "Blue"}), ax("Size", []string{"S", "M"}), ax("Fit", []string{"Tight"})}) // replaces the values an SKU uses
	e.a.ok("PATCH", "/products/"+p.ID, e.key("ax"), map[string]any{"expected_version": p.Version, "options": []map[string]any{
		ax("Color", []string{"Red", "Blue", "Green"}), ax("Size", []string{"S", "M"}), ax("Fit", []string{"Slim", "Loose"})}}, &p)
	if len(p.Options[0].Values) != 3 {
		t.Fatalf("adding a value is compatible: %+v", p.Options)
	}

	// no axes: title is "預設"; several legacy axis-less SKUs are allowed
	plain := e.product("Plain", nil)
	only := e.sku(plain.ID, "ONLY", 500, nil)
	if only.Title != "預設" || len(only.OptionValues) != 0 {
		t.Fatalf("axis-less variant title/values: %+v", only)
	}
	// axes cannot be introduced while an active axis-less SKU exists
	e.a.refuse(409, "PATCH", "/products/"+plain.ID, e.key("ax"), map[string]any{"expected_version": plain.Version, "options": []map[string]any{ax("Size", []string{"S"})}})

	// buyer: options + variants aligned, archived variant not offered
	e.activate(&p)
	d, st := e.detail(p.ID)
	if st != 200 || len(d.Options) != 3 || len(d.Variants) != 8 { // 8 active: 7 original + the re-created one
		t.Fatalf("buyer detail options/variants: %d %+v", st, d)
	}
	for _, v := range d.Variants {
		if len(v.OptionValues) != 3 || v.Title != strings.Join(v.OptionValues, " / ") {
			t.Fatalf("variant title/values: %+v", v)
		}
		for i, val := range v.OptionValues {
			if !slices.Contains(d.Options[i].Values, val) {
				t.Fatalf("variant value %q not in axis %q", val, d.Options[i].Name)
			}
		}
		if v.SKUID == skus[0].ID {
			t.Fatal("an archived variant is still offered to buyers")
		}
	}
}

// ---- CC05 compare-at ----------------------------------------------------------------------------------------------

func TestCatalogCoreCC05CompareAt(t *testing.T) {
	e := ccNew(t)
	p := e.product("Sale", nil)
	i64 := func(v int64) *int64 { return &v }
	create := func(want int, code string, price int64, cmp *int64) ccSKU {
		t.Helper()
		body := map[string]any{"product_id": p.ID, "code": strings.ToUpper(e.tag) + "-" + code, "price_minor": price}
		if cmp != nil {
			body["compare_at_minor"] = *cmp
		}
		var s ccSKU
		if want == 200 {
			e.a.ok("POST", "/skus", e.key("sku"), body, &s)
		} else {
			e.a.refuse(want, "POST", "/skus", e.key("sku"), body)
		}
		return s
	}
	create(422, "EQ", 1000, i64(1000))
	create(422, "LT", 1000, i64(999))
	create(422, "ZERO", 1000, i64(0))
	s := create(200, "OK", 1000, i64(1001))
	if s.Compare == nil || *s.Compare != 1001 {
		t.Fatalf("compare-at must be stored: %+v", s)
	}
	plain := create(200, "PLAIN", 800, nil)
	if plain.Compare != nil {
		t.Fatalf("compare-at is optional: %+v", plain)
	}
	// price route: rising past the compare-at is 422 unless compare_at_minor travels with it; null clears; absent keeps
	e.a.refuse(422, "POST", "/skus/"+s.ID+"/price", e.key("pr"), map[string]any{"price_minor": 1001, "expected_version": s.Version})
	e.a.refuse(422, "POST", "/skus/"+s.ID+"/price", e.key("pr"), map[string]any{"price_minor": 2000, "expected_version": s.Version})
	e.a.refuse(422, "POST", "/skus/"+s.ID+"/price", e.key("pr"), map[string]any{"price_minor": 500, "expected_version": s.Version, "compare_at_minor": 500})
	var up ccSKU
	e.a.ok("POST", "/skus/"+s.ID+"/price", e.key("pr"), map[string]any{"price_minor": 2000, "expected_version": s.Version, "compare_at_minor": 3000}, &up)
	if up.PriceMinor != 2000 || up.Compare == nil || *up.Compare != 3000 {
		t.Fatalf("price + compare-at together: %+v", up)
	}
	var keep ccSKU
	e.a.ok("POST", "/skus/"+s.ID+"/price", e.key("pr"), map[string]any{"price_minor": 1500, "expected_version": up.Version}, &keep)
	if keep.PriceMinor != 1500 || keep.Compare == nil || *keep.Compare != 3000 {
		t.Fatalf("absent compare_at_minor keeps it: %+v", keep)
	}
	// SKU update (full replace): compare-at must exceed the price there too
	var patched ccSKU
	full := func(price int64, cmp *int64, version int64) map[string]any {
		b := map[string]any{"product_id": p.ID, "code": keep.Code, "price_minor": price, "expected_version": version}
		if cmp != nil {
			b["compare_at_minor"] = *cmp
		}
		return b
	}
	e.a.refuse(422, "PATCH", "/skus/"+s.ID, e.key("up"), full(1500, i64(1500), keep.Version))
	e.a.ok("PATCH", "/skus/"+s.ID, e.key("up"), full(1500, i64(1600), keep.Version), &patched)
	if patched.Compare == nil || *patched.Compare != 1600 {
		t.Fatalf("patch compare-at: %+v", patched)
	}

	// buyer: shown as compare_at_min_minor (list) and compare_at_minor (variants); stock + activate first
	e.stock(s.ID, e.wh1, 3)
	e.activate(&p)
	l := e.list("q=" + url.QueryEscape(e.name("Sale")))
	if len(l.Products) != 1 || l.Products[0].CompareMin == nil || *l.Products[0].CompareMin != 1600 {
		t.Fatalf("list card compare_at_min_minor: %+v", l.Products)
	}
	d, _ := e.detail(p.ID)
	got := map[string]*int64{}
	for _, v := range d.Variants {
		got[v.SKUID] = v.Compare
	}
	if got[s.ID] == nil || *got[s.ID] != 1600 || got[plain.ID] != nil {
		t.Fatalf("variant compare_at_minor: %+v", got)
	}
	// clearing: null on the price route
	var cleared ccSKU
	e.a.ok("POST", "/skus/"+s.ID+"/price", e.key("pr"), map[string]any{"price_minor": 1500, "expected_version": patched.Version, "compare_at_minor": nil}, &cleared)
	if cleared.Compare != nil {
		t.Fatalf("null must clear compare-at: %+v", cleared)
	}
	if l := e.list("q=" + url.QueryEscape(e.name("Sale"))); l.Products[0].CompareMin != nil {
		t.Fatalf("a cleared compare-at must disappear from the card: %+v", l.Products[0])
	}

	// I05: checkout / quote use the server price only. A compare-at is set again, the buyer quotes the SKU.
	var strike ccSKU
	e.a.ok("POST", "/skus/"+s.ID+"/price", e.key("pr"), map[string]any{"price_minor": 1500, "expected_version": cleared.Version, "compare_at_minor": 2500}, &strike)
	cart := e.h.bcHarness
	cart.prepare(t, mustIssue(t, e.h.bcHarness.cqHarness.service, e.h.f.storeA1), []storefront.Item{{SKUID: s.ID, Quantity: 2}})
	if len(cart.quote.Lines) != 1 || cart.quote.Lines[0].UnitPriceMinor != 1500 || cart.quote.Lines[0].Amount.SubtotalMinor != 3000 {
		t.Fatalf("I05: quote must price the SKU at price_minor, never compare-at: %+v", cart.quote.Lines)
	}
}

// ---- CC06 buyer list ----------------------------------------------------------------------------------------------

func TestCatalogCoreCC06BuyerList(t *testing.T) {
	e := ccNew(t)
	mkActive := func(title string, price, units int64, extra map[string]any) (ccProduct, ccSKU) {
		p, s := e.simple(title, price, units, extra)
		e.activate(&p)
		return p, s
	}
	a, _ := mkActive("100% cotton tee", 3000, 5, map[string]any{"description": "plain"})
	b, _ := mkActive("100x cotton tee", 2000, 5, nil)
	c, _ := mkActive("a_b mug", 1000, 5, nil)
	d, _ := mkActive("aXb mug", 4000, 5, nil)
	f, skuF := mkActive(`back\slash cap`, 500, 5, map[string]any{"description": "has a Needle word"})
	// excluded rows: draft, archived, active without any active SKU, active whose only SKU is archived, foreign store
	draft, _ := e.simple("zz draft", 100, 5, nil)
	arch, _ := mkActive("zz archived", 100, 5, nil)
	e.setStatus(&arch, "archived")
	empty := e.product("zz nosku", map[string]any{"status": "active"})
	gone, goneSKU := e.simple("zz allarchived", 100, 5, nil)
	e.a.ok("POST", "/skus/"+goneSKU.ID+"/archive", e.key("arch"), map[string]any{"expected_version": goneSKU.Version}, nil)
	e.activate(&gone)
	other := ccAdmin{t: t, h: e.srv, token: e.h.f.tokens["b"], store: e.h.f.storeB}
	var foreign ccProduct
	other.ok("POST", "/products", e.key("p"), map[string]any{"name": e.name("zz foreign"), "status": "active"}, &foreign)
	other.ok("POST", "/skus", e.key("sku"), map[string]any{"product_id": foreign.ID, "code": strings.ToUpper(e.tag) + "-FOR", "price_minor": 100}, nil)

	q := func(s string) ccList { return e.list("q=" + url.QueryEscape(s)) }
	all := q(e.tag)
	for _, p := range []ccProduct{draft, arch, empty, gone, foreign} {
		if ccHas(all, p.ID) {
			t.Fatalf("product %s (%s) must not be in the buyer list", p.Name, p.Status)
		}
	}
	if len(all.Products) != 5 {
		t.Fatalf("the buyer list holds exactly the 5 active products with a variant: %v", ccIDs(all))
	}
	if all.Store.Currency == "" || all.Store.Name == "" || all.Next != nil {
		t.Fatalf("store summary / last page next=null: %+v", all)
	}
	// exact card shape
	var shape struct {
		Store    map[string]json.RawMessage   `json:"store"`
		Products []map[string]json.RawMessage `json:"products"`
	}
	raw := e.buyer("/v1/buyer/catalog/v2/products?q=" + url.QueryEscape(e.tag)).body
	var top map[string]json.RawMessage
	_ = json.Unmarshal(raw, &top)
	_ = json.Unmarshal(raw, &shape)
	if !reflect.DeepEqual(ccKeys(top), []string{"next", "products", "store"}) || !reflect.DeepEqual(ccKeys(shape.Store), []string{"currency", "name"}) {
		t.Fatalf("list top-level shape: %v %v", ccKeys(top), ccKeys(shape.Store))
	}
	wantCard := []string{"compare_at_min_minor", "cover_image_id", "id", "in_stock", "price_max_minor", "price_min_minor", "slug", "title"}
	if got := ccKeys(shape.Products[0]); !reflect.DeepEqual(got, wantCard) {
		t.Fatalf("card keys %v want %v", got, wantCard)
	}

	// q is a case-insensitive LITERAL: % and _ are not wildcards, backslash is not an escape
	only := func(l ccList, want ...string) {
		t.Helper()
		got := ccIDs(l)
		slices.Sort(got)
		w := slices.Clone(want)
		slices.Sort(w)
		if !slices.Equal(got, w) {
			t.Fatalf("got %v want %v", got, w)
		}
	}
	only(q(e.tag+" 100% cotton"), a.ID)              // would also match b ("100x") if % were a wildcard
	only(q(e.tag+" a_b"), c.ID)                      // would also match d ("aXb") if _ were a wildcard
	only(q(e.tag+" 100x"), b.ID)                     // plain substring
	only(q(strings.ToUpper(e.tag)+" A_B MUG"), c.ID) // case-insensitive
	only(q(`back\slash`), f.ID)                      // backslash is literal and does not break the query
	only(q(e.tag + ` \%`))                           // a lone escape + wildcard matches nothing
	only(q(e.tag + " %"))                            // no product holds "<tag> %": a wildcard would match every one
	only(q(e.tag + " _"))
	only(q("NEEDLE word"), f.ID)              // description is searched
	only(q(strings.ToLower(skuF.Code)), f.ID) // SKU code is searched, case-insensitively
	if r := e.buyer("/v1/buyer/catalog/v2/products?q=" + url.QueryEscape(strings.Repeat("x", 61))); r.status != 422 {
		t.Fatalf("q longer than 60 must be 422: %d", r.status)
	}
	if r := e.buyer("/v1/buyer/catalog/v2/products?q=" + url.QueryEscape(strings.Repeat("x", 60))); r.status != 200 {
		t.Fatalf("q of 60 is allowed: %d", r.status)
	}

	// sorts over single-variant products
	order := func(query string) []string { return ccIDs(e.list("q=" + e.tag + "&" + query)) }
	if got, want := order("sort=price_asc"), []string{f.ID, c.ID, b.ID, a.ID, d.ID}; !slices.Equal(got, want) {
		t.Fatalf("price_asc %v want %v", got, want)
	}
	if got, want := order("sort=price_desc"), []string{d.ID, a.ID, b.ID, c.ID, f.ID}; !slices.Equal(got, want) {
		t.Fatalf("price_desc %v want %v", got, want)
	}
	if got, want := order("sort=newest"), []string{f.ID, d.ID, c.ID, b.ID, a.ID}; !slices.Equal(got, want) { // created f,d,c,b,a reversed
		t.Fatalf("newest %v want %v", got, want)
	}
	if e.buyer("/v1/buyer/catalog/v2/products?sort=bogus").status != 422 {
		t.Fatal("unknown sort must be 422")
	}
	// price filter (single variants: inside [min,max] inclusive)
	only(e.list("q="+e.tag+"&min=1000&max=3000"), c.ID, b.ID, a.ID)
	only(e.list("q="+e.tag+"&min=3001"), d.ID)
	only(e.list("q=" + e.tag + "&max=499"))
	only(e.list("q="+e.tag+"&min=500&max=500"), f.ID)
	for _, bad := range []string{"min=-1", "min=abc", "max=1.5"} {
		if e.buyer("/v1/buyer/catalog/v2/products?"+bad).status != 422 {
			t.Fatalf("%s must be 422", bad)
		}
	}

	// limit, cursor
	if e.buyer("/v1/buyer/catalog/v2/products?limit=49").status != 422 || e.buyer("/v1/buyer/catalog/v2/products?limit=x").status != 422 {
		t.Fatal("limit must be at most 48")
	}
	if e.buyer("/v1/buyer/catalog/v2/products?limit=48&q="+e.tag).status != 200 {
		t.Fatal("limit 48 is allowed")
	}
	base := "q=" + e.tag + "&sort=price_asc&limit=2"
	p1 := e.list(base)
	if len(p1.Products) != 2 || p1.Next == nil {
		t.Fatalf("page 1: %+v", p1)
	}
	again := e.list(base)
	if !reflect.DeepEqual(ccIDs(p1), ccIDs(again)) {
		t.Fatalf("the same request must give the same page")
	}
	p2 := e.list(base + "&after=" + url.QueryEscape(*p1.Next))
	p2b := e.list(base + "&after=" + url.QueryEscape(*p1.Next))
	if !reflect.DeepEqual(ccIDs(p2), ccIDs(p2b)) || len(p2.Products) != 2 || p2.Next == nil {
		t.Fatalf("page 2 is stable: %+v / %+v", p2, p2b)
	}
	p3 := e.list(base + "&after=" + url.QueryEscape(*p2.Next))
	if len(p3.Products) != 1 || p3.Next != nil {
		t.Fatalf("last page: %+v", p3)
	}
	walked := append(append(ccIDs(p1), ccIDs(p2)...), ccIDs(p3)...)
	if !slices.Equal(walked, order("sort=price_asc")) {
		t.Fatalf("pages concatenate to the full ordered list: %v", walked)
	}
	// a cursor is bound to its filter: other query / other sort / other limit / garbage are 422
	for _, mut := range []string{"q=" + e.tag + "x&sort=price_asc&limit=2", "q=" + e.tag + "&sort=price_desc&limit=2", "q=" + e.tag + "&sort=price_asc&limit=3", "q=" + e.tag + "&sort=price_asc&limit=2&min=1"} {
		if r := e.buyer("/v1/buyer/catalog/v2/products?" + mut + "&after=" + url.QueryEscape(*p1.Next)); r.status != 422 {
			t.Fatalf("a cursor must not be accepted with a different filter (%s): %d", mut, r.status)
		}
	}
	for _, bad := range []string{"garbage", "AAAA", strings.Repeat("A", 400)} {
		if r := e.buyer("/v1/buyer/catalog/v2/products?" + base + "&after=" + url.QueryEscape(bad)); r.status != 422 {
			t.Fatalf("garbage cursor %q: %d", bad, r.status)
		}
	}
	// sort=title over plain lower-case words created out of order (collation-proof)
	for _, w := range []string{"kilo", "bravo", "tango"} {
		p, _ := mkActive("sorttitle-"+w, 100, 1, nil)
		_ = p
	}
	var words []string
	for _, c := range e.list("q=" + url.QueryEscape(e.name("sorttitle")) + "&sort=title").Products {
		words = append(words, strings.TrimPrefix(c.Title, e.name("sorttitle-")))
	}
	if !slices.Equal(words, []string{"bravo", "kilo", "tango"}) {
		t.Fatalf("sort=title ascending: %v", words)
	}
	// collection filter: members only; unknown or hidden collection is 404
	col := e.collection("List", nil, d.ID, a.ID)
	only(e.list("collection="+col.Slug), d.ID, a.ID)
	if r := e.buyer("/v1/buyer/catalog/v2/products?collection=nope-" + e.tag); r.status != 404 {
		t.Fatalf("unknown collection: %d", r.status)
	}
}

// ---- CC07 stock hints ---------------------------------------------------------------------------------------------

func TestCatalogCoreCC07StockHints(t *testing.T) {
	e := ccNew(t)
	p := e.product("Stock", nil)
	e.a.ok("PATCH", "/products/"+p.ID, e.key("ax"), map[string]any{"expected_version": p.Version, "options": []map[string]any{
		{"name": "Case", "values": []string{"zero", "one", "five", "six", "split6", "split5", "held", "drained"}}}}, &p)
	by := map[string]ccSKU{}
	for _, c := range []string{"zero", "one", "five", "six", "split6", "split5", "held", "drained"} {
		by[c] = e.sku(p.ID, c, 1000, []string{c})
	}
	e.stock(by["one"].ID, e.wh1, 1)
	e.stock(by["five"].ID, e.wh1, 5)
	e.stock(by["six"].ID, e.wh1, 6)
	e.stock(by["split6"].ID, e.wh1, 3) // 3 + 3 over two warehouses = 6 -> in (not the larger warehouse, not the first)
	e.stock(by["split6"].ID, e.wh2, 3)
	e.stock(by["split5"].ID, e.wh1, 2) // 2 + 3 = 5 -> low
	e.stock(by["split5"].ID, e.wh2, 3)
	e.stock(by["held"].ID, e.wh1, 4)
	e.stock(by["held"].ID, e.wh2, 3) // 7 -> in; reservations then move it
	e.stock(by["drained"].ID, e.wh1, 8)
	e.stock(by["drained"].ID, e.wh1, -8) // back to zero after movements: out
	e.activate(&p)
	hints := func() map[string]string {
		t.Helper()
		d, st := e.detail(p.ID)
		if st != 200 || len(d.Variants) != 8 {
			t.Fatalf("detail: %d %+v", st, d)
		}
		out := map[string]string{}
		for _, v := range d.Variants {
			out[v.OptionValues[0]] = v.Stock
		}
		return out
	}
	want := map[string]string{"zero": "out", "one": "low", "five": "low", "six": "in", "split6": "in", "split5": "low", "held": "in", "drained": "out"}
	if got := hints(); !reflect.DeepEqual(got, want) {
		t.Fatalf("stock hints (in>=6, low 1..5, out 0, summed over warehouses): got %v want %v", got, want)
	}
	// reservations lower the hint: on_hand 7 over two warehouses, reserve 1 -> 6 in, reserve 1 more -> 5 low, reserve 5 more -> 0 out
	reserve := func(q int64, wh string) {
		t.Helper()
		_, err := t04Scoped(context.Background(), e.h.f, e.h.f.tokens["a"], e.h.f.storeA1, "inventory:reserve", func(tx pgx.Tx, s platform.Scope) (inventory.Reservation, error) {
			return inventory.Reserve(context.Background(), tx, s, e.key("res"), []inventory.Line{{WarehouseID: wh, SKUID: by["held"].ID, Quantity: q}})
		})
		if err != nil {
			t.Fatalf("reserve %d: %v", q, err)
		}
	}
	reserve(1, e.wh1)
	if got := hints()["held"]; got != "in" {
		t.Fatalf("7-1=6 must still be in, got %s", got)
	}
	reserve(1, e.wh2)
	if got := hints()["held"]; got != "low" {
		t.Fatalf("7-2=5 must be low, got %s", got)
	}
	reserve(3, e.wh1)
	reserve(2, e.wh2)
	if got := hints()["held"]; got != "out" {
		t.Fatalf("everything reserved must be out, got %s", got)
	}
	// in_stock on the card: true while any variant has stock, false when none has
	if l := e.list("q=" + url.QueryEscape(e.name("Stock"))); len(l.Products) != 1 || !l.Products[0].InStock {
		t.Fatalf("card in_stock with stocked variants: %+v", l.Products)
	}
	sold, soldSKU := e.simple("Soldout", 700, 0, nil)
	_ = soldSKU
	e.activate(&sold)
	if l := e.list("q=" + url.QueryEscape(e.name("Soldout"))); len(l.Products) != 1 || l.Products[0].InStock {
		t.Fatalf("a product with no stock is listed with in_stock=false: %+v", l.Products)
	}
	// no raw count anywhere in a buyer payload
	for _, path := range []string{"/v1/buyer/catalog/v2/products/" + p.ID, "/v1/buyer/catalog/v2/products?q=" + e.tag} {
		body := string(e.buyer(path).body)
		for _, banned := range []string{"on_hand", "available", "reserved", "quantity", "warehouse", "allocated", "unavailable"} {
			if strings.Contains(body, banned) {
				t.Fatalf("buyer payload leaks %q: %s", banned, body)
			}
		}
	}
	// exact detail shape of the contract
	var top map[string]json.RawMessage
	_ = json.Unmarshal(e.buyer("/v1/buyer/catalog/v2/products/"+p.ID).body, &top)
	if got, w := ccKeys(top), []string{"collections", "description", "id", "images", "options", "seo", "slug", "title", "variants"}; !reflect.DeepEqual(got, w) {
		t.Fatalf("detail keys %v want %v", got, w)
	}
	var variants []map[string]json.RawMessage
	_ = json.Unmarshal(top["variants"], &variants)
	if got, w := ccKeys(variants[0]), []string{"compare_at_minor", "option_values", "price_minor", "sku_id", "stock", "title"}; !reflect.DeepEqual(got, w) {
		t.Fatalf("variant keys %v want %v", got, w)
	}
}

// ---- CC08 collections ---------------------------------------------------------------------------------------------

func TestCatalogCoreCC08Collections(t *testing.T) {
	e := ccNew(t)
	mk := func(title string, price int64) ccProduct {
		p, _ := e.simple(title, price, 5, nil)
		e.activate(&p)
		return p
	}
	pa, pb, pc := mk("A first", 3000), mk("B second", 1000), mk("C third", 2000)
	draft, _ := e.simple("D draft", 100, 5, nil)
	arch := mk("E archived", 100)
	e.setStatus(&arch, "archived")

	c := e.collection("Summer", map[string]any{"description": "plain <b>text</b>"})
	if c.Slug != e.tag+"-summer" || c.SortMode != "manual" || c.Status != "active" || c.Version != 1 || c.ProductCount != 0 || c.Description != "plain <b>text</b>" {
		t.Fatalf("new collection defaults: %+v", c)
	}
	e.a.refuse(409, "POST", "/collections", e.key("c"), map[string]any{"title": "x", "slug": c.Slug})
	for _, bad := range []map[string]any{{"title": ""}, {"title": strings.Repeat("t", 81)}, {"title": "x", "sort_mode": "title"}, {"title": "x", "status": "deleted"}, {"title": "x", "slug": "Bad Slug"}, {"title": "x", "slug": randomUUID()}, {"title": "x", "description": strings.Repeat("d", 2001)}} {
		e.a.refuse(422, "POST", "/collections", e.key("c"), bad)
	}
	var edge ccCollection
	e.a.ok("POST", "/collections", e.key("c"), map[string]any{"title": strings.Repeat("t", 80), "description": strings.Repeat("d", 2000), "slug": e.tag + "-edge"}, &edge)

	// optimistic versions
	var patched ccCollection
	e.a.ok("PATCH", "/collections/"+c.ID, e.key("c"), map[string]any{"expected_version": c.Version, "title": e.name("Summer 2")}, &patched)
	if patched.Version != 2 || patched.Title != e.name("Summer 2") || patched.Slug != c.Slug {
		t.Fatalf("patch: %+v", patched)
	}
	c = patched
	e.a.refuse(409, "PATCH", "/collections/"+c.ID, e.key("c"), map[string]any{"expected_version": 1, "title": "stale"})
	e.a.refuse(422, "PATCH", "/collections/"+c.ID, e.key("c"), map[string]any{"expected_version": c.Version})
	e.a.refuse(409, "PATCH", "/collections/"+c.ID, e.key("c"), map[string]any{"expected_version": c.Version, "slug": edge.Slug})

	// membership: complete ordered list; add, reorder, remove
	put := func(ids ...string) ccCollection {
		t.Helper()
		var out ccCollection
		e.a.ok("PUT", "/collections/"+c.ID+"/products", e.key("m"), map[string]any{"expected_version": c.Version, "product_ids": ids}, &out)
		c = out
		return out
	}
	got := put(pa.ID, pb.ID, pc.ID, draft.ID, arch.ID)
	if len(got.Products) != 5 || got.Products[0].ProductID != pa.ID || got.Products[2].ProductID != pc.ID {
		t.Fatalf("membership: %+v", got)
	}
	positions := []int{}
	for _, m := range got.Products {
		positions = append(positions, m.Position)
	}
	if !slices.IsSorted(positions) || positions[0] != 0 && positions[0] != 1 {
		t.Fatalf("positions must be ordered: %v", positions)
	}
	e.a.refuse(409, "PUT", "/collections/"+c.ID+"/products", e.key("m"), map[string]any{"expected_version": c.Version - 1, "product_ids": []string{}})
	e.a.refuse(422, "PUT", "/collections/"+c.ID+"/products", e.key("m"), map[string]any{"expected_version": c.Version, "product_ids": []string{pa.ID, pa.ID}})
	e.a.refuse(404, "PUT", "/collections/"+c.ID+"/products", e.key("m"), map[string]any{"expected_version": c.Version, "product_ids": []string{pa.ID, randomUUID()}})
	// a foreign tenant's product id is as unknown as a random one
	other := ccAdmin{t: t, h: e.srv, token: e.h.f.tokens["b"], store: e.h.f.storeB}
	var foreign ccProduct
	other.ok("POST", "/products", e.key("p"), map[string]any{"name": e.name("foreign"), "status": "active"}, &foreign)
	e.a.refuse(404, "PUT", "/collections/"+c.ID+"/products", e.key("m"), map[string]any{"expected_version": c.Version, "product_ids": []string{foreign.ID}})
	if n := len(put(pa.ID, pb.ID, pc.ID, draft.ID, arch.ID).Products); n != 5 {
		t.Fatalf("a refused PUT must leave membership unchanged: %d", n)
	}

	// manual order, then sort modes; an explicit sort overrides the collection's mode
	ids := func(q string) []string { return ccIDs(e.list(q)) }
	if g, w := ids("collection="+c.Slug), []string{pa.ID, pb.ID, pc.ID}; !slices.Equal(g, w) {
		t.Fatalf("manual order (draft and archived members hidden) %v want %v", g, w)
	}
	put(pc.ID, pa.ID, pb.ID, draft.ID)
	if g, w := ids("collection="+c.Slug), []string{pc.ID, pa.ID, pb.ID}; !slices.Equal(g, w) {
		t.Fatalf("reorder %v want %v", g, w)
	}
	if g, w := ids("collection="+c.Slug+"&sort=price_asc"), []string{pb.ID, pc.ID, pa.ID}; !slices.Equal(g, w) {
		t.Fatalf("explicit sort overrides manual: %v want %v", g, w)
	}
	setMode := func(mode string) {
		t.Helper()
		e.a.ok("PATCH", "/collections/"+c.ID, e.key("c"), map[string]any{"expected_version": c.Version, "sort_mode": mode}, &c)
	}
	setMode("price_asc")
	if g, w := ids("collection="+c.Slug), []string{pb.ID, pc.ID, pa.ID}; !slices.Equal(g, w) {
		t.Fatalf("sort_mode price_asc: %v want %v", g, w)
	}
	setMode("price_desc")
	if g, w := ids("collection="+c.Slug), []string{pa.ID, pc.ID, pb.ID}; !slices.Equal(g, w) {
		t.Fatalf("sort_mode price_desc: %v want %v", g, w)
	}
	setMode("newest")
	if g, w := ids("collection="+c.Slug), []string{pc.ID, pb.ID, pa.ID}; !slices.Equal(g, w) {
		t.Fatalf("sort_mode newest: %v want %v", g, w)
	}
	setMode("manual")
	// removal
	put(pa.ID, pc.ID)
	if g, w := ids("collection="+c.Slug), []string{pa.ID, pc.ID}; !slices.Equal(g, w) {
		t.Fatalf("remove b: %v want %v", g, w)
	}
	put(pc.ID, pb.ID, pa.ID, draft.ID, arch.ID)

	// a product may be in many collections
	second := e.collection("Second", nil, pa.ID)
	d, _ := e.detail(pa.ID)
	slugs := []string{}
	for _, cc := range d.Collections {
		slugs = append(slugs, cc.Slug)
	}
	slices.Sort(slugs)
	wantSlugs := []string{c.Slug, second.Slug}
	slices.Sort(wantSlugs)
	if !slices.Equal(slugs, wantSlugs) {
		t.Fatalf("product detail lists its active collections: %v want %v", slugs, wantSlugs)
	}

	// buyer collection reads: counts are of ACTIVE products only; exact shapes
	type card struct {
		Slug         string  `json:"slug"`
		Title        string  `json:"title"`
		ImageID      *string `json:"image_id"`
		ProductCount int     `json:"product_count"`
	}
	list := func() map[string]card {
		t.Helper()
		var out struct {
			Collections []card `json:"collections"`
		}
		r := e.buyer("/v1/buyer/catalog/v2/collections")
		if r.status != 200 || json.Unmarshal(r.body, &out) != nil {
			t.Fatalf("collections: %d %s", r.status, r.body)
		}
		m := map[string]card{}
		for _, x := range out.Collections {
			m[x.Slug] = x
		}
		return m
	}
	if got := list()[c.Slug]; got.ProductCount != 3 || got.Title != c.Title {
		t.Fatalf("product_count counts active products only (3 of 5 members): %+v", got)
	}
	var shape map[string]json.RawMessage
	// storefront-v2 §A amendment (unit storefront-integration, 0093): collection list and detail also carry `id`.
	_ = json.Unmarshal(e.buyer("/v1/buyer/catalog/v2/collections/"+c.Slug).body, &shape)
	if g, w := ccKeys(shape), []string{"description", "id", "image_id", "slug", "title"}; !reflect.DeepEqual(g, w) {
		t.Fatalf("collection detail keys %v want %v", g, w)
	}
	var cards struct {
		Collections []map[string]json.RawMessage `json:"collections"`
	}
	_ = json.Unmarshal(e.buyer("/v1/buyer/catalog/v2/collections").body, &cards)
	if len(cards.Collections) == 0 || !reflect.DeepEqual(ccKeys(cards.Collections[0]), []string{"id", "image_id", "product_count", "slug", "title"}) {
		t.Fatalf("collection card keys: %+v", cards)
	}

	// hidden collection: absent from the list, detail, filter and image
	e.a.ok("PATCH", "/collections/"+c.ID, e.key("c"), map[string]any{"expected_version": c.Version, "status": "hidden"}, &c)
	if _, there := list()[c.Slug]; there {
		t.Fatal("hidden collection in the buyer list")
	}
	if r := e.buyer("/v1/buyer/catalog/v2/collections/" + c.Slug); r.status != 404 {
		t.Fatalf("hidden collection detail: %d", r.status)
	}
	if r := e.buyer("/v1/buyer/catalog/v2/products?collection=" + c.Slug); r.status != 404 {
		t.Fatalf("hidden collection filter: %d", r.status)
	}
	if d, _ := e.detail(pc.ID); len(d.Collections) != 0 {
		t.Fatalf("a hidden collection is not listed on its products: %+v", d.Collections)
	}
	e.a.ok("PATCH", "/collections/"+c.ID, e.key("c"), map[string]any{"expected_version": c.Version, "status": "active"}, &c)
	if _, there := list()[c.Slug]; !there {
		t.Fatal("re-activated collection is back")
	}

	// the 500-product ceiling: 500 members are fine, 501 is 422 (products are seeded as drafts in bulk)
	big := e.collection("Big", nil)
	var seeded []string
	rows, err := e.h.f.owner.Query(context.Background(), `INSERT INTO catalog.products(tenant_id,store_id,id,name,description,status)
		SELECT $1,$2,gen_random_uuid(),$3||' bulk '||g,'', 'draft' FROM generate_series(1,501) g RETURNING id::text`, e.h.f.tenantA, e.h.f.storeA1, e.tag)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id string
		_ = rows.Scan(&id)
		seeded = append(seeded, id)
	}
	rows.Close()
	if len(seeded) != 501 {
		t.Fatalf("seeded %d", len(seeded))
	}
	e.a.refuse(422, "PUT", "/collections/"+big.ID+"/products", e.key("m"), map[string]any{"expected_version": big.Version, "product_ids": seeded})
	e.a.ok("PUT", "/collections/"+big.ID+"/products", e.key("m"), map[string]any{"expected_version": big.Version, "product_ids": seeded[:500]}, &big)
	if len(big.Products) != 500 {
		t.Fatalf("500 members: %d", len(big.Products))
	}

	// delete keeps the products; the buyer sees 404 afterwards; stale version is 409
	e.a.refuse(409, "POST", "/collections/"+second.ID+"/delete", e.key("d"), map[string]any{"expected_version": second.Version + 5})
	e.a.ok("POST", "/collections/"+second.ID+"/delete", e.key("d"), map[string]any{"expected_version": second.Version}, nil)
	e.a.refuse(404, "GET", "/collections/"+second.ID, "", nil)
	if r := e.buyer("/v1/buyer/catalog/v2/collections/" + second.Slug); r.status != 404 {
		t.Fatalf("deleted collection detail: %d", r.status)
	}
	if !ccHas(e.list("q="+url.QueryEscape(e.name("A first"))), pa.ID) {
		t.Fatal("deleting a collection must keep its products")
	}
	if n := countRows(t, e.h.f.owner, `SELECT count(*) FROM catalog.products WHERE id=$1 AND status='active'`, pa.ID); n != 1 {
		t.Fatal("product row changed by the collection delete")
	}
	// admin list shows the collection with its count
	var adminList struct {
		Items []ccCollection `json:"items"`
	}
	e.a.ok("GET", "/collections", "", nil, &adminList)
	found := false
	for _, it := range adminList.Items {
		if it.ID == c.ID {
			found = true
		}
		if it.ID == second.ID {
			t.Fatal("deleted collection in the merchant list")
		}
	}
	if !found {
		t.Fatalf("merchant collection list lacks %s", c.ID)
	}
}

// ---- CC09 collection image ----------------------------------------------------------------------------------------

func TestCatalogCoreCC09CollectionImage(t *testing.T) {
	e := ccNew(t)
	c := e.collection("Pictured", nil)
	pngA := ccPNG(20)
	st, body := e.upload("/collections/"+c.ID+"/image", pngA, e.key("img"))
	if st != 200 {
		t.Fatalf("png upload: %d %s", st, body)
	}
	var img struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(body, &img)
	if img.ID == "" {
		t.Fatalf("no image id: %s", body)
	}
	// bytes, sniffed type, immutable cache, nosniff
	status, data, header := e.media("c", c.ID, img.ID)
	if status != 200 || !bytes.Equal(data, pngA) || header.Get("Content-Type") != "image/png" || header.Get("X-Content-Type-Options") != "nosniff" ||
		!strings.Contains(header.Get("Cache-Control"), "public") || !strings.Contains(header.Get("Cache-Control"), "immutable") || !strings.Contains(header.Get("Cache-Control"), "max-age=") {
		t.Fatalf("collection media: %d type=%q cache=%q nosniff=%q", status, header.Get("Content-Type"), header.Get("Cache-Control"), header.Get("X-Content-Type-Options"))
	}
	// the merchant reads it back; the collection reports it
	rs, rdata, _ := e.a.raw("GET", "/collections/"+c.ID+"/image", "", nil, "", nil)
	if rs != 200 || !bytes.Equal(rdata, pngA) {
		t.Fatalf("merchant image read: %d", rs)
	}
	var got ccCollection
	e.a.ok("GET", "/collections/"+c.ID, "", nil, &got)
	if got.ImageID == nil || *got.ImageID != img.ID {
		t.Fatalf("collection image_id: %+v", got.ImageID)
	}
	// a JPEG is accepted too (sniffed)
	var jb bytes.Buffer
	_ = jpeg.Encode(&jb, image.NewRGBA(image.Rect(0, 0, 4, 4)), nil)
	if st, body := e.upload("/collections/"+c.ID+"/image", jb.Bytes(), e.key("img")); st != 200 {
		t.Fatalf("jpeg upload: %d %s", st, body)
	} else {
		_ = json.Unmarshal(body, &img)
	}
	if status, _, h := e.media("c", c.ID, img.ID); status != 200 || h.Get("Content-Type") != "image/jpeg" {
		t.Fatalf("jpeg media: %d %q", status, h.Get("Content-Type"))
	}
	// refused: SVG, GIF, text with an image name, empty, over 2 MiB
	var gb bytes.Buffer
	_ = gif.Encode(&gb, image.NewPaletted(image.Rect(0, 0, 2, 2), color.Palette{color.Black, color.White}), nil)
	for name, data := range map[string][]byte{
		"svg":   []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="1" height="1"/>`),
		"gif":   gb.Bytes(),
		"text":  []byte("GIF89a not really, plain text"),
		"empty": {},
	} {
		if st, body := e.upload("/collections/"+c.ID+"/image", data, e.key("img")); st != 422 && !(name == "empty" && st == 400) {
			t.Fatalf("%s must be refused 422, got %d %s", name, st, body)
		}
	}
	over := append(append([]byte{}, ccPNG(5)...), make([]byte, 2*1024*1024)...) // a valid PNG followed by padding: > 2 MiB
	if st, _ := e.upload("/collections/"+c.ID+"/image", over, e.key("img")); st != 413 {
		t.Fatalf("over 2 MiB must be 413, got %d", st)
	}
	exact := append(append([]byte{}, ccPNG(6)...), make([]byte, 2*1024*1024-len(ccPNG(6)))...)
	if st, body := e.upload("/collections/"+c.ID+"/image", exact, e.key("img")); st != 200 {
		t.Fatalf("exactly 2 MiB is allowed: %d %s", st, body)
	} else {
		_ = json.Unmarshal(body, &img)
	}
	// replace: new id each time; the old id is gone (immutable URLs never serve replaced bytes)
	old := img.ID
	st, body = e.upload("/collections/"+c.ID+"/image", ccPNG(30), e.key("img"))
	if st != 200 {
		t.Fatalf("replace: %d", st)
	}
	_ = json.Unmarshal(body, &img)
	if img.ID == old {
		t.Fatal("replacing the image must change its immutable id")
	}
	if status, _, _ := e.media("c", c.ID, old); status != 404 {
		t.Fatalf("a replaced image id is still served: %d", status)
	}
	if status, _, _ := e.media("c", c.ID, img.ID); status != 200 {
		t.Fatalf("the new image is served: %d", status)
	}
	// 404: unknown image, other collection's id pair, product-style path, hidden collection, other tenant's collection, unpublished store
	other := e.collection("Other", nil)
	if st, _ := e.upload("/collections/"+other.ID+"/image", ccPNG(40), e.key("img")); st != 200 {
		t.Fatal("second collection upload")
	}
	if status, _, _ := e.media("c", c.ID, randomUUID()); status != 404 {
		t.Fatalf("unknown image id: %d", status)
	}
	if status, _, _ := e.media("c", other.ID, img.ID); status != 404 {
		t.Fatalf("image id under another collection: %d", status)
	}
	if status, _, _ := e.media("c", randomUUID(), img.ID); status != 404 {
		t.Fatalf("unknown collection: %d", status)
	}
	if status, _, _ := e.media("p", c.ID, img.ID); status != 404 {
		t.Fatalf("a collection image must not be served on the product media path: %d", status)
	}
	foreign := ccAdmin{t: t, h: e.srv, token: e.h.f.tokens["b"], store: e.h.f.storeB}
	var fc ccCollection
	foreign.ok("POST", "/collections", e.key("col"), map[string]any{"title": e.name("Foreign")}, &fc)
	body2, ctype := ccMultipart(ccPNG(50), "f.png")
	if st, out, _ := foreign.raw("POST", "/collections/"+fc.ID+"/image", e.key("img"), body2, ctype, nil); st != 200 {
		t.Fatalf("foreign upload: %d %s", st, out)
	} else {
		var fi struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(out, &fi)
		if status, _, _ := e.media("c", fc.ID, fi.ID); status != 404 {
			t.Fatalf("another tenant's collection image is served on this store's origin: %d", status)
		}
	}
	// the merchant cannot touch it either
	e.a.refuse(404, "GET", "/collections/"+fc.ID+"/image", "", nil)
	if st, _ := e.upload("/collections/"+fc.ID+"/image", ccPNG(51), e.key("img")); st != 404 {
		t.Fatalf("uploading to another store's collection: %d", st)
	}
	var cur ccCollection
	e.a.ok("GET", "/collections/"+c.ID, "", nil, &cur)
	e.a.ok("PATCH", "/collections/"+c.ID, e.key("c"), map[string]any{"expected_version": cur.Version, "status": "hidden"}, &cur)
	if status, _, _ := e.media("c", c.ID, img.ID); status != 404 {
		t.Fatalf("hidden collection image: %d", status)
	}
	e.a.ok("PATCH", "/collections/"+c.ID, e.key("c"), map[string]any{"expected_version": cur.Version, "status": "active"}, &cur)
	// delete the image
	e.a.ok("POST", "/collections/"+c.ID+"/image/delete", e.key("imgdel"), map[string]any{}, nil)
	if status, _, _ := e.media("c", c.ID, img.ID); status != 404 {
		t.Fatalf("a deleted image is still served: %d", status)
	}
	e.a.ok("GET", "/collections/"+c.ID, "", nil, &cur)
	if cur.ImageID != nil {
		t.Fatalf("image_id after delete: %v", *cur.ImageID)
	}
	// unpublished store: every media read is 404 (publication removed by the owner pool, restored by cleanup of bhPublish)
	st, body = e.upload("/collections/"+c.ID+"/image", ccPNG(60), e.key("img"))
	_ = json.Unmarshal(body, &img)
	mustExec(t, e.h.f.owner, `UPDATE control.storefront_publications SET published=false WHERE tenant_id=$1 AND store_id=$2`, e.h.f.tenantA, e.h.f.storeA1)
	if status, _, _ := e.media("c", c.ID, img.ID); status != 404 {
		t.Fatalf("unpublished store media: %d", status)
	}
}

// ---- CC10 isolation and authority ---------------------------------------------------------------------------------

func TestCatalogCoreCC10IsolationAndAuthority(t *testing.T) {
	e := ccNew(t)
	p, sku := e.simple("Owned", 1000, 5, nil)
	e.activate(&p)
	col := e.collection("Owned", nil, p.ID)

	// ---- tenant / store isolation of every merchant row ----
	foreign := ccAdmin{t: t, h: e.srv, token: e.h.f.tokens["b"], store: e.h.f.storeB}
	foreign.refuse(404, "GET", "/products/"+p.ID, "", nil)
	foreign.refuse(404, "PATCH", "/products/"+p.ID, e.key("x"), map[string]any{"expected_version": p.Version, "status": "archived"})
	foreign.refuse(404, "POST", "/skus/"+sku.ID+"/price", e.key("x"), map[string]any{"price_minor": 1, "expected_version": sku.Version})
	foreign.refuse(404, "PATCH", "/skus/"+sku.ID, e.key("x"), map[string]any{"product_id": p.ID, "code": "X1X1X1", "price_minor": 5, "expected_version": sku.Version})
	foreign.refuse(404, "POST", "/skus/"+sku.ID+"/archive", e.key("x"), map[string]any{"expected_version": sku.Version})
	foreign.refuse(404, "GET", "/collections/"+col.ID, "", nil)
	foreign.refuse(404, "PATCH", "/collections/"+col.ID, e.key("x"), map[string]any{"expected_version": col.Version, "status": "hidden"})
	foreign.refuse(404, "PUT", "/collections/"+col.ID+"/products", e.key("x"), map[string]any{"expected_version": col.Version, "product_ids": []string{}})
	foreign.refuse(404, "POST", "/collections/"+col.ID+"/delete", e.key("x"), map[string]any{"expected_version": col.Version})
	if st, _ := foreign.upload(e, "/collections/"+col.ID+"/image"); st != 404 {
		t.Fatalf("foreign collection image upload: %d", st)
	}
	foreign.refuse(404, "POST", "/skus", e.key("x"), map[string]any{"product_id": p.ID, "code": "FORE1GN", "price_minor": 1})
	var list struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	foreign.ok("GET", "/catalog-products?q="+e.tag, "", nil, &list)
	if len(list.Items) != 0 {
		t.Fatalf("the other tenant's product list contains this tenant's rows: %+v", list)
	}
	foreign.ok("GET", "/collections", "", nil, &struct{}{})
	// a token of tenant B against store A's path: the path store is not theirs
	cross := ccAdmin{t: t, h: e.srv, token: e.h.f.tokens["b"], store: e.h.f.storeA1}
	for _, path := range []string{"/catalog-products", "/products/" + p.ID, "/collections", "/collections/" + col.ID} {
		if st, body := cross.call("GET", path, "", nil); st == 200 || strings.Contains(string(body), p.ID) {
			t.Fatalf("a foreign tenant read store A %s: %d %s", path, st, body)
		}
	}
	// the data itself is untouched
	var now ccProduct
	e.a.ok("GET", "/products/"+p.ID, "", nil, &now)
	if now.Status != "active" || now.Version != p.Version {
		t.Fatalf("a foreign write changed the product: %+v", now)
	}
	// scope comes from the path and the token only: X-Tenant-ID / Host / forwarded headers never select it
	for _, hdr := range []map[string]string{{"X-Tenant-ID": e.h.f.tenantB}, {"X-Store-ID": e.h.f.storeB}, {"X-Forwarded-Host": "store-b.example"}, {"Host": "store-b.example"}, {"X-Forwarded-For": "203.0.113.9"}} {
		edit := func(r *http.Request) {
			for k, v := range hdr {
				if k == "Host" {
					r.Host = v
				} else {
					r.Header.Set(k, v)
				}
			}
		}
		if st, body, _ := e.a.raw("GET", "/products/"+p.ID, "", nil, "", edit); st != 200 || !strings.Contains(string(body), p.ID) {
			t.Fatalf("a spoofed %v header changed the scope of a merchant read: %d %s", hdr, st, body)
		}
		if st, body, _ := foreign.raw("GET", "/products/"+p.ID, "", nil, "", func(r *http.Request) { edit(r); r.Header.Set("X-Tenant-ID", e.h.f.tenantA) }); st == 200 {
			t.Fatalf("a spoofed tenant header gave a foreign merchant access to store A rows: %s", body)
		}
	}
	// buyer: the same product is never served for another origin; spoofed headers do not select a store
	for _, hdr := range []map[string]string{{"X-Tenant-ID": e.h.f.tenantB}, {"X-Forwarded-Host": "store-b.example"}, {"X-Store-ID": e.h.f.storeB}} {
		r := e.h.request(t, "GET", "/v1/buyer/catalog/v2/products/"+p.ID, "", "", nil, func(r *http.Request) {
			for k, v := range hdr {
				r.Header.Set(k, v)
			}
		})
		// Refusing the header (403) or ignoring it (200, this store's product) are both fine; honouring it is not.
		if r.status != 403 && !(r.status == 200 && strings.Contains(string(r.body), p.ID)) {
			t.Fatalf("buyer read with a spoofed %v header must ignore or refuse it: %d %s", hdr, r.status, r.body)
		}

	}
	if r := e.h.request(t, "GET", "/v1/buyer/catalog/v2/products/"+p.ID, "", "", nil, func(r *http.Request) { r.Header.Set("X-Commerce-Storefront-Origin", "https://unknown.example") }); r.status != 404 {
		t.Fatalf("an origin that is not published must be 404: %d", r.status)
	}

	// ---- buyer routes refuse Authorization and Cookie ----
	for _, path := range []string{"/v1/buyer/catalog/v2/products", "/v1/buyer/catalog/v2/products/" + p.ID, "/v1/buyer/catalog/v2/collections", "/v1/buyer/catalog/v2/collections/" + col.Slug} {
		for name, edit := range map[string]func(*http.Request){
			"Authorization": func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+e.h.f.tokens["a"]) },
			"Cookie":        func(r *http.Request) { r.Header.Set("Cookie", "session=abc") },
		} {
			if r := e.h.request(t, "GET", path, "", "", nil, edit); r.status != 403 {
				t.Fatalf("%s on %s must be 403: %d", name, path, r.status)
			}
		}
	}

	// ---- per-route permissions ----
	mkToken := func(perms ...string) ccAdmin {
		_, token := lcPrincipal(t, e.h.f, e.h.f.tenantA, []string{e.h.f.storeA1}, perms...)
		return ccAdmin{t: t, h: e.srv, token: token, store: e.h.f.storeA1}
	}
	none := mkToken("store:read")
	readCat := mkToken("store:read", "catalog:read")
	readBoth := mkToken("store:read", "catalog:read", "inventory:read")
	writeOnly := mkToken("store:read", "catalog:write")
	none.refuse(403, "GET", "/catalog-products", "", nil)
	none.refuse(403, "GET", "/products/"+p.ID, "", nil)
	none.refuse(403, "GET", "/collections", "", nil)
	none.refuse(403, "GET", "/collections/"+col.ID, "", nil)
	// catalog:read + inventory:read together serve the product cards; catalog:read alone does not (stock is inventory data)
	readCat.refuse(403, "GET", "/catalog-products", "", nil)
	readBoth.ok("GET", "/catalog-products?q="+e.tag, "", nil, nil)
	readBoth.ok("GET", "/products/"+p.ID, "", nil, nil)
	readCat.ok("GET", "/collections", "", nil, nil)
	readCat.ok("GET", "/collections/"+col.ID, "", nil, nil)
	// every write needs catalog:write
	for name, call := range map[string]func(a ccAdmin) int{
		"product create": func(a ccAdmin) int {
			s, _ := a.call("POST", "/products", e.key("w"), map[string]any{"name": e.name("w")})
			return s
		},
		"product patch": func(a ccAdmin) int {
			s, _ := a.call("PATCH", "/products/"+p.ID, e.key("w"), map[string]any{"expected_version": p.Version, "seo_title": "x"})
			return s
		},
		"sku create": func(a ccAdmin) int {
			s, _ := a.call("POST", "/skus", e.key("w"), map[string]any{"product_id": p.ID, "code": "WWWW1", "price_minor": 1})
			return s
		},
		"sku price": func(a ccAdmin) int {
			s, _ := a.call("POST", "/skus/"+sku.ID+"/price", e.key("w"), map[string]any{"price_minor": 5, "expected_version": sku.Version})
			return s
		},
		"collection create": func(a ccAdmin) int {
			s, _ := a.call("POST", "/collections", e.key("w"), map[string]any{"title": "w"})
			return s
		},
		"collection patch": func(a ccAdmin) int {
			s, _ := a.call("PATCH", "/collections/"+col.ID, e.key("w"), map[string]any{"expected_version": col.Version, "title": "w"})
			return s
		},
		"collection members": func(a ccAdmin) int {
			s, _ := a.call("PUT", "/collections/"+col.ID+"/products", e.key("w"), map[string]any{"expected_version": col.Version, "product_ids": []string{}})
			return s
		},
		"collection delete": func(a ccAdmin) int {
			s, _ := a.call("POST", "/collections/"+col.ID+"/delete", e.key("w"), map[string]any{"expected_version": col.Version})
			return s
		},
		"collection image": func(a ccAdmin) int { s, _ := e.upload2(a, "/collections/"+col.ID+"/image"); return s },
	} {
		for who, a := range map[string]ccAdmin{"store:read only": none, "catalog:read+inventory:read": readBoth} {
			if st := call(a); st != 403 {
				t.Fatalf("%s as %s must be 403, got %d", name, who, st)
			}
		}
		_ = writeOnly
	}
	// write permission alone is enough to write, and nothing the refused calls attempted took effect
	var again ccProduct
	e.a.ok("GET", "/products/"+p.ID, "", nil, &again)
	if again.Version != p.Version || again.SEOTitle != "" {
		t.Fatalf("a refused write changed the product: %+v", again)
	}
	writeOnly.ok("PATCH", "/products/"+p.ID, e.key("w"), map[string]any{"expected_version": p.Version, "seo_title": "writer"}, nil)
	// unauthenticated / garbage bearer
	anon := ccAdmin{t: t, h: e.srv, token: "not-a-token", store: e.h.f.storeA1}
	anon.refuse(401, "GET", "/catalog-products", "", nil)
}

// upload / upload2 send a valid PNG as a collection image through the given merchant.
func (a ccAdmin) upload(e *ccEnv, path string) (int, []byte) { return e.upload2(a, path) }

func (e *ccEnv) upload2(a ccAdmin, path string) (int, []byte) {
	body, ctype := ccMultipart(ccPNG(77), "x.png")
	st, out, _ := a.raw("POST", path, e.key("img"), body, ctype, nil)
	return st, out
}

// ---- CC11 idempotency ---------------------------------------------------------------------------------------------

func TestCatalogCoreCC11Idempotency(t *testing.T) {
	e := ccNew(t)
	count := func(q string, args ...any) int { return countRows(t, e.h.f.owner, q, args...) }
	// every command: the same key and bytes replay the first answer byte for byte and write nothing; the same key with
	// other bytes is 409.
	replay := func(label, method, path string, payload, other any, rows string, args ...any) {
		t.Helper()
		key := e.key("idem")
		s1, b1 := e.a.call(method, path, key, payload)
		if s1 != 200 {
			t.Fatalf("%s first call: %d %s", label, s1, b1)
		}
		before := count(rows, args...)
		s2, b2 := e.a.call(method, path, key, payload)
		if s2 != 200 || !bytes.Equal(b1, b2) {
			t.Fatalf("%s replay must return the first answer: %d\n%s\n%s", label, s2, b1, b2)
		}
		if after := count(rows, args...); after != before {
			t.Fatalf("%s replay wrote rows: %d -> %d", label, before, after)
		}
		if other != nil {
			if s3, b3 := e.a.call(method, path, key, other); s3 != 409 {
				t.Fatalf("%s same key, other bytes must be 409: %d %s", label, s3, b3)
			}
		}
	}
	tagLike := e.tag + "%"
	replay("product create", "POST", "/products", map[string]any{"name": e.name("idem p")}, map[string]any{"name": e.name("idem p other")},
		`SELECT count(*) FROM catalog.products WHERE name LIKE $1`, tagLike)
	p := e.product("Idem", nil)
	replay("product patch", "PATCH", "/products/"+p.ID, map[string]any{"expected_version": p.Version, "seo_title": "idem"}, map[string]any{"expected_version": p.Version, "seo_title": "other"},
		`SELECT version FROM catalog.products WHERE id=$1`, p.ID)
	var cur ccProduct
	e.a.ok("GET", "/products/"+p.ID, "", nil, &cur)
	if cur.Version != p.Version+1 {
		t.Fatalf("a replayed patch must bump the version once: %d", cur.Version)
	}
	replay("sku create", "POST", "/skus", map[string]any{"product_id": p.ID, "code": strings.ToUpper(e.tag) + "-IDEM", "price_minor": 100}, map[string]any{"product_id": p.ID, "code": strings.ToUpper(e.tag) + "-IDEM", "price_minor": 101},
		`SELECT count(*) FROM catalog.skus WHERE product_id=$1`, p.ID)
	var sku ccSKU
	var detail struct {
		SKUs []ccSKU `json:"skus"`
	}
	e.a.ok("GET", "/products/"+p.ID, "", nil, &detail)
	sku = detail.SKUs[0]
	replay("sku price", "POST", "/skus/"+sku.ID+"/price", map[string]any{"price_minor": 150, "expected_version": sku.Version, "compare_at_minor": 400}, map[string]any{"price_minor": 151, "expected_version": sku.Version, "compare_at_minor": 400},
		`SELECT version FROM catalog.skus WHERE id=$1`, sku.ID)
	// null and absent compare_at_minor are different requests (null clears, absent keeps): the replay hash tells them apart
	e.a.ok("GET", "/products/"+p.ID, "", nil, &detail)
	sku = detail.SKUs[0]
	key := e.key("nullabsent")
	if st, b := e.a.call("POST", "/skus/"+sku.ID+"/price", key, map[string]any{"price_minor": 160, "expected_version": sku.Version, "compare_at_minor": nil}); st != 200 {
		t.Fatalf("clear via null: %d %s", st, b)
	}
	if st, _ := e.a.call("POST", "/skus/"+sku.ID+"/price", key, map[string]any{"price_minor": 160, "expected_version": sku.Version}); st != 409 {
		t.Fatalf("same key, compare_at null then absent is a different request: %d", st)
	}
	if st, _ := e.a.call("POST", "/skus/"+sku.ID+"/price", key, map[string]any{"price_minor": 160, "expected_version": sku.Version, "compare_at_minor": nil}); st != 200 {
		t.Fatalf("the identical null request replays: %d", st)
	}
	key2 := e.key("absentnull")
	e.a.ok("GET", "/products/"+p.ID, "", nil, &detail)
	sku = detail.SKUs[0]
	if st, _ := e.a.call("POST", "/skus/"+sku.ID+"/price", key2, map[string]any{"price_minor": 161, "expected_version": sku.Version}); st != 200 {
		t.Fatal("absent compare_at")
	}
	if st, _ := e.a.call("POST", "/skus/"+sku.ID+"/price", key2, map[string]any{"price_minor": 161, "expected_version": sku.Version, "compare_at_minor": nil}); st != 409 {
		t.Fatalf("same key, absent then null is a different request: %d", st)
	}
	e.a.ok("GET", "/products/"+p.ID, "", nil, &detail)
	sku = detail.SKUs[0]
	full := map[string]any{"product_id": p.ID, "code": sku.Code, "price_minor": sku.PriceMinor, "weight_grams": 170, "expected_version": sku.Version}
	replay("sku patch", "PATCH", "/skus/"+sku.ID, full, map[string]any{"product_id": p.ID, "code": sku.Code, "price_minor": sku.PriceMinor, "weight_grams": 171, "expected_version": sku.Version},
		`SELECT version FROM catalog.skus WHERE id=$1`, sku.ID)

	var col ccCollection
	replay("collection create", "POST", "/collections", map[string]any{"title": e.name("idem c")}, map[string]any{"title": e.name("idem c 2")},
		`SELECT count(*) FROM catalog.collections WHERE slug LIKE $1`, tagLike)
	col = e.collection("Idem2", nil)
	replay("collection patch", "PATCH", "/collections/"+col.ID, map[string]any{"expected_version": col.Version, "description": "d"}, map[string]any{"expected_version": col.Version, "description": "e"},
		`SELECT version FROM catalog.collections WHERE id=$1`, col.ID)
	e.a.ok("GET", "/collections/"+col.ID, "", nil, &col)
	replay("collection members", "PUT", "/collections/"+col.ID+"/products", map[string]any{"expected_version": col.Version, "product_ids": []string{p.ID}}, map[string]any{"expected_version": col.Version, "product_ids": []string{}},
		`SELECT count(*) FROM catalog.collection_products WHERE collection_id=$1`, col.ID)
	e.a.ok("GET", "/collections/"+col.ID, "", nil, &col)

	// image upload: same key + same bytes replays the SAME image id (no second image), other bytes is 409
	imgKey := e.key("idem-img")
	body, ctype := ccMultipart(ccPNG(91), "a.png")
	s1, b1, _ := e.a.raw("POST", "/collections/"+col.ID+"/image", imgKey, body, ctype, nil)
	s2, b2, _ := e.a.raw("POST", "/collections/"+col.ID+"/image", imgKey, body, ctype, nil)
	if s1 != 200 || s2 != 200 || !bytes.Equal(b1, b2) || count(`SELECT count(*) FROM catalog.collection_images WHERE collection_id=$1`, col.ID) != 1 {
		t.Fatalf("image replay: %d %d\n%s\n%s", s1, s2, b1, b2)
	}
	other, ct2 := ccMultipart(ccPNG(92), "b.png")
	if s3, _, _ := e.a.raw("POST", "/collections/"+col.ID+"/image", imgKey, other, ct2, nil); s3 != 409 {
		t.Fatalf("same key other image bytes: %d", s3)
	}
	e.a.ok("GET", "/collections/"+col.ID, "", nil, &col)
	replay("collection delete", "POST", "/collections/"+col.ID+"/delete", map[string]any{"expected_version": col.Version}, map[string]any{"expected_version": col.Version + 1},
		`SELECT count(*) FROM catalog.collections WHERE id=$1`, col.ID)

	// a key must be present and well-formed on every write
	if st, _ := e.a.call("POST", "/products", "", map[string]any{"name": e.name("nokey")}); st != 422 && st != 400 {
		t.Fatalf("a write without Idempotency-Key must be refused, got %d", st)
	}
	// replay of a result stored BEFORE catalog v2 (no slug/status/options/option_values/compare_at in the JSON) still decodes
	k := e.key("legacy-p")
	var prod ccProduct
	e.a.ok("POST", "/products", k, map[string]any{"name": e.name("legacy answer")}, &prod)
	mustExec(t, e.h.f.owner, `UPDATE ops.command_results SET response=response-'slug'-'seo_title'-'seo_description'-'options'-'status' WHERE tenant_id=$1 AND store_id=$2 AND idempotency_key=$3`, e.h.f.tenantA, e.h.f.storeA1, k)
	var legacy ccProduct
	e.a.ok("POST", "/products", k, map[string]any{"name": e.name("legacy answer")}, &legacy)
	if legacy.ID != prod.ID || legacy.Name != prod.Name {
		t.Fatalf("a pre-0086 stored answer must replay and decode (same product id and name): %+v", legacy)
	}
	k2 := e.key("legacy-s")
	var s ccSKU
	e.a.ok("POST", "/skus", k2, map[string]any{"product_id": prod.ID, "code": strings.ToUpper(e.tag) + "-LEG", "price_minor": 100}, &s)
	mustExec(t, e.h.f.owner, `UPDATE ops.command_results SET response=response-'option_values'-'compare_at_minor'-'title' WHERE tenant_id=$1 AND store_id=$2 AND idempotency_key=$3`, e.h.f.tenantA, e.h.f.storeA1, k2)
	var ls ccSKU
	e.a.ok("POST", "/skus", k2, map[string]any{"product_id": prod.ID, "code": strings.ToUpper(e.tag) + "-LEG", "price_minor": 100}, &ls)
	if ls.ID != s.ID || ls.Code != s.Code {
		t.Fatalf("a pre-0086 stored SKU answer must replay and decode (same sku id and code): %+v", ls)
	}
}
