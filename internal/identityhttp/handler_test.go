package identityhttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"livecommerce/internal/identity"
)

var testSecret = strings.Repeat("A", 43)

func TestIdentityHandleSuggestRemoved(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodGet} {
		s := &fakeService{}
		w := privateRequest(t, s, method, "/v1/identity/handle-suggest", `{"store_name":"New shop"}`, nil)
		if w.Code != http.StatusNotFound && w.Code != http.StatusMethodNotAllowed {
			t.Fatalf("retired %s suggestion route returned %d", method, w.Code)
		}
		if s.calls != 0 {
			t.Fatal("retired route called identity service")
		}
	}
}

type fakeService struct {
	calls      int
	err        error
	token, key string
	request    identity.StoreRequest
}

func (s *fakeService) Start(ctx context.Context) (identity.Flow, error) {
	s.calls++
	return identity.Flow{URL: "https://idp.example/authorize", Binding: testSecret, ExpiresAt: time.Now().Add(time.Minute)}, s.err
}
func (s *fakeService) Complete(ctx context.Context, state, binding, code string) (identity.Session, error) {
	s.calls++
	return identity.Session{Token: testSecret, ExpiresAt: time.Now().Add(time.Hour)}, s.err
}
func (s *fakeService) CreateInitialStore(ctx context.Context, token, key string, in identity.StoreRequest) (identity.Store, error) {
	s.calls++
	s.token = token
	s.key = key
	s.request = in
	return identity.Store{TenantID: "tenant", StoreID: "store", WarehouseID: "warehouse"}, s.err
}
func (s *fakeService) Logout(ctx context.Context, token string) error {
	s.calls++
	s.token = token
	return s.err
}

func privateRequest(t *testing.T, s *fakeService, method, path, bodyText string, mutate func(*http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	h, err := NewHandler(s, testSecret)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(method, path, strings.NewReader(bodyText))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Commerce-BFF-Key", testSecret)
	if mutate != nil {
		mutate(r)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestPrivateIdentityRejectsTransportBeforeService(t *testing.T) {
	for _, tc := range []struct {
		name, method, path, body string
		status                   int
		mutate                   func(*http.Request)
	}{
		{"missing_key", "POST", "/v1/identity/login/start", "{}", 401, func(r *http.Request) { r.Header.Del("X-Commerce-BFF-Key") }},
		{"wrong_key", "POST", "/v1/identity/login/start", "{}", 401, func(r *http.Request) { r.Header.Set("X-Commerce-BFF-Key", "wrong") }},
		{"duplicate_key", "POST", "/v1/identity/login/start", "{}", 401, func(r *http.Request) { r.Header.Add("X-Commerce-BFF-Key", testSecret) }},
		{"browser_origin", "POST", "/v1/identity/login/start", "{}", 403, func(r *http.Request) { r.Header.Set("Origin", "https://merchant.example") }},
		{"browser_cookie", "POST", "/v1/identity/login/start", "{}", 403, func(r *http.Request) { r.Header.Set("Cookie", "a=b") }},
		{"get_login", "GET", "/v1/identity/login/start", "", 405, nil},
		{"query", "POST", "/v1/identity/login/start?provider=attacker", "{}", 422, nil},
		{"unknown_route", "POST", "/v1/identity/unknown", "{}", 404, nil},
		{"null", "POST", "/v1/identity/login/start", "null", 400, nil},
		{"extra_authority", "POST", "/v1/identity/login/start", `{"principal_id":"attacker"}`, 400, nil},
		{"trailing", "POST", "/v1/identity/login/start", "{}{}", 400, nil},
		{"oversized", "POST", "/v1/identity/login/start", strings.Repeat(" ", 65537) + "{}", 400, nil},
		{"mime", "POST", "/v1/identity/login/start", "{}", 415, func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") }},
		{"missing_bearer", "POST", "/v1/identity/logout", "{}", 401, nil},
		{"duplicate_bearer", "POST", "/v1/identity/logout", "{}", 401, func(r *http.Request) {
			r.Header.Add("Authorization", "Bearer "+testSecret)
			r.Header.Add("Authorization", "Bearer "+testSecret)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &fakeService{}
			w := privateRequest(t, s, tc.method, tc.path, tc.body, tc.mutate)
			if w.Code != tc.status || s.calls != 0 {
				t.Fatalf("status=%d calls=%d", w.Code, s.calls)
			}
			assertSafe(t, w)
		})
	}
}

func assertSafe(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	if w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Access-Control-Allow-Origin") != "" || len(w.Header().Get("X-Request-ID")) != 32 {
		t.Fatal("unsafe response headers")
	}
	if strings.Contains(w.Body.String(), testSecret) {
		t.Fatal("secret leaked in error")
	}
	var body map[string]any
	if json.Unmarshal(w.Body.Bytes(), &body) != nil || body["code"] == nil {
		t.Fatal("missing sanitized error envelope")
	}
}

func TestPrivateIdentityExplicitSuccessProjection(t *testing.T) {
	s := &fakeService{}
	w := privateRequest(t, s, "POST", "/v1/identity/login/start", "{}", nil)
	var flow map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &flow)
	if w.Code != 200 || len(flow) != 3 || flow["binding"] != testSecret || flow["authorization_url"] == nil {
		t.Fatal("start DTO mismatch")
	}
	w = privateRequest(t, s, "POST", "/v1/identity/login/complete", `{"state":"state","binding":"binding","code":"code"}`, nil)
	var session map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &session)
	if w.Code != 200 || len(session) != 2 || session["token"] != testSecret || session["expires_at"] == nil {
		t.Fatal("complete DTO mismatch")
	}
	w = privateRequest(t, s, "POST", "/v1/identity/initial-store", `{"tenant_name":"Test","store_name":"Shop","warehouse_name":"Warehouse","currency":"TWD"}`, func(r *http.Request) {
		r.Header.Set("Authorization", "Bearer "+testSecret)
		r.Header.Set("Idempotency-Key", "create-store-1")
	})
	if w.Code != 200 || s.token != testSecret || s.key != "create-store-1" || s.request.Currency != "TWD" {
		t.Fatal("initial store request mismatch")
	}
	w = privateRequest(t, s, "POST", "/v1/identity/logout", "{}", func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+testSecret) })
	if w.Code != 204 || w.Body.Len() != 0 || s.calls != 4 {
		t.Fatal("logout did not commit before response")
	}
}

func TestPrivateIdentitySanitizesServiceFailures(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{{identity.ErrInvalid, 422}, {identity.ErrUnauthorized, 401}, {identity.ErrDisabled, 403}, {identity.ErrConflict, 409}, {identity.ErrUnavailable, 503}, {errors.New("synthetic provider secret " + testSecret), 503}} {
		s := &fakeService{err: tc.err}
		w := privateRequest(t, s, "POST", "/v1/identity/login/start", "{}", nil)
		if w.Code != tc.status {
			t.Fatal("incorrect service error mapping")
		}
		assertSafe(t, w)
	}
	if _, err := NewHandler(nil, testSecret); err == nil {
		t.Fatal("nil service accepted")
	}
	if _, err := NewHandler(&fakeService{}, "weak"); err == nil {
		t.Fatal("invalid transport key accepted")
	}
}
