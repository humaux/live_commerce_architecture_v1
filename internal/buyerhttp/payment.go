package buyerhttp

import (
	"context"
	"net/http"

	"livecommerce/internal/checkout"
)

type paymentPrepareInput struct {
	MethodCode    string `json:"method_code"`
	MethodVersion int64  `json:"method_version"`
	Locale        string `json:"locale"`
}

type paymentPrepareResponse struct {
	OrderID     string `json:"order_id"`
	State       string `json:"state"`
	Currency    string `json:"currency"`
	AmountMinor int64  `json:"amount_minor"`
}

func isPaymentRoute(kind routeKind) bool {
	return kind == paymentRoute || kind == paymentPrepareRoute || kind == paymentHandoffRoute
}

func (h *handler) paymentRequest(ctx context.Context, r *http.Request, selected route, storeID, token, key string) (any, error) {
	switch selected.kind {
	case paymentRoute:
		return h.payment.PaymentView(ctx, token, storeID, selected.id)
	case paymentPrepareRoute:
		var in paymentPrepareInput
		if err := decodeJSON(r, &in); err != nil {
			return nil, err
		}
		if in.MethodCode != "payuni_credit" || in.MethodVersion < 1 ||
			(in.Locale != "zh-CN" && in.Locale != "zh-TW" && in.Locale != "en") {
			return nil, responseError{http.StatusUnprocessableEntity, "invalid_request"}
		}
		result, err := h.payment.BeginHosted(ctx, token, storeID, key, checkout.HostedInput{
			OrderID: selected.id, MethodCode: in.MethodCode,
			MethodVersion: in.MethodVersion, Locale: in.Locale})
		if err != nil {
			return nil, err
		}
		return paymentPrepareResponse{OrderID: result.OrderID, State: result.State,
			Currency: result.Currency, AmountMinor: result.AmountMinor}, nil
	case paymentHandoffRoute:
		return h.payment.TakeHosted(ctx, token, storeID, selected.id)
	default:
		return nil, responseError{http.StatusNotFound, "not_found"}
	}
}
