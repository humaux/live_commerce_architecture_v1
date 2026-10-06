// Purpose: DB-free transport gates of the order-for-a-buyer routes (live-console-v1 §11 A15/A16, unit LC-B6): the full router builds without a route
// conflict, exact method/query/media/key/body checks run before any transaction, an unwired service answers 503 manual_order_unavailable, and the two
// new refusal codes reach the JSON body with the existing order id. Database behaviour is in tests/foundation/live_console_order_for_buyer_test.go.
// Depends on: NewHandler (the full router), internal/claims.ForBuyerError, internal/httperror (via respondErrorDetails).
// Used by: go test ./internal/httpapi.

package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"livecommerce/internal/claims"
)

func TestForBuyerRoutesTransportGates(t *testing.T) {
	const id = "22222222-2222-4222-8222-222222222222"
	for _, c := range []struct {
		name                        string
		method, path, ctype, key, b string
		want                        int
	}{
		{"prefill with a body", "GET", "/inbox/order-prefill?bundle_id=" + id, "", "", "x", 422},
		{"prefill with a key", "GET", "/inbox/order-prefill?bundle_id=" + id, "", "some-key-123", "", 422},
		{"prefill without a query", "GET", "/inbox/order-prefill", "", "", "", 422},
		{"prefill with both ids", "GET", "/inbox/order-prefill?bundle_id=" + id + "&conversation_id=" + id, "", "", "", 422},
		{"prefill with a bad id", "GET", "/inbox/order-prefill?bundle_id=nope", "", "", "", 422},
		{"prefill with an unknown key", "GET", "/inbox/order-prefill?x=" + id, "", "", "", 422},
		{"prefill wrong method", "POST", "/inbox/order-prefill?bundle_id=" + id, "application/json", "", `{}`, 405},
		{"prefill unwired", "GET", "/inbox/order-prefill?bundle_id=" + id, "", "", "", 503},
		{"for-buyer without a key", "POST", "/orders/for-buyer", "application/json", "", `{}`, 422},
		{"for-buyer not JSON", "POST", "/orders/for-buyer", "text/plain", "some-key-123", `x`, 415},
		{"for-buyer with a price field", "POST", "/orders/for-buyer", "application/json", "some-key-123",
			`{"items":[],"customer":{},"delivery":{},"payment_mode":"bank_transfer","locale":"en","for":{"bundle_ids":[],"conversation_id":null},"send_payment_link":false,"total_minor":1}`, 400},
		{"for-buyer with an unknown key in for", "POST", "/orders/for-buyer", "application/json", "some-key-123",
			`{"items":[],"customer":{},"delivery":{},"payment_mode":"bank_transfer","locale":"en","for":{"bundle_ids":[],"conversation_id":null,"tenant_id":"x"},"send_payment_link":false}`, 400},
		{"for-buyer missing send_payment_link", "POST", "/orders/for-buyer", "application/json", "some-key-123",
			`{"items":[],"customer":{},"delivery":{},"payment_mode":"bank_transfer","locale":"en","for":{"bundle_ids":[],"conversation_id":null}}`, 422},
		{"for-buyer wrong method", "PUT", "/orders/for-buyer", "application/json", "some-key-123", `{}`, 405},
	} {
		if w := toolsCall(t, c.method, c.path, c.ctype, c.key, c.b); w.Code != c.want {
			t.Errorf("%s: %d want %d: %s", c.name, w.Code, c.want, w.Body.String())
		}
	}
}

func TestUnwiredForBuyerAnswers503WithTheirCode(t *testing.T) {
	body := `{"items":[{"sku_id":"22222222-2222-4222-8222-222222222222","quantity":1}],"customer":{"name":"a","phone":"0912345678","email":""},` +
		`"delivery":{"option_key":"x","home_address":{"region":"","city":"c","postal_code":"","line1":"l","line2":""}},"payment_mode":"bank_transfer","locale":"en",` +
		`"for":{"bundle_ids":[],"conversation_id":null},"send_payment_link":false}`
	w := toolsCall(t, "POST", "/orders/for-buyer", "application/json", "some-key-123", body)
	var out struct{ Code, Message string }
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if w.Code != http.StatusServiceUnavailable || out.Code != "manual_order_unavailable" {
		t.Fatalf("unwired for-buyer: %d %s", w.Code, w.Body.String())
	}
}

// The refusal codes of A16 are in the httperror table (an unknown code would be rewritten to "internal") and bundle_already_ordered carries the
// existing order id so the UI can offer 「查看訂單」 instead of a second order.
func TestForBuyerRefusalsReachTheBody(t *testing.T) {
	for _, c := range []struct {
		err     error
		code    string
		orderID any
	}{
		{&claims.ForBuyerError{Status: 409, Code: "bundle_already_ordered", OrderID: "33333333-3333-4333-8333-333333333333"}, "bundle_already_ordered", "33333333-3333-4333-8333-333333333333"},
		{&claims.ForBuyerError{Status: 409, Code: "bundle_already_ordered"}, "bundle_already_ordered", nil},
		{&claims.ForBuyerError{Status: 409, Code: "bundle_buyer_mismatch"}, "bundle_buyer_mismatch", nil},
	} {
		w := httptest.NewRecorder()
		respondForBuyerError(w, c.err)
		var out struct {
			Code, Message string
			Details       map[string]any
		}
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		if w.Code != 409 || out.Code != c.code || out.Message == "" || out.Message == "Request could not be completed." || strings.Contains(w.Body.String(), "claims:") {
			t.Fatalf("%v: %d %s", c.err, w.Code, w.Body.String())
		}
		if c.code == "bundle_already_ordered" && out.Details["order_id"] != c.orderID {
			t.Fatalf("order id in details: %v", out.Details)
		}
	}
}
