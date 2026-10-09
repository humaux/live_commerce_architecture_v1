// Purpose: live-console-v1 §2.6 / §7.4 — the console comment HTTP adapter (unit LC-B2): A2 GET
// /live-sessions/{sid}/comments and A3 POST /live-sessions/{sid}/comments/{ref}/print. Every response is
// Cache-Control: no-store + Referrer-Policy: no-referrer (I15) because comment text/names are in the A2
// body and must never be cached or leaked by referrer. The store scope is pinned by platform.WithScope,
// never by a header or body; the query cursor values are bounded before any transaction opens.
// Depends on: claimsRoute/claimsBody/claimsScoped (claims.go), studioRoute (studio.go),
//
//	live.CommentStream (internal/live/stream.go).
//
// Used by: handler.go NewHandler → registerLiveStreamRoutes (gated on CommentStream).
package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/command"
	"livecommerce/internal/live"
	"livecommerce/internal/platform"
)

// registerLiveStreamRoutes mounts A2/A3 when the comment stream is configured (cmd/api builds it from the
// bridge client + the payload keyring). nil leaves the routes unmounted (404), never a disabled stub.
func registerLiveStreamRoutes(mux *http.ServeMux, pool *pgxpool.Pool, cs *live.CommentStream) {
	if cs == nil {
		return
	}
	const base = "/v1/admin/stores/{store_id}/live-sessions"

	// A2: one comment page. Query is parsed and bounded before a transaction; a malformed cursor value
	// answers 400 invalid_cursor, an unknown/duplicate key answers 422 invalid_request (§2.6).
	mux.HandleFunc("GET "+base+"/{session_id}/comments", claimsRoute(http.MethodGet, true, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		q, err := liveStreamQuery(r.URL)
		if err != nil {
			status, code := liveStreamClassify(err)
			respondError(w, status, code)
			return
		}
		liveStreamScoped(pool, "live:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return cs.Comments(ctx, tx, s, bearerToken(r), r.PathValue("session_id"), q)
		})(w, r)
	}))

	// A3: idempotent-per-key print fact (the required Idempotency-Key becomes the command.Run receipt key,
	// §7.4). Body is exactly {} (the label is rendered client-side); the comment_ref shape is re-checked in
	// the service (ErrInvalidRef → 422 invalid_ref).
	mux.HandleFunc("POST "+base+"/{session_id}/comments/{comment_ref}/print", claimsRoute(http.MethodPost, false, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		if _, ok := claimsBody[struct{}](w, r, nil, nil); !ok {
			return
		}
		liveStreamScoped(pool, "live:manage", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return cs.PrintComment(ctx, tx, s, bearerToken(r), r.Header.Get("Idempotency-Key"), r.PathValue("session_id"), r.PathValue("comment_ref"))
		})(w, r)
	}))

	// Methodless fallbacks keep wrong-method answers inside the same private response boundary.
	for _, path := range []string{base + "/{session_id}/comments", base + "/{session_id}/comments/{comment_ref}/print"} {
		mux.HandleFunc(path, studioRoute("", false, nil))
	}
}

// liveStreamQuery parses the A2 query: after_epoch/after_seq/limit 1..100 (default 50) and before_cursor.
// A non-integer or negative after_epoch/after_seq, or a malformed before_cursor, is live.ErrInvalidCursor
// (→ 400 invalid_cursor); every other query problem (unknown/duplicate key, oversized query) is
// command.ErrInvalid (→ 422 invalid_request), matching studioPage.
func liveStreamQuery(u *url.URL) (live.ConsolePageQuery, error) {
	q := live.ConsolePageQuery{Limit: 50}
	if u.ForceQuery || len(u.RawQuery) > 4096 {
		return q, command.ErrInvalid
	}
	if u.RawQuery == "" {
		return q, nil
	}
	values, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return q, command.ErrInvalid
	}
	for name, list := range values {
		if len(list) != 1 {
			return q, command.ErrInvalid
		}
		value := list[0]
		switch name {
		case "after_epoch":
			n, err := liveStreamSeq(value)
			if err != nil {
				return q, live.ErrInvalidCursor
			}
			q.AfterEpoch = &n
		case "after_seq":
			n, err := liveStreamSeq(value)
			if err != nil {
				return q, live.ErrInvalidCursor
			}
			q.AfterSeq = &n
		case "before_cursor":
			if value == "" || len(value) > 1024 || strings.ContainsAny(value, "%+=") {
				return q, live.ErrInvalidCursor
			}
			q.BeforeCursor = &value
		case "limit":
			if value == "" || strings.Trim(value, "0123456789") != "" {
				return q, command.ErrInvalid
			}
			n, err := strconv.Atoi(value)
			if err != nil || n < 1 || n > 100 {
				return q, command.ErrInvalid
			}
			q.Limit = n
		default:
			return q, command.ErrInvalid
		}
	}
	return q, nil
}

// liveStreamSeq parses a non-negative int64 cursor coordinate (digits only; no sign, no empty).
func liveStreamSeq(value string) (int64, error) {
	if value == "" || strings.Trim(value, "0123456789") != "" {
		return 0, command.ErrInvalid
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil || n < 0 {
		return 0, command.ErrInvalid
	}
	return n, nil
}

// liveStreamScoped is scoped with liveStreamClassify.
func liveStreamScoped(pool *pgxpool.Pool, permission string, fn action) http.HandlerFunc {
	return scopedAs(pool, permission, liveStreamClassify, fn)
}

// liveStreamClassify maps the console's fixed safe codes (§2.6/§7.4/§11) before the shared classifier:
// no_source → 409, stream_unavailable → 503, invalid_cursor → 400, invalid_ref → 422; everything else
// falls back to claimsClassify (which keeps deadlocks retryable and unknown errors 503, never 500).
func liveStreamClassify(err error) (int, string) {
	switch {
	case errors.Is(err, live.ErrNoSource):
		return http.StatusConflict, "no_source"
	case errors.Is(err, live.ErrStreamUnavailable):
		return http.StatusServiceUnavailable, "stream_unavailable"
	case errors.Is(err, live.ErrInvalidCursor):
		return http.StatusBadRequest, "invalid_cursor"
	case errors.Is(err, live.ErrInvalidRef):
		return http.StatusUnprocessableEntity, "invalid_ref"
	}
	return claimsClassify(err)
}
