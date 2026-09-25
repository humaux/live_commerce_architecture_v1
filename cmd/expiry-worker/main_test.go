package main

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestDisabledWorkerReadsOnlyFlag(t *testing.T) {
	for _, flag := range []string{"", "0"} {
		read := []string{}
		env := func(name string) string {
			read = append(read, name)
			if name == "COMMERCE_EXPIRY_WORKER_ENABLED" {
				return flag
			}
			t.Fatal("disabled worker inspected another variable")
			return ""
		}
		if err := run(context.Background(), env); err != nil || len(read) != 1 ||
			read[0] != "COMMERCE_EXPIRY_WORKER_ENABLED" {
			t.Fatalf("disabled worker: reads=%v err=%v", read, err)
		}
	}
}

func TestWorkerConfigStrictAndRedacted(t *testing.T) {
	base := map[string]string{
		"COMMERCE_EXPIRY_WORKER_ENABLED":      "1",
		"COMMERCE_EXPIRY_WORKER_DATABASE_URL": "postgres://secret@localhost/test",
	}
	read := func(values map[string]string) func(string) string {
		return func(name string) string { return values[name] }
	}
	config, err := loadConfig(read(base))
	if err != nil || !config.enabled || config.concurrency != 4 || config.dsn != base["COMMERCE_EXPIRY_WORKER_DATABASE_URL"] {
		t.Fatal("valid default configuration rejected")
	}
	for _, tc := range []struct{ field, value string }{
		{"COMMERCE_EXPIRY_WORKER_ENABLED", "true"},
		{"COMMERCE_EXPIRY_WORKER_DATABASE_URL", ""},
		{"COMMERCE_EXPIRY_WORKER_CONCURRENCY", "0"},
		{"COMMERCE_EXPIRY_WORKER_CONCURRENCY", "17"},
		{"COMMERCE_EXPIRY_WORKER_CONCURRENCY", "04"},
		{"COMMERCE_EXPIRY_WORKER_CONCURRENCY", "+4"},
	} {
		values := map[string]string{}
		for k, v := range base {
			values[k] = v
		}
		values[tc.field] = tc.value
		_, err := loadConfig(read(values))
		if !errors.Is(err, errWorkerConfig) {
			t.Fatalf("%s accepted", tc.field)
		}
		if strings.Contains(err.Error(), tc.value) && tc.value != "" {
			t.Fatalf("%s leaked input in error", tc.field)
		}
	}
}
