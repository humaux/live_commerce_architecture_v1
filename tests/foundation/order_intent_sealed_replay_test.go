// Purpose: REAL_PG sealed-intent recovery replays exact closure/recovery receipts through the merchant HTTP router.
// Depends on: lbuNew, mnrBody/mnrRecover, lcrRecover, durable ops.order_intents/command_results and owner-only corruption fixtures.
// Used by: foundation CI and focused TestOrderIntentSealedRecovery acceptance; no provider or local test execution.
// Invariants: I01/I02/I03/I06; sealed state alone never proves absence, original owner/key receipts are mandatory, replay changes no money/DM.
package foundation_test

import (
	"context"
	"testing"
	"time"
)

func oisCorruptProof(t *testing.T, e *lbuEnv, query string, args ...any) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tag, err := e.p.f.owner.Exec(ctx, query, args...)
	if err != nil || tag.RowsAffected() != 1 {
		t.Fatal("owner corruption fixture did not alter exactly one authoritative receipt")
	}
}

func TestOrderIntentSealedRecoveryReplay(t *testing.T) {
	for _, kind := range []string{"manual", "for_buyer"} {
		t.Run(kind, func(t *testing.T) {
			e := lbuNew(t)
			key := t04Key("ois-replay")
			body := mnrBody(e, 1)
			recover := mnrRecover
			if kind == "for_buyer" {
				body = e.body(1, nil, "", false)
				recover = lcrRecover
			}
			for replay := 0; replay < 3; replay++ {
				status, out := recover(t, e, e.token(), key, body)
				if status != 200 || out["state"] != "not_created" || len(out) != 1 {
					t.Fatalf("sealed %s recovery %d: status=%d state=%v code=%v fields=%d", kind, replay, status, out["state"], out["code"], len(out))
				}
				if e.count(`SELECT count(*) FROM ops.order_intents WHERE tenant_id=$1 AND store_id=$2 AND idempotency_key=$3 AND state='sealed' AND order_id IS NULL AND frozen_request IS NULL`, e.tenant(), e.store(), key) != 1 {
					t.Fatal("recovery did not retain the exact settled private intent")
				}
			}
			if mnrDefinitiveCount(e) != 2 || e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='order.attempt_recovered'`, e.store()) != 1 {
				t.Fatal("sealed replay recreated closure/recovery/audit")
			}
			if e.count(`SELECT count(*) FROM checkout.orders WHERE store_id=$1`, e.store()) != 0 || e.reserved() != 0 || e.plan.planned() != 0 {
				t.Fatal("sealed replay changed money/stock/DM")
			}
		})
	}
}

func TestOrderIntentSealedRecoveryRequiresReceipts(t *testing.T) {
	e := lbuNew(t)
	for _, fault := range []string{"missing_closure", "missing_recovery", "wrong_closure_owner", "wrong_recovery_state"} {
		t.Run(fault, func(t *testing.T) {
			key, body := t04Key("ois-proof"), mnrBody(e, 1)
			status, out := mnrRecover(t, e, e.token(), key, body)
			if status != 200 || out["state"] != "not_created" || len(out) != 1 {
				t.Fatalf("seal fixture: status=%d state=%v code=%v", status, out["state"], out["code"])
			}
			// Disclosed owner-only corruption touches this exact fixture after the real route committed both authoritative receipts.
			wantStatus, wantCode := 503, "retry_later"
			switch fault {
			case "missing_closure":
				oisCorruptProof(t, e, `DELETE FROM ops.command_results c USING ops.order_intents i WHERE c.tenant_id=i.tenant_id AND c.store_id=i.store_id AND c.operation='merchanttools.order.checkout.closed' AND c.idempotency_key=i.begin_key AND i.tenant_id=$1 AND i.store_id=$2 AND i.idempotency_key=$3`, e.tenant(), e.store(), key)
			case "missing_recovery":
				oisCorruptProof(t, e, `DELETE FROM ops.command_results WHERE tenant_id=$1 AND store_id=$2 AND operation='merchanttools.order.attempt.recovery' AND idempotency_key=$3`, e.tenant(), e.store(), key)
			case "wrong_closure_owner":
				oisCorruptProof(t, e, `UPDATE ops.command_results c SET response=jsonb_build_object('owner_id','00000000-0000-4000-8000-000000000001') FROM ops.order_intents i WHERE c.tenant_id=i.tenant_id AND c.store_id=i.store_id AND c.operation='merchanttools.order.checkout.closed' AND c.idempotency_key=i.begin_key AND i.tenant_id=$1 AND i.store_id=$2 AND i.idempotency_key=$3`, e.tenant(), e.store(), key)
				wantStatus, wantCode = 409, "idempotency_conflict"
			case "wrong_recovery_state":
				oisCorruptProof(t, e, `UPDATE ops.command_results SET response='{"state":"created","order_id":"00000000-0000-4000-8000-000000000001"}'::jsonb WHERE tenant_id=$1 AND store_id=$2 AND operation='merchanttools.order.attempt.recovery' AND idempotency_key=$3`, e.tenant(), e.store(), key)
				wantStatus, wantCode = 409, "idempotency_conflict"
			}
			before := e.count(`SELECT count(*) FROM ops.command_results WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store())
			status, out = mnrRecover(t, e, e.token(), key, body)
			if status != wantStatus || out["code"] != wantCode || out["state"] != nil || out["order_id"] != nil {
				t.Fatalf("%s bypassed sealed proof: status=%d code=%v state=%v", fault, status, out["code"], out["state"])
			}
			if e.count(`SELECT count(*) FROM ops.command_results WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store()) != before {
				t.Fatal("recovery fabricated a missing or corrected authoritative receipt")
			}
			if e.count(`SELECT count(*) FROM checkout.orders WHERE store_id=$1`, e.store()) != 0 || e.reserved() != 0 || e.plan.planned() != 0 {
				t.Fatal("failed sealed proof changed money/stock/DM")
			}
		})
	}
}
