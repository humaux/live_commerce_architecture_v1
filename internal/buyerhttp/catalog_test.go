package buyerhttp

import (
	"encoding/json"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"testing"

	"livecommerce/internal/command"
	"livecommerce/internal/pagination"
	"livecommerce/internal/storefront"
)

func TestBuyerHTTPCatalogQueryAdmission(t *testing.T) {
	if matchRoute("/v1/buyer/catalog").kind != catalogRoute || !allowed(catalogRoute, http.MethodGet) || allowed(catalogRoute, http.MethodPost) || matchRoute("/v1/buyer/catalog/").kind != unknownRoute {
		t.Fatal("catalog must be an exact GET-only route")
	}
	for _, path := range []string{"/v1/buyer/catalog?limit=1", "/v1/buyer/cart?limit=1", "/v1/buyer/catalog?"} {
		r, err := http.NewRequest(http.MethodGet, "https://shop.example"+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		wantForbidden := path != "/v1/buyer/catalog?limit=1"
		if forbiddenInput(r) != wantForbidden {
			t.Fatalf("query admission %q", path)
		}
	}
	request, err := catalogRequest("product_id=11111111-1111-1111-1111-111111111111&limit=2&cursor=opaque")
	if err != nil || request.ProductID != "11111111-1111-1111-1111-111111111111" || request.Page.Limit != 2 || request.Page.Cursor != "opaque" {
		t.Fatal("valid bounded query rejected")
	}
	for _, raw := range []string{
		"product_id=", "product_id=ABC", "limit=", "limit=0", "limit=101", "limit=-1", "limit=1.0", "limit=+1", "limit=1&limit=2", "limit=1&", "&limit=1", "limit=1&&cursor=x", "cursor=", "cursor=x&cursor=y", "unknown=x", "limit=1;cursor=x", "limit=%GG", strings.Repeat("x", 2049),
	} {
		if _, err := catalogRequest(raw); err != command.ErrInvalid {
			t.Fatalf("accepted invalid query %q: %v", raw, err)
		}
	}
}

var imageWidth = 7

func TestBuyerHTTPCatalogProjectionExactKeys(t *testing.T) {
	page := pagination.Page[storefront.CatalogItem]{Items: []storefront.CatalogItem{{
		ProductID: "product", SKUID: "sku", Name: "name", Description: "description", SKUCode: "code", Currency: "USD", PriceMinor: 123,
		Images: []storefront.CatalogImage{{ID: "img", Width: &imageWidth}},
	}}, NextCursor: "next"}
	projected := projectCatalog(page)
	projected.StoreName = "Shop"
	raw, err := json.Marshal(projected)
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]json.RawMessage
	if err = json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(sortedKeys(body), []string{"items", "next_cursor", "store_name"}) {
		t.Fatal("unexpected page keys")
	}
	var items []map[string]json.RawMessage
	if err = json.Unmarshal(body["items"], &items); err != nil || len(items) != 1 {
		t.Fatal("missing item")
	}
	if !reflect.DeepEqual(sortedKeys(items[0]), []string{"currency", "description", "images", "name", "price_minor", "product_id", "sku_code", "sku_id"}) {
		t.Fatal("unexpected item keys")
	}
	var images []map[string]json.RawMessage
	if err = json.Unmarshal(items[0]["images"], &images); err != nil || len(images) != 1 ||
		!reflect.DeepEqual(sortedKeys(images[0]), []string{"height", "id", "width"}) || string(images[0]["height"]) != "null" || string(images[0]["width"]) != "7" {
		t.Fatalf("unexpected image keys: %s", items[0]["images"])
	}
	noImages, _ := json.Marshal(projectCatalog(pagination.Page[storefront.CatalogItem]{Items: []storefront.CatalogItem{{ProductID: "p"}}}))
	if !strings.Contains(string(noImages), `"images":[]`) {
		t.Fatal("a product without photos must project images as []")
	}
	empty, err := json.Marshal(projectCatalog(pagination.Page[storefront.CatalogItem]{}))
	if err != nil || !strings.Contains(string(empty), `"items":[]`) {
		t.Fatal("empty items must be []")
	}
}

func sortedKeys(values map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
