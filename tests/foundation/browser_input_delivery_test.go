//go:build browser

package foundation_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"livecommerce/internal/httpapi"
	"livecommerce/internal/identity"
	"livecommerce/internal/identityhttp"
	"livecommerce/internal/live"
	"livecommerce/internal/oidclogin"
)

// BRW05 proves signed browser login -> HTTPS BFF -> Go scoped COMMIT -> token.
// The local WSS endpoint proves browser transport acceptance, not SFU media.
func TestBrowserInputDeliveryBRW05RealChain(t *testing.T) {
	if os.Getenv("LC_BROWSER_INPUT_DELIVERY_ACCEPTANCE") != "1" || os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" {
		t.Fatal("use scripts/dev/test-local.sh --browser-input-delivery")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	stopCase, revokeCase := brwRegistered(t), brwRegistered(t)
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	evidence := filepath.Join(root, "output", "playwright", "input-delivery-"+time.Now().UTC().Format("20060102T150405.000000000"))
	if err := os.MkdirAll(evidence, 0700); err != nil {
		t.Fatal(err)
	}

	// Separate TLS listener: the URL is WSS, never mixed-content WS under HTTPS.
	// Only the WebSocket handshake is implemented; decoded SFU media is BRI04.
	var socketOpens atomic.Int64
	socket := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("Sec-WebSocket-Key")
		decoded, err := base64.StdEncoding.DecodeString(key)
		if r.Method != http.MethodGet || !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") || err != nil || len(decoded) != 16 {
			w.WriteHeader(http.StatusUpgradeRequired)
			return
		}
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		conn, rw, err := hijacker.Hijack()
		if err != nil {
			return
		}
		defer conn.Close()
		accept := sha1.Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
		_, _ = fmt.Fprintf(rw, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n", base64.StdEncoding.EncodeToString(accept[:]))
		if rw.Flush() == nil {
			socketOpens.Add(1)
		}
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		_, _ = io.Copy(io.Discard, conn)
	}))
	t.Cleanup(socket.Close)
	wssURL := strings.Replace(socket.URL, "https://", "wss://", 1)
	runtime, err := live.NewBrowserInputRuntime([]live.BrowserInputProject{{
		ProjectID: "project_lma", CredentialVersion: 1, Config: lmeConfig(),
		Transport: lmeTransport("127.0.0.1:1"), BrowserURL: wssURL,
	}})
	if err != nil {
		t.Fatal("local browser runtime configuration rejected")
	}
	stopCase.runtime, revokeCase.runtime = runtime, runtime

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	_ = listener.Close()
	_, port, _ := net.SplitHostPort(address)
	upstream, _ := url.Parse("http://" + address)
	proxy := httputil.NewSingleHostReverseProxy(upstream)
	var revoked atomic.Bool
	var edge *httptest.Server
	edge = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/__test/revoke-input" {
			if r.Method != http.MethodPost || r.Header.Get("Origin") != edge.URL || !revoked.CompareAndSwap(false, true) {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			_, err := revokeCase.registrar.Exec(r.Context(), `SELECT live.revoke_prepared_media($1::uuid,$2::uuid,$3::uuid,$4::text)`, revokeCase.lp.f.tenantA, revokeCase.lp.f.storeA1, revokeCase.input.AuthorizationID, "operator_revoke")
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		r.Header.Set("X-Forwarded-Proto", "https")
		proxy.ServeHTTP(w, r)
	}))
	t.Cleanup(edge.Close)
	idp := newBrowserIDP(t, edge.URL+"/api/auth/callback")
	mustExec(t, stopCase.lp.f.owner, `INSERT INTO identity.external_identities(issuer,subject,principal_id) VALUES($1,'browser-subject',$2)`, idp.server.URL, stopCase.lp.actor)
	t.Cleanup(func() {
		_, _ = stopCase.lp.f.owner.Exec(context.Background(), `DELETE FROM identity.external_identities WHERE issuer=$1 AND subject='browser-subject'`, idp.server.URL)
		for _, actor := range []string{stopCase.lp.actor, revokeCase.lp.actor} {
			_, _ = stopCase.lp.f.owner.Exec(context.Background(), `DELETE FROM identity.session_events WHERE session_id IN (SELECT id FROM identity.sessions WHERE principal_id=$1)`, actor)
		}
	})
	_, _, authority := identityFixture(t)
	provider, err := oidclogin.New(ctx, oidclogin.Config{Issuer: idp.server.URL, ClientID: browserClientID, RedirectURL: idp.redirect, AllowLoopbackForTests: true})
	if err != nil {
		t.Fatal(err)
	}
	service, err := identity.New(authority, observedBrowserProvider{Provider: provider, t: t}, identity.Policy{
		ProviderKey: "browser-input-delivery-signed-mock-v1", SessionTTL: time.Hour,
		OnboardingEnabled: true, Currencies: []string{"TWD", "USD"},
	})
	if err != nil {
		t.Fatal(err)
	}
	bffKey := randomToken()
	private, err := identityhttp.NewHandler(service, bffKey)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/v1/identity/", private)
	mux.Handle("/", httpapi.NewHandler(stopCase.lp.f.runtime, httpapi.Options{SessionStoreList: true, Live: stopCase.planner, BrowserInput: runtime}))
	var signed, badSignature, inputCalls, stopExpiry, revokeExpiry atomic.Int64
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/input/token") {
			inputCalls.Add(1)
			switch r.Header.Get("Idempotency-Key") {
			case "brw05-malformed":
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Backend-Secret", "upstream-secret-sentinel")
				w.Header().Set("Set-Cookie", "upstream=upstream-secret-sentinel")
				_, _ = io.WriteString(w, `{"token":"upstream-secret-sentinel"}`)
				return
			case "brw05-oversized":
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Backend-Secret", "upstream-secret-sentinel")
				_, _ = io.WriteString(w, `{"token":"`+strings.Repeat("x", 8193)+`"}`)
				return
			}
			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, r)
			if recorder.Code == http.StatusOK {
				if !brw05SignedToken(recorder.Body.Bytes(), lmeConfig().APISecret) {
					badSignature.Add(1)
				} else {
					signed.Add(1)
					var receipt struct {
						ExpiresAt int64 `json:"expires_at"`
					}
					if json.Unmarshal(recorder.Body.Bytes(), &receipt) == nil {
						if strings.Contains(r.URL.Path, stopCase.session) {
							stopExpiry.Store(receipt.ExpiresAt)
						} else if strings.Contains(r.URL.Path, revokeCase.session) {
							revokeExpiry.Store(receipt.ExpiresAt)
						}
					}
				}
			}
			for name, values := range recorder.Header() {
				for _, value := range values {
					w.Header().Add(name, value)
				}
			}
			w.WriteHeader(recorder.Code)
			_, _ = w.Write(recorder.Body.Bytes())
			return
		}
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(api.Close)

	serverLog := browserLog(t, filepath.Join(evidence, "next.log"))
	server := exec.CommandContext(ctx, "node", filepath.Join(root, "apps/admin/.next/standalone/apps/admin/server.js"))
	server.Dir = root
	server.Env = browserEnvironment(map[string]string{
		"HOSTNAME": "127.0.0.1", "PORT": port, "NODE_ENV": "production",
		"COMMERCE_IDENTITY_ENABLED": "1", "COMMERCE_IDENTITY_ALLOW_LOOPBACK_TESTS": "1",
		"COMMERCE_PUBLIC_ORIGIN": edge.URL, "COMMERCE_API_ORIGIN": api.URL,
		"COMMERCE_OIDC_ISSUER": idp.server.URL, "COMMERCE_BFF_KEY": bffKey,
		"COMMERCE_ONBOARDING_ENABLED": "1", "COMMERCE_ONBOARDING_CURRENCIES": "TWD,USD",
	})
	server.Stdout, server.Stderr = serverLog, serverLog
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	serverDone := make(chan error, 1)
	go func() { serverDone <- server.Wait() }()
	t.Cleanup(func() {
		_ = server.Process.Kill()
		select {
		case <-serverDone:
		case <-time.After(5 * time.Second):
			t.Error("owned Next process did not stop")
		}
	})
	client := edge.Client()
	client.Timeout = time.Second
	ready := false
	for i := 0; i < 100; i++ {
		response, err := client.Get(edge.URL + "/api/stores")
		if err == nil {
			_ = response.Body.Close()
			ready = response.StatusCode == http.StatusUnauthorized
		}
		if ready {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("HTTPS Next readiness deadline")
		case <-time.After(100 * time.Millisecond):
		}
	}
	if !ready {
		t.Fatalf("HTTPS Next not ready; evidence=%s", evidence)
	}
	runBrowser := func(phase string, h *brwHarness) {
		t.Helper()
		log := browserLog(t, filepath.Join(evidence, phase+".log"))
		browser := exec.CommandContext(ctx, "node", "--test", "--experimental-strip-types", "tests/admin/input-delivery.spec.ts")
		browser.Dir = root
		browser.Env = browserEnvironment(map[string]string{
			"LC_BROWSER_PUBLIC_ORIGIN": edge.URL, "LC_BROWSER_INPUT_PHASE": phase,
			"LC_BROWSER_INPUT_STORE": h.lp.f.storeA1, "LC_BROWSER_INPUT_UNLISTED_STORE": h.lp.f.storeB,
			"LC_BROWSER_INPUT_SESSION": h.session, "LC_BROWSER_INPUT_AUTHORIZATION": h.input.AuthorizationID,
			"LC_BROWSER_INPUT_ATTEMPT": h.specification["attempt_id"].(string),
		})
		browser.Stdout, browser.Stderr = log, log
		if err := browser.Run(); err != nil {
			t.Fatalf("%s HTTPS browser token delivery failed; evidence=%s", phase, evidence)
		}
	}
	runBrowser("stop", stopCase)
	result, err := stopCase.lp.f.owner.Exec(ctx, `UPDATE identity.external_identities SET principal_id=$1 WHERE issuer=$2 AND subject='browser-subject' AND principal_id=$3`, revokeCase.lp.actor, idp.server.URL, stopCase.lp.actor)
	if err != nil {
		t.Fatalf("second signed-login owner fixture remap: %v", err)
	}
	if result.RowsAffected() != 1 {
		t.Fatalf("second signed-login owner fixture remap affected %d rows", result.RowsAffected())
	}
	runBrowser("revoke", revokeCase)
	if signed.Load() < 3 || badSignature.Load() != 0 || socketOpens.Load() < 2 || !revoked.Load() || inputCalls.Load() < 7 {
		t.Fatalf("token-delivery evidence incomplete: signed=%d bad_signature=%d wss=%d revoked=%v input_calls=%d evidence=%s", signed.Load(), badSignature.Load(), socketOpens.Load(), revoked.Load(), inputCalls.Load(), evidence)
	}
	for _, item := range []struct {
		h      *brwHarness
		expiry int64
	}{{stopCase, stopExpiry.Load()}, {revokeCase, revokeExpiry.Load()}} {
		var issued, expiry int64
		var room, publisher string
		attempt := item.h.specification["attempt_id"].(string)
		if err := item.h.lp.f.owner.QueryRow(ctx, `SELECT grant_iat,grant_exp,room_name,publisher_identity FROM live.media_input_custody WHERE attempt_id=$1`, attempt).Scan(&issued, &expiry, &room, &publisher); err != nil || issued < 1 || expiry <= issued || expiry-issued > 60 || expiry != item.expiry || room != "lc_"+strings.ReplaceAll(attempt, "-", "") || !strings.HasPrefix(publisher, "lcp_") {
			t.Fatalf("committed fixed input grant invalid: err=%v", err)
		}
	}
	idp.mu.Lock()
	exchanges := idp.exchanges
	idp.mu.Unlock()
	if exchanges != 2 {
		t.Fatalf("expected two genuine signed IdP logins, got %d", exchanges)
	}
	t.Logf("PASS: two independent signed HTTPS browser logins -> Next -> Go -> PG; token HMAC and WSS handshake verified; evidence=%s", evidence)
}

// Inspect only inside the Go fixture and return a boolean; never log a JWT.
func brw05SignedToken(raw []byte, secret string) bool {
	if len(raw) > 8192 {
		return false
	}
	var body struct {
		AttemptID         string `json:"attempt_id"`
		RoomName          string `json:"room_name"`
		PublisherIdentity string `json:"publisher_identity"`
		URL               string `json:"url"`
		Token             string `json:"token"`
		ExpiresAt         int64  `json:"expires_at"`
	}
	if json.Unmarshal(raw, &body) != nil || body.Token == "" {
		return false
	}
	parts := strings.Split(body.Token, ".")
	if len(parts) != 3 {
		return false
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = io.WriteString(mac, parts[0]+"."+parts[1])
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return false
	}
	data, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}
	var claims struct {
		Subject string `json:"sub"`
		Issued  int64  `json:"iat"`
		Expiry  int64  `json:"exp"`
		Video   struct {
			Room    string   `json:"room"`
			Sources []string `json:"canPublishSources"`
		} `json:"video"`
	}
	return json.Unmarshal(data, &claims) == nil && body.AttemptID != "" && body.RoomName == claims.Video.Room &&
		body.PublisherIdentity == claims.Subject && body.ExpiresAt == claims.Expiry && claims.Issued > 0 &&
		claims.Expiry > claims.Issued && len(claims.Video.Sources) == 2 && claims.Video.Sources[0] == "camera" && claims.Video.Sources[1] == "microphone" &&
		strings.HasPrefix(body.URL, "wss://127.0.0.1:")
}
