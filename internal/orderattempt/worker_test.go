// Purpose: reject unusable worker registration and sweep dependencies before startup.
// Depends on: the concrete orderattempt executor and upstream River registry.
// Used by: OA1 tests-first CI; native scheduling and execution live in foundation tests.
// Invariants: no fake consumer, optional-success dependency or extra public Abort API.
package orderattempt

import (
	"context"
	"testing"

	"github.com/riverqueue/river"
)

func TestOrderAttemptWorkerRejectsMissingExecutor(t *testing.T) {
	if err := AddWorkers(nil, nil); err == nil {
		t.Fatal("nil registry and executor accepted")
	}
	if err := AddWorkers(river.NewWorkers(), nil); err == nil {
		t.Fatal("nil executor accepted")
	}
	if PeriodicJob() == nil {
		t.Fatal("mandatory periodic recovery omitted")
	}
}

func TestOrderAttemptSweepRejectsMissingExecutor(t *testing.T) {
	var executor *Executor
	got, err := executor.Sweep(context.Background())
	if err == nil || got != (SweepResult{}) {
		t.Fatal("unconfigured sweep reported success or effects")
	}
}
