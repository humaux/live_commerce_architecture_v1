package foundation_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/command"
	"livecommerce/internal/live"
	"livecommerce/internal/platform"
)

// Each LSP case owns its principals and tokens. The shared PG fixture's
// integration permissions never confer live permission.
type lpHarness struct {
	f                                          *testFixture
	actor, peer, other, limited                string
	token, peerToken, otherToken, limitedToken string
}

func lpSetup(t *testing.T) lpHarness {
	t.Helper()
	f := fixture(t)
	h := lpHarness{f: f, actor: randomUUID(), peer: randomUUID(), other: randomUUID(), limited: randomUUID(),
		token: randomToken(), peerToken: randomToken(), otherToken: randomToken(), limitedToken: randomToken()}
	ctx := context.Background()
	tx, err := f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `INSERT INTO identity.principals(id) VALUES($1),($2),($3),($4)`, h.actor, h.peer, h.other, h.limited); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO identity.memberships(tenant_id,principal_id) VALUES($1,$2),($1,$3),($4,$5),($1,$6)`, f.tenantA, h.actor, h.peer, f.tenantB, h.other, h.limited); err != nil {
		t.Fatal(err)
	}
	var autoGrant int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM identity.store_grants WHERE principal_id=$1 AND permission LIKE 'live:%'`, h.actor).Scan(&autoGrant); err != nil || autoGrant != 0 {
		t.Fatalf("migration granted live access automatically: count=%d err=%v", autoGrant, err)
	}
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM identity.store_grants WHERE principal_id=$1 AND permission LIKE 'live:%'`, f.principalA).Scan(&autoGrant); err != nil || autoGrant != 0 {
		t.Fatalf("migration backfilled existing merchant live grants: count=%d err=%v", autoGrant, err)
	}
	for _, grant := range []struct {
		tenant, store, principal string
		permissions              []string
	}{
		{f.tenantA, f.storeA1, h.actor, []string{"store:read", "live:read", "live:manage"}},
		{f.tenantA, f.storeA2, h.actor, []string{"store:read", "live:read", "live:manage"}},
		{f.tenantA, f.storeA1, h.peer, []string{"store:read", "live:read", "live:manage"}},
		{f.tenantB, f.storeB, h.other, []string{"store:read", "live:read", "live:manage"}},
		{f.tenantA, f.storeA1, h.limited, []string{"store:read"}},
	} {
		if _, err = tx.Exec(ctx, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) SELECT $1,$2,$3,p FROM unnest($4::text[]) AS p`, grant.tenant, grant.store, grant.principal, grant.permissions); err != nil {
			t.Fatal(err)
		}
	}
	for _, session := range []struct{ token, principal string }{{h.token, h.actor}, {h.peerToken, h.peer}, {h.otherToken, h.other}, {h.limitedToken, h.limited}} {
		if err = insertSession(ctx, tx, session.token, session.principal, "merchant", time.Now().Add(time.Hour), nil); err != nil {
			t.Fatal(err)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		// The runner drops its task-owned database too; local cleanup keeps this
		// package's shared fixture usable by subsequent focused cases.
		for _, q := range []string{
			`DELETE FROM live.programs WHERE principal_id IN ($1,$2,$3,$4)`,
			`DELETE FROM live.sessions WHERE principal_id IN ($1,$2,$3,$4)`,
			`DELETE FROM ops.command_results WHERE principal_id IN ($1,$2,$3,$4) AND operation LIKE 'live.draft.%'`,
			`DELETE FROM ops.audit_events WHERE principal_id IN ($1,$2,$3,$4) AND action LIKE 'live.draft.%'`,
			`DELETE FROM identity.sessions WHERE principal_id IN ($1,$2,$3,$4)`,
			`DELETE FROM identity.store_grants WHERE principal_id IN ($1,$2,$3,$4)`,
			`DELETE FROM identity.memberships WHERE principal_id IN ($1,$2,$3,$4)`,
			`DELETE FROM identity.principals WHERE id IN ($1,$2,$3,$4)`,
		} {
			if _, e := f.owner.Exec(context.Background(), q, h.actor, h.peer, h.other, h.limited); e != nil {
				t.Errorf("live fixture cleanup: %v", e)
			}
		}
	})
	return h
}

func lpCreate(h lpHarness, token, store, key string, in live.DraftInput) (live.Draft, error) {
	ctx := context.Background()
	return t04Scoped(ctx, h.f, token, store, "store:read", func(tx pgx.Tx, s platform.Scope) (live.Draft, error) {
		return live.CreateDraft(ctx, tx, s, token, key, in)
	})
}
func lpUpdate(h lpHarness, token, store, key, id string, version int64, in live.DraftInput) (live.Draft, error) {
	ctx := context.Background()
	return t04Scoped(ctx, h.f, token, store, "store:read", func(tx pgx.Tx, s platform.Scope) (live.Draft, error) {
		return live.UpdateDraft(ctx, tx, s, token, key, id, version, in)
	})
}
func lpGet(h lpHarness, token, store, id string) (live.Draft, error) {
	ctx := context.Background()
	return t04Scoped(ctx, h.f, token, store, "store:read", func(tx pgx.Tx, s platform.Scope) (live.Draft, error) { return live.GetDraft(ctx, tx, s, token, id) })
}

func lpInput(title string) live.DraftInput { return live.DraftInput{Title: title, AspectRatio: "16:9"} }

// Principal-qualified counts avoid coupling to any other package fixture.
// The River count catches accidental scheduling by this draft-only slice.
func lpFacts(t *testing.T, h lpHarness) (got [5]int64) {
	t.Helper()
	err := h.f.owner.QueryRow(context.Background(), `SELECT
	 (SELECT count(*) FROM live.sessions WHERE principal_id IN ($1,$2,$3,$4)),
	 (SELECT count(*) FROM live.programs WHERE principal_id IN ($1,$2,$3,$4)),
	 (SELECT count(*) FROM ops.command_results WHERE principal_id IN ($1,$2,$3,$4) AND operation LIKE 'live.draft.%'),
	 (SELECT count(*) FROM ops.audit_events WHERE principal_id IN ($1,$2,$3,$4) AND action LIKE 'live.draft.%'),
	 (SELECT count(*) FROM river.river_job)`, h.actor, h.peer, h.other, h.limited).Scan(&got[0], &got[1], &got[2], &got[3], &got[4])
	if err != nil {
		t.Fatal(err)
	}
	return
}

type lpStored struct {
	Title, AspectRatio, State string
	Version                   int64
	UpdatedAt                 time.Time
}

func lpStoredRow(t *testing.T, h lpHarness, id string) (row lpStored) {
	t.Helper()
	err := h.f.owner.QueryRow(context.Background(), `SELECT s.title,p.aspect_ratio,p.state,s.version,s.updated_at
	FROM live.sessions s JOIN live.programs p ON (p.tenant_id,p.store_id,p.session_id)=(s.tenant_id,s.store_id,s.id)
	WHERE s.id=$1`, id).Scan(&row.Title, &row.AspectRatio, &row.State, &row.Version, &row.UpdatedAt)
	if err != nil {
		t.Fatal(err)
	}
	return
}

func lpDelta(t *testing.T, before, after [5]int64, want [5]int64) {
	t.Helper()
	var got [5]int64
	for i := range got {
		got[i] = after[i] - before[i]
	}
	if got != want {
		t.Fatalf("live session/program/receipt/audit/job delta=%v want %v", got, want)
	}
}

func lpDraft(t *testing.T, d live.Draft, version int64, in live.DraftInput) {
	t.Helper()
	if !command.ValidID(d.ID) || !command.ValidID(d.ProgramID) || d.ID == d.ProgramID || d.Title != in.Title || d.AspectRatio != in.AspectRatio || d.State != "DRAFT" || d.Version != version || d.CreatedAt.IsZero() || d.UpdatedAt.IsZero() {
		t.Fatalf("invalid draft DTO: %+v", d)
	}
	if in.ScheduledAt == nil {
		if d.ScheduledAt != nil {
			t.Fatalf("unexpected schedule: %v", d.ScheduledAt)
		}
	} else if d.ScheduledAt == nil || !d.ScheduledAt.Equal(in.ScheduledAt.UTC().Truncate(time.Microsecond)) {
		t.Fatalf("schedule=%v want %v", d.ScheduledAt, in.ScheduledAt)
	}
}

func TestLivePlanningLSP01AtomicPairAndSchedule(t *testing.T) {
	h := lpSetup(t)
	before := lpFacts(t, h)
	schedule := time.Date(2035, 4, 5, 6, 7, 8, 123456789, time.FixedZone("offset", 9*3600))
	in := live.DraftInput{Title: "上新直播", ScheduledAt: &schedule, AspectRatio: "9:16"}
	first, err := lpCreate(h, h.token, h.f.storeA1, t04Key("live-create"), in)
	if err != nil {
		t.Fatal(err)
	}
	lpDraft(t, first, 1, in)
	lpDelta(t, before, lpFacts(t, h), [5]int64{1, 1, 1, 1, 0})
	got, err := lpGet(h, h.token, h.f.storeA1, first.ID)
	if err != nil || !reflect.DeepEqual(got, first) {
		t.Fatalf("joined Get mismatch got=%+v first=%+v err=%v", got, first, err)
	}
	lpDelta(t, before, lpFacts(t, h), [5]int64{1, 1, 1, 1, 0})
	var parentID, creator, title, ratio, state string
	var storedSchedule time.Time
	err = h.f.owner.QueryRow(context.Background(), `SELECT p.session_id,s.principal_id,s.title,s.scheduled_at,p.aspect_ratio,p.state FROM live.sessions s JOIN live.programs p ON (p.tenant_id,p.store_id,p.session_id)=(s.tenant_id,s.store_id,s.id) WHERE s.id=$1`, first.ID).Scan(&parentID, &creator, &title, &storedSchedule, &ratio, &state)
	if err != nil || parentID != first.ID || creator != h.actor || title != in.Title || ratio != in.AspectRatio || state != "DRAFT" || !storedSchedule.Equal(schedule.UTC().Truncate(time.Microsecond)) {
		t.Fatalf("durable pair mismatch: %s %s %s %s %s %v %v", parentID, creator, title, ratio, state, storedSchedule, err)
	}
	changed := live.DraftInput{Title: "秋季直播", AspectRatio: "16:9"}
	second, err := lpUpdate(h, h.token, h.f.storeA1, t04Key("live-update"), first.ID, 1, changed)
	if err != nil {
		t.Fatal(err)
	}
	lpDraft(t, second, 2, changed)
	if second.ID != first.ID || second.ProgramID != first.ProgramID || !second.CreatedAt.Equal(first.CreatedAt) || !second.UpdatedAt.After(first.UpdatedAt) {
		t.Fatalf("identity or timestamps changed: first=%+v second=%+v", first, second)
	}
	lpDelta(t, before, lpFacts(t, h), [5]int64{1, 1, 2, 2, 0})
	storedBeforeRollback := lpStoredRow(t, h, first.ID)
	// A successful method call is still caller-owned until WithScope commits.
	rollbackKey := t04Key("live-rollback")
	err = platform.WithScope(context.Background(), h.f.runtime, h.token, h.f.storeA1, "store:read", func(tx pgx.Tx, s platform.Scope) error {
		if _, e := live.CreateDraft(context.Background(), tx, s, h.token, rollbackKey, lpInput("rollback")); e != nil {
			return e
		}
		return errors.New("force caller rollback")
	})
	if err == nil || err.Error() != "force caller rollback" {
		t.Fatalf("rollback error=%v", err)
	}
	lpDelta(t, before, lpFacts(t, h), [5]int64{1, 1, 2, 2, 0})
	err = platform.WithScope(context.Background(), h.f.runtime, h.token, h.f.storeA1, "store:read", func(tx pgx.Tx, s platform.Scope) error {
		if _, e := live.UpdateDraft(context.Background(), tx, s, h.token, t04Key("live-update-rollback"), first.ID, 2, lpInput("rolled back edit")); e != nil {
			return e
		}
		return errors.New("force update rollback")
	})
	if err == nil || err.Error() != "force update rollback" {
		t.Fatalf("update rollback error=%v", err)
	}
	lpDelta(t, before, lpFacts(t, h), [5]int64{1, 1, 2, 2, 0})
	if got := lpStoredRow(t, h, first.ID); got != storedBeforeRollback {
		t.Fatalf("update rollback mutated durable pair: %+v -> %+v", storedBeforeRollback, got)
	}
}

func TestLivePlanningLSP02ReplayAndCAS(t *testing.T) {
	h := lpSetup(t)
	in := lpInput("one deliberate draft")
	key := t04Key("live-concurrent")
	before := lpFacts(t, h)
	const contenders = 8
	results := make([]live.Draft, contenders)
	errs := make([]error, contenders)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			results[i], errs[i] = lpCreate(h, h.token, h.f.storeA1, key, in)
		}(i)
	}
	close(start)
	wg.Wait()
	for i, e := range errs {
		if e != nil || !reflect.DeepEqual(results[i], results[0]) {
			t.Fatalf("same-key concurrent replay[%d]=%+v err=%v first=%+v", i, results[i], e, results[0])
		}
	}
	lpDelta(t, before, lpFacts(t, h), [5]int64{1, 1, 1, 1, 0})
	if _, e := lpCreate(h, h.token, h.f.storeA1, key, lpInput("different body")); !errors.Is(e, command.ErrConflict) {
		t.Fatalf("changed body replay: %v", e)
	}
	if _, e := lpCreate(h, h.peerToken, h.f.storeA1, key, in); !errors.Is(e, command.ErrConflict) {
		t.Fatalf("different principal replay: %v", e)
	}
	lpDelta(t, before, lpFacts(t, h), [5]int64{1, 1, 1, 1, 0})
	// A separate key is a deliberate second draft; title is not unique.
	if _, e := lpCreate(h, h.token, h.f.storeA1, t04Key("live-second"), in); e != nil {
		t.Fatal(e)
	}
	lpDelta(t, before, lpFacts(t, h), [5]int64{2, 2, 2, 2, 0})
	updated := lpInput("first edit")
	updateKey := t04Key("live-edit")
	firstEdit, e := lpUpdate(h, h.token, h.f.storeA1, updateKey, results[0].ID, 1, updated)
	if e != nil {
		t.Fatal(e)
	}
	newer, e := lpUpdate(h, h.token, h.f.storeA1, t04Key("live-newer"), results[0].ID, 2, lpInput("later edit"))
	if e != nil || newer.Version != 3 {
		t.Fatalf("newer edit: %+v %v", newer, e)
	}
	replay, e := lpUpdate(h, h.token, h.f.storeA1, updateKey, results[0].ID, 1, updated)
	if e != nil || !reflect.DeepEqual(replay, firstEdit) {
		t.Fatalf("frozen update receipt changed after newer edit: %+v %v", replay, e)
	}
	lpDelta(t, before, lpFacts(t, h), [5]int64{2, 2, 4, 4, 0})
	updated2 := lpInput("CAS contender")
	cas := make([]error, 2)
	start = make(chan struct{})
	for i := range cas {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, cas[i] = lpUpdate(h, h.token, h.f.storeA1, t04Key(fmt.Sprintf("live-cas-%d", i)), results[0].ID, 3, updated2)
		}(i)
	}
	close(start)
	wg.Wait()
	success, conflicts := 0, 0
	for _, e := range cas {
		if e == nil {
			success++
		} else if errors.Is(e, command.ErrConflict) {
			conflicts++
		} else {
			t.Fatalf("CAS unexpected: %v", e)
		}
	}
	if success != 1 || conflicts != 1 {
		t.Fatalf("CAS outcomes success=%d conflicts=%d", success, conflicts)
	}
	lpDelta(t, before, lpFacts(t, h), [5]int64{2, 2, 5, 5, 0})
}

func TestLivePlanningLSP03AuthorityIsolationAndReadOnly(t *testing.T) {
	h := lpSetup(t)
	before := lpFacts(t, h)
	in := lpInput("private session")
	first, e := lpCreate(h, h.token, h.f.storeA1, t04Key("live-private"), in)
	if e != nil {
		t.Fatal(e)
	}
	baseline := lpFacts(t, h)
	for _, tc := range []struct {
		name, token, store string
		want               error
	}{
		{"missing permission", h.limitedToken, h.f.storeA1, platform.ErrForbidden},
		{"other store", h.token, h.f.storeA2, command.ErrNotFound},
		{"other tenant", h.otherToken, h.f.storeB, command.ErrNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, e := lpGet(h, tc.token, tc.store, first.ID); !errors.Is(e, tc.want) {
				t.Fatalf("Get error=%v want %v", e, tc.want)
			}
			if _, e := lpUpdate(h, tc.token, tc.store, t04Key("live-deny"), first.ID, 1, in); !errors.Is(e, tc.want) {
				t.Fatalf("Update error=%v want %v", e, tc.want)
			}
		})
	}
	// A read grant does not imply management; a management grant does not imply
	// read. Both permissions must be provisioned explicitly.
	mustExec(t, h.f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,'live:read')`, h.f.tenantA, h.f.storeA1, h.limited)
	if _, e := lpGet(h, h.limitedToken, h.f.storeA1, first.ID); e != nil {
		t.Fatalf("read-only positive control: %v", e)
	}
	if _, e := lpCreate(h, h.limitedToken, h.f.storeA1, t04Key("live-read-only"), in); !errors.Is(e, platform.ErrForbidden) {
		t.Fatalf("read-only create: %v", e)
	}
	if _, e := lpUpdate(h, h.limitedToken, h.f.storeA1, t04Key("live-read-only-update"), first.ID, 1, in); !errors.Is(e, platform.ErrForbidden) {
		t.Fatalf("read-only update: %v", e)
	}
	mustExec(t, h.f.owner, `DELETE FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND permission='live:read'`, h.f.tenantA, h.f.storeA1, h.limited)
	mustExec(t, h.f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,'live:manage')`, h.f.tenantA, h.f.storeA1, h.limited)
	if _, e := lpGet(h, h.limitedToken, h.f.storeA1, first.ID); !errors.Is(e, platform.ErrForbidden) {
		t.Fatalf("manage-only read: %v", e)
	}
	rollback := errors.New("management positive control rollback")
	err := platform.WithScope(context.Background(), h.f.runtime, h.limitedToken, h.f.storeA1, "store:read", func(tx pgx.Tx, s platform.Scope) error {
		if _, e := live.CreateDraft(context.Background(), tx, s, h.limitedToken, t04Key("live-manage-only"), in); e != nil {
			return e
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("manage-only positive control: %v", err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(string)
	}{
		{"revoked", func(token string) {
			mustExec(t, h.f.owner, `UPDATE identity.sessions SET revoked_at=clock_timestamp() WHERE token_hash=$1`, tokenHash(token))
		}},
		{"expired", func(token string) {
			mustExec(t, h.f.owner, `UPDATE identity.sessions SET expires_at=clock_timestamp()-interval '1 second' WHERE token_hash=$1`, tokenHash(token))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			token := randomToken()
			tx, err := h.f.owner.Begin(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if err = insertSession(context.Background(), tx, token, h.actor, "merchant", time.Now().Add(time.Hour), nil); err != nil {
				t.Fatal(err)
			}
			if err = tx.Commit(context.Background()); err != nil {
				t.Fatal(err)
			}
			tc.mutate(token)
			if _, e := lpCreate(h, token, h.f.storeA1, t04Key("live-denied"), in); !errors.Is(e, platform.ErrUnauthorized) {
				t.Fatalf("%s create: %v", tc.name, e)
			}
			if _, e := lpGet(h, token, h.f.storeA1, first.ID); !errors.Is(e, platform.ErrUnauthorized) {
				t.Fatalf("%s read: %v", tc.name, e)
			}
		})
	}
	// Revision and GUC must still be checked when the outer scope was valid.
	err = platform.WithScope(context.Background(), h.f.runtime, h.token, h.f.storeA1, "store:read", func(tx pgx.Tx, s platform.Scope) error {
		mustExec(t, h.f.owner, `UPDATE identity.memberships SET authz_revision=authz_revision+1 WHERE tenant_id=$1 AND principal_id=$2`, h.f.tenantA, h.actor)
		_, err := live.UpdateDraft(context.Background(), tx, s, h.token, t04Key("live-stale-revision"), first.ID, 1, in)
		return err
	})
	if !errors.Is(err, platform.ErrForbidden) {
		t.Fatalf("stale revision accepted: %v", err)
	}
	err = platform.WithScope(context.Background(), h.f.runtime, h.token, h.f.storeA1, "store:read", func(tx pgx.Tx, s platform.Scope) error {
		if _, err := tx.Exec(context.Background(), `SELECT set_config('app.store_id',$1,true)`, h.f.storeA2); err != nil {
			return err
		}
		_, err := live.GetDraft(context.Background(), tx, s, h.token, first.ID)
		return err
	})
	if err == nil {
		t.Fatal("forged GUC accepted")
	}
	tx, e := h.f.runtime.BeginTx(context.Background(), pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(context.Background())
	if _, e = tx.Exec(context.Background(), `SELECT set_config('app.tenant_id',$1,true),set_config('app.store_id',$2,true),set_config('app.principal_id',$3,true)`, h.f.tenantA, h.f.storeA1, h.actor); e != nil {
		t.Fatal(e)
	}
	_, e = live.CreateDraft(context.Background(), tx, platform.Scope{TenantID: h.f.tenantA, StoreID: h.f.storeA1, PrincipalID: h.actor, Revision: 2}, h.token, t04Key("live-isolation"), in)
	if !errors.Is(e, command.ErrInvalid) {
		t.Fatalf("repeatable-read transaction accepted with exact scope GUCs: %v", e)
	}
	if _, e = lpGet(h, h.token, h.f.storeA1, first.ID); e != nil {
		t.Fatalf("valid read after revision change: %v", e)
	}
	lpDelta(t, before, lpFacts(t, h), [5]int64{1, 1, 1, 1, 0})
	if lpFacts(t, h) != baseline {
		t.Fatal("Get/denials changed draft, receipt, audit or jobs")
	}
}

func TestLivePlanningLSP04ObservedWaitExpiry(t *testing.T) {
	for _, mode := range []string{"row-update", "advisory-replay"} {
		t.Run(mode, func(t *testing.T) {
			h := lpSetup(t)
			first, e := lpCreate(h, h.token, h.f.storeA1, t04Key("live-wait-create"), lpInput("before wait"))
			if e != nil {
				t.Fatal(e)
			}
			key := t04Key("live-wait-edit")
			in := lpInput("after wait")
			if mode == "advisory-replay" {
				if _, e = lpUpdate(h, h.token, h.f.storeA1, key, first.ID, 1, in); e != nil {
					t.Fatal(e)
				}
			}
			before := lpFacts(t, h)
			beforeRow := lpStoredRow(t, h, first.ID)
			var expires time.Time
			if e = h.f.owner.QueryRow(context.Background(), `UPDATE identity.sessions SET expires_at=clock_timestamp()+interval '2 seconds' WHERE token_hash=$1 RETURNING expires_at`, tokenHash(h.token)).Scan(&expires); e != nil {
				t.Fatal(e)
			}
			holder, e := h.f.owner.Begin(context.Background())
			if e != nil {
				t.Fatal(e)
			}
			defer holder.Rollback(context.Background())
			if mode == "row-update" {
				if _, e = holder.Exec(context.Background(), `UPDATE live.sessions SET title=title WHERE id=$1`, first.ID); e != nil {
					t.Fatal(e)
				}
			} else {
				lockKey := "command|" + h.f.tenantA + "|" + h.f.storeA1 + "|live.draft.update|" + key
				if _, e = holder.Exec(context.Background(), `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, lockKey); e != nil {
					t.Fatal(e)
				}
			}
			pids := make(chan int, 1)
			done := make(chan error, 1)
			go func() {
				done <- platform.WithScope(context.Background(), h.f.runtime, h.token, h.f.storeA1, "store:read", func(tx pgx.Tx, s platform.Scope) error {
					if _, err := tx.Exec(context.Background(), `SET LOCAL lock_timeout='3s'`); err != nil {
						return err
					}
					var pid int
					if err := tx.QueryRow(context.Background(), `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
						return err
					}
					pids <- pid
					version := int64(1)
					if mode == "advisory-replay" {
						version = 1
					}
					_, err := live.UpdateDraft(context.Background(), tx, s, h.token, key, first.ID, version, in)
					return err
				})
			}()
			var pid int
			select {
			case pid = <-pids:
			case e = <-done:
				t.Fatalf("operation ended before wait: %v", e)
			case <-time.After(time.Second):
				t.Fatal("no backend pid")
			}
			lpWaitLock(t, h.f, pid, mode == "advisory-replay")
			var valid bool
			if e = h.f.owner.QueryRow(context.Background(), `SELECT clock_timestamp()<$1`, expires).Scan(&valid); e != nil || !valid {
				t.Fatalf("expiry window lost before observed wait: valid=%t err=%v", valid, e)
			}
			mustExec(t, h.f.owner, `SELECT pg_sleep(GREATEST(0,extract(epoch FROM $1::timestamptz-clock_timestamp()))+0.02)`, expires)
			if e = holder.Commit(context.Background()); e != nil {
				t.Fatal(e)
			}
			if e = waitError(t, done); !errors.Is(e, platform.ErrUnauthorized) {
				t.Fatalf("expired token accepted after observed %s wait: %v", mode, e)
			}
			if after := lpFacts(t, h); after != before {
				t.Fatalf("expired %s changed facts: %v -> %v", mode, before, after)
			}
			if afterRow := lpStoredRow(t, h, first.ID); afterRow != beforeRow {
				t.Fatalf("expired %s mutated row: %+v -> %+v", mode, beforeRow, afterRow)
			}
		})
	}
}

func lpWaitLock(t *testing.T, f *testFixture, pid int, advisory bool) {
	t.Helper()
	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) {
		var waitType, waitEvent string
		err := f.owner.QueryRow(context.Background(), `SELECT coalesce(wait_event_type,''),coalesce(wait_event,'') FROM pg_stat_activity WHERE pid=$1`, pid).Scan(&waitType, &waitEvent)
		if err != nil {
			t.Fatal(err)
		}
		if waitType == "Lock" && (advisory && waitEvent == "advisory" || !advisory && waitEvent != "advisory") {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("PG backend %d had no observed %s lock wait", pid, map[bool]string{true: "advisory", false: "row"}[advisory])
}

func TestLivePlanningLSP05BoundariesConstraintsAndACL(t *testing.T) {
	h := lpSetup(t)
	before := lpFacts(t, h)
	long := make([]rune, 201)
	for i := range long {
		long[i] = '界'
	}
	tooOld := time.Date(1999, 12, 31, 23, 59, 59, 0, time.UTC)
	tooNew := time.Date(2200, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name string
		in   live.DraftInput
	}{
		{"empty", lpInput("")}, {"untrimmed", lpInput(" title ")}, {"control", lpInput("a\nb")},
		{"invalid utf8", lpInput(string([]byte{0xff}))}, {"too long", lpInput(string(long))},
		{"bad ratio", live.DraftInput{Title: "valid", AspectRatio: "1:1"}},
		{"before 2000", live.DraftInput{Title: "valid", AspectRatio: "16:9", ScheduledAt: &tooOld}},
		{"year 2200", live.DraftInput{Title: "valid", AspectRatio: "16:9", ScheduledAt: &tooNew}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, e := lpCreate(h, h.token, h.f.storeA1, t04Key("live-invalid"), tc.in); !errors.Is(e, command.ErrInvalid) {
				t.Fatalf("invalid input: %v", e)
			}
		})
	}
	lpDelta(t, before, lpFacts(t, h), [5]int64{})
	past := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	valid := live.DraftInput{Title: string(long[:200]), ScheduledAt: &past, AspectRatio: "9:16"}
	d, e := lpCreate(h, h.token, h.f.storeA1, t04Key("live-boundary"), valid)
	if e != nil {
		t.Fatal(e)
	}
	lpDraft(t, d, 1, valid)
	for _, id := range []string{"", "not-a-uuid", "ABCDEF00-0000-0000-0000-000000000000"} {
		if _, e := lpGet(h, h.token, h.f.storeA1, id); !errors.Is(e, command.ErrInvalid) {
			t.Fatalf("invalid Get id %q: %v", id, e)
		}
		if _, e := lpUpdate(h, h.token, h.f.storeA1, t04Key("live-bad-id"), id, 1, lpInput("x")); !errors.Is(e, command.ErrInvalid) {
			t.Fatalf("invalid Update id %q: %v", id, e)
		}
	}
	if _, e := lpUpdate(h, h.token, h.f.storeA1, t04Key("live-version"), d.ID, 0, lpInput("x")); !errors.Is(e, command.ErrInvalid) {
		t.Fatalf("invalid version: %v", e)
	}
	if _, e := lpGet(h, h.token, h.f.storeA1, randomUUID()); !errors.Is(e, command.ErrNotFound) {
		t.Fatalf("missing draft: %v", e)
	}
	// The runtime cannot alter ownership, identity, creation time or state, or delete.
	for _, q := range []string{
		`UPDATE live.sessions SET id=gen_random_uuid() WHERE id=$1`,
		`UPDATE live.sessions SET principal_id=gen_random_uuid() WHERE id=$1`,
		`UPDATE live.sessions SET created_at=clock_timestamp() WHERE id=$1`,
		`DELETE FROM live.sessions WHERE id=$1`,
		`UPDATE live.programs SET state='LIVE' WHERE session_id=$1`,
		`DELETE FROM live.programs WHERE session_id=$1`,
	} {
		err := platform.WithScope(context.Background(), h.f.runtime, h.token, h.f.storeA1, "store:read", func(tx pgx.Tx, s platform.Scope) error { _, e := tx.Exec(context.Background(), q, d.ID); return e })
		if sqlState(err) != "42501" {
			t.Fatalf("runtime mutation %q state=%s err=%v", q, sqlState(err), err)
		}
	}
	// Owner-side constraints remain effective independently of Go validation.
	for _, q := range []string{
		`UPDATE live.sessions SET title=' bad ' WHERE id=$1`,
		`UPDATE live.sessions SET version=0 WHERE id=$1`,
		`UPDATE live.programs SET aspect_ratio='1:1' WHERE session_id=$1`,
		`UPDATE live.programs SET state='LIVE' WHERE session_id=$1`,
	} {
		_, err := h.f.owner.Exec(context.Background(), q, d.ID)
		if sqlState(err) != "23514" {
			t.Fatalf("SQL constraint %q state=%s err=%v", q, sqlState(err), err)
		}
	}
	var forcedSessions, forcedPrograms, publicSchema, publicSessions, publicPrograms, buyerSessions, workerSessions bool
	err := h.f.owner.QueryRow(context.Background(), `SELECT
	 (SELECT relforcerowsecurity FROM pg_class WHERE oid='live.sessions'::regclass),
	 (SELECT relforcerowsecurity FROM pg_class WHERE oid='live.programs'::regclass),
	 EXISTS(SELECT 1 FROM pg_namespace n CROSS JOIN LATERAL aclexplode(coalesce(n.nspacl,acldefault('n',n.nspowner))) a WHERE n.oid='live'::regnamespace AND a.grantee=0 AND a.privilege_type='USAGE'),
	 EXISTS(SELECT 1 FROM pg_class c CROSS JOIN LATERAL aclexplode(coalesce(c.relacl,acldefault('r',c.relowner))) a WHERE c.oid='live.sessions'::regclass AND a.grantee=0),
	 EXISTS(SELECT 1 FROM pg_class c CROSS JOIN LATERAL aclexplode(coalesce(c.relacl,acldefault('r',c.relowner))) a WHERE c.oid='live.programs'::regclass AND a.grantee=0),
	 has_table_privilege('commerce_buyer_runtime','live.sessions','SELECT'),
	 has_table_privilege('commerce_worker','live.sessions','SELECT')`).Scan(&forcedSessions, &forcedPrograms, &publicSchema, &publicSessions, &publicPrograms, &buyerSessions, &workerSessions)
	if err != nil || !forcedSessions || !forcedPrograms || publicSchema || publicSessions || publicPrograms || buyerSessions || workerSessions {
		t.Fatalf("live ACL boundary forced=%t/%t public=%t/%t/%t buyer=%t worker=%t err=%v", forcedSessions, forcedPrograms, publicSchema, publicSessions, publicPrograms, buyerSessions, workerSessions, err)
	}
	lpDelta(t, before, lpFacts(t, h), [5]int64{1, 1, 1, 1, 0})
}
