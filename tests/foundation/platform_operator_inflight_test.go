package foundation_test

// OPS-01B follow-up gate (REAL_PG, MOCK Graph/Stripe): the "stop new outbound, keep money in flight" boundary of migration 0143.
//
//	OP10 an origin='auto' console send queued BEFORE the suspension ends BLOCKED_POLICY store_suspended with zero Graph HTTP
//	OP04b Stripe webhook capture and refund settlement still record for a suspended store (pinned next to the PAYUNi pin of
//	      TestK3W401BDisabledStoreStillRecordsAndWakes), plus a definer-source pin: no money-recording Stripe definer reads
//	      control.stores / control.tenants, so no future flag read can slip in unnoticed.

import (
	"context"
	"regexp"
	"testing"
	"time"
)

func TestPlatformOperatorOP10AutoSendQueuedBeforeSuspensionIsBlocked(t *testing.T) {
	e := lbSetup(t)
	f := e.h.f
	p := poSetup(t, f)
	t.Cleanup(func() { // shared claims database: never leave the store suspended for another test
		_, _ = f.owner.Exec(context.Background(), `UPDATE control.stores SET active=true WHERE id=$1`, f.storeA1)
	})
	conv := e.postDM(t, mciDigits(15), "hello", time.Now())
	out, err := e.sendDM(conv, "queued before the suspension", 0)
	if err != nil {
		t.Fatal(err)
	}
	// Re-label the frozen operation as an automatic-origin send (the human-origin principal_holds check would stop a
	// human send by itself; the automatic one is the case this guard exists for).
	mustExec(t, f.owner, `UPDATE integration.operations SET request=jsonb_set(request,'{origin}','"auto"') WHERE id=$1`, out.OperationID)
	check := func() string {
		var code string
		if err := f.owner.QueryRow(context.Background(), `SELECT inbox.check_send($1::uuid)`, out.OperationID).Scan(&code); err != nil {
			t.Fatal(err)
		}
		return code
	}
	if c := check(); c == "store_suspended" {
		t.Fatal("a serving store was refused as suspended")
	}
	p.must(p.setStore(f.storeA1, false, "fraud"))
	if c := check(); c != "store_suspended" {
		t.Fatalf("check_send for a suspended store = %q, want store_suspended", c)
	}
	before := e.g.count()
	e.run(t, out.OperationID)
	code, _ := e.awaitOp(t, out.OperationID, "BLOCKED_POLICY", 10*time.Second, "completed")
	if code != "store_suspended" || e.g.count() != before {
		t.Fatalf("queued auto send: code=%s graph calls %d->%d, want store_suspended with zero HTTP", code, before, e.g.count())
	}
	if e.secretCount(t, out.OperationID) != 0 {
		t.Fatal("send secret survived BLOCKED_POLICY")
	}
}

// OP04b (behaviour): a Stripe payment completes and its refund settles although the store is suspended in between.
func TestPlatformOperatorOP04bStripeRecordsForSuspendedStore(t *testing.T) {
	e := srfNew(t)
	f := e.f
	store := e.base.store()
	suspend := func(active bool) {
		reason := any("non_payment")
		if active {
			reason = nil
		}
		var raw string
		if err := f.owner.QueryRow(context.Background(), `SELECT control.set_store_active($1::uuid,$2,'op.test','TICKET-4',$3)::text`, store, active, reason).Scan(&raw); err != nil {
			t.Fatal(err)
		}
	}
	// (a) refund requested while serving, store suspended right after: settlement is worker-side and still records.
	refundOrder := e.base
	id := e.mustRefund(t, refundOrder, refundOrder.captured, "requested_by_customer")
	suspend(false)
	e.awaitRefundFact(t, id, refundOrder.attempt, "SUCCEEDED")
	suspend(true) // resume: the merchant read model (wantMoney) is itself refused while suspended
	e.wantMoney(t, refundOrder, "REFUNDED", refundOrder.captured, 0)
	// (b) a checkout session pinned while serving is paid and its webhook delivered while the store is suspended.
	s := e.base.s
	s.p = sstMoreHold(t, e.base.s.p)
	e.ensureStock(t, e.base)
	res, session := e.pinned(t, s)
	suspend(false)
	if !e.fake.SetState(session, "complete", "paid") {
		t.Fatal("fake pay")
	}
	if status := e.deliver(t, e.base.endpoint, e.base.secret, sflEvent(res.AttemptID, session)); status != 200 {
		t.Fatalf("webhook for a suspended store answered %d", status)
	}
	e.awaitFact(t, res.AttemptID, "CAPTURED")
}

// OP04b (structure): the Stripe/PAYUNi money-recording definers never read the tenant/store flags.
func TestPlatformOperatorOP04bMoneyDefinersDoNotReadSuspensionFlags(t *testing.T) {
	b := fixture(t)
	reads := regexp.MustCompile(`(?i)control\.(stores|tenants)`)
	checked := 0
	for _, fn := range []string{
		"payments.stripe_webhook_material(uuid)", "payments.apply_stripe_observation(uuid,bytea)",
		"payments.apply_stripe_refund(uuid,bytea)", "payments.apply_stripe_charge(uuid,bytea)",
	} {
		var src string
		if err := b.owner.QueryRow(context.Background(), `SELECT prosrc FROM pg_proc WHERE oid=$1::regprocedure`, fn).Scan(&src); err != nil {
			t.Fatalf("%s: %v", fn, err)
		}
		if reads.MatchString(src) {
			t.Errorf("%s reads control.stores/tenants: a suspension would stop money from being recorded", fn)
		}
		checked++
	}
	var webhook string
	if err := b.owner.QueryRow(context.Background(), `SELECT string_agg(prosrc,' ') FROM pg_proc WHERE oid IN (SELECT p.oid FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname='payments' AND p.proname='stripe_webhook_prepare')`).Scan(&webhook); err != nil {
		t.Fatal(err)
	}
	if reads.MatchString(webhook) {
		t.Error("payments.stripe_webhook_prepare reads control.stores/tenants")
	}
	if checked != 4 {
		t.Fatal("not every definer was checked")
	}
}
