// Purpose: construct the complete router and assert A16R and G3 mounted recovery transport contracts without PostgreSQL.
// Depends on: NewHandler, real CVS strict-body/bearer/key gates; merchanttools disabled-service behavior.
// Used by: Go HTTP tests and PR checks.
// Invariants: I01/I11; strict original request shapes and no-store responses.
package httpapi

import (
	"encoding/json"
	"livecommerce/internal/merchanttools"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestForBuyerRecoveryRouter(t *testing.T) {
	const body = `{"items":[{"sku_id":"22222222-2222-4222-8222-222222222222","quantity":1}],"customer":{"name":"Test","phone":"123456789","email":"buyer@example.test"},"delivery":{"option_key":"33333333-3333-4333-8333-333333333333|TW|home","home_address":{"region":"test","city":"test","postal_code":"100","line1":"test","line2":""}},"payment_mode":"bank_transfer","locale":"en","for":{"bundle_ids":[],"conversation_id":null},"send_payment_link":false}`
	for _, c := range []struct {
		name, ctype, key, body string
		status                 int
	}{
		{"mounted", "application/json", "recovery-123", body, 503},
		{"key required", "application/json", "", body, 422},
		{"json required", "text/plain", "recovery-123", body, 415},
		{"reject extra fields", "application/json", "recovery-123", strings.TrimSuffix(body, "}") + `,"price":1}`, 400},
	} {
		t.Run(c.name, func(t *testing.T) {
			w := toolsCall(t, http.MethodPost, "/orders/for-buyer/recover", c.ctype, c.key, c.body)
			if w.Code != c.status {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			if w.Header().Get("Cache-Control") != "private, no-store" {
				t.Fatalf("cache=%q", w.Header().Get("Cache-Control"))
			}
		})
	}
}

func TestForBuyerRecoveryAttemptClosedCode(t *testing.T) {
	w := httptest.NewRecorder()
	respondForBuyerError(w, &merchanttools.Error{Status: http.StatusConflict, Code: "attempt_closed"})
	var body struct {
		Code, Message string
		Retryable     bool
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if w.Code != 409 || body.Code != "attempt_closed" || body.Message == "" || body.Message == "Request could not be completed." || body.Retryable {
		t.Fatalf("closed attempt transport=%d %+v", w.Code, body)
	}
}

func TestManualRecoveryRouter(t *testing.T) {
	const body = `{"items":[],"customer":{},"delivery":{},"payment_mode":"bank_transfer","locale":"en"}`
	for _, c := range []struct {
		name, ctype, key, body string
		status                 int
	}{
		{"mounted", "application/json", "manual-recover-123", body, 503},
		{"key required", "application/json", "", body, 422},
		{"json required", "text/plain", "manual-recover-123", body, 415},
		{"extra fields", "application/json", "manual-recover-123", strings.TrimSuffix(body, "}") + `,"price":1}`, 400},
		{"nested fields", "application/json", "manual-recover-123", `{"items":[{"sku_id":"x","quantity":1,"price":1}],"customer":{},"delivery":{},"payment_mode":"bank_transfer","locale":"en"}`, 400},
	} {
		t.Run(c.name, func(t *testing.T) {
			w := toolsCall(t, http.MethodPost, "/orders/manual/recover", c.ctype, c.key, c.body)
			if w.Code != c.status {
				t.Fatalf("status=%d want=%d", w.Code, c.status)
			}
			if w.Header().Get("Cache-Control") != "private, no-store" {
				t.Fatal("missing no-store")
			}
		})
	}
}
