//go:build browser

// Purpose: register the twelve original PARITY-LEFTOVERS signed-frame checks on real admin routes.
// Depends on: isolated PG identity, brfStartAdmin/brfPlaywright and current read DTO parsers; business projections are MOCK.
// Used by: scripts/dev/test-local.sh --browser-live-settings; no product implementation is supplied here.
// Invariants: I01/I11; signed login and server-side scope stay real, only allowlisted GET projections are mocked, plus one MOCK image move (catalog:write, real request shape) on a separate write-path product so the picker's 加入所選圖片 is really clicked.
package foundation_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/platform"
)

const bplOrder = "90000000-0000-4000-8000-000000000500"
const bplProduct = "90000000-0000-4000-8000-000000000600"
const bplSKU = "90000000-0000-4000-8000-000000000601"
const bplImage = "90000000-0000-4000-8000-000000000602"
const bplMarket = "90000000-0000-4000-8000-000000000700"

// The write-path product: only the 媒体上传 apply test touches it, so the read-only frame checks never see its moved image.
const bplWriteProduct = "90000000-0000-4000-8000-000000000800"
const bplWriteSKU = "90000000-0000-4000-8000-000000000801"
const bplWriteImage = "90000000-0000-4000-8000-000000000802"

// bplPNG is an 8x8 neutral PNG served as the MOCK detail image bytes (no real photo or asset is shipped).
var bplPNG, _ = base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAgAAAAICAIAAABLbSncAAAAEUlEQVR42mO4cXgpVsQwtCQAUNCQAb9k4poAAAAASUVORK5CYII=")

func TestBrowserParityLeftovers(t *testing.T) {
	if os.Getenv("LC_BROWSER_LIVE_SETTINGS_ACCEPTANCE") != "1" || os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" {
		t.Fatal("use scripts/dev/test-local.sh --browser-live-settings")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 700*time.Second)
	defer cancel()
	f := pwIsolatedFixture(t)
	for _, permission := range []string{"orders:read", "catalog:read", "catalog:write", "inbox:reply", "live:manage", "inventory:reserve"} {
		mustExec(t, f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,$4)`, f.tenantA, f.storeA1, f.principalA, permission)
	}
	// A separate merchant identity keeps the denied path stable while signed frames use live:manage.
	deniedPrincipal := randomUUID()
	mustExec(t, f.owner, `INSERT INTO identity.principals(id) VALUES($1)`, deniedPrincipal)
	mustExec(t, f.owner, `INSERT INTO identity.memberships(tenant_id,principal_id) VALUES($1,$2)`, f.tenantA, deniedPrincipal)
	for _, permission := range []string{"store:read", "orders:read", "inbox:reply"} {
		mustExec(t, f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,$4)`, f.tenantA, f.storeA1, deniedPrincipal, permission)
	}
	var roleCount, manageCount int
	if err := f.owner.QueryRow(ctx, `SELECT count(*) FROM identity.store_staff WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3`, f.tenantA, f.storeA1, deniedPrincipal).Scan(&roleCount); err != nil {
		t.Fatal(err)
	}
	if err := f.owner.QueryRow(ctx, `SELECT count(*) FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND permission='live:manage'`, f.tenantA, f.storeA1, deniedPrincipal).Scan(&manageCount); err != nil {
		t.Fatal(err)
	}
	if roleCount != 0 || manageCount != 0 {
		t.Fatalf("denied identity must have no staff role or live:manage grant: roles=%d grants=%d", roleCount, manageCount)
	}
	t.Run("denied", func(t *testing.T) {
		// The same real store/PG is used, but a distinct signed principal has no owner role or live:manage.
		e := &rfxEnv{sflEnv: &sflEnv{sstEnv: &sstEnv{f: f}}}
		root, err := filepath.Abs("../..")
		if err != nil {
			t.Fatal(err)
		}
		evidence := brfEvidence(t, root, "parity-leftovers-denied")
		body := bplProjections()["message-templates"]
		stack := brfStartAdmin(t, ctx, e, deniedPrincipal, evidence, func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/v1/admin/stores/"+f.storeA1+"/message-templates" {
					next.ServeHTTP(w, r)
					return
				}
				err := platform.WithScope(r.Context(), f.runtime, strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), f.storeA1, "inbox:reply", func(_ pgx.Tx, s platform.Scope) error {
					if s.TenantID != f.tenantA || s.StoreID != f.storeA1 || s.PrincipalID != deniedPrincipal {
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
		brfPlaywright(t, ctx, stack, []string{"parity-leftovers.spec.ts"}, map[string]string{"LC_BROWSER_STORE": f.storeA1, "LC_BROWSER_PARITY_DENIED": "1"})
	})
	t.Run("signed", func(t *testing.T) { bplSignedFrames(t, ctx, f) })
}

func bplSignedFrames(t *testing.T, ctx context.Context, f *testFixture) {
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
import {parseManualOptions} from %q;
import {parseSummaryPage} from %q;
const p=JSON.parse(readFileSync(process.argv[1],'utf8'));
parseProduct(p['products/%s']);
if(parseManualOptions(p['orders/manual/options']).length!==1||parseSummaryPage(p['catalog-products']).items.length!==1) throw Error('invalid manual-order projection');
if(!validImageList(p['products/%s/images'])) throw Error('invalid image projection');
parseOrderDetail(p['orders/%s'],%q);
parseOrderListV2(p.orders); parseOrderActions(p['order-actions']);
parseShipmentHistory(p['orders/%s/shipment/history']);
settingsData('templates','GET',p['message-templates']);
console.log('PASS: current product/media/order/manual-order/template DTO parsers; MOCK projections only');
`, filepath.Join(root, "apps/admin/lib/catalog-v2-model.ts"), filepath.Join(root, "apps/admin/lib/image-list-contract.ts"), filepath.Join(root, "apps/admin/lib/orders-model.ts"), filepath.Join(root, "apps/admin/lib/orders-v2.ts"), filepath.Join(root, "apps/admin/lib/live-settings-model.ts"), filepath.Join(root, "apps/admin/lib/merchant-tools-model.ts"), filepath.Join(root, "apps/admin/lib/catalog-v2-model.ts"), bplProduct, bplProduct, bplOrder, bplOrder, bplOrder)
	log := browserLog(t, filepath.Join(evidence, "fixture-contract.log"))
	cmd := exec.CommandContext(ctx, "node", "--experimental-strip-types", "--input-type=module", "-e", check, filepath.Join(evidence, "mock-projections.json"))
	cmd.Dir = root
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Run(); err != nil {
		t.Fatalf("MOCK fixture contract preflight failed: %v; evidence=%s", err, evidence)
	}
	// The one MOCK write: the picker's real move of the write-path product's detail image (Go POST images/{id}/move, catalog:write,
	// body {"role":"main"}, Idempotency-Key). The first move is refused with the real 409 conflict code and changes nothing; the
	// next moves the image to main. The write-path image list GET reads that state, so a reload shows what the server holds.
	var moveMu sync.Mutex
	moveCalls, moved := 0, false
	writeImages := "products/" + bplWriteProduct + "/images"
	stack := brfStartAdmin(t, ctx, e, f.principalA, evidence, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			prefix := "/v1/admin/stores/" + f.storeA1 + "/"
			resource := strings.TrimPrefix(r.URL.Path, prefix)
			if r.Method == http.MethodPost && r.URL.Path == prefix+writeImages+"/"+bplWriteImage+"/move" {
				var in map[string]any
				if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&in); err != nil || len(in) != 1 || in["role"] != "main" || r.Header.Get("Idempotency-Key") == "" {
					consoleJSON(w, http.StatusUnprocessableEntity, map[string]any{"code": "invalid_request"})
					return
				}
				if err := platform.WithScope(r.Context(), f.runtime, strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), f.storeA1, "catalog:write", func(_ pgx.Tx, s platform.Scope) error {
					if s.TenantID != f.tenantA || s.StoreID != f.storeA1 {
						return platform.ErrForbidden
					}
					return nil
				}); err != nil {
					consoleJSON(w, http.StatusForbidden, map[string]any{"code": "forbidden"})
					return
				}
				moveMu.Lock()
				moveCalls++
				refused := moveCalls == 1
				moved = moved || !refused
				moveMu.Unlock()
				if refused {
					consoleJSON(w, http.StatusConflict, map[string]any{"code": "conflict"})
					return
				}
				consoleJSON(w, http.StatusOK, bplWriteImages("main", 2))
				return
			}
			body, ok := projections[resource]
			if resource == writeImages {
				moveMu.Lock()
				if moved {
					body = bplWriteImages("main", 2)
				}
				moveMu.Unlock()
			}
			if !strings.HasPrefix(r.URL.Path, prefix) || !ok || r.Method != http.MethodGet {
				next.ServeHTTP(w, r)
				return
			}
			permission := "orders:read"
			if strings.HasPrefix(resource, "products/") || resource == "catalog-products" || resource == "collections" || resource == "warehouses" {
				permission = "catalog:read"
			}
			if resource == "orders/manual/options" {
				permission = "inventory:reserve" // the real manual-order permission (internal/merchanttools manualPermission)
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
			if raw, isBytes := body.([]byte); isBytes { // image bytes: the BFF accepts only a raster content type
				w.Header().Set("Content-Type", "image/png")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(raw)
				return
			}
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
	// catalog-inventory-v1.md:62 binds every SKU currency to the same real scoped store, not a UI default.
	var storeCurrency string
	if err := f.owner.QueryRow(ctx, `SELECT currency FROM control.stores WHERE tenant_id=$1 AND id=$2`, f.tenantA, f.storeA1).Scan(&storeCurrency); err != nil {
		t.Fatalf("real store currency preflight failed: %v", err)
	}
	skuCurrencies := []map[string]string{}
	mismatches := []string{}
	for _, value := range projections["products/"+bplProduct].(map[string]any)["skus"].([]any) {
		sku := value.(map[string]any)
		code, currency := sku["code"].(string), sku["currency"].(string)
		skuCurrencies = append(skuCurrencies, map[string]string{"code": code, "currency": currency})
		if currency != storeCurrency {
			mismatches = append(mismatches, fmt.Sprintf("code=%s SKU=%s store=%s", code, currency, storeCurrency))
		}
	}
	receipt, err = json.MarshalIndent(map[string]any{"store_currency": storeCurrency, "skus": skuCurrencies}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evidence, "mock-currency-receipt.json"), receipt, 0600); err != nil {
		t.Fatal(err)
	}
	if len(skuCurrencies) == 0 {
		t.Fatal("MOCK SKU currency contract preflight requires at least one SKU")
	}
	if len(mismatches) != 0 {
		t.Fatalf("MOCK SKU currency contract mismatch: %s; evidence=%s", strings.Join(mismatches, "; "), evidence)
	}
	t.Logf("MOCK fixture contracts and authenticated GET preflight passed; signed login and original 12 UI assertions follow; evidence=%s", evidence)
	brfPlaywright(t, ctx, stack, []string{"parity-leftovers.spec.ts"}, map[string]string{"LC_BROWSER_STORE": f.storeA1, "LC_BROWSER_PARITY_DENIED": "0"})
}

// bplProjections prepares valid current DTOs. A current order requires a complete frozen destination, so the missing-recipient
// frame is driven through the real manual-order draft instead (integrator ruling 2026-11-04): one catalog product and one buyer-entered
// 7-ELEVEN option; the spec leaves phone and store empty. MOCK catalog-products returns that product whatever the query says.
// The image list carries one real-shaped detail image (bytes = bplPNG) so the media picker lists a tile from the image-list contract.
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
	sku := map[string]any{"id": bplSKU, "code": "MOCK-LEFTOVER", "title": "Standard", "price_minor": 1200, "compare_at_minor": nil, "option_values": []string{}, "version": 1, "currency": "USD", "available": 10, "inventory_tracked": true, "max_per_order": nil, "keyword": "", "on_hand": 10, "committed": 0, "weight_grams": 100, "length_mm": 10, "width_mm": 10, "height_mm": 10, "origin_country": "TW", "customs_name": "Synthetic item", "hs_candidate": ""}
	product := map[string]any{"id": bplProduct, "slug": "mock-leftover", "name": "Synthetic item", "description": "MOCK catalog read", "status": "draft", "version": 1, "seo_title": "", "seo_description": "", "options": []any{}, "skus": []any{sku}, "collection_ids": []any{}, "warehouse_id": nil}
	options := map[string]any{"options": []any{map[string]any{"option_key": bplMarket + "|TW|cvs_711", "market_id": bplMarket, "country": "TW", "delivery_code": "cvs_711", "delivery_kind": "cvs_711", "mode": "MANUAL", "name_hans": "7-ELEVEN", "name_hant": "7-ELEVEN", "name_en": "7-ELEVEN", "currency": "USD", "service_version": 1, "allocation_version": 1, "payment_modes": []string{"bank_transfer"}, "pickup_selection": "buyer_entered"}}}
	summaryRow := map[string]any{"id": bplProduct, "slug": "mock-leftover", "name": "Synthetic item", "status": "draft", "version": 1, "cover_image_id": nil, "price_min_minor": 1200, "price_max_minor": 1200, "currency": "USD", "sku_count": 1, "available": 10, "keyword": "", "inventory_tracked": true, "updated_at": "2026-10-25T00:00:00Z"}
	catalogPage := map[string]any{"items": []any{summaryRow}, "next_cursor": "", "total": 1, "status_counts": map[string]int{"draft": 1, "active": 0, "archived": 0}}
	skuW, productW := map[string]any{}, map[string]any{}
	for k, v := range sku {
		skuW[k] = v
	}
	for k, v := range product {
		productW[k] = v
	}
	skuW["id"], skuW["code"] = bplWriteSKU, "MOCK-LEFTOVER-W"
	productW["id"], productW["slug"], productW["skus"] = bplWriteProduct, "mock-leftover-write", []any{skuW}
	images := map[string]any{"items": []any{map[string]any{"id": bplImage, "product_id": bplProduct, "role": "detail", "position": 0, "content_type": "image/png", "size_bytes": len(bplPNG), "width": 8, "height": 8, "version": 1}}, "image_axis": nil, "option_images": []any{}}
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
	return map[string]any{"orders": map[string]any{"items": []any{row}, "next_cursor": "", "total": 1, "counts": counts, "sessions": []any{}}, "orders/" + bplOrder: detail, "orders/" + bplOrder + "/shipment/history": map[string]any{"items": []any{}}, "order-actions": map[string]any{"refund": false, "fulfillment_write": false, "orders_export": true}, "products/" + bplProduct: product, "products/" + bplProduct + "/images": images, "products/" + bplProduct + "/images/" + bplImage: bplPNG, "products/" + bplWriteProduct: productW, "products/" + bplWriteProduct + "/images": bplWriteImages("detail", 1), "products/" + bplWriteProduct + "/images/" + bplWriteImage: bplPNG, "orders/manual/options": options, "catalog-products": catalogPage, "collections": map[string]any{"items": []any{}, "next_cursor": ""}, "warehouses": map[string]any{"items": []any{}}, "message-templates": map[string]any{"items": templates}}
}

// bplWriteImages is the write-path product's MOCK image list: its one image in role at version (current image-list DTO).
func bplWriteImages(role string, version int) map[string]any {
	return map[string]any{"items": []any{map[string]any{"id": bplWriteImage, "product_id": bplWriteProduct, "role": role, "position": 0, "content_type": "image/png", "size_bytes": len(bplPNG), "width": 8, "height": 8, "version": version}}, "image_axis": nil, "option_images": []any{}}
}
