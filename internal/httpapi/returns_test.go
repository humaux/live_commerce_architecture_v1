package httpapi

// Purpose: DB-free transport and routing tests of the W3-08B routes (returns.go): the whole router builds (no pattern conflict with
//   orders/{order_id}), the gates refuse before any database work, and every coded refusal reaches the JSON body with its own message.
// Depends on: NewHandler(nil), shipments_test.go helpers (shipmentCall, shipStore, shipOrder), httperror, internal/returns.
// Used by: go test ./internal/httpapi (check-gates).

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"livecommerce/internal/httperror"
	"livecommerce/internal/returns"
)

const (
	retBase  = "/v1/admin/stores/" + shipStore
	retRMA   = "44444444-4444-4444-8444-444444444444"
	retKey   = "ret-key-0001"
	retLines = `[{"sku_id":"66666666-6666-4666-8666-666666666666","warehouse_id":null,"quantity":1}]`
)

func TestReturnRoutesTransportRules(t *testing.T) {
	h := NewHandler(nil) // the whole router: a route conflict with orders/{order_id} panics at registration
	order := retBase + "/orders/" + shipOrder
	rma := retBase + "/returns/" + retRMA
	withKey := func(r *http.Request) { r.Header.Set("Idempotency-Key", retKey) }
	for _, tc := range []struct {
		name, method, path, body string
		edit                     func(*http.Request)
		status                   int
		code                     string
	}{
		{"cancel admitted", "POST", order + "/cancel", `{"expected_state":"CONFIRMED","reason":"out of stock"}`, withKey, 401, "unauthorized"},
		{"register admitted", "POST", order + "/returns", `{"reason":"damaged","lines":` + retLines + `}`, withKey, 401, "unauthorized"},
		{"order returns admitted", "GET", order + "/returns", "", nil, 401, "unauthorized"},
		{"list admitted", "GET", retBase + "/returns", "", nil, 401, "unauthorized"},
		{"list state admitted", "GET", retBase + "/returns?state=CLOSED", "", nil, 401, "unauthorized"},
		{"receive admitted", "POST", rma + "/receive", `{"expected_version":1,"lines":[{"sku_id":"66666666-6666-4666-8666-666666666666","warehouse_id":null,"qty_received":1}]}`, withKey, 401, "unauthorized"},
		{"inspect admitted", "POST", rma + "/inspect", `{"expected_version":2,"lines":[{"sku_id":"66666666-6666-4666-8666-666666666666","warehouse_id":null,"qty_restock":1,"qty_scrap":0}]}`, withKey, 401, "unauthorized"},
		{"close admitted", "POST", rma + "/close", `{"expected_version":3,"refund_id":null}`, withKey, 401, "unauthorized"},
		{"rma cancel admitted", "POST", rma + "/cancel", `{"expected_version":1}`, withKey, 401, "unauthorized"},
		{"cancel no key", "POST", order + "/cancel", `{"expected_state":"CONFIRMED","reason":"x"}`, nil, 422, "invalid_request"},
		{"cancel unknown key", "POST", order + "/cancel", `{"expected_state":"CONFIRMED","reason":"x","tenant_id":"t"}`, withKey, 400, "invalid_json"},
		{"cancel duplicate key", "POST", order + "/cancel", `{"expected_state":"CONFIRMED","expected_state":"DRAFT","reason":"x"}`, withKey, 400, "invalid_json"},
		{"cancel query", "POST", order + "/cancel?x=1", `{"expected_state":"CONFIRMED","reason":"x"}`, withKey, 422, "invalid_request"},
		{"cancel bad order id", "POST", retBase + "/orders/NOT-A-UUID/cancel", `{"expected_state":"CONFIRMED","reason":"x"}`, withKey, 422, "invalid_request"},
		{"cancel wrong method", "GET", order + "/cancel", "", nil, 405, "method_not_allowed"},
		{"register no key", "POST", order + "/returns", `{"reason":"damaged","lines":` + retLines + `}`, nil, 422, "invalid_request"},
		{"register bad line field", "POST", order + "/returns", `{"reason":"damaged","lines":[{"sku_id":"66666666-6666-4666-8666-666666666666","qty":1}]}`, withKey, 400, "invalid_json"},
		{"order returns key", "GET", order + "/returns", "", withKey, 422, "invalid_request"},
		{"order returns query", "GET", order + "/returns?x=1", "", nil, 422, "invalid_request"},
		{"list extra query", "GET", retBase + "/returns?state=CLOSED&x=1", "", nil, 422, "invalid_request"},
		{"list other query", "GET", retBase + "/returns?x=1", "", nil, 422, "invalid_request"},
		{"list key", "GET", retBase + "/returns", "", withKey, 422, "invalid_request"},
		{"receive no key", "POST", rma + "/receive", `{"expected_version":1,"lines":[]}`, nil, 422, "invalid_request"},
		{"receive bad rma id", "POST", retBase + "/returns/NOT-A-UUID/receive", `{"expected_version":1,"lines":[]}`, withKey, 422, "invalid_request"},
		{"close unknown key", "POST", rma + "/close", `{"expected_version":3,"x":1}`, withKey, 400, "invalid_json"},
		{"rma cancel wrong method", "DELETE", rma + "/cancel", "", nil, 405, "method_not_allowed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := shipmentCall(h, tc.method, tc.path, tc.body, tc.edit)
			if w.Code != tc.status {
				t.Fatalf("status=%d want=%d body=%s", w.Code, tc.status, w.Body.String())
			}
			var envelope httperror.Envelope
			if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil || envelope.Code != tc.code {
				t.Fatalf("code=%q want=%q", envelope.Code, tc.code)
			}
		})
	}
	// The sibling literal route and the single-order routes still win over {order_id}.
	if w := shipmentCall(h, "GET", shipOrders+"/unshipped.csv", "", nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("unshipped.csv answered %d", w.Code)
	}
}

// Every refusal code of the returns/cancel definers must reach the JSON body with its own message and its own status (never the
// generic internal fallback), and an unmapped definer message must stay a 503.
func TestReturnCodesReachJSONBody(t *testing.T) {
	cases := map[string]string{}
	for _, c := range []string{"not_shipped", "not_returnable", "version_changed", "invalid_state", "not_restockable", "has_returns",
		"already_cancelled", "state_changed", "payment_in_flight", "refund_first", "already_shipped", "cvs_attempt_in_flight", "payment_review_open"} {
		cases[c] = "PT409"
	}
	for _, c := range []string{"unknown_line", "ambiguous_line", "exceeds_shipped", "exceeds_registered", "invalid_quantities", "quantities_mismatch",
		"lines_incomplete", "nothing_received", "refund_mismatch", "not_cancellable"} {
		cases[c] = "PT422"
	}
	for code, pt := range cases {
		t.Run(code, func(t *testing.T) {
			status, got := returnsClassify(returns.MapError(&pgconn.PgError{Code: pt, Message: code}))
			rec := httptest.NewRecorder()
			respondError(rec, status, got)
			var body httperror.Envelope
			want := map[string]int{"PT409": 409, "PT422": 422}[pt]
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || status != want || body.Code != code ||
				body.Message == "" || body.Message == "Request could not be completed." {
				t.Fatalf("status=%d code=%q message=%q err=%v", status, body.Code, body.Message, err)
			}
		})
	}
	if status, code := returnsClassify(returns.ErrUnavailable); status != http.StatusServiceUnavailable || code != "unavailable" {
		t.Fatalf("unavailable: %d %s", status, code)
	}
	if status, code := returnsClassify(returns.MapError(&pgconn.PgError{Code: "PT409", Message: "something_new"})); status != http.StatusServiceUnavailable || code != "unavailable" {
		t.Fatalf("unmapped PT409 message: %d %s", status, code)
	}
	if status, code := returnsClassify(returns.MapError(&pgconn.PgError{Code: "PT409", Message: "collection_state_changed"})); status != http.StatusConflict || code != "state_changed" {
		t.Fatalf("collection_state_changed: %d %s", status, code)
	}
}
