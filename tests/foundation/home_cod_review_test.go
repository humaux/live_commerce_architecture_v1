package foundation_test

// Home-cod R5 independent verification (unit home-cod-tests; author: cross-family test_worker/security_reviewer, not the implementer).
// Prefix `hcr`. Tier REAL_PG + HTTP_PG. Evidence label: REAL_PG (migration 0107 + post_river/0020 applied by the harness).
//
// Owns: the cases the implementer's tests/foundation/home_cod_test.go does not cover — client-tampered money fields over the real buyer
// HTTP route, the setting turned off / cap lowered between quote and begin, cap boundary arithmetic (total + surcharge), the surcharge
// snapshot surviving a settings change, concurrent returned/restock/cancel-vs-ship races, the full collection transition matrix,
// stock hold across the expiry job, payment-start refusal on a COD order, "one shared state machine" catalog assertions, DEFINER/ACL
// inventory incl. the retired commerce_worker role, cross-tenant isolation of the COD routes, and the finance date-range boundary.
// Never: the pay_at_pickup/CVS machine itself (TestCvs*), the browser gate, or the known defects (those are red-until-fixed in
// home_cod_defect_test.go, named TestHomeCodDefect*, and described in REVIEW-home-cod.md).
// Depends-on: the tcvEnv/tcvBuyer harness and the hcod* helpers of home_cod_test.go.
// Owner-pool writes (disclosed fixtures): identity grants (tcvEnv.member), orders.updated_at (finance boundary), expiry ageing (cofAge).

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"livecommerce/internal/checkout"
	"livecommerce/internal/storefront"
)

type hcrResp struct {
	status int
	out    map[string]any
	raw    []byte
}

// hcrParallel releases every fn at the same instant and returns their results in order.
func hcrParallel(fns ...func() hcrResp) []hcrResp {
	out := make([]hcrResp, len(fns))
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i, fn := range fns {
		wg.Add(1)
		go func(i int, fn func() hcrResp) {
			defer wg.Done()
			<-start
			out[i] = fn()
		}(i, fn)
	}
	close(start)
	wg.Wait()
	return out
}

// hcrGated makes the race deterministic: an owner transaction holds the order row lock, every fn is started, and the lock is released only
// once all of them are provably queued on a lock. Whatever the code under test does first (SELECT .. FOR UPDATE or the UPDATE), all
// sessions then contend for the same row at the same instant.
func (e *tcvEnv) hcrGated(order string, fns ...func() hcrResp) []hcrResp {
	e.t.Helper()
	release := e.cogHoldLock(order)
	out := make([]hcrResp, len(fns))
	var wg sync.WaitGroup
	for i, fn := range fns {
		wg.Add(1)
		go func(i int, fn func() hcrResp) {
			defer wg.Done()
			out[i] = fn()
		}(i, fn)
	}
	e.cogWaitLockWaiters(len(fns))
	release()
	wg.Wait()
	return out
}

func (e *tcvEnv) hcrShipPath(order string) string {
	return "/v1/admin/stores/" + e.store() + "/orders/" + order + "/shipment"
}

// hcrEnable switches COD on at settings version 0 (cap NT$20000, black_cat).
func (e *tcvEnv) hcrEnable(surchargeTWD int) {
	e.t.Helper()
	if st, out := e.hcodSettings(0, true, 20000, surchargeTWD, "black_cat"); st != 200 {
		e.t.Fatalf("enable COD: %d %v", st, out)
	}
}

// hcrPlace places a COD order for a fresh buyer (default basket: 2 x NT$12.50 = NT$25).
func (e *tcvEnv) hcrPlace(items ...storefront.Item) (string, *tcvBuyer) {
	e.t.Helper()
	b := e.newBuyer(items...)
	res, err := e.hcodPlace(b)
	if err != nil {
		e.t.Fatalf("place COD order: %v", err)
	}
	return res.OrderID, b
}

// hcrShip records the first manual shipment of an order.
func (e *tcvEnv) hcrShip(order string) {
	e.t.Helper()
	if st, _, raw := e.mcall(e.token(), "PUT", e.hcrShipPath(order), t04Key("hcr-ship"), mfxShip(0, "sf_express", "HCR"+t04Tag())); st != 200 {
		e.t.Fatalf("manual shipment of %s: %d %s", order, st, raw)
	}
}

func (e *tcvEnv) hcrRecord(token, order, expected, state string) hcrResp {
	st, out, raw := e.record(token, order, t04Key("hcr-rec"), expected, state)
	return hcrResp{st, out, raw}
}

func (e *tcvEnv) hcrRelease(token, order, action, expected string) hcrResp {
	st, out, raw := e.release(token, order, t04Key("hcr-rel"), action, expected)
	return hcrResp{st, out, raw}
}

func (e *tcvEnv) hcrLedger(order string) int {
	return e.count(`SELECT count(*) FROM inventory.ledger WHERE reservation_id=$1`, order)
}

// hcrRefused fails unless the response is a coded 409/422 refusal (never a 5xx, never a success).
func hcrRefused(t *testing.T, label string, r hcrResp, codes ...string) {
	t.Helper()
	code := tcvStr(r.out, "code")
	if r.status != 409 && r.status != 422 {
		t.Errorf("%s: want 409/422, got %d %s", label, r.status, r.raw)
		return
	}
	if len(codes) == 0 {
		return
	}
	for _, c := range codes {
		if code == c {
			return
		}
	}
	t.Errorf("%s: want code in %v, got %d %q", label, codes, r.status, code)
}

// HCR01: every money field a hostile client could add to the buyer begin body is refused or ignored; the order carries the server values.
func TestHomeCodReviewTamperedAmounts(t *testing.T) {
	e := tcvNew(t)
	e.grantCreator("orders:read", "fulfillment:write")
	e.hcrEnable(50) // NT$50 surcharge, NT$25 harness basket
	body := func(b *tcvBuyer, mode string, extra map[string]any) map[string]any {
		in := b.h.input
		m := map[string]any{"quote_id": in.QuoteID, "destination_id": in.DestinationID, "cart_version": in.CartVersion,
			"service_version": in.ServiceVersion, "allocation_version": in.AllocationVersion, "payment_mode": mode}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}
	t.Run("extra money keys: refused with no rows, or accepted with the server surcharge and total", func(t *testing.T) {
		for _, extra := range []map[string]any{
			{"cod_surcharge_minor": 0}, {"surcharge_twd": 0}, {"surcharge_minor": 0}, {"total_minor": 100}, {"amount_minor": 100},
			{"max_twd": 99999}, {"carrier": "hsinchu"}, {"collect_amount_minor": 1}, {"snapshot": map[string]any{"amount": map[string]any{"total_minor": 1}}},
		} {
			b := e.newBuyer()
			owner := b.cap.Scope.OwnerID
			h0, a0, o0 := e.tcbEffects(owner)
			res := b.req("POST", "/v1/buyer/checkout", t04Key("hcr-tamper"), body(b, "cash_on_delivery", extra), nil)
			if res.status >= 200 && res.status < 300 {
				order := tcvStr(tcvJSON(t, res.body), "order_id")
				_, _, mode, _, total, surcharge := e.hcodRow(order)
				if mode != "cash_on_delivery" || total != 2500 || surcharge != 5000 {
					t.Errorf("extra %v accepted but the order has mode=%s total=%d surcharge=%d, want cash_on_delivery/2500/5000", extra, mode, total, surcharge)
				}
				continue
			}
			if res.status < 400 || res.status >= 500 {
				t.Errorf("extra %v: want a 4xx refusal or acceptance with server values, got %d %s", extra, res.status, res.body)
			}
			if h, a, o := e.tcbEffects(owner); h != h0 || a != a0 || o != o0 {
				t.Errorf("refused extra %v left rows %d/%d/%d -> %d/%d/%d", extra, h0, a0, o0, h, a, o)
			}
		}
	})
	t.Run("payment_mode spelled any other way is refused with no rows", func(t *testing.T) {
		for _, mode := range []string{"CASH_ON_DELIVERY", "cod", "cash-on-delivery", " cash_on_delivery", "cash_on_delivery ", "cash_on_delivery\n", "cash_on_delivery'--"} {
			b := e.newBuyer()
			owner := b.cap.Scope.OwnerID
			h0, a0, o0 := e.tcbEffects(owner)
			res := b.req("POST", "/v1/buyer/checkout", t04Key("hcr-mode"), body(b, mode, nil), nil)
			if res.status < 400 || res.status >= 500 {
				t.Errorf("payment_mode %q: want a 4xx refusal, got %d %s", mode, res.status, res.body)
			}
			if h, a, o := e.tcbEffects(owner); h != h0 || a != a0 || o != o0 {
				t.Errorf("payment_mode %q left rows %d/%d/%d -> %d/%d/%d", mode, h0, a0, o0, h, a, o)
			}
		}
	})
}

// HCR01b: a CVS (pickup) destination can never take cash on delivery, over the real buyer HTTP route, and nothing is held.
func TestHomeCodReviewCvsDestinationRefusedOverHTTP(t *testing.T) {
	e := tcvNew(t)
	e.grantCreator("orders:read", "fulfillment:write")
	e.hcrEnable(0)
	cvs, _, _ := e.service("cvs_711", "MANUAL", 0)
	// even with pay-at-pickup enabled for the chain, the COD mode is refused on the CVS destination
	e.cvsSettings(tcvAllChains, true, "20000", 20)
	b := e.newBuyer()
	owner := b.cap.Scope.OwnerID
	pickup := e.tppEntered(b, cvs)
	dest, err := b.destination("cvs_711", pickup, hcodName, hcodPhone)
	if err != nil {
		t.Fatal(err)
	}
	quote, err := b.quote(cvs)
	if err != nil {
		t.Fatal(err)
	}
	h0, a0, o0 := e.tcbEffects(owner)
	body := map[string]any{"quote_id": quote.ID, "destination_id": dest.ID, "cart_version": b.cartVersion(), "service_version": e.svcVer[cvs], "allocation_version": 1, "payment_mode": "cash_on_delivery"}
	res := b.req("POST", "/v1/buyer/checkout", t04Key("hcr-cvs-cod"), body, nil)
	if res.status != 422 || tcvStr(tcvJSON(t, res.body), "code") != "cash_on_delivery_unavailable" {
		t.Errorf("COD on a CVS destination over HTTP: want 422 cash_on_delivery_unavailable, got %d %s", res.status, res.body)
	}
	if h, a, o := e.tcbEffects(owner); h != h0 || a != a0 || o != o0 {
		t.Errorf("refused COD begin left rows %d/%d/%d -> %d/%d/%d", h0, a0, o0, h, a, o)
	}
	if rows := tcbOptions(t, e.bh, b.cap.Token, e.p.market.ID); strings.Contains(hcodModes(rows["cvs_711"]), "cash_on_delivery") {
		t.Errorf("a CVS options row lists cash_on_delivery: %v", rows["cvs_711"])
	}
	// the same buyer can still place the legitimate pay_at_pickup order: the refusal poisoned nothing
	if _, err := e.tppPlace(b, cvs, pickup, hcodName, hcodPhone); err != nil {
		t.Errorf("pay_at_pickup after the refused COD attempt: %v", err)
	}
}

// HCR02: the setting and the cap are read at begin, not at quote/options time.
func TestHomeCodReviewSettingChangedAfterQuote(t *testing.T) {
	e := tcvNew(t)
	e.grantCreator("orders:read", "fulfillment:write")
	e.hcrEnable(50)
	b := e.newBuyer() // the quote (NT$25) and the home destination exist from here on
	owner := b.cap.Scope.OwnerID
	if rows := tcbOptions(t, e.bh, b.cap.Token, e.p.market.ID); !strings.Contains(hcodModes(rows["home"]), "cash_on_delivery") {
		t.Fatalf("home row must list cash_on_delivery while the setting is on: %v", rows["home"])
	}
	// 1) switched off after the quote
	if st, out := e.hcodSettings(1, false, 20000, 50, "black_cat"); st != 200 {
		t.Fatalf("disable COD: %d %v", st, out)
	}
	h0, a0, o0 := e.tcbEffects(owner)
	_, err := e.hcodPlace(b)
	tcvExpectRefusal(t, "COD begin after the setting was switched off", err, 422, "cash_on_delivery_unavailable")
	if h, a, o := e.tcbEffects(owner); h != h0 || a != a0 || o != o0 {
		t.Errorf("refused begin left rows %d/%d/%d -> %d/%d/%d", h0, a0, o0, h, a, o)
	}
	if rows := tcbOptions(t, e.bh, b.cap.Token, e.p.market.ID); strings.Contains(hcodModes(rows["home"]), "cash_on_delivery") {
		t.Errorf("home row still lists cash_on_delivery while the setting is off: %v", rows["home"])
	}
	// 2) back on, but the cap drops below total + surcharge (NT$25 + NT$50 = NT$75 > NT$50)
	if st, out := e.hcodSettings(2, true, 50, 50, "black_cat"); st != 200 {
		t.Fatalf("lower cap: %d %v", st, out)
	}
	_, err = e.hcodPlace(b)
	tcvExpectRefusal(t, "COD begin after the cap dropped below total + surcharge", err, 422, "cash_on_delivery_amount_exceeds")
	if h, a, o := e.tcbEffects(owner); h != h0 || a != a0 || o != o0 {
		t.Errorf("cap refusal left rows %d/%d/%d -> %d/%d/%d", h0, a0, o0, h, a, o)
	}
	// 3) the refusals did not poison the quote: restore the cap and the same buyer/quote places the order with the server surcharge
	if st, out := e.hcodSettings(3, true, 20000, 50, "black_cat"); st != 200 {
		t.Fatalf("restore: %d %v", st, out)
	}
	res, err := e.hcodPlace(b)
	if err != nil {
		t.Fatalf("COD begin after the settings were restored: %v", err)
	}
	if _, _, _, _, total, surcharge := e.hcodRow(res.OrderID); total != 2500 || surcharge != 5000 {
		t.Errorf("order total/surcharge %d/%d, want 2500/5000", total, surcharge)
	}
}

// HCR03: total + surcharge against the cap is inclusive at the boundary and exclusive one whole dollar above it; the hard NT$20000 ceiling holds.
func TestHomeCodReviewCapBoundary(t *testing.T) {
	e := tcvNew(t)
	e.grantCreator("orders:read", "fulfillment:write")
	st, out := e.hcodSettings(0, true, 100, 50, "black_cat") // cap NT$100, surcharge NT$50
	if st != 200 {
		t.Fatalf("settings: %d %v", st, out)
	}
	place := func(price int64) error {
		b := e.newBuyer(storefront.Item{SKUID: e.sku(price, 10), Quantity: 1})
		_, err := e.hcodPlace(b)
		return err
	}
	if err := place(5000); err != nil { // NT$50 + NT$50 surcharge == NT$100 cap
		t.Errorf("total + surcharge == cap must be accepted: %v", err)
	}
	tcvExpectRefusal(t, "total + surcharge == cap + NT$1", place(5100), 422, "cash_on_delivery_amount_exceeds")
	tcvExpectRefusal(t, "total + surcharge == cap + NT$0.50 (not whole TWD)", place(5050), 422, "cash_on_delivery_amount_exceeds")
	// the ceilings: cap 20000 + surcharge 1000 (both at their maxima)
	if st, out := e.hcodSettings(1, true, 20000, 1000, "hsinchu"); st != 200 {
		t.Fatalf("ceiling settings: %d %v", st, out)
	}
	if err := place(1900000); err != nil { // NT$19000 + NT$1000 == NT$20000
		t.Errorf("NT$19000 + NT$1000 surcharge under a NT$20000 cap must be accepted: %v", err)
	}
	tcvExpectRefusal(t, "NT$19001 + NT$1000 surcharge over the NT$20000 cap", place(1900100), 422, "cash_on_delivery_amount_exceeds")
	if st, _ := e.hcodSettings(2, true, 20001, 0, "hsinchu"); st != 422 {
		t.Errorf("cap NT$20001 must be refused 422 invalid_settings, got %d", st)
	}
	if st, _ := e.hcodSettings(2, true, 20000, 1001, "hsinchu"); st != 422 {
		t.Errorf("surcharge NT$1001 must be refused 422 invalid_settings, got %d", st)
	}
}

// HCR04: the placed order keeps its own surcharge snapshot; finance and the merchant projection use it, not the current setting.
func TestHomeCodReviewSurchargeSnapshot(t *testing.T) {
	e := tcvNew(t)
	e.grantCreator("orders:read", "fulfillment:write")
	e.hcrEnable(50)
	writer, _ := e.member("fulfillment:write", "orders:read")
	orderA, _ := e.hcrPlace() // placed under a NT$50 surcharge
	if st, out := e.hcodSettings(1, true, 20000, 0, "hsinchu"); st != 200 {
		t.Fatalf("change the surcharge to none: %d %v", st, out)
	}
	orderB, _ := e.hcrPlace() // placed under no surcharge
	if _, _, _, _, total, surcharge := e.hcodRow(orderA); total != 2500 || surcharge != 5000 {
		t.Errorf("order A total/surcharge %d/%d after the setting changed, want 2500/5000 (snapshot)", total, surcharge)
	}
	if _, _, _, _, total, surcharge := e.hcodRow(orderB); total != 2500 || surcharge != 0 {
		t.Errorf("order B total/surcharge %d/%d, want 2500/0", total, surcharge)
	}
	for _, o := range []string{orderA, orderB} {
		e.hcrShip(o)
		if r := e.hcrRecord(writer, o, "PENDING", "collected"); r.status != 200 {
			t.Fatalf("collect %s: %d %s", o, r.status, r.raw)
		}
	}
	// the merchant order projection carries each order's own surcharge
	for order, want := range map[string]float64{orderA: 5000, orderB: 0} {
		st, out, raw := e.mcall(e.token(), "GET", "/v1/admin/stores/"+e.store()+"/orders/"+order, "", "")
		if st != 200 || out["cod_surcharge_minor"] != want || out["total_minor"] != float64(2500) {
			t.Errorf("merchant detail of %s: %d cod_surcharge_minor=%v total_minor=%v, want %v/2500 (%s)", order, st, out["cod_surcharge_minor"], out["total_minor"], want, raw)
		}
	}
	// finance: (2500 + 5000) + (2500 + 0)
	today := time.Now().In(time.FixedZone("TPE", 8*3600))
	from, to := today.AddDate(0, 0, -1).Format("2006-01-02"), today.AddDate(0, 0, 1).Format("2006-01-02")
	count, minor := e.hcrFinanceCod(from, to)
	if count != 2 || minor != 2500+5000+2500 {
		t.Errorf("finance cod columns %d/%d, want 2/10000 (each order at its own placement-time surcharge)", count, minor)
	}
}

// hcrFinanceCod sums the COD columns of the TWD/LIVE total rows of the finance summary over [from, to].
func (e *tcvEnv) hcrFinanceCod(from, to string) (count, minor int64) {
	e.t.Helper()
	st, out, raw := e.mcall(e.token(), "GET", "/v1/admin/stores/"+e.store()+"/finance/summary?from="+from+"&to="+to, "", "")
	if st != 200 {
		e.t.Fatalf("finance %s..%s: %d %s", from, to, st, raw)
	}
	rows, _ := out["totals"].([]any)
	for _, r := range rows {
		m, _ := r.(map[string]any)
		if m["currency"] == "TWD" && m["environment"] == "LIVE" {
			c, _ := m["cod_collected_count"].(float64)
			v, _ := m["cod_collected_minor"].(float64)
			count += int64(c)
			minor += int64(v)
		}
	}
	return
}

// hcrFinanceDay returns the COD columns of the per-day TWD/LIVE row of `day` (0/0 when the day has no row).
func (e *tcvEnv) hcrFinanceDay(from, to, day string) (count, minor int64) {
	e.t.Helper()
	st, out, raw := e.mcall(e.token(), "GET", "/v1/admin/stores/"+e.store()+"/finance/summary?from="+from+"&to="+to, "", "")
	if st != 200 {
		e.t.Fatalf("finance %s..%s: %d %s", from, to, st, raw)
	}
	rows, _ := out["rows"].([]any)
	for _, r := range rows {
		m, _ := r.(map[string]any)
		if m["day"] == day && m["currency"] == "TWD" && m["environment"] == "LIVE" {
			c, _ := m["cod_collected_count"].(float64)
			v, _ := m["cod_collected_minor"].(float64)
			count += int64(c)
			minor += int64(v)
		}
	}
	return
}

// HCR05: concurrency. The same order row serialises every collection write: exactly one winner, exactly one DEALLOCATE per line.
func TestHomeCodReviewConcurrency(t *testing.T) {
	e := tcvNew(t)
	sku := e.p.stock.skus[0].ID
	e.grantCreator("orders:read", "fulfillment:write")
	e.hcrEnable(0)
	writer, _ := e.member("fulfillment:write", "orders:read")

	t.Run("two concurrent returned records (two keys): one 200, one 409, one audit row", func(t *testing.T) {
		order, _ := e.hcrPlace()
		e.hcrShip(order)
		audit := e.audit("fulfillment.collection_recorded")
		res := e.hcrGated(order,
			func() hcrResp { return e.hcrRecord(writer, order, "PENDING", "returned") },
			func() hcrResp { return e.hcrRecord(writer, order, "PENDING", "returned") },
		)
		ok, lost := 0, 0
		for _, r := range res {
			switch {
			case r.status == 200:
				ok++
			case r.status == 409 && tcvStr(r.out, "code") == "collection_state_changed":
				lost++
			default:
				t.Errorf("unexpected concurrent returned answer: %d %s", r.status, r.raw)
			}
		}
		if ok != 1 || lost != 1 {
			t.Errorf("concurrent returned: %d winners, %d losers, want 1/1", ok, lost)
		}
		if got := e.audit("fulfillment.collection_recorded"); got != audit+1 {
			t.Errorf("collection audit rows %d -> %d, want exactly one new", audit, got)
		}
		if s := e.collectionState(order); s != "RETURNED" {
			t.Errorf("collection_state %s, want RETURNED", s)
		}
	})

	t.Run("two concurrent restocks (two keys): one 200, one 409, exactly one DEALLOCATE, allocation released once", func(t *testing.T) {
		order, _ := e.hcrPlace()
		e.hcrShip(order)
		if r := e.hcrRecord(writer, order, "PENDING", "returned"); r.status != 200 {
			t.Fatalf("returned: %d %s", r.status, r.raw)
		}
		before := e.tppBalance(sku)
		res := e.hcrGated(order,
			func() hcrResp { return e.hcrRelease(writer, order, "restock", "RETURNED") },
			func() hcrResp { return e.hcrRelease(writer, order, "restock", "RETURNED") },
		)
		ok, lost := 0, 0
		for _, r := range res {
			switch {
			case r.status == 200:
				ok++
			case r.status == 409 && tcvStr(r.out, "code") == "collection_state_changed":
				lost++
			default:
				t.Errorf("unexpected concurrent restock answer: %d %s", r.status, r.raw)
			}
		}
		if ok != 1 || lost != 1 {
			t.Errorf("concurrent restock: %d winners, %d losers, want 1/1", ok, lost)
		}
		if rows := e.deallocRows(order); len(rows) != 1 || rows[0].op != "fulfillment.pay_at_pickup.restock" || rows[0].allocated != -2 {
			t.Errorf("DEALLOCATE rows after the race: %+v, want exactly one restock row of -2", rows)
		}
		if after := e.tppBalance(sku); after.allocated != before.allocated-2 || after.reserved != before.reserved || after.onHand != before.onHand {
			t.Errorf("balance %+v -> %+v: the allocation must be released exactly once and nothing else may move", before, after)
		}
	})

	t.Run("two concurrent restocks with the SAME key: both answer 200 with one identical body, one DEALLOCATE", func(t *testing.T) {
		order, _ := e.hcrPlace()
		e.hcrShip(order)
		if r := e.hcrRecord(writer, order, "PENDING", "returned"); r.status != 200 {
			t.Fatalf("returned: %d %s", r.status, r.raw)
		}
		key := t04Key("hcr-samekey")
		call := func() hcrResp {
			st, out, raw := e.release(writer, order, key, "restock", "RETURNED")
			return hcrResp{st, out, raw}
		}
		res := e.hcrGated(order, call, call)
		if res[0].status != 200 || res[1].status != 200 || fmt.Sprint(res[0].out) != fmt.Sprint(res[1].out) {
			t.Errorf("same-key restock race: %d %s / %d %s, want two identical 200s", res[0].status, res[0].raw, res[1].status, res[1].raw)
		}
		if rows := e.deallocRows(order); len(rows) != 1 {
			t.Errorf("DEALLOCATE rows: %d, want 1", len(rows))
		}
	})

	t.Run("cancel racing the manual shipment: never CANCELLED and MERCHANT_SHIPPED at once", func(t *testing.T) {
		for i := 0; i < 4; i++ {
			order, _ := e.hcrPlace()
			before := e.tppBalance(sku)
			res := e.hcrGated(order,
				func() hcrResp { return e.hcrRelease(writer, order, "cancel", "PENDING") },
				func() hcrResp {
					st, out, raw := e.mcall(e.token(), "PUT", e.hcrShipPath(order), t04Key("hcr-race-ship"), mfxShip(0, "sf_express", "RACE"+t04Tag()))
					return hcrResp{st, out, raw}
				},
			)
			commercial, fulfilment, _, collection, _, _ := e.hcodRow(order)
			cancelled := res[0].status == 200
			shipped := res[1].status == 200
			switch {
			case cancelled && !shipped:
				if commercial != "CANCELLED" || fulfilment != "CANCELLED" || collection == nil || *collection != "CANCELLED" {
					t.Errorf("round %d: cancel won but the order is %s/%s/%v", i, commercial, fulfilment, collection)
				}
				hcrRefused(t, fmt.Sprintf("round %d ship after cancel", i), res[1], "not_shippable")
				if after := e.tppBalance(sku); after.allocated != before.allocated-2 {
					t.Errorf("round %d: cancel won, allocation %d -> %d, want released", i, before.allocated, after.allocated)
				}
			case shipped && !cancelled:
				if commercial != "AWAITING_COLLECTION" || fulfilment != "MERCHANT_SHIPPED" || collection == nil || *collection != "PENDING" {
					t.Errorf("round %d: ship won but the order is %s/%s/%v", i, commercial, fulfilment, collection)
				}
				hcrRefused(t, fmt.Sprintf("round %d cancel after ship", i), res[0], "not_cancellable")
				if after := e.tppBalance(sku); after.allocated != before.allocated {
					t.Errorf("round %d: ship won, allocation %d -> %d, want unchanged", i, before.allocated, after.allocated)
				}
			default:
				t.Errorf("round %d: cancel=%d ship=%d (%s | %s): exactly one must win", i, res[0].status, res[1].status, res[0].raw, res[1].raw)
			}
		}
	})

	t.Run("two concurrent Begins with one open slot left: exactly one order, the other 429 cash_on_delivery_limit", func(t *testing.T) {
		open := e.count(`SELECT count(*) FROM checkout.orders WHERE store_id=$1 AND payment_mode='cash_on_delivery' AND collection_state='PENDING' AND fulfillment_state='MANUAL_UNASSIGNED'`, e.store())
		e.cvsSettings(tcvAllChains, true, "20000", open+1)
		b1, b2 := e.newBuyer(), e.newBuyer()
		type placed struct {
			order string
			err   error
		}
		results := make([]placed, 2)
		var wg sync.WaitGroup
		start := make(chan struct{})
		for i, b := range []*tcvBuyer{b1, b2} {
			wg.Add(1)
			go func(i int, b *tcvBuyer) {
				defer wg.Done()
				<-start
				res, err := e.hcodPlace(b)
				results[i] = placed{res.OrderID, err}
			}(i, b)
		}
		close(start)
		wg.Wait()
		ok, limited := 0, 0
		for _, r := range results {
			if r.err == nil {
				ok++
			} else if s, c := tcvRefusal(r.err); s == 429 && c == "cash_on_delivery_limit" {
				limited++
			} else {
				t.Errorf("unexpected Begin refusal: %v", r.err)
			}
		}
		if ok != 1 || limited != 1 {
			t.Errorf("concurrent Begins at one free slot: %d placed, %d limited, want 1/1", ok, limited)
		}
		if n := e.count(`SELECT count(*) FROM checkout.orders WHERE store_id=$1 AND payment_mode='cash_on_delivery' AND collection_state='PENDING' AND fulfillment_state='MANUAL_UNASSIGNED'`, e.store()); n != open+1 {
			t.Errorf("open COD orders %d, want %d", n, open+1)
		}
	})
}

// HCR06: the whole transition matrix. Every state refuses every write that is not its one forward step, with no state or ledger change.
func TestHomeCodReviewTransitionMatrix(t *testing.T) {
	e := tcvNew(t)
	sku := e.p.stock.skus[0].ID
	e.grantCreator("orders:read", "fulfillment:write")
	e.hcrEnable(0)
	writer, _ := e.member("fulfillment:write", "orders:read")

	// check runs every write in `refused` against the order and proves nothing moved.
	check := func(t *testing.T, label, order string, refused map[string][2]string, codes ...string) {
		t.Helper()
		state, ledger, bal := e.collectionState(order), e.hcrLedger(order), e.tppBalance(sku)
		commercial, fulfilment, _, _, _, _ := e.hcodRow(order)
		for name, w := range refused { // w = {kind, arg}; kind record: arg "expected,state"; kind release: arg "action,expected"
			parts := strings.SplitN(w[1], ",", 2)
			var r hcrResp
			if w[0] == "record" {
				r = e.hcrRecord(writer, order, parts[0], parts[1])
			} else {
				r = e.hcrRelease(writer, order, parts[0], parts[1])
			}
			hcrRefused(t, label+": "+name, r, codes...)
		}
		c2, f2, _, _, _, _ := e.hcodRow(order)
		if e.collectionState(order) != state || e.hcrLedger(order) != ledger || e.tppBalance(sku) != bal || c2 != commercial || f2 != fulfilment {
			t.Errorf("%s: a refused write moved state/ledger/balance", label)
		}
	}
	codes := []string{"collection_state_changed", "not_shipped", "not_cancellable", "invalid_request"}

	t.Run("PENDING and unshipped", func(t *testing.T) {
		order, _ := e.hcrPlace()
		check(t, "PENDING/unshipped", order, map[string][2]string{
			"collected":        {"record", "PENDING,collected"},
			"returned":         {"record", "PENDING,returned"},
			"refunded_offline": {"record", "PENDING,refunded_offline"},
			"restock":          {"release", "restock,RETURNED"},
		}, codes...)
		hcrRefused(t, "collected before shipped", e.hcrRecord(writer, order, "PENDING", "collected"), "not_shipped")
		hcrRefused(t, "returned before shipped", e.hcrRecord(writer, order, "PENDING", "returned"), "not_shipped")
	})

	t.Run("COLLECTED then REFUNDED_OFFLINE: terminal", func(t *testing.T) {
		order, _ := e.hcrPlace()
		e.hcrShip(order)
		if r := e.hcrRecord(writer, order, "PENDING", "collected"); r.status != 200 {
			t.Fatalf("collected: %d %s", r.status, r.raw)
		}
		check(t, "COLLECTED", order, map[string][2]string{
			"collected again":                 {"record", "PENDING,collected"},
			"returned":                        {"record", "PENDING,returned"},
			"refund from PENDING expectation": {"record", "PENDING,refunded_offline"},
			"cancel":                          {"release", "cancel,PENDING"},
			"restock":                         {"release", "restock,RETURNED"},
		}, codes...)
		if r := e.hcrRecord(writer, order, "COLLECTED", "refunded_offline"); r.status != 200 || tcvStr(r.out, "collection_state") != "REFUNDED_OFFLINE" {
			t.Fatalf("refunded_offline: %d %s", r.status, r.raw)
		}
		check(t, "REFUNDED_OFFLINE", order, map[string][2]string{
			"collected": {"record", "PENDING,collected"},
			"collected (stale expectation COLLECTED)": {"record", "COLLECTED,collected"},
			"returned":       {"record", "PENDING,returned"},
			"refunded again": {"record", "COLLECTED,refunded_offline"},
			"cancel":         {"release", "cancel,PENDING"},
			"restock":        {"release", "restock,RETURNED"},
		}, codes...)
		if allocated := e.count(`SELECT count(*) FROM inventory.ledger WHERE reservation_id=$1 AND kind='DEALLOCATE'`, order); allocated != 0 {
			t.Errorf("a COLLECTED/REFUNDED_OFFLINE COD order wrote %d DEALLOCATE rows, want 0 (stock stays sold)", allocated)
		}
	})

	t.Run("RETURNED then RESTOCKED: terminal", func(t *testing.T) {
		order, _ := e.hcrPlace()
		e.hcrShip(order)
		if r := e.hcrRecord(writer, order, "PENDING", "returned"); r.status != 200 {
			t.Fatalf("returned: %d %s", r.status, r.raw)
		}
		check(t, "RETURNED", order, map[string][2]string{
			"collected":        {"record", "PENDING,collected"},
			"returned again":   {"record", "PENDING,returned"},
			"refunded_offline": {"record", "COLLECTED,refunded_offline"},
			"cancel":           {"release", "cancel,PENDING"},
		}, codes...)
		if r := e.hcrRelease(writer, order, "restock", "RETURNED"); r.status != 200 || tcvStr(r.out, "collection_state") != "RESTOCKED" {
			t.Fatalf("restock: %d %s", r.status, r.raw)
		}
		check(t, "RESTOCKED", order, map[string][2]string{
			"collected":        {"record", "PENDING,collected"},
			"returned":         {"record", "PENDING,returned"},
			"refunded_offline": {"record", "COLLECTED,refunded_offline"},
			"cancel":           {"release", "cancel,PENDING"},
			"restock again":    {"release", "restock,RETURNED"},
		}, codes...)
	})

	t.Run("CANCELLED: terminal, never shippable", func(t *testing.T) {
		order, _ := e.hcrPlace()
		if r := e.hcrRelease(writer, order, "cancel", "PENDING"); r.status != 200 {
			t.Fatalf("cancel: %d %s", r.status, r.raw)
		}
		check(t, "CANCELLED", order, map[string][2]string{
			"collected":        {"record", "PENDING,collected"},
			"returned":         {"record", "PENDING,returned"},
			"refunded_offline": {"record", "COLLECTED,refunded_offline"},
			"cancel again":     {"release", "cancel,PENDING"},
			"restock":          {"release", "restock,RETURNED"},
		}, codes...)
		st, out, raw := e.mcall(e.token(), "PUT", e.hcrShipPath(order), t04Key("hcr-ship-cancelled"), mfxShip(0, "sf_express", "NOPE"+t04Tag()))
		if st != 422 || tcvStr(out, "code") != "not_shippable" {
			t.Errorf("shipping a CANCELLED COD order: want 422 not_shippable, got %d %s", st, raw)
		}
	})

	t.Run("shipped PENDING: cancel is refused as not_cancellable, restock as a stale expectation", func(t *testing.T) {
		order, _ := e.hcrPlace()
		e.hcrShip(order)
		check(t, "PENDING/shipped", order, map[string][2]string{
			"cancel":           {"release", "cancel,PENDING"},
			"restock":          {"release", "restock,RETURNED"},
			"refunded_offline": {"record", "PENDING,refunded_offline"},
		}, codes...)
		hcrRefused(t, "cancel of a shipped order", e.hcrRelease(writer, order, "cancel", "PENDING"), "not_cancellable")
	})
}

// HCR07: stock is held across the 15-minute expiry job, and a COD order can never start an online payment.
func TestHomeCodReviewStockHoldAndNoPayment(t *testing.T) {
	e := tcvNew(t)
	sku := e.p.stock.skus[0].ID
	ctx := context.Background()
	e.grantCreator("orders:read", "fulfillment:write")
	e.hcrEnable(50)
	b := e.newBuyer()
	before := e.tppBalance(sku)
	res, err := e.hcodPlace(b)
	if err != nil {
		t.Fatalf("place: %v", err)
	}
	placed := e.tppBalance(sku)
	if placed.allocated != before.allocated+2 || placed.reserved != before.reserved {
		t.Fatalf("placement balance %+v -> %+v, want allocated +2 and reserved unchanged", before, placed)
	}
	t.Run("the expiry job reaches STALE: stock and reservation untouched, order still AWAITING_COLLECTION", func(t *testing.T) {
		mustExec(t, e.p.f.owner, `UPDATE checkout.orders SET created_at=created_at-interval '3 hours',expires_at=expires_at-interval '3 hours' WHERE id=$1`, res.OrderID) // disclosed owner-pool fixture: the 15-minute job is now due
		if d, _ := e.cofExpire(res.OrderID); d != "STALE" {
			t.Errorf("expire_held on a committed COD order: %s, want STALE", d)
		}
		if got := e.tppBalance(sku); got != placed {
			t.Errorf("expiry moved stock %+v -> %+v", placed, got)
		}
		if n := e.count(`SELECT count(*) FROM inventory.reservations WHERE id=$1 AND state='COMMITTED'`, res.ReservationID); n != 1 {
			t.Error("the reservation must stay COMMITTED after the expiry job")
		}
		if n := e.count(`SELECT count(*) FROM inventory.ledger WHERE reservation_id=$1 AND actor_kind='SYSTEM_EXPIRY'`, res.OrderID); n != 0 {
			t.Errorf("%d SYSTEM_EXPIRY ledger rows for a COD order", n)
		}
		if commercial, _, _, collection, _, _ := e.hcodRow(res.OrderID); commercial != "AWAITING_COLLECTION" || collection == nil || *collection != "PENDING" {
			t.Errorf("order after the expiry job: %s/%v", commercial, collection)
		}
	})
	t.Run("start_payment refuses a COD order and leaves no attempt", func(t *testing.T) {
		if _, err := e.p.starter.StartPayment(ctx, b.cap.Token, e.store(), t04Key("hcr-start"), checkout.PaymentInput{OrderID: res.OrderID, MethodCode: "payuni_credit", MethodVersion: 1}); err == nil {
			t.Error("checkout.start_payment must refuse a cash_on_delivery order")
		}
		if n := e.count(`SELECT count(*) FROM checkout.payment_attempts WHERE order_id=$1`, res.OrderID); n != 0 {
			t.Errorf("a refused payment start left %d attempts", n)
		}
		for _, path := range []string{"/payment/prepare", "/payment/handoff", "/payment/refresh", "/payment/cancel", "/payment"} {
			r := b.req("POST", "/v1/buyer/orders/"+res.OrderID+path, t04Key("hcr-pay"), map[string]any{}, nil)
			if r.status >= 200 && r.status < 300 {
				t.Errorf("POST %s on a COD order answered %d %s, want a refusal", path, r.status, r.body)
			}
		}
		if n := e.count(`SELECT count(*) FROM checkout.payment_attempts WHERE order_id=$1`, res.OrderID); n != 0 {
			t.Errorf("payment routes left %d attempts for a COD order", n)
		}
	})
	t.Run("a card refund can never be requested for a COD order", func(t *testing.T) {
		before := e.count(`SELECT count(*) FROM payments.stripe_refunds`)
		refunder, _ := e.member("payments:refund", "orders:read")
		st, _, raw := e.mcall(refunder, "POST", "/v1/admin/stores/"+e.store()+"/orders/"+res.OrderID+"/refunds", t04Key("hcr-refund"), rfxBody(100, "requested_by_customer", 100))
		if st == 200 || st == 201 || st == 202 {
			t.Errorf("a card refund on a COD order answered %d %s, want a refusal", st, raw)
		}
		if got := e.count(`SELECT count(*) FROM payments.stripe_refunds`); got != before {
			t.Errorf("refund rows %d -> %d for a COD order", before, got)
		}
	})
}

// HCR08: "one shared function, not a copy": the catalog holds exactly one record_collection and one release_pay_at_pickup, both admit
// both modes, and no COD-specific copy of the machine exists next to them.
func TestHomeCodReviewSharedStateMachine(t *testing.T) {
	e := tcvNew(t)
	ctx := context.Background()
	for _, name := range []string{"record_collection", "release_pay_at_pickup"} {
		rows, err := e.p.f.owner.Query(ctx, `SELECT n.nspname, pg_get_functiondef(p.oid) FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE p.proname=$1`, name)
		if err != nil {
			t.Fatal(err)
		}
		var defs []string
		for rows.Next() {
			var schema, def string
			if err := rows.Scan(&schema, &def); err != nil {
				t.Fatal(err)
			}
			defs = append(defs, def)
		}
		rows.Close()
		if len(defs) != 1 {
			t.Errorf("%s: %d definitions in the catalog, want exactly one shared function", name, len(defs))
			continue
		}
		if !strings.Contains(defs[0], "'pay_at_pickup'") || !strings.Contains(defs[0], "'cash_on_delivery'") {
			t.Errorf("%s must admit both pay_at_pickup and cash_on_delivery in one body", name)
		}
	}
	// the COD-named objects are exactly the settings pair and the offer read; there is no COD collection/release/restock function
	rows, err := e.p.f.owner.Query(ctx, `SELECT n.nspname||'.'||p.proname FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
	  WHERE n.nspname IN ('checkout','fulfillment','inventory','payments','identity','notify') AND (p.proname ~ '(^|_)cod(_|$)' OR p.proname ~ 'cash_on_delivery') ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		got = append(got, s)
	}
	want := "checkout.read_cod_offer,payments.read_cash_on_delivery_settings,payments.set_cash_on_delivery_settings"
	if strings.Join(got, ",") != want {
		t.Errorf("COD-named functions %v, want exactly %s (no second copy of the collection state machine)", got, want)
	}
	// exactly one begin_hold survives (post_river/0020 replaced 0018 in place)
	if n := e.count(`SELECT count(*) FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname='checkout' AND p.proname='begin_hold'`); n != 1 {
		t.Errorf("%d checkout.begin_hold definitions, want 1", n)
	}
}

// HCR09: DEFINER/ACL inventory of everything 0107 and post_river/0020 added, incl. the retired commerce_worker role.
func TestHomeCodReviewACLInventory(t *testing.T) {
	e := tcvNew(t)
	ctx := context.Background()
	execGrantees := func(proc string) []string {
		rows, err := e.p.f.owner.Query(ctx, `SELECT coalesce(r.rolname,'PUBLIC') FROM pg_proc p CROSS JOIN LATERAL aclexplode(coalesce(p.proacl,acldefault('f',p.proowner))) a
		  LEFT JOIN pg_roles r ON r.oid=a.grantee WHERE p.oid=$1::regprocedure AND a.privilege_type='EXECUTE' ORDER BY 1`, proc)
		if err != nil {
			t.Fatalf("%s: %v", proc, err)
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var s string
			if err := rows.Scan(&s); err != nil {
				t.Fatal(err)
			}
			out = append(out, s)
		}
		return out
	}
	for proc, want := range map[string]string{
		"payments.read_cash_on_delivery_settings(bytea,uuid)":                                               "commerce_checkout_writer,commerce_runtime",
		"payments.set_cash_on_delivery_settings(bytea,uuid,text,bytea,bigint,boolean,integer,integer,text)": "commerce_checkout_writer,commerce_runtime",
		"checkout.read_cod_offer(bytea,uuid)":                                                               "commerce_checkout_runtime,commerce_checkout_writer",
		"fulfillment.record_collection(bytea,uuid,uuid,text,bytea,text,text)":                               "commerce_checkout_writer,commerce_runtime",
		"inventory.release_pay_at_pickup(bytea,uuid,uuid,text,bytea,text,text)":                             "commerce_checkout_writer,commerce_runtime",
		"checkout.begin_hold(bytea,uuid,text,bytea,uuid,jsonb,jsonb,bigint,text,text)":                      "commerce_checkout_runtime,commerce_checkout_writer",
	} {
		if got := strings.Join(execGrantees(proc), ","); got != want {
			t.Errorf("EXECUTE on %s: %s, want exactly %s (no PUBLIC, no worker role)", proc, got, want)
		}
	}
	// SECURITY DEFINER, owned by the checkout writer, pinned search_path
	for _, proc := range []string{"payments.read_cash_on_delivery_settings(bytea,uuid)", "payments.set_cash_on_delivery_settings(bytea,uuid,text,bytea,bigint,boolean,integer,integer,text)",
		"checkout.read_cod_offer(bytea,uuid)", "fulfillment.record_collection(bytea,uuid,uuid,text,bytea,text,text)", "inventory.release_pay_at_pickup(bytea,uuid,uuid,text,bytea,text,text)",
		"checkout.begin_hold(bytea,uuid,text,bytea,uuid,jsonb,jsonb,bigint,text,text)"} {
		var definer bool
		var owner string
		var cfg []string
		if err := e.p.f.owner.QueryRow(ctx, `SELECT p.prosecdef, r.rolname, coalesce(p.proconfig,'{}') FROM pg_proc p JOIN pg_roles r ON r.oid=p.proowner WHERE p.oid=$1::regprocedure`, proc).Scan(&definer, &owner, &cfg); err != nil {
			t.Fatalf("%s: %v", proc, err)
		}
		if !definer || owner != "commerce_checkout_writer" || !strings.Contains(strings.Join(cfg, ","), "search_path=pg_catalog") {
			t.Errorf("%s: definer=%v owner=%s config=%v, want SECURITY DEFINER owned by commerce_checkout_writer with search_path=pg_catalog", proc, definer, owner, cfg)
		}
	}
	// the retired commerce_worker role holds nothing of the new surface, directly or through PUBLIC/inheritance
	for _, proc := range []string{"payments.read_cash_on_delivery_settings(bytea,uuid)", "payments.set_cash_on_delivery_settings(bytea,uuid,text,bytea,bigint,boolean,integer,integer,text)",
		"checkout.read_cod_offer(bytea,uuid)", "fulfillment.record_collection(bytea,uuid,uuid,text,bytea,text,text)", "inventory.release_pay_at_pickup(bytea,uuid,uuid,text,bytea,text,text)",
		"checkout.begin_hold(bytea,uuid,text,bytea,uuid,jsonb,jsonb,bigint,text,text)"} {
		for _, role := range []string{"commerce_worker", "commerce_expiry_worker", "commerce_payment_worker", "commerce_claims_worker", "commerce_ads_worker"} {
			var exists, has bool
			if err := e.p.f.owner.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname=$1)`, role).Scan(&exists); err != nil {
				t.Fatal(err)
			}
			if !exists {
				continue
			}
			if err := e.p.f.owner.QueryRow(ctx, `SELECT has_function_privilege($1,$2::regprocedure,'EXECUTE')`, role, proc).Scan(&has); err != nil {
				t.Fatal(err)
			}
			if has {
				t.Errorf("role %s can EXECUTE %s", role, proc)
			}
		}
	}
	// the settings table: FORCE RLS, only the checkout writer holds privileges, nobody inherits them
	var rls, force bool
	if err := e.p.f.owner.QueryRow(ctx, `SELECT relrowsecurity,relforcerowsecurity FROM pg_class WHERE oid='checkout.cash_on_delivery_settings'::regclass`).Scan(&rls, &force); err != nil || !rls || !force {
		t.Errorf("cash_on_delivery_settings RLS enabled=%v forced=%v err=%v, want both", rls, force, err)
	}
	roles, err := e.p.f.owner.Query(ctx, `SELECT rolname FROM pg_roles WHERE rolname LIKE 'commerce\_%' ESCAPE '\' AND rolname<>'commerce_checkout_writer' ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for roles.Next() {
		var n string
		if err := roles.Scan(&n); err != nil {
			t.Fatal(err)
		}
		names = append(names, n)
	}
	roles.Close()
	if len(names) < 10 {
		t.Fatalf("role inventory too small to be meaningful: %v", names)
	}
	for _, role := range names {
		for _, priv := range []string{"SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "TRIGGER"} {
			var has bool
			if err := e.p.f.owner.QueryRow(ctx, `SELECT has_table_privilege($1,'checkout.cash_on_delivery_settings',$2)`, role, priv).Scan(&has); err != nil {
				t.Fatal(err)
			}
			if has {
				t.Errorf("role %s holds %s on checkout.cash_on_delivery_settings (only commerce_checkout_writer may)", role, priv)
			}
		}
	}
	// the surcharge column is not writable by any request role
	for _, role := range names {
		var has bool
		if err := e.p.f.owner.QueryRow(ctx, `SELECT has_column_privilege($1,'checkout.orders','cod_surcharge_minor','UPDATE')`, role).Scan(&has); err != nil {
			t.Fatal(err)
		}
		if has {
			t.Errorf("role %s can UPDATE checkout.orders.cod_surcharge_minor", role)
		}
	}
	var worker bool
	if err := e.p.f.owner.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname='commerce_worker') AND has_column_privilege('commerce_worker','checkout.orders','cod_surcharge_minor','SELECT')`).Scan(&worker); err != nil || worker {
		t.Errorf("retired commerce_worker can read checkout.orders.cod_surcharge_minor (%v, %v)", worker, err)
	}
}

// HCR10: tenant/store come from the server credential only. Another tenant, or the same tenant's other store, can neither read nor write
// this store's COD settings or collection, and its own COD switch never enables COD here.
func TestHomeCodReviewIsolation(t *testing.T) {
	e := tcvNew(t)
	f := e.p.f
	e.grantCreator("orders:read", "fulfillment:write")
	e.hcrEnable(50)
	order, _ := e.hcrPlace()
	e.hcrShip(order)
	allPerms := []string{"store:read", "orders:read", "fulfillment:write", "integration:read", "integration:manage"}
	_, tokenB := lcPrincipal(t, f, f.tenantB, []string{f.storeB}, allPerms...)
	storeA2 := randomUUID()
	mustExec(t, f.owner, `INSERT INTO control.stores(tenant_id,id,name,currency) VALUES($1,$2,'second store','TWD')`, e.tenant(), storeA2)
	_, tokenA2 := lcPrincipal(t, f, f.tenantA, []string{storeA2}, allPerms...)

	settingsBefore := e.count(`SELECT version FROM checkout.cash_on_delivery_settings WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store())
	auditBefore := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action IN ('checkout.cash_on_delivery_settings_changed','fulfillment.collection_recorded','fulfillment.pay_at_pickup_cancelled','fulfillment.pay_at_pickup_restocked')`, e.store())
	base := "/v1/admin/stores/" + e.store()
	for _, c := range []struct{ name, token string }{{"other tenant", tokenB}, {"same tenant, other store's grants", tokenA2}} {
		for _, r := range []struct{ method, path, body string }{
			{"GET", base + "/cash-on-delivery-settings", ""},
			{"PUT", base + "/cash-on-delivery-settings", `{"expected_version":1,"enabled":false,"max_twd":1,"surcharge_twd":1000,"carrier":"hsinchu"}`},
			{"POST", base + "/orders/" + order + "/collection", `{"expected_state":"PENDING","state":"collected"}`},
			{"POST", base + "/orders/" + order + "/pay-at-pickup-release", `{"action":"cancel","expected_state":"PENDING"}`},
		} {
			key := ""
			if r.method != "GET" {
				key = t04Key("hcr-iso")
			}
			st, _, raw := e.mcall(c.token, r.method, r.path, key, r.body)
			if st != 403 && st != 404 {
				t.Errorf("%s: %s %s answered %d %s, want 403/404", c.name, r.method, r.path, st, raw)
			}
		}
	}
	if n := e.count(`SELECT version FROM checkout.cash_on_delivery_settings WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store()); n != settingsBefore {
		t.Errorf("COD settings version %d -> %d through a foreign credential", settingsBefore, n)
	}
	if n := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action IN ('checkout.cash_on_delivery_settings_changed','fulfillment.collection_recorded','fulfillment.pay_at_pickup_cancelled','fulfillment.pay_at_pickup_restocked')`, e.store()); n != auditBefore {
		t.Errorf("audit rows %d -> %d through a foreign credential", auditBefore, n)
	}
	if s := e.collectionState(order); s != "PENDING" {
		t.Errorf("collection_state %s after foreign collection attempts, want PENDING", s)
	}
	// the other tenant's own COD switch is its own: turning it on there does not change this store
	st, out, raw := e.mcall(tokenB, "PUT", "/v1/admin/stores/"+f.storeB+"/cash-on-delivery-settings", t04Key("hcr-iso-b"), `{"expected_version":0,"enabled":true,"max_twd":20000,"surcharge_twd":0,"carrier":"black_cat"}`)
	if st != 200 {
		t.Fatalf("tenant B enables COD on its own store: %d %s", st, raw)
	}
	_ = out
	if st, out, _ := e.mcall(e.token(), "GET", base+"/cash-on-delivery-settings", "", ""); st != 200 || out["surcharge_twd"] != float64(50) || out["version"] != float64(1) {
		t.Errorf("this store's COD settings changed after tenant B enabled its own: %d %v", st, out)
	}
	if n := e.count(`SELECT count(*) FROM checkout.cash_on_delivery_settings WHERE store_id=$1 AND tenant_id<>$2`, f.storeB, f.tenantB); n != 0 {
		t.Errorf("%d COD settings rows for store B under another tenant", n)
	}
}

// HCR11: finance date-range boundary. The day is the Taipei calendar day of updated_at; ranges are inclusive at both ends and
// half-open on the instant: the first microsecond of a day belongs to it, the last microsecond of the previous day to the previous day.
func TestHomeCodReviewFinanceBoundaries(t *testing.T) {
	e := tcvNew(t)
	ctx := context.Background()
	e.grantCreator("orders:read", "fulfillment:write", "orders:export")
	e.hcrEnable(50)
	writer, _ := e.member("fulfillment:write", "orders:read")
	tpe := time.FixedZone("TPE", 8*3600)
	d := time.Now().In(tpe).AddDate(0, 0, -10)
	mid := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, tpe) // first instant of day D
	day := func(ts time.Time) string { return ts.In(tpe).Format("2006-01-02") }
	collect := func(at time.Time) string {
		order, _ := e.hcrPlace()
		e.hcrShip(order)
		if r := e.hcrRecord(writer, order, "PENDING", "collected"); r.status != 200 {
			t.Fatalf("collect: %d %s", r.status, r.raw)
		}
		mustExec(t, e.p.f.owner, `UPDATE checkout.orders SET updated_at=$2 WHERE id=$1`, order, at)
		return order
	}
	collect(mid)                        // D        (first instant)
	collect(mid.Add(-time.Microsecond)) // D-1      (last microsecond)
	collect(mid.AddDate(0, 0, 1))       // D+1      (first instant)
	collect(mid.Add(12 * time.Hour))    // D        (noon)
	// states that must never appear in the collected column, all inside the range
	for _, step := range []string{"pending", "returned", "restocked", "refunded", "cancelled"} {
		order, _ := e.hcrPlace()
		switch step {
		case "returned", "restocked", "refunded":
			e.hcrShip(order)
		}
		switch step {
		case "returned":
			e.hcrRecord(writer, order, "PENDING", "returned")
		case "restocked":
			e.hcrRecord(writer, order, "PENDING", "returned")
			e.hcrRelease(writer, order, "restock", "RETURNED")
		case "refunded":
			e.hcrRecord(writer, order, "PENDING", "collected")
			e.hcrRecord(writer, order, "COLLECTED", "refunded_offline")
		case "cancelled":
			e.hcrRelease(writer, order, "cancel", "PENDING")
		}
		mustExec(t, e.p.f.owner, `UPDATE checkout.orders SET updated_at=$2 WHERE id=$1`, order, mid.Add(6*time.Hour))
	}
	prev, cur, next := day(mid.AddDate(0, 0, -1)), day(mid), day(mid.AddDate(0, 0, 1))
	const one = 2500 + 5000 // total + surcharge
	for _, c := range []struct {
		from, to, day string
		count, minor  int64
	}{
		{prev, next, prev, 1, one}, {prev, next, cur, 2, 2 * one}, {prev, next, next, 1, one},
		{cur, cur, cur, 2, 2 * one}, {prev, prev, prev, 1, one}, {next, next, next, 1, one},
	} {
		if n, m := e.hcrFinanceDay(c.from, c.to, c.day); n != c.count || m != c.minor {
			t.Errorf("finance %s..%s day %s: %d/%d, want %d/%d", c.from, c.to, c.day, n, m, c.count, c.minor)
		}
	}
	for _, c := range []struct {
		from, to     string
		count, minor int64
	}{{cur, cur, 2, 2 * one}, {prev, prev, 1, one}, {prev, next, 4, 4 * one}, {day(mid.AddDate(0, 0, 2)), day(mid.AddDate(0, 0, 3)), 0, 0}} {
		if n, m := e.hcrFinanceCod(c.from, c.to); n != c.count || m != c.minor {
			t.Errorf("finance totals %s..%s: %d/%d, want %d/%d (pending/returned/restocked/refunded/cancelled orders never count)", c.from, c.to, n, m, c.count, c.minor)
		}
	}
	// never part of captured / net / pickup
	st, out, raw := e.mcall(e.token(), "GET", "/v1/admin/stores/"+e.store()+"/finance/summary?from="+prev+"&to="+next, "", "")
	if st != 200 {
		t.Fatalf("finance: %d %s", st, raw)
	}
	for _, r := range out["totals"].([]any) {
		m := r.(map[string]any)
		if m["captured_minor"] != float64(0) || m["net_minor"] != float64(0) || m["pickup_collected_minor"] != float64(0) || m["bank_transfer_confirmed_minor"] != float64(0) {
			t.Errorf("COD cash leaked into another money column: %v", m)
		}
	}
	// the CSV carries the same two figures in its last two columns
	csv := e.financeCSV(cur, cur)
	lines := strings.Split(strings.TrimRight(csv, "\r\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("csv for %s: %q", cur, csv)
	}
	f := strings.Split(strings.TrimRight(lines[1], "\r"), ",")
	if len(f) != 13 || f[11] != "2" || f[12] != fmt.Sprint(2*one) || f[0] != cur || f[2] != "LIVE" {
		t.Errorf("csv row %q, want 13 columns, LIVE, cod count 2 and minor %d", lines[1], 2*one)
	}
	_ = ctx
}

// HCR12: idempotent retries. Begin, the settings write and the collection record all answer an exact retry with the first answer and
// change nothing a second time; the same key with another body is a conflict.
func TestHomeCodReviewIdempotentRetries(t *testing.T) {
	e := tcvNew(t)
	ctx := context.Background()
	sku := e.p.stock.skus[0].ID
	e.grantCreator("orders:read", "fulfillment:write")
	e.hcrEnable(50)
	writer, _ := e.member("fulfillment:write", "orders:read")
	b := e.newBuyer()
	owner := b.cap.Scope.OwnerID
	in := b.h.input
	in.PaymentMode = "cash_on_delivery"
	key := t04Key("hcr-begin-replay")
	before := e.tppBalance(sku)
	r1, err1 := e.svc.Begin(ctx, b.cap.Token, e.store(), key, in)
	r2, err2 := e.svc.Begin(ctx, b.cap.Token, e.store(), key, in)
	if err1 != nil || err2 != nil || r1.OrderID == "" || r1.OrderID != r2.OrderID || r2.PaymentMode != "cash_on_delivery" || r2.CommercialState != "AWAITING_COLLECTION" {
		t.Fatalf("Begin replay: %+v %v / %+v %v, want the same COD order twice", r1, err1, r2, err2)
	}
	if h, a, o := e.tcbEffects(owner); h != 1 || a != 0 || o != 1 {
		t.Errorf("rows after a Begin replay: holds %d attempts %d orders %d, want 1/0/1", h, a, o)
	}
	if n := e.count(`SELECT count(*) FROM inventory.ledger WHERE reservation_id=$1 AND kind='ALLOCATE'`, r1.OrderID); n != 1 {
		t.Errorf("%d ALLOCATE rows after a Begin replay, want 1", n)
	}
	if after := e.tppBalance(sku); after.allocated != before.allocated+2 {
		t.Errorf("balance %+v -> %+v: a replay must not allocate twice", before, after)
	}
	// the same key with another body (card instead of COD) is refused and creates nothing
	other := in
	other.PaymentMode = ""
	if _, err := e.svc.Begin(ctx, b.cap.Token, e.store(), key, other); err == nil {
		t.Error("the same Idempotency-Key with another body must be refused")
	}
	if _, _, o := e.tcbEffects(owner); o != 1 {
		t.Errorf("%d orders after a conflicting replay, want 1", o)
	}
	// an exact replay after the merchant switched COD off still returns the saved order (no re-validation, no second hold)
	if st, out := e.hcodSettings(1, false, 20000, 50, "black_cat"); st != 200 {
		t.Fatalf("disable COD: %d %v", st, out)
	}
	if r3, err3 := e.svc.Begin(ctx, b.cap.Token, e.store(), key, in); err3 != nil || r3.OrderID != r1.OrderID {
		t.Errorf("replay after the setting was switched off: %+v %v, want the saved order", r3, err3)
	}
	// collection: replay = same answer, one audit row; same key + other body = 409 idempotency_conflict
	e.hcrShip(r1.OrderID)
	audit := e.audit("fulfillment.collection_recorded")
	ck := t04Key("hcr-col-replay")
	st1, out1, _ := e.record(writer, r1.OrderID, ck, "PENDING", "collected")
	st2, out2, _ := e.record(writer, r1.OrderID, ck, "PENDING", "collected")
	if st1 != 200 || st2 != 200 || fmt.Sprint(out1) != fmt.Sprint(out2) {
		t.Errorf("collection replay: %d %v / %d %v, want two identical 200s", st1, out1, st2, out2)
	}
	if got := e.audit("fulfillment.collection_recorded"); got != audit+1 {
		t.Errorf("collection audit rows %d -> %d, want exactly one", audit, got)
	}
	if st, out, raw := e.record(writer, r1.OrderID, ck, "COLLECTED", "refunded_offline"); st != 409 || tcvStr(out, "code") != "idempotency_conflict" {
		t.Errorf("same key, other body: want 409 idempotency_conflict, got %d %s", st, raw)
	}
	if s := e.collectionState(r1.OrderID); s != "COLLECTED" {
		t.Errorf("collection_state %s, want COLLECTED", s)
	}
}

// HCR13: the settings wire contract is strict (unknown, missing, mistyped and out-of-range fields never write), and two writers racing on
// the same expected version produce exactly one new version.
func TestHomeCodReviewSettingsWire(t *testing.T) {
	e := tcvNew(t)
	e.grantCreator("orders:read", "fulfillment:write")
	url := "/v1/admin/stores/" + e.store() + "/cash-on-delivery-settings"
	audit := e.audit("checkout.cash_on_delivery_settings_changed")
	for name, body := range map[string]string{
		"unknown key":              `{"expected_version":0,"enabled":true,"max_twd":100,"surcharge_twd":0,"carrier":"black_cat","surcharge_minor":0}`,
		"missing carrier":          `{"expected_version":0,"enabled":true,"max_twd":100,"surcharge_twd":0}`,
		"missing expected_version": `{"enabled":true,"max_twd":100,"surcharge_twd":0,"carrier":"black_cat"}`,
		"fractional cap":           `{"expected_version":0,"enabled":true,"max_twd":50.5,"surcharge_twd":0,"carrier":"black_cat"}`,
		"string cap":               `{"expected_version":0,"enabled":true,"max_twd":"100","surcharge_twd":0,"carrier":"black_cat"}`,
		"string enabled":           `{"expected_version":0,"enabled":"true","max_twd":100,"surcharge_twd":0,"carrier":"black_cat"}`,
		"negative surcharge":       `{"expected_version":0,"enabled":true,"max_twd":100,"surcharge_twd":-1,"carrier":"black_cat"}`,
		"fractional surcharge":     `{"expected_version":0,"enabled":true,"max_twd":100,"surcharge_twd":0.5,"carrier":"black_cat"}`,
		"negative version":         `{"expected_version":-1,"enabled":true,"max_twd":100,"surcharge_twd":0,"carrier":"black_cat"}`,
		"huge version":             `{"expected_version":9223372036854775807,"enabled":true,"max_twd":100,"surcharge_twd":0,"carrier":"black_cat"}`,
		"carrier case":             `{"expected_version":0,"enabled":true,"max_twd":100,"surcharge_twd":0,"carrier":"BLACK_CAT"}`,
		"empty object":             `{}`,
		"array":                    `[]`,
		"null":                     `null`,
	} {
		if st, _, raw := e.mcall(e.token(), "PUT", url, t04Key("hcr-wire"), body); st < 400 || st >= 500 {
			t.Errorf("%s: want a 4xx refusal, got %d %s", name, st, raw)
		}
	}
	valid := `{"expected_version":0,"enabled":true,"max_twd":100,"surcharge_twd":0,"carrier":"black_cat"}`
	if st, _, raw := e.mcall(e.token(), "PUT", url, "", valid); st < 400 || st >= 500 {
		t.Errorf("no Idempotency-Key: want a 4xx refusal, got %d %s", st, raw)
	}
	if st, _, raw := e.mcall(e.token(), "PUT", url, "short", valid); st < 400 || st >= 500 {
		t.Errorf("malformed Idempotency-Key: want a 4xx refusal, got %d %s", st, raw)
	}
	if st, _, raw := e.mcall(e.token(), "PUT", url+"?enabled=true", t04Key("hcr-wire-q"), valid); st < 400 || st >= 500 {
		t.Errorf("query string on the PUT: want a 4xx refusal, got %d %s", st, raw)
	}
	if st, out, raw := e.mcall(e.token(), "GET", url, "", ""); st != 200 || out["version"] != float64(0) || out["enabled"] != false {
		t.Errorf("settings after refused writes: %d %s, want the untouched version-0 default", st, raw)
	}
	if n := e.count(`SELECT count(*) FROM checkout.cash_on_delivery_settings WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store()); n != 0 {
		t.Errorf("%d settings rows after refused writes, want 0", n)
	}
	if got := e.audit("checkout.cash_on_delivery_settings_changed"); got != audit {
		t.Errorf("settings audit rows %d -> %d after refused writes", audit, got)
	}
	// two writers on the same expected version: exactly one wins
	res := hcrParallel(
		func() hcrResp {
			st, out, raw := e.mcall(e.token(), "PUT", url, t04Key("hcr-cas-a"), `{"expected_version":0,"enabled":true,"max_twd":100,"surcharge_twd":10,"carrier":"black_cat"}`)
			return hcrResp{st, out, raw}
		},
		func() hcrResp {
			st, out, raw := e.mcall(e.token(), "PUT", url, t04Key("hcr-cas-b"), `{"expected_version":0,"enabled":true,"max_twd":200,"surcharge_twd":20,"carrier":"hsinchu"}`)
			return hcrResp{st, out, raw}
		},
	)
	won, lost := 0, 0
	for _, r := range res {
		switch {
		case r.status == 200 && r.out["version"] == float64(1):
			won++
		case r.status == 409 && tcvStr(r.out, "code") == "version_changed":
			lost++
		default:
			t.Errorf("unexpected CAS answer: %d %s", r.status, r.raw)
		}
	}
	if won != 1 || lost != 1 {
		t.Errorf("settings CAS race: %d winners, %d losers, want 1/1", won, lost)
	}
	if n := e.count(`SELECT version FROM checkout.cash_on_delivery_settings WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store()); n != 1 {
		t.Errorf("settings version %d, want 1", n)
	}
}

// HCR14: the table CHECKs hold even for the owner role, so no code path can persist an impossible COD amount or a half-set mode.
func TestHomeCodReviewSchemaInvariants(t *testing.T) {
	e := tcvNew(t)
	ctx := context.Background()
	e.grantCreator("orders:read", "fulfillment:write")
	e.hcrEnable(50)
	order, _ := e.hcrPlace()
	cardless := e.count(`SELECT count(*) FROM checkout.orders WHERE id=$1 AND payment_mode='cash_on_delivery' AND cod_surcharge_minor=5000`, order)
	if cardless != 1 {
		t.Fatalf("setup: COD order with surcharge 5000 missing")
	}
	for _, c := range []struct{ name, stmt string }{
		{"surcharge not a whole dollar", `UPDATE checkout.orders SET cod_surcharge_minor=150 WHERE id=$1`},
		{"negative surcharge", `UPDATE checkout.orders SET cod_surcharge_minor=-100 WHERE id=$1`},
		{"surcharge above NT$1000", `UPDATE checkout.orders SET cod_surcharge_minor=100100 WHERE id=$1`},
		{"COD order without a surcharge", `UPDATE checkout.orders SET cod_surcharge_minor=NULL WHERE id=$1`},
		{"COD order without a collection state", `UPDATE checkout.orders SET collection_state=NULL WHERE id=$1`},
		{"COD order turned into a card order", `UPDATE checkout.orders SET payment_mode='card' WHERE id=$1`},
		{"COD order turned into pay_at_pickup keeps its surcharge", `UPDATE checkout.orders SET payment_mode='pay_at_pickup' WHERE id=$1`},
		{"unknown commercial state", `UPDATE checkout.orders SET commercial_state='AWAITING_CASH' WHERE id=$1`},
		{"unknown payment mode", `UPDATE checkout.orders SET payment_mode='cod' WHERE id=$1`},
		{"settings cap 0", `UPDATE checkout.cash_on_delivery_settings SET max_twd=0 WHERE store_id=(SELECT store_id FROM checkout.orders WHERE id=$1)`},
		{"settings cap above NT$20000", `UPDATE checkout.cash_on_delivery_settings SET max_twd=20001 WHERE store_id=(SELECT store_id FROM checkout.orders WHERE id=$1)`},
		{"settings surcharge above NT$1000", `UPDATE checkout.cash_on_delivery_settings SET surcharge_twd=1001 WHERE store_id=(SELECT store_id FROM checkout.orders WHERE id=$1)`},
		{"settings carrier outside the vocabulary", `UPDATE checkout.cash_on_delivery_settings SET carrier='dhl' WHERE store_id=(SELECT store_id FROM checkout.orders WHERE id=$1)`},
	} {
		tx, err := e.p.f.owner.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if st := tcvExecState(ctx, tx, c.stmt, order); st != "23514" && st != "23502" {
			t.Errorf("%s: want a CHECK/NOT NULL violation, got SQLSTATE %q", c.name, st)
		}
		tx.Rollback(ctx)
	}
}

// HCR15: inventory.guard_pay_at_pickup_ledger was widened for COD; it must not over-admit. Direct ledger writes as the checkout writer
// are refused (42501) unless the order is in exactly the state the legitimate writer produces.
func TestHomeCodReviewLedgerGuard(t *testing.T) {
	e := tcvNew(t)
	f := e.p.f
	ctx := context.Background()
	e.grantCreator("orders:read", "fulfillment:write")
	e.hcrEnable(0)
	writer, writerPrincipal := e.member("fulfillment:write", "orders:read")
	line := func(order string) (owner, session, wh, skuID string, qty int64) {
		if err := f.owner.QueryRow(ctx, `SELECT o.owner_id::text,o.creator_session_id::text,l.warehouse_id::text,l.sku_id::text,l.quantity FROM checkout.orders o JOIN inventory.reservation_lines l ON l.tenant_id=o.tenant_id AND l.store_id=o.store_id AND l.reservation_id=o.id WHERE o.id=$1`, order).
			Scan(&owner, &session, &wh, &skuID, &qty); err != nil {
			t.Fatalf("order lines: %v", err)
		}
		return
	}
	// try runs one INSERT as the checkout writer with the GUCs the definers set, and returns the SQLSTATE ("" = it would have succeeded).
	try := func(order, kind, actor, op, reason string, principal string) string {
		owner, session, wh, skuID, qty := line(order)
		tx, err := f.owner.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		if _, err := tx.Exec(ctx, `SET LOCAL ROLE commerce_checkout_writer`); err != nil {
			t.Fatalf("set role: %v", err)
		}
		for k, v := range map[string]string{"app.tenant_id": f.tenantA, "app.store_id": f.storeA1, "app.principal_id": principal, "app.buyer_id": owner, "app.buyer_session_id": session} {
			if _, err := tx.Exec(ctx, `SELECT set_config($1,$2,true)`, k, v); err != nil {
				t.Fatal(err)
			}
		}
		var stmt string
		if kind == "ALLOCATE" {
			stmt = fmt.Sprintf(`INSERT INTO inventory.ledger(tenant_id,store_id,warehouse_id,sku_id,kind,delta_reserved,delta_allocated,operation,command_key,reservation_id,principal_id,checkout_id,buyer_owner_id,buyer_session_id,actor_kind)
			  VALUES('%s','%s','%s','%s','ALLOCATE',%d,%d,'%s','%s','%s',NULL,'%s','%s','%s','%s')`, f.tenantA, f.storeA1, wh, skuID, -qty, qty, op, order+"-x", order, order, owner, session, actor)
		} else {
			stmt = fmt.Sprintf(`INSERT INTO inventory.ledger(tenant_id,store_id,warehouse_id,sku_id,kind,delta_allocated,operation,command_key,reservation_id,reason,principal_id,checkout_id,buyer_owner_id,buyer_session_id,actor_kind)
			  VALUES('%s','%s','%s','%s','DEALLOCATE',%d,'%s','%s','%s','%s','%s','%s','%s','%s','%s')`, f.tenantA, f.storeA1, wh, skuID, -qty, op, order+"-y", order, reason, principal, order, owner, session, actor)
		}
		st, _ := tcsSub(ctx, tx, stmt)
		return st
	}
	// a PENDING order: the cancel/restock DEALLOCATE rows need CANCELLED/RESTOCKED first
	pending, _ := e.hcrPlace()
	if st := try(pending, "DEALLOCATE", "MERCHANT", "fulfillment.pay_at_pickup.cancel", "PENDING", writerPrincipal); st != "42501" {
		t.Errorf("cancel DEALLOCATE of a PENDING COD order: want 42501, got %q", st)
	}
	if st := try(pending, "DEALLOCATE", "MERCHANT", "fulfillment.pay_at_pickup.restock", "RETURNED", writerPrincipal); st != "42501" {
		t.Errorf("restock DEALLOCATE of a PENDING COD order: want 42501, got %q", st)
	}
	if st := try(pending, "DEALLOCATE", "MERCHANT", "checkout.bank_transfer.refund_restock", "REFUNDED_OFFLINE", writerPrincipal); st != "42501" {
		t.Errorf("the bank-transfer restock operation on a COD order: want 42501, got %q", st)
	}
	if st := try(pending, "DEALLOCATE", "BUYER", "fulfillment.pay_at_pickup.cancel", "PENDING", writerPrincipal); st == "" {
		t.Error("a BUYER-actor DEALLOCATE must never be admitted")
	}
	// a COLLECTED order must not accept a fresh BUYER ALLOCATE (collection_state must be PENDING)
	collected, _ := e.hcrPlace()
	e.hcrShip(collected)
	if r := e.hcrRecord(writer, collected, "PENDING", "collected"); r.status != 200 {
		t.Fatalf("collect: %d %s", r.status, r.raw)
	}
	if st := try(collected, "ALLOCATE", "BUYER", "checkout.pay_at_pickup.commit", "", writerPrincipal); st != "42501" {
		t.Errorf("BUYER ALLOCATE on a COLLECTED COD order: want 42501, got %q", st)
	}
	// a cancelled order must not accept one either
	cancelled, _ := e.hcrPlace()
	if r := e.hcrRelease(writer, cancelled, "cancel", "PENDING"); r.status != 200 {
		t.Fatalf("cancel: %d %s", r.status, r.raw)
	}
	if st := try(cancelled, "ALLOCATE", "BUYER", "checkout.pay_at_pickup.commit", "", writerPrincipal); st != "42501" {
		t.Errorf("BUYER ALLOCATE on a CANCELLED COD order: want 42501, got %q", st)
	}
	// a second DEALLOCATE for an already released line is refused by the one-per-line unique index (whatever the key)
	if st := try(cancelled, "DEALLOCATE", "MERCHANT", "fulfillment.pay_at_pickup.cancel", "PENDING", writerPrincipal); st == "" {
		t.Error("a second DEALLOCATE row for the same order line must be refused (ledger_pay_at_pickup_release_once)")
	}
}
