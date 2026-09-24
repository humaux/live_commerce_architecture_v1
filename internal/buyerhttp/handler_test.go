package buyerhttp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"livecommerce/internal/buyer"
	"livecommerce/internal/command"
	"livecommerce/internal/domains"
	"livecommerce/internal/httperror"
)

const testBuyerKey = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

type failedBody struct{ err error }

func (b failedBody) Read([]byte) (int, error) { return 0, b.err }

func TestBodyTimeoutIsUnavailableNotMalformedInput(t *testing.T) {
	for _, decode := range []func(*http.Request) error{
		noBody,
		func(r *http.Request) error { return decodeJSON(r, &struct{}{}) },
	} {
		for _, err := range []error{os.ErrDeadlineExceeded, context.DeadlineExceeded, context.Canceled} {
			r := httptest.NewRequest("POST", "/v1/buyer/session", nil)
			r.Header.Set("Content-Type", "application/json")
			r.Body = io.NopCloser(failedBody{err})
			r.ContentLength = -1
			status, code := classify(decode(r))
			if status != 503 || code != "unavailable" {
				t.Fatal("body timeout or cancellation mislabeled as caller input error")
			}
		}
	}
}

func TestExactRoutesAndMethods(t *testing.T) {
	for _, tc := range []struct {
		path   string
		method string
		kind   routeKind
		ok     bool
	}{
		{"/v1/buyer/session", http.MethodPost, sessionRoute, true},
		{"/v1/buyer/session", http.MethodDelete, sessionRoute, true},
		{"/v1/buyer/session", http.MethodHead, sessionRoute, false},
		{"/v1/buyer/cart", http.MethodPut, cartRoute, true},
		{"/v1/buyer/cart", http.MethodOptions, cartRoute, false},
		{"/v1/buyer/quotes", http.MethodPost, quotesRoute, true},
		{"/v1/buyer/quotes/00000000-0000-0000-0000-000000000001", http.MethodGet, quoteRoute, true},
		{"/v1/buyer/destination", http.MethodPut, destinationRoute, true},
		{"/v1/buyer/destination", http.MethodGet, destinationRoute, true},
		{"/v1/buyer/destinations/00000000-0000-0000-0000-000000000001", http.MethodGet, destinationItemRoute, true},
		{"/v1/buyer/checkout", http.MethodPost, checkoutRoute, true},
		{"/v1/buyer/orders/00000000-0000-0000-0000-000000000001", http.MethodGet, orderRoute, true},
		{"/v1/buyer/session/", http.MethodGet, unknownRoute, false},
		{"/v1/buyer/quotes/a/b", http.MethodGet, unknownRoute, false},
		{"/v1/buyer/other", http.MethodGet, unknownRoute, false},
	} {
		found := matchRoute(tc.path)
		if found.kind != tc.kind || allowed(found.kind, tc.method) != tc.ok {
			t.Fatalf("route %s %s: got kind=%d allowed=%v", tc.method, tc.path, found.kind, allowed(found.kind, tc.method))
		}
	}
}

func TestCanonicalHeadersAndForbiddenScope(t *testing.T) {
	if !canonicalSecret(testBuyerKey) || canonicalSecret(testBuyerKey+"=") || canonicalSecret(strings.Repeat("a", 43)) {
		t.Fatal("canonical raw base64url 32-byte grammar drift")
	}
	if base64.RawURLEncoding.EncodeToString(make([]byte, 32)) != testBuyerKey {
		t.Fatal("fixture must be canonical")
	}
	r := httptest.NewRequest(http.MethodGet, "http://internal/v1/buyer/cart", nil)
	r.Header.Set("Authorization", "Bearer "+testBuyerKey)
	if token, ok := bearer(r); !ok || token != testBuyerKey {
		t.Fatal("canonical buyer bearer rejected")
	}
	r.Header.Add("Authorization", "Bearer "+testBuyerKey)
	if _, ok := bearer(r); ok {
		t.Fatal("duplicate bearer accepted")
	}
	r.Header.Del("Authorization")
	for _, value := range []string{"Bearer " + testBuyerKey + " ", "bearer " + testBuyerKey, "Bearer merchant-token"} {
		r.Header.Set("Authorization", value)
		if _, ok := bearer(r); ok {
			t.Fatalf("bad bearer accepted: %q", value)
		}
	}
	r.Header.Del("Authorization")
	for _, header := range []string{"Cookie", "Origin", "X-Tenant-ID", "X-Store-ID"} {
		r.Header.Set(header, "")
		if !forbiddenInput(r) {
			t.Fatalf("%s must be forbidden even when empty", header)
		}
		r.Header.Del(header)
	}
	r.URL.ForceQuery = true
	if !forbiddenInput(r) {
		t.Fatal("bare question mark accepted")
	}
	r.URL.ForceQuery = false
	r.URL.RawQuery = "x=1"
	if !forbiddenInput(r) {
		t.Fatal("query accepted")
	}
	r.URL.RawQuery = ""
	if forbiddenInput(r) {
		t.Fatal("clean request forbidden")
	}
	r.Header.Set("Idempotency-Key", "validkey1")
	if _, ok := keyFor(r, false, false); ok {
		t.Fatal("read accepted replay key")
	}
	if key, ok := keyFor(r, false, true); !ok || key != "validkey1" {
		t.Fatal("write rejected key")
	}
	if _, ok := keyFor(r, true, false); ok {
		t.Fatal("session accepted replay key")
	}
	r.Header.Add("Idempotency-Key", "validkey1")
	if _, ok := keyFor(r, false, true); ok {
		t.Fatal("duplicate replay key accepted")
	}
}

func TestStrictJSONAndBody(t *testing.T) {
	type input struct {
		Name  string `json:"name"`
		Inner struct {
			Count int `json:"count"`
		} `json:"inner"`
		Items []struct {
			Code string `json:"code"`
		} `json:"items"`
	}
	for _, tc := range []struct {
		body   string
		status int
	}{
		{`{"name":"ok","inner":{"count":1},"items":[{"code":"a"}]}`, 0},
		{`{"name":"ok"}`, 0},
		{`null`, 400},
		{`{"name":null}`, 400},
		{`{"name":null,"name":"later"}`, 400},
		{`{"inner":null}`, 400},
		{`{"inner":{"count":null}}`, 400},
		{`{"items":[null]}`, 400},
		{`{"items":[{"code":null}]}`, 400},
		{`{"inner":{"extra":1}}`, 400},
		{`{"items":[{"extra":1}]}`, 400},
		{`{"name":"ok"} {}`, 400},
		{`[]`, 400},
		{strings.Repeat(" ", maxJSON+1), 422},
	} {
		r := httptest.NewRequest(http.MethodPut, "http://internal/v1/buyer/cart", strings.NewReader(tc.body))
		r.Header.Set("Content-Type", "application/json; charset=utf-8")
		var got input
		err := decodeJSON(r, &got)
		if tc.status == 0 && err != nil {
			t.Fatalf("valid %q: %v", tc.body, err)
		}
		if tc.status != 0 {
			status, _ := classify(err)
			if status != tc.status {
				t.Fatalf("%q: status %d, want %d", tc.body, status, tc.status)
			}
		}
	}
	r := httptest.NewRequest(http.MethodPut, "http://internal/v1/buyer/cart", strings.NewReader(`{}`))
	if status, _ := classify(decodeJSON(r, &input{})); status != 415 {
		t.Fatalf("missing content type: %d", status)
	}
	for _, body := range []string{" ", "{}"} {
		r = httptest.NewRequest(http.MethodGet, "http://internal/v1/buyer/cart", strings.NewReader(body))
		if status, _ := classify(noBody(r)); status != 422 {
			t.Fatalf("GET accepted body %q", body)
		}
	}
	r = httptest.NewRequest(http.MethodGet, "http://internal/v1/buyer/cart", nil)
	if err := noBody(r); err != nil {
		t.Fatal(err)
	}
}

func TestSessionEarlyErrorsNeverRetryable(t *testing.T) {
	h := httperror.Middleware(&handler{bffKey: testBuyerKey})
	for _, tc := range []struct {
		name   string
		edit   func(*http.Request)
		status int
	}{
		{"missing key", func(*http.Request) {}, 401},
		{"missing origin", func(r *http.Request) {
			r.Header.Set("X-Commerce-Buyer-BFF-Key", testBuyerKey)
			r.Header.Del("X-Commerce-Storefront-Origin")
		}, 422},
		{"bad key", func(r *http.Request) { r.Header.Set("X-Commerce-Buyer-BFF-Key", strings.Repeat("B", 43)) }, 401},
		{"duplicate key", func(r *http.Request) {
			r.Header.Add("X-Commerce-Buyer-BFF-Key", testBuyerKey)
			r.Header.Add("X-Commerce-Buyer-BFF-Key", testBuyerKey)
		}, 401},
		{"scope cookie", func(r *http.Request) {
			r.Header.Set("X-Commerce-Buyer-BFF-Key", testBuyerKey)
			r.Header.Set("Cookie", "x=y")
		}, 403},
		{"authorization", func(r *http.Request) {
			r.Header.Set("X-Commerce-Buyer-BFF-Key", testBuyerKey)
			r.Header.Set("Authorization", "Bearer "+testBuyerKey)
		}, 401},
		{"replay key", func(r *http.Request) {
			r.Header.Set("X-Commerce-Buyer-BFF-Key", testBuyerKey)
			r.Header.Set("Idempotency-Key", "validkey1")
		}, 422},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "http://internal/v1/buyer/session", strings.NewReader(`{}`))
			r.Header.Set("X-Commerce-Storefront-Origin", "https://shop.example")
			tc.edit(r)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("status=%d, want %d", w.Code, tc.status)
			}
			var envelope struct {
				Retryable bool   `json:"retryable"`
				RequestID string `json:"request_id"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil || envelope.Retryable || envelope.RequestID == "" {
				t.Fatalf("unsafe issue failure: %s", w.Body.String())
			}
			if w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Fatal("missing transport safety headers")
			}
		})
	}
}

func TestTransportUnknownAndUnsupportedStayJSON(t *testing.T) {
	h := httperror.Middleware(&handler{bffKey: testBuyerKey})
	for _, tc := range []struct {
		method, path string
		status       int
		code         string
	}{
		{http.MethodGet, "/v1/buyer/unknown", 404, "not_found"},
		{http.MethodHead, "/v1/buyer/cart", 405, "method_not_allowed"},
		{http.MethodOptions, "/v1/buyer/checkout", 405, "method_not_allowed"},
		{http.MethodGet, "/v1/buyer/quotes/not-a-uuid", 422, "invalid_request"},
		{http.MethodGet, "/v1/buyer/cart/", 404, "not_found"},
		{http.MethodGet, "/v1/buyer/%63art", 404, "not_found"},
	} {
		r := httptest.NewRequest(tc.method, "http://internal"+tc.path, nil)
		r.Header.Set("X-Commerce-Buyer-BFF-Key", testBuyerKey)
		r.Header.Set("X-Commerce-Storefront-Origin", "https://shop.example")
		r.Header.Set("Authorization", "Bearer "+testBuyerKey)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		var got httperror.Envelope
		if w.Code != tc.status || json.Unmarshal(w.Body.Bytes(), &got) != nil || got.Code != tc.code || w.Header().Get("Location") != "" {
			t.Fatalf("%s %s: status=%d body=%s", tc.method, tc.path, w.Code, w.Body.String())
		}
	}
}

func TestErrorClassification(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{buyer.ErrUnauthorized, 401, "unauthorized"},
		{domains.ErrUnavailable, 404, "not_found"},
		{command.ErrNotFound, 404, "not_found"},
		{command.ErrConflict, 409, "conflict"},
		{command.ErrInsufficient, 409, "insufficient_inventory"},
		{errors.New("raw database secret"), 503, "unavailable"},
	} {
		status, code := classify(tc.err)
		if status != tc.status || code != tc.code {
			t.Fatalf("%v: got %d/%s", tc.err, status, code)
		}
	}
}
