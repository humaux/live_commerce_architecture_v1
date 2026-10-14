// Purpose: REAL_PG tests-first OA1 cancellation and pending-erasure compatibility.
// Depends on: actual C1/Go Executor, native customer/cancel APIs and protected PG rows.
// Used by: parent-owned CI; no dispatcher, provider call or owner-injected terminal state.
// Invariants: retained COMMITTED identity, exactly-once stock release, erasure attempt
// prefix before owners, ERASED settlement with body shredding and explicit generation CAS.
package foundation_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"livecommerce/internal/customers"
	"livecommerce/internal/fulfillment"
	"livecommerce/internal/orderattempt"
	"livecommerce/internal/platform"
)

func oaCECancel(ctx context.Context, e *lbuEnv, a orderattempt.AdmissionReceipt, key, state string) (fulfillment.CancelResult, error) {
	var out fulfillment.CancelResult
	err := platform.WithScope(ctx, e.p.f.runtime, e.token(), e.store(), "store:read", func(tx pgx.Tx, s platform.Scope) error {
		var err error
		out, err = fulfillment.CancelOrder(ctx, tx, s, e.token(), key, a.AttemptID, state, "synthetic cancellation")
		return err
	})
	return out, err
}

func oaCEErase(ctx context.Context, e *lbuEnv, owner, key string, pid chan<- uint32) error {
	return platform.WithScope(ctx, e.p.f.runtime, e.token(), e.store(), "store:read", func(tx pgx.Tx, s platform.Scope) error {
		if pid != nil {
			pid <- tx.Conn().PgConn().PID()
		}
		_, err := customers.Erase(ctx, tx, s, e.token(), key, owner)
		return err
	})
}

func oaCEAttemptBytes(t *testing.T, e *lbuEnv, a orderattempt.AdmissionReceipt) []byte {
	t.Helper()
	var raw []byte
	if err := e.p.f.owner.QueryRow(context.Background(), `SELECT to_jsonb(a) FROM ops.order_attempts a WHERE attempt_id=$1`, a.AttemptID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestOrderAttemptCancelNativeCommittedRecovery(t *testing.T) {
	for _, mode := range []string{"pay_at_pickup", "cash_on_delivery", "bank_transfer"} {
		t.Run(mode, func(t *testing.T) {
			e := lbuNew(t)
			e.grantCreator("fulfillment:write")
			a := oaAllocationAdmit(t, e, oaBeginSelection(t, e, mode, "duplicate-origins"))
			plan, _ := oaMaterialPlan(t, e, oaServiceKeys(t), a.AttemptID)
			executor := oaExecutorNew(t, e.p.f.runtime, oaServiceKeys(t))
			first, err := executor.Execute(context.Background(), e.token(), e.store(), a.AttemptID, a.Generation)
			if err != nil {
				t.Fatalf("actual full C2 before cancel: %v", err)
			}
			oaExecutorReceipt(t, first, a, "COMMITTED", "")
			state := "CONFIRMED"
			if mode == "cash_on_delivery" {
				state = "AWAITING_COLLECTION"
			} else if mode == "bank_transfer" {
				state = "AWAITING_TRANSFER"
			}
			var exact bool
			err = e.p.f.owner.QueryRow(context.Background(), `SELECT a.state='COMMITTED' AND a.order_id=a.attempt_id
			 AND a.receipt=$2::jsonb AND o.owner_id=$3 AND o.creator_kind='order_attempt'
			 AND o.creator_session_id IS NULL AND o.create_attempt_id=a.attempt_id AND o.payment_mode=$4
			 AND o.commercial_state=$5 AND o.total_minor=$6 AND r.create_attempt_id=a.attempt_id
			 AND r.state=CASE WHEN $4='bank_transfer' THEN 'HELD' ELSE 'COMMITTED' END
			 FROM ops.order_attempts a JOIN checkout.orders o ON o.id=a.order_id
			 JOIN inventory.reservations r ON r.id=o.id WHERE a.attempt_id=$1`,
				a.AttemptID, string(first.Receipt), plan.BuyerOwnerID, mode, state, plan.Calculation.TotalMinor).Scan(&exact)
			if err != nil || !exact {
				t.Fatalf("positive native order/stock/receipt prerequisite: exact=%v err=%v", exact, err)
			}
			before := oaCEAttemptBytes(t, e, a)
			key := "oa-cancel-" + randomUUID()
			cancelled, err := oaCECancel(context.Background(), e, a, key, state)
			if err != nil || cancelled.OrderID != a.AttemptID || cancelled.CommercialState != "CANCELLED" || cancelled.ReleasedLines < 1 {
				t.Fatalf("native cancel of committed %s: %v", mode, err)
			}
			cancelReplay, err := oaCECancel(context.Background(), e, a, key, state)
			if err != nil || cancelReplay.OrderID != cancelled.OrderID || cancelReplay.ReleasedLines != cancelled.ReleasedLines {
				t.Fatalf("same native cancellation must replay: %v", err)
			}
			again, err := executor.Execute(context.Background(), e.token(), e.store(), a.AttemptID, a.Generation)
			if err != nil {
				t.Fatalf("COMMITTED recovery after native CANCELLED: %v", err)
			}
			oaExecutorSameReceipt(t, first, again)
			if !bytes.Equal(before, oaCEAttemptBytes(t, e, a)) {
				t.Fatal("cancel/recovery mutated the retained terminal attempt or receipt")
			}
			err = e.p.f.owner.QueryRow(context.Background(), `SELECT
			 o.commercial_state='CANCELLED' AND o.id=a.order_id AND r.state='RELEASED'
			 AND (SELECT count(*) FROM checkout.orders WHERE create_attempt_id=$1)=1
			 AND (SELECT count(*) FROM checkout.order_pay_entries WHERE create_attempt_id=$1)=1
			 AND (SELECT min(link_version) FROM checkout.order_pay_entries WHERE create_attempt_id=$1)=1
			 AND (SELECT count(*) FROM river_expiry.river_job WHERE args->>'order_id'=($1::uuid)::text)=1
			 AND NOT EXISTS(SELECT 1 FROM ops.order_attempt_subjects WHERE attempt_id=$1)
			 AND NOT EXISTS(SELECT 1 FROM inbox.order_payment_notifications WHERE create_attempt_id=$1)
			 AND (SELECT coalesce(sum(delta_reserved),0) FROM inventory.ledger WHERE checkout_id=$1)=0
			 AND (SELECT coalesce(sum(delta_allocated),0) FROM inventory.ledger WHERE checkout_id=$1)=0
			 AND (SELECT count(*) FROM inventory.ledger WHERE checkout_id=$1 AND kind=$2)=1
			 AND NOT EXISTS(SELECT 1 FROM inventory.ledger WHERE checkout_id=$1
			 AND (creator_kind<>'order_attempt' OR create_attempt_id IS DISTINCT FROM $1::uuid
			 OR buyer_owner_id IS DISTINCT FROM a.buyer_owner_id OR buyer_session_id IS NOT NULL))
			 FROM ops.order_attempts a JOIN checkout.orders o ON o.id=a.order_id
			 JOIN inventory.reservations r ON r.id=o.id WHERE a.attempt_id=$1`,
				a.AttemptID, map[bool]string{true: "RELEASE", false: "DEALLOCATE"}[mode == "bank_transfer"]).Scan(&exact)
			if err != nil || !exact {
				t.Fatalf("cancel must release stock once without a renewed order/hold/entry/job: exact=%v err=%v", exact, err)
			}
		})
	}
}

func oaCEPending(t *testing.T, e *lbuEnv, linked bool) (orderattempt.AdmissionReceipt, orderattempt.FrozenPlan) {
	t.Helper()
	e.grantCreator("customers:privacy", "customers:read")
	selection := oaFactsSelection(e)
	if linked {
		owner, conversation := randomUUID(), randomUUID()
		// Fixture-only native existing customer/A14 link; no identity is inferred from PII.
		mustExec(t, e.p.f.owner, `INSERT INTO buyer.owners(id,tenant_id,store_id) VALUES($1,$2,$3)`, owner, e.tenant(), e.store())
		mustExec(t, e.p.f.owner, `INSERT INTO social.conversations(id,tenant_id,store_id,app_id,object,asset_id,peer_key)
		 VALUES($1,$2,$3,'123456789','page','123456789',repeat(replace($1::uuid::text,'-',''),2))`, conversation, e.tenant(), e.store())
		mustExec(t, e.p.f.owner, `INSERT INTO inbox.conversation_state(tenant_id,store_id,conversation_id,customer_id) VALUES($1,$2,$3,$4)`, e.tenant(), e.store(), conversation, owner)
		selection["for"] = map[string]any{"bundle_ids": []string{}, "conversation_id": conversation, "customer_id": owner}
	}
	a := oaAllocationAdmit(t, e, selection)
	plan, _ := oaMaterialPlan(t, e, oaServiceKeys(t), a.AttemptID)
	var sealed bool
	if err := e.p.f.owner.QueryRow(context.Background(), `SELECT state='PENDING' AND plan_ciphertext IS NOT NULL
	 AND plan_key_version IS NOT NULL AND EXISTS(SELECT 1 FROM ops.order_attempt_subjects s WHERE s.attempt_id=a.attempt_id)
	 FROM ops.order_attempts a WHERE attempt_id=$1`, a.AttemptID).Scan(&sealed); err != nil || !sealed {
		t.Fatalf("pending ciphertext and indexed subject prerequisite: sealed=%v err=%v", sealed, err)
	}
	return a, plan
}

func oaCEErased(t *testing.T, e *lbuEnv, a orderattempt.AdmissionReceipt, plan orderattempt.FrozenPlan) {
	t.Helper()
	var exact bool
	err := e.p.f.owner.QueryRow(context.Background(), `SELECT a.state='ABORTED' AND a.receipt->>'code'='ERASED'
	 AND a.receipt->>'attempt_id'=a.attempt_id::text AND (a.receipt->>'intent_generation')::bigint=a.intent_generation
	 AND a.plan_ciphertext IS NULL AND a.plan_key_version IS NULL AND a.order_id IS NULL
	 AND a.settled_at IS NOT NULL AND NOT o.active
	 AND NOT EXISTS(SELECT 1 FROM ops.order_attempt_subjects s WHERE s.attempt_id=a.attempt_id)
	 FROM ops.order_attempts a JOIN buyer.owners o ON o.id=a.buyer_owner_id WHERE a.attempt_id=$1`, a.AttemptID).Scan(&exact)
	if err != nil || !exact {
		t.Fatalf("erasure commit must already seal ABORTED ERASED and shred private body/index: exact=%v err=%v", exact, err)
	}
	oaExecutorNoEffects(t, e, a)
	executor := oaExecutorNew(t, e.p.f.runtime, oaServiceKeys(t))
	replay, err := executor.Execute(context.Background(), e.token(), e.store(), a.AttemptID, a.Generation)
	if err != nil {
		t.Fatalf("erased terminal recovery requires no private body: %v", err)
	}
	oaExecutorReceipt(t, replay, a, "ABORTED", "ERASED")
	req := oaMaterialRequest(t, e, oaFactsSelection(e))
	req.ExpectedScopeID, req.ExpectedGeneration, req.PreviousAttemptID = a.ScopeID, a.Generation, &a.AttemptID
	svc, err := orderattempt.NewService(e.p.f.runtime, oaServiceKeys(t))
	if err != nil {
		t.Fatal(err)
	}
	next, err := svc.Admit(context.Background(), e.token(), e.store(), req)
	if err != nil || next.State != "PENDING" || next.Generation != a.Generation+1 || next.AttemptID == a.AttemptID {
		t.Fatalf("explicit new manual generation after ERASED: state=%s generation=%d err=%v", next.State, next.Generation, err)
	}
	nextPlan, _ := oaMaterialPlan(t, e, oaServiceKeys(t), next.AttemptID)
	if nextPlan.BuyerOwnerID == plan.BuyerOwnerID {
		t.Fatal("new explicit intent reused the erased private/customer owner")
	}
}

func TestOrderAttemptErasureNativePendingSettlementAndExplicitCAS(t *testing.T) {
	for _, linked := range []bool{false, true} {
		name := "private-owner"
		if linked {
			name = "linked-customer"
		}
		t.Run(name, func(t *testing.T) {
			e := lbuNew(t)
			a, plan := oaCEPending(t, e, linked)
			if err := oaCEErase(context.Background(), e, plan.BuyerOwnerID, "oa-erase-"+randomUUID(), nil); err != nil {
				t.Fatalf("native authenticated pending owner erasure: %v", err)
			}
			oaCEErased(t, e, a, plan)
		})
	}
}

func TestOrderAttemptErasureLocksAttemptBeforeOwner(t *testing.T) {
	e := lbuNew(t)
	a, plan := oaCEPending(t, e, false)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	holder, err := e.p.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	oaAbortCleanupTx(t, holder)
	var holderPID uint32
	if err = holder.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&holderPID); err != nil {
		t.Fatal(err)
	}
	oaStateExec(t, holder, `SELECT 1 FROM buyer.owners WHERE id=$1 FOR NO KEY UPDATE`, plan.BuyerOwnerID)
	pid, done := make(chan uint32, 1), make(chan error, 1)
	go func() { done <- oaCEErase(ctx, e, plan.BuyerOwnerID, "oa-erase-prefix-"+randomUUID(), pid) }()
	var waiterPID uint32
	select {
	case waiterPID = <-pid:
	case <-ctx.Done():
		t.Fatal("erasure did not enter the authenticated native transaction")
	}
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	blocked := false
	for !blocked && ctx.Err() == nil {
		if err = e.p.f.owner.QueryRow(ctx, `SELECT $1::int=ANY(pg_blocking_pids($2::int))`, holderPID, waiterPID).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if !blocked {
			select {
			case <-ticker.C:
			case <-ctx.Done():
			}
		}
	}
	if !blocked {
		t.Fatal("deterministic native owner barrier was not observed")
	}
	probe, err := e.p.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	oaAbortCleanupTx(t, probe)
	_, probeErr := probe.Exec(ctx, `SELECT 1 FROM ops.order_attempts WHERE attempt_id=$1 FOR UPDATE NOWAIT`, a.AttemptID)
	_ = probe.Rollback(ctx)
	// Release the owner before reporting an assertion, so pool cleanup never waits on this barrier.
	if err = holder.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-done:
		if err != nil {
			t.Fatalf("erasure after releasing owner: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("erasure did not finish after releasing the owner")
	}
	var pgerr *pgconn.PgError
	if !errors.As(probeErr, &pgerr) || pgerr.Code != "55P03" {
		t.Fatalf("erasure must own attempt lock while waiting for owner, got %v", probeErr)
	}
	oaCEErased(t, e, a, plan)
}
