// Purpose: the merchant READ-ONLY settlement routes of the platform Stripe ledger (contracts/stripe-platform-account-v1.md §6.4/§6.5):
//   GET /v1/admin/stores/{store_id}/settlements[?before=YYYY-MM-DD&limit=1..52]    statements, newest first, no lines
//   GET /v1/admin/stores/{store_id}/settlements/{statement_id}                      one statement with its lines
// Each is a thin transport gate (exact method, exact query, canonical ids, no body or Idempotency-Key, private no-store) around one
// settlement.Read call; the SQL definer decides every rule (billing:manage, the store from the token, no settlement-currency fields).
// The admin BFF mirrors them (W4-U1, Codex).
// Depends on: internal/payments/settlement, internal/platform (scope tx via withToolsScope), studioRoute/canonicalBearer/claimsClassify.
// Used by: handler.go NewHandler (registerSettlementRoutes); tests/foundation/platform_settlement_*_test.go.
// Invariants: I01 (the store is authorised by the bearer's grants, never trusted from the path alone); no Stripe id, txn id, tenant id,
//   account id or settlement-currency amount appears in any response (the SQL builds the body).
// Status: MOCK + REAL_PG.

package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/command"
	"livecommerce/internal/payments/settlement"
	"livecommerce/internal/platform"
)

// registerSettlementRoutes mounts the two GET routes. They need only the pool (no worker, no provider), so NewHandler mounts them
// unconditionally like the reports; a deployment without any statement answers an empty list.
func registerSettlementRoutes(mux *http.ServeMux, pool *pgxpool.Pool) {
	const base = "/v1/admin/stores/{store_id}/settlements"
	mux.HandleFunc("GET "+base, settlementRoute(func(w http.ResponseWriter, r *http.Request) {
		before, limit, ok := settlementQuery(r.URL)
		if !ok {
			respondError(w, http.StatusUnprocessableEntity, "invalid_request")
			return
		}
		settlementServe(w, r, pool, func(ctx context.Context, tx pgx.Tx, s platform.Scope) (any, error) {
			return settlement.Read(ctx, tx, s, bearerToken(r), limit, before, "")
		})
	}))
	mux.HandleFunc("GET "+base+"/{statement_id}", settlementRoute(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "" || r.URL.ForceQuery || !command.ValidID(r.PathValue("statement_id")) {
			respondError(w, http.StatusUnprocessableEntity, "invalid_request")
			return
		}
		settlementServe(w, r, pool, func(ctx context.Context, tx pgx.Tx, s platform.Scope) (any, error) {
			return settlement.Read(ctx, tx, s, bearerToken(r), 52, "", r.PathValue("statement_id"))
		})
	}))
	mux.HandleFunc(base, studioRoute("", true, nil))                   // methodless fallback: 405 inside the private response boundary
	mux.HandleFunc(base+"/{statement_id}", studioRoute("", true, nil)) // same for the detail path
}

// settlementRoute is the GET gate: exact method, canonical store id, a bearer of the canonical shape, no body or key.
func settlementRoute(next http.HandlerFunc) http.HandlerFunc {
	return studioRoute(http.MethodGet, true, func(w http.ResponseWriter, r *http.Request) {
		if !command.ValidID(r.PathValue("store_id")) {
			respondError(w, http.StatusUnprocessableEntity, "invalid_request")
			return
		}
		if !canonicalBearer(r) {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next(w, r)
	})
}

// settlementQuery parses ?before=YYYY-MM-DD&limit=N (each at most once; nothing else). Defaults: no cursor, limit 52.
func settlementQuery(u *url.URL) (before string, limit int, ok bool) {
	limit = 52
	if u.ForceQuery || len(u.RawQuery) > 128 {
		return "", 0, false
	}
	if u.RawQuery == "" {
		return "", limit, true
	}
	values, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return "", 0, false
	}
	for name, vs := range values {
		if (name != "before" && name != "limit") || len(vs) != 1 || vs[0] == "" || strings.ContainsAny(vs[0], " +%") {
			return "", 0, false
		}
	}
	if v, present := values["limit"]; present {
		n, err := strconv.Atoi(v[0])
		if err != nil || n < 1 || n > 52 {
			return "", 0, false
		}
		limit = n
	}
	if v, present := values["before"]; present {
		if len(v[0]) != 10 {
			return "", 0, false
		}
		before = v[0]
	}
	return before, limit, true
}

// settlementServe runs fn in one scoped READ COMMITTED transaction (billing:manage opens the scope; the SQL re-checks it and
// re-resolves the token at the end) and writes the body or the coded refusal. No driver message is ever returned.
func settlementServe(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool, fn func(context.Context, pgx.Tx, platform.Scope) (any, error)) {
	var result any
	err := withToolsScope(r, pool, "billing:manage", 0, func(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
		var inner error
		result, inner = fn(ctx, tx, s)
		return inner
	})
	if err != nil {
		status, code := settlementClassify(err)
		respondError(w, status, code)
		return
	}
	respond(w, http.StatusOK, result)
}

// settlementClassify maps the settlement coded refusal, then the shared claims table.
func settlementClassify(err error) (int, string) {
	var coded *settlement.Error
	if errors.As(err, &coded) {
		return coded.Status, coded.Code
	}
	if errors.Is(err, settlement.ErrUnavailable) {
		return http.StatusServiceUnavailable, "unavailable"
	}
	return claimsClassify(err)
}
