//go:build browser

// Purpose: W3-U5 browser gate `TestBrowserReturnsUI` — drives tests/admin/returns-ui.spec.ts (merchant RMA register/receive/inspect/close/
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

	brfPlaywright(t, ctx, stack, []string{"returns-ui.spec.ts"}, map[string]string{
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
}
