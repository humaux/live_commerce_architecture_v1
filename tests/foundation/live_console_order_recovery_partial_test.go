// Purpose: kill actual PG pipeline transactions and distinguish checkout commit from full A16 completion.
// Depends on: real merchant HTTP/checkout/claims fixtures; scoped owner-only trigger faults; recording MOCK DM planner.
// Used by: REAL_PG focused recovery gates; the drawer browser gate independently checks the real queued DM.
// Invariants: partial orders retain the original attempt; explicit replay completes each durable side effect once.
package foundation_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"testing"
	"time"
)

func TestLiveConsoleRecoveryPartialPipeline(t *testing.T) {
	for _, c := range []struct {
		name, operation   string
		manual, upper, dm int
	}{
		{"BeginOnly", "merchanttools.order.manual", 0, 0, 0},
		{"ManualOnly", "merchanttools.order.for_buyer", 1, 0, 0},
		{"DMCommitted", "merchanttools.order.for_buyer.completed", 1, 1, 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			e := lbuNew(t)
			beforeOrders := e.count(`SELECT count(*) FROM checkout.orders WHERE store_id=$1`, e.store())
			beforeHeld := e.reserved()
			_, _, bundle, conv := e.scenario()
			key := t04Key("lcr-partial")
			body := e.body(2, []string{bundle}, conv, true)
			lcrPartialKill(t, e, key, body, c.operation)
			if n := e.count(`SELECT count(*) FROM checkout.orders WHERE store_id=$1`, e.store()); n != beforeOrders+1 || e.reserved() != beforeHeld+2 {
				t.Fatalf("fault must follow durable checkout: orders delta=%d held delta=%d", n-beforeOrders, e.reserved()-beforeHeld)
			}
			for op, want := range map[string]int{"merchanttools.order.manual": c.manual, "merchanttools.order.for_buyer": c.upper, "merchanttools.order.for_buyer.completed": 0} {
				if n := e.count(`SELECT count(*) FROM ops.command_results WHERE store_id=$1 AND operation=$2 AND idempotency_key=$3`, e.store(), op, key); n != want {
					t.Fatalf("partial receipt %s count=%d want=%d", op, n, want)
				}
			}
			if e.orders() != c.manual || e.plan.planned() != c.dm {
				t.Fatal("fault did not preserve the expected incomplete pipeline stage")
			}
			status, out := lcrRecover(t, e, e.token(), key, body)
			if status != 503 || out["code"] != "retry_later" || out["order_id"] != nil {
				t.Fatalf("partial checkout must stay UNKNOWN: status=%d code=%v", status, out["code"])
			}
			if e.count(`SELECT count(*) FROM ops.command_results WHERE store_id=$1 AND operation IN ('merchanttools.order.attempt.recovery','merchanttools.order.checkout.closed')`, e.store()) != 0 {
				t.Fatal("partial recovery wrote definitive authority")
			}
			status, out = e.post(e.token(), key, body)
			if status != 201 && status != 200 {
				t.Fatalf("explicit original-key retry failed: status=%d code=%v", status, out["code"])
			}
			id := out["order_id"]
			for i := 0; i < 2; i++ {
				status, replay := e.post(e.token(), key, body)
				if status != 200 || replay["order_id"] != id {
					t.Fatal("completed replay changed order identity")
				}
			}
			status, out = lcrRecover(t, e, e.token(), key, body)
			if status != 200 || out["state"] != "created" || out["order_id"] != id {
				t.Fatal("full pipeline was not recoverable after explicit retry")
			}
			if e.count(`SELECT count(*) FROM checkout.orders WHERE store_id=$1`, e.store()) != beforeOrders+1 || e.orders() != 1 || e.reserved() != beforeHeld+2 || e.plan.planned() != 1 {
				t.Fatal("retry duplicated order, hold, source or DM")
			}
			for _, op := range []string{"merchanttools.order.manual", "merchanttools.order.for_buyer", "merchanttools.order.for_buyer.completed"} {
				if e.count(`SELECT count(*) FROM ops.command_results WHERE store_id=$1 AND operation=$2 AND idempotency_key=$3`, e.store(), op, key) != 1 {
					t.Fatal("completed receipt missing or duplicated")
				}
			}
			if e.count(`SELECT count(*) FROM inbox.order_for_buyer WHERE bundle_id=$1 AND state='placed' AND order_id=$2`, bundle, id) != 1 {
				t.Fatal("original bundle was not completed")
			}
			for _, action := range []string{"order.manual_created", "order.for_buyer_created", "claims.merchant_origin_granted"} {
				if e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action=$2`, e.store(), action) != 1 {
					t.Fatal("pipeline audit missing or duplicated")
				}
			}
		})
	}
}

func TestLiveConsoleRecoveryPartialPastSourceWindow(t *testing.T) {
	e := lbuNew(t)
	key := t04Key("lcr-aged-partial")
	body := e.body(2, nil, "", false)
	lcrPartialKill(t, e, key, body, "merchanttools.order.manual")
	// Disclosed owner-only aging verifies the existing source guard; the product may not relabel an old storefront order.
	beginDigest := sha256.Sum256([]byte("mt|begin|" + key))
	beginKey := fmt.Sprintf("mt-%x", beginDigest[:16])
	mustExec(t, e.p.f.owner, `UPDATE checkout.orders SET created_at=created_at-interval '11 minutes',expires_at=expires_at-interval '11 minutes' WHERE store_id=$1 AND id=(SELECT order_id FROM checkout.command_results WHERE store_id=$1 AND operation='checkout.begin' AND idempotency_key=$2)`, e.store(), beginKey)
	status, out := e.post(e.token(), key, body)
	if status != 409 || out["code"] != "conflict" {
		t.Fatalf("old source guard status=%d code=%v", status, out["code"])
	}
	status, out = lcrRecover(t, e, e.token(), key, body)
	if status != 503 || out["code"] != "retry_later" || out["order_id"] != nil {
		t.Fatal("aged partial finalized or released the attempt")
	}
	if e.orders() != 0 || e.count(`SELECT count(*) FROM checkout.order_links WHERE store_id=$1`, e.store()) != 0 || e.count(`SELECT count(*) FROM ops.command_results WHERE store_id=$1 AND operation IN ('merchanttools.order.attempt.recovery','merchanttools.order.checkout.closed','merchanttools.order.for_buyer.completed')`, e.store()) != 0 {
		t.Fatal("aged partial gained source, link or false completion authority")
	}
}

func TestLiveConsoleRecoveryOldCreatedNeedsCompletion(t *testing.T) {
	e := lbuNew(t)
	key := t04Key("lcr-old-created")
	body := e.body(2, nil, "", false)
	status, placed := e.post(e.token(), key, body)
	if status != 201 {
		t.Fatal("placement setup failed")
	}
	status, _ = lcrRecover(t, e, e.token(), key, body)
	if status != 200 {
		t.Fatal("completed recovery setup failed")
	}
	// Models a pre-fix created recovery receipt: it must not act as full-pipeline authority on its own.
	mustExec(t, e.p.f.owner, `DELETE FROM ops.command_results WHERE store_id=$1 AND idempotency_key=$2 AND operation='merchanttools.order.for_buyer.completed'`, e.store(), key)
	status, out := lcrRecover(t, e, e.token(), key, body)
	if status != 503 || out["code"] != "retry_later" || out["order_id"] != nil {
		t.Fatal("old created receipt bypassed completion proof")
	}
	status, out = e.post(e.token(), key, body)
	if status != 200 || out["order_id"] != placed["order_id"] {
		t.Fatal("legacy completed replay failed")
	}
	status, out = lcrRecover(t, e, e.token(), key, body)
	if status != 200 || out["order_id"] != placed["order_id"] || e.orders() != 1 || e.reserved() != 2 {
		t.Fatal("completion replay changed the order")
	}
}

// This BEFORE INSERT barrier rolls back the upper transaction, including source/link/audit writes,
// while the independently committed checkout order survives. Only the exact owned advisory waiter is killed.
func lcrPartialKill(t *testing.T, e *lbuEnv, key string, body map[string]any, operation string) {
	t.Helper()
	barrier := "lcr-partial|" + e.store()
	mustExec(t, e.p.f.owner, fmt.Sprintf(`CREATE FUNCTION ops.lcr_partial_pause() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NEW.store_id='%s'::uuid AND NEW.idempotency_key='%s' AND NEW.operation='%s' THEN
 PERFORM set_config('lock_timeout','0',true); PERFORM pg_advisory_xact_lock(hashtextextended('%s',0)); END IF; RETURN NEW; END $$`, e.store(), key, operation, barrier))
	mustExec(t, e.p.f.owner, `CREATE TRIGGER lcr_partial_pause BEFORE INSERT ON ops.command_results FOR EACH ROW EXECUTE FUNCTION ops.lcr_partial_pause()`)
	t.Cleanup(func() {
		mustExec(t, e.p.f.owner, `DROP TRIGGER lcr_partial_pause ON ops.command_results`)
		mustExec(t, e.p.f.owner, `DROP FUNCTION ops.lcr_partial_pause()`)
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
	go func() { status, _ := e.post(e.token(), key, body); done <- status }()
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
		t.Fatal("owned pipeline did not reach its committed checkout barrier")
	}
	var killed bool
	if err = e.p.f.owner.QueryRow(context.Background(), `SELECT pg_terminate_backend($1)`, pid).Scan(&killed); err != nil || !killed {
		t.Fatal("owned pipeline backend was not terminated")
	}
	if status := <-done; status != 503 {
		t.Fatalf("terminated request status=%d want503", status)
	}
	if err = hold.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
}
