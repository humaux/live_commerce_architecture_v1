package foundation_test

// Home-cod R5 (unit home-cod, migration 0107 + post_river/0020, contracts/payment-methods-v1.md). Prefix `hcod`. Tier REAL_PG + HTTP_PG.
//
// Owns: the cash_on_delivery payment mode end to end through the real seams — the merchant settings route
// (GET/PUT /v1/admin/stores/{store}/cash-on-delivery-settings), the buyer checkout-options home row, placement
// (AWAITING_COLLECTION, committed stock, surcharge snapshot, no payment), the shared collection state machine
// (collected after shipping, refunded_offline from COLLECTED), the release guards (cancel/restock-once), the
// finance cod_collected columns and the ACL fences.
// Never: the pay_at_pickup/CVS state machine (TCV15-TCV17), the bank-transfer mode (COF01-COF05/COG01-COG09), the
// carrier API (there is none — the carrier label is a manual-fulfilment hint), or any PSP call.
// Depends-on: the shared tcvEnv/tcvBuyer harness (taiwan_cvs_env_test.go), the begin/options helpers
// (taiwan_cvs_begin_test.go), the collection/release/balance helpers (taiwan_cvs_pay_at_pickup_test.go), mfxShip
// (manual_fulfilment_env_test.go) and tcbOptions (buyer options).
// Used-by: the admin BFF and storefront e2e of the --browser-home-cod gate exercise the same routes in a browser.
// Owner-pool writes (disclosed fixtures): identity grants through tcvEnv.member (ACL subtests). Nothing else is fabricated.

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"livecommerce/internal/checkout"
	"livecommerce/internal/storefront"
)

const hcodName, hcodPhone = "王小明", "0912345678"

// hcodSettings PUTs the store's cash-on-delivery settings through the real merchant route (no Idempotency-Key reuse).
func (e *tcvEnv) hcodSettings(version int64, enabled bool, maxTWD, surchargeTWD int, carrier string) (int, map[string]any) {
	e.t.Helper()
	body := fmt.Sprintf(`{"expected_version":%d,"enabled":%v,"max_twd":%d,"surcharge_twd":%d,"carrier":%q}`, version, enabled, maxTWD, surchargeTWD, carrier)
	st, out, _ := e.mcall(e.token(), "PUT", "/v1/admin/stores/"+e.store()+"/cash-on-delivery-settings", t04Key("hcod-set"), body)
	return st, out
}

// hcodPlace places a home-delivery cash_on_delivery order for a buyer (the harness home service, shipping 0).
func (e *tcvEnv) hcodPlace(b *tcvBuyer) (checkout.Result, error) {
	in := b.h.input
	in.PaymentMode = "cash_on_delivery"
	return e.svc.Begin(context.Background(), b.cap.Token, e.store(), t04Key("hcod-begin"), in)
}

// hcodRehome bumps a buyer's cart and re-selects a home destination + quote, so the same owner can attempt a second order
// (prepare is single-use: its cart set asserts version 0, so a re-home edits the existing destination instead).
func (e *tcvEnv) hcodRehome(b *tcvBuyer) {
	e.t.Helper()
	b.recart()
	dest, err := bdSet(b.h.cqHarness, t04Key("hcod-rehome-dest"), storefront.DestinationInput{
		ExpectedVersion: b.h.destination.Version, CartVersion: b.cartVersion(), Kind: "home", Country: "TW",
		RecipientName: hcodName, Phone: hcodPhone,
		HomeAddress: storefront.HomeAddress{City: "Synthetic city", Line1: "Synthetic home address"},
	})
	if err != nil {
		e.t.Fatalf("rehome destination: %v", err)
	}
	quote, err := b.quote(b.h.delivery.Code)
	if err != nil {
		e.t.Fatalf("rehome quote: %v", err)
	}
	b.h.destination = dest
	b.h.quote = quote
	b.h.input = checkout.Input{QuoteID: quote.ID, DestinationID: dest.ID, CartVersion: b.cartVersion(), ServiceVersion: 1, AllocationVersion: 1}
}

// hcodRow reads the COD order's core columns (commercial, fulfilment, mode, collection, total, surcharge).
func (e *tcvEnv) hcodRow(order string) (commercial, fulfilment, mode string, collection *string, total, surcharge int64) {
	e.t.Helper()
	if err := e.p.f.owner.QueryRow(context.Background(),
		`SELECT commercial_state,fulfillment_state,payment_mode,collection_state,total_minor,cod_surcharge_minor FROM checkout.orders WHERE id=$1`, order).
		Scan(&commercial, &fulfilment, &mode, &collection, &total, &surcharge); err != nil {
		e.t.Fatal(err)
	}
	return
}

func hcodModes(row map[string]any) string { return strings.Join(tcbModes(row), ",") }

// HCOD01: settings -> options -> placement -> ship/collect -> finance -> refunded_offline, on the real routes.
func TestHomeCodLifecycle(t *testing.T) {
	e := tcvNew(t)
	ctx := context.Background()
	e.grantCreator("orders:read", "fulfillment:write", "orders:export")
	sku := e.p.stock.skus[0].ID
	cvs, _, _ := e.service("cvs_711", "MANUAL", 0)
	_ = cvs
	settingsURL := "/v1/admin/stores/" + e.store() + "/cash-on-delivery-settings"

	t.Run("settings: default off, create with surcharge, invalid bodies, replay, one audit", func(t *testing.T) {
		st, out, raw := e.mcall(e.token(), "GET", settingsURL, "", "")
		if st != 200 || out["version"] != float64(0) || out["enabled"] != false || out["max_twd"] != float64(20000) ||
			out["surcharge_twd"] != float64(0) || out["carrier"] != "black_cat" {
			t.Fatalf("default settings: %d %s", st, raw)
		}
		if st, out := e.hcodSettings(1, true, 20000, 50, "black_cat"); st != 409 || cofCode(out) != "version_changed" {
			t.Errorf("stale version: %d %v", st, out)
		}
		for _, bad := range []string{
			`{"expected_version":0,"enabled":true,"max_twd":0,"surcharge_twd":0,"carrier":"black_cat"}`,        // max below floor
			`{"expected_version":0,"enabled":true,"max_twd":20001,"surcharge_twd":0,"carrier":"black_cat"}`,    // max above ceiling
			`{"expected_version":0,"enabled":true,"max_twd":20000,"surcharge_twd":1001,"carrier":"black_cat"}`, // surcharge above ceiling
			`{"expected_version":0,"enabled":true,"max_twd":20000,"surcharge_twd":0,"carrier":"dhl"}`,          // carrier outside the vocabulary
		} {
			if st, out, _ := e.mcall(e.token(), "PUT", settingsURL, t04Key("hcod-bad"), bad); st != 422 || cofCode(out) != "invalid_settings" {
				t.Errorf("invalid %s: %d %v", bad, st, out)
			}
		}
		key := t04Key("hcod-replay")
		body := `{"expected_version":0,"enabled":true,"max_twd":20000,"surcharge_twd":50,"carrier":"black_cat"}`
		audit := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='checkout.cash_on_delivery_settings_changed'`, e.store())
		st1, first, raw1 := e.mcall(e.token(), "PUT", settingsURL, key, body)
		st2, _, raw2 := e.mcall(e.token(), "PUT", settingsURL, key, body)
		if st1 != 200 || st2 != 200 || string(raw1) != string(raw2) || first["version"] != float64(1) {
			t.Fatalf("create and replay: %d %d %s / %s", st1, st2, raw1, raw2)
		}
		if st, out, _ := e.mcall(e.token(), "PUT", settingsURL, key, strings.Replace(body, `"surcharge_twd":50`, `"surcharge_twd":60`, 1)); st != 409 || cofCode(out) != "idempotency_conflict" {
			t.Errorf("same key other body: %d %v", st, out)
		}
		if n := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='checkout.cash_on_delivery_settings_changed'`, e.store()); n != audit+1 {
			t.Errorf("settings audit %d -> %d, want exactly one new row (replay writes none)", audit, n)
		}
	})

	t.Run("buyer options: the home row lists cash_on_delivery with the surcharge, CVS rows never do", func(t *testing.T) {
		b := e.newBuyer()
		rows := tcbOptions(t, e.bh, b.cap.Token, e.p.market.ID)
		home := rows["home"]
		if home == nil || !strings.Contains(hcodModes(home), "cash_on_delivery") || home["cod_surcharge_minor"] != float64(5000) {
			t.Errorf("home row must list cash_on_delivery with cod_surcharge_minor 5000: %v", home)
		}
		if cv := rows["cvs_711"]; cv != nil && strings.Contains(hcodModes(cv), "cash_on_delivery") {
			t.Errorf("a CVS row must never list cash_on_delivery: %v", cv)
		}
	})

	t.Run("placement: AWAITING_COLLECTION, committed stock, surcharge snapshot, no payment", func(t *testing.T) {
		b := e.newBuyer()
		before := e.tppBalance(sku)
		res, err := e.hcodPlace(b)
		if err != nil {
			t.Fatalf("COD Begin: %v", err)
		}
		if res.PaymentMode != "cash_on_delivery" || res.CommercialState != "AWAITING_COLLECTION" {
			t.Errorf("result must carry payment_mode and commercial_state: %+v", res)
		}
		commercial, fulfilment, mode, collection, total, surcharge := e.hcodRow(res.OrderID)
		if commercial != "AWAITING_COLLECTION" || fulfilment != "MANUAL_UNASSIGNED" || mode != "cash_on_delivery" ||
			collection == nil || *collection != "PENDING" || total != 2500 || surcharge != 5000 {
			t.Errorf("order row: %s/%s/%s/%v total=%d surcharge=%d", commercial, fulfilment, mode, collection, total, surcharge)
		}
		if n := e.count(`SELECT count(*) FROM inventory.reservations WHERE id=$1 AND state='COMMITTED'`, res.ReservationID); n != 1 {
			t.Error("the reservation must be COMMITTED at placement (stock held as pay_at_pickup)")
		}
		var rows int
		var actor, op string
		if err := e.p.f.owner.QueryRow(ctx, `SELECT count(*),min(actor_kind),min(operation) FROM inventory.ledger WHERE reservation_id=$1 AND kind='ALLOCATE'`, res.ReservationID).Scan(&rows, &actor, &op); err != nil || rows != 1 || actor != "BUYER" || op != "checkout.pay_at_pickup.commit" {
			t.Errorf("ALLOCATE rows=%d actor=%s op=%s err=%v (want one BUYER checkout.pay_at_pickup.commit)", rows, actor, op, err)
		}
		if n := e.count(`SELECT count(*) FROM checkout.payment_attempts WHERE order_id=$1`, res.OrderID); n != 0 {
			t.Errorf("%d payment attempts for a cash-on-delivery order", n)
		}
		for _, action := range []string{"checkout.held", "checkout.cash_on_delivery_placed"} {
			if n := e.count(`SELECT count(*) FROM checkout.events WHERE order_id=$1 AND action=$2 AND actor_kind='BUYER'`, res.OrderID, action); n != 1 {
				t.Errorf("event %s: %d rows", action, n)
			}
		}
		after := e.tppBalance(sku)
		if after.allocated != before.allocated+2 || after.reserved != before.reserved {
			t.Errorf("balance %+v -> %+v (RESERVE then ALLOCATE nets allocated +2, as pay_at_pickup)", before, after)
		}
	})

	t.Run("collected only after shipping; finance carries the COD columns; refunded_offline from COLLECTED", func(t *testing.T) {
		b := e.newBuyer()
		res, err := e.hcodPlace(b)
		if err != nil {
			t.Fatal(err)
		}
		order := res.OrderID
		writer, _ := e.member("fulfillment:write", "orders:read")
		if n := e.count(`SELECT count(*) FROM fulfillment.order_money_shippable($1,$2,$3) s WHERE s`, e.tenant(), e.store(), order); n != 1 {
			t.Error("order_money_shippable must accept a PENDING AWAITING_COLLECTION COD order")
		}
		if st, _, raw := e.record(writer, order, t04Key("hcod-early"), "PENDING", "collected"); st != 422 || tcvStr(tcvJSON(t, raw), "code") != "not_shipped" {
			t.Errorf("collected before shipped: want 422 not_shipped, got %d %s", st, raw)
		}
		if st, _, raw := e.record(writer, order, t04Key("hcod-early-ret"), "PENDING", "returned"); st != 422 || tcvStr(tcvJSON(t, raw), "code") != "not_shipped" {
			t.Errorf("returned before shipped: want 422 not_shipped, got %d %s", st, raw)
		}
		if st, _, raw := e.mcall(e.token(), "PUT", "/v1/admin/stores/"+e.store()+"/orders/"+order+"/shipment", t04Key("hcod-ms"), mfxShip(0, "sf_express", "TRACKCOD123")); st != 200 {
			t.Fatalf("manual shipment: %d %s", st, raw)
		}
		if n := e.count(`SELECT count(*) FROM checkout.orders WHERE id=$1 AND fulfillment_state='MERCHANT_SHIPPED'`, order); n != 1 {
			t.Error("the order must be MERCHANT_SHIPPED after the manual record")
		}
		if st, out, raw := e.record(writer, order, t04Key("hcod-col"), "PENDING", "collected"); st != 200 || tcvStr(out, "collection_state") != "COLLECTED" || e.collectionState(order) != "COLLECTED" {
			t.Fatalf("collected: %d %s", st, raw)
		}
		if e.audit("fulfillment.collection_recorded") < 1 {
			t.Error("collection audit missing")
		}
		// finance: the collected COD order appears in its own columns (LIVE, total + surcharge), never in captured/net
		_, _, _, _, total, surcharge := e.hcodRow(order)
		today := time.Now().In(time.FixedZone("TPE", 8*3600))
		from, to := today.AddDate(0, 0, -1).Format("2006-01-02"), today.AddDate(0, 0, 1).Format("2006-01-02")
		st, out, raw := e.mcall(e.token(), "GET", "/v1/admin/stores/"+e.store()+"/finance/summary?from="+from+"&to="+to, "", "")
		if st != 200 {
			t.Fatalf("finance: %d %s", st, raw)
		}
		var codCount, codMinor float64
		for _, r := range func() []any { r, _ := out["totals"].([]any); return r }() {
			m, _ := r.(map[string]any)
			if m["currency"] == "TWD" && m["environment"] == "LIVE" {
				codCount += m["cod_collected_count"].(float64)
				codMinor += m["cod_collected_minor"].(float64)
			}
		}
		if codCount != 1 || codMinor != float64(total+surcharge) {
			t.Errorf("finance cod columns %v/%v, want 1/%d (%s)", codCount, codMinor, total+surcharge, raw)
		}
		csv := e.financeCSV(from, to)
		if lines := strings.Split(strings.TrimRight(csv, "\r\n"), "\n"); !strings.HasSuffix(lines[0], "cod_collected_count,cod_collected_minor") {
			t.Errorf("csv header must end with the two COD columns: %q", lines[0])
		}
		// refunded_offline from COLLECTED takes the order out of the collected column
		if st, _, raw := e.record(writer, order, t04Key("hcod-refoff"), "COLLECTED", "refunded_offline"); st != 200 || e.collectionState(order) != "REFUNDED_OFFLINE" {
			t.Errorf("refunded_offline from COLLECTED: %d %s state=%s", st, raw, e.collectionState(order))
		}
		if st, _, raw := e.record(writer, order, t04Key("hcod-refoff-pending"), "PENDING", "refunded_offline"); st == 200 {
			t.Errorf("refunded_offline from PENDING must be refused: %d %s", st, raw)
		}
	})
}

// HCOD02: mode eligibility (home only, setting on), whole-TWD amount + cap, and the shared open-order limit.
func TestHomeCodEligibility(t *testing.T) {
	e := tcvNew(t)
	e.grantCreator("orders:read", "fulfillment:write")
	cvs, _, _ := e.service("cvs_711", "MANUAL", 0)

	t.Run("unavailable: no settings row, then a CVS destination", func(t *testing.T) {
		b := e.newBuyer()
		h0, a0, o0 := e.tcbEffects(b.cap.Scope.OwnerID)
		_, err := e.hcodPlace(b)
		tcvExpectRefusal(t, "COD with no settings row", err, 422, "cash_on_delivery_unavailable")
		if h, a, o := e.tcbEffects(b.cap.Scope.OwnerID); h != h0 || a != a0 || o != o0 {
			t.Errorf("refused Begin left rows %d/%d/%d -> %d/%d/%d", h0, a0, o0, h, a, o)
		}
		if st, out := e.hcodSettings(0, true, 20000, 0, "black_cat"); st != 200 {
			t.Fatalf("enable COD: %d %v", st, out)
		}
		cv := e.newBuyer()
		pickup := e.tppEntered(cv, cvs)
		h1, a1, o1 := e.tcbEffects(cv.cap.Scope.OwnerID)
		_, err = e.tcbTry(cv, "cvs_711", cvs, pickup, hcodName, hcodPhone, "cash_on_delivery")
		tcvExpectRefusal(t, "COD on a CVS destination", err, 422, "cash_on_delivery_unavailable")
		if h, a, o := e.tcbEffects(cv.cap.Scope.OwnerID); h != h1 || a != a1 || o != o1 {
			t.Errorf("refused Begin left rows %d/%d/%d -> %d/%d/%d", h1, a1, o1, h, a, o)
		}
	})

	t.Run("amount: whole-TWD only, and total + surcharge never above the cap", func(t *testing.T) {
		e.hcodSettings(1, true, 100, 50, "black_cat") // NT$100 cap, NT$50 surcharge
		if _, err := e.hcodPlace(e.newBuyer()); err != nil {
			t.Errorf("total (NT$25) + surcharge (NT$50) within the NT$100 cap must be accepted: %v", err)
		}
		half := e.sku(50, 5) // NT$0.50: not a whole TWD
		b := e.newBuyer(storefront.Item{SKUID: half, Quantity: 1})
		tcvExpectRefusal(t, "non-whole TWD total", func() error { _, err := e.hcodPlace(b); return err }(), 422, "cash_on_delivery_amount_exceeds")
		push := e.sku(6000, 10) // NT$60: under the cap alone, over it once the NT$50 surcharge is added
		b2 := e.newBuyer(storefront.Item{SKUID: push, Quantity: 1})
		tcvExpectRefusal(t, "total + surcharge over the cap", func() error { _, err := e.hcodPlace(b2); return err }(), 422, "cash_on_delivery_amount_exceeds")
		over := e.sku(10100, 10) // NT$101: over the cap on its own
		b3 := e.newBuyer(storefront.Item{SKUID: over, Quantity: 1})
		tcvExpectRefusal(t, "total over the cap", func() error { _, err := e.hcodPlace(b3); return err }(), 422, "cash_on_delivery_amount_exceeds")
	})

	t.Run("limit: one open COD order per owner, then the shared store-wide cap", func(t *testing.T) {
		e.hcodSettings(2, true, 20000, 0, "hsinchu")
		a := e.newBuyer()
		if _, err := e.hcodPlace(a); err != nil {
			t.Fatalf("first COD order: %v", err)
		}
		e.hcodRehome(a)
		tcvExpectRefusal(t, "a second open COD order of the same owner", func() error { _, err := e.hcodPlace(a); return err }(), 429, "cash_on_delivery_limit")
		e.cvsSettings(tcvAllChains, true, "20000", 1)
		d := e.newBuyer()
		tcvExpectRefusal(t, "the store-wide open COD cap (pay_at_pickup_max_open)", func() error { _, err := e.hcodPlace(d); return err }(), 429, "cash_on_delivery_limit")
	})
}

// HCOD03: the shared release state machine admits COD (cancel/restock-once) and the settings/collection routes are ACL-gated.
func TestHomeCodReleaseAndACL(t *testing.T) {
	e := tcvNew(t)
	sku := e.p.stock.skus[0].ID
	e.grantCreator("orders:read", "fulfillment:write")
	if st, out := e.hcodSettings(0, true, 20000, 0, "black_cat"); st != 200 {
		t.Fatalf("enable COD: %d %v", st, out)
	}
	writer, writerPrincipal := e.member("fulfillment:write", "orders:read")
	readOnly, _ := e.member("orders:read")

	place := func() (string, *tcvBuyer) {
		b := e.newBuyer()
		res, err := e.hcodPlace(b)
		if err != nil {
			t.Fatalf("place a COD order: %v", err)
		}
		return res.OrderID, b
	}

	t.Run("ACL: settings read/manage and collection write are permission gated", func(t *testing.T) {
		settingsURL := "/v1/admin/stores/" + e.store() + "/cash-on-delivery-settings"
		if st, _, _ := e.mcall(readOnly, "GET", settingsURL, "", ""); st != 403 {
			t.Errorf("settings GET with orders:read only: want 403, got %d", st)
		}
		if st, _, _ := e.mcall(readOnly, "PUT", settingsURL, t04Key("hcod-acl"), `{"expected_version":1,"enabled":false,"max_twd":20000,"surcharge_twd":0,"carrier":"black_cat"}`); st != 403 {
			t.Errorf("settings PUT with orders:read only: want 403, got %d", st)
		}
		reader, _ := e.member("integration:read")
		if st, out, _ := e.mcall(reader, "GET", settingsURL, "", ""); st != 200 || out["version"] != float64(1) {
			t.Errorf("settings GET with integration:read: want 200 version 1, got %d %v", st, out)
		}
		manager, _ := e.member("integration:manage")
		if st, out, _ := e.mcall(manager, "PUT", settingsURL, t04Key("hcod-acl-mgr"), `{"expected_version":1,"enabled":true,"max_twd":20000,"surcharge_twd":0,"carrier":"black_cat"}`); st != 200 || out["version"] != float64(2) {
			t.Errorf("settings PUT with integration:manage: want 200 version 2, got %d %v", st, out)
		}
		order, _ := place()
		if st, _, _ := e.record(readOnly, order, t04Key("hcod-ro-col"), "PENDING", "collected"); st != 403 {
			t.Errorf("collection with orders:read only: want 403, got %d", st)
		}
		if st, _, _ := e.release(readOnly, order, t04Key("hcod-ro-rel"), "cancel", "PENDING"); st != 403 {
			t.Errorf("release with orders:read only: want 403, got %d", st)
		}
		// close the order out so the open count does not leak into the later subtests
		if st, _, raw := e.mcall(e.token(), "PUT", "/v1/admin/stores/"+e.store()+"/orders/"+order+"/shipment", t04Key("hcod-acl-ms"), mfxShip(0, "sf_express", "TRACKACL1")); st != 200 {
			t.Fatalf("manual shipment: %d %s", st, raw)
		}
		if st, _, raw := e.record(writer, order, t04Key("hcod-acl-col"), "PENDING", "collected"); st != 200 {
			t.Fatalf("collected: %d %s", st, raw)
		}
	})

	t.Run("cancel a PENDING unshipped COD order: CANCELLED + one DEALLOCATE, replay and stale", func(t *testing.T) {
		balBefore := e.tppBalance(sku)
		order, b := place()
		balPlaced := e.tppBalance(sku)
		if balPlaced.allocated != balBefore.allocated+2 {
			t.Fatalf("placement should allocate 2: %+v -> %+v", balBefore, balPlaced)
		}
		key := t04Key("hcod-cancel")
		st, out, raw := e.release(writer, order, key, "cancel", "PENDING")
		if st != 200 || tcvStr(out, "order_id") != order || tcvStr(out, "collection_state") != "CANCELLED" ||
			tcvStr(out, "commercial_state") != "CANCELLED" || out["released_lines"] != float64(1) {
			t.Fatalf("cancel: %d %s", st, raw)
		}
		commercial, fulfilment, mode, collection, _, _ := e.hcodRow(order)
		if commercial != "CANCELLED" || fulfilment != "CANCELLED" || mode != "cash_on_delivery" || collection == nil || *collection != "CANCELLED" {
			t.Errorf("states: %s/%s/%s/%v", commercial, fulfilment, mode, collection)
		}
		rows := e.deallocRows(order)
		if len(rows) != 1 || rows[0].op != "fulfillment.pay_at_pickup.cancel" || rows[0].allocated != -2 ||
			rows[0].principal != writerPrincipal || rows[0].owner != b.cap.Scope.OwnerID {
			t.Errorf("DEALLOCATE rows: %+v", rows)
		}
		if after := e.tppBalance(sku); after.allocated != balBefore.allocated || after.reserved != balBefore.reserved {
			t.Errorf("balance after cancel %+v, want allocated/reserved back to %+v", after, balBefore)
		}
		if e.audit("fulfillment.pay_at_pickup_cancelled") < 1 {
			t.Error("audit fulfillment.pay_at_pickup_cancelled missing")
		}
		ledger := e.count(`SELECT count(*) FROM inventory.ledger WHERE reservation_id=$1`, order)
		if st2, again, _ := e.release(writer, order, key, "cancel", "PENDING"); st2 != 200 || fmt.Sprint(again) != fmt.Sprint(out) || e.count(`SELECT count(*) FROM inventory.ledger WHERE reservation_id=$1`, order) != ledger {
			t.Errorf("replay: %d %v", st2, again)
		}
		if st2, _, raw2 := e.release(writer, order, t04Key("hcod-cancel2"), "cancel", "PENDING"); st2 != 409 || tcvStr(tcvJSON(t, raw2), "code") != "collection_state_changed" {
			t.Errorf("a new cancel key on a CANCELLED order: want 409 collection_state_changed, got %d %s", st2, raw2)
		}
	})

	t.Run("restock: ship -> returned -> RESTOCKED once; a second restock is refused", func(t *testing.T) {
		order, _ := place()
		if st, _, raw := e.mcall(e.token(), "PUT", "/v1/admin/stores/"+e.store()+"/orders/"+order+"/shipment", t04Key("hcod-ms2"), mfxShip(0, "chunghwa_post", "TRACKCOD2")); st != 200 {
			t.Fatalf("manual shipment: %d %s", st, raw)
		}
		if st, _, raw := e.record(writer, order, t04Key("hcod-ret"), "PENDING", "returned"); st != 200 {
			t.Fatalf("merchant returned: %d %s", st, raw)
		}
		balBefore := e.tppBalance(sku)
		key := t04Key("hcod-restock")
		st, out, raw := e.release(writer, order, key, "restock", "RETURNED")
		if st != 200 || tcvStr(out, "collection_state") != "RESTOCKED" || e.collectionState(order) != "RESTOCKED" {
			t.Fatalf("restock: %d %s", st, raw)
		}
		rows := e.deallocRows(order)
		if len(rows) != 1 || rows[0].op != "fulfillment.pay_at_pickup.restock" || rows[0].allocated != -2 {
			t.Errorf("restock ledger rows: %+v", rows)
		}
		if after := e.tppBalance(sku); after.allocated != balBefore.allocated-2 {
			t.Errorf("balance %+v -> %+v: restock releases the order allocation", balBefore, after)
		}
		if st2, _, raw2 := e.release(writer, order, t04Key("hcod-restock2"), "restock", "RETURNED"); st2 != 409 || len(e.deallocRows(order)) != 1 {
			t.Errorf("a second restock: want 409 and still exactly one DEALLOCATE row, got %d %s", st2, raw2)
		}
	})
}
