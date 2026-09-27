package foundation_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"

	"livecommerce/internal/command"
	"livecommerce/internal/integrations/livekit"
	"livecommerce/internal/live"
	"livecommerce/internal/platform"
	"livecommerce/migrations"
)

func lmrKeys() (*livekit.MaterialKeyring, error) {
	return livekit.NewMaterialKeyring("lma_key_1", map[string][]byte{"lma_key_1": bytes.Repeat([]byte{0x37}, 32)})
}

// These tests reuse LME's real PG18 roles, native River job, sealed material,
// and loopback-only TLS transport. The provider is always a task-owned mock.
func lmrSetup(t *testing.T, handler http.HandlerFunc) *lmeHarness {
	t.Helper()
	h := lmeSetup(t, handler)
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = h.lp.f.owner.Exec(ctx, `DELETE FROM ops.command_results WHERE principal_id IN ($1,$2,$3,$4) AND operation='live.media.stop'`, h.lp.actor, h.lp.peer, h.lp.other, h.lp.limited)
		_, _ = h.lp.f.owner.Exec(ctx, `DELETE FROM ops.audit_events WHERE principal_id IN ($1,$2,$3,$4) AND action='live.media.stop.requested'`, h.lp.actor, h.lp.peer, h.lp.other, h.lp.limited)
	})
	return h
}

func lmrRequest(ctx context.Context, h *lmeHarness, token, store, key string, in live.MediaStopInput) (live.MediaStopResult, error) {
	var out live.MediaStopResult
	err := platform.WithScope(ctx, h.lp.f.runtime, token, store, "store:read", func(tx pgx.Tx, scope platform.Scope) error {
		var err error
		out, err = h.planner.RequestStop(ctx, tx, scope, token, key, in)
		return err
	})
	return out, err
}

func lmrStop(t *testing.T, h *lmeHarness, key string) live.MediaStopResult {
	t.Helper()
	out, err := lmrRequest(context.Background(), h, h.lp.token, h.lp.f.storeA1, key,
		live.MediaStopInput{SessionID: h.session, AttemptID: h.plan.AttemptID})
	if err != nil {
		t.Fatalf("authorized stop: %v", err)
	}
	if out.SessionID != h.session || out.AttemptID != h.plan.AttemptID || out.OperationID != h.plan.OperationID {
		t.Fatalf("stop result identity: %+v", out)
	}
	return out
}

type lmrFacts struct {
	count, firstGen, lastGen        int64
	first, last, claimed, exhausted *time.Time
	observation                     string
	requested, requester            *string
	operation, resource, status     string
	cleanup, leaseOpen              bool
}

func lmrRead(t *testing.T, h *lmeHarness) lmrFacts {
	t.Helper()
	var f lmrFacts
	var first, last, claimed, exhausted *time.Time
	var observation, requested, requester *string
	err := h.lp.f.owner.QueryRow(context.Background(), `SELECT x.stop_wire_count,
	 coalesce(x.stop_first_generation,0),coalesce(x.stop_last_generation,0),
	 x.stop_first_reserved_at,x.stop_last_reserved_at,x.claim_started_at,x.stop_exhausted_at,
	 x.stop_observation_id::text,x.stop_requested_at::text,x.stop_requested_by::text,
	 o.state,x.resource_state,x.transport_status,x.cleanup_required,
	 coalesce(o.lease_until>clock_timestamp(),false)
	 FROM integration.operations o JOIN live.media_execution_state x ON x.operation_id=o.id WHERE o.id=$1`, h.plan.OperationID).
		Scan(&f.count, &f.firstGen, &f.lastGen, &first, &last, &claimed, &exhausted,
			&observation, &requested, &requester, &f.operation, &f.resource, &f.status, &f.cleanup, &f.leaseOpen)
	if err != nil {
		t.Fatal(err)
	}
	f.first, f.last, f.claimed, f.exhausted = first, last, claimed, exhausted
	if observation != nil {
		f.observation = *observation
	}
	f.requested, f.requester = requested, requester
	return f
}

func lmrStarted(t *testing.T, h *lmeHarness, id string) {
	t.Helper()
	lease := h.claim(t, 30)
	if lease.disposition != "claimed" || lease.mode != "dispatch" {
		t.Fatalf("start claim: %+v", lease)
	}
	h.load(t, lease)
	if err := h.reserve(t, lease); err != nil {
		t.Fatal(err)
	}
	if out, err := h.record(t, lease, "START", id, "EGRESS_ACTIVE", 100, 110, 0); err != nil || out != "observe" {
		t.Fatalf("start fact: %s %v", out, err)
	}
}

func lmrQuery(t *testing.T, h *lmeHarness, lease lmeLease, id, status string, started, updated, ended int64) (string, error) {
	t.Helper()
	var out string
	err := h.executor.QueryRow(context.Background(), `SELECT live.record_media_cleanup_query($1::uuid,$2::bigint,$3::bytea,$4::text,$5::text,$6::text,$7::bigint,$8::bigint,$9::bigint)`,
		h.plan.OperationID, lease.generation, lease.token, id, h.plan.RoomName, status, started, updated, ended).Scan(&out)
	return out, err
}

func lmrEvidence(t *testing.T, h *lmeHarness, f lmrFacts) {
	t.Helper()
	if f.count < 1 || f.count > 2 || f.observation == "" || f.first == nil || f.last == nil || f.firstGen < 1 || f.lastGen < f.firstGen {
		t.Fatalf("incomplete stop reservation: %+v", f)
	}
	var source, id, room, project string
	var generation int64
	err := h.lp.f.owner.QueryRow(context.Background(), `SELECT source,egress_id,room_name,project_id,generation
	 FROM live.media_observations WHERE id=$1::uuid AND attempt_id=$2 AND operation_id=$3`,
		f.observation, h.plan.AttemptID, h.plan.OperationID).Scan(&source, &id, &room, &project, &generation)
	if err != nil || source != "QUERY" || id == "" || room != h.plan.RoomName || project != "project_lma" || generation != f.lastGen {
		t.Fatalf("latest Stop pointer is not exact Query evidence: %s %s %s %s %d %v", source, id, room, project, generation, err)
	}
}

func TestLiveMediaStopLMR01CommandAuthorityCancellationAndACL(t *testing.T) {
	h := lmrSetup(t, func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "SQL gate only", 500) })
	lease := h.claim(t, 30)
	if lease.disposition != "claimed" || lease.mode != "dispatch" {
		t.Fatalf("dispatch: %+v", lease)
	}
	h.load(t, lease) // The loaded worker must still lose to pre-reserve cancellation.
	before := lmpFacts(t, h.lmpHarness)
	key := t04Key("lmr-cancel")
	out := lmrStop(t, h, key)
	if out.State != "cancelled_before_start" {
		t.Fatalf("pre-reserve stop state: %+v", out)
	}
	if replay := lmrStop(t, h, key); replay != out {
		t.Fatalf("receipt replay: %+v vs %+v", replay, out)
	}
	if fresh := lmrStop(t, h, t04Key("lmr-cancel-again")); fresh.State != "cancelled_before_start" {
		t.Fatalf("fresh key replay: %+v", fresh)
	}
	after := lmpFacts(t, h.lmpHarness)
	if after[0] != before[0] || after[1] != before[1] || after[3] != before[3] {
		t.Fatalf("Stop created attempt/operation/job: %v -> %v", before, after)
	}
	f := lmrRead(t, h)
	if f.operation != "CANCELLED" || f.resource != "UNOBSERVED" || !f.cleanup || f.count != 0 || f.requested == nil || f.requester == nil || *f.requester != h.lp.actor || f.leaseOpen {
		t.Fatalf("pre-reserve cancellation not durable/fenced: %+v", f)
	}
	if err := h.reserve(t, lease); sqlState(err) != "ME409" {
		t.Fatalf("loaded Start reserve after Stop: %v", err)
	}
	if h.starts.Load()+h.lists.Load()+h.queries.Load()+h.stops.Load() != 0 {
		t.Fatal("provider I/O during command/cancellation")
	}

	ctx := context.Background()
	for _, signature := range []string{
		"live.request_media_stop(bytea,uuid,uuid,uuid)",
		"live.record_media_cleanup_query(uuid,bigint,bytea,text,text,text,bigint,bigint,bigint)",
	} {
		var owner, path string
		var secdef, fixed bool
		if err := h.lp.f.owner.QueryRow(ctx, `SELECT pg_get_userbyid(p.proowner),p.prosecdef,
		 coalesce(p.proconfig @> ARRAY['search_path=pg_catalog']::text[],false),p.proname
		 FROM pg_proc p WHERE p.oid=to_regprocedure($1)`, signature).Scan(&owner, &secdef, &fixed, &path); err != nil {
			t.Fatalf("missing exact Stop function %s: %v", signature, err)
		}
		if owner != "commerce_media_writer" || !secdef || !fixed {
			t.Fatalf("unsafe Stop function %s: %s %t %t", path, owner, secdef, fixed)
		}
		for role, name := range map[string]string{"native": "commerce_media_worker", "registrar": "commerce_media_registrar", "runtime": "commerce_runtime", "executor": "commerce_media_executor"} {
			var allowed bool
			if err := h.lp.f.owner.QueryRow(ctx, `SELECT has_function_privilege($1,to_regprocedure($2),'EXECUTE')`, name, signature).Scan(&allowed); err != nil {
				t.Fatal(err)
			}
			want := (path == "request_media_stop" && role == "runtime") || (path == "record_media_cleanup_query" && role == "executor")
			if allowed != want {
				t.Fatalf("%s EXECUTE %s=%t, want %t", role, path, allowed, want)
			}
		}
	}
	for _, count := range []int{1, 2} {
		if _, err := h.lp.f.owner.Exec(ctx, `UPDATE live.media_execution_state SET stop_wire_count=$1 WHERE operation_id=$2`, count, h.plan.OperationID); sqlState(err) != "ME409" {
			t.Fatalf("ordinary role-less evidence mutation count=%d passed projection guard: %v", count, err)
		}
		tx, err := h.lp.f.owner.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, `ALTER TABLE live.media_execution_state DISABLE TRIGGER media_stop_projection`); err != nil {
			_ = tx.Rollback(ctx)
			t.Fatal(err)
		}
		_, err = tx.Exec(ctx, `UPDATE live.media_execution_state SET stop_wire_count=$1 WHERE operation_id=$2`, count, h.plan.OperationID)
		_ = tx.Rollback(ctx)
		if sqlState(err) != "23514" {
			t.Fatalf("incomplete Stop count=%d row passed CHECK: %v", count, err)
		}
	}
	var ready bool
	if err := h.executor.QueryRow(ctx, `SELECT live.media_worker_ready()`).Scan(&ready); err != nil || !ready {
		t.Fatalf("clean Stop readiness: %t %v", ready, err)
	}
	const merchantStop = "live.request_media_stop(bytea,uuid,uuid,uuid)"
	if _, err := h.lp.f.owner.Exec(ctx, `GRANT EXECUTE ON FUNCTION `+merchantStop+` TO commerce_media_executor`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = h.lp.f.owner.Exec(context.Background(), `REVOKE EXECUTE ON FUNCTION `+merchantStop+` FROM commerce_media_executor`)
	})
	if err := h.executor.QueryRow(ctx, `SELECT live.media_worker_ready()`).Scan(&ready); err != nil || ready {
		t.Fatalf("extra merchant EXECUTE admitted: %t %v", ready, err)
	}
	if _, err := h.lp.f.owner.Exec(ctx, `REVOKE EXECUTE ON FUNCTION `+merchantStop+` FROM commerce_media_executor`); err != nil {
		t.Fatal(err)
	}
	if err := h.executor.QueryRow(ctx, `SELECT live.media_worker_ready()`).Scan(&ready); err != nil || !ready {
		t.Fatalf("readiness not restored: %t %v", ready, err)
	}
	const privateProjector = "live.project_media_observation(uuid,bigint,bytea,text,text,text,text,bigint,bigint,bigint)"
	if _, err := h.lp.f.owner.Exec(ctx, `GRANT EXECUTE ON FUNCTION `+privateProjector+` TO commerce_hosted_runtime`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = h.lp.f.owner.Exec(context.Background(), `REVOKE EXECUTE ON FUNCTION `+privateProjector+` FROM commerce_hosted_runtime`)
	})
	if err := h.executor.QueryRow(ctx, `SELECT live.media_worker_ready()`).Scan(&ready); err != nil || ready {
		t.Fatalf("hosted grant on private projector admitted: %t %v", ready, err)
	}
	if _, err := h.lp.f.owner.Exec(ctx, `REVOKE EXECUTE ON FUNCTION `+privateProjector+` FROM commerce_hosted_runtime`); err != nil {
		t.Fatal(err)
	}
	if err := h.executor.QueryRow(ctx, `SELECT live.media_worker_ready()`).Scan(&ready); err != nil || !ready {
		t.Fatalf("readiness not restored after private grant: %t %v", ready, err)
	}
}

func TestLiveMediaStopLMR01CurrentAuthorityReplayAndObservedRevocation(t *testing.T) {
	t.Run("current-manager-not-original-and-replay-reauthorizes", func(t *testing.T) {
		h := lmrSetup(t, func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "SQL only", 500) })
		h.claim(t, 30) // Materialize execution state without reserving Start.
		in := live.MediaStopInput{SessionID: h.session, AttemptID: h.plan.AttemptID}
		before := lmrRead(t, h)
		for name, tc := range map[string]struct {
			token, store string
			in           live.MediaStopInput
			want         error
		}{
			"limited":         {h.lp.limitedToken, h.lp.f.storeA1, in, platform.ErrForbidden},
			"other-store":     {h.lp.token, h.lp.f.storeA2, in, command.ErrNotFound},
			"other-tenant":    {h.lp.otherToken, h.lp.f.storeB, in, command.ErrNotFound},
			"unknown-attempt": {h.lp.token, h.lp.f.storeA1, live.MediaStopInput{SessionID: h.session, AttemptID: randomUUID()}, command.ErrNotFound},
			"invalid-token":   {randomToken(), h.lp.f.storeA1, in, platform.ErrUnauthorized},
		} {
			t.Run(name, func(t *testing.T) {
				_, err := lmrRequest(context.Background(), h, tc.token, tc.store, t04Key("lmr01-deny"), tc.in)
				if !errors.Is(err, tc.want) {
					t.Fatalf("%s: %v, want %v", name, err, tc.want)
				}
			})
		}
		if after := lmrRead(t, h); after.count != before.count || after.requested != nil || after.cleanup {
			t.Fatalf("denied request wrote intent: %+v", after)
		}
		key := t04Key("lmr01-peer")
		out, err := lmrRequest(context.Background(), h, h.lp.peerToken, h.lp.f.storeA1, key, in)
		if err != nil || out.State != "cancelled_before_start" {
			t.Fatalf("current peer manager: %+v %v", out, err)
		}
		f := lmrRead(t, h)
		if f.requester == nil || *f.requester != h.lp.peer || f.requested == nil {
			t.Fatalf("Stop actor not current manager: %+v", f)
		}
		var intentEvents, receipts, audits int
		if err := h.lp.f.owner.QueryRow(context.Background(), `SELECT
		 (SELECT count(*) FROM integration.operation_events WHERE operation_id=$1 AND reason_code='media_cancelled_before_start'),
		 (SELECT count(*) FROM ops.command_results WHERE principal_id=$2 AND operation='live.media.stop'),
		 (SELECT count(*) FROM ops.audit_events WHERE principal_id=$2 AND action='live.media.stop.requested')`,
			h.plan.OperationID, h.lp.peer).Scan(&intentEvents, &receipts, &audits); err != nil || intentEvents != 1 || receipts != 1 || audits != 1 {
			t.Fatalf("first Stop accounting event/receipt/audit=%d/%d/%d %v", intentEvents, receipts, audits, err)
		}
		if replay, err := lmrRequest(context.Background(), h, h.lp.peerToken, h.lp.f.storeA1, key, in); err != nil || replay != out {
			t.Fatalf("peer receipt replay: %+v %v", replay, err)
		}
		if _, err := lmrRequest(context.Background(), h, h.lp.peerToken, h.lp.f.storeA1, key,
			live.MediaStopInput{SessionID: h.session, AttemptID: randomUUID()}); !errors.Is(err, command.ErrConflict) {
			t.Fatalf("same receipt key with different Stop request: %v", err)
		}
		if _, err := h.lp.f.owner.Exec(context.Background(), `DELETE FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND permission='live:manage'`, h.lp.f.tenantA, h.lp.f.storeA1, h.lp.peer); err != nil {
			t.Fatal(err)
		}
		if _, err := lmrRequest(context.Background(), h, h.lp.peerToken, h.lp.f.storeA1, key, in); !errors.Is(err, platform.ErrForbidden) {
			t.Fatalf("revoked peer replay admitted: %v", err)
		}
		if err := h.lp.f.owner.QueryRow(context.Background(), `SELECT
		 (SELECT count(*) FROM integration.operation_events WHERE operation_id=$1 AND reason_code='media_cancelled_before_start'),
		 (SELECT count(*) FROM ops.command_results WHERE principal_id=$2 AND operation='live.media.stop'),
		 (SELECT count(*) FROM ops.audit_events WHERE principal_id=$2 AND action='live.media.stop.requested')`,
			h.plan.OperationID, h.lp.peer).Scan(&intentEvents, &receipts, &audits); err != nil || intentEvents != 1 || receipts != 1 || audits != 1 {
			t.Fatalf("replay/revocation wrote duplicate intent: %d/%d/%d %v", intentEvents, receipts, audits, err)
		}
	})
	t.Run("SQL-final-check-after-observed-grant-wait", func(t *testing.T) {
		h := lmrSetup(t, func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "SQL only", 500) })
		h.claim(t, 30) // The SQL final-check waits on this existing operation.
		ctx := context.Background()
		holder, err := h.lp.f.owner.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer holder.Rollback(ctx)
		var holderPID int
		if err := holder.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&holderPID); err != nil {
			t.Fatal(err)
		}
		if _, err := holder.Exec(ctx, `SELECT 1 FROM integration.operations WHERE id=$1 FOR UPDATE`, h.plan.OperationID); err != nil {
			t.Fatal(err)
		}
		pid := make(chan int, 1)
		done := make(chan error, 1)
		go func() {
			err := platform.WithScope(ctx, h.lp.f.runtime, h.lp.token, h.lp.f.storeA1, "store:read", func(tx pgx.Tx, scope platform.Scope) error {
				var backend int
				if err := tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&backend); err != nil {
					return err
				}
				if _, err := tx.Exec(ctx, `SELECT set_config('app.authz_revision',$1,true)`, fmt.Sprint(scope.Revision)); err != nil {
					return err
				}
				pid <- backend
				hash := sha256.Sum256([]byte(h.lp.token))
				var raw []byte
				return tx.QueryRow(ctx, `SELECT live.request_media_stop($1::bytea,$2::uuid,$3::uuid,$4::uuid)`, hash[:], scope.StoreID, h.session, h.plan.AttemptID).Scan(&raw)
			})
			done <- err
		}()
		var waiterPID int
		select {
		case waiterPID = <-pid:
		case <-time.After(3 * time.Second):
			t.Fatal("Stop SQL waiter not entered")
		}
		lmaObserveBlock(t, h.lp.f.owner, waiterPID, holderPID, false)
		if _, err := holder.Exec(ctx, `DELETE FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND permission='live:manage'`, h.lp.f.tenantA, h.lp.f.storeA1, h.lp.actor); err != nil {
			t.Fatal(err)
		}
		if err := holder.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		select {
		case err := <-done:
			if sqlState(err) != "MP403" {
				t.Fatalf("revoked after SQL wait: %v", err)
			}
		case <-time.After(6 * time.Second):
			t.Fatal("Stop waiter did not finish")
		}
		f := lmrRead(t, h)
		if f.count != 0 || f.cleanup || f.requested != nil {
			t.Fatalf("waiter wrote after revocation: %+v", f)
		}
	})
}

func TestLiveMediaStopLMR01RenamedCheckCannotFakeReadiness(t *testing.T) {
	h := lmrSetup(t, func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "SQL only", 500) })
	ctx := context.Background()
	var ready bool
	if err := h.executor.QueryRow(ctx, `SELECT live.media_worker_ready()`).Scan(&ready); err != nil || !ready {
		t.Fatalf("clean readiness: %t %v", ready, err)
	}
	tx, err := h.lp.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `ALTER TABLE live.media_execution_state DROP CONSTRAINT media_stop_budget`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `ALTER TABLE live.media_execution_state ADD CONSTRAINT media_stop_budget CHECK (true)`); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `SELECT live.media_worker_ready()`).Scan(&ready); err != nil || ready {
		t.Fatalf("same-name CHECK(true) admitted: %t %v", ready, err)
	}
}

func TestLiveMediaStopLMR01Populated0036Upgrade(t *testing.T) {
	ctx := context.Background()
	old := lriPre0032Fixture(t)
	mcApplyHistorical(t, old, "0032_legacy_river_isolation.sql", "0033_live_planning.sql", "0034_live_media_authorization.sql", "0035_live_media_plan.sql")
	for _, schema := range []string{"river_payment", "river_expiry", "river_media"} {
		upstream, err := rivermigrate.New(riverpgxv5.New(old.owner), &rivermigrate.Config{Schema: schema, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := upstream.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
			t.Fatal(err)
		}
	}
	mcApplyHistorical(t, old, "post_river/0005_legacy_river_isolation.sql", "post_river/0006_live_media_queue.sql")
	lp := lpHarness{f: old, actor: old.principalA, token: old.tokens["a"]}
	mustExec(t, old.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,'live:read'),($1,$2,$3,'live:manage')`, old.tenantA, old.storeA1, old.principalA)
	draft, err := lpCreate(lp, lp.token, old.storeA1, t04Key("lmr-old-draft"), lpInput("historical active media"))
	if err != nil {
		t.Fatal(err)
	}
	auth := &lmaHarness{lp: lp, session: draft.ID}
	auth.media = auth.binding(t, old.tenantA, old.storeA1, old.principalA, "livekit", "project_lma")
	auth.facebook = auth.binding(t, old.tenantA, old.storeA1, old.principalA, "facebook", "page_lma")
	spec := auth.spec()
	_, registrar := lmaLogin(t, old, "commerce_media_registrar")
	nonce, ciphertext := lmaEnvelope()
	if _, err := lmaRegister(ctx, registrar, spec, nonce, ciphertext); err != nil {
		t.Fatal(err)
	}
	prior := &lmpHarness{lmaHarness: auth, specification: spec,
		input:   live.MediaStartInput{SessionID: draft.ID, AuthorizationID: spec["id"].(string), ExpectedSessionVersion: 1},
		planner: lmpPlanner(t, old.runtime, "river_media")}
	planned, err := lmpHistoricalStart0035(prior, t04Key("lmr-old-plan"))
	if err != nil || planned.State != "READY" {
		t.Fatalf("historical Start plan: %+v %v", planned, err)
	}
	mcApplyHistorical(t, old, "0036_live_media_execution.sql", "post_river/0007_live_media_execution.sql")
	_, executor := lmaLogin(t, old, "commerce_media_executor")
	// This executes the frozen old five-function admission predicate on a real
	// LOGIN. It is not an old executable binary, which is reported separately.
	oldFiveAdmits := func() bool {
		t.Helper()
		var admits bool
		err := executor.QueryRow(ctx, `SELECT
		 (SELECT count(DISTINCT p.proname) FROM pg_catalog.pg_proc p
		  JOIN pg_catalog.pg_namespace n ON n.oid=p.pronamespace
		  WHERE n.nspname='live' AND p.proname IN ('claim_media_operation','load_media_material',
		   'reserve_media_start','record_media_observation','finish_media_uncertain')
		  AND pg_catalog.has_function_privilege(current_user,p.oid,'EXECUTE'))=5
		 AND NOT EXISTS(SELECT 1 FROM pg_catalog.pg_proc p
		  JOIN pg_catalog.pg_namespace n ON n.oid=p.pronamespace
		  WHERE n.nspname='live' AND pg_catalog.has_schema_privilege(current_user,n.oid,'USAGE')
		  AND p.prorettype<>'trigger'::regtype
		  AND pg_catalog.has_function_privilege(current_user,p.oid,'EXECUTE')
		  AND p.proname NOT IN ('claim_media_operation','load_media_material','reserve_media_start',
		   'record_media_observation','finish_media_uncertain','media_worker_ready'))`).Scan(&admits)
		if err != nil {
			t.Fatal(err)
		}
		return admits
	}
	if !oldFiveAdmits() {
		t.Fatal("pre-0037 old five-function admission predicate rejected clean executor")
	}
	var disposition, mode string
	var generation int64
	if err := executor.QueryRow(ctx, `SELECT disposition,generation,mode FROM live.claim_media_operation($1::uuid,$2::bigint,30,$3::bytea)`,
		planned.OperationID, planned.JobID, randomBytes(32)).Scan(&disposition, &generation, &mode); err != nil || disposition != "claimed" || mode != "dispatch" {
		t.Fatalf("populated pre-Stop execution: %s/%d/%s %v", disposition, generation, mode, err)
	}
	var operation string
	if err := old.owner.QueryRow(ctx, `SELECT state FROM integration.operations WHERE id=$1`, planned.OperationID).Scan(&operation); err != nil {
		t.Fatal(err)
	}
	beforeJob := lriRows(t, old, "river_media.river_job", fmt.Sprintf("WHERE id=%d", planned.JobID))
	beforeLedger := lmpMigrationChecksums(t, old)
	if _, ok := beforeLedger["0037_live_media_stop.sql"]; ok {
		t.Fatal("historical fixture already installed Stop")
	}
	if err := migrations.Apply(ctx, old.owner); err != nil {
		t.Fatalf("populated0036 upgrade: %v", err)
	}
	if oldFiveAdmits() {
		t.Fatal("pre-0037 five-function admission predicate accepted new cleanup function")
	}
	for version, checksum := range beforeLedger {
		if lmpMigrationChecksums(t, old)[version] != checksum {
			t.Fatalf("upgrade changed historical checksum %s", version)
		}
	}
	var count int
	var first, request *time.Time
	var afterGeneration int64
	var afterOperation string
	if err := old.owner.QueryRow(ctx, `SELECT x.stop_wire_count,x.stop_first_reserved_at,x.stop_requested_at,o.generation,o.state
	 FROM live.media_execution_state x JOIN integration.operations o ON o.id=x.operation_id WHERE o.id=$1`, planned.OperationID).
		Scan(&count, &first, &request, &afterGeneration, &afterOperation); err != nil || count != 0 || first != nil || request != nil || afterGeneration != generation || afterOperation != operation {
		t.Fatalf("upgrade rewrote live execution: count=%d first=%v request=%v generation=%d state=%s err=%v", count, first, request, afterGeneration, afterOperation, err)
	}
	if afterJob := lriRows(t, old, "river_media.river_job", fmt.Sprintf("WHERE id=%d", planned.JobID)); afterJob != beforeJob {
		t.Fatal("upgrade rewrote native media job")
	}
	var ready bool
	if err := old.owner.QueryRow(ctx, `SELECT live.media_plan_ready() AND live.media_worker_ready()`).Scan(&ready); err != nil || !ready {
		t.Fatalf("populated Stop upgrade not ready: %t %v", ready, err)
	}
	afterFirst := lmpMigrationChecksums(t, old)
	if err := migrations.Apply(ctx, old.owner); err != nil {
		t.Fatalf("repeat Stop upgrade: %v", err)
	}
	for version, checksum := range afterFirst {
		if lmpMigrationChecksums(t, old)[version] != checksum {
			t.Fatalf("repeat Apply changed %s", version)
		}
	}
}

func TestLiveMediaStopLMR04FencedQueryAndPacing(t *testing.T) {
	h := lmrSetup(t, func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "SQL gate only", 500) })
	id := "EG_lmr04"
	lmrStarted(t, h, id)
	if out := lmrStop(t, h, t04Key("lmr-pacing")); out.State != "requested" {
		t.Fatalf("cleanup request: %+v", out)
	}
	lease := h.claim(t, 30)
	if lease.mode != "reconcile" {
		t.Fatalf("reconcile claim: %+v", lease)
	}
	for name, mutate := range map[string]func(*lmeLease, *string){
		"token":      func(l *lmeLease, _ *string) { l.token = randomBytes(32) },
		"generation": func(l *lmeLease, _ *string) { l.generation++ },
		"target":     func(_ *lmeLease, id *string) { *id = "EG_other" },
	} {
		t.Run(name, func(t *testing.T) {
			bad, target := lease, id
			mutate(&bad, &target)
			if _, err := lmrQuery(t, h, bad, target, "EGRESS_ACTIVE", 100, 120, 0); sqlState(err) != "ME409" {
				t.Fatalf("bad %s accepted: %v", name, err)
			}
		})
	}
	if f := lmrRead(t, h); f.count != 0 {
		t.Fatalf("fence consumed budget: %+v", f)
	}
	out, err := lmrQuery(t, h, lease, id, "EGRESS_ACTIVE", 100, 120, 0)
	if err != nil || out != "stop_reserved" {
		t.Fatalf("first exact Query reserve: %s %v", out, err)
	}
	first := lmrRead(t, h)
	lmrEvidence(t, h, first)
	if first.count != 1 || first.firstGen != lease.generation || !first.leaseOpen {
		t.Fatalf("first reservation: %+v", first)
	}
	if out, err = h.record(t, lease, "STOP", id, "EGRESS_ENDING", 100, 130, 0); err != nil || out != "observe" {
		t.Fatalf("Stop ACK must remain unresolved: %s %v", out, err)
	}
	next := h.claim(t, 30)
	if next.mode != "reconcile" || next.generation <= lease.generation {
		t.Fatalf("next claim: %+v", next)
	}
	out, err = lmrQuery(t, h, next, id, "EGRESS_ACTIVE", 100, 140, 0)
	if err != nil || out != "observe" {
		t.Fatalf("ENDING/pacing must suppress second Stop: %s %v", out, err)
	}
	if f := lmrRead(t, h); f.count != 1 || f.operation != "UNKNOWN" {
		t.Fatalf("false second reservation: %+v", f)
	}
}

func TestLiveMediaStopLMR04ActiveBeforeFiveSecondsCannotReserve(t *testing.T) {
	h := lmrSetup(t, func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "SQL only", 500) })
	id := "EG_lmr04early"
	lmrStarted(t, h, id)
	lmrStop(t, h, t04Key("lmr04-early"))
	firstLease := h.claim(t, 30)
	if out, err := lmrQuery(t, h, firstLease, id, "EGRESS_ACTIVE", 100, 120, 0); err != nil || out != "stop_reserved" {
		t.Fatalf("first Stop reservation: %s %v", out, err)
	}
	if out, err := h.record(t, firstLease, "STOP", id, "EGRESS_ACTIVE", 100, 130, 0); err != nil || out != "observe" {
		t.Fatalf("nonterminal first Stop reply: %s %v", out, err)
	}
	first := lmrRead(t, h)
	next := h.claim(t, 30)
	if next.generation <= firstLease.generation || next.mode != "reconcile" {
		t.Fatalf("second generation: %+v", next)
	}
	var stillEarly bool
	if err := h.lp.f.owner.QueryRow(context.Background(), `SELECT clock_timestamp()<$1`, first.first.Add(5*time.Second)).Scan(&stillEarly); err != nil || !stillEarly {
		t.Fatalf("pacing negative no longer inside five-second window: %t %v", stillEarly, err)
	}
	if out, err := lmrQuery(t, h, next, id, "EGRESS_ACTIVE", 100, 140, 0); err != nil || out != "observe" {
		t.Fatalf("fresh ACTIVE illegally reserved second Stop: %s %v", out, err)
	}
	if after := lmrRead(t, h); after.count != 1 || after.operation != "UNKNOWN" || h.stops.Load() != 0 {
		t.Fatalf("early ACTIVE consumed budget or sent wire: %+v", after)
	}
}

func lmrWaitDBClock(t *testing.T, owner *pgxpool.Pool, target time.Time) {
	t.Helper()
	deadline := time.Now().Add(7 * time.Second)
	for time.Now().Before(deadline) {
		var crossed bool
		if err := owner.QueryRow(context.Background(), `SELECT clock_timestamp()>=$1`, target).Scan(&crossed); err != nil {
			t.Fatal(err)
		}
		if crossed {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("database clock did not cross claim freshness bound")
}

func TestLiveMediaStopLMR04SlowQueryAndObservedWait(t *testing.T) {
	t.Run("old-record-path-is-not-a-Stop-ticket", func(t *testing.T) {
		h := lmrSetup(t, func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "SQL only", 500) })
		id := "EG_lmr04source"
		lmrStarted(t, h, id)
		lmrStop(t, h, t04Key("lmr04-source"))
		lease := h.claim(t, 30)
		before := h.facts(t)
		if _, err := h.record(t, lease, "QUERY", id, "EGRESS_ACTIVE", 100, 130, 0); sqlState(err) != "ME409" {
			t.Fatalf("old in-flight worker silently swallowed cleanup: %v", err)
		}
		if got := h.facts(t); got != before {
			t.Fatalf("old Query partially wrote: %+v -> %+v", before, got)
		}
		if out, err := lmrQuery(t, h, lease, id, "EGRESS_ACTIVE", 100, 130, 0); err != nil || out != "stop_reserved" {
			t.Fatalf("new Query path cannot reserve: %s %v", out, err)
		}
		if f := lmrRead(t, h); f.count != 1 || !f.cleanup || f.operation != "UNKNOWN" {
			t.Fatalf("old/new Query path split: %+v", f)
		}
	})
	t.Run("legacy-terminal-Query-is-still-evidence", func(t *testing.T) {
		h := lmrSetup(t, func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "SQL only", 500) })
		id := "EG_lmr04terminal"
		lmrStarted(t, h, id)
		lmrStop(t, h, t04Key("lmr04-legacy-terminal"))
		lease := h.claim(t, 30)
		if out, err := h.record(t, lease, "QUERY", id, "EGRESS_COMPLETE", 100, 140, 130); err != nil || out != "terminal" {
			t.Fatalf("old worker terminal proof rejected: %s %v", out, err)
		}
		if f := lmrRead(t, h); f.operation != "SUCCEEDED" || f.resource != "TERMINAL" || f.count != 0 {
			t.Fatalf("legacy terminal was not conserved: %+v", f)
		}
	})
	t.Run("cleanup-discovered-by-old-Query-rolls-back", func(t *testing.T) {
		h := lmrSetup(t, func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "SQL only", 500) })
		id := "EG_lmr04revoke"
		lmrStarted(t, h, id)
		if _, err := h.registrar.Exec(context.Background(), `SELECT live.revoke_prepared_media($1::uuid,$2::uuid,$3::uuid,$4::text)`, h.lp.f.tenantA, h.lp.f.storeA1, h.input.AuthorizationID, "operator_revoke"); err != nil {
			t.Fatal(err)
		}
		lease := h.claim(t, 30)
		before := h.facts(t)
		if _, err := h.record(t, lease, "QUERY", id, "EGRESS_ACTIVE", 100, 130, 0); sqlState(err) != "ME409" {
			t.Fatalf("old Query discovered cleanup but returned: %v", err)
		}
		if got := h.facts(t); got != before {
			t.Fatalf("old revocation Query partially wrote: %+v -> %+v", before, got)
		}
		if out, err := lmrQuery(t, h, lease, id, "EGRESS_ACTIVE", 100, 130, 0); err != nil || out != "stop_reserved" {
			t.Fatalf("new Query cannot handle revoked cleanup: %s %v", out, err)
		}
	})
	t.Run("actual-slow-local-TLS-Query", func(t *testing.T) {
		entered, release := make(chan struct{}, 1), make(chan struct{})
		var once sync.Once
		var h *lmeHarness
		id := "EG_lmr04slow"
		h = lmrSetup(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/twirp/livekit.Egress/ListEgress" || !lmrReadWire(t, w, r, id, h.plan.RoomName) {
				http.Error(w, "wrong Query", 500)
				return
			}
			select {
			case entered <- struct{}{}:
			default:
			}
			select {
			case <-release:
			case <-r.Context().Done():
				return
			}
			lmeReply(w, `{"items":[`+h.observation(id, "EGRESS_ACTIVE", 100, 140, 0)+`]}`)
		})
		t.Cleanup(func() { once.Do(func() { close(release) }) })
		lmrStarted(t, h, id)
		lmrStop(t, h, t04Key("lmr04-slow"))
		lease := h.claim(t, 30)
		client, err := livekit.New(h.project.Config, h.project.Transport)
		if err != nil {
			t.Fatal(err)
		}
		type result struct {
			obs livekit.Observation
			err error
		}
		done := make(chan result, 1)
		go func() {
			obs, err := client.Query(context.Background(), livekit.Target{RoomName: h.plan.RoomName, EgressID: id})
			done <- result{obs, err}
		}()
		select {
		case <-entered:
		case <-time.After(3 * time.Second):
			t.Fatal("TLS Query did not enter")
		}
		f := lmrRead(t, h)
		if f.claimed == nil {
			t.Fatal("successful claim did not store DB start")
		}
		lmrWaitDBClock(t, h.lp.f.owner, f.claimed.Add(5*time.Second))
		once.Do(func() { close(release) })
		select {
		case got := <-done:
			if got.err != nil || got.obs.EgressID != id {
				t.Fatalf("slow exact Query: %+v %v", got.obs, got.err)
			}
			if out, err := lmrQuery(t, h, lease, got.obs.EgressID, got.obs.Status, got.obs.StartedAtNS, got.obs.UpdatedAtNS, got.obs.EndedAtNS); err != nil || out != "observe" {
				t.Fatalf("slow Query was not safely recorded: %s %v", out, err)
			}
		case <-time.After(4 * time.Second):
			t.Fatal("slow Query did not return")
		}
		if f := lmrRead(t, h); f.count != 0 || f.operation != "UNKNOWN" || f.resource != "OBSERVED" || h.queries.Load() != 1 || h.stops.Load() != 0 {
			t.Fatalf("slow Query authorized Stop: %+v query/stop=%d/%d", f, h.queries.Load(), h.stops.Load())
		}
	})
	t.Run("operation-lock-wait-crosses-five-second-bound", func(t *testing.T) {
		h := lmrSetup(t, func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "SQL only", 500) })
		id := "EG_lmr04lock"
		lmrStarted(t, h, id)
		lmrStop(t, h, t04Key("lmr04-lock"))
		lease := h.claim(t, 30)
		claimed := lmrRead(t, h).claimed
		if claimed == nil {
			t.Fatal("missing claim timestamp")
		}
		holder, err := h.lp.f.owner.Begin(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		defer holder.Rollback(context.Background())
		var holderPID int
		if err := holder.QueryRow(context.Background(), `SELECT pg_backend_pid()`).Scan(&holderPID); err != nil {
			t.Fatal(err)
		}
		if _, err := holder.Exec(context.Background(), `SELECT 1 FROM integration.operations WHERE id=$1 FOR UPDATE`, h.plan.OperationID); err != nil {
			t.Fatal(err)
		}
		conn, err := h.executor.Acquire(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Release()
		var waiterPID int
		if err := conn.QueryRow(context.Background(), `SELECT pg_backend_pid()`).Scan(&waiterPID); err != nil {
			t.Fatal(err)
		}
		type result struct {
			out string
			err error
		}
		done := make(chan result, 1)
		go func() {
			var out string
			err := conn.QueryRow(context.Background(), `SELECT live.record_media_cleanup_query($1::uuid,$2::bigint,$3::bytea,$4::text,$5::text,$6::text,$7::bigint,$8::bigint,$9::bigint)`,
				h.plan.OperationID, lease.generation, lease.token, id, h.plan.RoomName, "EGRESS_ACTIVE", 100, 140, 0).Scan(&out)
			done <- result{out, err}
		}()
		lmaObserveBlock(t, h.lp.f.owner, waiterPID, holderPID, false)
		lmrWaitDBClock(t, h.lp.f.owner, claimed.Add(5*time.Second))
		if err := holder.Commit(context.Background()); err != nil {
			t.Fatal(err)
		}
		select {
		case got := <-done:
			if got.err != nil || got.out != "observe" {
				t.Fatalf("late waiter: %s %v", got.out, got.err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("late waiter did not finish")
		}
		if f := lmrRead(t, h); f.count != 0 || f.resource != "OBSERVED" || f.operation != "UNKNOWN" {
			t.Fatalf("lock wait yielded stale Stop reservation: %+v", f)
		}
	})
}

func lmrReadWire(t *testing.T, w http.ResponseWriter, r *http.Request, expectedID, expectedRoom string) bool {
	t.Helper()
	var payload map[string]json.RawMessage
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&payload); err != nil {
		t.Errorf("provider request JSON: %v", err)
		http.Error(w, "bad JSON", 400)
		return false
	}
	var id, room string
	if raw, ok := payload["egress_id"]; ok {
		_ = json.Unmarshal(raw, &id)
	}
	if raw, ok := payload["room_name"]; ok {
		_ = json.Unmarshal(raw, &room)
	}
	if id != expectedID || (expectedRoom != "" && room != expectedRoom) {
		t.Errorf("wrong frozen target on %s: id=%q room=%q", r.URL.Path, id, room)
		http.Error(w, "wrong target", 400)
		return false
	}
	if r.URL.Path == "/twirp/livekit.Egress/ListEgress" {
		var active bool
		if raw, ok := payload["active"]; !ok || json.Unmarshal(raw, &active) != nil || active {
			t.Error("exact Query changed active=false")
			http.Error(w, "wrong query", 400)
			return false
		}
	}
	return true
}

func lmrDropReply(t *testing.T, w http.ResponseWriter) {
	t.Helper()
	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		t.Errorf("hijack local TLS reply: %v", err)
		return
	}
	_ = conn.Close()
}

func TestLiveMediaStopLMR02LostBeforeAcceptanceSameIDRetry(t *testing.T) {
	var h *lmeHarness
	var accepted atomic.Int32
	id := "EG_lmr02"
	h = lmrSetup(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/twirp/livekit.Egress/ListEgress":
			if !lmrReadWire(t, w, r, id, h.plan.RoomName) {
				return
			}
			lmeReply(w, `{"items":[`+h.observation(id, "EGRESS_ACTIVE", 100, 120, 0)+`]}`)
		case "/twirp/livekit.Egress/StopEgress":
			if !lmrReadWire(t, w, r, id, "") {
				return
			}
			if h.stops.Load() == 1 {
				// Request reached the TLS double, but was dropped before the
				// provider's accept action. This is not a lost response.
				lmrDropReply(t, w)
				return
			}
			accepted.Add(1)
			lmeReply(w, h.observation(id, "EGRESS_COMPLETE", 100, 150, 140))
		default:
			http.Error(w, "unexpected wire", 500)
		}
	})
	lmrStarted(t, h, id)
	if out := lmrStop(t, h, t04Key("lmr02-stop")); out.State != "requested" {
		t.Fatalf("request state: %+v", out)
	}
	h.startWorker(t)
	h.await(t, 40*time.Second, func(f lmeFacts) bool { return f.resource == "TERMINAL" })
	f := lmrRead(t, h)
	lmrEvidence(t, h, f)
	if f.count != 2 || f.operation != "SUCCEEDED" || f.resource != "TERMINAL" || h.starts.Load() != 0 || h.lists.Load() != 0 || h.queries.Load() < 2 || h.stops.Load() != 2 || accepted.Load() != 1 {
		t.Fatalf("before-accept recovery: %+v wire start/list/query/stop=%d/%d/%d/%d acceptedEOS=%d", f,
			h.starts.Load(), h.lists.Load(), h.queries.Load(), h.stops.Load(), accepted.Load())
	}
	if f.first == nil || f.last == nil || f.last.Sub(*f.first) < 5*time.Second || f.lastGen <= f.firstGen {
		t.Fatalf("second Stop did not respect persistent generation/pacing: %+v", f)
	}
}

func TestLiveMediaStopLMR03AcceptedLostReplyEndingAndDelayedActive(t *testing.T) {
	for _, mode := range []string{"ending-suppresses", "delayed-active-bounded-duplicate"} {
		t.Run(mode, func(t *testing.T) {
			var h *lmeHarness
			var once sync.Once
			var accepted atomic.Int32
			id := "EG_lmr03"
			h = lmrSetup(t, func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/twirp/livekit.Egress/ListEgress":
					if !lmrReadWire(t, w, r, id, h.plan.RoomName) {
						return
					}
					status, ended := "EGRESS_ACTIVE", int64(0)
					if accepted.Load() > 0 {
						if mode == "ending-suppresses" {
							status = "EGRESS_ENDING"
							if h.queries.Load() >= 3 {
								status, ended = "EGRESS_COMPLETE", 160
							}
						} else if h.stops.Load() >= 2 {
							status, ended = "EGRESS_COMPLETE", 160
						}
					}
					lmeReply(w, `{"items":[`+h.observation(id, status, 100, 170, ended)+`]}`)
				case "/twirp/livekit.Egress/StopEgress":
					if !lmrReadWire(t, w, r, id, "") {
						return
					}
					once.Do(func() { accepted.Add(1) }) // local provider accepts EOS once
					if h.stops.Load() == 1 {
						lmrDropReply(t, w)
						return
					}
					lmeReply(w, h.observation(id, "EGRESS_ENDING", 100, 150, 0))
				default:
					http.Error(w, "unexpected wire", 500)
				}
			})
			lmrStarted(t, h, id)
			if out := lmrStop(t, h, t04Key("lmr03-stop")); out.State != "requested" {
				t.Fatalf("request state: %+v", out)
			}
			h.startWorker(t)
			h.await(t, 45*time.Second, func(f lmeFacts) bool { return f.resource == "TERMINAL" })
			f := lmrRead(t, h)
			lmrEvidence(t, h, f)
			wantWires := int32(1)
			if mode == "delayed-active-bounded-duplicate" {
				wantWires = 2
			}
			if h.stops.Load() != wantWires || f.count != int64(wantWires) || accepted.Load() != 1 || h.starts.Load() != 0 || h.lists.Load() != 0 || f.resource != "TERMINAL" {
				t.Fatalf("%s: %+v wire start/list/query/stop=%d/%d/%d/%d acceptedEOS=%d", mode, f,
					h.starts.Load(), h.lists.Load(), h.queries.Load(), h.stops.Load(), accepted.Load())
			}
		})
	}
}

// A simple-query implicit transaction has committed only after CommandComplete
// and ReadyForQuery('I'). Hold/drop that acknowledgement after the DB commit,
// not after merely sending SQL, so a killed worker cannot claim wire permission.
type lmrAckConn struct {
	net.Conn
	mode      string
	pending   atomic.Bool
	committed *atomic.Bool
}

func (c *lmrAckConn) Write(p []byte) (int, error) {
	n, err := c.Conn.Write(p)
	if err == nil && n == len(p) && len(p) > 6 && p[0] == 'Q' &&
		int(binary.BigEndian.Uint32(p[1:5])) == len(p)-1 && p[len(p)-1] == 0 &&
		bytes.HasPrefix(bytes.ToLower(bytes.TrimSpace(p[5:len(p)-1])), []byte("select live.record_media_cleanup_query(")) {
		c.pending.Store(true)
	}
	return n, err
}

func (c *lmrAckConn) Read(p []byte) (int, error) {
	if !c.pending.CompareAndSwap(true, false) {
		return c.Conn.Read(p)
	}
	completed := false
	for {
		var head [5]byte
		if _, err := io.ReadFull(c.Conn, head[:]); err != nil {
			return 0, err
		}
		n := int(binary.BigEndian.Uint32(head[1:]))
		if n < 4 || n > 1<<20 {
			return 0, io.ErrUnexpectedEOF
		}
		body := make([]byte, n-4)
		if _, err := io.ReadFull(c.Conn, body); err != nil {
			return 0, err
		}
		if head[0] == 'C' && bytes.HasPrefix(body, []byte("SELECT ")) {
			completed = true
		}
		if head[0] == 'Z' && len(body) == 1 && body[0] == 'I' && completed {
			c.committed.Store(true)
			if c.mode == "hold" {
				_, _ = fmt.Fprintln(os.Stdout, "LMR_ACK_HELD")
				select {}
			}
			_ = c.Conn.Close()
			return 0, io.ErrUnexpectedEOF
		}
		if head[0] == 'E' || head[0] == 'Z' {
			return 0, io.ErrUnexpectedEOF
		}
	}
}

func lmrFaultPool(t *testing.T, base *pgxpool.Pool, mode string, committed *atomic.Bool) *pgxpool.Pool {
	t.Helper()
	cfg := base.Config()
	cfg.MaxConns = 2
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	dial := cfg.ConnConfig.DialFunc
	cfg.ConnConfig.DialFunc = func(ctx context.Context, network, address string) (net.Conn, error) {
		conn, err := dial(ctx, network, address)
		if err != nil {
			return nil, err
		}
		return &lmrAckConn{Conn: conn, mode: mode, committed: committed}, nil
	}
	p, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return p
}

func TestLiveMediaStopCrashChild(t *testing.T) {
	if os.Getenv("LC_LMR_CHILD") != "1" {
		return
	}
	workerDSN, executorDSN, address := os.Getenv("LC_LMR_WORKER_DSN"), os.Getenv("LC_LMR_EXECUTOR_DSN"), os.Getenv("LC_LMR_TLS_ADDR")
	if workerDSN == "" || executorDSN == "" || !strings.HasPrefix(address, "127.0.0.1:") {
		t.Fatal("invalid local child fixture")
	}
	ctx := context.Background()
	worker, err := pgxpool.New(ctx, workerDSN)
	if err != nil {
		t.Fatal("child worker pool")
	}
	defer worker.Close()
	executor, err := pgxpool.New(ctx, executorDSN)
	if err != nil {
		t.Fatal("child executor pool")
	}
	defer executor.Close()
	committed := &atomic.Bool{}
	// The child owns this temporary connection and is deliberately SIGKILLed
	// after the parent observes the committed reservation in PostgreSQL.
	cfg := executor.Config()
	cfg.MaxConns = 2
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	dial := cfg.ConnConfig.DialFunc
	cfg.ConnConfig.DialFunc = func(ctx context.Context, network, address string) (net.Conn, error) {
		conn, err := dial(ctx, network, address)
		if err != nil {
			return nil, err
		}
		return &lmrAckConn{Conn: conn, mode: "hold", committed: committed}, nil
	}
	fault, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal("child fault pool")
	}
	defer fault.Close()
	keys, err := lmrKeys()
	if err != nil {
		t.Fatal("child material key")
	}
	project := live.MediaProject{ProjectID: "project_lma", CredentialVersion: 1, Config: lmeConfig(), Transport: lmeTransport(address)}
	client, err := live.NewMediaClient(ctx, worker, fault, keys, []live.MediaProject{project}, 1)
	if err != nil {
		t.Fatal("child constructor")
	}
	if err := client.Start(ctx); err != nil {
		t.Fatal("child River start")
	}
	select {}
}

func lmrChild(t *testing.T, h *lmeHarness) (*exec.Cmd, <-chan error, <-chan bool) {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(self, "-test.run=^TestLiveMediaStopCrashChild$", "-test.timeout=75s")
	cmd.Env = append(os.Environ(), "LC_LMR_CHILD=1", "LC_LMR_WORKER_DSN="+h.worker.Config().ConnString(),
		"LC_LMR_EXECUTOR_DSN="+h.executor.Config().ConnString(), "LC_LMR_TLS_ADDR="+h.server.Listener.Addr().String())
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		t.Fatal("child failed to start")
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	ack := make(chan bool, 1)
	go func() {
		scan := bufio.NewScanner(stdout)
		for scan.Scan() {
			if scan.Text() == "LMR_ACK_HELD" {
				ack <- true
				return
			}
		}
		ack <- false
	}()
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	return cmd, done, ack
}

func lmrKill(t *testing.T, child *exec.Cmd, done <-chan error) {
	t.Helper()
	if err := child.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("killed child exited cleanly")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("child not reaped")
	}
}

func lmrRescue(t *testing.T, h *lmeHarness) {
	t.Helper()
	ctx := context.Background()
	if _, err := h.lp.f.owner.Exec(ctx, `UPDATE integration.operations SET lease_until=clock_timestamp()-interval '1 second' WHERE id=$1`, h.plan.OperationID); err != nil {
		t.Fatal(err)
	}
	if _, err := h.lp.f.owner.Exec(ctx, `UPDATE river_media.river_job SET attempted_at=clock_timestamp()-interval '2 hours' WHERE id=$1 AND state='running'`, h.plan.JobID); err != nil {
		t.Fatal(err)
	}
}

func TestLiveMediaStopLMR05RealCrashAndCommitAckLoss(t *testing.T) {
	var h *lmeHarness
	id := "EG_lmr05"
	h = lmrSetup(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/twirp/livekit.Egress/ListEgress" || !lmrReadWire(t, w, r, id, h.plan.RoomName) {
			http.Error(w, "unexpected wire", 500)
			return
		}
		lmeReply(w, `{"items":[`+h.observation(id, "EGRESS_ACTIVE", 100, 120, 0)+`]}`)
	})
	lmrStarted(t, h, id)
	lmrStop(t, h, t04Key("lmr05-stop"))
	for n := int64(1); n <= 2; n++ {
		child, done, ack := lmrChild(t, h)
		select {
		case ok := <-ack:
			if !ok {
				t.Fatalf("child did not hold committed Stop %d acknowledgement", n)
			}
		case err := <-done:
			t.Fatalf("child exited before committed Stop %d: %v", n, err)
		case <-time.After(35 * time.Second):
			t.Fatalf("child never reached committed Stop %d acknowledgement", n)
		}
		f := lmrRead(t, h)
		if f.count != n || !f.leaseOpen || h.stops.Load() != 0 {
			t.Fatalf("crash-before-wire reservation %d: %+v stop=%d", n, f, h.stops.Load())
		}
		lmrEvidence(t, h, f)
		lmrKill(t, child, done)
		lmrRescue(t, h)
		if n == 1 {
			// A new generation, and not a sleep-based proof: the DB clock must
			// cross the persisted five-second pacing boundary before retry.
			lmrWaitDBClock(t, h.lp.f.owner, f.first.Add(5*time.Second))
		}
	}
	h.startWorker(t)
	h.await(t, 35*time.Second, func(f lmeFacts) bool { return f.observations >= 4 && !f.leaseOpen })
	f := lmrRead(t, h)
	if f.count != 2 || f.operation != "UNKNOWN" || f.resource != "OBSERVED" || f.exhausted == nil || h.stops.Load() != 0 || h.starts.Load() != 0 || h.queries.Load() < 3 {
		t.Fatalf("crash/restart falsely closed or retried: %+v start/query/stop=%d/%d/%d", f, h.starts.Load(), h.queries.Load(), h.stops.Load())
	}
}

func TestLiveMediaStopLMR05ImplicitAckLossNoWire(t *testing.T) {
	var h *lmeHarness
	id := "EG_lmr05ack"
	h = lmrSetup(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/twirp/livekit.Egress/ListEgress" || !lmrReadWire(t, w, r, id, h.plan.RoomName) {
			http.Error(w, "unexpected wire", 500)
			return
		}
		lmeReply(w, `{"items":[`+h.observation(id, "EGRESS_ACTIVE", 100, 120, 0)+`]}`)
	})
	lmrStarted(t, h, id)
	lmrStop(t, h, t04Key("lmr05-ack"))
	committed := &atomic.Bool{}
	fault := lmrFaultPool(t, h.executor, "drop", committed)
	client, err := live.NewMediaClient(context.Background(), h.worker, fault, h.keys, []live.MediaProject{h.project}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		defer cancel()
		_ = client.StopAndCancel(ctx)
	})
	deadline := time.Now().Add(15 * time.Second)
	for !committed.Load() && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if !committed.Load() {
		t.Fatal("implicit transaction did not reach committed ReadyForQuery(I)")
	}
	f := lmrRead(t, h)
	lmrEvidence(t, h, f)
	if f.count != 1 || h.stops.Load() != 0 || h.starts.Load() != 0 || f.resource != "OBSERVED" || f.operation != "UNKNOWN" {
		t.Fatalf("lost COMMIT acknowledgement issued Stop or forged terminal: %+v start/stop=%d/%d", f, h.starts.Load(), h.stops.Load())
	}
}

func TestLiveMediaStopLMR06AutomaticCleanupBudgetAndTerminalProof(t *testing.T) {
	t.Run("revocation-versus-start-deadline", func(t *testing.T) {
		for _, reason := range []string{"prepared-revoked", "deadline-only"} {
			t.Run(reason, func(t *testing.T) {
				h := lmrSetup(t, func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "SQL only", 500) })
				id := "EG_lmr06"
				lmrStarted(t, h, id)
				ctx := context.Background()
				if reason == "prepared-revoked" {
					if _, err := h.registrar.Exec(ctx, `SELECT live.revoke_prepared_media($1::uuid,$2::uuid,$3::uuid,$4::text)`, h.lp.f.tenantA, h.lp.f.storeA1, h.input.AuthorizationID, "operator_revoke"); err != nil {
						t.Fatal(err)
					}
				} else {
					if _, err := h.lp.f.owner.Exec(ctx, `UPDATE live.prepared_media_authorizations SET start_before=clock_timestamp()-interval '1 second' WHERE id=$1`, h.input.AuthorizationID); err != nil {
						t.Fatal(err)
					}
				}
				lease := h.claim(t, 30)
				if lease.mode != "reconcile" {
					t.Fatalf("claim: %+v", lease)
				}
				out, err := lmrQuery(t, h, lease, id, "EGRESS_ACTIVE", 100, 130, 0)
				want := "stop_reserved"
				if reason == "deadline-only" {
					want = "observe"
				}
				if err != nil || out != want {
					t.Fatalf("%s: Query=%s %v, want %s", reason, out, err, want)
				}
				f := lmrRead(t, h)
				if f.cleanup != (reason == "prepared-revoked") || f.count != map[bool]int64{true: 1, false: 0}[reason == "prepared-revoked"] || f.requested != nil || f.requester != nil {
					t.Fatalf("automatic cleanup polluted merchant provenance: %+v", f)
				}
			})
		}
	})
	t.Run("duration-limit", func(t *testing.T) {
		h := lmrSetup(t, func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "SQL only", 500) })
		id := "EG_lmr06duration"
		lmrStarted(t, h, id)
		lease := h.claim(t, 30)
		out, err := lmrQuery(t, h, lease, id, "EGRESS_ACTIVE", 100, 901000000100, 0)
		if err != nil || out != "stop_reserved" {
			t.Fatalf("duration cleanup: %s %v", out, err)
		}
		if f := lmrRead(t, h); !f.cleanup || f.count != 1 || f.requested != nil {
			t.Fatalf("duration intent: %+v", f)
		}
	})
	t.Run("two-reservations-do-not-close-liability", func(t *testing.T) {
		h := lmrSetup(t, func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "SQL only", 500) })
		id := "EG_lmr06budget"
		lmrStarted(t, h, id)
		lmrStop(t, h, t04Key("lmr06-budget"))
		one := h.claim(t, 30)
		if out, err := lmrQuery(t, h, one, id, "EGRESS_ACTIVE", 100, 120, 0); err != nil || out != "stop_reserved" {
			t.Fatalf("first: %s %v", out, err)
		}
		var finish string
		if err := h.executor.QueryRow(context.Background(), `SELECT live.finish_media_uncertain($1::uuid,$2::bigint,$3::bytea,'remote_unknown')`, h.plan.OperationID, one.generation, one.token).Scan(&finish); err != nil || finish != "observe" {
			t.Fatalf("first uncertain: %s %v", finish, err)
		}
		first := lmrRead(t, h)
		lmrWaitDBClock(t, h.lp.f.owner, first.first.Add(5*time.Second))
		two := h.claim(t, 30)
		if two.generation <= one.generation {
			t.Fatalf("second generation did not advance: %+v", two)
		}
		if out, err := lmrQuery(t, h, two, id, "EGRESS_ACTIVE", 100, 130, 0); err != nil || out != "stop_reserved" {
			t.Fatalf("second: %s %v", out, err)
		}
		if out, err := h.record(t, two, "STOP", id, "EGRESS_ACTIVE", 100, 135, 0); err != nil || out != "observe" {
			t.Fatalf("second nonterminal Stop reply: %s %v", out, err)
		}
		immediate := lmrRead(t, h)
		if immediate.count != 2 || immediate.exhausted == nil || immediate.operation != "UNKNOWN" || immediate.resource == "TERMINAL" {
			t.Fatalf("second Stop reply did not immediately exhaust budget: %+v", immediate)
		}
		var immediateEvents int
		if err := h.lp.f.owner.QueryRow(context.Background(), `SELECT count(*) FROM integration.operation_events WHERE operation_id=$1 AND reason_code='media_stop_budget_exhausted'`, h.plan.OperationID).Scan(&immediateEvents); err != nil || immediateEvents != 1 {
			t.Fatalf("second Stop reply exhaustion event=%d %v", immediateEvents, err)
		}
		three := h.claim(t, 30)
		if out, err := lmrQuery(t, h, three, id, "EGRESS_ACTIVE", 100, 140, 0); err != nil || out != "observe" {
			t.Fatalf("budget exhausted Query-only: %s %v", out, err)
		}
		f := lmrRead(t, h)
		lmrEvidence(t, h, f)
		if f.count != 2 || f.exhausted == nil || f.operation != "UNKNOWN" || f.resource != "OBSERVED" || h.stops.Load() != 0 {
			t.Fatalf("budget falsely reclaimed resource: %+v", f)
		}
		var exhaustedEvents int
		if err := h.lp.f.owner.QueryRow(context.Background(), `SELECT count(*) FROM integration.operation_events WHERE operation_id=$1 AND reason_code='media_stop_budget_exhausted'`, h.plan.OperationID).Scan(&exhaustedEvents); err != nil || exhaustedEvents != 1 {
			t.Fatalf("exhaustion event: %d %v", exhaustedEvents, err)
		}
		four := h.claim(t, 30)
		if out, err := lmrQuery(t, h, four, id, "EGRESS_COMPLETE", 100, 180, 160); err != nil || out != "terminal" {
			t.Fatalf("terminal after exhausted Stop: %s %v", out, err)
		}
		f = lmrRead(t, h)
		if f.count != 2 || f.operation != "SUCCEEDED" || f.resource != "TERMINAL" || h.stops.Load() != 0 {
			t.Fatalf("terminal proof ignored/third Stop: %+v", f)
		}
	})
	for _, ceiling := range []string{"generation", "age"} {
		t.Run("exhausted-budget-still-hits-original-"+ceiling+"-ceiling", func(t *testing.T) {
			h := lmrSetup(t, func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "SQL only", 500) })
			id := "EG_lmr06ceiling"
			lmrStarted(t, h, id)
			lmrStop(t, h, t04Key("lmr06-ceiling"))
			for n := int64(1); n <= 2; n++ {
				if n == 2 {
					first := lmrRead(t, h)
					lmrWaitDBClock(t, h.lp.f.owner, first.first.Add(5*time.Second))
				}
				lease := h.claim(t, 30)
				if out, err := lmrQuery(t, h, lease, id, "EGRESS_ACTIVE", 100, 110+n, 0); err != nil || out != "stop_reserved" {
					t.Fatalf("reservation %d: %s %v", n, out, err)
				}
				var finish string
				if err := h.executor.QueryRow(context.Background(), `SELECT live.finish_media_uncertain($1::uuid,$2::bigint,$3::bytea,'remote_unknown')`, h.plan.OperationID, lease.generation, lease.token).Scan(&finish); err != nil || finish != "observe" {
					t.Fatalf("reservation %d uncertain: %s %v", n, finish, err)
				}
			}
			before := lmrRead(t, h)
			if before.count != 2 || before.operation != "UNKNOWN" {
				t.Fatalf("budget liability before ceiling: %+v", before)
			}
			query := `UPDATE integration.operations SET generation=4096 WHERE id=$1`
			if ceiling == "age" {
				query = `UPDATE integration.operations SET created_at=clock_timestamp()-interval '25 hours' WHERE id=$1`
			}
			if _, err := h.lp.f.owner.Exec(context.Background(), query, h.plan.OperationID); err != nil {
				t.Fatal(err)
			}
			claim := h.claim(t, 30)
			if claim.disposition != "escalated" || claim.mode != "" {
				t.Fatalf("exhaustion bypassed original ceiling: %+v", claim)
			}
			after := lmrRead(t, h)
			if after.count != 2 || after.operation != "UNKNOWN" || after.resource == "TERMINAL" || h.stops.Load() != 0 {
				t.Fatalf("ceiling falsely settled exhausted Stop: %+v", after)
			}
		})
	}
	t.Run("credential-unavailable-retains-liability", func(t *testing.T) {
		var h *lmeHarness
		id := "EG_lmr06credential"
		h = lmrSetup(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/twirp/livekit.Egress/ListEgress" || !lmrReadWire(t, w, r, id, h.plan.RoomName) {
				http.Error(w, "unexpected", 500)
				return
			}
			lmeReply(w, `{"items":[`+h.observation(id, "EGRESS_ACTIVE", 100, 130, 0)+`]}`)
		})
		lmrStarted(t, h, id)
		lmrStop(t, h, t04Key("lmr06-credential"))
		wrong := h.project
		wrong.CredentialVersion++
		client, err := live.NewMediaClient(context.Background(), h.worker, h.executor, h.keys, []live.MediaProject{wrong}, 1)
		if err != nil {
			t.Fatal(err)
		}
		if err := client.Start(context.Background()); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
			defer cancel()
			_ = client.StopAndCancel(ctx)
		})
		h.await(t, 15*time.Second, func(f lmeFacts) bool { return f.result == "credential_unavailable" })
		f := lmrRead(t, h)
		if f.count != 0 || f.operation != "UNKNOWN" || f.resource != "OBSERVED" || h.stops.Load() != 0 || h.starts.Load() != 0 {
			t.Fatalf("unavailable frozen credential caused fallback/closure: %+v", f)
		}
	})
}
