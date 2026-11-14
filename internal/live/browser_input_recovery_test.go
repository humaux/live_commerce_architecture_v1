// Responsibility: DB-free mixed observer regressions.
// Dependencies: real LiveKit decoder with memory-only HTTP transport.
// Callers: focused Go test; invariants: no provider mutations or native claims.

package live

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/integrations/livekit"
)

type recoveryTransport struct {
	calls             []string
	deadlines         []time.Time
	participant, room string
}

func (r *recoveryTransport) RoundTrip(q *http.Request) (*http.Response, error) {
	r.calls = append(r.calls, q.URL.Path)
	d, _ := q.Context().Deadline()
	r.deadlines = append(r.deadlines, d)
	body := r.room
	if strings.HasSuffix(q.URL.Path, "GetParticipant") {
		body = r.participant
	}
	if strings.HasSuffix(q.URL.Path, "ListEgress") {
		body = `{"items":[{"egress_id":"EG_test_1","room_name":"lc_0123456789abcdef0123456789abcdef","status":"EGRESS_ACTIVE","started_at":"123","updated_at":"456","ended_at":"0"}]}`
	}
	return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: q}, nil
}

const recoveryRoom = "lc_0123456789abcdef0123456789abcdef"
const recoveryPublisher = "lcp_0123456789abcdef0123456789abcdef"

func recoveryFixture(t *testing.T, rt *recoveryTransport) (*livekit.Client, MediaProject, *BrowserInputRuntime) {
	t.Helper()
	cfg := livekit.Config{Environment: "MOCK", Endpoint: "https://test-project.livekit.cloud", APIKey: "test_key_1", APISecret: strings.Repeat("S", 32), StreamHosts: []string{"stream.example.com"}}
	c, e := livekit.New(cfg, rt)
	if e != nil {
		t.Fatal(e)
	}
	p := MediaProject{ProjectID: "project_1", CredentialVersion: 1, Config: cfg, Transport: rt}
	r, e := NewBrowserInputRuntime([]BrowserInputProject{{ProjectID: p.ProjectID, CredentialVersion: 1, Config: cfg, Transport: rt, BrowserURL: "ws://127.0.0.1:7880"}})
	if e != nil {
		t.Fatal(e)
	}
	return c, p, r
}
func TestBrowserInputRecoveryObserverModeIsolation(t *testing.T) {
	for _, r := range []*MediaRecoveryObserver{nil, NewMediaRecoveryLedger(nil), NewMediaRecoveryLedgerWithBrowserInput(new(pgxpool.Pool))} {
		if _, e := r.Observe(context.Background(), "episode", RecoveryMember{Disposition: "pending", OperationID: "op", JobID: 1}); !errors.Is(e, ErrMediaRecovery) {
			t.Fatal(e)
		}
	}
	if r, e := NewMediaRecoveryObserverWithBrowserInput(nil, nil, nil, nil); r != nil || !errors.Is(e, ErrMediaRecovery) {
		t.Fatal("nil constructor admitted")
	}
}
func TestBrowserInputRecoveryObserverConfiguration(t *testing.T) {
	rt := &recoveryTransport{}
	_, p, r := recoveryFixture(t, rt)
	for _, tc := range []struct {
		p  []MediaProject
		r  *BrowserInputRuntime
		ok bool
	}{
		{[]MediaProject{p}, r, true}, {nil, r, false}, {[]MediaProject{p, p}, r, false}, {[]MediaProject{p}, nil, false},
	} {
		_, _, e := recoveryProjectMaps(tc.p, tc.r)
		if (e == nil) != tc.ok {
			t.Fatalf("config result %v", e)
		}
	}
	bad := p
	bad.CredentialVersion = 2
	if _, _, e := recoveryProjectMaps([]MediaProject{bad}, r); e == nil {
		t.Fatal("unmatched version")
	}
	bad = p
	bad.Config.Endpoint = "https://other.livekit.cloud"
	if _, _, e := recoveryProjectMaps([]MediaProject{bad}, r); e == nil {
		t.Fatal("unmatched endpoint")
	}
	if len(rt.calls) != 0 {
		t.Fatal("configuration performed I/O")
	}
}
func TestBrowserInputRecoveryObserverIndependentSides(t *testing.T) {
	rt := &recoveryTransport{participant: `{"identity":"` + recoveryPublisher + `","sid":"PA_one"}`}
	c, p, input := recoveryFixture(t, rt)
	key := mediaProjectKey{p.ProjectID, 1}
	for _, tc := range []struct {
		name                                         string
		media, input, inReq, egReq, inProof, egProof bool
		want                                         int
	}{
		{"input missing", true, false, true, true, false, false, 1},
		{"egress missing", false, true, true, true, false, false, 1},
		{"input only", false, true, true, false, false, false, 1},
		{"both proven", true, true, true, true, true, true, 0},
		{"input proven", true, false, true, true, true, false, 1},
		{"egress proven", false, true, true, true, false, true, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rt.calls = nil
			r := &MediaRecoveryObserver{projects: map[mediaProjectKey]mediaEndpoint{}}
			if tc.media {
				r.projects[key] = mediaEndpoint{identity: p.Config.Endpoint, client: c}
			}
			if tc.input {
				r.input = input
			}
			media, in, mo, ino := r.recoveryEndpoints(key, p.Config.Endpoint)
			a, b := recoveryMissingSides(tc.inReq, tc.egReq, sql.NullString{Valid: tc.inProof}, sql.NullString{Valid: tc.egProof})
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if a && ino {
				if _, _, _, _, e := observeRecoveryInput(ctx, in.client, recoveryRoom, recoveryPublisher); e != nil {
					t.Fatal(e)
				}
			}
			if b && mo {
				if _, _, e := observeRecoveryEgress(ctx, media.client, recoveryRoom, "EG_test_1"); e != nil {
					t.Fatal(e)
				}
			}
			if len(rt.calls) != tc.want {
				t.Fatalf("calls %v", rt.calls)
			}
			if _, _, mo, ino := r.recoveryEndpoints(key, "wrong"); mo || ino {
				t.Fatal("target mismatch admitted")
			}
		})
	}
}
func TestBrowserInputRecoveryObserverFallbackAndSharedDeadline(t *testing.T) {
	for _, tc := range []struct {
		name, participant, room string
		absent                  bool
	}{
		{"present room", `{}`, `{"rooms":[{"name":"` + recoveryRoom + `","sid":"RM_one"}]}`, false},
		{"absent room", `{}`, `{"rooms":[]}`, true},
		{"wrong target", `{"identity":"lcp_ffffffffffffffffffffffffffffffff","sid":"PA_one"}`, `{"rooms":[{"name":"` + recoveryRoom + `","sid":"RM_one"}]}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rt := &recoveryTransport{participant: tc.participant, room: tc.room}
			c, _, _ := recoveryFixture(t, rt)
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			source, result, _, _, e := observeRecoveryInput(ctx, c, recoveryRoom, recoveryPublisher)
			if tc.absent {
				if e != nil || source != "ROOM" || result != "ABSENT" {
					t.Fatal(source, result, e)
				}
			} else if e == nil {
				t.Fatal("unqualified input accepted")
			}
			if _, _, e := observeRecoveryEgress(ctx, c, recoveryRoom, "EG_test_1"); e != nil {
				t.Fatal(e)
			}
			if len(rt.calls) != 3 {
				t.Fatal(rt.calls)
			}
			for _, d := range rt.deadlines {
				if !d.Equal(rt.deadlines[0]) {
					t.Fatal("provider budget reset")
				}
			}
		})
	}
}
func TestBrowserInputRecoveryObserverDeadline(t *testing.T) {
	rt := &recoveryTransport{}
	c, _, _ := recoveryFixture(t, rt)
	deadline := time.Now().Add(-time.Second)
	ctx, cancel := recoveryTurn(context.Background(), deadline)
	defer cancel()
	if ctx.Err() == nil {
		t.Fatal("expired episode allowed")
	}
	observeRecoveryInput(ctx, c, recoveryRoom, recoveryPublisher)
	observeRecoveryEgress(ctx, c, recoveryRoom, "EG_test_1")
	if len(rt.calls) != 0 {
		t.Fatal("expired provider calls")
	}
	d := time.Now().Add(time.Second)
	ctx2, cancel2 := recoveryTurn(context.Background(), d)
	defer cancel2()
	got, _ := ctx2.Deadline()
	if !got.Equal(d) {
		t.Fatal("episode deadline extended")
	}
}

func TestBrowserInputRecoveryObserverLegacyWitnessControls(t *testing.T) {
	pool := new(pgxpool.Pool)
	if _, err := NewMediaRecoveryLedgerWithBrowserInput(pool).Witness(context.Background(), "", "", "", 0); !errors.Is(err, ErrMediaRecovery) {
		t.Fatal(err)
	}
	if _, err := NewMediaRecoveryLedger(pool).WitnessWithInput(context.Background(), "", "", "", "", 0); !errors.Is(err, ErrMediaRecovery) {
		t.Fatal(err)
	}
}
func TestBrowserInputRecoveryObserverCallerDeadlineAndBinding(t *testing.T) {
	caller, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second))
	defer cancel()
	turn, stop := recoveryTurn(caller, time.Now().Add(time.Minute))
	defer stop()
	a, _ := caller.Deadline()
	b, _ := turn.Deadline()
	if !a.Equal(b) {
		t.Fatal("caller deadline extended")
	}
	r := &MediaRecoveryObserver{pool: new(pgxpool.Pool), browserInput: true, providerAdmitted: true}
	m := RecoveryMember{EpisodeID: "other", Disposition: "pending", OperationID: "op", JobID: 1, InputRequired: true, ExecutionProfile: "LOCAL_SFU_MOCK_EGRESS", DeadlineAt: time.Now().Add(time.Second)}
	if _, err := r.Observe(context.Background(), "episode", m); !errors.Is(err, ErrMediaRecovery) {
		t.Fatal("episode mismatch")
	}
	m.EpisodeID = "episode"
	m.DeadlineAt = time.Now().Add(-time.Second)
	if status, err := r.Observe(context.Background(), "episode", m); err != nil || status != "overdue" {
		t.Fatal(status, err)
	}
}
