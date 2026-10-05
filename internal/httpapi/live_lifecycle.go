// Purpose: A7 HTTP adapter (live-console-v1 §9/§11): POST /v1/admin/stores/{store_id}/live-sessions/{session_id}/lifecycle {action: start|end|archive, expected_version, open_window?} -> {lifecycle, version, window}. Private no-store; the store scope is pinned server-side by platform.WithScope (live:manage), never from a body field.
// Depends on: claimsRoute/claimsBody/scopedAs/claimsClassify (claims.go, handler.go), studioRoute (studio.go), live.Lifecycle (internal/live/lifecycle.go), claims.ErrInvalidTransition/ErrTooManyOpenWindows, command.ErrConflict.
// Used by: handler.go NewHandler -> registerLiveLifecycleRoutes (gated on Studio/Live like registerLiveFlowRoutes).
// Invariants: 409 codes are version_conflict | invalid_transition | too_many_open_windows, 402 billing_restricted (claimsClassify), 404 not_found, 403 forbidden (a live:read principal cannot transition).
package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/claims"
	"livecommerce/internal/command"
	"livecommerce/internal/live"
	"livecommerce/internal/platform"
)

// registerLiveLifecycleRoutes mounts A7 plus the methodless 405 fallback. enabled gates it on the live-session domain
// being on (Studio implies it), exactly like registerLiveFlowRoutes. Optional body key open_window may be absent, never null.
func registerLiveLifecycleRoutes(mux *http.ServeMux, pool *pgxpool.Pool, enabled bool) {
	if !enabled {
		return
	}
	const path = "/v1/admin/stores/{store_id}/live-sessions/{session_id}/lifecycle"
	mux.HandleFunc("POST "+path, claimsRoute(http.MethodPost, false, func(w http.ResponseWriter, r *http.Request) {
		in, ok := claimsBody[live.LifecycleInput](w, r, []string{"action", "expected_version"}, nil, "open_window")
		if !ok {
			return
		}
		scopedAs(pool, "live:manage", lifecycleClassify, func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return live.Lifecycle(ctx, tx, s, bearerToken(r), r.Header.Get("Idempotency-Key"), r.PathValue("session_id"), in)
		})(w, r)
	}))
	mux.HandleFunc(path, studioRoute("", false, nil))
}

// lifecycleClassify adds the A7 conflict codes in front of claimsClassify; every other error keeps the claims mapping
// (402 billing_restricted, 403, 404, 422, 503). A bare command.ErrConflict on this route is the lifecycle_version CAS
// (or an Idempotency-Key reused with a different body).
func lifecycleClassify(err error) (int, string) {
	switch {
	case errors.Is(err, claims.ErrInvalidTransition):
		return http.StatusConflict, "invalid_transition"
	case errors.Is(err, claims.ErrTooManyOpenWindows):
		return http.StatusConflict, "too_many_open_windows"
	case errors.Is(err, command.ErrConflict):
		return http.StatusConflict, "version_conflict"
	}
	return claimsClassify(err)
}
