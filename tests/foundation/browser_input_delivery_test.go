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
	var originGood, originBad, originBadDenied atomic.Int64
	isOriginProbe := func(key string) bool { return key == "brw05-origin-stop" || key == "brw05-origin-revoke" }
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
		if strings.HasSuffix(r.URL.Path, "/input/token") && isOriginProbe(r.Header.Get("Idempotency-Key")) {
			// Record only the Origin and status, never a cookie or token. The
			// positive control and forged request use the same live grant/key.
			origin := r.Header.Get("Origin")
			recorder := httptest.NewRecorder()
			proxy.ServeHTTP(recorder, r)
			if origin == edge.URL && recorder.Code == http.StatusOK {
				originGood.Add(1)
			}
			if origin == "https://attacker.invalid" {
				originBad.Add(1)
				if recorder.Code == http.StatusForbidden {
					originBadDenied.Add(1)
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
	var signed, badSignature, inputCalls, originProbeCalls, stopExpiry, revokeExpiry atomic.Int64
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Synthetic upstream faults exercise the BFF public DTO/size boundary;
		// real session IDs below still reach the actual Go/PG read implementation.
		if r.Method == http.MethodGet && (strings.HasSuffix(r.URL.Path, "/input") || strings.HasSuffix(r.URL.Path, "/input/prepared")) {
			malformed := strings.Contains(r.URL.Path, "/aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa/")
			oversized := strings.Contains(r.URL.Path, "/bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb/")
			if malformed || oversized {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Backend-Secret", "upstream-secret-sentinel")
				w.Header().Set("Set-Cookie", "upstream=upstream-secret-sentinel")
				if malformed {
					_, _ = io.WriteString(w, `{"project_id":"upstream-secret-sentinel"}`)
				} else {
					_, _ = io.WriteString(w, `{"project_id":"`+strings.Repeat("x", 8193)+`"}`)
				}
				return
			}
		}
		if strings.HasSuffix(r.URL.Path, "/input/token") {
			inputCalls.Add(1)
			if isOriginProbe(r.Header.Get("Idempotency-Key")) {
				originProbeCalls.Add(1)
			}
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
	if signed.Load() < 5 || badSignature.Load() != 0 || socketOpens.Load() < 2 || !revoked.Load() || inputCalls.Load() < 9 ||
		originGood.Load() != 2 || originBad.Load() != 2 || originBadDenied.Load() != 2 || originProbeCalls.Load() != 2 {
		t.Fatalf("token-delivery evidence incomplete: signed=%d bad_signature=%d wss=%d revoked=%v input_calls=%d origin_good=%d origin_bad=%d origin_denied=%d origin_upstream=%d evidence=%s",
			signed.Load(), badSignature.Load(), socketOpens.Load(), revoked.Load(), inputCalls.Load(), originGood.Load(), originBad.Load(), originBadDenied.Load(), originProbeCalls.Load(), evidence)
	}
	for _, item := range []struct {
		h      *brwHarness
		expiry int64
		key    string
	}{{stopCase, stopExpiry.Load(), "brw05-origin-stop"}, {revokeCase, revokeExpiry.Load(), "brw05-origin-revoke"}} {
		var issued, expiry int64
		var room, publisher string
		attempt := item.h.specification["attempt_id"].(string)
		if err := item.h.lp.f.owner.QueryRow(ctx, `SELECT grant_iat,grant_exp,room_name,publisher_identity FROM live.media_input_custody WHERE attempt_id=$1`, attempt).Scan(&issued, &expiry, &room, &publisher); err != nil || issued < 1 || expiry <= issued || expiry-issued > 60 || expiry != item.expiry || room != "lc_"+strings.ReplaceAll(attempt, "-", "") || !strings.HasPrefix(publisher, "lcp_") {
			t.Fatalf("committed fixed input grant invalid: err=%v", err)
		}
		var receiptAttempt string
		if err := item.h.lp.f.owner.QueryRow(ctx, `SELECT response->>'attempt_id' FROM ops.command_results
			WHERE tenant_id=$1 AND store_id=$2 AND operation='live.media.input.reserve' AND idempotency_key=$3`,
			item.h.lp.f.tenantA, item.h.lp.f.storeA1, item.key).Scan(&receiptAttempt); err != nil || receiptAttempt != attempt {
			t.Fatalf("committed phase-specific input receipt mismatch: err=%v", err)
		}
	}
	var stopped, stopClosed, stopRevoked bool
	var stopReason string
	if err := stopCase.lp.f.owner.QueryRow(ctx, `SELECT x.stop_requested_at IS NOT NULL,
		i.admission_closed_at IS NOT NULL,i.close_reason,
		EXISTS(SELECT 1 FROM live.media_authorization_revocations r WHERE r.authorization_id=i.authorization_id)
		FROM live.media_execution_state x JOIN live.media_input_custody i ON i.attempt_id=x.attempt_id
		WHERE x.attempt_id=$1`, stopCase.specification["attempt_id"]).Scan(&stopped, &stopClosed, &stopReason, &stopRevoked); err != nil || !stopped || !stopClosed || stopReason != "merchant_stop" || stopRevoked {
		t.Fatalf("Stop did not persist distinct input closure: err=%v stopped=%v closed=%v reason=%q revoked=%v", err, stopped, stopClosed, stopReason, stopRevoked)
	}
	// No worker or Stop ran for this attempt. Revocation records authority loss;
	// it does not create an execution projection or discharge the issued grant.
	var revokeRecorded, executionAbsent, liabilityRetained, originalJobRetained bool
	if err := revokeCase.lp.f.owner.QueryRow(ctx, `SELECT
		EXISTS(SELECT 1 FROM live.media_authorization_revocations r
		 WHERE r.authorization_id=i.authorization_id AND r.tenant_id=i.tenant_id
		 AND r.store_id=i.store_id AND r.reason_code='operator_revoke'),
		NOT EXISTS(SELECT 1 FROM live.media_execution_state x WHERE x.attempt_id=i.attempt_id),
		i.state='RESERVED' AND i.grant_iat IS NOT NULL AND i.admission_closed_at IS NULL,
		o.id=i.operation_id AND o.state='READY' AND o.generation=0
		 AND live.media_native_job(j.id,o.id) AND j.state='available'
		 AND j.attempt=0 AND j.finalized_at IS NULL
		FROM live.media_input_custody i JOIN live.media_attempts a ON a.id=i.attempt_id
		JOIN integration.operations o ON o.id=a.start_operation_id
		JOIN river_media.river_job j ON j.id=o.job_id
		WHERE i.attempt_id=$1 AND i.authorization_id=$2`, revokeCase.specification["attempt_id"], revokeCase.input.AuthorizationID).
		Scan(&revokeRecorded, &executionAbsent, &liabilityRetained, &originalJobRetained); err != nil || !revokeRecorded || !executionAbsent || !liabilityRetained || !originalJobRetained {
		t.Fatalf("prewire revoke durable facts invalid: err=%v revoked=%v execution_absent=%v liability_retained=%v original_job_retained=%v", err, revokeRecorded, executionAbsent, liabilityRetained, originalJobRetained)
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
