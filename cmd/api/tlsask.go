package main

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/tlsask"
)

// buildTLSAskHandler builds the edge TLS ask endpoint (R5 unit store-domains, Decision 4) on the runtime pool,
// which logs in as commerce_runtime and holds EXECUTE on control.resolve_storefront_ask (0106). The endpoint is
// internal-network only (Caddy's on_demand_tls `ask http://api:<port>/internal/tls-ask`); the per-service rate
// limit and negative cache live in internal/tlsask.
func buildTLSAskHandler(pool *pgxpool.Pool) (http.Handler, error) {
	svc, err := tlsask.New(pool, tlsask.DefaultMaxPerMinute, tlsask.DefaultDenyTTL, tlsask.DefaultAllowTTL)
	if err != nil {
		return nil, err
	}
	return svc.Handler(), nil
}

// mountTLSAsk serves /internal/tls-ask from ask and everything else from next.
func mountTLSAsk(next, ask http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/internal/tls-ask" {
			ask.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}
