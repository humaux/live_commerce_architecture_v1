// Purpose: FEAT-MANUAL-DISCOUNT HTTP/REAL_PG regression contract, batched in existing CI shard roots.
// Depends on: merchant-tools and live-console harnesses, real buyer checkout, command receipts and reporting.
// Used by: TestMerchantToolsManualOrder and TestLiveConsoleOrderForBuyerLivePrice in GitHub CI only.
// Invariants: server-computed money, scoped authority, immutable named adjustment, exact replay and paid refund cap.
// Fixtures: synthetic buyer/address; stock and settings use existing public seams. No provider is contacted.
package foundation_test

import (
	"context"
	"encoding/json"
	"errors"
	"image"
	_ "image/jpeg"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/checkout"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
	"livecommerce/internal/reporting"
)

func mdBody(sku string, qty int, option, mode string, delivery map[string]any, amount int64) map[string]any {
	b := mtBody(sku, qty, option, mode, delivery)
	b["customer"] = map[string]any{"name": "測試買家", "phone": "0900000000", "email": "buyer@example.test"}
	b["discount"] = map[string]any{"name": "直播優惠", "amount_minor": amount}
	return b
}

var mdHome = map[string]any{"home_address": map[string]any{"region": "測試區", "city": "測試市", "postal_code": "100", "line1": "測試地址一號", "line2": ""}}
var mdCVS = map[string]any{"cvs": map[string]any{"store_code": "123456", "store_name": "測試門市", "store_address": "測試地址一號"}}

func mdObject(t *testing.T, v any) map[string]any {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("expected JSON object, got %T", v)
	}
	return m
}

func mdDiscount(t *testing.T, value any, amount int64) {
	t.Helper()
	d := mdObject(t, value)
	if d["name"] != "直播優惠" || d["amount_minor"] != float64(amount) {
		t.Fatalf("named discount lost or changed: %v", d)
	}
}

func mdQuote(t *testing.T, e *tcvEnv, order string) map[string]any {
	t.Helper()
	var raw []byte
	if err := e.p.f.owner.QueryRow(context.Background(), `SELECT snapshot->'quote' FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2 AND id=$3`, e.tenant(), e.store(), order).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var q map[string]any
	if err := json.Unmarshal(raw, &q); err != nil {
		t.Fatal(err)
	}
	return q
}

func mdAdjustment(t *testing.T, q map[string]any, discount int64) {
	t.Helper()
	lines, ok := q["adjustments"].([]any)
	if !ok || len(lines) != 1 {
		t.Fatalf("quote must store exactly one named adjustment: %v", q["adjustments"])
	}
	line := mdObject(t, lines[0])
	mdDiscount(t, line, discount)
	if line["kind"] != "manual_discount" || line["currency"] != q["currency"] {
		t.Fatalf("adjustment must carry quote currency and kind: %v", line)
	}
}

func mdManualContract(t *testing.T) {
	e, a, cvsCode := mtOrderEnv(t)
	sku := e.p.stock.skus[0].ID
	var home, cvs string
	for _, o := range a.options() {
		if o.DeliveryKind == "home" && strings.Contains(strings.Join(o.PaymentModes, ","), "bank_transfer") {
			home = o.OptionKey
		}
		if o.DeliveryCode == cvsCode {
			cvs = o.OptionKey
		}
	}
	if home == "" || cvs == "" {
		t.Fatal("manual-order options fixture missing")
	}
	ctx := context.Background()
	place := func(t *testing.T, body map[string]any) (string, map[string]any) {
		t.Helper()
		code, out := a.place(t04Key("md-order"), body)
		if code != 201 {
			t.Fatalf("named discount must create an order: HTTP %d code=%v", code, out["code"])
		}
		id, ok := out["order_id"].(string)
		if !ok || id == "" {
			t.Fatal("missing order id")
		}
		return id, out
	}

	t.Run("happy path, frozen adjustment, buyer link, payment and merchant detail", func(t *testing.T) {
		body := mdBody(sku, 2, home, "bank_transfer", mdHome, 100)
		body["discount"].(map[string]any)["name"] = "  直播優惠  "
		order, out := place(t, body)
		mdDiscount(t, out["discount"], 100)
		if out["total_minor"] != float64(2400) {
			t.Fatalf("expected server 2500 - 100 = 2400: %v", out["total_minor"])
		}
		q := mdQuote(t, e, order)
		mdAdjustment(t, q, 100)
		amount := mdObject(t, q["amount"])
		if amount["subtotal_minor"] != float64(2500) || amount["discount_minor"] != float64(100) || amount["total_minor"] != float64(2400) {
			t.Fatalf("quote amounts: %v", amount)
		}
		// Independent read through the link capability: both buyer order and payment view must use the same frozen quote.
		link, ok := out["buyer_link"].(string)
		if !ok {
			t.Fatal("configured manual order must return its buyer link")
		}
		_, token := e.mtLinkParts(link, "zh-TW")
		exchanged, cap := e.mtRedeem(order, token)
		if exchanged.status != 200 {
			t.Fatalf("link exchange HTTP %d", exchanged.status)
		}
		read := e.bh.request(t, "GET", "/v1/buyer/orders/"+order, cap, "", nil, nil)
		if read.status != 200 {
			t.Fatalf("buyer detail HTTP %d", read.status)
		}
		buyerOrder := cofJSON(t, read.body)
		mdAdjustment(t, mdObject(t, mdObject(t, buyerOrder["snapshot"])["quote"]), 100)
		payment := e.bh.request(t, "GET", "/v1/buyer/orders/"+order+"/bank-transfer", cap, "", nil, nil)
		if payment.status != 200 {
			t.Fatalf("payment view HTTP %d", payment.status)
		}
		if got := cofJSON(t, payment.body)["amount_minor"]; got != float64(2400) {
			t.Fatalf("bank transfer must request 2400, got %v", got)
		}
		merchant := a.call("GET", "/orders/"+order, "", "", nil)
		if merchant.Code != 200 {
			t.Fatalf("merchant detail HTTP %d", merchant.Code)
		}
		mdAdjustment(t, map[string]any{"currency": "TWD", "adjustments": cofJSON(t, merchant.Body.Bytes())["adjustments"]}, 100)
	})

	t.Run("discount name and amount in receipt and one audit, exact replay and conflicts", func(t *testing.T) {
		body := mdBody(sku, 2, home, "bank_transfer", mdHome, 100)
		key := t04Key("md-replay")
		before := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='order.manual_created'`, e.store())
		code, first := a.place(key, body)
		if code != 201 {
			t.Fatalf("initial discount placement HTTP %d code=%v", code, first["code"])
		}
		_, held, _ := e.cofBalance(sku)
		code, second := a.place(key, body)
		if code != 200 || second["order_id"] != first["order_id"] || second["total_minor"] != first["total_minor"] || second["buyer_link"] != first["buyer_link"] {
			t.Fatalf("replay changed identity/amount/link: HTTP %d", code)
		}
		mdDiscount(t, second["discount"], 100)
		if _, after, _ := e.cofBalance(sku); after != held {
			t.Fatal("discount replay reserved inventory twice")
		}
		if n := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='order.manual_created'`, e.store()); n != before+1 {
			t.Fatalf("audit count delta %d, want 1", n-before)
		}
		var receipt []byte
		if err := e.p.f.owner.QueryRow(ctx, `SELECT response FROM ops.command_results WHERE store_id=$1 AND operation='merchanttools.order.manual' AND idempotency_key=$2`, e.store(), key).Scan(&receipt); err != nil {
			t.Fatal(err)
		}
		mdDiscount(t, cofJSON(t, receipt)["discount"], 100)
		// Audit rows have no order_id column; scope by store and exact structured discount facts.
		if n := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='order.manual_created' AND details->'discount'->>'name'='直播優惠' AND (details->'discount'->>'amount_minor')::bigint=100`, e.store()); n < 1 {
			t.Fatal("manual-created audit lost its structured discount")
		}
		for _, discount := range []map[string]any{{"name": "另一優惠", "amount_minor": 100}, {"name": "直播優惠", "amount_minor": 200}} {
			body["discount"] = discount
			if code, out := a.place(key, body); code != 409 || out["code"] != "idempotency_conflict" {
				t.Fatalf("different discount under same key: HTTP %d code=%v", code, out["code"])
			}
		}
	})

	t.Run("coded validation, shipping excluded and no order on refusal", func(t *testing.T) {
		before := e.count(`SELECT count(*) FROM checkout.orders WHERE store_id=$1`, e.store())
		for _, tc := range []struct {
			name   string
			d      any
			status int
			code   string
		}{
			{"empty", map[string]any{"name": " ", "amount_minor": 100}, 422, "invalid_request"},
			{"too long", map[string]any{"name": strings.Repeat("惠", 31), "amount_minor": 100}, 422, "invalid_request"},
			{"zero", map[string]any{"name": "直播優惠", "amount_minor": 0}, 422, "invalid_request"},
			{"negative", map[string]any{"name": "直播優惠", "amount_minor": -1}, 422, "invalid_request"},
			{"over goods", map[string]any{"name": "直播優惠", "amount_minor": 2501}, 422, "discount_exceeds_subtotal"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				b := mdBody(sku, 2, home, "bank_transfer", mdHome, 100)
				b["discount"] = tc.d
				if code, out := a.place(t04Key("md-invalid"), b); code != tc.status || out["code"] != tc.code {
					t.Fatalf("HTTP %d code=%v, want %d %s", code, out["code"], tc.status, tc.code)
				}
			})
		}
		if n := e.count(`SELECT count(*) FROM checkout.orders WHERE store_id=$1`, e.store()); n != before {
			t.Fatalf("invalid discount created %d orders", n-before)
		}
		code, _, _ := e.service("cvs_711", "MANUAL", 700)
		var shipped string
		for _, o := range a.options() {
			if o.DeliveryCode == code {
				shipped = o.OptionKey
			}
		}
		if shipped == "" {
			t.Fatal("missing shipping fixture")
		}
		if status, out := a.place(t04Key("md-shipping-refuse"), mdBody(sku, 2, shipped, "bank_transfer", mdCVS, 2501)); status != 422 || out["code"] != "discount_exceeds_subtotal" {
			t.Fatalf("shipping must not extend discountable goods: HTTP %d code=%v", status, out["code"])
		}
		order, out := place(t, mdBody(sku, 2, shipped, "bank_transfer", mdCVS, 2500))
		if out["total_minor"] != float64(700) || mdObject(t, mdQuote(t, e, order)["amount"])["shipping_minor"] != float64(700) {
			t.Fatal("full goods discount must retain shipping of 700")
		}
	})

	t.Run("permissions, cross tenant/store and injected body scope", func(t *testing.T) {
		body := mdBody(sku, 2, home, "bank_transfer", mdHome, 100)
		before := e.count(`SELECT count(*) FROM checkout.orders WHERE store_id=$1`, e.store())
		for _, perms := range [][]string{{"orders:read", "catalog:read"}, {"inventory:reserve"}} {
			staff, _ := e.member(perms...)
			if code, out := a.with(staff).place(t04Key("md-staff"), body); code != 403 {
				t.Fatalf("missing permission must refuse: HTTP %d code=%v", code, out["code"])
			}
		}
		for _, store := range []string{e.p.f.storeA2, e.p.f.storeB} {
			other := a
			other.store = store
			if code, out := other.place(t04Key("md-foreign"), body); code != 404 {
				t.Fatalf("foreign store must remain opaque: HTTP %d code=%v", code, out["code"])
			}
		}
		for _, field := range []string{"tenant_id", "store_id", "total_minor"} {
			b := mdBody(sku, 2, home, "bank_transfer", mdHome, 100)
			b[field] = e.p.f.storeB
			if code, _ := a.place(t04Key("md-body-scope"), b); code != 400 {
				t.Fatalf("unknown body authority/price field %s must be refused: HTTP %d", field, code)
			}
		}
		if n := e.count(`SELECT count(*) FROM checkout.orders WHERE store_id=$1`, e.store()); n != before {
			t.Fatal("refused authority created an order")
		}
	})

	t.Run("proportional allocation floors early lines and rounds to last, immutable on stale version", func(t *testing.T) {
		items := []map[string]any{{"sku_id": e.sku(1250, 10), "quantity": 1}, {"sku_id": e.sku(1250, 10), "quantity": 1}, {"sku_id": e.sku(1250, 10), "quantity": 1}}
		b := mdBody(sku, 1, home, "bank_transfer", mdHome, 100)
		b["items"] = items
		order, out := place(t, b)
		q := mdQuote(t, e, order)
		lines, ok := q["lines"].([]any)
		if !ok || len(lines) != 3 {
			t.Fatal("expected three frozen quote lines")
		}
		var refundable float64
		for i, raw := range lines {
			line := mdObject(t, raw)
			amount := mdObject(t, line["amount"])
			want := float64(33)
			if i == 2 {
				want = 34
			}
			if amount["discount_minor"] != want || amount["total_minor"] != 1250-want {
				t.Fatalf("line %d allocation: %v, want discount %v", i, amount, want)
			}
			refundable += amount["total_minor"].(float64)
		}
		if refundable != 3650 {
			t.Fatalf("partial line refund caps must sum to paid goods 3650, got %v", refundable)
		}
		_, token := e.mtLinkParts(out["buyer_link"].(string), "zh-TW")
		exchange, cap := e.mtRedeem(order, token)
		if exchange.status != 200 {
			t.Fatalf("link exchange HTTP %d", exchange.status)
		}
		saved, err := e.svc.Get(ctx, cap, e.store(), order)
		if err != nil {
			t.Fatal(err)
		}
		before := e.count(`SELECT count(*) FROM checkout.orders WHERE store_id=$1`, e.store())
		_, err = e.svc.Begin(ctx, cap, e.store(), t04Key("md-stale-version"), checkout.Input{
			QuoteID: saved.Snapshot.Quote.ID, DestinationID: saved.Snapshot.Destination.ID,
			CartVersion: saved.Snapshot.Quote.CartVersion + 1, ServiceVersion: saved.Snapshot.Service.Version,
			AllocationVersion: saved.Snapshot.Allocation.Version, PaymentMode: "bank_transfer",
		})
		if !errors.Is(err, command.ErrConflict) {
			t.Fatalf("stale cart version must conflict, got %v", err)
		}
		if n := e.count(`SELECT count(*) FROM checkout.orders WHERE store_id=$1`, e.store()); n != before {
			t.Fatal("version conflict created another discounted order")
		}
		mdAdjustment(t, mdQuote(t, e, order), 100)
	})

	t.Run("pay at pickup uses discounted total", func(t *testing.T) {
		_, out := place(t, mdBody(sku, 2, cvs, "pay_at_pickup", mdCVS, 100))
		if out["total_minor"] != float64(2400) || out["commercial_state"] != "CONFIRMED" {
			t.Fatal("pickup must collect the discounted 2400")
		}
	})

	t.Run("allocation never overdraws a cheap last line and never overflows int64", func(t *testing.T) {
		for _, tc := range []struct {
			price, discount int64
			count           int
		}{
			{1, 2, 3},                             // Naive floor/floor/all-remainder would subtract 2 from a last-line subtotal of 1.
			{400_000_000_000, 500_000_000_000, 2}, // discount * line subtotal exceeds int64.
		} {
			items := make([]map[string]any, tc.count)
			for i := range items {
				items[i] = map[string]any{"sku_id": e.sku(tc.price, 5), "quantity": 1}
			}
			body := mdBody(sku, 1, home, "bank_transfer", mdHome, tc.discount)
			body["items"] = items
			order, out := place(t, body)
			q := mdQuote(t, e, order)
			mdAdjustment(t, q, tc.discount)
			var discount, refundable float64
			for _, raw := range q["lines"].([]any) {
				amount := mdObject(t, mdObject(t, raw)["amount"])
				d, total := amount["discount_minor"].(float64), amount["total_minor"].(float64)
				if d < 0 || d > float64(tc.price) || total < 0 || d+total != float64(tc.price) {
					t.Fatalf("line may never refund more than its paid value: %v", amount)
				}
				discount += d
				refundable += total
			}
			if discount != float64(tc.discount) || refundable != float64(int64(tc.count)*tc.price-tc.discount) || out["total_minor"] != refundable {
				t.Fatalf("allocation must conserve discount and paid money: %v / %v", discount, refundable)
			}
		}
	})

	t.Run("COD limit rechecked after discount including surcharge", func(t *testing.T) {
		if status, out := e.hcodSettings(0, true, 20, 1, "black_cat"); status != 200 {
			t.Fatalf("COD settings fixture: HTTP %d code=%v", status, out["code"])
		}
		var codHome string
		for _, o := range a.options() {
			if o.DeliveryKind == "home" && strings.Contains(strings.Join(o.PaymentModes, ","), "cash_on_delivery") && o.DeliveryCode == strings.Split(home, "|")[2] {
				codHome = o.OptionKey
			}
		}
		if codHome == "" {
			t.Fatal("missing COD-enabled home fixture")
		}
		// Goods 2500 exceeds cap 2000, but 2500 - 600 + surcharge 100 equals cap exactly.
		order, out := place(t, mdBody(sku, 2, codHome, "cash_on_delivery", mdHome, 600))
		if out["total_minor"] != float64(1900) {
			t.Fatal("COD order total must be 1900")
		}
		_, _, _, _, total, surcharge := e.hcodRow(order)
		if total+surcharge != 2000 {
			t.Fatalf("COD collect %d + %d, want 2000", total, surcharge)
		}
		if code, out := a.place(t04Key("md-cod-over"), mdBody(sku, 2, codHome, "cash_on_delivery", mdHome, 500)); code != 422 || out["code"] != "cod_limit" {
			t.Fatalf("COD final total over cap: HTTP %d code=%v", code, out["code"])
		}
	})

	t.Run("offline confirmation, reports and refund cap read discounted money", func(t *testing.T) {
		order, _ := place(t, mdBody(sku, 2, home, "bank_transfer", mdHome, 100))
		if status, out := e.cofDecide(e.token(), order, "confirm", t04Key("md-confirm"), `{}`); status != 200 {
			t.Fatalf("confirm: HTTP %d code=%v", status, out["code"])
		}
		var paid int64
		if err := e.p.f.owner.QueryRow(ctx, `SELECT confirmed_amount_minor FROM checkout.bank_transfers WHERE order_id=$1`, order).Scan(&paid); err != nil {
			t.Fatal(err)
		}
		if paid != 2400 {
			t.Fatalf("confirmed %d, want discounted 2400", paid)
		}
		today := time.Now().In(rpTPE).Format("2006-01-02")
		rep, err := t04Scoped(ctx, e.p.f, e.token(), e.store(), "orders:read", func(tx pgx.Tx, scope platform.Scope) (reporting.ManualReport, error) {
			return reporting.ManualOrders(ctx, tx, scope, e.token(), today, today, "LIVE")
		})
		if err != nil {
			t.Fatal(err)
		}
		var offline int64
		for _, row := range rep.Rows {
			for _, money := range row.Money {
				offline += money.OfflineMinor
			}
		}
		if offline != 2400 {
			t.Fatalf("manual-order report must conserve discounted collection 2400, got %d", offline)
		}
		products, err := t04Scoped(ctx, e.p.f, e.token(), e.store(), "orders:read", func(tx pgx.Tx, scope platform.Scope) (reporting.ProductReport, error) {
			return reporting.Products(ctx, tx, scope, e.token(), today, today)
		})
		if err != nil {
			t.Fatal(err)
		}
		var productOffline int64
		for _, row := range products.Rows {
			productOffline += row.OfflineMinor
		}
		if productOffline != 2400 {
			t.Fatalf("product report must conserve discounted collection 2400, got %d", productOffline)
		}
		if status, out := e.cofDecide(e.token(), order, "refund-offline", t04Key("md-refund"), `{}`); status != 200 {
			t.Fatalf("refund: HTTP %d code=%v", status, out["code"])
		}
		if status, out := e.cofDecide(e.token(), order, "refund-offline", t04Key("md-refund-twice"), `{}`); status != 409 || out["code"] != "already_refunded" {
			t.Fatalf("refund beyond paid must fail: HTTP %d code=%v", status, out["code"])
		}
		mdAdjustment(t, mdQuote(t, e, order), 100)
	})
}

func mdForBuyerContract(t *testing.T) {
	e := lbuNew(t)
	body := e.body(2, nil, "", false)
	body["customer"] = map[string]any{"name": "測試買家", "phone": "0900000000", "email": "buyer@example.test"}
	body["delivery"].(map[string]any)["home_address"] = mdHome["home_address"]
	body["discount"] = map[string]any{"name": "直播優惠", "amount_minor": 100}
	key := t04Key("md-for-buyer")
	code, first := e.post(e.token(), key, body)
	if code != 201 {
		t.Fatalf("shared for-buyer pipeline must accept discount: HTTP %d code=%v", code, first["code"])
	}
	mdDiscount(t, first["discount"], 100)
	if len(e.plan.links) != 0 {
		t.Fatal("discount order must not send an unrequested customer message")
	}
	code, replay := e.post(e.token(), key, body)
	if code != 200 || replay["order_id"] != first["order_id"] || replay["total_minor"] != first["total_minor"] {
		t.Fatalf("for-buyer replay changed: HTTP %d", code)
	}
	mdDiscount(t, replay["discount"], 100)
	body["discount"] = map[string]any{"name": "另一優惠", "amount_minor": 100}
	if code, out := e.post(e.token(), key, body); code != 409 || out["code"] != "idempotency_conflict" {
		t.Fatalf("for-buyer discount replay conflict: HTTP %d code=%v", code, out["code"])
	}
}

func mdPickerContract(t *testing.T, a v2Admin, product, query string) {
	t.Helper()
	a.t = t
	var page struct {
		Items []map[string]any `json:"items"`
	}
	a.do("GET", "/catalog-products?q="+query+"&limit=100", "", nil, 200, &page)
	var pictured, without bool
	for _, row := range page.Items {
		value, present := row["cover_image_url"]
		if !present {
			t.Fatalf("picker list must add nullable cover_image_url: product %v", row["id"])
		}
		if row["cover_image_id"] == nil {
			without = true
			if value != nil {
				t.Fatal("a product without an image must return null thumbnail")
			}
		}
		if row["id"] != product {
			continue
		}
		pictured = true
		cover, ok := row["cover_image_id"].(string)
		if !ok {
			t.Fatal("pictured fixture lost its primary image")
		}
		path := "/products/" + product + "/images/" + cover + "?w=360"
		if value != "/api/stores/"+a.store+path {
			t.Fatalf("thumbnail must use existing authenticated media path at width 360: %v", value)
		}
		w := a.raw("GET", path, "", nil, "", 200, nil)
		cfg, format, err := image.DecodeConfig(w.Body)
		if err != nil || format != "jpeg" || cfg.Width > 360 {
			t.Fatalf("thumbnail rendition: %dx%d %s err=%v", cfg.Width, cfg.Height, format, err)
		}
		other := a
		other.token = ""
		other.raw("GET", path, "", nil, "", 401, nil)
	}
	if !pictured || !without {
		t.Fatalf("fixture must cover pictured and image-less products: %v/%v", pictured, without)
	}
}
