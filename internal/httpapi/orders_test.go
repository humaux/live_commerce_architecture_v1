package httpapi

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"livecommerce/internal/command"
)

func TestMerchantOrdersQuery(t *testing.T) {
	for _, raw := range []string{"", "limit=1&state=DRAFT", "limit=100&cursor=abc&state=all"} {
		u := &url.URL{RawQuery: raw}
		if _, err := parseOrdersQuery(u); err != nil {
			t.Fatalf("valid %q: %v", raw, err)
		}
	}
	for _, raw := range []string{"limit=0", "limit=101", "limit=01", "limit=", "limit=1&limit=2", "limit=1&", "state=all&&limit=1", "cursor=", "state=", "state=draft", "state=DRAFT&state=all", "other=x", "limit=bad", "limit=1;state=DRAFT"} {
		u := &url.URL{RawQuery: raw}
		if _, err := parseOrdersQuery(u); !errors.Is(err, command.ErrInvalid) {
			t.Fatalf("invalid %q accepted: %v", raw, err)
		}
	}
	if _, err := parseOrdersQuery(&url.URL{ForceQuery: true}); !errors.Is(err, command.ErrInvalid) {
		t.Fatalf("bare ?: %v", err)
	}
}

func TestMerchantOrdersTransportRejectsBeforeDatabase(t *testing.T) {
	handler := NewHandler(nil)
	base := "/v1/admin/stores/22222222-2222-4222-8222-222222222222/orders"
	for _, testcase := range []struct {
		name, method, path, body string
		setup                    func(*http.Request)
		status                   int
	}{
		{"head list", "HEAD", base, "", nil, 405},
		{"head detail", "HEAD", base + "/33333333-3333-4333-8333-333333333333", "", nil, 405},
		{"body", "GET", base, "x", nil, 422},
		{"transfer", "GET", base, "", func(r *http.Request) { r.TransferEncoding = []string{"chunked"} }, 422},
		{"idempotency even empty", "GET", base, "", func(r *http.Request) { r.Header["Idempotency-Key"] = []string{""} }, 422},
		{"duplicate", "GET", base + "?limit=1&limit=2", "", nil, 422},
		{"unknown", "GET", base + "?buyer_id=x", "", nil, 422},
		{"bare query", "GET", base + "?", "", nil, 422},
		{"detail query", "GET", base + "/33333333-3333-4333-8333-333333333333?state=all", "", nil, 422},
		{"invalid order id", "GET", base + "/NOT-ID", "", nil, 422},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			var req *http.Request
			if testcase.body == "" {
				req = httptest.NewRequest(testcase.method, testcase.path, nil)
			} else {
				req = httptest.NewRequest(testcase.method, testcase.path, strings.NewReader(testcase.body))
			}
			if testcase.setup != nil {
				testcase.setup(req)
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			if w.Code != testcase.status {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			if w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("missing no-store")
			}
		})
	}
}
