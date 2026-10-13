// Purpose: manual UNKNOWN recovery through the actual merchant HTTP router and disposable PostgreSQL.
// Depends on: lbuNew, checkout Begin receipts, manual source/link/audit and owner-only PG fault barriers.
// Used by: focused REAL_PG acceptance and foundation CI; storefront-v2 G3.
// Invariants: I01/I02/I03/I06/I11; Begin-only cannot release UNKNOWN; absence seals the original key atomically.
package foundation_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"
)

func mnrBody(e *lbuEnv, qty int) map[string]any {
	return mtBody(e.money, qty, e.homeKey, "bank_transfer", mtHome)
}

func mnrRecover(t *testing.T, e *lbuEnv, token, key string, body map[string]any) (int, map[string]any) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	w := e.admin(token).call("POST", "/orders/manual/recover", key, "application/json", raw)
	out := map[string]any{}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func mnrDefinitiveCount(e *lbuEnv) int {
	return e.count(`SELECT count(*) FROM ops.command_results WHERE store_id=$1 AND operation IN ('merchanttools.order.attempt.recovery','merchanttools.order.checkout.closed')`, e.store())
}

func TestManualRecoveryAbsentSeal(t *testing.T) {
	for _, name := range []string{"removed_option", "invalid_original"} {
		t.Run(name, func(t *testing.T) {
			e := lbuNew(t)
			body, key := mnrBody(e, 2), t04Key("mnr-absent")
			if name == "removed_option" {
				mustExec(t, e.p.f.owner, `UPDATE checkout.bank_transfer_settings SET enabled=false WHERE store_id=$1`, e.store())
			} else {
				body["customer"].(map[string]any)["phone"] = "invalid"
			}
			beforeOrders, beforeHeld := e.count(`SELECT count(*) FROM checkout.orders WHERE store_id=$1`, e.store()), e.reserved()
			status, out := e.admin(e.token()).place(key, body)
			if status != 422 {
				t.Fatalf("original refusal status=%d code=%v", status, out["code"])
			}
			// Treat the refused response as lost: authority must independently prove and seal absence.
			for i := 0; i < 2; i++ {
				status, out = mnrRecover(t, e, e.token(), key, body)
				if status != 200 || out["state"] != "not_created" || len(out) != 1 {
					t.Fatalf("absence status=%d state=%v fields=%d", status, out["state"], len(out))
				}
			}
			if mnrDefinitiveCount(e) != 2 || e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='order.attempt_recovered'`, e.store()) != 1 {
				t.Fatal("absence seal/audit not exactly once")
			}
			mustExec(t, e.p.f.owner, `UPDATE checkout.bank_transfer_settings SET enabled=true WHERE store_id=$1`, e.store())
			status, out = e.admin(e.token()).place(key, body)
			if status != 409 || out["code"] != "attempt_closed" {
				t.Fatalf("sealed placement=%d code=%v", status, out["code"])
			}
			if e.count(`SELECT count(*) FROM checkout.orders WHERE store_id=$1`, e.store()) != beforeOrders || e.reserved() != beforeHeld || e.count(`SELECT count(*) FROM checkout.order_links WHERE store_id=$1`, e.store()) != 0 {
				t.Fatal("recovery changed order/hold/link")
			}
		})
	}
}

func TestManualRecoveryCreatedReadback(t *testing.T) {
	e := lbuNew(t)
	body, key := mnrBody(e, 2), t04Key("mnr-created")
	beforeOrders, beforeHeld := e.count(`SELECT count(*) FROM checkout.orders WHERE store_id=$1`, e.store()), e.reserved()
	status, placed := e.admin(e.token()).place(key, body)
	if status != 201 {
		t.Fatalf("place status=%d code=%v", status, placed["code"])
	}
	id := placed["order_id"]
	// Delivery options disappearing after the response was lost cannot prevent authoritative readback.
	mustExec(t, e.p.f.owner, `UPDATE checkout.bank_transfer_settings SET enabled=false WHERE store_id=$1`, e.store())
	for i := 0; i < 2; i++ {
		if i == 1 {
			body["customer"].(map[string]any)["name"] = "  王小明  "
		}
		status, out := mnrRecover(t, e, e.token(), key, body)
		if status != 200 || out["state"] != "created" || out["order_id"] != id || len(out) != 2 {
			t.Fatalf("created status=%d state=%v fields=%d", status, out["state"], len(out))
		}
	}
	if e.count(`SELECT count(*) FROM checkout.orders WHERE store_id=$1`, e.store()) != beforeOrders+1 || e.reserved() != beforeHeld+2 || e.orders() != 1 {
		t.Fatal("readback changed order/hold/source")
	}
	if e.count(`SELECT count(*) FROM checkout.order_links WHERE store_id=$1 AND order_id=$2`, e.store(), id) != 1 || e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='order.manual_created'`, e.store()) != 1 || mnrDefinitiveCount(e) != 1 {
		t.Fatal("manual completion/link/audit proof missing")
	}
	// Both recovery and original placement must keep the original binding.
	changed := mnrBody(e, 3)
	for _, recover := range []bool{true, false} {
		var st int
		var out map[string]any
		if recover {
			st, out = mnrRecover(t, e, e.token(), key, changed)
		} else {
			st, out = e.admin(e.token()).place(key, changed)
		}
		if st != 409 || out["code"] != "idempotency_conflict" {
			t.Fatalf("changed body status=%d code=%v", st, out["code"])
		}
	}
}

func TestManualRecoveryPartialBegin(t *testing.T) {
	e := lbuNew(t)
	body, key := mnrBody(e, 2), t04Key("mnr-partial")
	beforeOrders, beforeHeld := e.count(`SELECT count(*) FROM checkout.orders WHERE store_id=$1`, e.store()), e.reserved()
	mnrPartialKill(t, e, key, body)
	if e.count(`SELECT count(*) FROM checkout.orders WHERE store_id=$1`, e.store()) != beforeOrders+1 || e.reserved() != beforeHeld+2 || e.orders() != 0 {
		t.Fatal("fault must retain only committed Begin")
	}
	if e.count(`SELECT count(*) FROM checkout.order_links WHERE store_id=$1`, e.store()) != 0 || e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='order.manual_created'`, e.store()) != 0 {
		t.Fatal("partial gained source/link/audit")
	}
	status, out := mnrRecover(t, e, e.token(), key, body)
	if status != 503 || out["code"] != "retry_later" || out["order_id"] != nil || mnrDefinitiveCount(e) != 0 {
		t.Fatalf("partial recovery status=%d code=%v", status, out["code"])
	}
	status, placed := e.admin(e.token()).place(key, body)
	if status != 201 {
		t.Fatalf("explicit original replay status=%d code=%v", status, placed["code"])
	}
	id := placed["order_id"]
	for i := 0; i < 2; i++ {
		status, out = e.admin(e.token()).place(key, body)
		if status != 200 || out["order_id"] != id {
			t.Fatal("original replay duplicated order identity")
		}
	}
	status, out = mnrRecover(t, e, e.token(), key, body)
	if status != 200 || out["state"] != "created" || out["order_id"] != id {
		t.Fatal("complete pipeline not recoverable")
	}
	if e.count(`SELECT count(*) FROM checkout.orders WHERE store_id=$1`, e.store()) != beforeOrders+1 || e.reserved() != beforeHeld+2 || e.orders() != 1 || e.count(`SELECT count(*) FROM checkout.order_links WHERE store_id=$1`, e.store()) != 1 || e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='order.manual_created'`, e.store()) != 1 {
		t.Fatal("original retry duplicated durable effects")
	}
}

func TestManualRecoveryAuthorityAndKind(t *testing.T) {
	e := lbuNew(t)
	body, key := mnrBody(e, 1), t04Key("mnr-auth")
	for _, permissions := range [][]string{{"inventory:reserve", "catalog:read"}, {"inventory:reserve", "orders:read"}, {"catalog:read", "orders:read"}} {
		token, _ := e.tcvEnv.member(permissions...)
		status, out := mnrRecover(t, e, token, key, body)
		if status != 403 || out["order_id"] != nil || mnrDefinitiveCount(e) != 0 {
			t.Fatalf("permission refusal status=%d", status)
		}
	}
	foreign := e.admin(e.token())
	foreign.store = "ffffffff-ffff-4fff-8fff-ffffffffffff"
	raw, _ := json.Marshal(body)
	if w := foreign.call("POST", "/orders/manual/recover", key, "application/json", raw); w.Code != 404 {
		t.Fatalf("scope refusal=%d", w.Code)
	}
	status, out := e.post(e.token(), key, e.body(1, nil, "", false))
	if status != 201 {
		t.Fatalf("for-buyer setup=%d code=%v", status, out["code"])
	}
	status, out = mnrRecover(t, e, e.token(), key, body)
	if status != 409 || out["code"] != "idempotency_conflict" || mnrDefinitiveCount(e) != 0 {
		t.Fatalf("kind conflict=%d code=%v", status, out["code"])
	}
}

func TestManualRecoveryCachedCreatedNeedsReceipt(t *testing.T) {
	e := lbuNew(t)
	body, key := mnrBody(e, 1), t04Key("mnr-cached")
	status, placed := e.admin(e.token()).place(key, body)
	if status != 201 {
		t.Fatalf("place=%d code=%v", status, placed["code"])
	}
	status, out := mnrRecover(t, e, e.token(), key, body)
	if status != 200 || out["state"] != "created" {
		t.Fatal("created setup failed")
	}
	// Disclosed owner fixture removes only the upper proof; cached recovery cannot bypass a fresh completion probe.
	mustExec(t, e.p.f.owner, `DELETE FROM ops.command_results WHERE store_id=$1 AND operation='merchanttools.order.manual' AND idempotency_key=$2`, e.store(), key)
	status, out = mnrRecover(t, e, e.token(), key, body)
	if status != 503 || out["code"] != "retry_later" || out["order_id"] != nil {
		t.Fatalf("cached created bypassed proof status=%d code=%v", status, out["code"])
	}
}

func TestManualRecoveryAuthorityAfterWait(t *testing.T) {
	e := lbuNew(t)
	token, principal := e.tcvEnv.member("inventory:reserve", "catalog:read", "orders:read")
	body, key := mnrBody(e, 1), t04Key("mnr-wait")
	mustExec(t, e.p.f.owner, `UPDATE checkout.bank_transfer_settings SET enabled=false WHERE store_id=$1`, e.store())
	status, out := e.admin(token).place(key, body)
	if status != 422 {
		t.Fatalf("setup refusal=%d code=%v", status, out["code"])
	}
	var owner string
	if err := e.p.f.owner.QueryRow(context.Background(), `SELECT owner_id::text FROM buyer.capability_sessions WHERE store_id=$1 ORDER BY created_at DESC LIMIT 1`, e.store()).Scan(&owner); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte("mt|begin|" + key))
	beginKey := fmt.Sprintf("mt-%x", digest[:16])
	hold, err := e.p.f.owner.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer hold.Rollback(context.Background())
	if _, err = hold.Exec(context.Background(), `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "checkout.begin|"+e.tenant()+"|"+e.store()+"|"+owner+"|"+beginKey); err != nil {
		t.Fatal(err)
	}
	done := make(chan int, 1)
	go func() { st, _ := mnrRecover(t, e, token, key, body); done <- st }()
	lcrWaitAdvisory(t, e, 1)
	mustExec(t, e.p.f.owner, `DELETE FROM identity.store_grants WHERE store_id=$1 AND principal_id=$2 AND permission='orders:read'`, e.store(), principal)
	if err = hold.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if st := <-done; st != 403 || mnrDefinitiveCount(e) != 0 {
		t.Fatalf("post-wait authority=%d", st)
	}
}

func TestManualRecoveryConcurrentSeal(t *testing.T) {
	e := lbuNew(t)
	body, key := mnrBody(e, 1), t04Key("mnr-concurrent")
	start := make(chan struct{})
	var wg sync.WaitGroup
	statuses := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			st, out := mnrRecover(t, e, e.token(), key, body)
			if out["state"] != "not_created" {
				st = 0
			}
			statuses <- st
		}()
	}
	close(start)
	wg.Wait()
	close(statuses)
	for st := range statuses {
		if st != 200 {
			t.Fatalf("concurrent status=%d", st)
		}
	}
	if mnrDefinitiveCount(e) != 2 || e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='order.attempt_recovered'`, e.store()) != 1 {
		t.Fatal("concurrent seals not exactly once")
	}
}

// Owner-only trigger kills precisely this task's blocked manual receipt transaction after checkout committed.
func mnrPartialKill(t *testing.T, e *lbuEnv, key string, body map[string]any) {
	t.Helper()
	barrier := "mnr-partial|" + e.store()
	mustExec(t, e.p.f.owner, fmt.Sprintf(`CREATE FUNCTION ops.mnr_partial_pause() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NEW.store_id='%s'::uuid AND NEW.idempotency_key='%s' AND NEW.operation='merchanttools.order.manual' THEN
 PERFORM set_config('lock_timeout','0',true);PERFORM pg_advisory_xact_lock(hashtextextended('%s',0));END IF;RETURN NEW;END $$`, e.store(), key, barrier))
	mustExec(t, e.p.f.owner, `CREATE TRIGGER mnr_partial_pause BEFORE INSERT ON ops.command_results FOR EACH ROW EXECUTE FUNCTION ops.mnr_partial_pause()`)
	t.Cleanup(func() {
		mustExec(t, e.p.f.owner, `DROP TRIGGER mnr_partial_pause ON ops.command_results`)
		mustExec(t, e.p.f.owner, `DROP FUNCTION ops.mnr_partial_pause()`)
	})
	hold, err := e.p.f.owner.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer hold.Rollback(context.Background())
	if _, err = hold.Exec(context.Background(), `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, barrier); err != nil {
		t.Fatal(err)
	}
	done := make(chan int, 1)
	go func() { status, _ := e.admin(e.token()).place(key, body); done <- status }()
	deadline := time.Now().Add(5 * time.Second)
	var pid int
	for time.Now().Before(deadline) {
		_ = e.p.f.owner.QueryRow(context.Background(), `SELECT pid FROM pg_locks WHERE locktype='advisory' AND NOT granted AND objsubid=1 AND classid=((hashtextextended($1,0)>>32)&4294967295)::oid AND objid=(hashtextextended($1,0)&4294967295)::oid`, barrier).Scan(&pid)
		if pid != 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if pid == 0 {
		t.Fatal("owned request did not reach receipt barrier")
	}
	var killed bool
	if err = e.p.f.owner.QueryRow(context.Background(), `SELECT pg_terminate_backend($1)`, pid).Scan(&killed); err != nil || !killed {
		t.Fatal("exact owned backend not terminated")
	}
	if status := <-done; status != 503 {
		t.Fatalf("terminated status=%d", status)
	}
	if err = hold.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
}
