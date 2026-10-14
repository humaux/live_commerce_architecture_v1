// Purpose: MODEL_ONLY subject/scope lock traces and independent lower-lock oracle.
// Depends on: the value-state intent model, frozen order-attempt-v1 §6 lock rules.
// Used by: CI fixed a–h traces and testing.F corpus; never product or REAL_PG proof.
// Invariants: admission observes subject/scope barriers; C2 acquires only its fence.
package foundation_test

import "testing"

func TestOrderAttemptModelSubjectAndScopeBarriers(t *testing.T) {
	for _, hold := range []oaModelCommand{
		{op: oaHoldSubjectBarrier, arg: 2},
		{op: oaHoldScope},
	} {
		t.Run("hold="+string(rune('a'+hold.op-oaHoldSubjectBarrier)), func(t *testing.T) {
			m := oaModelTrace(t, []oaModelCommand{hold, {op: oaAdmit}})
			if m.count != 0 {
				t.Fatal("MODEL_ONLY admission crossed its earlier held lock")
			}
			m = oaModelTrace(t, []oaModelCommand{{op: oaAdmit}, hold,
				{op: oaLock, generation: 1}, {op: oaStage, generation: 1, arg: oaEffects},
				{op: oaCommit, generation: 1}})
			if m.count != 1 || m.attempts[0].state != oaCommitted {
				t.Fatal("MODEL_ONLY C2 took an earlier subject or scope lock")
			}
		})
	}
}

func TestOrderAttemptModelRejectsEarlierC2Locks(t *testing.T) {
	m := oaModelTrace(t, []oaModelCommand{{op: oaAdmit}, {op: oaLock, generation: 1}})
	if m.attempts[0].executionLocks != oaModelFenceLock {
		t.Fatal("MODEL_ONLY positive fence-lock sentinel missing")
	}
	// Deliberately corrupt only independent model snapshots, not production code.
	// These injected failures run in CI and must be rejected by the oracle.
	for _, lowerLock := range []uint8{2, 4} {
		bad := m
		bad.attempts[0].executionLocks |= lowerLock
		observer := oaModelObserver{identity: map[int][6]int{}, terminal: map[int]oaModelReply{}}
		if err := observer.check(bad); err == nil {
			t.Fatal("MODEL_ONLY oracle accepted C2 scope/subject acquisition", lowerLock)
		}
	}
}
