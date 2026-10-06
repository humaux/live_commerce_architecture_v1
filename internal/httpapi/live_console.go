// Purpose: LC-B7 HTTP adapters of live-console-v1: A1 GET /v1/admin/stores/{store_id}/live-sessions/{session_id}/console (§7.1 read model, live:read) and the bounded stock edit POST .../inventory/adjustments (§7.2: inventory:write as before, or the narrow inventory:live_adjust with the bounds of inventory.AdjustOnHandBounded).
// Depends on: live.Console (internal/live/console.go), inventory.AdjustOnHand/AdjustOnHandBounded (internal/inventory), claimsRoute/scopedAs/bodyRouteAs/classify (claims.go, handler.go), studioRoute (studio.go), platform.WithScope.
// Used by: handler.go NewHandler -> registerLiveConsoleRoutes (gated like registerLiveLifecycleRoutes on the live-session domain) and the inventory block -> inventoryAdjustRoute.
// Invariants: store scope is pinned by platform.WithScope, never by a body/header field (I01); every response is private no-store; I03 (a live_adjust-only principal is always bounded: the permission probe below picks the route variant, the transaction re-authorizes whichever permission it chose).
package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/inventory"
	"livecommerce/internal/live"
	"livecommerce/internal/metaconnect"
	"livecommerce/internal/platform"
)

// registerLiveConsoleRoutes mounts A1 plus its methodless 405 fallback when the live-session domain is on (console may be nil only in tests: the
// route then 503s). Console reads are GETs: no body, no query, no Idempotency-Key (claimsRoute).
func registerLiveConsoleRoutes(mux *http.ServeMux, pool *pgxpool.Pool, enabled bool, console *live.Console) {
	if !enabled {
		return
	}
	const path = "/v1/admin/stores/{store_id}/live-sessions/{session_id}/console"
	mux.HandleFunc("GET "+path, claimsRoute(http.MethodGet, false, func(w http.ResponseWriter, r *http.Request) {
		scopedAs(pool, "live:read", claimsClassify, func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			if console == nil {
				return nil, live.ErrStreamUnavailable
			}
			return console.Read(ctx, tx, s, bearerToken(r), r.PathValue("session_id"))
		})(w, r)
	}))
	mux.HandleFunc(path, studioRoute("", false, nil))
}

// inventoryAdjustRoute serves POST /inventory/adjustments for both inventory:write and inventory:live_adjust principals (live-console-v1 §7.2).
// A short probe transaction asks whether the bearer holds inventory:write in this store; if not (platform.ErrForbidden) the request runs under
// inventory:live_adjust with the bounded variant. Any other probe outcome (unauthorized, unknown store) falls through to the inventory:write path so the
// usual 401/404 reaches the client. Neither probe result is trusted for the write: the real transaction resolves the chosen permission again, so a
// permission revoked in between simply makes that transaction 403.
func inventoryAdjustRoute(pool *pgxpool.Pool) http.HandlerFunc {
	write := bodyRouteAs(pool, "inventory:write", inventoryAdjustClassify,
		func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request, in inventory.Adjustment) (any, error) {
			return inventory.AdjustOnHand(ctx, tx, s, r.Header.Get("Idempotency-Key"), in)
		})
	bounded := bodyRouteAs(pool, "inventory:live_adjust", inventoryAdjustClassify,
		func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request, in inventory.Adjustment) (any, error) {
			return inventory.AdjustOnHandBounded(ctx, tx, s, r.Header.Get("Idempotency-Key"), in)
		})
	return func(w http.ResponseWriter, r *http.Request) {
		err := platform.WithScope(r.Context(), pool, bearerToken(r), r.PathValue("store_id"), "inventory:write", func(pgx.Tx, platform.Scope) error { return nil })
		if errors.Is(err, platform.ErrForbidden) {
			bounded(w, r)
			return
		}
		write(w, r)
	}
}

// inventoryAdjustClassify adds below_reserved (422) in front of the shared classifier.
func inventoryAdjustClassify(err error) (int, string) {
	if errors.Is(err, inventory.ErrBelowReserved) {
		return http.StatusUnprocessableEntity, "below_reserved"
	}
	return classify(err)
}

// capabilityReader is the §6 reader the console embeds: the meta connection-health reader when the surface is on, else none (empty capabilities).
func capabilityReader(h *metaconnect.Health) metaconnect.CapabilityReader {
	if h == nil || h.Reader == nil {
		return nil
	}
	return h.Reader
}
