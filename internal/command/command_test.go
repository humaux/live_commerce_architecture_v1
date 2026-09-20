package command

import (
	"math"
	"testing"
)

func TestCheckMoney(t *testing.T) {
	for _, tt := range []struct {
		unit, qty, want int64
		valid           bool
	}{
		{0, 1, 0, true}, {1, 1, 1, true}, {MaxMoney, 1, MaxMoney, true}, {10, 20, 200, true},
		{-1, 1, 0, false}, {1, 0, 0, false}, {1, -1, 0, false}, {MaxMoney, 2, 0, false},
		{math.MaxInt64, 2, 0, false}, {1, math.MaxInt64, 0, false}, {0, MaxQuantity + 1, 0, false},
	} {
		got, err := CheckMoney(tt.unit, tt.qty)
		if (err == nil) != tt.valid || got != tt.want {
			t.Fatalf("%+v got=%d err=%v", tt, got, err)
		}
	}
}

func FuzzCheckMoney(f *testing.F) {
	f.Add(int64(20), int64(10))
	f.Add(int64(math.MaxInt64), int64(2))
	f.Fuzz(func(t *testing.T, u, q int64) {
		got, err := CheckMoney(u, q)
		if err == nil && (got < 0 || got > MaxMoney || q < 1 || q > MaxQuantity || u < 0 || u > MaxMoney || got/q != u) {
			t.Fatal("money invariant")
		}
	})
}

func TestIDValidation(t *testing.T) {
	if !ValidID("11111111-1111-4111-8111-111111111111") {
		t.Fatal("canonical UUID rejected")
	}
	for _, s := range []string{"", "../other", "11111111111141118111111111111111", "AAAAAAAA-AAAA-AAAA-AAAA-AAAAAAAAAAAA"} {
		if ValidID(s) {
			t.Fatalf("accepted %q", s)
		}
	}
}

func TestResultMustBeNonNilPointer(t *testing.T) {
	var nilPointer *struct{ ID string }
	for _, v := range []any{nil, nilPointer, struct{ ID string }{}, "text", 42} {
		if resultPointer(v) {
			t.Fatalf("accepted invalid result %T", v)
		}
	}
	if !resultPointer(&struct{ ID string }{}) {
		t.Fatal("rejected result pointer")
	}
}
