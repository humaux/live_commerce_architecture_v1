// Purpose: DB-free transport rules of the W6-05B operations-ledger routes, built through the full NewHandler so a route conflict fails before PostgreSQL is needed, plus the error mapping.
// Depends on: claims_test.go helpers (claimsCall, claimsStore, claimsOther, claimsBearer); with a nil pool an admitted request is answered 401 at platform.WithScope.
// Used by: go test ./internal/httpapi.
// Invariants: exact POST body {"expected_attempts": n} (unknown/missing/negative rejected before any DB work), Idempotency-Key on every POST and none on GET, query/retry unmounted without the River client.

package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"livecommerce/internal/httperror"
	"livecommerce/internal/integrations/core"
)

func operationsHandler(t *testing.T, withJobs bool) http.Handler {
	t.Helper()
	if !withJobs {
		return NewHandler(nil, Options{})
	}
	jobs, err := river.NewClient[pgx.Tx](riverpgxv5.New(nil), &river.Config{Schema: "river"})
	if err != nil {
		t.Fatal(err)
	}
	return NewHandler(nil, Options{OperationJobs: jobs})
}

func TestOperationRoutesTransportRules(t *testing.T) {
	h := operationsHandler(t, true)
	base := "/v1/admin/stores/" + claimsStore + "/operations"
	one := base + "/" + claimsOther
	noKey := func(r *http.Request) { r.Header.Del("Idempotency-Key") }
	for _, tc := range []struct {
		name, method, path, body string
		edit                     func(*http.Request)
		status                   int
		code                     string
	}{
		{"list", "GET", base, "", nil, 401, "unauthorized"},
		{"list filtered", "GET", base + "?state=UNKNOWN&limit=20&cursor=abc", "", nil, 401, "unauthorized"},
		{"list unknown parameter", "GET", base + "?store_id=" + claimsOther, "", nil, 422, "invalid_request"},
		{"list duplicate state", "GET", base + "?state=UNKNOWN&state=READY", "", nil, 422, "invalid_request"},
		{"list empty state", "GET", base + "?state=", "", nil, 422, "invalid_request"},
		{"list bad limit", "GET", base + "?limit=101", "", nil, 422, "invalid_request"},
		{"list with key", "GET", base, "", func(r *http.Request) { r.Header.Set("Idempotency-Key", "claims-key-0001") }, 422, "invalid_request"},
		{"detail", "GET", one, "", nil, 401, "unauthorized"},
		{"detail bad id", "GET", base + "/NOT-A-UUID", "", nil, 422, "invalid_request"},
		{"detail with query", "GET", one + "?x=1", "", nil, 422, "invalid_request"},
		{"detail put", "PUT", one, `{}`, nil, 405, "method_not_allowed"},
		{"query", "POST", one + "/query", `{"expected_attempts":2}`, nil, 401, "unauthorized"},
		{"cancel", "POST", one + "/cancel", `{"expected_attempts":0}`, nil, 401, "unauthorized"},
		{"retry", "POST", one + "/retry", `{"expected_attempts":1}`, nil, 401, "unauthorized"},
		{"query without key", "POST", one + "/query", `{"expected_attempts":2}`, noKey, 422, "invalid_request"},
		{"retry bad key", "POST", one + "/retry", `{"expected_attempts":2}`, func(r *http.Request) { r.Header.Set("Idempotency-Key", "short") }, 422, "invalid_request"},
		{"missing expected_attempts", "POST", one + "/retry", `{}`, nil, 422, "invalid_request"},
		{"null expected_attempts", "POST", one + "/retry", `{"expected_attempts":null}`, nil, 422, "invalid_request"},
		{"negative expected_attempts", "POST", one + "/cancel", `{"expected_attempts":-1}`, nil, 422, "invalid_request"},
		{"unknown field", "POST", one + "/query", `{"expected_attempts":1,"operation_id":"` + claimsOther + `"}`, nil, 400, "invalid_json"},
		{"duplicate key", "POST", one + "/query", `{"expected_attempts":1,"expected_attempts":2}`, nil, 400, "invalid_json"},
		{"string number", "POST", one + "/query", `{"expected_attempts":"1"}`, nil, 400, "invalid_json"},
		{"bad id", "POST", base + "/NOT-A-UUID/cancel", `{"expected_attempts":1}`, nil, 422, "invalid_request"},
		{"post with query", "POST", one + "/cancel?x=1", `{"expected_attempts":1}`, nil, 422, "invalid_request"},
		{"cancel by get", "GET", one + "/cancel", "", nil, 405, "method_not_allowed"},
		{"list post", "POST", base, `{}`, nil, 405, "method_not_allowed"},
		{"foreign action", "POST", one + "/force", `{"expected_attempts":1}`, nil, 404, "not_found"},
	} {
		w := claimsCall(h, tc.method, tc.path, tc.body, tc.edit)
		if w.Code != tc.status {
			t.Errorf("%s: status %d (%s), want %d", tc.name, w.Code, w.Body.String(), tc.status)
			continue
		}
		var envelope httperror.Envelope
		if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil || envelope.Code != tc.code {
			t.Errorf("%s: code=%q want %q", tc.name, envelope.Code, tc.code)
		}
	}
}

// Without the River client only the reads and cancel are mounted: query and retry would have no way to enqueue their job.
func TestOperationRoutesQueryRetryNeedJobClient(t *testing.T) {
	h := operationsHandler(t, false)
	one := "/v1/admin/stores/" + claimsStore + "/operations/" + claimsOther
	for _, tc := range []struct {
		path   string
		status int
	}{{one + "/cancel", 401}, {one + "/query", 404}, {one + "/retry", 404}} {
		if w := claimsCall(h, "POST", tc.path, `{"expected_attempts":1}`, nil); w.Code != tc.status {
			t.Errorf("%s: status %d, want %d", tc.path, w.Code, tc.status)
		}
	}
	if w := claimsCall(h, "GET", "/v1/admin/stores/"+claimsStore+"/operations", "", nil); w.Code != 401 {
		t.Errorf("list unmounted without jobs: %d", w.Code)
	}
}

func TestOperationsClassify(t *testing.T) {
	for _, code := range []string{"reconcile_first", "retry_not_supported", "already_dispatched", "operation_changed"} {
		if status, got := operationsClassify(&core.OperationRefusal{Code: code}); status != http.StatusConflict || got != code {
			t.Errorf("%s -> %d %s", code, status, got)
		}
	}
}
