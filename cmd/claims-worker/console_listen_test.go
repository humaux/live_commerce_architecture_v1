// Purpose: issue #83 regression — the comment-bridge listener must fail startup synchronously (before
// claims_worker_ready) when its address cannot be bound, and a clean Shutdown must not count as a failure.
// #107: run()'s wiring after the bind — a later Serve failure cancels the run and exits with its own token
// (claims_worker_console_failed, never claims_worker_start_failed); a clean Shutdown keeps the River result.
// review-148 P1-1: in-flight bridge requests are cancelled by the run ctx and the console join at shutdown is bounded.
// Depends on: newConsoleServer, joinConsole, startConsoleServer, serveConsole, superviseConsole, exitError (main.go), jobqueue errors, net/http,
// loopback TCP only (MOCK tier, no database). The binary-level bind case: tests/foundation/claims_worker_console_process_test.go.
// Used by: go test ./cmd/claims-worker (Foundation unit shard).
package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"livecommerce/internal/jobqueue"
)

func TestConsoleServerBindFailureIsReturned(t *testing.T) {
	held, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("hold port: %v", err)
	}
	defer held.Close()
	srv := &http.Server{Addr: held.Addr().String(), Handler: http.NotFoundHandler(), ReadHeaderTimeout: time.Second}
	defer srv.Close()
	if _, err := startConsoleServer(srv); err == nil {
		t.Fatal("bind on an occupied address returned nil; worker would still log claims_worker_ready")
	}
}

func TestConsoleServerCleanShutdownIsNotAFailure(t *testing.T) {
	srv := &http.Server{Addr: "127.0.0.1:0", Handler: http.NotFoundHandler(), ReadHeaderTimeout: time.Second}
	failed, err := startConsoleServer(srv)
	if err != nil {
		t.Fatalf("start on a free port: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	select {
	case err, ok := <-failed:
		if ok {
			t.Fatalf("clean shutdown reported as failure: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve goroutine did not finish after Shutdown")
	}
}

// #107: a Serve failure after the bind (here: the listener is closed under a live server) must reach run() as a
// cancelled run context and the claims_worker_console_failed token, whatever River returned on the way out.
func TestConsoleServeFailureEndsRunWithConsoleToken(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := &http.Server{Handler: http.NotFoundHandler(), ReadHeaderTimeout: time.Second}
	defer srv.Close()
	runCtx, cancelRun := context.WithCancel(context.Background())
	defer cancelRun()
	stop := make(chan struct{})
	defer close(stop)
	down := superviseConsole(serveConsole(srv, ln), stop, cancelRun)
	_ = ln.Close() // Accept now fails with net.ErrClosed: a real Serve error, not ErrServerClosed
	select {
	case <-runCtx.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("Serve failure did not cancel the run")
	}
	// River saw the cancel during Start (ErrStart), after ready (nil) or while stopping (ErrStop): all must name the bridge.
	for _, runErr := range []error{nil, jobqueue.ErrStart, jobqueue.ErrStop} {
		if got := exitError(down, runErr); got == nil || got.Error() != "claims_worker_console_failed" {
			t.Fatalf("exitError(serve failure, %v) = %v, want claims_worker_console_failed", runErr, got)
		}
	}
}

func TestConsoleCleanShutdownKeepsRunResult(t *testing.T) {
	srv := &http.Server{Addr: "127.0.0.1:0", Handler: http.NotFoundHandler(), ReadHeaderTimeout: time.Second}
	failed, err := startConsoleServer(srv)
	if err != nil {
		t.Fatalf("start on a free port: %v", err)
	}
	runCtx, cancelRun := context.WithCancel(context.Background())
	defer cancelRun()
	down := superviseConsole(failed, make(chan struct{}), cancelRun)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	time.Sleep(100 * time.Millisecond) // the supervisor has seen the closed channel; a wrong cancel would be visible now
	if runCtx.Err() != nil {
		t.Fatal("clean shutdown cancelled the run")
	}
	for _, tc := range []struct{ in, want error }{{nil, nil}, {jobqueue.ErrStart, errWorkerStart}, {jobqueue.ErrStop, errWorkerStop}} {
		if got := exitError(down, tc.in); !errors.Is(got, tc.want) {
			t.Fatalf("exitError(clean, %v) = %v, want %v", tc.in, got, tc.want)
		}
		if got := exitError(nil, tc.in); !errors.Is(got, tc.want) { // no bridge configured
			t.Fatalf("exitError(no bridge, %v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// review-148 P1-1: a bridge handler blocked on its request ctx (stand-in for a DB call under Console.mu) must be released
// when the run ctx is cancelled, otherwise console.Run's stopAll waits on the mutex forever.
func TestConsoleServerCancelsInflightRequestsWithRunCtx(t *testing.T) {
	base, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered, released := make(chan struct{}), make(chan struct{})
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-r.Context().Done()
		close(released)
	})
	srv := newConsoleServer("127.0.0.1:0", h, base)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	serveConsole(srv, ln)
	defer srv.Close()
	go func() {
		if resp, err := http.Get("http://" + ln.Addr().String() + "/"); err == nil {
			resp.Body.Close()
		}
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("handler never entered")
	}
	cancel()
	select {
	case <-released:
	case <-time.After(3 * time.Second):
		t.Fatal("in-flight handler not cancelled by the run ctx; shutdown would wait on Console.mu behind it")
	}
}

func TestJoinConsoleIsBounded(t *testing.T) {
	var stuck sync.WaitGroup
	stuck.Add(1)
	defer stuck.Done()
	start := time.Now()
	if joinConsole(&stuck, 100*time.Millisecond) || time.Since(start) > 2*time.Second {
		t.Fatal("join of a stuck console.Run must give up at the bound")
	}
	var done sync.WaitGroup
	if !joinConsole(&done, time.Second) {
		t.Fatal("an empty join must return at once")
	}
}
