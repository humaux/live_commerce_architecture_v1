package foundation_test

// LCN12: the gate of unit LC-B6 (live-console-v1 §5 order for a buyer, A15/A16, Amendment 1 A1.3 + P2-7 + P2-8, migration 0129), written from the
// contract text. Tier: REAL_PG + HTTP_PG (the real merchant handler, the real buyer pipeline storefront/checkout/begin_hold, the real claims definers).
// The pay-link DM planner is a recording fake in the flow tests (the real inbox.PlanOrderPayLink is exercised against the real planner and a
// loopback Graph in live_console_order_pay_link_test.go). Evidence label of every passing line: MOCK (no PSP, no Graph in this file).
//
//   LCN12a TestLiveConsoleOrderForBuyerPrefill       A15: items from the bundle/conversation, live_quantity_remaining, customer/delivery only from an
//                                                    explicit link (never owner_id), conversation without peers -> 200 empty + unverified, authority
//   LCN12b TestLiveConsoleOrderForBuyerLivePrice     live price applied through the merchant-attested grant: Quote is the only price, ledger row,
//                                                    reservation row, audits, replay, second order on the bundle = 409 with the order id, cancel frees it
//   LCN12c TestLiveConsoleOrderForBuyerFailClosed    unknown peer / no conversation / no live:manage -> catalog price + reason; mismatched peers -> 409
//                                                    and nothing created; refused bodies; send without inbox:reply -> 409 capability, nothing created
//   LCN12d TestLiveConsoleOrderForBuyerConcurrentStaff  staff A and B on one bundle at once: one order, one hold, one 409
//   LCN12e TestLiveConsoleOrderForBuyerResume         kill after Place before the record: the replay completes, one order, one DM operation, a new link
//   LCN12f TestLiveConsoleOrderForBuyerPayLinkOutcomes  window closed / no conversation / not requested never fail the order
//   LCN12g TestLiveConsoleMerchantOriginGrantBranch   the SQL branch alone: bound to one quote, second quote = catalog, unbound / expired / foreign /
//                                                    archived / purged = catalog, single use under a race, fail-closed Begin
// Owner-pool writes (disclosed fixtures): bundle<->conversation peers, the conversation link, grant rows of the SQL-branch test, aging/archiving/
// purging for LCN12g, reading result tables. No product path writes those rows from the test.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/buyer"
	"livecommerce/internal/checkout"
	"livecommerce/internal/claims"
	"livecommerce/internal/command"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/inbox"
	"livecommerce/internal/merchanttools"
	"livecommerce/internal/platform"
	"livecommerce/internal/storefront"
)

// lbuPlanner records every pay-link DM the service asks for and answers like inbox.PlanOrderPayLink would: a saved receipt per (key, order), the probe
// (link == "") answers ErrPayLinkNotPlanned, a closed window answers window_closed.
type lbuPlanner struct {
	mu     sync.Mutex
	saved  map[string]inbox.SendOutput
	links  []string
	closed bool
}

func (p *lbuPlanner) PlanOrderPayLink(_ context.Context, _ pgx.Tx, _ platform.Scope, key, conversationID, orderID, link string) (inbox.SendOutput, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	id := key + "|" + conversationID + "|" + orderID
	if out, ok := p.saved[id]; ok {
		return out, nil
	}
	if p.closed {
		return inbox.SendOutput{}, &inbox.SendError{Status: 409, Code: "window_closed"}
	}
	if link == "" {
		return inbox.SendOutput{}, inbox.ErrPayLinkNotPlanned
	}
	out := inbox.SendOutput{OperationID: randomUUID(), OutboundID: randomUUID(), SendState: "queued"}
	p.saved[id] = out
	p.links = append(p.links, link)
	return out, nil
}

func (p *lbuPlanner) planned() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.links)
}

type lbuEnv struct {
	*ltgEnv
	manual  *merchanttools.ManualOrders
	fb      *merchanttools.ForBuyer
	plan    *lbuPlanner
	handler http.Handler
	homeKey string
}

// lbuNew is the ltgNew environment (store, market, delivery, claims harness) with the manual-order options (bank transfer 72 h), the
// for-buyer service on a recording planner and a merchant handler that mounts A15/A16.
func lbuNew(t *testing.T) *lbuEnv {
	t.Helper()
	e := ltgNew(t)
	e.grantCreator("orders:read", "integration:manage", "integration:read", "payments:refund", "fulfillment:write", "inbox:reply", "inbox:read", "customers:read")
	e.topUp()
	e.cofEnsureSettings(0, true, true, 72)
	manual := mtManualOrders(t, e.tcvEnv)
	plan := &lbuPlanner{saved: map[string]inbox.SendOutput{}}
	fb, err := merchanttools.NewForBuyer(manual, plan)
	if err != nil {
		t.Fatal(err)
	}
	env := &lbuEnv{ltgEnv: e, manual: manual, fb: fb, plan: plan,
		handler: httpapi.NewHandler(e.p.f.runtime, httpapi.Options{CVS: e.cvs, ManualOrders: manual, ForBuyer: fb})}
	options, err := manual.Options(context.Background(), e.token(), e.store())
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range options {
		if o.DeliveryKind == "home" && strings.Contains(strings.Join(o.PaymentModes, ","), "bank_transfer") {
			env.homeKey = o.OptionKey
		}
	}
	if env.homeKey == "" {
		t.Fatalf("no home + bank transfer option in %+v", options)
	}
	return env
}

func (e *lbuEnv) admin(token string) mtAdmin {
	return mtAdmin{t: e.t, h: e.handler, store: e.store(), token: token}
}

// body is the POST orders/for-buyer body for qty of the money SKU.
func (e *lbuEnv) body(qty int, bundles []string, conversation string, send bool) map[string]any {
	b := mtBody(e.money, qty, e.homeKey, "bank_transfer", mtHome)
	target := map[string]any{"bundle_ids": bundles, "conversation_id": nil}
	if conversation != "" {
		target["conversation_id"] = conversation
	}
	if bundles == nil {
		target["bundle_ids"] = []string{}
	}
	b["for"] = target
	b["send_payment_link"] = send
	return b
}

func (e *lbuEnv) post(token, key string, body map[string]any) (int, map[string]any) {
	e.t.Helper()
	raw, _ := json.Marshal(body)
	w := e.admin(token).call("POST", "/orders/for-buyer", key, "application/json", raw)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func (e *lbuEnv) get(token, query string) (int, map[string]any) {
	e.t.Helper()
	w := e.admin(token).call("GET", "/inbox/order-prefill"+query, "", "", nil)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

// scenario is a fresh session with a live-priced offer A1 (max 5), one claimed bundle "A1+2" and a conversation whose peer is linked to the bundle.
func (e *lbuEnv) scenario() (session string, offer claims.Offer, bundle, conversation string) {
	e.t.Helper()
	session, offer = e.session("A1", ltgLive, 5)
	bundle = e.h.accepted(e.t, session, "", "amy-"+t04Tag(), "A1+2").BundleID
	e.h.closeWindow(e.t, session) // the store caps its OPEN windows; the claim is recorded, the window is not needed any more
	conversation = lcConversation(e.t, e.p.f, e.tenant(), e.store(), "page")
	e.linkPeer(bundle, conversation)
	return
}

// linkPeer records the peer of the conversation on the bundle exactly as the LC-B4 Finish hook does after a SUCCEEDED private reply (owner fixture).
func (e *lbuEnv) linkPeer(bundle, conversation string) {
	e.t.Helper()
	mustExec(e.t, e.p.f.owner, `INSERT INTO inbox.bundle_peers(tenant_id,store_id,bundle_id,peer_key,app_id,object,asset_id,operation_id)
		SELECT tenant_id,store_id,$2::uuid,peer_key,app_id,object,asset_id,gen_random_uuid() FROM social.conversations WHERE id=$1`, conversation, bundle)
}

// member is a merchant principal with exactly the permissions the for-buyer routes need plus extra.
func (e *lbuEnv) member(extra ...string) string {
	token, _ := e.tcvEnv.member(append([]string{"inventory:reserve", "catalog:read", "orders:read"}, extra...)...)
	return token
}

// unit returns the unit price and price rule of the single line of the order's quote snapshot.
func (e *lbuEnv) unit(order string) (price int64, rule string) {
	e.t.Helper()
	var p int64
	var r *string
	if err := e.p.f.owner.QueryRow(context.Background(), `SELECT (x->>'unit_price_minor')::bigint, x->>'price_rule' FROM checkout.orders o
		JOIN storefront.quotes q ON q.id=o.quote_id CROSS JOIN LATERAL jsonb_array_elements(q.snapshot->'lines') x WHERE o.id=$1`, order).Scan(&p, &r); err != nil {
		e.t.Fatalf("order %s quote line: %v", order, err)
	}
	if r == nil {
		return p, ""
	}
	return p, *r
}

func (e *lbuEnv) orders() int {
	return e.count(`SELECT count(*) FROM checkout.orders WHERE store_id=$1 AND source='merchant_manual'`, e.store())
}

func (e *lbuEnv) reserved() int64 {
	_, r, _ := e.cofBalance(e.money)
	return r
}

func lbuStr(m map[string]any, k string) string { s, _ := m[k].(string); return s }

func lbuCode(m map[string]any) string { return lbuStr(m, "code") }

// ---------------------------------------------------------------------------------------------------------------------------------------

func TestLiveConsoleOrderForBuyerPrefill(t *testing.T) {
	e := lbuNew(t)
	_, offer, bundle, conv := e.scenario()
	token := e.token()

	t.Run("bundle: items, live price, remaining quantity, nothing pre-filled, no live price without a thread", func(t *testing.T) {
		st, out := e.get(token, "?bundle_id="+bundle)
		items, _ := out["items"].([]any)
		if st != 200 || len(items) != 1 {
			t.Fatalf("prefill: %d %v", st, out)
		}
		it := items[0].(map[string]any)
		if lbuStr(it, "sku_id") != e.money || lbuStr(it, "offer_id") != offer.ID || it["quantity"] != float64(2) || it["live_price_minor"] != float64(ltgLive) ||
			it["catalog_price_minor"] != float64(ltgCatalog) || it["sellable"] != true || it["live_quantity_remaining"] != float64(2) || lbuStr(it, "keyword") == "" || lbuStr(it, "name") == "" {
			t.Fatalf("item: %v", it)
		}
		if out["customer"] != nil || out["last_delivery"] != nil || out["suggested_option_key"] != nil || out["live_price_eligible"] != false || out["live_price_reason"] != "no_conversation" {
			t.Fatalf("a bare bundle pre-fills nothing and has no live price: %v", out)
		}
	})
	t.Run("conversation with a linked peer: eligible", func(t *testing.T) {
		st, out := e.get(token, "?conversation_id="+conv)
		if st != 200 || len(out["items"].([]any)) != 1 || out["live_price_eligible"] != true || out["live_price_reason"] != "" || out["customer"] != nil || out["last_delivery"] != nil {
			t.Fatalf("prefill by conversation: %d %v", st, out)
		}
		if b := out["bundles"].([]any); len(b) != 1 || b[0] != bundle {
			t.Fatalf("bundles: %v", out["bundles"])
		}
	})
	t.Run("P2-7 c: a conversation with no bundle peer row is 200 empty, not 404", func(t *testing.T) {
		lonely := lcConversation(t, e.p.f, e.tenant(), e.store(), "page")
		st, out := e.get(token, "?conversation_id="+lonely)
		if st != 200 || len(out["items"].([]any)) != 0 || out["live_price_eligible"] != false || out["live_price_reason"] != "bundle_buyer_unverified" {
			t.Fatalf("unlinked thread: %d %v", st, out)
		}
	})
	t.Run("another store's conversation or bundle is 404; bad queries are 422; authority", func(t *testing.T) {
		foreign := lcConversation(t, e.p.f, e.p.f.tenantB, e.p.f.storeB, "page")
		if st, _ := e.get(token, "?conversation_id="+foreign); st != 404 {
			t.Fatalf("foreign conversation: %d", st)
		}
		if st, _ := e.get(token, "?bundle_id="+randomUUID()); st != 404 {
			t.Fatalf("unknown bundle: %d", st)
		}
		for _, q := range []string{"", "?bundle_id=" + bundle + "&conversation_id=" + conv, "?bundle_id=nope", "?x=" + bundle, "?bundle_id=" + bundle + "&bundle_id=" + bundle} {
			if st, _ := e.get(token, q); st != 422 {
				t.Fatalf("query %q: %d want 422", q, st)
			}
		}
		noOrders, _ := e.tcvEnv.member("inventory:reserve")
		if st, _ := e.get(noOrders, "?bundle_id="+bundle); st != 403 {
			t.Fatalf("without orders:read: %d", st)
		}
		noReserve, _ := e.tcvEnv.member("orders:read")
		if st, _ := e.get(noReserve, "?bundle_id="+bundle); st != 403 {
			t.Fatalf("without inventory:reserve: %d", st)
		}
		if st, _ := e.get("", "?bundle_id="+bundle); st != 401 {
			t.Fatalf("no bearer: %d", st)
		}
	})
	t.Run("I09: customer and delivery come only from an explicit link, never from the bundle owner", func(t *testing.T) {
		// A buyer redeems the claim link and places a real order (so claims.bundles.owner_id is set and the owner has a last delivery).
		s, _ := e.session("A1", ltgLive, 5)
		ownedBundle, l := e.claimLink(s, "bob-"+t04Tag(), "A1+2")
		b := e.redeemed(l)
		q, _ := e.line(b, "")
		if _, err := e.placeHome(b, q); err != nil {
			t.Fatalf("buyer order: %v", err)
		}
		other := lcConversation(t, e.p.f, e.tenant(), e.store(), "page")
		e.linkPeer(ownedBundle, other)
		for _, query := range []string{"?bundle_id=" + ownedBundle, "?conversation_id=" + other} {
			st, out := e.get(token, query)
			if st != 200 || out["customer"] != nil || out["last_delivery"] != nil {
				t.Fatalf("%s pre-filled from the bundle owner: %d %v", query, st, out)
			}
			it := out["items"].([]any)[0].(map[string]any)
			if it["live_quantity_remaining"] != float64(0) {
				t.Fatalf("P2-7 a: the claimed quantity is held by the buyer's order, remaining must be 0: %v", it)
			}
		}
	})
	t.Run("the explicitly linked customer's last contact and delivery pre-fill (A14 link)", func(t *testing.T) {
		_, _, b2, c2 := e.scenario()
		st, out := e.post(token, t04Key("lcn12-prefill-order"), e.body(2, []string{b2}, c2, false))
		if st != 201 {
			t.Fatalf("seed order: %d %v", st, out)
		}
		var owner string
		if err := e.p.f.owner.QueryRow(context.Background(), `SELECT owner_id::text FROM checkout.orders WHERE id=$1`, lbuStr(out, "order_id")).Scan(&owner); err != nil {
			t.Fatal(err)
		}
		other := lcConversation(t, e.p.f, e.tenant(), e.store(), "page")
		mustExec(t, e.p.f.owner, `UPDATE inbox.conversation_state SET customer_id=$2 WHERE conversation_id=$1`, other, owner)
		st, got := e.get(token, "?conversation_id="+other)
		cust, _ := got["customer"].(map[string]any)
		ld, _ := got["last_delivery"].(map[string]any)
		if st != 200 || cust == nil || cust["name"] != "王小明" || cust["phone"] != "0912-345-678" || ld == nil || ld["option_key"] != e.homeKey {
			t.Fatalf("linked prefill: %d %v", st, got)
		}
		if addr, _ := ld["home_address"].(map[string]any); addr == nil || addr["city"] != "中正區" {
			t.Fatalf("home address: %v", ld)
		}
		// The link is read through inbox.conversation_meta, which re-checks inbox:read: without it the answer is 200 with nothing pre-filled.
		noInbox, _ := e.tcvEnv.member("orders:read", "inventory:reserve")
		if st, got = e.get(noInbox, "?conversation_id="+other); st != 200 || got["customer"] != nil || got["last_delivery"] != nil {
			t.Fatalf("without inbox:read the link is unknown: %d %v", st, got)
		}
	})
}

// ---------------------------------------------------------------------------------------------------------------------------------------

func TestLiveConsoleOrderForBuyerLivePrice(t *testing.T) {
	e := lbuNew(t)
	token := e.token()
	_, offer, bundle, conv := e.scenario()
	key := t04Key("lcn12-live")
	reservedBefore, ordersBefore := e.reserved(), e.orders()

	st, out := e.post(token, key, e.body(2, []string{bundle}, conv, true))
	if st != 201 || out["live_price"] != "applied" || out["live_price_reason"] != "" || out["source"] != "merchant_manual" || out["payment_mode"] != "bank_transfer" {
		t.Fatalf("for-buyer order: %d %v", st, out)
	}
	order := lbuStr(out, "order_id")
	send, _ := out["send"].(map[string]any)
	if send["state"] != "queued" || lbuStr(send, "operation_id") == "" {
		t.Fatalf("DM outcome: %v", out["send"])
	}
	t.Run("the Quote is the only price: live unit price on a live_claim line, the total follows", func(t *testing.T) {
		if p, r := e.unit(order); p != ltgLive || r != "live_claim" {
			t.Fatalf("unit price %d rule %q, want %d live_claim", p, r, ltgLive)
		}
		if got := e.reserved(); got != reservedBefore+2 {
			t.Fatalf("stock reserved %d -> %d, want +2 (one hold, begin_hold)", reservedBefore, got)
		}
		if got := int64(out["total_minor"].(float64)); got < 2*ltgLive || got >= 2*ltgCatalog {
			t.Fatalf("total %d is not a live-price total", got)
		}
	})
	t.Run("ledger, reservation row, single-use grant, audit", func(t *testing.T) {
		if qty, rows := e.held(bundle, offer.ID); qty != 2 || rows != 1 {
			t.Fatalf("0105 ledger: held %d rows %d", qty, rows)
		}
		if n := e.count(`SELECT count(*) FROM inbox.order_for_buyer WHERE bundle_id=$1 AND state='placed' AND order_id=$2 AND conversation_id=$3`, bundle, order, conv); n != 1 {
			t.Fatalf("reservation rows placed for the order: %d", n)
		}
		if n := e.count(`SELECT count(*) FROM claims.merchant_origin_grants g JOIN checkout.orders o ON o.owner_id=g.buyer_id WHERE g.bundle_id=$1 AND o.id=$2
			AND g.consumed_at IS NOT NULL AND g.quote_id=o.quote_id`, bundle, order); n != 1 {
			t.Fatalf("a consumed grant bound to the order's quote: %d", n)
		}
		for action, want := range map[string]int{"order.manual_created": 1, "order.for_buyer_created": 1, "claims.merchant_origin_granted": 1} {
			if n := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action=$2`, e.store(), action); n != want {
				t.Fatalf("audit %s: %d want %d", action, n, want)
			}
		}
		var detail string
		if err := e.p.f.owner.QueryRow(context.Background(), `SELECT details::text FROM ops.audit_events WHERE store_id=$1 AND action='claims.merchant_origin_granted'`, e.store()).Scan(&detail); err != nil ||
			!strings.Contains(detail, bundle) || !strings.Contains(detail, order) || !strings.Contains(detail, fmt.Sprint(ltgLive)) {
			t.Fatalf("grant audit detail (principal, bundle, buyer, order, lines, live price): %q %v", detail, err)
		}
		if e.plan.planned() != 1 {
			t.Fatalf("pay-link DM planned %d times", e.plan.planned())
		}
	})
	t.Run("replay with the same key: the same order, nothing new", func(t *testing.T) {
		st, again := e.post(token, key, e.body(2, []string{bundle}, conv, true))
		if st != 200 || lbuStr(again, "order_id") != order || again["live_price"] != "applied" {
			t.Fatalf("replay: %d %v", st, again)
		}
		if ssend, _ := again["send"].(map[string]any); lbuStr(ssend, "operation_id") != lbuStr(send, "operation_id") {
			t.Fatalf("replay planned another DM: %v vs %v", ssend, send)
		}
		if e.orders() != ordersBefore+1 || e.plan.planned() != 1 || e.reserved() != reservedBefore+2 {
			t.Fatalf("replay changed state: orders %d, DMs %d, reserved %d", e.orders(), e.plan.planned(), e.reserved())
		}
		if st, _ = e.post(token, key, e.body(3, []string{bundle}, conv, true)); st != 409 {
			t.Fatalf("the same key with another body: %d", st)
		}
	})
	t.Run("a second order for the bundle (another key) is 409 bundle_already_ordered with the order id, no second hold", func(t *testing.T) {
		st, refused := e.post(token, t04Key("lcn12-live-again"), e.body(2, []string{bundle}, conv, false))
		details, _ := refused["details"].(map[string]any)
		if st != 409 || lbuCode(refused) != "bundle_already_ordered" || lbuStr(details, "order_id") != order {
			t.Fatalf("second order: %d %v", st, refused)
		}
		if e.orders() != ordersBefore+1 || e.reserved() != reservedBefore+2 {
			t.Fatalf("a refused second order changed state: orders %d reserved %d", e.orders(), e.reserved())
		}
	})
	t.Run("cancelling the order frees the bundle and the claimed quantity: the next order is live again", func(t *testing.T) {
		e.proAge(order)
		if d, _ := e.cofExpire(order); d != "EXPIRED" {
			t.Fatalf("expire_held: %s", d)
		}
		st, next := e.post(token, t04Key("lcn12-live-after-cancel"), e.body(2, []string{bundle}, conv, false))
		if st != 201 || next["live_price"] != "applied" {
			t.Fatalf("after the cancel: %d %v", st, next)
		}
		if p, r := e.unit(lbuStr(next, "order_id")); p != ltgLive || r != "live_claim" {
			t.Fatalf("second live order: %d %q", p, r)
		}
		if qty, _ := e.held(bundle, offer.ID); qty != 2 {
			t.Fatalf("held %d (the cancelled order's quantity came back, the new order holds it)", qty)
		}
	})
	t.Run("a quantity above the claim stays at the catalog price (fail closed, never a partial live price)", func(t *testing.T) {
		_, _, b3, c3 := e.scenario()
		st, over := e.post(token, t04Key("lcn12-over"), e.body(3, []string{b3}, c3, false))
		if st != 201 || over["live_price"] != "not_applied" {
			t.Fatalf("over the claim: %d %v", st, over)
		}
		if p, r := e.unit(lbuStr(over, "order_id")); p != ltgCatalog || r != "" {
			t.Fatalf("over the claim priced %d %q", p, r)
		}
	})
}

// ---------------------------------------------------------------------------------------------------------------------------------------

func TestLiveConsoleOrderForBuyerFailClosed(t *testing.T) {
	e := lbuNew(t)
	token := e.token()
	grants := func(bundle string) int {
		return e.count(`SELECT count(*) FROM claims.merchant_origin_grants WHERE bundle_id=$1`, bundle)
	}
	reservations := func(bundle string) int {
		return e.count(`SELECT count(*) FROM inbox.order_for_buyer WHERE bundle_id=$1`, bundle)
	}
	catalogOrder := func(what string, st int, out map[string]any, reason string) {
		t.Helper()
		if st != 201 || out["live_price"] != "not_applied" || out["live_price_reason"] != reason {
			t.Fatalf("%s: %d %v", what, st, out)
		}
		if p, r := e.unit(lbuStr(out, "order_id")); p != ltgCatalog || r != "" {
			t.Fatalf("%s: priced %d %q, want the catalog price and no live evidence", what, p, r)
		}
	}

	t.Run("a bundle with no peer key: catalog price, bundle_buyer_unverified, no grant", func(t *testing.T) {
		s, _ := e.session("A1", ltgLive, 5)
		bundle := e.h.accepted(t, s, "", "dan-"+t04Tag(), "A1+2").BundleID
		e.h.closeWindow(t, s)
		conv := lcConversation(t, e.p.f, e.tenant(), e.store(), "page")
		st, out := e.post(token, t04Key("lcn12-unverified"), e.body(2, []string{bundle}, conv, false))
		catalogOrder("unknown bundle peer", st, out, "bundle_buyer_unverified")
		if grants(bundle) != 0 || reservations(bundle) != 1 {
			t.Fatalf("grants %d reservations %d: the order still reserves its bundle, but never grants a price", grants(bundle), reservations(bundle))
		}
		if qty, rows := e.held(bundle, e.offerOf(t, bundle)); qty != 0 || rows != 0 {
			t.Fatalf("a catalog order consumed the claim: %d %d", qty, rows)
		}
	})
	t.Run("no conversation (e.g. from a comment with no linked thread): catalog price, no_conversation", func(t *testing.T) {
		_, _, bundle, _ := e.scenario()
		st, out := e.post(token, t04Key("lcn12-noconv"), e.body(2, []string{bundle}, "", false))
		catalogOrder("no conversation", st, out, "no_conversation")
		if grants(bundle) != 0 {
			t.Fatalf("grants written without a conversation: %d", grants(bundle))
		}
	})
	t.Run("a caller without live:manage: catalog price, permission", func(t *testing.T) {
		_, _, bundle, conv := e.scenario()
		st, out := e.post(e.member(), t04Key("lcn12-perm"), e.body(2, []string{bundle}, conv, false))
		catalogOrder("no live:manage", st, out, "permission")
		if grants(bundle) != 0 {
			t.Fatalf("grants written without live:manage: %d", grants(bundle))
		}
	})
	t.Run("both peer keys known and different: 409 bundle_buyer_mismatch and NOTHING created", func(t *testing.T) {
		_, _, bundle, _ := e.scenario()
		stranger := lcConversation(t, e.p.f, e.tenant(), e.store(), "page")
		ordersBefore, reservedBefore := e.orders(), e.reserved()
		st, out := e.post(token, t04Key("lcn12-mismatch"), e.body(2, []string{bundle}, stranger, true))
		if st != 409 || lbuCode(out) != "bundle_buyer_mismatch" {
			t.Fatalf("mismatch: %d %v", st, out)
		}
		if e.orders() != ordersBefore || e.reserved() != reservedBefore || grants(bundle) != 0 || reservations(bundle) != 0 || e.plan.planned() != 0 {
			t.Fatalf("a mismatch created something: orders %d reserved %d grants %d reservations %d DMs %d", e.orders(), e.reserved(), grants(bundle), reservations(bundle), e.plan.planned())
		}
	})
	t.Run("a bundle with two peers where one differs is a mismatch too (the server never picks a peer)", func(t *testing.T) {
		_, _, bundle, conv := e.scenario()
		e.linkPeer(bundle, lcConversation(t, e.p.f, e.tenant(), e.store(), "page"))
		if st, out := e.post(token, t04Key("lcn12-twopeers"), e.body(2, []string{bundle}, conv, false)); st != 409 || lbuCode(out) != "bundle_buyer_mismatch" {
			t.Fatalf("two peers: %d %v", st, out)
		}
	})
	t.Run("another store's conversation or bundle: 404, nothing created", func(t *testing.T) {
		_, _, bundle, _ := e.scenario()
		foreign := lcConversation(t, e.p.f, e.p.f.tenantB, e.p.f.storeB, "page")
		ordersBefore := e.orders()
		if st, out := e.post(token, t04Key("lcn12-foreign-conv"), e.body(2, []string{bundle}, foreign, false)); st != 404 {
			t.Fatalf("foreign conversation: %d %v", st, out)
		}
		if st, out := e.post(token, t04Key("lcn12-foreign-bundle"), e.body(2, []string{randomUUID()}, "", false)); st != 404 {
			t.Fatalf("unknown bundle: %d %v", st, out)
		}
		if e.orders() != ordersBefore {
			t.Fatalf("a 404 created an order")
		}
	})
	t.Run("P2-7 b: send_payment_link without inbox:reply is 409 capability, nothing created", func(t *testing.T) {
		_, _, bundle, conv := e.scenario()
		ordersBefore := e.orders()
		st, out := e.post(e.member("live:manage"), t04Key("lcn12-capability"), e.body(2, []string{bundle}, conv, true))
		if st != 409 || lbuCode(out) != "capability" || e.orders() != ordersBefore || reservations(bundle) != 0 || grants(bundle) != 0 {
			t.Fatalf("capability: %d %v orders %d reservations %d grants %d", st, out, e.orders(), reservations(bundle), grants(bundle))
		}
	})
	t.Run("refused bodies: a price key, unknown keys, bad ids, more than five bundles", func(t *testing.T) {
		_, _, bundle, conv := e.scenario()
		base := func() map[string]any { return e.body(2, []string{bundle}, conv, false) }
		withKey := func(k string, v any) map[string]any { b := base(); b[k] = v; return b }
		priced := base()
		priced["items"] = []map[string]any{{"sku_id": e.money, "quantity": 2, "price_minor": 1}}
		forExtra := base()
		forExtra["for"] = map[string]any{"bundle_ids": []string{bundle}, "conversation_id": conv, "tenant_id": randomUUID()}
		missing := base()
		delete(missing, "send_payment_link")
		six := make([]string, 6)
		for i := range six {
			six[i] = randomUUID()
		}
		dup := base()
		dup["for"] = map[string]any{"bundle_ids": []string{bundle, bundle}, "conversation_id": conv}
		bad := base()
		bad["for"] = map[string]any{"bundle_ids": []string{"not-a-uuid"}, "conversation_id": conv}
		for name, c := range map[string]struct {
			body map[string]any
			want int
		}{
			"price_minor in an item": {priced, 400}, "total_minor": {withKey("total_minor", 1), 400}, "tenant_id": {withKey("tenant_id", randomUUID()), 400},
			"unknown key in for": {forExtra, 400}, "missing send_payment_link": {missing, 422}, "six bundles": {withKey("for", map[string]any{"bundle_ids": six, "conversation_id": nil}), 422},
			"duplicate bundles": {dup, 422}, "bad bundle id": {bad, 422},
		} {
			if st, out := e.post(token, t04Key("lcn12-refused"), c.body); st != c.want {
				t.Fatalf("%s: %d %v want %d", name, st, out, c.want)
			}
		}
		if st, _ := e.post(token, "", base()); st != 422 {
			t.Fatalf("no Idempotency-Key: %d", st)
		}
	})
	t.Run("a plain manual order through the same route: no bundle, no price, no reservation", func(t *testing.T) {
		st, out := e.post(token, t04Key("lcn12-plain"), e.body(2, nil, "", false))
		catalogOrder("plain order", st, out, "no_bundle")
		if send, _ := out["send"].(map[string]any); send["state"] != "not_sent" || send["reason"] != "not_requested" {
			t.Fatalf("send: %v", out["send"])
		}
	})
}

// offerOf returns the id of the bundle's single claim line's offer (fixture read).
func (e *lbuEnv) offerOf(t *testing.T, bundle string) string {
	t.Helper()
	var offer string
	if err := e.p.f.owner.QueryRow(context.Background(), `SELECT offer_id::text FROM claims.lines WHERE bundle_id=$1`, bundle).Scan(&offer); err != nil {
		t.Fatal(err)
	}
	return offer
}

// ---------------------------------------------------------------------------------------------------------------------------------------

func TestLiveConsoleOrderForBuyerConcurrentStaff(t *testing.T) {
	e := lbuNew(t)
	staffA, staffB := e.member("live:manage", "inbox:reply"), e.member("live:manage", "inbox:reply")
	for round := 0; round < 4; round++ {
		_, _, bundle, conv := e.scenario()
		ordersBefore, reservedBefore := e.orders(), e.reserved()
		var wg sync.WaitGroup
		start := make(chan struct{})
		statuses, bodies := make([]int, 2), make([]map[string]any, 2)
		for i, tok := range []string{staffA, staffB} {
			wg.Add(1)
			go func(i int, tok string) {
				defer wg.Done()
				<-start
				statuses[i], bodies[i] = e.post(tok, t04Key(fmt.Sprintf("lcn12-race-%d", i)), e.body(2, []string{bundle}, conv, false))
			}(i, tok)
		}
		close(start)
		wg.Wait()
		created, refused := 0, 0
		for i, st := range statuses {
			switch {
			case st == 201:
				created++
			case st == 409 && lbuCode(bodies[i]) == "bundle_already_ordered":
				refused++
			default:
				t.Fatalf("round %d staff %d: %d %v", round, i, st, bodies[i])
			}
		}
		if created != 1 || refused != 1 || e.orders() != ordersBefore+1 || e.reserved() != reservedBefore+2 {
			t.Fatalf("round %d: created %d refused %d orders %d reserved %d, want one order, one hold, one 409", round, created, refused, e.orders()-ordersBefore, e.reserved()-reservedBefore)
		}
		if n := e.count(`SELECT count(*) FROM inbox.order_for_buyer WHERE bundle_id=$1 AND state IN ('pending','placed')`, bundle); n != 1 {
			t.Fatalf("round %d: %d live reservation rows for one bundle", round, n)
		}
	}
}

// ---------------------------------------------------------------------------------------------------------------------------------------

func TestLiveConsoleOrderForBuyerResume(t *testing.T) {
	e := lbuNew(t)
	token, store := e.token(), e.store()
	_, offer, bundle, conv := e.scenario()
	key := t04Key("lcn12-resume")
	in := merchanttools.ForBuyerInput{
		Items:       []merchanttools.ManualItem{{SKUID: e.money, Quantity: 2}},
		Customer:    merchanttools.ManualCustomer{Name: "王小明", Phone: "0912-345-678", Email: "buyer@example.test"},
		Delivery:    merchanttools.ManualDelivery{OptionKey: e.homeKey, HomeAddress: &storefront.HomeAddress{Region: "台北市", City: "中正區", PostalCode: "100", Line1: "忠孝東路1號"}},
		PaymentMode: "bank_transfer", Locale: "zh-TW",
		For:             merchanttools.ForBuyerTarget{BundleIDs: []string{bundle}, ConversationID: &conv},
		SendPaymentLink: true,
	}
	ordersBefore, reservedBefore := e.orders(), e.reserved()

	// Kill after Place (the order exists, the reservation row is still pending, no DM): the request dies as a process kill would.
	e.fb.FaultAfterPlace = func() error { return errors.New("killed after Begin") }
	if _, _, err := e.fb.Place(context.Background(), token, store, key, in); err == nil {
		t.Fatal("the fault hook did not abort the request")
	}
	e.fb.FaultAfterPlace = nil
	if e.orders() != ordersBefore+1 || e.plan.planned() != 0 {
		t.Fatalf("after the kill: orders %d DMs %d", e.orders()-ordersBefore, e.plan.planned())
	}
	if n := e.count(`SELECT count(*) FROM inbox.order_for_buyer WHERE bundle_id=$1 AND state='pending' AND order_id IS NULL`, bundle); n != 1 {
		t.Fatalf("after the kill the reservation row must be pending without an order: %d", n)
	}

	// The replay under the same key completes the record and the DM: one order, one hold, one DM operation, a NEW buyer link.
	out, replayed, err := e.fb.Place(context.Background(), token, store, key, in)
	if err != nil || replayed {
		t.Fatalf("resume: %+v replayed=%v err=%v", out, replayed, err)
	}
	if e.orders() != ordersBefore+1 || e.reserved() != reservedBefore+2 {
		t.Fatalf("resume created a second order or hold: orders %d reserved %d", e.orders()-ordersBefore, e.reserved()-reservedBefore)
	}
	if out.LivePrice != "applied" {
		t.Fatalf("the resumed order lost its live price: %q %q", out.LivePrice, out.LivePriceReason)
	}
	if p, r := e.unit(out.OrderID); p != ltgLive || r != "live_claim" {
		t.Fatalf("resumed order priced %d %q", p, r)
	}
	if n := e.count(`SELECT count(*) FROM inbox.order_for_buyer WHERE bundle_id=$1 AND state='placed' AND order_id=$2`, bundle, out.OrderID); n != 1 {
		t.Fatalf("the replay did not complete the reservation row: %d", n)
	}
	if out.Send.State != "queued" || out.Send.OperationID == "" || e.plan.planned() != 1 {
		t.Fatalf("resume DM: %+v planned %d", out.Send, e.plan.planned())
	}
	if qty, rows := e.held(bundle, offer.ID); qty != 2 || rows != 1 {
		t.Fatalf("ledger after resume: %d %d", qty, rows)
	}
	// §5.1 replay clause c: the DM carries a NEW link (regenerate-link logic); the link Place re-derived is superseded and dead.
	dmLink := e.plan.links[0]
	if out.BuyerLink == nil || dmLink == *out.BuyerLink {
		t.Fatalf("the DM must carry a regenerated link: dm=%q place=%v", dmLink, out.BuyerLink)
	}
	_, oldToken := e.mtLinkParts(*out.BuyerLink, "zh-TW")
	_, newToken := e.mtLinkParts(dmLink, "zh-TW")
	if res, _ := e.mtRedeem(out.OrderID, oldToken); res.status == 200 {
		t.Fatalf("the superseded link still redeems")
	}
	if res, _ := e.mtRedeem(out.OrderID, newToken); res.status != 200 {
		t.Fatalf("the DM link does not redeem: %d %s", res.status, res.body)
	}

	// A third call is a pure replay: same order, same DM operation, no new planning, no new link.
	again, replayed, err := e.fb.Place(context.Background(), token, store, key, in)
	if err != nil || !replayed || again.OrderID != out.OrderID || again.Send.OperationID != out.Send.OperationID || e.plan.planned() != 1 {
		t.Fatalf("pure replay: %+v replayed=%v err=%v planned=%d", again, replayed, err, e.plan.planned())
	}
	if again.BuyerLink != nil {
		t.Fatalf("a pure replay returns no buyer link (§5.1): %v", *again.BuyerLink)
	}
}

// ---------------------------------------------------------------------------------------------------------------------------------------

func TestLiveConsoleOrderForBuyerPayLinkOutcomes(t *testing.T) {
	e := lbuNew(t)
	token := e.token()
	t.Run("window closed: the order is created, send says window_closed, no link was re-issued for nothing", func(t *testing.T) {
		_, _, bundle, conv := e.scenario()
		e.plan.closed = true
		defer func() { e.plan.closed = false }()
		st, out := e.post(token, t04Key("lcn12-closed"), e.body(2, []string{bundle}, conv, true))
		send, _ := out["send"].(map[string]any)
		if st != 201 || send["state"] != "not_sent" || send["reason"] != "window_closed" || lbuStr(out, "buyer_link") == "" {
			t.Fatalf("closed window: %d %v", st, out)
		}
		if e.plan.planned() != 0 {
			t.Fatalf("a DM was planned in a closed window")
		}
	})
	t.Run("no conversation: not_sent no_conversation", func(t *testing.T) {
		_, _, bundle, _ := e.scenario()
		st, out := e.post(token, t04Key("lcn12-noconv-dm"), e.body(2, []string{bundle}, "", true))
		if send, _ := out["send"].(map[string]any); st != 201 || send["state"] != "not_sent" || send["reason"] != "no_conversation" {
			t.Fatalf("no conversation: %d %v", st, out)
		}
	})
	t.Run("not requested: not_sent not_requested; the first-run DM carries Place's own link", func(t *testing.T) {
		_, _, bundle, conv := e.scenario()
		st, out := e.post(token, t04Key("lcn12-nodm"), e.body(2, []string{bundle}, conv, false))
		if send, _ := out["send"].(map[string]any); st != 201 || send["reason"] != "not_requested" {
			t.Fatalf("not requested: %d %v", st, out)
		}
		before := e.plan.planned()
		_, _, b2, c2 := e.scenario()
		st, out = e.post(token, t04Key("lcn12-dm"), e.body(2, []string{b2}, c2, true))
		if st != 201 || e.plan.planned() != before+1 || e.plan.links[before] != lbuStr(out, "buyer_link") {
			t.Fatalf("first run: %d %v dm links %v", st, out, e.plan.links)
		}
	})
}

// ---------------------------------------------------------------------------------------------------------------------------------------

// grantFixture is a buyer whose cart holds the money SKU with the claim origin of a bundle it does NOT own, plus (optionally) a grant row written
// by the owner pool (a disclosed fixture: in the product the row is written only by claims.for_buyer_begin).
type grantFixture struct {
	e               *lbuEnv
	buyer           *tcvBuyer
	session, bundle string
	offer           claims.Offer
}

func (e *lbuEnv) grantBuyer(withGrant bool) *grantFixture {
	e.t.Helper()
	session, offer := e.session("A1", ltgLive, 5)
	bundle := e.h.accepted(e.t, session, "", "gil-"+t04Tag(), "A1+2").BundleID
	e.h.closeWindow(e.t, session)
	b := e.buyerCap()
	if _, err := e.h.putCart(b.cap, t04Key("lcn12-grant-cart"), storefront.CartInput{Items: []storefront.Item{{SKUID: e.money, Quantity: ltgQty}},
		Origins: map[string]storefront.ClaimOrigin{e.money: {BundleID: bundle, OfferID: offer.ID, Quantity: 2}}}); err != nil {
		e.t.Fatalf("cart with a server-side origin: %v", err)
	}
	g := &grantFixture{e: e, buyer: b, session: session, bundle: bundle, offer: offer}
	if withGrant {
		g.insertGrant(b.cap.Scope.OwnerID)
	}
	return g
}

func (g *grantFixture) insertGrant(owner string) {
	g.e.t.Helper()
	mustExec(g.e.t, g.e.p.f.owner, `INSERT INTO claims.merchant_origin_grants(tenant_id,store_id,request_id,buyer_id,bundle_id,principal_id,expires_at)
		VALUES($1,$2,gen_random_uuid(),$3,$4,$5,clock_timestamp()+interval '15 minutes')`, g.e.tenant(), g.e.store(), owner, g.bundle, g.e.p.f.principalA)
}

func (g *grantFixture) bind(quote string) (int, error) {
	var n int
	err := buyer.WithScope(context.Background(), g.e.p.a.runtime, g.buyer.cap.Token, g.e.store(), func(ctx context.Context, tx pgx.Tx, _ buyer.Scope) error {
		var inner error
		n, inner = claims.BindMerchantOriginGrant(ctx, tx, quote)
		return inner
	})
	return n, err
}

func TestLiveConsoleMerchantOriginGrantBranch(t *testing.T) {
	e := lbuNew(t)
	owner := e.p.f.owner

	t.Run("bound to one quote: the second quote on the same grant is catalog; Begin of the first is live, consumes the grant once", func(t *testing.T) {
		g := e.grantBuyer(true)
		q1, l1 := e.line(g.buyer, "")
		e.wantLive("first quote on the grant", l1, ltgLive, g.bundle, g.offer.ID)
		if n, err := g.bind(q1.ID); err != nil || n != 1 {
			t.Fatalf("bind: %d %v", n, err)
		}
		_, l2 := e.line(g.buyer, "")
		e.wantCatalog("second quote on a bound grant", l2)
		if got := e.cartLive(g.buyer); got != 0 {
			t.Fatalf("the cart preview shows a live price for a grant bound to a quote: %d", got)
		}
		placed, err := e.placeHome(g.buyer, q1)
		if err != nil {
			t.Fatalf("Begin of the bound quote: %v", err)
		}
		if p, r := e.unit(placed.OrderID); p != ltgLive || r != "live_claim" {
			t.Fatalf("order unit price %d %q", p, r)
		}
		if qty, rows := e.held(g.bundle, g.offer.ID); qty != 2 || rows != 1 {
			t.Fatalf("ledger %d %d", qty, rows)
		}
		if n := e.count(`SELECT count(*) FROM claims.merchant_origin_grants WHERE bundle_id=$1 AND consumed_at IS NOT NULL AND quote_id=$2`, g.bundle, q1.ID); n != 1 {
			t.Fatalf("the grant is not consumed and bound to the quote: %d", n)
		}
		e.reCart(g.buyer, ltgQty)
		_, l3 := e.line(g.buyer, "")
		e.wantCatalog("quote after the grant was consumed", l3)
	})
	t.Run("an unbound grant prices a quote but Begin refuses it (fail closed): no order, no hold, no ledger row", func(t *testing.T) {
		g := e.grantBuyer(true)
		q, l := e.line(g.buyer, "")
		e.wantLive("unbound grant quote", l, ltgLive, g.bundle, g.offer.ID)
		before := e.reserved()
		if _, err := e.placeHome(g.buyer, q); err == nil {
			t.Fatal("Begin placed an order on a grant that was never bound to the quote")
		}
		if e.count(`SELECT count(*) FROM checkout.orders WHERE owner_id=$1`, g.buyer.cap.Scope.OwnerID) != 0 || e.reserved() != before {
			t.Fatalf("a refused Begin left an order or a hold")
		}
		if qty, rows := e.held(g.bundle, g.offer.ID); qty != 0 || rows != 0 {
			t.Fatalf("ledger %d %d", qty, rows)
		}
	})
	t.Run("no grant, an expired grant and another buyer's grant price nothing", func(t *testing.T) {
		g := e.grantBuyer(false)
		_, l := e.line(g.buyer, "")
		e.wantCatalog("a forged origin without any grant", l)
		g.insertGrant(randomUUID()) // another buyer's grant (a random owner id: no row binds it to this cart's owner)
		_, l = e.line(g.buyer, "")
		e.wantCatalog("another buyer's grant", l)
		g.insertGrant(g.buyer.cap.Scope.OwnerID)
		mustExec(t, owner, `UPDATE claims.merchant_origin_grants SET expires_at=clock_timestamp()-interval '1 second' WHERE bundle_id=$1`, g.bundle)
		_, l = e.line(g.buyer, "")
		e.wantCatalog("an expired grant", l)
	})
	t.Run("a purged bundle and an archived session price nothing", func(t *testing.T) {
		g := e.grantBuyer(true)
		mustExec(t, owner, `UPDATE claims.bundles SET purged_at=clock_timestamp() WHERE id=$1`, g.bundle)
		_, l := e.line(g.buyer, "")
		e.wantCatalog("a purged bundle", l)
		mustExec(t, owner, `UPDATE claims.bundles SET purged_at=NULL WHERE id=$1`, g.bundle)
		_, l = e.line(g.buyer, "")
		e.wantLive("the same grant before the session is archived", l, ltgLive, g.bundle, g.offer.ID)
		mustExec(t, owner, `UPDATE live.sessions SET lifecycle='archived' WHERE id=$1`, g.session)
		_, l = e.line(g.buyer, "")
		e.wantCatalog("an archived session", l)
	})
	t.Run("the 0105 ledger stays the ceiling: a grant never prices more than the claimed quantity left", func(t *testing.T) {
		g := e.grantBuyer(true)
		q, _ := e.line(g.buyer, "")
		if _, err := g.bind(q.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := e.placeHome(g.buyer, q); err != nil {
			t.Fatal(err)
		}
		// a second grant for the same, now fully consumed claim line (written by the fixture) prices nothing: remaining quantity is 0
		g2 := &grantFixture{e: e, session: g.session, bundle: g.bundle, offer: g.offer, buyer: e.buyerCap()}
		if _, err := e.h.putCart(g2.buyer.cap, t04Key("lcn12-grant-cart2"), storefront.CartInput{Items: []storefront.Item{{SKUID: e.money, Quantity: ltgQty}},
			Origins: map[string]storefront.ClaimOrigin{e.money: {BundleID: g.bundle, OfferID: g.offer.ID, Quantity: 2}}}); err != nil {
			t.Fatal(err)
		}
		g2.insertGrant(g2.buyer.cap.Scope.OwnerID)
		_, l := e.line(g2.buyer, "")
		e.wantCatalog("a grant on a fully consumed claim line", l)
	})
	t.Run("single use under a race: two Begins of one bound quote place exactly one order", func(t *testing.T) {
		g := e.grantBuyer(true)
		q, _ := e.line(g.buyer, "")
		if _, err := g.bind(q.ID); err != nil {
			t.Fatal(err)
		}
		cart := e.h.cartOf(t, g.buyer.cap)
		din := bdHome(cart)
		din.ExpectedVersion = g.buyer.h.destination.Version
		dest, err := bdSet(g.buyer.h.cqHarness, t04Key("lcn12-race-dest"), din)
		if err != nil {
			t.Fatal(err)
		}
		in := checkout.Input{QuoteID: q.ID, DestinationID: dest.ID, CartVersion: cart.Version, ServiceVersion: 1, AllocationVersion: 1}
		errs := make([]error, 2)
		var wg sync.WaitGroup
		start := make(chan struct{})
		for i := range errs {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				_, errs[i] = e.svc.Begin(context.Background(), g.buyer.cap.Token, e.store(), t04Key("lcn12-grant-race"), in)
			}(i)
		}
		close(start)
		wg.Wait()
		won := 0
		for _, err := range errs {
			if err == nil {
				won++
			} else if !errors.Is(err, command.ErrConflict) {
				t.Fatalf("a losing Begin must be the typed conflict: %v", err)
			}
		}
		if won != 1 || e.count(`SELECT count(*) FROM checkout.orders WHERE owner_id=$1`, g.buyer.cap.Scope.OwnerID) != 1 {
			t.Fatalf("won %d", won)
		}
		if qty, rows := e.held(g.bundle, g.offer.ID); qty != 2 || rows != 1 {
			t.Fatalf("ledger %d %d", qty, rows)
		}
	})
}
