// Purpose: the merchant keyword-tool routes of a live session (W3-06B): match simulator, keyword conflict check, auto-numbering suggestions and the atomic batch deactivate/rename of offers.
// Depends on: internal/claims (SimulateClaim, CheckKeywords, NextKeywords, BatchOffers); claims.go helpers (claimsRoute, claimsReadPostRoute, claimsBody, claimsBodyRoute, scopedAs, claimsClassify); studio.go (studioRoute).
// Used by: registerClaimRoutes (claims.go), mounted only when the claims routes are (COMMERCE_CLAIMS_ENABLED); the admin BFF must forward them (apps/admin/lib/claims-request.ts allow-list: UI unit).
// Invariants: simulate/check/next are reads (live:read, no row written, the comment or keyword text is never stored, logged or echoed in an error); batch is live:manage with a required Idempotency-Key and conflicts are 200 data;
//   an omitted match_mode is EXACT (owner ruling 2026-10-07); unknown body keys are 400 invalid_json.
//
// Routes (base = /v1/admin/stores/{store_id}/live-sessions/{session_id}/claims):
//   POST base/simulate           {comment, match_mode?}                    -> claims.SimulatedClaim
//   POST base/keywords/check     {keywords:[...], match_mode?}             -> claims.KeywordCheck
//   GET  base/keywords/next      ?prefix=&count=                           -> claims.KeywordSuggestion
//   POST base/offers/batch       {items:[{offer_id,expected_version,action,keyword?}]} -> claims.BatchResult

package httpapi

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/claims"
	"livecommerce/internal/platform"
)

// registerKeywordToolRoutes mounts the four W3-06B routes on the claims base path. Called once by registerClaimRoutes.
func registerKeywordToolRoutes(mux *http.ServeMux, pool *pgxpool.Pool, base string) {
	simulate, check, next, batch := base+"/simulate", base+"/keywords/check", base+"/keywords/next", base+"/offers/batch"
	mux.HandleFunc("POST "+simulate, claimsReadPostRoute(func(w http.ResponseWriter, r *http.Request) {
		in, ok := claimsBody[claims.SimulateInput](w, r, []string{"comment"}, nil, "match_mode")
		if !ok {
			return
		}
		claimsScoped(pool, "live:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return claims.SimulateClaim(ctx, tx, s, bearerToken(r), r.PathValue("session_id"), in)
		})(w, r)
	}))
	mux.HandleFunc("POST "+check, claimsReadPostRoute(func(w http.ResponseWriter, r *http.Request) {
		in, ok := claimsBody[claims.KeywordCheckInput](w, r, []string{"keywords"}, nil, "match_mode")
		if !ok {
			return
		}
		claimsScoped(pool, "live:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return claims.CheckKeywords(ctx, tx, s, bearerToken(r), r.PathValue("session_id"), in)
		})(w, r)
	}))
	mux.HandleFunc("GET "+next, claimsRoute(http.MethodGet, true, func(w http.ResponseWriter, r *http.Request) {
		prefix, count, ok := nextKeywordsQuery(r.URL)
		if !ok {
			respondError(w, http.StatusUnprocessableEntity, "invalid_request")
			return
		}
		claimsScoped(pool, "live:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return claims.NextKeywords(ctx, tx, s, bearerToken(r), r.PathValue("session_id"), prefix, count)
		})(w, r)
	}))
	mux.HandleFunc("POST "+batch, claimsRoute(http.MethodPost, false, claimsBodyRoute(pool, []string{"items"}, nil, nil,
		func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request, in claims.BatchInput) (any, error) {
			return claims.BatchOffers(ctx, tx, s, bearerToken(r), r.Header.Get("Idempotency-Key"), r.PathValue("session_id"), in)
		})))
	// Methodless fallbacks keep 405 inside the private boundary. No fallback for offers/batch: a methodless literal path would overlap
	// "PATCH .../offers/{offer_id}" with neither more specific (a ServeMux conflict); a GET there already reaches the offers/{offer_id} 405 fallback.
	for _, path := range []string{simulate, check, next} {
		mux.HandleFunc(path, studioRoute("", false, nil))
	}
}

// nextKeywordsQuery accepts only `prefix` and `count`, each at most once and non-empty; count is 1..20 plain digits. A missing
// value is the default (prefix "A", count 1: reported as "" and 0). The prefix letters are validated by claims.NextKeywords.
func nextKeywordsQuery(u *url.URL) (string, int, bool) {
	if u.ForceQuery || len(u.RawQuery) > 256 {
		return "", 0, false
	}
	values, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return "", 0, false
	}
	var prefix string
	count := 0
	for name, list := range values {
		if len(list) != 1 || list[0] == "" {
			return "", 0, false
		}
		switch name {
		case "prefix":
			prefix = list[0]
		case "count":
			n, err := strconv.Atoi(list[0])
			if err != nil || n < 1 || n > 20 || strconv.Itoa(n) != list[0] {
				return "", 0, false
			}
			count = n
		default:
			return "", 0, false
		}
	}
	return prefix, count, true
}
