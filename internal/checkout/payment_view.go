package checkout

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/buyer"
	"livecommerce/internal/command"
)

type PaymentMethodOption struct {
	Code     string `json:"code"`
	Version  int64  `json:"version"`
	NameHans string `json:"name_hans"`
	NameHant string `json:"name_hant"`
	NameEN   string `json:"name_en"`
}

// OrderPayment is already buyer-safe. The SQL projection deliberately excludes
// attempt/account IDs, credentials, stored form bytes and provider references.
type OrderPayment struct {
	OrderID          string                `json:"order_id"`
	Currency         string                `json:"currency"`
	TotalMinor       int64                 `json:"total_minor"`
	CommercialState  string                `json:"commercial_state"`
	TestMode         bool                  `json:"test_mode"`
	PaymentState     string                `json:"payment_state"`
	HandoffState     string                `json:"handoff_state"`
	HandoffExpiresAt *time.Time            `json:"handoff_expires_at"`
	Methods          []PaymentMethodOption `json:"methods"`
}

// PaymentView is an informational snapshot. BeginHosted and TakeHosted retain
// admission authority; calling this method never prepares or releases a form.
func (s *HostedPaymentStarter) PaymentView(ctx context.Context, token, storeID, orderID string) (OrderPayment, error) {
	if ctx == nil || s == nil || s.starter.pool == nil || !validPaymentProfile(s.starter.profile) || !command.ValidID(orderID) {
		return OrderPayment{}, command.ErrInvalid
	}
	tokenHash := sha256.Sum256([]byte(token))
	var out OrderPayment
	err := buyer.WithScope(ctx, s.starter.pool, token, storeID, func(callCtx context.Context, tx pgx.Tx, scope buyer.Scope) error {
		var body []byte
		if err := tx.QueryRow(callCtx, `SELECT checkout.hosted_payment_view($1,$2::uuid,$3::uuid,$4,$5)`,
			tokenHash[:], storeID, orderID, s.starter.profile, s.configDigest[:]).Scan(&body); err != nil {
			return err
		}
		if len(body) == 0 || len(body) > 64<<10 {
			return command.ErrConflict
		}
		decoder := json.NewDecoder(bytes.NewReader(body))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&out); err != nil || !validPaymentView(out, orderID) {
			return command.ErrConflict
		}
		if out.HandoffExpiresAt != nil {
			utc := out.HandoffExpiresAt.UTC()
			out.HandoffExpiresAt = &utc
		}
		return checkCapability(callCtx, tx, tokenHash[:], storeID, scope)
	})
	if err != nil {
		return OrderPayment{}, safeError(ctx, err)
	}
	return out, nil
}

func validPaymentView(out OrderPayment, orderID string) bool {
	if out.OrderID != orderID || len(out.Currency) != 3 || out.TotalMinor < 0 || out.TotalMinor > 1000000000000 ||
		out.Methods == nil || len(out.Methods) > 1 {
		return false
	}
	for _, ch := range out.Currency {
		if ch < 'A' || ch > 'Z' {
			return false
		}
	}
	switch out.CommercialState {
	case "DRAFT", "AWAITING_PAYMENT", "CONFIRMED", "CANCELLED":
	default:
		return false
	}
	switch out.PaymentState {
	case "NOT_STARTED", "PENDING", "AUTHORIZED", "CAPTURED", "REVIEW_REQUIRED":
	default:
		return false
	}
	switch out.HandoffState {
	case "NONE":
		if out.HandoffExpiresAt != nil {
			return false
		}
	case "UNAVAILABLE": // Historical profile may differ even without a page.
	case "PREPARED", "ISSUED", "EXPIRED":
		if out.HandoffExpiresAt == nil || out.HandoffExpiresAt.IsZero() {
			return false
		}
	default:
		return false
	}
	if len(out.Methods) > 0 && (out.PaymentState != "NOT_STARTED" || out.CommercialState != "DRAFT" || out.HandoffState != "NONE") {
		return false
	}
	for _, method := range out.Methods {
		if method.Code != "payuni_credit" || method.Version < 1 || method.NameHans == "" || method.NameHant == "" || method.NameEN == "" {
			return false
		}
	}
	return true
}
