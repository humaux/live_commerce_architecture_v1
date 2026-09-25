package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"
)

func testEnvironment() map[string]string {
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))
	replay := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{2}, 32))
	return map[string]string{
		"COMMERCE_PAYMENT_WORKER_ENABLED":      "1",
		"COMMERCE_PAYMENT_WORKER_DATABASE_URL": "postgres://secret@localhost/test",
		"COMMERCE_PAYMENT_WORKER_PROFILE":      "SANDBOX",
		"COMMERCE_ACCOUNT_ACTIVE_KEY_ID":       "key-1",
		"COMMERCE_ACCOUNT_KEYS_JSON":           `[{"id":"key-1","key_base64":"` + key + `"}]`,
		"COMMERCE_ACCOUNT_REPLAY_KEY":          replay,
	}
}

func TestDisabledWorkerReadsOnlyFlag(t *testing.T) {
	for _, flag := range []string{"", "0"} {
		read := []string{}
		env := func(name string) string {
			read = append(read, name)
			if name == "COMMERCE_PAYMENT_WORKER_ENABLED" {
				return flag
			}
			t.Fatal("disabled worker inspected another variable")
			return ""
		}
		if err := run(context.Background(), env); err != nil || len(read) != 1 ||
			read[0] != "COMMERCE_PAYMENT_WORKER_ENABLED" {
			t.Fatalf("disabled worker: reads=%v err=%v", read, err)
		}
	}
}

func TestWorkerConfigStrictAndRedacted(t *testing.T) {
	base := testEnvironment()
	read := func(values map[string]string) func(string) string {
		return func(name string) string { return values[name] }
	}
	config, err := loadConfig(read(base))
	if err != nil || !config.enabled || config.profile != "SANDBOX" ||
		config.concurrency != 4 || config.keys == nil {
		t.Fatal("valid default configuration rejected")
	}
	for _, tc := range []struct{ field, value string }{
		{"COMMERCE_PAYMENT_WORKER_ENABLED", "true"},
		{"COMMERCE_PAYMENT_WORKER_DATABASE_URL", ""},
		{"COMMERCE_PAYMENT_WORKER_PROFILE", "PROVIDER_MOCK"},
		{"COMMERCE_PAYMENT_WORKER_PROFILE", "LIVE "},
		{"COMMERCE_PAYMENT_WORKER_CONCURRENCY", "0"},
		{"COMMERCE_PAYMENT_WORKER_CONCURRENCY", "17"},
		{"COMMERCE_PAYMENT_WORKER_CONCURRENCY", "04"},
		{"COMMERCE_PAYMENT_WORKER_CONCURRENCY", "+4"},
		{"COMMERCE_ACCOUNT_KEYS_JSON", "secret-invalid-json"},
	} {
		values := testEnvironment()
		values[tc.field] = tc.value
		_, err := loadConfig(read(values))
		if !errors.Is(err, errWorkerConfig) && !errors.Is(err, errWorkerKeyring) {
			t.Fatalf("%s accepted", tc.field)
		}
		if strings.Contains(err.Error(), tc.value) && tc.value != "" {
			t.Fatalf("%s leaked input in error", tc.field)
		}
	}
}

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
		if !errors.Is(err, errWorkerStart) || cancel != nil {
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
		if !errors.Is(err, errWorkerStart) || cancel != nil {
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
