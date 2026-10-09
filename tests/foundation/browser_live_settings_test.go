//go:build browser

// Purpose: independently exercise W3-U2 real clicks through signed OIDC, production Next and its actual BFF.
// Depends on: lcSetup, identityFixture, loopback IdP, real PG scope and test-only MOCK settings backend; Playwright.
// Used by: scripts/dev/test-local.sh --browser-live-settings (LC_BROWSER_LIVE_SETTINGS_ACCEPTANCE=1).
// Invariants: I01/I02/I06/I11/I14/I18; no provider sends, raw buyer fields or command bodies enter evidence.
package foundation_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/identity"
	"livecommerce/internal/identityhttp"
	"livecommerce/internal/oidclogin"
	"livecommerce/internal/platform"
)

// TestBrowserLiveSettingsUIRealChain keeps identity/authorization real while business/provider facts are explicitly MOCK.
func TestBrowserLiveSettingsUIRealChain(t *testing.T) {
	if os.Getenv("LC_BROWSER_LIVE_SETTINGS_ACCEPTANCE") != "1" || os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" {
		t.Fatal("use scripts/dev/test-local.sh --browser-live-settings")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 700*time.Second)
	defer cancel()
	h := lcSetup(t)
	_, readToken := lcPrincipal(t, h.f, h.f.tenantA, []string{h.f.storeA1}, "store:read", "live:read", "inbox:read")
	for _, permission := range []string{"inbox:read", "inbox:reply"} {
		mustExec(t, h.f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,$4)`, h.f.tenantA, h.f.storeA1, h.actor, permission)
	}
	mustExec(t, h.f.owner, `INSERT INTO identity.store_staff(tenant_id,store_id,principal_id,role) VALUES($1,$2,$3,'admin')`, h.f.tenantA, h.f.storeA1, h.actor)
	t.Cleanup(func() { mustExec(t, h.f.owner, `DELETE FROM identity.store_staff WHERE principal_id=$1`, h.actor) })
	m := &blsMock{store: h.f.storeA1, otherStore: h.f.storeA2, scene: h.draft(t, h.f.storeA1), otherScene: h.draft(t, h.f.storeA2), bundle: randomUUID(), restrictedBundle: randomUUID(), entry: randomUUID(), nextEntry: randomUUID(), comment: "700_800"}
	m.reset()
	control := randomToken()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	_, port, _ := net.SplitHostPort(addr)
	_ = listener.Close()
	origin := browserFront(t, addr)
	idp := newBrowserIDP(t, origin+"/api/auth/callback")
	mustExec(t, h.f.owner, `INSERT INTO identity.external_identities(issuer,subject,principal_id) VALUES($1,'browser-subject',$2)`, idp.server.URL, h.actor)
	t.Cleanup(func() {
		mustExec(t, h.f.owner, `DELETE FROM identity.external_identities WHERE issuer=$1`, idp.server.URL)
	})
	_, _, authority := identityFixture(t)
	provider, err := oidclogin.New(ctx, oidclogin.Config{Issuer: idp.server.URL, ClientID: browserClientID, RedirectURL: idp.redirect, AllowLoopbackForTests: true})
	if err != nil {
		t.Fatal(err)
	}
	service, err := identity.New(authority, observedBrowserProvider{Provider: provider, t: t}, identity.Policy{ProviderKey: "browser-live-settings-v1", SessionTTL: time.Hour})
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
	mux.Handle("/", httpapi.NewHandler(h.f.runtime, httpapi.Options{SessionStoreList: true, Studio: true, ClaimLabels: &h.labels}))
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/__test/live-settings/") {
			if r.Header.Get("X-Settings-Control") != control {
				consoleJSON(w, 403, map[string]any{"code": "forbidden"})
				return
			}
			m.mu.Lock()
			defer m.mu.Unlock()
			switch strings.TrimPrefix(r.URL.Path, "/__test/live-settings/") {
			case "facts":
				consoleJSON(w, 200, m.facts())
			case "reset":
				if m.hold != nil {
					close(m.hold)
					m.hold = nil
				}
				m.reset()
				consoleJSON(w, 200, map[string]any{"reset": true})
			case "fault":
				var b struct {
					Mode string `json:"mode"`
				}
				if json.NewDecoder(r.Body).Decode(&b) != nil {
					consoleJSON(w, 400, map[string]any{"code": "invalid_request"})
					return
				}
				switch b.Mode {
				case "conflict", "unknown":
					m.fault = b.Mode
				case "hold-report":
					m.hold = make(chan struct{})
					m.held = false
				case "release":
					if m.hold != nil {
						close(m.hold)
						m.hold = nil
					}
				default:
					consoleJSON(w, 400, map[string]any{"code": "invalid_request"})
					return
				}
				consoleJSON(w, 200, map[string]any{"armed": true})
			default:
				consoleJSON(w, 404, map[string]any{"code": "not_found"})
			}
			return
		}
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) >= 5 && parts[0] == "v1" && parts[1] == "admin" && parts[2] == "stores" {
			if r.Header.Get("Cookie") != "" || r.Header.Get("X-Tenant-ID") != "" || r.Header.Get("X-Forwarded-Host") != "" {
				m.mu.Lock()
				m.badAuthority++
				m.mu.Unlock()
			}
			permission := "live:read"
			if parts[4] == "inbox" {
				permission = "inbox:read"
			}
			if r.Method != "GET" {
				permission = "live:manage"
			}
			err := platform.WithScope(r.Context(), h.f.runtime, strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), parts[3], permission, func(pgx.Tx, platform.Scope) error { return nil })
			if err != nil {
				status, code := 503, "unavailable"
				switch {
				case errors.Is(err, platform.ErrForbidden):
					status, code = 403, "forbidden"
				case errors.Is(err, platform.ErrUnauthorized):
					status, code = 401, "unauthorized"
				case errors.Is(err, platform.ErrScopeNotFound):
					status, code = 404, "not_found"
				}
				consoleJSON(w, status, map[string]any{"code": code})
				return
			}
			if m.serve(w, r, parts[3]) {
				return
			}
		}
		mux.ServeHTTP(w, r)
	}))
	defer api.Close()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	evidenceRoot := os.Getenv("LC_BROWSER_EVIDENCE_ROOT")
	if evidenceRoot == "" {
		evidenceRoot = filepath.Join(root, "output/playwright")
		if os.Getenv("CI") == "true" {
			evidenceRoot = filepath.Join(root, "output/ci-gates")
		}
	}
	evidence := filepath.Join(evidenceRoot, "live-settings", time.Now().UTC().Format("20060102T150405.000000000Z"))
	if err = os.MkdirAll(evidence, 0700); err != nil {
		t.Fatal(err)
	}
	blsSourceManifest(t, root, evidence)
	nextLog := browserLog(t, filepath.Join(evidence, "next.log"))
	next := exec.CommandContext(ctx, "node", filepath.Join(root, "apps/admin/.next/standalone/apps/admin/server.js"))
	next.Dir = root
	next.Env = browserEnvironment(map[string]string{"HOSTNAME": "127.0.0.1", "PORT": port, "NODE_ENV": "production", "COMMERCE_IDENTITY_ENABLED": "1", "COMMERCE_IDENTITY_ALLOW_LOOPBACK_TESTS": "1", "COMMERCE_PUBLIC_ORIGIN": origin, "COMMERCE_API_ORIGIN": api.URL, "COMMERCE_OIDC_ISSUER": idp.server.URL, "COMMERCE_BFF_KEY": bffKey})
	next.Stdout, next.Stderr = nextLog, nextLog
	if err = next.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- next.Wait() }()
	defer func() {
		_ = next.Process.Kill()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("owned Next did not stop")
		}
	}()
	client := &http.Client{Timeout: time.Second}
	ready := false
	for i := 0; i < 150; i++ {
		if res, e := client.Get("http://" + addr + "/en/"); e == nil {
			_ = res.Body.Close()
			if res.StatusCode == 200 {
				ready = true
				break
			}
		}
		select {
		case <-ctx.Done():
			t.Fatal("Next readiness deadline")
		case <-time.After(100 * time.Millisecond):
		}
	}
	if !ready {
		t.Fatalf("Next readiness failed; evidence=%s", evidence)
	}
	cmd := exec.CommandContext(ctx, "pnpm", "exec", "playwright", "test", "--project=live-settings", "--reporter=list", "--output="+filepath.Join(evidence, "results"))
	cmd.Dir = root
	if filter := os.Getenv("LC_BROWSER_SETTINGS_GREP"); filter != "" {
		cmd.Args = append(cmd.Args, "--grep", filter)
		t.Log("FOCUSED_SPEC_ONLY; full-mode acceptance NOT_RUN")
	}
	fixture := map[string]any{"store": m.store, "other_store": m.otherStore, "scene": m.scene, "other_scene": m.otherScene, "bundle": m.bundle, "restricted_bundle": m.restrictedBundle, "entry": m.entry, "next_entry": m.nextEntry, "comment": m.comment}
	cmd.Env = browserEnvironment(map[string]string{"LC_BROWSER_SUITE": "live-settings", "LC_BROWSER_PUBLIC_ORIGIN": origin, "LC_BROWSER_API_ORIGIN": api.URL, "LC_BROWSER_EVIDENCE": evidence, "LC_BROWSER_SETTINGS_CONTROL": control, "LC_BROWSER_SETTINGS_READ_TOKEN": readToken, "LC_BROWSER_SETTINGS_FIXTURE": mustJSON(t, fixture), "FORCE_COLOR": "0", "NO_COLOR": "1"})
	var diagnostics consoleDiagnosticBuffer
	cmd.Stdout, cmd.Stderr = &diagnostics, &diagnostics
	runErr := cmd.Run()
	summary := blsSummary(diagnostics.Bytes())
	t.Log(summary)
	if err = os.WriteFile(filepath.Join(evidence, "playwright.log"), []byte(summary), 0600); err != nil {
		t.Error(err)
	}
	m.mu.Lock()
	raw, _ := json.MarshalIndent(m.facts(), "", "  ")
	bad := m.badAuthority
	m.mu.Unlock()
	if err = os.WriteFile(filepath.Join(evidence, "mock-receipts.json"), raw, 0600); err != nil {
		t.Error(err)
	}
	if bad != 0 {
		t.Errorf("forwarded authority violations=%d", bad)
	}
	// I11: logs and receipts are checked without printing any matched private fixture value.
	_ = filepath.WalkDir(evidence, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			t.Error("evidence traversal failed")
			return nil
		}
		if entry.IsDir() || (filepath.Ext(path) != ".log" && filepath.Ext(path) != ".json") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Error("evidence read failed")
			return nil
		}
		if strings.Contains(string(data), "PRIVATE_") {
			t.Error("private fixture content leaked into text evidence")
		}
		return nil
	})
	if runErr != nil {
		t.Fatalf("live settings browser failed: %v; evidence=%s", runErr, evidence)
	}
	if !regexp.MustCompile(`\b[1-9][0-9]* passed\b`).MatchString(summary) {
		t.Fatal("zero matched browser tests is not acceptance")
	}
	if os.Getenv("LC_BROWSER_SETTINGS_GREP") == "" && !regexp.MustCompile(`(?m)^12 passed$`).MatchString(summary) {
		t.Fatal("full browser mode requires exactly 12 passed cases")
	}
	t.Logf("BROWSER MOCK business + REAL_PG signed authorization; evidence=%s", evidence)
}

func blsSourceManifest(t *testing.T, root, evidence string) {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = root
	head, err := cmd.Output()
	if err != nil {
		t.Fatal("source SHA unavailable")
	}
	hashes := map[string]string{}
	for _, path := range []string{"tests/admin/live-settings.spec.ts", "tests/foundation/browser_live_settings_test.go", "tests/foundation/browser_live_settings_fixture_test.go", "playwright.config.ts"} {
		raw, readErr := os.ReadFile(filepath.Join(root, path))
		if readErr != nil {
			t.Fatal("source hash unavailable")
		}
		hashes[path] = consoleHash(raw)
	}
	raw, err := json.MarshalIndent(map[string]any{"git_commit": strings.TrimSpace(string(head)), "source_sha256": hashes, "evidence_class": "BROWSER MOCK business + REAL_PG signed authorization", "provider_live": "NOT_RUN"}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(evidence, "source.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
}
func blsSummary(raw []byte) string {
	lines := []string{}
	for _, match := range regexp.MustCompile(inboxFailureCountPattern).FindAllSubmatch(raw, 30) {
		lines = append(lines, fmt.Sprintf("%s %s", match[1], match[2]))
	}
	for _, match := range regexp.MustCompile(`(?m)^[\t ]*[0-9]+\)[\t ]+.*live-settings\.spec\.ts:([0-9]{1,6}):([0-9]{1,6})[\t ]+›`).FindAllSubmatch(raw, 30) {
		lines = append(lines, fmt.Sprintf("failed live-settings.spec.ts:%s:%s", match[1], match[2]))
	}
	if len(lines) == 0 {
		return "No structured Playwright summary; raw private output withheld.\n"
	}
	return strings.Join(lines, "\n") + "\n"
}
