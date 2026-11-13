package foundation_test

// Responsibility: MRR07 public mixed observer regressions against the existing
// REAL_PG fixture. Dependencies: BRW/MRR admission helpers and in-memory LiveKit
// reads. Invariants: SQL readback qualifies proof; no lifecycle RPC or fabricated
// lease/receipt; neither a provider reply nor checked means episode completion.
import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/live"
)

type mrr07RPC struct {
	mu                                    sync.Mutex
	room, identity                        string
	inputMode                             string
	egressUnavailable                     bool
	participant, rooms, egress, forbidden int
}

type mrr07Counts struct{ participant, rooms, egress, forbidden int }

func (p *mrr07RPC) counts() mrr07Counts {
	p.mu.Lock()
	defer p.mu.Unlock()
	return mrr07Counts{p.participant, p.rooms, p.egress, p.forbidden}
}
func (p *mrr07RPC) set(input string, egressUnavailable bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.inputMode, p.egressUnavailable = input, egressUnavailable
}
func (p *mrr07RPC) RoundTrip(r *http.Request) (*http.Response, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	var request map[string]json.RawMessage
	if r.Method != http.MethodPost || r.URL.Host != "unit.livekit.cloud" || json.NewDecoder(r.Body).Decode(&request) != nil {
		p.forbidden++
		return nil, fmt.Errorf("invalid observer RPC envelope")
	}
	text := func(key string) string { var v string; _ = json.Unmarshal(request[key], &v); return v }
	status, body := http.StatusOK, ""
	switch r.URL.Path {
	case "/twirp/livekit.RoomService/GetParticipant":
		p.participant++
		if text("room") != p.room || text("identity") != p.identity {
			p.forbidden++
			return nil, fmt.Errorf("wrong participant target")
		}
		if p.inputMode == "unavailable" {
			return nil, fmt.Errorf("synthetic participant unavailable")
		}
		identity := p.identity
		if p.inputMode == "wrong_target" {
			identity = "publisher_wrong_target"
		}
		b, _ := json.Marshal(map[string]any{"identity": identity, "sid": "PA_mrr07", "state": "ACTIVE"})
		body = string(b)
	case "/twirp/livekit.RoomService/ListRooms":
		p.rooms++
		var names []string
		if json.Unmarshal(request["names"], &names) != nil || len(names) != 1 || names[0] != p.room {
			p.forbidden++
			return nil, fmt.Errorf("wrong room target")
		}
		b, _ := json.Marshal(map[string]any{"rooms": []any{map[string]any{"name": p.room, "sid": "RM_mrr07"}}})
		body = string(b)
	case "/twirp/livekit.Egress/ListEgress":
		p.egress++
		if text("room_name") != p.room || text("egress_id") != "" {
			p.forbidden++
			return nil, fmt.Errorf("wrong egress target")
		}
		if p.egressUnavailable {
			return nil, fmt.Errorf("synthetic egress unavailable")
		}
		b, _ := json.Marshal(map[string]any{"items": []any{map[string]any{"egress_id": "EG_mrr07", "room_name": p.room, "status": "EGRESS_ACTIVE", "started_at": "100", "updated_at": "120", "ended_at": "0"}}})
		body = string(b)
	default:
		// Strict allowlist catches Start, Stop, Remove, Delete and every other RPC.
		p.forbidden++
		return nil, fmt.Errorf("forbidden observer RPC")
	}
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
}

type mrr07Fixture struct {
	h        *brwHarness
	ctx      context.Context
	pool     *pgxpool.Pool
	observer *live.MediaRecoveryObserver
	rpc      *mrr07RPC
	episode  string
	member   live.RecoveryMember
}

func mrr07Observer(t *testing.T, ctx context.Context, pool *pgxpool.Pool, rpc *mrr07RPC, missingInput bool) *live.MediaRecoveryObserver {
	t.Helper()
	inputProject := "project_lma"
	projects := []live.MediaProject{{ProjectID: "project_lma", CredentialVersion: 1, Config: lmeConfig(), Transport: rpc}}
	if missingInput {
		inputProject = "project_mrr07_other"
		projects = append(projects, live.MediaProject{ProjectID: inputProject, CredentialVersion: 1, Config: lmeConfig(), Transport: rpc})
	}
	runtime, err := live.NewBrowserInputRuntime([]live.BrowserInputProject{{ProjectID: inputProject, CredentialVersion: 1, Config: lmeConfig(), Transport: rpc, BrowserURL: "ws://127.0.0.1:7880"}})
	if err != nil {
		t.Fatalf("public runtime: %v", err)
	}
	before := rpc.counts()
	observer, err := live.NewMediaRecoveryObserverWithBrowserInput(ctx, pool, projects, runtime)
	if rpc.counts() != before {
		t.Fatal("constructor performed provider RPC")
	}
	if err != nil || observer == nil {
		t.Fatalf("public mixed constructor: %v", err)
	}
	return observer
}

func mrr07Setup(t *testing.T, both, missingInput bool) *mrr07Fixture {
	t.Helper()
	h := brwStart(t)
	brwReserveGrant(t, h)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	t.Cleanup(cancel)
	if both {
		lease, err := brwClaim(ctx, h.bicHarness, h.plan, randomBytes(32))
		if err != nil || lease.disposition != "claimed" || lease.mode != "reconcile" {
			t.Fatalf("normal claim: %v", err)
		}
		brwSelectObserve(t, h, lease)
		material, err := brwReserveStart(ctx, h, lease, h.plan.RoomName, h.grant.PublisherIdentity, "PA_mrr07", "ACTIVE", true, false, true, false)
		if err != nil || material["mode"] != "dispatch" {
			t.Fatalf("normal wire admission: %v", err)
		}
		material = nil
		// Wait for the original real dispatch lease. No clock/lease edits or claim
		// token injection. This runs only in the later separately authorized PG phase.
		deadline := time.Now().Add(40 * time.Second)
		for {
			var expired bool
			if err := h.lp.f.owner.QueryRow(ctx, `SELECT lease_until IS NOT NULL AND lease_until<=clock_timestamp() FROM integration.operations WHERE id=$1::uuid AND generation=$2`, h.plan.OperationID, lease.generation).Scan(&expired); err != nil {
				t.Fatal(err)
			}
			if expired {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("original dispatch lease did not expire")
			}
			select {
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			case <-time.After(100 * time.Millisecond):
			}
		}
	}
	login, pool := lmaLogin(t, h.lp.f, "commerce_media_recovery")
	role := pgx.Identifier{login}.Sanitize()
	mustExec(t, h.lp.f.owner, "REVOKE commerce_media_recovery FROM "+role)
	mustExec(t, h.lp.f.owner, "GRANT commerce_media_recovery TO "+role+" WITH INHERIT TRUE, SET FALSE")
	rpc := &mrr07RPC{room: h.plan.RoomName, identity: h.grant.PublisherIdentity, inputMode: "healthy"}
	observer := mrr07Observer(t, ctx, pool, rpc, missingInput)
	// Same valid maps must fail on the existing application-role pool, without I/O.
	runtime, err := live.NewBrowserInputRuntime([]live.BrowserInputProject{{ProjectID: "project_lma", CredentialVersion: 1, Config: lmeConfig(), Transport: rpc, BrowserURL: "ws://127.0.0.1:7880"}})
	if err != nil {
		t.Fatal(err)
	}
	bad, err := live.NewMediaRecoveryObserverWithBrowserInput(ctx, h.lp.f.runtime, []live.MediaProject{{ProjectID: "project_lma", CredentialVersion: 1, Config: lmeConfig(), Transport: rpc}}, runtime)
	if err != live.ErrMediaRecovery || bad != nil || rpc.counts() != (mrr07Counts{}) {
		t.Fatalf("wrong role admission err=%v", err)
	}
	episode := randomUUID()
	mrr05Cleanup(t, h, episode)
	members, err := observer.Begin(ctx, episode, 0, 1, true)
	if err != nil || len(members) != 1 {
		t.Fatalf("public Begin: %+v %v", members, err)
	}
	m := members[0]
	var generation int64
	if err := h.lp.f.owner.QueryRow(ctx, `SELECT generation FROM integration.operations WHERE id=$1::uuid`, h.plan.OperationID).Scan(&generation); err != nil {
		t.Fatal(err)
	}
	if m.Disposition != "pending" || m.EpisodeID != episode || m.OperationID != h.plan.OperationID || m.JobID != h.plan.JobID || m.BaselineGeneration != generation || m.ExecutionProfile != "LOCAL_SFU_MOCK_EGRESS" || !m.InputRequired || m.EgressRequired != both || m.CandidateCount != 1 || !m.CoverageKnown || m.BlockedByEpisodeID != "" || m.DeadlineAt.IsZero() {
		t.Fatalf("admission: %+v", m)
	}
	f := &mrr07Fixture{h, ctx, pool, observer, rpc, episode, m}
	r := f.read(t)
	if r.InputObservationID != "" || r.ObservationID != "" {
		t.Fatal("fixture preseeded recovery proof")
	}
	if _, err := live.NewMediaRecoveryLedgerWithBrowserInput(pool).Observe(ctx, episode, m); err != live.ErrMediaRecovery {
		t.Fatalf("diagnostic Observe admitted: %v", err)
	}
	if rpc.counts() != (mrr07Counts{}) {
		t.Fatal("admission performed provider RPC")
	}
	return f
}

func (f *mrr07Fixture) read(t *testing.T) live.RecoveryReadback {
	t.Helper()
	rows, err := f.observer.Read(f.ctx, f.episode)
	if err != nil || len(rows) != 1 {
		t.Fatalf("public Read: %+v %v", rows, err)
	}
	r, m := rows[0], f.member
	if r.EpisodeID != m.EpisodeID || r.OperationID != m.OperationID || r.JobID != m.JobID || r.BaselineGeneration != m.BaselineGeneration || r.ExecutionProfile != m.ExecutionProfile || r.InputRequired != m.InputRequired || r.EgressRequired != m.EgressRequired || r.CandidateCount != 1 || !r.CoverageKnown || r.BlockedByEpisodeID != "" || !r.TimeoutAt.IsZero() || r.CleanupRequired {
		t.Fatalf("readback identity/masks: %+v", r)
	}
	return r
}
func (f *mrr07Fixture) observe(t *testing.T, want string) {
	t.Helper()
	got, err := f.observer.Observe(f.ctx, f.episode, f.member)
	if err != nil || got != want {
		t.Fatalf("Observe=%s want=%s err=%v", got, want, err)
	}
}
func (f *mrr07Fixture) inputProof(t *testing.T, r live.RecoveryReadback) {
	t.Helper()
	if r.InputObservationID == "" || r.InputObservationSource != "PARTICIPANT" || r.InputObservationResult != "PRESENT" || r.InputObservationGeneration <= f.member.BaselineGeneration {
		t.Fatalf("missing committed input proof: %+v", r)
	}
}
func (f *mrr07Fixture) egressProof(t *testing.T, r live.RecoveryReadback) {
	t.Helper()
	if r.ObservationID == "" || r.ObservationSource != "ROOM" || r.ObservationGeneration <= f.member.BaselineGeneration {
		t.Fatalf("missing committed egress proof: %+v", r)
	}
}
func (f *mrr07Fixture) counts(t *testing.T, want mrr07Counts) {
	t.Helper()
	if got := f.rpc.counts(); got != want {
		t.Fatalf("RPC counts=%+v want=%+v", got, want)
	}
}
func (f *mrr07Fixture) witness(t *testing.T, r live.RecoveryReadback, want string) {
	t.Helper()
	got, err := f.observer.WitnessWithInput(f.ctx, f.episode, f.member.OperationID, r.InputObservationID, r.ObservationID, 1000)
	if err != nil || got != want {
		t.Fatalf("public witness=%s want=%s err=%v", got, want, err)
	}
}

func TestLiveMediaRecoveryMRR07ObserverInputOnly(t *testing.T) {
	f := mrr07Setup(t, false, false)
	f.observe(t, "checked")
	first := f.read(t)
	f.inputProof(t, first)
	if first.ObservationID != "" || first.ObservationGeneration != 0 || first.ObservationSource != "" || first.ScopeStatus != "pending" || first.Disposition != "checked" {
		t.Fatalf("input-only endstate: %+v", first)
	}
	f.counts(t, mrr07Counts{participant: 1})
	f.observe(t, "already_observed")
	if again := f.read(t); again != first {
		t.Fatalf("durable replay changed readback: %+v", again)
	}
	f.counts(t, mrr07Counts{participant: 1})
	if _, err := f.observer.Witness(f.ctx, f.episode, f.member.OperationID, first.InputObservationID, 1000); err != live.ErrMediaRecovery {
		t.Fatalf("legacy Witness bypass: %v", err)
	}
	f.witness(t, first, "witnessed")
	if done := f.read(t); done.ScopeStatus != "finished" || done.Disposition != "witnessed" {
		t.Fatalf("qualified endstate: %+v", done)
	}
}

func TestLiveMediaRecoveryMRR07ObserverMissingInputStillEgress(t *testing.T) {
	f := mrr07Setup(t, true, true)
	f.observe(t, "partial")
	first := f.read(t)
	f.egressProof(t, first)
	if first.InputObservationID != "" || first.InputObservationGeneration != 0 || first.InputObservationSource != "" || first.InputObservationResult != "" || first.ScopeStatus != "pending" {
		t.Fatalf("missing input liability lost: %+v", first)
	}
	f.counts(t, mrr07Counts{egress: 1})
	f.witness(t, first, "unqualified")
	// Replace only immutable public configuration; durable episode/member unchanged.
	f.observer = mrr07Observer(t, f.ctx, f.pool, f.rpc, false)
	f.observe(t, "checked")
	second := f.read(t)
	f.inputProof(t, second)
	f.egressProof(t, second)
	if second.ObservationID != first.ObservationID || second.ObservationGeneration != first.ObservationGeneration || second.InputObservationGeneration <= first.ObservationGeneration {
		t.Fatalf("resume lost egress proof: %+v", second)
	}
	f.observe(t, "already_observed")
	f.counts(t, mrr07Counts{participant: 1, egress: 1})
}

func TestLiveMediaRecoveryMRR07ObserverPartialResume(t *testing.T) {
	f := mrr07Setup(t, true, false)
	f.rpc.set("healthy", true)
	f.observe(t, "partial")
	first := f.read(t)
	f.inputProof(t, first)
	if first.ObservationID != "" || first.ScopeStatus != "pending" {
		t.Fatalf("unexpected complete proof: %+v", first)
	}
	f.counts(t, mrr07Counts{participant: 1, egress: 1})
	// Observe must have called real fenced Finish. Inspect, never forge the token.
	var generation int64
	var released bool
	if err := f.h.lp.f.owner.QueryRow(f.ctx, `SELECT generation,lease_until IS NULL AND lease_token_hash IS NULL AND lease_mode='' FROM integration.operations WHERE id=$1::uuid`, f.member.OperationID).Scan(&generation, &released); err != nil || !released || generation != first.InputObservationGeneration {
		t.Fatalf("real Finish generation=%d released=%v err=%v", generation, released, err)
	}
	f.rpc.set("healthy", false)
	f.observe(t, "checked")
	second := f.read(t)
	f.inputProof(t, second)
	f.egressProof(t, second)
	if second.InputObservationID != first.InputObservationID || second.InputObservationGeneration != first.InputObservationGeneration || second.ObservationGeneration <= generation {
		t.Fatalf("durable partial resume: %+v", second)
	}
	f.counts(t, mrr07Counts{participant: 1, egress: 2})
	f.observe(t, "already_observed")
	if third := f.read(t); third != second {
		t.Fatalf("complete replay changed proof: %+v", third)
	}
	f.counts(t, mrr07Counts{participant: 1, egress: 2})
	f.witness(t, second, "witnessed")
	if done := f.read(t); done.ScopeStatus != "finished" {
		t.Fatalf("endstate: %+v", done)
	}
	// This is durable readback/resume, not commit-ACK-loss injection.
}

func TestLiveMediaRecoveryMRR07ObserverRoomPresentNoInputProof(t *testing.T) {
	for _, mode := range []string{"unavailable", "wrong_target"} {
		t.Run(mode, func(t *testing.T) {
			f := mrr07Setup(t, true, false)
			f.rpc.set(mode, false)
			f.observe(t, "partial")
			first := f.read(t)
			f.egressProof(t, first)
			if first.InputObservationID != "" || first.InputObservationSource != "" || first.InputObservationResult != "" || first.InputObservationGeneration != 0 || first.ScopeStatus != "pending" {
				t.Fatalf("ROOM PRESENT fabricated input proof: %+v", first)
			}
			f.counts(t, mrr07Counts{participant: 1, rooms: 1, egress: 1})
			f.witness(t, first, "unqualified")
			f.rpc.set("healthy", false)
			f.observe(t, "checked")
			second := f.read(t)
			f.inputProof(t, second)
			f.egressProof(t, second)
			if second.ObservationID != first.ObservationID || second.ObservationGeneration != first.ObservationGeneration || second.InputObservationGeneration <= first.ObservationGeneration {
				t.Fatalf("input retry changed egress: %+v", second)
			}
			f.observe(t, "already_observed")
			f.counts(t, mrr07Counts{participant: 2, rooms: 1, egress: 1})
			f.witness(t, second, "witnessed")
			if done := f.read(t); done.ScopeStatus != "finished" {
				t.Fatalf("endstate: %+v", done)
			}
		})
	}
}
