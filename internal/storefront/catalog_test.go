package storefront

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"livecommerce/internal/buyer"
	"livecommerce/internal/command"
)

func TestCatalogCursorCanonicalAndBound(t *testing.T) {
	scope := buyer.Scope{
		TenantID: "11111111-1111-1111-1111-111111111111",
		StoreID:  "22222222-2222-2222-2222-222222222222",
	}
	product := "33333333-3333-3333-3333-333333333333"
	after := "44444444-4444-4444-4444-444444444444"
	binding := catalogBinding(scope, product)
	if strings.Contains(binding, scope.TenantID) || strings.Contains(binding, scope.StoreID) || len(binding) != 64 {
		t.Fatal("binding must be a SHA256 digest, not raw scope")
	}
	encoded := encodeCatalogCursor(binding, after)
	got, err := decodeCatalogCursor(encoded, binding)
	if err != nil || got != after {
		t.Fatalf("canonical cursor: %q, %v", got, err)
	}
	if _, err := decodeCatalogCursor(encoded, catalogBinding(scope, "")); err != command.ErrInvalid {
		t.Fatal("cursor reused across product filters")
	}
	scope.StoreID = "55555555-5555-5555-5555-555555555555"
	if _, err := decodeCatalogCursor(encoded, catalogBinding(scope, product)); err != command.ErrInvalid {
		t.Fatal("cursor reused across stores")
	}
	bad := []string{
		"!", strings.Repeat("a", 1025), encoded + "=",
		`{"version":1,"binding":"` + binding + `","after":"` + after + `","after":"` + after + `"}`,
		`{"version":1,"binding":"` + binding + `","after":null}`,
		`{"version":1,"binding":"` + binding + `","after":"` + after + `","extra":1}`,
		`{"binding":"` + binding + `","version":1,"after":"` + after + `"}`,
		`{"version":2,"binding":"` + binding + `","after":"` + after + `"}`,
		`{"version":1,"binding":"` + binding + `","after":"ABC"}`,
		`{"version":1,"binding":"` + binding + `","after":"` + after + `"} {}`,
	}
	for _, value := range bad {
		if strings.HasPrefix(value, "{") {
			value = base64.RawURLEncoding.EncodeToString([]byte(value))
		}
		if _, err := decodeCatalogCursor(value, binding); err != command.ErrInvalid {
			t.Fatalf("accepted malformed cursor %q: %v", value, err)
		}
	}
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || !json.Valid(raw) {
		t.Fatal("encoded cursor is not JSON")
	}
}
