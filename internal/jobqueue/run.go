package jobqueue

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/riverqueue/river"
)

var (
	ErrStart = errors.New("worker_start_failed")
	ErrStop  = errors.New("worker_stop_failed")
)

// Run starts one fixed-queue River client and waits for its owning command's
// signal context. The caller retains ownership of the database pool.
func Run(ctx context.Context, client *river.Client[pgx.Tx], readyMessage string) error {
	if ctx == nil || client == nil || readyMessage == "" {
		startDiagnostic("invalid_input", "other", "")
		return ErrStart
	}
	// River Start performs a synchronous database probe. The watchdog cancels a
	// blocked probe, then is joined and disarmed so normal work stays live.
	cancelWorker, err := startWorker(ctx, 10*time.Second, client.Start)
	if err != nil {
		return err
	}
	defer cancelWorker()
	slog.Info(readyMessage)
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
		return ErrStop
	}
	return nil
}

func startWorker(signalCtx context.Context, timeout time.Duration,
	start func(context.Context) error) (context.CancelFunc, error) {
	if signalCtx == nil || start == nil || timeout <= 0 {
		startDiagnostic("invalid_input", "other", "")
		return nil, ErrStart
	}
	if signalCtx.Err() != nil {
		startDiagnostic("signal", "cancelled", "")
		return nil, ErrStart
	}
	workerCtx, cancelWorker := context.WithCancel(context.Background())
	done := make(chan struct{})
	watchStopped := make(chan struct{})
	cancelReason := make(chan string, 1)
	timer := time.NewTimer(timeout)
	go func() {
		defer close(watchStopped)
		select {
		case <-signalCtx.Done():
			cancelReason <- "signal"
			cancelWorker()
		case <-timer.C:
			cancelReason <- "watchdog"
			cancelWorker()
		case <-done:
		}
	}()
	err := start(workerCtx)
	close(done)
	timer.Stop()
	<-watchStopped
	if err != nil || workerCtx.Err() != nil {
		phase := "river_start_returned"
		category, sqlstate := startErrorCategory(err)
		if workerCtx.Err() != nil {
			phase = <-cancelReason
			category, sqlstate = "cancelled", ""
			if phase == "watchdog" {
				category = "deadline"
			}
		}
		startDiagnostic(phase, category, sqlstate)
		cancelWorker()
		return nil, ErrStart
	}
	return cancelWorker, nil
}

func startDiagnostic(phase, category, sqlstate string) {
	// ponytail: only fixed fields cross the log boundary; River's raw error can contain secrets.
	slog.Error("worker_start_diagnostic", "phase", phase, "category", category, "sqlstate", sqlstate)
}

func startErrorCategory(err error) (string, string) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr != nil {
		switch pgErr.Code {
		case "08001", "08006", "23505", "28P01", "3F000", "40001", "40P01", "42P01", "42501", "53300", "55P03", "57014", "57P01", "57P03", "XX000":
			return "sqlstate", pgErr.Code
		}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "deadline", ""
	}
	if errors.Is(err, context.Canceled) {
		return "cancelled", ""
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return "network", ""
	}
	return "other", ""
}
