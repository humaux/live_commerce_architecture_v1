// Purpose: DB-free tests of the PAYUNi activation routes (w4-02b): they mount only with a service, the full router builds without a
//   route conflict next to the settings/accounts routes, the strict request rules answer before any database use, and the new
//   refusals reach the JSON body with their stable codes. Non-goals: the money path (tests/foundation/payuni_activation_test.go).
// Depends on: internal/payments (NewActivation), internal/integrations/accounts (NewKeyring), net/http/httptest.
// Used by: go test ./internal/httpapi.
// Status: MOCK.

package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"livecommerce/internal/integrations/accounts"
	"livecommerce/internal/payments"
)

func testActivation(t *testing.T) *payments.Activation {
	t.Helper()
	keys, err := accounts.NewKeyring("k1", map[string][]byte{"k1": bytes.Repeat([]byte{1}, 32)}, bytes.Repeat([]byte{2}, 32))
	if err != nil {
		t.Fatal(err)
	}
	svc, err := payments.NewActivation(keys, payments.ActivationConfig{Profile: "PROVIDER_MOCK",
		NotifyBaseURL: "https://hooks.example.com", ReturnURL: "https://admin.example.com/settings/payments"})
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

func activationCall(h http.Handler, method, path, contentType, body string, bearer bool) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if contentType != "" {
		r.Header.Set("Content-Type", contentType)
	}
	if bearer {
		r.Header.Set("Authorization", "Bearer "+strings.Repeat("a", 43))
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

const (
	pa = "/v1/admin/stores/11111111-1111-4111-8111-111111111111/payments/payuni"
	pc = `{"connection_id":"22222222-2222-4222-8222-222222222222"}`
)

// The full router (settings + accounts + activation) must build: a route conflict panics in ServeMux registration.
func TestPaymentActivationRoutesMountOnlyWithService(t *testing.T) {
	routes := []struct{ method, path string }{
		{"POST", pa + "/verify"}, {"POST", pa + "/verify/33333333-3333-4333-8333-333333333333/check"},
		{"POST", pa + "/live-probe"}, {"GET", pa + "/connections/22222222-2222-4222-8222-222222222222/status"},
	}
	on := NewHandler(nil, Options{PayuniActivation: testActivation(t)})
	off := NewHandler(nil, Options{})
	for _, r := range routes {
		if w := activationCall(on, r.method, r.path, "application/json", pc, false); w.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s with service: status=%d, want 401 (mounted, auth first)", r.method, r.path, w.Code)
		}
		if w := activationCall(off, r.method, r.path, "application/json", pc, false); w.Code != http.StatusNotFound && w.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s %s without service: status=%d, want unmounted", r.method, r.path, w.Code)
		}
	}
}

func TestPaymentActivationPooledRoutesAreStrict(t *testing.T) {
	h := NewHandler(nil, Options{PayuniActivation: testActivation(t)})
	path := pa + "/live-probe"
	for name, c := range map[string]struct {
		contentType, body, query string
		bearer                   bool
		status                   int
		code                     string
	}{
		"no bearer":      {"application/json", pc, "", false, 401, "unauthorized"},
		"not json":       {"text/plain", pc, "", true, 415, "json_required"},
		"unknown field":  {"application/json", `{"connection_id":"x","store_id":"y"}`, "", true, 400, "invalid_json"},
		"trailing value": {"application/json", pc + `{}`, "", true, 400, "invalid_json"},
		"query string":   {"application/json", pc, "?x=1", true, 422, "invalid_request"},
		// valid JSON, malformed id: refused by the service before any database use (a nil pool would panic otherwise)
		"bad connection id": {"application/json", `{"connection_id":"nope"}`, "", true, 422, "invalid_request"},
	} {
		w := activationCall(h, "POST", path+c.query, c.contentType, c.body, c.bearer)
		var env struct{ Code string }
		_ = json.Unmarshal(w.Body.Bytes(), &env)
		if w.Code != c.status || env.Code != c.code {
			t.Errorf("%s: status=%d code=%q, want %d %q", name, w.Code, env.Code, c.status, c.code)
		}
	}
}

func TestActivationRefusalsReachTheJSONBody(t *testing.T) {
	for err, want := range map[error][2]any{
		payments.ErrNotQualified:      {409, "not_qualified"},
		payments.ErrPlatformDisabled:  {409, "platform_disabled"},
		payments.ErrProfileNotAllowed: {409, "profile_not_allowed"},
		payments.ErrProbeFailed:       {422, "payuni_probe_failed"},
	} {
		status, code := activationClassify(err)
		w := httptest.NewRecorder()
		respondError(w, status, code)
		var env struct{ Code, Message string }
		_ = json.Unmarshal(w.Body.Bytes(), &env)
		if status != want[0] || env.Code != want[1] || env.Message == "" || w.Code != want[0] {
			t.Errorf("%v: status=%d body=%s, want %v", err, status, w.Body.String(), want)
		}
	}
}
