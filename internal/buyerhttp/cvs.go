// cvs.go owns the four private buyer CVS routes of taiwan-cvs-logistics-v1 §5.2 / §16.1: POST /v1/buyer/cvs-selections (201),
// GET /v1/buyer/cvs-selections/{id}, POST /v1/buyer/cvs-selections/{id}/verify (keyless, no body) and POST /v1/buyer/cvs-stores (201).
// It never authenticates buyers itself (handler.go does: BFF key, storefront origin, bearer capability), decides no CVS rule (the SQL
// definers behind checkout.BuyerCVS do), and never logs a body, a store code or a token.
//
// Wiring (integrator hook): handler.go adds the four route kinds below to matchRoute / allowed / keyFor (verify is keyless) and
// dispatch calls cvsRequest. The BuyerCVS is reached through checkout.Service.CVS() (set by cmd/api with WithBuyerCVS), so the handler
// constructor does not change. Route -> Go endpoint: the BFF mirrors these under /api/buyer/.

package buyerhttp

import (
	"context"
	"errors"
	"net/http"

	"livecommerce/internal/checkout"
	"livecommerce/internal/fulfillment"
	"livecommerce/internal/promotions"
)

// The CVS route kinds live clear of handler.go's iota block so the two files never collide.
const (
	routeCVSSelectionOpen routeKind = 200 + iota
	routeCVSSelectionGet
	routeCVSSelectionVerify
	routeCVSStoreEnter
)

// isCVSRoute reports the four kinds this file serves.
func isCVSRoute(kind routeKind) bool {
	return kind >= routeCVSSelectionOpen && kind <= routeCVSStoreEnter
}

// createdResponse marks a body the dispatcher must send with 201 (POST cvs-selections, POST cvs-stores).
type createdResponse struct{ Body any }

// matchCVSRoute and allowedCVS retain the resource helpers used by existing tests;
// both consume the shared route table instead of a second path/method inventory.
func matchCVSRoute(path string) route {
	selected := matchRoute(path)
	if isCVSRoute(selected.kind) {
		return selected
	}
	return route{}
}

func allowedCVS(kind routeKind, method string) bool {
	return isCVSRoute(kind) && allowed(kind, method)
}

// codedResponse is a responseError that also carries a Retry-After (429 pay_at_pickup_limit, rate_limited). It unwraps to its
// responseError, so handler.go's classify (errors.As on responseError) maps it unchanged; ServeHTTP reads RetryAfterSeconds.
type codedResponse struct {
	responseError
	RetryAfterSeconds int
}

func (e codedResponse) Unwrap() error { return e.responseError }

// cvsHTTPError turns a coded refusal into the handler's response error; anything else passes to the shared classifier.
func cvsHTTPError(err error) error {
	var refusal *fulfillment.CVSError
	if errors.As(err, &refusal) {
		coded := responseError{refusal.Status, refusal.Code}
		if refusal.RetryAfter > 0 {
			return codedResponse{coded, refusal.RetryAfter}
		}
		return coded
	}
	// storefront-v2 §F: a discount-code refusal (quote-time check or BeginCheckout's redeem) is a coded 422 promo_* like a CVS refusal.
	var promo *promotions.Coded
	if errors.As(err, &promo) {
		return responseError{promo.Status, promo.Code}
	}
	return err
}

// storefrontOrigin is the BFF-authenticated origin (handler.go already required exactly one value and resolved it to this store);
// open_cvs_selection re-checks that it is an ACTIVE published domain of the store.
func storefrontOrigin(r *http.Request) string {
	value, _ := oneHeader(r, "X-Commerce-Storefront-Origin")
	return value
}

// mobileHint is the BFF's optional B20 hint: X-Commerce-Device: mobile selects the ECPay map Device=1 (buyers arrive from Facebook /
// Instagram on phones); anything else keeps the desktop map. It never carries an identity.
func mobileHint(r *http.Request) bool {
	value, one := oneHeader(r, "X-Commerce-Device")
	return one && value == "mobile"
}

// cvsRequest serves the four kinds. Bodies are strict (decodeJSON: JSON media type, <= 64 KiB, no unknown key, no null); the verify POST
// takes neither body nor key. It answers 404 when the CVS surface is not wired.
func (h *handler) cvsRequest(ctx context.Context, r *http.Request, selected route, storeID, token, key string) (any, error) {
	b := h.checkout.CVS()
	if b == nil {
		return nil, responseError{http.StatusNotFound, "not_found"}
	}
	out, err := h.cvsServe(ctx, b, r, selected, storeID, token, key)
	return out, cvsHTTPError(err)
}

func (h *handler) cvsServe(ctx context.Context, b *checkout.BuyerCVS, r *http.Request, selected route, storeID, token, key string) (any, error) {
	switch selected.kind {
	case routeCVSSelectionOpen:
		var in checkout.SelectionOpenInput
		if err := decodeJSON(r, &in); err != nil {
			return nil, err
		}
		opened, err := b.Open(ctx, token, storeID, key, storefrontOrigin(r), mobileHint(r), in)
		if err != nil {
			return nil, err
		}
		return createdResponse{Body: opened}, nil
	case routeCVSSelectionGet:
		return b.Get(ctx, token, storeID, selected.id)
	case routeCVSSelectionVerify:
		if err := noBody(r); err != nil {
			return nil, err
		}
		return b.Verify(ctx, token, storeID, selected.id)
	case routeCVSStoreEnter:
		var in checkout.StoreEntryInput
		if err := decodeJSON(r, &in); err != nil {
			return nil, err
		}
		entered, err := b.EnterStore(ctx, token, storeID, key, in)
		if err != nil {
			return nil, err
		}
		return createdResponse{Body: entered}, nil
	}
	return nil, responseError{http.StatusNotFound, "not_found"}
}
