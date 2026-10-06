// Purpose: W3-08B merchant-side minimal returns (RMA): the Go wrappers of the returns.* SQL definers (register, receive, inspect,
//   close, cancel, reads) and the shared mapping of their coded refusals. Every rule (shipped quantity, state machine, restock-once,
//   permissions, idempotency) lives in migration 0155; this file only bounds the input, hashes the request and decodes the answer.
// Depends on: returns.register_rma / receive_rma / inspect_rma / close_rma / cancel_rma / read_order_returns / list_returns
//   (migration 0155, contracts/returns-v1.md), platform.RequirePermission (second authority fence), internal/command.
// Used by: internal/httpapi/returns.go (routes) and internal/fulfillment/cancel.go (MapError, shared refusal codes).
// Invariants: I03/I13 (a return moves stock only at close, as one ledger DEALLOCATE row per sellable line; it never refunds), I02 (one
//   Idempotency-Key = one canonical request: the key's request hash binds the order/RMA, version and every line).
// Status: REAL_PG (no external system).

// Package returns owns the merchant-side minimal returns (RMA) commands of contracts/returns-v1.md: register, receive, inspect, close
// and cancel a return of a shipped order, plus the read projections and the shared coded-refusal mapping of the returns and merchant-cancel
// definers (migration 0155). It never moves stock, money or order state itself (SQL definers do, each behind a ledger provenance guard),
// never starts or changes a refund, and never reads a returns table directly.
package returns

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

// Error is a coded refusal of a returns/cancel definer with the HTTP status it maps to (409 state conflicts, 422 request rules).
type Error struct {
	Code   string
	Status int
}

func (e *Error) Error() string { return "returns: " + e.Code }

// ErrUnavailable is any definer failure that is not a closed-set refusal (the caller answers 503 unavailable).
var ErrUnavailable = errors.New("returns unavailable")

// codes is the closed set of PT409/PT422 messages the 0155 definers (and the release_pay_at_pickup definer they delegate to) raise.
var codes = map[string]int{
	"not_shipped": 409, "not_returnable": 409, "version_changed": 409, "invalid_state": 409, "not_restockable": 409, "has_returns": 409,
	"already_cancelled": 409, "state_changed": 409, "payment_in_flight": 409, "refund_first": 409, "already_shipped": 409,
	"cvs_attempt_in_flight": 409, "payment_review_open": 409,
	"unknown_line": 422, "ambiguous_line": 422, "exceeds_shipped": 422, "exceeds_registered": 422, "invalid_quantities": 422,
	"quantities_mismatch": 422, "lines_incomplete": 422, "nothing_received": 422, "refund_mismatch": 422, "not_cancellable": 422,
}

// MapError maps a definer error: PT400 invalid, PT401/PT403/PT404 platform errors, PT409/PT422 messages in the closed set (the §16.8
// release definer's collection_state_changed is a state_changed), idempotency_conflict as command.ErrConflict, anything else ErrUnavailable.
func MapError(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "PT400":
			return command.ErrInvalid
		case "PT401":
			return platform.ErrUnauthorized
		case "PT403":
			return platform.ErrForbidden
		case "PT404":
			return platform.ErrScopeNotFound
		case "PT409", "PT422":
			msg := pg.Message
			if msg == "idempotency_conflict" {
				return command.ErrConflict
			}
			if msg == "collection_state_changed" {
				msg = "state_changed"
			}
			if status, ok := codes[msg]; ok {
				return &Error{Code: msg, Status: status}
			}
		}
		return ErrUnavailable
	}
	if errors.Is(err, platform.ErrUnauthorized) || errors.Is(err, platform.ErrForbidden) || errors.Is(err, platform.ErrScopeNotFound) ||
		errors.Is(err, command.ErrInvalid) {
		return err
	}
	return ErrUnavailable
}

// ValidAuthority is the shape check every wrapper starts with: a resolved scope and a bearer token of the contract length.
func ValidAuthority(scope platform.Scope, token string) bool {
	return command.ValidID(scope.TenantID) && command.ValidID(scope.StoreID) && command.ValidID(scope.PrincipalID) &&
		scope.Revision > 0 && len(token) >= 32 && len(token) <= 512
}

// RegisterLine asks to return Quantity units of one order line; WarehouseID is only needed when the order holds the SKU in two
// warehouses (otherwise the server resolves it, 422 ambiguous_line when it cannot).
type RegisterLine struct {
	SKUID       string  `json:"sku_id"`
	WarehouseID *string `json:"warehouse_id"`
	Quantity    int64   `json:"quantity"`
}

// ReceiveLine records how many units of a registered line arrived (0..registered).
type ReceiveLine struct {
	SKUID       string  `json:"sku_id"`
	WarehouseID *string `json:"warehouse_id"`
	QtyReceived int64   `json:"qty_received"`
}

// InspectLine splits the received units of a line into sellable (QtyRestock) and scrap; QtyRestock+QtyScrap must equal what arrived.
type InspectLine struct {
	SKUID       string  `json:"sku_id"`
	WarehouseID *string `json:"warehouse_id"`
	QtyRestock  int64   `json:"qty_restock"`
	QtyScrap    int64   `json:"qty_scrap"`
}

// RMALine is one projected line; received/restock/scrap stay null until that stage.
type RMALine struct {
	WarehouseID   string `json:"warehouse_id"`
	SKUID         string `json:"sku_id"`
	QtyRegistered int64  `json:"qty_registered"`
	QtyReceived   *int64 `json:"qty_received"`
	QtyRestock    *int64 `json:"qty_restock"`
	QtyScrap      *int64 `json:"qty_scrap"`
}

// RMA is the projection every command and read returns. RestockedUnits is set by Close only.
type RMA struct {
	ID             string    `json:"id"`
	OrderID        string    `json:"order_id"`
	State          string    `json:"state"`
	Version        int64     `json:"version"`
	Reason         string    `json:"reason"`
	RefundID       *string   `json:"refund_id"`
	CreatedAt      string    `json:"created_at"`
	UpdatedAt      string    `json:"updated_at"`
	Lines          []RMALine `json:"lines"`
	RestockedUnits *int64    `json:"restocked_units,omitempty"`
}

var states = map[string]bool{"REGISTERED": true, "RECEIVED": true, "INSPECTED": true, "CLOSED": true, "CANCELLED": true}

func (r RMA) valid() bool {
	if !command.ValidID(r.ID) || !command.ValidID(r.OrderID) || !states[r.State] || r.Version < 1 || len(r.Lines) == 0 {
		return false
	}
	for _, l := range r.Lines {
		if !command.ValidID(l.WarehouseID) || !command.ValidID(l.SKUID) || l.QtyRegistered < 1 {
			return false
		}
	}
	return true
}

var sqlFor = map[string]string{
	"register": `SELECT returns.register_rma($1,$2::uuid,$3::uuid,$4,$5,$6,$7::jsonb)`,
	"receive":  `SELECT returns.receive_rma($1,$2::uuid,$3::uuid,$4,$5,$6,$7::jsonb)`,
	"inspect":  `SELECT returns.inspect_rma($1,$2::uuid,$3::uuid,$4,$5,$6,$7::jsonb)`,
	"close":    `SELECT returns.close_rma($1,$2::uuid,$3::uuid,$4,$5,$6,$7::uuid)`,
	"cancel":   `SELECT returns.cancel_rma($1,$2::uuid,$3::uuid,$4,$5,$6)`,
	"order":    `SELECT returns.read_order_returns($1,$2::uuid,$3::uuid)`,
	"list":     `SELECT returns.list_returns($1,$2::uuid,$3)`,
}

// run executes one definer statement and returns its jsonb. The Go-side fence re-checks every permission first (the definer checks again).
func run(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string, perms []string, fn string, args ...any) ([]byte, error) {
	if tx == nil || !ValidAuthority(scope, token) {
		return nil, command.ErrInvalid
	}
	for _, p := range perms {
		if err := platform.RequirePermission(ctx, tx, scope, token, p); err != nil {
			return nil, MapError(err)
		}
	}
	auth := sha256.Sum256([]byte(token))
	var raw []byte
	// Calls returns.<fn> (migration 0155, contracts/returns-v1.md); placeholders are bound, never concatenated.
	if err := tx.QueryRow(ctx, sqlFor[fn], append([]any{auth[:], scope.StoreID}, args...)...).Scan(&raw); err != nil {
		return nil, MapError(err)
	}
	if len(raw) == 0 || len(raw) > 4<<20 {
		return nil, ErrUnavailable
	}
	return raw, nil
}

func one(raw []byte, err error, state string) (RMA, error) {
	if err != nil {
		return RMA{}, err
	}
	var out RMA
	if json.Unmarshal(raw, &out) != nil || !out.valid() || (state != "" && out.State != state) {
		return RMA{}, ErrUnavailable
	}
	return out, nil
}

func many(raw []byte, err error) ([]RMA, error) {
	if err != nil {
		return nil, err
	}
	var out []RMA
	if json.Unmarshal(raw, &out) != nil {
		return nil, ErrUnavailable
	}
	for _, r := range out {
		if !r.valid() {
			return nil, ErrUnavailable
		}
	}
	if out == nil {
		out = []RMA{}
	}
	return out, nil
}

func validWH(w *string) bool { return w == nil || command.ValidID(*w) }

// digest binds the idempotency key to ONE canonical request: operation tag, target ids, version and the (sorted) lines.
func digest(tag string, parts ...any) []byte {
	b, _ := json.Marshal(parts)
	sum := sha256.Sum256(append([]byte(tag+"\n"), b...))
	return sum[:]
}

func whKey(w *string) string {
	if w == nil {
		return ""
	}
	return *w
}

// Register records a return of a SHIPPED order (fulfillment:write, Idempotency-Key). SQL checks shipped state and the returnable
// quantity of every line; this only bounds the request. Writes the RMA, its lines and one audit row; never stock or payments.
func Register(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key, orderID, reason string, lines []RegisterLine) (RMA, error) {
	if !command.ValidID(orderID) || len(reason) == 0 || len(reason) > 240 || len(lines) < 1 || len(lines) > 50 || !validKey(key) {
		return RMA{}, command.ErrInvalid
	}
	ls := append([]RegisterLine(nil), lines...)
	for _, l := range ls {
		if !command.ValidID(l.SKUID) || !validWH(l.WarehouseID) || l.Quantity < 1 || l.Quantity > 1e9 {
			return RMA{}, command.ErrInvalid
		}
	}
	sort.Slice(ls, func(i, j int) bool {
		return ls[i].SKUID+"/"+whKey(ls[i].WarehouseID) < ls[j].SKUID+"/"+whKey(ls[j].WarehouseID)
	})
	body, _ := json.Marshal(ls)
	raw, err := run(ctx, tx, scope, token, []string{"fulfillment:write"}, "register", orderID, key, digest("returns.rma.register.v1", orderID, reason, ls), reason, string(body))
	return one(raw, err, "REGISTERED")
}

// Receive records what arrived (fulfillment:write, Idempotency-Key, CAS on version): REGISTERED -> RECEIVED. Writes no stock.
func Receive(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key, rmaID string, expectedVersion int64, lines []ReceiveLine) (RMA, error) {
	if !command.ValidID(rmaID) || expectedVersion < 1 || len(lines) < 1 || len(lines) > 50 || !validKey(key) {
		return RMA{}, command.ErrInvalid
	}
	ls := append([]ReceiveLine(nil), lines...)
	for _, l := range ls {
		if !command.ValidID(l.SKUID) || !validWH(l.WarehouseID) || l.QtyReceived < 0 || l.QtyReceived > 1e9 {
			return RMA{}, command.ErrInvalid
		}
	}
	sort.Slice(ls, func(i, j int) bool {
		return ls[i].SKUID+"/"+whKey(ls[i].WarehouseID) < ls[j].SKUID+"/"+whKey(ls[j].WarehouseID)
	})
	body, _ := json.Marshal(ls)
	raw, err := run(ctx, tx, scope, token, []string{"fulfillment:write"}, "receive", rmaID, key, digest("returns.rma.receive.v1", rmaID, expectedVersion, ls), expectedVersion, string(body))
	return one(raw, err, "RECEIVED")
}

// Inspect records the sellable / scrap split (fulfillment:write AND inventory:write, Idempotency-Key, CAS): RECEIVED -> INSPECTED.
// Records the decision only; stock moves at Close.
func Inspect(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key, rmaID string, expectedVersion int64, lines []InspectLine) (RMA, error) {
	if !command.ValidID(rmaID) || expectedVersion < 1 || len(lines) < 1 || len(lines) > 50 || !validKey(key) {
		return RMA{}, command.ErrInvalid
	}
	ls := append([]InspectLine(nil), lines...)
	for _, l := range ls {
		if !command.ValidID(l.SKUID) || !validWH(l.WarehouseID) || l.QtyRestock < 0 || l.QtyScrap < 0 || l.QtyRestock > 1e9 || l.QtyScrap > 1e9 {
			return RMA{}, command.ErrInvalid
		}
	}
	sort.Slice(ls, func(i, j int) bool {
		return ls[i].SKUID+"/"+whKey(ls[i].WarehouseID) < ls[j].SKUID+"/"+whKey(ls[j].WarehouseID)
	})
	body, _ := json.Marshal(ls)
	raw, err := run(ctx, tx, scope, token, []string{"fulfillment:write", "inventory:write"}, "inspect", rmaID, key, digest("returns.rma.inspect.v1", rmaID, expectedVersion, ls), expectedVersion, string(body))
	return one(raw, err, "INSPECTED")
}

// Close finishes an INSPECTED RMA (fulfillment:write AND inventory:write, Idempotency-Key, CAS): one ledger DEALLOCATE row per line with
// qty_restock>0 returns those units to sellable stock; scrap writes nothing. refundID (optional) must belong to the same order.
func Close(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key, rmaID string, expectedVersion int64, refundID *string) (RMA, error) {
	if !command.ValidID(rmaID) || expectedVersion < 1 || !validKey(key) || (refundID != nil && !command.ValidID(*refundID)) {
		return RMA{}, command.ErrInvalid
	}
	var refund any
	if refundID != nil {
		refund = *refundID
	}
	raw, err := run(ctx, tx, scope, token, []string{"fulfillment:write", "inventory:write"}, "close", rmaID, key, digest("returns.rma.close.v1", rmaID, expectedVersion, refundID), expectedVersion, refund)
	return one(raw, err, "CLOSED")
}

// CancelRMA withdraws a REGISTERED RMA (fulfillment:write, Idempotency-Key, CAS); nothing else is cancellable.
func CancelRMA(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key, rmaID string, expectedVersion int64) (RMA, error) {
	if !command.ValidID(rmaID) || expectedVersion < 1 || !validKey(key) {
		return RMA{}, command.ErrInvalid
	}
	raw, err := run(ctx, tx, scope, token, []string{"fulfillment:write"}, "cancel", rmaID, key, digest("returns.rma.cancel.v1", rmaID, expectedVersion), expectedVersion)
	return one(raw, err, "CANCELLED")
}

// ForOrder lists the RMAs of one order of this store (orders:read). Read only.
func ForOrder(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, orderID string) ([]RMA, error) {
	if !command.ValidID(orderID) {
		return nil, command.ErrInvalid
	}
	raw, err := run(ctx, tx, scope, token, []string{"orders:read"}, "order", orderID)
	return many(raw, err)
}

// List lists the 100 newest RMAs of the store, optionally one state (orders:read). Read only.
func List(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, state string) ([]RMA, error) {
	if state != "" && !states[state] {
		return nil, command.ErrInvalid
	}
	var st any
	if state != "" {
		st = state
	}
	raw, err := run(ctx, tx, scope, token, []string{"orders:read"}, "list", st)
	return many(raw, err)
}

func validKey(key string) bool {
	if len(key) < 8 || len(key) > 128 {
		return false
	}
	for _, c := range key {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '.' || c == ':' || c == '-') {
			return false
		}
	}
	return true
}
