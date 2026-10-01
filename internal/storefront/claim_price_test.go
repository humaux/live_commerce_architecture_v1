package storefront

import (
	"errors"
	"testing"

	"livecommerce/internal/command"
)

const (
	bundleA = "11111111-1111-4111-8111-111111111111"
	offerA  = "22222222-2222-4222-8222-222222222222"
	bundleB = "33333333-3333-4333-8333-333333333333"
	offerB  = "44444444-4444-4444-8444-444444444444"
)

func TestCarryOriginsRules(t *testing.T) {
	t.Parallel()
	existing := map[string]ClaimOrigin{"x": {BundleID: bundleA, OfferID: offerA, Quantity: 3}}
	// Kept while quantity <= claimed (lowering is fine, an unchanged line keeps its price).
	for _, qty := range []int64{1, 3} {
		out, err := carryOrigins([]Item{{SKUID: "x", Quantity: qty}}, existing, nil)
		if err != nil || out["x"].BundleID != bundleA {
			t.Fatalf("qty %d: origin dropped (%v, %v)", qty, out, err)
		}
	}
	// Raising above the claim drops the origin: the whole line goes back to the catalog price.
	if out, _ := carryOrigins([]Item{{SKUID: "x", Quantity: 4}}, existing, nil); len(out) != 0 {
		t.Fatalf("origin survived a quantity above the claim: %v", out)
	}
	// A SKU that left the cart loses its origin; a direct line never gains one.
	if out, _ := carryOrigins([]Item{{SKUID: "y", Quantity: 1}}, existing, nil); len(out) != 0 {
		t.Fatalf("origin appeared for a SKU without one: %v", out)
	}
	// A fresh origin (redeem) replaces the old one for that SKU (last apply wins).
	fresh := map[string]ClaimOrigin{"x": {BundleID: bundleB, OfferID: offerB, Quantity: 2}}
	out, err := carryOrigins([]Item{{SKUID: "x", Quantity: 2}}, existing, fresh)
	if err != nil || out["x"].BundleID != bundleB {
		t.Fatalf("fresh origin not applied: %v %v", out, err)
	}
	// A fresh origin never applies to a SKU that is not in the written cart.
	if out, _ := carryOrigins([]Item{{SKUID: "other", Quantity: 1}}, nil, fresh); len(out) != 0 {
		t.Fatalf("origin for an absent SKU: %v", out)
	}
	// Malformed fresh origins are rejected, never stored.
	for _, bad := range []ClaimOrigin{{BundleID: "nope", OfferID: offerA, Quantity: 1}, {BundleID: bundleA, OfferID: offerA, Quantity: 0}, {BundleID: bundleA, OfferID: offerA, Quantity: 1000}} {
		if _, err := carryOrigins([]Item{{SKUID: "x", Quantity: 1}}, nil, map[string]ClaimOrigin{"x": bad}); !errors.Is(err, command.ErrInvalid) {
			t.Fatalf("bad origin %v: err %v, want ErrInvalid", bad, err)
		}
	}
}
