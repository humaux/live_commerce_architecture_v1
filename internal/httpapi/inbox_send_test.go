// Purpose: DB-free checks of the manual-send HTTP adapter (live-console-v1 §11 A4/A5/A6/A12): the whole router builds with the send side
// enabled (a ServeMux ambiguity panics here, not at first use), the transport guards run before any database, the error classifier maps
// planner deny codes verbatim, and every code a send can return survives the shared httperror table (ruling 15: unknown codes would be
// rewritten to "internal").
// Depends on: internal/httpapi (registerInboxSendRoutes, inboxSendClassify), internal/inbox (send side), internal/httperror.
// Used by: go test ./internal/httpapi.
// Status: MOCK (no database).

package httpapi

import (
	"bytes"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"livecommerce/internal/inbox"
	"livecommerce/internal/integrations/meta/pagetoken"
	"livecommerce/internal/msgtemplates"
)

const (
	sendStoreID = "11111111-1111-4111-8111-111111111111"
	sendSession = "22222222-2222-4222-8222-222222222222"
	sendOffer   = "33333333-3333-4333-8333-333333333333"
	sendConv    = "44444444-4444-4444-8444-444444444444"
)

// sendService builds an inbox service with the send side enabled (nil pool river client: insert-only, never used here).
func sendService(t *testing.T) *inbox.Service {
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
		t.Fatal(err)
	}
	svc, err := inbox.NewService(kr)
	if err != nil {
		t.Fatal(err)
	}
	priv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	seal, err := pagetoken.LoadSealKeys(func(name string) string {
		switch name {
		case "COMMERCE_META_PAGE_HPKE_PUBLIC_KEYS_JSON":
			return `{"keys":[{"id":"p1","public_key_base64":"` + base64.StdEncoding.EncodeToString(priv.PublicKey().Bytes()) + `"}]}`
		case "COMMERCE_META_PAGE_HPKE_ACTIVE_KEY_ID":
			return "p1"
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	jobs, err := river.NewClient[pgx.Tx](riverpgxv5.New(nil), &river.Config{Schema: "river"})
	if err != nil {
		t.Fatal(err)
	}
	if svc.SendEnabled() {
		t.Fatal("send enabled before EnableSend")
	}
	svc.EnableSend(seal, jobs, msgtemplates.NewService())
	if !svc.SendEnabled() {
		t.Fatal("send not enabled after EnableSend")
	}
	return svc
}

func TestInboxSendRoutesMountOnlyWhenEnabled(t *testing.T) {
	const a12 = "/v1/admin/stores/" + sendStoreID + "/inbox/conversations/" + sendConv + "/messages"
	const a5 = "/v1/admin/stores/" + sendStoreID + "/live-sessions/" + sendSession + "/comments/1111_2222/public-reply"
	const a4 = "/v1/admin/stores/" + sendStoreID + "/live-sessions/" + sendSession + "/comments/1111_2222/private-reply"
	const a6 = "/v1/admin/stores/" + sendStoreID + "/live-sessions/" + sendSession + "/claims/offers/" + sendOffer + "/recommend"
	post := func(h http.Handler, path, body string, headers map[string]string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		res := httptest.NewRecorder()
		h.ServeHTTP(res, req)
		return res
	}
	// Reads-only inbox (no send side): the POST routes are not mounted (a methodless fallback answers 405, never a send).
	off := NewHandler(nil, Options{Inbox: func() *inbox.Service { s := sendService(t); return rebuildWithoutSend(t, s) }()})
	if res := post(off, a12, `{}`, map[string]string{"Idempotency-Key": "key-12345678"}); res.Code != http.StatusMethodNotAllowed {
		t.Fatalf("A12 without send side: %d %s", res.Code, res.Body.String())
	}

	h := NewHandler(nil, Options{Inbox: sendService(t), CommentStream: testCommentStream(t)})
	// No bearer: 401 after the transport guards (so the routes ARE mounted and reach scopedAs).
	for name, path := range map[string]string{"A12": a12, "A4": a4, "A5": a5, "A6": a6} {
		body := map[string]string{"A12": `{"text":"hi","expected_generation":0}`, "A4": `{"text":"hi"}`, "A5": `{"text":"hi"}`, "A6": `{"expected_version":1}`}[name]
		if res := post(h, path, body, map[string]string{"Idempotency-Key": "key-12345678"}); res.Code != http.StatusUnauthorized {
			t.Fatalf("%s mounted? status=%d body=%s", name, res.Code, res.Body.String())
		}
	}
	// Transport guards run before any database: missing / duplicate / short Idempotency-Key, wrong media, strict bodies.
	if res := post(h, a12, `{"text":"hi","expected_generation":0}`, nil); res.Code != http.StatusUnprocessableEntity {
		t.Fatalf("A12 without Idempotency-Key: %d", res.Code)
	}
	if res := post(h, a12, `{"text":"hi","expected_generation":0}`, map[string]string{"Idempotency-Key": "short"}); res.Code != http.StatusUnprocessableEntity {
		t.Fatalf("A12 short key: %d", res.Code)
	}
	key := map[string]string{"Idempotency-Key": "key-12345678", "Authorization": "Bearer t"}
	for name, body := range map[string]string{
		"A12 both text and template": `{"text":"hi","template_id":"x/v1","expected_generation":0}`,
		"A12 neither text nor tmpl":  `{"expected_generation":0}`,
		"A12 missing generation":     `{"text":"hi"}`,
		"A12 null text":              `{"text":null,"expected_generation":0}`,
	} {
		if res := post(h, a12, body, key); res.Code != http.StatusUnprocessableEntity && res.Code != http.StatusBadRequest {
			t.Fatalf("%s: status %d", name, res.Code)
		}
	}
	for name, body := range map[string]string{"A12 unknown key": `{"text":"hi","expected_generation":0,"tenant_id":"x"}`, "A12 duplicate key": `{"text":"a","text":"b","expected_generation":0}`} {
		if res := post(h, a12, body, key); res.Code != http.StatusBadRequest {
			t.Fatalf("%s: status %d", name, res.Code)
		}
	}
	if res := post(h, a6, `{"post_comment":true}`, key); res.Code != http.StatusUnprocessableEntity {
		t.Fatalf("A6 without expected_version: %d", res.Code)
	}
}

// rebuildWithoutSend returns a service of the same keyring with the send side off (the read-only configuration).
func rebuildWithoutSend(t *testing.T, _ *inbox.Service) *inbox.Service {
	t.Helper()
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x42}, 32))
	kr, err := inbox.LoadKeyring(func(name string) string {
		if name == "COMMERCE_META_PAYLOAD_ACTIVE_KEY_ID" {
			return "k1"
		}
		if name == "COMMERCE_META_PAYLOAD_KEYS_JSON" {
			return `{"keys":[{"id":"k1","key_base64":"` + key + `"}]}`
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	svc, err := inbox.NewService(kr)
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

func TestInboxSendClassify(t *testing.T) {
	pg := func(code, msg string) error { return &pgconn.PgError{Code: code, Message: msg} }
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{pg("PT409", "window_closed"), 409, "window_closed"},
		{pg("PT409", "auto_pending_confirm"), 409, "auto_pending_confirm"},
		{pg("PT409", "used"), 409, "used"},
		{pg("PT409", "duplicate_recent"), 409, "duplicate_recent"},
		{pg("PT409", "some driver text with a PSID 123"), 409, "conflict"},
		{pg("PT422", "ig_live_unsupported"), 422, "ig_live_unsupported"},
		{pg("PT422", "anything else"), 422, "invalid_request"},
		{pg("PT429", "rate_limited"), 429, "rate_limited"},
		{pg("PT403", "forbidden"), 403, "forbidden"},
		{pg("PT404", "not_found"), 404, "not_found"},
		{&inbox.SendError{Status: 409, Code: "comment_facts_unavailable"}, 409, "comment_facts_unavailable"},
		{&inbox.SendError{Status: 422, Code: "invalid_text", Max: 2000}, 422, "invalid_text"},
		{inbox.ErrSendUnavailable, 503, "unavailable"},
		{errors.New("driver exploded with secret text"), 503, "unavailable"},
	} {
		status, code := inboxSendClassify(tc.err)
		if status != tc.status || code != tc.code {
			t.Fatalf("classify(%v) = %d %s, want %d %s", tc.err, status, code, tc.status, tc.code)
		}
	}
}

// Every code a send can answer must be in the shared httperror table, else the merchant sees "internal" (ruling 15).
func TestSendCodesSurviveHTTPError(t *testing.T) {
	codes := []string{"comment_unknown", "page_comment", "reply_comment_unsupported", "comment_facts_unavailable", "invalid_text",
		"public_reply_forbidden_content", "conversation_gone", "rate_limited", "forbidden", "not_found", "unavailable", "conflict"}
	for code := range sendDenyCodes {
		codes = append(codes, code)
	}
	for _, code := range codes {
		res := httptest.NewRecorder()
		respondError(res, http.StatusConflict, code)
		var env struct {
			Code string `json:"code"`
		}
		if err := json.Unmarshal(res.Body.Bytes(), &env); err != nil || env.Code != code {
			t.Fatalf("code %q rewritten to %q (%v)", code, env.Code, err)
		}
	}
}

func TestSendErrorDetailsReachTheEnvelope(t *testing.T) {
	res := httptest.NewRecorder()
	se := &inbox.SendError{Status: 422, Code: "invalid_text", Max: 1000}
	respondErrorDetails(res, se, 422, "invalid_text")
	var env struct {
		Details map[string]any `json:"details"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &env); err != nil || env.Details["max"] != float64(1000) {
		t.Fatalf("details lost: %s", res.Body.String())
	}
}
