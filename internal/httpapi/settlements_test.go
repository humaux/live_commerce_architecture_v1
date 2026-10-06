// settlements_test.go: DB-free router tests of the merchant settlement routes (contract §6.5, PF13 transport half).
// Purpose: NewHandler builds with the two GET routes next to every other route (a conflict panics at build), the transport gate answers
// before any database work, and no query or id reaches SQL unvalidated.
// Depends on: NewHandler, registerSettlementRoutes, settlementQuery. Callers: go test ./internal/httpapi.

package httpapi

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

const slStore = "11111111-2222-4333-8444-555555555555"
const slStatement = "99999999-2222-4333-8444-555555555555"
const slToken = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFG" // canonical bearer shape; never reaches a database here

func TestSettlementRoutesRouterBuildAndTransportGate(t *testing.T) {
	h := NewHandler(nil, Options{}) // panics on a route conflict
	base := "/v1/admin/stores/" + slStore + "/settlements"
	for _, c := range []struct {
		method, path, token string
		want                int
	}{
		{http.MethodGet, base, "", 401},
		{http.MethodGet, base + "/" + slStatement, "", 401},
		{http.MethodPost, base, slToken, 405},
		{http.MethodPut, base + "/" + slStatement, slToken, 405},
		{http.MethodDelete, base, slToken, 405},
		{http.MethodGet, base + "?limit=0", slToken, 422},
		{http.MethodGet, base + "?limit=53", slToken, 422},
		{http.MethodGet, base + "?before=2026-9-1", slToken, 422},
		{http.MethodGet, base + "?before=2026-09-21&before=2026-09-14", slToken, 422},
		{http.MethodGet, base + "?cursor=abc", slToken, 422},
		{http.MethodGet, base + "?", slToken, 422},
		{http.MethodGet, base + "/" + slStatement + "?limit=1", slToken, 422},
		{http.MethodGet, base + "/not-a-uuid", slToken, 422},
		{http.MethodGet, "/v1/admin/stores/not-a-uuid/settlements", slToken, 422},
	} {
		req := httptest.NewRequest(c.method, c.path, nil)
		if c.token != "" {
			req.Header.Set("Authorization", "Bearer "+c.token)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != c.want {
			t.Errorf("%s %s: got %d want %d", c.method, c.path, w.Code, c.want)
		}
		if cc := w.Header().Get("Cache-Control"); !strings.Contains(cc, "no-store") {
			t.Errorf("%s %s: Cache-Control %q", c.method, c.path, cc)
		}
	}
	// a body or an Idempotency-Key on a GET is refused too
	req := httptest.NewRequest(http.MethodGet, base, strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer "+slToken)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 422 {
		t.Errorf("GET with a body: %d", w.Code)
	}
}

func TestSettlementQueryParser(t *testing.T) {
	for raw, want := range map[string]struct {
		before string
		limit  int
		ok     bool
	}{
		"":                              {"", 52, true},
		"limit=10":                      {"", 10, true},
		"before=2026-09-21":             {"2026-09-21", 52, true},
		"before=2026-09-21&limit=52":    {"2026-09-21", 52, true},
		"limit=52&before=2026-09-21":    {"2026-09-21", 52, true},
		"limit=1":                       {"", 1, true},
		"limit=":                        {"", 0, false},
		"limit=0":                       {"", 0, false},
		"limit=-1":                      {"", 0, false},
		"limit=1.5":                     {"", 0, false},
		"limit=1&limit=2":               {"", 0, false},
		"before=20260921":               {"", 0, false},
		"before=2026-09-21%20":          {"", 0, false},
		"x=1":                           {"", 0, false},
		"limit=1&x=1":                   {"", 0, false},
		strings.Repeat("limit=1&", 100): {"", 0, false},
	} {
		got := struct {
			before string
			limit  int
			ok     bool
		}{}
		got.before, got.limit, got.ok = settlementQuery(&url.URL{RawQuery: raw})
		if got != want {
			t.Errorf("%q: %+v want %+v", raw, got, want)
		}
	}
}
