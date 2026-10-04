//go:build browser

package foundation_test

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/httpapi"
	"livecommerce/internal/identity"
	"livecommerce/internal/identityhttp"
	"livecommerce/internal/oidclogin"
)

// STU04: a signed browser session crosses packaged Next, the ordinary Go API,
// task-owned PG18 and the real media-worker binary. The provider is local MOCK.
func TestBrowserStudioUIRealChain(t *testing.T) {
	if os.Getenv("LC_BROWSER_STUDIO_UI_ACCEPTANCE") != "1" || os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" {
		t.Fatal("use scripts/dev/test-local.sh --browser-studio-ui")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 330*time.Second)
	defer cancel()
	h, ca, stopsAllowed := studioProcessFixture(t)
	// The local TLS provider replies with h.plan.RoomName. Unlike STU03,
	// this browser test starts the worker before the HTTP Start request, so
	// bind the frozen prepared attempt's room before the provider can respond.
	preparedAttempt, ok := h.specification["attempt_id"].(string)
	if !ok || len(preparedAttempt) != 36 {
		t.Fatal("invalid prepared attempt fixture")
	}
	h.plan.RoomName = "lc_" + strings.ReplaceAll(preparedAttempt, "-", "")
	if len(h.plan.RoomName) != 35 {
		t.Fatal("invalid prepared room fixture")
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	_ = listener.Close()
	_, port, _ := net.SplitHostPort(addr)
	origin := "http://" + addr
	idp := newBrowserIDP(t, origin+"/api/auth/callback")
	mustExec(t, h.lp.f.owner, `INSERT INTO identity.external_identities(issuer,subject,principal_id) VALUES($1,'browser-subject',$2)`, idp.server.URL, h.lp.actor)
	t.Cleanup(func() {
		_, _ = h.lp.f.owner.Exec(context.Background(), `DELETE FROM identity.external_identities WHERE issuer=$1 AND subject='browser-subject'`, idp.server.URL)
		_, _ = h.lp.f.owner.Exec(context.Background(), `DELETE FROM identity.session_events WHERE session_id IN (SELECT id FROM identity.sessions WHERE principal_id=$1)`, h.lp.actor)
	})
	mustExec(t, h.lp.f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,'live:read')`, h.lp.f.tenantA, h.lp.f.storeA1, h.lp.limited)
	// the spec leaves Studio through nav-orders; the role-aware nav (0089, apps/admin/lib/team-model.ts) shows it only with orders:read
	var addedOrdersRead bool // removed again only if this test added it (the live-planning fixture is shared)
	if err := h.lp.f.owner.QueryRow(ctx, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,'orders:read') ON CONFLICT DO NOTHING RETURNING true`,
		h.lp.f.tenantA, h.lp.f.storeA1, h.lp.actor).Scan(&addedOrdersRead); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		t.Fatal(err)
	}
	if addedOrdersRead {
		t.Cleanup(func() {
			_, _ = h.lp.f.owner.Exec(context.Background(), `DELETE FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND permission='orders:read'`, h.lp.f.tenantA, h.lp.f.storeA1, h.lp.actor)
		})
	}
	expiredToken := randomToken()
	tx, err := h.lp.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := insertSession(ctx, tx, expiredToken, h.lp.actor, "merchant", time.Now().Add(-time.Minute), nil); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	_, _, authority := identityFixture(t)
	provider, err := oidclogin.New(ctx, oidclogin.Config{Issuer: idp.server.URL, ClientID: browserClientID, RedirectURL: idp.redirect, AllowLoopbackForTests: true})
	if err != nil {
		t.Fatal(err)
	}
	service, err := identity.New(authority, observedBrowserProvider{Provider: provider, t: t}, identity.Policy{
		ProviderKey: "browser-studio-ui-signed-mock-v1", SessionTTL: time.Hour, OnboardingEnabled: true, Currencies: []string{"TWD", "USD"},
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
	mux.Handle("/", httpapi.NewHandler(h.lp.f.runtime, httpapi.Options{SessionStoreList: true, Live: h.planner}))
	var studioCalls, badAuthority, faultCount atomic.Int64
	var armed atomic.Bool
	var faultKeysMu sync.Mutex
	var faultKeys []string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/__test/studio-ui-arm-fault" {
			if r.Method != http.MethodPost {
				http.Error(w, "method", http.StatusMethodNotAllowed)
				return
			}
			armed.Store(true)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.URL.Path == "/__test/studio-ui-expire-login" {
			if r.Method != http.MethodPost {
				http.Error(w, "method", http.StatusMethodNotAllowed)
				return
			}
			// Task-owned identity fixture only: expire the OIDC-issued login
			// while its Studio page is open, never a production/provider login.
			result, err := h.lp.f.owner.Exec(r.Context(), `UPDATE identity.sessions s SET expires_at=clock_timestamp()-interval '1 minute'
				WHERE s.id=(SELECT ev.session_id FROM identity.session_events ev JOIN identity.external_identities e ON e.principal_id=ev.principal_id
				WHERE ev.action='session.issued' AND e.issuer=$1 AND e.subject='browser-subject' ORDER BY ev.created_at DESC,ev.id DESC LIMIT 1)`, idp.server.URL)
			if err != nil || result.RowsAffected() != 1 {
				http.Error(w, "fixture_expiry_failed", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/v1/admin/stores/") && strings.Contains(r.URL.Path, "/live-sessions") {
			studioCalls.Add(1)
			if r.Header.Get("Cookie") != "" || r.Header.Get("X-Tenant-ID") != "" || r.Header.Get("X-Forwarded-Host") != "" ||
				r.Header.Get("Authorization") == "Bearer "+h.lp.token {
				badAuthority.Add(1)
			}
			if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/rehearsal/stop") {
				stopsAllowed.Store(true)
			}
			if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/live-sessions") {
				faultKeysMu.Lock()
				faultKeys = append(faultKeys, r.Header.Get("Idempotency-Key"))
				faultKeysMu.Unlock()
				if armed.CompareAndSwap(true, false) {
					// Explicit lost-ACK fixture: commit through the real API first,
					// then discard only its HTTP response. No normal route is stubbed.
					recorded := httptest.NewRecorder()
					mux.ServeHTTP(recorded, r)
					if recorded.Code != http.StatusCreated && recorded.Code != http.StatusOK {
						for key, values := range recorded.Header() {
							w.Header()[key] = values
						}
						w.WriteHeader(recorded.Code)
						_, _ = w.Write(recorded.Body.Bytes())
						return
					}
					faultCount.Add(1)
					w.Header().Set("Cache-Control", "private, no-store")
					http.Error(w, "response_lost_after_commit", http.StatusServiceUnavailable)
					return
				}
			}
		}
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(api.Close)
	// Fixture cleanup must learn the browser-created attempt before LME's
	// cleanup runs, including when the browser assertion fails.
	defer func() {
		if err := h.lp.f.owner.QueryRow(context.Background(), `SELECT a.id::text,a.start_operation_id::text,o.job_id,a.room_name,a.program_id::text
			FROM live.media_attempts a JOIN integration.operations o ON o.id=a.start_operation_id WHERE a.session_id=$1 ORDER BY a.created_at DESC LIMIT 1`, h.session).
			Scan(&h.plan.AttemptID, &h.plan.OperationID, &h.plan.JobID, &h.plan.RoomName, &h.plan.ProgramID); err == nil {
			h.plan.SessionID = h.session
		}
	}()
	evidence := filepath.Join(root, "output", "playwright", "studio-ui-"+time.Now().UTC().Format("20060102T150405.000000000"))
	if err := os.MkdirAll(evidence, 0700); err != nil {
		t.Fatal(err)
	}
	nextLog := browserLog(t, filepath.Join(evidence, "next.log"))
	next := exec.CommandContext(ctx, "node", filepath.Join(root, "apps/admin/.next/standalone/apps/admin/server.js"))
	next.Dir = root
	next.Env = browserEnvironment(map[string]string{"HOSTNAME": "127.0.0.1", "PORT": port, "NODE_ENV": "production", "COMMERCE_IDENTITY_ENABLED": "1", "COMMERCE_IDENTITY_ALLOW_LOOPBACK_TESTS": "1", "COMMERCE_PUBLIC_ORIGIN": origin, "COMMERCE_API_ORIGIN": api.URL, "COMMERCE_OIDC_ISSUER": idp.server.URL, "COMMERCE_BFF_KEY": bffKey, "COMMERCE_ONBOARDING_ENABLED": "1", "COMMERCE_ONBOARDING_CURRENCIES": "TWD,USD"})
	next.Stdout, next.Stderr = nextLog, nextLog
	if err := next.Start(); err != nil {
		t.Fatal(err)
	}
	nextDone := make(chan error, 1)
	go func() { nextDone <- next.Wait() }()
	t.Cleanup(func() {
		_ = next.Process.Kill()
		select {
		case <-nextDone:
		case <-time.After(5 * time.Second):
			t.Error("owned Next did not stop")
		}
	})
	ready := false
	for i := 0; i < 120; i++ {
		response, e := (&http.Client{Timeout: time.Second}).Get(origin + "/api/stores")
		if e == nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusUnauthorized {
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
		t.Fatalf("packaged Next not ready: %s", evidence)
	}
	workerBinary := mrBuild(t, "../../cmd/media-worker", "studio-ui-worker")
	workerName, executorName := "studio_ui_worker_"+t04Tag(), "studio_ui_executor_"+t04Tag()
	workerEnv := lmwEnvironment(h, h.server.Listener.Addr().String(), ca)
	workerEnv = lmwReplace(lmwReplace(workerEnv, "COMMERCE_MEDIA_WORKER_DATABASE_URL", mrNamedDSN(t, h.worker.Config().ConnString(), workerName)), "COMMERCE_MEDIA_EXECUTOR_DATABASE_URL", mrNamedDSN(t, h.executor.Config().ConnString(), executorName))
	worker := lmwProcess(t, workerBinary, "studio-ui-worker", workerEnv)
	mrReadyLog(t, worker, "media_worker_ready")
	playwrightLog := browserLog(t, filepath.Join(evidence, "playwright.log"))
	browser := exec.CommandContext(ctx, "pnpm", "exec", "playwright", "test", "tests/admin/studio-ui.spec.ts", "--reporter=list", "--output="+filepath.Join(evidence, "results"))
	browser.Dir = root
	browser.Env = browserEnvironment(map[string]string{
		"LC_BROWSER_SUITE": "studio-ui", "LC_BROWSER_PUBLIC_ORIGIN": origin, "LC_BROWSER_API_ORIGIN": api.URL,
		"LC_BROWSER_STUDIO_STORE": h.lp.f.storeA1, "LC_BROWSER_STUDIO_FOREIGN_STORE": h.lp.f.storeA2,
		"LC_BROWSER_STUDIO_UNLISTED_STORE": h.lp.f.storeB, "LC_BROWSER_STUDIO_SESSION": h.session,
		"LC_BROWSER_STUDIO_READONLY_TOKEN": h.lp.limitedToken, "LC_BROWSER_STUDIO_EXPIRED_TOKEN": expiredToken,
		"LC_BROWSER_EVIDENCE": evidence,
	})
	browser.Stdout, browser.Stderr = playwrightLog, playwrightLog
	browserErr := browser.Run()
	mrStop(t, worker, syscall.SIGTERM, true)
	if studioCalls.Load() < 12 || badAuthority.Load() != 0 || faultCount.Load() != 3 || h.starts.Load() != 1 || h.stops.Load() != 1 {
		t.Errorf("real chain counters calls=%d authority=%d lost_ack=%d starts=%d stops=%d evidence=%s", studioCalls.Load(), badAuthority.Load(), faultCount.Load(), h.starts.Load(), h.stops.Load(), evidence)
	}
	faultKeysMu.Lock()
	keys := append([]string(nil), faultKeys...)
	faultKeysMu.Unlock()
	if len(keys) != 5 || keys[1] == "" || keys[1] != keys[2] || keys[3] == keys[1] || keys[4] == keys[1] || keys[4] == keys[3] {
		t.Errorf("lost-ACK retry changed command key: requests=%d evidence=%s", len(keys), evidence)
	}
	var duplicateCount int
	if err := h.lp.f.owner.QueryRow(ctx, `SELECT count(*) FROM live.sessions WHERE tenant_id=$1 AND store_id=$2 AND title='STU04 lost ACK scene'`, h.lp.f.tenantA, h.lp.f.storeA1).Scan(&duplicateCount); err != nil || duplicateCount != 1 {
		t.Errorf("lost-ACK effect count=%d err=%v evidence=%s", duplicateCount, err, evidence)
	}
	if err := h.lp.f.owner.QueryRow(ctx, `SELECT count(*) FROM live.sessions WHERE tenant_id=$1 AND store_id=$2 AND title='STU04 swapped login scene'`, h.lp.f.tenantA, h.lp.f.storeA1).Scan(&duplicateCount); err != nil || duplicateCount != 1 {
		t.Errorf("swapped-login effect count=%d err=%v evidence=%s", duplicateCount, err, evidence)
	}
	if err := h.lp.f.owner.QueryRow(ctx, `SELECT count(*) FROM live.sessions WHERE tenant_id=$1 AND store_id=$2 AND title='STU04 native uncertain scene'`, h.lp.f.tenantA, h.lp.f.storeA1).Scan(&duplicateCount); err != nil || duplicateCount != 1 {
		t.Errorf("native uncertain committed effect count=%d err=%v evidence=%s", duplicateCount, err, evidence)
	}
	var scheduled time.Time
	if err := h.lp.f.owner.QueryRow(ctx, `SELECT scheduled_at FROM live.sessions WHERE tenant_id=$1 AND store_id=$2 AND title='STU04 phone-edited scene'`, h.lp.f.tenantA, h.lp.f.storeA1).Scan(&scheduled); err != nil || scheduled.UTC().Format(time.RFC3339) != "2030-01-01T00:00:00Z" {
		t.Errorf("UTC schedule shifted on title edit: instant=%s err=%v evidence=%s", scheduled.UTC().Format(time.RFC3339), err, evidence)
	}
	var issued int
	if err := h.lp.f.owner.QueryRow(ctx, `SELECT count(*) FROM identity.sessions s JOIN identity.session_events ev ON ev.session_id=s.id AND ev.action='session.issued' JOIN identity.external_identities e ON e.principal_id=s.principal_id WHERE e.issuer=$1 AND e.subject='browser-subject' AND s.token_hash<>$2`, idp.server.URL, tokenHash(h.lp.token)).Scan(&issued); err != nil || issued != 5 { // 5: STU04 x2 native + swapped login + the main case, and STU05 (bare Studio route, D02)
		t.Errorf("signed browser login count=%d err=%v evidence=%s", issued, err, evidence)
	}
	var resource string
	if err := h.lp.f.owner.QueryRow(ctx, `SELECT resource_state FROM live.media_execution_state WHERE attempt_id=(SELECT id FROM live.media_attempts WHERE session_id=$1 ORDER BY created_at DESC LIMIT 1)`, h.session).Scan(&resource); err != nil || resource != "TERMINAL" {
		t.Errorf("worker terminal readback=%q err=%v evidence=%s", resource, err, evidence)
	}
	if raw, err := os.ReadFile(worker.logPath); err != nil || bytes.Contains(raw, []byte(h.lp.token)) || bytes.Contains(raw, []byte(h.streamURL)) {
		t.Errorf("worker log secret/read error: %v", err)
	}
	t.Logf("STU04 signed UI, BFF, Go/PG and MOCK worker readbacks completed; evidence=%s", evidence)
	if browserErr != nil {
		log, err := os.ReadFile(playwrightLog.Name())
		nativeFailed := 0
		for _, line := range strings.Split(string(log), "\n") {
			if strings.Contains(line, "✘") && (strings.Contains(line, "STU04 native visibility conceal and revalidation remains required") ||
				strings.Contains(line, "STU04 native conceal retains an uncertain committed request")) {
				nativeFailed++
			}
		}
		if err == nil && !t.Failed() && ((nativeFailed == 2 && bytes.Contains(log, []byte("3 passed")) && bytes.Contains(log, []byte("2 failed"))) ||
			(nativeFailed == 1 && bytes.Contains(log, []byte("4 passed")) && bytes.Contains(log, []byte("1 failed")))) {
			t.Errorf("STU04 native visibility lifecycle NOT_RUN: see exact browser failure(s); other browser cases and PG/worker readbacks passed; evidence=%s", evidence)
		} else {
			t.Errorf("STU04 browser chain failed: %v; evidence=%s", browserErr, evidence)
		}
	}
}
