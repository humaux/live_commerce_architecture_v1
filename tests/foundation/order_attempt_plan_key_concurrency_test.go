// Purpose: REAL_PG tests FIRST for concurrent cross-tenant C1 use of one A2 key.
// Depends on: actual Service.Admit, native scope locks and dedicated key-admin login.
// Used by: OA1 SQL review CI; no provider, fake admission or local execution.
// Invariants: unrelated admissions share the key prefix; retirement stays exclusive
// and cannot retire a version referenced by either committed pending attempt.
package foundation_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/orderattempt"
)

func oaKeyConcurrentPool(t *testing.T, f *testFixture, app string) *pgxpool.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(miRole(t, f, "commerce_runtime"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.MaxConns = 1
	cfg.ConnConfig.RuntimeParams["application_name"] = app
	p, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	var actual bool
	if err = p.QueryRow(context.Background(), `SELECT pg_has_role(session_user,'commerce_runtime','MEMBER')`).Scan(&actual); err != nil || !actual {
		t.Fatal("actual runtime key-prefix authority", err)
	}
	return p
}

func oaKeyConcurrentBlocker(t *testing.T, f *testFixture, ctx context.Context, app string, allowed ...uint32) uint32 {
	t.Helper()
	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()
	for {
		var pid uint32
		var blockers []int32
		err := f.owner.QueryRow(ctx, `SELECT pid,pg_blocking_pids(pid) FROM pg_stat_activity
		 WHERE application_name=$1 AND wait_event_type='Lock'`, app).Scan(&pid, &blockers)
		if err == nil {
			for _, expected := range allowed {
				for _, blocker := range blockers {
					if uint32(blocker) == expected {
						return pid
					}
				}
			}
			t.Fatalf("unrelated C1 serialized on shared key instead of its own scope: pid=%d blockers=%v", pid, blockers)
		}
		if err != pgx.ErrNoRows {
			t.Fatal("native key-prefix blocking sentinel", err)
		}
		select {
		case <-ctx.Done():
			t.Fatal("native key-prefix blocking sentinel absent before deadline")
		case <-tick.C:
		}
	}
}

func TestOrderAttemptPlanKeyCrossTenantSharedAdmission(t *testing.T) {
	e1, e2 := lbuNew(t), lbuNew(t)
	if e1.tenant() == e2.tenant() || e1.store() == e2.store() {
		t.Fatal("cross-tenant native fixture isolation absent")
	}
	version := "a2-shared-" + randomUUID()
	// Register the deployed opaque key before the race: concurrent creation of
	// a previously unknown registry row is a separate existing A2 test.
	mustExec(t, e1.p.f.owner, `INSERT INTO ops.order_attempt_plan_key_versions(key_version) VALUES($1)`, version)
	app1, app2, adminApp := "a2-share-"+randomUUID(), "a2-share-"+randomUUID(), "a2-retire-"+randomUUID()
	p1, p2 := oaKeyConcurrentPool(t, e1.p.f, app1), oaKeyConcurrentPool(t, e2.p.f, app2)
	s1, err := orderattempt.NewService(p1, oaKeyRing(t, version))
	if err != nil {
		t.Fatal(err)
	}
	s2, err := orderattempt.NewService(p2, oaKeyRing(t, version))
	if err != nil {
		t.Fatal(err)
	}
	r1, r2 := oaServiceRequest(t, e1), oaServiceRequest(t, e2)
	admin := oaKeyAdmin(t, e1.p.f)
	adminConn, err := admin.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer adminConn.Release()
	if _, err = adminConn.Exec(context.Background(), `SELECT set_config('application_name',$1,false)`, adminApp); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	var holders [2]pgx.Tx
	type admissionOutcome struct {
		receipt orderattempt.AdmissionReceipt
		err     error
	}
	type retirementOutcome struct {
		state string
		err   error
	}
	done1, done2 := make(chan admissionOutcome, 1), make(chan admissionOutcome, 1)
	retired := make(chan retirementOutcome, 1)
	started, received := [3]bool{}, [3]bool{}
	defer func() {
		cancel()
		clean, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		for _, holder := range holders {
			if holder != nil {
				_ = holder.Rollback(clean)
			}
		}
		for i, ch := range []chan admissionOutcome{done1, done2} {
			if started[i] && !received[i] {
				select {
				case <-ch:
				case <-clean.Done():
					t.Error("bounded cross-tenant admission cleanup")
				}
			}
		}
		if started[2] && !received[2] {
			select {
			case <-retired:
			case <-clean.Done():
				t.Error("bounded concurrent retirement cleanup")
			}
		}
	}()
	for i, scope := range []string{r1.ExpectedScopeID, r2.ExpectedScopeID} {
		holders[i], err = e1.p.f.owner.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		oaStateExec(t, holders[i], `SELECT scope_id FROM ops.order_attempt_scopes WHERE scope_id=$1 FOR UPDATE`, scope)
	}
	// All three positive barriers must be observed inside C1's existing 1s
	// lock budget. There is no sleep or widened product timeout.
	barrierCtx, stopBarrier := context.WithTimeout(ctx, 750*time.Millisecond)
	defer stopBarrier()
	started[0] = true
	go func() {
		a, callErr := s1.Admit(ctx, e1.token(), e1.store(), r1)
		done1 <- admissionOutcome{a, callErr}
	}()
	pid1 := oaKeyConcurrentBlocker(t, e1.p.f, barrierCtx, app1, holders[0].Conn().PgConn().PID())
	started[1] = true
	go func() {
		a, callErr := s2.Admit(ctx, e2.token(), e2.store(), r2)
		done2 <- admissionOutcome{a, callErr}
	}()
	pid2 := oaKeyConcurrentBlocker(t, e1.p.f, barrierCtx, app2, holders[1].Conn().PgConn().PID())
	started[2] = true
	go func() {
		state, callErr := oaKeyRetire(ctx, adminConn, version)
		retired <- retirementOutcome{state, callErr}
	}()
	oaKeyConcurrentBlocker(t, e1.p.f, barrierCtx, adminApp, pid1, pid2)
	for _, holder := range holders {
		if err = holder.Rollback(ctx); err != nil {
			t.Fatal(err)
		}
	}
	for i, ch := range []chan admissionOutcome{done1, done2} {
		select {
		case out := <-ch:
			received[i] = true
			if out.err != nil || out.receipt.State != "PENDING" || out.receipt.AttemptID == "" || out.receipt.Replay {
				t.Fatalf("independent tenant C1 must commit: index=%d state=%s err=%v", i, out.receipt.State, out.err)
			}
			var exact bool
			err = e1.p.f.owner.QueryRow(ctx, `SELECT state='PENDING' AND admission_finished AND plan_key_version=$2
			 AND plan_ciphertext IS NOT NULL AND order_id IS NULL AND tenant_id=$3 AND store_id=$4
			 AND EXISTS(SELECT 1 FROM river.river_job j WHERE j.id=a.execution_job_id AND j.kind='order_attempt_v1'
			 AND j.args->>'attempt_id'=a.attempt_id::text) FROM ops.order_attempts a WHERE attempt_id=$1`,
				out.receipt.AttemptID, version, []*lbuEnv{e1, e2}[i].tenant(), []*lbuEnv{e1, e2}[i].store()).Scan(&exact)
			if err != nil || !exact {
				t.Fatal("native cross-tenant pending ciphertext/job custody", err)
			}
		case <-ctx.Done():
			t.Fatal("bounded cross-tenant admission completion")
		}
	}
	select {
	case out := <-retired:
		received[2] = true
		oaKeyError(t, out.state, out.err, "PT409", "plan_key_referenced")
	case <-ctx.Done():
		t.Fatal("bounded retirement refusal completion")
	}
	if miCount(t, e1.p.f.owner, `SELECT count(*) FROM ops.order_attempt_plan_key_versions WHERE key_version=$1 AND retired_at IS NULL`, version) != 1 {
		t.Fatal("retirement crossed pending shared-key references")
	}
}
