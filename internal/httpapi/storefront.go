// storefront.go mounts the merchant's storefront publication routes (R3 unit storefront-publish; contract
// published-storefront-resolver-v1 "Writer (R3)"):
//
//	GET  /v1/admin/stores/{store_id}/storefront              integration:read   -> storefrontadmin.Read
//	POST /v1/admin/stores/{store_id}/storefront/publication  integration:manage -> storefrontadmin.SetPublished
//	     body exactly {"published": bool, "expected_version": int>=0}; stale version = 409 conflict; no Idempotency-Key.
//
// and the merchant self-service domain routes (R5 unit store-domains, Decision 3):
//
//	GET  /v1/admin/stores/{store_id}/storefront/domains          integration:read   -> storefrontdomains.Read
//	POST /v1/admin/stores/{store_id}/storefront/domains          integration:manage -> storefrontdomains.Request
//	     body {"hostname": "<lower-case host>"}; returns the REQUESTED row + one-time DNS instructions.
//	POST /v1/admin/stores/{store_id}/storefront/domains/suspend  integration:manage -> storefrontdomains.Suspend
//	POST /v1/admin/stores/{store_id}/storefront/domains/detach   integration:manage -> storefrontdomains.Detach
//	     body {"origin": "https://..."}.
//
// The three write routes require Idempotency-Key exactly once (receipt grammar, command.Run idempotency) and the
// read forbids it; the key is rejected before any transaction opens.
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
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/command"
	"livecommerce/internal/platform"
	"livecommerce/internal/storefrontadmin"
	"livecommerce/internal/storefrontdomains"
)

type publicationInput struct {
	Published       *bool  `json:"published"`
	ExpectedVersion *int64 `json:"expected_version"`
}

// domainRequestInput is the merchant's proposed hostname; validated (and lower-cased) by storefrontdomains.Request.
type domainRequestInput struct {
	Hostname string `json:"hostname"`
}

// domainOriginInput names one of the store's own origins to suspend/detach.
type domainOriginInput struct {
	Origin string `json:"origin"`
}

func registerStorefrontRoutes(mux *http.ServeMux, pool *pgxpool.Pool, baseDomain string) {
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
	const domains = base + "/domains"
	mux.HandleFunc("GET "+domains, exactResourceRoute(storefrontDomainUnkeyed(scoped(pool, "integration:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
		return storefrontdomains.Read(ctx, tx, s, bearerToken(r))
	}))))
	mux.HandleFunc("POST "+domains, exactResourceRoute(storefrontDomainKeyed(storefrontDomainRequest(pool, baseDomain))))
	mux.HandleFunc("POST "+domains+"/suspend", exactResourceRoute(storefrontDomainKeyed(bodyRoute(pool, "integration:manage", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request, in domainOriginInput) (any, error) {
		return storefrontdomains.Suspend(ctx, tx, s, bearerToken(r), r.Header.Get("Idempotency-Key"), in.Origin)
	}))))
	mux.HandleFunc("POST "+domains+"/detach", exactResourceRoute(storefrontDomainKeyed(bodyRoute(pool, "integration:manage", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request, in domainOriginInput) (any, error) {
		return storefrontdomains.Detach(ctx, tx, s, bearerToken(r), r.Header.Get("Idempotency-Key"), in.Origin)
	}))))
}

// storefrontDomainKeyed requires Idempotency-Key exactly once in the receipt grammar (claimsKey, the same grammar
// command.Run re-checks) on the three domain write routes, before any transaction opens.
func storefrontDomainKeyed(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		keys := r.Header.Values("Idempotency-Key")
		if len(keys) != 1 || !claimsKey.MatchString(keys[0]) {
			respondError(w, http.StatusUnprocessableEntity, "invalid_request")
			return
		}
		next(w, r)
	}
}

// storefrontDomainUnkeyed forbids Idempotency-Key on the domain read route.
func storefrontDomainUnkeyed(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if len(r.Header.Values("Idempotency-Key")) != 0 {
			respondError(w, http.StatusUnprocessableEntity, "invalid_request")
			return
		}
		next(w, r)
	}
}

// storefrontDomainRequest is POST /domains: the strict body decode (same boundary as bodyRoute), the edge address
// resolution outside the transaction, then the scoped call into storefrontdomains.Request. The edge set is the same
// source the verifier accepts (resolved addresses of stores.<base>), resolved best-effort and attached only for an
// apex host by the service; a failed resolution just leaves the apex instruction without addresses.
func storefrontDomainRequest(pool *pgxpool.Pool, baseDomain string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || media != "application/json" {
			respondError(w, http.StatusUnsupportedMediaType, "json_required")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		defer r.Body.Close()
		var in domainRequestInput
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err = dec.Decode(&in); err != nil {
			respondError(w, http.StatusBadRequest, "invalid_json")
			return
		}
		var extra any
		if err = dec.Decode(&extra); err != io.EOF {
			respondError(w, http.StatusBadRequest, "invalid_json")
			return
		}
		var edge []string
		if baseDomain != "" {
			ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
			edge, _ = storefrontdomains.SystemResolver{}.LookupAddr(ctx, "stores."+baseDomain)
			cancel()
		}
		scoped(pool, "integration:manage", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return storefrontdomains.Request(ctx, tx, s, bearerToken(r), r.Header.Get("Idempotency-Key"), in.Hostname, baseDomain, edge)
		})(w, r)
	}
}
