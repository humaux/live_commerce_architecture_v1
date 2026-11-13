// Pure in-memory parent composition tests; no SQL, provider or real-clock SLO.
package main

import (
	"context"
	"errors"
	"fmt"
	"livecommerce/internal/live"
	"reflect"
	"testing"
	"time"
)

func mixedFixture(t *testing.T, n int) (*recoveryEpisode, []live.RecoveryReadback) {
	t.Helper()
	e, err := newRecoveryEpisodeAt(time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	e.mixed = true
	e.admitted = true
	rows := make([]live.RecoveryReadback, n)
	for i := range rows {
		id := fmt.Sprintf("op-%d", i)
		e.members = append(e.members, live.RecoveryMember{EpisodeID: e.id, OperationID: id, JobID: int64(i + 1), BaselineGeneration: 1, Disposition: "pending", ExecutionProfile: "LOCAL_SFU_MOCK_EGRESS", InputRequired: true, EgressRequired: true, CandidateCount: n, CoverageKnown: true})
		rows[i] = live.RecoveryReadback{EpisodeID: e.id, OperationID: id, JobID: int64(i + 1), BaselineGeneration: 1, ScopeStatus: "pending", Disposition: "checked", ExecutionProfile: "LOCAL_SFU_MOCK_EGRESS", InputRequired: true, EgressRequired: true, CandidateCount: n, CoverageKnown: true, ObservationID: "egress-" + id, ObservationSource: "QUERY", ObservationGeneration: 2, InputObservationID: "input-" + id, InputObservationSource: "PARTICIPANT", InputObservationResult: "PRESENT", InputObservationGeneration: 3}
	}
	return e, rows
}

type mixedSpy struct {
	legacy               int
	calls                []recoveryAttestation
	status               string
	err                  error
	episodes, operations []string
}

func (s *mixedSpy) Witness(context.Context, string, string, string, int64) (string, error) {
	s.legacy++
	return s.status, s.err
}
func (s *mixedSpy) WitnessWithInput(_ context.Context, e, o, i, g string, ms int64) (string, error) {
	s.calls = append(s.calls, recoveryAttestation{observationID: g, inputObservationID: i, elapsedMS: ms})
	s.episodes = append(s.episodes, e)
	s.operations = append(s.operations, o)
	return s.status, s.err
}

func TestRecoveryMixedBatchAtomic(t *testing.T) {
	mutations := map[string]func([]live.RecoveryReadback){
		"foreign":   func(r []live.RecoveryReadback) { r[1].OperationID = "foreign" },
		"duplicate": func(r []live.RecoveryReadback) { r[1] = r[0] },
		"episode":   func(r []live.RecoveryReadback) { r[1].EpisodeID = "foreign" },
		"job":       func(r []live.RecoveryReadback) { r[1].JobID++ },
		"profile":   func(r []live.RecoveryReadback) { r[1].ExecutionProfile = "" },
		"mask":      func(r []live.RecoveryReadback) { r[1].InputRequired = false },
		"baseline":  func(r []live.RecoveryReadback) { r[1].BaselineGeneration++ },
		"coverage":  func(r []live.RecoveryReadback) { r[1].CoverageKnown = false },
		"count":     func(r []live.RecoveryReadback) { r[1].CandidateCount++ },
		"blocked":   func(r []live.RecoveryReadback) { r[1].BlockedByEpisodeID = "other" },
		"scope":     func(r []live.RecoveryReadback) { r[1].ScopeStatus = "finished" },
		"status_only": func(r []live.RecoveryReadback) {
			for i := range r {
				r[i].ScopeStatus = "finished"
			}
		},
		"source": func(r []live.RecoveryReadback) { r[1].InputObservationSource = "ROOM" },
		"stale":  func(r []live.RecoveryReadback) { r[1].ObservationGeneration = 1 },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			e, r := mixedFixture(t, 2)
			mutate(r)
			e.acceptReadback(r, 100)
			if len(e.pending) != 0 || len(e.witnessed) != 0 || len(e.rejected) != 0 || e.resolved || e.pendingProof.Load() || e.resolvedProof.Load() {
				t.Fatal("malformed batch mutated state")
			}
		})
	}
	for _, n := range []int{0, 1, 3} {
		t.Run(fmt.Sprint("cardinality", n), func(t *testing.T) {
			e, r := mixedFixture(t, 2)
			if n == 3 {
				r = append(r, r[0])
			} else {
				r = r[:n]
			}
			e.acceptReadback(r, 100)
			if len(e.pending) != 0 || e.resolved {
				t.Fatal("cardinality accepted")
			}
		})
	}
}

func TestRecoveryMixedProofs(t *testing.T) {
	for _, mask := range []string{"both", "input", "egress", "provider"} {
		for _, partial := range []bool{false, true} {
			t.Run(fmt.Sprint(mask, partial), func(t *testing.T) {
				e, r := mixedFixture(t, 1)
				if mask == "input" {
					e.members[0].EgressRequired = false
					r[0].EgressRequired = false
					r[0].ObservationID = ""
					r[0].ObservationSource = ""
					r[0].ObservationGeneration = 0
				}
				if mask == "egress" || mask == "provider" {
					e.members[0].InputRequired = false
					r[0].InputRequired = false
					r[0].InputObservationID = ""
					r[0].InputObservationSource = ""
					r[0].InputObservationResult = ""
					r[0].InputObservationGeneration = 0
				}
				if mask == "provider" {
					e.members[0].ExecutionProfile = "PROVIDER_MOCK"
					r[0].ExecutionProfile = "PROVIDER_MOCK"
				}
				if partial {
					r[0].Disposition = "pending"
					if r[0].InputRequired {
						r[0].InputObservationID = ""
						r[0].InputObservationSource = ""
						r[0].InputObservationResult = ""
						r[0].InputObservationGeneration = 0
					} else {
						r[0].ObservationID = ""
						r[0].ObservationSource = ""
						r[0].ObservationGeneration = 0
					}
				}
				e.acceptReadback(r, 90000)
				if partial {
					if len(e.pending) != 0 || e.pendingProof.Load() {
						t.Fatal("partial proof cached")
					}
					return
				}
				s := &mixedSpy{status: "witnessed"}
				e.attemptPending(context.Background(), s)
				if s.legacy != 0 || len(s.calls) != 1 || s.calls[0].inputObservationID != r[0].InputObservationID || s.calls[0].observationID != r[0].ObservationID || !e.resolved {
					t.Fatal("composite dispatch failed")
				}
			})
		}
	}
	e, r := mixedFixture(t, 2)
	r[1].Disposition = "pending"
	r[1].InputObservationID = ""
	r[1].InputObservationSource = ""
	r[1].InputObservationResult = ""
	r[1].InputObservationGeneration = 0
	e.acceptReadback(r, 100)
	if len(e.pending) != 1 || e.pendingProof.Load() || e.resolved {
		t.Fatal("qualified subset lost or episode completed")
	}
}

func TestRecoveryMixedTimingRetry(t *testing.T) {
	for _, ms := range []int64{-1, 89999, 90000, 90001} {
		t.Run(fmt.Sprint(ms), func(t *testing.T) {
			e, r := mixedFixture(t, 1)
			e.acceptReadback(r, ms)
			want := ms >= 0 && ms <= 90000
			if (len(e.pending) == 1) != want {
				t.Fatal("boundary")
			}
		})
	}
	e, r := mixedFixture(t, 1)
	e.acceptReadback(r, 89999)
	original := e.pending[r[0].OperationID]
	s := &mixedSpy{err: errors.New("transient")}
	e.attemptPending(context.Background(), s)
	r[0].InputObservationID = "later-input"
	r[0].ObservationID = "later-egress"
	e.acceptReadback(r, 90001)
	if !reflect.DeepEqual(original, e.pending[r[0].OperationID]) {
		t.Fatal("cache replaced")
	}
	s.err = nil
	s.status = "already_witnessed"
	e.attemptPending(context.Background(), s)
	if s.legacy != 0 || len(s.calls) != 2 || !reflect.DeepEqual(s.calls[0], s.calls[1]) || s.calls[1].elapsedMS != 89999 || s.episodes[1] != e.id || s.operations[1] != r[0].OperationID || !e.resolved {
		t.Fatal("retry tuple changed")
	}
}

func TestRecoveryMixedTimeout(t *testing.T) {
	for _, status := range []string{"timeout_wins", "unqualified", "read_timeout"} {
		t.Run(status, func(t *testing.T) {
			e, r := mixedFixture(t, 1)
			e.acceptReadback(r, 10)
			if status == "read_timeout" {
				r[0].ScopeStatus = "overdue"
				r[0].Disposition = "timeout"
				r[0].TimeoutAt = time.Unix(100, 0)
				e.acceptReadback(r, 90001)
			} else {
				e.attemptPending(context.Background(), &mixedSpy{status: status})
			}
			if len(e.pending) != 0 || !e.rejected[r[0].OperationID] || e.pendingProof.Load() || e.resolved {
				t.Fatal("sticky rejection lost")
			}
			r[0].ScopeStatus = "pending"
			r[0].Disposition = "checked"
			r[0].TimeoutAt = time.Time{}
			e.acceptReadback(r, 20)
			if len(e.pending) != 0 {
				t.Fatal("rejected recached")
			}
		})
	}
}

func TestRecoveryMixedEmpty(t *testing.T) {
	for _, bad := range []string{"", "unknown", "proof", "member", "count", "duplicate", "negative"} {
		t.Run(bad, func(t *testing.T) {
			e, _ := mixedFixture(t, 0)
			e.members = []live.RecoveryMember{{EpisodeID: e.id, Disposition: "empty", CoverageKnown: true}}
			r := []live.RecoveryReadback{{EpisodeID: e.id, ScopeStatus: "empty", CoverageKnown: true}}
			ms := int64(90001)
			switch bad {
			case "unknown":
				r[0].CoverageKnown = false
			case "proof":
				r[0].InputObservationID = "fake"
			case "member":
				r[0].OperationID = "fake"
			case "count":
				r[0].CandidateCount = 1
			case "duplicate":
				r = append(r, r[0])
			case "negative":
				ms = -1
			}
			e.acceptReadback(r, ms)
			if e.resolved != (bad == "") || len(e.pending) != 0 || len(e.witnessed) != 0 || e.pendingProof.Load() {
				t.Fatal("empty classification")
			}
		})
	}
}

func TestRecoveryMixedAdmissionAndRetainedState(t *testing.T) {
	mutations := map[string]func(*recoveryEpisode){
		"unadmitted": func(e *recoveryEpisode) { e.admitted = false },
		"unknown":    func(e *recoveryEpisode) { e.coverageKnown = false },
		"duplicate":  func(e *recoveryEpisode) { e.members[1] = e.members[0] },
		"count":      func(e *recoveryEpisode) { e.members[1].CandidateCount++ },
		"coverage":   func(e *recoveryEpisode) { e.members[1].CoverageKnown = false },
		"blocked":    func(e *recoveryEpisode) { e.members[1].BlockedByEpisodeID = "other" },
		"profile":    func(e *recoveryEpisode) { e.members[1].ExecutionProfile = "" },
		"mask":       func(e *recoveryEpisode) { e.members[1].InputRequired = false; e.members[1].EgressRequired = false },
		"job":        func(e *recoveryEpisode) { e.members[1].JobID = 0 },
		"baseline":   func(e *recoveryEpisode) { e.members[1].BaselineGeneration = -1 },
		"overflow":   func(e *recoveryEpisode) { e.members[1].Disposition = "capacity_exceeded" },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			e, r := mixedFixture(t, 2)
			mutate(e)
			e.acceptReadback(r, 10)
			if len(e.pending) != 0 || e.resolved {
				t.Fatal("invalid admission accepted")
			}
		})
	}
	e, r := mixedFixture(t, 2)
	e.acceptReadback(r, 100)
	before := map[string]recoveryAttestation{}
	for id, p := range e.pending {
		before[id] = p
	}
	r[0].Disposition = "witnessed"
	r[0].WitnessElapsedMS = 100
	r[1].JobID++
	e.acceptReadback(r, 90001)
	if !reflect.DeepEqual(before, e.pending) || len(e.witnessed) != 0 || len(e.rejected) != 0 || !e.pendingProof.Load() || e.resolved {
		t.Fatal("bad later row changed retained state")
	}
	// An altered admitted binding cannot be sent using an old cached tuple.
	e.members[0].JobID++
	s := &mixedSpy{status: "witnessed"}
	e.attemptPending(context.Background(), s)
	if len(s.calls) != 0 || s.legacy != 0 {
		t.Fatal("cache/member mismatch dispatched")
	}
}

func TestRecoveryMixedDurableFinished(t *testing.T) {
	for _, bad := range []string{"", "late_witness", "partial", "timeout"} {
		t.Run(bad, func(t *testing.T) {
			e, r := mixedFixture(t, 1)
			r[0].ScopeStatus = "finished"
			r[0].Disposition = "witnessed"
			r[0].WitnessElapsedMS = 89999
			switch bad {
			case "late_witness":
				r[0].WitnessElapsedMS = 90001
			case "partial":
				r[0].InputObservationID = ""
				r[0].InputObservationSource = ""
				r[0].InputObservationResult = ""
				r[0].InputObservationGeneration = 0
			case "timeout":
				r[0].TimeoutAt = time.Unix(2, 0)
			}
			e.acceptReadback(r, 90001)
			if e.resolved != (bad == "") {
				t.Fatal("durable finished validation")
			}
		})
	}
	e, r := mixedFixture(t, 1)
	e.acceptReadback(r, 89999)
	r[0].ScopeStatus = "finished"
	r[0].Disposition = "witnessed"
	r[0].WitnessElapsedMS = 89999
	r[0].ObservationID = "different"
	e.acceptReadback(r, 90001)
	if e.resolved || len(e.pending) != 1 {
		t.Fatal("conflicting durable tuple accepted")
	}
}

func TestRecoveryMixedOptionalSidesAndSources(t *testing.T) {
	for _, inputSource := range []string{"PARTICIPANT", "ROOM"} {
		for _, egressSource := range []string{"QUERY", "ROOM"} {
			t.Run(inputSource+egressSource, func(t *testing.T) {
				e, r := mixedFixture(t, 1)
				r[0].InputObservationSource = inputSource
				if inputSource == "ROOM" {
					r[0].InputObservationResult = "ABSENT"
				}
				r[0].ObservationSource = egressSource
				e.acceptReadback(r, 100)
				if len(e.pending) != 1 {
					t.Fatal("valid source rejected")
				}
			})
		}
	}
	for _, side := range []string{"input", "egress"} {
		t.Run(side, func(t *testing.T) {
			e, r := mixedFixture(t, 1)
			if side == "input" {
				e.members[0].InputRequired = false
				r[0].InputRequired = false
			} else {
				e.members[0].EgressRequired = false
				r[0].EgressRequired = false
			}
			e.acceptReadback(r, 100)
			if len(e.pending) != 0 {
				t.Fatal("optional-side nonzero accepted")
			}
		})
	}
	// A legacy-shaped read cannot silently downgrade a constructed mixed episode.
	e, r := mixedFixture(t, 1)
	r[0].ExecutionProfile = ""
	r[0].InputRequired = false
	r[0].EgressRequired = false
	e.acceptReadback(r, 10)
	if len(e.pending) != 0 || e.resolved {
		t.Fatal("downgrade")
	}
	legacy, err := newRecoveryEpisode()
	if err != nil {
		t.Fatal(err)
	}
	if legacy.mixed {
		t.Fatal("default activated")
	}
	legacy.pending["legacy"] = recoveryAttestation{observationID: "old", elapsedMS: 10}
	s := &mixedSpy{status: "witnessed"}
	legacy.attemptPending(context.Background(), s)
	if s.legacy != 1 || len(s.calls) != 0 {
		t.Fatal("legacy dispatch changed")
	}
}

func TestRecoveryMixedScopeConsistency(t *testing.T) {
	for _, scope := range []string{"pending", "overdue"} {
		t.Run(scope, func(t *testing.T) {
			e, r := mixedFixture(t, 1)
			r[0].ScopeStatus = scope
			r[0].Disposition = "witnessed"
			r[0].WitnessElapsedMS = 1
			e.acceptReadback(r, 90001)
			if e.resolved || len(e.witnessed) != 0 {
				t.Fatal("inconsistent scope accepted")
			}
		})
	}
	e, r := mixedFixture(t, 2)
	for i := range r {
		r[i].ScopeStatus = "overdue"
	}
	r[0].Disposition = "witnessed"
	r[0].WitnessElapsedMS = 10
	r[1].Disposition = "timeout"
	r[1].TimeoutAt = time.Unix(100, 0)
	e.acceptReadback(r, 90001)
	if !e.witnessed[r[0].OperationID] || !e.rejected[r[1].OperationID] || e.resolved || e.pendingProof.Load() {
		t.Fatal("valid sticky timeout mixture rejected")
	}
	for _, disposition := range []string{"checked", "witnessed", "timeout", "terminal"} {
		t.Run("admission_"+disposition, func(t *testing.T) {
			e, r := mixedFixture(t, 1)
			e.members[0].Disposition = disposition
			e.acceptReadback(r, 100)
			if len(e.pending) != 0 {
				t.Fatal("non-Begin disposition admitted")
			}
		})
	}
}
