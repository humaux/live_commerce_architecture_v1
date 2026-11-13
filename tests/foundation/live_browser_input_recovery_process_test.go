// Responsibility: real native parent diagnostic admission with invalid input.
// Existing PG/process fixtures only; no real-SFU or SIGKILL recovery claim.
package foundation_test

import (
	"context"
	"net/http"
	"os/exec"
	"strconv"
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
