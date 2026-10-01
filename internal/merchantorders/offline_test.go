package merchantorders

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

const (
	offOrder = "11111111-1111-4111-8111-111111111111"
	offKey   = "key-12345678"
)

func offScope() platform.Scope {
	return platform.Scope{TenantID: "22222222-2222-4222-8222-222222222222", StoreID: "33333333-3333-4333-8333-333333333333",
		PrincipalID: "44444444-4444-4444-8444-444444444444", Revision: 1}
}

// storefront-v2 §C: a bank_transfer summary never has a payment attempt, fact, refund or work item; it waits AWAITING_TRANSFER with
// nothing shipped, is CONFIRMED only by the merchant act, and only a transfer order can wait for a transfer.
func TestValidSummaryBankTransfer(t *testing.T) {
	base := func() Summary {
		return Summary{OrderID: offOrder, CreatedAt: "2026-10-01T00:00:00.000000Z", UpdatedAt: "2026-10-01T00:00:00.000000Z", Currency: "TWD",
			TotalMinor: 90000, CommercialState: "AWAITING_TRANSFER", FulfillmentState: "MANUAL_UNASSIGNED", PaymentState: "NOT_STARTED",
			WorkState: "NONE", PaymentMode: "bank_transfer"}
	}
	if !validSummary(base()) {
		t.Fatal("awaiting transfer summary rejected")
	}
	confirmed := base()
	confirmed.CommercialState = "CONFIRMED"
	shipped := confirmed
	shipped.FulfillmentState = "MERCHANT_SHIPPED"
	cancelled := base()
	cancelled.CommercialState, cancelled.FulfillmentState = "CANCELLED", "CANCELLED"
	for name, v := range map[string]Summary{"confirmed": confirmed, "shipped": shipped, "expired": cancelled} {
		if !validSummary(v) {
			t.Errorf("%s transfer summary rejected", name)
		}
	}
	for name, edit := range map[string]func(*Summary){
		"collection state on a transfer": func(v *Summary) { c := "PENDING"; v.CollectionState = &c },
		"card payment state":             func(v *Summary) { v.PaymentState = "CAPTURED" },
		"ready work item":                func(v *Summary) { v.WorkState = "READY" },
		"refunded":                       func(v *Summary) { v.RefundedMinor = 1 },
		"test mode":                      func(v *Summary) { v.TestMode = true },
		"shipped while awaiting":         func(v *Summary) { v.FulfillmentState = "MERCHANT_SHIPPED" },
		"allocation failed":              func(v *Summary) { v.FulfillmentState = "PAID_ALLOCATION_FAILED"; v.CommercialState = "CONFIRMED" },
	} {
		v := base()
		edit(&v)
		if validSummary(v) {
			t.Errorf("%s accepted", name)
		}
	}
	card := base()
	card.PaymentMode = "card"
	if validSummary(card) {
		t.Error("a card order cannot be AWAITING_TRANSFER")
	}
	// The unshipped list includes a CONFIRMED transfer order (work NONE) exactly like a pay_at_pickup one.
	if !matchesState("unshipped", confirmed) || matchesState("unshipped", base()) {
		t.Error("unshipped filter for transfer orders")
	}
	if !validState("AWAITING_TRANSFER") || !matchesState("AWAITING_TRANSFER", base()) || matchesState("AWAITING_TRANSFER", confirmed) {
		t.Error("AWAITING_TRANSFER filter")
	}
}

// noTx is a non-nil pgx.Tx whose methods panic: a refusal that returns means the check ran before any SQL.
type noTx struct{ pgx.Tx }

func TestDecideTransferRefusesBeforeSQL(t *testing.T) {
	ok, ws, ctrl, long := "amount does not match", "   ", "bad\nreason", strings.Repeat("x", 201)
	token := strings.Repeat("t", 40)
	for name, c := range map[string]struct {
		key, order, action string
		reason             *string
		wantCode           string // "" = command.ErrInvalid, else a coded 422
	}{
		"bad key":               {"short", offOrder, "confirm", nil, ""},
		"bad order":             {offKey, "nope", "confirm", nil, ""},
		"unknown action":        {offKey, offOrder, "cancel", nil, ""},
		"confirm with reason":   {offKey, offOrder, "confirm", &ok, ""},
		"refund with reason":    {offKey, offOrder, "refund_offline", &ok, ""},
		"reject without reason": {offKey, offOrder, "reject", nil, ""},
		"blank reason":          {offKey, offOrder, "reject", &ws, "invalid_reason"},
		"control in reason":     {offKey, offOrder, "reject", &ctrl, "invalid_reason"},
		"reason too long":       {offKey, offOrder, "reject", &long, "invalid_reason"},
	} {
		_, err := DecideTransfer(context.Background(), noTx{}, offScope(), token, c.key, c.order, c.action, c.reason)
		var coded *TransferError
		switch {
		case c.wantCode == "" && !errors.Is(err, command.ErrInvalid):
			t.Errorf("%s: %v", name, err)
		case c.wantCode != "" && (!errors.As(err, &coded) || coded.Code != c.wantCode || coded.Status != 422):
			t.Errorf("%s: %v", name, err)
		}
	}
	// A nil transaction is refused first, whatever the arguments.
	if _, err := DecideTransfer(context.Background(), nil, offScope(), token, offKey, offOrder, "confirm", nil); !errors.Is(err, command.ErrInvalid) {
		t.Errorf("nil tx: %v", err)
	}
}

func TestMapTransferError(t *testing.T) {
	for _, c := range []struct {
		code, msg string
		status    int
	}{{"PT409", "already_confirmed", 409}, {"PT409", "transfer_window_closed", 409}, {"PT422", "not_bank_transfer", 422}, {"PT422", "invalid_settings", 422}} {
		var coded *TransferError
		if err := mapTransferError(&pgconn.PgError{Code: c.code, Message: c.msg}); !errors.As(err, &coded) || coded.Status != c.status || coded.Code != c.msg {
			t.Errorf("%s %s: %v", c.code, c.msg, err)
		}
	}
	if err := mapTransferError(&pgconn.PgError{Code: "PT409", Message: "Customer Alice at 1 Main St"}); !errors.Is(err, command.ErrConflict) {
		t.Errorf("uncoded 409 must stay a plain conflict: %v", err)
	}
	if err := mapTransferError(&pgconn.PgError{Code: "PT403"}); !errors.Is(err, platform.ErrForbidden) {
		t.Errorf("authority: %v", err)
	}
	if err := mapTransferError(&pgconn.PgError{Code: "XX000", Message: "boom"}); !errors.Is(err, ErrUnavailable) {
		t.Errorf("unknown: %v", err)
	}
}

func TestValidTransferSettings(t *testing.T) {
	good := TransferSettings{Version: 1, Enabled: true, BankName: "Taiwan Bank", Branch: "Taipei", AccountName: "Shop Ltd", AccountNumber: "123-456-7890", WindowHours: 72}
	if !validTransferSettings(good) {
		t.Fatal("good settings rejected")
	}
	for name, edit := range map[string]func(*TransferSettings){
		"window too short":  func(s *TransferSettings) { s.WindowHours = 5 },
		"window too long":   func(s *TransferSettings) { s.WindowHours = 169 },
		"enabled no number": func(s *TransferSettings) { s.AccountNumber = "" },
		"enabled no name":   func(s *TransferSettings) { s.AccountName = "" },
		"letters in number": func(s *TransferSettings) { s.AccountNumber = "12AB5678" },
		"short number":      func(s *TransferSettings) { s.AccountNumber = "123" },
		"control in bank":   func(s *TransferSettings) { s.BankName = "a\nb" },
		"long bank":         func(s *TransferSettings) { s.BankName = strings.Repeat("b", 61) },
	} {
		s := good
		edit(&s)
		if validTransferSettings(s) {
			t.Errorf("%s accepted", name)
		}
	}
	off := TransferSettings{WindowHours: 72}
	if !validTransferSettings(off) {
		t.Fatal("the disabled default must be valid")
	}
}

func TestValidTransferDetail(t *testing.T) {
	at := "2026-10-01T00:00:00.000000Z"
	base := func() TransferDetail {
		return TransferDetail{OrderID: offOrder, State: "AWAITING", WindowHours: 72, DeadlineAt: at, Currency: "TWD", AmountMinor: 90000,
			Bank: &TransferBank{BankName: "B", AccountNumber: "123456"}}
	}
	if !validTransferDetail(base(), offOrder) {
		t.Fatal("awaiting detail rejected")
	}
	confirmed := base()
	confirmed.State, confirmed.ConfirmedAt = "CONFIRMED", &at
	if !validTransferDetail(confirmed, offOrder) {
		t.Fatal("confirmed detail rejected")
	}
	refunded := confirmed
	refunded.State, refunded.RefundedAt = "REFUNDED_OFFLINE", &at
	if !validTransferDetail(refunded, offOrder) {
		t.Fatal("refunded detail rejected")
	}
	for name, edit := range map[string]func(*TransferDetail){
		"merchant always sees bank": func(v *TransferDetail) { v.Bank = nil },
		"confirmed without time":    func(v *TransferDetail) { v.State = "CONFIRMED" },
		"bad deadline":              func(v *TransferDetail) { v.DeadlineAt = "tomorrow" },
		"unknown state":             func(v *TransferDetail) { v.State = "PAID" },
		"reject reason on awaiting": func(v *TransferDetail) { r := "x"; v.RejectReason = &r },
	} {
		v := base()
		edit(&v)
		if validTransferDetail(v, offOrder) {
			t.Errorf("%s accepted", name)
		}
	}
}
