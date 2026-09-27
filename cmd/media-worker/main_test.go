package main

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestMediaWorkerLMW01DisabledReadsOnlyFlag(t *testing.T) {
	for _, flag := range []string{"", "0"} {
		var reads []string
		err := run(nil, func(name string) string {
			reads = append(reads, name)
			if name == "COMMERCE_MEDIA_WORKER_ENABLED" {
				return flag
			}
			return "must-not-read"
		})
		if err != nil || len(reads) != 1 || reads[0] != "COMMERCE_MEDIA_WORKER_ENABLED" {
			t.Fatalf("disabled %q read=%v err=%v", flag, reads, err)
		}
	}
	for _, flag := range []string{"false", "2", " 1", "1 "} {
		var reads []string
		err := run(context.Background(), func(name string) string { reads = append(reads, name); return flag })
		if !errors.Is(err, errWorkerConfig) || len(reads) != 1 {
			t.Fatalf("noncanonical flag %q read=%v err=%v", flag, reads, err)
		}
	}
}

func TestMediaWorkerLMW01ConfigBeforeDatabase(t *testing.T) {
	vars := map[string]string{
		"COMMERCE_MEDIA_WORKER_ENABLED":         "1",
		"COMMERCE_MEDIA_WORKER_DATABASE_URL":    "postgres://private-user:private-pass@127.0.0.1:1/test",
		"COMMERCE_MEDIA_EXECUTOR_DATABASE_URL":  "postgres://private-user:private-pass@127.0.0.1:1/test",
		"COMMERCE_MEDIA_WORKER_CONCURRENCY":     "1",
		"COMMERCE_MEDIA_MATERIAL_ACTIVE_KEY_ID": "bad",
		"COMMERCE_MEDIA_MATERIAL_KEYS_JSON":     "private-malformed-key",
		"COMMERCE_MEDIA_PROJECTS_JSON":          "private-malformed-project",
	}
	get := func(name string) string { return vars[name] }
	if err := run(context.Background(), get); !errors.Is(err, errWorkerConfig) || strings.Contains(err.Error(), "private") {
		t.Fatalf("malformed config reached DB or exposed input: %v", err)
	}
	// Nil run context is rejected before any DB I/O when configuration is valid;
	// the loader's full valid case is exercised in livekit's package tests.
}
