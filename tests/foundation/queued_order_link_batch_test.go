// Purpose: reproduce PR32 reminder cross-order blocking and pin legacy guards to immutable bundle/order origins.
// Depends on: real PG/merchant HTTP/inbox planner, existing crOrder synthetic commercial-state fixtures and loopback Graph.
// Used by: focused QueuedOrderLink regression acceptance; dispatcher remains paused until explicit run.
// Invariants: each unpaid buyer gets one READY reminder, sealed links survive refused rotation, no unrelated-order refusal.
package foundation_test

import (
	"context"
	"fmt"
	"testing"
)

func TestQueuedOrderLinkTwoBuyerReminderBatch(t *testing.T) {
	e, m, first, firstOrder, _ := qlSetup(t)
	second := e.crBuyer(t)
	secondOrder := e.crOrder(t, second.bundle, "AWAITING_PAYMENT", "merchant_manual", false)
	// crOrder is the existing disclosed synthetic order fixture. The planner/link issuer/HTTP/PG path is real.
	mustExec(t, e.h.f.owner, `INSERT INTO buyer.owners(tenant_id,store_id,id) SELECT tenant_id,store_id,owner_id FROM checkout.orders WHERE id=$1`, secondOrder)
	code, out := qlHTTP(e, m, "/live-sessions/"+e.session+"/reminders", "")
	e.crPark(t)
	if code != 200 || out["queued"] != float64(2) || out["refused"] != float64(0) {
		t.Fatalf("two-buyer batch must queue both unpaid orders: status=%d queued=%v refused=%v", code, out["queued"], out["refused"])
	}
	ops := e.crOps(t)
	if len(ops) != 2 || len(e.g.all()) != 0 {
		t.Fatalf("paused dispatch graph: operations=%d graph_sends=%d", len(ops), len(e.g.all()))
	}
	for _, pair := range [][2]string{{first.bundle, firstOrder}, {second.bundle, secondOrder}} {
		var operation string
		var jobs int
		if err := e.h.f.owner.QueryRow(context.Background(), `SELECT o.id::text,count(j.id) FROM integration.operations o
		 JOIN river.river_job j ON j.id=o.job_id WHERE o.tenant_id=$1 AND o.store_id=$2 AND o.state='READY'
		 AND o.request->>'bundle_id'=$3 AND o.request->>'order_id'=$4 GROUP BY o.id`, e.h.f.tenantA, e.h.f.storeA1, pair[0], pair[1]).Scan(&operation, &jobs); err != nil || jobs != 1 || e.secretCount(t, operation) != 1 {
			t.Fatalf("one order-bound READY operation/job/secret per buyer: jobs=%d err=%v", jobs, err)
		}
		before := qlLinkCount(t, e, pair[1])
		code, result := qlHTTP(e, m, "/orders/manual/regenerate-link", fmt.Sprintf(`{"order_id":%q,"locale":"zh-TW"}`, pair[1]))
		if code != 409 || result["code"] != "conflict" || qlLinkCount(t, e, pair[1]) != before {
			t.Fatalf("queued order rotation: status=%d code=%v", code, result["code"])
		}
		var valid int
		if err := e.h.f.owner.QueryRow(context.Background(), `SELECT count(*) FROM checkout.order_links WHERE order_id=$1 AND redeemed_at IS NULL AND expires_at>clock_timestamp()`, pair[1]).Scan(&valid); err != nil || valid != 1 {
			t.Fatalf("queued order retains exactly one valid token: n=%d err=%v", valid, err)
		}
	}
	// A third unpaid order is outside both READY operations; rotating it must remain possible.
	third := e.crBuyer(t)
	thirdOrder := e.crOrder(t, third.bundle, "AWAITING_PAYMENT", "merchant_manual", false)
	mustExec(t, e.h.f.owner, `INSERT INTO buyer.owners(tenant_id,store_id,id) SELECT tenant_id,store_id,owner_id FROM checkout.orders WHERE id=$1`, thirdOrder)
	code, result := qlHTTP(e, m, "/orders/manual/regenerate-link", fmt.Sprintf(`{"order_id":%q,"locale":"zh-TW"}`, thirdOrder))
	if code != 201 || len(e.crOps(t)) != 2 {
		t.Fatalf("unrelated unpaid order must remain usable: status=%d", code)
	}
	qlAssertLink(t, e, thirdOrder, result["buyer_link"].(string))
}

func TestQueuedOrderLinkLegacyReminderScope(t *testing.T) {
	e, m, b, order, _ := qlSetup(t)
	code, out := qlHTTP(e, m, "/live-sessions/"+e.session+"/reminders/"+b.bundle, "")
	e.crPark(t)
	if code != 200 || out["queued"] != float64(1) {
		t.Fatalf("real legacy reminder setup: status=%d queued=%v", code, out["queued"])
	}
	ops := e.crOps(t)
	if len(ops) != 1 {
		t.Fatalf("legacy reminder count=%d", len(ops))
	}
	// Owner-only upgrade fixture: remove precisely the field absent in 0144, leaving the actual sealed operation/journal intact.
	mustExec(t, e.h.f.owner, `UPDATE integration.operations SET request=request-'order_id' WHERE id=$1`, ops[0])
	other := e.crBuyer(t)
	otherOrder := e.crOrder(t, other.bundle, "AWAITING_PAYMENT", "merchant_manual", false)
	mustExec(t, e.h.f.owner, `INSERT INTO buyer.owners(tenant_id,store_id,id) SELECT tenant_id,store_id,owner_id FROM checkout.orders WHERE id=$1`, otherOrder)
	for _, fixture := range []string{"request bundle", "journal fallback"} {
		if fixture == "journal fallback" {
			// Missing legacy request metadata is still correlated by the persisted reminder journal, never by every store order.
			mustExec(t, e.h.f.owner, `UPDATE integration.operations SET request=request-'bundle_id' WHERE id=$1`, ops[0])
		}
		before := qlLinkCount(t, e, order)
		code, result := qlHTTP(e, m, "/orders/manual/regenerate-link", fmt.Sprintf(`{"order_id":%q,"locale":"zh-TW"}`, order))
		if code != 409 || result["code"] != "conflict" || qlLinkCount(t, e, order) != before {
			t.Fatalf("%s legacy sealed order rotation: status=%d code=%v", fixture, code, result["code"])
		}
		code, result = qlHTTP(e, m, "/orders/manual/regenerate-link", fmt.Sprintf(`{"order_id":%q,"locale":"zh-TW"}`, otherOrder))
		if code != 201 {
			t.Errorf("%s legacy reminder blocked a different buyer's order: status=%d code=%v", fixture, code, result["code"])
		} else {
			qlAssertLink(t, e, otherOrder, result["buyer_link"].(string))
		}
	}
	if e.secretCount(t, ops[0]) != 1 || len(e.g.all()) != 0 {
		t.Fatal("legacy guard changed sealed secret or dispatched an UNKNOWN send")
	}
}
