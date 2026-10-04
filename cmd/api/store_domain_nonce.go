package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/httperror"
)

// noncePathPrefix is the per-row TLS proof path (R5 store-domains P1-2): Caddy routes it from the catch-all site
// to this api for a TLS_PENDING host, and the worker fetches it with SNI = the host and compares the body.
const noncePathPrefix = "/.well-known/lc-domain-check/"

// nonceQuerier is the one call the nonce endpoint needs; *pgxpool.Pool satisfies it, tests fake it.
type nonceQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// buildStoreDomainNonceHandler builds the per-row TLS nonce endpoint on the runtime pool, which logs in as
// commerce_runtime and holds EXECUTE on control.tls_pending_nonce (0106). It answers 200 with the nonce body only
// when r.Host is a TLS_PENDING origin whose tls_nonce equals the path nonce; every other host, nonce or method is
// 404 (fail closed, no oracle). The host comes from the edge-set Host (the SNI Caddy forwarded), never a header a
// client controls beyond the SNI itself.
func buildStoreDomainNonceHandler(pool *pgxpool.Pool) http.Handler {
	return buildStoreDomainNonceHandlerOn(pool)
}

// buildStoreDomainNonceHandlerOn is buildStoreDomainNonceHandler over a nonceQuerier (test seam for the pool path).
func buildStoreDomainNonceHandlerOn(q nonceQuerier) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			httperror.Write(w, http.StatusMethodNotAllowed, "method_not_allowed")
			return
		}
		nonce := nonceFromPath(r.URL.Path)
		host := requestHostname(r.Host)
		if nonce == "" || host == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var ok bool
		// control.tls_pending_nonce (0106): true only for a TLS_PENDING origin whose tls_nonce equals p_nonce.
		if err := q.QueryRow(r.Context(), `SELECT control.tls_pending_nonce($1, $2)`, host, nonce).Scan(&ok); err != nil {
			httperror.Write(w, http.StatusServiceUnavailable, "unavailable")
			return
		}
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, nonce)
	})
}

// mountStoreDomainNonce serves /.well-known/lc-domain-check/* from nonce and everything else from next. It is the OUTERMOST
// wrapper in main.go, so it must never be a ServeMux: a mux cleans "//" and ".." and answers 307/301 to the canonical path
// before mountMeta/mountStripe/mountPlatformBilling (which reserve their namespaces precisely to see the original, uncleaned
// request and refuse aliases with 404) can run. Match the literal prefix and hand every other request on untouched, like mountTLSAsk.
func mountStoreDomainNonce(next, nonce http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, noncePathPrefix) {
			nonce.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// nonceFromPath returns the single nonce segment of /.well-known/lc-domain-check/<nonce>, or "" when the path is
// not exactly that shape (empty nonce, a trailing slash, or an extra segment).
func nonceFromPath(p string) string {
	if !strings.HasPrefix(p, noncePathPrefix) {
		return ""
	}
	nonce := strings.TrimPrefix(p, noncePathPrefix)
	if nonce == "" || strings.Contains(nonce, "/") {
		return ""
	}
	return nonce
}

// requestHostname strips any port from the Host header and lower-cases it; the SQL nonce/ask checks already reject
// a non-hostname string, so this only normalizes the SNI host Caddy forwards.
func requestHostname(host string) string {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return strings.ToLower(strings.TrimSpace(host))
}
