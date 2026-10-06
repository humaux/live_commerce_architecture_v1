package foundation_test

// LCN12 (DM half): the REAL inbox.Service.PlanOrderPayLink against the real planner inbox.plan_dm (migrations 0128 + 0129), the real River dispatch and
// a loopback Graph (lbSetup). Evidence label of every passing line: MOCK (loopback Graph, no real Meta call).
//   TestLiveConsoleOrderPayLinkPlanner  semantic key mdm:+sha256(conversation|order-pay-link|order), the link only in the sealed dispatch copy (the
//                                       Graph body carries it, the stored display copy and the frozen request do not), one operation per (conversation,
//                                       order) however often it is planned, two different orders to one buyer are not duplicate_recent, the replay
//                                       probe, a closed window and a missing inbox:reply are refused with the planner's codes.
// Owner-pool writes (disclosed fixtures): moving the inbound time to close the window, reading result tables.

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/inbox"
	"livecommerce/internal/platform"
)

func TestLiveConsoleOrderPayLinkPlanner(t *testing.T) {
	e := lbSetup(t)
	f := e.h.f
	ctx := context.Background()
	psid := mciDigits(15)
	conv := e.postDM(t, psid, "buyer asks to be invoiced", time.Now())
	order := randomUUID()
	linkFor := func(order string) string {
		return "https://shop.example.test/zh-TW/order-link#o=" + order + "&t=" + strings.Repeat("Tk", 21) + "x"
	}
	planAs := func(token, permission, key, conversation, orderID, link string) (inbox.SendOutput, error) {
		var out inbox.SendOutput
		err := platform.WithScope(ctx, f.runtime, token, f.storeA1, permission, func(tx pgx.Tx, s platform.Scope) error {
			var planErr error
			out, planErr = e.svc.PlanOrderPayLink(ctx, tx, s, key, conversation, orderID, link)
			return planErr
		})
		return out, err
	}
	plan := func(token, key, conversation, orderID, link string) (inbox.SendOutput, error) {
		return planAs(token, "inbox:reply", key, conversation, orderID, link)
	}
	key := lbKey()

	if _, err := plan(e.h.token, key, conv, order, ""); err != inbox.ErrPayLinkNotPlanned {
		t.Fatalf("the probe before planning: %v", err)
	}
	out, err := plan(e.h.token, key, conv, order, linkFor(order))
	if err != nil || out.SendState != "queued" || out.OperationID == "" {
		t.Fatalf("plan: %+v %v", out, err)
	}
	t.Run("one operation, the order key, the link only in the sealed copy", func(t *testing.T) {
		var keyOK bool
		var request string
		if err := f.owner.QueryRow(ctx, `SELECT semantic_key='mdm:'||substr(encode(sha256(convert_to($2||'|order-pay-link|'||$3,'UTF8')),'hex'),1,48), request::text
			FROM integration.operations WHERE id=$1`, out.OperationID, conv, order).Scan(&keyOK, &request); err != nil || !keyOK {
			t.Fatalf("semantic key must be mdm:+hex(sha256(conversation|order-pay-link|order))[:48]: %v", err)
		}
		if !strings.Contains(request, order) || strings.Contains(request, "shop.example.test") || strings.Contains(request, psid) {
			t.Fatalf("frozen request: %s", request)
		}
		if n := miCount(t, f.owner, `SELECT count(*) FROM integration.operations WHERE action='meta.dm_send' AND request->>'order_id'=$1`, order); n != 1 {
			t.Fatalf("operations for the order: %d", n)
		}
		if e.secretCount(t, out.OperationID) != 1 {
			t.Fatal("no sealed dispatch copy")
		}
	})
	t.Run("the replay of the same key is the saved result; the probe now finds it", func(t *testing.T) {
		again, err := plan(e.h.token, key, conv, order, linkFor(order))
		if err != nil || again.OperationID != out.OperationID {
			t.Fatalf("replay: %+v %v", again, err)
		}
		probed, err := plan(e.h.token, key, conv, order, "")
		if err != nil || probed.OperationID != out.OperationID {
			t.Fatalf("probe after planning: %+v %v", probed, err)
		}
	})
	t.Run("another key for the same (conversation, order) can never plan a second DM", func(t *testing.T) {
		if _, err := plan(e.h.token, lbKey(), conv, order, linkFor(order)); err == nil {
			t.Fatal("a second pay-link DM was planned for one order")
		}
		if n := miCount(t, f.owner, `SELECT count(*) FROM integration.operations WHERE action='meta.dm_send' AND request->>'order_id'=$1`, order); n != 1 {
			t.Fatalf("operations for the order: %d", n)
		}
	})
	t.Run("dispatch: Graph gets the link, the display copy keeps the placeholder", func(t *testing.T) {
		e.run(t, out.OperationID)
		e.awaitOp(t, out.OperationID, "SUCCEEDED", 10*time.Second, "completed")
		reqs := e.g.all()
		msg, _ := reqs[len(reqs)-1].body["message"].(map[string]any)
		text, _ := msg["text"].(string)
		if !strings.Contains(text, linkFor(order)) || !strings.Contains(text, "您的訂單已建立") {
			t.Fatalf("Graph text: %q", text)
		}
		var view inbox.ThreadView
		if err := e.scoped(t, "inbox:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
			var readErr error
			view, readErr = e.svc.ReadThread(ctx, tx, conv, nil, 50)
			return readErr
		}); err != nil {
			t.Fatal(err)
		}
		seen := 0
		for _, it := range view.Items {
			if it.Direction == "out" {
				seen++
				if strings.Contains(it.Text, "https") || strings.Contains(it.Text, order) || !strings.Contains(it.Text, "{{連結}}") {
					t.Fatalf("display copy keeps the link: %q", it.Text)
				}
			}
		}
		if seen != 1 {
			t.Fatalf("outbound display copies: %d", seen)
		}
		if e.secretCount(t, out.OperationID) != 0 {
			t.Fatal("send secret survived SUCCEEDED")
		}
	})
	t.Run("a different order to the same buyer is not duplicate_recent (the order id is in the body digest)", func(t *testing.T) {
		second := randomUUID()
		got, err := plan(e.h.token, lbKey(), conv, second, linkFor(second))
		if err != nil || got.OperationID == out.OperationID {
			t.Fatalf("second order DM: %+v %v", got, err)
		}
	})
	t.Run("closed window: refused with window_closed, probe says so before any link is issued", func(t *testing.T) {
		old := e.postDM(t, mciDigits(15), "long ago", time.Now())
		e.setInbound(t, old, time.Now().Add(-25*time.Hour))
		o := randomUUID()
		if _, err := plan(e.h.token, lbKey(), old, o, ""); planCode(err) != "window_closed" {
			t.Fatalf("probe in a closed window: %v", err)
		}
		if _, err := plan(e.h.token, lbKey(), old, o, linkFor(o)); planCode(err) != "window_closed" {
			t.Fatalf("plan in a closed window: %v", err)
		}
	})
	t.Run("without inbox:reply the planner refuses (forbidden)", func(t *testing.T) {
		_, token := lcPerson(t, f, f.tenantA, f.storeA1, "inventory:reserve", "inbox:read")
		o := randomUUID()
		// The route's own permission is inventory:reserve (the for-buyer pipeline); the planner must still refuse a principal without inbox:reply.
		if _, err := planAs(token, "inventory:reserve", lbKey(), conv, o, linkFor(o)); planCode(err) != "forbidden" {
			t.Fatalf("without inbox:reply: %v", err)
		}
	})
}
