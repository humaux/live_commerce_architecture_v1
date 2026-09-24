package checkout

import (
	"encoding/base64"
	"strings"
	"testing"

	"livecommerce/internal/buyer"
	"livecommerce/internal/command"
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
