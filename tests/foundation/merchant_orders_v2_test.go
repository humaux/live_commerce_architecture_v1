package foundation_test

// Purpose: verify scoped orders-v2 reads, private search bounds and safe frozen item summaries.
// Depends on: real isolated PG fixtures, merchant order HTTP handlers, catalog and inventory commands.
// Used by: Foundation checks and Gates foundation shards.
// Invariants: synthetic merchandise only; no PII in lists; reads never change business facts.

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http/httptest"
	"strings"
	"testing"

	"livecommerce/internal/catalog"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/inventory"
	"livecommerce/internal/platform"
	"livecommerce/internal/storefront"
)

// PARITY-ORDERS: the list must expose bounded frozen merchandise without a
// recipient detail read. This uses real synthetic checkout lines, not a rewritten
// order snapshot; financial, authority and privacy gates remain unchanged.
func TestMerchantOrdersV2ItemSummaryContract(t *testing.T) {
	q := rm2StripePending(t)
	moGrant(t, q.f, q.f.tenantA, q.f.storeA1, q.f.principalA)
	ctx := context.Background()
	items := []storefront.Item{}
	for i := 1; i <= 4; i++ {
		sku, err := t04Scoped(ctx, q.f, q.f.tokens["a"], q.f.storeA1, "catalog:write", func(tx pgx.Tx, s platform.Scope) (catalog.SKU, error) {
			return catalog.CreateSKU(ctx, tx, s, t04Key("parity-summary-sku"), catalog.SKUInput{
				ProductID: q.stock.product.ID, Code: fmt.Sprintf("PARITY-%s-%d", t04Tag(), i), PriceMinor: 1250,
				WeightGrams: 100, LengthMM: 10, WidthMM: 20, HeightMM: 30, OriginCountry: "US", CustomsName: "synthetic item", HSCandidate: "851840",
			})
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = t04Scoped(ctx, q.f, q.f.tokens["a"], q.f.storeA1, "inventory:write", func(tx pgx.Tx, s platform.Scope) (inventory.Balance, error) {
			return inventory.AdjustOnHand(ctx, tx, s, t04Key("parity-summary-stock"), inventory.Adjustment{
				WarehouseID: q.stock.warehouse.ID, SKUID: sku.ID, Delta: 10, ExpectedVersion: 0, Reason: "synthetic summary fixture",
			})
		}); err != nil {
			t.Fatal(err)
		}
		items = append(items, storefront.Item{SKUID: sku.ID, Quantity: int64(i)})
	}
	cap := mustIssue(t, q.cqHarness.service, q.f.storeA1)
	q.bcHarness.prepare(t, cap, items)
	order, err := q.bcHarness.begin(t04Key("parity-summary-checkout"))
	if err != nil {
		t.Fatal(err)
	}
	var frozen []byte
	if err = q.f.owner.QueryRow(ctx, `SELECT snapshot#>'{quote,lines}' FROM checkout.orders WHERE id=$1`, order.OrderID).Scan(&frozen); err != nil {
		t.Fatal(err)
	}
	var lines []struct {
		Name     string `json:"name"`
		Quantity int64  `json:"quantity"`
	}
	if err = json.Unmarshal(frozen, &lines); err != nil || len(lines) != 4 {
		t.Fatalf("four real frozen lines: len=%d err=%v", len(lines), err)
	}
	before := q.bcHarness.facts(t)
	handler := httpapi.NewHandler(q.f.runtime)
	for _, search := range []bool{false, true} {
		t.Run(fmt.Sprintf("private-search=%t", search), func(t *testing.T) {
			path := "/v1/admin/stores/" + q.f.storeA1 + "/orders?view=v2&state=all"
			r := httptest.NewRequest("GET", path, nil)
			if search {
				r = httptest.NewRequest("POST", "/v1/admin/stores/"+q.f.storeA1+"/orders/search?view=v2&state=all", strings.NewReader(`{"q":"`+order.OrderID+`"}`))
				r.Header.Set("Content-Type", "application/json")
			}
			r.Header.Set("Authorization", "Bearer "+q.f.tokens["a"])
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != 200 {
				t.Fatalf("summary read status=%d", w.Code)
			}
			var page struct {
				Items []map[string]json.RawMessage `json:"items"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
				t.Fatal(err)
			}
			found := false
			for _, row := range page.Items {
				if string(row["order_id"]) != `"`+order.OrderID+`"` {
					continue
				}
				found = true
				if _, ok := row["item_summary"]; !ok {
					t.Fatal("PARITY-ORDERS: list is missing additive item_summary")
				}
				var summary struct {
					Items []struct {
						ProductName  string  `json:"product_name"`
						VariantLabel *string `json:"variant_label"`
						Quantity     int64   `json:"quantity"`
					} `json:"items"`
					TotalItemCount int64 `json:"total_item_count"`
				}
				if err := json.Unmarshal(row["item_summary"], &summary); err != nil {
					t.Fatal(err)
				}
				if len(summary.Items) != 3 || summary.TotalItemCount != 10 {
					t.Fatalf("summary must cap at 3 frozen lines and count all 10 units: %s", row["item_summary"])
				}
				for i, item := range summary.Items {
					if item.ProductName != lines[i].Name || item.Quantity != lines[i].Quantity || item.VariantLabel != nil {
						t.Fatalf("frozen summary line %d differs (no invented variant)", i)
					}
				}
				var safe map[string]json.RawMessage
				if err := json.Unmarshal(row["item_summary"], &safe); err != nil || len(safe) != 2 {
					t.Fatal("summary must contain exactly items and total_item_count")
				}
				var projected []map[string]json.RawMessage
				if err := json.Unmarshal(safe["items"], &projected); err != nil {
					t.Fatal(err)
				}
				for _, line := range projected {
					if len(line) != 3 || line["product_name"] == nil || line["variant_label"] == nil || line["quantity"] == nil {
						t.Fatal("summary line must contain only safe merchandise fields")
					}
				}
				for _, key := range []string{"destination", "recipient_name", "phone", "home_address", "snapshot"} {
					if row[key] != nil {
						t.Fatalf("list disclosed %s", key)
					}
				}
			}
			if !found {
				t.Fatal("summary fixture missing from scoped list")
			}
			for _, value := range []string{"Synthetic Buyer", "+886900000001", "Synthetic home address"} {
				if strings.Contains(w.Body.String(), value) {
					t.Fatal("list disclosed full buyer PII")
				}
			}
		})
	}
	if after := q.bcHarness.facts(t); after != before {
		t.Fatal("list/search changed business state")
	}
}

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
	q := rm2StripePending(t)
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
