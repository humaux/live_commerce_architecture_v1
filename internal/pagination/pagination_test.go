package pagination

import (
	"encoding/base64"
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

func TestMerchantOrdersTimestampIDCursor(t *testing.T) {
	b := Binding{TenantID: binding.TenantID, StoreID: binding.StoreID, Collection: "merchant-orders", Filter: "all"}
	stamp := "2026-09-25T04:05:06.123456Z"
	encoded, err := Encode(b, []string{stamp, key})
	if err != nil {
		t.Fatal(err)
	}
	limit, keys, err := Decode(Request{Limit: 100, Cursor: encoded}, b, 2)
	if err != nil || limit != 100 || len(keys) != 2 || keys[0] != stamp || keys[1] != key {
		t.Fatalf("limit=%d keys=%v err=%v", limit, keys, err)
	}
	for _, change := range []Binding{
		{TenantID: binding.TenantID, StoreID: "44444444-4444-4444-8444-444444444444", Collection: "merchant-orders", Filter: "all"},
		{TenantID: binding.TenantID, StoreID: binding.StoreID, Collection: "merchant-orders", Filter: "DRAFT"},
		binding,
	} {
		if _, _, err := Decode(Request{Cursor: encoded}, change, 2); !errors.Is(err, command.ErrInvalid) {
			t.Fatalf("cross-binding accepted: %+v %v", change, err)
		}
	}
	for _, bad := range [][]string{
		{stamp}, {stamp, "NOT-A-UUID"}, {"2026-09-25T04:05:06Z", key},
		{"2026-09-25T04:05:06.123456+00:00", key}, {"2026-09-25T04:05:06.1234567Z", key},
	} {
		if _, err := Encode(b, bad); !errors.Is(err, command.ErrInvalid) {
			t.Fatalf("invalid keys accepted: %v %v", bad, err)
		}
	}
	if _, err := Encode(Binding{TenantID: binding.TenantID, StoreID: binding.StoreID, Collection: "merchant-orders"}, []string{stamp, key}); !errors.Is(err, command.ErrInvalid) {
		t.Fatalf("missing state accepted: %v", err)
	}
	decoded, _ := base64.RawURLEncoding.DecodeString(encoded)
	for _, malformed := range []string{
		encoded + "=", encoded + "\n", base64.RawURLEncoding.EncodeToString(append([]byte(" "), decoded...)),
	} {
		if _, _, err := Decode(Request{Cursor: malformed}, b, 2); !errors.Is(err, command.ErrInvalid) {
			t.Fatalf("noncanonical cursor accepted: %q %v", malformed, err)
		}
	}
}
