// Package httperror owns transport-safe error envelopes, never domain policy.
package httperror

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
)

type Envelope struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	RequestID string         `json:"request_id"`
	Retryable bool           `json:"retryable"`
	Details   map[string]any `json:"details"`
}

type requestKey struct{}

// Middleware assigns a server-owned correlation ID. Nested platform handlers
// reuse the context value, never an untrusted inbound X-Request-ID header.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, _ := r.Context().Value(requestKey{}).(string)
		if id == "" {
			var value [16]byte
			_, _ = rand.Read(value[:]) // Go's crypto/rand terminates on entropy failure.
			id = hex.EncodeToString(value[:])
			r = r.WithContext(context.WithValue(r.Context(), requestKey{}, id))
		}
		w.Header().Set("X-Request-ID", id)
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(&errorWriter{ResponseWriter: w}, r)
	})
}

func Write(w http.ResponseWriter, status int, code string) {
	messages := map[string]string{
		"unauthorized": "Sign-in required.", "forbidden": "Operation not permitted.",
		"not_found": "Resource not found.", "method_not_allowed": "Method not allowed.",
		"invalid_request": "Request validation failed.", "invalid_json": "Malformed JSON body.",
		"json_required": "JSON content type required.", "conflict": "Request conflicts with current state.",
		"insufficient_inventory": "Insufficient available inventory.",
		"retry_later":            "Temporarily unavailable.", "unavailable": "Temporarily unavailable.",
		"internal": "Request could not be completed.",
	}
	message, ok := messages[code]
	if !ok {
		code, message = "internal", messages["internal"]
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Envelope{Code: code, Message: message,
		RequestID: w.Header().Get("X-Request-ID"), Retryable: status == http.StatusServiceUnavailable,
		Details: map[string]any{}})
}

// ServeMux generates plain-text 404/405 responses. Translate only non-JSON
// errors at the boundary; never buffer application responses or expose text.
type errorWriter struct {
	http.ResponseWriter
	wrote, suppressed bool
}

func (w *errorWriter) WriteHeader(status int) {
	if w.wrote {
		return
	}
	w.wrote = true
	if status >= 400 && !strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") {
		w.suppressed = true
		code := "internal"
		if status == 404 {
			code = "not_found"
		}
		if status == 405 {
			code = "method_not_allowed"
		}
		w.Header().Del("Content-Length")
		Write(w.ResponseWriter, status, code)
		return
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *errorWriter) Write(p []byte) (int, error) {
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}
	if w.suppressed {
		return len(p), nil
	}
	return w.ResponseWriter.Write(p)
}

func (w *errorWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
