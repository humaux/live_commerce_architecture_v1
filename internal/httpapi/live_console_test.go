// Purpose: DB-free tests of the LC-B7 routes (A1 console GET, bounded stock edit): the full NewHandler router (a ServeMux pattern conflict panics here, not at first use), the transport rules that hold before any transaction, and the below_reserved classification. With a nil pool a request that passes every transport rule reaches platform.WithScope and answers 401, the "admitted" marker. Real-PG behaviour: tests/foundation/live_console_read_test.go.
// Depends on: NewHandler, claimsCall/claimsStore/claimsSession (claims_test.go), inventoryAdjustClassify.
// Used by: go test ./internal/httpapi (--live-console mode).
package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"

	"livecommerce/internal/httperror"
	"livecommerce/internal/inventory"
)

func TestLiveConsoleRouteOnFullRouter(t *testing.T) {
	h := NewHandler(nil, Options{Studio: true})
	path := "/v1/admin/stores/" + claimsStore + "/live-sessions/" + claimsSession + "/console"
	for _, tc := range []struct {
		name, method, path, body string
		edit                     func(*http.Request)
		status                   int
		code                     string
	}{
		{"get admitted", "GET", path, "", nil, 401, "unauthorized"},
		{"query refused", "GET", path + "?x=1", "", nil, 422, "invalid_request"},
		{"idempotency key refused on a read", "GET", path, "", func(r *http.Request) { r.Header.Set("Idempotency-Key", "claims-key-0001") }, 422, "invalid_request"},
		{"bad session id", "GET", "/v1/admin/stores/" + claimsStore + "/live-sessions/not-a-uuid/console", "", nil, 422, "invalid_request"},
		{"post", "POST", path, `{}`, nil, 405, "method_not_allowed"},
		{"patch", "PATCH", path, `{}`, nil, 405, "method_not_allowed"},
	} {
		w := claimsCall(h, tc.method, tc.path, tc.body, tc.edit)
		var envelope httperror.Envelope
		_ = json.Unmarshal(w.Body.Bytes(), &envelope)
		if w.Code != tc.status || (tc.code != "" && envelope.Code != tc.code) {
			t.Errorf("%s: %d %s, want %d %s", tc.name, w.Code, w.Body.String(), tc.status, tc.code)
		}
		if tc.status != 405 && w.Header().Get("Cache-Control") == "" {
			t.Errorf("%s: no Cache-Control", tc.name)
		}
	}
	// Unmounted when the live-session domain is off.
	if w := claimsCall(NewHandler(nil), "GET", path, "", nil); w.Code != http.StatusNotFound {
		t.Errorf("mounted without Studio: %d", w.Code)
	}
}

// The adjustments route stays mounted without Studio and answers like the old inventory:write route for an unauthenticated or malformed request.
func TestInventoryAdjustRouteOnFullRouter(t *testing.T) {
	h := NewHandler(nil)
	path := "/v1/admin/stores/" + claimsStore + "/inventory/adjustments"
	body := `{"warehouse_id":"` + claimsSession + `","sku_id":"` + claimsSession + `","delta":1,"expected_version":0,"reason":"x"}`
	if w := claimsCall(h, "POST", path, body, nil); w.Code != http.StatusUnauthorized {
		t.Errorf("admitted request: %d %s", w.Code, w.Body.String())
	}
	if w := claimsCall(h, "POST", path, body, func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") }); w.Code != http.StatusUnsupportedMediaType {
		t.Errorf("non-JSON: %d %s", w.Code, w.Body.String())
	}
}

func TestInventoryAdjustClassify(t *testing.T) {
	if status, code := inventoryAdjustClassify(inventory.ErrBelowReserved); status != 422 || code != "below_reserved" {
		t.Errorf("below_reserved -> %d %s", status, code)
	}
	if status, code := inventoryAdjustClassify(nil); status == 422 && code == "below_reserved" {
		t.Errorf("nil classified as below_reserved")
	}
}
