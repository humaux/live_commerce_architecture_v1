package catalog

import (
	"math"
	"testing"
)

func TestInputValidationBounds(t *testing.T) {
	valid := SKUInput{ProductID: "11111111-1111-4111-8111-111111111111", Code: "a-1", PriceMinor: 0}
	if !validSKUInput(valid, false) {
		t.Fatal("valid SKU rejected")
	}
	for _, in := range []SKUInput{{}, {ProductID: valid.ProductID, Code: "bad space"}, {ProductID: valid.ProductID, Code: "ok", PriceMinor: -1}, {ProductID: valid.ProductID, Code: "ok", PriceMinor: math.MaxInt64}, {ProductID: valid.ProductID, Code: "ok", OriginCountry: "usa"}} {
		if validSKUInput(in, false) {
			t.Fatalf("invalid SKU accepted: %+v", in)
		}
	}
	if validProductInput(ProductInput{Name: "", Description: "x"}, false) || validProductInput(ProductInput{Name: "x", ExpectedVersion: 0}, true) {
		t.Fatal("product version or name bound missed")
	}
}

func FuzzSKUValidation(f *testing.F) {
	f.Add("A-1", int64(0), "US", "123456")
	f.Add("bad space", int64(-1), "usa", "12")
	f.Fuzz(func(t *testing.T, code string, price int64, country, hs string) {
		_ = validSKUInput(SKUInput{ProductID: "11111111-1111-4111-8111-111111111111", Code: code, PriceMinor: price, OriginCountry: country, HSCandidate: hs}, false)
	})
}
