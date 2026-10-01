// orderlink.go owns the one private buyer route of the manual-order buyer link (contracts/storefront-v2.md section G3): POST /v1/buyer/orders/link.
// The merchant creates an order in the admin and pastes a link `<origin>/<locale>/order-link#o=<order>&t=<token>` to the customer. The storefront page
// reads the fragment in the browser (never in a URL sent anywhere) and posts it ONCE to the storefront BFF, which mints a fresh capability token exactly
// like session/prepare, sends it as the bearer with {order_id, token} and, only on a 200, sets it as the buyer cookie (the same pattern as the guest
// lookup, lookup.go). Here the link token is hashed and checkout.redeem_order_link (commerce_buyer_issuer) throttles, finds the single-use link row of that
// store and order, and issues a FULL capability (not view-only) for the order's owner: that owner belongs to this one manual order, so the buyer holds
// exactly the rights a checkout-issued capability for it has and can pay (bank-transfer proof, pay-at-pickup views).
//
// It never learns why a link is refused: unknown order, wrong token, other store, used, expired, inactive owner are the same 404 not_found. It never trusts a
// store or tenant from the body (the store is the published-origin resolver's), never logs the token and never stores the client IP (only its sha256 inside a
// throttle bucket, shared with the guest lookup).
// Route kind lives clear of handler.go's iota block (221), next to lookup.go's 220.

package buyerhttp

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/http"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"livecommerce/internal/command"
)

const (
	routeOrderLink routeKind = 221
	orderLinkPath            = "/v1/buyer/orders/link"
)

var linkTokenPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)

type orderLinkInput struct {
	OrderID string `json:"order_id"`
	Token   string `json:"token"`
}

// orderLink serves the route; token is the capability the BFF just minted (canonical, validated by ServeHTTP).
func (h *handler) orderLink(ctx context.Context, w http.ResponseWriter, r *http.Request, storeID, token string) error {
	var in orderLinkInput
	if err := decodeJSON(r, &in); err != nil {
		return err
	}
	ip, ipOK := lookupClientIP(r)
	if !command.ValidID(in.OrderID) || !linkTokenPattern.MatchString(in.Token) || !ipOK || h.issuerPool == nil || h.ttl < time.Minute || h.ttl > 30*24*time.Hour {
		return responseError{http.StatusUnprocessableEntity, "invalid_request"}
	}
	linkHash := sha256.Sum256([]byte(in.Token))
	tokenHash := sha256.Sum256([]byte(token))
	var orderID string
	// checkout.redeem_order_link (0094): throttle ip/order/store, single-use <= 7 day link by (store, order, hash), then buyer.issue_owner_capability.
	err := h.issuerPool.QueryRow(ctx, `SELECT order_id::text FROM checkout.redeem_order_link($1::uuid,$2::uuid,$3,$4,$5::bigint,$6)`,
		storeID, in.OrderID, linkHash[:], tokenHash[:], int64(h.ttl/time.Second), ip).Scan(&orderID)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	var pg *pgconn.PgError
	switch {
	case err == nil:
	case errors.Is(err, pgx.ErrNoRows):
		return responseError{http.StatusNotFound, "not_found"}
	case errors.As(err, &pg) && pg.Code == "PT429":
		return codedResponse{responseError{http.StatusTooManyRequests, "rate_limited"}, lookupWindow - int(time.Now().Unix()%lookupWindow)}
	case errors.As(err, &pg) && pg.Code == "PT400":
		return responseError{http.StatusUnprocessableEntity, "invalid_request"}
	default:
		return responseError{http.StatusServiceUnavailable, "unavailable"}
	}
	writeOK(w, struct {
		OrderID string `json:"order_id"`
	}{orderID})
	return nil
}
