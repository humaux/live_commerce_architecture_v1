// Purpose: MODEL_ONLY subject/scope lock traces and independent lower-lock oracle.
// Depends on: the value-state intent model, frozen order-attempt-v1 §6 lock rules.
// Used by: CI fixed a–h traces and testing.F corpus; never product or REAL_PG proof.
// Invariants: admission observes subject/scope barriers; C2 acquires only its fence.
package foundation_test

import "testing"

// These fixed traces cover the local authority portion of a–h. Case g stops at
// native cancellation (provider/link outcomes are OA2); h tests the generation
// prerequisite, not OA3's versioned HTTP adapter. Neither is dispatch proof.
func TestOrderAttemptModelContractSeedsAH(t *testing.T) {
	admit := oaModelCommand{op: oaAdmit}
	lock := oaModelCommand{op: oaLock, generation: 1}
	stage := oaModelCommand{op: oaStage, generation: 1, arg: oaEffects}
	commit := oaModelCommand{op: oaCommit, generation: 1}
	for _, tc := range []struct {
		name string
		commands []oaModelCommand
		state, count int
	}{
		{"a_crash_resume", []oaModelCommand{admit, lock, stage, {op: oaCrash}, lock, stage, commit}, oaCommitted, 1},
		{"b_cross_client", []oaModelCommand{admit, {op: oaAdmit, key: 1}, lock, stage, commit, {op: oaAdmit, key: 1}}, oaCommitted, 1},
		{"c_lost_recovery_ack", []oaModelCommand{admit, lock, stage, commit, {op: oaLoseAck}, {op: oaReplay}}, oaCommitted, 1},
		{"d_rotate_original_owner", []oaModelCommand{admit, {op: oaRotate}, lock, stage, commit}, oaCommitted, 1},
		{"e_erase_subject_then_CAS", []oaModelCommand{admit, {op: oaErase, arg: 2}, lock, {op: oaAdmit, generation: 1, key: 1}}, oaAborted, 2},
		{"f_cancel_terminal_replay", []oaModelCommand{admit, lock, stage, commit, {op: oaCancel, generation: 1}, {op: oaReplay}}, oaCommitted, 1},
		{"g_local_cancel_notification_deferred", []oaModelCommand{admit, lock, stage, commit, {op: oaCancel, generation: 1}, {op: oaCancel, generation: 1}}, oaCommitted, 1},
		{"h_generation_required_HTTP_deferred", []oaModelCommand{admit, lock, stage, commit, {op: oaAdmit, key: 1}, {op: oaAdmit, generation: 1, key: 1}}, oaCommitted, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := oaModelTrace(t, tc.commands)
			if m.count != tc.count || m.attempts[0].state != tc.state || m.attempts[0].owner != 100 {
				t.Fatal("MODEL_ONLY fixed local authority seed failed", tc.name)
			}
		})
	}
}

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
