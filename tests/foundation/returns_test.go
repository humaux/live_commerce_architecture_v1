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
	"os"
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
		st, out := e.rtClose(e.token(), id, "rt-cls-0030", v, "99999999-9999-4999-8999-999999999999")
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

	t.Run("P1-1 a shipment cannot be voided while a live RMA exists (a return presumes the parcel left)", func(t *testing.T) {
		ov, _ := e.rtPaid()
		e.rtShip(ov)
		st, out := e.rtRegister(e.token(), ov, "rt-reg-0060", 1)
		rtExpect(t, "register", st, out, 201, "")
		id := out["id"].(string)
		st, out, _ = e.mcall(e.token(), "PUT", e.rtPath("/orders/"+ov+"/shipment"), "rt-void-0001", mfxVoid(1, "wrong_order"))
		rtExpect(t, "void with a live RMA", st, out, 409, "has_returns")
		if c, fu, _ := e.rtStates(ov); c != "CONFIRMED" || fu != "MERCHANT_SHIPPED" {
			t.Fatalf("a refused void changed the order to %s %s", c, fu)
		}
		st, out, _ = e.mcall(e.token(), "POST", e.rtPath("/returns/"+id+"/cancel"), "rt-can-0060", `{"expected_version":1}`)
		rtExpect(t, "withdraw the RMA", st, out, 200, "")
		st, out, _ = e.mcall(e.token(), "PUT", e.rtPath("/orders/"+ov+"/shipment"), "rt-void-0002", mfxVoid(1, "wrong_order"))
		rtExpect(t, "void once no live RMA is left", st, out, 200, "")
		if _, fu, _ := e.rtStates(ov); fu != "MANUAL_UNASSIGNED" {
			t.Fatalf("voided order is %s", fu)
		}
	})

	t.Run("P1-2 a collected home-COD order can be returned (commercial_state stays AWAITING_COLLECTION); uncollected is refused", func(t *testing.T) {
		e.hcrEnable(0)
		cod, _ := e.hcrPlace()
		e.hcrShip(cod)
		st, out := e.rtRegister(e.token(), cod, "rt-reg-0070", 2)
		rtExpect(t, "uncollected COD", st, out, 409, "not_returnable")
		writer, _ := e.member("fulfillment:write", "orders:read")
		if r := e.hcrRecord(writer, cod, "PENDING", "collected"); r.status != 200 {
			t.Fatalf("collect: %d %s", r.status, r.raw)
		}
		if c, _, _ := e.rtStates(cod); c != "AWAITING_COLLECTION" {
			t.Fatalf("COD commercial_state %s after collection, the fixture premise changed", c)
		}
		_, _, al0 := e.cofBalance(sku)
		id, v := e.rtInspected(cod, 2, "rt-cod")
		st, out = e.rtClose(e.token(), id, "rt-cls-0070", v, "")
		rtExpect(t, "close COD return", st, out, 200, "")
		if out["restocked_units"] != float64(2) || e.rtLedger("returns.rma.restock") < 1 {
			t.Fatalf("close body %v", out)
		}
		if _, _, al1 := e.cofBalance(sku); al1 != al0-2 {
			t.Fatalf("allocated %d -> %d, want -2 once", al0, al1)
		}
		st, out = e.rtClose(e.token(), id, "rt-cls-0070", v, "")
		if st != 200 || e.count(`SELECT count(*) FROM inventory.ledger WHERE operation='returns.rma.restock' AND checkout_id=$1`, cod) != 1 {
			t.Fatalf("COD close replay: %d %v", st, out)
		}
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
		// 4) a second, fully legitimate-looking CLOSED RMA (forged by the owner) for units the first RMA already released: every provenance
		// condition holds, ONLY the remaining-allocation bound refuses it, with its own message.
		o10, _ := e.rtPaid()
		e.rtShip(o10)
		id1, v1 := e.rtInspected(o10, 2, "rt-fb")
		if st, out := e.rtClose(e.token(), id1, "rt-cls-fb01", v1, ""); st != 200 {
			t.Fatalf("close the first RMA: %d %v", st, out)
		}
		var owner10, session10 string
		if err := f.owner.QueryRow(context.Background(), `SELECT owner_id::text,creator_session_id::text FROM checkout.orders WHERE id=$1`, o10).Scan(&owner10, &session10); err != nil {
			t.Fatal(err)
		}
		rma2 := randomUUID()
		mustExec(t, f.owner, `INSERT INTO returns.rmas(tenant_id,store_id,id,owner_id,order_id,state,reason,created_by,received_by,received_at,inspected_by,inspected_at,closed_by,closed_at)
		 VALUES($1,$2,$3,$4,$5,'CLOSED','forged',$6,$6,now(),$6,now(),$6,now())`, f.tenantA, f.storeA1, rma2, owner10, o10, f.principalA)
		mustExec(t, f.owner, `INSERT INTO returns.rma_lines(tenant_id,store_id,rma_id,order_id,warehouse_id,sku_id,qty_registered,qty_received,qty_restock,qty_scrap)
		 VALUES($1,$2,$3,$4,$5,$6,2,2,2,0)`, f.tenantA, f.storeA1, rma2, o10, wh, sku)
		_, _, alBefore := e.cofBalance(sku)
		tx, err := f.owner.Begin(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(context.Background())
		for k, v := range map[string]string{"app.tenant_id": f.tenantA, "app.store_id": f.storeA1, "app.principal_id": f.principalA, "app.buyer_id": owner10, "app.buyer_session_id": session10} {
			if _, err := tx.Exec(context.Background(), `SELECT set_config($1,$2,true)`, k, v); err != nil {
				t.Fatal(err)
			}
		}
		sp, _ := tx.Begin(context.Background())
		_, ferr := sp.Exec(context.Background(), `INSERT INTO inventory.ledger(tenant_id,store_id,warehouse_id,sku_id,kind,delta_allocated,operation,command_key,reservation_id,reason,principal_id,checkout_id,buyer_owner_id,buyer_session_id,actor_kind)
		 VALUES($1,$2,$3,$4,'DEALLOCATE',-2,'returns.rma.restock',$5::text,$6::uuid,'rma_restock',$7,$6::uuid,$8,$9,'MERCHANT')`, f.tenantA, f.storeA1, wh, sku, rma2, o10, f.principalA, owner10, session10)
		_ = sp.Rollback(context.Background())
		if sqlState(ferr) != "42501" || ferr == nil || !strings.Contains(ferr.Error(), "deallocate exceeds the remaining allocation of the order line") {
			t.Fatalf("second RMA restock of the same units: %v, want 42501 'deallocate exceeds the remaining allocation of the order line'", ferr)
		}
		if _, _, al := e.cofBalance(sku); al != alBefore {
			t.Fatalf("allocated %d -> %d after the refused second restock", alBefore, al)
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

// TestReturnsBankTransfer: P1-1 root two (one remaining-allocation bound for EVERY DEALLOCATE writer) and the contract's
// AWAITING_TRANSFER refusal, on bank-transfer orders.
func TestReturnsBankTransfer(t *testing.T) {
	e := cogNew(t, 72)
	e.grantCreator("inventory:write")
	f := e.p.f
	sku, _ := e.rtSKU()

	t.Run("AWAITING_TRANSFER is 422 not_cancellable (the expiry or the offline refund ends it)", func(t *testing.T) {
		_, pending := e.cogPlace("")
		st, out, _ := e.mcall(e.token(), "POST", e.rtPath("/orders/"+pending+"/cancel"), "rt-can-2001", `{"expected_state":"AWAITING_TRANSFER","reason":"x"}`)
		rtExpect(t, "cancel AWAITING_TRANSFER", st, out, 422, "not_cancellable")
	})

	t.Run("RMA restock then an offline refund WITH restock: the second release is refused, no phantom stock", func(t *testing.T) {
		_, holder := e.cokConfirmed(true) // another order holding the same SKU: the balance CHECK alone would not catch a double release
		_, order := e.cokConfirmed(true)
		_ = holder
		e.rtShip(order)
		id, v := e.rtInspected(order, 2, "rt-bank")
		st, out := e.rtClose(e.token(), id, "rt-cls-2001", v, "")
		rtExpect(t, "close", st, out, 200, "")
		st, out, _ = e.mcall(e.token(), "PUT", e.rtPath("/orders/"+order+"/shipment"), "rt-void-2001", mfxVoid(1, "wrong_order"))
		rtExpect(t, "void after a CLOSED RMA", st, out, 409, "has_returns")
		// Disclosed owner fixture: bypass the void guard (the order row alone is rewritten; the head/state constraint trigger is
		// disabled for this one statement) to prove the LEDGER refuses the second release on its own.
		tx, err := f.owner.Begin(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		for _, stmt := range []string{`ALTER TABLE checkout.orders DISABLE TRIGGER manual_shipment_state_orders`,
			`UPDATE checkout.orders SET fulfillment_state='MANUAL_UNASSIGNED' WHERE id='` + order + `'`,
			`ALTER TABLE checkout.orders ENABLE TRIGGER manual_shipment_state_orders`} {
			if _, err := tx.Exec(context.Background(), stmt); err != nil {
				t.Fatalf("%s: %v", stmt, err)
			}
		}
		if err := tx.Commit(context.Background()); err != nil {
			t.Fatal(err)
		}
		oh0, rs0, al0 := e.cofBalance(sku)
		rows := e.count(`SELECT count(*) FROM inventory.ledger WHERE checkout_id=$1 AND kind='DEALLOCATE'`, order)
		st, out = e.cofDecide(e.token(), order, "refund-offline", t04Key("rt-bank-restock"), `{"restock":true}`)
		// The bank definer raises the guard's SQLSTATE 42501 ("deallocate exceeds the remaining allocation of the order line"); the offline
		// route maps an unclassified definer failure to 503 unavailable (never a 200, never a 4xx the merchant could mistake for a rule).
		if st != 503 || out["code"] != "unavailable" {
			t.Fatalf("offline refund with restock after an RMA restock: got %d %v, want 503 unavailable (guard 42501)", st, out)
		}
		if oh, rs, al := e.cofBalance(sku); oh != oh0 || rs != rs0 || al != al0 {
			t.Fatalf("balance %d,%d,%d -> %d,%d,%d after the refused second release", oh0, rs0, al0, oh, rs, al)
		}
		if n := e.count(`SELECT count(*) FROM inventory.ledger WHERE checkout_id=$1 AND kind='DEALLOCATE'`, order); n != rows || rows != 1 {
			t.Fatalf("DEALLOCATE rows %d -> %d, want exactly the RMA row", rows, n)
		}
	})
}

// rtHelperSigs are the internal helpers of 0155: nobody but their owner (a definer of the same role) may EXECUTE them.
var rtHelperSigs = []string{"returns.authorize(bytea,uuid,text,text)", "returns.reauthorize(bytea,uuid,text,text,jsonb)", "returns.line_set(jsonb,text,text)",
	"returns.rma_json(uuid,uuid,uuid)", "returns.order_returnable(text,text,text)", "inventory.guard_returns_ledger()"}

func TestReturnsHelperACLAndGuardBodies(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	for _, sig := range rtHelperSigs {
		var owner string
		var public bool
		var callers []string
		if err := f.owner.QueryRow(ctx, `SELECT pg_get_userbyid(p.proowner),
		 EXISTS(SELECT 1 FROM aclexplode(coalesce(p.proacl,acldefault('f',p.proowner))) a WHERE a.grantee=0 AND a.privilege_type='EXECUTE'),
		 ARRAY(SELECT rolname FROM pg_roles WHERE rolname LIKE 'commerce\_%' AND rolname<>pg_get_userbyid(p.proowner) AND has_function_privilege(oid,p.oid,'EXECUTE') ORDER BY 1)
		 FROM pg_proc p WHERE p.oid=to_regprocedure($1)`, sig).Scan(&owner, &public, &callers); err != nil {
			t.Fatalf("%s: %v", sig, err)
		}
		if owner != "commerce_checkout_writer" || public || len(callers) != 0 {
			t.Errorf("%s: owner=%s public-exec=%v extra callers=%v, want an owner-only helper", sig, owner, public, callers)
		}
		if !srsBoolPG(t, f, `SELECT obj_description(to_regprocedure($1),'pg_proc') IS NOT NULL`, sig) {
			t.Errorf("%s has no COMMENT ON FUNCTION", sig)
		}
	}
	// The anchor-patched guards must keep carrying the 0155 branches (a later migration that copies an older body would drop them).
	for fn, needles := range map[string][]string{
		"inventory.guard_checkout_ledger()":      {"checkout.merchant_cancel"},
		"inventory.guard_pay_at_pickup_ledger()": {"fulfillment.pay_at_pickup.cancel", "fulfillment.pay_at_pickup.restock"},
		"fulfillment.record_manual_shipment(bytea,uuid,uuid,text,bytea,bigint,text,text,text,text,text,text,text)": {"has_returns"},
		"inventory.guard_returns_ledger()": {"returns.rma.restock", "checkout.merchant_cancel", "v_alloc<-NEW.delta_allocated", "deallocate exceeds the remaining allocation of the order line", "a.kind IN ('ALLOCATE','DEALLOCATE')"},
	} {
		var src string
		if err := f.owner.QueryRow(ctx, `SELECT prosrc FROM pg_proc WHERE oid=to_regprocedure($1)`, fn).Scan(&src); err != nil {
			t.Fatalf("%s: %v", fn, err)
		}
		for _, n := range needles {
			if !strings.Contains(src, n) {
				t.Errorf("%s lost %q", fn, n)
			}
		}
	}
}

func srsBoolPG(t *testing.T, f *testFixture, q string, args ...any) bool {
	t.Helper()
	var b bool
	if err := f.owner.QueryRow(context.Background(), q, args...).Scan(&b); err != nil {
		t.Fatal(err)
	}
	return b
}

// TestMerchantCancelRefundGap: a cancel counts in-flight refunds; when such a refund later FAILS the order stays cancelled and the
// merchant sees it in GET /orders/cancel-refund-gaps until the money is refunded again (P2-2).
func TestMerchantCancelRefundGap(t *testing.T) {
	e := tcvNew(t, tcvOpts{stripe: true})
	e.r.startWorker(t)
	e.grantCreator("orders:read", "fulfillment:write", "inventory:write")
	order, rf := e.rtPaid()
	gaps := func() []any {
		st, out, raw := e.mcall(e.token(), "GET", e.rtPath("/orders/cancel-refund-gaps"), "", "")
		if st != 200 {
			t.Fatalf("gaps: %d %s", st, raw)
		}
		items, _ := out["items"].([]any)
		return items
	}
	refund := e.r.mustRefund(t, rf, rf.captured, "requested_by_customer")
	st, out, _ := e.mcall(e.token(), "POST", e.rtPath("/orders/"+order+"/cancel"), "rt-can-3001", `{"expected_state":"CONFIRMED","reason":"customer asked"}`)
	rtExpect(t, "cancel with the refund requested", st, out, 200, "")
	if n := len(gaps()); n != 0 {
		t.Fatalf("gap list has %d rows while the refund is healthy", n)
	}
	e.r.awaitRefundFact(t, refund, rf.attempt, "SUCCEEDED")
	fakeID := e.r.fake.RefundByRef(refund)
	e.r.fake.SetRefundStatus(fakeID, "failed", "declined")
	body := e.r.fake.RefundEventBody("evt_rt_failed_"+t04Tag(), "refund.failed", fakeID, false)
	if status := e.r.deliverRaw(t, rf.endpoint, rf.secret, body); status != 200 {
		t.Fatalf("refund.failed webhook answered %d", status)
	}
	e.r.awaitRefundFact(t, refund, rf.attempt, "FAILED")
	items := gaps()
	if len(items) != 1 {
		t.Fatalf("gap list after the refund failed: %v", items)
	}
	g := items[0].(map[string]any)
	if g["order_id"] != order || g["captured_minor"] != float64(rf.captured) || g["refunded_minor"] != float64(0) || g["gap_minor"] != float64(rf.captured) || g["reason"] != "cancel_refund_failed" {
		t.Fatalf("gap row %v", g)
	}
	if c, _, rv := e.rtStates(order); c != "CANCELLED" || rv != "RELEASED" {
		t.Fatalf("order %s %s: the failed refund must not reopen it", c, rv)
	}
	e.r.mustRefund(t, rf, rf.captured, "requested_by_customer") // the merchant refunds again on the cancelled order
	if n := len(gaps()); n != 0 {
		t.Fatalf("gap list still has %d rows after the re-refund", n)
	}
	noRead, _ := e.member("fulfillment:write")
	if st, out, _ := e.mcall(noRead, "GET", e.rtPath("/orders/cancel-refund-gaps"), "", ""); st != 403 {
		t.Fatalf("gap list without orders:read: %d %v", st, out)
	}
}

// TestMerchantCancelGroupRace: an order that joins a parcel group between the cancel's unlocked membership read and its order lock
// is answered 503 retry_later BEFORE any write (the group lock must never be taken after the order lock: begin_parcel_group_shipment
// locks group -> order); the retry locks group -> order and succeeds (P2-3).
func TestMerchantCancelGroupRace(t *testing.T) {
	e := tcvNew(t, tcvOpts{stripe: true})
	e.r.startWorker(t)
	e.grantCreator("orders:read", "fulfillment:write", "inventory:write")
	f := e.p.f
	ctx := context.Background()
	order, rf := e.rtPaid()
	e.r.mustRefund(t, rf, rf.captured, "requested_by_customer")
	var owner string
	if err := f.owner.QueryRow(ctx, `SELECT owner_id::text FROM checkout.orders WHERE id=$1`, order).Scan(&owner); err != nil {
		t.Fatal(err)
	}
	tx, err := f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT 1 FROM checkout.orders WHERE id=$1 FOR UPDATE`, order); err != nil {
		t.Fatal(err)
	}
	type res struct {
		st  int
		out map[string]any
	}
	done := make(chan res, 1)
	go func() {
		st, out, _ := e.mcall(e.token(), "POST", e.rtPath("/orders/"+order+"/cancel"), "rt-can-3010", `{"expected_state":"CONFIRMED","reason":"x"}`)
		done <- res{st, out}
	}()
	deadline := time.Now().Add(20 * time.Second)
	for e.count(`SELECT count(*) FROM pg_stat_activity WHERE wait_event_type='Lock' AND query LIKE '%merchant_cancel_order%'`) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the cancel never blocked on the order lock")
		}
		time.Sleep(50 * time.Millisecond)
	}
	group := randomUUID()
	// the creator of a group commits its membership while still holding the order lock
	if _, err := tx.Exec(ctx, `INSERT INTO fulfillment.parcel_groups(tenant_id,store_id,id,owner_id,destination_hash,state,created_by) VALUES($1,$2,$3,$4,sha256('rt'::bytea),'OPEN',$5)`,
		e.tenant(), e.store(), group, owner, f.principalA); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO fulfillment.parcel_group_orders(tenant_id,store_id,group_id,owner_id,order_id) VALUES($1,$2,$3,$4,$5)`, e.tenant(), e.store(), group, owner, order); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	r := <-done
	rtExpect(t, "cancel that raced a group creation", r.st, r.out, 503, "retry_later")
	if c, _, rv := e.rtStates(order); c != "CONFIRMED" || rv != "COMMITTED" {
		t.Fatalf("the refused attempt changed the order to %s %s", c, rv)
	}
	st, out, _ := e.mcall(e.token(), "POST", e.rtPath("/orders/"+order+"/cancel"), "rt-can-3011", `{"expected_state":"CONFIRMED","reason":"x"}`)
	rtExpect(t, "retry", st, out, 200, "")
	if pg, _ := out["parcel_group"].(map[string]any); pg["id"] != group || pg["state"] != "DISSOLVED" {
		t.Fatalf("parcel_group %v", out["parcel_group"])
	}
}

// TestMerchantCancelClosesWorkItem is the 0162 root-cause gate (unit cancel-closes-work-item, returns-v1 §3 Amendment 0162): a merchant
// cancel CLOSES the order's READY payment work item in the same transaction, so the merchant projection shows work_state=NONE for every
// cancelled order — including the fully refunded one that kept work_state=READY on trunk (the W3-08B symptom the relaxed Go/TS validators
// only papered over). Money that still needs a human after a cancel is the gap list's job (fact-based, never reads the work item): an
// in-flight refund counted at cancel time that later FAILS keeps the order in GET /orders/cancel-refund-gaps. The backfill subtests re-run
// the 0162 file (the 0117 runBackfill pattern) over fabricated legacy rows: every READY row of a CANCELLED order closes (open gap
// included — the gap list stays), a REVIEW_REQUIRED row and a CONFIRMED order's READY row survive; the DO block itself refuses a role
// without RLS bypass and fails loudly when a delete is suppressed.
func TestMerchantCancelClosesWorkItem(t *testing.T) {
	e := tcvNew(t, tcvOpts{stripe: true})
	e.r.startWorker(t)
	e.grantCreator("orders:read", "fulfillment:write", "inventory:write")

	// workRows counts the durable data under the projection; projWorkState is what the merchant actually sees (GET detail).
	workRows := func(order string) int {
		return e.count(`SELECT count(*) FROM fulfillment.payment_work_items WHERE order_id=$1`, order)
	}
	projWorkState := func(order string) string {
		t.Helper()
		st, out, raw := e.mcall(e.token(), "GET", e.rtPath("/orders/"+order), "", "")
		if st != 200 {
			t.Fatalf("order detail answered %d: %s", st, raw)
		}
		ws, _ := out["work_state"].(string)
		return ws
	}
	cancelConfirmed := func(order, key string) (int, map[string]any) {
		st, out, _ := e.mcall(e.token(), "POST", e.rtPath("/orders/"+order+"/cancel"), key, `{"expected_state":"CONFIRMED","reason":"customer asked"}`)
		return st, out
	}
	// failRefund walks an existing refund to a FAILED fact through the real webhook path (the TestMerchantCancelRefundGap recipe).
	failRefund := func(rf rfxOrder, refund, tag string) {
		t.Helper()
		e.r.awaitRefundFact(t, refund, rf.attempt, "SUCCEEDED")
		fakeID := e.r.fake.RefundByRef(refund)
		e.r.fake.SetRefundStatus(fakeID, "failed", "declined")
		body := e.r.fake.RefundEventBody("evt_cw_"+tag+"_"+t04Tag(), "refund.failed", fakeID, false)
		if status := e.r.deliverRaw(t, rf.endpoint, rf.secret, body); status != 200 {
			t.Fatalf("refund.failed webhook answered %d", status)
		}
		e.r.awaitRefundFact(t, refund, rf.attempt, "FAILED")
	}
	gapOf := func(order string) map[string]any {
		t.Helper()
		st, out, raw := e.mcall(e.token(), "GET", e.rtPath("/orders/cancel-refund-gaps"), "", "")
		if st != 200 {
			t.Fatalf("gap list answered %d: %s", st, raw)
		}
		items, _ := out["items"].([]any)
		for _, it := range items {
			if m, _ := it.(map[string]any); m != nil && m["order_id"] == order {
				return m
			}
		}
		return nil
	}

	t.Run("a never-paid hold cancels to work_state NONE", func(t *testing.T) {
		b := e.newBuyer()
		res, err := e.svc.Begin(context.Background(), b.cap.Token, e.store(), t04Key("cw-begin-np"), b.h.input)
		if err != nil {
			t.Fatal(err)
		}
		st, out, _ := e.mcall(e.token(), "POST", e.rtPath("/orders/"+res.OrderID+"/cancel"), "cw-can-np01", `{"expected_state":"DRAFT","reason":"customer asked"}`)
		rtExpect(t, "cancel the unpaid hold", st, out, 200, "")
		if n := workRows(res.OrderID); n != 0 {
			t.Fatalf("an unpaid hold must never own a work item, found %d", n)
		}
		if ws := projWorkState(res.OrderID); ws != "NONE" {
			t.Fatalf("never-paid cancel projects work_state=%s, want NONE", ws)
		}
	})

	t.Run("cancelling a fully refunded paid order closes the work item: work_state NONE (0162)", func(t *testing.T) {
		order, rf := e.rtPaid()
		if n := workRows(order); n != 1 {
			t.Fatalf("capture must leave exactly one work item, found %d", n)
		}
		if ws := projWorkState(order); ws != "READY" {
			t.Fatalf("paid unshipped order projects work_state=%s, want READY", ws)
		}
		refund := e.r.mustRefund(t, rf, rf.captured, "requested_by_customer")
		e.r.awaitRefundFact(t, refund, rf.attempt, "SUCCEEDED")
		st, out := cancelConfirmed(order, "cw-can-full01")
		rtExpect(t, "cancel the fully refunded order", st, out, 200, "")
		if n := workRows(order); n != 0 {
			t.Fatalf("0162: the cancel left %d work item rows — the projection would keep work_state=READY on a CANCELLED order", n)
		}
		st, det, raw := e.mcall(e.token(), "GET", e.rtPath("/orders/"+order), "", "")
		if st != 200 || det["commercial_state"] != "CANCELLED" || det["work_state"] != "NONE" || det["payment_state"] != "REFUNDED" {
			t.Fatalf("cancelled+refunded detail %d: %s", st, raw)
		}
		if g := gapOf(order); g != nil {
			t.Fatalf("a fully refunded cancel is not a gap: %v", g)
		}
	})

	t.Run("a refund in flight at cancel time closes the work item; its later failure stays visible in the gap list", func(t *testing.T) {
		order, rf := e.rtPaid()
		refund := e.r.mustRefund(t, rf, rf.captured, "requested_by_customer")
		st, out := cancelConfirmed(order, "cw-can-infl01") // held includes the in-flight refund (returns-v1 §3)
		rtExpect(t, "cancel with the refund in flight", st, out, 200, "")
		if n := workRows(order); n != 0 {
			t.Fatalf("0162: the cancel left %d work item rows while the refund was in flight", n)
		}
		failRefund(rf, refund, "inflight")
		// The gap list — not the work item — is the surface where this money needs a human (Amendment 0162 ruling).
		g := gapOf(order)
		if g == nil {
			t.Fatal("the failed refund's order is missing from the gap list")
		}
		if g["reason"] != "cancel_refund_failed" || g["gap_minor"] != float64(rf.captured) || g["refunded_minor"] != float64(0) {
			t.Fatalf("gap row %v", g)
		}
		if ws := projWorkState(order); ws != "NONE" {
			t.Fatalf("gap order projects work_state=%s; the work item stays closed and the gap list carries the work", ws)
		}
		e.r.mustRefund(t, rf, rf.captured, "requested_by_customer") // the merchant refunds again on the cancelled order
		if g := gapOf(order); g != nil {
			t.Fatalf("the re-refund must clear the gap list row: %v", g)
		}
	})

	// legacyCancel fabricates a pre-0162 cancelled order faithfully: cancel through the REAL definer — a pre-0162 cancel wrote the same
	// CANCELLED/CANCELLED flip and the same checkout.merchant_cancel DEALLOCATE rows the gap list keys on — then re-insert the work item
	// row the pre-0162 definer left behind (0018 shape; 0162 deletes it, nothing else differs). afterCancel (optional) runs between the
	// two, so a refund can fail AFTER the cancel counted it without the re-inserted row existing yet.
	legacyCancel := func(order string, rf rfxOrder, key string, afterCancel func()) {
		t.Helper()
		st, out := cancelConfirmed(order, key)
		rtExpect(t, "cancel legacy fixture", st, out, 200, "")
		if afterCancel != nil {
			afterCancel()
		}
		mustExec(t, e.p.f.owner, `INSERT INTO fulfillment.payment_work_items(tenant_id,store_id,owner_id,order_id,attempt_id,capture_kind,state)
			SELECT o.tenant_id,o.store_id,o.owner_id,o.id,$2::uuid,'CAPTURED','READY' FROM checkout.orders o WHERE o.id=$1`, order, rf.attempt)
		if n := workRows(order); n != 1 {
			t.Fatalf("legacy fixture %s: want the re-inserted READY row the old definer left, found %d", order, n)
		}
	}
	workState := func(order string) string {
		var s string
		if err := e.p.f.owner.QueryRow(context.Background(), `SELECT state FROM fulfillment.payment_work_items WHERE order_id=$1`, order).Scan(&s); err != nil {
			t.Fatalf("work item state of %s: %v", order, err)
		}
		return s
	}

	t.Run("the 0162 backfill closes every legacy cancelled READY row (open gap included) and spares live and review rows", func(t *testing.T) {
		orderA, rfA := e.rtPaid() // A: refund SUCCEEDED before the cancel, no gap -> closed
		refundA := e.r.mustRefund(t, rfA, rfA.captured, "requested_by_customer")
		e.r.awaitRefundFact(t, refundA, rfA.attempt, "SUCCEEDED")
		orderB, rfB := e.rtPaid() // B: refund in flight at cancel time, FAILED afterwards -> outstanding gap -> ALSO closed (ruling: close all)
		refundB := e.r.mustRefund(t, rfB, rfB.captured, "requested_by_customer")
		orderC, rfC := e.rtPaid() // C: cancelled, but its work row is REVIEW_REQUIRED -> negative control for the state filter
		refundC := e.r.mustRefund(t, rfC, rfC.captured, "requested_by_customer")
		e.r.awaitRefundFact(t, refundC, rfC.attempt, "SUCCEEDED")
		orderD, _ := e.rtPaid() // D: still CONFIRMED with its READY row -> negative control for the order-state filter
		legacyCancel(orderA, rfA, "cw-can-lgA01", nil)
		// held counts the in-flight refund (returns-v1 §3); B's counted refund then fails after the cancel: the outstanding gap
		legacyCancel(orderB, rfB, "cw-can-lgB01", func() { failRefund(rfB, refundB, "legacy") })
		legacyCancel(orderC, rfC, "cw-can-lgC01", nil)
		mustExec(t, e.p.f.owner, `UPDATE fulfillment.payment_work_items SET state='REVIEW_REQUIRED' WHERE order_id=$1`, orderC)
		if workRows(orderD) != 1 || workState(orderD) != "READY" || e.count(`SELECT count(*) FROM checkout.orders WHERE id=$1 AND commercial_state='CONFIRMED'`, orderD) != 1 {
			t.Fatal("control fixture D must be a CONFIRMED order with one READY row before the backfill")
		}
		runCancelClosesWorkItemMigration(t, e)
		for _, o := range []struct{ name, order string }{{"refunded", orderA}, {"open-gap", orderB}} {
			if n := workRows(o.order); n != 0 {
				t.Fatalf("backfill left %d rows on the %s legacy cancel (ruling: every READY row of a CANCELLED order closes)", n, o.name)
			}
			if ws := projWorkState(o.order); ws != "NONE" {
				t.Fatalf("%s legacy cancel projects work_state=%s after the backfill, want NONE", o.name, ws)
			}
		}
		// The gap stays visible through the fact-based list, which never reads the work item.
		if g := gapOf(orderB); g == nil || g["reason"] != "cancel_refund_failed" {
			t.Fatalf("the legacy gap row must stay listed after its work item closes: %v", g)
		}
		if g := gapOf(orderA); g != nil {
			t.Fatalf("the refunded legacy cancel is not a gap: %v", g)
		}
		// Negative controls: the backfill's predicate is CANCELLED order AND READY row — nothing else is touched.
		if workRows(orderC) != 1 || workState(orderC) != "REVIEW_REQUIRED" {
			t.Fatalf("backfill touched the REVIEW_REQUIRED row of a cancelled order (rows=%d state=%q)", workRows(orderC), workState(orderC))
		}
		if workRows(orderD) != 1 || workState(orderD) != "READY" {
			t.Fatalf("backfill touched the READY row of a CONFIRMED order (rows=%d)", workRows(orderD))
		}
		if ws := projWorkState(orderD); ws != "READY" {
			t.Fatalf("live paid order projects work_state=%s after the backfill, want READY", ws)
		}
		runCancelClosesWorkItemMigration(t, e) // re-run: idempotent, and its count assertion must hold at zero
		if workRows(orderA) != 0 || workRows(orderB) != 0 || workRows(orderC) != 1 || workRows(orderD) != 1 {
			t.Fatal("the backfill re-run changed the outcome")
		}
	})

	t.Run("the backfill block refuses to run without RLS bypass and fails loudly when a delete is suppressed", func(t *testing.T) {
		orderE, rfE := e.rtPaid()
		refundE := e.r.mustRefund(t, rfE, rfE.captured, "requested_by_customer")
		e.r.awaitRefundFact(t, refundE, rfE.attempt, "SUCCEEDED")
		legacyCancel(orderE, rfE, "cw-can-lgE01", nil)
		// FORCE RLS hides every row from a role without BYPASSRLS, so count and delete would both see zero and "succeed": the block must
		// refuse such a role up front instead (the SET LOCAL ROLE'd tx is always rolled back).
		err := runCancelClosesWorkItemBackfill(t, e, `SET LOCAL ROLE commerce_checkout_writer`)
		if err == nil || !strings.Contains(err.Error(), "0162 backfill:") || !strings.Contains(err.Error(), "BYPASSRLS") {
			t.Fatalf("backfill as a NOBYPASSRLS role must raise the RLS-bypass error, got %v", err)
		}
		if workRows(orderE) != 1 {
			t.Fatalf("the refused backfill changed data: %d rows", workRows(orderE))
		}
		// A BEFORE DELETE trigger that swallows the delete leaves the expected row in place: the pre-counted expectation must not match
		// the DELETE's own row count, and the whole block (tx) must fail.
		err = runCancelClosesWorkItemBackfill(t, e, `CREATE FUNCTION public.zz_cw_swallow() RETURNS trigger LANGUAGE plpgsql AS $f$ BEGIN RETURN NULL; END $f$;
			CREATE TRIGGER zz_cw_swallow BEFORE DELETE ON fulfillment.payment_work_items FOR EACH ROW EXECUTE FUNCTION public.zz_cw_swallow()`)
		if err == nil || !strings.Contains(err.Error(), "0162 backfill:") || !strings.Contains(err.Error(), "expected") {
			t.Fatalf("a suppressed delete must fail the backfill with the count-mismatch error, got %v", err)
		}
		if workRows(orderE) != 1 {
			t.Fatalf("the failed backfill changed data: %d rows", workRows(orderE))
		}
		runCancelClosesWorkItemMigration(t, e) // no trigger now (rolled back): closes E
		if workRows(orderE) != 0 {
			t.Fatalf("the clean backfill left %d rows on E", workRows(orderE))
		}
	})
}

// runCancelClosesWorkItemMigration re-executes the whole 0162 file as the migration owner (the 0117 runBackfill pattern): GRANT and
// CREATE OR REPLACE are idempotent, and the backfill DO block's count assertion must also hold on an already-closed data set.
func runCancelClosesWorkItemMigration(t *testing.T, e *tcvEnv) {
	t.Helper()
	body, err := os.ReadFile("../../migrations/0162_cancel_closes_work_item.sql")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := e.p.f.owner.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(context.Background(), string(body)); err != nil {
		t.Fatalf("apply 0162 again: %v", err)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
}

// runCancelClosesWorkItemBackfill executes ONLY the 0162 backfill DO block (section 3) as the migration owner after the given setup
// statements (e.g. SET LOCAL ROLE, a fault-injecting trigger) inside a transaction that is ALWAYS rolled back, and returns its error.
func runCancelClosesWorkItemBackfill(t *testing.T, e *tcvEnv, setup string) error {
	t.Helper()
	body, err := os.ReadFile("../../migrations/0162_cancel_closes_work_item.sql")
	if err != nil {
		t.Fatal(err)
	}
	i := strings.Index(string(body), "\nDO $$")
	if i < 0 {
		t.Fatal("0162: backfill DO block not found")
	}
	tx, err := e.p.f.owner.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	_, err = tx.Exec(context.Background(), setup+";\n"+string(body)[i:])
	return err
}
