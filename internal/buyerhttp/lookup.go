// lookup.go owns the one private buyer route of the guest order lookup (contracts/storefront-v2.md §E5): POST /v1/buyer/orders/lookup. The
// storefront BFF mints a fresh capability token exactly like session/prepare, sends it as the bearer with {order_ref, contact} and, only on a
// 200, sets it as the buyer cookie. Here the order number (12 hex, or a full id) and the buyer's email or delivery phone are normalised and
// hashed, and checkout.guest_order_lookup (commerce_buyer_issuer) throttles, matches and registers the token for the order's existing owner.
//
// It never learns whether an order exists from anything but a successful match: every mismatch (unknown order, wrong contact, other store,
// erased owner) is the same 404 not_found, the throttle counts before any read, and the comparison is digest against digest in SQL. It never
// trusts a store or tenant from the body (the store is the published-origin resolver's), never logs the order ref, the contact or the token,
// and never stores the client IP (only its sha256 inside a throttle bucket).
//
// Guest-lookup sessions are VIEW-ONLY (integrator ruling, contracts/storefront-v2.md §E5): the capability row carries view_order_id, and
// authorizeView below is the one shared gate (called by ServeHTTP for every authenticated route) that lets such a session do exactly four things
// and is closed by default for every other route, present or future.
// Route kind lives clear of handler.go's iota block (220), like cvs.go's and transfer.go's.

package buyerhttp

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	routeLookup  routeKind = 220
	lookupPath             = "/v1/buyer/orders/lookup"
	lookupWindow           = 600 // seconds; checkout.lookup_hit's fixed window
)

type lookupInput struct {
	OrderRef string `json:"order_ref"`
	Contact  string `json:"contact"`
}

// normalizeRef accepts the order number as shown in mail (XXXX-XXXX-XXXX, any case, spaces) or a full order id; it returns lowercase hex of
// length 12 or 32.
func normalizeRef(raw string) (string, bool) {
	if len(raw) == 0 || len(raw) > 64 {
		return "", false
	}
	var b strings.Builder
	for _, r := range strings.ToLower(raw) {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'f':
			b.WriteRune(r)
		case r == '-' || r == ' ':
		default:
			return "", false
		}
	}
	if n := b.Len(); n != 12 && n != 32 {
		return "", false
	}
	return b.String(), true
}

// contactDigest classifies the contact as an email (one @) or a phone and returns sha256 of its normal form, the same normal form SQL derives
// from the stored value: email lowercased; phone digits without a leading 886 and then without one leading 0.
func contactDigest(raw string) (kind string, digest [32]byte, ok bool) {
	c := strings.TrimSpace(raw)
	if c == "" || len(c) > 254 {
		return "", digest, false
	}
	if strings.Contains(c, "@") {
		at := strings.Index(c, "@")
		if at < 1 || at == len(c)-1 || strings.Count(c, "@") != 1 || strings.IndexFunc(c, func(r rune) bool { return r <= ' ' || r == 0x7f }) >= 0 {
			return "", digest, false
		}
		return "email", sha256.Sum256([]byte(strings.ToLower(c))), true
	}
	var d strings.Builder
	for _, r := range c {
		switch {
		case r >= '0' && r <= '9':
			d.WriteRune(r)
		case r == '+' || r == '(' || r == ')' || r == '-' || r == ' ':
		default:
			return "", digest, false
		}
	}
	if d.Len() < 6 || d.Len() > 20 {
		return "", digest, false
	}
	n := strings.TrimPrefix(strings.TrimPrefix(d.String(), "886"), "0")
	return "phone", sha256.Sum256([]byte(n)), true
}

// lookupClientIP reads the optional X-Commerce-Client-IP (one valid literal, as for password auth). Absent is allowed (the bucket is then the
// shared "none" bucket, strictly tighter); present and invalid is a refusal.
func lookupClientIP(r *http.Request) ([]byte, bool) {
	values := r.Header.Values("X-Commerce-Client-IP")
	if len(values) == 0 {
		return nil, true
	}
	if len(values) != 1 {
		return nil, false
	}
	ip, err := netip.ParseAddr(values[0])
	if err != nil || ip.Zone() != "" {
		return nil, false
	}
	sum := sha256.Sum256([]byte(ip.Unmap().String()))
	return sum[:], true
}

// orderLookup serves the route. token is the bearer the BFF just minted (canonical, validated by ServeHTTP); its sha256 becomes the new
// capability's hash, exactly as buyer.RegisterForTrustedStore does for bootstrap.
func (h *handler) orderLookup(ctx context.Context, w http.ResponseWriter, r *http.Request, storeID, token string) error {
	var in lookupInput
	if err := decodeJSON(r, &in); err != nil {
		return err
	}
	ref, refOK := normalizeRef(in.OrderRef)
	kind, digest, contactOK := contactDigest(in.Contact)
	ip, ipOK := lookupClientIP(r)
	if !refOK || !contactOK || !ipOK || h.issuerPool == nil || h.ttl < time.Minute || h.ttl > 30*24*time.Hour {
		return responseError{http.StatusUnprocessableEntity, "invalid_request"}
	}
	tokenHash := sha256.Sum256([]byte(token))
	var orderID string
	// checkout.guest_order_lookup (0090): throttle ip/ref/store, range-scan the order by id prefix, digest compare, issue_order_capability.
	err := h.issuerPool.QueryRow(ctx, `SELECT order_id::text FROM checkout.guest_order_lookup($1::uuid,$2,$3,$4,$5,$6::bigint,$7)`,
		storeID, ref, kind, digest[:], tokenHash[:], int64(h.ttl/time.Second), ip).Scan(&orderID)
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

// viewOnly asks buyer.session_view_order (commerce_buyer_issuer) whether the bearer is a view-only guest-lookup session and for which order.
// An unknown hash is simply not restricted: the route's own resolve_scope then refuses it exactly as before.
func (h *handler) viewOnly(ctx context.Context, storeID, token string) (orderID string, restricted bool, err error) {
	hash := sha256.Sum256([]byte(token))
	var found bool
	var view *string
	// buyer.session_view_order (0090): {found, view_order_id}; classification only, no expiry judgement.
	if err = h.issuerPool.QueryRow(ctx, `SELECT found,view_order_id::text FROM buyer.session_view_order($1,$2::uuid)`, hash[:], storeID).Scan(&found, &view); err != nil {
		return "", false, err
	}
	if !found || view == nil {
		return "", false, nil
	}
	return *view, true, nil
}

// viewAllowed is the whole allowlist of a view-only session: session status (GET), bootstrap, retire, logout (DELETE) and GET of its one order,
// that order's payment status and its bank-transfer instructions. Everything else (order list, other orders, any POST/PUT incl. payment prepare/handoff
// and the transfer proof, CVS, claims, consents, privacy export / erasure, cart, checkout) is closed.
func viewAllowed(selected route, method, viewOrder string) bool {
	switch selected.kind {
	case sessionRoute:
		return method == http.MethodGet || method == http.MethodDelete
	case bootstrapRoute, retireRoute:
		return method == http.MethodPost
	// the order, its payment status and its bank-transfer instructions: read-only GETs of the session's own order, so an unpaid order can be paid
	case orderRoute, paymentRoute, routeTransferGet:
		return method == http.MethodGet && selected.id == viewOrder
	}
	return false
}

// authorizeView is the shared authorization point for view-only sessions; it returns a 403 refusal for a restricted session on a closed route.
// Routes without a buyer bearer (media, design) never reach it.
func (h *handler) authorizeView(ctx context.Context, selected route, method, storeID, token string) error {
	if h.issuerPool == nil {
		return responseError{http.StatusServiceUnavailable, "unavailable"}
	}
	order, restricted, err := h.viewOnly(ctx, storeID, token)
	if err != nil {
		return responseError{http.StatusServiceUnavailable, "unavailable"}
	}
	if restricted && !viewAllowed(selected, method, order) {
		return responseError{http.StatusForbidden, "forbidden"}
	}
	return nil
}
