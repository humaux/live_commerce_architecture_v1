package foundation_test

// TestCvsCollectedGuardT2102: independent gate for the collected guard (contracts/taiwan-cvs-logistics-v1.md §16.4;
// migration 0102). Definer under test: fulfillment.cvs_parcel_picked_up (the single predicate, mirroring
// fulfillment.cvs_parcel_returned of 0083) and the 'collected' branch of fulfillment.record_collection. Written from the
// contract, not from the implementation. Tier REAL_PG + HTTP_PG + MOCK ECPay (ecpaytest fake, signed status posts).
// Owner-pool writes (disclosed fixtures): identity grants, and planting checkout.orders.collection_state back to PENDING to
// reach the PICKED_UP-but-PENDING state no event sequence produces (the same disclosure as TestCvsRestockGuardT2101).
// Nothing else is fabricated.

import (
	"context"
	"fmt"
	"testing"
)

func TestCvsCollectedGuardT2102(t *testing.T) {
	e := tcvNew(t, tcvOpts{stripe: true})
	f := e.p.f
	e.r.startWorker(t)
	e.startDispatcher()
	e.grantCreator("orders:read", "fulfillment:write", "integration:manage", "integration:read")
	man, _, _ := e.service("cvs_711", "MANUAL", 0)
	e.cvsSettings(tcvAllChains, true, "20000", 500)
	writer, _ := e.member("fulfillment:write", "orders:read")

	code := func(raw []byte) string { return tcvStr(tcvJSON(t, raw), "code") }
	picked := func(order string) (b bool) {
		t.Helper()
		if err := f.owner.QueryRow(context.Background(), `SELECT fulfillment.cvs_parcel_picked_up($1,$2,$3)`, e.tenant(), e.store(), order).Scan(&b); err != nil {
			t.Fatalf("cvs_parcel_picked_up: %v", err)
		}
		return
	}
	plantPending := func(order string) {
		mustExec(t, f.owner, `UPDATE checkout.orders SET collection_state='PENDING' WHERE id=$1`, order)
	}
	// refused: the call must be 409 parcel_not_picked_up, change nothing and write no audit row.
	refused := func(label, order string) {
		t.Helper()
		rec0 := e.audit("fulfillment.collection_recorded")
		st, _, raw := e.record(writer, order, t04Key("cg-"+label), "PENDING", "collected")
		if st != 409 || code(raw) != "parcel_not_picked_up" || e.collectionState(order) != "PENDING" {
			t.Errorf("%s: want 409 parcel_not_picked_up and PENDING, got %d %s (%s)", label, st, raw, e.collectionState(order))
		}
		if got := e.audit("fulfillment.collection_recorded") - rec0; got != 0 {
			t.Errorf("%s: a refused collected wrote %d collection_recorded audits, want 0", label, got)
		}
	}
	manualOrder := func() string {
		t.Helper()
		b := e.newBuyer()
		res, err := e.tppPlace(b, man, e.tppEntered(b, man), tppName, tppPhone)
		if err != nil {
			t.Fatalf("place: %v", err)
		}
		if st, _, raw := e.mcall(e.token(), "PUT", "/v1/admin/stores/"+e.store()+"/orders/"+res.OrderID+"/shipment", t04Key("cg-ms"), mfxShip(0, "seven_eleven_cvs", "0012345678")); st != 200 {
			t.Fatalf("manual shipment: %d %s", st, raw)
		}
		return res.OrderID
	}

	t.Run("manual path: no ECPay attempt -> collected allowed, predicate true, replay idempotent", func(t *testing.T) {
		o := manualOrder()
		if !picked(o) {
			t.Error("predicate: no attempt ever handed to ECPay must be true")
		}
		key := t04Key("cg-manual")
		st, out, raw := e.record(writer, o, key, "PENDING", "collected")
		if st != 200 || tcvStr(out, "collection_state") != "COLLECTED" || e.collectionState(o) != "COLLECTED" {
			t.Fatalf("manual collected: %d %s", st, raw)
		}
		if st2, again, _ := e.record(writer, o, key, "PENDING", "collected"); st2 != 200 || fmt.Sprint(again) != fmt.Sprint(out) {
			t.Errorf("replay of the same collected key: %d %v (want the saved answer)", st2, again)
		}
	})

	e.connect("C2C")
	api, _, _ := e.service("cvs_711", "API", 0)
	endpoint := e.endpointID()
	pap := func() string {
		order, _ := e.cvsOrder(tcvOrderSpec{kind: "cvs_711", code: api, paymentMode: "pay_at_pickup"})
		return order
	}
	shipCreated := func(order string) {
		t.Helper()
		if st, _, raw := e.ship(e.token(), order, 0, "", true); st != 202 {
			t.Fatalf("request shipment: %d %s", st, raw)
		}
		e.awaitShip(order, "CREATED")
	}

	t.Run("ECPay path: collected refused at AT_DC / AT_STORE / UNCLAIMED, allowed at PICKED_UP, replay idempotent", func(t *testing.T) {
		o := pap()
		shipCreated(o)
		if picked(o) {
			t.Error("predicate must be false while CREATED")
		}
		e.tppStatuses(endpoint, o, "2030") // AT_DC (collection stays PENDING)
		if st, _, _ := e.shipState(o); st != "AT_DC" || e.collectionState(o) != "PENDING" {
			t.Fatalf("2030: shipment %s collection %s, want AT_DC/PENDING", st, e.collectionState(o))
		}
		refused("at-dc", o)
		e.tppStatuses(endpoint, o, "2073") // AT_STORE
		refused("at-store", o)
		e.tppStatuses(endpoint, o, "2074") // UNCLAIMED + RETURNED by ECPay
		plantPending(o)                    // the merchant's record is attempted from the stale PENDING view
		refused("unclaimed", o)
		if picked(o) {
			t.Error("predicate must be false while UNCLAIMED")
		}
		// 7-ELEVEN re-delivery then pickup: 2098 -> AT_STORE (revert), 2067 -> PICKED_UP + COLLECTED by ECPay
		e.tppStatuses(endpoint, o, "2098", "2067")
		if st, _, _ := e.shipState(o); st != "PICKED_UP" || e.collectionState(o) != "COLLECTED" {
			t.Fatalf("2098/2067: shipment %s collection %s, want PICKED_UP/COLLECTED", st, e.collectionState(o))
		}
		if !picked(o) {
			t.Error("predicate must be true while PICKED_UP")
		}
		plantPending(o) // ECPay already recorded COLLECTED; plant PENDING to exercise the merchant's own collected record
		key := t04Key("cg-collected")
		st, out, raw := e.record(writer, o, key, "PENDING", "collected")
		if st != 200 || tcvStr(out, "collection_state") != "COLLECTED" || e.collectionState(o) != "COLLECTED" {
			t.Fatalf("collected at PICKED_UP: %d %s", st, raw)
		}
		if st2, again, _ := e.record(writer, o, key, "PENDING", "collected"); st2 != 200 || fmt.Sprint(again) != fmt.Sprint(out) {
			t.Errorf("replay of the same collected key: %d %v (want the saved answer)", st2, again)
		}
		if st, _, raw := e.record(writer, o, t04Key("cg-stale"), "PENDING", "collected"); st != 409 || code(raw) != "collection_state_changed" {
			t.Errorf("a new key on a COLLECTED order: want 409 collection_state_changed, got %d %s", st, raw)
		}
	})
}
