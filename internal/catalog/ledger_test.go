package catalog

import "testing"

func TestLedgerFilter(t *testing.T) {
	q, status, ok := ledgerFilter("  A%_  ", "")
	if !ok || q != "A%_" || status != "all" {
		t.Fatalf("%q %q %v", q, status, ok)
	}
	if _, _, ok := ledgerFilter("x\n", "active"); ok {
		t.Fatal("control query accepted")
	}
	if _, _, ok := ledgerFilter("x", "other"); ok {
		t.Fatal("invalid status accepted")
	}
}
