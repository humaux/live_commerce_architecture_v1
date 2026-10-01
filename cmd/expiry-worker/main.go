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
	"livecommerce/internal/notify"
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
	mail        *mailConfig // nil = no buyer mail loop (COMMERCE_BUYER_MAIL_ENABLED unset or 0)
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
	switch getenv("COMMERCE_BUYER_MAIL_ENABLED") {
	case "", "0":
	case "1":
		m, err := loadMailConfig(getenv)
		if err != nil {
			return workerConfig{}, err
		}
		config.mail = m
	default:
		return workerConfig{}, errWorkerConfig
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
	// notify.Worker: the buyer / merchant mail outbox loop (mail.go). It stops with the River client and is waited for, so a record in flight
	// is written before the pool closes.
	loopCtx, stopLoop := context.WithCancel(ctx)
	loopDone := make(chan struct{})
	close(loopDone)
	if config.mail != nil {
		nw, err := notify.NewWorker(pool, config.mail.smtp, config.mail.dailyCap)
		if err != nil {
			stopLoop()
			return errWorkerConfig
		}
		loopDone = make(chan struct{})
		go func() { defer close(loopDone); nw.Run(loopCtx) }()
	}
	defer func() { stopLoop(); <-loopDone }()
	switch err := jobqueue.Run(ctx, client, "expiry_worker_ready"); {
	case errors.Is(err, jobqueue.ErrStart):
		return errWorkerStart
	case errors.Is(err, jobqueue.ErrStop):
		return errWorkerStop
	default:
		return err
	}
}
