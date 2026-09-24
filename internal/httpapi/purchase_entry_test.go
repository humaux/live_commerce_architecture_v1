package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"livecommerce/internal/catalog"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

const purchaseEntryPath = "/v1/admin/stores/11111111-1111-4111-8111-111111111111/products/22222222-2222-4222-8222-222222222222/purchase-entry"

func TestPurchaseEntryQuery(t *testing.T) {
	for _, raw := range []string{"", "?", "?locale=", "?locale=fr", "?locale=en&locale=en", "?locale=en&x=1", "?x=1", "?locale=%zz", "?locale=en;other=1", "?locale=" + strings.Repeat("a", 65)} {
		r := httptest.NewRequest(http.MethodGet, purchaseEntryPath+raw, nil)
		if _, err := purchaseEntryLocale(r.URL); !errors.Is(err, command.ErrInvalid) {
			t.Fatalf("accepted %q: %v", raw, err)
		}
	}
	for _, raw := range []string{"?locale=en", "?locale=zh-CN", "?locale=zh-TW"} {
		r := httptest.NewRequest(http.MethodGet, purchaseEntryPath+raw, nil)
		if value, err := purchaseEntryLocale(r.URL); err != nil || value == "" {
			t.Fatalf("rejected %q: %v", raw, err)
		}
	}
}

func TestPurchaseEntryHTTPBoundariesBeforeDatabase(t *testing.T) {
	h := NewHandler(nil)
	for _, tc := range []struct {
		name, method, url, body string
		headers                 map[string]string
		status                  int
	}{
		{"missing locale", "GET", purchaseEntryPath, "", nil, 422},
		{"duplicate locale", "GET", purchaseEntryPath + "?locale=en&locale=zh-CN", "", nil, 422},
		{"body", "GET", purchaseEntryPath + "?locale=en", "{}", nil, 422},
		{"idempotency", "GET", purchaseEntryPath + "?locale=en", "", map[string]string{"Idempotency-Key": "abc"}, 422},
		{"head", "HEAD", purchaseEntryPath + "?locale=en", "", nil, 405},
		{"post", "POST", purchaseEntryPath + "?locale=en", "", nil, 405},
		{"no auth", "GET", purchaseEntryPath + "?locale=en", "", nil, 401},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, tc.url, strings.NewReader(tc.body))
			for key, value := range tc.headers {
				r.Header.Set(key, value)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("status %d, headers %v, body %s", w.Code, w.Header(), w.Body.String())
			}
		})
	}
}

func TestPurchaseEntryErrorAndJSONContract(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{catalog.ErrPurchaseEntryUnavailable, 503, "unavailable"},
		{platform.ErrUnauthorized, 401, "unauthorized"},
		{platform.ErrForbidden, 403, "forbidden"},
		{platform.ErrScopeNotFound, 404, "not_found"},
	} {
		status, code := classify(tc.err)
		if status != tc.status || code != tc.code {
			t.Fatalf("classify %v: %d %s", tc.err, status, code)
		}
	}
	b, err := json.Marshal(catalog.PurchaseEntry{ProductID: "product", Locale: "en", State: "no_active_sku"})
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(b, &fields); err != nil || len(fields) != 4 || fields["url"] != "" {
		t.Fatalf("not the four-key DTO: %s, %v", b, err)
	}
}
