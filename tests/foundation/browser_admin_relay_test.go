//go:build browser

// Purpose: keep a browser fixture's public admin origin owned while Next starts on an automatic private port; start()
// also launches a Go-owned child on a private port and relaunches ONLY after a bind collision (EADDRINUSE), see #116.
// Depends on: Go net/http/httptest/httputil, os/exec, /proc (Linux listener-ownership proof) and browserFront's WebKit TLS boundary; no database.
// Used by: publish, domains, merchant-buyer, catalog-media and manual-order-link browser fixtures and every Go browser fixture
// that spawns the packaged admin Next via relay.start; GATE-PORT real-socket regression tests (the freed-port handoff
// red/green witness and TestBrowserAdminRelayStartRetriesBindCollision).
package foundation_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

type browserAdminRelay struct {
	origin string
	server *httptest.Server
	target atomic.Pointer[url.URL]
	// pickPort chooses a child's private port; nil = a free loopback port. Tests inject a held port to force a collision.
	pickPort func() string
	// owns proves a child's process group owns the LISTEN socket (nil = /proc); kill signals a group (nil = SIGKILL). Test seams.
	owns func(pgid, port int) (bool, error)
	kill func(pgid int)
}

// newBrowserAdminRelay preserves the IdP/CSRF origin instead of lending its port to a later child process.
func newBrowserAdminRelay(t *testing.T) *browserAdminRelay {
	t.Helper()
	front := &browserAdminRelay{}
	proxy := &httputil.ReverseProxy{Rewrite: func(r *httputil.ProxyRequest) {
		r.SetURL(front.target.Load())
		r.Out.URL.RawQuery = r.In.URL.RawQuery // #122: Rewrite proxies drop unparsable queries (`?limit=%`); security negatives need them raw. Test transport only.
		r.Out.Host = r.In.Host
		// Match browserFront: keep its HTTPS marker and the inbound client-IP fixture, not a new relay hop.
		for _, name := range []string{"X-Forwarded-For", "X-Forwarded-Proto"} {
			if values, ok := r.In.Header[name]; ok {
				r.Out.Header[name] = values
			}
		}
	}}
	front.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if front.target.Load() == nil {
			http.Error(w, "fixture admin not connected", http.StatusServiceUnavailable)
			return
		}
		proxy.ServeHTTP(w, r)
	}))
	t.Cleanup(front.server.Close)
	front.origin = browserFront(t, front.server.Listener.Addr().String())
	return front
}

var bindCollision = regexp.MustCompile(`EADDRINUSE|address already in use`)

// start launches cmd (a not-yet-started Node/Go child built with exec.CommandContext(ctx, ...)) on a private loopback port
// (PORT is set here) and attaches it to the relay's stable public address, so the IdP/CSRF origin never depends on that port.
// The pick-then-release window cannot be closed for a child process, so a launch that dies with EADDRINUSE in its own output
// is relaunched on a new port (3 launches, each retry logged, every relaunch bound to the same ctx); any other failure, a
// cancelled ctx, or a child whose listener cannot be proven to be its own fails the test. Returns the live command.
func (front *browserAdminRelay) start(t *testing.T, ctx context.Context, cmd *exec.Cmd) *exec.Cmd {
	t.Helper()
	child, err := front.launch(t, ctx, cmd)
	if err != nil {
		t.Fatal(err)
	}
	return child
}

func (front *browserAdminRelay) launch(t *testing.T, ctx context.Context, cmd *exec.Cmd) (*exec.Cmd, error) {
	t.Helper()
	logFile, ok := cmd.Stdout.(*os.File)
	if !ok {
		return nil, errors.New("relay.start needs the child's *os.File log as Stdout (browserLog)")
	}
	owns, kill := front.owns, front.kill
	if owns == nil {
		owns = processGroupListens
	}
	if kill == nil {
		kill = func(pgid int) { _ = syscall.Kill(-pgid, syscall.SIGKILL) }
	}
	for attempt := 1; ; attempt++ {
		var port string
		if front.pickPort != nil {
			port = front.pickPort()
		} else {
			l, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				return nil, err
			}
			_, port, _ = net.SplitHostPort(l.Addr().String())
			_ = l.Close() // ponytail: still pick-then-release; the EADDRINUSE relaunch below is what makes it safe
		}
		child := cmd
		if attempt > 1 { // an exec.Cmd cannot be restarted; same path/args/dir/env/log, fresh process, same ctx
			child = exec.CommandContext(ctx, cmd.Path, cmd.Args[1:]...)
			child.Dir, child.Env, child.Stdout, child.Stderr = cmd.Dir, cmd.Env, cmd.Stdout, cmd.Stderr
		}
		env := make([]string, 0, len(child.Env)+1)
		for _, entry := range child.Env {
			if !strings.HasPrefix(entry, "PORT=") {
				env = append(env, entry)
			}
		}
		child.Env = append(env, "PORT="+port)
		child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		var from int64
		if info, err := logFile.Stat(); err == nil {
			from = info.Size()
		}
		if err := child.Start(); err != nil {
			return nil, err
		}
		// Group-kill protocol: the group is signalled exactly once and never after its leader was reaped for good, so a
		// recycled PGID is never hit. The reaper sweeps leftovers the moment the leader exits; cleanup covers a live leader.
		pgid := child.Process.Pid
		var mu sync.Mutex
		killed := false
		exited := make(chan struct{})
		go func() {
			_ = child.Wait()
			mu.Lock()
			if !killed {
				kill(pgid)
				killed = true
			}
			mu.Unlock()
			close(exited)
		}()
		t.Cleanup(func() {
			mu.Lock()
			if !killed {
				kill(pgid)
				killed = true
			}
			mu.Unlock()
			<-exited
		})
		collided, err := awaitOwnedPort(ctx, owns, pgid, port, exited, logFile.Name(), from)
		if err == nil {
			front.target.Store(&url.URL{Scheme: "http", Host: net.JoinHostPort("127.0.0.1", port)})
			return child, nil
		}
		if !collided || attempt == 3 {
			return nil, err
		}
		t.Logf("fixture child lost 127.0.0.1:%s (EADDRINUSE); relaunching on a new port (%d/3)", port, attempt+1)
	}
}

// awaitOwnedPort returns nil once the child's own process group listens on port. A child that exits first is reported with
// collided=true only when its output (from offset `from`) names a bind collision; anything else, including a cancelled ctx
// and an ownership probe that cannot answer, is a plain failure (fail closed).
func awaitOwnedPort(ctx context.Context, owns func(pgid, port int) (bool, error), pgid int, port string, exited <-chan struct{}, logPath string, from int64) (collided bool, err error) {
	number, _ := strconv.Atoi(port)
	for deadline := time.Now().Add(60 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		select {
		case <-exited:
			out, _ := os.ReadFile(logPath)
			collided = from <= int64(len(out)) && bindCollision.Match(out[from:])
			return collided, fmt.Errorf("fixture child exited before owning 127.0.0.1:%s; see %s", port, logPath)
		case <-ctx.Done():
			return false, fmt.Errorf("fixture child start cancelled before owning 127.0.0.1:%s: %w", port, ctx.Err())
		default:
		}
		owned, err := owns(pgid, number)
		if err != nil {
			return false, fmt.Errorf("cannot prove 127.0.0.1:%s belongs to the fixture child: %w", port, err)
		}
		if owned {
			return false, nil
		}
		if time.Now().After(deadline) {
			return false, fmt.Errorf("fixture child never owned 127.0.0.1:%s within 60s; see %s", port, logPath)
		}
	}
}

// processGroupListens proves the LISTEN socket on port belongs to the child's own process group, so a rival that took the
// freed port cannot pass for the child. Needs /proc (Linux, where CI runs); without it the proof is unavailable and the
// caller fails closed, because a successful dial proves a listener exists, not whose it is.
func processGroupListens(pgid, port int) (bool, error) {
	tcp4, err := os.ReadFile("/proc/net/tcp")
	if err != nil {
		return false, fmt.Errorf("listener ownership proof needs /proc: %w", err)
	}
	tcp6, _ := os.ReadFile("/proc/net/tcp6")
	inodes := map[string]bool{}
	for _, line := range strings.Split(string(tcp4)+string(tcp6), "\n") {
		if f := strings.Fields(line); len(f) > 9 && f[3] == "0A" && strings.HasSuffix(f[1], fmt.Sprintf(":%04X", port)) {
			inodes["socket:["+f[9]+"]"] = true
		}
	}
	procs, _ := os.ReadDir("/proc")
	for _, proc := range procs {
		pid, err := strconv.Atoi(proc.Name())
		if err != nil {
			continue
		}
		if group, err := syscall.Getpgid(pid); err != nil || group != pgid {
			continue
		}
		fds, _ := os.ReadDir(fmt.Sprintf("/proc/%d/fd", pid))
		for _, fd := range fds {
			if link, err := os.Readlink(fmt.Sprintf("/proc/%d/fd/%s", pid, fd.Name())); err == nil && inodes[link] {
				return true, nil
			}
		}
	}
	return false, nil
}

// connect accepts only an internal loopback port behind the caller's existing authenticated control endpoint.
func (front *browserAdminRelay) connect(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Port int `json:"port"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&in) != nil || decoder.Decode(new(any)) != io.EOF || in.Port < 1 || in.Port > 65535 ||
		front.server.Listener.Addr().String() == net.JoinHostPort("127.0.0.1", strconv.Itoa(in.Port)) ||
		front.origin == "https://127.0.0.1:"+strconv.Itoa(in.Port) {
		http.Error(w, "invalid fixture admin port", http.StatusBadRequest)
		return
	}
	target := &url.URL{Scheme: "http", Host: net.JoinHostPort("127.0.0.1", strconv.Itoa(in.Port))}
	if !front.target.CompareAndSwap(nil, target) && front.target.Load().Host != target.Host {
		http.Error(w, "fixture admin already connected", http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func TestBrowserAdminRelayOwnsExplicitOrigin(t *testing.T) {
	t.Setenv("LC_BROWSER_ENGINE", "chromium")
	front := newBrowserAdminRelay(t)
	address := front.server.Listener.Addr().String()
	stolen, err := net.Listen("tcp", address) // deterministic rival fixture, exactly the PR29 collision window
	if err == nil {
		stolen.Close()
		t.Fatal("another fixture stole the explicit public admin port before Next startup")
	}
	for range 3 {
		other := httptest.NewServer(http.NotFoundHandler())
		t.Cleanup(other.Close)
		if other.Listener.Addr().String() == address {
			t.Fatal("a fixture's listen(0) reused the held public admin port")
		}
	}
	response, err := front.server.Client().Get(front.origin + "/api/stores")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("unconnected public relay: got %d, want 503", response.StatusCode)
	}
	front.server.Close()
	reclaimed, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatalf("owned cleanup did not release the public listener: %v", err)
	}
	reclaimed.Close()
}

// TestBrowserAdminRelayHandoffHasNoFreePortWindow is the GATE-PORT round-1 (K3 P1) red/green witness for the
// catalog-media and manual-order-link fixtures. RED: replay their pre-fix handoff on real sockets — reserve a
// public port, pin the origin with browserFront, free the port, and a deterministic rival takes it inside the
// window; the explicit-port bind the gate's Next child then performs fails with EADDRINUSE, which
// startNextWithPortRetry never retries for a caller-supplied port (proved in tests/ci/next-startup.test.mjs).
// GREEN: the relay handoff frees nothing — the public listener stays owned (the same rival must fail), Next
// binds an automatic private port and connect() attaches it to the unchanged public origin.
func TestBrowserAdminRelayHandoffHasNoFreePortWindow(t *testing.T) {
	for _, engine := range []string{"chromium", "webkit"} {
		t.Run(engine, func(t *testing.T) {
			t.Setenv("LC_BROWSER_ENGINE", engine)
			// ---- RED: the pre-fix reserve-close-handoff of the two converted fixtures ------------------------------
			reserved, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			address := reserved.Addr().String()
			_ = browserFront(t, address) // the pre-fix fixtures pinned the public origin BEFORE freeing the port
			if err := reserved.Close(); err != nil {
				t.Fatal(err)
			}
			rival, err := net.Listen("tcp", address) // the PR29 window: a parallel process takes the just-freed port
			if err != nil {
				t.Fatalf("red witness broken: the freed public admin port was not stealable: %v", err)
			}
			explicit, err := net.Listen("tcp", address) // what the gate's Next child did with the caller-supplied LC_*_ADMIN_PORT
			if explicit != nil {
				_ = explicit.Close()
			}
			if !errors.Is(err, syscall.EADDRINUSE) {
				t.Fatalf("the explicit-port bind on the stolen port must fail with EADDRINUSE, got %v", err)
			}
			if err := rival.Close(); err != nil {
				t.Fatal(err)
			}

			// ---- GREEN: the relay handoff the two fixtures use now --------------------------------------------------
			front := newBrowserAdminRelay(t)
			stolen, err := net.Listen("tcp", front.server.Listener.Addr().String())
			if err == nil {
				_ = stolen.Close()
				t.Fatal("a rival stole the public admin port while the relay holds it")
			}
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.WriteString(w, "ready "+r.Host) // stand-in for the ready automatic-port admin Next
			}))
			t.Cleanup(upstream.Close)
			_, port, err := net.SplitHostPort(upstream.Listener.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			recorder := httptest.NewRecorder()
			front.connect(recorder, httptest.NewRequest("POST", "/admin-upstream", strings.NewReader(`{"port":`+port+`}`)))
			if recorder.Code != http.StatusNoContent {
				t.Fatalf("fixture registration: got %d, want 204", recorder.Code)
			}
			client := front.server.Client()
			if engine == "webkit" {
				client = browserAdminTLSClient(t)
			}
			response, err := client.Get(front.origin + "/api/stores")
			if err != nil {
				t.Fatal(err)
			}
			body, err := io.ReadAll(response.Body)
			_ = response.Body.Close()
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(string(body), "ready ") {
				t.Fatalf("the owned public origin did not reach the automatic-port upstream: %q", body)
			}
		})
	}
}

func TestBrowserAdminRelayPreservesOriginAndTLSHeaders(t *testing.T) {
	for _, engine := range []string{"chromium", "webkit"} {
		t.Run(engine, func(t *testing.T) {
			t.Setenv("LC_BROWSER_ENGINE", engine)
			front := newBrowserAdminRelay(t)
			origin, err := url.Parse(front.origin)
			if err != nil {
				t.Fatal(err)
			}
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Host != origin.Host || r.URL.RequestURI() != "/api/auth/callback?code=synthetic&state=synthetic" ||
					r.Header.Get("X-Forwarded-For") != "192.0.2.12" ||
					(engine == "webkit" && r.Header.Get("X-Forwarded-Proto") != "https") {
					t.Errorf("public origin/path or forwarded fixture headers changed: host=%s path=%s", r.Host, r.URL.RequestURI())
				}
				w.Header().Set("Set-Cookie", "__Host-synthetic=fixture; Path=/; Secure; HttpOnly; SameSite=Lax")
				w.WriteHeader(http.StatusUnauthorized)
			}))
			t.Cleanup(upstream.Close)
			_, port, err := net.SplitHostPort(upstream.Listener.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			_, selfPort, err := net.SplitHostPort(front.server.Listener.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			for _, body := range []string{`{"port":0}`, `{"port":65536}`, `{"port":1,"host":"example.invalid"}`, `{"port":1} {}`, `{"port":` + selfPort + `}`} {
				r := httptest.NewRecorder()
				front.connect(r, httptest.NewRequest("POST", "/admin-upstream", strings.NewReader(body)))
				if r.Code != http.StatusBadRequest || front.target.Load() != nil {
					t.Fatalf("invalid control body accepted: %s status=%d", body, r.Code)
				}
			}
			r := httptest.NewRecorder()
			front.connect(r, httptest.NewRequest("POST", "/admin-upstream", strings.NewReader(`{"port":`+port+`}`)))
			if r.Code != http.StatusNoContent || front.origin != origin.String() {
				t.Fatal("connect changed the public origin or did not acknowledge")
			}
			for _, body := range []string{`{"port":0}`, `{"port":1}`} {
				r := httptest.NewRecorder()
				front.connect(r, httptest.NewRequest("POST", "/admin-upstream", strings.NewReader(body)))
				if (r.Code != http.StatusBadRequest && r.Code != http.StatusConflict) || front.target.Load().Host != upstream.Listener.Addr().String() {
					t.Fatal("a bad or different registration changed the already connected upstream")
				}
			}
			request, err := http.NewRequest("GET", front.origin+"/api/auth/callback?code=synthetic&state=synthetic", nil)
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("X-Forwarded-For", "192.0.2.12")
			// The same existing httptest self-signed front as browserFront; no production trust is bypassed.
			client := upstream.Client()
			if engine == "webkit" {
				client = browserAdminTLSClient(t)
			}
			response, err := client.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != http.StatusUnauthorized || !strings.Contains(response.Header.Get("Set-Cookie"), "Secure; HttpOnly") {
				t.Fatalf("upstream auth status/cookie changed: status=%d", response.StatusCode)
			}
		})
	}
}

func TestBrowserAdminRelayPreservesPostCSRFAndRedirect(t *testing.T) {
	for _, engine := range []string{"chromium", "webkit"} {
		t.Run(engine, func(t *testing.T) {
			t.Setenv("LC_BROWSER_ENGINE", engine)
			front := newBrowserAdminRelay(t)
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil || r.Method != "POST" || string(body) != `{"synthetic":true}` ||
					r.Header.Get("Origin") != front.origin || r.Header.Get("Cookie") != "__Host-commerce_session=synthetic" ||
					r.Header.Get("X-CSRF-Token") != "synthetic-csrf" || r.URL.RequestURI() != "/api/stores?fixture=1" {
					t.Error("owned frontend changed the request body, method, Origin, CSRF, cookie or query")
				}
				w.Header().Set("Location", front.origin+"/en")
				w.WriteHeader(http.StatusTemporaryRedirect)
			}))
			t.Cleanup(upstream.Close)
			_, port, err := net.SplitHostPort(upstream.Listener.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			r := httptest.NewRecorder()
			front.connect(r, httptest.NewRequest("POST", "/admin-upstream", strings.NewReader(`{"port":`+port+`}`)))
			if r.Code != http.StatusNoContent {
				t.Fatal("fixture registration did not acknowledge")
			}
			request, err := http.NewRequest("POST", front.origin+"/api/stores?fixture=1", strings.NewReader(`{"synthetic":true}`))
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("Origin", front.origin)
			request.Header.Set("Cookie", "__Host-commerce_session=synthetic")
			request.Header.Set("X-CSRF-Token", "synthetic-csrf")
			client := &http.Client{}
			if engine == "webkit" {
				client = browserAdminTLSClient(t)
			}
			client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
			response, err := client.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != http.StatusTemporaryRedirect || response.Header.Get("Location") != front.origin+"/en" {
				t.Fatal("owned frontend changed the response status or public redirect")
			}
		})
	}
}

// browserAdminTLSClient trusts httptest's standard test certificate without disabling certificate verification.
func browserAdminTLSClient(t *testing.T) *http.Client {
	t.Helper()
	certificate := httptest.NewTLSServer(http.NotFoundHandler())
	t.Cleanup(certificate.Close)
	return certificate.Client()
}

// relayTestChild is a stand-in Node child: the test binary re-executed to bind $PORT. Like Node it prints EADDRINUSE and
// exits 1 when it cannot. fail=true is a child that dies for an unrelated reason.
func relayTestChild(t *testing.T, ctx context.Context, fail bool) *exec.Cmd {
	t.Helper()
	log, err := os.Create(t.TempDir() + "/child.log")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = log.Close() })
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestBrowserAdminRelayStartRetriesBindCollision$")
	cmd.Env = append(os.Environ(), "LC_RELAY_TEST_CHILD=1")
	if fail {
		cmd = exec.CommandContext(ctx, "sh", "-c", "echo boom; exit 3")
	}
	cmd.Stdout, cmd.Stderr = log, log
	return cmd
}

// relayHeldPort listens on a loopback port for the whole test, the rival that wins the pick-then-release window.
func relayHeldPort(t *testing.T) string {
	t.Helper()
	rival, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rival.Close() })
	_, port, _ := net.SplitHostPort(rival.Addr().String())
	return port
}

// TestBrowserAdminRelayStartRetriesBindCollision is the #116 red/green witness for relay.start (fixes #119 and #120 of PR118).
// RED: a rival LISTENING on the picked port (the pick-then-release window) kills a single-shot launch — the old behaviour.
// GREEN: start() relaunches on a new port, attaches it to the stable public origin, relaunches ONLY on EADDRINUSE, keeps the
// caller's context on the relaunched child (#119: cancel must stop it) and signals each attempt's process group exactly once,
// as soon as that attempt is reaped and never from a deferred cleanup of an already-recycled PGID (#120).
func TestBrowserAdminRelayStartRetriesBindCollision(t *testing.T) {
	if os.Getenv("LC_RELAY_TEST_CHILD") == "1" {
		l, err := net.Listen("tcp", "127.0.0.1:"+os.Getenv("PORT"))
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error: listen EADDRINUSE:", err)
			os.Exit(1)
		}
		_ = http.Serve(l, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "child ok") }))
		os.Exit(0)
	}
	t.Setenv("LC_BROWSER_ENGINE", "chromium")
	held := relayHeldPort(t)

	// ---- RED: one launch on the rival-held port, exactly what the pick-then-release harnesses did ----
	single := relayTestChild(t, context.Background(), false)
	single.Env = append(single.Env, "PORT="+held)
	if err := single.Run(); err == nil {
		t.Fatal("red witness broken: a launch on a rival-held port must fail")
	}
	out, _ := os.ReadFile(single.Stdout.(*os.File).Name())
	if !bindCollision.Match(out) {
		t.Fatalf("red witness broken: the failed launch did not report a bind collision: %q", out)
	}

	// ---- GREEN: the first pick collides with the rival; the relaunch succeeds behind the unchanged public origin ----
	var (
		mu     sync.Mutex
		kills  []int
		picks  []string
		relay  = newBrowserAdminRelay(t)
		public = relay.server.Listener.Addr().String()
	)
	relay.kill = func(pgid int) {
		mu.Lock()
		kills = append(kills, pgid)
		mu.Unlock()
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
	}
	relay.pickPort = func() string {
		port := held
		if len(picks) > 0 {
			l, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			_, port, _ = net.SplitHostPort(l.Addr().String())
			_ = l.Close()
		}
		picks = append(picks, port)
		return port
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	t.Run("relaunch", func(t *testing.T) {
		if _, err := relay.launch(t, ctx, relayTestChild(t, ctx, false)); err != nil {
			t.Fatalf("collision on the first port was not retried: %v", err)
		}
		if len(picks) != 2 || relay.target.Load().Host != net.JoinHostPort("127.0.0.1", picks[1]) {
			t.Fatalf("relay is not attached to the relaunched child: picks=%v target=%v", picks, relay.target.Load())
		}
		get := func() (string, error) {
			response, err := relay.server.Client().Get(relay.server.URL)
			if err != nil {
				return "", err
			}
			defer response.Body.Close()
			body, _ := io.ReadAll(response.Body)
			if response.StatusCode != http.StatusOK {
				return "", fmt.Errorf("status %d", response.StatusCode)
			}
			return string(body), nil
		}
		if body, err := get(); err != nil || body != "child ok" || relay.server.Listener.Addr().String() != public {
			t.Fatalf("public origin did not reach the relaunched child: %q %v", body, err)
		}
		// #120: the failed attempt's group was signalled when it was reaped, before launch returned — not left to a cleanup
		// that would run minutes later against a possibly recycled PGID.
		mu.Lock()
		early := append([]int(nil), kills...)
		mu.Unlock()
		if len(early) != 1 {
			t.Fatalf("failed attempt's group must be signalled exactly once, at reap time: kills=%v", early)
		}
		// #119: the relaunch carries the caller's context, so cancelling it stops the relaunched child.
		cancel()
		for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(50 * time.Millisecond) {
			if _, err := get(); err != nil {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("cancelling the caller's context did not stop the relaunched child")
			}
		}
	})
	mu.Lock()
	defer mu.Unlock()
	if len(kills) != 2 || kills[0] == kills[1] {
		t.Fatalf("each attempt's group must be signalled exactly once in total (stale deferred kill?): %v", kills)
	}

	// ---- only EADDRINUSE is retried: any other early exit fails on the first launch ----
	other := newBrowserAdminRelay(t)
	launches := 0
	other.pickPort = func() string { launches++; return held }
	if _, err := other.launch(t, context.Background(), relayTestChild(t, context.Background(), true)); err == nil || launches != 1 {
		t.Fatalf("a non-bind failure must not be retried: err=%v launches=%d", err, launches)
	}
}

// TestBrowserAdminRelayFailsClosedWithoutOwnershipProof pins #121: a listener that merely accepts a connection is not the
// child. RED: the rival accepts a dial (what the old non-/proc fallback treated as ready). GREEN: with the ownership probe
// unavailable, launch fails closed instead of attaching the relay to the rival, and never retries on a guess.
func TestBrowserAdminRelayFailsClosedWithoutOwnershipProof(t *testing.T) {
	t.Setenv("LC_BROWSER_ENGINE", "chromium")
	held := relayHeldPort(t)
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", held), time.Second)
	if err != nil {
		t.Fatalf("red witness broken: the rival must accept connections: %v", err)
	}
	_ = conn.Close()
	relay := newBrowserAdminRelay(t)
	relay.pickPort = func() string { return held }
	relay.owns = func(int, int) (bool, error) { return false, errors.New("no /proc") }
	if _, err := relay.launch(t, context.Background(), relayTestChild(t, context.Background(), false)); err == nil || !strings.Contains(err.Error(), "cannot prove") {
		t.Fatalf("launch must fail closed when ownership cannot be proven: %v", err)
	}
	if relay.target.Load() != nil {
		t.Fatal("relay was attached to a listener that was not proven to be the child's")
	}
}

// TestBrowserAdminRelayForwardsRawQuery pins #122: the orders BFF negative sends `?limit=%` and expects the BFF to reject it.
// RED: a stock ReverseProxy Rewrite (SetURL only) drops the unparsable query, turning the negative into a legal request.
// GREEN: the relay hands the upstream the inbound RawQuery byte for byte (bad percent escape, semicolon, valid query).
func TestBrowserAdminRelayForwardsRawQuery(t *testing.T) {
	t.Setenv("LC_BROWSER_ENGINE", "chromium")
	var seen atomic.Value
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { seen.Store(r.URL.RawQuery) }))
	t.Cleanup(upstream.Close)
	target, _ := url.Parse(upstream.URL)
	send := func(base, query string) string {
		seen.Store("<none>")
		response, err := http.Get(base + "/api/orders?" + query)
		if err != nil {
			t.Fatal(err)
		}
		_ = response.Body.Close()
		return seen.Load().(string)
	}
	stock := httptest.NewServer(&httputil.ReverseProxy{Rewrite: func(r *httputil.ProxyRequest) { r.SetURL(target) }})
	t.Cleanup(stock.Close)
	if got := send(stock.URL, "limit=%"); got != "" {
		t.Fatalf("red witness broken: a stock Rewrite proxy was expected to drop the malformed query, got %q", got)
	}
	relay := newBrowserAdminRelay(t)
	relay.target.Store(target)
	for _, query := range []string{"limit=%", "limit=%zz&cursor=a", "a=1;b=2", "limit=25&status=paid"} {
		if got := send(relay.server.URL, query); got != query {
			t.Fatalf("relay changed the query: sent %q, upstream saw %q", query, got)
		}
	}
}
