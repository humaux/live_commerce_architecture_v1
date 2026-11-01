//go:build browser

// Purpose: register the twelve original PARITY-LEFTOVERS signed-frame checks on real admin routes.
// Depends on: isolated PG identity, brfStartAdmin/brfPlaywright and current read DTO parsers; business projections are MOCK.
// Used by: scripts/dev/test-local.sh --browser-live-settings; no product implementation is supplied here.
// Invariants: I01/I11; signed login and server-side scope stay real, only allowlisted GET projections are mocked.
package foundation_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/platform"
)

const bplOrder = "90000000-0000-4000-8000-000000000500"
const bplProduct = "90000000-0000-4000-8000-000000000600"
const bplSKU = "90000000-0000-4000-8000-000000000601"

func TestBrowserParityLeftovers(t *testing.T) {
	if os.Getenv("LC_BROWSER_LIVE_SETTINGS_ACCEPTANCE") != "1" || os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" {
		t.Fatal("use scripts/dev/test-local.sh --browser-live-settings")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 700*time.Second)
	defer cancel()
	f := pwIsolatedFixture(t)
	for _, permission := range []string{"orders:read", "catalog:read", "inbox:reply"} {
		mustExec(t, f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,$4)`, f.tenantA, f.storeA1, f.principalA, permission)
	}
	// brfStartAdmin only needs the fixture and optional refund jobs. No PSP fake or payment facts are needed for GET-only frames.
	e := &rfxEnv{sflEnv: &sflEnv{sstEnv: &sstEnv{f: f}}}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	evidence := brfEvidence(t, root, "parity-leftovers")
	projections := bplProjections()
	raw := mustJSON(t, projections)
	if err := os.WriteFile(filepath.Join(evidence, "mock-projections.json"), []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	// Fail before opening the browser if fixture DTOs drift. This checks the real production parsers, not copied validators.
	check := fmt.Sprintf(`import {readFileSync} from 'node:fs';
import {parseProduct} from %q;
import {validImageList} from %q;
import {parseOrderDetail,parseOrderActions,parseShipmentHistory} from %q;
import {parseOrderListV2} from %q;
import {settingsData} from %q;
const p=JSON.parse(readFileSync(process.argv[1],'utf8'));
parseProduct(p['products/%s']);
if(!validImageList(p['products/%s/images'])) throw Error('invalid image projection');
parseOrderDetail(p['orders/%s'],%q);
parseOrderListV2(p.orders); parseOrderActions(p['order-actions']);
parseShipmentHistory(p['orders/%s/shipment/history']);
settingsData('templates','GET',p['message-templates']);
console.log('PASS: current product/media/order/template DTO parsers; MOCK projections only');
`, filepath.Join(root, "apps/admin/lib/catalog-v2-model.ts"), filepath.Join(root, "apps/admin/lib/image-list-contract.ts"), filepath.Join(root, "apps/admin/lib/orders-model.ts"), filepath.Join(root, "apps/admin/lib/orders-v2.ts"), filepath.Join(root, "apps/admin/lib/live-settings-model.ts"), bplProduct, bplProduct, bplOrder, bplOrder, bplOrder)
	log := browserLog(t, filepath.Join(evidence, "fixture-contract.log"))
	cmd := exec.CommandContext(ctx, "node", "--experimental-strip-types", "--input-type=module", "-e", check, filepath.Join(evidence, "mock-projections.json"))
	cmd.Dir = root
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Run(); err != nil {
		t.Fatalf("MOCK fixture contract preflight failed: %v; evidence=%s", err, evidence)
	}
	stack := brfStartAdmin(t, ctx, e, f.principalA, evidence, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			prefix := "/v1/admin/stores/" + f.storeA1 + "/"
			resource := strings.TrimPrefix(r.URL.Path, prefix)
			body, ok := projections[resource]
			if !strings.HasPrefix(r.URL.Path, prefix) || !ok || r.Method != http.MethodGet {
				next.ServeHTTP(w, r)
				return
			}
			permission := "orders:read"
			if strings.HasPrefix(resource, "products/") || resource == "collections" || resource == "warehouses" {
				permission = "catalog:read"
			}
			if resource == "message-templates" {
				permission = "inbox:reply"
			}
			err := platform.WithScope(r.Context(), f.runtime, strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), f.storeA1, permission, func(_ pgx.Tx, s platform.Scope) error {
				if s.TenantID != f.tenantA || s.StoreID != f.storeA1 {
					return platform.ErrForbidden
				}
				return nil
			})
			if err != nil {
				consoleJSON(w, http.StatusForbidden, map[string]any{"code": "forbidden"})
				return
			}
			w.Header().Set("Cache-Control", "private, no-store")
			consoleJSON(w, http.StatusOK, body)
		})
	})
	// Raw API receipts separate fixture failures from legitimate UI assertion RED. Tokens never enter artifacts.
	receipts := map[string]int{}
	for resource := range projections {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, stack.api.URL+"/v1/admin/stores/"+f.storeA1+"/"+resource, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+f.tokens["a"])
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		receipts[resource] = response.StatusCode
		if response.StatusCode != http.StatusOK {
			t.Fatalf("MOCK GET preflight %s: status %d", resource, response.StatusCode)
		}
	}
	receipt, err := json.MarshalIndent(receipts, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(evidence, "mock-get-receipts.json"), receipt, 0600); err != nil {
		t.Fatal(err)
	}
	t.Logf("MOCK fixture contracts and authenticated GET preflight passed; signed login and original 12 UI assertions follow; evidence=%s", evidence)
	brfPlaywright(t, ctx, stack, []string{"parity-leftovers.spec.ts"}, map[string]string{"LC_BROWSER_STORE": f.storeA1})
}

// bplProjections prepares valid current DTOs. A current order requires a complete frozen destination;
// the missing-recipient pre-order screen has no equivalent DTO here and must fail visibly in product assertions.
func bplProjections() map[string]any {
	// orders-model requires canonical microseconds and no test-mode payment flag before any attempt exists.
	summary := map[string]any{"order_id": bplOrder, "created_at": "2026-10-25T00:00:00.000000Z", "updated_at": "2026-10-25T00:00:00.000000Z", "currency": "TWD", "total_minor": 1200, "commercial_state": "DRAFT", "fulfillment_state": "MANUAL_UNASSIGNED", "payment_state": "NOT_STARTED", "test_mode": false, "work_state": "NONE", "refunded_minor": 0, "refund_pending_minor": 0, "source": "storefront", "pickup_source": nil, "payment_mode": "card", "collection_state": nil, "cod_surcharge_minor": nil, "cod_collect_minor": nil}
	detail := map[string]any{}
	row := map[string]any{}
	for k, v := range summary {
		detail[k] = v
		row[k] = v
	}
	detail["shipment"] = nil
	detail["country"] = "TW"
	detail["service_code"] = "home"
	detail["items"] = []any{map[string]any{"sku_id": bplSKU, "code": "MOCK-LEFTOVER", "name": "Synthetic item", "quantity": 1, "unit_price_minor": 1200, "amount": map[string]any{"subtotal_minor": 1200, "discount_minor": 0, "tax_minor": 0, "total_minor": 1200}}}
	detail["totals"] = map[string]any{"subtotal_minor": 1200, "discount_minor": 0, "shipping_minor": 0, "shipping_tax_minor": 0, "tax_minor": 0, "total_minor": 1200}
	detail["destination"] = map[string]any{"kind": "home", "country": "TW", "recipient_name": "Synthetic Buyer", "phone": "+886900000001", "home_address": map[string]any{"region": "", "city": "Synthetic city", "postal_code": "", "line1": "Synthetic address", "line2": ""}, "pickup": nil}
	row["order_number"] = "LC-" + strings.ToUpper(strings.ReplaceAll(bplOrder, "-", ""))
	row["recipient_masked"] = "S***"
	row["delivery_kind"] = "home"
	row["live_sessions"] = []any{}
	row["item_summary"] = map[string]any{"items": []any{map[string]any{"product_name": "Synthetic item", "variant_label": nil, "quantity": 1}}, "total_item_count": 1}
	counts := map[string]int{"all": 1, "unpaid": 1, "transfer_review": 0, "ready_to_ship": 0, "ready_to_consign": 0, "shipped": 0, "completed": 0, "cancelled": 0}
	sku := map[string]any{"id": bplSKU, "code": "MOCK-LEFTOVER", "title": "Standard", "price_minor": 1200, "compare_at_minor": nil, "option_values": []string{}, "version": 1, "currency": "TWD", "available": 10, "inventory_tracked": true, "max_per_order": nil, "keyword": "", "on_hand": 10, "committed": 0, "weight_grams": 100, "length_mm": 10, "width_mm": 10, "height_mm": 10, "origin_country": "TW", "customs_name": "Synthetic item", "hs_candidate": ""}
	product := map[string]any{"id": bplProduct, "slug": "mock-leftover", "name": "Synthetic item", "description": "MOCK catalog read", "status": "draft", "version": 1, "seo_title": "", "seo_description": "", "options": []any{}, "skus": []any{sku}, "collection_ids": []any{}, "warehouse_id": nil}
	templates := []any{}
	for i, name := range []string{"關鍵字協助", "結帳說明", "物流查詢", "人工回覆說明", "退款說明", "直播預告"} {
		kinds := []string{"dm"}
		version := 1
		if i == 0 {
			kinds = []string{"private_reply"}
			version = 2
		}
		if i == 3 {
			kinds = []string{"dm", "private_reply"}
		}
		if i == 5 {
			kinds = []string{"public_reply", "recommend"}
		}
		templates = append(templates, map[string]any{"template_id": fmt.Sprintf("parity-leftovers/%d", i+1), "version": version, "name": name, "kinds": kinds, "public_safe": i == 5, "created_at": fmt.Sprintf("2026-10-%02dT00:00:00Z", 25-i)})
	}
	return map[string]any{"orders": map[string]any{"items": []any{row}, "next_cursor": "", "total": 1, "counts": counts, "sessions": []any{}}, "orders/" + bplOrder: detail, "orders/" + bplOrder + "/shipment/history": map[string]any{"items": []any{}}, "order-actions": map[string]any{"refund": false, "fulfillment_write": false, "orders_export": true}, "products/" + bplProduct: product, "products/" + bplProduct + "/images": map[string]any{"items": []any{}, "image_axis": nil, "option_images": []any{}}, "collections": map[string]any{"items": []any{}, "next_cursor": ""}, "warehouses": map[string]any{"items": []any{}}, "message-templates": map[string]any{"items": templates}}
}
