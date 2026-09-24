package buyerhttp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"livecommerce/internal/httperror"
)

func TestBootstrapExactRouteAndNoReplayKey(t *testing.T) {
	if got := matchRoute("/v1/buyer/session/bootstrap"); got.kind != bootstrapRoute || !allowed(got.kind, http.MethodPost) {
		t.Fatal("bootstrap POST route missing")
	}
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodDelete, http.MethodPut, http.MethodOptions} {
		if allowed(bootstrapRoute, method) {
			t.Fatalf("bootstrap accepted %s", method)
		}
	}
	for _, path := range []string{"/v1/buyer/session/bootstrap/", "/v1/buyer/session/bootstrap/extra"} {
		if matchRoute(path).kind != unknownRoute {
			t.Fatalf("bootstrap accepted path %s", path)
		}
	}
	r := httptest.NewRequest(http.MethodPost, "http://internal/v1/buyer/session/bootstrap", strings.NewReader(`{}`))
	if _, ok := keyFor(r, true, false); !ok {
		t.Fatal("bootstrap requires no replay key")
	}
	r.Header.Set("Idempotency-Key", "validkey1")
	if _, ok := keyFor(r, true, false); ok {
		t.Fatal("bootstrap accepted replay key")
	}
}

func TestBootstrapEarlyAdmissionAndBody(t *testing.T) {
	h := httperror.Middleware(&handler{bffKey: testBuyerKey})
	for _, tc := range []struct {
		name   string
		edit   func(*http.Request)
		status int
	}{
		{"missing bearer", func(r *http.Request) { r.Header.Del("Authorization") }, 401},
		{"duplicate bearer", func(r *http.Request) { r.Header.Add("Authorization", "Bearer "+testBuyerKey) }, 401},
		{"replay key", func(r *http.Request) { r.Header.Set("Idempotency-Key", "validkey1") }, 422},
		{"query", func(r *http.Request) { r.URL.RawQuery = "x=1" }, 403},
		{"bare query", func(r *http.Request) { r.URL.ForceQuery = true }, 403},
		{"browser origin", func(r *http.Request) { r.Header.Set("Origin", "https://shop.example") }, 403},
		{"missing BFF key", func(r *http.Request) { r.Header.Del("X-Commerce-Buyer-BFF-Key") }, 401},
		{"missing storefront origin", func(r *http.Request) { r.Header.Del("X-Commerce-Storefront-Origin") }, 422},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "http://internal/v1/buyer/session/bootstrap", strings.NewReader(`{}`))
			r.Header.Set("X-Commerce-Buyer-BFF-Key", testBuyerKey)
			r.Header.Set("X-Commerce-Storefront-Origin", "https://shop.example")
			r.Header.Set("Authorization", "Bearer "+testBuyerKey)
			r.Header.Set("Content-Type", "application/json")
			tc.edit(r)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("status=%d, want %d", w.Code, tc.status)
			}
		})
	}
	for _, body := range []string{"{}", `{"extra":1}`, "null", "[]", "{} {}", ""} {
		r := httptest.NewRequest(http.MethodPost, "http://internal/v1/buyer/session/bootstrap", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		err := decodeJSON(r, &struct{}{})
		if body == "{}" && err != nil || body != "{}" && err == nil {
			t.Fatalf("body %q accepted=%v", body, err == nil)
		}
	}
}
