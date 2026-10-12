// Purpose: A16R recovery gate through the real merchant router and checkout transaction on disposable PG.
// Depends on: lbuNew fixtures; merchanttools, checkout.begin receipts, ops.command_results; no provider.
// Used by: focused REAL_PG acceptance and full foundation CI.
// Invariants: I01/I02/I03/I06/I11; a missing merchant receipt is never proof of no checkout order.
package foundation_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"livecommerce/internal/buyer"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/merchanttools"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestLiveConsoleRecoveryFreshAuthorityAfterCheckoutWait(t *testing.T) {
	for _, c := range []struct {
		name   string
		want   int
		absent bool
	}{{"permission", 403, false}, {"store", 404, false}, {"session", 401, false}, {"permission-absent", 403, true}, {"store-absent", 404, true}, {"session-absent", 401, true}} {
		t.Run(c.name, func(t *testing.T) {
			e := lbuNew(t)
			token, principal := e.tcvEnv.member("inventory:reserve", "catalog:read", "orders:read")
			key := t04Key("lcr-revoke")
			body := e.body(2, nil, "", false)
			if c.absent {
				mustExec(t, e.p.f.owner, `UPDATE checkout.bank_transfer_settings SET enabled=false WHERE store_id=$1`, e.store())
			}
			status, placed := e.post(token, key, body)
			if (!c.absent && status != 201) || (c.absent && status != 422) {
				t.Fatalf("place: %d %v", status, placed)
			}
			var owner, beginKey string
			if !c.absent {
				if err := e.p.f.owner.QueryRow(context.Background(), `SELECT owner_id::text,idempotency_key FROM checkout.command_results WHERE store_id=$1 AND order_id=$2 AND operation='checkout.begin'`, e.store(), placed["order_id"]).Scan(&owner, &beginKey); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := e.p.f.owner.QueryRow(context.Background(), `SELECT owner_id::text FROM buyer.capability_sessions WHERE store_id=$1 ORDER BY created_at DESC LIMIT 1`, e.store()).Scan(&owner); err != nil {
					t.Fatal(err)
				}
				hash := sha256.Sum256([]byte("mt|begin|" + key))
				beginKey = fmt.Sprintf("mt-%x", hash[:16])
			}
			hold, err := e.p.f.owner.Begin(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			defer hold.Rollback(context.Background())
			if _, err = hold.Exec(context.Background(), `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "checkout.begin|"+e.tenant()+"|"+e.store()+"|"+owner+"|"+beginKey); err != nil {
				t.Fatal(err)
			}
			type result struct {
				status int
				out    map[string]any
			}
			finished := make(chan result, 1)
			go func() { st, out := lcrRecover(t, e, token, key, body); finished <- result{st, out} }()
			lcrWaitAdvisory(t, e, 1)
			switch strings.TrimSuffix(c.name, "-absent") {
			case "permission":
				mustExec(t, e.p.f.owner, `DELETE FROM identity.store_grants WHERE store_id=$1 AND principal_id=$2 AND permission='orders:read'`, e.store(), principal)
			case "store":
				mustExec(t, e.p.f.owner, `DELETE FROM identity.store_grants WHERE store_id=$1 AND principal_id=$2 AND permission='store:read'`, e.store(), principal)
			case "session":
				hash := sha256.Sum256([]byte(token))
				mustExec(t, e.p.f.owner, `UPDATE identity.sessions SET revoked_at=clock_timestamp() WHERE token_hash=$1`, hash[:])
			}
			if err = hold.Commit(context.Background()); err != nil {
				t.Fatal(err)
			}
			r := <-finished
			if r.status != c.want || r.out["order_id"] != nil {
				t.Fatalf("post-wait authority: %d %v want %d", r.status, r.out, c.want)
			}
			var receipts int
			if err = e.p.f.owner.QueryRow(context.Background(), `SELECT count(*) FROM ops.command_results WHERE store_id=$1 AND operation IN ('merchanttools.order.attempt.recovery','merchanttools.order.checkout.closed')`, e.store()).Scan(&receipts); err != nil {
				t.Fatal(err)
			}
			if receipts != 0 {
				t.Fatal("revoked recovery wrote an authoritative receipt")
			}
		})
	}
}

func TestLiveConsoleRecoveryTwoConnectionPool(t *testing.T) {
	e := lbuNew(t)
	config := e.p.f.runtime.Config()
	config.MaxConns, config.MinConns, config.MinIdleConns = 2, 0, 0
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	issuer, err := buyer.New(e.p.a.issuer, 30*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	manual, err := merchanttools.NewManualOrders(pool, issuer, e.p.a.runtime, e.svc, []byte(strings.Repeat("x", 32)))
	if err != nil {
		t.Fatal(err)
	}
	fb, err := merchanttools.NewForBuyer(manual, e.plan)
	if err != nil {
		t.Fatal(err)
	}
	e.manual, e.fb = manual, fb
	e.handler = httpapi.NewHandler(pool, httpapi.Options{CVS: e.cvs, ManualOrders: manual, ForBuyer: fb})
	raw, _ := json.Marshal(e.body(1, nil, "", false))
	var in merchanttools.ForBuyerInput
	if err = json.Unmarshal(raw, &in); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func(key string) {
			_, _, err := fb.Place(context.Background(), e.token(), e.store(), key, in)
			done <- err
		}(t04Key("lcr-pool"))
	}
	for i := 0; i < 2; i++ {
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("two-connection pipeline deadlocked")
		}
	}
	key := t04Key("lcr-pool-seal")
	status, out := lcrRecover(t, e, e.token(), key, e.body(1, nil, "", false))
	if status != 200 || out["state"] != "not_created" {
		t.Fatalf("pool seal: %d %v", status, out)
	}
	status, out = e.post(e.token(), key, e.body(1, nil, "", false))
	if status != 409 || out["code"] != "attempt_closed" {
		t.Fatalf("pool closed: %d %v", status, out)
	}
	if e.orders() != 2 || e.reserved() != 2 || e.plan.planned() != 0 {
		t.Fatal("bounded pool changed money/stock/DM")
	}
}

func TestLiveConsoleRecoveryLostSealConnectionRollsBack(t *testing.T) {
	e := lbuNew(t)
	key := t04Key("lcr-seal-loss")
	body := e.body(2, nil, "", false)
	barrier := "lcr-seal-insert|" + e.store()
	mustExec(t, e.p.f.owner, fmt.Sprintf(`CREATE FUNCTION ops.lcr_pause_seal() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NEW.store_id='%s'::uuid AND NEW.operation='merchanttools.order.checkout.closed' THEN PERFORM set_config('lock_timeout','0',true); PERFORM pg_advisory_xact_lock(hashtextextended('%s',0)); END IF; RETURN NEW; END $$`, e.store(), barrier))
	mustExec(t, e.p.f.owner, `CREATE TRIGGER lcr_pause_seal BEFORE INSERT ON ops.command_results FOR EACH ROW EXECUTE FUNCTION ops.lcr_pause_seal()`)
	t.Cleanup(func() {
		mustExec(t, e.p.f.owner, `DROP TRIGGER lcr_pause_seal ON ops.command_results`)
		mustExec(t, e.p.f.owner, `DROP FUNCTION ops.lcr_pause_seal()`)
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
	go func() { status, _ := lcrRecover(t, e, e.token(), key, body); done <- status }()
	lcrWaitAdvisory(t, e, 1)
	var killed bool
	// The closure INSERT waiter holds the checkout lock on THIS backend. Terminating it must roll back both receipts.
	if err = e.p.f.owner.QueryRow(context.Background(), `SELECT pg_terminate_backend(pid) FROM pg_locks WHERE locktype='advisory' AND NOT granted AND objsubid=1 AND classid=((hashtextextended($1,0)>>32)&4294967295)::oid AND objid=(hashtextextended($1,0)&4294967295)::oid`, barrier).Scan(&killed); err != nil || !killed {
		t.Fatalf("seal backend termination: %v", err)
	}
	if status := <-done; status != 503 {
		t.Fatalf("lost seal status=%d", status)
	}
	if err = hold.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	var receipts int
	if err = e.p.f.owner.QueryRow(context.Background(), `SELECT count(*) FROM ops.command_results WHERE store_id=$1 AND operation IN ('merchanttools.order.checkout.closed','merchanttools.order.attempt.recovery')`, e.store()).Scan(&receipts); err != nil {
		t.Fatal(err)
	}
	if receipts != 0 {
		t.Fatal("lost lock transaction left an authoritative receipt")
	}
	status, out := e.post(e.token(), key, body)
	if status != 201 {
		t.Fatalf("unsealed original: %d %v", status, out)
	}
	if e.orders() != 1 || e.reserved() != 2 {
		t.Fatal("unsealed original money mismatch")
	}
}

func TestLiveConsoleRecoveryLostOuterLockFencesCheckout(t *testing.T) {
	e := lbuNew(t)
	key := t04Key("lcr-lost-lock")
	body := e.body(2, nil, "", false)
	barrier := "lcr-before-begin|" + e.store()
	// Owned disposable fixture: stop the original transaction at destination insertion, before Begin.
	mustExec(t, e.p.f.owner, fmt.Sprintf(`CREATE FUNCTION storefront.lcr_pause_destination() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NEW.store_id='%s'::uuid THEN PERFORM set_config('lock_timeout','0',true); PERFORM pg_advisory_xact_lock(hashtextextended('%s',0)); END IF;
 RETURN NEW; END $$`, e.store(), barrier))
	mustExec(t, e.p.f.owner, `CREATE TRIGGER lcr_pause_destination BEFORE INSERT ON storefront.destination_snapshots FOR EACH ROW EXECUTE FUNCTION storefront.lcr_pause_destination()`)
	t.Cleanup(func() {
		mustExec(t, e.p.f.owner, `DROP TRIGGER lcr_pause_destination ON storefront.destination_snapshots`)
		mustExec(t, e.p.f.owner, `DROP FUNCTION storefront.lcr_pause_destination()`)
	})
	hold, err := e.p.f.owner.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer hold.Rollback(context.Background())
	if _, err = hold.Exec(context.Background(), `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, barrier); err != nil {
		t.Fatal(err)
	}
	finished := make(chan int, 1)
	go func() { status, _ := e.post(e.token(), key, body); finished <- status }()
	lcrWaitAdvisory(t, e, 1)
	var killed bool
	err = e.p.f.owner.QueryRow(context.Background(), `SELECT pg_terminate_backend(pid) FROM pg_locks
 WHERE locktype='advisory' AND granted AND objsubid=1
 AND classid=((hashtextextended($1,0)>>32)&4294967295)::oid
 AND objid=(hashtextextended($1,0)&4294967295)::oid`, "merchanttools.attempt|"+e.tenant()+"|"+e.store()+"|"+key).Scan(&killed)
	if err != nil || !killed {
		t.Fatalf("outer lock termination failed: %v", err)
	}
	status, out := lcrRecover(t, e, e.token(), key, body)
	if status != 200 || out["state"] != "not_created" {
		t.Fatalf("seal after outer loss: %d %v", status, out)
	}
	if err = hold.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("original request failed to finish after barrier release")
	}
	if e.orders() != 0 || e.reserved() != 0 || e.plan.planned() != 0 {
		t.Fatal("original created an order after recovery's authoritative seal lost its outer lock")
	}
	status, out = lcrRecover(t, e, e.token(), key, body)
	if status != 200 || out["state"] != "not_created" {
		t.Fatalf("lost-lock replay: %d %v", status, out)
	}
}

func lcrRecover(t *testing.T, e *lbuEnv, token, key string, body map[string]any) (int, map[string]any) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	w := e.admin(token).call("POST", "/orders/for-buyer/recover", key, "application/json", raw)
	var out map[string]any
	if err = json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("recovery status=%d invalid JSON", w.Code)
	}
	return w.Code, out
}

func TestLiveConsoleRecoveryStatic422CanCloseFrozenBody(t *testing.T) {
	for _, name := range []string{"recipient", "payment"} {
		t.Run(name, func(t *testing.T) {
			e := lbuNew(t)
			key := t04Key("lcr-invalid")
			body := e.body(2, nil, "", false)
			if name == "recipient" {
				body["customer"].(map[string]any)["phone"] = "invalid"
			} else {
				body["payment_mode"] = "invalid"
			}
			status, out := e.post(e.token(), key, body)
			if status != 422 {
				t.Fatalf("static original refusal: %d %v", status, out)
			}
			status, out = lcrRecover(t, e, e.token(), key, body)
			if status != 200 || out["state"] != "not_created" {
				t.Fatalf("static refusal recovery: %d %v", status, out)
			}
			status, out = e.post(e.token(), key, body)
			if status != 409 || out["code"] != "attempt_closed" {
				t.Fatalf("closed invalid original: %d %v", status, out)
			}
			status, out = e.post(e.token(), key, e.body(2, nil, "", false))
			if status != 409 || out["code"] != "idempotency_conflict" {
				t.Fatalf("samekey corrected body: %d %v", status, out)
			}
			if e.orders() != 0 || e.reserved() != 0 || e.plan.planned() != 0 {
				t.Fatal("static refusal recovery created money/stock/DM")
			}
		})
	}
}

func TestLiveConsoleRecoverySealLateAndLostReply(t *testing.T) {
	e := lbuNew(t)
	key := t04Key("lcr-seal")
	body := e.body(2, nil, "", false)
	before, held := e.orders(), e.reserved()
	st, out := lcrRecover(t, e, e.token(), key, body)
	if st != 200 || out["state"] != "not_created" || len(out) != 1 {
		t.Fatalf("seal: %d %v", st, out)
	}
	// Lost reply: the caller keeps the original key/body and resolves again.
	st, out = lcrRecover(t, e, e.token(), key, body)
	if st != 200 || out["state"] != "not_created" || len(out) != 1 {
		t.Fatalf("lost seal reply: %d %v", st, out)
	}
	st, out = e.post(e.token(), key, body)
	if st != 409 || out["code"] != "attempt_closed" {
		t.Fatalf("late original: %d %v", st, out)
	}
	st, out = lcrRecover(t, e, e.token(), key, body)
	if st != 200 || out["state"] != "not_created" {
		t.Fatalf("resolve after closed replay: %d %v", st, out)
	}
	changed := e.body(3, nil, "", false)
	st, out = lcrRecover(t, e, e.token(), key, changed)
	if st != 409 || out["code"] != "idempotency_conflict" {
		t.Fatalf("changed body: %d %v", st, out)
	}
	if e.orders() != before || e.reserved() != held || e.plan.planned() != 0 {
		t.Fatal("sealed attempt changed money/stock/DM")
	}
}

func TestLiveConsoleRecoveryCheckoutReceiptWithoutMerchantReceipt(t *testing.T) {
	e := lbuNew(t)
	key := t04Key("lcr-orphan")
	body := e.body(2, nil, "", false)
	st, placed := e.post(e.token(), key, body)
	if st != 201 {
		t.Fatalf("setup: %d %v", st, placed)
	}
	id := placed["order_id"]
	// Privileged fixture models the crash gap after checkout's durable order/receipt commit,
	// before either merchant receipt. Never deletes a checkout receipt, order or stock row.
	mustExec(t, e.p.f.owner, `DELETE FROM ops.command_results WHERE store_id=$1 AND idempotency_key=$2 AND operation IN ('merchanttools.order.manual','merchanttools.order.for_buyer')`, e.store(), key)
	// Dynamic payment policy changes before retry; the unchanged body now fails pre-Begin.
	mustExec(t, e.p.f.owner, `UPDATE checkout.bank_transfer_settings SET enabled=false WHERE store_id=$1`, e.store())
	before, held := e.orders(), e.reserved()
	st, out := e.post(e.token(), key, body)
	if st != 422 {
		t.Fatalf("need actual dynamic 422: %d %v", st, out)
	}
	st, out = lcrRecover(t, e, e.token(), key, body)
	if st != 200 || out["state"] != "created" || out["order_id"] != id || len(out) != 2 {
		t.Fatalf("orphan recovery: %d %v", st, out)
	}
	st, out = lcrRecover(t, e, e.token(), key, body)
	if st != 200 || out["order_id"] != id {
		t.Fatalf("lost created reply: %d %v", st, out)
	}
	if e.orders() != before || e.reserved() != held || e.plan.planned() != 0 {
		t.Fatal("created recovery changed money/stock/DM")
	}
}

func TestLiveConsoleRecoveryAuthority(t *testing.T) {
	e := lbuNew(t)
	body := e.body(1, nil, "", true)
	raw, _ := json.Marshal(body)
	for _, c := range []struct {
		name, token string
		status      int
	}{
		{"no session", "", 401},
	} {
		t.Run(c.name, func(t *testing.T) {
			w := e.admin(c.token).call("POST", "/orders/for-buyer/recover", t04Key("lcr-auth"), "application/json", raw)
			if w.Code != c.status {
				t.Fatalf("got %d", w.Code)
			}
		})
	}
	for _, permission := range []string{"inventory:reserve", "catalog:read", "orders:read"} {
		var permissions []string
		for _, p := range []string{"inventory:reserve", "catalog:read", "orders:read"} {
			if p != permission {
				permissions = append(permissions, p)
			}
		}
		token, _ := e.tcvEnv.member(permissions...)
		st, out := lcrRecover(t, e, token, t04Key("lcr-perm"), body)
		if st != http.StatusForbidden {
			t.Fatalf("without %s: %d %v", permission, st, out)
		}
	}
	// Recovery sends nothing, so inbox:reply is unnecessary even with send=true.
	st, out := lcrRecover(t, e, e.member(), t04Key("lcr-no-inbox"), body)
	if st != 200 || out["state"] != "not_created" {
		t.Fatalf("no inbox permission: %d %v", st, out)
	}
	foreign := e.admin(e.token())
	foreign.store = randomUUID()
	w := foreign.call("POST", "/orders/for-buyer/recover", t04Key("lcr-scope"), "application/json", raw)
	if w.Code != 404 {
		t.Fatalf("foreign scope=%d", w.Code)
	}
}

func TestLiveConsoleRecoveryDynamic422AndKindConflict(t *testing.T) {
	e := lbuNew(t)
	key := t04Key("lcr-dynamic")
	body := e.body(2, nil, "", false)
	mustExec(t, e.p.f.owner, `UPDATE checkout.bank_transfer_settings SET enabled=false WHERE store_id=$1`, e.store())
	st, out := e.post(e.token(), key, body)
	if st != 422 {
		t.Fatalf("dynamic refusal: %d %v", st, out)
	}
	st, out = lcrRecover(t, e, e.token(), key, body)
	if st != 200 || out["state"] != "not_created" {
		t.Fatalf("resolve dynamic: %d %v", st, out)
	}
	mustExec(t, e.p.f.owner, `UPDATE checkout.bank_transfer_settings SET enabled=true WHERE store_id=$1`, e.store())
	st, out = e.post(e.token(), key, body)
	if st != 409 || out["code"] != "attempt_closed" {
		t.Fatalf("late original after re-enable: %d %v", st, out)
	}
	// Manual and A16 share the same private checkout capability. Different route kind must not bypass a seal.
	raw, _ := json.Marshal(mtBody(e.money, 2, e.homeKey, "bank_transfer", mtHome))
	w := e.admin(e.token()).call("POST", "/orders/manual", key, "application/json", raw)
	var manual map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &manual)
	if w.Code != 409 || manual["code"] != "idempotency_conflict" {
		t.Fatalf("manual bypass: %d %v", w.Code, manual)
	}
	if e.orders() != 0 || e.reserved() != 0 {
		t.Fatal("closed key created money/stock after policy re-enable")
	}
}

func TestLiveConsoleRecoveryPlacementWinsConcurrentSeal(t *testing.T) {
	e := lbuNew(t)
	key := t04Key("lcr-place-first")
	body := e.body(2, nil, "", false)
	raw, _ := json.Marshal(body)
	var in merchanttools.ForBuyerInput
	_ = json.Unmarshal(raw, &in)
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	e.fb.FaultAfterPlace = func() error { close(entered); <-release; return errors.New("injected gap before for-buyer receipt") }
	placedDone := make(chan error, 1)
	go func() {
		_, _, err := e.fb.Place(context.Background(), e.token(), e.store(), key, in)
		placedDone <- err
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("placement never reached committed gap")
	}
	recovered := make(chan struct {
		status int
		out    map[string]any
	}, 1)
	go func() {
		st, out := lcrRecover(t, e, e.token(), key, body)
		recovered <- struct {
			status int
			out    map[string]any
		}{st, out}
	}()
	// Causal synchronization: recovery must be waiting on a PG advisory lock while original request is still in flight.
	lcrWaitAdvisory(t, e, 1)
	select {
	case <-recovered:
		t.Fatal("recovery escaped the in-flight placement lock")
	default:
	}
	unblock()
	if err := <-placedDone; err == nil {
		t.Fatal("gap hook failed to abort original response")
	}
	result := <-recovered
	if result.status != 200 || result.out["state"] != "created" || len(result.out) != 2 {
		t.Fatalf("concurrent committed recovery: %d %v", result.status, result.out)
	}
	if e.orders() != 1 || e.reserved() != 2 || e.plan.planned() != 0 {
		t.Fatal("concurrent recovery changed committed stock/order/DM")
	}
}

func lcrWaitAdvisory(t *testing.T, e *lbuEnv, want int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var n int
		if err := e.p.f.owner.QueryRow(context.Background(), `SELECT count(*) FROM pg_locks WHERE locktype='advisory' AND NOT granted`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n >= want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("expected recovery/original advisory waiter was never visible")
}

func TestLiveConsoleRecoverySealWinsConcurrentPlacement(t *testing.T) {
	e := lbuNew(t)
	key := t04Key("lcr-seal-first")
	body := e.body(2, nil, "", false)
	tx, err := e.p.f.owner.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(context.Background(), `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "merchanttools.attempt|"+e.tenant()+"|"+e.store()+"|"+key); err != nil {
		t.Fatal(err)
	}
	resolved := make(chan struct {
		status int
		out    map[string]any
	}, 1)
	go func() {
		st, out := lcrRecover(t, e, e.token(), key, body)
		resolved <- struct {
			status int
			out    map[string]any
		}{st, out}
	}()
	lcrWaitAdvisory(t, e, 1) // resolution is first in PG's FIFO advisory wait queue
	placed := make(chan struct {
		status int
		out    map[string]any
	}, 1)
	go func() {
		st, out := e.post(e.token(), key, body)
		placed <- struct {
			status int
			out    map[string]any
		}{st, out}
	}()
	lcrWaitAdvisory(t, e, 2)
	if err = tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	a, b := <-resolved, <-placed
	if a.status != 200 || a.out["state"] != "not_created" || b.status != 409 || b.out["code"] != "attempt_closed" {
		t.Fatalf("seal=%d %v placement=%d %v", a.status, a.out, b.status, b.out)
	}
	if e.orders() != 0 || e.reserved() != 0 || e.plan.planned() != 0 {
		t.Fatal("seal-first race wrote order/stock/DM")
	}
}

func TestLiveConsoleRecoveryMissingCheckoutReceiptFailsClosed(t *testing.T) {
	e := lbuNew(t)
	key := t04Key("lcr-corrupt")
	body := e.body(2, nil, "", false)
	st, out := e.post(e.token(), key, body)
	if st != 201 {
		t.Fatalf("setup %d %v", st, out)
	}
	// Privileged corruption fixture: absence lookup must observe the durable order and refuse to seal.
	mustExec(t, e.p.f.owner, `DELETE FROM checkout.command_results WHERE store_id=$1 AND operation='checkout.begin'`, e.store())
	st, out = lcrRecover(t, e, e.token(), key, body)
	if st != 503 {
		t.Fatalf("inconsistent durable facts must stay UNKNOWN: %d %v", st, out)
	}
	if n := e.count(`SELECT count(*) FROM ops.command_results WHERE store_id=$1 AND operation='merchanttools.order.attempt.recovery'`, e.store()); n != 0 {
		t.Fatal("corrupt receipt was sealed as absent")
	}
}

func TestLiveConsoleRecoveryLegacyUnboundPartialStaysUnknown(t *testing.T) {
	e := lbuNew(t)
	key := t04Key("lcr-legacy")
	body := e.body(2, nil, "", false)
	st, out := e.post(e.token(), key, body)
	if st != 201 {
		t.Fatalf("setup %d %v", st, out)
	}
	// Model legacy pre-amendment process gap: no full A16 receipt or canonical new binding survives.
	mustExec(t, e.p.f.owner, `DELETE FROM ops.command_results WHERE store_id=$1 AND idempotency_key=$2 AND operation IN ('merchanttools.order.for_buyer','merchanttools.order.attempt')`, e.store(), key)
	st, out = lcrRecover(t, e, e.token(), key, e.body(2, nil, "", true))
	if st != 503 {
		t.Fatalf("unverifiable legacy A16 target/send must remain UNKNOWN: %d %v", st, out)
	}
	if n := e.count(`SELECT count(*) FROM ops.command_results WHERE store_id=$1 AND idempotency_key=$2 AND operation IN ('merchanttools.order.attempt','merchanttools.order.attempt.recovery')`, e.store(), key); n != 0 {
		t.Fatal("new body claimed/sealed legacy partial order")
	}
	if e.orders() != 1 || e.reserved() != 2 {
		t.Fatal("legacy recovery changed existing money/stock")
	}
}
