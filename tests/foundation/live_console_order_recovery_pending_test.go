// Purpose: REAL_PG A16R regression for a committed bundle reservation before checkout Begin.
// Depends on: lbuNew/lcrRecover real-router fixtures, scoped PG fault barriers, claims reservation/grant ledgers.
// Used by: focused recovery acceptance and foundation CI; no provider or fabricated API response.
// Invariants: I01/I02/I03/I04/I06/I11; not_created atomically seals and releases only the original pending attempt.
package foundation_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"testing"
	"time"
)

func TestLiveConsoleRecoveryPendingReservationReleasesOnlyOriginalAttempt(t *testing.T) {
	for _, bundleCount := range []int{1, 2} {
		t.Run(fmt.Sprintf("bundles-%d", bundleCount), func(t *testing.T) {
			lcrpReleaseOriginalAttempt(t, bundleCount)
		})
	}
}

func lcrpReleaseOriginalAttempt(t *testing.T, bundleCount int) {
	t.Helper()
	e := lbuNew(t)
	baseline := lcrpCheckoutBaseline(e)
	body := lcrpPendingBundleBody(t, e, bundleCount)
	_, _, otherBundle, otherConversation := e.scenario()
	key, otherKey := t04Key("lcrp-original"), t04Key("lcrp-other")
	otherBody := e.body(2, []string{otherBundle}, otherConversation, false)
	barrier, remove := lcrpFaultBarrier(t, e, "destination")
	lcrpInterrupt(t, e, barrier, func() (int, map[string]any) { return e.post(e.token(), key, body) })
	lcrpInterrupt(t, e, barrier, func() (int, map[string]any) { return e.post(e.token(), otherKey, otherBody) })
	remove()
	pending := lcrpFacts{Pending: bundleCount, LiveGrants: bundleCount}
	unrelated := lcrpFacts{Pending: 1, LiveGrants: 1}
	lcrpRequireFacts(t, e, key, pending)
	lcrpRequireFacts(t, e, otherKey, unrelated)
	lcrpRequireNoCheckout(t, e, baseline)

	status, recovered := lcrRecover(t, e, e.token(), key, body)
	if status != 200 || recovered["state"] != "not_created" || recovered["order_id"] != nil {
		t.Fatalf("pending recovery status=%d state=%s", status, lbuStr(recovered, "state"))
	}
	// RED target: the old handler seals not_created but leaves this pending row and its grant live.
	lcrpRequireFacts(t, e, key, lcrpFacts{Released: bundleCount, ExpiredGrants: bundleCount, Seals: 1, Recoveries: 1})
	lcrpRequireAtomicReleaseRows(t, e, key, bundleCount)
	lcrpRequireFacts(t, e, otherKey, unrelated)
	lcrpRequireNoCheckout(t, e, baseline)

	newKey := t04Key("lcrp-new")
	status, placed := e.post(e.token(), newKey, body)
	if status != 201 || lbuStr(placed, "order_id") == "" {
		t.Fatalf("immediate new attempt status=%d code=%s", status, lbuCode(placed))
	}
	if e.orders() != 1 || e.reserved() != baseline.held+2 || e.plan.planned() != 0 {
		t.Fatal("new attempt did not produce exactly one order/hold and zero DM")
	}
	// Quantity remains two units on one SKU: one bundle prices that line; Finish expires the other bundle's unused grant.
	lcrpRequireFacts(t, e, newKey, lcrpFacts{Placed: bundleCount, OrderLinks: bundleCount, ConsumedGrants: 1, ExpiredGrants: bundleCount - 1})
	lcrpRequireFacts(t, e, otherKey, unrelated)

	// Neither replaying the old seal nor recovering the completed new attempt may release its placed row.
	for i := 0; i < 2; i++ {
		status, replay := e.post(e.token(), key, body)
		if status != 409 || lbuCode(replay) != "attempt_closed" {
			t.Fatalf("late old attempt status=%d code=%s", status, lbuCode(replay))
		}
		status, oldRecovery := lcrRecover(t, e, e.token(), key, body)
		if status != 200 || oldRecovery["state"] != "not_created" {
			t.Fatalf("old recovery replay status=%d state=%s", status, lbuStr(oldRecovery, "state"))
		}
	}
	status, created := lcrRecover(t, e, e.token(), newKey, body)
	if status != 200 || created["state"] != "created" || created["order_id"] != placed["order_id"] {
		t.Fatalf("created recovery status=%d state=%s", status, lbuStr(created, "state"))
	}
	lcrpRequireFacts(t, e, key, lcrpFacts{Released: bundleCount, ExpiredGrants: bundleCount, Seals: 1, Recoveries: 1})
	lcrpRequireFacts(t, e, newKey, lcrpFacts{Placed: bundleCount, OrderLinks: bundleCount, ConsumedGrants: 1, ExpiredGrants: bundleCount - 1, Recoveries: 1})
	lcrpRequireFacts(t, e, otherKey, unrelated)
	if e.orders() != 1 || e.reserved() != baseline.held+2 {
		t.Fatal("recovery replay changed the one placed order/hold")
	}
}

func TestLiveConsoleRecoveryPendingReservationReleaseRollsBackWithSeal(t *testing.T) {
	for _, bundleCount := range []int{1, 2} {
		t.Run(fmt.Sprintf("bundles-%d", bundleCount), func(t *testing.T) {
			lcrpReleaseRollback(t, bundleCount)
		})
	}
}

func lcrpReleaseRollback(t *testing.T, bundleCount int) {
	t.Helper()
	e := lbuNew(t)
	baseline := lcrpCheckoutBaseline(e)
	key := t04Key("lcrp-atomic")
	body := lcrpPendingBundleBody(t, e, bundleCount)
	barrier, remove := lcrpFaultBarrier(t, e, "destination")
	lcrpInterrupt(t, e, barrier, func() (int, map[string]any) { return e.post(e.token(), key, body) })
	remove()
	pending := lcrpFacts{Pending: bundleCount, LiveGrants: bundleCount}
	lcrpRequireFacts(t, e, key, pending)
	lcrpRequireNoCheckout(t, e, baseline)

	barrier, remove = lcrpFaultBarrier(t, e, "seal")
	lcrpInterrupt(t, e, barrier, func() (int, map[string]any) { return lcrRecover(t, e, e.token(), key, body) })
	remove()
	// Killing the seal transaction must retain both reservation and usable grant, with neither receipt committed.
	lcrpRequireFacts(t, e, key, pending)
	lcrpRequireNoCheckout(t, e, baseline)
	status, recovered := lcrRecover(t, e, e.token(), key, body)
	if status != 200 || recovered["state"] != "not_created" {
		t.Fatalf("atomic recovery retry status=%d state=%s", status, lbuStr(recovered, "state"))
	}
	lcrpRequireFacts(t, e, key, lcrpFacts{Released: bundleCount, ExpiredGrants: bundleCount, Seals: 1, Recoveries: 1})
	lcrpRequireAtomicReleaseRows(t, e, key, bundleCount)
	lcrpRequireNoCheckout(t, e, baseline)
}

// lcrpPendingBundleBody reuses accepted claims with matching peer evidence; bundle count never multiplies the requested SKU quantity.
func lcrpPendingBundleBody(t *testing.T, e *lbuEnv, bundleCount int) map[string]any {
	t.Helper()
	if bundleCount == 1 {
		_, _, bundle, conversation := e.scenario()
		return e.body(2, []string{bundle}, conversation, false)
	}
	if bundleCount != 2 {
		t.Fatal("unsupported pending bundle fixture count")
	}
	b1, b2, conversation := e.twoBundles()
	return e.body(2, []string{b1, b2}, conversation, false)
}

// lcrpFacts contains only counts; errors cannot expose keys, request bodies, private owners or buyer data.
type lcrpFacts struct {
	Pending, Released, Placed, OrderLinks     int
	LiveGrants, ExpiredGrants, ConsumedGrants int
	Seals, Recoveries                         int
}

func lcrpKeys(e *lbuEnv, key string) ([]byte, string) {
	keyHash := sha256.Sum256([]byte("order-for-buyer|" + e.store() + "|" + key))
	beginHash := sha256.Sum256([]byte("mt|begin|" + key))
	return keyHash[:], fmt.Sprintf("mt-%x", beginHash[:16])
}

func lcrpRequireFacts(t *testing.T, e *lbuEnv, key string, want lcrpFacts) {
	t.Helper()
	keyHash, beginKey := lcrpKeys(e, key)
	var got lcrpFacts
	err := e.p.f.owner.QueryRow(context.Background(), `WITH reservations AS (
 SELECT * FROM inbox.order_for_buyer WHERE tenant_id=$1 AND store_id=$2 AND idempotency_key_hash=$3
), grants AS (
 SELECT g.* FROM claims.merchant_origin_grants g JOIN reservations r
 ON (g.tenant_id,g.store_id,g.request_id,g.bundle_id)=(r.tenant_id,r.store_id,r.request_id,r.bundle_id)
)
 SELECT (SELECT count(*) FROM reservations WHERE state='pending'),
 (SELECT count(*) FROM reservations WHERE state='released'),
 (SELECT count(*) FROM reservations WHERE state='placed'),
 (SELECT count(*) FROM reservations WHERE order_id IS NOT NULL),
 (SELECT count(*) FROM grants WHERE consumed_at IS NULL AND expires_at>clock_timestamp()),
 (SELECT count(*) FROM grants WHERE consumed_at IS NULL AND expires_at<=clock_timestamp()),
 (SELECT count(*) FROM grants WHERE consumed_at IS NOT NULL),
 (SELECT count(*) FROM ops.command_results WHERE tenant_id=$1 AND store_id=$2
 AND operation='merchanttools.order.checkout.closed' AND idempotency_key=$4),
 (SELECT count(*) FROM ops.command_results WHERE tenant_id=$1 AND store_id=$2
 AND operation='merchanttools.order.attempt.recovery' AND idempotency_key=$5)`,
		e.tenant(), e.store(), keyHash, beginKey, key).Scan(&got.Pending, &got.Released, &got.Placed, &got.OrderLinks,
		&got.LiveGrants, &got.ExpiredGrants, &got.ConsumedGrants, &got.Seals, &got.Recoveries)
	if err != nil {
		t.Fatal("pending recovery ledger query failed")
	}
	if got != want {
		t.Fatalf("pending recovery ledger counts: got=%+v want=%+v", got, want)
	}
}

type lcrpCheckoutFacts struct {
	orders, begins int
	held           int64
}

// The shared fixture seeds ordinary storefront orders; faults must preserve their exact counts.
func lcrpCheckoutBaseline(e *lbuEnv) lcrpCheckoutFacts {
	return lcrpCheckoutFacts{
		orders: e.count(`SELECT count(*) FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store()),
		begins: e.count(`SELECT count(*) FROM checkout.command_results WHERE tenant_id=$1 AND store_id=$2 AND operation='checkout.begin'`, e.tenant(), e.store()),
		held:   e.reserved(),
	}
}

func lcrpRequireNoCheckout(t *testing.T, e *lbuEnv, baseline lcrpCheckoutFacts) {
	t.Helper()
	if lcrpCheckoutBaseline(e) != baseline || e.orders() != 0 || e.plan.planned() != 0 {
		t.Fatal("pending fixture unexpectedly passed checkout Begin or planned a DM")
	}
}

func lcrpRequireAtomicRelease(t *testing.T, e *lbuEnv, key string) {
	t.Helper()
	lcrpRequireAtomicReleaseRows(t, e, key, 1)
}

// Every original reservation and grant must share the same transaction as both authoritative receipts.
func lcrpRequireAtomicReleaseRows(t *testing.T, e *lbuEnv, key string, wantRows int) {
	t.Helper()
	keyHash, beginKey := lcrpKeys(e, key)
	n := e.count(`SELECT count(*) FROM inbox.order_for_buyer x
 JOIN claims.merchant_origin_grants g ON (g.tenant_id,g.store_id,g.request_id,g.bundle_id)=(x.tenant_id,x.store_id,x.request_id,x.bundle_id)
 JOIN ops.command_results c ON c.tenant_id=x.tenant_id AND c.store_id=x.store_id AND c.operation='merchanttools.order.checkout.closed'
 AND c.idempotency_key=$4 AND c.xmin=x.xmin AND c.xmin=g.xmin
 JOIN ops.command_results r ON r.tenant_id=x.tenant_id AND r.store_id=x.store_id AND r.operation='merchanttools.order.attempt.recovery'
 AND r.idempotency_key=$5 AND r.xmin=c.xmin
 WHERE x.tenant_id=$1 AND x.store_id=$2 AND x.idempotency_key_hash=$3 AND x.state='released'
 AND g.consumed_at IS NULL AND g.expires_at<=clock_timestamp()`, e.tenant(), e.store(), keyHash, beginKey, key)
	if n != wantRows {
		t.Fatalf("release/grant expiry/seal/recovery shared transaction count=%d want=%d", n, wantRows)
	}
}

// lcrpFaultBarrier installs an owned store-scoped trigger; destination precedes checkout Begin, seal shares the release transaction.
func lcrpFaultBarrier(t *testing.T, e *lbuEnv, stage string) (string, func()) {
	t.Helper()
	schema, table, condition := "storefront", "destination_snapshots", ""
	if stage == "seal" {
		schema, table, condition = "ops", "command_results", " AND NEW.operation='merchanttools.order.checkout.closed'"
	} else if stage != "destination" {
		t.Fatal("unknown pending recovery fault stage")
	}
	name := "lcrp_pause_" + stage
	barrier := "lcrp-" + stage + "|" + e.store()
	mustExec(t, e.p.f.owner, fmt.Sprintf(`CREATE FUNCTION %s.%s() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NEW.store_id='%s'::uuid%s THEN PERFORM set_config('lock_timeout','0',true);
 PERFORM pg_advisory_xact_lock(hashtextextended('%s',0)); END IF; RETURN NEW; END $$`, schema, name, e.store(), condition, barrier))
	mustExec(t, e.p.f.owner, fmt.Sprintf(`CREATE TRIGGER %s BEFORE INSERT ON %s.%s FOR EACH ROW EXECUTE FUNCTION %s.%s()`, name, schema, table, schema, name))
	removed := false
	remove := func() {
		if !removed {
			mustExec(t, e.p.f.owner, fmt.Sprintf(`DROP TRIGGER %s ON %s.%s`, name, schema, table))
			mustExec(t, e.p.f.owner, fmt.Sprintf(`DROP FUNCTION %s.%s()`, schema, name))
			removed = true
		}
	}
	t.Cleanup(remove)
	return barrier, remove
}

func lcrpInterrupt(t *testing.T, e *lbuEnv, barrier string, call func() (int, map[string]any)) {
	t.Helper()
	ctx := context.Background()
	hold, err := e.p.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal("pending recovery barrier transaction failed")
	}
	defer hold.Rollback(ctx)
	if _, err = hold.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, barrier); err != nil {
		t.Fatal("pending recovery barrier acquisition failed")
	}
	type response struct {
		status int
		code   string
	}
	done := make(chan response, 1)
	go func() { status, out := call(); done <- response{status, lbuCode(out)} }()
	deadline := time.Now().Add(5 * time.Second)
	waiting := false
	for time.Now().Before(deadline) {
		if e.count(`SELECT count(*) FROM pg_locks WHERE locktype='advisory' AND NOT granted AND objsubid=1
 AND classid=((hashtextextended($1,0)>>32)&4294967295)::oid AND objid=(hashtextextended($1,0)&4294967295)::oid`, barrier) == 1 {
			waiting = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !waiting {
		t.Fatal("exact pending recovery fault waiter did not appear")
	}
	var killed bool
	if err = e.p.f.owner.QueryRow(ctx, `SELECT pg_terminate_backend(pid) FROM pg_locks
 WHERE locktype='advisory' AND NOT granted AND objsubid=1
 AND classid=((hashtextextended($1,0)>>32)&4294967295)::oid AND objid=(hashtextextended($1,0)&4294967295)::oid`, barrier).Scan(&killed); err != nil || !killed {
		t.Fatal("exact pending recovery fault waiter termination failed")
	}
	if err = hold.Commit(ctx); err != nil {
		t.Fatal("pending recovery barrier release failed")
	}
	select {
	case reply := <-done:
		if reply.status != 503 || reply.code != "retry_later" {
			t.Fatalf("interrupted real request status=%d code=%s", reply.status, reply.code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("interrupted real request did not finish")
	}
}
