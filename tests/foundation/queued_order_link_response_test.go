// Purpose: reject a stale buyer URL when a real checkout reminder supersedes Place's link before A16 DM planning.
// Depends on: real checkout/claims/merchant HTTP pipeline, inbox planners, River and HPKE; lbSetup's signed Meta intake.
// Used by: PR32 thread 4256912975 focused REAL_PG regression; loopback Graph remains paused (Meta LIVE NOT_RUN).
// Invariants: I01/I02/I04/I06/I11; exactly one order/reminder/secret/job, no dead response URL or extra work on replay.
package foundation_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/httpapi"
	"livecommerce/internal/inbox"
	socialmeta "livecommerce/internal/integrations/meta"
	"livecommerce/internal/merchanttools"
	"livecommerce/internal/msgtemplates"
	"livecommerce/internal/platform"
)

// qlrBeforeProbe delays only the first empty-link probe. Both the competing reminder and every planner call are real.
type qlrBeforeProbe struct {
	delegate *inbox.Service
	before   func(context.Context, string)
}

func (p *qlrBeforeProbe) PlanOrderPayLink(ctx context.Context, tx pgx.Tx, scope platform.Scope, key, conv, order, link string) (inbox.SendOutput, error) {
	if link == "" && p.before != nil {
		before := p.before
		p.before = nil
		before(ctx, order)
	}
	return p.delegate.PlanOrderPayLink(ctx, tx, scope, key, conv, order, link)
}

func TestQueuedOrderLinkForBuyerResponseAfterReminder(t *testing.T) {
	lb, e := lbSetup(t), lbuNew(t)
	ctx := context.Background()
	e.grantCreator("integration:execute")
	// lbuNew owns a fresh trade store, unlike lbSetup. Register a separate synthetic binding into that store explicitly;
	// reuse only lb's real signed consumer, inbox planner, River and loopback worker. No operation/order is owner-inserted.
	meta := *lb.mciEnv
	meta.h, meta.origin = e.h, e.origin
	meta.pageAsset = miAsset()
	meta.pageBinding = miBinding(t, meta.page, meta.pageAsset, "facebook", e.tenant(), e.store(), e.p.f.principalA)
	meta.registerToken(t, "facebook", meta.pageBinding, meta.pageAsset, []string{"pages_messaging", "pages_manage_engagement"}, meta.pageToken)
	miRoute(t, meta.page, meta.pageAsset, e.tenant(), e.store(), meta.pageBinding)
	mustExec(t, e.p.f.owner, `INSERT INTO integration.meta_connections(tenant_id,store_id,page_id,page_name,fb_binding,scopes,status,connected_by,route_expires_at)
		VALUES($1,$2,$3,'PR32 queued-link response MOCK Page',$4,ARRAY['pages_messaging','pages_manage_engagement'],'active',$5,clock_timestamp()+interval '1 day')`,
		e.tenant(), e.store(), meta.pageAsset, meta.pageBinding, e.p.f.principalA)
	mustExec(t, e.p.f.owner, `INSERT INTO integration.binding_capabilities(tenant_id,store_id,page_id,binding_id,provider,capability,state,reason,evidence,checked_at)
		VALUES($1,$2,$3,$4,'facebook','dm_session','ok','fixture_ok','MOCK',clock_timestamp())`, e.tenant(), e.store(), meta.pageAsset, meta.pageBinding)
	meta.session, meta.offer = e.session("A1", ltgLive, 5)
	meta.sku, meta.postID = e.money, meta.pageAsset+"_"+mciDigits(10)
	meta.srcFB = meta.mustSource(t, "page", meta.pageAsset, meta.postID, false)
	t.Cleanup(func() {
		for _, query := range []string{
			`UPDATE river.river_job SET queue='qlr_hold' WHERE kind='external_operation_v1' AND state IN ('available','retryable','scheduled')
			 AND args->>'operation_id' IN (SELECT id::text FROM integration.operations WHERE tenant_id=$1 AND store_id=$2 AND action='meta.dm_send')`,
			`DELETE FROM claims.meta_intake WHERE tenant_id=$1 AND store_id=$2 AND session_id=$3`,
			`DELETE FROM live.claim_sources WHERE tenant_id=$1 AND store_id=$2 AND session_id=$3`,
			`DELETE FROM live.claim_window_intervals WHERE tenant_id=$1 AND store_id=$2 AND session_id=$3`,
		} {
			args := []any{e.tenant(), e.store()}
			if strings.Contains(query, "$3") {
				args = append(args, meta.session)
			}
			mustExec(t, e.p.f.owner, query, args...)
		}
	})
	bridge := &lbEnv{mciEnv: &meta, svc: lb.svc, jobs: lb.jobs, g: lb.g}
	psid := mciDigits(15)
	// Admit a real signed message, then resolve it in this trade store (lbEnv.postDM assumes the shared fixture store).
	raw := []byte(fmt.Sprintf(`{"object":"page","entry":[{"id":%q,"time":123,"messaging":[{"sender":{"id":%q},"recipient":{"id":%q},"timestamp":%d,"message":{"mid":%q,"text":"synthetic buyer asks for checkout"}}]}]}`,
		meta.pageAsset, psid, meta.pageAsset, time.Now().UnixMilli(), "m."+randomUUID()))
	ev := mcPost(t, meta.page, meta.pageAsset, raw)
	mcAwait(t, meta.page, ev)
	var conv string
	if err := e.p.f.owner.QueryRow(ctx, `SELECT id::text FROM social.conversations WHERE tenant_id=$1 AND store_id=$2 AND object='page' AND asset_id=$3 AND peer_key=$4`,
		e.tenant(), e.store(), meta.pageAsset, socialmeta.SocialPeerKey(miApp, "page", meta.pageAsset, psid)).Scan(&conv); err != nil {
		t.Fatal("conversation of the real signed message in the trade store", err)
	}
	claim := meta.planReply(t, false, psid, "A1+2")
	if claim.bundleID == "" || claim.op != "" {
		t.Fatal("signed comment must produce a bundle without planning a private reply")
	}
	bundle := claim.bundleID
	bridge.linkPeer(t, bundle, conv, randomUUID()) // disclosed peer fixture: the exact relation a completed private reply would write
	mustExec(t, e.p.f.owner, `UPDATE claims.bundles SET created_at=clock_timestamp()-interval '20 minutes' WHERE id=$1`, bundle)
	e.h.closeWindow(t, meta.session)
	// Reuse the real planner with a test-owned HPKE pair so the sealed pending reminder can be inspected without dispatching it.
	sealKeys, open := mcnPageRing(t)
	lb.svc.EnableSend(sealKeys, lb.jobs, msgtemplates.NewService())
	reminders, err := merchanttools.NewCheckoutReminders(e.p.f.runtime, e.manual, lb.svc)
	if err != nil {
		t.Fatal(err)
	}
	var order string
	var originalHash []byte
	probes := 0
	planner := &qlrBeforeProbe{delegate: lb.svc, before: func(callCtx context.Context, committedOrder string) {
		probes++
		order = committedOrder
		// Positive seam sentinel: Place and the upper bundle receipt have committed before the real reminder starts.
		if n := e.count(`SELECT count(*) FROM checkout.orders o JOIN inbox.order_for_buyer f ON f.order_id=o.id
			WHERE o.id=$1 AND o.source='merchant_manual' AND o.commercial_state='AWAITING_PAYMENT' AND f.bundle_id=$2 AND f.state='placed'`, order, bundle); n != 1 {
			t.Fatal("probe arrived before the real order and placed bundle committed")
		}
		if err := e.p.f.owner.QueryRow(callCtx, `SELECT token_hash FROM checkout.order_links WHERE order_id=$1 AND redeemed_at IS NULL`, order).Scan(&originalHash); err != nil {
			t.Fatal("read initial committed link hash", err)
		}
		reminded, err := reminders.Trigger(callCtx, e.token(), e.store(), lbKey(), meta.session, bundle)
		mustExec(t, e.p.f.owner, `UPDATE river.river_job SET queue='cr_hold' WHERE kind='external_operation_v1' AND state='available' AND queue='default'
			AND args->>'operation_id' IN (SELECT id::text FROM integration.operations WHERE tenant_id=$1 AND store_id=$2 AND request->>'message_type'='checkout_reminder')`, e.tenant(), e.store())
		if err != nil || reminded.Queued != 1 || reminded.Refused != 0 || reminded.AlreadyReminded != 0 || len(reminded.Results) != 1 || reminded.Results[0].BundleID != bundle || reminded.Results[0].Outcome != "queued" {
			t.Fatalf("real competing reminder: queued=%d refused=%d already=%d rows=%d err=%v", reminded.Queued, reminded.Refused, reminded.AlreadyReminded, len(reminded.Results), err)
		}
	}}
	e.fb, err = merchanttools.NewForBuyer(e.manual, planner)
	if err != nil {
		t.Fatal(err)
	}
	e.handler = httpapi.NewHandler(e.p.f.runtime, httpapi.Options{CVS: e.cvs, ManualOrders: e.manual, ForBuyer: e.fb, Inbox: lb.svc})
	key := lbKey()
	body := e.body(1, []string{bundle}, conv, true)
	code, out := e.post(e.token(), key, body)
	if code != 201 || probes != 1 || lbuStr(out, "order_id") != order {
		t.Fatalf("fresh placement: status=%d probes=%d same_order=%v", code, probes, lbuStr(out, "order_id") == order)
	}
	assertResponse := func(status int, result map[string]any) {
		t.Helper()
		send, _ := result["send"].(map[string]any)
		if send["state"] != "not_sent" || send["reason"] != "link_unavailable" {
			t.Errorf("status=%d send state=%v reason=%v, want not_sent/link_unavailable", status, send["state"], send["reason"])
		}
		if link, exists := result["buyer_link"]; !exists || link != nil {
			t.Error("buyer_link must be explicitly null after the competing reminder supersedes the original link")
		}
	}
	assertResponse(code, out)
	assertFacts := func() {
		t.Helper()
		queries := map[string]string{
			"orders":             `SELECT count(*) FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2`,
			"links":              `SELECT count(*) FROM checkout.order_links WHERE tenant_id=$1 AND store_id=$2`,
			"active_links":       `SELECT count(*) FROM checkout.order_links WHERE tenant_id=$1 AND store_id=$2 AND redeemed_at IS NULL AND expires_at>clock_timestamp()`,
			"placed":             `SELECT count(*) FROM inbox.order_for_buyer WHERE tenant_id=$1 AND store_id=$2 AND state='placed'`,
			"origins":            `SELECT count(*) FROM claims.order_origins WHERE tenant_id=$1 AND store_id=$2`,
			"manual_audit":       `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action='order.manual_created'`,
			"for_buyer_audit":    `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action='order.for_buyer_created'`,
			"grant_audit":        `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action='claims.merchant_origin_granted'`,
			"regeneration_audit": `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action='order.manual_link_regenerated'`,
			"dm_operations":      `SELECT count(*) FROM integration.operations WHERE tenant_id=$1 AND store_id=$2 AND action='meta.dm_send'`,
		}
		for label, want := range map[string]int{"orders": 1, "links": 2, "active_links": 1, "placed": 1, "origins": 1, "manual_audit": 1, "for_buyer_audit": 1, "grant_audit": 1, "regeneration_audit": 0, "dm_operations": 1} {
			if got := e.count(queries[label], e.tenant(), e.store()); got != want {
				t.Errorf("%s=%d want=%d", label, got, want)
			}
		}
		for _, operation := range []string{"merchanttools.order.manual", "merchanttools.order.for_buyer", "merchanttools.order.for_buyer.completed"} {
			if n := e.count(`SELECT count(*) FROM ops.command_results WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND operation=$4 AND idempotency_key=$5`, e.tenant(), e.store(), e.p.f.principalA, operation, key); n != 1 {
				t.Errorf("receipt %s=%d want=1", operation, n)
			}
		}
		if qty, rows := e.held(bundle, meta.offer.ID); qty != 1 || rows != 1 {
			t.Errorf("claim finish held=%d rows=%d want=1/1", qty, rows)
		}
		if n := e.count(`SELECT count(*) FROM checkout.order_links WHERE order_id=$1 AND token_hash=$2 AND redeemed_at IS NOT NULL`, order, originalHash); n != 1 {
			t.Error("competing reminder must have invalidated the original committed URL")
		}
		var op string
		if err := e.p.f.owner.QueryRow(ctx, `SELECT id::text FROM integration.operations WHERE tenant_id=$1 AND store_id=$2
			AND action='meta.dm_send' AND request->>'message_type'='checkout_reminder' AND request->>'session_id'=$3`, e.tenant(), e.store(), meta.session).Scan(&op); err != nil {
			t.Fatal("real checkout reminder operation in the trade store", err)
		}
		if n := e.count(`SELECT count(*) FROM integration.operations o JOIN river.river_job j ON j.id=o.job_id
			WHERE o.id=$1 AND o.state='READY' AND o.action='meta.dm_send' AND o.request->>'conversation_id'=$2
			AND o.request->>'bundle_id'=$3 AND o.request->>'reminder_state'='awaiting_payment' AND j.state='available' AND j.queue='cr_hold'`, op, conv, bundle); n != 1 || bridge.secretCount(t, op) != 1 {
			t.Error("reminder must retain one real READY operation, held River job and sealed secret")
		}
		var enc, sealed []byte
		if err := e.p.f.owner.QueryRow(ctx, `SELECT enc,sealed FROM inbox.send_secrets WHERE operation_id=$1`, op).Scan(&enc, &sealed); err != nil {
			t.Fatal(err)
		}
		plain, err := open.OpenSend(e.tenant(), e.store(), op, enc, sealed)
		if err != nil {
			t.Fatal("open test-owned reminder seal", err)
		}
		var message struct {
			Text string `json:"text"`
		}
		err = json.Unmarshal(plain, &message)
		clear(plain)
		if err != nil {
			t.Fatal("decode sealed reminder", err)
		}
		match := regexp.MustCompile(`/order-link#o=` + regexp.QuoteMeta(order) + `&t=([A-Za-z0-9_-]+)`).FindStringSubmatch(message.Text)
		message.Text = ""
		if len(match) != 2 {
			t.Fatal("sealed reminder does not contain the committed order link")
		}
		hash := sha256.Sum256([]byte(match[1]))
		match = nil
		if n := e.count(`SELECT count(*) FROM checkout.order_links WHERE order_id=$1 AND token_hash=$2 AND redeemed_at IS NULL AND expires_at>clock_timestamp()`, order, hash[:]); n != 1 {
			t.Error("sealed reminder URL must still redeem the current order")
		}
		if lb.g.count() != 0 {
			t.Error("paused dispatcher must not call Graph")
		}
	}
	assertFacts()
	replayCode, replayed := e.post(e.token(), key, body)
	if replayCode != 200 || lbuStr(replayed, "order_id") != order || probes != 1 {
		t.Fatalf("same-key replay: status=%d same_order=%v probe_callbacks=%d", replayCode, lbuStr(replayed, "order_id") == order, probes)
	}
	assertResponse(replayCode, replayed)
	assertFacts()
}
