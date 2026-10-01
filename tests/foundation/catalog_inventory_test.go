package foundation_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/catalog"
	"livecommerce/internal/command"
	"livecommerce/internal/inventory"
	"livecommerce/internal/platform"
)

var (
	t04GrantOnce sync.Once
	t04GrantErr  error
)

func t04Fixture(t *testing.T) *testFixture {
	t.Helper()
	f := fixture(t)
	t04GrantOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		for _, subject := range []struct {
			token string
			store string
		}{
			{f.tokens["a"], f.storeA1},
			{f.tokens["a2"], f.storeA2},
			{f.tokens["b"], f.storeB},
		} {
			var tenant, principal string
			t04GrantErr = f.owner.QueryRow(ctx, `SELECT m.tenant_id,s.principal_id
				FROM identity.sessions s JOIN identity.memberships m USING(principal_id)
				JOIN control.stores st ON st.tenant_id=m.tenant_id
				WHERE s.token_hash=$1 AND st.id=$2`, tokenHash(subject.token), subject.store).Scan(&tenant, &principal)
			if t04GrantErr != nil {
				return
			}
			_, t04GrantErr = f.owner.Exec(ctx, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission)
				SELECT $1,$2,$3,p FROM unnest(ARRAY['catalog:read','catalog:write','inventory:read','inventory:write','inventory:reserve']) p
				ON CONFLICT DO NOTHING`, tenant, subject.store, principal)
			if t04GrantErr != nil {
				return
			}
		}
	})
	if t04GrantErr != nil {
		t.Fatalf("seed T04 permissions in isolated fixture: %v", t04GrantErr)
	}
	return f
}

func t04Scoped[T any](ctx context.Context, f *testFixture, token, store, permission string, fn func(pgx.Tx, platform.Scope) (T, error)) (T, error) {
	var result T
	err := platform.WithScope(ctx, f.runtime, token, store, permission, func(tx pgx.Tx, scope platform.Scope) error {
		var err error
		result, err = fn(tx, scope)
		return err
	})
	return result, err
}

func t04Tag() string { return strings.ReplaceAll(randomUUID(), "-", "")[:12] }

func t04Key(prefix string) string { return prefix + ":" + t04Tag() }

type t04Stock struct {
	product   catalog.Product
	skus      []catalog.SKU
	warehouse inventory.Warehouse
	balances  []inventory.Balance
}

// t04PreV2Schema reports whether catalog.products predates migration 0086. The historical-schema fixtures (mcPre0029Fixture,
// lriPre0032Fixture) run the CURRENT Go against older migrations; catalog.CreateProduct/CreateSKU write the 0086 columns
// (slug, options, option_values, compare_at_minor, status draft), so on such a schema the fixture rows are inserted with
// the pre-0086 statements instead. Test-only: the product code has no old-schema path.
func t04PreV2Schema(t *testing.T, f *testFixture) bool {
	t.Helper()
	var has bool
	if err := f.owner.QueryRow(context.Background(), `SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema='catalog' AND table_name='products' AND column_name='slug')`).Scan(&has); err != nil {
		t.Fatal(err)
	}
	return !has
}

func t04LegacyProduct(tx pgx.Tx, scope platform.Scope, name, description string) (catalog.Product, error) {
	var p catalog.Product
	err := tx.QueryRow(context.Background(), `INSERT INTO catalog.products(tenant_id,store_id,name,description) VALUES($1,$2,$3,$4) RETURNING id::text,name,description,status,version`,
		scope.TenantID, scope.StoreID, name, description).Scan(&p.ID, &p.Name, &p.Description, &p.Status, &p.Version)
	return p, err
}

func t04LegacySKU(tx pgx.Tx, scope platform.Scope, in catalog.SKUInput) (catalog.SKU, error) {
	ctx := context.Background()
	var s catalog.SKU
	err := tx.QueryRow(ctx, `INSERT INTO catalog.skus(tenant_id,store_id,product_id,code,currency,price_minor,weight_grams,length_mm,width_mm,height_mm,origin_country,customs_name,hs_candidate)
		SELECT $1,$2,$3,$4,st.currency,$5,$6,$7,$8,$9,$10,$11,$12 FROM control.stores st WHERE st.tenant_id=$1 AND st.id=$2
		RETURNING id::text,product_id::text,code,status,currency,price_minor,version,weight_grams,length_mm,width_mm,height_mm,origin_country,customs_name,hs_candidate`,
		scope.TenantID, scope.StoreID, in.ProductID, in.Code, in.PriceMinor, in.WeightGrams, in.LengthMM, in.WidthMM, in.HeightMM, in.OriginCountry, in.CustomsName, in.HSCandidate).
		Scan(&s.ID, &s.ProductID, &s.Code, &s.Status, &s.Currency, &s.PriceMinor, &s.Version, &s.WeightGrams, &s.LengthMM, &s.WidthMM, &s.HeightMM, &s.OriginCountry, &s.CustomsName, &s.HSCandidate)
	if err != nil {
		return s, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO catalog.price_history(tenant_id,store_id,sku_id,version,price_minor,currency,principal_id) VALUES($1,$2,$3,$4,$5,$6,$7)`,
		scope.TenantID, scope.StoreID, s.ID, s.Version, s.PriceMinor, s.Currency, scope.PrincipalID)
	return s, err
}

func t04CreateStock(t *testing.T, f *testFixture, token, store string, quantities ...int64) t04Stock {
	t.Helper()
	ctx := context.Background()
	tag := t04Tag()
	legacy := t04PreV2Schema(t, f)
	product, err := t04Scoped(ctx, f, token, store, "catalog:write", func(tx pgx.Tx, scope platform.Scope) (catalog.Product, error) {
		if legacy {
			return t04LegacyProduct(tx, scope, "t04-"+tag, "isolated test stock")
		}
		return catalog.CreateProduct(ctx, tx, scope, t04Key("product"), catalog.ProductInput{Name: "t04-" + tag, Description: "isolated test stock", Status: catalog.StatusActive})
	})
	if err != nil {
		t.Fatalf("create product: %v", err)
	}
	warehouse, err := t04Scoped(ctx, f, token, store, "inventory:write", func(tx pgx.Tx, scope platform.Scope) (inventory.Warehouse, error) {
		return inventory.CreateWarehouse(ctx, tx, scope, t04Key("warehouse"), "t04-"+tag)
	})
	if err != nil {
		t.Fatalf("create warehouse: %v", err)
	}
	stock := t04Stock{product: product, warehouse: warehouse}
	for i, quantity := range quantities {
		sku, err := t04Scoped(ctx, f, token, store, "catalog:write", func(tx pgx.Tx, scope platform.Scope) (catalog.SKU, error) {
			in := catalog.SKUInput{
				ProductID: product.ID, Code: fmt.Sprintf("T04-%s-%d", tag, i), PriceMinor: 1250,
				WeightGrams: 100, LengthMM: 10, WidthMM: 20, HeightMM: 30,
				OriginCountry: "US", CustomsName: "test item", HSCandidate: "851840",
			}
			if legacy {
				return t04LegacySKU(tx, scope, in)
			}
			return catalog.CreateSKU(ctx, tx, scope, t04Key("sku"), in)
		})
		if err != nil {
			t.Fatalf("create SKU %d: %v", i, err)
		}
		balance, err := t04Scoped(ctx, f, token, store, "inventory:write", func(tx pgx.Tx, scope platform.Scope) (inventory.Balance, error) {
			return inventory.AdjustOnHand(ctx, tx, scope, t04Key("adjust"), inventory.Adjustment{
				WarehouseID: warehouse.ID, SKUID: sku.ID, Delta: quantity, ExpectedVersion: 0, Reason: "T04 fixture stock",
			})
		})
		if err != nil {
			t.Fatalf("adjust SKU %d: %v", i, err)
		}
		stock.skus = append(stock.skus, sku)
		stock.balances = append(stock.balances, balance)
	}
	return stock
}

func t04Reserve(ctx context.Context, f *testFixture, token, store, key string, lines []inventory.Line) (inventory.Reservation, error) {
	return t04Scoped(ctx, f, token, store, "inventory:reserve", func(tx pgx.Tx, scope platform.Scope) (inventory.Reservation, error) {
		return inventory.Reserve(ctx, tx, scope, key, lines)
	})
}

func t04Release(ctx context.Context, f *testFixture, token, store, key, id string, expire bool) (inventory.Reservation, error) {
	return t04Scoped(ctx, f, token, store, "inventory:reserve", func(tx pgx.Tx, scope platform.Scope) (inventory.Reservation, error) {
		return inventory.ReleaseReservation(ctx, tx, scope, key, id, expire)
	})
}

func t04Count(t *testing.T, f *testFixture, query string, args ...any) int {
	t.Helper()
	var count int
	if err := f.owner.QueryRow(context.Background(), query, args...).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func TestT04MigrationScopeForeignKeysAndRuntimePrivileges(t *testing.T) {
	f := t04Fixture(t)
	ctx := context.Background()

	var versions string
	var checksumsValid bool
	if err := f.owner.QueryRow(ctx, `SELECT string_agg(version,',' ORDER BY version),bool_and(checksum ~ '^[0-9a-f]{64}$')
		FROM public.lc_schema_migrations WHERE version IN ('0001_foundation.sql','0002_catalog_inventory.sql')`).Scan(&versions, &checksumsValid); err != nil {
		t.Fatal(err)
	}
	if versions != "0001_foundation.sql,0002_catalog_inventory.sql" || !checksumsValid {
		t.Fatalf("migration ledger versions=%q checksums_valid=%t", versions, checksumsValid)
	}

	stock := t04CreateStock(t, f, f.tokens["a"], f.storeA1, 4)
	productsA2, err := t04Scoped(ctx, f, f.tokens["a2"], f.storeA2, "catalog:read", func(tx pgx.Tx, scope platform.Scope) ([]catalog.Product, error) {
		return catalog.ListProducts(ctx, tx, scope)
	})
	if err != nil || len(productsA2) != 0 {
		t.Fatalf("same-tenant foreign-store products=%v err=%v, want empty", productsA2, err)
	}
	productsB, err := t04Scoped(ctx, f, f.tokens["b"], f.storeB, "catalog:read", func(tx pgx.Tx, scope platform.Scope) ([]catalog.Product, error) {
		return catalog.ListProducts(ctx, tx, scope)
	})
	if err != nil || len(productsB) != 0 {
		t.Fatalf("foreign-tenant products=%v err=%v, want empty", productsB, err)
	}
	if got := t04Count(t, f, `SELECT count(*) FROM catalog.products`); got < 1 {
		t.Fatal("owner fixture cannot read created product")
	}
	var noScope int
	if err := f.runtime.QueryRow(ctx, `SELECT count(*) FROM catalog.products`).Scan(&noScope); err != nil || noScope != 0 {
		t.Fatalf("no-scope product count=%d err=%v, want 0", noScope, err)
	}
	warehousesA2, err := t04Scoped(ctx, f, f.tokens["a2"], f.storeA2, "inventory:read", func(tx pgx.Tx, scope platform.Scope) ([]inventory.Warehouse, error) {
		return inventory.ListWarehouses(ctx, tx, scope)
	})
	if err != nil || len(warehousesA2) != 0 {
		t.Fatalf("same-tenant foreign-store warehouses=%v err=%v, want empty", warehousesA2, err)
	}
	balancesB, err := t04Scoped(ctx, f, f.tokens["b"], f.storeB, "inventory:read", func(tx pgx.Tx, scope platform.Scope) ([]inventory.Balance, error) {
		return inventory.ListBalances(ctx, tx, scope)
	})
	if err != nil || len(balancesB) != 0 {
		t.Fatalf("foreign-tenant balances=%v err=%v, want empty", balancesB, err)
	}

	_, err = t04Scoped(ctx, f, f.tokens["a2"], f.storeA2, "catalog:write", func(tx pgx.Tx, scope platform.Scope) (catalog.SKU, error) {
		return catalog.CreateSKU(ctx, tx, scope, t04Key("foreignsku"), catalog.SKUInput{ProductID: stock.product.ID, Code: "FOREIGN-" + t04Tag(), PriceMinor: 1})
	})
	if !errors.Is(err, command.ErrNotFound) {
		t.Fatalf("foreign product CreateSKU error=%v, want ErrNotFound", err)
	}
	err = platform.WithScope(ctx, f.runtime, f.tokens["a2"], f.storeA2, "catalog:write", func(tx pgx.Tx, scope platform.Scope) error {
		_, err := tx.Exec(ctx, `INSERT INTO catalog.skus(tenant_id,store_id,id,product_id,code,currency,price_minor)
			VALUES($1,$2,$3,$4,$5,'USD',1)`, scope.TenantID, scope.StoreID, randomUUID(), stock.product.ID, "FK-"+t04Tag())
		return err
	})
	if sqlState(err) != "23503" {
		t.Fatalf("cross-store catalog FK state=%q err=%v, want 23503", sqlState(err), err)
	}
	err = platform.WithScope(ctx, f.runtime, f.tokens["a2"], f.storeA2, "inventory:reserve", func(tx pgx.Tx, scope platform.Scope) error {
		reservationID := randomUUID()
		if _, err := tx.Exec(ctx, `INSERT INTO inventory.reservations(tenant_id,store_id,id,state) VALUES($1,$2,$3,'HELD')`, scope.TenantID, scope.StoreID, reservationID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO inventory.reservation_lines(tenant_id,store_id,reservation_id,warehouse_id,sku_id,quantity)
			VALUES($1,$2,$3,$4,$5,1)`, scope.TenantID, scope.StoreID, reservationID, stock.warehouse.ID, stock.skus[0].ID)
		return err
	})
	if sqlState(err) != "23503" {
		t.Fatalf("cross-store inventory FK state=%q err=%v, want 23503", sqlState(err), err)
	}

	err = platform.WithScope(ctx, f.runtime, f.tokens["a"], f.storeA1, "inventory:write", func(tx pgx.Tx, scope platform.Scope) error {
		_, err := tx.Exec(ctx, `UPDATE inventory.balances SET on_hand=on_hand+1
			WHERE tenant_id=$1 AND store_id=$2 AND warehouse_id=$3 AND sku_id=$4`, scope.TenantID, scope.StoreID, stock.warehouse.ID, stock.skus[0].ID)
		return err
	})
	if sqlState(err) != "42501" {
		t.Fatalf("direct balance update state=%q err=%v, want 42501", sqlState(err), err)
	}
	err = platform.WithScope(ctx, f.runtime, f.tokens["a"], f.storeA1, "inventory:write", func(tx pgx.Tx, _ platform.Scope) error {
		_, err := tx.Exec(ctx, `SET LOCAL ROLE commerce_inventory_writer`)
		return err
	})
	if sqlState(err) != "42501" {
		t.Fatalf("runtime SET ROLE writer state=%q err=%v, want 42501", sqlState(err), err)
	}
	err = platform.WithScope(ctx, f.runtime, f.tokens["a2"], f.storeA2, "inventory:write", func(tx pgx.Tx, _ platform.Scope) error {
		var version int64
		err := tx.QueryRow(ctx, `SELECT version FROM inventory.lock_balance($1,$2)`, stock.warehouse.ID, stock.skus[0].ID).Scan(&version)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("foreign lock_balance returned version=%d err=%v", version, err)
	})
	if err != nil {
		t.Fatal(err)
	}

	priced, err := t04Scoped(ctx, f, f.tokens["a"], f.storeA1, "catalog:write", func(tx pgx.Tx, scope platform.Scope) (catalog.SKU, error) {
		return catalog.SetSKUPrice(ctx, tx, scope, t04Key("price"), stock.skus[0].ID, catalog.PriceInput{PriceMinor: 1300, ExpectedVersion: stock.skus[0].Version})
	})
	if err != nil {
		t.Fatalf("set price for append-only probe: %v", err)
	}
	for name, tc := range map[string]struct {
		permission string
		query      string
		args       []any
	}{
		"price history update": {"catalog:write", `UPDATE catalog.price_history SET price_minor=price_minor+1 WHERE sku_id=$1`, []any{priced.ID}},
		"price history delete": {"catalog:write", `DELETE FROM catalog.price_history WHERE sku_id=$1`, []any{priced.ID}},
		"ledger update":        {"inventory:write", `UPDATE inventory.ledger SET reason='changed' WHERE sku_id=$1`, []any{priced.ID}},
		"ledger delete":        {"inventory:write", `DELETE FROM inventory.ledger WHERE sku_id=$1`, []any{priced.ID}},
	} {
		t.Run(name, func(t *testing.T) {
			err := platform.WithScope(ctx, f.runtime, f.tokens["a"], f.storeA1, tc.permission, func(tx pgx.Tx, _ platform.Scope) error {
				_, err := tx.Exec(ctx, tc.query, tc.args...)
				return err
			})
			if sqlState(err) != "42501" {
				t.Fatalf("append-only state=%q err=%v, want 42501", sqlState(err), err)
			}
		})
	}
}

func TestT04DuplicateLedgerConflictHasNoBalanceSideEffect(t *testing.T) {
	f := t04Fixture(t)
	ctx := context.Background()
	stock := t04CreateStock(t, f, f.tokens["a"], f.storeA1, 7)
	var ledgerID string
	if err := f.owner.QueryRow(ctx, `SELECT id FROM inventory.ledger
		WHERE tenant_id=$1 AND store_id=$2 AND warehouse_id=$3 AND sku_id=$4 AND kind='ADJUST'`,
		f.tenantA, f.storeA1, stock.warehouse.ID, stock.skus[0].ID).Scan(&ledgerID); err != nil {
		t.Fatal(err)
	}
	ledgerBefore := t04Count(t, f, `SELECT count(*) FROM inventory.ledger
		WHERE tenant_id=$1 AND store_id=$2 AND warehouse_id=$3 AND sku_id=$4`,
		f.tenantA, f.storeA1, stock.warehouse.ID, stock.skus[0].ID)

	err := platform.WithScope(ctx, f.runtime, f.tokens["a"], f.storeA1, "inventory:write", func(tx pgx.Tx, _ platform.Scope) error {
		tag, err := tx.Exec(ctx, `INSERT INTO inventory.ledger(
			tenant_id,store_id,warehouse_id,sku_id,kind,delta_on_hand,delta_reserved,
			delta_allocated,delta_unavailable,operation,command_key,reservation_id,reason,principal_id)
			SELECT tenant_id,store_id,warehouse_id,sku_id,kind,delta_on_hand,delta_reserved,
				delta_allocated,delta_unavailable,operation,command_key,reservation_id,reason,principal_id
			FROM inventory.ledger WHERE id=$1 ON CONFLICT DO NOTHING`, ledgerID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 0 {
			return fmt.Errorf("duplicate ledger rows affected=%d, want 0", tag.RowsAffected())
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := t04Count(t, f, `SELECT count(*) FROM inventory.ledger
		WHERE tenant_id=$1 AND store_id=$2 AND warehouse_id=$3 AND sku_id=$4`,
		f.tenantA, f.storeA1, stock.warehouse.ID, stock.skus[0].ID); got != ledgerBefore {
		t.Fatalf("ledger count=%d, want unchanged %d", got, ledgerBefore)
	}
	var onHand, reserved, allocated, unavailable, version int64
	if err := f.owner.QueryRow(ctx, `SELECT on_hand,reserved,allocated,unavailable,version FROM inventory.balances
		WHERE tenant_id=$1 AND store_id=$2 AND warehouse_id=$3 AND sku_id=$4`,
		f.tenantA, f.storeA1, stock.warehouse.ID, stock.skus[0].ID).
		Scan(&onHand, &reserved, &allocated, &unavailable, &version); err != nil {
		t.Fatal(err)
	}
	before := stock.balances[0]
	if onHand != before.OnHand || reserved != before.Reserved || allocated != before.Allocated || unavailable != before.Unavailable || version != before.Version {
		t.Fatalf("balance changed after skipped ledger: got=(%d,%d,%d,%d,v%d) want=(%d,%d,%d,%d,v%d)",
			onHand, reserved, allocated, unavailable, version,
			before.OnHand, before.Reserved, before.Allocated, before.Unavailable, before.Version)
	}
}

func TestT04CommandReplayOptimisticCatalogAndPriceHistory(t *testing.T) {
	f := t04Fixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	key := t04Key("same-product")
	input := catalog.ProductInput{Name: "replay-" + t04Tag(), Description: "same canonical request"}
	auditBefore := t04Count(t, f, `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action='catalog.product.created'`, f.tenantA, f.storeA1)

	type outcome struct {
		product catalog.Product
		err     error
	}
	start := make(chan struct{})
	results := make(chan outcome, 2)
	for range 2 {
		go func() {
			<-start
			product, err := t04Scoped(ctx, f, f.tokens["a"], f.storeA1, "catalog:write", func(tx pgx.Tx, scope platform.Scope) (catalog.Product, error) {
				return catalog.CreateProduct(ctx, tx, scope, key, input)
			})
			results <- outcome{product, err}
		}()
	}
	close(start)
	one, two := <-results, <-results
	if one.err != nil || two.err != nil || one.product.ID == "" || one.product.ID != two.product.ID {
		t.Fatalf("simultaneous replay results=%+v / %+v", one, two)
	}
	if got := t04Count(t, f, `SELECT count(*) FROM ops.command_results WHERE tenant_id=$1 AND store_id=$2 AND idempotency_key=$3`, f.tenantA, f.storeA1, key); got != 1 {
		t.Fatalf("command results=%d, want 1", got)
	}
	if got := t04Count(t, f, `SELECT count(*) FROM catalog.products WHERE tenant_id=$1 AND store_id=$2 AND id=$3`, f.tenantA, f.storeA1, one.product.ID); got != 1 {
		t.Fatalf("products=%d, want 1", got)
	}
	if got := t04Count(t, f, `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action='catalog.product.created'`, f.tenantA, f.storeA1); got != auditBefore+1 {
		t.Fatalf("created audit count=%d, want %d", got, auditBefore+1)
	}

	_, err := t04Scoped(ctx, f, f.tokens["a"], f.storeA1, "catalog:write", func(tx pgx.Tx, scope platform.Scope) (catalog.Product, error) {
		return catalog.CreateProduct(ctx, tx, scope, key, catalog.ProductInput{Name: input.Name + "-changed", Description: input.Description})
	})
	if !errors.Is(err, command.ErrConflict) {
		t.Fatalf("changed replay error=%v, want ErrConflict", err)
	}

	updated, err := t04Scoped(ctx, f, f.tokens["a"], f.storeA1, "catalog:write", func(tx pgx.Tx, scope platform.Scope) (catalog.Product, error) {
		return catalog.UpdateProduct(ctx, tx, scope, t04Key("update-product"), one.product.ID, catalog.ProductInput{Name: input.Name + "-v2", Description: input.Description, ExpectedVersion: 1})
	})
	if err != nil || updated.Version != 2 {
		t.Fatalf("update product=%+v err=%v, want version 2", updated, err)
	}
	_, err = t04Scoped(ctx, f, f.tokens["a"], f.storeA1, "catalog:write", func(tx pgx.Tx, scope platform.Scope) (catalog.Product, error) {
		return catalog.UpdateProduct(ctx, tx, scope, t04Key("stale-product"), one.product.ID, catalog.ProductInput{Name: "stale", ExpectedVersion: 1})
	})
	if !errors.Is(err, command.ErrConflict) {
		t.Fatalf("stale product error=%v, want ErrConflict", err)
	}

	stock := t04CreateStock(t, f, f.tokens["a"], f.storeA1, 2)
	sku := stock.skus[0]
	var initialVersion, initialPrice int64
	var initialCurrency string
	if err := f.owner.QueryRow(ctx, `SELECT version,price_minor,currency FROM catalog.price_history
		WHERE tenant_id=$1 AND store_id=$2 AND sku_id=$3`, f.tenantA, f.storeA1, sku.ID).Scan(&initialVersion, &initialPrice, &initialCurrency); err != nil {
		t.Fatalf("initial price history: %v", err)
	}
	if initialVersion != 1 || initialPrice != sku.PriceMinor || initialCurrency != sku.Currency {
		t.Fatalf("initial price history=(v%d,%d,%s), want (v1,%d,%s)", initialVersion, initialPrice, initialCurrency, sku.PriceMinor, sku.Currency)
	}
	metadata, err := t04Scoped(ctx, f, f.tokens["a"], f.storeA1, "catalog:write", func(tx pgx.Tx, scope platform.Scope) (catalog.SKU, error) {
		return catalog.UpdateSKU(ctx, tx, scope, t04Key("update-sku"), sku.ID, catalog.SKUInput{
			ProductID: sku.ProductID, Code: sku.Code + "-V2", PriceMinor: sku.PriceMinor, ExpectedVersion: sku.Version,
			WeightGrams: 101, LengthMM: 11, WidthMM: 21, HeightMM: 31, OriginCountry: "US", CustomsName: "updated", HSCandidate: "851840",
		})
	})
	if err != nil || metadata.Version != sku.Version+1 {
		t.Fatalf("metadata update=%+v err=%v", metadata, err)
	}
	_, err = t04Scoped(ctx, f, f.tokens["a"], f.storeA1, "catalog:write", func(tx pgx.Tx, scope platform.Scope) (catalog.SKU, error) {
		return catalog.UpdateSKU(ctx, tx, scope, t04Key("price-via-metadata"), sku.ID, catalog.SKUInput{
			ProductID: sku.ProductID, Code: metadata.Code, PriceMinor: metadata.PriceMinor + 1, ExpectedVersion: metadata.Version,
		})
	})
	if err == nil {
		t.Fatal("UpdateSKU changed price without SetSKUPrice")
	}
	historyBefore := t04Count(t, f, `SELECT count(*) FROM catalog.price_history WHERE tenant_id=$1 AND store_id=$2 AND sku_id=$3`, f.tenantA, f.storeA1, sku.ID)
	if historyBefore != 1 {
		t.Fatalf("initial price history rows=%d, want 1", historyBefore)
	}
	priced, err := t04Scoped(ctx, f, f.tokens["a"], f.storeA1, "catalog:write", func(tx pgx.Tx, scope platform.Scope) (catalog.SKU, error) {
		return catalog.SetSKUPrice(ctx, tx, scope, t04Key("zero-price"), sku.ID, catalog.PriceInput{PriceMinor: 0, ExpectedVersion: metadata.Version})
	})
	if err != nil || priced.PriceMinor != 0 || priced.Version != metadata.Version+1 {
		t.Fatalf("zero price result=%+v err=%v", priced, err)
	}
	if got := t04Count(t, f, `SELECT count(*) FROM catalog.price_history WHERE tenant_id=$1 AND store_id=$2 AND sku_id=$3`, f.tenantA, f.storeA1, sku.ID); got != historyBefore+1 {
		t.Fatalf("price history count=%d, want %d", got, historyBefore+1)
	}
	var changedPrice int64
	if err := f.owner.QueryRow(ctx, `SELECT price_minor FROM catalog.price_history
		WHERE tenant_id=$1 AND store_id=$2 AND sku_id=$3 AND version=$4`, f.tenantA, f.storeA1, sku.ID, priced.Version).Scan(&changedPrice); err != nil || changedPrice != 0 {
		t.Fatalf("changed price history=%d err=%v, want version %d price 0", changedPrice, err, priced.Version)
	}
	_, err = t04Scoped(ctx, f, f.tokens["a"], f.storeA1, "catalog:write", func(tx pgx.Tx, scope platform.Scope) (catalog.SKU, error) {
		return catalog.SetSKUPrice(ctx, tx, scope, t04Key("stale-price"), sku.ID, catalog.PriceInput{PriceMinor: 1, ExpectedVersion: metadata.Version})
	})
	if !errors.Is(err, command.ErrConflict) {
		t.Fatalf("stale price error=%v, want ErrConflict", err)
	}
	if total, err := command.CheckMoney(command.MaxMoney, 2); !errors.Is(err, command.ErrInvalid) || total != 0 {
		t.Fatalf("overflow total=%d err=%v", total, err)
	}
	adjusted, err := t04Scoped(ctx, f, f.tokens["a"], f.storeA1, "inventory:write", func(tx pgx.Tx, scope platform.Scope) (inventory.Balance, error) {
		return inventory.AdjustOnHand(ctx, tx, scope, t04Key("adjust-version"), inventory.Adjustment{
			WarehouseID: stock.warehouse.ID, SKUID: sku.ID, Delta: 1, ExpectedVersion: stock.balances[0].Version, Reason: "version winner",
		})
	})
	if err != nil || adjusted.Version != stock.balances[0].Version+1 {
		t.Fatalf("versioned adjustment=%+v err=%v", adjusted, err)
	}
	_, err = t04Scoped(ctx, f, f.tokens["a"], f.storeA1, "inventory:write", func(tx pgx.Tx, scope platform.Scope) (inventory.Balance, error) {
		return inventory.AdjustOnHand(ctx, tx, scope, t04Key("adjust-stale"), inventory.Adjustment{
			WarehouseID: stock.warehouse.ID, SKUID: sku.ID, Delta: 1, ExpectedVersion: stock.balances[0].Version, Reason: "stale loser",
		})
	})
	if !errors.Is(err, command.ErrConflict) {
		t.Fatalf("stale adjustment error=%v, want ErrConflict", err)
	}
}

func TestT04ConcurrentReserveOrderingCanonicalDuplicatesAndAtomicFailure(t *testing.T) {
	f := t04Fixture(t)

	t.Run("one unit has one winner", func(t *testing.T) {
		stock := t04CreateStock(t, f, f.tokens["a"], f.storeA1, 1)
		line := []inventory.Line{{WarehouseID: stock.warehouse.ID, SKUID: stock.skus[0].ID, Quantity: 1}}
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		start := make(chan struct{})
		errs := make(chan error, 2)
		for range 2 {
			key := t04Key("reserve-one")
			go func() {
				<-start
				_, err := t04Reserve(ctx, f, f.tokens["a"], f.storeA1, key, line)
				errs <- err
			}()
		}
		close(start)
		got := []error{<-errs, <-errs}
		success, insufficient := 0, 0
		for _, err := range got {
			switch {
			case err == nil:
				success++
			case errors.Is(err, command.ErrInsufficient):
				insufficient++
			default:
				t.Fatalf("unexpected reserve error: %v", err)
			}
		}
		if success != 1 || insufficient != 1 {
			t.Fatalf("success/insufficient=%d/%d, want 1/1", success, insufficient)
		}
		var reserved int64
		if err := f.owner.QueryRow(context.Background(), `SELECT reserved FROM inventory.balances WHERE tenant_id=$1 AND store_id=$2 AND warehouse_id=$3 AND sku_id=$4`, f.tenantA, f.storeA1, stock.warehouse.ID, stock.skus[0].ID).Scan(&reserved); err != nil || reserved != 1 {
			t.Fatalf("reserved=%d err=%v, want 1", reserved, err)
		}
	})

	t.Run("reverse order cannot deadlock or partially write", func(t *testing.T) {
		stock := t04CreateStock(t, f, f.tokens["a"], f.storeA1, 1, 1)
		orders := [][]inventory.Line{
			{{WarehouseID: stock.warehouse.ID, SKUID: stock.skus[0].ID, Quantity: 1}, {WarehouseID: stock.warehouse.ID, SKUID: stock.skus[1].ID, Quantity: 1}},
			{{WarehouseID: stock.warehouse.ID, SKUID: stock.skus[1].ID, Quantity: 1}, {WarehouseID: stock.warehouse.ID, SKUID: stock.skus[0].ID, Quantity: 1}},
		}
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		start := make(chan struct{})
		errs := make(chan error, 2)
		for i := range orders {
			lines := orders[i]
			key := t04Key("reserve-order")
			go func() {
				<-start
				_, err := t04Reserve(ctx, f, f.tokens["a"], f.storeA1, key, lines)
				errs <- err
			}()
		}
		close(start)
		got := []error{<-errs, <-errs}
		if !((got[0] == nil && errors.Is(got[1], command.ErrInsufficient)) || (got[1] == nil && errors.Is(got[0], command.ErrInsufficient))) {
			t.Fatalf("reverse-order errors=%v, want one success and one insufficient", got)
		}
		var reserved []int64
		rows, err := f.owner.Query(context.Background(), `SELECT reserved FROM inventory.balances WHERE tenant_id=$1 AND store_id=$2 AND warehouse_id=$3 AND sku_id IN ($4,$5) ORDER BY sku_id`, f.tenantA, f.storeA1, stock.warehouse.ID, stock.skus[0].ID, stock.skus[1].ID)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		for rows.Next() {
			var quantity int64
			if err := rows.Scan(&quantity); err != nil {
				t.Fatal(err)
			}
			reserved = append(reserved, quantity)
		}
		if err := rows.Err(); err != nil || len(reserved) != 2 || reserved[0] != 1 || reserved[1] != 1 {
			t.Fatalf("reserved=%v err=%v, want [1 1]", reserved, err)
		}
	})

	t.Run("duplicate lines canonicalize", func(t *testing.T) {
		stock := t04CreateStock(t, f, f.tokens["a"], f.storeA1, 3)
		reservation, err := t04Reserve(context.Background(), f, f.tokens["a"], f.storeA1, t04Key("reserve-duplicates"), []inventory.Line{
			{WarehouseID: stock.warehouse.ID, SKUID: stock.skus[0].ID, Quantity: 1},
			{WarehouseID: stock.warehouse.ID, SKUID: stock.skus[0].ID, Quantity: 2},
		})
		if err != nil || len(reservation.Lines) != 1 || reservation.Lines[0].Quantity != 3 {
			t.Fatalf("canonical reservation=%+v err=%v", reservation, err)
		}
		if got := t04Count(t, f, `SELECT count(*) FROM inventory.reservation_lines WHERE tenant_id=$1 AND store_id=$2 AND reservation_id=$3 AND quantity=3`, f.tenantA, f.storeA1, reservation.ID); got != 1 {
			t.Fatalf("canonical stored line count=%d, want 1", got)
		}
	})

	t.Run("duplicate quantity sum overflow writes nothing", func(t *testing.T) {
		stock := t04CreateStock(t, f, f.tokens["a"], f.storeA1, command.MaxQuantity)
		key := t04Key("reserve-overflow")
		_, err := t04Reserve(context.Background(), f, f.tokens["a"], f.storeA1, key, []inventory.Line{
			{WarehouseID: stock.warehouse.ID, SKUID: stock.skus[0].ID, Quantity: command.MaxQuantity},
			{WarehouseID: stock.warehouse.ID, SKUID: stock.skus[0].ID, Quantity: 1},
		})
		if !errors.Is(err, command.ErrInvalid) {
			t.Fatalf("duplicate overflow error=%v, want ErrInvalid", err)
		}
		if got := t04Count(t, f, `SELECT count(*) FROM ops.command_results WHERE tenant_id=$1 AND store_id=$2 AND idempotency_key=$3`, f.tenantA, f.storeA1, key); got != 0 {
			t.Fatalf("overflow command count=%d, want 0", got)
		}
	})

	t.Run("mixed valid and foreign lines write nothing", func(t *testing.T) {
		stock := t04CreateStock(t, f, f.tokens["a"], f.storeA1, 2)
		foreign := t04CreateStock(t, f, f.tokens["a2"], f.storeA2, 2)
		key := t04Key("reserve-atomic")
		_, err := t04Reserve(context.Background(), f, f.tokens["a"], f.storeA1, key, []inventory.Line{
			{WarehouseID: stock.warehouse.ID, SKUID: stock.skus[0].ID, Quantity: 1},
			{WarehouseID: foreign.warehouse.ID, SKUID: foreign.skus[0].ID, Quantity: 1},
		})
		if err == nil {
			t.Fatal("mixed-scope reservation succeeded")
		}
		if got := t04Count(t, f, `SELECT count(*) FROM ops.command_results WHERE tenant_id=$1 AND store_id=$2 AND idempotency_key=$3`, f.tenantA, f.storeA1, key); got != 0 {
			t.Fatalf("failed command result count=%d, want 0", got)
		}
		if got := t04Count(t, f, `SELECT reserved FROM inventory.balances WHERE tenant_id=$1 AND store_id=$2 AND warehouse_id=$3 AND sku_id=$4`, f.tenantA, f.storeA1, stock.warehouse.ID, stock.skus[0].ID); got != 0 {
			t.Fatalf("partial reserved=%d, want 0", got)
		}
	})
}

func TestT04ReleaseExpiryStatesAndRollback(t *testing.T) {
	f := t04Fixture(t)
	ctx := context.Background()

	t.Run("release is terminal and idempotent", func(t *testing.T) {
		stock := t04CreateStock(t, f, f.tokens["a"], f.storeA1, 2)
		reservation, err := t04Reserve(ctx, f, f.tokens["a"], f.storeA1, t04Key("held"), []inventory.Line{{WarehouseID: stock.warehouse.ID, SKUID: stock.skus[0].ID, Quantity: 1}})
		if err != nil {
			t.Fatal(err)
		}
		key := t04Key("release")
		first, err := t04Release(ctx, f, f.tokens["a"], f.storeA1, key, reservation.ID, false)
		if err != nil || first.State != "RELEASED" {
			t.Fatalf("release=%+v err=%v", first, err)
		}
		replay, err := t04Release(ctx, f, f.tokens["a"], f.storeA1, key, reservation.ID, false)
		if err != nil || replay.ID != first.ID || replay.State != first.State {
			t.Fatalf("release replay=%+v err=%v", replay, err)
		}
		terminal, err := t04Release(ctx, f, f.tokens["a"], f.storeA1, t04Key("release-terminal"), reservation.ID, false)
		if err != nil || terminal.State != "RELEASED" {
			t.Fatalf("terminal release=%+v err=%v", terminal, err)
		}
		if got := t04Count(t, f, `SELECT count(*) FROM inventory.ledger WHERE tenant_id=$1 AND store_id=$2 AND reservation_id=$3 AND kind='RELEASE'`, f.tenantA, f.storeA1, reservation.ID); got != 1 {
			t.Fatalf("release ledger count=%d, want 1", got)
		}
	})

	t.Run("pending and committed refuse release", func(t *testing.T) {
		stock := t04CreateStock(t, f, f.tokens["a"], f.storeA1, 2)
		for _, state := range []string{"PAYMENT_PENDING", "COMMITTED"} {
			reservation, err := t04Reserve(ctx, f, f.tokens["a"], f.storeA1, t04Key("state-held"), []inventory.Line{{WarehouseID: stock.warehouse.ID, SKUID: stock.skus[0].ID, Quantity: 1}})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := f.owner.Exec(ctx, `UPDATE inventory.reservations SET state=$1 WHERE tenant_id=$2 AND store_id=$3 AND id=$4`, state, f.tenantA, f.storeA1, reservation.ID); err != nil {
				t.Fatalf("seed %s state: %v", state, err)
			}
			_, err = t04Release(ctx, f, f.tokens["a"], f.storeA1, t04Key("refuse-state"), reservation.ID, false)
			if !errors.Is(err, command.ErrConflict) {
				t.Fatalf("state %s release error=%v, want ErrConflict", state, err)
			}
		}
	})

	t.Run("expiry requires elapsed database time", func(t *testing.T) {
		stock := t04CreateStock(t, f, f.tokens["a"], f.storeA1, 1)
		reservation, err := t04Reserve(ctx, f, f.tokens["a"], f.storeA1, t04Key("expire-held"), []inventory.Line{{WarehouseID: stock.warehouse.ID, SKUID: stock.skus[0].ID, Quantity: 1}})
		if err != nil {
			t.Fatal(err)
		}
		earlyKey := t04Key("expire-early")
		_, err = t04Release(ctx, f, f.tokens["a"], f.storeA1, earlyKey, reservation.ID, true)
		if !errors.Is(err, command.ErrConflict) {
			t.Fatalf("early expiry error=%v, want ErrConflict", err)
		}
		if got := t04Count(t, f, `SELECT count(*) FROM ops.command_results WHERE tenant_id=$1 AND store_id=$2 AND idempotency_key=$3`, f.tenantA, f.storeA1, earlyKey); got != 0 {
			t.Fatalf("early expiry command count=%d, want 0", got)
		}
		if _, err := f.owner.Exec(ctx, `UPDATE inventory.reservations SET expires_at=clock_timestamp()-interval '1 second' WHERE tenant_id=$1 AND store_id=$2 AND id=$3`, f.tenantA, f.storeA1, reservation.ID); err != nil {
			t.Fatalf("seed elapsed expiry: %v", err)
		}
		expired, err := t04Release(ctx, f, f.tokens["a"], f.storeA1, t04Key("expire-elapsed"), reservation.ID, true)
		if err != nil || expired.State != "EXPIRED" {
			t.Fatalf("elapsed expiry=%+v err=%v", expired, err)
		}
	})

	for _, tc := range []struct {
		name string
		err  error
	}{
		{"returned error", errors.New("T04 rollback sentinel")},
		{"canceled context", context.Canceled},
	} {
		t.Run("rollback "+tc.name, func(t *testing.T) {
			stock := t04CreateStock(t, f, f.tokens["a"], f.storeA1, 1)
			key := t04Key("rollback-reserve")
			auditBefore := t04Count(t, f, `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action='inventory.reserved'`, f.tenantA, f.storeA1)
			var rolled inventory.Reservation
			callCtx, cancel := context.WithCancel(context.Background())
			err := platform.WithScope(callCtx, f.runtime, f.tokens["a"], f.storeA1, "inventory:reserve", func(tx pgx.Tx, scope platform.Scope) error {
				var err error
				rolled, err = inventory.Reserve(callCtx, tx, scope, key, []inventory.Line{{WarehouseID: stock.warehouse.ID, SKUID: stock.skus[0].ID, Quantity: 1}})
				if err != nil {
					return err
				}
				if errors.Is(tc.err, context.Canceled) {
					cancel()
				}
				return tc.err
			})
			cancel()
			if !errors.Is(err, tc.err) {
				t.Fatalf("rollback error=%v, want %v", err, tc.err)
			}
			if rolled.ID == "" {
				t.Fatal("reserve did not reach mutation before rollback")
			}
			if got := t04Count(t, f, `SELECT count(*) FROM inventory.reservations WHERE tenant_id=$1 AND store_id=$2 AND id=$3`, f.tenantA, f.storeA1, rolled.ID); got != 0 {
				t.Fatalf("rolled reservation count=%d, want 0", got)
			}
			if got := t04Count(t, f, `SELECT count(*) FROM inventory.ledger WHERE tenant_id=$1 AND store_id=$2 AND command_key=$3`, f.tenantA, f.storeA1, key); got != 0 {
				t.Fatalf("rolled ledger count=%d, want 0", got)
			}
			if got := t04Count(t, f, `SELECT count(*) FROM ops.command_results WHERE tenant_id=$1 AND store_id=$2 AND idempotency_key=$3`, f.tenantA, f.storeA1, key); got != 0 {
				t.Fatalf("rolled command count=%d, want 0", got)
			}
			if got := t04Count(t, f, `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action='inventory.reserved'`, f.tenantA, f.storeA1); got != auditBefore {
				t.Fatalf("rolled audit count=%d, want %d", got, auditBefore)
			}
			if got := t04Count(t, f, `SELECT reserved FROM inventory.balances WHERE tenant_id=$1 AND store_id=$2 AND warehouse_id=$3 AND sku_id=$4`, f.tenantA, f.storeA1, stock.warehouse.ID, stock.skus[0].ID); got != 0 {
				t.Fatalf("rolled balance reserved=%d, want 0", got)
			}
		})
	}
}

func TestT04ArchivedCatalogRejectsNewReservationsButKeepsHistory(t *testing.T) {
	f := t04Fixture(t)
	ctx := context.Background()

	for _, archive := range []string{"product", "sku"} {
		t.Run(archive, func(t *testing.T) {
			stock := t04CreateStock(t, f, f.tokens["a"], f.storeA1, 2)
			historical, err := t04Reserve(ctx, f, f.tokens["a"], f.storeA1, t04Key("historical"), []inventory.Line{{WarehouseID: stock.warehouse.ID, SKUID: stock.skus[0].ID, Quantity: 1}})
			if err != nil {
				t.Fatal(err)
			}
			switch archive {
			case "product":
				_, err = t04Scoped(ctx, f, f.tokens["a"], f.storeA1, "catalog:write", func(tx pgx.Tx, scope platform.Scope) (catalog.Product, error) {
					return catalog.ArchiveProduct(ctx, tx, scope, t04Key("archive-product"), stock.product.ID, stock.product.Version)
				})
			case "sku":
				_, err = t04Scoped(ctx, f, f.tokens["a"], f.storeA1, "catalog:write", func(tx pgx.Tx, scope platform.Scope) (catalog.SKU, error) {
					return catalog.ArchiveSKU(ctx, tx, scope, t04Key("archive-sku"), stock.skus[0].ID, stock.skus[0].Version)
				})
			}
			if err != nil {
				t.Fatalf("archive %s: %v", archive, err)
			}
			_, err = t04Reserve(ctx, f, f.tokens["a"], f.storeA1, t04Key("reserve-archived"), []inventory.Line{{WarehouseID: stock.warehouse.ID, SKUID: stock.skus[0].ID, Quantity: 1}})
			if err == nil {
				t.Fatalf("reserve succeeded with archived %s", archive)
			}
			if got := t04Count(t, f, `SELECT count(*) FROM inventory.reservations WHERE tenant_id=$1 AND store_id=$2 AND id=$3`, f.tenantA, f.storeA1, historical.ID); got != 1 {
				t.Fatalf("historical reservation count=%d, want 1", got)
			}
			if got := t04Count(t, f, `SELECT count(*) FROM inventory.reservation_lines WHERE tenant_id=$1 AND store_id=$2 AND reservation_id=$3`, f.tenantA, f.storeA1, historical.ID); got != 1 {
				t.Fatalf("historical line count=%d, want 1", got)
			}
		})
	}
}
