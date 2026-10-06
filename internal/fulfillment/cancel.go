// Purpose: W3-08B merchant cancel of an unshipped order: the Go wrapper of fulfillment.merchant_cancel_order (migration 0155). One
//   command for every payment mode: an unpaid hold releases its reservation, a paid card order is cancelled only after refunds cover
//   the whole capture (409 refund_first, never an automatic refund), a pay-at-pickup/COD order goes through the existing §16.8 definer.
// Depends on: fulfillment.merchant_cancel_order (contracts/returns-v1.md §3), platform.RequirePermission (second authority fence),
//   internal/returns (MapError: the closed set of coded refusals), internal/command.
// Used by: internal/httpapi/returns.go (POST /orders/{id}/cancel).
// Invariants: I13 (cancel, refund and return stay separate; the order is never cancelled by a refund and a cancel never starts one),
//   I03 (stock is released only through a guarded ledger row), I02 (Idempotency-Key bound to order, expected state and reason).
// Status: REAL_PG (no external system).

package fulfillment

import (
	"context"
	"crypto/sha256"
	"encoding/json"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
	"livecommerce/internal/returns"
)

// CancelGroup describes the parcel group an order left when it was cancelled (nil when it was in none).
type CancelGroup struct {
	ID      string `json:"id"`
	State   string `json:"state"` // OPEN (still >= 2 orders) or DISSOLVED (the last survivor dissolved it)
	Version int64  `json:"version"`
}

// CancelResult is the 200 body: the order is CANCELLED and ReleasedLines order lines gave their stock back.
type CancelResult struct {
	OrderID         string       `json:"order_id"`
	CommercialState string       `json:"commercial_state"`
	ReleasedLines   int          `json:"released_lines"`
	ParcelGroup     *CancelGroup `json:"parcel_group"`
	Reason          string       `json:"reason"`
}

// CancelOrder cancels an unshipped order (fulfillment:write, Idempotency-Key). expectedState is the commercial state the merchant saw
// (DRAFT, AWAITING_PAYMENT, CONFIRMED, AWAITING_COLLECTION); the definer answers 409 state_changed when it moved. Writes the order and
// reservation state, ledger release rows and one audit row; never a payment or refund row.
func CancelOrder(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key, orderID, expectedState, reason string) (CancelResult, error) {
	if tx == nil || !returns.ValidAuthority(scope, token) || !command.ValidID(orderID) || len(reason) == 0 || len(reason) > 240 || len(key) < 8 || len(key) > 128 {
		return CancelResult{}, command.ErrInvalid
	}
	switch expectedState {
	case "DRAFT", "AWAITING_PAYMENT", "CONFIRMED", "AWAITING_COLLECTION":
	default:
		return CancelResult{}, command.ErrInvalid
	}
	if err := platform.RequirePermission(ctx, tx, scope, token, "fulfillment:write"); err != nil {
		return CancelResult{}, returns.MapError(err)
	}
	auth := sha256.Sum256([]byte(token))
	sum := sha256.Sum256([]byte("fulfillment.merchant_cancel.v1\n" + orderID + "\n" + expectedState + "\n" + reason))
	var raw []byte
	// Calls fulfillment.merchant_cancel_order (migration 0155, contracts/returns-v1.md §3); idempotency key = the request's own key.
	if err := tx.QueryRow(ctx, `SELECT fulfillment.merchant_cancel_order($1,$2::uuid,$3::uuid,$4,$5,$6,$7)`,
		auth[:], scope.StoreID, orderID, key, sum[:], expectedState, reason).Scan(&raw); err != nil {
		return CancelResult{}, returns.MapError(err)
	}
	var out CancelResult
	if len(raw) == 0 || len(raw) > 1<<16 || json.Unmarshal(raw, &out) != nil || out.OrderID != orderID || out.CommercialState != "CANCELLED" || out.ReleasedLines < 0 {
		return CancelResult{}, returns.ErrUnavailable
	}
	return out, nil
}
