// transfer.go owns the two private buyer routes of the bank_transfer payment mode (contracts/storefront-v2.md §C): GET
// /v1/buyer/orders/{id}/bank-transfer (the order page's bank details, deadline, state and the buyer's last proof) and PUT
// /v1/buyer/orders/{id}/bank-transfer/proof (the "I have transferred" form: last 5 digits, amount, paid_at; keyed, replaceable until the
// merchant confirms). The BFF mirrors them under /api/buyer/.
//
// It never authenticates buyers itself (handler.go does: BFF key, storefront origin, bearer capability), decides no transfer rule (the SQL
// definers behind checkout.Service do: owner scope, window, state, idempotency), and never logs a body, an account number or a token.
// Route kinds live clear of handler.go's iota block (210+), like cvs.go's.

package buyerhttp

import (
	"context"
	"net/http"
	"strings"

	"livecommerce/internal/checkout"
)

const (
	routeTransferGet routeKind = 210 + iota
	routeTransferProof
)

func isTransferRoute(kind routeKind) bool {
	return kind == routeTransferGet || kind == routeTransferProof
}

// matchTransferRoute is the path table of the two routes; handler.go's matchRoute delegates to it before the generic /orders/{id} prefix.
func matchTransferRoute(path string) route {
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

// allowedTransfer: the view is a keyless GET, the proof a keyed PUT (handler.go derives "write" from the method).
func allowedTransfer(kind routeKind, method string) bool {
	switch kind {
	case routeTransferGet:
		return method == http.MethodGet
	case routeTransferProof:
		return method == http.MethodPut
	}
	return false
}

// transferRequest serves the two kinds. The PUT body is strict (decodeJSON: JSON media type, <= 64 KiB, no unknown key, no null).
func (h *handler) transferRequest(ctx context.Context, r *http.Request, selected route, storeID, token, key string) (any, error) {
	switch selected.kind {
	case routeTransferGet:
		return h.checkout.TransferView(ctx, token, storeID, selected.id)
	case routeTransferProof:
		var in checkout.ProofInput
		if err := decodeJSON(r, &in); err != nil {
			return nil, err
		}
		return h.checkout.SubmitTransferProof(ctx, token, storeID, key, selected.id, in)
	}
	return nil, responseError{http.StatusNotFound, "not_found"}
}
