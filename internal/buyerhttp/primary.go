package buyerhttp

// primary.go serves the 301-to-primary lookup (R5 unit store-domains, Decision 3): the storefront BFF asks whether
// the verified origin is a non-primary ACTIVE origin and, if so, which primary origin to redirect to. Like the other
// public reads it takes NO buyer bearer (BFF key + X-Commerce-Storefront-Origin only). SQL touched:
// control.resolve_primary_origin (migrations/0106, EXECUTE commerce_buyer_runtime, this pool), which returns the
// store's primary ACTIVE origin (a merchant ACTIVE row first, else the platform subdomain) or NULL when the request
// origin is not ACTIVE or is already primary (so no redirect). The answer is no-store (the redirect decision must
// never be served from a shared cache across origins).

import (
	"context"
	"net/http"

	"livecommerce/internal/storefrontdomains"
)

const primaryOriginPath = "/v1/buyer/storefront/primary-origin"

func (h *handler) primaryOriginGet(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	if err := noBody(r); err != nil {
		return err
	}
	origin := r.Header.Get("X-Commerce-Storefront-Origin")
	primary, err := storefrontdomains.PrimaryOrigin(ctx, h.pool, origin)
	if err != nil {
		return err
	}
	var out *string
	if primary != "" {
		out = &primary
	}
	writeOK(w, struct {
		PrimaryOrigin *string `json:"primary_origin"`
	}{out})
	return nil
}
