package jobqueue

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWorkerStartupWatchdog(t *testing.T) {
	t.Run("startup deadline cancels blocking Start", func(t *testing.T) {
		entered := make(chan struct{})
		returned := make(chan struct{})
		cancel, err := startWorker(context.Background(), 20*time.Millisecond, func(ctx context.Context) error {
			close(entered)
			<-ctx.Done()
			close(returned)
			return ctx.Err()
		})
		if !errors.Is(err, ErrStart) || cancel != nil {
			t.Fatalf("stalled startup was accepted: %v", err)
		}
		<-entered
		<-returned
	})
	t.Run("signal cancels blocking Start", func(t *testing.T) {
		signalCtx, signalCancel := context.WithCancel(context.Background())
		defer signalCancel()
		entered := make(chan struct{})
		go func() {
			<-entered
			signalCancel()
		}()
		cancel, err := startWorker(signalCtx, time.Hour, func(ctx context.Context) error {
			close(entered)
			<-ctx.Done()
			return ctx.Err()
		})
		if !errors.Is(err, ErrStart) || cancel != nil {
			t.Fatalf("signalled startup was accepted: %v", err)
		}
	})
	t.Run("successful Start keeps lifetime beyond deadline", func(t *testing.T) {
		var workerCtx context.Context
		cancel, err := startWorker(context.Background(), 20*time.Millisecond, func(ctx context.Context) error {
			workerCtx = ctx
			return nil
		})
		if err != nil || cancel == nil {
			t.Fatalf("successful startup rejected: %v", err)
		}
		time.Sleep(40 * time.Millisecond)
		if workerCtx.Err() != nil {
			t.Fatal("startup watchdog cancelled healthy worker")
		}
		cancel()
		if workerCtx.Err() != context.Canceled {
			t.Fatal("worker lifetime cancellation did not work")
		}
	})
}
