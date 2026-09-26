package livekit_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	livekit "livecommerce/internal/integrations/livekit"
)

const (
	room       = "lc_0123456789abcdef0123456789abcdef"
	egressID   = "EG_test_1"
	apiSecret  = "0123456789abcdef0123456789abcdef"
	streamOne  = "rtmps://ingest.example.com/live/secret-key-ONE?token=secret-query"
	streamTwo  = "rtmps://second.example.net/live/secret-key-TWO"
	secretWord = "ATTACKER_SECRET_CANARY"
)

func config() livekit.Config {
	return livekit.Config{Environment: "MOCK", Endpoint: "https://tenant.livekit.cloud", APIKey: "test_key-1", APISecret: apiSecret,
		StreamHosts: []string{"ingest.example.com", "second.example.net"}}
}

func input() livekit.StartInput {
	return livekit.StartInput{RoomName: room, AspectRatio: "16:9", StreamURLs: []string{streamOne, streamTwo}}
}

func target() livekit.Target { return livekit.Target{RoomName: room, EgressID: egressID} }

func info() string {
	return `{"egress_id":"` + egressID + `","room_name":"` + room + `","status":"EGRESS_ACTIVE","started_at":"123","updated_at":456,"ended_at":"0"}`
}

// Dial only the local TLS fixture while retaining the frozen Cloud URL, Host and path.
func fixtureTransport(s *httptest.Server, extras ...*httptest.Server) *http.Transport {
	tr := s.Client().Transport.(*http.Transport).Clone()
	tr.Proxy = nil
	tr.DisableKeepAlives = true
	tr.TLSClientConfig = tr.TLSClientConfig.Clone()
	tr.TLSClientConfig.InsecureSkipVerify = true // Local fixture certificates; dial targets are explicitly pinned below.
	routes := map[string]string{"tenant.livekit.cloud:443": s.Listener.Addr().String()}
	for _, extra := range extras {
		u, err := url.Parse(extra.URL)
		if err != nil {
			panic(err)
		}
		routes[u.Host] = extra.Listener.Addr().String()
	}
	tr.DialContext = func(ctx context.Context, _, address string) (net.Conn, error) {
		addr, ok := routes[address]
		if !ok {
			return nil, errors.New("unapproved fixture address")
		}
		return (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	}
	return tr
}

func fixtureClient(t *testing.T, handler http.HandlerFunc) (*livekit.Client, *httptest.Server, *atomic.Int32) {
	t.Helper()
	calls := new(atomic.Int32)
	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		handler(w, r)
	}))
	t.Cleanup(s.Close)
	c, err := livekit.New(config(), fixtureTransport(s))
	if err != nil {
		t.Fatal(err)
	}
	return c, s, calls
}

func reply(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, body)
}

func requireErr(t *testing.T, err, want error) {
	t.Helper()
	if err != want {
		t.Fatalf("error class: got %v, want %v", err, want)
	}
	for _, canary := range []string{secretWord, streamOne, streamTwo, apiSecret} {
		if strings.Contains(fmt.Sprint(err), canary) {
			t.Fatal("error leaked a secret")
		}
	}
}

func requireZero(t *testing.T, got livekit.Observation) {
	t.Helper()
	if got != (livekit.Observation{}) {
		t.Fatalf("failure returned partial observation: %+v", got)
	}
}

func requireJSON(t *testing.T, body []byte, want string) {
	t.Helper()
	var a, b any
	if err := json.Unmarshal(body, &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(want), &b); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("wrong provider JSON: got %s, want %s", body, want)
	}
}

func requireJWT(t *testing.T, authorization string) {
	t.Helper()
	if !strings.HasPrefix(authorization, "Bearer ") {
		t.Fatal("missing bearer token")
	}
	parts := strings.Split(strings.TrimPrefix(authorization, "Bearer "), ".")
	if len(parts) != 3 {
		t.Fatal("JWT segment count")
	}
	decode := func(s string) map[string]any {
		t.Helper()
		b, err := base64.RawURLEncoding.DecodeString(s)
		if err != nil {
			t.Fatal(err)
		}
		var v map[string]any
		if err := json.Unmarshal(b, &v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	if got := decode(parts[0]); !reflect.DeepEqual(got, map[string]any{"alg": "HS256", "typ": "JWT"}) {
		t.Fatalf("JWT header: %v", got)
	}
	mac := hmac.New(sha256.New, []byte(apiSecret))
	_, _ = io.WriteString(mac, parts[0]+"."+parts[1])
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(sig, mac.Sum(nil)) {
		t.Fatal("JWT HMAC mismatch")
	}
	p := decode(parts[1])
	if len(p) != 5 || p["iss"] != "test_key-1" || !reflect.DeepEqual(p["video"], map[string]any{"room": room, "roomRecord": true}) {
		t.Fatalf("JWT grants/issuer: %v", p)
	}
	now := time.Now().Unix()
	iat, iok := p["iat"].(float64)
	nbf, nok := p["nbf"].(float64)
	exp, eok := p["exp"].(float64)
	if !iok || !nok || !eok || int64(iat) < now-3 || int64(iat) > now+3 || iat != nbf || exp != iat+60 {
		t.Fatalf("JWT lifetime: iat=%v nbf=%v exp=%v", p["iat"], p["nbf"], p["exp"])
	}
}

func TestLKP01ExactWireAndJWT(t *testing.T) {
	var wantBody string
	var wantPath string
	type wireRequest struct {
		method, path, host, query, idempotency, authorization string
		body                                                  []byte
		readErr                                               error
	}
	captured := make(chan wireRequest, 4)
	c, _, calls := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		captured <- wireRequest{r.Method, r.URL.Path, r.Host, r.URL.RawQuery, r.Header.Get("Idempotency-Key"), r.Header.Get("Authorization"), body, err}
		if strings.HasSuffix(r.URL.Path, "/ListEgress") {
			reply(w, `{"items":[`+info()+`]}`)
		} else {
			reply(w, info())
		}
	})
	assertWire := func() {
		t.Helper()
		r := <-captured
		if r.readErr != nil || r.method != http.MethodPost || r.path != wantPath || r.host != "tenant.livekit.cloud" || r.query != "" || r.idempotency != "" {
			t.Fatalf("wrong provider request: %+v", r)
		}
		requireJWT(t, r.authorization)
		requireJSON(t, r.body, wantBody)
	}
	for _, shape := range []struct{ aspect, preset string }{{"16:9", "H264_720P_30"}, {"9:16", "PORTRAIT_H264_720P_30"}} {
		in := input()
		in.AspectRatio = shape.aspect
		wantPath = "/twirp/livekit.Egress/StartEgress"
		wantBody = `{"room_name":"` + room + `","template":{"layout":"speaker"},"preset":"` + shape.preset + `","outputs":[{"stream":{"protocol":"RTMP","urls":["` + streamOne + `","` + streamTwo + `"]}}]}`
		got, err := c.Start(context.Background(), in)
		if err != nil || got.EgressID != egressID || got.RoomName != room || got.StartedAtNS != 123 || got.UpdatedAtNS != 456 {
			t.Fatalf("Start: observation=%+v err=%v", got, err)
		}
		assertWire()
	}
	wantPath = "/twirp/livekit.Egress/ListEgress"
	wantBody = `{"room_name":"` + room + `","egress_id":"` + egressID + `","active":false}`
	if got, err := c.Query(context.Background(), target()); err != nil || got.EgressID != egressID {
		t.Fatalf("Query: %+v %v", got, err)
	}
	assertWire()
	wantPath = "/twirp/livekit.Egress/StopEgress"
	wantBody = `{"egress_id":"` + egressID + `"}`
	if got, err := c.Stop(context.Background(), target()); err != nil || got.EgressID != egressID {
		t.Fatalf("Stop: %+v %v", got, err)
	}
	assertWire()
	if calls.Load() != 4 {
		t.Fatalf("expected one request per call: %d", calls.Load())
	}
}

func TestLKP02ConstructorValidationAndRedaction(t *testing.T) {
	for name, change := range map[string]func(*livekit.Config){
		"environment":        func(c *livekit.Config) { c.Environment = "mock" },
		"http":               func(c *livekit.Config) { c.Endpoint = "http://tenant.livekit.cloud" },
		"endpoint port":      func(c *livekit.Config) { c.Endpoint += ":443" },
		"endpoint subdomain": func(c *livekit.Config) { c.Endpoint = "a.b.livekit.cloud" },
		"endpoint uppercase": func(c *livekit.Config) { c.Endpoint = "https://Tenant.livekit.cloud" },
		"endpoint userinfo":  func(c *livekit.Config) { c.Endpoint = "https://u@tenant.livekit.cloud" },
		"endpoint query":     func(c *livekit.Config) { c.Endpoint += "/?x=1" },
		"endpoint path":      func(c *livekit.Config) { c.Endpoint += "/twirp" },
		"endpoint fragment":  func(c *livekit.Config) { c.Endpoint += "#secret" },
		"api key":            func(c *livekit.Config) { c.APIKey = "bad key" },
		"api key length":     func(c *livekit.Config) { c.APIKey = strings.Repeat("a", 129) },
		"api secret":         func(c *livekit.Config) { c.APISecret = "short" },
		"api secret space":   func(c *livekit.Config) { c.APISecret = strings.Repeat("a", 32) + " " },
		"api secret unicode": func(c *livekit.Config) { c.APISecret = strings.Repeat("a", 32) + "密" },
		"empty hosts":        func(c *livekit.Config) { c.StreamHosts = nil },
		"too many hosts": func(c *livekit.Config) {
			c.StreamHosts = make([]string, 17)
			for i := range c.StreamHosts {
				c.StreamHosts[i] = fmt.Sprintf("host%d.example.com", i)
			}
		},
		"duplicate hosts": func(c *livekit.Config) { c.StreamHosts = []string{"ingest.example.com", "ingest.example.com"} },
		"wildcard":        func(c *livekit.Config) { c.StreamHosts = []string{"*.example.com"} },
		"ip literal":      func(c *livekit.Config) { c.StreamHosts = []string{"127.0.0.1"} },
		"localhost":       func(c *livekit.Config) { c.StreamHosts = []string{"localhost"} },
		"single label":    func(c *livekit.Config) { c.StreamHosts = []string{"ingest"} },
		"uppercase host":  func(c *livekit.Config) { c.StreamHosts = []string{"Ingest.example.com"} },
	} {
		t.Run(name, func(t *testing.T) {
			cfg := config()
			change(&cfg)
			if _, err := livekit.New(cfg, http.RoundTripper(roundTrip(func(*http.Request) (*http.Response, error) { t.Fatal("unexpected I/O"); return nil, nil }))); !errors.Is(err, livekit.ErrInvalid) {
				t.Fatalf("constructor: %v", err)
			}
		})
	}
	if _, err := livekit.New(config()); !errors.Is(err, livekit.ErrInvalid) {
		t.Fatalf("MOCK silently used a default transport: %v", err)
	}
	var typedNil *http.Transport
	if _, err := livekit.New(config(), typedNil); !errors.Is(err, livekit.ErrInvalid) {
		t.Fatalf("typed-nil test transport accepted: %v", err)
	}
	live := config()
	live.Environment = "LIVE"
	if _, err := livekit.New(live, http.RoundTripper(roundTrip(func(*http.Request) (*http.Response, error) { t.Fatal("unexpected I/O"); return nil, nil }))); !errors.Is(err, livekit.ErrInvalid) {
		t.Fatalf("LIVE transport injection accepted: %v", err)
	}

	for _, value := range []any{config(), input()} {
		for _, display := range []string{fmt.Sprint(value), fmt.Sprintf("%#v", value)} {
			for _, secret := range []string{apiSecret, streamOne, streamTwo} {
				if strings.Contains(display, secret) {
					t.Fatal("String or GoString leaked a secret")
				}
			}
		}
		b, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		for _, secret := range []string{apiSecret, streamOne, streamTwo} {
			if bytes.Contains(b, []byte(secret)) {
				t.Fatal("MarshalJSON leaked a secret")
			}
		}
	}
	c, _, _ := fixtureClient(t, func(w http.ResponseWriter, _ *http.Request) { reply(w, info()) })
	for _, display := range []string{fmt.Sprint(c), fmt.Sprintf("%#v", c)} {
		if strings.Contains(display, apiSecret) {
			t.Fatal("Client display leaked secret")
		}
	}
	b, err := json.Marshal(c)
	if err != nil || bytes.Contains(b, []byte(apiSecret)) {
		t.Fatalf("Client JSON leaked secret: %v", err)
	}
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestLKP02InputAndTargetRejectedBeforeIO(t *testing.T) {
	c, _, calls := fixtureClient(t, func(w http.ResponseWriter, _ *http.Request) { reply(w, info()) })
	for name, change := range map[string]func(*livekit.StartInput){
		"room":      func(v *livekit.StartInput) { v.RoomName = "lc_NOT_HEX" },
		"aspect":    func(v *livekit.StartInput) { v.AspectRatio = "4:3" },
		"zero URLs": func(v *livekit.StartInput) { v.StreamURLs = nil },
		"three URLs": func(v *livekit.StartInput) {
			v.StreamURLs = []string{streamOne, streamTwo, "rtmps://ingest.example.com/live/third"}
		},
		"duplicate URL": func(v *livekit.StartInput) { v.StreamURLs = []string{streamOne, streamOne} },
		"scheme":        func(v *livekit.StartInput) { v.StreamURLs = []string{"rtmp://ingest.example.com/live/key"} },
		"host":          func(v *livekit.StartInput) { v.StreamURLs = []string{"rtmps://evil.example.com/live/key"} },
		"suffix spoof":  func(v *livekit.StartInput) { v.StreamURLs = []string{"rtmps://ingest.example.com.evil.test/live/key"} },
		"userinfo":      func(v *livekit.StartInput) { v.StreamURLs = []string{"rtmps://u@ingest.example.com/live/key"} },
		"port":          func(v *livekit.StartInput) { v.StreamURLs = []string{"rtmps://ingest.example.com:444/live/key"} },
		"fragment":      func(v *livekit.StartInput) { v.StreamURLs = []string{"rtmps://ingest.example.com/live/key#secret"} },
		"empty path":    func(v *livekit.StartInput) { v.StreamURLs = []string{"rtmps://ingest.example.com"} },
		"control":       func(v *livekit.StartInput) { v.StreamURLs = []string{"rtmps://ingest.example.com/live/\nkey"} },
		"space":         func(v *livekit.StartInput) { v.StreamURLs = []string{"rtmps://ingest.example.com/live/a b"} },
		"unicode":       func(v *livekit.StartInput) { v.StreamURLs = []string{"rtmps://ingest.example.com/live/密钥"} },
		"oversize": func(v *livekit.StartInput) {
			v.StreamURLs = []string{"rtmps://ingest.example.com/live/" + strings.Repeat("a", 4097)}
		},
	} {
		t.Run(name, func(t *testing.T) {
			v := input()
			change(&v)
			got, err := c.Start(context.Background(), v)
			requireErr(t, err, livekit.ErrInvalid)
			requireZero(t, got)
		})
	}
	for name, v := range map[string]livekit.Target{
		"room":          {RoomName: "other", EgressID: egressID},
		"egress":        {RoomName: room, EgressID: "bad"},
		"egress length": {RoomName: room, EgressID: "EG_" + strings.Repeat("a", 101)},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := c.Query(context.Background(), v)
			requireErr(t, err, livekit.ErrInvalid)
			requireZero(t, got)
			got, err = c.Stop(context.Background(), v)
			requireErr(t, err, livekit.ErrInvalid)
			requireZero(t, got)
		})
	}
	var zero *livekit.Client
	if got, err := zero.Start(context.Background(), input()); !errors.Is(err, livekit.ErrInvalid) || got != (livekit.Observation{}) {
		t.Fatalf("nil client Start: %+v %v", got, err)
	}
	var value livekit.Client
	for _, method := range []string{"Start", "Query", "Stop"} {
		t.Run("zero client/"+method, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("zero client %s panicked: %v", method, r)
				}
			}()
			got, err := invoke(&value, method, context.Background())
			requireErr(t, err, livekit.ErrInvalid)
			requireZero(t, got)
		})
	}
	for _, call := range []func() (livekit.Observation, error){
		func() (livekit.Observation, error) { return c.Start(nil, input()) },
		func() (livekit.Observation, error) { return c.Query(nil, target()) },
		func() (livekit.Observation, error) { return c.Stop(nil, target()) },
	} {
		got, err := call()
		requireErr(t, err, livekit.ErrInvalid)
		requireZero(t, got)
	}
	if calls.Load() != 0 {
		t.Fatalf("invalid input caused %d I/O calls", calls.Load())
	}
}

func TestLKP02HostAllowlistIsCopied(t *testing.T) {
	cfg := config()
	calls := new(atomic.Int32)
	// This separate construction retains cfg's slice so the caller can mutate it.
	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { calls.Add(1); reply(w, info()) }))
	defer s.Close()
	copyClient, err := livekit.New(cfg, fixtureTransport(s))
	if err != nil {
		t.Fatal(err)
	}
	cfg.StreamHosts[0] = "evil.example.com"
	good := input()
	good.StreamURLs = []string{streamOne}
	if _, err := copyClient.Start(context.Background(), good); err != nil {
		t.Fatalf("original host invalidated by caller mutation: %v", err)
	}
	bad := input()
	bad.StreamURLs = []string{"rtmps://evil.example.com/live/key"}
	got, err := copyClient.Start(context.Background(), bad)
	requireErr(t, err, livekit.ErrInvalid)
	requireZero(t, got)
	if calls.Load() != 1 {
		t.Fatalf("host allowlist was aliased: calls=%d", calls.Load())
	}
}

func TestLKP02BoundaryValidStreamURLs(t *testing.T) {
	c, _, calls := fixtureClient(t, func(w http.ResponseWriter, _ *http.Request) { reply(w, info()) })
	for _, stream := range []string{
		"rtmps://ingest.example.com:443/live/key",
		"rtmps://ingest.example.com/live/" + strings.Repeat("a", 4096-len("rtmps://ingest.example.com/live/")),
	} {
		in := input()
		in.StreamURLs = []string{stream}
		if _, err := c.Start(context.Background(), in); err != nil {
			t.Fatalf("valid stream boundary rejected: %v", err)
		}
	}
	if calls.Load() != 2 {
		t.Fatalf("valid stream URLs did not reach provider: %d", calls.Load())
	}
}

func TestLKP03DecodeAndCorrelation(t *testing.T) {
	var body string
	c, _, _ := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/ListEgress") {
			reply(w, `{"items":[`+body+`]}`)
		} else {
			reply(w, body)
		}
	})
	for _, tc := range []struct {
		name, data, status      string
		started, updated, ended int64
	}{
		{"snake-string", info(), "EGRESS_ACTIVE", 123, 456, 0},
		{"camel-number", `{"egressId":"` + egressID + `","roomName":"` + room + `","status":1,"startedAt":12,"updatedAt":"34","endedAt":0}`, "EGRESS_ACTIVE", 12, 34, 0},
		{"proto defaults", `{"egress_id":"` + egressID + `","room_name":"` + room + `"}`, "EGRESS_STARTING", 0, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body = tc.data
			for _, call := range []func() (livekit.Observation, error){
				func() (livekit.Observation, error) { return c.Start(context.Background(), input()) },
				func() (livekit.Observation, error) { return c.Query(context.Background(), target()) },
				func() (livekit.Observation, error) { return c.Stop(context.Background(), target()) },
			} {
				got, err := call()
				if err != nil || got.EgressID != egressID || got.RoomName != room || got.Status != tc.status || got.StartedAtNS != tc.started || got.UpdatedAtNS != tc.updated || got.EndedAtNS != tc.ended {
					t.Fatalf("decode: %+v %v", got, err)
				}
			}
		})
	}
	for _, status := range []int{0, 1, 2, 3, 4, 5, 6} {
		body = fmt.Sprintf(`{"egress_id":%q,"room_name":%q,"status":%d}`, egressID, room, status)
		if _, err := c.Start(context.Background(), input()); err != nil {
			t.Fatalf("enum %d: %v", status, err)
		}
	}
	for _, status := range []string{"EGRESS_STARTING", "EGRESS_ACTIVE", "EGRESS_ENDING", "EGRESS_COMPLETE", "EGRESS_FAILED", "EGRESS_ABORTED", "EGRESS_LIMIT_REACHED"} {
		body = fmt.Sprintf(`{"egress_id":%q,"room_name":%q,"status":%q}`, egressID, room, status)
		got, err := c.Start(context.Background(), input())
		if err != nil || got.Status != status {
			t.Fatalf("canonical enum %s: %+v %v", status, got, err)
		}
	}
	// The root object counts as container 1. The scalar leaf does not add a level.
	body = strings.TrimSuffix(info(), "}") + `,"extra":` + strings.Repeat(`[`, 31) + `0` + strings.Repeat(`]`, 31) + `}`
	if _, err := c.Start(context.Background(), input()); err != nil {
		t.Fatalf("32-container boundary rejected: %v", err)
	}
	for name, data := range map[string]string{
		"wrong room": `{"egress_id":"` + egressID + `","room_name":"other"}`,
		"wrong ID":   `{"egress_id":"EG_other","room_name":"` + room + `"}`,
		"invalid ID": `{"egress_id":"bad","room_name":"` + room + `"}`,
	} {
		t.Run(name, func(t *testing.T) {
			body = data
			got, err := c.Start(context.Background(), input())
			if name == "wrong ID" {
				// Start has no requested egress ID to correlate; a valid new ID is allowed.
				if err != nil || got.EgressID != "EG_other" {
					t.Fatalf("Start rejected a valid provider-assigned ID: %+v %v", got, err)
				}
			} else {
				requireErr(t, err, livekit.ErrUnknown)
				requireZero(t, got)
			}
			got, err = c.Stop(context.Background(), target())
			requireErr(t, err, livekit.ErrUnknown)
			requireZero(t, got)
			got, err = c.Query(context.Background(), target())
			requireErr(t, err, livekit.ErrUnavailable)
			requireZero(t, got)
		})
	}
}

func TestLKP03StrictJSONAndRedaction(t *testing.T) {
	var body []byte
	c, _, _ := fixtureClient(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(body) })
	base := info()
	for name, data := range map[string][]byte{
		"malformed":                []byte(`{"egress_id":`),
		"trailing":                 []byte(base + `{}`),
		"duplicate known":          []byte(strings.Replace(base, `"room_name":`, `"room_name":"`+room+`","room_name":`, 1)),
		"duplicate unknown nested": []byte(strings.TrimSuffix(base, "}") + `,"extra":{"x":1,"x":2}}`),
		"alias pair":               []byte(strings.TrimSuffix(base, "}") + `,"roomName":"` + room + `"}`),
		"nonobject":                []byte(`[]`),
		"invalid UTF8":             bytes.Replace([]byte(base), []byte(egressID), []byte{'E', 'G', '_', 0xff}, 1),
		"deep unknown":             []byte(strings.TrimSuffix(base, "}") + `,"extra":` + strings.Repeat(`[`, 33) + `0` + strings.Repeat(`]`, 33) + `}`),
		"oversized":                []byte(strings.TrimSuffix(base, "}") + `,"extra":"` + strings.Repeat("x", 65536) + `"}`),
		"status null":              []byte(strings.Replace(base, `"EGRESS_ACTIVE"`, `null`, 1)),
		"status unknown":           []byte(strings.Replace(base, `"EGRESS_ACTIVE"`, `"UNKNOWN_`+secretWord+`"`, 1)),
		"status out of range":      []byte(strings.Replace(base, `"EGRESS_ACTIVE"`, `7`, 1)),
		"status wrong type":        []byte(strings.Replace(base, `"EGRESS_ACTIVE"`, `true`, 1)),
		"time null":                []byte(strings.Replace(base, `"123"`, `null`, 1)),
		"time negative":            []byte(strings.Replace(base, `"123"`, `-1`, 1)),
		"time overflow":            []byte(strings.Replace(base, `"123"`, `"9223372036854775808"`, 1)),
		"time fraction":            []byte(strings.Replace(base, `"123"`, `1.5`, 1)),
		"time exponent":            []byte(strings.Replace(base, `"123"`, `1e3`, 1)),
		"time alias pair":          []byte(strings.TrimSuffix(base, "}") + `,"startedAt":"123"}`),
		"id wrong type":            []byte(strings.Replace(base, `"EG_test_1"`, `1`, 1)),
	} {
		t.Run(name, func(t *testing.T) {
			body = data
			got, err := c.Start(context.Background(), input())
			requireErr(t, err, livekit.ErrUnknown)
			requireZero(t, got)
		})
	}
	body = []byte(strings.TrimSuffix(base, "}") + `,"request":{"stream_urls":["` + secretWord + `"]},"error":"` + secretWord + `","details":"` + secretWord + `"}`)
	got, err := c.Start(context.Background(), input())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(fmt.Sprintf("%+v", got), secretWord) {
		t.Fatal("observation leaked unknown provider fields")
	}
}

func TestLKP03DecompressedResponseCeiling(t *testing.T) {
	var plain string
	c, _, calls := fixtureClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		z := gzip.NewWriter(w)
		_, _ = io.WriteString(z, plain)
		_ = z.Close()
	})
	prefix := strings.TrimSuffix(info(), "}") + `,"future":"`
	suffix := `"}`
	for _, extra := range []int{0, 1} {
		plain = prefix + strings.Repeat("x", 65536-len(prefix)-len(suffix)+extra) + suffix
		got, err := c.Start(context.Background(), input())
		if extra == 0 {
			if err != nil || got.EgressID != egressID {
				t.Fatalf("exact 64KiB decompressed body rejected: %+v %v", got, err)
			}
		} else {
			requireErr(t, err, livekit.ErrUnknown)
			requireZero(t, got)
		}
	}
	if calls.Load() != 2 {
		t.Fatalf("unexpected gzip request count: %d", calls.Load())
	}
}

func TestLKP03ListPaginationBeforeEmptyAndAmbiguity(t *testing.T) {
	var body string
	c, _, _ := fixtureClient(t, func(w http.ResponseWriter, _ *http.Request) { reply(w, body) })
	for name, data := range map[string]string{
		"empty":                          `{}`,
		"empty items":                    `{"items":[]}`,
		"empty pagination":               `{"items":[],"next_page_token":{}}`,
		"empty token":                    `{"items":[],"nextPageToken":{"token":""}}`,
		"empty token with unknown field": `{"items":[],"next_page_token":{"token":"","future_field":42}}`,
	} {
		t.Run(name, func(t *testing.T) {
			body = data
			got, err := c.Query(context.Background(), target())
			requireErr(t, err, livekit.ErrNotObserved)
			requireZero(t, got)
		})
	}
	for name, data := range map[string]string{
		"nonempty token":         `{"items":[],"next_page_token":{"token":"more"}}`,
		"null token":             `{"items":[],"next_page_token":null}`,
		"string token":           `{"items":[],"next_page_token":"more"}`,
		"wrong token field type": `{"items":[],"next_page_token":{"token":42}}`,
		"pagination aliases":     `{"items":[],"next_page_token":{},"nextPageToken":{}}`,
		"items null":             `{"items":null}`,
		"items wrong type":       `{"items":{}}`,
		"unrelated row":          `{"items":[{"egress_id":"EG_other","room_name":"` + room + `"}]}`,
		"duplicate match":        `{"items":[` + info() + `,` + info() + `]}`,
		"match plus unrelated":   `{"items":[` + info() + `,{"egress_id":"EG_other","room_name":"` + room + `"}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			body = data
			got, err := c.Query(context.Background(), target())
			requireErr(t, err, livekit.ErrUnavailable)
			requireZero(t, got)
		})
	}
}

func TestLKP04HTTPFailuresNoRetry(t *testing.T) {
	for _, method := range []string{"Start", "Query", "Stop"} {
		for _, code := range []int{302, 404, 503} {
			t.Run(fmt.Sprintf("%s/%d", method, code), func(t *testing.T) {
				redirected := new(atomic.Int32)
				second := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					redirected.Add(1)
					reply(w, info())
				}))
				defer second.Close()
				calls := new(atomic.Int32)
				first := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					calls.Add(1)
					if code == 302 {
						w.Header().Set("Location", second.URL+"/stolen")
					}
					w.WriteHeader(code)
					_, _ = io.WriteString(w, secretWord)
				}))
				defer first.Close()
				c, err := livekit.New(config(), fixtureTransport(first, second))
				if err != nil {
					t.Fatal(err)
				}
				got, err := invoke(c, method, context.Background())
				want := livekit.ErrUnknown
				if method == "Query" {
					want = livekit.ErrUnavailable
				}
				requireErr(t, err, want)
				requireZero(t, got)
				if calls.Load() != 1 || redirected.Load() != 0 {
					t.Fatalf("retry or redirect: first=%d second=%d", calls.Load(), redirected.Load())
				}
			})
		}
	}
}

func invoke(c *livekit.Client, method string, ctx context.Context) (livekit.Observation, error) {
	switch method {
	case "Start":
		return c.Start(ctx, input())
	case "Query":
		return c.Query(ctx, target())
	default:
		return c.Stop(ctx, target())
	}
}

func TestLKP04RealDropHangAndCancel(t *testing.T) {
	for _, method := range []string{"Start", "Query", "Stop"} {
		t.Run(method+"/drop", func(t *testing.T) {
			c, _, calls := fixtureClient(t, func(w http.ResponseWriter, _ *http.Request) {
				conn, _, err := w.(http.Hijacker).Hijack()
				if err != nil {
					t.Error(err)
					return
				}
				_ = conn.Close()
			})
			got, err := invoke(c, method, context.Background())
			want := livekit.ErrUnknown
			if method == "Query" {
				want = livekit.ErrUnavailable
			}
			requireErr(t, err, want)
			requireZero(t, got)
			if calls.Load() != 1 {
				t.Fatalf("dropped reply retried %d times", calls.Load())
			}
		})
		t.Run(method+"/hang", func(t *testing.T) {
			entered := make(chan struct{})
			release := make(chan struct{})
			defer close(release)
			c, _, calls := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				close(entered)
				select {
				case <-r.Context().Done():
				case <-release:
				}
			})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			type outcome struct {
				got livekit.Observation
				err error
			}
			finished := make(chan outcome, 1)
			go func() {
				got, err := invoke(c, method, ctx)
				finished <- outcome{got, err}
			}()
			select {
			case <-entered:
			case <-time.After(2 * time.Second):
				t.Fatal("request never reached the local hanging fixture")
			}
			cancel()
			var result outcome
			select {
			case result = <-finished:
			case <-time.After(2 * time.Second):
				t.Fatal("canceled request did not return")
			}
			want := livekit.ErrUnknown
			if method == "Query" {
				want = livekit.ErrUnavailable
			}
			requireErr(t, result.err, want)
			requireZero(t, result.got)
			if calls.Load() != 1 {
				t.Fatalf("hanging call retried %d times", calls.Load())
			}
		})
	}
	c, _, calls := fixtureClient(t, func(w http.ResponseWriter, _ *http.Request) { reply(w, info()) })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, method := range []string{"Start", "Query", "Stop"} {
		got, err := invoke(c, method, ctx)
		want := livekit.ErrUnknown
		if method == "Query" {
			want = livekit.ErrUnavailable
		}
		requireErr(t, err, want)
		requireZero(t, got)
	}
	if calls.Load() != 0 {
		t.Fatalf("pre-cancelled call sent %d requests", calls.Load())
	}
}

func TestLKP04FixedTenSecondCeiling(t *testing.T) {
	entered := make(chan struct{})
	observedCancel := make(chan struct{})
	release := make(chan struct{})
	defer close(release)
	c, _, calls := fixtureClient(t, func(_ http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		close(entered)
		select {
		case <-r.Context().Done():
			close(observedCancel)
		case <-release:
		}
	})
	type outcome struct {
		got livekit.Observation
		err error
	}
	finished := make(chan outcome, 1)
	started := time.Now()
	go func() {
		got, err := c.Start(context.Background(), input())
		finished <- outcome{got, err}
	}()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("request never reached local hanging fixture")
	}
	var result outcome
	select {
	case result = <-finished:
	case <-time.After(13 * time.Second):
		t.Fatal("client did not enforce the ten-second request ceiling")
	}
	duration := time.Since(started)
	requireErr(t, result.err, livekit.ErrUnknown)
	requireZero(t, result.got)
	if duration < 9*time.Second || duration > 13*time.Second || calls.Load() != 1 {
		t.Fatalf("request ceiling/retry: duration=%s calls=%d", duration, calls.Load())
	}
	select {
	case <-observedCancel:
	case <-time.After(time.Second):
		t.Fatal("local server did not observe client cancellation")
	}
}
