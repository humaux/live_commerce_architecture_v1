// Purpose: calibrate the observer verdict with deterministic MOCK observations.
// Depends on: testing and time only; no substitute for native Go/PG correctness.
// Used by: T19 Go W1 CI before the native fixture run.
// Invariants: I18; empty, zero-success and inconsistent artifacts cannot pass.
package w1

import (
	"context"
	"testing"
	"time"
)

func TestVerdictCalibration(t *testing.T) {
	now := time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC)
	good := Block{Start: now, End: now.Add(Window)}
	for i := 0; i < Attempts; i++ {
		good.Attempts = append(good.Attempts, Attempt{Index: i, PlannedNS: int64(i) * int64(Window/Attempts), StartedNS: int64(i) * int64(Window/Attempts), CompletedNS: int64(i)*int64(Window/Attempts) + 100, LatencyNS: 100, Status: 200, ExpectedStatus: 200, Started: true, Completed: true})
	}
	summarize(&good)
	if err := Validate(good); err != nil {
		t.Fatal("MOCK normal calibration", err)
	}
	t.Log("MOCK normal data: measurement verdict PASS")
	bad := good
	bad.Attempts = nil
	if Validate(bad) == nil {
		t.Fatal("empty data passed")
	}
	t.Log("MOCK negative calibration: empty-data measurement verdict FAIL (expected)")
	bad = good
	bad.ACKs = 0
	if Validate(bad) == nil {
		t.Fatal("zero-success data passed")
	}
	bad = good
	bad.P99NS = 1
	if Validate(bad) == nil {
		t.Fatal("unordered quantiles passed")
	}
	bad = good
	bad.Attempts = append([]Attempt(nil), good.Attempts...)
	bad.Attempts[0].Error = "timeout"
	if Validate(bad) == nil {
		t.Fatal("timeout passed")
	}
}

func TestCancelledRunRetainsAllAttempts(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	inputs := make([]Request, Attempts)
	for i := range inputs {
		inputs[i] = Request{Body: []byte(`{}`)}
	}
	b, err := Run(ctx, "http://127.0.0.1:1/owned", "synthetic-only", inputs, 1901101)
	if err != nil {
		t.Fatal(err)
	}
	if b.Planned != Attempts || b.Dropped != Attempts || b.Started != 0 || b.Errors != Attempts || b.Completed != 0 {
		t.Fatalf("cancelled attempts lost: %+v", b)
	}
	if Validate(b) == nil {
		t.Fatal("cancelled zero-success run passed")
	}
}

func TestTargetGuard(t *testing.T) {
	for _, target := range []string{"https://127.0.0.1:80/x", "http://localhost:80/x", "http://192.0.2.1:80/x", "http://127.0.0.1:80/x?q=secret"} {
		if _, err := Run(context.Background(), target, "synthetic", make([]Request, Attempts), 1); err == nil {
			t.Fatal("unowned target accepted")
		}
	}
}
