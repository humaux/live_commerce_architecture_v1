//go:build browser

package foundation_test

// INDEPENDENT browser gate TestBrowserStaffTeam (unit staff-team, contracts/storefront-v2.md §D). Tier BROWSER (MOCK mail): Chromium
// (tests/admin/staff-team.spec.ts) -> packaged Next admin BFF -> in-process Go api (identityhttp staff routes + password handler + the
// admin API) -> real PG, with the REAL *mail.SMTP adapter against the loopback mailtest server. Only the mailbox is faked.
// Run through `bash scripts/dev/test-local.sh --browser-password-auth` (same harness as PA11: loopback SMTP fake, password login on,
// OIDC off). The mailbox inspection endpoint (GET /mail?to=) exists only in this test binary on a separate loopback listener.
// After the spec the Go side proves in PG that the browser really did it (invitations accepted, the five audit actions, the owner
// still the only owner of every store), that the real SMTP server received the invitation mails, and scans the admin server log,
// the browser log, the Go log capture AND every API request URI (path + query, recorded here) for every sentinel the spec wrote
// (invitation tokens, passwords, codes, addresses): none may appear. Owner-pool use: reads only.
// LC_STAFF_TEAM_SPEC_ARGS (default empty) is appended to the playwright command (evidence runs: `--grep-invert @defect`).
// Evidence: output/playwright/staff-team-<ts>/ (or LC_BROWSER_EVIDENCE_ROOT).

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
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

func TestBrowserStaffTeam(t *testing.T) {
	if os.Getenv("LC_BROWSER_PASSWORD_AUTH_ACCEPTANCE") != "1" || os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" {
		t.Fatal("use scripts/dev/test-local.sh --browser-password-auth; isolated fixtures are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	logs := pwaCaptureLogs(t)
	e := newPwa(t, pwaCap(5000)) // sign-up/login mails of every person in every chain must never hit the daily cap
	f := e.f
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	uiAddress := listener.Addr().String()
	_ = listener.Close()
	_, uiPort, _ := net.SplitHostPort(uiAddress)
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
	mux.Handle("/v1/identity/staff/", staffHandler) // the same mount cmd/api's withStaff makes (longest pattern wins)
	mux.Handle("/v1/identity/password/", e.handler)
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
	var urlMu sync.Mutex
	var requestURIs []string // path AND query of every API request the BFF made: a token must be in none of them
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		urlMu.Lock()
		requestURIs = append(requestURIs, r.Method+" "+r.URL.RequestURI())
		urlMu.Unlock()
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

	inspect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/mail" {
			http.NotFound(w, r)
			return
		}
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
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	}))
	t.Cleanup(inspect.Close)

	evidenceRoot := os.Getenv("LC_BROWSER_EVIDENCE_ROOT")
	if evidenceRoot == "" {
		evidenceRoot = filepath.Join(root, "output", "playwright")
	}
	evidence := filepath.Join(evidenceRoot, "staff-team-"+time.Now().UTC().Format("20060102T150405.000000000"))
	if err := os.MkdirAll(evidence, 0700); err != nil {
		t.Fatal(err)
	}
	serverLog := browserLog(t, filepath.Join(evidence, "next.log"))
	server := exec.CommandContext(ctx, "node", filepath.Join(root, "apps/admin/.next/standalone/apps/admin/server.js"))
	server.Dir = root
	server.Env = browserEnvironment(map[string]string{
		"HOSTNAME": "127.0.0.1", "PORT": uiPort, "NODE_ENV": "production",
		"COMMERCE_IDENTITY_ENABLED": "1", "COMMERCE_IDENTITY_ALLOW_LOOPBACK_TESTS": "1",
		"COMMERCE_PASSWORD_LOGIN_ENABLED": "1",
		"COMMERCE_PUBLIC_ORIGIN":          publicOrigin, "COMMERCE_API_ORIGIN": api.URL,
		"COMMERCE_BFF_KEY":            e.bffKey,
		"COMMERCE_ONBOARDING_ENABLED": "1", "COMMERCE_ONBOARDING_CURRENCIES": "TWD,USD",
	})
	server.Stdout, server.Stderr = serverLog, serverLog
	if err := server.Start(); err != nil {
		t.Fatal("could not start packaged Next server")
	}
	serverDone := make(chan error, 1)
	go func() { serverDone <- server.Wait() }()
	t.Cleanup(func() {
		_ = server.Process.Kill() // exact child PID only
		select {
		case <-serverDone:
		case <-time.After(5 * time.Second):
			t.Error("owned Next process did not stop")
		}
	})
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
	args := []string{"exec", "playwright", "test", "tests/admin/staff-team.spec.ts", "--reporter=list", "--output=" + filepath.Join(evidence, "results")}
	if extra := strings.Fields(os.Getenv("LC_STAFF_TEAM_SPEC_ARGS")); len(extra) > 0 {
		args = append(args, extra...)
	}
	browser := exec.CommandContext(ctx, "pnpm", args...)
	browser.Dir = root
	browser.Env = browserEnvironment(map[string]string{
		"LC_BROWSER_SUITE":         "staff-team",
		"LC_BROWSER_PUBLIC_ORIGIN": publicOrigin, "LC_BROWSER_API_ORIGIN": api.URL,
		"LC_BROWSER_INSPECT_ORIGIN": inspect.URL, "LC_BROWSER_EVIDENCE_DIR": evidence,
	})
	browser.Stdout, browser.Stderr = browserLogFile, browserLogFile
	specErr := browser.Run()
	if specErr != nil {
		t.Errorf("browser spec failed; local evidence: %s", evidence)
	}

	// (a) Independent PG proof that the browser flows were real.
	count := func(q string, args ...any) int {
		var n int
		if err := f.owner.QueryRow(ctx, q, args...).Scan(&n); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		return n
	}
	grep := func(action string) int {
		return count(`SELECT count(*) FROM ops.audit_events a WHERE a.action=$1 AND EXISTS (SELECT 1 FROM identity.password_credentials c WHERE c.principal_id IN (SELECT principal_id FROM identity.store_staff s WHERE s.store_id=a.store_id) AND c.email LIKE 'stf.%@example.test')`, action)
	}
	if n := count(`SELECT count(*) FROM identity.staff_invitations WHERE email LIKE 'stf.%@example.test' AND accepted_at IS NOT NULL AND role='fulfilment'`); n < 4 {
		t.Errorf("accepted fulfilment invitations created through the browser: %d, want >= 4 (one per chain)", n)
	}
	for action, min := range map[string]int{"staff.invited:fulfilment": 4, "staff.accepted:fulfilment": 4, "staff.role_changed:viewer": 4, "staff.removed": 4} {
		if n := grep(action); n < min {
			t.Errorf("audit %s: %d, want >= %d", action, n, min)
		}
	}
	if n := count(`SELECT count(*) FROM (SELECT s.store_id FROM identity.store_staff s JOIN identity.password_credentials c ON c.principal_id=s.principal_id WHERE s.role='owner' AND c.email LIKE 'stf.%@example.test' GROUP BY s.store_id HAVING count(*) = 1) x`); n < 4 {
		t.Errorf("stores whose single owner is the browser-created creator: %d, want >= 4", n)
	}
	if n := count(`SELECT count(*) FROM identity.staff_invitations i WHERE i.email LIKE 'stf.%@example.test' AND i.accepted_at IS NOT NULL AND i.accepted_by IS NOT NULL AND EXISTS (SELECT 1 FROM identity.password_credentials c WHERE c.principal_id=i.accepted_by AND c.email=i.email)`); n < 4 {
		t.Errorf("invitations accepted by the account holding the invited address: %d, want >= 4", n)
	}
	invites := 0
	for _, m := range e.smtp.Messages() {
		if strings.Contains(string(m.Raw), "/invite/") && strings.HasPrefix(m.To, "stf.") {
			invites++
		}
	}
	if invites < 4 {
		t.Errorf("invitation mails received by the real SMTP server: %d, want >= 4", invites)
	}

	// (b) Canary scan: nothing the spec recorded may appear in any server-side log or API URL.
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
	if len(canaries) < 30 {
		t.Fatalf("only %d canaries recorded; the scan would be vacuous", len(canaries))
	}
	tokens := 0
	for _, c := range canaries {
		if len(c) == 43 && !strings.ContainsAny(c, "@.") {
			tokens++
		}
	}
	if tokens < 4 {
		t.Fatalf("only %d invitation tokens among the canaries", tokens)
	}
	for name, path := range map[string]string{"admin server log": filepath.Join(evidence, "next.log"), "browser process log": filepath.Join(evidence, "playwright.log")} {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range canaries {
			if strings.Contains(string(b), c) {
				t.Errorf("%s contains a recorded sentinel (token, password, code or address)", name)
				break
			}
		}
	}
	captured := logs.String()
	for _, c := range canaries {
		if strings.Contains(captured, c) {
			t.Error("Go log capture contains a recorded sentinel (token, password, code or address)")
			break
		}
	}
	urlMu.Lock()
	uris := strings.Join(requestURIs, "\n")
	urlMu.Unlock()
	if len(requestURIs) < 50 {
		t.Errorf("only %d API requests recorded", len(requestURIs))
	}
	for _, c := range canaries {
		if strings.Contains(uris, c) {
			t.Errorf("an API request URI (path or query) contains a recorded sentinel")
			break
		}
	}
	if !strings.Contains(uris, "POST /v1/identity/staff/accept") || !strings.Contains(uris, "POST /v1/identity/staff/invite") {
		t.Error("the staff routes were not exercised through the BFF")
	}
	if specErr == nil {
		t.Logf("PASS: browser -> Next -> Go staff routes -> real PG, invitation mails over the real SMTP adapter; %d canaries (%d invitation tokens) not found in any log or API URL; evidence=%s", len(canaries), tokens, evidence)
	}
}
