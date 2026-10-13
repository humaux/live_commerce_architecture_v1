// Purpose: native COMMIT acknowledgement-loss acceptance for C1 and C2/recovery.
// Depends on: real runtime/worker pools, concrete executor and the existing PG wire fault.
// Used by: OA1 CI contract sections 6a/6c; never substitutes an in-memory result.
// Invariants: durable commits survive lost replies, uncertain callers see no receipt,
// and exact replay preserves one order, one inventory hold and the original pay entry.
package foundation_test

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/orderattempt"
)

func oaCommitAckPool(t *testing.T, base *pgxpool.Pool) (*pgxpool.Pool, *hpCommitLoss) {
	t.Helper()
	config := base.Config()
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
	return pool, loss
}

func oaCommitAckUnknown(t *testing.T, loss *hpCommitLoss, err error) {
	t.Helper()
	var coded *orderattempt.AdmissionError
	if !loss.committed.Load() || !errors.As(err, &coded) || !coded.OutcomeUnknown() {
		t.Fatalf("witnessed successful PG COMMIT with dropped acknowledgement must remain UNKNOWN: committed=%v err=%v", loss.committed.Load(), err)
	}
}

func TestOrderAttemptAdmissionCommitAcknowledgementLoss(t *testing.T) {
	e := lbuNew(t)
	keys := oaServiceKeys(t)
	pool, loss := oaCommitAckPool(t, e.p.f.runtime)
	faulty, err := orderattempt.NewService(pool, keys)
	if err != nil {
		t.Fatal(err)
	}
	req := oaServiceRequest(t, e)
	loss.armed.Store(true)
	got, err := faulty.Admit(context.Background(), e.token(), e.store(), req)
	oaCommitAckUnknown(t, loss, err)
	if got.AttemptID != "" || got.ScopeID != "" || got.State != "" || len(got.Receipt) != 0 {
		t.Fatal("unacknowledged C1 exposed an authoritative admission receipt")
	}
	service, err := orderattempt.NewService(e.p.f.runtime, keys)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := service.Admit(context.Background(), e.token(), e.store(), req)
	if err != nil || !replay.Replay || replay.State != "PENDING" || replay.Generation != 1 || replay.ScopeID != req.ExpectedScopeID {
		t.Fatalf("lost C1 acknowledgement did not recover the admitted intent: %+v err=%v", replay, err)
	}
	var attempts, jobs int
	err = e.p.f.owner.QueryRow(context.Background(), `SELECT
	 (SELECT count(*) FROM ops.order_attempts WHERE scope_id=$1),
	 (SELECT count(*) FROM river.river_job WHERE kind='order_attempt_v1' AND args->>'attempt_id'=$2)`, replay.ScopeID, replay.AttemptID).Scan(&attempts, &jobs)
	if err != nil || attempts != 1 || jobs != 1 {
		t.Fatalf("lost C1 reply duplicated admission/job: attempts=%d jobs=%d err=%v", attempts, jobs, err)
	}
	oaExecutorNoEffects(t, e, replay)
	plan, _ := oaMaterialPlan(t, e, keys, replay.AttemptID)
	terminal, err := oaExecutorNew(t, e.p.f.runtime, keys).Execute(context.Background(), e.token(), e.store(), replay.AttemptID, replay.Generation)
	if err != nil {
		t.Fatal("execute the only durably admitted intent", err)
	}
	oaExecutorCommitted(t, e, replay, plan, terminal, 5)
}

func TestOrderAttemptExecutionAndRecoveryCommitAcknowledgementLoss(t *testing.T) {
	for _, worker := range []bool{false, true} {
		name := "runtime"
		if worker {
			name = "claims-worker"
		}
		t.Run(name, func(t *testing.T) {
			e := lbuNew(t)
			a := oaFenceAdmit(t, e)
			keys := oaServiceKeys(t)
			plan, _ := oaMaterialPlan(t, e, keys, a.AttemptID)
			base, token, store := e.p.f.runtime, e.token(), e.store()
			if worker {
				base, token, store = miPool(t, e.p.f, "commerce_claims_worker"), "", ""
			}
			pool, loss := oaCommitAckPool(t, base)
			faulty := oaExecutorNew(t, pool, keys)
			loss.armed.Store(true)
			got, err := faulty.Execute(context.Background(), token, store, a.AttemptID, a.Generation)
			oaCommitAckUnknown(t, loss, err)
			if got.State != "" || len(got.Receipt) != 0 {
				t.Fatal("unacknowledged C2 exposed a terminal receipt")
			}
			var durable orderattempt.ExecutionReceipt
			if err := e.p.f.owner.QueryRow(context.Background(), `SELECT state,receipt FROM ops.order_attempts WHERE attempt_id=$1`, a.AttemptID).Scan(&durable.State, &durable.Receipt); err != nil {
				t.Fatal(err)
			}
			original := oaExecutorCommitted(t, e, a, plan, durable, 5)
			// Drop the acknowledgement again, this time for the read-only terminal
			// replay transaction. No browser receipt or current draft is supplied.
			loss.committed.Store(false)
			got, err = faulty.Execute(context.Background(), token, store, a.AttemptID, a.Generation)
			oaCommitAckUnknown(t, loss, err)
			if got.State != "" || len(got.Receipt) != 0 {
				t.Fatal("lost recovery acknowledgement exposed a terminal receipt")
			}
			recovered, err := oaExecutorNew(t, base, keys).Execute(context.Background(), token, store, a.AttemptID, a.Generation)
			if err != nil {
				t.Fatal("exact recovery after two lost replies", err)
			}
			oaExecutorSameReceipt(t, durable, recovered)
			if again := oaExecutorCommitted(t, e, a, plan, recovered, 5); again != original {
				t.Fatal("recovery rotated or rewrote the durable pay entry")
			}
		})
	}
}
