// reports_test.go covers the W6-02B report routes' transport rules before any database work: the full router builds (a route conflict panics),
// methods, the exact query grammar, the 0..91 day range (422), the bearer shape and the funnel-only session_id. Real-PG money, tenancy and
// export-audit outcomes are tests/foundation/reports_test.go.
//
// Purpose: DB-free gate for internal/httpapi/reports.go. Depends on: NewHandler (handler.go), customerCall helpers (customers_test.go).
// Used by: go test ./internal/httpapi.

package httpapi

import (
	"net/http"
	"net/url"
	"testing"
)

func TestReportRoutesTransportRulesBeforeDatabase(t *testing.T) {
	h := NewHandler(nil) // the whole router: a pattern conflict with any other family panics here
	base := "/v1/admin/stores/" + custStore + "/reports/"
	const q = "?from=2026-09-01&to=2026-09-30"
	sess := "&session_id=" + custID
	for _, tc := range []struct {
		name, method, path string
		edit               func(*http.Request)
		status             int
	}{
		{"products", "GET", base + "products" + q, nil, 401},
		{"channels", "GET", base + "channels" + q, nil, 401},
		{"funnel", "GET", base + "funnel" + q, nil, 401},
		{"funnel session", "GET", base + "funnel" + q + sess, nil, 401},
		{"manual", "GET", base + "manual-orders" + q, nil, 401},
		{"products csv", "GET", base + "products.csv" + q, nil, 401},
		{"channels csv", "GET", base + "channels.csv" + q, nil, 401},
		{"funnel csv", "GET", base + "funnel.csv" + q + sess, nil, 401},
		{"manual csv", "GET", base + "manual-orders.csv" + q, nil, 401},
		{"max range", "GET", base + "products?from=2026-01-01&to=2026-04-02", nil, 401},
		{"93 days", "GET", base + "products?from=2026-01-01&to=2026-04-03", nil, 422},
		{"93 days csv", "GET", base + "channels.csv?from=2026-01-01&to=2026-04-03", nil, 422},
		{"reversed", "GET", base + "manual-orders?from=2026-09-02&to=2026-09-01", nil, 422},
		{"no query", "GET", base + "products", nil, 422},
		{"missing to", "GET", base + "products?from=2026-09-01", nil, 422},
		{"bad date", "GET", base + "products?from=2026-02-30&to=2026-03-01", nil, 422},
		{"extra key", "GET", base + "products" + q + "&tz=UTC", nil, 422},
		{"duplicate from", "GET", base + "products?from=2026-09-01&from=2026-09-02&to=2026-09-03", nil, 422},
		{"session on products", "GET", base + "products" + q + sess, nil, 422},
		{"session on csv products", "GET", base + "products.csv" + q + sess, nil, 422},
		{"bad session", "GET", base + "funnel" + q + "&session_id=nope", nil, 422},
		{"post", "POST", base + "products" + q, nil, 405},
		{"head", "HEAD", base + "channels" + q, nil, 405},
		{"with key", "GET", base + "products" + q, func(r *http.Request) { r.Header.Set("Idempotency-Key", "cust-key-0001") }, 422},
		{"no bearer", "GET", base + "products" + q, func(r *http.Request) { r.Header.Del("Authorization") }, 401},
		{"unknown report", "GET", base + "profit" + q, nil, 404},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := customerCall(h, tc.method, tc.path, "", tc.edit)
			if w.Code != tc.status {
				t.Fatalf("status=%d want=%d body=%s", w.Code, tc.status, w.Body.String())
			}
			if tc.status == 422 || tc.status == 401 {
				if cc := w.Header().Get("Cache-Control"); cc != "no-store, private" {
					t.Fatalf("cache-control %q", cc)
				}
			}
		})
	}
}

func TestParseReportQuery(t *testing.T) {
	u, _ := url.Parse("http://x/?from=2026-09-01&to=2026-09-30&session_id=" + custID)
	if from, to, session, err := parseReportQuery(u, true); err != nil || from != "2026-09-01" || to != "2026-09-30" || session != custID {
		t.Fatalf("%s %s %s %v", from, to, session, err)
	}
	if _, _, _, err := parseReportQuery(u, false); err == nil {
		t.Fatal("session_id accepted where not allowed")
	}
}
