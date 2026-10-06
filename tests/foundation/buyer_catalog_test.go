package foundation_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/catalog"
	"livecommerce/internal/platform"
)

// This test-owned wire shape must not follow a broadened merchant/domain DTO.
type bcatItem struct {
	ProductID   string `json:"product_id"`
	SKUID       string `json:"sku_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	SKUCode     string `json:"sku_code"`
	Currency    string `json:"currency"`
	PriceMinor  int64  `json:"price_minor"`
}

type bcatPage struct {
	Items      []bcatItem `json:"items"`
	NextCursor string     `json:"next_cursor"`
}

func bcatRead(t *testing.T, h bhHarness, query string) bcatPage {
	t.Helper()
	path := "/v1/buyer/catalog"
	if query != "" {
		path += "?" + query
	}
	r := h.request(t, "GET", path, h.cap.Token, "", nil, nil)
	out := bhRead[bcatPage](t, r, 200)
	var raw struct {
		Items []map[string]json.RawMessage `json:"items"`
	}
	var page map[string]json.RawMessage
	if json.Unmarshal(r.body, &raw) != nil || json.Unmarshal(r.body, &page) != nil ||
		len(page) != 3 || page["items"] == nil || page["next_cursor"] == nil || page["store_name"] == nil || out.Items == nil {
		t.Fatal("catalog page is not an exact non-null page projection")
	}
	want := []string{"currency", "description", "image_id", "images", "name", "price_minor", "product_id", "sku_code", "sku_id"}
	for _, item := range raw.Items {
		keys := make([]string, 0, len(item))
		for key := range item {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		if !slices.Equal(keys, want) {
			t.Fatal("catalog item exposes missing or extra fields")
		}
	}
	return out
}

func bcatForeignStore(t *testing.T, h bhHarness, tenant string) (store, product string) {
	t.Helper()
	store, product = randomUUID(), randomUUID()
	// Own fresh stores in the disposable test database. Other foundation gates
	// intentionally reserve storeA2/storeB as empty isolation negative controls.
	// These synthetic read-only targets need no stock ledger or merchant writes.
	mustExec(t, h.f.owner, `INSERT INTO control.stores(tenant_id,id,name,currency) VALUES($1,$2,'BCAT isolated target','USD')`, tenant, store)
	mustExec(t, h.f.owner, `INSERT INTO catalog.products(tenant_id,store_id,id,name) VALUES($1,$2,$3,'BCAT foreign product')`, tenant, store, product)
	mustExec(t, h.f.owner, `INSERT INTO catalog.skus(tenant_id,store_id,id,product_id,code,currency,price_minor)
		VALUES($1,$2,$3,$4,'BCAT-FOREIGN','USD',1250)`, tenant, store, randomUUID(), product)
	return store, product
}

func TestBuyerHTTPCatalogPaginationScopeAndReadOnly(t *testing.T) {
	h := bhSetup(t)
	otherStore, otherProduct := bcatForeignStore(t, h, h.f.tenantA)
	_, foreignProduct := bcatForeignStore(t, h, h.f.tenantB)
	before := h.facts(t)
	readFacts := func() []int {
		out := make([]int, 0, 5)
		for _, table := range []string{"storefront.carts", "storefront.quotes", "storefront.events", "buyer.command_results"} {
			out = append(out, countRows(t, h.f.owner, `SELECT count(*) FROM `+table+` WHERE owner_id=$1`, h.cap.Scope.OwnerID))
		}
		out = append(out, countRows(t, h.f.owner, `SELECT count(*) FROM inventory.ledger WHERE sku_id=ANY($1::uuid[])`,
			[]string{h.stock.skus[0].ID, h.stock.skus[1].ID}))
		return out
	}
	beforeRead := readFacts()
	query := "product_id=" + h.stock.product.ID + "&limit=1"
	first := bcatRead(t, h, query)
	if len(first.Items) != 1 || first.NextCursor == "" {
		t.Fatal("first page did not bound results and return a continuation")
	}
	cursor, err := base64.RawURLEncoding.DecodeString(first.NextCursor)
	if err != nil {
		t.Fatal("cursor is not base64url")
	}
	for _, sensitive := range []string{h.f.tenantA, h.f.storeA1, h.cap.Scope.OwnerID, h.cap.Scope.SessionID, h.cap.Token} {
		if strings.Contains(string(cursor), sensitive) {
			t.Fatal("cursor exposes scope identity or credential")
		}
	}
	second := bcatRead(t, h, query+"&cursor="+url.QueryEscape(first.NextCursor))
	if len(second.Items) != 1 || second.NextCursor != "" || first.Items[0].SKUID >= second.Items[0].SKUID {
		t.Fatal("keyset page overlaps, misorders, or has a false continuation")
	}
	for _, row := range append(first.Items, second.Items...) {
		var found bool
		for _, sku := range h.stock.skus {
			if row.SKUID == sku.ID {
				found = true
				if row.ProductID != h.stock.product.ID || row.Name != h.stock.product.Name || row.Description != h.stock.product.Description ||
					row.SKUCode != sku.Code || row.Currency != sku.Currency || row.PriceMinor != sku.PriceMinor {
					t.Fatal("catalog projection changed real persisted fields")
				}
			}
		}
		if !found {
			t.Fatal("product filter leaked another SKU")
		}
	}
	for _, product := range []string{otherProduct, foreignProduct, randomUUID()} {
		page := bcatRead(t, h, "product_id="+product)
		if len(page.Items) != 0 || page.NextCursor != "" {
			t.Fatal("foreign or missing product is distinguishable from empty")
		}
	}
	defaultPage := bcatRead(t, h, "")
	if len(defaultPage.Items) == 0 || len(defaultPage.Items) > 50 {
		t.Fatal("unfiltered catalog default limit failed")
	}
	// Shared fixture stores contain earlier tests' catalog rows. Validate each
	// returned ID against the intended scope instead of asserting a global count.
	for _, item := range defaultPage.Items {
		if countRows(t, h.f.owner, `SELECT count(*) FROM catalog.skus s JOIN catalog.products p
			ON (p.tenant_id,p.store_id,p.id)=(s.tenant_id,s.store_id,s.product_id)
			WHERE s.tenant_id=$1 AND s.store_id=$2 AND s.id=$3 AND s.status='active' AND p.status='active'`,
			h.f.tenantA, h.f.storeA1, item.SKUID) != 1 {
			t.Fatal("unfiltered result is outside active buyer store catalog")
		}
	}
	bhError(t, h.request(t, "GET", "/v1/buyer/catalog?product_id="+otherProduct+"&cursor="+first.NextCursor,
		h.cap.Token, "", nil, nil), 422, "invalid_request")
	bhPublish(t, h.bcHarness, "https://catalog-other.example", h.f.tenantA, otherStore)
	otherCap := mustIssue(t, h.cqHarness.service, otherStore)
	positive := bhRead[bcatPage](t, h.request(t, "GET", "/v1/buyer/catalog?product_id="+otherProduct,
		otherCap.Token, "", nil, func(r *http.Request) { r.Header.Set("X-Commerce-Storefront-Origin", "https://catalog-other.example") }), 200)
	if len(positive.Items) != 1 || positive.Items[0].ProductID != otherProduct {
		t.Fatal("other-store isolation negative control lacks a readable positive")
	}
	bhError(t, h.request(t, "GET", "/v1/buyer/catalog?product_id="+h.stock.product.ID+"&cursor="+first.NextCursor,
		otherCap.Token, "", nil, func(r *http.Request) { r.Header.Set("X-Commerce-Storefront-Origin", "https://catalog-other.example") }), 422, "invalid_request")
	if h.facts(t) != before || !reflect.DeepEqual(readFacts(), beforeRead) {
		t.Fatal("catalog reads changed durable purchase facts")
	}
}

func TestBuyerHTTPCatalogCurrentPriceAndArchive(t *testing.T) {
	h := bhSetup(t)
	query := "product_id=" + h.stock.product.ID
	if len(bcatRead(t, h, query).Items) != len(h.stock.skus) {
		t.Fatal("initial active catalog is missing rows")
	}
	ctx := context.Background()
	updated, err := t04Scoped(ctx, h.f, h.f.tokens["a"], h.f.storeA1, "catalog:write", func(tx pgx.Tx, s platform.Scope) (catalog.SKU, error) {
		return catalog.SetSKUPrice(ctx, tx, s, t04Key("bcat-price"), h.stock.skus[0].ID,
			catalog.PriceInput{PriceMinor: 1777, ExpectedVersion: h.stock.skus[0].Version})
	})
	if err != nil {
		t.Fatal(err)
	}
	var seen bool
	for _, item := range bcatRead(t, h, query).Items {
		if item.SKUID == updated.ID {
			seen = item.PriceMinor == 1777
		}
	}
	if !seen {
		t.Fatal("read did not observe the current merchant price")
	}
	_, err = t04Scoped(ctx, h.f, h.f.tokens["a"], h.f.storeA1, "catalog:write", func(tx pgx.Tx, s platform.Scope) (catalog.SKU, error) {
		return catalog.ArchiveSKU(ctx, tx, s, t04Key("bcat-sku-archive"), updated.ID, updated.Version)
	})
	if err != nil {
		t.Fatal(err)
	}
	remaining := bcatRead(t, h, query)
	if len(remaining.Items) != 1 || remaining.Items[0].SKUID == updated.ID {
		t.Fatal("archived SKU remains discoverable")
	}
	_, err = t04Scoped(ctx, h.f, h.f.tokens["a"], h.f.storeA1, "catalog:write", func(tx pgx.Tx, s platform.Scope) (catalog.Product, error) {
		return catalog.ArchiveProduct(ctx, tx, s, t04Key("bcat-product-archive"), h.stock.product.ID, h.stock.product.Version)
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(bcatRead(t, h, query).Items) != 0 {
		t.Fatal("archived product remains discoverable")
	}
}

func TestBuyerHTTPCatalogStrictQueryAndAuthority(t *testing.T) {
	h := bhSetup(t)
	path := "/v1/buyer/catalog"
	for _, query := range []string{
		"limit=0", "limit=-1", "limit=101", "limit=1.0", "limit=%2B1", "limit=1&limit=1", "limit=",
		"cursor=", "cursor=null", "cursor=" + strings.Repeat("a", 1025), "limit=1;cursor=x", "cursor=%GG",
		"product_id=", "product_id=bad", "product_id=" + h.stock.product.ID + "&product_id=" + h.stock.product.ID,
		"tenant_id=" + h.f.tenantA, "unknown=x", "cursor=" + strings.Repeat("a", 2049),
	} {
		t.Run(query[:min(len(query), 48)], func(t *testing.T) {
			bhError(t, h.request(t, "GET", path+"?"+query, h.cap.Token, "", nil, nil), 422, "invalid_request")
		})
	}
	bhError(t, h.request(t, "GET", path+"?", h.cap.Token, "", nil, nil), 403, "forbidden")
	bhError(t, h.request(t, "GET", "/v1/buyer/cart?limit=1", h.cap.Token, "", nil, nil), 403, "forbidden")
	bhError(t, h.request(t, "GET", path, h.cap.Token, t04Key("not-a-write"), nil, nil), 422, "invalid_request")
	bhError(t, h.request(t, "GET", path, h.cap.Token, "", struct{}{}, nil), 422, "invalid_request")
	bhError(t, h.request(t, "POST", path, h.cap.Token, "", nil, nil), 405, "method_not_allowed")
	bhError(t, h.request(t, "GET", path+"/", h.cap.Token, "", nil, nil), 404, "not_found")
	bhError(t, h.request(t, "GET", path, "", "", nil, nil), 401, "unauthorized")
	bhError(t, h.request(t, "GET", path, h.cap.Token, "", nil, func(r *http.Request) { r.Header.Del("X-Commerce-Buyer-BFF-Key") }), 401, "unauthorized")
	bhError(t, h.request(t, "GET", path, h.cap.Token, "", nil, func(r *http.Request) { r.Header.Set("X-Commerce-Storefront-Origin", "https://unknown-catalog.example") }), 404, "not_found")
	for _, header := range []string{"Cookie", "Origin", "X-Tenant-ID", "X-Store-ID"} {
		bhError(t, h.request(t, "GET", path, h.cap.Token, "", nil, func(r *http.Request) { r.Header.Set(header, "forged") }), 403, "forbidden")
	}
	mustExec(t, h.f.owner, `UPDATE control.storefront_publications SET published=false,version=version+1 WHERE tenant_id=$1 AND store_id=$2`, h.f.tenantA, h.f.storeA1)
	bhError(t, h.request(t, "GET", path, h.cap.Token, "", nil, nil), 404, "not_found")
	mustExec(t, h.f.owner, `UPDATE control.storefront_publications SET published=true,version=version+1 WHERE tenant_id=$1 AND store_id=$2`, h.f.tenantA, h.f.storeA1)
	if r := h.request(t, "DELETE", "/v1/buyer/session", h.cap.Token, "", nil, nil); r.status != 204 {
		t.Fatal("revoke test session failed")
	}
	bhError(t, h.request(t, "GET", path, h.cap.Token, "", nil, nil), 401, "unauthorized")
}
