package checkout

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"livecommerce/internal/buyer"
	"livecommerce/internal/command"
	"livecommerce/internal/pagination"
)

func TestOrdersCursorCanonicalAndOwnerBound(t *testing.T) {
	scope := buyer.Scope{TenantID: "11111111-1111-1111-1111-111111111111", StoreID: "22222222-2222-2222-2222-222222222222", OwnerID: "33333333-3333-3333-3333-333333333333"}
	order := OrderSummary{OrderID: "44444444-4444-4444-4444-444444444444", CreatedAt: time.Date(2026, 9, 1, 12, 0, 0, 123456000, time.UTC)}
	binding := ordersBinding(scope)
	encoded := encodeOrdersCursor(binding, order)
	got, err := decodeOrdersCursor(encoded, binding)
	if err != nil || got.OrderID != order.OrderID || !got.CreatedAt.Equal(order.CreatedAt) {
		t.Fatalf("cursor roundtrip: %+v %v", got, err)
	}
	raw, _ := base64.RawURLEncoding.DecodeString(encoded)
	for _, id := range []string{scope.TenantID, scope.StoreID, scope.OwnerID} {
		if strings.Contains(string(raw), id) {
			t.Fatal("cursor exposed raw authority")
		}
	}
	for _, field := range []string{"tenant", "store", "owner"} {
		changed := scope
		switch field {
		case "tenant":
			changed.TenantID = order.OrderID
		case "store":
			changed.StoreID = order.OrderID
		case "owner":
			changed.OwnerID = order.OrderID
		}
		if _, err := decodeOrdersCursor(encoded, ordersBinding(changed)); err != command.ErrInvalid {
			t.Fatalf("cursor crossed %s", field)
		}
	}
	scope.SessionID = order.OrderID
	if ordersBinding(scope) != binding {
		t.Fatal("history should follow owner across sessions")
	}
	for _, bad := range []string{
		"!", encoded + "=", strings.Repeat("a", 1025), "{}", string(raw) + " ",
		strings.Replace(string(raw), `"version":1`, `"version":2`, 1),
		strings.Replace(string(raw), `"version":1`, `"version":1,"version":1`, 1),
		strings.Replace(string(raw), `"version":1`, `"version":1,"extra":0`, 1),
		strings.Replace(string(raw), order.OrderID, "invalid", 1),
		strings.Replace(string(raw), "2026-09-01T12:00:00.123456Z", "0001-01-01T00:00:00Z", 1),
		strings.Replace(string(raw), "2026-09-01T12:00:00.123456Z", "2026-09-01T12:00:00.123456+00:00", 1),
		strings.Replace(string(raw), ".123456Z", ".1234567Z", 1),
		strings.Replace(string(raw), `"created_at":"2026-09-01T12:00:00.123456Z"`, `"created_at":null`, 1),
		encodeOptionsCursor(binding, Option{MarketID: scope.StoreID, Country: "TW", DeliveryCode: "home_delivery"}),
	} {
		if strings.HasPrefix(bad, "{") {
			bad = base64.RawURLEncoding.EncodeToString([]byte(bad))
		}
		if _, err := decodeOrdersCursor(bad, binding); err != command.ErrInvalid {
			t.Fatalf("malformed cursor accepted: %q", bad)
		}
	}
}

func TestOrdersInvalidServiceDoesNotPanic(t *testing.T) {
	var service *Service
	for _, request := range []pagination.Request{{}, {Limit: -1}, {Limit: 101}, {Cursor: strings.Repeat("a", 1025)}} {
		page, err := service.ListOrders(context.Background(), "", "", request)
		if err != command.ErrInvalid || page.Items == nil || len(page.Items) != 0 || page.NextCursor != "" {
			t.Fatal("invalid service/request did not fail closed")
		}
	}
}
