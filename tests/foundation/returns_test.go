package foundation_test

// Purpose: W3-08B merchant returns (RMA) and merchant order cancel over REAL_PG and the real HTTP handler (unit w3-08b-returns):
//   RT01 register -> receive -> inspect -> close with ONE guarded DEALLOCATE row and payments untouched, RT02 quantity/shipped/cross-store
//   refusals, RT03 inspect/close need inventory:write, RT07 refund stays decoupled from stock, RT09 replay and version drift, concurrent
//   disposition and partial RMAs restock exactly once, direct ledger forgeries refused; TestMerchantCancel: RT04 unpaid hold released,
//   RT05 payment in flight, RT06 refund_first then cancel, RT08 parcel group shrink/dissolve, RT10 cancel vs payment start, cancelled
//   order consequences (notify, no auto refund), already shipped, replays.
// Depends on: tcvEnv (stripe mode: card home orders paid through the rfx capture path), the 0155 definers and guard
//   inventory.guard_returns_ledger, merchant routes /orders/{id}/cancel, /orders/{id}/returns, /returns, /returns/{id}/*,
//   /orders/{id}/shipment, /parcel-groups, the refund route (e.r.mustRefund).
// Used by: go test ./tests/foundation (-run '^TestReturns|^TestMerchantCancel'); evidence class REAL_PG with MOCK Stripe fakes.

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func (e *tcvEnv) rtPath(suffix string) string { return "/v1/admin/stores/" + e.store() + suffix }

// rtPaid places and pays one card home order of a fresh buyer (2 units of the harness SKU) and returns it with its rfx handle.
func (e *tcvEnv) rtPaid() (string, rfxOrder) {
	e.t.Helper()
	b := e.newBuyer()
	e.hcodRehome(b)
	res, err := e.svc.Begin(context.Background(), b.cap.Token, e.store(), t04Key("rt-begin"), b.h.input)
	if err != nil {
		e.t.Fatalf("place card home order: %v", err)
	}
	rf := e.payHold(res, b)
	return rf.order, rf
}

// rtPaidOf is rtPaid for an existing buyer (so several orders can share one address and one parcel group).
func (e *tcvEnv) rtPaidOf(b *tcvBuyer) (string, rfxOrder) {
	e.t.Helper()
	e.hcodRehome(b)
	res, err := e.svc.Begin(context.Background(), b.cap.Token, e.store(), t04Key("rt-begin"), b.h.input)
	if err != nil {
		e.t.Fatalf("place card home order: %v", err)
	}
	rf := e.payHold(res, b)
	return rf.order, rf
}

func (e *tcvEnv) rtShip(order string) {
	e.t.Helper()
	st, _, raw := e.mcall(e.token(), "PUT", e.rtPath("/orders/"+order+"/shipment"), "rt-ship-"+order[:8], mfxShip(0, "black_cat", "RT"+order[:8]))
	if st != 200 {
		e.t.Fatalf("ship %s: %d %s", order, st, raw)
	}
}

func (e *tcvEnv) rtSKU() (sku, wh string) { return e.p.stock.skus[0].ID, e.p.stock.warehouse.ID }

func (e *tcvEnv) rtLines(key string, vals ...int) string {
	sku, _ := e.rtSKU()
	var parts []string
	for _, v := range vals {
		parts = append(parts, fmt.Sprintf(`{"sku_id":%q,"%s":%d}`, sku, key, v))
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func (e *tcvEnv) rtRegister(token, order, key string, qty int) (int, map[string]any) {
	st, out, _ := e.mcall(token, "POST", e.rtPath("/orders/"+order+"/returns"), key, fmt.Sprintf(`{"reason":"damaged","lines":%s}`, e.rtLines("quantity", qty)))
	return st, out
}

func (e *tcvEnv) rtReceive(token, rma, key string, version, qty int) (int, map[string]any) {
	st, out, _ := e.mcall(token, "POST", e.rtPath("/returns/"+rma+"/receive"), key, fmt.Sprintf(`{"expected_version":%d,"lines":%s}`, version, e.rtLines("qty_received", qty)))
	return st, out
}

func (e *tcvEnv) rtInspect(token, rma, key string, version, restock, scrap int) (int, map[string]any) {
	sku, _ := e.rtSKU()
	body := fmt.Sprintf(`{"expected_version":%d,"lines":[{"sku_id":%q,"qty_restock":%d,"qty_scrap":%d}]}`, version, sku, restock, scrap)
	st, out, _ := e.mcall(token, "POST", e.rtPath("/returns/"+rma+"/inspect"), key, body)
	return st, out
}

func (e *tcvEnv) rtClose(token, rma, key string, version int, refund string) (int, map[string]any) {
	body := fmt.Sprintf(`{"expected_version":%d}`, version)
	if refund != "" {
		body = fmt.Sprintf(`{"expected_version":%d,"refund_id":%q}`, version, refund)
	}
	st, out, _ := e.mcall(token, "POST", e.rtPath("/returns/"+rma+"/close"), key, body)
	return st, out
}

// rtInspected drives a fresh RMA of qty units to INSPECTED (all sellable) and returns its id and version.
func (e *tcvEnv) rtInspected(order string, qty int, key string) (string, int) {
	e.t.Helper()
	st, out := e.rtRegister(e.token(), order, key+"-reg", qty)
	if st != 201 {
		e.t.Fatalf("register: %d %v", st, out)
	}
	id := out["id"].(string)
	if st, out = e.rtReceive(e.token(), id, key+"-rcv", 1, qty); st != 200 {
		e.t.Fatalf("receive: %d %v", st, out)
	}
	if st, out = e.rtInspect(e.token(), id, key+"-ins", 2, qty, 0); st != 200 {
		e.t.Fatalf("inspect: %d %v", st, out)
	}
	return id, 3
}

func rtExpect(t *testing.T, what string, st int, out map[string]any, wantStatus int, wantCode string) {
	t.Helper()
	if st != wantStatus || (wantCode != "" && out["code"] != wantCode) {
		t.Fatalf("%s: got %d %v, want %d %s", what, st, out, wantStatus, wantCode)
	}
}

// rtLedger counts ledger rows of one operation (optionally for one checkout).
func (e *tcvEnv) rtLedger(op string) int {
	return e.count(`SELECT count(*) FROM inventory.ledger WHERE store_id=$1 AND operation=$2`, e.store(), op)
}

// rtAll counts every ledger row of the store (a refund must not add any).
func (e *tcvEnv) rtAll() int {
	return e.count(`SELECT count(*) FROM inventory.ledger WHERE store_id=$1`, e.store())
}

func (e *tcvEnv) rtStates(order string) (commercial, fulfilment, reservation string) {
	if err := e.p.f.owner.QueryRow(context.Background(), `SELECT o.commercial_state,o.fulfillment_state,r.state FROM checkout.orders o
	 JOIN inventory.reservations r ON r.tenant_id=o.tenant_id AND r.store_id=o.store_id AND r.id=o.id WHERE o.id=$1`, order).Scan(&commercial, &fulfilment, &reservation); err != nil {
		e.t.Fatal(err)
	}
	return
}

func TestReturns(t *testing.T) {
	e := tcvNew(t, tcvOpts{stripe: true})
	e.r.startWorker(t)
	e.grantCreator("orders:read", "fulfillment:write", "inventory:write")
	f := e.p.f
	sku, wh := e.rtSKU()

	o1, rf1 := e.rtPaid()
	e.rtShip(o1)
	var rma1 string

	t.Run("RT01 register 2, receive 2, 1 sellable + 1 scrap: one DEALLOCATE row, stock back once, payments untouched", func(t *testing.T) {
		money, refunds, ledger := e.pgPaymentFingerprint(), e.count(`SELECT count(*) FROM payments.stripe_refunds WHERE store_id=$1`, e.store()), e.rtAll()
		oh0, rs0, al0 := e.cofBalance(sku)
		st, out := e.rtRegister(e.token(), o1, "rt-reg-0001", 2)
		rtExpect(t, "register", st, out, 201, "")
		rma1 = out["id"].(string)
		if out["state"] != "REGISTERED" || out["version"] != float64(1) || out["order_id"] != o1 {
			t.Fatalf("register body %v", out)
		}
		st, out = e.rtInspect(e.token(), rma1, "rt-ins-0000", 1, 1, 1)
		rtExpect(t, "inspect before receive", st, out, 409, "invalid_state")
		st, out = e.rtReceive(e.token(), rma1, "rt-rcv-0001", 1, 2)
		rtExpect(t, "receive", st, out, 200, "")
		if out["state"] != "RECEIVED" || out["version"] != float64(2) {
			t.Fatalf("receive body %v", out)
		}
		st, out = e.rtInspect(e.token(), rma1, "rt-ins-0001", 2, 1, 1)
		rtExpect(t, "inspect", st, out, 200, "")
		if out["state"] != "INSPECTED" || out["version"] != float64(3) {
			t.Fatalf("inspect body %v", out)
		}
		if e.rtAll() != ledger {
			t.Fatal("inspect (the decision) must not write the ledger")
		}
		st, out = e.rtClose(e.token(), rma1, "rt-cls-0001", 3, "")
		rtExpect(t, "close", st, out, 200, "")
		if out["state"] != "CLOSED" || out["version"] != float64(4) || out["restocked_units"] != float64(1) {
			t.Fatalf("close body %v", out)
		}
		lines := out["lines"].([]any)
		l0 := lines[0].(map[string]any)
		if l0["qty_registered"] != float64(2) || l0["qty_received"] != float64(2) || l0["qty_restock"] != float64(1) || l0["qty_scrap"] != float64(1) || l0["warehouse_id"] != wh {
			t.Fatalf("line projection %v", l0)
		}
		// exactly one DEALLOCATE row for the 1 sellable unit, none for the scrapped one
		if n := e.rtLedger("returns.rma.restock"); n != 1 || e.rtAll() != ledger+1 {
			t.Fatalf("restock ledger rows %d (all %d -> %d)", n, ledger, e.rtAll())
		}
		var kind, reason, key string
		var dAlloc, dOn, dRes int64
		if err := f.owner.QueryRow(context.Background(), `SELECT kind,reason,command_key,delta_allocated,delta_on_hand,delta_reserved FROM inventory.ledger WHERE store_id=$1 AND operation='returns.rma.restock'`, e.store()).
			Scan(&kind, &reason, &key, &dAlloc, &dOn, &dRes); err != nil {
			t.Fatal(err)
		}
		if kind != "DEALLOCATE" || reason != "rma_restock" || key != rma1 || dAlloc != -1 || dOn != 0 || dRes != 0 {
			t.Fatalf("ledger row %s %s %s %d/%d/%d", kind, reason, key, dAlloc, dOn, dRes)
		}
		oh1, rs1, al1 := e.cofBalance(sku)
		if oh1 != oh0 || rs1 != rs0 || al1 != al0-1 {
			t.Fatalf("balance (on_hand,reserved,allocated) %d,%d,%d -> %d,%d,%d: want only allocated -1", oh0, rs0, al0, oh1, rs1, al1)
		}
		if got := e.pgPaymentFingerprint(); got != money {
			t.Fatalf("I05: payments/orders money changed %v -> %v", money, got)
		}
		if n := e.count(`SELECT count(*) FROM payments.stripe_refunds WHERE store_id=$1`, e.store()); n != refunds {
			t.Fatalf("a return created a refund (%d -> %d)", refunds, n)
		}
		if c, fu, rv := e.rtStates(o1); c != "CONFIRMED" || fu != "MERCHANT_SHIPPED" || rv != "COMMITTED" {
			t.Fatalf("order states %s %s %s changed by the return", c, fu, rv)
		}
		for _, a := range []string{"returns.rma_registered", "returns.rma_received", "returns.rma_inspected", "returns.rma_closed"} {
			if n := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action=$2`, e.store(), a); n != 1 {
				t.Fatalf("audit %s rows %d", a, n)
			}
		}
		// replay of the close: same body, nothing written; a new key on a CLOSED RMA is refused
		st, again := e.rtClose(e.token(), rma1, "rt-cls-0001", 3, "")
		if st != 200 || again["id"] != rma1 || again["state"] != "CLOSED" || e.rtLedger("returns.rma.restock") != 1 {
			t.Fatalf("close replay: %d %v", st, again)
		}
		st, out = e.rtClose(e.token(), rma1, "rt-cls-0002", 3, "")
		rtExpect(t, "second close, stale version", st, out, 409, "version_changed")
		st, out = e.rtClose(e.token(), rma1, "rt-cls-0003", 4, "")
		rtExpect(t, "second close, current version", st, out, 409, "invalid_state")
		if e.rtLedger("returns.rma.restock") != 1 {
			t.Fatal("a second close restocked again")
		}
		// the same key for a different request is a conflict, never a second effect
		st, out = e.rtClose(e.token(), rma1, "rt-cls-0001", 4, "")
		rtExpect(t, "same key, other body", st, out, 409, "conflict")
	})

	t.Run("RT02 refusals: unshipped, over the shipped quantity (closed RMAs count), unknown line, cross-store", func(t *testing.T) {
		o2, _ := e.rtPaid() // paid, never shipped
		st, out := e.rtRegister(e.token(), o2, "rt-reg-0010", 1)
		rtExpect(t, "unshipped order", st, out, 409, "not_shipped")
		// o1 shipped 2 units and RMA 1 (CLOSED) registered all 2: they cannot be returned again
		st, out = e.rtRegister(e.token(), o1, "rt-reg-0011", 1)
		rtExpect(t, "returned twice", st, out, 422, "exceeds_shipped")

		o3, _ := e.rtPaid()
		e.rtShip(o3)
		st, out = e.rtRegister(e.token(), o3, "rt-reg-0012", 3)
		rtExpect(t, "3 of 2 shipped", st, out, 422, "exceeds_shipped")
		st, out = e.rtRegister(e.token(), o3, "rt-reg-0013", 0)
		rtExpect(t, "zero units", st, out, 422, "invalid_request")
		unknown := `{"reason":"damaged","lines":[{"sku_id":"99999999-9999-4999-8999-999999999999","quantity":1}]}`
		st, out, _ = e.mcall(e.token(), "POST", e.rtPath("/orders/"+o3+"/returns"), "rt-reg-0014", unknown)
		rtExpect(t, "unknown sku", st, out, 422, "unknown_line")
		st, out, _ = e.mcall(e.token(), "POST", e.rtPath("/orders/99999999-9999-4999-8999-999999999999/returns"), "rt-reg-0015", fmt.Sprintf(`{"reason":"damaged","lines":%s}`, e.rtLines("quantity", 1)))
		rtExpect(t, "unknown order", st, out, 404, "not_found")
		if n := e.count(`SELECT count(*) FROM returns.rmas WHERE order_id IN ($1,$2)`, o2, o3); n != 0 {
			t.Fatalf("refused registrations left %d RMAs", n)
		}

		// a registration that is then withdrawn frees its quantity (only REGISTERED can be cancelled)
		st, out = e.rtRegister(e.token(), o3, "rt-reg-0016", 2)
		rtExpect(t, "register 2 of 2", st, out, 201, "")
		id := out["id"].(string)
		st, out = e.rtRegister(e.token(), o3, "rt-reg-0017", 1)
		rtExpect(t, "1 more while 2 are live", st, out, 422, "exceeds_shipped")
		st, out, _ = e.mcall(e.token(), "POST", e.rtPath("/returns/"+id+"/cancel"), "rt-can-0001", `{"expected_version":1}`)
		rtExpect(t, "cancel RMA", st, out, 200, "")
		if out["state"] != "CANCELLED" {
			t.Fatalf("cancel body %v", out)
		}
		st, out = e.rtRegister(e.token(), o3, "rt-reg-0018", 2)
		rtExpect(t, "register again after the cancel", st, out, 201, "")
		st, out, _ = e.mcall(e.token(), "POST", e.rtPath("/returns/"+id+"/cancel"), "rt-can-0002", `{"expected_version":2}`)
		rtExpect(t, "cancel a CANCELLED RMA", st, out, 409, "invalid_state")

		// cross-store: the other store's operator (all needed permissions there) cannot reach this store's order or RMA
		otherStore := randomUUID() // a second store of the same tenant (disclosed owner fixture)
		mustExec(t, f.owner, `INSERT INTO control.stores(tenant_id,id,name,currency) VALUES($1,$2,'rt-other-store','TWD')`, f.tenantA, otherStore)
		a2token, a2principal := randomToken(), randomUUID()
		mustExec(t, f.owner, `INSERT INTO identity.principals(id) VALUES($1)`, a2principal)
		mustExec(t, f.owner, `INSERT INTO identity.memberships(tenant_id,principal_id) VALUES($1,$2)`, f.tenantA, a2principal)
		for _, perm := range []string{"store:read", "orders:read", "fulfillment:write", "inventory:write"} {
			mustExec(t, f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,$4)`, f.tenantA, otherStore, a2principal, perm)
		}
		a2tx, err := f.owner.Begin(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if err := insertSession(context.Background(), a2tx, a2token, a2principal, "merchant", time.Now().Add(time.Hour), nil); err != nil {
			t.Fatal(err)
		}
		if err := a2tx.Commit(context.Background()); err != nil {
			t.Fatal(err)
		}
		other := "/v1/admin/stores/" + otherStore
		before := e.count(`SELECT count(*) FROM returns.rmas`)
		st, out, _ = e.mcall(a2token, "POST", other+"/orders/"+o3+"/returns", "rt-reg-0019", fmt.Sprintf(`{"reason":"damaged","lines":%s}`, e.rtLines("quantity", 1)))
		rtExpect(t, "register against another store's order", st, out, 404, "not_found")
		st, out, _ = e.mcall(a2token, "GET", other+"/orders/"+o3+"/returns", "", "")
		rtExpect(t, "read another store's order returns", st, out, 404, "not_found")
		st, out, _ = e.mcall(a2token, "POST", other+"/returns/"+rma1+"/receive", "rt-rcv-0019", fmt.Sprintf(`{"expected_version":1,"lines":%s}`, e.rtLines("qty_received", 1)))
		rtExpect(t, "receive another store's RMA", st, out, 404, "not_found")
		st, out, _ = e.mcall(a2token, "POST", other+"/orders/"+o3+"/cancel", "rt-can-0019", `{"expected_state":"CONFIRMED","reason":"x"}`)
		rtExpect(t, "cancel another store's order", st, out, 404, "not_found")
		if e.count(`SELECT count(*) FROM returns.rmas`) != before {
			t.Fatal("a cross-store request wrote a row")
		}
		if c, _, _ := e.rtStates(o3); c != "CONFIRMED" {
			t.Fatalf("cross-store cancel moved the order to %s", c)
		}
	})

	t.Run("RT03 inspect and close need inventory:write on top of fulfillment:write; state unchanged", func(t *testing.T) {
		o4, _ := e.rtPaid()
		e.rtShip(o4)
		st, out := e.rtRegister(e.token(), o4, "rt-reg-0020", 2)
		rtExpect(t, "register", st, out, 201, "")
		id := out["id"].(string)
		ops, _ := e.member("orders:read", "fulfillment:write") // no inventory:write
		if st, out = e.rtReceive(ops, id, "rt-rcv-0020", 1, 2); st != 200 {
			t.Fatalf("receive with fulfillment:write only: %d %v", st, out)
		}
		ledger := e.rtAll()
		st, out = e.rtInspect(ops, id, "rt-ins-0020", 2, 2, 0)
		rtExpect(t, "inspect without inventory:write", st, out, 403, "forbidden")
		if st, out = e.rtInspect(e.token(), id, "rt-ins-0021", 2, 2, 0); st != 200 {
			t.Fatalf("inspect with both: %d %v", st, out)
		}
		st, out = e.rtClose(ops, id, "rt-cls-0020", 3, "")
		rtExpect(t, "close without inventory:write", st, out, 403, "forbidden")
		stockOnly, _ := e.member("orders:read", "inventory:write") // no fulfillment:write
		st, out = e.rtClose(stockOnly, id, "rt-cls-0021", 3, "")
		rtExpect(t, "close without fulfillment:write", st, out, 403, "forbidden")
		var state string
		var version int
		if err := f.owner.QueryRow(context.Background(), `SELECT state,version FROM returns.rmas WHERE id=$1`, id).Scan(&state, &version); err != nil || state != "INSPECTED" || version != 3 {
			t.Fatalf("RMA after the refusals: %s v%d (%v)", state, version, err)
		}
		if e.rtAll() != ledger {
			t.Fatal("a refused disposition wrote the ledger")
		}
		// reads need orders:read
		noRead, _ := e.member("fulfillment:write")
		st, out, _ = e.mcall(noRead, "GET", e.rtPath("/orders/"+o4+"/returns"), "", "")
		rtExpect(t, "read without orders:read", st, out, 403, "forbidden")
		st, out, _ = e.mcall(e.token(), "GET", e.rtPath("/orders/"+o4+"/returns"), "", "")
		if items, _ := out["items"].([]any); st != 200 || len(items) != 1 {
			t.Fatalf("order returns: %d %v", st, out)
		}
		st, out, _ = e.mcall(e.token(), "GET", e.rtPath("/returns?state=INSPECTED"), "", "")
		if items, _ := out["items"].([]any); st != 200 || len(items) != 1 {
			t.Fatalf("list INSPECTED: %d %v", st, out)
		}
		st, out, _ = e.mcall(e.token(), "GET", e.rtPath("/returns?state=BOGUS"), "", "")
		rtExpect(t, "bad state filter", st, out, 422, "invalid_request")
	})

	t.Run("RT07 a refund neither restocks nor moves the order; the RMA only links it, a foreign refund is refused", func(t *testing.T) {
		o5, rf5 := e.rtPaid()
		e.rtShip(o5)
		ledger := e.rtAll()
		refund := e.r.mustRefund(t, rf5, rf5.captured, "requested_by_customer") // M-8: a shipped order may be refunded without a return
		if e.rtAll() != ledger {
			t.Fatal("a refund wrote the inventory ledger (RD6)")
		}
		if c, fu, rv := e.rtStates(o5); c != "CONFIRMED" || fu != "MERCHANT_SHIPPED" || rv != "COMMITTED" {
			t.Fatalf("a refund changed the order to %s %s %s", c, fu, rv)
		}
		id, v := e.rtInspected(o5, 2, "rt-rt07")
		// a refund of ANOTHER order is not linkable
		st, out := e.rtClose(e.token(), id, "rt-cls-0030", v, refund[:len(refund)-1]+"0")
		rtExpect(t, "unknown refund id", st, out, 422, "refund_mismatch")
		_ = rf1
		other := e.r.mustRefund(t, rf1, 100, "requested_by_customer")
		st, out = e.rtClose(e.token(), id, "rt-cls-0031", v, other)
		rtExpect(t, "refund of another order", st, out, 422, "refund_mismatch")
		if n := e.rtLedger("returns.rma.restock"); n != 1 {
			t.Fatalf("refused closes wrote restock rows (%d)", n)
		}
		st, out = e.rtClose(e.token(), id, "rt-cls-0032", v, refund)
		rtExpect(t, "close linked to the order's own refund", st, out, 200, "")
		if out["refund_id"] != refund || out["restocked_units"] != float64(2) {
			t.Fatalf("close body %v", out)
		}
		if n := e.count(`SELECT count(*) FROM payments.stripe_refunds WHERE order_id=$1`, o5); n != 1 {
			t.Fatalf("the return changed the refund rows (%d)", n)
		}
	})

	t.Run("RT09 replay returns the original, other body on the same key conflicts, stale version drifts", func(t *testing.T) {
		o6, _ := e.rtPaid()
		e.rtShip(o6)
		st, first := e.rtRegister(e.token(), o6, "rt-reg-0040", 1)
		rtExpect(t, "register", st, first, 201, "")
		st, again := e.rtRegister(e.token(), o6, "rt-reg-0040", 1)
		if st != 201 || again["id"] != first["id"] || e.count(`SELECT count(*) FROM returns.rmas WHERE order_id=$1`, o6) != 1 {
			t.Fatalf("register replay: %d %v", st, again)
		}
		st, out := e.rtRegister(e.token(), o6, "rt-reg-0040", 2)
		rtExpect(t, "same key other quantity", st, out, 409, "conflict")
		id := first["id"].(string)
		st, out = e.rtReceive(e.token(), id, "rt-rcv-0040", 7, 1)
		rtExpect(t, "stale version", st, out, 409, "version_changed")
		st, out = e.rtReceive(e.token(), id, "rt-rcv-0041", 1, 5)
		rtExpect(t, "received more than registered", st, out, 422, "exceeds_registered")
		st, out = e.rtReceive(e.token(), id, "rt-rcv-0042", 1, 0)
		rtExpect(t, "nothing received", st, out, 422, "nothing_received")
		st, out = e.rtReceive(e.token(), id, "rt-rcv-0043", 1, 1)
		rtExpect(t, "receive", st, out, 200, "")
		st, out = e.rtReceive(e.token(), id, "rt-rcv-0043", 1, 1)
		if st != 200 || out["version"] != float64(2) {
			t.Fatalf("receive replay: %d %v", st, out)
		}
		st, out = e.rtInspect(e.token(), id, "rt-ins-0040", 2, 1, 1)
		rtExpect(t, "split that does not add up", st, out, 422, "quantities_mismatch")
		// a scrap-only disposition closes without any stock row
		ledger := e.rtAll()
		if st, out = e.rtInspect(e.token(), id, "rt-ins-0041", 2, 0, 1); st != 200 {
			t.Fatalf("inspect scrap: %d %v", st, out)
		}
		st, out = e.rtClose(e.token(), id, "rt-cls-0040", 3, "")
		rtExpect(t, "close scrap only", st, out, 200, "")
		if out["restocked_units"] != float64(0) || e.rtAll() != ledger {
			t.Fatalf("scrap-only close restocked %v / wrote ledger rows %d", out["restocked_units"], e.rtAll()-ledger)
		}
	})

	t.Run("two concurrent closes of one RMA restock exactly once", func(t *testing.T) {
		o7, _ := e.rtPaid()
		e.rtShip(o7)
		id, v := e.rtInspected(o7, 2, "rt-cc")
		_, _, al0 := e.cofBalance(sku)
		var wg sync.WaitGroup
		codes := make([]int, 2)
		for i := range codes {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				st, _ := e.rtClose(e.token(), id, fmt.Sprintf("rt-cls-par%d", i), v, "")
				codes[i] = st
			}(i)
		}
		wg.Wait()
		if !(codes[0] == 200 && codes[1] == 409 || codes[0] == 409 && codes[1] == 200) {
			t.Fatalf("concurrent closes answered %v, want exactly one 200 and one 409", codes)
		}
		if n := e.count(`SELECT count(*) FROM inventory.ledger WHERE operation='returns.rma.restock' AND command_key=$1`, id); n != 1 {
			t.Fatalf("restock rows %d", n)
		}
		if _, _, al1 := e.cofBalance(sku); al1 != al0-2 {
			t.Fatalf("allocated %d -> %d, want -2 exactly once", al0, al1)
		}
	})

	t.Run("two RMAs of one line closed concurrently each restock their own units, never more than shipped", func(t *testing.T) {
		o8, _ := e.rtPaid()
		e.rtShip(o8)
		a, va := e.rtInspected(o8, 1, "rt-pa")
		b, vb := e.rtInspected(o8, 1, "rt-pb")
		_, _, al0 := e.cofBalance(sku)
		var wg sync.WaitGroup
		codes := make([]int, 2)
		for i, id := range []string{a, b} {
			wg.Add(1)
			go func(i int, id string, v int) {
				defer wg.Done()
				st, _ := e.rtClose(e.token(), id, fmt.Sprintf("rt-cls-pp%d", i), v, "")
				codes[i] = st
			}(i, id, map[string]int{a: va, b: vb}[id])
		}
		wg.Wait()
		if codes[0] != 200 || codes[1] != 200 {
			t.Fatalf("partial closes answered %v", codes)
		}
		if n := e.count(`SELECT count(*) FROM inventory.ledger WHERE operation='returns.rma.restock' AND checkout_id=$1`, o8); n != 2 {
			t.Fatalf("restock rows %d", n)
		}
		if _, _, al1 := e.cofBalance(sku); al1 != al0-2 {
			t.Fatalf("allocated %d -> %d, want -2", al0, al1)
		}
		st, out := e.rtRegister(e.token(), o8, "rt-reg-0050", 1)
		rtExpect(t, "a third unit does not exist", st, out, 422, "exceeds_shipped")
	})

	t.Run("the ledger refuses forged rows with the reserved operations", func(t *testing.T) {
		// Disclosed owner fixture: the same GUCs the definers set, then a direct INSERT (triggers fire for every writer).
		o9, _ := e.rtPaid()
		e.rtShip(o9)
		var owner, session string
		if err := f.owner.QueryRow(context.Background(), `SELECT owner_id::text,creator_session_id::text FROM checkout.orders WHERE id=$1`, o9).Scan(&owner, &session); err != nil {
			t.Fatal(err)
		}
		forge := func(stmt string, args ...any) string {
			tx, err := f.owner.Begin(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			for k, v := range map[string]string{"app.tenant_id": f.tenantA, "app.store_id": f.storeA1, "app.principal_id": f.principalA, "app.buyer_id": owner, "app.buyer_session_id": session} {
				if _, err := tx.Exec(context.Background(), `SELECT set_config($1,$2,true)`, k, v); err != nil {
					t.Fatal(err)
				}
			}
			return tcvExecState(context.Background(), tx, stmt, args...)
		}
		ledger := e.rtAll()
		// 1) a plain merchant ADJUST that claims the restock operation (no RMA behind it)
		if st := forge(`INSERT INTO inventory.ledger(tenant_id,store_id,warehouse_id,sku_id,kind,delta_on_hand,operation,command_key,reason,principal_id,actor_kind)
		 VALUES($1,$2,$3,$4,'ADJUST',5,'returns.rma.restock','forged-key-1','rma_restock',$5,'MERCHANT')`, f.tenantA, f.storeA1, wh, sku, f.principalA); st != "42501" {
			t.Fatalf("forged ADJUST with the restock operation: SQLSTATE %q, want 42501", st)
		}
		// 2) a DEALLOCATE of this live order under the cancel operation (order not cancelled)
		if st := forge(`INSERT INTO inventory.ledger(tenant_id,store_id,warehouse_id,sku_id,kind,delta_allocated,operation,command_key,reservation_id,reason,principal_id,checkout_id,buyer_owner_id,buyer_session_id,actor_kind)
		 VALUES($1,$2,$3,$4,'DEALLOCATE',-2,'checkout.merchant_cancel',$5::text,$9::uuid,'merchant_cancel',$6,$9::uuid,$7,$8,'MERCHANT')`, f.tenantA, f.storeA1, wh, sku, o9, f.principalA, owner, session, o9); st != "42501" {
			t.Fatalf("forged cancel DEALLOCATE of a live order: SQLSTATE %q, want 42501", st)
		}
		// 3) a restock row naming an RMA that is not CLOSED
		id, _ := e.rtInspected(o9, 1, "rt-fg")
		if st := forge(`INSERT INTO inventory.ledger(tenant_id,store_id,warehouse_id,sku_id,kind,delta_allocated,operation,command_key,reservation_id,reason,principal_id,checkout_id,buyer_owner_id,buyer_session_id,actor_kind)
		 VALUES($1,$2,$3,$4,'DEALLOCATE',-1,'returns.rma.restock',$5::text,$6::uuid,'rma_restock',$7,$6::uuid,$8,$9,'MERCHANT')`, f.tenantA, f.storeA1, wh, sku, id, o9, f.principalA, owner, session); st != "42501" {
			t.Fatalf("forged restock of an INSPECTED RMA: SQLSTATE %q, want 42501", st)
		}
		if e.rtAll() != ledger {
			t.Fatal("a forged row survived")
		}
	})
}

// rtPayStarted places an order and starts its card payment WITHOUT paying (AWAITING_PAYMENT, a payment attempt exists).
func (e *tcvEnv) rtPayStarted() string {
	e.t.Helper()
	b := e.newBuyer()
	res, err := e.svc.Begin(context.Background(), b.cap.Token, e.store(), t04Key("rt-hold"), b.h.input)
	if err != nil {
		e.t.Fatalf("hold: %v", err)
	}
	s := e.ro.s
	s.p.hold = res
	s.p.cap = b.cap
	e.r.pinned(e.t, s)
	return res.OrderID
}

func TestMerchantCancel(t *testing.T) {
	e := tcvNew(t, tcvOpts{stripe: true})
	e.r.startWorker(t)
	e.grantCreator("orders:read", "fulfillment:write", "inventory:write")
	f := e.p.f
	sku, _ := e.rtSKU()
	cancel := func(token, order, key, state string) (int, map[string]any) {
		st, out, _ := e.mcall(token, "POST", e.rtPath("/orders/"+order+"/cancel"), key, fmt.Sprintf(`{"expected_state":%q,"reason":"customer asked"}`, state))
		return st, out
	}

	t.Run("RT04 an unpaid hold is cancelled: reservation released, sellable quantity restored, replay safe", func(t *testing.T) {
		b := e.newBuyer()
		res, err := e.svc.Begin(context.Background(), b.cap.Token, e.store(), t04Key("rt04"), b.h.input)
		if err != nil {
			t.Fatal(err)
		}
		order := res.OrderID
		oh0, rs0, al0 := e.cofBalance(sku)
		if c, _, rv := e.rtStates(order); c != "DRAFT" || rv != "HELD" {
			t.Fatalf("hold states %s %s", c, rv)
		}
		st, out := cancel(e.token(), order, "rt-can-1001", "CONFIRMED")
		rtExpect(t, "wrong expected state", st, out, 409, "state_changed")
		st, out = cancel(e.token(), order, "rt-can-1002", "DRAFT")
		rtExpect(t, "cancel the hold", st, out, 200, "")
		if out["commercial_state"] != "CANCELLED" || out["released_lines"] != float64(1) || out["parcel_group"] != nil {
			t.Fatalf("cancel body %v", out)
		}
		if c, fu, rv := e.rtStates(order); c != "CANCELLED" || fu != "CANCELLED" || rv != "RELEASED" {
			t.Fatalf("states after cancel %s %s %s", c, fu, rv)
		}
		oh1, rs1, al1 := e.cofBalance(sku)
		if oh1 != oh0 || al1 != al0 || rs1 != rs0-2 {
			t.Fatalf("balance %d,%d,%d -> %d,%d,%d: want reserved -2 only", oh0, rs0, al0, oh1, rs1, al1)
		}
		var kind, reason string
		if err := f.owner.QueryRow(context.Background(), `SELECT kind,reason FROM inventory.ledger WHERE checkout_id=$1 AND operation='checkout.merchant_cancel'`, order).Scan(&kind, &reason); err != nil || kind != "RELEASE" || reason != "merchant_cancel" {
			t.Fatalf("ledger row %s %s (%v)", kind, reason, err)
		}
		if n := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='orders.merchant_cancelled'`, e.store()); n < 1 {
			t.Fatal("no cancel audit row")
		}
		st, again := cancel(e.token(), order, "rt-can-1002", "DRAFT")
		if st != 200 || again["order_id"] != order || e.count(`SELECT count(*) FROM inventory.ledger WHERE checkout_id=$1 AND operation='checkout.merchant_cancel'`, order) != 1 {
			t.Fatalf("replay: %d %v", st, again)
		}
		st, out = cancel(e.token(), order, "rt-can-1003", "DRAFT")
		rtExpect(t, "cancel a cancelled order", st, out, 409, "already_cancelled")
		st, out = cancel(e.token(), order, "rt-can-1002", "CONFIRMED")
		rtExpect(t, "same key, other body", st, out, 409, "conflict")
	})

	t.Run("RT05 a payment in progress cannot be cancelled by the merchant", func(t *testing.T) {
		order := e.rtPayStarted()
		_, rs0, al0 := e.cofBalance(sku)
		ledger := e.rtAll()
		st, out := cancel(e.token(), order, "rt-can-1010", "AWAITING_PAYMENT")
		rtExpect(t, "cancel AWAITING_PAYMENT", st, out, 409, "payment_in_flight")
		if c, _, rv := e.rtStates(order); c != "AWAITING_PAYMENT" || rv != "PAYMENT_PENDING" {
			t.Fatalf("states %s %s after the refusal", c, rv)
		}
		if _, rs1, al1 := e.cofBalance(sku); rs1 != rs0 || al1 != al0 || e.rtAll() != ledger {
			t.Fatal("a refused cancel moved stock")
		}
	})

	var o1 string
	t.Run("RT06 a paid order needs refunds covering the capture first (409 refund_first); never an automatic refund; then it cancels", func(t *testing.T) {
		var rf rfxOrder
		o1, rf = e.rtPaid()
		oh0, rs0, al0 := e.cofBalance(sku)
		money, refunds, ledger := e.pgPaymentFingerprint(), e.count(`SELECT count(*) FROM payments.stripe_refunds WHERE store_id=$1`, e.store()), e.rtAll()
		st, out := cancel(e.token(), o1, "rt-can-1020", "CONFIRMED")
		rtExpect(t, "cancel, nothing refunded", st, out, 409, "refund_first")
		e.r.mustRefund(t, rf, 100, "requested_by_customer") // a partial refund is not enough
		refunds++
		st, out = cancel(e.token(), o1, "rt-can-1021", "CONFIRMED")
		rtExpect(t, "cancel, partially refunded", st, out, 409, "refund_first")
		if c, fu, rv := e.rtStates(o1); c != "CONFIRMED" || fu != "MANUAL_UNASSIGNED" || rv != "COMMITTED" || e.rtAll() != ledger {
			t.Fatalf("refused cancels changed the order (%s %s %s) or the ledger", c, fu, rv)
		}
		e.r.mustRefund(t, rf, rf.captured-100, "requested_by_customer") // the remainder: succeeded + in flight now equal the capture
		refunds++
		st, out = cancel(e.token(), o1, "rt-can-1022", "CONFIRMED")
		rtExpect(t, "cancel, fully refunded", st, out, 200, "")
		if out["commercial_state"] != "CANCELLED" || out["released_lines"] != float64(1) {
			t.Fatalf("cancel body %v", out)
		}
		if c, fu, rv := e.rtStates(o1); c != "CANCELLED" || fu != "CANCELLED" || rv != "RELEASED" {
			t.Fatalf("states after cancel %s %s %s", c, fu, rv)
		}
		oh1, rs1, al1 := e.cofBalance(sku)
		if oh1 != oh0 || rs1 != rs0 || al1 != al0-2 {
			t.Fatalf("balance %d,%d,%d -> %d,%d,%d: want allocated -2 only", oh0, rs0, al0, oh1, rs1, al1)
		}
		var kind, reason string
		if err := f.owner.QueryRow(context.Background(), `SELECT kind,reason FROM inventory.ledger WHERE checkout_id=$1 AND operation='checkout.merchant_cancel'`, o1).Scan(&kind, &reason); err != nil || kind != "DEALLOCATE" || reason != "merchant_cancel" {
			t.Fatalf("ledger row %s %s (%v)", kind, reason, err)
		}
		if got := e.pgPaymentFingerprint(); got != money {
			t.Fatalf("I05: the cancel changed payments/orders money %v -> %v", money, got)
		}
		if n := e.count(`SELECT count(*) FROM payments.stripe_refunds WHERE store_id=$1`, e.store()); n != refunds {
			t.Fatalf("refund rows %d, want %d: the cancel must never start a refund", n, refunds)
		}
		if n := e.count(`SELECT count(*) FROM notify.outbox WHERE order_id=$1 AND kind='cancelled'`, o1); n != 1 {
			t.Fatalf("buyer cancelled notice rows %d, want 1 (existing notify trigger)", n)
		}
		// the cancelled order can no longer ship, return or be cancelled again
		st, out, _ = e.mcall(e.token(), "PUT", e.rtPath("/orders/"+o1+"/shipment"), "rt-ship-1020", mfxShip(0, "black_cat", "RTC0001"))
		rtExpect(t, "ship a cancelled order", st, out, 422, "not_shippable")
		st, out = e.rtRegister(e.token(), o1, "rt-reg-1020", 1)
		rtExpect(t, "return a cancelled order", st, out, 409, "not_shipped")
		st, out = cancel(e.token(), o1, "rt-can-1023", "CONFIRMED")
		rtExpect(t, "cancel twice", st, out, 409, "already_cancelled")
	})

	t.Run("a shipped order is not cancellable (returns path); permissions are enforced", func(t *testing.T) {
		o2, rf := e.rtPaid()
		e.rtShip(o2)
		e.r.mustRefund(t, rf, rf.captured, "requested_by_customer")
		st, out := cancel(e.token(), o2, "rt-can-1030", "CONFIRMED")
		rtExpect(t, "cancel a shipped order", st, out, 409, "already_shipped")
		noWrite, _ := e.member("orders:read")
		o3, _ := e.rtPaid()
		st, out = cancel(noWrite, o3, "rt-can-1031", "CONFIRMED")
		rtExpect(t, "cancel without fulfillment:write", st, out, 403, "forbidden")
		st, out, _ = e.mcall(e.token(), "POST", e.rtPath("/orders/"+o3+"/cancel"), "rt-can-1032", `{"expected_state":"SHIPPED","reason":"x"}`)
		rtExpect(t, "bad expected state", st, out, 422, "invalid_request")
	})

	t.Run("concurrent cancels of one refunded order: one wins, stock released once", func(t *testing.T) {
		o4, rf := e.rtPaid()
		e.r.mustRefund(t, rf, rf.captured, "requested_by_customer")
		_, _, al0 := e.cofBalance(sku)
		var wg sync.WaitGroup
		codes := make([]int, 2)
		for i := range codes {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				st, _ := cancel(e.token(), o4, fmt.Sprintf("rt-can-par%d", i), "CONFIRMED")
				codes[i] = st
			}(i)
		}
		wg.Wait()
		if !(codes[0] == 200 && codes[1] == 409 || codes[0] == 409 && codes[1] == 200) {
			t.Fatalf("concurrent cancels answered %v", codes)
		}
		if n := e.count(`SELECT count(*) FROM inventory.ledger WHERE checkout_id=$1 AND operation='checkout.merchant_cancel'`, o4); n != 1 {
			t.Fatalf("cancel ledger rows %d", n)
		}
		if _, _, al1 := e.cofBalance(sku); al1 != al0-2 {
			t.Fatalf("allocated %d -> %d, want -2 once", al0, al1)
		}
	})

	t.Run("RT08 cancelling a member of an OPEN parcel group removes it; the last survivor dissolves the group", func(t *testing.T) {
		b := e.newBuyer()
		var ids []string
		var rfs []rfxOrder
		for i := 0; i < 3; i++ {
			id, rf := e.rtPaidOf(b)
			ids = append(ids, id)
			rfs = append(rfs, rf)
		}
		st, out, raw := e.mcall(e.token(), "POST", e.rtPath("/parcel-groups"), "rt-grp-0001", `{"order_ids":["`+strings.Join(ids, `","`)+`"]}`)
		if st != 201 {
			t.Fatalf("create group: %d %s", st, raw)
		}
		group := out["id"].(string)
		for _, rf := range rfs[:2] {
			e.r.mustRefund(t, rf, rf.captured, "requested_by_customer")
		}
		st, out = cancel(e.token(), ids[0], "rt-can-1040", "CONFIRMED")
		rtExpect(t, "cancel member 1 of 3", st, out, 200, "")
		pg, _ := out["parcel_group"].(map[string]any)
		if pg["id"] != group || pg["state"] != "OPEN" || pg["version"] != float64(2) {
			t.Fatalf("parcel_group %v", out["parcel_group"])
		}
		if n := e.count(`SELECT count(*) FROM fulfillment.parcel_group_orders WHERE group_id=$1`, group); n != 2 {
			t.Fatalf("group members %d, want 2", n)
		}
		if n := e.count(`SELECT count(*) FROM fulfillment.parcel_group_orders WHERE order_id=$1`, ids[0]); n != 0 {
			t.Fatal("the cancelled order is still a group member")
		}
		// a stale dissolve (version 1) is refused: the group changed
		st, out, _ = e.mcall(e.token(), "DELETE", e.rtPath("/parcel-groups/"+group+"?expected_version=1"), "", "")
		rtExpect(t, "dissolve with the old version", st, out, 409, "version_changed")
		st, out = cancel(e.token(), ids[1], "rt-can-1041", "CONFIRMED")
		rtExpect(t, "cancel member 2 of 3", st, out, 200, "")
		pg, _ = out["parcel_group"].(map[string]any)
		if pg["id"] != group || pg["state"] != "DISSOLVED" {
			t.Fatalf("parcel_group %v", out["parcel_group"])
		}
		if n := e.count(`SELECT count(*) FROM fulfillment.parcel_group_orders WHERE group_id=$1`, group); n != 0 {
			t.Fatalf("a dissolved group still has %d members", n)
		}
		var state string
		if err := f.owner.QueryRow(context.Background(), `SELECT state FROM fulfillment.parcel_groups WHERE id=$1`, group).Scan(&state); err != nil || state != "DISSOLVED" {
			t.Fatalf("group row %s (%v)", state, err)
		}
		// the survivor is free: it may ship alone again
		e.rtShip(ids[2])
		if c, fu, _ := e.rtStates(ids[2]); c != "CONFIRMED" || fu != "MERCHANT_SHIPPED" {
			t.Fatalf("survivor %s %s", c, fu)
		}
	})

	t.Run("a refused cancel leaves its parcel group untouched", func(t *testing.T) {
		b := e.newBuyer()
		x, _ := e.rtPaidOf(b)
		y, _ := e.rtPaidOf(b)
		st, out, raw := e.mcall(e.token(), "POST", e.rtPath("/parcel-groups"), "rt-grp-0002", `{"order_ids":["`+x+`","`+y+`"]}`)
		if st != 201 {
			t.Fatalf("create group: %d %s", st, raw)
		}
		st, out = cancel(e.token(), x, "rt-can-1050", "CONFIRMED") // not refunded
		rtExpect(t, "cancel, nothing refunded", st, out, 409, "refund_first")
		if n := e.count(`SELECT count(*) FROM fulfillment.parcel_group_orders WHERE group_id=$1`, rtGroupOf(e, x)); n != 2 {
			t.Fatalf("group members %d after a refused cancel", n)
		}
	})

	t.Run("RT10 cancelling a hold while its payment starts: exactly one wins, stock stays consistent", func(t *testing.T) {
		b := e.newBuyer()
		res, err := e.svc.Begin(context.Background(), b.cap.Token, e.store(), t04Key("rt10"), b.h.input)
		if err != nil {
			t.Fatal(err)
		}
		order := res.OrderID
		s := e.ro.s
		s.p.hold = res
		s.p.cap = b.cap
		_, rs0, al0 := e.cofBalance(sku)
		var wg sync.WaitGroup
		var cancelStatus int
		var startErr error
		wg.Add(2)
		go func() {
			defer wg.Done()
			cancelStatus, _ = cancel(e.token(), order, "rt-can-1060", "DRAFT")
		}()
		go func() {
			defer wg.Done()
			_, startErr = s.begin(e.r.svc, t04Key("rt10-pay"), s.input("zh-TW"))
		}()
		wg.Wait()
		c, _, rv := e.rtStates(order)
		_, rs1, al1 := e.cofBalance(sku)
		switch {
		case cancelStatus == 200 && startErr != nil:
			if c != "CANCELLED" || rv != "RELEASED" || rs1 != rs0-2 || al1 != al0 {
				t.Fatalf("cancel won but states %s %s balance reserved %d->%d", c, rv, rs0, rs1)
			}
		case cancelStatus == 409 && startErr == nil:
			if c != "AWAITING_PAYMENT" || rv != "PAYMENT_PENDING" || rs1 != rs0 || al1 != al0 {
				t.Fatalf("payment won but states %s %s balance reserved %d->%d", c, rv, rs0, rs1)
			}
		default:
			t.Fatalf("not exactly one winner: cancel %d, payment start error %v (order %s %s)", cancelStatus, startErr, c, rv)
		}
	})

	t.Run("a pay-at-pickup order cancels through the same command (existing §16.8 release)", func(t *testing.T) {
		e.hcodSettings(0, true, 20000, 0, "black_cat")
		b := e.newBuyer()
		e.hcodRehome(b)
		res, err := e.hcodPlace(b)
		if err != nil {
			t.Fatalf("COD order: %v", err)
		}
		_, _, al0 := e.cofBalance(sku)
		st, out := cancel(e.token(), res.OrderID, "rt-can-1070", "AWAITING_COLLECTION")
		rtExpect(t, "cancel a COD order", st, out, 200, "")
		if c, fu, rv := e.rtStates(res.OrderID); c != "CANCELLED" || fu != "CANCELLED" || rv != "RELEASED" {
			t.Fatalf("COD states %s %s %s", c, fu, rv)
		}
		if _, _, al1 := e.cofBalance(sku); al1 != al0-2 {
			t.Fatalf("allocated %d -> %d, want -2", al0, al1)
		}
		st, again := cancel(e.token(), res.OrderID, "rt-can-1070", "AWAITING_COLLECTION")
		if st != 200 || again["order_id"] != res.OrderID {
			t.Fatalf("replay: %d %v", st, again)
		}
		_ = o1
	})
}

// rtGroupOf returns the parcel group id of an order (disclosed owner read).
func rtGroupOf(e *tcvEnv, order string) string {
	var g string
	if err := e.p.f.owner.QueryRow(context.Background(), `SELECT group_id::text FROM fulfillment.parcel_group_orders WHERE order_id=$1`, order).Scan(&g); err != nil && err != pgx.ErrNoRows {
		e.t.Fatal(err)
	}
	return g
}
