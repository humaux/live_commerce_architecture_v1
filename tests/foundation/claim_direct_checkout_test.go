// Purpose: CDC01/CDC02 real-PG sold-out hints, skip/replenish semantics and definer isolation.
// Depends on: lcHarness, buyer scopes, claims B1/B2 and task-owned PostgreSQL.
// Used by: test-focused.sh, full G07 and claim-direct-checkout delivery evidence.
package foundation_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/buyer"
	"livecommerce/internal/inventory"
	"livecommerce/internal/platform"
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
	for _, role := range []string{"commerce_buyer_runtime", "commerce_runtime", "commerce_buyer_issuer"} {
		err := h.f.owner.QueryRow(ctx, `SELECT has_function_privilege($1,'inventory.buyer_sku_availability(uuid[])','EXECUTE')`, role).Scan(&allowed)
		if err != nil {
			t.Fatal(err)
		}
		if allowed != (role == "commerce_buyer_runtime") {
			t.Fatalf("CDC02 unexpected EXECUTE %s=%v", role, allowed)
		}
	}
	// PUBLIC is a pseudo-role, inspected through ACL expansion rather than role lookup.
	err := h.f.owner.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_proc p, LATERAL aclexplode(coalesce(p.proacl,acldefault('f',p.proowner))) a WHERE p.oid='inventory.buyer_sku_availability(uuid[])'::regprocedure AND a.grantee=0 AND a.privilege_type='EXECUTE')`).Scan(&allowed)
	if err != nil || allowed {
		t.Fatalf("CDC02 PUBLIC EXECUTE: %v %v", allowed, err)
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
	_, foreignToken := lcPrincipal(t, h.f, h.f.tenantA, []string{h.f.storeA2}, "store:read", "catalog:read", "catalog:write", "inventory:read", "inventory:write")
	foreign := t04CreateStock(t, h.f, foreignToken, h.f.storeA2, 1, 1)
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

// TestClaimDirectCheckoutStockMath covers all stock counters and multiple warehouses without a hold.
func TestClaimDirectCheckoutStockMath(t *testing.T) {
	h := lcSetup(t)
	_, _, _, link := h.claimSetup(t, "synthetic CDC warehouse math")
	sku := h.stock.skus[0].ID
	// Owner-only counterexample: on_hand=4 looks plentiful, but each excluded counter consumes one unit.
	mustExec(t, h.f.owner, `UPDATE inventory.balances SET on_hand=4,reserved=1,allocated=1,unavailable=1 WHERE tenant_id=$1 AND store_id=$2 AND sku_id=$3`, h.f.tenantA, h.f.storeA1, sku)
	pv, err := h.preview(h.cap, link.Token)
	if err != nil || !pv.Lines[0].SoldOut {
		t.Fatalf("counter exclusions: %+v %v", pv, err)
	}
	warehouse, err := t04Scoped(h.ctx, h.f, h.f.tokens["a"], h.f.storeA1, "inventory:write", func(tx pgx.Tx, s platform.Scope) (inventory.Warehouse, error) {
		return inventory.CreateWarehouse(h.ctx, tx, s, t04Key("cdc-warehouse"), "synthetic CDC second warehouse")
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = t04Scoped(h.ctx, h.f, h.f.tokens["a"], h.f.storeA1, "inventory:write", func(tx pgx.Tx, s platform.Scope) (inventory.Balance, error) {
		return inventory.AdjustOnHand(h.ctx, tx, s, t04Key("cdc-stock"), inventory.Adjustment{WarehouseID: warehouse.ID, SKUID: sku, Delta: 1, ExpectedVersion: 0, Reason: "synthetic counterexample stock"})
	})
	if err != nil {
		t.Fatal(err)
	}
	pv, err = h.preview(h.cap, link.Token)
	if err != nil || pv.Lines[0].SoldOut {
		t.Fatalf("cross-warehouse total equals target: %+v %v", pv, err)
	}
	missing := lcSKUs(t, h.f, h.f.tenantA, h.f.storeA1, "USD", 1)[0]
	err = buyer.WithScope(h.ctx, h.a.runtime, h.cap.Token, h.cap.Scope.StoreID, func(ctx context.Context, tx pgx.Tx, _ buyer.Scope) error {
		var tracked bool
		var units int64
		e := tx.QueryRow(ctx, `SELECT tracked,available FROM inventory.buyer_sku_availability($1::uuid[])`, []string{missing}).Scan(&tracked, &units)
		if e == nil && (!tracked || units != 0) {
			t.Errorf("no balances: tracked=%v units=%d", tracked, units)
		}
		return e
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestClaimDirectCheckoutDefinerGuards exercises tampered buyer scopes and closed role properties.
func TestClaimDirectCheckoutDefinerGuards(t *testing.T) {
	h := lcSetup(t)
	ctx := context.Background()
	sku := h.stock.skus[0].ID
	for _, tc := range []struct{ name, key, value string }{
		{"missing tenant", "app.tenant_id", ""}, {"malformed tenant", "app.tenant_id", "broken"},
		{"missing store", "app.store_id", ""}, {"malformed store", "app.store_id", "broken"},
		{"missing buyer", "app.buyer_id", ""}, {"malformed buyer", "app.buyer_id", "broken"},
		{"missing session", "app.buyer_session_id", ""}, {"malformed session", "app.buyer_session_id", "broken"},
		{"merchant principal", "app.principal_id", h.actor},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := buyer.WithScope(ctx, h.a.runtime, h.cap.Token, h.cap.Scope.StoreID, func(ctx context.Context, tx pgx.Tx, _ buyer.Scope) error {
				if _, e := tx.Exec(ctx, `SELECT set_config($1,$2,true)`, tc.key, tc.value); e != nil {
					return e
				}
				_, e := tx.Exec(ctx, `SELECT * FROM inventory.buyer_sku_availability($1::uuid[])`, []string{sku})
				return e
			})
			requirePGCode(t, err, "22023", tc.name)
		})
	}
	for _, tc := range []struct{ name, sql string }{
		{"null array", `SELECT * FROM inventory.buyer_sku_availability(NULL::uuid[])`},
		{"null entry", `SELECT * FROM inventory.buyer_sku_availability(ARRAY[$1::uuid,NULL])`},
		{"multidimensional", `SELECT * FROM inventory.buyer_sku_availability(ARRAY[[$1::uuid],[$1::uuid]])`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := buyer.WithScope(ctx, h.a.runtime, h.cap.Token, h.cap.Scope.StoreID, func(ctx context.Context, tx pgx.Tx, _ buyer.Scope) error {
				var e error
				if tc.name == "null array" {
					_, e = tx.Exec(ctx, tc.sql)
				} else {
					_, e = tx.Exec(ctx, tc.sql, sku)
				}
				return e
			})
			requirePGCode(t, err, "22023", tc.name)
		})
	}
	t.Run("repeatable read", func(t *testing.T) {
		tx, err := h.f.owner.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		if _, err = tx.Exec(ctx, `SET LOCAL ROLE commerce_buyer_runtime`); err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, `SELECT set_config('app.tenant_id',$1,true),set_config('app.store_id',$2,true),set_config('app.buyer_id',$3,true),set_config('app.buyer_session_id',$4,true),set_config('app.principal_id','',true)`, h.cap.Scope.TenantID, h.cap.Scope.StoreID, h.cap.Scope.OwnerID, h.cap.Scope.SessionID); err != nil {
			t.Fatal(err)
		}
		_, err = tx.Exec(ctx, `SELECT * FROM inventory.buyer_sku_availability($1::uuid[])`, []string{sku})
		requirePGCode(t, err, "22023", "repeatable read")
	})
	var owner, volatility string
	var definer, searchPath, login, bypass bool
	err := h.f.owner.QueryRow(ctx, `SELECT r.rolname,p.provolatile::text,p.prosecdef,coalesce(p.proconfig @> ARRAY['search_path=pg_catalog'],false),r.rolcanlogin,r.rolbypassrls FROM pg_proc p JOIN pg_roles r ON r.oid=p.proowner WHERE p.oid='inventory.buyer_sku_availability(uuid[])'::regprocedure`).Scan(&owner, &volatility, &definer, &searchPath, &login, &bypass)
	if err != nil || owner != "commerce_inventory_writer" || volatility != "s" || !definer || !searchPath || login || bypass {
		t.Fatalf("definer properties: %s %s %v %v login=%v bypass=%v %v", owner, volatility, definer, searchPath, login, bypass, err)
	}
	foreign := t04CreateStock(t, h.f, h.f.tokens["b"], h.f.storeB, 1)
	err = buyer.WithScope(ctx, h.a.runtime, h.cap.Token, h.cap.Scope.StoreID, func(ctx context.Context, tx pgx.Tx, _ buyer.Scope) error {
		var n int
		e := tx.QueryRow(ctx, `SELECT count(*) FROM inventory.buyer_sku_availability($1::uuid[])`, []string{foreign.skus[0].ID}).Scan(&n)
		if e == nil && n != 0 {
			t.Errorf("cross-tenant inventory leak: %d", n)
		}
		return e
	})
	if err != nil {
		t.Fatal(err)
	}
}
