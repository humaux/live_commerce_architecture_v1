package foundation_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/buyerhttp"
	"livecommerce/internal/fulfillment"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/platform"
	"livecommerce/internal/pricing"
)

// delivery-allocation P0 (backend): a merchant only enables a delivery service without configuring
// "warehouse allocation". These tests prove the merchant settings route auto-creates the allocation in
// the same transaction (so the buyer sees the option), that concurrent enables stay at one head, and
// that the one-time backfill migration fills the pre-fix gap idempotently.

// bhAutoSetup enables one home service through the real merchant settings route (httpapi.NewHandler,
// the same Go route the admin UI hits) and then wires the real buyer HTTP checkout-options handler
// around the same store, exactly like bhSetup but without any dsSet/daSet Go fixture.
func bhAutoSetup(t *testing.T) (bhHarness, fulfillment.ServiceInput, *httptest.ResponseRecorder) {
	t.Helper()
	h, in := dsSetup(t)
	handler := httpapi.NewHandler(h.f.runtime)
	body, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	path := "/v1/admin/stores/" + h.f.storeA1 + "/markets/" + h.market.ID + "/countries/TW/delivery-services/" + in.Code
	rec := adminRequest(handler, http.MethodPut, path, h.f.tokens["a"], body, "application/json", map[string]string{"Idempotency-Key": t04Key("auto-service")})

	url := bcRole(t, h.f, "commerce_checkout_runtime")
	pool, err := platform.OpenCheckoutPool(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	service := bcService(t, pool)
	key := base64.RawURLEncoding.EncodeToString(randomBytes(32))
	origin := "https://buyer.example"
	b := bcHarness{cqHarness: h, pool: pool, service: service, poolURL: url}
	bhPublish(t, b, origin, h.f.tenantA, h.f.storeA1)
	bhHandler, err := buyerhttp.New(context.Background(), h.a.issuer, h.a.runtime, service, key, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(bhHandler)
	t.Cleanup(srv.Close)
	return bhHarness{bcHarness: b, server: srv, key: key, origin: origin}, in, rec
}

func TestDeliveryAllocationAutoMerchantHTTPShowsBuyerOption(t *testing.T) {
	h, in, rec := bhAutoSetup(t)
	if rec.Code != http.StatusOK {
		t.Fatalf("settings delivery-service enable status=%d body=%s", rec.Code, rec.Body.String())
	}
	var svc fulfillment.Service
	if err := json.Unmarshal(rec.Body.Bytes(), &svc); err != nil {
		t.Fatal(err)
	}
	if !svc.Enabled || svc.Version != 1 || svc.DefaultWarehouseID == "" {
		t.Fatalf("auto-allocation response: %+v", svc)
	}
	// The response's default warehouse must be the one actually persisted as the allocation's position-1 child.
	var allocated string
	if err := h.f.owner.QueryRow(context.Background(), `SELECT warehouse_id::text FROM fulfillment.allocation_warehouses
		WHERE market_id=$1 AND country=$2 AND code=$3 AND version=1 AND position=1`,
		in.MarketID, in.Country, in.Code).Scan(&allocated); err != nil {
		t.Fatal(err)
	}
	if allocated != svc.DefaultWarehouseID {
		t.Fatalf("response default=%q but allocated=%q", svc.DefaultWarehouseID, allocated)
	}
	// The buyer sees and can select the service in checkout options (the exact pre-fix regression).
	found := false
	for _, row := range boptRead(t, h, "market_id="+h.market.ID+"&country=TW").Items {
		if row.DeliveryCode != in.Code {
			continue
		}
		found = true
		if row.ServiceVersion != 1 || row.AllocationVersion != 1 || row.DeliveryKind != "home" {
			t.Fatalf("option projection: %+v", row)
		}
	}
	if !found {
		t.Fatalf("enabled service %q missing from buyer options", in.Code)
	}
}

func TestDeliveryAllocationAutoConcurrentEnableSingleHead(t *testing.T) {
	h, in := dsSetup(t)
	handler := httpapi.NewHandler(h.f.runtime)
	body, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	path := "/v1/admin/stores/" + h.f.storeA1 + "/markets/" + h.market.ID + "/countries/TW/delivery-services/" + in.Code
	// Same idempotency key across N concurrent enables: the command receipt replays, and the
	// allocation advisory lock + existence re-check must still yield exactly one head.
	key := t04Key("auto-concurrent")
	const n = 4
	recs := make([]*httptest.ResponseRecorder, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			recs[i] = adminRequest(handler, http.MethodPut, path, h.f.tokens["a"], body, "application/json", map[string]string{"Idempotency-Key": key})
		}(i)
	}
	wg.Wait()
	first := fulfillment.Service{}
	for i, rec := range recs {
		if rec.Code != http.StatusOK {
			t.Fatalf("concurrent enable[%d] status=%d body=%s", i, rec.Code, rec.Body.String())
		}
		var svc fulfillment.Service
		if err := json.Unmarshal(rec.Body.Bytes(), &svc); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = svc
		} else if svc.DefaultWarehouseID != first.DefaultWarehouseID {
			t.Fatalf("concurrent responses disagree on default warehouse: %q vs %q", svc.DefaultWarehouseID, first.DefaultWarehouseID)
		}
	}
	if first.DefaultWarehouseID == "" {
		t.Fatal("auto-allocation returned no default warehouse")
	}
	for _, check := range []struct{ table, want string }{
		{"fulfillment.service_versions", "1"}, {"fulfillment.service_heads", "1"},
		{"fulfillment.allocation_versions", "1"}, {"fulfillment.allocation_warehouses", "1"}, {"fulfillment.allocation_heads", "1"},
	} {
		if n := countRows(t, h.f.owner, `SELECT count(*) FROM `+check.table+` WHERE market_id=$1`, h.market.ID); n != 1 {
			t.Fatalf("%s rows=%d want=%s", check.table, n, check.want)
		}
	}
}

// runBackfill applies the raw migration file (the same statement text the migration runner executes),
// in one owner transaction, so the test exercises the shipped SQL rather than a re-typed copy.
func runBackfill(t *testing.T, h cqHarness) {
	t.Helper()
	body, err := os.ReadFile("../../migrations/0117_delivery_allocation_backfill.sql")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := h.f.owner.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(context.Background(), string(body)); err != nil {
		t.Fatalf("apply backfill migration: %v", err)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestDeliveryAllocationBackfillMigration(t *testing.T) {
	h, in := dsSetup(t)
	// The pre-fix state: an enabled service with no allocation head.
	if _, err := dsSet(h, t04Key("backfill-enabled"), in); err != nil {
		t.Fatal(err)
	}
	// A disabled service must never be backfilled.
	off := in
	off.Code = "off_" + t04Tag()
	off.Enabled = false
	dsPolicy(t, h, off, 0, 30, true)
	if _, err := dsSet(h, t04Key("backfill-disabled"), off); err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, h.f.owner, `SELECT count(*) FROM fulfillment.allocation_heads WHERE market_id=$1`, h.market.ID); n != 0 {
		t.Fatal("pre-existing allocation heads before backfill")
	}

	runBackfill(t, h)

	// The enabled service got exactly one single-warehouse head.
	var version, serviceVersion, warehouseCount int64
	if err := h.f.owner.QueryRow(context.Background(), `SELECT version,service_version,warehouse_count FROM fulfillment.allocation_versions
		WHERE market_id=$1 AND code=$2`, h.market.ID, in.Code).Scan(&version, &serviceVersion, &warehouseCount); err != nil {
		t.Fatalf("backfilled version: %v", err)
	}
	if version != 1 || serviceVersion != 1 || warehouseCount != 1 {
		t.Fatalf("backfilled allocation version=%d service_version=%d count=%d", version, serviceVersion, warehouseCount)
	}
	var current int64
	if err := h.f.owner.QueryRow(context.Background(), `SELECT current_version FROM fulfillment.allocation_heads WHERE market_id=$1 AND code=$2`,
		h.market.ID, in.Code).Scan(&current); err != nil || current != 1 {
		t.Fatalf("backfilled head current=%d err=%v", current, err)
	}
	// The chosen warehouse belongs to the store and is active.
	var active bool
	if err := h.f.owner.QueryRow(context.Background(), `SELECT w.active FROM fulfillment.allocation_warehouses a
		JOIN inventory.warehouses w ON w.tenant_id=a.tenant_id AND w.store_id=a.store_id AND w.id=a.warehouse_id
		WHERE a.market_id=$1 AND a.code=$2 AND a.version=1 AND a.position=1`, h.market.ID, in.Code).Scan(&active); err != nil || !active {
		t.Fatalf("backfilled warehouse active=%t err=%v", active, err)
	}
	// The disabled service stayed without a head.
	if n := countRows(t, h.f.owner, `SELECT count(*) FROM fulfillment.allocation_heads WHERE market_id=$1 AND code=$2`, h.market.ID, off.Code); n != 0 {
		t.Fatal("disabled service was backfilled")
	}

	// Re-running is a no-op: no new version, same single head.
	runBackfill(t, h)
	if n := countRows(t, h.f.owner, `SELECT count(*) FROM fulfillment.allocation_versions WHERE market_id=$1 AND code=$2`, h.market.ID, in.Code); n != 1 {
		t.Fatalf("backfill re-run created history rows=%d", n)
	}
	if err := h.f.owner.QueryRow(context.Background(), `SELECT current_version FROM fulfillment.allocation_heads WHERE market_id=$1 AND code=$2`,
		h.market.ID, in.Code).Scan(&current); err != nil || current != 1 {
		t.Fatalf("backfill re-run changed head current=%d err=%v", current, err)
	}
}

// TestDeliveryAllocationAutoWarehouseDefaultAndNoWarehouse covers the two warehouse-selection
// invariants on a fresh store (the shared storeA1 accumulates warehouses from other tests, so its
// "first active warehouse" is not deterministic here).
func TestDeliveryAllocationAutoWarehouseDefaultAndNoWarehouse(t *testing.T) {
	ctx := context.Background()
	f := fixture(t)
	store := randomUUID()
	mustExec(t, f.owner, `INSERT INTO control.stores(tenant_id,id,name,currency) VALUES($1,$2,$3,'USD')`, f.tenantA, store, "auto-"+t04Tag())
	mustExec(t, f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES
		($1,$2,$3,'store:read'),($1,$2,$3,'integration:manage'),($1,$2,$3,'integration:read'),($1,$2,$3,'pricing:write'),($1,$2,$3,'pricing:read')`,
		f.tenantA, store, f.principalA)
	market, err := pricingScoped(ctx, f, f.tokens["a"], store, "pricing:write", func(tx pgx.Tx, s platform.Scope) (pricing.Market, error) {
		return pricing.CreateMarket(ctx, tx, s, t04Key("market"), pricing.MarketInput{Code: "auto_" + t04Tag(), Name: "auto market", Currency: "USD"})
	})
	if err != nil {
		t.Fatal(err)
	}
	in := fulfillment.ServiceInput{MarketID: market.ID, Country: "TW", Code: "home_" + t04Tag(), PolicyVersion: 1,
		NameHans: "标准配送", NameHant: "標準配送", NameEN: "Standard delivery", DeliveryKind: "home", Mode: "MANUAL", Enabled: true, Visible: true, SortOrder: 10}
	shipping, tax := int64(50), int64(500)
	pol := pricing.PolicyInput{MarketID: market.ID, Country: "TW", Method: "delivery:" + in.Code, Currency: "USD", ShippingMode: "country_flat", ShippingMinor: &shipping, TaxMode: "exclusive", TaxBasis: "goods_and_shipping", TaxRateBPS: &tax, QuoteTTLSeconds: 60, Enabled: true, ConfigurationRef: "synthetic"}
	if _, err := pricingScoped(ctx, f, f.tokens["a"], store, "pricing:write", func(tx pgx.Tx, s platform.Scope) (pricing.Policy, error) {
		return pricing.SetPolicy(ctx, tx, s, t04Key("auto-policy"), pol)
	}); err != nil {
		t.Fatal(err)
	}

	handler := httpapi.NewHandler(f.runtime)
	body, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	base := "/v1/admin/stores/" + store + "/markets/" + market.ID + "/countries/TW/delivery-services/" + in.Code

	// No active warehouse: explicit conflict, and the service write rolls back with it (never a silent
	// "enabled but no allocation").
	rec := adminRequest(handler, http.MethodPut, base, f.tokens["a"], body, "application/json", map[string]string{"Idempotency-Key": t04Key("auto-nowh")})
	assertAdminError(t, rec, http.StatusConflict, "conflict")
	if n := countRows(t, f.owner, `SELECT count(*) FROM fulfillment.service_heads WHERE market_id=$1`, market.ID); n != 0 {
		t.Fatal("service head persisted despite no-warehouse conflict")
	}

	// Two active warehouses with an explicit creation order: the earlier one is the default.
	earlier, later := randomUUID(), randomUUID()
	mustExec(t, f.owner, `INSERT INTO inventory.warehouses(tenant_id,store_id,id,name,created_at) VALUES($1,$2,$3,'earlier','2020-01-01'),($1,$2,$4,'later','2021-01-01')`, f.tenantA, store, earlier, later)
	rec = adminRequest(handler, http.MethodPut, base, f.tokens["a"], body, "application/json", map[string]string{"Idempotency-Key": t04Key("auto-wh")})
	if rec.Code != http.StatusOK {
		t.Fatalf("enable with warehouses status=%d body=%s", rec.Code, rec.Body.String())
	}
	var svc fulfillment.Service
	if err := json.Unmarshal(rec.Body.Bytes(), &svc); err != nil {
		t.Fatal(err)
	}
	if svc.DefaultWarehouseID != earlier {
		t.Fatalf("default warehouse=%q want earlier=%q", svc.DefaultWarehouseID, earlier)
	}
}
