package foundation_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"livecommerce/internal/catalog"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/inventory"
)

func TestCatalogInventoryHTTPRealWorkflow(t *testing.T) {
	f := t04Fixture(t)
	h := httpapi.NewHandler(f.runtime)
	base := "/v1/admin/stores/" + f.storeA1
	request := func(method, path, key string, payload any, want int, out any) {
		t.Helper()
		var body []byte
		if payload != nil {
			var err error
			body, err = json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
		}
		r := httptest.NewRequest(method, path, bytes.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+f.tokens["a"])
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Idempotency-Key", key)
		r.Header.Set("X-Tenant-ID", f.tenantB)
		r.Host = "untrusted.attacker.invalid"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s %s: status=%d body=%s", method, path, w.Code, w.Body.String())
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("cacheable admin result")
		}
		if out != nil {
			if err := json.Unmarshal(w.Body.Bytes(), out); err != nil {
				t.Fatal(err)
			}
		}
	}
	key := t04Key("http-product")
	in := catalog.ProductInput{Name: "API商品-" + t04Tag(), Description: "商品説明"}
	var p, replay catalog.Product
	request("POST", base+"/products", key, in, 200, &p)
	request("POST", base+"/products", key, in, 200, &replay)
	if p != replay || p.ID == "" || p.Version != 1 {
		t.Fatalf("bad create/replay: %+v %+v", p, replay)
	}
	in.Name = "Changed payload"
	request("POST", base+"/products", key, in, 409, nil)
	var products []catalog.Product
	request("GET", base+"/products", "", nil, 200, &products)
	if len(products) == 0 || len(products) > 100 {
		t.Fatalf("bad product list %d", len(products))
	}
	in.ExpectedVersion = 1
	request("PATCH", base+"/products/"+p.ID, t04Key("http-update"), in, 200, &p)
	if p.Name != in.Name || p.Version != 2 {
		t.Fatal("update not persisted")
	}
	request("PATCH", base+"/products/"+p.ID, t04Key("http-stale"), in, 409, nil)
	var sku catalog.SKU
	sin := catalog.SKUInput{ProductID: p.ID, Code: "http-" + t04Tag(), PriceMinor: 12900, WeightGrams: 250, OriginCountry: "CN", CustomsName: "試用商品"}
	request("POST", base+"/skus", t04Key("http-sku"), sin, 200, &sku)
	if sku.Currency != "USD" || sku.PriceMinor != 12900 {
		t.Fatalf("wrong server currency/price: %+v", sku)
	}
	sin.ExpectedVersion = sku.Version
	sin.WeightGrams = 300
	request("PATCH", base+"/skus/"+sku.ID, t04Key("http-sku-edit"), sin, 200, &sku)
	request("POST", base+"/skus/"+sku.ID+"/price", t04Key("http-price"), catalog.PriceInput{PriceMinor: 13500, ExpectedVersion: sku.Version}, 200, &sku)
	var skus []catalog.SKU
	request("GET", base+"/products/"+p.ID+"/skus", "", nil, 200, &skus)
	if len(skus) != 1 || skus[0].PriceMinor != 13500 || skus[0].WeightGrams != 300 {
		t.Fatal("sku readback mismatch")
	}
	var warehouse inventory.Warehouse
	request("POST", base+"/warehouses", t04Key("http-warehouse"), map[string]string{"name": "HTTP仓-" + t04Tag()}, 200, &warehouse)
	var warehouses []inventory.Warehouse
	request("GET", base+"/warehouses", "", nil, 200, &warehouses)
	if warehouse.ID == "" || len(warehouses) == 0 || len(warehouses) > 100 {
		t.Fatal("warehouse readback mismatch")
	}
	var balance inventory.Balance
	adjust := inventory.Adjustment{WarehouseID: warehouse.ID, SKUID: sku.ID, Delta: 5, Reason: "isolated HTTP acceptance"}
	adjustKey := t04Key("http-adjust")
	request("POST", base+"/inventory/adjustments", adjustKey, adjust, 200, &balance)
	request("POST", base+"/inventory/adjustments", adjustKey, adjust, 200, &balance)
	if balance.OnHand != 5 || balance.Available != 5 || balance.Version != 1 {
		t.Fatalf("duplicate adjustment: %+v", balance)
	}
	var balances []inventory.Balance
	request("GET", base+"/inventory", "", nil, 200, &balances)
	found := false
	for _, b := range balances {
		if b.SKUID == sku.ID {
			found = true
			if b != balance {
				t.Fatal("inventory readback mismatch")
			}
		}
	}
	if !found {
		t.Fatal("missing inventory")
	}
	request("POST", base+"/skus/"+sku.ID+"/archive", t04Key("http-archive-sku"), map[string]int64{"expected_version": sku.Version}, 200, &sku)
	request("POST", base+"/products/"+p.ID+"/archive", t04Key("http-archive-product"), map[string]int64{"expected_version": p.Version}, 200, &p)
	if sku.Status != "archived" || p.Status != "archived" {
		t.Fatal("archive status not persisted")
	}
	request("POST", fmt.Sprintf("/v1/admin/stores/%s/products", f.storeA2), t04Key("http-cross-store"), in, 401, nil)
}

func TestCatalogHTTPPermissionIsRouteOwned(t *testing.T) {
	f := t04Fixture(t)
	h := httpapi.NewHandler(f.runtime)
	// Expired/buyer sessions remain denied despite valid body and spoofed headers.
	for _, who := range []string{"buyer", "expired", "revoked"} {
		r := httptest.NewRequest("POST", "/v1/admin/stores/"+f.storeA1+"/products", bytes.NewBufferString(`{"name":"unauthorized"}`))
		r.Header.Set("Authorization", "Bearer "+f.tokens[who])
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Idempotency-Key", t04Key("denied"))
		r.Header.Set("X-Permission", "catalog:write")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 401 {
			t.Fatalf("%s status %d", who, w.Code)
		}
	}
}
