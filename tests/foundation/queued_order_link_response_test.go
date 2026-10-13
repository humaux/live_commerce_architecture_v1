// Purpose: reject a stale buyer URL when a real concurrent pay-link plan supersedes Place's link before A16 DM planning.
// Depends on: real checkout/claims/merchant HTTP pipeline, inbox planners, River and HPKE; lbSetup's signed Meta intake.
// Used by: PR32 thread 4256912975 focused REAL_PG regression; loopback Graph remains paused (Meta LIVE NOT_RUN).
// Invariants: I01/I02/I04/I06/I11; exactly one order/DM/secret/job, no dead response URL or extra work on replay.
// This representative race uses the allowed bank-transfer manual flow; real reminder compatibility has separate batch/target tests.
// Transaction seams return errors; never call testing.Fatal/Goexit inside WithScope, so failures release the transaction.
package foundation_test

import (
	"bytes"
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

// qlrBeforeProbe delays only the first empty-link probe. Both the competing plan and every planner call are real.
type qlrBeforeProbe struct {
	delegate  *inbox.Service
	before    func(context.Context, string) error
	beforeErr error
}

func (p *qlrBeforeProbe) PlanOrderPayLink(ctx context.Context, tx pgx.Tx, scope platform.Scope, key, conv, order, link string) (inbox.SendOutput, error) {
	if link == "" && p.before != nil {
		before := p.before
		p.before = nil
		if p.beforeErr = before(ctx, order); p.beforeErr != nil {
			return inbox.SendOutput{}, p.beforeErr
		}
	}
	return p.delegate.PlanOrderPayLink(ctx, tx, scope, key, conv, order, link)
}

func TestQueuedOrderLinkForBuyerResponseAfterConcurrentPlan(t *testing.T) {
	lb, e := lbSetup(t), lbuNew(t)
	ctx := context.Background()
	e.grantCreator("integration:execute")
	// lbuNew -> psSetup already commits one real storefront hold. Account for that exact fixture order without hiding new orders.
	baseOrders := e.count(`SELECT count(*) FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store())
	if baseOrders != 1 || e.count(`SELECT count(*) FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2 AND id=$3 AND source='storefront'`, e.tenant(), e.store(), e.p.hold.OrderID) != 1 || e.orders() != 0 {
		t.Fatal("fixture must contain only its known storefront hold and no merchant manual order")
	}
	// Ordinary claim-checkout origins are a separate ledger: A16 attests its buyer via a quote-bound merchant-origin grant.
	baseOrigins := e.count(`SELECT count(*) FROM claims.order_origins WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store())
	if baseOrigins != 0 {
		t.Fatal("fixture must have no ordinary claim-checkout origin")
	}
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
	// Reuse the real planner with a test-owned HPKE pair so the sealed pending DM can be inspected without dispatching it.
	sealKeys, open := mcnPageRing(t)
	lb.svc.EnableSend(sealKeys, lb.jobs, msgtemplates.NewService())
	key, regenerationKey, concurrentDMKey := lbKey(), lbKey(), lbKey()
	if key == regenerationKey || key == concurrentDMKey || regenerationKey == concurrentDMKey {
		t.Fatal("placement, public regeneration and competing DM must use distinct keys")
	}
	var order string
	var originalHash, regeneratedHash []byte
	var concurrentPlan inbox.SendOutput
	probes := 0
	planner := &qlrBeforeProbe{delegate: lb.svc, before: func(callCtx context.Context, committedOrder string) error {
		callCtx, cancel := context.WithTimeout(callCtx, 10*time.Second)
		defer cancel()
		probes++
		order = committedOrder
		// Positive seam sentinel: the allowed bank-transfer Place and upper bundle receipt have committed before interposition.
		var n int
		if err := e.p.f.owner.QueryRow(callCtx, `SELECT count(*) FROM checkout.orders o JOIN inbox.order_for_buyer f ON f.order_id=o.id
			WHERE o.id=$1 AND o.source='merchant_manual' AND o.commercial_state='AWAITING_TRANSFER' AND f.bundle_id=$2 AND f.state='placed'
			AND o.tenant_id=$3 AND o.store_id=$4 AND f.tenant_id=$3 AND f.store_id=$4`, order, bundle, e.tenant(), e.store()).Scan(&n); err != nil {
			return fmt.Errorf("read committed order/bundle seam sentinel: %w", err)
		}
		if n != 1 {
			return fmt.Errorf("probe arrived before the real order and placed bundle committed: rows=%d", n)
		}
		if err := e.p.f.owner.QueryRow(callCtx, `SELECT token_hash FROM checkout.order_links WHERE order_id=$1 AND redeemed_at IS NULL`, order).Scan(&originalHash); err != nil {
			return fmt.Errorf("read initial committed link hash: %w", err)
		}
		regenerated, replayed, err := e.manual.RegenerateLink(callCtx, e.token(), e.store(), regenerationKey, merchanttools.ManualRegenerateInput{OrderID: order, Locale: "zh-TW"})
		if err != nil {
			return fmt.Errorf("real public link regeneration: %w", err)
		}
		if replayed || regenerated.OrderID != order || regenerated.Source != "merchant_manual" || regenerated.BuyerLink == "" {
			return fmt.Errorf("real public regeneration must issue one new link for the committed manual order: replayed=%v same_order=%v source=%s has_link=%v", replayed, regenerated.OrderID == order, regenerated.Source, regenerated.BuyerLink != "")
		}
		if err := e.p.f.owner.QueryRow(callCtx, `SELECT token_hash FROM checkout.order_links WHERE order_id=$1 AND redeemed_at IS NULL`, order).Scan(&regeneratedHash); err != nil {
			return fmt.Errorf("read publicly regenerated link hash: %w", err)
		}
		if bytes.Equal(originalHash, regeneratedHash) {
			return fmt.Errorf("public regeneration did not replace the original link")
		}
		if err := platform.WithScope(callCtx, e.p.f.runtime, e.token(), e.store(), "inbox:reply", func(tx pgx.Tx, scope platform.Scope) error {
			var err error
			concurrentPlan, err = lb.svc.PlanOrderPayLink(callCtx, tx, scope, concurrentDMKey, conv, order, regenerated.BuyerLink)
			return err
		}); err != nil {
			return fmt.Errorf("real competing payment DM plan: %w", err)
		}
		if concurrentPlan.OperationID == "" || concurrentPlan.OutboundID == "" || concurrentPlan.SendState != "queued" {
			return fmt.Errorf("real competing payment DM must return its queued operation and outbound: has_operation=%v has_outbound=%v state=%s", concurrentPlan.OperationID != "", concurrentPlan.OutboundID != "", concurrentPlan.SendState)
		}
		parked, err := e.p.f.owner.Exec(callCtx, `UPDATE river.river_job SET queue='qlr_hold' WHERE kind='external_operation_v1' AND state='available' AND queue='default'
			AND args->>'operation_id'=$1 AND EXISTS(SELECT 1 FROM integration.operations WHERE id::text=$1 AND tenant_id=$2 AND store_id=$3)`, concurrentPlan.OperationID, e.tenant(), e.store())
		if err != nil {
			return fmt.Errorf("hold real competing payment DM job: %w", err)
		}
		if parked.RowsAffected() != 1 {
			return fmt.Errorf("hold exactly the returned payment DM job: rows=%d", parked.RowsAffected())
		}
		return nil
	}}
	var err error
	e.fb, err = merchanttools.NewForBuyer(e.manual, planner)
	if err != nil {
		t.Fatal(err)
	}
	e.handler = httpapi.NewHandler(e.p.f.runtime, httpapi.Options{CVS: e.cvs, ManualOrders: e.manual, ForBuyer: e.fb, Inbox: lb.svc})
	body := e.body(1, []string{bundle}, conv, true)
	code, out := e.post(e.token(), key, body)
	if planner.beforeErr != nil {
		t.Fatalf("real competing payment DM interposition: %v", planner.beforeErr)
	}
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
			t.Error("buyer_link must be explicitly null after the competing payment DM plan supersedes the original link")
		}
	}
	assertResponse(code, out)
	assertFacts := func() {
		t.Helper()
		if n := e.count(`SELECT count(*) FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store()); n != baseOrders+1 {
			t.Errorf("store orders=%d want baseline %d + exactly one new order", n, baseOrders)
		}
		if n := e.count(`SELECT count(*) FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2 AND source='merchant_manual'`, e.tenant(), e.store()); n != 1 {
			t.Errorf("merchant manual orders=%d want=1", n)
		}
		if n := e.count(`SELECT count(*) FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2 AND id=$3 AND source='storefront'`, e.tenant(), e.store(), e.p.hold.OrderID); n != 1 {
			t.Error("the exact baseline storefront hold must remain unchanged")
		}
		if n := e.count(`SELECT count(*) FROM claims.order_origins WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store()); n != baseOrigins {
			t.Errorf("ordinary claim-checkout origins=%d want unchanged baseline=%d", n, baseOrigins)
		}
		// Prove the actual A16 origin, rather than expecting its independently owned claim to appear in the ordinary checkout ledger.
		if n := e.count(`SELECT count(*) FROM checkout.orders o
			JOIN inbox.order_for_buyer f ON f.tenant_id=o.tenant_id AND f.store_id=o.store_id AND f.order_id=o.id AND f.buyer_id=o.owner_id
			JOIN claims.merchant_origin_grants g ON g.tenant_id=f.tenant_id AND g.store_id=f.store_id AND g.request_id=f.request_id
			 AND g.bundle_id=f.bundle_id AND g.buyer_id=f.buyer_id AND g.quote_id=o.quote_id AND g.consumed_at IS NOT NULL
			JOIN claims.live_price_uses u ON u.tenant_id=o.tenant_id AND u.store_id=o.store_id AND u.order_id=o.id AND u.bundle_id=f.bundle_id
			WHERE o.tenant_id=$1 AND o.store_id=$2 AND o.id=$3 AND o.source='merchant_manual' AND o.commercial_state='AWAITING_TRANSFER'
			AND f.bundle_id=$4 AND f.conversation_id=$5 AND f.state='placed' AND f.principal_id=$6 AND g.principal_id=$6
			AND u.offer_id=$7 AND u.quantity=1 AND u.unit_price_minor=$8`, e.tenant(), e.store(), order, bundle, conv, e.p.f.principalA, meta.offer.ID, ltgLive); n != 1 {
			t.Errorf("exact committed A16 order/bundle/conversation/consumed quote grant/live-price origin=%d want=1", n)
		}
		queries := map[string]string{
			"links":              `SELECT count(*) FROM checkout.order_links WHERE tenant_id=$1 AND store_id=$2`,
			"active_links":       `SELECT count(*) FROM checkout.order_links WHERE tenant_id=$1 AND store_id=$2 AND redeemed_at IS NULL AND expires_at>clock_timestamp()`,
			"placed":             `SELECT count(*) FROM inbox.order_for_buyer WHERE tenant_id=$1 AND store_id=$2 AND state='placed'`,
			"manual_audit":       `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action='order.manual_created'`,
			"for_buyer_audit":    `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action='order.for_buyer_created'`,
			"grant_audit":        `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action='claims.merchant_origin_granted'`,
			"regeneration_audit": `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action='order.manual_link_regenerated'`,
			"dm_operations":      `SELECT count(*) FROM integration.operations WHERE tenant_id=$1 AND store_id=$2 AND action='meta.dm_send'`,
		}
		for label, want := range map[string]int{"links": 2, "active_links": 1, "placed": 1, "manual_audit": 1, "for_buyer_audit": 1, "grant_audit": 1, "regeneration_audit": 1, "dm_operations": 1} {
			if got := e.count(queries[label], e.tenant(), e.store()); got != want {
				t.Errorf("%s=%d want=%d", label, got, want)
			}
		}
		for _, operation := range []string{"merchanttools.order.manual", "merchanttools.order.for_buyer", "merchanttools.order.for_buyer.completed"} {
			if n := e.count(`SELECT count(*) FROM ops.command_results WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND operation=$4 AND idempotency_key=$5`, e.tenant(), e.store(), e.p.f.principalA, operation, key); n != 1 {
				t.Errorf("receipt %s=%d want=1", operation, n)
			}
		}
		for operation, receiptKey := range map[string]string{"merchanttools.order.relink": regenerationKey, "inbox.dm.order_pay_link": concurrentDMKey} {
			if n := e.count(`SELECT count(*) FROM ops.command_results WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND operation=$4 AND idempotency_key=$5`, e.tenant(), e.store(), e.p.f.principalA, operation, receiptKey); n != 1 {
				t.Errorf("competing receipt %s=%d want=1", operation, n)
			}
		}
		if qty, rows := e.held(bundle, meta.offer.ID); qty != 1 || rows != 1 {
			t.Errorf("claim finish held=%d rows=%d want=1/1", qty, rows)
		}
		if n := e.count(`SELECT count(*) FROM checkout.order_links WHERE order_id=$1 AND token_hash=$2 AND redeemed_at IS NOT NULL`, order, originalHash); n != 1 {
			t.Error("public regeneration must have invalidated the original committed URL")
		}
		op := concurrentPlan.OperationID
		if n := e.count(`SELECT count(*) FROM integration.operations o JOIN river.river_job j ON j.id=o.job_id
			WHERE o.id=$1 AND o.state='READY' AND o.action='meta.dm_send' AND o.request->>'conversation_id'=$2
			AND o.request->>'order_id'=$3 AND o.request->>'message_type'='dm' AND o.request->>'origin'='human'
			AND o.request->>'template_id'='order-pay-link/v1' AND o.request->>'template_version'='1' AND o.request->>'outbound_id'=$4
			AND o.tenant_id=$5 AND o.store_id=$6 AND j.state='available' AND j.queue='qlr_hold'`, op, conv, order, concurrentPlan.OutboundID, e.tenant(), e.store()); n != 1 || bridge.secretCount(t, op) != 1 {
			t.Error("payment DM must retain one real READY operation, held River job and sealed secret")
		}
		var enc, sealed []byte
		if err := e.p.f.owner.QueryRow(ctx, `SELECT enc,sealed FROM inbox.send_secrets WHERE operation_id=$1`, op).Scan(&enc, &sealed); err != nil {
			t.Fatal(err)
		}
		plain, err := open.OpenSend(e.tenant(), e.store(), op, enc, sealed)
		if err != nil {
			t.Fatal("open test-owned payment DM seal", err)
		}
		var message struct {
			Text string `json:"text"`
		}
		err = json.Unmarshal(plain, &message)
		clear(plain)
		if err != nil {
			t.Fatal("decode sealed payment DM", err)
		}
		match := regexp.MustCompile(`/order-link#o=` + regexp.QuoteMeta(order) + `&t=([A-Za-z0-9_-]+)`).FindStringSubmatch(message.Text)
		message.Text = ""
		if len(match) != 2 {
			t.Fatal("sealed payment DM does not contain the committed order link")
		}
		hash := sha256.Sum256([]byte(match[1]))
		match = nil
		if !bytes.Equal(hash[:], regeneratedHash) || bytes.Equal(hash[:], originalHash) {
			t.Error("sealed payment DM must carry exactly the publicly regenerated link")
		}
		if n := e.count(`SELECT count(*) FROM checkout.order_links WHERE order_id=$1 AND token_hash=$2 AND redeemed_at IS NULL AND expires_at>clock_timestamp()`, order, hash[:]); n != 1 {
			t.Error("sealed payment DM URL must still redeem the current order")
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
