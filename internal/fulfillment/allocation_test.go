package fulfillment

import (
	"math"
	"testing"
)

const allocationWarehouse = "22222222-2222-4222-8222-222222222222"

func TestValidAllocationInput(t *testing.T) {
	t.Parallel()
	base := AllocationInput{MarketID: testID, Country: "TW", Code: "home", ExpectedServiceVersion: 1}
	if !validAllocationInput(base) {
		t.Fatal("empty priority list must be allowed")
	}
	base.WarehouseIDs = []string{allocationWarehouse, testID}
	if !validAllocationInput(base) {
		t.Fatal("ordered distinct warehouses rejected")
	}
	cases := []struct {
		name string
		edit func(*AllocationInput)
	}{
		{"market", func(v *AllocationInput) { v.MarketID = "bad" }},
		{"country", func(v *AllocationInput) { v.Country = "tw" }},
		{"code", func(v *AllocationInput) { v.Code = "Bad" }},
		{"negative version", func(v *AllocationInput) { v.ExpectedVersion = -1 }},
		{"max version", func(v *AllocationInput) { v.ExpectedVersion = math.MaxInt64 }},
		{"missing service", func(v *AllocationInput) { v.ExpectedServiceVersion = 0 }},
		{"duplicate", func(v *AllocationInput) { v.WarehouseIDs = []string{testID, testID} }},
		{"malformed warehouse", func(v *AllocationInput) { v.WarehouseIDs = []string{"bad"} }},
		{"too many", func(v *AllocationInput) { v.WarehouseIDs = make([]string, 17) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := base
			tc.edit(&v)
			if validAllocationInput(v) {
				t.Fatal("invalid allocation accepted")
			}
		})
	}
}
