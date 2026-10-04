package foundation_test

// K3-01..K3-04 (integrator rulings on the K3 review of migration 0088; fixed by 0099). REAL_PG, real merchant/buyer HTTP handlers; no PSP on this
// path, so every passing line is MOCK/REAL_PG (nothing SANDBOX or LIVE). Prefix `cok`.
//   K3-01 TestBankTransferK301SameKeyAcrossOrders   one Idempotency-Key reused across two orders serializes: second caller is 409, never a 5xx
//   K3-02 TestBankTransferK302RefundRestock         offline refund with restock=true releases the allocation in the refund transaction; refused after shipment
//   K3-03 TestBankTransferK303ProofPrivacy          the buyer transfer proof is in the buyer export; erasure clears last5 only
//   K3-04 TestBankTransferK304ConfirmWithoutProof   a confirm with no buyer submission stays allowed but is flagged in the audit trail and the answer
// Owner-pool writes (disclosed fixtures): one rolled-back forged ledger INSERT (the guard), one OWNER read of balances/ledger/audit rows.

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"sync"
	"testing"
)

func (e *tcvEnv) cokAudit(action string) int {
	return e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action=$2`, e.store(), action)
}

// cokConfirmed places an order, optionally submits a proof, and confirms it through the real routes.
func (e *tcvEnv) cokConfirmed(proof bool) (*tcvBuyer, string) {
	e.t.Helper()
	b, order := e.cogPlace("")
	if proof {
		if r := b.cofProof(order, t04Key("cok-proof"), "12345", e.cogTotal(order)); r.status != 200 {
			e.t.Fatalf("proof: %d %s", r.status, r.body)
		}
	}
	if d := e.cogConfirm(order, t04Key("cok-confirm")); d.status != 200 {
		e.t.Fatalf("confirm: %d %v", d.status, d.out)
	}
	return b, order
}

func TestBankTransferK301SameKeyAcrossOrders(t *testing.T) {
	e := cogNew(t, 72)
	f := e.p.f
	ctx := context.Background()
	_, orderA := e.cogPlace("")
	_, orderB := e.cogPlace("")
	sku := e.p.stock.skus[0].ID

	t.Run("session 1 holds the key inside the definer; session 2 (HTTP, same key, other order) waits then gets idempotency_conflict, not a 5xx", func(t *testing.T) {
		key := t04Key("cok-race")
		hash := sha256.Sum256([]byte(e.token()))
		tx, err := f.runtime.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		var raw []byte
		if err := tx.QueryRow(ctx, `SELECT payments.decide_bank_transfer($1,$2::uuid,$3::uuid,$4,$5,'confirm',NULL)`, hash[:], e.store(), orderA, key, make([]byte, 32)).Scan(&raw); err != nil {
			t.Fatalf("session 1 confirm of order A: %v", err)
		}
		var wg sync.WaitGroup
		var st int
		var out map[string]any
		wg.Add(1)
		go func() { defer wg.Done(); st, out = e.cofDecide(e.token(), orderB, "confirm", key, `{}`) }()
		e.cogWaitLockWaiters(1) // session 2 is provably queued behind session 1
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		wg.Wait()
		if st != 409 || cofCode(out) != "idempotency_conflict" {
			t.Errorf("the second caller under the same key: %d %v, want 409 idempotency_conflict", st, out)
		}
		if n := e.count(`SELECT count(*) FROM checkout.bank_transfers WHERE order_id=$1 AND state='AWAITING'`, orderB); n != 1 {
			t.Error("the refused caller must leave order B untouched")
		}
		if n := e.count(`SELECT count(*) FROM checkout.bank_transfers WHERE order_id=$1 AND state='CONFIRMED'`, orderA); n != 1 {
			t.Error("session 1's confirm of order A committed")
		}
	})

	t.Run("two simultaneous HTTP callers, one key, two orders: exactly one 200, the other 409, never a 5xx", func(t *testing.T) {
		_, o1 := e.cogPlace("")
		_, o2 := e.cogPlace("")
		_, _, a0 := e.cofBalance(sku)
		key := t04Key("cok-race2")
		var wg sync.WaitGroup
		var st [2]int
		var out [2]map[string]any
		start := make(chan struct{})
		for i, o := range []string{o1, o2} {
			wg.Add(1)
			go func() { defer wg.Done(); <-start; st[i], out[i] = e.cofDecide(e.token(), o, "confirm", key, `{}`) }()
		}
		close(start)
		wg.Wait()
		ok, conflict := 0, 0
		for i := range st {
			switch {
			case st[i] == 200:
				ok++
			case st[i] == 409 && cofCode(out[i]) == "idempotency_conflict":
				conflict++
			default:
				t.Errorf("caller %d: %d %v", i, st[i], out[i])
			}
		}
		if ok != 1 || conflict != 1 {
			t.Errorf("ok=%d conflict=%d, want 1 and 1", ok, conflict)
		}
		if _, _, a1 := e.cofBalance(sku); a1 != a0+2 {
			t.Errorf("exactly one order is allocated: allocated %d -> %d", a0, a1)
		}
	})
}

func TestBankTransferK302RefundRestock(t *testing.T) {
	e := cogNew(t, 72)
	f := e.p.f
	ctx := context.Background()
	sku := e.p.stock.skus[0].ID

	t.Run("restock=true before shipment: allocation released in the refund transaction, ledger + audit, idempotent, flip is a conflict", func(t *testing.T) {
		_, order := e.cokConfirmed(true)
		onHand, reserved, allocated := e.cofBalance(sku)
		auditRestock, auditPlain := e.cokAudit("checkout.bank_transfer_refunded_offline_restock"), e.cokAudit("checkout.bank_transfer_refunded_offline")
		key := t04Key("cok-restock")
		st1, out1 := e.cofDecide(e.token(), order, "refund-offline", key, `{"restock":true}`)
		st2, out2 := e.cofDecide(e.token(), order, "refund-offline", key, `{"restock":true}`)
		if st1 != 200 || st2 != 200 || out1["state"] != "REFUNDED_OFFLINE" || out1["restocked"] != true || fmt.Sprint(out1) != fmt.Sprint(out2) {
			t.Fatalf("restock refund and replay: %d %v / %d %v", st1, out1, st2, out2)
		}
		if oh, r, a := e.cofBalance(sku); oh != onHand || r != reserved || a != allocated-2 {
			t.Errorf("balance on_hand %d->%d reserved %d->%d allocated %d->%d: the 2 allocated units return to available, nothing else moves", onHand, oh, reserved, r, allocated, a)
		}
		var rows int
		var actor, op string
		if err := f.owner.QueryRow(ctx, `SELECT count(*),min(actor_kind),min(operation) FROM inventory.ledger WHERE reservation_id=$1 AND kind='DEALLOCATE'`, order).Scan(&rows, &actor, &op); err != nil {
			t.Fatal(err)
		}
		if rows != 1 || actor != "MERCHANT" || op != "checkout.bank_transfer.refund_restock" {
			t.Errorf("DEALLOCATE rows=%d actor=%s op=%s", rows, actor, op)
		}
		if n := e.count(`SELECT count(*) FROM inventory.reservations WHERE id=$1 AND state='RELEASED'`, order); n != 1 {
			t.Error("the reservation ends RELEASED")
		}
		if n, p := e.cokAudit("checkout.bank_transfer_refunded_offline_restock"), e.cokAudit("checkout.bank_transfer_refunded_offline"); n != auditRestock+1 || p != auditPlain {
			t.Errorf("audit: restock rows %d -> %d (want +1, replay none), plain rows %d -> %d (want unchanged)", auditRestock, n, auditPlain, p)
		}
		if st, out := e.cofDecide(e.token(), order, "refund-offline", key, `{"restock":false}`); st != 409 || cofCode(out) != "idempotency_conflict" {
			t.Errorf("the same key with the restock flag flipped: %d %v", st, out)
		}
		if st, out := e.cofDecide(e.token(), order, "refund-offline", t04Key("cok-restock2"), `{"restock":true}`); st != 409 || cofCode(out) != "already_refunded" {
			t.Errorf("a second refund with a new key: %d %v", st, out)
		}
		if n := e.count(`SELECT count(*) FROM inventory.ledger WHERE reservation_id=$1 AND kind='DEALLOCATE'`, order); n != 1 {
			t.Errorf("DEALLOCATE rows after the repeats: %d", n)
		}
		if disposition, _ := e.cofExpire(order); disposition != "STALE" {
			t.Errorf("the expiry job of a refunded order: %s, want STALE", disposition)
		}
	})

	t.Run("no restock choice ({} or restock=false): the refund is the pre-0099 fact and stock stays allocated", func(t *testing.T) {
		for i, body := range []string{`{}`, `{"restock":false}`} {
			_, order := e.cokConfirmed(false)
			_, reserved, allocated := e.cofBalance(sku)
			st, out := e.cofDecide(e.token(), order, "refund-offline", t04Key(fmt.Sprintf("cok-norestock%d", i)), body)
			if st != 200 || out["state"] != "REFUNDED_OFFLINE" || out["restocked"] != false {
				t.Fatalf("%s: %d %v", body, st, out)
			}
			if _, r, a := e.cofBalance(sku); r != reserved || a != allocated {
				t.Errorf("%s moved stock: reserved %d->%d allocated %d->%d", body, reserved, r, allocated, a)
			}
			if n := e.count(`SELECT count(*) FROM inventory.ledger WHERE reservation_id=$1 AND kind='DEALLOCATE'`, order); n != 0 {
				t.Errorf("%s wrote %d DEALLOCATE rows", body, n)
			}
			if n := e.count(`SELECT count(*) FROM inventory.reservations WHERE id=$1 AND state='COMMITTED'`, order); n != 1 {
				t.Errorf("%s changed the reservation", body)
			}
		}
	})

	t.Run("after shipment restock is refused (422 already_shipped), nothing changes; the plain refund still works", func(t *testing.T) {
		_, order := e.cokConfirmed(false)
		if st, _, raw := e.mcall(e.token(), "PUT", "/v1/admin/stores/"+e.store()+"/orders/"+order+"/shipment", t04Key("cok-ship"), mfxShip(0, "sf_express", "SF0001")); st != 200 {
			t.Fatalf("record the shipment: %d %s", st, raw)
		}
		_, reserved, allocated := e.cofBalance(sku)
		audit := e.cokAudit("checkout.bank_transfer_refunded_offline_restock")
		if st, out := e.cofDecide(e.token(), order, "refund-offline", t04Key("cok-shipped"), `{"restock":true}`); st != 422 || cofCode(out) != "already_shipped" {
			t.Fatalf("restock after shipment: %d %v", st, out)
		}
		if n := e.count(`SELECT count(*) FROM checkout.bank_transfers WHERE order_id=$1 AND state='CONFIRMED'`, order); n != 1 {
			t.Error("a refused restock must roll the whole refund back")
		}
		if _, r, a := e.cofBalance(sku); r != reserved || a != allocated || e.cokAudit("checkout.bank_transfer_refunded_offline_restock") != audit {
			t.Error("a refused restock moved stock or wrote an audit row")
		}
		if st, out := e.cofDecide(e.token(), order, "refund-offline", t04Key("cok-shipped2"), `{"restock":false}`); st != 200 || out["restocked"] != false {
			t.Errorf("plain refund of a shipped order: %d %v", st, out)
		}
	})

	t.Run("the body is strict and the route needs payments:refund like every transfer decision", func(t *testing.T) {
		_, order := e.cokConfirmed(false)
		for name, body := range map[string]string{"string flag": `{"restock":"yes"}`, "unknown field": `{"restock":true,"x":1}`, "array": `[]`} {
			if st, _ := e.cofDecide(e.token(), order, "refund-offline", t04Key("cok-bad"), body); st != 400 {
				t.Errorf("%s: %d, want 400", name, st)
			}
		}
		if st, _ := e.cofDecide("", order, "refund-offline", t04Key("cok-anon"), `{"restock":true}`); st != 401 {
			t.Errorf("no token: %d", st)
		}
		if n := e.count(`SELECT count(*) FROM checkout.bank_transfers WHERE order_id=$1 AND state='CONFIRMED'`, order); n != 1 {
			t.Error("refused bodies changed the transfer")
		}
	})

	t.Run("ledger provenance: a forged restock DEALLOCATE for a transfer that is not REFUNDED_OFFLINE is refused (42501)", func(t *testing.T) {
		_, order := e.cokConfirmed(false)
		var owner, session, wh, skuID string
		var qty int64
		if err := f.owner.QueryRow(ctx, `SELECT o.owner_id::text,o.creator_session_id::text,l.warehouse_id::text,l.sku_id::text,l.quantity FROM checkout.orders o JOIN inventory.reservation_lines l ON l.tenant_id=o.tenant_id AND l.store_id=o.store_id AND l.reservation_id=o.id WHERE o.id=$1`, order).Scan(&owner, &session, &wh, &skuID, &qty); err != nil {
			t.Fatal(err)
		}
		tx, err := f.owner.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id',$1,true),set_config('app.store_id',$2,true),set_config('app.principal_id',$3,true),set_config('app.buyer_id',$4,true),set_config('app.buyer_session_id',$5,true)`,
			e.tenant(), e.store(), f.principalA, owner, session); err != nil {
			t.Fatal(err)
		}
		st, _ := tcsSub(ctx, tx, `INSERT INTO inventory.ledger(tenant_id,store_id,warehouse_id,sku_id,kind,delta_reserved,delta_allocated,operation,command_key,reservation_id,actor_kind,checkout_id,buyer_owner_id,buyer_session_id,principal_id)
		  VALUES($1::uuid,$2::uuid,$3::uuid,$4::uuid,'DEALLOCATE',0,$5::bigint,'checkout.bank_transfer.refund_restock',$6::text,$6::uuid,'MERCHANT',$6::uuid,$7::uuid,$8::uuid,$9::uuid)`,
			e.tenant(), e.store(), wh, skuID, -qty, order, owner, session, f.principalA)
		if st != "42501" {
			t.Errorf("a restock DEALLOCATE for a CONFIRMED (not refunded) transfer: want 42501 from the ledger guard, got %q", st)
		}
	})
}

func TestBankTransferK303ProofPrivacy(t *testing.T) {
	e := cogNew(t, 72)
	bx, ox := e.cogPlace("")
	by, oy := e.cogPlace("")
	total := e.cogTotal(ox)
	for _, c := range []struct {
		b     *tcvBuyer
		order string
		last5 string
	}{{bx, ox, "12345"}, {by, oy, "98765"}} {
		if r := c.b.cofProof(c.order, t04Key("cok-p"), c.last5, e.cogTotal(c.order)); r.status != 200 {
			t.Fatalf("proof: %d %s", r.status, r.body)
		}
	}
	exp := bx.req("POST", "/v1/buyer/privacy/export", t04Key("cok-exp"), nil, nil)
	if exp.status != 200 {
		t.Fatalf("export: %d %s", exp.status, exp.body)
	}
	doc := cofJSON(t, exp.body)
	orders, _ := doc["orders"].([]any)
	if len(orders) != 1 {
		t.Fatalf("buyer X's export holds %d orders", len(orders))
	}
	proof, _ := orders[0].(map[string]any)["bank_transfer_proof"].(map[string]any)
	if proof["last5"] != "12345" || proof["amount_minor"] != float64(total) || proof["paid_at"] == nil {
		t.Errorf("the export must carry the buyer's own transfer proof: %v", proof)
	}
	if strings.Contains(string(exp.body), "98765") {
		t.Error("buyer X's export contains buyer Y's proof")
	}
	if d := e.cogConfirm(ox, t04Key("cok-c")); d.status != 200 {
		t.Fatalf("confirm: %d %v", d.status, d.out)
	}
	if er := bx.req("POST", "/v1/buyer/privacy/erasure", t04Key("cok-erase"), map[string]any{"confirm": "ERASE"}, nil); er.status != 200 {
		t.Fatalf("erasure after the confirmation: %d %s", er.status, er.body)
	}
	if n := e.count(`SELECT count(*) FROM checkout.bank_transfers WHERE order_id=$1 AND proof_last5 IS NULL`, ox); n != 1 {
		t.Error("erasure clears proof_last5 (buyer-typed account fragment)")
	}
	if n := e.count(`SELECT count(*) FROM checkout.bank_transfers WHERE order_id=$1 AND state='CONFIRMED' AND proof_amount_minor=$2 AND proof_paid_at IS NOT NULL AND confirmed_amount_minor=$2`, ox, total); n != 1 {
		t.Error("amount, paid_at and the confirmed fact stay as the financial record")
	}
	if n := e.count(`SELECT count(*) FROM checkout.bank_transfers WHERE order_id=$1 AND proof_last5='98765'`, oy); n != 1 {
		t.Error("erasing buyer X cleared buyer Y's proof")
	}
	// an order with no submission exports a null proof
	bz, _ := e.cogPlace("")
	if z := bz.req("POST", "/v1/buyer/privacy/export", t04Key("cok-expz"), nil, nil); z.status != 200 || !strings.Contains(string(z.body), `"bank_transfer_proof":null`) {
		t.Errorf("an order without a submission exports bank_transfer_proof null: %d %s", z.status, z.body)
	}
}

func TestBankTransferK304ConfirmWithoutProof(t *testing.T) {
	e := cogNew(t, 72)
	t.Run("no submission: the confirm succeeds, the answer says so and the audit trail gets the flag row", func(t *testing.T) {
		_, order := e.cogPlace("")
		plain, flag := e.cokAudit("checkout.bank_transfer_confirmed"), e.cokAudit("checkout.bank_transfer_confirmed_without_proof")
		key := t04Key("cok-np")
		st1, out1 := e.cofDecide(e.token(), order, "confirm", key, `{}`)
		st2, out2 := e.cofDecide(e.token(), order, "confirm", key, `{}`)
		if st1 != 200 || st2 != 200 || out1["state"] != "CONFIRMED" || out1["confirmed_without_proof"] != true || fmt.Sprint(out1) != fmt.Sprint(out2) {
			t.Fatalf("confirm and replay: %d %v / %d %v", st1, out1, st2, out2)
		}
		if p, f := e.cokAudit("checkout.bank_transfer_confirmed"), e.cokAudit("checkout.bank_transfer_confirmed_without_proof"); p != plain+1 || f != flag+1 {
			t.Errorf("audit rows: confirmed %d -> %d, confirmed_without_proof %d -> %d (want +1 each, replay none)", plain, p, flag, f)
		}
	})
	t.Run("with a submission (even a rejected one resubmitted): confirmed_without_proof is false and no flag row", func(t *testing.T) {
		b, order := e.cogPlace("")
		if r := b.cofProof(order, t04Key("cok-wp"), "12345", e.cogTotal(order)); r.status != 200 {
			t.Fatalf("proof: %d %s", r.status, r.body)
		}
		flag := e.cokAudit("checkout.bank_transfer_confirmed_without_proof")
		if st, out := e.cofDecide(e.token(), order, "confirm", t04Key("cok-wp-c"), `{}`); st != 200 || out["confirmed_without_proof"] != false {
			t.Errorf("confirm with a submission: %d %v", st, out)
		}
		if f := e.cokAudit("checkout.bank_transfer_confirmed_without_proof"); f != flag {
			t.Errorf("flag rows %d -> %d: a submitted proof must not be flagged", flag, f)
		}
	})
}
