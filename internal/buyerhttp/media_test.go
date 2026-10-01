package buyerhttp

// media_test.go: route admission of GET /v1/buyer/media/p/{product}/{image} before any SQL or origin resolution
// (a zero handler has no resolver or pool, so every case below must be answered earlier). The bytes, cache headers
// and published/active filtering run against real PG in the independent test phase.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"livecommerce/internal/httperror"
)

const (
	mediaProduct = "11111111-1111-4111-8111-111111111111"
	mediaImage   = "22222222-2222-4222-8222-222222222222"
	mediaPath    = "/v1/buyer/media/p/" + mediaProduct + "/" + mediaImage
)

func TestMediaRouteMatching(t *testing.T) {
	if r := matchRoute(mediaPath); r.kind != mediaRoute || r.id != mediaProduct || r.image != mediaImage {
		t.Fatalf("media route = %+v", r)
	}
	for _, path := range []string{
		"/v1/buyer/media/p/", "/v1/buyer/media/p/" + mediaProduct, "/v1/buyer/media/p/" + mediaProduct + "/",
		mediaPath + "/", mediaPath + "/extra", "/v1/buyer/media/q/" + mediaProduct + "/" + mediaImage, "/v1/buyer/media/p//" + mediaImage,
	} {
		if matchRoute(path).kind == mediaRoute {
			t.Errorf("%q matched as a media route", path)
		}
	}
	if !allowed(mediaRoute, http.MethodGet) || allowed(mediaRoute, http.MethodPost) || allowed(mediaRoute, http.MethodPut) || allowed(mediaRoute, http.MethodDelete) {
		t.Fatal("media must be GET only")
	}
}

func TestMediaEarlyRefusals(t *testing.T) {
	h := httperror.Middleware(&handler{bffKey: testBuyerKey})
	for _, tc := range []struct {
		name, method, path string
		edit               func(*http.Request)
		status             int
	}{
		{"no bff key", http.MethodGet, mediaPath, func(r *http.Request) { r.Header.Del("X-Commerce-Buyer-BFF-Key") }, 401},
		{"bad bff key", http.MethodGet, mediaPath, func(r *http.Request) { r.Header.Set("X-Commerce-Buyer-BFF-Key", strings.Repeat("B", 43)) }, 401},
		{"no origin", http.MethodGet, mediaPath, func(r *http.Request) { r.Header.Del("X-Commerce-Storefront-Origin") }, 422},
		{"bearer is forbidden", http.MethodGet, mediaPath, func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+testBuyerKey) }, 403},
		{"cookie is forbidden", http.MethodGet, mediaPath, func(r *http.Request) { r.Header.Set("Cookie", "a=b") }, 403},
		{"query is forbidden", http.MethodGet, mediaPath + "?x=1", func(*http.Request) {}, 403},
		{"post", http.MethodPost, mediaPath, func(*http.Request) {}, 405},
		{"upper-case id", http.MethodGet, "/v1/buyer/media/p/" + strings.ToUpper(mediaProduct) + "/" + mediaImage, func(*http.Request) {}, 422},
		{"bad image id", http.MethodGet, "/v1/buyer/media/p/" + mediaProduct + "/nope", func(*http.Request) {}, 422},
		{"idempotency key", http.MethodGet, mediaPath, func(r *http.Request) { r.Header.Set("Idempotency-Key", "validkey1") }, 422},
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
