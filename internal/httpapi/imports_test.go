// imports_test.go is the DB-free test of the W5-02B customer-import routes: it builds the FULL router (NewHandler, so a pattern
// conflict with any other route panics here, before PostgreSQL is needed), the transport rules that hold before any database work
// (method, Content-Type, strict query, no Idempotency-Key, ids) and the error-code table. Real-PG outcomes are CI01-CI09 in
// tests/foundation/customer_import_test.go.

package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"livecommerce/internal/httperror"
	"livecommerce/internal/migrationimport"
)

func TestCustomerImportRoutesTransportRules(t *testing.T) {
	h := NewHandler(nil) // the whole router: route conflicts panic at registration
	base := "/v1/admin/stores/" + custStore + "/imports"
	csvBody := "customer_id,name\nc-1,Test One\n"
	asCSV := func(r *http.Request) { r.Header.Set("Content-Type", "text/csv") }
	withKey := func(r *http.Request) { asCSV(r); r.Header.Set("Idempotency-Key", "imp-key-0001") }
	noBearer := func(r *http.Request) { asCSV(r); r.Header.Del("Authorization") }
	for _, tc := range []struct {
		name, method, path, body string
		edit                     func(*http.Request)
		status                   int
		code                     string
	}{
		// Admitted requests reach platform.WithScopeBudget (nil pool -> 401 unauthorized).
		{"preview", "POST", base + "/customers/preview", csvBody, asCSV, 401, "unauthorized"},
		{"preview with mapping", "POST", base + "/customers/preview?mapping=%7B%22name%22%3A%22Name%22%7D", csvBody, asCSV, 401, "unauthorized"},
		{"commit", "POST", base + "/customers/commit?expected_apply_rows=1", csvBody, asCSV, 401, "unauthorized"},
		{"results", "GET", base + "/" + custID + "/results.csv", "", nil, 401, "unauthorized"},
		{"results only failed", "GET", base + "/" + custID + "/results.csv?only=failed", "", nil, 401, "unauthorized"},
		// Bearer, Content-Type, Idempotency-Key (the file hash is the idempotency key: a header is refused).
		{"preview no bearer", "POST", base + "/customers/preview", csvBody, noBearer, 401, "unauthorized"},
		{"preview json body", "POST", base + "/customers/preview", csvBody, func(r *http.Request) { r.Header.Set("Content-Type", "application/json") }, 415, "invalid_request"},
		{"preview no content type", "POST", base + "/customers/preview", csvBody, func(r *http.Request) { r.Header.Del("Content-Type") }, 415, "invalid_request"},
		{"preview with key", "POST", base + "/customers/preview", csvBody, withKey, 422, "invalid_request"},
		{"commit with key", "POST", base + "/customers/commit?expected_apply_rows=1", csvBody, withKey, 422, "invalid_request"},
		// Query grammar.
		{"preview unknown query", "POST", base + "/customers/preview?x=1", csvBody, asCSV, 422, "invalid_request"},
		{"preview bad mapping json", "POST", base + "/customers/preview?mapping=nope", csvBody, asCSV, 422, "invalid_request"},
		{"preview mapping null", "POST", base + "/customers/preview?mapping=null", csvBody, asCSV, 422, "invalid_request"},
		{"preview mapping too long", "POST", base + "/customers/preview?mapping=" + strings.Repeat("a", 2049), csvBody, asCSV, 422, "invalid_request"},
		{"preview repeated mapping", "POST", base + "/customers/preview?mapping=%7B%7D&mapping=%7B%7D", csvBody, asCSV, 422, "invalid_request"},
		{"commit no count", "POST", base + "/customers/commit", csvBody, asCSV, 422, "invalid_request"},
		{"commit empty count", "POST", base + "/customers/commit?expected_apply_rows=", csvBody, asCSV, 422, "invalid_request"},
		{"commit negative count", "POST", base + "/customers/commit?expected_apply_rows=-1", csvBody, asCSV, 422, "invalid_request"},
		{"commit count above cap", "POST", base + "/customers/commit?expected_apply_rows=5001", csvBody, asCSV, 422, "invalid_request"},
		{"commit text count", "POST", base + "/customers/commit?expected_apply_rows=many", csvBody, asCSV, 422, "invalid_request"},
		{"results unknown filter", "GET", base + "/" + custID + "/results.csv?only=ok", "", nil, 422, "invalid_request"},
		{"results unknown query", "GET", base + "/" + custID + "/results.csv?x=1", "", nil, 422, "invalid_request"},
		{"results bad batch id", "GET", base + "/NOT-A-UUID/results.csv", "", nil, 422, "invalid_request"},
		{"results with key", "GET", base + "/" + custID + "/results.csv", "", func(r *http.Request) { r.Header.Set("Idempotency-Key", "imp-key-0001") }, 422, "invalid_request"},
		// Methods.
		{"preview get", "GET", base + "/customers/preview", "", nil, 405, ""},
		{"commit put", "PUT", base + "/customers/commit?expected_apply_rows=1", csvBody, asCSV, 405, ""},
		{"results post", "POST", base + "/" + custID + "/results.csv", csvBody, asCSV, 405, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var r *http.Request
			if tc.body == "" {
				r = httptest.NewRequest(tc.method, tc.path, nil)
			} else {
				r = httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			}
			r.Header.Set("Authorization", custBearer)
			if tc.edit != nil {
				tc.edit(r)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("status=%d want=%d body=%s", w.Code, tc.status, w.Body.String())
			}
			if tc.code != "" {
				var envelope httperror.Envelope
				if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil || envelope.Code != tc.code {
					t.Fatalf("code=%q want=%q", envelope.Code, tc.code)
				}
			}
			if tc.status != 405 && w.Header().Get("Cache-Control") != "private, no-store" {
				t.Fatalf("Cache-Control=%q", w.Header().Get("Cache-Control"))
			}
		})
	}
}

// Every coded refusal of the importer must be in httperror's message table, or respondError rewrites it to "internal".
func TestCustomerImportErrorCodesAreInTheTable(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{migrationimport.NothingToApply, 422, "nothing_to_apply"},
		{migrationimport.ErrUnavailable, 503, "retry_later"},
		{&migrationimport.Error{Status: 422, Code: "too_many_rows"}, 422, "too_many_rows"},
		{&migrationimport.Error{Status: 422, Code: "encoding_not_utf8"}, 422, "encoding_not_utf8"},
		{&migrationimport.Error{Status: 422, Code: "required"}, 422, "required"},
		{&migrationimport.Error{Status: 409, Code: "idempotency_conflict"}, 409, "idempotency_conflict"},
	} {
		status, code := importClassify(tc.err)
		if status != tc.status || code != tc.code {
			t.Fatalf("%v: %d %s want %d %s", tc.err, status, code, tc.status, tc.code)
		}
		rec := httptest.NewRecorder()
		respondError(rec, status, code)
		var envelope httperror.Envelope
		if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil || envelope.Code != tc.code || envelope.Message == "" {
			t.Fatalf("%s is not in the httperror table: %+v", tc.code, envelope)
		}
	}
}
