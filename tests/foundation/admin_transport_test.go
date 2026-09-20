package foundation_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
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

type adminTransportFixture struct {
	f            *testFixture
	tenant       string
	store        string
	principal    string
	token        string
	parent       string
	otherParent  string
	productIDs   []string
	skuIDs       []string
	warehouseIDs []string
}

func newAdminTransportFixture(t *testing.T) adminTransportFixture {
	t.Helper()
	f := t04Fixture(t)
	ctx := context.Background()
	tf := adminTransportFixture{
		f: f, tenant: f.tenantA, store: randomUUID(), principal: randomUUID(), token: randomToken(),
		parent: randomUUID(), otherParent: randomUUID(),
	}
	tx, err := f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, `INSERT INTO control.stores(tenant_id,id,name,currency) VALUES($1,$2,$3,'USD')`, tf.tenant, tf.store, "transport-"+t04Tag()); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO identity.principals(id) VALUES($1)`, tf.principal); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO identity.memberships(tenant_id,principal_id) VALUES($1,$2)`, tf.tenant, tf.principal); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission)
		SELECT $1,$2,$3,p FROM unnest(ARRAY['store:read','audit:read','audit:write','catalog:read','catalog:write','inventory:read','inventory:write','inventory:reserve']) p`, tf.tenant, tf.store, tf.principal); err != nil {
		t.Fatal(err)
	}
	if err = insertSession(ctx, tx, tf.token, tf.principal, "merchant", time.Now().UTC().Add(time.Hour), nil); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	for _, id := range []string{tf.parent, tf.otherParent} {
		tf.productIDs = append(tf.productIDs, id)
	}
	for range 103 {
		tf.productIDs = append(tf.productIDs, randomUUID())
	}
	for range 105 {
		tf.skuIDs = append(tf.skuIDs, randomUUID())
		tf.warehouseIDs = append(tf.warehouseIDs, randomUUID())
	}
	err = platform.WithScope(ctx, f.runtime, tf.token, tf.store, "catalog:write", func(tx pgx.Tx, scope platform.Scope) error {
		for i, id := range tf.productIDs {
			if _, err := tx.Exec(ctx, `INSERT INTO catalog.products(tenant_id,store_id,id,name,description)
				VALUES($1,$2,$3,$4,'transport pagination fixture')`, scope.TenantID, scope.StoreID, id, fmt.Sprintf("transport-product-%03d-%s", i, t04Tag())); err != nil {
				return err
			}
		}
		for i, id := range tf.skuIDs {
			if _, err := tx.Exec(ctx, `INSERT INTO catalog.skus(tenant_id,store_id,id,product_id,code,currency,price_minor)
				VALUES($1,$2,$3,$4,$5,'USD',$6)`, scope.TenantID, scope.StoreID, id, tf.parent, fmt.Sprintf("PAGE-%03d-%s", i, t04Tag()), int64(i)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seed paged catalog through scope: %v", err)
	}
	err = platform.WithScope(ctx, f.runtime, tf.token, tf.store, "inventory:write", func(tx pgx.Tx, scope platform.Scope) error {
		for i, id := range tf.warehouseIDs {
			if _, err := tx.Exec(ctx, `INSERT INTO inventory.warehouses(tenant_id,store_id,id,name) VALUES($1,$2,$3,$4)`, scope.TenantID, scope.StoreID, id, fmt.Sprintf("transport-warehouse-%03d-%s", i, t04Tag())); err != nil {
				return err
			}
		}
		for i, skuID := range tf.skuIDs {
			warehouseID := tf.warehouseIDs[0]
			if i >= 53 {
				warehouseID = tf.warehouseIDs[1]
			}
			if _, err := tx.Exec(ctx, `INSERT INTO inventory.ledger(
				tenant_id,store_id,warehouse_id,sku_id,kind,delta_on_hand,operation,command_key,reason,principal_id)
				VALUES($1,$2,$3,$4,'ADJUST',1,'transport.seed',$5,'pagination fixture',$6)`,
				scope.TenantID, scope.StoreID, warehouseID, skuID, fmt.Sprintf("transport:%03d:%s", i, t04Tag()), scope.PrincipalID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seed paged inventory through scope: %v", err)
	}
	return tf
}

func collectAdminPages[T any](t *testing.T, fetch func(pagination.Request) (pagination.Page[T], error), key func(T) string) ([]T, []int, []string) {
	t.Helper()
	var all []T
	var sizes []int
	var cursors []string
	cursor := ""
	previous := ""
	seen := make(map[string]bool)
	for {
		page, err := fetch(pagination.Request{Limit: 17, Cursor: cursor})
		if err != nil {
			t.Fatalf("fetch page after %q: %v", cursor, err)
		}
		if page.Items == nil {
			t.Fatal("items is null, want []")
		}
		sizes = append(sizes, len(page.Items))
		for _, item := range page.Items {
			current := key(item)
			if current <= previous || seen[current] {
				t.Fatalf("non-increasing or duplicate key %q after %q", current, previous)
			}
			previous = current
			seen[current] = true
			all = append(all, item)
		}
		if page.NextCursor == "" {
			break
		}
		if page.NextCursor == cursor || len(page.NextCursor) > 1024 {
			t.Fatalf("invalid next cursor length=%d", len(page.NextCursor))
		}
		cursor = page.NextCursor
		cursors = append(cursors, cursor)
	}
	return all, sizes, cursors
}

func TestAdminTransportFourListsTraverseMoreThan100Rows(t *testing.T) {
	tf := newAdminTransportFixture(t)
	ctx := context.Background()

	products, productSizes, productCursors := collectAdminPages(t, func(request pagination.Request) (pagination.Page[catalog.Product], error) {
		return t04Scoped(ctx, tf.f, tf.token, tf.store, "catalog:read", func(tx pgx.Tx, scope platform.Scope) (pagination.Page[catalog.Product], error) {
			return catalog.ListProductsPage(ctx, tx, scope, request)
		})
	}, func(p catalog.Product) string { return p.ID })
	if len(products) != 105 || len(productCursors) != 6 || productSizes[len(productSizes)-1] != 3 {
		t.Fatalf("product traversal rows=%d cursors=%d sizes=%v, want 105/6/final3", len(products), len(productCursors), productSizes)
	}

	skus, skuSizes, skuCursors := collectAdminPages(t, func(request pagination.Request) (pagination.Page[catalog.SKU], error) {
		return t04Scoped(ctx, tf.f, tf.token, tf.store, "catalog:read", func(tx pgx.Tx, scope platform.Scope) (pagination.Page[catalog.SKU], error) {
			return catalog.ListSKUsPage(ctx, tx, scope, tf.parent, request)
		})
	}, func(s catalog.SKU) string { return s.ID })
	if len(skus) != 105 || len(skuCursors) != 6 || skuSizes[len(skuSizes)-1] != 3 {
		t.Fatalf("SKU traversal rows=%d cursors=%d sizes=%v, want 105/6/final3", len(skus), len(skuCursors), skuSizes)
	}

	warehouses, warehouseSizes, _ := collectAdminPages(t, func(request pagination.Request) (pagination.Page[inventory.Warehouse], error) {
		return t04Scoped(ctx, tf.f, tf.token, tf.store, "inventory:read", func(tx pgx.Tx, scope platform.Scope) (pagination.Page[inventory.Warehouse], error) {
			return inventory.ListWarehousesPage(ctx, tx, scope, request)
		})
	}, func(w inventory.Warehouse) string { return w.ID })
	if len(warehouses) != 105 || warehouseSizes[len(warehouseSizes)-1] != 3 {
		t.Fatalf("warehouse traversal rows=%d sizes=%v, want 105/final3", len(warehouses), warehouseSizes)
	}

	balances, balanceSizes, _ := collectAdminPages(t, func(request pagination.Request) (pagination.Page[inventory.Balance], error) {
		return t04Scoped(ctx, tf.f, tf.token, tf.store, "inventory:read", func(tx pgx.Tx, scope platform.Scope) (pagination.Page[inventory.Balance], error) {
			return inventory.ListBalancesPage(ctx, tx, scope, request)
		})
	}, func(b inventory.Balance) string { return b.WarehouseID + "/" + b.SKUID })
	if len(balances) != 105 || balanceSizes[len(balanceSizes)-1] != 3 {
		t.Fatalf("balance traversal rows=%d sizes=%v, want 105/final3", len(balances), balanceSizes)
	}
	if balances[0].WarehouseID == balances[len(balances)-1].WarehouseID {
		t.Fatal("balance fixture did not exercise the first key boundary")
	}

	empty, err := t04Scoped(ctx, tf.f, tf.token, tf.store, "catalog:read", func(tx pgx.Tx, scope platform.Scope) (pagination.Page[catalog.SKU], error) {
		return catalog.ListSKUsPage(ctx, tx, scope, tf.otherParent, pagination.Request{})
	})
	if err != nil || empty.Items == nil || len(empty.Items) != 0 || empty.NextCursor != "" {
		t.Fatalf("empty SKU page=%+v err=%v", empty, err)
	}

	first, err := t04Scoped(ctx, tf.f, tf.token, tf.store, "catalog:read", func(tx pgx.Tx, scope platform.Scope) (pagination.Page[catalog.Product], error) {
		return catalog.ListProductsPage(ctx, tx, scope, pagination.Request{Limit: 10})
	})
	if err != nil || len(first.Items) != 10 || first.NextCursor == "" {
		t.Fatalf("first insertion page=%+v err=%v", first, err)
	}
	const lowID = "00000000-0000-4000-8000-000000000001"
	err = platform.WithScope(ctx, tf.f.runtime, tf.token, tf.store, "catalog:write", func(tx pgx.Tx, scope platform.Scope) error {
		_, err := tx.Exec(ctx, `INSERT INTO catalog.products(tenant_id,store_id,id,name) VALUES($1,$2,$3,'inserted-before-cursor')`, scope.TenantID, scope.StoreID, lowID)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := t04Scoped(ctx, tf.f, tf.token, tf.store, "catalog:read", func(tx pgx.Tx, scope platform.Scope) (pagination.Page[catalog.Product], error) {
		return catalog.ListProductsPage(ctx, tx, scope, pagination.Request{Limit: 10, Cursor: first.NextCursor})
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, product := range second.Items {
		if product.ID == lowID {
			t.Fatal("insert before cursor leaked into later page")
		}
	}
	refresh, err := t04Scoped(ctx, tf.f, tf.token, tf.store, "catalog:read", func(tx pgx.Tx, scope platform.Scope) (pagination.Page[catalog.Product], error) {
		return catalog.ListProductsPage(ctx, tx, scope, pagination.Request{Limit: 1})
	})
	if err != nil || len(refresh.Items) != 1 || refresh.Items[0].ID != lowID {
		t.Fatalf("refresh did not see earlier insert: %+v err=%v", refresh, err)
	}
}

func TestAdminTransportCursorBindingsFailClosed(t *testing.T) {
	tf := newAdminTransportFixture(t)
	ctx := context.Background()
	productPage, err := t04Scoped(ctx, tf.f, tf.token, tf.store, "catalog:read", func(tx pgx.Tx, scope platform.Scope) (pagination.Page[catalog.Product], error) {
		return catalog.ListProductsPage(ctx, tx, scope, pagination.Request{Limit: 1})
	})
	if err != nil || productPage.NextCursor == "" {
		t.Fatalf("product cursor: %+v %v", productPage, err)
	}
	skuPage, err := t04Scoped(ctx, tf.f, tf.token, tf.store, "catalog:read", func(tx pgx.Tx, scope platform.Scope) (pagination.Page[catalog.SKU], error) {
		return catalog.ListSKUsPage(ctx, tx, scope, tf.parent, pagination.Request{Limit: 1})
	})
	if err != nil || skuPage.NextCursor == "" {
		t.Fatalf("SKU cursor: %+v %v", skuPage, err)
	}

	cases := []struct {
		name string
		run  func() error
	}{
		{"store mismatch", func() error {
			_, err := t04Scoped(ctx, tf.f, tf.f.tokens["a2"], tf.f.storeA2, "catalog:read", func(tx pgx.Tx, scope platform.Scope) (pagination.Page[catalog.Product], error) {
				return catalog.ListProductsPage(ctx, tx, scope, pagination.Request{Limit: 1, Cursor: productPage.NextCursor})
			})
			return err
		}},
		{"collection mismatch", func() error {
			_, err := t04Scoped(ctx, tf.f, tf.token, tf.store, "inventory:read", func(tx pgx.Tx, scope platform.Scope) (pagination.Page[inventory.Warehouse], error) {
				return inventory.ListWarehousesPage(ctx, tx, scope, pagination.Request{Limit: 1, Cursor: productPage.NextCursor})
			})
			return err
		}},
		{"parent mismatch", func() error {
			_, err := t04Scoped(ctx, tf.f, tf.token, tf.store, "catalog:read", func(tx pgx.Tx, scope platform.Scope) (pagination.Page[catalog.SKU], error) {
				return catalog.ListSKUsPage(ctx, tx, scope, tf.otherParent, pagination.Request{Limit: 1, Cursor: skuPage.NextCursor})
			})
			return err
		}},
		{"malformed cursor", func() error {
			_, err := t04Scoped(ctx, tf.f, tf.token, tf.store, "catalog:read", func(tx pgx.Tx, scope platform.Scope) (pagination.Page[catalog.Product], error) {
				return catalog.ListProductsPage(ctx, tx, scope, pagination.Request{Cursor: `' OR 1=1--`})
			})
			return err
		}},
		{"oversized cursor", func() error {
			_, err := t04Scoped(ctx, tf.f, tf.token, tf.store, "catalog:read", func(tx pgx.Tx, scope platform.Scope) (pagination.Page[catalog.Product], error) {
				return catalog.ListProductsPage(ctx, tx, scope, pagination.Request{Cursor: strings.Repeat("A", 1025)})
			})
			return err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.run(); !errors.Is(err, command.ErrInvalid) {
				t.Fatalf("error=%v, want ErrInvalid", err)
			}
		})
	}
}

type adminErrorEnvelope struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	RequestID string         `json:"request_id"`
	Retryable bool           `json:"retryable"`
	Details   map[string]any `json:"details"`
}

func adminRequest(handler http.Handler, method, path, token string, body []byte, contentType string, headers map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, bytes.NewReader(body))
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	if contentType != "" {
		r.Header.Set("Content-Type", contentType)
	}
	for name, value := range headers {
		r.Header.Set(name, value)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}

func assertAdminError(t *testing.T, response *httptest.ResponseRecorder, status int, code string, forbidden ...string) adminErrorEnvelope {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status=%d body=%s, want %d", response.Code, response.Body.String(), status)
	}
	if response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("unsafe error headers: %v", response.Header())
	}
	var envelope adminErrorEnvelope
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode error envelope: %v body=%q", err, response.Body.String())
	}
	if envelope.Code != code || envelope.Message == "" || envelope.RequestID == "" || envelope.Details == nil {
		t.Fatalf("incomplete envelope: %+v", envelope)
	}
	if response.Header().Get("X-Request-ID") != envelope.RequestID {
		t.Fatalf("request id header/body mismatch: %q / %q", response.Header().Get("X-Request-ID"), envelope.RequestID)
	}
	text := strings.ToLower(response.Body.String())
	for _, value := range forbidden {
		if value != "" && strings.Contains(text, strings.ToLower(value)) {
			t.Fatalf("error leaked %q: %s", value, response.Body.String())
		}
	}
	return envelope
}

type adminAuthFixture struct {
	readOnlyToken, noVisibilityToken, revokedMembershipToken, inactiveStore string
}

func seedAdminAuthFixture(t *testing.T, f *testFixture) adminAuthFixture {
	t.Helper()
	ctx := context.Background()
	tx, err := f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	out := adminAuthFixture{readOnlyToken: randomToken(), noVisibilityToken: randomToken(), revokedMembershipToken: randomToken(), inactiveStore: randomUUID()}
	readOnly, invisible, revoked := randomUUID(), randomUUID(), randomUUID()
	if _, err = tx.Exec(ctx, `INSERT INTO control.stores(tenant_id,id,name,currency,active) VALUES($1,$2,$3,'USD',false)`, f.tenantA, out.inactiveStore, "inactive-"+t04Tag()); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO identity.principals(id) VALUES($1),($2),($3)`, readOnly, invisible, revoked); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO identity.memberships(tenant_id,principal_id,active) VALUES($1,$2,true),($1,$3,true),($1,$4,false)`, f.tenantA, readOnly, invisible, revoked); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES
		($1,$2,$3,'store:read'),($1,$4,$3,'store:read'),($1,$2,$5,'store:read')`, f.tenantA, f.storeA1, readOnly, out.inactiveStore, revoked); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Add(time.Hour)
	if err = insertSession(ctx, tx, out.readOnlyToken, readOnly, "merchant", now, nil); err != nil {
		t.Fatal(err)
	}
	if err = insertSession(ctx, tx, out.noVisibilityToken, invisible, "merchant", now, nil); err != nil {
		t.Fatal(err)
	}
	if err = insertSession(ctx, tx, out.revokedMembershipToken, revoked, "merchant", now, nil); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestAdminTransportHTTPAuthAndErrorMatrix(t *testing.T) {
	f := t04Fixture(t)
	tf := newAdminTransportFixture(t)
	auth := seedAdminAuthFixture(t, f)
	handler := httpapi.NewHandler(f.runtime)
	deniedName := "denied-" + t04Tag()
	body := []byte(`{"name":"` + deniedName + `"}`)
	base := "/v1/admin/stores/"
	cases := []struct {
		name, token, store, code string
		status                   int
	}{
		{"invalid token", "not-a-valid-session-token-xxxxxxxx", f.storeA1, "unauthorized", 401},
		{"expired session", f.tokens["expired"], f.storeA1, "unauthorized", 401},
		{"revoked session", f.tokens["revoked"], f.storeA1, "unauthorized", 401},
		{"wrong audience", f.tokens["buyer"], f.storeA1, "unauthorized", 401},
		{"nonexistent store", f.tokens["a"], randomUUID(), "not_found", 404},
		{"foreign tenant store", f.tokens["a"], f.storeB, "not_found", 404},
		{"no store read grant", auth.noVisibilityToken, f.storeA1, "not_found", 404},
		{"revoked membership", auth.revokedMembershipToken, f.storeA1, "not_found", 404},
		{"inactive store", auth.readOnlyToken, auth.inactiveStore, "not_found", 404},
		{"missing operation grant", auth.readOnlyToken, f.storeA1, "forbidden", 403},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			response := adminRequest(handler, http.MethodPost, base+tc.store+"/products", tc.token, body, "application/json", map[string]string{"Idempotency-Key": t04Key("denied-http")})
			envelope := assertAdminError(t, response, tc.status, tc.code, tc.token, tc.store, f.tenantA, "postgres://", "sqlstate")
			if envelope.Retryable {
				t.Fatalf("auth denial marked retryable: %+v", envelope)
			}
		})
	}
	if got := t04Count(t, f, `SELECT count(*) FROM catalog.products WHERE name=$1`, deniedName); got != 0 {
		t.Fatalf("denied callback wrote %d products", got)
	}

	queryCases := []string{
		"?limit=1&limit=2",
		"?offset=1",
		"?limit=" + url.QueryEscape("1 OR 1=1"),
		"?cursor=" + url.QueryEscape(`' OR 1=1--`),
		"?cursor=" + strings.Repeat("A", 1025),
	}
	for _, suffix := range queryCases {
		response := adminRequest(handler, http.MethodGet, base+tf.store+"/products"+suffix, tf.token, nil, "", nil)
		assertAdminError(t, response, 422, "invalid_request", tf.token, tf.store, tf.tenant, "sqlstate")
	}

	invalidJSON := adminRequest(handler, http.MethodPost, base+tf.store+"/products", tf.token, []byte(`{"name":`), "application/json", map[string]string{"Idempotency-Key": t04Key("invalid-json")})
	assertAdminError(t, invalidJSON, 400, "invalid_json", tf.token, tf.store)
	wrongMedia := adminRequest(handler, http.MethodPost, base+tf.store+"/products", tf.token, []byte(`{"name":"x"}`), "text/plain", map[string]string{"Idempotency-Key": t04Key("wrong-media")})
	assertAdminError(t, wrongMedia, 415, "json_required", tf.token, tf.store)

	key := t04Key("http-conflict")
	created := adminRequest(handler, http.MethodPost, base+tf.store+"/products", tf.token, []byte(`{"name":"first"}`), "application/json", map[string]string{"Idempotency-Key": key})
	if created.Code != http.StatusOK {
		t.Fatalf("conflict setup status=%d body=%s", created.Code, created.Body.String())
	}
	conflict := adminRequest(handler, http.MethodPost, base+tf.store+"/products", tf.token, []byte(`{"name":"changed"}`), "application/json", map[string]string{"Idempotency-Key": key})
	assertAdminError(t, conflict, 409, "conflict", tf.token, tf.store)

	unknownOne := adminRequest(handler, http.MethodGet, "/missing-route", "", nil, "", map[string]string{"X-Request-ID": "forged-request-id"})
	firstEnvelope := assertAdminError(t, unknownOne, 404, "not_found", "forged-request-id")
	unknownTwo := adminRequest(handler, http.MethodGet, "/another-missing-route", "", nil, "", map[string]string{"X-Request-ID": "forged-request-id"})
	secondEnvelope := assertAdminError(t, unknownTwo, 404, "not_found", "forged-request-id")
	if firstEnvelope.RequestID == secondEnvelope.RequestID {
		t.Fatal("server reused request ID")
	}
	wrongMethod := adminRequest(handler, http.MethodDelete, base+tf.store+"/products", tf.token, nil, "", nil)
	assertAdminError(t, wrongMethod, 405, "method_not_allowed", tf.token, tf.store)
	if wrongMethod.Header().Get("Allow") == "" {
		t.Fatal("405 omitted Allow header")
	}

	canceledRequest := httptest.NewRequest(http.MethodGet, base+tf.store+"/products", nil)
	canceledRequest.Header.Set("Authorization", "Bearer "+tf.token)
	canceledContext, cancel := context.WithCancel(canceledRequest.Context())
	cancel()
	canceledRequest = canceledRequest.WithContext(canceledContext)
	canceledResponse := httptest.NewRecorder()
	handler.ServeHTTP(canceledResponse, canceledRequest)
	retry := assertAdminError(t, canceledResponse, 503, "retry_later", tf.token, tf.store)
	if !retry.Retryable {
		t.Fatal("503 is not retryable")
	}

	list := adminRequest(handler, http.MethodGet, base+tf.store+"/products?limit=2", tf.token, nil, "", nil)
	if list.Code != http.StatusOK {
		t.Fatalf("paged HTTP list status=%d body=%s", list.Code, list.Body.String())
	}
	var page pagination.Page[catalog.Product]
	if err := json.Unmarshal(list.Body.Bytes(), &page); err != nil || len(page.Items) != 2 || page.NextCursor == "" {
		t.Fatalf("paged HTTP response=%+v err=%v", page, err)
	}
}
