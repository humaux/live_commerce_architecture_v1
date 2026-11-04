// Purpose: typed transport wrappers for separate buyer after-sales applications (0210); legacy RMA parser stays unchanged.
// Depends on: pgx scoped transactions, command validation, buyer capability and merchant platform permissions.
// Used by: buyerhttp/return_requests.go and httpapi/return_requests.go.
// Invariants: static bound SQL only; safe projections; receipt/CAS and all business decisions belong to the definer transaction.

package returns

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

type RequestInput struct {
	Reason string         `json:"reason"`
	Note   string         `json:"note"`
	Lines  []RegisterLine `json:"lines"`
}
type RequestDecision struct {
	ExpectedVersion int64  `json:"expected_version"`
	Reason          string `json:"reason,omitempty"`
}
type RequestLine struct {
	SKUID       string `json:"sku_id"`
	WarehouseID string `json:"warehouse_id"`
	Quantity    int64  `json:"quantity"`
}
type RequestEvent struct {
	Status string `json:"status"`
	At     string `json:"at"`
}
type BuyerRequest struct {
	ID              string         `json:"id"`
	Number          string         `json:"number"`
	OrderID         string         `json:"order_id"`
	Status          string         `json:"status"`
	Version         int64          `json:"version"`
	Reason          string         `json:"reason"`
	Note            string         `json:"note"`
	RejectionReason *string        `json:"rejection_reason"`
	ReturnID        *string        `json:"return_id"`
	ReturnState     *string        `json:"return_state"`
	CreatedAt       string         `json:"created_at"`
	UpdatedAt       string         `json:"updated_at"`
	Lines           []RequestLine  `json:"lines"`
	Timeline        []RequestEvent `json:"timeline"`
}
type RequestList struct {
	Items []BuyerRequest `json:"items"`
}

func requestText(s string, limit int) bool {
	return utf8.ValidString(s) && utf8.RuneCountInString(s) <= limit && strings.IndexFunc(s, unicode.IsControl) < 0
}
func validRequestInput(in RequestInput) bool {
	if in.Reason != "damaged" || !requestText(in.Note, 1000) || len(in.Lines) < 1 || len(in.Lines) > 50 {
		return false
	}
	seen := map[string]bool{}
	for _, l := range in.Lines {
		k := l.SKUID + ":" + whKey(l.WarehouseID)
		if !command.ValidID(l.SKUID) || !validWH(l.WarehouseID) || l.Quantity < 1 || l.Quantity > 999999999 || seen[k] {
			return false
		}
		seen[k] = true
	}
	return true
}
func (r BuyerRequest) valid() bool {
	if !command.ValidID(r.ID) || !command.ValidID(r.OrderID) || !strings.HasPrefix(r.Number, "R-") || r.Version < 1 || r.Reason != "damaged" || len(r.Lines) == 0 || len(r.Timeline) == 0 {
		return false
	}
	switch r.Status {
	case "requested", "approved", "rejected", "withdrawn":
	default:
		return false
	}
	for _, l := range r.Lines {
		if !command.ValidID(l.SKUID) || !command.ValidID(l.WarehouseID) || l.Quantity < 1 {
			return false
		}
	}
	return (r.Status == "approved") == (r.ReturnID != nil) && (r.ReturnID == nil || command.ValidID(*r.ReturnID) && r.ReturnState != nil && states[*r.ReturnState])
}
func requestQuery(ctx context.Context, tx pgx.Tx, token, sql string, args ...any) ([]byte, error) {
	if tx == nil || len(token) < 32 || len(token) > 512 {
		return nil, command.ErrInvalid
	}
	hash := sha256.Sum256([]byte(token))
	var raw []byte
	if err := tx.QueryRow(ctx, sql, append([]any{hash[:]}, args...)...).Scan(&raw); err != nil {
		return nil, MapError(err)
	}
	if len(raw) == 0 || len(raw) > 4<<20 {
		return nil, ErrUnavailable
	}
	return raw, nil
}
func requestOne(raw []byte, err error) (BuyerRequest, error) {
	if err != nil {
		return BuyerRequest{}, err
	}
	var out BuyerRequest
	if json.Unmarshal(raw, &out) != nil || !out.valid() {
		return BuyerRequest{}, ErrUnavailable
	}
	return out, nil
}
func CreateBuyerRequest(ctx context.Context, tx pgx.Tx, token, store, order, key string, in RequestInput) (BuyerRequest, error) {
	if !command.ValidID(store) || !command.ValidID(order) || !validKey(key) || !validRequestInput(in) {
		return BuyerRequest{}, command.ErrInvalid
	}
	lines, err := json.Marshal(in.Lines)
	if err != nil {
		return BuyerRequest{}, command.ErrInvalid
	}
	return requestOne(requestQuery(ctx, tx, token, `SELECT returns.create_buyer_request($1,$2::uuid,$3::uuid,$4,$5,$6,$7::jsonb)`, store, order, key, in.Reason, in.Note, lines))
}

// ReadBuyerRequests returns either a list or one application. Merchant authority is checked in Go and SQL; buyer scope is resolved in SQL.
func ReadBuyerRequests(ctx context.Context, tx pgx.Tx, token, store, id string, merchant bool, scope platform.Scope) (any, error) {
	if !command.ValidID(store) || id != "" && !command.ValidID(id) {
		return nil, command.ErrInvalid
	}
	if merchant {
		if err := platform.RequirePermission(ctx, tx, scope, token, "orders:read"); err != nil {
			return nil, MapError(err)
		}
	}
	var target any
	if id != "" {
		target = id
	}
	raw, err := requestQuery(ctx, tx, token, `SELECT returns.read_buyer_requests($1,$2::uuid,$3::uuid,$4)`, store, target, merchant)
	if id != "" {
		return requestOne(raw, err)
	}
	if err != nil {
		return nil, err
	}
	var out RequestList
	if json.Unmarshal(raw, &out) != nil || out.Items == nil {
		return nil, ErrUnavailable
	}
	for _, r := range out.Items {
		if !r.valid() {
			return nil, ErrUnavailable
		}
	}
	return out, nil
}
func DecideBuyerRequest(ctx context.Context, tx pgx.Tx, token, store, id, key, action string, in RequestDecision, scope platform.Scope) (BuyerRequest, error) {
	if !command.ValidID(store) || !command.ValidID(id) || !validKey(key) || in.ExpectedVersion < 1 {
		return BuyerRequest{}, command.ErrInvalid
	}
	switch action {
	case "approve", "reject":
		if err := platform.RequirePermission(ctx, tx, scope, token, "fulfillment:write"); err != nil {
			return BuyerRequest{}, MapError(err)
		}
	case "withdraw":
	default:
		return BuyerRequest{}, command.ErrInvalid
	}
	if action == "reject" && (strings.TrimSpace(in.Reason) == "" || !requestText(in.Reason, 1000)) {
		return BuyerRequest{}, &Error{Code: "invalid_request", Status: 422}
	}
	if action != "reject" && in.Reason != "" {
		return BuyerRequest{}, command.ErrInvalid
	}
	return requestOne(requestQuery(ctx, tx, token, `SELECT returns.decide_buyer_request($1,$2::uuid,$3::uuid,$4,$5,$6,$7)`, store, id, key, in.ExpectedVersion, action, in.Reason))
}
