// Purpose: private buyer return-request transport; full capability only, no view-only or merchant mutation routes.
// Depends on: existing BFF/origin/capability admission, buyer.WithScope and internal/returns typed wrappers.
// Used by: handler.dispatch through the closed routeRows inventory.
// Invariants: strict JSON, no caller tenant/store, no PII logging, COMMIT before success, no money movement.
package buyerhttp

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/buyer"
	"livecommerce/internal/platform"
	"livecommerce/internal/returns"
)

const (
	routeReturnRequestCreate routeKind = 230 + iota
	routeReturnRequestList
	routeReturnRequestDetail
	routeReturnRequestWithdraw
)

func isReturnRequestRoute(k routeKind) bool {
	return k >= routeReturnRequestCreate && k <= routeReturnRequestWithdraw
}
func (h *handler) returnRequest(ctx context.Context, r *http.Request, selected route, store, token, key string) (any, error) {
	var in returns.RequestInput
	var decision struct {
		ExpectedVersion int64 `json:"expected_version"`
	}
	if r.Method == http.MethodGet {
		if r.URL.RawQuery != "" || r.URL.ForceQuery {
			return nil, responseError{422, "invalid_request"}
		}
		if err := noBody(r); err != nil {
			return nil, err
		}
	} else if selected.kind == routeReturnRequestCreate {
		if err := decodeJSON(r, &in); err != nil {
			return nil, err
		}
	} else if err := decodeJSON(r, &decision); err != nil {
		return nil, err
	}
	if r.URL.RawQuery != "" || r.URL.ForceQuery {
		return nil, responseError{422, "invalid_request"}
	}
	out, err := scoped(ctx, h.pool, token, store, func(c context.Context, tx pgx.Tx, _ buyer.Scope) (any, error) {
		switch selected.kind {
		case routeReturnRequestCreate:
			value, e := returns.CreateBuyerRequest(c, tx, token, store, selected.id, key, in)
			return createdResponse{Body: value}, e
		case routeReturnRequestWithdraw:
			return returns.DecideBuyerRequest(c, tx, token, store, selected.id, key, "withdraw", returns.RequestDecision{ExpectedVersion: decision.ExpectedVersion}, platform.Scope{})
		default:
			return returns.ReadBuyerRequests(c, tx, token, store, selected.id, false, platform.Scope{})
		}
	})
	if err != nil {
		var coded *returns.Error
		switch {
		case errors.As(err, &coded):
			return nil, responseError{coded.Status, coded.Code}
		case errors.Is(err, platform.ErrUnauthorized):
			return nil, responseError{401, "unauthorized"}
		case errors.Is(err, platform.ErrScopeNotFound):
			return nil, responseError{404, "not_found"}
		}
	}
	return out, err
}
