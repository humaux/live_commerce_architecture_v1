// Purpose: unit cases for the "claim-blocklist" collection used by internal/claims.ListBlockedActors (W3-05B): a store-level, two-key (created_at, id) newest-first cursor.
// Depends on: pagination.Encode/Decode. Used by: go test ./internal/pagination. Non-goal: SQL paging (covered by the real-PG blocklist gates).

package pagination

import (
	"errors"
	"testing"

	"livecommerce/internal/command"
)

func TestClaimBlocklistCursorIsStoreLevelTwoKey(t *testing.T) {
	list := Binding{TenantID: binding.TenantID, StoreID: binding.StoreID, Collection: "claim-blocklist"}
	stamp := "2026-10-06T01:02:03.000456Z"
	encoded, err := Encode(list, []string{stamp, key})
	if err != nil {
		t.Fatal(err)
	}
	if _, after, err := Decode(Request{Limit: 10, Cursor: encoded}, list, 2); err != nil || len(after) != 2 || after[0] != stamp || after[1] != key {
		t.Fatalf("roundtrip: %v %v", after, err)
	}
	// A parent id (session) or filter makes the binding invalid; another store or collection never accepts the position.
	for _, bad := range []Binding{
		{TenantID: binding.TenantID, StoreID: binding.StoreID, Collection: "claim-blocklist", ParentID: key},
		{TenantID: binding.TenantID, StoreID: binding.StoreID, Collection: "claim-blocklist", Filter: "x"},
	} {
		if _, err := Encode(bad, []string{stamp, key}); !errors.Is(err, command.ErrInvalid) {
			t.Fatalf("invalid binding accepted: %+v %v", bad, err)
		}
	}
	for _, other := range []Binding{
		{TenantID: binding.TenantID, StoreID: key, Collection: "claim-blocklist"},
		{TenantID: binding.TenantID, StoreID: binding.StoreID, Collection: "live-sessions"},
	} {
		if _, _, err := Decode(Request{Cursor: encoded}, other, 2); !errors.Is(err, command.ErrInvalid) {
			t.Fatalf("foreign cursor accepted for %+v: %v", other, err)
		}
	}
	if _, err := Encode(list, []string{key}); !errors.Is(err, command.ErrInvalid) {
		t.Fatalf("single-key position accepted: %v", err)
	}
}
