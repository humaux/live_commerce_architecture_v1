package foundation_test

// merchant_tools_order_smoke_test.go: author smoke of unit merchant-tools G1 (dashboard) and G3 (manual, merchant-created order) through the
// REAL merchant handler, the REAL buyer pipeline (storefront / checkout / begin_hold) and the REAL buyer HTTP handler on real PostgreSQL.
// Evidence label: REAL_PG (isolated disposable PG; no PSP exists on the bank-transfer / pay-at-pickup paths, so nothing is SANDBOX or LIVE).
//   MTO01 TestMerchantToolsManualOrder      options (no card) -> home + bank transfer / CVS + pay at pickup / CVS + bank transfer ->
//                                           source=merchant_manual, audit, stock reserved once, buyer link works, replay, other body 409,
//                                           refusals (card, unavailable mode, unknown SKU, insufficient stock, price field), authority
//   MTO02 TestMerchantToolsDashboard        counts, GMV equals the finance route (no second ledger), to-do counters, low-stock boundary,
//                                           latest orders, read-only, authority
//   MTO03 TestMerchantToolsDashboardScale   10,000 orders: the whole read stays under the 300 ms budget (measurement)
// Owner-pool writes (disclosed fixtures): grants (tcvEnv.grantCreator / member), and for MTO03 a bulk copy of one real order row with
// session_replication_role=replica (FKs off) to reach 10,000 rows.

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/buyer"
	"livecommerce/internal/catalog"
	"livecommerce/internal/fulfillment"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/inventory"
	"livecommerce/internal/merchantorders"
	"livecommerce/internal/merchanttools"
	"livecommerce/internal/pagination"
	"livecommerce/internal/platform"
	"livecommerce/internal/pricing"
	"livecommerce/internal/reporting"
)

// mtOrderEnv builds the CVS/checkout environment, bank transfer (72 h, CVS allowed), a MANUAL 7-ELEVEN service with pay-at-pickup on,
// and the merchant handler that carries the manual-order pipeline wired exactly as cmd/api wires it.
func mtOrderEnv(t *testing.T, opts ...tcvOpts) (*tcvEnv, mtAdmin, string) {
	t.Helper()
	e := tcvNew(t, opts...)
	e.grantCreator("orders:read", "integration:manage", "integration:read", "payments:refund", "fulfillment:write")
	e.topUp()
	e.cofEnsureSettings(0, true, true, 72)
	cvsCode, _, _ := e.service("cvs_711", "MANUAL", 0)
	e.cvsSettings(tcvAllChains, true, "20000", 20)
	manual := mtManualOrders(t, e)
	h := httpapi.NewHandler(e.p.f.runtime, httpapi.Options{CVS: e.cvs, ManualOrders: manual})
	return e, mtAdmin{t: t, h: h, store: e.store(), token: e.token()}, cvsCode
}

// mtManualOrders wires the manual-order pipeline the way cmd/api buildBuyerWithCVS does: its own 30-day issuer service, the buyer runtime pool, the FINAL
// checkout service, and a deployment secret.
func mtManualOrders(t *testing.T, e *tcvEnv) *merchanttools.ManualOrders {
	t.Helper()
	links, err := buyer.New(e.p.a.issuer, 30*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	secret := base64.RawURLEncoding.EncodeToString(randomBytes(32))
	manual, err := merchanttools.NewManualOrders(e.p.f.runtime, links, e.p.a.runtime, e.svc, []byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return manual
}

type mtOptions struct {
	Options []merchanttools.ManualOption `json:"options"`
}

func (a mtAdmin) options() []merchanttools.ManualOption {
	a.t.Helper()
	w := a.call("GET", "/orders/manual/options", "", "", nil)
	if w.Code != 200 {
		a.t.Fatalf("options: %d %s", w.Code, w.Body.String())
	}
	var out mtOptions
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		a.t.Fatal(err)
	}
	return out.Options
}

var mtIPSeed atomic.Int64

// mtRedeem exchanges a manual-order link for a buyer capability through the REAL buyer handler: a fresh bearer (the BFF's minted token), the link token in the
// body, and a distinct client IP per call so the shared 10-per-10-minutes IP throttle bucket is never the thing under test. It returns the HTTP answer and the bearer.
func (e *tcvEnv) mtRedeem(order, linkToken string) (bhResponse, string) {
	e.t.Helper()
	bearer := randomToken()
	n := mtIPSeed.Add(1)
	res := e.bh.request(e.t, "POST", "/v1/buyer/orders/link", bearer, "", map[string]any{"order_id": order, "token": linkToken},
		func(r *http.Request) {
			r.Header.Set("X-Commerce-Client-IP", fmt.Sprintf("198.51.%d.%d", n/250, n%250+1))
		})
	return res, bearer
}

// mtLinkParts splits a returned buyer_link into (order id, link token) and checks its shape against the storefront origin.
func (e *tcvEnv) mtLinkParts(link, locale string) (order, token string) {
	e.t.Helper()
	rest, ok := strings.CutPrefix(link, e.origin+"/"+locale+"/order-link#o=")
	order, token, found := strings.Cut(rest, "&t=")
	if !ok || !found || len(order) != 36 || len(token) != 43 {
		e.t.Fatalf("link shape: %q", link)
	}
	return order, token
}

func (a mtAdmin) place(key string, body map[string]any) (int, map[string]any) {
	a.t.Helper()
	raw, _ := json.Marshal(body)
	w := a.call("POST", "/orders/manual", key, "application/json", raw)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func mtBody(sku string, qty int, optionKey, mode string, delivery map[string]any) map[string]any {
	d := map[string]any{"option_key": optionKey}
	for k, v := range delivery {
		d[k] = v
	}
	return map[string]any{
		"items":        []map[string]any{{"sku_id": sku, "quantity": qty}},
		"customer":     map[string]any{"name": "王小明", "phone": "0912-345-678", "email": "buyer@example.test"},
		"delivery":     d,
		"payment_mode": mode,
		"locale":       "zh-TW",
	}
}

var (
	mtHome = map[string]any{"home_address": map[string]any{"region": "台北市", "city": "中正區", "postal_code": "100", "line1": "忠孝東路1號", "line2": ""}}
	mtCVS  = map[string]any{"cvs": map[string]any{"store_code": "131386", "store_name": "建盛", "store_address": "新竹市東區建中一路52號1樓"}}
)

func TestMerchantToolsManualOrder(t *testing.T) {
	t.Run("FEAT-MANUAL-DISCOUNT", mdManualContract)
	e, a, cvsCode := mtOrderEnv(t)
	sku := e.p.stock.skus[0].ID
	var homeKey, cvsKey string
	t.Run("options list bank transfer and pay at pickup, never card", func(t *testing.T) {
		for _, o := range a.options() {
			for _, mode := range o.PaymentModes {
				if mode == "card" {
					t.Fatalf("a merchant cannot take a card: %+v", o)
				}
			}
			if o.DeliveryKind == "home" && strings.Contains(strings.Join(o.PaymentModes, ","), "bank_transfer") {
				homeKey = o.OptionKey
			}
			if o.DeliveryKind == "cvs_711" && o.DeliveryCode == cvsCode {
				cvsKey = o.OptionKey
				if o.PickupSelection == nil || *o.PickupSelection != "buyer_entered" || len(o.PaymentModes) != 2 || o.PickupMaxMinor != 2_000_000 {
					t.Fatalf("cvs option: %+v", o)
				}
			}
		}
		if homeKey == "" || cvsKey == "" {
			t.Fatalf("missing options: home=%q cvs=%q in %+v", homeKey, cvsKey, a.options())
		}
	})

	reservedBefore := func() int64 { _, r, _ := e.cofBalance(sku); return r }
	var order1, token1 string
	var total1 int64
	key1 := t04Key("mto-home")
	t.Run("home delivery with bank transfer: source, audit, reservation, link", func(t *testing.T) {
		before := reservedBefore()
		code, out := a.place(key1, mtBody(sku, 2, homeKey, "bank_transfer", mtHome))
		if code == 503 {
			e.mtDiagnose()
		}
		if code != 201 || out["commercial_state"] != "AWAITING_TRANSFER" || out["payment_mode"] != "bank_transfer" || out["source"] != "merchant_manual" {
			t.Fatalf("place: %d %v", code, out)
		}
		order1, _ = out["order_id"].(string)
		if exp, _ := out["expires_at"].(string); !strings.HasSuffix(exp, "Z") {
			t.Fatalf("expires_at must be RFC 3339 UTC (the admin parser requires Z): %q", exp)
		}
		total1 = int64(out["total_minor"].(float64))
		var source, state string
		var dbTotal int64
		var email *string
		if err := e.p.f.owner.QueryRow(context.Background(), `SELECT source,commercial_state,total_minor,buyer_email FROM checkout.orders WHERE id=$1`, order1).Scan(&source, &state, &dbTotal, &email); err != nil {
			t.Fatal(err)
		}
		if source != "merchant_manual" || state != "AWAITING_TRANSFER" || dbTotal != total1 || total1 != 2*1250 || email == nil || *email != "buyer@example.test" {
			t.Fatalf("order row: source=%s state=%s total=%d/%d email=%v", source, state, dbTotal, total1, email)
		}
		if got := reservedBefore(); got != before+2 {
			t.Fatalf("stock reserved %d -> %d, want +2 through begin_hold", before, got)
		}
		if n := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='order.manual_created'`, e.store()); n != 1 {
			t.Fatalf("audit rows: %d", n)
		}
		link, _ := out["buyer_link"].(string)
		linkOrder, linkToken := e.mtLinkParts(link, "zh-TW")
		if out["link_state"] != "configured" || linkOrder != order1 {
			t.Fatalf("link: %v %v", out["link_state"], link)
		}
		// The link token is NOT a capability: used as a bearer it opens nothing; exchanged once it yields a full capability of THIS order.
		if bare := e.bh.request(t, "GET", "/v1/buyer/orders/"+order1+"/bank-transfer", linkToken, "", nil, nil); bare.status == 200 {
			t.Fatalf("the link token must not work as a bearer: %d", bare.status)
		}
		exchanged, capability := e.mtRedeem(order1, linkToken)
		if exchanged.status != 200 || !strings.Contains(string(exchanged.body), order1) {
			t.Fatalf("redeem: %d %s", exchanged.status, exchanged.body)
		}
		token1 = capability
		view := e.bh.request(t, "GET", "/v1/buyer/orders/"+order1+"/bank-transfer", token1, "", nil, nil)
		if view.status != 200 || !strings.Contains(string(view.body), order1) {
			t.Fatalf("buyer view with the link capability: %d %s", view.status, view.body)
		}
		other := e.bh.request(t, "GET", "/v1/buyer/orders/"+order1+"/bank-transfer", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", "", nil, nil)
		if other.status == 200 {
			t.Fatalf("another capability must not read the order: %d", other.status)
		}
	})

	t.Run("replay: same key and body is the same order, other body is idempotency_conflict", func(t *testing.T) {
		before := reservedBefore()
		code, out := a.place(key1, mtBody(sku, 2, homeKey, "bank_transfer", mtHome))
		if exp, _ := out["expires_at"].(string); code != 200 || out["order_id"] != order1 || out["buyer_link"] == nil || !strings.HasSuffix(exp, "Z") {
			t.Fatalf("replay: %d %v", code, out)
		}
		if got := reservedBefore(); got != before {
			t.Fatalf("a replay reserved stock again: %d -> %d", before, got)
		}
		if n := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='order.manual_created'`, e.store()); n != 1 {
			t.Fatalf("a replay wrote an audit row: %d", n)
		}
		if code, out = a.place(key1, mtBody(sku, 3, homeKey, "bank_transfer", mtHome)); code != 409 || out["code"] != "idempotency_conflict" {
			t.Fatalf("other body: %d %v", code, out)
		}
	})

	t.Run("refused fractional pickup quote explains total and cap without reserving stock", func(t *testing.T) {
		before := reservedBefore()
		code, out := a.place(t04Key("mto-pap-fraction"), mtBody(sku, 1, cvsKey, "pay_at_pickup", mtCVS))
		details, ok := out["details"].(map[string]any)
		if code != 422 || out["code"] != "pay_at_pickup_amount_exceeds" || !ok || len(details) != 4 ||
			details["subtotal_minor"] != float64(1250) || details["shipping_minor"] != float64(0) ||
			details["total_minor"] != float64(1250) || details["max_minor"] != float64(2_000_000) || reservedBefore() != before {
			t.Fatalf("refused pickup quote: status %d, code %v, details %v", code, out["code"], details)
		}
	})

	var order2 string
	t.Run("CVS store entered by the merchant: pay at pickup is confirmed at placement, bank transfer waits", func(t *testing.T) {
		code, out := a.place(t04Key("mto-pap"), mtBody(sku, 2, cvsKey, "pay_at_pickup", mtCVS)) // begin_hold: pay at pickup needs a whole-TWD total (2 x 12.50)
		if code != 201 || out["commercial_state"] != "CONFIRMED" || out["payment_mode"] != "pay_at_pickup" {
			t.Fatalf("pay at pickup: %d %v", code, out)
		}
		order2, _ = out["order_id"].(string)
		var kind, source string
		if err := e.p.f.owner.QueryRow(context.Background(), `SELECT snapshot#>>'{destination,pickup,verification_kind}',source FROM checkout.orders WHERE id=$1`, order2).Scan(&kind, &source); err != nil {
			t.Fatal(err)
		}
		if kind != "BUYER_ENTERED" || source != "merchant_manual" {
			t.Fatalf("pickup kind %s source %s", kind, source)
		}
		if code, out = a.place(t04Key("mto-cvsbank"), mtBody(sku, 1, cvsKey, "bank_transfer", mtCVS)); code != 201 || out["commercial_state"] != "AWAITING_TRANSFER" {
			t.Fatalf("cvs bank transfer: %d %v", code, out)
		}
	})

	t.Run("refusals: card, unavailable mode, wrong destination kind, unknown SKU, insufficient stock, any price field, bad body", func(t *testing.T) {
		before := e.count(`SELECT count(*) FROM checkout.orders WHERE store_id=$1`, e.store())
		cases := []struct {
			name string
			body map[string]any
			want int
			code string
		}{
			{"card", mtBody(sku, 1, homeKey, "card", mtHome), 422, "invalid_request"},
			{"pay at pickup on a home option", mtBody(sku, 1, homeKey, "pay_at_pickup", mtHome), 422, "pay_at_pickup_unavailable"},
			{"cvs fields on a home option", mtBody(sku, 1, homeKey, "bank_transfer", mtCVS), 422, "invalid_request"},
			{"unknown sku", mtBody(randomUUID(), 1, homeKey, "bank_transfer", mtHome), 422, "invalid_request"},
			{"more than the stock", mtBody(sku, 1000, homeKey, "bank_transfer", mtHome), 409, "insufficient_inventory"},
			{"unknown option", mtBody(sku, 1, randomUUID()+"|TW|nope", "bank_transfer", mtHome), 422, "invalid_request"},
		}
		for _, c := range cases {
			if code, out := a.place(t04Key("mto-bad"), c.body); code != c.want || out["code"] != c.code {
				t.Errorf("%s: %d %v, want %d %s", c.name, code, out, c.want, c.code)
			}
		}
		priced := mtBody(sku, 1, homeKey, "bank_transfer", mtHome)
		priced["total_minor"] = 1
		if code, _ := a.place(t04Key("mto-price"), priced); code != 400 && code != 422 {
			t.Errorf("a price field must be refused: %d", code)
		}
		priced = mtBody(sku, 1, homeKey, "bank_transfer", mtHome)
		priced["items"].([]map[string]any)[0]["unit_price_minor"] = 1
		if code, _ := a.place(t04Key("mto-price2"), priced); code != 400 && code != 422 {
			t.Errorf("a per-line price must be refused: %d", code)
		}
		if w := a.call("POST", "/orders/manual", "", "application/json", []byte(`{}`)); w.Code != 422 {
			t.Errorf("no Idempotency-Key: %d", w.Code)
		}
		if after := e.count(`SELECT count(*) FROM checkout.orders WHERE store_id=$1`, e.store()); after != before {
			t.Errorf("a refused request created an order: %d -> %d", before, after)
		}
	})

	t.Run("authority: inventory:reserve only; scope from the bearer", func(t *testing.T) {
		reader, _ := e.member("orders:read", "catalog:read")
		if w := a.with(reader).call("GET", "/orders/manual/options", "", "", nil); w.Code != 403 {
			t.Errorf("options without inventory:reserve: %d", w.Code)
		}
		if code, out := a.with(reader).place(t04Key("mto-forbid"), mtBody(sku, 1, homeKey, "bank_transfer", mtHome)); code != 403 {
			t.Errorf("place without inventory:reserve: %d %v", code, out)
		}
		if w := a.with("").call("GET", "/orders/manual/options", "", "", nil); w.Code != 401 {
			t.Errorf("no bearer: %d", w.Code)
		}
		reserveOnly, _ := e.member("inventory:reserve") // a fulfilment-like role: stock yes, catalog no
		if w := a.with(reserveOnly).call("GET", "/orders/manual/options", "", "", nil); w.Code != 403 {
			t.Errorf("manual orders also need catalog:read: %d", w.Code)
		}
		if code, out := a.with(reserveOnly).place(t04Key("mto-nocat"), mtBody(sku, 1, homeKey, "bank_transfer", mtHome)); code != 403 {
			t.Errorf("place with inventory:reserve but no catalog:read: %d %v", code, out)
		}
		reserver, _ := e.member("inventory:reserve", "catalog:read")
		if w := a.with(reserver).call("GET", "/orders/manual/options", "", "", nil); w.Code != 200 {
			t.Errorf("an inventory:reserve + catalog:read member may list options: %d", w.Code)
		}
		other := a
		other.store = randomUUID()
		if w := other.call("GET", "/orders/manual/options", "", "", nil); w.Code != 404 && w.Code != 403 {
			t.Errorf("unknown store: %d", w.Code)
		}
	})

	t.Run("the unwired deployment answers 503 manual_order_unavailable", func(t *testing.T) {
		off := mtAdmin{t: t, h: httpapi.NewHandler(e.p.f.runtime), store: e.store(), token: e.token()}
		w := off.call("GET", "/orders/manual/options", "", "", nil)
		if w.Code != 503 || !strings.Contains(w.Body.String(), "manual_order_unavailable") {
			t.Errorf("unwired options: %d %s", w.Code, w.Body.String())
		}
	})
	_ = order2
	_ = token1
	_ = total1
}

// mtDiagnose prints (never fails) what stands behind a 503: whether the order exists and what the source-marking definer answers.
func (e *tcvEnv) mtDiagnose() {
	e.t.Helper()
	ctx := context.Background()
	var order, source string
	err := e.p.f.owner.QueryRow(ctx, `SELECT id::text,source FROM checkout.orders WHERE store_id=$1 ORDER BY created_at DESC LIMIT 1`, e.store()).Scan(&order, &source)
	e.t.Logf("diagnose: latest order=%s source=%s err=%v", order, source, err)
	if err != nil {
		return
	}
	hash := sha256.Sum256([]byte(e.token()))
	tx, err := e.p.f.runtime.Begin(ctx)
	if err != nil {
		e.t.Logf("diagnose: begin: %v", err)
		return
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `SELECT fulfillment.mark_order_merchant_manual($1,$2::uuid,$3::uuid)`, hash[:], e.store(), order)
	e.t.Logf("diagnose: mark_order_merchant_manual err=%v sqlstate=%s", err, sqlState(err))
}

func (e *tcvEnv) mtDashboard(a mtAdmin) map[string]any {
	e.t.Helper()
	w := a.call("GET", "/dashboard", "", "", nil)
	if w.Code != 200 {
		e.t.Fatalf("dashboard: %d %s", w.Code, w.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		e.t.Fatal(err)
	}
	return out
}

func mtNum(t *testing.T, m map[string]any, path ...string) float64 {
	t.Helper()
	var cur any = m
	for _, p := range path {
		next, ok := cur.(map[string]any)
		if !ok {
			t.Fatalf("path %v: %T at %q", path, cur, p)
		}
		cur = next[p]
	}
	n, ok := cur.(float64)
	if !ok {
		t.Fatalf("path %v is %T", path, cur)
	}
	return n
}

func TestMerchantToolsDashboard(t *testing.T) {
	e, a, cvsCode := mtOrderEnv(t)
	_ = cvsCode
	sku := e.p.stock.skus[0].ID
	var homeKey, cvsKey string
	for _, o := range a.options() {
		if o.DeliveryKind == "home" && strings.Contains(strings.Join(o.PaymentModes, ","), "bank_transfer") {
			homeKey = o.OptionKey
		}
		if o.DeliveryKind == "cvs_711" {
			cvsKey = o.OptionKey
		}
	}
	// The harness store already holds one DRAFT hold (a card order that never started payment): it is listed but is not a placed order.
	empty := e.mtDashboard(a)
	if mtNum(t, empty, "orders", "today") != 0 || mtNum(t, empty, "todos", "to_ship") != 0 || len(empty["latest_orders"].([]any)) != 1 || len(empty["gmv"].([]any)) != 0 || empty["timezone"] != "Asia/Taipei" {
		t.Fatalf("store with only a draft hold: %v", empty)
	}

	code1, out1 := a.place(t04Key("mtd-1"), mtBody(sku, 2, homeKey, "bank_transfer", mtHome))
	if code1 != 201 {
		t.Fatalf("fixture order 1: %d %v", code1, out1)
	}
	order1, _ := out1["order_id"].(string)
	total1 := int64(out1["total_minor"].(float64))
	_, linkToken1 := e.mtLinkParts(out1["buyer_link"].(string), "zh-TW")
	redeemed, token1 := e.mtRedeem(out1["order_id"].(string), linkToken1)
	if redeemed.status != 200 {
		t.Fatalf("redeem: %d %s", redeemed.status, redeemed.body)
	}
	_, out2 := a.place(t04Key("mtd-2"), mtBody(sku, 2, cvsKey, "pay_at_pickup", mtCVS))
	_, out3 := a.place(t04Key("mtd-3"), mtBody(sku, 1, cvsKey, "bank_transfer", mtCVS))
	if out1["order_id"] == nil || out2["order_id"] == nil || out3["order_id"] == nil {
		t.Fatalf("fixture orders: %v %v %v", out1, out2, out3)
	}
	audit := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1`, e.store())

	d := e.mtDashboard(a)
	if n := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1`, e.store()); n != audit {
		t.Errorf("the dashboard is read-only: audit rows %d -> %d", audit, n)
	}
	if mtNum(t, d, "orders", "today") != 3 || mtNum(t, d, "orders", "last_7_days") != 3 {
		t.Errorf("placed orders: %v", d["orders"])
	}
	if mtNum(t, d, "todos", "awaiting_transfer_confirmation") != 0 || mtNum(t, d, "todos", "to_ship") != 1 || mtNum(t, d, "todos", "cvs_awaiting_label") != 1 ||
		mtNum(t, d, "todos", "low_stock_skus") != 0 || mtNum(t, d, "todos", "open_refunds") != 0 {
		t.Errorf("todos before any transfer proof: %v", d["todos"])
	}
	latest := d["latest_orders"].([]any)
	if len(latest) != 4 || latest[0].(map[string]any)["order_id"] != out3["order_id"] || latest[2].(map[string]any)["order_id"] != order1 || latest[3].(map[string]any)["commercial_state"] != "DRAFT" {
		t.Errorf("latest orders must be newest first: %v", latest)
	}

	// The buyer (capability from the link) submits the transfer proof; the merchant sees it as a to-do, then confirms.
	proof := e.bh.request(t, "PUT", "/v1/buyer/orders/"+order1+"/bank-transfer/proof", token1, t04Key("mtd-proof"),
		map[string]any{"last5": "12345", "amount_minor": total1, "paid_at": time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)}, nil)
	if proof.status != 200 {
		t.Fatalf("proof: %d %s", proof.status, proof.body)
	}
	if d = e.mtDashboard(a); mtNum(t, d, "todos", "awaiting_transfer_confirmation") != 1 {
		t.Errorf("a submitted proof is a to-do: %v", d["todos"])
	}
	if st, res := e.cofDecide(e.token(), order1, "confirm", t04Key("mtd-confirm"), `{}`); st != 200 {
		t.Fatalf("confirm: %d %v", st, res)
	}
	d = e.mtDashboard(a)
	if mtNum(t, d, "todos", "awaiting_transfer_confirmation") != 0 || mtNum(t, d, "todos", "to_ship") != 2 || mtNum(t, d, "todos", "cvs_awaiting_label") != 1 {
		t.Errorf("after the confirmation: %v", d["todos"])
	}
	gmv := d["gmv"].([]any)
	if len(gmv) != 1 {
		t.Fatalf("gmv entries: %v", gmv)
	}
	entry := gmv[0].(map[string]any)
	if entry["currency"] != "TWD" || mtNum(t, entry, "today", "bank_transfer_minor") != float64(total1) || mtNum(t, entry, "last_7_days", "bank_transfer_minor") != float64(total1) ||
		mtNum(t, entry, "today", "card_minor") != 0 || mtNum(t, entry, "today", "pay_at_pickup_minor") != 0 {
		t.Errorf("gmv: %v", entry)
	}
	// MT01 reuse: the dashboard money IS the finance route's money (identity.read_finance_summary), column for column.
	today := time.Now().In(time.FixedZone("TPE", 8*3600))
	from, to := today.AddDate(0, 0, -6).Format("2006-01-02"), today.Format("2006-01-02")
	_, fin, raw := e.mcall(e.token(), "GET", "/v1/admin/stores/"+e.store()+"/finance/summary?from="+from+"&to="+to, "", "")
	totals, _ := fin["totals"].([]any)
	if len(totals) != 1 {
		t.Fatalf("finance totals: %s", raw)
	}
	ft := totals[0].(map[string]any)
	if ft["bank_transfer_confirmed_minor"] != entry["last_7_days"].(map[string]any)["bank_transfer_minor"] || ft["captured_minor"] != entry["last_7_days"].(map[string]any)["card_minor"] ||
		ft["pickup_collected_minor"] != entry["last_7_days"].(map[string]any)["pay_at_pickup_minor"] {
		t.Errorf("dashboard GMV differs from finance: %v vs %v", entry["last_7_days"], ft)
	}
	if n := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1`, e.store()); n != audit+1 { // only the merchant's confirm audits; every dashboard read added none
		t.Errorf("audit rows %d -> %d, want +1 (the confirm) and none from the dashboard reads", audit, n)
	}

	t.Run("low stock counts available <= 5 (boundary 5 low, 6 not)", func(t *testing.T) {
		on, reserved, allocated := e.cofBalance(sku)
		var unavailable int64
		if err := e.p.f.owner.QueryRow(context.Background(), `SELECT unavailable FROM inventory.balances WHERE tenant_id=$1 AND store_id=$2 AND sku_id=$3`, e.tenant(), e.store(), sku).Scan(&unavailable); err != nil {
			t.Fatal(err)
		}
		setAvailable := func(target int64) {
			var version int64
			if err := e.p.f.owner.QueryRow(context.Background(), `SELECT version FROM inventory.balances WHERE tenant_id=$1 AND store_id=$2 AND sku_id=$3`, e.tenant(), e.store(), sku).Scan(&version); err != nil {
				t.Fatal(err)
			}
			if _, err := t04Scoped(context.Background(), e.p.f, e.p.f.tokens["a"], e.store(), "inventory:write", func(tx pgx.Tx, s platform.Scope) (inventory.Balance, error) {
				return inventory.AdjustOnHand(context.Background(), tx, s, t04Key("mtd-adj"), inventory.Adjustment{WarehouseID: e.p.stock.warehouse.ID, SKUID: sku, Delta: target - (on - reserved - allocated - unavailable), ExpectedVersion: version, Reason: "mt low stock"})
			}); err != nil {
				t.Fatal(err)
			}
			on, reserved, allocated = e.cofBalance(sku)
		}
		setAvailable(6)
		if d := e.mtDashboard(a); mtNum(t, d, "todos", "low_stock_skus") != 0 {
			t.Errorf("6 available is not low: %v", d["todos"])
		}
		setAvailable(5)
		if d := e.mtDashboard(a); mtNum(t, d, "todos", "low_stock_skus") != 1 {
			t.Errorf("5 available is low: %v", d["todos"])
		}
	})

	t.Run("authority", func(t *testing.T) {
		noOrders, _ := e.member("catalog:read")
		if w := a.with(noOrders).call("GET", "/dashboard", "", "", nil); w.Code != 403 {
			t.Errorf("dashboard without orders:read: %d", w.Code)
		}
		if w := a.with("").call("GET", "/dashboard", "", "", nil); w.Code != 401 {
			t.Errorf("no bearer: %d", w.Code)
		}
		if w := a.call("GET", "/dashboard?store_id="+randomUUID(), "", "", nil); w.Code != 422 {
			t.Errorf("a query string is refused: %d", w.Code)
		}
		other := a
		other.store = randomUUID()
		if w := other.call("GET", "/dashboard", "", "", nil); w.Code != 404 && w.Code != 403 {
			t.Errorf("another store: %d", w.Code)
		}
		reader, _ := e.member("orders:read")
		if w := a.with(reader).call("GET", "/dashboard", "", "", nil); w.Code != 200 {
			t.Errorf("orders:read alone is enough: %d", w.Code)
		}
	})
}

func TestMerchantToolsDashboardScale(t *testing.T) {
	e, a, _ := mtOrderEnv(t)
	sku := e.p.stock.skus[0].ID
	var homeKey string
	for _, o := range a.options() {
		if o.DeliveryKind == "home" {
			homeKey = o.OptionKey
		}
	}
	if code, out := a.place(t04Key("mts-1"), mtBody(sku, 1, homeKey, "bank_transfer", mtHome)); code != 201 {
		t.Fatalf("seed order: %d %v", code, out)
	}
	ctx := context.Background()
	// 10,000 copies of the real order row, spread over 10 days and mixed states; FK checks off for this fixture only. A realistic store: the
	// CONFIRMED rows are almost all shipped (only g%100=0, 100 rows, are an unshipped backlog): to_ship costs ~0.35 ms per candidate
	// (manual_shipment_eligible, the order list's own predicate), so a 2,000-order backlog would cost ~0.7 s (measured in run 7).
	tx, err := e.p.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SET LOCAL session_replication_role=replica`); err != nil {
		t.Skipf("NOT_RUN: the owner role cannot disable FK triggers (%v)", err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO checkout.orders(tenant_id,store_id,owner_id,id,creator_session_id,cart_id,cart_version,quote_id,destination_id,market_id,country,service_code,
			service_version,allocation_version,currency,total_minor,commercial_state,fulfillment_state,generation,expires_at,job_id,snapshot,created_at,updated_at,payment_mode,collection_state)
		SELECT o.tenant_id,o.store_id,o.owner_id,gen_random_uuid(),o.creator_session_id,o.cart_id,1000+g,o.quote_id,o.destination_id,o.market_id,o.country,o.service_code,
			o.service_version,o.allocation_version,o.currency,o.total_minor,
			CASE g%5 WHEN 0 THEN 'CONFIRMED' WHEN 1 THEN 'CANCELLED' ELSE 'AWAITING_TRANSFER' END,
			CASE WHEN g%5=1 THEN 'CANCELLED' WHEN g%5=0 AND g%100<>0 THEN 'MERCHANT_SHIPPED' ELSE 'MANUAL_UNASSIGNED' END,1,
			CASE WHEN g%5=0 THEN clock_timestamp()-interval '20 days'-(g%240)*interval '1 hour'+interval '10 minutes' ELSE clock_timestamp()-(g%240)*interval '1 hour'+interval '10 minutes' END,
			900000000+g,o.snapshot,
			CASE WHEN g%5=0 THEN clock_timestamp()-interval '20 days'-(g%240)*interval '1 hour' ELSE clock_timestamp()-(g%240)*interval '1 hour' END,
			CASE WHEN g%5=0 THEN clock_timestamp()-interval '20 days'-(g%240)*interval '1 hour' ELSE clock_timestamp()-(g%240)*interval '1 hour' END,o.payment_mode,o.collection_state
		FROM checkout.orders o, generate_series(1,10000) g WHERE o.id=(SELECT id FROM checkout.orders WHERE store_id=$1 LIMIT 1)`, e.store()); err != nil {
		t.Skipf("NOT_RUN: bulk fixture insert refused (%v)", err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	// Cleanup also VACUUMs: the 10k copied rows carry the order snapshot (TOAST), and a plain DELETE keeps those pages, which pushed
	// the full G07 suite past the 256 MiB tmpfs data directory (release gate b843c8c: "No space left on device" in the next file).
	defer func() {
		mustExec(t, e.p.f.owner, `DELETE FROM checkout.orders WHERE store_id=$1 AND job_id>=900000000`, e.store())
		mustExec(t, e.p.f.owner, `VACUUM checkout.orders`)
	}()
	_ = e.mtDashboard(a) // warm
	var worst time.Duration
	for i := 0; i < 5; i++ {
		start := time.Now()
		w := a.call("GET", "/dashboard", "", "", nil)
		took := time.Since(start)
		if w.Code != 200 {
			// Diagnose: the same read in process prints the real cause (never fails by itself).
			err := platform.WithScope(ctx, e.p.f.runtime, e.token(), e.store(), "orders:read", func(tx pgx.Tx, s platform.Scope) error {
				_, inner := merchanttools.Dashboard(ctx, tx, s, e.token())
				return inner
			})
			t.Fatalf("dashboard on 10k orders: %d %s (in-process: %v)", w.Code, w.Body.String(), err)
		}
		if took > worst {
			worst = took
		}
	}
	// Per-block timing (the same calls the dashboard makes, one scoped transaction): where the milliseconds go.
	{
		hash := sha256.Sum256([]byte(e.token()))
		_ = platform.WithScope(ctx, e.p.f.runtime, e.token(), e.store(), "orders:read", func(tx pgx.Tx, s platform.Scope) error {
			timeit := func(label string, fn func() error) {
				start := time.Now()
				err := fn()
				t.Logf("MTO03 block %-18s %8s err=%v", label, time.Since(start).Round(time.Millisecond), err)
			}
			var raw []byte
			timeit("dashboard_orders", func() error {
				return tx.QueryRow(ctx, `SELECT identity.dashboard_orders($1,$2::uuid)`, hash[:], e.store()).Scan(&raw)
			})
			timeit("dashboard_todos", func() error {
				return tx.QueryRow(ctx, `SELECT identity.dashboard_todos($1,$2::uuid)`, hash[:], e.store()).Scan(&raw)
			})
			timeit("finance 7 days", func() error {
				_, err := reporting.Finance(ctx, tx, s, e.token(), time.Now().AddDate(0, 0, -6).Format("2006-01-02"), time.Now().Format("2006-01-02"))
				return err
			})
			timeit("latest 10 orders", func() error {
				_, err := merchantorders.List(ctx, tx, s, e.token(), merchantorders.ListRequest{Page: pagination.Request{Limit: 10}, State: "all"})
				return err
			})
			return nil
		})
	}
	t.Logf("MTO03 dashboard on %d orders (store total): worst of 5 = %s", e.count(`SELECT count(*) FROM checkout.orders WHERE store_id=$1`, e.store()), worst)
	if worst > 300*time.Millisecond {
		t.Errorf("dashboard worst %s exceeds the 300 ms budget", worst)
	}
}

// The only merchant setup here is creating a product and stocking the warehouse returned by onboarding.
func onboardManualCVS(t *testing.T) *tcvEnv {
	t.Helper()
	ctx := context.Background()
	f := *fixture(t)
	ident, _, _ := identityFixtureOn(t, &f)
	session := identityLogin(t, ident)
	store, err := ident.CreateInitialStore(ctx, session.Token, "fresh-default-cvs", firstStoreRequest())
	if err != nil {
		t.Fatal(err)
	}
	f.tenantA, f.storeA1, f.principalA = store.TenantID, store.StoreID, session.PrincipalID
	f.tokens = map[string]string{"a": session.Token}
	h := bcHarness{cqHarness: cqHarness{f: &f, a: openBuyerTestPools(t, &f)}}
	h.stock.warehouse.ID = store.WarehouseID
	h.stock.product, err = t04Scoped(ctx, &f, session.Token, store.StoreID, "catalog:write", func(tx pgx.Tx, s platform.Scope) (catalog.Product, error) {
		return catalog.CreateProduct(ctx, tx, s, t04Key("cvs-product"), catalog.ProductInput{Name: "Fixture product", Status: catalog.StatusActive})
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = f.owner.QueryRow(ctx, `SELECT id::text FROM pricing.markets WHERE tenant_id=$1 AND store_id=$2 AND code='tw'`, store.TenantID, store.StoreID).Scan(&h.market.ID); err != nil {
		t.Fatal(err)
	}
	h.pool, err = platform.OpenCheckoutPool(ctx, bcRole(t, &f, "commerce_checkout_runtime"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.pool.Close)
	h.service = bcService(t, h.pool).WithoutCardPayment()
	h.cqHarness.service, err = buyer.New(h.a.issuer, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	h.delivery.Code = "quickstart_cvs_familymart" // the shared cart fixture prepares a quote before the buyer chooses a brand
	e := tcvNew(t, tcvOpts{onboarded: &h})
	e.p.stock.skus = []catalog.SKU{{ID: e.sku(1250, 100)}}
	return e
}

func TestDefaultManualCVSOnboardAndOrders(t *testing.T) {
	e := onboardManualCVS(t)
	seed := func() {
		mustExec(t, e.p.f.owner, `SELECT fulfillment.seed_manual_cvs($1,$2,$3)`, e.tenant(), e.store(), e.p.f.principalA)
	}
	manual := mtManualOrders(t, e)
	a := mtAdmin{t: t, h: httpapi.NewHandler(e.p.f.runtime, httpapi.Options{CVS: e.cvs, ManualOrders: manual}), store: e.store(), token: e.token()}
	if n := len(a.options()); n != 4 {
		t.Fatalf("fresh merchant payable pickup options = %d; want 4", n)
	}
	check := func(kind, shop string, fee int64) {
		t.Helper()
		b := e.newBuyer()
		code := "quickstart_" + kind
		o := tceOptions(t, b)[kind]
		modes, ok := o["payment_modes"].([]any)
		if !ok || len(modes) != 1 || modes[0] != "pay_at_pickup" || o["delivery_code"] != code || o["pickup_selection"] != "buyer_entered" {
			t.Fatalf("buyer option %s: %v", kind, o)
		}
		store := b.enteredStore(code, shop, "", "")
		if store.status != 201 {
			t.Fatalf("pickup %s: %d %s", kind, store.status, store.body)
		}
		dest, err := b.destination(kind, tcvStr(tcvJSON(t, store.body), "pickup_id"), "王小明", "0912345678")
		if err != nil {
			t.Fatal(err)
		}
		q, err := b.quote(code)
		if err != nil || q.Amount.ShippingMinor != fee {
			t.Fatalf("buyer shipping %s: %d %v", kind, q.Amount.ShippingMinor, err)
		}
		r, err := b.begin(dest, q, int64(o["service_version"].(float64)), "pay_at_pickup")
		if err != nil || r.OrderID == "" {
			t.Fatalf("buyer begin %s: %v", kind, err)
		}
		for _, option := range a.options() {
			if option.DeliveryCode != code {
				continue
			}
			if option.PickupMaxMinor != 2000000 {
				t.Fatalf("default collection guard: %d", option.PickupMaxMinor)
			}
			key := t04Key("default-cvs-place")
			location := tcvJSON(t, store.body)
			body := mtBody(e.p.stock.skus[0].ID, 2, option.OptionKey, "pay_at_pickup", map[string]any{"cvs": map[string]any{"store_code": shop, "store_name": location["name"], "store_address": location["address"]}})
			status, receipt := a.place(key, body)
			if status != 201 || receipt["total_minor"] != float64(q.Amount.TotalMinor) {
				t.Fatalf("merchant place %s: %d %v", kind, status, receipt)
			}
			replayed, again := a.place(key, body)
			if replayed != 200 || again["order_id"] != receipt["order_id"] {
				t.Fatalf("manual replay: %d %v", replayed, again)
			}
			if n := e.count(`SELECT count(*) FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2 AND id IN ($3,$4)
			 AND payment_mode='pay_at_pickup' AND collection_state='PENDING' AND total_minor=$5`, e.tenant(), e.store(), r.OrderID, receipt["order_id"], q.Amount.TotalMinor); n != 2 {
				t.Fatalf("orders pending collection at quoted amount: %d", n)
			}
			if kind == "cvs_711" && fee == 0 {
				status, _, raw := e.mcall(e.token(), "PUT", "/v1/admin/stores/"+e.store()+"/orders/"+tcvStr(receipt, "order_id")+"/shipment", t04Key("default-cvs-ship"), mfxShip(0, "seven_eleven_cvs", "0012345678"))
				if status != 200 {
					t.Fatalf("manual shipment: %d %s", status, raw)
				}
			}
			return
		}
		t.Fatalf("merchant option missing: %s", kind)
	}
	for _, x := range []struct{ kind, shop string }{{"cvs_711", "131386"}, {"cvs_familymart", "011344"}, {"cvs_hilife", "F959"}, {"cvs_okmart", "000005"}} {
		check(x.kind, x.shop, 0)
	}
	// A rejected basket never creates an order; the maximum is a safety limit, not the collectible amount.
	before := e.count(`SELECT count(*) FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store())
	big := e.sku(2000100, 1)
	for _, opt := range a.options() {
		if opt.DeliveryKind != "cvs_711" {
			continue
		}
		status, out := a.place(t04Key("over-default-cap"), mtBody(big, 1, opt.OptionKey, "pay_at_pickup", mtCVS))
		if status != 422 || out["code"] != "pay_at_pickup_amount_exceeds" {
			t.Fatalf("over default guard: %d %v", status, out)
		}
	}
	if after := e.count(`SELECT count(*) FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store()); after != before {
		t.Fatalf("refused basket created order")
	}
	code := "quickstart_cvs_711"
	path := "/v1/admin/stores/" + e.store() + "/markets/" + e.p.market.ID + "/countries/TW/delivery-services/" + code
	fee := int64(6000)
	zero := int64(0)
	p := pricing.PolicyInput{MarketID: e.p.market.ID, Country: "TW", Currency: "TWD", ShippingMode: "country_flat", TaxMode: "inclusive", TaxBasis: "goods", TaxRateBPS: &zero, QuoteTTLSeconds: 300, Enabled: true, ConfigurationRef: "merchant fee edit"}
	p.ExpectedVersion, p.Method, p.ShippingMinor = 1, "delivery:"+code, &fee
	write := func(path string, v any) {
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		status, _, reply := e.mcall(e.token(), "PUT", path, t04Key("default-cvs-edit"), string(raw))
		if status != 200 {
			t.Fatalf("settings %s: %d %s", path, status, reply)
		}
	}
	write(path+"/policy", p)
	in := fulfillment.ServiceInput{MarketID: e.p.market.ID, Country: "TW", Code: code, NameHans: "7-ELEVEN", NameHant: "7-ELEVEN", NameEN: "7-ELEVEN", DeliveryKind: "cvs_711", Mode: "MANUAL", Enabled: true, Visible: true, SortOrder: 1, ExpectedVersion: 1, PolicyVersion: 2}
	write(path, in)
	seed()
	check("cvs_711", "131386", fee)
	in.ExpectedVersion, in.Enabled = 2, false
	write(path, in)
	seed()
	if it := tceOptions(t, e.newBuyer())["cvs_711"]; it != nil {
		t.Fatalf("reopened disabled service: %+v", it)
	}
	for _, o := range a.options() {
		if o.DeliveryKind == "cvs_711" {
			t.Fatalf("merchant reopened service: %+v", o)
		}
	}
	for _, table := range []string{"checkout.payment_attempts", "integration.merchant_accounts", "fulfillment.cvs_shipments"} {
		if n := e.count("SELECT count(*) FROM "+table+" WHERE tenant_id=$1 AND store_id=$2", e.tenant(), e.store()); n != 0 {
			t.Fatalf("unexpected %s rows: %d", table, n)
		}
	}
}

func TestDefaultManualCVSExistingCollectionSettings(t *testing.T) {
	e := tcvNew(t)
	e.grantCreator("orders:read")
	seed := func() {
		mustExec(t, e.p.f.owner, `SELECT fulfillment.seed_manual_cvs($1,$2,$3)`, e.tenant(), e.store(), e.p.f.principalA)
	}
	options := func() []merchanttools.ManualOption {
		out, err := mtManualOrders(t, e).Options(context.Background(), e.token(), e.store())
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	// The store was already operating before 0227 with a deliberately low cap and one chain.
	e.cvsSettings(`["cvs_711"]`, true, "19", 2)
	seed()
	found := options()
	if len(found) != 1 || found[0].DeliveryKind != "cvs_711" || found[0].PickupMaxMinor != 1900 {
		t.Fatalf("overrode saved low cap or chains: %+v", found)
	}
	e.cvsSettings(tcvAllChains, false, "null", 2)
	seed()
	if found = options(); len(found) != 0 {
		t.Fatalf("overrode explicit opt-out: %+v", found)
	}
}

func TestDefaultManualCVSAmbiguousMarketTax(t *testing.T) {
	// The harness market already has a current untaxed home policy. A brand's
	// saved policy is authoritative even when the market has no common tax.
	e := tcvNew(t)
	p := e.p.policy
	fee, rate := int64(6500), int64(500)
	p.Method, p.ShippingMinor = "delivery:quickstart_cvs_familymart", &fee
	p.TaxMode, p.TaxBasis, p.TaxRateBPS = "exclusive", "goods_and_shipping", &rate
	saved, err := e.p.setPolicy(p)
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, e.p.f.owner, `SELECT fulfillment.seed_manual_cvs($1,$2,$3)`, e.tenant(), e.store(), e.p.f.principalA)
	if n := e.count(`SELECT count(*) FROM pricing.policy_heads
		WHERE tenant_id=$1 AND store_id=$2 AND market_id=$3 AND country='TW'
		AND method IN ('delivery:quickstart_cvs_711','delivery:quickstart_cvs_hilife','delivery:quickstart_cvs_okmart')`, e.tenant(), e.store(), e.p.market.ID); n != 0 {
		t.Fatalf("ambiguous market created %d unconfigured brand policies", n)
	}
	b := e.newBuyer()
	rows := tceOptions(t, b)
	if rows["cvs_familymart"]["delivery_code"] != "quickstart_cvs_familymart" || rows["cvs_711"] != nil || rows["cvs_hilife"] != nil || rows["cvs_okmart"] != nil {
		t.Fatalf("ambiguous market buyer options: %v", rows)
	}
	q, err := b.quote("quickstart_cvs_familymart")
	if err != nil || q.Policy.Version != saved.Version || q.Policy.TaxMode != p.TaxMode || q.Policy.TaxBasis != p.TaxBasis ||
		q.Policy.TaxRateBPS != rate || q.Amount.ShippingMinor != fee || q.Amount.ShippingTaxMinor != 325 {
		t.Fatalf("dedicated policy changed: policy=%+v amount=%+v err=%v", q.Policy, q.Amount, err)
	}
}
