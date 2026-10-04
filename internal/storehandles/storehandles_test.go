// Operator grammar stays unchanged; random assignment/retry/backfill are real PG gates.
package storehandles

import (
	"strings"
	"testing"
)

func TestValid(t *testing.T) {
	for _, tc := range []struct {
		in string
		ok bool
	}{
		{"abc", true},
		{"my-store", true},
		{"store-123", true},
		{"123", true},
		{"a-b", true},
		{"ab", false},                    // too short
		{strings.Repeat("a", 30), true},  // max
		{strings.Repeat("a", 31), false}, // too long
		{"ABC", false},                   // upper-case
		{"-abc", false},                  // leading hyphen
		{"abc-", false},                  // trailing hyphen
		{"ab--cd", true},                 // consecutive hyphens admitted (parity with the DB regex)
		{"a c", false},                   // space
		{"商店", false},                    // non-ASCII
		{"www", false},                   // reserved
		{"admin", false},                 // reserved
		{"stores", false},                // reserved
		{"xn--abc", false},               // punycode
		{"a.b", false},                   // dot
	} {
		if got := Valid(tc.in); got != tc.ok {
			t.Errorf("Valid(%q) = %v, want %v", tc.in, got, tc.ok)
		}
	}
}

func TestReserved(t *testing.T) {
	for _, w := range []string{"www", "admin", "api", "hooks", "shop", "mail", "static", "cdn", "assets", "app", "help", "support", "status", "stores", "xn--", "xn--abc"} {
		if !Reserved(w) {
			t.Errorf("Reserved(%q) = false, want true", w)
		}
	}
	for _, w := range []string{"shopify", "store", "my-shop", "xstore", "abc"} {
		if Reserved(w) {
			t.Errorf("Reserved(%q) = true, want false", w)
		}
	}
}
