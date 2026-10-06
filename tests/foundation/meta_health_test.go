package foundation_test

// meta_health_test.go: MOCK author-smoke gates of unit w1-01b-meta-health (contracts/meta-connection-health-v1.md §11.2 MCH01-MCH12,
// backend only). Each test connects a synthetic Page into tenant A / store A1 (owner SQL: bindings, sealed Page credential, head,
// meta_connections — the trigger then creates the probe row due now), then runs the claims-worker probe sweep against a dedicated
// fake Graph that serves exactly the three read-only calls P1 (token) / P2 (permissions) / P3 (subscribed_apps). Nothing here calls
// Meta: the Graph base is the fake's loopback URL, so the probe starts rows at evidence MOCK (metareply.NewProber §3.4).
//
// Owns: MCH02 (probe derives/records; token-invalid/page-gone flip; permission/subscription), MCH03 (rate-limit/unknown keep
// state, probe_failing warning), MCH05 (missing credential -> unknown), MCH06 (token never in a URL or persisted surface), MCH07
// (B1/B2 banner routes), MCH08 (TableReader == SnapshotReader on an empty table; table rows win; binding_capability_state), MCH09
// (reconnect resets the probe and clears probed rows). MCH01/MCH04 (reader unit rules), MCH10/MCH11 (notify worker) and MCH12
// (LIVE probe) live elsewhere or are NOT_RUN — see docs/delivery/GATES.md.
//
// The owner (superuser) pool is used only for synthetic setup and read-back of columns no runtime role may read; every assertion
// the product surface owns goes through commerce_runtime (B1/B2, the readers) or the claims-worker authority (the probe).
//
// Evidence label: MOCK (REAL_PG + fake Graph). Run: bash scripts/dev/test-focused.sh '^TestMetaHealth' (the test-local.sh
// --meta-health mode).
//
// Status: MOCK.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/httpapi"
	"livecommerce/internal/integrations/meta/pagetoken"
	"livecommerce/internal/integrations/metareply"
	"livecommerce/internal/metaconnect"
	"livecommerce/internal/platform"
)

// mhVersion is the Graph version the probe is built with; the fake asserts it in every path.
const mhVersion = "v99.0"

// mhScopes is the connected Page's granted scopes: every §6 capability's permission, so only the Graph reading (P2/P3) or the
// custody can demote a capability in the happy paths.
var mhScopes = []string{"pages_read_engagement", "pages_messaging", "pages_manage_engagement", "instagram_manage_comments", "instagram_manage_messages"}

// mhConfig is the reader/probe ReaderConfig with all required permissions under Advanced Access and LC-U11 closed, so a granted
// permission derives ok (not review_required) and dm_session may derive ok.
func mhConfig() metaconnect.ReaderConfig {
	return metaconnect.ReaderConfig{
		AdvancedAccess: map[string]bool{
			"pages_read_engagement":     true,
			"pages_messaging":           true,
			"pages_manage_engagement":   true,
			"instagram_manage_comments": true,
			"instagram_manage_messages": true,
		},
		DMConfirmed: true,
	}
}

// mhPage is one connected Page the harness owns, with its sealed FB token (sentinel, split only inside the token string).
type mhPage struct {
	pageID, pageName string
	fbBinding        string
	igBinding        string
	token            string
}

// mhCap is one integration.binding_capabilities row read back by the owner pool.
type mhCap struct {
	binding, provider, capability, state, reason, evidence string
	checkedAt                                              *time.Time
}

// mhProbe is the integration.meta_health_probes row read back by the owner pool.
type mhProbe struct {
	severity, lastOutcome, permSource string
	generation, episode               int64
	consecutive                       int
	checkedAt, episodeClosedAt        *time.Time
}

// mhReq is one fake-Graph request the probe made (the token legitimately appears only in Authorization).
type mhReq struct{ method, path, rawQuery, auth string }

// mhGraph is the dedicated probe fake Graph: P1 GET /v99.0/{page}?fields=id, P2 GET /v99.0/me/permissions, P3
// GET /v99.0/{page}/subscribed_apps. It refuses nothing, but it flags any access_token in a URL and records every request so
// MCH06 can scan them. Default responses model a fully healthy Page; a test flips one call with setP*.
type mhGraph struct {
	srv                          *httptest.Server
	mu                           sync.Mutex
	reqs                         []mhReq
	leaked                       bool
	p1Status, p2Status, p3Status int
	p1Body, p2Body, p3Body       string
}

func newMhGraph(t *testing.T, appID string) *mhGraph {
	g := &mhGraph{p1Status: http.StatusOK, p2Status: http.StatusOK, p3Status: http.StatusOK}
	g.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.mu.Lock()
		if strings.Contains(r.URL.RawQuery, "access_token") || strings.Contains(r.URL.Path, "access_token") {
			g.leaked = true
		}
		g.reqs = append(g.reqs, mhReq{r.Method, r.URL.Path, r.URL.RawQuery, r.Header.Get("Authorization")})
		p1Status, p1Body := g.p1Status, g.p1Body
		p2Status, p2Body := g.p2Status, g.p2Body
		p3Status, p3Body := g.p3Status, g.p3Body
		g.mu.Unlock()

		const vp = "/" + mhVersion + "/"
		switch {
		case r.URL.Path == vp+"me/permissions":
			if p2Body == "" {
				p2Body = mhPermsBody
			}
			mhWrite(w, p2Status, p2Body)
		case strings.HasSuffix(r.URL.Path, "/subscribed_apps"):
			if p3Body == "" {
				p3Body = fmt.Sprintf(`{"data":[{"id":%q,"subscribed_fields":[{"name":"feed"},{"name":"messages"}]}]}`, appID)
			}
			mhWrite(w, p3Status, p3Body)
		default: // P1: GET /v99.0/{page}
			page := strings.TrimPrefix(r.URL.Path, vp)
			if p1Body == "" {
				p1Body = fmt.Sprintf(`{"id":%q}`, page)
			}
			mhWrite(w, p1Status, p1Body)
		}
	}))
	t.Cleanup(g.srv.Close)
	return g
}

func (g *mhGraph) url() string { return g.srv.URL }

func (g *mhGraph) setP1(status int, body string) {
	g.mu.Lock()
	g.p1Status, g.p1Body = status, body
	g.mu.Unlock()
}
func (g *mhGraph) setP2(status int, body string) {
	g.mu.Lock()
	g.p2Status, g.p2Body = status, body
	g.mu.Unlock()
}
func (g *mhGraph) setP3(status int, body string) {
	g.mu.Lock()
	g.p3Status, g.p3Body = status, body
	g.mu.Unlock()
}

func (g *mhGraph) sawURLLeak() bool { g.mu.Lock(); defer g.mu.Unlock(); return g.leaked }
func (g *mhGraph) requests() []mhReq {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := make([]mhReq, len(g.reqs))
	copy(out, g.reqs)
	return out
}

// mhPermsBody is the healthy P2 body: every scope granted.
const mhPermsBody = `{"data":[{"permission":"pages_read_engagement","status":"granted"},{"permission":"pages_messaging","status":"granted"},{"permission":"pages_manage_engagement","status":"granted"},{"permission":"instagram_manage_comments","status":"granted"},{"permission":"instagram_manage_messages","status":"granted"}]}`

func mhWrite(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}

// mhErr is a Graph error envelope (P1/P2/P3); subcode 0 omits error_subcode.
func mhErr(code, subcode int) (int, string) {
	if subcode != 0 {
		return http.StatusBadRequest, fmt.Sprintf(`{"error":{"message":"synthetic","type":"OAuthException","code":%d,"error_subcode":%d}}`, code, subcode)
	}
	return http.StatusBadRequest, fmt.Sprintf(`{"error":{"message":"synthetic","type":"OAuthException","code":%d}}`, code)
}

// mhEnv is the meta-health harness: fixture + a merchant principal (store:read/integration:read/integration:manage on store A1),
// the v1/v2 Page-token custody, the claims-worker pool that NewProber validated, the probe, and the fake Graph.
type mhEnv struct {
	t         *testing.T
	f         *testFixture
	v1        *metareply.PageTokenKeyring
	seal      *pagetoken.SealKeys
	probe     *metareply.Prober
	worker    *pgxpool.Pool
	fake      *mhGraph
	tenant    string
	store     string
	principal string
	token     string
	pages     []mhPage
}

func mhSetup(t *testing.T) *mhEnv {
	t.Helper()
	f := fixture(t)
	principal, token := lcPrincipal(t, f, f.tenantA, []string{f.storeA1}, "store:read", "integration:read", "integration:manage")
	v1, err := metareply.NewPageTokenKeyring("pt_key_1", map[string][]byte{"pt_key_1": randomBytes(32)})
	if err != nil {
		t.Fatal("v1 keyring", err)
	}
	seal, open := mcnPageRing(t)
	fake := newMhGraph(t, miApp)
	worker := miPool(t, f, waClaims)
	probe, err := metareply.NewProber(worker, v1, open, fake.url(), mhVersion, miApp, mhConfig())
	if err != nil {
		t.Fatal("probe", err)
	}
	m := &mhEnv{t: t, f: f, v1: v1, seal: seal, probe: probe, worker: worker, fake: fake,
		tenant: f.tenantA, store: f.storeA1, principal: principal, token: token}
	t.Cleanup(m.cleanup)
	return m
}

// cleanup deletes exactly the Pages this harness connected, in dependency order, and the meta-health mail/audit rows it wrote.
// LIFO cleanup order runs this before the harness pools close and before lcPurgeSessions.
func (m *mhEnv) cleanup() {
	t, f := m.t, m.f
	ctx := context.Background()
	pageIDs := make([]string, 0, len(m.pages))
	bindings := make([]string, 0, len(m.pages)*2)
	for _, p := range m.pages {
		pageIDs = append(pageIDs, p.pageID)
		bindings = append(bindings, p.fbBinding)
		if p.igBinding != "" {
			bindings = append(bindings, p.igBinding)
		}
	}
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`DELETE FROM integration.meta_connections WHERE tenant_id=$1 AND store_id=$2 AND page_id=ANY($3::text[])`, []any{f.tenantA, f.storeA1, pageIDs}},
		{`DELETE FROM integration.meta_page_heads WHERE tenant_id=$1 AND store_id=$2 AND binding_id=ANY($3::uuid[])`, []any{f.tenantA, f.storeA1, bindings}},
		{`DELETE FROM integration.meta_page_credentials WHERE tenant_id=$1 AND store_id=$2 AND binding_id=ANY($3::uuid[])`, []any{f.tenantA, f.storeA1, bindings}},
		{`DELETE FROM integration.bindings WHERE tenant_id=$1 AND store_id=$2 AND id=ANY($3::uuid[])`, []any{f.tenantA, f.storeA1, bindings}},
		{`DELETE FROM notify.merchant_alerts WHERE tenant_id=$1 AND store_id=$2 AND subject=ANY($3::text[])`, []any{f.tenantA, f.storeA1, pageIDs}},
		{`DELETE FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action='meta.connect.reauth_required' AND principal_id=$3`, []any{f.tenantA, f.storeA1, m.principal}},
	} {
		if _, err := f.owner.Exec(ctx, q.sql, q.args...); err != nil {
			t.Errorf("meta-health cleanup: %v", err)
		}
	}
}

// connectPage seeds a connected Page into store A1: a FB binding (and optional IG binding), a sealed FB Page token under the
// custody the probe opens (v2 HPKE when sealV2, else v1 AES), its head credential, and the meta_connections row (the trigger
// then creates the due probe row). token is a sentinel used by MCH06 to prove it never persists.
func (m *mhEnv) connectPage(t *testing.T, name string, withIG bool, scopes []string, sealV2 bool) mhPage {
	t.Helper()
	f := m.f
	pageID := miAsset()
	pageName := "page-" + name + "-" + t04Tag()
	sentinel := "SENTINEL-EAAP-" + t04Tag() + t04Tag()
	fb := miBinding(t, miTest{f: f}, pageID, "facebook", f.tenantA, f.storeA1, m.principal)

	var keyID string
	var nonce, ciphertext []byte
	var err error
	if sealV2 {
		keyID, nonce, ciphertext, err = m.seal.Seal(pagetoken.Scope{
			TenantID: f.tenantA, StoreID: f.storeA1, BindingID: fb, Provider: "facebook", AssetID: pageID, Version: 1,
		}, []byte(sentinel))
	} else {
		keyID, nonce, ciphertext, err = m.v1.Seal(metareply.PageTokenScope{
			TenantID: f.tenantA, StoreID: f.storeA1, BindingID: fb, Provider: "facebook", AssetID: pageID, Version: 1,
		}, sentinel)
	}
	if err != nil {
		t.Fatal("seal page token", err)
	}
	mustExec(t, f.owner, `INSERT INTO integration.meta_page_credentials(tenant_id,store_id,binding_id,provider,asset_id,version,key_id,nonce,ciphertext,scopes_attested,principal_id)
		VALUES($1,$2,$3,'facebook',$4,1,$5,$6,$7,$8,$9)`,
		f.tenantA, f.storeA1, fb, pageID, keyID, nonce, ciphertext, scopes, m.principal)
	mustExec(t, f.owner, `INSERT INTO integration.meta_page_heads(tenant_id,store_id,binding_id,current_version) VALUES($1,$2,$3,1)`,
		f.tenantA, f.storeA1, fb)

	var igBinding, igID, igName any
	if withIG {
		igIDv := miAsset()
		igBinding = miBinding(t, miTest{f: f}, igIDv, "instagram", f.tenantA, f.storeA1, m.principal)
		igID, igName = igIDv, "ig-"+t04Tag()
	}
	mustExec(t, f.owner, `INSERT INTO integration.meta_connections(tenant_id,store_id,page_id,page_name,fb_binding,ig_binding,ig_id,ig_username,scopes,status,connected_by,route_expires_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,'active',$10,$11)`,
		f.tenantA, f.storeA1, pageID, pageName, fb, igBinding, igID, igName, scopes, m.principal, time.Now().Add(24*time.Hour))
	p := mhPage{pageID: pageID, pageName: pageName, fbBinding: fb, token: sentinel}
	if igBinding != nil {
		p.igBinding = igBinding.(string)
	}
	m.pages = append(m.pages, p)
	return p
}

// sweep runs one probe sweep directly (the River periodic job is scheduled elsewhere; its worker is the same Work method).
func (m *mhEnv) sweep(t *testing.T) {
	t.Helper()
	if err := m.probe.Work(context.Background(), nil); err != nil {
		t.Fatalf("probe sweep: %v", err)
	}
}

// dueNow pulls a probe's next_due_at to now (owner SQL) so a second sweep re-claims it inside the same test.
func (m *mhEnv) dueNow(t *testing.T, page string) {
	t.Helper()
	mustExec(t, m.f.owner, `UPDATE integration.meta_health_probes SET next_due_at=clock_timestamp() WHERE page_id=$1`, page)
}

// capRows reads the probed capability rows of one binding back by the owner pool (checked_at is nil when NULL).
func (m *mhEnv) capRows(t *testing.T, binding string) map[string]mhCap {
	t.Helper()
	rows, err := m.f.owner.Query(context.Background(), `SELECT binding_id::text,provider,capability,state,reason,evidence,checked_at
		FROM integration.binding_capabilities WHERE binding_id=$1::uuid ORDER BY capability`, binding)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]mhCap{}
	for rows.Next() {
		var c mhCap
		if err := rows.Scan(&c.binding, &c.provider, &c.capability, &c.state, &c.reason, &c.evidence, &c.checkedAt); err != nil {
			t.Fatal(err)
		}
		out[c.capability] = c
	}
	return out
}

// probeRow reads one meta_health_probes row back by the owner pool.
func (m *mhEnv) probeRow(t *testing.T, page string) mhProbe {
	t.Helper()
	var p mhProbe
	err := m.f.owner.QueryRow(context.Background(), `SELECT severity,coalesce(last_outcome,''),coalesce(perm_source,''),
		generation,last_checked_at,consecutive_failures,episode,episode_closed_at
		FROM integration.meta_health_probes WHERE page_id=$1`, page).
		Scan(&p.severity, &p.lastOutcome, &p.permSource, &p.generation, &p.checkedAt, &p.consecutive, &p.episode, &p.episodeClosedAt)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// assertStates asserts the exact state/reason of every capability of one binding (and that there are no extra rows).
func (m *mhEnv) assertStates(t *testing.T, binding string, want map[string][2]string) {
	t.Helper()
	got := m.capRows(t, binding)
	if len(got) != len(want) {
		t.Fatalf("binding %s: %d capability rows, want %d (%v)", binding, len(got), len(want), got)
	}
	for capability, wr := range want {
		g := got[capability]
		if g.state != wr[0] || g.reason != wr[1] {
			t.Fatalf("%s/%s: state=%s reason=%s, want %s/%s", binding, capability, g.state, g.reason, wr[0], wr[1])
		}
	}
}

// mhAll builds the want map with one (state, reason) for all four capabilities.
func mhAll(state, reason string) map[string][2]string {
	return map[string][2]string{
		"read_comment":  {state, reason},
		"private_reply": {state, reason},
		"dm_session":    {state, reason},
		"reply_public":  {state, reason},
	}
}

// auditReauthCount counts the reauth audit rows of this harness's principal (the record path writes connected_by as principal).
func (m *mhEnv) auditReauthCount(t *testing.T) int64 {
	t.Helper()
	return miCount(t, m.f.owner, `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action='meta.connect.reauth_required' AND principal_id=$3`, m.tenant, m.store, m.principal)
}

// alertCount counts the merchant alerts enqueued for one Page.
func (m *mhEnv) alertCount(t *testing.T, page string) int64 {
	t.Helper()
	return miCount(t, m.f.owner, `SELECT count(*) FROM notify.merchant_alerts WHERE tenant_id=$1 AND store_id=$2 AND subject=$3`, m.tenant, m.store, page)
}

// readCaps runs reader.Capabilities inside a scoped transaction (commerce_runtime, store:read), the same way the API does.
func (m *mhEnv) readCaps(t *testing.T, reader metaconnect.CapabilityReader, binding string) []metaconnect.CapabilityState {
	t.Helper()
	var out []metaconnect.CapabilityState
	err := platform.WithScope(context.Background(), m.f.runtime, m.token, m.store, "store:read", func(tx pgx.Tx, s platform.Scope) error {
		var err error
		out, err = reader.Capabilities(context.Background(), tx, s, binding)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// doAdminReq issues one admin request to the meta-health surface with a bearer token and optional JSON body.
func doAdminReq(t *testing.T, h http.Handler, store, token, method, path string, body map[string]any) (int, map[string]any) {
	t.Helper()
	var rd io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		rd = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, rd)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func mhBase(store string) string { return "/v1/admin/stores/" + store + "/meta/health" }

// MCH02: the probe derives and records the four §6 capabilities per binding (v2 HPKE and v1 AES custody both open), starting
// rows at evidence MOCK with checked_at set; a healthy Page is severity none, generation 1.
func TestMetaHealthProbeDerivesAndRecords(t *testing.T) {
	m := mhSetup(t)
	v2 := m.connectPage(t, "ok-v2", true, mhScopes, true)
	v1 := m.connectPage(t, "ok-v1", true, mhScopes, false)
	m.sweep(t)

	wantOK := mhAll("ok", "ok")
	for _, p := range []mhPage{v2, v1} {
		m.assertStates(t, p.fbBinding, wantOK)
		wantIG := mhAll("ok", "ok")
		wantIG["dm_session"] = [2]string{"ok", "ok_app_level_assumed"} // IG dm_session has no IG-scoped perm to grant → ok by app-level assume (§4.3 rule 6)
		m.assertStates(t, p.igBinding, wantIG)
		// Same fact read back through the row itself (Derive rule 9).
		if ig := m.capRows(t, p.igBinding)["dm_session"]; ig.reason != "ok_app_level_assumed" {
			t.Fatalf("IG dm_session reason %q, want ok_app_level_assumed", ig.reason)
		}
		for cap, c := range m.capRows(t, p.fbBinding) {
			if c.evidence != "MOCK" || c.checkedAt == nil {
				t.Fatalf("%s/%s evidence=%s checked_at=%v, want MOCK + set", p.fbBinding, cap, c.evidence, c.checkedAt)
			}
		}
		pr := m.probeRow(t, p.pageID)
		if pr.severity != "none" || pr.lastOutcome != "probed" || pr.generation != 1 || pr.permSource != "graph" {
			t.Fatalf("probe row %+v", pr)
		}
	}
}

// MCH02: an invalid token (458/460/463) or a vanished Page flips the connection once (audited) and opens one blocking episode
// with one merchant alert; a second sweep with the same token must not double-flip or double-mail.
func TestMetaHealthTokenInvalidFlipsConnection(t *testing.T) {
	for _, tc := range []struct {
		name        string
		code, sub   int
		wantOutcome string
	}{
		{"revoked", 190, 458, "probed"},
		{"expired", 190, 463, "probed"},
		{"page_gone", 100, 0, "probed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := mhSetup(t)
			p := m.connectPage(t, tc.name, true, mhScopes, true)
			st, body := mhErr(tc.code, tc.sub)
			m.fake.setP1(st, body)
			before := m.auditReauthCount(t)
			m.sweep(t)

			// The probe returns early on P1 failure: no states derived, so the table stays empty and the reader falls back
			// to the snapshot (status=reauth_required -> reauth_required). The connection itself is the flip.
			if n := miCount(t, m.f.owner, `SELECT count(*) FROM integration.binding_capabilities WHERE page_id=$1`, p.pageID); n != 0 {
				t.Fatalf("token failure wrote %d capability rows, want 0", n)
			}
			var status string
			if err := m.f.owner.QueryRow(context.Background(), `SELECT status FROM integration.meta_connections WHERE page_id=$1`, p.pageID).Scan(&status); err != nil {
				t.Fatal(err)
			}
			if status != "reauth_required" {
				t.Fatalf("connection status %q, want reauth_required", status)
			}
			if got := m.auditReauthCount(t); got != before+1 {
				t.Fatalf("audit rows %d, want %d", got, before+1)
			}
			pr := m.probeRow(t, p.pageID)
			if pr.severity != "blocking" || pr.episode != 1 || pr.lastOutcome != tc.wantOutcome {
				t.Fatalf("probe row %+v", pr)
			}
			if got := m.alertCount(t, p.pageID); got != 1 {
				t.Fatalf("merchant alerts %d, want 1", got)
			}

			// Second sweep: the connection is already reauth_required, so no new audit, no new episode, no second mail.
			m.dueNow(t, p.pageID)
			m.sweep(t)
			if got := m.auditReauthCount(t); got != before+1 {
				t.Fatalf("second sweep audited again: %d", got)
			}
			if got := m.alertCount(t, p.pageID); got != 1 {
				t.Fatalf("second sweep re-mailed: %d alerts", got)
			}
			if pr2 := m.probeRow(t, p.pageID); pr2.episode != 1 {
				t.Fatalf("second sweep opened a new episode: %d", pr2.episode)
			}
		})
	}
}

// MCH02: a permission removed in P2 / a permission error (snapshot fallback) / a missing webhook field / our app absent.
func TestMetaHealthPermissionAndSubscription(t *testing.T) {
	m := mhSetup(t)

	t.Run("permission_revoked", func(t *testing.T) {
		p := m.connectPage(t, "perm", false, mhScopes, true)
		m.fake.setP2(http.StatusOK, `{"data":[{"permission":"pages_read_engagement","status":"granted"},{"permission":"pages_manage_engagement","status":"granted"},{"permission":"instagram_manage_comments","status":"granted"},{"permission":"instagram_manage_messages","status":"granted"}]}`)
		m.sweep(t)
		caps := m.capRows(t, p.fbBinding)
		if caps["private_reply"].state != "missing_permission" || caps["private_reply"].reason != "perm_pages_messaging" {
			t.Fatalf("private_reply %+v", caps["private_reply"])
		}
		if caps["dm_session"].state != "missing_permission" || caps["dm_session"].reason != "perm_pages_messaging" {
			t.Fatalf("dm_session %+v", caps["dm_session"])
		}
		if caps["read_comment"].state != "ok" || caps["reply_public"].state != "ok" {
			t.Fatalf("unaffected capabilities %+v", caps)
		}
	})

	t.Run("p2_permission_error_snapshot", func(t *testing.T) {
		p := m.connectPage(t, "snap", false, mhScopes, true)
		st, body := mhErr(100, 0)
		m.fake.setP2(st, body)
		m.sweep(t)
		// §4.2: a P2 permission error falls back to the connect snapshot (full scopes) → ok_snapshot.
		m.assertStates(t, p.fbBinding, mhAll("ok", "ok_snapshot"))
		if pr := m.probeRow(t, p.pageID); pr.permSource != "snapshot" {
			t.Fatalf("perm_source %q, want snapshot", pr.permSource)
		}
	})

	t.Run("messages_not_subscribed", func(t *testing.T) {
		p := m.connectPage(t, "msgs", false, mhScopes, true)
		m.fake.setP3(http.StatusOK, fmt.Sprintf(`{"data":[{"id":%q,"subscribed_fields":[{"name":"feed"}]}]}`, miApp))
		m.sweep(t)
		caps := m.capRows(t, p.fbBinding)
		if caps["dm_session"].state != "not_subscribed" || caps["dm_session"].reason != "sub_messages" {
			t.Fatalf("dm_session %+v", caps["dm_session"])
		}
		if caps["read_comment"].state != "ok" {
			t.Fatalf("read_comment %+v", caps["read_comment"])
		}
	})

	t.Run("app_absent", func(t *testing.T) {
		p := m.connectPage(t, "absent", false, mhScopes, true)
		m.fake.setP3(http.StatusOK, `{"data":[]}`)
		m.sweep(t)
		caps := m.capRows(t, p.fbBinding)
		if caps["read_comment"].state != "not_subscribed" || caps["read_comment"].reason != "sub_feed" {
			t.Fatalf("read_comment %+v", caps["read_comment"])
		}
		if caps["dm_session"].state != "not_subscribed" || caps["dm_session"].reason != "sub_messages" {
			t.Fatalf("dm_session %+v", caps["dm_session"])
		}
	})
}

// MCH03: a rate-limited sweep keeps the previous states and only bumps consecutive_failures; six consecutive unknown outcomes
// escalate to a banner-only probe_failing warning (no mail — only blocking mails).
func TestMetaHealthRateLimitAndProbeFailing(t *testing.T) {
	m := mhSetup(t)

	p := m.connectPage(t, "rl", false, mhScopes, true)
	m.sweep(t) // healthy baseline
	st, body := mhErr(4, 0)
	m.fake.setP1(st, body)
	m.dueNow(t, p.pageID)
	m.sweep(t)
	pr := m.probeRow(t, p.pageID)
	if pr.lastOutcome != "rate_limited" || pr.severity != "none" || pr.consecutive != 1 {
		t.Fatalf("rate-limited probe row %+v", pr)
	}
	if caps := m.capRows(t, p.fbBinding); caps["read_comment"].state != "ok" {
		t.Fatalf("rate limit changed a state: %+v", caps["read_comment"])
	}

	q := m.connectPage(t, "fail", false, mhScopes, true)
	st, body = mhErr(1, 0) // not 190/100 and not a rate-limit code → unknown
	m.fake.setP1(st, body)
	for i := 0; i < 6; i++ {
		m.dueNow(t, q.pageID)
		m.sweep(t)
	}
	pr2 := m.probeRow(t, q.pageID)
	if pr2.lastOutcome != "unknown" || pr2.severity != "warning" || pr2.consecutive != 6 {
		t.Fatalf("probe-failing row %+v", pr2)
	}
	if got := m.alertCount(t, q.pageID); got != 0 {
		t.Fatalf("probe_failing warning mailed: %d alerts", got)
	}
}

// MCH05: a connection whose FB head credential is missing is recorded as an unknown outcome (no capability rows), and the
// TableReader still shows the snapshot fallback for that binding.
func TestMetaHealthMissingCredentialRecordsUnknown(t *testing.T) {
	m := mhSetup(t)
	p := m.connectPage(t, "nocred", false, mhScopes, true)
	mustExec(t, m.f.owner, `DELETE FROM integration.meta_page_heads WHERE binding_id=$1`, p.fbBinding)
	mustExec(t, m.f.owner, `DELETE FROM integration.meta_page_credentials WHERE binding_id=$1`, p.fbBinding)
	m.sweep(t)

	if pr := m.probeRow(t, p.pageID); pr.lastOutcome != "unknown" || pr.severity != "none" {
		t.Fatalf("missing-credential probe row %+v", pr)
	}
	if n := miCount(t, m.f.owner, `SELECT count(*) FROM integration.binding_capabilities WHERE page_id=$1`, p.pageID); n != 0 {
		t.Fatalf("missing credential wrote %d capability rows, want 0", n)
	}
	snap := m.readCaps(t, metaconnect.SnapshotReader{Config: mhConfig()}, p.fbBinding)
	tab := m.readCaps(t, metaconnect.TableReader{Fallback: metaconnect.SnapshotReader{Config: mhConfig()}}, p.fbBinding)
	if !reflect.DeepEqual(snap, tab) || len(snap) != 4 {
		t.Fatalf("fallback mismatch: snap=%d tab=%d equal=%v", len(snap), len(tab), reflect.DeepEqual(snap, tab))
	}
}

// MCH06: the sentinel token never appears in a URL (path or query) and never persists in any surface the probe writes — only
// the in-memory Authorization header of the fake Graph carries it.
func TestMetaHealthTokenNeverLeaks(t *testing.T) {
	m := mhSetup(t)
	p := m.connectPage(t, "leak", false, mhScopes, true)
	m.sweep(t)

	if m.fake.sawURLLeak() {
		t.Fatal("probe put access_token in a URL")
	}
	for _, r := range m.fake.requests() {
		if strings.Contains(r.rawQuery, p.token) || strings.Contains(r.path, p.token) || strings.Contains(r.rawQuery, "access_token") {
			t.Fatalf("sentinel token in URL: %s?%s", r.path, r.rawQuery)
		}
	}
	for _, tbl := range []string{
		"integration.binding_capabilities",
		"integration.meta_health_probes",
		"integration.meta_connections",
		"integration.meta_page_credentials",
		"integration.meta_page_heads",
		"notify.merchant_alerts",
		"ops.audit_events",
	} {
		if n := miCount(t, m.f.owner, `SELECT count(*) FROM `+tbl+` x WHERE to_jsonb(x)::text LIKE '%'||$1||'%'`, p.token); n != 0 {
			t.Fatalf("sentinel token persisted in %s", tbl)
		}
	}
}

// MCH07: B1 serves the per-Page banner model (store:read, cross-store 404, snapshot fallback before the first sweep); B2 is
// integration:manage only (403), pulls the next probe to now (202) and is refused within 60 s of the last sweep (429).
func TestMetaHealthBannerRoutes(t *testing.T) {
	m := mhSetup(t)
	cfg := mhConfig()
	handler := httpapi.NewHandler(m.f.runtime, httpapi.Options{MetaHealth: &metaconnect.Health{
		Reader: metaconnect.TableReader{Fallback: metaconnect.SnapshotReader{Config: cfg}},
	}})
	p := m.connectPage(t, "banner", false, mhScopes, true)

	// B1 before any sweep: 200, severity none, the connected Page with the snapshot fallback's 4 capabilities (evidence DESIGN).
	code, body := doAdminReq(t, handler, m.store, m.token, "GET", mhBase(m.store), nil)
	if code != http.StatusOK {
		t.Fatalf("B1 status %d", code)
	}
	if body["severity"] != "none" {
		t.Fatalf("B1 severity %v", body["severity"])
	}
	pages, _ := body["pages"].([]any)
	if len(pages) != 1 {
		t.Fatalf("B1 pages %v", body["pages"])
	}
	page0, _ := pages[0].(map[string]any)
	if page0["page_id"] != p.pageID {
		t.Fatalf("B1 page %v", page0)
	}
	caps, _ := page0["capabilities"].([]any)
	if len(caps) != 4 {
		t.Fatalf("B1 capability count %d, want 4", len(caps))
	}
	// Each row names its capability (contract §9 B1 CapabilityState): the UI labels rows by name, never by position.
	names := map[string]bool{}
	for _, c := range caps {
		row, _ := c.(map[string]any)
		name, _ := row["capability"].(string)
		if name == "" {
			t.Fatalf("B1 capability row without a capability name: %v", row)
		}
		names[row["binding_id"].(string)+"/"+name] = true
	}
	if len(names) != 4 {
		t.Fatalf("B1 capability names not distinct per binding: %v", names)
	}

	// Cross-store (the principal has no store:read there) → 404; cross-tenant likewise.
	if code, _ := doAdminReq(t, handler, m.f.storeA2, m.token, "GET", mhBase(m.f.storeA2), nil); code != http.StatusNotFound {
		t.Fatalf("cross-store B1 status %d, want 404", code)
	}
	if code, _ := doAdminReq(t, handler, m.f.storeB, m.token, "GET", mhBase(m.f.storeB), nil); code != http.StatusNotFound {
		t.Fatalf("cross-tenant B1 status %d, want 404", code)
	}

	// B2 without integration:manage → 403.
	_, roToken := lcPrincipal(t, m.f, m.f.tenantA, []string{m.f.storeA1}, "store:read")
	if code, _ := doAdminReq(t, handler, m.store, roToken, "POST", mhBase(m.store)+"/recheck", map[string]any{"page_id": p.pageID}); code != http.StatusForbidden {
		t.Fatalf("B2 read-only status %d, want 403", code)
	}

	// B2 before any sweep → 202 with a next_check_at.
	code, body = doAdminReq(t, handler, m.store, m.token, "POST", mhBase(m.store)+"/recheck", map[string]any{"page_id": p.pageID})
	if code != http.StatusAccepted || body["next_check_at"] == nil {
		t.Fatalf("B2 %d %v", code, body)
	}

	// A sweep sets last_checked_at, so the immediate re-check is refused (429) — the 60 s gate.
	m.sweep(t)
	if code, body := doAdminReq(t, handler, m.store, m.token, "POST", mhBase(m.store)+"/recheck", map[string]any{"page_id": p.pageID}); code != http.StatusTooManyRequests {
		t.Fatalf("B2 after sweep %d %v, want 429", code, body)
	}

	// B1 after the sweep shows the probed (MOCK) rows.
	_, body = doAdminReq(t, handler, m.store, m.token, "GET", mhBase(m.store), nil)
	pages, _ = body["pages"].([]any)
	page0, _ = pages[0].(map[string]any)
	caps, _ = page0["capabilities"].([]any)
	if len(caps) != 4 {
		t.Fatalf("B1 post-sweep capability count %d", len(caps))
	}
	if c0, _ := caps[0].(map[string]any); c0["evidence"] != "MOCK" {
		t.Fatalf("B1 post-sweep evidence %v, want MOCK", c0["evidence"])
	}
}

// MCH08: on an empty table the TableReader equals the SnapshotReader (identical fixtures); once probed, the table rows win
// (MOCK, checked_at set) and the server-side binding_capability_state returns the table's state.
func TestMetaHealthReaderSwap(t *testing.T) {
	m := mhSetup(t)
	p := m.connectPage(t, "swap", false, mhScopes, true)
	cfg := mhConfig()

	snap := m.readCaps(t, metaconnect.SnapshotReader{Config: cfg}, p.fbBinding)
	tab := m.readCaps(t, metaconnect.TableReader{Fallback: metaconnect.SnapshotReader{Config: cfg}}, p.fbBinding)
	if !reflect.DeepEqual(snap, tab) || len(snap) != 4 {
		t.Fatalf("empty table: snap != table (snap=%d tab=%d equal=%v)", len(snap), len(tab), reflect.DeepEqual(snap, tab))
	}

	m.sweep(t)
	probed := m.readCaps(t, metaconnect.TableReader{Fallback: metaconnect.SnapshotReader{Config: cfg}}, p.fbBinding)
	if len(probed) != 4 {
		t.Fatalf("probed table rows %d", len(probed))
	}
	byCap := map[string]metaconnect.CapabilityState{}
	for _, c := range probed {
		byCap[c.Capability] = c
	}
	if byCap["read_comment"].Evidence != "MOCK" || byCap["read_comment"].CheckedAt.IsZero() {
		t.Fatalf("table row did not win: %+v", byCap["read_comment"])
	}
	if reflect.DeepEqual(snap, probed) {
		t.Fatal("probed table rows equal the snapshot fallback")
	}

	// §7.5 server-side Check returns the table row's state for a probed capability.
	var state string
	err := m.worker.QueryRow(context.Background(), `SELECT integration.binding_capability_state($1::uuid,$2::uuid,$3::uuid,'read_comment',$4::text[])`,
		m.tenant, m.store, p.fbBinding, []string{"pages_read_engagement"}).Scan(&state)
	if err != nil {
		t.Fatal(err)
	}
	if state != "ok" {
		t.Fatalf("binding_capability_state %q, want ok", state)
	}
}

// MCH09: a reconnect (status back to active) resets the probe (due now) and clears the probed rows; the next sweep re-derives
// healthy states and closes the episode.
func TestMetaHealthReauthReconnectResets(t *testing.T) {
	m := mhSetup(t)
	p := m.connectPage(t, "reconnect", false, mhScopes, true)

	st, body := mhErr(190, 458)
	m.fake.setP1(st, body)
	m.sweep(t)
	if pr := m.probeRow(t, p.pageID); pr.severity != "blocking" || pr.episode != 1 {
		t.Fatalf("reauth probe row %+v", pr)
	}

	// Reconnect: status back to active fires the trigger (reset probe + clear rows) without re-sealing (owner SQL stands in
	// for the real re-seal + connect write).
	mustExec(t, m.f.owner, `UPDATE integration.meta_connections SET status='active', updated_at=clock_timestamp() WHERE page_id=$1`, p.pageID)
	if n := miCount(t, m.f.owner, `SELECT count(*) FROM integration.binding_capabilities WHERE page_id=$1`, p.pageID); n != 0 {
		t.Fatalf("reconnect left %d capability rows", n)
	}
	if pr := m.probeRow(t, p.pageID); pr.generation != 1 || pr.severity == "none" {
		// The reset only pulls next_due_at earlier and clears rows; generation/severity/episode stay until the next record.
		t.Fatalf("reconnect probe row before re-sweep %+v", pr)
	}

	m.fake.setP1(http.StatusOK, "")
	m.sweep(t)
	m.assertStates(t, p.fbBinding, mhAll("ok", "ok"))
	pr := m.probeRow(t, p.pageID)
	if pr.severity != "none" || pr.lastOutcome != "probed" {
		t.Fatalf("post-reconnect probe row %+v", pr)
	}
	if pr.episode != 1 || pr.episodeClosedAt == nil {
		t.Fatalf("episode not closed: %+v", pr)
	}
}
