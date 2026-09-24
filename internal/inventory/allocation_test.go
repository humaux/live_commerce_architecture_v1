package inventory

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"livecommerce/internal/command"
)

const idC = "33333333-3333-4333-8333-333333333333"

func TestPlanAllocationPriorityAndConservation(t *testing.T) {
	t.Parallel()
	warehouses := []string{idB, idA}
	demands := []Demand{{SKUID: idC, Quantity: 6}, {SKUID: idA, Quantity: 2}}
	balances := []Balance{
		{WarehouseID: idA, SKUID: idC, OnHand: 5, Reserved: 1, Available: -99},
		{WarehouseID: idB, SKUID: idC, OnHand: 3, Available: -99},
		{WarehouseID: idB, SKUID: idA, OnHand: 2, Available: -99},
	}
	before := append([]Balance(nil), balances...)
	plan, err := PlanAllocation(warehouses, demands, balances)
	want := []Line{
		{WarehouseID: idA, SKUID: idC, Quantity: 3},
		{WarehouseID: idB, SKUID: idA, Quantity: 2},
		{WarehouseID: idB, SKUID: idC, Quantity: 3},
	}
	if err != nil || !reflect.DeepEqual(plan, want) {
		t.Fatalf("plan=%+v err=%v, want=%+v", plan, err, want)
	}
	if !reflect.DeepEqual(balances, before) || !reflect.DeepEqual(warehouses, []string{idB, idA}) {
		t.Fatal("input mutated")
	}
	reversed, err := PlanAllocation(warehouses, []Demand{demands[1], demands[0]}, []Balance{balances[2], balances[0], balances[1]})
	if err != nil || !reflect.DeepEqual(reversed, want) {
		t.Fatalf("permutation changed plan: %+v, %v", reversed, err)
	}
}

func TestPlanAllocationShortReturnsNoPartialPlan(t *testing.T) {
	t.Parallel()
	plan, err := PlanAllocation([]string{idA}, []Demand{{SKUID: idA, Quantity: 1}, {SKUID: idB, Quantity: 2}},
		[]Balance{{WarehouseID: idA, SKUID: idA, OnHand: 1}})
	if plan != nil || !errors.Is(err, command.ErrInsufficient) {
		t.Fatalf("partial plan=%+v err=%v", plan, err)
	}
}

func TestPlanAllocationInvalid(t *testing.T) {
	t.Parallel()
	baseWarehouses := []string{idA}
	baseDemand := []Demand{{SKUID: idB, Quantity: 1}}
	baseBalances := []Balance{{WarehouseID: idA, SKUID: idB, OnHand: 1}}
	cases := []struct {
		name       string
		warehouses []string
		demands    []Demand
		balances   []Balance
	}{
		{"no warehouses", nil, baseDemand, baseBalances},
		{"duplicate warehouses", []string{idA, idA}, baseDemand, baseBalances},
		{"bad warehouse", []string{"bad"}, baseDemand, baseBalances},
		{"no demands", baseWarehouses, nil, baseBalances},
		{"duplicate SKU", baseWarehouses, []Demand{{idB, 1}, {idB, 1}}, baseBalances},
		{"zero demand", baseWarehouses, []Demand{{idB, 0}}, baseBalances},
		{"oversized demand", baseWarehouses, []Demand{{idB, command.MaxQuantity + 1}}, baseBalances},
		{"bad SKU", baseWarehouses, []Demand{{"bad", 1}}, baseBalances},
		{"duplicate row", baseWarehouses, baseDemand, []Balance{baseBalances[0], baseBalances[0]}},
		{"foreign warehouse", baseWarehouses, baseDemand, []Balance{{WarehouseID: idC, SKUID: idB, OnHand: 1}}},
		{"extraneous SKU", baseWarehouses, baseDemand, []Balance{{WarehouseID: idA, SKUID: idC, OnHand: 1}}},
		{"negative stock", baseWarehouses, baseDemand, []Balance{{WarehouseID: idA, SKUID: idB, OnHand: -1}}},
		{"oversized stock", baseWarehouses, baseDemand, []Balance{{WarehouseID: idA, SKUID: idB, OnHand: command.MaxQuantity + 1}}},
		{"overcommitted", baseWarehouses, baseDemand, []Balance{{WarehouseID: idA, SKUID: idB, OnHand: 1, Reserved: 1, Allocated: 1}}},
		{"negative version", baseWarehouses, baseDemand, []Balance{{WarehouseID: idA, SKUID: idB, OnHand: 1, Version: -1}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan, err := PlanAllocation(tc.warehouses, tc.demands, tc.balances)
			if plan != nil || !errors.Is(err, command.ErrInvalid) {
				t.Fatalf("plan=%+v err=%v", plan, err)
			}
		})
	}
}

func TestPlanAllocationEightHundredLines(t *testing.T) {
	t.Parallel()
	warehouses := make([]string, 16)
	demands := make([]Demand, 50)
	balances := make([]Balance, 0, 800)
	for i := range warehouses {
		warehouses[i] = fmt.Sprintf("%08x-1111-4111-8111-111111111111", i+1)
	}
	for i := range demands {
		demands[i] = Demand{SKUID: fmt.Sprintf("%08x-2222-4222-8222-222222222222", i+1), Quantity: 16}
		for _, warehouse := range warehouses {
			balances = append(balances, Balance{WarehouseID: warehouse, SKUID: demands[i].SKUID, OnHand: 1})
		}
	}
	plan, err := PlanAllocation(warehouses, demands, balances)
	if err != nil || len(plan) != 800 {
		t.Fatalf("lines=%d err=%v", len(plan), err)
	}
	for i := 1; i < len(plan); i++ {
		if plan[i-1].WarehouseID > plan[i].WarehouseID ||
			(plan[i-1].WarehouseID == plan[i].WarehouseID && plan[i-1].SKUID >= plan[i].SKUID) {
			t.Fatalf("not sorted at %d", i)
		}
	}
	_, err = PlanAllocation(warehouses, demands, append(balances, balances[0]))
	if !errors.Is(err, command.ErrInvalid) {
		t.Fatalf("801 rows accepted: %v", err)
	}
}

func FuzzPlanAllocation(f *testing.F) {
	f.Add(int64(1), int64(1), int64(0))
	f.Add(command.MaxQuantity, command.MaxQuantity, int64(0))
	f.Add(int64(2), int64(1), int64(1))
	f.Fuzz(func(t *testing.T, requested, onHand, reserved int64) {
		plan, err := PlanAllocation([]string{idA}, []Demand{{SKUID: idB, Quantity: requested}},
			[]Balance{{WarehouseID: idA, SKUID: idB, OnHand: onHand, Reserved: reserved, Available: command.MaxQuantity}})
		if err != nil {
			if plan != nil {
				t.Fatal("error returned partial plan")
			}
			return
		}
		if len(plan) != 1 || plan[0].Quantity != requested || requested < 1 || requested > command.MaxQuantity ||
			onHand < 0 || onHand > command.MaxQuantity || reserved < 0 || reserved > onHand || requested > onHand-reserved {
			t.Fatalf("invalid successful plan: %+v", plan)
		}
	})
}
