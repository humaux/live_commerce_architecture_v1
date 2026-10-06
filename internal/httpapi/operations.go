// Purpose: the W6-05B merchant HTTP adapter of the failed/UNKNOWN operations ledger (contracts/external-operation-v1.md "Amendment W6-05B"): list, detail and the
//   three audited actions query / cancel / retry. A thin transport gate over internal/integrations/core ledger.go; every rule lives in migration 0159.
// Depends on: core.ListLedger/GetLedger and core.Service.QueryOperation/CancelOperation/RetryOperation; studioRoute/studioPage/studioDecodeRaw/parcelKey helpers;
//   platform.WithScope via scopedAs (one READ COMMITTED transaction, store from the authenticated scope); the insert-only River client (Options.OperationJobs) for query/retry.
// Used by: NewHandler (handler.go registerOperationRoutes). Tests: operations_test.go (DB-free router) and tests/foundation/operations_queue_test.go (REAL_PG).
// Invariants: Idempotency-Key exactly once on every POST, none on GET; the POST body is exactly {"expected_attempts": n}; reads need integration:read, actions integration:execute;
//   a refused action is 409 with its machine code (never "internal"); no request, secret, provider text or buyer PII is ever in a response or a log.
//
// Routes (base = /v1/admin/stores/{store_id}/operations):
//   GET  base?state=&limit=&cursor=     attention list newest first          -> {items:[LedgerItem], next_cursor}
//   GET  base/{operation_id}            one operation + newest 50 events     -> LedgerDetail
//   POST base/{operation_id}/query      re-read provider state (reconcile)   -> {operation_id,state,attempts}   (mounted only with Options.OperationJobs)
//   POST base/{operation_id}/cancel     READY -> CANCELLED                   -> {operation_id,state,attempts}
//   POST base/{operation_id}/retry      re-open / re-queue (never UNKNOWN)   -> {operation_id,state,attempts}   (mounted only with Options.OperationJobs)

package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"livecommerce/internal/command"
	"livecommerce/internal/integrations/core"
	"livecommerce/internal/pagination"
	"livecommerce/internal/platform"
)

// operationActionBody is the exact POST body; a pointer tells a missing field from zero.
type operationActionBody struct {
	ExpectedAttempts *int64 `json:"expected_attempts"`
}

// registerOperationRoutes mounts the ledger routes. query and retry insert a River job, so they are mounted only when jobs is non-nil; the reads and cancel need none.
func registerOperationRoutes(mux *http.ServeMux, pool *pgxpool.Pool, jobs *river.Client[pgx.Tx]) {
	const base = "/v1/admin/stores/{store_id}/operations"
	svc := core.NewLedgerService(jobs)
	mux.HandleFunc("GET "+base, studioRoute(http.MethodGet, true, func(w http.ResponseWriter, r *http.Request) {
		state, page, err := operationsListQuery(r.URL)
		if err != nil {
			respondError(w, http.StatusUnprocessableEntity, "invalid_request")
			return
		}
		scopedAs(pool, "integration:read", operationsClassify, func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return core.ListLedger(ctx, tx, s, bearerToken(r), state, page)
		})(w, r)
	}))
	mux.HandleFunc("GET "+base+"/{operation_id}", studioRoute(http.MethodGet, false, func(w http.ResponseWriter, r *http.Request) {
		if !command.ValidID(r.PathValue("operation_id")) {
			respondError(w, http.StatusUnprocessableEntity, "invalid_request")
			return
		}
		scopedAs(pool, "integration:read", operationsClassify, func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return core.GetLedger(ctx, tx, s, bearerToken(r), r.PathValue("operation_id"))
		})(w, r)
	}))
	type ledgerAction struct {
		name string
		do   func(context.Context, pgx.Tx, platform.Scope, string, string, string, int64) (core.LedgerActionResult, error)
	}
	actions := []ledgerAction{{"cancel", svc.CancelOperation}}
	if jobs != nil {
		actions = append(actions, ledgerAction{"query", svc.QueryOperation}, ledgerAction{"retry", svc.RetryOperation})
	}
	for _, a := range actions {
		do := a.do
		mux.HandleFunc("POST "+base+"/{operation_id}/"+a.name, studioRoute(http.MethodPost, false, func(w http.ResponseWriter, r *http.Request) {
			key, ok := parcelKey(w, r)
			if !ok {
				return
			}
			in, _, ok := studioDecodeRaw[operationActionBody](w, r, []string{"expected_attempts"})
			if !ok {
				return
			}
			if in.ExpectedAttempts == nil || *in.ExpectedAttempts < 0 || !command.ValidID(r.PathValue("operation_id")) {
				respondError(w, http.StatusUnprocessableEntity, "invalid_request")
				return
			}
			scopedAs(pool, "integration:execute", operationsClassify, func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
				return do(ctx, tx, s, bearerToken(r), key, r.PathValue("operation_id"), *in.ExpectedAttempts)
			})(w, r)
		}))
		mux.HandleFunc(base+"/{operation_id}/"+a.name, studioRoute("", false, nil)) // methodless fallback keeps 405 inside the private boundary
	}
	mux.HandleFunc(base, studioRoute("", false, nil))
	mux.HandleFunc(base+"/{operation_id}", studioRoute("", false, nil))
}

// operationsListQuery accepts exactly the optional state (one value), limit and cursor parameters; anything else is invalid.
func operationsListQuery(u *url.URL) (string, pagination.Request, error) {
	var page pagination.Request
	if u.ForceQuery || len(u.RawQuery) > 4096 {
		return "", page, command.ErrInvalid
	}
	values, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return "", page, command.ErrInvalid
	}
	state := ""
	for name, list := range values {
		if len(list) != 1 || list[0] == "" {
			return "", page, command.ErrInvalid
		}
		if name == "state" {
			state = list[0]
			delete(values, name)
		}
	}
	page, err = studioPage(&url.URL{RawQuery: values.Encode()})
	return state, page, err
}

// operationsClassify maps a coded ledger refusal to 409 with its own machine code, then the shared table (404 not_found, 403 forbidden, 422 invalid_request, ...).
func operationsClassify(err error) (int, string) {
	var refusal *core.OperationRefusal
	if errors.As(err, &refusal) {
		return http.StatusConflict, refusal.Code
	}
	return classify(err)
}
