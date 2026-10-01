package buyerhttp

import (
	"crypto/sha256"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLookupRouteTable(t *testing.T) {
	if r := matchRoute(lookupPath); r.kind != routeLookup || r.id != "" {
		t.Fatalf("lookup path must win over /orders/{id}: %+v", r)
	}
	if !allowed(routeLookup, http.MethodPost) || allowed(routeLookup, http.MethodGet) || allowed(routeLookup, http.MethodPut) {
		t.Fatal("lookup is POST only")
	}
	if matchRoute("/v1/buyer/orders/lookup/extra").kind == routeLookup {
		t.Fatal("exact path only")
	}
}

func TestNormalizeRef(t *testing.T) {
	for in, want := range map[string]string{
		"0123-ABCD-4567":                       "0123abcd4567",
		" 0123 abcd 4567 ":                     "0123abcd4567",
		"0123abcd-4567-4def-8123-456789abcdef": "0123abcd45674def8123456789abcdef",
	} {
		if got, ok := normalizeRef(in); !ok || got != want {
			t.Fatalf("normalizeRef(%q) = %q,%v want %q", in, got, ok, want)
		}
	}
	for _, bad := range []string{"", "0123-ABCD-456", "0123-ABCD-4567-8", "0123-ABCD-456G", "0123_ABCD_4567", "'; DROP TABLE x;--"} {
		if _, ok := normalizeRef(bad); ok {
			t.Fatalf("normalizeRef(%q) must refuse", bad)
		}
	}
}

func TestContactDigest(t *testing.T) {
	sum := func(s string) [32]byte { return sha256.Sum256([]byte(s)) }
	for _, tc := range []struct {
		in, kind, norm string
	}{
		{"Buyer@Example.Test", "email", "buyer@example.test"},
		{"  buyer@example.test ", "email", "buyer@example.test"},
		{"0912-345-678", "phone", "912345678"},
		{"+886 912 345 678", "phone", "912345678"},
		{"(02) 2345-6789", "phone", "223456789"},
	} {
		kind, digest, ok := contactDigest(tc.in)
		if !ok || kind != tc.kind || digest != sum(tc.norm) {
			t.Fatalf("contactDigest(%q) = %s,%v", tc.in, kind, ok)
		}
	}
	for _, bad := range []string{"", "@x.y", "a@", "a@b@c", "a b@c.d", "12345", "123456789012345678901", "abc", "0912-345-67x", "a\x00@b.c"} {
		if _, _, ok := contactDigest(bad); ok {
			t.Fatalf("contactDigest(%q) must refuse", bad)
		}
	}
}

func TestLookupClientIP(t *testing.T) {
	ok := func(vals ...string) ([]byte, bool) {
		r := httptest.NewRequest(http.MethodPost, lookupPath, nil)
		for _, v := range vals {
			r.Header.Add("X-Commerce-Client-IP", v)
		}
		return lookupClientIP(r)
	}
	if h, good := ok(); h != nil || !good {
		t.Fatal("absent header is allowed (shared bucket)")
	}
	a, _ := ok("203.0.113.9")
	b, _ := ok("::ffff:203.0.113.9")
	if len(a) != 32 || string(a) != string(b) {
		t.Fatal("hash of the unmapped address")
	}
	for _, bad := range [][]string{{"1.1.1.1", "2.2.2.2"}, {"nope"}, {"fe80::1%eth0"}, {"1.1.1.1:80"}} {
		if _, good := ok(bad...); good {
			t.Fatalf("%v must be refused", bad)
		}
	}
}
