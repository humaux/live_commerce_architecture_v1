// Purpose: PAY-RM1 gate RM03 — the API mount chain no longer reserves /v1/hooks/payuni: a notify POST
//
//	falls through to the base httpapi router and 404s, while the neighbouring webhook mounts are unchanged.
//
// Depends on: internal/httpapi (NewHandler with a nil pool: DB-free), the cmd/api mount* wrappers.
// Used by: go test ./cmd/api (unit gate); referenced by output/pay-rm1-remove-payuni/DELIVERY.md.
// Invariants: removal must not reroute any other namespace (I18: the 404 is asserted, not assumed).
package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"livecommerce/internal/httpapi"
)

// TestRemovePayuniNotifyRM03RouterHasNoNotifyRoute builds the same mount chain as run() in main.go and
// asserts the removed PAYUNi notify namespace is gone while the other mounts still route. DB-free.
func TestRemovePayuniNotifyRM03RouterHasNoNotifyRoute(t *testing.T) {
	routed := map[string]int{}
	stub := func(name string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			routed[name]++
			w.WriteHeader(http.StatusOK)
		})
	}
	base := httpapi.NewHandler(nil)
	h := mountBuyer(base, nil)
	h = mountMeta(h, stub("meta"))
	h = mountStripe(h, stub("stripe"))
	h = mountPlatformBilling(h, nil)
	h = mountTLSAsk(h, nil)
	h = mountStoreDomainNonce(h, nil)

	token := strings.Repeat("A", 43)
	for _, p := range []string{"/v1/hooks/payuni/notify/" + token, "/v1/hooks/payuni", "/v1/hooks/payunix"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, p, nil))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("POST %s = %d, want 404 (receiver removed); routed=%v", p, rec.Code, routed)
		}
	}
	if routed["payuni"] != 0 {
		t.Fatalf("a payuni stub still intercepts the namespace: %v", routed)
	}
	// Control: the neighbouring webhook namespaces still reach their handlers ("no other route changed").
	for path, want := range map[string]string{"/v1/stripe/webhook/x": "stripe", "/v1/meta/webhook/x": "meta"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, nil))
		if rec.Code != http.StatusOK || routed[want] != 1 {
			t.Fatalf("POST %s = %d routed=%v, want the %s stub", path, rec.Code, routed, want)
		}
	}
}
