// live_console_comments_test.go — LC-B2 (live-console comment read-through) real-PG foundation gates
// LCN01/02/04/05, written by the independent test_worker from contracts/live-console-v1.md §2 (poller, leases,
// caps, bridge incl. comment-facts, cursors, deletion eviction), §2.5 (marks), §7.4 (prints) and the A2/A3 routes.
//
// Owns: the real-PG console-comment gates. The poller (internal/integrations/metareply.Console) runs against a
// fake Graph served on 127.0.0.1 (the only loopback origin metareply.Config accepts), reading through the real
// SECURITY DEFINER lease/token/credential functions in migration 0123; the bridge is the Console's own HTTP handler.
//
// The owner (superuser) pool is used only for synthetic setup, fault injection (expiring a held lease) and read-back
// of columns no runtime role may read (the poll lease row), exactly as the other live gates do.
//
// Isolation: every test owns a fresh principal (live:* grants explicit), a fresh OPEN-window session and fresh
// sources/bindings/routes. The store-wide "one OPEN window" rule means these tests run serially (no t.Parallel);
// lcPurgeSessions removes every session this actor created, and the comment_prints rows are deleted first (their FK
// to live.sessions does not cascade). Evidence label: REAL_PG, MOCK Graph (loopback).
package foundation_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/claims"
	"livecommerce/internal/command"
	"livecommerce/internal/integrations/metareply"
	"livecommerce/internal/live"
	"livecommerce/internal/platform"
)

// ---------------------------------------------------------------------------------------
// Fake Graph (§2.2 poller + §2.3 comment-facts read target; MOCK, loopback only)
// ---------------------------------------------------------------------------------------

type lcnHit struct {
	method, target string // target = path?query, never contains a secret
	bearer         bool   // Authorization header present (token travels in the header, never the URL)
}

// lcnGraph serves GET /{version}/{objID}/comments (forward + older backfill) and GET /{version}/{ref}
// (comment-facts single read). The page read answers with the {"data":[...],"paging":{...}} shape; the single
// read answers with the BARE comment object, exactly like Graph. paging.next, when set, is a marker URL the
// poller must never follow.
type lcnGraph struct {
	mu       sync.Mutex
	comments map[string][]map[string]any // objID -> comments, oldest first
	refs     map[string]map[string]any   // ref -> one comment (facts fallback)
	next     string
	hitsList []lcnHit
}

func newLcnGraph() *lcnGraph {
	return &lcnGraph{comments: map[string][]map[string]any{}, refs: map[string]map[string]any{}}
}

func (g *lcnGraph) setComments(objID string, list []map[string]any) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.comments[objID] = list
}

func (g *lcnGraph) setRef(ref string, c map[string]any) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.refs[ref] = c
}

func (g *lcnGraph) setNext(next string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.next = next
}

func (g *lcnGraph) hits() []lcnHit {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]lcnHit(nil), g.hitsList...)
}

func (g *lcnGraph) hasTarget(sub string) bool {
	for _, h := range g.hits() {
		if strings.Contains(h.target, sub) {
			return true
		}
	}
	return false
}

func (g *lcnGraph) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	g.hitsList = append(g.hitsList, lcnHit{method: r.Method, target: r.URL.RequestURI(), bearer: r.Header.Get("Authorization") != ""})
	g.mu.Unlock()

	p := strings.TrimPrefix(r.URL.Path, "/")
	version, rest, ok := strings.Cut(p, "/")
	if !ok || version != "v99.0" {
		http.NotFound(w, r)
		return
	}
	if strings.HasSuffix(rest, "/comments") {
		objID := strings.TrimSuffix(rest, "/comments")
		g.mu.Lock()
		list, found := g.comments[objID]
		next := g.next
		g.mu.Unlock()
		if !found {
			http.NotFound(w, r)
			return
		}
		body := map[string]any{
			"data": list,
			"paging": map[string]any{
				"cursors": map[string]any{"after": "AFTER_CURSOR_0", "before": "BEFORE_CURSOR_0"},
			},
		}
		if next != "" {
			body["paging"].(map[string]any)["next"] = next
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(body)
		return
	}
	// comment-facts single read
	g.mu.Lock()
	c, found := g.refs[rest]
	g.mu.Unlock()
	if !found {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(c)
}

// lcnComment builds one synthetic Graph comment. fromID == assetID makes it a page comment.
func lcnComment(ref, created, fromID, name, text, parent string, attachment bool) map[string]any {
	row := map[string]any{"id": ref, "message": text, "created_time": created}
	if fromID != "" {
		from := map[string]any{"id": fromID}
		if name != "" {
			from["name"] = name
		}
		row["from"] = from
	}
	if parent != "" {
		row["parent"] = map[string]any{"id": parent}
	}
	if attachment {
		row["attachment"] = map[string]any{"type": "sticker"}
	}
	return row
}

// lcnRef is a plain Meta comment id shape (numeric).
func lcnRef() string { return mciDigits(15) + "_" + mciDigits(10) }

// ---------------------------------------------------------------------------------------
// Harness
// ---------------------------------------------------------------------------------------

type lcnEnv struct {
	f           *testFixture
	h           *lcHarness
	m           miTest
	pageKeys    *metareply.PageTokenKeyring
	worker      *pgxpool.Pool
	session     string
	asset       string
	binding     string
	postID      string
	sourceID    string
	pageToken   string
	graph       *lcnGraph
	graphURL    string
	bridgeToken []byte
	cursorKey   []byte
}

// lcnSetup builds one claim-ready live session (OPEN window) with one Facebook Page route + active claim source and
// a registered Page token attested pages_read_engagement, plus a fake loopback Graph and a commerce_claims_worker pool.
func lcnSetup(t *testing.T) *lcnEnv {
	t.Helper()
	h := lcSetup(t)
	f := h.f
	e := &lcnEnv{f: f, h: h}
	e.m = miTest{f: f, registrar: miPool(t, f, "commerce_meta_registrar")}
	e.worker = miPool(t, f, "commerce_claims_worker")
	// integration:execute: put_claim_source. integration:manage: register_meta_page_token.
	mustExec(t, f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission)
		VALUES($1,$2,$3,'integration:execute'),($1,$2,$3,'integration:manage') ON CONFLICT DO NOTHING`, f.tenantA, f.storeA1, h.actor)
	raw := randomBytes(32)
	var err error
	if e.pageKeys, err = metareply.NewPageTokenKeyring("pt_key_1", map[string][]byte{"pt_key_1": raw}); err != nil {
		t.Fatal(err)
	}
	e.session = h.draft(t, f.storeA1)
	h.open(t, e.session, claims.MatchExact)
	e.asset = miAsset()
	e.binding = miBinding(t, e.m, e.asset, "facebook", f.tenantA, f.storeA1, f.principalA)
	miRoute(t, e.m, e.asset, f.tenantA, f.storeA1, e.binding)
	e.pageToken = "SENTINEL-EAAG-PAGE-" + t04Tag() + t04Tag()
	if _, err = metareply.RegisterPageToken(context.Background(), e.m.registrar, e.pageKeys, metareply.Registration{
		TenantID: f.tenantA, StoreID: f.storeA1, PrincipalID: h.actor, BindingID: e.binding, Provider: "facebook",
		AssetID: e.asset, ExpectedVersion: 0, Scopes: []string{"pages_read_engagement"}}, e.pageToken); err != nil {
		t.Fatalf("register synthetic Page token: %v", err)
	}
	e.postID = e.asset + "_" + mciDigits(10)
	e.sourceID = e.mustSource(t, e.session, e.asset, e.postID, true)

	e.graph = newLcnGraph()
	srv := httptest.NewServer(e.graph)
	t.Cleanup(srv.Close)
	e.graphURL = srv.URL
	e.bridgeToken = randomBytes(32)
	e.cursorKey = randomBytes(32)
	// comment_prints has no ON DELETE CASCADE to live.sessions: delete before lcPurgeSessions (LIFO).
	t.Cleanup(func() {
		mustExec(t, f.owner, `DELETE FROM live.comment_prints WHERE tenant_id=$1 AND store_id=$2 AND session_id=$3`, f.tenantA, f.storeA1, e.session)
	})
	return e
}

// putSource calls live.put_claim_source as the harness actor (commerce_runtime login, real GUC state).
func (e *lcnEnv) putSource(session, asset, objectID string, active bool, expected int64) (string, error) {
	return e.putSourceObject(session, "page", asset, objectID, active, expected)
}

// putSourceObject is putSource for any object (page|instagram).
func (e *lcnEnv) putSourceObject(session, object, asset, objectID string, active bool, expected int64) (string, error) {
	var id string
	err := platform.WithScope(context.Background(), e.f.runtime, e.h.token, e.f.storeA1, "store:read", func(tx pgx.Tx, _ platform.Scope) error {
		return tx.QueryRow(context.Background(), `SELECT live.put_claim_source($1::uuid,$2,$3,$4,$5,$6,$7,$8::bigint)::text`,
			session, object, asset, objectID, false, "zh-TW", active, expected).Scan(&id)
	})
	return id, err
}

func (e *lcnEnv) mustSource(t *testing.T, session, asset, objectID string, active bool) string {
	t.Helper()
	id, err := e.putSource(session, asset, objectID, active, 0)
	if err != nil {
		t.Fatalf("put_claim_source %s: %v", asset, err)
	}
	return id
}

// addPageSource adds one more Facebook Page route + active/inactive source (no token: the cap/refusal path
// never reaches a poll, and draft/archived sources are never candidates at all).
func (e *lcnEnv) addPageSource(t *testing.T, session string, active bool) (asset, sourceID string) {
	t.Helper()
	asset = miAsset()
	binding := miBinding(t, e.m, asset, "facebook", e.f.tenantA, e.f.storeA1, e.f.principalA)
	miRoute(t, e.m, asset, e.f.tenantA, e.f.storeA1, binding)
	sourceID = e.mustSource(t, session, asset, asset+"_"+mciDigits(10), active)
	return asset, sourceID
}

// console builds a poller/bridge replica; zero tunables are filled by ConsoleConfig.withDefaults.
func (e *lcnEnv) console(t *testing.T, holder string, cfg metareply.ConsoleConfig) *metareply.Console {
	t.Helper()
	if cfg.Graph.GraphBaseURL == "" {
		cfg.Graph.GraphBaseURL = e.graphURL
	}
	if cfg.Graph.GraphVersion == "" {
		cfg.Graph.GraphVersion = "v99.0"
	}
	if cfg.HolderID == "" {
		cfg.HolderID = holder
	}
	if len(cfg.BridgeToken) == 0 {
		cfg.BridgeToken = e.bridgeToken
	}
	if len(cfg.CursorKey) == 0 {
		cfg.CursorKey = e.cursorKey
	}
	c, err := metareply.NewConsole(e.worker, e.pageKeys, nil, cfg)
	if err != nil {
		t.Fatalf("NewConsole(%s): %v", holder, err)
	}
	return c
}

func (e *lcnEnv) lease(t *testing.T, sourceID string) (gen int64, holder string, ok bool) {
	t.Helper()
	err := e.f.owner.QueryRow(context.Background(), `SELECT generation, holder_id FROM live.comment_poll_leases WHERE source_id=$1`, sourceID).Scan(&gen, &holder)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, "", false
	}
	if err != nil {
		t.Fatalf("read poll lease: %v", err)
	}
	return gen, holder, true
}

func (e *lcnEnv) page(t *testing.T, handler http.Handler, sourceID string, after *metareply.Cursor, before *string, limit int) (int, string, metareply.BridgePage) {
	t.Helper()
	if limit == 0 {
		limit = 100
	}
	req := metareply.BridgePageRequest{TenantID: e.f.tenantA, StoreID: e.f.storeA1, SessionID: e.session, SourceID: sourceID, After: after, BeforeCursor: before, Limit: limit}
	var out metareply.BridgePage
	status, code := lcnPost(t, handler, e.bridgeToken, "/internal/v1/comment-page", req, &out)
	return status, code, out
}

func (e *lcnEnv) pageAuth(t *testing.T, handler http.Handler, sourceID string, token []byte) (int, string, metareply.BridgePage) {
	t.Helper()
	req := metareply.BridgePageRequest{TenantID: e.f.tenantA, StoreID: e.f.storeA1, SessionID: e.session, SourceID: sourceID, Limit: 100}
	var out metareply.BridgePage
	status, code := lcnPost(t, handler, token, "/internal/v1/comment-page", req, &out)
	return status, code, out
}

func (e *lcnEnv) facts(t *testing.T, handler http.Handler, sourceID, ref string) (int, string, metareply.CommentFacts) {
	t.Helper()
	req := metareply.BridgeFactsRequest{TenantID: e.f.tenantA, StoreID: e.f.storeA1, SessionID: e.session, SourceID: sourceID, CommentRef: ref}
	var out metareply.CommentFacts
	status, code := lcnPost(t, handler, e.bridgeToken, "/internal/v1/comment-facts", req, &out)
	return status, code, out
}

// demandAndPoll is the demand path: a page read is the demand that acquires the source, then one sweep polls it.
func (e *lcnEnv) demandAndPoll(t *testing.T, c *metareply.Console, sourceID string) metareply.BridgePage {
	t.Helper()
	if status, code, _ := e.page(t, c.Handler(), sourceID, nil, nil, 100); status != http.StatusOK {
		t.Fatalf("demand page status=%d code=%s", status, code)
	}
	c.SweepOnce(context.Background())
	status, code, page := e.page(t, c.Handler(), sourceID, nil, nil, 100)
	if status != http.StatusOK {
		t.Fatalf("post-poll page status=%d code=%s", status, code)
	}
	return page
}

func lcnPost(t *testing.T, handler http.Handler, token []byte, path string, body, out any) (status int, code string) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal bridge body: %v", err)
	}
	r := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	r.Header.Set("Content-Type", "application/json")
	if token != nil {
		r.Header.Set("Authorization", "Bearer "+base64.StdEncoding.EncodeToString(token))
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	status = w.Code
	if status == http.StatusOK && out != nil {
		if err := json.Unmarshal(w.Body.Bytes(), out); err != nil {
			t.Fatalf("decode bridge response %s: %v (body %s)", path, err, w.Body.String())
		}
	} else if status != http.StatusOK {
		var e struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(w.Body.Bytes(), &e) == nil {
			code = e.Error
		}
	}
	return status, code
}

// lcnCredLoad runs the lease-fenced credential loader with an explicit generation/token (token-fence probes).
func lcnCredLoad(t *testing.T, e *lcnEnv, sourceID string, gen int64, token []byte) error {
	t.Helper()
	var dummy int
	return e.worker.QueryRow(context.Background(), `SELECT 1 FROM integration.load_meta_page_token_for_poll($1::uuid,$2::bigint,$3::bytea)`, sourceID, gen, token).Scan(&dummy)
}

// ---------------------------------------------------------------------------------------
// LCN01 — poller leases, caps, buffer life and cursors
// ---------------------------------------------------------------------------------------

func TestLiveConsoleLCN01LeaseAndTokenFence(t *testing.T) {
	e := lcnSetup(t)
	cA := e.console(t, "worker-a", metareply.ConsoleConfig{})
	cB := e.console(t, "worker-b", metareply.ConsoleConfig{})
	cA.SweepOnce(context.Background())
	if n := countRows(t, e.f.owner, `SELECT count(*) FROM live.comment_poll_leases WHERE source_id=$1`, e.sourceID); n != 1 {
		t.Fatalf("lease rows=%d want 1", n)
	}
	gen, holder, ok := e.lease(t, e.sourceID)
	if !ok || holder != "worker-a" || gen != 1 {
		t.Fatalf("lease gen=%d holder=%s ok=%v", gen, holder, ok)
	}
	// A second replica must not take over an unexpired lease (I23: at most one poller per source fleet-wide).
	cB.SweepOnce(context.Background())
	gen2, holder2, _ := e.lease(t, e.sourceID)
	if gen2 != 1 || holder2 != "worker-a" {
		t.Fatalf("lease moved to gen=%d holder=%s", gen2, holder2)
	}
	// Token fence: wrong token / stale generation / short token / unknown source.
	requirePGCode(t, lcnCredLoad(t, e, e.sourceID, gen, randomBytes(32)), "40001", "wrong lease token")
	requirePGCode(t, lcnCredLoad(t, e, e.sourceID, gen+1, randomBytes(32)), "40001", "stale generation")
	requirePGCode(t, lcnCredLoad(t, e, e.sourceID, gen, randomBytes(16)), "22023", "short lease token")
	requirePGCode(t, lcnCredLoad(t, e, randomUUID(), 1, randomBytes(32)), "P0002", "unknown source")
}

func TestLiveConsoleLCN01CapsAndNoPoll(t *testing.T) {
	e := lcnSetup(t)
	// One console slot: the first demand holds it; the second source is refused before any poll
	// (a read is the demand; Validate forces TenantCap <= FleetCap, so 1/1 is the single-slot cap).
	c := e.console(t, "worker-cap", metareply.ConsoleConfig{FleetCap: 1, TenantCap: 1})
	if status, code, _ := e.page(t, c.Handler(), e.sourceID, nil, nil, 100); status != http.StatusOK {
		t.Fatalf("first demand status=%d code=%s", status, code)
	}
	_, srcB := e.addPageSource(t, e.session, true)
	if status, code, _ := e.page(t, c.Handler(), srcB, nil, nil, 100); status != 421 || code != "not_owner" {
		t.Fatalf("second demand status=%d code=%s want 421 not_owner", status, code)
	}
	if n := countRows(t, e.f.owner, `SELECT count(*) FROM live.comment_poll_leases WHERE source_id IN ($1,$2)`, e.sourceID, srcB); n != 1 {
		t.Fatalf("lease rows=%d want 1 (cap refused the second source)", n)
	}
	// Tenant cap with fleet room: a per-tenant cap of 1 still binds (a second console, two sources).
	c2 := e.console(t, "worker-tenantcap", metareply.ConsoleConfig{TenantCap: 1})
	_, srcC := e.addPageSource(t, e.session, true)
	if status, _, _ := e.page(t, c2.Handler(), srcC, nil, nil, 100); status != http.StatusOK {
		t.Fatalf("tenant-cap first demand status=%d", status)
	}
	_, srcD := e.addPageSource(t, e.session, true)
	if status, code, _ := e.page(t, c2.Handler(), srcD, nil, nil, 100); status != 421 || code != "not_owner" {
		t.Fatalf("tenant-cap second demand status=%d code=%s want 421 not_owner", status, code)
	}
	// A never-opened session (CLOSED gen-0 window, no OPEN window) and archived (inactive) sources are
	// never poll candidates. The CLOSED window row exists so the claim_sources→claim_windows FK is
	// satisfiable; the poller excludes it because there is no OPEN window and no demand.
	draftSession := e.h.draft(t, e.f.storeA1)
	e.h.mustWindow(t, draftSession, 0, claims.WindowClosed, claims.MatchExact)
	_, draftSrc := e.addPageSource(t, draftSession, true)
	_, archivedSrc := e.addPageSource(t, e.session, false)
	for label, src := range map[string]string{"draft": draftSrc, "archived": archivedSrc} {
		if n := countRows(t, e.worker, `SELECT count(*) FROM live.comment_poll_sources() WHERE source_id=$1`, src); n != 0 {
			t.Fatalf("%s source is a poll candidate (%d rows)", label, n)
		}
	}
	c3 := e.console(t, "worker-nopoll", metareply.ConsoleConfig{})
	c3.SweepOnce(context.Background())
	if n := countRows(t, e.f.owner, `SELECT count(*) FROM live.comment_poll_leases WHERE source_id IN ($1,$2)`, draftSrc, archivedSrc); n != 0 {
		t.Fatalf("draft/archived source acquired a lease")
	}
}

func TestLiveConsoleLCN01BufferCapAgeAndCursor(t *testing.T) {
	t.Run("ring-cap", func(t *testing.T) {
		e := lcnSetup(t)
		now := time.Now().UTC()
		refs := []string{lcnRef(), lcnRef(), lcnRef()}
		e.graph.setComments(e.postID, []map[string]any{
			lcnComment(refs[0], now.Add(-3*time.Minute).Format(time.RFC3339), e.asset, "", "c0", "", false),
			lcnComment(refs[1], now.Add(-2*time.Minute).Format(time.RFC3339), e.asset, "", "c1", "", false),
			lcnComment(refs[2], now.Add(-time.Minute).Format(time.RFC3339), e.asset, "", "c2", "", false),
		})
		c := e.console(t, "worker-cap", metareply.ConsoleConfig{BufferCap: 2})
		page := e.demandAndPoll(t, c, e.sourceID)
		if len(page.Items) != 2 {
			t.Fatalf("kept %d want 2", len(page.Items))
		}
		if page.Items[0].Ref != refs[2] || page.Items[1].Ref != refs[1] {
			t.Fatalf("buffer items %+v (want newest-first c2,c1)", page.Items)
		}
	})
	t.Run("age-out", func(t *testing.T) {
		e := lcnSetup(t)
		old := time.Now().UTC().Add(-3 * time.Hour)
		e.graph.setComments(e.postID, []map[string]any{lcnComment(lcnRef(), old.Format(time.RFC3339), e.asset, "", "old", "", false)})
		c := e.console(t, "worker-age", metareply.ConsoleConfig{BufferAge: time.Hour})
		page := e.demandAndPoll(t, c, e.sourceID)
		if len(page.Items) != 0 {
			t.Fatalf("aged-out kept %d", len(page.Items))
		}
	})
	t.Run("idle-drop-reset", func(t *testing.T) {
		e := lcnSetup(t)
		now := time.Now().UTC()
		e.graph.setComments(e.postID, []map[string]any{lcnComment(lcnRef(), now.Format(time.RFC3339), e.asset, "", "x", "", false)})
		c := e.console(t, "worker-idle", metareply.ConsoleConfig{IdleDrop: 3 * time.Second}) // wide enough that a loaded shared runner cannot drop it before the first poll
		page1 := e.demandAndPoll(t, c, e.sourceID)
		if page1.Epoch != 1 || len(page1.Items) != 1 {
			t.Fatalf("page1 epoch=%d items=%d", page1.Epoch, len(page1.Items))
		}
		time.Sleep(3500 * time.Millisecond)
		c.SweepOnce(context.Background()) // idle drop removes the in-memory buffer (lease row stays)
		// Force a take-over of the still-held-but-now-expired lease; the fresh buffer bumps poll_epoch → reset:true.
		mustExec(t, e.f.owner, `UPDATE live.comment_poll_leases SET lease_until=clock_timestamp()-interval '1 second' WHERE source_id=$1`, e.sourceID)
		status, code, page2 := e.page(t, c.Handler(), e.sourceID, nil, nil, 100)
		if status != http.StatusOK {
			t.Fatalf("re-demand status=%d code=%s", status, code)
		}
		if page2.Epoch != 2 || len(page2.Items) != 0 {
			t.Fatalf("page2 epoch=%d items=%d (want fresh reset 2/0)", page2.Epoch, len(page2.Items))
		}
	})
	t.Run("cursor-tamper-and-paging-next", func(t *testing.T) {
		e := lcnSetup(t)
		now := time.Now().UTC()
		e.graph.setComments(e.postID, []map[string]any{lcnComment(lcnRef(), now.Format(time.RFC3339), e.asset, "", "y", "", false)})
		e.graph.setNext(e.graphURL + "/v99.0/next-page-marker")
		c := e.console(t, "worker-cursor", metareply.ConsoleConfig{})
		page := e.demandAndPoll(t, c, e.sourceID)
		if page.OlderCursor == nil {
			t.Fatal("no older_cursor")
		}
		tampered := *page.OlderCursor + "x"
		if status, code, _ := e.page(t, c.Handler(), e.sourceID, nil, &tampered, 100); status != 400 || code != "invalid_cursor" {
			t.Fatalf("tampered cursor status=%d code=%s want 400 invalid_cursor", status, code)
		}
		before := *page.OlderCursor
		status, code, older := e.page(t, c.Handler(), e.sourceID, nil, &before, 100)
		if status != http.StatusOK || len(older.Items) != 1 {
			t.Fatalf("older page status=%d code=%s items=%d", status, code, len(older.Items))
		}
		if e.graph.hasTarget("/next-page-marker") {
			t.Fatal("poller followed paging.next")
		}
		if !e.graph.hasTarget("before=BEFORE_CURSOR_0") {
			t.Fatal("backfill did not send the signed before cursor")
		}
		for _, hit := range e.graph.hits() {
			if strings.Contains(hit.target, e.pageToken) {
				t.Fatalf("page token leaked into URL %q", hit.target)
			}
			if strings.HasPrefix(hit.target, "GET /v99.0/") && strings.Contains(hit.target, "/comments") && !hit.bearer {
				t.Fatalf("Graph read without Authorization header: %q", hit.target)
			}
		}
	})
}

// ---------------------------------------------------------------------------------------
// LCN02 — bridge statuses and comment-facts
// ---------------------------------------------------------------------------------------

func TestLiveConsoleLCN02BridgeStatusesAndFacts(t *testing.T) {
	e := lcnSetup(t)
	now := time.Now().UTC()
	pageRef := lcnRef()
	replyRef := lcnRef()
	e.graph.setComments(e.postID, []map[string]any{
		lcnComment(pageRef, now.Add(-2*time.Minute).Format(time.RFC3339), e.asset, "Page Name", "page comment", "", false),
		lcnComment(replyRef, now.Add(-time.Minute).Format(time.RFC3339), mciDigits(16), "Buyer Name", "buyer reply", pageRef, true),
	})
	c := e.console(t, "worker-facts", metareply.ConsoleConfig{})
	// 401 no / wrong bearer before any database access.
	if status, code, _ := e.pageAuth(t, c.Handler(), e.sourceID, nil); status != 401 || code != "auth" {
		t.Fatalf("no-auth status=%d code=%s", status, code)
	}
	if status, code, _ := e.pageAuth(t, c.Handler(), e.sourceID, randomBytes(32)); status != 401 || code != "auth" {
		t.Fatalf("wrong-auth status=%d code=%s", status, code)
	}
	// 404 no_source for an unknown source id.
	if status, code, _ := e.page(t, c.Handler(), randomUUID(), nil, nil, 100); status != 404 || code != "no_source" {
		t.Fatalf("unknown source status=%d code=%s", status, code)
	}
	// demand + poll so facts resolve from the ring buffer.
	page := e.demandAndPoll(t, c, e.sourceID)
	if len(page.Items) != 2 {
		t.Fatalf("polled %d items", len(page.Items))
	}
	if status, code, facts := e.facts(t, c.Handler(), e.sourceID, pageRef); status != 200 || !facts.Found || !facts.IsPage || facts.IsReply {
		t.Fatalf("page facts %d %s %+v", status, code, facts)
	}
	if status, code, facts := e.facts(t, c.Handler(), e.sourceID, replyRef); status != 200 || !facts.Found || facts.IsPage || !facts.IsReply {
		t.Fatalf("reply facts %d %s %+v", status, code, facts)
	}
	// Unknown ref → one Graph read → 404 → Found=false (never an error).
	if status, code, facts := e.facts(t, c.Handler(), e.sourceID, lcnRef()); status != 200 || facts.Found {
		t.Fatalf("unknown facts %d %s %+v", status, code, facts)
	}
	// invalid_ref shape → 400.
	if status, code, _ := e.facts(t, c.Handler(), e.sourceID, "BAD-REF!"); status != 400 || code != "invalid_ref" {
		t.Fatalf("bad ref status=%d code=%s", status, code)
	}
	// 421 not_owner: a second replica while the first holds the unexpired lease.
	cB := e.console(t, "worker-facts-b", metareply.ConsoleConfig{})
	if status, code, _ := e.page(t, cB.Handler(), e.sourceID, nil, nil, 100); status != 421 || code != "not_owner" {
		t.Fatalf("not-owner status=%d code=%s", status, code)
	}
}

// ---------------------------------------------------------------------------------------
// LCN04 — comment text/name/PSID never persisted or logged
// ---------------------------------------------------------------------------------------

func TestLiveConsoleLCN04NoCommentTextPersisted(t *testing.T) {
	e := lcnSetup(t)
	logs := lcServerLog(t, e.f)
	tag := t04Tag()
	text := "LCN04-SENTINEL-TEXT-" + tag
	name := "LCN04-SENTINEL-NAME-" + tag
	psid := mciDigits(16)
	ref := lcnRef()
	now := time.Now().UTC()
	e.graph.setComments(e.postID, []map[string]any{lcnComment(ref, now.Format(time.RFC3339), psid, name, text, "", false)})
	c := e.console(t, "worker-leak", metareply.ConsoleConfig{})
	page := e.demandAndPoll(t, c, e.sourceID)
	if len(page.Items) != 1 || page.Items[0].Text != text {
		t.Fatalf("polled %+v", page.Items)
	}
	// The worker's ring buffer is the only copy; no base table persists the text, name or buyer PSID.
	for needle, label := range map[string]string{text: "text", name: "name", psid: "psid"} {
		if hits := lcFind(t, e.f, needle, nil); len(hits) != 0 {
			t.Fatalf("comment %s persisted in %v", label, hits)
		}
	}
	// The PostgreSQL server log must not echo any of it either (no ERROR DETAIL with the comment).
	for _, needle := range []string{text, name, psid} {
		if strings.Contains(logs(), needle) {
			t.Fatalf("server log leaked %q", needle)
		}
	}
}

// ---------------------------------------------------------------------------------------
// LCN05 — print idempotency (A3) and marks mapping (A2)
// ---------------------------------------------------------------------------------------

func TestLiveConsoleLCN05PrintAndMarks(t *testing.T) {
	e := lcnSetup(t)
	now := time.Now().UTC()
	ref := lcnRef()
	text := "LCN05-print-label-" + t04Tag()
	e.graph.setComments(e.postID, []map[string]any{lcnComment(ref, now.Format(time.RFC3339), e.asset, "", text, "", false)})
	c := e.console(t, "worker-print", metareply.ConsoleConfig{})
	bridgeSrv := httptest.NewServer(c.Handler())
	t.Cleanup(bridgeSrv.Close)
	bridge, err := metareply.NewBridgeClient(bridgeSrv.URL, e.bridgeToken)
	if err != nil {
		t.Fatal(err)
	}
	stream, err := live.NewCommentStream(bridge, nil)
	if err != nil {
		t.Fatal(err)
	}
	page := e.demandAndPoll(t, c, e.sourceID)
	if len(page.Items) != 1 || page.Items[0].Ref != ref {
		t.Fatalf("polled %+v", page.Items)
	}
	// A3 print twice → idempotent count 1 then 2.
	print := func() live.CommentPrint {
		var out live.CommentPrint
		err := platform.WithScope(context.Background(), e.f.runtime, e.h.token, e.f.storeA1, "live:manage", func(tx pgx.Tx, s platform.Scope) error {
			var err error
			out, err = stream.PrintComment(context.Background(), tx, s, e.h.token, e.session, ref)
			return err
		})
		if err != nil {
			t.Fatalf("print: %v", err)
		}
		return out
	}
	if p := print(); p.PrintCount != 1 || p.LastPrintedAt == nil {
		t.Fatalf("print1 %+v", p)
	}
	if p := print(); p.PrintCount != 2 || p.LastPrintedAt == nil {
		t.Fatalf("print2 %+v", p)
	}
	// A2 read + marks join (no text join; the print fact is visible).
	var sp live.ConsoleStreamPage
	err = platform.WithScope(context.Background(), e.f.runtime, e.h.token, e.f.storeA1, "live:read", func(tx pgx.Tx, s platform.Scope) error {
		var err error
		sp, err = stream.Comments(context.Background(), tx, s, e.h.token, e.session, live.ConsolePageQuery{Limit: 50})
		return err
	})
	if err != nil {
		t.Fatalf("comments: %v", err)
	}
	if len(sp.Items) != 1 {
		t.Fatalf("comments items=%d", len(sp.Items))
	}
	it := sp.Items[0]
	if it.Text != text {
		t.Fatalf("comment text %q", it.Text)
	}
	if it.Marks.Printed == nil || it.Marks.Printed.Count != 2 {
		t.Fatalf("printed mark %+v", it.Marks.Printed)
	}
	if !it.Marks.PrivateReplyAvailable || it.Marks.PublicReplies != 0 || it.Marks.Intake != nil || it.Marks.Claim != nil || it.Marks.PrivateReply != nil {
		t.Fatalf("marks %+v", it.Marks)
	}
	// A3 against a foreign/cross-store session → 404.
	err = platform.WithScope(context.Background(), e.f.runtime, e.h.token, e.f.storeA1, "live:manage", func(tx pgx.Tx, s platform.Scope) error {
		_, err := stream.PrintComment(context.Background(), tx, s, e.h.token, randomUUID(), ref)
		return err
	})
	if !errors.Is(err, command.ErrNotFound) {
		t.Fatalf("foreign print err=%v want command.ErrNotFound", err)
	}
}
