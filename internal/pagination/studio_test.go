package pagination

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"livecommerce/internal/command"
)

func TestStudioCursorTwoKeyScopeAndCanonicalShape(t *testing.T) {
	studio := Binding{TenantID: binding.TenantID, StoreID: binding.StoreID, Collection: "live-sessions"}
	stamp := "2026-09-27T02:03:04.123456Z"
	encoded, err := Encode(studio, []string{stamp, key})
	if err != nil {
		t.Fatal(err)
	}
	limit, after, err := Decode(Request{Limit: 2, Cursor: encoded}, studio, 2)
	if err != nil || limit != 2 || len(after) != 2 || after[0] != stamp || after[1] != key {
		t.Fatalf("Studio two-key roundtrip: limit=%d keys=%v err=%v", limit, after, err)
	}
	for _, changed := range []Binding{
		{TenantID: binding.TenantID, StoreID: "44444444-4444-4444-8444-444444444444", Collection: "live-sessions"},
		{TenantID: "55555555-5555-4555-8555-555555555555", StoreID: binding.StoreID, Collection: "live-sessions"},
		{TenantID: binding.TenantID, StoreID: binding.StoreID, Collection: "merchant-orders", Filter: "all"},
	} {
		if _, _, err := Decode(Request{Cursor: encoded}, changed, 2); !errors.Is(err, command.ErrInvalid) {
			t.Fatalf("cross-scope cursor accepted: %+v err=%v", changed, err)
		}
	}
	if _, err := Encode(studio, []string{key}); !errors.Is(err, command.ErrInvalid) {
		t.Fatalf("one-key Studio cursor accepted: %v", err)
	}
	for _, invalid := range [][]string{{stamp, "invalid"}, {"2026-09-27T02:03:04Z", key}, {"2026-09-27T02:03:04.123456+00:00", key}} {
		if _, err := Encode(studio, invalid); !errors.Is(err, command.ErrInvalid) {
			t.Fatalf("invalid Studio keys accepted: %v %v", invalid, err)
		}
	}
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	for _, malformed := range []string{
		encoded + "=", encoded + "\n", base64.RawURLEncoding.EncodeToString(append([]byte(" "), raw...)),
		base64.RawURLEncoding.EncodeToString([]byte(strings.Replace(string(raw), `"version":1`, `"version":1,"version":1`, 1))),
	} {
		if _, _, err := Decode(Request{Cursor: malformed}, studio, 2); !errors.Is(err, command.ErrInvalid) {
			t.Fatalf("noncanonical Studio cursor accepted: %q %v", malformed, err)
		}
	}
	// Existing merchant-order cursor semantics remain unchanged.
	orders := Binding{TenantID: binding.TenantID, StoreID: binding.StoreID, Collection: "merchant-orders", Filter: "all"}
	old, err := Encode(orders, []string{stamp, key})
	if err != nil {
		t.Fatal(err)
	}
	_, oldKeys, err := Decode(Request{Cursor: old}, orders, 2)
	if err != nil || len(oldKeys) != 2 || oldKeys[0] != stamp {
		t.Fatalf("merchant-order cursor regression: keys=%v err=%v", oldKeys, err)
	}
}
