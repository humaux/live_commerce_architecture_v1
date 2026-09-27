package foundation_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"

	"livecommerce/internal/live"
	"livecommerce/migrations"
)

// Reserve runs as an implicit transaction. Consume its successful PostgreSQL
// CommandComplete and idle ReadyForQuery, then discard only that acknowledgement.
// A sent SQL frame alone does not establish that the reservation committed.
type lmeReserveLossConn struct {
	net.Conn
	loss       *hpCommitLoss
	seen       *atomic.Bool
	sawReserve atomic.Bool
	pending    atomic.Bool
}

func (c *lmeReserveLossConn) Write(p []byte) (int, error) {
	n, err := c.Conn.Write(p)
	if err == nil && n == len(p) && len(p) >= 6 && p[0] == 'Q' &&
		int(binary.BigEndian.Uint32(p[1:5])) == len(p)-1 && p[len(p)-1] == 0 &&
		bytes.HasPrefix(bytes.ToLower(bytes.TrimSpace(p[5:len(p)-1])), []byte("select live.reserve_media_start(")) {
		c.sawReserve.Store(true)
		c.loss.armed.Store(true)
		c.seen.Store(true)
		c.pending.Store(true)
	}
	return n, err
}

func (c *lmeReserveLossConn) Read(p []byte) (int, error) {
	if !c.pending.CompareAndSwap(true, false) {
		return c.Conn.Read(p)
	}
	_ = c.Conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	defer c.Conn.Close()
	completed := false
	for {
		var head [5]byte
		if _, err := io.ReadFull(c.Conn, head[:]); err != nil {
			return 0, err
		}
		length := int(binary.BigEndian.Uint32(head[1:]))
		if length < 4 || length > 1<<20 {
			return 0, io.ErrUnexpectedEOF
		}
		body := make([]byte, length-4)
		if _, err := io.ReadFull(c.Conn, body); err != nil {
			return 0, err
		}
		if head[0] == 'C' && bytes.HasPrefix(body, []byte("SELECT ")) {
			completed = true
		}
		if head[0] == 'Z' && len(body) == 1 && body[0] == 'I' && completed {
			c.loss.committed.Store(true)
			return 0, io.ErrUnexpectedEOF
		}
		if head[0] == 'E' || head[0] == 'Z' {
			return 0, io.ErrUnexpectedEOF
		}
	}
}

func lmeLossPool(t *testing.T, base *pgxpool.Pool, loss *hpCommitLoss, seen *atomic.Bool) *pgxpool.Pool {
	t.Helper()
	cfg := base.Config()
	cfg.MaxConns = 1
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	dial := cfg.ConnConfig.DialFunc
	cfg.ConnConfig.DialFunc = func(ctx context.Context, network, address string) (net.Conn, error) {
		conn, err := dial(ctx, network, address)
		if err != nil {
			return nil, err
		}
		return &lmeReserveLossConn{Conn: conn, loss: loss, seen: seen}, nil
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestLiveMediaExecutionLME05ReserveCommitLossAndAtomicFailures(t *testing.T) {
	t.Run("concurrent-native-workers-one-wire", func(t *testing.T) {
		var h *lmeHarness
		h = lmeSetup(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/twirp/livekit.Egress/StartEgress" {
				http.Error(w, "unexpected", 500)
				return
			}
			_, _ = io.Copy(io.Discard, r.Body)
			lmeReply(w, h.observation("EG_lme05race", "EGRESS_ACTIVE", 100, 110, 0))
		})
		h.startWorker(t)
		h.startWorker(t)
		f := h.await(t, 15*time.Second, func(f lmeFacts) bool { return f.observations == 1 })
		if h.starts.Load() != 1 || f.operation != "UNKNOWN" || f.egress != "EG_lme05race" || !f.reserved {
			t.Fatalf("concurrent workers duplicated wire or lost facts: %+v starts=%d", f, h.starts.Load())
		}
	})
	t.Run("real-commit-ack-loss", func(t *testing.T) {
		var h *lmeHarness
		h = lmeSetup(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/twirp/livekit.Egress/ListEgress" {
				http.Error(w, "unexpected wire", 500)
				return
			}
			lmeReply(w, `{"items":[]}`)
		})
		loss := &hpCommitLoss{}
		seen := &atomic.Bool{}
		faultPool := lmeLossPool(t, h.executor, loss, seen)
		client, err := live.NewMediaClient(context.Background(), h.worker, faultPool, h.keys, []live.MediaProject{h.project}, 1)
		if err != nil {
			t.Fatal(err)
		}
		if err = client.Start(context.Background()); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = client.StopAndCancel(ctx)
		})
		deadline := time.Now().Add(15 * time.Second)
		for time.Now().Before(deadline) && !loss.committed.Load() {
			time.Sleep(10 * time.Millisecond)
		}
		if !seen.Load() || !loss.committed.Load() {
			t.Fatalf("real reserve COMMIT acknowledgement was not lost: seen=%t committed=%t", seen.Load(), loss.committed.Load())
		}
		f := h.await(t, 5*time.Second, func(f lmeFacts) bool { return f.reserved })
		if h.starts.Load() != 0 || f.observations != 0 {
			t.Fatalf("unknown commit sent Start or fabricated observation: %+v starts=%d", f, h.starts.Load())
		}
		// Any later claim, whether this client retries or a replacement starts,
		// must be reconcile; it may never recover a second Start allowance.
		if _, err = h.lp.f.owner.Exec(context.Background(), `UPDATE integration.operations SET lease_until=clock_timestamp()-interval '1 second' WHERE id=$1`, h.plan.OperationID); err != nil {
			t.Fatal(err)
		}
		lease := h.claim(t, 30)
		if lease.disposition != "claimed" || lease.mode != "reconcile" {
			t.Fatalf("reserved commit retried dispatch: %+v", lease)
		}
		h.load(t, lease)
		if h.starts.Load() != 0 {
			t.Fatal("Start sent after unknown reserve COMMIT")
		}
	})
	t.Run("pre-reservation-failure", func(t *testing.T) {
		h := lmeSetup(t, func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "no provider", 500) })
		lease := h.claim(t, 30)
		if lease.mode != "dispatch" {
			t.Fatal("bad dispatch fixture")
		}
		if _, err := h.registrar.Exec(context.Background(), `SELECT live.revoke_prepared_media($1::uuid,$2::uuid,$3::uuid,$4::text)`,
			h.lp.f.tenantA, h.lp.f.storeA1, h.input.AuthorizationID, "operator_revoke"); err != nil {
			t.Fatal(err)
		}
		if err := h.reserve(t, lease); err == nil {
			t.Fatal("revoked unreserved dispatch obtained wire allowance")
		}
		if f := h.facts(t); f.reserved || f.observations != 0 || h.starts.Load() != 0 {
			t.Fatalf("pre-reservation failure wrote facts: %+v", f)
		}
	})
	t.Run("observation-write-rollback", func(t *testing.T) {
		h := lmeSetup(t, func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "no provider", 500) })
		lease := h.claim(t, 30)
		if err := h.reserve(t, lease); err != nil {
			t.Fatal(err)
		}
		before := h.facts(t)
		name := "lme_fail_" + t04Tag()
		function := pgx.Identifier{"public", name}.Sanitize()
		trigger := pgx.Identifier{name}.Sanitize()
		mustExec(t, h.lp.f.owner, fmt.Sprintf(`CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.attempt_id=%s::uuid THEN RAISE EXCEPTION 'lme fixture fault'; END IF; RETURN NEW; END $$`, function, quoteLiteral(h.plan.AttemptID)))
		mustExec(t, h.lp.f.owner, `CREATE TRIGGER `+trigger+` BEFORE INSERT ON live.media_observations FOR EACH ROW EXECUTE FUNCTION `+function+`() `)
		t.Cleanup(func() {
			mustExec(t, h.lp.f.owner, `DROP TRIGGER `+trigger+` ON live.media_observations`)
			mustExec(t, h.lp.f.owner, `DROP FUNCTION `+function+`() `)
		})
		if _, err := h.record(t, lease, "START", "EG_lme05", "EGRESS_ACTIVE", 100, 110, 0); err == nil {
			t.Fatal("observation fault committed")
		}
		if got := h.facts(t); got != before {
			t.Fatalf("failed observation partially committed: before=%+v after=%+v", before, got)
		}
	})
	t.Run("finish-event-rollback", func(t *testing.T) {
		h := lmeSetup(t, func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "no provider", 500) })
		lease := h.claim(t, 30)
		if err := h.reserve(t, lease); err != nil {
			t.Fatal(err)
		}
		before := h.facts(t)
		name := "lme_finish_" + t04Tag()
		function := pgx.Identifier{"public", name}.Sanitize()
		trigger := pgx.Identifier{name}.Sanitize()
		mustExec(t, h.lp.f.owner, fmt.Sprintf(`CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.operation_id=%s::uuid THEN RAISE EXCEPTION 'lme fixture fault'; END IF; RETURN NEW; END $$`, function, quoteLiteral(h.plan.OperationID)))
		mustExec(t, h.lp.f.owner, `CREATE TRIGGER `+trigger+` BEFORE INSERT ON integration.operation_events FOR EACH ROW EXECUTE FUNCTION `+function+`() `)
		t.Cleanup(func() {
			mustExec(t, h.lp.f.owner, `DROP TRIGGER `+trigger+` ON integration.operation_events`)
			mustExec(t, h.lp.f.owner, `DROP FUNCTION `+function+`() `)
		})
		if err := h.executor.QueryRow(context.Background(), `SELECT live.finish_media_uncertain($1::uuid,$2::bigint,$3::bytea,'remote_unknown')`, h.plan.OperationID, lease.generation, lease.token).Scan(new(string)); err == nil {
			t.Fatal("finish fault committed")
		}
		if got := h.facts(t); got != before {
			t.Fatalf("failed finish partially committed: before=%+v after=%+v", before, got)
		}
	})
}

func TestLiveMediaExecutionLME06ClosedObservationAndMonotonicProjection(t *testing.T) {
	t.Run("terminal-positive-end-with-absent-start-update", func(t *testing.T) {
		h := lmeSetup(t, func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "direct SQL only", 500) })
		lease := h.claim(t, 30)
		if err := h.reserve(t, lease); err != nil {
			t.Fatal(err)
		}
		out, err := h.record(t, lease, "START", "EG_zero_time", "EGRESS_COMPLETE", 0, 0, 120)
		if err != nil || out != "terminal" {
			t.Fatalf("positive ended with absent starts denied: %s %v", out, err)
		}
		if f := h.facts(t); f.resource != "TERMINAL" || f.operation != "SUCCEEDED" || f.startedNS != 0 || f.updatedNS != 0 || f.endedNS != 120 {
			t.Fatalf("terminal proof/times wrong: %+v", f)
		}
	})
	t.Run("contradictory-partial-merge-retains-history", func(t *testing.T) {
		h := lmeSetup(t, func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "direct SQL only", 500) })
		a := h.claim(t, 30)
		if err := h.reserve(t, a); err != nil {
			t.Fatal(err)
		}
		if out, err := h.record(t, a, "START", "EG_partial", "EGRESS_ACTIVE", 200, 0, 0); err != nil || out != "observe" {
			t.Fatalf("first partial report: %s %v", out, err)
		}
		before := h.facts(t)
		b := h.claim(t, 30)
		if b.mode != "reconcile" {
			t.Fatalf("reconcile lease missing: %+v", b)
		}
		if out, err := h.record(t, b, "QUERY", "EG_partial", "EGRESS_COMPLETE", 0, 150, 140); err != nil || out != "observe" {
			t.Fatalf("valid but merge-incoherent report: %s %v", out, err)
		}
		f := h.facts(t)
		if f.observations != 2 || f.resource == "TERMINAL" || f.operation != "UNKNOWN" || f.startedNS != before.startedNS || f.updatedNS != before.updatedNS || f.endedNS != before.endedNS {
			t.Fatalf("incoherent merged partial fabricated terminal: before=%+v after=%+v", before, f)
		}
	})
	t.Run("coherent-cross-report-duration", func(t *testing.T) {
		h := lmeSetup(t, func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "direct SQL only", 500) })
		a := h.claim(t, 30)
		if err := h.reserve(t, a); err != nil {
			t.Fatal(err)
		}
		if out, err := h.record(t, a, "START", "EG_duration", "EGRESS_ACTIVE", 100, 0, 0); err != nil || out != "observe" {
			t.Fatalf("duration A: %s %v", out, err)
		}
		b := h.claim(t, 30)
		if out, err := h.record(t, b, "QUERY", "EG_duration", "EGRESS_ACTIVE", 0, 901_000_000_100, 0); err != nil || out != "observe" {
			t.Fatalf("duration B: %s %v", out, err)
		}
		f := h.facts(t)
		if !f.cleanup || f.resource != "OBSERVED" || f.operation != "UNKNOWN" || f.startedNS != 100 || f.updatedNS != 901_000_000_100 || f.observations != 2 {
			t.Fatalf("cross-report duration did not set sticky cleanup: %+v", f)
		}
	})
	for _, rows := range []int{0, 2} {
		t.Run(fmt.Sprintf("room-discovery-%d", rows), func(t *testing.T) {
			var h *lmeHarness
			h = lmeSetup(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/twirp/livekit.Egress/ListEgress" {
					http.Error(w, "unexpected", 500)
					return
				}
				if rows == 0 {
					lmeReply(w, `{"items":[]}`)
					return
				}
				lmeReply(w, `{"items":[`+h.observation("EG_one", "EGRESS_ACTIVE", 100, 110, 0)+`,`+h.observation("EG_two", "EGRESS_ACTIVE", 100, 110, 0)+`]}`)
			})
			lease := h.claim(t, 30)
			if err := h.reserve(t, lease); err != nil {
				t.Fatal(err)
			}
			var disposition string
			if err := h.executor.QueryRow(context.Background(), `SELECT live.finish_media_uncertain($1::uuid,$2::bigint,$3::bytea,'remote_unknown')`, h.plan.OperationID, lease.generation, lease.token).Scan(&disposition); err != nil || disposition != "observe" {
				t.Fatalf("uncertain fixture: %s %v", disposition, err)
			}
			h.startWorker(t)
			deadline := time.Now().Add(15 * time.Second)
			for time.Now().Before(deadline) && h.lists.Load() == 0 {
				time.Sleep(20 * time.Millisecond)
			}
			if h.lists.Load() == 0 {
				t.Fatal("reconcile did not list exact room")
			}
			f := h.facts(t)
			if h.starts.Load() != 0 || h.stops.Load() != 0 || f.egress != "" || f.resource == "TERMINAL" || f.operation != "UNKNOWN" {
				t.Fatalf("empty/ambiguous room falsely closed resource: %+v", f)
			}
		})
	}
	for _, tc := range []struct {
		status string
		ended  int64
		want   string
	}{
		{"EGRESS_STARTING", 0, "observe"}, {"EGRESS_ACTIVE", 0, "observe"}, {"EGRESS_ENDING", 0, "observe"},
		{"EGRESS_COMPLETE", 0, "observe"},
		{"EGRESS_COMPLETE", 120, "terminal"}, {"EGRESS_FAILED", 120, "terminal"},
		{"EGRESS_ABORTED", 120, "terminal"}, {"EGRESS_LIMIT_REACHED", 120, "terminal"},
	} {
		t.Run(tc.status, func(t *testing.T) {
			h := lmeSetup(t, func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "direct SQL only", 500) })
			lease := h.claim(t, 30)
			if err := h.reserve(t, lease); err != nil {
				t.Fatal(err)
			}
			out, err := h.record(t, lease, "START", "EG_lme06", tc.status, 100, 130, tc.ended)
			if err != nil || out != tc.want {
				t.Fatalf("%s -> %s %v, want %s", tc.status, out, err, tc.want)
			}
			f := h.facts(t)
			if f.egress != "EG_lme06" || f.status != tc.status || f.observations != 1 || !f.reserved {
				t.Fatalf("bad typed evidence: %+v", f)
			}
			if tc.want == "terminal" {
				wantOp := "FAILED_FINAL"
				if tc.status == "EGRESS_COMPLETE" {
					wantOp = "SUCCEEDED"
				}
				if f.operation != wantOp || f.resource != "TERMINAL" {
					t.Fatalf("terminal proof not durable: %+v", f)
				}
				closed := h.claim(t, 30)
				if closed.disposition != "terminal" || closed.mode != "" || closed.generation != f.generation {
					t.Fatalf("terminal claim leased: %+v", closed)
				}
				before := h.facts(t)
				if _, err := h.record(t, lease, "START", "EG_conflict", "EGRESS_FAILED", 100, 140, 135); err == nil {
					t.Fatal("closed lease accepted conflicting terminal")
				}
				if after := h.facts(t); after != before {
					t.Fatalf("closed terminal changed: %+v -> %+v", before, after)
				}
			} else if f.operation != "UNKNOWN" || f.resource != "OBSERVED" {
				t.Fatalf("nonterminal ACK falsely completed: %+v", f)
			}
			if h.starts.Load()+h.lists.Load()+h.queries.Load()+h.stops.Load() != 0 {
				t.Fatal("direct SQL case made network I/O")
			}
		})
	}
	t.Run("invalid-times-and-duplicate", func(t *testing.T) {
		h := lmeSetup(t, func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "no provider", 500) })
		lease := h.claim(t, 30)
		if err := h.reserve(t, lease); err != nil {
			t.Fatal(err)
		}
		before := h.facts(t)
		for _, tc := range []struct {
			status             string
			start, update, end int64
		}{
			{"EGRESS_ACTIVE", 100, 90, 0}, {"EGRESS_ACTIVE", 100, 120, 110}, {"EGRESS_COMPLETE", 100, 0, 90},
			{"EGRESS_UNKNOWN", 100, 120, 0}, {"EGRESS_ACTIVE", -1, 0, 0},
		} {
			if _, err := h.record(t, lease, "START", "EG_lme06bad", tc.status, tc.start, tc.update, tc.end); err == nil {
				t.Fatalf("bad provider shape accepted: %+v", tc)
			}
			if got := h.facts(t); got != before {
				t.Fatalf("bad provider shape partially wrote: %+v -> %+v", before, got)
			}
		}
		out, err := h.record(t, lease, "START", "EG_lme06good", "EGRESS_ACTIVE", 100, 120, 0)
		if err != nil || out != "observe" {
			t.Fatalf("valid after bad attempts: %s %v", out, err)
		}
		before = h.facts(t)
		if _, err := h.record(t, lease, "START", "EG_lme06good", "EGRESS_ACTIVE", 100, 120, 0); err == nil {
			t.Fatal("stale duplicate completion accepted")
		}
		if got := h.facts(t); got != before {
			t.Fatalf("duplicate changed facts: %+v -> %+v", before, got)
		}
		follow := h.claim(t, 30)
		if follow.mode != "reconcile" {
			t.Fatalf("pinned recovery not reconcile: %+v", follow)
		}
		out, err = h.record(t, follow, "QUERY", "EG_lme06good", "EGRESS_STARTING", 100, 110, 0)
		if err != nil || out != "observe" {
			t.Fatalf("out-of-order report rejected: %s %v", out, err)
		}
		if got := h.facts(t); got.status != "EGRESS_ACTIVE" || got.observations != before.observations+1 {
			t.Fatalf("projection regressed or history lost: %+v", got)
		}
	})
	t.Run("wrong-room-and-ID-collision", func(t *testing.T) {
		first := lmeSetup(t, func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "no provider", 500) })
		a := first.claim(t, 30)
		if err := first.reserve(t, a); err != nil {
			t.Fatal(err)
		}
		if _, err := first.executor.Exec(context.Background(), `SELECT live.record_media_observation($1::uuid,$2::bigint,$3::bytea,'START','EG_collision','lc_00000000000000000000000000000001','EGRESS_ACTIVE',100,110,0)`, first.plan.OperationID, a.generation, a.token); err == nil {
			t.Fatal("wrong room accepted")
		}
		if out, err := first.record(t, a, "START", "EG_collision", "EGRESS_ACTIVE", 100, 110, 0); err != nil || out != "observe" {
			t.Fatalf("first ID pin: %s %v", out, err)
		}
		second := lmeSetup(t, func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "no provider", 500) })
		b := second.claim(t, 30)
		if err := second.reserve(t, b); err != nil {
			t.Fatal(err)
		}
		before := second.facts(t)
		if _, err := second.record(t, b, "START", "EG_collision", "EGRESS_ACTIVE", 100, 110, 0); err == nil {
			t.Fatal("cross-attempt ID adoption accepted")
		}
		if got := second.facts(t); got != before {
			t.Fatalf("collision partially wrote: %+v -> %+v", before, got)
		}
	})
}

func TestLiveMediaExecutionLME07NativeLifecycleReadinessAndUpgrade(t *testing.T) {
	h := lmeSetup(t, func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "no provider", 500) })
	ctx := context.Background()
	var ready bool
	if err := h.lp.f.owner.QueryRow(ctx, `SELECT live.media_worker_ready()`).Scan(&ready); err != nil || !ready {
		t.Fatalf("clean media worker readiness: %t %v", ready, err)
	}
	const readinessFunction = "live.claim_media_operation(uuid,bigint,integer,bytea)"
	mustExec(t, h.lp.f.owner, `GRANT USAGE ON SCHEMA live TO commerce_hosted_runtime`)
	mustExec(t, h.lp.f.owner, `GRANT EXECUTE ON FUNCTION `+readinessFunction+` TO commerce_hosted_runtime`)
	t.Cleanup(func() {
		_, _ = h.lp.f.owner.Exec(context.Background(), `REVOKE EXECUTE ON FUNCTION `+readinessFunction+` FROM commerce_hosted_runtime`)
		_, _ = h.lp.f.owner.Exec(context.Background(), `REVOKE USAGE ON SCHEMA live FROM commerce_hosted_runtime`)
	})
	if err := h.lp.f.owner.QueryRow(ctx, `SELECT live.media_worker_ready()`).Scan(&ready); err != nil || ready {
		t.Fatalf("hosted direct EXECUTE contamination passed readiness: %t %v", ready, err)
	}
	mustExec(t, h.lp.f.owner, `REVOKE EXECUTE ON FUNCTION `+readinessFunction+` FROM commerce_hosted_runtime`)
	mustExec(t, h.lp.f.owner, `REVOKE USAGE ON SCHEMA live FROM commerce_hosted_runtime`)
	if err := h.lp.f.owner.QueryRow(ctx, `SELECT live.media_worker_ready()`).Scan(&ready); err != nil || !ready {
		t.Fatalf("clean readiness not restored after hosted poison: %t %v", ready, err)
	}
	guardTx, err := h.lp.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer guardTx.Rollback(ctx)
	if _, err := guardTx.Exec(ctx, `ALTER TABLE live.media_execution_state DISABLE TRIGGER media_execution_identity`); err != nil {
		t.Fatal(err)
	}
	if err := guardTx.QueryRow(ctx, `SELECT live.media_worker_ready()`).Scan(&ready); err != nil || ready {
		t.Fatalf("disabled custody identity guard passed readiness: %t %v", ready, err)
	}
	if err := guardTx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if err := h.lp.f.owner.QueryRow(ctx, `SELECT live.media_worker_ready()`).Scan(&ready); err != nil || !ready {
		t.Fatalf("guard rollback did not restore readiness: %t %v", ready, err)
	}
	// Preserve LMP's initial INSERT guard; post0007 only admits the dedicated
	// native worker's lifecycle UPDATE of an already-linked job.
	for _, state := range []string{"scheduled", "pending", "completed", "cancelled"} {
		if _, err := h.lp.f.runtime.Exec(ctx, `INSERT INTO river_media.river_job(kind,args,max_attempts,queue,state,scheduled_at,finalized_at)
		 VALUES('live_media_operation_v1',jsonb_build_object('operation_id',$1::text,'version',1),3,'media_mock_v1',$2::river_media.river_job_state,clock_timestamp(),
		 CASE WHEN $2::text IN ('completed','cancelled') THEN clock_timestamp() END)`, randomUUID(), state); sqlState(err) != "22023" {
			t.Fatalf("initial %s INSERT gate weakened: %v", state, err)
		}
	}
	for _, q := range []string{
		`UPDATE river_media.river_job SET state='running' WHERE id=$1`,
		`UPDATE river_media.river_job SET scheduled_at=clock_timestamp()+interval '1 day' WHERE id=$1`,
		`DELETE FROM river_media.river_job WHERE id=$1`,
	} {
		if _, err := h.lp.f.runtime.Exec(ctx, q, h.plan.JobID); err == nil {
			t.Fatalf("merchant runtime mutated native lifecycle: %s", q)
		}
	}
	if _, err := h.worker.Exec(ctx, `SELECT live.claim_media_operation($1::uuid,$2::bigint,30,$3::bytea)`, h.plan.OperationID, h.plan.JobID, randomBytes(32)); err == nil {
		t.Fatal("native River role executed business claim")
	}
	if _, err := h.executor.Exec(ctx, `UPDATE river_media.river_job SET state='running' WHERE id=$1`, h.plan.JobID); err == nil {
		t.Fatal("executor changed native lifecycle")
	}
	fetchTx, err := h.worker.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer fetchTx.Rollback(ctx)
	var fetched int64
	if err := fetchTx.QueryRow(ctx, `SELECT id FROM river_media.river_job WHERE id=$1 AND state='available' FOR UPDATE SKIP LOCKED`, h.plan.JobID).Scan(&fetched); err != nil || fetched != h.plan.JobID {
		t.Fatalf("native worker could not fetch linked job: %d %v", fetched, err)
	}
	if err := fetchTx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	queueTx, err := h.worker.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer queueTx.Rollback(ctx)
	if _, err := queueTx.Exec(ctx, `INSERT INTO river_media.river_queue(name) VALUES('media_mock_v1') ON CONFLICT (name) DO UPDATE SET metadata=excluded.metadata`); err != nil {
		t.Fatalf("dedicated native queue maintenance denied: %v", err)
	}
	if err := queueTx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	for _, schema := range []string{"river_payment", "river_expiry", "river_meta", "river"} {
		if _, err := h.worker.Exec(ctx, `SELECT count(*) FROM `+schema+`.river_job`); err == nil {
			t.Fatalf("media native worker read other River lane %s", schema)
		}
		if _, err := h.worker.Exec(ctx, `UPDATE `+schema+`.river_queue SET metadata=metadata WHERE name='media_mock_v1'`); err == nil {
			t.Fatalf("media native worker maintained other River queue %s", schema)
		}
	}
	// A valid native state transition cannot globally close the producer gate.
	if _, err := h.worker.Exec(ctx, `UPDATE river_media.river_job SET state='running',attempt=1,attempted_at=clock_timestamp() WHERE id=$1`, h.plan.JobID); err != nil {
		t.Fatalf("dedicated native lifecycle denied: %v", err)
	}
	if err := h.lp.f.owner.QueryRow(ctx, `SELECT live.media_plan_ready() AND live.media_worker_ready()`).Scan(&ready); err != nil || !ready {
		t.Fatalf("running linked job disabled readiness: %t %v", ready, err)
	}
	before := lmpMigrationChecksums(t, h.lp.f)
	if err := migrations.Apply(ctx, h.lp.f.owner); err != nil {
		t.Fatalf("repeated Apply: %v", err)
	}
	for version, checksum := range before {
		if lmpMigrationChecksums(t, h.lp.f)[version] != checksum {
			t.Fatalf("historical checksum changed: %s", version)
		}
	}
	t.Run("populated-0035-forward-upgrade", func(t *testing.T) {
		old := lriPre0032Fixture(t)
		mcApplyHistorical(t, old, "0032_legacy_river_isolation.sql", "0033_live_planning.sql", "0034_live_media_authorization.sql", "0035_live_media_plan.sql")
		for _, schema := range []string{"river_payment", "river_expiry", "river_media"} {
			upstream, err := rivermigrate.New(riverpgxv5.New(old.owner), &rivermigrate.Config{Schema: schema, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = upstream.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
				t.Fatal(err)
			}
		}
		mcApplyHistorical(t, old, "post_river/0005_legacy_river_isolation.sql", "post_river/0006_live_media_queue.sql")
		lp := lpHarness{f: old, actor: old.principalA, token: old.tokens["a"]}
		mustExec(t, old.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,'live:read'),($1,$2,$3,'live:manage')`, old.tenantA, old.storeA1, old.principalA)
		draft, err := lpCreate(lp, lp.token, old.storeA1, t04Key("lme-old-draft"), lpInput("historical media draft"))
		if err != nil {
			t.Fatal(err)
		}
		auth := &lmaHarness{lp: lp, session: draft.ID}
		auth.media = auth.binding(t, old.tenantA, old.storeA1, old.principalA, "livekit", "project_lma")
		auth.facebook = auth.binding(t, old.tenantA, old.storeA1, old.principalA, "facebook", "page_lma")
		spec := auth.spec()
		_, registrar := lmaLogin(t, old, "commerce_media_registrar")
		nonce, ciphertext := lmaEnvelope()
		if _, err = lmaRegister(ctx, registrar, spec, nonce, ciphertext); err != nil {
			t.Fatal(err)
		}
		prior := &lmpHarness{lmaHarness: auth, specification: spec,
			input:   live.MediaStartInput{SessionID: draft.ID, AuthorizationID: spec["id"].(string), ExpectedSessionVersion: 1},
			planner: lmpPlanner(t, old.runtime, "river_media")}
		planned, err := prior.start(t04Key("lme-old-plan"))
		if err != nil {
			t.Fatal(err)
		}
		if planned.State != "READY" || planned.JobID <= 0 {
			t.Fatalf("historical 0035 producer not populated: %+v", planned)
		}
		jobBefore := lriRows(t, old, "river_media.river_job", fmt.Sprintf("WHERE id=%d", planned.JobID))
		ledgerBefore := lmpMigrationChecksums(t, old)
		if _, ok := ledgerBefore["0035_live_media_plan.sql"]; !ok {
			t.Fatal("historical 0035 ledger absent")
		}
		if _, ok := ledgerBefore["0036_live_media_execution.sql"]; ok {
			t.Fatal("historical fixture already had execution migration")
		}
		if err = migrations.Apply(ctx, old.owner); err != nil {
			t.Fatalf("populated0035 upgrade: %v", err)
		}
		for version, checksum := range ledgerBefore {
			if lmpMigrationChecksums(t, old)[version] != checksum {
				t.Fatalf("upgrade changed historical checksum %s", version)
			}
		}
		if after := lriRows(t, old, "river_media.river_job", fmt.Sprintf("WHERE id=%d", planned.JobID)); after != jobBefore {
			t.Fatal("upgrade rewrote existing media job")
		}
		if err = old.owner.QueryRow(ctx, `SELECT live.media_plan_ready() AND live.media_worker_ready()`).Scan(&ready); err != nil || !ready {
			t.Fatalf("upgraded populated lane not ready: %t %v", ready, err)
		}
		afterFirst := lmpMigrationChecksums(t, old)
		if err = migrations.Apply(ctx, old.owner); err != nil {
			t.Fatalf("repeat upgraded Apply: %v", err)
		}
		for version, checksum := range afterFirst {
			if lmpMigrationChecksums(t, old)[version] != checksum {
				t.Fatalf("repeat Apply changed %s", version)
			}
		}
	})
}

func TestLiveMediaExecutionLME07NativeMaintenanceIsolation(t *testing.T) {
	ctx := context.Background()
	jobs := make([]*lmeHarness, 4)
	for i := range jobs {
		jobs[i] = lmeSetup(t, func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "paused queue fetched provider", 500) })
	}
	f := jobs[0].lp.f
	ids := miIsoMaintenanceIDs{scheduled: jobs[0].plan.JobID, retryable: jobs[1].plan.JobID,
		stale: jobs[2].plan.JobID, terminal: jobs[3].plan.JobID}
	for _, change := range []struct {
		id  int64
		sql string
	}{
		{ids.scheduled, `UPDATE river_media.river_job SET state='scheduled',scheduled_at=clock_timestamp()-interval '1 second' WHERE id=$1`},
		{ids.retryable, `UPDATE river_media.river_job SET state='retryable',attempt=1,attempted_at=clock_timestamp()-interval '2 hours',scheduled_at=clock_timestamp()-interval '1 second' WHERE id=$1`},
		{ids.stale, `UPDATE river_media.river_job SET state='running',attempt=1,attempted_at=clock_timestamp()-interval '2 hours' WHERE id=$1`},
		{ids.terminal, `UPDATE river_media.river_job SET state='completed',finalized_at=clock_timestamp()-interval '3 days' WHERE id=$1`},
	} {
		mustExec(t, f.owner, change.sql, change.id)
	}
	var priorPausedAt *time.Time
	queueErr := f.owner.QueryRow(ctx, `SELECT paused_at FROM river_media.river_queue WHERE name='media_mock_v1'`).Scan(&priorPausedAt)
	if queueErr != nil && queueErr != pgx.ErrNoRows {
		t.Fatal(queueErr)
	}
	t.Cleanup(func() {
		var err error
		if queueErr == pgx.ErrNoRows {
			_, err = f.owner.Exec(context.Background(), `DELETE FROM river_media.river_queue WHERE name='media_mock_v1'`)
		} else {
			_, err = f.owner.Exec(context.Background(), `UPDATE river_media.river_queue SET paused_at=$1 WHERE name='media_mock_v1'`, priorPausedAt)
		}
		if err != nil {
			t.Errorf("restore native queue pause: %v", err)
		}
	})
	mustExec(t, f.owner, `INSERT INTO river_media.river_queue(name,paused_at) VALUES('media_mock_v1',clock_timestamp()) ON CONFLICT(name) DO UPDATE SET paused_at=excluded.paused_at`)
	var foreignID int64
	if err := f.owner.QueryRow(ctx, `INSERT INTO river.river_job(kind,queue,args,max_attempts,state,attempt,attempted_at,scheduled_at)
	 VALUES('lme_foreign_maintenance','default','{}',3,'retryable',1,clock_timestamp()-interval '2 hours',clock_timestamp()-interval '1 second') RETURNING id`).Scan(&foreignID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = f.owner.Exec(context.Background(), `DELETE FROM river.river_job WHERE id=$1`, foreignID)
	})
	foreignBefore := miIsoRows(t, f, "river.river_job", fmt.Sprintf("WHERE id=%d", foreignID))
	if foreignBefore == "[]" || miIsoMaintained(t, f, "river_media.river_job", ids) {
		t.Fatal("maintenance controls were empty or pre-satisfied")
	}
	jobs[0].startWorker(t)
	miIsoWaitMaintenance(t, f, "river_media.river_job", ids)
	if got := miIsoRows(t, f, "river.river_job", fmt.Sprintf("WHERE id=%d", foreignID)); got != foreignBefore {
		t.Fatalf("media native maintenance crossed old River family: %v", legacyChangedColumns(foreignBefore, got))
	}
	for _, job := range jobs {
		if job.starts.Load()+job.lists.Load()+job.queries.Load()+job.stops.Load() != 0 {
			t.Fatal("paused maintenance made provider call")
		}
	}
}

func TestLiveMediaExecutionLME08EscalationAndNoFallback(t *testing.T) {
	t.Run("generation-ceiling", func(t *testing.T) {
		h := lmeSetup(t, func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "no provider", 500) })
		lease := h.claim(t, 30)
		if err := h.reserve(t, lease); err != nil {
			t.Fatal(err)
		}
		if err := h.executor.QueryRow(context.Background(), `SELECT live.finish_media_uncertain($1::uuid,$2::bigint,$3::bytea,'remote_unknown')`, h.plan.OperationID, lease.generation, lease.token).Scan(new(string)); err != nil {
			t.Fatal(err)
		}
		if _, err := h.lp.f.owner.Exec(context.Background(), `UPDATE integration.operations SET generation=4096 WHERE id=$1`, h.plan.OperationID); err != nil {
			t.Fatal(err)
		}
		out := h.claim(t, 30)
		f := h.facts(t)
		if out.disposition != "escalated" || out.mode != "" || !f.escalated || !f.reserved || f.resource == "TERMINAL" || f.operation != "UNKNOWN" || h.starts.Load() != 0 {
			t.Fatalf("generation ceiling discharged liability: %+v %+v", out, f)
		}
	})
	t.Run("age-ceiling-and-retention", func(t *testing.T) {
		h := lmeSetup(t, func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "no provider", 500) })
		lease := h.claim(t, 30)
		if err := h.reserve(t, lease); err != nil {
			t.Fatal(err)
		}
		if err := h.executor.QueryRow(context.Background(), `SELECT live.finish_media_uncertain($1::uuid,$2::bigint,$3::bytea,'remote_unknown')`, h.plan.OperationID, lease.generation, lease.token).Scan(new(string)); err != nil {
			t.Fatal(err)
		}
		if _, err := h.lp.f.owner.Exec(context.Background(), `UPDATE integration.operations SET created_at=clock_timestamp()-interval '25 hours' WHERE id=$1`, h.plan.OperationID); err != nil {
			t.Fatal(err)
		}
		out := h.claim(t, 30)
		f := h.facts(t)
		if out.disposition != "escalated" || out.mode != "" || !f.escalated || !f.reserved || f.resource == "TERMINAL" || f.operation != "UNKNOWN" || h.starts.Load() != 0 {
			t.Fatalf("age exhausted but liability discharged: claim=%+v facts=%+v", out, f)
		}
		if _, err := h.lp.f.owner.Exec(context.Background(), `UPDATE river_media.river_job SET state='completed',finalized_at=clock_timestamp() WHERE id=$1`, h.plan.JobID); err != nil {
			t.Fatal(err)
		}
		if _, err := h.lp.f.owner.Exec(context.Background(), `DELETE FROM river_media.river_job WHERE id=$1`, h.plan.JobID); err != nil {
			t.Fatal(err)
		}
		var ready bool
		if err := h.lp.f.owner.QueryRow(context.Background(), `SELECT live.media_plan_ready() AND live.media_worker_ready()`).Scan(&ready); err != nil || !ready {
			t.Fatalf("durably escalated retention globally blocked future planning: %t %v", ready, err)
		}
	})
	t.Run("wrong-credential-no-fallback", func(t *testing.T) {
		h := lmeSetup(t, func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "no provider", 500) })
		wrong := h.project
		wrong.CredentialVersion = 2
		client, err := live.NewMediaClient(context.Background(), h.worker, h.executor, h.keys, []live.MediaProject{wrong}, 1)
		if err != nil {
			t.Fatal(err)
		}
		if err = client.Start(context.Background()); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = client.StopAndCancel(ctx)
		})
		h.await(t, 15*time.Second, func(f lmeFacts) bool {
			return f.generation > 0 && f.operation == "UNKNOWN" && !f.leaseOpen && f.events >= 2
		})
		if f := h.facts(t); f.reserved || f.resource == "TERMINAL" || h.starts.Load()+h.lists.Load()+h.queries.Load()+h.stops.Load() != 0 {
			t.Fatalf("credential fallback or false terminal: %+v", f)
		}
	})
}
