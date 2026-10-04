package foundation_test

import (
	"context"
	"testing"
)

// AT1/R2: REAL_PG schema precondition, deliberately red before 0113.
func TestAdsAttributionStorage(t *testing.T) {
	f := newT06GoFixture(t)
	var relation *string
	if err := f.base.owner.QueryRow(context.Background(), `SELECT to_regclass('orders.order_attribution')::text`).Scan(&relation); err != nil {
		t.Fatal(err)
	}
	if relation == nil {
		t.Fatal("R2 requires orders.order_attribution outside immutable snapshots")
	}
}
