// Purpose: merchant read/approve/reject of buyer applications, separate from legacy RMA transport/parser.
// Depends on: returnScope, shipmentRoute, parcelKey, studioDecodeRaw and returns request wrappers (0210).
// Used by: registerReturnRoutes in NewHandler; REAL_PG TestBuyerReturnRequests.
// Invariants: orders:read for read; fulfillment:write for decisions; strict input; COMMIT before output; no refund.
package httpapi

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
	"livecommerce/internal/returns"
)

func requestRoute(method string, next http.HandlerFunc) http.HandlerFunc {
	return shipmentRoute(method, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "private, no-store")
		if id := r.PathValue("request_id"); id != "" && !command.ValidID(id) {
			respondError(w, 422, "invalid_request")
			return
		}
		next(w, r)
	})
}
func registerBuyerReturnRequestRoutes(mux *http.ServeMux, pool *pgxpool.Pool) {
	const base = "/v1/admin/stores/{store_id}/return-requests"
	read := func(w http.ResponseWriter, r *http.Request) {
		if len(r.Header.Values("Idempotency-Key")) != 0 || (r.URL.RawQuery != "" || r.URL.ForceQuery) {
			respondError(w, 422, "invalid_request")
			return
		}
		result, ok := returnScope(w, r, pool, "orders:read", func(c context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return returns.ReadBuyerRequests(c, tx, bearerToken(r), s.StoreID, r.PathValue("request_id"), true, s)
		})
		if ok {
			respond(w, 200, result)
		}
	}
	mux.HandleFunc("GET "+base, requestRoute(http.MethodGet, read))
	mux.HandleFunc("GET "+base+"/{request_id}", requestRoute(http.MethodGet, read))
	mux.HandleFunc("POST "+base+"/{request_id}/approve", requestRoute(http.MethodPost, func(w http.ResponseWriter, r *http.Request) {
		key, ok := parcelKey(w, r)
		if !ok {
			return
		}
		in, _, ok := studioDecodeRaw[cancelReturnBody](w, r, []string{"expected_version"})
		if !ok {
			return
		}
		result, ok := returnScope(w, r, pool, "fulfillment:write", func(c context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return returns.DecideBuyerRequest(c, tx, bearerToken(r), s.StoreID, r.PathValue("request_id"), key, "approve", returns.RequestDecision{ExpectedVersion: in.ExpectedVersion}, s)
		})
		if ok {
			respond(w, 200, result)
		}
	}))
	mux.HandleFunc("POST "+base+"/{request_id}/reject", requestRoute(http.MethodPost, func(w http.ResponseWriter, r *http.Request) {
		key, ok := parcelKey(w, r)
		if !ok {
			return
		}
		in, _, ok := studioDecodeRaw[returns.RequestDecision](w, r, []string{"expected_version", "reason"})
		if !ok {
			return
		}
		result, ok := returnScope(w, r, pool, "fulfillment:write", func(c context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return returns.DecideBuyerRequest(c, tx, bearerToken(r), s.StoreID, r.PathValue("request_id"), key, "reject", in, s)
		})
		if ok {
			respond(w, 200, result)
		}
	}))
}
