// Purpose: REAL_PG acceptance of the packaged opt-in OA1 claims-worker composition.
// Depends on: isolated process fixtures, signed ingress, real runtime C1 and native C2 tables.
// Used by: parent-owned OA1 CI; no mocked registration or prewritten C2 artifacts.
// Invariants: disabled worker keeps its default lane but leaves claims untouched;
// enabled worker commits the entire READY write set with real custody and no HTTP dispatch.
package foundation_test

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"livecommerce/internal/integrations/meta"
	"livecommerce/internal/orderattempt"
)

// Unlike the shared notification fixture, every ingress/consumer pool here is
// created on the same isolated database that the packaged worker will consume.
func oaEnabledProcessConversation(t *testing.T, e *lbuEnv) (miTest, string, string) {
	t.Helper()
	m := mrSetup(t, e.p.f)
	asset, peer := miAsset(), miAsset()
	binding := miBinding(t, m, asset, "facebook", e.tenant(), e.store(), e.p.f.principalA)
	miRoute(t, m, asset, e.tenant(), e.store(), binding)
	mustExec(t, m.f.owner, `INSERT INTO integration.meta_connections(tenant_id,store_id,page_id,page_name,fb_binding,scopes,status,connected_by,route_expires_at)
	 VALUES($1,$2,$3,'synthetic OA process page',$4,ARRAY['pages_messaging'],'active',$5,clock_timestamp()+interval '1 day')`, e.tenant(), e.store(), asset, binding, e.p.f.principalA)
	client, err := meta.NewConsumerClient(context.Background(), miPool(t, m.f, "commerce_meta_worker"), mcConsumer(t, m), mcKeys(t, m), 1)
	if err != nil {
		t.Fatal("isolated signed-ingress consumer", err)
	}
	if err = client.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		if err := client.StopAndCancel(ctx); err != nil {
			t.Errorf("signed-ingress consumer stop: %v", err)
		}
	})
	raw := []byte(fmt.Sprintf(`{"object":"page","entry":[{"id":%q,"messaging":[{"sender":{"id":%q},"recipient":{"id":%q},"timestamp":%d,"message":{"mid":%q,"text":"synthetic OA process inbound"}}]}]}`, asset, peer, asset, time.Now().UnixMilli(), "m."+randomUUID()))
	mcAwait(t, m, mcPost(t, m, asset, raw))
	var conversation string
	if err = m.f.owner.QueryRow(context.Background(), `SELECT id::text FROM social.conversations WHERE tenant_id=$1 AND store_id=$2 AND app_id=$3 AND object='page' AND asset_id=$4 AND peer_key=$5`, e.tenant(), e.store(), miApp, asset, meta.SocialPeerKey(miApp, "page", asset, peer)).Scan(&conversation); err != nil {
		t.Fatal("isolated signed conversation", err)
	}
	return m, conversation, binding
}

func oaEnabledProcessWait(t *testing.T, e *lbuEnv, p *mrProcess, predicate string, args ...any) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		var ready bool
		if err := e.p.f.owner.QueryRow(ctx, predicate, args...).Scan(&ready); err != nil {
			t.Fatal("bounded native process observation", err)
		}
		if ready {
			return
		}
		select {
		case err := <-p.done:
			p.exited = true
			t.Fatalf("worker exited before native witness: %v (log %s)", err, p.logPath)
		case <-ctx.Done():
			t.Fatal("native process witness exceeded 15 seconds")
		case <-ticker.C:
		}
	}
}

func TestOrderAttemptWorkerEnabledProcessNativeReady(t *testing.T) {
	e := oaExpiryIsolated(t)
	e.grantCreator("inbox:read", "inbox:reply", "integration:read", "integration:manage", "customers:read")
	// The expiry-only isolated fixture does not publish a storefront. READY
	// requires the same native verified origin as the existing notification flow.
	e.origin = "https://oa-worker-" + strings.ReplaceAll(randomUUID(), "-", "") + ".example"
	bhPublish(t, e.p.bcHarness, e.origin, e.tenant(), e.store())
	var published bool
	if err := e.p.f.owner.QueryRow(context.Background(), `SELECT EXISTS(SELECT 1
	 FROM control.storefront_domains d JOIN control.storefront_publications p USING(tenant_id,store_id)
	 WHERE d.tenant_id=$1 AND d.store_id=$2 AND d.origin=$3 AND d.state='ACTIVE' AND p.published
	 AND d.ownership_verified_at<=clock_timestamp() AND d.tls_verified_at<=clock_timestamp()
	 AND d.valid_until>clock_timestamp())`, e.tenant(), e.store(), e.origin).Scan(&published); err != nil || !published {
		t.Fatal("isolated READY prerequisite: current published origin", err)
	}
	m, conversation, binding := oaEnabledProcessConversation(t, e)
	selection := oaBeginSelection(t, e, "bank_transfer", "duplicate-origins")
	selection["for"] = map[string]any{"bundle_ids": []string{}, "conversation_id": conversation}
	selection["send_payment_link"] = true
	a := oaAllocationAdmit(t, e, selection)
	plan, _ := oaMaterialPlan(t, e, oaServiceKeys(t), a.AttemptID)
	w := crNewWorkerEnv(t, e.p.f)
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		rw.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	w.env = append(w.env, "COMMERCE_META_GRAPH_BASE_URL="+server.URL)
	private, err := ecdh.X25519().NewPrivateKey(randomBytes(32))
	if err != nil {
		t.Fatal(err)
	}
	b64 := base64.StdEncoding.EncodeToString
	planKey, payKey, payloadKey := b64(bytes.Repeat([]byte{7}, 32)), b64(bytes.Repeat([]byte{0x41}, 32)), b64(m.key)
	custody := []string{
		"LC_ORDER_ATTEMPT_PLAN_ACTIVE_KEY=fixture_v1", `LC_ORDER_ATTEMPT_PLAN_KEYS={"fixture_v1":"` + planKey + `"}`,
		"LC_ORDER_ATTEMPT_PAY_ENTRY_ACTIVE_KEY=v1", `LC_ORDER_ATTEMPT_PAY_ENTRY_KEYS={"v1":"` + payKey + `"}`,
		"COMMERCE_META_PAYLOAD_ACTIVE_KEY_ID=" + miKeyID, `COMMERCE_META_PAYLOAD_KEYS_JSON={"keys":[{"id":"` + miKeyID + `","key_base64":"` + payloadKey + `"}]}`,
		"COMMERCE_META_PAGE_HPKE_ACTIVE_KEY_ID=oa-process-v1", `COMMERCE_META_PAGE_HPKE_PUBLIC_KEYS_JSON={"keys":[{"id":"oa-process-v1","public_key_base64":"` + b64(private.PublicKey().Bytes()) + `"}]}`,
	}
	binary := mrBuild(t, "../../cmd/claims-worker", "oa-enabled-worker")
	if !t.Run("disabled-keeps-default-lane-without-consuming-claims", func(t *testing.T) {
		env := append(append([]string(nil), w.env...), "LC_ORDER_ATTEMPT_ENABLED=0")
		for _, entry := range custody {
			name, _, _ := strings.Cut(entry, "=")
			env = append(env, name+"=synthetic-invalid-disabled-custody")
		}
		p := crLaunch(t, binary, env)
		// A completed ordinary job proves the actual River process is working;
		// absence of OA execution is not inferred from merely sleeping.
		oaEnabledProcessWait(t, e, p, `SELECT EXISTS(SELECT 1 FROM river.river_job WHERE kind='claims_retention_v1' AND queue='default' AND state='completed')`)
		var untouched bool
		err := e.p.f.owner.QueryRow(context.Background(), `SELECT a.state='PENDING' AND a.receipt IS NULL
		 AND j.kind='order_attempt_v1' AND j.queue='claims' AND j.state='available' AND j.attempt=0
		 AND NOT EXISTS(SELECT 1 FROM river.river_job WHERE kind='order_attempt_sweep_v1')
		 FROM ops.order_attempts a JOIN river.river_job j ON j.args->>'attempt_id'=a.attempt_id::text WHERE a.attempt_id=$1`, a.AttemptID).Scan(&untouched)
		if err != nil || !untouched {
			t.Fatalf("disabled OA consumed claims or scheduled sweep: %v", err)
		}
		oaExecutorNoEffects(t, e, a)
		mrStop(t, p, syscall.SIGTERM, true)
		mrLogNoSecrets(t, p, planKey, payKey, payloadKey, w.jobDSNSecret)
	}) {
		t.Fatal("disabled process positive prerequisite failed")
	}
	// Pause the ordinary dispatch lane before starting the next process. River
	// reads persisted pause state at startup; the claims producer stays enabled.
	mustExec(t, e.p.f.owner, `UPDATE river.river_queue SET paused_at=clock_timestamp() WHERE name='default'`)
	if !t.Run("enabled-completes-native-ready", func(t *testing.T) {
		env := append(append(append([]string(nil), w.env...), custody...), "LC_ORDER_ATTEMPT_ENABLED=1")
		p := crLaunch(t, binary, env)
		oaEnabledProcessWait(t, e, p, `SELECT EXISTS(SELECT 1 FROM ops.order_attempts a JOIN river.river_job j ON j.args->>'attempt_id'=a.attempt_id::text
		 WHERE a.attempt_id=$1 AND a.state='COMMITTED' AND j.kind='order_attempt_v1' AND j.queue='claims' AND j.state='completed')`, a.AttemptID)
		oaEnabledProcessWait(t, e, p, `SELECT EXISTS(SELECT 1 FROM river.river_job WHERE kind='order_attempt_sweep_v1' AND queue='claims' AND state='completed')`)
		mrStop(t, p, syscall.SIGTERM, true)
		mrLogNoSecrets(t, p, planKey, payKey, payloadKey, w.jobDSNSecret, "synthetic OA process inbound", "synthetic@example.invalid")
	}) {
		t.Fatal("enabled native process did not complete")
	}
	if requests.Load() != 0 {
		t.Fatal("local C2 acceptance made an HTTP dispatch request")
	}
	oaEnabledProcessReadyOracle(t, e, a, plan, conversation, binding)
	t.Run("malformed-enabled-custody-fails-before-ready", func(t *testing.T) {
		env := append(append([]string(nil), w.env...), "LC_ORDER_ATTEMPT_ENABLED=1")
		for _, entry := range custody {
			if strings.HasPrefix(entry, "LC_ORDER_ATTEMPT_PAY_ENTRY_KEYS=") {
				entry = "LC_ORDER_ATTEMPT_PAY_ENTRY_KEYS=synthetic-malformed-secret"
			}
			env = append(env, entry)
		}
		p := mrLaunch(t, binary, "oa-malformed-enabled", env)
		mrFailsBeforeReady(t, p, "claims_worker_invalid_config", planKey, payKey, payloadKey, "synthetic-malformed-secret")
		if strings.Contains(crReadLog(t, p), "claims_worker_ready") || strings.Contains(crReadLog(t, p), "claims_worker_routes") {
			t.Fatal("malformed OA custody crossed startup admission")
		}
	})
}

func oaEnabledProcessReadyOracle(t *testing.T, e *lbuEnv, a orderattempt.AdmissionReceipt, plan orderattempt.FrozenPlan, conversation, binding string) {
	t.Helper()
	var receipt []byte
	if err := e.p.f.owner.QueryRow(context.Background(), `SELECT receipt FROM ops.order_attempts WHERE attempt_id=$1`, a.AttemptID).Scan(&receipt); err != nil {
		t.Fatal(err)
	}
	oaExecutorReceipt(t, orderattempt.ExecutionReceipt{State: "COMMITTED", Receipt: receipt}, a, "COMMITTED", "")
	var send struct {
		Send struct {
			State          string `json:"state"`
			Reason         string `json:"reason"`
			OperationID    string `json:"operation_id"`
			NotificationID string `json:"notification_id"`
		} `json:"send"`
	}
	if json.Unmarshal(receipt, &send) != nil || send.Send.State != "queued" || send.Send.OperationID == "" || send.Send.NotificationID == "" {
		t.Fatalf("process receipt lost READY notification identities: state=%s reason=%s operation=%t notification=%t", send.Send.State, send.Send.Reason, send.Send.OperationID != "", send.Send.NotificationID != "")
	}
	var exact bool
	err := e.p.f.owner.QueryRow(context.Background(), `SELECT a.state='COMMITTED' AND a.order_id=a.attempt_id
	 AND a.plan_ciphertext IS NULL AND a.plan_key_version IS NULL AND a.settled_at IS NOT NULL
	 AND o.create_attempt_id=a.attempt_id AND o.creator_kind='order_attempt' AND o.creator_session_id IS NULL
	 AND o.tenant_id=a.tenant_id AND o.store_id=a.store_id AND o.owner_id=a.buyer_owner_id
	 AND o.source='merchant_manual' AND o.payment_mode='bank_transfer' AND o.commercial_state='AWAITING_TRANSFER' AND o.generation=1 AND o.total_minor=$2
	 AND p.create_attempt_id=a.attempt_id AND p.owner_id=o.owner_id AND p.link_version=1 AND p.revoked_at IS NULL
	 AND p.expires_at=LEAST(o.expires_at,p.issued_at+interval '72 hours') AND p.expires_at>clock_timestamp()
	 AND octet_length(p.token_hash)=32 AND octet_length(p.credential_ciphertext)=71
	 AND r.creator_kind='order_attempt' AND r.create_attempt_id=a.attempt_id AND r.buyer_session_id IS NULL AND r.state='HELD' AND r.generation=1
	 AND expiry.kind='checkout_expiry_v1' AND expiry.args->>'order_id'=o.id::text
	 AND n.id=$3 AND n.operation_id=$4 AND n.tenant_id=a.tenant_id AND n.store_id=a.store_id AND n.owner_id=a.buyer_owner_id
	 AND n.order_id=o.id AND n.link_version=1 AND n.state='READY' AND n.void_stage='none' AND n.worker_generation=0 AND n.dispatch_authorized_at IS NULL
	 AND n.conversation_id=$5 AND n.binding_id=$6 AND x.state='READY' AND x.action='meta.dm_send' AND x.binding_id=n.binding_id AND x.generation=n.worker_generation
	 AND x.request->>'policy'='order-attempt-notification/v1' AND x.request->>'notification_id'=n.id::text AND x.request->>'order_id'=o.id::text
	 AND j.kind='external_operation_v1' AND j.queue='default' AND j.state='available' AND j.attempt=0 AND j.unique_key IS NULL
	 AND j.args=jsonb_build_object('operation_id',x.id::text,'version',1)
	 AND b.id=n.outbound_id AND b.operation_id=x.id AND b.conversation_id=n.conversation_id AND b.template_id='order-pay-link/v1' AND b.template_version=1
	 AND octet_length(b.nonce)=12 AND octet_length(b.ciphertext)>16 AND octet_length(s.enc)=32 AND octet_length(s.sealed)>16
	 FROM ops.order_attempts a JOIN checkout.orders o ON o.id=a.order_id JOIN checkout.order_pay_entries p ON p.order_id=o.id
	 JOIN inventory.reservations r ON r.id=o.id JOIN river_expiry.river_job expiry ON expiry.id=o.job_id
	 JOIN inbox.order_payment_notifications n ON n.create_attempt_id=a.attempt_id JOIN integration.operations x ON x.id=n.operation_id
	 JOIN river.river_job j ON j.id=x.job_id JOIN inbox.outbound_messages b ON b.id=n.outbound_id JOIN inbox.send_secrets s ON s.operation_id=x.id
	 WHERE a.attempt_id=$1`, a.AttemptID, plan.Calculation.TotalMinor, send.Send.NotificationID, send.Send.OperationID, conversation, binding).Scan(&exact)
	if err != nil || !exact {
		t.Fatalf("packaged worker full native READY write set: %v", err)
	}
	var counts bool
	err = e.p.f.owner.QueryRow(context.Background(), `SELECT
	 (SELECT count(*)=1 FROM checkout.orders WHERE create_attempt_id=$1) AND (SELECT count(*)=1 FROM storefront.carts WHERE create_attempt_id=$1)
	 AND (SELECT count(*)=1 FROM storefront.quotes WHERE create_attempt_id=$1) AND (SELECT count(*)=1 FROM storefront.destination_snapshots WHERE create_attempt_id=$1)
	 AND (SELECT count(*)=1 FROM inventory.reservations WHERE create_attempt_id=$1) AND (SELECT count(*)=1 FROM checkout.command_results WHERE create_attempt_id=$1)
	 AND (SELECT count(*)=1 FROM checkout.bank_transfers WHERE order_id=$1) AND (SELECT count(*)=1 FROM checkout.order_pay_entries WHERE create_attempt_id=$1)
	 AND (SELECT count(*)=1 FROM river_expiry.river_job WHERE args->>'order_id'=($1::uuid)::text)
	 AND (SELECT count(*)=1 FROM inbox.order_payment_notifications WHERE create_attempt_id=$1)
	 AND (SELECT count(*)=1 FROM integration.operations WHERE action='meta.dm_send' AND request->>'order_id'=($1::uuid)::text)
	 AND NOT EXISTS(SELECT 1 FROM ops.order_attempt_subjects WHERE attempt_id=$1)
	 AND (SELECT sum(quantity)=5 FROM inventory.reservation_lines WHERE reservation_id=$1 AND sku_id=$2)
	 AND (SELECT sum(delta_reserved)=5 FROM inventory.ledger WHERE create_attempt_id=$1 AND sku_id=$2 AND actor_kind='ORDER_ATTEMPT')
	 AND (SELECT count(*)=1 FROM ops.audit_events WHERE tenant_id=$3 AND store_id=$4 AND principal_id=$5 AND action='order.manual_created'
	 AND details=jsonb_build_object('attempt_id',$1::text,'order_id',$1::text,'intent_generation',$6::bigint))`, a.AttemptID, e.money, e.tenant(), e.store(), e.p.f.principalA, a.Generation).Scan(&counts)
	if err != nil || !counts {
		t.Fatalf("packaged worker exact effect counts and stock conservation: %v", err)
	}
	var material orderattempt.PayEntryMaterial
	var digest []byte
	if err = e.p.f.owner.QueryRow(context.Background(), `SELECT key_version,token_hash,credential_ciphertext FROM checkout.order_pay_entries WHERE order_id=$1`, a.AttemptID).Scan(&material.KeyVersion, &digest, &material.Ciphertext); err != nil || len(digest) != len(material.Digest) {
		t.Fatal("process credential replay material missing")
	}
	copy(material.Digest[:], digest)
	credential, err := payEntryTestKeys(t).Open(orderattempt.PayEntryBinding{TenantID: e.tenant(), StoreID: e.store(), OrderID: a.AttemptID, Version: 1}, material)
	if err != nil || len(credential) != 43 {
		t.Fatal("process did not encrypt its reusable entry with configured pay keys")
	}
	if bytes.Contains(receipt, []byte(credential)) {
		t.Fatal("terminal process receipt exposed the pay credential")
	}
}
