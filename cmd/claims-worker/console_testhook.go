//go:build claimsconsoletest

// Purpose: test-only build tag hook. No earlier than one second after the comment bridge binds, and only once
// claims_worker_ready has been written, it closes the bound listener so Serve fails with a real non-ErrServerClosed error on
// the post-ready path; lets tests/foundation prove run() exits with claims_worker_console_failed without racing readiness
// (#116 P2-1, #147 CW-2). Never in a release build.
// Depends on: consoleBound (main.go), log/slog, net, os, sync, time.
// Used by: tests/foundation/claims_worker_console_process_test.go (go build -tags claimsconsoletest); tests/ci/release-tag-guard.test.mjs keeps the tag out of every release path.
package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"sync"
	"time"
)

// consoleHookWindow keeps the old one-second observation window: while the bridge is bound, the process test reads the
// OS-chosen addr from claims_worker_console_ready before the hook makes Serve fail.
const consoleHookWindow = time.Second

// readyCloseGate closes the bound listener once the observation window has passed AND the ready record has been written.
// Closing only after both makes the post-ready exit deterministic instead of racing River Start's database probe (#147 CW-2).
type readyCloseGate struct {
	mu    sync.Mutex
	ln    net.Listener
	armed bool // consoleHookWindow elapsed
	ready bool // claims_worker_ready written
}

func (g *readyCloseGate) bind(ln net.Listener) {
	g.mu.Lock()
	g.ln = ln
	g.mu.Unlock()
	time.AfterFunc(consoleHookWindow, g.arm)
}

func (g *readyCloseGate) arm() {
	g.mu.Lock()
	g.armed = true
	ln := g.take()
	g.mu.Unlock()
	if ln != nil {
		_ = ln.Close()
	}
}

func (g *readyCloseGate) readyWritten() {
	g.mu.Lock()
	g.ready = true
	ln := g.take()
	g.mu.Unlock()
	if ln != nil {
		_ = ln.Close()
	}
}

// take returns the listener when both gates are open and clears it, so Close runs once.
func (g *readyCloseGate) take() net.Listener {
	if !g.armed || !g.ready || g.ln == nil {
		return nil
	}
	ln := g.ln
	g.ln = nil
	return ln
}

// readyCloseHandler forwards every record first: claims_worker_ready is already on stderr when the gate closes the listener,
// so the process log orders readiness before the failure.
type readyCloseHandler struct {
	next slog.Handler
	gate *readyCloseGate
}

func (h *readyCloseHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *readyCloseHandler) Handle(ctx context.Context, r slog.Record) error {
	err := h.next.Handle(ctx, r)
	if r.Message == "claims_worker_ready" {
		h.gate.readyWritten()
	}
	return err
}

func (h *readyCloseHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &readyCloseHandler{next: h.next.WithAttrs(attrs), gate: h.gate}
}

func (h *readyCloseHandler) WithGroup(name string) slog.Handler {
	return &readyCloseHandler{next: h.next.WithGroup(name), gate: h.gate}
}

func init() {
	gate := &readyCloseGate{}
	consoleBound = gate.bind
	// A fresh TextHandler, not slog.Default().Handler(): the built-in default handler writes through the standard log
	// package, whose output slog.SetDefault redirects back into this handler, so wrapping it recurses.
	slog.SetDefault(slog.New(&readyCloseHandler{next: slog.NewTextHandler(os.Stderr, nil), gate: gate}))
}
