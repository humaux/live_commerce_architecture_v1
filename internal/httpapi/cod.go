// cod.go owns the merchant routes of the cash_on_delivery payment mode (home-cod R5, migration 0107): the store's cash-on-delivery
// settings. Each route is a thin transport gate (exact method, no query, canonical ids, strict body, Idempotency-Key exactly where it
// writes) around one internal/merchantorders function; the SQL definers of migration 0107 decide every rule and permission. The admin
// BFF mirrors these under /api/admin/.
//
// Non-goals: no business rule, no carrier API call, no PSP call, no amount from the client (I05); a coded refusal returns only its
// contract code. Mounted unconditionally by NewHandler (offline payment precedent).

package httpapi

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/merchantorders"
	"livecommerce/internal/platform"
)

var codSettingsFields = []string{"expected_version", "enabled", "max_twd", "surcharge_twd", "carrier"}

func registerCodPaymentRoutes(mux *http.ServeMux, pool *pgxpool.Pool) {
	const base = "/v1/admin/stores/{store_id}"
	mux.HandleFunc("GET "+base+"/cash-on-delivery-settings", cvsRoute(http.MethodGet, false, func(w http.ResponseWriter, r *http.Request) {
		offlineServe(w, r, pool, "integration:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope) (any, error) {
			return merchantorders.ReadCodSettings(ctx, tx, s, bearerToken(r))
		})
	}))
	mux.HandleFunc("PUT "+base+"/cash-on-delivery-settings", cvsRoute(http.MethodPut, true, func(w http.ResponseWriter, r *http.Request) {
		in, ok := cvsStrictBody[merchantorders.CodSettingsInput](w, r, codSettingsFields, nil)
		if !ok {
			return
		}
		offlineServe(w, r, pool, "integration:manage", func(ctx context.Context, tx pgx.Tx, s platform.Scope) (any, error) {
			return merchantorders.SetCodSettings(ctx, tx, s, bearerToken(r), r.Header.Get("Idempotency-Key"), in)
		})
	}))
}
