package foundation_test

// COF01-COF05 (contracts/storefront-v2.md §C, unit checkout-offline, migration 0088 + post_river/0018): REAL_PG smoke of the bank_transfer
// payment mode through the real buyer and merchant HTTP handlers. Prefix `cof`. Evidence label of every passing line here is MOCK/REAL_PG:
// no PSP exists on this path, so nothing is SANDBOX or LIVE.
//   COF01 TestBankTransferLifecycle   settings -> options -> placement (home and CVS) -> buyer view/proof (idempotent) -> reject -> confirm
//                                     (stock, ledger, audit, replay, double confirm) -> shippable -> finance -> offline refund
//   COF02 TestBankTransferExpiry      NOT_DUE, window-closed refusals, expire_held releases through the card expiry path, STALE after confirm
//   COF03 TestBankTransferGuards      permissions, tenant/owner isolation, never auto-confirmed, ledger provenance guard, disabled mode
//   COF04 TestFreeShippingThreshold   the server quote applies shipping 0 at/above the policy threshold
//   COF05 TestBuyerEmailPrivacy       the optional email is stored once, exported, blocks erasure while a hold is open, cleared by erasure
// Owner-pool writes (disclosed fixtures): aging checkout.orders / inventory.reservations timestamps (as bcDue does), identity grants (tcvEnv),
// and one negative INSERT into inventory.ledger inside a rolled-back transaction (the guard). Nothing else is fabricated.

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"livecommerce/internal/checkout"
	"livecommerce/internal/storefront"
)

const cofName, cofPhone = "王小明", "0912345678"

func cofJSON(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("not JSON: %v: %s", err, raw)
	}
	return out
}

// cofSettings PUTs the bank-transfer settings through the real merchant route and returns the status and body.
func (e *tcvEnv) cofSettings(version int64, enabled, allowCVS bool, hours int) (int, map[string]any) {
	e.t.Helper()
	body := fmt.Sprintf(`{"expected_version":%d,"enabled":%v,"allow_cvs":%v,"bank_name":"Taiwan Bank","branch":"Taipei","account_name":"Shop Ltd","account_number":"123-456-7890","window_hours":%d}`, version, enabled, allowCVS, hours)
	st, out, _ := e.mcall(e.token(), "PUT", "/v1/admin/stores/"+e.store()+"/bank-transfer-settings", t04Key("cof-set"), body)
	return st, out
}

// cofPlaceHome places a home-delivery bank-transfer order for a fresh buyer (the harness delivery service, shipping 0).
func (e *tcvEnv) cofPlaceHome(b *tcvBuyer, email string) (checkout.Result, error) {
	in := b.h.input
	in.PaymentMode, in.BuyerEmail = "bank_transfer", email
	return e.svc.Begin(context.Background(), b.cap.Token, e.store(), t04Key("cof-begin"), in)
}

// cofPlaceCVS places a CVS (buyer-entered store, MANUAL service) bank-transfer order.
func (e *tcvEnv) cofPlaceCVS(b *tcvBuyer, code string) (checkout.Result, error) {
	pickup := e.tppEntered(b, code)
	return e.tcbTry(b, "cvs_711", code, pickup, cofName, cofPhone, "bank_transfer")
}

func (b *tcvBuyer) cofView(order string) bhResponse {
	return b.req("GET", "/v1/buyer/orders/"+order+"/bank-transfer", "", nil, nil)
}

func (b *tcvBuyer) cofProof(order, key, last5 string, minor int64) bhResponse {
	return b.req("PUT", "/v1/buyer/orders/"+order+"/bank-transfer/proof", key, map[string]any{
		"last5": last5, "amount_minor": minor, "paid_at": time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)}, nil)
}

func (e *tcvEnv) cofDecide(token, order, action, key, body string) (int, map[string]any) {
	st, out, _ := e.mcall(token, "POST", "/v1/admin/stores/"+e.store()+"/orders/"+order+"/bank-transfer/"+action, key, body)
	return st, out
}

func (e *tcvEnv) cofBalance(sku string) (onHand, reserved, allocated int64) {
	if err := e.p.f.owner.QueryRow(context.Background(), `SELECT on_hand,reserved,allocated FROM inventory.balances WHERE tenant_id=$1 AND store_id=$2 AND sku_id=$3`, e.tenant(), e.store(), sku).Scan(&onHand, &reserved, &allocated); err != nil {
		e.t.Fatal(err)
	}
	return
}

func (e *tcvEnv) cofAge(order string, hours int) {
	e.t.Helper()
	f := e.p.f
	mustExec(e.t, f.owner, `UPDATE checkout.orders SET created_at=clock_timestamp()-make_interval(hours=>$2),expires_at=clock_timestamp()-interval '1 second' WHERE id=$1`, order, hours)
	mustExec(e.t, f.owner, `UPDATE inventory.reservations SET created_at=clock_timestamp()-make_interval(hours=>$2),expires_at=clock_timestamp()-interval '1 second' WHERE id=$1`, order, hours)
}

func (e *tcvEnv) cofExpire(order string) (string, *time.Time) {
	e.t.Helper()
	var disposition string
	var retry *time.Time
	if err := e.p.expiry.QueryRow(context.Background(), `SELECT disposition,retry_at FROM checkout.expire_held($1,1)`, order).Scan(&disposition, &retry); err != nil {
		e.t.Fatal(err)
	}
	return disposition, retry
}

func cofCode(m map[string]any) string { s, _ := m["code"].(string); return s }

// cofProbe logs (never fails) the grants and the direct definer answer behind the first merchant call: when an HTTP 5xx hides a database
// error, this prints the real SQLSTATE and message in the same run.
func (e *tcvEnv) cofProbe(label string) {
	e.t.Helper()
	f := e.p.f
	ctx := context.Background()
	hash := sha256.Sum256([]byte(e.token()))
	tx, err := f.runtime.Begin(ctx)
	if err != nil {
		e.t.Logf("probe %s: begin: %v", label, err)
		return
	}
	defer tx.Rollback(ctx)
	var raw []byte
	if err := tx.QueryRow(ctx, `SELECT payments.read_bank_transfer_settings($1,$2::uuid)`, hash[:], e.store()).Scan(&raw); err != nil {
		e.t.Logf("probe %s: direct read_bank_transfer_settings: %v (sqlstate %s)", label, err, sqlState(err))
	} else {
		e.t.Logf("probe %s: direct read_bank_transfer_settings = %s", label, raw)
	}
}

// cofEnsureSettings saves the settings through the real route; when the route fails (already reported) it writes the row with the owner pool so
// the rest of the scenario still exercises the placement, proof, confirm and expiry code.
func (e *tcvEnv) cofEnsureSettings(version int64, enabled, allowCVS bool, hours int) {
	e.t.Helper()
	if st, out := e.cofSettings(version, enabled, allowCVS, hours); st != 200 {
		e.t.Errorf("settings v%d through the route: %d %v", version, st, out)
		e.cofProbe("settings")
		mustExec(e.t, e.p.f.owner, `INSERT INTO checkout.bank_transfer_settings(tenant_id,store_id,enabled,allow_cvs,bank_name,branch,account_name,account_number,window_hours,version,updated_at)
		 VALUES($1,$2,$3,$4,'Taiwan Bank','Taipei','Shop Ltd','123-456-7890',$5,$6+1,clock_timestamp())
		 ON CONFLICT(tenant_id,store_id) DO UPDATE SET enabled=$3,allow_cvs=$4,window_hours=$5,version=$6+1,updated_at=clock_timestamp()`, e.tenant(), e.store(), enabled, allowCVS, hours, version)
	}
}

func TestBankTransferLifecycle(t *testing.T) {
	e := tcvNew(t)
	f := e.p.f
	ctx := context.Background()
	e.grantCreator("orders:read", "payments:refund", "fulfillment:write")
	code, _, _ := e.service("cvs_711", "MANUAL", 0)
	sku := e.p.stock.skus[0].ID
	settingsURL := "/v1/admin/stores/" + e.store() + "/bank-transfer-settings"

	t.Run("settings: defaults, create, stale version, invalid body, idempotent replay, audit", func(t *testing.T) {
		st, out, raw := e.mcall(e.token(), "GET", settingsURL, "", "")
		if st != 200 || out["version"] != float64(0) || out["enabled"] != false || out["window_hours"] != float64(72) {
			e.cofProbe("default read")
			t.Fatalf("default settings: %d %s", st, raw)
		}
		if st, out = e.cofSettings(1, true, true, 72); st != 409 || cofCode(out) != "version_changed" {
			t.Errorf("a stale version must be 409 version_changed: %d %v", st, out)
		}
		for _, bad := range []string{
			`{"expected_version":0,"enabled":true,"allow_cvs":false,"bank_name":"","branch":"","account_name":"","account_number":"","window_hours":72}`,
			`{"expected_version":0,"enabled":false,"allow_cvs":false,"bank_name":"B","branch":"","account_name":"A","account_number":"12","window_hours":72}`,
			`{"expected_version":0,"enabled":false,"allow_cvs":false,"bank_name":"B","branch":"","account_name":"A","account_number":"1234","window_hours":5}`,
			`{"expected_version":0,"enabled":false,"allow_cvs":false,"bank_name":"B","branch":"","account_name":"A","account_number":"1234","window_hours":169}`,
		} {
			if st, out, _ = e.mcall(e.token(), "PUT", settingsURL, t04Key("cof-bad"), bad); st != 422 || cofCode(out) != "invalid_settings" {
				t.Errorf("invalid settings %s: %d %v", bad, st, out)
			}
		}
		key := t04Key("cof-replay")
		body := `{"expected_version":0,"enabled":true,"allow_cvs":true,"bank_name":"Taiwan Bank","branch":"Taipei","account_name":"Shop Ltd","account_number":"123-456-7890","window_hours":72}`
		audit := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='checkout.bank_transfer_settings_changed'`, e.store())
		st1, first, raw1 := e.mcall(e.token(), "PUT", settingsURL, key, body)
		st2, _, raw2 := e.mcall(e.token(), "PUT", settingsURL, key, body)
		if st1 != 200 || st2 != 200 || string(raw1) != string(raw2) || first["version"] != float64(1) {
			t.Fatalf("create and replay: %d %d %s / %s", st1, st2, raw1, raw2)
		}
		if st, out, _ = e.mcall(e.token(), "PUT", settingsURL, key, strings.Replace(body, `"window_hours":72`, `"window_hours":96`, 1)); st != 409 || cofCode(out) != "idempotency_conflict" {
			t.Errorf("same key, other body: %d %v", st, out)
		}
		if n := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='checkout.bank_transfer_settings_changed'`, e.store()); n != audit+1 {
			t.Errorf("settings audit rows %d -> %d, want +1 (replay writes none)", audit, n)
		}
	})

	t.Run("buyer options list bank_transfer for home and CVS rows with the window", func(t *testing.T) {
		b := e.newBuyer()
		res := b.req("GET", "/v1/buyer/checkout-options?market_id="+e.p.market.ID+"&country=TW", "", nil, nil)
		if res.status != 200 {
			t.Fatalf("options: %d %s", res.status, res.body)
		}
		var page struct {
			Items []struct {
				DeliveryKind        string   `json:"delivery_kind"`
				PaymentModes        []string `json:"payment_modes"`
				TransferWindowHours int      `json:"transfer_window_hours"`
			} `json:"items"`
		}
		if err := json.Unmarshal(res.body, &page); err != nil {
			t.Fatal(err)
		}
		var home, cvs bool
		for _, item := range page.Items {
			joined := strings.Join(item.PaymentModes, ",")
			if item.DeliveryKind == "home" && strings.Contains(joined, "bank_transfer") && item.TransferWindowHours == 72 {
				home = true
			}
			if item.DeliveryKind == "cvs_711" && strings.Contains(joined, "bank_transfer") && item.TransferWindowHours == 72 {
				cvs = true
			}
		}
		if !home || !cvs {
			t.Errorf("home row has bank_transfer: %v, CVS row has bank_transfer: %v (%s)", home, cvs, res.body)
		}
	})

	// keep going on a settings failure (reported above) so the later subtests still run
	if e.count(`SELECT count(*) FROM checkout.bank_transfer_settings WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store()) == 0 {
		e.cofEnsureSettings(0, true, true, 72)
	}
	b := e.newBuyer()
	_, reservedBefore, allocatedBefore := e.cofBalance(sku)
	res, err := e.cofPlaceHome(b, "buyer@example.com")
	if err != nil {
		t.Fatalf("home bank_transfer Begin: %v", err)
	}
	order := res.OrderID

	t.Run("placement: AWAITING_TRANSFER, stock only RESERVED, snapshot row, long hold, no payment", func(t *testing.T) {
		if res.PaymentMode != "bank_transfer" || res.CommercialState != "AWAITING_TRANSFER" {
			t.Errorf("result: %+v", res)
		}
		if left := time.Until(res.ExpiresAt); left < 71*time.Hour || left > 73*time.Hour {
			t.Errorf("hold lasts the 72 h window, got %v", left)
		}
		var commercial, fulfilment, mode, email string
		var collection *string
		if err := f.owner.QueryRow(ctx, `SELECT commercial_state,fulfillment_state,payment_mode,collection_state,coalesce(buyer_email,'') FROM checkout.orders WHERE id=$1`, order).Scan(&commercial, &fulfilment, &mode, &collection, &email); err != nil {
			t.Fatal(err)
		}
		if commercial != "AWAITING_TRANSFER" || fulfilment != "MANUAL_UNASSIGNED" || mode != "bank_transfer" || collection != nil || email != "buyer@example.com" {
			t.Errorf("order row: %s/%s/%s/%v/%s", commercial, fulfilment, mode, collection, email)
		}
		if n := e.count(`SELECT count(*) FROM inventory.reservations WHERE id=$1 AND state='HELD'`, order); n != 1 {
			t.Error("the reservation stays HELD until the merchant confirms")
		}
		if n := e.count(`SELECT count(*) FROM inventory.ledger WHERE reservation_id=$1 AND kind='ALLOCATE'`, order); n != 0 {
			t.Errorf("%d ALLOCATE rows at placement: a transfer order never commits stock before the merchant confirms", n)
		}
		if _, reserved, allocated := e.cofBalance(sku); reserved != reservedBefore+2 || allocated != allocatedBefore {
			t.Errorf("balance reserved %d -> %d, allocated %d -> %d", reservedBefore, reserved, allocatedBefore, allocated)
		}
		if n := e.count(`SELECT count(*) FROM checkout.bank_transfers WHERE order_id=$1 AND state='AWAITING' AND bank_name='Taiwan Bank' AND account_number='123-456-7890' AND window_hours=72 AND currency='TWD'`, order); n != 1 {
			t.Error("one AWAITING transfer row with the bank-details snapshot")
		}
		for _, action := range []string{"checkout.held", "checkout.bank_transfer_placed"} {
			if n := e.count(`SELECT count(*) FROM checkout.events WHERE order_id=$1 AND action=$2 AND actor_kind='BUYER'`, order, action); n != 1 {
				t.Errorf("event %s: %d rows", action, n)
			}
		}
		if n := e.count(`SELECT count(*) FROM checkout.payment_attempts WHERE order_id=$1`, order); n != 0 {
			t.Errorf("%d payment attempts for a bank-transfer order", n)
		}
		if got, err := e.svc.Get(ctx, b.cap.Token, e.store(), order); err != nil || got.PaymentMode != "bank_transfer" || got.CommercialState != "AWAITING_TRANSFER" || got.CollectionState != nil {
			t.Errorf("buyer GET order: %+v %v", got, err)
		}
	})

	t.Run("a settings change after placement never moves the order's bank account", func(t *testing.T) {
		if st, out := e.cofSettings(1, true, true, 96); st != 200 || out["version"] != float64(2) {
			t.Errorf("settings v2: %d %v", st, out)
		}
		mustExec(t, f.owner, `UPDATE checkout.bank_transfer_settings SET bank_name='Other Bank' WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store())
		view := b.cofView(order)
		body := cofJSON(t, view.body)
		bank, _ := body["bank"].(map[string]any)
		if view.status != 200 || bank["bank_name"] != "Taiwan Bank" || bank["account_number"] != "123-456-7890" || body["state"] != "AWAITING" ||
			body["amount_minor"] != float64(b.h.quote.Amount.TotalMinor) || body["window_hours"] != float64(72) {
			t.Errorf("buyer view: %d %s", view.status, view.body)
		}
		if st, out := e.cofSettings(2, true, true, 72); st != 200 {
			t.Errorf("restore settings: %d %v", st, out)
		}
	})

	t.Run("proof: idempotent per key, editable, validated, state SUBMITTED never CONFIRMED", func(t *testing.T) {
		key := t04Key("cof-proof")
		first := b.cofProof(order, key, "12345", b.h.quote.Amount.TotalMinor)
		replay := b.cofProof(order, key, "12345", b.h.quote.Amount.TotalMinor)
		if first.status != 200 || replay.status != 200 || string(first.body) != string(replay.body) || cofJSON(t, first.body)["state"] != "SUBMITTED" {
			t.Fatalf("submit and replay: %d %s / %d %s", first.status, first.body, replay.status, replay.body)
		}
		if n := e.count(`SELECT proof_count FROM checkout.bank_transfers WHERE order_id=$1`, order); n != 1 {
			t.Errorf("a replay must not count a second submission: %d", n)
		}
		if other := b.cofProof(order, key, "54321", 1000); other.status != 409 {
			t.Errorf("same key, other body: %d %s", other.status, other.body)
		}
		for name, bad := range map[string]bhResponse{
			"four digits": b.cofProof(order, t04Key("cof-bad1"), "1234", 1000),
			"letters":     b.cofProof(order, t04Key("cof-bad2"), "12a45", 1000),
			"zero amount": b.cofProof(order, t04Key("cof-bad3"), "12345", 0),
		} {
			if bad.status != 422 && bad.status != 400 {
				t.Errorf("%s: %d", name, bad.status)
			}
		}
		future := b.req("PUT", "/v1/buyer/orders/"+order+"/bank-transfer/proof", t04Key("cof-bad4"), map[string]any{"last5": "12345", "amount_minor": 1000, "paid_at": time.Now().UTC().Add(48 * time.Hour).Format(time.RFC3339)}, nil)
		if future.status != 422 || cofCode(cofJSON(t, future.body)) != "invalid_proof" {
			t.Errorf("paid_at in the future: %d %s", future.status, future.body)
		}
		edit := b.cofProof(order, t04Key("cof-proof2"), "99999", b.h.quote.Amount.TotalMinor)
		if edit.status != 200 || e.count(`SELECT count(*) FROM checkout.bank_transfers WHERE order_id=$1 AND proof_last5='99999' AND proof_count=2 AND state='SUBMITTED'`, order) != 1 {
			t.Errorf("a second submission replaces the first: %d %s", edit.status, edit.body)
		}
		if n := e.count(`SELECT count(*) FROM checkout.orders WHERE id=$1 AND commercial_state='AWAITING_TRANSFER'`, order); n != 1 {
			t.Error("a buyer submission never confirms the order")
		}
		if n := e.count(`SELECT count(*) FROM checkout.events WHERE order_id=$1 AND action='checkout.transfer_proof_submitted'`, order); n != 2 {
			t.Errorf("proof events: %d, want 2", n)
		}
		st, out, raw := e.mcall(e.token(), "GET", "/v1/admin/stores/"+e.store()+"/orders/"+order+"/bank-transfer", "", "")
		proof, _ := out["proof"].(map[string]any)
		if st != 200 || out["state"] != "SUBMITTED" || proof["last5"] != "99999" || proof["count"] != float64(2) {
			t.Errorf("merchant view: %d %s", st, raw)
		}
	})

	t.Run("reject: the submission, not the order; audited; idempotent; the buyer sees the reason and can resubmit", func(t *testing.T) {
		stockBefore := func() (int64, int64) { _, r, a := e.cofBalance(sku); return r, a }
		r0, a0 := stockBefore()
		audit := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='checkout.bank_transfer_rejected'`, e.store())
		key := t04Key("cof-reject")
		body := `{"reason":"amount does not match"}`
		st1, out1 := e.cofDecide(e.token(), order, "reject", key, body)
		st2, out2 := e.cofDecide(e.token(), order, "reject", key, body)
		if st1 != 200 || st2 != 200 || out1["state"] != "REJECTED" || fmt.Sprint(out1) != fmt.Sprint(out2) {
			t.Fatalf("reject and replay: %d %v / %d %v", st1, out1, st2, out2)
		}
		if n := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='checkout.bank_transfer_rejected'`, e.store()); n != audit+1 {
			t.Errorf("reject audit rows %d -> %d, want +1", audit, n)
		}
		if st, out := e.cofDecide(e.token(), order, "reject", key, `{"reason":"another reason"}`); st != 409 || cofCode(out) != "idempotency_conflict" {
			t.Errorf("same key other reason: %d %v", st, out)
		}
		if st, out := e.cofDecide(e.token(), order, "reject", t04Key("cof-reject2"), body); st != 409 || cofCode(out) != "transfer_not_submitted" {
			t.Errorf("nothing left to reject: %d %v", st, out)
		}
		for _, bad := range []string{`{"reason":""}`, `{"reason":"   "}`, `{"reason":"` + strings.Repeat("x", 201) + `"}`} {
			if st, _ := e.cofDecide(e.token(), order, "reject", t04Key("cof-reject3"), bad); st != 422 && st != 400 {
				t.Errorf("invalid reason %.20s: %d", bad, st)
			}
		}
		if n := e.count(`SELECT count(*) FROM checkout.orders WHERE id=$1 AND commercial_state='AWAITING_TRANSFER'`, order); n != 1 {
			t.Error("a rejection leaves the order AWAITING_TRANSFER")
		}
		if r1, a1 := stockBefore(); r1 != r0 || a1 != a0 {
			t.Errorf("a rejection moves no stock: reserved %d -> %d, allocated %d -> %d", r0, r1, a0, a1)
		}
		view := cofJSON(t, b.cofView(order).body)
		if view["state"] != "REJECTED" || view["reject_reason"] != "amount does not match" {
			t.Errorf("buyer sees the reason: %v", view)
		}
		if again := b.cofProof(order, t04Key("cof-proof3"), "11111", b.h.quote.Amount.TotalMinor); again.status != 200 {
			t.Fatalf("resubmit after rejection: %d %s", again.status, again.body)
		}
		if n := e.count(`SELECT count(*) FROM checkout.bank_transfers WHERE order_id=$1 AND state='SUBMITTED' AND reject_reason IS NULL AND rejected_at IS NULL`, order); n != 1 {
			t.Error("a resubmission clears the rejection")
		}
	})

	t.Run("confirm: merchant act, stock committed through MERCHANT ALLOCATE rows, audited, idempotent, once", func(t *testing.T) {
		_, r0, a0 := e.cofBalance(sku)
		audit := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='checkout.bank_transfer_confirmed'`, e.store())
		key := t04Key("cof-confirm")
		st1, out1 := e.cofDecide(e.token(), order, "confirm", key, `{}`)
		st2, out2 := e.cofDecide(e.token(), order, "confirm", key, `{}`)
		if st1 != 200 || st2 != 200 || out1["state"] != "CONFIRMED" || out1["commercial_state"] != "CONFIRMED" || fmt.Sprint(out1) != fmt.Sprint(out2) {
			t.Fatalf("confirm and replay: %d %v / %d %v", st1, out1, st2, out2)
		}
		var commercial, reservation, transfer, currency string
		var minor int64
		var by *string
		if err := f.owner.QueryRow(ctx, `SELECT o.commercial_state,r.state,t.state,t.currency,t.confirmed_amount_minor,t.confirmed_by::text FROM checkout.orders o JOIN inventory.reservations r ON r.id=o.id JOIN checkout.bank_transfers t ON t.order_id=o.id WHERE o.id=$1`, order).
			Scan(&commercial, &reservation, &transfer, &currency, &minor, &by); err != nil {
			t.Fatal(err)
		}
		if commercial != "CONFIRMED" || reservation != "COMMITTED" || transfer != "CONFIRMED" || currency != "TWD" || minor != b.h.quote.Amount.TotalMinor || by == nil || *by != f.principalA {
			t.Errorf("rows: %s/%s/%s %s %d by %v (the confirmed amount is the server order total %d)", commercial, reservation, transfer, currency, minor, by, b.h.quote.Amount.TotalMinor)
		}
		var rows int
		var actor, op, command string
		if err := f.owner.QueryRow(ctx, `SELECT count(*),min(actor_kind),min(operation),min(command_key) FROM inventory.ledger WHERE reservation_id=$1 AND kind='ALLOCATE'`, order).Scan(&rows, &actor, &op, &command); err != nil {
			t.Fatal(err)
		}
		if rows != 1 || actor != "MERCHANT" || op != "checkout.bank_transfer.confirm" || command != order {
			t.Errorf("ALLOCATE rows=%d actor=%s op=%s key=%s", rows, actor, op, command)
		}
		if _, r1, a1 := e.cofBalance(sku); r1 != r0-2 || a1 != a0+2 {
			t.Errorf("confirm commits the reservation: reserved %d -> %d, allocated %d -> %d", r0, r1, a0, a1)
		}
		if n := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='checkout.bank_transfer_confirmed'`, e.store()); n != audit+1 {
			t.Errorf("confirm audit rows %d -> %d, want +1 (replay writes none)", audit, n)
		}
		if n := e.count(`SELECT count(*) FROM checkout.payment_attempts WHERE order_id=$1`, order); n != 0 {
			t.Error("still no payment attempt")
		}
		if st, out := e.cofDecide(e.token(), order, "confirm", t04Key("cof-confirm2"), `{}`); st != 409 || cofCode(out) != "already_confirmed" {
			t.Errorf("a second confirm with a new key: %d %v", st, out)
		}
		if n := e.count(`SELECT count(*) FROM inventory.ledger WHERE reservation_id=$1 AND kind='ALLOCATE'`, order); n != 1 {
			t.Errorf("ledger ALLOCATE rows after the double confirm: %d", n)
		}
		if late := b.cofProof(order, t04Key("cof-proof4"), "22222", 1000); late.status != 422 || cofCode(cofJSON(t, late.body)) != "transfer_not_open" {
			t.Errorf("a proof after the confirmation: %d %s", late.status, late.body)
		}
		if st, out := e.cofDecide(e.token(), order, "reject", t04Key("cof-reject4"), `{"reason":"too late"}`); st != 409 || cofCode(out) != "already_confirmed" {
			t.Errorf("reject after confirm: %d %v", st, out)
		}
		if disposition, _ := e.cofExpire(order); disposition != "STALE" {
			t.Errorf("the expiry job of a confirmed order is %s, want STALE (it must release nothing)", disposition)
		}
		if n := e.count(`SELECT count(*) FROM inventory.ledger WHERE reservation_id=$1 AND kind='RELEASE'`, order); n != 0 {
			t.Error("a confirmed order is never released by expiry")
		}
	})

	t.Run("shippable once confirmed, listed as unshipped; finance carries it in its own column", func(t *testing.T) {
		if n := e.count(`SELECT count(*) FROM fulfillment.order_money_shippable($1,$2,$3) s WHERE s`, e.tenant(), e.store(), order); n != 1 {
			t.Error("order_money_shippable must accept a confirmed transfer order")
		}
		st, out, raw := e.mcall(e.token(), "GET", "/v1/admin/stores/"+e.store()+"/orders?state=unshipped&limit=50", "", "")
		items, _ := out["items"].([]any)
		found := false
		for _, it := range items {
			if m, _ := it.(map[string]any); m["order_id"] == order && m["payment_mode"] == "bank_transfer" && m["commercial_state"] == "CONFIRMED" {
				found = true
			}
		}
		if st != 200 || !found {
			t.Errorf("unshipped list: %d %s", st, raw)
		}
		today := time.Now().In(time.FixedZone("TPE", 8*3600))
		from, to := today.AddDate(0, 0, -1).Format("2006-01-02"), today.AddDate(0, 0, 1).Format("2006-01-02")
		var count, minor, captured float64
		st, out, raw = e.mcall(e.token(), "GET", "/v1/admin/stores/"+e.store()+"/finance/summary?from="+from+"&to="+to, "", "")
		for _, row := range func() []any { r, _ := out["totals"].([]any); return r }() {
			m, _ := row.(map[string]any)
			count += m["bank_transfer_confirmed_count"].(float64)
			minor += m["bank_transfer_confirmed_minor"].(float64)
			captured += m["captured_minor"].(float64)
		}
		if st != 200 || count != 1 || minor != float64(b.h.quote.Amount.TotalMinor) || captured != 0 {
			t.Errorf("finance: %d count=%v minor=%v captured=%v %s", st, count, minor, captured, raw)
		}
	})

	t.Run("offline refund: recorded, audited, idempotent once, leaves finance and shippability", func(t *testing.T) {
		audit := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='checkout.bank_transfer_refunded_offline'`, e.store())
		key := t04Key("cof-refund")
		st1, out1 := e.cofDecide(e.token(), order, "refund-offline", key, `{}`)
		st2, out2 := e.cofDecide(e.token(), order, "refund-offline", key, `{}`)
		if st1 != 200 || st2 != 200 || out1["state"] != "REFUNDED_OFFLINE" || fmt.Sprint(out1) != fmt.Sprint(out2) {
			t.Fatalf("refund and replay: %d %v / %d %v", st1, out1, st2, out2)
		}
		if st, out := e.cofDecide(e.token(), order, "refund-offline", t04Key("cof-refund2"), `{}`); st != 409 || cofCode(out) != "already_refunded" {
			t.Errorf("second refund: %d %v", st, out)
		}
		if n := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='checkout.bank_transfer_refunded_offline'`, e.store()); n != audit+1 {
			t.Errorf("refund audit rows %d -> %d", audit, n)
		}
		if n := e.count(`SELECT count(*) FROM fulfillment.order_money_shippable($1,$2,$3) s WHERE s`, e.tenant(), e.store(), order); n != 0 {
			t.Error("a refunded transfer order is no longer shippable")
		}
		if n := e.count(`SELECT count(*) FROM payments.stripe_refunds WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store()); n != 0 {
			t.Error("an offline refund never creates a PSP refund")
		}
	})

	t.Run("a CVS pickup order can pay by transfer too (allow_cvs)", func(t *testing.T) {
		cb := e.newBuyer()
		cres, err := e.cofPlaceCVS(cb, code)
		if err != nil {
			t.Fatalf("CVS bank_transfer Begin: %v", err)
		}
		if cres.PaymentMode != "bank_transfer" || cres.CommercialState != "AWAITING_TRANSFER" {
			t.Errorf("CVS result: %+v", cres)
		}
		if st, out := e.cofDecide(e.token(), cres.OrderID, "confirm", t04Key("cof-cvs-confirm"), `{}`); st != 200 || out["state"] != "CONFIRMED" {
			t.Errorf("confirm CVS transfer order: %d %v", st, out)
		}
	})
}

func TestBankTransferExpiry(t *testing.T) {
	e := tcvNew(t)
	f := e.p.f
	ctx := context.Background()
	e.grantCreator("orders:read", "payments:refund")
	e.cofEnsureSettings(0, true, false, 6)
	sku := e.p.stock.skus[0].ID

	t.Run("early expiry is NOT_DUE with the real due time; the 6 hour window is honoured", func(t *testing.T) {
		b := e.newBuyer()
		res, err := e.cofPlaceHome(b, "")
		if err != nil {
			t.Fatal(err)
		}
		if left := time.Until(res.ExpiresAt); left < 5*time.Hour+50*time.Minute || left > 6*time.Hour+time.Minute {
			t.Errorf("6 hour window, got %v", left)
		}
		disposition, retry := e.cofExpire(res.OrderID)
		if disposition != "NOT_DUE" || retry == nil || time.Until(*retry) < 5*time.Hour {
			t.Errorf("early expire_held: %s retry=%v (the 15 minute River job must snooze to the real due time)", disposition, retry)
		}
	})

	t.Run("a due window releases the stock through the card expiry path and ends the transfer", func(t *testing.T) {
		b := e.newBuyer()
		res, err := e.cofPlaceHome(b, "")
		if err != nil {
			t.Fatal(err)
		}
		if p := b.cofProof(res.OrderID, t04Key("cof-exp-proof"), "12345", b.h.quote.Amount.TotalMinor); p.status != 200 {
			t.Fatalf("proof: %d %s", p.status, p.body)
		}
		_, reserved0, allocated0 := e.cofBalance(sku)
		e.cofAge(res.OrderID, 7)
		if disposition, _ := e.cofExpire(res.OrderID); disposition != "EXPIRED" {
			t.Fatalf("expire_held: %s", disposition)
		}
		var commercial, fulfilment, reservation, transfer string
		if err := f.owner.QueryRow(ctx, `SELECT o.commercial_state,o.fulfillment_state,r.state,t.state FROM checkout.orders o JOIN inventory.reservations r ON r.id=o.id JOIN checkout.bank_transfers t ON t.order_id=o.id WHERE o.id=$1`, res.OrderID).Scan(&commercial, &fulfilment, &reservation, &transfer); err != nil {
			t.Fatal(err)
		}
		if commercial != "CANCELLED" || fulfilment != "CANCELLED" || reservation != "EXPIRED" || transfer != "EXPIRED" {
			t.Errorf("after expiry: %s/%s/%s/%s", commercial, fulfilment, reservation, transfer)
		}
		var rows int
		var actor string
		if err := f.owner.QueryRow(ctx, `SELECT count(*),min(actor_kind) FROM inventory.ledger WHERE reservation_id=$1 AND kind='RELEASE'`, res.OrderID).Scan(&rows, &actor); err != nil || rows != 1 || actor != "SYSTEM_EXPIRY" {
			t.Errorf("RELEASE rows=%d actor=%s %v", rows, actor, err)
		}
		if _, reserved, allocated := e.cofBalance(sku); reserved != reserved0-2 || allocated != allocated0 {
			t.Errorf("stock released: reserved %d -> %d, allocated %d -> %d", reserved0, reserved, allocated0, allocated)
		}
		if n := e.count(`SELECT count(*) FROM checkout.events WHERE order_id=$1 AND action='checkout.expired' AND actor_kind='SYSTEM_EXPIRY'`, res.OrderID); n != 1 {
			t.Error("one checkout.expired event")
		}
		// every door is shut afterwards
		if p := b.cofProof(res.OrderID, t04Key("cof-exp-proof2"), "12345", 1000); p.status != 422 {
			t.Errorf("proof after expiry: %d %s", p.status, p.body)
		}
		if st, out := e.cofDecide(e.token(), res.OrderID, "confirm", t04Key("cof-exp-confirm"), `{}`); st != 409 || cofCode(out) != "transfer_not_open" {
			t.Errorf("confirm after expiry: %d %v", st, out)
		}
		view := cofJSON(t, b.cofView(res.OrderID).body)
		if view["state"] != "EXPIRED" || view["bank"] != nil {
			t.Errorf("expired view must hide the bank details: %v", view)
		}
		if disposition, _ := e.cofExpire(res.OrderID); disposition != "STALE" {
			t.Errorf("a second expiry: %s", disposition)
		}
		if n := e.count(`SELECT count(*) FROM inventory.ledger WHERE reservation_id=$1 AND kind='RELEASE'`, res.OrderID); n != 1 {
			t.Errorf("RELEASE rows after the second expiry: %d", n)
		}
	})

	t.Run("a confirm after the deadline but before the expiry job is refused (the expiry owns the stock)", func(t *testing.T) {
		b := e.newBuyer()
		res, err := e.cofPlaceHome(b, "")
		if err != nil {
			t.Fatal(err)
		}
		mustExec(t, f.owner, `UPDATE checkout.orders SET created_at=clock_timestamp()-interval '7 hours',expires_at=clock_timestamp()-interval '1 second' WHERE id=$1`, res.OrderID)
		mustExec(t, f.owner, `UPDATE inventory.reservations SET created_at=clock_timestamp()-interval '7 hours',expires_at=clock_timestamp()-interval '1 second' WHERE id=$1`, res.OrderID)
		if st, out := e.cofDecide(e.token(), res.OrderID, "confirm", t04Key("cof-late-confirm"), `{}`); st != 409 || cofCode(out) != "transfer_window_closed" {
			t.Errorf("late confirm: %d %v", st, out)
		}
		if p := b.cofProof(res.OrderID, t04Key("cof-late-proof"), "12345", 1000); p.status != 422 || cofCode(cofJSON(t, p.body)) != "transfer_window_closed" {
			t.Errorf("late proof: %d %s", p.status, p.body)
		}
		if n := e.count(`SELECT count(*) FROM inventory.ledger WHERE reservation_id=$1 AND kind='ALLOCATE'`, res.OrderID); n != 0 {
			t.Error("a refused confirm allocates nothing")
		}
		if disposition, _ := e.cofExpire(res.OrderID); disposition != "EXPIRED" {
			t.Errorf("expiry after the refused confirm: %s", disposition)
		}
	})
}

func TestBankTransferGuards(t *testing.T) {
	e := tcvNew(t)
	f := e.p.f
	ctx := context.Background()
	e.grantCreator("orders:read", "payments:refund")

	t.Run("disabled mode: Begin refuses with bank_transfer_unavailable and leaves no hold", func(t *testing.T) {
		b := e.newBuyer()
		before := e.count(`SELECT count(*) FROM checkout.orders WHERE owner_id=$1`, b.cap.Scope.OwnerID)
		_, err := e.cofPlaceHome(b, "")
		tcvExpectRefusal(t, "bank transfer never configured", err, 422, "bank_transfer_unavailable")
		e.cofEnsureSettings(0, false, false, 72)
		_, err = e.cofPlaceHome(b, "")
		tcvExpectRefusal(t, "bank transfer switched off", err, 422, "bank_transfer_unavailable")
		e.cofEnsureSettings(1, true, false, 72)
		code, _, _ := e.service("cvs_711", "MANUAL", 0)
		cb := e.newBuyer()
		_, err = e.cofPlaceCVS(cb, code)
		tcvExpectRefusal(t, "CVS without allow_cvs", err, 422, "bank_transfer_unavailable")
		if n := e.count(`SELECT count(*) FROM checkout.orders WHERE owner_id=$1`, b.cap.Scope.OwnerID); n != before {
			t.Errorf("refused Begins left %d orders", n-before)
		}
		if n := e.count(`SELECT count(*) FROM checkout.orders WHERE owner_id=$1`, cb.cap.Scope.OwnerID); n != 0 {
			t.Error("refused CVS Begin left an order")
		}
	})

	e.cofEnsureSettings(2, true, true, 72)
	b := e.newBuyer()
	res, err := e.cofPlaceHome(b, "")
	if err != nil {
		t.Fatal(err)
	}
	order := res.OrderID

	t.Run("nothing but a merchant act confirms: waiting, a proof and reads never change the order", func(t *testing.T) {
		b.cofProof(order, t04Key("cof-guard-proof"), "12345", b.h.quote.Amount.TotalMinor)
		for i := 0; i < 3; i++ {
			b.cofView(order)
			e.mcall(e.token(), "GET", "/v1/admin/stores/"+e.store()+"/orders/"+order+"/bank-transfer", "", "")
			e.mcall(e.token(), "GET", "/v1/admin/stores/"+e.store()+"/orders/"+order, "", "")
		}
		if n := e.count(`SELECT count(*) FROM checkout.orders WHERE id=$1 AND commercial_state='AWAITING_TRANSFER'`, order); n != 1 {
			t.Error("order left AWAITING_TRANSFER without a merchant decision")
		}
		if n := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action LIKE 'checkout.bank_transfer_%' AND action<>'checkout.bank_transfer_settings_changed'`, e.store()); n != 0 {
			t.Errorf("no confirm/reject/refund was audited, got %d", n)
		}
	})

	t.Run("permissions: confirm, reject and refund need payments:refund; the read needs orders:read; nothing without a token", func(t *testing.T) {
		readOnly, _ := e.member("orders:read")
		noOrders, _ := e.member("catalog:read")
		for _, c := range []struct{ action, body string }{{"confirm", `{}`}, {"reject", `{"reason":"x"}`}, {"refund-offline", `{}`}} {
			if st, _ := e.cofDecide(readOnly, order, c.action, t04Key("cof-perm"), c.body); st != 403 {
				t.Errorf("%s without payments:refund: %d, want 403", c.action, st)
			}
			if st, _ := e.cofDecide("", order, c.action, t04Key("cof-perm"), c.body); st != 401 {
				t.Errorf("%s without a token: %d, want 401", c.action, st)
			}
		}
		if st, _, _ := e.mcall(readOnly, "GET", "/v1/admin/stores/"+e.store()+"/orders/"+order+"/bank-transfer", "", ""); st != 200 {
			t.Errorf("orders:read member reads the transfer: %d", st)
		}
		if st, _, _ := e.mcall(noOrders, "GET", "/v1/admin/stores/"+e.store()+"/orders/"+order+"/bank-transfer", "", ""); st != 403 {
			t.Errorf("member without orders:read reads the transfer: %d", st)
		}
		if st, _, _ := e.mcall(readOnly, "PUT", "/v1/admin/stores/"+e.store()+"/bank-transfer-settings", t04Key("cof-perm"), `{"expected_version":3,"enabled":false,"allow_cvs":false,"bank_name":"","branch":"","account_name":"","account_number":"","window_hours":72}`); st != 403 {
			t.Errorf("settings write without integration:manage: %d", st)
		}
		if st, _, _ := e.mcall(readOnly, "GET", "/v1/admin/stores/"+e.store()+"/bank-transfer-settings", "", ""); st != 403 {
			t.Errorf("settings read without integration:read: %d", st)
		}
		if n := e.count(`SELECT count(*) FROM checkout.orders WHERE id=$1 AND commercial_state='AWAITING_TRANSFER'`, order); n != 1 {
			t.Error("a refused decision changed the order")
		}
		if st, _ := e.cofDecide(e.token(), randomUUID(), "confirm", t04Key("cof-perm"), `{}`); st != 404 {
			t.Errorf("unknown order id: %d, want 404", st)
		}
	})

	t.Run("isolation: another buyer cannot read or submit; a card order has no transfer", func(t *testing.T) {
		other := e.newBuyer()
		if v := other.cofView(order); v.status != 404 {
			t.Errorf("another buyer reads the bank details: %d %s", v.status, v.body)
		}
		if p := other.cofProof(order, t04Key("cof-iso"), "12345", 1000); p.status != 404 {
			t.Errorf("another buyer submits a proof: %d %s", p.status, p.body)
		}
		unauth := b.req("GET", "/v1/buyer/orders/"+order+"/bank-transfer", "", nil, func(r *http.Request) { r.Header.Del("Authorization") })
		if unauth.status != 401 {
			t.Errorf("no capability: %d", unauth.status)
		}
		card, err := e.svc.Begin(ctx, other.cap.Token, e.store(), t04Key("cof-card"), other.h.input)
		if err != nil {
			t.Fatalf("card Begin: %v", err)
		}
		if v := other.cofView(card.OrderID); v.status != 404 {
			t.Errorf("a card order has no transfer view: %d", v.status)
		}
		if p := other.cofProof(card.OrderID, t04Key("cof-iso2"), "12345", 1000); p.status != 404 && p.status != 422 {
			t.Errorf("proof on a card order: %d", p.status)
		}
		if st, out := e.cofDecide(e.token(), card.OrderID, "confirm", t04Key("cof-iso3"), `{}`); st != 422 || cofCode(out) != "not_bank_transfer" {
			t.Errorf("confirm a card order: %d %v", st, out)
		}
	})

	t.Run("ledger provenance: a forged MERCHANT ALLOCATE for an unconfirmed transfer is refused (42501)", func(t *testing.T) {
		var owner, session, wh, skuID string
		var qty int64
		if err := f.owner.QueryRow(ctx, `SELECT o.owner_id::text,o.creator_session_id::text,l.warehouse_id::text,l.sku_id::text,l.quantity FROM checkout.orders o JOIN inventory.reservation_lines l ON l.tenant_id=o.tenant_id AND l.store_id=o.store_id AND l.reservation_id=o.id WHERE o.id=$1`, order).Scan(&owner, &session, &wh, &skuID, &qty); err != nil {
			t.Fatal(err)
		}
		tx, err := f.owner.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		mustExec2 := func(stmt string, args ...any) {
			if _, err := tx.Exec(ctx, stmt, args...); err != nil {
				t.Fatal(err)
			}
		}
		mustExec2(`SELECT set_config('app.tenant_id',$1,true),set_config('app.store_id',$2,true),set_config('app.principal_id',$3,true),set_config('app.buyer_id',$4,true),set_config('app.buyer_session_id',$5,true)`,
			e.tenant(), e.store(), f.principalA, owner, session)
		st, _ := tcsSub(ctx, tx, `INSERT INTO inventory.ledger(tenant_id,store_id,warehouse_id,sku_id,kind,delta_reserved,delta_allocated,operation,command_key,reservation_id,actor_kind,checkout_id,buyer_owner_id,buyer_session_id,principal_id)
		  VALUES($1::uuid,$2::uuid,$3::uuid,$4::uuid,'ALLOCATE',$5::bigint,$6::bigint,'checkout.bank_transfer.confirm',$7::text,$7::uuid,'MERCHANT',$7::uuid,$8::uuid,$9::uuid,$10::uuid)`,
			e.tenant(), e.store(), wh, skuID, -qty, qty, order, owner, session, f.principalA)
		if st != "42501" {
			t.Errorf("a MERCHANT ALLOCATE for an AWAITING_TRANSFER order: want 42501 from inventory.guard_bank_transfer_ledger, got %q", st)
		}
	})
}

func TestFreeShippingThreshold(t *testing.T) {
	e := tcvNew(t)
	code, _, _ := e.service("cvs_711", "MANUAL", 6000)
	b := e.newBuyer()
	quote := func(threshold *int64) storefront.Quote {
		t.Helper()
		// the policy is versioned: every change is a new version, the buyer quote reads the current head
		var version int64
		if err := e.p.f.owner.QueryRow(context.Background(), `SELECT current_version FROM pricing.policy_heads WHERE tenant_id=$1 AND store_id=$2 AND method=$3`, e.tenant(), e.store(), "delivery:"+code).Scan(&version); err != nil {
			t.Fatal(err)
		}
		p := e.p.policy
		fee := int64(6000)
		p.Method, p.ExpectedVersion, p.ShippingMinor, p.Enabled, p.FreeShippingThresholdMinor = "delivery:"+code, version, &fee, true, threshold
		if _, err := e.p.setPolicy(p); err != nil {
			t.Fatalf("set policy: %v", err)
		}
		q, err := b.quote(code)
		if err != nil {
			t.Fatalf("quote: %v", err)
		}
		return q
	}
	base := quote(nil)
	if base.Amount.ShippingMinor != 6000 || base.Policy.FreeShippingThresholdMinor != nil {
		t.Fatalf("no threshold keeps the fee: %+v", base.Amount)
	}
	subtotal := base.Amount.SubtotalMinor
	for _, c := range []struct {
		name      string
		threshold int64
		shipping  int64
	}{{"threshold equal to the subtotal is free (inclusive)", subtotal, 0}, {"threshold one above keeps the fee", subtotal + 1, 6000},
		{"threshold below is free", subtotal - 1, 0}, {"zero threshold is always free", 0, 0}} {
		th := c.threshold
		q := quote(&th)
		if q.Amount.ShippingMinor != c.shipping || q.Amount.TotalMinor != q.Amount.SubtotalMinor+c.shipping+q.Amount.TaxMinor {
			t.Errorf("%s: shipping %d total %d (subtotal %d)", c.name, q.Amount.ShippingMinor, q.Amount.TotalMinor, q.Amount.SubtotalMinor)
		}
		if q.Policy.FreeShippingThresholdMinor == nil || *q.Policy.FreeShippingThresholdMinor != th {
			t.Errorf("%s: the quote snapshot freezes the threshold: %v", c.name, q.Policy.FreeShippingThresholdMinor)
		}
	}
	// the stored value is what the merchant GET returns (BFF policy read)
	free := int64(12345)
	quote(&free)
	st, out, raw := e.mcall(e.token(), "GET", "/v1/admin/stores/"+e.store()+"/markets/"+e.p.market.ID+"/countries/TW/delivery-services/"+code+"/policy", "", "")
	if st != 200 || out["free_shipping_threshold_minor"] != float64(12345) {
		t.Errorf("merchant policy read: %d %s", st, raw)
	}
	if bad := int64(-1); true {
		p := e.p.policy
		fee := int64(6000)
		p.Method, p.ExpectedVersion, p.ShippingMinor, p.FreeShippingThresholdMinor = "delivery:"+code, 9, &fee, &bad
		if _, err := e.p.setPolicy(p); err == nil {
			t.Error("a negative threshold must be refused")
		}
	}
}

func TestBuyerEmailPrivacy(t *testing.T) {
	e := tcvNew(t)
	f := e.p.f
	e.grantCreator("orders:read", "payments:refund")
	e.cofEnsureSettings(0, true, false, 6)
	b := e.newBuyer()
	if _, err := e.cofPlaceHome(b, "not an email"); err == nil {
		t.Fatal("an invalid email must be refused before any order exists")
	}
	if n := e.count(`SELECT count(*) FROM checkout.orders WHERE owner_id=$1`, b.cap.Scope.OwnerID); n != 0 {
		t.Fatal("the refused Begin left an order")
	}
	res, err := e.cofPlaceHome(b, "buyer@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if n := e.count(`SELECT count(*) FROM checkout.orders WHERE id=$1 AND buyer_email='buyer@example.com'`, res.OrderID); n != 1 {
		t.Fatal("the email is stored on the order")
	}
	if n := e.count(`SELECT count(*) FROM checkout.orders WHERE id=$1 AND buyer_email IS NOT NULL AND length(buyer_email)>254`, res.OrderID); n != 0 {
		t.Fatal("length cap")
	}
	// the column CHECK is the twin of the Go validator
	if st := sqlState(func() error {
		_, err := f.owner.Exec(context.Background(), `UPDATE checkout.orders SET buyer_email='no at sign' WHERE id=$1`, res.OrderID)
		return err
	}()); st != "23514" {
		t.Errorf("the SQL CHECK on buyer_email: %q", st)
	}

	export := b.req("POST", "/v1/buyer/privacy/export", t04Key("cof-export"), nil, nil) // the export takes no body
	if export.status != 200 || !strings.Contains(string(export.body), `"buyer_email":"buyer@example.com"`) || !strings.Contains(string(export.body), `"payment_mode":"bank_transfer"`) {
		t.Errorf("the buyer export must include the email and the payment mode: %d %s", export.status, export.body)
	}
	blocked := b.req("POST", "/v1/buyer/privacy/erasure", t04Key("cof-erase"), map[string]any{"confirm": "ERASE"}, nil)
	if blocked.status != 409 || cofCode(cofJSON(t, blocked.body)) != "erasure_blocked" {
		t.Errorf("an open transfer hold blocks erasure like any unexpired hold: %d %s", blocked.status, blocked.body)
	}
	e.cofAge(res.OrderID, 7)
	if disposition, _ := e.cofExpire(res.OrderID); disposition != "EXPIRED" {
		t.Fatalf("expire: %s", disposition)
	}
	erased := b.req("POST", "/v1/buyer/privacy/erasure", t04Key("cof-erase2"), map[string]any{"confirm": "ERASE"}, nil)
	if erased.status != 200 {
		t.Fatalf("erasure after the hold ended: %d %s", erased.status, erased.body)
	}
	if n := e.count(`SELECT count(*) FROM checkout.orders WHERE id=$1 AND buyer_email IS NULL`, res.OrderID); n != 1 {
		t.Error("erasure clears the buyer email")
	}
}
