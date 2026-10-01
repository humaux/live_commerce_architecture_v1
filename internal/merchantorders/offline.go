// offline.go is the merchant side of the bank_transfer payment mode (contracts/storefront-v2.md §C): the store's bank-transfer settings, the
// transfer detail of one order, and the three audited decisions the merchant takes on it: confirm, reject the buyer's submission, record an
// offline refund.
//
// Every call is one SQL definer of migration 0088 (checkout.read_bank_transfer_settings / set_bank_transfer_settings /
// read_bank_transfer_merchant / decide_bank_transfer) inside the caller's platform.WithScope transaction; Go validates shape, SQL owns
// every rule, permission and the idempotent receipt. A confirm is a merchant act only: there is no webhook, poll or bank feed, and the
// confirmed amount is the server order total, never a client value (I05).
//
// Non-goals: no PSP call (an offline refund moves no money here), no buyer route (internal/checkout serves the buyer), no notification send,
// no stock rule (decide_bank_transfer is the only writer of the confirm's ledger rows).

package merchantorders

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

// TransferError is a coded refusal of the 0088 definers (409/422). Code is the contract code the HTTP layer returns (internal/httperror table).
type TransferError struct {
	Status int
	Code   string
}

func (e *TransferError) Error() string { return e.Code }

var (
	transferCode = regexp.MustCompile(`^[a-z][a-z0-9_]{2,59}$`)
	transferKey  = regexp.MustCompile(`^[A-Za-z0-9_.:-]{8,128}$`)
	accountShape = regexp.MustCompile(`^([0-9][0-9 -]{3,31})?$`)
)

// mapTransferError turns the definers' SQLSTATEs into domain errors: PT400 invalid, PT409/PT422 a coded refusal, authority via mapError.
func mapTransferError(err error) error {
	var coded *TransferError
	if errors.As(err, &coded) {
		return err
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "PT400", "22023":
			return command.ErrInvalid
		case "PT409":
			if transferCode.MatchString(pg.Message) {
				return &TransferError{Status: http.StatusConflict, Code: pg.Message}
			}
			return command.ErrConflict
		case "PT422":
			if transferCode.MatchString(pg.Message) {
				return &TransferError{Status: http.StatusUnprocessableEntity, Code: pg.Message}
			}
			return command.ErrInvalid
		}
	}
	return mapError(err)
}

func strictDecode(raw []byte, into any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil {
		return ErrUnavailable
	}
	return nil
}

func digestOf(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, command.ErrInvalid
	}
	sum := sha256.Sum256(raw)
	return sum[:], nil
}

// ---- settings -------------------------------------------------------------------------------------

// TransferSettings is the merchant's bank-transfer configuration (GET/PUT /v1/admin/stores/{store_id}/bank-transfer-settings). No row reads as
// version 0, off, window 72.
type TransferSettings struct {
	Version       int64  `json:"version"`
	Enabled       bool   `json:"enabled"`
	AllowCVS      bool   `json:"allow_cvs"`
	BankName      string `json:"bank_name"`
	Branch        string `json:"branch"`
	AccountName   string `json:"account_name"`
	AccountNumber string `json:"account_number"`
	WindowHours   int    `json:"window_hours"`
}

// TransferSettingsInput is the exact PUT body (every key present).
type TransferSettingsInput struct {
	ExpectedVersion int64  `json:"expected_version"`
	Enabled         bool   `json:"enabled"`
	AllowCVS        bool   `json:"allow_cvs"`
	BankName        string `json:"bank_name"`
	Branch          string `json:"branch"`
	AccountName     string `json:"account_name"`
	AccountNumber   string `json:"account_number"`
	WindowHours     int    `json:"window_hours"`
}

func validText(s string, max int) bool {
	if !utf8.ValidString(s) || utf8.RuneCountInString(s) > max {
		return false
	}
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

func validTransferSettings(s TransferSettings) bool {
	return s.Version >= 0 && s.WindowHours >= 6 && s.WindowHours <= 168 && validText(s.BankName, 60) && validText(s.Branch, 60) &&
		validText(s.AccountName, 60) && accountShape.MatchString(s.AccountNumber) &&
		(!s.Enabled || (s.BankName != "" && s.AccountName != "" && s.AccountNumber != ""))
}

// ReadTransferSettings reads the store's bank-transfer settings (integration:read; the SQL definer re-authorizes).
func ReadTransferSettings(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string) (TransferSettings, error) {
	if tx == nil || !validAuthorityInput(scope, token) {
		return TransferSettings{}, command.ErrInvalid
	}
	hash := sha256.Sum256([]byte(token))
	var raw []byte
	// checkout.read_bank_transfer_settings (0088): integration:read, GUCs from resolve_access, fresh final fence.
	if err := tx.QueryRow(ctx, `SELECT checkout.read_bank_transfer_settings($1,$2::uuid)`, hash[:], scope.StoreID).Scan(&raw); err != nil {
		return TransferSettings{}, mapTransferError(err)
	}
	var out TransferSettings
	if err := strictDecode(raw, &out); err != nil || !validTransferSettings(out) {
		return TransferSettings{}, ErrUnavailable
	}
	return out, nil
}

// SetTransferSettings writes the settings with version CAS (0 inserts). 422 invalid_settings when a rule fails, 409 version_changed on a
// stale version. A change never touches orders already placed (each keeps its own bank-details snapshot).
func SetTransferSettings(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key string, in TransferSettingsInput) (TransferSettings, error) {
	if tx == nil || !validAuthorityInput(scope, token) || !transferKey.MatchString(key) || in.ExpectedVersion < 0 || in.ExpectedVersion >= 1<<62 {
		return TransferSettings{}, command.ErrInvalid
	}
	if !validTransferSettings(TransferSettings{Version: in.ExpectedVersion, Enabled: in.Enabled, AllowCVS: in.AllowCVS, BankName: in.BankName,
		Branch: in.Branch, AccountName: in.AccountName, AccountNumber: in.AccountNumber, WindowHours: in.WindowHours}) {
		return TransferSettings{}, &TransferError{Status: http.StatusUnprocessableEntity, Code: "invalid_settings"}
	}
	digest, err := digestOf(struct {
		Op string `json:"op"`
		TransferSettingsInput
	}{"checkout.bank_transfer_settings", in})
	if err != nil {
		return TransferSettings{}, err
	}
	hash := sha256.Sum256([]byte(token))
	var raw []byte
	// checkout.set_bank_transfer_settings (0088): integration:manage, advisory lock per store, idempotent receipt, audit row.
	if err = tx.QueryRow(ctx, `SELECT checkout.set_bank_transfer_settings($1,$2::uuid,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12::integer)`,
		hash[:], scope.StoreID, key, digest, in.ExpectedVersion, in.Enabled, in.AllowCVS, in.BankName, in.Branch, in.AccountName,
		in.AccountNumber, int32(in.WindowHours)).Scan(&raw); err != nil {
		return TransferSettings{}, mapTransferError(err)
	}
	var out TransferSettings
	if err = strictDecode(raw, &out); err != nil || !validTransferSettings(out) || out.Version < 1 {
		return TransferSettings{}, ErrUnavailable
	}
	return out, nil
}

// ---- one order's transfer ---------------------------------------------------------------------------

// TransferBank is the account snapshot of the order (merchants always see it).
type TransferBank struct {
	BankName      string `json:"bank_name"`
	Branch        string `json:"branch"`
	AccountName   string `json:"account_name"`
	AccountNumber string `json:"account_number"`
}

// TransferProof is the buyer's latest claim; the merchant compares it with their bank statement.
type TransferProof struct {
	Last5       string `json:"last5"`
	AmountMinor int64  `json:"amount_minor"`
	PaidAt      string `json:"paid_at"`
	SubmittedAt string `json:"submitted_at"`
	Count       int    `json:"count"`
}

// TransferDetail is GET .../orders/{order_id}/bank-transfer. State: AWAITING | SUBMITTED | REJECTED | CONFIRMED | EXPIRED | REFUNDED_OFFLINE.
// AmountMinor is the server order total the buyer must transfer (I05).
type TransferDetail struct {
	OrderID      string         `json:"order_id"`
	State        string         `json:"state"`
	WindowHours  int            `json:"window_hours"`
	DeadlineAt   string         `json:"deadline_at"`
	Currency     string         `json:"currency"`
	AmountMinor  int64          `json:"amount_minor"`
	Bank         *TransferBank  `json:"bank"`
	Proof        *TransferProof `json:"proof"`
	RejectReason *string        `json:"reject_reason"`
	ConfirmedAt  *string        `json:"confirmed_at"`
	RefundedAt   *string        `json:"refunded_at"`
}

var transferStates = map[string]bool{"AWAITING": true, "SUBMITTED": true, "REJECTED": true, "CONFIRMED": true, "EXPIRED": true, "REFUNDED_OFFLINE": true}

func validTransferDetail(v TransferDetail, orderID string) bool {
	if v.OrderID != orderID || !transferStates[v.State] || v.WindowHours < 6 || v.WindowHours > 168 || !currency(v.Currency) || !money(v.AmountMinor) ||
		v.Bank == nil {
		return false
	}
	if _, err := canonicalTime(v.DeadlineAt); err != nil {
		return false
	}
	for _, at := range []*string{v.ConfirmedAt, v.RefundedAt} {
		if at != nil {
			if _, err := canonicalTime(*at); err != nil {
				return false
			}
		}
	}
	if v.Proof != nil {
		if len(v.Proof.Last5) != 5 || v.Proof.Count < 1 || v.Proof.AmountMinor < 1 {
			return false
		}
		for _, at := range []string{v.Proof.PaidAt, v.Proof.SubmittedAt} {
			if _, err := canonicalTime(at); err != nil {
				return false
			}
		}
	}
	if v.State == "SUBMITTED" && (v.Proof == nil || v.RejectReason != nil) || (v.State == "REJECTED") != (v.RejectReason != nil) {
		return false
	}
	return (v.State == "CONFIRMED" || v.State == "REFUNDED_OFFLINE") == (v.ConfirmedAt != nil) && (v.State == "REFUNDED_OFFLINE") == (v.RefundedAt != nil)
}

// ReadTransfer reads one order's transfer (orders:read; a non-transfer or unknown order is command.ErrNotFound / platform.ErrScopeNotFound).
func ReadTransfer(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, orderID string) (TransferDetail, error) {
	if tx == nil || !validAuthorityInput(scope, token) || !command.ValidID(orderID) {
		return TransferDetail{}, command.ErrInvalid
	}
	hash := sha256.Sum256([]byte(token))
	var raw []byte
	// checkout.read_bank_transfer_merchant (0088): orders:read, fresh final fence.
	if err := tx.QueryRow(ctx, `SELECT checkout.read_bank_transfer_merchant($1,$2::uuid,$3::uuid)`, hash[:], scope.StoreID, orderID).Scan(&raw); err != nil {
		return TransferDetail{}, mapTransferError(err)
	}
	var out TransferDetail
	if err := strictDecode(raw, &out); err != nil || !validTransferDetail(out, orderID) {
		return TransferDetail{}, ErrUnavailable
	}
	return out, nil
}

// TransferDecisionResult is the 200 body of confirm / reject / refund-offline.
type TransferDecisionResult struct {
	OrderID         string `json:"order_id"`
	State           string `json:"state"`
	CommercialState string `json:"commercial_state"`
	ReleasedLines   int    `json:"released_lines"`
}

// RejectInput is the exact reject body: why the submission was rejected (1..200 characters, shown to the buyer).
type RejectInput struct {
	Reason string `json:"reason"`
}

// DecideTransfer runs one merchant decision: action "confirm" | "reject" | "refund_offline" (reason only for reject). payments:refund is
// required (owners hold it); the SQL definer re-authorizes, takes the order lock, writes the idempotent receipt and exactly one audit row.
// Replaying the same key and body returns the first answer; the same key with another body is 409 idempotency_conflict.
func DecideTransfer(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key, orderID, action string, reason *string) (TransferDecisionResult, error) {
	if tx == nil || !validAuthorityInput(scope, token) || !transferKey.MatchString(key) || !command.ValidID(orderID) ||
		(action != "confirm" && action != "reject" && action != "refund_offline") || (action == "reject") != (reason != nil) {
		return TransferDecisionResult{}, command.ErrInvalid
	}
	if reason != nil {
		trimmed := *reason
		if n := utf8.RuneCountInString(trimmed); n < 1 || n > 200 || !validText(trimmed, 200) || len(bytes.TrimSpace([]byte(trimmed))) == 0 {
			return TransferDecisionResult{}, &TransferError{Status: http.StatusUnprocessableEntity, Code: "invalid_reason"}
		}
	}
	digest, err := digestOf(struct {
		Op      string  `json:"op"`
		OrderID string  `json:"order_id"`
		Action  string  `json:"action"`
		Reason  *string `json:"reason"`
	}{"checkout.bank_transfer", orderID, action, reason})
	if err != nil {
		return TransferDecisionResult{}, err
	}
	hash := sha256.Sum256([]byte(token))
	var raw []byte
	// checkout.decide_bank_transfer (0088): the ONLY writer of a transfer confirmation (never auto-confirmed, never from a PSP signal).
	if err = tx.QueryRow(ctx, `SELECT checkout.decide_bank_transfer($1,$2::uuid,$3::uuid,$4,$5,$6,$7)`,
		hash[:], scope.StoreID, orderID, key, digest, action, reason).Scan(&raw); err != nil {
		return TransferDecisionResult{}, mapTransferError(err)
	}
	// Second fence with the original Go Scope (merchantorders.read pattern).
	if err = platform.RequirePermission(ctx, tx, scope, token, "payments:refund"); err != nil {
		return TransferDecisionResult{}, mapError(err)
	}
	var out TransferDecisionResult
	if err = strictDecode(raw, &out); err != nil || out.OrderID != orderID || out.ReleasedLines < 0 {
		return TransferDecisionResult{}, ErrUnavailable
	}
	want := map[string]string{"confirm": "CONFIRMED", "reject": "REJECTED", "refund_offline": "REFUNDED_OFFLINE"}[action]
	if out.State != want {
		return TransferDecisionResult{}, ErrUnavailable
	}
	return out, nil
}
