// Purpose: REAL_PG tests-first proof that actual C2 can commit a READY notification.
// Depends on: real signed same-store ingress, C1, concrete Executor and native crypto/jobs.
// Used by: parent-owned OA1 CI; no native stage is prewritten and no dispatcher starts.
// Invariants: exact job transaction provenance, complete native commit, immutable replay,
// scoped outbox/operation/encrypted artifacts; no provider call or synthetic COMMITTED row.
package foundation_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"livecommerce/internal/inbox"
	"livecommerce/internal/msgtemplates"
	"livecommerce/internal/orderattempt"
)

func oaNotificationExecutorWithPayloadKey(t *testing.T, pool *pgxpool.Pool, payloadKey []byte) *orderattempt.Executor {
	t.Helper()
	keyJSON := `{"keys":[{"id":"` + miKeyID + `","key_base64":"` + base64.StdEncoding.EncodeToString(payloadKey) + `"}]}`
	keys, err := inbox.LoadKeyring(func(n string) string {
		if n == "COMMERCE_META_PAYLOAD_ACTIVE_KEY_ID" {
			return miKeyID
		}
		if n == "COMMERCE_META_PAYLOAD_KEYS_JSON" {
			return keyJSON
		}
		return ""
	})
	if err != nil {
		t.Fatal("actual synthetic payload ring", err)
	}
	notifications, err := inbox.NewService(keys)
	if err != nil {
		t.Fatal(err)
	}
	seal, _ := mcnPageRing(t)
	jobs, err := river.NewClient[pgx.Tx](riverpgxv5.New(pool), &river.Config{Schema: "river"})
	if err != nil {
		t.Fatal(err)
	}
	notifications.EnableSend(seal, jobs, msgtemplates.NewService())
	executor, err := orderattempt.NewExecutor(pool, oaServiceKeys(t), payEntryTestKeys(t), notifications)
	if err != nil {
		t.Fatal("concrete native executor", err)
	}
	return executor
}

func TestOrderAttemptNotificationExecutorReadyNativeJobTransaction(t *testing.T) {
	for _, worker := range []bool{false, true} {
		name := "runtime"
		if worker {
			name = "claims-worker"
		}
		t.Run(name, func(t *testing.T) {
			e := lbuNew(t)
			m, conversation, binding, _ := oaNotificationSignedConversation(t, e)
			selection := oaBeginSelection(t, e, "bank_transfer", "duplicate-origins")
			selection["for"] = map[string]any{"bundle_ids": []string{}, "conversation_id": conversation}
			selection["send_payment_link"] = true
			a := oaAllocationAdmit(t, e, selection)
			pool, token, store := e.p.f.runtime, e.token(), e.store()
			if worker {
				pool, token, store = miPool(t, e.p.f, "commerce_claims_worker"), "", ""
			}
			executor := oaNotificationExecutorWithPayloadKey(t, pool, m.key)
			// Only C1 and signed ingress exist. The executor itself must materialize
			// quote/destination/hold/Begin/pay-entry and create the exact River job.
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			first, err := executor.Execute(ctx, token, store, a.AttemptID, a.Generation)
			if err != nil {
				t.Fatalf("actual %s READY executor must satisfy native lcn_emit transaction provenance: %v", name, err)
			}
			if first.State != "COMMITTED" {
				t.Fatal("READY execution did not commit its native order")
			}
			var receipt struct {
				State     string `json:"state"`
				AttemptID string `json:"attempt_id"`
				OrderID   string `json:"order_id"`
				Send      struct {
					State          string `json:"state"`
					OperationID    string `json:"operation_id"`
					NotificationID string `json:"notification_id"`
				} `json:"send"`
			}
			if json.Unmarshal(first.Receipt, &receipt) != nil || receipt.State != "COMMITTED" || receipt.AttemptID != a.AttemptID || receipt.OrderID != a.AttemptID || receipt.Send.State != "queued" || receipt.Send.OperationID == "" || receipt.Send.NotificationID == "" {
				t.Fatal("READY terminal receipt must retain native opaque send identities")
			}
			for _, private := range []string{"synthetic OA inbound", "buyer_link", "credential_ciphertext", "dispatch_ciphertext", "psid", "/pay-entry#"} {
				if bytes.Contains(first.Receipt, []byte(private)) {
					t.Fatal("terminal receipt exposed private notification material")
				}
			}
			var exact bool
			var originalEntry string
			err = e.p.f.owner.QueryRow(ctx, `SELECT
			 a.state='COMMITTED' AND a.receipt=$2::jsonb AND a.order_id=a.attempt_id AND a.plan_ciphertext IS NULL AND a.plan_key_version IS NULL
			 AND n.id=$3 AND n.operation_id=$4 AND n.tenant_id=a.tenant_id AND n.store_id=a.store_id AND n.owner_id=a.buyer_owner_id
			 AND n.create_attempt_id=a.attempt_id AND n.order_id=o.id AND n.link_version=1 AND n.state='READY' AND n.void_stage='none'
			 AND n.conversation_id=$5 AND n.binding_id=$6 AND n.dispatch_authorized_at IS NULL AND n.worker_generation=0
			 AND o.creator_kind='order_attempt' AND o.create_attempt_id=a.attempt_id AND o.creator_session_id IS NULL
			 AND o.source='merchant_manual' AND o.commercial_state='AWAITING_TRANSFER' AND o.generation=1
			 AND r.state='HELD' AND r.create_attempt_id=a.attempt_id AND r.generation=o.generation
			 AND p.link_version=1 AND p.revoked_at IS NULL AND p.expires_at>clock_timestamp()
			 AND x.state='READY' AND x.action='meta.dm_send' AND x.binding_id=n.binding_id AND x.generation=n.worker_generation
			 AND x.request->>'policy'='order-attempt-notification/v1' AND x.request->>'notification_id'=n.id::text
			 AND x.request->>'order_id'=o.id::text AND x.request->>'link_version'='1'
			 AND j.kind='external_operation_v1' AND j.queue='default' AND j.unique_key IS NULL
			 AND j.args=jsonb_build_object('operation_id',x.id::text,'version',1)
			 AND b.operation_id=x.id AND b.id=n.outbound_id AND b.conversation_id=n.conversation_id
			 AND b.template_id='order-pay-link/v1' AND b.template_version=1 AND octet_length(b.nonce)=12 AND octet_length(b.ciphertext)>16
			 AND s.operation_id=x.id AND octet_length(s.enc)=32 AND octet_length(s.sealed)>16
			 AND expiry.kind='checkout_expiry_v1' AND expiry.args->>'order_id'=o.id::text,
			 to_jsonb(p)::text
			 FROM ops.order_attempts a JOIN checkout.orders o ON o.id=a.order_id
			 JOIN inventory.reservations r ON r.id=o.id JOIN checkout.order_pay_entries p ON p.order_id=o.id
			 JOIN inbox.order_payment_notifications n ON n.create_attempt_id=a.attempt_id
			 JOIN integration.operations x ON x.id=n.operation_id JOIN river.river_job j ON j.id=x.job_id
			 JOIN inbox.outbound_messages b ON b.id=n.outbound_id JOIN inbox.send_secrets s ON s.operation_id=x.id
			 JOIN river_expiry.river_job expiry ON expiry.id=o.job_id WHERE a.attempt_id=$1`, a.AttemptID, string(first.Receipt), receipt.Send.NotificationID, receipt.Send.OperationID, conversation, binding).Scan(&exact, &originalEntry)
			if err != nil || !exact {
				t.Fatalf("committed exact native notification and encrypted job pair: exact=%v err=%v", exact, err)
			}
			again, err := executor.Execute(ctx, token, store, a.AttemptID, a.Generation)
			if err != nil || again.State != first.State || !bytes.Equal(first.Receipt, again.Receipt) {
				t.Fatal("READY executor replay changed durable receipt or failed")
			}
			var unchanged bool
			err = e.p.f.owner.QueryRow(ctx, `SELECT
			 (SELECT to_jsonb(p)::text=$2 FROM checkout.order_pay_entries p WHERE p.order_id=$1)
			 AND (SELECT count(*)=1 FROM inbox.order_payment_notifications WHERE create_attempt_id=$1)
			 AND (SELECT count(*)=1 FROM integration.operations WHERE action='meta.dm_send' AND request->>'order_id'=($1::uuid)::text)
			 AND (SELECT count(*)=1 FROM river.river_job WHERE args->>'operation_id'=$3::text)
			 AND (SELECT count(*)=1 FROM inbox.outbound_messages WHERE operation_id=$3::uuid)
			 AND (SELECT count(*)=1 FROM inbox.send_secrets WHERE operation_id=$3::uuid)
			 AND NOT EXISTS(SELECT 1 FROM ops.order_attempt_subjects WHERE attempt_id=$1)`, a.AttemptID, originalEntry, receipt.Send.OperationID).Scan(&unchanged)
			if err != nil || !unchanged {
				t.Fatalf("READY replay rotated entry or duplicated native send effects: %v", err)
			}
		})
	}
}

func TestOrderAttemptNotificationExecutorAEADDependencyRetainsPending(t *testing.T) {
	e := oaAreaIsolated(t)
	m, conversation, _ := oaEnabledProcessConversation(t, e)
	selection := oaBeginSelection(t, e, "bank_transfer", "duplicate-origins")
	selection["for"] = map[string]any{"bundle_ids": []string{}, "conversation_id": conversation}
	selection["send_payment_link"] = true
	a := oaAllocationAdmit(t, e, selection)
	var originalPlan []byte
	var originalKey string
	var externalJobs int
	err := e.p.f.owner.QueryRow(context.Background(), `SELECT plan_ciphertext,plan_key_version,
	 (SELECT count(*) FROM river.river_job WHERE kind='external_operation_v1') FROM ops.order_attempts WHERE attempt_id=$1`, a.AttemptID).Scan(&originalPlan, &originalKey, &externalJobs)
	if err != nil || len(originalPlan) == 0 || originalKey == "" || externalJobs != 0 {
		t.Fatal("real C1 encrypted plan custody sentinel")
	}
	defer clear(originalPlan)
	// Same recognized ID, different valid synthetic bytes: this exercises AEAD
	// authentication, rather than missing configuration or handcrafted DTO errors.
	wrongKey := bytes.Clone(m.key)
	wrongKey[0] ^= 1
	defer clear(wrongKey)
	broken := oaNotificationExecutorWithPayloadKey(t, e.p.f.runtime, wrongKey)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	failed, err := broken.Execute(ctx, e.token(), e.store(), a.AttemptID, a.Generation)
	if err == nil || failed.State != "" || len(failed.Receipt) != 0 {
		t.Fatal("actual AEAD dependency failure fabricated a terminal receipt")
	}
	var preserved bool
	err = e.p.f.owner.QueryRow(ctx, `SELECT state='PENDING' AND receipt IS NULL AND order_id IS NULL
	 AND plan_ciphertext=$2 AND plan_key_version=$3
	 AND NOT EXISTS(SELECT 1 FROM checkout.orders WHERE create_attempt_id=$1)
	 AND NOT EXISTS(SELECT 1 FROM storefront.carts WHERE create_attempt_id=$1)
	 AND NOT EXISTS(SELECT 1 FROM storefront.quotes WHERE create_attempt_id=$1)
	 AND NOT EXISTS(SELECT 1 FROM storefront.destination_snapshots WHERE create_attempt_id=$1)
	 AND NOT EXISTS(SELECT 1 FROM inventory.reservations WHERE create_attempt_id=$1)
	 AND NOT EXISTS(SELECT 1 FROM checkout.order_pay_entries WHERE create_attempt_id=$1)
	 AND NOT EXISTS(SELECT 1 FROM checkout.command_results WHERE create_attempt_id=$1)
	 AND NOT EXISTS(SELECT 1 FROM checkout.bank_transfers WHERE order_id=$1)
	 AND NOT EXISTS(SELECT 1 FROM inbox.order_payment_notifications WHERE create_attempt_id=$1)
	 AND NOT EXISTS(SELECT 1 FROM integration.operations WHERE request->>'order_id'=($1::uuid)::text)
	 AND NOT EXISTS(SELECT 1 FROM river_expiry.river_job WHERE args->>'order_id'=($1::uuid)::text)
	 AND (SELECT count(*)=$4 FROM river.river_job WHERE kind='external_operation_v1')
	 FROM ops.order_attempts WHERE attempt_id=$1`, a.AttemptID, originalPlan, originalKey, externalJobs).Scan(&preserved)
	if err != nil || !preserved {
		t.Fatalf("AEAD fault changed PENDING custody or retained partial native effects: %v", err)
	}
	// Restoring the actual original key makes the same admitted intent runnable.
	// The successful commit also distinguishes dependency failure from a VOID.
	healthy := oaNotificationExecutorWithPayloadKey(t, e.p.f.runtime, m.key)
	good, err := healthy.Execute(ctx, e.token(), e.store(), a.AttemptID, a.Generation)
	if err != nil || good.State != "COMMITTED" {
		t.Fatalf("original key could not resume unchanged intent: %v", err)
	}
	var ready bool
	err = e.p.f.owner.QueryRow(ctx, `SELECT a.state='COMMITTED' AND a.plan_ciphertext IS NULL
	 AND a.receipt->'send'->>'state'='queued' AND n.state='READY' AND n.operation_id IS NOT NULL
	 FROM ops.order_attempts a JOIN inbox.order_payment_notifications n ON n.create_attempt_id=a.attempt_id WHERE a.attempt_id=$1`, a.AttemptID).Scan(&ready)
	if err != nil || !ready {
		t.Fatalf("crypto recovery did not produce actual native READY disposition: %v", err)
	}
}
