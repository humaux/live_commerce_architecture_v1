package httpapi

// Purpose: DB-free transport and routing tests of the W3-07B parcel routes (parcels.go): the whole router builds (no pattern
//   conflict with orders/{order_id}), the gates refuse before any database work, and every coded refusal reaches the JSON body.
// Depends on: NewHandler(nil), shipments_test.go helpers (shipmentCall, shipBearer, shipStore, shipBodyOK), httperror.
// Used by: go test ./internal/httpapi (check-gates).

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"livecommerce/internal/httperror"
	"livecommerce/internal/merchantorders"
)

const (
	parcelBase  = "/v1/admin/stores/" + shipStore
	parcelGroup = "44444444-4444-4444-8444-444444444444"
	parcelBody  = `{"order_ids":["33333333-3333-4333-8333-333333333333","55555555-5555-4555-8555-555555555555"]}`
)

func TestParcelRoutesTransportRules(t *testing.T) {
	h := NewHandler(nil) // the whole router: a route conflict with orders/{order_id} panics at registration
	suggest := parcelBase + "/orders/merge-suggestions"
	groups := parcelBase + "/parcel-groups"
	one := groups + "/" + parcelGroup
	ship := one + "/shipment"
	noKey := func(r *http.Request) { r.Header.Del("Idempotency-Key") }
	withKey := func(r *http.Request) { r.Header.Set("Idempotency-Key", "grp-key-0001") } // shipmentCall only sets it on PUT
	for _, tc := range []struct {
		name, method, path, body string
		edit                     func(*http.Request)
		status                   int
		code                     string
	}{
		{"suggestions admitted", "GET", suggest, "", nil, 401, "unauthorized"},
		{"create admitted", "POST", groups, parcelBody, withKey, 401, "unauthorized"},
		{"dissolve admitted", "DELETE", one + "?expected_version=1", "", nil, 401, "unauthorized"},
		{"ship admitted", "PUT", ship, shipBodyOK, nil, 401, "unauthorized"},
		{"suggestions query", "GET", suggest + "?x=1", "", nil, 422, "invalid_request"},
		{"suggestions key", "GET", suggest, "", func(r *http.Request) { r.Header.Set("Idempotency-Key", "ship-key-0001") }, 422, "invalid_request"},
		{"create no key", "POST", groups, parcelBody, nil, 422, "invalid_request"},
		{"create missing order_ids", "POST", groups, `{}`, withKey, 422, "invalid_request"},
		{"create unknown key", "POST", groups, `{"order_ids":[],"tenant_id":"x"}`, withKey, 400, "invalid_json"},
		{"create query", "POST", groups + "?x=1", parcelBody, withKey, 422, "invalid_request"},
		{"dissolve no version", "DELETE", one, "", nil, 422, "invalid_request"},
		{"dissolve zero version", "DELETE", one + "?expected_version=0", "", nil, 422, "invalid_request"},
		{"dissolve extra query", "DELETE", one + "?expected_version=1&x=1", "", nil, 422, "invalid_request"},
		{"dissolve bad id", "DELETE", groups + "/NOT-A-UUID?expected_version=1", "", nil, 422, "invalid_request"},
		{"ship no key", "PUT", ship, shipBodyOK, noKey, 422, "invalid_request"},
		{"ship bad group id", "PUT", groups + "/NOT-A-UUID/shipment", shipBodyOK, nil, 422, "invalid_request"},
		{"ship missing key in body", "PUT", ship, `{"status":"SHIPPED"}`, nil, 422, "invalid_request"},
		{"ship query", "PUT", ship + "?x=1", shipBodyOK, nil, 422, "invalid_request"},
		{"get group shipment", "GET", ship, "", nil, 405, "method_not_allowed"},
		{"post dissolve", "POST", one, "", nil, 405, "method_not_allowed"},
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
	// The sibling literal route still wins over {order_id} and the single-order routes keep working.
	w := shipmentCall(h, "GET", shipOrders+"/unshipped.csv", "", nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unshipped.csv answered %d", w.Code)
	}
}

// Every parcel refusal code must reach the JSON body with its own message and a 409 (never the generic internal fallback).
func TestParcelCodesReachJSONBody(t *testing.T) {
	for _, code := range []string{"in_parcel_group", "cod_not_mergeable", "cvs_not_mergeable", "not_mergeable", "already_in_group",
		"owner_mismatch", "destination_mismatch", "group_not_open", "group_incomplete"} {
		t.Run(code, func(t *testing.T) {
			status, got := shipmentClassify(&merchantorders.ParcelError{Code: code})
			rec := httptest.NewRecorder()
			respondError(rec, status, got)
			var body httperror.Envelope
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || status != http.StatusConflict || body.Code != code ||
				body.Message == "" || body.Message == "Request could not be completed." {
				t.Fatalf("status=%d code=%q message=%q err=%v", status, body.Code, body.Message, err)
			}
		})
	}
}
