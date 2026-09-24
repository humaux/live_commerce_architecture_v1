package httpapi

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/command"
	"livecommerce/internal/fulfillment"
	"livecommerce/internal/pagination"
	"livecommerce/internal/payments"
	"livecommerce/internal/platform"
	"livecommerce/internal/pricing"
)

func registerSettingsDiscoveryRoutes(mux *http.ServeMux, pool *pgxpool.Pool) {
	const markets = "/v1/admin/stores/{store_id}/markets"
	mux.HandleFunc("GET "+markets, listRoute(pool, "pricing:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request, page pagination.Request) (any, error) {
		return pricing.ListMarketsPage(ctx, tx, s, bearerToken(r), page)
	}))
	mux.HandleFunc("POST "+markets, exactResourceRoute(bodyRoute(pool, "pricing:write", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request, in pricing.MarketInput) (any, error) {
		if err := platform.RequirePermission(ctx, tx, s, bearerToken(r), "pricing:write"); err != nil {
			return nil, err
		}
		out, err := pricing.CreateMarket(ctx, tx, s, r.Header.Get("Idempotency-Key"), in)
		if err != nil {
			return nil, err
		}
		if err := platform.RequirePermission(ctx, tx, s, bearerToken(r), "pricing:write"); err != nil {
			return nil, err
		}
		return out, nil
	})))
	mux.HandleFunc("GET "+settingsBase+"/delivery-services", listRoute(pool, "integration:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request, page pagination.Request) (any, error) {
		return fulfillment.ListServicesPage(ctx, tx, s, bearerToken(r), r.PathValue("market_id"), r.PathValue("country"), page)
	}))
	mux.HandleFunc("GET /v1/admin/stores/{store_id}/markets/{market_id}/countries/TW/payment-methods", exactResourceRoute(scoped(pool, "integration:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
		return payments.ListMethods(ctx, tx, s, bearerToken(r), r.PathValue("market_id"), "TW")
	})))
	mux.HandleFunc("GET "+settingsBase+"/delivery-services/{code}/policy", exactResourceRoute(scoped(pool, "pricing:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
		return pricing.GetDeliveryPolicy(ctx, tx, s, bearerToken(r), r.PathValue("market_id"), r.PathValue("country"), r.PathValue("code"))
	})))
	mux.HandleFunc("PUT "+settingsBase+"/delivery-services/{code}/policy", exactResourceRoute(bodyRoute(pool, "pricing:write", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request, in pricing.PolicyInput) (any, error) {
		method, err := pricing.DeliveryMethod(r.PathValue("code"))
		if err != nil || in.MarketID != r.PathValue("market_id") || in.Country != r.PathValue("country") || in.Method != method {
			return nil, command.ErrInvalid
		}
		if err := platform.RequirePermission(ctx, tx, s, bearerToken(r), "pricing:write"); err != nil {
			return nil, err
		}
		out, err := pricing.SetPolicy(ctx, tx, s, r.Header.Get("Idempotency-Key"), in)
		if err != nil {
			return nil, err
		}
		if err := platform.RequirePermission(ctx, tx, s, bearerToken(r), "pricing:write"); err != nil {
			return nil, err
		}
		return out, nil
	})))
}
