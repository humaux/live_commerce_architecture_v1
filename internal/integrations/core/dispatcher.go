package core

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

var ErrPolicyDenied = errors.New("policy denied")

var (
	errInvalidJob          = errors.New("external_operation_invalid_job")
	errOperationMissing    = errors.New("external_operation_missing")
	errRouteMissing        = errors.New("external_operation_route_missing")
	errDatabase            = errors.New("external_operation_database")
	errCompletionUncertain = errors.New("external_operation_completion_uncertain")
	errBindingBlocked      = errors.New("external_operation_binding_blocked")
	errBudgetExhausted     = errors.New("external_operation_reconcile_budget_exhausted")
)

type DispatchRequest struct {
	OperationID       string          `json:"operation_id"`
	TenantID          string          `json:"tenant_id"`
	StoreID           string          `json:"store_id"`
	PrincipalID       string          `json:"principal_id"`
	BindingID         string          `json:"binding_id"`
	BindingVersion    int64           `json:"binding_version"`
	Provider          string          `json:"provider"`
	ExternalAssetID   string          `json:"external_asset_id"`
	Purpose           string          `json:"purpose"`
	Action            string          `json:"action"`
	Request           json.RawMessage `json:"request"`
	ProviderReference string          `json:"provider_reference"`
	IdempotencyKey    string          `json:"idempotency_key"`
}

type DispatchRoute struct {
	Provider  string
	Action    string
	Purpose   string
	Check     func(context.Context, DispatchRequest) error
	Dispatch  func(context.Context, DispatchRequest) (Outcome, error)
	Reconcile func(context.Context, DispatchRequest) (Outcome, error)
}

type DispatcherOptions struct {
	LeaseSeconds   int
	DBTimeout      time.Duration
	CallTimeout    time.Duration
	RetryDelay     time.Duration
	MaxGenerations int64
}

func DefaultDispatcherOptions() DispatcherOptions {
	return DispatcherOptions{
		LeaseSeconds:   30,
		DBTimeout:      2 * time.Second,
		CallTimeout:    10 * time.Second,
		RetryDelay:     5 * time.Second,
		MaxGenerations: 10,
	}
}

type dispatchRouteKey struct {
	provider string
	action   string
	purpose  string
}

// Dispatcher is a River worker for external_operation_v1. The caller retains
// ownership of its pool and River client lifecycle.
type Dispatcher struct {
	river.WorkerDefaults[externalOperationArgs]
	pool    *pgxpool.Pool
	routes  map[dispatchRouteKey]DispatchRoute
	options DispatcherOptions
	service Service
}

var _ river.Worker[externalOperationArgs] = (*Dispatcher)(nil)

func NewDispatcher(ctx context.Context, pool *pgxpool.Pool, routes []DispatchRoute, options DispatcherOptions) (*Dispatcher, error) {
	if ctx == nil || !validDispatcherOptions(options) {
		return nil, errInvalidJob
	}
	compiled, err := compileDispatchRoutes(routes)
	if err != nil {
		return nil, err
	}
	if err := platform.ValidateWorkerPool(ctx, pool); err != nil {
		return nil, err
	}
	return &Dispatcher{pool: pool, routes: compiled, options: options}, nil
}

func validDispatcherOptions(options DispatcherOptions) bool {
	if options.LeaseSeconds < 5 || options.LeaseSeconds > 300 ||
		options.DBTimeout < 100*time.Millisecond || options.DBTimeout > 5*time.Second ||
		options.CallTimeout < 100*time.Millisecond || options.CallTimeout > 60*time.Second ||
		options.RetryDelay < 100*time.Millisecond || options.RetryDelay > 5*time.Minute ||
		options.MaxGenerations < 2 || options.MaxGenerations > 100 {
		return false
	}
	lease := time.Duration(options.LeaseSeconds) * time.Second
	return options.CallTimeout+2*options.DBTimeout+time.Second < lease
}

func compileDispatchRoutes(routes []DispatchRoute) (map[dispatchRouteKey]DispatchRoute, error) {
	if len(routes) < 1 || len(routes) > 64 {
		return nil, errInvalidJob
	}
	compiled := make(map[dispatchRouteKey]DispatchRoute, len(routes))
	for _, route := range routes {
		if !validProvider(route.Provider) || !actionPattern.MatchString(route.Action) || !validPurpose(route.Purpose) ||
			route.Check == nil || route.Dispatch == nil || route.Reconcile == nil {
			return nil, errInvalidJob
		}
		key := dispatchRouteKey{provider: route.Provider, action: route.Action, purpose: route.Purpose}
		if _, exists := compiled[key]; exists {
			return nil, errInvalidJob
		}
		compiled[key] = route
	}
	return compiled, nil
}

func (d *Dispatcher) Timeout(*river.Job[externalOperationArgs]) time.Duration {
	if d == nil {
		return 0
	}
	return time.Duration(d.options.LeaseSeconds) * time.Second
}

func (d *Dispatcher) NextRetry(*river.Job[externalOperationArgs]) time.Time {
	if d == nil {
		return time.Time{}
	}
	return time.Now().UTC().Add(d.options.RetryDelay)
}

func (d *Dispatcher) Work(ctx context.Context, job *river.Job[externalOperationArgs]) error {
	if d == nil || job == nil || job.Args.Version != 1 || !command.ValidID(job.Args.OperationID) {
		return errInvalidJob
	}
	return d.runOperation(ctx, job.Args.OperationID)
}

func (d *Dispatcher) runOperation(ctx context.Context, operationID string) error {
	if d == nil || d.pool == nil || !command.ValidID(operationID) {
		return errInvalidJob
	}
	operation, err := d.readOperation(ctx, operationID)
	if err != nil {
		return d.safeDatabaseError(ctx, err)
	}
	if terminalOperationState(operation.State) {
		return nil
	}
	if operation.Generation >= d.options.MaxGenerations && operation.ResultCode == "reconcile_budget_exhausted" && operation.LeaseUntil == nil {
		return river.JobCancel(errBudgetExhausted)
	}

	key := dispatchRouteKey{provider: operation.Provider, action: operation.Action, purpose: operation.Purpose}
	route, ok := d.routes[key]
	if !ok {
		return errRouteMissing
	}

	claim, operation, err := d.claimOperation(ctx, operationID)
	if err != nil {
		return d.safeDatabaseError(ctx, err)
	}
	switch claim.Disposition {
	case "busy":
		return river.JobSnooze(d.options.RetryDelay)
	case "terminal":
		if operation.State == "STALE_BINDING" {
			return river.JobCancel(errBindingBlocked)
		}
		return nil
	case "blocked_binding":
		return river.JobCancel(errBindingBlocked)
	case "claimed":
		if claim.Mode != "dispatch" && claim.Mode != "reconcile" {
			return errDatabase
		}
	default:
		return errDatabase
	}

	// A previous max-generation attempt may have died before persisting its
	// terminal queue disposition. This generation is cleanup-only.
	if operation.Generation > d.options.MaxGenerations {
		return d.completeAndFinish(ctx, operation, claim, Outcome{
			State:             "UNKNOWN",
			Code:              "reconcile_budget_exhausted",
			ProviderReference: operation.ProviderReference,
		}, true)
	}

	callCtx, cancel := context.WithTimeout(ctx, d.options.CallTimeout)
	defer cancel()
	checkErr, checkPanicked := invokeCheck(callCtx, route.Check, dispatchRequest(operation))
	if ctx.Err() != nil {
		return river.JobSnooze(d.options.RetryDelay)
	}
	if callCtx.Err() != nil {
		return d.completeAmbiguous(ctx, operation, claim, "callback_timeout")
	}
	if checkPanicked {
		return d.completeAmbiguous(ctx, operation, claim, "callback_panic")
	}
	if checkErr != nil {
		if claim.Mode == "dispatch" && errors.Is(checkErr, ErrPolicyDenied) {
			return d.completeAndFinish(ctx, operation, claim, Outcome{State: "BLOCKED_POLICY", Code: "policy_denied"}, false)
		}
		return d.completeAmbiguous(ctx, operation, claim, "policy_check_failed")
	}

	gate, err := d.finalDispatchGate(callCtx, operation, claim)
	if ctx.Err() != nil {
		return river.JobSnooze(d.options.RetryDelay)
	}
	if err != nil {
		if callCtx.Err() != nil {
			return d.completeAmbiguous(ctx, operation, claim, "callback_timeout")
		}
		return d.completeAmbiguous(ctx, operation, claim, "dispatch_gate_failed")
	}
	if !gate.frozen || !gate.lease {
		return errCompletionUncertain
	}
	if !gate.binding {
		if claim.Mode == "dispatch" {
			return d.completeAndFinish(ctx, operation, claim, Outcome{State: "BLOCKED_POLICY", Code: "binding_changed"}, false)
		}
		return d.completeAmbiguous(ctx, operation, claim, "binding_changed")
	}

	request := dispatchRequest(operation)
	var outcome Outcome
	var callbackErr error
	var callbackPanicked bool
	if claim.Mode == "dispatch" {
		outcome, callbackErr, callbackPanicked = invokeOutcome(callCtx, route.Dispatch, request)
	} else {
		outcome, callbackErr, callbackPanicked = invokeOutcome(callCtx, route.Reconcile, request)
	}
	if ctx.Err() != nil {
		return river.JobSnooze(d.options.RetryDelay)
	}
	if callCtx.Err() != nil {
		return d.completeAmbiguous(ctx, operation, claim, "callback_timeout")
	}
	if callbackPanicked {
		return d.completeAmbiguous(ctx, operation, claim, "callback_panic")
	}
	if callbackErr != nil {
		return d.completeAmbiguous(ctx, operation, claim, "callback_failed")
	}
	if !validAdapterOutcome(outcome) {
		return d.completeAmbiguous(ctx, operation, claim, "adapter_result_invalid")
	}
	if (outcome.State == "UNKNOWN" || outcome.State == "ACKNOWLEDGED") && outcome.ProviderReference == "" {
		outcome.ProviderReference = operation.ProviderReference
	}
	exhausted := operation.Generation >= d.options.MaxGenerations &&
		(outcome.State == "UNKNOWN" || outcome.State == "ACKNOWLEDGED")
	if exhausted {
		outcome.Code = "reconcile_budget_exhausted"
	}
	return d.completeAndFinish(ctx, operation, claim, outcome, exhausted)
}

func (d *Dispatcher) safeDatabaseError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return river.JobSnooze(d.options.RetryDelay)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return errOperationMissing
	}
	return errDatabase
}

func (d *Dispatcher) completeAmbiguous(ctx context.Context, operation Operation, claim ClaimResult, code string) error {
	outcome := Outcome{State: "UNKNOWN", Code: code, ProviderReference: operation.ProviderReference}
	exhausted := operation.Generation >= d.options.MaxGenerations
	if exhausted {
		outcome.Code = "reconcile_budget_exhausted"
	}
	return d.completeAndFinish(ctx, operation, claim, outcome, exhausted)
}

func (d *Dispatcher) completeAndFinish(ctx context.Context, operation Operation, claim ClaimResult, outcome Outcome, exhausted bool) error {
	if ctx.Err() != nil {
		return river.JobSnooze(d.options.RetryDelay)
	}
	if err := d.completeOperation(ctx, operation.ID, claim, outcome); err != nil {
		return errCompletionUncertain
	}
	if exhausted {
		return river.JobCancel(errBudgetExhausted)
	}
	switch outcome.State {
	case "SUCCEEDED", "FAILED_FINAL":
		return nil
	case "BLOCKED_POLICY":
		return nil
	case "UNKNOWN", "ACKNOWLEDGED":
		return river.JobSnooze(d.options.RetryDelay)
	default:
		return errCompletionUncertain
	}
}

func (d *Dispatcher) readOperation(ctx context.Context, operationID string) (Operation, error) {
	bounded, cancel := context.WithTimeout(ctx, d.options.DBTimeout)
	defer cancel()
	return scanOperation(d.pool.QueryRow(bounded, operationQuery, operationID))
}

func (d *Dispatcher) claimOperation(ctx context.Context, operationID string) (ClaimResult, Operation, error) {
	bounded, cancel := context.WithTimeout(ctx, d.options.DBTimeout)
	defer cancel()
	tx, err := d.pool.BeginTx(bounded, pgx.TxOptions{})
	if err != nil {
		return ClaimResult{}, Operation{}, err
	}
	defer d.rollback(tx)
	claim, err := d.service.Claim(bounded, tx, operationID, d.options.LeaseSeconds)
	if err != nil {
		return ClaimResult{}, Operation{}, err
	}
	operation, err := scanOperation(tx.QueryRow(bounded, operationQuery, operationID))
	if err != nil {
		return ClaimResult{}, Operation{}, err
	}
	if err := tx.Commit(bounded); err != nil {
		return ClaimResult{}, Operation{}, err
	}
	return claim, operation, nil
}

func (d *Dispatcher) completeOperation(ctx context.Context, operationID string, claim ClaimResult, outcome Outcome) error {
	bounded, cancel := context.WithTimeout(ctx, d.options.DBTimeout)
	defer cancel()
	tx, err := d.pool.BeginTx(bounded, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer d.rollback(tx)
	if err := d.service.Complete(bounded, tx, operationID, claim.Generation, claim.LeaseToken, outcome); err != nil {
		return err
	}
	return tx.Commit(bounded)
}

func (d *Dispatcher) rollback(tx pgx.Tx) {
	cleanup, cancel := context.WithTimeout(context.Background(), d.options.DBTimeout)
	defer cancel()
	_ = tx.Rollback(cleanup)
}

type dispatchGate struct {
	frozen  bool
	binding bool
	lease   bool
}

func (d *Dispatcher) finalDispatchGate(ctx context.Context, operation Operation, claim ClaimResult) (dispatchGate, error) {
	bounded, cancel := context.WithTimeout(ctx, d.options.DBTimeout)
	defer cancel()
	var gate dispatchGate
	err := d.pool.QueryRow(bounded, `SELECT
		o.binding_id=$2 AND o.binding_version=$3 AND o.provider=$4 AND o.external_asset_id=$5,
		b.enabled AND b.semantic_version=o.binding_version AND b.provider=o.provider AND b.external_asset_id=o.external_asset_id,
		o.generation=$6 AND o.lease_mode=$7 AND o.lease_until>clock_timestamp()
		  AND o.lease_token_hash=sha256($8) AND o.state=CASE WHEN $7='dispatch' THEN 'DISPATCHING' ELSE 'UNKNOWN' END
		FROM integration.operations o
		JOIN integration.bindings b ON b.id=o.binding_id AND b.tenant_id=o.tenant_id AND b.store_id=o.store_id
		WHERE o.id=$1 AND o.actor_kind='MERCHANT'`, operation.ID, operation.BindingID, operation.BindingVersion, operation.Provider,
		operation.ExternalAssetID, claim.Generation, claim.Mode, claim.LeaseToken).Scan(&gate.frozen, &gate.binding, &gate.lease)
	return gate, err
}

const operationQuery = `SELECT id::text,tenant_id::text,store_id::text,principal_id::text,binding_id::text,
	binding_version,provider,external_asset_id,purpose,action,request,state,generation,lease_mode,lease_until,
	result_code,provider_reference FROM integration.operations WHERE id=$1 AND actor_kind='MERCHANT'`

type rowScanner interface {
	Scan(...any) error
}

func scanOperation(row rowScanner) (operation Operation, err error) {
	err = row.Scan(&operation.ID, &operation.TenantID, &operation.StoreID, &operation.PrincipalID, &operation.BindingID,
		&operation.BindingVersion, &operation.Provider, &operation.ExternalAssetID, &operation.Purpose, &operation.Action,
		&operation.Request, &operation.State, &operation.Generation, &operation.LeaseMode, &operation.LeaseUntil,
		&operation.ResultCode, &operation.ProviderReference)
	return operation, err
}

func dispatchRequest(operation Operation) DispatchRequest {
	return DispatchRequest{
		OperationID: operation.ID, TenantID: operation.TenantID, StoreID: operation.StoreID,
		PrincipalID: operation.PrincipalID, BindingID: operation.BindingID, BindingVersion: operation.BindingVersion,
		Provider: operation.Provider, ExternalAssetID: operation.ExternalAssetID, Purpose: operation.Purpose,
		Action: operation.Action, Request: append(json.RawMessage(nil), operation.Request...),
		ProviderReference: operation.ProviderReference, IdempotencyKey: "lc:" + operation.ID,
	}
}

func invokeCheck(ctx context.Context, callback func(context.Context, DispatchRequest) error, request DispatchRequest) (err error, panicked bool) {
	defer func() {
		if recover() != nil {
			err, panicked = nil, true
		}
	}()
	return callback(ctx, request), false
}

func invokeOutcome(ctx context.Context, callback func(context.Context, DispatchRequest) (Outcome, error), request DispatchRequest) (outcome Outcome, err error, panicked bool) {
	defer func() {
		if recover() != nil {
			outcome, err, panicked = Outcome{}, nil, true
		}
	}()
	outcome, err = callback(ctx, request)
	return outcome, err, false
}

func validAdapterOutcome(outcome Outcome) bool {
	return (outcome.State == "SUCCEEDED" || outcome.State == "FAILED_FINAL" || outcome.State == "UNKNOWN" || outcome.State == "ACKNOWLEDGED") &&
		codePattern.MatchString(outcome.Code) && validReference(outcome.ProviderReference)
}

func terminalOperationState(state string) bool {
	return state == "SUCCEEDED" || state == "FAILED_FINAL" || state == "CANCELLED" || state == "BLOCKED_POLICY" || state == "STALE_BINDING"
}
