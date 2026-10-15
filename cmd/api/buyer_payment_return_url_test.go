// Purpose: unit gate for canonicalBuyerPaymentReturnURL, the Stripe success/cancel return-URL check. Its cases were carried over from the
//
//	retired accounts.HostedConfig tests (PAY-RM2 deleted them with the PAYUNi adapter); the function now guards the live Stripe return page.
//
// Depends on: cmd/api/buyer_payment.go (canonicalBuyerPaymentReturnURL); stdlib only, DB-free.
// Used by: go test ./cmd/api (unit gate); PR #70 review finding (return-URL validator lost its tests).
// Invariants: only https, a public DNS host, the default port, a clean path and no query/fragment/userinfo/escapes are accepted; host and :443 normalise.
package main

import "testing"

func TestBuyerPaymentReturnURLRejectsUnsafeEndpoints(t *testing.T) {
	for _, bad := range []string{"", "http://pay.example.com/return", "https://pay.example.com:8443/return",
		"https://user@pay.example.com/return", "https://pay.example.com/return?x=1",
		"https://pay.example.com/return#x", "https://127.0.0.1/return",
		"https://localhost/return", "https://shop.localhost/return", "https://pay.example.com/a/../return",
		"https://pay.example.com:/return", "https://pay.example.com/return%2Fextra",
		"https://pay.example.com/返回", "https://pay.example.com/return<one>",
		"https://pay.example.com/return\"one", "https://intranet/return", "https://pay.example.com"} {
		if got, ok := canonicalBuyerPaymentReturnURL(bad); ok {
			t.Fatalf("unsafe return URL accepted: %q -> %q", bad, got)
		}
	}
}

func TestBuyerPaymentReturnURLCanonicalForm(t *testing.T) {
	for raw, want := range map[string]string{
		"https://PAY.example.com:443/return": "https://pay.example.com/return",
		"https://pay.example.com/return":     "https://pay.example.com/return",
		"https://pay.example.com/return-v2":  "https://pay.example.com/return-v2",
	} {
		got, ok := canonicalBuyerPaymentReturnURL(raw)
		if !ok || got != want {
			t.Fatalf("canonical return URL %q = %q, %v; want %q", raw, got, ok, want)
		}
		if again, ok := canonicalBuyerPaymentReturnURL(got); !ok || again != got {
			t.Fatalf("canonical form %q is not a fixed point: %q, %v", got, again, ok)
		}
	}
}
