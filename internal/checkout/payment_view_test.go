package checkout

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"livecommerce/internal/command"
)

func TestPaymentViewBoundedShape(t *testing.T) {
	const order = "00000000-0000-4000-8000-000000000001"
	base := OrderPayment{OrderID: order, Currency: "TWD", TotalMinor: 100,
		CommercialState: "DRAFT", TestMode: true, PaymentState: "NOT_STARTED", HandoffState: "NONE",
		Methods: []PaymentMethodOption{{Code: "payuni_credit", Version: 1, NameHans: "卡", NameHant: "卡", NameEN: "Card"}}}
	if !validPaymentView(base, order) {
		t.Fatal("valid bounded view rejected")
	}
	for name, change := range map[string]func(*OrderPayment){
		"owner-order":      func(v *OrderPayment) { v.OrderID = "different" },
		"currency":         func(v *OrderPayment) { v.Currency = "twd" },
		"amount":           func(v *OrderPayment) { v.TotalMinor = -1 },
		"null-methods":     func(v *OrderPayment) { v.Methods = nil },
		"second-method":    func(v *OrderPayment) { v.Methods = append(v.Methods, v.Methods[0]) },
		"unknown-state":    func(v *OrderPayment) { v.PaymentState = "PAID_BY_REDIRECT" },
		"unknown-handoff":  func(v *OrderPayment) { v.HandoffState = "RETRYABLE" },
		"missing-deadline": func(v *OrderPayment) { v.HandoffState = "PREPARED" },
		"pending-option":   func(v *OrderPayment) { v.PaymentState = "PENDING" },
		"cancelled-option": func(v *OrderPayment) { v.CommercialState = "CANCELLED" },
	} {
		t.Run(name, func(t *testing.T) {
			v := base
			change(&v)
			if validPaymentView(v, order) {
				t.Fatal("invalid view accepted")
			}
		})
	}
	base.Methods = []PaymentMethodOption{}
	base.PaymentState, base.HandoffState = "CAPTURED", "UNAVAILABLE"
	if !validPaymentView(base, order) {
		t.Fatal("historical different-profile view must remain readable")
	}
	now := time.Now().UTC()
	base.HandoffState, base.HandoffExpiresAt = "ISSUED", &now
	if !validPaymentView(base, order) {
		t.Fatal("issued captured view rejected")
	}
}

func TestPaymentViewNotFoundMapping(t *testing.T) {
	if !errors.Is(safeError(context.Background(), &pgconn.PgError{Code: "PT404"}), command.ErrNotFound) {
		t.Fatal("private missing/foreign order must not become database unavailable")
	}
}
