package payments

import (
	"context"
	"encoding/hex"
	"errors"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

const captureTimeout = 5 * time.Second

var (
	errCaptureJob      = errors.New("payment_capture_invalid_job")
	errCaptureDatabase = errors.New("payment_capture_database")
	captureHashPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

type paymentReconcileArgs struct {
	OperationID string `json:"operation_id"`
	ReportHash  string `json:"report_hash"`
	Version     int    `json:"version"`
}

func (paymentReconcileArgs) Kind() string { return "payment_reconcile_v1" }

func validCaptureArgs(args paymentReconcileArgs) bool {
	return args.Version == 1 && command.ValidID(args.OperationID) &&
		captureHashPattern.MatchString(args.ReportHash)
}

func captureApplyError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && (pgErr.Code == "22023" || pgErr.Code == "PT409") {
		return river.JobCancel(errCaptureJob)
	}
	return errCaptureDatabase
}

// CaptureWorker applies only a previously persisted authenticated query report.
// The SQL function owns financial, stock and review decisions in one transaction.
type CaptureWorker struct {
	river.WorkerDefaults[paymentReconcileArgs]
	pool *pgxpool.Pool
}

var _ river.Worker[paymentReconcileArgs] = (*CaptureWorker)(nil)

func NewCaptureWorker(ctx context.Context, pool *pgxpool.Pool) (*CaptureWorker, error) {
	if ctx == nil {
		return nil, errCaptureJob
	}
	if err := platform.ValidateWorkerPool(ctx, pool, platform.WorkerPayment, platform.WorkerPaymentLive); err != nil {
		return nil, errCaptureDatabase
	}
	return &CaptureWorker{pool: pool}, nil
}

func (*CaptureWorker) Timeout(*river.Job[paymentReconcileArgs]) time.Duration {
	return captureTimeout
}

func (w *CaptureWorker) Work(ctx context.Context, job *river.Job[paymentReconcileArgs]) error {
	if job == nil || !validCaptureArgs(job.Args) {
		return river.JobCancel(errCaptureJob)
	}
	if ctx == nil || w == nil || w.pool == nil {
		return errCaptureDatabase
	}
	reportHash, _ := hex.DecodeString(job.Args.ReportHash)
	bounded, cancel := context.WithTimeout(ctx, captureTimeout)
	defer cancel()
	tx, err := w.pool.BeginTx(bounded, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return errCaptureDatabase
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 2*time.Second)
		defer stop()
		_ = tx.Rollback(cleanup)
	}()
	if _, err = tx.Exec(bounded, `SELECT set_config('statement_timeout','5s',true),
		set_config('lock_timeout','1s',true),set_config('idle_in_transaction_session_timeout','5s',true)`); err != nil {
		return errCaptureDatabase
	}
	if _, err = tx.Exec(bounded, `SELECT payments.apply_capture($1::uuid,$2::bytea)`,
		job.Args.OperationID, reportHash); err != nil {
		return captureApplyError(err)
	}
	if err = tx.Commit(bounded); err != nil {
		return errCaptureDatabase
	}
	return nil
}
