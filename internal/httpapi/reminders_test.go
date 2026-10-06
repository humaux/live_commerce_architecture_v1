// Purpose: DB-free checks of the W3-03B reminder HTTP adapter: the whole router builds with the reminder routes (a ServeMux ambiguity panics
// here, not at first use), the routes mount only with the send side enabled, the transport guards run before any database, and every code
// the routes can return survives the shared httperror table.
// Depends on: internal/httpapi (registerReminderRoutes via NewHandler), internal/inbox, sendService/rebuildWithoutSend (inbox_send_test.go).
// Used by: go test ./internal/httpapi.
// Status: MOCK (no database).

package httpapi

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestReminderRoutesMountAndGuards(t *testing.T) {
	const session = "/v1/admin/stores/" + sendStoreID + "/live-sessions/" + sendSession + "/reminders"
	const settings = "/v1/admin/stores/" + sendStoreID + "/live-settings/reminder"
	do := func(h http.Handler, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
		var reader io.Reader = http.NoBody // a bodiless GET must look bodiless to the transport guard
		if body != "" {
			reader = strings.NewReader(body)
		}
		req := httptest.NewRequest(method, path, reader)
		req.Header.Set("Content-Type", "application/json")
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		res := httptest.NewRecorder()
		h.ServeHTTP(res, req)
		return res
	}
	key := map[string]string{"Idempotency-Key": "key-12345678"}
	authed := map[string]string{"Idempotency-Key": "key-12345678", "Authorization": "Bearer t"}
	bearer := map[string]string{"Authorization": "Bearer t"}

	// Without the send side nothing mounts: every reminder verb answers 404 (no route), never a send.
	off := NewHandler(nil, Options{Inbox: rebuildWithoutSend(t, nil)})
	if res := do(off, http.MethodPost, session, "", key); res.Code != http.StatusNotFound {
		t.Fatalf("POST reminders without the send side: %d", res.Code)
	}
	if res := do(off, http.MethodGet, settings, "", nil); res.Code != http.StatusNotFound {
		t.Fatalf("GET settings without the send side: %d", res.Code)
	}

	h := NewHandler(nil, Options{Inbox: sendService(t)})
	// Mounted: no bearer is 401 after the transport guards.
	for name, tc := range map[string]struct{ method, path, body string }{
		"trigger":      {http.MethodPost, session, ""},
		"trigger one":  {http.MethodPost, session + "/" + sendOffer, ""},
		"report":       {http.MethodGet, session, ""},
		"get settings": {http.MethodGet, settings, ""},
		"put settings": {http.MethodPut, settings, `{"enabled":true,"delay_minutes":30,"expected_version":0}`},
	} {
		headers := key
		if tc.method != http.MethodPost { // settings and the report are keyless
			headers = nil
		}
		if res := do(h, tc.method, tc.path, tc.body, headers); res.Code != http.StatusUnauthorized {
			t.Fatalf("%s mounted? status=%d body=%s", name, res.Code, res.Body.String())
		}
	}
	// Transport guards before any database.
	if res := do(h, http.MethodPost, session, "", nil); res.Code != http.StatusUnprocessableEntity {
		t.Fatalf("trigger without Idempotency-Key: %d", res.Code)
	}
	if res := do(h, http.MethodPost, session+"?x=1", "", authed); res.Code != http.StatusUnprocessableEntity {
		t.Fatalf("trigger with a query string: %d", res.Code)
	}
	if res := do(h, http.MethodGet, session, "", key); res.Code != http.StatusUnprocessableEntity {
		t.Fatalf("report with an Idempotency-Key: %d", res.Code)
	}
	badSession := "/v1/admin/stores/" + sendStoreID + "/live-sessions/not-a-uuid/reminders"
	if res := do(h, http.MethodPost, badSession, "", authed); res.Code != http.StatusUnprocessableEntity {
		t.Fatalf("trigger with a malformed session id: %d", res.Code)
	}
	for name, body := range map[string]string{
		"missing delay":   `{"enabled":true,"expected_version":0}`,
		"missing version": `{"enabled":true,"delay_minutes":30}`,
		"null enabled":    `{"enabled":null,"delay_minutes":30,"expected_version":0}`,
		"empty":           `{}`,
	} {
		if res := do(h, http.MethodPut, settings, body, bearer); res.Code != http.StatusUnprocessableEntity && res.Code != http.StatusBadRequest {
			t.Fatalf("%s: status %d", name, res.Code)
		}
	}
	for name, body := range map[string]string{
		"unknown key":   `{"enabled":true,"delay_minutes":30,"expected_version":0,"tenant_id":"x"}`,
		"duplicate key": `{"enabled":true,"enabled":false,"delay_minutes":30,"expected_version":0}`,
	} {
		if res := do(h, http.MethodPut, settings, body, bearer); res.Code != http.StatusBadRequest {
			t.Fatalf("%s: status %d", name, res.Code)
		}
	}
	// A wrong verb stays inside the private boundary.
	if res := do(h, http.MethodDelete, settings, "", nil); res.Code != http.StatusMethodNotAllowed {
		t.Fatalf("DELETE settings: %d", res.Code)
	}
}

// The codes the reminder routes add or reuse must map to fixed transport codes and survive the httperror table (ruling 15).
func TestReminderErrorCodes(t *testing.T) {
	pg := func(code, msg string) error { return &pgconn.PgError{Code: code, Message: msg} }
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{pg("PT409", "version_conflict"), 409, "version_conflict"},
		{pg("PT403", "forbidden"), 403, "forbidden"},
		{pg("PT404", "not_found"), 404, "not_found"},
		{pg("PT422", "invalid_request"), 422, "invalid_request"},
	} {
		if status, code := inboxSendClassify(tc.err); status != tc.status || code != tc.code {
			t.Fatalf("classify(%v) = %d %s, want %d %s", tc.err, status, code, tc.status, tc.code)
		}
	}
	res := httptest.NewRecorder()
	respondError(res, http.StatusConflict, "no_storefront")
	if !strings.Contains(res.Body.String(), `"no_storefront"`) {
		t.Fatalf("no_storefront rewritten: %s", res.Body.String())
	}
}
