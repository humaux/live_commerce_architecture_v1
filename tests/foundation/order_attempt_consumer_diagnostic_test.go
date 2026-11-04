// Purpose: bounded failure-only evidence for native order-attempt Meta consumption.
// Depends on: owner reads of River/pg_stat_activity and pgxpool statistics; no product hooks.
// Used by: oaNotificationSignedConversation before its registered client cleanup.
// Invariants: one 2s budget, aggregate whitelist only, no query/error/identity/payload output.

package foundation_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// This is evidence after an existing failure, not readiness or an acceptance gate.
func oaNotificationConsumerDiagnostic(t *testing.T, owner, worker, consumer *pgxpool.Pool, clientID string, stopped <-chan struct{}) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	clientStopped := false
	select {
	case <-stopped:
		clientStopped = true
	default:
	}
	t.Logf("OA1 consumer diagnostic client_stopped=%t fetch_error=UNKNOWN", clientStopped)
	for _, item := range []struct {
		role string
		pool *pgxpool.Pool
	}{{"owner", owner}, {"worker", worker}, {"consumer", consumer}} {
		s := item.pool.Stat()
		t.Logf("OA1 consumer pool role=%s total=%d acquired=%d idle=%d constructing=%d max=%d empty_acquires=%d canceled_acquires=%d",
			item.role, s.TotalConns(), s.AcquiredConns(), s.IdleConns(), s.ConstructingConns(), s.MaxConns(), s.EmptyAcquireCount(), s.CanceledAcquireCount())
	}
	// All emitted strings are literal labels or native River state enum values.
	// Exact fixture logins and client ID are query parameters only, never output.
	var snapshot string
	err := owner.QueryRow(ctx, `WITH job_states AS (
        SELECT state::text AS state, count(*) AS n
        FROM river_meta.river_job GROUP BY state
    ), wait_groups AS (
        SELECT CASE WHEN usename=$2 THEN 'worker' ELSE 'consumer' END AS role,
          CASE WHEN wait_event_type IS NULL THEN 'none'
            WHEN wait_event_type IN ('Activity','BufferPin','Client','Extension','IO','IPC','Lock','LWLock','Timeout')
              THEN wait_event_type ELSE 'other' END AS wait_type,
          count(*) AS n,
          count(*) FILTER (WHERE cardinality(pg_blocking_pids(pid))>0) AS blocked
        FROM pg_stat_activity
        WHERE datname=current_database() AND usename IN ($2,$3)
        GROUP BY 1,2
    ), waits AS (
        SELECT * FROM wait_groups ORDER BY role,wait_type LIMIT 20
    )
    SELECT jsonb_build_object(
        'queue_paused',EXISTS(SELECT 1 FROM river_meta.river_queue
          WHERE name='meta_inbox' AND paused_at IS NOT NULL),
        'current_client_running',(SELECT count(*) FROM river_meta.river_job
          WHERE queue='meta_inbox' AND state='running' AND $1=ANY(attempted_by)),
        'job_states',coalesce((SELECT jsonb_object_agg(state,n) FROM job_states),'{}'::jsonb),
        'pg_waits_truncated',(SELECT count(*)>20 FROM wait_groups),
        'pg_waits',coalesce((SELECT jsonb_agg(jsonb_build_object(
          'role',role,'wait_type',wait_type,'count',n,'blocked',blocked) ORDER BY role,wait_type)
          FROM waits),'[]'::jsonb))::text`, clientID, worker.Config().ConnConfig.User,
		consumer.Config().ConnConfig.User).Scan(&snapshot)
	if err != nil {
		t.Logf("OA1 consumer diagnostic snapshot_error=%s", oaNotificationDiagnosticError(err))
		return
	}
	t.Logf("OA1 consumer diagnostic snapshot=%s", snapshot)
}

// Only context classifications or a five-character SQLSTATE may leave this helper.
func oaNotificationDiagnosticError(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "deadline"
	}
	if errors.Is(err, context.Canceled) {
		return "canceled"
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && len(pgErr.Code) == 5 {
		for _, c := range pgErr.Code {
			if !(c >= '0' && c <= '9' || c >= 'A' && c <= 'Z') {
				return "database_error"
			}
		}
		return "sqlstate_" + pgErr.Code
	}
	return "database_error"
}
