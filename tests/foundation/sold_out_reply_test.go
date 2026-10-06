// Purpose: REAL_PG + fake Graph gates SO01-SO08 of W3-04B (sold-out automatic private reply) plus the settings definers, the template rules and the ACL pins of migration 0151.
// Depends on: mciEnv / lbEnv harnesses (meta_claims_intake_*_test.go, live_console_send_test.go), claims.SetSoldOutReply/GetSoldOutReply, metareply routes against the fake Graph.
// Used by: scripts/dev/test-focused.sh 'SoldOutReply', GitHub shard gates.
// Invariants: one private reply per comment (mpr: key); a sold-out claim is recorded but never gets a link; Graph UNKNOWN is never re-sent.
// Status: MOCK (REAL_PG + River + fake Graph); Meta LIVE NOT_RUN.
package foundation_test

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/claims"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

// soldOut makes the harness SKU tracked with the given available units (owner-only synthetic stock; the ledger is not under test here).
func (e *mciEnv) soldOut(t *testing.T, available int) {
	t.Helper()
	f := e.h.f
	mustExec(t, f.owner, `UPDATE catalog.skus SET inventory_tracked=true, max_per_order=NULL WHERE tenant_id=$1 AND store_id=$2 AND id=$3`, f.tenantA, f.storeA1, e.sku)
	mustExec(t, f.owner, `UPDATE inventory.balances SET on_hand=reserved+allocated+unavailable+$4 WHERE tenant_id=$1 AND store_id=$2 AND sku_id=$3`, f.tenantA, f.storeA1, e.sku, available)
}

type soOp struct {
	ID, Type, State, Text string
	Links                 int64
}

// soOpOf reads the private-reply operation of a comment and whether its bundle has a claim link ("" ID when none was planned).
func (e *mciEnv) soOpOf(t *testing.T, ref string) soOp {
	t.Helper()
	var o soOp
	err := e.h.f.owner.QueryRow(context.Background(), `SELECT o.id::text, o.request->>'message_type', o.state, coalesce(o.request->>'text',''),
		(SELECT count(*) FROM claims.links k WHERE k.bundle_id=(o.request->>'bundle_id')::uuid)
		FROM integration.operations o WHERE o.action='meta.private_reply' AND o.request->>'comment_ref'=$1`, ref).Scan(&o.ID, &o.Type, &o.State, &o.Text, &o.Links)
	if errors.Is(err, pgx.ErrNoRows) {
		return soOp{}
	}
	if err != nil {
		t.Fatalf("operation of %s: %v", ref, err)
	}
	return o
}

func (e *mciEnv) productName(t *testing.T) string {
	t.Helper()
	var name string
	if err := e.h.f.owner.QueryRow(context.Background(), `SELECT p.name FROM catalog.skus k JOIN catalog.products p ON p.id=k.product_id WHERE k.id=$1`, e.sku).Scan(&name); err != nil {
		t.Fatal(err)
	}
	return name
}

func (e *mciEnv) soAudit(t *testing.T, action string) int64 {
	return miCount(t, e.h.f.owner, `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action=$3`, e.h.f.tenantA, e.h.f.storeA1, action)
}

func (e *mciEnv) setSoldOut(t *testing.T, enabled bool, tpl string, tplVersion, expected int64) (claims.SoldOutReply, error) {
	t.Helper()
	var out claims.SoldOutReply
	err := platform.WithScope(context.Background(), e.h.f.runtime, e.h.token, e.h.f.storeA1, "live:manage", func(tx pgx.Tx, _ platform.Scope) error {
		var err error
		out, err = claims.SetSoldOutReply(context.Background(), tx, enabled, tpl, tplVersion, expected)
		return err
	})
	return out, err
}

func (e *mciEnv) getSoldOut(t *testing.T) claims.SoldOutReply {
	t.Helper()
	var out claims.SoldOutReply
	err := platform.WithScope(context.Background(), e.h.f.runtime, e.h.token, e.h.f.storeA1, "live:read", func(tx pgx.Tx, _ platform.Scope) error {
		var err error
		out, err = claims.GetSoldOutReply(context.Background(), tx)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func soCleanSettings(t *testing.T, e *mciEnv) {
	t.Helper()
	f := e.h.f
	mustExec(t, f.owner, `DELETE FROM claims.sold_out_settings WHERE tenant_id=$1 AND store_id=$2`, f.tenantA, f.storeA1)
	t.Cleanup(func() {
		_, _ = f.owner.Exec(context.Background(), `DELETE FROM claims.sold_out_settings WHERE tenant_id=$1 AND store_id=$2`, f.tenantA, f.storeA1)
		_, _ = f.owner.Exec(context.Background(), `DELETE FROM msgtemplates.templates WHERE tenant_id=$1 AND store_id=$2 AND template_id LIKE 'so-%'`, f.tenantA, f.storeA1)
	})
}

// SO01 + SO02 + partial (SO-OPEN-1): a tracked sold-out offer gets one sold_out_reply with the rendered text and NO claim link; an
// untracked SKU and a stocked SKU still get the claim link; 3 claimed with 1 left is sold out.
func TestSoldOutReplyPlanShape(t *testing.T) {
	e := mciSetup(t, mciOpts{private: true})
	soCleanSettings(t, e)
	f := e.h.f
	name := e.productName(t)

	e.soldOut(t, 0)
	auditBefore := e.soAudit(t, "claim_reply_sold_out")
	r := e.planReply(t, false, "", "A1")
	if r.ev.outcome != "ACCEPTED" || r.op == "" {
		t.Fatalf("SO01 claim must be recorded and answered: %+v", r.intake)
	}
	o := e.soOpOf(t, r.s.comment)
	wantText := "抱歉，" + name + " 已售完，補貨時會在直播中通知，請留意直播。"
	if o.Type != "sold_out_reply" || o.State != "READY" || o.Links != 0 || o.Text != wantText {
		t.Fatalf("SO01 operation = %+v, want sold_out_reply READY with no link and text %q", o, wantText)
	}
	if strings.Contains(o.Text, "https://") || strings.Contains(o.Text, "claim#t=") {
		t.Fatal("sold-out text carries a link")
	}
	var key, tpl string
	var tplVersion int
	var size int
	if err := f.owner.QueryRow(context.Background(), `SELECT semantic_key, request->>'template', (request->>'template_version')::int, octet_length(request::text) FROM integration.operations WHERE id=$1`, o.ID).Scan(&key, &tpl, &tplVersion, &size); err != nil {
		t.Fatal(err)
	}
	if key != lbMprKey("page", e.pageAsset, r.s.comment) || tpl != "sold-out-reply/v1" || tplVersion != 1 || size > 2048 {
		t.Fatalf("SO01 key/template/size: %q %q %d %d", key, tpl, tplVersion, size)
	}
	if got := e.soAudit(t, "claim_reply_sold_out"); got != auditBefore+1 {
		t.Fatalf("claim_reply_sold_out audit +%d want +1", got-auditBefore)
	}
	if n := miCount(t, f.owner, `SELECT count(*) FROM claims.links WHERE bundle_id=$1`, r.bundleID); n != 0 {
		t.Fatalf("SO01 sold-out claim got %d claim links", n)
	}
	// The claim itself is a normal claim line (intent is never lost).
	if n := miCount(t, f.owner, `SELECT count(*) FROM claims.lines WHERE bundle_id=$1 AND quantity=1`, r.bundleID); n != 1 {
		t.Fatalf("SO01 claim line missing: %d", n)
	}

	t.Run("SO02 untracked SKU sells the link", func(t *testing.T) {
		mustExec(t, f.owner, `UPDATE catalog.skus SET inventory_tracked=false, max_per_order=5 WHERE tenant_id=$1 AND store_id=$2 AND id=$3`, f.tenantA, f.storeA1, e.sku)
		mustExec(t, f.owner, `UPDATE inventory.balances SET on_hand=reserved+allocated+unavailable WHERE sku_id=$1`, e.sku)
		r := e.planReply(t, false, "", "A1")
		o := e.soOpOf(t, r.s.comment)
		if o.Type != "first_private_reply" || o.Links != 1 || o.Text != "" {
			t.Fatalf("SO02 operation = %+v, want the claim-link reply", o)
		}
	})
	t.Run("stocked SKU sells the link", func(t *testing.T) {
		e.soldOut(t, 5)
		r := e.planReply(t, false, "", "A1+5")
		if o := e.soOpOf(t, r.s.comment); o.Type != "first_private_reply" || o.Links != 1 {
			t.Fatalf("5 claimed with 5 available = %+v", o)
		}
	})
	t.Run("SO-OPEN-1 partial stock counts as sold out", func(t *testing.T) {
		e.soldOut(t, 1)
		r := e.planReply(t, false, "", "A1+3")
		if o := e.soOpOf(t, r.s.comment); o.Type != "sold_out_reply" || o.Links != 0 {
			t.Fatalf("3 claimed with 1 available = %+v", o)
		}
	})
	t.Run("counters reserved/allocated/unavailable are not sellable", func(t *testing.T) {
		mustExec(t, f.owner, `UPDATE inventory.balances SET on_hand=4, reserved=1, allocated=1, unavailable=1 WHERE sku_id=$1`, e.sku)
		r := e.planReply(t, false, "", "A1+2")
		if o := e.soOpOf(t, r.s.comment); o.Type != "sold_out_reply" {
			t.Fatalf("available 1 for 2 claimed = %+v", o)
		}
	})
}

// SO03: a paused offer never reaches the reply planner (ingest rejects OFFER_INACTIVE and creates no bundle, so there is neither a link nor a
// reply); the shared predicate still treats an inactive offer as sold out, which is what keeps a late deactivation from selling a link.
func TestSoldOutReplyPausedOffer(t *testing.T) {
	e := mciSetup(t, mciOpts{private: true})
	soCleanSettings(t, e)
	f := e.h.f
	e.soldOut(t, 9)
	// (a) claim first, pause afterwards: the predicate reads the offer as sold out although stock exists.
	r := e.planReply(t, false, "", "A1")
	if o := e.soOpOf(t, r.s.comment); o.Type != "first_private_reply" {
		t.Fatalf("stocked claim = %+v", o)
	}
	probe := func() (soldOut bool) {
		tx, err := f.owner.Begin(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(context.Background()) }()
		if _, err := tx.Exec(context.Background(), `SELECT set_config('app.tenant_id',$1,true), set_config('app.store_id',$2,true)`, f.tenantA, f.storeA1); err != nil {
			t.Fatal(err)
		}
		if err := tx.QueryRow(context.Background(), `SELECT sold_out FROM integration.claim_sold_out_facts($1,$2,$3)`, f.tenantA, f.storeA1, r.intake.InboxEvent).Scan(&soldOut); err != nil {
			t.Fatalf("facts: %v", err)
		}
		return soldOut
	}
	if probe() {
		t.Fatal("stocked active offer read as sold out")
	}
	mustExec(t, f.owner, `UPDATE live.offers SET active=false, version=version+1 WHERE id=$1`, e.offer.ID)
	if !probe() {
		t.Fatal("SO03 inactive offer must read as sold out")
	}
	// (b) a comment on the paused offer: rejected, no bundle, no operation.
	r2 := e.planReply(t, false, "", "A1")
	if r2.ev.outcome != "REJECTED" || r2.op != "" {
		t.Fatalf("SO03 comment on a paused offer: outcome=%q op=%q", r2.ev.outcome, r2.op)
	}
}

// SO04 + SO05 + SO06 on the real manual planner: a sold-out reply consumes the comment's reply budget; with the switch OFF the budget stays for
// one manual reply; a manual reply sent first turns the sold-out claim into the audited reply_used skip.
func TestSoldOutReplyBudget(t *testing.T) {
	e := lbSetup(t)
	e.onlySource(t, "facebook")
	soCleanSettings(t, e.mciEnv)
	text := func(s string) string { return s + "-" + t04Tag() }
	e.soldOut(t, 0)

	t.Run("SO04 manual after the sold-out reply is refused", func(t *testing.T) {
		r := e.planReply(t, false, "", "A1")
		if o := e.soOpOf(t, r.s.comment); o.Type != "sold_out_reply" {
			t.Fatalf("setup: %+v", o)
		}
		if _, err := e.manual(r.s.comment, text("after-sold-out"), time.Now().Add(-10*time.Minute), false); planCode(err) != "used" {
			t.Fatalf("SO04 manual after a sold-out reply: %v", err)
		}
	})

	t.Run("SO05 switch off keeps the budget for the merchant", func(t *testing.T) {
		if out, err := e.setSoldOut(t, false, "sold-out-reply/v1", 1, 0); err != nil || out.Enabled || out.Version != 1 {
			t.Fatalf("switch off: %+v %v", out, err)
		}
		skipBefore := e.soAudit(t, "claim_reply_skipped:sold_out_off")
		r := e.planReply(t, false, "", "A1")
		if r.ev.outcome != "ACCEPTED" || r.op != "" {
			t.Fatalf("SO05 want the claim recorded and no operation: outcome=%q op=%q", r.ev.outcome, r.op)
		}
		if got := e.soAudit(t, "claim_reply_skipped:sold_out_off"); got != skipBefore+1 {
			t.Fatalf("sold_out_off audit +%d want +1", got-skipBefore)
		}
		if n := miCount(t, e.h.f.owner, `SELECT count(*) FROM claims.links WHERE bundle_id=$1`, r.bundleID); n != 0 {
			t.Fatalf("a skipped sold-out claim got a link: %d", n)
		}
		if _, err := e.manual(r.s.comment, text("manual-1"), time.Now().Add(-10*time.Minute), false); err != nil {
			t.Fatalf("SO05 the merchant keeps one manual private reply: %v", err)
		}
		if _, err := e.manual(r.s.comment, text("manual-2"), time.Now().Add(-10*time.Minute), false); planCode(err) != "used" {
			t.Fatalf("SO05 a second manual reply: %v", err)
		}
		// Switch on again: the next sold-out claim is answered.
		if out, err := e.setSoldOut(t, true, "sold-out-reply/v1", 1, 1); err != nil || !out.Enabled || out.Version != 2 {
			t.Fatalf("switch on: %+v %v", out, err)
		}
		r3 := e.planReply(t, false, "", "A1")
		if o := e.soOpOf(t, r3.s.comment); o.Type != "sold_out_reply" {
			t.Fatalf("after switching on: %+v", o)
		}
	})

	t.Run("SO06 manual first, then the sold-out claim", func(t *testing.T) {
		ref := lbRef()
		if _, err := e.manual(ref, text("first"), time.Now().Add(-10*time.Minute), false); err != nil {
			t.Fatalf("manual before intake: %v", err)
		}
		skipBefore := e.soAudit(t, "claim_reply_skipped:reply_used")
		s := e.postFB(t, ref, "", "A1", mciAt(3*time.Second), nil)
		e.apply(t)
		if got := e.soAudit(t, "claim_reply_skipped:reply_used"); got != skipBefore+1 {
			t.Fatalf("SO06 reply_used audit +%d want +1", got-skipBefore)
		}
		if n := miCount(t, e.h.f.owner, `SELECT count(*) FROM integration.operations WHERE action='meta.private_reply' AND request->>'comment_ref'=$1`, s.comment); n != 1 {
			t.Fatalf("SO06 %d operations for the comment, want only the manual one", n)
		}
	})
}

// SO07: the same sold-out comment delivered 20 times concurrently and applied by 20 workers plans exactly one operation.
func TestSoldOutReplyConcurrentReplay(t *testing.T) {
	e := mciSetup(t, mciOpts{private: true})
	soCleanSettings(t, e)
	e.soldOut(t, 0)
	s := e.postFB(t, "", "", "A1", mciAt(3*time.Second), nil)
	var wg sync.WaitGroup
	for i := 0; i < 19; i++ { // identical signed body: the event key de-duplicates the replay
		wg.Add(1)
		go func() {
			defer wg.Done()
			mcPost(t, e.page, e.pageAsset, []byte(s.raw))
		}()
	}
	wg.Wait()
	e.applyConcurrently(t, 20)
	if n := e.opCount(t, s.comment); n != 1 {
		t.Fatalf("SO07 %d operations for one comment, want exactly 1", n)
	}
	if o := e.soOpOf(t, s.comment); o.Type != "sold_out_reply" {
		t.Fatalf("SO07 operation = %+v", o)
	}
}

// SO01 send + SO08: the dispatcher posts the frozen text once with no link; a Graph failure is UNKNOWN and never re-sent; the comment feed
// marks the sold-out reply as out_of_stock.
func TestSoldOutReplySendAndUnknown(t *testing.T) {
	e := mciSetup(t, mciOpts{private: true})
	soCleanSettings(t, e)
	g := newMciGraph(t)
	d := e.newDispatcher(t, g, nil, nil)
	e.soldOut(t, 0)
	name := e.productName(t)

	r := e.planReply(t, false, "", "A1")
	g.setMode("ok")
	d.run(t, r.op)
	_, ref := e.awaitOp(t, r.op, "SUCCEEDED", 30*time.Second, "completed")
	var posted []mciGraphReq
	for _, q := range g.all() {
		if q.comment == r.s.comment {
			posted = append(posted, q)
		}
	}
	if len(posted) != 1 || !strings.HasPrefix(ref, "m_SYNTH_") {
		t.Fatalf("SO01 posts=%d ref=%q", len(posted), ref)
	}
	q := posted[0]
	if q.method != http.MethodPost || q.path != "/v99.0/"+r.asset+"/messages" || q.bodyToken != e.pageToken ||
		!strings.Contains(q.text, name+" 已售完") || strings.Contains(q.text, "http") || strings.Contains(q.text, "#t=") {
		t.Fatalf("SO01 request %s %s text=%q", q.method, q.path, q.text)
	}

	// Feed marker: kind out_of_stock, state SUCCEEDED. console_marks resolves comments through the session's first active source, so only FB stays active.
	mustExec(t, e.h.f.owner, `UPDATE live.claim_sources SET active=false, updated_at=clock_timestamp() WHERE id=$1`, e.srcIG)
	var kind, state string
	err := platform.WithScope(context.Background(), e.h.f.runtime, e.h.token, e.h.f.storeA1, "live:read", func(tx pgx.Tx, _ platform.Scope) error {
		return tx.QueryRow(context.Background(), `SELECT private_reply_kind, private_reply_state FROM live.console_marks($1::uuid,$2::text[])`, e.session, []string{r.s.comment}).Scan(&kind, &state)
	})
	if err != nil || kind != "out_of_stock" || state != "SUCCEEDED" {
		t.Fatalf("console mark kind=%q state=%q err=%v", kind, state, err)
	}

	t.Run("SO08 Graph failure is UNKNOWN and never re-sent", func(t *testing.T) {
		r := e.planReply(t, false, "", "A1")
		g.setMode("5xx")
		d.run(t, r.op)
		e.awaitOp(t, r.op, "UNKNOWN", 40*time.Second, "cancelled", "discarded")
		g.setMode("ok") // a wrongful second POST would now succeed and be counted
		if n := g.posts(r.s.comment); n != 1 {
			t.Fatalf("SO08 %d POSTs, want exactly 1", n)
		}
		if n := e.opCount(t, r.s.comment); n != 1 {
			t.Fatal("SO08 operation count changed")
		}
	})
}

// Check denials share the claim-link semantics: a source switched off after planning blocks the sold-out reply with zero Graph calls.
func TestSoldOutReplyCheckDenial(t *testing.T) {
	e := mciSetup(t, mciOpts{private: true})
	soCleanSettings(t, e)
	g := newMciGraph(t)
	d := e.newDispatcher(t, g, nil, nil)
	e.soldOut(t, 0)
	r := e.planReply(t, false, "", "A1")
	mustExec(t, e.h.f.owner, `UPDATE live.claim_sources SET private_reply=false WHERE id=$1`, e.srcFB)
	g.setMode("ok")
	d.run(t, r.op)
	e.awaitOp(t, r.op, "BLOCKED_POLICY", 30*time.Second)
	if n := g.posts(r.s.comment); n != 0 {
		t.Fatalf("%d POSTs for a denied sold-out reply", n)
	}
}

// Settings + template rules: defaults, CAS, permission, usable templates only, a store template renders, an unusable chosen template falls back.
func TestSoldOutReplySettingsAndTemplates(t *testing.T) {
	e := mciSetup(t, mciOpts{private: true})
	soCleanSettings(t, e)
	f := e.h.f

	if got := e.getSoldOut(t); !got.Enabled || got.TemplateID != "sold-out-reply/v1" || got.TemplateVersion != 1 || got.Version != 0 {
		t.Fatalf("default = %+v", got)
	}
	if out, err := e.setSoldOut(t, true, "sold-out-reply/v1", 1, 0); err != nil || out.Version != 1 {
		t.Fatalf("first save: %+v %v", out, err)
	}
	if _, err := e.setSoldOut(t, false, "sold-out-reply/v1", 1, 0); !errors.Is(err, command.ErrConflict) {
		t.Fatalf("a second first-save: %v", err)
	}
	if _, err := e.setSoldOut(t, false, "sold-out-reply/v1", 1, 7); !errors.Is(err, command.ErrConflict) {
		t.Fatalf("stale version: %v", err)
	}
	if n := e.soAudit(t, "claims.sold_out_reply.set"); n < 1 {
		t.Fatalf("settings audit missing: %d", n)
	}

	pub := func(id string, body string, kinds string, safe bool) {
		mustExec(t, f.owner, `INSERT INTO msgtemplates.templates(tenant_id,store_id,template_id,version,name,body,kinds,public_safe,created_by)
			VALUES($1,$2,$3,1,$3,$4,$5::text[],$6,$7)`, f.tenantA, f.storeA1, id, body, kinds, safe, e.h.actor)
	}
	pub("so-good", "Sorry, {{product.name}} is gone", "{private_reply}", false)
	pub("so-dm-only", "Sorry {{product.name}}", "{dm}", false)
	pub("so-link", "Sorry {{連結}}", "{private_reply}", false)
	pub("so-other", "Sorry {{variant}}", "{private_reply}", false)
	pub("so-long", strings.Repeat("好", 281), "{private_reply}", false)
	pub("so-newline", "a\nb", "{private_reply}", false)
	for _, id := range []string{"so-dm-only", "so-link", "so-other", "so-long", "so-newline", "so-missing", "offer-recommend/v1", "order-pay-link/v1"} {
		if _, err := e.setSoldOut(t, true, id, 1, 1); !errors.Is(err, command.ErrInvalid) {
			t.Errorf("template %s accepted: %v", id, err)
		}
	}
	if _, err := e.setSoldOut(t, true, "so-good", 2, 1); !errors.Is(err, command.ErrInvalid) {
		t.Errorf("missing version accepted: %v", err)
	}
	// A store cannot choose another store's template.
	mustExec(t, f.owner, `INSERT INTO msgtemplates.templates(tenant_id,store_id,template_id,version,name,body,kinds,public_safe,created_by)
		VALUES($1,$2,'so-other-store',1,'x','Hi {{product.name}}','{private_reply}',false,$3)`, f.tenantA, f.storeA2, e.h.actor)
	t.Cleanup(func() {
		_, _ = f.owner.Exec(context.Background(), `DELETE FROM msgtemplates.templates WHERE template_id='so-other-store'`)
	})
	if _, err := e.setSoldOut(t, true, "so-other-store", 1, 1); !errors.Is(err, command.ErrInvalid) {
		t.Errorf("cross-store template accepted: %v", err)
	}

	// Read-only principal: forbidden.
	_, ro := lcPrincipal(t, f, f.tenantA, []string{f.storeA1}, "store:read", "live:read")
	err := platform.WithScope(context.Background(), f.runtime, ro, f.storeA1, "live:read", func(tx pgx.Tx, _ platform.Scope) error {
		if _, err := claims.GetSoldOutReply(context.Background(), tx); err != nil {
			return err
		}
		_, err := claims.SetSoldOutReply(context.Background(), tx, false, "sold-out-reply/v1", 1, 1)
		return err
	})
	if !errors.Is(err, platform.ErrForbidden) {
		t.Fatalf("live:read principal saved the setting: %v", err)
	}

	// A chosen store template renders (and a template that later became unusable falls back to the fixed one instead of failing the claim).
	if out, err := e.setSoldOut(t, true, "so-good", 1, 1); err != nil || out.TemplateID != "so-good" || out.Version != 2 {
		t.Fatalf("choose store template: %+v %v", out, err)
	}
	e.soldOut(t, 0)
	name := e.productName(t)
	r := e.planReply(t, false, "", "A1")
	if o := e.soOpOf(t, r.s.comment); o.Type != "sold_out_reply" || o.Text != "Sorry, "+name+" is gone" {
		t.Fatalf("store template render = %+v", o)
	}
	mustExec(t, f.owner, `UPDATE claims.sold_out_settings SET template_id='so-gone', template_version=9 WHERE tenant_id=$1 AND store_id=$2`, f.tenantA, f.storeA1)
	r = e.planReply(t, false, "", "A1")
	if o := e.soOpOf(t, r.s.comment); o.Type != "sold_out_reply" || !strings.Contains(o.Text, name+" 已售完") {
		t.Fatalf("fallback render = %+v", o)
	}
	var tpl string
	if err := f.owner.QueryRow(context.Background(), `SELECT request->>'template' FROM integration.operations WHERE id=$1`, e.soOpOf(t, r.s.comment).ID).Scan(&tpl); err != nil || tpl != "sold-out-reply/v1" {
		t.Fatalf("fallback template recorded as %q %v", tpl, err)
	}
}

// ACL pins of 0151: definer owners, grants, no PUBLIC execute, the table is closed to login roles.
func TestSoldOutReplyACL(t *testing.T) {
	e := mciSetup(t, mciOpts{})
	f := e.h.f
	ctx := context.Background()
	type fn struct{ sig, owner string }
	for _, c := range []fn{
		{"integration.claim_sold_out_facts(uuid,uuid,uuid)", "commerce_integration_writer"},
		{"inventory.claim_sku_sold_out(uuid,integer)", "commerce_inventory_writer"},
		{"msgtemplates.sold_out_body(uuid,uuid,text,bigint)", "commerce_msgtemplates_writer"},
		{"claims.get_sold_out_reply()", "commerce_integration_writer"},
		{"claims.set_sold_out_reply(boolean,text,bigint,bigint)", "commerce_integration_writer"},
		{"integration.plan_claim_reply(uuid,uuid,bytea,text,bigint)", "commerce_integration_writer"},
		{"integration.claim_reply_plannable(uuid)", "commerce_integration_writer"},
	} {
		var owner string
		if err := f.owner.QueryRow(ctx, `SELECT pg_get_userbyid(proowner) FROM pg_proc WHERE oid=$1::regprocedure`, c.sig).Scan(&owner); err != nil || owner != c.owner {
			t.Errorf("%s owner %q err %v, want %s", c.sig, owner, err, c.owner)
		}
		for _, role := range []string{"public", "commerce_buyer_runtime", "commerce_claims_worker"} {
			var ok bool
			if err := f.owner.QueryRow(ctx, `SELECT has_function_privilege($1::text,$2::regprocedure,'EXECUTE')`, role, c.sig).Scan(&ok); err != nil || ok {
				t.Errorf("%s executable by %s (%v)", c.sig, role, err)
			}
		}
	}
	for sig, role := range map[string]string{"claims.get_sold_out_reply()": "commerce_runtime", "claims.set_sold_out_reply(boolean,text,bigint,bigint)": "commerce_runtime",
		"inventory.claim_sku_sold_out(uuid,integer)": "commerce_integration_writer", "msgtemplates.sold_out_body(uuid,uuid,text,bigint)": "commerce_integration_writer"} {
		var ok bool
		if err := f.owner.QueryRow(ctx, `SELECT has_function_privilege($1,$2::regprocedure,'EXECUTE')`, role, sig).Scan(&ok); err != nil || !ok {
			t.Errorf("%s not executable by %s (%v)", sig, role, err)
		}
	}
	for _, role := range []string{"commerce_runtime", "commerce_buyer_runtime", "commerce_claims_worker", "commerce_claims_intake"} {
		var ok bool
		if err := f.owner.QueryRow(ctx, `SELECT has_table_privilege($1,'claims.sold_out_settings','SELECT,INSERT,UPDATE,DELETE')`, role).Scan(&ok); err != nil || ok {
			t.Errorf("%s reaches claims.sold_out_settings directly (%v)", role, err)
		}
	}
	var rls, force bool
	if err := f.owner.QueryRow(ctx, `SELECT relrowsecurity, relforcerowsecurity FROM pg_class WHERE oid='claims.sold_out_settings'::regclass`).Scan(&rls, &force); err != nil || !rls || !force {
		t.Errorf("claims.sold_out_settings RLS/FORCE = %v/%v (%v)", rls, force, err)
	}
	var def string
	if err := f.owner.QueryRow(ctx, `SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conname='fixed_templates_template_id_check'`).Scan(&def); err != nil ||
		!strings.Contains(def, "sold-out-reply/v1") || !strings.Contains(def, "checkout-reminder/v1") {
		t.Errorf("fixed template id CHECK lost an id: %q %v", def, err)
	}
}

// P1-1 (review): the text SQL freezes must be exactly what the Go adapter accepts, whatever the product name holds, so a sold-out reply can never
// consume the comment's one private reply without sending. Full-width space, ZWJ and a newline in the name, a 120-character name, a template with
// several placeholders and a C1 control character all end in a SENT message (or a refused template), never UNKNOWN-with-no-send or a lost claim.
func TestSoldOutReplyTextMatchesAdapter(t *testing.T) {
	e := mciSetup(t, mciOpts{private: true})
	soCleanSettings(t, e)
	f := e.h.f
	g := newMciGraph(t)
	d := e.newDispatcher(t, g, nil, nil)
	e.soldOut(t, 0)
	rename := func(name string) {
		mustExec(t, f.owner, `UPDATE catalog.products SET name=$1 WHERE id=(SELECT product_id FROM catalog.skus WHERE id=$2)`, name, e.sku)
	}
	sendOK := func(t *testing.T, r mciReply, want string) {
		t.Helper()
		g.setMode("ok")
		d.run(t, r.op)
		e.awaitOp(t, r.op, "SUCCEEDED", 30*time.Second, "completed")
		for _, q := range g.all() {
			if q.comment == r.s.comment {
				if q.text != want {
					t.Fatalf("sent %q, want %q", q.text, want)
				}
				return
			}
		}
		t.Fatal("nothing was posted")
	}

	t.Run("full-width space, ZWJ and newline in the name", func(t *testing.T) {
		rename("韓版　針織衫‍\n外套")
		r := e.planReply(t, false, "", "A1")
		want := "抱歉，韓版　針織衫‍ 外套 已售完，補貨時會在直播中通知，請留意直播。"
		if o := e.soOpOf(t, r.s.comment); o.Type != "sold_out_reply" || o.Text != want {
			t.Fatalf("frozen text %q, want %q", o.Text, want)
		}
		sendOK(t, r, want)
	})
	t.Run("120-character name is clipped by characters and still sends", func(t *testing.T) {
		rename(strings.Repeat("好", 120))
		r := e.planReply(t, false, "", "A1")
		want := "抱歉，" + strings.Repeat("好", 60) + " 已售完，補貨時會在直播中通知，請留意直播。"
		if o := e.soOpOf(t, r.s.comment); o.Text != want {
			t.Fatalf("frozen text %q, want %q", o.Text, want)
		}
		sendOK(t, r, want)
	})
	t.Run("multi-placeholder or C1-control templates are refused and a stored one falls back", func(t *testing.T) {
		rename("測試商品")
		pub := func(id, body string) {
			mustExec(t, f.owner, `INSERT INTO msgtemplates.templates(tenant_id,store_id,template_id,version,name,body,kinds,public_safe,created_by)
				VALUES($1,$2,$3,1,$3,$4,'{private_reply}',false,$5)`, f.tenantA, f.storeA1, id, body, e.h.actor)
		}
		pub("so-six", strings.Repeat("{{product.name}}", 6))
		pub("so-two", "{{product.name}} / {{product.name}}")
		pub("so-c1", "a\u0085b {{product.name}}")
		for _, id := range []string{"so-six", "so-two", "so-c1"} {
			if _, err := e.setSoldOut(t, true, id, 1, 0); !errors.Is(err, command.ErrInvalid) {
				t.Errorf("template %s accepted: %v", id, err)
			}
		}
		// A row that bypassed the setter (older data / owner repair) must not lose or strand the claim: the fixed text is planned instead.
		mustExec(t, f.owner, `INSERT INTO claims.sold_out_settings(tenant_id,store_id,enabled,template_id,template_version,version,principal_id) VALUES($1,$2,true,'so-six',1,1,$3)`, f.tenantA, f.storeA1, e.h.actor)
		r := e.planReply(t, false, "", "A1")
		want := "抱歉，測試商品 已售完，補貨時會在直播中通知，請留意直播。"
		var tpl string
		if err := f.owner.QueryRow(context.Background(), `SELECT request->>'template' FROM integration.operations WHERE id=$1`, r.op).Scan(&tpl); err != nil || tpl != "sold-out-reply/v1" || r.ev.outcome != "ACCEPTED" {
			t.Fatalf("fallback: template %q outcome %q err %v", tpl, r.ev.outcome, err)
		}
		sendOK(t, r, want)
	})
}

// P1-2 (review): claim_reply_plannable and plan_claim_reply read stock and the switch in separate statements. A change between them (simulated by a
// trigger on the River job insert, which sits exactly between the two calls in the poller) must never raise a final error and lose the claim: the
// disagreement takes the normal link branch.
func TestSoldOutReplyDisagreementKeepsTheClaim(t *testing.T) {
	e := mciSetup(t, mciOpts{private: true})
	soCleanSettings(t, e)
	f := e.h.f
	flipTo := func(units int) {
		// Database-level settings keep the trigger's inputs out of the poller's session.
		mustExec(t, f.owner, `DROP TRIGGER IF EXISTS so_flip ON river.river_job`)
		mustExec(t, f.owner, `CREATE FUNCTION public.so_flip_tmp() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
			BEGIN UPDATE inventory.balances SET on_hand=reserved+allocated+unavailable+`+strconv.Itoa(units)+` WHERE sku_id='`+e.sku+`'; RETURN NEW; END $$`)
		mustExec(t, f.owner, `CREATE TRIGGER so_flip AFTER INSERT ON river.river_job FOR EACH ROW WHEN (NEW.kind='external_operation_v1') EXECUTE FUNCTION public.so_flip_tmp()`)
		t.Cleanup(func() {
			_, _ = f.owner.Exec(context.Background(), `DROP TRIGGER IF EXISTS so_flip ON river.river_job`)
			_, _ = f.owner.Exec(context.Background(), `DROP FUNCTION IF EXISTS public.so_flip_tmp()`)
		})
	}
	done := func() {
		mustExec(t, f.owner, `DROP TRIGGER IF EXISTS so_flip ON river.river_job`)
		mustExec(t, f.owner, `DROP FUNCTION IF EXISTS public.so_flip_tmp()`)
	}

	t.Run("switch OFF, stock disappears between the two checks", func(t *testing.T) {
		if _, err := e.setSoldOut(t, false, "sold-out-reply/v1", 1, 0); err != nil {
			t.Fatal(err)
		}
		e.soldOut(t, 5)
		flipTo(0)
		defer done()
		r := e.planReply(t, false, "", "A1")
		if r.intake.State != "APPLIED" || r.ev.outcome != "ACCEPTED" {
			t.Fatalf("claim lost: intake %s/%s outcome %q", r.intake.State, r.intake.FailCode, r.ev.outcome)
		}
		if o := e.soOpOf(t, r.s.comment); o.Type != "first_private_reply" || o.Links != 1 {
			t.Fatalf("want the normal link reply, got %+v", o)
		}
	})
	t.Run("switch ON, stock returns between the two checks", func(t *testing.T) {
		if _, err := e.setSoldOut(t, true, "sold-out-reply/v1", 1, 1); err != nil {
			t.Fatal(err)
		}
		e.soldOut(t, 0)
		flipTo(5)
		defer done()
		r := e.planReply(t, false, "", "A1")
		if r.intake.State != "APPLIED" || r.ev.outcome != "ACCEPTED" {
			t.Fatalf("claim lost: intake %s/%s", r.intake.State, r.intake.FailCode)
		}
		if o := e.soOpOf(t, r.s.comment); o.Type != "first_private_reply" || o.Links != 1 {
			t.Fatalf("want the normal link reply, got %+v", o)
		}
		if k := miCount(t, f.owner, `SELECT count(*) FROM claims.events WHERE id=$1 AND reply_kind IS NOT NULL`, r.intake.AppliedEvent); k != 0 {
			t.Fatal("a link reply must not mark the event sold_out")
		}
	})
}

// The claim event carries reply_kind='sold_out' exactly when the sold-out reply was planned (brief scope 2), and the column is a closed vocabulary.
func TestSoldOutReplyEventMark(t *testing.T) {
	e := mciSetup(t, mciOpts{private: true})
	soCleanSettings(t, e)
	f := e.h.f
	e.soldOut(t, 0)
	r := e.planReply(t, false, "", "A1")
	var kind *string
	if err := f.owner.QueryRow(context.Background(), `SELECT reply_kind FROM claims.events WHERE id=$1`, r.intake.AppliedEvent).Scan(&kind); err != nil || kind == nil || *kind != "sold_out" {
		t.Fatalf("event reply_kind = %v (%v), want sold_out", kind, err)
	}
	e.soldOut(t, 9)
	r = e.planReply(t, false, "", "A1")
	if err := f.owner.QueryRow(context.Background(), `SELECT reply_kind FROM claims.events WHERE id=$1`, r.intake.AppliedEvent).Scan(&kind); err != nil || kind != nil {
		t.Fatalf("link-reply event reply_kind = %v (%v), want NULL", kind, err)
	}
	if _, err := f.owner.Exec(context.Background(), `UPDATE claims.events SET reply_kind='other' WHERE id=$1`, r.intake.AppliedEvent); err == nil {
		t.Fatal("reply_kind accepted a value outside {NULL, sold_out}")
	}
}
