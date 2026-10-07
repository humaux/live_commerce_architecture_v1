// Purpose: run expiry jobs and independently opted-in buyer/merchant SMTP loops.
// Depends on: checkout/jobqueue/notify/platform, worker DSN, COMMERCE_BUYER_MAIL_ENABLED and COMMERCE_MERCHANT_ALERT_MAIL.
// Used by: expiry-worker Compose process and cmd/expiry-worker configuration tests.
package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"sync"
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
	mail        *mailConfig // nil when neither mail loop is opted in
	buyerMail   bool
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
		config.buyerMail = true
	default:
		return workerConfig{}, errWorkerConfig
	}
	merchantMail := false
	switch getenv("COMMERCE_MERCHANT_ALERT_MAIL") {
	case "", "0":
	case "1":
		merchantMail = true
	default:
		return workerConfig{}, errWorkerConfig
	}
	if config.buyerMail || merchantMail {
		m, err := loadMailConfig(getenv, merchantMail)
		if err != nil {
			return workerConfig{}, err
		}
		config.mail = m
	}
	return config, nil
}

func run(ctx context.Context, getenv func(string) string) error {
	config, err := loadConfig(getenv)
	if err != nil || !config.enabled {
		return err
	}
	pool, err := platform.OpenWorkerPool(ctx, config.dsn, platform.WorkerExpiry)
	if err != nil {
		return errWorkerDatabase
	}
	defer pool.Close()
	client, err := checkout.NewExpiryClient(ctx, pool, config.concurrency)
	if err != nil {
		return err
	}
	// notify.Worker: the buyer / merchant mail outbox loop (mail.go), plus the meta connection-health owner mail (0125 §5.2) when
	// COMMERCE_MERCHANT_ALERT_MAIL is opted in. Both stop with River and finish before the pool closes.
	loopCtx, stopLoop := context.WithCancel(ctx)
	var loops sync.WaitGroup
	if config.mail != nil {
		if config.buyerMail {
			nw, err := notify.NewWorker(pool, config.mail.smtp, config.mail.dailyCap)
			if err != nil {
				stopLoop()
				return errWorkerConfig
			}
			loops.Add(1)
			go func() { defer loops.Done(); nw.Run(loopCtx) }()
		}
		if config.mail.adminOrigin != "" {
			mw, err := notify.NewMerchantWorker(pool, config.mail.smtp, config.mail.dailyCap, config.mail.adminOrigin)
			if err != nil {
				stopLoop()
				loops.Wait()
				return errWorkerConfig
			}
			loops.Add(1)
			go func() { defer loops.Done(); mw.Run(loopCtx) }()
		}
	}
	defer func() { stopLoop(); loops.Wait() }()
	switch err := jobqueue.Run(ctx, client, "expiry_worker_ready"); {
	case errors.Is(err, jobqueue.ErrStart):
		return errWorkerStart
	case errors.Is(err, jobqueue.ErrStop):
		return errWorkerStop
	default:
		return err
	}
}
