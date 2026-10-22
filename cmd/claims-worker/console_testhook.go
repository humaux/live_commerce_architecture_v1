//go:build claimsconsoletest

// Purpose: test-only build tag hook — one second after the comment bridge binds, close its listener so Serve fails with a real
// non-ErrServerClosed error; lets tests/foundation prove run() exits with claims_worker_console_failed (#116). Never in a release build.
// Depends on: consoleBound (main.go), net, time.
// Used by: tests/foundation/claims_worker_console_process_test.go (go build -tags claimsconsoletest); tests/ci/release-tag-guard.test.mjs keeps the tag out of every release path.
package main

import (
	"net"
	"time"
)

func init() {
	consoleBound = func(ln net.Listener) { time.AfterFunc(time.Second, func() { _ = ln.Close() }) }
}
