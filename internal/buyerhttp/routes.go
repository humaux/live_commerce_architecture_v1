// Purpose: declare the private buyer transport's complete method/path inventory once.
// Depends on: net/http, strings and command's existing catalog ID validation.
// Used by: handler.ServeHTTP (matchRoute/allowed) and the contractdrift AST scanner.
// Invariants: Routes is read-only; first path match wins, no cleaning/redirects or HEAD aliases.

package buyerhttp

import (
	"net/http"
	"strings"

	"livecommerce/internal/command"
)

// Route binds a method and strict segment pattern to the existing service dispatch kind.
// Parameter names select route fields; ID syntax is still checked after method validation.
type Route struct {
	Method string
	Path   string
	kind   routeKind
}

// RouteTable owns both strict endpoint selection and interception of cleaning aliases.
type RouteTable []Route

// HasPrefix deliberately includes aliases such as /v1/buyer-extra, as mountBuyer
// always did. The strict matcher rejects them without a credential-bearing redirect.
func (RouteTable) HasPrefix(path string) bool {
	return strings.HasPrefix(path, "/v1/buyer")
}

// Routes is the complete buyerhttp inventory, in the original matching precedence.
// Treat this exported table as immutable. Feed belongs to cmd/api.mountFeed, not this handler.
//
//contractdrift:routes /v1/buyer matchRoute allowed
var Routes = RouteTable{
	{Method: http.MethodGet, Path: "/v1/buyer/session", kind: sessionRoute},
	{Method: http.MethodPost, Path: "/v1/buyer/session", kind: sessionRoute},
	{Method: http.MethodDelete, Path: "/v1/buyer/session", kind: sessionRoute},
	{Method: http.MethodPost, Path: "/v1/buyer/session/bootstrap", kind: bootstrapRoute},
	{Method: http.MethodPost, Path: "/v1/buyer/session/retire", kind: retireRoute},
	{Method: http.MethodGet, Path: designPublishedPath, kind: designPublishedRoute},
	{Method: http.MethodGet, Path: designPreviewPath, kind: designPreviewRoute},
	{Method: http.MethodGet, Path: primaryOriginPath, kind: primaryOriginRoute},
	{Method: http.MethodGet, Path: "/v1/buyer/catalog", kind: catalogRoute},
	{Method: http.MethodGet, Path: "/v1/buyer/checkout-options", kind: optionsRoute},
	{Method: http.MethodGet, Path: "/v1/buyer/cart", kind: cartRoute},
	{Method: http.MethodPut, Path: "/v1/buyer/cart", kind: cartRoute},
	{Method: http.MethodPost, Path: "/v1/buyer/quotes", kind: quotesRoute},
	{Method: http.MethodGet, Path: "/v1/buyer/destination", kind: destinationRoute},
	{Method: http.MethodPut, Path: "/v1/buyer/destination", kind: destinationRoute},
	{Method: http.MethodPost, Path: "/v1/buyer/checkout", kind: checkoutRoute},
	{Method: http.MethodGet, Path: "/v1/buyer/orders", kind: ordersRoute},
	{Method: http.MethodGet, Path: claimLinkPath, kind: claimLinkRoute},
	{Method: http.MethodPost, Path: claimRedeemPath, kind: claimRedeemRoute},
	{Method: http.MethodGet, Path: privacyPath, kind: privacyRoute},
	{Method: http.MethodPut, Path: consentsPath, kind: consentsRoute},
	{Method: http.MethodPost, Path: privacyExportPath, kind: privacyExportRoute},
	{Method: http.MethodPost, Path: privacyErasurePath, kind: privacyErasureRoute},
	{Method: http.MethodPost, Path: "/v1/buyer/cvs-selections", kind: routeCVSSelectionOpen},
	{Method: http.MethodPost, Path: "/v1/buyer/cvs-stores", kind: routeCVSStoreEnter},
	{Method: http.MethodPost, Path: "/v1/buyer/cvs-selections/{id}/verify", kind: routeCVSSelectionVerify},
	{Method: http.MethodGet, Path: "/v1/buyer/cvs-selections/{id}", kind: routeCVSSelectionGet},
	{Method: http.MethodPut, Path: "/v1/buyer/orders/{id}/bank-transfer/proof", kind: routeTransferProof},
	{Method: http.MethodGet, Path: "/v1/buyer/orders/{id}/bank-transfer", kind: routeTransferGet},
	{Method: http.MethodPost, Path: lookupPath, kind: routeLookup},
	{Method: http.MethodPost, Path: orderLinkPath, kind: routeOrderLink},
	{Method: http.MethodGet, Path: "/v1/buyer/media/s/{image}", kind: storeMediaRoute},
	{Method: http.MethodGet, Path: v2Prefix + "products", kind: catalogV2ProductsRoute},
	{Method: http.MethodGet, Path: v2Prefix + "collections", kind: catalogV2CollectionsRoute},
	{Method: http.MethodGet, Path: v2Prefix + "products/{slug_or_id}", kind: catalogV2ProductRoute},
	{Method: http.MethodGet, Path: v2Prefix + "collections/{slug}", kind: catalogV2CollectionRoute},
	{Method: http.MethodGet, Path: "/v1/buyer/media/c/{id}/{image}", kind: collectionMediaRoute},
	{Method: http.MethodGet, Path: "/v1/buyer/media/p/{id}/{image}", kind: mediaRoute},
	{Method: http.MethodPost, Path: "/v1/buyer/orders/{id}/payment/prepare", kind: paymentPrepareRoute},
	{Method: http.MethodPost, Path: "/v1/buyer/orders/{id}/payment/handoff", kind: paymentHandoffRoute},
	{Method: http.MethodPost, Path: "/v1/buyer/orders/{id}/payment/refresh", kind: paymentRefreshRoute},
	{Method: http.MethodPost, Path: "/v1/buyer/orders/{id}/payment/cancel", kind: paymentCancelRoute},
	{Method: http.MethodGet, Path: "/v1/buyer/orders/{id}/payment", kind: paymentRoute},
	{Method: http.MethodGet, Path: "/v1/buyer/quotes/{id}", kind: quoteRoute},
	{Method: http.MethodGet, Path: "/v1/buyer/destinations/{id}", kind: destinationItemRoute},
	{Method: http.MethodGet, Path: "/v1/buyer/orders/{id}", kind: orderRoute},
}

func matchRoute(path string) route {
	for _, entry := range Routes {
		if selected, ok := matchPattern(entry, path); ok {
			return selected
		}
	}
	return route{}
}

func allowed(kind routeKind, method string) bool {
	for _, entry := range Routes {
		if entry.kind == kind && entry.Method == method {
			return true
		}
	}
	return false
}

// matchPattern only splits literal segments. It never cleans or decodes a URL.
func matchPattern(entry Route, path string) (route, bool) {
	pattern, segments := strings.Split(entry.Path, "/"), strings.Split(path, "/")
	if len(pattern) != len(segments) {
		return route{}, false
	}
	selected := route{kind: entry.kind}
	for i, part := range pattern {
		value := segments[i]
		switch part {
		case "{id}":
			if value == "" {
				return route{}, false
			}
			selected.id = value
		case "{image}":
			if value == "" {
				return route{}, false
			}
			selected.image = value
		case "{slug_or_id}":
			if !(v2Slug.MatchString(value) && len(value) <= 80 || command.ValidID(value)) {
				return route{}, false
			}
			selected.slug = value
		case "{slug}":
			if !v2Slug.MatchString(value) || len(value) > 80 || command.ValidID(value) {
				return route{}, false
			}
			selected.slug = value
		default:
			if value != part {
				return route{}, false
			}
		}
	}
	return selected, true
}
