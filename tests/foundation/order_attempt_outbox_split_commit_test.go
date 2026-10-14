// Purpose: REAL_PG acceptance of an independent outbox-only transaction commit.
// Depends on: isolated authenticated native C1/Executor READY and deferred pair guard.
// Used by: OA1 CI and the parent-owned exact notification-pair mutation gate.
// Invariants: I01/I08; a one-sided paired-state write must fail at COMMIT and
// preserve every native commercial, notification, credential and River artifact.

package foundation_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestOrderAttemptOutboxOnlyCommitRejectsSplitNativeReadyPair(t *testing.T) {
	s := oaCrashNativeScenario(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := s.executor.Execute(ctx, s.e.token(), s.e.store(), s.a.AttemptID, s.a.Generation)
	if err != nil {
		t.Fatal("authenticated native READY control", err)
	}
	durable := oaCrashCommitted(t, s, out)
	commercial := oaAreaCommercialSnapshot(t, s.e, s.a.AttemptID)
	custody := oaCrashCustody(t, s)
	defer clear(durable)
	defer clear(commercial)
	defer clear(custody)
	// The committed order, operation, job, outbound and secrets are all native.
	// The fixture owner attempts only a one-sided state write in a NEW Tx;
	// no runtime grant, fake order or trigger bypass participates in the probe.
	tx, err := s.e.p.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		clean, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		_ = tx.Rollback(clean)
	}()
	changed, err := tx.Exec(ctx, `UPDATE inbox.order_payment_notifications
	 SET state='VOID',void_stage='before_c3',void_code='capability'
	 WHERE create_attempt_id=$1 AND kind='initial' AND state='READY'`, s.a.AttemptID)
	if err != nil || changed.RowsAffected() != 1 {
		t.Fatalf("outbox-only statement must reach deferred COMMIT guard: rows=%d err=%v", changed.RowsAffected(), err)
	}
	var split bool
	err = tx.QueryRow(ctx, `SELECT n.state='VOID' AND n.void_stage='before_c3' AND n.void_code='capability'
	 AND n.dispatch_authorized_at IS NULL AND o.state='READY'
	 FROM inbox.order_payment_notifications n JOIN integration.operations o ON o.id=n.operation_id
	 WHERE n.create_attempt_id=$1 AND n.kind='initial'`, s.a.AttemptID).Scan(&split)
	if err != nil || !split {
		t.Fatal("positive uncommitted one-sided pair sentinel", err)
	}
	// Deliberately do not SET CONSTRAINTS: the independent transaction itself
	// must not commit an invalid outbox/operation pair.
	err = tx.Commit(ctx)
	var refusal *pgconn.PgError
	if !errors.As(err, &refusal) || refusal.Code != "23514" || refusal.Message != "notification_pair_invalid" {
		t.Fatalf("independent outbox-only COMMIT must reject split pair with exact 23514: %v", err)
	}
	if !bytes.Equal(durable, oaCrashCommitted(t, s, out)) ||
		!bytes.Equal(commercial, oaAreaCommercialSnapshot(t, s.e, s.a.AttemptID)) ||
		!bytes.Equal(custody, oaCrashCustody(t, s)) {
		t.Fatal("rejected independent COMMIT changed native proof or left an orphan job")
	}
	replay, err := s.executor.Execute(ctx, s.e.token(), s.e.store(), s.a.AttemptID, s.a.Generation)
	if err != nil {
		t.Fatal(err)
	}
	oaExecutorSameReceipt(t, out, replay)
	if !bytes.Equal(durable, oaCrashCommitted(t, s, replay)) ||
		!bytes.Equal(commercial, oaAreaCommercialSnapshot(t, s.e, s.a.AttemptID)) ||
		!bytes.Equal(custody, oaCrashCustody(t, s)) {
		t.Fatal("terminal replay changed exact native artifacts after rejected outbox-only COMMIT")
	}
}
