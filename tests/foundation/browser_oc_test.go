//go:build browser

package foundation_test

// INDEPENDENT browser gate TestBrowserOC (card OC-04, contracts/merchant-password-auth-v1.md §16 / §16.1): Playwright suite `oc`
// (tests/admin/oc/*.spec.ts) -> packaged Next admin BFF -> in-process Go api (identityhttp password + staff routes, the admin API)
// -> real PG, with the REAL *mail.SMTP adapter against the loopback mailtest server. Runs in `--browser-password-auth` (same
// loopback-SMTP harness as PA11 / staff-team). Password login on AND an OIDC issuer configured, so the dialog offers both routes.
// Tiers: password re-auth (start -> emailed code -> complete), the guarded 403 codes and the replay are REAL (Go + PG + SMTP fake).
// The OIDC IdP is MOCK: this binary answers POST /v1/identity/login/reauth/start and the re-auth branch of
// POST /v1/identity/login/complete itself (the shape of internal/identityhttp/reauth_routes.go; the SQL side, complete_oidc_reauth,
// is OC-02 RA04/RA07) and marks that session fresh in PG, so the BFF callback and the dialog are exercised end to end.
// Test-only inspection listener (loopback): GET /mail?to=, POST /age?email= (sets authenticated_at 2 h back for that account's
// sessions), GET /authorize?state= (MOCK IdP: 303 back to <public origin>/api/auth/callback).
// After the spec: PG proves every invited address was invited exactly once (one replay, never two), that no re-auth issued a
// session, and the logs / API URIs carry none of the recorded passwords, codes or addresses.
// Evidence: output/playwright/oc-<ts>/ (or LC_BROWSER_EVIDENCE_ROOT).

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/identity"
	"livecommerce/internal/identityhttp"
	"livecommerce/internal/integrations/accounts"
	"livecommerce/internal/integrations/core"
)

func TestBrowserOC(t *testing.T) {
	if os.Getenv("LC_BROWSER_PASSWORD_AUTH_ACCEPTANCE") != "1" || os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" {
		t.Fatal("use scripts/dev/test-local.sh --browser-password-auth; isolated fixtures are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	logs := pwaCaptureLogs(t)
	e := newPwa(t, pwaCap(5000))
	f := e.f
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	relay := newBrowserAdminRelay(t)
	uiAddress := relay.server.Listener.Addr().String()
	publicOrigin := browserFront(t, uiAddress)

	private, err := identityhttp.NewHandler(e.oidc, e.bffKey)
	if err != nil {
		t.Fatal(err)
	}
	staffService, err := identity.NewStaff(e.pool, e.mailer, publicOrigin)
	if err != nil {
		t.Fatal(err)
	}
	staffHandler, err := identityhttp.NewStaffHandler(staffService, e.bffKey)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/v1/identity/staff/", staffHandler)
	mux.Handle("/v1/identity/password/", e.handler) // includes password/reauth/{start,complete} (OC-15)
	mux.Handle("/v1/identity/", private)
	jobs, err := river.NewClient(riverpgxv5.New(f.runtime), &river.Config{Schema: "river"})
	if err != nil {
		t.Fatal(err)
	}
	bindings, err := core.New(jobs)
	if err != nil {
		t.Fatal(err)
	}
	accountKeys, err := accounts.NewKeyring("browser_fixture", map[string][]byte{"browser_fixture": randomBytes(32)}, randomBytes(32))
	if err != nil {
		t.Fatal(err)
	}
	accountService, err := accounts.New(accountKeys, bindings)
	if err != nil {
		t.Fatal(err)
	}
	mux.Handle("/", httpapi.NewHandler(f.runtime, httpapi.Options{SessionStoreList: true, Accounts: accountService}))

	// MOCK IdP state: binding -> (state, calling session hash). Never logged.
	type flow struct {
		state   string
		session [32]byte
	}
	var flowMu sync.Mutex
	flows := map[string]flow{}
	token := func() string { return base64.RawURLEncoding.EncodeToString(randomBytes(32)) }
	writeJSON := func(w http.ResponseWriter, status int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}

	inspect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/mail":
			to := strings.ToLower(r.URL.Query().Get("to"))
			type item struct {
				Subject string `json:"subject"`
				Text    string `json:"text"`
				HTML    string `json:"html"`
			}
			out := []item{}
			for _, m := range e.smtp.Messages() {
				if strings.EqualFold(m.To, to) {
					out = append(out, item{m.Subject, m.Text, m.HTML})
				}
			}
			writeJSON(w, http.StatusOK, out)
		case r.Method == http.MethodPost && r.URL.Path == "/age":
			tag, err := f.owner.Exec(r.Context(), `UPDATE identity.sessions s SET authenticated_at = clock_timestamp() - interval '2 hours'
				FROM identity.password_credentials c WHERE c.principal_id = s.principal_id AND lower(c.email) = lower($1) AND s.revoked_at IS NULL`,
				r.URL.Query().Get("email"))
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "age"})
				return
			}
			writeJSON(w, http.StatusOK, map[string]int64{"sessions": tag.RowsAffected()})
		case r.Method == http.MethodGet && r.URL.Path == "/authorize":
			// MOCK IdP: the person "signs in" at once and is sent back to the one registered callback.
			http.Redirect(w, r, publicOrigin+"/api/auth/callback?"+url.Values{"state": {r.URL.Query().Get("state")}, "code": {"mock-" + token()[:16]}}.Encode(), http.StatusSeeOther)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(inspect.Close)

	var urlMu sync.Mutex
	var requestURIs []string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		urlMu.Lock()
		requestURIs = append(requestURIs, r.Method+" "+r.URL.RequestURI())
		urlMu.Unlock()
		if r.Method == http.MethodPost && (r.URL.Path == "/v1/identity/login/reauth/start" || r.URL.Path == "/v1/identity/login/complete") {
			if r.Header.Get("X-Commerce-BFF-Key") != e.bffKey {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"code": "unauthorized"})
				return
			}
			if r.URL.Path == "/v1/identity/login/reauth/start" {
				bearer := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
				if bearer == "" || bearer == r.Header.Get("Authorization") {
					writeJSON(w, http.StatusUnauthorized, map[string]string{"code": "unauthorized"})
					return
				}
				state, binding := token(), token()
				flowMu.Lock()
				flows[binding] = flow{state: state, session: sha256.Sum256([]byte(bearer))}
				flowMu.Unlock()
				writeJSON(w, http.StatusOK, map[string]any{"authorization_url": inspect.URL + "/authorize?state=" + state, "binding": binding, "expires_at": time.Now().Add(5 * time.Minute).UTC()})
				return
			}
			var in struct{ State, Binding, Code string }
			_ = json.NewDecoder(r.Body).Decode(&in)
			flowMu.Lock()
			fl, ok := flows[in.Binding]
			delete(flows, in.Binding)
			flowMu.Unlock()
			if !ok || fl.state != in.State {
				writeJSON(w, http.StatusForbidden, map[string]string{"code": "reauth_failed"})
				return
			}
			var at time.Time
			if err := f.owner.QueryRow(r.Context(), `UPDATE identity.sessions SET authenticated_at = clock_timestamp() WHERE token_hash = $1 RETURNING authenticated_at`, fl.session[:]).Scan(&at); err != nil {
				writeJSON(w, http.StatusForbidden, map[string]string{"code": "reauth_failed"})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"reauth": true, "authenticated_at": at.UTC()})
			return
		}
		recorded := httptest.NewRecorder()
		mux.ServeHTTP(recorded, r)
		for name, values := range recorded.Header() {
			w.Header()[name] = values
		}
		w.WriteHeader(recorded.Code)
		_, _ = w.Write(recorded.Body.Bytes())
		t.Logf("Go transport %s %s -> %d", r.Method, r.URL.Path, recorded.Code)
	}))
	t.Cleanup(api.Close)

	evidenceRoot := os.Getenv("LC_BROWSER_EVIDENCE_ROOT")
	if evidenceRoot == "" {
		evidenceRoot = filepath.Join(root, "output", "playwright")
	}
	evidence := filepath.Join(evidenceRoot, "oc-"+time.Now().UTC().Format("20060102T150405.000000000"))
	if err := os.MkdirAll(evidence, 0700); err != nil {
		t.Fatal(err)
	}
	serverLog := browserLog(t, filepath.Join(evidence, "next.log"))
	server := exec.CommandContext(ctx, "node", filepath.Join(root, "apps/admin/.next/standalone/apps/admin/server.js"))
	server.Dir = root
	server.Env = browserEnvironment(map[string]string{
		"HOSTNAME": "localhost", "NODE_OPTIONS": "--dns-result-order=ipv4first", "NODE_ENV": "production",
		"COMMERCE_IDENTITY_ENABLED": "1", "COMMERCE_IDENTITY_ALLOW_LOOPBACK_TESTS": "1",
		"COMMERCE_PASSWORD_LOGIN_ENABLED": "1", "COMMERCE_OIDC_ISSUER": inspect.URL,
		"COMMERCE_PUBLIC_ORIGIN": publicOrigin, "COMMERCE_API_ORIGIN": api.URL,
		"COMMERCE_BFF_KEY":            e.bffKey,
		"COMMERCE_ONBOARDING_ENABLED": "1", "COMMERCE_ONBOARDING_CURRENCIES": "TWD,USD",
	})
	server.Stdout, server.Stderr = serverLog, serverLog
	server = relay.start(t, ctx, server)
	client := &http.Client{Timeout: time.Second}
	ready := false
	for attempt := 0; attempt < 100 && !ready; attempt++ {
		if response, err := client.Get("http://" + uiAddress + "/api/stores"); err == nil {
			_ = response.Body.Close()
			ready = response.StatusCode == http.StatusUnauthorized
		}
		if !ready {
			select {
			case <-ctx.Done():
				t.Fatal("Next readiness deadline")
			case <-time.After(100 * time.Millisecond):
			}
		}
	}
	if !ready {
		t.Fatalf("Next readiness failed; local evidence: %s", evidence)
	}
	browserLogFile := browserLog(t, filepath.Join(evidence, "playwright.log"))
	browser := exec.CommandContext(ctx, "pnpm", "exec", "playwright", "test", "--reporter=list", "--output="+filepath.Join(evidence, "results"))
	browser.Dir = root
	browser.Env = browserEnvironment(map[string]string{
		"LC_BROWSER_SUITE":         "oc",
		"LC_BROWSER_PUBLIC_ORIGIN": publicOrigin, "LC_BROWSER_API_ORIGIN": api.URL,
		"LC_BROWSER_INSPECT_ORIGIN": inspect.URL, "LC_BROWSER_EVIDENCE_DIR": evidence,
	})
	browser.Stdout, browser.Stderr = browserLogFile, browserLogFile
	specErr := browser.Run()
	if specErr != nil {
		t.Errorf("browser spec failed; local evidence: %s", evidence)
	}

	// (a) PG: each address the spec invited after a reauth_required was invited exactly once (the replay ran once, never twice).
	count := func(q string, args ...any) int {
		var n int
		if err := f.owner.QueryRow(ctx, q, args...).Scan(&n); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		return n
	}
	if n := count(`SELECT count(*) FROM identity.staff_invitations WHERE email LIKE 'oc4i.%@example.test'`); n < 3 {
		t.Errorf("invitations replayed after re-auth: %d, want >= 3 (one per chain)", n)
	}
	if n := count(`SELECT count(*) FROM (SELECT email FROM identity.staff_invitations WHERE email LIKE 'oc4i.%@example.test' GROUP BY email HAVING count(*) <> 1) x`); n != 0 {
		t.Errorf("addresses invited more than once (a replay ran twice): %d", n)
	}
	// (b) PG: re-authentication never issued a session - each browser-created owner still has exactly its sign-up session.
	if n := count(`SELECT count(DISTINCT c.principal_id) FROM identity.password_credentials c WHERE c.email LIKE 'oc4o.%@example.test'`); n < 3 {
		t.Errorf("owners created through the browser: %d, want >= 3", n)
	}
	if n := count(`SELECT count(*) FROM (SELECT s.principal_id FROM identity.sessions s JOIN identity.password_credentials c ON c.principal_id = s.principal_id
		WHERE c.email LIKE 'oc4o.%@example.test' GROUP BY s.principal_id HAVING count(*) <> 1) x`); n != 0 {
		t.Errorf("owners with more or fewer than one session after re-authentication: %d", n)
	}
	// (c) the routes were really used through the BFF.
	urlMu.Lock()
	uris := strings.Join(requestURIs, "\n")
	urlMu.Unlock()
	for _, want := range []string{"POST /v1/identity/password/reauth/start", "POST /v1/identity/password/reauth/complete",
		"POST /v1/identity/login/reauth/start", "POST /v1/identity/login/complete", "POST /v1/identity/staff/invite"} {
		if !strings.Contains(uris, want) {
			t.Errorf("%s was not exercised through the BFF", want)
		}
	}
	// (d) canary scan: no recorded password, code or address in any log or API URI.
	raw, err := os.ReadFile(filepath.Join(evidence, "canaries.txt"))
	if err != nil {
		t.Fatalf("the spec recorded no canaries: %v", err)
	}
	var canaries []string
	for _, line := range strings.Split(string(raw), "\n") {
		if len(strings.TrimSpace(line)) >= 6 {
			canaries = append(canaries, strings.TrimSpace(line))
		}
	}
	if len(canaries) < 10 {
		t.Fatalf("only %d canaries recorded; the scan would be vacuous", len(canaries))
	}
	for name, path := range map[string]string{"admin server log": filepath.Join(evidence, "next.log"), "browser process log": filepath.Join(evidence, "playwright.log")} {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range canaries {
			if strings.Contains(string(b), c) {
				t.Errorf("%s contains a recorded sentinel (password, code or address)", name)
				break
			}
		}
	}
	captured := logs.String()
	for _, c := range canaries {
		if strings.Contains(captured, c) {
			t.Error("Go log capture contains a recorded sentinel (password, code or address)")
			break
		}
		if strings.Contains(uris, c) {
			t.Error("an API request URI contains a recorded sentinel")
			break
		}
	}
	if specErr == nil {
		t.Logf("PASS: browser -> Next -> Go re-auth/staff/admin routes -> real PG, codes over the real SMTP adapter, OIDC IdP MOCK; %d canaries clean; evidence=%s", len(canaries), evidence)
	}
}
