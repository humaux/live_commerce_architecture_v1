// Purpose: merchant HTTP routes of the PAYUNi self-serve activation (w4-02b): start/check an NT$1 SANDBOX verification, run the
//   LIVE credential probe, read the activation status; plus the classifier that gives SetMethod's new refusals their JSON codes.
// Depends on: internal/payments (Activation, ErrNotQualified, ErrPlatformDisabled, ErrProfileNotAllowed, ErrProbeFailed),
//   internal/platform (WithScope), internal/httperror via respondError; contracts/payment-methods-v1.md Amendment W4-02B.
// Used by: handler.go (NewHandler mounts them when Options.PayuniActivation is set), settings.go (PUT payment-methods classifier),
//   cmd/api (builds the service), payment_activation_test.go and tests/foundation/payuni_activation_test.go.
// Invariants: store/tenant come from the authenticated scope, never the body; Check and LiveProbe do their provider call outside
//   any database transaction (payments.Activation phases); responses carry no credential and no raw provider body.
// Status: MOCK/SANDBOX.

package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/payments"
	"livecommerce/internal/platform"
)

const activationBase = "/v1/admin/stores/{store_id}/payments/payuni"

type activationConnection struct {
	ConnectionID string `json:"connection_id"`
}

// activationClassify maps the activation/enable refusals to their stable codes, then falls back to the shared classifier.
func activationClassify(err error) (int, string) {
	switch {
	case errors.Is(err, payments.ErrNotQualified):
		return http.StatusConflict, "not_qualified"
	case errors.Is(err, payments.ErrPlatformDisabled):
		return http.StatusConflict, "platform_disabled"
	case errors.Is(err, payments.ErrProfileNotAllowed):
		return http.StatusConflict, "profile_not_allowed"
	case errors.Is(err, payments.ErrProbeFailed):
		return http.StatusUnprocessableEntity, "payuni_probe_failed"
	}
	return classify(err)
}

// activationPooled is a body route whose handler owns its transactions (provider call between two scoped transactions), so it
// cannot use bodyRoute's single tx. Same bearer, content-type, strict-JSON and no-query rules as bodyRouteAs.
func activationPooled(fn func(ctx context.Context, token, storeID string, in activationConnection, r *http.Request) (any, error)) http.HandlerFunc {
	return exactResourceRoute(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") || strings.ContainsAny(strings.TrimPrefix(header, "Bearer "), " \t\r\n") {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || media != "application/json" {
			respondError(w, http.StatusUnsupportedMediaType, "json_required")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
		defer r.Body.Close()
		var in activationConnection
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var extra any
		if err = dec.Decode(&in); err != nil || dec.Decode(&extra) != io.EOF {
			respondError(w, http.StatusBadRequest, "invalid_json")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second) // two scoped txs + one 8 s provider call
		defer cancel()
		out, err := fn(ctx, strings.TrimPrefix(header, "Bearer "), r.PathValue("store_id"), in, r)
		if err != nil {
			status, code := activationClassify(err)
			respondErrorDetails(w, err, status, code)
			return
		}
		respond(w, http.StatusOK, out)
	})
}

// registerPaymentActivationRoutes mounts the activation routes; a nil service leaves them unmounted (cmd/api builds it when
// LC_PAYUNI_NOTIFY_BASE_URL and LC_PAYUNI_VERIFY_RETURN_URL are set).
func registerPaymentActivationRoutes(mux *http.ServeMux, pool *pgxpool.Pool, svc *payments.Activation) {
	if svc == nil {
		return
	}
	// POST verify: start/replay (Idempotency-Key) one NT$1 SANDBOX verification; returns the hosted form. One scoped tx.
	mux.HandleFunc("POST "+activationBase+"/verify", exactResourceRoute(bodyRouteAs(pool, "integration:manage", activationClassify,
		func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request, in activationConnection) (any, error) {
			return svc.Start(ctx, tx, s, bearerToken(r), r.Header.Get("Idempotency-Key"), in.ConnectionID)
		})))
	// GET status of one connection's activation.
	mux.HandleFunc("GET "+activationBase+"/connections/{connection_id}/status", exactResourceRoute(scopedAs(pool, "integration:read", activationClassify,
		func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return svc.Status(ctx, tx, s, bearerToken(r), r.PathValue("connection_id"))
		})))
	// POST check: one PAYUNi query of the verification trade, stored as evidence (provider call outside any tx).
	mux.HandleFunc("POST "+activationBase+"/verify/{verification_id}/check", activationPooled(
		func(ctx context.Context, token, storeID string, in activationConnection, r *http.Request) (any, error) {
			return svc.Check(ctx, pool, token, storeID, in.ConnectionID, r.PathValue("verification_id"))
		}))
	// POST live-probe: read-only LIVE credential probe; issues REAL_LIVE only on an authenticated "no such trade" reply.
	mux.HandleFunc("POST "+activationBase+"/live-probe", activationPooled(
		func(ctx context.Context, token, storeID string, in activationConnection, r *http.Request) (any, error) {
			return svc.LiveProbe(ctx, pool, token, storeID, in.ConnectionID)
		}))
}
