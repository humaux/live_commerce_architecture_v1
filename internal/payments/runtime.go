package payments

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

	"livecommerce/internal/integrations/accounts"
	"livecommerce/internal/jobqueue"
	"livecommerce/internal/platform"
)

var (
	errPaymentWorkerConfig       = errors.New("payment_worker_invalid_config")
	errPaymentWorkerDatabase     = errors.New("payment_worker_database")
	errPaymentWorkerQueueUnready = errors.New("payment_worker_queue_unready")
)

// NewWorkerClient assembles only the query and capture workers on the queue
// owned by profile. The caller owns both the returned client and pool.
func NewWorkerClient(ctx context.Context, pool *pgxpool.Pool, keys *accounts.Keyring,
	profile string, concurrency int, queryOptions QueryWorkerOptions) (*river.Client[pgx.Tx], error) {
	queue := jobqueue.ForProfile(profile)
	if ctx == nil || pool == nil || keys == nil || queue == "" || concurrency < 1 || concurrency > 16 ||
		!validQueryWorkerOptions(queryOptions) || (profile == "PROVIDER_MOCK") != (queryOptions.MockTransport != nil) {
		return nil, errPaymentWorkerConfig
	}
	if err := platform.ValidateWorkerPool(ctx, pool); err != nil {
		return nil, errPaymentWorkerDatabase
	}
	// The privileged SQL predicate checks the installed deferred router and all
	// active payment and reserved-queue rows. Nothing is fetched on false/error.
	preflight, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var ready bool
	if err := pool.QueryRow(preflight, `SELECT integration.payment_queue_ready()`).Scan(&ready); err != nil || !ready {
		return nil, errPaymentWorkerQueueUnready
	}
	query, err := NewQueryWorker(ctx, pool, keys, profile, queryOptions)
	if err != nil {
		return nil, errPaymentWorkerDatabase
	}
	capture, err := NewCaptureWorker(ctx, pool)
	if err != nil {
		return nil, errPaymentWorkerDatabase
	}
	workers := river.NewWorkers()
	river.AddWorker(workers, query)
	river.AddWorker(workers, capture)
	// River can include job errors in logs; keep the separate process silent
	// until diagnostics have an explicit redaction contract. The payment schema
	// confines leader maintenance; the profile queue still limits fetch.
	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Schema: "river_payment", Workers: workers,
		Queues: map[string]river.QueueConfig{queue: {MaxWorkers: concurrency}},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		return nil, errPaymentWorkerDatabase
	}
	return client, nil
}
