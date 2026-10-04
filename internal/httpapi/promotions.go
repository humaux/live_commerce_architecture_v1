// promotions.go owns the merchant routes of discount codes (contracts/storefront-v2.md §F): list, create, update/pause. Each route is a thin
// transport gate (exact method, no query, canonical ids, strict body, Idempotency-Key exactly where it writes) around one internal/promotions
// function; the SQL definers of migration 0091 decide every rule and permission. The admin BFF mirrors these under /api/stores/{store}/promotions.
// Route -> Go endpoint: these handlers ARE the Go endpoints. The buyer side has no route of its own: a code rides the existing quote request
// (buyerhttp POST /v1/buyer/quotes `promo_code`) and BeginCheckout.
//
// Non-goals: no business rule, no buyer data, no code text in a log or error body (a coded refusal returns only its contract code). Serving
// and error classification are offline.go's offlineServe/offlineClassify (same transaction + coded-refusal contract).

package httpapi

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/command"
	"livecommerce/internal/platform"
	"livecommerce/internal/promotions"
)

func registerPromotionRoutes(mux *http.ServeMux, pool *pgxpool.Pool) {
	const base = "/v1/admin/stores/{store_id}/promotions"
	mux.HandleFunc("GET "+base, cvsRoute(http.MethodGet, false, func(w http.ResponseWriter, r *http.Request) {
		offlineServe(w, r, pool, "pricing:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope) (any, error) {
			list, err := promotions.List(ctx, tx, s, bearerToken(r))
			return map[string]any{"promotions": list}, err
		})
	}))
	mux.HandleFunc("POST "+base, cvsRoute(http.MethodPost, true, func(w http.ResponseWriter, r *http.Request) {
		in, ok := cvsStrictBody[promotions.CreateInput](w, r, promotions.CreateFields, promotions.Nullable)
		if !ok {
			return
		}
		offlineServe(w, r, pool, "pricing:write", func(ctx context.Context, tx pgx.Tx, s platform.Scope) (any, error) {
			return promotions.Create(ctx, tx, s, bearerToken(r), r.Header.Get("Idempotency-Key"), in)
		})
	}))
	mux.HandleFunc("POST "+base+"/{promotion_id}", cvsRoute(http.MethodPost, true, func(w http.ResponseWriter, r *http.Request) {
		if !command.ValidID(r.PathValue("promotion_id")) {
			respondError(w, http.StatusUnprocessableEntity, "invalid_request")
			return
		}
		in, ok := cvsStrictBody[promotions.UpdateInput](w, r, promotions.UpdateFields, promotions.Nullable)
		if !ok {
			return
		}
		offlineServe(w, r, pool, "pricing:write", func(ctx context.Context, tx pgx.Tx, s platform.Scope) (any, error) {
			return promotions.Update(ctx, tx, s, bearerToken(r), r.Header.Get("Idempotency-Key"), r.PathValue("promotion_id"), in)
		})
	}))
}
