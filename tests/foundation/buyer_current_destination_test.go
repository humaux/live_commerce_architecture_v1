package foundation_test

import (
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	"livecommerce/internal/storefront"
)

// Recovery discovers a current owned selection, not the outcome of a lost
// command. Synthetic expired selections must remain readable for head CAS.
func TestBuyerHTTPCurrentDestinationRecovery(t *testing.T) {
	h := bhSetup(t)
	type current struct {
		Destination *storefront.Destination `json:"destination"`
	}
	read := func(token string) current {
		t.Helper()
		r := h.request(t, "GET", "/v1/buyer/destination", token, "", nil, nil)
		var keys map[string]json.RawMessage
		if json.Unmarshal(r.body, &keys) != nil || len(keys) != 1 || keys["destination"] == nil {
			t.Fatal("current destination must have exactly one nullable projection")
		}
		return bhRead[current](t, r, 200)
	}
	before := h.facts(t)
	destinationFacts := bdFacts(t, h.cqHarness)
	first := read(h.cap.Token).Destination
	if first == nil || first.ID != h.destination.ID || first.Version != 1 || first.RecipientName != h.destination.RecipientName {
		t.Fatal("owned selection was not discovered")
	}
	if !reflect.DeepEqual(before, h.facts(t)) {
		t.Fatal("read changed transactional facts")
	}
	if bdFacts(t, h.cqHarness) != destinationFacts {
		t.Fatal("read changed destination heads, snapshots, events or receipts")
	}
	issued := bhRead[struct {
		Token string `json:"token"`
	}](t, h.request(t, "POST", "/v1/buyer/session", "", "", struct{}{}, nil), 200)
	if read(issued.Token).Destination != nil {
		t.Fatal("another owner saw a destination")
	}
	// After expiry, hide neither the UUID nor head version and do not renew it.
	// One statement clock preserves the exact maximum TTL; two volatile clock
	// reads can differ by microseconds and violate the database constraint.
	mustExec(t, h.f.owner, `UPDATE storefront.destination_snapshots SET selected_at=statement_timestamp()-interval '31 minutes',expires_at=statement_timestamp()-interval '1 minute' WHERE id=$1`, first.ID)
	expired := read(h.cap.Token).Destination
	if expired == nil || expired.Version != 1 || !expired.ExpiresAt.Before(first.ExpiresAt) {
		t.Fatal("expired head must remain available for explicit replacement")
	}
	cart := bhRead[storefront.Cart](t, h.request(t, "GET", "/v1/buyer/cart", h.cap.Token, "", nil, nil), 200)
	in := bdHome(cart)
	in.RecipientName = "Replacement Synthetic Buyer"
	bhRead[map[string]any](t, h.request(t, "PUT", "/v1/buyer/destination", h.cap.Token, t04Key("late-old-head"), in, nil), 409)
	in.ExpectedVersion = expired.Version
	second := bhRead[storefront.Destination](t, h.request(t, "PUT", "/v1/buyer/destination", h.cap.Token, t04Key("replace-current-head"), in, nil), 200)
	if got := read(h.cap.Token).Destination; got == nil || got.ID != second.ID || got.Version != 2 {
		t.Fatal("current read did not advance to the new immutable selection")
	}
	historical := bhRead[storefront.Destination](t, h.request(t, "GET", "/v1/buyer/destinations/"+first.ID, h.cap.Token, "", nil, nil), 200)
	if historical.Version != 1 || historical.RecipientName != first.RecipientName {
		t.Fatal("historical destination was changed")
	}
	for _, path := range []string{"/v1/buyer/destination?owner_id=x", "/v1/buyer/destination?"} {
		if status := h.request(t, "GET", path, h.cap.Token, "", nil, nil).status; status != 403 {
			t.Fatalf("current read query admission status=%d for %s", status, path)
		}
	}
	if h.request(t, "GET", "/v1/buyer/destination", "", "", nil, nil).status != 401 {
		t.Fatal("current read allowed missing capability")
	}
	if h.request(t, "GET", "/v1/buyer/destination", h.cap.Token, "", nil, func(r *http.Request) {
		r.Header.Set("X-Commerce-Storefront-Origin", "https://not-published.example")
	}).status != 404 {
		t.Fatal("current read ignored published host authority")
	}
}

func TestBuyerHTTPBuyNowPurchaseCartQuery(t *testing.T) {
	h := bhSetup(t)
	const currentPath = "/v1/buyer/destination"
	direct := bhRead[storefront.Cart](t, h.request(t, "POST", "/v1/buyer/buy-now", h.cap.Token, t04Key("query-buy-now"),
		storefront.BuyNowInput{SKUID: h.stock.skus[0].ID, Quantity: 1}, nil), 200)
	query := "?buy_now_id=" + direct.ID
	in := bdHome(direct)
	in.BuyNowID = direct.ID
	destination := bhRead[storefront.Destination](t, h.request(t, "PUT", currentPath, h.cap.Token, t04Key("query-destination"), in, nil), 200)
	quote := bhRead[storefront.Quote](t, h.request(t, "POST", "/v1/buyer/quotes", h.cap.Token, t04Key("query-quote"), storefront.QuoteInput{
		BuyNowID: direct.ID, CartVersion: direct.Version, MarketID: h.market.ID, Country: "TW", Method: "delivery:" + h.delivery.Code,
	}, nil), 200)
	if destination.CartID != direct.ID || quote.CartID != direct.ID {
		t.Fatal("Buy now setup returned a different cart")
	}
	type current struct {
		Destination *storefront.Destination `json:"destination"`
	}
	readCurrent := func(path, token string, edit func(*http.Request)) *storefront.Destination {
		t.Helper()
		return bhRead[current](t, h.request(t, "GET", path, token, "", nil, edit), 200).Destination
	}
	if got := readCurrent(currentPath+query, h.cap.Token, nil); got == nil || got.ID != destination.ID {
		t.Fatal("exact selector did not find its own destination")
	}
	if got := readCurrent(currentPath, h.cap.Token, nil); got == nil || got.ID != h.destination.ID {
		t.Fatal("ordinary recovery changed after the Buy now selection")
	}
	other := mustIssue(t, h.cqHarness.service, h.f.storeA1)
	if readCurrent(currentPath+query, other.Token, nil) != nil {
		t.Fatal("Buy now destination escaped its buyer")
	}
	const otherOrigin = "https://buyer-other-store.example"
	bhPublish(t, h.bcHarness, otherOrigin, h.f.tenantB, h.f.storeB)
	otherStore := mustIssue(t, h.cqHarness.service, h.f.storeB)
	useOtherStore := func(r *http.Request) { r.Header.Set("X-Commerce-Storefront-Origin", otherOrigin) }
	if readCurrent(currentPath+query, otherStore.Token, useOtherStore) != nil {
		t.Fatal("Buy now destination escaped its store")
	}

	type detail struct {
		ID     string `json:"id"`
		CartID string `json:"cart_id"`
	}
	details := []struct{ path, id string }{
		{"/v1/buyer/destinations/" + destination.ID, destination.ID},
		{"/v1/buyer/quotes/" + quote.ID, quote.ID},
	}
	for _, item := range details {
		got := bhRead[detail](t, h.request(t, "GET", item.path+query, h.cap.Token, "", nil, nil), 200)
		if got.ID != item.id || got.CartID != direct.ID {
			t.Fatalf("GET %s returned the wrong Buy now receipt", item.path)
		}
		bhRead[map[string]any](t, h.request(t, "GET", item.path, h.cap.Token, "", nil, nil), 404)
		bhRead[map[string]any](t, h.request(t, "GET", item.path+query, other.Token, "", nil, nil), 404)
		bhRead[map[string]any](t, h.request(t, "GET", item.path+query, otherStore.Token, "", nil, useOtherStore), 404)
	}

	for _, path := range []string{currentPath, details[0].path, details[1].path} {
		for _, invalid := range []string{"?", "?owner_id=x", "?buy_now_id=", "?buy_now_id=invalid", query + "&buy_now_id=" + direct.ID, query + "&owner_id=x"} {
			bhRead[map[string]any](t, h.request(t, "GET", path+invalid, h.cap.Token, "", nil, nil), 403)
		}
	}
	bhRead[map[string]any](t, h.request(t, "GET", currentPath+"?owner_id=x", "", "", nil, nil), 403)
	bhRead[map[string]any](t, h.request(t, "PUT", currentPath+query, h.cap.Token, t04Key("query-not-write"), in, nil), 403)
	bhRead[map[string]any](t, h.request(t, "GET", "/v1/buyer/cart"+query, h.cap.Token, "", nil, nil), 403)
	bhRead[map[string]any](t, h.request(t, "GET", "/v1/buyer/buy-now/"+direct.ID+query, h.cap.Token, "", nil, nil), 403)
}
