// Package buyerhttp is the private, BFF-only buyer transport. It borrows its
// database pools; publication and buyer authority are resolved per request.
package buyerhttp

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/buyer"
	"livecommerce/internal/checkout"
	"livecommerce/internal/command"
	"livecommerce/internal/domains"
	"livecommerce/internal/httperror"
	"livecommerce/internal/pagination"
	"livecommerce/internal/platform"
	"livecommerce/internal/storefront"
)

const maxJSON = 64 << 10

var idempotencyKey = regexp.MustCompile(`^[A-Za-z0-9_.:-]{8,128}$`)

type handler struct {
	resolver *domains.Resolver
	issuer   *buyer.Service
	pool     *pgxpool.Pool
	checkout *checkout.Service
	bffKey   string
}

// New validates both borrowed authorities and builds the private transport.
// The caller retains ownership of every pool and the checkout service.
func New(ctx context.Context, issuerPool, buyerPool *pgxpool.Pool, checkoutService *checkout.Service, bffKey string, ttl time.Duration) (http.Handler, error) {
	if ctx == nil || checkoutService == nil || !canonicalSecret(bffKey) {
		return nil, command.ErrInvalid
	}
	issuer, err := buyer.New(issuerPool, ttl)
	if err != nil {
		return nil, err
	}
	resolver, err := domains.New(ctx, issuerPool) // validates issuer authority
	if err != nil {
		return nil, err
	}
	if err = platform.ValidateBuyerPool(ctx, buyerPool); err != nil {
		return nil, err
	}
	return httperror.Middleware(&handler{resolver: resolver, issuer: issuer, pool: buyerPool, checkout: checkoutService, bffKey: bffKey}), nil
}

type routeKind uint8

const (
	unknownRoute routeKind = iota
	sessionRoute
	bootstrapRoute
	retireRoute
	catalogRoute
	optionsRoute
	cartRoute
	quotesRoute
	quoteRoute
	destinationRoute
	destinationItemRoute
	checkoutRoute
	orderRoute
)

type route struct {
	kind routeKind
	id   string
}

func matchRoute(path string) route {
	switch path {
	case "/v1/buyer/session":
		return route{kind: sessionRoute}
	case "/v1/buyer/session/bootstrap":
		return route{kind: bootstrapRoute}
	case "/v1/buyer/session/retire":
		return route{kind: retireRoute}
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

func allowed(kind routeKind, method string) bool {
	switch kind {
	case sessionRoute:
		return method == http.MethodGet || method == http.MethodPost || method == http.MethodDelete
	case bootstrapRoute, retireRoute:
		return method == http.MethodPost
	case catalogRoute, optionsRoute:
		return method == http.MethodGet
	case cartRoute:
		return method == http.MethodGet || method == http.MethodPut
	case quotesRoute, checkoutRoute:
		return method == http.MethodPost
	case destinationRoute:
		return method == http.MethodPut
	case quoteRoute, destinationItemRoute, orderRoute:
		return method == http.MethodGet
	}
	return false
}

func canonicalSecret(value string) bool {
	if len(value) != 43 {
		return false
	}
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(value)
	return err == nil && len(decoded) == 32 && base64.RawURLEncoding.EncodeToString(decoded) == value
}

func oneHeader(r *http.Request, name string) (string, bool) {
	values := r.Header.Values(name)
	returnValue := ""
	if len(values) == 1 {
		returnValue = values[0]
	}
	return returnValue, len(values) == 1
}

func forbiddenInput(r *http.Request) bool {
	queryRoute := r.URL != nil && r.Method == http.MethodGet && r.URL.EscapedPath() == r.URL.Path &&
		(r.URL.Path == "/v1/buyer/catalog" || r.URL.Path == "/v1/buyer/checkout-options")
	if r.URL == nil || r.URL.ForceQuery || (!queryRoute && r.URL.RawQuery != "") {
		return true
	}
	for _, name := range []string{"Cookie", "Origin", "X-Tenant-ID", "X-Store-ID"} {
		if len(r.Header.Values(name)) != 0 {
			return true
		}
	}
	return false
}

func catalogRequest(raw string) (storefront.CatalogRequest, error) {
	var request storefront.CatalogRequest
	if len(raw) > 2048 || strings.Contains(raw, ";") || strings.HasPrefix(raw, "&") || strings.HasSuffix(raw, "&") || strings.Contains(raw, "&&") {
		return request, command.ErrInvalid
	}
	values, err := url.ParseQuery(raw)
	if err != nil {
		return request, command.ErrInvalid
	}
	for name, values := range values {
		if len(values) != 1 || values[0] == "" {
			return request, command.ErrInvalid
		}
		switch name {
		case "product_id":
			if !command.ValidID(values[0]) {
				return request, command.ErrInvalid
			}
			request.ProductID = values[0]
		case "limit":
			if len(values[0]) > 3 {
				return request, command.ErrInvalid
			}
			for _, digit := range values[0] {
				if digit < '0' || digit > '9' {
					return request, command.ErrInvalid
				}
			}
			request.Page.Limit, err = strconv.Atoi(values[0])
			if err != nil || request.Page.Limit < 1 || request.Page.Limit > 100 {
				return request, command.ErrInvalid
			}
		case "cursor":
			request.Page.Cursor = values[0]
		default:
			return request, command.ErrInvalid
		}
	}
	return request, nil
}

func optionsRequest(raw string) (checkout.OptionsRequest, error) {
	var request checkout.OptionsRequest
	if len(raw) > 2048 || strings.Contains(raw, ";") || strings.HasPrefix(raw, "&") || strings.HasSuffix(raw, "&") || strings.Contains(raw, "&&") {
		return request, command.ErrInvalid
	}
	values, err := url.ParseQuery(raw)
	if err != nil {
		return request, command.ErrInvalid
	}
	for name, entries := range values {
		if len(entries) != 1 || entries[0] == "" {
			return request, command.ErrInvalid
		}
		value := entries[0]
		switch name {
		case "market_id":
			if !command.ValidID(value) {
				return request, command.ErrInvalid
			}
			request.MarketID = value
		case "country":
			if len(value) != 2 || value[0] < 'A' || value[0] > 'Z' || value[1] < 'A' || value[1] > 'Z' {
				return request, command.ErrInvalid
			}
			request.Country = value
		case "limit":
			if len(value) > 3 {
				return request, command.ErrInvalid
			}
			for _, digit := range value {
				if digit < '0' || digit > '9' {
					return request, command.ErrInvalid
				}
			}
			request.Page.Limit, err = strconv.Atoi(value)
			if err != nil || request.Page.Limit < 1 || request.Page.Limit > 100 {
				return request, command.ErrInvalid
			}
		case "cursor":
			request.Page.Cursor = value
		default:
			return request, command.ErrInvalid
		}
	}
	return request, nil
}

func bearer(r *http.Request) (string, bool) {
	value, one := oneHeader(r, "Authorization")
	if !one || !strings.HasPrefix(value, "Bearer ") {
		return "", false
	}
	token := strings.TrimPrefix(value, "Bearer ")
	return token, canonicalSecret(token)
}

func keyFor(r *http.Request, noReplayKey bool, write bool) (string, bool) {
	values := r.Header.Values("Idempotency-Key")
	if noReplayKey || !write {
		return "", len(values) == 0
	}
	returnValue, one := oneHeader(r, "Idempotency-Key")
	return returnValue, one && idempotencyKey.MatchString(returnValue)
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	issue := r.Method == http.MethodPost && r.URL != nil && r.URL.Path == "/v1/buyer/session"
	fail := func(status int, code string) {
		if issue {
			httperror.WriteNonRetryable(w, status, code)
		} else {
			httperror.Write(w, status, code)
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	deadline, _ := ctx.Deadline()
	controller := http.NewResponseController(w)
	_ = controller.SetReadDeadline(deadline)
	_ = controller.SetWriteDeadline(deadline)
	if forbiddenInput(r) {
		fail(http.StatusForbidden, "forbidden")
		return
	}
	if r.URL.EscapedPath() != r.URL.Path {
		fail(http.StatusNotFound, "not_found")
		return
	}
	providedKey, one := oneHeader(r, "X-Commerce-Buyer-BFF-Key")
	if !one || !canonicalSecret(providedKey) || subtle.ConstantTimeCompare([]byte(providedKey), []byte(h.bffKey)) != 1 {
		fail(http.StatusUnauthorized, "unauthorized")
		return
	}
	origin, one := oneHeader(r, "X-Commerce-Storefront-Origin")
	if !one || origin == "" {
		fail(http.StatusUnprocessableEntity, "invalid_request")
		return
	}
	selected := matchRoute(r.URL.Path)
	if selected.kind == unknownRoute {
		fail(http.StatusNotFound, "not_found")
		return
	}
	if !allowed(selected.kind, r.Method) {
		fail(http.StatusMethodNotAllowed, "method_not_allowed")
		return
	}
	if selected.id != "" && !command.ValidID(selected.id) {
		fail(http.StatusUnprocessableEntity, "invalid_request")
		return
	}
	noReplayKey := issue || selected.kind == bootstrapRoute || selected.kind == retireRoute
	write := r.Method == http.MethodPut || (r.Method == http.MethodPost && !noReplayKey)
	key, valid := keyFor(r, noReplayKey, write)
	if !valid {
		fail(http.StatusUnprocessableEntity, "invalid_request")
		return
	}
	token := ""
	if issue {
		if len(r.Header.Values("Authorization")) != 0 {
			fail(http.StatusUnauthorized, "unauthorized")
			return
		}
	} else {
		var ok bool
		token, ok = bearer(r)
		if !ok {
			fail(http.StatusUnauthorized, "unauthorized")
			return
		}
	}
	routeInfo, err := h.resolver.Resolve(ctx, origin)
	if err != nil {
		status, code := classify(err)
		fail(status, code)
		return
	}
	if err = h.dispatch(ctx, w, r, selected, routeInfo.StoreID, token, key); err != nil {
		status, code := classify(err)
		fail(status, code)
	}
}

type responseError struct {
	status int
	code   string
}

func (e responseError) Error() string { return e.code }

func (h *handler) dispatch(ctx context.Context, w http.ResponseWriter, r *http.Request, selected route, storeID, token, key string) error {
	if selected.kind == sessionRoute && r.Method == http.MethodPost {
		if err := decodeJSON(r, &struct{}{}); err != nil {
			return err
		}
		capability, err := h.issuer.IssueForTrustedStore(ctx, storeID)
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		writeOK(w, struct {
			Token     string    `json:"token"`
			ExpiresAt time.Time `json:"expires_at"`
		}{capability.Token, capability.ExpiresAt})
		return nil
	}
	if selected.kind == bootstrapRoute {
		if err := decodeJSON(r, &struct{}{}); err != nil {
			return err
		}
		capability, err := h.issuer.RegisterForTrustedStore(ctx, storeID, token)
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		writeOK(w, struct {
			Authenticated bool      `json:"authenticated"`
			ExpiresAt     time.Time `json:"expires_at"`
		}{true, capability.ExpiresAt})
		return nil
	}
	if selected.kind == retireRoute {
		if err := decodeJSON(r, &struct{}{}); err != nil {
			return err
		}
		if err := h.issuer.RetireForTrustedStore(ctx, storeID, token); err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		w.WriteHeader(http.StatusNoContent)
		return nil
	}
	if r.Method == http.MethodGet || r.Method == http.MethodDelete {
		if err := noBody(r); err != nil {
			return err
		}
	}
	if selected.kind == sessionRoute {
		if r.Method == http.MethodDelete {
			if err := h.issuer.Revoke(ctx, token, storeID); err != nil {
				return err
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			w.WriteHeader(http.StatusNoContent)
			return nil
		}
		_, err := scoped(ctx, h.pool, token, storeID, func(context.Context, pgx.Tx, buyer.Scope) (bool, error) { return true, nil })
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		writeOK(w, struct {
			Authenticated bool `json:"authenticated"`
		}{true})
		return nil
	}
	var out any
	var err error
	switch selected.kind {
	case optionsRoute:
		var request checkout.OptionsRequest
		request, err = optionsRequest(r.URL.RawQuery)
		if err == nil {
			var page pagination.Page[checkout.Option]
			page, err = h.checkout.ListOptions(ctx, token, storeID, request)
			if err == nil {
				out = projectOptions(page)
			}
		}
	case catalogRoute:
		var request storefront.CatalogRequest
		request, err = catalogRequest(r.URL.RawQuery)
		if err == nil {
			var page pagination.Page[storefront.CatalogItem]
			page, err = scoped(ctx, h.pool, token, storeID, func(c context.Context, tx pgx.Tx, s buyer.Scope) (pagination.Page[storefront.CatalogItem], error) {
				return storefront.ListCatalog(c, tx, s, request)
			})
			if err == nil {
				out = projectCatalog(page)
			}
		}
	case cartRoute:
		if r.Method == http.MethodGet {
			out, err = scoped(ctx, h.pool, token, storeID, storefront.GetCart)
		} else {
			var in storefront.CartInput
			if err = decodeJSON(r, &in); err == nil {
				out, err = scoped(ctx, h.pool, token, storeID, func(c context.Context, tx pgx.Tx, s buyer.Scope) (storefront.Cart, error) {
					return storefront.SetCart(c, tx, s, key, in)
				})
			}
		}
		if err == nil {
			out = projectCart(out.(storefront.Cart))
		}
	case quotesRoute:
		var in storefront.QuoteInput
		if err = decodeJSON(r, &in); err == nil {
			out, err = scoped(ctx, h.pool, token, storeID, func(c context.Context, tx pgx.Tx, s buyer.Scope) (storefront.Quote, error) {
				return storefront.CreateQuote(c, tx, s, key, in)
			})
		}
		if err == nil {
			out = projectQuote(out.(storefront.Quote))
		}
	case quoteRoute:
		out, err = scoped(ctx, h.pool, token, storeID, func(c context.Context, tx pgx.Tx, s buyer.Scope) (storefront.Quote, error) {
			return storefront.GetQuote(c, tx, s, selected.id)
		})
		if err == nil {
			out = projectQuote(out.(storefront.Quote))
		}
	case destinationRoute:
		var in storefront.DestinationInput
		if err = decodeJSON(r, &in); err == nil {
			out, err = scoped(ctx, h.pool, token, storeID, func(c context.Context, tx pgx.Tx, s buyer.Scope) (storefront.Destination, error) {
				return storefront.SetDestination(c, tx, s, key, in)
			})
		}
		if err == nil {
			out = projectDestination(out.(storefront.Destination))
		}
	case destinationItemRoute:
		out, err = scoped(ctx, h.pool, token, storeID, func(c context.Context, tx pgx.Tx, s buyer.Scope) (storefront.Destination, error) {
			return storefront.GetDestination(c, tx, s, selected.id)
		})
		if err == nil {
			out = projectDestination(out.(storefront.Destination))
		}
	case checkoutRoute:
		var in checkout.Input
		if err = decodeJSON(r, &in); err == nil {
			var result checkout.Result
			result, err = h.checkout.Begin(ctx, token, storeID, key, in)
			if err == nil {
				out = projectCheckout(result)
			}
		}
	case orderRoute:
		var result checkout.Order
		result, err = h.checkout.Get(ctx, token, storeID, selected.id)
		if err == nil {
			out = projectOrder(result)
		}
	}
	if err != nil {
		return err
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	writeOK(w, out)
	return nil
}

func scoped[T any](ctx context.Context, pool *pgxpool.Pool, token, storeID string, fn func(context.Context, pgx.Tx, buyer.Scope) (T, error)) (T, error) {
	var out T
	err := buyer.WithScope(ctx, pool, token, storeID, func(c context.Context, tx pgx.Tx, s buyer.Scope) error {
		var operationError error
		out, operationError = fn(c, tx, s)
		return operationError
	})
	return out, err
}

func noBody(r *http.Request) error {
	if r.ContentLength > 0 {
		return responseError{http.StatusUnprocessableEntity, "invalid_request"}
	}
	if r.Body == nil {
		return nil
	}
	var one [1]byte
	n, err := io.ReadFull(r.Body, one[:])
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	if bodyTimeout(err) {
		return context.DeadlineExceeded
	}
	if n != 0 || err != nil && !errors.Is(err, io.EOF) {
		return responseError{http.StatusUnprocessableEntity, "invalid_request"}
	}
	return nil
}

func decodeJSON(r *http.Request, value any) error {
	contentType, one := oneHeader(r, "Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if !one || err != nil || mediaType != "application/json" {
		return responseError{http.StatusUnsupportedMediaType, "json_required"}
	}
	if r.ContentLength > maxJSON {
		return responseError{http.StatusUnprocessableEntity, "invalid_request"}
	}
	if r.Body == nil {
		return responseError{http.StatusBadRequest, "invalid_json"}
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxJSON+1))
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	if bodyTimeout(err) {
		return context.DeadlineExceeded
	}
	if err != nil {
		return responseError{http.StatusBadRequest, "invalid_json"}
	}
	if len(body) > maxJSON {
		return responseError{http.StatusUnprocessableEntity, "invalid_request"}
	}
	if !json.Valid(body) {
		return responseError{http.StatusBadRequest, "invalid_json"}
	}
	// Token scanning sees null even in duplicate object keys that an ordinary
	// map decode would overwrite.
	scanner := json.NewDecoder(bytes.NewReader(body))
	for {
		token, scanErr := scanner.Token()
		if errors.Is(scanErr, io.EOF) {
			break
		}
		if scanErr != nil || token == nil {
			return responseError{http.StatusBadRequest, "invalid_json"}
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(value); err != nil {
		return responseError{http.StatusBadRequest, "invalid_json"}
	}
	return nil
}

func classify(err error) (int, string) {
	var response responseError
	if errors.As(err, &response) {
		return response.status, response.code
	}
	switch {
	case errors.Is(err, buyer.ErrUnauthorized):
		return http.StatusUnauthorized, "unauthorized"
	case errors.Is(err, buyer.ErrRateLimited):
		return http.StatusTooManyRequests, "rate_limited"
	case errors.Is(err, domains.ErrInvalid), errors.Is(err, buyer.ErrInvalid), errors.Is(err, command.ErrInvalid):
		return http.StatusUnprocessableEntity, "invalid_request"
	case errors.Is(err, platform.ErrForbidden):
		return http.StatusForbidden, "forbidden"
	case errors.Is(err, domains.ErrUnavailable), errors.Is(err, command.ErrNotFound):
		return http.StatusNotFound, "not_found"
	case errors.Is(err, command.ErrInsufficient):
		return http.StatusConflict, "insufficient_inventory"
	case errors.Is(err, command.ErrConflict):
		return http.StatusConflict, "conflict"
	default:
		return http.StatusServiceUnavailable, "unavailable"
	}
}

func bodyTimeout(err error) bool {
	var timed net.Error
	return errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &timed) && timed.Timeout())
}

func writeOK(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(value)
}
