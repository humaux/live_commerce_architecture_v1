// Package stripetest owns the independent MOCK Stripe Checkout HTTP service.
// It never contacts Stripe, decides payment state in PG, or stores real card data.
// Depends on: Go net/http and httptest for isolated wire behavior; no external service.
// Used by: Stripe payment worker and HTTP integration tests.
package stripetest

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Fault is consumed by the next create request. RateLimit and Validation run
// before execution; Cached500 and DropAfterExecute run after the cache write.
type Fault struct {
	RateLimit, Conflict, Validation, Cached500, DropAfterExecute bool
	Delay                                                        time.Duration
}

type cached struct {
	params string
	status int
	body   []byte
}

type session struct {
	ID                 string            `json:"id"`
	Object             string            `json:"object"`
	Status             string            `json:"status"`
	PaymentStatus      string            `json:"payment_status"`
	Livemode           bool              `json:"livemode"`
	Currency           string            `json:"currency"`
	AmountTotal        int64             `json:"amount_total"`
	AmountSubtotal     int64             `json:"amount_subtotal"`
	TotalDetails       map[string]int64  `json:"total_details"`
	ClientReferenceID  string            `json:"client_reference_id"`
	Metadata           map[string]string `json:"metadata"`
	ExpiresAt          int64             `json:"expires_at"`
	Created            int64             `json:"created"`
	Mode               string            `json:"mode"`
	PaymentMethodTypes []string          `json:"payment_method_types"`
	PaymentIntent      *paymentIntent    `json:"payment_intent"`
	URL                string            `json:"url,omitempty"`
}

type paymentIntent struct {
	ID             string `json:"id"`
	Object         string `json:"object"`
	Status         string `json:"status"`
	AmountReceived int64  `json:"amount_received"`
	Currency       string `json:"currency"`
}

// Server serves only local HTTP and rewrites api.stripe.com requests to itself.
// Each instance has an independent idempotency cache and session collection.
type Server struct {
	mu               sync.Mutex
	http             *httptest.Server
	account          string
	next             int
	sessions         map[string]*session
	cache            map[string]cached
	inflight         map[string]bool
	createKeys       []string
	fault            Fault
	requireKey       bool
	keyDigest        [32]byte
	accepted, denied int
}

// New starts a local-only Stripe fake. The account ID is deliberately synthetic.
func New(accountID string) *Server {
	s := &Server{account: accountID, sessions: make(map[string]*session), cache: make(map[string]cached), inflight: make(map[string]bool)}
	s.http = httptest.NewServer(http.HandlerFunc(s.serveHTTP))
	return s
}

func (s *Server) Close()                { s.http.Close() }
func (s *Server) URL() string           { return s.http.URL }
func (s *Server) Handler() http.Handler { return http.HandlerFunc(s.serveHTTP) }

// Transport intercepts the documented Stripe host and never forwards any
// other request. Tests may pass it to stripe.NewWithMockTransport.
func (s *Server) Transport() http.RoundTripper {
	return &transport{server: s.http.URL, client: s.http.Client()}
}

type transport struct {
	server string
	client *http.Client
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Scheme != "https" || req.URL.Host != "api.stripe.com" {
		return nil, fmt.Errorf("stripetest: refused non-Stripe destination")
	}
	copy := req.Clone(req.Context())
	copy.URL.Scheme = "http"
	copy.URL.Host = strings.TrimPrefix(t.server, "http://")
	copy.Host = copy.URL.Host
	return t.client.Transport.RoundTrip(copy)
}

func (s *Server) SetNextFault(f Fault) { s.mu.Lock(); s.fault = f; s.mu.Unlock() }

// RequireAPIKey enables exact Bearer admission for all fake endpoints. Only a
// digest is retained; neither request counts nor failures expose the key.
// https://docs.stripe.com/api/authentication (retrieved 2026-09-28).
func (s *Server) RequireAPIKey(key string) error {
	if key == "" || strings.ContainsAny(key, " \t\r\n") {
		return fmt.Errorf("stripetest: invalid API key fixture")
	}
	s.mu.Lock()
	s.keyDigest = sha256.Sum256([]byte(key))
	s.requireKey = true
	s.mu.Unlock()
	return nil
}

type RequestCounts struct{ Accepted, Denied int }

func (s *Server) Counts() RequestCounts {
	s.mu.Lock()
	defer s.mu.Unlock()
	return RequestCounts{Accepted: s.accepted, Denied: s.denied}
}
func (s *Server) CreateKeys() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.createKeys...)
}
func (s *Server) SessionIDs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := make([]string, 0, len(s.sessions))
	for id := range s.sessions {
		ids = append(ids, id)
	}
	return ids
}

// SetState controls provider observations without relying on webhook delivery.
// It also permits expired+paid and complete+unpaid anomaly fixtures.
func (s *Server) SetState(id, status, paymentStatus string) bool {
	if status != "open" && status != "complete" && status != "expired" {
		return false
	}
	if paymentStatus != "paid" && paymentStatus != "unpaid" && paymentStatus != "no_payment_required" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v := s.sessions[id]
	if v == nil {
		return false
	}
	v.Status, v.PaymentStatus = status, paymentStatus
	if status != "open" {
		v.URL = ""
	}
	if paymentStatus == "paid" {
		v.PaymentIntent = &paymentIntent{ID: "pi_test_" + id, Object: "payment_intent", Status: "succeeded", AmountReceived: v.AmountTotal, Currency: v.Currency}
	}
	return true
}

// SignWebhook uses Stripe's t.raw-body HMAC-SHA256 v1 wire format.
// https://docs.stripe.com/webhooks/signature (retrieved 2026-09-28).
func SignWebhook(secret string, body []byte, at time.Time) string {
	t := strconv.FormatInt(at.Unix(), 10)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(t + "."))
	h.Write(body)
	return "t=" + t + ",v1=" + hex.EncodeToString(h.Sum(nil))
}

func (s *Server) serveHTTP(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		errorJSON(w, http.StatusUnauthorized, "authentication_error", "invalid_api_key")
		return
	}
	if r.URL.Path == "/v1/account" && r.Method == http.MethodGet {
		writeJSON(w, 200, map[string]any{"id": s.account, "object": "account", "livemode": false})
		return
	}
	base := "/v1/checkout/sessions"
	if r.URL.Path == base && r.Method == http.MethodPost {
		s.create(w, r)
		return
	}
	if r.URL.Path == base && r.Method == http.MethodGet {
		s.list(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, base+"/") {
		id := strings.TrimPrefix(r.URL.Path, base+"/")
		if strings.HasSuffix(id, "/expire") && r.Method == http.MethodPost {
			s.expire(w, strings.TrimSuffix(id, "/expire"))
			return
		}
		if r.Method == http.MethodGet {
			s.retrieve(w, id)
			return
		}
	}
	errorJSON(w, 404, "invalid_request_error", "resource_missing")
}

func (s *Server) authorized(r *http.Request) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.requireKey {
		s.accepted++
		return true
	}
	values := r.Header.Values("Authorization")
	if len(values) != 1 {
		s.denied++
		return false
	}
	token, ok := strings.CutPrefix(values[0], "Bearer ")
	if !ok || token == "" || strings.ContainsAny(token, " \t\r\n") {
		s.denied++
		return false
	}
	got := sha256.Sum256([]byte(token))
	if subtle.ConstantTimeCompare(got[:], s.keyDigest[:]) != 1 {
		s.denied++
		return false
	}
	s.accepted++
	return true
}

func (s *Server) create(w http.ResponseWriter, r *http.Request) {
	key := r.Header.Get("Idempotency-Key")
	if key == "" || len(key) > 255 {
		errorJSON(w, 400, "invalid_request_error", "parameter_invalid_empty")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 8192))
	if err != nil {
		errorJSON(w, 400, "invalid_request_error", "invalid_request")
		return
	}
	vals, err := url.ParseQuery(string(body))
	if err != nil {
		errorJSON(w, 400, "invalid_request_error", "invalid_request")
		return
	}
	params := vals.Encode() // Stripe compares parameters, not byte formatting.
	s.mu.Lock()
	s.createKeys = append(s.createKeys, key)
	if prior, ok := s.cache[key]; ok {
		s.mu.Unlock()
		if prior.params != params {
			errorJSON(w, 400, "idempotency_error", "idempotency_key_in_use")
			return
		}
		w.Header().Set("Idempotent-Replayed", "true")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(prior.status)
		_, _ = w.Write(prior.body)
		return
	}
	if s.inflight[key] {
		s.mu.Unlock()
		errorJSON(w, 409, "invalid_request_error", "lock_timeout")
		return
	}
	f := s.fault
	s.fault = Fault{}
	if f.RateLimit || f.Conflict || f.Validation {
		s.mu.Unlock()
		switch {
		case f.RateLimit:
			errorJSON(w, 429, "invalid_request_error", "rate_limit")
		case f.Conflict:
			errorJSON(w, 409, "invalid_request_error", "lock_timeout")
		default:
			errorJSON(w, 400, "invalid_request_error", "invalid_request")
		}
		return
	}
	s.inflight[key] = true
	s.mu.Unlock()
	defer func() { s.mu.Lock(); delete(s.inflight, key); s.mu.Unlock() }()
	if f.Delay > 0 {
		select {
		case <-time.After(f.Delay):
		case <-r.Context().Done():
			return
		}
	}
	if vals.Get("mode") != "payment" || vals.Get("payment_method_types[0]") != "card" || vals.Get("client_reference_id") == "" || vals.Get("line_items[0][price_data][currency]") == "" {
		errorJSON(w, 400, "invalid_request_error", "invalid_request")
		return // validation never enters cache
	}
	amount, err := strconv.ParseInt(vals.Get("line_items[0][price_data][unit_amount]"), 10, 64)
	if err != nil || amount <= 0 {
		errorJSON(w, 400, "invalid_request_error", "amount_too_small")
		return
	}
	expires, err := strconv.ParseInt(vals.Get("expires_at"), 10, 64)
	if err != nil || expires <= 0 {
		errorJSON(w, 400, "invalid_request_error", "invalid_expiry")
		return
	}
	s.mu.Lock()
	s.next++
	id := fmt.Sprintf("cs_test_fake_%d", s.next)
	v := &session{ID: id, Object: "checkout.session", Status: "open", PaymentStatus: "unpaid", Currency: vals.Get("line_items[0][price_data][currency]"), AmountTotal: amount, AmountSubtotal: amount, TotalDetails: map[string]int64{"amount_discount": 0, "amount_tax": 0, "amount_shipping": 0}, ClientReferenceID: vals.Get("client_reference_id"), Metadata: map[string]string{"lc_attempt": vals.Get("metadata[lc_attempt]"), "lc_order": vals.Get("metadata[lc_order]"), "lc_profile": vals.Get("metadata[lc_profile]"), "lc_v": vals.Get("metadata[lc_v]")}, ExpiresAt: expires, Created: time.Now().Unix(), Mode: "payment", PaymentMethodTypes: []string{"card"}, URL: "https://checkout.stripe.com/c/pay/" + id}
	s.sessions[id] = v
	status := 200
	var reply []byte
	if f.Cached500 {
		status = 500
		reply = []byte(`{"error":{"type":"api_error","code":"api_error"}}`)
	} else {
		reply, _ = json.Marshal(v)
	}
	s.cache[key] = cached{params: params, status: status, body: reply}
	s.mu.Unlock()
	if f.DropAfterExecute {
		hijack, ok := w.(http.Hijacker)
		if ok {
			conn, _, e := hijack.Hijack()
			if e == nil {
				_ = conn.Close()
				return
			}
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(reply)
}

func (s *Server) retrieve(w http.ResponseWriter, id string) {
	s.mu.Lock()
	v := s.sessions[id]
	var snapshot session
	if v != nil {
		snapshot = *v
	}
	s.mu.Unlock()
	if v == nil {
		errorJSON(w, 404, "invalid_request_error", "resource_missing")
		return
	}
	writeJSON(w, 200, snapshot)
}
func (s *Server) expire(w http.ResponseWriter, id string) {
	s.mu.Lock()
	v := s.sessions[id]
	if v == nil || v.Status != "open" {
		s.mu.Unlock()
		errorJSON(w, 400, "invalid_request_error", "checkout_session_not_expirable")
		return
	}
	v.Status = "expired"
	v.URL = ""
	snapshot := *v
	s.mu.Unlock()
	writeJSON(w, 200, snapshot)
}
func (s *Server) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit < 1 || limit > 100 {
		errorJSON(w, 400, "invalid_request_error", "invalid_limit")
		return
	}
	gte, _ := strconv.ParseInt(q.Get("created[gte]"), 10, 64)
	lte, _ := strconv.ParseInt(q.Get("created[lte]"), 10, 64)
	s.mu.Lock()
	data := make([]*session, 0, len(s.sessions))
	for _, v := range s.sessions {
		if v.Created >= gte && v.Created <= lte {
			copy := *v
			data = append(data, &copy)
		}
	}
	s.mu.Unlock()
	// Stable order is enough for the bounded lookup protocol.
	for i := 0; i < len(data); i++ {
		for j := i + 1; j < len(data); j++ {
			if data[i].ID < data[j].ID {
				data[i], data[j] = data[j], data[i]
			}
		}
	}
	start := 0
	if after := q.Get("starting_after"); after != "" {
		for i, v := range data {
			if v.ID == after {
				start = i + 1
				break
			}
		}
	}
	if start > len(data) {
		start = len(data)
	}
	data = data[start:]
	hasMore := len(data) > limit
	if hasMore {
		data = data[:limit]
	}
	writeJSON(w, 200, map[string]any{"object": "list", "data": data, "has_more": hasMore})
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func errorJSON(w http.ResponseWriter, status int, typ, code string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"type": typ, "code": code}})
}
