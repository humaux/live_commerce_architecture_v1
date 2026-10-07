// Purpose: unit cases for the "operations" collection used by internal/integrations/core.ListLedger (W6-05B): a store-level two-key (created_at, id) newest-first cursor bound to one state filter.
// Depends on: pagination.Encode/Decode. Used by: go test ./internal/pagination. Non-goal: SQL paging (covered by the real-PG operations-queue gate).

package pagination

import (
	"errors"
	"testing"

	"livecommerce/internal/command"
)

func TestOperationsCursorIsBoundToItsStateFilter(t *testing.T) {
	list := Binding{TenantID: binding.TenantID, StoreID: binding.StoreID, Collection: "operations", Filter: "UNKNOWN"}
	stamp := "2026-10-07T01:02:03.000456Z"
	encoded, err := Encode(list, []string{stamp, key})
	if err != nil {
		t.Fatal(err)
	}
	if _, after, err := Decode(Request{Limit: 10, Cursor: encoded}, list, 2); err != nil || len(after) != 2 || after[0] != stamp || after[1] != key {
		t.Fatalf("roundtrip: %v %v", after, err)
	}
	// Another filter, store, collection or an open/free-text filter never accepts the position.
	for _, other := range []Binding{
		{TenantID: binding.TenantID, StoreID: binding.StoreID, Collection: "operations", Filter: "attention"},
		{TenantID: binding.TenantID, StoreID: key, Collection: "operations", Filter: "UNKNOWN"},
		{TenantID: binding.TenantID, StoreID: binding.StoreID, Collection: "claim-blocklist"},
	} {
		if _, _, err := Decode(Request{Cursor: encoded}, other, 2); !errors.Is(err, command.ErrInvalid) {
			t.Fatalf("foreign cursor accepted for %+v: %v", other, err)
		}
	}
	for _, bad := range []Binding{
		{TenantID: binding.TenantID, StoreID: binding.StoreID, Collection: "operations"},
		{TenantID: binding.TenantID, StoreID: binding.StoreID, Collection: "operations", Filter: "SUCCEEDED"},
		{TenantID: binding.TenantID, StoreID: binding.StoreID, Collection: "operations", Filter: "UNKNOWN", ParentID: key},
	} {
		if _, err := Encode(bad, []string{stamp, key}); !errors.Is(err, command.ErrInvalid) {
			t.Fatalf("invalid binding accepted: %+v %v", bad, err)
		}
	}
	if _, err := Encode(list, []string{key}); !errors.Is(err, command.ErrInvalid) {
		t.Fatalf("single-key position accepted: %v", err)
	}
}
