// Purpose: the merchant meta connection-health HTTP adapter (contract meta-connection-health-v1 §9): GET
// /v1/admin/stores/{store_id}/meta/health (B1, the store's per-Page health banner model, permission store:read) and POST
// /meta/health/recheck (B2, integration:manage, body exactly {page_id}, 202 {next_check_at}). The banner never gates
// anything: B1 runs store:read like billing standing; B2 is not a command (no Idempotency-Key) because it only pulls
// next_due_at earlier, so it is naturally idempotent.
// Depends on: metaconnect.Health (Status/Recheck), platform.WithScope, metaConnectScope/metaConnectClassify (shared refusal
// mapping: 404 not_found, 429 recheck_too_soon), the Studio private no-store boundary (adsRoute/studioRoute).
// Used by: cmd/api via registerMetaHealthRoutes (Options.MetaHealth); tests internal/httpapi (MCH09 route test).
// Invariants: server-resolved tenant/store only (I01); the response never carries a token, scopes dump or Graph body; B1
// and B2 keep Cache-Control: private, no-store and strict JSON; any B1 read failure shows no banner (a 5xx stays server-side).
// Status: MOCK (REAL_PG + fake Graph; LIVE probe is MCH12, NOT_RUN).

package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/metaconnect"
	"livecommerce/internal/platform"
)

var metaHealthRecheckFields = []string{"page_id"}

// registerMetaHealthRoutes mounts the health banner surface; a nil service leaves it unmounted (cmd/api builds it only when
// the meta-connect surface is on, since the reader shares its snapshot).
func registerMetaHealthRoutes(mux *http.ServeMux, pool *pgxpool.Pool, svc *metaconnect.Health) {
	if svc == nil {
		return
	}
	const base = "/v1/admin/stores/{store_id}/meta/health"

	// B1: the banner read model (store:read, like billing standing — every member sees the banner).
	mux.HandleFunc("GET "+base, adsRoute(http.MethodGet, false, false, func(w http.ResponseWriter, r *http.Request) {
		metaConnectScope(w, r, pool, "store:read", http.StatusOK, func(ctx context.Context, tx pgx.Tx, s platform.Scope) (any, error) {
			return svc.Status(ctx, tx, s)
		})
	}))
	// B2: manual re-check (integration:manage). Body exactly {page_id}; no Idempotency-Key (not a command).
	mux.HandleFunc("POST "+base+"/recheck", adsRoute(http.MethodPost, false, false, func(w http.ResponseWriter, r *http.Request) {
		in, ok := claimsBody[struct {
			PageID string `json:"page_id"`
		}](w, r, metaHealthRecheckFields, nil)
		if !ok {
			return
		}
		metaConnectScope(w, r, pool, "integration:manage", http.StatusAccepted, func(ctx context.Context, tx pgx.Tx, s platform.Scope) (any, error) {
			next, err := svc.Recheck(ctx, tx, s, bearerToken(r), in.PageID)
			if err != nil {
				return nil, err
			}
			return map[string]string{"next_check_at": next.UTC().Format(time.RFC3339Nano)}, nil
		})
	}))
	// Methodless fallbacks keep 405 inside the same private no-store boundary.
	for _, suffix := range []string{"", "/recheck"} {
		mux.HandleFunc(base+suffix, studioRoute("", false, nil))
	}
}
