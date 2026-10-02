// storehandles_test.go: pure-logic unit tests for the handle grammar (Decision 1). They lock Go/SQL parity for the
// slug, the reserved list, the store-<id8> fallback and the suggestion truncation; the DB helpers (uniqueness, suffix
// on committed collision, backfill) are proven against real PG by the gate suite.

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

func TestSlug(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"Hello World", "hello-world"},
		{"  Foo  Bar  ", "foo-bar"},
		{"Foo__Bar", "foo-bar"},
		{"a  b", "a-b"},
		{"!!!hello!!!", "hello"},
		{"商店", ""},
		{"Café Bar", "caf-bar"},
		{"Already-Lower", "already-lower"},
		{"-leading", "leading"},
		{"trailing-", "trailing"},
		{"ACME 123", "acme-123"},
		{"", ""},
	} {
		if got := Slug(tc.in); got != tc.want {
			t.Errorf("Slug(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestFallback(t *testing.T) {
	id := "3f1b0c9e-5a77-4d1e-9d2a-0a7f4c2b9e11"
	if got := Fallback(id); got != "store-3f1b0c9e" {
		t.Errorf("Fallback(%q) = %q, want store-3f1b0c9e", id, got)
	}
	if got := Fallback("ABCDEF00-1111-4111-8111-111111111111"); got != "store-abcdef00" {
		t.Errorf("Fallback upper-case = %q, want store-abcdef00", got)
	}
	for _, bad := range []string{"", "abc", "1234567", "zzzz-zzzz-zzzz-zzzz-zzzzzzzzzzzz"} {
		if got := Fallback(bad); got != "" {
			t.Errorf("Fallback(%q) = %q, want empty", bad, got)
		}
	}
}

func TestSuggest(t *testing.T) {
	id := "3f1b0c9e-5a77-4d1e-9d2a-0a7f4c2b9e11"
	for _, tc := range []struct{ name, id, want string }{
		{"My Store", id, "my-store"},
		{"商店", id, "store-3f1b0c9e"},    // non-ASCII name falls back
		{"Admin", id, "store-3f1b0c9e"}, // reserved slug falls back
		{"  ", id, "store-3f1b0c9e"},    // empty slug falls back
		{"Foo", id, "foo"},
		{"ab", id, "store-3f1b0c9e"},                           // too-short slug falls back
		{strings.Repeat("a", 35), id, strings.Repeat("a", 30)}, // long slug is truncated, not fallen back (SQL parity)
		{"商店", "", ""},                                         // no slug and no id -> no suggestion
	} {
		if got := Suggest(tc.name, tc.id); got != tc.want {
			t.Errorf("Suggest(%q, %q) = %q, want %q", tc.name, tc.id, got, tc.want)
		}
	}
}
