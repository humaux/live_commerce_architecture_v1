package foundation_test

// LPC: gate of R4S-01 (P1, output/r4-security-review/REVIEW.md): a claim's live price may be used for at most the claimed quantity across
// all of the buyer's orders that still hold it (contracts/live-keyword-claims-v1.md "Live tools (R4)" rules 5 and 7 as amended by 0105).
// Written from the finding and the amended rule, not from migrations/0105: every case drives the real Quote / checkout.Begin / expiry /
// merchant / payment paths and only reads the ledger and order rows from the owner pool.
//
//   LPC01 TestLiveToolsGateConsumptionReorderAndExpiry  order at the live price, re-send the same cart: catalog price (cart preview too),
//                                                       a catalog order consumes nothing; expire_held gives the quantity back
//   LPC02 TestLiveToolsGateConsumptionPartial           claim 3, order 2 live: 1 more is live, 2 more is catalog; the last unit once
//   LPC03 TestLiveToolsGateConsumptionPickupCancel      a CONFIRMED pay-at-pickup order keeps the quantity; the merchant cancel releases it
//   LPC04 TestLiveToolsGateConsumptionPaidKeeps         a card order paid through the real capture path keeps the quantity
//   LPC05 TestLiveToolsGateConsumptionConcurrentBegins  two concurrent Begins of one live quote: exactly one order, one ledger row;
//                                                       a direct second consumption of the same order is refused (lock + recount)
//   LPC06 TestLiveToolsGateConsumptionLedgerACL         no runtime role holds any privilege on the ledger; only the checkout pool executes
//                                                       the consumption definer; the buyer pool cannot
//
// Owner-pool writes (disclosed fixtures): aging a DRAFT order past its hold before expire_held (proAge, as the promotions gates do).

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/buyer"
	"livecommerce/internal/checkout"
	"livecommerce/internal/command"
	"livecommerce/internal/storefront"
)

// reCart re-sends the cart as the buyer would (PUT cart with the same SKU): a direct SetCart keeps the claim origin while qty <= claim_quantity.
func (e *ltgEnv) reCart(b *tcvBuyer, qty int64) {
	e.t.Helper()
	cart := e.h.cartOf(e.t, b.cap)
	if _, err := e.h.putCart(b.cap, t04Key("lpc-recart"), storefront.CartInput{ExpectedVersion: cart.Version,
		Items: []storefront.Item{{SKUID: e.money, Quantity: qty}}}); err != nil {
		e.t.Fatalf("re-send cart: %v", err)
	}
	b.h.input.CartVersion = e.h.cartOf(e.t, b.cap).Version
}

// held is the claimed quantity the ledger records for the claim line (bundle, offer) on orders that are not CANCELLED, and the row count.
func (e *ltgEnv) held(bundle, offer string) (quantity, rows int) {
	e.t.Helper()
	return e.count(`SELECT coalesce(sum(u.quantity),0)::int FROM claims.live_price_uses u JOIN checkout.orders o ON o.id=u.order_id
			WHERE u.bundle_id=$1 AND u.offer_id=$2 AND o.commercial_state<>'CANCELLED'`, bundle, offer),
		e.count(`SELECT count(*) FROM claims.live_price_uses WHERE bundle_id=$1 AND offer_id=$2`, bundle, offer)
}

func (e *ltgEnv) total(order string) int64 {
	e.t.Helper()
	return int64(e.count(`SELECT total_minor FROM checkout.orders WHERE id=$1`, order))
}

func TestLiveToolsGateConsumptionReorderAndExpiry(t *testing.T) {
	e := ltgNew(t)
	s, o := e.session("A1", ltgLive, 5)
	bundle, l := e.claimLink(s, "amy", "A1+2")
	b := e.redeemed(l)
	q, ln := e.line(b, "")
	e.wantLive("first quote", ln, ltgLive, bundle, o.ID)
	first, err := e.placeHome(b, q)
	if err != nil {
		t.Fatalf("order 1 at the live price: %v", err)
	}
	if got := e.total(first.OrderID); got != q.Amount.TotalMinor || got != ltgQty*ltgLive+q.Amount.ShippingMinor+q.Amount.TaxMinor {
		t.Fatalf("order 1 total %d", got)
	}

	// The R4S-01 exploit: re-send the same items (the origin is carried, 2 <= claimed 2) and quote again.
	e.reCart(b, ltgQty)
	if got := e.cartLive(b); got != 0 {
		t.Fatalf("the cart still previews the consumed live price %d", got)
	}
	q2, ln2 := e.line(b, "")
	e.wantCatalog("R4S-01: re-quote after the claimed quantity was ordered", ln2)
	if n := e.count(`SELECT count(*) FROM claims.live_price_uses WHERE order_id=$1 AND bundle_id=$2 AND offer_id=$3 AND quantity=2`, first.OrderID, bundle, o.ID); n != 1 {
		t.Fatalf("order 1 at the live price has %d ledger rows, want 1 (never a live price without a ledger row)", n)
	}
	second, err := e.placeHome(b, q2)
	if err != nil {
		t.Fatalf("order 2 at the catalog price: %v", err)
	}
	if got := e.total(second.OrderID); got != ltgQty*ltgCatalog+q2.Amount.ShippingMinor+q2.Amount.TaxMinor {
		t.Fatalf("order 2 total %d, want the catalog total", got)
	}
	if qty, rows := e.held(bundle, o.ID); qty != 2 || rows != 1 {
		t.Fatalf("a catalog order consumed the claim: held %d rows %d", qty, rows)
	}

	// Order 1 expires (the River expiry path): its stock and its claimed quantity come back.
	e.proAge(first.OrderID)
	if d, _ := e.cofExpire(first.OrderID); d != "EXPIRED" {
		t.Fatalf("expire_held: %s", d)
	}
	if qty, rows := e.held(bundle, o.ID); qty != 0 || rows != 1 {
		t.Fatalf("after expiry: held %d rows %d (release is derived from order state, the row stays)", qty, rows)
	}
	e.reCart(b, ltgQty)
	q3, ln3 := e.line(b, "")
	e.wantLive("re-quote after the live order expired", ln3, ltgLive, bundle, o.ID)
	third, err := e.placeHome(b, q3)
	if err != nil {
		t.Fatalf("order 3 at the live price after the expiry: %v", err)
	}
	if got := e.total(third.OrderID); got != q3.Amount.TotalMinor {
		t.Fatalf("order 3 total %d", got)
	}
	if qty, rows := e.held(bundle, o.ID); qty != 2 || rows != 2 {
		t.Fatalf("after order 3: held %d rows %d", qty, rows)
	}
	e.reCart(b, ltgQty)
	_, ln4 := e.line(b, "")
	e.wantCatalog("re-quote after order 3", ln4)
}

func TestLiveToolsGateConsumptionPartial(t *testing.T) {
	e := ltgNew(t)
	s, o := e.session("A1", ltgLive, 5)
	bundle, l := e.claimLink(s, "amy", "A1+3")
	b := e.redeemed(l)
	e.reCart(b, 2)
	q, ln := e.line(b, "")
	e.wantLive("2 of 3", ln, ltgLive, bundle, o.ID)
	if ln.Quantity != 2 {
		t.Fatalf("quote quantity %d", ln.Quantity)
	}
	if _, err := e.placeHome(b, q); err != nil {
		t.Fatalf("order of 2 at the live price: %v", err)
	}
	e.reCart(b, 2)
	_, ln = e.line(b, "")
	e.wantCatalog("2 more with 1 left", ln)
	e.reCart(b, 1)
	q1, ln := e.line(b, "")
	e.wantLive("the 1 left", ln, ltgLive, bundle, o.ID)
	last, err := e.placeHome(b, q1)
	if err != nil {
		t.Fatalf("order of the last unit at the live price: %v", err)
	}
	if got := e.total(last.OrderID); got != ltgLive+q1.Amount.ShippingMinor+q1.Amount.TaxMinor {
		t.Fatalf("last-unit order total %d", got)
	}
	if qty, rows := e.held(bundle, o.ID); qty != 3 || rows != 2 {
		t.Fatalf("held %d rows %d, want 3 over 2 orders", qty, rows)
	}
	e.reCart(b, 1)
	_, ln = e.line(b, "")
	e.wantCatalog("claim fully consumed", ln)
}

func TestLiveToolsGateConsumptionPickupCancel(t *testing.T) {
	e := ltgNew(t)
	e.cvsSettings(tcvAllChains, true, "20000", 500)
	code, _, _ := e.service("cvs_711", "MANUAL", 0)
	s, o := e.session("B1", 12000, 5)
	bundle, l := e.claimLink(s, "bob", "B1+2")
	pb := e.redeemed(l)
	pb.h.input.CartVersion = e.h.cartOf(t, pb.cap).Version
	pickup := e.tppEntered(pb, code)
	dest, err := pb.destination("cvs_711", pickup, tppName, tppPhone)
	if err != nil {
		t.Fatal(err)
	}
	pq, err := pb.quoteCodeFor(code, "")
	if err != nil {
		t.Fatalf("pay-at-pickup quote: %v", err)
	}
	e.wantLive("pay-at-pickup quote", pq.Lines[0], 12000, bundle, o.ID)
	placed, err := pb.begin(dest, pq, e.svcVer[code], "pay_at_pickup")
	if err != nil {
		t.Fatalf("pay-at-pickup placement: %v", err)
	}
	if state := e.count(`SELECT count(*) FROM checkout.orders WHERE id=$1 AND commercial_state='CONFIRMED'`, placed.OrderID); state != 1 {
		t.Fatal("a pay-at-pickup order is CONFIRMED at placement")
	}
	// CONFIRMED keeps the quantity.
	again, err := pb.quoteCodeFor(code, "")
	if err != nil || len(again.Lines) != 1 {
		t.Fatalf("re-quote: %+v %v", again, err)
	}
	e.wantCatalog("re-quote while the CONFIRMED pay-at-pickup order holds the claim", again.Lines[0])
	// The merchant cancels the unshipped order (stock released): the quantity comes back.
	writer, _ := e.member("fulfillment:write", "orders:read")
	if st, _, raw := e.release(writer, placed.OrderID, t04Key("lpc-cancel"), "cancel", "PENDING"); st != 200 {
		t.Fatalf("merchant cancel: %d %s", st, raw)
	}
	if qty, rows := e.held(bundle, o.ID); qty != 0 || rows != 1 {
		t.Fatalf("after cancel: held %d rows %d", qty, rows)
	}
	back, err := pb.quoteCodeFor(code, "")
	if err != nil || len(back.Lines) != 1 {
		t.Fatalf("re-quote after cancel: %+v %v", back, err)
	}
	e.wantLive("re-quote after the merchant cancelled", back.Lines[0], 12000, bundle, o.ID)
}

func TestLiveToolsGateConsumptionPaidKeeps(t *testing.T) {
	e := ltgNew(t, tcvOpts{stripe: true})
	e.r.startWorker(t)
	s, o := e.session("A1", ltgLive, 5)
	bundle, l := e.claimLink(s, "amy", "A1+2")
	b := e.redeemed(l)
	q, ln := e.line(b, "")
	e.wantLive("quote", ln, ltgLive, bundle, o.ID)
	hold, err := e.placeHome(b, q)
	if err != nil {
		t.Fatalf("hold: %v", err)
	}
	paid := e.payHold(hold, b)
	if paid.captured != q.Amount.TotalMinor {
		t.Fatalf("captured %d, want the live total %d", paid.captured, q.Amount.TotalMinor)
	}
	if n := e.count(`SELECT count(*) FROM checkout.orders WHERE id=$1 AND commercial_state='CONFIRMED'`, hold.OrderID); n != 1 {
		t.Fatal("paid order is not CONFIRMED")
	}
	e.reCart(b, ltgQty)
	_, ln = e.line(b, "")
	e.wantCatalog("re-quote after the live order was paid", ln)
	if qty, _ := e.held(bundle, o.ID); qty != 2 {
		t.Fatalf("a paid order holds %d, want 2", qty)
	}
}

func TestLiveToolsGateConsumptionConcurrentBegins(t *testing.T) {
	e := ltgNew(t)
	s, o := e.session("A1", ltgLive, 5)
	bundle, l := e.claimLink(s, "amy", "A1+2")
	b := e.redeemed(l)
	q, ln := e.line(b, "")
	e.wantLive("quote", ln, ltgLive, bundle, o.ID)
	cart := e.h.cartOf(t, b.cap)
	din := bdHome(cart)
	din.ExpectedVersion = b.h.destination.Version
	dest, err := bdSet(b.h.cqHarness, t04Key("lpc-dest"), din)
	if err != nil {
		t.Fatal(err)
	}
	in := checkout.Input{QuoteID: q.ID, DestinationID: dest.ID, CartVersion: cart.Version, ServiceVersion: 1, AllocationVersion: 1}
	const n = 2
	results, errs := make([]checkout.Result, n), make([]error, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			results[i], errs[i] = e.svc.Begin(context.Background(), b.cap.Token, e.store(), t04Key("lpc-race"), in)
		}(i)
	}
	close(start)
	wg.Wait()
	won := -1
	for i, err := range errs {
		switch {
		case err == nil && won < 0:
			won = i
		case err == nil:
			t.Fatalf("both concurrent Begins placed an order: %+v %+v", results[won], results[i])
		case !errors.Is(err, command.ErrConflict):
			t.Fatalf("Begin %d: want the typed conflict, got %v", i, err)
		}
	}
	if won < 0 {
		t.Fatalf("no Begin won: %v", errs)
	}
	if got := e.count(`SELECT count(*) FROM checkout.orders WHERE owner_id=$1`, b.cap.Scope.OwnerID); got != 1 {
		t.Fatalf("%d orders, want 1", got)
	}
	if qty, rows := e.held(bundle, o.ID); qty != 2 || rows != 1 {
		t.Fatalf("held %d rows %d, want one consumption of 2", qty, rows)
	}
	// A second consumption of the same fresh order (bypassing Begin's advisory lock and RevalidateQuote) is refused by the definer itself:
	// it locks the claim line and recounts held uses, so the live price can never be charged twice for one claimed quantity.
	err = buyer.WithScope(context.Background(), e.p.pool, b.cap.Token, e.store(), func(ctx context.Context, tx pgx.Tx, _ buyer.Scope) error {
		var c int
		return tx.QueryRow(ctx, `SELECT claims.consume_live_prices($1::uuid)`, results[won].OrderID).Scan(&c)
	})
	requirePGCode(t, err, "PT409", "second consumption of the same claimed quantity")
}

func TestLiveToolsGateConsumptionLedgerACL(t *testing.T) {
	e := ltgNew(t)
	if got := e.count(`SELECT count(*) FROM pg_roles r WHERE r.rolname LIKE 'commerce\_%' AND r.rolname<>'commerce_claims_writer'
		AND (has_table_privilege(r.oid,'claims.live_price_uses','SELECT,INSERT,UPDATE,DELETE,TRUNCATE,REFERENCES,TRIGGER')
		  OR has_any_column_privilege(r.oid,'claims.live_price_uses','SELECT,INSERT,UPDATE,REFERENCES'))`); got != 0 {
		t.Fatalf("%d runtime roles hold a privilege on the consumption ledger", got)
	}
	if got := e.count(`SELECT count(*) FROM pg_roles r WHERE r.rolname LIKE 'commerce\_%'
		AND has_function_privilege(r.oid,'claims.consume_live_prices(uuid)','EXECUTE')
		AND r.rolname NOT IN ('commerce_claims_writer','commerce_checkout_runtime','commerce_hosted_runtime')`); got != 0 {
		t.Fatalf("%d other roles can execute the consumption definer", got)
	}
	s, _ := e.session("A1", ltgLive, 5)
	_, l := e.claimLink(s, "amy", "A1+2")
	b := e.redeemed(l)
	q, _ := e.line(b, "")
	res, err := e.placeHome(b, q)
	if err != nil {
		t.Fatal(err)
	}
	// The buyer pool cannot consume (the 0103 lesson: EXECUTE follows the pool that runs the call).
	err = buyer.WithScope(context.Background(), e.p.a.runtime, b.cap.Token, e.store(), func(ctx context.Context, tx pgx.Tx, _ buyer.Scope) error {
		_, x := tx.Exec(ctx, `SELECT claims.consume_live_prices($1::uuid)`, res.OrderID)
		return x
	})
	requirePGCode(t, err, "42501", "buyer pool consume")
	// The checkout pool can neither read nor write the ledger directly.
	for _, stmt := range []string{`SELECT 1 FROM claims.live_price_uses LIMIT 1`,
		`INSERT INTO claims.live_price_uses(tenant_id,store_id,bundle_id,offer_id,order_id,quantity) SELECT tenant_id,store_id,gen_random_uuid(),gen_random_uuid(),id,1 FROM checkout.orders WHERE id='` + res.OrderID + `'`,
		`DELETE FROM claims.live_price_uses`} {
		err = buyer.WithScope(context.Background(), e.p.pool, b.cap.Token, e.store(), func(ctx context.Context, tx pgx.Tx, _ buyer.Scope) error {
			_, x := tx.Exec(ctx, stmt)
			return x
		})
		requirePGCode(t, err, "42501", "checkout pool "+stmt)
	}
}
