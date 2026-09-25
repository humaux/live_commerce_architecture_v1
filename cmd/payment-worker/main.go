package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"livecommerce/internal/integrations/accounts"
	"livecommerce/internal/payments"
	"livecommerce/internal/platform"
)

var (
	errWorkerConfig   = errors.New("payment_worker_invalid_config")
	errWorkerKeyring  = errors.New("payment_worker_invalid_keyring")
	errWorkerDatabase = errors.New("payment_worker_database")
	errWorkerStart    = errors.New("payment_worker_start_failed")
	errWorkerStop     = errors.New("payment_worker_stop_failed")
)

type workerConfig struct {
	enabled     bool
	dsn         string
	profile     string
	concurrency int
	keys        *accounts.Keyring
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
	switch getenv("COMMERCE_PAYMENT_WORKER_ENABLED") {
	case "", "0":
		return config, nil
	case "1":
		config.enabled = true
	default:
		return workerConfig{}, errWorkerConfig
	}
	config.dsn = getenv("COMMERCE_PAYMENT_WORKER_DATABASE_URL")
	config.profile = getenv("COMMERCE_PAYMENT_WORKER_PROFILE")
	if config.dsn == "" || (config.profile != "SANDBOX" && config.profile != "LIVE") {
		return workerConfig{}, errWorkerConfig
	}
	rawConcurrency := getenv("COMMERCE_PAYMENT_WORKER_CONCURRENCY")
	config.concurrency = 4
	if rawConcurrency != "" {
		n, err := strconv.Atoi(rawConcurrency)
		if err != nil || n < 1 || n > 16 || strconv.Itoa(n) != rawConcurrency {
			return workerConfig{}, errWorkerConfig
		}
		config.concurrency = n
	}
	var err error
	config.keys, err = accounts.LoadKeyring(getenv)
	if err != nil {
		return workerConfig{}, errWorkerKeyring
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
	client, err := payments.NewWorkerClient(ctx, pool, config.keys, config.profile,
		config.concurrency, payments.DefaultQueryWorkerOptions())
	if err != nil {
		return err
	}
	// River Start does a synchronous database probe. Cancel a stalled startup
	// on signal or deadline, then disarm that watchdog for normal operation.
	cancelWorker, err := startWorker(ctx, 10*time.Second, client.Start)
	if err != nil {
		return err
	}
	defer cancelWorker()
	<-ctx.Done()
	graceful, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	err = client.Stop(graceful)
	cancel()
	if err == nil {
		return nil
	}
	forced, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.StopAndCancel(forced); err != nil {
		return errWorkerStop
	}
	return nil
}

func startWorker(signalCtx context.Context, timeout time.Duration,
	start func(context.Context) error) (context.CancelFunc, error) {
	if signalCtx == nil || start == nil || timeout <= 0 || signalCtx.Err() != nil {
		return nil, errWorkerStart
	}
	workerCtx, cancelWorker := context.WithCancel(context.Background())
	done := make(chan struct{})
	watchStopped := make(chan struct{})
	timer := time.NewTimer(timeout)
	go func() {
		defer close(watchStopped)
		select {
		case <-signalCtx.Done():
			cancelWorker()
		case <-timer.C:
			cancelWorker()
		case <-done:
		}
	}()
	err := start(workerCtx)
	close(done)
	timer.Stop()
	<-watchStopped
	if err != nil || workerCtx.Err() != nil {
		cancelWorker()
		return nil, errWorkerStart
	}
	return cancelWorker, nil
}
