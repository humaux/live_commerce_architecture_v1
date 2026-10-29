// Purpose: OA-2 C3/C4 authority for the order payment notification: one locked
// dispatch permit, paired outbox/operation outcomes, lease expiry to UNKNOWN and
// the generic-retry refusal. This file is the frozen command/result surface for
// OA-3; it performs no provider I/O itself and owns no HTTP route.
// Depends on: the requested coordinator definers (see output/oa-2/SQL-REQUEST.md)
// and the OA-1 outbox/pair trigger in migrations/0188_order_attempt_notification.sql.
// Used by: the claims-worker payment-DM entry, order cancellation and rotation
// composition (integrator-assigned paths), and OA-3 public commands.
// Invariants: READY->DISPATCHING is the only send permit; provider I/O happens
// outside PG exactly once per winning generation; every outcome writes the outbox
// and its operation in one transaction; expired leases become UNKNOWN and never
// authorize another send; READY/VOID/FAILED_FINAL are not retryable generically.
// Status: MOCK. C3/C4 acceptance is NOT_RUN until the assigned migration exists.

package metareply

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// OrderPaymentLeaseSeconds is the single C3 dispatch/reconcile lease bound.
	// It is deliberately inside integration.claim_operation's 5..300 window.
	OrderPaymentLeaseSeconds = 60
	// OrderPaymentSweepLimit bounds one discovery batch; each row still commits alone.
	OrderPaymentSweepLimit = 100
)

var (
	// ErrOrderPaymentNotRetryable is the typed refusal for generic operation retry.
	ErrOrderPaymentNotRetryable = errors.New("metareply: order payment notification is not retryable")
	// ErrOrderPaymentStaleGeneration rejects a completion that lost the lease or generation fence.
	ErrOrderPaymentStaleGeneration = errors.New("metareply: order payment notification stale generation")
	// ErrOrderPaymentState rejects an outcome outside the frozen paired transition table.
	ErrOrderPaymentState = errors.New("metareply: order payment notification invalid state")
	// ErrOrderPaymentSQLPending marks the seam before the integrator-assigned definers exist.
	ErrOrderPaymentSQLPending = errors.New("metareply: order payment notification SQL is not assigned")
)

// OrderPaymentNotification is the paired outbox projection. It carries no
// recipient, body, credential or bearer URL.
type OrderPaymentNotification struct {
	NotificationID       string     `json:"notification_id"`
	State                string     `json:"state"`
	OperationID          string     `json:"operation_id"`
	OrderID              string     `json:"order_id"`
	LinkVersion          int64      `json:"link_version"`
	WorkerGeneration     int64      `json:"worker_generation"`
	VoidStage            string     `json:"void_stage"`
	VoidCode             string     `json:"void_code"`
	ConversationID       string     `json:"conversation_id"`
	BindingID            string     `json:"binding_id"`
	DispatchAuthorizedAt *time.Time `json:"dispatch_authorized_at"`
}

// OrderPaymentClaim is the C3 result. LeaseToken is never serialised.
type OrderPaymentClaim struct {
	Disposition  string                   `json:"disposition"`
	Generation   int64                    `json:"generation"`
	Mode         string                   `json:"mode"`
	Notification OrderPaymentNotification `json:"notification"`
	LeaseToken   []byte                   `json:"-"`
}

// OrderPaymentOutcome is the C4 request in outbox vocabulary.
type OrderPaymentOutcome struct {
	State             string `json:"state"`
	Code              string `json:"code"`
	ProviderReference string `json:"provider_reference"`
}

// OrderPaymentDispatch is the immutable input of one provider call.
type OrderPaymentDispatch struct {
	OperationID  string
	Generation   int64
	Mode         string
	LeaseToken   []byte
	Notification OrderPaymentNotification
}

// OrderPaymentResult is the frozen OA-3 dispatch result. Calls is the number of
// external calls this invocation made; it is 0 or 1, never more.
type OrderPaymentResult struct {
	Disposition  string                   `json:"disposition"`
	Outcome      OrderPaymentOutcome      `json:"outcome"`
	Notification OrderPaymentNotification `json:"notification"`
	Calls        int                      `json:"calls"`
}

// OrderPaymentSweepResult is the lease-sweep receipt.
type OrderPaymentSweepResult struct {
	Scanned int      `json:"scanned"`
	Expired []string `json:"expired"`
}

// OrderPaymentStore is the coordinator-definer seam. Each method is exactly one
// of the requested functions and must commit both paired rows or neither.
type OrderPaymentStore interface {
	Claim(ctx context.Context, operationID string, leaseSeconds int) (OrderPaymentClaim, error)
	Complete(ctx context.Context, operationID string, generation int64, token []byte, outcome OrderPaymentOutcome) (OrderPaymentNotification, error)
	Read(ctx context.Context, operationID string) (OrderPaymentNotification, bool, error)
	Expired(ctx context.Context, limit int) ([]string, error)
	Expire(ctx context.Context, operationID string) (OrderPaymentNotification, error)
}

// OrderPaymentDispatcher owns the C3/C4 state machine. Send and Query are
// injected so the C3 permit cannot be bypassed by a caller-owned HTTP path.
type OrderPaymentDispatcher struct {
	store OrderPaymentStore
	send  func(context.Context, OrderPaymentDispatch) (OrderPaymentOutcome, error)
	query func(context.Context, OrderPaymentDispatch) (OrderPaymentOutcome, error)
	lease int
}

// NewOrderPaymentDispatcher wires the store and the one-shot provider callbacks.
// A nil send or query disables that mode; it never falls back to a generic retry.
func NewOrderPaymentDispatcher(store OrderPaymentStore, send, query func(context.Context, OrderPaymentDispatch) (OrderPaymentOutcome, error)) (*OrderPaymentDispatcher, error) {
	if store == nil {
		return nil, errors.New("metareply: order payment store required")
	}
	if send == nil && query == nil {
		return nil, errors.New("metareply: order payment callback required")
	}
	return &OrderPaymentDispatcher{store: store, send: send, query: query, lease: OrderPaymentLeaseSeconds}, nil
}

// Dispatch runs one job invocation. A busy or terminal row returns with zero
// calls. A claimed row gets exactly one external call; the paired outcome is
// written before returning. If that write fails, the lease sweep settles the row
// as UNKNOWN; this method never repeats the call.
func (d *OrderPaymentDispatcher) Dispatch(ctx context.Context, operationID string) (OrderPaymentResult, error) {
	var out OrderPaymentResult
	if d == nil || d.store == nil || !validUUID(operationID) {
		return out, errors.New("metareply: invalid operation")
	}
	claim, err := d.store.Claim(ctx, operationID, d.lease)
	if err != nil {
		return out, err
	}
	out.Disposition = claim.Disposition
	out.Notification = claim.Notification
	switch claim.Disposition {
	case "busy", "terminal", "void", "blocked_binding":
		return out, nil
	case "claimed":
	default:
		return out, ErrOrderPaymentState
	}
	if claim.Generation < 1 || (claim.Mode != "dispatch" && claim.Mode != "reconcile") || len(claim.LeaseToken) != 32 {
		return out, ErrOrderPaymentState
	}
	call := func(fn func(context.Context, OrderPaymentDispatch) (OrderPaymentOutcome, error)) (OrderPaymentOutcome, error) {
		if fn == nil {
			return OrderPaymentOutcome{}, ErrOrderPaymentState
		}
		out.Calls++
		return fn(ctx, OrderPaymentDispatch{
			OperationID: operationID, Generation: claim.Generation, Mode: claim.Mode,
			LeaseToken: append([]byte(nil), claim.LeaseToken...), Notification: claim.Notification,
		})
	}
	var outcome OrderPaymentOutcome
	if claim.Mode == "dispatch" {
		outcome, err = call(d.send)
	} else {
		outcome, err = call(d.query)
	}
	if err != nil {
		// The external call may already have happened; only UNKNOWN is truthful.
		outcome = OrderPaymentOutcome{State: "DELIVERY_UNKNOWN", Code: "callback_failed"}
	}
	outcome, err = normalizeOrderPaymentOutcome(outcome, claim.Mode)
	if err != nil {
		return out, err
	}
	notification, err := d.store.Complete(ctx, operationID, claim.Generation, claim.LeaseToken, outcome)
	if err != nil {
		return out, err
	}
	out.Outcome = outcome
	out.Notification = notification
	return out, nil
}

// Sweep discovers expired DISPATCHING leases without locks, then settles each in
// its own fenced transaction as DELIVERY_UNKNOWN/UNKNOWN. It never sends.
func (d *OrderPaymentDispatcher) Sweep(ctx context.Context) (OrderPaymentSweepResult, error) {
	var out OrderPaymentSweepResult
	if d == nil || d.store == nil {
		return out, errors.New("metareply: order payment store required")
	}
	ids, err := d.store.Expired(ctx, OrderPaymentSweepLimit)
	if err != nil {
		return out, err
	}
	out.Scanned = len(ids)
	for _, operationID := range ids {
		if !validUUID(operationID) {
			continue
		}
		notification, err := d.store.Expire(ctx, operationID)
		if err != nil {
			return out, err
		}
		out.Expired = append(out.Expired, operationID)
		_ = notification
	}
	return out, nil
}

// NotRetryable is the frozen hook for the generic operation API. Every paired
// payment-DM row, in every state, refuses generic retry: READY/VOID/FAILED_FINAL
// reopen is forbidden and a new send must be a new reminder intent.
func (d *OrderPaymentDispatcher) NotRetryable(ctx context.Context, operationID string) (bool, string, error) {
	if d == nil || d.store == nil || !validUUID(operationID) {
		return false, "", errors.New("metareply: invalid operation")
	}
	notification, found, err := d.store.Read(ctx, operationID)
	if err != nil {
		return false, "", err
	}
	if !found {
		return false, "", nil
	}
	return true, notification.State, nil
}

// ReminderSemanticKey is the per-version unique key (tenant/store are columns,
// not key text). link_version is part of the key.
func ReminderSemanticKey(orderID string, linkVersion int64, clientKey string) ([]byte, error) {
	if !validUUID(orderID) || linkVersion < 1 || !validClientKey(clientKey) {
		return nil, errors.New("metareply: invalid reminder identity")
	}
	sum := sha256.Sum256([]byte("reminder|" + orderID + "|" + strconv.FormatInt(linkVersion, 10) + "|reminder|" + clientKey))
	return sum[:], nil
}

// ReminderIntentKey binds one client action across rotations and states so a lost
// response reuses the original row instead of sending again.
func ReminderIntentKey(orderID, clientKey string) ([]byte, error) {
	if !validUUID(orderID) || !validClientKey(clientKey) {
		return nil, errors.New("metareply: invalid reminder identity")
	}
	sum := sha256.Sum256([]byte("reminder-intent|" + orderID + "|reminder|" + clientKey))
	return sum[:], nil
}

// ReminderKeyHex is the SQL bytea spelling used by the requested admit definer.
func ReminderKeyHex(key []byte) string { return hex.EncodeToString(key) }

func normalizeOrderPaymentOutcome(outcome OrderPaymentOutcome, mode string) (OrderPaymentOutcome, error) {
	switch outcome.State {
	case "ACCEPTED":
		outcome.State = "ACCEPTED"
		if outcome.Code == "" {
			outcome.Code = "provider_accepted"
		}
	case "DELIVERY_UNKNOWN", "ACKNOWLEDGED":
		// ACKNOWLEDGED without acceptance proof is still unknown to the buyer.
		outcome.State = "DELIVERY_UNKNOWN"
		if outcome.Code == "" {
			outcome.Code = "delivery_unknown"
		}
	case "VOID":
		// Only reconcile may prove a dispatch produced no effect; a dispatch
		// callback cannot both act and claim authoritative no-send.
		if mode != "reconcile" {
			return OrderPaymentOutcome{}, ErrOrderPaymentState
		}
		if outcome.Code == "" {
			outcome.Code = "no_effect"
		}
	default:
		return OrderPaymentOutcome{}, ErrOrderPaymentState
	}
	if len(outcome.Code) == 0 || len(outcome.Code) > 80 || !paymentCodePattern.MatchString(outcome.Code) {
		return OrderPaymentOutcome{}, ErrOrderPaymentState
	}
	if len(outcome.ProviderReference) > 200 {
		return OrderPaymentOutcome{}, ErrOrderPaymentState
	}
	return outcome, nil
}

var (
	paymentCodePattern = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,80}$`)
	uuidPattern        = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	clientKeyPattern   = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,128}$`)
)

func validUUID(value string) bool { return uuidPattern.MatchString(value) }
func validClientKey(value string) bool {
	return clientKeyPattern.MatchString(value)
}

// PGOrderPaymentStore is the pgx implementation of the coordinator seam. Every
// call is one statement, hence one database transaction owned by the definer.
type PGOrderPaymentStore struct {
	pool *pgxpool.Pool
}

// NewPGOrderPaymentStore returns the production seam. The requested definers are
// named in output/oa-2/SQL-REQUEST.md.
func NewPGOrderPaymentStore(pool *pgxpool.Pool) (*PGOrderPaymentStore, error) {
	if pool == nil {
		return nil, errors.New("metareply: order payment pool required")
	}
	return &PGOrderPaymentStore{pool: pool}, nil
}

// Claim calls inbox.payment_dm_c3: order/conversation/binding/outbox/operation
// locks in contract order, revalidation, then the existing operation claim.
func (s *PGOrderPaymentStore) Claim(ctx context.Context, operationID string, leaseSeconds int) (OrderPaymentClaim, error) {
	var out OrderPaymentClaim
	if s == nil || s.pool == nil || !validUUID(operationID) || leaseSeconds < 5 || leaseSeconds > 300 {
		return out, errors.New("metareply: invalid claim")
	}
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return out, err
	}
	var raw []byte
	if err := s.pool.QueryRow(ctx, `SELECT inbox.payment_dm_c3($1::uuid,$2::integer,$3::bytea)`,
		operationID, leaseSeconds, token).Scan(&raw); err != nil {
		return out, mapOrderPaymentSQLError(err)
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return OrderPaymentClaim{}, err
	}
	if out.Disposition == "claimed" {
		out.LeaseToken = token
	}
	return out, nil
}

// Complete calls inbox.payment_dm_c4: the generation/token fence plus the paired
// operation/outbox write in one transaction.
func (s *PGOrderPaymentStore) Complete(ctx context.Context, operationID string, generation int64, token []byte, outcome OrderPaymentOutcome) (OrderPaymentNotification, error) {
	var out OrderPaymentNotification
	if s == nil || s.pool == nil || !validUUID(operationID) || generation < 1 || len(token) != 32 {
		return out, errors.New("metareply: invalid completion")
	}
	if _, err := normalizeOrderPaymentOutcome(outcome, "reconcile"); err != nil {
		return out, err
	}
	var raw []byte
	if err := s.pool.QueryRow(ctx, `SELECT inbox.payment_dm_c4($1::uuid,$2::bigint,$3::bytea,$4::text,$5::text,$6::text)`,
		operationID, generation, token, outcome.State, outcome.Code, outcome.ProviderReference).Scan(&raw); err != nil {
		return out, mapOrderPaymentSQLError(err)
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return OrderPaymentNotification{}, err
	}
	return out, nil
}

// Read calls the narrow paired projection; worker roles have no outbox SELECT.
func (s *PGOrderPaymentStore) Read(ctx context.Context, operationID string) (OrderPaymentNotification, bool, error) {
	var out OrderPaymentNotification
	if s == nil || s.pool == nil || !validUUID(operationID) {
		return out, false, errors.New("metareply: invalid read")
	}
	var raw []byte
	if err := s.pool.QueryRow(ctx, `SELECT inbox.payment_dm_read($1::uuid)`, operationID).Scan(&raw); err != nil {
		return out, false, mapOrderPaymentSQLError(err)
	}
	if len(raw) == 0 || string(raw) == "null" {
		return out, false, nil
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return OrderPaymentNotification{}, false, err
	}
	return out, true, nil
}

// Expired calls the lock-free discovery definer (bounded batch, no locks).
func (s *PGOrderPaymentStore) Expired(ctx context.Context, limit int) ([]string, error) {
	if s == nil || s.pool == nil || limit < 1 || limit > OrderPaymentSweepLimit {
		return nil, errors.New("metareply: invalid sweep bound")
	}
	rows, err := s.pool.Query(ctx, `SELECT inbox.payment_dm_expired($1::integer)`, limit)
	if err != nil {
		return nil, mapOrderPaymentSQLError(err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// Expire calls the per-row fenced UNKNOWN transition; the caller discovers each
// row without holding another order's lock.
func (s *PGOrderPaymentStore) Expire(ctx context.Context, operationID string) (OrderPaymentNotification, error) {
	var out OrderPaymentNotification
	if s == nil || s.pool == nil || !validUUID(operationID) {
		return out, errors.New("metareply: invalid expire")
	}
	var raw []byte
	if err := s.pool.QueryRow(ctx, `SELECT inbox.payment_dm_expire($1::uuid)`, operationID).Scan(&raw); err != nil {
		return out, mapOrderPaymentSQLError(err)
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return OrderPaymentNotification{}, err
	}
	return out, nil
}

func mapOrderPaymentSQLError(err error) error {
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		switch pgErr.SQLState() {
		case "40001":
			return fmt.Errorf("%w: %v", ErrOrderPaymentStaleGeneration, err)
		case "23514", "PT409":
			return fmt.Errorf("%w: %v", ErrOrderPaymentState, err)
		case "42883":
			return fmt.Errorf("%w: %v", ErrOrderPaymentSQLPending, err)
		}
	}
	return err
}

// compile-time assertion that the pgx seam satisfies the dispatcher.
var _ OrderPaymentStore = (*PGOrderPaymentStore)(nil)
