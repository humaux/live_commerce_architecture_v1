//go:build browser

// Purpose: LC-U2b real inbox browser acceptance over signed MOCK OIDC, Next, Go and isolated PG.
// Depends on: lbSetup signed inbound/fake Graph, real inbox service, metaconnect.Health and inbox-ui.spec.ts.
// Used by: --browser-inbox on GitHub runners; no LIVE Meta or production acceptance.
// Invariants: failure logs expose only INU IDs, source coordinates and counts; evidence stays in the CI upload root.
package foundation_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"livecommerce/internal/claims"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/identity"
	"livecommerce/internal/identityhttp"
	"livecommerce/internal/metaconnect"
	"livecommerce/internal/oidclogin"
)

const inboxFailureCasePattern = `(?m)^[\t ]*[0-9]+\)[\t ]+(?:tests/admin/)?inbox-ui\.spec\.ts:([0-9]{1,6}):([0-9]{1,6})[\t ]+›[\t ]+(INU[0-9]{2})\b`
const inboxFailureCountPattern = `(?m)^[\t ]*([0-9]{1,6}) (failed|passed|skipped|timed out|interrupted)\b`

// inboxPlaywrightFailureSummary discards titles, assertions and DOM text before anything enters the Go/CI log.
func inboxPlaywrightFailureSummary(output []byte) []string {
	lines := []string{}
	seen := map[string]bool{}
	add := func(line string) {
		if !seen[line] && len(lines) < 20 {
			seen[line] = true
			lines = append(lines, line)
		}
	}
	for _, match := range regexp.MustCompile(inboxFailureCountPattern).FindAllSubmatch(output, 20) {
		add(fmt.Sprintf("%s %s", match[1], match[2]))
	}
	for _, match := range regexp.MustCompile(inboxFailureCasePattern).FindAllSubmatch(output, 20) {
		add(fmt.Sprintf("failed %s (inbox-ui.spec.ts:%s:%s)", match[3], match[1], match[2]))
	}
	return lines
}

// TestBrowserInboxUIRealChain runs real UI writes, authority refusals and native hide/revalidation.
func TestBrowserInboxUIRealChain(t *testing.T) {
	if os.Getenv("LC_BROWSER_INBOX_ACCEPTANCE") != "1" || os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" {
		t.Fatal("use scripts/dev/test-local.sh --browser-inbox on CI")
	}
	calibration := os.Getenv("LC_INBOX_CALIBRATION")
	if calibration != "" && calibration != "retain-thread" {
		t.Fatal("unsupported LC_INBOX_CALIBRATION")
	}
	t.Run("failure_diagnostics", func(t *testing.T) {
		const private = "SYNTHETIC_PRIVATE_DM_NAME_PSID"
		output := []byte("  1) tests/admin/inbox-ui.spec.ts:450:1 › INU05 hidden thread " + private + " INU99 inbox-ui.spec.ts:999:9 › INU98\n    Error: " + private + "\n    1 failed " + private + "\n    7 passed (2m)\n  2) tests/admin/other.spec.ts:8:1 › INU99 " + private + "\n")
		want := "1 failed\n7 passed\nfailed INU05 (inbox-ui.spec.ts:450:1)"
		if got := strings.Join(inboxPlaywrightFailureSummary(output), "\n"); got != want {
			t.Fatal("failure summary differs or admits private text")
		}
		var many strings.Builder
		for n := 1; n <= 30; n++ {
			fmt.Fprintf(&many, "  %d) tests/admin/inbox-ui.spec.ts:%d:1 › INU05\n", n, n)
		}
		if len(inboxPlaywrightFailureSummary([]byte(many.String()))) != 20 {
			t.Fatal("failure summary must be bounded to 20 records")
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	e := lbSetup(t)
	f := e.h.f
	// Use the actual role catalogue rather than reproducing its grants in TypeScript.
	rolePerson := func(role string) (string, string) {
		var permissions []string
		if err := f.owner.QueryRow(ctx, `SELECT identity.staff_role_permissions($1)`, role).Scan(&permissions); err != nil {
			t.Fatal(err)
		}
		filtered := []string{}
		for _, permission := range permissions {
			if permission != "store:read" {
				filtered = append(filtered, permission)
			}
		}
		return lcPerson(t, f, f.tenantA, f.storeA1, filtered...)
	}
	operator, _ := rolePerson("live_operator")
	_, viewerToken := rolePerson("viewer")
	_, readerToken := lcPerson(t, f, f.tenantA, f.storeA1, "inbox:read")
	for _, permission := range []string{"store:read", "inbox:read", "inbox:reply"} {
		mustExec(t, f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, f.tenantA, f.storeA2, operator, permission)
	}
	// These MOCK capability snapshots are fixture setup, not a production probe result.
	for provider, binding := range map[string]string{"facebook": e.pageBinding, "instagram": e.igBinding} {
		mustExec(t, f.owner, `INSERT INTO integration.binding_capabilities(tenant_id,store_id,page_id,binding_id,provider,capability,state,reason,evidence,checked_at)
			VALUES($1,$2,$3,$4,$5,'dm_session','ok','fixture_ok','MOCK',clock_timestamp()) ON CONFLICT (tenant_id,store_id,binding_id,capability) DO UPDATE SET state='ok',reason='fixture_ok',evidence='MOCK'`, f.tenantA, f.storeA1, e.pageID, binding, provider)
	}
	const dm = "SYNTHETIC-INBOX-DM-PRIVATE-7c32"
	const name = "SYNTHETIC-INBOX-NAME-7c32"
	const psid = "900007320001"
	// Sender name is carried by the signed encrypted ingress; never assigned by a fake BFF.
	raw := []byte(fmt.Sprintf(`{"object":"page","entry":[{"id":%q,"time":123,"messaging":[{"sender":{"id":%q,"name":%q},"recipient":{"id":%q},"timestamp":%d,"message":{"mid":%q,"text":%q}}]}]}`,
		e.pageAsset, psid, name, e.pageAsset, time.Now().UnixMilli(), "m."+randomUUID(), dm))
	event := mcPost(t, e.page, e.pageAsset, raw)
	mcAwait(t, e.page, event)
	ids := map[string]string{"open": e.convOf(t, "page", e.pageAsset, psid)}
	ids["closed"] = e.postDM(t, "900007320002", "SYNTHETIC-CLOSED-WINDOW", time.Now().Add(-25*time.Hour))
	ids["instagram"] = e.postIGDM(t, "900007320003", "SYNTHETIC-INSTAGRAM-DM", time.Now())
	ids["retry"] = e.postDM(t, "900007320004", "SYNTHETIC-INBOX-RETRY-INBOUND", time.Now())
	ids["foreign"] = lcConversation(t, f, f.tenantA, f.storeA2, "page")
	// Isolate recovery from mciSetup's unused claim window; 0122 permits several OPEN windows per store.
	e.h.closeWindow(t, e.session)
	// Real commands create the bundle; only its unavailable-private-reply flag is owner fixture setup.
	ids["bundle_session"] = e.h.draft(t, f.storeA1)
	e.h.offer(t, ids["bundle_session"], "BR1", e.h.stock.skus[0].ID, 1)
	e.h.open(t, ids["bundle_session"], claims.MatchExact)
	ids["bundle"] = e.h.accepted(t, ids["bundle_session"], "", "SYNTHETIC-BUNDLE-BUYER", "BR1+1").BundleID
	mustExec(t, f.owner, `UPDATE claims.bundles SET link_pending_manual=true WHERE tenant_id=$1 AND store_id=$2 AND id=$3`, f.tenantA, f.storeA1, ids["bundle"])
	const bundleOrigin = "https://inbox-bundle.example"
	bhPublish(t, bcHarness{cqHarness: e.h.cqHarness}, bundleOrigin, f.tenantA, f.storeA1)
	// Match command.Run's canonical IssueLink request; no credential is part of this receipt digest.
	linkRequest, err := json.Marshal(struct {
		PrincipalID        string `json:"principal_id"`
		SessionID          string `json:"session_id"`
		BundleID           string `json:"bundle_id"`
		ExpectedGeneration int64  `json:"expected_generation"`
		ReleaseBinding     bool   `json:"release_binding"`
	}{operator, ids["bundle_session"], ids["bundle"], 0, false})
	if err != nil {
		t.Fatal("bundle fixture request encoding failed")
	}
	linkRequestHash := sha256.Sum256(linkRequest)
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	evidence := filepath.Join(root, "output/playwright/inbox-ui", time.Now().UTC().Format("20060102T150405.000000000"))
	if err := os.MkdirAll(evidence, 0700); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	_ = listener.Close()
	_, port, _ := net.SplitHostPort(address)
	origin := "http://" + address
	idp := newBrowserIDP(t, origin+"/api/auth/callback")
	mustExec(t, f.owner, `INSERT INTO identity.external_identities(issuer,subject,principal_id) VALUES($1,'browser-subject',$2)`, idp.server.URL, operator)
	_, _, authority := identityFixture(t)
	provider, err := oidclogin.New(ctx, oidclogin.Config{Issuer: idp.server.URL, ClientID: browserClientID, RedirectURL: idp.redirect, AllowLoopbackForTests: true})
	if err != nil {
		t.Fatal(err)
	}
	service, err := identity.New(authority, observedBrowserProvider{Provider: provider, t: t}, identity.Policy{ProviderKey: "browser-inbox-v1", SessionTTL: time.Hour, OnboardingEnabled: true, Currencies: []string{"TWD", "USD"}})
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
	mux.Handle("/", httpapi.NewHandler(f.runtime, httpapi.Options{SessionStoreList: true, Studio: true, ClaimLabels: &e.h.labels, Inbox: e.svc, MetaHealth: &metaconnect.Health{}}))
	var reads, writes, stripped atomic.Int64
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/__test/inbox-bundle-facts" {
			// Read-only claims.links/ops.command_results observation. Digests are never returned or logged.
			var input struct {
				CredentialSHA256 string `json:"credentialSHA256"`
				IdempotencyKey   string `json:"idempotencyKey"`
			}
			decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
			decoder.DisallowUnknownFields()
			if decoder.Decode(&input) != nil || (input.CredentialSHA256 != "" && !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(input.CredentialSHA256)) ||
				(input.IdempotencyKey != "" && !regexp.MustCompile(`^[A-Za-z0-9_-]{8,128}$`).MatchString(input.IdempotencyKey)) {
				http.Error(w, "fixture observation invalid", 400)
				return
			}
			var links, generation, receipts, dmOperations int64
			var hashMatches, principalMatches, ttlValid, receiptValid, pendingManual bool
			err := f.owner.QueryRow(r.Context(), `SELECT
			 (SELECT count(*) FROM claims.links WHERE tenant_id=$1 AND store_id=$2 AND bundle_id=$3::uuid),
			 coalesce(l.generation,0),coalesce(encode(l.token_hash,'hex')=$4 AND $4<>'',false),
			 coalesce(l.principal_id=$5::uuid,false),
			 coalesce(l.expires_at>clock_timestamp() AND l.expires_at-l.issued_at=interval '72 hours',false),
			 (SELECT count(*) FROM ops.command_results c WHERE c.tenant_id=$1 AND c.store_id=$2 AND c.operation='live.claim.link.issue' AND c.response->>'bundle_id'=$3::uuid::text),
			 EXISTS(SELECT 1 FROM ops.command_results c WHERE c.tenant_id=$1 AND c.store_id=$2 AND c.operation='live.claim.link.issue'
			   AND c.idempotency_key=$6 AND c.principal_id=$5::uuid AND c.request_hash=$7::bytea
			   AND c.response->>'bundle_id'=$3::uuid::text AND (c.response->>'generation')::bigint=l.generation
			   AND (c.response->>'expires_at')::timestamptz=l.expires_at AND c.response->>'released'='false'
			   AND c.response - ARRAY['bundle_id','generation','expires_at','released']::text[] = '{}'::jsonb),
			 b.link_pending_manual,
			 (SELECT count(*) FROM integration.operations WHERE tenant_id=$1 AND store_id=$2 AND action='meta.dm_send')
			 FROM claims.bundles b LEFT JOIN claims.links l ON l.tenant_id=b.tenant_id AND l.store_id=b.store_id AND l.bundle_id=b.id
			 WHERE b.tenant_id=$1 AND b.store_id=$2 AND b.id=$3::uuid`,
				f.tenantA, f.storeA1, ids["bundle"], input.CredentialSHA256, operator, input.IdempotencyKey, linkRequestHash[:]).
				Scan(&links, &generation, &hashMatches, &principalMatches, &ttlValid, &receipts, &receiptValid, &pendingManual, &dmOperations)
			if err != nil {
				http.Error(w, "fixture observation failed", 500)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-store")
			_ = json.NewEncoder(w).Encode(map[string]any{"links": links, "generation": generation, "hash_matches": hashMatches,
				"principal_matches": principalMatches, "ttl_valid": ttlValid, "receipt_count": receipts, "receipt_valid": receiptValid,
				"pending_manual": pendingManual, "dm_operations": dmOperations})
			return
		}
		if r.Method == "GET" && r.URL.Path == "/__test/inbox-facts" {
			// Read-only PG observation is evidence; it cannot substitute for the UI action.
			conversation := ids["open"]
			if r.URL.Query().Get("conversation") == "retry" {
				conversation = ids["retry"]
			}
			var readSeq, generation, outbounds int64
			var mode string
			err := f.owner.QueryRow(r.Context(), `SELECT read_seq,takeover_generation,mode,(SELECT count(*) FROM inbox.outbound_messages WHERE conversation_id=$1) FROM inbox.conversation_state WHERE conversation_id=$1`, conversation).Scan(&readSeq, &generation, &mode, &outbounds)
			if err != nil {
				http.Error(w, "fixture read failed", 500)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"read_seq": readSeq, "generation": generation, "mode": mode, "outbounds": outbounds, "reads": reads.Load(), "writes": writes.Load()})
			return
		}
		if strings.Contains(r.URL.Path, "/inbox/") {
			if r.Method == "GET" {
				reads.Add(1)
			} else {
				writes.Add(1)
			}
			if r.Header.Get("Cookie") != "" || r.Header.Get("X-Tenant-ID") != "" || r.Header.Get("X-Forwarded-Host") != "" {
				stripped.Add(1)
			}
			for _, sentinel := range []string{dm, name, psid} {
				if strings.Contains(r.URL.String(), sentinel) {
					http.Error(w, "private data in URL", 500)
					return
				}
			}
		}
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(api.Close)
	nextLog := browserLog(t, filepath.Join(evidence, "next.log"))
	next := exec.CommandContext(ctx, "node", filepath.Join(root, "apps/admin/.next/standalone/apps/admin/server.js"))
	next.Dir = root
	// Signed identity forbids the generic fixture bypass. The UI's calibrated defect requires these exact loopback gate guards.
	next.Env = browserEnvironment(map[string]string{"HOSTNAME": "127.0.0.1", "PORT": port, "NODE_ENV": "production", "COMMERCE_IDENTITY_ENABLED": "1", "COMMERCE_IDENTITY_ALLOW_LOOPBACK_TESTS": "1", "COMMERCE_PUBLIC_ORIGIN": origin, "COMMERCE_API_ORIGIN": api.URL, "COMMERCE_OIDC_ISSUER": idp.server.URL, "COMMERCE_BFF_KEY": bffKey, "COMMERCE_ONBOARDING_ENABLED": "1", "COMMERCE_ONBOARDING_CURRENCIES": "TWD,USD", "COMMERCE_FIXTURE_ENABLED": "0", "LC_BROWSER_INBOX_ACCEPTANCE": "1", "LC_INBOX_CALIBRATION": calibration})
	next.Stdout, next.Stderr = nextLog, nextLog
	if err := next.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- next.Wait() }()
	t.Cleanup(func() {
		_ = next.Process.Kill()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("owned Next process did not stop")
		}
	})
	client := &http.Client{Timeout: time.Second}
	ready := false
	for attempt := 0; attempt < 100; attempt++ {
		response, err := client.Get(origin + "/api/stores")
		if err == nil {
			_ = response.Body.Close()
			ready = response.StatusCode == http.StatusUnauthorized
		}
		if ready {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("Next readiness timeout")
		case <-time.After(100 * time.Millisecond):
		}
	}
	if !ready {
		t.Fatalf("Next readiness failed; evidence=%s", evidence)
	}
	fixtureJSON, _ := json.Marshal(ids)
	playwrightLog := filepath.Join(evidence, "playwright.log")
	log := browserLog(t, playwrightLog)
	browser := exec.CommandContext(ctx, "pnpm", "exec", "playwright", "test", "tests/admin/inbox-ui.spec.ts", "--reporter=list", "--output="+filepath.Join(evidence, "results"))
	browser.Dir = root
	browser.Env = browserEnvironment(map[string]string{"LC_BROWSER_SUITE": "inbox", "LC_BROWSER_PUBLIC_ORIGIN": origin, "LC_BROWSER_API_ORIGIN": api.URL, "LC_BROWSER_EVIDENCE": evidence, "LC_BROWSER_INBOX_STORE": f.storeA1, "LC_BROWSER_INBOX_OTHER_STORE": f.storeA2, "LC_BROWSER_INBOX_IDS": string(fixtureJSON), "LC_BROWSER_INBOX_READER_TOKEN": readerToken, "LC_BROWSER_INBOX_VIEWER_TOKEN": viewerToken, "LC_INBOX_CALIBRATION": calibration, "FORCE_COLOR": "0", "NO_COLOR": "1"})
	browser.Stdout, browser.Stderr = log, log
	runErr := browser.Run()
	closeErr := log.Close()
	if runErr != nil {
		if output, err := os.ReadFile(playwrightLog); err == nil {
			summary := inboxPlaywrightFailureSummary(output)
			if len(summary) == 0 {
				t.Log("Playwright has no case summary; inspect uploaded evidence")
			}
			for _, line := range summary {
				t.Logf("Playwright %s", line)
			}
		} else {
			t.Log("Playwright summary unavailable; inspect uploaded evidence")
		}
		t.Fatalf("inbox browser failed: %v; evidence=%s", runErr, evidence)
	}
	if closeErr != nil {
		t.Fatal("Playwright evidence log could not be closed")
	}
	if reads.Load() < 5 || writes.Load() < 4 || stripped.Load() != 0 {
		t.Fatalf("BFF real-chain proof reads=%d writes=%d stripped=%d", reads.Load(), writes.Load(), stripped.Load())
	}
	// The copied credential must never appear in logs, DOM screenshots or retained traces.
	var persistedHash []byte
	if err := f.owner.QueryRow(ctx, `SELECT token_hash FROM claims.links WHERE tenant_id=$1 AND store_id=$2 AND bundle_id=$3`, f.tenantA, f.storeA1, ids["bundle"]).Scan(&persistedHash); err != nil {
		t.Fatal("bundle UI did not persist a link")
	}
	// No raw hash is logged: compare candidate credential digests inside the process only.
	credentialPattern := regexp.MustCompile(`[A-Za-z0-9_-]{43}`)
	if err := filepath.WalkDir(evidence, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(string(data), hex.EncodeToString(persistedHash)) {
			return fmt.Errorf("credential digest retained")
		}
		for _, candidate := range credentialPattern.FindAll(data, -1) {
			digest := sha256.Sum256(candidate)
			if hex.EncodeToString(digest[:]) == hex.EncodeToString(persistedHash) {
				return fmt.Errorf("credential retained")
			}
		}
		return nil
	}); err != nil {
		t.Fatal("I11 bundle evidence boundary failed")
	}
	// Each clicked submit flow (including committed-but-lost acknowledgment) creates exactly one operation.
	for _, key := range []string{"open", "retry"} {
		var count int
		var operation string
		if err := f.owner.QueryRow(ctx, `SELECT count(*),min(operation_id::text) FROM inbox.outbound_messages WHERE conversation_id=$1`, ids[key]).Scan(&count, &operation); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("%s UI flow planned %d sends, expected exactly one", key, count)
		}
		e.run(t, operation)
		e.awaitOp(t, operation, "SUCCEEDED", 10*time.Second, "completed")
	}
	if e.g.count() != 2 {
		t.Fatalf("fake Graph send count=%d", e.g.count())
	}
	for index, expected := range []string{psid, "900007320004"} {
		recipient, ok := e.g.all()[index].body["recipient"].(map[string]any)
		if !ok || recipient["id"] != expected {
			t.Fatal("fake Graph recipient differs from the signed inbound conversation")
		}
	}
	for _, filename := range []string{"next.log", "browser-console.json"} {
		data, err := os.ReadFile(filepath.Join(evidence, filename))
		if err != nil {
			t.Fatal(err)
		}
		for _, sentinel := range []string{dm, name, psid, "SYNTHETIC-INBOX-REPLY-7c32", "SYNTHETIC-INBOX-ACK-LOST-7c32"} {
			if strings.Contains(string(data), sentinel) {
				t.Fatalf("I11 leak in %s", filename)
			}
		}
	}
	t.Logf("BROWSER MOCK real inbox chain checked; evidence=%s", evidence)
}
