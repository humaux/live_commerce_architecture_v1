// storefront_test.go covers the storefront publication transport rules that hold before any database work. A request
// that passes every transport rule reaches platform.WithScope with a nil pool and is answered 401, which these tests
// use as the "admitted to the transaction" marker (same convention as claimsource_test.go). Real-PG behaviour
// (compare-and-set, audit, grants, scope) is the independent gate suite's job.

package httpapi

import (
	"errors"
	"net/http"
	"testing"

	"livecommerce/internal/storefrontadmin"
)

func TestStorefrontTransportRulesBeforeDatabase(t *testing.T) {
	h := NewHandler(nil)
	base := "/v1/admin/stores/" + claimsStore + "/storefront"
	body := `{"published":true,"expected_version":0}`
	for _, tc := range []struct {
		name, method, path, body string
		edit                     func(*http.Request)
		status                   int
	}{
		{"GET admitted", "GET", base, "", nil, 401},
		{"GET query", "GET", base + "?x=1", "", nil, 422},
		{"GET bare question mark", "GET", base + "?", "", nil, 422},
		{"GET no bearer", "GET", base, "", func(r *http.Request) { r.Header.Del("Authorization") }, 401},
		{"POST admitted", "POST", base + "/publication", body, nil, 401},
		{"POST query", "POST", base + "/publication?x=1", body, nil, 422},
		{"POST media type", "POST", base + "/publication", body, func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") }, 415},
		{"POST unknown key", "POST", base + "/publication", `{"published":true,"expected_version":0,"tenant_id":"x"}`, nil, 400},
		{"POST duplicate-free trailing value", "POST", base + "/publication", body + ` {}`, nil, 400},
		{"POST not json", "POST", base + "/publication", `nope`, nil, 400},
		{"POST on the read resource", "POST", base, body, nil, 405},
		{"GET on the write resource", "GET", base + "/publication", "", nil, 405},
		// the operator half has no merchant route of any spelling
		{"no domain route POST", "POST", base + "/domains", body, nil, 404},
		{"no domain route PUT", "PUT", base + "/domain", body, nil, 404},
		{"no domain bind", "POST", base + "/domains/bind", body, nil, 404},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := claimsCall(h, tc.method, tc.path, tc.body, tc.edit)
			if w.Code != tc.status {
				t.Fatalf("status=%d want=%d body=%s", w.Code, tc.status, w.Body.String())
			}
		})
	}
}

func TestStorefrontUnavailableMapsTo503(t *testing.T) {
	if status, code := classify(storefrontadmin.ErrUnavailable); status != http.StatusServiceUnavailable || code != "unavailable" {
		t.Fatalf("classify(ErrUnavailable) = %d %s", status, code)
	}
	if status, _ := classify(errors.New("other")); status != http.StatusInternalServerError {
		t.Fatalf("unmapped error = %d", status)
	}
}
