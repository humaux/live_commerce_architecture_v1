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

func TestProviderAccountCursorIsScoped(t *testing.T) {
	accountBinding := Binding{TenantID: binding.TenantID, StoreID: binding.StoreID, Collection: "provider-accounts"}
	encoded, err := Encode(accountBinding, []string{key})
	if err != nil {
		t.Fatal(err)
	}
	for _, changed := range []Binding{
		{TenantID: binding.TenantID, StoreID: "44444444-4444-4444-8444-444444444444", Collection: "provider-accounts"},
		binding,
	} {
		if _, _, err := Decode(Request{Cursor: encoded}, changed, 1); !errors.Is(err, command.ErrInvalid) {
			t.Fatalf("cross-scope cursor accepted: %+v", changed)
		}
	}
	if _, keys, err := Decode(Request{Cursor: encoded}, accountBinding, 1); err != nil || len(keys) != 1 || keys[0] != key {
		t.Fatalf("account cursor roundtrip: keys=%v err=%v", keys, err)
	}
}

func TestDeliveryCodeCursorIsBoundToMarketAndCountry(t *testing.T) {
	delivery := Binding{TenantID: binding.TenantID, StoreID: binding.StoreID, Collection: "delivery-services", ParentID: key, Filter: "TW"}
	encoded, err := Encode(delivery, []string{"manual_home"})
	if err != nil {
		t.Fatal(err)
	}
	if _, keys, err := Decode(Request{Cursor: encoded}, delivery, 1); err != nil || len(keys) != 1 || keys[0] != "manual_home" {
		t.Fatalf("delivery cursor: keys=%v err=%v", keys, err)
	}
	for _, changed := range []Binding{
		{TenantID: binding.TenantID, StoreID: binding.StoreID, Collection: "delivery-services", ParentID: "44444444-4444-4444-8444-444444444444", Filter: "TW"},
		{TenantID: binding.TenantID, StoreID: binding.StoreID, Collection: "delivery-services", ParentID: key, Filter: "US"},
		{TenantID: binding.TenantID, StoreID: binding.StoreID, Collection: "markets"},
	} {
		if _, _, err := Decode(Request{Cursor: encoded}, changed, 1); !errors.Is(err, command.ErrInvalid) {
			t.Fatalf("cross-binding cursor accepted: %+v", changed)
		}
	}
	for _, bad := range []string{"UPPER", "1leading", "bad:code", strings.Repeat("a", 41), key} {
		if _, err := Encode(delivery, []string{bad}); !errors.Is(err, command.ErrInvalid) {
			t.Fatalf("invalid delivery code %q accepted: %v", bad, err)
		}
	}
	if _, err := Encode(delivery, []string{"a", "b"}); !errors.Is(err, command.ErrInvalid) {
		t.Fatalf("two-key delivery cursor accepted: %v", err)
	}
	if _, err := Encode(binding, []string{"manual_home"}); !errors.Is(err, command.ErrInvalid) {
		t.Fatalf("old collection accepted code cursor: %v", err)
	}
}
