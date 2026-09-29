package foundation_test

// TCV15 TestCvsPayAtPickupBegin, TCV16 TestCvsCollectionStatus, TCV17 TestCvsPayAtPickupRelease (contracts/taiwan-cvs-logistics-v1.md §10, §16.2-§16.5,
// §16.8, §6 last two rows, §7.4/§7.5). Prefix `tpp`. Tier REAL_PG + HTTP_PG (+ MOCK ECPay for TCV16/TCV17: the in-process dispatcher over the
// ecpaytest fake). Definers: checkout.begin_hold (pay_at_pickup branch), fulfillment.record_collection, fulfillment.ingest_ecpay_status,
// inventory.release_pay_at_pickup, fulfillment.request_cvs_shipment, fulfillment.order_money_shippable.
// Owner-pool writes (disclosed fixtures): identity grants (tcvEnv.member), aging checkout.orders.expires_at (the expiry-job case), and direct
// negative INSERTs into inventory.ledger inside rolled-back transactions (the ledger guards). Nothing else is fabricated.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"testing"

	"livecommerce/internal/checkout"
	"livecommerce/internal/storefront"
)

const tppName, tppPhone = "王小明", "0912345678"

// cvsSettings PUTs the store settings and tracks the version.
func (e *tcvEnv) cvsSettings(chains string, on bool, capTWD string, maxOpen int) {
	e.t.Helper()
	body := fmt.Sprintf(`{"expected_version":%d,"enabled_chains":%s,"pay_at_pickup_enabled":%v,"pay_at_pickup_max_twd":%s,"pay_at_pickup_max_open":%d}`, e.settingsVer, chains, on, capTWD, maxOpen)
	st, out, raw := e.mcall(e.token(), "PUT", "/v1/admin/stores/"+e.store()+"/logistics/cvs-settings", t04Key("tpp-settings"), body)
	if st != 200 {
		e.t.Fatalf("cvs-settings v%d: %d %s", e.settingsVer, st, raw)
	}
	if v, ok := out["version"].(float64); ok {
		e.settingsVer = int64(v)
	} else {
		e.settingsVer++
	}
}

const tcvAllChains = `["cvs_711","cvs_familymart","cvs_hilife","cvs_okmart"]`

// tppEntered enters a buyer store (buyer_entered mode: the store has no ECPay profile) and returns the pickup id.
func (e *tcvEnv) tppEntered(b *tcvBuyer, code string) string {
	e.t.Helper()
	res := b.enteredStore(code, "123456", "取貨門市", "台北市取貨路1號")
	if res.status != 201 {
		e.t.Fatalf("enter store: %d %s", res.status, res.body)
	}
	return tcvStr(tcvJSON(e.t, res.body), "pickup_id")
}

func (e *tcvEnv) tppPlace(b *tcvBuyer, code, pickup, name, phone string) (checkout.Result, error) {
	return e.tcbTry(b, "cvs_711", code, pickup, name, phone, "pay_at_pickup")
}

type tppBalance struct{ onHand, reserved, allocated int64 }

func (e *tcvEnv) tppBalance(sku string) tppBalance {
	var b tppBalance
	if err := e.p.f.owner.QueryRow(context.Background(), `SELECT on_hand,reserved,allocated FROM inventory.balances WHERE tenant_id=$1 AND store_id=$2 AND sku_id=$3`, e.tenant(), e.store(), sku).Scan(&b.onHand, &b.reserved, &b.allocated); err != nil {
		e.t.Fatal(err)
	}
	return b
}

func TestCvsPayAtPickupBegin(t *testing.T) {
	e := tcvNew(t, tcvOpts{stripe: true})
	f := e.p.f
	ctx := context.Background()
	e.r.startWorker(t)
	e.grantCreator("orders:read", "fulfillment:write", "integration:manage")
	code, _, _ := e.service("cvs_711", "MANUAL", 0)
	e.cvsSettings(tcvAllChains, true, "20000", 500)
	sku := e.p.stock.skus[0].ID

	t.Run("Begin writes: CONFIRMED order, COMMITTED reservation, RESERVE+ALLOCATE ledger rows, no payment", func(t *testing.T) {
		b := e.newBuyer()
		pickup := e.tppEntered(b, code)
		before := e.tppBalance(sku)
		stripeBefore := e.count(`SELECT count(*) FROM payments.stripe_sessions`)
		stripeReqBefore := len(e.r.fake.Requests())
		res, err := e.tppPlace(b, code, pickup, tppName, tppPhone)
		if err != nil {
			t.Fatalf("pay-at-pickup Begin: %v", err)
		}
		if res.PaymentMode != "pay_at_pickup" || res.CommercialState != "CONFIRMED" {
			t.Errorf("result must carry payment_mode and commercial_state: %+v", res)
		}
		var commercial, fulfilment, mode string
		var collection *string
		if err := f.owner.QueryRow(ctx, `SELECT commercial_state,fulfillment_state,payment_mode,collection_state FROM checkout.orders WHERE id=$1`, res.OrderID).Scan(&commercial, &fulfilment, &mode, &collection); err != nil {
			t.Fatal(err)
		}
		if commercial != "CONFIRMED" || fulfilment != "MANUAL_UNASSIGNED" || mode != "pay_at_pickup" || collection == nil || *collection != "PENDING" {
			t.Errorf("order row: %s/%s/%s/%v", commercial, fulfilment, mode, collection)
		}
		if n := e.count(`SELECT count(*) FROM inventory.reservations WHERE id=$1 AND state='COMMITTED'`, res.ReservationID); n != 1 {
			t.Error("the reservation is COMMITTED (HELD -> COMMITTED, section 11.5) at placement")
		}
		if n := e.count(`SELECT count(*) FROM inventory.ledger WHERE reservation_id=$1 AND kind='RESERVE'`, res.ReservationID); n != 1 {
			t.Errorf("RESERVE rows: %d", n)
		}
		var actor, op, key string
		var dRes, dAlloc, dHand int64
		var attempt, factKind *string
		var n int
		rows, err := f.owner.Query(ctx, `SELECT actor_kind,operation,command_key,delta_reserved,delta_allocated,delta_on_hand,payment_attempt_id::text,payment_fact_kind FROM inventory.ledger WHERE reservation_id=$1 AND kind='ALLOCATE'`, res.ReservationID)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			n++
			_ = rows.Scan(&actor, &op, &key, &dRes, &dAlloc, &dHand, &attempt, &factKind)
		}
		rows.Close()
		if n != 1 || actor != "BUYER" || op != "checkout.pay_at_pickup.commit" || key != res.OrderID || dRes != -2 || dAlloc != 2 || dHand != 0 || attempt != nil || factKind != nil {
			t.Errorf("ALLOCATE rows=%d actor=%s op=%s key=%s deltas res/alloc/hand=%d/%d/%d attempt=%v fact=%v", n, actor, op, key, dRes, dAlloc, dHand, attempt, factKind)
		}
		after := e.tppBalance(sku)
		if after.reserved != before.reserved || after.allocated != before.allocated+2 || after.onHand != before.onHand {
			t.Errorf("balance %+v -> %+v (RESERVE then ALLOCATE nets reserved 0 and allocated +2)", before, after)
		}
		for _, action := range []string{"checkout.held", "checkout.pay_at_pickup_placed"} {
			if n := e.count(`SELECT count(*) FROM checkout.events WHERE order_id=$1 AND action=$2 AND actor_kind='BUYER'`, res.OrderID, action); n != 1 {
				t.Errorf("event %s: %d rows", action, n)
			}
		}
		if n := e.count(`SELECT count(*) FROM checkout.payment_attempts WHERE order_id=$1`, res.OrderID); n != 0 {
			t.Errorf("%d payment attempts for a pay-at-pickup order", n)
		}
		if got := e.count(`SELECT count(*) FROM payments.stripe_sessions`); got != stripeBefore {
			t.Errorf("Stripe sessions %d -> %d: pay-at-pickup never touches Stripe", stripeBefore, got)
		}
		if n := e.count(`SELECT count(*) FROM payments.stripe_sessions WHERE owner_id=$1`, b.cap.Scope.OwnerID); n != 0 {
			t.Errorf("%d payments.stripe_sessions rows for the buyer", n)
		}
		if got := len(e.r.fake.Requests()); got != stripeReqBefore {
			t.Errorf("Stripe requests %d -> %d: pay-at-pickup never calls Stripe", stripeReqBefore, got)
		}

		// start_payment (PayUni mock) and start_stripe_payment both refuse a non-DRAFT order
		if _, err := e.p.starter.StartPayment(ctx, b.cap.Token, e.store(), t04Key("tpp-start"), checkout.PaymentInput{OrderID: res.OrderID, MethodCode: "payuni_credit", MethodVersion: 1}); err == nil {
			t.Error("checkout.start_payment must refuse a pay-at-pickup (non-DRAFT) order")
		}
		s := e.ro.s
		s.p.hold = res
		if _, err := s.begin(e.r.svc, t04Key("tpp-stripe"), s.input("zh-TW")); err == nil {
			t.Error("checkout.start_stripe_payment must refuse a pay-at-pickup (non-DRAFT) order")
		}
		if n := e.count(`SELECT count(*) FROM checkout.payment_attempts WHERE order_id=$1`, res.OrderID); n != 0 {
			t.Errorf("a refused payment start left %d attempts", n)
		}
		if n := e.count(`SELECT count(*) FROM payments.stripe_sessions WHERE owner_id=$1`, b.cap.Scope.OwnerID); n != 0 {
			t.Errorf("a refused Stripe start left %d sessions", n)
		}

		// the expiry job reaches STALE and releases nothing (disclosed owner-pool fixture: created_at and expires_at move together so the job is due)
		mustExec(t, f.owner, `UPDATE checkout.orders SET created_at=created_at-interval '3 hours',expires_at=expires_at-interval '3 hours' WHERE id=$1`, res.OrderID)
		var disposition string
		if err := e.p.worker.QueryRow(ctx, `SELECT disposition FROM checkout.expire_held($1::uuid,$2::bigint)`, res.OrderID, res.Generation).Scan(&disposition); err != nil {
			t.Fatalf("expire_held: %v", err)
		}
		if disposition != "STALE" {
			t.Errorf("expiry of a CONFIRMED pay-at-pickup order: %s, want STALE", disposition)
		}
		if got := e.tppBalance(sku); got != after {
			t.Errorf("expiry released stock: %+v -> %+v", after, got)
		}
		if n := e.count(`SELECT count(*) FROM inventory.reservations WHERE id=$1 AND state='COMMITTED'`, res.ReservationID); n != 1 {
			t.Error("the reservation must stay COMMITTED after the expiry job")
		}
	})

	t.Run("amount cap: total = cap accepted, +NT$1 / non-whole / > NT$20000 refused (pay_at_pickup_amount_exceeds)", func(t *testing.T) {
		e.cvsSettings(tcvAllChains, true, "100", 500) // NT$100 cap = 10000 minor
		exact, over, half := e.sku(10000, 10), e.sku(10100, 10), e.sku(50, 10)
		try := func(label string, items []storefront.Item, wantErr string) {
			b := e.newBuyer(items...)
			pickup := e.tppEntered(b, code)
			h0, a0, o0 := e.tcbEffects(b.cap.Scope.OwnerID)
			_, err := e.tppPlace(b, code, pickup, tppName, tppPhone)
			if wantErr == "" {
				if err != nil {
					t.Errorf("%s: %v", label, err)
				}
				return
			}
			tcvExpectRefusal(t, label, err, 422, wantErr)
			if h, a, o := e.tcbEffects(b.cap.Scope.OwnerID); h != h0 || a != a0 || o != o0 {
				t.Errorf("%s: a refused Begin left holds/attempts/orders %d/%d/%d -> %d/%d/%d", label, h0, a0, o0, h, a, o)
			}
		}
		try("total = pay_at_pickup_max_twd x100", []storefront.Item{{SKUID: exact, Quantity: 1}}, "")
		try("cap + NT$1", []storefront.Item{{SKUID: over, Quantity: 1}}, "pay_at_pickup_amount_exceeds")
		try("non-whole TWD (NT$0.50)", []storefront.Item{{SKUID: half, Quantity: 1}}, "pay_at_pickup_amount_exceeds")
		e.cvsSettings(tcvAllChains, true, "20000", 500)
		big, one := e.sku(200000, 30), e.sku(100, 5)
		try("NT$20000 (F20 ceiling) accepted", []storefront.Item{{SKUID: big, Quantity: 10}}, "")
		try("NT$20001 refused whatever the store cap", []storefront.Item{{SKUID: big, Quantity: 10}, {SKUID: one, Quantity: 1}}, "pay_at_pickup_amount_exceeds")
	})

	t.Run("unavailable: setting off, chain not enabled, home destination -> pay_at_pickup_unavailable with zero holds", func(t *testing.T) {
		try := func(label string, place func(b *tcvBuyer) error) {
			b := e.newBuyer()
			h0, a0, o0 := e.tcbEffects(b.cap.Scope.OwnerID)
			err := place(b)
			tcvExpectRefusal(t, label, err, 422, "pay_at_pickup_unavailable")
			if h, a, o := e.tcbEffects(b.cap.Scope.OwnerID); h != h0 || a != a0 || o != o0 {
				t.Errorf("%s: holds/attempts/orders %d/%d/%d -> %d/%d/%d", label, h0, a0, o0, h, a, o)
			}
		}
		e.cvsSettings(tcvAllChains, false, "null", 500)
		try("pay-at-pickup disabled", func(b *tcvBuyer) error {
			_, err := e.tppPlace(b, code, e.tppEntered(b, code), tppName, tppPhone)
			return err
		})
		e.cvsSettings(`["cvs_familymart"]`, true, "20000", 500)
		try("destination chain not in enabled_chains", func(b *tcvBuyer) error {
			// the buyer cannot even enter a store for a disabled chain (record_buyer_cvs_store refuses); Begin must still refuse a pickup
			// entered while the chain was enabled: enter first, then narrow the settings
			e.cvsSettings(tcvAllChains, true, "20000", 500)
			pickup := e.tppEntered(b, code)
			e.cvsSettings(`["cvs_familymart"]`, true, "20000", 500)
			_, err := e.tppPlace(b, code, pickup, tppName, tppPhone)
			return err
		})
		e.cvsSettings(tcvAllChains, true, "20000", 500)
		try("home destination", func(b *tcvBuyer) error {
			in := b.h.input // newBuyer prepared a cart, a home destination and a home quote
			in.PaymentMode = "pay_at_pickup"
			_, err := e.svc.Begin(ctx, b.cap.Token, e.store(), t04Key("tpp-home"), in)
			return err
		})
	})

	t.Run("C3 recipient rule: golden table through SQL and through Begin", func(t *testing.T) {
		raw, err := os.ReadFile("../integrations/ecpay/testdata/recipient.json")
		if err != nil {
			t.Fatal(err)
		}
		var rows []struct {
			Why             string `json:"why"`
			Name            string `json:"name"`
			Phone           string `json:"phone"`
			OK              bool   `json:"ok"`
			PhoneNormalized string `json:"phone_normalized"`
		}
		if err := json.Unmarshal(raw, &rows); err != nil {
			t.Fatal(err)
		}
		for _, r := range rows {
			var got bool
			if err := f.owner.QueryRow(ctx, `SELECT fulfillment.ecpay_recipient_ok($1,$2)`, r.Name, r.Phone).Scan(&got); err != nil {
				t.Fatalf("ecpay_recipient_ok(%q,%q): %v", r.Name, r.Phone, err)
			}
			if got != r.OK {
				t.Errorf("SQL twin, %s: fulfillment.ecpay_recipient_ok(%q,%q)=%v want %v", r.Why, r.Name, r.Phone, got, r.OK)
			}
		}
		e.cvsSettings(tcvAllChains, true, "20000", 500)
		place := func(name, phone string) error {
			b := e.newBuyer()
			pickup := e.tppEntered(b, code)
			_, err := e.tppPlace(b, code, pickup, name, phone)
			return err
		}
		for _, ok := range [][2]string{{"王小明", "0912345678"}, {"王小明", "+886912345678"}, {"王小明", "09 1234-5678"}} {
			if err := place(ok[0], ok[1]); err != nil {
				t.Errorf("recipient %q %q must be accepted: %v", ok[0], ok[1], err)
			}
		}
		for label, bad := range map[string][2]string{"landline": {"王小明", "0212345678"}, "08 prefix": {"王小明", "0812345678"}, "name with digit": {"王小明1", "0912345678"}, "one-char name": {"王", "0912345678"},
			"emoji name": {"王小明😀", "0912345678"}, "short phone": {"王小明", "091234567"}} {
			b := e.newBuyer()
			pickup := e.tppEntered(b, code)
			h0, a0, o0 := e.tcbEffects(b.cap.Scope.OwnerID)
			_, err := e.tppPlace(b, code, pickup, bad[0], bad[1])
			if err == nil {
				t.Errorf("%s: a recipient outside the ECPay rule must be refused for pay-at-pickup", label)
			}
			if h, a, o := e.tcbEffects(b.cap.Scope.OwnerID); h != h0 || a != a0 || o != o0 {
				t.Errorf("%s: holds/attempts/orders %d/%d/%d -> %d/%d/%d", label, h0, a0, o0, h, a, o)
			}
		}
	})

	t.Run("shippability, manual shipment and refunds", func(t *testing.T) {
		b := e.newBuyer()
		res, err := e.tppPlace(b, code, e.tppEntered(b, code), tppName, tppPhone)
		if err != nil {
			t.Fatal(err)
		}
		var shippable bool
		if err := f.owner.QueryRow(ctx, `SELECT fulfillment.order_money_shippable($1::uuid,$2::uuid,$3::uuid)`, f.tenantA, f.storeA1, res.OrderID).Scan(&shippable); err != nil || !shippable {
			t.Errorf("order_money_shippable for a CONFIRMED PENDING pay-at-pickup order: %v %v", shippable, err)
		}
		// the Stripe refund request refuses (no CAPTURED attempt) and writes nothing
		refundsBefore := e.count(`SELECT count(*) FROM payments.stripe_refunds`)
		st, _, raw := e.mcall(e.token(), "POST", "/v1/admin/stores/"+e.store()+"/orders/"+res.OrderID+"/refunds", t04Key("tpp-refund"), rfxBody(100, "requested_by_customer", 100))
		if st < 400 {
			t.Errorf("a refund request for a pay-at-pickup order must be refused, got %d %s", st, raw)
		}
		if got := e.count(`SELECT count(*) FROM payments.stripe_refunds`); got != refundsBefore {
			t.Errorf("%d payments.stripe_refunds rows written for a pay-at-pickup order", got-refundsBefore)
		}
		// manual shipment (0063) is allowed
		st, _, raw = e.mcall(e.token(), "PUT", "/v1/admin/stores/"+e.store()+"/orders/"+res.OrderID+"/shipment", t04Key("tpp-manual"), mfxShip(0, "seven_eleven_cvs", "0012345678"))
		if st != 200 {
			t.Errorf("0063 manual shipment of a CONFIRMED pay-at-pickup order: %d %s", st, raw)
		}
		if n := e.count(`SELECT count(*) FROM checkout.orders WHERE id=$1 AND fulfillment_state='MERCHANT_SHIPPED'`, res.OrderID); n != 1 {
			t.Error("the order must be MERCHANT_SHIPPED after the manual record")
		}
	})

	t.Run("ledger guard: a BUYER ALLOCATE for a card order is refused (42501)", func(t *testing.T) {
		hold := e.ro.s.p.hold // the DRAFT card order of the rfx store
		var owner, session, wh, skuID string
		var qty int64
		if err := f.owner.QueryRow(ctx, `SELECT o.owner_id::text,o.creator_session_id::text,l.warehouse_id::text,l.sku_id::text,l.quantity FROM checkout.orders o JOIN inventory.reservation_lines l ON l.tenant_id=o.tenant_id AND l.store_id=o.store_id AND l.reservation_id=o.id WHERE o.id=$1`, hold.OrderID).
			Scan(&owner, &session, &wh, &skuID, &qty); err != nil {
			t.Fatalf("fixture order lines: %v", err)
		}
		tx, err := f.owner.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		st, _ := tcsSub(ctx, tx, `INSERT INTO inventory.ledger(tenant_id,store_id,warehouse_id,sku_id,kind,delta_reserved,delta_allocated,operation,command_key,reservation_id,actor_kind,checkout_id,buyer_owner_id,buyer_session_id,principal_id)
		  VALUES($1::uuid,$2::uuid,$3::uuid,$4::uuid,'ALLOCATE',$5::bigint,$6::bigint,'checkout.pay_at_pickup.commit',$7::text,$7::uuid,'BUYER',$7::uuid,$8::uuid,$9::uuid,NULL)`, f.tenantA, f.storeA1, wh, skuID, -qty, qty, hold.OrderID, owner, session)
		if st != "42501" {
			t.Errorf("a BUYER ALLOCATE row for a card order: want 42501 from inventory.guard_pay_at_pickup_ledger, got %q", st)
		}
	})

	t.Run("open-order limit: max_open store-wide, one per owner, two concurrent Begins at max_open-1 yield one order (round 4)", func(t *testing.T) {
		// a fresh store keeps the open-order count exact
		e2 := tcvNew(t)
		e2.grantCreator("orders:read", "fulfillment:write", "integration:manage")
		code2, _, _ := e2.service("cvs_711", "MANUAL", 0)
		e2.cvsSettings(tcvAllChains, true, "20000", 2)
		a, b := e2.newBuyer(), e2.newBuyer()
		var resA checkout.Result
		var err error
		if resA, err = e2.tppPlace(a, code2, e2.tppEntered(a, code2), tppName, tppPhone); err != nil {
			t.Fatalf("first open order: %v", err)
		}
		if _, err = e2.tppPlace(b, code2, e2.tppEntered(b, code2), tppName, tppPhone); err != nil {
			t.Fatalf("second open order: %v", err)
		}
		c := e2.newBuyer()
		pickupC := e2.tppEntered(c, code2)
		h0, at0, o0 := e2.tcbEffects(c.cap.Scope.OwnerID)
		_, err = e2.tppPlace(c, code2, pickupC, tppName, tppPhone)
		tcvExpectRefusal(t, "the (max_open+1)th open order", err, 429, "pay_at_pickup_limit")
		if h, at, o := e2.tcbEffects(c.cap.Scope.OwnerID); h != h0 || at != at0 || o != o0 {
			t.Errorf("a refused order left rows %d/%d/%d -> %d/%d/%d", h0, at0, o0, h, at, o)
		}
		// one open order per buyer owner, whatever the store limit
		e2.cvsSettings(tcvAllChains, true, "20000", 500)
		a.recart()
		pickupA2 := e2.tppEntered(a, code2)
		_, err = e2.tppPlace(a, code2, pickupA2, tppName, tppPhone)
		tcvExpectRefusal(t, "a second open order of the same buyer", err, 429, "pay_at_pickup_limit")
		_ = resA
		// concurrency: at max_open-1 open orders two simultaneous Begins produce exactly one order
		e2.cvsSettings(tcvAllChains, true, "20000", 3) // open = 2 = max_open-1
		type ready struct {
			b     *tcvBuyer
			dest  storefront.Destination
			quote storefront.Quote
		}
		var racers []ready
		for i := 0; i < 2; i++ {
			rb := e2.newBuyer()
			pk := e2.tppEntered(rb, code2)
			dest, err := rb.destination("cvs_711", pk, tppName, tppPhone)
			if err != nil {
				t.Fatal(err)
			}
			quote, err := rb.quote(code2)
			if err != nil {
				t.Fatal(err)
			}
			racers = append(racers, ready{rb, dest, quote})
		}
		var wg sync.WaitGroup
		start := make(chan struct{})
		results := make([]error, len(racers))
		for i := range racers {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				_, results[i] = racers[i].b.begin(racers[i].dest, racers[i].quote, e2.svcVer[code2], "pay_at_pickup")
			}(i)
		}
		close(start)
		wg.Wait()
		ok, limited := 0, 0
		for _, err := range results {
			if err == nil {
				ok++
			} else if s, c := tcvRefusal(err); s == 429 && c == "pay_at_pickup_limit" {
				limited++
			} else {
				t.Errorf("unexpected concurrent Begin result: %v", err)
			}
		}
		if ok != 1 || limited != 1 {
			t.Errorf("two concurrent Begins at max_open-1: %d succeeded, %d limited (want exactly one of each)", ok, limited)
		}
		if n := e2.count(`SELECT count(*) FROM checkout.orders WHERE store_id=$1 AND payment_mode='pay_at_pickup' AND collection_state='PENDING'`, e2.store()); n != 3 {
			t.Errorf("open orders after the race: %d, want exactly max_open=3", n)
		}
	})

	t.Run("payment_mode is a closed vocabulary", func(t *testing.T) {
		b := e.newBuyer()
		pickup := e.tppEntered(b, code)
		h0, a0, o0 := e.tcbEffects(b.cap.Scope.OwnerID)
		if _, err := e.tcbTry(b, "cvs_711", code, pickup, tppName, tppPhone, "cash"); err == nil {
			t.Error("an unknown payment_mode must be refused")
		}
		if h, a, o := e.tcbEffects(b.cap.Scope.OwnerID); h != h0 || a != a0 || o != o0 {
			t.Errorf("unknown payment_mode left rows: %d/%d/%d -> %d/%d/%d", h0, a0, o0, h, a, o)
		}
	})
}
