// Purpose: OC-15 recent re-authentication HTTP gates (contracts/merchant-password-auth-v1.md §16): the re-auth routes of
// internal/identityhttp (password start/complete, OIDC start, the re-auth branch of POST /v1/identity/login/complete, which
// never returns a session token), their error codes through the password writer (reauth_failed 403, invalid_code 422,
// mail_unavailable 503, throttled 429), the 403 reauth_required / owner_role_disabled transport of each guarded domain
// (staff routes; orders CSV, finance CSV, customer erasure and bank-transfer decisions through internal/httpapi), session
// A/B isolation and the principal + session-hash throttle keys over a real database, and the httperror code table
// (reauth_required, owner_role_disabled, bank_transfer_closed).
// Depends on: internal/identityhttp (NewHandler, NewPasswordHandler, NewStaffHandler), internal/httpapi (NewHandler),
// internal/httperror (Middleware, Write), internal/identity (Passwords, Staff, HashPassword, error values), internal/mail,
// internal/platform (OpenIdentityPool, OpenPool), livecommerce/migrations; the disposable lc_foundation_test database
// (LC_TEST_DATABASE_ALLOWED=1, LC_TEST_DATABASE_URL) for the RH1x tests, which SKIP (NOT_RUN) without it.
// Used by: go test ./tests/identityhttp (CI shard:unit via scripts/dev/test-local.sh).
package reauthhttptests

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/billing"
	"livecommerce/internal/billing/billingtest"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/httperror"
	"livecommerce/internal/identity"
	"livecommerce/internal/identityhttp"
	"livecommerce/internal/mail"
	"livecommerce/internal/platform"
	"livecommerce/migrations"
)

// ---------------------------------------------------------------------------------------------------------------
// Transport (no database): fakes behind the identityhttp seams.
// ---------------------------------------------------------------------------------------------------------------

func secret() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func token() string { return secret() }

var bffKey = secret()

// fakePasswords is *identity.Passwords as the password handler sees it, plus the two re-auth steps.
type fakePasswords struct {
	mu                       sync.Mutex
	calls                    int
	bearer, password, locale string
	code                     string
	ip                       netip.Addr
	startErr, completeErr    error
	expires, authenticatedAt time.Time
}

func (f *fakePasswords) Signup(context.Context, netip.Addr, string, string, string) (identity.Challenge, error) {
	return identity.Challenge{}, identity.ErrUnavailable
}
func (f *fakePasswords) Login(context.Context, netip.Addr, string, string, string) (identity.Challenge, error) {
	return identity.Challenge{}, identity.ErrUnavailable
}
func (f *fakePasswords) Reset(context.Context, netip.Addr, string, string) (identity.Challenge, error) {
	return identity.Challenge{}, identity.ErrUnavailable
}
func (f *fakePasswords) Complete(context.Context, netip.Addr, string, string, string, string) (identity.Session, error) {
	return identity.Session{}, identity.ErrUnavailable
}
func (f *fakePasswords) StartReauth(_ context.Context, ip netip.Addr, bearer, password, locale string) (time.Time, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.ip, f.bearer, f.password, f.locale = ip, bearer, password, locale
	return f.expires, f.startErr
}
func (f *fakePasswords) CompleteReauth(_ context.Context, ip netip.Addr, bearer, code string) (time.Time, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.ip, f.bearer, f.code = ip, bearer, code
	return f.authenticatedAt, f.completeErr
}

// fakeOIDC is *identity.Service as the OIDC handler sees it, plus the re-auth start and the two-kind callback.
type fakeOIDC struct {
	mu            sync.Mutex
	calls         int
	bearer        string
	completeCalls int
	result        identity.LoginResult
	err           error
	flow          identity.Flow
}

func (f *fakeOIDC) Start(context.Context) (identity.Flow, error) { return f.flow, nil }
func (f *fakeOIDC) Complete(context.Context, string, string, string) (identity.Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.completeCalls++
	// The pre-OC-15 path: a re-auth flow is not a login flow, so the old Complete can only refuse it.
	return identity.Session{}, identity.ErrUnauthorized
}
func (f *fakeOIDC) CreateInitialStore(context.Context, string, string, identity.StoreRequest) (identity.Store, error) {
	return identity.Store{}, identity.ErrUnavailable
}
func (f *fakeOIDC) Logout(context.Context, string) error { return nil }
func (f *fakeOIDC) StartOIDCReauth(_ context.Context, bearer string) (identity.Flow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.bearer = bearer
	return f.flow, f.err
}
func (f *fakeOIDC) CompleteLogin(context.Context, string, string, string) (identity.LoginResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return f.result, f.err
}

type reply struct {
	status int
	header http.Header
	body   map[string]any
	raw    string
}

func (r reply) code() string    { s, _ := r.body["code"].(string); return s }
func (r reply) message() string { s, _ := r.body["message"].(string); return s }

// send issues one BFF-shaped request (BFF key, JSON, one client ip) with an optional bearer.
func send(t *testing.T, h http.Handler, path, bearer, body string) reply {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("X-Commerce-BFF-Key", bffKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Commerce-Client-IP", "198.51.100.7")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	out := reply{status: rec.Code, header: rec.Header(), raw: rec.Body.String()}
	_ = json.Unmarshal(rec.Body.Bytes(), &out.body)
	return out
}

func want(t *testing.T, what string, got reply, status int, code string) {
	t.Helper()
	if got.status != status || got.code() != code {
		t.Errorf("%s: HTTP %d code %q, want %d %q (body %s)", what, got.status, got.code(), status, code, got.raw)
	}
}

const (
	pathPwStart     = "/v1/identity/password/reauth/start"
	pathPwComplete  = "/v1/identity/password/reauth/complete"
	pathOIDCStart   = "/v1/identity/login/reauth/start"
	pathOIDCDone    = "/v1/identity/login/complete"
	reauthFailedMsg = "Your identity could not be confirmed."
)

// RH01: password re-auth start. The session comes only from the bearer (no bearer = 401 before any work; an email in the
// body is an unknown key = 400); a wrong password is 403 reauth_failed with its fixed message (never 401: the BFF keeps
// the session); mail down is 503 mail_unavailable; throttled is 429 with Retry-After; success is 200 {expires_at}.
func TestReauthRH01PasswordStartTransport(t *testing.T) {
	f := &fakePasswords{expires: time.Date(2026, 10, 27, 1, 2, 3, 0, time.UTC)}
	h, err := identityhttp.NewPasswordHandler(f, bffKey)
	if err != nil {
		t.Fatal(err)
	}
	bearer := token()

	want(t, "no bearer", send(t, h, pathPwStart, "", `{"password":"pw","locale":"en"}`), http.StatusUnauthorized, "unauthorized")
	want(t, "email in body", send(t, h, pathPwStart, bearer, `{"email":"a@example.test","password":"pw","locale":"en"}`), http.StatusBadRequest, "invalid_json")
	if f.calls != 0 {
		t.Errorf("service called %d times before the bearer and body checks passed", f.calls)
	}

	f.startErr = identity.ErrReauthFailed
	got := send(t, h, pathPwStart, bearer, `{"password":"wrong horse","locale":"zh-TW"}`)
	want(t, "wrong password", got, http.StatusForbidden, "reauth_failed")
	if got.message() != reauthFailedMsg {
		t.Errorf("reauth_failed message %q, want %q", got.message(), reauthFailedMsg)
	}
	if f.bearer != bearer || f.password != "wrong horse" || f.locale != "zh-TW" || f.ip != netip.MustParseAddr("198.51.100.7") {
		t.Errorf("service got bearer=%v password=%q locale=%q ip=%v", f.bearer == bearer, f.password, f.locale, f.ip)
	}

	f.startErr = identity.ErrMailUnavailable
	want(t, "mail down", send(t, h, pathPwStart, bearer, `{"password":"pw","locale":"en"}`), http.StatusServiceUnavailable, "mail_unavailable")
	f.startErr = identity.ThrottleError{RetryAfter: 90 * time.Second}
	got = send(t, h, pathPwStart, bearer, `{"password":"pw","locale":"en"}`)
	want(t, "throttled", got, http.StatusTooManyRequests, "throttled")
	if got.header.Get("Retry-After") != "90" {
		t.Errorf("Retry-After %q, want 90", got.header.Get("Retry-After"))
	}
	f.startErr = identity.ErrUnauthorized
	want(t, "dead session", send(t, h, pathPwStart, bearer, `{"password":"pw","locale":"en"}`), http.StatusUnauthorized, "unauthorized")

	f.startErr = nil
	got = send(t, h, pathPwStart, bearer, `{"password":"pw","locale":"en"}`)
	if got.status != http.StatusOK || got.body["expires_at"] != "2026-10-27T01:02:03Z" || len(got.body) != 1 {
		t.Errorf("start success: HTTP %d body %s, want 200 {expires_at}", got.status, got.raw)
	}
}

// RH02: password re-auth complete. A code that is not six digits is 422 invalid_code before any work; a wrong code is
// 422 invalid_code (not the login 401); success is 200 {authenticated_at} and never a token.
func TestReauthRH02PasswordCompleteTransport(t *testing.T) {
	f := &fakePasswords{authenticatedAt: time.Date(2026, 10, 27, 4, 5, 6, 0, time.UTC)}
	h, err := identityhttp.NewPasswordHandler(f, bffKey)
	if err != nil {
		t.Fatal(err)
	}
	bearer := token()
	want(t, "no bearer", send(t, h, pathPwComplete, "", `{"code":"123456"}`), http.StatusUnauthorized, "unauthorized")
	want(t, "malformed code", send(t, h, pathPwComplete, bearer, `{"code":"12ab56"}`), http.StatusUnprocessableEntity, "invalid_code")
	if f.calls != 0 {
		t.Errorf("service called %d times for a request refused at the transport", f.calls)
	}
	f.completeErr = identity.ErrInvalidCode
	want(t, "wrong code", send(t, h, pathPwComplete, bearer, `{"code":"123456"}`), http.StatusUnprocessableEntity, "invalid_code")
	if f.bearer != bearer || f.code != "123456" {
		t.Errorf("service got bearer=%v code=%q", f.bearer == bearer, f.code)
	}
	f.completeErr = identity.ThrottleError{RetryAfter: time.Second}
	want(t, "throttled", send(t, h, pathPwComplete, bearer, `{"code":"123456"}`), http.StatusTooManyRequests, "throttled")
	f.completeErr = nil
	got := send(t, h, pathPwComplete, bearer, `{"code":"123456"}`)
	if got.status != http.StatusOK || got.body["authenticated_at"] != "2026-10-27T04:05:06Z" || len(got.body) != 1 {
		t.Errorf("complete success: HTTP %d body %s, want 200 {authenticated_at} only", got.status, got.raw)
	}
}

// RH03: OIDC re-auth. The start route is session-authenticated (bearer to the service, 401 without it). The shared
// callback POST /v1/identity/login/complete answers a re-auth flow with {reauth:true, authenticated_at} and no token, a
// refused re-auth (other account, stale auth_time) with 403 reauth_failed, and still answers a login flow with a token.
func TestReauthRH03OIDCTransport(t *testing.T) {
	f := &fakeOIDC{flow: identity.Flow{URL: "https://idp.example.test/authorize?prompt=login", Binding: "b", ExpiresAt: time.Date(2026, 10, 27, 0, 10, 0, 0, time.UTC)}}
	h, err := identityhttp.NewHandler(f, bffKey)
	if err != nil {
		t.Fatal(err)
	}
	bearer := token()
	want(t, "start without bearer", send(t, h, pathOIDCStart, "", `{}`), http.StatusUnauthorized, "unauthorized")
	got := send(t, h, pathOIDCStart, bearer, `{}`)
	if got.status != http.StatusOK || got.body["authorization_url"] != f.flow.URL || got.body["binding"] != "b" || f.bearer != bearer {
		t.Errorf("oidc re-auth start: HTTP %d body %s bearer passed=%v", got.status, got.raw, f.bearer == bearer)
	}

	cb := `{"state":"s","binding":"b","code":"c"}`
	f.result = identity.LoginResult{Reauth: true, AuthenticatedAt: time.Date(2026, 10, 27, 7, 8, 9, 0, time.UTC)}
	got = send(t, h, pathOIDCDone, "", cb)
	if got.status != http.StatusOK || got.body["reauth"] != true || got.body["authenticated_at"] != "2026-10-27T07:08:09Z" {
		t.Errorf("re-auth callback: HTTP %d body %s, want 200 {reauth:true, authenticated_at}", got.status, got.raw)
	}
	if _, ok := got.body["token"]; ok {
		t.Errorf("re-auth callback returned a session token: %s", got.raw)
	}
	if f.completeCalls != 0 {
		t.Errorf("login/complete used the login-only Complete (%d calls), want CompleteLogin", f.completeCalls)
	}

	f.result, f.err = identity.LoginResult{}, identity.ErrReauthFailed
	got = send(t, h, pathOIDCDone, "", cb)
	want(t, "refused re-auth callback", got, http.StatusForbidden, "reauth_failed")
	if got.message() != reauthFailedMsg {
		t.Errorf("reauth_failed message %q, want %q", got.message(), reauthFailedMsg)
	}

	f.err = nil
	f.result = identity.LoginResult{Session: identity.Session{Token: "login-token", ExpiresAt: time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC)}}
	got = send(t, h, pathOIDCDone, "", cb)
	if got.status != http.StatusOK || got.body["token"] != "login-token" || got.body["reauth"] != nil {
		t.Errorf("login callback: HTTP %d body %s, want 200 {token, expires_at}", got.status, got.raw)
	}
}

// RH04: the shared httperror code table carries the re-auth codes and the OC-16 bank_transfer_closed refusal as public
// codes (an unknown code is rewritten to "internal", which is what the BFF and UI must never see here).
func TestReauthRH04HTTPErrorCodes(t *testing.T) {
	for _, c := range []struct {
		status int
		code   string
	}{
		{http.StatusForbidden, "reauth_required"},
		{http.StatusForbidden, "owner_role_disabled"},
		{http.StatusUnprocessableEntity, "bank_transfer_closed"},
	} {
		rec := httptest.NewRecorder()
		httperror.Write(rec, c.status, c.code)
		var env httperror.Envelope
		if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
			t.Fatal(err)
		}
		if rec.Code != c.status || env.Code != c.code || env.Message == "" || env.Retryable {
			t.Errorf("%s: HTTP %d code %q message %q retryable %v", c.code, rec.Code, env.Code, env.Message, env.Retryable)
		}
	}
}

// ---------------------------------------------------------------------------------------------------------------
// Real PostgreSQL (lc_foundation_test).
// ---------------------------------------------------------------------------------------------------------------

type env struct {
	owner   *pgxpool.Pool // migration owner: fixtures and assertions only
	id      *pgxpool.Pool // a commerce_identity login (platform.OpenIdentityPool), as the identity service uses
	runtime *pgxpool.Pool // a commerce_runtime login (platform.OpenPool), as httpapi uses
}

var (
	envOnce sync.Once
	envVal  *env
	envSkip string
	envErr  error
)

func fixture(t *testing.T) *env {
	t.Helper()
	envOnce.Do(func() { envVal, envSkip, envErr = newEnv() })
	if envSkip != "" {
		t.Skip(envSkip)
	}
	if envErr != nil {
		t.Fatal(envErr)
	}
	return envVal
}

func newEnv() (*env, string, error) {
	if os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" || os.Getenv("LC_TEST_DATABASE_URL") == "" {
		return nil, "LC_TEST_DATABASE_ALLOWED=1 and LC_TEST_DATABASE_URL are required; OC-15 real-PG gates NOT_RUN", nil
	}
	cfg, err := pgxpool.ParseConfig(os.Getenv("LC_TEST_DATABASE_URL"))
	if err != nil {
		return nil, "", errors.New("LC_TEST_DATABASE_URL is invalid")
	}
	if cfg.ConnConfig.Host != "127.0.0.1" || cfg.ConnConfig.Database != "lc_foundation_test" {
		return nil, "", errors.New("refusing database outside 127.0.0.1/lc_foundation_test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	owner, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, "", err
	}
	if err := migrations.Apply(ctx, owner); err != nil {
		return nil, "", fmt.Errorf("apply migrations: %w", err)
	}
	login := func(group string) (string, error) {
		var raw [12]byte
		_, _ = rand.Read(raw[:])
		role := "oc15_" + hex.EncodeToString(raw[:])
		pass := hex.EncodeToString(raw[:]) + hex.EncodeToString(raw[:])
		if _, err := owner.Exec(ctx, `CREATE ROLE `+pgx.Identifier{role}.Sanitize()+` LOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE IN ROLE `+group+` PASSWORD '`+pass+`'`); err != nil {
			return "", err
		}
		u, _ := url.Parse(os.Getenv("LC_TEST_DATABASE_URL"))
		u.User = url.UserPassword(role, pass)
		return u.String(), nil
	}
	idURL, err := login("commerce_identity")
	if err != nil {
		return nil, "", err
	}
	rtURL, err := login("commerce_runtime")
	if err != nil {
		return nil, "", err
	}
	id, err := platform.OpenIdentityPool(ctx, idURL)
	if err != nil {
		return nil, "", err
	}
	rt, err := platform.OpenPool(ctx, rtURL)
	if err != nil {
		return nil, "", err
	}
	return &env{owner: owner, id: id, runtime: rt}, "", nil
}

func hash(s string) []byte { h := sha256.Sum256([]byte(s)); return h[:] }

func rnd32() []byte { b := make([]byte, 32); _, _ = rand.Read(b); return b }

func uuid() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

var seq atomic.Int64

func email(prefix string) string {
	return fmt.Sprintf("%s.%d.%d@example.test", prefix, os.Getpid(), seq.Add(1))
}

func mustExec(t *testing.T, p *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := p.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func count(t *testing.T, p *pgxpool.Pool, sql string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := p.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return n
}

func principal(t *testing.T, e *env) string {
	t.Helper()
	p := uuid()
	mustExec(t, e.owner, `INSERT INTO identity.principals(id) VALUES ($1::uuid)`, p)
	return p
}

// session inserts a live merchant session with its session.issued event and returns the bearer token.
func session(t *testing.T, e *env, p string) string {
	t.Helper()
	tok, id := token(), uuid()
	mustExec(t, e.owner, `INSERT INTO identity.sessions(id,token_hash,principal_id,audience,expires_at) VALUES ($1::uuid,$2,$3::uuid,'merchant',clock_timestamp()+interval '1 hour')`, id, hash(tok), p)
	mustExec(t, e.owner, `INSERT INTO identity.session_events(principal_id,session_id,action) VALUES ($1::uuid,$2::uuid,'session.issued')`, p, id)
	return tok
}

func setAuthAge(t *testing.T, e *env, tok, age string) {
	t.Helper()
	mustExec(t, e.owner, `UPDATE identity.sessions SET authenticated_at = clock_timestamp() - $2::interval WHERE token_hash=$1`, hash(tok), age)
}

func authAt(t *testing.T, e *env, tok string) time.Time {
	t.Helper()
	var at time.Time
	if err := e.owner.QueryRow(context.Background(), `SELECT authenticated_at FROM identity.sessions WHERE token_hash=$1`, hash(tok)).Scan(&at); err != nil {
		t.Fatalf("authenticated_at: %v", err)
	}
	return at
}

// live asserts the session was not logged out by a 403.
func live(t *testing.T, e *env, what, tok string) {
	t.Helper()
	if n := count(t, e.owner, `SELECT count(*) FROM identity.sessions WHERE token_hash=$1 AND revoked_at IS NULL AND expires_at>clock_timestamp()`, hash(tok)); n != 1 {
		t.Errorf("%s: the session was invalidated (live rows %d); a 403 must not log out", what, n)
	}
}

type mailer struct {
	mu   sync.Mutex
	msgs []mail.Message
}

func (m *mailer) Send(_ context.Context, msg mail.Message) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.msgs = append(m.msgs, msg)
	return "250 queued", nil
}

func (m *mailer) codes() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []string{}
	for _, msg := range m.msgs {
		out = append(out, digits6.FindString(msg.Text))
	}
	return out
}

var digits6 = regexp.MustCompile(`\b[0-9]{6}\b`)

const phrase = "correct horse battery 1"

// passwordPrincipal is a principal with a verified password credential.
func passwordPrincipal(t *testing.T, e *env) string {
	t.Helper()
	phc, err := identity.HashPassword(phrase)
	if err != nil {
		t.Fatal(err)
	}
	p := principal(t, e)
	mustExec(t, e.owner, `INSERT INTO identity.password_credentials(principal_id,email,password_hash,email_verified_at) VALUES ($1::uuid,$2,$3,clock_timestamp())`, p, email("oc15"), phc)
	return p
}

func passwordHandler(t *testing.T, e *env, m identity.Mailer, pepper []byte) http.Handler {
	t.Helper()
	p, err := identity.NewPasswords(e.id, m, identity.PasswordPolicy{Pepper: pepper, SessionTTL: time.Hour, BreachCheck: "off", AllowLoopback: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close(context.Background()) })
	h, err := identityhttp.NewPasswordHandler(p, bffKey)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// RH10: over HTTP and a real database, a code mailed to session A cannot refresh session B (422 invalid_code, neither
// session moves), a wrong password is 403 reauth_failed and keeps the session live, and A's own code refreshes A only.
func TestReauthRH10SessionBindingOverHTTP(t *testing.T) {
	e := fixture(t)
	p := passwordPrincipal(t, e)
	tokA, tokB := session(t, e, p), session(t, e, p)
	setAuthAge(t, e, tokA, "2 hours")
	setAuthAge(t, e, tokB, "2 hours")
	beforeA, beforeB := authAt(t, e, tokA), authAt(t, e, tokB)
	m := &mailer{}
	h := passwordHandler(t, e, m, rnd32())

	got := send(t, h, pathPwStart, tokA, `{"password":"wrong horse battery 9","locale":"en"}`)
	want(t, "wrong password", got, http.StatusForbidden, "reauth_failed")
	if got.message() != reauthFailedMsg {
		t.Errorf("reauth_failed message %q, want %q", got.message(), reauthFailedMsg)
	}
	live(t, e, "after reauth_failed", tokA)

	got = send(t, h, pathPwStart, tokA, `{"password":"`+phrase+`","locale":"en"}`)
	if got.status != http.StatusOK || got.body["expires_at"] == nil {
		t.Fatalf("start on A: HTTP %d body %s", got.status, got.raw)
	}
	codes := m.codes()
	if len(codes) != 1 || codes[0] == "" {
		t.Fatalf("re-auth mails %v, want one code", codes)
	}
	want(t, "A's code on session B", send(t, h, pathPwComplete, tokB, `{"code":"`+codes[0]+`"}`), http.StatusUnprocessableEntity, "invalid_code")
	if a, b := authAt(t, e, tokA), authAt(t, e, tokB); !a.Equal(beforeA) || !b.Equal(beforeB) {
		t.Errorf("a refused cross-session code moved authenticated_at: A %v -> %v, B %v -> %v", beforeA, a, beforeB, b)
	}
	got = send(t, h, pathPwComplete, tokA, `{"code":"`+codes[0]+`"}`)
	if got.status != http.StatusOK || got.body["authenticated_at"] == nil || got.body["token"] != nil {
		t.Fatalf("complete on A: HTTP %d body %s", got.status, got.raw)
	}
	if a := authAt(t, e, tokA); !a.After(beforeA) {
		t.Errorf("A not refreshed: %v -> %v", beforeA, a)
	}
	if b := authAt(t, e, tokB); !b.Equal(beforeB) {
		t.Errorf("B moved by A's re-auth: %v -> %v", beforeB, b)
	}
	live(t, e, "session B", tokB)
}

// RH11: the re-auth throttle keys are the principal and the session hash (§16), for both steps: the four buckets
// HMAC(pepper, kind ":" value) (per window, as auth_throttle_hit stores them) exist after one start and one complete, keyed by the calling session's principal and hash.
func TestReauthRH11ThrottleKeys(t *testing.T) {
	e := fixture(t)
	p := passwordPrincipal(t, e)
	tok := session(t, e, p)
	setAuthAge(t, e, tok, "2 hours")
	pepper := rnd32()
	h := passwordHandler(t, e, &mailer{}, pepper)
	if got := send(t, h, pathPwStart, tok, `{"password":"`+phrase+`","locale":"en"}`); got.status != http.StatusOK {
		t.Fatalf("start: HTTP %d body %s", got.status, got.raw)
	}
	want(t, "wrong code", send(t, h, pathPwComplete, tok, `{"code":"000000"}`), http.StatusUnprocessableEntity, "invalid_code")
	// The stored row key is sha256(HMAC(pepper, kind ":" value) || int4(window seconds) || int4(offset)) (0070
	// auth_throttle_hit, F1); the start step uses the 900 s email-pw window, the complete step the 600 s binding window.
	key := func(kind, value string, window uint32) []byte {
		m := hmac.New(sha256.New, pepper)
		m.Write([]byte(kind + ":" + value))
		var w [8]byte
		binary.BigEndian.PutUint32(w[:4], window)
		k := sha256.Sum256(append(m.Sum(nil), w[:]...))
		return k[:]
	}
	sessionHex := hex.EncodeToString(hash(tok))
	for _, b := range []struct {
		kind, value string
		window      uint32
	}{
		{"reauth-pw-principal", p, 900}, {"reauth-pw-session", sessionHex, 900},
		{"reauth-code-principal", p, 600}, {"reauth-code-session", sessionHex, 600},
	} {
		if n := count(t, e.owner, `SELECT count(*) FROM identity.auth_throttle WHERE bucket=$1`, key(b.kind, b.value, b.window)); n < 1 {
			t.Errorf("no throttle bucket %s keyed by the calling session's %s", b.kind, b.kind[strings.LastIndex(b.kind, "-")+1:])
		}
	}
}

// world is one store whose creator owns every permission, with a stale (2 hours) and a fresh session of that owner.
type world struct {
	e                    *env
	tenant, store, owner string
	stale, fresh, member string
}

func newWorld(t *testing.T, e *env) *world {
	t.Helper()
	w := &world{e: e, owner: principal(t, e)}
	boot := session(t, e, w.owner)
	if err := e.id.QueryRow(context.Background(),
		`SELECT tenant_id::text, store_id::text FROM identity.create_initial_store($1,$2,$3,'OC15 tenant','OC15 store','OC15 warehouse','TWD')`,
		hash(boot), "oc15-"+uuid(), rnd32()).Scan(&w.tenant, &w.store); err != nil {
		t.Fatalf("fixture store: %v", err)
	}
	w.stale, w.fresh = session(t, e, w.owner), session(t, e, w.owner)
	setAuthAge(t, e, w.stale, "2 hours")
	setAuthAge(t, e, w.fresh, "0 seconds")
	w.member = principal(t, e)
	mustExec(t, e.owner, `INSERT INTO identity.memberships(tenant_id,principal_id) VALUES ($1::uuid,$2::uuid)`, w.tenant, w.member)
	mustExec(t, e.owner, `INSERT INTO identity.store_staff(tenant_id,store_id,principal_id,role) VALUES ($1::uuid,$2::uuid,$3::uuid,'viewer')`, w.tenant, w.store, w.member)
	return w
}

// RH12: staff domain (internal/identity/staff.go -> identityhttp staff error table). A stale session gets 403
// reauth_required on invite, set-role and remove and stays live; a fresh session asking for the owner role gets 403
// owner_role_disabled, and so does accepting an owner invitation issued before 0203.
func TestReauthRH12StaffCodes(t *testing.T) {
	e := fixture(t)
	w := newWorld(t, e)
	staff, err := identity.NewStaff(e.id, &mailer{}, "https://admin.example.test")
	if err != nil {
		t.Fatal(err)
	}
	h, err := identityhttp.NewStaffHandler(staff, bffKey)
	if err != nil {
		t.Fatal(err)
	}
	want(t, "stale invite", send(t, h, "/v1/identity/staff/invite", w.stale,
		`{"store_id":"`+w.store+`","email":"`+email("oc15.invite")+`","role":"viewer","locale":"en"}`), http.StatusForbidden, "reauth_required")
	want(t, "stale set-role", send(t, h, "/v1/identity/staff/set-role", w.stale,
		`{"store_id":"`+w.store+`","principal_id":"`+w.member+`","role":"admin"}`), http.StatusForbidden, "reauth_required")
	want(t, "stale remove", send(t, h, "/v1/identity/staff/remove", w.stale,
		`{"store_id":"`+w.store+`","principal_id":"`+w.member+`"}`), http.StatusForbidden, "reauth_required")
	live(t, e, "stale owner session after reauth_required", w.stale)
	want(t, "owner invite", send(t, h, "/v1/identity/staff/invite", w.fresh,
		`{"store_id":"`+w.store+`","email":"`+email("oc15.owner")+`","role":"owner","locale":"en"}`), http.StatusForbidden, "owner_role_disabled")
	want(t, "owner set-role", send(t, h, "/v1/identity/staff/set-role", w.fresh,
		`{"store_id":"`+w.store+`","principal_id":"`+w.member+`","role":"owner"}`), http.StatusForbidden, "owner_role_disabled")

	// An owner invitation issued before 0203 (inserted in the shape staff_invite wrote) cannot be accepted.
	inviteeEmail, inviteToken := email("oc15.accept"), token()
	phc, err := identity.HashPassword(phrase)
	if err != nil {
		t.Fatal(err)
	}
	invitee := principal(t, e)
	mustExec(t, e.owner, `INSERT INTO identity.password_credentials(principal_id,email,password_hash,email_verified_at) VALUES ($1::uuid,$2,$3,clock_timestamp())`, invitee, inviteeEmail, phc)
	inviteeTok := session(t, e, invitee)
	mustExec(t, e.owner, `WITH c AS (SELECT clock_timestamp() AS t)
		INSERT INTO identity.staff_invitations(id,tenant_id,store_id,email,role,locale,token_hash,invited_by,created_at,expires_at)
		SELECT $1::uuid,$2::uuid,$3::uuid,$4,'owner','en',$5,$6::uuid,c.t,c.t+interval '72 hours' FROM c`,
		uuid(), w.tenant, w.store, inviteeEmail, hash(inviteToken), w.owner)
	want(t, "accept old owner invitation", send(t, h, "/v1/identity/staff/accept", inviteeTok, `{"token":"`+inviteToken+`"}`),
		http.StatusForbidden, "owner_role_disabled")
	if n := count(t, e.owner, `SELECT count(*) FROM identity.store_staff WHERE store_id=$1::uuid AND principal_id=$2::uuid`, w.store, invitee); n != 0 {
		t.Errorf("the refused owner invitation added a staff row (%d)", n)
	}
}

// admin issues one merchant API request to internal/httpapi as the admin BFF does.
func admin(t *testing.T, h http.Handler, method, path, bearer, key, body string) reply {
	t.Helper()
	var rd *bytes.Reader
	if body != "" {
		rd = bytes.NewReader([]byte(body))
	}
	var req *http.Request
	if rd != nil {
		req = httptest.NewRequest(method, path, rd)
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req.Header.Set("Authorization", "Bearer "+bearer)
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	out := reply{status: rec.Code, header: rec.Header(), raw: rec.Body.String()}
	_ = json.Unmarshal(rec.Body.Bytes(), &out.body)
	return out
}

// RH13: end to end through internal/httpapi (handler.go classify decides the code): a session authenticated 2 hours ago
// performing a guarded operation gets HTTP 403 with body code reauth_required in each guarded domain wired here -
// orders CSV (merchantorders/orders.go), finance CSV and products report CSV (reporting), customer erasure (customers/read.go),
// the bank-transfer decisions (merchantorders offline -> orders.go), platform Stripe settings (payments/platformstripe coded
// 403) and billing checkout (billing/errors.go via billing.pin_customer) - and stays live; a fresh session gets the export.
func TestReauthRH13GuardedDomainsEndToEnd(t *testing.T) {
	e := fixture(t)
	w := newWorld(t, e)
	// Platform billing over the billingtest fake (never api.stripe.com): checkout creates the Stripe customer, then the guarded
	// billing.pin_customer refuses the stale session.
	fake := billingtest.New("acct_OC15" + hex.EncodeToString(rnd32()[:6]))
	t.Cleanup(fake.Close)
	cfg := billing.Config{SecretKey: "sk_" + "test_" + hex.EncodeToString(rnd32()[:12]), WebhookSecret: "whsec_" + hex.EncodeToString(rnd32()[:16]),
		ReturnOrigin: "https://admin.example.test", PriceIDs: []string{"price_OC15Month"}}
	fake.RequireKey(cfg.SecretKey)
	fake.AddPrice("price_OC15Month", 30000, "twd", "month", "Pro Monthly", true, false)
	bill, err := billing.New(context.Background(), e.runtime, cfg, fake.Transport())
	if err != nil || !bill.Enabled() {
		t.Fatalf("billing service: enabled=%v err=%v", bill != nil && bill.Enabled(), err)
	}
	h := httpapi.NewHandler(e.runtime, httpapi.Options{Billing: bill, PaymentProfile: "SANDBOX"})
	base := "/v1/admin/stores/" + w.store
	today := time.Now().UTC()
	from, to := today.AddDate(0, 0, -7).Format("2006-01-02"), today.Format("2006-01-02")
	key := func() string { return "oc15-" + hex.EncodeToString(rnd32()[:8]) }
	for _, c := range []struct {
		name, method, path, key, body string
	}{
		{"orders unshipped.csv", http.MethodGet, base + "/orders/unshipped.csv", "", ""},
		{"finance summary.csv", http.MethodGet, base + "/finance/summary.csv?from=" + from + "&to=" + to, "", ""},
		{"customer erasure", http.MethodPost, base + "/customers/" + uuid() + "/erasure", key(), `{"confirm":"ERASE"}`},
		{"bank transfer confirm", http.MethodPost, base + "/orders/" + uuid() + "/bank-transfer/confirm", key(), `{}`},
		{"bank transfer refund-offline", http.MethodPost, base + "/orders/" + uuid() + "/bank-transfer/refund-offline", key(), `{}`},
		{"products report csv", http.MethodGet, base + "/reports/products.csv?from=" + from + "&to=" + to, "", ""},
		{"platform Stripe settings", http.MethodPut, base + "/payments/card", "", `{"enabled":false,"terms_version":"","descriptor_suffix":null,"expected_version":0}`},
		{"billing checkout", http.MethodPost, base + "/billing/checkout", "", `{"price_id":"price_OC15Month"}`},
	} {
		want(t, c.name, admin(t, h, c.method, c.path, w.stale, c.key, c.body), http.StatusForbidden, "reauth_required")
	}
	live(t, e, "stale session after the guarded refusals", w.stale)
	if got := admin(t, h, http.MethodGet, base+"/orders/unshipped.csv", w.fresh, "", ""); got.status != http.StatusOK {
		t.Errorf("fresh session export: HTTP %d body %s, want 200", got.status, got.raw)
	}
}
