// imports_test.go is the DB-free test of the W5-02B customer-import routes: it builds the FULL router (NewHandler, so a pattern
// conflict with any other route panics here, before PostgreSQL is needed), the transport rules that hold before any database work
// (method, Content-Type, strict query, no Idempotency-Key, ids) and the error-code table. Real-PG outcomes are CI01-CI15 in
// tests/foundation/customer_import_test.go.

package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"livecommerce/internal/httperror"
	"livecommerce/internal/migrationimport"
)

// §4.1/6.1: one projection serves preview 200 and fresh stale 409; neither capability may alter its domain input.
func TestImportPreviewResponseCapabilityAndCopy(t *testing.T) {
	preview := migrationimport.CustomerPreview{Rows: []migrationimport.PreviewRow{
		{Row: 1, Outcome: "failed", Code: "required", ErrorField: "external_id"},
		{Row: 2, Outcome: "failed", Code: "required", ErrorField: "name", ConsentIgnored: true},
		{Row: 3, Outcome: "created", ErrorField: "external_id"},
		{Row: 4, Outcome: "failed", Code: "invalid_name", ErrorField: "name"},
		{Row: 5, Outcome: "failed", Code: "required", ErrorField: "synthetic-header"},
		{Row: 6, Outcome: "failed", Code: "required"}, // legacy absence remains valid
	}}
	before, err := json.Marshal(preview)
	if err != nil {
		t.Fatal(err)
	}
	for _, enabled := range []bool{false, true} {
		got := importPreviewResponse(preview, enabled).(migrationimport.CustomerPreview)
		wire, err := json.Marshal(got)
		if err != nil {
			t.Fatal(err)
		}
		var raw struct {
			Rows []map[string]json.RawMessage `json:"rows"`
		}
		if err := json.Unmarshal(wire, &raw); err != nil {
			t.Fatal(err)
		}
		for i, row := range raw.Rows {
			want := ""
			if enabled && i < 2 {
				want = []string{`"external_id"`, `"name"`}[i]
			}
			if string(row["error_field"]) != want {
				t.Errorf("enabled=%v row=%d error_field=%s want=%s", enabled, i+1, row["error_field"], want)
			}
		}
		if !enabled {
			row, _ := json.Marshal(got.Rows[0])
			if string(row) != `{"row":1,"external_id":"","outcome":"failed","code":"required"}` {
				t.Errorf("legacy row shape changed: %s", row)
			}
		}
		got.Rows[0].Code = "synthetic-mutation"
		after, err := json.Marshal(preview)
		if err != nil || string(after) != string(before) {
			t.Fatalf("projection mutated input: enabled=%v err=%v", enabled, err)
		}
	}
	for _, result := range []any{migrationimport.CustomerCommitResult{BatchID: "synthetic-batch"}, migrationimport.OrderPreview{}} {
		if !reflect.DeepEqual(importPreviewResponse(result, true), result) {
			t.Fatalf("non-customer-preview changed: %T", result)
		}
	}
	for _, rows := range [][]migrationimport.PreviewRow{nil, {}} {
		input := migrationimport.CustomerPreview{Rows: rows}
		if !reflect.DeepEqual(importPreviewResponse(input, false), input) {
			t.Fatal("nil/empty rows shape changed")
		}
	}
}

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
		// W5-03B: the order-history rows follow the same transport rules.
		{"orders preview", "POST", base + "/orders/preview", csvBody, asCSV, 401, "unauthorized"},
		{"orders commit", "POST", base + "/orders/commit?expected_apply_rows=1", csvBody, asCSV, 401, "unauthorized"},
		{"orders preview with key", "POST", base + "/orders/preview", csvBody, withKey, 422, "invalid_request"},
		{"orders preview json body", "POST", base + "/orders/preview", csvBody, func(r *http.Request) { r.Header.Set("Content-Type", "application/json") }, 415, "invalid_request"},
		{"orders preview unknown query", "POST", base + "/orders/preview?x=1", csvBody, asCSV, 422, "invalid_request"},
		{"orders commit no count", "POST", base + "/orders/commit", csvBody, asCSV, 422, "invalid_request"},
		{"orders commit count above cap", "POST", base + "/orders/commit?expected_apply_rows=5001", csvBody, asCSV, 422, "invalid_request"},
		{"orders preview get", "GET", base + "/orders/preview", "", nil, 405, ""},
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

// A canonical synthetic bearer passes the outer route guard before query
// validation. Valid queries then reach the nil-pool scope refusal (401), while
// invalid queries return 422. Without a bearer the outer guard always returns 401.
func TestCustomerImportRowErrorsCapabilityTransport(t *testing.T) {
	h := NewHandler(nil)
	base := "/v1/admin/stores/" + custStore + "/imports"
	for _, endpoint := range []string{"/customers/preview", "/customers/commit?expected_apply_rows=1"} {
		sep := "?"
		if strings.Contains(endpoint, "?") {
			sep = "&"
		}
		for _, query := range []string{"row_errors=field-v1", "row_errors=unknown"} {
			t.Run("no bearer"+endpoint+"/"+query, func(t *testing.T) {
				r := httptest.NewRequest("POST", base+endpoint+sep+query, strings.NewReader("customer_id,name\nFIELD-1,Synthetic\n"))
				r.Header.Set("Content-Type", "text/csv")
				w := httptest.NewRecorder()
				h.ServeHTTP(w, r)
				var envelope httperror.Envelope
				if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil || w.Code != 401 || envelope.Code != "unauthorized" {
					t.Fatalf("missing bearer: status=%d code=%q err=%v", w.Code, envelope.Code, err)
				}
				if w.Header().Get("Cache-Control") != "private, no-store" {
					t.Fatal("private cache policy changed")
				}
			})
		}
		for _, tc := range []struct {
			query  string
			status int
		}{
			{"row_errors=field-v1", 401},
			{"row%5Ferrors=field-v1", 401},
			{"row_errors=%66ield-v1", 401},
			{"row_errors=", 422},
			{"row_errors=unknown", 422},
			{"row_errors=FIELD-V1", 422},
			{"row_errors=%20field-v1", 422},
			{"row_errors=field-v1%20", 422},
			{"Row_errors=field-v1", 422},
			{"row_errors%5B%5D=field-v1", 422},
			{"row_errors=field-v1,other", 422},
			{"row_errors=field-v1&row_errors=field-v1", 422},
			{"row_errors=field-v1&row%5Ferrors=field-v1", 422},
			{"row_errors=field-v1&row_errors=other", 422},
			{"row_errors=%ZZ", 422},
			{"row_errors=field-v1&x=1", 422},
			{"row_errors=" + strings.Repeat("a", 4097), 422},
		} {
			t.Run(endpoint+"/"+tc.query, func(t *testing.T) {
				r := httptest.NewRequest("POST", base+endpoint+sep+tc.query, strings.NewReader("customer_id,name\nFIELD-1,Synthetic\n"))
				r.Header.Set("Authorization", custBearer)
				r.Header.Set("Content-Type", "text/csv")
				w := httptest.NewRecorder()
				h.ServeHTTP(w, r)
				var envelope httperror.Envelope
				wantCode := "invalid_request"
				if tc.status == 401 {
					wantCode = "unauthorized"
				}
				if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil || w.Code != tc.status || envelope.Code != wantCode {
					t.Fatalf("status=%d code=%q want=%d/%s err=%v", w.Code, envelope.Code, tc.status, wantCode, err)
				}
				if w.Header().Get("Cache-Control") != "private, no-store" {
					t.Fatal("private cache policy changed")
				}
			})
		}
	}
	for _, endpoint := range []string{"/orders/preview?", "/orders/commit?expected_apply_rows=1&", "/" + custID + "/results.csv?"} {
		t.Run("unsupported"+endpoint, func(t *testing.T) {
			method := "POST"
			if strings.Contains(endpoint, "results.csv") {
				method = "GET"
			}
			r := httptest.NewRequest(method, base+endpoint+"row_errors=field-v1", strings.NewReader("order_id,name\nFIELD-1,Synthetic\n"))
			r.Header.Set("Authorization", custBearer)
			r.Header.Set("Content-Type", "text/csv")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			var envelope httperror.Envelope
			if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil || w.Code != 422 || envelope.Code != "invalid_request" {
				t.Fatalf("unsupported capability: %d %s", w.Code, w.Body.String())
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

// W5-03B: the archive read of one customer. Same router, customers:read, ids and query validated before any database work.
func TestHistoricalOrdersRouteTransportRules(t *testing.T) {
	h := NewHandler(nil)
	path := "/v1/admin/stores/" + custStore + "/customers/" + custID + "/historical-orders"
	for _, tc := range []struct {
		name, method, path string
		status             int
	}{
		{"read", "GET", path, 401},
		{"read with paging", "GET", path + "?limit=10", 401},
		{"bad customer id", "GET", "/v1/admin/stores/" + custStore + "/customers/NOT-A-UUID/historical-orders", 422},
		{"search filter refused", "GET", path + "?q=a", 422},
		{"tag filter refused", "GET", path + "?tag=" + custID, 422},
		{"post", "POST", path, 405},
	} {
		r := httptest.NewRequest(tc.method, tc.path, nil)
		r.Header.Set("Authorization", custBearer)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Errorf("%s: status=%d want=%d body=%s", tc.name, w.Code, tc.status, w.Body.String())
		}
	}
}
