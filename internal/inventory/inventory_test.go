package inventory

import (
	"errors"
	"testing"

	"livecommerce/internal/command"
)

const idA = "11111111-1111-4111-8111-111111111111"
const idB = "22222222-2222-4222-8222-222222222222"

func TestCanonicalLinesMergeSortAndOverflow(t *testing.T) {
	lines, err := canonicalLines([]Line{{WarehouseID: idB, SKUID: idA, Quantity: 2}, {WarehouseID: idA, SKUID: idB, Quantity: 1}, {WarehouseID: idB, SKUID: idA, Quantity: 3}})
	if err != nil || len(lines) != 2 || lines[0].WarehouseID != idA || lines[1].Quantity != 5 {
		t.Fatalf("canonical=%+v err=%v", lines, err)
	}
	_, err = canonicalLines([]Line{{WarehouseID: idA, SKUID: idB, Quantity: command.MaxQuantity}, {WarehouseID: idA, SKUID: idB, Quantity: 1}})
	if !errors.Is(err, command.ErrInvalid) {
		t.Fatalf("overflow err=%v", err)
	}
}

func TestAdjustmentCapacity(t *testing.T) {
	b := Balance{OnHand: 10, Reserved: 3, Allocated: 2, Unavailable: 1}
	if !canAdjust(b, -4) || canAdjust(b, -5) || !canAdjust(b, 1) {
		t.Fatal("adjustment capacity mismatch")
	}
	if validAdjustment(Adjustment{WarehouseID: idA, SKUID: idB, Delta: 0, Reason: "x"}) {
		t.Fatal("zero adjustment accepted")
	}
}

func FuzzCanonicalLines(f *testing.F) {
	f.Add(int64(1), int64(2))
	f.Add(command.MaxQuantity, int64(1))
	f.Fuzz(func(t *testing.T, first, second int64) {
		_, _ = canonicalLines([]Line{{WarehouseID: idA, SKUID: idB, Quantity: first}, {WarehouseID: idA, SKUID: idB, Quantity: second}})
	})
}
