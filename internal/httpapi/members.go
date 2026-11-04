// Purpose: merchant member/points HTTP adapter of contracts/loyalty-points-v1.md §2 (RED stub: routes not mounted yet).
// Depends on: internal/loyalty.
// Used by: NewHandler (integrator mounts registerMemberRoutes).
// Invariants: names, contact notes, notes and search terms never appear in a URL or a log line.

package httpapi

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

func registerMemberRoutes(mux *http.ServeMux, pool *pgxpool.Pool) {}
