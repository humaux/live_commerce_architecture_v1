package buyerhttp

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"livecommerce/internal/checkout"
	"livecommerce/internal/command"
	"livecommerce/internal/pagination"
)

func TestBuyerHTTPOptionsQueryAdmission(t *testing.T) {
	if matchRoute("/v1/buyer/checkout-options").kind != optionsRoute || !allowed(optionsRoute, http.MethodGet) || allowed(optionsRoute, http.MethodPost) || matchRoute("/v1/buyer/checkout-options/").kind != unknownRoute {
		t.Fatal("options must be an exact GET-only route")
	}
	for _, item := range []struct {
		method, path string
		forbidden    bool
	}{
		{http.MethodGet, "/v1/buyer/checkout-options?limit=1", false},
		{http.MethodGet, "/v1/buyer/catalog?limit=1", false},
		{http.MethodGet, "/v1/buyer/cart?limit=1", true},
		{http.MethodPost, "/v1/buyer/checkout-options?limit=1", true},
		{http.MethodGet, "/v1/buyer/checkout-options?", true},
	} {
		r, err := http.NewRequest(item.method, "https://shop.example"+item.path, nil)
		if err != nil || forbiddenInput(r) != item.forbidden {
			t.Fatalf("query boundary %s %s: %v", item.method, item.path, err)
		}
	}
	request, err := optionsRequest("market_id=11111111-1111-1111-1111-111111111111&country=TW&limit=2&cursor=opaque")
	if err != nil || request.MarketID != "11111111-1111-1111-1111-111111111111" || request.Country != "TW" || request.Page.Limit != 2 || request.Page.Cursor != "opaque" {
		t.Fatal("valid query rejected")
	}
	for _, raw := range []string{
		"market_id=", "market_id=ABC", "country=", "country=tw", "country=T", "limit=", "limit=0", "limit=101", "limit=-1", "limit=1.0", "limit=1&limit=2", "cursor=", "country=TW&country=TW", "unknown=x", "limit=1;country=TW", "limit=%GG", "&country=TW", "country=TW&", "limit=1&&country=TW", strings.Repeat("x", 2049),
	} {
		if _, err := optionsRequest(raw); err != command.ErrInvalid {
			t.Fatalf("accepted invalid query %q: %v", raw, err)
		}
	}
}

func TestBuyerHTTPOptionsProjectionExactKeys(t *testing.T) {
	page := pagination.Page[checkout.Option]{Items: []checkout.Option{{
		MarketID: "market", MarketCode: "tw", MarketName: "Taiwan", Country: "TW", Currency: "TWD",
		DeliveryCode: "home", Method: "delivery:home", ServiceVersion: 2, AllocationVersion: 3,
		DeliveryKind: "home", Mode: "MANUAL", NameHans: "简体", NameHant: "繁體", NameEN: "Home", SortOrder: 7,
	}}, NextCursor: "next"}
	raw, err := json.Marshal(projectOptions(page))
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]json.RawMessage
	if err = json.Unmarshal(raw, &body); err != nil || !reflect.DeepEqual(sortedKeys(body), []string{"items", "next_cursor"}) {
		t.Fatal("wrong page keys")
	}
	var items []map[string]json.RawMessage
	if err = json.Unmarshal(body["items"], &items); err != nil || len(items) != 1 {
		t.Fatal("wrong items")
	}
	if !reflect.DeepEqual(sortedKeys(items[0]), []string{"allocation_version", "country", "currency", "delivery_code", "delivery_kind", "market_code", "market_id", "market_name", "method", "mode", "name_en", "name_hans", "name_hant", "service_version", "sort_order"}) {
		t.Fatal("wrong option keys")
	}
	empty, err := json.Marshal(projectOptions(pagination.Page[checkout.Option]{}))
	if err != nil || !strings.Contains(string(empty), `"items":[]`) {
		t.Fatal("empty options must be []")
	}
}
