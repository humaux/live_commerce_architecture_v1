// Purpose: #107 (PR #96 review P2-3) — run()'s bridge-listener wiring in the real claims-worker binary: an occupied
// COMMERCE_CLAIMS_CONSOLE_ADDR fails startup with claims_worker_start_failed and never logs claims_worker_ready (#83).
// Also: a Serve failure AFTER the bind (the test-tag build closes the bound listener, cmd/claims-worker/console_testhook.go)
// must end run() with claims_worker_console_failed (never start_failed, never a hang) — the main.go wiring of
// superviseConsole/cancelRun/exitError (#116 P2-1).
// CW-2 (#147): the hook closes the bound listener only after claims_worker_ready is written (console_testhook.go), so this test
// pins the post-ready path; a bridge loss logged before readiness would exercise only the startup path (#116 P2-1).
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
	// P2 (PR #168 review): the pre-PR exit envelope was fourteen seconds from launch; address discovery spends from it instead of
	// adding an independent eight-second window before the exit wait.
	deadline := time.Now().Add(14 * time.Second)
	mrConsoleBoundAddr(t, p, time.Until(deadline))
	mrFailsBeforeReadyWithin(t, p, time.Until(deadline), "claims_worker_console_failed", linkKey, pageKey, w.jobDSNSecret)
	log := crReadLog(t, p)
	if strings.Contains(log, "claims_worker_start_failed") {
		t.Fatal("bridge loss after the bind was reported as a startup failure")
	}
	readyAt, failedAt := strings.Index(log, "claims_worker_ready"), strings.Index(log, "claims_worker_console_failed")
	if readyAt < 0 {
		t.Fatalf("bridge loss was never preceded by claims_worker_ready; the post-ready path is unpinned: %s", p.logPath)
	}
	if failedAt < 0 || readyAt > failedAt {
		t.Fatalf("claims_worker_ready must be logged before claims_worker_console_failed: %s", p.logPath)
	}
}
