// storefront.go mounts the merchant's storefront publication routes (R3 unit storefront-publish; contract
// published-storefront-resolver-v1 "Writer (R3)"):
//
//	GET  /v1/admin/stores/{store_id}/storefront              integration:read   -> storefrontadmin.Read
//	POST /v1/admin/stores/{store_id}/storefront/publication  integration:manage -> storefrontadmin.SetPublished
//	     body exactly {"published": bool, "expected_version": int>=0}; stale version = 409 conflict; no Idempotency-Key.
//
// Behind them: admin BFF apps/admin/app/api/stores/[store]/[...resource]/route.ts (GET storefront, POST
// storefront/publication) and the Settings card apps/admin/components/StorefrontSettings.tsx.
// Store scope comes from the verified bearer only (platform.WithScope); the definers re-verify it in SQL.
//
// Non-goals: no domain route of any kind (domain binding is the operator CLI cmd/store-admin, never merchant HTTP),
// no unpublish side effects on open carts (the resolver simply stops admitting new requests).

package httpapi

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/command"
	"livecommerce/internal/platform"
	"livecommerce/internal/storefrontadmin"
)

type publicationInput struct {
	Published       *bool  `json:"published"`
	ExpectedVersion *int64 `json:"expected_version"`
}

func registerStorefrontRoutes(mux *http.ServeMux, pool *pgxpool.Pool) {
	const base = "/v1/admin/stores/{store_id}/storefront"
	mux.HandleFunc("GET "+base, exactResourceRoute(scoped(pool, "integration:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
		return storefrontadmin.Read(ctx, tx, s, bearerToken(r))
	})))
	mux.HandleFunc("POST "+base+"/publication", exactResourceRoute(bodyRoute(pool, "integration:manage", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request, in publicationInput) (any, error) {
		if in.Published == nil || in.ExpectedVersion == nil {
			return nil, command.ErrInvalid
		}
		return storefrontadmin.SetPublished(ctx, tx, s, bearerToken(r), *in.Published, *in.ExpectedVersion)
	})))
}
