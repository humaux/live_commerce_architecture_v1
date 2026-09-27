package foundation_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/live"
	"livecommerce/internal/platform"
)

// Keep LMR's confirmed CommandComplete+ReadyForQuery ACK drop. Only the SQL
// selector changes: this is the browser input's final Start permission.
type brwReserveAckConn struct {
	*lmrAckConn
	seen *atomic.Bool
}

func (c *brwReserveAckConn) Write(p []byte) (int, error) {
	n, err := c.Conn.Write(p)
	if err == nil && n == len(p) && len(p) > 6 && p[0] == 'Q' &&
		int(binary.BigEndian.Uint32(p[1:5])) == len(p)-1 && p[len(p)-1] == 0 &&
		bytes.HasPrefix(bytes.ToLower(bytes.TrimSpace(p[5:len(p)-1])),
			[]byte("select live.reserve_media_input_start(")) {
		c.seen.Store(true)
		c.pending.Store(true)
	}
	return n, err
}

func brwReserveAckPool(t *testing.T, base *pgxpool.Pool, seen, committed *atomic.Bool) *pgxpool.Pool {
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
		return &brwReserveAckConn{lmrAckConn: &lmrAckConn{
			Conn: conn, mode: "drop", committed: committed,
		}, seen: seen}, nil
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

type brwEgressReadCounter struct {
	base  http.RoundTripper
	reads atomic.Int32
}

func (c *brwEgressReadCounter) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Path == "/twirp/livekit.Egress/ListEgress" {
		c.reads.Add(1)
	}
	return c.base.RoundTrip(r)
}

func TestLiveBrowserInputBRW02FinalReservationCommitAckLoss(t *testing.T) {
	f := brwWireStarted(t)
	brwReserveGrant(t, f.h)
	seen, committed := &atomic.Bool{}, &atomic.Bool{}
	fault := brwReserveAckPool(t, f.h.executor, seen, committed)
	reads := &brwEgressReadCounter{base: f.egress.Transport}
	f.egress.Transport = reads
	client, err := live.NewBrowserInputMediaClient(context.Background(), f.h.worker, fault,
		f.keys, []live.MediaProject{f.egress}, f.h.runtime, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		defer cancel()
		if err := client.StopAndCancel(ctx); err != nil {
			t.Errorf("browser fault worker stop: %v", err)
		}
	})
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) && !committed.Load() {
		time.Sleep(10 * time.Millisecond)
	}
	if !seen.Load() || !committed.Load() {
		t.Fatalf("final Start reservation COMMIT ACK was not dropped: seen=%t committed=%t",
			seen.Load(), committed.Load())
	}
	_, _, reserved := brwWireFacts(t, f.h)
	if reserved == nil || f.inputs.Load() == 0 || f.starts.Load() != 0 || reads.reads.Load() != 0 {
		t.Fatalf("lost ACK authorized I/O or did not commit: reserved=%v input=%d start=%d reads=%d",
			reserved, f.inputs.Load(), f.starts.Load(), reads.reads.Load())
	}
	// No owner clock rewrite: the original 30s lease must expire naturally.
	deadline = time.Now().Add(75 * time.Second)
	var egressID, jobState string
	var jobID int64
	var finalized *time.Time
	for time.Now().Before(deadline) {
		if err := f.h.lp.f.owner.QueryRow(context.Background(), `SELECT coalesce(x.egress_id,''),o.job_id,j.state,j.finalized_at
			FROM integration.operations o JOIN live.media_execution_state x ON x.operation_id=o.id
			JOIN river_media.river_job j ON j.id=o.job_id WHERE o.id=$1`, f.h.plan.OperationID).
			Scan(&egressID, &jobID, &jobState, &finalized); err != nil {
			t.Fatal(err)
		}
		if reads.reads.Load() > 0 && egressID == "EG_brw" {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if reads.reads.Load() == 0 || egressID != "EG_brw" || f.starts.Load() != 0 ||
		jobID != f.h.plan.JobID || finalized != nil ||
		jobState == "completed" || jobState == "cancelled" || jobState == "discarded" {
		t.Fatalf("same original job did not reconcile without Start: reads=%d egress=%s start=%d job=%d/%s finalized=%v",
			reads.reads.Load(), egressID, f.starts.Load(), jobID, jobState, finalized)
	}
}

func brwBlockedFinalReserve(t *testing.T, h *brwHarness, lease brwLease) (int, <-chan error) {
	t.Helper()
	conn, err := h.executor.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(conn.Release)
	var pid int
	if err := conn.QueryRow(context.Background(), `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	done := make(chan error, 1)
	go func() {
		var raw []byte
		done <- conn.QueryRow(ctx, `SELECT live.reserve_media_input_start(
			$1::uuid,$2::bigint,$3::bytea,$4::text,$5::text,$6::text,$7::text,
			$8::boolean,$9::boolean,$10::boolean,$11::boolean)`,
			h.plan.OperationID, lease.generation, lease.key, h.plan.RoomName,
			h.grant.PublisherIdentity, "PA_brw", "ACTIVE", true, false, true, false).Scan(&raw)
	}()
	return pid, done
}

func brwAwaitDeniedReserve(t *testing.T, done <-chan error) {
	t.Helper()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("concurrent authority mutation allowed final Start reservation")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("blocked final reservation did not resolve")
	}
}

func TestLiveBrowserInputBRW02ConcurrentStopAndRevokeBeforeFinalReservation(t *testing.T) {
	t.Run("revoke-commits-while-final-reserve-waits", func(t *testing.T) {
		f := brwWireStarted(t)
		brwReserveGrant(t, f.h)
		lease, err := brwClaim(context.Background(), f.h.bicHarness, f.h.plan, bytes.Repeat([]byte{0x91}, 32))
		if err != nil || lease.disposition != "claimed" {
			t.Fatalf("claim: %+v %v", lease, err)
		}
		brwSelectObserve(t, f.h, lease)
		holder, err := f.h.lp.f.owner.Begin(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		defer holder.Rollback(context.Background())
		var holderPID int
		if err := holder.QueryRow(context.Background(), `SELECT pg_backend_pid()`).Scan(&holderPID); err != nil {
			t.Fatal(err)
		}
		// The final reservation takes binding SHARE before authorization SHARE.
		// Holding the binding lets revoke commit before the waiter can take its
		// authorization SHARE lock; holding operation here would self-deadlock.
		if _, err := holder.Exec(context.Background(), `SELECT id FROM integration.bindings WHERE id=$1 FOR UPDATE`, f.h.media); err != nil {
			t.Fatal(err)
		}
		waiterPID, done := brwBlockedFinalReserve(t, f.h, lease)
		lmaObserveBlock(t, f.h.lp.f.owner, waiterPID, holderPID, false)
		revokeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := f.h.registrar.Exec(revokeCtx,
			`SELECT live.revoke_prepared_media($1::uuid,$2::uuid,$3::uuid,'operator_revoke')`,
			f.h.lp.f.tenantA, f.h.lp.f.storeA1, f.h.input.AuthorizationID); err != nil {
			t.Fatal(err)
		}
		var revoked time.Time
		if err := f.h.lp.f.owner.QueryRow(context.Background(),
			`SELECT revoked_at FROM live.media_authorization_revocations WHERE authorization_id=$1`,
			f.h.input.AuthorizationID).Scan(&revoked); err != nil || revoked.IsZero() {
			t.Fatalf("revoke not durably committed before reservation: %v %v", revoked, err)
		}
		if err := holder.Commit(context.Background()); err != nil {
			t.Fatal(err)
		}
		brwAwaitDeniedReserve(t, done)
		_, _, reserved := brwWireFacts(t, f.h)
		if reserved != nil || f.starts.Load() != 0 {
			t.Fatalf("revoke-first final gate authorized Start: reserved=%v start=%d", reserved, f.starts.Load())
		}
	})

	t.Run("stop-locks-original-operation-before-final-reserve", func(t *testing.T) {
		f := brwWireStarted(t)
		brwReserveGrant(t, f.h)
		lease, err := brwClaim(context.Background(), f.h.bicHarness, f.h.plan, bytes.Repeat([]byte{0x92}, 32))
		if err != nil || lease.disposition != "claimed" {
			t.Fatalf("claim: %+v %v", lease, err)
		}
		brwSelectObserve(t, f.h, lease)
		holder, err := f.h.lp.f.owner.Begin(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		defer holder.Rollback(context.Background())
		var holderPID int
		if err := holder.QueryRow(context.Background(), `SELECT pg_backend_pid()`).Scan(&holderPID); err != nil {
			t.Fatal(err)
		}
		if _, err := holder.Exec(context.Background(), `SELECT attempt_id FROM live.media_execution_state WHERE attempt_id=$1 FOR UPDATE`,
			f.h.plan.AttemptID); err != nil {
			t.Fatal(err)
		}
		cfg := f.h.lp.f.runtime.Config()
		cfg.MaxConns = 1
		name := "brw_stop_wait_" + t04Tag()
		if cfg.ConnConfig.RuntimeParams == nil {
			cfg.ConnConfig.RuntimeParams = make(map[string]string)
		}
		cfg.ConnConfig.RuntimeParams["application_name"] = name
		stopPool, err := pgxpool.NewWithConfig(context.Background(), cfg)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(stopPool.Close)
		stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		t.Cleanup(cancel)
		stopDone := make(chan error, 1)
		go func() {
			stopDone <- platform.WithScope(stopCtx, stopPool, f.h.logins.b,
				f.h.lp.f.storeA1, "store:read", func(tx pgx.Tx, scope platform.Scope) error {
					_, err := f.h.planner.RequestStop(stopCtx, tx, scope,
						f.h.logins.b, t04Key("brw-fault-stop"), live.MediaStopInput{
							SessionID: f.h.session, AttemptID: f.h.plan.AttemptID,
						})
					return err
				})
		}()
		var stopPID int
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if err := f.h.lp.f.owner.QueryRow(context.Background(), `SELECT coalesce((SELECT pid FROM pg_stat_activity
				WHERE application_name=$1 AND $2::int=ANY(pg_blocking_pids(pid)) AND wait_event_type='Lock' LIMIT 1),0)`,
				name, holderPID).Scan(&stopPID); err != nil {
				t.Fatal(err)
			}
			if stopPID != 0 {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		if stopPID == 0 {
			t.Fatal("Stop did not reach held input projection lock")
		}
		waiterPID, reserveDone := brwBlockedFinalReserve(t, f.h, lease)
		lmaObserveBlock(t, f.h.lp.f.owner, waiterPID, stopPID, false)
		if err := holder.Commit(context.Background()); err != nil {
			t.Fatal(err)
		}
		select {
		case err := <-stopDone:
			if err != nil {
				t.Fatalf("committed Stop failed: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("Stop did not complete after releasing projection lock")
		}
		brwAwaitDeniedReserve(t, reserveDone)
		var stopped *time.Time
		if err := f.h.lp.f.owner.QueryRow(context.Background(), `SELECT stop_requested_at FROM live.media_execution_state WHERE attempt_id=$1`,
			f.h.plan.AttemptID).Scan(&stopped); err != nil || stopped == nil {
			t.Fatalf("Stop did not commit before denied reservation: %v %v", stopped, err)
		}
		_, _, reserved := brwWireFacts(t, f.h)
		if reserved != nil || f.starts.Load() != 0 {
			t.Fatalf("Stop-first final gate authorized Start: reserved=%v start=%d", reserved, f.starts.Load())
		}
	})
}
