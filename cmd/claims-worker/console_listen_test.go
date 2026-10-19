// Purpose: issue #83 regression — the comment-bridge listener must fail startup synchronously (before
// claims_worker_ready) when its address cannot be bound, and a clean Shutdown must not count as a failure.
// Depends on: startConsoleServer (main.go), net/http, loopback TCP only (MOCK tier, no database).
// Used by: go test ./cmd/claims-worker (Foundation unit shard).
package main

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"
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
