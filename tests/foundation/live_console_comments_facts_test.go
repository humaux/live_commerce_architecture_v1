// Purpose: REAL_PG + MOCK-Graph gates for live-console-v1 Amendment 1 A1.2 (LC-B2 delta): the platform-branched
// bridge comment-facts read (FB and IG), the IG webhook-copy fallback social.read_comment_facts decrypted on
// the API side, and the facts_unavailable disable of manual private reply. LCN02 IG cases.
// Depends on: live_console_comments_test.go harness (lcnEnv, fake Graph), meta_* test helpers (real webhook ->
//
//	inbox -> consumer function), internal/live CommentStream, metareply Console/BridgeClient.
//
// Used by: bash scripts/dev/test-local.sh --live-console.
// Invariants: I11 (author id / text never persisted), I01 (scope from the authenticated transaction).
// Status: MOCK (loopback Graph; the IG Graph field set is probe R3 / LC-U12, NOT_RUN against Meta).
package foundation_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/claims"
	"livecommerce/internal/command"
	"livecommerce/internal/integrations/meta"
	"livecommerce/internal/integrations/metareply"
	"livecommerce/internal/live"
	"livecommerce/internal/platform"
)

// lcnIGFact is one synthetic IG Graph comment as the facts read returns it (bare object).
func lcnIGFact(ref, ts, fromID, parentID string) map[string]any {
	row := map[string]any{"id": ref, "timestamp": ts}
	if fromID != "" {
		row["from"] = map[string]any{"id": fromID}
	}
	if parentID != "" {
		row["parent_id"] = parentID
	}
	return row
}

// lcnIG is one claim-ready IG session: an IG route + binding + registered token + active IG source.
type lcnIG struct {
	m        miTest
	session  string
	asset    string
	media    string
	sourceID string
}

// lcnSetupIG adds a second live session whose only active source is an Instagram media.
func lcnSetupIG(t *testing.T, e *lcnEnv) *lcnIG {
	t.Helper()
	m := miInstagram(t, miSetup(t))
	if m.f != e.f {
		t.Fatal("meta fixture and live fixture must share one database fixture")
	}
	ig := &lcnIG{m: m, asset: miAsset(), media: mciDigits(17)}
	ig.session = e.h.draft(t, e.f.storeA1)
	// claim_sources needs the session's window row, and a store allows one OPEN window: close the FB session's
	// (facts are demand-driven and need no window) and open the IG session's.
	w := e.h.board(t, e.session).Window
	e.h.mustWindow(t, e.session, w.Version, claims.WindowClosed, w.MatchMode)
	e.h.open(t, ig.session, claims.MatchExact)
	binding := miBinding(t, m, ig.asset, "instagram", e.f.tenantA, e.f.storeA1, e.f.principalA)
	miInstagramRoute(t, m, ig.asset, binding)
	if _, err := metareply.RegisterPageToken(context.Background(), e.m.registrar, e.pageKeys, metareply.Registration{
		TenantID: e.f.tenantA, StoreID: e.f.storeA1, PrincipalID: e.h.actor, BindingID: binding, Provider: "instagram",
		AssetID: ig.asset, ExpectedVersion: 0, Scopes: []string{"pages_read_engagement"}}, "SENTINEL-IGQ-"+t04Tag()+t04Tag()); err != nil {
		t.Fatalf("register synthetic IG token: %v", err)
	}
	id, err := e.putSourceObject(ig.session, "instagram", ig.asset, ig.media, true, 0)
	if err != nil {
		t.Fatalf("put IG claim source: %v", err)
	}
	ig.sourceID = id
	return ig
}

// webhook posts one real signed IG comment webhook, runs the consumer projection so a social.comment_events
// row exists, and returns nothing: the copy is encrypted at rest and only the API can open it.
func (ig *lcnIG) webhook(t *testing.T, media, ref, fromID, parentID string, at time.Time) {
	t.Helper()
	var extra map[string]any
	if parentID != "" {
		extra = map[string]any{"parent_id": parentID}
	}
	ev := mcPost(t, ig.m, ig.asset, mciIGBody(ig.asset, "live_comments", media, ref, fromID, "ig_user", "LCN-IG-TEXT-"+t04Tag(), &at, extra))
	mcRunning(t, ig.m, ev, 1)
	if _, err := mcConsumer(t, ig.m).Exec(context.Background(), `SELECT meta_inbox.finish_social_event($1::uuid,$2::bigint,$3::integer,'comment',$4)`,
		ev.id, ev.job, 1, crCommentKey(miApp, "instagram", ig.asset, ref)); err != nil {
		t.Fatalf("project IG comment: %v", err)
	}
}

// lcnStream builds a CommentStream over a real bridge (httptest server of the Console handler).
func (e *lcnEnv) lcnStream(t *testing.T, c *metareply.Console, keys *meta.PayloadKeyring) *live.CommentStream {
	t.Helper()
	srv := httptest.NewServer(c.Handler())
	t.Cleanup(srv.Close)
	bridge, err := metareply.NewBridgeClient(srv.URL, e.bridgeToken)
	if err != nil {
		t.Fatal(err)
	}
	stream, err := live.NewCommentStream(bridge, keys)
	if err != nil {
		t.Fatal(err)
	}
	return stream
}

// lcnFacts runs CommentStream.Facts in a live:read scope transaction.
func (e *lcnEnv) lcnFacts(stream *live.CommentStream, session, ref string) (live.CommentFacts, error) {
	var out live.CommentFacts
	err := platform.WithScope(context.Background(), e.f.runtime, e.h.token, e.f.storeA1, "live:read", func(tx pgx.Tx, s platform.Scope) error {
		var err error
		out, err = stream.Facts(context.Background(), tx, s, e.h.token, session, ref)
		return err
	})
	return out, err
}

// lcnPRMarks runs CommentStream.PrivateReplyMarks in a live:read scope transaction.
func (e *lcnEnv) lcnPRMarks(stream *live.CommentStream, session, ref string) (live.ConsoleMarks, error) {
	var out live.ConsoleMarks
	err := platform.WithScope(context.Background(), e.f.runtime, e.h.token, e.f.storeA1, "live:read", func(tx pgx.Tx, s platform.Scope) error {
		var err error
		out, err = stream.PrivateReplyMarks(context.Background(), tx, s, e.h.token, session, ref)
		return err
	})
	return out, err
}

// LCN02 (IG): bridge comment-facts is platform-branched. The IG Graph branch asks the IG field set, derives
// created_at from `timestamp`, is_page from from.id == asset, is_reply from parent_id; a comment whose author
// cannot be confirmed is found:false (never a guessed is_page=false). FB keeps its own field set.
func TestLiveConsoleLCN02IGFactsBridgeGraphBranch(t *testing.T) {
	e := lcnSetup(t)
	ig := lcnSetupIG(t, e)
	c := e.console(t, "worker-igfacts", metareply.ConsoleConfig{})
	igFacts := func(ref string) (int, string, metareply.CommentFacts) {
		req := metareply.BridgeFactsRequest{TenantID: e.f.tenantA, StoreID: e.f.storeA1, SessionID: ig.session, SourceID: ig.sourceID, CommentRef: ref}
		var out metareply.CommentFacts
		status, code := lcnPost(t, c.Handler(), e.bridgeToken, "/internal/v1/comment-facts", req, &out)
		return status, code, out
	}
	ts := time.Now().UTC().Add(-3 * time.Minute).Truncate(time.Second)
	pageRef, buyerRef, replyRef, noAuthRef := lcnRef(), lcnRef(), lcnRef(), lcnRef()
	e.graph.setRef(pageRef, lcnIGFact(pageRef, ts.Format("2006-01-02T15:04:05-0700"), ig.asset, ""))
	e.graph.setRef(buyerRef, lcnIGFact(buyerRef, ts.Format("2006-01-02T15:04:05-0700"), mciDigits(16), ""))
	e.graph.setRef(replyRef, lcnIGFact(replyRef, ts.Format("2006-01-02T15:04:05-0700"), mciDigits(16), buyerRef))
	e.graph.setRef(noAuthRef, lcnIGFact(noAuthRef, ts.Format("2006-01-02T15:04:05-0700"), "", ""))

	if status, code, f := igFacts(pageRef); status != 200 || !f.Found || !f.IsPage || f.IsReply || f.CreatedAt == nil || !f.CreatedAt.Equal(ts) {
		t.Fatalf("IG page facts %d %s %+v (want created_at %s)", status, code, f, ts)
	}
	if status, code, f := igFacts(buyerRef); status != 200 || !f.Found || f.IsPage || f.IsReply {
		t.Fatalf("IG buyer facts %d %s %+v", status, code, f)
	}
	if status, code, f := igFacts(replyRef); status != 200 || !f.Found || f.IsPage || !f.IsReply {
		t.Fatalf("IG reply facts %d %s %+v", status, code, f)
	}
	if status, code, f := igFacts(noAuthRef); status != 200 || f.Found {
		t.Fatalf("IG comment without a confirmable author must be found:false, got %d %s %+v", status, code, f)
	}
	if status, code, f := igFacts(lcnRef()); status != 200 || f.Found {
		t.Fatalf("IG unknown ref %d %s %+v", status, code, f)
	}
	// The IG branch asked for exactly the IG field set, with the token in the header and never in the URL.
	var sawIG bool
	for _, h := range e.graph.hits() {
		u, err := url.Parse(h.target)
		if err != nil || u.Query().Get("fields") != "timestamp,from{id},parent_id" {
			continue
		}
		sawIG = true
		if !h.bearer {
			t.Fatalf("IG facts read without the bearer header: %+v", h)
		}
	}
	if !sawIG {
		t.Fatalf("no IG-branch Graph read observed: %+v", e.graph.hits())
	}
	// The FB source of the same console keeps the FB field set (no message/name pulled for facts).
	fbRef := lcnRef()
	e.graph.setRef(fbRef, lcnComment(fbRef, time.Now().UTC().Format(time.RFC3339), mciDigits(16), "N", "T", "", false))
	if status, code, f := e.facts(t, c.Handler(), e.sourceID, fbRef); status != 200 || !f.Found || f.IsPage {
		t.Fatalf("FB facts %d %s %+v", status, code, f)
	}
	var sawFB bool
	for _, h := range e.graph.hits() {
		if u, err := url.Parse(h.target); err == nil && u.Query().Get("fields") == "created_time,from{id},parent{id}" {
			sawFB = true
		}
	}
	if !sawFB {
		t.Fatalf("no FB-branch Graph read observed: %+v", e.graph.hits())
	}
}

// LCN02 (IG): the API-side lookup order and facts_unavailable. (a) Graph via the bridge, (b) the encrypted IG
// webhook copy (social.read_comment_facts, decrypted on the API side), (c) neither -> ErrFactsUnavailable and
// marks reason facts_unavailable. Claims-worker is never given the payload keyring.
func TestLiveConsoleLCN02IGFactsFallbackAndUnavailable(t *testing.T) {
	e := lcnSetup(t)
	ig := lcnSetupIG(t, e)
	c := e.console(t, "worker-igapi", metareply.ConsoleConfig{})
	keys := mcKeys(t, ig.m)
	stream := e.lcnStream(t, c, keys)
	at := time.Now().UTC().Add(-30 * time.Second)

	// (a) Graph answers: no webhook copy needed.
	graphRef := lcnRef()
	ts := time.Now().UTC().Add(-2 * time.Minute).Truncate(time.Second)
	e.graph.setRef(graphRef, lcnIGFact(graphRef, ts.Format("2006-01-02T15:04:05-0700"), mciDigits(16), ""))
	f, err := e.lcnFacts(stream, ig.session, graphRef)
	if err != nil || !f.Found || f.IsPage || f.IsReply || !f.CreatedAt.Equal(ts) {
		t.Fatalf("graph facts %+v err=%v", f, err)
	}

	// (b) Graph 404 -> webhook copy: page comment, reply, plain buyer comment.
	// IG comment ids are plain digits (the webhook normalizer rejects anything else).
	pageRef, replyRef, plainRef, otherMedia := mciDigits(17), mciDigits(17), mciDigits(17), mciDigits(17)
	authorSentinel := mciDigits(16)
	ig.webhook(t, ig.media, pageRef, ig.asset, "", at)
	ig.webhook(t, ig.media, replyRef, authorSentinel, pageRef, at)
	ig.webhook(t, ig.media, plainRef, mciDigits(16), "", at)
	ig.webhook(t, mciDigits(17), otherMedia, mciDigits(16), "", at) // another media of the same IG asset
	for name, tc := range map[string]struct {
		ref           string
		page, isReply bool
	}{"page": {pageRef, true, false}, "reply": {replyRef, false, true}, "plain": {plainRef, false, false}} {
		got, err := e.lcnFacts(stream, ig.session, tc.ref)
		if err != nil || !got.Found || got.IsPage != tc.page || got.IsReply != tc.isReply {
			t.Fatalf("webhook facts %s %+v err=%v", name, got, err)
		}
		if d := got.CreatedAt.Sub(at); d < -time.Minute || d > time.Minute {
			t.Fatalf("webhook facts %s created_at=%s want near %s", name, got.CreatedAt, at)
		}
	}
	// A2 IG read-through (social.read_comment_events): the three comments of THIS media, not the other media's.
	var sp live.ConsoleStreamPage
	if err := platform.WithScope(context.Background(), e.f.runtime, e.h.token, e.f.storeA1, "live:read", func(tx pgx.Tx, s platform.Scope) error {
		var err error
		sp, err = stream.Comments(context.Background(), tx, s, e.h.token, ig.session, live.ConsolePageQuery{Limit: 50})
		return err
	}); err != nil {
		t.Fatalf("IG comments: %v", err)
	}
	seen := map[string]live.ConsoleComment{}
	for _, it := range sp.Items {
		seen[it.Ref] = it
	}
	if len(sp.Items) != 3 || !seen[pageRef].IsPage || seen[replyRef].ParentRef == nil || *seen[replyRef].ParentRef != pageRef || seen[plainRef].Ref == "" || sp.Stream.SourcePlatform != "instagram" {
		t.Fatalf("IG comments page %+v", sp.Items)
	}
	// The author id stays in API memory: it is not in any table (the copy is ciphertext) and not in a response.
	if hits := lcFind(t, e.f, authorSentinel, nil); len(hits) != 0 {
		t.Fatalf("IG author id persisted in clear: %v", hits)
	}

	// (c) neither Graph nor a copy -> facts_unavailable; a copy of a different media is not this source's.
	for name, ref := range map[string]string{"unknown": lcnRef(), "other-media": otherMedia} {
		if _, err := e.lcnFacts(stream, ig.session, ref); !errors.Is(err, live.ErrFactsUnavailable) {
			t.Fatalf("%s: err=%v want ErrFactsUnavailable", name, err)
		}
	}
	m, err := e.lcnPRMarks(stream, ig.session, lcnRef())
	if err != nil || m.PrivateReplyAvailable || m.PrivateReplyUnavailableReason != live.FactsUnavailableReason {
		t.Fatalf("unavailable marks %+v err=%v", m, err)
	}
	if m, err = e.lcnPRMarks(stream, ig.session, pageRef); err != nil || !m.PrivateReplyAvailable || m.PrivateReplyUnavailableReason != "" {
		t.Fatalf("available marks %+v err=%v", m, err)
	}
	// An existing used/auto_pending reason keeps precedence over facts_unavailable.
	if got := (live.ConsoleMarks{PrivateReplyAvailable: false, PrivateReplyUnavailableReason: "used"}).WithFactsUnavailable(); got.PrivateReplyUnavailableReason != "used" {
		t.Fatalf("used reason overwritten: %+v", got)
	}

	// An api process without the payload keyring cannot read the copy: still facts_unavailable, never a guess.
	noKeys := e.lcnStream(t, c, nil)
	if _, err := e.lcnFacts(noKeys, ig.session, pageRef); !errors.Is(err, live.ErrFactsUnavailable) {
		t.Fatalf("no keyring: err=%v want ErrFactsUnavailable", err)
	}

	// A foreign session id is not found; a malformed ref is invalid (no SQL, no Graph).
	if _, err := e.lcnFacts(stream, randomUUID(), pageRef); !errors.Is(err, command.ErrNotFound) {
		t.Fatalf("foreign session err=%v", err)
	}
	if _, err := e.lcnFacts(stream, ig.session, "BAD-REF!"); !errors.Is(err, live.ErrInvalidRef) {
		t.Fatalf("bad ref err=%v", err)
	}
	// FB source: unknown to Graph is Found=false (A4 comment_unknown), not facts_unavailable.
	if f, err := e.lcnFacts(stream, e.session, lcnRef()); err != nil || f.Found {
		t.Fatalf("FB unknown %+v err=%v", f, err)
	}
	// A bridge outage on an IG comment with no copy is a retryable stream error, not "unavailable".
	dead := httptest.NewServer(http.NotFoundHandler())
	deadURL := dead.URL
	dead.Close()
	bridge, err := metareply.NewBridgeClient(deadURL, e.bridgeToken)
	if err != nil {
		t.Fatal(err)
	}
	downStream, _ := live.NewCommentStream(bridge, keys)
	if _, err := e.lcnFacts(downStream, ig.session, lcnRef()); !errors.Is(err, live.ErrStreamUnavailable) {
		t.Fatalf("bridge down err=%v want ErrStreamUnavailable", err)
	}
	// ...but the webhook copy still answers while the bridge is down.
	if f, err := e.lcnFacts(downStream, ig.session, pageRef); err != nil || !f.Found || !f.IsPage {
		t.Fatalf("copy with bridge down %+v err=%v", f, err)
	}
}
