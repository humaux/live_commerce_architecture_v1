// Purpose: #107 (PR #96 review P2-3) — run()'s bridge-listener wiring in the real claims-worker binary: an occupied
// COMMERCE_CLAIMS_CONSOLE_ADDR fails startup with claims_worker_start_failed and never logs claims_worker_ready (#83).
// Also: a Serve failure AFTER the bind (the test-tag build closes the bound listener, cmd/claims-worker/console_testhook.go)
// must end run() with claims_worker_console_failed (never start_failed, never a hang) — the main.go wiring of
// superviseConsole/cancelRun/exitError (#116 P2-1).
// CW-2 (#147) is deliberately NOT pinned: whether claims_worker_ready precedes the console failure is a race between the hook's 1s
// timer and River Start's DB probe (jobqueue.Run skips the ready line once ctx is cancelled), so asserting it would flake on a loaded runner.
// CMD-2 (#147): the serve-failure worker binds a loopback :0 (mrEphemeralAddr) and the test reads the OS-chosen port from the
// claims_worker_console_ready line (mrConsoleBoundAddr); the occupied-address case still holds its own port because the collision is the fixture.
// Depends on: crNewWorkerEnv/crReadLog (claims_retention_worker_test.go), mrFixture/mrBuild/mrLaunch/mrFailsBeforeReady/
// mrEphemeralAddr/mrConsoleBoundAddr (meta_runtime*_test.go), REAL_PG; cmd/claims-worker.
// Used by: go test ./tests/foundation (Foundation suite and the foundation shards).

package foundation_test

import (
	"encoding/base64"
	"net"
	"strings"
	"testing"
	"time"
)

func TestClaimsWorkerConsoleBindFailureNeverReady(t *testing.T) {
	f := mrFixture(t)
	w := crNewWorkerEnv(t, f)
	held, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	linkKey := base64.StdEncoding.EncodeToString(w.linkRaw)
	pageKey := base64.StdEncoding.EncodeToString(w.pageKeyRaw)
	env := append(append([]string(nil), w.env...), "COMMERCE_CLAIMS_CONSOLE_ADDR="+held.Addr().String(),
		"COMMERCE_CLAIMS_BRIDGE_TOKEN="+linkKey, "COMMERCE_CLAIMS_CURSOR_KEY="+pageKey)
	p := mrLaunch(t, mrBuild(t, "../../cmd/claims-worker", "claims-console-bind"), "claims-console-bind", env)
	mrFailsBeforeReady(t, p, "claims_worker_start_failed", linkKey, pageKey, w.jobDSNSecret)
	if log := crReadLog(t, p); strings.Contains(log, "claims_worker_ready") || strings.Contains(log, "claims_worker_console_ready") {
		t.Fatal("worker reported readiness with its bridge address unavailable")
	}
}

func TestClaimsWorkerConsoleServeFailureExitsWithConsoleToken(t *testing.T) {
	f := mrFixture(t)
	w := crNewWorkerEnv(t, f)
	linkKey := base64.StdEncoding.EncodeToString(w.linkRaw)
	pageKey := base64.StdEncoding.EncodeToString(w.pageKeyRaw)
	env := append(append([]string(nil), w.env...), "COMMERCE_CLAIMS_CONSOLE_ADDR="+mrEphemeralAddr,
		"COMMERCE_CLAIMS_BRIDGE_TOKEN="+linkKey, "COMMERCE_CLAIMS_CURSOR_KEY="+pageKey)
	bin := mrBuild(t, "../../cmd/claims-worker", "claims-console-serve-failure", "-tags", "claimsconsoletest")
	p := mrLaunch(t, bin, "claims-console-serve-failure", env)
	mrConsoleBoundAddr(t, p, 8*time.Second)
	mrFailsBeforeReady(t, p, "claims_worker_console_failed", linkKey, pageKey, w.jobDSNSecret)
	if strings.Contains(crReadLog(t, p), "claims_worker_start_failed") {
		t.Fatal("bridge loss after the bind was reported as a startup failure")
	}
}
