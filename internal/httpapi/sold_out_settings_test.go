// Purpose: DB-free full-router collision and strict transport gates for sold-out settings.
// Depends on: httpapi.NewHandler and existing claims request test helpers.
// Used by: go test ./internal/httpapi -run TestLiveSettingsHTTP.
package httpapi

import (
	"livecommerce/internal/msgtemplates"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLiveSettingsHTTPFullRouterTransport(t *testing.T) {
	labelsHandler := claimsHandler(t) // NewHandler builds all normal routes, including claim routes.
	base := "/v1/admin/stores/" + claimsStore + "/live-settings/sold-out-reply"
	body := `{"enabled":false,"template_id":"merchant-sold-out","template_version":1,"expected_version":0}`
	for _, tc := range []struct {
		name, method, path, body string
		edit                     func(*http.Request)
		status                   int
	}{
		{"no bearer", "GET", base, "", func(r *http.Request) { r.Header.Del("Authorization") }, 401},
		{"query", "GET", base + "?tenant_id=other", "", nil, 422},
		{"empty query", "GET", base + "?", "", nil, 422},
		{"GET body", "GET", base, `{}`, nil, 422},
		{"GET key", "GET", base, "", func(r *http.Request) { r.Header.Set("Idempotency-Key", "live-settings-0001") }, 422},
		{"HEAD", "HEAD", base, "", nil, 405},
		{"POST", "POST", base, body, nil, 405},
		{"missing key", "PUT", base, body, func(r *http.Request) { r.Header.Del("Idempotency-Key") }, 422},
		{"duplicate key", "PUT", base, body, func(r *http.Request) { r.Header.Add("Idempotency-Key", "second-key-0001") }, 422},
		{"unknown field", "PUT", base, `{"enabled":false,"template_id":"x","template_version":1,"expected_version":0,"tenant_id":"x"}`, nil, 400},
		{"null boolean", "PUT", base, `{"enabled":null,"template_id":"x","template_version":1,"expected_version":0}`, nil, 400},
		// W3-U2 ruling: preserve claimsBody rejection; top-level null is 422, field null is 400.
		{"top null", "PUT", base, `null`, nil, 422},
		{"missing boolean", "PUT", base, `{"template_id":"x","template_version":1,"expected_version":0}`, nil, 422},
		{"duplicate field", "PUT", base, `{"enabled":true,"enabled":false,"template_id":"x","template_version":1,"expected_version":0}`, nil, 400},
		{"negative CAS", "PUT", base, `{"enabled":false,"template_id":"x","template_version":1,"expected_version":-1}`, nil, 422},
		{"zero template version", "PUT", base, `{"enabled":false,"template_id":"x","template_version":0,"expected_version":0}`, nil, 422},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := claimsCall(labelsHandler, tc.method, tc.path, tc.body, tc.edit)
			if w.Code != tc.status {
				t.Fatalf("want %d got %d %s", tc.status, w.Code, w.Body)
			}
			if w.Header().Get("Cache-Control") != "private, no-store" {
				t.Fatal("private boundary missing")
			}
		})
	}
	// Disabled Claims leaves this surface absent even when the template service exists.
	w := httptest.NewRecorder()
	NewHandler(nil, Options{Studio: true, MsgTemplates: msgtemplates.NewService()}).ServeHTTP(w, httptest.NewRequest("GET", base, nil))
	if w.Code != 404 {
		t.Fatalf("mounted without Claims: %d", w.Code)
	}
}
