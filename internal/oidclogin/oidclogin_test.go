package oidclogin

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	testClientID = "merchant-admin"
	testNonce    = "nonce-with-enough-entropy-1234567890"
	testVerifier = "verifier-abcdefghijklmnopqrstuvwxyz-0123456789-ABCDE"
)

func TestAuthorizationURLAndExchange(t *testing.T) {
	idp := newMockIDP(t)
	provider := idp.provider(t)
	authorizationURL, err := provider.AuthorizationURL("state-123", testNonce, testVerifier)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(authorizationURL)
	if err != nil {
		t.Fatal(err)
	}
	challenge := sha256.Sum256([]byte(testVerifier))
	values := parsed.Query()
	for key, want := range map[string]string{
		"scope": "openid", "state": "state-123", "nonce": testNonce,
		"code_challenge": base64.RawURLEncoding.EncodeToString(challenge[:]), "code_challenge_method": "S256",
	} {
		if got := values.Get(key); got != want {
			t.Fatalf("%s = %q, want %q", key, got, want)
		}
	}
	code := idp.issue(defaultClaims(idp.server.URL), testVerifier, idp.signingKey)
	identity, err := provider.Exchange(context.Background(), code, testNonce, testVerifier)
	if err != nil {
		t.Fatal(err)
	}
	if identity != (Identity{Issuer: idp.server.URL, Subject: "subject-123"}) {
		t.Fatalf("identity = %#v", identity)
	}
}

func TestExchangeRejectsInvalidIDTokens(t *testing.T) {
	idp := newMockIDP(t)
	provider := idp.provider(t)
	wrongKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	tests := []struct {
		name   string
		mutate func(map[string]any)
		key    *rsa.PrivateKey
	}{
		{name: "issuer", mutate: func(claims map[string]any) { claims["iss"] = "https://issuer.invalid" }},
		{name: "audience", mutate: func(claims map[string]any) { claims["aud"] = "another-client" }},
		{name: "signature", mutate: func(map[string]any) {}, key: wrongKey},
		{name: "expiry", mutate: func(claims map[string]any) { claims["exp"] = now.Add(-time.Hour).Unix() }},
		{name: "nonce", mutate: func(claims map[string]any) { claims["nonce"] = "wrong-nonce" }},
		{name: "empty subject", mutate: func(claims map[string]any) { claims["sub"] = "" }},
		{name: "oversized subject", mutate: func(claims map[string]any) { claims["sub"] = strings.Repeat("s", 256) }},
		{name: "multiple audience missing azp", mutate: func(claims map[string]any) { claims["aud"] = []string{testClientID, "other"} }},
		{name: "multiple audience wrong azp", mutate: func(claims map[string]any) {
			claims["aud"] = []string{testClientID, "other"}
			claims["azp"] = "other"
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			claims := defaultClaims(idp.server.URL)
			test.mutate(claims)
			key := test.key
			if key == nil {
				key = idp.signingKey
			}
			code := idp.issue(claims, testVerifier, key)
			if _, err := provider.Exchange(context.Background(), code, testNonce, testVerifier); err == nil {
				t.Fatal("exchange succeeded")
			}
		})
	}
}

func TestExchangeAcceptsMultipleAudienceWithAuthorizedParty(t *testing.T) {
	idp := newMockIDP(t)
	provider := idp.provider(t)
	claims := defaultClaims(idp.server.URL)
	claims["aud"] = []string{testClientID, "other"}
	claims["azp"] = testClientID
	code := idp.issue(claims, testVerifier, idp.signingKey)
	if _, err := provider.Exchange(context.Background(), code, testNonce, testVerifier); err != nil {
		t.Fatal(err)
	}
}

func TestExchangeEnforcesPKCEAndCodeReplay(t *testing.T) {
	idp := newMockIDP(t)
	provider := idp.provider(t)
	code := idp.issue(defaultClaims(idp.server.URL), testVerifier, idp.signingKey)
	wrongVerifier := "wrong-verifier-abcdefghijklmnopqrstuvwxyz-0123456789-ABCDE"
	if _, err := provider.Exchange(context.Background(), code, testNonce, wrongVerifier); err == nil || err.Error() != "oidc token exchange failed" {
		t.Fatalf("wrong verifier error = %v", err)
	}
	if calls := idp.tokenCallCount(); calls != 1 {
		t.Fatalf("wrong verifier token calls = %d, want 1", calls)
	}
	replayCode := idp.issue(defaultClaims(idp.server.URL), testVerifier, idp.signingKey)
	if _, err := provider.Exchange(context.Background(), replayCode, testNonce, testVerifier); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Exchange(context.Background(), replayCode, testNonce, testVerifier); err == nil || err.Error() != "oidc token exchange failed" {
		t.Fatalf("replay error = %v", err)
	}
}

func TestNewRejectsUnsafeConfiguration(t *testing.T) {
	tests := []Config{
		{Issuer: "http://issuer.example", ClientID: testClientID, RedirectURL: "https://app.example/callback"},
		{Issuer: "https://issuer.example", ClientID: testClientID, RedirectURL: "http://app.example/callback"},
		{Issuer: "http://issuer.example", ClientID: testClientID, RedirectURL: "http://app.example/callback", AllowLoopbackForTests: true},
	}
	for _, config := range tests {
		if _, err := New(context.Background(), config); err == nil {
			t.Fatalf("unsafe config accepted: %#v", config)
		}
	}
}

func TestNewRejectsIssuerMismatch(t *testing.T) {
	idp := newMockIDP(t)
	idp.discoveryIssuer = "https://issuer.invalid"
	if _, err := New(context.Background(), idp.config()); err == nil || err.Error() != "oidc discovery failed" {
		t.Fatalf("issuer mismatch error = %v", err)
	}
}

func TestNetworkRedirectsFailClosed(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, http.StatusOK, map[string]any{})
	}))
	t.Cleanup(target.Close)
	discoveryRedirect := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, target.URL, http.StatusFound)
	}))
	t.Cleanup(discoveryRedirect.Close)
	_, err := New(context.Background(), Config{
		Issuer: discoveryRedirect.URL, ClientID: testClientID,
		RedirectURL: discoveryRedirect.URL + "/callback", AllowLoopbackForTests: true,
	})
	if err == nil || err.Error() != "oidc discovery failed" {
		t.Fatalf("discovery redirect error = %v", err)
	}

	idp := newMockIDP(t)
	idp.tokenRedirect = target.URL
	provider := idp.provider(t)
	code := idp.issue(defaultClaims(idp.server.URL), testVerifier, idp.signingKey)
	if _, err := provider.Exchange(context.Background(), code, testNonce, testVerifier); err == nil || err.Error() != "oidc token exchange failed" {
		t.Fatalf("token redirect error = %v", err)
	}
}

type mockIDP struct {
	t               *testing.T
	server          *httptest.Server
	signingKey      *rsa.PrivateKey
	discoveryIssuer string
	tokenRedirect   string

	mu         sync.Mutex
	next       int
	tokenCalls int
	codes      map[string]*mockCode
}

type mockCode struct {
	claims    map[string]any
	challenge string
	key       *rsa.PrivateKey
	used      bool
}

func newMockIDP(t *testing.T) *mockIDP {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	idp := &mockIDP{t: t, signingKey: key, codes: make(map[string]*mockCode)}
	idp.server = httptest.NewServer(http.HandlerFunc(idp.serveHTTP))
	t.Cleanup(idp.server.Close)
	return idp
}

func (idp *mockIDP) config() Config {
	return Config{
		Issuer: idp.server.URL, ClientID: testClientID, ClientSecret: "test-secret",
		RedirectURL: idp.server.URL + "/callback", AllowLoopbackForTests: true,
	}
}

func (idp *mockIDP) provider(t *testing.T) *Provider {
	t.Helper()
	provider, err := New(context.Background(), idp.config())
	if err != nil {
		t.Fatal(err)
	}
	return provider
}

func (idp *mockIDP) issue(claims map[string]any, verifier string, key *rsa.PrivateKey) string {
	idp.mu.Lock()
	defer idp.mu.Unlock()
	idp.next++
	code := fmt.Sprintf("code-%d", idp.next)
	challenge := sha256.Sum256([]byte(verifier))
	idp.codes[code] = &mockCode{claims: claims, challenge: base64.RawURLEncoding.EncodeToString(challenge[:]), key: key}
	return code
}

func (idp *mockIDP) serveHTTP(writer http.ResponseWriter, request *http.Request) {
	switch request.URL.Path {
	case "/.well-known/openid-configuration":
		issuer := idp.server.URL
		if idp.discoveryIssuer != "" {
			issuer = idp.discoveryIssuer
		}
		writeJSON(writer, http.StatusOK, map[string]any{
			"issuer": issuer, "authorization_endpoint": idp.server.URL + "/authorize",
			"token_endpoint": idp.server.URL + "/token", "jwks_uri": idp.server.URL + "/jwks",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	case "/jwks":
		exponent := big.NewInt(int64(idp.signingKey.PublicKey.E)).Bytes()
		writeJSON(writer, http.StatusOK, map[string]any{"keys": []map[string]any{{
			"kty": "RSA", "kid": "test-key", "use": "sig", "alg": "RS256",
			"n": base64.RawURLEncoding.EncodeToString(idp.signingKey.PublicKey.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(exponent),
		}}})
	case "/token":
		if idp.tokenRedirect != "" {
			http.Redirect(writer, request, idp.tokenRedirect, http.StatusTemporaryRedirect)
			return
		}
		idp.serveToken(writer, request)
	default:
		http.NotFound(writer, request)
	}
}

func (idp *mockIDP) serveToken(writer http.ResponseWriter, request *http.Request) {
	idp.mu.Lock()
	idp.tokenCalls++
	idp.mu.Unlock()
	if request.Method != http.MethodPost || request.ParseForm() != nil {
		writeJSON(writer, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return
	}
	codeValue := request.Form.Get("code")
	idp.mu.Lock()
	code, ok := idp.codes[codeValue]
	if ok && !code.used {
		code.used = true
	} else {
		ok = false
	}
	idp.mu.Unlock()
	challenge := sha256.Sum256([]byte(request.Form.Get("code_verifier")))
	if !ok || base64.RawURLEncoding.EncodeToString(challenge[:]) != code.challenge {
		writeJSON(writer, http.StatusBadRequest, map[string]string{"error": "invalid_grant"})
		return
	}
	token, err := signJWT(code.claims, code.key)
	if err != nil {
		idp.t.Error(err)
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": "server_error"})
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"access_token": "opaque-test-access-token", "token_type": "Bearer", "expires_in": 300, "id_token": token,
	})
}

func (idp *mockIDP) tokenCallCount() int {
	idp.mu.Lock()
	defer idp.mu.Unlock()
	return idp.tokenCalls
}

func defaultClaims(issuer string) map[string]any {
	now := time.Now()
	return map[string]any{
		"iss": issuer, "sub": "subject-123", "aud": testClientID,
		"exp": now.Add(time.Minute).Unix(), "iat": now.Add(-time.Second).Unix(), "nonce": testNonce,
	}
}

func signJWT(claims map[string]any, key *rsa.PrivateKey) (string, error) {
	header, err := json.Marshal(map[string]string{"alg": "RS256", "kid": "test-key", "typ": "JWT"})
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}
