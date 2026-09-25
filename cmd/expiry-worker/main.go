package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"livecommerce/internal/checkout"
	"livecommerce/internal/jobqueue"
	"livecommerce/internal/platform"
)

var (
	errWorkerConfig   = errors.New("expiry_worker_invalid_config")
	errWorkerDatabase = errors.New("expiry_worker_database")
	errWorkerStart    = errors.New("expiry_worker_start_failed")
	errWorkerStop     = errors.New("expiry_worker_stop_failed")
)

type workerConfig struct {
	enabled     bool
	dsn         string
	concurrency int
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Getenv); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}

func loadConfig(getenv func(string) string) (workerConfig, error) {
	var config workerConfig
	if getenv == nil {
		return config, errWorkerConfig
	}
	switch getenv("COMMERCE_EXPIRY_WORKER_ENABLED") {
	case "", "0":
		return config, nil
	case "1":
		config.enabled = true
	default:
		return workerConfig{}, errWorkerConfig
	}
	config.dsn = getenv("COMMERCE_EXPIRY_WORKER_DATABASE_URL")
	if config.dsn == "" {
		return workerConfig{}, errWorkerConfig
	}
	config.concurrency = 4
	if raw := getenv("COMMERCE_EXPIRY_WORKER_CONCURRENCY"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 16 || strconv.Itoa(n) != raw {
			return workerConfig{}, errWorkerConfig
		}
		config.concurrency = n
	}
	return config, nil
}

func run(ctx context.Context, getenv func(string) string) error {
	config, err := loadConfig(getenv)
	if err != nil || !config.enabled {
		return err
	}
	pool, err := platform.OpenWorkerPool(ctx, config.dsn)
	if err != nil {
		return errWorkerDatabase
	}
	defer pool.Close()
	client, err := checkout.NewExpiryClient(ctx, pool, config.concurrency)
	if err != nil {
		return err
	}
	switch err := jobqueue.Run(ctx, client, "expiry_worker_ready"); {
	case errors.Is(err, jobqueue.ErrStart):
		return errWorkerStart
	case errors.Is(err, jobqueue.ErrStop):
		return errWorkerStop
	default:
		return err
	}
}
