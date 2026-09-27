package foundation_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

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
		bicAssertOriginalLiability(t, h.bicHarness, h.plan)
		if _, err := h.lp.f.owner.Exec(context.Background(),
			`UPDATE river_media.river_job SET state='completed',finalized_at=clock_timestamp() WHERE id=$1`,
			h.plan.JobID); sqlState(err) != "22023" {
			t.Fatalf("native guard finalized unresolved input/Egress job: %v", err)
		}
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
	if _, err := h.lp.f.owner.Exec(context.Background(),
		`UPDATE river_media.river_job SET state='completed',finalized_at=clock_timestamp() WHERE id=$1`,
		h.plan.JobID); sqlState(err) != "22023" {
		t.Fatalf("native job finalized with unresolved input after Egress terminal: %v", err)
	}
	if _, err := bicStop(context.Background(), h.bicHarness, h.logins.b,
		t04Key("brw-stop-after-egress-terminal"), h.plan); err != nil {
		t.Fatalf("input Stop disabled by terminal Egress: %v", err)
	}
	bicAssertOriginalLiability(t, h.bicHarness, h.plan)
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
