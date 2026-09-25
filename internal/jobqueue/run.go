package jobqueue

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
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
	if signalCtx == nil || start == nil || timeout <= 0 || signalCtx.Err() != nil {
		return nil, ErrStart
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
		return nil, ErrStart
	}
	return cancelWorker, nil
}
