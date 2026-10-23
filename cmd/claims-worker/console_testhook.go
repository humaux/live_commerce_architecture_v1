//go:build claimsconsoletest

// Purpose: TEMP RED witness for CW-2 (#147). It keeps the old pre-ready console failure and drops the readiness record, so the
// old test still passes (claims_worker_console_failed) while the new claims_worker_ready assertion cannot. Replaced in the next
// commit by the post-ready close gate that makes the pinned path deterministic.
// Depends on: consoleBound (main.go), log/slog, net, os.
// Used by: tests/foundation/claims_worker_console_process_test.go (go build -tags claimsconsoletest); tests/ci/release-tag-guard.test.mjs.
package main

import (
	"context"
	"log/slog"
	"net"
	"os"
)

type dropReady struct{ next slog.Handler }

func (h dropReady) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h dropReady) Handle(ctx context.Context, r slog.Record) error {
	if r.Message == "claims_worker_ready" {
		return nil
	}
	return h.next.Handle(ctx, r)
}

func (h dropReady) WithAttrs(attrs []slog.Attr) slog.Handler {
	return dropReady{next: h.next.WithAttrs(attrs)}
}

func (h dropReady) WithGroup(name string) slog.Handler {
	return dropReady{next: h.next.WithGroup(name)}
}

func init() {
	// A fresh TextHandler, not slog.Default().Handler(): the built-in default handler writes through the standard log
	// package, whose output slog.SetDefault redirects back into this handler, so wrapping it recurses.
	slog.SetDefault(slog.New(dropReady{next: slog.NewTextHandler(os.Stderr, nil)}))
	consoleBound = func(ln net.Listener) { _ = ln.Close() }
}
