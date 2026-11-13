// Responsibility: combined-client role/readiness negatives against existing PG.
// This is constructor coverage, not native dualqueue or real-SFU acceptance.
package foundation_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"livecommerce/internal/live"
)

func TestLiveBrowserInputProcessReadiness(t *testing.T) {
	f := brwWireStarted(t)
	h := f.h
	ctx := context.Background()
	construct := func() error {
		_, err := live.NewMediaClientWithBrowserInput(ctx, h.worker, h.executor, f.keys, []live.MediaProject{f.egress}, h.runtime, 1)
		return err
	}
	if err := construct(); err != nil {
		t.Fatalf("clean combined constructor: %v", err)
	}
	for _, tc := range []struct{ name, grant, revoke string }{
		{"input", `GRANT EXECUTE ON FUNCTION live.browser_input_next_action(uuid) TO PUBLIC`, `REVOKE EXECUTE ON FUNCTION live.browser_input_next_action(uuid) FROM PUBLIC`},
		{"legacy", `GRANT EXECUTE ON FUNCTION live.reserve_media_start(uuid,bigint,bytea) TO commerce_runtime`, `REVOKE EXECUTE ON FUNCTION live.reserve_media_start(uuid,bigint,bytea) FROM commerce_runtime`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mustExec(t, h.lp.f.owner, tc.grant)
			t.Cleanup(func() { mustExec(t, h.lp.f.owner, tc.revoke) })
			if err := construct(); err == nil {
				t.Fatal("combined constructor accepted unsafe readiness")
			}
			mustExec(t, h.lp.f.owner, tc.revoke)
			if err := construct(); err != nil {
				t.Fatalf("restored constructor: %v", err)
			}
		})
	}
	if _, err := live.NewMediaClientWithBrowserInput(ctx, h.executor, h.worker, f.keys, []live.MediaProject{f.egress}, h.runtime, 1); err == nil {
		t.Fatal("swapped native roles accepted")
	}
	if f.inputs.Load() != 0 || f.starts.Load() != 0 {
		t.Fatal("constructor performed provider I/O")
	}
}

// Native fixture: only the official pinned executable answers input RPCs.
// TLS terminates in the existing CA fixture and forwards unchanged RPC bodies
// to a literal-loopback SFU; credentials and response bodies are never logged.
type brpSFU struct {
	dial, ca, ws, key, secret string
	mu                        sync.Mutex
	calls                     []time.Time
	participant, rooms        int
	room, identity            string
}

func brpNativeSFU(t *testing.T) *brpSFU {
	t.Helper()
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" || os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" || os.Getenv("LC_TEST_DATABASE_URL") == "" {
		t.Fatal("native PROCESS requires Linux amd64 and real PG; NOT_RUN")
	}
	binary := os.Getenv("COMMERCE_R04_LIVEKIT_BINARY")
	pin := os.Getenv("COMMERCE_R04_LIVEKIT_BINARY_SHA256")
	data, err := os.ReadFile(binary)
	if err != nil || !filepath.IsAbs(binary) || len(pin) != 64 {
		t.Fatal("verified SFU fixture missing; NOT_RUN")
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != pin {
		t.Fatal("SFU executable checksum mismatch")
	}
	version, err := exec.Command(binary, "--version").Output()
	if err != nil || !bytes.Contains(version, []byte("1.13.7")) {
		t.Fatal("SFU version mismatch")
	}
	port := func() int {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		n := l.Addr().(*net.TCPAddr).Port
		_ = l.Close()
		return n
	}
	signal, rtc := port(), port()
	udp, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	udpPort := udp.LocalAddr().(*net.UDPAddr).Port
	_ = udp.Close()
	if signal == rtc || signal == udpPort || rtc == udpPort {
		t.Fatal("SFU fixture port collision")
	}
	f := &brpSFU{key: "native" + hex.EncodeToString(randomBytes(8)), secret: hex.EncodeToString(randomBytes(32)), ws: fmt.Sprintf("ws://127.0.0.1:%d", signal)}
	config := filepath.Join(t.TempDir(), "livekit.yaml")
	body := fmt.Sprintf("port: %d\nbind_addresses: [127.0.0.1]\nrtc:\n  tcp_port: %d\n  udp_port: %d\n  use_external_ip: false\n  node_ip: 127.0.0.1\nkeys:\n  %s: %s\n", signal, rtc, udpPort, f.key, f.secret)
	if os.WriteFile(config, []byte(body), 0600) != nil {
		t.Fatal("SFU private config write")
	}
	cmd := exec.Command(binary, "--config", config)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH")}
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	if err := cmd.Start(); err != nil {
		t.Fatal("SFU launch failed")
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	t.Cleanup(func() {
		_ = cmd.Process.Signal(syscall.SIGTERM)
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			_ = cmd.Process.Kill()
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Error("SFU not reaped")
			}
		}
		if err := os.Remove(config); err != nil {
			t.Error("SFU config cleanup failed")
		}
		t.Logf("REAL_LOCAL_SFU cleanup pid=%d reaped/config removed", cmd.Process.Pid)
	})
	target, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", signal))
	transport := &http.Transport{Proxy: nil, DisableKeepAlives: true}
	t.Cleanup(transport.CloseIdleConnections)
	client := &http.Client{Transport: transport, Timeout: time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	ready := false
	for until := time.Now().Add(10 * time.Second); time.Now().Before(until); {
		response, err := client.Get(target.String())
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == 200 {
				ready = true
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !ready {
		t.Fatal("SFU readiness failed")
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = transport
	server, ca := lmwTLS(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != "unit.livekit.cloud" || r.TLS == nil {
			t.Error("input TLS target mismatch")
			http.Error(w, "target", 400)
			return
		}
		raw, err := io.ReadAll(io.LimitReader(r.Body, 4096))
		if err != nil {
			t.Error("input request read")
			return
		}
		var q struct {
			Room, Identity string
			Names          []string
		}
		if json.Unmarshal(raw, &q) != nil {
			t.Error("input request decode")
			return
		}
		f.mu.Lock()
		valid := false
		switch r.URL.Path {
		case "/twirp/livekit.RoomService/GetParticipant":
			valid = q.Room == f.room && q.Identity == f.identity
			f.participant++
		case "/twirp/livekit.RoomService/ListRooms":
			valid = len(q.Names) == 1 && q.Names[0] == f.room
			f.rooms++
		}
		f.calls = append(f.calls, time.Now())
		f.mu.Unlock()
		if !valid {
			t.Error("input cross-route or target mismatch")
			http.Error(w, "target", 400)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(raw))
		proxy.ServeHTTP(w, r)
	}))
	f.dial, f.ca = server.Listener.Addr().String(), ca
	t.Logf("REAL_LOCAL_SFU version=1.13.7 pid=%d executable_sha256=%s", cmd.Process.Pid, pin)
	return f
}

func brpEnvironment(t *testing.T, h *lmeHarness, input *brwHarness, f *brpSFU, dial, ca string) []string {
	t.Helper()
	f.mu.Lock()
	f.room, f.identity = input.plan.RoomName, input.grant.PublisherIdentity
	f.mu.Unlock()
	env := lmwReplace(lmwEnvironment(h, dial, ca), "COMMERCE_MEDIA_WORKER_CONCURRENCY", "2")
	var document map[string][]map[string]any
	if json.Unmarshal([]byte(lmwEnvValue(env, "COMMERCE_MEDIA_PROJECTS_JSON")), &document) != nil {
		t.Fatal("fixture configuration")
	}
	document["projects"][0]["browser_input"] = map[string]any{"api_key": f.key, "api_secret": f.secret, "mock_dial_address": f.dial, "mock_ca_pem": f.ca, "browser_url": f.ws}
	data, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	env = lmwReplace(env, "COMMERCE_MEDIA_PROJECTS_JSON", string(data))
	recovery, _ := mrrRecoveryPool(t, h)
	return append(env, "COMMERCE_MEDIA_BROWSER_INPUT_ENABLED=1", "COMMERCE_MEDIA_RECOVERY_SUPERVISED=1", "COMMERCE_MEDIA_RECOVERY_DATABASE_URL="+recovery.Config().ConnString())
}

func TestLiveBrowserInputProcessNativeQueues(t *testing.T) {
	sfu := brpNativeSFU(t)
	binary := mrBuild(t, "../../cmd/media-worker", "native-queues")
	input := brwStart(t)
	brwReserveGrant(t, input)
	h := lmeSetup(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("unused in-process Egress route called")
		http.Error(w, "unused", 500)
	})
	server, ca := lmwTLS(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != "unit.livekit.cloud" || r.TLS == nil {
			t.Error("Egress TLS target mismatch")
		}
		brpEgressAuth(t, r)
		var q struct {
			RoomName string `json:"room_name"`
		}
		if json.NewDecoder(r.Body).Decode(&q) != nil || q.RoomName != h.plan.RoomName {
			t.Error("Egress cross-route/target mismatch")
			http.Error(w, "target", 400)
			return
		}
		switch r.URL.Path {
		case "/twirp/livekit.Egress/StartEgress":
			lmeReply(w, h.observation("EG_native", "EGRESS_ACTIVE", 100, 110, 0))
		case "/twirp/livekit.Egress/ListEgress":
			lmeReply(w, `{"items":[`+h.observation("EG_native", "EGRESS_ACTIVE", 100, 120, 0)+`]}`)
		default:
			t.Error("unexpected Egress mutation")
			http.Error(w, "method", 400)
		}
	}))
	// Causal negative: the same original input job cannot be reassigned to the
	// Egress queue. The unchanged job then runs through the real native client.
	if _, err := h.lp.f.owner.Exec(context.Background(), `UPDATE river_media.river_job SET queue='media_mock_v1' WHERE id=$1`, input.plan.JobID); sqlState(err) != "22023" {
		t.Fatalf("cross-route negative state=%s", sqlState(err))
	}
	t.Log("REAL_PG queue cross-route rejected SQLSTATE=22023")
	env := brpEnvironment(t, h, input, sfu, server.Listener.Addr().String(), ca)
	launched := time.Now()
	p := lmwProcess(t, binary, "native-queues", env)
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
	mrReadyLog(t, p, "media_worker_ready")
	deadline := time.Now().Add(35 * time.Second)
	var inputAttempt, legacyAttempt int
	var turn string
	for time.Now().Before(deadline) {
		err := h.lp.f.owner.QueryRow(context.Background(), `SELECT j.attempt,c.last_runtime_turn FROM river_media.river_job j JOIN live.media_input_custody c ON c.operation_id=$2::uuid WHERE j.id=$1`, input.plan.JobID, input.plan.OperationID).Scan(&inputAttempt, &turn)
		if err != nil {
			t.Fatal(err)
		}
		if err := h.lp.f.owner.QueryRow(context.Background(), `SELECT attempt FROM river_media.river_job WHERE id=$1`, h.plan.JobID).Scan(&legacyAttempt); err != nil {
			t.Fatal(err)
		}
		if inputAttempt > 0 && legacyAttempt > 0 && turn == "INPUT_OBSERVE" && h.facts(t).egress == "EG_native" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if inputAttempt < 1 || legacyAttempt < 1 || turn != "INPUT_OBSERVE" || h.facts(t).egress != "EG_native" {
		t.Fatalf("native queues not consumed: input=%d legacy=%d turn=%s", inputAttempt, legacyAttempt, turn)
	}
	for _, pair := range []struct {
		op             string
		job            int64
		queue, profile string
	}{{input.plan.OperationID, input.plan.JobID, "media_input_mock_v1", "LOCAL_SFU_MOCK_EGRESS"}, {h.plan.OperationID, h.plan.JobID, "media_mock_v1", "PROVIDER_MOCK"}} {
		var job int64
		var queue, profile string
		if err := h.lp.f.owner.QueryRow(context.Background(), `SELECT o.job_id,j.queue,a.execution_profile FROM integration.operations o JOIN river_media.river_job j ON j.id=o.job_id JOIN live.media_attempts a ON a.id=o.media_attempt_id WHERE o.id=$1::uuid`, pair.op).Scan(&job, &queue, &profile); err != nil {
			t.Fatal(err)
		}
		if job != pair.job || queue != pair.queue || profile != pair.profile {
			t.Fatal("original job/profile/queue binding changed")
		}
	}
	sfu.mu.Lock()
	participant := sfu.participant
	sfu.mu.Unlock()
	if participant < 1 {
		t.Fatal("compiled child never reached real SFU")
	}
	t.Logf("REAL_PG native dual queues parent_pid=%d input_job=%d egress_job=%d attempts=%d/%d input_turn=%s", p.cmd.Process.Pid, input.plan.JobID, h.plan.JobID, inputAttempt, legacyAttempt, turn)
	mrStop(t, p, syscall.SIGTERM, true)
}

func brpEgressAuth(t *testing.T, r *http.Request) {
	t.Helper()
	parts := strings.Split(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), ".")
	if len(parts) != 3 {
		t.Error("missing Egress credentials")
		return
	}
	mac := hmac.New(sha256.New, []byte(strings.Repeat("s", 40)))
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(signature, mac.Sum(nil)) {
		t.Error("Egress credentials crossed input route")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Error("Egress JWT format")
		return
	}
	var claims struct {
		Issuer string `json:"iss"`
	}
	if json.Unmarshal(payload, &claims) != nil || claims.Issuer != "lme_test_key" {
		t.Error("Egress issuer mismatch")
	}
}

func brpReadyCount(t *testing.T, p *mrProcess, want int) {
	t.Helper()
	for until := time.Now().Add(10 * time.Second); time.Now().Before(until); {
		data, err := os.ReadFile(p.logPath)
		if err == nil && strings.Count(string(data), "media_worker_ready") >= want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("native child readiness count not reached")
}
