package buyerhttp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"livecommerce/internal/checkout"
)

type paymentPrepareInput struct {
	MethodCode    string `json:"method_code"`
	MethodVersion int64  `json:"method_version"`
	Locale        string `json:"locale"`
}

func (in *paymentPrepareInput) UnmarshalJSON(body []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	open, err := decoder.Token()
	if err != nil || open != json.Delim('{') {
		return errors.New("invalid payment input")
	}
	seen := make(map[string]bool, 3)
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		name, ok := token.(string)
		if !ok || seen[name] {
			return errors.New("duplicate or invalid payment field")
		}
		seen[name] = true
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return err
		}
		if bytes.Equal(value, []byte("null")) {
			return errors.New("null payment field")
		}
		switch name {
		case "method_code":
			err = json.Unmarshal(value, &in.MethodCode)
		case "method_version":
			err = json.Unmarshal(value, &in.MethodVersion)
		case "locale":
			err = json.Unmarshal(value, &in.Locale)
		default:
			return errors.New("unknown payment field")
		}
		if err != nil {
			return err
		}
	}
	end, err := decoder.Token()
	if err != nil || end != json.Delim('}') || len(seen) != 3 {
		return errors.New("incomplete payment input")
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return errors.New("trailing payment input")
	}
	return nil
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
