package foundation_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/catalog"
	"livecommerce/internal/command"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/inventory"
	"livecommerce/internal/pagination"
	"livecommerce/internal/platform"
)

func collectLedgerPages(t *testing.T, tf adminTransportFixture, warehouse, query, status string) ([]catalog.LedgerRow, []int, []string) {
	t.Helper()
	ctx := context.Background()
	return collectAdminPages(t, func(page pagination.Request) (pagination.Page[catalog.LedgerRow], error) {
		return t04Scoped(ctx, tf.f, tf.token, tf.store, "catalog:read", func(tx pgx.Tx, scope platform.Scope) (pagination.Page[catalog.LedgerRow], error) {
			return catalog.ListLedger(ctx, tx, scope, catalog.LedgerRequest{Page: page, WarehouseID: warehouse, Query: query, Status: status})
		})
	}, func(row catalog.LedgerRow) string { return row.SKUID })
}

func TestCatalogLedgerProjectionPaginationAndBalanceTruth(t *testing.T) {
	tf := newAdminTransportFixture(t)
	ctx := context.Background()
	archived, err := t04Scoped(ctx, tf.f, tf.token, tf.store, "catalog:write", func(tx pgx.Tx, scope platform.Scope) (catalog.SKU, error) {
		return catalog.ArchiveSKU(ctx, tx, scope, t04Key("ledger-archive"), tf.skuIDs[0], 1)
	})
	if err != nil {
		t.Fatalf("archive fixture SKU: %v", err)
	}

	rows, sizes, cursors := collectLedgerPages(t, tf, tf.warehouseIDs[0], "", "all")
	if len(rows) != 105 || len(cursors) != 6 || sizes[len(sizes)-1] != 3 {
		t.Fatalf("ledger traversal rows=%d cursors=%d sizes=%v, want 105/6/final3", len(rows), len(cursors), sizes)
	}

	balances, _, _ := collectAdminPages(t, func(page pagination.Request) (pagination.Page[inventory.Balance], error) {
		return t04Scoped(ctx, tf.f, tf.token, tf.store, "inventory:read", func(tx pgx.Tx, scope platform.Scope) (pagination.Page[inventory.Balance], error) {
			return inventory.ListBalancesPage(ctx, tx, scope, page)
		})
	}, func(balance inventory.Balance) string { return balance.WarehouseID + "/" + balance.SKUID })
	want := make(map[string]inventory.Balance)
	for _, balance := range balances {
		if balance.WarehouseID == tf.warehouseIDs[0] {
			want[balance.SKUID] = balance
		}
	}
	zero, archivedRows := 0, 0
	for _, row := range rows {
		if row.WarehouseID != tf.warehouseIDs[0] {
			t.Fatalf("row %s warehouse=%s, want %s", row.SKUID, row.WarehouseID, tf.warehouseIDs[0])
		}
		balance, ok := want[row.SKUID]
		if !ok {
			zero++
			if row.OnHand != 0 || row.Reserved != 0 || row.Allocated != 0 || row.Unavailable != 0 || row.Available != 0 || row.BalanceVersion != 0 {
				t.Fatalf("missing balance was not zero-filled: %+v", row)
			}
		} else if row.OnHand != balance.OnHand || row.Reserved != balance.Reserved || row.Allocated != balance.Allocated || row.Unavailable != balance.Unavailable || row.Available != balance.Available || row.BalanceVersion != balance.Version {
			t.Fatalf("ledger row does not equal scoped balance: row=%+v balance=%+v", row, balance)
		}
		if row.Status == "archived" {
			archivedRows++
			if row.SKUID != archived.ID {
				t.Fatalf("unexpected archived row: %+v", row)
			}
		}
	}
	if zero != 52 || archivedRows != 1 {
		t.Fatalf("zero balances=%d archived=%d, want 52/1", zero, archivedRows)
	}

	active, _, _ := collectLedgerPages(t, tf, tf.warehouseIDs[0], "", "active")
	archivedOnly, archivedSizes, archivedCursors := collectLedgerPages(t, tf, tf.warehouseIDs[0], "", "archived")
	if len(active) != 104 || len(archivedOnly) != 1 || archivedOnly[0].SKUID != archived.ID || len(archivedCursors) != 0 || len(archivedSizes) != 1 || archivedSizes[0] != 1 {
		t.Fatalf("status filters active=%d archived=%+v sizes=%v cursors=%d", len(active), archivedOnly, archivedSizes, len(archivedCursors))
	}
}

func createLedgerSearchSKU(t *testing.T, tf adminTransportFixture, name, code string) catalog.SKU {
	t.Helper()
	ctx := context.Background()
	product, err := t04Scoped(ctx, tf.f, tf.token, tf.store, "catalog:write", func(tx pgx.Tx, scope platform.Scope) (catalog.Product, error) {
		return catalog.CreateProduct(ctx, tx, scope, t04Key("ledger-product"), catalog.ProductInput{Name: name, Description: "literal search fixture"})
	})
	if err != nil {
		t.Fatalf("create search product: %v", err)
	}
	sku, err := t04Scoped(ctx, tf.f, tf.token, tf.store, "catalog:write", func(tx pgx.Tx, scope platform.Scope) (catalog.SKU, error) {
		return catalog.CreateSKU(ctx, tx, scope, t04Key("ledger-sku"), catalog.SKUInput{
			ProductID: product.ID, Code: code, PriceMinor: 321,
			WeightGrams: 1, LengthMM: 1, WidthMM: 1, HeightMM: 1,
			OriginCountry: "US", CustomsName: "search fixture", HSCandidate: "851840",
		})
	})
	if err != nil {
		t.Fatalf("create search SKU: %v", err)
	}
	return sku
}

func TestCatalogLedgerSearchTreatsPercentAndUnderscoreLiterally(t *testing.T) {
	tf := newAdminTransportFixture(t)
	tag := t04Tag()
	percent := createLedgerSearchSKU(t, tf, "literal-percent-%-"+tag, "PCT-"+tag)
	underscore := createLedgerSearchSKU(t, tf, "literal underscore "+tag, "UNDER_"+tag)

	for _, tc := range []struct {
		query string
		want  string
	}{
		{"%", percent.ID},
		{"_", underscore.ID},
	} {
		t.Run(url.QueryEscape(tc.query), func(t *testing.T) {
			rows, _, _ := collectLedgerPages(t, tf, tf.warehouseIDs[0], tc.query, "all")
			if len(rows) != 1 || rows[0].SKUID != tc.want {
				t.Fatalf("literal query %q rows=%+v, want only SKU %s", tc.query, rows, tc.want)
			}
			if rows[0].BalanceVersion != 0 || rows[0].Available != 0 {
				t.Fatalf("search-only SKU missing balance was not zero-filled: %+v", rows[0])
			}
		})
	}
}

func TestCatalogLedgerCursorBindsWarehouseAndFilters(t *testing.T) {
	tf := newAdminTransportFixture(t)
	ctx := context.Background()
	first, err := t04Scoped(ctx, tf.f, tf.token, tf.store, "catalog:read", func(tx pgx.Tx, scope platform.Scope) (pagination.Page[catalog.LedgerRow], error) {
		return catalog.ListLedger(ctx, tx, scope, catalog.LedgerRequest{Page: pagination.Request{Limit: 2}, WarehouseID: tf.warehouseIDs[0], Status: "all"})
	})
	if err != nil || len(first.Items) != 2 || first.NextCursor == "" {
		t.Fatalf("first ledger page=%+v err=%v", first, err)
	}

	for _, tc := range []struct {
		name, warehouse, query, status string
	}{
		{"warehouse", tf.warehouseIDs[1], "", "all"},
		{"query", tf.warehouseIDs[0], "PAGE", "all"},
		{"status", tf.warehouseIDs[0], "", "active"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := t04Scoped(ctx, tf.f, tf.token, tf.store, "catalog:read", func(tx pgx.Tx, scope platform.Scope) (pagination.Page[catalog.LedgerRow], error) {
				return catalog.ListLedger(ctx, tx, scope, catalog.LedgerRequest{
					Page: pagination.Request{Limit: 2, Cursor: first.NextCursor}, WarehouseID: tc.warehouse, Query: tc.query, Status: tc.status,
				})
			})
			if !errors.Is(err, command.ErrInvalid) {
				t.Fatalf("cursor binding error=%v, want ErrInvalid", err)
			}
		})
	}
}

func ledgerPermissionToken(t *testing.T, tf adminTransportFixture, permissions ...string) string {
	t.Helper()
	ctx := context.Background()
	principal, token := randomUUID(), randomToken()
	tx, err := tf.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, `INSERT INTO identity.principals(id) VALUES($1)`, principal); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO identity.memberships(tenant_id,principal_id) VALUES($1,$2)`, tf.tenant, principal); err != nil {
		t.Fatal(err)
	}
	permissions = append([]string{"store:read"}, permissions...)
	for _, permission := range permissions {
		if _, err = tx.Exec(ctx, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,$4)`, tf.tenant, tf.store, principal, permission); err != nil {
			t.Fatal(err)
		}
	}
	if err = insertSession(ctx, tx, token, principal, "merchant", time.Now().UTC().Add(time.Hour), nil); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return token
}

func decodeLedgerHTTPPage(t *testing.T, responseBody []byte) pagination.Page[catalog.LedgerRow] {
	t.Helper()
	var page pagination.Page[catalog.LedgerRow]
	if err := json.Unmarshal(responseBody, &page); err != nil {
		t.Fatalf("decode ledger response: %v body=%s", err, responseBody)
	}
	if page.Items == nil {
		t.Fatal("ledger items is null, want []")
	}
	return page
}

func TestCatalogLedgerHTTPPermissionIsolationAndQueryMatrix(t *testing.T) {
	tf := newAdminTransportFixture(t)
	foreign := t04CreateStock(t, tf.f, tf.f.tokens["a"], tf.f.storeA1, 1)
	catalogOnly := ledgerPermissionToken(t, tf, "catalog:read")
	inventoryOnly := ledgerPermissionToken(t, tf, "inventory:read")
	handler := httpapi.NewHandler(tf.f.runtime)
	base := "/v1/admin/stores/" + tf.store + "/catalog-ledger"
	query := func(warehouse string, values url.Values) string {
		values.Set("warehouse_id", warehouse)
		return base + "?" + values.Encode()
	}

	for _, tc := range []struct {
		name, token string
	}{
		{"missing inventory read", catalogOnly},
		{"missing catalog read", inventoryOnly},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := adminRequest(handler, http.MethodGet, query(tf.warehouseIDs[0], url.Values{}), tc.token, nil, "", nil)
			assertAdminError(t, response, http.StatusForbidden, "forbidden", tc.token, tf.store, tf.tenant)
		})
	}

	for _, tc := range []struct {
		name, path string
	}{
		{"cross store", "/v1/admin/stores/" + tf.f.storeA2 + "/catalog-ledger?warehouse_id=" + tf.warehouseIDs[0]},
		{"foreign warehouse", query(foreign.warehouse.ID, url.Values{})},
		{"missing warehouse", query(randomUUID(), url.Values{})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := adminRequest(handler, http.MethodGet, tc.path, tf.token, nil, "", nil)
			assertAdminError(t, response, http.StatusNotFound, "not_found", tf.token, tf.store, tf.tenant, foreign.warehouse.ID)
		})
	}

	invalid := []string{
		base,
		query("not-a-uuid", url.Values{}),
		query(tf.warehouseIDs[0], url.Values{"offset": {"1"}}),
		query(tf.warehouseIDs[0], url.Values{"status": {"deleted"}}),
		query(tf.warehouseIDs[0], url.Values{"q": {"bad\x01query"}}),
		query(tf.warehouseIDs[0], url.Values{"q": {strings.Repeat("界", 121)}}),
		query(tf.warehouseIDs[0], url.Values{"limit": {"0"}}),
		query(tf.warehouseIDs[0], url.Values{"cursor": {"' OR 1=1--"}}),
	}
	invalid = append(invalid, base+"?warehouse_id="+tf.warehouseIDs[0]+"&warehouse_id="+tf.warehouseIDs[1])
	for i, path := range invalid {
		t.Run(fmt.Sprintf("invalid query %d", i), func(t *testing.T) {
			response := adminRequest(handler, http.MethodGet, path, tf.token, nil, "", nil)
			assertAdminError(t, response, http.StatusUnprocessableEntity, "invalid_request", tf.token, tf.store, tf.tenant)
		})
	}

	firstResponse := adminRequest(handler, http.MethodGet, query(tf.warehouseIDs[0], url.Values{"limit": {"100"}}), tf.token, nil, "", nil)
	if firstResponse.Code != http.StatusOK {
		t.Fatalf("first endpoint page status=%d body=%s", firstResponse.Code, firstResponse.Body.String())
	}
	first := decodeLedgerHTTPPage(t, firstResponse.Body.Bytes())
	if len(first.Items) != 100 || first.NextCursor == "" {
		t.Fatalf("first endpoint page rows=%d cursor=%q", len(first.Items), first.NextCursor)
	}
	secondResponse := adminRequest(handler, http.MethodGet, query(tf.warehouseIDs[0], url.Values{"limit": {"100"}, "cursor": {first.NextCursor}}), tf.token, nil, "", nil)
	if secondResponse.Code != http.StatusOK {
		t.Fatalf("second endpoint page status=%d body=%s", secondResponse.Code, secondResponse.Body.String())
	}
	second := decodeLedgerHTTPPage(t, secondResponse.Body.Bytes())
	if len(second.Items) != 5 || second.NextCursor != "" || first.Items[99].SKUID >= second.Items[0].SKUID {
		t.Fatalf("second endpoint page=%+v after=%s", second, first.Items[99].SKUID)
	}
}
