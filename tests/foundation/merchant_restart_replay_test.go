//go:build browser

// Purpose: prove a successful idempotent merchant write, replayed after a real API process restart, returns its stored
// receipt from ops.command_results without touching durable state, and a same-key/different-body replay is refused.
// Depends on: the built cmd/api binary; pwIsolatedFixture (own PG container, so full-table snapshots are exact);
// POST /v1/admin/stores/{store}/warehouses -> inventory.CreateWarehouse -> command.Run/command.Audit;
// browserLog/browserEnvironment (browser_identity_chain_test.go); assertAdminError (admin_transport_test.go).
// Used by: scripts/dev/test-local.sh --browser-identity.
// Invariant: the second process never re-executes the write; every row of warehouses, command_results and
// audit_events for the store is byte-identical before and after both replays.
package foundation_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestMerchantWriteReplayAfterAPIRestart(t *testing.T) {
	if os.Getenv("LC_BROWSER_IDENTITY_ACCEPTANCE") != "1" || os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" {
		t.Fatal("use scripts/dev/test-local.sh --browser-identity; isolated fixtures are required")
	}
	f := pwIsolatedFixture(t)
	mustExec(t, f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission)
 SELECT $1,$2,$3,p FROM unnest(ARRAY['inventory:read','inventory:write']) p ON CONFLICT DO NOTHING`, f.tenantA, f.storeA1, f.principalA)
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	evidence := filepath.Join(root, "output", "playwright", "restart-replay-"+time.Now().UTC().Format("20060102T150405.000000000"))
	if err := os.MkdirAll(evidence, 0700); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(evidence, "api")
	buildLog := browserLog(t, filepath.Join(evidence, "build.log"))
	buildCtx, cancelBuild := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancelBuild()
	build := exec.CommandContext(buildCtx, "go", "build", "-o", binary, "./cmd/api")
	build.Dir = root
	build.Env = browserEnvironment(nil)
	build.Stdout, build.Stderr = buildLog, buildLog
	if err := build.Run(); err != nil {
		t.Fatalf("build API failed; see %s: %v", buildLog.Name(), err)
	}
	var origin string // the API's bound address, set by start from its api_ready line
	dsn := f.runtime.Config().ConnString()
	childEnv := browserEnvironment(map[string]string{"LISTEN_ADDR": mrEphemeralAddr, "DATABASE_URL": dsn})
	// Registered before any process starts so the scan also runs when an assertion fails midway.
	t.Cleanup(func() {
		secrets := []string{f.tokens["a"], dsn, f.databaseURL}
		if pw := f.runtime.Config().ConnConfig.Password; pw != "" {
			secrets = append(secrets, pw)
		}
		for _, logName := range []string{"first.log", "second.log"} {
			logBytes, err := os.ReadFile(filepath.Join(evidence, logName))
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				t.Error(err)
				continue
			}
			for _, secret := range secrets {
				if strings.Contains(string(logBytes), secret) {
					t.Error("API process log contained sensitive fixture material")
				}
			}
		}
	})
	client := &http.Client{Timeout: 5 * time.Second}
	start := func(name string) (*exec.Cmd, chan error) {
		t.Helper()
		log := browserLog(t, filepath.Join(evidence, name+".log"))
		cmd := exec.Command(binary)
		cmd.Dir, cmd.Env = root, childEnv
		cmd.Stdout, cmd.Stderr = log, log
		if err := cmd.Start(); err != nil {
			t.Fatalf("start API %s: %v", name, err)
		}
		done := make(chan error, 1)
		go func() {
			done <- cmd.Wait()
			close(done)
		}()
		t.Cleanup(func() {
			select {
			case <-done:
				return
			default:
			}
			_ = cmd.Process.Kill()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Errorf("API PID %d did not exit during cleanup", cmd.Process.Pid)
			}
		})
		deadline := time.Now().Add(15 * time.Second)
		origin = "http://" + mrBoundAddr(t, &mrProcess{done: done, logPath: log.Name()}, time.Until(deadline))
		for time.Now().Before(deadline) {
			select {
			case err := <-done:
				t.Fatalf("API %s exited before readiness; see %s: %v", name, log.Name(), err)
			default:
			}
			resp, err := client.Get(origin + "/healthz")
			if err == nil {
				_ = resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					return cmd, done
				}
			}
			time.Sleep(50 * time.Millisecond)
		}
		t.Fatalf("API %s readiness timed out; see %s", name, log.Name())
		return nil, nil
	}
	stop := func(cmd *exec.Cmd, done chan error) {
		t.Helper()
		if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
			t.Fatalf("signal API PID %d: %v", cmd.Process.Pid, err)
		}
		select {
		case err := <-done:
			if err != nil || cmd.ProcessState == nil || !cmd.ProcessState.Success() {
				t.Fatalf("API PID %d did not exit cleanly: %v", cmd.Process.Pid, err)
			}
		case <-time.After(15 * time.Second):
			_ = cmd.Process.Kill()
			<-done
			t.Fatalf("API PID %d did not stop on SIGTERM", cmd.Process.Pid)
		}
	}
	request := func(method, path, key string, body []byte) *httptest.ResponseRecorder {
		t.Helper()
		req, err := http.NewRequest(method, origin+path, bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+f.tokens["a"])
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("API request %s failed: %v", method, err)
		}
		defer resp.Body.Close()
		w := httptest.NewRecorder()
		w.Code = resp.StatusCode
		w.HeaderMap = resp.Header.Clone()
		if _, err := io.Copy(w.Body, io.LimitReader(resp.Body, 1<<20)); err != nil {
			t.Fatal(err)
		}
		return w
	}
	// Full rows, not counts: a replay that rewrote response/created_at or re-audited would still keep counts equal.
	snapshot := func() string {
		t.Helper()
		var out string
		if err := f.owner.QueryRow(context.Background(), `SELECT jsonb_build_object(
 'warehouses', (SELECT coalesce(jsonb_agg(to_jsonb(w) ORDER BY w.id),'[]') FROM inventory.warehouses w WHERE w.tenant_id=$1 AND w.store_id=$2),
 'command_results', (SELECT coalesce(jsonb_agg(to_jsonb(c) ORDER BY c.operation,c.idempotency_key),'[]') FROM ops.command_results c WHERE c.tenant_id=$1 AND c.store_id=$2),
 'audit_events', (SELECT coalesce(jsonb_agg(to_jsonb(a) ORDER BY a.id),'[]') FROM ops.audit_events a WHERE a.tenant_id=$1 AND a.store_id=$2))::text`,
			f.tenantA, f.storeA1).Scan(&out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	path := "/v1/admin/stores/" + f.storeA1 + "/warehouses"
	name := "restart-replay-" + t04Tag()
	key, body := t04Key("restart-replay"), []byte(`{"name":"`+name+`"}`)

	firstProcess, firstDone := start("first")
	firstPID := firstProcess.Process.Pid
	first := request("POST", path, key, body)
	if first.Code != http.StatusOK || !strings.Contains(first.Body.String(), name) {
		t.Fatalf("first write status=%d body=%s", first.Code, first.Body.String())
	}
	afterWrite := snapshot()
	var warehouses, receipts, audits int
	if err := f.owner.QueryRow(context.Background(), `SELECT
 (SELECT count(*) FROM inventory.warehouses WHERE tenant_id=$1 AND store_id=$2 AND name=$3),
 (SELECT count(*) FROM ops.command_results WHERE tenant_id=$1 AND store_id=$2 AND operation='inventory.warehouse.create' AND idempotency_key=$4),
 (SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action='inventory.warehouse.created')`,
		f.tenantA, f.storeA1, name, key).Scan(&warehouses, &receipts, &audits); err != nil {
		t.Fatal(err)
	}
	if warehouses != 1 || receipts != 1 || audits != 1 {
		t.Fatalf("first write persisted warehouses=%d receipts=%d audits=%d, want 1/1/1", warehouses, receipts, audits)
	}
	stop(firstProcess, firstDone)

	secondProcess, secondDone := start("second")
	if secondProcess.Process.Pid == firstPID {
		t.Fatal("API PID did not change across restart")
	}
	replay := request("POST", path, key, body)
	if replay.Code != first.Code || !bytes.Equal(replay.Body.Bytes(), first.Body.Bytes()) {
		t.Fatalf("replay after restart: status=%d body=%s, want status=%d body=%s", replay.Code, replay.Body.String(), first.Code, first.Body.String())
	}
	if got := snapshot(); got != afterWrite {
		t.Fatalf("replay after restart changed durable state:\nbefore=%s\nafter=%s", afterWrite, got)
	}
	// The route maps command.ErrConflict (stored request_hash mismatch) to 409 "conflict" (internal/httpapi/handler.go).
	assertAdminError(t, request("POST", path, key, []byte(`{"name":"`+name+`-other"}`)), http.StatusConflict, "conflict", f.tokens["a"])
	if got := snapshot(); got != afterWrite {
		t.Fatalf("conflicting replay changed durable state:\nbefore=%s\nafter=%s", afterWrite, got)
	}
	if list := request("GET", path+"?limit=50", "", nil); list.Code != http.StatusOK || strings.Count(list.Body.String(), `"`+name+`"`) != 1 {
		t.Fatalf("list after replays: status=%d body=%s", list.Code, list.Body.String())
	}
	stop(secondProcess, secondDone)
	t.Logf("PASS: actual API PIDs %d -> %d, same isolated PG; evidence=%s", firstPID, secondProcess.Process.Pid, evidence)
}
