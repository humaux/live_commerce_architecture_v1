package pricing

import (
	"errors"
	"math/rand"
	"testing"

	"livecommerce/internal/command"
)

func promoPolicy(mode string) Policy {
	// USD: step 1, so the generic math below is exact; the TWD whole-dollar rule has its own test.
	return Policy{MarketID: "11111111-1111-4111-8111-111111111111", Country: "TW", Method: "home", Currency: "USD", ShippingMode: "country_flat",
		TaxMode: mode, TaxBasis: "goods", Version: 1, ShippingMinor: 6000, TaxRateBPS: 500, QuoteTTLSeconds: 600, Enabled: true}
}

// storefront-v2 §F: the discount comes off goods only; shipping is untouched, tax follows the discounted goods, the totals reconcile exactly
// the way merchantorders' order validator re-derives them (line total = subtotal - discount [+ tax]; total = subtotal - discount + shipping [+ tax]).
func TestCalculateWithPromoMath(t *testing.T) {
	lines := []AmountLine{{UnitPriceMinor: 3333, Quantity: 3}, {UnitPriceMinor: 5001, Quantity: 1}} // 9999 + 5001 = 15000
	for _, c := range []struct {
		name     string
		promo    *Promo
		discount int64
	}{
		{"none", nil, 0},
		{"10 percent", &Promo{Kind: "percent", Percent: 10}, 1500},
		{"floor rounding", &Promo{Kind: "percent", Percent: 33}, 4950},
		{"90 percent cap", &Promo{Kind: "percent", Percent: 90}, 13500},
		{"fixed below subtotal", &Promo{Kind: "fixed", FixedMinor: 777}, 777},
		{"fixed above subtotal is capped, never negative", &Promo{Kind: "fixed", FixedMinor: 99999999}, 15000},
	} {
		for _, mode := range []string{"none", "exclusive", "inclusive"} {
			p := promoPolicy(mode)
			if mode == "none" {
				p.TaxRateBPS = 0
			}
			got, err := CalculateWith(p, lines, c.promo)
			if err != nil {
				t.Fatalf("%s/%s: %v", c.name, mode, err)
			}
			if got.SubtotalMinor != 15000 || got.DiscountMinor != c.discount || got.ShippingMinor != 6000 {
				t.Fatalf("%s/%s: subtotal %d discount %d shipping %d", c.name, mode, got.SubtotalMinor, got.DiscountMinor, got.ShippingMinor)
			}
			var sumDiscount, sumTax int64
			for i, l := range got.Lines {
				if l.SubtotalMinor != lines[i].UnitPriceMinor*lines[i].Quantity || l.DiscountMinor < 0 || l.DiscountMinor > l.SubtotalMinor {
					t.Fatalf("%s/%s: line %d out of range %+v", c.name, mode, i, l)
				}
				want := l.SubtotalMinor - l.DiscountMinor
				if mode == "exclusive" {
					want += l.TaxMinor
				}
				if l.TotalMinor != want {
					t.Fatalf("%s/%s: line %d total %d want %d", c.name, mode, i, l.TotalMinor, want)
				}
				sumDiscount += l.DiscountMinor
				sumTax += l.TaxMinor
			}
			if sumDiscount != got.DiscountMinor || sumTax != got.TaxMinor {
				t.Fatalf("%s/%s: lines discount %d tax %d vs aggregate %d/%d", c.name, mode, sumDiscount, sumTax, got.DiscountMinor, got.TaxMinor)
			}
			want := got.SubtotalMinor - got.DiscountMinor + got.ShippingMinor
			if mode == "exclusive" {
				want += got.TaxMinor
			}
			if got.TotalMinor != want {
				t.Fatalf("%s/%s: total %d want %d", c.name, mode, got.TotalMinor, want)
			}
		}
	}
}

// Free shipping compares the PRE-discount subtotal (§F): a 90 percent code must not take away shipping the cart qualified for.
func TestCalculateWithPromoKeepsFreeShipping(t *testing.T) {
	p := promoPolicy("exclusive")
	threshold := int64(15000)
	p.FreeShippingThresholdMinor = &threshold
	got, err := CalculateWith(p, []AmountLine{{UnitPriceMinor: 15000, Quantity: 1}}, &Promo{Kind: "percent", Percent: 90})
	if err != nil || got.ShippingMinor != 0 || got.DiscountMinor != 13500 {
		t.Fatalf("free shipping with a code: %+v %v", got, err)
	}
}

// With no code CalculateWith must equal the historical Calculate result (the pre-0091 quote, snapshot and validators depend on it).
func TestCalculateWithNilPromoEqualsCalculate(t *testing.T) {
	r := rand.New(rand.NewSource(7))
	for i := 0; i < 200; i++ {
		n := 1 + r.Intn(5)
		lines := make([]AmountLine, n)
		for j := range lines {
			lines[j] = AmountLine{UnitPriceMinor: 1 + r.Int63n(100000), Quantity: 1 + r.Int63n(9)}
		}
		for _, mode := range []string{"exclusive", "inclusive"} {
			a, errA := Calculate(promoPolicy(mode), lines)
			b, errB := CalculateWith(promoPolicy(mode), lines, nil)
			if errA != nil || errB != nil || a.TotalMinor != b.TotalMinor || a.DiscountMinor != 0 || len(a.Lines) != len(b.Lines) {
				t.Fatalf("calc mismatch %+v %+v", a, b)
			}
		}
	}
}

// The line allocation always sums to the aggregate and never exceeds a line, including at the 1e12 money ceiling where a naive
// discount*line product overflows int64.
func TestAllocateDiscountInvariants(t *testing.T) {
	r := rand.New(rand.NewSource(11))
	for i := 0; i < 2000; i++ {
		n := 1 + r.Intn(50)
		subs := make([]int64, n)
		var total int64
		for j := range subs {
			subs[j] = r.Int63n(command.MaxMoney / 100)
			total += subs[j]
		}
		if total == 0 {
			continue
		}
		discount := r.Int63n(total + 1)
		got := allocateDiscount(discount, subs, total)
		var sum int64
		for j, share := range got {
			if share < 0 || share > subs[j] {
				t.Fatalf("share %d out of [0,%d]", share, subs[j])
			}
			sum += share
		}
		if sum != discount {
			t.Fatalf("allocation sums to %d, want %d", sum, discount)
		}
	}
	// A tie goes to the lower index: 100 off two equal lines of 3 -> remainder unit lands on line 0 first.
	if got := allocateDiscount(3, []int64{5, 5}, 10); got[0] != 2 || got[1] != 1 {
		t.Fatalf("tie allocation %v", got)
	}
}

func TestPromoRejectsMalformedEffects(t *testing.T) {
	lines := []AmountLine{{UnitPriceMinor: 1000, Quantity: 1}}
	for name, promo := range map[string]*Promo{
		"unknown kind":     {Kind: "bogo"},
		"percent zero":     {Kind: "percent", Percent: 0},
		"percent over 90":  {Kind: "percent", Percent: 91},
		"percent with fix": {Kind: "percent", Percent: 10, FixedMinor: 5},
		"fixed zero":       {Kind: "fixed", FixedMinor: 0},
		"fixed over max":   {Kind: "fixed", FixedMinor: command.MaxMoney + 1},
		"fixed with pct":   {Kind: "fixed", FixedMinor: 5, Percent: 10},
	} {
		if _, err := CalculateWith(promoPolicy("exclusive"), lines, promo); !errors.Is(err, command.ErrInvalid) {
			t.Errorf("%s: err %v, want ErrInvalid", name, err)
		}
	}
}

// TWD is charged in whole dollars (stripe-psp-v1 D15, amount%100): a percent or fixed discount is rounded DOWN to a whole dollar, so a quote
// whose goods and shipping are whole dollars stays payable after the discount. Never up: the buyer never gets more than the code promised.
func TestCalculateWithPromoKeepsTWDWholeDollars(t *testing.T) {
	p := promoPolicy("none")
	p.Currency, p.TaxRateBPS = "TWD", 0
	lines := []AmountLine{{UnitPriceMinor: 33300, Quantity: 1}, {UnitPriceMinor: 12700, Quantity: 2}} // 58700
	for _, c := range []struct {
		name     string
		promo    Promo
		discount int64
	}{
		{"percent floors to the dollar", Promo{Kind: "percent", Percent: 7}, 4100}, // 4109 -> 4100
		{"percent already whole", Promo{Kind: "percent", Percent: 10}, 5800},       // 5870 -> 5800
		{"fixed below a dollar step", Promo{Kind: "fixed", FixedMinor: 99}, 0},     // < 1 dollar
		{"fixed whole", Promo{Kind: "fixed", FixedMinor: 20000}, 20000},
		{"fixed capped", Promo{Kind: "fixed", FixedMinor: 99999999}, 58700 - 58700%100},
	} {
		got, err := CalculateWith(p, lines, &c.promo)
		if err != nil || got.DiscountMinor != c.discount || got.DiscountMinor%100 != 0 {
			t.Fatalf("%s: discount %d err %v, want %d", c.name, got.DiscountMinor, err, c.discount)
		}
		if got.TotalMinor%100 != 0 {
			t.Fatalf("%s: total %d is not a whole dollar", c.name, got.TotalMinor)
		}
	}
}
