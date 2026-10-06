// payment_card_test.go: DB-free router tests of the platform-Stripe card routes (PF08, stripe-platform-account-v1 §3.3).
// Depends on: NewHandler, registerPaymentCardRoutes. Used by: go test ./internal/httpapi.

package httpapi

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const pcStore = "11111111-2222-4333-8444-555555555555"

func body(method string) io.Reader {
	if method == http.MethodPut {
		return strings.NewReader("{}")
	}
	return nil
}

func TestPlatformStripePF08RouterBuild(t *testing.T) {
	// NewHandler panics on a route conflict: building it for every profile proves the three routes coexist with the rest.
	for _, profile := range []string{"PROVIDER_MOCK", "SANDBOX", "LIVE"} {
		h := NewHandler(nil, Options{PaymentProfile: profile})
		for _, c := range []struct {
			method, token string
			want          int
		}{{http.MethodGet, "", 401}, {http.MethodPut, "", 401}, {http.MethodPost, "", 405}, {http.MethodDelete, "", 405}} {
			req := httptest.NewRequest(c.method, "/v1/admin/stores/"+pcStore+"/payments/card", body(c.method))
			if c.token != "" {
				req.Header.Set("Authorization", "Bearer "+c.token)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if w.Code != c.want {
				t.Fatalf("%s %s: got %d want %d", profile, c.method, w.Code, c.want)
			}
		}
	}
	// no profile (payment-free deployment): the surface is not mounted
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/stores/"+pcStore+"/payments/card", nil)
	w := httptest.NewRecorder()
	NewHandler(nil, Options{}).ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("unmounted surface answered %d", w.Code)
	}
}
