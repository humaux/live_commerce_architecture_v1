package foundation_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/buyer"
	"livecommerce/internal/checkout"
	"livecommerce/internal/command"
	"livecommerce/internal/storefront"
)

func TestBuyerCheckoutBuyNowIsolation(t *testing.T) {
	b := bcSetup(t)
	ctx := context.Background()
	regular, err := cqBuyer(b.a.runtime, b.cap, storefront.GetCart)
	if err != nil {
		t.Fatal(err)
	}
	sku := b.stock.skus[1].ID
	create := func(key string, qty int64) (storefront.Cart, error) {
		return cqBuyer(b.a.runtime, b.cap, func(ctx context.Context, tx pgx.Tx, scope buyer.Scope) (storefront.Cart, error) {
			return storefront.CreateBuyNow(ctx, tx, scope, key, storefront.BuyNowInput{SKUID: sku, Quantity: qty})
		})
	}
	key := t04Key("buy-now-selection")
	direct, err := create(key, 2)
	if err != nil || direct.ID == regular.ID || direct.Version != 1 || !reflect.DeepEqual(direct.Items, []storefront.Item{{SKUID: sku, Quantity: 2}}) {
		t.Fatalf("direct selection: %+v %v", direct, err)
	}
	if again, err := create(key, 2); err != nil || !reflect.DeepEqual(again, direct) {
		t.Fatalf("creation replay: %+v %v", again, err)
	}
	if _, err := create(key, 3); !errors.Is(err, command.ErrConflict) {
		t.Fatalf("creation key changed selection: %v", err)
	}
	get := func(cap buyer.Capability, id string) (storefront.Cart, error) {
		return cqBuyer(b.a.runtime, cap, func(ctx context.Context, tx pgx.Tx, s buyer.Scope) (storefront.Cart, error) {
			return storefront.GetBuyNow(ctx, tx, s, id)
		})
	}
	for _, denied := range []struct {
		cap buyer.Capability
		id  string
	}{
		{b.cap, regular.ID},
		{mustIssue(t, b.cqHarness.service, b.f.storeA1), direct.ID},
		{mustIssue(t, b.cqHarness.service, b.f.storeB), direct.ID},
	} {
		if _, err := get(denied.cap, denied.id); !errors.Is(err, command.ErrNotFound) {
			t.Fatalf("wrong use, owner or store read: %v", err)
		}
	}
	if found, err := cqBuyer(b.a.runtime, b.cap, storefront.GetCart); err != nil || !reflect.DeepEqual(found, regular) {
		t.Fatalf("entry changed regular cart: %+v %v", found, err)
	}
	quote := func(c storefront.Cart) storefront.Quote {
		t.Helper()
		q, err := cqBuyer(b.a.runtime, b.cap, func(ctx context.Context, tx pgx.Tx, s buyer.Scope) (storefront.Quote, error) {
			return storefront.CreateQuote(ctx, tx, s, t04Key("buy-now-quote"), storefront.QuoteInput{
				BuyNowID: c.ID, CartVersion: c.Version, MarketID: b.market.ID, Country: "TW", Method: "delivery:" + b.delivery.Code,
			})
		})
		if err != nil || q.CartID != c.ID || len(q.Lines) != 1 || q.Lines[0].SKUID != c.Items[0].SKUID || q.Lines[0].Quantity != c.Items[0].Quantity {
			t.Fatalf("direct quote: %+v %v", q, err)
		}
		return q
	}
	inputFor := func(c storefront.Cart, q storefront.Quote) checkout.Input {
		t.Helper()
		in := bdHome(c)
		in.BuyNowID = c.ID
		dest, err := bdSet(b.cqHarness, t04Key("buy-now-dest"), in)
		if err != nil || dest.CartID != c.ID {
			t.Fatalf("direct destination: %s %v", dest.CartID, err)
		}
		return checkout.Input{BuyNowID: c.ID, QuoteID: q.ID, DestinationID: dest.ID, CartVersion: c.Version, ServiceVersion: 1, AllocationVersion: 1}
	}
	begin := func(key string, in checkout.Input) (checkout.Result, error) {
		return b.service.Begin(ctx, b.cap.Token, b.f.storeA1, key, in)
	}
	q := quote(direct)
	in := inputFor(direct, q)
	ordinary := in
	ordinary.BuyNowID = ""
	mixed := in
	mixed.DestinationID = b.destination.ID
	for _, rejected := range []checkout.Input{ordinary, mixed} {
		if _, err := begin(t04Key("buy-now-wrong-source"), rejected); !errors.Is(err, command.ErrConflict) {
			t.Fatalf("mismatched checkout source: %v", err)
		}
	}
	before := b.facts(t)
	mustExec(t, b.f.owner, `UPDATE catalog.skus SET price_minor=price_minor+50,version=version+1 WHERE id=$1`, sku)
	if _, err := begin(t04Key("buy-now-old-price"), in); !errors.Is(err, command.ErrConflict) || b.facts(t) != before {
		t.Fatalf("stale price placed an order or changed facts: %v", err)
	}
	newQuote := quote(direct)
	if newQuote.Amount.TotalMinor <= q.Amount.TotalMinor {
		t.Fatalf("fresh authoritative price was not applied: %d -> %d", q.Amount.TotalMinor, newQuote.Amount.TotalMinor)
	}
	in.QuoteID = newQuote.ID
	oversized, err := create(t04Key("buy-now-stock"), 99)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := begin(t04Key("buy-now-no-stock"), inputFor(oversized, quote(oversized))); !errors.Is(err, command.ErrInsufficient) || b.facts(t) != before {
		t.Fatalf("insufficient stock placed an order or changed facts: %v", err)
	}

	start := make(chan struct{})
	edited := make(chan error, 1)
	go func() {
		<-start
		cart, err := b.cart(t04Key("buy-now-shared-edit"), storefront.CartInput{ExpectedVersion: regular.Version,
			Items: []storefront.Item{{SKUID: regular.Items[0].SKUID, Quantity: 3}}})
		if err == nil && (cart.ID != regular.ID || cart.Version != regular.Version+1 || cart.Items[0].Quantity != 3) {
			err = errors.New("normal cart CAS wrote the wrong cart")
		}
		edited <- err
	}()
	orderKey := t04Key("buy-now-place")
	close(start)
	result, err := begin(orderKey, in)
	editErr := <-edited
	if err != nil || editErr != nil {
		t.Fatalf("concurrent regular cart edit and direct checkout: %v / %v", err, editErr)
	}
	if replay, err := begin(orderKey, in); err != nil || !reflect.DeepEqual(replay, result) {
		t.Fatalf("order replay: %+v %v", replay, err)
	}
	if _, err := begin(t04Key("buy-now-duplicate"), in); !errors.Is(err, command.ErrConflict) {
		t.Fatalf("new key double ordered a direct selection: %v", err)
	}
	if got, err := get(b.cap, direct.ID); err != nil || !reflect.DeepEqual(got, direct) {
		t.Fatalf("placed selection changed: %+v %v", got, err)
	}
	if got, err := b.service.Get(ctx, b.cap.Token, b.f.storeA1, result.OrderID); err != nil || !reflect.DeepEqual(got.Snapshot.Quote, newQuote) {
		t.Fatalf("saved order does not match the direct quote: %v", err)
	}
}
