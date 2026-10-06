package foundation_test

// LCN12 review follow-ups (Opus money review of LC-B6, P2-1..P2-6): the "one live order per bundle" rule survives a crash between Place and the record,
// a resumed row and a Place failure after the order exists; the same Idempotency-Key with another body is refused; grants do not outlive their request;
// the audit records the price the ledger consumed; the peer-state helper re-checks inventory:reserve. Tier REAL_PG + HTTP_PG, evidence class MOCK.
//   LCN12h TestLiveConsoleOrderForBuyerReservationKeepsItsOrder   P2-1 a (updated_at, not created_at), b (pending row WITH an order is never stolen,
//                                                                 replay completes), c (release refuses a bundle that has an order), finish raises
//                                                                 reservation_lost
//   LCN12i TestLiveConsoleOrderForBuyerKeyBoundToItsBody          P2-4
//   LCN12j TestLiveConsoleOrderForBuyerGrantsDoNotOutliveRequest  P2-3, P2-5
//   LCN12k TestLiveConsoleOrderForBuyerAuditAndHelperAuthority    P2-2 (unit price consumed, full ids), P2-6
// Owner-pool writes (disclosed fixtures): ageing/rewriting reservation rows, inserting a competing reservation row.

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/claims"
	"livecommerce/internal/command"
	"livecommerce/internal/merchanttools"
	"livecommerce/internal/platform"
	"livecommerce/internal/storefront"
)

func (e *lbuEnv) input(bundles []string, conversation string) merchanttools.ForBuyerInput {
	in := merchanttools.ForBuyerInput{
		Items:       []merchanttools.ManualItem{{SKUID: e.money, Quantity: 2}},
		Customer:    merchanttools.ManualCustomer{Name: "王小明", Phone: "0912-345-678", Email: "buyer@example.test"},
		Delivery:    merchanttools.ManualDelivery{OptionKey: e.homeKey, HomeAddress: &storefront.HomeAddress{Region: "台北市", City: "中正區", PostalCode: "100", Line1: "忠孝東路1號"}},
		PaymentMode: "bank_transfer", Locale: "zh-TW",
		For: merchanttools.ForBuyerTarget{BundleIDs: bundles},
	}
	if conversation != "" {
		in.For.ConversationID = &conversation
	}
	return in
}

func (e *lbuEnv) scopedTx(token, permission string, fn func(context.Context, pgx.Tx) error) error {
	return platform.WithScope(context.Background(), e.p.f.runtime, token, e.store(), permission, func(tx pgx.Tx, _ platform.Scope) error {
		return fn(context.Background(), tx)
	})
}

// crashAfterPlace runs a request that dies after Place (the order exists, the reservation row is pending).
func (e *lbuEnv) crashAfterPlace(key string, in merchanttools.ForBuyerInput) {
	e.t.Helper()
	e.fb.FaultAfterPlace = func() error { return errors.New("killed after Begin") }
	defer func() { e.fb.FaultAfterPlace = nil }()
	if _, _, err := e.fb.Place(context.Background(), e.token(), e.store(), key, in); err == nil {
		e.t.Fatal("the fault hook did not abort the request")
	}
}

func (e *lbuEnv) requestOf(bundle string) (request, buyer string) {
	e.t.Helper()
	if err := e.p.f.owner.QueryRow(context.Background(), `SELECT request_id::text,buyer_id::text FROM inbox.order_for_buyer WHERE bundle_id=$1 ORDER BY created_at DESC LIMIT 1`, bundle).Scan(&request, &buyer); err != nil {
		e.t.Fatal(err)
	}
	return
}

func TestLiveConsoleOrderForBuyerReservationKeepsItsOrder(t *testing.T) {
	e := lbuNew(t)
	token := e.token()
	owner := e.p.f.owner

	t.Run("P2-1 b: a crash between Place and the record leaves a pending row WITH an order; it is never stolen, however old; the replay completes", func(t *testing.T) {
		_, _, bundle, conv := e.scenario()
		key := t04Key("lcn12-p21b")
		in := e.input([]string{bundle}, conv)
		e.crashAfterPlace(key, in)
		ordersBefore := e.orders()
		mustExec(t, owner, `UPDATE inbox.order_for_buyer SET created_at=clock_timestamp()-interval '2 hours', updated_at=clock_timestamp()-interval '2 hours' WHERE bundle_id=$1`, bundle)
		st, out := e.post(token, t04Key("lcn12-p21b-other"), e.body(2, []string{bundle}, conv, false))
		details, _ := out["details"].(map[string]any)
		if st != 409 || lbuCode(out) != "bundle_already_ordered" || lbuStr(details, "order_id") == "" {
			t.Fatalf("a second request took a bundle that has an order: %d %v", st, out)
		}
		if e.orders() != ordersBefore {
			t.Fatalf("a second order was placed")
		}
		got, replayed, err := e.fb.Place(context.Background(), token, e.store(), key, in)
		if err != nil || replayed || got.OrderID != lbuStr(details, "order_id") {
			t.Fatalf("replay: %+v replayed=%v err=%v (want the same order %s)", got, replayed, err, lbuStr(details, "order_id"))
		}
		if n := e.count(`SELECT count(*) FROM inbox.order_for_buyer WHERE bundle_id=$1 AND state='placed' AND order_id=$2`, bundle, got.OrderID); n != 1 || e.orders() != ordersBefore {
			t.Fatalf("after the replay: placed rows %d, new orders %d", n, e.orders()-ordersBefore)
		}
	})
	t.Run("P2-1 a: staleness is updated_at; a resumed row (old created_at, fresh updated_at) is not stealable, an untouched one without an order is", func(t *testing.T) {
		_, _, bundle, conv := e.scenario()
		mustExec(t, owner, `INSERT INTO inbox.order_for_buyer(tenant_id,store_id,bundle_id,request_id,idempotency_key_hash,request_hash,buyer_id,state,principal_id,created_at,updated_at)
			VALUES($1,$2,$3,gen_random_uuid(),sha256(gen_random_uuid()::text::bytea),sha256(gen_random_uuid()::text::bytea),gen_random_uuid(),'pending',$4,
			clock_timestamp()-interval '2 hours',clock_timestamp())`, e.tenant(), e.store(), bundle, e.p.f.principalA)
		if st, out := e.post(token, t04Key("lcn12-p21a-fresh"), e.body(2, []string{bundle}, conv, false)); st != 409 || lbuCode(out) != "bundle_already_ordered" {
			t.Fatalf("a freshly touched pending row was stolen: %d %v", st, out)
		}
		mustExec(t, owner, `UPDATE inbox.order_for_buyer SET updated_at=clock_timestamp()-interval '16 minutes' WHERE bundle_id=$1`, bundle)
		if st, out := e.post(token, t04Key("lcn12-p21a-stale"), e.body(2, []string{bundle}, conv, false)); st != 201 {
			t.Fatalf("an untouched pending row without an order must free the bundle: %d %v", st, out)
		}
		if n := e.count(`SELECT count(*) FROM inbox.order_for_buyer WHERE bundle_id=$1 AND state='released'`, bundle); n != 1 {
			t.Fatalf("the stale row was not released: %d", n)
		}
	})
	t.Run("P2-1 c: release never frees a bundle whose request has an order; finish raises when a row of the request is gone", func(t *testing.T) {
		_, _, bundle, conv := e.scenario()
		key := t04Key("lcn12-p21c")
		e.crashAfterPlace(key, e.input([]string{bundle}, conv))
		request, _ := e.requestOf(bundle)
		var order string
		if err := owner.QueryRow(context.Background(), `SELECT o.id::text FROM checkout.orders o JOIN inbox.order_for_buyer x ON x.buyer_id=o.owner_id WHERE x.request_id=$1`, request).Scan(&order); err != nil {
			t.Fatal(err)
		}
		var released int
		if err := e.scopedTx(token, "inventory:reserve", func(ctx context.Context, tx pgx.Tx) error {
			return tx.QueryRow(ctx, `SELECT claims.for_buyer_release($1::uuid)`, request).Scan(&released)
		}); err != nil || released != 0 {
			t.Fatalf("release of a request with an order: %d %v", released, err)
		}
		if n := e.count(`SELECT count(*) FROM inbox.order_for_buyer WHERE request_id=$1 AND state='pending'`, request); n != 1 {
			t.Fatalf("the bundle was released although its order exists: %d", n)
		}
		mustExec(t, owner, `UPDATE inbox.order_for_buyer SET state='released' WHERE request_id=$1`, request)
		err := e.scopedTx(token, "inventory:reserve", func(ctx context.Context, tx pgx.Tx) error {
			_, inner := claims.FinishForBuyer(ctx, tx, request, order)
			return inner
		})
		if !errors.Is(err, command.ErrConflict) {
			t.Fatalf("finish with a lost row must be the typed conflict, got %v", err)
		}
	})
}

func TestLiveConsoleOrderForBuyerKeyBoundToItsBody(t *testing.T) {
	e := lbuNew(t)
	_, _, b1, c1 := e.scenario()
	_, _, b2, _ := e.scenario()
	key := t04Key("lcn12-p24")
	e.crashAfterPlace(key, e.input([]string{b1}, c1))
	ordersBefore := e.orders()
	_, _, err := e.fb.Place(context.Background(), e.token(), e.store(), key, e.input([]string{b2}, c1))
	var refusal *claims.ForBuyerError
	if !errors.As(err, &refusal) || refusal.Code != "idempotency_conflict" || refusal.Status != 409 {
		t.Fatalf("the same key with another bundle set: %v", err)
	}
	if n := e.count(`SELECT count(*) FROM inbox.order_for_buyer WHERE bundle_id=$1`, b2); n != 0 || e.orders() != ordersBefore {
		t.Fatalf("the refused body reserved or ordered something: rows %d orders %d", n, e.orders()-ordersBefore)
	}
	if _, replayed, err := e.fb.Place(context.Background(), e.token(), e.store(), key, e.input([]string{b1}, c1)); err != nil || replayed {
		t.Fatalf("the original body still resumes: %v replayed=%v", err, replayed)
	}
}

func TestLiveConsoleOrderForBuyerGrantsDoNotOutliveRequest(t *testing.T) {
	e := lbuNew(t)
	token := e.token()
	live := func(request string) int {
		return e.count(`SELECT count(*) FROM claims.merchant_origin_grants WHERE request_id=$1 AND consumed_at IS NULL AND expires_at>clock_timestamp()`, request)
	}
	t.Run("P2-3: a grant of a bundle that priced no line expires when the request finishes", func(t *testing.T) {
		session, _, b1, conv := e.scenario()
		b2 := e.h.accepted(t, session, "", "eve-"+t04Tag(), "A1+2").BundleID
		e.linkPeer(b2, conv)
		st, out := e.post(token, t04Key("lcn12-p23"), e.body(2, []string{b1, b2}, conv, false))
		if st != 201 || out["live_price"] != "applied" {
			t.Fatalf("two-bundle order: %d %v", st, out)
		}
		request, _ := e.requestOf(b1)
		if n := e.count(`SELECT count(*) FROM claims.merchant_origin_grants WHERE request_id=$1`, request); n != 2 || live(request) != 0 {
			t.Fatalf("grants %d, still usable %d: the unconsumed grant must be expired by finish", n, live(request))
		}
	})
	t.Run("P2-5: a replay whose eligibility now fails expires the grants an earlier run wrote", func(t *testing.T) {
		_, _, bundle, conv := e.scenario()
		keyHash, reqHash := make([]byte, 32), make([]byte, 32)
		keyHash[0], reqHash[0] = 7, 9
		begin := func(conversation *string) (claims.ForBuyerBegun, error) {
			var begun claims.ForBuyerBegun
			err := e.scopedTx(token, "inventory:reserve", func(ctx context.Context, tx pgx.Tx) error {
				var inner error
				begun, inner = claims.BeginForBuyer(ctx, tx, keyHash, reqHash, conversation, []string{bundle}, randomUUID())
				return inner
			})
			return begun, err
		}
		first, err := begin(&conv)
		if err != nil || first.Reason != "" || live(first.RequestID) != 1 {
			t.Fatalf("first begin: %+v usable grants %d %v", first, live(first.RequestID), err)
		}
		second, err := begin(nil)
		if err != nil || second.Reason != "no_conversation" || second.RequestID != first.RequestID || live(first.RequestID) != 0 {
			t.Fatalf("replay without a conversation: %+v usable grants %d %v", second, live(first.RequestID), err)
		}
	})
}

func TestLiveConsoleOrderForBuyerAuditAndHelperAuthority(t *testing.T) {
	e := lbuNew(t)
	token := e.token()
	t.Run("P2-2: the audit records the unit price the ledger consumed (not the offer's price now) with full offer and SKU ids", func(t *testing.T) {
		_, offer, bundle, conv := e.scenario()
		st, out := e.post(token, t04Key("lcn12-p22"), e.body(2, []string{bundle}, conv, false))
		if st != 201 || out["live_price"] != "applied" {
			t.Fatalf("order: %d %v", st, out)
		}
		order := lbuStr(out, "order_id")
		// The merchant edits the offer's live price right after the order: the audit must still say what was charged.
		mustExec(t, e.p.f.owner, `UPDATE live.offers SET live_price_minor=1 WHERE id=$1`, offer.ID)
		if n := e.count(`SELECT count(*) FROM claims.live_price_uses WHERE order_id=$1 AND unit_price_minor=$2`, order, ltgLive); n != 1 {
			t.Fatalf("the ledger row does not carry the consumed unit price: %d", n)
		}
		var detail string
		if err := e.p.f.owner.QueryRow(context.Background(), `SELECT details::text FROM ops.audit_events WHERE store_id=$1 AND action='claims.merchant_origin_granted'
			AND details->>'order_id'=$2`, e.store(), order).Scan(&detail); err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{offer.ID, e.money, `"p": 20000`} {
			if !strings.Contains(detail, want) {
				t.Fatalf("audit detail lacks %q: %s", want, detail)
			}
		}
		if strings.Contains(detail, `"p": 1,`) || strings.Contains(detail, `"p": 1}`) {
			t.Fatalf("the audit recorded the offer's price at audit time: %s", detail)
		}
	})
	t.Run("P2-6: the peer-state helper re-checks inventory:reserve", func(t *testing.T) {
		_, _, bundle, conv := e.scenario()
		reader, _ := e.tcvEnv.member("orders:read")
		err := e.scopedTx(reader, "orders:read", func(ctx context.Context, tx pgx.Tx) error {
			var reason string
			return tx.QueryRow(ctx, `SELECT claims.for_buyer_peer_state($1::uuid,$2::uuid[])`, conv, []string{bundle}).Scan(&reason)
		})
		if err == nil || !strings.Contains(err.Error(), "forbidden") {
			t.Fatalf("a principal without inventory:reserve read the peer state: %v", err)
		}
	})
}
