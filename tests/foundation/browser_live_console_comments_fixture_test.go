//go:build browser

// Purpose: LC-U2a browser-only fixtures for the real A2/A4/A5/A8/A13 Go/PG seams.
// Depends on: lbSetup signed ingress/send worker, lcnEnv poller, MOCK loopback Graph,
// live.comment_poll_leases and msgtemplates.publish; no production credentials.
// Used by: TestBrowserLiveConsoleRealChain; controls return synthetic IDs and counts only.
// Invariants: I01/I06/I07/I11/I18; reset replaces a real worker buffer and increments its DB epoch.
package foundation_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/integrations/metareply"
	"livecommerce/internal/live"
	"livecommerce/internal/platform"
)

type consoleCommentsFixture struct {
	e                                                        *lbEnv
	poll                                                     *lcnEnv
	stream                                                   *live.CommentStream
	mu                                                       sync.Mutex
	bridgeMu                                                 sync.RWMutex
	console                                                  *metareply.Console
	rows                                                     []map[string]any
	resetRow                                                 map[string]any
	reset                                                    bool
	ids                                                      map[string]any
	requests, olderRequests, privateRequests, publicRequests int
}

// newConsoleCommentsFixture constructs a real session and seeds a signed ACCEPTED claim.
func newConsoleCommentsFixture(t *testing.T) *consoleCommentsFixture {
	t.Helper()
	e := lbSetup(t)
	e.onlySource(t, "facebook")
	f := e.h.f
	// Fixture setup disables automatic replies, preserving the real manual quota check.
	mustExec(t, f.owner, `UPDATE live.claim_sources SET private_reply=false WHERE id=$1`, e.srcFB)
	e.registerTokenVersion(t, "facebook", e.pageBinding, e.pageAsset, 2,
		[]string{"pages_read_engagement", "pages_messaging", "pages_manage_engagement"}, e.pageToken+"-poll")
	// MOCK capability evidence is explicit; the UI and planner still read the real scoped table.
	for _, capability := range []string{"read_comment", "private_reply", "reply_public"} {
		mustExec(t, f.owner, `INSERT INTO integration.binding_capabilities(tenant_id,store_id,binding_id,provider,capability,state,reason,evidence,checked_at,page_id)
		 VALUES($1,$2,$3,'facebook',$4,'ok','ok','MOCK',clock_timestamp(),$5)
		 ON CONFLICT (tenant_id,store_id,binding_id,capability) DO UPDATE SET state='ok',reason='ok',evidence='MOCK',checked_at=clock_timestamp()`, f.tenantA, f.storeA1, e.pageBinding, capability, e.pageID)
	}
	x := &consoleCommentsFixture{e: e, ids: map[string]any{"session": e.session, "total": 60, "platform": "facebook", "template_id": "lc-u2a-safe"}}
	graph := newLcnGraph()
	// Only the network edge is faked. Older-page requests return the first ten rows and end pagination.
	graphServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/comments") && r.URL.Query().Get("before") != "" {
			x.mu.Lock()
			rows := append([]map[string]any(nil), x.rows[:10]...)
			if x.reset {
				rows = []map[string]any{}
			}
			x.mu.Unlock()
			consoleJSON(w, 200, map[string]any{"data": rows, "paging": map[string]any{}})
			return
		}
		graph.ServeHTTP(w, r)
	}))
	t.Cleanup(graphServer.Close)
	x.poll = &lcnEnv{f: f, h: e.h, worker: e.worker, pageKeys: e.pageKeys, session: e.session, asset: e.pageAsset, binding: e.pageBinding,
		postID: e.postID, sourceID: e.srcFB, graph: graph, graphURL: graphServer.URL, bridgeToken: randomBytes(32), cursorKey: randomBytes(32)}
	at := time.Now().UTC()
	claimRef, replyRef, resetRef := lcnRef(), lcnRef(), lcnRef()
	replyRefs := make([]string, 0, 6)
	for i := 1; i <= 60; i++ {
		ref := lcnRef()
		if i == 59 {
			ref = replyRef
		}
		if i == 60 {
			ref = claimRef
		}
		if i >= 50 && i <= 55 {
			replyRefs = append(replyRefs, ref)
		}
		author, name := mciDigits(15), "LC-U2a synthetic author"
		if i == 58 {
			author, name = "", ""
		}
		row := lcnComment(ref, at.Add(time.Duration(i-60)*time.Second).Format(time.RFC3339), author, name, fmt.Sprintf("LC-U2a row %02d", i), "", i == 57)
		x.rows = append(x.rows, row)
		graph.setRef(ref, row)
		if i == 1 {
			x.ids["old_ref"] = ref
		}
	}
	x.ids["claim_ref"], x.ids["latest_ref"], x.ids["reply_ref"], x.ids["reset_ref"] = claimRef, claimRef, replyRef, resetRef
	x.ids["reply_refs"] = replyRefs
	x.resetRow = lcnComment(resetRef, at.Format(time.RFC3339), "", "", "LC-U2a reset row", "", false)
	graph.setRef(resetRef, x.resetRow)
	// Calls signed Meta ingress and the real intake worker (live-console-v1 §2.5): text remains encrypted.
	claimAt := time.Now().UTC().Add(time.Second)
	e.postFB(t, claimRef, "", "A1+1", &claimAt, nil)
	e.apply(t)
	claim := e.claimOf(t, claimRef)
	if claim.Outcome != "ACCEPTED" || claim.Bundle == "" {
		t.Fatal("LC-U2a synthetic claim not accepted")
	}
	x.ids["bundle"] = claim.Bundle
	// A flagged bundle exercises A8 unreplied + A13 without inventing a linked customer identity.
	mustExec(t, f.owner, `UPDATE claims.bundles SET link_pending_manual=true WHERE id=$1`, claim.Bundle)
	// Calls msgtemplates.publish in an authenticated merchant scope (live-console-v1 §11).
	err := platform.WithScope(context.Background(), f.runtime, e.h.token, f.storeA1, "live:manage", func(tx pgx.Tx, _ platform.Scope) error {
		_, err := tx.Exec(context.Background(), `SELECT * FROM msgtemplates.publish('lc-u2a-safe','LC-U2a safe template','Synthetic thanks',ARRAY['private_reply','public_reply'],true)`)
		return err
	})
	if err != nil {
		t.Fatal("LC-U2a template setup failed")
	}
	t.Cleanup(func() {
		mustExec(t, f.owner, `DELETE FROM msgtemplates.templates WHERE tenant_id=$1 AND store_id=$2 AND template_id='lc-u2a-safe'`, f.tenantA, f.storeA1)
	})
	graph.setComments(e.postID, x.rows)
	c := x.poll.console(t, "lc-u2a-initial", metareply.ConsoleConfig{})
	page := x.poll.demandAndPoll(t, c, e.srcFB)
	if len(page.Items) != 60 {
		t.Fatal("LC-U2a initial comment buffer not populated")
	}
	x.console = c
	bridgeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		x.bridgeMu.RLock()
		defer x.bridgeMu.RUnlock()
		x.console.SweepOnce(r.Context())
		x.console.Handler().ServeHTTP(w, r)
	}))
	t.Cleanup(bridgeServer.Close)
	client, err := metareply.NewBridgeClient(bridgeServer.URL, x.poll.bridgeToken)
	if err != nil {
		t.Fatal(err)
	}
	x.stream, err = live.NewCommentStream(client, nil)
	if err != nil {
		t.Fatal(err)
	}
	return x
}

// resetComments replaces the in-memory worker after expiring only this fixture's lease.
func (x *consoleCommentsFixture) resetComments(ctx context.Context) error {
	x.bridgeMu.Lock()
	defer x.bridgeMu.Unlock()
	x.mu.Lock()
	x.reset = true
	x.mu.Unlock()
	x.poll.graph.setComments(x.e.postID, []map[string]any{x.resetRow})
	if _, err := x.e.h.f.owner.Exec(ctx, `UPDATE live.comment_poll_leases SET lease_until=clock_timestamp()-interval '1 second' WHERE source_id=$1`, x.e.srcFB); err != nil {
		return err
	}
	c, err := metareply.NewConsole(x.e.worker, x.e.pageKeys, nil, metareply.ConsoleConfig{HolderID: "lc-u2a-reset", Graph: metareply.Config{GraphBaseURL: x.poll.graphURL, GraphVersion: "v99.0"}, BridgeToken: x.poll.bridgeToken, CursorKey: x.poll.cursorKey})
	if err != nil {
		return err
	}
	// A bridge demand acquires the fresh epoch. No product response is modified by the test.
	request := metareply.BridgePageRequest{TenantID: x.e.h.f.tenantA, StoreID: x.e.h.f.storeA1, SessionID: x.e.session, SourceID: x.e.srcFB, Limit: 100}
	raw, _ := json.Marshal(request)
	req := httptest.NewRequest(http.MethodPost, "/internal/v1/comment-page", strings.NewReader(string(raw))).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer "+base64.StdEncoding.EncodeToString(x.poll.bridgeToken))
	rr := httptest.NewRecorder()
	c.Handler().ServeHTTP(rr, req)
	if rr.Code != 200 {
		return fmt.Errorf("reset demand status %d", rr.Code)
	}
	c.SweepOnce(ctx)
	x.console = c
	return nil
}

// dispatch enqueues only this fixture's already planned replies onto its MOCK Graph worker.
func (x *consoleCommentsFixture) dispatch(ctx context.Context) error {
	_, err := x.e.h.f.owner.Exec(ctx, `UPDATE river.river_job SET queue=$1 WHERE state='available' AND id IN
	 (SELECT job_id FROM integration.operations WHERE tenant_id=$2 AND store_id=$3 AND request->>'session_id'=$4
	 AND action IN ('meta.private_reply','meta.public_reply'))`, x.e.queue, x.e.h.f.tenantA, x.e.h.f.storeA1, x.e.session)
	return err
}

// observe records only request counts; it never stores URI queries, headers or bodies.
func (x *consoleCommentsFixture) observe(r *http.Request) {
	prefix := "/v1/admin/stores/" + x.e.h.f.storeA1 + "/live-sessions/" + x.e.session + "/comments"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		return
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	if r.Method == http.MethodGet && r.URL.Path == prefix {
		x.requests++
		if r.URL.Query().Has("before_cursor") {
			x.olderRequests++
		}
	}
	if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/private-reply") {
		x.privateRequests++
	}
	if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/public-reply") {
		x.publicRequests++
	}
}

// facts emits counts for cadence/quota assertions; plaintext and external identities stay in memory.
func (x *consoleCommentsFixture) facts(ctx context.Context) (map[string]any, error) {
	x.mu.Lock()
	out := map[string]any{"requests": x.requests, "older_requests": x.olderRequests, "private_requests": x.privateRequests, "public_requests": x.publicRequests}
	x.mu.Unlock()
	var count int
	err := x.e.h.f.owner.QueryRow(ctx, `SELECT count(*) FROM integration.operations WHERE tenant_id=$1 AND store_id=$2 AND action='meta.private_reply' AND request->>'session_id'=$3 AND request->>'message_type'='manual_private_reply'`, x.e.h.f.tenantA, x.e.h.f.storeA1, x.e.session).Scan(&count)
	out["private_operations"], out["graph_sends"] = count, x.e.g.count()
	if err == nil {
		var unknown int
		err = x.e.h.f.owner.QueryRow(ctx, `SELECT count(*) FROM integration.operations WHERE tenant_id=$1 AND store_id=$2 AND action='meta.public_reply' AND request->>'session_id'=$3 AND state='UNKNOWN'`, x.e.h.f.tenantA, x.e.h.f.storeA1, x.e.session).Scan(&unknown)
		out["public_unknown"] = unknown
	}
	return out, err
}
