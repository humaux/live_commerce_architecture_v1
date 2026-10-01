package foundation_test

// meta_connect_test.go: PG gates of unit meta-connect (merchant self-serve Facebook Page / Instagram connect, migration 0095,
// contracts/meta-claims-intake-v1.md "Merchant connect (R4)"). Tier: MOCK. Meta is tests/metaconnect/fakegraph (loopback), the
// webhook / claim / private-reply chain is the real MCI harness (River consumer, claims intake poller, metareply dispatcher
// against the mciGraph fake); nothing here proves Meta, Facebook Login for Business or any provider behaviour (SANDBOX/LIVE are
// NOT_RUN). Run: bash scripts/dev/test-focused.sh '^TestMetaConnect' .

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"livecommerce/internal/httpapi"
	metaoauth "livecommerce/internal/integrations/meta/oauth"
	"livecommerce/internal/integrations/metareply"
	"livecommerce/internal/metaconnect"
	"livecommerce/tests/metaconnect/fakegraph"
)

const (
	mcnRedirect = "https://admin.example.test/api/meta/callback"
	mcnVersion  = "v99.0"
	mcnConfig   = "2952863798433821"
)

var (
	mcnFullPerms = []string{"business_management", "instagram_basic", "instagram_manage_comments", "instagram_manage_messages",
		"pages_manage_metadata", "pages_messaging", "pages_read_engagement", "pages_show_list"}
	mcnTasks = []string{"ADVERTISE", "MESSAGING", "MODERATE", "MANAGE"}
)

type mcnEnv struct {
	t       *testing.T
	e       *mciEnv
	f       *testFixture
	fake    *fakegraph.Server
	handler http.Handler
	store   string
	token   string
}

func newMcnEnv(t *testing.T) *mcnEnv {
	t.Helper()
	e := mciSetup(t, mciOpts{})
	f := e.h.f
	mustExec(t, f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,'integration:read') ON CONFLICT DO NOTHING`, f.tenantA, f.storeA1, e.h.actor)
	fake := fakegraph.New()
	t.Cleanup(fake.Close)
	fake.ExpectApp(miApp, miSecret, mcnRedirect)
	graph, err := metaoauth.NewGraph(fake.URL(), mcnVersion, nil)
	if err != nil {
		t.Fatal(err)
	}
	jobs, err := newInsertOnlyClient(f)
	if err != nil {
		t.Fatal(err)
	}
	svc, err := metaconnect.New(metaconnect.Config{Graph: graph, App: metaoauth.App{ID: miApp, RedirectURI: mcnRedirect, Secret: []byte(miSecret)},
		ConfigID: mcnConfig, GraphVersion: mcnVersion, PageAppID: miApp, IGAppID: miApp, StateKey: metaconnect.StateKeyFor([]byte(miSecret)), Keys: e.pageKeys}, jobs)
	if err != nil {
		t.Fatal(err)
	}
	return &mcnEnv{t: t, e: e, f: f, fake: fake, handler: httpapi.NewHandler(f.runtime, httpapi.Options{MetaConnect: svc}), store: f.storeA1, token: e.h.token}
}

type mcnResp struct {
	Status int
	Raw    []byte
	JSON   map[string]any
}

func (r mcnResp) str(k string) string { s, _ := r.JSON[k].(string); return s }
func (r mcnResp) code() string {
	if s, ok := r.JSON["code"].(string); ok {
		return s
	}
	s, _ := r.JSON["error"].(string)
	return s
}

// callOn issues one request to the meta-connect surface of store as token; key adds a fresh Idempotency-Key.
func (m *mcnEnv) callOn(store, token, method, path string, key bool, body any) mcnResp {
	m.t.Helper()
	var rd *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			m.t.Fatal(err)
		}
		rd = bytes.NewReader(raw)
	}
	var req *http.Request
	if rd != nil {
		req = httptest.NewRequest(method, "/v1/admin/stores/"+store+"/meta-connect"+path, rd)
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, "/v1/admin/stores/"+store+"/meta-connect"+path, nil)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if key {
		req.Header.Set("Idempotency-Key", t04Key("mcn"))
	}
	w := httptest.NewRecorder()
	m.handler.ServeHTTP(w, req)
	out := mcnResp{Status: w.Code, Raw: w.Body.Bytes()}
	_ = json.Unmarshal(out.Raw, &out.JSON)
	return out
}

func (m *mcnEnv) call(method, path string, key bool, body any) mcnResp {
	m.t.Helper()
	return m.callOn(m.store, m.token, method, path, key, body)
}

// connect runs start -> (browser at the dialog) -> callback with a fresh code for user; it returns the state id and the callback
// response. The state parameter is read from the dialog URL exactly as the browser would return it.
func (m *mcnEnv) connect(store, token string, user fakegraph.User) (string, mcnResp) {
	m.t.Helper()
	s := m.callOn(store, token, "POST", "/start", true, nil)
	if s.Status != 201 {
		m.t.Fatalf("start: %d %s", s.Status, s.Raw)
	}
	dialog, err := url.Parse(s.str("dialog_url"))
	if err != nil {
		m.t.Fatal(err)
	}
	code := "SYNTH-CODE-" + t04Tag() + t04Tag()
	m.fake.AddCode(code, user)
	cb := m.callOn(store, token, "GET", "/callback?code="+url.QueryEscape(code)+"&state="+url.QueryEscape(dialog.Query().Get("state")), false, nil)
	return s.str("state_id"), cb
}

func (m *mcnEnv) pick(stateID, page string, ig bool) mcnResp {
	m.t.Helper()
	return m.call("POST", "/pick", true, map[string]any{"state_id": stateID, "page_id": page, "include_instagram": ig})
}

func (m *mcnEnv) status() mcnResp {
	m.t.Helper()
	return m.call("GET", "/status", false, nil)
}

func (m *mcnEnv) count(q string, args ...any) int64 { return miCount(m.t, m.f.owner, q, args...) }

func mcnPage(name string, withIG bool) fakegraph.Page {
	p := fakegraph.Page{ID: miAsset(), Name: name, Token: "SENTINEL-EAAP-" + t04Tag() + t04Tag(), Tasks: mcnTasks}
	if withIG {
		p.IGID, p.IGName = miAsset(), "synthetic_ig_"+t04Tag()
	}
	return p
}

// reset disconnects whatever the store has so every subtest starts from "not connected" (404 = already clean).
func (m *mcnEnv) reset() {
	m.t.Helper()
	if r := m.call("POST", "/disconnect", true, map[string]any{}); r.Status != 200 && r.Status != 404 {
		m.t.Fatalf("reset disconnect: %d %s", r.Status, r.Raw)
	}
}

// ---------------------------------------------------------------------------------------------------------------------

func TestMetaConnectFlow(t *testing.T) {
	m := newMcnEnv(t)
	f := m.f
	ctx := context.Background()

	t.Run("start builds the Login for Business dialog and replays by Idempotency-Key", func(t *testing.T) {
		req := func(key string) mcnResp {
			r := httptest.NewRequest("POST", "/v1/admin/stores/"+m.store+"/meta-connect/start", nil)
			r.Header.Set("Authorization", "Bearer "+m.token)
			r.Header.Set("Idempotency-Key", key)
			w := httptest.NewRecorder()
			m.handler.ServeHTTP(w, r)
			out := mcnResp{Status: w.Code, Raw: w.Body.Bytes()}
			_ = json.Unmarshal(out.Raw, &out.JSON)
			return out
		}
		key := t04Key("mcn-dialog")
		a, b := req(key), req(key)
		if a.Status != 201 || b.Status != 201 || a.str("dialog_url") != b.str("dialog_url") || a.str("state_id") != b.str("state_id") {
			t.Fatalf("start/replay: %d %d %s", a.Status, b.Status, a.Raw)
		}
		u, err := url.Parse(a.str("dialog_url"))
		q := u.Query()
		if err != nil || u.Scheme != "https" || u.Host != "www.facebook.com" || u.Path != "/"+mcnVersion+"/dialog/oauth" ||
			q.Get("client_id") != miApp || q.Get("config_id") != mcnConfig || q.Get("redirect_uri") != mcnRedirect ||
			q.Get("response_type") != "code" || q.Get("override_default_response_type") != "true" || len(q.Get("state")) != 43 {
			t.Fatalf("dialog url wrong: %s", a.str("dialog_url"))
		}
		if m.count(`SELECT count(*) FROM integration.meta_connect_states WHERE state_hash=sha256(convert_to($1,'UTF8')) AND tenant_id=$2`, q.Get("state"), f.tenantA) != 1 {
			t.Fatal("only the SHA-256 of the state may be stored")
		}
		if m.count(`SELECT count(*) FROM ops.command_results WHERE response::text LIKE '%'||$1||'%'`, q.Get("state")) != 0 {
			t.Fatal("the OAuth state must not be in the replay receipt")
		}
	})

	t.Run("state mismatch, wrong principal, single use and expiry are refused", func(t *testing.T) {
		user := fakegraph.User{Permissions: mcnFullPerms, Pages: []fakegraph.Page{mcnPage("Synthetic Page", false)}}
		exchanges := m.fake.Count("GET", "oauth/access_token")
		// A state nobody started.
		s := m.call("POST", "/start", true, nil)
		d, _ := url.Parse(s.str("dialog_url"))
		code := "SYNTH-CODE-" + t04Tag()
		m.fake.AddCode(code, user)
		bad := strings.Repeat("A", 43)
		if r := m.call("GET", "/callback?code="+code+"&state="+bad, false, nil); r.Status != 409 || r.code() != "state_mismatch" {
			t.Fatalf("unknown state: %d %s", r.Status, r.Raw)
		}
		// Another principal of the same store presents the real state: uniform refusal, the state stays usable for its owner.
		_, other := lcPrincipal(t, f, f.tenantA, []string{f.storeA1}, "store:read", "integration:manage", "integration:read")
		if r := m.callOn(m.store, other, "GET", "/callback?code="+code+"&state="+url.QueryEscape(d.Query().Get("state")), false, nil); r.Status != 409 || r.code() != "state_mismatch" {
			t.Fatalf("foreign principal: %d %s", r.Status, r.Raw)
		}
		if got := m.fake.Count("GET", "oauth/access_token"); got != exchanges {
			t.Fatalf("a refused state must not reach Meta (exchanges %d -> %d)", exchanges, got)
		}
		ok := m.call("GET", "/callback?code="+code+"&state="+url.QueryEscape(d.Query().Get("state")), false, nil)
		if ok.Status != 200 || ok.str("state_id") != s.str("state_id") {
			t.Fatalf("owner callback: %d %s", ok.Status, ok.Raw)
		}
		// Single use: the same state again.
		again := "SYNTH-CODE-" + t04Tag()
		m.fake.AddCode(again, user)
		if r := m.call("GET", "/callback?code="+again+"&state="+url.QueryEscape(d.Query().Get("state")), false, nil); r.Status != 409 || r.code() != "state_mismatch" {
			t.Fatalf("replayed state: %d %s", r.Status, r.Raw)
		}
		// Expiry (10 minutes): age a fresh state in the owner pool.
		s2 := m.call("POST", "/start", true, nil)
		d2, _ := url.Parse(s2.str("dialog_url"))
		mustExec(t, f.owner, `UPDATE integration.meta_connect_states SET created_at=created_at-interval '1 hour',expires_at=clock_timestamp()-interval '1 minute' WHERE id=$1`, s2.str("state_id"))
		late := "SYNTH-CODE-" + t04Tag()
		m.fake.AddCode(late, user)
		if r := m.call("GET", "/callback?code="+late+"&state="+url.QueryEscape(d2.Query().Get("state")), false, nil); r.Status != 410 || r.code() != "state_expired" {
			t.Fatalf("expired state: %d %s", r.Status, r.Raw)
		}
		// A failed exchange burns the state and answers the fixed 502 (no cause).
		s3 := m.call("POST", "/start", true, nil)
		d3, _ := url.Parse(s3.str("dialog_url"))
		if r := m.call("GET", "/callback?code=NOT-A-CODE&state="+url.QueryEscape(d3.Query().Get("state")), false, nil); r.Status != 502 || r.code() != "meta_connect_failed" || strings.Contains(string(r.Raw), "invalid code") {
			t.Fatalf("failed exchange: %d %s", r.Status, r.Raw)
		}
	})

	t.Run("a missing permission or Page task refuses the pick and enables nothing", func(t *testing.T) {
		m.reset()
		noMsg := []string{}
		for _, p := range mcnFullPerms {
			if p != "pages_messaging" {
				noMsg = append(noMsg, p)
			}
		}
		page := mcnPage("Needs messaging grant", false)
		noMod := mcnPage("Needs moderate task", false)
		noMod.Tasks = []string{"MESSAGING"}
		state, cb := m.connect(m.store, m.token, fakegraph.User{Permissions: noMsg, Pages: []fakegraph.Page{page, noMod}})
		if cb.Status != 200 {
			t.Fatalf("callback: %d %s", cb.Status, cb.Raw)
		}
		subs := m.fake.Count("POST", "/subscribed_apps")
		list := m.call("GET", "/states/"+state, false, nil)
		raw := string(list.Raw)
		if list.Status != 200 || !strings.Contains(raw, "pages_messaging") || !strings.Contains(raw, "task_moderate") || strings.Contains(raw, "SENTINEL") {
			t.Fatalf("pick list must list what to re-grant and no token: %d %s", list.Status, raw)
		}
		for _, id := range []string{page.ID, noMod.ID} {
			if r := m.pick(state, id, false); r.Status != 422 || r.code() != "missing_permission" {
				t.Fatalf("pick %s: %d %s", id, r.Status, r.Raw)
			}
			if m.count(`SELECT count(*) FROM integration.bindings WHERE external_asset_id=$1`, id) != 0 ||
				m.count(`SELECT count(*) FROM meta_inbox.routes WHERE asset_id=$1`, id) != 0 ||
				m.count(`SELECT count(*) FROM integration.meta_page_credentials WHERE asset_id=$1`, id) != 0 {
				t.Fatalf("a refused pick of %s left a binding, route or credential", id)
			}
		}
		if m.fake.Count("POST", "/subscribed_apps") != subs {
			t.Fatal("a refused pick must not call subscribed_apps")
		}
		if r := m.pick(state, miAsset(), false); r.Status != 422 || r.code() != "not_in_pick_list" {
			t.Fatalf("page outside the pick list: %d %s", r.Status, r.Raw)
		}
		if s := m.status(); s.Status != 200 || s.JSON["connected"] != false {
			t.Fatalf("status after refusals: %d %s", s.Status, s.Raw)
		}
	})

	t.Run("Instagram permissions missing: IG pick refused, Facebook-only pick succeeds", func(t *testing.T) {
		m.reset()
		noIG := []string{}
		for _, p := range mcnFullPerms {
			if p != "instagram_manage_messages" {
				noIG = append(noIG, p)
			}
		}
		page := mcnPage("Page with IG, IG not granted", true)
		state, cb := m.connect(m.store, m.token, fakegraph.User{Permissions: noIG, Pages: []fakegraph.Page{page}})
		if cb.Status != 200 {
			t.Fatalf("callback: %d %s", cb.Status, cb.Raw)
		}
		if r := m.pick(state, page.ID, true); r.Status != 422 || r.code() != "missing_permission" {
			t.Fatalf("IG pick: %d %s", r.Status, r.Raw)
		}
		if m.count(`SELECT count(*) FROM integration.bindings WHERE external_asset_id=ANY($1)`, []string{page.ID, page.IGID}) != 0 {
			t.Fatal("a refused IG pick left bindings (never partial-enable)")
		}
		r := m.pick(state, page.ID, false)
		if r.Status != 201 || r.JSON["instagram"] != false {
			t.Fatalf("Facebook-only pick: %d %s", r.Status, r.Raw)
		}
		s := m.status()
		if s.JSON["connected"] != true || s.JSON["instagram"] != nil {
			t.Fatalf("status: %s", s.Raw)
		}
		if m.count(`SELECT count(*) FROM integration.bindings WHERE external_asset_id=$1`, page.IGID) != 0 {
			t.Fatal("Facebook-only pick must not create the Instagram binding")
		}
		m.reset()
	})

	// ---- the main chain --------------------------------------------------------------------------------------------
	pageA, pageB := mcnPage("Synthetic Page A", true), mcnPage("Synthetic Page B", false)
	m.reset()
	state, cb := m.connect(m.store, m.token, fakegraph.User{Permissions: mcnFullPerms, Pages: []fakegraph.Page{pageA, pageB}})
	if cb.Status != 200 {
		t.Fatalf("callback: %d %s", cb.Status, cb.Raw)
	}
	for _, r := range m.fake.Requests() { // tokens only ever travel as a Bearer header or a JSON body, never in a URL
		if r.HasQueryToken {
			t.Fatalf("a token travelled in a URL: %+v", r)
		}
	}
	if m.fake.Count("GET", "oauth/access_token") < 2 {
		t.Fatal("expected the code exchange and the long-lived extend")
	}
	if m.count(`SELECT count(*) FROM integration.meta_connect_states WHERE id=$1 AND pending_ciphertext IS NOT NULL AND done_at IS NULL`, state) != 1 {
		t.Fatal("the pending user token must be sealed in the state until pick")
	}

	var fbBinding, igBinding string
	t.Run("pick Page A with Instagram binds, subscribes, routes and seals", func(t *testing.T) {
		r := m.pick(state, pageA.ID, true)
		if r.Status != 201 || r.JSON["instagram"] != true || r.str("page_id") != pageA.ID {
			t.Fatalf("pick: %d %s", r.Status, r.Raw)
		}
		if !m.fake.Subscribed(pageA.ID) || m.fake.Subscribed(pageB.ID) {
			t.Fatal("exactly the picked Page must be subscribed")
		}
		for _, q := range m.fake.Requests() {
			if strings.HasSuffix(q.Path, "/subscribed_apps") && q.Method == "POST" && (!q.BodyHasToken || q.HasQueryToken) {
				t.Fatalf("subscribed_apps token placement: %+v", q)
			}
		}
		s := m.status()
		page, _ := s.JSON["page"].(map[string]any)
		ig, _ := s.JSON["instagram"].(map[string]any)
		if s.JSON["connected"] != true || s.JSON["status"] != "active" || page["id"] != pageA.ID || page["name"] != pageA.Name ||
			ig["id"] != pageA.IGID || ig["username"] != pageA.IGName || s.JSON["last_event_at"] != nil || s.str("route_expires_at") == "" {
			t.Fatalf("status card: %s", s.Raw)
		}
		perms := strings.Join(toStrings(s.JSON["permissions"]), ",")
		for _, want := range []string{"pages_messaging", "pages_manage_metadata", "instagram_manage_comments"} {
			if !strings.Contains(perms, want) {
				t.Fatalf("permissions missing %s: %s", want, perms)
			}
		}
		// Rows: two enabled bindings, two credential heads at version 1, two enabled routes, state finished and wiped.
		if err := f.owner.QueryRow(ctx, `SELECT id::text FROM integration.bindings WHERE tenant_id=$1 AND store_id=$2 AND provider='facebook' AND external_asset_id=$3 AND enabled`, f.tenantA, m.store, pageA.ID).Scan(&fbBinding); err != nil {
			t.Fatalf("facebook binding: %v", err)
		}
		if err := f.owner.QueryRow(ctx, `SELECT id::text FROM integration.bindings WHERE tenant_id=$1 AND store_id=$2 AND provider='instagram' AND external_asset_id=$3 AND enabled`, f.tenantA, m.store, pageA.IGID).Scan(&igBinding); err != nil {
			t.Fatalf("instagram binding: %v", err)
		}
		if m.count(`SELECT count(*) FROM integration.meta_page_heads WHERE binding_id=ANY($1::uuid[]) AND current_version=1`, []string{fbBinding, igBinding}) != 2 ||
			m.count(`SELECT count(*) FROM meta_inbox.routes WHERE enabled AND app_id=$1 AND binding_id=ANY($2::uuid[])`, miApp, []string{fbBinding, igBinding}) != 2 ||
			m.count(`SELECT count(*) FROM integration.meta_connect_states WHERE id=$1 AND done_at IS NOT NULL AND pending_ciphertext IS NULL`, state) != 1 {
			t.Fatal("expected 2 heads at v1, 2 enabled routes and a finished state without its sealed user token")
		}
		// The sealed credential opens (only) with the page-token keyring, to exactly Page A's token, for BOTH providers.
		for _, c := range []struct{ binding, provider, asset string }{{fbBinding, "facebook", pageA.ID}, {igBinding, "instagram", pageA.IGID}} {
			var keyID string
			var nonce, ct []byte
			if err := f.owner.QueryRow(ctx, `SELECT key_id,nonce,ciphertext FROM integration.meta_page_credentials WHERE binding_id=$1 AND version=1`, c.binding).Scan(&keyID, &nonce, &ct); err != nil {
				t.Fatal(err)
			}
			sec, err := m.e.pageKeys.Open(metareply.PageTokenScope{TenantID: f.tenantA, StoreID: m.store, BindingID: c.binding, Provider: c.provider, AssetID: c.asset, Version: 1}, keyID, nonce, ct)
			if err != nil || string(sec.Reveal()) != pageA.Token {
				t.Fatalf("%s credential does not open to the Page token: %v", c.provider, err)
			}
			if _, err := m.e.pageKeys.Open(metareply.PageTokenScope{TenantID: f.tenantA, StoreID: m.store, BindingID: c.binding, Provider: c.provider, AssetID: c.asset, Version: 2}, keyID, nonce, ct); err == nil {
				t.Fatal("the AAD must bind the version")
			}
		}
		// A state is single use for picks too, and a connected store refuses another Page until disconnect.
		if r := m.pick(state, pageA.ID, true); r.Status != 409 || r.code() != "state_used" {
			t.Fatalf("second pick of a finished state: %d %s", r.Status, r.Raw)
		}
		st2, cb2 := m.connect(m.store, m.token, fakegraph.User{Permissions: mcnFullPerms, Pages: []fakegraph.Page{pageA, pageB}})
		if cb2.Status != 200 {
			t.Fatalf("second callback: %d %s", cb2.Status, cb2.Raw)
		}
		if r := m.pick(st2, pageB.ID, false); r.Status != 409 || r.code() != "already_connected" {
			t.Fatalf("other Page while connected: %d %s", r.Status, r.Raw)
		}
	})

	t.Run("no token or secret in any stored text, URL or audit row", func(t *testing.T) {
		for _, needle := range []string{pageA.Token, pageB.Token, "SENTINEL-EAAS", "SENTINEL-EAAL", miSecret} {
			for _, q := range []string{
				`SELECT count(*) FROM integration.meta_connect_states s WHERE to_jsonb(s)::text LIKE '%'||$1||'%'`,
				`SELECT count(*) FROM integration.meta_connections c WHERE to_jsonb(c)::text LIKE '%'||$1||'%'`,
				`SELECT count(*) FROM integration.meta_page_credentials c WHERE to_jsonb(c)::text LIKE '%'||$1||'%'`,
				`SELECT count(*) FROM integration.bindings b WHERE to_jsonb(b)::text LIKE '%'||$1||'%'`,
				`SELECT count(*) FROM ops.audit_events a WHERE to_jsonb(a)::text LIKE '%'||$1||'%'`,
				`SELECT count(*) FROM ops.command_results r WHERE to_jsonb(r)::text LIKE '%'||$1||'%'`,
				`SELECT count(*) FROM meta_inbox.routes r WHERE to_jsonb(r)::text LIKE '%'||$1||'%'`,
			} {
				if m.count(q, needle) != 0 {
					t.Fatalf("secret material found by %.60s", q)
				}
			}
		}
		for _, r := range m.fake.Requests() {
			if r.HasQueryToken || (r.HasClientSecret && r.Path != "oauth/access_token") {
				t.Fatalf("secret in a URL: %+v", r)
			}
		}
	})

	t.Run("webhook comment -> claim -> private reply (MOCK) on the merchant-connected Page; Graph 190 asks to reconnect", func(t *testing.T) {
		e := m.e
		e.pageAsset, e.pageBinding, e.igAsset, e.igBinding = pageA.ID, fbBinding, pageA.IGID, igBinding
		e.pageToken = pageA.Token
		e.postID, e.mediaID = pageA.ID+"_"+mciDigits(10), "178"+mciDigits(13)
		e.srcFB = e.mustSource(t, "page", pageA.ID, e.postID, true)
		e.srcIG = e.mustSource(t, "instagram", pageA.IGID, e.mediaID, true)
		g := newMciGraph(t)
		d := e.newDispatcher(t, g, nil, nil)
		for _, ig := range []bool{false, true} {
			r := e.planReply(t, ig, "", "A1")
			if r.op == "" || r.bundleID == "" {
				t.Fatalf("ig=%v: the connected Page's comment did not become a claim with a planned reply", ig)
			}
			d.run(t, r.op)
			e.awaitOp(t, r.op, "SUCCEEDED", 30*time.Second, "completed")
			var posted *mciGraphReq
			for _, q := range g.all() {
				q := q
				if q.comment == r.s.comment {
					posted = &q
				}
			}
			if posted == nil || posted.bodyToken != pageA.Token || posted.path != "/v99.0/"+r.asset+"/messages" || posted.header.Get("Authorization") != "" {
				t.Fatalf("ig=%v: private reply request wrong: %+v", ig, posted)
			}
		}
		// The card now shows the time of the last routed comment (meta_inbox.connect_last_event).
		if s := m.status(); s.str("last_event_at") == "" {
			t.Fatalf("status must report last_event_at after routed comments: %s", s.Raw)
		}
		// Graph 190 on a reply: the operation stays UNKNOWN (never re-sent) and the card flips to reauth_required.
		r := e.planReply(t, false, "", "A1")
		g.setMode("400")
		d.run(t, r.op)
		e.awaitOp(t, r.op, "UNKNOWN", 40*time.Second, "cancelled", "discarded")
		g.setMode("ok")
		deadline := time.Now().Add(5 * time.Second)
		var s mcnResp
		for time.Now().Before(deadline) {
			if s = m.status(); s.JSON["status"] == "reauth_required" {
				break
			}
			time.Sleep(20 * time.Millisecond) // polls the persisted flag; the dispatcher flips it right after the 400
		}
		if s.JSON["status"] != "reauth_required" {
			t.Fatalf("card status after a Graph 190: %s", s.Raw)
		}
		// Reconnect with a rotated Page token: status active again, credential version 2, new token used for the next reply.
		rotated := pageA
		rotated.Token = "SENTINEL-EAAP-ROTATED-" + t04Tag()
		st, cb := m.connect(m.store, m.token, fakegraph.User{Permissions: mcnFullPerms, Pages: []fakegraph.Page{rotated}})
		if cb.Status != 200 {
			t.Fatalf("reconnect callback: %d %s", cb.Status, cb.Raw)
		}
		if p := m.pick(st, pageA.ID, true); p.Status != 201 {
			t.Fatalf("reconnect pick: %d %s", p.Status, p.Raw)
		}
		if s = m.status(); s.JSON["status"] != "active" {
			t.Fatalf("status after reconnect: %s", s.Raw)
		}
		if m.count(`SELECT count(*) FROM integration.meta_page_heads WHERE binding_id=ANY($1::uuid[]) AND current_version=2`, []string{fbBinding, igBinding}) != 2 {
			t.Fatal("reconnect must append credential version 2 for both bindings")
		}
		r2 := e.planReply(t, false, "", "A1")
		d.run(t, r2.op)
		e.awaitOp(t, r2.op, "SUCCEEDED", 30*time.Second, "completed")
		last := g.all()[len(g.all())-1]
		if last.comment != r2.s.comment || last.bodyToken != rotated.Token {
			t.Fatal("the reply after a reconnect must use the rotated Page token")
		}
	})

	t.Run("another store or tenant cannot bind the same Page (409, no leak, no Meta call)", func(t *testing.T) {
		_, tokB := lcPrincipal(t, f, f.tenantB, []string{f.storeB}, "store:read", "integration:manage", "integration:read")
		subs := m.fake.Count("POST", "/subscribed_apps")
		st, cb := m.connect(f.storeB, tokB, fakegraph.User{Permissions: mcnFullPerms, Pages: []fakegraph.Page{pageA}})
		if cb.Status != 200 {
			t.Fatalf("foreign callback: %d %s", cb.Status, cb.Raw)
		}
		for _, ig := range []bool{false, true} {
			r := m.callOn(f.storeB, tokB, "POST", "/pick", true, map[string]any{"state_id": st, "page_id": pageA.ID, "include_instagram": ig})
			if r.Status != 409 || r.code() != "page_taken" {
				t.Fatalf("cross-store pick ig=%v: %d %s", ig, r.Status, r.Raw)
			}
			raw := string(r.Raw)
			if strings.Contains(raw, f.storeA1) || strings.Contains(raw, f.tenantA) || strings.Contains(raw, f.tenantB) {
				t.Fatalf("refusal leaks a scope id: %s", raw)
			}
		}
		if m.fake.Count("POST", "/subscribed_apps") != subs {
			t.Fatal("a refused cross-store pick must not call Meta")
		}
		if m.count(`SELECT count(*) FROM integration.bindings WHERE store_id=$1 AND external_asset_id=ANY($2)`, f.storeB, []string{pageA.ID, pageA.IGID}) != 0 {
			t.Fatal("the other store got a binding")
		}
		if m.count(`SELECT count(*) FROM meta_inbox.routes WHERE asset_id=ANY($1) AND store_id<>$2`, []string{pageA.ID, pageA.IGID}, m.store) != 0 {
			t.Fatal("the other store got a route")
		}
	})

	t.Run("a viewer with integration:read sees the card but cannot connect", func(t *testing.T) {
		_, viewer := lcPrincipal(t, f, f.tenantA, []string{f.storeA1}, "store:read", "integration:read")
		if r := m.callOn(m.store, viewer, "GET", "/status", false, nil); r.Status != 200 || r.JSON["connected"] != true {
			t.Fatalf("viewer status: %d %s", r.Status, r.Raw)
		}
		for _, c := range []struct {
			method, path string
			body         any
		}{{"POST", "/start", nil}, {"POST", "/disconnect", map[string]any{}}} {
			if r := m.callOn(m.store, viewer, c.method, c.path, true, c.body); r.Status != 403 {
				t.Fatalf("viewer %s: %d %s", c.path, r.Status, r.Raw)
			}
		}
		if r := m.callOn(m.store, "x", "GET", "/status", false, nil); r.Status != 401 {
			t.Fatalf("bad token: %d", r.Status)
		}
	})

	t.Run("disconnect destroys the token, stops intake, disables bindings and unsubscribes", func(t *testing.T) {
		e := m.e
		r := m.call("POST", "/disconnect", true, map[string]any{})
		if r.Status != 200 {
			t.Fatalf("disconnect: %d %s", r.Status, r.Raw)
		}
		if s := m.status(); s.JSON["connected"] != false {
			t.Fatalf("status after disconnect: %s", s.Raw)
		}
		if m.count(`SELECT count(*) FROM integration.meta_page_heads WHERE binding_id=ANY($1::uuid[])`, []string{fbBinding, igBinding}) != 0 ||
			m.count(`SELECT count(*) FROM integration.meta_page_credentials WHERE binding_id=ANY($1::uuid[])`, []string{fbBinding, igBinding}) != 0 {
			t.Fatal("the sealed Page token (every version) must be destroyed")
		}
		if m.count(`SELECT count(*) FROM integration.bindings WHERE id=ANY($1::uuid[]) AND enabled`, []string{fbBinding, igBinding}) != 0 ||
			m.count(`SELECT count(*) FROM meta_inbox.routes WHERE binding_id=ANY($1::uuid[]) AND enabled`, []string{fbBinding, igBinding}) != 0 {
			t.Fatal("bindings and routes must be disabled")
		}
		if m.fake.Subscribed(pageA.ID) {
			t.Fatal("best-effort unsubscribe did not reach the fake Graph")
		}
		if m.count(`SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action IN ('meta.connect.started','meta.connect.callback','meta.connect.page_connected','meta.connect.disconnected')`, f.tenantA, m.store) < 4 {
			t.Fatal("every connect step must be audited")
		}
		// Intake stops: the same comment path now lands quarantined, with no claim row.
		raw := mciFBBody(pageA.ID, e.postID, mciDigits(15)+"_"+mciDigits(10), mciDigits(15), "n", "A1", mciAt(2*time.Second), nil)
		if code, body := miPost(t, e.page, raw); code != 200 {
			t.Fatalf("webhook post: %d %s", code, body)
		}
		if m.count(`SELECT count(*) FROM meta_inbox.events WHERE asset_id=$1 AND disposition='QUARANTINED' AND created_at>clock_timestamp()-interval '1 minute'`, pageA.ID) < 1 {
			t.Fatal("a comment on a disconnected Page must be quarantined")
		}
		if r := m.call("POST", "/disconnect", true, map[string]any{}); r.Status != 404 {
			t.Fatalf("second disconnect: %d %s", r.Status, r.Raw)
		}
		// The Page stays owned by this store (it cannot silently move), and the same merchant can connect it again.
		st, cb := m.connect(m.store, m.token, fakegraph.User{Permissions: mcnFullPerms, Pages: []fakegraph.Page{pageA}})
		if cb.Status != 200 {
			t.Fatalf("callback: %d %s", cb.Status, cb.Raw)
		}
		if p := m.pick(st, pageA.ID, true); p.Status != 201 {
			t.Fatalf("re-pick after disconnect: %d %s", p.Status, p.Raw)
		}
		if m.count(`SELECT count(*) FROM integration.bindings WHERE tenant_id=$1 AND store_id=$2 AND external_asset_id=$3`, f.tenantA, m.store, pageA.ID) != 1 {
			t.Fatal("re-connect must reuse the (re-enabled) binding, not create a second one")
		}
		if m.count(`SELECT count(*) FROM integration.meta_page_heads WHERE binding_id=ANY($1::uuid[]) AND current_version=1`, []string{fbBinding, igBinding}) != 2 {
			t.Fatal("after a disconnect the credential restarts at version 1")
		}
		m.reset()
	})
}

func toStrings(v any) []string {
	list, _ := v.([]any)
	out := make([]string, 0, len(list))
	for _, x := range list {
		if s, ok := x.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
