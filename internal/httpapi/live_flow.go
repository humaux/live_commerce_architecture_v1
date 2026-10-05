// Purpose: A5 live-session flow HTTP adapter (backend-only, docs/delivery/units/live-a5-session-flow.md): GET /live-sessions/results (A5-1 order/money read model), POST /live-sessions/{id}/copy (A5-2 one-click copy), and the page-live-videos read/plan pair (A5-3, MOCK). All routes are private no-store; the store scope is pinned server-side by platform.WithScope, never from a header or body. The API never loads a Page token here — the plan only mints a read-only operation the claims worker dispatches.
// Depends on: claimsRoute/claimsScoped/claimsBodyRoute (claims.go), studioRoute/studioBodyRoute (studio.go), live.Results/CopySession/GetPageLiveVideos/ReadPageLiveVideos (internal/live), command.ValidID, river.
// Used by: handler.go NewHandler → registerLiveFlowRoutes (gated on Studio/Live and LiveFlowJobs).
package httpapi

import (
	"context"
	"net/http"
	"net/url"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"livecommerce/internal/command"
	"livecommerce/internal/live"
	"livecommerce/internal/platform"
)

// registerLiveFlowRoutes mounts the four A5 rows plus methodless 405 fallbacks. enabled gates the
// whole family on the live-session domain being on (Studio implies it); jobs gates only the POST
// page-live-videos/read row, which needs the insert-only main-schema River client (nil = unmounted).
func registerLiveFlowRoutes(mux *http.ServeMux, pool *pgxpool.Pool, enabled bool, jobs *river.Client[pgx.Tx]) {
	if !enabled {
		return
	}
	const base = "/v1/admin/stores/{store_id}/live-sessions"

	mux.HandleFunc("GET "+base+"/results", claimsRoute(http.MethodGet, true, func(w http.ResponseWriter, r *http.Request) {
		ids, ok := liveFlowSessionIDs(r.URL)
		if !ok {
			respondError(w, http.StatusUnprocessableEntity, "invalid_request")
			return
		}
		claimsScoped(pool, "live:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return live.Results(ctx, tx, s, bearerToken(r), ids)
		})(w, r)
	}))

	// studioBodyRoute (not claimsBodyRoute) so scheduled_at may be null, exactly like DraftInput;
	// canonicalCopyInput re-checks title and the year bound inside the aggregate.
	mux.HandleFunc("POST "+base+"/{session_id}/copy", claimsRoute(http.MethodPost, false, studioBodyRoute(pool, "live:manage", []string{"title", "scheduled_at", "expected_version"}, func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request, in live.CopyInput) (any, error) {
		return live.CopySession(ctx, tx, s, bearerToken(r), r.Header.Get("Idempotency-Key"), r.PathValue("session_id"), in)
	})))

	mux.HandleFunc("GET "+base+"/{session_id}/page-live-videos", claimsRoute(http.MethodGet, true, func(w http.ResponseWriter, r *http.Request) {
		bindingID, ok := liveFlowBindingID(r.URL)
		if !ok {
			respondError(w, http.StatusUnprocessableEntity, "invalid_request")
			return
		}
		claimsScoped(pool, "live:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return live.GetPageLiveVideos(ctx, tx, s, bearerToken(r), bindingID)
		})(w, r)
	}))

	if jobs != nil {
		mux.HandleFunc("POST "+base+"/{session_id}/page-live-videos/read", claimsRoute(http.MethodPost, false, claimsBodyRoute(pool, []string{"binding_id"}, nil, nil, func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request, in live.PageLiveVideosReadInput) (any, error) {
			return live.ReadPageLiveVideos(ctx, tx, s, bearerToken(r), r.Header.Get("Idempotency-Key"), in.BindingID, jobs)
		})))
	}

	// Methodless fallbacks keep wrong-method answers inside the same private response boundary.
	// base+"/results" cannot be methodless: it is more path-specific than, but less method-specific
	// than, studio's "GET base/{session_id}" (both three segments), which Go 1.22's ServeMux rejects
	// as ambiguous. Register its wrong-method fallbacks with explicit methods instead; the longer
	// copy/picker paths have no equal-length studio sibling, so they stay methodless.
	for _, path := range []string{base + "/{session_id}/copy", base + "/{session_id}/page-live-videos"} {
		mux.HandleFunc(path, studioRoute("", false, nil))
	}
	for _, m := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		mux.HandleFunc(m+" "+base+"/results", studioRoute("", false, nil))
	}
	if jobs != nil {
		mux.HandleFunc(base+"/{session_id}/page-live-videos/read", studioRoute("", false, nil))
	}
}

// liveFlowSessionIDs parses the A5-1 query: 1..50 distinct canonical session_id values and nothing
// else (the SQL definer returns one item per id in request order).
func liveFlowSessionIDs(u *url.URL) ([]string, bool) {
	if u.ForceQuery || len(u.RawQuery) > 4096 || u.RawQuery == "" {
		return nil, false
	}
	values, err := url.ParseQuery(u.RawQuery)
	if err != nil || len(values) != 1 {
		return nil, false
	}
	ids, ok := values["session_id"]
	if !ok || len(ids) < 1 || len(ids) > 50 {
		return nil, false
	}
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if !command.ValidID(id) || seen[id] {
			return nil, false
		}
		seen[id] = true
	}
	return ids, true
}

// liveFlowBindingID parses the A5-3 GET query: exactly one canonical binding_id, nothing else.
func liveFlowBindingID(u *url.URL) (string, bool) {
	if u.ForceQuery || len(u.RawQuery) > 4096 || u.RawQuery == "" {
		return "", false
	}
	values, err := url.ParseQuery(u.RawQuery)
	if err != nil || len(values) != 1 || len(values["binding_id"]) != 1 {
		return "", false
	}
	id := values["binding_id"][0]
	if !command.ValidID(id) {
		return "", false
	}
	return id, true
}
