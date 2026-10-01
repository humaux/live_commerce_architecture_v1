package buyerhttp

// catalogv2_test.go: pure logic of catalogv2.go (route admission, strict query grammar, cursor, closed projection)
// before any SQL; visibility, stock hints and paging against real PG are in tests/foundation/catalog_v2_smoke_test.go.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"livecommerce/internal/httperror"
)

func TestCatalogV2RouteMatching(t *testing.T) {
	for path, want := range map[string]routeKind{
		v2Prefix + "products":                                  catalogV2ProductsRoute,
		v2Prefix + "collections":                               catalogV2CollectionsRoute,
		v2Prefix + "products/summer-dress":                     catalogV2ProductRoute,
		v2Prefix + "products/" + mediaProduct:                  catalogV2ProductRoute,
		v2Prefix + "collections/new-in":                        catalogV2CollectionRoute,
		"/v1/buyer/media/c/" + mediaProduct + "/" + mediaImage: collectionMediaRoute,
	} {
		if got := matchRoute(path); got.kind != want {
			t.Errorf("%s -> %v want %v", path, got.kind, want)
		}
	}
	if r := matchRoute(v2Prefix + "products/summer-dress"); r.slug != "summer-dress" {
		t.Errorf("slug = %q", r.slug)
	}
	for _, path := range []string{
		v2Prefix, v2Prefix + "other", v2Prefix + "products/", v2Prefix + "products/Upper", v2Prefix + "products/a/b", v2Prefix + "products/a_b",
		v2Prefix + "collections/" + mediaProduct, v2Prefix + "collections/", v2Prefix + "products/" + strings.Repeat("a", 81),
		"/v1/buyer/media/c/", "/v1/buyer/media/c/" + mediaProduct, "/v1/buyer/media/c/" + mediaProduct + "/" + mediaImage + "/x",
	} {
		if k := matchRoute(path).kind; k != unknownRoute {
			t.Errorf("%q matched route kind %v", path, k)
		}
	}
	for _, k := range []routeKind{catalogV2ProductsRoute, catalogV2ProductRoute, catalogV2CollectionsRoute, catalogV2CollectionRoute, collectionMediaRoute} {
		if !allowed(k, http.MethodGet) || allowed(k, http.MethodPost) || allowed(k, http.MethodPut) || allowed(k, http.MethodDelete) || !isPublicRoute(k) {
			t.Errorf("route kind %v must be a public GET-only route", k)
		}
	}
}

func TestCatalogV2EarlyRefusals(t *testing.T) {
	h := httperror.Middleware(&handler{bffKey: testBuyerKey})
	for _, tc := range []struct {
		name, method, path string
		edit               func(*http.Request)
		status             int
	}{
		{"no bff key", http.MethodGet, v2Prefix + "products", func(r *http.Request) { r.Header.Del("X-Commerce-Buyer-BFF-Key") }, 401},
		{"bearer is forbidden", http.MethodGet, v2Prefix + "collections", func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+testBuyerKey) }, 403},
		{"cookie is forbidden", http.MethodGet, v2Prefix + "products", func(r *http.Request) { r.Header.Set("Cookie", "a=b") }, 403},
		{"query on collections is forbidden", http.MethodGet, v2Prefix + "collections?x=1", func(*http.Request) {}, 403},
		{"query on detail is forbidden", http.MethodGet, v2Prefix + "products/a?x=1", func(*http.Request) {}, 403},
		{"bare question mark", http.MethodGet, v2Prefix + "products?", func(*http.Request) {}, 403},
		{"post", http.MethodPost, v2Prefix + "products", func(*http.Request) {}, 405},
		{"unknown v2 path", http.MethodGet, v2Prefix + "orders", func(*http.Request) {}, 404},
		{"collection media bearer", http.MethodGet, "/v1/buyer/media/c/" + mediaProduct + "/" + mediaImage, func(r *http.Request) { r.Header.Set("Authorization", "Bearer x") }, 403},
		{"collection media bad id", http.MethodGet, "/v1/buyer/media/c/" + mediaProduct + "/nope", func(*http.Request) {}, 422},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, "http://internal"+tc.path, nil)
			r.Header.Set("X-Commerce-Buyer-BFF-Key", testBuyerKey)
			r.Header.Set("X-Commerce-Storefront-Origin", "https://shop.example")
			tc.edit(r)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("status %d want %d body %s", w.Code, tc.status, w.Body.String())
			}
		})
	}
}

func TestParseV2Query(t *testing.T) {
	q, after, err := parseV2Query("")
	if err != nil || q.limit != v2DefaultLimit || after != "" || q.offset != 0 {
		t.Fatalf("default query: %+v %q %v", q, after, err)
	}
	q, _, err = parseV2Query("collection=new-in&q=%E5%A4%8F%E5%AD%A3&sort=price_asc&min=100&max=900&limit=48")
	if err != nil || q.collection != "new-in" || q.q != "夏季" || q.sort != "price_asc" || *q.min != 100 || *q.max != 900 || q.limit != 48 {
		t.Fatalf("full query: %+v %v", q, err)
	}
	for _, bad := range []string{
		"limit=0", "limit=49", "limit=x", "limit=-1", "limit=", "q=", "sort=bogus", "sort=manual", "min=-1", "min=1.5", "min=abc", "max=1000000000001",
		"min=500&max=100", "collection=Bad", "collection=a_b", "x=1", "q=a&q=b", "q=" + strings.Repeat("a", 61), "q=%00", "after=!!!", "a=1;b=2", "&limit=1", "limit=1&",
		"after=" + strings.Repeat("A", 300),
	} {
		if _, _, err := parseV2Query(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	if q, _, err := parseV2Query("q=" + strings.Repeat("字", 60)); err != nil || utf8Len(q.q) != 60 {
		t.Errorf("60 rune query rejected: %v", err)
	}
}

func utf8Len(s string) int { return len([]rune(s)) }

func TestV2CursorBoundToFilter(t *testing.T) {
	a, _, _ := parseV2Query("sort=title&limit=2")
	b, _, _ := parseV2Query("sort=newest&limit=2")
	c, _, _ := parseV2Query("sort=title&limit=3")
	if a.digest() == b.digest() || a.digest() == c.digest() {
		t.Fatal("filter digest must cover sort and limit")
	}
	cursor := encodeV2Cursor(a.digest(), 2)
	q, after, err := parseV2Query("sort=title&limit=2&after=" + cursor)
	if err != nil || q.offset != 2 || after != cursor {
		t.Fatalf("round trip: %+v %v", q, err)
	}
	for name, raw := range map[string]string{
		"other sort":  "sort=newest&limit=2&after=" + cursor,
		"other limit": "sort=title&limit=3&after=" + cursor,
		"zero offset": "sort=title&limit=2&after=" + encodeV2Cursor(a.digest(), 0),
		"huge offset": "sort=title&limit=2&after=" + encodeV2Cursor(a.digest(), v2MaxOffset+1),
		"not base64":  "sort=title&limit=2&after=@@@",
		"padded":      "sort=title&limit=2&after=" + cursor + "=",
		"unknown key": "sort=title&limit=2&after=eyJ2IjoxLCJmIjoieCIsIm8iOjEsInoiOjF9",
	} {
		if _, _, err := parseV2Query(raw); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestProjectV2IsClosedAndNonNull(t *testing.T) {
	q, _, _ := parseV2Query("")
	out, err := projectV2(catalogV2ProductsRoute, []byte(`{"store":{"name":"S","currency":"TWD"},"products":[],"next_offset":24}`), q)
	if err != nil {
		t.Fatal(err)
	}
	list := out.(v2List)
	if list.Next == nil || list.Products == nil || len(list.Products) != 0 {
		t.Fatalf("list projection: %+v", list)
	}
	if _, after, err := parseV2Query("after=" + *list.Next); err != nil || after == "" {
		t.Errorf("projected cursor does not round-trip: %v", err)
	}
	last, _ := projectV2(catalogV2ProductsRoute, []byte(`{"store":{"name":"S","currency":"TWD"},"products":null,"next_offset":null}`), q)
	if l := last.(v2List); l.Next != nil || l.Products == nil {
		t.Errorf("last page: %+v", l)
	}
	// An unknown key means the database drifted from the contract: refuse rather than widen the response.
	for kind, raw := range map[routeKind]string{
		catalogV2ProductsRoute:    `{"store":{"name":"S","currency":"TWD"},"products":[{"id":"x","slug":"s","title":"t","price_min_minor":1,"price_max_minor":1,"compare_at_min_minor":null,"cover_image_id":null,"in_stock":true,"on_hand":5}],"next_offset":null}`,
		catalogV2ProductRoute:     `{"id":"x","slug":"s","title":"t","description":"","seo":{"title":"","description":""},"images":[],"options":[],"variants":[],"collections":[],"cost":1}`,
		catalogV2CollectionsRoute: `{"collections":[{"slug":"s","title":"t","image_id":null,"product_count":1,"internal":1}]}`,
		catalogV2CollectionRoute:  `{"slug":"s","title":"t","description":"","image_id":null,"status":"active"}`,
	} {
		if _, err := projectV2(kind, []byte(raw), q); err == nil {
			t.Errorf("kind %v accepted an unknown key", kind)
		}
	}
	detail, err := projectV2(catalogV2ProductRoute, []byte(`{"id":"x","slug":"s","title":"t","description":"","seo":{"title":"","description":""},"images":null,"options":null,"variants":[{"sku_id":"y","title":"預設","option_values":null,"price_minor":1,"compare_at_minor":null,"stock":"in"}],"collections":null}`), q)
	if err != nil {
		t.Fatal(err)
	}
	d := detail.(v2Detail)
	if d.Images == nil || d.Options == nil || d.Collections == nil || d.Variants[0].OptionValues == nil {
		t.Errorf("null slices leaked: %+v", d)
	}
}
