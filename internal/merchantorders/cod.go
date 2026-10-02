// cod.go is the merchant side of the cash_on_delivery payment mode (home-cod R5, contracts/payment-methods-v1.md, migration 0107):
// the store's cash-on-delivery settings (enable switch, per-order cap, optional whole-TWD surcharge, carrier label). The collection
// itself is the shared state machine of the pay_at_pickup mode (internal/fulfillment CVS.RecordCollection / Release), so there is no
// COD-specific decision here.
//
// Every call is one SQL definer of migration 0107 (payments.read_cash_on_delivery_settings / set_cash_on_delivery_settings) inside the
// caller's platform.WithScope transaction; Go validates shape, SQL owns every rule, permission and the idempotent receipt. The carrier
// label is a manual-fulfilment hint (black_cat / hsinchu), never a carrier API, and no amount here comes from the client (I05).
//
// Non-goals: no carrier API call, no PSP call, no buyer route (internal/checkout serves the buyer), no money movement (the COD amount is
// collected by the carrier on delivery and reported through identity.read_finance_summary).

package merchantorders

import (
	"context"
	"crypto/sha256"
	"net/http"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

// CodSettings is the merchant's cash-on-delivery configuration (GET/PUT /v1/admin/stores/{store_id}/cash-on-delivery-settings). No row
// reads as version 0, off, cap 20000, surcharge 0, carrier black_cat.
type CodSettings struct {
	Version      int64  `json:"version"`
	Enabled      bool   `json:"enabled"`
	MaxTWD       int    `json:"max_twd"`
	SurchargeTWD int    `json:"surcharge_twd"`
	Carrier      string `json:"carrier"`
}

// CodSettingsInput is the exact PUT body (every key present).
type CodSettingsInput struct {
	ExpectedVersion int64  `json:"expected_version"`
	Enabled         bool   `json:"enabled"`
	MaxTWD          int    `json:"max_twd"`
	SurchargeTWD    int    `json:"surcharge_twd"`
	Carrier         string `json:"carrier"`
}

func validCodSettings(s CodSettings) bool {
	return s.Version >= 0 && s.MaxTWD >= 1 && s.MaxTWD <= 20000 && s.SurchargeTWD >= 0 && s.SurchargeTWD <= 1000 &&
		(s.Carrier == "black_cat" || s.Carrier == "hsinchu")
}

// ReadCodSettings reads the store's cash-on-delivery settings (integration:read; the SQL definer re-authorizes).
func ReadCodSettings(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string) (CodSettings, error) {
	if tx == nil || !validAuthorityInput(scope, token) {
		return CodSettings{}, command.ErrInvalid
	}
	hash := sha256.Sum256([]byte(token))
	var raw []byte
	// payments.read_cash_on_delivery_settings (0107): integration:read, GUCs from resolve_access, fresh final fence.
	if err := tx.QueryRow(ctx, `SELECT payments.read_cash_on_delivery_settings($1,$2::uuid)`, hash[:], scope.StoreID).Scan(&raw); err != nil {
		return CodSettings{}, mapTransferError(err)
	}
	var out CodSettings
	if err := strictDecode(raw, &out); err != nil || !validCodSettings(out) {
		return CodSettings{}, ErrUnavailable
	}
	return out, nil
}

// SetCodSettings writes the settings with version CAS (0 inserts). 422 invalid_settings when a rule fails, 409 version_changed on a
// stale version. A change never touches orders already placed (each keeps its own surcharge snapshot).
func SetCodSettings(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key string, in CodSettingsInput) (CodSettings, error) {
	if tx == nil || !validAuthorityInput(scope, token) || !transferKey.MatchString(key) || in.ExpectedVersion < 0 || in.ExpectedVersion >= 1<<62 {
		return CodSettings{}, command.ErrInvalid
	}
	if !validCodSettings(CodSettings{Version: in.ExpectedVersion, Enabled: in.Enabled, MaxTWD: in.MaxTWD, SurchargeTWD: in.SurchargeTWD, Carrier: in.Carrier}) {
		return CodSettings{}, &TransferError{Status: http.StatusUnprocessableEntity, Code: "invalid_settings"}
	}
	digest, err := digestOf(struct {
		Op string `json:"op"`
		CodSettingsInput
	}{"checkout.cash_on_delivery_settings", in})
	if err != nil {
		return CodSettings{}, err
	}
	hash := sha256.Sum256([]byte(token))
	var raw []byte
	// payments.set_cash_on_delivery_settings (0107): integration:manage, advisory lock per store, idempotent receipt, audit row.
	if err = tx.QueryRow(ctx, `SELECT payments.set_cash_on_delivery_settings($1,$2::uuid,$3,$4,$5,$6,$7,$8,$9)`,
		hash[:], scope.StoreID, key, digest, in.ExpectedVersion, in.Enabled, int32(in.MaxTWD), int32(in.SurchargeTWD), in.Carrier).Scan(&raw); err != nil {
		return CodSettings{}, mapTransferError(err)
	}
	var out CodSettings
	if err = strictDecode(raw, &out); err != nil || !validCodSettings(out) || out.Version < 1 {
		return CodSettings{}, ErrUnavailable
	}
	return out, nil
}
