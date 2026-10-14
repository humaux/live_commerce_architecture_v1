// Purpose: execute a frozen C1 intent as one complete local C2 transaction.
// Depends on: protected ops stages, native inventory allocation, plan/pay keyrings,
// inbox notification cryptography and River InsertTx on this exact pool.
// Used by: inline execution and the order_attempt_v1 worker, never ordinary Place.
// Invariants: attempt fence first; terminal replay wins; only exact definitive
// failures may abort after rollback; no provider call, new admission or bearer receipt.

package orderattempt

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"livecommerce/internal/checkout"
	"livecommerce/internal/command"
	"livecommerce/internal/inbox"
	"livecommerce/internal/integrations/core"
	"livecommerce/internal/inventory"
)

// Executor borrows immutable keys and one pool. Both execution lanes use the
// same implementation; SQL authenticates the actual login and original intent.
type Executor struct {
	pool          *pgxpool.Pool
	plans         *PlanKeys
	pay           *PayEntryKeys
	notifications *inbox.Service
	jobs          *river.Client[pgx.Tx]
}

// NewExecutor performs no I/O and starts no workers. The process composition
// root must establish database identity and provision the distinct keyrings.
func NewExecutor(pool *pgxpool.Pool, plans *PlanKeys, pay *PayEntryKeys, notifications *inbox.Service) (*Executor, error) {
	if pool == nil || plans == nil || pay == nil || notifications == nil || !notifications.SendEnabled() {
		return nil, &AdmissionError{code: "invalid_request"}
	}
	jobs, err := river.NewClient(riverpgxv5.New(pool), &river.Config{Schema: "river"})
	if err != nil {
		return nil, &AdmissionError{code: "order_attempt_unavailable"}
	}
	return &Executor{pool: pool, plans: plans, pay: pay, notifications: notifications, jobs: jobs}, nil
}

type executionFence struct {
	State    string       `json:"state"`
	Binding  *PlanBinding `json:"binding"`
	Envelope *SealedPlan  `json:"envelope"`
}

// Execute returns a terminal receipt only after commit. Empty merchantToken is
// accepted by SQL only for the actual claims-worker login. Timeout/connection
// loss never clears admission; a later exact execution reads the durable state.
func (e *Executor) Execute(ctx context.Context, merchantToken, store, attempt string, generation int64) (ExecutionReceipt, error) {
	if ctx == nil || e == nil || e.pool == nil || !validExecutionTarget(merchantToken, store, attempt, generation) {
		return ExecutionReceipt{}, &AdmissionError{code: "invalid_request"}
	}
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := e.pool.BeginTx(bounded, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return ExecutionReceipt{}, admissionFailure(err, false)
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 2*time.Second)
		defer stop()
		_ = tx.Rollback(cleanup)
	}()
	if _, err = tx.Exec(bounded, `SELECT set_config('statement_timeout','5s',true),
	 set_config('lock_timeout','5s',true),set_config('idle_in_transaction_session_timeout','5s',true)`); err != nil {
		return ExecutionReceipt{}, admissionFailure(err, false)
	}
	var authHash []byte
	if merchantToken != "" {
		digest := sha256.Sum256([]byte(merchantToken))
		authHash = digest[:]
	}
	var storeID *string
	if store != "" {
		storeID = &store
	}
	var raw []byte
	// SECURITY DEFINER: ops.lock_order_attempt takes the first business row lock,
	// rechecks generation/read authority, and returns immutable terminal or plan data.
	err = tx.QueryRow(bounded, `SELECT ops.lock_order_attempt($1::bytea,$2::uuid,$3::uuid,$4::bigint)`,
		authHash, storeID, attempt, generation).Scan(&raw)
	if err != nil {
		return ExecutionReceipt{}, admissionFailure(err, false)
	}
	defer clear(raw)
	// The first fence may consume the existing five-second C2 budget while
	// another executor finishes. Later domain locks retain their one-second cap.
	if _, err = tx.Exec(bounded, `SELECT set_config('lock_timeout','1s',true)`); err != nil {
		return ExecutionReceipt{}, admissionFailure(err, false)
	}
	var fence executionFence
	if len(raw) > 2*maxPlanBytes || json.Unmarshal(raw, &fence) != nil {
		return ExecutionReceipt{}, &AdmissionError{code: "order_attempt_invalid_response"}
	}
	if fence.State == "COMMITTED" || fence.State == "ABORTED" {
		out, err := readExecutionReceipt(raw, attempt, generation)
		if err != nil {
			return ExecutionReceipt{}, err
		}
		if err = tx.Commit(bounded); err != nil {
			return ExecutionReceipt{}, admissionFailure(err, true)
		}
		return out, nil
	}
	if fence.State != "PENDING" || fence.Binding == nil || fence.Envelope == nil ||
		fence.Binding.AttemptID != attempt || fence.Binding.Generation != generation {
		return ExecutionReceipt{}, &AdmissionError{code: "order_attempt_invalid_response"}
	}
	defer clear(fence.Envelope.Ciphertext)
	// Native notification planning pins the River row to the top-level xid.
	// C2 therefore runs without Go savepoints; a definitive refusal first rolls
	// back everything and then proves absence in a new, attempt-first fence.
	out, executionErr := e.executeFenced(bounded, tx, *fence.Binding, *fence.Envelope)
	if executionErr != nil {
		if errors.Is(executionErr, ErrPlanKeyUnavailable) {
			// A process-local ring miss is compatible with deployment skew and
			// does not establish permanent loss. Preserve the admitted intent.
			return ExecutionReceipt{}, &AdmissionError{code: "order_attempt_retryable"}
		}
		code := executionAbortCode(executionErr)
		if code == "" {
			return ExecutionReceipt{}, admissionFailure(executionErr, false)
		}
		if err = tx.Rollback(bounded); err != nil {
			return ExecutionReceipt{}, admissionFailure(err, true)
		}
		// Abort rechecks durable state under its own fence. A competing executor
		// that committed meanwhile wins as terminal replay, never as ABORTED.
		// The original five-second budget also bounds this second transaction.
		return (&Service{pool: e.pool}).Abort(bounded, merchantToken, store, attempt, generation, code)
	}
	if err = tx.Commit(bounded); err != nil {
		return ExecutionReceipt{}, admissionFailure(err, true)
	}
	return out, nil
}

func (e *Executor) executeFenced(ctx context.Context, tx pgx.Tx, binding PlanBinding, envelope SealedPlan) (ExecutionReceipt, error) {
	var expired bool
	if err := tx.QueryRow(ctx, `SELECT clock_timestamp()>=$1::timestamptz`, binding.ExecuteBy).Scan(&expired); err != nil {
		return ExecutionReceipt{}, err
	}
	if expired {
		return ExecutionReceipt{}, &pgconn.PgError{Code: "PT409", Message: "attempt_expired"}
	}
	plan, err := e.plans.Open(binding, envelope)
	if err != nil {
		return ExecutionReceipt{}, err
	}
	defer clear(plan.Facts)
	if err = plan.VerifyCalculation(); err != nil {
		return ExecutionReceipt{}, err
	}
	plain, err := json.Marshal(plan)
	if err != nil {
		return ExecutionReceipt{}, ErrPlanInvalid
	}
	defer clear(plain)
	raw, err := executeStage(ctx, tx, binding, plain, "prepare", map[string]any{})
	if err != nil {
		return ExecutionReceipt{}, err
	}
	var prepared struct {
		Stage         string `json:"stage"`
		CartID        string `json:"cart_id"`
		QuoteID       string `json:"quote_id"`
		DestinationID string `json:"destination_id"`
		Allocation    struct {
			OrderID      string              `json:"order_id"`
			WarehouseIDs []string            `json:"warehouse_ids"`
			Demands      []inventory.Demand  `json:"demands"`
			Balances     []inventory.Balance `json:"balances"`
		} `json:"allocation"`
	}
	if json.Unmarshal(raw, &prepared) != nil || prepared.Stage != "PREPARED" || prepared.Allocation.OrderID != binding.AttemptID ||
		!command.ValidID(prepared.CartID) || !command.ValidID(prepared.QuoteID) || !command.ValidID(prepared.DestinationID) {
		return ExecutionReceipt{}, &AdmissionError{code: "order_attempt_invalid_response"}
	}
	lines := []inventory.Line{}
	if len(prepared.Allocation.Demands) > 0 {
		lines, err = inventory.PlanAllocation(prepared.Allocation.WarehouseIDs, prepared.Allocation.Demands, prepared.Allocation.Balances)
		if err != nil {
			return ExecutionReceipt{}, err
		}
	}
	raw, err = executeStage(ctx, tx, binding, plain, "begin", map[string]any{
		"cart_id": prepared.CartID, "quote_id": prepared.QuoteID, "destination_id": prepared.DestinationID, "allocation": lines,
	})
	if err != nil {
		return ExecutionReceipt{}, err
	}
	var begun checkout.Result
	if json.Unmarshal(raw, &begun) != nil || begun.OrderID != binding.AttemptID || begun.ReservationID != binding.AttemptID || begun.JobID < 1 {
		return ExecutionReceipt{}, &AdmissionError{code: "order_attempt_invalid_response"}
	}
	payBinding := PayEntryBinding{TenantID: binding.TenantID, StoreID: binding.StoreID, OrderID: binding.AttemptID, Version: 1}
	material, err := e.pay.Issue(payBinding)
	if err != nil {
		return ExecutionReceipt{}, err
	}
	defer clear(material.Ciphertext)
	payInput := map[string]any{
		"key_version": material.KeyVersion, "token_hash_hex": hex.EncodeToString(material.Digest[:]),
		"credential_ciphertext_hex": hex.EncodeToString(material.Ciphertext),
	}
	raw, err = executeStage(ctx, tx, binding, plain, "pay", payInput)
	if err != nil {
		return ExecutionReceipt{}, err
	}
	var pay struct {
		Stage string `json:"stage"`
		Entry struct {
			OrderID string `json:"order_id"`
			Version int64  `json:"link_version"`
		} `json:"pay_entry"`
		Notification json.RawMessage `json:"notification"`
	}
	if json.Unmarshal(raw, &pay) != nil || pay.Stage != "PAY_PREPARED" || pay.Entry.OrderID != binding.AttemptID || pay.Entry.Version != 1 {
		return ExecutionReceipt{}, &AdmissionError{code: "order_attempt_invalid_response"}
	}
	defer clear(pay.Notification)
	// If the notification window closes before finish, roll back all C2 effects
	// and retain PENDING. The next same-intent execution derives VOID afresh;
	// there can be no orphan job or partial order from this transaction.
	artifact, err := e.notificationArtifact(ctx, tx, pay.Notification, payBinding, material)
	if err != nil {
		return ExecutionReceipt{}, err
	}
	defer clear(artifact)
	raw, err = executeStage(ctx, tx, binding, plain, "finish", artifact)
	if err != nil {
		return ExecutionReceipt{}, err
	}
	return readExecutionReceipt(raw, binding.AttemptID, binding.Generation)
}

func (e *Executor) notificationArtifact(ctx context.Context, tx pgx.Tx, prepared json.RawMessage, binding PayEntryBinding, material PayEntryMaterial) (json.RawMessage, error) {
	var disposition struct {
		State  string `json:"state"`
		Code   string `json:"void_code"`
		Origin string `json:"origin"`
		Locale string `json:"locale"`
	}
	if json.Unmarshal(prepared, &disposition) != nil {
		return nil, &AdmissionError{code: "order_attempt_invalid_response"}
	}
	switch disposition.State {
	case "NOT_REQUESTED":
		return json.RawMessage(`{"state":"NOT_REQUESTED"}`), nil
	case "VOID":
		return json.Marshal(map[string]string{"state": "VOID", "void_code": disposition.Code})
	case "PREPARED":
		// The credential is decrypted only after the native pay entry and current
		// notification/origin facts were established under the retained order lock.
		token, err := e.pay.Open(binding, material)
		if err != nil {
			return nil, err
		}
		link := disposition.Origin + "/" + disposition.Locale + "/pay-entry#o=" + binding.OrderID + "&t=" + token
		var operation, outbound string
		if err = tx.QueryRow(ctx, `SELECT gen_random_uuid()::text,gen_random_uuid()::text`).Scan(&operation, &outbound); err != nil {
			return nil, err
		}
		job, err := core.InsertOperationJob(ctx, e.jobs, tx, operation)
		if err != nil {
			return nil, err
		}
		return e.notifications.PrepareOrderAttemptNotification(prepared, link, operation, outbound, job)
	default:
		return nil, &AdmissionError{code: "order_attempt_invalid_response"}
	}
}

func executeStage(ctx context.Context, tx pgx.Tx, binding PlanBinding, plain []byte, step string, input any) ([]byte, error) {
	encoded, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	defer clear(encoded)
	var raw []byte
	// SECURITY DEFINER: ops.execute_order_attempt validates the current-xid
	// original plan and calls native domain helpers in canonical lock order.
	err = tx.QueryRow(ctx, `SELECT ops.execute_order_attempt($1::uuid,$2::bigint,$3::text,$4::text,$5::jsonb)`,
		binding.AttemptID, binding.Generation, string(plain), step, string(encoded)).Scan(&raw)
	return raw, err
}

// executionAbortCode is deliberately a closed pair table. A generic 4xx, SQL
// error, transport observation or custody inconsistency never proves absence.
func executionAbortCode(err error) string {
	if errors.Is(err, command.ErrInsufficient) {
		return "insufficient_inventory"
	}
	if errors.Is(err, ErrPlanKeyUnavailable) {
		return "PLAN_UNAVAILABLE"
	}
	if errors.Is(err, ErrPlanInvalid) {
		return "PLAN_INVALID"
	}
	var pgerr *pgconn.PgError
	if !errors.As(err, &pgerr) {
		return ""
	}
	switch pgerr.Code {
	case "PT400":
		if pgerr.Message == "plan_invalid" || pgerr.Message == "invalid_order_attempt_plan" {
			return "PLAN_INVALID"
		}
	case "PT402":
		if pgerr.Message == "insufficient_stock" {
			return "insufficient_inventory"
		}
	case "PT404":
		switch pgerr.Message {
		case "resource_unavailable", "store_unavailable", "sku_not_found":
			return "resource_unavailable"
		}
	case "PT409":
		switch pgerr.Message {
		case "attempt_expired":
			return "EXPIRED"
		case "owner_unavailable", "claim_unavailable", "allocation_unavailable", "warehouse_unavailable", "claim_quantity_exceeded":
			return "resource_unavailable"
		case "bundle_reserved", "bundle_order_exists":
			return pgerr.Message
		case "offline_settings_unavailable":
			return "payment_unavailable"
		}
	case "PT410":
		if pgerr.Message == "owner_erased" {
			return "ERASED"
		}
	case "PT422":
		switch pgerr.Message {
		case "delivery_service_unavailable":
			return "invalid_destination"
		case "pay_at_pickup_unavailable", "cash_on_delivery_unavailable":
			return "payment_unavailable"
		case "sku_inactive", "sku_changed", "sku_unavailable":
			return "resource_unavailable"
		case "max_per_order_exceeded":
			return "limit_exceeded"
		}
	case "PT429":
		if pgerr.Message == "offline_order_limit" {
			return "limit_exceeded"
		}
	}
	return ""
}
