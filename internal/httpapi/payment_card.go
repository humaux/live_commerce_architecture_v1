// Purpose: merchant routes of platform-Stripe card payments, GET/PUT /v1/admin/stores/{store_id}/payments/card
// (contracts/stripe-platform-account-v1.md §3.3). Each route is a thin transport gate (exact method, no query, canonical ids,
// strict JSON body, no Idempotency-Key: the PUT is CAS-protected) around one internal/payments/platformstripe call; the SQL
// definers decide every rule. The profile comes from COMMERCE_PAYMENT_PROFILE (cmd/api), never from the request.
// Depends on: internal/payments/platformstripe, internal/platform (scope tx), cvsRoute/cvsStrictBody helpers (cvs.go).
// Used by: handler.go NewHandler (registerPaymentCardRoutes); the admin BFF mirrors it (W4-U1, Codex).
// Invariants: I01 (scope from the token); no account id, key, approval id or tenant id in any response.

package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/payments/platformstripe"
	"livecommerce/internal/platform"
)

var paymentCardFields = []string{"enabled", "terms_version", "descriptor_suffix", "expected_version"}

// registerPaymentCardRoutes mounts GET and PUT .../payments/card. An empty profile (payment-free deployment) leaves the
// surface unmounted, exactly like a nil client elsewhere; an unknown profile is refused by cmd/api at startup.
func registerPaymentCardRoutes(mux *http.ServeMux, pool *pgxpool.Pool, profile string) {
	if profile != "PROVIDER_MOCK" && profile != "SANDBOX" && profile != "LIVE" {
		return
	}
	const path = "/v1/admin/stores/{store_id}/payments/card"
	mux.HandleFunc("GET "+path, cvsRoute(http.MethodGet, false, func(w http.ResponseWriter, r *http.Request) {
		paymentCardServe(w, r, pool, "integration:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope) (any, error) {
			return platformstripe.Read(ctx, tx, s, bearerToken(r), profile)
		})
	}))
	mux.HandleFunc("PUT "+path, cvsRoute(http.MethodPut, false, func(w http.ResponseWriter, r *http.Request) {
		// descriptor_suffix may be null (no suffix); every key must be present.
		in, ok := cvsStrictBody[platformstripe.Input](w, r, paymentCardFields, []string{"descriptor_suffix"})
		if !ok {
			return
		}
		paymentCardServe(w, r, pool, "billing:manage", func(ctx context.Context, tx pgx.Tx, s platform.Scope) (any, error) {
			return platformstripe.Set(ctx, tx, s, bearerToken(r), profile, in)
		})
	}))
	mux.HandleFunc(path, studioRoute("", false, nil)) // methodless fallback: 405 inside the private response boundary
}

// paymentCardServe runs fn in one scoped transaction (the permission only opens the scope; the SQL re-checks it) and writes
// the result or the coded refusal. A nil return of platform.WithScope is the COMMIT acknowledgement.
func paymentCardServe(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool, permission string,
	fn func(context.Context, pgx.Tx, platform.Scope) (any, error)) {
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	var result any
	err := platform.WithScope(ctx, pool, bearerToken(r), r.PathValue("store_id"), permission, func(tx pgx.Tx, s platform.Scope) error {
		var inner error
		result, inner = fn(ctx, tx, s)
		return inner
	})
	if err != nil {
		status, code := paymentCardClassify(err)
		respondError(w, status, code)
		return
	}
	respond(w, http.StatusOK, result)
}

// paymentCardClassify maps the platformstripe coded refusals (403/409/422), then the shared claims table. No driver message.
func paymentCardClassify(err error) (int, string) {
	var coded *platformstripe.Error
	if errors.As(err, &coded) {
		return coded.Status, coded.Code
	}
	if errors.Is(err, platformstripe.ErrUnavailable) {
		return http.StatusServiceUnavailable, "unavailable"
	}
	return claimsClassify(err)
}
