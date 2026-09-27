package foundation_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/command"
	"livecommerce/internal/live"
	"livecommerce/internal/pagination"
	"livecommerce/internal/platform"
)

func studioList(h lpHarness, token, store string, request pagination.Request) (pagination.Page[live.Draft], error) {
	ctx := context.Background()
	return t04Scoped(ctx, h.f, token, store, "store:read", func(tx pgx.Tx, scope platform.Scope) (pagination.Page[live.Draft], error) {
		return live.ListDrafts(ctx, tx, scope, token, request)
	})
}

func studioGet(h lpHarness, token, store, session string) (live.Studio, error) {
	ctx := context.Background()
	return t04Scoped(ctx, h.f, token, store, "store:read", func(tx pgx.Tx, scope platform.Scope) (live.Studio, error) {
		return live.GetStudio(ctx, tx, scope, token, session)
	})
}

func studioJSON(t *testing.T, value any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func studioKeys(t *testing.T, value map[string]any, want ...string) {
	t.Helper()
	if len(value) != len(want) {
		t.Fatalf("fields=%v want=%v", value, want)
	}
	for _, key := range want {
		if _, ok := value[key]; !ok {
			t.Fatalf("missing %q in %v", key, value)
		}
	}
}

type studioBeforeProjectionTx struct {
	pgx.Tx
	once   sync.Once
	before func()
}

func (tx *studioBeforeProjectionTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if strings.Contains(sql, "live.read_studio_media") {
		tx.once.Do(tx.before)
	}
	return tx.Tx.QueryRow(ctx, sql, args...)
}

func TestStudioBackendSTU01DraftListAndAuthority(t *testing.T) {
	h := lpSetup(t)
	first, err := lpCreate(h, h.token, h.f.storeA1, t04Key("studio-first"), lpInput("first"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := lpCreate(h, h.token, h.f.storeA1, t04Key("studio-second"), lpInput("second"))
	if err != nil {
		t.Fatal(err)
	}
	third, err := lpCreate(h, h.token, h.f.storeA1, t04Key("studio-third"), lpInput("third"))
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := lpCreate(h, h.token, h.f.storeA2, t04Key("studio-foreign"), lpInput("foreign"))
	if err != nil {
		t.Fatal(err)
	}
	page, err := studioList(h, h.token, h.f.storeA1, pagination.Request{Limit: 2})
	if err != nil || len(page.Items) != 2 || page.NextCursor == "" {
		t.Fatalf("first page: %+v err=%v", page, err)
	}
	if page.Items[0].ID != third.ID || page.Items[1].ID != second.ID {
		t.Fatalf("descending keyset order: %+v", page.Items)
	}
	next, err := studioList(h, h.token, h.f.storeA1, pagination.Request{Limit: 2, Cursor: page.NextCursor})
	if err != nil || len(next.Items) != 1 || next.Items[0].ID != first.ID || next.NextCursor != "" {
		t.Fatalf("second page: %+v err=%v", next, err)
	}
	for _, tc := range []struct {
		token, store, cursor string
		want                 error
	}{
		{h.token, h.f.storeA2, page.NextCursor, command.ErrInvalid},
		{h.otherToken, h.f.storeB, page.NextCursor, command.ErrInvalid},
		{h.limitedToken, h.f.storeA1, "", platform.ErrForbidden},
	} {
		if _, err := studioList(h, tc.token, tc.store, pagination.Request{Limit: 2, Cursor: tc.cursor}); !errors.Is(err, tc.want) {
			t.Fatalf("list scope token/store=%s/%s error=%v want=%v", tc.token, tc.store, err, tc.want)
		}
	}
	other, err := studioList(h, h.token, h.f.storeA2, pagination.Request{})
	if err != nil || len(other.Items) != 1 || other.Items[0].ID != foreign.ID {
		t.Fatalf("store isolation: %+v err=%v", other, err)
	}
	for _, tc := range []struct {
		token, store string
		want         error
	}{
		{h.token, h.f.storeA2, command.ErrNotFound},
		{h.otherToken, h.f.storeB, command.ErrNotFound},
		{h.limitedToken, h.f.storeA1, platform.ErrForbidden},
	} {
		if _, err := studioGet(h, tc.token, tc.store, first.ID); !errors.Is(err, tc.want) {
			t.Fatalf("detail scope error=%v want=%v", err, tc.want)
		}
	}
	readOnly, err := lpGet(h, h.peerToken, h.f.storeA1, first.ID)
	if err != nil || readOnly.ID != first.ID {
		t.Fatalf("existing read control: %+v %v", readOnly, err)
	}
	updated, err := lpUpdate(h, h.token, h.f.storeA1, t04Key("studio-edit"), first.ID, 1, lpInput("edited"))
	if err != nil || updated.Version != 2 || updated.Title != "edited" {
		t.Fatalf("edit: %+v %v", updated, err)
	}
	if _, err := lpUpdate(h, h.token, h.f.storeA1, t04Key("studio-stale"), first.ID, 1, lpInput("stale")); !errors.Is(err, command.ErrConflict) {
		t.Fatalf("stale version accepted: %v", err)
	}
	key := t04Key("studio-replay")
	replayed, err := lpUpdate(h, h.token, h.f.storeA1, key, first.ID, 2, lpInput("replayed"))
	if err != nil {
		t.Fatal(err)
	}
	again, err := lpUpdate(h, h.token, h.f.storeA1, key, first.ID, 2, lpInput("replayed"))
	if err != nil || !reflect.DeepEqual(again, replayed) {
		t.Fatalf("idempotent replay: %+v %+v %v", again, replayed, err)
	}
	if _, err := lpUpdate(h, h.token, h.f.storeA1, key, first.ID, 2, lpInput("changed replay")); !errors.Is(err, command.ErrConflict) {
		t.Fatalf("changed replay accepted: %v", err)
	}
	got, err := studioGet(h, h.token, h.f.storeA1, first.ID)
	if err != nil || got.Draft.ID != first.ID || got.Draft.Title != "replayed" || got.Draft.Version != 3 {
		t.Fatalf("current detail: %+v err=%v", got, err)
	}
	// Read permission without management still gets a useful, safe Studio.
	mustExec(t, h.f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,'live:read')`, h.f.tenantA, h.f.storeA1, h.limited)
	ro, err := studioGet(h, h.limitedToken, h.f.storeA1, first.ID)
	if err != nil || ro.CanManage || ro.Draft.ID != first.ID {
		t.Fatalf("read-only Studio: %+v err=%v", ro, err)
	}
	if _, err := lpCreate(h, h.limitedToken, h.f.storeA1, t04Key("studio-readonly"), lpInput("denied")); !errors.Is(err, platform.ErrForbidden) {
		t.Fatalf("read-only create accepted: %v", err)
	}
}

func TestStudioBackendSTU02SafeProjectionAndRevocation(t *testing.T) {
	h := lmpSetup(t, true)
	studio, err := studioGet(h.lp, h.lp.token, h.lp.f.storeA1, h.session)
	if err != nil {
		t.Fatal(err)
	}
	projected := studioJSON(t, studio)
	studioKeys(t, projected, "draft", "prepared", "attempt", "can_manage")
	if projected["can_manage"] != true || projected["attempt"] != nil {
		t.Fatalf("incorrect candidate state: %v", projected)
	}
	prepared, ok := projected["prepared"].(map[string]any)
	if !ok {
		t.Fatalf("prepared missing: %v", projected)
	}
	studioKeys(t, prepared, "authorization_id", "session_version", "start_before", "environment", "destinations")
	if prepared["authorization_id"] != h.input.AuthorizationID || prepared["environment"] != "MOCK" || prepared["session_version"] != float64(1) {
		t.Fatalf("candidate mismatch: %v", prepared)
	}
	destinations, ok := prepared["destinations"].([]any)
	if !ok || len(destinations) != 2 {
		t.Fatalf("destination count: %v", prepared["destinations"])
	}
	for i, destination := range destinations {
		item := destination.(map[string]any)
		studioKeys(t, item, "ordinal", "provider")
		if item["ordinal"] != float64(i+1) {
			t.Fatalf("destination order: %v", destinations)
		}
	}
	// Inspect the SECURITY DEFINER result itself, not only the Go decoder. The
	// runtime sees exactly two fixed JSON fields and no material or room data.
	var rawProjection []byte
	err = platform.WithScope(context.Background(), h.lp.f.runtime, h.lp.token, h.lp.f.storeA1, "store:read", func(tx pgx.Tx, scope platform.Scope) error {
		if _, e := tx.Exec(context.Background(), `SELECT set_config('app.authz_revision',$1,true)`, strconv.FormatInt(scope.Revision, 10)); e != nil {
			return e
		}
		if e := tx.QueryRow(context.Background(), `SELECT live.read_studio_media($1,$2,$3)`, tokenHash(h.lp.token), h.lp.f.storeA1, h.session).Scan(&rawProjection); e != nil {
			return e
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var rawFields map[string]any
	if err := json.Unmarshal(rawProjection, &rawFields); err != nil {
		t.Fatal(err)
	}
	studioKeys(t, rawFields, "prepared", "attempt")
	if bytes.Contains(rawProjection, []byte("project_lma")) || bytes.Contains(rawProjection, []byte("unit.livekit.cloud")) || bytes.Contains(rawProjection, []byte("stream")) {
		t.Fatalf("raw projection leaked private media data: %s", rawProjection)
	}
	err = platform.WithScope(context.Background(), h.lp.f.runtime, h.lp.token, h.lp.f.storeA1, "store:read", func(tx pgx.Tx, scope platform.Scope) error {
		if _, e := tx.Exec(context.Background(), `SELECT set_config('app.authz_revision',$1,true)`, strconv.FormatInt(scope.Revision, 10)); e != nil {
			return e
		}
		var denied []byte
		return tx.QueryRow(context.Background(), `SELECT live.read_studio_media($1,$2,$3)`, tokenHash(h.lp.otherToken), h.lp.f.storeA1, h.session).Scan(&denied)
	})
	if sqlState(err) != "MP403" && sqlState(err) != "MP401" {
		t.Fatalf("foreign token read raw projection: %v", err)
	}
	// Only the runtime is allowed the narrow projection; not the worker or registrar.
	for _, role := range []string{"commerce_media_registrar", "commerce_media_worker", "commerce_media_executor", "commerce_worker"} {
		var grants bool
		err := h.lp.f.owner.QueryRow(context.Background(), `SELECT has_function_privilege($1,'live.read_studio_media(bytea,uuid,uuid)','EXECUTE')`, role).Scan(&grants)
		if err != nil || grants {
			t.Fatalf("projection ACL %s=%v err=%v", role, grants, err)
		}
	}
	var publicGrant bool
	if err := h.lp.f.owner.QueryRow(context.Background(), `SELECT EXISTS (SELECT 1 FROM pg_proc p, aclexplode(p.proacl) a WHERE p.oid='live.read_studio_media(bytea,uuid,uuid)'::regprocedure AND a.grantee=0 AND a.privilege_type='EXECUTE')`).Scan(&publicGrant); err != nil || publicGrant {
		t.Fatalf("PUBLIC projection EXECUTE=%v err=%v", publicGrant, err)
	}
	for _, table := range []string{"live.prepared_media_authorizations", "live.media_authorization_destinations", "live.media_authorization_revocations"} {
		var grants bool
		err := h.lp.f.owner.QueryRow(context.Background(), `SELECT has_table_privilege('commerce_runtime',$1,'SELECT')`, table).Scan(&grants)
		if err != nil || grants {
			t.Fatalf("private SELECT ACL %s=%v err=%v", table, grants, err)
		}
	}
	// Disabled bindings and revocation remove the candidate without leaking details.
	mustExec(t, h.lp.f.owner, `UPDATE integration.bindings SET enabled=false WHERE id=$1`, h.facebook)
	stale, err := studioGet(h.lp, h.lp.token, h.lp.f.storeA1, h.session)
	if err != nil || studioJSON(t, stale)["prepared"] != nil {
		t.Fatalf("stale candidate visible: %+v %v", stale, err)
	}
	mustExec(t, h.lp.f.owner, `UPDATE integration.bindings SET enabled=true WHERE id=$1`, h.facebook)
	mustExec(t, h.lp.f.owner, `INSERT INTO live.media_authorization_revocations(authorization_id,tenant_id,store_id,reason_code,revoked_by) VALUES($1,$2,$3,'test','studio_test')`, h.input.AuthorizationID, h.lp.f.tenantA, h.lp.f.storeA1)
	revoked, err := studioGet(h.lp, h.lp.token, h.lp.f.storeA1, h.session)
	if err != nil || studioJSON(t, revoked)["prepared"] != nil {
		t.Fatalf("revoked candidate visible: %+v %v", revoked, err)
	}
	active := lmpSetup(t, true)
	start, err := active.start(t04Key("studio-attempt"))
	if err != nil {
		t.Fatal(err)
	}
	observed, err := studioGet(active.lp, active.lp.token, active.lp.f.storeA1, active.session)
	if err != nil {
		t.Fatal(err)
	}
	state := studioJSON(t, observed)
	if state["prepared"] != nil {
		t.Fatalf("attempt also shown as prepared: %v", state)
	}
	attempt, ok := state["attempt"].(map[string]any)
	if !ok {
		t.Fatalf("attempt missing: %v", state)
	}
	studioKeys(t, attempt, "attempt_id", "environment", "operation_state", "resource_state", "transport_status", "cleanup_required", "stop_requested", "stop_wire_count", "escalated", "updated_at", "destinations")
	if attempt["attempt_id"] != start.AttemptID || attempt["environment"] != "MOCK" || attempt["resource_state"] != "UNOBSERVED" || attempt["transport_status"] != "" || attempt["cleanup_required"] != false {
		t.Fatalf("unobserved attempt falsely claimed as active/cleaned: %v", attempt)
	}
	seenDestinations, ok := attempt["destinations"].([]any)
	if !ok || len(seenDestinations) != 2 {
		t.Fatalf("attempt destinations: %v", attempt)
	}
	for _, destination := range seenDestinations {
		studioKeys(t, destination.(map[string]any), "ordinal", "provider")
	}
	mustExec(t, active.lp.f.owner, `UPDATE integration.operations SET binding_version=binding_version+1 WHERE id=$1`, start.OperationID)
	if _, err := studioGet(active.lp, active.lp.token, active.lp.f.storeA1, active.session); !errors.Is(err, command.ErrConflict) {
		t.Fatalf("incoherent operation binding was projected: %v", err)
	}
	// Admission and method execution must not keep using the transaction's old
	// timestamp after the token expires.
	expiring := randomToken()
	tx, err := h.lp.f.owner.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := insertSession(context.Background(), tx, expiring, h.lp.actor, "merchant", time.Now().Add(time.Hour), nil); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, call := range []func(pgx.Tx, platform.Scope) error{
		func(tx pgx.Tx, scope platform.Scope) error {
			_, e := live.ListDrafts(context.Background(), tx, scope, expiring, pagination.Request{})
			return e
		},
		func(tx pgx.Tx, scope platform.Scope) error {
			_, e := live.GetStudio(context.Background(), tx, scope, expiring, h.session)
			return e
		},
	} {
		mustExec(t, h.lp.f.owner, `UPDATE identity.sessions SET expires_at=clock_timestamp()+interval '200 milliseconds' WHERE token_hash=$1`, tokenHash(expiring))
		err := platform.WithScope(context.Background(), h.lp.f.runtime, expiring, h.lp.f.storeA1, "store:read", func(tx pgx.Tx, scope platform.Scope) error {
			if _, e := tx.Exec(context.Background(), `SELECT pg_sleep(0.35)`); e != nil {
				return e
			}
			return call(tx, scope)
		})
		if !errors.Is(err, platform.ErrUnauthorized) {
			t.Fatalf("expired after BEGIN accepted: %v", err)
		}
	}
}

func TestStudioBackendSTU02NoMixedDraftAndAttemptSnapshot(t *testing.T) {
	h := lmpSetup(t, false)
	startDone := make(chan error, 1)
	var raced bool
	var seen live.Studio
	err := platform.WithScope(context.Background(), h.lp.f.runtime, h.lp.token, h.lp.f.storeA1, "store:read", func(tx pgx.Tx, scope platform.Scope) error {
		wrapped := &studioBeforeProjectionTx{Tx: tx, before: func() {
			// GetDraft has returned, but read_studio_media has not run. A second
			// connection starts the exact same prepared session at this seam.
			go func() { _, e := h.start(t04Key("studio-interleave")); startDone <- e }()
			select {
			case e := <-startDone:
				if e != nil {
					t.Errorf("interleaved Start: %v", e)
				} else {
					raced = true
				}
			case <-time.After(time.Second):
				// A session lock may correctly defer Start until this read commits.
			}
		}}
		var e error
		seen, e = live.GetStudio(context.Background(), wrapped, scope, h.lp.token, h.session)
		return e
	})
	if !raced {
		select {
		case e := <-startDone:
			if e != nil {
				t.Fatalf("deferred Start failed: %v", e)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("deferred Start did not finish after read transaction")
		}
	}
	if err != nil && !errors.Is(err, live.ErrStudioProjection) && !errors.Is(err, command.ErrConflict) {
		t.Fatalf("interleaved Studio read: %v", err)
	}
	if err == nil && ((seen.Draft.State == "DRAFT" && seen.Attempt != nil) ||
		(seen.Draft.State == "READY" && seen.Prepared != nil) ||
		(seen.Prepared != nil && seen.Prepared.SessionVersion != seen.Draft.Version)) {
		t.Fatalf("mixed draft/media snapshot accepted: %+v", seen)
	}
}

func TestStudioBackendSTU02NoStaleDraftWhenProjectionEmpty(t *testing.T) {
	h := lpSetup(t)
	draft, err := lpCreate(h, h.token, h.f.storeA1, t04Key("studio-empty-create"), lpInput("before edit"))
	if err != nil {
		t.Fatal(err)
	}
	updateDone := make(chan error, 1)
	var raced bool
	var seen live.Studio
	err = platform.WithScope(context.Background(), h.f.runtime, h.token, h.f.storeA1, "store:read", func(tx pgx.Tx, scope platform.Scope) error {
		wrapped := &studioBeforeProjectionTx{Tx: tx, before: func() {
			go func() {
				_, e := lpUpdate(h, h.token, h.f.storeA1, t04Key("studio-empty-edit"), draft.ID, 1, lpInput("after edit"))
				updateDone <- e
			}()
			select {
			case e := <-updateDone:
				if e != nil {
					t.Errorf("interleaved edit: %v", e)
				} else {
					raced = true
				}
			case <-time.After(time.Second):
				// A session lock may serialize the edit after this read.
			}
		}}
		var e error
		seen, e = live.GetStudio(context.Background(), wrapped, scope, h.token, draft.ID)
		return e
	})
	if !raced {
		select {
		case e := <-updateDone:
			if e != nil {
				t.Fatalf("deferred edit: %v", e)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("deferred edit did not finish after read transaction")
		}
	}
	if err != nil && !errors.Is(err, live.ErrStudioProjection) && !errors.Is(err, command.ErrConflict) {
		t.Fatalf("interleaved empty projection: %v", err)
	}
	if raced && err == nil && (seen.Draft.Version != 2 || seen.Draft.Title != "after edit") {
		t.Fatalf("stale draft with empty projection accepted after committed edit: %+v", seen)
	}
	current, err := lpGet(h, h.token, h.f.storeA1, draft.ID)
	if err != nil || current.Version != 2 || current.Title != "after edit" {
		t.Fatalf("interleaved edit was not durable: %+v %v", current, err)
	}
}
