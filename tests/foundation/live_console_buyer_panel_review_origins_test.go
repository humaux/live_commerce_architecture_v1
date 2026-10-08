// Purpose: independently guard A13 price-neutral checkout origins, projection deduplication,
// latest-20 display bounds and full-set purchase ordinals without weakening the original gate.
// Depends on: bpEnv real HTTP router/REAL_PG fixtures; claims.order_origins, claims.live_price_uses,
// inbox.order_for_buyer, inbox.bundle_peers and checkout.orders; frozen LC-B3b A13 and I09.
// Used by: focused TestLiveConsoleBuyerPanelReviewOriginProjections acceptance runs.

package foundation_test

import (
	"testing"
	"time"
)

func TestLiveConsoleBuyerPanelReviewOriginProjections(t *testing.T) {
	e := bpNew(t)
	session := e.session("bp-review-origin")
	offer := e.offer(session, "ORIGIN", 100)
	e.exec(`UPDATE live.offers SET live_price_minor=NULL WHERE tenant_id=$1 AND store_id=$2 AND session_id=$3 AND id=$4`, e.tenant, e.store1, session, offer)
	bundle := e.bundle(session, "facebook", bpActorKey("bp-review-origin"), &e.owner1, nil, false)
	e.line(bundle, session, offer, 1)
	conv := e.conversation(e.store1, "page", "1234567890")
	e.linkPeer(bundle, conv, "page", "1234567890")

	// Twenty-two ordinary checkouts exist ONLY in order_origins. The remaining order
	// appears in all three projections and must still contribute exactly once.
	orders := make([]string, 23)
	for i := range orders {
		state := "CONFIRMED"
		if i == len(orders)-1 {
			state = "AWAITING_COLLECTION"
		}
		orders[i] = e.order(state, "MANUAL_UNASSIGNED", 100, time.Duration(i+3)*time.Hour)
		e.origin(bundle, offer, orders[i], session)
	}
	e.use(bundle, offer, orders[0], 1, 100)
	e.forBuyer(bundle, orders[0], "placed")
	unpaid := e.order("AWAITING_PAYMENT", "MANUAL_UNASSIGNED", 100, 2*time.Hour)
	e.origin(bundle, offer, unpaid, session)
	cancelled := e.order("CANCELLED", "CANCELLED", 100, time.Hour)
	e.origin(bundle, offer, cancelled, session)

	// Conversation scope spans another linked session, whereas bundle scope does not.
	siblingSession := e.session("bp-review-origin-sibling")
	siblingOffer := e.offer(siblingSession, "SIBLING", 100)
	e.exec(`UPDATE live.offers SET live_price_minor=NULL WHERE tenant_id=$1 AND store_id=$2 AND session_id=$3 AND id=$4`, e.tenant, e.store1, siblingSession, siblingOffer)
	sibling := e.bundle(siblingSession, "instagram", bpActorKey("bp-review-origin-sibling"), nil, nil, false)
	e.line(sibling, siblingSession, siblingOffer, 1)
	e.linkPeer(sibling, conv, "page", "1234567890")
	siblingOrder := e.order("CONFIRMED", "MANUAL_UNASSIGNED", 100, 30*time.Minute)
	e.origin(sibling, siblingOffer, siblingOrder, siblingSession)

	// Same buyer owner is deliberately insufficient evidence of a conversation link (I09).
	unlinked := e.bundle(session, "facebook", bpActorKey("bp-review-origin-unlinked"), &e.owner1, nil, false)
	e.line(unlinked, session, offer, 1)
	e.origin(unlinked, offer, e.order("CONFIRMED", "MANUAL_UNASSIGNED", 100, time.Minute), session)

	for _, tc := range []struct {
		name, query string
		wantIDs     []string
		ordinal     int
	}{
		{"conversation", "conversation_id=" + conv, append([]string{siblingOrder, cancelled, unpaid}, orders[:17]...), 24},
		{"bundle", "bundle_id=" + bundle, append([]string{cancelled, unpaid}, orders[:18]...), 23},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, out, raw, _ := e.get(e.token, "/inbox/buyer-panel?"+tc.query)
			if code != 200 {
				t.Fatalf("A13 status=%d body=%s", code, raw)
			}
			gotOrders := bpArr(out, "orders")
			if len(gotOrders) != 20 {
				t.Fatalf("orders=%d want latest 20; body=%s", len(gotOrders), raw)
			}
			seen := make(map[string]bool)
			for i, row := range gotOrders {
				id := bpStr(bpMap(row), "order_id")
				if id != tc.wantIDs[i] || seen[id] {
					t.Fatalf("orders[%d]=%s want %s without duplicates; body=%s", i, id, tc.wantIDs[i], raw)
				}
				seen[id] = true
			}
			if got := bpNum(out, "purchase_ordinal"); got != float64(tc.ordinal) {
				t.Errorf("purchase_ordinal=%v want %d (full scoped set, CONFIRMED/AWAITING_COLLECTION only)", got, tc.ordinal)
			}
			code, out, raw, _ = e.get(e.roToken, "/inbox/buyer-panel?"+tc.query)
			if code != 200 {
				t.Fatalf("read-only A13 status=%d body=%s", code, raw)
			}
			if _, exists := out["orders"]; exists {
				t.Errorf("orders key present without orders:read: %s", raw)
			}
			if got := bpNum(out, "purchase_ordinal"); got != float64(tc.ordinal) {
				t.Errorf("read-only purchase_ordinal=%v want %d", got, tc.ordinal)
			}
		})
	}
}
