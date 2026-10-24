// Purpose: one shared bounded HTTP readiness probe for the real-process tests in this package.
// Depends on: net/http, net/http/httptest (witness), mrProcess (meta_runtime_process_test.go), time.
// Used by: local_recovery_test.go, meta_runtime_process_test.go, account_process_test.go,
//
//	merchant_restart_replay_test.go, studio_planning_only_test.go, studio_process_test.go and
//	order_attempt_process_test.go.
package foundation_test

import (
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

// pollHTTPReady polls target until it answers 200. The caller owns the shared
// readiness deadline; each individual request is capped by perRequest.
func pollHTTPReady(proc *mrProcess, target string, perRequest, interval time.Duration, deadline time.Time) error {
	client := &http.Client{Timeout: perRequest}
	for time.Now().Before(deadline) {
		if proc != nil {
			select {
			case err := <-proc.done:
				proc.exited = true
				return fmt.Errorf("%w: %v", errHTTPReadyExited, err)
			default:
			}
		}
		resp, err := client.Get(target)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(interval)
	}
	return errHTTPReadyDeadline
}

// waitHTTPReady is pollHTTPReady's fatal wrapper for process tests.
func waitHTTPReady(t *testing.T, proc *mrProcess, target, label string, perRequest, interval time.Duration, deadline time.Time) {
	t.Helper()
	if err := pollHTTPReady(proc, target, perRequest, interval, deadline); err != nil {
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
	defer close(release)

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
