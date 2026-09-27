package foundation_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/integrations/livekit"
	"livecommerce/internal/live"
	"livecommerce/internal/platform"
)

// BRW fixtures use actual PG roles and the original BIC operation/job. They
// exercise SQL authority without treating owner-seeded state as provider proof.
type brwHarness struct {
	*bicHarness
	runtime *live.BrowserInputRuntime
	planKey string
	plan    live.MediaStartResult
	grant   live.MediaInputGrant
}

func brwRegistered(t *testing.T) *brwHarness {
	t.Helper()
	h := &brwHarness{bicHarness: bicSetup(t)}
	bicRegister(t, h.bicHarness)
	if _, err := h.registrar.Exec(context.Background(),
		`SELECT live.register_media_input_runtime_profile($1::uuid)`, h.input.AuthorizationID); err != nil {
		t.Fatalf("runtime-1 registrar profile: %v", err)
	}
	// This cleanup runs before bicSetup's custody/profile teardown.
	t.Cleanup(func() {
		if _, err := h.lp.f.owner.Exec(context.Background(),
			`DELETE FROM live.prepared_media_input_runtime_profiles WHERE authorization_id=$1`, h.input.AuthorizationID); err != nil {
			t.Errorf("runtime profile fixture cleanup: %v", err)
		}
	})
	var err error
	h.runtime, err = live.NewBrowserInputRuntime([]live.BrowserInputProject{{
		ProjectID: "project_lma", CredentialVersion: 1,
		Config: lmeConfig(), Transport: lmeTransport("127.0.0.1:1"),
		BrowserURL: "ws://127.0.0.1:7880",
	}})
	if err != nil {
		t.Fatalf("typed local runtime mapping: %v", err)
	}
	return h
}

func brwStart(t *testing.T) *brwHarness {
	t.Helper()
	h := brwRegistered(t)
	h.planKey = t04Key("brw-start")
	var err error
	h.plan, err = brwPlan(context.Background(), h, h.lp.f.runtime, h.logins.a,
		h.lp.f.storeA1, h.planKey, h.input)
	if err != nil {
		t.Fatalf("runtime-1 original-job plan: %v", err)
	}
	if h.plan.OperationID == "" || h.plan.JobID < 1 || h.plan.AttemptID == "" {
		t.Fatalf("runtime-1 plan lost original operation/job: %+v", h.plan)
	}
	var marker int
	var queue string
	if err := h.lp.f.owner.QueryRow(context.Background(), `SELECT c.runtime_version,j.queue
		FROM live.media_input_custody c JOIN integration.operations o ON o.id=c.operation_id
		JOIN river_media.river_job j ON j.id=o.job_id WHERE c.attempt_id=$1`,
		h.plan.AttemptID).Scan(&marker, &queue); err != nil || marker != 1 || queue != "media_input_mock_v1" {
		t.Fatalf("original input job/marker mismatch: marker=%d queue=%q err=%v", marker, queue, err)
	}
	return h
}

func brwPlan(ctx context.Context, h *brwHarness, pool *pgxpool.Pool,
	token, store, key string, in live.MediaStartInput) (live.MediaStartResult, error) {
	var out live.MediaStartResult
	err := platform.WithScope(ctx, pool, token, store, "store:read", func(tx pgx.Tx, scope platform.Scope) error {
		var err error
		out, err = h.planner.PlanBrowserInputStart(ctx, tx, scope, token, key, in, h.runtime)
		return err
	})
	return out, err
}

type brwLease struct {
	disposition, mode string
	generation        int64
	key               []byte
}

func brwClaim(ctx context.Context, h *bicHarness, plan live.MediaStartResult, key []byte) (brwLease, error) {
	l := brwLease{key: key}
	err := h.executor.QueryRow(ctx, `SELECT disposition,generation,mode
		FROM live.claim_browser_input_operation($1::uuid,$2::bigint,$3::integer,$4::bytea)`,
		plan.OperationID, plan.JobID, 30, key).Scan(&l.disposition, &l.generation, &l.mode)
	return l, err
}

func brwSelectObserve(t *testing.T, h *brwHarness, lease brwLease) {
	t.Helper()
	var turn string
	if err := h.executor.QueryRow(context.Background(),
		`SELECT live.next_media_input_turn($1::uuid,$2::bigint,$3::bytea)`,
		h.plan.OperationID, lease.generation, lease.key).Scan(&turn); err != nil || turn != "INPUT_OBSERVE" {
		t.Fatalf("final Start claim did not persist INPUT_OBSERVE turn: %q %v", turn, err)
	}
}

func brwAssertNativePending(t *testing.T, h *brwHarness, wantOperation string) {
	t.Helper()
	if _, err := h.lp.f.owner.Exec(context.Background(),
		`UPDATE river_media.river_job SET state='completed',finalized_at=clock_timestamp() WHERE id=$1`,
		h.plan.JobID); err != nil {
		t.Fatalf("native pending rewrite rejected unresolved input: %v", err)
	}
	var jobState string
	var finalized *time.Time
	if err := h.lp.f.owner.QueryRow(context.Background(),
		`SELECT state,finalized_at FROM river_media.river_job WHERE id=$1`, h.plan.JobID).
		Scan(&jobState, &finalized); err != nil || jobState != "pending" || finalized != nil {
		t.Fatalf("native completion escaped pending guard: job=%s finalized=%v err=%v", jobState, finalized, err)
	}
	var operation, child string
	if err := h.lp.f.owner.QueryRow(context.Background(), `SELECT o.state,c.state
		FROM integration.operations o JOIN live.media_input_custody c ON c.operation_id=o.id
		WHERE o.id=$1`, h.plan.OperationID).Scan(&operation, &child); err != nil ||
		operation != wantOperation || child == "CLOSED" {
		t.Fatalf("native pending rewrite lost original liability: operation=%s child=%s err=%v",
			operation, child, err)
	}
}

func brwReserveGrant(t *testing.T, h *brwHarness) {
	t.Helper()
	var err error
	h.grant, err = bicReserve(context.Background(), h.bicHarness, h.lp.f.runtime,
		h.logins.a, h.lp.f.storeA1, t04Key("brw-grant"), live.MediaInputReserveInput{
			SessionID: h.session, AttemptID: h.plan.AttemptID, ExpectedSessionVersion: 1,
		})
	if err != nil {
		t.Fatalf("runtime-1 nonsecret grant: %v", err)
	}
	bicGrantMap(t, h.grant)
}

func brwReserveStart(ctx context.Context, h *brwHarness, lease brwLease,
	room, identity, sid, state string, camera, cameraMuted, microphone, microphoneMuted bool) (map[string]any, error) {
	var raw []byte
	err := h.executor.QueryRow(ctx, `SELECT live.reserve_media_input_start(
		$1::uuid,$2::bigint,$3::bytea,$4::text,$5::text,$6::text,$7::text,
		$8::boolean,$9::boolean,$10::boolean,$11::boolean)`,
		h.plan.OperationID, lease.generation, lease.key, room, identity, sid, state,
		camera, cameraMuted, microphone, microphoneMuted).Scan(&raw)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func brwWireFacts(t *testing.T, h *brwHarness) (generation int64, leaseUntil *time.Time, reserved *time.Time) {
	t.Helper()
	if err := h.lp.f.owner.QueryRow(context.Background(), `SELECT o.generation,o.lease_until,x.wire_reserved_at
		FROM integration.operations o JOIN live.media_execution_state x ON x.operation_id=o.id
		WHERE o.id=$1`, h.plan.OperationID).Scan(&generation, &leaseUntil, &reserved); err != nil {
		t.Fatal(err)
	}
	return
}

func TestLiveBrowserInputBRW01MarkerRoleAndOldClaimIsolation(t *testing.T) {
	t.Run("marker-zero-is-kernel-only", func(t *testing.T) {
		h, plan, _ := bicStarted(t)
		before, err := brwClaim(context.Background(), h, plan, bytes.Repeat([]byte{0x11}, 32))
		if err != nil || before.disposition != "kernel_only" || before.generation != 0 || before.mode != "" {
			t.Fatalf("marker-0 new claim granted wire lease: %+v err=%v", before, err)
		}
		var generation int64
		var leaseUntil *time.Time
		if err := h.lp.f.owner.QueryRow(context.Background(),
			`SELECT generation,lease_until FROM integration.operations WHERE id=$1`, plan.OperationID).
			Scan(&generation, &leaseUntil); err != nil || generation != 0 || leaseUntil != nil {
			t.Fatalf("marker-0 claim wrote generation/lease: %d %v %v", generation, leaseUntil, err)
		}
		if _, err := h.executor.Exec(context.Background(),
			`SELECT live.load_media_input_material($1::uuid,$2::bigint,$3::bytea)`,
			plan.OperationID, int64(0), bytes.Repeat([]byte{0x11}, 32)); err == nil {
			t.Fatal("kernel-only input loaded wire material")
		}
	})
	t.Run("runtime-one-rejects-old-claim-and-is-unleased-before-grant", func(t *testing.T) {
		h := brwStart(t)
		if _, _, _, err := bicClaim(context.Background(), h.bicHarness, h.plan, bytes.Repeat([]byte{0x12}, 32)); err == nil {
			t.Fatal("old BIC claim accepted runtime-1 marker")
		}
		claim, err := brwClaim(context.Background(), h.bicHarness, h.plan, bytes.Repeat([]byte{0x13}, 32))
		if err != nil || claim.disposition != "await_admission" || claim.generation != 0 || claim.mode != "" {
			t.Fatalf("unissued runtime-1 claim granted provider lease: %+v err=%v", claim, err)
		}
		generation, leaseUntil, reserved := brwWireFacts(t, h)
		if generation != 0 || leaseUntil != nil || reserved != nil {
			t.Fatalf("unissued marker changed wire state: %d %v %v", generation, leaseUntil, reserved)
		}
		for _, item := range []struct {
			role, signature string
			want            bool
		}{
			{"commerce_media_registrar", "live.register_media_input_runtime_profile(uuid)", true},
			{"commerce_runtime", "live.register_media_input_runtime_profile(uuid)", false},
			{"commerce_media_executor", "live.claim_browser_input_operation(uuid,bigint,integer,bytea)", true},
			{"commerce_runtime", "live.claim_browser_input_operation(uuid,bigint,integer,bytea)", false},
			{"commerce_media_worker", "live.reserve_media_input_start(uuid,bigint,bytea,text,text,text,text,boolean,boolean,boolean,boolean)", false},
			{"commerce_media_executor", "live.reserve_media_input_start(uuid,bigint,bytea,text,text,text,text,boolean,boolean,boolean,boolean)", true},
		} {
			var allowed bool
			if err := h.lp.f.owner.QueryRow(context.Background(),
				`SELECT has_function_privilege($1,$2::regprocedure,'EXECUTE')`, item.role, item.signature).
				Scan(&allowed); err != nil || allowed != item.want {
				t.Fatalf("actual role ACL %s %s: allowed=%t err=%v", item.role, item.signature, allowed, err)
			}
		}
		var ready bool
		if err := h.worker.QueryRow(context.Background(), `SELECT live.media_browser_input_worker_ready()`).Scan(&ready); err != nil || !ready {
			t.Fatalf("actual worker role not ready: ready=%t err=%v", ready, err)
		}
		for _, item := range []struct{ name, grant, revoke string }{
			{"private-helper-public",
				`GRANT EXECUTE ON FUNCTION live.browser_input_next_action(uuid) TO PUBLIC`,
				`REVOKE EXECUTE ON FUNCTION live.browser_input_next_action(uuid) FROM PUBLIC`},
			{"executor-step-to-runtime",
				`GRANT EXECUTE ON FUNCTION live.reserve_media_input_cleanup(uuid,bigint,bytea) TO commerce_runtime`,
				`REVOKE EXECUTE ON FUNCTION live.reserve_media_input_cleanup(uuid,bigint,bytea) FROM commerce_runtime`},
			{"private-child-column-read",
				`GRANT SELECT(ordinal) ON live.media_input_wire_steps TO commerce_runtime`,
				`REVOKE SELECT(ordinal) ON live.media_input_wire_steps FROM commerce_runtime`},
		} {
			t.Run(item.name, func(t *testing.T) {
				mustExec(t, h.lp.f.owner, item.grant)
				t.Cleanup(func() { _, _ = h.lp.f.owner.Exec(context.Background(), item.revoke) })
				if err := h.worker.QueryRow(context.Background(), `SELECT live.media_browser_input_worker_ready()`).Scan(&ready); err != nil || ready {
					t.Fatalf("readiness accepted %s authority poison: ready=%t err=%v", item.name, ready, err)
				}
				mustExec(t, h.lp.f.owner, item.revoke)
				if err := h.worker.QueryRow(context.Background(), `SELECT live.media_browser_input_worker_ready()`).Scan(&ready); err != nil || !ready {
					t.Fatalf("readiness failed restoration after %s: ready=%t err=%v", item.name, ready, err)
				}
			})
		}
	})
}

func TestLiveBrowserInputBRW01MappingAndReplayRemainCurrent(t *testing.T) {
	t.Run("mapping-is-required-before-plan", func(t *testing.T) {
		h := brwRegistered(t)
		before := bicOwnedFacts(t, h.bicHarness)
		h.runtime = nil
		if _, err := brwPlan(context.Background(), h, h.lp.f.runtime, h.logins.a,
			h.lp.f.storeA1, t04Key("brw-nil-map"), h.input); err == nil {
			t.Fatal("nil local binding planned a runtime-1 input")
		}
		if got := bicOwnedFacts(t, h.bicHarness); got != before {
			t.Fatalf("missing mapping left attempt/job/receipt artifacts: %v -> %v", before, got)
		}
		config := lmeConfig()
		config.Endpoint = "https://other.livekit.cloud"
		var err error
		h.runtime, err = live.NewBrowserInputRuntime([]live.BrowserInputProject{{
			ProjectID: "project_lma", CredentialVersion: 1, Config: config,
			Transport: lmeTransport("127.0.0.1:1"), BrowserURL: "ws://127.0.0.1:7880",
		}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := brwPlan(context.Background(), h, h.lp.f.runtime, h.logins.a,
			h.lp.f.storeA1, t04Key("brw-wrong-endpoint"), h.input); err == nil {
			t.Fatal("wrong endpoint mapping planned a runtime-1 input")
		}
		if got := bicOwnedFacts(t, h.bicHarness); got != before {
			t.Fatalf("wrong mapping left attempt/job/receipt artifacts: %v -> %v", before, got)
		}
	})
	t.Run("replay-rechecks-binding", func(t *testing.T) {
		h := brwStart(t)
		before := bicOwnedFacts(t, h.bicHarness)
		original := h.runtime
		h.runtime = nil
		if _, err := brwPlan(context.Background(), h, h.lp.f.runtime, h.logins.a,
			h.lp.f.storeA1, h.planKey, h.input); err == nil {
			t.Fatal("replay bypassed current local binding")
		}
		h.runtime = original
		replayed, err := brwPlan(context.Background(), h, h.lp.f.runtime, h.logins.a,
			h.lp.f.storeA1, h.planKey, h.input)
		if err != nil || replayed != h.plan {
			t.Fatalf("exact replay changed original receipt: got=%+v want=%+v err=%v", replayed, h.plan, err)
		}
		if got := bicOwnedFacts(t, h.bicHarness); got != before {
			t.Fatalf("replay duplicated original artifacts: %v -> %v", before, got)
		}
	})
}

func TestLiveBrowserInputBRW02DualTrackSingleStartAndStop(t *testing.T) {
	for _, item := range []struct {
		name, room, identity, sid, state                 string
		camera, cameraMuted, microphone, microphoneMuted bool
	}{
		{"wrong-room", "lc_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "", "PA_brw", "ACTIVE", true, false, true, false},
		{"wrong-identity", "", "lcp_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "PA_brw", "ACTIVE", true, false, true, false},
		{"not-joined", "", "", "PA_brw", "JOINING", true, false, true, false},
		{"bad-sid", "", "", "wrong", "ACTIVE", true, false, true, false},
		{"missing-camera", "", "", "PA_brw", "ACTIVE", false, false, true, false},
		{"muted-camera", "", "", "PA_brw", "ACTIVE", true, true, true, false},
		{"missing-microphone", "", "", "PA_brw", "ACTIVE", true, false, false, false},
		{"muted-microphone", "", "", "PA_brw", "ACTIVE", true, false, true, true},
	} {
		t.Run(item.name, func(t *testing.T) {
			h := brwStart(t)
			brwReserveGrant(t, h)
			lease, err := brwClaim(context.Background(), h.bicHarness, h.plan, bytes.Repeat([]byte{0x21}, 32))
			if err != nil || lease.disposition != "claimed" || lease.mode != "reconcile" {
				t.Fatalf("reserved claim unavailable: %+v err=%v", lease, err)
			}
			brwSelectObserve(t, h, lease)
			room, identity := item.room, item.identity
			if room == "" {
				room = h.plan.RoomName
			}
			if identity == "" {
				identity = h.grant.PublisherIdentity
			}
			if _, err := brwReserveStart(context.Background(), h, lease, room, identity, item.sid, item.state,
				item.camera, item.cameraMuted, item.microphone, item.microphoneMuted); err == nil {
				t.Fatal("invalid exact dual-track observation reserved Start")
			}
			_, _, reserved := brwWireFacts(t, h)
			if reserved != nil {
				t.Fatal("invalid observation left wire reservation")
			}
		})
	}
	t.Run("single-reservation-survives-stop", func(t *testing.T) {
		h := brwStart(t)
		brwReserveGrant(t, h)
		lease, err := brwClaim(context.Background(), h.bicHarness, h.plan, bytes.Repeat([]byte{0x22}, 32))
		if err != nil || lease.disposition != "claimed" || lease.mode != "reconcile" {
			t.Fatalf("reserved claim unavailable: %+v err=%v", lease, err)
		}
		brwSelectObserve(t, h, lease)
		material, err := brwReserveStart(context.Background(), h, lease, h.plan.RoomName,
			h.grant.PublisherIdentity, "PA_brw", "ACTIVE", true, false, true, false)
		if err != nil || material["mode"] != "dispatch" || len(material) != 15 {
			t.Fatalf("valid dual-track input did not reserve exact legacy Start material: %v %v", material, err)
		}
		if _, err := brwReserveStart(context.Background(), h, lease, h.plan.RoomName,
			h.grant.PublisherIdentity, "PA_brw", "ACTIVE", true, false, true, false); err == nil {
			t.Fatal("same generation obtained second Start reservation")
		}
		if _, err := bicStop(context.Background(), h.bicHarness, h.logins.b,
			t04Key("brw-stop-after-wire"), h.plan); err != nil {
			t.Fatal(err)
		}
		var operation, jobState, child string
		var jobID int64
		var leaseUntil, finalized, stopRequested *time.Time
		if err := h.lp.f.owner.QueryRow(context.Background(), `SELECT o.state,o.job_id,j.state,
			j.finalized_at,o.lease_until,x.stop_requested_at,c.state
			FROM integration.operations o JOIN river_media.river_job j ON j.id=o.job_id
			JOIN live.media_execution_state x ON x.operation_id=o.id
			JOIN live.media_input_custody c ON c.operation_id=o.id WHERE o.id=$1`, h.plan.OperationID).
			Scan(&operation, &jobID, &jobState, &finalized, &leaseUntil, &stopRequested, &child); err != nil ||
			operation != "DISPATCHING" || jobID != h.plan.JobID || jobState == "completed" ||
			finalized != nil || leaseUntil == nil || stopRequested == nil || child == "CLOSED" {
			t.Fatalf("post-reservation Stop lost sticky dual liability: op=%s job=%d/%s finalized=%v lease=%v stop=%v child=%s err=%v",
				operation, jobID, jobState, finalized, leaseUntil, stopRequested, child, err)
		}
		brwAssertNativePending(t, h, "DISPATCHING")
		if _, err := h.lp.f.owner.Exec(context.Background(),
			`DELETE FROM river_media.river_job WHERE id=$1`, h.plan.JobID); sqlState(err) != "22023" {
			t.Fatalf("native guard deleted unresolved original job: %v", err)
		}
	})
	t.Run("stop-before-final-reservation", func(t *testing.T) {
		h := brwStart(t)
		brwReserveGrant(t, h)
		lease, err := brwClaim(context.Background(), h.bicHarness, h.plan, bytes.Repeat([]byte{0x23}, 32))
		if err != nil || lease.disposition != "claimed" {
			t.Fatalf("claim: %+v %v", lease, err)
		}
		brwSelectObserve(t, h, lease)
		if _, err := bicStop(context.Background(), h.bicHarness, h.logins.b,
			t04Key("brw-stop-before-wire"), h.plan); err != nil {
			t.Fatal(err)
		}
		if _, err := brwReserveStart(context.Background(), h, lease, h.plan.RoomName,
			h.grant.PublisherIdentity, "PA_brw", "ACTIVE", true, false, true, false); err == nil {
			t.Fatal("committed prewire Stop allowed Start")
		}
		_, _, reserved := brwWireFacts(t, h)
		if reserved != nil {
			t.Fatal("Stop-before-wire left Start reservation")
		}
		bicAssertOriginalLiability(t, h.bicHarness, h.plan)
	})
}

func brwClosedGrant(t *testing.T) *brwHarness {
	t.Helper()
	h := brwStart(t)
	brwReserveGrant(t, h)
	if _, err := bicStop(context.Background(), h.bicHarness, h.logins.b,
		t04Key("brw-cleanup-stop"), h.plan); err != nil {
		t.Fatalf("close issued input admission: %v", err)
	}
	bicAssertOriginalLiability(t, h.bicHarness, h.plan)
	return h
}

func brwCleanupStep(t *testing.T, h *brwHarness, number int) (brwLease, map[string]any) {
	t.Helper()
	lease, err := brwClaim(context.Background(), h.bicHarness, h.plan,
		bytes.Repeat([]byte{byte(0x30 + number)}, 32))
	if err != nil || lease.disposition != "claimed" || lease.mode != "reconcile" {
		t.Fatalf("cleanup step %d claim: %+v err=%v", number, lease, err)
	}
	var turn string
	if err := h.executor.QueryRow(context.Background(),
		`SELECT live.next_media_input_turn($1::uuid,$2::bigint,$3::bytea)`,
		h.plan.OperationID, lease.generation, lease.key).Scan(&turn); err != nil || turn != "INPUT_CLEANUP" {
		t.Fatalf("cleanup step %d selected %q: %v", number, turn, err)
	}
	var raw []byte
	if err := h.executor.QueryRow(context.Background(),
		`SELECT live.reserve_media_input_cleanup($1::uuid,$2::bigint,$3::bytea)`,
		h.plan.OperationID, lease.generation, lease.key).Scan(&raw); err != nil {
		t.Fatalf("cleanup step %d reserve: %v", number, err)
	}
	if len(raw) == 0 {
		t.Fatalf("cleanup step %d unexpectedly held before reservation", number)
	}
	var step map[string]any
	if err := json.Unmarshal(raw, &step); err != nil {
		t.Fatal(err)
	}
	if len(step) != 8 || step["ordinal"] != float64(number) ||
		step["room_name"] != h.plan.RoomName || step["publisher_identity"] != h.grant.PublisherIdentity ||
		step["project_id"] != "project_lma" || step["endpoint_identity"] != "https://unit.livekit.cloud" ||
		step["credential_version"] != float64(1) {
		t.Fatalf("cleanup step %d lost exact bounded target: %v", number, step)
	}
	if number == 1 || number == 5 {
		if cutoff, ok := step["revoke_before"].(float64); !ok || cutoff <= 0 {
			t.Fatalf("Remove step %d lacks DB cutoff: %v", number, step)
		}
	} else if step["revoke_before"] != nil {
		t.Fatalf("non-Remove step %d has cutoff: %v", number, step)
	}
	return lease, step
}

func brwRecordCleanup(t *testing.T, h *brwHarness, lease brwLease, ordinal int, result, sid string) string {
	t.Helper()
	var disposition string
	if err := h.executor.QueryRow(context.Background(),
		`SELECT live.record_media_input_cleanup($1::uuid,$2::bigint,$3::bytea,$4::integer,$5::text,$6::text)`,
		h.plan.OperationID, lease.generation, lease.key, ordinal, result, sid).Scan(&disposition); err != nil {
		t.Fatalf("record cleanup %d %s: %v", ordinal, result, err)
	}
	if disposition != "observe" && disposition != "held" && disposition != "terminal" {
		t.Fatalf("noncanonical cleanup disposition: %q", disposition)
	}
	return disposition
}

func TestLiveBrowserInputBRW03BoundedCleanupAndPendingLoss(t *testing.T) {
	t.Run("pending-mutation-is-consumed-after-lease-loss", func(t *testing.T) {
		h := brwClosedGrant(t)
		lease, step := brwCleanupStep(t, h, 1)
		if step["action"] != "REMOVE" {
			t.Fatalf("first cleanup was not Remove: %v", step)
		}
		// Owner advances only this disposable fixture's lease clock. The
		// unacknowledged reservation must consume ordinal 1 forever.
		mustExec(t, h.lp.f.owner,
			`UPDATE integration.operations SET lease_until=clock_timestamp()-interval '1 second' WHERE id=$1`,
			h.plan.OperationID)
		next, step2 := brwCleanupStep(t, h, 2)
		if step2["action"] != "DELETE_ROOM" || next.generation <= lease.generation {
			t.Fatalf("lost first result repeated mutation: next=%+v step=%v", next, step2)
		}
		if got := brwRecordCleanup(t, h, next, 2, "UNKNOWN", ""); got != "observe" && got != "held" {
			t.Fatalf("unknown Delete falsely completed: %q", got)
		}
		if _, err := h.executor.Exec(context.Background(),
			`SELECT live.record_media_input_cleanup($1::uuid,$2::bigint,$3::bytea,1,'ACK','')`,
			h.plan.OperationID, lease.generation, lease.key); err == nil {
			t.Fatal("old generation accepted late result for consumed Remove")
		}
		var count, maxOrdinal int
		if err := h.lp.f.owner.QueryRow(context.Background(),
			`SELECT count(*),max(ordinal) FROM live.media_input_wire_steps WHERE attempt_id=$1`,
			h.plan.AttemptID).Scan(&count, &maxOrdinal); err != nil || count != 2 || maxOrdinal != 2 {
			t.Fatalf("pending reservation was refunded: count=%d max=%d err=%v", count, maxOrdinal, err)
		}
		bicAssertOriginalLiability(t, h.bicHarness, h.plan)
	})
	t.Run("all-eight-slots-no-third-mutation", func(t *testing.T) {
		h := brwClosedGrant(t)
		wantActions := []string{"REMOVE", "DELETE_ROOM", "READ_PARTICIPANT", "READ_ROOM",
			"REMOVE", "DELETE_ROOM", "READ_PARTICIPANT", "READ_ROOM"}
		results := []string{"UNKNOWN", "UNKNOWN", "PRESENT", "PRESENT",
			"ACK", "ACK", "PRESENT", "ABSENT"}
		for i, action := range wantActions {
			ordinal := i + 1
			lease, step := brwCleanupStep(t, h, ordinal)
			if step["action"] != action {
				t.Fatalf("cleanup ordinal %d action=%v want=%s", ordinal, step["action"], action)
			}
			sid := ""
			if results[i] == "PRESENT" {
				if action == "READ_PARTICIPANT" {
					sid = "PA_brw"
				} else {
					sid = "RM_brw"
				}
			}
			brwRecordCleanup(t, h, lease, ordinal, results[i], sid)
		}
		var count, removes, deletes, reads int
		if err := h.lp.f.owner.QueryRow(context.Background(), `SELECT count(*),
			count(*) FILTER (WHERE action='REMOVE'),count(*) FILTER (WHERE action='DELETE_ROOM'),
			count(*) FILTER (WHERE action IN ('READ_PARTICIPANT','READ_ROOM'))
			FROM live.media_input_wire_steps WHERE attempt_id=$1`, h.plan.AttemptID).
			Scan(&count, &removes, &deletes, &reads); err != nil || count != 8 || removes != 2 || deletes != 2 || reads != 4 {
			t.Fatalf("cleanup budget drift: total=%d remove=%d delete=%d reads=%d err=%v",
				count, removes, deletes, reads, err)
		}
		bicAssertOriginalLiability(t, h.bicHarness, h.plan)
		lease, err := brwClaim(context.Background(), h.bicHarness, h.plan, bytes.Repeat([]byte{0x49}, 32))
		if err != nil || lease.disposition != "held" || lease.mode != "" {
			t.Fatalf("exhausted cleanup kept provider capability: %+v err=%v", lease, err)
		}
	})
	t.Run("unknown-participant-does-not-authorize-second-round", func(t *testing.T) {
		h := brwClosedGrant(t)
		for ordinal, result := range []string{"UNKNOWN", "UNKNOWN", "UNKNOWN", "PRESENT"} {
			number := ordinal + 1
			lease, action := brwCleanupStep(t, h, number)
			sid := ""
			if action["action"] == "READ_ROOM" {
				sid = "RM_brw"
			}
			brwRecordCleanup(t, h, lease, number, result, sid)
		}
		var count int
		if err := h.lp.f.owner.QueryRow(context.Background(),
			`SELECT count(*) FROM live.media_input_wire_steps WHERE attempt_id=$1`,
			h.plan.AttemptID).Scan(&count); err != nil || count != 4 {
			t.Fatalf("unsafe second round already reserved: %d %v", count, err)
		}
		lease, err := brwClaim(context.Background(), h.bicHarness, h.plan, bytes.Repeat([]byte{0x4a}, 32))
		if err != nil || lease.disposition != "held" || lease.mode != "" {
			t.Fatalf("unknown participant authorized retry: %+v %v", lease, err)
		}
		bicAssertOriginalLiability(t, h.bicHarness, h.plan)
	})
}

func TestLiveBrowserInputBRW04EgressTerminalDoesNotDiscardInput(t *testing.T) {
	h := brwStart(t)
	brwReserveGrant(t, h)
	lease, err := brwClaim(context.Background(), h.bicHarness, h.plan, bytes.Repeat([]byte{0x61}, 32))
	if err != nil || lease.disposition != "claimed" || lease.mode != "reconcile" {
		t.Fatalf("input observation claim: %+v %v", lease, err)
	}
	brwSelectObserve(t, h, lease)
	if _, err := brwReserveStart(context.Background(), h, lease, h.plan.RoomName,
		h.grant.PublisherIdentity, "PA_brw", "JOINED", true, false, true, false); err != nil {
		t.Fatalf("exact observed camera/mic failed final reservation: %v", err)
	}
	// This SQL fact is a disposable mock-provider observation. It proves the
	// original ledger/native guard still owns input after Egress becomes terminal.
	var disposition string
	if err := h.executor.QueryRow(context.Background(), `SELECT live.record_media_observation(
		$1::uuid,$2::bigint,$3::bytea,'START','EG_brw',$4::text,
		'EGRESS_COMPLETE',100::bigint,140::bigint,130::bigint)`,
		h.plan.OperationID, lease.generation, lease.key, h.plan.RoomName).Scan(&disposition); err != nil {
		t.Fatalf("mock terminal Egress observation: %v", err)
	}
	if disposition == "terminal" {
		t.Fatalf("terminal Egress falsely completed input liability: %q", disposition)
	}
	var operation, resource, child string
	if err := h.lp.f.owner.QueryRow(context.Background(), `SELECT o.state,x.resource_state,c.state
		FROM integration.operations o JOIN live.media_execution_state x ON x.operation_id=o.id
		JOIN live.media_input_custody c ON c.operation_id=o.id WHERE o.id=$1`, h.plan.OperationID).
		Scan(&operation, &resource, &child); err != nil || operation != "UNKNOWN" || resource != "TERMINAL" || child == "CLOSED" {
		t.Fatalf("terminal Egress suppressed input: operation=%s resource=%s child=%s err=%v",
			operation, resource, child, err)
	}
	brwAssertNativePending(t, h, "UNKNOWN")
	if _, err := bicStop(context.Background(), h.bicHarness, h.logins.b,
		t04Key("brw-stop-after-egress-terminal"), h.plan); err != nil {
		t.Fatalf("input Stop disabled by terminal Egress: %v", err)
	}
	next, err := brwClaim(context.Background(), h.bicHarness, h.plan, bytes.Repeat([]byte{0x62}, 32))
	if err != nil || next.disposition != "claimed" || next.mode != "reconcile" {
		t.Fatalf("terminal Egress blocked input cleanup: %+v err=%v", next, err)
	}
	var turn string
	if err := h.executor.QueryRow(context.Background(),
		`SELECT live.next_media_input_turn($1::uuid,$2::bigint,$3::bytea)`,
		h.plan.OperationID, next.generation, next.key).Scan(&turn); err != nil || turn != "INPUT_CLEANUP" {
		t.Fatalf("terminal Egress starved input cleanup: turn=%q err=%v", turn, err)
	}
}

type brwRejectTransport struct{ calls atomic.Int32 }

func (r *brwRejectTransport) RoundTrip(*http.Request) (*http.Response, error) {
	r.calls.Add(1)
	return nil, errors.New("unexpected provider call")
}

type brwWireFixture struct {
	h             *brwHarness
	keys          *livekit.MaterialKeyring
	egress        live.MediaProject
	inputs        atomic.Int32
	starts        atomic.Int32
	removes       atomic.Int32
	ordered       atomic.Bool
	completeQuery atomic.Bool
	cameraMode    atomic.Int32 // 0=ready, 1=missing, 2=muted
	micMode       atomic.Int32
}

func brwWireStarted(t *testing.T) *brwWireFixture {
	t.Helper()
	f := &brwWireFixture{h: &brwHarness{bicHarness: bicSetup(t)}}
	h := f.h
	var err error
	f.keys, err = lmrKeys()
	if err != nil {
		t.Fatal(err)
	}
	sfu := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/twirp/livekit.RoomService/RemoveParticipant":
			f.removes.Add(1)
			lmeReply(w, `{}`)
			return
		case "/twirp/livekit.RoomService/DeleteRoom":
			lmeReply(w, `{}`)
			return
		case "/twirp/livekit.RoomService/ListRooms":
			lmeReply(w, fmt.Sprintf(`{"rooms":[{"name":%q,"sid":"RM_brw"}]}`, h.plan.RoomName))
			return
		case "/twirp/livekit.RoomService/GetParticipant":
			f.inputs.Add(1)
		default:
			http.Error(w, "unexpected SFU call", http.StatusBadRequest)
			return
		}
		var target struct{ Room, Identity string }
		if err := json.NewDecoder(io.LimitReader(r.Body, 2048)).Decode(&target); err != nil ||
			target.Room != h.plan.RoomName || target.Identity != h.grant.PublisherIdentity {
			http.Error(w, "wrong input target", http.StatusBadRequest)
			return
		}
		tracks := make([]map[string]any, 0, 2)
		if mode := f.cameraMode.Load(); mode != 1 {
			tracks = append(tracks, map[string]any{"sid": "TR_cam", "source": "CAMERA", "type": "VIDEO", "muted": mode == 2})
		}
		if mode := f.micMode.Load(); mode != 1 {
			tracks = append(tracks, map[string]any{"sid": "TR_mic", "source": "MICROPHONE", "type": "AUDIO", "muted": mode == 2})
		}
		body, err := json.Marshal(map[string]any{"identity": target.Identity, "sid": "PA_brw", "state": "ACTIVE", "tracks": tracks})
		if err != nil {
			t.Error(err)
			http.Error(w, "fixture", 500)
			return
		}
		lmeReply(w, string(body))
	}))
	t.Cleanup(sfu.Close)
	egressServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/twirp/livekit.Egress/StartEgress":
			if f.inputs.Load() > 0 {
				f.ordered.Store(true)
			}
			f.starts.Add(1)
		case "/twirp/livekit.Egress/ListEgress":
		default:
			http.Error(w, "unexpected Egress call", http.StatusBadRequest)
			return
		}
		status, updated, ended := "EGRESS_ACTIVE", "110", "0"
		if r.URL.Path == "/twirp/livekit.Egress/ListEgress" && f.completeQuery.Load() {
			status, updated, ended = "EGRESS_COMPLETE", "140", "130"
		}
		observation := fmt.Sprintf(`{"egress_id":"EG_brw","room_name":%q,"status":%q,"started_at":"100","updated_at":%q,"ended_at":%q}`,
			h.plan.RoomName, status, updated, ended)
		if r.URL.Path == "/twirp/livekit.Egress/ListEgress" {
			lmeReply(w, `{"items":[`+observation+`]}`)
		} else {
			lmeReply(w, observation)
		}
	}))
	t.Cleanup(egressServer.Close)
	egressConfig := lmeConfig()
	egressTransport := lmeTransport(egressServer.Listener.Addr().String())
	egressClient, err := livekit.New(egressConfig, egressTransport)
	if err != nil {
		t.Fatal(err)
	}
	// The LMP/BIC fixture registers a separate unused authorization with
	// placeholder material. Register this fresh authorization with authentic
	// sealed material before profile selection and planning.
	spec := h.spec()
	room := "lc_" + strings.ReplaceAll(spec["attempt_id"].(string), "-", "")
	sealed, err := f.keys.Seal(livekit.MaterialScope{
		TenantID: h.lp.f.tenantA, StoreID: h.lp.f.storeA1, SessionID: h.session,
		AttemptID: spec["attempt_id"].(string), ProjectID: "project_lma",
		CredentialVersion: 1, MaterialVersion: 1,
	}, egressClient, livekit.StartInput{
		RoomName: room, AspectRatio: "16:9",
		StreamURLs: []string{"rtmps://ingest.example.com/live/brw-" + t04Tag()},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lmaRegister(context.Background(), h.registrar, spec, sealed.Nonce, sealed.Ciphertext); err != nil {
		t.Fatal(err)
	}
	h.input = live.MediaStartInput{SessionID: h.session, AuthorizationID: spec["id"].(string), ExpectedSessionVersion: 1}
	bicRegister(t, h.bicHarness)
	if _, err := h.registrar.Exec(context.Background(),
		`SELECT live.register_media_input_runtime_profile($1::uuid)`, h.input.AuthorizationID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := h.lp.f.owner.Exec(context.Background(),
			`DELETE FROM live.prepared_media_input_runtime_profiles WHERE authorization_id=$1`, h.input.AuthorizationID); err != nil {
			t.Errorf("runtime wire profile cleanup: %v", err)
		}
	})
	sfuConfig := lmeConfig()
	sfuConfig.APIKey = "brw_sfu_test_key"
	sfuConfig.APISecret = strings.Repeat("i", 40)
	h.runtime, err = live.NewBrowserInputRuntime([]live.BrowserInputProject{{
		ProjectID: "project_lma", CredentialVersion: 1, Config: sfuConfig,
		Transport: lmeTransport(sfu.Listener.Addr().String()), BrowserURL: "wss://127.0.0.1:7880",
	}})
	if err != nil {
		t.Fatal(err)
	}
	h.planKey = t04Key("brw-real-worker")
	h.plan, err = brwPlan(context.Background(), h, h.lp.f.runtime, h.logins.a,
		h.lp.f.storeA1, h.planKey, h.input)
	if err != nil {
		t.Fatal(err)
	}
	f.egress = live.MediaProject{ProjectID: "project_lma", CredentialVersion: 1,
		Config: egressConfig, Transport: egressTransport}
	return f
}

func brwStartWorker(t *testing.T, h *bicHarness, keys *livekit.MaterialKeyring,
	egress live.MediaProject, runtime *live.BrowserInputRuntime) {
	t.Helper()
	client, err := live.NewBrowserInputMediaClient(context.Background(), h.worker, h.executor,
		keys, []live.MediaProject{egress}, runtime, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		defer cancel()
		if err := client.StopAndCancel(ctx); err != nil {
			t.Errorf("browser worker stop: %v", err)
		}
	})
}

func TestLiveBrowserInputBRW01MarkerZeroWorkerNeverTouchesProvider(t *testing.T) {
	h, plan, _ := bicStarted(t)
	keys, err := lmrKeys()
	if err != nil {
		t.Fatal(err)
	}
	inputTransport, egressTransport := &brwRejectTransport{}, &brwRejectTransport{}
	runtime, err := live.NewBrowserInputRuntime([]live.BrowserInputProject{{
		ProjectID: "project_lma", CredentialVersion: 1, Config: lmeConfig(),
		Transport: inputTransport, BrowserURL: "ws://127.0.0.1:7880",
	}})
	if err != nil {
		t.Fatal(err)
	}
	brwStartWorker(t, h, keys, live.MediaProject{
		ProjectID: "project_lma", CredentialVersion: 1, Config: lmeConfig(), Transport: egressTransport,
	}, runtime)
	deadline := time.Now().Add(12 * time.Second)
	var state string
	for time.Now().Before(deadline) {
		if err := h.lp.f.owner.QueryRow(context.Background(),
			`SELECT state FROM river_media.river_job WHERE id=$1`, plan.JobID).Scan(&state); err != nil {
			t.Fatal(err)
		}
		if state == "scheduled" {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if state != "scheduled" || inputTransport.calls.Load() != 0 || egressTransport.calls.Load() != 0 {
		t.Fatalf("marker-0 worker dispatched provider: state=%s input=%d egress=%d",
			state, inputTransport.calls.Load(), egressTransport.calls.Load())
	}
	var generation int64
	var leaseUntil *time.Time
	if err := h.lp.f.owner.QueryRow(context.Background(),
		`SELECT generation,lease_until FROM integration.operations WHERE id=$1`, plan.OperationID).
		Scan(&generation, &leaseUntil); err != nil || generation != 0 || leaseUntil != nil {
		t.Fatalf("marker-0 worker changed lease: generation=%d until=%v err=%v", generation, leaseUntil, err)
	}
}

func TestLiveBrowserInputBRW02OriginalJobWorkerBeforeSingleStart(t *testing.T) {
	t.Run("unissued-has-no-provider-call", func(t *testing.T) {
		f := brwWireStarted(t)
		brwStartWorker(t, f.h.bicHarness, f.keys, f.egress, f.h.runtime)
		deadline := time.Now().Add(12 * time.Second)
		var state string
		for time.Now().Before(deadline) {
			if err := f.h.lp.f.owner.QueryRow(context.Background(),
				`SELECT state FROM river_media.river_job WHERE id=$1`, f.h.plan.JobID).Scan(&state); err != nil {
				t.Fatal(err)
			}
			if state == "scheduled" {
				break
			}
			time.Sleep(25 * time.Millisecond)
		}
		if state != "scheduled" {
			t.Fatalf("original unissued job not snoozed: %s", state)
		}
		if f.inputs.Load() != 0 || f.starts.Load() != 0 {
			t.Fatalf("unissued runtime made provider calls: input=%d start=%d", f.inputs.Load(), f.starts.Load())
		}
		generation, leaseUntil, reserved := brwWireFacts(t, f.h)
		if generation != 0 || leaseUntil != nil || reserved != nil {
			t.Fatalf("unissued worker created wire lease: %d %v %v", generation, leaseUntil, reserved)
		}
	})
	t.Run("dualtrack-observed-before-one-start", func(t *testing.T) {
		f := brwWireStarted(t)
		brwReserveGrant(t, f.h)
		brwStartWorker(t, f.h.bicHarness, f.keys, f.egress, f.h.runtime)
		deadline := time.Now().Add(15 * time.Second)
		for time.Now().Before(deadline) && f.starts.Load() == 0 {
			time.Sleep(25 * time.Millisecond)
		}
		if f.starts.Load() != 1 || f.inputs.Load() < 1 || !f.ordered.Load() {
			t.Fatalf("original worker failed observe-before-single-Start: input=%d starts=%d ordered=%t",
				f.inputs.Load(), f.starts.Load(), f.ordered.Load())
		}
		_, _, reserved := brwWireFacts(t, f.h)
		if reserved == nil {
			t.Fatal("provider Start lacked committed wire reservation")
		}
	})
	for _, item := range []struct {
		name        string
		camera, mic int32
	}{
		{"camera-missing", 1, 0}, {"microphone-muted", 0, 2},
	} {
		t.Run(item.name+"-has-no-Start", func(t *testing.T) {
			f := brwWireStarted(t)
			f.cameraMode.Store(item.camera)
			f.micMode.Store(item.mic)
			brwReserveGrant(t, f.h)
			brwStartWorker(t, f.h.bicHarness, f.keys, f.egress, f.h.runtime)
			deadline := time.Now().Add(12 * time.Second)
			var jobState string
			for time.Now().Before(deadline) {
				if err := f.h.lp.f.owner.QueryRow(context.Background(),
					`SELECT state FROM river_media.river_job WHERE id=$1`, f.h.plan.JobID).Scan(&jobState); err != nil {
					t.Fatal(err)
				}
				if f.inputs.Load() > 0 && jobState == "scheduled" {
					break
				}
				time.Sleep(25 * time.Millisecond)
			}
			_, _, reserved := brwWireFacts(t, f.h)
			if f.inputs.Load() == 0 || jobState != "scheduled" || f.starts.Load() != 0 || reserved != nil {
				t.Fatalf("invalid actual tracks reached Start: input=%d job=%s start=%d reserved=%v",
					f.inputs.Load(), jobState, f.starts.Load(), reserved)
			}
		})
	}
}

func TestLiveBrowserInputBRW04ActualWorkerKeepsJobAfterEgressTerminal(t *testing.T) {
	f := brwWireStarted(t)
	f.completeQuery.Store(true)
	brwReserveGrant(t, f.h)
	brwStartWorker(t, f.h.bicHarness, f.keys, f.egress, f.h.runtime)
	deadline := time.Now().Add(25 * time.Second)
	var resource, operation, jobState, child string
	var finalized *time.Time
	for time.Now().Before(deadline) {
		if err := f.h.lp.f.owner.QueryRow(context.Background(), `SELECT x.resource_state,o.state,j.state,j.finalized_at,c.state
			FROM live.media_execution_state x JOIN integration.operations o ON o.id=x.operation_id
			JOIN river_media.river_job j ON j.id=o.job_id
			JOIN live.media_input_custody c ON c.attempt_id=x.attempt_id WHERE o.id=$1`, f.h.plan.OperationID).
			Scan(&resource, &operation, &jobState, &finalized, &child); err != nil {
			t.Fatal(err)
		}
		if resource == "TERMINAL" {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if f.starts.Load() != 1 || resource != "TERMINAL" || operation != "UNKNOWN" ||
		jobState == "completed" || jobState == "cancelled" || jobState == "discarded" ||
		finalized != nil || child == "CLOSED" {
		t.Fatalf("terminal Egress completed original input job: start=%d resource=%s op=%s job=%s finalized=%v child=%s",
			f.starts.Load(), resource, operation, jobState, finalized, child)
	}
	if _, err := bicStop(context.Background(), f.h.bicHarness, f.h.logins.b,
		t04Key("brw-actual-terminal-stop"), f.h.plan); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) && f.removes.Load() == 0 {
		time.Sleep(25 * time.Millisecond)
	}
	if f.removes.Load() == 0 {
		t.Fatal("Egress terminal starved original worker input cleanup")
	}
	if err := f.h.lp.f.owner.QueryRow(context.Background(),
		`SELECT state,finalized_at FROM river_media.river_job WHERE id=$1`, f.h.plan.JobID).
		Scan(&jobState, &finalized); err != nil || finalized != nil ||
		(jobState == "completed" || jobState == "cancelled" || jobState == "discarded") {
		t.Fatalf("post-Egress input cleanup lost native original job: %s %v %v", jobState, finalized, err)
	}
}
