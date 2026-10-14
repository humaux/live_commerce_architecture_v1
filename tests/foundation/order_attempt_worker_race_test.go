// Purpose: REAL_PG proof that an actual River consumer races inline C2 safely.
// Depends on: real C1, registered River workers, exact-login pools and native lock barriers.
// Used by: parent-owned OA1 CI and the M6 authoritative SQL fence mutation.
// Invariants: the first attempt lock cannot be replaced by an incidental FK wait;
// one native commit, immutable replay and exact generation checks in both login lanes.
package foundation_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"livecommerce/internal/orderattempt"
)

// Identify the actual backend and SQL stage, not merely any descendant blocked
// by the holder. In particular a late UPDATE in execute_order_attempt is not a
// valid first-fence sentinel, and a periodic sweep cannot satisfy this predicate.
func oaRaceFencePID(t *testing.T, e *lbuEnv, ctx context.Context, holder pgx.Tx, login string) int32 {
	t.Helper()
	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()
	for {
		var pid int32
		err := e.p.f.owner.QueryRow(ctx, `WITH RECURSIVE blocked(pid) AS (
		 SELECT pid FROM pg_stat_activity WHERE $1::int=ANY(pg_blocking_pids(pid))
		 UNION SELECT a.pid FROM pg_stat_activity a JOIN blocked b ON b.pid=ANY(pg_blocking_pids(a.pid)))
		 SELECT a.pid FROM pg_stat_activity a JOIN blocked b USING(pid)
		 WHERE a.usename=$2 AND a.wait_event_type='Lock'
		 AND a.query LIKE 'SELECT ops.lock_order_attempt(%' ORDER BY a.pid LIMIT 1`,
			int32(holder.Conn().PgConn().PID()), login).Scan(&pid)
		if err == nil {
			return pid
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			t.Fatal("native first-fence PID observation", err)
		}
		select {
		case <-ctx.Done():
			t.Fatal("actual login never blocked in the first attempt-fence SQL")
		case <-tick.C:
		}
	}
}

func TestOrderAttemptWorkerRealConsumerRacesInline(t *testing.T) {
	e := lbuNew(t)
	a := oaFenceAdmit(t, e)
	plan, _ := oaMaterialPlan(t, e, oaServiceKeys(t), a.AttemptID)
	job := oaSweepJob(t, e, a.AttemptID)
	oaSweepJobState(t, e, job, "pending")
	pool := miPool(t, e.p.f, "commerce_claims_worker")
	worker := oaExecutorNew(t, pool, oaServiceKeys(t))
	inline := oaExecutorNew(t, e.p.f.runtime, oaServiceKeys(t))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var workerLogin, runtimeLogin string
	if err := pool.QueryRow(ctx, `SELECT session_user`).Scan(&workerLogin); err != nil {
		t.Fatal(err)
	}
	if err := e.p.f.runtime.QueryRow(ctx, `SELECT session_user`).Scan(&runtimeLogin); err != nil {
		t.Fatal(err)
	}
	var baseline int64
	if err := e.p.f.owner.QueryRow(ctx, `SELECT coalesce(max(id),0) FROM river.river_job WHERE kind='order_attempt_sweep_v1'`).Scan(&baseline); err != nil {
		t.Fatal(err)
	}
	oaSweepConsumer(t, pool, worker)
	oaSweepAwait(t, ctx, func() bool {
		var ready bool
		if err := e.p.f.owner.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM river.river_job
		 WHERE id>$1 AND kind='order_attempt_sweep_v1' AND state='completed')`, baseline).Scan(&ready); err != nil {
			t.Fatal(err)
		}
		return ready
	}, "actual consumer startup sweep never completed")
	oaCompositeNoEffects(t, e, a)
	holder, err := e.p.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	oaAbortCleanupTx(t, holder)
	// KEY SHARE from the protected witness FK is compatible with this lock.
	// Only a genuine row write lock can establish the intended first barrier.
	oaStateExec(t, holder, `SELECT attempt_id FROM ops.order_attempts WHERE attempt_id=$1 FOR NO KEY UPDATE`, a.AttemptID)
	oaSweepJobState(t, e, job, "available")
	workerPID := oaRaceFencePID(t, e, ctx, holder, workerLogin)
	var running bool
	if err = e.p.f.owner.QueryRow(ctx, `SELECT state='running' AND attempt=1 AND coalesce(cardinality(errors),0)=0
	 AND kind='order_attempt_v1' AND queue='claims' AND priority=1
	 AND args=jsonb_build_object('attempt_id',$2::text,'intent_generation',$3::bigint)
	 FROM river.river_job WHERE id=$1`, job, a.AttemptID, a.Generation).Scan(&running); err != nil || !running {
		t.Fatalf("original C1 job was not fetched by the actual consumer: running=%v err=%v", running, err)
	}
	done := make(chan oaExecutorOutcome, 1)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		out, callErr := inline.Execute(ctx, e.token(), e.store(), a.AttemptID, a.Generation)
		done <- oaExecutorOutcome{out, callErr}
	}()
	defer func() {
		_ = holder.Rollback(context.Background())
		cancel()
		select {
		case <-finished:
		case <-time.After(3 * time.Second):
			t.Error("inline contender did not stop")
		}
	}()
	runtimePID := oaRaceFencePID(t, e, ctx, holder, runtimeLogin)
	if runtimePID == workerPID {
		t.Fatal("consumer and inline did not use distinct physical connections")
	}
	select {
	case <-finished:
		t.Fatal("inline returned before the native attempt fence was released")
	default:
	}
	if err = holder.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	var got orderattempt.ExecutionReceipt
	select {
	case result := <-done:
		if result.err != nil {
			t.Fatal("inline contender failed after consumer won the fence", result.err)
		}
		got = result.receipt
	case <-ctx.Done():
		t.Fatal("inline contender did not finish")
	}
	oaSweepAwait(t, ctx, func() bool {
		var complete bool
		if err := e.p.f.owner.QueryRow(ctx, `SELECT state='completed' AND attempt=1
		 AND coalesce(cardinality(errors),0)=0 FROM river.river_job WHERE id=$1`, job).Scan(&complete); err != nil {
			t.Fatal(err)
		}
		return complete
	}, "actual consumer did not complete its original job once without retry")
	oaExecutorCommitted(t, e, a, plan, got, 5)
	if oaSweepJob(t, e, a.AttemptID) != job {
		t.Fatal("race replaced the original execution job")
	}
	var jobs int
	var durable orderattempt.ExecutionReceipt
	if err = e.p.f.owner.QueryRow(ctx, `SELECT state,receipt,
	 (SELECT count(*) FROM river.river_job WHERE kind='order_attempt_v1' AND args->>'attempt_id'=($1::uuid)::text)
	 FROM ops.order_attempts WHERE attempt_id=$1::uuid`, a.AttemptID).Scan(&durable.State, &durable.Receipt, &jobs); err != nil || jobs != 1 {
		t.Fatalf("race did not preserve exactly one original job: jobs=%d err=%v", jobs, err)
	}
	oaExecutorSameReceipt(t, got, durable)
	// Work intentionally returns no receipt. Read its durable result and compare
	// it with inline's receipt; do not substitute a direct worker Execute call.
	replay, err := inline.Execute(ctx, e.token(), e.store(), a.AttemptID, a.Generation)
	if err != nil {
		t.Fatal(err)
	}
	oaExecutorSameReceipt(t, got, replay)
}

func TestOrderAttemptWorkerFirstFenceAndStaleGeneration(t *testing.T) {
	for _, lane := range []string{"runtime", "claims"} {
		for _, stale := range []bool{false, true} {
			name := lane + "/exact"
			if stale {
				name = lane + "/stale"
			}
			t.Run(name, func(t *testing.T) {
				e := lbuNew(t)
				a := oaFenceAdmit(t, e)
				if stale {
					// Obtain generation two through real C2 commit plus explicit C1 CAS;
					// testing generation one against it needs no forged job or row.
					svc, err := orderattempt.NewService(e.p.f.runtime, oaServiceKeys(t))
					if err != nil {
						t.Fatal(err)
					}
					plan, _ := oaMaterialPlan(t, e, oaServiceKeys(t), a.AttemptID)
					terminal, err := oaExecutorNew(t, e.p.f.runtime, oaServiceKeys(t)).Execute(context.Background(), e.token(), e.store(), a.AttemptID, a.Generation)
					if err != nil {
						t.Fatal(err)
					}
					oaExecutorCommitted(t, e, a, plan, terminal, 5)
					req := oaServiceRequest(t, e)
					previous := a.AttemptID
					req.PreviousAttemptID = &previous
					a, err = svc.Admit(context.Background(), e.token(), e.store(), req)
					if err != nil || a.Generation != 2 || a.State != "PENDING" {
						t.Fatal("real next-generation C1 admission failed", err)
					}
				}
				pool, hash := e.p.f.runtime, tokenHash(e.token())
				if lane == "claims" {
					pool, hash = miPool(t, e.p.f, "commerce_claims_worker"), nil
				}
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				holder, err := e.p.f.owner.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				oaAbortCleanupTx(t, holder)
				oaStateExec(t, holder, `SELECT attempt_id FROM ops.order_attempts WHERE attempt_id=$1 FOR NO KEY UPDATE`, a.AttemptID)
				contender, err := pool.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				oaAbortCleanupTx(t, contender)
				oaStateExec(t, contender, `SET LOCAL lock_timeout='1s'`)
				generation := a.Generation
				if stale {
					generation--
				}
				type result struct {
					raw []byte
					err error
				}
				done := make(chan result, 1)
				finished := make(chan struct{})
				go func() {
					defer close(finished)
					raw, callErr := oaFenceCall(ctx, contender, hash, e.store(), a.AttemptID, generation)
					done <- result{raw, callErr}
				}()
				defer func() {
					_ = holder.Rollback(context.Background())
					cancel()
					select {
					case <-finished:
					case <-time.After(3 * time.Second):
						t.Error("raw fence contender did not stop")
					}
				}()
				tick := time.NewTicker(5 * time.Millisecond)
				defer tick.Stop()
				blocked := false
				for !blocked {
					select {
					case <-finished:
						t.Fatal("SQL fence returned under FOR NO KEY UPDATE: first attempt lock absent")
					default:
					}
					if err = e.p.f.owner.QueryRow(ctx, `SELECT $2::int=ANY(pg_blocking_pids($1::int))`,
						int32(contender.Conn().PgConn().PID()), int32(holder.Conn().PgConn().PID())).Scan(&blocked); err != nil {
						t.Fatal(err)
					}
					if !blocked {
						select {
						case <-ctx.Done():
							t.Fatal("raw SQL fence did not reach its native barrier")
						case <-tick.C:
						}
					}
				}
				if err = holder.Rollback(ctx); err != nil {
					t.Fatal(err)
				}
				select {
				case got := <-done:
					if stale {
						oaFenceCode(t, got.raw, got.err, "PT409")
						var pgerr *pgconn.PgError
						if !errors.As(got.err, &pgerr) || pgerr.Message != "intent_conflict" || pgerr.Hint != "" {
							t.Fatal("stale generation did not return the exact non-disclosing conflict")
						}
					} else {
						if got.err != nil {
							t.Fatal(got.err)
						}
						oaFenceOpen(t, got.raw, a)
					}
				case <-ctx.Done():
					t.Fatal("raw SQL fence failed to finish after release")
				}
				if err = contender.Rollback(ctx); err != nil {
					t.Fatal(err)
				}
				oaCompositeNoEffects(t, e, a)
			})
		}
	}
}
