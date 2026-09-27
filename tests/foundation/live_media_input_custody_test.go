package foundation_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/command"
	"livecommerce/internal/live"
	"livecommerce/internal/platform"
)

// BIC is the storage/planner kernel only. These tests never issue a publisher
// token, start a provider client, or claim browser/Cloud acceptance.
type bicHarness struct {
	*lmpHarness
	logins   mlcLogins
	executor *pgxpool.Pool
	worker   *pgxpool.Pool
}

func bicSetup(t *testing.T) *bicHarness {
	t.Helper()
	h := &bicHarness{lmpHarness: lmpSetup(t, false)}
	h.logins = mlcTwoLogins(t, h.lmpHarness)
	_, h.executor = lmaLogin(t, h.lp.f, "commerce_media_executor")
	_, h.worker = lmaLogin(t, h.lp.f, "commerce_media_worker")
	// Native executor/worker logins inherit only their exact authority.
	for _, item := range []struct {
		role string
		pool *pgxpool.Pool
	}{
		{"commerce_media_executor", h.executor}, {"commerce_media_worker", h.worker},
	} {
		var login string
		if err := item.pool.QueryRow(context.Background(), `SELECT current_user`).Scan(&login); err != nil {
			t.Fatal(err)
		}
		mustExec(t, h.lp.f.owner, "REVOKE "+item.role+" FROM "+pgx.Identifier{login}.Sanitize())
		mustExec(t, h.lp.f.owner, "GRANT "+item.role+" TO "+pgx.Identifier{login}.Sanitize()+" WITH INHERIT TRUE, SET FALSE")
	}
	// This owner-only teardown precedes lmpSetup's attempt/operation deletion.
	t.Cleanup(func() {
		ctx := context.Background()
		for _, item := range []struct{ query, arg string }{
			{`DELETE FROM live.media_observations WHERE attempt_id IN (SELECT id FROM live.media_attempts WHERE session_id=$1)`, h.session},
			{`DELETE FROM live.media_execution_state WHERE attempt_id IN (SELECT id FROM live.media_attempts WHERE session_id=$1)`, h.session},
			{`DELETE FROM live.media_input_custody WHERE attempt_id IN (SELECT id FROM live.media_attempts WHERE session_id=$1)`, h.session},
			{`DELETE FROM live.prepared_media_input_profiles WHERE authorization_id=$1`, h.input.AuthorizationID},
		} {
			if _, err := h.lp.f.owner.Exec(ctx, item.query, item.arg); err != nil {
				t.Errorf("BIC owner fixture cleanup: %v", err)
			}
		}
		for _, q := range []string{
			`DELETE FROM ops.command_results WHERE principal_id=$1 AND operation IN ('live.media.input.start','live.media.input.reserve','live.media.stop')`,
			`DELETE FROM ops.audit_events WHERE principal_id=$1 AND (action LIKE 'live.media.input.%' OR action='live.media.stop.requested')`,
		} {
			if _, err := h.lp.f.owner.Exec(ctx, q, h.lp.actor); err != nil {
				t.Errorf("BIC command fixture cleanup: %v", err)
			}
		}
	})
	return h
}

func bicRegister(t *testing.T, h *bicHarness) {
	t.Helper()
	if _, err := h.registrar.Exec(context.Background(), `SELECT live.register_media_input_profile($1::uuid)`, h.input.AuthorizationID); err != nil {
		t.Fatalf("actual registrar profile registration: %v", err)
	}
}

func bicPlan(ctx context.Context, h *bicHarness, pool *pgxpool.Pool, planner *live.MediaPlanner, token, store, key string, in live.MediaStartInput) (live.MediaStartResult, error) {
	var out live.MediaStartResult
	err := platform.WithScope(ctx, pool, token, store, "store:read", func(tx pgx.Tx, scope platform.Scope) error {
		var err error
		out, err = planner.PlanInputStart(ctx, tx, scope, token, key, in)
		return err
	})
	return out, err
}

func bicReserve(ctx context.Context, h *bicHarness, pool *pgxpool.Pool, token, store, key string, in live.MediaInputReserveInput) (live.MediaInputGrant, error) {
	var out live.MediaInputGrant
	err := platform.WithScope(ctx, pool, token, store, "store:read", func(tx pgx.Tx, scope platform.Scope) error {
		var err error
		out, err = h.planner.ReserveInput(ctx, tx, scope, token, key, in)
		return err
	})
	return out, err
}

func bicClaim(ctx context.Context, h *bicHarness, plan live.MediaStartResult, key []byte) (string, int64, string, error) {
	var disposition, mode string
	var generation int64
	err := h.executor.QueryRow(ctx, `SELECT disposition,generation,mode FROM live.claim_media_input_operation($1::uuid,$2::bigint,$3::integer,$4::bytea)`,
		plan.OperationID, plan.JobID, 30, key).Scan(&disposition, &generation, &mode)
	return disposition, generation, mode, err
}

func bicOwnedFacts(t *testing.T, h *bicHarness) [8]int64 {
	t.Helper()
	var out [8]int64
	err := h.lp.f.owner.QueryRow(context.Background(), `SELECT
	 (SELECT count(*) FROM live.prepared_media_input_profiles WHERE authorization_id=$2),
	 (SELECT count(*) FROM live.media_attempts WHERE session_id=$1),
	 (SELECT count(*) FROM live.media_login_custody WHERE attempt_id IN (SELECT id FROM live.media_attempts WHERE session_id=$1)),
	 (SELECT count(*) FROM live.media_input_custody WHERE attempt_id IN (SELECT id FROM live.media_attempts WHERE session_id=$1)),
	 (SELECT count(*) FROM integration.operations WHERE media_attempt_id IN (SELECT id FROM live.media_attempts WHERE session_id=$1)),
	 (SELECT count(*) FROM river_media.river_job j JOIN integration.operations o ON o.job_id=j.id
	  WHERE j.queue='media_input_mock_v1' AND o.media_attempt_id IN (SELECT id FROM live.media_attempts WHERE session_id=$1)),
	 (SELECT count(*) FROM ops.command_results WHERE principal_id=$3 AND operation='live.media.input.start'),
	 (SELECT count(*) FROM ops.command_results WHERE principal_id=$3 AND operation='live.media.input.reserve')`,
		h.session, h.input.AuthorizationID, h.lp.actor).Scan(&out[0], &out[1], &out[2], &out[3], &out[4], &out[5], &out[6], &out[7])
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func bicPlanReady(t *testing.T, h *bicHarness) {
	t.Helper()
	for _, pool := range []*pgxpool.Pool{h.lp.f.runtime, h.executor, h.worker} {
		var ready bool
		if err := pool.QueryRow(context.Background(), `SELECT live.media_input_plan_ready()`).Scan(&ready); err != nil || !ready {
			t.Fatalf("input readiness from actual role: ready=%v err=%v", ready, err)
		}
	}
}

func TestLiveMediaExecutionBIC01ProfilePlanAndIsolation(t *testing.T) {
	t.Run("atomic-new-queue", func(t *testing.T) {
		h := bicSetup(t)
		bicPlanReady(t, h)
		if _, err := bicPlan(context.Background(), h, h.lp.f.runtime, h.planner, h.logins.a, h.lp.f.storeA1, t04Key("bic-unprofiled"), h.input); err == nil {
			t.Fatal("input planner accepted authorization without private profile")
		}
		if got := bicOwnedFacts(t, h); got != [8]int64{} {
			t.Fatalf("unprofiled denial left artifacts: %v", got)
		}
		bicRegister(t, h)
		bicRegister(t, h) // idempotent registrar only, not another child
		if _, err := lmpStart(context.Background(), h.lmpHarness, h.planner, h.logins.a, h.lp.f.storeA1, t04Key("bic-legacy"), h.input); err == nil {
			t.Fatal("legacy planner consumed input-profiled authorization")
		}
		before := bicOwnedFacts(t, h)
		if before != [8]int64{1} {
			t.Fatalf("registration or legacy denial created artifacts: %v", before)
		}
		for _, item := range []struct {
			name, token, store string
			input              live.MediaStartInput
		}{
			{"other-store", h.logins.a, h.lp.f.storeA2, h.input},
			{"other-tenant", h.lp.otherToken, h.lp.f.storeB, h.input},
			{"wrong-version", h.logins.a, h.lp.f.storeA1, live.MediaStartInput{SessionID: h.session, AuthorizationID: h.input.AuthorizationID, ExpectedSessionVersion: 2}},
		} {
			if _, err := bicPlan(context.Background(), h, h.lp.f.runtime, h.planner, item.token, item.store, t04Key("bic-"+item.name), item.input); err == nil {
				t.Fatalf("input planner accepted %s", item.name)
			}
			if got := bicOwnedFacts(t, h); got != before {
				t.Fatalf("%s denial left input artifacts: %v", item.name, got)
			}
		}
		rollback := errors.New("rollback after input child and native InsertTx")
		err := platform.WithScope(context.Background(), h.lp.f.runtime, h.logins.a, h.lp.f.storeA1, "store:read", func(tx pgx.Tx, scope platform.Scope) error {
			if _, err := h.planner.PlanInputStart(context.Background(), tx, scope, h.logins.a, t04Key("bic-rollback"), h.input); err != nil {
				return err
			}
			return rollback
		})
		if !errors.Is(err, rollback) || bicOwnedFacts(t, h) != before {
			t.Fatalf("rolled-back input plan retained attempt, job, child or receipt: %v facts=%v", err, bicOwnedFacts(t, h))
		}
		out, err := bicPlan(context.Background(), h, h.lp.f.runtime, h.planner, h.logins.a, h.lp.f.storeA1, t04Key("bic-plan"), h.input)
		if err != nil {
			t.Fatal(err)
		}
		if out.AttemptID != h.specification["attempt_id"] || out.JobID < 1 || out.State != "READY" || !command.ValidID(out.OperationID) {
			t.Fatalf("bad frozen plan: %+v", out)
		}
		if got := bicOwnedFacts(t, h); got != [8]int64{1, 1, 1, 1, 1, 1, 1, 0} {
			t.Fatalf("not one atomic input child/old operation/new native job: %v", got)
		}
		if _, err := bicPlan(context.Background(), h, h.lp.f.runtime, h.planner, h.logins.b, h.lp.f.storeA1, t04Key("bic-plan"), h.input); err == nil {
			t.Fatal("same principal second login recovered input Start replay")
		}
		var profile, queue, operation, jobKind string
		var jobID int64
		err = h.lp.f.owner.QueryRow(context.Background(), `SELECT a.execution_profile,j.queue,o.id::text,j.kind,j.id
		 FROM live.media_attempts a JOIN integration.operations o ON o.media_attempt_id=a.id
		 JOIN river_media.river_job j ON j.id=o.job_id WHERE a.id=$1`, out.AttemptID).
			Scan(&profile, &queue, &operation, &jobKind, &jobID)
		if err != nil || profile != "LOCAL_SFU_MOCK_EGRESS" || queue != "media_input_mock_v1" || operation != out.OperationID || jobKind != "live_media_operation_v1" || jobID != out.JobID {
			t.Fatalf("input profile/queue/original job mismatch: %s %s %s %s %d %v", profile, queue, operation, jobKind, jobID, err)
		}
		if _, err := h.lp.f.owner.Exec(context.Background(), `UPDATE river_media.river_job SET queue='media_mock_v1' WHERE id=$1`, out.JobID); sqlState(err) != "22023" {
			t.Fatalf("native guard did not reject input→legacy queue mutation: %v", err)
		}
		var oldDisposition, oldMode string
		var oldGeneration int64
		if err := h.executor.QueryRow(context.Background(), `SELECT disposition,generation,mode FROM live.claim_media_operation($1::uuid,$2::bigint,$3::integer,$4::bytea)`,
			out.OperationID, out.JobID, 30, bytes.Repeat([]byte{0x60}, 32)).Scan(&oldDisposition, &oldGeneration, &oldMode); err == nil {
			t.Fatalf("legacy claim accepted new input profile: %s/%d/%s", oldDisposition, oldGeneration, oldMode)
		}
	})

	t.Run("registration-serializes-legacy-planning", func(t *testing.T) {
		h := bicSetup(t)
		ctx := context.Background()
		holder, err := h.registrar.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer holder.Rollback(ctx)
		var holderPID int
		if err := holder.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&holderPID); err != nil {
			t.Fatal(err)
		}
		if _, err := holder.Exec(ctx, `SELECT live.register_media_input_profile($1::uuid)`, h.input.AuthorizationID); err != nil {
			t.Fatal(err)
		}
		waiterPID, done := mlcWaiter(h.lmpHarness, h.logins.a, t04Key("bic-legacy-race"))
		lmaObserveBlock(t, h.lp.f.owner, <-waiterPID, holderPID, false)
		if err := holder.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		if result := lmpAwaitWaiter(t, done); result.err == nil {
			t.Fatalf("legacy planner consumed freshly registered profile: %+v", result.out)
		}
		if got := bicOwnedFacts(t, h); got != [8]int64{1} {
			t.Fatalf("registration/legacy planning race created artifacts: %v", got)
		}
		if _, err := bicPlan(ctx, h, h.lp.f.runtime, h.planner, h.logins.a, h.lp.f.storeA1, t04Key("bic-race-input"), h.input); err != nil {
			t.Fatalf("registered authorization unusable by input planner: %v", err)
		}
	})
}

func bicGrantMap(t *testing.T, grant live.MediaInputGrant) map[string]any {
	t.Helper()
	raw, err := json.Marshal(grant)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	want := []string{"attempt_id", "room_name", "publisher_identity", "project_id", "endpoint_identity", "credential_version", "issued_at", "expires_at"}
	if len(fields) != len(want) {
		t.Fatalf("grant had %d fields, want exactly %d", len(fields), len(want))
	}
	for _, key := range want {
		if _, ok := fields[key]; !ok {
			t.Fatalf("grant missing %s", key)
		}
	}
	for _, forbidden := range []string{"token", "jwt", "secret", "login_session_id", "authz_revision"} {
		if strings.Contains(strings.ToLower(string(raw)), forbidden) {
			t.Fatalf("nonsecret grant exposed %s", forbidden)
		}
	}
	return fields
}

func bicStarted(t *testing.T) (*bicHarness, live.MediaStartResult, live.MediaInputReserveInput) {
	t.Helper()
	h := bicSetup(t)
	bicRegister(t, h)
	plan, err := bicPlan(context.Background(), h, h.lp.f.runtime, h.planner, h.logins.a, h.lp.f.storeA1, t04Key("bic-start"), h.input)
	if err != nil {
		t.Fatal(err)
	}
	return h, plan, live.MediaInputReserveInput{SessionID: h.session, AttemptID: plan.AttemptID, ExpectedSessionVersion: 1}
}

func TestLiveMediaExecutionBIC02GrantReplayAndAuthority(t *testing.T) {
	t.Run("same-grant-new-key-and-exact-login", func(t *testing.T) {
		h, plan, in := bicStarted(t)
		key := t04Key("bic-reserve")
		grant, err := bicReserve(context.Background(), h, h.lp.f.runtime, h.logins.a, h.lp.f.storeA1, key, in)
		if err != nil {
			t.Fatal(err)
		}
		fields := bicGrantMap(t, grant)
		if fields["attempt_id"] != plan.AttemptID || fields["room_name"] != plan.RoomName || fields["project_id"] != "project_lma" ||
			fields["endpoint_identity"] != "https://unit.livekit.cloud" || fields["credential_version"] != float64(1) {
			t.Fatal("grant lost frozen attempt/project binding")
		}
		issued, iOK := fields["issued_at"].(float64)
		expires, eOK := fields["expires_at"].(float64)
		identity, idOK := fields["publisher_identity"].(string)
		if !iOK || !eOK || !idOK || !strings.HasPrefix(identity, "lcp_") || len(identity) != 36 ||
			expires <= issued || expires-issued > 60 {
			t.Fatal("grant lifetime/identity not bounded")
		}
		for _, replayKey := range []string{key, t04Key("bic-reserve-other-key")} {
			replayed, err := bicReserve(context.Background(), h, h.lp.f.runtime, h.logins.a, h.lp.f.storeA1, replayKey, in)
			if err != nil || !reflect.DeepEqual(bicGrantMap(t, replayed), fields) {
				t.Fatalf("same grant changed under %s: %v", replayKey, err)
			}
		}
		if _, err := bicReserve(context.Background(), h, h.lp.f.runtime, h.logins.b, h.lp.f.storeA1, key, in); err == nil {
			t.Fatal("same principal second login recovered input grant")
		}
		changed := in
		changed.ExpectedSessionVersion++
		if _, err := bicReserve(context.Background(), h, h.lp.f.runtime, h.logins.a, h.lp.f.storeA1, key, changed); !errors.Is(err, command.ErrConflict) {
			t.Fatalf("same key changed expected version: %v", err)
		}
		if got := bicOwnedFacts(t, h); got != [8]int64{1, 1, 1, 1, 1, 1, 1, 2} {
			t.Fatalf("replay created artifacts: %v", got)
		}
		rows, err := h.lp.f.owner.Query(context.Background(), `SELECT response FROM ops.command_results
		 WHERE principal_id=$1 AND operation='live.media.input.reserve'`, h.lp.actor)
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		for rows.Next() {
			var response []byte
			if err := rows.Scan(&response); err != nil {
				t.Fatal(err)
			}
			var rawFields map[string]json.RawMessage
			if err := json.Unmarshal(response, &rawFields); err != nil || len(rawFields) != len(fields) {
				t.Fatalf("durable grant receipt has unexpected shape: fields=%d err=%v", len(rawFields), err)
			}
			for key := range fields {
				if _, ok := rawFields[key]; !ok {
					t.Fatalf("durable grant receipt missing %s", key)
				}
			}
			var got live.MediaInputGrant
			if err := json.Unmarshal(response, &got); err != nil || !reflect.DeepEqual(bicGrantMap(t, got), fields) {
				t.Fatalf("durable receipt differs from nonsecret fixed grant: %v", err)
			}
			count++
		}
		rowsErr := rows.Err()
		rows.Close()
		if rowsErr != nil || count != 2 {
			t.Fatalf("grant command receipts missing or extra: %d %v", count, rowsErr)
		}
		var secretColumns int
		if err := h.lp.f.owner.QueryRow(context.Background(), `SELECT count(*) FROM information_schema.columns
		 WHERE table_schema='live' AND table_name='media_input_custody'
		 AND column_name ~* '(token|jwt|secret|bearer|hash)'`).Scan(&secretColumns); err != nil || secretColumns != 0 {
			t.Fatalf("input custody schema has raw-token shaped column: %d %v", secretColumns, err)
		}
	})

	t.Run("unissued-claim-does-not-forfeit-prewire-reservation", func(t *testing.T) {
		h, plan, in := bicStarted(t)
		lease := bytes.Repeat([]byte{0x68}, 32)
		disposition, generation, mode, err := bicClaim(context.Background(), h, plan, lease)
		if err != nil || disposition != "claimed" || mode != "reconcile" || generation < 1 {
			t.Fatalf("BIC-only unissued claim not reconcilable: %s/%d/%s %v", disposition, generation, mode, err)
		}
		grant, err := bicReserve(context.Background(), h, h.lp.f.runtime, h.logins.a, h.lp.f.storeA1, t04Key("bic-after-claim"), in)
		if err != nil || bicGrantMap(t, grant)["attempt_id"] != plan.AttemptID {
			t.Fatalf("claim without wire/Stop consumed input admission: %v", err)
		}
		if got := bicOwnedFacts(t, h); got != [8]int64{1, 1, 1, 1, 1, 1, 1, 1} {
			t.Fatalf("prewire claim/reserve duplicated durable artifacts: %v", got)
		}
	})

	t.Run("lost-commit-ack-does-not-create-second-grant", func(t *testing.T) {
		h, _, in := bicStarted(t)
		config := h.lp.f.runtime.Config()
		config.MaxConns = 1
		loss := &hpCommitLoss{}
		dial := config.ConnConfig.DialFunc
		config.ConnConfig.DialFunc = func(ctx context.Context, network, address string) (net.Conn, error) {
			conn, err := dial(ctx, network, address)
			if err != nil {
				return nil, err
			}
			return &hpCommitLossConn{Conn: conn, loss: loss}, nil
		}
		pool, err := pgxpool.NewWithConfig(context.Background(), config)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(pool.Close)
		key := t04Key("bic-reserve-ack")
		loss.armed.Store(true)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, err = bicReserve(ctx, h, pool, h.logins.a, h.lp.f.storeA1, key, in)
		if err == nil || !loss.committed.Load() {
			t.Fatalf("COMMIT ack loss not observed: err=%v committed=%t", err, loss.committed.Load())
		}
		var issued, expires int64
		var identity string
		if err := h.lp.f.owner.QueryRow(context.Background(), `SELECT grant_iat,grant_exp,publisher_identity FROM live.media_input_custody WHERE attempt_id=$1`, in.AttemptID).
			Scan(&issued, &expires, &identity); err != nil || issued <= 0 || expires <= issued || identity == "" {
			t.Fatalf("committed reservation missing after ack loss: %d %d %q %v", issued, expires, identity, err)
		}
		replayed, err := bicReserve(context.Background(), h, h.lp.f.runtime, h.logins.a, h.lp.f.storeA1, key, in)
		if err != nil {
			t.Fatal(err)
		}
		fields := bicGrantMap(t, replayed)
		if fields["publisher_identity"] != identity || fields["issued_at"] != float64(issued) || fields["expires_at"] != float64(expires) {
			t.Fatal("ack-loss replay changed durable grant")
		}
		if got := bicOwnedFacts(t, h); got != [8]int64{1, 1, 1, 1, 1, 1, 1, 1} {
			t.Fatalf("ack-loss duplicated artifacts: %v", got)
		}
	})

	t.Run("exact-login-expiry-after-observed-wait", func(t *testing.T) {
		h, _, in := bicStarted(t)
		ctx := context.Background()
		proceed := make(chan struct{})
		pidCh := make(chan int, 1)
		done := make(chan error, 1)
		go func() {
			done <- platform.WithScope(ctx, h.lp.f.runtime, h.logins.a, h.lp.f.storeA1, "store:read", func(tx pgx.Tx, scope platform.Scope) error {
				var pid int
				if err := tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
					return err
				}
				pidCh <- pid
				<-proceed
				_, err := h.planner.ReserveInput(ctx, tx, scope, h.logins.a, t04Key("bic-wait-expiry"), in)
				return err
			})
		}()
		waiterPID := <-pidCh
		holder, err := h.lp.f.owner.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer holder.Rollback(ctx)
		var holderPID int
		if err := holder.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&holderPID); err != nil {
			t.Fatal(err)
		}
		if _, err := holder.Exec(ctx, `SELECT id FROM identity.sessions WHERE id=$1 FOR UPDATE`, h.logins.aID); err != nil {
			t.Fatal(err)
		}
		close(proceed)
		lmaObserveBlock(t, h.lp.f.owner, waiterPID, holderPID, false)
		if _, err := holder.Exec(ctx, `UPDATE identity.sessions SET expires_at=clock_timestamp()-interval '1 second' WHERE id=$1`, h.logins.aID); err != nil {
			t.Fatal(err)
		}
		if err := holder.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		if err := <-done; err == nil {
			t.Fatal("reservation survived exact login expiry after lock wait")
		}
		var issued, expires *int64
		if err := h.lp.f.owner.QueryRow(ctx, `SELECT grant_iat,grant_exp FROM live.media_input_custody WHERE attempt_id=$1`, in.AttemptID).Scan(&issued, &expires); err != nil || issued != nil || expires != nil {
			t.Fatalf("failed post-wait reservation leaked grant: %v/%v %v", issued, expires, err)
		}
		if got := bicOwnedFacts(t, h); got != [8]int64{1, 1, 1, 1, 1, 1, 1, 0} {
			t.Fatalf("failed post-wait reservation added receipt/job: %v", got)
		}
	})
}

func bicStop(ctx context.Context, h *bicHarness, token, key string, plan live.MediaStartResult) (live.MediaStopResult, error) {
	var out live.MediaStopResult
	err := platform.WithScope(ctx, h.lp.f.runtime, token, h.lp.f.storeA1, "store:read", func(tx pgx.Tx, scope platform.Scope) error {
		var err error
		out, err = h.planner.RequestStop(ctx, tx, scope, token, key,
			live.MediaStopInput{SessionID: h.session, AttemptID: plan.AttemptID})
		return err
	})
	return out, err
}

func bicClose(ctx context.Context, h *bicHarness, plan live.MediaStartResult, generation int64, key []byte, reason string) (string, error) {
	var disposition string
	err := h.executor.QueryRow(ctx, `SELECT live.close_media_input_admission($1::uuid,$2::bigint,$3::bytea,$4::text)`,
		plan.OperationID, generation, key, reason).Scan(&disposition)
	return disposition, err
}

func bicLoad(ctx context.Context, h *bicHarness, plan live.MediaStartResult, generation int64, key []byte) (map[string]any, error) {
	var raw []byte
	err := h.executor.QueryRow(ctx, `SELECT live.load_media_input_custody($1::uuid,$2::bigint,$3::bytea)`,
		plan.OperationID, generation, key).Scan(&raw)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func bicOperationAndJob(t *testing.T, h *bicHarness, plan live.MediaStartResult) (string, string, *time.Time) {
	t.Helper()
	var operation, jobState string
	var finalized *time.Time
	err := h.lp.f.owner.QueryRow(context.Background(), `SELECT o.state,j.state,j.finalized_at
	 FROM integration.operations o JOIN river_media.river_job j ON j.id=o.job_id
	 WHERE o.id=$1 AND o.job_id=$2`, plan.OperationID, plan.JobID).Scan(&operation, &jobState, &finalized)
	if err != nil {
		t.Fatal(err)
	}
	return operation, jobState, finalized
}

func bicAssertOriginalLiability(t *testing.T, h *bicHarness, plan live.MediaStartResult) {
	t.Helper()
	operation, jobState, finalized := bicOperationAndJob(t, h, plan)
	if operation != "UNKNOWN" || finalized != nil ||
		(jobState != "available" && jobState != "scheduled" && jobState != "retryable" && jobState != "running") {
		t.Fatalf("issued input lost original job: operation=%s native=%s finalized=%v", operation, jobState, finalized)
	}
	var latestEvent string
	if err := h.lp.f.owner.QueryRow(context.Background(), `SELECT state FROM integration.operation_events
	 WHERE operation_id=$1 ORDER BY id DESC LIMIT 1`, plan.OperationID).Scan(&latestEvent); err != nil || latestEvent != "UNKNOWN" {
		t.Fatalf("issued input has terminal/missing latest event: %s %v", latestEvent, err)
	}
	var state string
	var closed *time.Time
	if err := h.lp.f.owner.QueryRow(context.Background(), `SELECT state,admission_closed_at FROM live.media_input_custody WHERE attempt_id=$1`,
		plan.AttemptID).Scan(&state, &closed); err != nil || state == "CLOSED" || closed == nil {
		t.Fatalf("issued local custody falsely closed/open admission: state=%s close=%v err=%v", state, closed, err)
	}
}

func TestLiveMediaExecutionBIC03OriginalJobOutlivesInputLiability(t *testing.T) {
	t.Run("unissued-stop-joint-terminal", func(t *testing.T) {
		h, plan, _ := bicStarted(t)
		if _, err := bicStop(context.Background(), h, h.logins.b, t04Key("bic-stop-unissued"), plan); err != nil {
			t.Fatal(err)
		}
		var operation, input string
		err := h.lp.f.owner.QueryRow(context.Background(), `SELECT o.state,c.state FROM integration.operations o
		 JOIN live.media_input_custody c ON c.operation_id=o.id WHERE o.id=$1`, plan.OperationID).Scan(&operation, &input)
		if err != nil || input != "CLOSED" || operation != "CANCELLED" {
			t.Fatalf("unissued Stop not jointly terminal: operation=%s input=%s err=%v", operation, input, err)
		}
	})

	t.Run("reserved-stop-keeps-original-job", func(t *testing.T) {
		h, plan, in := bicStarted(t)
		if _, err := bicReserve(context.Background(), h, h.lp.f.runtime, h.logins.a, h.lp.f.storeA1, t04Key("bic-issued"), in); err != nil {
			t.Fatal(err)
		}
		if _, err := bicStop(context.Background(), h, h.logins.b, t04Key("bic-stop-issued"), plan); err != nil {
			t.Fatal(err)
		}
		key := bytes.Repeat([]byte{0x61}, 32)
		disposition, generation, mode, err := bicClaim(context.Background(), h, plan, key)
		if err != nil || disposition != "claimed" || mode != "reconcile" || generation < 1 {
			t.Fatalf("original job cannot reconcile issued input: %s/%d/%s %v", disposition, generation, mode, err)
		}
		material, err := bicLoad(context.Background(), h, plan, generation, key)
		if err != nil || material["attempt_id"] != plan.AttemptID || material["operation_id"] != plan.OperationID || material["execution_profile"] != "LOCAL_SFU_MOCK_EGRESS" {
			t.Fatalf("fenced original custody load mismatch: %v", err)
		}
		result, err := bicClose(context.Background(), h, plan, generation, key, "merchant_stop")
		if err != nil || result != "held" {
			t.Fatalf("local issued grant should hold unresolved: %q %v", result, err)
		}
		bicAssertOriginalLiability(t, h, plan)
	})

	t.Run("revoked-merchant-denied-but-executor-still-cleans", func(t *testing.T) {
		h, plan, in := bicStarted(t)
		key := t04Key("bic-revoked-issued")
		if _, err := bicReserve(context.Background(), h, h.lp.f.runtime, h.logins.a, h.lp.f.storeA1, key, in); err != nil {
			t.Fatal(err)
		}
		if err := h.logins.service.Logout(context.Background(), h.logins.a); err != nil {
			t.Fatal(err)
		}
		if _, err := bicReserve(context.Background(), h, h.lp.f.runtime, h.logins.a, h.lp.f.storeA1, key, in); err == nil {
			t.Fatal("revoked initiating login recovered previously committed grant")
		}
		lease := bytes.Repeat([]byte{0x62}, 32)
		disposition, generation, mode, err := bicClaim(context.Background(), h, plan, lease)
		if err != nil || disposition != "claimed" || mode != "reconcile" {
			t.Fatalf("revocation removed original cleanup authority: %s/%d/%s %v", disposition, generation, mode, err)
		}
		if _, err := bicLoad(context.Background(), h, plan, generation, lease); err != nil {
			t.Fatalf("revocation denied fenced executor custody: %v", err)
		}
		result, err := bicClose(context.Background(), h, plan, generation, lease, "login_lost")
		if err != nil || result != "held" {
			t.Fatalf("revocation incorrectly finalized issued input: %s %v", result, err)
		}
		bicAssertOriginalLiability(t, h, plan)
	})

	t.Run("authorization-policy-loss-keeps-issued-job", func(t *testing.T) {
		h, plan, in := bicStarted(t)
		key := t04Key("bic-policy-issued")
		if _, err := bicReserve(context.Background(), h, h.lp.f.runtime, h.logins.a, h.lp.f.storeA1, key, in); err != nil {
			t.Fatal(err)
		}
		if _, err := h.registrar.Exec(context.Background(), `SELECT live.revoke_prepared_media($1::uuid,$2::uuid,$3::uuid,$4::text)`,
			h.lp.f.tenantA, h.lp.f.storeA1, h.input.AuthorizationID, "operator_revoke"); err != nil {
			t.Fatal(err)
		}
		if _, err := bicReserve(context.Background(), h, h.lp.f.runtime, h.logins.a, h.lp.f.storeA1, key, in); err == nil {
			t.Fatal("revoked authorization recovered grant from durable receipt")
		}
		lease := bytes.Repeat([]byte{0x67}, 32)
		disposition, generation, mode, err := bicClaim(context.Background(), h, plan, lease)
		if err != nil || disposition != "claimed" || mode != "reconcile" {
			t.Fatalf("policy denial discarded original job: %s/%d/%s %v", disposition, generation, mode, err)
		}
		result, err := bicClose(context.Background(), h, plan, generation, lease, "authorization_lost")
		if err != nil || result != "held" {
			t.Fatalf("policy loss falsely finalized issued input: %q %v", result, err)
		}
		bicAssertOriginalLiability(t, h, plan)
	})
}

func TestLiveMediaExecutionBIC04FenceACLAndNativeGuard(t *testing.T) {
	h, plan, in := bicStarted(t)
	if _, err := bicReserve(context.Background(), h, h.lp.f.runtime, h.logins.a, h.lp.f.storeA1, t04Key("bic-fence-reserve"), in); err != nil {
		t.Fatal(err)
	}
	key1 := bytes.Repeat([]byte{0x63}, 32)
	disposition, generation, mode, err := bicClaim(context.Background(), h, plan, key1)
	if err != nil || disposition != "claimed" || mode != "reconcile" || generation < 1 {
		t.Fatalf("input claim did not fence original job: %s/%d/%s %v", disposition, generation, mode, err)
	}
	wrong := bytes.Repeat([]byte{0x64}, 32)
	if _, err := bicLoad(context.Background(), h, plan, generation, wrong); err == nil {
		t.Fatal("wrong lease key loaded private custody")
	}
	if _, err := bicClose(context.Background(), h, plan, generation, wrong, "merchant_stop"); err == nil {
		t.Fatal("wrong lease key closed admission")
	}
	if _, err := bicLoad(context.Background(), h, plan, generation-1, key1); err == nil {
		t.Fatal("stale generation loaded private custody")
	}
	if _, err := h.executor.Exec(context.Background(), `SELECT live.reserve_media_start($1::uuid,$2::bigint,$3::bytea)`, plan.OperationID, generation, key1); err == nil {
		t.Fatal("legacy Egress Start reserved input-profile attempt")
	}
	// Owner advances only the disposable fixture's lease clock. The next exact
	// claim must fence the old generation without an actual provider call.
	mustExec(t, h.lp.f.owner, `UPDATE integration.operations SET lease_until=clock_timestamp()-interval '1 second' WHERE id=$1`, plan.OperationID)
	key2 := bytes.Repeat([]byte{0x65}, 32)
	disposition, nextGeneration, mode, err := bicClaim(context.Background(), h, plan, key2)
	if err != nil || disposition != "claimed" || nextGeneration <= generation || mode != "reconcile" {
		t.Fatalf("expired lease did not re-fence: %s/%d/%s %v", disposition, nextGeneration, mode, err)
	}
	if _, err := bicLoad(context.Background(), h, plan, generation, key1); err == nil {
		t.Fatal("old generation survived a new claim")
	}
	if _, err := bicClose(context.Background(), h, plan, nextGeneration, key2, "runtime_unavailable"); err != nil {
		t.Fatal(err)
	}
	bicAssertOriginalLiability(t, h, plan)
	for _, item := range []struct{ name, query string }{
		{"issued-local-cannot-be-closed", `UPDATE live.media_input_custody SET state='CLOSED' WHERE attempt_id=$1`},
		{"grant-over-sixty-seconds", `UPDATE live.media_input_custody SET grant_exp=grant_iat+61 WHERE attempt_id=$1`},
		{"closed-admission-cannot-return-reserved", `UPDATE live.media_input_custody SET state='RESERVED' WHERE attempt_id=$1`},
		{"closed-admission-cannot-return-unissued", `UPDATE live.media_input_custody SET state='UNISSUED' WHERE attempt_id=$1`},
		{"nonfinite-attempt-creation", `UPDATE live.media_attempts SET created_at='infinity'::timestamptz WHERE id=$1`},
	} {
		if _, err := h.lp.f.owner.Exec(context.Background(), item.query, plan.AttemptID); err == nil {
			t.Fatalf("owner bypassed frozen input invariant %s", item.name)
		}
	}
	// The native guard, not a best-effort readiness check, must reject both
	// finalization and deletion while any reserved input liability remains.
	if _, err := h.lp.f.owner.Exec(context.Background(), `UPDATE river_media.river_job SET state='completed',finalized_at=clock_timestamp() WHERE id=$1`, plan.JobID); sqlState(err) != "22023" {
		t.Fatalf("native lifecycle guard did not reject finalization: %v", err)
	}
	if _, err := h.lp.f.owner.Exec(context.Background(), `DELETE FROM river_media.river_job WHERE id=$1`, plan.JobID); sqlState(err) != "22023" {
		t.Fatalf("native lifecycle guard did not reject deletion: %v", err)
	}
	bicAssertOriginalLiability(t, h, plan)
	// Owner seeds only the generation budget on this disposable fixture. BIC
	// must stop new leases yet retain the original unresolved operation/job.
	mustExec(t, h.lp.f.owner, `UPDATE integration.operations SET generation=4096 WHERE id=$1`, plan.OperationID)
	disposition, exhaustedGeneration, exhaustedMode, err := bicClaim(context.Background(), h, plan, bytes.Repeat([]byte{0x66}, 32))
	if err != nil || disposition != "held" || exhaustedGeneration != 4096 || exhaustedMode != "" {
		t.Fatalf("generation exhaustion was not held without lease: %s/%d/%s %v", disposition, exhaustedGeneration, exhaustedMode, err)
	}
	bicAssertOriginalLiability(t, h, plan)
	for _, role := range []string{"commerce_runtime", "commerce_media_registrar", "commerce_media_executor", "commerce_media_worker"} {
		for _, table := range []string{"live.prepared_media_input_profiles", "live.media_input_custody"} {
			var direct bool
			err := h.lp.f.owner.QueryRow(context.Background(), `SELECT has_table_privilege($1,$2,'SELECT') OR has_table_privilege($1,$2,'INSERT') OR has_table_privilege($1,$2,'UPDATE') OR has_table_privilege($1,$2,'DELETE')`, role, table).Scan(&direct)
			if err != nil || direct {
				t.Fatalf("private input table ACL granted to %s on %s: %v %v", role, table, direct, err)
			}
		}
	}
	for _, item := range []struct {
		role, signature string
		want            bool
	}{
		{"commerce_media_registrar", "live.register_media_input_profile(uuid)", true},
		{"commerce_runtime", "live.register_media_input_profile(uuid)", false},
		{"commerce_media_executor", "live.register_media_input_profile(uuid)", false},
		{"commerce_runtime", "live.plan_media_input_start(bytea,uuid,uuid,uuid,bigint,text,uuid,bigint)", true},
		{"commerce_media_registrar", "live.plan_media_input_start(bytea,uuid,uuid,uuid,bigint,text,uuid,bigint)", false},
		{"commerce_runtime", "live.reserve_media_input(bytea,uuid,uuid,uuid,bigint)", true},
		{"commerce_media_executor", "live.reserve_media_input(bytea,uuid,uuid,uuid,bigint)", false},
		{"commerce_media_executor", "live.claim_media_input_operation(uuid,bigint,integer,bytea)", true},
		{"commerce_runtime", "live.claim_media_input_operation(uuid,bigint,integer,bytea)", false},
		{"commerce_media_executor", "live.load_media_input_custody(uuid,bigint,bytea)", true},
		{"commerce_media_executor", "live.close_media_input_admission(uuid,bigint,bytea,text)", true},
		{"commerce_media_worker", "live.close_media_input_admission(uuid,bigint,bytea,text)", false},
	} {
		var allowed bool
		if err := h.lp.f.owner.QueryRow(context.Background(), `SELECT has_function_privilege($1,$2::regprocedure,'EXECUTE')`, item.role, item.signature).Scan(&allowed); err != nil || allowed != item.want {
			t.Fatalf("BIC function ACL drift %s %s: %t %v", item.role, item.signature, allowed, err)
		}
	}
	if _, err := h.lp.f.runtime.Exec(context.Background(), `SELECT * FROM live.media_input_custody WHERE attempt_id=$1`, plan.AttemptID); sqlState(err) != "42501" {
		t.Fatalf("runtime direct custody read did not fail with privilege denial: %v", err)
	}
	if _, err := h.lp.f.owner.Exec(context.Background(), `GRANT SELECT ON live.media_input_custody TO commerce_runtime`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = h.lp.f.owner.Exec(context.Background(), `REVOKE SELECT ON live.media_input_custody FROM commerce_runtime`)
	})
	var ready bool
	if err := h.lp.f.runtime.QueryRow(context.Background(), `SELECT live.media_input_plan_ready()`).Scan(&ready); err != nil || ready {
		t.Fatalf("readiness accepted public custody SELECT drift: %v %v", ready, err)
	}
	if _, err := h.lp.f.owner.Exec(context.Background(), `REVOKE SELECT ON live.media_input_custody FROM commerce_runtime`); err != nil {
		t.Fatal(err)
	}
	bicPlanReady(t, h)
}

func TestLiveMediaExecutionBIC05LegacyQueueUnaffected(t *testing.T) {
	h := lmpSetup(t, false)
	plan, err := h.start(t04Key("bic-legacy-control"))
	if err != nil {
		t.Fatal(err)
	}
	var profile, queue string
	var inputChildren int
	err = h.lp.f.owner.QueryRow(context.Background(), `SELECT a.execution_profile,j.queue,
	 (SELECT count(*) FROM live.media_input_custody c WHERE c.attempt_id=a.id)
	 FROM live.media_attempts a JOIN integration.operations o ON o.media_attempt_id=a.id
	 JOIN river_media.river_job j ON j.id=o.job_id WHERE a.id=$1`, plan.AttemptID).
		Scan(&profile, &queue, &inputChildren)
	if err != nil || profile != "PROVIDER_MOCK" || queue != "media_mock_v1" || inputChildren != 0 {
		t.Fatalf("legacy attempt changed by BIC: %s %s %d %v", profile, queue, inputChildren, err)
	}
	if _, err := h.registrar.Exec(context.Background(), `SELECT live.register_media_input_profile($1::uuid)`, h.input.AuthorizationID); err == nil {
		t.Fatal("registrar attached input profile to already planned legacy authorization")
	}
}
