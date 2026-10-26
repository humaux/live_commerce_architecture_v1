// Purpose: one shared bounded HTTP readiness probe for the real-process tests in this package.
// Depends on: net/http, net/http/httptest (witness), mrProcess (meta_runtime_process_test.go), time.
// Used by: local_recovery_test.go, meta_runtime_process_test.go, account_process_test.go,
// merchant_restart_replay_test.go, studio_planning_only_test.go, studio_process_test.go and
// order_attempt_process_test.go.
package foundation_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

var (
	errHTTPReadyDeadline = errors.New("readiness deadline expired")
	errHTTPReadyExited   = errors.New("process exited before readiness")
)

// httpReadyOption overrides one probe setting. Options exist so that a caller
// keeps the HTTP client it had before the probes were consolidated: the Meta
// process test needs a no-redirect client, every other caller keeps
// http.DefaultClient.
type httpReadyOption func(*httpReadyConfig)

type httpReadyConfig struct {
	client *http.Client
}

// withHTTPClient makes the probe use client instead of http.DefaultClient.
func withHTTPClient(client *http.Client) httpReadyOption {
	return func(cfg *httpReadyConfig) {
		if client != nil {
			cfg.client = client
		}
	}
}

func newHTTPReadyConfig(opts []httpReadyOption) httpReadyConfig {
	cfg := httpReadyConfig{client: http.DefaultClient}
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

// pollHTTPReady polls target until it answers 200 before the caller-owned shared
// deadline. Each request runs under a context bounded by min(perRequest,
// remaining budget), and a 200 is accepted only if it arrives before the
// deadline: a late response is stale evidence, not readiness.
func pollHTTPReady(proc *mrProcess, target string, perRequest, interval time.Duration, deadline time.Time, opts ...httpReadyOption) error {
	// RED witness: the option is wired here but the dispatch below still uses
	// http.DefaultClient, so the Meta no-redirect policy is not yet honoured.
	_ = newHTTPReadyConfig(opts)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return errHTTPReadyDeadline
		}
		if proc != nil {
			select {
			case err := <-proc.done:
				proc.exited = true
				return fmt.Errorf("%w: %v", errHTTPReadyExited, err)
			default:
			}
		}
		timeout := perRequest
		if remaining < timeout {
			timeout = remaining
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if err != nil {
			cancel()
			return err
		}
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			// Check expiry before accepting success: the 200 may have arrived
			// after the shared deadline while the request context was expiring.
			if resp.StatusCode == http.StatusOK && time.Now().Before(deadline) {
				cancel()
				return nil
			}
		}
		cancel()
		if !time.Now().Before(deadline) {
			return errHTTPReadyDeadline
		}
		pause := interval
		if left := time.Until(deadline); left < pause {
			pause = left
		}
		time.Sleep(pause)
	}
}

// waitHTTPReady is pollHTTPReady's fatal wrapper for process tests.
func waitHTTPReady(t *testing.T, proc *mrProcess, target, label string, perRequest, interval time.Duration, deadline time.Time, opts ...httpReadyOption) {
	t.Helper()
	if err := pollHTTPReady(proc, target, perRequest, interval, deadline, opts...); err != nil {
		if proc != nil {
			t.Fatalf("API %s readiness failed: %v log=%s", label, err, proc.logPath)
		}
		t.Fatalf("API %s readiness failed: %v", label, err)
	}
}

// TestPollHTTPReadyRejectsSuccessAfterDeadline is the unit witness for the
// shared-deadline guarantee: a server that holds the response until the
// deadline has passed must not produce a successful probe.
func TestPollHTTPReadyRejectsSuccessAfterDeadline(t *testing.T) {
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		select {
		case started <- struct{}{}:
		default:
		}
		<-release
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	deadline := time.Now().Add(100 * time.Millisecond)
	type result struct{ err error }
	done := make(chan result, 1)
	go func() { done <- result{pollHTTPReady(nil, server.URL, time.Second, 10*time.Millisecond, deadline)} }()

	<-started                                                // the first request is in flight
	<-time.After(time.Until(deadline) + 50*time.Millisecond) // ... and its 200 now arrives late.
	close(release)
	if res := <-done; !errors.Is(res.err, errHTTPReadyDeadline) {
		t.Fatalf("probe accepted a 200 that arrived after the shared deadline: %v", res.err)
	}
}

// TestPollHTTPReadyAcceptsFastResponse is the positive control for the witness.
func TestPollHTTPReadyAcceptsFastResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	if err := pollHTTPReady(nil, server.URL, time.Second, 10*time.Millisecond, time.Now().Add(time.Second)); err != nil {
		t.Fatalf("fast 200 was not accepted: %v", err)
	}
}

// TestPollHTTPReadyHonoursNoRedirectClient is the negative control for the Meta
// readiness policy: /healthz answers 302 to a page that returns 200. The Meta
// process test has always probed with a client whose CheckRedirect returns
// http.ErrUseLastResponse, so a redirect must never count as ready — otherwise a
// routing or middleware regression would satisfy readiness.
func TestPollHTTPReadyHonoursNoRedirectClient(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/routed-ok", http.StatusFound)
	})
	mux.HandleFunc("/routed-ok", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	noRedirect := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	deadline := time.Now().Add(250 * time.Millisecond)
	err := pollHTTPReady(nil, server.URL+"/healthz", time.Second, 20*time.Millisecond, deadline, withHTTPClient(noRedirect))
	if !errors.Is(err, errHTTPReadyDeadline) {
		t.Fatalf("Meta readiness accepted a 200 reached through a redirect: %v", err)
	}
}

// TestPollHTTPReadyExpiredDeadlineSendsNoRequest pins the pre-request expiry check.
func TestPollHTTPReadyExpiredDeadlineSendsNoRequest(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	err := pollHTTPReady(nil, server.URL, time.Second, time.Millisecond, time.Now().Add(-time.Millisecond))
	if !errors.Is(err, errHTTPReadyDeadline) || calls.Load() != 0 {
		t.Fatalf("expired deadline: err=%v requests=%d", err, calls.Load())
	}
}
