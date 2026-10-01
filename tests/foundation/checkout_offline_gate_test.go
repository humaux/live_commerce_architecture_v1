package foundation_test

// COG01-COG09: independent R4 gate tests for unit checkout-offline (contracts/storefront-v2.md §C, migration 0088 + post_river/0018), written from
// the contract by a test author who is not the implementer. REAL_PG, MOCK tier (no PSP exists on this path). Prefix `cog`. The harness
// helpers (tcvEnv, tcvBuyer) are shared with the author's smoke tests; every expectation below comes from the contract text or from a
// money/security invariant, not from reading the SQL body.
//   COG01 TestCogConfirmExpiryRace      two real sessions on one order row: a due window (expiry owns the stock), 14 near-boundary races,
//                                       concurrent double confirm (same key / distinct keys) -> exactly one ALLOCATE, one audit row
//   COG02 TestCogOfflineRefund          refund only after confirm, never above the confirmed amount (full only), once, concurrent, no PSP row
//   COG03 TestCogIsolation              every transfer route x {other tenant, other store, read-only member, no token}; a buyer from another
//                                       order; bank details only on the buyer's own order capability (leak scan + column privileges)
//   COG04 TestCogStockNumerics          reserved/allocated/on_hand numbers after place/reject/confirm/expire; the oversell gate
//   COG05 TestCogFreeShippingSnapshot   inclusive threshold boundary through real orders; the placed order never re-prices after a policy change
//   COG06 TestCogBuyerEmail             validation, export scope, erasure scope, merchant surfaces never expose it
//   COG07 TestCogFinanceBothColumns     0085 then 0088 apply in order; the finance summary carries pickup + transfer columns together
//   COG08 TestCogTransferACL            exact EXECUTE grantees of every new function, no column grant leaks account details, nothing else can confirm
//   COG09 TestCogNeverAutoConfirms      waiting, proofs of any amount and the expiry worker never confirm; confirmed amount is the server total
// Owner-pool writes (disclosed fixtures): aging/shortening checkout.orders + inventory.reservations expires_at (as bcDue does), identity grants,
// SELECTs for assertions, and a row lock held by an owner transaction to line two sessions up (COG01).

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/fulfillment"
	"livecommerce/internal/platform"
	"livecommerce/internal/reporting"
	"livecommerce/internal/storefront"
)

const (
	cogBank = "SentinelBank"
	cogAcct = "987-654-3210"
)

// cogNew builds the env with a store that offers bank transfer (home and CVS) for `window` hours, saved through the real merchant route.
func cogNew(t *testing.T, window int) *tcvEnv {
	t.Helper()
	e := tcvNew(t)
	e.grantCreator("orders:read", "payments:refund", "fulfillment:write", "orders:export")
	body := fmt.Sprintf(`{"expected_version":0,"enabled":true,"allow_cvs":true,"bank_name":%q,"branch":"Zhongshan","account_name":"Sentinel Shop Ltd","account_number":%q,"window_hours":%d}`, cogBank, cogAcct, window)
	if st, _, raw := e.mcall(e.token(), "PUT", "/v1/admin/stores/"+e.store()+"/bank-transfer-settings", t04Key("cog-set"), body); st != 200 {
		t.Fatalf("settings through the real route: %d %s", st, raw)
	}
	return e
}

func (e *tcvEnv) cogPlace(email string, items ...storefront.Item) (*tcvBuyer, string) {
	e.t.Helper()
	b := e.newBuyer(items...)
	res, err := e.cofPlaceHome(b, email)
	if err != nil {
		e.t.Fatalf("place a bank_transfer order: %v", err)
	}
	return b, res.OrderID
}

func (e *tcvEnv) cogTotal(order string) int64 {
	e.t.Helper()
	var total int64
	if err := e.p.f.owner.QueryRow(context.Background(), `SELECT total_minor FROM checkout.orders WHERE id=$1`, order).Scan(&total); err != nil {
		e.t.Fatal(err)
	}
	return total
}

type cogRow struct {
	commercial, fulfilment, reservation, transfer string
	allocate, release                             int
}

func (e *tcvEnv) cogRow(order string) cogRow {
	e.t.Helper()
	var r cogRow
	if err := e.p.f.owner.QueryRow(context.Background(), `SELECT o.commercial_state,o.fulfillment_state,r.state,t.state,
	  (SELECT count(*) FROM inventory.ledger l WHERE l.reservation_id=o.id AND l.kind='ALLOCATE'),
	  (SELECT count(*) FROM inventory.ledger l WHERE l.reservation_id=o.id AND l.kind='RELEASE')
	  FROM checkout.orders o JOIN inventory.reservations r ON r.id=o.id JOIN checkout.bank_transfers t ON t.order_id=o.id WHERE o.id=$1`, order).
		Scan(&r.commercial, &r.fulfilment, &r.reservation, &r.transfer, &r.allocate, &r.release); err != nil {
		e.t.Fatal(err)
	}
	return r
}

// cogOutcome names the coherent end state of a transfer order and reports an incoherent mix as an error (never both ALLOCATE and RELEASE).
func (r cogRow) cogOutcome() (string, error) {
	switch {
	case r.commercial == "CONFIRMED" && r.reservation == "COMMITTED" && r.transfer == "CONFIRMED" && r.allocate == 1 && r.release == 0:
		return "confirmed", nil
	case r.commercial == "CANCELLED" && r.fulfilment == "CANCELLED" && r.reservation == "EXPIRED" && r.transfer == "EXPIRED" && r.allocate == 0 && r.release == 1:
		return "expired", nil
	}
	return "", fmt.Errorf("incoherent end state: %+v", r)
}

// cogExpireUntilSettled is what the River expiry worker does: call expire_held, and while it answers NOT_DUE wait for the due time it names.
func (e *tcvEnv) cogExpireUntilSettled(order string, within time.Duration) (string, error) {
	deadline := time.Now().Add(within)
	for {
		var disposition string
		var retry *time.Time
		if err := e.p.worker.QueryRow(context.Background(), `SELECT disposition,retry_at FROM checkout.expire_held($1,1)`, order).Scan(&disposition, &retry); err != nil {
			return "", err
		}
		if disposition != "NOT_DUE" || time.Now().After(deadline) {
			return disposition, nil
		}
		wait := 20 * time.Millisecond
		if retry != nil {
			if w := time.Until(*retry) + 10*time.Millisecond; w > wait {
				wait = w
			}
		}
		time.Sleep(wait)
	}
}

// cogHoldLock holds FOR UPDATE on the order row in an owner transaction until release is called.
func (e *tcvEnv) cogHoldLock(order string) (release func()) {
	e.t.Helper()
	ctx := context.Background()
	tx, err := e.p.f.owner.Begin(ctx)
	if err != nil {
		e.t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `SELECT 1 FROM checkout.orders WHERE id=$1 FOR UPDATE`, order); err != nil {
		_ = tx.Rollback(ctx)
		e.t.Fatal(err)
	}
	return func() {
		if err := tx.Commit(ctx); err != nil {
			e.t.Error(err)
		}
	}
}

// cogWaitLockWaiters waits until n sessions of this database are queued on a lock (the two real sessions are then provably both waiting).
func (e *tcvEnv) cogWaitLockWaiters(n int) {
	e.t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if e.count(`SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock'`) >= n {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	e.t.Fatalf("%d sessions never queued on the order row lock", n)
}

// cogShorten makes the hold end in d (order and reservation), keeping created_at (the CHECK is expires_at > created_at).
func (e *tcvEnv) cogShorten(order string, d time.Duration) {
	e.t.Helper()
	ms := d.Milliseconds()
	mustExec(e.t, e.p.f.owner, `UPDATE checkout.orders SET expires_at=clock_timestamp()+make_interval(secs=>$2::double precision/1000) WHERE id=$1`, order, ms)
	mustExec(e.t, e.p.f.owner, `UPDATE inventory.reservations SET expires_at=clock_timestamp()+make_interval(secs=>$2::double precision/1000) WHERE id=$1`, order, ms)
}

type cogDecision struct {
	status int
	out    map[string]any
}

func (e *tcvEnv) cogConfirm(order, key string) cogDecision {
	st, out := e.cofDecide(e.token(), order, "confirm", key, `{}`)
	return cogDecision{st, out}
}

func TestCogConfirmExpiryRace(t *testing.T) {
	e := cogNew(t, 6)
	sku := e.p.stock.skus[0].ID

	// PG grants a row lock to its waiters in arrival order, so queueing the two real sessions one after the other makes each order of the race
	// deterministic: confirm-first and expiry-first on a window that has already ended.
	for _, confirmFirst := range []bool{true, false} {
		name := "a due window, both real sessions queued on the order row, expiry first: the confirm finds a cancelled order"
		if confirmFirst {
			name = "a due window, both real sessions queued on the order row, confirm first: the confirm is refused, the expiry still owns the stock"
		}
		t.Run(name, func(t *testing.T) {
			_, order := e.cogPlace("")
			e.cofAge(order, 7)
			_, reserved0, allocated0 := e.cofBalance(sku)
			release := e.cogHoldLock(order)
			var wg sync.WaitGroup
			var dec cogDecision
			var disposition string
			var expErr error
			startConfirm := func() {
				wg.Add(1)
				go func() { defer wg.Done(); dec = e.cogConfirm(order, t04Key("cog-race-confirm")) }()
			}
			startExpiry := func() {
				wg.Add(1)
				go func() {
					defer wg.Done()
					disposition, expErr = e.cogExpireUntilSettled(order, 2*time.Second)
				}()
			}
			if confirmFirst {
				startConfirm()
				e.cogWaitLockWaiters(1)
				startExpiry()
			} else {
				startExpiry()
				e.cogWaitLockWaiters(1)
				startConfirm()
			}
			e.cogWaitLockWaiters(2)
			release()
			wg.Wait()
			if expErr != nil || disposition != "EXPIRED" {
				t.Errorf("expiry session: %q %v", disposition, expErr)
			}
			if c := cofCode(dec.out); dec.status != 409 || (c != "transfer_window_closed" && c != "transfer_not_open") {
				t.Errorf("confirm session after the deadline: %d %v (a due order can no longer be confirmed)", dec.status, dec.out)
			}
			if o, err := e.cogRow(order).cogOutcome(); err != nil || o != "expired" {
				t.Errorf("end state: %q %v", o, err)
			}
			if _, r1, a1 := e.cofBalance(sku); r1 != reserved0-2 || a1 != allocated0 {
				t.Errorf("stock: reserved %d -> %d (want -2), allocated %d -> %d (want unchanged)", reserved0, r1, allocated0, a1)
			}
		})
	}

	t.Run("14 near-boundary races: whichever session wins, the order ends in exactly one coherent state and the stock adds up", func(t *testing.T) {
		const n = 14
		_, reserved0, allocated0 := e.cofBalance(sku)
		orders := make([]string, n)
		for i := range orders {
			_, orders[i] = e.cogPlace("")
		}
		_, reservedHeld, _ := e.cofBalance(sku)
		if reservedHeld != reserved0+2*n {
			t.Fatalf("reserved after %d placements: %d -> %d", n, reserved0, reservedHeld)
		}
		var wg sync.WaitGroup
		decisions := make([]cogDecision, n)
		dispositions := make([]string, n)
		errs := make([]error, n)
		for i, order := range orders {
			e.cogShorten(order, time.Duration(120+(i*41)%260)*time.Millisecond) // 120..380 ms, spread across the confirm round trip
		}
		start := make(chan struct{})
		for i, order := range orders {
			wg.Add(2)
			go func() { defer wg.Done(); <-start; decisions[i] = e.cogConfirm(order, t04Key("cog-near")) }()
			go func() {
				defer wg.Done()
				<-start
				dispositions[i], errs[i] = e.cogExpireUntilSettled(order, 5*time.Second)
			}()
		}
		close(start)
		wg.Wait()
		confirmed, expired := 0, 0
		for i, order := range orders {
			if errs[i] != nil {
				t.Errorf("order %d expire_held: %v", i, errs[i])
			}
			outcome, err := e.cogRow(order).cogOutcome()
			if err != nil {
				t.Errorf("order %d: %v (confirm %d %v, expiry %s)", i, err, decisions[i].status, decisions[i].out, dispositions[i])
				continue
			}
			switch outcome {
			case "confirmed":
				confirmed++
				if decisions[i].status != 200 {
					t.Errorf("order %d is CONFIRMED but the confirm call answered %d %v", i, decisions[i].status, decisions[i].out)
				}
				if dispositions[i] == "EXPIRED" {
					t.Errorf("order %d is CONFIRMED yet the expiry reports EXPIRED", i)
				}
			case "expired":
				expired++
				if decisions[i].status == 200 {
					t.Errorf("order %d is CANCELLED yet the confirm call answered 200", i)
				}
				if c := cofCode(decisions[i].out); c != "transfer_window_closed" && c != "transfer_not_open" {
					t.Errorf("order %d lost the confirm with %d %q", i, decisions[i].status, c)
				}
			}
		}
		_, reserved1, allocated1 := e.cofBalance(sku)
		if reserved1 != reserved0 || allocated1 != allocated0+int64(2*confirmed) {
			t.Errorf("stock after %d confirmed / %d expired: reserved %d -> %d (want unchanged), allocated %d -> %d (want +%d)", confirmed, expired, reserved0, reserved1, allocated0, allocated1, 2*confirmed)
		}
		t.Logf("near-boundary race outcomes: %d confirmed, %d expired (both are valid; the invariants above are the gate)", confirmed, expired)
	})

	t.Run("six concurrent confirms (three share a key, three differ): exactly one commits the stock and one audit row is written", func(t *testing.T) {
		_, order := e.cogPlace("")
		_, reserved0, allocated0 := e.cofBalance(sku)
		audit0 := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='checkout.bank_transfer_confirmed'`, e.store())
		release := e.cogHoldLock(order)
		same := t04Key("cog-same")
		keys := []string{same, same, same, t04Key("cog-d1"), t04Key("cog-d2"), t04Key("cog-d3")}
		out := make([]cogDecision, len(keys))
		var wg sync.WaitGroup
		for i, k := range keys {
			wg.Add(1)
			go func() { defer wg.Done(); out[i] = e.cogConfirm(order, k) }()
		}
		e.cogWaitLockWaiters(len(keys))
		release()
		wg.Wait()
		winners := map[string]bool{}
		for i, d := range out {
			switch {
			case d.status == 200 && d.out["state"] == "CONFIRMED":
				winners[keys[i]] = true
			case d.status == 409 && cofCode(d.out) == "already_confirmed":
			default:
				t.Errorf("confirm %d (key %s): %d %v", i, keys[i], d.status, d.out)
			}
		}
		if len(winners) != 1 {
			t.Errorf("%d distinct keys got a 200; exactly one decision may win: %v", len(winners), winners)
		}
		for i, k := range keys[:3] {
			if out[i].status == 200 && !winners[k] {
				t.Errorf("a replay of key %s answered 200 without being the winner", k)
			}
		}
		if o, err := e.cogRow(order).cogOutcome(); err != nil || o != "confirmed" {
			t.Errorf("end state %q %v", o, err)
		}
		if _, r1, a1 := e.cofBalance(sku); r1 != reserved0-2 || a1 != allocated0+2 {
			t.Errorf("stock committed once: reserved %d -> %d, allocated %d -> %d", reserved0, r1, allocated0, a1)
		}
		if n := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='checkout.bank_transfer_confirmed'`, e.store()); n != audit0+1 {
			t.Errorf("confirm audit rows %d -> %d, want +1", audit0, n)
		}
	})
}

func TestCogOfflineRefund(t *testing.T) {
	e := cogNew(t, 72)

	t.Run("refund needs a CONFIRMED transfer: never from AWAITING, SUBMITTED, REJECTED or EXPIRED", func(t *testing.T) {
		b, order := e.cogPlace("")
		refund := func(label string) {
			if st, out := e.cofDecide(e.token(), order, "refund-offline", t04Key("cog-ref"), `{}`); st == 200 || cofCode(out) != "transfer_not_confirmed" && cofCode(out) != "transfer_not_open" {
				t.Errorf("%s: refund answered %d %v, want a 4xx refusal", label, st, out)
			}
		}
		refund("awaiting")
		b.cofProof(order, t04Key("cog-ref-p"), "12345", b.h.quote.Amount.TotalMinor)
		refund("submitted")
		e.cofDecide(e.token(), order, "reject", t04Key("cog-ref-r"), `{"reason":"no match"}`)
		refund("rejected")
		e.cofAge(order, 80)
		if d, _ := e.cofExpire(order); d != "EXPIRED" {
			t.Fatalf("expire: %s", d)
		}
		refund("expired")
		if n := e.count(`SELECT count(*) FROM checkout.bank_transfers WHERE order_id=$1 AND state='EXPIRED' AND refunded_at IS NULL`, order); n != 1 {
			t.Error("a refused refund changed the transfer row")
		}
	})

	b, order := e.cogPlace("")
	total := e.cogTotal(order)
	b.cofProof(order, t04Key("cog-ref2-p"), "12345", total)
	if d := e.cogConfirm(order, t04Key("cog-ref2-c")); d.status != 200 {
		t.Fatalf("confirm: %d %v", d.status, d.out)
	}
	ledger0 := e.count(`SELECT count(*) FROM inventory.ledger WHERE reservation_id=$1`, order)
	facts0 := e.count(`SELECT count(*) FROM payments.facts WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store())

	t.Run("the refund is the whole confirmed amount: a body carrying an amount is refused, nothing changes", func(t *testing.T) {
		for _, body := range []string{`{"amount_minor":1}`, fmt.Sprintf(`{"amount_minor":%d}`, total+1), `{"refund_minor":1}`} {
			if st, _ := e.cofDecide(e.token(), order, "refund-offline", t04Key("cog-ref3"), body); st != 400 && st != 422 {
				t.Errorf("refund body %s: %d, want 400/422 (there is no partial or larger offline refund)", body, st)
			}
		}
		if n := e.count(`SELECT count(*) FROM checkout.bank_transfers WHERE order_id=$1 AND state='CONFIRMED'`, order); n != 1 {
			t.Error("a refused refund moved the transfer")
		}
	})

	t.Run("two concurrent refunds with different keys: exactly one is recorded and audited", func(t *testing.T) {
		audit0 := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='checkout.bank_transfer_refunded_offline'`, e.store())
		release := e.cogHoldLock(order)
		out := make([]cogDecision, 2)
		var wg sync.WaitGroup
		for i := range out {
			wg.Add(1)
			go func() {
				defer wg.Done()
				st, o := e.cofDecide(e.token(), order, "refund-offline", t04Key("cog-ref-par"), `{}`)
				out[i] = cogDecision{st, o}
			}()
		}
		e.cogWaitLockWaiters(2)
		release()
		wg.Wait()
		ok, refused := 0, 0
		for _, d := range out {
			if d.status == 200 && d.out["state"] == "REFUNDED_OFFLINE" {
				ok++
			} else if d.status == 409 && cofCode(d.out) == "already_refunded" {
				refused++
			}
		}
		if ok != 1 || refused != 1 {
			t.Errorf("results %+v: want one 200 and one 409 already_refunded", out)
		}
		if n := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='checkout.bank_transfer_refunded_offline'`, e.store()); n != audit0+1 {
			t.Errorf("refund audit rows %d -> %d, want +1", audit0, n)
		}
	})

	t.Run("after the refund: no PSP money object, the confirmed fact is kept at the server total, every other door is shut", func(t *testing.T) {
		var amount int64
		var by, refundedBy *string
		if err := e.p.f.owner.QueryRow(context.Background(), `SELECT confirmed_amount_minor,confirmed_by::text,refunded_by::text FROM checkout.bank_transfers WHERE order_id=$1 AND state='REFUNDED_OFFLINE'`, order).Scan(&amount, &by, &refundedBy); err != nil {
			t.Fatal(err)
		}
		if amount != total || by == nil || refundedBy == nil {
			t.Errorf("confirmed %d (server total %d) by %v refunded by %v", amount, total, by, refundedBy)
		}
		if n := e.count(`SELECT count(*) FROM payments.stripe_refunds WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store()); n != 0 {
			t.Errorf("%d PSP refund rows for an offline refund", n)
		}
		if n := e.count(`SELECT count(*) FROM payments.facts WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store()); n != facts0 {
			t.Errorf("payments.facts %d -> %d: the offline path never writes a PSP fact", facts0, n)
		}
		if n := e.count(`SELECT count(*) FROM checkout.payment_attempts WHERE order_id=$1`, order); n != 0 {
			t.Error("a payment attempt exists for a transfer order")
		}
		if n := e.count(`SELECT count(*) FROM inventory.ledger WHERE reservation_id=$1`, order); n != ledger0 {
			t.Errorf("ledger rows %d -> %d: the offline refund moves no stock", ledger0, n)
		}
		if d := e.cogConfirm(order, t04Key("cog-ref-c2")); d.status != 409 {
			t.Errorf("confirm after refund: %d %v", d.status, d.out)
		}
		if st, _ := e.cofDecide(e.token(), order, "reject", t04Key("cog-ref-r2"), `{"reason":"late"}`); st != 409 {
			t.Errorf("reject after refund: %d", st)
		}
		if p := b.cofProof(order, t04Key("cog-ref-p2"), "55555", 1000); p.status != 422 {
			t.Errorf("proof after refund: %d %s", p.status, p.body)
		}
		if v := cofJSON(t, b.cofView(order).body); v["state"] != "REFUNDED_OFFLINE" {
			t.Errorf("the buyer sees %v", v["state"])
		}
	})
}

func TestCogIsolation(t *testing.T) {
	e := cogNew(t, 72)
	f := e.p.f
	b, order := e.cogPlace("")
	b.cofProof(order, t04Key("cog-iso-p"), "12345", b.h.quote.Amount.TotalMinor)
	allPerms := []string{"store:read", "orders:read", "payments:refund", "integration:read", "integration:manage"}
	_, tokenB := lcPrincipal(t, f, f.tenantB, []string{f.storeB}, allPerms...) // another tenant, every permission on ITS store
	storeA2 := randomUUID()                                                    // a second store of the same tenant
	mustExec(t, f.owner, `INSERT INTO control.stores(tenant_id,id,name,currency) VALUES($1,$2,'second store','TWD')`, e.tenant(), storeA2)
	_, tokenA2 := lcPrincipal(t, f, f.tenantA, []string{storeA2}, allPerms...) // same tenant, that other store only
	readOnly, _ := e.member("orders:read")                                     // right store, too few permissions for the writes
	settingsBefore := e.count(`SELECT version FROM checkout.bank_transfer_settings WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store())
	auditBefore := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action LIKE 'checkout.bank_transfer_%'`, e.store())

	type route struct{ method, path, body string }
	routes := func(store string) []route {
		base := "/v1/admin/stores/" + store
		o := base + "/orders/" + order + "/bank-transfer"
		return []route{
			{"GET", base + "/bank-transfer-settings", ""},
			{"PUT", base + "/bank-transfer-settings", `{"expected_version":1,"enabled":false,"allow_cvs":false,"bank_name":"","branch":"","account_name":"","account_number":"","window_hours":72}`},
			{"GET", o, ""},
			{"POST", o + "/confirm", `{}`},
			{"POST", o + "/reject", `{"reason":"intruder"}`},
			{"POST", o + "/refund-offline", `{}`},
		}
	}
	isWrite := func(r route) bool { return r.method != "GET" }
	var leaked []string
	scan := func(label string, raw []byte) {
		if s := string(raw); strings.Contains(s, cogAcct) || strings.Contains(s, cogBank) {
			leaked = append(leaked, label)
		}
	}
	for _, c := range []struct {
		name, token, store string
	}{
		{"other tenant on this store's path", tokenB, e.store()},
		{"other tenant on its own store's path with this order id", tokenB, f.storeB},
		{"same tenant, other store's grants, this store's path", tokenA2, e.store()},
		{"same tenant, other store's grants, its own store's path with this order id", tokenA2, storeA2},
	} {
		for _, r := range routes(c.store) {
			if c.store == f.storeB || c.store == storeA2 {
				// the settings routes of the caller's OWN store are legitimate (they never carry this store's data); the order routes are not
				if strings.Contains(r.path, "bank-transfer-settings") {
					st, _, raw := e.mcall(c.token, r.method, r.path, t04Key("cog-iso"), r.body)
					scan(c.name+" "+r.method+" "+r.path, raw)
					_ = st
					continue
				}
			}
			st, _, raw := e.mcall(c.token, r.method, r.path, t04Key("cog-iso"), r.body)
			scan(c.name+" "+r.method+" "+r.path, raw)
			if st != 403 && st != 404 {
				t.Errorf("%s: %s %s answered %d, want 403/404", c.name, r.method, r.path, st)
			}
		}
	}
	for _, r := range routes(e.store()) {
		st, _, raw := e.mcall("", r.method, r.path, t04Key("cog-iso"), r.body)
		scan("no token "+r.path, raw)
		if st != 401 {
			t.Errorf("no token: %s %s answered %d, want 401", r.method, r.path, st)
		}
		if isWrite(r) {
			st, _, raw = e.mcall(readOnly, r.method, r.path, t04Key("cog-iso"), r.body)
			scan("read-only member "+r.path, raw)
			if st != 403 {
				t.Errorf("read-only member: %s %s answered %d, want 403", r.method, r.path, st)
			}
		}
	}
	if len(leaked) > 0 {
		t.Errorf("bank details appeared in refused responses: %v", leaked)
	}
	if n := e.count(`SELECT version FROM checkout.bank_transfer_settings WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store()); n != settingsBefore {
		t.Errorf("the store's bank settings changed version %d -> %d under foreign callers", settingsBefore, n)
	}
	if n := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action LIKE 'checkout.bank_transfer_%'`, e.store()); n != auditBefore {
		t.Errorf("audit rows %d -> %d: a refused call wrote an audit row", auditBefore, n)
	}
	if r := e.cogRow(order); r.commercial != "AWAITING_TRANSFER" || r.transfer != "SUBMITTED" || r.allocate != 0 || r.release != 0 {
		t.Errorf("the order changed under refused calls: %+v", r)
	}

	t.Run("a buyer from another order, and a buyer capability of another store, reach nothing", func(t *testing.T) {
		other, otherOrder := e.cogPlace("")
		other.cofProof(otherOrder, t04Key("cog-iso-op"), "77777", 1000)
		proofCount := e.count(`SELECT proof_count FROM checkout.bank_transfers WHERE order_id=$1`, order)
		if v := other.cofView(order); v.status != 404 {
			t.Errorf("buyer of order 2 reads order 1's bank view: %d %s", v.status, v.body)
		} else {
			scan("other buyer view", v.body)
		}
		if p := other.cofProof(order, t04Key("cog-iso-xp"), "99999", 1); p.status != 404 {
			t.Errorf("buyer of order 2 submits a proof on order 1: %d %s", p.status, p.body)
		}
		if n := e.count(`SELECT proof_count FROM checkout.bank_transfers WHERE order_id=$1`, order); n != proofCount {
			t.Error("a foreign proof changed the proof count")
		}
		if n := e.count(`SELECT count(*) FROM checkout.bank_transfers WHERE order_id=$1 AND proof_last5='12345'`, order); n != 1 {
			t.Error("a foreign proof overwrote the buyer's own proof")
		}
		foreign := mustIssue(t, e.p.cqHarness.service, f.storeB)
		useForeign := func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+foreign.Token) }
		for _, call := range []struct{ method, path string }{{"GET", "/v1/buyer/orders/" + order + "/bank-transfer"}, {"GET", "/v1/buyer/orders/" + order}} {
			res := b.req(call.method, call.path, "", nil, useForeign)
			scan("foreign-store capability "+call.path, res.body)
			if res.status == 200 {
				t.Errorf("a capability of another store read %s: %d", call.path, res.status)
			}
		}
		if res := b.req("PUT", "/v1/buyer/orders/"+order+"/bank-transfer/proof", t04Key("cog-iso-fp"), map[string]any{"last5": "11111", "amount_minor": 1, "paid_at": time.Now().UTC().Format(time.RFC3339)}, useForeign); res.status == 200 {
			t.Errorf("a capability of another store submitted a proof: %d", res.status)
		}
		if len(leaked) > 0 {
			t.Errorf("bank details leaked to a foreign buyer: %v", leaked)
		}
	})

	t.Run("bank details appear only on the buyer's own bank-transfer view (leak scan of every other buyer and merchant read)", func(t *testing.T) {
		own := b.cofView(order)
		if own.status != 200 || !strings.Contains(string(own.body), cogAcct) || !strings.Contains(string(own.body), cogBank) {
			t.Fatalf("positive control: the buyer's own view must show the details: %d %s", own.status, own.body)
		}
		var hits []string
		check := func(label string, status int, raw []byte) {
			if status == 0 {
				return
			}
			if s := string(raw); strings.Contains(s, cogAcct) || strings.Contains(s, cogBank) || strings.Contains(s, "Sentinel Shop Ltd") {
				hits = append(hits, label)
			}
		}
		opts := b.req("GET", "/v1/buyer/checkout-options?market_id="+e.p.market.ID+"&country=TW", "", nil, nil)
		check("buyer options", opts.status, opts.body)
		ord := b.req("GET", "/v1/buyer/orders/"+order, "", nil, nil)
		check("buyer order", ord.status, ord.body)
		proof := b.cofProof(order, t04Key("cog-leak-p"), "12345", b.h.quote.Amount.TotalMinor)
		check("buyer proof response", proof.status, proof.body)
		for _, p := range []string{"/orders/" + order, "/orders?limit=50", "/finance/summary?from=2020-01-01&to=2020-01-10"} {
			st, _, raw := e.mcall(e.token(), "GET", "/v1/admin/stores/"+e.store()+p, "", "")
			check("merchant "+p, st, raw)
		}
		// the merchant's own transfer detail is the other legitimate place: positive control, so the scan above is not vacuous
		if st, _, raw := e.mcall(e.token(), "GET", "/v1/admin/stores/"+e.store()+"/orders/"+order+"/bank-transfer", "", ""); st != 200 || !strings.Contains(string(raw), cogAcct) {
			t.Errorf("positive control, merchant transfer view: %d %s", st, raw)
		}
		if len(hits) > 0 {
			t.Errorf("bank details leaked through %v", hits)
		}
	})

	t.Run("column privileges: no application role can read the bank details or write the transfer state except the owning writer", func(t *testing.T) {
		rows, err := f.owner.Query(context.Background(), `SELECT r.rolname FROM pg_roles r WHERE r.rolname LIKE 'commerce\_%' AND NOT r.rolsuper
		  AND r.rolname<>(SELECT tableowner FROM pg_tables WHERE schemaname='checkout' AND tablename='bank_transfers')`)
		if err != nil {
			t.Fatal(err)
		}
		var roles []string
		for rows.Next() {
			var r string
			if err := rows.Scan(&r); err != nil {
				t.Fatal(err)
			}
			roles = append(roles, r)
		}
		rows.Close()
		if len(roles) < 5 {
			t.Fatalf("only %d application roles found: the probe would be vacuous", len(roles))
		}
		for _, role := range roles {
			for _, col := range []string{"account_number", "bank_name", "branch", "account_name", "proof_last5", "proof_amount_minor", "reject_reason"} {
				if e.count(`SELECT CASE WHEN has_column_privilege($1,'checkout.bank_transfers',$2,'SELECT') THEN 1 ELSE 0 END`, role, col) == 1 {
					t.Errorf("role %s can SELECT checkout.bank_transfers.%s", role, col)
				}
			}
			for _, col := range []string{"state", "confirmed_at", "confirmed_by", "confirmed_amount_minor", "account_number"} {
				for _, priv := range []string{"UPDATE", "INSERT"} {
					if e.count(`SELECT CASE WHEN has_column_privilege($1,'checkout.bank_transfers',$2,$3) THEN 1 ELSE 0 END`, role, col, priv) == 1 && role != "commerce_checkout_writer" {
						t.Errorf("role %s can %s checkout.bank_transfers.%s", role, priv, col)
					}
				}
			}
			if e.count(`SELECT CASE WHEN has_table_privilege($1,'checkout.bank_transfer_settings','SELECT') THEN 1 ELSE 0 END`, role) == 1 && role != "commerce_checkout_writer" {
				t.Errorf("role %s can SELECT the settings table directly", role)
			}
		}
	})
}

func TestCogStockNumerics(t *testing.T) {
	e := cogNew(t, 72)
	sku := e.p.stock.skus[0].ID
	type bal struct{ onHand, reserved, allocated int64 }
	read := func() bal {
		o, r, a := e.cofBalance(sku)
		return bal{o, r, a}
	}
	expect := func(label string, got, want bal) {
		t.Helper()
		if got != want {
			t.Errorf("%s: on_hand/reserved/allocated = %+v, want %+v", label, got, want)
		}
	}
	// newBuyer tops the SKU up by 200 on every call; take the baseline after both buyers exist so on_hand is constant from here on
	b1, b2 := e.newBuyer(), e.newBuyer()
	base := read()
	res1, err := e.cofPlaceHome(b1, "")
	if err != nil {
		t.Fatal(err)
	}
	expect("after placing order 1 (2 units)", read(), bal{base.onHand, base.reserved + 2, base.allocated})
	res2, err := e.cofPlaceHome(b2, "")
	if err != nil {
		t.Fatal(err)
	}
	expect("after placing order 2", read(), bal{base.onHand, base.reserved + 4, base.allocated})
	b1.cofProof(res1.OrderID, t04Key("cog-st-p"), "12345", b1.h.quote.Amount.TotalMinor)
	expect("after the buyer's proof (a proof never moves stock)", read(), bal{base.onHand, base.reserved + 4, base.allocated})
	if st, _ := e.cofDecide(e.token(), res1.OrderID, "reject", t04Key("cog-st-r"), `{"reason":"amount mismatch"}`); st != 200 {
		t.Fatalf("reject: %d", st)
	}
	expect("after the merchant rejects the submission (the order and its hold stay)", read(), bal{base.onHand, base.reserved + 4, base.allocated})
	if d := e.cogConfirm(res1.OrderID, t04Key("cog-st-c")); d.status != 200 {
		t.Fatalf("confirm: %d %v", d.status, d.out)
	}
	expect("after confirming order 1: 2 units move reserved -> allocated, on_hand untouched", read(), bal{base.onHand, base.reserved + 2, base.allocated + 2})
	e.cofAge(res2.OrderID, 80)
	if d, _ := e.cofExpire(res2.OrderID); d != "EXPIRED" {
		t.Fatalf("expire: %s", d)
	}
	expect("after order 2 expires: its 2 units are released, order 1's stay allocated", read(), bal{base.onHand, base.reserved, base.allocated + 2})
	if _, err = e.cofPlaceHome(b2, ""); err == nil {
		t.Log("note: the expired cart version can be placed again (not asserted)")
	}

	t.Run("oversell gate: a SKU with exactly 2 units serves one transfer hold; the second buyer is refused until the first hold ends", func(t *testing.T) {
		scarce := e.sku(1250, 2)
		item := storefront.Item{SKUID: scarce, Quantity: 2}
		first := e.newBuyer(item)
		r1, err := e.cofPlaceHome(first, "")
		if err != nil {
			t.Fatalf("first buyer: %v", err)
		}
		second := e.newBuyer(item)
		before := e.count(`SELECT count(*) FROM checkout.orders WHERE owner_id=$1`, second.cap.Scope.OwnerID)
		if _, err = e.cofPlaceHome(second, ""); err == nil {
			t.Fatal("the second buyer got a hold on stock that is already held for a transfer: oversell")
		}
		if n := e.count(`SELECT count(*) FROM checkout.orders WHERE owner_id=$1`, second.cap.Scope.OwnerID); n != before {
			t.Error("the refused placement left an order")
		}
		e.cofAge(r1.OrderID, 80)
		if d, _ := e.cofExpire(r1.OrderID); d != "EXPIRED" {
			t.Fatalf("expire: %s", d)
		}
		third := e.newBuyer(item)
		if _, err = e.cofPlaceHome(third, ""); err != nil {
			t.Errorf("after the first hold expired the stock must be sellable again: %v", err)
		}
	})
}

func TestCogFreeShippingSnapshot(t *testing.T) {
	e := cogNew(t, 72)
	code, _, _ := e.service("cvs_711", "MANUAL", 6000)
	setThreshold := func(th *int64) {
		t.Helper()
		var version int64
		if err := e.p.f.owner.QueryRow(context.Background(), `SELECT current_version FROM pricing.policy_heads WHERE tenant_id=$1 AND store_id=$2 AND method=$3`, e.tenant(), e.store(), "delivery:"+code).Scan(&version); err != nil {
			t.Fatal(err)
		}
		p := e.p.policy
		fee := int64(6000)
		p.Method, p.ExpectedVersion, p.ShippingMinor, p.Enabled, p.FreeShippingThresholdMinor = "delivery:"+code, version, &fee, true, th
		saved, err := e.p.setPolicy(p)
		if err != nil {
			t.Fatalf("set policy: %v", err)
		}
		// the delivery service pins a policy version (Begin refuses a quote of another one): the merchant UI saves the policy and then the service
		e.updateService(code, func(in *fulfillment.ServiceInput) { in.PolicyVersion = saved.Version })
	}
	// a pickup placement helper that returns the order and its quote (the buyer's quote is what Begin compares)
	prepare := func() (*tcvBuyer, storefront.Destination, storefront.Quote) {
		t.Helper()
		b := e.newBuyer()
		dest, err := b.destination("cvs_711", e.tppEntered(b, code), cofName, cofPhone)
		if err != nil {
			t.Fatal(err)
		}
		q, err := b.quote(code)
		if err != nil {
			t.Fatal(err)
		}
		return b, dest, q
	}
	_, _, probe := prepare()
	subtotal := probe.Amount.SubtotalMinor

	place := func(label string, th *int64, mode string) (string, int64) {
		t.Helper()
		setThreshold(th)
		b, dest, q := prepare()
		res, err := b.begin(dest, q, e.svcVer[code], mode)
		if err != nil {
			t.Fatalf("%s: Begin: %v", label, err)
		}
		return res.OrderID, e.cogTotal(res.OrderID)
	}
	boundary := func(delta int64) *int64 { v := subtotal + delta; return &v }

	t.Run("the inclusive boundary on real orders, for card and for bank transfer alike", func(t *testing.T) {
		for _, mode := range []string{"bank_transfer", ""} {
			if _, total := place("no threshold", nil, mode); total != subtotal+6000 {
				t.Errorf("%q no threshold: total %d, want subtotal+6000 = %d", mode, total, subtotal+6000)
			}
			if _, total := place("threshold = subtotal", boundary(0), mode); total != subtotal {
				t.Errorf("%q threshold == subtotal must ship free (>= is inclusive): total %d want %d", mode, total, subtotal)
			}
			if _, total := place("threshold = subtotal+1", boundary(1), mode); total != subtotal+6000 {
				t.Errorf("%q threshold one above the subtotal keeps the fee: total %d want %d", mode, total, subtotal+6000)
			}
			if _, total := place("threshold = subtotal-1", boundary(-1), mode); total != subtotal {
				t.Errorf("%q threshold below the subtotal is free: total %d want %d", mode, total, subtotal)
			}
		}
	})

	t.Run("a placed order is a frozen snapshot: raising the threshold afterwards neither re-prices it nor changes what a confirm records", func(t *testing.T) {
		order, total := place("free at the boundary", boundary(0), "bank_transfer")
		if total != subtotal {
			t.Fatalf("setup: total %d", total)
		}
		setThreshold(boundary(1)) // the policy now charges shipping for the same cart
		if got := e.cogTotal(order); got != subtotal {
			t.Errorf("order total moved to %d after the policy change", got)
		}
		var shipping int64
		if err := e.p.f.owner.QueryRow(context.Background(), `SELECT (snapshot->'quote'->'amount'->>'shipping_minor')::bigint FROM checkout.orders WHERE id=$1`, order).Scan(&shipping); err == nil && shipping != 0 {
			t.Errorf("the snapshot's shipping is %d, want 0", shipping)
		}
		if d := e.cogConfirm(order, t04Key("cog-fs-c")); d.status != 200 {
			t.Fatalf("confirm: %d %v", d.status, d.out)
		}
		var confirmed int64
		if err := e.p.f.owner.QueryRow(context.Background(), `SELECT confirmed_amount_minor FROM checkout.bank_transfers WHERE order_id=$1`, order).Scan(&confirmed); err != nil || confirmed != subtotal {
			t.Errorf("the confirmed amount is %d (%v), want the frozen order total %d", confirmed, err, subtotal)
		}
	})

	t.Run("a quote taken before a policy change cannot be placed after it (no silent re-pricing), and nothing is held", func(t *testing.T) {
		setThreshold(boundary(0))
		b, dest, q := prepare()
		if q.Amount.ShippingMinor != 0 {
			t.Fatalf("setup: the quote at the boundary charges %d shipping", q.Amount.ShippingMinor)
		}
		setThreshold(boundary(1))
		reserved0 := e.count(`SELECT count(*) FROM checkout.orders WHERE owner_id=$1`, b.cap.Scope.OwnerID)
		if res, err := b.begin(dest, q, e.svcVer[code], "bank_transfer"); err == nil {
			t.Errorf("a stale quote (shipping 0) was accepted after the threshold rose: order %s total %d", res.OrderID, e.cogTotal(res.OrderID))
		}
		if n := e.count(`SELECT count(*) FROM checkout.orders WHERE owner_id=$1`, b.cap.Scope.OwnerID); n != reserved0 {
			t.Error("the refused Begin left an order")
		}
	})

	t.Run("a null threshold means no free shipping", func(t *testing.T) {
		setThreshold(nil)
		if q := func() storefront.Quote { _, _, q := prepare(); return q }(); q.Amount.ShippingMinor != 6000 || q.Policy.FreeShippingThresholdMinor != nil {
			t.Errorf("null threshold means no free shipping: %+v %v", q.Amount, q.Policy.FreeShippingThresholdMinor)
		}
	})
}

func TestCogBuyerEmail(t *testing.T) {
	e := cogNew(t, 72)
	t.Run("validation: malformed, injected or over-long addresses are refused before any order exists; blank means none", func(t *testing.T) {
		for _, bad := range []string{"not an email", "a b@c.com", "a@b.com,c@d.com", "a@b.com\r\nBcc: x@y.com", "@b.com", "a@", "a@@b.com", "<a@b.com>", strings.Repeat("a", 250) + "@b.com"} {
			b := e.newBuyer()
			if _, err := e.cofPlaceHome(b, bad); err == nil {
				t.Errorf("email %q was accepted", bad)
			}
			if n := e.count(`SELECT count(*) FROM checkout.orders WHERE owner_id=$1`, b.cap.Scope.OwnerID); n != 0 {
				t.Errorf("email %q: the refused Begin left an order", bad)
			}
		}
		b := e.newBuyer()
		res, err := e.cofPlaceHome(b, "")
		if err != nil {
			t.Fatal(err)
		}
		if n := e.count(`SELECT count(*) FROM checkout.orders WHERE id=$1 AND buyer_email IS NULL`, res.OrderID); n != 1 {
			t.Error("a blank email must be stored as NULL, not as an empty string")
		}
	})

	t.Run("the email is captured for a card order too, and is exported only to its own buyer", func(t *testing.T) {
		bx, ox := e.cogPlace("x.buyer@example.com")
		_, oy := e.cogPlace("y.buyer@example.com")
		card := e.newBuyer()
		in := card.h.input
		in.BuyerEmail = "card.buyer@example.com"
		cres, err := e.svc.Begin(context.Background(), card.cap.Token, e.store(), t04Key("cog-card"), in)
		if err != nil {
			t.Fatal(err)
		}
		if n := e.count(`SELECT count(*) FROM checkout.orders WHERE id=$1 AND buyer_email='card.buyer@example.com'`, cres.OrderID); n != 1 {
			t.Error("storefront-v2 §C: the optional email is captured at checkout (every payment mode), not only for transfers")
		}
		export := bx.req("POST", "/v1/buyer/privacy/export", t04Key("cog-exp"), nil, nil)
		if export.status != 200 || !strings.Contains(string(export.body), "x.buyer@example.com") {
			t.Fatalf("the export lacks the buyer's own email: %d %s", export.status, export.body)
		}
		for _, other := range []string{"y.buyer@example.com", "card.buyer@example.com"} {
			if strings.Contains(string(export.body), other) {
				t.Errorf("buyer X's export contains %s of another buyer", other)
			}
		}
		// merchant surfaces never carry the address (it is for notifications, which this release does not send)
		for _, p := range []string{"/orders/" + ox, "/orders?limit=50", "/orders/" + ox + "/bank-transfer"} {
			if st, _, raw := e.mcall(e.token(), "GET", "/v1/admin/stores/"+e.store()+p, "", ""); st == 200 && strings.Contains(string(raw), "buyer@example.com") {
				t.Errorf("merchant read %s exposes a buyer email", p)
			}
		}
		// hold blocks erasure; after the merchant confirms, erasure runs and clears X's email only
		blocked := bx.req("POST", "/v1/buyer/privacy/erasure", t04Key("cog-erase"), map[string]any{"confirm": "ERASE"}, nil)
		if blocked.status != 409 || cofCode(cofJSON(t, blocked.body)) != "erasure_blocked" {
			t.Errorf("an open transfer hold blocks erasure: %d %s", blocked.status, blocked.body)
		}
		if n := e.count(`SELECT count(*) FROM checkout.orders WHERE id=$1 AND buyer_email='x.buyer@example.com'`, ox); n != 1 {
			t.Error("a blocked erasure still cleared the email")
		}
		bx.cofProof(ox, t04Key("cog-erase-p"), "12345", e.cogTotal(ox))
		if d := e.cogConfirm(ox, t04Key("cog-erase-c")); d.status != 200 {
			t.Fatalf("confirm: %d %v", d.status, d.out)
		}
		erased := bx.req("POST", "/v1/buyer/privacy/erasure", t04Key("cog-erase2"), map[string]any{"confirm": "ERASE"}, nil)
		if erased.status != 200 {
			t.Fatalf("erasure after the transfer was confirmed: %d %s", erased.status, erased.body)
		}
		if n := e.count(`SELECT count(*) FROM checkout.orders WHERE id=$1 AND buyer_email IS NULL`, ox); n != 1 {
			t.Error("erasure left X's email on the order")
		}
		if n := e.count(`SELECT count(*) FROM checkout.orders WHERE id=$1 AND buyer_email='y.buyer@example.com'`, oy); n != 1 {
			t.Error("erasing buyer X cleared buyer Y's email")
		}
		if n := e.count(`SELECT count(*) FROM checkout.bank_transfers WHERE order_id=$1 AND state='CONFIRMED' AND confirmed_amount_minor=$2`, ox, e.cogTotal(ox)); n != 1 {
			t.Error("erasure damaged the confirmed money fact")
		}
	})
}

func TestCogFinanceBothColumns(t *testing.T) {
	e := cogNew(t, 72)
	f := e.p.f
	t.Run("migrations 0085 then 0088 are both applied, in order, and the live function carries both column pairs", func(t *testing.T) {
		var v0085, v0088 *time.Time
		var names []string
		rows, err := f.owner.Query(context.Background(), `SELECT version,applied_at FROM public.lc_schema_migrations WHERE version LIKE '0085%' OR version LIKE '0088%' ORDER BY version`)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var v string
			var at time.Time
			if err := rows.Scan(&v, &at); err != nil {
				t.Fatal(err)
			}
			names = append(names, v)
			if strings.HasPrefix(v, "0085") {
				v0085 = &at
			} else {
				v0088 = &at
			}
		}
		rows.Close()
		if v0085 == nil || v0088 == nil || v0085.After(*v0088) {
			t.Fatalf("migrations applied: %v (0085 %v, 0088 %v): both must be applied, 0085 first", names, v0085, v0088)
		}
		if n := e.count(`SELECT count(*) FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname='identity' AND p.proname='read_finance_summary'`); n != 1 {
			t.Errorf("%d read_finance_summary definitions", n)
		}
		var def string
		if err := f.owner.QueryRow(context.Background(), `SELECT pg_get_functiondef('identity.read_finance_summary(bytea,uuid,date,date)'::regprocedure)`).Scan(&def); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"captured_count", "refunded_minor", "net_minor", "pickup_collected_count", "pickup_collected_minor", "bank_transfer_confirmed_count", "bank_transfer_confirmed_minor"} {
			if !strings.Contains(def, key) {
				t.Errorf("the live finance function lost %s: a later migration overwrote an earlier one", key)
			}
		}
	})

	// one pay-at-pickup order collected (0085) and one bank transfer confirmed (0088), same Taipei day and currency
	e.cvsSettings(tcvAllChains, true, "20000", 500)
	manual, _, _ := e.service("cvs_711", "MANUAL", 0)
	pb := e.newBuyer()
	pres, err := e.tppPlace(pb, manual, e.tppEntered(pb, manual), tppName, tppPhone)
	if err != nil {
		t.Fatal(err)
	}
	pickupOrder := pres.OrderID
	if st, _, raw := e.mcall(e.token(), "PUT", "/v1/admin/stores/"+e.store()+"/orders/"+pickupOrder+"/shipment", t04Key("cog-fin-ms"), mfxShip(0, "seven_eleven_cvs", "0012345678")); st != 200 {
		t.Fatalf("manual shipment: %d %s", st, raw)
	}
	if st, _, raw := e.record(e.token(), pickupOrder, t04Key("cog-fin-col"), "PENDING", "collected"); st != 200 {
		t.Fatalf("collected: %d %s", st, raw)
	}
	pickupTotal := e.cogTotal(pickupOrder)
	tb, transferOrder := e.cogPlace("")
	tb.cofProof(transferOrder, t04Key("cog-fin-p"), "12345", e.cogTotal(transferOrder))
	if d := e.cogConfirm(transferOrder, t04Key("cog-fin-c")); d.status != 200 {
		t.Fatalf("confirm: %d %v", d.status, d.out)
	}
	transferTotal := e.cogTotal(transferOrder)
	// a second transfer that is placed and rejected, and a third that expires, must add nothing
	rb, rejected := e.cogPlace("")
	rb.cofProof(rejected, t04Key("cog-fin-rp"), "22222", 100)
	e.cofDecide(e.token(), rejected, "reject", t04Key("cog-fin-rr"), `{"reason":"mismatch"}`)
	_, expiring := e.cogPlace("")
	e.cofAge(expiring, 80)
	e.cofExpire(expiring)

	sums := func() (pCount, pMinor, tCount, tMinor, captured, net float64) {
		today := time.Now().In(time.FixedZone("TPE", 8*3600))
		from, to := today.AddDate(0, 0, -1).Format("2006-01-02"), today.AddDate(0, 0, 1).Format("2006-01-02")
		st, out, raw := e.mcall(e.token(), "GET", "/v1/admin/stores/"+e.store()+"/finance/summary?from="+from+"&to="+to, "", "")
		if st != 200 {
			t.Fatalf("finance summary: %d %s", st, raw)
		}
		rows, _ := out["totals"].([]any)
		for _, r := range rows {
			m, _ := r.(map[string]any)
			if m["currency"] != "TWD" {
				continue
			}
			pCount += m["pickup_collected_count"].(float64)
			pMinor += m["pickup_collected_minor"].(float64)
			tCount += m["bank_transfer_confirmed_count"].(float64)
			tMinor += m["bank_transfer_confirmed_minor"].(float64)
			captured += m["captured_minor"].(float64)
			net += m["net_minor"].(float64)
		}
		return
	}
	t.Run("the summary carries the pickup and the transfer column together, each exactly once, never inside captured/net", func(t *testing.T) {
		pc, pm, tc, tm, cap, net := sums()
		if pc != 1 || pm != float64(pickupTotal) || tc != 1 || tm != float64(transferTotal) || cap != 0 || net != 0 {
			t.Errorf("pickup %v/%v (want 1/%d), transfer %v/%v (want 1/%d), captured %v net %v (want 0/0)", pc, pm, pickupTotal, tc, tm, transferTotal, cap, net)
		}
	})
	t.Run("the CSV export has all 11 columns with the same figures", func(t *testing.T) {
		today := time.Now().In(time.FixedZone("TPE", 8*3600))
		from, to := today.AddDate(0, 0, -1).Format("2006-01-02"), today.AddDate(0, 0, 1).Format("2006-01-02")
		raw := e.financeCSV(from, to)
		lines := strings.Split(strings.TrimRight(raw, "\r\n"), "\n")
		if !strings.HasSuffix(lines[0], "pickup_collected_count,pickup_collected_minor,bank_transfer_confirmed_count,bank_transfer_confirmed_minor") {
			t.Fatalf("csv header %q", lines[0])
		}
		var pc, pm, tc, tm int64
		for _, l := range lines[1:] {
			c := strings.Split(strings.TrimSpace(l), ",")
			if len(c) != 11 {
				t.Fatalf("csv row %q has %d columns", l, len(c))
			}
			var a, b, cc, d int64
			fmt.Sscan(c[7], &a)
			fmt.Sscan(c[8], &b)
			fmt.Sscan(c[9], &cc)
			fmt.Sscan(c[10], &d)
			pc, pm, tc, tm = pc+a, pm+b, tc+cc, tm+d
		}
		if pc != 1 || pm != pickupTotal || tc != 1 || tm != transferTotal {
			t.Errorf("csv pickup %d/%d transfer %d/%d, want 1/%d and 1/%d", pc, pm, tc, tm, pickupTotal, transferTotal)
		}
	})
	t.Run("an offline refund takes the transfer out of the confirmed column and leaves the pickup column alone", func(t *testing.T) {
		if st, out := e.cofDecide(e.token(), transferOrder, "refund-offline", t04Key("cog-fin-ref"), `{}`); st != 200 {
			t.Fatalf("refund: %d %v", st, out)
		}
		pc, pm, tc, tm, cap, _ := sums()
		if pc != 1 || pm != float64(pickupTotal) || tc != 0 || tm != 0 || cap != 0 {
			t.Errorf("after the refund: pickup %v/%v transfer %v/%v captured %v", pc, pm, tc, tm, cap)
		}
	})
}

func TestCogTransferACL(t *testing.T) {
	e := cogNew(t, 72)
	owner := e.p.f.owner
	grantees := func(sig string) []string {
		rows, err := owner.Query(context.Background(), `SELECT coalesce(r.rolname,'PUBLIC') FROM pg_proc p, aclexplode(p.proacl) a LEFT JOIN pg_roles r ON r.oid=a.grantee
		  WHERE p.oid=$1::regprocedure AND a.privilege_type='EXECUTE' AND a.grantee<>p.proowner ORDER BY 1`, sig)
		if err != nil {
			t.Fatalf("%s: %v", sig, err)
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var s string
			if err := rows.Scan(&s); err != nil {
				t.Fatal(err)
			}
			out = append(out, s)
		}
		return out
	}
	for sig, want := range map[string][]string{
		"payments.decide_bank_transfer(bytea,uuid,uuid,text,bytea,text,text)":                                           {"commerce_runtime"},
		"payments.set_bank_transfer_settings(bytea,uuid,text,bytea,bigint,boolean,boolean,text,text,text,text,integer)": {"commerce_runtime"},
		"payments.read_bank_transfer_settings(bytea,uuid)":                                                              {"commerce_runtime"},
		"payments.read_bank_transfer_merchant(bytea,uuid,uuid)":                                                         {"commerce_runtime"},
		"checkout.submit_transfer_proof(bytea,uuid,text,bytea,uuid,text,bigint,timestamptz)":                            {"commerce_checkout_runtime"},
		"checkout.read_bank_transfer_buyer(bytea,uuid,uuid)":                                                            {"commerce_checkout_runtime"},
		"checkout.read_transfer_offer(bytea,uuid)":                                                                      {"commerce_checkout_runtime"},
		"checkout.set_order_buyer_email(bytea,uuid,uuid,text)":                                                          {"commerce_checkout_runtime"},
		"checkout.clear_buyer_email(uuid,uuid,uuid)":                                                                    {"commerce_privacy_writer"},
		"checkout.expire_held(uuid,bigint)":                                                                             {"commerce_worker"},
		"checkout.bank_transfer_json(uuid,uuid,uuid,boolean)":                                                           nil,
	} {
		got := grantees(sig)
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("EXECUTE on %s: %v, want %v", sig, got, want)
		}
	}

	t.Run("confirming is reachable by one function only: no other function updates the transfer, none other sets CONFIRMED, no triggers", func(t *testing.T) {
		names := func(pattern string) []string {
			rows, err := owner.Query(context.Background(), `SELECT n.nspname||'.'||p.proname FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE p.prosrc ~* $1 ORDER BY 1`, pattern)
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()
			var out []string
			for rows.Next() {
				var s string
				if err := rows.Scan(&s); err != nil {
					t.Fatal(err)
				}
				out = append(out, s)
			}
			return out
		}
		if got := names(`update\s+checkout\.bank_transfers`); fmt.Sprint(got) != "[checkout.expire_held checkout.submit_transfer_proof payments.decide_bank_transfer]" {
			t.Errorf("functions that UPDATE the transfer: %v", got)
		}
		if got := names(`set\s+state\s*=\s*'CONFIRMED'`); fmt.Sprint(got) != "[payments.decide_bank_transfer]" {
			t.Errorf("functions that set a transfer CONFIRMED: %v (anything else is an auto-confirm path)", got)
		}
		if n := e.count(`SELECT count(*) FROM pg_trigger WHERE tgrelid='checkout.bank_transfers'::regclass AND NOT tgisinternal`); n != 0 {
			t.Errorf("%d triggers on checkout.bank_transfers", n)
		}
	})
}

func TestCogNeverAutoConfirms(t *testing.T) {
	e := cogNew(t, 72)
	b, order := e.cogPlace("")
	total := e.cogTotal(order)
	for i, amount := range []int64{total, 1, total * 10, 1000000000000} {
		if p := b.cofProof(order, t04Key("cog-auto"), fmt.Sprintf("%05d", 10000+i), amount); p.status != 200 {
			t.Fatalf("proof of %d: %d %s", amount, p.status, p.body)
		}
		if r := e.cogRow(order); r.commercial != "AWAITING_TRANSFER" || r.transfer != "SUBMITTED" || r.allocate != 0 {
			t.Fatalf("a buyer proof of %d moved the order: %+v", amount, r)
		}
	}
	if d, _ := e.cofExpire(order); d != "NOT_DUE" {
		t.Errorf("the expiry worker on an open window answered %s", d)
	}
	if st, _, _ := e.mcall(e.token(), "GET", "/v1/admin/stores/"+e.store()+"/orders/"+order+"/bank-transfer", "", ""); st != 200 {
		t.Errorf("merchant read: %d", st)
	}
	if r := e.cogRow(order); r.commercial != "AWAITING_TRANSFER" || r.reservation != "HELD" || r.allocate != 0 || r.release != 0 {
		t.Errorf("waiting, reading and a worker pass changed the order: %+v", r)
	}
	// the buyer-facing amount is the server total, whatever the buyer claimed
	view := cofJSON(t, b.cofView(order).body)
	if view["amount_minor"] != float64(total) {
		t.Errorf("the buyer view amount %v, want the server total %d", view["amount_minor"], total)
	}
	if d := e.cogConfirm(order, t04Key("cog-auto-c")); d.status != 200 {
		t.Fatalf("merchant confirm: %d %v", d.status, d.out)
	}
	var confirmed int64
	if err := e.p.f.owner.QueryRow(context.Background(), `SELECT confirmed_amount_minor FROM checkout.bank_transfers WHERE order_id=$1`, order).Scan(&confirmed); err != nil || confirmed != total {
		t.Errorf("confirmed amount %d (%v): the buyer's claimed amounts must never become the money fact; want %d", confirmed, err, total)
	}
}

// financeCSV runs reporting.FinanceCSV in the creator's scope (the export route's own function; it needs orders:export and writes an audit row).
func (e *tcvEnv) financeCSV(from, to string) string {
	e.t.Helper()
	ctx := context.Background()
	var out []byte
	token := e.token()
	err := platform.WithScope(ctx, e.p.f.runtime, token, e.store(), "store:read", func(tx pgx.Tx, s platform.Scope) (er error) {
		out, er = reporting.FinanceCSV(ctx, tx, s, token, from, to)
		return er
	})
	if err != nil {
		e.t.Fatalf("finance csv: %v", err)
	}
	return string(out)
}
