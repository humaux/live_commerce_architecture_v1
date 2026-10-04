package buyerhttp

// design_test.go: route admission of the three store-design reads (GET design/published, design/preview, media/s/{id})
// before any SQL or origin resolution (a zero handler has no resolver or pool, so every case must be answered earlier).
// A missing/malformed preview token is judged after origin resolution (404, same as an unknown token), so it is a PG
// case. The definers, preview-token binding and bytes run against real PG in tests/foundation/store_design_test.go.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"livecommerce/internal/httperror"
)

const storeMediaPath = "/v1/buyer/media/s/" + mediaImage

func TestDesignRouteMatching(t *testing.T) {
	if r := matchRoute(designPublishedPath); r.kind != designPublishedRoute {
		t.Fatalf("published = %+v", r)
	}
	if r := matchRoute(designPreviewPath); r.kind != designPreviewRoute {
		t.Fatalf("preview = %+v", r)
	}
	if r := matchRoute(storeMediaPath); r.kind != storeMediaRoute || r.image != mediaImage || r.id != "" {
		t.Fatalf("store media = %+v", r)
	}
	for _, path := range []string{"/v1/buyer/media/s/", "/v1/buyer/media/s/" + mediaImage + "/", storeMediaPath + "/x", "/v1/buyer/design", "/v1/buyer/design/published/", "/v1/buyer/design/draft"} {
		if k := matchRoute(path).kind; k == storeMediaRoute || k == designPublishedRoute || k == designPreviewRoute {
			t.Errorf("%q matched design kind %d", path, k)
		}
	}
	for _, k := range []routeKind{designPublishedRoute, designPreviewRoute, storeMediaRoute} {
		if !allowed(k, http.MethodGet) || allowed(k, http.MethodPost) || allowed(k, http.MethodPut) || allowed(k, http.MethodDelete) || !isDesignRoute(k) {
			t.Fatalf("kind %d must be GET only", k)
		}
	}
}

func TestDesignEarlyRefusals(t *testing.T) {
	h := httperror.Middleware(&handler{bffKey: testBuyerKey})
	token := strings.Repeat("A", 43)
	for _, tc := range []struct {
		name, method, path string
		edit               func(*http.Request)
		status             int
	}{
		{"published without bff key", http.MethodGet, designPublishedPath, func(r *http.Request) { r.Header.Del("X-Commerce-Buyer-BFF-Key") }, 401},
		{"published no origin", http.MethodGet, designPublishedPath, func(r *http.Request) { r.Header.Del("X-Commerce-Storefront-Origin") }, 422},
		{"published bearer", http.MethodGet, designPublishedPath, func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+testBuyerKey) }, 403},
		{"published query", http.MethodGet, designPublishedPath + "?preview=" + token, func(*http.Request) {}, 403},
		{"published post", http.MethodPost, designPublishedPath, func(*http.Request) {}, 405},
		{"published with a preview header", http.MethodGet, designPublishedPath, func(r *http.Request) { r.Header.Set(previewTokenHeader, token) }, 422},
		{"preview token in query", http.MethodGet, designPreviewPath + "?token=" + token, func(*http.Request) {}, 403},
		{"preview cookie", http.MethodGet, designPreviewPath, func(r *http.Request) { r.Header.Set("Cookie", "a=b") }, 403},
		{"media bearer", http.MethodGet, storeMediaPath, func(r *http.Request) { r.Header.Set("Authorization", "Bearer x") }, 403},
		{"media bad id", http.MethodGet, "/v1/buyer/media/s/nope", func(*http.Request) {}, 422},
		{"media upper-case id", http.MethodGet, "/v1/buyer/media/s/" + strings.ToUpper(mediaImage), func(*http.Request) {}, 422},
		{"media query", http.MethodGet, storeMediaPath + "?x=1", func(*http.Request) {}, 403},
		{"media post", http.MethodPost, storeMediaPath, func(*http.Request) {}, 405},
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
