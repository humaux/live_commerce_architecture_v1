package meta

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
)

// NewHandler only acknowledges after commit returns. A production commit must
// atomically persist every receipt, job and quarantine record before nil.
func NewHandler(v *Verifier, commit func(context.Context, Batch) error) (http.Handler, error) {
	if !v.valid() || commit == nil {
		return nil, ErrConfig
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		switch r.Method {
		case http.MethodGet:
			challenge(w, r, v)
		case http.MethodPost:
			receive(w, r, v, commit)
		default:
			w.Header().Set("Allow", "GET, POST")
			writeCode(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED")
		}
	}), nil
}

func writeCode(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, code)
}

func challenge(w http.ResponseWriter, r *http.Request, v *Verifier) {
	q, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil || len(q) != 3 || len(q["hub.mode"]) != 1 || len(q["hub.verify_token"]) != 1 || len(q["hub.challenge"]) != 1 || q.Get("hub.mode") != "subscribe" || !validChallenge(q.Get("hub.challenge")) {
		writeCode(w, http.StatusBadRequest, "BAD_CHALLENGE")
		return
	}
	got, want := sha256.Sum256([]byte(q.Get("hub.verify_token"))), sha256.Sum256([]byte(v.verifyToken))
	if subtle.ConstantTimeCompare(got[:], want[:]) != 1 {
		writeCode(w, http.StatusForbidden, "BAD_VERIFY_TOKEN")
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, q.Get("hub.challenge"))
}

func validChallenge(s string) bool {
	if len(s) < 1 || len(s) > 200 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '.' || c == '_' || c == '-') {
			return false
		}
	}
	return true
}

func receive(w http.ResponseWriter, r *http.Request, v *Verifier, commit func(context.Context, Batch) error) {
	if r.URL.RawQuery != "" || r.URL.ForceQuery {
		writeCode(w, http.StatusBadRequest, "BAD_QUERY")
		return
	}
	if !jsonContentType(r.Header.Values("Content-Type")) || !identityEncoding(r.Header.Values("Content-Encoding")) {
		writeCode(w, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE")
		return
	}
	signatures := r.Header.Values("X-Hub-Signature-256")
	if len(signatures) != 1 {
		writeCode(w, http.StatusForbidden, "BAD_SIGNATURE")
		return
	}
	if r.Body == nil {
		writeCode(w, http.StatusBadRequest, "BAD_BODY")
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, maxBody+1))
	if err != nil {
		writeCode(w, http.StatusBadRequest, "BAD_BODY")
		return
	}
	if len(raw) > maxBody {
		writeCode(w, http.StatusRequestEntityTooLarge, "BODY_TOO_LARGE")
		return
	}
	batch, err := v.Verify(raw, signatures[0])
	if err != nil {
		switch {
		case errors.Is(err, ErrSignature):
			writeCode(w, http.StatusForbidden, "BAD_SIGNATURE")
		case errors.Is(err, ErrTooLarge):
			writeCode(w, http.StatusRequestEntityTooLarge, "BATCH_TOO_LARGE")
		default:
			writeCode(w, http.StatusBadRequest, "BAD_JSON")
		}
		return
	}
	if r.Context().Err() != nil || commit(r.Context(), batch) != nil || r.Context().Err() != nil {
		writeCode(w, http.StatusServiceUnavailable, "COMMIT_UNAVAILABLE")
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "EVENT_RECEIVED")
}

func jsonContentType(values []string) bool {
	if len(values) != 1 {
		return false
	}
	media, params, err := mime.ParseMediaType(values[0])
	if err != nil || media != "application/json" || len(params) > 1 {
		return false
	}
	if len(params) == 1 {
		charset, present := params["charset"]
		if !present || !strings.EqualFold(charset, "utf-8") {
			return false
		}
	}
	return true
}

func identityEncoding(values []string) bool {
	return len(values) == 0 || (len(values) == 1 && strings.EqualFold(strings.TrimSpace(values[0]), "identity"))
}
