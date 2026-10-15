// Purpose: freeze pre-table path and method semantics for differential CI acceptance.
// Depends on: the e4dad98cc source snapshot, existing route constants and validation.
// Used by: TestRouteTableMatchesLegacy; never linked into the product.
// Invariant: this oracle is independent of Routes and must not change with the implementation.
package buyerhttp

import (
	"livecommerce/internal/command"
	"net/http"
	"strings"
)

func legacyMatchRoute(path string) route {
	switch path {
	case "/v1/buyer/session":
		return route{kind: sessionRoute}
	case "/v1/buyer/session/bootstrap":
		return route{kind: bootstrapRoute}
	case "/v1/buyer/session/retire":
		return route{kind: retireRoute}
	case designPublishedPath:
		return route{kind: designPublishedRoute}
	case designPreviewPath:
		return route{kind: designPreviewRoute}
	case primaryOriginPath:
		return route{kind: primaryOriginRoute}
	case "/v1/buyer/catalog":
		return route{kind: catalogRoute}
	case "/v1/buyer/checkout-options":
		return route{kind: optionsRoute}
	case "/v1/buyer/cart":
		return route{kind: cartRoute}
	case "/v1/buyer/quotes":
		return route{kind: quotesRoute}
	case "/v1/buyer/destination":
		return route{kind: destinationRoute}
	case "/v1/buyer/checkout":
		return route{kind: checkoutRoute}
	case "/v1/buyer/orders":
		return route{kind: ordersRoute}
	case claimLinkPath:
		return route{kind: claimLinkRoute}
	case claimRedeemPath:
		return route{kind: claimRedeemRoute}
	case privacyPath:
		return route{kind: privacyRoute}
	case consentsPath:
		return route{kind: consentsRoute}
	case privacyExportPath:
		return route{kind: privacyExportRoute}
	case privacyErasurePath:
		return route{kind: privacyErasureRoute}
	}
	if cvs := legacyMatchCVSRoute(path); cvs.kind != unknownRoute {
		return cvs
	}
	if transfer := legacyMatchTransferRoute(path); transfer.kind != unknownRoute {
		return transfer
	}
	if path == lookupPath {
		return route{kind: routeLookup}
	}
	if path == orderLinkPath {
		return route{kind: routeOrderLink}
	}
	if image, ok := strings.CutPrefix(path, "/v1/buyer/media/s/"); ok {
		if image != "" && !strings.Contains(image, "/") {
			return route{kind: storeMediaRoute, image: image}
		}
		return route{}
	}
	if rest, ok := strings.CutPrefix(path, v2Prefix); ok {
		return legacyMatchCatalogV2(rest)
	}
	if rest, ok := strings.CutPrefix(path, "/v1/buyer/media/c/"); ok {
		if collection, image, two := strings.Cut(rest, "/"); two && collection != "" && image != "" && !strings.Contains(image, "/") {
			return route{kind: collectionMediaRoute, id: collection, image: image}
		}
		return route{}
	}
	if rest, ok := strings.CutPrefix(path, "/v1/buyer/media/p/"); ok {
		if product, image, two := strings.Cut(rest, "/"); two && product != "" && image != "" && !strings.Contains(image, "/") {
			return route{kind: mediaRoute, id: product, image: image}
		}
		return route{}
	}
	if rest, ok := strings.CutPrefix(path, "/v1/buyer/orders/"); ok {
		for _, entry := range []struct {
			suffix string
			kind   routeKind
		}{{"/payment/prepare", paymentPrepareRoute}, {"/payment/handoff", paymentHandoffRoute},
			{"/payment/refresh", paymentRefreshRoute}, {"/payment/cancel", paymentCancelRoute}, {"/payment", paymentRoute}} {
			if id, matched := strings.CutSuffix(rest, entry.suffix); matched && id != "" && !strings.Contains(id, "/") {
				return route{kind: entry.kind, id: id}
			}
		}
	}
	for _, entry := range []struct {
		prefix string
		kind   routeKind
	}{{"/v1/buyer/quotes/", quoteRoute}, {"/v1/buyer/destinations/", destinationItemRoute}, {"/v1/buyer/orders/", orderRoute}} {
		if id, ok := strings.CutPrefix(path, entry.prefix); ok && id != "" && !strings.Contains(id, "/") {
			return route{kind: entry.kind, id: id}
		}
	}
	return route{}
}

func legacyAllowed(kind routeKind, method string) bool {
	if isCVSRoute(kind) {
		return legacyAllowedCVS(kind, method)
	}
	if isTransferRoute(kind) {
		return legacyAllowedTransfer(kind, method)
	}
	if kind == routeLookup || kind == routeOrderLink {
		return method == http.MethodPost
	}
	switch kind {
	case sessionRoute:
		return method == http.MethodGet || method == http.MethodPost || method == http.MethodDelete
	case bootstrapRoute, retireRoute:
		return method == http.MethodPost
	case catalogRoute, optionsRoute, ordersRoute, paymentRoute, mediaRoute, designPublishedRoute, designPreviewRoute, storeMediaRoute, collectionMediaRoute,
		catalogV2ProductsRoute, catalogV2ProductRoute, catalogV2CollectionsRoute, catalogV2CollectionRoute, primaryOriginRoute:
		return method == http.MethodGet
	case cartRoute:
		return method == http.MethodGet || method == http.MethodPut
	case quotesRoute, checkoutRoute:
		return method == http.MethodPost
	case paymentPrepareRoute, paymentHandoffRoute, paymentRefreshRoute, paymentCancelRoute:
		return method == http.MethodPost
	case destinationRoute:
		return method == http.MethodGet || method == http.MethodPut
	case quoteRoute, destinationItemRoute, orderRoute:
		return method == http.MethodGet
	case claimLinkRoute:
		return method == http.MethodGet
	case claimRedeemRoute:
		return method == http.MethodPost
	case privacyRoute:
		return method == http.MethodGet
	case consentsRoute:
		return method == http.MethodPut
	case privacyExportRoute, privacyErasureRoute:
		return method == http.MethodPost
	}
	return false
}

func legacyMatchCVSRoute(path string) route {
	switch path {
	case "/v1/buyer/cvs-selections":
		return route{kind: routeCVSSelectionOpen}
	case "/v1/buyer/cvs-stores":
		return route{kind: routeCVSStoreEnter}
	}
	if rest, ok := strings.CutPrefix(path, "/v1/buyer/cvs-selections/"); ok && rest != "" {
		if id, verify := strings.CutSuffix(rest, "/verify"); verify {
			if id != "" && !strings.Contains(id, "/") {
				return route{kind: routeCVSSelectionVerify, id: id}
			}
			return route{}
		}
		if !strings.Contains(rest, "/") {
			return route{kind: routeCVSSelectionGet, id: rest}
		}
	}
	return route{}
}

// legacyAllowedCVS: open and enter-store are POST with a key, verify is a keyless POST, get is a GET.
func legacyAllowedCVS(kind routeKind, method string) bool {
	switch kind {
	case routeCVSSelectionOpen, routeCVSSelectionVerify, routeCVSStoreEnter:
		return method == http.MethodPost
	case routeCVSSelectionGet:
		return method == http.MethodGet
	}
	return false
}

func legacyMatchTransferRoute(path string) route {
	rest, ok := strings.CutPrefix(path, "/v1/buyer/orders/")
	if !ok {
		return route{}
	}
	if id, matched := strings.CutSuffix(rest, "/bank-transfer/proof"); matched && id != "" && !strings.Contains(id, "/") {
		return route{kind: routeTransferProof, id: id}
	}
	if id, matched := strings.CutSuffix(rest, "/bank-transfer"); matched && id != "" && !strings.Contains(id, "/") {
		return route{kind: routeTransferGet, id: id}
	}
	return route{}
}

// legacyAllowedTransfer: the view is a keyless GET, the proof a keyed PUT (handler.go derives "write" from the method).
func legacyAllowedTransfer(kind routeKind, method string) bool {
	switch kind {
	case routeTransferGet:
		return method == http.MethodGet
	case routeTransferProof:
		return method == http.MethodPut
	}
	return false
}

func legacyMatchCatalogV2(rest string) route {
	switch {
	case rest == "products":
		return route{kind: catalogV2ProductsRoute}
	case rest == "collections":
		return route{kind: catalogV2CollectionsRoute}
	}
	if key, ok := strings.CutPrefix(rest, "products/"); ok && (v2Slug.MatchString(key) && len(key) <= 80 || command.ValidID(key)) {
		return route{kind: catalogV2ProductRoute, slug: key}
	}
	if key, ok := strings.CutPrefix(rest, "collections/"); ok && v2Slug.MatchString(key) && len(key) <= 80 && !command.ValidID(key) { // UUID-shaped slugs cannot exist (SQL CHECK)
		return route{kind: catalogV2CollectionRoute, slug: key}
	}
	return route{}
}
