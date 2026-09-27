package foundation_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"livecommerce/internal/command"
	"livecommerce/internal/live"
	"livecommerce/internal/platform"
	"livecommerce/migrations"
)

// This suite runs only against the task-owned PG18 fixture. LMA's synthetic
// registrar provides MOCK custody; no client, provider or network action runs.
type lmpHarness struct {
	*lmaHarness
	planner       *live.MediaPlanner
	specification map[string]any
	input         live.MediaStartInput
}

func lmpPlanner(t *testing.T, pool *pgxpool.Pool, schema string) *live.MediaPlanner {
	t.Helper()
	jobs, err := river.NewClient(riverpgxv5.New(pool), &river.Config{Schema: schema})
	if err != nil {
		t.Fatal(err)
	}
	planner, err := live.NewMediaPlanner(jobs)
	if err != nil {
		t.Fatal(err)
	}
	return planner
}

func lmpSetup(t *testing.T, twoDestinations bool) *lmpHarness {
	t.Helper()
	h := &lmpHarness{lmaHarness: lmaSetup(t)}
	h.specification = h.spec()
	if twoDestinations {
		h.specification["destinations"] = []any{
			lmaDestination(h.facebook, "facebook", "page_lma"),
			lmaDestination(h.instagram, "instagram", "ig_lma"),
		}
	}
	nonce, ciphertext := lmaEnvelope()
	if _, err := lmaRegister(context.Background(), h.registrar, h.specification, nonce, ciphertext); err != nil {
		t.Fatal(err)
	}
	h.input = live.MediaStartInput{
		SessionID: h.session, AuthorizationID: h.specification["id"].(string), ExpectedSessionVersion: 1,
	}
	h.planner = lmpPlanner(t, h.lp.f.runtime, "river_media")
	// Registered after lmaSetup: operation/attempt/job rows are removed before
	// LMA's authorization rows and LSP's program/session rows.
	t.Cleanup(func() {
		ctx := context.Background()
		tx, err := h.lp.f.owner.Begin(ctx)
		if err != nil {
			t.Error(err)
			return
		}
		defer tx.Rollback(ctx)
		if _, err = tx.Exec(ctx, `SET CONSTRAINTS ALL DEFERRED`); err != nil {
			t.Errorf("media plan cleanup constraints: %v", err)
			return
		}
		queries := []string{
			`DELETE FROM integration.operation_events WHERE operation_id IN (SELECT o.id FROM integration.operations o JOIN live.media_attempts a ON a.id=o.media_attempt_id WHERE a.session_id=$1)`,
			`DELETE FROM integration.operations WHERE media_attempt_id IN (SELECT id FROM live.media_attempts WHERE session_id=$1)`,
			// Owner-only teardown, with deferred FKs: remove business ownership before
			// its native job. Missing custody now correctly fails closed while the
			// operation exists; never disable that production guard for a fixture.
			`DELETE FROM river_media.river_job WHERE args->>'operation_id' IN (SELECT start_operation_id::text FROM live.media_attempts WHERE session_id=$1)`,
			`DELETE FROM live.media_attempts WHERE session_id=$1`,
			`DELETE FROM ops.command_results WHERE principal_id=$1 AND operation='live.media.start'`,
			`DELETE FROM ops.audit_events WHERE principal_id=$1 AND action='live.media.start.planned'`,
		}
		for i, query := range queries {
			arg := h.session
			if i >= 4 {
				arg = h.lp.actor
			}
			if _, err = tx.Exec(ctx, query, arg); err != nil {
				t.Errorf("media plan cleanup: %v", err)
				return
			}
		}
		if err = tx.Commit(ctx); err != nil {
			t.Errorf("media plan cleanup commit: %v", err)
		}
	})
	return h
}

func lmpStart(ctx context.Context, h *lmpHarness, planner *live.MediaPlanner, token, store, key string, in live.MediaStartInput) (live.MediaStartResult, error) {
	var result live.MediaStartResult
	err := platform.WithScope(ctx, h.lp.f.runtime, token, store, "store:read", func(tx pgx.Tx, scope platform.Scope) error {
		var err error
		result, err = planner.PlanStart(ctx, tx, scope, token, key, in)
		return err
	})
	return result, err
}

func (h *lmpHarness) start(key string) (live.MediaStartResult, error) {
	return lmpStart(context.Background(), h, h.planner, h.lp.token, h.lp.f.storeA1, key, h.input)
}

// Counts are deliberately scoped to this fixture's session/principal. An
// aborted planner must not leave a River row, ledger row, receipt or audit.
func lmpFacts(t *testing.T, h *lmpHarness) (got [6]int64) {
	t.Helper()
	err := h.lp.f.owner.QueryRow(context.Background(), `SELECT
	 (SELECT count(*) FROM live.media_attempts WHERE session_id=$1),
	 (SELECT count(*) FROM integration.operations WHERE actor_kind='MEDIA_ATTEMPT' AND tenant_id=$2 AND store_id=$3 AND media_attempt_id IN (SELECT id FROM live.media_attempts WHERE session_id=$1)),
	 (SELECT count(*) FROM integration.operation_events WHERE operation_id IN (SELECT id FROM integration.operations WHERE media_attempt_id IN (SELECT id FROM live.media_attempts WHERE session_id=$1))),
	 (SELECT count(*) FROM river_media.river_job WHERE kind='live_media_operation_v1'),
	 (SELECT count(*) FROM ops.command_results WHERE principal_id=$4 AND operation='live.media.start'),
	 (SELECT count(*) FROM ops.audit_events WHERE principal_id=$4 AND action='live.media.start.planned')`,
		h.session, h.lp.f.tenantA, h.lp.f.storeA1, h.lp.actor).
		Scan(&got[0], &got[1], &got[2], &got[3], &got[4], &got[5])
	if err != nil {
		t.Fatal(err)
	}
	return
}

func lmpDelta(t *testing.T, before, after [6]int64, want [6]int64) {
	t.Helper()
	for i := range before {
		if after[i]-before[i] != want[i] {
			t.Fatalf("attempt/operation/event/native-job/receipt/audit delta=%v -> %v, want %v", before, after, want)
		}
	}
}

func lmpNoChange(t *testing.T, h *lmpHarness, before [6]int64, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("denied media plan succeeded")
	}
	lmpDelta(t, before, lmpFacts(t, h), [6]int64{})
}

func TestLiveMediaPlanLMP01AtomicStartAndNativeJob(t *testing.T) {
	for _, destinations := range []int{1, 2} {
		t.Run(fmt.Sprint(destinations), func(t *testing.T) {
			h := lmpSetup(t, destinations == 2)
			before := lmpFacts(t, h)
			out, err := h.start(t04Key("lmp-positive"))
			if err != nil {
				t.Fatal(err)
			}
			if out.State != "READY" || out.SessionID != h.session || out.AttemptID != h.specification["attempt_id"] ||
				!command.ValidID(out.OperationID) || out.JobID <= 0 ||
				out.RoomName != "lc_"+strings.ReplaceAll(out.AttemptID, "-", "") {
				t.Fatalf("invalid frozen start result: %+v", out)
			}
			lmpDelta(t, before, lmpFacts(t, h), [6]int64{1, 1, 1, 1, 1, 1})
			var session, program, authorization, original, operationPrincipal, room, state, profile, actor, action, purpose string
			var sessionVersion int64
			var request, args, receipt []byte
			err = h.lp.f.owner.QueryRow(context.Background(), `SELECT a.session_id::text,a.program_id::text,a.authorization_id::text,
			 a.original_principal_id::text,coalesce(o.principal_id::text,''),a.room_name,p.state,a.execution_profile,o.actor_kind,o.action,o.purpose,
			 s.version,o.request::text::bytea,j.args::text::bytea,c.response::text::bytea
			 FROM live.media_attempts a JOIN live.sessions s ON s.id=a.session_id
			 JOIN live.programs p ON p.id=a.program_id
			 JOIN integration.operations o ON o.id=a.start_operation_id
			 JOIN river_media.river_job j ON j.id=o.job_id
			 JOIN ops.command_results c ON c.tenant_id=a.tenant_id AND c.store_id=a.store_id
			  AND c.principal_id=a.original_principal_id AND c.operation='live.media.start'
			 WHERE a.id=$1`, out.AttemptID).Scan(&session, &program, &authorization, &original, &operationPrincipal, &room,
				&state, &profile, &actor, &action, &purpose, &sessionVersion, &request, &args, &receipt)
			if err != nil {
				t.Fatal(err)
			}
			if session != h.session || program != out.ProgramID || authorization != h.input.AuthorizationID ||
				original != h.lp.actor || operationPrincipal != h.lp.actor || room != out.RoomName || state != "READY" || sessionVersion != 1 ||
				profile != "PROVIDER_MOCK" || actor != "MEDIA_ATTEMPT" || action != "livekit.egress.start" || purpose != "service" {
				t.Fatalf("frozen DB identity mismatch: session=%s program=%s auth=%s original=%s operation-principal=%s state=%s actor=%s", session, program, authorization, original, operationPrincipal, state, actor)
			}
			var requestFields, jobFields map[string]any
			if err = json.Unmarshal(request, &requestFields); err != nil {
				t.Fatal(err)
			}
			if len(requestFields) != 3 || requestFields["attempt_id"] != out.AttemptID || requestFields["session_id"] != h.session || requestFields["version"] != float64(1) {
				t.Fatalf("unsafe operation request: %s", request)
			}
			if err = json.Unmarshal(args, &jobFields); err != nil {
				t.Fatal(err)
			}
			if len(jobFields) != 2 || jobFields["operation_id"] != out.OperationID || jobFields["version"] != float64(1) {
				t.Fatalf("unsafe native job args: %s", args)
			}
			if bytes.Contains(bytes.Join([][]byte{request, args, receipt}, nil), []byte("lma_key_1")) ||
				bytes.Contains(bytes.Join([][]byte{request, args, receipt}, nil), []byte("ciphertext")) {
				t.Fatal("secret material leaked into operation, job or receipt")
			}
			var kind, queue string
			var uniqueKey []byte
			if err = h.lp.f.owner.QueryRow(context.Background(), `SELECT kind,queue,unique_key FROM river_media.river_job WHERE id=$1`, out.JobID).
				Scan(&kind, &queue, &uniqueKey); err != nil || kind != "live_media_operation_v1" || queue != "media_mock_v1" || uniqueKey != nil {
				t.Fatalf("native River identity kind=%s queue=%s unique=%x err=%v", kind, queue, uniqueKey, err)
			}
		})
	}
	t.Run("wrong-schema", func(t *testing.T) {
		h := lmpSetup(t, false)
		before := lmpFacts(t, h)
		_, err := lmpStart(context.Background(), h, lmpPlanner(t, h.lp.f.runtime, "river"), h.lp.token, h.lp.f.storeA1, t04Key("lmp-wrong-schema"), h.input)
		lmpNoChange(t, h, before, err)
	})
}

func TestLiveMediaPlanLMP02ReplayAndOneSession(t *testing.T) {
	h := lmpSetup(t, false)
	secondSpec := h.spec()
	nonce, ciphertext := lmaEnvelope()
	if _, err := lmaRegister(context.Background(), h.registrar, secondSpec, nonce, ciphertext); err != nil {
		t.Fatal(err)
	}
	key := t04Key("lmp-concurrent")
	results := make([]live.MediaStartResult, 2)
	errorsFound := make([]error, 2)
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errorsFound[i] = h.start(key)
		}(i)
	}
	wg.Wait()
	for i, err := range errorsFound {
		if err != nil || results[i] != results[0] {
			t.Fatalf("same-key concurrent result[%d]=%+v err=%v; first=%+v", i, results[i], err, results[0])
		}
	}
	if got := lmpFacts(t, h); got != [6]int64{1, 1, 1, 1, 1, 1} {
		t.Fatalf("concurrent replay facts=%v", got)
	}
	before := lmpFacts(t, h)
	changed := h.input
	changed.ExpectedSessionVersion = 2
	_, err := lmpStart(context.Background(), h, h.planner, h.lp.token, h.lp.f.storeA1, key, changed)
	if !errors.Is(err, command.ErrConflict) {
		t.Fatalf("changed same key: %v", err)
	}
	lmpDelta(t, before, lmpFacts(t, h), [6]int64{})
	_, err = lmpStart(context.Background(), h, h.planner, h.lp.peerToken, h.lp.f.storeA1, key, h.input)
	if !errors.Is(err, command.ErrConflict) {
		t.Fatalf("changed principal same key: %v", err)
	}
	_, err = h.start(t04Key("lmp-second-key"))
	lmpNoChange(t, h, before, err)
	secondInput := h.input
	secondInput.AuthorizationID = secondSpec["id"].(string)
	_, err = lmpStart(context.Background(), h, h.planner, h.lp.token, h.lp.f.storeA1, t04Key("lmp-second-auth"), secondInput)
	lmpNoChange(t, h, before, err)
	// READY freezes authorization custody, including an exact replay of the
	// original registration. A second DRAFT authorization minted no attempt.
	_, err = lmaRegister(context.Background(), h.registrar, h.specification, nonce, ciphertext)
	lmaBad(t, err, "lmp-registrar-canary")
	nextSpec := h.spec()
	// A third registration after READY is also denied.
	_, err = lmaRegister(context.Background(), h.registrar, nextSpec, nonce, ciphertext)
	lmaBad(t, err, "lmp-registrar-canary")
	if _, err = h.registrar.Exec(context.Background(), `SELECT live.revoke_prepared_media($1,$2,$3,'lmp_replay')`, h.lp.f.tenantA, h.lp.f.storeA1, h.input.AuthorizationID); err != nil {
		t.Fatal(err)
	}
	got, err := h.start(key)
	if err != nil || got != results[0] {
		t.Fatalf("authorized replay after revoke=%+v err=%v", got, err)
	}
	lmpDelta(t, before, lmpFacts(t, h), [6]int64{})
	if _, err = h.lp.f.owner.Exec(context.Background(), `DELETE FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND permission='live:manage'`, h.lp.f.tenantA, h.lp.f.storeA1, h.lp.actor); err != nil {
		t.Fatal(err)
	}
	_, err = h.start(key)
	if !errors.Is(err, platform.ErrForbidden) {
		t.Fatalf("revoked merchant replay: %v", err)
	}
	lmpDelta(t, before, lmpFacts(t, h), [6]int64{})
}

func TestLiveMediaPlanLMP03AdmissionAndNoPartialWrites(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*testing.T, *lmpHarness) (string, string, live.MediaStartInput)
	}{
		{"missing-live-grant", func(_ *testing.T, h *lmpHarness) (string, string, live.MediaStartInput) {
			return h.lp.limitedToken, h.lp.f.storeA1, h.input
		}},
		{"foreign-store", func(_ *testing.T, h *lmpHarness) (string, string, live.MediaStartInput) {
			return h.lp.token, h.lp.f.storeA2, h.input
		}},
		{"foreign-tenant", func(_ *testing.T, h *lmpHarness) (string, string, live.MediaStartInput) {
			return h.lp.otherToken, h.lp.f.storeB, h.input
		}},
		{"bad-token", func(_ *testing.T, h *lmpHarness) (string, string, live.MediaStartInput) {
			return randomToken(), h.lp.f.storeA1, h.input
		}},
		{"wrong-version", func(_ *testing.T, h *lmpHarness) (string, string, live.MediaStartInput) {
			in := h.input
			in.ExpectedSessionVersion++
			return h.lp.token, h.lp.f.storeA1, in
		}},
		{"wrong-authorization", func(_ *testing.T, h *lmpHarness) (string, string, live.MediaStartInput) {
			in := h.input
			in.AuthorizationID = randomUUID()
			return h.lp.token, h.lp.f.storeA1, in
		}},
		{"disabled-media", func(t *testing.T, h *lmpHarness) (string, string, live.MediaStartInput) {
			mustExec(t, h.lp.f.owner, `UPDATE integration.bindings SET enabled=false WHERE id=$1`, h.media)
			return h.lp.token, h.lp.f.storeA1, h.input
		}},
		{"revised-destination", func(t *testing.T, h *lmpHarness) (string, string, live.MediaStartInput) {
			mustExec(t, h.lp.f.owner, `UPDATE integration.bindings SET semantic_version=semantic_version+1 WHERE id=$1`, h.facebook)
			return h.lp.token, h.lp.f.storeA1, h.input
		}},
		{"changed-aspect", func(t *testing.T, h *lmpHarness) (string, string, live.MediaStartInput) {
			mustExec(t, h.lp.f.owner, `UPDATE live.programs SET aspect_ratio='9:16' WHERE session_id=$1`, h.session)
			return h.lp.token, h.lp.f.storeA1, h.input
		}},
		{"expired-authorization", func(t *testing.T, h *lmpHarness) (string, string, live.MediaStartInput) {
			mustExec(t, h.lp.f.owner, `UPDATE live.prepared_media_authorizations SET start_before=clock_timestamp()-interval '1 second' WHERE id=$1`, h.input.AuthorizationID)
			return h.lp.token, h.lp.f.storeA1, h.input
		}},
		{"revoked-authorization", func(t *testing.T, h *lmpHarness) (string, string, live.MediaStartInput) {
			_, err := h.registrar.Exec(context.Background(), `SELECT live.revoke_prepared_media($1,$2,$3,'lmp_deny')`, h.lp.f.tenantA, h.lp.f.storeA1, h.input.AuthorizationID)
			if err != nil {
				t.Fatal(err)
			}
			return h.lp.token, h.lp.f.storeA1, h.input
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := lmpSetup(t, false)
			token, store, input := tc.mutate(t, h)
			before := lmpFacts(t, h)
			out, err := lmpStart(context.Background(), h, h.planner, token, store, t04Key("lmp-deny"), input)
			if out != (live.MediaStartResult{}) {
				t.Fatalf("denied plan returned result: %+v", out)
			}
			lmpNoChange(t, h, before, err)
		})
	}
	t.Run("pre-io-input-validation", func(t *testing.T) {
		h := lmpSetup(t, false)
		before := lmpFacts(t, h)
		if _, err := live.NewMediaPlanner(nil); !errors.Is(err, command.ErrInvalid) {
			t.Fatalf("nil jobs: %v", err)
		}
		for _, in := range []live.MediaStartInput{
			{SessionID: "bad", AuthorizationID: h.input.AuthorizationID, ExpectedSessionVersion: 1},
			{SessionID: h.session, AuthorizationID: "bad", ExpectedSessionVersion: 1},
			{SessionID: h.session, AuthorizationID: h.input.AuthorizationID, ExpectedSessionVersion: 0},
		} {
			_, err := lmpStart(context.Background(), h, h.planner, h.lp.token, h.lp.f.storeA1, t04Key("lmp-invalid"), in)
			if !errors.Is(err, command.ErrInvalid) {
				t.Fatalf("invalid input %+v: %v", in, err)
			}
		}
		_, err := lmpStart(context.Background(), h, h.planner, h.lp.token, h.lp.f.storeA1, "short", h.input)
		if !errors.Is(err, command.ErrInvalid) {
			t.Fatalf("invalid key: %v", err)
		}
		lmpDelta(t, before, lmpFacts(t, h), [6]int64{})
	})
}

func TestLiveMediaPlanLMP04ActualRoleAndFrozenDraft(t *testing.T) {
	h := lmpSetup(t, false)
	out, err := h.start(t04Key("lmp-roles"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	_, worker := lmaLogin(t, h.lp.f, "commerce_worker")
	before := lmpFacts(t, h)
	// The ordinary runtime has an INSERT grant on the legacy operation table;
	// its RLS policy, not the absence of a grant, must reject MEDIA_ATTEMPT.
	err = platform.WithScope(ctx, h.lp.f.runtime, h.lp.token, h.lp.f.storeA1, "store:read", func(tx pgx.Tx, scope platform.Scope) error {
		_, err := tx.Exec(ctx, `INSERT INTO integration.operations(
		 id,tenant_id,store_id,principal_id,binding_id,binding_version,provider,external_asset_id,
		 purpose,action,semantic_key,request_hash,request,job_id,actor_kind,media_attempt_id)
		 VALUES($1,$2,$3,$4,$5,1,'livekit','project_lma','service','livekit.egress.start',$6,
			 decode(repeat('01',32),'hex'),jsonb_build_object('attempt_id',$7::text,'session_id',$8::text,'version',1),$9,'MEDIA_ATTEMPT',$7::uuid)`,
			randomUUID(), scope.TenantID, scope.StoreID, scope.PrincipalID, h.media,
			t04Key("lmp-forgery"), out.AttemptID, h.session, out.JobID)
		return err
	})
	if sqlState(err) != "42501" {
		t.Fatalf("runtime MEDIA operation INSERT did not hit RLS: %v", err)
	}
	err = platform.WithScope(ctx, h.lp.f.runtime, h.lp.token, h.lp.f.storeA1, "store:read", func(tx pgx.Tx, scope platform.Scope) error {
		_, err := tx.Exec(ctx, `INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
		 VALUES($1,$2,$3,0,'READY','','forged_media_event')`, scope.TenantID, scope.StoreID, out.OperationID)
		return err
	})
	if sqlState(err) != "42501" {
		t.Fatalf("runtime MEDIA event INSERT did not hit RLS: %v", err)
	}
	for _, pool := range []*pgxpool.Pool{h.lp.f.runtime, worker} {
		if _, err := pool.Exec(ctx, `SELECT nonce,ciphertext FROM live.prepared_media_authorizations WHERE id=$1`, h.input.AuthorizationID); sqlState(err) != "42501" {
			t.Fatalf("ordinary login read ciphertext: %v", err)
		}
		if _, err := pool.Exec(ctx, `DELETE FROM live.media_attempts WHERE id=$1`, out.AttemptID); sqlState(err) != "42501" {
			t.Fatalf("ordinary login deleted attempt: %v", err)
		}
	}
	var seen int
	if err := worker.QueryRow(ctx, `SELECT count(*) FROM integration.operations WHERE id=$1`, out.OperationID).Scan(&seen); err != nil || seen != 0 {
		t.Fatalf("worker sees MEDIA operation: count=%d err=%v", seen, err)
	}
	if err := worker.QueryRow(ctx, `SELECT count(*) FROM integration.operation_events WHERE operation_id=$1`, out.OperationID).Scan(&seen); err != nil || seen != 0 {
		t.Fatalf("worker sees MEDIA event: count=%d err=%v", seen, err)
	}
	token := randomBytes(32)
	if _, err := worker.Exec(ctx, `SELECT * FROM integration.claim_operation($1,30,$2)`, out.OperationID, token); err == nil {
		t.Fatal("ordinary worker claimed MEDIA operation")
	}
	// Give the operation a syntactically valid active lease as owner fixture:
	// without an actor guard the old generic completion would succeed.
	_, err = h.lp.f.owner.Exec(ctx, `UPDATE integration.operations SET state='DISPATCHING',generation=1,lease_mode='dispatch',
	 lease_until=clock_timestamp()+interval '1 minute',lease_token_hash=sha256($2::bytea) WHERE id=$1`, out.OperationID, token)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := worker.Exec(ctx, `SELECT integration.complete_operation($1,1,$2,'SUCCEEDED','forged','')`, out.OperationID, token); err == nil {
		t.Fatal("ordinary worker completed MEDIA operation with valid lease")
	}
	var state string
	if err := h.lp.f.owner.QueryRow(ctx, `SELECT state FROM integration.operations WHERE id=$1`, out.OperationID).Scan(&state); err != nil || state != "DISPATCHING" {
		t.Fatalf("forged completion changed state=%s err=%v", state, err)
	}
	lmpDelta(t, before, lmpFacts(t, h), [6]int64{})
	// Both service and raw SQL are denied after the attempt is persisted.
	_, err = lpUpdate(h.lp, h.lp.token, h.lp.f.storeA1, t04Key("lmp-frozen"), h.session, 1, live.DraftInput{Title: "mutated", AspectRatio: "9:16"})
	if err == nil {
		t.Fatal("Go draft mutation after READY")
	}
	err = platform.WithScope(ctx, h.lp.f.runtime, h.lp.token, h.lp.f.storeA1, "store:read", func(tx pgx.Tx, _ platform.Scope) error {
		_, err := tx.Exec(ctx, `UPDATE live.sessions SET title='forged',version=version+1 WHERE id=$1`, h.session)
		return err
	})
	if err == nil {
		t.Fatal("raw session mutation after READY")
	}
	err = platform.WithScope(ctx, h.lp.f.runtime, h.lp.token, h.lp.f.storeA1, "store:read", func(tx pgx.Tx, _ platform.Scope) error {
		_, err := tx.Exec(ctx, `UPDATE live.programs SET aspect_ratio='9:16' WHERE session_id=$1`, h.session)
		return err
	})
	if err == nil {
		t.Fatal("raw program mutation after READY")
	}
	// Historical MERCHANT claim and completion still work through the same SQL.
	legacyID, _ := t06AuthorityOperation(t)
	legacyToken := randomBytes(32)
	claim := t06AuthorityTake(t, worker, legacyID, legacyToken)
	if claim.disposition != "claimed" || claim.mode != "dispatch" {
		t.Fatalf("legacy MERCHANT claim regressed: %+v", claim)
	}
	if err := t06AuthorityFinish(worker, legacyID, claim.generation, legacyToken, "SUCCEEDED"); err != nil {
		t.Fatalf("legacy MERCHANT completion regressed: %v", err)
	}
}

type lmpJobArgs struct {
	OperationID string `json:"operation_id"`
	Version     int    `json:"version"`
}

func (lmpJobArgs) Kind() string { return "live_media_operation_v1" }

func TestLiveMediaPlanLMP05QueueIntegrityAndRollback(t *testing.T) {
	t.Run("raw-runtime-job-state", func(t *testing.T) {
		h := lmpSetup(t, false)
		ctx := context.Background()
		before := lmpFacts(t, h)
		for _, state := range []string{"available", "scheduled", "pending", "completed", "cancelled"} {
			tx, err := h.lp.f.runtime.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			operation := randomUUID()
			var id int64
			var scheduled time.Time
			err = tx.QueryRow(ctx, `INSERT INTO river_media.river_job(kind,args,max_attempts,queue,state,scheduled_at,finalized_at)
			 VALUES('live_media_operation_v1',jsonb_build_object('operation_id',$1::text,'version',1),3,'media_mock_v1',
			 $2::river_media.river_job_state,clock_timestamp()+interval '1 hour',CASE WHEN $2::text IN ('completed','cancelled') THEN clock_timestamp() END)
			 RETURNING id,scheduled_at`, operation, state).Scan(&id, &scheduled)
			if state == "available" {
				if err != nil {
					t.Fatalf("runtime available future job rejected instead of DB-clock normalized: %v", err)
				}
				var dbNow time.Time
				if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&dbNow); err != nil || scheduled.After(dbNow) {
					t.Fatalf("available job remained parked: scheduled=%s now=%s err=%v", scheduled, dbNow, err)
				}
			} else if sqlState(err) != "22023" {
				t.Fatalf("runtime %s job not rejected by initial family guard: %v", state, err)
			}
			if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
				t.Fatal(rollbackErr)
			}
		}
		lmpDelta(t, before, lmpFacts(t, h), [6]int64{})
	})
	for _, terminal := range []string{"completed", "cancelled"} {
		t.Run("terminal-on-runtime-insert-"+terminal, func(t *testing.T) {
			h := lmpSetup(t, false)
			before := lmpFacts(t, h)
			name := "a_lmp_terminal_" + t04Tag()
			function := pgx.Identifier{"public", name}.Sanitize()
			trigger := pgx.Identifier{name}.Sanitize()
			// The owner fixture alters the row being inserted by the real runtime
			// planner. Without an initial-state guard the linked READY intent could
			// otherwise commit with an already-terminal native job.
			mustExec(t, h.lp.f.owner, fmt.Sprintf(`CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $$
			 BEGIN NEW.state=%s; NEW.finalized_at=clock_timestamp(); RETURN NEW; END $$`, function, quoteLiteral(terminal)))
			mustExec(t, h.lp.f.owner, `CREATE TRIGGER `+trigger+` BEFORE INSERT ON river_media.river_job FOR EACH ROW EXECUTE FUNCTION `+function+`() `)
			t.Cleanup(func() {
				mustExec(t, h.lp.f.owner, `DROP TRIGGER `+trigger+` ON river_media.river_job`)
				mustExec(t, h.lp.f.owner, `DROP FUNCTION `+function+`() `)
			})
			_, err := h.start(t04Key("lmp-terminal-" + terminal))
			lmpNoChange(t, h, before, err)
		})
	}
	t.Run("orphan-and-update-kind", func(t *testing.T) {
		h := lmpSetup(t, false)
		ctx := context.Background()
		jobs, err := river.NewClient(riverpgxv5.New(h.lp.f.owner), &river.Config{Schema: "river_media"})
		if err != nil {
			t.Fatal(err)
		}
		before := lmpFacts(t, h)
		tx, err := h.lp.f.owner.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		job, err := jobs.InsertTx(ctx, tx, lmpJobArgs{OperationID: randomUUID(), Version: 1}, &river.InsertOpts{Queue: "media_mock_v1"})
		if err != nil {
			_ = tx.Rollback(ctx)
			t.Fatalf("deferred orphan fixture rejected at INSERT: %v", err)
		}
		if err = tx.Commit(ctx); err == nil {
			t.Fatal("orphan media job committed")
		}
		var orphans int
		if err = h.lp.f.owner.QueryRow(ctx, `SELECT count(*) FROM river_media.river_job WHERE id=$1`, job.Job.ID).Scan(&orphans); err != nil || orphans != 0 {
			t.Fatalf("orphan persisted count=%d err=%v", orphans, err)
		}
		lmpDelta(t, before, lmpFacts(t, h), [6]int64{})
		out, err := h.start(t04Key("lmp-job-guard"))
		if err != nil {
			t.Fatal(err)
		}
		tx, err = h.lp.f.owner.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, `SET CONSTRAINTS ALL IMMEDIATE`); err != nil {
			_ = tx.Rollback(ctx)
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, `UPDATE river_media.river_job SET kind=kind WHERE id=$1`, out.JobID); err != nil {
			_ = tx.Rollback(ctx)
			t.Fatalf("River no-op kind update rejected: %v", err)
		}
		if _, err = tx.Exec(ctx, `UPDATE river_media.river_job SET kind='external_operation_v1' WHERE id=$1`, out.JobID); err == nil {
			_ = tx.Rollback(ctx)
			t.Fatal("UPDATE(kind) bypassed immediate family guard")
		}
		_ = tx.Rollback(ctx)
		for _, args := range []string{
			fmt.Sprintf(`{"operation_id":%q,"version":1.0}`, out.OperationID),
			fmt.Sprintf(`{"operation_id":%q,"version":1,"extra":true}`, out.OperationID),
		} {
			if _, err = h.lp.f.owner.Exec(ctx, `UPDATE river_media.river_job SET args=$2::jsonb WHERE id=$1`, out.JobID, args); err == nil {
				t.Fatalf("noncanonical job args accepted: %s", args)
			}
		}
	})
	for _, table := range []string{"river_media.river_job", "live.media_attempts", "integration.operations", "integration.operation_events", "ops.command_results", "ops.audit_events"} {
		t.Run(table, func(t *testing.T) {
			h := lmpSetup(t, false)
			ctx := context.Background()
			before := lmpFacts(t, h)
			name := "lmp_fault_" + t04Tag()
			function := pgx.Identifier{"public", name}.Sanitize()
			trigger := pgx.Identifier{name}.Sanitize()
			sequence := pgx.Identifier{"public", name + "_hits"}.Sanitize()
			mustExec(t, h.lp.f.owner, `CREATE SEQUENCE `+sequence)
			mustExec(t, h.lp.f.owner, `GRANT USAGE ON SEQUENCE `+sequence+` TO commerce_runtime,commerce_media_writer`)
			_, err := h.lp.f.owner.Exec(ctx, fmt.Sprintf(`CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $$
			 BEGIN IF current_setting('app.principal_id',true)=%s THEN PERFORM nextval('%s'::regclass); RAISE EXCEPTION 'lmp injected fault'; END IF; RETURN NEW; END $$`,
				function, quoteLiteral(h.lp.actor), sequence))
			if err != nil {
				t.Fatal(err)
			}
			mustExec(t, h.lp.f.owner, `CREATE TRIGGER `+trigger+` BEFORE INSERT ON `+table+` FOR EACH ROW EXECUTE FUNCTION `+function+`() `)
			t.Cleanup(func() {
				mustExec(t, h.lp.f.owner, `DROP TRIGGER `+trigger+` ON `+table)
				mustExec(t, h.lp.f.owner, `DROP FUNCTION `+function+`() `)
				mustExec(t, h.lp.f.owner, `DROP SEQUENCE `+sequence)
			})
			_, err = h.start(t04Key("lmp-fault"))
			lmpNoChange(t, h, before, err)
			var fired bool
			if err = h.lp.f.owner.QueryRow(ctx, `SELECT is_called FROM `+sequence).Scan(&fired); err != nil || !fired {
				t.Fatalf("fault site %s not reached: fired=%t err=%v", table, fired, err)
			}
		})
	}
	t.Run("disabled-queue-gate", func(t *testing.T) {
		h := lmpSetup(t, false)
		var trigger string
		err := h.lp.f.owner.QueryRow(context.Background(), `SELECT tgname FROM pg_trigger WHERE tgrelid='river_media.river_job'::regclass
		 AND NOT tgisinternal AND tgdeferrable ORDER BY tgname LIMIT 1`).Scan(&trigger)
		if err != nil {
			t.Fatal(err)
		}
		name := pgx.Identifier{trigger}.Sanitize()
		mustExec(t, h.lp.f.owner, `ALTER TABLE river_media.river_job DISABLE TRIGGER `+name)
		t.Cleanup(func() { mustExec(t, h.lp.f.owner, `ALTER TABLE river_media.river_job ENABLE TRIGGER `+name) })
		before := lmpFacts(t, h)
		_, err = h.start(t04Key("lmp-gate-off"))
		lmpNoChange(t, h, before, err)
	})
}

type lmpWaitResult struct {
	out live.MediaStartResult
	err error
}

func lmpWaiter(h *lmpHarness, key string) (<-chan int, <-chan lmpWaitResult) {
	pid := make(chan int, 1)
	done := make(chan lmpWaitResult, 1)
	go func() {
		var out live.MediaStartResult
		err := platform.WithScope(context.Background(), h.lp.f.runtime, h.lp.token, h.lp.f.storeA1, "store:read", func(tx pgx.Tx, scope platform.Scope) error {
			var backend int
			if err := tx.QueryRow(context.Background(), `SELECT pg_backend_pid()`).Scan(&backend); err != nil {
				return err
			}
			if _, err := tx.Exec(context.Background(), `SELECT set_config('lock_timeout','0',true)`); err != nil {
				return err
			}
			pid <- backend
			var err error
			out, err = h.planner.PlanStart(context.Background(), tx, scope, h.lp.token, key, h.input)
			return err
		})
		done <- lmpWaitResult{out: out, err: err}
	}()
	return pid, done
}

// Call the fixed SQL entry directly. The native job and SQL request share the
// real runtime transaction, while no Go planner authorization runs afterward.
func lmpDirectSQL(h *lmpHarness, key string, entered chan<- int) (out live.MediaStartResult, err error) {
	ctx := context.Background()
	jobs, err := river.NewClient(riverpgxv5.New(h.lp.f.runtime), &river.Config{Schema: "river_media"})
	if err != nil {
		return out, err
	}
	err = platform.WithScope(ctx, h.lp.f.runtime, h.lp.token, h.lp.f.storeA1, "store:read", func(tx pgx.Tx, scope platform.Scope) error {
		if _, err := tx.Exec(ctx, `SELECT set_config('app.authz_revision',$1,true)`, strconv.FormatInt(scope.Revision, 10)); err != nil {
			return err
		}
		operation := randomUUID()
		job, err := jobs.InsertTx(ctx, tx, lmpJobArgs{OperationID: operation, Version: 1}, &river.InsertOpts{Queue: "media_mock_v1"})
		if err != nil {
			return err
		}
		if entered != nil {
			var pid int
			if err := tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
				return err
			}
			entered <- pid
		}
		hash := sha256.Sum256([]byte(h.lp.token))
		var raw []byte
		if err := tx.QueryRow(ctx, `SELECT live.plan_media_start($1,$2::uuid,$3::uuid,$4::uuid,$5,$6,$7::uuid,$8)`,
			hash[:], scope.StoreID, h.input.SessionID, h.input.AuthorizationID,
			h.input.ExpectedSessionVersion, key, operation, job.Job.ID).Scan(&raw); err != nil {
			return err
		}
		return json.Unmarshal(raw, &out)
	})
	return out, err
}

// lmpHistoricalStart0035 seeds an explicitly pre-0039 database through its
// frozen 0035 SQL and the original command/receipt/River transaction shape.
// It is test-only migration data, not proof that an old executable binary ran.
func lmpHistoricalStart0035(h *lmpHarness, key string) (out live.MediaStartResult, err error) {
	ctx := context.Background()
	jobs, err := river.NewClient(riverpgxv5.New(h.lp.f.runtime), &river.Config{Schema: "river_media"})
	if err != nil {
		return out, err
	}
	err = platform.WithScope(ctx, h.lp.f.runtime, h.lp.token, h.lp.f.storeA1, "store:read", func(tx pgx.Tx, scope platform.Scope) error {
		if err := platform.RequirePermission(ctx, tx, scope, h.lp.token, "live:manage"); err != nil {
			return err
		}
		request := struct {
			PrincipalID string `json:"principal_id"`
			live.MediaStartInput
		}{scope.PrincipalID, h.input}
		if err := command.Run(ctx, tx, scope, "live.media.start", key, request, &out, func() error {
			if err := platform.RequirePermission(ctx, tx, scope, h.lp.token, "live:manage"); err != nil {
				return err
			}
			var ready bool
			if err := tx.QueryRow(ctx, `SELECT live.media_plan_ready()`).Scan(&ready); err != nil {
				return err
			}
			if !ready {
				return command.ErrConflict
			}
			var operation string
			if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text`).Scan(&operation); err != nil {
				return err
			}
			job, err := jobs.InsertTx(ctx, tx, lmpJobArgs{OperationID: operation, Version: 1}, &river.InsertOpts{Queue: "media_mock_v1"})
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `SELECT set_config('app.authz_revision',$1,true)`, strconv.FormatInt(scope.Revision, 10)); err != nil {
				return err
			}
			hash := sha256.Sum256([]byte(h.lp.token))
			var raw []byte
			if err := tx.QueryRow(ctx, `SELECT live.plan_media_start($1,$2::uuid,$3::uuid,$4::uuid,$5,$6,$7::uuid,$8)`,
				hash[:], scope.StoreID, h.input.SessionID, h.input.AuthorizationID, h.input.ExpectedSessionVersion,
				key, operation, job.Job.ID).Scan(&raw); err != nil {
				return err
			}
			if err := json.Unmarshal(raw, &out); err != nil {
				return err
			}
			return command.Audit(ctx, tx, scope, "live.media.start.planned")
		}); err != nil {
			return err
		}
		return platform.RequirePermission(ctx, tx, scope, h.lp.token, "live:manage")
	})
	return out, err
}

func lmpAwaitWaiter(t *testing.T, ch <-chan lmpWaitResult) lmpWaitResult {
	t.Helper()
	select {
	case out := <-ch:
		return out
	case <-time.After(6 * time.Second):
		t.Fatal("media planner waiter did not finish")
		return lmpWaitResult{}
	}
}

func lmpWaitPastDBClock(t *testing.T, owner *pgxpool.Pool, deadline time.Time) {
	t.Helper()
	cutoff := time.Now().Add(4 * time.Second)
	for time.Now().Before(cutoff) {
		var passed bool
		if err := owner.QueryRow(context.Background(), `SELECT clock_timestamp()>=$1`, deadline).Scan(&passed); err != nil {
			t.Fatal(err)
		}
		if passed {
			return
		}
		time.Sleep(10 * time.Millisecond) // Blocking is separately proved by pg_blocking_pids.
	}
	t.Fatal("database clock did not cross prepared deadline")
}

func TestLiveMediaPlanLMP06ObservedWaitAndFinalClock(t *testing.T) {
	t.Run("direct-sql-positive", func(t *testing.T) {
		h := lmpSetup(t, false)
		before := lmpFacts(t, h)
		out, err := lmpDirectSQL(h, t04Key("lmp-direct-positive"), nil)
		if err != nil || out.AttemptID != h.specification["attempt_id"] || out.State != "READY" {
			t.Fatalf("real runtime direct SQL control: %+v %v", out, err)
		}
		lmpDelta(t, before, lmpFacts(t, h), [6]int64{1, 1, 1, 1, 0, 0})
	})
	for _, change := range []string{"grant-revoke", "membership-revision"} {
		t.Run("direct-sql-wait-"+change, func(t *testing.T) {
			h := lmpSetup(t, false)
			ctx := context.Background()
			before := lmpFacts(t, h)
			holder, err := h.lp.f.owner.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer holder.Rollback(ctx)
			var holderPID int
			if err = holder.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&holderPID); err != nil {
				t.Fatal(err)
			}
			if _, err = holder.Exec(ctx, `SELECT id FROM integration.bindings WHERE id=$1 FOR UPDATE`, h.media); err != nil {
				t.Fatal(err)
			}
			entered := make(chan int, 1)
			done := make(chan lmpWaitResult, 1)
			go func() {
				out, err := lmpDirectSQL(h, t04Key("lmp-direct-wait"), entered)
				done <- lmpWaitResult{out: out, err: err}
			}()
			var waiterPID int
			select {
			case waiterPID = <-entered:
			case result := <-done:
				t.Fatalf("direct SQL failed before observed wait: %v", result.err)
			case <-time.After(6 * time.Second):
				t.Fatal("direct SQL did not reach binding wait")
			}
			lmaObserveBlock(t, h.lp.f.owner, waiterPID, holderPID, false)
			switch change {
			case "grant-revoke":
				_, err = holder.Exec(ctx, `DELETE FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND permission='live:manage'`, h.lp.f.tenantA, h.lp.f.storeA1, h.lp.actor)
			case "membership-revision":
				_, err = holder.Exec(ctx, `UPDATE identity.memberships SET authz_revision=authz_revision+1 WHERE tenant_id=$1 AND principal_id=$2`, h.lp.f.tenantA, h.lp.actor)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err = holder.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			result := lmpAwaitWaiter(t, done)
			if result.err == nil || result.out != (live.MediaStartResult{}) {
				t.Fatalf("direct SQL stale access admitted: %+v %v", result.out, result.err)
			}
			lmpDelta(t, before, lmpFacts(t, h), [6]int64{})
		})
	}
	for _, change := range []string{"binding-version", "authorization-revoke", "token-revoke"} {
		t.Run(change, func(t *testing.T) {
			h := lmpSetup(t, false)
			ctx := context.Background()
			before := lmpFacts(t, h)
			holder, err := h.lp.f.owner.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer holder.Rollback(ctx)
			var holderPID int
			if err = holder.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&holderPID); err != nil {
				t.Fatal(err)
			}
			if _, err = holder.Exec(ctx, `SELECT id FROM integration.bindings WHERE id=$1 FOR UPDATE`, h.media); err != nil {
				t.Fatal(err)
			}
			pid, done := lmpWaiter(h, t04Key("lmp-wait-"+change))
			lmaObserveBlock(t, h.lp.f.owner, <-pid, holderPID, false)
			switch change {
			case "binding-version":
				_, err = holder.Exec(ctx, `UPDATE integration.bindings SET semantic_version=semantic_version+1 WHERE id=$1`, h.media)
			case "authorization-revoke":
				_, err = holder.Exec(ctx, `INSERT INTO live.media_authorization_revocations(authorization_id,tenant_id,store_id,reason_code,revoked_by)
				 VALUES($1,$2,$3,'lmp_wait','fixture_owner')`, h.input.AuthorizationID, h.lp.f.tenantA, h.lp.f.storeA1)
			case "token-revoke":
				hash := sha256.Sum256([]byte(h.lp.token))
				_, err = holder.Exec(ctx, `UPDATE identity.sessions SET revoked_at=clock_timestamp() WHERE token_hash=$1`, hash[:])
			}
			if err != nil {
				t.Fatal(err)
			}
			if err = holder.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			result := lmpAwaitWaiter(t, done)
			if result.out != (live.MediaStartResult{}) {
				t.Fatalf("waited denial returned %+v", result.out)
			}
			lmpNoChange(t, h, before, result.err)
		})
	}
	t.Run("final-event-write-deadline", func(t *testing.T) {
		h := lmpSetup(t, false)
		ctx := context.Background()
		var deadline time.Time
		if err := h.lp.f.owner.QueryRow(ctx, `UPDATE live.prepared_media_authorizations
		 SET start_before=clock_timestamp()+interval '2 seconds' WHERE id=$1 RETURNING start_before`, h.input.AuthorizationID).Scan(&deadline); err != nil {
			t.Fatal(err)
		}
		name := "lmp_last_" + t04Tag()
		function := pgx.Identifier{"public", name}.Sanitize()
		trigger := pgx.Identifier{name}.Sanitize()
		lockKey := int64(time.Now().UnixNano())
		_, err := h.lp.f.owner.Exec(ctx, fmt.Sprintf(`CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $$
		 BEGIN IF NEW.tenant_id=%s::uuid AND NEW.reason_code='media_start_planned' THEN PERFORM pg_advisory_xact_lock(%d); END IF; RETURN NEW; END $$`,
			function, quoteLiteral(h.lp.f.tenantA), lockKey))
		if err != nil {
			t.Fatal(err)
		}
		mustExec(t, h.lp.f.owner, `CREATE TRIGGER `+trigger+` BEFORE INSERT ON integration.operation_events FOR EACH ROW EXECUTE FUNCTION `+function+`() `)
		t.Cleanup(func() {
			mustExec(t, h.lp.f.owner, `DROP TRIGGER `+trigger+` ON integration.operation_events`)
			mustExec(t, h.lp.f.owner, `DROP FUNCTION `+function+`() `)
		})
		before := lmpFacts(t, h)
		holder, err := h.lp.f.owner.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer holder.Rollback(ctx)
		var holderPID int
		if err = holder.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&holderPID); err != nil {
			t.Fatal(err)
		}
		if _, err = holder.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, lockKey); err != nil {
			t.Fatal(err)
		}
		pid, done := lmpWaiter(h, t04Key("lmp-final-clock"))
		lmaObserveBlock(t, h.lp.f.owner, <-pid, holderPID, true)
		lmpWaitPastDBClock(t, h.lp.f.owner, deadline)
		if err = holder.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		result := lmpAwaitWaiter(t, done)
		if result.out != (live.MediaStartResult{}) {
			t.Fatalf("final-clock denial returned %+v", result.out)
		}
		lmpNoChange(t, h, before, result.err)
	})
	t.Run("concurrent-draft-update", func(t *testing.T) {
		h := lmpSetup(t, false)
		var plan live.MediaStartResult
		var planErr, updateErr error
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); plan, planErr = h.start(t04Key("lmp-race-plan")) }()
		go func() {
			defer wg.Done()
			_, updateErr = lpUpdate(h.lp, h.lp.token, h.lp.f.storeA1, t04Key("lmp-race-edit"), h.session, 1, live.DraftInput{Title: "changed before planning", AspectRatio: "9:16"})
		}()
		wg.Wait()
		if (planErr == nil) == (updateErr == nil) {
			t.Fatalf("nonserial outcome: plan=%+v err=%v updateErr=%v", plan, planErr, updateErr)
		}
		if planErr == nil {
			if facts := lmpFacts(t, h); facts != [6]int64{1, 1, 1, 1, 1, 1} {
				t.Fatalf("successful plan facts=%v", facts)
			}
		} else if facts := lmpFacts(t, h); facts != [6]int64{} {
			t.Fatalf("draft update won but plan wrote=%v", facts)
		}
	})
}

type lmpLegacyArgs struct {
	OperationID string `json:"operation_id"`
	Version     int    `json:"version"`
}

func (lmpLegacyArgs) Kind() string { return "external_operation_v1" }

func lmpMigrationChecksums(t *testing.T, f *testFixture) map[string]string {
	t.Helper()
	rows, err := f.owner.Query(context.Background(), `SELECT version,checksum FROM public.lc_schema_migrations`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	versions := map[string]string{}
	for rows.Next() {
		var version, checksum string
		if err := rows.Scan(&version, &checksum); err != nil {
			t.Fatal(err)
		}
		versions[version] = checksum
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return versions
}

func TestLiveMediaPlanLMP07PopulatedUpgradeAndLegacyControl(t *testing.T) {
	f := lriPre0032Fixture(t)
	ctx := context.Background()
	jobClient, err := river.NewClient(riverpgxv5.New(f.owner), &river.Config{Schema: "river"})
	if err != nil {
		t.Fatal(err)
	}
	operation := randomUUID()
	job, err := jobClient.Insert(ctx, lmpLegacyArgs{OperationID: operation, Version: 1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	tenant, store, principal, binding := randomUUID(), randomUUID(), randomUUID(), randomUUID()
	mustExec(t, f.owner, `INSERT INTO control.tenants(id,name) VALUES($1,'LMP old tenant')`, tenant)
	mustExec(t, f.owner, `INSERT INTO control.stores(tenant_id,id,name,currency) VALUES($1,$2,'LMP old store','USD')`, tenant, store)
	mustExec(t, f.owner, `INSERT INTO identity.principals(id) VALUES($1)`, principal)
	mustExec(t, f.owner, `INSERT INTO identity.memberships(tenant_id,principal_id) VALUES($1,$2)`, tenant, principal)
	mustExec(t, f.owner, `INSERT INTO integration.bindings(id,tenant_id,store_id,principal_id,provider,external_asset_id) VALUES($1,$2,$3,$4,'mock','old-asset')`, binding, tenant, store, principal)
	mustExec(t, f.owner, `INSERT INTO integration.operations(id,tenant_id,store_id,principal_id,binding_id,binding_version,provider,external_asset_id,purpose,action,semantic_key,request_hash,request,job_id)
	 VALUES($1,$2,$3,$4,$5,1,'mock','old-asset','service','mock.old','lmp:old:1',decode(repeat('11',32),'hex'),'{}',$6)`, operation, tenant, store, principal, binding, job.Job.ID)
	oldJob := lriRows(t, f, "river.river_job", fmt.Sprintf("WHERE id=%d", job.Job.ID))
	oldLedger := lmpMigrationChecksums(t, f)
	if len(oldLedger) == 0 {
		t.Fatal("historical migration ledger empty")
	}
	if err = migrations.Apply(ctx, f.owner); err != nil {
		t.Fatalf("populated upgrade: %v", err)
	}
	for version, checksum := range oldLedger {
		if got := lmpMigrationChecksums(t, f)[version]; got != checksum {
			t.Fatalf("historical migration %s checksum changed", version)
		}
	}
	if got := lriRows(t, f, "river.river_job", fmt.Sprintf("WHERE id=%d", job.Job.ID)); got != oldJob {
		t.Fatal("historical legacy River job changed")
	}
	var kind, actor, action string
	if err = f.owner.QueryRow(ctx, `SELECT actor_kind,action,provider FROM integration.operations WHERE id=$1`, operation).Scan(&actor, &action, &kind); err != nil || actor != "MERCHANT" || action != "mock.old" || kind != "mock" {
		t.Fatalf("historical operation relabelled: actor=%s action=%s provider=%s err=%v", actor, action, kind, err)
	}
	var ready bool
	if err = f.owner.QueryRow(ctx, `SELECT live.media_plan_ready()`).Scan(&ready); err != nil || !ready {
		t.Fatalf("native media queue not ready after migration: %t %v", ready, err)
	}
	var newSchemaJobs int
	if err = f.owner.QueryRow(ctx, `SELECT count(*) FROM river_media.river_job`).Scan(&newSchemaJobs); err != nil || newSchemaJobs != 0 {
		t.Fatalf("historical job moved into media lane: %d %v", newSchemaJobs, err)
	}
	postFirst := lriLedger(t, f, "")
	if err = migrations.Apply(ctx, f.owner); err != nil {
		t.Fatalf("repeat Apply: %v", err)
	}
	if got := lriLedger(t, f, ""); got != postFirst {
		t.Fatal("repeat Apply changed migration ledger")
	}
	if got := lriRows(t, f, "river.river_job", fmt.Sprintf("WHERE id=%d", job.Job.ID)); got != oldJob {
		t.Fatal("repeat Apply changed legacy job")
	}
	if err = f.owner.QueryRow(ctx, `SELECT live.media_plan_ready()`).Scan(&ready); err != nil || !ready {
		t.Fatalf("repeat Apply lost queue gate: %t %v", ready, err)
	}
}
