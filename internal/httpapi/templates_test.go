package httpapi

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"livecommerce/internal/command"
	"livecommerce/internal/msgtemplates"
)

func TestTemplatesClassify(t *testing.T) {
	cases := []struct {
		code   string
		status int
		body   string
	}{
		{"PT403", http.StatusForbidden, "forbidden"},
		{"PT404", http.StatusNotFound, "not_found"},
		{"PT409", http.StatusConflict, "template_fixed"},
		{"PT422", http.StatusUnprocessableEntity, "invalid_request"},
	}
	for _, c := range cases {
		status, body := templatesClassify(&pgconn.PgError{Code: c.code})
		if status != c.status || body != c.body {
			t.Errorf("%s: want %d %q, got %d %q", c.code, c.status, c.body, status, body)
		}
	}
	if status, body := templatesClassify(msgtemplates.PublicUnsafe()); status != http.StatusUnprocessableEntity || body != "public_reply_forbidden_content" {
		t.Errorf("public unsafe: want 422 public_reply_forbidden_content, got %d %q", status, body)
	}
	if status, body := templatesClassify(command.ErrNotFound); status != http.StatusNotFound || body != "not_found" {
		t.Errorf("not found: want 404 not_found, got %d %q", status, body)
	}
	if status, body := templatesClassify(command.ErrConflict); status != http.StatusConflict || body != "conflict" {
		t.Errorf("conflict: want 409 conflict, got %d %q", status, body)
	}
	if status, body := templatesClassify(errors.New("boom")); status != http.StatusInternalServerError || body != "internal" {
		t.Errorf("unknown: want 500 internal, got %d %q", status, body)
	}
}

// TestTemplateRoutesMountWhenServicePresent builds the whole router twice (the route-conflict surface of NewHandler)
// and asserts the message-templates routes are absent with a nil service and present (401, not 404, without a bearer)
// with a non-nil one. No database is touched: route registration only, so a nil pool is safe here.
func TestTemplateRoutesMountWhenServicePresent(t *testing.T) {
	t.Run("nil service unmounted", func(t *testing.T) {
		h := NewHandler(nil)
		res := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v1/admin/stores/"+testUUID+"/message-templates", nil)
		h.ServeHTTP(res, req)
		if res.Code != http.StatusNotFound {
			t.Fatalf("unmounted route status=%d body=%s, want 404", res.Code, res.Body.String())
		}
	})

	t.Run("non-nil service mounted", func(t *testing.T) {
		h := NewHandler(nil, Options{MsgTemplates: msgtemplates.NewService()})
		res := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v1/admin/stores/"+testUUID+"/message-templates", nil)
		h.ServeHTTP(res, req)
		if res.Code != http.StatusUnauthorized {
			t.Fatalf("mounted route status=%d body=%s, want 401", res.Code, res.Body.String())
		}
		if got := res.Header().Get("Cache-Control"); got != "private, no-store" {
			t.Fatalf("Cache-Control=%q, want private, no-store", got)
		}
	})
}

// TestTemplateRouteTransportRules pins the receipt/decoder boundary without a database: the Idempotency-Key grammar is
// enforced before any transaction (422), a valid key reaches the strict body decoder (415 without a media type), and a
// fully-formed write reaches the bearer gate (401, proving the pipeline reaches scope without touching PG).
func TestTemplateRouteTransportRules(t *testing.T) {
	h := NewHandler(nil, Options{MsgTemplates: msgtemplates.NewService()})
	base := "/v1/admin/stores/" + testUUID + "/message-templates"
	const key = "12345678-abcdef-0001"

	do := func(method, path string, body string, headers map[string]string) *httptest.ResponseRecorder {
		t.Helper()
		var rd io.Reader
		if body != "" {
			rd = strings.NewReader(body)
		}
		req := httptest.NewRequest(method, path, rd)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		res := httptest.NewRecorder()
		h.ServeHTTP(res, req)
		return res
	}

	// GET with an Idempotency-Key (even empty) is refused on the read.
	if res := do(http.MethodGet, base, "", map[string]string{"Idempotency-Key": key}); res.Code != http.StatusUnprocessableEntity {
		t.Fatalf("GET with key: want 422, got %d", res.Code)
	}
	// GET without a key reaches the bearer gate.
	if res := do(http.MethodGet, base, "", nil); res.Code != http.StatusUnauthorized {
		t.Fatalf("GET without key: want 401, got %d", res.Code)
	}

	// POST without a key is refused before any transaction.
	if res := do(http.MethodPost, base, `{"template_id":"a","name":"n","kinds":["dm"],"public_safe":false,"body":"b"}`, nil); res.Code != http.StatusUnprocessableEntity {
		t.Fatalf("POST without key: want 422, got %d", res.Code)
	}
	// POST with a malformed key is refused.
	if res := do(http.MethodPost, base, `{"template_id":"a","name":"n","kinds":["dm"],"public_safe":false,"body":"b"}`, map[string]string{"Idempotency-Key": "short"}); res.Code != http.StatusUnprocessableEntity {
		t.Fatalf("POST with bad key: want 422, got %d", res.Code)
	}
	// A valid key reaches the strict body decoder: no media type -> 415 json_required.
	if res := do(http.MethodPost, base, "", map[string]string{"Idempotency-Key": key}); res.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("POST valid key no body: want 415, got %d", res.Code)
	}
	// A fully-formed write reaches the bearer gate: 401 without a token (no database touched).
	full := `{"template_id":"welcome-msg","name":"歡迎","kinds":["dm"],"public_safe":false,"body":"嗨，歡迎光臨"}`
	if res := do(http.MethodPost, base, full, map[string]string{"Idempotency-Key": key, "Content-Type": "application/json"}); res.Code != http.StatusUnauthorized {
		t.Fatalf("POST full body: want 401, got %d body=%s", res.Code, res.Body.String())
	}
	// public_safe:null must be a 400 invalid_json (null never decodes to a zero-value false), not a silent publish.
	nullSafe := `{"template_id":"welcome-msg","name":"歡迎","kinds":["public_reply"],"public_safe":null,"body":"嗨，歡迎光臨"}`
	if res := do(http.MethodPost, base, nullSafe, map[string]string{"Idempotency-Key": key, "Content-Type": "application/json"}); res.Code != http.StatusBadRequest {
		t.Fatalf("public_safe null: want 400, got %d", res.Code)
	}
}
