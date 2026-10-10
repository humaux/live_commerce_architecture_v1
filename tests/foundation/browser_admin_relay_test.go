//go:build browser

// Purpose: keep a browser fixture's public admin origin owned while Next starts on an automatic private port.
// Depends on: Go net/http/httptest/httputil and browserFront's existing WebKit TLS boundary; no database.
// Used by: publish, domains and merchant-buyer browser fixtures; GATE-PORT real-socket regression tests.
package foundation_test

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

type browserAdminRelay struct {
	origin string
	server *httptest.Server
	target atomic.Pointer[url.URL]
}

// newBrowserAdminRelay preserves the IdP/CSRF origin instead of lending its port to a later child process.
func newBrowserAdminRelay(t *testing.T) *browserAdminRelay {
	t.Helper()
	front := &browserAdminRelay{}
	proxy := &httputil.ReverseProxy{Rewrite: func(r *httputil.ProxyRequest) {
		r.SetURL(front.target.Load())
		r.Out.Host = r.In.Host
		// Match browserFront: keep its HTTPS marker and the inbound client-IP fixture, not a new relay hop.
		for _, name := range []string{"X-Forwarded-For", "X-Forwarded-Proto"} {
			if values, ok := r.In.Header[name]; ok {
				r.Out.Header[name] = values
			}
		}
	}}
	front.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if front.target.Load() == nil {
			http.Error(w, "fixture admin not connected", http.StatusServiceUnavailable)
			return
		}
		proxy.ServeHTTP(w, r)
	}))
	t.Cleanup(front.server.Close)
	front.origin = browserFront(t, front.server.Listener.Addr().String())
	return front
}

// connect accepts only an internal loopback port behind the caller's existing authenticated control endpoint.
func (front *browserAdminRelay) connect(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Port int `json:"port"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&in) != nil || decoder.Decode(new(any)) != io.EOF || in.Port < 1 || in.Port > 65535 ||
		front.server.Listener.Addr().String() == net.JoinHostPort("127.0.0.1", strconv.Itoa(in.Port)) ||
		front.origin == "https://127.0.0.1:"+strconv.Itoa(in.Port) {
		http.Error(w, "invalid fixture admin port", http.StatusBadRequest)
		return
	}
	target := &url.URL{Scheme: "http", Host: net.JoinHostPort("127.0.0.1", strconv.Itoa(in.Port))}
	if !front.target.CompareAndSwap(nil, target) && front.target.Load().Host != target.Host {
		http.Error(w, "fixture admin already connected", http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func TestBrowserAdminRelayOwnsExplicitOrigin(t *testing.T) {
	t.Setenv("LC_BROWSER_ENGINE", "chromium")
	front := newBrowserAdminRelay(t)
	address := front.server.Listener.Addr().String()
	stolen, err := net.Listen("tcp", address) // deterministic rival fixture, exactly the PR29 collision window
	if err == nil {
		stolen.Close()
		t.Fatal("another fixture stole the explicit public admin port before Next startup")
	}
	for range 3 {
		other := httptest.NewServer(http.NotFoundHandler())
		t.Cleanup(other.Close)
		if other.Listener.Addr().String() == address {
			t.Fatal("a fixture's listen(0) reused the held public admin port")
		}
	}
	response, err := front.server.Client().Get(front.origin + "/api/stores")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("unconnected public relay: got %d, want 503", response.StatusCode)
	}
	front.server.Close()
	reclaimed, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatalf("owned cleanup did not release the public listener: %v", err)
	}
	reclaimed.Close()
}

func TestBrowserAdminRelayPreservesOriginAndTLSHeaders(t *testing.T) {
	for _, engine := range []string{"chromium", "webkit"} {
		t.Run(engine, func(t *testing.T) {
			t.Setenv("LC_BROWSER_ENGINE", engine)
			front := newBrowserAdminRelay(t)
			origin, err := url.Parse(front.origin)
			if err != nil {
				t.Fatal(err)
			}
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Host != origin.Host || r.URL.RequestURI() != "/api/auth/callback?code=synthetic&state=synthetic" ||
					r.Header.Get("X-Forwarded-For") != "192.0.2.12" ||
					(engine == "webkit" && r.Header.Get("X-Forwarded-Proto") != "https") {
					t.Errorf("public origin/path or forwarded fixture headers changed: host=%s path=%s", r.Host, r.URL.RequestURI())
				}
				w.Header().Set("Set-Cookie", "__Host-synthetic=fixture; Path=/; Secure; HttpOnly; SameSite=Lax")
				w.WriteHeader(http.StatusUnauthorized)
			}))
			t.Cleanup(upstream.Close)
			_, port, err := net.SplitHostPort(upstream.Listener.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			_, selfPort, err := net.SplitHostPort(front.server.Listener.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			for _, body := range []string{`{"port":0}`, `{"port":65536}`, `{"port":1,"host":"example.invalid"}`, `{"port":1} {}`, `{"port":` + selfPort + `}`} {
				r := httptest.NewRecorder()
				front.connect(r, httptest.NewRequest("POST", "/admin-upstream", strings.NewReader(body)))
				if r.Code != http.StatusBadRequest || front.target.Load() != nil {
					t.Fatalf("invalid control body accepted: %s status=%d", body, r.Code)
				}
			}
			r := httptest.NewRecorder()
			front.connect(r, httptest.NewRequest("POST", "/admin-upstream", strings.NewReader(`{"port":`+port+`}`)))
			if r.Code != http.StatusNoContent || front.origin != origin.String() {
				t.Fatal("connect changed the public origin or did not acknowledge")
			}
			for _, body := range []string{`{"port":0}`, `{"port":1}`} {
				r := httptest.NewRecorder()
				front.connect(r, httptest.NewRequest("POST", "/admin-upstream", strings.NewReader(body)))
				if (r.Code != http.StatusBadRequest && r.Code != http.StatusConflict) || front.target.Load().Host != upstream.Listener.Addr().String() {
					t.Fatal("a bad or different registration changed the already connected upstream")
				}
			}
			request, err := http.NewRequest("GET", front.origin+"/api/auth/callback?code=synthetic&state=synthetic", nil)
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("X-Forwarded-For", "192.0.2.12")
			// The same existing httptest self-signed front as browserFront; no production trust is bypassed.
			client := upstream.Client()
			if engine == "webkit" {
				client = browserAdminTLSClient(t)
			}
			response, err := client.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != http.StatusUnauthorized || !strings.Contains(response.Header.Get("Set-Cookie"), "Secure; HttpOnly") {
				t.Fatalf("upstream auth status/cookie changed: status=%d", response.StatusCode)
			}
		})
	}
}

func TestBrowserAdminRelayPreservesPostCSRFAndRedirect(t *testing.T) {
	for _, engine := range []string{"chromium", "webkit"} {
		t.Run(engine, func(t *testing.T) {
			t.Setenv("LC_BROWSER_ENGINE", engine)
			front := newBrowserAdminRelay(t)
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil || r.Method != "POST" || string(body) != `{"synthetic":true}` ||
					r.Header.Get("Origin") != front.origin || r.Header.Get("Cookie") != "__Host-commerce_session=synthetic" ||
					r.Header.Get("X-CSRF-Token") != "synthetic-csrf" || r.URL.RequestURI() != "/api/stores?fixture=1" {
					t.Error("owned frontend changed the request body, method, Origin, CSRF, cookie or query")
				}
				w.Header().Set("Location", front.origin+"/en")
				w.WriteHeader(http.StatusTemporaryRedirect)
			}))
			t.Cleanup(upstream.Close)
			_, port, err := net.SplitHostPort(upstream.Listener.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			r := httptest.NewRecorder()
			front.connect(r, httptest.NewRequest("POST", "/admin-upstream", strings.NewReader(`{"port":`+port+`}`)))
			if r.Code != http.StatusNoContent {
				t.Fatal("fixture registration did not acknowledge")
			}
			request, err := http.NewRequest("POST", front.origin+"/api/stores?fixture=1", strings.NewReader(`{"synthetic":true}`))
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("Origin", front.origin)
			request.Header.Set("Cookie", "__Host-commerce_session=synthetic")
			request.Header.Set("X-CSRF-Token", "synthetic-csrf")
			client := &http.Client{}
			if engine == "webkit" {
				client = browserAdminTLSClient(t)
			}
			client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
			response, err := client.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != http.StatusTemporaryRedirect || response.Header.Get("Location") != front.origin+"/en" {
				t.Fatal("owned frontend changed the response status or public redirect")
			}
		})
	}
}

// browserAdminTLSClient trusts httptest's standard test certificate without disabling certificate verification.
func browserAdminTLSClient(t *testing.T) *http.Client {
	t.Helper()
	certificate := httptest.NewTLSServer(http.NotFoundHandler())
	t.Cleanup(certificate.Close)
	return certificate.Client()
}
