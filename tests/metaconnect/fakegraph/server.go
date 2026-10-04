// Package fakegraph is the MOCK Meta Graph of the merchant Page-connect gates (unit meta-connect): just the five calls
// internal/metaconnect makes (code exchange + long-lived extend, /me/permissions, /me/accounts, POST/DELETE subscribed_apps),
// served on a loopback httptest server. It owns every programmed user, Page and fault and the request log; it never validates
// more of Graph than those calls and never proves anything about the real Meta (MOCK tier, docs/delivery/GATES.md).
package fakegraph

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
)

// Page is one managed Page of a user. Token is the Page access token /me/accounts returns for it.
type Page struct {
	ID, Name, Token string
	Tasks           []string
	IGID, IGName    string // empty = no Instagram business account
}

// User is what a code resolves to: the granted permissions and the managed Pages.
type User struct {
	Permissions []string
	Pages       []Page
}

// Req is one captured request (token values are NOT stored; HasQueryToken records a token in the URL).
type Req struct {
	Method, Path    string
	Bearer          bool // Authorization: Bearer present
	BodyHasToken    bool
	HasQueryToken   bool
	HasClientSecret bool
}

// Server is the fake Graph.
type Server struct {
	srv *httptest.Server
	mu  sync.Mutex

	app           struct{ id, secret, redirect string }
	codes         map[string]string // one-shot code -> short token
	short, long   map[string]string // short token -> long token ; long token -> user key
	users         map[string]*User  // user key -> user
	subscribed    map[string]bool   // page id -> subscribed
	pageOf        map[string]string // page token -> page id
	failSubscribe bool
	failAccounts  bool
	reqs          []Req
	seq           int
}

// New starts the server; Close stops it.
func New() *Server {
	s := &Server{codes: map[string]string{}, short: map[string]string{}, long: map[string]string{}, users: map[string]*User{},
		subscribed: map[string]bool{}, pageOf: map[string]string{}}
	s.srv = httptest.NewServer(http.HandlerFunc(s.serve))
	return s
}

// URL is the loopback base the code under test dials.
func (s *Server) URL() string { return s.srv.URL }

// Close stops the server.
func (s *Server) Close() { s.srv.CloseClientConnections(); s.srv.Close() }

// ExpectApp pins the client_id / secret / redirect_uri the exchange must present.
func (s *Server) ExpectApp(id, secret, redirect string) {
	s.app.id, s.app.secret, s.app.redirect = id, secret, redirect
}

// AddCode registers a one-shot OAuth code resolving to a user. The short/long token values are generated (visible via the
// returned pair so a test can scan for leaks).
func (s *Server) AddCode(code string, u User) (shortTok, longTok string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	key := "user" + itoa(s.seq)
	shortTok, longTok = "SENTINEL-EAAS-"+key+"-short", "SENTINEL-EAAL-"+key+"-long"
	s.codes[code], s.short[shortTok], s.long[longTok] = shortTok, longTok, key
	s.users[key] = &u
	for _, p := range u.Pages {
		s.pageOf[p.Token] = p.ID
	}
	return shortTok, longTok
}

// FailSubscribe / FailAccounts program a Graph error on the next calls until cleared.
func (s *Server) FailSubscribe(v bool) { s.mu.Lock(); s.failSubscribe = v; s.mu.Unlock() }
func (s *Server) FailAccounts(v bool)  { s.mu.Lock(); s.failAccounts = v; s.mu.Unlock() }

// Subscribed reports whether the Page is currently subscribed.
func (s *Server) Subscribed(page string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.subscribed[page]
}

// Requests is a copy of the request log.
func (s *Server) Requests() []Req {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Req(nil), s.reqs...)
}

// Count counts logged requests by method and path suffix.
func (s *Server) Count(method, suffix string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, r := range s.reqs {
		if r.Method == method && strings.HasSuffix(r.Path, suffix) {
			n++
		}
	}
	return n
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}

func graphError(w http.ResponseWriter, status, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": msg, "type": "OAuthException", "code": code}})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	var body map[string]any
	_ = json.Unmarshal(raw, &body)
	q := r.URL.Query()
	bearer := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	hasBearer := strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ")
	path := strings.TrimPrefix(r.URL.Path, "/")
	if i := strings.Index(path, "/"); i > 0 { // drop the version segment
		path = path[i+1:]
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, bodyTok := body["access_token"]
	s.reqs = append(s.reqs, Req{Method: r.Method, Path: path, Bearer: hasBearer, BodyHasToken: bodyTok,
		HasQueryToken: q.Get("access_token") != "", HasClientSecret: q.Get("client_secret") != ""})
	switch {
	case r.Method == http.MethodGet && path == "oauth/access_token":
		if q.Get("client_id") != s.app.id || q.Get("client_secret") != s.app.secret {
			graphError(w, 400, 101, "invalid app")
			return
		}
		if gt := q.Get("grant_type"); gt == "fb_exchange_token" {
			long, ok := s.short[q.Get("fb_exchange_token")]
			if !ok {
				graphError(w, 400, 190, "invalid token")
				return
			}
			writeJSON(w, map[string]any{"access_token": long, "token_type": "bearer"})
			return
		}
		if q.Get("redirect_uri") != s.app.redirect {
			graphError(w, 400, 100, "redirect_uri mismatch")
			return
		}
		short, ok := s.codes[q.Get("code")]
		if !ok {
			graphError(w, 400, 100, "invalid code")
			return
		}
		delete(s.codes, q.Get("code")) // single use
		writeJSON(w, map[string]any{"access_token": short, "token_type": "bearer"})
	case r.Method == http.MethodGet && path == "me/permissions":
		u := s.userOf(bearer)
		if u == nil {
			graphError(w, 401, 190, "invalid token")
			return
		}
		data := []map[string]string{}
		for _, p := range u.Permissions {
			data = append(data, map[string]string{"permission": p, "status": "granted"})
		}
		writeJSON(w, map[string]any{"data": data})
	case r.Method == http.MethodGet && path == "me/accounts":
		u := s.userOf(bearer)
		if u == nil || s.failAccounts {
			graphError(w, 401, 190, "invalid token")
			return
		}
		data := []map[string]any{}
		for _, p := range u.Pages {
			item := map[string]any{"id": p.ID, "name": p.Name, "tasks": p.Tasks, "access_token": p.Token}
			if p.IGID != "" {
				item["instagram_business_account"] = map[string]string{"id": p.IGID, "username": p.IGName}
			}
			data = append(data, item)
		}
		writeJSON(w, map[string]any{"data": data})
	case r.Method == http.MethodPost && strings.HasSuffix(path, "/subscribed_apps"):
		page := strings.TrimSuffix(path, "/subscribed_apps")
		tok, _ := body["access_token"].(string)
		if s.pageOf[tok] != page || s.failSubscribe || q.Get("subscribed_fields") != "feed" {
			graphError(w, 400, 200, "cannot subscribe")
			return
		}
		s.subscribed[page] = true
		writeJSON(w, map[string]any{"success": true})
	case r.Method == http.MethodDelete && strings.HasSuffix(path, "/subscribed_apps"):
		page := strings.TrimSuffix(path, "/subscribed_apps")
		if s.pageOf[bearer] != page {
			graphError(w, 400, 200, "cannot unsubscribe")
			return
		}
		s.subscribed[page] = false
		writeJSON(w, map[string]any{"success": true})
	default:
		graphError(w, 404, 803, "unknown route")
	}
}

func (s *Server) userOf(long string) *User {
	key, ok := s.long[long]
	if !ok {
		return nil
	}
	return s.users[key]
}
