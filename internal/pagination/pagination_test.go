package pagination

import (
	"errors"
	"strings"
	"testing"

	"livecommerce/internal/command"
)

var binding = Binding{TenantID: "11111111-1111-4111-8111-111111111111", StoreID: "22222222-2222-4222-8222-222222222222", Collection: "products"}
var key = "33333333-3333-4333-8333-333333333333"

func TestRoundTripAndBinding(t *testing.T) {
	cursor, err := Encode(binding, []string{key})
	if err != nil {
		t.Fatal(err)
	}
	limit, after, err := Decode(Request{Cursor: cursor}, binding, 1)
	if err != nil || limit != 50 || len(after) != 1 || after[0] != key {
		t.Fatalf("limit=%d after=%v err=%v", limit, after, err)
	}
	wrong := binding
	wrong.Collection = "warehouses"
	if _, _, err := Decode(Request{Cursor: cursor}, wrong, 1); !errors.Is(err, command.ErrInvalid) {
		t.Fatalf("cross-collection err=%v", err)
	}
}

func TestRejectsBoundsAndMalformedCursor(t *testing.T) {
	for _, request := range []Request{{Limit: -1}, {Limit: 101}, {Cursor: "%%%"}, {Cursor: strings.Repeat("a", 1025)}} {
		if _, _, err := Decode(request, binding, 1); !errors.Is(err, command.ErrInvalid) {
			t.Fatalf("request=%+v err=%v", request, err)
		}
	}
}
