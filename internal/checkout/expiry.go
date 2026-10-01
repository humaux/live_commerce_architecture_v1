package checkout

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

const expiryTimeout = 5 * time.Second

var errInvalidExpiryJob = errors.New("invalid checkout expiry job")

type expiryArgs struct {
	OrderID    string `json:"order_id"`
	Generation int64  `json:"generation"`
	Version    int    `json:"version"`
}

func (expiryArgs) Kind() string { return "checkout_expiry_v1" }

// ExpiryWorker executes only the fixed database transition. Its pool and River
// client lifecycle remain with the owning process.
type ExpiryWorker struct {
	river.WorkerDefaults[expiryArgs]
	pool *pgxpool.Pool
}

var _ river.Worker[expiryArgs] = (*ExpiryWorker)(nil)

func NewExpiryWorker(ctx context.Context, workerPool *pgxpool.Pool) (*ExpiryWorker, error) {
	if ctx == nil {
		return nil, command.ErrInvalid
	}
	if err := platform.ValidateWorkerPool(ctx, workerPool, platform.WorkerExpiry); err != nil {
		return nil, err
	}
	return &ExpiryWorker{pool: workerPool}, nil
}

func (w *ExpiryWorker) Work(ctx context.Context, job *river.Job[expiryArgs]) error {
	if ctx == nil || w == nil || w.pool == nil || job == nil || !command.ValidID(job.Args.OrderID) ||
		job.Args.Generation < 1 || job.Args.Version != 1 {
		return errInvalidExpiryJob
	}
	bounded, cancel := context.WithTimeout(ctx, expiryTimeout)
	defer cancel()
	tx, err := w.pool.BeginTx(bounded, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return expiryError(bounded, err)
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 2*time.Second)
		defer stop()
		_ = tx.Rollback(cleanup)
	}()
	if _, err = tx.Exec(bounded, `SELECT set_config('statement_timeout','5s',true),
		set_config('lock_timeout','1s',true),set_config('idle_in_transaction_session_timeout','5s',true)`); err != nil {
		return expiryError(bounded, err)
	}
	var disposition string
	var retryAt pgtype.Timestamptz
	var dbNow time.Time
	err = tx.QueryRow(bounded, `SELECT e.disposition,e.retry_at,clock_timestamp()
		FROM checkout.expire_held($1::uuid,$2::bigint) e`, job.Args.OrderID, job.Args.Generation).
		Scan(&disposition, &retryAt, &dbNow)
	if err != nil {
		return expiryError(bounded, err)
	}
	switch disposition {
	case "EXPIRED", "STALE":
	case "NOT_DUE":
		if !retryAt.Valid {
			return errCheckoutDatabase
		}
	default:
		return errCheckoutDatabase
	}
	if err = tx.Commit(bounded); err != nil {
		return expiryError(bounded, err)
	}
	if disposition == "NOT_DUE" {
		delay := retryAt.Time.Sub(dbNow)
		if delay < time.Second {
			delay = time.Second
		}
		return river.JobSnooze(delay)
	}
	return nil
}

func expiryError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return errCheckoutDatabase
}
