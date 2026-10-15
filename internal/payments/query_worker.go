// query_worker.go dispatches only Stripe Checkout operations.
// Depends on the profile worker pool and River; called by payment-worker.
// Historical providers are rejected before lease claim, credential load, provider I/O or observation writes.

package payments

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"livecommerce/internal/command"
	"livecommerce/internal/integrations/accounts"
	"livecommerce/internal/integrations/core"
	"livecommerce/internal/platform"
)

var (
	errPaymentQueryJob      = errors.New("payment_query_invalid_job")
	errPaymentQueryDatabase = errors.New("payment_query_database")
	errPaymentQueryFamily   = errors.New("payment_query_wrong_family")
)

type paymentQueryArgs struct {
	OperationID string `json:"operation_id"`
	Version     int    `json:"version"`
}

func (paymentQueryArgs) Kind() string { return "payment_query_v1" }

type QueryWorkerOptions struct {
	LeaseSeconds   int
	DBTimeout      time.Duration
	CallTimeout    time.Duration
	RetryDelay     time.Duration
	MaxAge         time.Duration
	MaxGenerations int64
	MockTransport  http.RoundTripper
}

func DefaultQueryWorkerOptions() QueryWorkerOptions {
	return QueryWorkerOptions{LeaseSeconds: 30, DBTimeout: 2 * time.Second,
		CallTimeout: 10 * time.Second, RetryDelay: 2 * time.Minute,
		MaxAge: 24 * time.Hour, MaxGenerations: 720}
}

func validQueryWorkerOptions(o QueryWorkerOptions) bool {
	if o.LeaseSeconds < 5 || o.LeaseSeconds > 300 ||
		o.DBTimeout < 100*time.Millisecond || o.DBTimeout > 5*time.Second ||
		o.CallTimeout < 100*time.Millisecond || o.CallTimeout > 60*time.Second ||
		o.RetryDelay < time.Second || o.RetryDelay > time.Hour ||
		o.MaxAge < time.Minute || o.MaxAge > 7*24*time.Hour ||
		o.MaxGenerations < 2 || o.MaxGenerations > 10000 {
		return false
	}
	return o.CallTimeout+2*o.DBTimeout+time.Second < time.Duration(o.LeaseSeconds)*time.Second
}

// QueryWorker only queries a frozen buyer attempt. The caller owns the pool
// and River lifecycle.
type QueryWorker struct {
	river.WorkerDefaults[paymentQueryArgs]
	pool    *pgxpool.Pool
	keys    *accounts.Keyring
	profile string
	options QueryWorkerOptions
	core    core.Service
	jobs    *river.Client[pgx.Tx]
	stripe  *StripeRuntime
}

var _ river.Worker[paymentQueryArgs] = (*QueryWorker)(nil)

func NewQueryWorker(ctx context.Context, pool *pgxpool.Pool, keys *accounts.Keyring,
	profile string, options QueryWorkerOptions) (*QueryWorker, error) {
	if ctx == nil || keys == nil || !validQueryWorkerProfile(profile) || !validQueryWorkerOptions(options) ||
		(profile == "PROVIDER_MOCK") != (options.MockTransport != nil) {
		return nil, errPaymentQueryJob
	}
	if err := platform.ValidateWorkerPool(ctx, pool, WorkerAuthority(profile)); err != nil {
		return nil, errPaymentQueryDatabase
	}
	// Reconciliation jobs enter the same schema that payment consumers maintain.
	jobs, err := river.NewClient(riverpgxv5.New(pool), &river.Config{Schema: "river_payment"})
	if err != nil {
		return nil, errPaymentQueryDatabase
	}
	return &QueryWorker{pool: pool, keys: keys, profile: profile, options: options, jobs: jobs}, nil
}

func validQueryWorkerProfile(profile string) bool {
	return profile == "PROVIDER_MOCK" || profile == "SANDBOX" || profile == "LIVE"
}

func (w *QueryWorker) Timeout(*river.Job[paymentQueryArgs]) time.Duration {
	if w == nil {
		return 0
	}
	return time.Duration(w.options.LeaseSeconds) * time.Second
}

func (w *QueryWorker) NextRetry(*river.Job[paymentQueryArgs]) time.Time {
	if w == nil {
		return time.Time{}
	}
	return time.Now().UTC().Add(w.options.RetryDelay)
}

type queryOperation struct {
	ActorKind, Provider, Action, Purpose, State, ResultCode string
	Generation                                              int64
	LeaseUntil                                              *time.Time
}

func (w *QueryWorker) Work(ctx context.Context, job *river.Job[paymentQueryArgs]) error {
	if ctx == nil || w == nil || w.pool == nil || w.keys == nil || job == nil ||
		job.Args.Version != 1 || !command.ValidID(job.Args.OperationID) {
		return errPaymentQueryJob
	}
	id := job.Args.OperationID
	op, err := w.read(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		// T21-02: RLS shows this authority only payment-lane operations, so a job naming a foreign-family (or missing)
		// operation reads as no row: same outcome as the explicit family check below, nothing claimed, no provider call.
		return river.JobCancel(errPaymentQueryFamily)
	}
	if err != nil {
		return w.dbError(ctx)
	}
	if !validQueryOperation(op) {
		return river.JobCancel(errPaymentQueryFamily)
	}
	// A disabled Stripe worker leaves the job and operation untouched for an enabled worker.
	if w.stripe == nil {
		return river.JobSnooze(5 * time.Second)
	}
	return w.stripeStep(ctx, id, op)
}

func validQueryOperation(op queryOperation) bool {
	return op.ActorKind == "BUYER_PAYMENT_QUERY" && op.Purpose == "transactional" &&
		op.Provider == "stripe" && op.Action == "stripe.checkout_session"
}

func (w *QueryWorker) read(ctx context.Context, id string) (queryOperation, error) {
	bounded, cancel := context.WithTimeout(ctx, w.options.DBTimeout)
	defer cancel()
	var op queryOperation
	err := w.pool.QueryRow(bounded, `SELECT actor_kind,provider,action,purpose,state,generation,
		lease_until,result_code FROM integration.operations WHERE id=$1::uuid`, id).
		Scan(&op.ActorKind, &op.Provider, &op.Action, &op.Purpose, &op.State,
			&op.Generation, &op.LeaseUntil, &op.ResultCode)
	return op, err
}

func (w *QueryWorker) claim(ctx context.Context, id string) (core.ClaimResult, string, error) {
	bounded, cancel := context.WithTimeout(ctx, w.options.DBTimeout)
	defer cancel()
	tx, err := w.pool.BeginTx(bounded, pgx.TxOptions{})
	if err != nil {
		return core.ClaimResult{}, "", err
	}
	defer w.rollback(tx)
	claim, err := w.core.Claim(bounded, tx, id, w.options.LeaseSeconds)
	if err != nil {
		return core.ClaimResult{}, "", err
	}
	var providerReference string
	if err := tx.QueryRow(bounded, `SELECT provider_reference FROM integration.operations WHERE id=$1::uuid`, id).
		Scan(&providerReference); err != nil {
		return core.ClaimResult{}, "", err
	}
	if err := tx.Commit(bounded); err != nil {
		return core.ClaimResult{}, "", err
	}
	return claim, providerReference, nil
}

func (w *QueryWorker) dbError(ctx context.Context) error {
	if ctx.Err() != nil {
		return river.JobSnooze(w.options.RetryDelay)
	}
	return errPaymentQueryDatabase
}

func (w *QueryWorker) rollback(tx pgx.Tx) {
	cleanup, cancel := context.WithTimeout(context.Background(), w.options.DBTimeout)
	defer cancel()
	_ = tx.Rollback(cleanup)
}
