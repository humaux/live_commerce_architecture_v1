//go:build browser

package foundation_test

// Independent promotions browser gate (R4 test author; docs/delivery/units/promotions.md, contracts/storefront-v2.md §F). `TestBrowserPromotions`.
// Run through `bash scripts/dev/test-local.sh --browser-promotions` (isolated PG 18, production admin + storefront Next builds).
//
// Stack (MOCK tier): production admin Next + signed MOCK IdP + real BFF -> Go API -> PG for the merchant (brcStartAdmin), the production storefront Next
// behind a disposable TLS/CONNECT edge for the anonymous buyer (tests/storefront/promotions-gate.mjs), the real buyer HTTP handler behind it.
// Evidence labels: BROWSER, MOCK IdP, bank_transfer payment mode (no PSP exists on this path); not Stripe SANDBOX, not WebKit, no real DNS/TLS.
// Seeded fixtures (disclosed, owner-pool or in-process through the real domain functions): the store, its market/policy/delivery service (psSetup), the
// bank-transfer settings (the real merchant route), the 4 x 2 products with stock, the storefront publication + domain row of tcvNew (bhPublish).
// Everything the journey is about goes through the browser: the merchant creates the codes in /promotions, the buyer applies them in the checkout of
// the new storefront, places the bank_transfer order, the merchant reads the order and confirms the transfer, finance shows the confirmed amount.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"livecommerce/internal/catalog"
	"livecommerce/internal/fulfillment"
	"livecommerce/internal/inventory"
	"livecommerce/internal/platform"

	"github.com/jackc/pgx/v5"
)

type brpProductFixture struct {
	ID    string `json:"id"`
	SKU   string `json:"sku_id"`
	Name  string `json:"name"`
	Price int64  `json:"price"`
}

type brpCell struct {
	Locale   string              `json:"locale"`
	Viewport string              `json:"vp"`
	Code     string              `json:"code"`
	BigCode  string              `json:"big_code"`
	Products []brpProductFixture `json:"products"`
}

// brpProduct creates an active product with one SKU in the harness warehouse (so the delivery allocation covers it) and stocks it.
func brpProduct(t *testing.T, e *tcvEnv, name string, price int64) brpProductFixture {
	t.Helper()
	ctx := context.Background()
	f := e.p.f
	product, err := t04Scoped(ctx, f, f.tokens["a"], f.storeA1, "catalog:write", func(tx pgx.Tx, s platform.Scope) (catalog.Product, error) {
		return catalog.CreateProduct(ctx, tx, s, t04Key("brp-product"), catalog.ProductInput{Name: name, Description: "Synthetic promotions browser gate fixture", Status: catalog.StatusActive})
	})
	if err != nil {
		t.Fatalf("create product: %v", err)
	}
	tag := t04Tag()
	sku, err := t04Scoped(ctx, f, f.tokens["a"], f.storeA1, "catalog:write", func(tx pgx.Tx, s platform.Scope) (catalog.SKU, error) {
		return catalog.CreateSKU(ctx, tx, s, t04Key("brp-sku"), catalog.SKUInput{ProductID: product.ID, Code: "BRP-" + tag, PriceMinor: price, WeightGrams: 100, LengthMM: 10, WidthMM: 20, HeightMM: 30,
			OriginCountry: "TW", CustomsName: "test item", HSCandidate: "851840"})
	})
	if err != nil {
		t.Fatalf("create sku: %v", err)
	}
	if _, err = t04Scoped(ctx, f, f.tokens["a"], f.storeA1, "inventory:write", func(tx pgx.Tx, s platform.Scope) (inventory.Balance, error) {
		return inventory.AdjustOnHand(ctx, tx, s, t04Key("brp-stock"), inventory.Adjustment{WarehouseID: e.p.stock.warehouse.ID, SKUID: sku.ID, Delta: 50, ExpectedVersion: 0, Reason: "promotions browser fixture"})
	}); err != nil {
		t.Fatalf("stock: %v", err)
	}
	return brpProductFixture{ID: product.ID, SKU: sku.ID, Name: name, Price: price}
}

func TestBrowserPromotions(t *testing.T) {
	if os.Getenv("LC_BROWSER_PROMOTIONS_ACCEPTANCE") != "1" || os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" {
		t.Fatal("use scripts/dev/test-local.sh --browser-promotions")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel()
	root, _ := filepath.Abs("../..")
	evidence := brfEvidence(t, root, "promotions")
	const origin = "https://buyer.example"
	e := tcvNew(t, tcvOpts{origin: origin})
	f := e.p.f
	e.grantCreator("pricing:read", "pricing:write", "orders:read", "orders:export", "payments:refund", "fulfillment:write")
	e.cofEnsureSettings(0, true, true, 72) // bank_transfer enabled through the real merchant route

	// One flat-fee home service (NT$60) with names the browser can select by (the harness 'home' service keeps its own names).
	fee := int64(6000)
	in := e.p.delivery
	in.Code, in.ExpectedVersion = "pgx"+t04Tag(), 0
	in.NameEN, in.NameHant, in.NameHans = "Promo Express", "促銷快遞", "促销快递"
	pol := e.p.policy
	pol.Method, pol.ExpectedVersion, pol.ShippingMinor, pol.Enabled, pol.FreeShippingThresholdMinor = "delivery:"+in.Code, 0, &fee, true, nil
	if _, err := e.p.setPolicy(pol); err != nil {
		t.Fatal(err)
	}
	// delivery-allocation P0: enable the service through the real merchant settings route (the same BFF/Go
	// route the admin UI uses), so the auto-allocation is what the buyer sees; the store has exactly one
	// warehouse, so the response must name it as the chosen default.
	serviceBody, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	servicePath := "/v1/admin/stores/" + f.storeA1 + "/markets/" + e.p.market.ID + "/countries/TW/delivery-services/" + in.Code
	enabled := adminRequest(e.merchant, http.MethodPut, servicePath, f.tokens["a"], serviceBody, "application/json", map[string]string{"Idempotency-Key": t04Key("brp-service")})
	if enabled.Code != http.StatusOK {
		t.Fatalf("settings delivery-service enable status=%d body=%s", enabled.Code, enabled.Body.String())
	}
	var enabledService fulfillment.Service
	if err := json.Unmarshal(enabled.Body.Bytes(), &enabledService); err != nil {
		t.Fatal(err)
	}
	if !enabledService.Enabled || enabledService.Version != 1 || enabledService.DefaultWarehouseID != e.p.stock.warehouse.ID {
		t.Fatalf("auto-allocation default=%q service=%+v", enabledService.DefaultWarehouseID, enabledService)
	}
	matrix := []struct{ locale, vp string }{{"zh-TW", "desktop"}, {"zh-TW", "mobile"}, {"en", "desktop"}, {"en", "mobile"}}
	cells := make([]brpCell, len(matrix))
	for i, m := range matrix {
		cells[i] = brpCell{Locale: m.locale, Viewport: m.vp, Code: fmt.Sprintf("PGB%dTEN", i+1), BigCode: fmt.Sprintf("PGB%dBIG", i+1),
			Products: []brpProductFixture{brpProduct(t, e, fmt.Sprintf("Promo gate item %d A", i+1), 40000), brpProduct(t, e, fmt.Sprintf("Promo gate item %d B", i+1), 60000)}}
	}
	fixtures, _ := json.Marshal(map[string]any{"cells": cells, "currency": "TWD", "store": e.store(), "delivery_en": in.NameEN, "delivery_hant": in.NameHant})

	ordersBefore := countRows(t, f.owner, `SELECT count(*) FROM checkout.orders WHERE store_id=$1`, e.store()) // psSetup places one hold of its own
	stack := brcStartAdmin(t, ctx, e, evidence)
	cmd := exec.CommandContext(ctx, "node", "tests/storefront/promotions-gate.mjs")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	defer func() {
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
	}()
	cmd.Dir = root
	cmd.Env = browserEnvironment(map[string]string{
		"COMMERCE_BUYER_WEB_ENABLED": "1", "COMMERCE_BUYER_DEMO_LABEL": "1", "COMMERCE_BUYER_API_ORIGIN": e.bh.server.URL, "COMMERCE_BUYER_BFF_KEY": e.bh.key,
		"COMMERCE_BUYER_COOKIE_KEY": brToken(), "COMMERCE_BUYER_SESSION_TTL": "3600",
		"LC_PM_EVIDENCE": evidence, "LC_PM_ADMIN_ORIGIN": stack.origin, "LC_PM_FIXTURES": string(fixtures),
	})
	log := browserLog(t, filepath.Join(evidence, "browser.log"))
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Run(); err != nil {
		t.Fatalf("promotions browser gate failed: %v; evidence=%s", err, evidence)
	}

	// ---- independent PostgreSQL readback of what the browsers did ------------------------------------------------------------
	raw, err := os.ReadFile(filepath.Join(evidence, "result.json"))
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Cases   int `json:"cases"`
		Results []struct {
			Locale   string `json:"locale"`
			Viewport string `json:"viewport"`
			Code     string `json:"code"`
			BigCode  string `json:"big_code"`
			OrderID  string `json:"order_id"`
			Starts   string `json:"starts_wall"`
			Ends     string `json:"ends_wall"`
		} `json:"results"`
	}
	if json.Unmarshal(raw, &result) != nil || len(result.Results) != len(matrix) || result.Cases < len(matrix)*8 {
		t.Fatalf("missing exact browser gate results (cases=%d results=%d)", result.Cases, len(result.Results))
	}
	owner := f.owner
	taipei := func(wall string) time.Time {
		at, err := time.ParseInLocation("2006-01-02T15:04", wall, pgTaipei)
		if err != nil {
			t.Fatalf("bad Taipei wall time %q: %v", wall, err)
		}
		return at
	}
	for i, r := range result.Results {
		fx := cells[i]
		var kind string
		var percent, minimum int64
		var starts, ends time.Time
		var status string
		if err := owner.QueryRow(ctx, `SELECT kind,coalesce(percent,0),min_subtotal_minor,starts_at,ends_at,status FROM promotions.codes WHERE tenant_id=$1 AND store_id=$2 AND code=$3`,
			e.tenant(), e.store(), r.Code).Scan(&kind, &percent, &minimum, &starts, &ends, &status); err != nil {
			t.Fatalf("cell %d code %s: %v", i, r.Code, err)
		}
		if kind != "percent" || percent != 10 || minimum != 50000 || status != "active" {
			t.Errorf("cell %d code: %s %d%% min %d %s, want a 10%% percent code with minimum 50000 (NT$500)", i, kind, percent, minimum, status)
		}
		if !starts.Equal(taipei(r.Starts)) || !ends.Equal(taipei(r.Ends)) {
			t.Errorf("cell %d window stored as %v .. %v, the merchant typed Taipei wall time %s .. %s", i, starts.UTC(), ends.UTC(), r.Starts, r.Ends)
		}
		var total, subtotal, discount, shipping, redDiscount int64
		var state, mode string
		if err := owner.QueryRow(ctx, `SELECT o.total_minor,(o.snapshot#>>'{quote,amount,subtotal_minor}')::bigint,(o.snapshot#>>'{quote,amount,discount_minor}')::bigint,
			(o.snapshot#>>'{quote,amount,shipping_minor}')::bigint,r.discount_minor,o.commercial_state,o.payment_mode
			FROM checkout.orders o JOIN promotions.redemptions r ON r.order_id=o.id JOIN promotions.codes c ON c.id=r.code_id WHERE o.id=$1 AND c.code=$2 AND o.store_id=$3`,
			r.OrderID, r.Code, e.store()).Scan(&total, &subtotal, &discount, &shipping, &redDiscount, &state, &mode); err != nil {
			t.Fatalf("cell %d order %s: %v", i, r.OrderID, err)
		}
		if subtotal != 100000 || discount != 10000 || redDiscount != 10000 || shipping != 6000 || total != 96000 || state != "CONFIRMED" || mode != "bank_transfer" {
			t.Errorf("cell %d order: subtotal %d discount %d/%d shipping %d total %d %s %s, want 100000/10000/10000/6000/96000 CONFIRMED bank_transfer", i, subtotal, discount, redDiscount, shipping, total, state, mode)
		}
		var confirmed int64
		if err := owner.QueryRow(ctx, `SELECT confirmed_amount_minor FROM checkout.bank_transfers WHERE order_id=$1 AND state='CONFIRMED'`, r.OrderID).Scan(&confirmed); err != nil || confirmed != 96000 {
			t.Errorf("cell %d confirmed transfer amount %d (%v), want the discounted total 96000", i, confirmed, err)
		}
		if n := countRows(t, owner, `SELECT count(*) FROM promotions.redemptions r JOIN promotions.codes c ON c.id=r.code_id WHERE c.code=$1`, r.BigCode); n != 0 {
			t.Errorf("cell %d: the below-minimum code %s was redeemed %d times", i, r.BigCode, n)
		}
		_ = fx
	}
	if n := countRows(t, owner, `SELECT count(*) FROM checkout.orders WHERE store_id=$1`, e.store()); n != ordersBefore+len(matrix) {
		t.Errorf("%d orders in the store, want exactly %d + %d (one per cell: refused codes never placed an order)", n, ordersBefore, len(matrix))
	}
	if _, confirmedMinor, _, _ := e.pgFinance(); confirmedMinor != float64(96000*len(matrix)) {
		t.Errorf("finance bank_transfer_confirmed_minor %v, want %d", confirmedMinor, 96000*len(matrix))
	}
	var shots []struct{ File, Sha256, Locale, Viewport, Page string }
	rawShots, err := os.ReadFile(filepath.Join(evidence, "screenshots.json"))
	if err != nil || json.Unmarshal(rawShots, &shots) != nil || len(shots) < len(matrix)*4 {
		t.Fatalf("screenshot manifest: %d entries, %v", len(shots), err)
	}
	for _, s := range shots {
		info, err := os.Stat(filepath.Join(evidence, s.File))
		if err != nil || info.Size() < 2000 || len(s.Sha256) != 64 {
			t.Errorf("screenshot %s missing, tiny or unhashed", s.File)
		}
	}
	t.Logf("BROWSER (MOCK IdP, bank_transfer) promotions gate passed: 4 cells (zh-TW/en x desktop/390px), %d cases, DB readback exact; evidence=%s", result.Cases, evidence)
}
