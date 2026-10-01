package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"livecommerce/internal/merchanttools"
)

// Transport gates of the merchant-tools routes that need no database: exact method/media type/key/body checks run before any transaction,
// the coded refusals reach the JSON body, and an unwired manual-order pipeline answers 503 manual_order_unavailable. Database behaviour
// is in tests/foundation/merchant_tools_*_smoke_test.go.

const toolsBase = "/v1/admin/stores/11111111-1111-4111-8111-111111111111"

func toolsCall(t *testing.T, method, path, ctype, key, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader // no body at all on a GET, like the real server (http.NoBody)
	if body != "" {
		reader = strings.NewReader(body)
	}
	r := httptest.NewRequest(method, toolsBase+path, reader)
	r.Header.Set("Authorization", "Bearer "+strings.Repeat("A", 43))
	if ctype != "" {
		r.Header.Set("Content-Type", ctype)
	}
	if key != "" {
		r.Header.Set("Idempotency-Key", key)
	}
	w := httptest.NewRecorder()
	NewHandler(nil).ServeHTTP(w, r)
	return w
}

func TestMerchantToolsTransportGates(t *testing.T) {
	for _, c := range []struct {
		name                        string
		method, path, ctype, key, b string
		want                        int
	}{
		{"import with a JSON body", "POST", "/products/import/preview", "application/json", "", "{}", 415},
		{"import commit with a JSON body", "POST", "/products/import/commit", "application/json", "", "{}", 415},
		{"import with a key (it is idempotent by file hash)", "POST", "/products/import/commit", "text/csv", "some-key-123", "a,b", 422},
		{"import with a query", "POST", "/products/import/preview?x=1", "text/csv", "", "a,b", 422},
		{"import with no body", "POST", "/products/import/preview", "text/csv", "", "", 413},
		{"dashboard with a query", "GET", "/dashboard?x=1", "", "", "", 422},
		{"dashboard with a key", "GET", "/dashboard", "", "some-key-123", "", 422},
		{"export with a query", "GET", "/products/export.csv?x=1", "", "", "", 422},
		{"manual order without a key", "POST", "/orders/manual", "application/json", "", `{}`, 422},
		{"manual order with a price field", "POST", "/orders/manual", "application/json", "some-key-123", `{"items":[],"customer":{},"delivery":{},"payment_mode":"bank_transfer","locale":"en","total_minor":1}`, 400},
		{"manual order with an unknown nested key", "POST", "/orders/manual", "application/json", "some-key-123", `{"items":[{"sku_id":"x","quantity":1,"price_minor":1}],"customer":{},"delivery":{},"payment_mode":"bank_transfer","locale":"en"}`, 400},
		{"manual order not JSON", "POST", "/orders/manual", "text/plain", "some-key-123", `x`, 415},
		{"wrong method on options", "POST", "/orders/manual/options", "application/json", "", `{}`, 405},
	} {
		if w := toolsCall(t, c.method, c.path, c.ctype, c.key, c.b); w.Code != c.want {
			t.Errorf("%s: %d want %d: %s", c.name, w.Code, c.want, w.Body.String())
		}
	}
	// Without a bearer nothing is attempted.
	r := httptest.NewRequest("GET", toolsBase+"/dashboard", nil)
	w := httptest.NewRecorder()
	NewHandler(nil).ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("no bearer: %d", w.Code)
	}
}

func TestUnwiredManualOrdersAnswer503WithTheirCode(t *testing.T) {
	w := toolsCall(t, "GET", "/orders/manual/options", "", "", "")
	var body struct{ Code, Message string }
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if w.Code != http.StatusServiceUnavailable || body.Code != "manual_order_unavailable" || body.Message == "" || body.Message == "Request could not be completed." {
		t.Fatalf("options unwired: %d %+v", w.Code, body)
	}
}

// Every refusal code the manual-order and CSV paths can answer must survive httperror's closed message table (ruling 15: an unknown code is
// rewritten to "internal").
func TestMerchantToolsCodesReachJSONBody(t *testing.T) {
	for _, c := range []struct {
		err    error
		status int
		code   string
	}{
		{merchanttools.ErrManualDisabled, 503, "manual_order_unavailable"},
		{merchanttools.ErrExportTooLarge, 422, "export_too_large"},
		{merchanttools.ErrUnavailable, 503, "retry_later"},
		{&merchanttools.Error{Status: 422, Code: "cvs_entry_unavailable"}, 422, "cvs_entry_unavailable"},
		{&merchanttools.Error{Status: 422, Code: "bank_transfer_unavailable"}, 422, "bank_transfer_unavailable"},
		{&merchanttools.Error{Status: 422, Code: "pay_at_pickup_unavailable"}, 422, "pay_at_pickup_unavailable"},
		{&merchanttools.Error{Status: 409, Code: "insufficient_inventory"}, 409, "insufficient_inventory"},
		{&merchanttools.Error{Status: 409, Code: "idempotency_conflict"}, 409, "idempotency_conflict"},
		{&merchanttools.Error{Status: 422, Code: "invalid_request"}, 422, "invalid_request"},
	} {
		status, code := toolsClassify(c.err)
		w := httptest.NewRecorder()
		respondError(w, status, code)
		var body struct{ Code, Message string }
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || w.Code != c.status || body.Code != c.code || body.Message == "" || body.Message == "Request could not be completed." {
			t.Errorf("%v: %d %+v %v, want %d %s", c.err, w.Code, body, err, c.status, c.code)
		}
	}
}
