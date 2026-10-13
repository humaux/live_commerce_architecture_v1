// Purpose: REAL_PG A15 preserves the newest order email only for the explicitly linked, authorized customer.
// Depends on: lbuNew, real buyer Quote/Begin, A14 customer-link HTTP, customers/order projections and narrow fulfillment.order_prefill_email.
// Used by: foundation CI TestLiveConsoleOrderForBuyerPrefillEmail; owner requires CI-only execution.
// Invariants: I01/I09; no identity guessing, no cross-store email or optional customer prefill without both read permissions, no new money write on GET.
package foundation_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/buyer"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/inbox"
	"livecommerce/internal/platform"
	"livecommerce/internal/storefront"
)

func oipeInbox(t *testing.T) *inbox.Service {
	t.Helper()
	keys, err := inbox.LoadKeyring(func(name string) string {
		switch name {
		case "COMMERCE_META_PAYLOAD_ACTIVE_KEY_ID":
			return "oipe"
		case "COMMERCE_META_PAYLOAD_KEYS_JSON":
			return `{"keys":[{"id":"oipe","key_base64":"` + base64.StdEncoding.EncodeToString(randomBytes(32)) + `"}]}`
		}
		return ""
	})
	if err != nil {
		t.Fatal("prefill fixture keyring construction failed")
	}
	svc, err := inbox.NewService(keys)
	if err != nil {
		t.Fatal("prefill fixture inbox construction failed")
	}
	return svc
}

func TestLiveConsoleOrderForBuyerPrefillEmail(t *testing.T) {
	e := lbuNew(t)
	e.handler = httpapi.NewHandler(e.p.f.runtime, httpapi.Options{CVS: e.cvs, ManualOrders: e.manual, ForBuyer: e.fb, Inbox: oipeInbox(t)})
	b := e.newBuyer(storefront.Item{SKUID: e.money, Quantity: 1})
	older, err := e.cofPlaceHome(b, "older.synthetic@example.test")
	if err != nil {
		t.Fatal("older real buyer order fixture failed")
	}
	// Use the same private buyer through the ordinary cart/destination/Quote/Begin seams for a second purchase.
	cleared, err := b.h.cart(t04Key("oipe-clear"), storefront.CartInput{ExpectedVersion: b.cartVersion(), Items: []storefront.Item{}})
	if err != nil {
		t.Fatal("explicit next purchase cart failed")
	}
	cart, err := b.h.cart(t04Key("oipe-cart"), storefront.CartInput{ExpectedVersion: cleared.Version, Items: []storefront.Item{{SKUID: e.money, Quantity: 1}}})
	if err != nil {
		t.Fatal("second real cart fixture failed")
	}
	destinationInput := bdHome(cart)
	destinationInput.ExpectedVersion = b.h.destination.Version
	destination, err := bdSet(b.h.cqHarness, t04Key("oipe-dest"), destinationInput)
	if err != nil {
		t.Fatal("second real destination fixture failed")
	}
	b.h.input.CartVersion, b.h.input.DestinationID = cart.Version, destination.ID
	quote, err := b.quote(b.h.delivery.Code)
	if err != nil {
		t.Fatal("second real Quote fixture failed")
	}
	b.h.input.QuoteID, b.h.quote, b.h.destination = quote.ID, quote, destination
	latest, err := e.cofPlaceHome(b, "latest.synthetic@example.test")
	if err != nil || latest.OrderID == older.OrderID {
		t.Fatal("latest real buyer order fixture failed")
	}
	if e.count(`SELECT count(*) FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2 AND owner_id=$3 AND id IN ($4,$5)`, e.tenant(), e.store(), b.cap.Scope.OwnerID, older.OrderID, latest.OrderID) != 2 {
		t.Fatal("contact history does not belong to the same original buyer")
	}
	conversation := lcConversation(t, e.p.f, e.tenant(), e.store(), "page")
	raw, _ := json.Marshal(map[string]any{"customer_id": b.cap.Scope.OwnerID, "expected_version": 0})
	linked := e.admin(e.token()).call("POST", "/inbox/conversations/"+conversation+"/customer-link", t04Key("oipe-link"), "application/json", raw)
	if linked.Code != 200 {
		t.Fatalf("real explicit A14 fixture: status=%d", linked.Code)
	}
	orders, held, receipts := e.count(`SELECT count(*) FROM checkout.orders WHERE store_id=$1`, e.store()), e.reserved(), e.count(`SELECT count(*) FROM ops.command_results WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store())
	status, out := e.get(e.token(), "?conversation_id="+conversation)
	customer, _ := out["customer"].(map[string]any)
	if status != 200 || customer == nil || customer["email"] != "latest.synthetic@example.test" || customer["name"] == "" || customer["phone"] == "" || out["last_delivery"] == nil {
		t.Fatalf("latest explicitly linked authorized contact: status=%d email=%v has_customer=%v has_delivery=%v", status, customer["email"], customer != nil, out["last_delivery"] != nil)
	}
	noCustomers := e.member("inbox:read")
	status, out = e.get(noCustomers, "?conversation_id="+conversation)
	if status != 200 || out["customer"] != nil || out["last_delivery"] != nil {
		t.Fatalf("optional customer prefill without customers:read: status=%d has_customer=%v", status, out["customer"] != nil)
	}
	unlinked := lcConversation(t, e.p.f, e.tenant(), e.store(), "page")
	if status, out = e.get(e.token(), "?conversation_id="+unlinked); status != 200 || out["customer"] != nil || out["last_delivery"] != nil {
		t.Fatal("unlinked conversation inferred a customer contact")
	}
	// The narrow definer itself enforces both read permissions and tenant/store, independently of caller projections.
	readEmail := func(token, order string) (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		var email string
		err := platform.WithScope(ctx, e.p.f.runtime, token, e.store(), "orders:read", func(tx pgx.Tx, scope platform.Scope) error {
			hash := sha256.Sum256([]byte(token))
			return tx.QueryRow(ctx, `SELECT fulfillment.order_prefill_email($1,$2::uuid,$3::uuid)`, hash[:], scope.StoreID, order).Scan(&email)
		})
		return email, err
	}
	if email, err := readEmail(noCustomers, latest.OrderID); sqlState(err) != "PT403" || email != "" {
		t.Fatal("email definer leaked a contact without customers:read")
	}
	foreign := lbuNew(t)
	foreignBuyer := foreign.newBuyer(storefront.Item{SKUID: foreign.money, Quantity: 1})
	foreignOrder, err := foreign.cofPlaceHome(foreignBuyer, "foreign.synthetic@example.test")
	if err != nil {
		t.Fatal("foreign real buyer order fixture failed")
	}
	if email, err := readEmail(e.token(), foreignOrder.OrderID); sqlState(err) != "PT404" || email != "" {
		t.Fatal("email definer leaked a foreign store contact")
	}
	// Positive sentinel after all negative controls: the authorized latest value is still available and unchanged.
	status, out = e.get(e.token(), "?conversation_id="+conversation)
	customer, _ = out["customer"].(map[string]any)
	if status != 200 || customer["email"] != "latest.synthetic@example.test" || e.count(`SELECT count(*) FROM checkout.orders WHERE store_id=$1`, e.store()) != orders || e.reserved() != held || e.count(`SELECT count(*) FROM ops.command_results WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store()) != receipts || e.plan.planned() != 0 {
		t.Fatal("optional prefill lost its authorized contact or changed money/stock/receipts/DM")
	}
}

// Keep the existing typed buyer scope in the fixture contract visible to dependency gates.
var _ buyer.Scope
