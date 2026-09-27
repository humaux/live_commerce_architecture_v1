package foundation_test

import (
	"bytes"
	"context"
	"testing"
	"time"
)

// brwObservedActive establishes a real original-job Egress projection through
// executor SQL. The mock START observation seeds these selection tests; it is
// not evidence that a provider accepted a production Start.
func brwObservedActive(t *testing.T) *brwWireFixture {
	t.Helper()
	f := brwWireStarted(t)
	brwReserveGrant(t, f.h)
	lease, err := brwClaim(context.Background(), f.h.bicHarness, f.h.plan, bytes.Repeat([]byte{0x71}, 32))
	if err != nil || lease.disposition != "claimed" || lease.mode != "reconcile" {
		t.Fatalf("original job input claim: %+v err=%v", lease, err)
	}
	brwSelectObserve(t, f.h, lease)
	if _, err := brwReserveStart(context.Background(), f.h, lease, f.h.plan.RoomName,
		f.h.grant.PublisherIdentity, "PA_brw", "ACTIVE", true, false, true, false); err != nil {
		t.Fatalf("input-observed Start reservation: %v", err)
	}
	var result string
	if err := f.h.executor.QueryRow(context.Background(), `SELECT live.record_media_observation(
		$1::uuid,$2::bigint,$3::bytea,'START','EG_brw',$4::text,
		'EGRESS_ACTIVE',100::bigint,110::bigint,0::bigint)`,
		f.h.plan.OperationID, lease.generation, lease.key, f.h.plan.RoomName).Scan(&result); err != nil || result != "observe" {
		t.Fatalf("active Egress projection: result=%q err=%v", result, err)
	}
	return f
}

func TestLiveBrowserInputBRW04InputHoldStillReconcilesEgress(t *testing.T) {
	f := brwObservedActive(t)
	if _, err := bicStop(context.Background(), f.h.bicHarness, f.h.logins.b,
		t04Key("brw-input-held-stop"), f.h.plan); err != nil {
		t.Fatal(err)
	}
	// Owner-only legal hold seed isolates the selector/actual worker path. It
	// does not prove the production path that arrives at unsafe_retry.
	tag, err := f.h.lp.f.owner.Exec(context.Background(), `UPDATE live.media_input_custody
		SET input_cleanup_held_at=clock_timestamp(),input_cleanup_hold_reason='unsafe_retry'
		WHERE attempt_id=$1 AND admission_closed_at IS NOT NULL AND runtime_version=1`, f.h.plan.AttemptID)
	if err != nil || tag.RowsAffected() != 1 {
		t.Fatalf("input hold seed missed issued custody: rows=%d err=%v", tag.RowsAffected(), err)
	}
	reads := &brwEgressReadCounter{base: f.egress.Transport}
	f.egress.Transport = reads
	brwStartWorker(t, f.h.bicHarness, f.keys, f.egress, f.h.runtime)
	deadline := time.Now().Add(15 * time.Second)
	var queries int64
	for time.Now().Before(deadline) {
		if err := f.h.lp.f.owner.QueryRow(context.Background(), `SELECT count(*) FROM live.media_observations
			WHERE operation_id=$1 AND source='QUERY'`, f.h.plan.OperationID).Scan(&queries); err != nil {
			t.Fatal(err)
		}
		if reads.reads.Load() > 0 && queries > 0 {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if reads.reads.Load() == 0 || queries == 0 {
		t.Fatalf("input hold starved actual Egress Query/reconcile: reads=%d observations=%d", reads.reads.Load(), queries)
	}
	var jobID int64
	var state, child, resource, holdReason string
	var finalized *time.Time
	if err := f.h.lp.f.owner.QueryRow(context.Background(), `SELECT o.job_id,j.state,j.finalized_at,
		c.state,c.input_cleanup_hold_reason,x.resource_state
		FROM integration.operations o JOIN river_media.river_job j ON j.id=o.job_id
		JOIN live.media_input_custody c ON c.operation_id=o.id
		JOIN live.media_execution_state x ON x.operation_id=o.id WHERE o.id=$1`, f.h.plan.OperationID).
		Scan(&jobID, &state, &finalized, &child, &holdReason, &resource); err != nil ||
		jobID != f.h.plan.JobID || finalized != nil || state == "completed" || state == "cancelled" ||
		state == "discarded" || child == "CLOSED" || holdReason != "unsafe_retry" || resource == "TERMINAL" {
		t.Fatalf("input hold lost independent Egress/original job: job=%d/%d %s finalized=%v child=%s hold=%s resource=%s err=%v",
			jobID, f.h.plan.JobID, state, finalized, child, holdReason, resource, err)
	}
}

func TestLiveBrowserInputBRW04EgressHoldStillCleansInput(t *testing.T) {
	f := brwObservedActive(t)
	if _, err := bicStop(context.Background(), f.h.bicHarness, f.h.logins.b,
		t04Key("brw-egress-held-stop"), f.h.plan); err != nil {
		t.Fatal(err)
	}
	// Owner-only legal escalation seed isolates input selection; it does not
	// prove the bounded production escalation transition.
	tag, err := f.h.lp.f.owner.Exec(context.Background(), `UPDATE live.media_execution_state
		SET escalated_at=clock_timestamp(),escalation_code='reconcile_exhausted'
		WHERE operation_id=$1 AND wire_reserved_at IS NOT NULL AND resource_state<>'TERMINAL'`, f.h.plan.OperationID)
	if err != nil || tag.RowsAffected() != 1 {
		t.Fatalf("Egress escalation seed missed active projection: rows=%d err=%v", tag.RowsAffected(), err)
	}
	reads := &brwEgressReadCounter{base: f.egress.Transport}
	f.egress.Transport = reads
	brwStartWorker(t, f.h.bicHarness, f.keys, f.egress, f.h.runtime)
	deadline := time.Now().Add(15 * time.Second)
	var recorded int64
	for time.Now().Before(deadline) {
		if err := f.h.lp.f.owner.QueryRow(context.Background(), `SELECT count(*) FROM live.media_input_wire_steps
			WHERE operation_id=$1 AND ordinal=1 AND action='REMOVE' AND result='ACK'`, f.h.plan.OperationID).
			Scan(&recorded); err != nil {
			t.Fatal(err)
		}
		if f.removes.Load() > 0 && recorded == 1 {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if f.removes.Load() == 0 || recorded != 1 || reads.reads.Load() != 0 {
		t.Fatalf("Egress hold starved actual input cleanup: removes=%d recorded=%d Egress reads=%d",
			f.removes.Load(), recorded, reads.reads.Load())
	}
	var jobID int64
	var state, child, escalation string
	var finalized *time.Time
	if err := f.h.lp.f.owner.QueryRow(context.Background(), `SELECT o.job_id,j.state,j.finalized_at,
		c.state,x.escalation_code FROM integration.operations o
		JOIN river_media.river_job j ON j.id=o.job_id
		JOIN live.media_input_custody c ON c.operation_id=o.id
		JOIN live.media_execution_state x ON x.operation_id=o.id WHERE o.id=$1`, f.h.plan.OperationID).
		Scan(&jobID, &state, &finalized, &child, &escalation); err != nil ||
		jobID != f.h.plan.JobID || finalized != nil || state == "completed" || state == "cancelled" ||
		state == "discarded" || child == "CLOSED" || escalation != "reconcile_exhausted" {
		t.Fatalf("Egress hold lost independent input/original job: job=%d/%d %s finalized=%v child=%s escalation=%s err=%v",
			jobID, f.h.plan.JobID, state, finalized, child, escalation, err)
	}
}
