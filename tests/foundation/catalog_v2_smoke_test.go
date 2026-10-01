package foundation_test

// catalog_v2_smoke_test.go: author smoke of unit catalog-core (migration 0086, contracts/storefront-v2.md section A) on
// real PostgreSQL through the real merchant handler (httpapi) and the real buyer handler (buyerhttp). Evidence label:
// SANDBOX (isolated disposable PG, synthetic published origin). This is the AUTHOR's smoke; the independent gate tests
// (TCV*) are written by the tester from the contract and live elsewhere. Every product here carries a unique tag and the
// buyer queries filter on it (q=tag), because the shared fixture store holds other tests' rows.

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"livecommerce/internal/httpapi"
)

type v2Admin struct {
	t     *testing.T
	h     http.Handler
	token string
	store string
}

func (a v2Admin) do(method, path, key string, payload any, want int, out any) {
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
	a.raw(method, path, key, body, ctype, want, out)
}

func (a v2Admin) raw(method, path, key string, body []byte, ctype string, want int, out any) *httptest.ResponseRecorder {
	a.t.Helper()
	r := httptest.NewRequest(method, "/v1/admin/stores/"+a.store+path, bytes.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+a.token)
	if ctype != "" {
		r.Header.Set("Content-Type", ctype)
	}
	if key != "" {
		r.Header.Set("Idempotency-Key", key)
	}
	w := httptest.NewRecorder()
	a.h.ServeHTTP(w, r)
	if w.Code != want {
		a.t.Fatalf("%s %s: status=%d want=%d body=%s", method, path, w.Code, want, w.Body.String())
	}
	if out != nil {
		if err := json.Unmarshal(w.Body.Bytes(), out); err != nil {
			a.t.Fatalf("%s %s: %v body=%s", method, path, err, w.Body.String())
		}
	}
	return w
}

type v2Axis struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
}

type v2Product struct {
	ID             string   `json:"id"`
	Slug           string   `json:"slug"`
	Name           string   `json:"name"`
	Status         string   `json:"status"`
	SEOTitle       string   `json:"seo_title"`
	SEODescription string   `json:"seo_description"`
	Version        int64    `json:"version"`
	Options        []v2Axis `json:"options"`
}

type v2SKU struct {
	ID             string   `json:"id"`
	Code           string   `json:"code"`
	Title          string   `json:"title"`
	PriceMinor     int64    `json:"price_minor"`
	CompareAtMinor *int64   `json:"compare_at_minor"`
	OptionValues   []string `json:"option_values"`
	Version        int64    `json:"version"`
}

type v2Card struct {
	ID           string  `json:"id"`
	Slug         string  `json:"slug"`
	Title        string  `json:"title"`
	PriceMin     int64   `json:"price_min_minor"`
	PriceMax     int64   `json:"price_max_minor"`
	CompareAtMin *int64  `json:"compare_at_min_minor"`
	CoverImageID *string `json:"cover_image_id"`
	InStock      bool    `json:"in_stock"`
}
type v2List struct {
	Store    struct{ Name, Currency string } `json:"store"`
	Products []v2Card                        `json:"products"`
	Next     *string                         `json:"next"`
}

func v2ListRead(t *testing.T, h bhHarness, query string, status int) v2List {
	t.Helper()
	r := h.request(t, "GET", "/v1/buyer/catalog/v2/products?"+query, "", "", nil, nil)
	if status != 200 {
		bhError(t, r, status, map[int]string{404: "not_found", 422: "invalid_request"}[status])
		return v2List{}
	}
	return bhRead[v2List](t, r, 200)
}

type v2Detail struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Description string `json:"description"`
	SEO         struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	} `json:"seo"`
	Images   []struct{ ID string } `json:"images"`
	Options  []v2Axis              `json:"options"`
	Variants []struct {
		SKUID        string   `json:"sku_id"`
		Title        string   `json:"title"`
		OptionValues []string `json:"option_values"`
		Price        int64    `json:"price_minor"`
		CompareAt    *int64   `json:"compare_at_minor"`
		Stock        string   `json:"stock"`
	} `json:"variants"`
	Collections []struct {
		Slug  string `json:"slug"`
		Title string `json:"title"`
	} `json:"collections"`
}

func v2PNG(t *testing.T, shade uint8) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 3, 2))
	img.Set(0, 0, color.RGBA{shade, 0, 0, 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func v2Multipart(t *testing.T, data []byte) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, _ := w.CreateFormFile("file", "x.png")
	_, _ = part.Write(data)
	_ = w.Close()
	return buf.Bytes(), w.FormDataContentType()
}

func TestCatalogV2Smoke(t *testing.T) {
	h := bhSetup(t)
	a := v2Admin{t: t, h: httpapi.NewHandler(h.f.runtime), token: h.f.tokens["a"], store: h.f.storeA1}
	tag := strings.ToLower(t04Tag())
	t.Cleanup(func() {
		// The shared fixture store is reused by later gates: leave none of this test's products buyer-visible.
		mustExec(t, h.f.owner, `UPDATE catalog.products SET status='archived' WHERE tenant_id=$1 AND store_id=$2 AND name LIKE $3`, h.f.tenantA, h.f.storeA1, "v2-"+tag+"%")
		mustExec(t, h.f.owner, `UPDATE catalog.collections SET status='hidden' WHERE tenant_id=$1 AND store_id=$2 AND slug LIKE $3`, h.f.tenantA, h.f.storeA1, "v2-"+tag+"%")
	})

	// ---- product: default draft, generated slug, SEO, options -------------------------------------------------
	var tee, dup v2Product
	a.do("POST", "/products", t04Key("v2-p1"), map[string]any{"name": "v2-" + tag + " Summer Tee", "description": "100% cotton_soft"}, 200, &tee)
	if tee.Status != "draft" || tee.Slug != "v2-"+tag+"-summer-tee" || tee.Version != 1 || tee.Options == nil {
		t.Fatalf("new product must be a draft with a generated slug: %+v", tee)
	}
	a.do("POST", "/products", t04Key("v2-p2"), map[string]any{"name": "v2-" + tag + " Summer Tee"}, 200, &dup)
	if dup.Slug != tee.Slug+"-2" {
		t.Fatalf("colliding generated slug must get a suffix, got %q", dup.Slug)
	}
	var zh v2Product
	a.do("POST", "/products", t04Key("v2-p3"), map[string]any{"name": "夏季洋裝"}, 200, &zh)
	if len(zh.Slug) != 12 || strings.Trim(zh.Slug, "0123456789abcdef") != "" {
		t.Fatalf("title without ASCII must fall back to the id prefix, got %q", zh.Slug)
	}
	a.do("POST", "/products", t04Key("v2-p4"), map[string]any{"name": "x", "slug": tee.Slug}, 409, nil)
	a.do("POST", "/products", t04Key("v2-p5"), map[string]any{"name": "x", "slug": "Bad Slug"}, 422, nil)
	a.do("POST", "/products", t04Key("v2-p6"), map[string]any{"name": "x", "status": "archived"}, 422, nil)

	a.do("PATCH", "/products/"+tee.ID, t04Key("v2-patch"), map[string]any{"expected_version": tee.Version,
		"seo_title": "Tee SEO", "seo_description": "Soft tee", "options": []map[string]any{
			{"name": "Size", "values": []string{"S", "M", "L"}}, {"name": "Color", "values": []string{"Red", "Blue"}}}}, 200, &tee)
	if tee.Version != 2 || tee.SEOTitle != "Tee SEO" || len(tee.Options) != 2 {
		t.Fatalf("patch result: %+v", tee)
	}
	a.do("PATCH", "/products/"+tee.ID, t04Key("v2-stale"), map[string]any{"expected_version": 1, "status": "active"}, 409, nil)
	a.do("PATCH", "/products/"+tee.ID, t04Key("v2-empty"), map[string]any{"expected_version": tee.Version}, 422, nil)
	a.do("PATCH", "/products/"+tee.ID, t04Key("v2-badopt"), map[string]any{"expected_version": tee.Version, "options": []map[string]any{{"name": "x", "values": []string{}}}}, 422, nil)

	// ---- SKUs on a draft: option matrix, uniqueness, compare-at ------------------------------------------------
	mk := func(code string, price int64, compare *int64, values []string, want int) v2SKU {
		var s v2SKU
		body := map[string]any{"product_id": tee.ID, "code": code, "price_minor": price, "option_values": values}
		if compare != nil {
			body["compare_at_minor"] = *compare
		}
		a.do("POST", "/skus", t04Key("v2-sku-"+code), body, want, &s)
		return s
	}
	cmp := func(v int64) *int64 { return &v }
	sRedS := mk("V2"+tag+"-RS", 1000, cmp(1500), []string{"S", "Red"}, 200)
	sRedM := mk("V2"+tag+"-RM", 1200, nil, []string{"M", "Red"}, 200)
	sBlueL := mk("V2"+tag+"-BL", 900, nil, []string{"L", "Blue"}, 200)
	if sRedS.Title != "S / Red" || sRedS.CompareAtMinor == nil || *sRedS.CompareAtMinor != 1500 {
		t.Fatalf("derived title / compare-at: %+v", sRedS)
	}
	mk("V2"+tag+"-DUP", 1000, nil, []string{"S", "Red"}, 409)        // same combination
	mk("V2"+tag+"-BAD1", 1000, nil, []string{"XL", "Red"}, 422)      // value not on the axis
	mk("V2"+tag+"-BAD2", 1000, nil, []string{"S"}, 422)              // wrong arity
	mk("V2"+tag+"-BAD3", 1000, nil, nil, 422)                        // axes exist, no values
	mk("V2"+tag+"-CMP", 1000, cmp(1000), []string{"S", "Blue"}, 422) // compare-at must exceed price
	// Axes cannot drop a value an active SKU uses.
	a.do("PATCH", "/products/"+tee.ID, t04Key("v2-dropval"), map[string]any{"expected_version": tee.Version, "options": []map[string]any{
		{"name": "Size", "values": []string{"S", "M"}}, {"name": "Color", "values": []string{"Red", "Blue"}}}}, 409, nil)
	// Price change keeps compare-at coherent.
	a.do("POST", "/skus/"+sRedS.ID+"/price", t04Key("v2-price-over"), map[string]any{"price_minor": 1600, "expected_version": sRedS.Version}, 422, nil)
	var repriced v2SKU
	a.do("POST", "/skus/"+sRedS.ID+"/price", t04Key("v2-price"), map[string]any{"price_minor": 1600, "expected_version": sRedS.Version, "compare_at_minor": 2000}, 200, &repriced)
	if repriced.PriceMinor != 1600 || repriced.CompareAtMinor == nil || *repriced.CompareAtMinor != 2000 {
		t.Fatalf("price + compare-at: %+v", repriced)
	}
	var cleared v2SKU
	a.do("POST", "/skus/"+sRedS.ID+"/price", t04Key("v2-price2"), map[string]any{"price_minor": 1000, "expected_version": repriced.Version, "compare_at_minor": nil}, 200, &cleared)
	if cleared.CompareAtMinor != nil {
		t.Fatalf("compare_at null must clear: %+v", cleared)
	}
	a.do("POST", "/skus/"+sRedS.ID+"/price", t04Key("v2-price3"), map[string]any{"price_minor": 1000, "expected_version": cleared.Version, "compare_at_minor": 1500}, 200, &repriced)

	// ---- stock on a draft (allowed), photos on a draft ----------------------------------------------------------
	var wh struct{ ID string }
	a.do("POST", "/warehouses", t04Key("v2-wh"), map[string]any{"name": "v2-" + tag}, 200, &wh)
	adjust := func(sku string, delta int64) {
		a.do("POST", "/inventory/adjustments", t04Key("v2-adj"), map[string]any{"warehouse_id": wh.ID, "sku_id": sku, "delta": delta, "expected_version": 0, "reason": "v2 smoke"}, 200, nil)
	}
	adjust(sRedS.ID, 10) // in
	adjust(sRedM.ID, 3)  // low
	// sBlueL has no balance -> out
	body, ctype := v2Multipart(t, v2PNG(t, 10))
	a.raw("POST", "/products/"+tee.ID+"/images", t04Key("v2-img"), body, ctype, 200, nil)

	// ---- draft is invisible to buyers; publishing makes it visible ----------------------------------------------
	if got := v2ListRead(t, h, "q="+tag, 200); len(got.Products) != 0 {
		t.Fatalf("draft products leaked into the buyer list: %+v", got.Products)
	}
	for _, key := range []string{tee.ID, tee.Slug} {
		bhError(t, h.request(t, "GET", "/v1/buyer/catalog/v2/products/"+key, "", "", nil, nil), 404, "not_found")
	}
	a.do("PATCH", "/products/"+tee.ID, t04Key("v2-publish"), map[string]any{"expected_version": tee.Version, "status": "active"}, 200, &tee)

	list := v2ListRead(t, h, "q="+url.QueryEscape(tag+" summer"), 200)
	if list.Store.Currency == "" || list.Store.Name == "" || len(list.Products) != 1 {
		t.Fatalf("published list: %+v", list)
	}
	card := list.Products[0]
	if card.ID != tee.ID || card.Slug != tee.Slug || card.PriceMin != 900 || card.PriceMax != 1200 || card.CompareAtMin == nil || *card.CompareAtMin != 1500 || !card.InStock || card.CoverImageID == nil {
		t.Fatalf("card: %+v", card)
	}
	var detail v2Detail
	for _, key := range []string{tee.ID, tee.Slug} {
		detail = bhRead[v2Detail](t, h.request(t, "GET", "/v1/buyer/catalog/v2/products/"+key, "", "", nil, nil), 200)
		if detail.ID != tee.ID || len(detail.Variants) != 3 || len(detail.Options) != 2 || len(detail.Images) != 1 || detail.SEO.Title != "Tee SEO" {
			t.Fatalf("detail by %q: %+v", key, detail)
		}
	}
	stock := map[string]string{}
	for _, v := range detail.Variants {
		stock[v.Title] = v.Stock
	}
	if stock["S / Red"] != "in" || stock["M / Red"] != "low" || stock["L / Blue"] != "out" {
		t.Fatalf("stock hints: %v", stock)
	}
	if strings.Contains(string(h.request(t, "GET", "/v1/buyer/catalog/v2/products/"+tee.ID, "", "", nil, nil).body), "on_hand") {
		t.Fatal("raw stock counts leaked")
	}
	// Foreign / unknown key is indistinguishable from a draft.
	bhError(t, h.request(t, "GET", "/v1/buyer/catalog/v2/products/"+randomUUID(), "", "", nil, nil), 404, "not_found")
	bhError(t, h.request(t, "GET", "/v1/buyer/catalog/v2/products/no-such-slug", "", "", nil, nil), 404, "not_found")
	// Reservation moves in -> low -> out over the SUM of warehouses: adjust a second warehouse.
	var wh2 struct{ ID string }
	a.do("POST", "/warehouses", t04Key("v2-wh2"), map[string]any{"name": "v2-b-" + tag}, 200, &wh2)
	a.do("POST", "/inventory/adjustments", t04Key("v2-adj2"), map[string]any{"warehouse_id": wh2.ID, "sku_id": sBlueL.ID, "delta": 6, "expected_version": 0, "reason": "v2 smoke"}, 200, nil)
	r := h.request(t, "GET", "/v1/buyer/catalog/v2/products/"+tee.ID, "", "", nil, nil)
	if !strings.Contains(string(r.body), `"title":"L / Blue"`) || !strings.Contains(string(r.body), `"stock":"in"`) {
		t.Fatalf("available must sum across warehouses (6 -> in): %s", r.body)
	}

	// ---- listing: search literal, sort, price filter, paging ---------------------------------------------------
	var pct, cheap v2Product
	a.do("POST", "/products", t04Key("v2-pct"), map[string]any{"name": "v2-" + tag + " 50% off_sale", "status": "active"}, 200, &pct)
	a.do("POST", "/skus", t04Key("v2-pct-sku"), map[string]any{"product_id": pct.ID, "code": "V2" + tag + "-PCT", "price_minor": 100}, 200, nil)
	a.do("POST", "/products", t04Key("v2-cheap"), map[string]any{"name": "v2-" + tag + " Cheap Hat", "status": "active"}, 200, &cheap)
	var cheapSKU v2SKU
	a.do("POST", "/skus", t04Key("v2-cheap-sku"), map[string]any{"product_id": cheap.ID, "code": "V2" + tag + "-HAT", "price_minor": 50}, 200, &cheapSKU)
	if got := v2ListRead(t, h, "q="+url.QueryEscape("50% off_"), 200); len(got.Products) != 1 || got.Products[0].ID != pct.ID {
		t.Fatalf("q must be a literal match (escaped %% and _): %+v", got.Products)
	}
	if got := v2ListRead(t, h, "q="+url.QueryEscape("50%x"), 200); len(got.Products) != 0 {
		t.Fatalf("q wildcard leaked: %+v", got.Products)
	}
	if got := v2ListRead(t, h, "q="+strings.ToUpper("V2"+tag+"-hat"), 200); len(got.Products) != 1 || got.Products[0].ID != cheap.ID {
		t.Fatalf("q must match SKU code case-insensitively: %+v", got.Products)
	}
	ids := func(l v2List) []string {
		out := []string{}
		for _, p := range l.Products {
			out = append(out, p.ID)
		}
		return out
	}
	want := func(l v2List, order ...string) {
		t.Helper()
		if got := ids(l); strings.Join(got, ",") != strings.Join(order, ",") {
			t.Fatalf("order %v want %v", got, order)
		}
	}
	want(v2ListRead(t, h, "q=v2-"+tag+"&sort=price_asc", 200), cheap.ID, pct.ID, tee.ID)
	want(v2ListRead(t, h, "q=v2-"+tag+"&sort=price_desc", 200), tee.ID, pct.ID, cheap.ID) // by max price: 1200, 100, 50
	want(v2ListRead(t, h, "q=v2-"+tag+"&sort=title", 200), pct.ID, cheap.ID, tee.ID)      // "50% off.." < "cheap hat" < "summer tee"
	want(v2ListRead(t, h, "q=v2-"+tag+"&sort=price_asc&min=60&max=950", 200), pct.ID, tee.ID)
	want(v2ListRead(t, h, "q=v2-"+tag+"&min=2000", 200))
	first := v2ListRead(t, h, "q=v2-"+tag+"&sort=price_asc&limit=2", 200)
	if len(first.Products) != 2 || first.Next == nil {
		t.Fatalf("first page: %+v", first)
	}
	second := v2ListRead(t, h, "q=v2-"+tag+"&sort=price_asc&limit=2&after="+url.QueryEscape(*first.Next), 200)
	if len(second.Products) != 1 || second.Next != nil || second.Products[0].ID != tee.ID {
		t.Fatalf("second page: %+v", second)
	}
	v2ListRead(t, h, "q=v2-"+tag+"&sort=title&limit=2&after="+url.QueryEscape(*first.Next), 422) // cursor bound to its filter
	v2ListRead(t, h, "limit=49", 422)
	v2ListRead(t, h, "collection=nope", 404)

	// ---- collections: CRUD, membership order, sort modes, image, hidden ----------------------------------------
	type colJSON = struct {
		ID           string  `json:"id"`
		Slug         string  `json:"slug"`
		Title        string  `json:"title"`
		SortMode     string  `json:"sort_mode"`
		Status       string  `json:"status"`
		ImageID      *string `json:"image_id"`
		Version      int64   `json:"version"`
		ProductCount int     `json:"product_count"`
		Products     []struct {
			ProductID string `json:"product_id"`
			Position  int    `json:"position"`
		} `json:"products"`
	}
	var c colJSON
	a.do("POST", "/collections", t04Key("v2-col"), map[string]any{"title": "v2-" + tag + " New In", "description": "fresh"}, 200, &c)
	if c.Slug != "v2-"+tag+"-new-in" || c.SortMode != "manual" || c.Status != "active" || c.Version != 1 || c.ProductCount != 0 {
		t.Fatalf("new collection: %+v", c)
	}
	a.do("POST", "/collections", t04Key("v2-col-dup"), map[string]any{"title": "x", "slug": c.Slug}, 409, nil)
	a.do("POST", "/collections", t04Key("v2-col-bad"), map[string]any{"title": "x", "sort_mode": "title"}, 422, nil)
	a.do("PUT", "/collections/"+c.ID+"/products", t04Key("v2-set"), map[string]any{"expected_version": c.Version, "product_ids": []string{cheap.ID, tee.ID, pct.ID}}, 200, &c)
	if c.Version != 2 || len(c.Products) != 3 || c.Products[0].ProductID != cheap.ID || c.Products[2].ProductID != pct.ID {
		t.Fatalf("membership: %+v", c)
	}
	a.do("PUT", "/collections/"+c.ID+"/products", t04Key("v2-set-stale"), map[string]any{"expected_version": 1, "product_ids": []string{}}, 409, nil)
	a.do("PUT", "/collections/"+c.ID+"/products", t04Key("v2-set-dup"), map[string]any{"expected_version": c.Version, "product_ids": []string{cheap.ID, cheap.ID}}, 422, nil)
	a.do("PUT", "/collections/"+c.ID+"/products", t04Key("v2-set-foreign"), map[string]any{"expected_version": c.Version, "product_ids": []string{randomUUID()}}, 404, nil)
	// Manual order is the default sort of the collection page; an explicit sort overrides it.
	want(v2ListRead(t, h, "collection="+c.Slug, 200), cheap.ID, tee.ID, pct.ID)
	want(v2ListRead(t, h, "collection="+c.Slug+"&sort=price_asc", 200), cheap.ID, pct.ID, tee.ID)
	a.do("PATCH", "/collections/"+c.ID, t04Key("v2-col-sort"), map[string]any{"expected_version": c.Version, "sort_mode": "price_desc"}, 200, &c)
	want(v2ListRead(t, h, "collection="+c.Slug, 200), tee.ID, pct.ID, cheap.ID)
	// A draft product in a collection is not shown.
	var hidden v2Product
	a.do("POST", "/products", t04Key("v2-draft-member"), map[string]any{"name": "v2-" + tag + " Draft Member"}, 200, &hidden)
	a.do("PUT", "/collections/"+c.ID+"/products", t04Key("v2-set2"), map[string]any{"expected_version": c.Version, "product_ids": []string{hidden.ID, cheap.ID, tee.ID, pct.ID}}, 200, &c)
	want(v2ListRead(t, h, "collection="+c.Slug, 200), tee.ID, pct.ID, cheap.ID)

	// image: upload, buyer bytes via /media/c, replace (new id), old id 404
	pngA := v2PNG(t, 20)
	body, ctype = v2Multipart(t, pngA)
	var img struct {
		ID string `json:"id"`
	}
	json.Unmarshal(a.raw("POST", "/collections/"+c.ID+"/image", t04Key("v2-cimg"), body, ctype, 200, nil).Body.Bytes(), &img)
	mediaGet := func(collection, image string) (int, []byte, http.Header) {
		req, _ := http.NewRequest("GET", h.server.URL+"/v1/buyer/media/c/"+collection+"/"+image, nil)
		req.Header.Set("X-Commerce-Buyer-BFF-Key", h.key)
		req.Header.Set("X-Commerce-Storefront-Origin", h.origin)
		res, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		data, _ := io.ReadAll(res.Body)
		return res.StatusCode, data, res.Header
	}
	if status, data, header := mediaGet(c.ID, img.ID); status != 200 || !bytes.Equal(data, pngA) || header.Get("Content-Type") != "image/png" ||
		!strings.Contains(header.Get("Cache-Control"), "immutable") || header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("collection media: status=%d type=%q cache=%q", status, header.Get("Content-Type"), header.Get("Cache-Control"))
	}
	if status, _, _ := mediaGet(c.ID, randomUUID()); status != 404 {
		t.Fatalf("unknown collection image id: %d", status)
	}
	bad, ctype2 := v2Multipart(t, []byte("<svg xmlns='http://www.w3.org/2000/svg'/>"))
	a.raw("POST", "/collections/"+c.ID+"/image", t04Key("v2-cimg-svg"), bad, ctype2, 422, nil)
	body, ctype = v2Multipart(t, v2PNG(t, 30))
	var img2 struct {
		ID string `json:"id"`
	}
	json.Unmarshal(a.raw("POST", "/collections/"+c.ID+"/image", t04Key("v2-cimg2"), body, ctype, 200, nil).Body.Bytes(), &img2)
	if img2.ID == img.ID {
		t.Fatal("replacing the image must change the immutable id")
	}
	if status, _, _ := mediaGet(c.ID, img.ID); status != 404 {
		t.Fatalf("replaced image id still served: %d", status)
	}
	cols := bhRead[struct {
		Collections []struct {
			Slug         string  `json:"slug"`
			ImageID      *string `json:"image_id"`
			ProductCount int     `json:"product_count"`
		} `json:"collections"`
	}](t, h.request(t, "GET", "/v1/buyer/catalog/v2/collections", "", "", nil, nil), 200)
	found := false
	for _, cc := range cols.Collections {
		if cc.Slug == c.Slug {
			found = true
			if cc.ProductCount != 3 || cc.ImageID == nil || *cc.ImageID != img2.ID { // draft member not counted
				t.Fatalf("collection card: %+v", cc)
			}
		}
	}
	if !found {
		t.Fatal("active collection missing from the buyer list")
	}
	one := bhRead[map[string]any](t, h.request(t, "GET", "/v1/buyer/catalog/v2/collections/"+c.Slug, "", "", nil, nil), 200)
	if one["title"] != "v2-"+tag+" New In" || len(one) != 4 {
		t.Fatalf("collection detail: %v", one)
	}
	// detail of the product lists its active collections
	if raw := h.request(t, "GET", "/v1/buyer/catalog/v2/products/"+tee.Slug, "", "", nil, nil).body; !strings.Contains(string(raw), `"collections":[{"slug":"`+c.Slug+`"`) {
		t.Fatalf("product detail lacks its collection: %s", raw)
	}
	// hidden collection is gone for buyers (list, detail, filter, image)
	a.do("PATCH", "/collections/"+c.ID, t04Key("v2-hide"), map[string]any{"expected_version": c.Version, "status": "hidden"}, 200, &c)
	bhError(t, h.request(t, "GET", "/v1/buyer/catalog/v2/collections/"+c.Slug, "", "", nil, nil), 404, "not_found")
	v2ListRead(t, h, "collection="+c.Slug, 404)
	if status, _, _ := mediaGet(c.ID, img2.ID); status != 404 {
		t.Fatalf("hidden collection image served: %d", status)
	}

	// ---- admin reads: product list / detail, collection list / delete ------------------------------------------
	var summaries struct {
		Items []struct {
			ID            string  `json:"id"`
			Status        string  `json:"status"`
			CoverImageID  *string `json:"cover_image_id"`
			PriceMinMinor *int64  `json:"price_min_minor"`
			PriceMaxMinor *int64  `json:"price_max_minor"`
			SKUCount      int     `json:"sku_count"`
			Available     int64   `json:"available"`
		} `json:"items"`
		NextCursor string `json:"next_cursor"`
	}
	a.do("GET", "/catalog-products?q="+tag+"&limit=2", "", nil, 200, &summaries)
	if len(summaries.Items) != 2 || summaries.NextCursor == "" {
		t.Fatalf("summaries page 1: %+v", summaries)
	}
	var total int
	for cursor := ""; ; {
		var pg = summaries
		pg.Items = nil
		path := "/catalog-products?q=" + tag + "&limit=2"
		if cursor != "" {
			path += "&cursor=" + url.QueryEscape(cursor)
		}
		a.do("GET", path, "", nil, 200, &pg)
		total += len(pg.Items)
		if cursor = pg.NextCursor; cursor == "" {
			break
		}
	}
	if total != 5 { // tee, dup, pct, cheap, hidden (zh has no tag)
		t.Fatalf("summaries paged total %d want 5", total)
	}
	a.do("GET", "/catalog-products?q="+tag+"&status=draft", "", nil, 200, &summaries)
	if len(summaries.Items) != 2 { // dup and the draft collection member carry the tag; the zh product does not
		t.Fatalf("draft filter: %+v", summaries)
	}
	a.do("GET", "/catalog-products?status=bogus", "", nil, 422, nil)
	a.do("GET", "/catalog-products?q="+tag+"&sort=x", "", nil, 422, nil)
	var detailAdmin struct {
		SKUs []struct {
			Title     string `json:"title"`
			Available int64  `json:"available"`
		} `json:"skus"`
	}
	a.do("GET", "/products/"+tee.ID, "", nil, 200, &detailAdmin)
	if len(detailAdmin.SKUs) != 3 {
		t.Fatalf("admin product detail: %+v", detailAdmin)
	}
	a.do("GET", "/products/"+randomUUID(), "", nil, 404, nil)
	var listed struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	a.do("GET", "/collections", "", nil, 200, &listed)
	// ---- tenant isolation: another tenant's merchant cannot touch these rows ---------------------------------
	other := v2Admin{t: t, h: a.h, token: h.f.tokens["b"], store: h.f.storeB}
	other.do("GET", "/products/"+tee.ID, "", nil, 404, nil)
	other.do("PATCH", "/products/"+tee.ID, t04Key("v2-x"), map[string]any{"expected_version": tee.Version, "status": "archived"}, 404, nil)
	other.do("PUT", "/collections/"+c.ID+"/products", t04Key("v2-x2"), map[string]any{"expected_version": 1, "product_ids": []string{}}, 404, nil)
	crossStore := v2Admin{t: t, h: a.h, token: h.f.tokens["b"], store: h.f.storeA1}
	crossStore.do("GET", "/catalog-products", "", nil, 404, nil) // wrong tenant for the path store: scope not found

	a.do("POST", "/collections/"+c.ID+"/delete", t04Key("v2-del-stale"), map[string]any{"expected_version": 1}, 409, nil)
	a.do("POST", "/collections/"+c.ID+"/delete", t04Key("v2-del"), map[string]any{"expected_version": c.Version}, 200, nil)
	a.do("GET", "/collections/"+c.ID, "", nil, 404, nil)

	// ---- publication gate: unpublish the origin and every v2 read is 404 ---------------------------------------
	mustExec(t, h.f.owner, `UPDATE control.storefront_publications SET published=false WHERE tenant_id=$1 AND store_id=$2`, h.f.tenantA, h.f.storeA1)
	v2ListRead(t, h, "q="+tag, 404)
	bhError(t, h.request(t, "GET", "/v1/buyer/catalog/v2/collections", "", "", nil, nil), 404, "not_found")
	bhError(t, h.request(t, "GET", "/v1/buyer/catalog/v2/products/"+tee.Slug, "", "", nil, nil), 404, "not_found")
	if status, _, _ := mediaGet(c.ID, img2.ID); status != 404 {
		t.Fatalf("unpublished store media: %d", status)
	}
}
