//go:build browser

// Purpose: W3-U5/C5 browser gate `TestBrowserReturnsUI` — drives returns-ui.spec.ts and parity-order-ops.spec.ts (merchant RMA register/receive/inspect/close/
//   withdraw, merchant cancel refusals, cancel-refund-gaps list, permission gating) against the production admin Next build over a real PG chain.
// Depends on: tcvEnv (stripe mode: card home orders paid through the real rfx capture path, fake Stripe), the 0155 definers via the real
//   merchant routes (/orders/{id}/shipment, /orders/{id}/cancel, refund route), brfStartAdmin + brfPlaywright (browser_refund_fulfilment_test.go),
//   rt* helpers (returns_test.go); env LC_BROWSER_RETURNS_UI_ACCEPTANCE, LC_TEST_DATABASE_ALLOWED; Playwright.
// Used by: scripts/dev/test-local.sh --browser-returns-ui (CI gate only; heavy, not run on the dev Mac).
// Invariants: returns-v1 (a return never refunds; cancel never auto-refunds), contracts/invariants.json I05/I13.
// Status: BROWSER(MOCK) — orders are paid and refunded through the fake Stripe; no provider or deployment acceptance.

package foundation_test

import (
	"context"
	"encoding/json"

	"os"
	"path/filepath"
	"testing"
	"time"

	"livecommerce/internal/httpapi"
)

// bruSeed holds the fixture orders the spec reads through LC_BROWSER_* (fixture contract in the spec header).
type bruSeed struct{ shipped, paid, inFlight, gap string }

// bruShots checks the hashed screenshot manifest of the spec: zh-TW and en, desktop and 390 px (the spec does not shoot zh-CN).
func bruShots(t *testing.T, evidence string, min int) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(evidence, "screenshots.json"))
	if err != nil {
		t.Fatalf("screenshot hash manifest missing: %v; evidence=%s", err, evidence)
	}
	var shots []struct{ File, Sha256, Locale, Viewport string }
	if err := json.Unmarshal(raw, &shots); err != nil || len(shots) < min {
		t.Fatalf("screenshot manifest has %d entries (want >= %d): %v", len(shots), min, err)
	}
	seen := map[string]bool{}
	for _, s := range shots {
		if len(s.Sha256) != 64 {
			t.Fatalf("screenshot %s is not hashed", s.File)
		}
		seen[s.Locale+"/"+s.Viewport] = true
	}
	for _, k := range []string{"zh-TW/desktop", "zh-TW/mobile", "en/desktop", "en/mobile"} {
		if !seen[k] {
			t.Fatalf("no screenshot for %s; evidence=%s", k, evidence)
		}
	}
}

// bruSeedOrders builds the four fixture orders through the real merchant/buyer paths (no forged rows):
// shipped = paid card order shipped by PUT /shipment; paid = CONFIRMED + CAPTURED (cancel -> refund_first);
// inFlight = payment started, not paid (cancel -> payment_in_flight); gap = paid, fully refunded, cancelled with the refund in flight,
// then the provider reports refund.failed so cancel-refund-gaps lists it (TestMerchantCancelRefundGap's path).
func bruSeedOrders(t *testing.T, e *tcvEnv) bruSeed {
	t.Helper()
	var s bruSeed
	s.shipped, _ = e.rtPaid()
	e.rtShip(s.shipped)
	s.paid, _ = e.rtPaid()
	s.inFlight = e.rtPayStarted()
	order, rf := e.rtPaid()
	s.gap = order
	refund := e.r.mustRefund(t, rf, rf.captured, "requested_by_customer")
	st, out, _ := e.mcall(e.token(), "POST", e.rtPath("/orders/"+order+"/cancel"), "bru-cancel-gap", `{"expected_state":"CONFIRMED","reason":"customer asked"}`)
	rtExpect(t, "cancel the gap order with its refund requested", st, out, 200, "")
	e.r.awaitRefundFact(t, refund, rf.attempt, "SUCCEEDED")
	fakeID := e.r.fake.RefundByRef(refund)
	e.r.fake.SetRefundStatus(fakeID, "failed", "declined")
	body := e.r.fake.RefundEventBody("evt_bru_failed_"+t04Tag(), "refund.failed", fakeID, false)
	if status := e.r.deliverRaw(t, rf.endpoint, rf.secret, body); status != 200 {
		t.Fatalf("refund.failed webhook answered %d", status)
	}
	e.r.awaitRefundFact(t, refund, rf.attempt, "FAILED")
	st, out, raw := e.mcall(e.token(), "GET", e.rtPath("/orders/cancel-refund-gaps"), "", "")
	if items, _ := out["items"].([]any); st != 200 || len(items) != 1 {
		t.Fatalf("gap list before the browser run: %d %s", st, raw)
	}
	// The cancelled gap order (CANCELLED, payment CAPTURED, work READY) must not make the store list or its own detail unavailable
	// (merchantorders.validSummary used to 503 both: W3-U5 CI run 37512707012).
	for _, path := range []string{"/orders?view=v2&limit=10&state=all", "/orders/" + s.gap} {
		if st, _, raw := e.mcall(e.token(), "GET", e.rtPath(path), "", ""); st != 200 {
			t.Fatalf("GET %s with the refund-gap order present: %d %s", path, st, raw)
		}
	}
	return s
}

func TestBrowserReturnsUI(t *testing.T) {
	if os.Getenv("LC_BROWSER_RETURNS_UI_ACCEPTANCE") != "1" || os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" {
		t.Fatal("use scripts/dev/test-local.sh --browser-returns-ui")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	e := tcvNew(t, tcvOpts{stripe: true})
	e.r.startWorker(t)
	e.grantCreator("orders:read", "fulfillment:write", "inventory:write")
	seed := bruSeedOrders(t, e)
	restricted, _ := e.member("orders:read") // no fulfillment:write, no inventory:write
	root, _ := filepath.Abs("../..")
	evidence := brfEvidence(t, root, "returns-ui")
	stack := brfStartAdmin(t, ctx, e.r, e.p.f.principalA, evidence)
	// Money snapshot: CAPTURED facts and refund requests only (the real worker may still observe the unpaid in-flight attempt).
	money := func() [2]int {
		return [2]int{e.count(`SELECT count(*) FROM payments.facts WHERE store_id=$1 AND kind='CAPTURED'`, e.store()),
			e.count(`SELECT count(*) FROM payments.stripe_refunds WHERE store_id=$1`, e.store())}
	}
	before := money()
	cancels := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='orders.merchant_cancelled'`, e.store())

	brfPlaywright(t, ctx, stack, []string{"returns-ui.spec.ts", "parity-order-ops.spec.ts"}, map[string]string{
		"LC_BROWSER_STORE": e.store(), "LC_BROWSER_ORDER_SHIPPED": seed.shipped, "LC_BROWSER_ORDER_PAID": seed.paid,
		"LC_BROWSER_ORDER_IN_FLIGHT": seed.inFlight, "LC_BROWSER_GAP_ORDER": seed.gap, "LC_BROWSER_RESTRICTED_TOKEN": restricted})

	// PG facts after the browser run: the UI drove real commands, and nothing it did moved money or cancelled a refused order.
	states := map[string]int{}
	rows, err := e.p.f.owner.Query(ctx, `SELECT state,count(*) FROM returns.rmas WHERE order_id=$1 GROUP BY state`, seed.shipped)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var state string
		var n int
		if err := rows.Scan(&state, &n); err != nil {
			t.Fatal(err)
		}
		states[state] = n
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(states) != 2 || states["CLOSED"] != 1 || states["CANCELLED"] != 1 {
		t.Fatalf("RMAs of the shipped order by state %v: want one CLOSED (full walk) and one CANCELLED (withdrawn); the restricted member's forged POST must add none; evidence=%s", states, evidence)
	}
	if n := e.rtLedger("returns.rma.restock"); n != 1 {
		t.Fatalf("%d restock ledger rows, want exactly 1 (the closed RMA's one sellable unit); evidence=%s", n, evidence)
	}
	if after := money(); after != before {
		t.Fatalf("payments moved during the returns UI run: %v -> %v (a return and a refused cancel must never touch money); evidence=%s", before, after, evidence)
	}
	for order, want := range map[string]string{seed.paid: "CONFIRMED", seed.inFlight: "AWAITING_PAYMENT", seed.gap: "CANCELLED"} {
		if c, _, _ := e.rtStates(order); c != want {
			t.Fatalf("order %s is %s, want %s (a refused cancel must leave it untouched); evidence=%s", order, c, want, evidence)
		}
	}
	if n := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='orders.merchant_cancelled'`, e.store()); n != cancels {
		t.Fatalf("cancel audit rows %d -> %d: the UI cancelled an order the server refuses; evidence=%s", cancels, n, evidence)
	}
	bruShots(t, evidence, 8)
	t.Logf("W3-U5 BROWSER(MOCK): returns/cancel UI passed; screenshots hashed; evidence=%s", evidence)
	t.Run("buyer-requests", bruBuyerRequests)
}

func bruBuyerRequests(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	t.Cleanup(cancel)
	alpha, beta := tcvNew(t), tcvNew(t)
	// loadPage chooses the smallest UUID if no explicit store exists.
	if alpha.store() > beta.store() {
		alpha, beta = beta, alpha
	}
	external := alpha.p.f.storeB
	aliases := map[string]string{alpha.store(): "default-a", beta.store(): "non-default-b", external: "outside"}
	for _, e := range []*tcvEnv{alpha, beta} {
		e.grantCreator("orders:read", "fulfillment:write")
		mustExec(t, e.p.f.owner, "UPDATE control.stores SET name=$2 WHERE id=$1", e.store(), "R02 "+aliases[e.store()])
		st, out := e.hcodSettings(0, true, 20000, 0, "black_cat")
		rtExpect(t, "R02 enable synthetic COD", st, out, 200, "")
	}
	const productName = "帆布通勤提袋"
	const skuCode = "TOTE-NATURAL"
	mustExec(t, beta.p.f.owner, "UPDATE catalog.products SET name=$2 WHERE id=$1", beta.p.stock.product.ID, productName)
	mustExec(t, beta.p.f.owner, "UPDATE catalog.skus SET code=$2 WHERE id=$1", beta.p.stock.skus[0].ID, skuCode)
	buyerOrder := func(e *tcvEnv) (*tcvBuyer, string) {
		b := e.newBuyer()
		order, err := e.hcodPlace(b)
		if err != nil {
			t.Fatalf("R02 synthetic COD placement %s: %v", aliases[e.store()], err)
		}
		e.rtShip(order.OrderID)
		st, out, _ := e.record(e.token(), order.OrderID, t04Key("r02-collect"), "PENDING", "collected")
		rtExpect(t, "R02 synthetic COD record", st, out, 200, "")
		return b, order.OrderID
	}
	rma := func(e *tcvEnv, order string) string {
		res := `{"reason":"damaged: R02 ` + aliases[e.store()] + ` RMA","lines":` + e.rtLines("quantity", 1) + `}`
		st, out, _ := e.mcall(e.token(), "POST", e.rtPath("/orders/"+order+"/returns"), t04Key("r02-rma"), res)
		rtExpect(t, "R02 existing RMA marker", st, out, 201, "")
		if id := tcvStr(out, "id"); id != "" {
			return id
		}
		t.Fatal("R02 existing RMA marker ID missing")
		return ""
	}
	_, orderA := buyerOrder(alpha)
	_, orderB := buyerOrder(beta)
	rmaA, rmaB := rma(alpha, orderA), rma(beta, orderB)
	type application struct {
		ID              string  `json:"id"`
		Number          string  `json:"number"`
		Order           string  `json:"order_id"`
		Status          string  `json:"status"`
		Version         int     `json:"version"`
		ReturnID        *string `json:"return_id"`
		RejectionReason *string `json:"rejection_reason"`
	}
	sku, warehouse := beta.rtSKU()
	create := func(note string) (*tcvBuyer, application) {
		b, order := buyerOrder(beta)
		res := beta.bh.request(t, "POST", "/v1/buyer/orders/"+order+"/return-requests", b.cap.Token, t04Key("r02-damage"),
			map[string]any{"reason": "damaged", "note": note, "lines": []any{map[string]any{"sku_id": sku, "warehouse_id": warehouse, "quantity": 1}}}, nil)
		out := bhRead[application](t, res, 201)
		if out.ID == "" || out.Number == "" || out.Order != order || out.Status != "requested" || out.Version != 1 || out.ReturnID != nil {
			t.Fatal("R02 buyer request needs an open request with no RMA")
		}
		return b, out
	}
	const approveNote = "拆封後發現提帶有一道裂口，請協助確認。"
	const rejectNote = "提袋到貨時布面有磨痕，想確認能否退貨。"
	approveBuyer, approve := create(approveNote)
	rejectBuyer, reject := create(rejectNote)
	withdrawBuyer, withdraw := create("已確認商品狀況，先撤回申請。")
	withdrawPath := "/v1/buyer/return-requests/" + withdraw.ID
	withdrawn := bhRead[application](t, beta.bh.request(t, "POST", withdrawPath+"/withdraw", withdrawBuyer.cap.Token,
		t04Key("r02-withdraw"), map[string]any{"expected_version": 1}, nil), 200)
	if withdrawn.Status != "withdrawn" || withdrawn.Version != 2 || withdrawn.ReturnID != nil {
		t.Fatal("R02 withdrawal did not persist a withdrawn request")
	}
	const rejectReason = "Synthetic package review found no damage"
	// One employee has actual read grants and an explicit fulfilment role in both fixture stores.
	token, principal := beta.member("orders:read", "fulfillment:write", "inventory:write")
	readOnly, _ := beta.member("orders:read")
	f := beta.p.f
	mustExec(t, f.owner, "INSERT INTO identity.memberships(tenant_id,principal_id) VALUES($1,$2)", alpha.tenant(), principal)
	mustExec(t, f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission)
 SELECT $1,$2,$3,p FROM unnest(ARRAY['store:read','orders:read','fulfillment:write','inventory:write']) p`, alpha.tenant(), alpha.store(), principal)
	for _, e := range []*tcvEnv{alpha, beta} {
		mustExec(t, f.owner, "INSERT INTO identity.store_staff(tenant_id,store_id,principal_id,role) VALUES($1,$2,$3,'fulfilment')", e.tenant(), e.store(), principal)
	}
	root, _ := filepath.Abs("../..")
	evidence := brfEvidence(t, root, "r02")
	receipt := map[string]any{"buyerPost": 201, "buyerRequestState": "requested", "buyerRequestHasRMA": false, "principalRole": "fulfilment"}
	for _, prober := range []struct {
		alias, id string
		want      int
		items     int
	}{{"default-a", alpha.store(), 200, 0}, {"non-default-b", beta.store(), 200, 3}, {"outside", external, 0, 0}} {
		st, out, _ := beta.mcall(token, "GET", "/v1/admin/stores/"+prober.id+"/return-requests", "", "")
		if prober.want == 0 {
			if st != 403 && st != 404 {
				t.Fatalf("R02 outside requests status=%d", st)
			}
		} else {
			rtExpect(t, "R02 Go scoped request list "+prober.alias, st, out, prober.want, "")
			items, ok := out["items"].([]any)
			if !ok || len(items) != prober.items {
				t.Fatalf("R02 Go request count %s got=%d", prober.alias, len(items))
			}

		}
		receipt["go-request-list-"+prober.alias] = map[string]any{"status": st, "items": prober.items}
		rmaStatus, rmaOut, _ := beta.mcall(token, "GET", "/v1/admin/stores/"+prober.id+"/returns", "", "")
		if prober.want == 0 {
			if rmaStatus != 403 && rmaStatus != 404 {
				t.Fatalf("R02 outside RMA status=%d", rmaStatus)
			}
		} else {
			rtExpect(t, "R02 Go scoped RMA "+prober.alias, rmaStatus, rmaOut, 200, "")
			items, ok := rmaOut["items"].([]any)
			if !ok || len(items) != 1 {
				t.Fatalf("R02 Go RMA count %s got=%d", prober.alias, len(items))
			}
		}
		receipt["go-rma-list-"+prober.alias] = rmaStatus
	}
	count := func() map[string]int {
		bound := func(q string) int { return beta.count(q, alpha.store(), beta.store()) }
		return map[string]int{
			"paymentFacts":  bound("SELECT count(*) FROM payments.facts WHERE store_id IN ($1,$2)"),
			"refunds":       bound("SELECT count(*) FROM payments.stripe_refunds WHERE store_id IN ($1,$2)"),
			"refundLedger":  bound("SELECT count(*) FROM payments.facts WHERE store_id IN ($1,$2) AND kind LIKE 'REFUND%'"),
			"rmas":          bound("SELECT count(*) FROM returns.rmas WHERE store_id IN ($1,$2)"),
			"buyerRequests": bound("SELECT count(*) FROM returns.buyer_requests WHERE store_id IN ($1,$2)"),
		}
	}
	counts := count()
	receipt["beforeBrowser"] = counts
	t.Cleanup(func() {
		after := count()
		receipt["afterBrowser"] = after
		for k, v := range counts {
			if k == "rmas" {
				v++
			}
			if after[k] != v {
				t.Errorf("R02 %s: want %d, got %d", k, v, after[k])
			}
		}
		if err := os.WriteFile(filepath.Join(evidence, "go-receipt.json"), []byte(mustJSON(t, receipt)), 0600); err != nil {
			t.Error(err)
		}
	})
	stack := brfStartAdminOn(t, ctx, f, principal, evidence, httpapi.Options{SessionStoreList: true}, browserEnvironment)
	brfPlaywright(t, ctx, stack, []string{"returns-ui.spec.ts"}, map[string]string{
		"LC_BROWSER_RETURN_REQUESTS": mustJSON(t, map[string]string{
			"defaultStore": alpha.store(), "nonDefaultStore": beta.store(), "outside": external,
			"defaultRma": rmaA, "nonDefaultRma": rmaB, "buyerRequestNumber": approve.Number, "buyerRequestId": approve.ID,
			"rejectRequestId": reject.ID, "withdrawnRequestId": withdraw.ID, "rejectReason": rejectReason,
			"approveOrder": approve.Order, "rejectOrder": reject.Order, "approveNote": approveNote, "rejectNote": rejectNote,
			"productName": productName, "skuCode": skuCode,
		}),
		"LC_BROWSER_RETURN_REQUESTS_READ_ONLY": readOnly,
	})
	readBuyer := func(b *tcvBuyer, id string) application {
		return bhRead[application](t, beta.bh.request(t, "GET", "/v1/buyer/return-requests/"+id, b.cap.Token, "", nil, nil), 200)
	}
	approved := readBuyer(approveBuyer, approve.ID)
	if n := beta.count(`SELECT count(*) FROM returns.rmas WHERE order_id=$1`, approve.Order); n != 1 ||
		approved.Status != "approved" || approved.Version != 2 || approved.ReturnID == nil ||
		beta.count(`SELECT count(*) FROM returns.rmas WHERE id=$1 AND order_id=$2 AND state='REGISTERED'`, *approved.ReturnID, approve.Order) != 1 {
		t.Fatal("R02 approval must expose exactly one associated REGISTERED RMA to the buyer")
	}
	rejected := readBuyer(rejectBuyer, reject.ID)
	if rejected.Status != "rejected" || rejected.Version != 2 || rejected.ReturnID != nil ||
		rejected.RejectionReason == nil || *rejected.RejectionReason != rejectReason ||
		beta.count(`SELECT count(*) FROM returns.rmas WHERE order_id IN ($1,$2)`, reject.Order, withdraw.Order) != 0 {
		t.Fatal("R02 rejection must expose the reason to the buyer and register no RMA")
	}
	if saved := readBuyer(withdrawBuyer, withdraw.ID); saved.Status != "withdrawn" || saved.Version != 2 || saved.ReturnID != nil {
		t.Fatal("R02 withdrawn request did not persist")
	}
	receipt["afterDecisions"] = map[string]any{"approved": true, "linkedRmas": 1, "buyerReadRejectionReason": true, "withdrawn": true}
}
