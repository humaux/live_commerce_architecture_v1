package foundation_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/command"
	"livecommerce/internal/live"
	"livecommerce/internal/platform"
)

func brwPlanNoIOMap(t *testing.T) (*live.BrowserInputRuntime, *brwRejectTransport) {
	t.Helper()
	transport := &brwRejectTransport{}
	runtime, err := live.NewBrowserInputRuntime([]live.BrowserInputProject{{
		ProjectID: "project_lma", CredentialVersion: 1, Config: lmeConfig(),
		Transport: transport, BrowserURL: "ws://127.0.0.1:7880",
	}})
	if err != nil {
		t.Fatal(err)
	}
	return runtime, transport
}

func TestLiveBrowserInputBRW01PlannerRollbackAndCommitAckLoss(t *testing.T) {
	t.Run("runtime-one-rollback-is-atomic", func(t *testing.T) {
		h := brwRegistered(t)
		runtime, transport := brwPlanNoIOMap(t)
		h.runtime = runtime
		before := bicOwnedFacts(t, h.bicHarness)
		if before != [8]int64{1} {
			t.Fatalf("unexpected registered baseline: %v", before)
		}
		ctx := context.Background()
		abort := errors.New("abort after original input job insertion")
		var planned live.MediaStartResult
		err := platform.WithScope(ctx, h.lp.f.runtime, h.logins.a, h.lp.f.storeA1, "store:read", func(tx pgx.Tx, scope platform.Scope) error {
			var planErr error
			planned, planErr = h.planner.PlanBrowserInputStart(ctx, tx, scope, h.logins.a,
				t04Key("brw-plan-rollback"), h.input, h.runtime)
			if planErr != nil {
				return planErr
			}
			return abort
		})
		if !errors.Is(err, abort) || planned.AttemptID == "" || planned.OperationID == "" || planned.JobID < 1 {
			t.Fatalf("marked plan was not rolled back after insertion: plan=%+v err=%v", planned, err)
		}
		if got := bicOwnedFacts(t, h.bicHarness); got != before {
			t.Fatalf("rollback left attempt, input custody, operation, job or receipt: %v -> %v", before, got)
		}
		var orphan [4]int64
		if err := h.lp.f.owner.QueryRow(ctx, `SELECT
			(SELECT count(*) FROM live.media_attempts WHERE id=$1),
			(SELECT count(*) FROM live.media_input_custody WHERE attempt_id=$1),
			(SELECT count(*) FROM integration.operations WHERE id=$2),
			(SELECT count(*) FROM river_media.river_job WHERE id=$3)`,
			planned.AttemptID, planned.OperationID, planned.JobID).
			Scan(&orphan[0], &orphan[1], &orphan[2], &orphan[3]); err != nil || orphan != [4]int64{} {
			t.Fatalf("rollback retained original identities: facts=%v err=%v", orphan, err)
		}
		var audit int64
		if err := h.lp.f.owner.QueryRow(ctx, `SELECT count(*) FROM ops.audit_events
			WHERE principal_id=$1 AND action='live.media.input.start.planned'`, h.lp.actor).Scan(&audit); err != nil || audit != 0 {
			t.Fatalf("rollback retained planned audit: count=%d err=%v", audit, err)
		}
		if transport.calls.Load() != 0 {
			t.Fatal("planner performed provider I/O before commit")
		}
	})

	t.Run("committed-ack-loss-replays-same-marked-original-job", func(t *testing.T) {
		h := brwRegistered(t)
		runtime, transport := brwPlanNoIOMap(t)
		h.runtime = runtime
		pool, loss := bicCommitAckPool(t, h.lp.f.runtime)
		planner := lmpPlanner(t, pool, "river_media")
		key := t04Key("brw-plan-ack-loss")
		loss.armed.Store(true)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		var preAck live.MediaStartResult
		err := platform.WithScope(ctx, pool, h.logins.a, h.lp.f.storeA1, "store:read", func(tx pgx.Tx, scope platform.Scope) error {
			var planErr error
			preAck, planErr = planner.PlanBrowserInputStart(ctx, tx, scope, h.logins.a, key, h.input, h.runtime)
			return planErr
		})
		if err == nil || !loss.committed.Load() {
			t.Fatalf("COMMIT ACK loss was not injected after durable marked plan: err=%v committed=%t", err, loss.committed.Load())
		}
		if got := bicOwnedFacts(t, h.bicHarness); got != [8]int64{1, 1, 1, 1, 1, 1, 1, 0} {
			t.Fatalf("lost ACK did not retain exactly one original attempt/operation/job: %v", got)
		}
		replayed, err := brwPlan(context.Background(), h, h.lp.f.runtime, h.logins.a,
			h.lp.f.storeA1, key, h.input)
		if err != nil || preAck.AttemptID == "" || preAck.OperationID == "" || preAck.JobID < 1 ||
			replayed.AttemptID == "" || replayed.OperationID == "" || replayed.JobID < 1 || preAck != replayed {
			t.Fatalf("marked receipt replay changed committed identities: pre=%+v replay=%+v err=%v", preAck, replayed, err)
		}
		if got := bicOwnedFacts(t, h.bicHarness); got != [8]int64{1, 1, 1, 1, 1, 1, 1, 0} {
			t.Fatalf("replay duplicated original job or custody: %v", got)
		}
		var marker int
		var steps int64
		var unissued, noExecutionProjection bool
		if err := h.lp.f.owner.QueryRow(context.Background(), `SELECT c.runtime_version,
			c.grant_iat IS NULL,
			NOT EXISTS(SELECT 1 FROM live.media_execution_state x WHERE x.attempt_id=c.attempt_id),
			(SELECT count(*) FROM live.media_input_wire_steps WHERE attempt_id=c.attempt_id)
			FROM live.media_input_custody c
			WHERE c.attempt_id=$1`, replayed.AttemptID).
			Scan(&marker, &unissued, &noExecutionProjection, &steps); err != nil || marker != 1 || !unissued || !noExecutionProjection || steps != 0 {
			t.Fatalf("ACK replay bypassed marker or performed wire work: marker=%d unissued=%t noExecutionProjection=%t steps=%d err=%v",
				marker, unissued, noExecutionProjection, steps, err)
		}
		raw, err := json.Marshal(replayed)
		if err != nil {
			t.Fatalf("planner receipt could not be encoded: %v", err)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			t.Fatalf("planner receipt could not be decoded: %v", err)
		}
		allowed := []string{"session_id", "program_id", "attempt_id", "operation_id", "job_id", "room_name", "state"}
		if len(fields) != len(allowed) {
			t.Fatalf("planner receipt field count changed: got %d, want %d", len(fields), len(allowed))
		}
		for _, field := range allowed {
			if _, ok := fields[field]; !ok {
				t.Fatalf("planner receipt missing permitted field %q", field)
			}
		}
		if transport.calls.Load() != 0 {
			t.Fatal("ACK-loss plan or replay performed provider I/O")
		}
	})

	t.Run("kernel-zero-replay-cannot-upgrade", func(t *testing.T) {
		h := bicSetup(t)
		bicRegister(t, h)
		runtime, transport := brwPlanNoIOMap(t)
		key := t04Key("brw-kernel-zero-replay")
		kernel, err := bicPlan(context.Background(), h, h.lp.f.runtime, h.planner,
			h.logins.a, h.lp.f.storeA1, key, h.input)
		if err != nil {
			t.Fatal(err)
		}
		before := bicOwnedFacts(t, h)
		if _, err := h.registrar.Exec(context.Background(),
			`SELECT live.register_media_input_runtime_profile($1::uuid)`, h.input.AuthorizationID); sqlState(err) != "MP409" {
			t.Fatalf("registrar rejection must be MP409 for existing kernel-only attempt: err=%v", err)
		}
		err = platform.WithScope(context.Background(), h.lp.f.runtime, h.logins.a, h.lp.f.storeA1,
			"store:read", func(tx pgx.Tx, scope platform.Scope) error {
				_, planErr := h.planner.PlanBrowserInputStart(context.Background(), tx, scope, h.logins.a,
					key, h.input, runtime)
				return planErr
			})
		if !errors.Is(err, command.ErrConflict) || bicOwnedFacts(t, h) != before {
			t.Fatalf("browser planner replayed/upgraded marker 0: err=%v facts=%v -> %v", err, before, bicOwnedFacts(t, h))
		}
		var marker int
		if err := h.lp.f.owner.QueryRow(context.Background(), `SELECT runtime_version FROM live.media_input_custody
			WHERE attempt_id=$1`, kernel.AttemptID).Scan(&marker); err != nil || marker != 0 {
			t.Fatalf("kernel-only attempt marker changed: marker=%d err=%v", marker, err)
		}
		if transport.calls.Load() != 0 {
			t.Fatal("marker-0 denial performed provider I/O")
		}
	})
}
