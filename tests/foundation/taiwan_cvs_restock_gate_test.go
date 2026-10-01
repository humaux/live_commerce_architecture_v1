package foundation_test

// TCV17b TestCvsRestockGuardT2101: independent gate for amendment T21-01 (contracts/taiwan-cvs-logistics-v1.md §16 "Amendment T21-01",
// §16.4 status table, §6 code table; migration 0083). Written from the contract, not from the implementation.
// Tier REAL_PG + HTTP_PG + MOCK ECPay (ecpaytest fake, signed status posts). Definers under test: fulfillment.cvs_parcel_returned (the single
// predicate), fulfillment.ingest_ecpay_status (2098 revert), fulfillment.record_collection (returned), inventory.release_pay_at_pickup (restock).
// Owner-pool writes (disclosed fixtures, same as TestCvsPayAtPickupRelease): identity grants, and planting checkout.orders.collection_state /
// fulfillment_state to reach a state no event sequence reaches any more (defence in depth of the predicate). Nothing else is fabricated.
// Every stock claim is numeric: balances (on_hand/reserved/allocated) and ledger rows are compared before/after.

import (
	"context"
	"fmt"
	"testing"

	"livecommerce/internal/integrations/shipping/ecpay/ecpaytest"
)

func TestCvsRestockGuardT2101(t *testing.T) {
	e := tcvNew(t, tcvOpts{stripe: true})
	f := e.p.f
	ctx := context.Background()
	e.r.startWorker(t)
	e.startDispatcher()
	e.grantCreator("orders:read", "fulfillment:write", "integration:manage", "integration:read")
	man, _, _ := e.service("cvs_711", "MANUAL", 0)
	e.cvsSettings(tcvAllChains, true, "20000", 500)
	sku := e.p.stock.skus[0].ID
	writer, _ := e.member("fulfillment:write", "orders:read")
	const qty = int64(2) // newBuyer's default cart: 2 x skus[0]

	bal := e.tppBalance
	ledgerRows := func(order string) int {
		return e.count(`SELECT count(*) FROM inventory.ledger WHERE reservation_id=$1`, order)
	}
	resState := func(order string) (s string) {
		if err := f.owner.QueryRow(ctx, `SELECT state FROM inventory.reservations WHERE id=$1`, order).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return
	}
	lineQty := func(order string) (n int64) {
		if err := f.owner.QueryRow(ctx, `SELECT coalesce(sum(quantity),0) FROM inventory.reservation_lines WHERE reservation_id=$1`, order).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return
	}
	parcelReturned := func(order string) (b bool) {
		if err := f.owner.QueryRow(ctx, `SELECT fulfillment.cvs_parcel_returned($1,$2,$3)`, e.tenant(), e.store(), order).Scan(&b); err != nil {
			t.Fatalf("cvs_parcel_returned: %v", err)
		}
		return
	}
	plant := func(order, set string) { mustExec(t, f.owner, `UPDATE checkout.orders SET `+set+` WHERE id=$1`, order) }
	code := func(raw []byte) string { return tcvStr(tcvJSON(t, raw), "code") }
	wantBal := func(label string, got, want tppBalance) {
		t.Helper()
		if got != want {
			t.Errorf("%s: balance %+v, want %+v", label, got, want)
		}
	}
	restock := func(order, key string) (int, map[string]any, []byte) {
		return e.release(writer, order, key+":"+order, "restock", "RETURNED") // stable per (key, order): replay-able
	}
	// refused: the call must be 409 wantCode and change nothing (balance, ledger rows, reservation, collection state).
	refused := func(label string, st int, raw []byte, wantCode, order string, balBefore tppBalance, rowsBefore int, collBefore string) {
		t.Helper()
		if st != 409 || code(raw) != wantCode {
			t.Errorf("%s: want 409 %s, got %d %s", label, wantCode, st, raw)
		}
		wantBal(label+" (refused call changes no stock)", bal(sku), balBefore)
		if n := ledgerRows(order); n != rowsBefore {
			t.Errorf("%s: ledger rows %d -> %d", label, rowsBefore, n)
		}
		if n := len(e.deallocRows(order)); n != 0 && collBefore != "RESTOCKED" {
			t.Errorf("%s: %d DEALLOCATE rows, want 0", label, n)
		}
		if got := e.collectionState(order); got != collBefore {
			t.Errorf("%s: collection %s -> %s", label, collBefore, got)
		}
	}
	// restocked: 200 RESTOCKED, exactly one DEALLOCATE row per line summing to -qty, allocated falls by qty, on_hand/reserved untouched.
	restocked := func(label, order string) {
		t.Helper()
		if q := lineQty(order); q != qty {
			t.Fatalf("%s: reservation line quantity %d, want %d", label, q, qty)
		}
		before, rows := bal(sku), ledgerRows(order)
		st, out, raw := restock(order, "r-"+label)
		if st != 200 || tcvStr(out, "collection_state") != "RESTOCKED" || e.collectionState(order) != "RESTOCKED" {
			t.Fatalf("%s: restock %d %s (collection %s)", label, st, raw, e.collectionState(order))
		}
		wantBal(label+" after restock", bal(sku), tppBalance{onHand: before.onHand, reserved: before.reserved, allocated: before.allocated - qty})
		d := e.deallocRows(order)
		if len(d) != 1 || d[0].allocated != -qty || d[0].onHand != 0 || d[0].reserved != 0 || d[0].unavailable != 0 || d[0].op != "fulfillment.pay_at_pickup.restock" || d[0].reason != "RETURNED" {
			t.Errorf("%s: DEALLOCATE rows %+v", label, d)
		}
		if n := ledgerRows(order); n != rows+1 || resState(order) != "RELEASED" {
			t.Errorf("%s: ledger rows %d -> %d (want +1), reservation %s (want RELEASED)", label, rows, n, resState(order))
		}
	}

	// ---- manual path: no ECPay attempt at all --------------------------------------------------------------------------------
	t.Run("manual path: MERCHANT_SHIPPED with no ECPay attempt: returned then restock allowed, numeric, replay once", func(t *testing.T) {
		b := e.newBuyer()
		res, err := e.tppPlace(b, man, e.tppEntered(b, man), tppName, tppPhone)
		if err != nil {
			t.Fatalf("place: %v", err)
		}
		order := res.OrderID
		if st, _, raw := e.mcall(e.token(), "PUT", "/v1/admin/stores/"+e.store()+"/orders/"+order+"/shipment", t04Key("rg-ms"), mfxShip(0, "seven_eleven_cvs", "0012345601")); st != 200 {
			t.Fatalf("manual shipment: %d %s", st, raw)
		}
		if !parcelReturned(order) {
			t.Error("predicate: no attempt ever handed to ECPay must be true")
		}
		if st, _, raw := e.record(writer, order, t04Key("rg-ret"), "PENDING", "returned"); st != 200 {
			t.Fatalf("merchant returned (manual path): %d %s", st, raw)
		}
		restocked("manual", order)
		before, rows := bal(sku), ledgerRows(order)
		st1, out1, _ := restock(order, "r-manual") // same key as restocked() used
		if st1 != 200 || tcvStr(out1, "collection_state") != "RESTOCKED" || ledgerRows(order) != rows || len(e.deallocRows(order)) != 1 {
			t.Errorf("replay of the same restock key: %d %v rows %d", st1, out1, ledgerRows(order))
		}
		wantBal("replay", bal(sku), before)
	})

	e.connect("C2C")
	api, _, _ := e.service("cvs_711", "API", 0)
	apiFami, _, _ := e.service("cvs_familymart", "API", 0)
	endpoint := e.endpointID()
	place := func(kind, svc string) string {
		order, _ := e.cvsOrder(tcvOrderSpec{kind: kind, code: svc, paymentMode: "pay_at_pickup"})
		return order
	}
	created := func(order string) {
		t.Helper()
		if st, _, raw := e.ship(e.token(), order, 0, "", true); st != 202 {
			t.Fatalf("request shipment: %d %s", st, raw)
		}
		e.awaitShip(order, "CREATED")
	}
	type snap struct {
		events, ledger, coll, audit, reverted, conflict int
		ship, collection                                string
		version                                         int64
		bal                                             tppBalance
	}
	take := func(order string) snap {
		st, _, v := e.shipState(order)
		return snap{events: e.events(order, ""), ledger: ledgerRows(order), audit: e.audit("fulfillment.collection_reported"),
			reverted: e.events(order, "AND event_code='collection.reverted'"), conflict: e.events(order, "AND event_code='alert.collection_conflict'"),
			ship: st, collection: e.collectionState(order), version: v, bal: bal(sku)}
	}
	// post sends one signed status and then re-delivers the byte-identical body: the replay must answer 1|OK and change nothing.
	post := func(order, rtn string) {
		t.Helper()
		fields := e.statusFor(order, rtn)
		w := e.postStatus(endpoint, fields)
		if w.Code != 200 || w.Body.String() != "1|OK" {
			t.Fatalf("status %s for %s: %d %q", rtn, order, w.Code, w.Body.String())
		}
		s := take(order)
		w = e.postStatus(endpoint, fields)
		if w.Code != 200 || w.Body.String() != "1|OK" {
			t.Fatalf("replay of status %s: %d %q (always ACK)", rtn, w.Code, w.Body.String())
		}
		if again := take(order); again != s {
			t.Errorf("replay of status %s changed state: %+v -> %+v", rtn, s, again)
		}
	}
	posts := func(order string, codes ...string) {
		t.Helper()
		for _, c := range codes {
			post(order, c)
		}
	}

	// ---- 7-ELEVEN real sequence ----------------------------------------------------------------------------------------------
	t.Run("7-ELEVEN 2030,2073,2074,2098,2067 ends COLLECTED: no DEALLOCATE, restock refused, stock untouched, replays inert", func(t *testing.T) {
		order := place("cvs_711", api)
		created(order)
		base, rows0, audit0 := bal(sku), ledgerRows(order), e.audit("fulfillment.collection_reported")
		if parcelReturned(order) {
			t.Error("predicate must be false while CREATED")
		}
		steps := []struct {
			rtn, ship, coll string
			returned        bool
		}{
			{"2030", "AT_DC", "PENDING", false},
			{"2073", "AT_STORE", "PENDING", false},
			{"2074", "UNCLAIMED", "RETURNED", true},
			{"2098", "AT_STORE", "PENDING", false},
			{"2067", "PICKED_UP", "COLLECTED", false},
		}
		for _, s := range steps {
			post(order, s.rtn)
			if st, _, _ := e.shipState(order); st != s.ship || e.collectionState(order) != s.coll || parcelReturned(order) != s.returned {
				st, _, _ := e.shipState(order)
				t.Errorf("after %s: shipment %s collection %s predicate %v, want %s/%s/%v", s.rtn, st, e.collectionState(order), parcelReturned(order), s.ship, s.coll, s.returned)
			}
			if s.rtn == "2098" { // the parcel is back at the store: a restock request made on the stale RETURNED view is refused
				st, _, raw := restock(order, "r-after-2098")
				refused("restock after 2098", st, raw, "collection_state_changed", order, base, rows0, "PENDING")
			}
		}
		if n := e.events(order, "AND event_code='collection.reverted'"); n != 1 {
			t.Errorf("collection.reverted events = %d, want 1", n)
		}
		if n := e.events(order, "AND event_code='alert.collection_conflict'"); n != 0 {
			t.Errorf("collection_conflict alerts = %d, want 0 (2067 reaches COLLECTED through the normal branch)", n)
		}
		if got := e.audit("fulfillment.collection_reported") - audit0; got != 3 {
			t.Errorf("collection_reported audits +%d, want +3 (2074 RETURNED, 2098 revert, 2067 COLLECTED)", got)
		}
		wantBal("status sequence", bal(sku), base)
		if ledgerRows(order) != rows0 || len(e.deallocRows(order)) != 0 || resState(order) != "COMMITTED" {
			t.Errorf("status sequence wrote ledger rows (%d -> %d) / DEALLOCATE %d / reservation %s", rows0, ledgerRows(order), len(e.deallocRows(order)), resState(order))
		}
		st, _, raw := restock(order, "r-after-2067")
		refused("restock after the buyer collected", st, raw, "collection_state_changed", order, base, rows0, "COLLECTED")
		if st, _, raw := e.release(writer, order, t04Key("r-wrong-expected"), "restock", "COLLECTED"); st < 400 {
			t.Errorf("restock with expected COLLECTED must be refused, got %d %s", st, raw)
		}
		// Defence in depth: the order planted RETURNED while ECPay says PICKED_UP -> parcel_not_returned.
		plant(order, `collection_state='RETURNED'`)
		st, _, raw = restock(order, "r-planted-picked-up")
		refused("RETURNED but PICKED_UP", st, raw, "parcel_not_returned", order, base, rows0, "RETURNED")
		wantBal("end", bal(sku), base)
	})

	t.Run("2074 -> restock allowed; 2098 then 2067 on a RESTOCKED order stay RESTOCKED with no stock or audit change", func(t *testing.T) {
		order := place("cvs_711", api)
		created(order)
		posts(order, "2030", "2073", "2074")
		if st, _, _ := e.shipState(order); st != "UNCLAIMED" || e.collectionState(order) != "RETURNED" || !parcelReturned(order) {
			t.Fatalf("2074: shipment %s collection %s predicate %v", st, e.collectionState(order), parcelReturned(order))
		}
		restocked("2074", order)
		afterBal, afterRows, audit0 := bal(sku), ledgerRows(order), e.audit("fulfillment.collection_reported")
		post(order, "2098")
		if st, _, _ := e.shipState(order); st != "AT_STORE" || e.collectionState(order) != "RESTOCKED" {
			t.Errorf("2098 on RESTOCKED: shipment %s collection %s, want AT_STORE/RESTOCKED", st, e.collectionState(order))
		}
		if n := e.events(order, "AND event_code='collection.reverted'"); n != 0 {
			t.Errorf("RESTOCKED must never be reverted: %d collection.reverted events", n)
		}
		post(order, "2067")
		if st, _, _ := e.shipState(order); st != "PICKED_UP" || e.collectionState(order) != "RESTOCKED" {
			t.Errorf("2067 on RESTOCKED: shipment %s collection %s, want PICKED_UP/RESTOCKED", st, e.collectionState(order))
		}
		if n := e.events(order, "AND event_code='alert.collection_conflict'"); n != 1 {
			t.Errorf("2067 on a RESTOCKED order: %d collection_conflict alerts, want 1 (event + alert only)", n)
		}
		if got := e.audit("fulfillment.collection_reported") - audit0; got != 0 {
			t.Errorf("2098/2067 on RESTOCKED wrote %d collection_reported audits, want 0", got)
		}
		wantBal("2098+2067 on RESTOCKED", bal(sku), afterBal)
		if ledgerRows(order) != afterRows || len(e.deallocRows(order)) != 1 {
			t.Errorf("ledger rows %d -> %d, DEALLOCATE %d (want unchanged, 1)", afterRows, ledgerRows(order), len(e.deallocRows(order)))
		}
		st, _, raw := restock(order, "r-twice")
		refused("second restock", st, raw, "collection_state_changed", order, afterBal, afterRows, "RESTOCKED")
	})

	t.Run("2098 cycles: each 2074->2098 reverts once; a repeated 2098 does not; the final 2074 restocks", func(t *testing.T) {
		order := place("cvs_711", api)
		created(order)
		posts(order, "2030", "2073", "2074", "2098", "2098", "2074", "2098", "2074")
		if st, _, _ := e.shipState(order); st != "UNCLAIMED" || e.collectionState(order) != "RETURNED" {
			t.Errorf("after the cycles: shipment %s collection %s, want UNCLAIMED/RETURNED", st, e.collectionState(order))
		}
		if n := e.events(order, "AND event_code='collection.reverted'"); n != 2 {
			t.Errorf("collection.reverted events = %d, want 2 (one per UNCLAIMED->AT_STORE)", n)
		}
		restocked("cycles", order)
	})

	// ---- predicate x restock / returned matrix --------------------------------------------------------------------------------
	t.Run("predicate matrix: true only for UNCLAIMED, FAILED-only and ABANDONED-only; false for REQUESTED/UNKNOWN/CREATED/AT_DC/AT_STORE/PICKED_UP", func(t *testing.T) {
		mk := func(name string, drive func(order string)) string {
			order := place("cvs_711", api)
			drive(order)
			return order
		}
		failedDrive := func(order string) {
			e.fake.SetCreateMode(ecpaytest.CreateReject, "balance too low")
			defer e.fake.SetCreateMode(ecpaytest.CreateOK, "")
			if st, _, raw := e.ship(e.token(), order, 0, "", true); st != 202 {
				t.Fatalf("request: %d %s", st, raw)
			}
			e.awaitShip(order, "FAILED")
		}
		cases := []struct {
			name  string
			drive func(order string)
			want  bool
		}{
			{"REQUESTED", func(o string) {
				if st, _, raw := e.ship(e.token(), o, 0, "", false); st != 202 { // job not routed: stays REQUESTED
					t.Fatalf("request: %d %s", st, raw)
				}
			}, false},
			{"UNKNOWN", func(o string) {
				e.fake.SetCreateMode(ecpaytest.Create403, "")
				defer e.fake.SetCreateMode(ecpaytest.CreateOK, "")
				if st, _, raw := e.ship(e.token(), o, 0, "", true); st != 202 {
					t.Fatalf("request: %d %s", st, raw)
				}
				e.awaitShip(o, "UNKNOWN")
			}, false},
			{"CREATED", func(o string) { created(o) }, false},
			{"AT_DC", func(o string) { created(o); posts(o, "2030") }, false},
			{"AT_STORE", func(o string) { created(o); posts(o, "2030", "2073") }, false},
			{"PICKED_UP", func(o string) { created(o); posts(o, "2030", "2073", "2067") }, false},
			{"UNCLAIMED", func(o string) { created(o); posts(o, "2030", "2073", "2074") }, true},
			{"FAILED", failedDrive, true},
			{"ABANDONED", func(o string) {
				e.fake.SetCreateMode(ecpaytest.Create403, "")
				defer e.fake.SetCreateMode(ecpaytest.CreateOK, "")
				if st, _, raw := e.ship(e.token(), o, 0, "", true); st != 202 {
					t.Fatalf("request: %d %s", st, raw)
				}
				e.awaitShip(o, "UNKNOWN")
				if st, _, raw := e.abandon(o); st != 200 {
					t.Fatalf("abandon: %d %s", st, raw)
				}
			}, true},
		}
		for _, c := range cases {
			order := mk(c.name, c.drive)
			if got, _, _ := e.shipState(order); got != c.name {
				t.Errorf("%s: drove the shipment to %s", c.name, got)
				continue
			}
			if got := parcelReturned(order); got != c.want {
				t.Errorf("%s: cvs_parcel_returned = %v, want %v", c.name, got, c.want)
			}
			// merchant-recorded returned needs a shipped fulfillment state: the flow sets PROVIDER_LABEL_CREATED for CREATED..UNCLAIMED;
			// FAILED/ABANDONED-only history is the manual path (MERCHANT_SHIPPED); REQUESTED/UNKNOWN stay MANUAL_UNASSIGNED (422 not_shipped).
			if c.name == "FAILED" || c.name == "ABANDONED" {
				if st, _, raw := e.mcall(e.token(), "PUT", "/v1/admin/stores/"+e.store()+"/orders/"+order+"/shipment", t04Key("rg-ms-"+c.name), mfxShip(0, "seven_eleven_cvs", fmt.Sprintf("0012345%03d", len(c.name)))); st != 200 {
					t.Errorf("%s: manual shipment: %d %s", c.name, st, raw)
					continue
				}
			}
			plant(order, `collection_state='PENDING'`)
			base, rows := bal(sku), ledgerRows(order)
			st, _, raw := e.record(writer, order, t04Key("rg-m-"+c.name), "PENDING", "returned")
			switch {
			case c.name == "REQUESTED" || c.name == "UNKNOWN":
				if st != 422 || code(raw) != "not_shipped" || e.collectionState(order) != "PENDING" {
					t.Errorf("%s: merchant returned: want 422 not_shipped and PENDING, got %d %s (%s)", c.name, st, raw, e.collectionState(order))
				}
				plant(order, `collection_state='RETURNED'`)
			case c.want:
				if st != 200 || e.collectionState(order) != "RETURNED" {
					t.Errorf("%s: merchant returned must be allowed, got %d %s", c.name, st, raw)
				}
			default:
				refused(c.name+": merchant returned", st, raw, "parcel_not_returned", order, base, rows, "PENDING")
				plant(order, `collection_state='RETURNED'`)
			}
			// restock
			if c.want {
				restocked(c.name, order)
			} else {
				st, _, raw := restock(order, "r-m-"+c.name)
				refused(c.name+": restock", st, raw, "parcel_not_returned", order, base, rows, "RETURNED")
			}
		}
	})

	// ---- merchant-recorded returned: natural flow ----------------------------------------------------------------------------
	t.Run("merchant returned while CREATED / AT_STORE refused; at UNCLAIMED allowed and a later 2098 reverts it", func(t *testing.T) {
		o1 := place("cvs_711", api)
		created(o1)
		base, rows := bal(sku), ledgerRows(o1)
		rec0 := e.audit("fulfillment.collection_recorded")
		st, _, raw := e.record(writer, o1, t04Key("rg-ret-created"), "PENDING", "returned")
		refused("returned while CREATED", st, raw, "parcel_not_returned", o1, base, rows, "PENDING")
		posts(o1, "2030", "2073")
		st, _, raw = e.record(writer, o1, t04Key("rg-ret-store"), "PENDING", "returned")
		refused("returned while AT_STORE", st, raw, "parcel_not_returned", o1, base, rows, "PENDING")
		if got := e.audit("fulfillment.collection_recorded") - rec0; got != 0 {
			t.Errorf("refused returned wrote %d collection_recorded audits, want 0", got)
		}
		// the same order reaches UNCLAIMED: ECPay sets RETURNED itself; the merchant path is allowed from PENDING (planted) and 2098 reverts it
		posts(o1, "2074")
		plant(o1, `collection_state='PENDING'`)
		if st, out, raw := e.record(writer, o1, t04Key("rg-ret-unclaimed"), "PENDING", "returned"); st != 200 || tcvStr(out, "collection_state") != "RETURNED" {
			t.Fatalf("returned while UNCLAIMED: %d %s", st, raw)
		}
		post(o1, "2098")
		if got := e.collectionState(o1); got != "PENDING" {
			t.Errorf("2098 after a merchant-recorded returned: collection %s, want PENDING (revert covers any RETURNED)", got)
		}
		st, _, raw = restock(o1, "r-after-merchant-2098")
		refused("restock after 2098 of a merchant-returned order", st, raw, "collection_state_changed", o1, base, rows, "PENDING")
		post(o1, "2067")
		if e.collectionState(o1) != "COLLECTED" || e.events(o1, "AND event_code='alert.collection_conflict'") != 0 {
			t.Errorf("2067 after the revert: collection %s, conflicts %d, want COLLECTED/0", e.collectionState(o1), e.events(o1, "AND event_code='alert.collection_conflict'"))
		}
		wantBal("natural flow", bal(sku), base)
	})

	t.Run("FAILED attempt then manual shipment (MERCHANT_SHIPPED): returned then restock allowed", func(t *testing.T) {
		order := place("cvs_711", api)
		e.fake.SetCreateMode(ecpaytest.CreateReject, "balance too low")
		if st, _, raw := e.ship(e.token(), order, 0, "", true); st != 202 {
			t.Fatalf("request: %d %s", st, raw)
		}
		e.awaitShip(order, "FAILED")
		e.fake.SetCreateMode(ecpaytest.CreateOK, "")
		if st, _, raw := e.mcall(e.token(), "PUT", "/v1/admin/stores/"+e.store()+"/orders/"+order+"/shipment", t04Key("rg-ms-failed"), mfxShip(0, "seven_eleven_cvs", "0012345602")); st != 200 {
			t.Fatalf("manual shipment after FAILED: %d %s", st, raw)
		}
		if !parcelReturned(order) {
			t.Error("predicate: FAILED-only history must be true")
		}
		if st, _, raw := e.record(writer, order, t04Key("rg-ret-failed"), "PENDING", "returned"); st != 200 {
			t.Fatalf("returned: %d %s", st, raw)
		}
		restocked("failed-manual", order)
	})

	// ---- FamilyMart: the contract has no 3020-family re-delivery (2098 is 7-ELEVEN only, §6) ---------------------------------
	t.Run("FamilyMart 3020 restock allowed; 3022 -> COLLECTED refused; 2098 is not a FamilyMart code (event only, no revert)", func(t *testing.T) {
		o1 := place("cvs_familymart", apiFami)
		created(o1)
		posts(o1, "3024", "3018", "3020")
		if st, _, _ := e.shipState(o1); st != "UNCLAIMED" || e.collectionState(o1) != "RETURNED" || !parcelReturned(o1) {
			t.Fatalf("3020: shipment %s collection %s predicate %v", st, e.collectionState(o1), parcelReturned(o1))
		}
		// a 7-ELEVEN re-delivery code on a FamilyMart trade is unknown for the subtype: stored, no transition, no revert
		a0 := e.audit("fulfillment.collection_reported")
		post(o1, "2098")
		if st, _, _ := e.shipState(o1); st != "UNCLAIMED" || e.collectionState(o1) != "RETURNED" || e.events(o1, "AND event_code='collection.reverted'") != 0 || e.audit("fulfillment.collection_reported") != a0 {
			st, _, _ := e.shipState(o1)
			t.Errorf("2098 on FamilyMart: shipment %s collection %s reverted %d (want UNCLAIMED/RETURNED/0)", st, e.collectionState(o1), e.events(o1, "AND event_code='collection.reverted'"))
		}
		restocked("familymart-3020", o1)

		o2 := place("cvs_familymart", apiFami)
		created(o2)
		posts(o2, "3024", "3018")
		base, rows := bal(sku), ledgerRows(o2)
		post(o2, "3022")
		if st, _, _ := e.shipState(o2); st != "PICKED_UP" || e.collectionState(o2) != "COLLECTED" || parcelReturned(o2) {
			t.Errorf("3022: shipment %s collection %s predicate %v, want PICKED_UP/COLLECTED/false", st, e.collectionState(o2), parcelReturned(o2))
		}
		st, _, raw := restock(o2, "r-fm-collected")
		refused("FamilyMart restock after 3022", st, raw, "collection_state_changed", o2, base, rows, "COLLECTED")
		plant(o2, `collection_state='RETURNED'`)
		st, _, raw = restock(o2, "r-fm-planted")
		refused("FamilyMart RETURNED but PICKED_UP", st, raw, "parcel_not_returned", o2, base, rows, "RETURNED")
		post(o2, "3020") // a late return code after pickup: backwards jump, event only
		if st, _, _ := e.shipState(o2); st != "PICKED_UP" {
			t.Errorf("3020 after 3022 moved the shipment to %s", st)
		}
	})
}
