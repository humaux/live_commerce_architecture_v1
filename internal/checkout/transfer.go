// transfer.go is the buyer side of the bank_transfer payment mode (contracts/storefront-v2.md §C): the order page's transfer view
// (GET /v1/buyer/orders/{id}/bank-transfer) and the buyer's proof submission (PUT .../bank-transfer/proof).
//
// Both calls go through SQL definers on the checkout pool (checkout.read_bank_transfer_buyer, checkout.submit_transfer_proof, migration 0088):
// Go validates shape, the database owns every rule (owner scope, window, state, idempotency). The proof is a claim the merchant reads, never
// money: nothing here confirms an order, moves stock or trusts the claimed amount (I05); only the merchant's confirm (internal/merchantorders,
// payments.decide_bank_transfer) does.
//
// Non-goals: no merchant action, no payment attempt, no notification send, no bank detail outside the buyer's own order response.

package checkout

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/buyer"
	"livecommerce/internal/command"
)

// TransferBank is the merchant's account snapshot taken at placement; null once the order EXPIRED.
type TransferBank struct {
	BankName      string `json:"bank_name"`
	Branch        string `json:"branch"`
	AccountName   string `json:"account_name"`
	AccountNumber string `json:"account_number"`
}

// TransferProof is the buyer's latest claim (editable until the merchant confirms).
type TransferProof struct {
	Last5       string    `json:"last5"`
	AmountMinor int64     `json:"amount_minor"`
	PaidAt      time.Time `json:"paid_at"`
	SubmittedAt time.Time `json:"submitted_at"`
	Count       int       `json:"count"`
}

// TransferView is GET /v1/buyer/orders/{id}/bank-transfer. State: AWAITING | SUBMITTED | REJECTED | CONFIRMED | EXPIRED | REFUNDED_OFFLINE.
// AmountMinor/Currency are the server order total the buyer must transfer (I05), DeadlineAt the end of the window.
type TransferView struct {
	OrderID      string         `json:"order_id"`
	State        string         `json:"state"`
	WindowHours  int            `json:"window_hours"`
	DeadlineAt   time.Time      `json:"deadline_at"`
	Currency     string         `json:"currency"`
	AmountMinor  int64          `json:"amount_minor"`
	Bank         *TransferBank  `json:"bank"`
	Proof        *TransferProof `json:"proof"`
	RejectReason *string        `json:"reject_reason"`
	ConfirmedAt  *time.Time     `json:"confirmed_at"`
	RefundedAt   *time.Time     `json:"refunded_at"`
}

// ProofInput is the exact PUT body: the last five digits of the sending account, the amount the buyer says they sent and when.
type ProofInput struct {
	Last5       string    `json:"last5"`
	AmountMinor int64     `json:"amount_minor"`
	PaidAt      time.Time `json:"paid_at"`
}

// ProofResult is the PUT answer (SQL-built, strict).
type ProofResult struct {
	OrderID     string    `json:"order_id"`
	State       string    `json:"state"`
	ProofCount  int       `json:"proof_count"`
	SubmittedAt time.Time `json:"submitted_at"`
}

var (
	last5Pattern      = regexp.MustCompile(`^[0-9]{5}$`)
	transferStates    = map[string]bool{"AWAITING": true, "SUBMITTED": true, "REJECTED": true, "CONFIRMED": true, "EXPIRED": true, "REFUNDED_OFFLINE": true}
	maxProofAmount    = int64(1_000_000_000_000)
	transferStrictDec = func(raw []byte, into any) error {
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		return dec.Decode(into)
	}
)

func validProof(in ProofInput) bool {
	return last5Pattern.MatchString(in.Last5) && in.AmountMinor >= 1 && in.AmountMinor <= maxProofAmount && !in.PaidAt.IsZero()
}

// validTransferView re-verifies the SQL projection: a malformed one is drift, not data.
func validTransferView(v TransferView, orderID string) bool {
	if v.OrderID != orderID || !transferStates[v.State] || v.WindowHours < 6 || v.WindowHours > 168 || v.DeadlineAt.IsZero() ||
		len(v.Currency) != 3 || v.AmountMinor < 0 || v.AmountMinor > maxProofAmount {
		return false
	}
	// Bank details are present exactly while the order is not EXPIRED (the buyer must never lose or keep them wrongly).
	if (v.Bank == nil) != (v.State == "EXPIRED") {
		return false
	}
	if v.Proof != nil && (!last5Pattern.MatchString(v.Proof.Last5) || v.Proof.Count < 1 || v.Proof.AmountMinor < 1) {
		return false
	}
	if v.State == "SUBMITTED" && (v.Proof == nil || v.RejectReason != nil) {
		return false
	}
	if (v.State == "REJECTED") != (v.RejectReason != nil) {
		return false
	}
	return (v.State == "CONFIRMED" || v.State == "REFUNDED_OFFLINE") == (v.ConfirmedAt != nil) && (v.State == "REFUNDED_OFFLINE") == (v.RefundedAt != nil)
}

// TransferView reads the buyer's own transfer order; another owner's id is command.ErrNotFound (SQL).
func (s *Service) TransferView(ctx context.Context, token, storeID, orderID string) (TransferView, error) {
	if ctx == nil || s == nil || s.pool == nil || !command.ValidID(orderID) {
		return TransferView{}, command.ErrInvalid
	}
	var out TransferView
	tokenHash := sha256.Sum256([]byte(token))
	err := buyer.WithScope(ctx, s.pool, token, storeID, func(callCtx context.Context, tx pgx.Tx, scope buyer.Scope) error {
		var raw []byte
		// checkout.read_bank_transfer_buyer: owner-scoped definer; the only place bank details leave the database for a buyer.
		if err := tx.QueryRow(callCtx, `SELECT checkout.read_bank_transfer_buyer($1,$2::uuid,$3::uuid)`, tokenHash[:], storeID, orderID).Scan(&raw); err != nil {
			return err
		}
		if err := transferStrictDec(raw, &out); err != nil || !validTransferView(out, orderID) {
			return command.ErrConflict
		}
		return checkCapability(callCtx, tx, tokenHash[:], storeID, scope)
	})
	if err != nil {
		return TransferView{}, safeError(ctx, err)
	}
	return out, nil
}

// SubmitTransferProof records (or replaces, until the merchant confirms) the buyer's claim. Idempotent per key: an exact replay returns the
// first answer, another body under the same key is command.ErrConflict. Coded 422 from SQL: transfer_not_open, transfer_window_closed,
// invalid_proof, not_bank_transfer.
func (s *Service) SubmitTransferProof(ctx context.Context, token, storeID, key, orderID string, in ProofInput) (ProofResult, error) {
	if ctx == nil || s == nil || s.pool == nil || !checkoutKey.MatchString(key) || !command.ValidID(orderID) || !validProof(in) {
		return ProofResult{}, command.ErrInvalid
	}
	in.PaidAt = in.PaidAt.UTC().Truncate(time.Microsecond)
	request, err := json.Marshal(struct {
		Op      string `json:"op"`
		OrderID string `json:"order_id"`
		ProofInput
	}{"checkout.transfer.submit", orderID, in})
	if err != nil {
		return ProofResult{}, command.ErrInvalid
	}
	digest := sha256.Sum256(request)
	tokenHash := sha256.Sum256([]byte(token))
	var out ProofResult
	err = buyer.WithScope(ctx, s.pool, token, storeID, func(callCtx context.Context, tx pgx.Tx, scope buyer.Scope) error {
		var raw []byte
		// checkout.submit_transfer_proof: the buyer's own AWAITING_TRANSFER order inside the window; writes only the proof columns.
		if err := tx.QueryRow(callCtx, `SELECT checkout.submit_transfer_proof($1,$2::uuid,$3,$4,$5::uuid,$6,$7::bigint,$8::timestamptz)`,
			tokenHash[:], storeID, key, digest[:], orderID, in.Last5, in.AmountMinor, in.PaidAt).Scan(&raw); err != nil {
			return err
		}
		if err := transferStrictDec(raw, &out); err != nil || out.OrderID != orderID || out.State != "SUBMITTED" || out.ProofCount < 1 || out.SubmittedAt.IsZero() {
			return command.ErrConflict
		}
		return checkCapability(callCtx, tx, tokenHash[:], storeID, scope)
	})
	if err != nil {
		return ProofResult{}, safeError(ctx, err)
	}
	return out, nil
}
