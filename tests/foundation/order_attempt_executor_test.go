// Purpose: REAL_PG tests-first acceptance of the complete Go-owned C2 transaction.
// Depends on: real C1, actual runtime/claims logins, native domain rows and concrete keyrings/inbox.
// Used by: parent-owned OA1 CI; no test replaces a domain writer or starts a dispatcher.
// Invariants: one complete commit or zero effects, durable replay, no placement reauthorization,
// original plan-key custody, closed abort classification and bounded native lock barriers.
package foundation_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"livecommerce/internal/inbox"
	"livecommerce/internal/msgtemplates"
	"livecommerce/internal/orderattempt"
)

func oaExecutorNew(t *testing.T, pool *pgxpool.Pool, plans *orderattempt.PlanKeys) *orderattempt.Executor {
	t.Helper()
	// Synthetic keys configure the actual inbox implementation. No recording planner,
	// callback or fake-success notification hook is passed to the executor.
	keyJSON := `{"keys":[{"id":"oa_executor_fixture","key_base64":"` + base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{43}, 32)) + `"}]}`
	keys, err := inbox.LoadKeyring(func(name string) string {
		switch name {
		case "COMMERCE_META_PAYLOAD_ACTIVE_KEY_ID":
			return "oa_executor_fixture"
		case "COMMERCE_META_PAYLOAD_KEYS_JSON":
			return keyJSON
		default:
			return ""
		}
	})
	if err != nil {
		t.Fatal("real synthetic inbox keyring", err)
	}
	notifications, err := inbox.NewService(keys)
	if err != nil {
		t.Fatal(err)
	}
	seal, _ := mcnPageRing(t)
	jobs, err := river.NewClient[pgx.Tx](riverpgxv5.New(pool), &river.Config{Schema: "river"})
	if err != nil {
		t.Fatal(err)
	}
	notifications.EnableSend(seal, jobs, msgtemplates.NewService())
	executor, err := orderattempt.NewExecutor(pool, plans, payEntryTestKeys(t), notifications)
	if err != nil {
		t.Fatal("real executor construction", err)
	}
	return executor
}

func oaExecutorReceipt(t *testing.T, got orderattempt.ExecutionReceipt, a orderattempt.AdmissionReceipt, state, code string) {
	t.Helper()
	var receipt struct {
		State      string `json:"state"`
		AttemptID  string `json:"attempt_id"`
		Generation int64  `json:"intent_generation"`
		OrderID    string `json:"order_id"`
		Code       string `json:"code"`
	}
	if err := json.Unmarshal(got.Receipt, &receipt); err != nil {
		t.Fatal("terminal receipt decoding", err)
	}
	if got.State != state || receipt.State != state || receipt.AttemptID != a.AttemptID || receipt.Generation != a.Generation || receipt.Code != code {
		t.Fatal("executor did not return the exact durable terminal identity/disposition")
	}
	if state == "COMMITTED" && receipt.OrderID != a.AttemptID {
		t.Fatal("committed order must be the preallocated attempt ID")
	}
	if state == "ABORTED" && receipt.OrderID != "" {
		t.Fatal("aborted receipt disclosed an order identity")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(got.Receipt, &fields); err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{"plan", "ciphertext", "token", "buyer_link", "customer", "email", "phone", "address"} {
		if _, found := fields[private]; found {
			t.Fatalf("receipt contains forbidden field %s", private)
		}
	}
}

func oaExecutorSameReceipt(t *testing.T, first, again orderattempt.ExecutionReceipt) {
	t.Helper()
	var before, after any
	if json.Unmarshal(first.Receipt, &before) != nil || json.Unmarshal(again.Receipt, &after) != nil || first.State != again.State || !reflect.DeepEqual(before, after) {
		t.Fatal("replay changed the persisted terminal receipt")
	}
}

func oaExecutorCommitted(t *testing.T, e *lbuEnv, a orderattempt.AdmissionReceipt, plan orderattempt.FrozenPlan, got orderattempt.ExecutionReceipt, quantity int64) string {
	t.Helper()
	oaExecutorReceipt(t, got, a, "COMMITTED", "")
	var exact bool
	var entry string
	err := e.p.f.owner.QueryRow(context.Background(), `SELECT
	 a.state='COMMITTED' AND a.order_id=a.attempt_id AND a.receipt=$2::jsonb
	 AND a.plan_ciphertext IS NULL AND a.plan_key_version IS NULL AND a.settled_at IS NOT NULL
	 AND o.id=a.attempt_id AND o.create_attempt_id=a.attempt_id AND o.owner_id=a.buyer_owner_id
	 AND o.tenant_id=a.tenant_id AND o.store_id=a.store_id AND o.creator_kind='order_attempt'
	 AND o.creator_session_id IS NULL AND o.source='merchant_manual' AND o.payment_mode='bank_transfer'
	 AND o.commercial_state='AWAITING_TRANSFER' AND o.total_minor=$3 AND o.generation=1
	 AND p.order_id=o.id AND p.create_attempt_id=a.attempt_id AND p.owner_id=o.owner_id
	 AND p.link_version=1 AND p.revoked_at IS NULL AND p.expires_at=LEAST(o.expires_at,p.issued_at+interval '72 hours')
	 AND p.expires_at>clock_timestamp() AND octet_length(p.token_hash)=32 AND octet_length(p.credential_ciphertext)=71
	 AND r.id=o.id AND r.creator_kind='order_attempt' AND r.create_attempt_id=a.attempt_id
	 AND r.buyer_session_id IS NULL AND r.state='HELD' AND r.generation=1
	 AND j.id=o.job_id AND j.kind='checkout_expiry_v1' AND j.args->>'order_id'=o.id::text,
	 to_jsonb(p)::text
	 FROM ops.order_attempts a JOIN checkout.orders o ON o.id=a.order_id
	 JOIN checkout.order_pay_entries p ON p.order_id=o.id
	 JOIN inventory.reservations r ON r.id=o.id
	 JOIN river_expiry.river_job j ON j.id=o.job_id WHERE a.attempt_id=$1`,
		a.AttemptID, string(got.Receipt), plan.Calculation.TotalMinor).Scan(&exact, &entry)
	if err != nil || !exact {
		t.Fatalf("canonical native commit and credential/expiry identity: exact=%v err=%v", exact, err)
	}
	var countsRaw []byte
	err = e.p.f.owner.QueryRow(context.Background(), `SELECT jsonb_build_object(
	 'orders',(SELECT count(*) FROM checkout.orders WHERE create_attempt_id=$1),
	 'carts',(SELECT count(*) FROM storefront.carts WHERE create_attempt_id=$1),
	 'quotes',(SELECT count(*) FROM storefront.quotes WHERE create_attempt_id=$1),
	 'destinations',(SELECT count(*) FROM storefront.destination_snapshots WHERE create_attempt_id=$1),
	 'reservations',(SELECT count(*) FROM inventory.reservations WHERE create_attempt_id=$1),
	 'pay_entries',(SELECT count(*) FROM checkout.order_pay_entries WHERE create_attempt_id=$1),
	 'begin_receipts',(SELECT count(*) FROM checkout.command_results WHERE create_attempt_id=$1),
	 'bank_transfers',(SELECT count(*) FROM checkout.bank_transfers WHERE order_id=$1),
	 'expiry_jobs',(SELECT count(*) FROM river_expiry.river_job WHERE args->>'order_id'=($1::uuid)::text),
	 'subjects',(SELECT count(*) FROM ops.order_attempt_subjects WHERE attempt_id=$1),
	 'notifications',(SELECT count(*) FROM inbox.order_payment_notifications WHERE create_attempt_id=$1),
	 'operations',(SELECT count(*) FROM integration.operations WHERE action='meta.dm_send' AND request->>'order_id'=($1::uuid)::text),
	 'audit',(SELECT count(*) FROM ops.audit_events WHERE tenant_id=$2 AND store_id=$3 AND details->>'attempt_id'=($1::uuid)::text))`,
		a.AttemptID, e.tenant(), e.store()).Scan(&countsRaw)
	if err != nil {
		t.Fatal(err)
	}
	var counts map[string]int
	if err = json.Unmarshal(countsRaw, &counts); err != nil {
		t.Fatal(err)
	}
	want := map[string]int{"orders": 1, "carts": 1, "quotes": 1, "destinations": 1, "reservations": 1,
		"pay_entries": 1, "begin_receipts": 1, "bank_transfers": 1, "expiry_jobs": 1,
		"subjects": 0, "notifications": 0, "operations": 0, "audit": 1}
	if !reflect.DeepEqual(counts, want) {
		t.Fatalf("complete native write set counts: got %v want %v", counts, want)
	}
	var audit bool
	err = e.p.f.owner.QueryRow(context.Background(), `SELECT principal_id=$4 AND action='order.manual_created'
	 AND details=jsonb_build_object('attempt_id',$1::text,'order_id',$1::text,'intent_generation',$5::bigint)
	 FROM ops.audit_events WHERE tenant_id=$2 AND store_id=$3 AND details->>'attempt_id'=$1::text`,
		a.AttemptID, e.tenant(), e.store(), e.p.f.principalA, a.Generation).Scan(&audit)
	if err != nil || !audit {
		t.Fatalf("native audit must use admitted actor and opaque details only: %v", err)
	}
	var reserved, ledger int64
	err = e.p.f.owner.QueryRow(context.Background(), `SELECT
	 (SELECT coalesce(sum(quantity),0) FROM inventory.reservation_lines WHERE reservation_id=$1 AND sku_id=$2),
	 (SELECT coalesce(sum(delta_reserved),0) FROM inventory.ledger WHERE create_attempt_id=$1 AND sku_id=$2 AND actor_kind='ORDER_ATTEMPT')`, a.AttemptID, e.money).Scan(&reserved, &ledger)
	if err != nil || reserved != quantity || ledger != quantity {
		t.Fatalf("tracked SKU must reserve aggregated frozen quantity once: reserved=%d ledger=%d err=%v", reserved, ledger, err)
	}
	return entry
}

func oaExecutorNoEffects(t *testing.T, e *lbuEnv, a orderattempt.AdmissionReceipt) {
	t.Helper()
	oaBeginZeroEffects(t, e, a.AttemptID)
	var count int
	err := e.p.f.owner.QueryRow(context.Background(), `SELECT
	 (SELECT count(*) FROM checkout.order_pay_entries WHERE create_attempt_id=$1)
	 +(SELECT count(*) FROM inbox.order_payment_notifications WHERE create_attempt_id=$1)
	 +(SELECT count(*) FROM claims.order_origins WHERE order_id=$1)
	 +(SELECT count(*) FROM claims.live_price_uses WHERE order_id=$1)
	 +(SELECT count(*) FROM ops.audit_events WHERE tenant_id=$2 AND store_id=$3 AND details->>'attempt_id'=($1::uuid)::text)
	 +(SELECT count(*) FROM integration.operations WHERE action='meta.dm_send' AND request->>'order_id'=($1::uuid)::text)`, a.AttemptID, e.tenant(), e.store()).Scan(&count)
	if err != nil || count != 0 {
		t.Fatalf("rolled-back C2 left pay/source/claim/audit/notification effects: count=%d err=%v", count, err)
	}
}

func oaExecutorAborted(t *testing.T, e *lbuEnv, a orderattempt.AdmissionReceipt, got orderattempt.ExecutionReceipt, code string) {
	t.Helper()
	oaExecutorReceipt(t, got, a, "ABORTED", code)
	oaExecutorNoEffects(t, e, a)
	var terminal bool
	err := e.p.f.owner.QueryRow(context.Background(), `SELECT state='ABORTED' AND receipt=$2::jsonb
	 AND order_id IS NULL AND plan_ciphertext IS NULL AND plan_key_version IS NULL AND settled_at IS NOT NULL
	 AND NOT EXISTS(SELECT 1 FROM ops.order_attempt_subjects s WHERE s.attempt_id=a.attempt_id)
	 FROM ops.order_attempts a WHERE attempt_id=$1`, a.AttemptID, string(got.Receipt)).Scan(&terminal)
	if err != nil || !terminal {
		t.Fatalf("abort not durably sealed after zero-effects proof: %v", err)
	}
}

func TestOrderAttemptExecutorNativeCommitAndLostResponseReplay(t *testing.T) {
	for _, worker := range []bool{false, true} {
		name := "runtime"
		if worker {
			name = "claims-worker"
		}
		t.Run(name, func(t *testing.T) {
			e := lbuNew(t)
			a := oaFenceAdmit(t, e)
			plan, _ := oaMaterialPlan(t, e, oaServiceKeys(t), a.AttemptID)
			pool, token, store := e.p.f.runtime, e.token(), e.store()
			if worker {
				pool, token, store = miPool(t, e.p.f, "commerce_claims_worker"), "", ""
			}
			executor := oaExecutorNew(t, pool, oaServiceKeys(t))
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			first, err := executor.Execute(ctx, token, store, a.AttemptID, a.Generation)
			if err != nil {
				t.Fatalf("actual %s execution: %v", name, err)
			}
			original := oaExecutorCommitted(t, e, a, plan, first, 5)
			// No receipt/body is supplied on retry, as when the caller lost its response.
			// This is a replay oracle; it does not claim a network commit-ack fault injection.
			again, err := executor.Execute(ctx, token, store, a.AttemptID, a.Generation)
			if err != nil {
				t.Fatal("retry after discarded transport result", err)
			}
			oaExecutorSameReceipt(t, first, again)
			if repeated := oaExecutorCommitted(t, e, a, plan, again, 5); repeated != original {
				t.Fatal("replay rotated or rewrote the native pay entry")
			}
		})
	}
}

func TestOrderAttemptExecutorCompletesNativeClaims(t *testing.T) {
	e := lbuNew(t)
	_, offer, bundle, conversation := e.scenario()
	a := oaAllocationAdmit(t, e, oaClaimSelection(e, []string{bundle}, []string{offer.ID}, conversation))
	plan, _ := oaMaterialPlan(t, e, oaServiceKeys(t), a.AttemptID)
	executor := oaExecutorNew(t, miPool(t, e.p.f, "commerce_claims_worker"), oaServiceKeys(t))
	got, err := executor.Execute(context.Background(), "", "", a.AttemptID, a.Generation)
	if err != nil {
		t.Fatal("execute actual admitted claim", err)
	}
	oaExecutorCommitted(t, e, a, plan, got, 2)
	var placed, origins, uses, consumed int
	err = e.p.f.owner.QueryRow(context.Background(), `SELECT
	 (SELECT count(*) FROM inbox.order_for_buyer WHERE create_attempt_id=$1 AND request_id=$1 AND order_id=$1 AND state='placed'),
	 (SELECT count(*) FROM claims.order_origins WHERE order_id=$1 AND bundle_id=$2 AND offer_id=$3),
	 (SELECT count(*) FROM claims.live_price_uses WHERE order_id=$1 AND bundle_id=$2 AND offer_id=$3 AND quantity=2),
	 (SELECT count(*) FROM claims.merchant_origin_grants g JOIN checkout.orders o ON o.quote_id=g.quote_id
	  WHERE g.create_attempt_id=$1 AND g.bundle_id=$2 AND g.consumed_at IS NOT NULL AND o.id=$1)`, a.AttemptID, bundle, offer.ID).Scan(&placed, &origins, &uses, &consumed)
	if err != nil || placed != 1 || origins != 1 || uses != 1 || consumed != 1 {
		t.Fatalf("native claim pipeline incomplete: placed=%d origins=%d uses=%d consumed=%d err=%v", placed, origins, uses, consumed, err)
	}
	again, err := executor.Execute(context.Background(), "", "", a.AttemptID, a.Generation)
	if err != nil {
		t.Fatal(err)
	}
	oaExecutorSameReceipt(t, got, again)
}

type oaExecutorOutcome struct {
	receipt orderattempt.ExecutionReceipt
	err     error
}

func oaExecutorWaitBlocked(t *testing.T, e *lbuEnv, ctx context.Context, holder pgx.Tx, count int, finished <-chan struct{}) {
	t.Helper()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		var blocked int
		// A second row-lock waiter can block behind the first waiter's tuple lock
		// rather than directly behind holder. Prove the complete native wait chain.
		err := e.p.f.owner.QueryRow(ctx, `WITH RECURSIVE blocked(pid) AS (
		 SELECT $1::int UNION SELECT a.pid FROM pg_stat_activity a JOIN blocked b
		 ON b.pid=ANY(pg_blocking_pids(a.pid)) WHERE a.wait_event_type='Lock')
		 SELECT count(*) FROM blocked WHERE pid<>$1::int`, int32(holder.Conn().PgConn().PID())).Scan(&blocked)
		if err != nil {
			t.Fatal(err)
		}
		if blocked >= count {
			return
		}
		select {
		case <-finished:
			t.Fatal("executor finished before reaching the native held-row barrier")
		case <-ctx.Done():
			t.Fatal("executor did not reach the native held-row barrier")
		case <-ticker.C:
		}
	}
}

func TestOrderAttemptExecutorInlineWorkerSerializeAndReplay(t *testing.T) {
	e := lbuNew(t)
	a := oaFenceAdmit(t, e)
	plan, _ := oaMaterialPlan(t, e, oaServiceKeys(t), a.AttemptID)
	inline := oaExecutorNew(t, e.p.f.runtime, oaServiceKeys(t))
	worker := oaExecutorNew(t, miPool(t, e.p.f, "commerce_claims_worker"), oaServiceKeys(t))
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	holder, err := e.p.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	oaAbortCleanupTx(t, holder)
	oaStateExec(t, holder, `SELECT attempt_id FROM ops.order_attempts WHERE attempt_id=$1 FOR UPDATE`, a.AttemptID)
	done := make(chan oaExecutorOutcome, 2)
	finished := make(chan struct{})
	var group sync.WaitGroup
	group.Add(2)
	go func() {
		defer group.Done()
		out, callErr := inline.Execute(ctx, e.token(), e.store(), a.AttemptID, a.Generation)
		done <- oaExecutorOutcome{out, callErr}
	}()
	go func() {
		defer group.Done()
		out, callErr := worker.Execute(ctx, "", "", a.AttemptID, a.Generation)
		done <- oaExecutorOutcome{out, callErr}
	}()
	go func() { group.Wait(); close(finished) }()
	defer func() {
		_ = holder.Rollback(context.Background())
		cancel()
		select {
		case <-finished:
		case <-time.After(3 * time.Second):
			t.Error("executor goroutines failed to stop")
		}
	}()
	oaExecutorWaitBlocked(t, e, ctx, holder, 2, finished)
	if err = holder.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	var results [2]orderattempt.ExecutionReceipt
	for i := range results {
		select {
		case got := <-done:
			if got.err != nil {
				t.Fatalf("concurrent native execution: %v", got.err)
			}
			results[i] = got.receipt
		case <-ctx.Done():
			t.Fatal("concurrent executor did not finish")
		}
	}
	oaExecutorSameReceipt(t, results[0], results[1])
	oaExecutorCommitted(t, e, a, plan, results[0], 5)
}

func TestOrderAttemptExecutorInsufficientStockAbortsWithoutEffects(t *testing.T) {
	e := lbuNew(t)
	a := oaFenceAdmit(t, e)
	// C1 succeeded with native stock. Exhaust availability before C2, leaving the
	// frozen prices and plan intact; the real Go allocator must refuse placement.
	mustExec(t, e.p.f.owner, `UPDATE inventory.balances SET on_hand=reserved+allocated+unavailable WHERE store_id=$1 AND sku_id=$2`, e.store(), e.money)
	executor := oaExecutorNew(t, e.p.f.runtime, oaServiceKeys(t))
	got, err := executor.Execute(context.Background(), e.token(), e.store(), a.AttemptID, a.Generation)
	if err != nil {
		t.Fatal("definitive native stock refusal must settle after rolling C2 back", err)
	}
	oaExecutorAborted(t, e, a, got, "insufficient_inventory")
	again, err := executor.Execute(context.Background(), e.token(), e.store(), a.AttemptID, a.Generation)
	if err != nil {
		t.Fatal(err)
	}
	oaExecutorSameReceipt(t, got, again)
}

func TestOrderAttemptExecutorTransientBalanceLockLeavesPending(t *testing.T) {
	e := lbuNew(t)
	a := oaFenceAdmit(t, e)
	plan, _ := oaMaterialPlan(t, e, oaServiceKeys(t), a.AttemptID)
	executor := oaExecutorNew(t, miPool(t, e.p.f, "commerce_claims_worker"), oaServiceKeys(t))
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	holder, err := e.p.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	oaAbortCleanupTx(t, holder)
	var held int
	err = holder.QueryRow(ctx, `SELECT count(*) FROM (SELECT warehouse_id FROM inventory.balances
	 WHERE store_id=$1 AND sku_id=$2 FOR UPDATE) locked`, e.store(), e.money).Scan(&held)
	if err != nil || held < 1 {
		t.Fatalf("native balance lock positive sentinel: %d %v", held, err)
	}
	done := make(chan oaExecutorOutcome, 1)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		out, callErr := executor.Execute(ctx, "", "", a.AttemptID, a.Generation)
		done <- oaExecutorOutcome{out, callErr}
	}()
	defer func() {
		_ = holder.Rollback(context.Background())
		cancel()
		select {
		case <-finished:
		case <-time.After(3 * time.Second):
			t.Error("blocked executor did not stop")
		}
	}()
	oaExecutorWaitBlocked(t, e, ctx, holder, 1, finished)
	select {
	case got := <-done:
		if got.err == nil || got.receipt.State != "" || len(got.receipt.Receipt) != 0 {
			t.Fatal("transient lock failure falsely returned a terminal result")
		}
		var failure *orderattempt.AdmissionError
		if !errors.As(got.err, &failure) || failure.SQLState() != "55P03" || failure.Code() != "order_attempt_retryable" || failure.OutcomeUnknown() {
			t.Fatalf("expected the native lock timeout after positive balance barrier: %v", got.err)
		}
	case <-ctx.Done():
		t.Fatal("executor exceeded its existing transaction/lock budget")
	}
	if err = holder.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	oaCompositeNoEffects(t, e, a)
	oaExecutorNoEffects(t, e, a)
	got, err := executor.Execute(ctx, "", "", a.AttemptID, a.Generation)
	if err != nil {
		t.Fatal("same attempt did not recover after transient lock release", err)
	}
	oaExecutorCommitted(t, e, a, plan, got, 5)
}

func TestOrderAttemptExecutorPlanRotationAndMissingOriginalKey(t *testing.T) {
	for _, retain := range []bool{true, false} {
		name := "original-retained"
		if !retain {
			name = "original-unavailable"
		}
		t.Run(name, func(t *testing.T) {
			e := lbuNew(t)
			a := oaFenceAdmit(t, e)
			plan, _ := oaMaterialPlan(t, e, oaServiceKeys(t), a.AttemptID)
			ring := map[string][]byte{"fixture_v2": bytes.Repeat([]byte{19}, 32)}
			if retain {
				ring["fixture_v1"] = bytes.Repeat([]byte{7}, 32)
			}
			rotated, err := orderattempt.NewPlanKeys("fixture_v2", ring)
			if err != nil {
				t.Fatal(err)
			}
			executor := oaExecutorNew(t, miPool(t, e.p.f, "commerce_claims_worker"), rotated)
			got, err := executor.Execute(context.Background(), "", "", a.AttemptID, a.Generation)
			if retain {
				if err != nil {
					t.Fatal("native continuation after retained plan-ring rotation", err)
				}
				oaExecutorCommitted(t, e, a, plan, got, 5)
			} else {
				var retryable *orderattempt.AdmissionError
				if !errors.As(err, &retryable) || retryable.Code() != "order_attempt_retryable" || retryable.OutcomeUnknown() || got.State != "" || len(got.Receipt) != 0 {
					t.Fatal("a process-local missing key cannot settle the admitted attempt", err)
				}
				oaExecutorNoEffects(t, e, a)
				got, err = oaExecutorNew(t, miPool(t, e.p.f, "commerce_claims_worker"), oaServiceKeys(t)).Execute(context.Background(), "", "", a.AttemptID, a.Generation)
				if err != nil {
					t.Fatal("the process retaining the key must still complete this attempt", err)
				}
				oaExecutorCommitted(t, e, a, plan, got, 5)
				again, err := executor.Execute(context.Background(), "", "", a.AttemptID, a.Generation)
				if err != nil {
					t.Fatal("terminal committed receipt must replay without a local key", err)
				}
				oaExecutorSameReceipt(t, got, again)
			}
		})
	}
}

func TestOrderAttemptExecutorWorkerIgnoresEarlierLocksAndPlacementSession(t *testing.T) {
	e := lbuNew(t)
	a := oaFenceAdmit(t, e)
	plan, _ := oaMaterialPlan(t, e, oaServiceKeys(t), a.AttemptID)
	// Admission remains authoritative after the original placement session and
	// membership permissions are gone. These fixture changes commit before C2.
	mustExec(t, e.p.f.owner, `UPDATE identity.sessions SET revoked_at=clock_timestamp() WHERE token_hash=$1`, tokenHash(e.token()))
	mustExec(t, e.p.f.owner, `UPDATE identity.memberships SET active=false,authz_revision=authz_revision+1 WHERE tenant_id=$1 AND principal_id=$2`, e.tenant(), e.p.f.principalA)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	holder, err := e.p.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	oaAbortCleanupTx(t, holder)
	defer holder.Rollback(context.Background())
	oaStateExec(t, holder, `SELECT scope_id FROM ops.order_attempt_scopes WHERE scope_id=$1 FOR UPDATE`, a.ScopeID)
	oaStateExec(t, holder, `SELECT id FROM identity.principals WHERE id=$1 FOR UPDATE`, e.p.f.principalA)
	oaStateExec(t, holder, `SELECT pg_advisory_xact_lock(hashtextextended('order-attempt-subject:'||tenant_id::text||':'||store_id::text||':'||subject_kind||':'||subject_id::text,0))
	 FROM ops.order_attempt_subjects WHERE attempt_id=$1 ORDER BY subject_kind,subject_id`, a.AttemptID)
	executor := oaExecutorNew(t, miPool(t, e.p.f, "commerce_claims_worker"), oaServiceKeys(t))
	got, err := executor.Execute(ctx, "", "", a.AttemptID, a.Generation)
	if err != nil {
		t.Fatal("durable worker continuation reacquired a placement or earlier-level gate", err)
	}
	oaExecutorCommitted(t, e, a, plan, got, 5)
}
