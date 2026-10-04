package foundation_test

// merchant_tools_link_smoke_test.go: author smoke of the manual-order buyer link exchange and of the order source projection (contracts/storefront-v2.md
// G3, migration 0094: checkout.order_links, checkout.redeem_order_link, buyer.issue_owner_capability, identity.read_order_sources) through the REAL buyer HTTP
// handler and the REAL merchant handler on real PostgreSQL. Evidence label: REAL_PG.
//   MTL01 TestMerchantToolsOrderLinkExchange   single use (also under concurrency), wrong token/order/store = the same refusal and NOT consuming the link,
//                                              expiry, hashed storage, <= 7 days, full rights (the buyer can submit the transfer proof), owner scope,
//                                              replay of the placement returns the same (dead after use) link, the throttle
//   MTL02 TestMerchantToolsOrderSource         the merchant orders list and detail carry source (merchant_manual vs storefront)
// Owner-pool writes (disclosed fixtures): aging a link row (created_at/expires_at) to test expiry.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"
)

func mtRefusalBody(t *testing.T, res bhResponse) string {
	t.Helper()
	var out struct{ Code string }
	_ = json.Unmarshal(res.body, &out)
	return fmt.Sprintf("%d/%s", res.status, out.Code)
}

func TestMerchantToolsOrderLinkExchange(t *testing.T) {
	e, a, _ := mtOrderEnv(t)
	sku := e.p.stock.skus[0].ID
	ctx := context.Background()
	var homeKey string
	for _, o := range a.options() {
		if o.DeliveryKind == "home" && len(o.PaymentModes) > 0 {
			homeKey = o.OptionKey
		}
	}
	place := func(label string) (order, linkToken, link string, key string) {
		key = t04Key("mtl-" + label)
		code, out := a.place(key, mtBody(sku, 1, homeKey, "bank_transfer", mtHome))
		if code != 201 {
			t.Fatalf("place %s: %d %v", label, code, out)
		}
		link, _ = out["buyer_link"].(string)
		order, linkToken = e.mtLinkParts(link, "zh-TW")
		return
	}
	order1, linkToken1, link1, key1 := place("one")

	t.Run("the link token exists only hashed, within 7 days", func(t *testing.T) {
		hash := sha256.Sum256([]byte(linkToken1))
		var n int
		var created, expires time.Time
		if err := e.p.f.owner.QueryRow(ctx, `SELECT count(*),min(created_at),min(expires_at) FROM checkout.order_links WHERE token_hash=$1 AND order_id=$2 AND redeemed_at IS NULL`, hash[:], order1).Scan(&n, &created, &expires); err != nil || n != 1 {
			t.Fatalf("link row: %d %v", n, err)
		}
		if d := expires.Sub(created); d <= 6*24*time.Hour || d > 7*24*time.Hour {
			t.Fatalf("expiry window %s, want about 7 days", d)
		}
		var dump string
		if err := e.p.f.owner.QueryRow(ctx, `SELECT coalesce(string_agg(l::text,'|'),'') FROM checkout.order_links l WHERE store_id=$1`, e.store()).Scan(&dump); err != nil || bytes.Contains([]byte(dump), []byte(linkToken1)) {
			t.Fatalf("the plaintext link token must never be stored: %v", err)
		}
	})

	t.Run("wrong token, wrong order and another store are the same refusal and do not consume the link", func(t *testing.T) {
		wrongToken := randomToken()
		other := randomUUID()
		r1, _ := e.mtRedeem(order1, wrongToken)
		r2, _ := e.mtRedeem(other, linkToken1)
		// another store: the SQL exchange with a different store id (the origin resolver is what picks the store over HTTP)
		var rows int
		hash, bearer := sha256.Sum256([]byte(linkToken1)), sha256.Sum256([]byte(randomToken()))
		ip := sha256.Sum256([]byte("203.0.113.77"))
		err := e.p.a.issuer.QueryRow(ctx, `SELECT count(*) FROM checkout.redeem_order_link($1::uuid,$2::uuid,$3,$4,$5::bigint,$6)`, randomUUID(), order1, hash[:], bearer[:], int64(3600), ip[:]).Scan(&rows)
		if err != nil || rows != 0 {
			t.Fatalf("another store: rows=%d err=%v", rows, err)
		}
		if mtRefusalBody(t, r1) != "404/not_found" || mtRefusalBody(t, r2) != "404/not_found" {
			t.Fatalf("refusals must be identical 404 not_found: %s %s", mtRefusalBody(t, r1), mtRefusalBody(t, r2))
		}
		if n := e.count(`SELECT count(*) FROM checkout.order_links WHERE order_id=$1 AND redeemed_at IS NOT NULL`, order1); n != 0 {
			t.Fatal("a refused attempt consumed the link")
		}
	})

	var capability string
	t.Run("single use: the first exchange wins with a FULL capability, the second is the same refusal", func(t *testing.T) {
		first, bearer := e.mtRedeem(order1, linkToken1)
		if first.status != 200 {
			t.Fatalf("first: %d %s", first.status, first.body)
		}
		capability = bearer
		second, _ := e.mtRedeem(order1, linkToken1)
		if mtRefusalBody(t, second) != "404/not_found" {
			t.Fatalf("second use: %s", mtRefusalBody(t, second))
		}
		var view *string
		if err := e.p.f.owner.QueryRow(ctx, `SELECT view_order_id::text FROM buyer.capability_sessions WHERE token_hash=$1`, func() []byte { h := sha256.Sum256([]byte(capability)); return h[:] }()).Scan(&view); err != nil || view != nil {
			t.Fatalf("the exchanged capability must be FULL (view_order_id NULL): %v %v", view, err)
		}
		// Full rights: the buyer reads the order and submits the transfer proof (a view-only session may not).
		if res := e.bh.request(t, "GET", "/v1/buyer/orders/"+order1+"/bank-transfer", capability, "", nil, nil); res.status != 200 {
			t.Fatalf("view: %d %s", res.status, res.body)
		}
		proof := e.bh.request(t, "PUT", "/v1/buyer/orders/"+order1+"/bank-transfer/proof", capability, t04Key("mtl-proof"),
			map[string]any{"last5": "12345", "amount_minor": 1250, "paid_at": time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)}, nil)
		if proof.status != 200 {
			t.Fatalf("proof with the exchanged capability: %d %s", proof.status, proof.body)
		}
		// Owner scope: this capability belongs to this manual order's owner only; the harness store's other orders are not readable.
		var other string
		if err := e.p.f.owner.QueryRow(ctx, `SELECT id::text FROM checkout.orders WHERE store_id=$1 AND id<>$2 ORDER BY created_at LIMIT 1`, e.store(), order1).Scan(&other); err == nil {
			if res := e.bh.request(t, "GET", "/v1/buyer/orders/"+other, capability, "", nil, nil); res.status == 200 {
				t.Fatalf("the capability read another owner's order %s", other)
			}
		}
	})

	t.Run("a replay of the placement returns the same link, now dead", func(t *testing.T) {
		code, out := a.place(key1, mtBody(sku, 1, homeKey, "bank_transfer", mtHome))
		if code != 200 || out["buyer_link"] != link1 {
			t.Fatalf("replay: %d %v", code, out)
		}
		if res, _ := e.mtRedeem(order1, linkToken1); mtRefusalBody(t, res) != "404/not_found" {
			t.Fatalf("a used link stays dead: %s", mtRefusalBody(t, res))
		}
	})

	t.Run("expiry: a link past its 7 days is the same refusal", func(t *testing.T) {
		order2, linkToken2, _, _ := place("two")
		mustExec(t, e.p.f.owner, `UPDATE checkout.order_links SET created_at=clock_timestamp()-interval '8 days',expires_at=clock_timestamp()-interval '2 days' WHERE order_id=$1`, order2)
		res, _ := e.mtRedeem(order2, linkToken2)
		if mtRefusalBody(t, res) != "404/not_found" {
			t.Fatalf("expired: %s", mtRefusalBody(t, res))
		}
		if n := e.count(`SELECT count(*) FROM checkout.order_links WHERE order_id=$1 AND redeemed_at IS NOT NULL`, order2); n != 0 {
			t.Fatal("an expired link must not be marked used")
		}
	})

	t.Run("concurrent exchanges: exactly one wins", func(t *testing.T) {
		order3, linkToken3, _, _ := place("three")
		var wg sync.WaitGroup
		statuses := make([]int, 4)
		for i := range statuses {
			wg.Add(1)
			go func() {
				defer wg.Done()
				res, _ := e.mtRedeem(order3, linkToken3)
				statuses[i] = res.status
			}()
		}
		wg.Wait()
		ok, refused := 0, 0
		for _, s := range statuses {
			switch s {
			case 200:
				ok++
			case 404, 429:
				refused++
			}
		}
		if ok != 1 || ok+refused != len(statuses) {
			t.Fatalf("statuses %v: want exactly one 200", statuses)
		}
		if n := e.count(`SELECT count(*) FROM buyer.capability_sessions c JOIN checkout.orders o ON o.owner_id=c.owner_id WHERE o.id=$1 AND c.view_order_id IS NULL`, order3); n != 2 {
			t.Fatalf("expected the order-time capability plus exactly one exchanged one, got %d sessions", n)
		}
	})

	t.Run("throttle: more than 5 attempts per order in the window is a 429", func(t *testing.T) {
		order4, _, _, _ := place("four")
		last := ""
		for i := 0; i < 7; i++ {
			res, _ := e.mtRedeem(order4, randomToken())
			last = mtRefusalBody(t, res)
		}
		if last != "429/rate_limited" {
			t.Fatalf("after 7 wrong attempts: %s", last)
		}
	})

	t.Run("F2 idempotent re-delivery: the same bearer re-delivers the same order within 10 minutes; any other bearer is refused", func(t *testing.T) {
		order5, linkToken5, _, _ := place("five")
		bearer := randomToken()
		ip := func(r *http.Request) { r.Header.Set("X-Commerce-Client-IP", "203.0.113.201") }
		first := e.bh.request(t, "POST", "/v1/buyer/orders/link", bearer, "", map[string]any{"order_id": order5, "token": linkToken5}, ip)
		if first.status != 200 {
			t.Fatalf("first: %d %s", first.status, first.body)
		}
		// A lost response is retried with the SAME bearer (the BFF re-derives it from the browser-bound proof): the same order re-delivers.
		second := e.bh.request(t, "POST", "/v1/buyer/orders/link", bearer, "", map[string]any{"order_id": order5, "token": linkToken5}, ip)
		if second.status != 200 {
			t.Fatalf("same-bearer replay: %d %s", second.status, second.body)
		}
		var re struct {
			OrderID string `json:"order_id"`
		}
		if err := json.Unmarshal(second.body, &re); err != nil || re.OrderID != order5 {
			t.Fatalf("re-delivery must name the same order: %q %v", re.OrderID, err)
		}
		// Exactly ONE capability session for that bearer: no new session, no second capability for a different browser.
		if n := e.count(`SELECT count(*) FROM buyer.capability_sessions WHERE token_hash=$1`, func() []byte { h := sha256.Sum256([]byte(bearer)); return h[:] }()); n != 1 {
			t.Fatalf("same-bearer replay must not mint a new session, got %d", n)
		}
		// A DIFFERENT bearer (a different browser-bound proof) is the identical refusal.
		third := e.bh.request(t, "POST", "/v1/buyer/orders/link", randomToken(), "", map[string]any{"order_id": order5, "token": linkToken5}, ip)
		if mtRefusalBody(t, third) != "404/not_found" {
			t.Fatalf("different bearer: %s", mtRefusalBody(t, third))
		}
	})

	t.Run("F2 regenerate: the merchant re-issues the link, the old one is dead and the new one works", func(t *testing.T) {
		order6, linkToken6, _, _ := place("six")
		key := t04Key("mtl-regen")
		body, _ := json.Marshal(map[string]any{"order_id": order6, "locale": "zh-TW"})
		w := a.call("POST", "/orders/manual/regenerate-link", key, "application/json", body)
		if w.Code != 201 {
			t.Fatalf("regenerate: %d %s", w.Code, w.Body.String())
		}
		var out map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		newLink, _ := out["buyer_link"].(string)
		newOrder, newToken := e.mtLinkParts(newLink, "zh-TW")
		if newOrder != order6 || newToken == linkToken6 {
			t.Fatalf("regenerated link must name the same order with a fresh token: %q %q", newOrder, newToken)
		}
		// The old link is invalidated even before any use (marked redeemed), and no longer exchanges.
		if n := e.count(`SELECT count(*) FROM checkout.order_links WHERE order_id=$1 AND redeemed_at IS NOT NULL`, order6); n != 1 {
			t.Fatalf("regenerate must invalidate the old link, got %d redeemed rows", n)
		}
		if res, _ := e.mtRedeem(order6, linkToken6); mtRefusalBody(t, res) != "404/not_found" {
			t.Fatalf("old link after regenerate: %s", mtRefusalBody(t, res))
		}
		// The new link works and marks it used.
		if first, _ := e.mtRedeem(order6, newToken); first.status != 200 {
			t.Fatalf("new link: %d %s", first.status, first.body)
		}
		if n := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='order.manual_link_regenerated'`, e.store()); n != 1 {
			t.Fatalf("manual-link-regenerated audit rows: %d", n)
		}
		// A replay of the SAME key is a no-op returning the SAME link (200, not a fresh link).
		w2 := a.call("POST", "/orders/manual/regenerate-link", key, "application/json", body)
		if w2.Code != 200 {
			t.Fatalf("regenerate replay: %d %s", w2.Code, w2.Body.String())
		}
		var out2 map[string]any
		_ = json.Unmarshal(w2.Body.Bytes(), &out2)
		if out2["buyer_link"] != newLink {
			t.Fatalf("regenerate replay must return the same link, got %v want %v", out2["buyer_link"], newLink)
		}
	})

	t.Run("F1 store-scoped IP throttle: one store's flood does not 429 another store behind the same IP", func(t *testing.T) {
		ip := sha256.Sum256([]byte("203.0.113.99"))
		contact := sha256.Sum256([]byte("nobody@example.test"))
		tokenHash := sha256.Sum256([]byte(randomToken()))
		storeB := randomUUID()
		lookup := func(store, ref string) error {
			var n int
			return e.p.a.issuer.QueryRow(ctx, `SELECT count(*) FROM checkout.guest_order_lookup($1::uuid,$2,$3,$4,$5,$6::bigint,$7)`,
				store, ref, "email", contact[:], tokenHash[:], int64(3600), ip[:]).Scan(&n)
		}
		// 10 hits is the per-store IP bucket limit; the order ref is varied so only the IP bucket accumulates.
		for i := 0; i < 10; i++ {
			if err := lookup(e.store(), fmt.Sprintf("%012x", i+1)); err != nil {
				t.Fatalf("hit %d: %v", i+1, err)
			}
		}
		if err := lookup(e.store(), fmt.Sprintf("%012x", 11)); sqlState(err) != "PT429" {
			t.Fatalf("the 11th same-store hit must 429 on the IP bucket, got %v", err)
		}
		// The same edge IP against a DIFFERENT store is untouched: its own IP bucket is empty.
		if err := lookup(storeB, fmt.Sprintf("%012x", 1)); err != nil {
			t.Fatalf("another store must not inherit the flood: %v", err)
		}
	})
}

func TestMerchantToolsOrderSource(t *testing.T) {
	e, a, _ := mtOrderEnv(t)
	sku := e.p.stock.skus[0].ID
	var homeKey string
	for _, o := range a.options() {
		if o.DeliveryKind == "home" {
			homeKey = o.OptionKey
		}
	}
	code, out := a.place(t04Key("mts-1"), mtBody(sku, 1, homeKey, "bank_transfer", mtHome))
	if code != 201 {
		t.Fatalf("place: %d %v", code, out)
	}
	manual := out["order_id"].(string)
	var storefront string
	if err := e.p.f.owner.QueryRow(context.Background(), `SELECT id::text FROM checkout.orders WHERE store_id=$1 AND source='storefront' ORDER BY created_at LIMIT 1`, e.store()).Scan(&storefront); err != nil {
		t.Fatalf("a storefront order exists in the harness: %v", err)
	}
	base := "/v1/admin/stores/" + e.store() + "/orders"
	st, list, raw := e.mcall(e.token(), "GET", base+"?limit=50", "", "")
	if st != 200 {
		t.Fatalf("list: %d %s", st, raw)
	}
	seen := map[string]string{}
	for _, it := range list["items"].([]any) {
		m := it.(map[string]any)
		seen[m["order_id"].(string)], _ = m["source"].(string)
	}
	if seen[manual] != "merchant_manual" || seen[storefront] != "storefront" {
		t.Fatalf("list sources: manual=%q storefront=%q (%v)", seen[manual], seen[storefront], seen)
	}
	for id, want := range map[string]string{manual: "merchant_manual", storefront: "storefront"} {
		st, detail, raw := e.mcall(e.token(), "GET", base+"/"+id, "", "")
		if st != 200 || detail["source"] != want {
			t.Fatalf("detail %s: %d source=%v %s", id, st, detail["source"], raw)
		}
	}
	// A reader without orders:read gets nothing (the source rides the orders:read projection).
	noOrders, _ := e.member("catalog:read")
	if st, _, _ := e.mcall(noOrders, "GET", base+"?limit=5", "", ""); st != 403 {
		t.Fatalf("list without orders:read: %d", st)
	}
	// Dashboard latest orders carry it too.
	d := e.mtDashboard(a)
	found := false
	for _, it := range d["latest_orders"].([]any) {
		if m := it.(map[string]any); m["order_id"] == manual && m["source"] == "merchant_manual" {
			found = true
		}
	}
	if !found {
		t.Fatalf("dashboard latest_orders must show the manual order with its source: %v", d["latest_orders"])
	}
}
