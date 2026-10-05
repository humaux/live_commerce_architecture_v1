package httpapi

import (
	"context"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/command"
	"livecommerce/internal/fulfillment"
	"livecommerce/internal/payments"
	"livecommerce/internal/platform"
)

const settingsBase = "/v1/admin/stores/{store_id}/markets/{market_id}/countries/{country}"

func registerSettingsRoutes(mux *http.ServeMux, pool *pgxpool.Pool) {
	mux.HandleFunc("GET "+settingsBase+"/delivery-services/{code}", exactResourceRoute(scoped(pool, "integration:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
		return fulfillment.GetService(ctx, tx, s, bearerToken(r), r.PathValue("market_id"), r.PathValue("country"), r.PathValue("code"))
	})))
	mux.HandleFunc("PUT "+settingsBase+"/delivery-services/{code}", exactResourceRoute(bodyRoute(pool, "integration:manage", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request, in fulfillment.ServiceInput) (any, error) {
		if !matchesSettingsTarget(r, in.MarketID, in.Country, in.Code) {
			return nil, command.ErrInvalid
		}
		// delivery-allocation P0: enabling/updating a service through the merchant settings path also
		// ensures its allocation in the same transaction, so the buyer sees the option without the
		// merchant configuring "warehouse allocation" explicitly.
		return fulfillment.SetServiceWithDefaultAllocation(ctx, tx, s, bearerToken(r), r.Header.Get("Idempotency-Key"), in)
	})))
	mux.HandleFunc("GET "+settingsBase+"/payment-methods/{code}", exactResourceRoute(scoped(pool, "integration:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
		return payments.GetMethod(ctx, tx, s, bearerToken(r), r.PathValue("market_id"), r.PathValue("country"), r.PathValue("code"))
	})))
	mux.HandleFunc("PUT "+settingsBase+"/payment-methods/{code}", exactResourceRoute(bodyRoute(pool, "integration:manage", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request, in payments.MethodInput) (any, error) {
		if !matchesSettingsTarget(r, in.MarketID, in.Country, in.Code) {
			return nil, command.ErrInvalid
		}
		return payments.SetMethod(ctx, tx, s, bearerToken(r), r.Header.Get("Idempotency-Key"), in)
	})))
	mux.HandleFunc("POST "+settingsBase+"/payment-methods/{code}/inspect", exactResourceRoute(bodyRoute(pool, "integration:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request, in payments.CheckInput) (any, error) {
		if !matchesSettingsTarget(r, in.MarketID, in.Country, in.Code) {
			return nil, command.ErrInvalid
		}
		return payments.InspectMethod(ctx, tx, s, bearerToken(r), in)
	})))
}

func exactResourceRoute(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "" || r.URL.ForceQuery {
			respondError(w, http.StatusUnprocessableEntity, "invalid_request")
			return
		}
		next(w, r)
	}
}

func matchesSettingsTarget(r *http.Request, marketID, country, code string) bool {
	return marketID == r.PathValue("market_id") && country == r.PathValue("country") && code == r.PathValue("code")
}

func bearerToken(r *http.Request) string {
	return strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
}
