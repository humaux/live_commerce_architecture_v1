package foundation_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/buyer"
	"livecommerce/internal/checkout"
	"livecommerce/internal/pagination"
	"livecommerce/internal/platform"
	"livecommerce/internal/storefront"
)

// Real owner-scoped reads and sequential checkout use the existing isolated
// HTTP/PG harness. All buyer/contact data is synthetic.
func TestBuyerHTTPOrderHistorySequentialPurchases(t *testing.T) {
	h := bhSetup(t)
	ctx := context.Background()
	read := func(path string) pagination.Page[checkout.OrderSummary] {
		t.Helper()
		return bhRead[pagination.Page[checkout.OrderSummary]](t, h.request(t, "GET", path, h.cap.Token, "", nil, nil), 200)
	}
	empty := read("/v1/buyer/orders")
	if empty.Items == nil || len(empty.Items) != 0 || empty.NextCursor != "" {
		t.Fatal("empty history must be a non-null empty page")
	}
	before := h.facts(t)
	firstKey, firstInput := t04Key("history-first"), h.input
	first, err := h.begin(firstKey)
	if err != nil {
		t.Fatal(err)
	}
	firstOrder, err := h.service.Get(ctx, h.cap.Token, h.f.storeA1, first.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	cleared, err := h.cart(t04Key("history-continue"), storefront.CartInput{ExpectedVersion: h.quote.CartVersion, Items: []storefront.Item{}})
	if err != nil || cleared.ID != h.quote.CartID || cleared.Version != h.quote.CartVersion+1 {
		t.Fatalf("explicit next cart failed: %v", err)
	}
	cart, err := h.cart(t04Key("history-second-cart"), storefront.CartInput{ExpectedVersion: cleared.Version, Items: []storefront.Item{{SKUID: h.stock.skus[0].ID, Quantity: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	destinationInput := bdHome(cart)
	destinationInput.ExpectedVersion = h.destination.Version
	destination, err := bdSet(h.cqHarness, t04Key("history-destination"), destinationInput)
	if err != nil {
		t.Fatal(err)
	}
	quote, err := cqBuyer(h.a.runtime, h.cap, func(ctx context.Context, tx pgx.Tx, scope buyer.Scope) (storefront.Quote, error) {
		return storefront.CreateQuote(ctx, tx, scope, t04Key("history-quote"), storefront.QuoteInput{CartVersion: cart.Version, MarketID: h.market.ID, Country: "TW", Method: "delivery:" + h.delivery.Code})
	})
	if err != nil {
		t.Fatal(err)
	}
	h.input.QuoteID, h.input.DestinationID, h.input.CartVersion = quote.ID, destination.ID, cart.Version
	second, err := h.begin(t04Key("history-second"))
	if err != nil || first.OrderID == second.OrderID {
		t.Fatalf("second independent order: %v", err)
	}
	for i, count := range h.facts(t) {
		if count-before[i] != 2 {
			t.Fatalf("order/hold/job/receipt delta[%d]=%d want 2", i, count-before[i])
		}
	}
	afterPurchases := h.facts(t)
	replay, err := h.service.Begin(ctx, h.cap.Token, h.f.storeA1, firstKey, firstInput)
	if err != nil || replay != first {
		t.Fatalf("old checkout receipt changed: %v", err)
	}
	firstAgain, err := h.service.Get(ctx, h.cap.Token, h.f.storeA1, first.OrderID)
	if err != nil || !reflect.DeepEqual(firstAgain, firstOrder) {
		t.Fatal("old order snapshot changed")
	}
	page := read("/v1/buyer/orders?limit=1")
	if len(page.Items) != 1 || page.Items[0].OrderID != second.OrderID || page.Items[0].CartID != cart.ID || page.Items[0].CartVersion != cart.Version || page.NextCursor == "" {
		t.Fatalf("newest first page: %+v", page)
	}
	last := read("/v1/buyer/orders?limit=1&cursor=" + url.QueryEscape(page.NextCursor))
	if len(last.Items) != 1 || last.Items[0].OrderID != first.OrderID || last.Items[0].CartVersion != firstInput.CartVersion || last.NextCursor != "" {
		t.Fatalf("older exact page: %+v", last)
	}
	full := read("/v1/buyer/orders?limit=100")
	if len(full.Items) != 2 || full.NextCursor != "" || full.Items[0].Currency != quote.Currency || full.Items[0].TotalMinor != quote.Amount.TotalMinor {
		t.Fatal("full page or snapshot amounts changed")
	}
	detail := bhRead[struct {
		CartID      string `json:"cart_id"`
		CartVersion int64  `json:"cart_version"`
	}](t, h.request(t, "GET", "/v1/buyer/orders/"+first.OrderID, h.cap.Token, "", nil, nil), 200)
	if detail.CartID != firstOrder.Snapshot.Quote.CartID || detail.CartVersion != firstInput.CartVersion {
		t.Fatal("owned detail did not retain original quote cart provenance")
	}
	raw := h.request(t, "GET", "/v1/buyer/orders", h.cap.Token, "", nil, nil).body
	for _, secret := range []string{h.cap.Token, h.cap.Scope.OwnerID, h.cap.Scope.SessionID, h.destination.RecipientName, h.destination.Phone, h.destination.HomeAddress.Line1, "destination", "recipient", "snapshot", "job_id", "reservation_id", "generation"} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("history leaked %q", secret)
		}
	}
	if h.facts(t) != afterPurchases {
		t.Fatal("history/read/replay mutated order facts")
	}
	// A timestamp tie must use id DESC, never skip or duplicate a row.
	mustExec(t, h.f.owner, `UPDATE checkout.orders SET created_at=(SELECT max(created_at) FROM checkout.orders WHERE owner_id=$1) WHERE owner_id=$1`, h.cap.Scope.OwnerID)
	want := []string{first.OrderID, second.OrderID}
	sort.Sort(sort.Reverse(sort.StringSlice(want)))
	tied := read("/v1/buyer/orders?limit=1")
	tiedLast := read("/v1/buyer/orders?limit=1&cursor=" + url.QueryEscape(tied.NextCursor))
	if len(tied.Items) != 1 || len(tiedLast.Items) != 1 || tied.Items[0].OrderID != want[0] || tiedLast.Items[0].OrderID != want[1] || tiedLast.NextCursor != "" {
		t.Fatal("equal timestamp pagination skipped/duplicated order")
	}
	other := mustIssue(t, h.cqHarness.service, h.f.storeA1)
	otherPage := bhRead[pagination.Page[checkout.OrderSummary]](t, h.request(t, "GET", "/v1/buyer/orders", other.Token, "", nil, nil), 200)
	if len(otherPage.Items) != 0 {
		t.Fatal("history crossed owner")
	}
	bhError(t, h.request(t, "GET", "/v1/buyer/orders?cursor="+url.QueryEscape(page.NextCursor), other.Token, "", nil, nil), 422, "invalid_request")
	secondOrigin := "https://history-second.example"
	bhPublish(t, h.bcHarness, secondOrigin, h.f.tenantA, h.f.storeA2)
	foreign := mustIssue(t, h.cqHarness.service, h.f.storeA2)
	for _, token := range []string{h.cap.Token, foreign.Token} {
		status, code := 422, "invalid_request"
		if token == h.cap.Token {
			status, code = 401, "unauthorized"
		}
		bhError(t, h.request(t, "GET", "/v1/buyer/orders?cursor="+url.QueryEscape(page.NextCursor), token, "", nil, func(r *http.Request) { r.Header.Set("X-Commerce-Storefront-Origin", secondOrigin) }), status, code)
	}
	for _, query := range []string{"limit=0", "limit=101", "limit=1&limit=2", "cursor=bad", "cursor=", "cursor=a&cursor=b", "owner_id=" + h.cap.Scope.OwnerID, "country=TW", "limit=1;cursor=x", "limit=1&", "limit=%"} {
		bhError(t, h.request(t, "GET", "/v1/buyer/orders?"+query, h.cap.Token, "", nil, nil), 422, "invalid_request")
	}
	bhError(t, h.request(t, "GET", "/v1/buyer/orders", h.cap.Token, "", map[string]string{"x": "y"}, nil), 422, "invalid_request")
	bhError(t, h.request(t, "POST", "/v1/buyer/orders", h.cap.Token, "", nil, nil), 405, "method_not_allowed")
	bhError(t, h.request(t, "GET", "/v1/buyer/orders", h.cap.Token, "", nil, func(r *http.Request) { r.Header.Del("X-Commerce-Buyer-BFF-Key") }), 401, "unauthorized")
	mustExec(t, h.f.owner, `UPDATE control.storefront_publications SET published=false,version=version+1 WHERE store_id=$1`, h.f.storeA1)
	bhError(t, h.request(t, "GET", "/v1/buyer/orders", h.cap.Token, "", nil, nil), 404, "not_found")
	mustExec(t, h.f.owner, `UPDATE control.storefront_publications SET published=true,version=version+1 WHERE store_id=$1`, h.f.storeA1)
	if err := h.cqHarness.service.Revoke(ctx, h.cap.Token, h.f.storeA1); err != nil {
		t.Fatal(err)
	}
	bhError(t, h.request(t, "GET", "/v1/buyer/orders", h.cap.Token, "", nil, nil), 401, "unauthorized")
	mustExec(t, h.f.owner, `UPDATE buyer.capability_sessions SET created_at=clock_timestamp()-interval '2 seconds',expires_at=clock_timestamp()-interval '1 second' WHERE id=$1`, other.Scope.SessionID)
	bhError(t, h.request(t, "GET", "/v1/buyer/orders", other.Token, "", nil, nil), 401, "unauthorized")
	if !json.Valid(raw) || h.facts(t) != afterPurchases {
		t.Fatal("history transport changed durable order facts")
	}
}

func TestBuyerHTTPOrderHistoryFinalCapabilityClock(t *testing.T) {
	b := bcSetup(t)
	if _, err := b.begin(t04Key("history-expiry-order")); err != nil {
		t.Fatal(err)
	}
	name := "history-wait-" + t04Tag()
	pool, err := platform.OpenCheckoutPool(context.Background(), withApplicationName(t, b.poolURL, name))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	service := bcService(t, pool)
	holder, err := b.f.owner.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback(context.Background())
	if _, err := holder.Exec(context.Background(), `LOCK TABLE checkout.orders IN ACCESS EXCLUSIVE MODE`); err != nil {
		t.Fatal(err)
	}
	var expiry time.Time
	if err = b.f.owner.QueryRow(context.Background(), `UPDATE buyer.capability_sessions SET expires_at=clock_timestamp()+interval '700 milliseconds' WHERE id=$1 RETURNING expires_at`, b.cap.Scope.SessionID).Scan(&expiry); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		page, err := service.ListOrders(context.Background(), b.cap.Token, b.f.storeA1, pagination.Request{})
		if len(page.Items) != 0 || page.NextCursor != "" {
			err = errors.New("expired list returned data")
		}
		done <- err
	}()
	waitForDatabaseLock(t, b.f.owner, name)
	var valid bool
	if err = b.f.owner.QueryRow(context.Background(), `SELECT clock_timestamp()<$1`, expiry).Scan(&valid); err != nil || !valid {
		t.Fatalf("test missed valid-before-wait window: %v", err)
	}
	mustExec(t, b.f.owner, `SELECT pg_sleep(GREATEST(0,extract(epoch FROM $1::timestamptz-clock_timestamp()))+0.02)`, expiry)
	if err = holder.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err = waitError(t, done); !errors.Is(err, buyer.ErrUnauthorized) {
		t.Fatalf("final capability clock accepted expired list: %v", err)
	}
}
