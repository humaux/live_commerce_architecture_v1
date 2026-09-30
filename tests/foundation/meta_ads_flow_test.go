package foundation_test

// MA04..MA07 (contracts/meta-ads-v1.md §9 rows MA04, MA05, MA06, MA07; §2 flow, §3 wire, §5 rules, §6 workers). Tier MOCK:
// the real merchant HTTP surface, the real ads sweepers and dispatcher (River queue `ads`), REAL_PG, and the fake Graph
// (tests/ads/fakegraph). Written from the contract and the frozen ads-core/ads-graph blocks; see meta_ads_harness_test.go for
// the disclosed owner-pool fixtures. Tables/functions touched: ads.* (through HTTP + sweepers), integration.operations /
// operation_events / bindings (read-back, aging), river.river_job (queue/priority read-back, holds), host = loopback fake.

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/platform"
	"livecommerce/tests/ads/fakegraph"
)

func TestMetaAdsMA04Chain(t *testing.T) {
	t.Run("full chain: campaign PAUSED, children ACTIVE, preflight, then the only spend switch", func(t *testing.T) {
		e := newAdsEnv(t, adsOpts{})
		d := e.newDraft(adsDraftIn{})
		if got := e.status(d); got != "DRAFT" {
			t.Fatalf("new draft status %s", got)
		}
		e.mustApprove(d)
		if got := e.status(d); got != "APPROVED" {
			t.Fatalf("approved draft status %s", got)
		}
		mark := e.g.Mark()
		e.mustPublish(d)
		e.driveTo(d, "campaign", 1)
		// AD3: the campaign exists PAUSED before anything else can exist; nothing activates until the switch
		camp := e.g.Objects("campaign")
		if len(camp) != 1 || camp[0].Status != "PAUSED" {
			t.Fatalf("campaign after create: %+v", camp)
		}
		e.driveTo(d, "ad", 1)
		if got := e.status(d); got != "REMOTE_PAUSED" && got != "SUBMITTING" {
			t.Errorf("status with 4 remote ids pinned and no activation: %s", got)
		}
		adsets, creatives, remoteAds := e.g.Objects("adset"), e.g.Objects("creative"), e.g.Objects("ad")
		if len(adsets) != 1 || len(creatives) != 1 || len(remoteAds) != 1 {
			t.Fatalf("objects: %d adsets %d creatives %d ads (want exactly one each: no create is ever repeated)", len(adsets), len(creatives), len(remoteAds))
		}
		// F9/F22: children are ACTIVE themselves, effective CAMPAIGN_PAUSED: nothing can deliver
		for _, o := range []fakegraph.Object{adsets[0], remoteAds[0]} {
			if o.Status != "ACTIVE" || e.g.EffectiveStatus(o.ID) != "CAMPAIGN_PAUSED" {
				t.Errorf("%s %s: status %s effective %s, want ACTIVE / CAMPAIGN_PAUSED", o.Kind, o.ID, o.Status, e.g.EffectiveStatus(o.ID))
			}
		}
		if e.g.Count(fakegraph.RouteStatusPost) != 0 {
			t.Fatal("a status POST happened before preflight/activate")
		}
		// AD4: one lifetime budget with an end time, in Meta's TWD unit (offset 1: NT$3000 = 300000 minor -> 3000), no daily budget
		body := string(e.createBody(fakegraph.RouteCreateAdset))
		if !strings.Contains(body, `"lifetime_budget":3000`) || !strings.Contains(body, `"end_time"`) || strings.Contains(body, "daily_budget") {
			t.Errorf("adset body: %s", body)
		}
		// the tag: every created object is named lc-<operation uuid> of the op that created it
		for kind, obj := range map[string]fakegraph.Object{"campaign": camp[0], "adset": adsets[0], "creative": creatives[0], "ad": remoteAds[0]} {
			op := e.mustOp(d, map[string]string{"campaign": "campaign", "adset": "adset", "creative": "creative", "ad": "ad"}[kind], 1)
			if obj.Name != "lc-"+op.ID || op.Ref != obj.ID {
				t.Errorf("%s: remote name %q id %s, op %s ref %s", kind, obj.Name, obj.ID, op.ID, op.Ref)
			}
		}
		// ad set parent / ad parent wiring
		if adsets[0].Parent != camp[0].ID || remoteAds[0].Parent != adsets[0].ID {
			t.Errorf("wiring: adset parent %s (campaign %s), ad parent %s (adset %s)", adsets[0].Parent, camp[0].ID, remoteAds[0].Parent, adsets[0].ID)
		}

		e.driveTo(d, "preflight", 1)
		act := e.driveTo(d, "activate", 1)
		if act.State != "SUCCEEDED" {
			t.Fatal("activate not SUCCEEDED")
		}
		if o, _ := e.g.Object(camp[0].ID); o.Status != "ACTIVE" {
			t.Errorf("campaign after activate: %s", o.Status)
		}
		for _, id := range []string{adsets[0].ID, remoteAds[0].ID} {
			if e.g.EffectiveStatus(id) != "ACTIVE" {
				t.Errorf("effective status of %s after activate: %s", id, e.g.EffectiveStatus(id))
			}
		}
		if got := e.status(d); got != "ACTIVE" {
			t.Errorf("draft status after activate: %s", got)
		}
		// request order on the wire: creates, then the preflight read, then the single status POST
		order := []string{}
		for _, r := range e.g.RequestsSince(mark) {
			switch r.Route {
			case fakegraph.RouteCreateCampaign, fakegraph.RouteCreateAdset, fakegraph.RouteCreateCreative, fakegraph.RouteCreateAd, fakegraph.RouteAccountGet, fakegraph.RouteStatusPost:
				order = append(order, r.Route)
			}
		}
		if got, want := strings.Join(order, ","), "create_campaign,create_adset,create_creative,create_ad,account_get,status_post"; got != want {
			t.Errorf("wire order\n got %s\nwant %s", got, want)
		}
		// AD11/§3: the BISU token never rides in a URL; only the granted token is ever used; no cross-account call
		for _, r := range e.g.RequestsSince(mark) {
			if r.TokenSource == "query" || strings.Contains(r.RawQuery, "access_token") || strings.Contains(r.Path, e.botToken) {
				t.Errorf("token in URL: %s %s?%s", r.Method, r.Path, r.RawQuery)
			}
			if r.Token != e.botToken {
				t.Errorf("request %d used token %q", r.Seq, r.Token)
			}
			if !strings.Contains(r.Version, adsVersion) {
				t.Errorf("version %s", r.Version)
			}
		}
		if v := e.g.Violations(); len(v) != 0 {
			t.Errorf("token->account isolation violations: %v", v)
		}
		// D4: queue ads, priority 3 for every non-pause op; the merchant runtime planned publish, sweeper the rest
		for _, o := range e.ops(d) {
			if o.Queue != "ads" || o.Priority != 3 {
				t.Errorf("op %s#%d on queue %q priority %d, want ads/3", o.Kind, o.Seq, o.Queue, o.Priority)
			}
		}
		if n := e.count(`SELECT count(*) FROM river.river_job j JOIN integration.operations o ON o.job_id=j.id WHERE o.store_id=$1 AND o.provider='meta_ads' AND j.queue<>'ads'`, e.store); n != 0 {
			t.Errorf("%d ads jobs outside queue ads", n)
		}
		// ops: purpose marketing, actor MERCHANT, principal = the approver (§2 step 7)
		var purpose, actor, principal string
		if err := e.f.owner.QueryRow(e.ctx, `SELECT purpose,actor_kind,principal_id::text FROM integration.operations WHERE id=$1`, act.ID).Scan(&purpose, &actor, &principal); err != nil {
			t.Fatal(err)
		}
		if purpose != "marketing" || actor != "MERCHANT" || principal != e.creator {
			t.Errorf("activate op purpose=%s actor=%s principal=%s", purpose, actor, principal)
		}
		// Check ran once per dispatch claim and never with a secret (its signature has none); mode is dispatch
		for _, c := range e.probe.Checks() {
			if c.Mode != "dispatch" {
				t.Errorf("Check of %s ran in mode %q", c.Action, c.Mode)
			}
		}
	})
}

// ownerReplica runs owner SQL with triggers off (session_replication_role=replica; fixture superuser, disclosed per test).
func (e *adsEnv) ownerReplica(q string, args ...any) {
	e.t.Helper()
	tx, err := e.f.owner.Begin(e.ctx)
	if err != nil {
		e.t.Fatal(err)
	}
	defer tx.Rollback(e.ctx)
	if _, err := tx.Exec(e.ctx, `SET LOCAL session_replication_role=replica`); err != nil {
		e.t.Fatal(err)
	}
	if _, err := tx.Exec(e.ctx, q, args...); err != nil {
		e.t.Fatalf("owner (replica) %.60s: %v", q, err)
	}
	if err := tx.Commit(e.ctx); err != nil {
		e.t.Fatal(err)
	}
}

// readyToActivate publishes an approved draft and drives it until preflight #1 is SUCCEEDED; the next advance sweep plans activate #1.
func (e *adsEnv) readyToActivate(in adsDraftIn) string {
	e.t.Helper()
	d := e.newDraft(in)
	e.mustApprove(d)
	e.mustPublish(d)
	e.driveTo(d, "preflight", 1)
	return d
}

// blockedActivate holds activate #1 inside its Check (op DISPATCHING, claimed, Check not yet decided), applies mutate, releases,
// and returns the settled operation. The fake's status-POST counter is compared by the caller.
func (e *adsEnv) blockedActivate(d string, mutate func()) adsOp {
	e.t.Helper()
	entered, release := e.gateCheck("meta.ads.activate")
	defer release()
	e.sweep("advance")
	select {
	case <-entered:
	case <-time.After(20 * time.Second):
		e.t.Fatalf("activate never reached Check:%s", e.dump())
	}
	if mutate != nil {
		mutate()
	}
	release()
	e.settle()
	return e.mustOp(d, "activate", 1)
}

func (e *adsEnv) wantBlocked(o adsOp, code string) {
	e.t.Helper()
	if o.State != "BLOCKED_POLICY" {
		e.t.Fatalf("%s#%d is %s/%s, want BLOCKED_POLICY %s", o.Kind, o.Seq, o.State, o.Code, code)
	}
	if code != "" && o.Code != code {
		e.t.Fatalf("%s#%d BLOCKED_POLICY code %q, want %q", o.Kind, o.Seq, o.Code, code)
	}
	if code == "" && (o.Code == "" || o.Code == "policy_denied") {
		e.t.Fatalf("%s#%d BLOCKED_POLICY code %q is not a coded denial (D3)", o.Kind, o.Seq, o.Code)
	}
}

func (e *adsEnv) mustPublish2(d string) { e.mustPublish(d) }

// createBody returns the request body the fake received for the last create of a route.
func (e *adsEnv) createBody(route string) []byte {
	var out []byte
	for _, r := range e.g.Requests() {
		if r.Route == route && r.Method == http.MethodPost {
			out = r.Body
		}
	}
	return out
}

func TestMetaAdsMA04Refusals(t *testing.T) {
	// Every case: the activate op reaches Check, is denied with a coded BLOCKED_POLICY, and NO status POST reaches Meta.
	cases := []struct {
		name   string
		code   string // "" = any coded denial (the contract names no code for this rule)
		mutate func(e *adsEnv, d string)
	}{
		{"approval row gone (activate without approval)", "", func(e *adsEnv, d string) {
			e.ownerReplica(`DELETE FROM ads.draft_approvals WHERE draft_id=$1`, d)
		}},
		{"stale hash: identity binding re-pointed after approval", "", func(e *adsEnv, d string) {
			e.ownerReplica(`UPDATE integration.bindings SET semantic_version=semantic_version+1 WHERE id=$1`, e.idBinding)
		}},
		{"approver lost ads:approve", "", func(e *adsEnv, d string) { e.revoke(e.creator, "ads:approve") }},
		{"over allowance (operator lowered it after approval)", "over_allowance", func(e *adsEnv, d string) {
			e.setSettings("SANDBOX", e.account, 100000, "TWD")
		}},
		{"stale preflight (older than 10 min)", "preflight_stale", func(e *adsEnv, d string) {
			e.age(e.mustOp(d, "preflight", 1).ID, 11*time.Minute)
		}},
		{"account not ready (st != 1 in the fresh preflight)", "account_not_ready", func(e *adsEnv, d string) {
			e.ownerReplica(`UPDATE integration.operations SET provider_reference=replace(provider_reference,'st=1','st=2') WHERE id=$1`, e.mustOp(d, "preflight", 1).ID)
		}},
		{"preflight currency differs from the draft currency", "account_not_ready", func(e *adsEnv, d string) {
			e.ownerReplica(`UPDATE integration.operations SET provider_reference=replace(provider_reference,'cur=TWD','cur=USD') WHERE id=$1`, e.mustOp(d, "preflight", 1).ID)
		}},
		{"SANDBOX environment with a non-sandbox ad account", "sandbox_account_required", func(e *adsEnv, d string) {
			e.setSettings("SANDBOX", "999"+adsDigits(8), e.opts.allowance, "TWD")
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newAdsEnv(t, adsOpts{})
			d := e.readyToActivate(adsDraftIn{})
			posts := e.g.Count(fakegraph.RouteStatusPost)
			o := e.blockedActivate(d, func() { c.mutate(e, d) })
			e.wantBlocked(o, c.code)
			if got := e.g.Count(fakegraph.RouteStatusPost); got != posts {
				t.Fatalf("a status POST reached Meta for a denied activate (%d -> %d)", posts, got)
			}
			if e.status(d) == "ACTIVE" {
				t.Fatal("draft ACTIVE after a denied activate")
			}
			if o, _ := e.g.Object(e.g.Objects("campaign")[0].ID); o.Status != "PAUSED" {
				t.Fatalf("campaign left %s", o.Status)
			}
			t.Logf("EVIDENCE %s -> BLOCKED_POLICY %s", c.name, o.Code)
		})
	}

	t.Run("stale preflight: the sweeper plans preflight seq+1 and activate seq+1, and the spend switch then goes through", func(t *testing.T) {
		e := newAdsEnv(t, adsOpts{})
		d := e.readyToActivate(adsDraftIn{})
		o := e.blockedActivate(d, func() { e.age(e.mustOp(d, "preflight", 1).ID, 11*time.Minute) })
		e.wantBlocked(o, "preflight_stale")
		e.driveTo(d, "preflight", 2)
		a2 := e.driveTo(d, "activate", 2)
		if a2.State != "SUCCEEDED" || e.status(d) != "ACTIVE" {
			t.Fatalf("activate #2: %+v status %s", a2, e.status(d))
		}
	})

	for name, mutate := range map[string]func(e *adsEnv){
		"account_status 2": func(e *adsEnv) { e.g.SetAccount(e.account, func(a *fakegraph.Account) { a.Status = 2 }) },
		"currency USD":     func(e *adsEnv) { e.g.SetAccount(e.account, func(a *fakegraph.Account) { a.Currency = "USD" }) },
	} {
		t.Run("a not-ready account ("+name+") never gets an activate planned by the sweeper and never a status POST", func(t *testing.T) {
			e := newAdsEnv(t, adsOpts{})
			mutate(e)
			d := e.newDraft(adsDraftIn{})
			e.mustApprove(d)
			e.mustPublish(d)
			e.driveTo(d, "preflight", 1)
			for i := 0; i < 3; i++ {
				e.sweep("advance")
				e.settle()
			}
			if _, planned := e.op(d, "activate", 1); planned || e.g.Count(fakegraph.RouteStatusPost) != 0 {
				t.Errorf("activate planned or status POST sent")
			}
		})
	}

	t.Run("LIVE needs a funded account (fund=1)", func(t *testing.T) {
		live := newAdsEnv(t, adsOpts{live: true, unfunded: true})
		d := live.newDraft(adsDraftIn{})
		live.mustApprove(d)
		live.mustPublish(d)
		live.driveTo(d, "preflight", 1)
		for i := 0; i < 2; i++ {
			live.sweep("advance")
			live.settle()
		}
		if _, planned := live.op(d, "activate", 1); planned || live.g.Count(fakegraph.RouteStatusPost) != 0 {
			t.Error("LIVE with an unfunded account planned an activate")
		}
	})

	t.Run("SANDBOX does not need funding", func(t *testing.T) {
		sb := newAdsEnv(t, adsOpts{unfunded: true})
		d2 := sb.newDraft(adsDraftIn{})
		sb.mustApprove(d2)
		sb.mustPublish(d2)
		if a := sb.driveTo(d2, "activate", 1); a.State != "SUCCEEDED" {
			t.Errorf("SANDBOX unfunded activate: %+v", a)
		}
	})

	t.Run("creates are refused too in SANDBOX with a non-sandbox account: zero HTTP", func(t *testing.T) {
		e := newAdsEnv(t, adsOpts{})
		e.setSettings("SANDBOX", "999"+adsDigits(8), e.opts.allowance, "TWD")
		d := e.newDraft(adsDraftIn{})
		e.mustApprove(d)
		before := e.g.Mark()
		e.mustPublish(d)
		e.settle()
		o := e.mustOp(d, "campaign", 1)
		e.wantBlocked(o, "sandbox_account_required")
		e.sweep("advance")
		e.settle()
		if n := len(e.g.RequestsSince(before)); n != 0 {
			t.Fatalf("%d Graph requests after a refused create (want zero)", n)
		}
		if _, next := e.op(d, "adset", 1); next {
			t.Fatal("a later step was planned after a refused create")
		}
		if got := e.status(d); got != "FAILED" && got != "SUBMITTING" && got != "APPROVED" {
			t.Logf("status after refused create: %s", got)
		}
	})

	t.Run("LIVE environment lets the same draft through", func(t *testing.T) {
		e := newAdsEnv(t, adsOpts{live: true})
		d := e.newDraft(adsDraftIn{})
		e.mustApprove(d)
		e.mustPublish(d)
		if a := e.driveTo(d, "activate", 1); a.State != "SUCCEEDED" {
			t.Fatalf("LIVE activate: %+v", a)
		}
	})

	t.Run("pause is allowed in SANDBOX even when the sandbox rule would refuse a create", func(t *testing.T) {
		e := newAdsEnv(t, adsOpts{})
		d := e.newDraft(adsDraftIn{})
		e.mustApprove(d)
		e.mustPublish(d)
		e.driveTo(d, "activate", 1)
		e.setSettings("SANDBOX", "999"+adsDigits(8), e.opts.allowance, "TWD")
		if r := e.pause(d); r.Status != 200 {
			t.Fatalf("pause: %d %s", r.Status, r.Raw)
		}
		e.settle()
		p := e.mustOp(d, "pause", 1)
		if p.State != "SUCCEEDED" {
			t.Fatalf("pause in SANDBOX with a non-sandbox account: %s/%s", p.State, p.Code)
		}
		if o, _ := e.g.Object(e.g.Objects("campaign")[0].ID); o.Status != "PAUSED" {
			t.Fatalf("remote campaign %s", o.Status)
		}
		if p.Queue != "ads" || p.Priority != 1 {
			t.Errorf("pause op on queue %q priority %d, want the ads priority lane 1 (§6 header, D4)", p.Queue, p.Priority)
		}
	})

	t.Run("publish of an unapproved draft plans nothing and calls nothing", func(t *testing.T) {
		e := newAdsEnv(t, adsOpts{})
		d := e.newDraft(adsDraftIn{})
		mark := e.g.Mark()
		r := e.publish(d)
		if r.Status < 400 || r.Status >= 500 {
			t.Fatalf("publish of an unapproved draft answered %d %s", r.Status, r.Raw)
		}
		e.sweep("advance")
		e.settle()
		if n := len(e.ops(d)); n != 0 || len(e.g.RequestsSince(mark)) != 0 {
			t.Fatalf("ops=%d graph requests=%d after refused publish", n, len(e.g.RequestsSince(mark)))
		}
		t.Logf("EVIDENCE unapproved publish -> %d %s", r.Status, r.code())
	})

	t.Run("approval freezes the draft: PUT after approve is refused, revision and hash unchanged; frozen columns cannot be edited under an approval", func(t *testing.T) {
		e := newAdsEnv(t, adsOpts{})
		d := e.newDraft(adsDraftIn{})
		rev := int(e.draft(d).JSON["revision"].(float64))
		// editing a DRAFT bumps the revision; approving the old revision is refused
		body := e.draftBody(adsDraftIn{Budget: 400000})
		put := e.api("PUT", "/drafts/"+d, e.token, mergeHdr(adsKey(), map[string]string{"If-Match": fmt.Sprint(rev)}), body)
		if put.Status != 200 {
			t.Fatalf("PUT draft: %d %s", put.Status, put.Raw)
		}
		if got := int(put.JSON["revision"].(float64)); got != rev+1 {
			t.Fatalf("revision after edit %d, want %d", got, rev+1)
		}
		stale := e.api("POST", "/drafts/"+d+"/approve", e.token, nil, map[string]any{"revision": rev})
		if stale.Status != 409 { // the code string is asserted by TestMetaAdsMA04ErrorCodes
			t.Fatalf("approve of a stale revision: %d %s", stale.Status, stale.Raw)
		}
		e.mustApprove(d)
		var sha1 []byte
		_ = e.f.owner.QueryRow(e.ctx, `SELECT draft_sha256 FROM ads.draft_approvals WHERE draft_id=$1 ORDER BY revision DESC LIMIT 1`, d).Scan(&sha1)
		rev2 := int(e.draft(d).JSON["revision"].(float64))
		edit := e.api("PUT", "/drafts/"+d, e.token, mergeHdr(adsKey(), map[string]string{"If-Match": fmt.Sprint(rev2)}), e.draftBody(adsDraftIn{Budget: 500000}))
		if edit.Status != 409 {
			t.Fatalf("PUT of an approved draft: %d %s (want 409 draft_approved)", edit.Status, edit.Raw)
		}
		if got := int(e.draft(d).JSON["revision"].(float64)); got != rev2 {
			t.Fatalf("revision moved to %d by a refused edit", got)
		}
		// AD5 at the database: a frozen column cannot change under an approval of the current revision, not even for the owner
		_, err := e.f.owner.Exec(e.ctx, `UPDATE ads.campaign_drafts SET lifetime_budget_minor=lifetime_budget_minor+100 WHERE id=$1`, d)
		if err == nil {
			t.Fatal("frozen column changed while an approval exists")
		}
		var sha2 []byte
		_ = e.f.owner.QueryRow(e.ctx, `SELECT draft_sha256 FROM ads.draft_approvals WHERE draft_id=$1 ORDER BY revision DESC LIMIT 1`, d).Scan(&sha2)
		if string(sha1) != string(sha2) {
			t.Fatal("approval hash changed")
		}
	})
}

func mergeHdr(a, b map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

var _ = fmt.Sprintf
var _ = regexp.MustCompile
var _ = time.Second

// TestMetaAdsMA04ErrorCodes: every frozen HTTP error code of the ads surface (ads-core "Frozen HTTP", contract §7) reaches
// the merchant as that code with its frozen status. Each subtest provokes one code through the real handler. The platform
// envelope (`{"code":...}`) or the brief's `{"error":...}` key are both read by adsResp.code(); a code rewritten to
// "internal" is a failure of THIS gate (the refusal is invisible to the merchant UI, which keys its copy on the code).
func TestMetaAdsMA04ErrorCodes(t *testing.T) {
	e := newAdsEnv(t, adsOpts{noWorker: true, dataset: true})
	want := func(t *testing.T, r adsResp, status int, code string) {
		t.Helper()
		if r.Status != status || r.code() != code {
			t.Fatalf("got %d code %q (%s), want %d %q", r.Status, r.code(), r.Raw, status, code)
		}
	}
	put := func(d string, budget int64) adsResp {
		rev := int(e.draft(d).JSON["revision"].(float64))
		return e.api("PUT", "/drafts/"+d, e.token, mergeHdr(adsKey(), map[string]string{"If-Match": fmt.Sprint(rev)}), e.draftBody(adsDraftIn{Budget: budget}))
	}

	t.Run("revision_changed 409", func(t *testing.T) {
		d := e.newDraft(adsDraftIn{})
		if r := put(d, 400000); r.Status != 200 {
			t.Fatal(r.Raw)
		}
		want(t, e.api("POST", "/drafts/"+d+"/approve", e.token, nil, map[string]any{"revision": 1}), 409, "revision_changed")
	})
	t.Run("draft_approved 409", func(t *testing.T) {
		d := e.newDraft(adsDraftIn{})
		e.mustApprove(d)
		want(t, put(d, 400000), 409, "draft_approved")
	})
	t.Run("over_allowance 409 at approve", func(t *testing.T) {
		d := e.newDraft(adsDraftIn{Budget: adsDefaultAllow + 100})
		want(t, e.approve(d), 409, "over_allowance")
	})
	t.Run("not_whole_unit 422", func(t *testing.T) {
		want(t, e.api("POST", "/drafts", e.token, adsKey(), e.draftBody(adsDraftIn{Budget: 12345})), 422, "not_whole_unit")
	})
	t.Run("currency_mismatch 422", func(t *testing.T) {
		want(t, e.api("POST", "/drafts", e.token, adsKey(), e.draftBody(adsDraftIn{Currency: "USD", Budget: 5000})), 422, "currency_mismatch")
	})
	t.Run("starts_too_soon 422", func(t *testing.T) {
		want(t, e.api("POST", "/drafts", e.token, adsKey(), e.draftBody(adsDraftIn{Start: time.Now().Add(5 * time.Minute)})), 422, "starts_too_soon")
	})
	t.Run("source_not_owned 422", func(t *testing.T) {
		want(t, e.api("POST", "/drafts", e.token, adsKey(), e.draftBody(adsDraftIn{Source: "12345_67890"})), 422, "source_not_owned")
	})
	t.Run("product_not_published 422", func(t *testing.T) {
		b := e.draftBody(adsDraftIn{})
		b["template"], b["source_ref"] = "PRODUCT_TRAFFIC", randomUUID()
		want(t, e.api("POST", "/drafts", e.token, adsKey(), b), 422, "product_not_published")
	})
	t.Run("binding_disabled 409 at approve", func(t *testing.T) {
		d := e.newDraft(adsDraftIn{})
		e.ownerReplica(`UPDATE integration.bindings SET enabled=false,semantic_version=semantic_version+1 WHERE id=$1`, e.adBinding)
		defer e.ownerReplica(`UPDATE integration.bindings SET enabled=true WHERE id=$1`, e.adBinding)
		want(t, e.approve(d), 409, "binding_disabled")
	})
	t.Run("attempt_changed 409", func(t *testing.T) {
		d := e.newDraft(adsDraftIn{})
		e.mustApprove(d)
		want(t, e.api("POST", "/drafts/"+d+"/publish", e.token, adsKey(), map[string]any{"publish_attempt": 3}), 409, "attempt_changed")
	})
	t.Run("budget_below_minimum 422", func(t *testing.T) {
		e2 := newAdsEnv(t, adsOpts{noWorker: true})
		e2.setSettings("SANDBOX", e2.account, 100000, "USD")
		want(t, e2.api("POST", "/drafts", e2.token, adsKey(), e2.draftBody(adsDraftIn{Currency: "USD", Budget: 50})), 422, "budget_below_minimum")
	})

	// connect flow codes
	connectState := func(en *adsEnv, token string) (id, state string) {
		c := en.api("POST", "/meta/connect", token, adsKey(), nil)
		if c.Status != 201 {
			t.Fatalf("connect: %d %s", c.Status, c.Raw)
		}
		u, _ := url.Parse(c.str("dialog_url"))
		return c.str("state_id"), u.Query().Get("state")
	}
	cb := func(en *adsEnv, token, code, state string) adsResp {
		return en.api("GET", "/meta/callback?code="+url.QueryEscape(code)+"&state="+url.QueryEscape(state), token, nil, nil)
	}
	t.Run("state_mismatch 409: another principal presents the state", func(t *testing.T) {
		other := e.principal("other-manager", "ads:manage", "integration:manage")
		_, state := connectState(e, e.token)
		code := "SYNTH-CODE-" + t04Tag()
		e.g.AddCode(code, e.botToken)
		want(t, cb(e, other, code, state), 409, "state_mismatch")
		if e.g.Count(fakegraph.RouteOAuthToken) != 1 { // only the connect() of newAdsEnv exchanged a code: the mismatch never reached Meta
			t.Errorf("code exchange attempted for a mismatching principal")
		}
	})
	t.Run("state_mismatch 409: unknown or malformed state", func(t *testing.T) {
		want(t, cb(e, e.token, "SYNTH-CODE-x", strings.Repeat("A", 43)), 409, "state_mismatch")
		want(t, cb(e, e.token, "SYNTH-CODE-x", "short"), 409, "state_mismatch")
	})
	t.Run("state is single use", func(t *testing.T) {
		_, state := connectState(e, e.token)
		code := "SYNTH-CODE-" + t04Tag()
		e.g.AddCode(code, e.botToken)
		if r := cb(e, e.token, code, state); r.Status != 200 {
			t.Fatalf("first callback: %d %s", r.Status, r.Raw)
		}
		code2 := "SYNTH-CODE-" + t04Tag()
		e.g.AddCode(code2, e.botToken)
		if r := cb(e, e.token, code2, state); r.Status < 400 {
			t.Fatalf("replayed state accepted: %d %s", r.Status, r.Raw)
		}
	})
	t.Run("state_expired 410", func(t *testing.T) {
		id, state := connectState(e, e.token)
		e.ownerReplica(`UPDATE ads.oauth_states SET created_at=created_at-interval '2 hours',expires_at=expires_at-interval '2 hours' WHERE id=$1`, id)
		code := "SYNTH-CODE-" + t04Tag()
		e.g.AddCode(code, e.botToken)
		want(t, cb(e, e.token, code, state), 410, "state_expired")
	})
	t.Run("meta_connect_failed 502: Meta refuses the code", func(t *testing.T) {
		_, state := connectState(e, e.token)
		want(t, cb(e, e.token, "SYNTH-CODE-never-issued", state), 502, "meta_connect_failed")
	})
	t.Run("not_in_pick_list 422", func(t *testing.T) {
		id, state := connectState(e, e.token)
		code := "SYNTH-CODE-" + t04Tag()
		e.g.AddCode(code, e.botToken)
		if r := cb(e, e.token, code, state); r.Status != 200 {
			t.Fatal(r.Raw)
		}
		want(t, e.api("POST", "/meta/bindings", e.token, adsKey(), map[string]any{"state_id": id, "ad_account_id": "1" + adsDigits(11)}), 422, "not_in_pick_list")
	})
	t.Run("client_business_changed 409: the same ad account re-connected under another client business", func(t *testing.T) {
		tok2 := "SENTINEL-EAAG-BISU2-" + t04Tag()
		e.g.Grant(tok2, "52"+adsDigits(11), []string{"ads_management", "ads_read"}, []string{e.account}, nil)
		id, state := connectState(e, e.token)
		code := "SYNTH-CODE-" + t04Tag()
		e.g.AddCode(code, tok2)
		if r := cb(e, e.token, code, state); r.Status != 200 {
			t.Fatal(r.Raw)
		}
		want(t, e.api("POST", "/meta/bindings", e.token, adsKey(), map[string]any{"state_id": id, "ad_account_id": e.account}), 409, "client_business_changed")
	})
	t.Run("billing_restricted 402 is the platform code", func(t *testing.T) {
		e3 := newAdsEnv(t, adsOpts{noWorker: true})
		ingress := mustStripeIngress(t, e3.f)
		cbxRestrict(t, e3.f, ingress, e3.tenant, e3.store)
		d := e3.newDraft(adsDraftIn{})
		want(t, e3.approve(d), 402, "billing_restricted")
	})
	t.Run("forbidden / not_found / unauthorized", func(t *testing.T) {
		viewer := e.principal("viewer", "ads:read")
		d := e.newDraft(adsDraftIn{})
		want(t, e.api("POST", "/drafts/"+d+"/approve", viewer, nil, map[string]any{"revision": 1}), 403, "forbidden")
		want(t, e.api("GET", "/drafts/"+randomUUID(), e.token, nil, nil), 404, "not_found")
		want(t, e.api("GET", "/drafts", "", nil, nil), 401, "unauthorized")
	})
}

func mustStripeIngress(t *testing.T, f *testFixture) *pgxpool.Pool {
	t.Helper()
	p, err := platform.OpenStripeIngressPool(context.Background(), sstLogin(t, f, "commerce_stripe_ingress"))
	if err != nil {
		t.Fatalf("stripe ingress pool: %v", err)
	}
	t.Cleanup(p.Close)
	return p
}

// ---------------------------------------------------------------------------------------------------------------------
// MA05 - UNKNOWN, reconcile by tag, pause retries, pause-vs-activate races (contract §3 Reconcile, §5.3, §6.3, AD3, AD6, F9)
// ---------------------------------------------------------------------------------------------------------------------

func (e *adsEnv) opStates(op string) []string {
	e.t.Helper()
	rows, err := e.f.owner.Query(e.ctx, `SELECT state FROM integration.operation_events WHERE operation_id=$1 ORDER BY id`, op)
	if err != nil {
		e.t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		_ = rows.Scan(&s)
		out = append(out, s)
	}
	return out
}

func indexOf(list []string, want string) int {
	for i, v := range list {
		if v == want {
			return i
		}
	}
	return -1
}

func (e *adsEnv) statusPosts() []string {
	var out []string
	for _, r := range e.g.Requests() {
		if r.Route == fakegraph.RouteStatusPost {
			out = append(out, string(r.Body))
		}
	}
	return out
}

func TestMetaAdsMA05Unknown(t *testing.T) {
	kinds := []struct{ kind, route string }{
		{"campaign", fakegraph.RouteCreateCampaign}, {"adset", fakegraph.RouteCreateAdset},
		{"creative", fakegraph.RouteCreateCreative}, {"ad", fakegraph.RouteCreateAd},
	}
	for _, k := range kinds {
		t.Run("timeout after the remote create at "+k.kind+": reconcile by tag pins the object, no second POST", func(t *testing.T) {
			e := newAdsEnv(t, adsOpts{})
			e.g.Inject(fakegraph.Fault{Route: k.route, Kind: fakegraph.FaultTimeout, Effect: true})
			d := e.newDraft(adsDraftIn{})
			e.mustApprove(d)
			e.mustPublish(d)
			op := e.driveTo(d, k.kind, 1)
			if n := e.g.Count(k.route); n != 1 {
				t.Fatalf("%d POSTs to %s, want exactly 1 (F9: no idempotency key, a second POST could duplicate)", n, k.route)
			}
			objs := e.g.Objects(k.kind)
			if len(objs) != 1 || op.Ref != objs[0].ID || objs[0].Name != "lc-"+op.ID {
				t.Fatalf("objects %+v, op ref %s", objs, op.Ref)
			}
			if op.Generation < 2 {
				t.Errorf("op generation %d: expected a dispatch claim and a reconcile claim", op.Generation)
			}
			st := e.opStates(op.ID)
			if i, j := indexOf(st, "UNKNOWN"), indexOf(st, "SUCCEEDED"); i < 0 || j < i {
				t.Errorf("event states %v: want UNKNOWN before SUCCEEDED", st)
			}
			// A-10: reconcile ran with a token loaded through the reconcile claim, and Check saw mode "reconcile" for it
			var sawLoad, sawCheck bool
			for _, l := range e.probe.Loads() {
				if l.Op == op.ID && l.OpState == "UNKNOWN" && l.Lease == "reconcile" && !l.InCheck {
					sawLoad = true
				}
			}
			for _, c := range e.probe.Checks() {
				if c.Op == op.ID && c.Mode == "reconcile" {
					sawCheck = true
				}
			}
			if !sawLoad || !sawCheck {
				t.Errorf("reconcile-mode loader call seen=%v, reconcile-mode Check seen=%v", sawLoad, sawCheck)
			}
			// the chain continues from the pinned id and every later create is a single POST
			e.driveTo(d, "activate", 1)
			for _, kk := range kinds {
				if n := e.g.Count(kk.route); n != 1 {
					t.Errorf("%s POSTs: %d, want 1", kk.route, n)
				}
			}
			if r := e.remote(d); r["campaign_id"] == "" || r["ad_id"] == "" {
				t.Errorf("remote ids not pinned: %v", r)
			}
			if v := e.g.Violations(); len(v) != 0 {
				t.Errorf("isolation violations %v", v)
			}
		})
	}

	for name, fl := range map[string]fakegraph.Fault{
		"503 after create":             {Route: fakegraph.RouteCreateCampaign, Kind: fakegraph.Fault5xx, Effect: true},
		"unparseable 200 after create": {Route: fakegraph.RouteCreateCampaign, Kind: fakegraph.FaultGarbled, Effect: true},
		"200 without id after create":  {Route: fakegraph.RouteCreateCampaign, Kind: fakegraph.FaultShapeless, Effect: true},
	} {
		t.Run(name+": UNKNOWN then reconciled by tag", func(t *testing.T) {
			e := newAdsEnv(t, adsOpts{})
			e.g.Inject(fl)
			d := e.newDraft(adsDraftIn{})
			e.mustApprove(d)
			e.mustPublish(d)
			op := e.driveTo(d, "campaign", 1)
			if e.g.Count(fakegraph.RouteCreateCampaign) != 1 || len(e.g.Objects("campaign")) != 1 || op.Ref != e.g.Objects("campaign")[0].ID {
				t.Fatalf("posts=%d objects=%d ref=%s", e.g.Count(fakegraph.RouteCreateCampaign), len(e.g.Objects("campaign")), op.Ref)
			}
		})
	}

	t.Run("timeout with no remote effect: reconcile finds nothing, the op stays UNKNOWN, nothing is recreated", func(t *testing.T) {
		e := newAdsEnv(t, adsOpts{})
		e.g.Inject(fakegraph.Fault{Route: fakegraph.RouteCreateCampaign, Kind: fakegraph.FaultTimeout})
		d := e.newDraft(adsDraftIn{})
		e.mustApprove(d)
		e.mustPublish(d)
		for i := 0; i < 4; i++ {
			e.sweep("advance")
			e.settle()
		}
		op := e.mustOp(d, "campaign", 1)
		if op.State != "UNKNOWN" {
			t.Fatalf("campaign op %s/%s, want UNKNOWN", op.State, op.Code)
		}
		if n := e.g.Count(fakegraph.RouteCreateCampaign); n != 1 || len(e.g.Objects("")) != 0 {
			t.Fatalf("POSTs=%d objects=%d: an UNKNOWN create must never be re-POSTed (F9)", n, len(e.g.Objects("")))
		}
		if _, next := e.op(d, "adset", 1); next {
			t.Fatal("chain advanced past an UNKNOWN create")
		}
		if got := e.status(d); got != "UNKNOWN" {
			t.Errorf("draft status %s, want UNKNOWN (§5.1: any op UNKNOWN)", got)
		}
		// the merchant can still pause the (unpinned) draft? no campaign id exists, so the API must refuse or no-op without calling Meta
		mark := e.g.Mark()
		_ = e.pause(d)
		e.settle()
		for _, r := range e.g.RequestsSince(mark) {
			if r.Method == http.MethodPost {
				t.Errorf("pause without a pinned campaign made a POST: %s", r.Path)
			}
		}
	})

	t.Run("more than one remote object with the tag: UNKNOWN duplicate_remote_objects, never a guess", func(t *testing.T) {
		e := newAdsEnv(t, adsOpts{})
		e.g.Inject(fakegraph.Fault{Route: fakegraph.RouteCreateCampaign, Kind: fakegraph.FaultTimeout})
		entered, release := e.gateCheck("meta.ads.create_campaign")
		defer release()
		d := e.newDraft(adsDraftIn{})
		e.mustApprove(d)
		e.mustPublish(d)
		<-entered
		op := e.mustOp(d, "campaign", 1)
		e.g.Seed("campaign", "lc-"+op.ID, e.account, "", "PAUSED")
		e.g.Seed("campaign", "lc-"+op.ID, e.account, "", "PAUSED")
		release()
		e.settle()
		op = e.mustOp(d, "campaign", 1)
		if op.State != "UNKNOWN" || !strings.Contains(op.Code, "duplicate_remote_objects") && op.Code != "reconcile_budget_exhausted" {
			t.Fatalf("op %s/%s", op.State, op.Code)
		}
		if e.g.Count(fakegraph.RouteCreateCampaign) != 1 {
			t.Fatalf("second create POST after duplicates: %d", e.g.Count(fakegraph.RouteCreateCampaign))
		}
		if op.Ref != "" {
			t.Fatalf("a provider reference %q was pinned from an ambiguous match", op.Ref)
		}
		// the reason code the reconcile recorded (before the budget ran out) is in the event history
		var seen bool
		rows, _ := e.f.owner.Query(e.ctx, `SELECT reason_code FROM integration.operation_events WHERE operation_id=$1`, op.ID)
		for rows.Next() {
			var c string
			_ = rows.Scan(&c)
			seen = seen || c == "duplicate_remote_objects"
		}
		rows.Close()
		if !seen {
			t.Errorf("no operation event carried duplicate_remote_objects")
		}
	})

	t.Run("reconcile pages through the parent's children (<=10 pages of 100) and stops there", func(t *testing.T) {
		e := newAdsEnv(t, adsOpts{maxGen: 3})
		for i := 0; i < 250; i++ {
			e.g.Seed("campaign", fmt.Sprintf("decoy-%d", i), e.account, "", "PAUSED")
		}
		e.g.Inject(fakegraph.Fault{Route: fakegraph.RouteCreateCampaign, Kind: fakegraph.FaultTimeout, Effect: true})
		d := e.newDraft(adsDraftIn{})
		e.mustApprove(d)
		e.mustPublish(d)
		op := e.driveTo(d, "campaign", 1)
		if e.g.Count(fakegraph.RouteCreateCampaign) != 1 || op.Ref == "" {
			t.Fatalf("posts=%d ref=%q", e.g.Count(fakegraph.RouteCreateCampaign), op.Ref)
		}
		if n := e.g.Count(fakegraph.RouteListCampaigns); n < 3 {
			t.Errorf("only %d list requests for a match on the third page", n)
		}
	})
	t.Run("a match beyond page 10 is not found: UNKNOWN, never a re-POST", func(t *testing.T) {
		e := newAdsEnv(t, adsOpts{maxGen: 3})
		for i := 0; i < 1005; i++ {
			e.g.Seed("campaign", fmt.Sprintf("decoy-%d", i), e.account, "", "PAUSED")
		}
		e.g.Inject(fakegraph.Fault{Route: fakegraph.RouteCreateCampaign, Kind: fakegraph.FaultTimeout, Effect: true})
		d := e.newDraft(adsDraftIn{})
		e.mustApprove(d)
		e.mustPublish(d)
		for i := 0; i < 2; i++ {
			e.sweep("advance")
			e.settle()
		}
		op := e.mustOp(d, "campaign", 1)
		if op.State != "UNKNOWN" || e.g.Count(fakegraph.RouteCreateCampaign) != 1 {
			t.Fatalf("op %s/%s posts=%d", op.State, op.Code, e.g.Count(fakegraph.RouteCreateCampaign))
		}
		perGeneration := e.g.Count(fakegraph.RouteListCampaigns)
		if perGeneration == 0 || perGeneration > 10*int(op.Generation) {
			t.Errorf("%d list requests over %d generations: more than 10 pages per reconcile", perGeneration, op.Generation)
		}
	})

	t.Run("rate-limited pause: UNKNOWN (never terminal), reconcile does not prove it, seq 2 after 15 minutes with an ops alert", func(t *testing.T) {
		logs := captureSlog(t)
		e := newAdsEnv(t, adsOpts{})
		d := e.newDraft(adsDraftIn{})
		e.mustApprove(d)
		e.mustPublish(d)
		e.driveTo(d, "activate", 1)
		camp := e.g.Objects("campaign")[0].ID
		e.g.Inject(fakegraph.Fault{Route: fakegraph.RouteStatusPost, Kind: fakegraph.FaultGraphError, Code: 4, HTTP: 429, Times: 40})
		if r := e.pause(d); r.Status != 200 {
			t.Fatalf("pause: %d %s", r.Status, r.Raw)
		}
		e.settle()
		p1 := e.mustOp(d, "pause", 1)
		if p1.State != "UNKNOWN" {
			t.Fatalf("rate-limited pause is %s/%s, want UNKNOWN (a rejected status POST proves nothing)", p1.State, p1.Code)
		}
		if o, _ := e.g.Object(camp); o.Status != "ACTIVE" {
			t.Fatalf("remote status %s: the pause was rate limited", o.Status)
		}
		if got := e.status(d); got != "UNKNOWN" {
			t.Errorf("draft status %s, want UNKNOWN (the pause op is UNKNOWN)", got)
		}
		e.sweep("advance")
		e.settle()
		if _, planned := e.op(d, "pause", 2); planned {
			t.Fatal("pause seq 2 planned before 15 minutes")
		}
		for _, o := range e.ops(d) { // the whole draft's history moves 16 minutes back, so relative order (activate before the pause claim) is kept
			e.age(o.ID, 16*time.Minute)
		}
		e.g.ClearFaults()
		e.sweep("advance")
		p2 := e.mustOp(d, "pause", 2)
		if p2.Queue != "ads" || p2.Priority != 1 {
			t.Errorf("pause seq 2 on queue %q priority %d, want ads/1", p2.Queue, p2.Priority)
		}
		e.settle()
		if p2 = e.mustOp(d, "pause", 2); p2.State != "SUCCEEDED" {
			t.Fatalf("pause seq 2: %s/%s", p2.State, p2.Code)
		}
		if o, _ := e.g.Object(camp); o.Status != "PAUSED" {
			t.Fatalf("remote status %s after pause seq 2", o.Status)
		}
		for _, o := range e.ops(d) {
			if o.Kind == "pause" && o.State == "FAILED_FINAL" {
				t.Errorf("pause seq %d ended FAILED_FINAL", o.Seq)
			}
		}
		if !strings.Contains(logs.String(), "ads pause retry planned") {
			t.Errorf("no ops alert logged for the pause retry:\n%s", logs.String())
		}
		if _, act2 := e.op(d, "activate", 2); act2 {
			t.Fatal("an activate was planned after a pause (X7)")
		}
	})

	t.Run("pause SUCCEEDED while activate is still READY: activate BLOCKED_POLICY pause_requested, no activate POST", func(t *testing.T) {
		e := newAdsEnv(t, adsOpts{})
		d := e.readyToActivate(adsDraftIn{})
		e.pauseDispatch()
		e.sweep("advance") // plans activate #1 (READY, undispatched)
		act := e.mustOp(d, "activate", 1)
		if act.State != "READY" {
			t.Fatalf("activate %s", act.State)
		}
		e.hold(act.ID)
		e.resumeDispatch()
		if r := e.pause(d); r.Status != 200 {
			t.Fatalf("pause: %d %s", r.Status, r.Raw)
		}
		e.awaitOp(e.mustOp(d, "pause", 1).ID, 15*time.Second, "SUCCEEDED")
		e.release(act.ID)
		e.settle()
		act = e.mustOp(d, "activate", 1)
		e.wantBlocked(act, "pause_requested")
		posts := e.statusPosts()
		if len(posts) != 1 || !strings.Contains(posts[0], "PAUSED") {
			t.Fatalf("status POSTs %v: want exactly the pause", posts)
		}
		if o, _ := e.g.Object(e.g.Objects("campaign")[0].ID); o.Status != "PAUSED" {
			t.Fatalf("campaign %s", o.Status)
		}
		for i := 0; i < 2; i++ {
			e.sweep("advance")
			e.settle()
		}
		if _, again := e.op(d, "activate", 2); again {
			t.Fatal("the sweeper planned another activate for a draft with a pause op (X7)")
		}
	})

	t.Run("pause SUCCEEDED while the activate call is in flight: draft keeps counting, then the sweeper plans pause seq 2 and the campaign ends PAUSED", func(t *testing.T) {
		e := newAdsEnv(t, adsOpts{callTimeout: 10 * time.Second})
		d := e.readyToActivate(adsDraftIn{Budget: 300000})
		hold, arrived := make(chan struct{}), make(chan struct{}, 1)
		e.g.Inject(fakegraph.Fault{Route: fakegraph.RouteStatusPost, Kind: fakegraph.FaultHold, Hold: hold, Arrived: arrived})
		e.sweep("advance") // plans activate #1; the dispatcher claims it, Check passes, the POST is now held at the fake
		select {
		case <-arrived:
		case <-time.After(20 * time.Second):
			t.Fatalf("activate POST never arrived:%s", e.dump())
		}
		act := e.mustOp(d, "activate", 1)
		if act.State != "DISPATCHING" {
			t.Fatalf("activate %s while its call is in flight", act.State)
		}
		if r := e.pause(d); r.Status != 200 {
			t.Fatalf("pause: %d %s", r.Status, r.Raw)
		}
		e.awaitOp(e.mustOp(d, "pause", 1).ID, 15*time.Second, "SUCCEEDED")
		// AD6/§5.3: the pause was claimed while the activate was DISPATCHING, so the draft still counts toward the allowance
		other := e.newDraft(adsDraftIn{Budget: adsDefaultAllow - 300000 + 100})
		if r := e.approve(other); r.Status != 409 {
			t.Fatalf("allowance released although the activate had not left DISPATCHING: approve %d %s", r.Status, r.Raw)
		}
		close(hold) // the activate call now takes effect at Meta
		e.awaitOp(act.ID, 15*time.Second, "SUCCEEDED")
		if o, _ := e.g.Object(e.g.Objects("campaign")[0].ID); o.Status != "ACTIVE" {
			t.Fatalf("expected the late activate to have taken effect, remote %s", o.Status)
		}
		// the pause was claimed BEFORE the activate finished: it does not release; the sweeper plans pause seq 2
		if r := e.approve(other); r.Status != 409 {
			t.Fatalf("a pause claimed before the last activate finished released the allowance: %d %s", r.Status, r.Raw)
		}
		e.sweep("advance")
		p2 := e.mustOp(d, "pause", 2)
		if p2.Priority != 1 || p2.Queue != "ads" {
			t.Errorf("pause seq 2 lane %s/%d", p2.Queue, p2.Priority)
		}
		e.settle()
		if p2 = e.mustOp(d, "pause", 2); p2.State != "SUCCEEDED" {
			t.Fatalf("pause seq 2 %s/%s", p2.State, p2.Code)
		}
		if o, _ := e.g.Object(e.g.Objects("campaign")[0].ID); o.Status != "PAUSED" {
			t.Fatalf("final remote status %s, want PAUSED", o.Status)
		}
		// now the confirmed pause was claimed after the activate left DISPATCHING: the draft no longer counts
		if r := e.approve(other); r.Status != 200 {
			t.Fatalf("allowance not released after a confirmed pause claimed after the activate: %d %s", r.Status, r.Raw)
		}
		if _, act2 := e.op(d, "activate", 2); act2 {
			t.Fatal("re-activation planned (X7)")
		}
	})
}

// captureSlog swaps the default slog handler for a buffer for the test (the sweepers log through slog's default).
func captureSlog(t *testing.T) *logBuf {
	t.Helper()
	b := &logBuf{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(b, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return b
}

type logBuf struct {
	mu sync.Mutex
	b  strings.Builder
}

func (l *logBuf) Write(p []byte) (int, error) { l.mu.Lock(); defer l.mu.Unlock(); return l.b.Write(p) }
func (l *logBuf) String() string              { l.mu.Lock(); defer l.mu.Unlock(); return l.b.String() }
