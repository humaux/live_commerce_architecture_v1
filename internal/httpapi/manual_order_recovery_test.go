// Purpose: construct the full router and pin the manual recovery transport before any database access.
// Depends on: NewHandler and real strict-body/bearer/key transport gates.
// Used by: HTTP package and PR checks; storefront-v2 G3.
package httpapi

import (
	"net/http"
	"strings"
	"testing"
)

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
