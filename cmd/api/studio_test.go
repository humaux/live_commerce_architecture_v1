package main

import (
	"context"
	"errors"
	"testing"
)

func TestStudioBackendSTU05StartupAdmission(t *testing.T) {
	for _, value := range []string{"", "0"} {
		config, err := loadStudioConfig(func(string) string { return value }, false, "0.0.0.0:8080")
		if err != nil || config.enabled {
			t.Fatalf("default-off %q: %+v %v", value, config, err)
		}
		planner, err := buildStudioPlanner(context.Background(), nil, config)
		if err != nil || planner != nil {
			t.Fatalf("off touched DB: %v %v", planner, err)
		}
	}
	for _, value := range []string{"true", "false", "yes", "2", " 1", "1 ", "-1"} {
		_, err := loadStudioConfig(func(string) string { return value }, true, "127.0.0.1:8080")
		if !errors.Is(err, errStudioConfig) {
			t.Fatalf("noncanonical flag %q accepted: %v", value, err)
		}
	}
	for _, tc := range []struct {
		identity bool
		addr     string
	}{
		{false, "127.0.0.1:8080"},
		{true, "0.0.0.0:8080"},
		{true, "localhost:8080"},
		{true, "192.0.2.10:8080"},
	} {
		_, err := loadStudioConfig(func(string) string { return "1" }, tc.identity, tc.addr)
		if !errors.Is(err, errStudioConfig) {
			t.Fatalf("enabled without identity/loopback %+v: %v", tc, err)
		}
	}
	config, err := loadStudioConfig(func(string) string { return "1" }, true, "127.0.0.1:8080")
	if err != nil || !config.enabled {
		t.Fatalf("valid enabled config: %+v %v", config, err)
	}
	if planner, err := buildStudioPlanner(context.Background(), nil, config); planner != nil || !errors.Is(err, errStudioDatabase) {
		t.Fatalf("enabled nil pool false readiness: %v %v", planner, err)
	}
}
