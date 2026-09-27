package foundation_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/live"
)

// Reuse LMR's PostgreSQL CommandComplete+ReadyForQuery('I') proof. Only the
// one-shot SQL selector differs: a committed cleanup result loses its ACK.
type brwCleanupAckConn struct {
	*lmrAckConn
	seen *atomic.Bool
}

func (c *brwCleanupAckConn) Write(p []byte) (int, error) {
	n, err := c.Conn.Write(p)
	if err == nil && n == len(p) && len(p) > 6 && p[0] == 'Q' &&
		int(binary.BigEndian.Uint32(p[1:5])) == len(p)-1 && p[len(p)-1] == 0 &&
		bytes.HasPrefix(bytes.ToLower(bytes.TrimSpace(p[5:len(p)-1])),
			[]byte("select live.record_media_input_cleanup(")) && c.seen.CompareAndSwap(false, true) {
		c.pending.Store(true)
	}
	return n, err
}

func brwCleanupAckPool(t *testing.T, base *pgxpool.Pool, seen, committed *atomic.Bool) *pgxpool.Pool {
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
		return &brwCleanupAckConn{lmrAckConn: &lmrAckConn{
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

type brwCleanupProvider struct {
	remove, deleteRoom, participant, listRooms atomic.Int32
	bad                                        atomic.Int32
	cutoff                                     atomic.Int64
}

func (p *brwCleanupProvider) serve(room, publisher string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fail := func() { p.bad.Add(1); http.Error(w, "wrong cleanup target", http.StatusBadRequest) }
		switch r.URL.Path {
		case "/twirp/livekit.RoomService/RemoveParticipant":
			var got struct {
				Room          string `json:"room"`
				Identity      string `json:"identity"`
				RevokeTokenTS string `json:"revoke_token_ts"`
			}
			if json.NewDecoder(r.Body).Decode(&got) != nil || got.Room != room || got.Identity != publisher {
				fail()
				return
			}
			cutoff, err := strconv.ParseInt(got.RevokeTokenTS, 10, 64)
			if err != nil || cutoff <= 0 || cutoff > time.Now().Unix() || time.Now().Unix()-cutoff > 30 {
				fail()
				return
			}
			p.cutoff.Store(cutoff)
			p.remove.Add(1)
			lmeReply(w, `{}`)
		case "/twirp/livekit.RoomService/DeleteRoom":
			var got struct{ Room string }
			if json.NewDecoder(r.Body).Decode(&got) != nil || got.Room != room {
				fail()
				return
			}
			p.deleteRoom.Add(1)
			lmeReply(w, `{}`)
		case "/twirp/livekit.RoomService/GetParticipant":
			var got struct{ Room, Identity string }
			if json.NewDecoder(r.Body).Decode(&got) != nil || got.Room != room || got.Identity != publisher {
				fail()
				return
			}
			p.participant.Add(1)
			http.Error(w, "not observed", http.StatusNotFound)
		case "/twirp/livekit.RoomService/ListRooms":
			var got struct{ Names []string }
			if json.NewDecoder(r.Body).Decode(&got) != nil || len(got.Names) != 1 || got.Names[0] != room {
				fail()
				return
			}
			p.listRooms.Add(1)
			lmeReply(w, fmt.Sprintf(`{"rooms":[{"name":%q,"sid":"RM_brw"}]}`, room))
		default:
			fail()
		}
	}
}

func brwCleanupWireFixture(t *testing.T) (*brwWireFixture, *brwCleanupProvider) {
	t.Helper()
	f := brwWireStarted(t)
	brwReserveGrant(t, f.h)
	if out, err := bicStop(context.Background(), f.h.bicHarness, f.h.logins.b,
		t04Key("brw-cleanup-fault-stop"), f.h.plan); err != nil || out.State != "requested" {
		t.Fatalf("close issued input: %+v %v", out, err)
	}
	p := &brwCleanupProvider{}
	server := httptest.NewTLSServer(p.serve(f.h.plan.RoomName, f.h.grant.PublisherIdentity))
	t.Cleanup(server.Close)
	config := lmeConfig()
	config.APIKey = "brw_sfu_test_key"
	config.APISecret = strings.Repeat("i", 40)
	var err error
	f.h.runtime, err = live.NewBrowserInputRuntime([]live.BrowserInputProject{{
		ProjectID: "project_lma", CredentialVersion: 1, Config: config,
		Transport: lmeTransport(server.Listener.Addr().String()), BrowserURL: "wss://127.0.0.1:7880",
	}})
	if err != nil {
		t.Fatal(err)
	}
	return f, p
}

type brwCleanupFaultStep struct {
	ordinal    int
	action     string
	result     string
	generation int64
}

func brwCleanupFaultFacts(t *testing.T, f *brwWireFixture) ([]brwCleanupFaultStep, string, *time.Time, string, *time.Time, int64) {
	t.Helper()
	rows, err := f.h.lp.f.owner.Query(context.Background(), `SELECT ordinal,action,result,generation
		FROM live.media_input_wire_steps WHERE attempt_id=$1 ORDER BY ordinal`, f.h.plan.AttemptID)
	if err != nil {
		t.Fatal(err)
	}
	var steps []brwCleanupFaultStep
	for rows.Next() {
		var step brwCleanupFaultStep
		if err := rows.Scan(&step.ordinal, &step.action, &step.result, &step.generation); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		steps = append(steps, step)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		t.Fatal(err)
	}
	rows.Close()
	var custody, jobState string
	var held, finalized *time.Time
	var jobID int64
	if err := f.h.lp.f.owner.QueryRow(context.Background(), `SELECT c.state,c.input_cleanup_held_at,
		j.state,j.finalized_at,o.job_id FROM integration.operations o
		JOIN live.media_input_custody c ON c.operation_id=o.id
		JOIN river_media.river_job j ON j.id=o.job_id WHERE o.id=$1`, f.h.plan.OperationID).
		Scan(&custody, &held, &jobState, &finalized, &jobID); err != nil {
		t.Fatal(err)
	}
	return steps, custody, held, jobState, finalized, jobID
}

func brwAssertCleanupFault(t *testing.T, f *brwWireFixture, p *brwCleanupProvider,
	wantFirst string) {
	t.Helper()
	steps, custody, held, jobState, finalized, jobID := brwCleanupFaultFacts(t, f)
	wantActions := []string{"REMOVE", "DELETE_ROOM", "READ_PARTICIPANT", "READ_ROOM"}
	wantResults := []string{wantFirst, "ACK", "UNKNOWN", "PRESENT"}
	if len(steps) != 4 {
		t.Fatalf("cleanup count=%d steps=%+v", len(steps), steps)
	}
	for i, step := range steps {
		if step.ordinal != i+1 || step.action != wantActions[i] || step.result != wantResults[i] || step.generation < 1 {
			t.Fatalf("cleanup ordinal %d: %+v want %s/%s", i+1, step, wantActions[i], wantResults[i])
		}
	}
	if custody == "CLOSED" || held == nil || jobID != f.h.plan.JobID || finalized != nil ||
		jobState == "completed" || jobState == "cancelled" || jobState == "discarded" ||
		p.remove.Load() != 1 || p.deleteRoom.Load() != 1 || p.participant.Load() != 1 ||
		p.listRooms.Load() != 1 || p.bad.Load() != 0 || p.cutoff.Load() <= 0 || f.starts.Load() != 0 {
		t.Fatalf("unsafe cleanup: custody=%s held=%v job=%d/%s finalized=%v calls=%d/%d/%d/%d bad=%d cutoff=%d start=%d",
			custody, held, jobID, jobState, finalized, p.remove.Load(), p.deleteRoom.Load(),
			p.participant.Load(), p.listRooms.Load(), p.bad.Load(), p.cutoff.Load(), f.starts.Load())
	}
}

func TestLiveBrowserInputBRW03CommittedCleanupResultAckLoss(t *testing.T) {
	f, provider := brwCleanupWireFixture(t)
	seen, committed := &atomic.Bool{}, &atomic.Bool{}
	fault := brwCleanupAckPool(t, f.h.executor, seen, committed)
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
			t.Errorf("browser worker stop: %v", err)
		}
	})
	deadline := time.Now().Add(75 * time.Second)
	for time.Now().Before(deadline) {
		steps, _, held, _, _, _ := brwCleanupFaultFacts(t, f)
		if committed.Load() && held != nil && len(steps) == 4 {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if !seen.Load() || !committed.Load() {
		t.Fatalf("record cleanup COMMIT ACK not dropped: seen=%t committed=%t", seen.Load(), committed.Load())
	}
	brwAssertCleanupFault(t, f, provider, "ACK")
}
