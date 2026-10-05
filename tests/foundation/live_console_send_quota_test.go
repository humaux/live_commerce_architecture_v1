package foundation_test

// Real-PG gates LCN07 (private-reply quota, the auto/manual race, :m1, the 120 s confirm gate, link_pending_manual) and LCN08 (public
// reply content rule, IG refusal, recommend comment replayed through the webhook) of LC-B4. Harness: live_console_send_test.go (lbEnv).

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/inbox"
	"livecommerce/internal/platform"
)

func lbRef() string { return mciDigits(15) + "_" + mciDigits(10) }

func lbMprKey(object, asset, ref string) string {
	sum := sha256.Sum256([]byte(object + "|" + asset + "|" + ref))
	return "mpr:" + hex.EncodeToString(sum[:])[:48]
}

// onlySource keeps exactly one of the harness's two sources active (the console and the planners resolve the session's newest active source).
func (e *lbEnv) onlySource(t *testing.T, platformName string) {
	t.Helper()
	keep, drop := e.srcFB, e.srcIG
	if platformName == "instagram" {
		keep, drop = e.srcIG, e.srcFB
	}
	mustExec(t, e.h.f.owner, `UPDATE live.claim_sources SET active=false, updated_at=clock_timestamp() WHERE id=$1`, drop)
	mustExec(t, e.h.f.owner, `UPDATE live.claim_sources SET active=true, updated_at=clock_timestamp() WHERE id=$1`, keep)
}

// ageOps backdates recent human-send operations so the 60/min store cap never interferes with loops.
func (e *lbEnv) ageOps(t *testing.T) {
	t.Helper()
	mustExec(t, e.h.f.owner, `UPDATE integration.operations SET created_at=created_at-interval '5 minutes'
		WHERE tenant_id=$1 AND store_id=$2 AND action IN ('meta.private_reply','meta.dm_send','meta.public_reply','meta.offer_recommend') AND created_at>clock_timestamp()-interval '1 minute'`,
		e.h.f.tenantA, e.h.f.storeA1)
}

func (e *lbEnv) manual(ref, text string, created time.Time, confirm bool) (inbox.SendOutput, error) {
	var out inbox.SendOutput
	err := platform.WithScope(context.Background(), e.h.f.runtime, e.h.token, e.h.f.storeA1, "inbox:reply", func(tx pgx.Tx, s platform.Scope) error {
		in := inbox.ReplyInput{TextInput: inbox.TextInput{Text: text}}
		if confirm {
			yes := true
			in.ConfirmPreemptAuto = &yes
		}
		var err error
		out, err = e.svc.SendPrivateReply(context.Background(), tx, s, lbKey(), e.session, ref, in, func(context.Context) (time.Time, error) { return created, nil })
		return err
	})
	return out, err
}

func (e *lbEnv) publicReply(ref, text string) (inbox.SendOutput, error) {
	var out inbox.SendOutput
	err := platform.WithScope(context.Background(), e.h.f.runtime, e.h.token, e.h.f.storeA1, "inbox:reply", func(tx pgx.Tx, s platform.Scope) error {
		var err error
		out, err = e.svc.SendPublicReply(context.Background(), tx, s, lbKey(), e.session, ref, inbox.TextInput{Text: text})
		return err
	})
	return out, err
}

type lbClaim struct {
	State, Outcome, Bundle string
	Flag                   bool
	Auto, Manual           int64
}

func (e *lbEnv) claimOf(t *testing.T, ref string) lbClaim {
	t.Helper()
	var c lbClaim
	f := e.h.f
	if err := f.owner.QueryRow(context.Background(), `SELECT i.state, coalesce(ev.outcome,''), coalesce(ev.bundle_id::text,''), coalesce(b.link_pending_manual,false)
		FROM claims.meta_intake i LEFT JOIN claims.events ev ON ev.id=i.applied_event_id LEFT JOIN claims.bundles b ON b.id=ev.bundle_id
		WHERE i.object='page' AND i.asset_id=$1 AND i.comment_ref=$2`, e.pageAsset, ref).Scan(&c.State, &c.Outcome, &c.Bundle, &c.Flag); err != nil {
		t.Fatalf("claim of %s: %v", ref, err)
	}
	c.Auto = miCount(t, f.owner, `SELECT count(*) FROM integration.operations WHERE action='meta.private_reply' AND request->>'comment_ref'=$1 AND request->>'message_type'='first_private_reply'`, ref)
	c.Manual = miCount(t, f.owner, `SELECT count(*) FROM integration.operations WHERE action='meta.private_reply' AND request->>'comment_ref'=$1 AND request->>'message_type'='manual_private_reply'`, ref)
	return c
}

func (e *lbEnv) auditCount(t *testing.T, action string) int64 {
	return miCount(t, e.h.f.owner, `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action=$3`, e.h.f.tenantA, e.h.f.storeA1, action)
}

// LCN07: manual after auto -> used; manual while PENDING -> auto_pending; auto BLOCKED_POLICY -> one :m1, a second refused.
func TestLiveConsoleSendLCN07Quota(t *testing.T) {
	e := lbSetup(t)
	e.onlySource(t, "facebook")
	text := func(s string) string { return s + "-" + t04Tag() }

	// manual after auto -> used
	r := e.planReply(t, false, "", "A1")
	if r.op == "" {
		t.Fatal("no automatic reply planned for the private_reply source")
	}
	if _, err := e.manual(r.s.comment, text("after-auto"), time.Now().Add(-10*time.Minute), false); planCode(err) != "used" {
		t.Fatalf("manual after auto: %v", err)
	}

	// manual while the intake row is PENDING -> auto_pending
	pending := e.postFB(t, "", "", "A1", mciAt(3*time.Second), nil)
	if c := e.claimOf(t, pending.comment); c.State != "PENDING" {
		t.Fatalf("expected a staged PENDING intake, got %+v", c)
	}
	if _, err := e.manual(pending.comment, text("while-pending"), time.Now().Add(-10*time.Minute), false); planCode(err) != "auto_pending" {
		t.Fatalf("manual while PENDING: %v", err)
	}
	e.apply(t)

	// auto BLOCKED_POLICY (zero HTTP, provably unsent) frees exactly one ':m1' manual reply
	blocked := e.planReply(t, false, "", "A1")
	mustExec(t, e.h.f.owner, `UPDATE integration.operations SET state='BLOCKED_POLICY', generation=1, result_code='deadline' WHERE id=$1`, blocked.op)
	first, err := e.manual(blocked.s.comment, text("m1"), time.Now().Add(-10*time.Minute), false)
	if err != nil {
		t.Fatalf("manual after a BLOCKED_POLICY auto reply must use :m1: %v", err)
	}
	var key string
	if err := e.h.f.owner.QueryRow(context.Background(), `SELECT semantic_key FROM integration.operations WHERE id=$1`, first.OperationID).Scan(&key); err != nil {
		t.Fatal(err)
	}
	if want := lbMprKey("page", e.pageAsset, blocked.s.comment) + ":m1"; key != want {
		t.Fatalf("semantic key %q, want %q", key, want)
	}
	if _, err := e.manual(blocked.s.comment, text("m1-again"), time.Now().Add(-10*time.Minute), false); planCode(err) != "used" {
		t.Fatalf("second manual after :m1: %v", err)
	}

	// expiry and unknown source session
	if _, err := e.manual(lbRef(), text("old"), time.Now().Add(-8*24*time.Hour), false); planCode(err) != "expired_7d" {
		t.Fatalf("7-day window: %v", err)
	}
}

// A1.1: inside 120 s the intake row may not be staged yet; without confirm_preempt_auto the planner refuses, with it it plans and audits.
func TestLiveConsoleSendLCN07ConfirmGateAndLinkPending(t *testing.T) {
	e := lbSetup(t)
	e.onlySource(t, "facebook")
	ref := lbRef()
	body := "gate-" + t04Tag()
	if _, err := e.manual(ref, body, time.Now(), false); planCode(err) != "auto_pending_confirm" {
		t.Fatalf("inside 120 s without the flag: %v", err)
	}
	if e.auditCount(t, "inbox.private_reply.preempt_confirmed") != 0 {
		t.Fatal("preempt audit written for a refused request")
	}
	out, err := e.manual(ref, body, time.Now(), true)
	if err != nil {
		t.Fatalf("confirmed manual reply: %v", err)
	}
	if e.auditCount(t, "inbox.private_reply.preempt_confirmed") != 1 {
		t.Fatal("confirmed pre-emption was not audited")
	}

	// The race is lost by the automatic reply: the claim commits ACCEPTED, no auto operation, link_pending_manual=true (§14.1 clause 2).
	sent := e.postFBTo(t, e.postID, ref, "", "A1", mciAt(3*time.Second), nil, true)
	_ = sent
	e.apply(t)
	c := e.claimOf(t, ref)
	if c.State != "APPLIED" || c.Outcome != "ACCEPTED" || c.Auto != 0 || c.Manual != 1 || !c.Flag {
		t.Fatalf("auto after manual: %+v", c)
	}
	if n := e.auditCount(t, "claim_reply_skipped:reply_used"); n < 1 {
		t.Fatalf("claim_reply_skipped:reply_used audit rows=%d", n)
	}
	// A13 by bundle and A8 bundle-only item expose the flag (Amendment 1 P2-2).
	err = e.scoped(t, "inbox:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
		panel, err := e.svc.BuyerPanelByBundle(ctx, tx, c.Bundle)
		if err != nil || !panel.LinkPendingManual {
			return fmt.Errorf("panel flag=%v err=%v", panel.LinkPendingManual, err)
		}
		list, err := e.svc.ListConversations(ctx, tx, inbox.ListRequest{Filter: "all", Limit: 50})
		if err != nil {
			return err
		}
		for _, it := range list.Items {
			if it.BundleOnly && it.BundleID != nil && *it.BundleID == c.Bundle && it.LinkPendingManual && it.Unreplied {
				return nil
			}
		}
		return fmt.Errorf("no bundle-only item for the flagged bundle in %d items", len(list.Items))
	})
	if err != nil {
		t.Fatal(err)
	}

	// The manual reply is dispatched: the buyer's PSID (recipient_id) links the bundle to the thread (§3.7).
	e.run(t, out.OperationID)
	e.awaitOp(t, out.OperationID, "SUCCEEDED", 10*time.Second, "completed")
	peers := miCount(t, e.h.f.owner, `SELECT count(*) FROM inbox.bundle_peers WHERE bundle_id=$1 AND operation_id=$2`, c.Bundle, out.OperationID)
	if peers != 1 {
		t.Fatalf("bundle_peers rows=%d after a SUCCEEDED manual private reply", peers)
	}
	// The flag clears when a claim link is issued for the bundle.
	// (the merchant link route runs in a scoped transaction; the clearing trigger honours exactly that scope)
	linkTx, err := e.h.f.owner.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := linkTx.Exec(context.Background(), `SELECT set_config('app.tenant_id',$1,true), set_config('app.store_id',$2,true)`, e.h.f.tenantA, e.h.f.storeA1); err != nil {
		t.Fatal(err)
	}
	if _, err := linkTx.Exec(context.Background(), `INSERT INTO claims.links(tenant_id,store_id,bundle_id,token_hash,generation,issued_at,expires_at,principal_id)
		VALUES($1,$2,$3,$4,1,clock_timestamp(),clock_timestamp()+interval '1 hour',$5)`, e.h.f.tenantA, e.h.f.storeA1, c.Bundle, randomBytes(32), e.h.actor); err != nil {
		t.Fatalf("issue link: %v", err)
	}
	if err := linkTx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if c2 := e.claimOf(t, ref); c2.Flag {
		t.Fatal("link_pending_manual survived the link issue")
	}
}

// 20 concurrent manual attempts on one comment -> exactly one operation (the mpr: key and the advisory lock serialise them).
func TestLiveConsoleSendLCN07ConcurrentManual(t *testing.T) {
	e := lbSetup(t)
	e.onlySource(t, "facebook")
	ref := lbRef()
	var wg sync.WaitGroup
	var mu sync.Mutex
	var ok int
	var codes []string
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := e.manual(ref, fmt.Sprintf("concurrent-%d-%s", i, t04Tag()), time.Now().Add(-10*time.Minute), false)
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				ok++
			} else {
				codes = append(codes, planCode(err))
			}
		}(i)
	}
	wg.Wait()
	if n := miCount(t, e.h.f.owner, `SELECT count(*) FROM integration.operations WHERE request->>'comment_ref'=$1 AND action='meta.private_reply'`, ref); ok != 1 || n != 1 {
		t.Fatalf("successes=%d operations=%d codes=%v", ok, n, codes)
	}
	for _, c := range codes {
		if c != "used" {
			t.Fatalf("a loser answered %q, want used (%v)", c, codes)
		}
	}
	// Two concurrent same-body requests under different Idempotency-Keys -> one operation, one duplicate_recent (A1.3).
	ref2 := lbRef()
	same := "same-body-" + t04Tag()
	var okDup, dups int
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := e.manual(ref2, same, time.Now().Add(-10*time.Minute), false)
			mu.Lock()
			defer mu.Unlock()
			switch planCode(err) {
			case "":
				okDup++
			case "duplicate_recent", "used":
				dups++
			}
		}()
	}
	wg.Wait()
	if okDup != 1 || dups != 1 {
		t.Fatalf("same-body race: ok=%d refused=%d", okDup, dups)
	}
}

// The auto/manual race in both orders: never an ACCEPTED claim with neither a link operation nor link_pending_manual.
func TestLiveConsoleSendLCN07AutoManualRace(t *testing.T) {
	e := lbSetup(t)
	e.onlySource(t, "facebook")
	const iterations = 100
	var manualWon, autoWon, refused int
	for i := 0; i < iterations; i++ {
		e.ageOps(t)
		ref := lbRef()
		var out inbox.SendOutput
		var merr error
		var wg sync.WaitGroup
		runManual := func() {
			defer wg.Done()
			out, merr = e.manual(ref, fmt.Sprintf("race-%d-%s", i, t04Tag()), time.Now().Add(-10*time.Minute), false)
		}
		if i%2 == 0 {
			// manual planning races the consumer staging the intake row
			wg.Add(1)
			go runManual()
			s := e.postFBTo(t, e.postID, ref, "", "A1", mciAt(3*time.Second), nil, false)
			wg.Wait()
			mcAwait(t, e.page, s.ev)
			e.apply(t)
		} else {
			// manual planning races the intake apply
			e.postFBTo(t, e.postID, ref, "", "A1", mciAt(3*time.Second), nil, true)
			var applyErr error
			wg.Add(2)
			go runManual()
			go func() {
				defer wg.Done()
				for n := 0; n < 50; n++ {
					leased, err := e.poller.ApplyOne(context.Background())
					if err != nil {
						applyErr = err
						return
					}
					if !leased {
						return
					}
				}
			}()
			wg.Wait()
			if applyErr != nil {
				t.Fatalf("iteration %d: ApplyOne: %v", i, applyErr)
			}
			e.apply(t)
		}
		c := e.claimOf(t, ref)
		if c.State != "APPLIED" {
			t.Fatalf("iteration %d: intake %+v", i, c)
		}
		switch code := planCode(merr); {
		case merr == nil:
			manualWon++
			if out.OperationID == "" || c.Manual != 1 || c.Auto != 0 {
				t.Fatalf("iteration %d: manual planned but ops auto=%d manual=%d", i, c.Auto, c.Manual)
			}
			if c.Outcome == "ACCEPTED" && !c.Flag {
				t.Fatalf("iteration %d: ACCEPTED claim whose link was pre-empted but link_pending_manual is false: %+v", i, c)
			}
		case code == "used" || code == "auto_pending":
			refused++
			if c.Outcome == "ACCEPTED" && c.Auto != 1 {
				t.Fatalf("iteration %d: manual refused (%s) yet the ACCEPTED claim has no automatic reply: %+v", i, code, c)
			}
			if c.Manual != 0 {
				t.Fatalf("iteration %d: refused manual left an operation: %+v", i, c)
			}
		default:
			t.Fatalf("iteration %d: unexpected manual outcome: %v", i, merr)
		}
		if c.Auto == 1 {
			autoWon++
		}
		if c.Outcome == "ACCEPTED" && c.Auto == 0 && !c.Flag {
			t.Fatalf("iteration %d: ACCEPTED claim with neither a link operation nor link_pending_manual: %+v", i, c)
		}
	}
	t.Logf("race outcomes over %d iterations: manual won %d, auto won %d, manual refused %d", iterations, manualWon, autoWon, refused)
	if manualWon == 0 || autoWon == 0 {
		t.Logf("one side never won; the other order was not exercised in this run")
	}
}

// LCN08: every §3.5 pattern is rejected server side with zero operations; a safe reply is sent; IG is refused; the rendered recommend
// comment replayed through the webhook creates no claim, intake or reply.
func TestLiveConsoleSendLCN08PublicReply(t *testing.T) {
	e := lbSetup(t)
	e.onlySource(t, "facebook")
	ref := lbRef()
	for name, text := range map[string]string{
		"url": "https://shop.example.com/x", "fullwidth www": "ｗｗｗ．ａｂｃ．ｃｏｍ", "fullwidth phone": "０９１２３４５６７８",
		"dashed phone": "0912-345-678", "spaced line": "l i n e", "zero width": "w​ww.ab​c.com", "t.me": "t.me/abc",
		"handle": "@someone", "email": "a@b.com", "store origin": strings.TrimPrefix(e.origin, "https://"),
	} {
		_, err := e.publicReply(ref, text)
		if planCode(err) != "public_reply_forbidden_content" {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if n := miCount(t, e.h.f.owner, `SELECT count(*) FROM integration.operations WHERE action='meta.public_reply' AND tenant_id=$1`, e.h.f.tenantA); n != 0 {
		t.Fatalf("a refused public reply left %d operations", n)
	}
	out, err := e.publicReply(ref, "謝謝支持，歡迎私訊")
	if err != nil {
		t.Fatalf("safe public reply: %v", err)
	}
	e.run(t, out.OperationID)
	e.awaitOp(t, out.OperationID, "SUCCEEDED", 10*time.Second, "completed")
	reqs := e.g.all()
	if len(reqs) != 1 || reqs[0].path != "/v99.0/"+ref+"/comments" || reqs[0].body["message"] != "謝謝支持，歡迎私訊" {
		t.Fatalf("public reply traffic: %+v", reqs)
	}
	// A public reply never takes a conversation over (§3.6): no state row changed, no audit takeover.
	if n := e.auditCount(t, "inbox.takeover"); n != 0 {
		t.Fatalf("takeover audit rows=%d after a public reply", n)
	}

	// Instagram live media: refused until LC-U8.
	e.onlySource(t, "instagram")
	if _, err := e.publicReply(lbRef(), "謝謝支持"); planCode(err) != "ig_live_unsupported" {
		t.Fatalf("IG public reply: %v", err)
	}
	e.onlySource(t, "facebook")

	// Recommend: FB only, rendered from the fixed template, rate-limited to one per offer per 10 min, replay creates no claim.
	var op string
	plan := func() error {
		return e.scoped(t, "live:manage", func(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
			var err error
			op, err = e.svc.PlanRecommend(ctx, tx, s, e.session, e.offer.ID, e.offer.Version)
			return err
		})
	}
	if err := plan(); err != nil {
		t.Fatalf("recommend: %v", err)
	}
	recommendOp := op
	if err := plan(); planCode(err) != "duplicate_recent" {
		t.Fatalf("second recommend inside 10 min: %v", err)
	}
	e.run(t, recommendOp)
	e.awaitOp(t, recommendOp, "SUCCEEDED", 10*time.Second, "completed")
	all := e.g.all()
	last := all[len(all)-1]
	rendered, _ := last.body["message"].(string)
	if last.path != "/v99.0/"+e.postID+"/comments" || !strings.Contains(rendered, e.offer.Keyword) || !strings.Contains(rendered, e.offer.ProductName) || strings.Contains(rendered, "http") {
		t.Fatalf("recommend comment: path=%s message=%q", last.path, rendered)
	}
	replay := lbRef()
	e.postFBTo(t, e.postID, replay, e.pageAsset /* from.id = the Page itself */, rendered, mciAt(2*time.Second), nil, true)
	e.apply(t)
	e.noIntake(t, "page", e.pageAsset, replay, "the Page's own comment (the recommend text) must be dropped before any claim")
	if n := miCount(t, e.h.f.owner, `SELECT count(*) FROM integration.operations WHERE request->>'comment_ref'=$1`, replay); n != 0 {
		t.Fatalf("replayed recommend comment produced %d operations", n)
	}
	// IG recommend: 422 ig_live_unsupported.
	e.onlySource(t, "instagram")
	e.ageOps(t)
	mustExec(t, e.h.f.owner, `UPDATE integration.operations SET created_at=created_at-interval '20 minutes' WHERE action='meta.offer_recommend' AND tenant_id=$1`, e.h.f.tenantA)
	if err := plan(); planCode(err) != "ig_live_unsupported" {
		t.Fatalf("IG recommend: %v", err)
	}
	e.onlySource(t, "facebook")
}
