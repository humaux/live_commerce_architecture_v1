// Purpose: DB-free transport rules of the W3-06B keyword-tool routes, built through the full NewHandler so a route conflict (a ServeMux panic) fails before PostgreSQL is needed.
// Depends on: claims_test.go helpers (claimsHandler, claimsCall, constants); with a nil pool an admitted request is answered 401 at platform.WithScope.
// Used by: go test ./internal/httpapi.
// Invariants: unknown body keys are 400 invalid_json (no tenant_id/store_id from a client), reads (simulate, check, next) need no Idempotency-Key, batch requires one; GET next accepts only prefix and count.

package httpapi

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"livecommerce/internal/httperror"
)

func TestKeywordToolRoutesTransportRules(t *testing.T) {
	h := claimsHandler(t)
	base := "/v1/admin/stores/" + claimsStore + "/live-sessions/" + claimsSession + "/claims"
	noKey := func(r *http.Request) { r.Header.Del("Idempotency-Key") }
	badKey := func(r *http.Request) { r.Header.Set("Idempotency-Key", "x") }
	twoKeys := func(r *http.Request) { r.Header.Add("Idempotency-Key", "claims-key-0002") }
	item := `{"offer_id":"` + claimsOther + `","expected_version":2,"action":"deactivate"}`
	for _, tc := range []struct {
		name, method, path, body string
		edit                     func(*http.Request)
		status                   int
		code                     string
	}{
		// simulate: a read POST (key optional and ignored), body {comment, match_mode?}.
		{"simulate", "POST", base + "/simulate", `{"comment":"A1+2"}`, nil, 401, "unauthorized"},
		{"simulate with mode", "POST", base + "/simulate", `{"comment":"我要H1+1","match_mode":"KEYWORD_QTY_CONTAINS"}`, nil, 401, "unauthorized"},
		{"simulate no key", "POST", base + "/simulate", `{"comment":"A1"}`, noKey, 401, "unauthorized"},
		{"simulate empty comment is a sample", "POST", base + "/simulate", `{"comment":""}`, nil, 401, "unauthorized"},
		{"simulate malformed key", "POST", base + "/simulate", `{"comment":"A1"}`, badKey, 422, "invalid_request"},
		{"simulate two keys", "POST", base + "/simulate", `{"comment":"A1"}`, twoKeys, 422, "invalid_request"},
		{"simulate missing comment", "POST", base + "/simulate", `{"match_mode":"EXACT"}`, nil, 422, "invalid_request"},
		{"simulate null comment", "POST", base + "/simulate", `{"comment":null}`, nil, 400, "invalid_json"},
		{"simulate unknown key", "POST", base + "/simulate", `{"comment":"A1","tenant_id":"x"}`, nil, 400, "invalid_json"},
		{"simulate duplicate key", "POST", base + "/simulate", `{"comment":"A1","comment":"A2"}`, nil, 400, "invalid_json"},
		{"simulate wrong type", "POST", base + "/simulate", `{"comment":7}`, nil, 400, "invalid_json"},
		{"simulate query", "POST", base + "/simulate?x=1", `{"comment":"A1"}`, nil, 422, "invalid_request"},
		{"simulate bad session id", "POST", "/v1/admin/stores/" + claimsStore + "/live-sessions/NOT-A-UUID/claims/simulate", `{"comment":"A1"}`, nil, 422, "invalid_request"},
		{"simulate by get", "GET", base + "/simulate", "", nil, 405, "method_not_allowed"},
		{"simulate by put", "PUT", base + "/simulate", `{}`, nil, 405, "method_not_allowed"},
		// keywords/check.
		{"check", "POST", base + "/keywords/check", `{"keywords":["A1","ａｂ"]}`, nil, 401, "unauthorized"},
		{"check empty list", "POST", base + "/keywords/check", `{"keywords":[]}`, nil, 401, "unauthorized"},
		{"check with mode", "POST", base + "/keywords/check", `{"keywords":["1"],"match_mode":"KEYWORD_QTY_CONTAINS"}`, noKey, 401, "unauthorized"},
		{"check missing list", "POST", base + "/keywords/check", `{}`, nil, 422, "invalid_request"},
		{"check null list", "POST", base + "/keywords/check", `{"keywords":null}`, nil, 400, "invalid_json"},
		{"check string list", "POST", base + "/keywords/check", `{"keywords":"A1"}`, nil, 400, "invalid_json"},
		{"check unknown key", "POST", base + "/keywords/check", `{"keywords":[],"store_id":"x"}`, nil, 400, "invalid_json"},
		{"check by get", "GET", base + "/keywords/check", "", nil, 405, "method_not_allowed"},
		// keywords/next.
		{"next", "GET", base + "/keywords/next", "", nil, 401, "unauthorized"},
		{"next prefix count", "GET", base + "/keywords/next?prefix=B&count=5", "", nil, 401, "unauthorized"},
		{"next count 20", "GET", base + "/keywords/next?count=20", "", nil, 401, "unauthorized"},
		{"next count 0", "GET", base + "/keywords/next?count=0", "", nil, 422, "invalid_request"},
		{"next count 21", "GET", base + "/keywords/next?count=21", "", nil, 422, "invalid_request"},
		{"next count padded", "GET", base + "/keywords/next?count=05", "", nil, 422, "invalid_request"},
		{"next count text", "GET", base + "/keywords/next?count=x", "", nil, 422, "invalid_request"},
		{"next empty prefix", "GET", base + "/keywords/next?prefix=", "", nil, 422, "invalid_request"},
		{"next unknown param", "GET", base + "/keywords/next?limit=5", "", nil, 422, "invalid_request"},
		{"next repeated param", "GET", base + "/keywords/next?count=1&count=2", "", nil, 422, "invalid_request"},
		{"next with key", "GET", base + "/keywords/next", "", func(r *http.Request) { r.Header.Set("Idempotency-Key", "claims-key-0001") }, 422, "invalid_request"},
		{"next by post", "POST", base + "/keywords/next", `{}`, nil, 405, "method_not_allowed"},
		// offers/batch: a write, key required.
		{"batch", "POST", base + "/offers/batch", `{"items":[` + item + `]}`, nil, 401, "unauthorized"},
		{"batch rename", "POST", base + "/offers/batch", `{"items":[{"offer_id":"` + claimsOther + `","expected_version":1,"action":"rename","keyword":"B1"}]}`, nil, 401, "unauthorized"},
		{"batch without key", "POST", base + "/offers/batch", `{"items":[` + item + `]}`, noKey, 422, "invalid_request"},
		{"batch missing items", "POST", base + "/offers/batch", `{}`, nil, 422, "invalid_request"},
		{"batch null items", "POST", base + "/offers/batch", `{"items":null}`, nil, 400, "invalid_json"},
		{"batch unknown item key", "POST", base + "/offers/batch", `{"items":[{"offer_id":"` + claimsOther + `","expected_version":1,"action":"deactivate","tenant_id":"x"}]}`, nil, 400, "invalid_json"},
		{"batch unknown key", "POST", base + "/offers/batch", `{"items":[],"store_id":"x"}`, nil, 400, "invalid_json"},
		{"batch query", "POST", base + "/offers/batch?x=1", `{"items":[` + item + `]}`, nil, 422, "invalid_request"},
		{"batch by get", "GET", base + "/offers/batch", "", nil, 405, "method_not_allowed"},
		// The pre-existing offer routes keep their rules next to the new literal path.
		{"offer patch is not batch", "PATCH", base + "/offers/" + claimsOther, `{"expected_version":1,"max_quantity_per_claim":3,"active":false}`, nil, 401, "unauthorized"},
		{"offer patch bad id", "PATCH", base + "/offers/batch", `{"expected_version":1,"max_quantity_per_claim":3,"active":false}`, nil, 422, "invalid_request"},
	} {
		w := claimsCall(h, tc.method, tc.path, tc.body, tc.edit)
		if w.Code != tc.status {
			t.Errorf("%s: status %d (%s), want %d", tc.name, w.Code, w.Body.String(), tc.status)
			continue
		}
		if w.Header().Get("Cache-Control") != "private, no-store" {
			t.Errorf("%s: Cache-Control %q", tc.name, w.Header().Get("Cache-Control"))
		}
		if tc.code != "" {
			var envelope httperror.Envelope
			if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil || envelope.Code != tc.code {
				t.Errorf("%s: code=%q want %q", tc.name, envelope.Code, tc.code)
			}
		}
	}
}

func TestNextKeywordsQuery(t *testing.T) {
	for _, tc := range []struct {
		raw    string
		prefix string
		count  int
		ok     bool
	}{
		{"", "", 0, true}, {"prefix=B", "B", 0, true}, {"count=3", "", 3, true}, {"prefix=ab&count=20", "ab", 20, true},
		{"count=0", "", 0, false}, {"count=+3", "", 0, false}, {"count=1&count=2", "", 0, false}, {"prefix=", "", 0, false},
		{"x=1", "", 0, false}, {"prefix=%zz", "", 0, false},
	} {
		path := "/p"
		if tc.raw != "" {
			path += "?" + tc.raw
		}
		u, err := url.Parse(path)
		if err != nil {
			t.Fatal(err)
		}
		prefix, count, ok := nextKeywordsQuery(u)
		if ok != tc.ok || (ok && (prefix != tc.prefix || count != tc.count)) {
			t.Errorf("%q => %q %d %t, want %q %d %t", tc.raw, prefix, count, ok, tc.prefix, tc.count, tc.ok)
		}
	}
}
