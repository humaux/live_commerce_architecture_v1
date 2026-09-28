// stripe_signal.go owns River's Stripe signal kind on the existing payment queue.
// It never treats an unauthenticated job argument as a payment or stock decision.
package payments

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"livecommerce/internal/command"
	"livecommerce/internal/integrations/core"
	"livecommerce/internal/integrations/psp/stripe"
	"livecommerce/internal/jobqueue"
	"livecommerce/internal/platform"
)

var (
	errStripeSignalJob      = errors.New("stripe_signal_invalid_job")
	errStripeSignalFamily   = errors.New("stripe_signal_wrong_family")
	errStripeSignalDatabase = errors.New("stripe_signal_database")
)

type paymentSignalArgs struct {
	OperationID string `json:"operation_id"`
	SignalID    string `json:"signal_id"`
	Version     int    `json:"version"`
}

func (paymentSignalArgs) Kind() string { return "payment_signal_v1" }

func validStripeSignalArgs(a paymentSignalArgs) bool {
	return a.Version == 1 && command.ValidID(a.OperationID) && command.ValidID(a.SignalID)
}

type SignalWorker struct {
	river.WorkerDefaults[paymentSignalArgs]
	pool    *pgxpool.Pool
	stripe  *StripeRuntime
	profile string
	options QueryWorkerOptions
	core    core.Service
	jobs    *river.Client[pgx.Tx]
}

var _ river.Worker[paymentSignalArgs] = (*SignalWorker)(nil)

func NewSignalWorker(ctx context.Context, pool *pgxpool.Pool, s *StripeRuntime,
	profile string, o QueryWorkerOptions) (*SignalWorker, error) {
	if ctx == nil || pool == nil || !validQueryWorkerProfile(profile) || !validQueryWorkerOptions(o) ||
		(s != nil && (s.pool != pool || s.profile != profile)) {
		return nil, errStripeSignalJob
	}
	if err := platform.ValidateWorkerPool(ctx, pool); err != nil {
		return nil, errStripeSignalDatabase
	}
	jobs, err := river.NewClient(riverpgxv5.New(pool), &river.Config{Schema: "river_payment"})
	if err != nil {
		return nil, errStripeSignalDatabase
	}
	return &SignalWorker{pool: pool, stripe: s, profile: profile, options: o, jobs: jobs}, nil
}

func (w *SignalWorker) Timeout(*river.Job[paymentSignalArgs]) time.Duration {
	if w == nil {
		return 0
	}
	return time.Duration(w.options.LeaseSeconds) * time.Second
}

func (w *SignalWorker) Work(ctx context.Context, job *river.Job[paymentSignalArgs]) (result error) {
	if job == nil || !validStripeSignalArgs(job.Args) {
		return river.JobCancel(errStripeSignalJob)
	}
	if ctx == nil || w == nil || w.pool == nil {
		return errStripeSignalDatabase
	}
	bounded, cancel := context.WithTimeout(ctx, w.options.DBTimeout)
	defer cancel()
	var op queryOperation
	// The operation family is checked before a disabled runtime can snooze the job.
	if err := w.pool.QueryRow(bounded, `SELECT actor_kind,provider,action,purpose,state
		FROM integration.operations WHERE id=$1::uuid`, job.Args.OperationID).
		Scan(&op.ActorKind, &op.Provider, &op.Action, &op.Purpose, &op.State); err != nil {
		return errStripeSignalDatabase
	}
	if !validQueryOperation(op) || op.Provider != "stripe" {
		return river.JobCancel(errStripeSignalFamily)
	}
	if w.stripe == nil {
		return river.JobSnooze(5 * time.Second)
	}
	claim, err := w.claimSignal(ctx, job.Args.OperationID)
	if err != nil {
		return errStripeSignalDatabase
	}
	switch claim.Disposition {
	case "busy":
		return river.JobSnooze(2 * time.Second)
	case "terminal":
		return nil
	case "blocked_binding":
		return river.JobSnooze(w.options.RetryDelay)
	case "claimed":
		if claim.Mode != "reconcile" {
			return errStripeSignalDatabase
		}
	default:
		return errStripeSignalDatabase
	}
	defer func() {
		if recover() != nil {
			if ctx.Err() != nil {
				result = river.JobSnooze(5 * time.Second)
			} else {
				result = w.finishSignal(ctx, job.Args.OperationID, claim, "stripe_panic", 5*time.Second)
			}
		}
	}()
	callCtx, stop := context.WithTimeout(ctx, w.options.CallTimeout)
	defer stop()
	sig, err := w.loadSignal(callCtx, job.Args, job.ID, claim, nil)
	if ctx.Err() != nil {
		return river.JobSnooze(5 * time.Second)
	}
	if callCtx.Err() != nil {
		return w.finishSignal(ctx, job.Args.OperationID, claim, "stripe_timeout", 5*time.Second)
	}
	if err != nil {
		return w.finishSignal(ctx, job.Args.OperationID, claim, "stripe_unavailable", 5*time.Second)
	}
	if sig.ConsumedAt != nil {
		return w.consumeSignal(ctx, job.Args, job.ID, claim, sig, "", "stripe_signal_consumed")
	}
	if sig.DBNow.Sub(sig.CreatedAt) > 10*time.Minute {
		return w.consumeSignal(ctx, job.Args, job.ID, claim, sig, "STALE_DROPPED", "stripe_signal_stale")
	}
	snapshot, err := w.stripe.loadSession(callCtx, job.Args.OperationID, claim, w.options.DBTimeout)
	if ctx.Err() != nil {
		return river.JobSnooze(5 * time.Second)
	}
	if callCtx.Err() != nil {
		return w.finishSignal(ctx, job.Args.OperationID, claim, "stripe_timeout", 5*time.Second)
	}
	if err != nil {
		return w.finishSignal(ctx, job.Args.OperationID, claim, "stripe_unavailable", 5*time.Second)
	}
	target := snapshot.SessionID
	if sig.SessionID != "" {
		target = sig.SessionID
	}
	if (snapshot.Captured || snapshot.ClosedUnpaid) && sig.SessionID == "" {
		return w.consumeSignal(ctx, job.Args, job.ID, claim, sig, "NOOP_TERMINAL", "stripe_terminal_observed")
	}
	if target == "" {
		return w.finishSignal(ctx, job.Args.OperationID, claim, "stripe_unavailable", 5*time.Second)
	}
	client, err := w.stripe.clientForClaim(callCtx, snapshot, claim, w.options.DBTimeout)
	if ctx.Err() != nil {
		return river.JobSnooze(5 * time.Second)
	}
	if callCtx.Err() != nil {
		return w.finishSignal(ctx, job.Args.OperationID, claim, "stripe_timeout", 5*time.Second)
	}
	if err != nil {
		return w.finishSignal(ctx, job.Args.OperationID, claim, "stripe_unavailable", 5*time.Second)
	}
	session, meta, err := client.RetrieveCheckoutSession(callCtx, target)
	if err != nil {
		return w.finishSignal(ctx, job.Args.OperationID, claim, "stripe_retrieve_failed", 15*time.Second)
	}
	if !stripeSessionIdentity(session, snapshot) {
		return w.finishSignal(ctx, job.Args.OperationID, claim, "stripe_session_mismatch", 2*time.Minute)
	}
	via, outcome := "retrieve", "OBSERVED"
	if sig.Source == "BUYER_CANCEL" && session.Status == "open" {
		if err := w.noteSignalExpire(callCtx, job.Args.OperationID, claim); err != nil {
			return w.finishSignal(ctx, job.Args.OperationID, claim, "stripe_retrieve_failed", 15*time.Second)
		}
		_, _, _ = client.ExpireCheckoutSession(callCtx, target,
			stripe.ExpireIdempotencyKey(snapshot.AttemptID, claim.Generation))
		session, meta, err = client.RetrieveCheckoutSession(callCtx, target)
		if err != nil {
			return w.finishSignal(ctx, job.Args.OperationID, claim, "stripe_retrieve_failed", 15*time.Second)
		}
		if !stripeSessionIdentity(session, snapshot) {
			return w.finishSignal(ctx, job.Args.OperationID, claim, "stripe_session_mismatch", 2*time.Minute)
		}
		via, outcome = "expire", "EXPIRE_REQUESTED"
	}
	if ctx.Err() != nil {
		return river.JobSnooze(5 * time.Second)
	}
	if callCtx.Err() != nil {
		return w.finishSignal(ctx, job.Args.OperationID, claim, "stripe_timeout", 5*time.Second)
	}
	report := session.Observation(via, meta, snapshot.AccountID, snapshot.CredentialVersion, 0)
	if err := w.recordSignal(ctx, job.Args, job.ID, claim, sig, report, session.URL(), outcome); err != nil {
		return w.finishSignal(ctx, job.Args.OperationID, claim, "stripe_record_failed", 5*time.Second)
	}
	return nil
}

type stripeSignalSnapshot struct {
	Source, SessionID string
	CreatedAt, DBNow  time.Time
	ConsumedAt        *time.Time
}

func (w *SignalWorker) claimSignal(ctx context.Context, id string) (core.ClaimResult, error) {
	bounded, cancel := context.WithTimeout(ctx, w.options.DBTimeout)
	defer cancel()
	tx, err := w.pool.BeginTx(bounded, pgx.TxOptions{})
	if err != nil {
		return core.ClaimResult{}, err
	}
	defer w.rollbackSignal(tx)
	claim, err := w.core.Claim(bounded, tx, id, w.options.LeaseSeconds)
	if err != nil {
		return core.ClaimResult{}, err
	}
	return claim, tx.Commit(bounded)
}

func (w *SignalWorker) loadSignal(ctx context.Context, args paymentSignalArgs, jobID int64,
	claim core.ClaimResult, tx pgx.Tx) (stripeSignalSnapshot, error) {
	bounded, cancel := context.WithTimeout(ctx, w.options.DBTimeout)
	defer cancel()
	var out stripeSignalSnapshot
	query := `SELECT source,session_id,created_at,consumed_at,db_now FROM integration.load_stripe_signal(
		$1::uuid,$2::bigint,$3::bytea,$4::text,$5::bigint,$6::uuid)`
	var sessionID *string
	var err error
	if tx == nil {
		err = w.pool.QueryRow(bounded, query, args.OperationID, claim.Generation, claim.LeaseToken,
			w.profile, jobID, args.SignalID).Scan(&out.Source, &sessionID, &out.CreatedAt, &out.ConsumedAt, &out.DBNow)
	} else {
		err = tx.QueryRow(bounded, query, args.OperationID, claim.Generation, claim.LeaseToken,
			w.profile, jobID, args.SignalID).Scan(&out.Source, &sessionID, &out.CreatedAt, &out.ConsumedAt, &out.DBNow)
	}
	if sessionID != nil {
		out.SessionID = *sessionID
	}
	return out, err
}

func (w *SignalWorker) consumeSignal(ctx context.Context, args paymentSignalArgs, jobID int64,
	claim core.ClaimResult, initial stripeSignalSnapshot, outcome, code string) error {
	bounded, cancel := context.WithTimeout(ctx, w.options.DBTimeout)
	defer cancel()
	tx, err := w.pool.BeginTx(bounded, pgx.TxOptions{})
	if err != nil {
		return errStripeSignalDatabase
	}
	defer w.rollbackSignal(tx)
	current, err := w.loadSignal(bounded, args, jobID, claim, tx)
	if err != nil || (current.ConsumedAt == nil) != (initial.ConsumedAt == nil) ||
		current.Source != initial.Source || current.SessionID != initial.SessionID {
		return errStripeSignalDatabase
	}
	if outcome != "" {
		if current.ConsumedAt != nil {
			return errStripeSignalDatabase
		}
		if _, err = tx.Exec(bounded, `SELECT integration.consume_stripe_signal($1::uuid,$2::uuid,$3::bigint,$4::bytea,$5::text,$6::text)`,
			args.SignalID, args.OperationID, claim.Generation, claim.LeaseToken, w.profile, outcome); err != nil {
			return errStripeSignalDatabase
		}
	}
	var providerReference string
	if err = tx.QueryRow(bounded, `SELECT provider_reference FROM integration.operations WHERE id=$1::uuid`,
		args.OperationID).Scan(&providerReference); err != nil {
		return errStripeSignalDatabase
	}
	if err = w.core.Complete(bounded, tx, args.OperationID, claim.Generation, claim.LeaseToken,
		core.Outcome{State: "UNKNOWN", Code: code, ProviderReference: providerReference}); err != nil {
		return errStripeSignalDatabase
	}
	if err = tx.Commit(bounded); err != nil {
		return errStripeSignalDatabase
	}
	return nil
}

func (w *SignalWorker) recordSignal(ctx context.Context, args paymentSignalArgs, jobID int64,
	claim core.ClaimResult, initial stripeSignalSnapshot, report stripe.Observation, url, outcome string) error {
	encoded, err := json.Marshal(report)
	if err != nil {
		return errStripeSignalDatabase
	}
	bounded, cancel := context.WithTimeout(ctx, w.options.DBTimeout)
	defer cancel()
	tx, err := w.pool.BeginTx(bounded, pgx.TxOptions{})
	if err != nil {
		return errStripeSignalDatabase
	}
	defer w.rollbackSignal(tx)
	current, err := w.loadSignal(bounded, args, jobID, claim, tx)
	if err != nil || current.ConsumedAt != nil || current.Source != initial.Source || current.SessionID != initial.SessionID {
		return errStripeSignalDatabase
	}
	var hash string
	if err = tx.QueryRow(bounded, `SELECT encode(sha256(convert_to($1::jsonb::text,'UTF8')),'hex')`, encoded).Scan(&hash); err != nil {
		return errStripeSignalDatabase
	}
	job, err := w.jobs.InsertTx(bounded, tx, paymentReconcileArgs{OperationID: args.OperationID,
		ReportHash: hash, Version: 1}, &river.InsertOpts{Queue: jobqueue.ForProfile(w.profile)})
	if err != nil {
		return errStripeSignalDatabase
	}
	if _, err = tx.Exec(bounded, `SELECT integration.consume_stripe_signal($1::uuid,$2::uuid,$3::bigint,$4::bytea,$5::text,$6::text)`,
		args.SignalID, args.OperationID, claim.Generation, claim.LeaseToken, w.profile, outcome); err != nil {
		return errStripeSignalDatabase
	}
	var sessionURL any
	if url != "" {
		sessionURL = url
	}
	if _, err = tx.Exec(bounded, `SELECT integration.record_stripe_observation($1::uuid,$2::bigint,$3::bytea,$4::text,$5::jsonb,$6::bigint,$7::text)`,
		args.OperationID, claim.Generation, claim.LeaseToken, w.profile, encoded, job.Job.ID, sessionURL); err != nil {
		return errStripeSignalDatabase
	}
	if err = tx.Commit(bounded); err != nil {
		return errStripeSignalDatabase
	}
	return nil
}

func (w *SignalWorker) noteSignalExpire(ctx context.Context, id string, claim core.ClaimResult) error {
	bounded, cancel := context.WithTimeout(ctx, w.options.DBTimeout)
	defer cancel()
	_, err := w.pool.Exec(bounded, `SELECT integration.note_stripe_expire($1::uuid,$2::bigint,$3::bytea,$4::text)`,
		id, claim.Generation, claim.LeaseToken, w.profile)
	return err
}

func (w *SignalWorker) finishSignal(ctx context.Context, id string, claim core.ClaimResult,
	code string, delay time.Duration) error {
	if ctx.Err() != nil {
		return river.JobSnooze(5 * time.Second)
	}
	bounded, cancel := context.WithTimeout(ctx, w.options.DBTimeout)
	defer cancel()
	_, err := w.pool.Exec(bounded, `SELECT integration.finish_stripe_query($1::uuid,$2::bigint,$3::bytea,$4::text,$5::text)`,
		id, claim.Generation, claim.LeaseToken, w.profile, code)
	if err != nil {
		return errStripeSignalDatabase
	}
	if delay > 0 {
		return river.JobSnooze(delay)
	}
	return nil
}

func (w *SignalWorker) rollbackSignal(tx pgx.Tx) {
	cleanup, cancel := context.WithTimeout(context.Background(), w.options.DBTimeout)
	defer cancel()
	_ = tx.Rollback(cleanup)
}
