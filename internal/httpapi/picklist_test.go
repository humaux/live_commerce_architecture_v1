// Purpose: the DB-free transport gates of the W3-02B pick-list routes (unit w3-02b-picklist). These
//   tables pin the classifier, the carrier-template query parse and the {order_ids} XOR {session_id}
//   selection decode without a database: every refusal below is decided in Go before any SQL.
// Depends on: merchantorders.ErrPickListTooMany/ErrUnavailable/ValidCarrierTemplate/PickListSelection,
//   command.ErrInvalid, platform.ErrUnauthorized/ErrForbidden/ErrScopeNotFound, studioDecodeRaw.
// Used by: nothing else (pure unit table tests).
// Invariants: 501 ids and a >500-order session both classify 422 too_many; an unclassified error is
//   503 unavailable, never 500 internal; the export query is exactly one template key.

package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"livecommerce/internal/command"
	"livecommerce/internal/merchantorders"
	"livecommerce/internal/platform"
)

const (
	plStore = "22222222-2222-4222-8222-222222222222"
	plOrder = "33333333-3333-4333-8333-333333333333"
)

func TestPicklistClassify(t *testing.T) {
	for _, tc := range []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"too_many", merchantorders.ErrPickListTooMany, http.StatusUnprocessableEntity, "too_many"},
		{"unauthorized", platform.ErrUnauthorized, http.StatusUnauthorized, "unauthorized"},
		{"forbidden", platform.ErrForbidden, http.StatusForbidden, "forbidden"},
		{"scope not found", platform.ErrScopeNotFound, http.StatusNotFound, "not_found"},
		{"invalid", command.ErrInvalid, http.StatusUnprocessableEntity, "invalid_request"},
		{"unavailable sentinel", merchantorders.ErrUnavailable, http.StatusServiceUnavailable, "unavailable"},
		{"deadline", context.DeadlineExceeded, http.StatusServiceUnavailable, "retry_later"},
		{"unclassified", errors.New("driver detail must not leak"), http.StatusServiceUnavailable, "unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			status, code := picklistClassify(tc.err)
			if status != tc.wantStatus || code != tc.wantCode {
				t.Fatalf("classify(%v) = %d %q, want %d %q", tc.err, status, code, tc.wantStatus, tc.wantCode)
			}
		})
	}
}

func TestExportTemplate(t *testing.T) {
	for _, tc := range []struct {
		query  string
		want   string
		wantOK bool
	}{
		{"?template=black_cat", "black_cat", true},
		{"?template=hsinchu", "hsinchu", true},
		{"?template=chunghwa_post", "chunghwa_post", true},
		{"?template=generic", "generic", true},
		{"?template=UPS", "", false},
		{"", "", false},
		{"?template=black_cat&extra=1", "", false},
		{"?template=black_cat&template=hsinchu", "", false},
		{"?template=", "", false},
	} {
		u, err := url.Parse(tc.query)
		if err != nil {
			t.Fatalf("parse %q: %v", tc.query, err)
		}
		got, ok := exportTemplate(u)
		if ok != tc.wantOK || got != tc.want {
			t.Fatalf("exportTemplate(%q) = %q,%v want %q,%v", tc.query, got, ok, tc.want, tc.wantOK)
		}
	}
}

func TestPickListSelection(t *testing.T) {
	for _, tc := range []struct {
		name        string
		body        string
		contentType string
		wantStatus  int // 0 means ok
		wantOrders  int
		wantSession string
	}{
		{"order_ids", `{"order_ids":["` + plOrder + `"]}`, "application/json", 0, 1, ""},
		{"session_id", `{"session_id":"` + plOrder + `"}`, "application/json", 0, 0, plOrder},
		{"both selectors", `{"order_ids":["` + plOrder + `"],"session_id":"` + plOrder + `"}`, "application/json", http.StatusUnprocessableEntity, 0, ""},
		{"neither selector", `{}`, "application/json", http.StatusUnprocessableEntity, 0, ""},
		{"empty order_ids", `{"order_ids":[]}`, "application/json", http.StatusUnprocessableEntity, 0, ""},
		{"empty session_id", `{"session_id":""}`, "application/json", http.StatusUnprocessableEntity, 0, ""},
		{"wrong media", `{"order_ids":["` + plOrder + `"]}`, "text/plain", http.StatusUnsupportedMediaType, 0, ""},
		{"unknown key", `{"order_ids":["` + plOrder + `"],"nope":1}`, "application/json", http.StatusBadRequest, 0, ""},
		{"duplicate key", `{"order_ids":["` + plOrder + `"],"order_ids":["` + plOrder + `"]}`, "application/json", http.StatusBadRequest, 0, ""},
		{"invalid json", `{`, "application/json", http.StatusBadRequest, 0, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v1/admin/stores/"+plStore+"/orders/pick-list", strings.NewReader(tc.body))
			if tc.contentType != "" {
				req.Header.Set("Content-Type", tc.contentType)
			}
			rec := httptest.NewRecorder()
			sel, ok := pickListSelection(rec, req)
			if tc.wantStatus != 0 {
				if ok || rec.Code != tc.wantStatus {
					t.Fatalf("selection = %+v,ok=%v status=%d, want status %d", sel, ok, rec.Code, tc.wantStatus)
				}
				return
			}
			if !ok || len(sel.OrderIDs) != tc.wantOrders || sel.SessionID != tc.wantSession {
				t.Fatalf("selection = %+v,ok=%v, want orders=%d session=%q", sel, ok, tc.wantOrders, tc.wantSession)
			}
		})
	}
}
