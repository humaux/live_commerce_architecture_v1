// Purpose: native counterexamples for OA1 Go sweep isolation, key skew and fence waiting.
// Depends on: real C1/C2, worker/runtime logins, isolated PG and protected repair witnesses.
// Used by: parent-owned Go-area review CI; no provider, replacement SQL helper or local run.
// Invariants: one failing candidate cannot strand its successor; local key absence is
// not durable loss authority; a loser can wait within the unchanged five-second C2 budget.
package foundation_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/orderattempt"
)

func oaAreaGoSecondStore(t *testing.T, first *lbuEnv) *lbuEnv {
	t.Helper()
	p := psSetupItemsOn(t, first.p.f, 1)
	base := &tcvEnv{t: t, p: p}
	base.grantCreator("orders:read")
	mustExec(t, p.f.owner, `INSERT INTO checkout.bank_transfer_settings
	 (tenant_id,store_id,enabled,allow_cvs,bank_name,account_name,account_number,window_hours,version,updated_at)
	 VALUES($1,$2,true,false,'Synthetic bank','Synthetic merchant','12345678',6,1,clock_timestamp())`, base.tenant(), base.store())
	return &lbuEnv{ltgEnv: &ltgEnv{tcvEnv: base, money: p.stock.skus[0].ID}}
}

func oaAreaGoPoisonRepair(t *testing.T, e *lbuEnv, attempt string) {
	t.Helper()
	name := "oa_area_go_" + strings.ReplaceAll(randomUUID(), "-", "")
	fn := pgx.Identifier{"public", name}.Sanitize()
	trigger := pgx.Identifier{name}.Sanitize()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		for _, sql := range []string{"DROP TRIGGER IF EXISTS " + trigger + " ON ops.order_attempt_repair_enqueues", "DROP FUNCTION IF EXISTS " + fn + "()"} {
			if _, err := e.p.f.owner.Exec(ctx, sql); err != nil {
				t.Error("bounded native repair fault cleanup", err)
			}
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	ddl := fmt.Sprintf(`CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
	 BEGIN RAISE EXCEPTION 'oa_area_go_repair_fault' USING ERRCODE='XX000'; END $$`, fn)
	if _, err := e.p.f.owner.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	if _, err := e.p.f.owner.Exec(ctx, "REVOKE ALL ON FUNCTION "+fn+"() FROM PUBLIC"); err != nil {
		t.Fatal(err)
	}
	ddl = fmt.Sprintf("CREATE TRIGGER %s BEFORE INSERT ON ops.order_attempt_repair_enqueues FOR EACH ROW WHEN (NEW.attempt_id='%s'::uuid) EXECUTE FUNCTION %s()", trigger, attempt, fn)
	if _, err := e.p.f.owner.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
}

func TestOrderAttemptAreaGoSweepContinuesAfterPoisonedHead(t *testing.T) {
	first := oaExpiryIsolated(t)
	head := oaFenceAdmit(t, first)
	second := oaAreaGoSecondStore(t, first)
	tail := oaFenceAdmit(t, second)
	headJob, tailJob := oaSweepJob(t, first, head.AttemptID), oaSweepJob(t, second, tail.AttemptID)
	oaSweepJobState(t, first, headJob, "discarded")
	oaSweepJobState(t, second, tailJob, "discarded")
	headIntent, tailIntent := oaSweepIntent(t, first, head.AttemptID), oaSweepIntent(t, second, tail.AttemptID)
	pool := miPool(t, first.p.f, "commerce_claims_worker")
	var exact bool
	// Calls the real opaque oldest-first discovery; no synthetic deadline or rank.
	err := pool.QueryRow(context.Background(), `SELECT ops.order_attempt_sweep('discover',NULL,NULL)=
	 jsonb_build_array(jsonb_build_object('attempt_id',$1::text,'intent_generation',$2::bigint),
	 jsonb_build_object('attempt_id',$3::text,'intent_generation',$4::bigint))`, head.AttemptID, head.Generation, tail.AttemptID, tail.Generation).Scan(&exact)
	if err != nil || !exact {
		t.Fatal("poisoned head and healthy successor must be the actual ordered batch", err)
	}
	oaAreaGoPoisonRepair(t, first, head.AttemptID)
	executor := oaExecutorNew(t, pool, oaServiceKeys(t))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	got, err := executor.Sweep(ctx)
	var admission *orderattempt.AdmissionError
	if !errors.As(err, &admission) || admission.SQLState() != "XX000" || admission.OutcomeUnknown() {
		t.Fatal("native candidate fault must remain observable and retryable", err)
	}
	if strings.Contains(err.Error(), "oa_area_go_repair_fault") || strings.Contains(err.Error(), head.AttemptID) {
		t.Fatal("sweep exposed a raw native fault or attempt identifier")
	}
	if got.Scanned != 2 || got.Requeued != 1 || got.Aborted != 0 {
		t.Fatal("poisoned first candidate prevented bounded progress of its healthy successor", got)
	}
	oaSweepRepair(t, second, tail, tailJob, tailIntent)
	if oaSweepJob(t, first, head.AttemptID) != headJob || oaSweepIntent(t, first, head.AttemptID) != headIntent {
		t.Fatal("failed candidate changed its job or frozen authority")
	}
	oaExecutorNoEffects(t, first, head)
}

func TestOrderAttemptAreaGoLocalPlanRingSkewRetainsPending(t *testing.T) {
	e := oaExpiryIsolated(t)
	a := oaFenceAdmit(t, e)
	plan, _ := oaMaterialPlan(t, e, oaServiceKeys(t), a.AttemptID)
	intent := oaSweepIntent(t, e, a.AttemptID)
	stale, err := orderattempt.NewPlanKeys("fixture_v2", map[string][]byte{"fixture_v2": bytes.Repeat([]byte{19}, 32)})
	if err != nil {
		t.Fatal(err)
	}
	pool := miPool(t, e.p.f, "commerce_claims_worker")
	got, err := oaExecutorNew(t, pool, stale).Execute(context.Background(), "", "", a.AttemptID, a.Generation)
	var admission *orderattempt.AdmissionError
	if !errors.As(err, &admission) || admission.OutcomeUnknown() || got.State != "" {
		t.Fatal("local ring skew cannot authorize terminal PLAN_UNAVAILABLE", err)
	}
	oaExecutorNoEffects(t, e, a)
	if oaSweepIntent(t, e, a.AttemptID) != intent {
		t.Fatal("temporary local ring skew changed admitted authority")
	}
	// A concurrent correctly configured process still possesses the original key.
	got, err = oaExecutorNew(t, pool, oaServiceKeys(t)).Execute(context.Background(), "", "", a.AttemptID, a.Generation)
	if err != nil {
		t.Fatal("original key remains usable after another process's skew", err)
	}
	oaExecutorCommitted(t, e, a, plan, got, 5)
}

func TestOrderAttemptAreaGoFenceWaitWithinC2Budget(t *testing.T) {
	for _, worker := range []bool{false, true} {
		t.Run(fmt.Sprintf("worker=%v", worker), func(t *testing.T) {
			e := oaExpiryIsolated(t)
			a := oaFenceAdmit(t, e)
			plan, _ := oaMaterialPlan(t, e, oaServiceKeys(t), a.AttemptID)
			pool, token, store := e.p.f.runtime, e.token(), e.store()
			if worker {
				pool, token, store = miPool(t, e.p.f, "commerce_claims_worker"), "", ""
			}
			executor := oaExecutorNew(t, pool, oaServiceKeys(t))
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			holder, err := e.p.f.owner.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			oaAbortCleanupTx(t, holder)
			defer holder.Rollback(context.Background())
			oaStateExec(t, holder, `SELECT attempt_id FROM ops.order_attempts WHERE attempt_id=$1 FOR NO KEY UPDATE`, a.AttemptID)
			var login string
			if err = pool.QueryRow(ctx, `SELECT session_user`).Scan(&login); err != nil {
				t.Fatal(err)
			}
			done := make(chan oaExecutorOutcome, 1)
			finished := make(chan struct{})
			go func() {
				defer close(finished)
				out, callErr := executor.Execute(ctx, token, store, a.AttemptID, a.Generation)
				done <- oaExecutorOutcome{out, callErr}
			}()
			defer func() {
				_ = holder.Rollback(context.Background())
				cancel()
				select {
				case <-finished:
				case <-time.After(3 * time.Second):
					t.Error("bounded native executor cleanup did not finish")
				}
			}()
			pid := oaRaceFencePID(t, e, ctx, holder, login)
			// A positively observed native fence must tolerate more than the old
			// one-second lock timeout, while leaving ample time in the fixed 5s C2.
			timer := time.NewTimer(1300 * time.Millisecond)
			defer timer.Stop()
			select {
			case <-done:
				t.Fatal("executor returned before a bounded native fence wait ended")
			case <-timer.C:
			}
			var waiting bool
			if err = e.p.f.owner.QueryRow(ctx, `SELECT wait_event_type='Lock' AND $2::int=ANY(pg_blocking_pids(pid))
			 FROM pg_stat_activity WHERE pid=$1`, pid, int32(holder.Conn().PgConn().PID())).Scan(&waiting); err != nil || !waiting {
				t.Fatal("same physical executor must still wait on its first fence", err)
			}
			if err = holder.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
			select {
			case outcome := <-done:
				if outcome.err != nil {
					t.Fatal("executor did not continue after the bounded fence wait", outcome.err)
				}
				oaExecutorCommitted(t, e, a, plan, outcome.receipt, 5)
			case <-ctx.Done():
				t.Fatal("executor exceeded its unchanged enclosing bound")
			}
		})
	}
}
