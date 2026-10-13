// Purpose: REAL_PG regression for PR53 4264535427, cancelled claim orders admitting a fresh intent.
// Depends on: actual C1/Executor, native 0193 cancellation/0190 expiry, accepted claim events and merchant authentication.
// Used by: parent OA1 CI; tests-first, no injected terminal state or reservation cleanup.
// Invariants: I01/I02/I03/I04/I08/I14; frozen §2 one live order per bundle, §3 immutable terminal replay,
// exact new-generation CAS, retained old order/origin/grant audit, and no cancellation-side claim locks.
package foundation_test

import (
	"bytes"
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/checkout"
	"livecommerce/internal/orderattempt"
	"livecommerce/internal/platform"
)

type oaCancelReadmitFixture struct {
	e            *lbuEnv
	bundle       string
	offer        string
	conversation string
	selection    map[string]any
	request      orderattempt.AdmissionRequest
	attempt      orderattempt.AdmissionReceipt
	plan         orderattempt.FrozenPlan
	receipt      orderattempt.ExecutionReceipt
	svc          *orderattempt.Service
	executor     *orderattempt.Executor
	terminal     []byte
	audit        string
	priced       bool
}

func oaCancelReadmitAudit(t *testing.T, f oaCancelReadmitFixture) string {
	t.Helper()
	var raw string
	err := f.e.p.f.owner.QueryRow(context.Background(), `SELECT jsonb_build_object(
	 'reservation',(SELECT to_jsonb(x)-'state'-'updated_at' FROM inbox.order_for_buyer x WHERE create_attempt_id=$1 AND bundle_id=$2),
	 'origins',(SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY to_jsonb(x)::text),'[]'::jsonb) FROM claims.order_origins x WHERE order_id=$1),
	 'uses',(SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY to_jsonb(x)::text),'[]'::jsonb) FROM claims.live_price_uses x WHERE order_id=$1),
	 'grants',(SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY to_jsonb(x)::text),'[]'::jsonb) FROM claims.merchant_origin_grants x WHERE create_attempt_id=$1),
	 'audit',(SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY to_jsonb(x)::text),'[]'::jsonb) FROM ops.audit_events x
	  WHERE tenant_id=$3 AND store_id=$4 AND details->>'attempt_id'=$1::uuid::text))::text`,
		f.attempt.AttemptID, f.bundle, f.e.tenant(), f.e.store()).Scan(&raw)
	if err != nil {
		t.Fatal("retained original claim/order audit", err)
	}
	return raw
}

func oaCancelReadmitSetup(t *testing.T, mode string, priced bool) oaCancelReadmitFixture {
	t.Helper()
	e := lbuNew(t)
	e.grantCreator("fulfillment:write")
	_, offer, bundle, conversation := e.scenario()
	if !priced {
		// Disclosed catalog-only claim fixture; accepted quantity and source provenance remain native.
		mustExec(t, e.p.f.owner, `UPDATE live.offers SET live_price_minor=NULL WHERE id=$1`, offer.ID)
	}
	selection := oaClaimSelection(e, []string{bundle}, []string{offer.ID}, conversation)
	payment := mode
	if mode == "expiry" {
		payment = "bank_transfer"
	}
	if payment != "bank_transfer" {
		base := oaBeginSelection(t, e, payment, "duplicate-origins")
		selection["delivery"] = base["delivery"]
		selection["customer"] = base["customer"]
		selection["payment_mode"] = payment
	}
	keys := oaServiceKeys(t)
	svc, err := orderattempt.NewService(e.p.f.runtime, keys)
	if err != nil {
		t.Fatal(err)
	}
	req := oaMaterialRequest(t, e, selection)
	a, err := svc.Admit(context.Background(), e.token(), e.store(), req)
	if err != nil || a.State != "PENDING" || a.Replay || a.Generation != req.ExpectedGeneration+1 {
		t.Fatalf("positive native claim C1 before cancel: %+v err=%v", a, err)
	}
	plan, _ := oaMaterialPlan(t, e, keys, a.AttemptID)
	executor := oaExecutorNew(t, e.p.f.runtime, keys)
	first, err := executor.Execute(context.Background(), e.token(), e.store(), a.AttemptID, a.Generation)
	if err != nil {
		t.Fatal("positive full native claim C2 before cancel", err)
	}
	oaExecutorReceipt(t, first, a, "COMMITTED", "")
	f := oaCancelReadmitFixture{e: e, bundle: bundle, offer: offer.ID, conversation: conversation, selection: selection,
		request: req, attempt: a, plan: plan, receipt: first, svc: svc, executor: executor, priced: priced}
	var exact bool
	err = e.p.f.owner.QueryRow(context.Background(), `SELECT a.state='COMMITTED' AND a.order_id=o.id AND a.receipt=$2::jsonb
	 AND o.create_attempt_id=a.attempt_id AND o.owner_id=a.buyer_owner_id AND o.payment_mode=$3
	 AND o.commercial_state=CASE WHEN $3='bank_transfer' THEN 'AWAITING_TRANSFER'
	  WHEN $3='cash_on_delivery' THEN 'AWAITING_COLLECTION' ELSE 'CONFIRMED' END
	 AND o.total_minor=$4 AND r.state=CASE WHEN $3='bank_transfer' THEN 'HELD' ELSE 'COMMITTED' END
	 AND x.state='placed' AND x.request_id=a.attempt_id AND x.order_id=o.id AND x.buyer_id=o.owner_id
	 AND (SELECT count(*) FROM claims.order_origins WHERE order_id=o.id AND bundle_id=$5 AND offer_id=$6)=1
	 AND (SELECT count(*) FROM claims.live_price_uses WHERE order_id=o.id AND bundle_id=$5 AND offer_id=$6 AND quantity=2)=$7
	 AND (SELECT count(*) FROM claims.merchant_origin_grants WHERE create_attempt_id=a.attempt_id AND consumed_at IS NOT NULL)=$7
	 FROM ops.order_attempts a JOIN checkout.orders o ON o.id=a.order_id JOIN inventory.reservations r ON r.id=o.id
	 JOIN inbox.order_for_buyer x ON x.create_attempt_id=a.attempt_id AND x.bundle_id=$5 WHERE a.attempt_id=$1`,
		a.AttemptID, string(first.Receipt), payment, plan.Calculation.TotalMinor, bundle, offer.ID, map[bool]int{true: 1, false: 0}[priced]).Scan(&exact)
	if err != nil || !exact {
		t.Fatalf("positive exact placed order, claim origin/use/grant prerequisite: exact=%v err=%v", exact, err)
	}
	f.terminal, f.audit = oaCEAttemptBytes(t, e, a), oaCancelReadmitAudit(t, f)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if mode == "expiry" {
		// Existing clock fixture ages only the native hold/order after real C2; no attempt metadata is changed.
		bcDue(t, e.p.bcHarness, checkout.Result{OrderID: a.AttemptID})
	}
	holder, err := e.p.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	oaAbortCleanupTx(t, holder)
	// Cancellation/expiry is an order-only command (§3). Holding the entire claim prefix
	// proves it cannot acquire claim locks to free the bundle from its side.
	oaStateExec(t, holder, `SELECT id FROM claims.bundles WHERE id=$1 FOR UPDATE`, bundle)
	oaStateExec(t, holder, `SELECT offer_id FROM claims.lines WHERE bundle_id=$1 ORDER BY offer_id FOR UPDATE`, bundle)
	oaStateExec(t, holder, `SELECT request_id FROM inbox.order_for_buyer WHERE bundle_id=$1 ORDER BY request_id FOR UPDATE`, bundle)
	oaStateExec(t, holder, `SELECT request_id FROM claims.merchant_origin_grants WHERE bundle_id=$1 ORDER BY request_id FOR UPDATE`, bundle)
	if mode == "expiry" {
		var disposition string
		var retry *time.Time
		err = e.p.expiry.QueryRow(ctx, `SELECT disposition,retry_at FROM checkout.expire_held($1,1)`, a.AttemptID).Scan(&disposition, &retry)
		if err != nil || disposition != "EXPIRED" || retry != nil {
			t.Fatalf("native bank expiry must finish without any claim-prefix lock: %s %v", disposition, err)
		}
	} else {
		state := map[string]string{"bank_transfer": "AWAITING_TRANSFER", "cash_on_delivery": "AWAITING_COLLECTION", "pay_at_pickup": "CONFIRMED"}[payment]
		out, callErr := oaCECancel(ctx, e, a, "oa-readmit-cancel-"+randomUUID(), state)
		if callErr != nil || out.OrderID != a.AttemptID || out.CommercialState != "CANCELLED" || out.ReleasedLines != 1 {
			t.Fatalf("native cancellation must finish without any claim-prefix lock: %+v %v", out, callErr)
		}
	}
	if err = holder.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	var remaining int64
	err = e.p.f.owner.QueryRow(ctx, `SELECT o.commercial_state='CANCELLED' AND a.state='COMMITTED' AND a.order_id=o.id
	 AND x.state='placed' AND x.order_id=o.id AND x.create_attempt_id=a.attempt_id
	 AND p.revoked_at IS NOT NULL AND r.state=$4,
	 l.quantity-(SELECT coalesce(sum(u.quantity),0) FROM claims.live_price_uses u JOIN checkout.orders z
	  ON z.tenant_id=u.tenant_id AND z.store_id=u.store_id AND z.id=u.order_id AND z.commercial_state<>'CANCELLED'
	  WHERE u.tenant_id=l.tenant_id AND u.store_id=l.store_id AND u.bundle_id=l.bundle_id AND u.offer_id=l.offer_id)
	 FROM ops.order_attempts a JOIN checkout.orders o ON o.id=a.order_id JOIN inventory.reservations r ON r.id=o.id
	 JOIN checkout.order_pay_entries p ON p.order_id=o.id JOIN inbox.order_for_buyer x ON x.create_attempt_id=a.attempt_id AND x.bundle_id=$2
	 JOIN claims.lines l ON l.bundle_id=$2 AND l.offer_id=$3 WHERE a.attempt_id=$1`,
		a.AttemptID, bundle, offer.ID, map[bool]string{true: "EXPIRED", false: "RELEASED"}[mode == "expiry"]).Scan(&exact, &remaining)
	if err != nil || !exact || remaining != 2 {
		t.Fatalf("durably cancelled exact old order restores allowance without touching claim reservation: exact=%v remaining=%d err=%v", exact, remaining, err)
	}
	oaCancelReadmitRetained(t, f)
	return f
}

func oaCancelReadmitRetained(t *testing.T, f oaCancelReadmitFixture) {
	t.Helper()
	if !bytes.Equal(f.terminal, oaCEAttemptBytes(t, f.e, f.attempt)) || f.audit != oaCancelReadmitAudit(t, f) {
		t.Fatal("new admission/cancel changed the old terminal receipt, reservation binding, origins, live uses or consumed grant audit")
	}
	again, err := f.executor.Execute(context.Background(), f.e.token(), f.e.store(), f.attempt.AttemptID, f.attempt.Generation)
	if err != nil {
		t.Fatal("old cancelled intent replay", err)
	}
	oaExecutorSameReceipt(t, f.receipt, again)
}

func TestOrderAttemptCancelReadmitFreshCAS(t *testing.T) {
	for _, mode := range []string{"bank_transfer", "pay_at_pickup", "cash_on_delivery", "expiry", "catalog_only"} {
		t.Run(mode, func(t *testing.T) {
			payment := mode
			if mode == "catalog_only" {
				payment = "bank_transfer"
			}
			f := oaCancelReadmitSetup(t, payment, mode != "catalog_only")
			next := f.request
			next.ExpectedGeneration = f.attempt.Generation
			next.PreviousAttemptID = &f.attempt.AttemptID
			next.TransportKey = "oa-readmit-fresh-" + randomUUID()
			a, err := f.svc.Admit(context.Background(), f.e.token(), f.e.store(), next)
			if err != nil || a.State != "PENDING" || a.Replay || a.AttemptID == f.attempt.AttemptID ||
				a.ScopeID != f.attempt.ScopeID || a.Generation != f.attempt.Generation+1 {
				t.Fatalf("fresh explicit CAS must admit the cancelled exact bundle: %+v err=%v", a, err)
			}
			var exact bool
			err = f.e.p.f.owner.QueryRow(context.Background(), `SELECT
			 (SELECT count(*) FROM inbox.order_for_buyer WHERE bundle_id=$2 AND state IN ('pending','placed'))=1
			 AND EXISTS(SELECT 1 FROM inbox.order_for_buyer WHERE create_attempt_id=$1 AND request_id=$1 AND bundle_id=$2
			  AND state='pending' AND order_id IS NULL)
			 AND EXISTS(SELECT 1 FROM inbox.order_for_buyer WHERE create_attempt_id=$3 AND bundle_id=$2 AND state='released' AND order_id=$3)
			 AND NOT EXISTS(SELECT 1 FROM checkout.orders WHERE create_attempt_id=$1)`, a.AttemptID, f.bundle, f.attempt.AttemptID).Scan(&exact)
			if err != nil || !exact {
				t.Fatalf("C1 frees only old placed exact CANCELLED binding while retaining its order pointer: %v", err)
			}
			oaCancelReadmitRetained(t, f)
			replayed, err := f.svc.Admit(context.Background(), f.e.token(), f.e.store(), next)
			if err != nil || !replayed.Replay || replayed.AttemptID != a.AttemptID || replayed.Generation != a.Generation {
				t.Fatalf("new key must replay its exact new attempt: %+v err=%v", replayed, err)
			}
			got, err := f.executor.Execute(context.Background(), f.e.token(), f.e.store(), a.AttemptID, a.Generation)
			if err != nil {
				t.Fatal("actual second C2 on same cancelled bundle", err)
			}
			oaExecutorReceipt(t, got, a, "COMMITTED", "")
			err = f.e.p.f.owner.QueryRow(context.Background(), `SELECT
			 (SELECT count(*) FROM claims.order_origins x JOIN checkout.orders o ON o.id=x.order_id
			  WHERE x.bundle_id=$2 AND x.offer_id=$4 AND o.commercial_state<>'CANCELLED')=1
			 AND EXISTS(SELECT 1 FROM inbox.order_for_buyer WHERE bundle_id=$2 AND create_attempt_id=$1 AND state='placed' AND order_id=$1)
			 AND (SELECT count(*) FROM claims.live_price_uses WHERE bundle_id=$2 AND offer_id=$4 AND order_id IN ($1,$3) AND quantity=2)=$5
			 AND (SELECT coalesce(sum(u.quantity),0) FROM claims.live_price_uses u JOIN checkout.orders o ON o.id=u.order_id
			  WHERE u.bundle_id=$2 AND u.offer_id=$4 AND o.commercial_state<>'CANCELLED')=$6
			 AND (SELECT count(*) FROM claims.merchant_origin_grants WHERE create_attempt_id=$1 AND consumed_at IS NOT NULL)=$7`,
				a.AttemptID, f.bundle, f.attempt.AttemptID, f.offer, map[bool]int{true: 2, false: 0}[f.priced],
				map[bool]int{true: 2, false: 0}[f.priced], map[bool]int{true: 1, false: 0}[f.priced]).Scan(&exact)
			if err != nil || !exact {
				t.Fatalf("second order consumes the restored allowance once and retains both historical origin/use rows: %v", err)
			}
			oaCancelReadmitRetained(t, f)
		})
	}
}

func TestOrderAttemptCancelReadmitLegacyBegin(t *testing.T) {
	f := oaCancelReadmitSetup(t, "bank_transfer", true)
	var request, reason string
	err := platform.WithScope(context.Background(), f.e.p.f.runtime, f.e.token(), f.e.store(), "store:read", func(tx pgx.Tx, _ platform.Scope) error {
		return tx.QueryRow(context.Background(), `SELECT o_request::text,o_reason FROM claims.for_buyer_begin($1,$2,$3,ARRAY[$4::uuid],$5)`,
			randomBytes(32), randomBytes(32), f.conversation, f.bundle, f.plan.BuyerOwnerID).Scan(&request, &reason)
	})
	if err != nil || request == "" || request == f.attempt.AttemptID || reason != "" {
		t.Fatalf("legacy fresh reservation must accept the durably CANCELLED exact attempt-owned order: request=%s reason=%s err=%v", request, reason, err)
	}
	var exact bool
	err = f.e.p.f.owner.QueryRow(context.Background(), `SELECT
	 EXISTS(SELECT 1 FROM inbox.order_for_buyer WHERE bundle_id=$1 AND request_id=$2 AND create_attempt_id IS NULL AND state='pending' AND order_id IS NULL)
	 AND EXISTS(SELECT 1 FROM inbox.order_for_buyer WHERE bundle_id=$1 AND create_attempt_id=$3 AND state='released' AND order_id=$3)
	 AND (SELECT count(*) FROM inbox.order_for_buyer WHERE bundle_id=$1 AND state IN ('pending','placed'))=1
	 AND EXISTS(SELECT 1 FROM claims.merchant_origin_grants WHERE bundle_id=$1 AND request_id=$2 AND create_attempt_id IS NULL AND consumed_at IS NULL)
	 AND NOT EXISTS(SELECT 1 FROM checkout.orders o JOIN claims.order_origins x ON x.order_id=o.id
	  WHERE x.bundle_id=$1 AND o.commercial_state<>'CANCELLED')`, f.bundle, request, f.attempt.AttemptID).Scan(&exact)
	if err != nil || !exact {
		t.Fatalf("legacy path must retain old exact order audit and create one independent pending request: %v", err)
	}
	oaCancelReadmitRetained(t, f)
}

func TestOrderAttemptCancelReadmitConcurrentPrincipals(t *testing.T) {
	f := oaCancelReadmitSetup(t, "bank_transfer", true)
	_, tokenA := lcPrincipal(t, f.e.p.f, f.e.tenant(), []string{f.e.store()}, "store:read", "inventory:reserve", "catalog:read", "orders:read", "live:manage", "inbox:reply", "inbox:read", "customers:read")
	_, tokenB := lcPrincipal(t, f.e.p.f, f.e.tenant(), []string{f.e.store()}, "store:read", "inventory:reserve", "catalog:read", "orders:read", "live:manage", "inbox:reply", "inbox:read", "customers:read")
	tokens := []string{tokenA, tokenB}
	requests := []orderattempt.AdmissionRequest{oaCrossRequest(t, f.e, tokenA, f.selection), oaCrossRequest(t, f.e, tokenB, f.selection)}
	if requests[0].ExpectedScopeID == requests[1].ExpectedScopeID {
		t.Fatal("concurrent readmission needs independent authenticated scopes")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	holder, err := f.e.p.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	oaAbortCleanupTx(t, holder)
	oaStateExec(t, holder, `SELECT id FROM claims.bundles WHERE id=$1 FOR UPDATE`, f.bundle)
	type outcome struct {
		index   int
		receipt orderattempt.AdmissionReceipt
		err     error
	}
	done := make(chan outcome, 2)
	finished := make(chan struct{})
	var wg sync.WaitGroup
	for i := range tokens {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			a, callErr := f.svc.Admit(ctx, tokens[index], f.e.store(), requests[index])
			done <- outcome{index, a, callErr}
		}(i)
	}
	go func() { wg.Wait(); close(finished) }()
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		_ = holder.Rollback(cleanup)
		cancel()
		select {
		case <-finished:
		case <-cleanup.Done():
			t.Error("concurrent admission did not stop within cleanup bound")
		}
	}()
	oaExecutorWaitBlocked(t, f.e, ctx, holder, 2, finished)
	if err = holder.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	var winner outcome
	accepted, refused := 0, 0
	for range tokens {
		select {
		case out := <-done:
			if out.err == nil {
				accepted++
				winner = out
				if out.receipt.State != "PENDING" || out.receipt.Replay || out.receipt.Generation != 1 || out.receipt.ScopeID != requests[out.index].ExpectedScopeID {
					t.Fatalf("concurrent C1 returned invalid fresh admission: %+v", out.receipt)
				}
			} else {
				refused++
				oaCrossBundleRefusal(t, out.receipt, out.err, "bundle_already_ordered")
			}
		case <-ctx.Done():
			t.Fatal("concurrent readmissions did not complete after native bundle unlock")
		}
	}
	if accepted != 1 || refused != 1 {
		t.Fatalf("cancelled bundle must admit exactly one concurrent fresh key: accepted=%d refused=%d", accepted, refused)
	}
	got, err := f.executor.Execute(ctx, tokens[winner.index], f.e.store(), winner.receipt.AttemptID, winner.receipt.Generation)
	if err != nil {
		t.Fatal("execute sole concurrent winner", err)
	}
	oaExecutorReceipt(t, got, winner.receipt, "COMMITTED", "")
	var exact bool
	err = f.e.p.f.owner.QueryRow(ctx, `SELECT
	 (SELECT count(*) FROM inbox.order_for_buyer WHERE bundle_id=$1 AND state IN ('pending','placed'))=1
	 AND (SELECT count(*) FROM claims.order_origins x JOIN checkout.orders o ON o.id=x.order_id WHERE x.bundle_id=$1 AND o.commercial_state<>'CANCELLED')=1
	 AND EXISTS(SELECT 1 FROM inbox.order_for_buyer WHERE bundle_id=$1 AND create_attempt_id=$2 AND order_id=$2 AND state='placed')
	 AND EXISTS(SELECT 1 FROM inbox.order_for_buyer WHERE bundle_id=$1 AND create_attempt_id=$3 AND order_id=$3 AND state='released')
	 AND (SELECT count(*) FROM ops.order_attempts WHERE scope_id IN ($4,$5))=1`,
		f.bundle, winner.receipt.AttemptID, f.attempt.AttemptID, requests[0].ExpectedScopeID, requests[1].ExpectedScopeID).Scan(&exact)
	if err != nil || !exact {
		t.Fatalf("concurrent new keys created another live order/reservation or stole the old pointer: %v", err)
	}
	oaCancelReadmitRetained(t, f)
	// Body/transport-key replay remains immutable for the old COMMITTED intent after another principal's success.
	old, err := f.svc.Admit(ctx, f.e.token(), f.e.store(), f.request)
	if err != nil || !old.Replay || old.State != "COMMITTED" || old.AttemptID != f.attempt.AttemptID ||
		!json.Valid(old.Receipt) {
		t.Fatalf("old exact key lost its cancelled COMMITTED replay: %+v err=%v", old, err)
	}
	oaExecutorSameReceipt(t, f.receipt, orderattempt.ExecutionReceipt{State: old.State, Receipt: old.Receipt})
}
