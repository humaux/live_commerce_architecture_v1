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
	first := read(h.cap.Token).Destination
	if first == nil || first.ID != h.destination.ID || first.Version != 1 || first.RecipientName != h.destination.RecipientName {
		t.Fatal("owned selection was not discovered")
	}
	if !reflect.DeepEqual(before, h.facts(t)) {
		t.Fatal("read changed transactional facts")
	}
	issued := bhRead[struct {
		Token string `json:"token"`
	}](t, h.request(t, "POST", "/v1/buyer/session", "", "", struct{}{}, nil), 200)
	if read(issued.Token).Destination != nil {
		t.Fatal("another owner saw a destination")
	}
	// After expiry, hide neither the UUID nor head version and do not renew it.
	mustExec(t, h.f.owner, `UPDATE storefront.destination_snapshots SET selected_at=clock_timestamp()-interval '31 minutes',expires_at=clock_timestamp()-interval '1 minute' WHERE id=$1`, first.ID)
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
