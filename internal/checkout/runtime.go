package checkout

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"livecommerce/internal/jobqueue"
	"livecommerce/internal/platform"
)

var (
	errExpiryWorkerConfig       = errors.New("expiry_worker_invalid_config")
	errExpiryWorkerDatabase     = errors.New("expiry_worker_database")
	errExpiryWorkerQueueUnready = errors.New("expiry_worker_queue_unready")
)

// NewExpiryClient consumes only the fixed checkout expiry queue. The caller owns
// the client and pool; failed admission never starts a consumer.
func NewExpiryClient(ctx context.Context, pool *pgxpool.Pool, concurrency int) (*river.Client[pgx.Tx], error) {
	if ctx == nil || pool == nil || concurrency < 1 || concurrency > 16 {
		return nil, errExpiryWorkerConfig
	}
	if err := platform.ValidateWorkerPool(ctx, pool); err != nil {
		return nil, errExpiryWorkerDatabase
	}
	preflight, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var ready bool
	if err := pool.QueryRow(preflight, `SELECT checkout.expiry_queue_ready()`).Scan(&ready); err != nil || !ready {
		return nil, errExpiryWorkerQueueUnready
	}
	worker, err := NewExpiryWorker(ctx, pool)
	if err != nil {
		return nil, errExpiryWorkerDatabase
	}
	workers := river.NewWorkers()
	river.AddWorker(workers, worker)
	// Queue limits fetch; the schema also confines River leader maintenance.
	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Schema: "river_expiry", Workers: workers,
		Queues: map[string]river.QueueConfig{jobqueue.CheckoutExpiry: {MaxWorkers: concurrency}},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		return nil, errExpiryWorkerDatabase
	}
	return client, nil
}
