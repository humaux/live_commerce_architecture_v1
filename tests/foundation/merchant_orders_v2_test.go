package foundation_test

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http/httptest"
	"strings"
	"testing"

	"livecommerce/internal/httpapi"
	"livecommerce/internal/storefront"
)

// Direct SQL v2 variant for the unchanged observed-lock revocation adversary.
func moReadV2(ctx context.Context, pool *pgxpool.Pool, token, tenant, store, principal, _, state string, limit int) ([]map[string]any, error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, `SELECT set_config('app.tenant_id',$1,true),set_config('app.store_id',$2,true),set_config('app.principal_id',$3,true)`, tenant, store, principal); err != nil {
		return nil, err
	}
	var raw []byte
	err = tx.QueryRow(ctx, `SELECT identity.read_merchant_orders_v2($1,$2,$3,NULL,NULL,$4,'all','','','',NULL,NULL,NULL)`, tokenHash(token), store, limit, state).Scan(&raw)
	if err != nil {
		return nil, err
	}
	var value struct {
		Items []map[string]any `json:"items"`
	}
	err = json.Unmarshal(raw, &value)
	return value.Items, err
}

// The first red/green boundary: the requested read projection must exist and
// return server counts while hiding a separate real checkout draft.
func TestMerchantOrdersV2ReadContract(t *testing.T) {
	q := pqSetup(t)
	moGrant(t, q.f, q.f.tenantA, q.f.storeA1, q.f.principalA)
	buyer := mustIssue(t, q.cqHarness.service, q.f.storeA1)
	other := q.bcHarness
	other.prepare(t, buyer, []storefront.Item{{SKUID: q.stock.skus[0].ID, Quantity: 1}})
	if _, err := other.begin(t04Key("mo-v2-draft")); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("GET", "/v1/admin/stores/"+q.f.storeA1+"/orders?view=v2", nil)
	req.Header.Set("Authorization", "Bearer "+q.f.tokens["a"])
	w := httptest.NewRecorder()
	httpapi.NewHandler(q.f.runtime).ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("v2 read status=%d body=%s", w.Code, w.Body.String())
	}
	var out struct {
		Items  []json.RawMessage `json:"items"`
		Total  int               `json:"total"`
		Counts map[string]int    `json:"counts"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Items) != 1 || out.Total != 1 || len(out.Counts) != 8 || out.Counts["all"] != 1 || out.Counts["unpaid"] != 1 {
		t.Fatalf("v2 default-draft projection: %s", w.Body.String())
	}
}

func TestMerchantOrdersV2SearchStreamLimit(t *testing.T) {
	// HTTP/2 can stream without a declared length. Reject before any DB access.
	r := httptest.NewRequest("POST", "/v1/admin/stores/00000000-0000-0000-0000-000000000001/orders/search?view=v2", strings.NewReader(`{"q":"test"}`+strings.Repeat(" ", 1024)))
	r.ContentLength = -1
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	httpapi.NewHandler(nil).ServeHTTP(w, r)
	if w.Code != 400 || !strings.Contains(w.Body.String(), `"code":"invalid_json"`) {
		t.Fatalf("unknown-length oversized search: status=%d", w.Code)
	}
}
