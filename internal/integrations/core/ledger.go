// Purpose: the merchant failed/UNKNOWN operations ledger (W6-05B): a store-scoped read model of integration.operations and three audited actions
//   (query, cancel, retry) that only ever move an operation through the transitions of contracts/external-operation-v1.md "Amendment W6-05B".
// Depends on: SQL integration.read_operation_ledger / request_operation_query / cancel_operation / retry_operation (migration 0159, owner commerce_integration_writer,
//   EXECUTE commerce_runtime); command.Run (idempotency receipt) and command.AuditDetails (ops.audit_events); River InsertTx via InsertOperationJob[On] (jobs.go);
//   pagination (collection "operations"); platform.RequirePermission (integration:read / integration:execute).
//   River jobs enqueued: external_operation_v1 {operation_id, version:1} on the default queue (query and retry only; the ads lane is read + cancel only).
// Used by: internal/httpapi/operations.go (merchant routes); tests/foundation/operations_queue_test.go.
// Invariants: I06/I07 -- UNKNOWN is never retried (retry answers reconcile_first; query only ever leads to a reconcile-mode claim, never dispatch); a retry reuses the same
//   operation id / semantic key / lc:<operation_id> provider key; the DTO carries exactly the keys below and no request, secret, provider body or buyer PII.
// Status: REAL_PG + MOCK provider evidence only (no provider call happens in this file).

package core

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/riverqueue/river"

	"livecommerce/internal/command"
	"livecommerce/internal/pagination"
	"livecommerce/internal/platform"
)

// ledgerCursorTime is the byte-canonical microsecond UTC layout the "operations" cursor collection accepts.
const ledgerCursorTime = "2006-01-02T15:04:05.000000Z"

// LedgerAction says whether one action is available for an operation and, when it is not, the stable machine reason (the same code the POST answers with as 409).
type LedgerAction struct {
	Available bool   `json:"available"`
	Reason    string `json:"reason"`
}

// LedgerActions are the three merchant actions of an operation.
type LedgerActions struct {
	Query  LedgerAction `json:"query"`
	Cancel LedgerAction `json:"cancel"`
	Retry  LedgerAction `json:"retry"`
}

// LedgerObject is the business object an operation belongs to (order, conversation, claim_bundle, ad_draft, payment_attempt, binding, live_session), by internal id only.
type LedgerObject struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

// LedgerItem is the exact public projection of one operation. SQL builds these keys; nothing else of the row (request, semantic key, asset id,
// provider reference, principal, lease) is ever read into Go.
type LedgerItem struct {
	OperationID string        `json:"operation_id"`
	Provider    string        `json:"provider"`
	Action      string        `json:"action"`
	Purpose     string        `json:"purpose"`
	State       string        `json:"state"`
	ReasonCode  string        `json:"reason_code"`
	Attempts    int64         `json:"attempts"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
	Object      *LedgerObject `json:"object"`
	Actions     LedgerActions `json:"actions"`
}

// LedgerEvent is one history row of an operation (machine codes only).
type LedgerEvent struct {
	Generation int64     `json:"generation"`
	State      string    `json:"state"`
	ReasonCode string    `json:"reason_code"`
	CreatedAt  time.Time `json:"created_at"`
}

// LedgerDetail is a LedgerItem plus its newest events (at most 50, newest first).
type LedgerDetail struct {
	LedgerItem
	Events []LedgerEvent `json:"events"`
}

// LedgerActionResult is the answer of query/cancel/retry.
type LedgerActionResult struct {
	OperationID string `json:"operation_id"`
	State       string `json:"state"`
	Attempts    int64  `json:"attempts"`
}

// NewLedgerService is New for the ledger routes: jobs may be nil, in which case list, detail and cancel work and query/retry answer command.ErrInvalid (they need the
// insert-only River client to enqueue the follow-up job).
func NewLedgerService(jobs *river.Client[pgx.Tx]) *Service { return &Service{jobs: jobs} }

// OperationRefusal is a coded 409 of a ledger action: the operation exists but the action is not allowed in its current state. Code is one of refusalCodes.
type OperationRefusal struct{ Code string }

// Error implements error with the machine code only.
func (r *OperationRefusal) Error() string { return "operation action refused: " + r.Code }

// refusalCodes are the PT409 messages integration.ledger_capability / ledger_open may raise; anything else maps to a plain command.ErrConflict.
var refusalCodes = map[string]bool{
	"lane_unsupported": true, "not_in_doubt": true, "lease_active": true, "binding_changed": true, "query_in_progress": true, "query_too_soon": true,
	"already_dispatched": true, "operation_closed": true, "already_succeeded": true, "reconcile_first": true, "retry_not_supported": true,
	"already_queued": true, "operation_changed": true,
}

// normalizeLedgerState maps the request's state parameter to the closed filter set ("" -> attention, FAILED -> FAILED_FINAL); ok is false for anything else.
func normalizeLedgerState(state string) (string, bool) {
	switch state {
	case "", "attention":
		return "attention", true
	case "FAILED", "FAILED_FINAL":
		return "FAILED_FINAL", true
	case "UNKNOWN", "ACKNOWLEDGED", "BLOCKED_POLICY", "STALE_BINDING", "READY":
		return state, true
	}
	return "", false
}

// ListLedger pages the store's ledger newest first (integration:read). Read only: one SQL definer call, no write, no provider call.
func ListLedger(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, state string, page pagination.Request) (pagination.Page[LedgerItem], error) {
	empty := pagination.Page[LedgerItem]{Items: []LedgerItem{}}
	filter, ok := normalizeLedgerState(state)
	if !ok {
		return empty, command.ErrInvalid
	}
	if err := authorize(ctx, tx, scope, token, readPermission); err != nil {
		return empty, err
	}
	binding := pagination.Binding{TenantID: scope.TenantID, StoreID: scope.StoreID, Collection: "operations", Filter: filter}
	limit, keys, err := pagination.Decode(page, binding, 2)
	if err != nil {
		return empty, err
	}
	var afterAt, afterID any
	if len(keys) == 2 {
		afterAt, afterID = keys[0], keys[1]
	}
	hash := sha256.Sum256([]byte(token))
	var raw []byte
	// integration.read_operation_ledger (0159): fresh integration:read auth, store-scoped, exact DTO keys; one extra row says whether a next page exists.
	if err := tx.QueryRow(ctx, `SELECT integration.read_operation_ledger($1,$2::uuid,$3,NULL,$4::timestamptz,$5::uuid,$6)`,
		hash[:], scope.StoreID, filter, afterAt, afterID, limit).Scan(&raw); err != nil {
		return empty, mapLedgerError(err)
	}
	var res struct {
		Items   []LedgerItem `json:"items"`
		HasMore bool         `json:"has_more"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return empty, err
	}
	out := pagination.Page[LedgerItem]{Items: res.Items}
	if out.Items == nil {
		out.Items = []LedgerItem{}
	}
	if res.HasMore && len(out.Items) > 0 {
		last := out.Items[len(out.Items)-1]
		if out.NextCursor, err = pagination.Encode(binding, []string{last.CreatedAt.UTC().Format(ledgerCursorTime), last.OperationID}); err != nil {
			return empty, err
		}
	}
	return out, nil
}

// GetLedger reads one operation of the caller's store with its newest events (integration:read). Another store's id, a payment/media-lane or non-merchant
// operation is command.ErrNotFound.
func GetLedger(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, operationID string) (LedgerDetail, error) {
	if !command.ValidID(operationID) {
		return LedgerDetail{}, command.ErrInvalid
	}
	if err := authorize(ctx, tx, scope, token, readPermission); err != nil {
		return LedgerDetail{}, err
	}
	hash := sha256.Sum256([]byte(token))
	var raw []byte
	// integration.read_operation_ledger (0159): single-id form; PT404 when the id is not a visible operation of this store.
	if err := tx.QueryRow(ctx, `SELECT integration.read_operation_ledger($1,$2::uuid,'attention',$3::uuid,NULL,NULL,1)`,
		hash[:], scope.StoreID, operationID).Scan(&raw); err != nil {
		return LedgerDetail{}, mapLedgerError(err)
	}
	var res struct {
		Items  []LedgerItem  `json:"items"`
		Events []LedgerEvent `json:"events"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return LedgerDetail{}, err
	}
	if len(res.Items) != 1 {
		return LedgerDetail{}, command.ErrNotFound
	}
	if res.Events == nil {
		res.Events = []LedgerEvent{}
	}
	return LedgerDetail{LedgerItem: res.Items[0], Events: res.Events}, nil
}

// QueryOperation asks the existing dispatcher to re-read provider state for an UNKNOWN/ACKNOWLEDGED/expired-DISPATCHING operation: it inserts one external_operation_v1 job in
// this transaction and restarts the bounded reconcile budget. The job can only be claimed in reconcile mode (claim_operation never returns dispatch for these states).
func (s *Service) QueryOperation(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key, operationID string, expectedAttempts int64) (LedgerActionResult, error) {
	return s.ledgerAction(ctx, tx, scope, token, key, "query", operationID, expectedAttempts)
}

// CancelOperation marks a never-dispatched (READY) operation CANCELLED. It is a local fact, never a claim of remote reversal, and is refused once a dispatch may have happened.
func (s *Service) CancelOperation(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key, operationID string, expectedAttempts int64) (LedgerActionResult, error) {
	return s.ledgerAction(ctx, tx, scope, token, key, "cancel", operationID, expectedAttempts)
}

// RetryOperation re-opens a FAILED_FINAL operation of a registered read-only kind, or re-queues a READY operation without a live job. UNKNOWN is never retried
// (OperationRefusal reconcile_first). Reuses the same operation id, semantic key and provider idempotency key; inserts one job in this transaction.
func (s *Service) RetryOperation(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key, operationID string, expectedAttempts int64) (LedgerActionResult, error) {
	return s.ledgerAction(ctx, tx, scope, token, key, "retry", operationID, expectedAttempts)
}

// ledgerAudit is the ops.audit_events action recorded for each ledger action.
var ledgerAudit = map[string]string{
	"query": "integration.operation_query_requested", "cancel": "integration.operation_cancelled", "retry": "integration.operation_retry_authorized",
}

func (s *Service) ledgerAction(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key, kind, operationID string, expectedAttempts int64) (LedgerActionResult, error) {
	if s == nil || (kind != "cancel" && s.jobs == nil) || !command.ValidID(operationID) || expectedAttempts < 0 {
		return LedgerActionResult{}, command.ErrInvalid
	}
	if err := authorize(ctx, tx, scope, token, executePermission); err != nil {
		return LedgerActionResult{}, err
	}
	hash := sha256.Sum256([]byte(token))
	request := struct {
		OperationID      string `json:"operation_id"`
		ExpectedAttempts int64  `json:"expected_attempts"`
	}{operationID, expectedAttempts}
	var out LedgerActionResult
	err := command.Run(ctx, tx, scope, "integration.operation."+kind, key, request, &out, func() error {
		var jobID int64
		if kind != "cancel" {
			// The follow-up job rides the default lane exactly as Plan creates it. An ads-lane operation is refused by ledger_capability before its job is
			// looked at, and the refusal rolls this insert back with the rest of the transaction.
			var err error
			if jobID, err = InsertOperationJob(ctx, s.jobs, tx, operationID); err != nil {
				return mapLedgerError(err)
			}
		}
		var raw []byte
		var err error
		// integration.request_operation_query / cancel_operation / retry_operation (0159): integration:execute auth, binding FOR SHARE -> operation FOR UPDATE, generation CAS,
		// ledger_capability, same-transaction job check (query/retry). A refusal raises PT409 and the whole transaction, job included, rolls back.
		switch kind {
		case "query":
			err = tx.QueryRow(ctx, `SELECT integration.request_operation_query($1,$2::uuid,$3::uuid,$4,$5)`, hash[:], scope.StoreID, operationID, expectedAttempts, jobID).Scan(&raw)
		case "retry":
			err = tx.QueryRow(ctx, `SELECT integration.retry_operation($1,$2::uuid,$3::uuid,$4,$5)`, hash[:], scope.StoreID, operationID, expectedAttempts, jobID).Scan(&raw)
		default:
			err = tx.QueryRow(ctx, `SELECT integration.cancel_operation($1,$2::uuid,$3::uuid,$4)`, hash[:], scope.StoreID, operationID, expectedAttempts).Scan(&raw)
		}
		if err != nil {
			return mapLedgerError(err)
		}
		if err := json.Unmarshal(raw, &out); err != nil {
			return err
		}
		// The audit row carries the operation id only: no request, no provider text.
		return command.AuditDetails(ctx, tx, scope, ledgerAudit[kind], map[string]any{"operation_id": operationID})
	})
	if err != nil {
		return LedgerActionResult{}, mapLedgerError(err)
	}
	// A replay remains an authenticated request even after its lock wait.
	if err := authorize(ctx, tx, scope, token, executePermission); err != nil {
		return LedgerActionResult{}, err
	}
	return out, nil
}

// mapLedgerError maps the PTnnn SQLSTATEs of the 0159 definers to the shared transport errors; a coded 409 becomes an *OperationRefusal.
func mapLedgerError(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "PT401":
			return platform.ErrUnauthorized
		case "PT403":
			return platform.ErrForbidden
		case "PT404":
			return command.ErrNotFound
		case "PT422":
			return command.ErrInvalid
		case "PT409":
			if refusalCodes[pg.Message] {
				return &OperationRefusal{Code: pg.Message}
			}
			return command.ErrConflict
		}
	}
	return mapError(err)
}
