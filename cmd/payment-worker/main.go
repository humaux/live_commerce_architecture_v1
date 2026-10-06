// Purpose: cmd/payment-worker process: River payment queue worker (PAYUNi query/reconcile, Stripe) plus, on the SANDBOX profile,
//   the PAYUNi activation issuer sweep.
// Depends on: internal/payments (workers, RunVerificationSweeper), internal/platform (worker pool authority), internal/jobqueue,
//   internal/integrations/accounts (keyring); env COMMERCE_PAYMENT_WORKER_*, COMMERCE_STRIPE_*; SQL payments.sweep_payuni_verifications (0137).
// Used by: deploy compose/systemd unit of the payment worker; main_test.go.
// Invariants: a worker never serves HTTP; the LIVE worker never issues SANDBOX/MOCK qualifications.

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

	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/integrations/accounts"
	"livecommerce/internal/integrations/psp/stripe"
	"livecommerce/internal/jobqueue"
	"livecommerce/internal/payments"
	"livecommerce/internal/platform"
)

var (
	errWorkerConfig   = errors.New("payment_worker_invalid_config")
	errWorkerKeyring  = errors.New("payment_worker_invalid_keyring")
	errWorkerDatabase = errors.New("payment_worker_database")
	errWorkerStart    = errors.New("payment_worker_start_failed")
	errWorkerStop     = errors.New("payment_worker_stop_failed")
	errWorkerStripe   = errors.New("payment_worker_stripe_unavailable")
)

type workerConfig struct {
	enabled     bool
	dsn         string
	profile     string
	concurrency int
	keys        *accounts.Keyring
	// stripe adds Stripe dispatch to this profile's queue (COMMERCE_STRIPE_ENABLED=1). The worker
	// never reads STRIPE_* or whsec variables: API keys come per attempt from PG, sealed under keys.
	stripe bool
	// live is the owner's COMMERCE_STRIPE_LIVE_ENABLED + _APPROVAL_REF pair; complete only for stripe on
	// profile LIVE (stripe-live-enable-v1 §5.2), zero otherwise.
	live stripe.LiveApproval
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
	// Read only here, i.e. only when the worker itself is enabled (contracts/stripe-psp-v1.md §12).
	switch getenv("COMMERCE_STRIPE_ENABLED") {
	case "", "0":
	case "1":
		// LIVE Stripe needs the owner's flag+reference pair (both or neither, §12; stripe-live-enable-v1 §5.2).
		// The pair is read only for LIVE, so a SANDBOX worker reads exactly the names it always did.
		if config.profile == "LIVE" {
			live, err := platform.LoadStripeLiveApproval(getenv)
			if err != nil || !live.Valid() {
				return workerConfig{}, errWorkerConfig
			}
			config.live = live
		}
		config.stripe = true
	default:
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

// startVerificationSweeper runs the PAYUNi activation issuer loop (w4-02b, payments.sweep_payuni_verifications) on the SANDBOX
// worker only: commerce_payment_live holds no EXECUTE on it and the LIVE worker must never issue SANDBOX/MOCK proof. It makes no
// provider call; it reports whether it started.
func startVerificationSweeper(ctx context.Context, pool *pgxpool.Pool, profile string) bool {
	if profile != "SANDBOX" {
		return false
	}
	go payments.RunVerificationSweeper(ctx, pool, 5*time.Second)
	return true
}

func run(ctx context.Context, getenv func(string) string) error {
	config, err := loadConfig(getenv)
	if err != nil || !config.enabled {
		return err
	}
	pool, err := platform.OpenWorkerPool(ctx, config.dsn, payments.WorkerAuthority(config.profile))
	if err != nil {
		return errWorkerDatabase
	}
	defer pool.Close()
	var runtime *payments.StripeRuntime
	if config.stripe {
		// Checks the worker role, queue and scoped SQL capabilities; no provider call at startup.
		if config.profile == "LIVE" {
			// The worker keeps running for a store whose method is disabled or revoked: stopping it would strand
			// PAYMENT_PENDING stock and UNKNOWN refunds (LD6), so the kill switch never lives here.
			runtime, err = payments.NewLiveStripeRuntime(ctx, pool, config.keys, config.live)
		} else {
			runtime, err = payments.NewStripeRuntime(ctx, pool, config.keys, config.profile)
		}
		if err != nil {
			return errWorkerStripe
		}
	}
	client, err := payments.NewPaymentWorkerClient(ctx, pool, payments.WorkerConfig{
		Profile: config.profile, Concurrency: config.concurrency,
		Query: payments.DefaultQueryWorkerOptions(), Keys: config.keys, Stripe: runtime})
	if err != nil {
		return err
	}
	startVerificationSweeper(ctx, pool, config.profile)
	// Fixed local startup witness; never implies provider access or payment.
	switch err := jobqueue.Run(ctx, client, "payment_worker_ready"); {
	case errors.Is(err, jobqueue.ErrStart):
		return errWorkerStart
	case errors.Is(err, jobqueue.ErrStop):
		return errWorkerStop
	default:
		return err
	}
}
