// Purpose: FEAT-MANUAL-DISCOUNT HTTP/REAL_PG regression contract, batched in existing CI shard roots.
// Depends on: merchant-tools and live-console harnesses, real buyer checkout, command receipts and reporting.
// Used by: TestMerchantToolsManualOrder and TestLiveConsoleOrderForBuyerLivePrice in GitHub CI only.
// Invariants: server-computed money, scoped authority, immutable named adjustment, exact replay and paid refund cap.
// Fixtures: synthetic buyer/address; stock and settings use existing public seams. No provider is contacted.
package foundation_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"image"
	_ "image/jpeg"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/buyer"
	"livecommerce/internal/checkout"
	"livecommerce/internal/command"
	"livecommerce/internal/merchanttools"
	"livecommerce/internal/platform"
	"livecommerce/internal/pricing"
	"livecommerce/internal/reporting"
	"livecommerce/internal/storefront"
)

func mdBody(sku string, qty int, option, mode string, delivery map[string]any, amount int64) map[string]any {
	b := mtBody(sku, qty, option, mode, delivery)
	b["customer"] = map[string]any{"name": "測試買家", "phone": "0900000000", "email": "buyer@example.test"}
	b["discount"] = map[string]any{"name": "直播優惠", "amount_minor": amount}
	return b
}

var mdHome = map[string]any{"home_address": map[string]any{"region": "測試區", "city": "測試市", "postal_code": "100", "line1": "測試地址一號", "line2": ""}}
var mdCVS = map[string]any{"cvs": map[string]any{"store_code": "110035", "store_name": "一勝", "store_address": "台中市大肚區福山村沙田路一段432-1號"}}

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

// mdPartialErrorContract exercises an unfinished real HTTP pipeline, not a seeded error or a revoked caller.
// Each leaf owns its environment; both merchants share the existing handler and its capability issuer/secret.
func mdPartialErrorContract(t *testing.T, changedCart bool) {
	t.Helper()
	e, a, _ := mtOrderEnv(t)
	tokenA, principalA := e.member("inventory:reserve", "catalog:read")
	tokenB, principalB := e.member("inventory:reserve", "catalog:read")
	if principalA == principalB {
		t.Fatal("fixture must provide two different merchant principals")
	}
	a = a.with(tokenA)
	b := a.with(tokenB)
	sku := e.p.stock.skus[0].ID
	ctx := context.Background()
	readTuples := func(protectedOnly bool) []byte {
		t.Helper()
		state := map[string]json.RawMessage{}
		// Closed table list; owner reads witness full tuples, including authorization created_at/hash/quote binding.
		tables := []string{"buyer.owners", "buyer.capability_sessions", "storefront.manual_order_discounts", "buyer.command_results", "storefront.carts", "storefront.cart_lines", "storefront.quotes", "ops.command_results", "inventory.reservations", "inventory.reservation_lines", "inventory.balances", "checkout.orders", "ops.audit_events"}
		if protectedOnly {
			tables = []string{"storefront.quotes", "checkout.orders", "inventory.reservations", "inventory.reservation_lines", "inventory.balances", "inventory.ledger"}
		}
		for _, table := range tables {
			var rows []byte
			if err := e.p.f.owner.QueryRow(ctx, "SELECT coalesce(jsonb_agg(to_jsonb(w) ORDER BY to_jsonb(w)::text),'[]'::jsonb) FROM "+table+" w WHERE tenant_id=$1 AND store_id=$2", e.tenant(), e.store()).Scan(&rows); err != nil {
				t.Fatal(err)
			}
			state[table] = rows
		}
		out, err := json.Marshal(state)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	var home string
	for _, option := range a.options() {
		if option.DeliveryKind == "home" && strings.Contains(strings.Join(option.PaymentModes, ","), "bank_transfer") {
			home = option.OptionKey
		}
	}
	if home == "" {
		t.Fatal("bank-transfer home option missing")
	}
	// Positive authority control: B can use the same route before any refused write.
	if w := b.call("GET", "/orders/manual/options", "", "", nil); w.Code != 200 {
		t.Fatalf("second merchant options authority: HTTP %d", w.Code)
	}
	protectedBefore := readTuples(true)
	_, heldBefore, allocatedBefore := e.cofBalance(sku)
	t.Log("pre-POST six-table fixture baseline captured")
	body := mdBody(sku, 2, home, "bank_transfer", mdHome, 2501)
	key := t04Key("md-partial")
	if code, out := a.place(key, body); code != 422 || out["code"] != "discount_exceeds_subtotal" {
		t.Fatalf("real 2500 subtotal must reject discount 2501: HTTP %d code=%v", code, out["code"])
	}
	// The fixture is already normalized. Encode its typed wire shape independently of ValidateManual.
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	var normalized merchanttools.ManualInput
	if err := json.Unmarshal(raw, &normalized); err != nil {
		t.Fatal(err)
	}
	raw, err = json.Marshal(normalized)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(raw)
	var owner string
	if err := e.p.f.owner.QueryRow(ctx, `SELECT owner_id::text FROM storefront.manual_order_discounts
 WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND request_hash=$4
 AND name='直播優惠' AND amount_minor=2501 AND quote_id IS NULL`, e.tenant(), e.store(), principalA, hash[:]).Scan(&owner); err != nil {
		t.Fatal("partial pipeline must retain A's normalized authorization with no quote: ", err)
	}
	if e.count(`SELECT count(*) FROM storefront.manual_order_discounts WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store()) != 1 ||
		e.count(`SELECT count(*) FROM buyer.command_results WHERE tenant_id=$1 AND store_id=$2 AND owner_id=$3 AND operation='cart.set'`, e.tenant(), e.store(), owner) != 1 ||
		e.count(`SELECT count(*) FROM storefront.cart_lines WHERE tenant_id=$1 AND store_id=$2 AND owner_id=$3 AND sku_id=$4 AND quantity=2`, e.tenant(), e.store(), owner, sku) != 1 {
		t.Fatal("partial pipeline must have exactly one authorization and the original owner's quantity-2 cart receipt")
	}
	if !bytes.Equal(protectedBefore, readTuples(true)) {
		t.Fatal("initial rejection changed protected quote/order/hold/stock/ledger fixture tuples")
	}
	if e.count(`SELECT count(*) FROM storefront.quotes WHERE tenant_id=$1 AND store_id=$2 AND owner_id=$3`, e.tenant(), e.store(), owner) != 0 ||
		e.count(`SELECT count(*) FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2 AND owner_id=$3`, e.tenant(), e.store(), owner) != 0 ||
		e.count(`SELECT count(*) FROM inventory.reservations WHERE tenant_id=$1 AND store_id=$2 AND buyer_owner_id=$3`, e.tenant(), e.store(), owner) != 0 {
		t.Fatal("initial rejection wrote a quote, order or reservation for the manual owner")
	}
	t.Log("initial rejection preserved all six fixture tables and the manual owner has no quote/order/hold")
	if e.count(`SELECT count(*) FROM ops.command_results WHERE tenant_id=$1 AND store_id=$2 AND operation='merchanttools.order.manual'`, e.tenant(), e.store()) != 0 ||
		e.count(`SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action='order.manual_created'`, e.tenant(), e.store()) != 0 {
		t.Fatal("partial rejection completed a merchant receipt or order audit")
	}
	if _, held, allocated := e.cofBalance(sku); held != heldBefore || allocated != allocatedBefore {
		t.Fatal("partial rejection reserved or allocated stock")
	}
	t.Log("initial partial pipeline state verified")
	readState := func() []byte { return readTuples(false) }
	caller, retryBody, wantCode := b, body, "conflict"
	if changedCart {
		caller, retryBody, wantCode = a, mdBody(sku, 3, home, "bank_transfer", mdHome, 100), "idempotency_conflict"
	}
	before := readState()
	if !changedCart {
		var probeTx pgx.Tx
		targetEntered := false
		err := platform.WithScope(ctx, e.p.f.runtime, tokenB, e.store(), "inventory:reserve", func(tx pgx.Tx, scope platform.Scope) error {
			probeTx = tx
			if scope.TenantID != e.tenant() || scope.StoreID != e.store() || scope.PrincipalID != principalB {
				return errors.New("principal-conflict witness opened the wrong merchant scope")
			}
			if err := platform.RequirePermission(ctx, tx, scope, tokenB, "catalog:read"); err != nil {
				return err
			}
			targetEntered = true
			_, err := tx.Exec(ctx, `SELECT fulfillment.authorize_manual_order_discount($1,$2::uuid,$3::uuid,$4,$5::text,$6::bigint)`, tokenHash(tokenB), e.store(), owner, hash[:], "直播優惠", int64(2501))
			return err
		})
		// sqlState extracts an actual *pgconn.PgError with errors.As; no message matching or synthetic error.
		if !targetEntered || sqlState(err) != "PT409" {
			t.Fatalf("runtime principal-conflict witness: target_entered=%t SQLSTATE=%s", targetEntered, sqlState(err))
		}
		if !errors.Is(probeTx.Rollback(ctx), pgx.ErrTxClosed) || !bytes.Equal(before, readState()) {
			t.Fatal("principal-conflict witness must leave a closed rolled-back transaction and unchanged state")
		}
		t.Log("real runtime B reached authorization target, received PT409 and rolled back without state changes")
	}
	code, out := caller.place(key, retryBody)
	if !bytes.Equal(before, readState()) {
		t.Fatal("refused resume changed authorization/cart/quote/receipt/hold/order/audit tuples")
	}
	t.Log("refused resume retained all witnessed tuples")
	if replayCode, replay := a.place(key, body); replayCode != 422 || replay["code"] != "discount_exceeds_subtotal" {
		t.Fatalf("original body must still reach the original discount refusal: HTTP %d code=%v", replayCode, replay["code"])
	}
	if !bytes.Equal(before, readState()) {
		t.Fatal("original-body replay changed partial pipeline state")
	}
	t.Log("original-body replay retained the original partial state")
	// Run the successful write before the exact error assertion, so expected RED cannot skip this control.
	if freshCode, fresh := caller.place(t04Key("md-partial-fresh"), mdBody(sku, 3, home, "bank_transfer", mdHome, 100)); freshCode != 201 || fresh["total_minor"] != float64(3650) {
		t.Fatalf("fresh key positive control: HTTP %d code=%v total=%v", freshCode, fresh["code"], fresh["total_minor"])
	}
	t.Log("fresh-key same-handler authority and valid-discount positive control reached HTTP 201")
	if code != 409 || out["code"] != wantCode {
		t.Fatalf("unfinished pipeline error contract: HTTP %d code=%v, want 409 %s", code, out["code"], wantCode)
	}
}

func mdManualContract(t *testing.T) {
	t.Run("partial authorization principal conflict", func(t *testing.T) { mdPartialErrorContract(t, false) })
	t.Run("partial changed cart and discount idempotency conflict", func(t *testing.T) { mdPartialErrorContract(t, true) })
	t.Run("runtime database boundary", mdDBBoundaryContract)
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

// mdDBBoundaryContract tests the granted database boundary, not an HTTP injection path.
// Owner-pool writes below only create isolated synthetic authorization/quote fixtures;
// no grant, trigger, existing authorization or existing frozen snapshot is changed.
// Every attempted authorization INSERT/bind uses the actual restricted f.runtime login.
func mdDBBoundaryContract(t *testing.T) {
	e, a, _ := mtOrderEnv(t)
	ctx := context.Background()
	f := e.p.f
	var home, deliveryCode string
	for _, o := range a.options() {
		if o.DeliveryKind == "home" && strings.Contains(strings.Join(o.PaymentModes, ","), "bank_transfer") {
			home, deliveryCode = o.OptionKey, o.DeliveryCode
			break
		}
	}
	if home == "" {
		t.Fatal("DB boundary fixture missing home/bank-transfer option")
	}
	var sameLogin, runtimeRole, privileged bool
	if err := f.runtime.QueryRow(ctx, `SELECT current_user=session_user,
  pg_has_role(current_user,'commerce_runtime','USAGE'),
  r.rolsuper OR r.rolbypassrls OR pg_has_role(current_user,'commerce_auth','USAGE')
  FROM pg_roles r WHERE r.rolname=current_user`).Scan(&sameLogin, &runtimeRole, &privileged); err != nil {
		t.Fatal(err)
	}
	if !sameLogin || !runtimeRole || privileged {
		t.Fatal("DB boundary must run as the genuine unprivileged commerce_runtime login")
	}

	// This existing route is the positive control for the real merchant pipeline.
	if status, out := a.place(t04Key("md-db-control"), mdBody(e.p.stock.skus[0].ID, 2, home, "bank_transfer", mdHome, 100)); status != 201 || out["total_minor"] != float64(2400) {
		t.Fatalf("DB boundary HTTP positive control: status=%d total=%v", status, out["total_minor"])
	}

	// A callback-entry witness prevents an invalid session/scope from passing a negative.
	scoped := func(t *testing.T, token, principal, permission string, fn func(pgx.Tx, platform.Scope) error) error {
		t.Helper()
		reached := false
		err := platform.WithScope(ctx, f.runtime, token, e.store(), permission, func(tx pgx.Tx, s platform.Scope) error {
			if s.TenantID != e.tenant() || s.StoreID != e.store() || s.PrincipalID != principal {
				t.Fatal("unexpected server-resolved runtime scope")
			}
			reached = true
			return fn(tx, s)
		})
		if !reached {
			t.Fatalf("DB target never reached; scope setup failed: SQLSTATE=%s", sqlState(err))
		}
		return err
	}
	quote := func(t *testing.T, b *tcvBuyer, manual bool) storefront.Quote {
		t.Helper()
		q, err := cqBuyer(b.h.a.runtime, b.cap, func(c context.Context, tx pgx.Tx, s buyer.Scope) (storefront.Quote, error) {
			in := storefront.QuoteInput{CartVersion: b.cartVersion(), MarketID: e.p.market.ID, Country: "TW", Method: "delivery:" + deliveryCode}
			if manual {
				return storefront.CreateManualQuote(c, tx, s, t04Key("md-db-quote"), in)
			}
			return storefront.CreateQuote(c, tx, s, t04Key("md-db-quote"), in)
		})
		if err != nil || q.ID == "" || q.Amount.SubtotalMinor != 2500 {
			t.Fatalf("real same-owner quote setup failed: SQLSTATE=%s subtotal=%d", sqlState(err), q.Amount.SubtotalMinor)
		}
		return q
	}
	seed := func(t *testing.T, owner, principal string, d *pricing.ManualDiscount) {
		t.Helper()
		var name, amount any
		if d != nil {
			name, amount = d.Name, d.AmountMinor
		}
		hash := sha256.Sum256([]byte("synthetic DB-boundary authorization " + owner))
		mustExec(t, f.owner, `INSERT INTO storefront.manual_order_discounts
  (tenant_id,store_id,owner_id,principal_id,request_hash,name,amount_minor)
  VALUES($1,$2,$3,$4,$5,$6,$7)`, e.tenant(), e.store(), owner, principal, hash[:], name, amount)
	}
	readAuth := func(t *testing.T, owner string) []byte {
		t.Helper()
		var raw []byte
		if err := f.owner.QueryRow(ctx, `SELECT to_jsonb(d) FROM storefront.manual_order_discounts d
  WHERE tenant_id=$1 AND store_id=$2 AND owner_id=$3`, e.tenant(), e.store(), owner).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		return raw
	}
	bind := func(t *testing.T, token, principal, id string) error {
		t.Helper()
		return scoped(t, token, principal, "inventory:reserve", func(tx pgx.Tx, s platform.Scope) error {
			if err := platform.RequirePermission(ctx, tx, s, token, "catalog:read"); err != nil {
				t.Fatalf("binder caller must have both permissions: SQLSTATE=%s", sqlState(err))
			}
			hash := sha256.Sum256([]byte(token))
			_, err := tx.Exec(ctx, `SELECT fulfillment.bind_manual_order_discount($1,$2::uuid,$3::uuid)`, hash[:], e.store(), id)
			return err
		})
	}

	controlledQuote := func(t *testing.T, b *tcvBuyer, d *pricing.ManualDiscount) storefront.Quote {
		t.Helper()
		owner := b.cap.Scope.OwnerID
		candidate := quote(t, b, false)
		if d == nil {
			return candidate
		}
		// Controlled DB-boundary fixture, not an HTTP-produced mismatch:
		// insert a NEW same-owner quote from the real cart/session/header.
		// Standard pricing supplies consistent line/tax/total allocations;
		// every FK/check/trigger remains enabled. Existing snapshots stay frozen.
		baseID := candidate.ID
		candidate.ID = randomUUID()
		candidate.Adjustments = []pricing.Adjustment{{Kind: "manual_discount", Currency: candidate.Currency, ManualDiscount: *d}}
		inputs := make([]pricing.AmountLine, len(candidate.Lines))
		for i, line := range candidate.Lines {
			inputs[i] = pricing.AmountLine{UnitPriceMinor: line.UnitPriceMinor, Quantity: line.Quantity}
		}
		var err error
		candidate.Amount, err = pricing.CalculateAdjusted(candidate.Policy, inputs, nil, candidate.Adjustments)
		if err != nil {
			t.Fatal("controlled adjusted quote pricing failed")
		}
		for i := range candidate.Lines {
			candidate.Lines[i].Amount = candidate.Amount.Lines[i]
		}
		raw, err := json.Marshal(candidate)
		if err != nil {
			t.Fatal(err)
		}
		mustExec(t, f.owner, `INSERT INTO storefront.quotes
  (tenant_id,store_id,owner_id,id,cart_id,creator_session_id,cart_version,market_id,market_version,country,method,policy_version,currency,created_at,expires_at,snapshot,creator_kind,create_attempt_id)
  SELECT tenant_id,store_id,owner_id,$4,cart_id,creator_session_id,cart_version,market_id,market_version,country,method,policy_version,currency,created_at,expires_at,$5,creator_kind,create_attempt_id
  FROM storefront.quotes WHERE tenant_id=$1 AND store_id=$2 AND owner_id=$3 AND id=$6`, e.tenant(), e.store(), owner, candidate.ID, raw, baseID)
		return candidate
	}

	// The two direct matrices use store:read only for scope entry. Target SQLSTATE
	// is captured separately, so neither a Go guard nor scope setup can satisfy it.
	t.Run("bind_authority", func(t *testing.T) {
		const targetSQL = `SELECT fulfillment.bind_manual_order_discount($1::bytea,$2::uuid,$3::uuid)`
		const signature = "fulfillment.bind_manual_order_discount(bytea,uuid,uuid)"
		var functionOID uint32
		if err := f.owner.QueryRow(ctx, `SELECT $1::regprocedure::oid`, signature).Scan(&functionOID); err != nil {
			t.Fatal(err)
		}
		d := &pricing.ManualDiscount{Name: "直播優惠", AmountMinor: 100}
		fixture := func(t *testing.T, principal string) (string, storefront.Quote, []byte, []byte) {
			t.Helper()
			b := e.newBuyer()
			owner := b.cap.Scope.OwnerID
			seed(t, owner, principal, d)
			q := quote(t, b, true) // Real CreateManualQuote, matching this authorization.
			var frozen []byte
			if err := f.owner.QueryRow(ctx, `SELECT snapshot FROM storefront.quotes WHERE tenant_id=$1 AND store_id=$2 AND owner_id=$3 AND id=$4`, e.tenant(), e.store(), owner, q.ID).Scan(&frozen); err != nil {
				t.Fatal(err)
			}
			var saved storefront.Quote
			if err := json.Unmarshal(frozen, &saved); err != nil || saved.ID != q.ID || saved.Currency != "TWD" || len(saved.Adjustments) != 1 || saved.Adjustments[0].Kind != "manual_discount" || saved.Adjustments[0].Currency != "TWD" || saved.Adjustments[0].Name != d.Name || saved.Adjustments[0].AmountMinor != d.AmountMinor {
				t.Fatal("binder authority fixture must have the exact real frozen adjustment")
			}
			hash := sha256.Sum256([]byte("synthetic DB-boundary authorization " + owner))
			if e.count(`SELECT count(*) FROM storefront.manual_order_discounts WHERE tenant_id=$1 AND store_id=$2 AND owner_id=$3 AND principal_id=$4 AND request_hash=$5 AND name=$6 AND amount_minor=$7 AND quote_id IS NULL`, e.tenant(), e.store(), owner, principal, hash[:], d.Name, d.AmountMinor) != 1 {
				t.Fatal("binder authority fixture must have the complete matching unbound tuple")
			}
			return owner, q, readAuth(t, owner), frozen
		}
		unchanged := func(t *testing.T, owner string, q storefront.Quote, before, frozen []byte) {
			t.Helper()
			var afterQuote []byte
			if err := f.owner.QueryRow(ctx, `SELECT snapshot FROM storefront.quotes WHERE tenant_id=$1 AND store_id=$2 AND owner_id=$3 AND id=$4`, e.tenant(), e.store(), owner, q.ID).Scan(&afterQuote); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, readAuth(t, owner)) || !bytes.Equal(frozen, afterQuote) || e.count(`SELECT count(*) FROM storefront.manual_order_discounts WHERE owner_id=$1 AND quote_id IS NULL`, owner) != 1 {
				t.Fatal("rejected binder must preserve the full authorization tuple, NULL binding and frozen quote bytes")
			}
		}
		positive := func(t *testing.T, token, principal string) {
			t.Helper()
			owner, q, before, frozen := fixture(t, principal)
			hash := sha256.Sum256([]byte(token))
			entered := false
			var targetErr error
			err := scoped(t, token, principal, "store:read", func(tx pgx.Tx, _ platform.Scope) error {
				entered = true
				_, targetErr = tx.Exec(ctx, targetSQL, hash[:], e.store(), q.ID)
				return targetErr
			})
			if !entered || err != nil || targetErr != nil {
				t.Fatalf("direct binder positive target: entered=%t SQLSTATE=%s wrapper=%s", entered, sqlState(targetErr), sqlState(err))
			}
			var old, saved map[string]any
			if json.Unmarshal(before, &old) != nil || json.Unmarshal(readAuth(t, owner), &saved) != nil {
				t.Fatal("binder positive tuple decode")
			}
			old["quote_id"] = q.ID
			want, _ := json.Marshal(old)
			got, _ := json.Marshal(saved)
			var afterQuote []byte
			if err := f.owner.QueryRow(ctx, `SELECT snapshot FROM storefront.quotes WHERE id=$1`, q.ID).Scan(&afterQuote); err != nil || !bytes.Equal(want, got) || !bytes.Equal(frozen, afterQuote) {
				t.Fatal("binder positive must change only the exact quote binding")
			}
			t.Log("bind_authority positive target entered and exact tuple bound")
		}
		for _, revoke := range []bool{false, true} {
			name := "locked_no_revoke"
			if revoke {
				name = "locked_catalog_revoked"
			}
			t.Run(name, func(t *testing.T) {
				token, principal := e.member("inventory:reserve", "catalog:read")
				owner, q, before, frozen := fixture(t, principal)
				lock, err := f.owner.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer lock.Rollback(ctx)
				var holderPID int
				if err = lock.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&holderPID); err != nil {
					t.Fatal(err)
				}
				var one int
				if err = lock.QueryRow(ctx, `SELECT 1 FROM storefront.manual_order_discounts WHERE tenant_id=$1 AND store_id=$2 AND owner_id=$3 FOR UPDATE`, e.tenant(), e.store(), owner).Scan(&one); err != nil {
					t.Fatal(err)
				}
				runCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				defer cancel()
				pids := make(chan int, 1)
				type result struct{ target, wrapper error }
				done := make(chan result, 1)
				go func() {
					var targetErr error
					wrapper := platform.WithScope(runCtx, f.runtime, token, e.store(), "store:read", func(tx pgx.Tx, s platform.Scope) error {
						var pid int
						if err := tx.QueryRow(runCtx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
							return err
						}
						pids <- pid
						hash := sha256.Sum256([]byte(token))
						_, targetErr = tx.Exec(runCtx, targetSQL, hash[:], e.store(), q.ID)
						return targetErr
					})
					done <- result{targetErr, wrapper}
				}()
				joined := false
				defer func() {
					cancel()
					_ = lock.Rollback(context.Background())
					if !joined {
						select {
						case <-done:
						case <-time.After(time.Second):
							t.Error("HARNESS_FAIL binder did not stop")
						}
					}
				}()
				var pid int
				select {
				case pid = <-pids:
				case r := <-done:
					joined = true
					t.Fatalf("HARNESS_FAIL scope before binder: %v", r.wrapper)
				case <-runCtx.Done():
					t.Fatal("HARNESS_FAIL no binder pid")
				}
				deadline := time.Now().Add(800 * time.Millisecond)
				blocked := false
				for time.Now().Before(deadline) {
					if err = f.owner.QueryRow(runCtx, `SELECT $2=ANY(pg_blocking_pids($1))`, pid, holderPID).Scan(&blocked); err != nil {
						t.Fatalf("HARNESS_FAIL blocker check: %v", err)
					}
					if blocked {
						break
					}
					time.Sleep(5 * time.Millisecond)
				}
				if !blocked {
					t.Fatal("HARNESS_FAIL binder never blocked on holder")
				}
				if revoke {
					tag, err := f.owner.Exec(runCtx, `DELETE FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND permission='catalog:read'`, e.tenant(), e.store(), principal)
					if err != nil || tag.RowsAffected() != 1 {
						t.Fatalf("HARNESS_FAIL revoke: %v rows=%d", err, tag.RowsAffected())
					}
				}
				if err = lock.Commit(ctx); err != nil {
					t.Fatalf("HARNESS_FAIL release: %v", err)
				}
				var got result
				select {
				case got = <-done:
					joined = true
				case <-runCtx.Done():
					t.Fatal("HARNESS_FAIL binder did not finish")
				}
				if revoke {
					if sqlState(got.target) != "PT403" {
						t.Fatalf("binder after revoked grant SQLSTATE=%s wrapper=%v, want PT403", sqlState(got.target), got.wrapper)
					}
					unchanged(t, owner, q, before, frozen)
				} else if got.target != nil || got.wrapper != nil || e.count(`SELECT count(*) FROM storefront.manual_order_discounts WHERE owner_id=$1 AND quote_id=$2`, owner, q.ID) != 1 {
					t.Fatalf("binder without revocation: target=%v wrapper=%v", got.target, got.wrapper)
				}
			})
		}
		for _, tc := range []struct {
			name, guc string
			perms     []string
		}{
			{"both permissions", "", []string{"inventory:reserve", "catalog:read"}},
			{"missing catalog read", "", []string{"inventory:reserve"}},
			{"missing inventory reserve", "", []string{"catalog:read"}},
			{"neither permission", "", nil},
			{"tenant GUC mismatch", "app.tenant_id", []string{"inventory:reserve", "catalog:read"}},
			{"store GUC mismatch", "app.store_id", []string{"inventory:reserve", "catalog:read"}},
			{"principal GUC mismatch", "app.principal_id", []string{"inventory:reserve", "catalog:read"}},
			{"mixed buyer GUC", "app.buyer_id", []string{"inventory:reserve", "catalog:read"}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				token, principal := e.member(tc.perms...)
				if len(tc.perms) == 2 {
					positive(t, token, principal)
				} else {
					controlToken, controlPrincipal := e.member("inventory:reserve", "catalog:read")
					positive(t, controlToken, controlPrincipal) // Same constructor, both permissions, matching principal.
				}
				if tc.name == "both permissions" {
					return
				}
				owner, q, before, frozen := fixture(t, principal)
				_, otherPrincipal := e.member("inventory:reserve", "catalog:read")
				values := map[string]string{"app.tenant_id": f.tenantB, "app.store_id": f.storeA2, "app.principal_id": otherPrincipal, "app.buyer_id": owner}
				hash := sha256.Sum256([]byte(token))
				entered := false
				var targetErr error
				err := scoped(t, token, principal, "store:read", func(tx pgx.Tx, s platform.Scope) error {
					for _, permission := range []string{"inventory:reserve", "catalog:read"} {
						want := 0
						for _, granted := range tc.perms {
							if granted == permission {
								want = 1
							}
						}
						if e.count(`SELECT count(*) FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND permission=$4`, s.TenantID, s.StoreID, s.PrincipalID, permission) != want {
							t.Fatal("binder authority actual permission fixture differs")
						}
					}
					if tc.guc != "" {
						var saved string
						if err := tx.QueryRow(ctx, `SELECT set_config($1,$2,true)`, tc.guc, values[tc.guc]).Scan(&saved); err != nil || saved != values[tc.guc] {
							t.Fatal("binder GUC setup failed before target")
						}
					}
					entered = true
					_, targetErr = tx.Exec(ctx, targetSQL, hash[:], e.store(), q.ID)
					return targetErr
				})
				if !entered || sqlState(targetErr) != "PT403" || sqlState(err) != "PT403" {
					t.Fatalf("direct binder authority must deny: entered=%t target SQLSTATE=%s wrapper=%s", entered, sqlState(targetErr), sqlState(err))
				}
				unchanged(t, owner, q, before, frozen)
				t.Log("bind_authority negative target entered: PT403 and unchanged full tuple/quote")
			})
		}
		for _, role := range []string{"commerce_buyer_runtime", "commerce_checkout_runtime"} {
			t.Run(role+" EXECUTE denied", func(t *testing.T) {
				token, principal := e.member("inventory:reserve", "catalog:read")
				positive(t, token, principal)
				owner, q, before, frozen := fixture(t, principal)
				pool := e.p.a.runtime
				if role == "commerce_checkout_runtime" {
					pool = e.p.pool
				}
				tx, err := pool.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer tx.Rollback(ctx)
				mdRestrictedDefinerLogin(t, tx, role, "fulfillment", functionOID, true)
				hash := sha256.Sum256([]byte(token))
				_, targetErr := tx.Exec(ctx, targetSQL, hash[:], e.store(), q.ID)
				if err := tx.Rollback(ctx); err != nil {
					t.Fatal(err)
				}
				if sqlState(targetErr) != "42501" {
					t.Fatalf("actual role binder EXECUTE: SQLSTATE=%s", sqlState(targetErr))
				}
				unchanged(t, owner, q, before, frozen)
				t.Log("bind_authority actual-login target entered: 42501 and unchanged full tuple/quote")
			})
		}
	})

	t.Run("read_adjustments_authority", func(t *testing.T) {
		const targetSQL = `SELECT identity.read_order_adjustments($1::bytea,$2::uuid,$3::uuid)`
		const signature = "identity.read_order_adjustments(bytea,uuid,uuid)"
		var functionOID uint32
		if err := f.owner.QueryRow(ctx, `SELECT $1::regprocedure::oid`, signature).Scan(&functionOID); err != nil {
			t.Fatal(err)
		}
		place := func(t *testing.T, discounted bool) (string, []byte) {
			t.Helper()
			body := mdBody(e.p.stock.skus[0].ID, 2, home, "bank_transfer", mdHome, 100)
			if !discounted {
				delete(body, "discount")
			}
			status, out := a.place(t04Key("md-reader-order"), body)
			order, ok := out["order_id"].(string)
			wantTotal := float64(2500)
			if discounted {
				wantTotal = 2400
			}
			if status != 201 || !ok || order == "" || out["total_minor"] != wantTotal {
				t.Fatalf("reader real manual-order fixture: HTTP=%d total=%v", status, out["total_minor"])
			}
			var frozen []byte
			if err := f.owner.QueryRow(ctx, `SELECT coalesce(snapshot#>'{quote,adjustments}','[]'::jsonb) FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2 AND id=$3 AND source='merchant_manual'`, e.tenant(), e.store(), order).Scan(&frozen); err != nil {
				t.Fatal(err)
			}
			mdExactAdjustments(t, frozen, discounted)
			return order, frozen
		}
		positive := func(t *testing.T, token, principal, order string, discounted bool, frozen []byte) {
			t.Helper()
			hash := sha256.Sum256([]byte(token))
			entered := false
			var raw []byte
			var targetErr error
			err := scoped(t, token, principal, "store:read", func(tx pgx.Tx, _ platform.Scope) error {
				entered = true
				targetErr = tx.QueryRow(ctx, targetSQL, hash[:], e.store(), order).Scan(&raw)
				return targetErr
			})
			if !entered || err != nil || targetErr != nil || !bytes.Equal(frozen, raw) {
				t.Fatalf("direct reader positive target: entered=%t target SQLSTATE=%s wrapper=%s exact=%t", entered, sqlState(targetErr), sqlState(err), bytes.Equal(frozen, raw))
			}
			mdExactAdjustments(t, raw, discounted)
			t.Log("read_adjustments_authority positive target entered with exact frozen JSON")
		}
		for _, discounted := range []bool{true, false} {
			name := "exact named adjustment"
			if !discounted {
				name = "no discount empty array"
			}
			t.Run(name, func(t *testing.T) {
				token, principal := e.member("orders:read")
				order, frozen := place(t, discounted)
				positive(t, token, principal, order, discounted, frozen)
			})
		}
		for _, tc := range []struct {
			name, guc string
			missing   bool
		}{
			{"missing orders read", "", true},
			{"tenant GUC mismatch", "app.tenant_id", false},
			{"store GUC mismatch", "app.store_id", false},
			{"principal GUC mismatch", "app.principal_id", false},
			{"same tenant foreign store order", "", false},
			{"cross tenant order", "", false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				controlToken, controlPrincipal := e.member("orders:read")
				order, frozen := place(t, true)
				positive(t, controlToken, controlPrincipal, order, true, frozen)
				token, principal := controlToken, controlPrincipal
				if tc.missing {
					token, principal = e.member()
				}
				wantState := "PT403"
				if tc.name == "same tenant foreign store order" || tc.name == "cross tenant order" {
					foreignStore, foreignOrder := moBeginInOtherStore(t, f, tc.name == "same tenant foreign store order")
					var tenant, store string
					if err := f.owner.QueryRow(ctx, `SELECT tenant_id::text,store_id::text FROM checkout.orders WHERE id=$1`, foreignOrder).Scan(&tenant, &store); err != nil || store != foreignStore || store == e.store() || (tenant == e.tenant()) != (tc.name == "same tenant foreign store order") {
						t.Fatal("reader foreign order must be a real checkout order in the intended tenant/store")
					}
					order, wantState = foreignOrder, "PT404"
				}
				_, otherPrincipal := e.member("orders:read")
				values := map[string]string{"app.tenant_id": f.tenantB, "app.store_id": f.storeA2, "app.principal_id": otherPrincipal}
				hash := sha256.Sum256([]byte(token))
				entered := false
				var raw []byte
				var targetErr error
				err := scoped(t, token, principal, "store:read", func(tx pgx.Tx, s platform.Scope) error {
					want := 1
					if tc.missing {
						want = 0
					}
					if e.count(`SELECT count(*) FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND permission='orders:read'`, s.TenantID, s.StoreID, s.PrincipalID) != want {
						t.Fatal("reader authority actual permission fixture differs")
					}
					if tc.guc != "" {
						var saved string
						if err := tx.QueryRow(ctx, `SELECT set_config($1,$2,true)`, tc.guc, values[tc.guc]).Scan(&saved); err != nil || saved != values[tc.guc] {
							t.Fatal("reader GUC setup failed before target")
						}
					}
					entered = true
					targetErr = tx.QueryRow(ctx, targetSQL, hash[:], e.store(), order).Scan(&raw)
					return targetErr
				})
				if !entered || sqlState(targetErr) != wantState || sqlState(err) != wantState || len(raw) != 0 {
					t.Fatalf("direct reader authority must deny without data: entered=%t target SQLSTATE=%s wrapper=%s bytes=%d want=%s", entered, sqlState(targetErr), sqlState(err), len(raw), wantState)
				}
				t.Logf("read_adjustments_authority negative target entered: %s and no data", wantState)
			})
		}
		for _, role := range []string{"commerce_buyer_runtime", "commerce_checkout_runtime"} {
			t.Run(role+" schema-or-function access denied", func(t *testing.T) {
				token, principal := e.member("orders:read")
				order, frozen := place(t, true)
				positive(t, token, principal, order, true, frozen)
				pool := e.p.a.runtime
				if role == "commerce_checkout_runtime" {
					pool = e.p.pool
				}
				tx, err := pool.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer tx.Rollback(ctx)
				mdRestrictedDefinerLogin(t, tx, role, "identity", functionOID, false)
				hash := sha256.Sum256([]byte(token))
				var raw []byte
				targetErr := tx.QueryRow(ctx, targetSQL, hash[:], e.store(), order).Scan(&raw)
				if err := tx.Rollback(ctx); err != nil {
					t.Fatal(err)
				}
				if sqlState(targetErr) != "42501" || len(raw) != 0 {
					t.Fatalf("actual role reader schema-or-function access: SQLSTATE=%s bytes=%d", sqlState(targetErr), len(raw))
				}
				t.Log("read_adjustments_authority actual-login call attempted: schema-or-function 42501/no data; function EXECUTE independently false by OID")
			})
		}
		// The reader passes resolve_access + GUC checks, then waits on checkout.orders.
		// Revoking only orders:read meanwhile must reach the final fence:
		// identity.merchant_access_denied returns PT403 for a missing required grant
		// while store:read remains (migrations/0063_manual_fulfilment.sql:253-254).
		// An absent order without revocation is PT404 (migrations/0220_manual_order_discount.sql:161).
		for _, tc := range []struct {
			name             string
			existing, revoke bool
			want             string
		}{
			{"locked_absent_no_revoke", false, false, "PT404"},
			{"locked_absent_orders_revoked", false, true, "PT403"},
			{"locked_existing_orders_revoked", true, true, "PT403"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				token, principal := e.member("orders:read")
				order := randomUUID()
				if tc.existing {
					order, _ = place(t, true)
				}
				if n := e.count(`SELECT count(*) FROM checkout.orders WHERE id=$1`, order); (n == 1) != tc.existing {
					t.Fatalf("HARNESS_FAIL order existence=%d want existing=%t", n, tc.existing)
				}
				lock, err := f.owner.Begin(ctx)
				if err != nil {
					t.Fatalf("HARNESS_FAIL begin holder: %v", err)
				}
				defer lock.Rollback(ctx)
				var holderPID int
				if err = lock.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&holderPID); err != nil {
					t.Fatalf("HARNESS_FAIL holder pid: %v", err)
				}
				if _, err = lock.Exec(ctx, `LOCK TABLE checkout.orders IN ACCESS EXCLUSIVE MODE`); err != nil {
					t.Fatalf("HARNESS_FAIL lock orders: %v", err)
				}
				runCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				defer cancel()
				pids := make(chan int, 1)
				type result struct{ target, wrapper error }
				done := make(chan result, 1)
				go func() {
					var targetErr error
					wrapper := platform.WithScope(runCtx, f.runtime, token, e.store(), "store:read", func(tx pgx.Tx, s platform.Scope) error {
						var pid int
						if err := tx.QueryRow(runCtx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
							return err
						}
						pids <- pid
						hash := sha256.Sum256([]byte(token))
						var raw []byte
						targetErr = tx.QueryRow(runCtx, targetSQL, hash[:], e.store(), order).Scan(&raw)
						return targetErr
					})
					done <- result{targetErr, wrapper}
				}()
				joined := false
				defer func() {
					cancel()
					_ = lock.Rollback(context.Background())
					if !joined {
						select {
						case <-done:
						case <-time.After(time.Second):
							t.Error("HARNESS_FAIL reader did not stop")
						}
					}
				}()
				var pid int
				select {
				case pid = <-pids:
				case r := <-done:
					joined = true
					t.Fatalf("HARNESS_FAIL scope before reader: %v", r.wrapper)
				case <-runCtx.Done():
					t.Fatal("HARNESS_FAIL no reader pid")
				}
				deadline := time.Now().Add(800 * time.Millisecond)
				blocked := false
				for time.Now().Before(deadline) {
					if err = f.owner.QueryRow(runCtx, `SELECT $2=ANY(pg_blocking_pids($1)) AND EXISTS(SELECT 1 FROM pg_locks
  WHERE pid=$1 AND NOT granted AND locktype='relation' AND relation='checkout.orders'::regclass)`, pid, holderPID).Scan(&blocked); err != nil {
						t.Fatalf("HARNESS_FAIL blocker check: %v", err)
					}
					if blocked {
						break
					}
					time.Sleep(5 * time.Millisecond) // Poll interval for the witness, not synchronization.
				}
				if !blocked {
					t.Fatal("HARNESS_FAIL reader never waited on checkout.orders behind holder")
				}
				if tc.revoke {
					tag, err := f.owner.Exec(runCtx, `DELETE FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND permission='orders:read'`, e.tenant(), e.store(), principal)
					if err != nil || tag.RowsAffected() != 1 {
						t.Fatalf("HARNESS_FAIL revoke: %v rows=%d", err, tag.RowsAffected())
					}
				}
				if err = lock.Commit(ctx); err != nil {
					t.Fatalf("HARNESS_FAIL release: %v", err)
				}
				var got result
				select {
				case got = <-done:
					joined = true
				case <-runCtx.Done():
					t.Fatal("HARNESS_FAIL reader did not finish")
				}
				if sqlState(got.target) == "55P03" {
					t.Fatalf("HARNESS_FAIL reader lock_timeout before release: %v", got.target)
				}
				if sqlState(got.target) != tc.want {
					t.Fatalf("reader after initial auth (existing=%t revoked=%t) SQLSTATE=%s wrapper=%v, want %s", tc.existing, tc.revoke, sqlState(got.target), got.wrapper, tc.want)
				}
			})
		}
	})

	// Exercise the replacement SECURITY DEFINER itself: store:read opens the
	// callback even when either write permission is absent. No Go write guard
	// can supply the expected database error on behalf of the target function.
	t.Run("authorize", func(t *testing.T) {
		const authorizeSQL = `SELECT fulfillment.authorize_manual_order_discount($1::bytea,$2::uuid,$3::uuid,$4::bytea,$5::text,$6::bigint)`
		const authorizeSignature = "fulfillment.authorize_manual_order_discount(bytea,uuid,uuid,bytea,text,bigint)"
		for _, tc := range []struct {
			name  string
			perms []string
		}{
			{"neither permission", nil},
			{"missing inventory reserve", []string{"catalog:read"}},
			{"missing catalog read", []string{"inventory:reserve"}},
			{"both permissions persist unbound authorization", []string{"inventory:reserve", "catalog:read"}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				token, principal := e.member(tc.perms...)
				b := e.newBuyer() // Real owner FK; invalid setup cannot pass by SQLSTATE.
				owner := b.cap.Scope.OwnerID
				if e.count(`SELECT count(*) FROM storefront.manual_order_discounts WHERE owner_id=$1`, owner) != 0 {
					t.Fatal("authorize fixture must start empty")
				}
				tokenHash := sha256.Sum256([]byte(token))
				requestHash := sha256.Sum256([]byte("synthetic direct authorize " + owner))
				targetEntered := false
				err := scoped(t, token, principal, "store:read", func(tx pgx.Tx, s platform.Scope) error {
					for _, permission := range []string{"inventory:reserve", "catalog:read"} {
						want := false
						for _, granted := range tc.perms {
							want = want || granted == permission
						}
						// Owner readback is independent of the runtime permission helper.
						actual := e.count(`SELECT count(*) FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND permission=$4`, s.TenantID, s.StoreID, s.PrincipalID, permission)
						got := platform.RequirePermission(ctx, tx, s, token, permission)
						if (actual == 1) != want || actual > 1 || (want && got != nil) || (!want && !errors.Is(got, platform.ErrForbidden)) {
							t.Fatalf("authorize actual permission fixture differs: %s=%t rows=%d", permission, want, actual)
						}
					}
					targetEntered = true
					_, err := tx.Exec(ctx, authorizeSQL, tokenHash[:], e.store(), owner, requestHash[:], "直播優惠", int64(100))
					return err
				})
				if !targetEntered {
					t.Fatal("authorize function call never reached")
				}
				n := e.count(`SELECT count(*) FROM storefront.manual_order_discounts WHERE owner_id=$1`, owner)
				if len(tc.perms) != 2 {
					if sqlState(err) != "PT403" || n != 0 {
						t.Fatalf("direct authorize must deny missing permission without rows: SQLSTATE=%s rows=%d", sqlState(err), n)
					}
					return
				}
				if err != nil || n != 1 {
					t.Fatalf("direct authorize success control: SQLSTATE=%s rows=%d", sqlState(err), n)
				}
				var tenant, store, savedOwner, savedPrincipal, name string
				var savedHash []byte
				var amount int64
				var unbound bool
				if err := f.owner.QueryRow(ctx, `SELECT tenant_id::text,store_id::text,owner_id::text,principal_id::text,request_hash,name,amount_minor,quote_id IS NULL
  FROM storefront.manual_order_discounts WHERE owner_id=$1`, owner).Scan(&tenant, &store, &savedOwner, &savedPrincipal, &savedHash, &name, &amount, &unbound); err != nil {
					t.Fatal(err)
				}
				if tenant != e.tenant() || store != e.store() || savedOwner != owner || savedPrincipal != principal || !bytes.Equal(savedHash, requestHash[:]) || name != "直播優惠" || amount != 100 || !unbound {
					t.Fatal("authorize must persist exact derived scope/request/discount with NULL quote")
				}
			})
		}

		for _, tc := range []struct{ name, guc string }{
			{"tenant GUC mismatch", "app.tenant_id"},
			{"store GUC mismatch", "app.store_id"},
			{"principal GUC mismatch", "app.principal_id"},
			{"mixed buyer GUC", "app.buyer_id"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				token, principal := e.member("inventory:reserve", "catalog:read")
				_, otherPrincipal := e.member("inventory:reserve", "catalog:read")
				b := e.newBuyer()
				owner := b.cap.Scope.OwnerID
				values := map[string]string{"app.tenant_id": f.tenantB, "app.store_id": f.storeA2, "app.principal_id": otherPrincipal, "app.buyer_id": owner}
				tokenHash := sha256.Sum256([]byte(token))
				requestHash := sha256.Sum256([]byte("synthetic authorize GUC " + owner))
				if e.count(`SELECT count(*) FROM storefront.manual_order_discounts WHERE owner_id=$1`, owner) != 0 {
					t.Fatal("GUC fixture must start empty")
				}
				targetEntered := false
				err := scoped(t, token, principal, "store:read", func(tx pgx.Tx, s platform.Scope) error {
					for _, permission := range []string{"inventory:reserve", "catalog:read"} {
						if e.count(`SELECT count(*) FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND permission=$4`, s.TenantID, s.StoreID, s.PrincipalID, permission) != 1 || platform.RequirePermission(ctx, tx, s, token, permission) != nil {
							t.Fatal("GUC target requires both actual permissions")
						}
					}
					var saved string
					if err := tx.QueryRow(ctx, `SELECT set_config($1,$2,true)`, tc.guc, values[tc.guc]).Scan(&saved); err != nil || saved != values[tc.guc] {
						t.Fatal("GUC mismatch setup failed before target")
					}
					targetEntered = true
					_, err := tx.Exec(ctx, authorizeSQL, tokenHash[:], e.store(), owner, requestHash[:], "直播優惠", int64(100))
					return err
				})
				n := e.count(`SELECT count(*) FROM storefront.manual_order_discounts WHERE owner_id=$1`, owner)
				if !targetEntered || sqlState(err) != "PT403" || n != 0 {
					t.Fatalf("authorize must reject mismatched/mixed GUC without rows: reached=%t SQLSTATE=%s rows=%d", targetEntered, sqlState(err), n)
				}
			})
		}

		for _, role := range []string{"commerce_buyer_runtime", "commerce_checkout_runtime"} {
			t.Run(role+" EXECUTE denied", func(t *testing.T) {
				token, _ := e.member("inventory:reserve", "catalog:read")
				b := e.newBuyer()
				owner := b.cap.Scope.OwnerID
				// Borrow the genuine existing role pools, never SET ROLE or forged GUCs.
				pool := b.h.a.runtime
				if role == "commerce_checkout_runtime" {
					pool = e.p.pool
				}
				tx, err := pool.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer tx.Rollback(ctx)
				var sameLogin, intendedRole, privileged, schemaUsage, execute bool
				if err := tx.QueryRow(ctx, `SELECT current_user=session_user,pg_has_role(current_user,$1,'USAGE'),
  r.rolsuper OR r.rolbypassrls OR pg_has_role(current_user,'commerce_auth','USAGE') OR pg_has_role(current_user,'commerce_runtime','USAGE'),
  has_schema_privilege(current_user,'fulfillment','USAGE'),has_function_privilege(current_user,$2,'EXECUTE')
  FROM pg_roles r WHERE r.rolname=current_user`, role, authorizeSignature).Scan(&sameLogin, &intendedRole, &privileged, &schemaUsage, &execute); err != nil {
					t.Fatal(err)
				}
				if !sameLogin || !intendedRole || privileged || !schemaUsage || execute {
					t.Fatal("authorize ACL requires actual restricted role, schema USAGE and no function EXECUTE")
				}
				if e.count(`SELECT count(*) FROM storefront.manual_order_discounts WHERE owner_id=$1`, owner) != 0 {
					t.Fatal("ACL fixture must start empty")
				}
				tokenHash := sha256.Sum256([]byte(token))
				requestHash := sha256.Sum256([]byte("synthetic authorize ACL " + owner))
				_, err = tx.Exec(ctx, authorizeSQL, tokenHash[:], e.store(), owner, requestHash[:], "直播優惠", int64(100))
				if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
					t.Fatal(rollbackErr)
				}
				n := e.count(`SELECT count(*) FROM storefront.manual_order_discounts WHERE owner_id=$1`, owner)
				if sqlState(err) != "42501" || n != 0 {
					t.Fatalf("actual buyer/checkout authorize EXECUTE must fail without rows: SQLSTATE=%s rows=%d", sqlState(err), n)
				}
			})
		}
	})

	for _, tc := range []struct {
		name  string
		perms []string
	}{
		{"neither permission", nil},
		{"missing inventory reserve", []string{"catalog:read"}},
		{"missing catalog read", []string{"inventory:reserve"}},
		{"both permissions cannot preset binding", []string{"inventory:reserve", "catalog:read"}},
	} {
		for _, preset := range []bool{false, true} {
			if len(tc.perms) == 2 && !preset {
				continue
			}
			name := tc.name + "/unbound"
			if preset {
				name = tc.name + "/preset quote"
			}
			t.Run("direct INSERT/"+name, func(t *testing.T) {
				token, principal := e.member(tc.perms...)
				b := e.newBuyer()
				q := quote(t, b, false) // Real FK target, never an arbitrary UUID.
				owner := b.cap.Scope.OwnerID
				if n := e.count(`SELECT count(*) FROM storefront.manual_order_discounts WHERE owner_id=$1`, owner); n != 0 {
					t.Fatal("negative INSERT fixture must start with no authorization")
				}
				var quoteID any
				if preset {
					quoteID = q.ID
				}
				hash := sha256.Sum256([]byte("synthetic direct INSERT " + owner))
				err := scoped(t, token, principal, "store:read", func(tx pgx.Tx, s platform.Scope) error {
					for _, permission := range []string{"inventory:reserve", "catalog:read"} {
						want := false
						for _, granted := range tc.perms {
							want = want || granted == permission
						}
						got := platform.RequirePermission(ctx, tx, s, token, permission)
						if (want && got != nil) || (!want && !errors.Is(got, platform.ErrForbidden)) {
							t.Fatalf("permission fixture differs from intended %s=%t", permission, want)
						}
					}
					_, err := tx.Exec(ctx, `INSERT INTO storefront.manual_order_discounts
  (tenant_id,store_id,owner_id,principal_id,request_hash,name,amount_minor,quote_id)
  VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, s.TenantID, s.StoreID, owner, s.PrincipalID, hash[:], "直播優惠", int64(100), quoteID)
					return err
				})
				n := e.count(`SELECT count(*) FROM storefront.manual_order_discounts WHERE owner_id=$1`, owner)
				if err == nil || (sqlState(err) != "42501" && sqlState(err) != "PT403") || n != 0 {
					t.Fatalf("unauthorized or preset-binding runtime INSERT must be denied and leave zero rows: SQLSTATE=%s rows=%d preset=%t", sqlState(err), n, preset)
				}
			})
		}
	}

	aToken, aPrincipal := e.member("inventory:reserve", "catalog:read")
	bToken, bPrincipal := e.member("inventory:reserve", "catalog:read")
	if aPrincipal == bPrincipal {
		t.Fatal("binder requires distinct merchant principals")
	}
	discount := &pricing.ManualDiscount{Name: "直播優惠", AmountMinor: 100}
	for _, tc := range []struct {
		name           string
		auth           *pricing.ManualDiscount
		frozen         *pricing.ManualDiscount
		otherPrincipal bool
	}{
		{"another merchant principal", discount, discount, true},
		{"missing frozen adjustment", discount, nil, false},
		{"different frozen name", discount, &pricing.ManualDiscount{Name: "另一優惠", AmountMinor: 100}, false},
		{"different frozen amount", discount, &pricing.ManualDiscount{Name: "直播優惠", AmountMinor: 200}, false},
		{"no discount tombstone with adjusted quote", nil, discount, false},
	} {
		t.Run("bind/"+tc.name, func(t *testing.T) {
			// Independent valid control first: same principal, matching authorization,
			// same quote builder as the negative, including the no-discount tombstone.
			// The HTTP 201 control above independently covers genuine CreateManualQuote.
			control := e.newBuyer()
			seed(t, control.cap.Scope.OwnerID, aPrincipal, tc.auth)
			var valid storefront.Quote
			if tc.otherPrincipal {
				valid = quote(t, control, true)
			} else {
				valid = controlledQuote(t, control, tc.auth)
			}
			if err := bind(t, aToken, aPrincipal, valid.ID); err != nil {
				t.Fatalf("matching binder control refused: SQLSTATE=%s", sqlState(err))
			}
			if n := e.count(`SELECT count(*) FROM storefront.manual_order_discounts WHERE owner_id=$1 AND quote_id=$2`, control.cap.Scope.OwnerID, valid.ID); n != 1 {
				t.Fatal("matching control did not bind exactly one real quote")
			}

			b := e.newBuyer()
			owner := b.cap.Scope.OwnerID
			seed(t, owner, aPrincipal, tc.auth)
			var candidate storefront.Quote
			if tc.otherPrincipal {
				candidate = quote(t, b, true)
			} else {
				candidate = controlledQuote(t, b, tc.frozen)
			}

			var frozen []byte
			if err := f.owner.QueryRow(ctx, `SELECT snapshot FROM storefront.quotes WHERE tenant_id=$1 AND store_id=$2 AND owner_id=$3 AND id=$4`, e.tenant(), e.store(), owner, candidate.ID).Scan(&frozen); err != nil {
				t.Fatal(err)
			}
			var saved storefront.Quote
			if err := json.Unmarshal(frozen, &saved); err != nil || saved.ID != candidate.ID || saved.CartID != candidate.CartID || saved.Amount.DiscountMinor != candidate.Amount.DiscountMinor || saved.Amount.TotalMinor != candidate.Amount.TotalMinor || len(saved.Adjustments) != len(candidate.Adjustments) {
				t.Fatal("candidate quote fixture was not inserted for this owner")
			}
			for i, adjustment := range saved.Adjustments {
				if adjustment != candidate.Adjustments[i] {
					t.Fatal("candidate frozen adjustment readback differs from setup")
				}
			}
			before := readAuth(t, owner)
			if n := e.count(`SELECT count(*) FROM storefront.manual_order_discounts WHERE owner_id=$1 AND quote_id IS NULL`, owner); n != 1 {
				t.Fatal("negative binding fixture must start unbound")
			}
			token, principal := aToken, aPrincipal
			if tc.otherPrincipal {
				token, principal = bToken, bPrincipal
			}
			err := bind(t, token, principal, candidate.ID)
			after := readAuth(t, owner)
			var afterQuote []byte
			if readErr := f.owner.QueryRow(ctx, `SELECT snapshot FROM storefront.quotes WHERE tenant_id=$1 AND store_id=$2 AND owner_id=$3 AND id=$4`, e.tenant(), e.store(), owner, candidate.ID).Scan(&afterQuote); readErr != nil {
				t.Fatal(readErr)
			}
			if !bytes.Equal(frozen, afterQuote) {
				t.Fatal("binder changed the frozen quote snapshot")
			}
			if err == nil || (sqlState(err) != "PT403" && sqlState(err) != "PT409") || !bytes.Equal(before, after) {
				t.Fatalf("binder must refuse principal/adjustment mismatch without changing authorization: SQLSTATE=%s unchanged=%t", sqlState(err), bytes.Equal(before, after))
			}
		})
	}
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

// mdRestrictedDefinerLogin verifies a genuine login, including SET-reachable
// privileged membership; an owner transaction with SET ROLE cannot pass.
func mdRestrictedDefinerLogin(t *testing.T, tx pgx.Tx, role, schema string, functionOID uint32, wantUsage bool) {
	t.Helper()
	var sameLogin, intended, privileged, usage, execute bool
	if err := tx.QueryRow(context.Background(), `SELECT current_user=session_user,
 pg_has_role(current_user,$1,'USAGE'),
 EXISTS(SELECT 1 FROM pg_roles r WHERE (r.rolsuper OR r.rolbypassrls OR r.rolcreaterole OR r.rolcreatedb OR r.rolreplication OR r.rolname IN ('commerce_auth','commerce_runtime')) AND pg_has_role(current_user,r.oid,'MEMBER')),
 has_schema_privilege(current_user,$2,'USAGE'),has_function_privilege(current_user,$3::oid,'EXECUTE')`, role, schema, functionOID).Scan(&sameLogin, &intended, &privileged, &usage, &execute); err != nil {
		t.Fatal(err)
	}
	if !sameLogin || !intended || privileged || usage != wantUsage || execute {
		t.Fatalf("definer ACL login/schema/function mismatch: sameLogin=%t intended=%t privileged=%t schemaUsage=%t want=%t execute=%t", sameLogin, intended, privileged, usage, wantUsage, execute)
	}
	t.Logf("actual restricted login: schema=%s USAGE=%t functionOID=%d EXECUTE=%t", schema, usage, functionOID, execute)
}

// mdExactAdjustments independently fixes every allowed JSON field and value.
func mdExactAdjustments(t *testing.T, raw []byte, discounted bool) {
	t.Helper()
	var lines []map[string]any
	if err := json.Unmarshal(raw, &lines); err != nil || lines == nil {
		t.Fatal("adjustment reader must return a JSON array, never null")
	}
	if !discounted {
		if len(lines) != 0 || string(raw) != "[]" {
			t.Fatal("no-discount reader must return exactly []")
		}
		return
	}
	if len(lines) != 1 || len(lines[0]) != 4 || lines[0]["kind"] != "manual_discount" || lines[0]["name"] != "直播優惠" || lines[0]["currency"] != "TWD" || lines[0]["amount_minor"] != float64(100) {
		t.Fatal("reader must return only the exact frozen kind/name/currency/amount")
	}
}
