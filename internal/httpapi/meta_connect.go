// meta_connect.go owns the merchant Facebook Page / Instagram connect HTTP adapter (contracts/meta-claims-intake-v1.md
// "Merchant connect (R4)") under /v1/admin/stores/{store_id}/meta-connect: POST start, GET callback, GET status, GET
// states/{state_id}, POST pick, POST disconnect. The admin BFF issues the browser redirect itself (apps/admin
// app/api/meta/connect + callback) and forwards status/states/pick/disconnect through the generic store BFF.
//
// It decides no rule (internal/metaconnect and the integration.meta_connect_* definers do), never calls Meta itself, and never
// returns a driver message, a token, an OAuth code or a state value: the callback does not log and no formatter here sees
// the URL query. Every route keeps the Studio private no-store boundary and strict JSON.
// Route -> Go endpoint: these handlers ARE the Go endpoints (apps/admin lib/backend.ts callBackend mirrors them under /api/admin).

package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/metaconnect"
	"livecommerce/internal/platform"
)

var metaConnectPickFields = []string{"state_id", "page_id", "include_instagram"}
var metaConnectDisconnectFields = []string{"page_id"}

// registerMetaConnectRoutes mounts the connect surface; a nil service leaves it unmounted (cmd/api builds it only when
// COMMERCE_META_LOGIN_CONFIG_ID and the Meta app are configured).
func registerMetaConnectRoutes(mux *http.ServeMux, pool *pgxpool.Pool, svc *metaconnect.Service) {
	if svc == nil {
		return
	}
	const base = "/v1/admin/stores/{store_id}/meta-connect"
	fallbacks := []string{"/start", "/callback", "/status", "/states/{state_id}", "/pick", "/disconnect"}

	mux.HandleFunc("POST "+base+"/start", adsRoute(http.MethodPost, true, false, func(w http.ResponseWriter, r *http.Request) {
		if !adsNoBody(w, r) {
			return
		}
		metaConnectScope(w, r, pool, "integration:manage", http.StatusCreated, func(ctx context.Context, tx pgx.Tx, s platform.Scope) (any, error) {
			return svc.Start(ctx, tx, s, bearerToken(r), r.Header.Get("Idempotency-Key"))
		})
	}))
	mux.HandleFunc("GET "+base+"/callback", adsRoute(http.MethodGet, false, true, func(w http.ResponseWriter, r *http.Request) {
		metaConnectCallback(w, r, pool, svc)
	}))
	mux.HandleFunc("GET "+base+"/status", adsRoute(http.MethodGet, false, false, func(w http.ResponseWriter, r *http.Request) {
		metaConnectScope(w, r, pool, "integration:read", http.StatusOK, func(ctx context.Context, tx pgx.Tx, s platform.Scope) (any, error) {
			return svc.Status(ctx, tx, s, bearerToken(r))
		})
	}))
	mux.HandleFunc("GET "+base+"/states/{state_id}", adsRoute(http.MethodGet, false, false, func(w http.ResponseWriter, r *http.Request) {
		metaConnectScope(w, r, pool, "integration:manage", http.StatusOK, func(ctx context.Context, tx pgx.Tx, s platform.Scope) (any, error) {
			return svc.GetState(ctx, tx, s, bearerToken(r), r.PathValue("state_id"))
		})
	}))
	mux.HandleFunc("POST "+base+"/pick", adsRoute(http.MethodPost, true, false, func(w http.ResponseWriter, r *http.Request) {
		in, ok := claimsBody[metaconnect.PickInput](w, r, metaConnectPickFields, nil)
		if !ok {
			return
		}
		metaConnectLong(w, r, func(ctx context.Context) (any, int, error) {
			out, err := svc.Pick(ctx, pool, bearerToken(r), r.PathValue("store_id"), in)
			return out, http.StatusCreated, err
		})
	}))
	mux.HandleFunc("POST "+base+"/disconnect", adsRoute(http.MethodPost, true, false, func(w http.ResponseWriter, r *http.Request) {
		in, ok := claimsBody[struct {
			PageID string `json:"page_id"`
		}](w, r, metaConnectDisconnectFields, nil)
		if !ok {
			return
		}
		metaConnectLong(w, r, func(ctx context.Context) (any, int, error) {
			return map[string]bool{"disconnected": true}, http.StatusOK,
				svc.Disconnect(ctx, pool, bearerToken(r), r.PathValue("store_id"), in.PageID)
		})
	}))
	for _, suffix := range fallbacks {
		mux.HandleFunc(base+suffix, studioRoute("", false, nil))
	}
}

// metaConnectScope runs fn in a platform.WithScope transaction opened with permission and writes the response: status on
// success (only after COMMIT is acknowledged), the classified error otherwise.
func metaConnectScope(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool, permission string, status int,
	fn func(context.Context, pgx.Tx, platform.Scope) (any, error)) {
	if !canonicalBearer(r) {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	var result any
	err := platform.WithScope(ctx, pool, bearerToken(r), r.PathValue("store_id"), permission, func(tx pgx.Tx, s platform.Scope) error {
		var inner error
		result, inner = fn(ctx, tx, s)
		return inner
	})
	if err != nil {
		code, name := metaConnectClassify(err)
		respondError(w, code, name)
		return
	}
	respond(w, status, result)
}

// metaConnectLong runs a multi-transaction step (pick, disconnect: two transactions around Graph calls) under a 45 s deadline.
func metaConnectLong(w http.ResponseWriter, r *http.Request, fn func(context.Context) (any, int, error)) {
	if !canonicalBearer(r) {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	result, status, err := fn(ctx)
	if err != nil {
		code, name := metaConnectClassify(err)
		respondError(w, code, name)
		return
	}
	respond(w, status, result)
}

// metaConnectCallback is GET callback?code&state: 200 {"state_id"}; the admin BFF issues the 303. The URL and query are never
// logged or echoed; the codes are the fixed state_mismatch 409, state_expired 410 and meta_connect_failed 502.
func metaConnectCallback(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool, svc *metaconnect.Service) {
	if !canonicalBearer(r) {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	values := r.URL.Query()
	if len(r.URL.RawQuery) > 2200 || len(values) != 2 || len(values["code"]) != 1 || len(values["state"]) != 1 ||
		!adsCodePattern.MatchString(values["code"][0]) {
		respondError(w, http.StatusUnprocessableEntity, "invalid_request")
		return
	}
	if !adsStatePattern.MatchString(values["state"][0]) {
		respondError(w, http.StatusConflict, "state_mismatch")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 40*time.Second)
	defer cancel()
	stateID, err := svc.Callback(ctx, pool, bearerToken(r), r.PathValue("store_id"), values["code"][0], values["state"][0])
	if err != nil {
		code, name := metaConnectClassify(err)
		respondError(w, code, name)
		return
	}
	respond(w, http.StatusOK, map[string]string{"state_id": stateID})
}

// metaConnectClassify maps connect refusals and ErrConnectFailed to their frozen statuses and codes, then the claims table
// (deadlock and unknown errors become a retryable 503). No driver message is ever returned.
func metaConnectClassify(err error) (int, string) {
	var refused *metaconnect.Refusal
	switch {
	case errors.As(err, &refused):
		return refused.Status, refused.Code
	case errors.Is(err, metaconnect.ErrConnectFailed):
		return http.StatusBadGateway, "meta_connect_failed"
	}
	return claimsClassify(err)
}
