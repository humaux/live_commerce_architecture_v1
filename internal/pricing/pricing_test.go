package pricing

import (
	"errors"
	"testing"

	"livecommerce/internal/command"
)

func TestDeliveryMethod(t *testing.T) {
	t.Parallel()

	for _, code := range []string{"home_tw", "cvs-711", "a"} {
		method, err := DeliveryMethod(code)
		if err != nil {
			t.Fatalf("DeliveryMethod(%q): %v", code, err)
		}
		if want := "delivery:" + code; method != want {
			t.Fatalf("DeliveryMethod(%q) = %q, want %q", code, method, want)
		}
		if !ValidMethod(method) {
			t.Fatalf("generated method %q was rejected", method)
		}
	}

	for _, code := range []string{"", "UPPER", "-bad", "contains:colon", "abcdefghijklmnopqrstuvwxyzabcdefghijklmno"} {
		if _, err := DeliveryMethod(code); !errors.Is(err, command.ErrInvalid) {
			t.Fatalf("DeliveryMethod(%q) error = %v, want ErrInvalid", code, err)
		}
	}
}

func TestValidMethodRemainsClosed(t *testing.T) {
	t.Parallel()

	for _, method := range []string{"home", "cvs_711", "cvs_familymart", "delivery:tw_home"} {
		if !ValidMethod(method) {
			t.Fatalf("ValidMethod(%q) = false", method)
		}
	}
	for _, method := range []string{"delivery:", "delivery:UPPER", "delivery:x:y", "carrier:tw_home", "pickup"} {
		if ValidMethod(method) {
			t.Fatalf("ValidMethod(%q) = true", method)
		}
	}
}

// storefront-v2 §C: shipping is 0 (and its tax 0) exactly when the server-side merchandise subtotal reaches the policy threshold; nil never
// waives it; the boundary is inclusive; everything else in the quote is unchanged.
func TestCalculateFreeShippingThreshold(t *testing.T) {
	policy := Policy{MarketID: "11111111-1111-4111-8111-111111111111", Country: "TW", Method: "home", Currency: "TWD", ShippingMode: "country_flat",
		TaxMode: "exclusive", TaxBasis: "goods_and_shipping", Version: 1, ShippingMinor: 6000, TaxRateBPS: 500, QuoteTTLSeconds: 600, Enabled: true}
	lines := []AmountLine{{UnitPriceMinor: 40000, Quantity: 1}, {UnitPriceMinor: 10000, Quantity: 1}} // subtotal 50000
	at := func(v int64) *int64 { return &v }
	for _, c := range []struct {
		name      string
		threshold *int64
		shipping  int64
		shipTax   int64
	}{
		{"no threshold keeps the flat fee", nil, 6000, 300},
		{"below the threshold keeps the fee", at(50001), 6000, 300},
		{"exactly at the threshold is free", at(50000), 0, 0},
		{"above the subtotal is free", at(1), 0, 0},
		{"zero threshold is always free", at(0), 0, 0},
	} {
		p := policy
		p.FreeShippingThresholdMinor = c.threshold
		got, err := Calculate(p, lines)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got.ShippingMinor != c.shipping || got.ShippingTaxMinor != c.shipTax || got.SubtotalMinor != 50000 {
			t.Fatalf("%s: shipping %d tax %d subtotal %d", c.name, got.ShippingMinor, got.ShippingTaxMinor, got.SubtotalMinor)
		}
		// I05: the total always equals subtotal + shipping + goods tax + shipping tax (exclusive tax).
		if want := got.SubtotalMinor + got.ShippingMinor + got.TaxMinor; got.TotalMinor != want {
			t.Fatalf("%s: total %d want %d", c.name, got.TotalMinor, want)
		}
	}
	bad := policy
	bad.FreeShippingThresholdMinor = at(-1)
	if _, err := Calculate(bad, lines); err == nil {
		t.Fatal("a negative threshold must be refused")
	}
	if validPolicyInput(PolicyInput{}) {
		t.Fatal("empty policy input accepted")
	}
}

func TestResolveUnitPrice(t *testing.T) {
	t.Parallel()
	ptr := func(v int64) *int64 { return &v }
	for _, tc := range []struct {
		name    string
		catalog int64
		live    *int64
		price   int64
		rule    string
	}{
		{"no origin", 1000, nil, 1000, RuleCatalog},
		{"live lower", 1000, ptr(700), 700, RuleLiveClaim},
		{"live higher is applied as set", 1000, ptr(1500), 1500, RuleLiveClaim},
		{"zero is not a price", 1000, ptr(0), 1000, RuleCatalog},
		{"negative is not a price", 1000, ptr(-5), 1000, RuleCatalog},
		{"above the checkout bound", 1000, ptr(1_000_000_000_001), 1000, RuleCatalog},
		{"upper bound ok", 1000, ptr(1_000_000_000_000), 1_000_000_000_000, RuleLiveClaim},
	} {
		if got, rule := ResolveUnitPrice(tc.catalog, tc.live); got != tc.price || rule != tc.rule {
			t.Fatalf("%s: got (%d,%s), want (%d,%s)", tc.name, got, rule, tc.price, tc.rule)
		}
	}
}
