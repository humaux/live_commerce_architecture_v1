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

func compositeFixture(t *testing.T) (*recoveryEpisode, []live.RecoveryReadback) {
	t.Helper()
	e, err := newRecoveryEpisode()
	if err != nil {
		t.Fatal(err)
	}
	e.withInput, e.admitted = true, true
	e.members = []live.RecoveryMember{
		{EpisodeID: e.id, OperationID: "op-a", JobID: 10, Disposition: "pending", BaselineGeneration: 3,
			ExecutionProfile: "LOCAL_SFU_MOCK_EGRESS", InputRequired: true, EgressRequired: true,
			CoverageKnown: true, CandidateCount: 2},
		{EpisodeID: e.id, OperationID: "op-b", JobID: 11, Disposition: "pending", BaselineGeneration: 0,
			ExecutionProfile: "LOCAL_SFU_MOCK_EGRESS", InputRequired: true,
			CoverageKnown: true, CandidateCount: 2},
	}
	rows := []live.RecoveryReadback{
		{EpisodeID: e.id, OperationID: "op-a", JobID: 10, ScopeStatus: "pending", Disposition: "checked",
			ExecutionProfile: "LOCAL_SFU_MOCK_EGRESS", InputRequired: true, EgressRequired: true,
			CoverageKnown: true, CandidateCount: 2, BaselineGeneration: 3,
			InputObservationID: "input-a", InputObservationSource: "PARTICIPANT", InputObservationResult: "PRESENT",
			InputObservationGeneration: 4, ObservationID: "egress-a", ObservationSource: "QUERY", ObservationGeneration: 5},
		{EpisodeID: e.id, OperationID: "op-b", JobID: 11, ScopeStatus: "pending", Disposition: "checked",
			ExecutionProfile: "LOCAL_SFU_MOCK_EGRESS", InputRequired: true,
			CoverageKnown: true, CandidateCount: 2, BaselineGeneration: 0,
			InputObservationID: "input-b", InputObservationSource: "ROOM", InputObservationResult: "ABSENT",
			InputObservationGeneration: 1},
	}
	return e, rows
}

func TestCompositeReadbackCrossGenerationAndTimelyAttestation(t *testing.T) {
	e, rows := compositeFixture(t)
	e.acceptReadback(rows, 89900)
	if !e.pendingProof.Load() || len(e.pending) != 2 {
		t.Fatal("complete cross-generation batch did not become pending")
	}
	if proof := e.pending["op-a"]; proof.inputID != "input-a" || proof.egressID != "egress-a" || proof.elapsedMS != 89900 {
		t.Fatalf("composite proof mismatch: %+v", proof)
	}
	rows[0].InputObservationID = "later-input"
	e.acceptReadback(rows, 90001)
	if proof := e.pending["op-a"]; proof.inputID != "input-a" || proof.elapsedMS != 89900 {
		t.Fatalf("late read replaced original proof: %+v", proof)
	}
}

func TestCompositeReadbackRejectsMalformedWholeBatch(t *testing.T) {
	cases := map[string]func([]live.RecoveryReadback) []live.RecoveryReadback{
		"missing row":   func(r []live.RecoveryReadback) []live.RecoveryReadback { return r[:1] },
		"duplicate":     func(r []live.RecoveryReadback) []live.RecoveryReadback { r[1] = r[0]; return r },
		"foreign id":    func(r []live.RecoveryReadback) []live.RecoveryReadback { r[1].OperationID = "foreign"; return r },
		"wrong episode": func(r []live.RecoveryReadback) []live.RecoveryReadback { r[1].EpisodeID = "other"; return r },
		"wrong job":     func(r []live.RecoveryReadback) []live.RecoveryReadback { r[1].JobID++; return r },
		"wrong profile": func(r []live.RecoveryReadback) []live.RecoveryReadback {
			r[1].ExecutionProfile = "PROVIDER_MOCK"
			return r
		},
		"wrong mask":     func(r []live.RecoveryReadback) []live.RecoveryReadback { r[1].EgressRequired = true; return r },
		"wrong baseline": func(r []live.RecoveryReadback) []live.RecoveryReadback { r[1].BaselineGeneration++; return r },
		"wrong count":    func(r []live.RecoveryReadback) []live.RecoveryReadback { r[1].CandidateCount++; return r },
		"wrong scope":    func(r []live.RecoveryReadback) []live.RecoveryReadback { r[1].ScopeStatus = "finished"; return r },
		"stale egress":   func(r []live.RecoveryReadback) []live.RecoveryReadback { r[0].ObservationGeneration = 3; return r },
		"extra egress":   func(r []live.RecoveryReadback) []live.RecoveryReadback { r[1].ObservationID = "extra"; return r },
		"bad input source": func(r []live.RecoveryReadback) []live.RecoveryReadback {
			r[0].InputObservationSource = "ROOM"
			return r
		},
		"false finished": func(r []live.RecoveryReadback) []live.RecoveryReadback {
			for i := range r {
				r[i].ScopeStatus = "finished"
			}
			return r
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			e, rows := compositeFixture(t)
			e.acceptReadback(mutate(rows), 89900)
			if len(e.pending) != 0 || len(e.witnessed) != 0 || e.pendingProof.Load() || e.resolvedProof.Load() {
				t.Fatal("malformed batch affected proof state")
			}
		})
	}
}

func TestCompositeFirstLateOrPartialCannotHideDeadline(t *testing.T) {
	for _, partial := range []bool{false, true} {
		e, rows := compositeFixture(t)
		if partial {
			rows[1].InputObservationID = ""
			rows[1].InputObservationSource = ""
			rows[1].InputObservationResult = ""
			rows[1].InputObservationGeneration = 0
			e.acceptReadback(rows, 89900)
			if e.pendingProof.Load() || len(e.pending) != 1 {
				t.Fatal("partial proof must retain only the complete member and signal deadline")
			}
		}
		e.started = time.Now().Add(-90*time.Second + 40*time.Millisecond)
		e.acceptReadback(rows, 90001)
		if (!partial && len(e.pending) != 0) || e.pendingProof.Load() {
			t.Fatal("late or partial batch created full proof")
		}
		e.startDeadlineSignal(context.Background())
		time.Sleep(100 * time.Millisecond)
		e.stopDeadline()
		if !e.alerted.Load() {
			t.Fatal("deadline alert was hidden")
		}
	}
}

func TestCompositeSchedulerRetriesMissingSideAfterPartialOrAckLoss(t *testing.T) {
	for _, disposition := range []string{"partial", "remote_unknown", "checked", "already_observed"} {
		t.Run(disposition, func(t *testing.T) {
			e, rows := compositeFixture(t)
			member := e.members[0]
			e.inflight[member.OperationID] = true
			e.noteObserveResult(observeResult{episodeID: e.id, operationID: member.OperationID, disposition: disposition})
			if !e.shouldObserve(member) || e.observed[member.OperationID] {
				t.Fatal("provider status suppressed durable readback/retry")
			}
			// Input fails while Egress G commits. A later G+1 input receipt
			// completes the original Egress proof without copying or replacing it.
			rows[0].InputObservationID = ""
			rows[0].InputObservationSource = ""
			rows[0].InputObservationResult = ""
			rows[0].InputObservationGeneration = 0
			e.acceptReadback(rows, 1000)
			if !e.shouldObserve(member) || e.pendingProof.Load() {
				t.Fatal("partial receipt stopped retry of missing side")
			}
			rows[0].InputObservationID = "input-next-generation"
			rows[0].InputObservationSource = "PARTICIPANT"
			rows[0].InputObservationResult = "PRESENT"
			rows[0].InputObservationGeneration = 6
			e.acceptReadback(rows, 2000)
			if e.shouldObserve(member) || e.pending[member.OperationID].egressID != "egress-a" {
				t.Fatal("complete readback did not stop provider I/O or preserve first Egress proof")
			}
		})
	}
}

func TestCompositeTimeoutReadbackClearsPendingProof(t *testing.T) {
	e, rows := compositeFixture(t)
	e.acceptReadback(rows, 89900)
	if !e.pendingProof.Load() {
		t.Fatal("fixture has no pending proof")
	}
	rows[0].Disposition = "timeout"
	e.acceptReadback(rows, 90001)
	if e.pendingProof.Load() || e.resolvedProof.Load() || len(e.pending) != 1 {
		t.Fatal("durable timeout did not defeat one pending witness")
	}
}

func TestCompositeProviderMockEgressOnlyAndKnownEmpty(t *testing.T) {
	e, err := newRecoveryEpisode()
	if err != nil {
		t.Fatal(err)
	}
	e.withInput, e.admitted = true, true
	e.members = []live.RecoveryMember{{EpisodeID: e.id, OperationID: "mock-op", JobID: 42,
		Disposition: "pending", ExecutionProfile: "PROVIDER_MOCK", EgressRequired: true,
		CoverageKnown: true, CandidateCount: 1, BaselineGeneration: 3}}
	row := live.RecoveryReadback{EpisodeID: e.id, OperationID: "mock-op", JobID: 42,
		ScopeStatus: "pending", Disposition: "checked", ExecutionProfile: "PROVIDER_MOCK",
		EgressRequired: true, CoverageKnown: true, CandidateCount: 1, BaselineGeneration: 3,
		ObservationID: "mock-egress", ObservationSource: "ROOM", ObservationGeneration: 4}
	e.acceptReadback([]live.RecoveryReadback{row}, 1500)
	if proof := e.pending["mock-op"]; proof.inputID != "" || proof.egressID != "mock-egress" {
		t.Fatalf("Egress-only proof incorrect: %+v", proof)
	}
	e2, err := newRecoveryEpisode()
	if err != nil {
		t.Fatal(err)
	}
	e2.withInput, e2.admitted = true, true
	e2.members = []live.RecoveryMember{{EpisodeID: e2.id, Disposition: "empty", CoverageKnown: true}}
	e2.acceptReadback([]live.RecoveryReadback{{EpisodeID: e2.id, ScopeStatus: "empty", CoverageKnown: true}}, 90001)
	if !e2.resolvedProof.Load() || len(e2.pending) != 0 {
		t.Fatal("known complete empty scope was not resolved as no work")
	}
}
