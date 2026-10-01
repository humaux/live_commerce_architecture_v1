package checkout

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"livecommerce/internal/buyer"
	"livecommerce/internal/command"
	"livecommerce/internal/fulfillment"
)

func TestOptionsCursorCanonicalAndScoped(t *testing.T) {
	scope := buyer.Scope{TenantID: "11111111-1111-1111-1111-111111111111", StoreID: "22222222-2222-2222-2222-222222222222"}
	in := OptionsRequest{MarketID: "33333333-3333-3333-3333-333333333333", Country: "TW"}
	option := Option{MarketID: in.MarketID, Country: "TW", DeliveryCode: "home_delivery"}
	binding := optionsBinding(scope, in)
	if len(binding) != 64 || strings.Contains(binding, scope.TenantID) || strings.Contains(binding, scope.StoreID) {
		t.Fatal("binding must hide raw scope")
	}
	encoded := encodeOptionsCursor(binding, option)
	got, err := decodeOptionsCursor(encoded, binding)
	if err != nil || got.MarketID != option.MarketID || got.Country != option.Country || got.DeliveryCode != option.DeliveryCode {
		t.Fatal("valid cursor failed")
	}
	for _, changed := range []OptionsRequest{{MarketID: in.MarketID}, {Country: "TW"}, {MarketID: in.MarketID, Country: "US"}} {
		if _, err := decodeOptionsCursor(encoded, optionsBinding(scope, changed)); err != command.ErrInvalid {
			t.Fatal("cursor crossed filter")
		}
	}
	scope.StoreID = "44444444-4444-4444-4444-444444444444"
	if _, err := decodeOptionsCursor(encoded, optionsBinding(scope, in)); err != command.ErrInvalid {
		t.Fatal("cursor crossed store")
	}
	bad := []string{
		"!", encoded + "=", strings.Repeat("a", 1025),
		`{"version":1,"binding":"` + binding + `","market_id":"` + in.MarketID + `","country":"TW","delivery_code":"home_delivery","country":"TW"}`,
		`{"version":1,"binding":"` + binding + `","market_id":"` + in.MarketID + `","country":null,"delivery_code":"home_delivery"}`,
		`{"version":1,"binding":"` + binding + `","market_id":"` + in.MarketID + `","country":"TW","delivery_code":"home_delivery","extra":1}`,
		`{"binding":"` + binding + `","version":1,"market_id":"` + in.MarketID + `","country":"TW","delivery_code":"home_delivery"}`,
		`{"version":1,"binding":"` + binding + `","market_id":"` + in.MarketID + `","country":"tw","delivery_code":"home_delivery"}`,
		`{"version":1,"binding":"` + binding + `","market_id":"` + in.MarketID + `","country":"TW","delivery_code":"BAD"}`,
	}
	for _, value := range bad {
		if strings.HasPrefix(value, "{") {
			value = base64.RawURLEncoding.EncodeToString([]byte(value))
		}
		if _, err := decodeOptionsCursor(value, binding); err != command.ErrInvalid {
			t.Fatalf("accepted malformed cursor %q: %v", value, err)
		}
	}
}

// OP1: card is offered only while the process can take card payment; a row with no mode at all is dropped.
func TestPaymentModesOP1(t *testing.T) {
	for _, c := range []struct {
		card, pap, bank bool
		want            string
	}{{true, false, false, "card"}, {true, true, false, "card,pay_at_pickup"}, {false, true, false, "pay_at_pickup"}, {false, false, false, ""},
		// storefront-v2 §C: bank_transfer joins last, and alone it keeps a row offered when card cannot be taken.
		{true, true, true, "card,pay_at_pickup,bank_transfer"}, {false, false, true, "bank_transfer"}} {
		if got := strings.Join(paymentModes(c.card, c.pap, c.bank), ","); got != c.want {
			t.Fatalf("paymentModes(%v,%v,%v)=%q want %q", c.card, c.pap, c.bank, got, c.want)
		}
	}
}

// OP1: home rows are card-only, so without card payment decorateCVS offers none of them (no DB needed: no CVS row, no offer query).
func TestDecorateHomeRowsOP1(t *testing.T) {
	rows := []Option{{DeliveryKind: "home", DeliveryCode: "home_delivery"}}
	on, err := (&Service{}).decorateCVS(nil, nil, nil, "", rows, transferOffer{})
	if err != nil || len(on) != 1 || on[0].PaymentModes != nil {
		t.Fatalf("card on: %v %v", on, err)
	}
	off, err := (&Service{noCard: true}).decorateCVS(nil, nil, nil, "", rows, transferOffer{})
	if err != nil || len(off) != 0 {
		t.Fatalf("card off must drop home rows: %v %v", off, err)
	}
	// storefront-v2 §C: with bank transfer on, a home row lists its modes (and the window); without card it stays offered as bank-only.
	bt := transferOffer{enabled: true, windowHours: 72}
	both, err := (&Service{}).decorateCVS(nil, nil, nil, "", rows, bt)
	if err != nil || len(both) != 1 || strings.Join(both[0].PaymentModes, ",") != "card,bank_transfer" || both[0].TransferWindowHours != 72 {
		t.Fatalf("home card+bank: %+v %v", both, err)
	}
	only, err := (&Service{noCard: true}).decorateCVS(nil, nil, nil, "", rows, bt)
	if err != nil || len(only) != 1 || strings.Join(only[0].PaymentModes, ",") != "bank_transfer" {
		t.Fatalf("home bank only: %+v %v", only, err)
	}
}

// OP1: the shared Begin refusal is a typed 422 for card only; pay_at_pickup and a card-capable process pass.
func TestCardRefusalOP1(t *testing.T) {
	var refusal *fulfillment.CVSError
	if err := cardRefusal("card", true); !errors.As(err, &refusal) || refusal.Status != 422 || refusal.Code != "card_unavailable" {
		t.Fatalf("card with no PSP must be refused: %v", err)
	}
	if cardRefusal("card", false) != nil || cardRefusal("pay_at_pickup", true) != nil {
		t.Fatal("only card without a PSP is refused")
	}
}
