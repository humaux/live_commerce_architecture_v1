// Purpose: REAL_PG tests-first complete worker continuation with current DM refusal.
// Depends on: signed ingress, real C1 claim custody, concrete Executor and native writers.
// Used by: parent-owned OA1 CI; shared crash/fence oracles, no dispatcher or product hook.
// Invariants: placement permission/session loss cannot restart Place; VOID commits the
// complete order once without an operation/job; late refusal rolls back to PENDING.
package foundation_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/orderattempt"
)

type oaNotificationResumeScenario struct {
	oaCrashScenario
	plan orderattempt.FrozenPlan
}

func oaNotificationResumeNative(t *testing.T) oaNotificationResumeScenario {
	t.Helper()
	e := lbuNew(t)
	session, offer := e.session("A1", ltgLive, 5)
	bundle := e.h.accepted(t, session, "", "resume-"+t04Tag(), "A1+2").BundleID
	e.h.closeWindow(t, session)
	m, conversation, binding, _ := oaNotificationSignedConversation(t, e)
	e.linkPeer(bundle, conversation)
	selection := oaClaimSelection(e, []string{bundle}, []string{offer.ID}, conversation)
	selection["send_payment_link"] = true
	a := oaAllocationAdmit(t, e, selection)
	plan, _ := oaMaterialPlan(t, e, oaServiceKeys(t), a.AttemptID)
	config := miPool(t, e.p.f, "commerce_claims_worker").Config().Copy()
	application := "oa_resume_" + strings.ReplaceAll(randomUUID(), "-", "")
	if config.ConnConfig.RuntimeParams == nil {
		config.ConnConfig.RuntimeParams = map[string]string{}
	}
	config.ConnConfig.RuntimeParams["application_name"] = application
	config.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatal("isolated actual claims-worker pool", err)
	}
	t.Cleanup(pool.Close)
	s := oaNotificationResumeScenario{oaCrashScenario: oaCrashScenario{
		e: e, a: a, pool: pool, application: application, bundle: bundle,
		offer: offer.ID, conversation: conversation, binding: binding,
		executor: oaNotificationExecutorWithPayloadKey(t, pool, m.key),
	}, plan: plan}
	var ready bool
	err = e.p.f.owner.QueryRow(context.Background(), `SELECT a.state='PENDING' AND a.admission_finished
	 AND a.receipt IS NULL AND a.order_id IS NULL AND a.plan_ciphertext IS NOT NULL
	 AND b.enabled AND integration.binding_capability_state(a.tenant_id,a.store_id,b.id,'dm_session',ARRAY[]::text[]) IN ('ok','review_required')
	 AND c.tenant_id=a.tenant_id AND c.store_id=a.store_id AND c.id=$2
	 AND st.last_inbound_at+interval '24 hours'-interval '5 minutes'>clock_timestamp()
	 AND identity.principal_holds(a.tenant_id,a.store_id,$4,ARRAY['inbox:read','inbox:reply','catalog:read','inventory:reserve'])
	 AND EXISTS(SELECT 1 FROM inbox.order_for_buyer WHERE create_attempt_id=a.attempt_id AND bundle_id=$5 AND state='pending')
	 AND EXISTS(SELECT 1 FROM claims.merchant_origin_grants WHERE create_attempt_id=a.attempt_id AND bundle_id=$5 AND consumed_at IS NULL)
	 AND NOT EXISTS(SELECT 1 FROM river.river_job WHERE kind='external_operation_v1')
	 FROM ops.order_attempts a JOIN social.conversations c ON c.id=$2
	 JOIN inbox.conversation_state st ON st.conversation_id=c.id JOIN integration.bindings b ON b.id=$3
	 WHERE a.attempt_id=$1`, a.AttemptID, conversation, binding, e.p.f.principalA, bundle).Scan(&ready)
	if err != nil || !ready {
		t.Fatalf("signed conversation/current capability/actual C1 positive sentinel: %v", err)
	}
	return s
}

// These are fixture mutations after actual C1, not prewritten C2 stages. Ordinary
// Place now lacks its session and catalog/inventory authority in every case.
func oaNotificationResumeLosePlacement(t *testing.T, s oaNotificationResumeScenario) {
	t.Helper()
	mustExec(t, s.e.p.f.owner, `DELETE FROM identity.sessions WHERE token_hash=$1`, tokenHash(s.e.token()))
	mustExec(t, s.e.p.f.owner, `DELETE FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2
	 AND principal_id=$3 AND permission IN ('catalog:read','inventory:reserve')`, s.e.tenant(), s.e.store(), s.e.p.f.principalA)
	var denied bool
	err := s.e.p.f.owner.QueryRow(context.Background(), `SELECT
	 NOT EXISTS(SELECT 1 FROM identity.sessions WHERE token_hash=$1)
	 AND NOT identity.principal_holds($2,$3,$4,ARRAY['catalog:read','inventory:reserve'])`,
		tokenHash(s.e.token()), s.e.tenant(), s.e.store(), s.e.p.f.principalA).Scan(&denied)
	if err != nil || !denied {
		t.Fatalf("ordinary placement must be unavailable before worker continuation: %v", err)
	}
}

// VOID changes the notification portion of the shared READY crash oracle only.
// Check the same complete native quote/order/hold/claim/pay/audit identities.
func oaNotificationResumeCommittedVoid(t *testing.T, s oaNotificationResumeScenario, out orderattempt.ExecutionReceipt, code string) []byte {
	t.Helper()
	oaExecutorReceipt(t, out, s.a, "COMMITTED", "")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var exact bool
	var durable []byte
	err := s.e.p.f.owner.QueryRow(ctx, `SELECT
	 a.state='COMMITTED' AND a.order_id=a.attempt_id AND a.receipt=$2::jsonb
	 AND a.receipt->'send'=jsonb_build_object('state','not_sent','reason',$7::text,'notification_id',n.id::text)
	 AND a.plan_ciphertext IS NULL AND a.plan_key_version IS NULL AND a.settled_at IS NOT NULL
	 AND o.create_attempt_id=a.attempt_id AND o.owner_id=a.buyer_owner_id AND o.creator_kind='order_attempt'
	 AND o.tenant_id=a.tenant_id AND o.store_id=a.store_id AND o.creator_session_id IS NULL
	 AND o.source='merchant_manual' AND o.payment_mode='bank_transfer' AND o.commercial_state='AWAITING_TRANSFER'
	 AND o.total_minor=$6 AND o.generation=1
	 AND p.create_attempt_id=a.attempt_id AND p.owner_id=o.owner_id AND p.link_version=1 AND p.revoked_at IS NULL
	 AND octet_length(p.token_hash)=32 AND octet_length(p.credential_ciphertext)=71
	 AND p.expires_at=LEAST(o.expires_at,p.issued_at+interval '72 hours') AND p.expires_at>clock_timestamp()
	 AND r.create_attempt_id=a.attempt_id AND r.creator_kind='order_attempt' AND r.buyer_session_id IS NULL
	 AND r.state='HELD' AND r.generation=o.generation
	 AND (SELECT sum(quantity)=2 FROM inventory.reservation_lines WHERE reservation_id=o.id)
	 AND (SELECT sum(delta_reserved)=2 FROM inventory.ledger WHERE create_attempt_id=a.attempt_id AND actor_kind='ORDER_ATTEMPT')
	 AND n.kind='initial' AND n.semantic_key=sha256(convert_to('initial|'||a.attempt_id::text,'UTF8'))
	 AND n.tenant_id=a.tenant_id AND n.store_id=a.store_id AND n.owner_id=o.owner_id AND n.order_id=o.id
	 AND n.conversation_id=$5 AND n.link_version=p.link_version AND n.worker_generation=0
	 AND n.state='VOID' AND n.void_stage='before_c3' AND n.void_code=$7
	 AND n.operation_id IS NULL AND n.outbound_id IS NULL AND n.binding_id IS NULL AND n.dispatch_authorized_at IS NULL
	 AND expiry.kind='checkout_expiry_v1' AND expiry.args->>'order_id'=o.id::text
	 AND (SELECT count(*)=1 FROM checkout.orders WHERE create_attempt_id=$1)
	 AND (SELECT count(*)=1 FROM storefront.carts WHERE create_attempt_id=$1)
	 AND (SELECT count(*)=1 FROM storefront.quotes WHERE create_attempt_id=$1)
	 AND (SELECT count(*)=1 FROM storefront.destination_snapshots WHERE create_attempt_id=$1)
	 AND (SELECT count(*)=1 FROM inventory.reservations WHERE create_attempt_id=$1)
	 AND (SELECT count(*)=1 FROM checkout.command_results WHERE create_attempt_id=$1)
	 AND (SELECT count(*)=1 FROM checkout.bank_transfers WHERE order_id=$1)
	 AND (SELECT count(*)=1 FROM checkout.order_pay_entries WHERE create_attempt_id=$1)
	 AND (SELECT count(*)=1 FROM river_expiry.river_job WHERE args->>'order_id'=($1::uuid)::text)
	 AND (SELECT count(*)=1 FROM inbox.order_for_buyer WHERE create_attempt_id=$1 AND bundle_id=$3 AND state='placed' AND order_id=$1)
	 AND (SELECT count(*)=1 FROM claims.order_origins WHERE order_id=$1 AND bundle_id=$3 AND offer_id=$4)
	 AND (SELECT count(*)=1 FROM claims.live_price_uses WHERE order_id=$1 AND bundle_id=$3 AND offer_id=$4 AND quantity=2)
	 AND (SELECT count(*)=1 FROM claims.merchant_origin_grants WHERE create_attempt_id=$1 AND bundle_id=$3 AND quote_id=o.quote_id AND consumed_at IS NOT NULL)
	 AND (SELECT count(*)=1 FROM inbox.order_payment_notifications WHERE create_attempt_id=$1)
	 AND NOT EXISTS(SELECT 1 FROM integration.operations WHERE request->>'order_id'=($1::uuid)::text)
	 AND NOT EXISTS(SELECT 1 FROM river.river_job WHERE kind='external_operation_v1')
	 AND NOT EXISTS(SELECT 1 FROM inbox.outbound_messages WHERE conversation_id=$5)
	 AND NOT EXISTS(SELECT 1 FROM inbox.send_secrets z JOIN integration.operations x ON x.id=z.operation_id WHERE x.request->>'order_id'=($1::uuid)::text)
	 AND (SELECT count(*)=1 FROM ops.audit_events WHERE details->>'attempt_id'=($1::uuid)::text AND action='order.manual_created'
	  AND principal_id=(a.admission_provenance->>'principal_id')::uuid AND details=jsonb_build_object('attempt_id',$1::text,'order_id',$1::text,'intent_generation',a.intent_generation))
	 AND NOT EXISTS(SELECT 1 FROM ops.order_attempt_subjects WHERE attempt_id=$1),
	 jsonb_build_object('receipt',a.receipt,'entry',to_jsonb(p),'outbox',to_jsonb(n),'order',to_jsonb(o),'hold',to_jsonb(r))
	 FROM ops.order_attempts a JOIN checkout.orders o ON o.id=a.order_id
	 JOIN checkout.order_pay_entries p ON p.order_id=o.id JOIN inventory.reservations r ON r.id=o.id
	 JOIN inbox.order_payment_notifications n ON n.create_attempt_id=a.attempt_id
	 JOIN river_expiry.river_job expiry ON expiry.id=o.job_id WHERE a.attempt_id=$1`,
		s.a.AttemptID, string(out.Receipt), s.bundle, s.offer, s.conversation, s.plan.Calculation.TotalMinor, code).Scan(&exact, &durable)
	if err != nil || !exact {
		t.Fatalf("complete native commit with definitive VOID and zero notification operation/job: exact=%v err=%v", exact, err)
	}
	return durable
}

func oaNotificationResumeExecuteVoid(t *testing.T, s oaNotificationResumeScenario, code string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	first, err := s.executor.Execute(ctx, "", "", s.a.AttemptID, s.a.Generation)
	if err != nil {
		t.Fatalf("accepted worker continuation must commit current VOID without ordinary Place gates: %v", err)
	}
	original := oaNotificationResumeCommittedVoid(t, s, first, code)
	defer clear(original)
	again, err := s.executor.Execute(ctx, "", "", s.a.AttemptID, s.a.Generation)
	if err != nil {
		t.Fatal("same accepted intent terminal replay", err)
	}
	oaExecutorSameReceipt(t, first, again)
	repeated := oaNotificationResumeCommittedVoid(t, s, again, code)
	defer clear(repeated)
	if !bytes.Equal(original, repeated) {
		t.Fatal("terminal VOID replay rewrote the native order/hold/credential/outbox")
	}
}

func TestOrderAttemptNotificationResumeWorkerCommitsCurrentVoid(t *testing.T) {
	for _, tc := range []struct{ name, code string }{
		{"permission", "forbidden"}, {"binding", "capability"}, {"window", "window_closed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := oaNotificationResumeNative(t)
			oaNotificationResumeLosePlacement(t, s)
			switch tc.name {
			case "permission":
				mustExec(t, s.e.p.f.owner, `DELETE FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND permission='inbox:reply'`, s.e.tenant(), s.e.store(), s.e.p.f.principalA)
			case "binding":
				mustExec(t, s.e.p.f.owner, `UPDATE integration.bindings SET enabled=false WHERE id=$1 AND tenant_id=$2 AND store_id=$3`, s.binding, s.e.tenant(), s.e.store())
			case "window":
				mustExec(t, s.e.p.f.owner, `UPDATE inbox.conversation_state SET last_inbound_at=clock_timestamp()-interval '25 hours' WHERE conversation_id=$1`, s.conversation)
			}
			oaNotificationResumeExecuteVoid(t, s, tc.code)
		})
	}
}

func TestOrderAttemptNotificationResumeLateRefusalRollsBackThenVoid(t *testing.T) {
	s := oaNotificationResumeNative(t)
	oaNotificationResumeLosePlacement(t, s)
	before := oaCrashCustody(t, s.oaCrashScenario)
	defer clear(before)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	holder, err := s.e.p.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	holderPID := int32(holder.Conn().PgConn().PID())
	defer func() {
		clean, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		_ = holder.Rollback(clean)
	}()
	key := "oa.resume|" + randomUUID()
	if _, err = holder.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, key); err != nil {
		t.Fatal(err)
	}
	// Go inserts this exact external job only after PAY_PREPARED and actual
	// AEAD/HPKE preparation. No operation/outbox is committed at this barrier.
	drop := oaCrashInstallBarrier(t, s.oaCrashScenario, oaCrashBarrier{
		name: "prepared-before-finish", table: "river.river_job", timing: "AFTER", event: "INSERT",
		when: "NEW.kind='external_operation_v1'",
		witness: `NEW.args=jsonb_build_object('operation_id',NEW.args->>'operation_id','version',1)
		 AND EXISTS(SELECT 1 FROM checkout.order_pay_entries WHERE create_attempt_id=ATTEMPT)
		 AND EXISTS(SELECT 1 FROM inbox.order_for_buyer WHERE create_attempt_id=ATTEMPT AND state='pending')
		 AND NOT EXISTS(SELECT 1 FROM inbox.order_payment_notifications WHERE create_attempt_id=ATTEMPT)`,
	}, key)
	done := make(chan oaExecutorOutcome, 1)
	go func() {
		out, callErr := s.executor.Execute(ctx, "", "", s.a.AttemptID, s.a.Generation)
		done <- oaExecutorOutcome{out, callErr}
	}()
	finished := false
	defer func() {
		cancel()
		clean, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		_ = holder.Rollback(clean)
		if !finished {
			select {
			case <-done:
			case <-clean.Done():
				t.Error("bounded late-refusal executor shutdown")
			}
		}
	}()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		var blocked bool
		err = s.e.p.f.owner.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity
		 WHERE application_name=$1 AND wait_event_type='Lock' AND wait_event='advisory'
		 AND $2::int=ANY(pg_blocking_pids(pid)))`, s.application, holderPID).Scan(&blocked)
		if err != nil {
			t.Fatal("positive native preparation barrier", err)
		}
		if blocked {
			break
		}
		select {
		case result := <-done:
			finished = true
			t.Fatalf("executor did not reach prepared notification barrier: %v", result.err)
		case <-ctx.Done():
			t.Fatal("positive native preparation barrier absent")
		case <-ticker.C:
		}
	}
	// Current permission can change without acquiring the retained conversation
	// lock; this avoids a timing-based window race or any altered timeout budget.
	mustExec(t, s.e.p.f.owner, `DELETE FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2
	 AND principal_id=$3 AND permission='inbox:reply'`, s.e.tenant(), s.e.store(), s.e.p.f.principalA)
	if err = holder.Rollback(ctx); err != nil {
		t.Fatal("release own positively observed preparation barrier", err)
	}
	var result oaExecutorOutcome
	select {
	case result = <-done:
		finished = true
	case <-ctx.Done():
		t.Fatal("late refusal did not return within the original bounded execution")
	}
	drop()
	oaCrashPending(t, s.oaCrashScenario, before)
	var failure *orderattempt.AdmissionError
	if !errors.As(result.err, &failure) || failure.SQLState() != "PT409" || failure.Code() != "order_attempt_retryable" || failure.OutcomeUnknown() {
		t.Fatalf("late notification_reprepare must remain typed internal retryable after known rollback: %v", result.err)
	}
	if result.receipt.State != "" || len(result.receipt.Receipt) != 0 {
		t.Fatal("late refusal fabricated a terminal receipt")
	}
	oaNotificationResumeExecuteVoid(t, s, "forbidden")
}
