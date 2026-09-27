package main

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"livecommerce/internal/live"
)

func TestRecoveryReleaseGate(t *testing.T) {
	readEnd, writeEnd, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer readEnd.Close()
	done := make(chan bool, 1)
	go func() { done <- awaitRecoveryRelease(readEnd) }()
	select {
	case <-done:
		t.Fatal("child passed before release")
	case <-time.After(30 * time.Millisecond):
	}
	if _, err := writeEnd.Write([]byte{1}); err != nil {
		t.Fatal(err)
	}
	select {
	case allowed := <-done:
		if !allowed {
			t.Fatal("release rejected")
		}
	case <-time.After(time.Second):
		t.Fatal("release not received")
	}
	_ = writeEnd.Close()
}

func TestRecoveryReleaseEOFRejects(t *testing.T) {
	readEnd, writeEnd, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer readEnd.Close()
	_ = writeEnd.Close()
	if awaitRecoveryRelease(readEnd) {
		t.Fatal("EOF became release")
	}
}

func TestRecoveryChildEnvironmentDropsRecoveryDSN(t *testing.T) {
	t.Setenv("COMMERCE_MEDIA_RECOVERY_DATABASE_URL", "secret-recovery-dsn")
	for _, entry := range sanitizedNativeEnvironment() {
		if strings.Contains(entry, "secret-recovery-dsn") {
			t.Fatal("recovery DSN leaked to child")
		}
	}
}

func TestRecoveryTimelyReadbackKeepsOriginalAttestation(t *testing.T) {
	e, err := newRecoveryEpisode()
	if err != nil {
		t.Fatal(err)
	}
	e.started = time.Now().Add(-90*time.Second + 40*time.Millisecond)
	e.admitted = true
	e.members = []live.RecoveryMember{
		{OperationID: "op-a", Disposition: "pending", CandidateCount: 2, CoverageKnown: true},
		{OperationID: "op-b", Disposition: "pending", CandidateCount: 2, CoverageKnown: true},
	}
	rows := []live.RecoveryReadback{
		{OperationID: "op-a", ScopeStatus: "pending", CoverageKnown: true, CandidateCount: 2,
			ObservationID: "obs-a", ObservationSource: "ROOM", BaselineGeneration: 1, ObservationGeneration: 2},
		{OperationID: "op-b", ScopeStatus: "pending", CoverageKnown: true, CandidateCount: 2,
			ObservationID: "obs-b", ObservationSource: "QUERY", BaselineGeneration: 1, ObservationGeneration: 3},
	}
	e.acceptReadback(rows, 89900)
	if !e.pendingProof.Load() {
		t.Fatal("timely batch not pending")
	}
	e.startDeadlineSignal(context.Background())
	defer e.stopDeadline()
	time.Sleep(100 * time.Millisecond)
	if e.alerted.Load() {
		t.Fatal("pending timely witness misclassified as missed")
	}
	rows[0].ObservationID = "later-observation"
	e.acceptReadback(rows, 90001)
	if got := e.pending["op-a"]; got.observationID != "obs-a" || got.elapsedMS != 89900 {
		t.Fatalf("original witness changed: %+v", got)
	}
	e.witnessed["op-a"], e.witnessed["op-b"] = true, true
	delete(e.pending, "op-a")
	delete(e.pending, "op-b")
	e.updateProofState()
	if !e.resolvedProof.Load() {
		t.Fatal("durable late witness not resolved")
	}
}

func TestRecoveryIncompleteBatchStillSignalsDeadlineMiss(t *testing.T) {
	e, err := newRecoveryEpisode()
	if err != nil {
		t.Fatal(err)
	}
	e.started = time.Now().Add(-90*time.Second + 40*time.Millisecond)
	e.admitted = true
	e.members = []live.RecoveryMember{
		{OperationID: "op-a", Disposition: "pending", CandidateCount: 2, CoverageKnown: true},
		{OperationID: "op-b", Disposition: "pending", CandidateCount: 2, CoverageKnown: true},
	}
	e.acceptReadback([]live.RecoveryReadback{{OperationID: "op-a", ScopeStatus: "pending",
		ObservationID: "obs-a", ObservationSource: "ROOM", BaselineGeneration: 1, ObservationGeneration: 2}}, 89900)
	if e.pendingProof.Load() {
		t.Fatal("partial witness hid a missing member")
	}
	e.startDeadlineSignal(context.Background())
	defer e.stopDeadline()
	time.Sleep(100 * time.Millisecond)
	if !e.alerted.Load() {
		t.Fatal("missing member had no independent deadline signal")
	}
}

func TestRecoveryFirstLateReadbackCannotCreateProof(t *testing.T) {
	for _, source := range []string{"ROOM", "QUERY"} {
		t.Run(source, func(t *testing.T) {
			e, err := newRecoveryEpisode()
			if err != nil {
				t.Fatal(err)
			}
			e.admitted = true
			e.members = []live.RecoveryMember{{OperationID: "op-late", Disposition: "pending", CandidateCount: 1, CoverageKnown: true}}
			// This is the readback-classifier boundary, not a real-clock SLO test.
			// Unlike a timely proof retained across a later read, no prior proof exists.
			e.acceptReadback([]live.RecoveryReadback{{
				OperationID: "op-late", ScopeStatus: "pending", CoverageKnown: true, CandidateCount: 1,
				Disposition: "checked", ObservationID: "obs-late", ObservationSource: source,
				BaselineGeneration: 1, ObservationGeneration: 2,
			}}, 90001)
			if len(e.pending) != 0 || e.pendingProof.Load() || e.resolvedProof.Load() {
				t.Fatal("first post-deadline readback invented timely or resolved proof")
			}
		})
	}
}
