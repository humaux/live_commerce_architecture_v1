// Purpose: REAL_PG acceptance of complete C2 settlement through the public composite.
// Depends on: real C1, runtime/worker fence, native prepare/Begin, encrypted pay entry.
// Used by: focused OA1 CI; finish/pay stages may be absent for the tests-first RED.
// Invariants: one physical transaction, durable immutable redacted receipt, no partial
// commit, no late scope/principal/subject lock, and exact replay after response loss.
package foundation_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/checkout"
	"livecommerce/internal/orderattempt"
)

type oaSettlementReceipt struct {
	State string `json:"state"`
	AttemptID string `json:"attempt_id"`
	Generation int64 `json:"intent_generation"`
	OrderID string `json:"order_id"`
	CommercialState string `json:"commercial_state"`
	PaymentMode string `json:"payment_mode"`
	TotalMinor int64 `json:"total_minor"`
	Currency string `json:"currency"`
	Send struct {
		State string `json:"state"`
		Reason string `json:"reason,omitempty"`
	} `json:"send"`
}

func oaSettlementBegin(t *testing.T, e *lbuEnv, a orderattempt.AdmissionReceipt, worker bool) (pgx.Tx, []byte) {
	t.Helper()
	tx, plain := oaCompositeTx(t, e, a, worker)
	oaAbortCleanupTx(t, tx)
	prepared, lines := oaCompositePrepare(t, tx, a, plain)
	raw, err := oaCompositeCall(tx, a, plain, "begin", oaCompositeBeginInput(prepared, lines))
	var got checkout.Result
	if err != nil || json.Unmarshal(raw, &got) != nil || got.OrderID != a.AttemptID || got.JobID <= 0 {
		t.Fatalf("real composite Begin prerequisite: %v", err)
	}
	return tx, plain
}

func oaSettlementPay(t *testing.T, e *lbuEnv, tx pgx.Tx, a orderattempt.AdmissionReceipt, plain []byte) json.RawMessage {
	t.Helper()
	material, err := payEntryTestKeys(t).Issue(orderattempt.PayEntryBinding{TenantID:e.tenant(), StoreID:e.store(), OrderID:a.AttemptID, Version:1})
	if err != nil { t.Fatal("synthetic dedicated pay-entry key prerequisite") }
	raw, err := oaCompositeCall(tx, a, plain, "pay", map[string]any{
		"key_version":material.KeyVersion, "token_hash_hex":hex.EncodeToString(material.Digest[:]),
		"credential_ciphertext_hex":hex.EncodeToString(material.Ciphertext),
	})
	var got struct {
		Stage string `json:"stage"`
		Entry oaPayEntryStoreWire `json:"pay_entry"`
		Notification json.RawMessage `json:"notification"`
	}
	if err != nil || json.Unmarshal(raw, &got) != nil || got.Stage != "PAY_PREPARED" || got.Entry.OrderID != a.AttemptID || got.Entry.LinkVersion != 1 || got.Entry.Source != "merchant_manual" || got.Entry.ExpiresAt.IsZero() {
		t.Fatalf("real composite pay prerequisite: %v", err)
	}
	var notification map[string]any
	if json.Unmarshal(got.Notification, &notification) != nil || len(notification) != 1 || notification["state"] != "NOT_REQUESTED" {
		t.Fatal("frozen no-send plan must produce only NOT_REQUESTED notification artifact")
	}
	return got.Notification
}

func oaSettlementCheckReceipt(t *testing.T, raw []byte, a orderattempt.AdmissionReceipt, plain []byte) json.RawMessage {
	t.Helper()
	var wire struct { State string `json:"state"`; Receipt json.RawMessage `json:"receipt"` }
	var outer, receipt, send map[string]json.RawMessage
	var got oaSettlementReceipt
	var plan orderattempt.FrozenPlan
	if json.Unmarshal(raw, &wire) != nil || json.Unmarshal(raw, &outer) != nil || len(outer) != 2 || wire.State != "COMMITTED" {
		t.Fatal("finish must return only state and committed receipt")
	}
	if json.Unmarshal(wire.Receipt, &got) != nil || json.Unmarshal(wire.Receipt, &receipt) != nil || json.Unmarshal(receipt["send"], &send) != nil || json.Unmarshal(plain, &plan) != nil {
		t.Fatal("invalid finish receipt")
	}
	if len(receipt) != 9 || got.State != "COMMITTED" || got.AttemptID != a.AttemptID || got.Generation != a.Generation || got.OrderID != a.AttemptID || got.CommercialState != "AWAITING_TRANSFER" || got.PaymentMode != "bank_transfer" || got.TotalMinor != plan.Calculation.TotalMinor || got.Currency != "TWD" || got.Send.State != "not_sent" {
		t.Fatal("finish receipt lost exact admitted/native order facts or included extra private fields")
	}
	// A no-send receipt may omit reason, or use the single fixed not_requested code.
	// Strict field counts exclude bearer, ciphertext, contact and operation identity.
	if len(send) != 1 && !(len(send) == 2 && got.Send.Reason == "not_requested") {
		t.Fatal("no-send receipt must not invent an operation, private field or freeform reason")
	}
	return wire.Receipt
}

func oaSettlementZero(t *testing.T, e *lbuEnv, a orderattempt.AdmissionReceipt, auditBefore int) {
	t.Helper()
	oaCompositeNoEffects(t, e, a)
	oaBeginZeroEffects(t, e, a.AttemptID)
	if n := e.count(`SELECT count(*) FROM checkout.order_pay_entries WHERE create_attempt_id=$1`, a.AttemptID); n != 0 { t.Fatal("pay entry survived unfinished C2 rollback") }
	if n := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='order.manual_created'`, e.store()); n != auditBefore { t.Fatal("manual-created audit survived unfinished C2 rollback") }
	if n := e.count(`SELECT count(*) FROM ops.order_attempt_subjects WHERE attempt_id=$1`, a.AttemptID); n == 0 { t.Fatal("pending subject index was shredded despite rollback") }
}

func TestOrderAttemptSettlementDurableReceiptAndLostAckReplay(t *testing.T) {
	for _, worker := range []bool{false, true} {
		name := "runtime"
		if worker { name = "claims-worker" }
		t.Run(name, func(t *testing.T) {
			e := lbuNew(t)
			req := oaServiceRequest(t, e)
			svc, err := orderattempt.NewService(e.p.f.runtime, oaServiceKeys(t))
			if err != nil { t.Fatal(err) }
			a, err := svc.Admit(context.Background(), e.token(), e.store(), req)
			if err != nil || a.State != "PENDING" { t.Fatalf("actual C1 admission sentinel: %v", err) }
			if n := e.count(`SELECT count(*) FROM ops.order_attempt_subjects WHERE attempt_id=$1`, a.AttemptID); n == 0 { t.Fatal("real C1 subject-index sentinel") }
			auditBefore := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='order.manual_created'`, e.store())
			operationsBefore := e.count(`SELECT count(*) FROM integration.operations WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store())
			tx, plain := oaSettlementBegin(t, e, a, worker)
			notification := oaSettlementPay(t, e, tx, a, plain)
			oaStateExec(t, tx, `SELECT set_config('app.tenant_id','caller-marker',true),set_config('app.store_id','caller-marker',true),set_config('app.principal_id','caller-marker',true),set_config('app.buyer_id','caller-marker',true),set_config('app.buyer_session_id','caller-marker',true),set_config('app.order_attempt_id','caller-marker',true)`)
			raw, err := oaCompositeCall(tx, a, plain, "finish", notification)
			if err != nil { t.Fatalf("public finish after complete real local prerequisites: %v", err) }
			receipt := oaSettlementCheckReceipt(t, raw, a, plain)
			var restored bool
			err = tx.QueryRow(context.Background(), `SELECT current_setting('app.tenant_id')='caller-marker' AND current_setting('app.store_id')='caller-marker' AND current_setting('app.principal_id')='caller-marker' AND current_setting('app.buyer_id')='caller-marker' AND current_setting('app.buyer_session_id')='caller-marker' AND current_setting('app.order_attempt_id')='caller-marker'`).Scan(&restored)
			if err != nil || !restored { t.Fatalf("finish leaked execution context: %v", err) }
			if err = tx.Commit(context.Background()); err != nil { t.Fatalf("full C2 must pass native deferred commit guard: %v", err) }
			var exact bool
			err = e.p.f.owner.QueryRow(context.Background(), `SELECT a.state='COMMITTED' AND a.order_id=$1::uuid AND a.receipt=$2::jsonb
			 AND a.plan_ciphertext IS NULL AND a.plan_key_version IS NULL AND a.settled_at>=a.admitted_at
			 AND NOT EXISTS(SELECT 1 FROM ops.order_attempt_subjects s WHERE s.attempt_id=a.attempt_id)
			 AND EXISTS(SELECT 1 FROM checkout.orders o JOIN checkout.order_pay_entries p ON p.order_id=o.id
			  WHERE o.id=a.order_id AND o.create_attempt_id=a.attempt_id AND o.creator_kind='order_attempt'
			   AND o.creator_session_id IS NULL AND o.source='merchant_manual' AND o.commercial_state='AWAITING_TRANSFER'
			   AND p.create_attempt_id=a.attempt_id AND p.link_version=1 AND p.revoked_at IS NULL)
			 AND EXISTS(SELECT 1 FROM inventory.reservations r WHERE r.id=a.order_id AND r.create_attempt_id=a.attempt_id AND r.state='HELD')
			 FROM ops.order_attempts a WHERE a.attempt_id=$1`, a.AttemptID, string(receipt)).Scan(&exact)
			if err != nil || !exact { t.Fatalf("committed native effects/receipt/cryptoshred/subject cleanup: %v", err) }
			if n := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='order.manual_created' AND principal_id=$2`, e.store(), e.p.f.principalA); n != auditBefore+1 { t.Fatal("one native manual-created audit required") }
			if n := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='order.manual_created' AND principal_id=$2 AND details=jsonb_build_object('attempt_id',$3::uuid,'order_id',$3::uuid,'intent_generation',$4::bigint)`, e.store(), e.p.f.principalA, a.AttemptID, a.Generation); n != 1 { t.Fatal("audit must contain only exact attempt/order/generation identity") }
			if n := e.count(`SELECT count(*) FROM integration.operations WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store()); n != operationsBefore { t.Fatal("NOT_REQUESTED created a provider operation") }
			// Discarding the first response models response loss after durable commit.
			// Recovery uses the original attempt/generation; it must not re-run stages.
			pool := e.p.f.runtime
			hash := tokenHash(e.token())
			if worker { pool = miPool(t, e.p.f, "commerce_claims_worker"); hash = nil }
			replayTx, err := pool.Begin(context.Background())
			if err != nil { t.Fatal(err) }
			oaAbortCleanupTx(t, replayTx)
			replay, err := oaFenceCall(context.Background(), replayTx, hash, e.store(), a.AttemptID, a.Generation)
			if err != nil || !bytes.Equal(raw, replay) { t.Fatalf("lost-ack terminal replay changed exact durable receipt: %v", err) }
			if err = replayTx.Commit(context.Background()); err != nil { t.Fatal(err) }
			admissionReplay, err := svc.Admit(context.Background(), e.token(), e.store(), req)
			if err != nil || !admissionReplay.Replay || admissionReplay.State != "COMMITTED" || admissionReplay.AttemptID != a.AttemptID || !bytes.Equal(admissionReplay.Receipt, receipt) { t.Fatalf("same transport key must recover the same terminal receipt: %v", err) }
			if n := e.count(`SELECT count(*) FROM checkout.orders WHERE create_attempt_id=$1`, a.AttemptID); n != 1 { t.Fatal("terminal replay duplicated native order") }
			if n := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='order.manual_created'`, e.store()); n != auditBefore+1 { t.Fatal("terminal replay duplicated audit") }
		})
	}
}

func TestOrderAttemptSettlementRollbackAtBothSidesOfFinish(t *testing.T) {
	for _, afterFinish := range []bool{false, true} {
		t.Run(map[bool]string{false:"before-finish", true:"late-failure-after-finish"}[afterFinish], func(t *testing.T) {
			e := lbuNew(t)
			a := oaFenceAdmit(t, e)
			auditBefore := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='order.manual_created'`, e.store())
			tx, plain := oaSettlementBegin(t, e, a, false)
			notification := oaSettlementPay(t, e, tx, a, plain)
			if afterFinish {
				raw, err := oaCompositeCall(tx, a, plain, "finish", notification)
				if err != nil { t.Fatalf("finish before injected late failure: %v", err) }
				oaSettlementCheckReceipt(t, raw, a, plain)
				_, err = tx.Exec(context.Background(), `SELECT 1/0`)
				oaFenceCode(t, nil, err, "22012")
				if err = tx.Commit(context.Background()); err != pgx.ErrTxCommitRollback { t.Fatalf("failed outer Tx must roll back the returned finish result: %v", err) }
			} else {
				oaFenceCode(t, nil, tx.Commit(context.Background()), "23514")
			}
			oaSettlementZero(t, e, a, auditBefore)
		})
	}
}

func TestOrderAttemptSettlementRejectsPartialAndForgedArtifact(t *testing.T) {
	for _, mode := range []string{"no-effects", "no-pay-entry", "unknown-state", "extra-order", "forged-operation", "null-artifact"} {
		t.Run(mode, func(t *testing.T) {
			e := lbuNew(t)
			a := oaFenceAdmit(t, e)
			auditBefore := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='order.manual_created'`, e.store())
			var tx pgx.Tx
			var plain []byte
			var input any = map[string]any{"state":"NOT_REQUESTED"}
			code := "PT400"
			if mode == "no-effects" { tx, plain = oaCompositeTx(t, e, a, false); code = "PT409" } else {
				tx, plain = oaSettlementBegin(t, e, a, false)
				if mode == "no-pay-entry" { code = "PT409" } else { input = oaSettlementPay(t, e, tx, a, plain) }
			}
			oaAbortCleanupTx(t, tx)
			switch mode {
			case "unknown-state": input = map[string]any{"state":"SENT"}
			case "extra-order": input = map[string]any{"state":"NOT_REQUESTED", "order_id":randomUUID()}
			case "forged-operation": input = map[string]any{"state":"NOT_REQUESTED", "operation_id":randomUUID()}
			case "null-artifact": input = nil
			}
			raw, err := oaCompositeCall(tx, a, plain, "finish", input)
			oaFenceCode(t, raw, err, code)
			_ = tx.Rollback(context.Background())
			oaSettlementZero(t, e, a, auditBefore)
		})
	}
}

func TestOrderAttemptSettlementWorkerNoLateAuthorityLocks(t *testing.T) {
	e := lbuNew(t)
	a := oaFenceAdmit(t, e)
	holder, err := e.p.f.owner.Begin(context.Background())
	if err != nil { t.Fatal(err) }
	oaAbortCleanupTx(t, holder)
	oaStateExec(t, holder, `SELECT scope_id FROM ops.order_attempt_scopes WHERE scope_id=$1 FOR UPDATE`, a.ScopeID)
	oaStateExec(t, holder, `SELECT id FROM identity.principals WHERE id=$1 FOR UPDATE`, e.p.f.principalA)
	oaStateExec(t, holder, `SELECT pg_advisory_xact_lock(hashtextextended('order-attempt-subject:'||tenant_id::text||':'||store_id::text||':'||subject_kind||':'||subject_id::text,0)) FROM ops.order_attempt_subjects WHERE attempt_id=$1 ORDER BY subject_kind,subject_id`, a.AttemptID)
	// The worker has no merchant-session reauthorization. Preserve the existing
	// 750ms lock bound through finish AND COMMIT to catch implicit FK regressions.
	worker := miPool(t, e.p.f, "commerce_claims_worker")
	tx, err := worker.Begin(context.Background())
	if err != nil { t.Fatal(err) }
	oaAbortCleanupTx(t, tx)
	oaStateExec(t, tx, `SET LOCAL lock_timeout='750ms'`)
	oaStateExec(t, tx, `SET LOCAL statement_timeout='5s'`)
	var actual bool
	if err = tx.QueryRow(context.Background(), `SELECT session_user=current_user AND pg_has_role(session_user,'commerce_claims_worker','MEMBER')`).Scan(&actual); err != nil || !actual { t.Fatal("requires actual worker session") }
	fenced, err := oaFenceCall(context.Background(), tx, nil, e.store(), a.AttemptID, a.Generation)
	if err != nil { t.Fatalf("worker fence under held earlier barriers: %v", err) }
	wire := oaFenceOpen(t, fenced, a)
	plan, err := oaServiceKeys(t).Open(*wire.Binding, *wire.Envelope)
	if err != nil { t.Fatal(err) }
	plain, err := json.Marshal(plan)
	if err != nil { t.Fatal(err) }
	prepared, lines := oaCompositePrepare(t, tx, a, plain)
	if _, err = oaCompositeCall(tx, a, plain, "begin", oaCompositeBeginInput(prepared, lines)); err != nil { t.Fatalf("Begin under held earlier barriers: %v", err) }
	notification := oaSettlementPay(t, e, tx, a, plain)
	raw, err := oaCompositeCall(tx, a, plain, "finish", notification)
	if err != nil { t.Fatalf("finish must not reacquire scope/principal/subject authority: %v", err) }
	oaSettlementCheckReceipt(t, raw, a, plain)
	if err = tx.Commit(context.Background()); err != nil { t.Fatalf("settlement COMMIT acquired reverse FK locks: %v", err) }
	if err = holder.Rollback(context.Background()); err != nil { t.Fatal(err) }
}
