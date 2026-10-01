// notify.go owns the merchant route of the new-order mail opt-out (contracts/storefront-v2.md §E6): GET|PUT
// /v1/admin/stores/{store_id}/notification-settings. A thin transport gate (exact method, no query, canonical ids, strict body, a keyed PUT
// like every other settings write) around internal/notify.ReadSettings / SetSettings; the SQL definers of migration 0090 authorize
// (integration:read / integration:manage) and decide. The admin BFF mirrors it under /api/stores/{store}/notification-settings.
//
// It never carries a mail address or a body, and the PUT is a plain idempotent set: the Idempotency-Key is validated (so a retry cannot
// be confused with a second command) but needs no stored receipt. Mounted unconditionally by NewHandler (offline.go precedent).

package httpapi

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/notify"
	"livecommerce/internal/platform"
)

func registerNotifySettingsRoutes(mux *http.ServeMux, pool *pgxpool.Pool) {
	const path = "/v1/admin/stores/{store_id}/notification-settings"
	mux.HandleFunc("GET "+path, cvsRoute(http.MethodGet, false, func(w http.ResponseWriter, r *http.Request) {
		offlineServe(w, r, pool, "integration:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope) (any, error) {
			return notify.ReadSettings(ctx, tx, s, bearerToken(r))
		})
	}))
	mux.HandleFunc("PUT "+path, cvsRoute(http.MethodPut, true, func(w http.ResponseWriter, r *http.Request) {
		in, ok := cvsStrictBody[notify.Settings](w, r, []string{"merchant_new_order_email"}, nil)
		if !ok {
			return
		}
		offlineServe(w, r, pool, "integration:manage", func(ctx context.Context, tx pgx.Tx, s platform.Scope) (any, error) {
			return notify.SetSettings(ctx, tx, s, bearerToken(r), in.MerchantNewOrderEmail)
		})
	}))
}
