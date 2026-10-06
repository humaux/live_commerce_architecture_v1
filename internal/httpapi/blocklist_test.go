// Purpose: DB-free transport rules of the W3-05B blocklist routes (built through the full NewHandler so a route conflict fails before PostgreSQL is needed) and the error mapping.
// Depends on: claims_test.go helpers (claimsHandler, claimsCall, constants); with a nil pool an admitted request is answered 401 at platform.WithScope.
// Used by: go test ./internal/httpapi.
// Invariants: no actor_key in a request (unknown body keys are 400 invalid_json), exactly one reference (422), idempotency key rules as the rest of the claims family.

package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"

	"livecommerce/internal/claims"
	"livecommerce/internal/httperror"
)

func TestBlocklistRoutesTransportRules(t *testing.T) {
	h := claimsHandler(t)
	base := "/v1/admin/stores/" + claimsStore + "/live-sessions/" + claimsSession + "/claims/blocklist"
	noKey := func(r *http.Request) { r.Header.Del("Idempotency-Key") }
	for _, tc := range []struct {
		name, method, path, body string
		edit                     func(*http.Request)
		status                   int
		code                     string
	}{
		{"list", "GET", base, "", nil, 401, "unauthorized"},
		{"list paged", "GET", base + "?limit=20&cursor=abc", "", nil, 401, "unauthorized"},
		{"list bad limit", "GET", base + "?limit=101", "", nil, 422, "invalid_request"},
		{"list label filter", "GET", base + "?label=amy", "", nil, 422, "invalid_request"},
		{"list with key", "GET", base, "", func(r *http.Request) { r.Header.Set("Idempotency-Key", "claims-key-0001") }, 422, "invalid_request"},
		{"add by comment", "POST", base, `{"comment_ref":"123_456","note":"spam"}`, nil, 401, "unauthorized"},
		{"add by bundle", "POST", base, `{"bundle_id":"` + claimsOther + `"}`, nil, 401, "unauthorized"},
		{"add by conversation", "POST", base, `{"conversation_id":"` + claimsOther + `"}`, nil, 401, "unauthorized"},
		{"add without key", "POST", base, `{"bundle_id":"` + claimsOther + `"}`, noKey, 422, "invalid_request"},
		{"add no reference", "POST", base, `{"note":"x"}`, nil, 422, "invalid_request"},
		{"add two references", "POST", base, `{"comment_ref":"1","bundle_id":"` + claimsOther + `"}`, nil, 422, "invalid_request"},
		{"add actor_key is not a reference", "POST", base, `{"actor_key":"` + claimsOther + `"}`, nil, 400, "invalid_json"},
		{"add actor_key beside a reference", "POST", base, `{"bundle_id":"` + claimsOther + `","actor_key":"x"}`, nil, 400, "invalid_json"},
		{"add null note", "POST", base, `{"bundle_id":"` + claimsOther + `","note":null}`, nil, 400, "invalid_json"},
		{"add query", "POST", base + "?x=1", `{"bundle_id":"` + claimsOther + `"}`, nil, 422, "invalid_request"},
		{"remove", "DELETE", base + "/entries/" + claimsOther, "", nil, 401, "unauthorized"},
		{"remove bad id", "DELETE", base + "/entries/NOT-A-UUID", "", nil, 422, "invalid_request"},
		{"remove without key", "DELETE", base + "/entries/" + claimsOther, "", noKey, 422, "invalid_request"},
		{"remove by get", "GET", base + "/entries/" + claimsOther, "", nil, 405, "method_not_allowed"},
		{"check", "GET", base + "/check?bundle_id=" + claimsOther, "", nil, 401, "unauthorized"},
		{"check missing bundle", "GET", base + "/check", "", nil, 422, "invalid_request"},
		{"check bad bundle", "GET", base + "/check?bundle_id=abc", "", nil, 422, "invalid_request"},
		{"check extra param", "GET", base + "/check?bundle_id=" + claimsOther + "&x=1", "", nil, 422, "invalid_request"},
		{"check duplicate param", "GET", base + "/check?bundle_id=" + claimsOther + "&bundle_id=" + claimsOther, "", nil, 422, "invalid_request"},
		{"check by post", "POST", base + "/check", `{}`, nil, 405, "method_not_allowed"},
		{"list put", "PUT", base, `{}`, nil, 405, "method_not_allowed"},
	} {
		w := claimsCall(h, tc.method, tc.path, tc.body, tc.edit)
		if w.Code != tc.status {
			t.Errorf("%s: status %d (%s), want %d", tc.name, w.Code, w.Body.String(), tc.status)
			continue
		}
		if tc.code != "" {
			var envelope httperror.Envelope
			if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil || envelope.Code != tc.code {
				t.Errorf("%s: code=%q want %q", tc.name, envelope.Code, tc.code)
			}
		}
	}
}

func TestBlocklistClassify(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{claims.ErrBlocklistFull, http.StatusConflict, "limit_reached"},
		{claims.ErrAmbiguousActor, http.StatusConflict, "ambiguous_actor"},
	} {
		if status, code := blocklistClassify(tc.err); status != tc.status || code != tc.code {
			t.Errorf("%v -> %d %s, want %d %s", tc.err, status, code, tc.status, tc.code)
		}
	}
}
