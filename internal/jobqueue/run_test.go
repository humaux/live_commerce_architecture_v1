package jobqueue

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func captureStartLogs(t *testing.T, run func()) []map[string]any {
	t.Helper()
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	defer slog.SetDefault(previous)
	run()
	if bytes.Contains(output.Bytes(), []byte("SECRET_MARKER_do_not_log")) {
		t.Fatal("secret marker leaked in raw diagnostic")
	}
	var logs []map[string]any
	for _, line := range bytes.Split(bytes.TrimSpace(output.Bytes()), []byte{'\n'}) {
		if len(line) == 0 {
			continue
		}
		var fields map[string]any
		if err := json.Unmarshal(line, &fields); err != nil {
			t.Fatalf("invalid structured diagnostic: %v", err)
		}
		logs = append(logs, fields)
	}
	return logs
}

func assertStartLog(t *testing.T, logs []map[string]any, phase, category, sqlstate string) {
	t.Helper()
	if len(logs) != 1 {
		t.Fatalf("want one start diagnostic, got %d", len(logs))
	}
	fields := logs[0]
	for key := range fields {
		switch key {
		case "time", "level", "msg", "phase", "category", "sqlstate":
		default:
			t.Fatalf("unexpected diagnostic field: %s", key)
		}
	}
	if fields["msg"] != "worker_start_diagnostic" || fields["phase"] != phase ||
		fields["category"] != category || fields["sqlstate"] != sqlstate {
		t.Fatalf("unexpected diagnostic values: %v", fields)
	}
}

func TestWorkerStartupWatchdog(t *testing.T) {
	t.Run("startup deadline cancels blocking Start", func(t *testing.T) {
		entered := make(chan struct{})
		returned := make(chan struct{})
		logs := captureStartLogs(t, func() {
			cancel, err := startWorker(context.Background(), 20*time.Millisecond, func(ctx context.Context) error {
				close(entered)
				<-ctx.Done()
				close(returned)
				return ctx.Err()
			})
			if !errors.Is(err, ErrStart) || cancel != nil {
				t.Fatalf("stalled startup was accepted: %v", err)
			}
		})
		assertStartLog(t, logs, "watchdog", "deadline", "")
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
		logs := captureStartLogs(t, func() {
			cancel, err := startWorker(signalCtx, time.Hour, func(ctx context.Context) error {
				close(entered)
				<-ctx.Done()
				return ctx.Err()
			})
			if !errors.Is(err, ErrStart) || cancel != nil {
				t.Fatalf("signalled startup was accepted: %v", err)
			}
		})
		assertStartLog(t, logs, "signal", "cancelled", "")
	})
	t.Run("successful Start keeps lifetime beyond deadline", func(t *testing.T) {
		var workerCtx context.Context
		logs := captureStartLogs(t, func() {
			cancel, err := startWorker(context.Background(), 20*time.Millisecond, func(ctx context.Context) error {
				workerCtx = ctx
				return nil
			})
			if err != nil || cancel == nil {
				t.Fatalf("successful startup rejected: %v", err)
			}
			defer cancel()
			time.Sleep(40 * time.Millisecond)
			if workerCtx.Err() != nil {
				t.Fatal("startup watchdog cancelled healthy worker")
			}
		})
		if len(logs) != 0 {
			t.Fatalf("healthy startup emitted diagnostic: %v", logs)
		}
		if workerCtx.Err() != context.Canceled {
			t.Fatal("worker lifetime cancellation did not work")
		}
	})
}

func TestWorkerStartupDiagnosticSanitization(t *testing.T) {
	const secret = "SECRET_MARKER_do_not_log"
	cases := []struct {
		name, phase, category, state string
		start                        func(context.Context) error
	}{
		{"native error", "river_start_returned", "other", "", func(context.Context) error { return errors.New(secret) }},
		{"wrapped PostgreSQL error", "river_start_returned", "sqlstate", "42501", func(context.Context) error {
			return fmt.Errorf("%s: %w", secret, &pgconn.PgError{Code: "42501", Message: secret, Detail: secret, Hint: secret})
		}},
		{"forged PostgreSQL state", "river_start_returned", "other", "", func(context.Context) error {
			return &pgconn.PgError{Code: "42501_" + secret, Message: secret}
		}},
		{"unknown PostgreSQL state", "river_start_returned", "other", "", func(context.Context) error {
			return &pgconn.PgError{Code: "ZZZZZ", Message: secret}
		}},
		{"nil PostgreSQL error", "river_start_returned", "other", "", func(context.Context) error {
			var pgErr *pgconn.PgError
			return pgErr
		}},
		{"deadline", "river_start_returned", "deadline", "", func(context.Context) error {
			return fmt.Errorf("%s: %w", secret, context.DeadlineExceeded)
		}},
		{"cancelled", "river_start_returned", "cancelled", "", func(context.Context) error {
			return fmt.Errorf("%s: %w", secret, context.Canceled)
		}},
		{"network", "river_start_returned", "network", "", func(context.Context) error {
			return &net.DNSError{Err: secret, IsTimeout: true}
		}},
	}
	for _, state := range []string{"08001", "08006", "23505", "28P01", "3F000", "40001", "40P01", "42P01", "53300", "55P03", "57014", "57P01", "57P03", "XX000"} {
		cases = append(cases, struct {
			name, phase, category, state string
			start                        func(context.Context) error
		}{"allowlisted " + state, "river_start_returned", "sqlstate", state, func(context.Context) error {
			return &pgconn.PgError{Code: state, Message: secret}
		}})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			logs := captureStartLogs(t, func() {
				cancel, err := startWorker(context.Background(), time.Second, tc.start)
				if cancel != nil || !errors.Is(err, ErrStart) || err != ErrStart {
					t.Fatalf("start error contract changed: %v", err)
				}
			})
			assertStartLog(t, logs, tc.phase, tc.category, tc.state)
		})
	}
	t.Run("invalid input", func(t *testing.T) {
		logs := captureStartLogs(t, func() {
			if err := Run(context.Background(), nil, "ready"); err != ErrStart {
				t.Fatalf("invalid Run error changed: %v", err)
			}
		})
		assertStartLog(t, logs, "invalid_input", "other", "")
	})
	t.Run("pre-cancelled signal", func(t *testing.T) {
		ctx, cancelSignal := context.WithCancel(context.Background())
		cancelSignal()
		logs := captureStartLogs(t, func() {
			cancel, err := startWorker(ctx, time.Second, func(context.Context) error { return nil })
			if cancel != nil || err != ErrStart {
				t.Fatalf("pre-cancelled error changed: %v", err)
			}
		})
		assertStartLog(t, logs, "signal", "cancelled", "")
	})
}
