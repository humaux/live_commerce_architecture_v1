// live_lifecycle_test.go covers the A7 route DB-free: the full NewHandler router (Studio on, so studio + live-flow +
// claims-adjacent families share one mux and a pattern conflict panics here, not at first use) and the transport rules
// that hold before any transaction. With a nil pool a request that passes every transport rule reaches
// platform.WithScope and is answered 401, the "admitted" marker. Real-PG behaviour: tests/foundation/live_lifecycle_test.go.
package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"

	"livecommerce/internal/claims"
	"livecommerce/internal/command"
	"livecommerce/internal/httperror"
	"livecommerce/internal/platform"
)

func TestLiveLifecycleRouteOnFullRouter(t *testing.T) {
	h := NewHandler(nil, Options{Studio: true})
	path := "/v1/admin/stores/" + claimsStore + "/live-sessions/" + claimsSession + "/lifecycle"
	noKey := func(r *http.Request) { r.Header.Del("Idempotency-Key") }
	for _, tc := range []struct {
		name, method, path, body string
		edit                     func(*http.Request)
		status                   int
		code                     string
	}{
		{"start admitted", "POST", path, `{"action":"start","expected_version":1}`, nil, 401, "unauthorized"},
		{"end with open_window", "POST", path, `{"action":"end","expected_version":2,"open_window":false}`, nil, 401, "unauthorized"},
		{"no idempotency key", "POST", path, `{"action":"start","expected_version":1}`, noKey, 422, "invalid_request"},
		{"missing expected_version", "POST", path, `{"action":"start"}`, nil, 422, "invalid_request"},
		{"missing action", "POST", path, `{"expected_version":1}`, nil, 422, "invalid_request"},
		{"unknown field", "POST", path, `{"action":"start","expected_version":1,"tenant_id":"x"}`, nil, 400, "invalid_json"},
		{"null open_window", "POST", path, `{"action":"start","expected_version":1,"open_window":null}`, nil, 400, "invalid_json"},
		{"bad session id", "POST", "/v1/admin/stores/" + claimsStore + "/live-sessions/not-a-uuid/lifecycle", `{"action":"start","expected_version":1}`, nil, 422, "invalid_request"},
		{"query refused", "POST", path + "?x=1", `{"action":"start","expected_version":1}`, nil, 422, "invalid_request"},
		{"get", "GET", path, "", nil, 405, "method_not_allowed"},
		{"patch", "PATCH", path, `{}`, nil, 405, "method_not_allowed"},
	} {
		w := claimsCall(h, tc.method, tc.path, tc.body, tc.edit)
		var envelope httperror.Envelope
		_ = json.Unmarshal(w.Body.Bytes(), &envelope)
		if w.Code != tc.status || (tc.code != "" && envelope.Code != tc.code) {
			t.Errorf("%s: %d %s, want %d %s", tc.name, w.Code, w.Body.String(), tc.status, tc.code)
		}
	}
	// Unmounted when the live-session domain is off.
	if w := claimsCall(NewHandler(nil), "POST", path, `{"action":"start","expected_version":1}`, nil); w.Code != http.StatusNotFound {
		t.Errorf("mounted without Studio: %d", w.Code)
	}
}

func TestLifecycleClassify(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{claims.ErrInvalidTransition, 409, "invalid_transition"},
		{claims.ErrTooManyOpenWindows, 409, "too_many_open_windows"},
		{command.ErrConflict, 409, "version_conflict"},
		{claims.ErrBillingRestricted, 402, "billing_restricted"},
		{command.ErrNotFound, 404, "not_found"},
		{platform.ErrForbidden, 403, "forbidden"},
		{command.ErrInvalid, 422, "invalid_request"},
	} {
		if status, code := lifecycleClassify(tc.err); status != tc.status || code != tc.code {
			t.Errorf("%v -> %d %s, want %d %s", tc.err, status, code, tc.status, tc.code)
		}
	}
}
