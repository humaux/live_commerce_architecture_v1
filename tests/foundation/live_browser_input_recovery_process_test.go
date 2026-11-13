// Responsibility: real native parent diagnostic admission with invalid input.
// Existing PG/process fixtures only; no real-SFU or SIGKILL recovery claim.
package foundation_test

import (
	"context"
	"encoding/json"
	"livecommerce/internal/live"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestLiveBrowserInputProcessInvalidConfigDiagnostic(t *testing.T) {
	binary := mrBuild(t, "../../cmd/media-worker", "input-process-diagnostic")
	for _, flag := range []string{"1", "invalid"} {
		t.Run(flag, func(t *testing.T) {
			h := lmeSetup(t, func(w http.ResponseWriter, _ *http.Request) {
				t.Error("invalid config made provider call")
				http.Error(w, "unexpected", 500)
			})
			recovery, _ := mrrRecoveryPool(t, h)
			env := lmwEnvironment(h, "127.0.0.1:1", "invalid-ca")
			env = lmwReplace(env, "COMMERCE_MEDIA_BROWSER_INPUT_ENABLED", flag)
			env = lmwReplace(env, "COMMERCE_MEDIA_PROJECTS_JSON", "malformed-projects")
			env = append(env, "COMMERCE_MEDIA_RECOVERY_SUPERVISED=1", "COMMERCE_MEDIA_RECOVERY_DATABASE_URL="+recovery.Config().ConnString())
			launched := time.Now()
			parent := lmwProcess(t, binary, "input-invalid-"+flag, env)
			mrReadyLog(t, parent, "media_recovery_native_config_invalid")
			episode := mrrWaitEpisode(t, h.lp.f.owner, launched)
			t.Cleanup(func() {
				if !parent.exited {
					mrStop(t, parent, syscall.SIGTERM, true)
				}
				// This case creates only its own zero-member diagnostic scope.
				if _, err := h.lp.f.owner.Exec(context.Background(), `DELETE FROM live.media_recovery_episode_scope WHERE episode_id=$1::uuid AND candidate_count=0`, episode); err != nil {
					t.Errorf("diagnostic scope cleanup: %v", err)
				}
			})
			var mixed, known bool
			var status string
			var capture int64
			var deadline time.Time
			err := h.lp.f.owner.QueryRow(context.Background(), `SELECT include_browser_input,coverage_known,scope_status,capture_elapsed_ms,deadline_at FROM live.media_recovery_episode_scope WHERE episode_id=$1::uuid`, episode).Scan(&mixed, &known, &status, &capture, &deadline)
			if err != nil || !mixed || known || status == "finished" || capture < 0 || deadline.Before(launched.Add(80*time.Second)) {
				t.Fatalf("invalid input fell back or lost original timer: mixed=%v known=%v status=%s capture=%d deadline=%v err=%v", mixed, known, status, capture, deadline, err)
			}
			// Read process inventory only; this does not emulate a child or provider.
			child, err := exec.Command("pgrep", "-P", strconv.Itoa(parent.cmd.Process.Pid)).Output()
			if err == nil || len(child) != 0 {
				t.Fatal("invalid config launched a child")
			}
			if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
				t.Fatalf("cannot verify child absence: %v", err)
			}
			mrStop(t, parent, syscall.SIGTERM, true)
		})
	}
}

// The blocked Egress is an explicit MOCK fault. Input absence must come from
// the pinned native SFU, and the parent must preserve its original real clock.
func TestLiveBrowserInputProcessNativeMixedRecovery(t *testing.T) {
	sfu := brpNativeSFU(t)
	binary := mrBuild(t, "../../cmd/media-worker", "native-mixed-recovery")
	input := brwStart(t)
	brwReserveGrant(t, input)
	h := lmeSetup(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("in-process route used")
		http.Error(w, "unused", 500)
	})
	mrrReserveUnansweredStart(t, h) // real 30-second lease, never aged or reset
	var mu sync.Mutex
	var calls []time.Time
	server, ca := lmwTLS(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		brpEgressAuth(t, r)
		var q struct {
			RoomName string `json:"room_name"`
			EgressID string `json:"egress_id"`
		}
		if r.TLS == nil || r.Host != "unit.livekit.cloud" || r.URL.Path != "/twirp/livekit.Egress/ListEgress" || json.NewDecoder(r.Body).Decode(&q) != nil || q.RoomName != h.plan.RoomName || q.EgressID != "" {
			t.Error("recovery Egress wrong route/target or mutation")
			http.Error(w, "target", 400)
			return
		}
		mu.Lock()
		calls = append(calls, time.Now())
		mu.Unlock()
		http.Error(w, "fixture unavailable", 503)
	}))
	env := brpEnvironment(t, h, input, sfu, server.Listener.Addr().String(), ca)
	recovery, _ := mrrRecoveryPool(t, h)
	ledger := live.NewMediaRecoveryLedgerWithBrowserInput(recovery)
	launched := time.Now()
	p := lmwProcess(t, binary, "native-mixed-recovery", env)
	t.Cleanup(func() {
		if !p.exited {
			mrStop(t, p, syscall.SIGTERM, true)
		}
	})
	episode := mrrWaitEpisode(t, h.lp.f.owner, launched)
	mrr05Cleanup(t, input, episode)
	t.Cleanup(func() {
		if !p.exited {
			mrStop(t, p, syscall.SIGTERM, true)
		}
	})
	var deadline time.Time
	var capture int64
	if err := h.lp.f.owner.QueryRow(context.Background(), `SELECT deadline_at,capture_elapsed_ms FROM live.media_recovery_episode_scope WHERE episode_id=$1::uuid`, episode).Scan(&deadline, &capture); err != nil {
		t.Fatal(err)
	}
	if capture < 0 || deadline.Before(launched.Add(80*time.Second)) {
		t.Fatal("episode pre-aged")
	}
	child, err := mrrChildPID(p.cmd.Process.Pid, 0)
	if err != nil {
		t.Fatal(err)
	}
	brpReadyCount(t, p, 1)
	killed := time.Now()
	if err := syscall.Kill(child, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	replacement, err := mrrChildPID(p.cmd.Process.Pid, child)
	if err != nil {
		t.Fatal(err)
	}
	brpReadyCount(t, p, 2)
	// Freeze only the replacement child so the observer's post-deadline I/O can
	// be measured independently from native cleanup. Parent remains running.
	if err := syscall.Kill(replacement, syscall.SIGSTOP); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = syscall.Kill(replacement, syscall.SIGCONT) })
	t.Logf("PROCESS SIGKILL old_child=%d at=%s replacement=%d episode=%s deadline=%s capture_ms=%d", child, killed.UTC().Format(time.RFC3339Nano), replacement, episode, deadline.UTC().Format(time.RFC3339Nano), capture)
	var inputProof live.RecoveryReadback
	var rows []live.RecoveryReadback
	for time.Now().Before(deadline.Add(6 * time.Second)) {
		rows, err = ledger.Read(context.Background(), episode)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 2 {
			t.Fatalf("mixed membership=%d want=2", len(rows))
		}
		timedOut := false
		for _, r := range rows {
			if r.EpisodeID != episode || !r.CoverageKnown || r.CandidateCount != 2 {
				t.Fatal("mixed scope changed")
			}
			switch r.OperationID {
			case input.plan.OperationID:
				if r.JobID != input.plan.JobID || r.ExecutionProfile != "LOCAL_SFU_MOCK_EGRESS" || !r.InputRequired || r.EgressRequired || r.ObservationID != "" {
					t.Fatal("input original/mask mismatch")
				}
				if r.InputObservationID != "" {
					inputProof = r
				}
			case h.plan.OperationID:
				if r.JobID != h.plan.JobID || r.ExecutionProfile != "PROVIDER_MOCK" || r.InputRequired || !r.EgressRequired || r.InputObservationID != "" || r.ObservationID != "" {
					t.Fatal("Egress fault fabricated proof")
				}
				timedOut = !r.TimeoutAt.IsZero()
			default:
				t.Fatal("foreign operation admitted")
			}
		}
		if timedOut {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if inputProof.InputObservationID == "" || inputProof.InputObservationSource != "ROOM" || inputProof.InputObservationResult != "ABSENT" || inputProof.InputObservationGeneration <= inputProof.BaselineGeneration {
		t.Fatal("no independent real SFU absence receipt")
	}
	var timeout time.Time
	var afterDeadline time.Time
	if err := h.lp.f.owner.QueryRow(context.Background(), `SELECT timeout_at,deadline_at FROM live.media_recovery_episode_scope WHERE episode_id=$1::uuid`, episode).Scan(&timeout, &afterDeadline); err != nil {
		t.Fatal(err)
	}
	if !afterDeadline.Equal(deadline) || timeout.Before(deadline) || time.Since(launched) < 90*time.Second {
		t.Fatal("original real 90s clock lost")
	}
	mrReadyLog(t, p, "media_recovery_deadline_missed")
	// Observe a real post-deadline interval, then independently read persisted SQL.
	time.Sleep(2 * time.Second)
	after, err := ledger.Read(context.Background(), episode)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(rows) {
		t.Fatal("sticky membership changed")
	}
	for i := range rows {
		if rows[i] != after[i] {
			t.Fatal("late proof changed committed recovery readback")
		}
	}
	sfu.mu.Lock()
	inputCalls := append([]time.Time(nil), sfu.calls...)
	participant, rooms := sfu.participant, sfu.rooms
	sfu.mu.Unlock()
	mu.Lock()
	egressCalls := append([]time.Time(nil), calls...)
	mu.Unlock()
	if participant < 1 || rooms < 1 || len(egressCalls) < 1 {
		t.Fatal("missing real SFU or mock Egress observation")
	}
	for _, at := range append(inputCalls, egressCalls...) {
		if at.After(deadline) {
			t.Fatal("new provider I/O after original deadline")
		}
	}
	var count int
	if err := h.lp.f.owner.QueryRow(context.Background(), `SELECT count(*) FROM live.media_recovery_episode_scope WHERE created_at >= $1`, launched).Scan(&count); err != nil || count != 1 {
		t.Fatalf("child crash reset episode count=%d err=%v", count, err)
	}
	log, err := os.ReadFile(p.logPath)
	if err != nil || !strings.Contains(string(log), "media_recovery_timeout_persisted") {
		t.Fatal("sticky timeout log missing")
	}
	t.Logf("REAL_PG SQL episode=%s input_receipt=%s generation=%d baseline=%d result=ROOM/ABSENT Egress=UNKNOWN timeout=%s original_jobs=%d/%d input_RPC=%d egress_RPC=%d post90_new_IO=0", episode, inputProof.InputObservationID, inputProof.InputObservationGeneration, inputProof.BaselineGeneration, timeout.UTC().Format(time.RFC3339Nano), input.plan.JobID, h.plan.JobID, len(inputCalls), len(egressCalls))
	_ = syscall.Kill(replacement, syscall.SIGCONT)
	mrStop(t, p, syscall.SIGTERM, true)
}
