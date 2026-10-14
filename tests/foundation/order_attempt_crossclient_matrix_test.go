// Purpose: independent REAL_PG cross-principal OA1 acceptance for frozen §6b/j.
// Depends on: real Service.Admit/Executor, native merchant sessions, lbu fixtures and SQL row barriers.
// Used by: parent-owned CI tests-first acceptance; no product hooks, provider calls or local execution.
// Invariants: one bundle reservation cannot change owners; independent scopes sharing a buyer
// commit once in distinct attempt carts without reading, locking or changing the storefront cart.
package foundation_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"livecommerce/internal/orderattempt"
)

func oaCrossRequest(t *testing.T, e *lbuEnv, token string, selection map[string]any) orderattempt.AdmissionRequest {
	t.Helper()
	var raw []byte
	if err := e.p.f.runtime.QueryRow(context.Background(), `SELECT ops.read_order_attempt_scope($1,$2)`, tokenHash(token), e.store()).Scan(&raw); err != nil {
		t.Fatal("real authenticated scope", err)
	}
	var scope oaScopeReceipt
	if err := json.Unmarshal(raw, &scope); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(selection)
	if err != nil {
		t.Fatal(err)
	}
	if scope.ScopeID == "" || scope.Generation != 0 {
		t.Fatal("fresh principal scope positive sentinel missing")
	}
	return orderattempt.AdmissionRequest{ExpectedScopeID: scope.ScopeID, ExpectedGeneration: 0,
		TransportKey: "oa-cross-" + randomUUID(), Selection: body}
}

func oaCrossAdmit(t *testing.T, e *lbuEnv, svc *orderattempt.Service, token string, req orderattempt.AdmissionRequest) orderattempt.AdmissionReceipt {
	t.Helper()
	a, err := svc.Admit(context.Background(), token, e.store(), req)
	if err != nil || a.State != "PENDING" || a.Replay || a.AttemptID == "" || a.ScopeID != req.ExpectedScopeID || a.Generation != 1 {
		t.Fatalf("native C1 positive sentinel: state=%s generation=%d err=%v", a.State, a.Generation, err)
	}
	return a
}

func oaCrossBundleRefusal(t *testing.T, out orderattempt.AdmissionReceipt, err error, code string) {
	t.Helper()
	var denied *orderattempt.AdmissionError
	if !errors.As(err, &denied) || denied.SQLState() != "PT409" || denied.Code() != code || denied.OutcomeUnknown() || out.AttemptID != "" || out.ScopeID != "" || len(out.Receipt) != 0 {
		t.Fatalf("competing principal must receive exact non-admission refusal %s: %v", code, err)
	}
}

// The existing complete-C2 helper pins bank-transfer money, holds, credentials and
// expiry identity but assumes fixture principalA. This oracle uses each actual actor.
func oaCrossDurable(t *testing.T, e *lbuEnv, a orderattempt.AdmissionReceipt, actor string, got orderattempt.ExecutionReceipt, quantity, total int64) string {
	t.Helper()
	var exact bool
	var cart string
	err := e.p.f.owner.QueryRow(context.Background(), `SELECT
	 a.state='COMMITTED' AND a.receipt=$2::jsonb AND a.order_id=a.attempt_id AND a.settled_at IS NOT NULL
	 AND a.plan_ciphertext IS NULL AND a.plan_key_version IS NULL AND s.principal_id=$3
	 AND o.id=a.attempt_id AND o.create_attempt_id=a.attempt_id AND o.owner_id=a.buyer_owner_id
	 AND o.tenant_id=a.tenant_id AND o.store_id=a.store_id AND o.creator_kind='order_attempt'
	 AND o.creator_session_id IS NULL AND o.source='merchant_manual' AND o.payment_mode='bank_transfer'
	 AND o.commercial_state='AWAITING_TRANSFER' AND o.total_minor=$4 AND o.total_minor=(q.snapshot->'amount'->>'total_minor')::bigint AND o.generation=1
	 AND c.cart_kind='order_attempt' AND c.create_attempt_id=a.attempt_id AND c.owner_id=a.buyer_owner_id
	 AND c.creator_session_id IS NULL AND q.create_attempt_id=a.attempt_id AND q.cart_id=c.id
	 AND r.id=o.id AND r.create_attempt_id=a.attempt_id AND r.buyer_owner_id=a.buyer_owner_id
	 AND r.creator_kind='order_attempt' AND r.buyer_session_id IS NULL AND r.state='HELD' AND r.generation=1
	 AND p.order_id=o.id AND p.create_attempt_id=a.attempt_id AND p.owner_id=a.buyer_owner_id
	 AND p.link_version=1 AND p.revoked_at IS NULL AND octet_length(p.token_hash)=32 AND octet_length(p.credential_ciphertext)=71
	 AND p.expires_at=LEAST(o.expires_at,p.issued_at+interval '72 hours') AND p.expires_at>clock_timestamp()
	 AND j.kind='checkout_expiry_v1' AND j.args->>'order_id'=o.id::text
	 AND aj.kind='order_attempt_v1' AND aj.args=jsonb_build_object('attempt_id',a.attempt_id,'intent_generation',a.intent_generation)
	 AND (SELECT count(*) FROM ops.audit_events x WHERE x.tenant_id=a.tenant_id AND x.store_id=a.store_id
	  AND x.principal_id=$3 AND x.action='order.manual_created' AND x.details=jsonb_build_object(
	   'attempt_id',a.attempt_id,'order_id',a.attempt_id,'intent_generation',a.intent_generation))=1,
	 c.id::text FROM ops.order_attempts a JOIN ops.order_attempt_scopes s ON s.scope_id=a.scope_id
	 JOIN checkout.orders o ON o.id=a.order_id JOIN storefront.quotes q ON q.id=o.quote_id
	 JOIN storefront.carts c ON c.id=q.cart_id JOIN inventory.reservations r ON r.id=o.id
	 JOIN checkout.order_pay_entries p ON p.order_id=o.id JOIN river_expiry.river_job j ON j.id=o.job_id
	 JOIN river.river_job aj ON aj.id=a.execution_job_id WHERE a.attempt_id=$1`, a.AttemptID, string(got.Receipt), actor, total).Scan(&exact, &cart)
	if err != nil || !exact || cart == "" {
		t.Fatalf("exact durable cross-principal order/cart/hold/pay/job/audit receipts: exact=%v err=%v", exact, err)
	}
	var counts []int64
	err = e.p.f.owner.QueryRow(context.Background(), `SELECT ARRAY[
	 (SELECT count(*) FROM checkout.orders WHERE create_attempt_id=$1),
	 (SELECT count(*) FROM storefront.carts WHERE create_attempt_id=$1),
	 (SELECT count(*) FROM storefront.quotes WHERE create_attempt_id=$1),
	 (SELECT count(*) FROM storefront.destination_snapshots WHERE create_attempt_id=$1),
	 (SELECT count(*) FROM inventory.reservations WHERE create_attempt_id=$1),
	 (SELECT count(*) FROM checkout.command_results WHERE create_attempt_id=$1),
	 (SELECT count(*) FROM checkout.order_pay_entries WHERE create_attempt_id=$1),
	 (SELECT count(*) FROM checkout.bank_transfers WHERE order_id=$1),
	 (SELECT count(*) FROM river_expiry.river_job WHERE args->>'order_id'=($1::uuid)::text),
	 (SELECT count(*) FROM river.river_job WHERE kind='order_attempt_v1' AND args->>'attempt_id'=($1::uuid)::text),
	 (SELECT coalesce(sum(quantity),0) FROM inventory.reservation_lines WHERE reservation_id=$1 AND sku_id=$2),
	 (SELECT coalesce(sum(delta_reserved),0) FROM inventory.ledger WHERE create_attempt_id=$1 AND sku_id=$2 AND actor_kind='ORDER_ATTEMPT'),
	 (SELECT count(*) FROM ops.order_attempt_subjects WHERE attempt_id=$1),
	 (SELECT count(*) FROM inbox.order_payment_notifications WHERE create_attempt_id=$1),
	 (SELECT count(*) FROM integration.operations WHERE action='meta.dm_send' AND request->>'order_id'=($1::uuid)::text)
	]`, a.AttemptID, e.money).Scan(&counts)
	want := []int64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, quantity, quantity, 0, 0, 0}
	if err != nil || !reflect.DeepEqual(counts, want) {
		t.Fatalf("effects and native jobs must exist exactly once: got=%v want=%v err=%v", counts, want, err)
	}
	return cart
}

func TestOrderAttemptCrossClientDifferentPrincipalsCannotStealBundle(t *testing.T) {
	e := lbuNew(t)
	actorA, tokenA := lcPrincipal(t, e.p.f, e.tenant(), []string{e.store()}, "store:read", "inventory:reserve", "catalog:read", "orders:read", "live:manage", "inbox:reply", "inbox:read", "customers:read")
	actorB, tokenB := lcPrincipal(t, e.p.f, e.tenant(), []string{e.store()}, "store:read", "inventory:reserve", "catalog:read", "orders:read", "live:manage", "inbox:reply", "inbox:read", "customers:read")
	_, offer, bundle, conversation := e.scenario()
	svc, err := orderattempt.NewService(e.p.f.runtime, oaServiceKeys(t))
	if err != nil {
		t.Fatal(err)
	}
	requestA := oaCrossRequest(t, e, tokenA, oaClaimSelection(e, []string{bundle}, []string{offer.ID}, conversation))
	requestB := oaCrossRequest(t, e, tokenB, oaClaimSelection(e, []string{bundle}, []string{offer.ID}, conversation))
	if actorA == actorB || requestA.ExpectedScopeID == requestB.ExpectedScopeID {
		t.Fatal("test requires distinct authenticated principals and scopes")
	}
	a := oaCrossAdmit(t, e, svc, tokenA, requestA)
	plan, _ := oaMaterialPlan(t, e, oaServiceKeys(t), a.AttemptID)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	holder, err := e.p.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	oaAbortCleanupTx(t, holder)
	oaStateExec(t, holder, `SELECT id FROM claims.bundles WHERE id=$1 FOR UPDATE`, bundle)
	type admissionOutcome struct {
		receipt orderattempt.AdmissionReceipt
		err     error
	}
	done := make(chan admissionOutcome, 1)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		out, callErr := svc.Admit(ctx, tokenB, e.store(), requestB)
		done <- admissionOutcome{out, callErr}
	}()
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		_ = holder.Rollback(cleanup)
		cancel()
		select {
		case <-finished:
		case <-cleanup.Done():
			t.Error("competing C1 did not stop within cleanup bound")
		}
	}()
	oaExecutorWaitBlocked(t, e, ctx, holder, 1, finished)
	var pending bool
	err = e.p.f.owner.QueryRow(ctx, `SELECT x.create_attempt_id=$1 AND x.request_id=$1 AND x.principal_id=$3
	 AND x.buyer_id=a.buyer_owner_id AND x.state='pending' AND x.order_id IS NULL AND x.expires_at=a.execute_by
	 AND g.create_attempt_id=$1 AND g.request_id=$1 AND g.principal_id=$3 AND g.buyer_id=a.buyer_owner_id
	 AND g.expires_at=a.execute_by AND g.consumed_at IS NULL
	 FROM inbox.order_for_buyer x JOIN ops.order_attempts a ON a.attempt_id=x.create_attempt_id
	 JOIN claims.merchant_origin_grants g ON g.create_attempt_id=a.attempt_id AND g.bundle_id=x.bundle_id
	 WHERE x.bundle_id=$2 AND x.state='pending'`, a.AttemptID, bundle, actorA).Scan(&pending)
	if err != nil || !pending {
		t.Fatalf("winner's native pending reservation/grant positive sentinel: %v", err)
	}
	if err = holder.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case out := <-done:
		oaCrossBundleRefusal(t, out.receipt, out.err, "bundle_already_ordered")
	case <-ctx.Done():
		t.Fatal("competing principal did not return from the native bundle barrier")
	}
	executor := oaExecutorNew(t, e.p.f.runtime, oaServiceKeys(t))
	got, err := executor.Execute(ctx, tokenA, e.store(), a.AttemptID, a.Generation)
	if err != nil {
		t.Fatal("native winner C2", err)
	}
	oaExecutorReceipt(t, got, a, "COMMITTED", "")
	oaCrossDurable(t, e, a, actorA, got, 2, plan.Calculation.TotalMinor)
	var remaining int64
	var settledPlaced, settledUses int
	err = e.p.f.owner.QueryRow(ctx, `SELECT l.quantity-(SELECT coalesce(sum(u.quantity),0)
	 FROM claims.live_price_uses u JOIN checkout.orders o ON o.tenant_id=u.tenant_id AND o.store_id=u.store_id AND o.id=u.order_id
	 WHERE u.tenant_id=l.tenant_id AND u.store_id=l.store_id AND u.bundle_id=l.bundle_id AND u.offer_id=l.offer_id AND o.commercial_state<>'CANCELLED'),
	 (SELECT count(*) FROM inbox.order_for_buyer WHERE bundle_id=$2 AND create_attempt_id=$1 AND request_id=$1 AND principal_id=$4 AND order_id=$1 AND state='placed'),
	 (SELECT count(*) FROM claims.live_price_uses WHERE order_id=$1 AND bundle_id=$2 AND offer_id=$3 AND quantity=2)
	 FROM claims.lines l WHERE l.bundle_id=$2 AND l.offer_id=$3 AND l.sku_id=$5`, a.AttemptID, bundle, offer.ID, actorA, e.money).Scan(&remaining, &settledPlaced, &settledUses)
	if err != nil || remaining != 0 || settledPlaced != 1 || settledUses != 1 {
		t.Fatalf("loaded settled claim must be exhausted by the exact winner: remaining=%d placed=%d uses=%d err=%v", remaining, settledPlaced, settledUses, err)
	}
	// This is the internal C1 Service refusal, not OA3's public bundle_order_exists
	// envelope mapping. The loaded exhausted origin hits claim_quantity_exceeded
	// before the occupied-bundle predicate; its fixed opaque PT409 is phase-specific.
	for i := 0; i < 2; i++ {
		out, callErr := svc.Admit(ctx, tokenB, e.store(), requestB)
		oaCrossBundleRefusal(t, out, callErr, "order_attempt_conflict")
	}
	var placed, origins, uses, grants, loserAttempts int
	err = e.p.f.owner.QueryRow(ctx, `SELECT
	 (SELECT count(*) FROM inbox.order_for_buyer WHERE bundle_id=$2 AND create_attempt_id=$1 AND request_id=$1 AND principal_id=$3 AND order_id=$1 AND state='placed'),
	 (SELECT count(*) FROM claims.order_origins WHERE order_id=$1 AND bundle_id=$2 AND offer_id=$4),
	 (SELECT count(*) FROM claims.live_price_uses WHERE order_id=$1 AND bundle_id=$2 AND offer_id=$4 AND quantity=2),
	 (SELECT count(*) FROM claims.merchant_origin_grants WHERE create_attempt_id=$1 AND bundle_id=$2 AND consumed_at IS NOT NULL),
	 (SELECT count(*) FROM ops.order_attempts WHERE scope_id=$5)`, a.AttemptID, bundle, actorA, offer.ID, requestB.ExpectedScopeID).Scan(&placed, &origins, &uses, &grants, &loserAttempts)
	if err != nil || placed != 1 || origins != 1 || uses != 1 || grants != 1 || loserAttempts != 0 {
		t.Fatalf("bundle ownership/claim accounting changed or loser admitted: %d/%d/%d/%d/%d err=%v", placed, origins, uses, grants, loserAttempts, err)
	}
	var unchanged bool
	err = e.p.f.owner.QueryRow(ctx, `SELECT current_generation=0
	 AND (SELECT count(*) FROM inbox.order_for_buyer WHERE bundle_id=$2)=1
	 AND (SELECT count(*) FROM claims.merchant_origin_grants WHERE bundle_id=$2)=1
	 FROM ops.order_attempt_scopes WHERE scope_id=$1 AND principal_id=$3`, requestB.ExpectedScopeID, bundle, actorB).Scan(&unchanged)
	if err != nil || !unchanged {
		t.Fatalf("refusal advanced loser scope or duplicated bundle artifacts: %v", err)
	}
	again, err := executor.Execute(ctx, tokenA, e.store(), a.AttemptID, a.Generation)
	if err != nil {
		t.Fatal(err)
	}
	oaExecutorSameReceipt(t, got, again)
	oaCrossDurable(t, e, a, actorA, again, 2, plan.Calculation.TotalMinor)
}

func TestOrderAttemptCrossClientSameBuyerIndependentCarts(t *testing.T) {
	e := lbuNew(t)
	actorA, tokenA := lcPrincipal(t, e.p.f, e.tenant(), []string{e.store()}, "store:read", "inventory:reserve", "catalog:read", "orders:read", "inbox:read", "customers:read")
	actorB, tokenB := lcPrincipal(t, e.p.f, e.tenant(), []string{e.store()}, "store:read", "inventory:reserve", "catalog:read", "orders:read", "inbox:read", "customers:read")
	owner, conversation, browserSession, browserCart := randomUUID(), lcConversation(t, e.p.f, e.tenant(), e.store(), "page"), randomUUID(), randomUUID()
	mustExec(t, e.p.f.owner, `INSERT INTO buyer.owners(id,tenant_id,store_id) VALUES($1,$2,$3)`, owner, e.tenant(), e.store())
	mustExec(t, e.p.f.owner, `UPDATE inbox.conversation_state SET customer_id=$4 WHERE tenant_id=$1 AND store_id=$2 AND conversation_id=$3`, e.tenant(), e.store(), conversation, owner)
	// Native capability-session provenance, same as the existing materialization
	// fixture; neither C1 nor Executor receives this browser session as authority.
	mustExec(t, e.p.f.owner, `INSERT INTO buyer.capability_sessions(id,tenant_id,store_id,owner_id,token_hash,expires_at)
	 VALUES($1,$2,$3,$4,sha256(convert_to(($1::uuid)::text,'UTF8')),clock_timestamp()+interval '1 hour')`, browserSession, e.tenant(), e.store(), owner)
	mustExec(t, e.p.f.owner, `INSERT INTO storefront.carts(tenant_id,store_id,owner_id,id,creator_session_id,currency,version)
	 VALUES($1,$2,$3,$4,$5,'TWD',7)`, e.tenant(), e.store(), owner, browserCart, browserSession)
	mustExec(t, e.p.f.owner, `INSERT INTO storefront.cart_lines(tenant_id,store_id,owner_id,cart_id,sku_id,quantity)
	 VALUES($1,$2,$3,$4,$5,1)`, e.tenant(), e.store(), owner, browserCart, e.money)
	svc, err := orderattempt.NewService(e.p.f.runtime, oaServiceKeys(t))
	if err != nil {
		t.Fatal(err)
	}
	var attempts [2]orderattempt.AdmissionReceipt
	var totals [2]int64
	for i, token := range []string{tokenA, tokenB} {
		selection := oaFactsSelection(e)
		selection["for"] = map[string]any{"bundle_ids": []string{}, "conversation_id": conversation, "customer_id": owner}
		attempts[i] = oaCrossAdmit(t, e, svc, token, oaCrossRequest(t, e, token, selection))
		plan, _ := oaMaterialPlan(t, e, oaServiceKeys(t), attempts[i].AttemptID)
		totals[i] = plan.Calculation.TotalMinor
	}
	if actorA == actorB || attempts[0].ScopeID == attempts[1].ScopeID || attempts[0].AttemptID == attempts[1].AttemptID {
		t.Fatal("independent actor/scope/intent positive sentinel missing")
	}
	// A buyer changes their own cart between C1 and C2. Freeze the full rows after
	// that change, then keep its lock held until both actual C2 transactions finish.
	mustExec(t, e.p.f.owner, `UPDATE storefront.cart_lines SET quantity=3 WHERE cart_id=$1`, browserCart)
	mustExec(t, e.p.f.owner, `UPDATE storefront.carts SET version=version+1 WHERE id=$1`, browserCart)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	snapshot := func() string {
		var raw string
		if err := e.p.f.owner.QueryRow(ctx, `SELECT jsonb_build_object('cart',to_jsonb(c),'lines',
		 (SELECT jsonb_agg(to_jsonb(l) ORDER BY l.sku_id) FROM storefront.cart_lines l WHERE l.cart_id=c.id))::text
		 FROM storefront.carts c WHERE c.id=$1 AND c.cart_kind='storefront' AND c.version=8
		 AND c.owner_id=$2 AND c.creator_session_id=$3 AND c.create_attempt_id IS NULL
		 AND EXISTS(SELECT 1 FROM storefront.cart_lines l WHERE l.cart_id=c.id AND l.sku_id=$4 AND l.quantity=3)`, browserCart, owner, browserSession, e.money).Scan(&raw); err != nil {
			t.Fatal("loaded versioned buyer storefront sentinel", err)
		}
		return raw
	}
	before := snapshot()
	cartLock, err := e.p.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	oaAbortCleanupTx(t, cartLock)
	oaStateExec(t, cartLock, `SELECT id FROM storefront.carts WHERE id=$1 FOR UPDATE`, browserCart)
	holder, err := e.p.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	oaAbortCleanupTx(t, holder)
	oaStateExec(t, holder, `SELECT id FROM buyer.owners WHERE id=$1 FOR UPDATE`, owner)
	executor := oaExecutorNew(t, e.p.f.runtime, oaServiceKeys(t))
	done := make(chan struct {
		i int
		oaExecutorOutcome
	}, 2)
	finished := make(chan struct{})
	var group sync.WaitGroup
	for i, token := range []string{tokenA, tokenB} {
		group.Add(1)
		go func(i int, token string) {
			defer group.Done()
			out, callErr := executor.Execute(ctx, token, e.store(), attempts[i].AttemptID, attempts[i].Generation)
			done <- struct {
				i int
				oaExecutorOutcome
			}{i, oaExecutorOutcome{out, callErr}}
		}(i, token)
	}
	go func() { group.Wait(); close(finished) }()
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		_ = holder.Rollback(cleanup)
		_ = cartLock.Rollback(cleanup)
		cancel()
		select {
		case <-finished:
		case <-cleanup.Done():
			t.Error("same-buyer executors did not stop within cleanup bound")
		}
	}()
	oaExecutorWaitBlocked(t, e, ctx, holder, 2, finished)
	if err = holder.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	var receipts [2]orderattempt.ExecutionReceipt
	for range attempts {
		select {
		case out := <-done:
			if out.err != nil {
				t.Fatalf("independent same-buyer C2 failed while storefront cart lock held: %v", out.err)
			}
			receipts[out.i] = out.receipt
		case <-ctx.Done():
			t.Fatal("C2 must complete before the storefront cart lock is released")
		}
	}
	var carts [2]string
	for i, actor := range []string{actorA, actorB} {
		oaExecutorReceipt(t, receipts[i], attempts[i], "COMMITTED", "")
		carts[i] = oaCrossDurable(t, e, attempts[i], actor, receipts[i], 5, totals[i])
		var actualOwner string
		if err = e.p.f.owner.QueryRow(ctx, `SELECT buyer_owner_id::text FROM ops.order_attempts WHERE attempt_id=$1`, attempts[i].AttemptID).Scan(&actualOwner); err != nil || actualOwner != owner {
			t.Fatalf("C1 did not persist the same explicit buyer owner: %v", err)
		}
		again, callErr := executor.Execute(ctx, []string{tokenA, tokenB}[i], e.store(), attempts[i].AttemptID, attempts[i].Generation)
		if callErr != nil {
			t.Fatal(callErr)
		}
		oaExecutorSameReceipt(t, receipts[i], again)
		oaCrossDurable(t, e, attempts[i], actor, again, 5, totals[i])
	}
	if carts[0] == carts[1] || carts[0] == browserCart || carts[1] == browserCart || snapshot() != before {
		t.Fatal("attempt carts collapsed, or buyer's full storefront cart/lines/version changed")
	}
	if err = cartLock.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
}
