// offline.go owns the merchant routes of the bank_transfer payment mode (contracts/storefront-v2.md §C): the store's bank-transfer
// settings, one order's transfer detail, and the three audited decisions (confirm, reject the submission, record an offline refund).
// Each route is a thin transport gate (exact method, no query, canonical ids, strict body, Idempotency-Key exactly where it writes) around one
// internal/merchantorders function; the SQL definers of migration 0088 decide every rule and permission. The admin BFF mirrors these under
// /api/admin/. Route -> Go endpoint: these handlers ARE the Go endpoints.
//
// Non-goals: no business rule, no PSP call, no bank detail or buyer email in a log or an error body (a coded refusal returns only its
// contract code). The codes live in internal/httperror's table (ruling 15). Mounted unconditionally by NewHandler (shipments precedent).

package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/merchantorders"
	"livecommerce/internal/platform"
	"livecommerce/internal/promotions"
)

var (
	offlineSettingsFields = []string{"expected_version", "enabled", "allow_cvs", "bank_name", "branch", "account_name", "account_number", "window_hours"}
	offlineRejectFields   = []string{"reason"}
)

func registerOfflinePaymentRoutes(mux *http.ServeMux, pool *pgxpool.Pool) {
	const base = "/v1/admin/stores/{store_id}"
	const order = base + "/orders/{order_id}/bank-transfer"
	mux.HandleFunc("GET "+base+"/bank-transfer-settings", cvsRoute(http.MethodGet, false, func(w http.ResponseWriter, r *http.Request) {
		offlineServe(w, r, pool, "integration:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope) (any, error) {
			return merchantorders.ReadTransferSettings(ctx, tx, s, bearerToken(r))
		})
	}))
	mux.HandleFunc("PUT "+base+"/bank-transfer-settings", cvsRoute(http.MethodPut, true, func(w http.ResponseWriter, r *http.Request) {
		in, ok := cvsStrictBody[merchantorders.TransferSettingsInput](w, r, offlineSettingsFields, nil)
		if !ok {
			return
		}
		offlineServe(w, r, pool, "integration:manage", func(ctx context.Context, tx pgx.Tx, s platform.Scope) (any, error) {
			return merchantorders.SetTransferSettings(ctx, tx, s, bearerToken(r), r.Header.Get("Idempotency-Key"), in)
		})
	}))
	mux.HandleFunc("GET "+order, cvsRoute(http.MethodGet, false, func(w http.ResponseWriter, r *http.Request) {
		offlineServe(w, r, pool, "orders:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope) (any, error) {
			return merchantorders.ReadTransfer(ctx, tx, s, bearerToken(r), r.PathValue("order_id"))
		})
	}))
	// confirm and refund-offline carry an empty JSON object: the order id in the path and the idempotency key are the whole request.
	for _, action := range []struct{ path, name string }{{"/confirm", "confirm"}, {"/refund-offline", "refund_offline"}} {
		mux.HandleFunc("POST "+order+action.path, cvsRoute(http.MethodPost, true, func(w http.ResponseWriter, r *http.Request) {
			if _, ok := cvsStrictBody[struct{}](w, r, nil, nil); !ok {
				return
			}
			offlineServe(w, r, pool, "payments:refund", func(ctx context.Context, tx pgx.Tx, s platform.Scope) (any, error) {
				return merchantorders.DecideTransfer(ctx, tx, s, bearerToken(r), r.Header.Get("Idempotency-Key"), r.PathValue("order_id"), action.name, nil)
			})
		}))
	}
	mux.HandleFunc("POST "+order+"/reject", cvsRoute(http.MethodPost, true, func(w http.ResponseWriter, r *http.Request) {
		in, ok := cvsStrictBody[merchantorders.RejectInput](w, r, offlineRejectFields, nil)
		if !ok {
			return
		}
		offlineServe(w, r, pool, "payments:refund", func(ctx context.Context, tx pgx.Tx, s platform.Scope) (any, error) {
			return merchantorders.DecideTransfer(ctx, tx, s, bearerToken(r), r.Header.Get("Idempotency-Key"), r.PathValue("order_id"), "reject", &in.Reason)
		})
	}))
}

// offlineServe runs fn in one platform.WithScope transaction opened with permission; a nil return is the COMMIT acknowledgement every 2xx
// waits for. It writes the classified error (never a driver message) on failure.
func offlineServe(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool, permission string, fn func(context.Context, pgx.Tx, platform.Scope) (any, error)) {
	ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
	defer cancel()
	var result any
	err := platform.WithScope(ctx, pool, bearerToken(r), r.PathValue("store_id"), permission, func(tx pgx.Tx, s platform.Scope) error {
		var inner error
		result, inner = fn(ctx, tx, s)
		return inner
	})
	if err != nil {
		status, code := offlineClassify(err)
		respondError(w, status, code)
		return
	}
	respond(w, http.StatusOK, result)
}

// offlineClassify maps the coded refusals of the 0088 (and 0091 promotions) definers first, then the shared claims table (authority, not found, conflict,
// deadlock and unknown -> 503).
func offlineClassify(err error) (int, string) {
	var coded *merchantorders.TransferError
	if errors.As(err, &coded) {
		return coded.Status, coded.Code
	}
	var promo *promotions.Coded // storefront-v2 §F coded refusals (promo_exists, invalid_promotion, version_changed, idempotency_conflict)
	if errors.As(err, &promo) {
		return promo.Status, promo.Code
	}
	return claimsClassify(err)
}
