package buyerhttp

import (
	"net/http"
	"testing"
)

// The manual-order link exchange route (orderlink.go): exact path, POST only, and a token grammar that keeps junk off the private transport.
func TestOrderLinkRouteTableAndTokenGrammar(t *testing.T) {
	if r := matchRoute(orderLinkPath); r.kind != routeOrderLink || r.id != "" {
		t.Fatalf("path: %+v", r)
	}
	for _, m := range []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		if allowed(routeOrderLink, m) {
			t.Errorf("%s must not be allowed", m)
		}
	}
	if !allowed(routeOrderLink, http.MethodPost) {
		t.Error("POST must be allowed")
	}
	for _, p := range []string{orderLinkPath + "/extra", orderLinkPath + "/", "/v1/buyer/orders/link2"} {
		if matchRoute(p).kind == routeOrderLink {
			t.Errorf("%s must not match", p)
		}
	}
	good := "AbCdEfGhIjKlMnOpQrStUvWxYz0123456789_-abcde"
	if len(good) != 43 || !linkTokenPattern.MatchString(good) {
		t.Fatal("a 43-char base64url token must match")
	}
	for _, bad := range []string{"", good[:42], good + "A", good[:42] + "=", good[:42] + " ", good[:42] + "+"} {
		if linkTokenPattern.MatchString(bad) {
			t.Errorf("%q must not match", bad)
		}
	}
}
