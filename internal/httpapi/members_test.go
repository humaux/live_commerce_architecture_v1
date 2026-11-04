// Purpose: DB-free transport gates of the member/points routes (loyalty-points-v1 §2) and of the member_award key of the manual-order
//   routes (LP7, LP16 / gate LPG11): exact method/query/media/key/body checks run before any transaction, unknown or server-owned keys
//   are 400 invalid_json, every refusal code survives the httperror table, and orders/for-buyer refuses member_award.
// Depends on: registerMemberRoutes (members.go), NewHandler via toolsCall (merchanttools_test.go), internal/httperror, internal/loyalty.
// Used by: go test ./internal/httpapi. Database behaviour: tests/foundation/loyalty_points_test.go.

package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"livecommerce/internal/loyalty"
)

const lpMember = "33333333-3333-4333-8333-333333333333"

func memberCall(t *testing.T, method, path, ctype, key, body string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	registerMemberRoutes(mux, nil)
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, toolsBase+path, strings.NewReader(body))
	} else {
		r = httptest.NewRequest(method, toolsBase+path, nil)
	}
	r.Header.Set("Authorization", "Bearer "+strings.Repeat("A", 43))
	if ctype != "" {
		r.Header.Set("Content-Type", ctype)
	}
	if key != "" {
		r.Header.Set("Idempotency-Key", key)
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	return w
}

func TestMemberRoutesTransportGates(t *testing.T) {
	const j, k = "application/json", "lp-key-123"
	long := `{"q":"` + strings.Repeat("a", 17<<10) + `"}`
	for _, c := range []struct {
		name                        string
		method, path, ctype, key, b string
		want                        int
		code                        string
	}{
		// 401 = the request passed every transport gate and reached the (nil-pool) scope: the shape is accepted.
		{"search ok", "POST", "/members/search", j, "", `{"q":"王","include_erased":false,"cursor":null,"limit":20}`, 401, "unauthorized"},
		{"search empty body object", "POST", "/members/search", j, "", `{}`, 401, "unauthorized"},
		{"search with an Idempotency-Key", "POST", "/members/search", j, k, `{}`, 422, "invalid_request"},
		{"search term in the query string", "POST", "/members/search?q=x", j, "", `{}`, 422, "invalid_request"},
		{"search not JSON", "POST", "/members/search", "text/plain", "", `{}`, 415, "json_required"},
		{"search unknown key", "POST", "/members/search", j, "", `{"qq":"x"}`, 400, "invalid_json"},
		{"search tenant_id key", "POST", "/members/search", j, "", `{"tenant_id":"x"}`, 400, "invalid_json"},
		{"search body over 16 KiB", "POST", "/members/search", j, "", long, 400, "invalid_json"},
		{"create ok", "POST", "/members", j, k, `{"display_name":"測試會員","contact_note":null,"notes":null}`, 401, "unauthorized"},
		{"create without a key", "POST", "/members", j, "", `{"display_name":"測試會員"}`, 422, "invalid_request"},
		{"create with a short key", "POST", "/members", j, "short", `{"display_name":"測試會員"}`, 422, "invalid_request"},
		{"create without display_name", "POST", "/members", j, k, `{"notes":"x"}`, 422, "invalid_request"},
		{"create with balance", "POST", "/members", j, k, `{"display_name":"測試會員","balance":5}`, 400, "invalid_json"},
		{"create with store_id", "POST", "/members", j, k, `{"display_name":"測試會員","store_id":"x"}`, 400, "invalid_json"},
		{"create with created_at", "POST", "/members", j, k, `{"display_name":"測試會員","created_at":"x"}`, 400, "invalid_json"},
		{"create with a duplicate key", "POST", "/members", j, k, `{"display_name":"a","display_name":"b"}`, 400, "invalid_json"},
		{"detail ok", "GET", "/members/" + lpMember, "", "", "", 401, "unauthorized"},
		{"detail with a ledger cursor", "GET", "/members/" + lpMember + "?ledger_cursor=abc", "", "", "", 401, "unauthorized"},
		{"detail with another query key", "GET", "/members/" + lpMember + "?q=x", "", "", "", 422, "invalid_request"},
		{"detail with a bad id", "GET", "/members/not-a-uuid", "", "", "", 422, "invalid_request"},
		{"detail with a key", "GET", "/members/" + lpMember, "", k, "", 422, "invalid_request"},
		{"patch ok", "PATCH", "/members/" + lpMember, j, k, `{"expected_version":1,"contact_note":null}`, 401, "unauthorized"},
		{"patch without expected_version", "PATCH", "/members/" + lpMember, j, k, `{"notes":"x"}`, 422, "invalid_request"},
		{"patch with nothing to change", "PATCH", "/members/" + lpMember, j, k, `{"expected_version":1}`, 422, "invalid_request"},
		{"patch display_name null", "PATCH", "/members/" + lpMember, j, k, `{"expected_version":1,"display_name":null}`, 422, "invalid_request"},
		{"patch with balance", "PATCH", "/members/" + lpMember, j, k, `{"expected_version":1,"balance":3}`, 400, "invalid_json"},
		{"adjust ok", "POST", "/members/" + lpMember + "/adjustments", j, k, `{"delta":-5,"note":"扣點","order_id":null}`, 401, "unauthorized"},
		{"adjust without a note", "POST", "/members/" + lpMember + "/adjustments", j, k, `{"delta":5}`, 422, "invalid_request"},
		{"adjust with actor", "POST", "/members/" + lpMember + "/adjustments", j, k, `{"delta":5,"note":"x","actor":"y"}`, 400, "invalid_json"},
		{"adjust without a key", "POST", "/members/" + lpMember + "/adjustments", j, "", `{"delta":5,"note":"x"}`, 422, "invalid_request"},
		{"entries ok", "GET", "/members/entries?order_id=" + lpMember, "", "", "", 401, "unauthorized"},
		{"entries without order_id", "GET", "/members/entries", "", "", "", 422, "invalid_request"},
		{"entries with a bad order_id", "GET", "/members/entries?order_id=x", "", "", "", 422, "invalid_request"},
		{"entries with an extra key", "GET", "/members/entries?order_id=" + lpMember + "&q=x", "", "", "", 422, "invalid_request"},
		{"export ok", "POST", "/members/" + lpMember + "/export", j, k, `{}`, 401, "unauthorized"},
		{"export without a key", "POST", "/members/" + lpMember + "/export", j, "", `{}`, 422, "invalid_request"},
		{"erasure ok", "POST", "/members/" + lpMember + "/erasure", j, k, `{}`, 401, "unauthorized"},
		{"erasure with a field", "POST", "/members/" + lpMember + "/erasure", j, k, `{"x":1}`, 400, "invalid_json"},
	} {
		w := memberCall(t, c.method, c.path, c.ctype, c.key, c.b)
		var out struct{ Code string }
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		if w.Code != c.want || out.Code != c.code {
			t.Errorf("%s: %d %s, want %d %s", c.name, w.Code, w.Body.String(), c.want, c.code)
		}
		if cc := w.Header().Get("Cache-Control"); !strings.Contains(cc, "no-store") || !strings.Contains(cc, "private") {
			t.Errorf("%s: Cache-Control %q", c.name, cc)
		}
	}
	if w := memberCall(t, "DELETE", "/members/"+lpMember, j, k, ""); w.Code != http.StatusMethodNotAllowed {
		t.Errorf("DELETE member: %d, want 405", w.Code)
	}
}

// Every refusal the member routes and the award can answer is a contract §2 code that survives httperror's closed table (ruling 15:
// an unknown code is rewritten to "internal").
func TestMemberRefusalCodesSurviveHTTPError(t *testing.T) {
	for _, c := range []struct {
		err    error
		status int
		code   string
	}{
		{loyalty.ErrInsufficient, 409, "points_insufficient"},
		{loyalty.ErrErased, 409, "member_erased"},
		{loyalty.ErrLimit, 422, "limit_reached"},
		{loyalty.ErrVersion, 409, "conflict"},
		{loyalty.ErrExportTooLarge, 409, "export_too_large"},
		{loyalty.ErrIdempotencyConflict, 409, "idempotency_conflict"},
		{loyalty.ErrUnavailable, 503, "unavailable"},
	} {
		status, code, ok := loyalty.Classify(c.err)
		if !ok || status != c.status || code != c.code {
			t.Errorf("Classify(%v) = %d %q %v, want %d %q", c.err, status, code, ok, c.status, c.code)
			continue
		}
		w := httptest.NewRecorder()
		respondError(w, status, code)
		var out struct{ Code string }
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		if w.Code != c.status || out.Code != c.code {
			t.Errorf("%s reached the body as %d %q", c.code, w.Code, out.Code)
		}
	}
}

// member_award on POST orders/manual passes the transport gate (an unwired pipeline then answers 503); on POST orders/for-buyer it is
// an unknown key (LP16, gate LPG11: 400 invalid_json before any order).
func TestMemberAwardBodyKey(t *testing.T) {
	award := `"member_award":{"member_id":"` + lpMember + `","points":30}`
	manual := `{"items":[{"sku_id":"22222222-2222-4222-8222-222222222222","quantity":1}],"customer":{"name":"a","phone":"0912345678","email":""},` +
		`"delivery":{"option_key":"x","home_address":{"region":"","city":"c","postal_code":"","line1":"l","line2":""}},"payment_mode":"bank_transfer","locale":"en"`
	forBuyer := manual + `,"for":{"bundle_ids":[],"conversation_id":null},"send_payment_link":false`
	for _, c := range []struct {
		name, path, body string
		want             int
		code             string
	}{
		{"manual with member_award", "/orders/manual", manual + "," + award + "}", 503, "manual_order_unavailable"},
		{"manual without member_award", "/orders/manual", manual + "}", 503, "manual_order_unavailable"},
		{"manual member_award null", "/orders/manual", manual + `,"member_award":null}`, 422, "invalid_request"},
		{"manual member_award missing points", "/orders/manual", manual + `,"member_award":{"member_id":"` + lpMember + `"}}`, 422, "invalid_request"},
		{"manual member_award unknown nested key", "/orders/manual", manual + `,"member_award":{"member_id":"` + lpMember + `","points":1,"balance":9}}`, 400, "invalid_json"},
		{"LPG11 for-buyer with member_award", "/orders/for-buyer", forBuyer + "," + award + "}", 400, "invalid_json"},
		{"for-buyer without member_award", "/orders/for-buyer", forBuyer + "}", 503, "manual_order_unavailable"},
	} {
		w := toolsCall(t, "POST", c.path, "application/json", "some-key-123", c.body)
		var out struct{ Code string }
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		if w.Code != c.want || out.Code != c.code {
			t.Errorf("%s: %d %s, want %d %s", c.name, w.Code, w.Body.String(), c.want, c.code)
		}
	}
}
