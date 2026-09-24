package payuni

import (
	"errors"
	"math"
	"testing"
)

func TestAmountTWDFromMinorPreservesExactValue(t *testing.T) {
	for _, pair := range []struct{ minor, yuan int64 }{{100, 1}, {300, 3}, {30000, 300}, {19999900, 199999}, {math.MaxInt64 - 7, (math.MaxInt64 - 7) / 100}} {
		got, err := AmountTWDFromMinor("TWD", pair.minor)
		if err != nil || got != pair.yuan || got*100 != pair.minor {
			t.Fatalf("minor=%d: %d, %v", pair.minor, got, err)
		}
	}
	for _, currency := range []string{"USD", "JPY", "twd", "", "TWD "} {
		if _, err := AmountTWDFromMinor(currency, 30000); !errors.Is(err, ErrInvalid) {
			t.Fatalf("wrong currency accepted: %q", currency)
		}
	}
	for _, amount := range []int64{-100, 0, 1, 99, 101, 30001, math.MaxInt64} {
		if _, err := AmountTWDFromMinor("TWD", amount); !errors.Is(err, ErrInvalid) {
			t.Fatalf("non-exact positive TWD accepted: %d", amount)
		}
	}
	// Conversion is not a provider-limit grant. The form builder still refuses
	// exact values above the selected method's limit.
	c := wireClient(t)
	in := wireRequest()
	in.AmountTWD, _ = AmountTWDFromMinor("TWD", 20000000)
	if _, err := c.BuildHosted(in); !errors.Is(err, ErrInvalid) {
		t.Fatal("conversion bypassed method limit", err)
	}
}
