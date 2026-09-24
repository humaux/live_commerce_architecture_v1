//go:build browser

package foundation_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"livecommerce/internal/integrations/accounts"
	"livecommerce/internal/pagination"
)

// This gate deliberately restarts the built API executable. A fresh handler
// using the same in-memory service would not prove key reload or PG recovery.
func TestMerchantAccountAPIProcessRestart(t *testing.T) {
	if os.Getenv("LC_BROWSER_IDENTITY_ACCEPTANCE") != "1" || os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" {
		t.Fatal("use scripts/dev/test-local.sh --browser-identity; isolated fixtures are required")
	}
	m := maSetup(t)
	_, _, authority := identityFixture(t)
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	evidence := filepath.Join(root, "output", "playwright", "account-process-"+time.Now().UTC().Format("20060102T150405.000000000"))
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
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	origin := "http://" + addr
	idp := newBrowserIDP(t, origin+"/api/auth/callback")
	keyEntries := []map[string]string{}
	for _, id := range []string{"fixture_v1", "fixture_v2"} {
		keyEntries = append(keyEntries, map[string]string{"id": id, "key_base64": base64.StdEncoding.EncodeToString(m.keys[id])})
	}
	keyJSON, err := json.Marshal(keyEntries)
	if err != nil {
		t.Fatal(err)
	}
	childEnv := browserEnvironment(map[string]string{
		"LISTEN_ADDR":                            addr,
		"DATABASE_URL":                           m.f.base.runtime.Config().ConnString(),
		"COMMERCE_IDENTITY_ENABLED":              "1",
		"COMMERCE_IDENTITY_ALLOW_LOOPBACK_TESTS": "1",
		"COMMERCE_PUBLIC_ORIGIN":                 origin,
		"COMMERCE_IDENTITY_DATABASE_URL":         authority.Config().ConnString(),
		"COMMERCE_BFF_KEY":                       randomToken(),
		"COMMERCE_OIDC_ISSUER":                   idp.server.URL,
		"COMMERCE_OIDC_CLIENT_ID":                browserClientID,
		"COMMERCE_IDENTITY_PROVIDER_KEY":         "account-process-signed-mock-v1",
		"COMMERCE_SESSION_TTL":                   "1h",
		"COMMERCE_ONBOARDING_ENABLED":            "1",
		"COMMERCE_ONBOARDING_CURRENCIES":         "TWD,USD",
		"COMMERCE_ACCOUNTS_ENABLED":              "1",
		"COMMERCE_ACCOUNT_ACTIVE_KEY_ID":         "fixture_v1",
		"COMMERCE_ACCOUNT_KEYS_JSON":             string(keyJSON),
		"COMMERCE_ACCOUNT_REPLAY_KEY":            base64.StdEncoding.EncodeToString(m.replayKey),
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
		go func() { done <- cmd.Wait() }()
		t.Cleanup(func() {
			if cmd.ProcessState == nil {
				_ = cmd.Process.Kill()
				select {
				case <-done:
				case <-time.After(5 * time.Second):
				}
			}
		})
		deadline := time.Now().Add(15 * time.Second)
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
		req.Header.Set("Authorization", "Bearer "+m.f.token)
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
	path, in, key := mahPath(m.f.store), maInput(), t04Key("account-process")
	body := mahBody(t, in)
	before := m.facts(t)
	firstProcess, firstDone := start("first")
	firstPID := firstProcess.Process.Pid
	first := mahRead(t, request("POST", path, key, body), in.Credentials)
	if first.CredentialVersion != 1 || first.State != "CONFIGURED_UNVERIFIED" || first.KeyID != "" {
		t.Fatal("first process returned unsafe account metadata")
	}
	maAssertPersistedSecret(t, m, first, 1, in.Credentials)
	afterCreate := m.facts(t)
	if afterCreate[0] != before[0]+1 || afterCreate[1] != before[1]+1 || afterCreate[2] != before[2]+1 || afterCreate[5] != before[5] || afterCreate[6] != before[6] {
		t.Fatal("create did not persist exactly one account or created provider work")
	}
	stop(firstProcess, firstDone)
	secondProcess, secondDone := start("second")
	if secondProcess.Process.Pid == firstPID {
		t.Fatal("API PID did not change across restart")
	}
	got := mahRead(t, request("GET", path+"/"+first.ID, "", nil), in.Credentials)
	if !reflect.DeepEqual(got, first) {
		t.Fatal("new process could not read persisted account")
	}
	page := settingsRead[pagination.Page[accounts.Connection]](t, request("GET", path+"?limit=10", "", nil))
	if len(page.Items) != 1 || !reflect.DeepEqual(page.Items[0], first) || page.NextCursor != "" {
		t.Fatal("new process list did not return safe persisted metadata")
	}
	if replay := mahRead(t, request("POST", path, key, body), in.Credentials); !reflect.DeepEqual(replay, first) || m.facts(t) != afterCreate {
		t.Fatal("replay after restart changed response or durable state")
	}
	rotatedSecret := accounts.Credentials{HashKey: strings.Repeat("N", 32), HashIV: strings.Repeat("W", 16)}
	rotation := settingsBody(t, map[string]any{"expected_version": 1, "credentials": map[string]string{"hash_key": rotatedSecret.HashKey, "hash_iv": rotatedSecret.HashIV}})
	rotated := mahRead(t, request("POST", path+"/"+first.ID+"/rotate", t04Key("account-process-rotate"), rotation), rotatedSecret)
	if rotated.CredentialVersion != 2 || rotated.ID != first.ID || rotated.BindingID != first.BindingID || rotated.State != first.State {
		t.Fatal("rotation after restart changed account identity or state")
	}
	maAssertPersistedSecret(t, m, first, 1, in.Credentials)
	maAssertPersistedSecret(t, m, rotated, 2, rotatedSecret)
	afterRotate := m.facts(t)
	if afterRotate[2] != afterCreate[2]+1 || afterRotate[5] != before[5] || afterRotate[6] != before[6] {
		t.Fatal("rotation did not persist exactly one revision or created provider work")
	}
	stop(secondProcess, secondDone)
	for _, name := range []string{"first.log", "second.log"} {
		logBytes, err := os.ReadFile(filepath.Join(evidence, name))
		if err != nil {
			t.Fatal(err)
		}
		for _, secret := range []string{m.f.token, in.Credentials.HashKey, in.Credentials.HashIV, rotatedSecret.HashKey, rotatedSecret.HashIV, string(keyJSON), m.f.base.runtime.Config().ConnString()} {
			if strings.Contains(string(logBytes), secret) {
				t.Fatal("API process log contained sensitive fixture material")
			}
		}
	}
	t.Logf("PASS: actual API PIDs %d -> %d, same isolated PG and key config; evidence=%s", firstPID, secondProcess.Process.Pid, evidence)
}
