// Purpose: REAL_PG generic recovery survives a lost HTTP response and stays pinned to its original server intent.
// Depends on: lbuNew, real merchant routes, manual Begin kill / for-buyer FaultAfterPlace, immutable command/intent receipts.
// Used by: foundation CI TestOrderIntentGenericRecovery acceptance; owner requires CI-only execution.
// Invariants: I01/I02/I03/I06; settlement never turns an ambiguous retry into none, later intents cannot retarget a transport key, terminal proof stays authoritative.
package foundation_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"
)

// oigrDropResponse loses only the response body after the real handler's transactions complete.
// It never mocks an upstream result, skips the route, or rewrites a durable receipt.
type oigrDropResponse struct{ http.ResponseWriter }

func (w oigrDropResponse) Write(body []byte) (int, error) { return len(body), nil }

func oigrRecover(t *testing.T, e *lbuEnv, token, recoveryKey string) (int, map[string]any) {
	t.Helper()
	w := e.admin(token).call("POST", "/orders/attempt/recover", recoveryKey, "application/json", []byte(`{}`))
	var out map[string]any
	if json.Unmarshal(w.Body.Bytes(), &out) != nil {
		t.Fatal("generic recovery returned no JSON response")
	}
	return w.Code, out
}

func TestOrderIntentGenericRecoveryLostResponseReplaysOriginalTarget(t *testing.T) {
	for _, kind := range []string{"manual", "for_buyer"} {
		t.Run(kind, func(t *testing.T) {
			e := lbuNew(t)
			beforeOrders, beforeHeld, beforeManual := e.count(`SELECT count(*) FROM checkout.orders WHERE store_id=$1`, e.store()), e.reserved(), e.orders()
			key, body := t04Key("oigr-first"), mnrBody(e, 2)
			if kind == "manual" {
				mnrPartialKill(t, e, key, body)
			} else {
				body = e.body(2, nil, "", false)
				e.fb.FaultAfterPlace = func() error { return errors.New("injected death before upper receipt") }
				status, out := e.post(e.token(), key, body)
				e.fb.FaultAfterPlace = nil
				if status != 503 || out["code"] != "retry_later" {
					t.Fatalf("real partial fixture: status=%d code=%v", status, out["code"])
				}
			}
			if e.count(`SELECT count(*) FROM ops.order_intents WHERE tenant_id=$1 AND store_id=$2 AND idempotency_key=$3 AND state='pending'`, e.tenant(), e.store(), key) != 1 {
				t.Fatal("lost-response fixture has no actual pending intent")
			}
			recoveryKey := t04Key("oigr-transport")
			actual := e.handler
			e.handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { actual.ServeHTTP(oigrDropResponse{w}, r) })
			lost := e.admin(e.token()).call("POST", "/orders/attempt/recover", recoveryKey, "application/json", []byte(`{}`))
			e.handler = actual
			if lost.Code != 200 || lost.Body.Len() != 0 {
				t.Fatalf("first real recovery was not committed with its response lost: status=%d bytes=%d", lost.Code, lost.Body.Len())
			}
			var original string
			oiaRead(t, e, `SELECT order_id::text FROM ops.order_intents WHERE tenant_id=$1 AND store_id=$2 AND idempotency_key=$3 AND state='created' AND frozen_request IS NULL`, []any{e.tenant(), e.store(), key}, &original)
			assertOriginal := func(transport string) {
				status, out := oigrRecover(t, e, e.token(), transport)
				if status != 200 || out["state"] != "created" || out["order_id"] != original || len(out) != 2 {
					t.Fatalf("lost recovery retargeted or cleared authority: status=%d state=%v order=%v code=%v fields=%d", status, out["state"], out["order_id"], out["code"], len(out))
				}
			}
			for replay := 0; replay < 3; replay++ {
				assertOriginal(recoveryKey)
			}
			// A new browser may have lost its local transport key too; latest terminal state still proves the exact existing order.
			assertOriginal(t04Key("oigr-new-browser"))
			if e.count(`SELECT count(*) FROM checkout.orders WHERE store_id=$1`, e.store()) != beforeOrders+1 || e.reserved() != beforeHeld+2 || e.orders() != beforeManual+1 || e.plan.planned() != 0 || e.count(`SELECT count(*) FROM checkout.order_links WHERE store_id=$1 AND order_id=$2`, e.store(), original) != 1 {
				t.Fatal("recovery replay duplicated checkout, stock, source, link or DM")
			}
			if e.count(`SELECT count(*) FROM ops.command_results WHERE tenant_id=$1 AND store_id=$2 AND operation='merchanttools.order.intent.recovery.target' AND idempotency_key=$3`, e.tenant(), e.store(), recoveryKey) != 1 {
				t.Fatal("the transport key has no exact durable target binding")
			}
			// A later pending request must not steal an old ambiguous transport key.
			laterKey := t04Key("oigr-later")
			mnrPartialKill(t, e, laterKey, mnrBody(e, 1))
			assertOriginal(recoveryKey)
			if e.count(`SELECT count(*) FROM ops.order_intents WHERE tenant_id=$1 AND store_id=$2 AND idempotency_key=$3 AND state='pending'`, e.tenant(), e.store(), laterKey) != 1 || e.count(`SELECT count(*) FROM checkout.orders WHERE store_id=$1`, e.store()) != beforeOrders+2 || e.reserved() != beforeHeld+3 || e.orders() != beforeManual+1 {
				t.Fatal("original transport replay resumed or sealed the unrelated later pending request")
			}
		})
	}
}

func TestOrderIntentGenericRecoveryTerminalRequiresOriginalProof(t *testing.T) {
	e := lbuNew(t)
	key, body, recoveryKey := t04Key("oigr-sealed"), mnrBody(e, 1), t04Key("oigr-seal-transport")
	orders, held, planned := e.count(`SELECT count(*) FROM checkout.orders WHERE store_id=$1`, e.store()), e.reserved(), e.plan.planned()
	if status, out := mnrRecover(t, e, e.token(), key, body); status != 200 || out["state"] != "not_created" {
		t.Fatalf("real sealed fixture: status=%d code=%v", status, out["code"])
	}
	if status, out := oigrRecover(t, e, e.token(), recoveryKey); status != 200 || out["state"] != "not_created" || len(out) != 1 {
		t.Fatalf("terminal generic recovery: status=%d state=%v code=%v", status, out["state"], out["code"])
	}
	oisCorruptProof(t, e, `UPDATE ops.command_results c SET response=jsonb_build_object('owner_id','00000000-0000-4000-8000-000000000001') FROM ops.order_intents i WHERE c.tenant_id=i.tenant_id AND c.store_id=i.store_id AND c.operation='merchanttools.order.checkout.closed' AND c.idempotency_key=i.begin_key AND i.tenant_id=$1 AND i.store_id=$2 AND i.idempotency_key=$3`, e.tenant(), e.store(), key)
	status, out := oigrRecover(t, e, e.token(), recoveryKey)
	if status != 409 || out["code"] != "idempotency_conflict" || out["state"] != nil || out["order_id"] != nil {
		t.Fatalf("cached transport bypassed the original absence proof: status=%d code=%v state=%v", status, out["code"], out["state"])
	}
	if e.count(`SELECT count(*) FROM checkout.orders WHERE store_id=$1`, e.store()) != orders || e.reserved() != held || e.plan.planned() != planned {
		t.Fatal("terminal generic recovery changed checkout, stock or DM")
	}
}
