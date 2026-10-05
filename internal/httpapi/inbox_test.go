package httpapi

import (
	"bytes"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"livecommerce/internal/command"
	"livecommerce/internal/inbox"
)

const testUUID = "11111111-2222-3333-4444-555555555555"

func TestEncodeDecodeInboxCursor(t *testing.T) {
	at := time.Date(2026, 10, 5, 12, 0, 0, 123456789, time.UTC)
	c := encodeInboxCursor(at, testUUID)
	gotAt, gotID, err := decodeInboxCursor(c)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !gotAt.Equal(at) || gotID != testUUID {
		t.Fatalf("round trip mismatch: %v %q", gotAt, gotID)
	}
	if _, _, err := decodeInboxCursor("not-base64!!"); err == nil {
		t.Fatal("malformed cursor: want error")
	}
	if _, _, err := decodeInboxCursor(encodeInboxCursor(at, "not-a-uuid")); err == nil {
		t.Fatal("cursor with bad id: want error")
	}
}

func TestParseInboxListRequest(t *testing.T) {
	req, err := parseInboxListRequest(&url.URL{RawQuery: "filter=instagram&limit=10"})
	if err != nil || req.Filter != "instagram" || req.Limit != 10 || req.LastAt != nil {
		t.Fatalf("explicit filter/limit: %+v err=%v", req, err)
	}
	req, err = parseInboxListRequest(&url.URL{})
	if err != nil || req.Filter != "all" || req.Limit != 50 {
		t.Fatalf("defaults: %+v err=%v", req, err)
	}
	if _, err = parseInboxListRequest(&url.URL{ForceQuery: true}); err == nil {
		t.Fatal("force query: want error")
	}

	if _, err = parseInboxListRequest(&url.URL{RawQuery: "filter=bogus"}); err == nil {
		t.Fatal("invalid filter: want error")
	} else if qe, ok := err.(*inboxQueryError); !ok || qe.status != http.StatusBadRequest || qe.code != "invalid_filter" {
		t.Fatalf("invalid filter: want 400 invalid_filter, got %v", err)
	}
	for _, q := range []string{"limit=0", "limit=51", "limit=x", "session_id=nope", "cursor=!!", "unknown=1"} {
		if _, err = parseInboxListRequest(&url.URL{RawQuery: q}); err == nil {
			t.Fatalf("%q: want error", q)
		}
	}

	if _, err = parseInboxListRequest(&url.URL{RawQuery: "session_id=" + testUUID}); err != nil {
		t.Fatalf("valid session_id: %v", err)
	}
	at := time.Date(2026, 10, 5, 1, 2, 3, 0, time.UTC)
	req, err = parseInboxListRequest(&url.URL{RawQuery: "cursor=" + encodeInboxCursor(at, testUUID)})
	if err != nil || req.LastAt == nil || req.CursorID == nil || !req.LastAt.Equal(at) || *req.CursorID != testUUID {
		t.Fatalf("cursor decode: %+v err=%v", req, err)
	}
}

func TestParseInboxThreadQuery(t *testing.T) {
	before, limit, err := parseInboxThreadQuery(&url.URL{RawQuery: "before_seq=10&limit=20"})
	if err != nil || before == nil || *before != 10 || limit != 20 {
		t.Fatalf("explicit: %v %d err=%v", before, limit, err)
	}
	before, limit, err = parseInboxThreadQuery(&url.URL{})
	if err != nil || before != nil || limit != 50 {
		t.Fatalf("defaults: %v %d err=%v", before, limit, err)
	}
	for _, q := range []string{"before_seq=0", "before_seq=-1", "limit=0", "limit=51", "before_seq=x", "extra=1"} {
		if _, _, err = parseInboxThreadQuery(&url.URL{RawQuery: q}); err == nil {
			t.Fatalf("%q: want error", q)
		}
	}
}

func TestParseInboxBuyerPanel(t *testing.T) {
	cid, bid, err := parseInboxBuyerPanel(&url.URL{RawQuery: "conversation_id=" + testUUID})
	if err != nil || cid != testUUID || bid != "" {
		t.Fatalf("conversation_id: %q %q err=%v", cid, bid, err)
	}
	cid, bid, err = parseInboxBuyerPanel(&url.URL{RawQuery: "bundle_id=" + testUUID})
	if err != nil || cid != "" || bid != testUUID {
		t.Fatalf("bundle_id: %q %q err=%v", cid, bid, err)
	}
	for _, q := range []string{"", "conversation_id=" + testUUID + "&bundle_id=" + testUUID, "conversation_id=nope", "x=1"} {
		if _, _, err = parseInboxBuyerPanel(&url.URL{RawQuery: q}); err == nil {
			t.Fatalf("%q: want error", q)
		}
	}
}

func TestInboxClassify(t *testing.T) {
	cases := []struct {
		code, msg string
		status    int
		body      string
	}{
		{"PT400", "invalid_filter", http.StatusBadRequest, "invalid_filter"},
		{"PT403", "forbidden", http.StatusForbidden, "forbidden"},
		{"PT404", "not_found", http.StatusNotFound, "not_found"},
		{"PT409", "takeover_changed", http.StatusConflict, "takeover_changed"},
		{"PT409", "version_conflict", http.StatusConflict, "version_conflict"},
		{"PT422", "invalid_request", http.StatusUnprocessableEntity, "invalid_request"},
	}
	for _, c := range cases {
		status, body := inboxClassify(&pgconn.PgError{Code: c.code, Message: c.msg})
		if status != c.status || body != c.body {
			t.Errorf("%s/%s: want %d %q, got %d %q", c.code, c.msg, c.status, c.body, status, body)
		}
	}
	if status, body := inboxClassify(command.ErrNotFound); status != http.StatusNotFound || body != "not_found" {
		t.Errorf("not found: want 404 not_found, got %d %q", status, body)
	}
	if status, body := inboxClassify(errors.New("boom")); status != http.StatusInternalServerError || body != "internal" {
		t.Errorf("unknown: want 500 internal, got %d %q", status, body)
	}
}

func TestInboxCustomerLinkBody(t *testing.T) {
	body := func(t *testing.T, raw string) (*httptest.ResponseRecorder, *http.Request) {
		t.Helper()
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		return rec, req
	}

	rec, req := body(t, `{"customer_id":null,"expected_version":0}`)
	in, ok := inboxCustomerLinkBody(rec, req)
	if !ok || in.CustomerID != nil || in.ExpectedVersion != 0 {
		t.Fatalf("null unlink: %+v ok=%v", in, ok)
	}

	rec, req = body(t, `{"customer_id":"`+testUUID+`","expected_version":3}`)
	in, ok = inboxCustomerLinkBody(rec, req)
	if !ok || in.CustomerID == nil || *in.CustomerID != testUUID || in.ExpectedVersion != 3 {
		t.Fatalf("link: %+v ok=%v", in, ok)
	}

	rec, req = body(t, `{"expected_version":0}`)
	if _, ok = inboxCustomerLinkBody(rec, req); ok || rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("missing customer_id: ok=%v code=%d", ok, rec.Code)
	}
	rec, req = body(t, `{"customer_id":"nope","expected_version":0}`)
	if _, ok = inboxCustomerLinkBody(rec, req); ok || rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid uuid: ok=%v code=%d", ok, rec.Code)
	}
	rec, req = body(t, `{"customer_id":null,"expected_version":-1}`)
	if _, ok = inboxCustomerLinkBody(rec, req); ok || rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("negative version: ok=%v code=%d", ok, rec.Code)
	}
	rec, req = body(t, `{"customer_id":null,"customer_id":null,"expected_version":0}`)
	if _, ok = inboxCustomerLinkBody(rec, req); ok || rec.Code != http.StatusBadRequest {
		t.Fatalf("duplicate key: ok=%v code=%d", ok, rec.Code)
	}
	rec, req = body(t, `{"customer_id":null,"expected_version":0,"extra":1}`)
	if _, ok = inboxCustomerLinkBody(rec, req); ok || rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown field: ok=%v code=%d", ok, rec.Code)
	}
}

// TestInboxRoutesMountWhenServicePresent builds the whole router twice (the route-conflict surface of NewHandler) and
// asserts the inbox routes are absent with a nil service and present (401, not 404, without a bearer) with a non-nil one.
// No database is touched: route registration only, so a nil pool is safe here exactly like every other DB-free router test.
func TestInboxRoutesMountWhenServicePresent(t *testing.T) {
	svc := func(t *testing.T) *inbox.Service {
		t.Helper()
		key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x42}, 32))
		kr, err := inbox.LoadKeyring(func(name string) string {
			switch name {
			case "COMMERCE_META_PAYLOAD_ACTIVE_KEY_ID":
				return "k1"
			case "COMMERCE_META_PAYLOAD_KEYS_JSON":
				return `{"keys":[{"id":"k1","key_base64":"` + key + `"}]}`
			}
			return ""
		})
		if err != nil {
			t.Fatalf("LoadKeyring: %v", err)
		}
		s, err := inbox.NewService(kr)
		if err != nil {
			t.Fatalf("NewService: %v", err)
		}
		return s
	}

	t.Run("nil service unmounted", func(t *testing.T) {
		h := NewHandler(nil) // Options zero value: Inbox nil
		res := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v1/admin/stores/"+testUUID+"/inbox/conversations", nil)
		h.ServeHTTP(res, req)
		if res.Code != http.StatusNotFound {
			t.Fatalf("unmounted inbox route status=%d body=%s, want 404", res.Code, res.Body.String())
		}
	})

	t.Run("non-nil service mounted", func(t *testing.T) {
		h := NewHandler(nil, Options{Inbox: svc(t)})
		res := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v1/admin/stores/"+testUUID+"/inbox/conversations", nil)
		h.ServeHTTP(res, req)
		if res.Code != http.StatusUnauthorized {
			t.Fatalf("mounted inbox route status=%d body=%s, want 401", res.Code, res.Body.String())
		}
		if got := res.Header().Get("Cache-Control"); got != "private, no-store" {
			t.Fatalf("Cache-Control=%q, want private, no-store", got)
		}
	})
}
