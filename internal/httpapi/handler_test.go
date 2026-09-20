package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

func TestClassifyDoesNotExposeDriverDetails(t *testing.T) {
	for _, tt := range []struct {
		err    error
		status int
		code   string
	}{
		{platform.ErrUnauthorized, 401, "unauthorized"}, {command.ErrInvalid, 422, "invalid_request"},
		{platform.ErrScopeNotFound, 404, "not_found"}, {platform.ErrForbidden, 403, "forbidden"},
		{command.ErrNotFound, 404, "not_found"}, {command.ErrConflict, 409, "conflict"},
		{command.ErrInsufficient, 409, "insufficient_inventory"}, {context.DeadlineExceeded, 503, "retry_later"},
		{&pgconn.PgError{Code: "23505", Message: "sensitive sku value"}, 409, "conflict"},
		{&pgconn.PgError{Code: "55P03", Message: "sensitive query"}, 503, "retry_later"},
		{errors.New("sensitive connection string"), 500, "internal"},
	} {
		status, code := classify(tt.err)
		if status != tt.status || code != tt.code {
			t.Fatalf("got %d %s", status, code)
		}
	}
}

func TestPageQueryValidation(t *testing.T) {
	for _, raw := range []string{"limit=0", "limit=-1", "limit=101", "limit=+1", "limit=1.0", "limit=1&limit=2", "unknown=x", "cursor=a&cursor=b", "limit=%zz", "cursor=" + strings.Repeat("a", 1025)} {
		if _, err := parsePage(raw); !errors.Is(err, command.ErrInvalid) {
			t.Fatalf("accepted bad page query %q", raw)
		}
	}
	got, err := parsePage("limit=20&cursor=opaque")
	if err != nil || got.Limit != 20 || got.Cursor != "opaque" {
		t.Fatalf("valid page %+v %v", got, err)
	}
}

func TestAdminHTTPTransportGuards(t *testing.T) {
	h := NewHandler(nil)
	const path = "/v1/admin/stores/11111111-1111-4111-8111-111111111111/products"
	for _, tt := range []struct {
		name, method, body, content, auth string
		want                              int
	}{
		{"no auth", "GET", "", "", "", 401},
		{"cookie is not login", "GET", "", "", "", 401},
		{"spaces in bearer", "GET", "", "", "Bearer abc def", 401},
		{"wrong content type", "POST", `{"name":"a"}`, "text/plain", "", 415},
		{"unknown tenant field", "POST", `{"name":"a","tenant_id":"x"}`, "application/json", "", 400},
		{"trailing json", "POST", `{"name":"a"} {}`, "application/json", "", 400},
		{"oversize", "POST", `{"name":"` + strings.Repeat("a", 65536) + `"}`, "application/json", "", 400},
		{"bad json", "POST", `{"name":`, "application/json", "", 400},
		{"valid payload needs login", "POST", `{"name":"a"}`, "application/json", "", 401},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(tt.method, path, strings.NewReader(tt.body))
			r.Header.Set("Content-Type", tt.content)
			r.Header.Set("Authorization", tt.auth)
			r.Header.Set("Cookie", "tenant_id=forged;session=not-authority")
			r.Header.Set("X-Tenant-ID", "forged")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tt.want {
				t.Fatalf("status %d body %s", w.Code, w.Body.String())
			}
			if w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Fatal("missing response protection")
			}
		})
	}
}

func TestNoPublicReserveOrGETMutation(t *testing.T) {
	h := NewHandler(nil)
	for _, path := range []string{"/v1/public/reservations", "/v1/admin/stores/11111111-1111-4111-8111-111111111111/inventory/reserve"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != 404 {
			t.Fatalf("exposed mutation route %s", path)
		}
	}
}
