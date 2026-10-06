// Purpose: DB-free guard that the shared fixture suffix t04Tag can never trip the §3.5 public-safe phone rule.
// Depends on: internal/msgtemplates (ValidatePublicSafe), t04Tag in catalog_inventory_test.go.
// Used by: go test ./tests/foundation (runs without PostgreSQL).
// Invariants: live-console-v1 §3.5 public-safe rule is untouched; only fixture names change.

package foundation_test

import (
	"testing"

	"livecommerce/internal/msgtemplates"
)

// TestT04TagIsPublicSafe generates 10k fixture names the way product/offer fixtures do and requires every one to pass
// ValidatePublicSafe (random hex with >=8 consecutive digits used to read as a phone number and flake LC-N08/K3-B4).
func TestT04TagIsPublicSafe(t *testing.T) {
	for i := 0; i < 10000; i++ {
		name := "lb-product-" + t04Tag()
		if r := msgtemplates.ValidatePublicSafe(name, ""); r != "" {
			t.Fatalf("%q refused as %s", name, r)
		}
	}
}
