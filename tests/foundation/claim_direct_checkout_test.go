// Purpose: CDC01/CDC02 real-PG sold-out hints, skip/replenish semantics and definer isolation.
// Depends on: lcHarness, buyer scopes, claims B1/B2 and task-owned PostgreSQL.
// Used by: test-focused.sh, full G07 and claim-direct-checkout delivery evidence.
package foundation_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/buyer"
	"livecommerce/internal/claims"
)

// TestClaimDirectCheckoutAvailability proves stock is hinted, never reserved by a claim.
func TestClaimDirectCheckoutAvailability(t *testing.T) {
	h := lcSetup(t)
	_, _, c, link := h.claimSetup(t, "synthetic CDC buyer")
	sku := h.stock.skus[0].ID
	// Owner-only synthetic stock depletion; product writes still use the ledger.
	mustExec(t, h.f.owner, `UPDATE inventory.balances SET on_hand=reserved+allocated+unavailable+1 WHERE tenant_id=$1 AND store_id=$2 AND sku_id=$3`, h.f.tenantA, h.f.storeA1, sku)
	before := h.cartOf(t, h.cap)
	pv, err := h.preview(h.cap, link.Token)
	if err != nil {
		t.Fatal(err)
	}
	if len(pv.Lines) != 1 || !pv.Lines[0].SoldOut || !pv.Lines[0].Available {
		t.Fatalf("CDC01 want available catalog + sold_out quantity 2: %+v", pv.Lines)
	}
	out, err := h.redeem(h.cap, t04Key("cdc-sold-out"), link.Token, c.BundleVersion)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Applied) != 0 || len(out.Skipped) != 1 || out.Skipped[0].Reason != "sold_out" || out.Cart.Version != before.Version {
		t.Fatalf("CDC01 must skip without cart write: %+v", out)
	}
	pv, err = h.preview(h.cap, link.Token)
	if err != nil || !pv.Lines[0].Pending {
		t.Fatalf("CDC01 skipped claim lost pending: %+v %v", pv, err)
	}
	mustExec(t, h.f.owner, `UPDATE inventory.balances SET on_hand=reserved+allocated+unavailable+2 WHERE tenant_id=$1 AND store_id=$2 AND sku_id=$3`, h.f.tenantA, h.f.storeA1, sku)
	out, err = h.redeem(h.cap, t04Key("cdc-replenish"), link.Token, c.BundleVersion)
	if err != nil || len(out.Applied) != 1 || out.Applied[0].Quantity != 2 || len(out.Skipped) != 0 {
		t.Fatalf("CDC01 replenish retry: %+v %v", out, err)
	}
}

// TestClaimDirectCheckoutUntracked proves A6 SKUs are never marked sold out.
func TestClaimDirectCheckoutUntracked(t *testing.T) {
	h := lcSetup(t)
	_, _, c, link := h.claimSetup(t, "synthetic CDC untracked")
	sku := h.stock.skus[0].ID
	// Owner-only synthetic A6 fixture; no product privilege expansion.
	mustExec(t, h.f.owner, `UPDATE catalog.skus SET inventory_tracked=false,max_per_order=5 WHERE id=$1`, sku)
	mustExec(t, h.f.owner, `UPDATE inventory.balances SET on_hand=reserved+allocated+unavailable WHERE sku_id=$1`, sku)
	pv, err := h.preview(h.cap, link.Token)
	if err != nil || pv.Lines[0].SoldOut {
		t.Fatalf("CDC01 untracked: %+v %v", pv, err)
	}
	out, err := h.redeem(h.cap, t04Key("cdc-untracked"), link.Token, c.BundleVersion)
	if err != nil || len(out.Applied) != 1 {
		t.Fatalf("CDC01 untracked redeem: %+v %v", out, err)
	}
}

// TestClaimDirectCheckoutDefiner proves closed ACLs, buyer scope and bounded input.
func TestClaimDirectCheckoutDefiner(t *testing.T) {
	h := lcSetup(t)
	ctx := context.Background()
	var allowed bool
	for _, role := range []string{"commerce_buyer_runtime", "commerce_runtime", "public"} {
		err := h.f.owner.QueryRow(ctx, `SELECT has_function_privilege($1,'inventory.buyer_sku_availability(uuid[])','EXECUTE')`, role).Scan(&allowed)
		if err != nil {
			t.Fatal(err)
		}
		if allowed != (role == "commerce_buyer_runtime") {
			t.Fatalf("CDC02 unexpected EXECUTE %s=%v", role, allowed)
		}
	}
	tx, err := h.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `SET LOCAL ROLE commerce_buyer_runtime`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec(ctx, `SELECT * FROM inventory.buyer_sku_availability(ARRAY[]::uuid[])`)
	requirePGCode(t, err, "22023", "CDC02 missing buyer scope")
	skus := make([]string, 51)
	for i := range skus {
		skus[i] = h.stock.skus[0].ID
	}
	err = buyer.WithScope(ctx, h.a.runtime, h.cap.Token, h.cap.Scope.StoreID, func(ctx context.Context, tx pgx.Tx, _ buyer.Scope) error {
		_, e := tx.Exec(ctx, `SELECT * FROM inventory.buyer_sku_availability($1::uuid[])`, skus)
		return e
	})
	requirePGCode(t, err, "22023", "CDC02 >50 SKUs")
	foreign := t04CreateStock(t, h.f, h.f.tokens["a"], h.f.storeA2, 1, 1)
	err = buyer.WithScope(ctx, h.a.runtime, h.cap.Token, h.cap.Scope.StoreID, func(ctx context.Context, tx pgx.Tx, _ buyer.Scope) error {
		var n int
		e := tx.QueryRow(ctx, `SELECT count(*) FROM inventory.buyer_sku_availability($1::uuid[])`, []string{h.stock.skus[0].ID, foreign.skus[0].ID}).Scan(&n)
		if e == nil && n != 1 {
			t.Errorf("CDC02 leaked foreign-store SKU count=%d", n)
		}
		return e
	})
	if err != nil {
		t.Fatal(err)
	}
	// Buyer has only the projection, never SELECT on stock or the A6 tracking column.
	err = h.f.owner.QueryRow(ctx, `SELECT has_table_privilege('commerce_buyer_runtime','inventory.balances','SELECT') OR has_column_privilege('commerce_buyer_runtime','catalog.skus','inventory_tracked','SELECT')`).Scan(&allowed)
	if err != nil || allowed {
		t.Fatalf("CDC02 buyer raw data privilege: %v %v", allowed, err)
	}
}

// Keep the accepted claim DTO in this gate's documented dependency set.
var _ claims.Preview
