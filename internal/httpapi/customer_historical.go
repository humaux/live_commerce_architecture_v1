// Purpose: the merchant read route of the historical-order archive (W5-03B, contracts/migration-import-v1.md section 7):
//   GET /v1/admin/stores/{store_id}/customers/{customer_id}/historical-orders[?limit&cursor]. It decides no rule: internal/customers and
//   the 0156 definer customers.read_historical_orders do.
// Depends on: internal/customers (ListHistoricalOrders), customers.go helpers (customerRoute, customersScope, parseCustomersQuery),
//   customer_tags.go parseNotesQuery (the same limit/cursor grammar, no q/tag filter).
// Used by: customers.go registerCustomerRoutes (mounted unconditionally).
// Invariants: permission customers:read; tenant and store come from the bearer, so another store's customer is 404; the response is
//   private and non-cacheable and never carries a street address, phone, email or payment detail.

package httpapi

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/customers"
	"livecommerce/internal/platform"
)

// registerCustomerHistoricalRoutes mounts the one read row. The literal "historical-orders" segment never overlaps another
// customers/{customer_id}/... row (the DB-free full-router test fails at registration on any conflict).
func registerCustomerHistoricalRoutes(mux *http.ServeMux, pool *pgxpool.Pool) {
	mux.HandleFunc("GET "+customerBase+"/{customer_id}/historical-orders", customerRoute(http.MethodGet, true, func(w http.ResponseWriter, r *http.Request) {
		page, err := parseNotesQuery(r.URL)
		if err != nil {
			respondError(w, http.StatusUnprocessableEntity, "invalid_request")
			return
		}
		if result, ok := customersScope(w, r, pool, "customers:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return customers.ListHistoricalOrders(ctx, tx, s, bearerToken(r), r.PathValue("customer_id"), page)
		}); ok {
			respond(w, http.StatusOK, result)
		}
	}))
}
