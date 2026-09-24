package httperror

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEnvelopeAndNestedCorrelation(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 2; i++ {
		h := Middleware(Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { Write(w, 503, "retry_later") })))
		r := httptest.NewRequest("GET", "/", nil)
		r.Header.Set("X-Request-ID", "attacker-controlled")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		var e Envelope
		if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
			t.Fatal(err)
		}
		if e.Code != "retry_later" || !e.Retryable || e.Message == "" || e.Details == nil || len(e.RequestID) != 32 || seen[e.RequestID] || e.RequestID != w.Header().Get("X-Request-ID") {
			t.Fatalf("bad envelope %+v", e)
		}
		seen[e.RequestID] = true
		if w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatal("missing headers")
		}
	}
}

func TestNonRetryableFailurePreservesSafeEnvelope(t *testing.T) {
	for _, status := range []int{401, 404, 429, 503} {
		w := httptest.NewRecorder()
		Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			WriteNonRetryable(w, status, "unavailable")
		})).ServeHTTP(w, httptest.NewRequest("POST", "/session", nil))
		var e Envelope
		if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
			t.Fatal(err)
		}
		if w.Code != status || e.Retryable || e.Code != "unavailable" || e.Message != "Temporarily unavailable." || e.Details == nil || e.RequestID != w.Header().Get("X-Request-ID") || len(e.RequestID) != 32 {
			t.Fatal("nonretryable envelope changed or implied retry")
		}
		if w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatal("missing safe response headers")
		}
	}
}

func TestRouterFailuresRemainJSON(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /only", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })
	h := Middleware(mux)
	for _, tc := range []struct {
		method, path, code string
		status             int
	}{{"GET", "/unknown", "not_found", 404}, {"POST", "/only", "method_not_allowed", 405}} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		var e Envelope
		if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
			t.Fatal(err)
		}
		if w.Code != tc.status || e.Code != tc.code || e.Retryable || e.RequestID == "" {
			t.Fatalf("bad router response %d %+v", w.Code, e)
		}
		if tc.status == 405 && w.Header().Get("Allow") == "" {
			t.Fatal("lost Allow")
		}
	}
}

func TestRateLimitCodeUsesFixedPublicMessage(t *testing.T) {
	w := httptest.NewRecorder()
	Write(w, http.StatusTooManyRequests, "rate_limited")
	var envelope Envelope
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if w.Code != 429 || envelope.Code != "rate_limited" || envelope.Message != "Too many requests." || !envelope.Retryable {
		t.Fatalf("unsafe rate response: %+v", envelope)
	}
}
