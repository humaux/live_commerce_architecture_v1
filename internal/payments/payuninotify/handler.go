// Purpose: HTTP handler for POST /v1/hooks/payuni/notify/{endpoint_token}: hash token, authenticate the callback, record + wake, ACK.
// Depends on: internal/integrations/psp/payuni (NewNotify, AuthenticateNotification), internal/integrations/accounts
//   (OpenPayuniNotify); the Store interface implemented by inbox.go; no env vars.
// Used by: cmd/api/payuni_notify.go (mounted in cmd/api/main.go), handler_test.go.
// Invariants: token is routing only (sha256, never tenant authority); no ACK before COMMIT; bad signature writes zero rows.
// Status: MOCK/SANDBOX (ACK semantics NOT_VERIFIED against PAYUNi docs).

// handler.go: POST /v1/hooks/payuni/notify/{endpoint_token}. The endpoint_token is a per-connection
// secret that only selects the connection; it is hashed (sha256) before any DB call and is never
// tenant authority. A notification is only a trigger to query: the handler verifies the callback
// signature with the connection's decrypted HashKey/HashIV, then records one idempotent receipt in
// the same transaction that wakes the attempt's existing payment_query_v1 job. Nothing is ACKed
// before COMMIT, and no money fact is ever written here.

package payuninotify

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"regexp"
	"strings"
	"time"

	"livecommerce/internal/integrations/accounts"
	"livecommerce/internal/integrations/psp/payuni"
)

const (
	routePrefix   = "/v1/hooks/payuni/notify/"
	maxBody       = 8 << 10 // brief body bound (read limit + 1 byte detects overflow)
	maxInFlight   = 32      // non-blocking admission semaphore
	requestBudget = 5 * time.Second
)

// tokenPattern: base64url of 32 random bytes without padding is exactly 43 characters.
var tokenPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)

// tradeStatusPattern mirrors payments.payuni_notify_receipts.trade_status; AuthenticateNotification
// deliberately does not validate TradeStatus, so the handler rejects a malformed value with a 4xx
// rather than letting the definer's 22023 turn it into a retry-forever 503.
var tradeStatusPattern = regexp.MustCompile(`^[0-9]{1,4}$`)

type handler struct {
	in     *Inbox
	sem    chan struct{}
	budget time.Duration // per-request deadline for body read plus lookup/verify/record (requestBudget)
}

// NewHandler is the only way to obtain a notify entry point; no exported method accepts a callback
// that has not passed the connection's signature check.
func NewHandler(inbox *Inbox) (http.Handler, error) {
	// Only PROVIDER_MOCK|SANDBOX are admitted; LIVE is never admitted for PAYUNi notify.
	if inbox == nil || inbox.store == nil || inbox.keys == nil || inbox.now == nil ||
		!validProfile(inbox.profile) {
		return nil, ErrConfig
	}
	return &handler{in: inbox, sem: make(chan struct{}, maxInFlight), budget: requestBudget}, nil
}

func reply(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}

func fail(w http.ResponseWriter, status int, code string) {
	reply(w, status, `{"error":"`+code+`"}`)
}

// ack is the officially unverified success response (EVIDENCE_GAP): HTTP 200, empty body.
func ack(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
}

// logReject records only a fixed code and a short hash of the endpoint token: never the token,
// body, header or any key material.
func logReject(tokenTag, code string) {
	slog.Warn("payuni_notify_rejected", "code", code, "token_hash", tokenTag)
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		fail(w, http.StatusMethodNotAllowed, "method_not_allowed")
		return
	}
	// Exact literal path; RawPath is set when the client used a non-canonical escape (%61 etc.).
	token, ok := strings.CutPrefix(r.URL.Path, routePrefix)
	if !ok || r.URL.RawPath != "" || !tokenPattern.MatchString(token) {
		fail(w, http.StatusNotFound, "not_found")
		return
	}
	if r.URL.RawQuery != "" || r.URL.ForceQuery {
		fail(w, http.StatusBadRequest, "invalid_request")
		return
	}
	// The token selects the connection but is not authority; only its hash leaves this handler.
	tokenHash := sha256.Sum256([]byte(token))
	tokenTag := hex.EncodeToString(tokenHash[:8])
	if !formContentType(r.Header.Values("Content-Type")) || !identityEncoding(r.Header.Values("Content-Encoding")) {
		fail(w, http.StatusUnsupportedMediaType, "unsupported_media_type")
		return
	}
	if r.Body == nil {
		fail(w, http.StatusBadRequest, "invalid_request")
		return
	}
	// The body is read BEFORE an admission slot is taken and under its own read deadline, so an
	// unauthenticated client trickling a body (any well-shaped token path passes the checks above)
	// cannot hold slots: the slot only covers lookup, verification and record. The deadline also
	// bounds the read itself (io.ReadAll ignores ctx); ErrNotSupported (test recorders) is ignored.
	_ = http.NewResponseController(w).SetReadDeadline(time.Now().Add(h.budget))
	raw, err := io.ReadAll(io.LimitReader(r.Body, maxBody+1))
	if err != nil {
		fail(w, http.StatusBadRequest, "invalid_request")
		return
	}
	if len(raw) > maxBody {
		fail(w, http.StatusRequestEntityTooLarge, "payload_too_large")
		return
	}
	select {
	case h.sem <- struct{}{}:
		defer func() { <-h.sem }()
	default:
		w.Header().Set("Retry-After", "5")
		fail(w, http.StatusServiceUnavailable, "busy")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.budget)
	defer cancel()

	m, found, err := h.in.store.material(ctx, tokenHash[:])
	if err != nil {
		// Why 503: the lookup failed, not the delivery; PAYUNi retries with the same payload.
		logReject(tokenTag, "unavailable")
		fail(w, http.StatusServiceUnavailable, "unavailable")
		return
	}
	// An unknown/disabled endpoint is invisible (404); a wrong-profile endpoint is too.
	if !found || m.Profile != h.in.profile {
		fail(w, http.StatusNotFound, "not_found")
		return
	}
	scope := func(version int64) accounts.PayuniCredentialScope {
		return accounts.PayuniCredentialScope{TenantID: m.TenantID, StoreID: m.StoreID,
			ConnectionID: m.ConnectionID, Environment: m.Environment, AccountID: m.AccountID,
			CredentialVersion: version}
	}
	creds, err := h.in.keys.OpenPayuniNotify(scope(m.CurVersion), m.CurKeyID, m.CurNonce, m.CurCiphertext)
	usedPrev := false
	if err != nil {
		// Rotation grace: retry the previous credential version before failing the delivery.
		if m.PrevVersion == nil || m.PrevKeyID == nil {
			logReject(tokenTag, "signing_unavailable")
			fail(w, http.StatusServiceUnavailable, "signing_unavailable")
			return
		}
		usedPrev = true
		creds, err = h.in.keys.OpenPayuniNotify(scope(*m.PrevVersion), *m.PrevKeyID, m.PrevNonce, m.PrevCiphertext)
		if err != nil {
			logReject(tokenTag, "signing_unavailable")
			fail(w, http.StatusServiceUnavailable, "signing_unavailable")
			return
		}
	}
	// authenticateNotification checks outer MerID == connection MerID == decrypted inner MerID, the
	// HashInfo over HashKey/HashIV, and the status pair. A crypto/signature failure is 400 with no rows.
	client, err := payuni.NewNotify(payuni.Config{Environment: m.Environment, MerchantID: m.AccountID,
		HashKey: creds.HashKey, HashIV: creds.HashIV})
	if err != nil {
		logReject(tokenTag, "signing_unavailable")
		fail(w, http.StatusServiceUnavailable, "signing_unavailable")
		return
	}
	auth, err := client.AuthenticateNotification(raw)
	// Rotation grace: the merchant may still sign with the previous HashKey until the provider-side
	// key swap completes, so a callback that fails the current signature is retried with the previous
	// credential version before it is refused (HashInfo over current OR previous credential).
	if err != nil && !usedPrev && m.PrevVersion != nil && m.PrevKeyID != nil {
		if prevCreds, prevErr := h.in.keys.OpenPayuniNotify(scope(*m.PrevVersion), *m.PrevKeyID, m.PrevNonce, m.PrevCiphertext); prevErr == nil {
			if prevClient, cErr := payuni.NewNotify(payuni.Config{Environment: m.Environment, MerchantID: m.AccountID,
				HashKey: prevCreds.HashKey, HashIV: prevCreds.HashIV}); cErr == nil {
				auth, err = prevClient.AuthenticateNotification(raw)
			}
		}
	}
	if err != nil {
		logReject(tokenTag, "invalid_signature")
		fail(w, http.StatusBadRequest, "invalid_signature")
		return
	}
	if !tradeStatusPattern.MatchString(auth.TradeStatus) {
		logReject(tokenTag, "invalid_request")
		fail(w, http.StatusBadRequest, "invalid_request")
		return
	}
	payloadSHA := sha256.Sum256(raw)
	if _, err := h.in.store.record(ctx, tokenHash[:], payloadSHA[:], auth); err != nil {
		// Why 503: DB error; no receipt was committed, so PAYUNi's retry is safe and deduplicated
		// per (connection, payload_sha256).
		logReject(tokenTag, "unavailable")
		fail(w, http.StatusServiceUnavailable, "unavailable")
		return
	}
	ack(w)
}

func formContentType(values []string) bool {
	if len(values) != 1 {
		return false
	}
	media, params, err := mime.ParseMediaType(values[0])
	if err != nil || media != "application/x-www-form-urlencoded" || len(params) > 1 {
		return false
	}
	if len(params) == 1 {
		charset, present := params["charset"]
		return present && strings.EqualFold(charset, "utf-8")
	}
	return true
}

func identityEncoding(values []string) bool {
	return len(values) == 0 || (len(values) == 1 && strings.EqualFold(strings.TrimSpace(values[0]), "identity"))
}
