// Purpose: REAL_PG tests-first crash acceptance at every native C2 write boundary.
// Depends on: real signed ingress, C1 claim custody, concrete Executor and native writers.
// Used by: parent-owned OA1 CI; isolated fixture triggers never become product hooks.
// Invariants: a positively witnessed precommit connection kill preserves PENDING and
// zero C2 effects; one explicit retry commits the complete set and replay is immutable.
package foundation_test

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/orderattempt"
)

type oaCrashScenario struct {
	e                                                 *lbuEnv
	a                                                 orderattempt.AdmissionReceipt
	executor                                          *orderattempt.Executor
	pool                                              *pgxpool.Pool
	application, bundle, offer, conversation, binding string
}

func oaCrashNativeScenario(t *testing.T) oaCrashScenario {
	t.Helper()
	return oaCrashNativeDeliveryScenario(t, false)
}

func oaCrashNativeDeliveryScenario(t *testing.T, cvs bool) oaCrashScenario {
	t.Helper()
	e := oaAreaIsolated(t)
	session, offer := e.session("A1", ltgLive, 5)
	bundle := e.h.accepted(t, session, "", "crash-"+t04Tag(), "A1+2").BundleID
	e.h.closeWindow(t, session)
	m, conversation, binding := oaEnabledProcessConversation(t, e)
	// The accepted claim is native. This existing fixture helper records its
	// peer association, exactly as the other native claim acceptance tests do.
	e.linkPeer(bundle, conversation)
	selection := oaClaimSelection(e, []string{bundle}, []string{offer.ID}, conversation)
	if cvs {
		pickup := oaMaterialCVS(t, e, "cvs_711", "001234", "bank_transfer")
		selection["delivery"], selection["customer"] = pickup["delivery"], pickup["customer"]
	}
	selection["send_payment_link"] = true
	a := oaAllocationAdmit(t, e, selection)
	config := e.p.f.runtime.Config().Copy()
	application := "oa_crash_" + strings.ReplaceAll(randomUUID(), "-", "")
	if config.ConnConfig.RuntimeParams == nil {
		config.ConnConfig.RuntimeParams = map[string]string{}
	}
	config.ConnConfig.RuntimeParams["application_name"] = application
	config.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatal("isolated actual runtime executor pool", err)
	}
	t.Cleanup(pool.Close)
	return oaCrashScenario{e: e, a: a, executor: oaNotificationExecutorWithPayloadKey(t, pool, m.key),
		pool: pool, application: application, bundle: bundle, offer: offer.ID, conversation: conversation, binding: binding}
}

// C1 reservations/grants/subject indexes are custody, not C2 effects. Compare
// their complete original rows and encrypted plan, without logging private bytes.
func oaCrashCustody(t *testing.T, s oaCrashScenario) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var raw []byte
	err := s.e.p.f.owner.QueryRow(ctx, `SELECT jsonb_build_object(
	 'state',a.state,'receipt',a.receipt,'order',a.order_id,'plan',encode(a.plan_ciphertext,'hex'),'key',a.plan_key_version,
	 'subjects',(SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY x.subject_kind,x.subject_id),'[]'::jsonb) FROM ops.order_attempt_subjects x WHERE x.attempt_id=a.attempt_id),
	 'reservations',(SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY x.bundle_id),'[]'::jsonb) FROM inbox.order_for_buyer x WHERE x.create_attempt_id=a.attempt_id),
	 'grants',(SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY x.bundle_id),'[]'::jsonb) FROM claims.merchant_origin_grants x WHERE x.create_attempt_id=a.attempt_id),
	 'external_jobs',(SELECT coalesce(jsonb_agg(id ORDER BY id),'[]'::jsonb) FROM river.river_job WHERE kind='external_operation_v1'))
	 FROM ops.order_attempts a WHERE a.attempt_id=$1`, s.a.AttemptID).Scan(&raw)
	if err != nil {
		t.Fatal("original C1 custody readback", err)
	}
	return raw
}

func oaCrashPending(t *testing.T, s oaCrashScenario, before []byte) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	after := oaCrashCustody(t, s)
	defer clear(after)
	if !bytes.Equal(before, after) {
		t.Fatal("connection kill changed original C1 custody or left an orphan external job")
	}
	var pending bool
	err := s.e.p.f.owner.QueryRow(ctx, `SELECT state='PENDING' AND receipt IS NULL AND order_id IS NULL
	 AND plan_ciphertext IS NOT NULL AND plan_key_version IS NOT NULL AND admission_finished
	 AND EXISTS(SELECT 1 FROM ops.order_attempt_subjects WHERE attempt_id=$1)
	 AND (SELECT count(*)=1 FROM inbox.order_for_buyer WHERE create_attempt_id=$1 AND state='pending' AND order_id IS NULL)
	 AND (SELECT count(*)=1 FROM claims.merchant_origin_grants WHERE create_attempt_id=$1 AND quote_id IS NULL AND consumed_at IS NULL)
	 FROM ops.order_attempts WHERE attempt_id=$1`, s.a.AttemptID).Scan(&pending)
	if err != nil || !pending {
		t.Fatalf("precommit crash must retain original PENDING intent: %v", err)
	}
	oaExecutorNoEffects(t, s.e, s.a)
	var residue int
	err = s.e.p.f.owner.QueryRow(ctx, `SELECT
	 (SELECT count(*) FROM storefront.events WHERE create_attempt_id=$1)
	 +(SELECT count(*) FROM storefront.destination_heads WHERE destination_id IN
	   (SELECT id FROM storefront.destination_snapshots WHERE create_attempt_id=$1))
	 +(SELECT count(*) FROM inbox.outbound_messages WHERE conversation_id=$2)
	 +(SELECT count(*) FROM inbox.send_secrets z JOIN integration.operations x ON x.id=z.operation_id WHERE x.request->>'order_id'=($1::uuid)::text)`, s.a.AttemptID, s.conversation).Scan(&residue)
	if err != nil || residue != 0 {
		t.Fatalf("precommit crash retained native event/outbound/secret effects: count=%d err=%v", residue, err)
	}
}

// This exact relational oracle is shared by the clean control, explicit recovery
// and replay. A fixture that cannot commit all stages cannot pass as crash proof.
func oaCrashCommitted(t *testing.T, s oaCrashScenario, out orderattempt.ExecutionReceipt) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	oaExecutorReceipt(t, out, s.a, "COMMITTED", "")
	var exact bool
	var durable []byte
	err := s.e.p.f.owner.QueryRow(ctx, `SELECT
	 a.state='COMMITTED' AND a.order_id=a.attempt_id AND a.receipt=$2::jsonb AND a.receipt->'send'->>'state'='queued'
	 AND a.plan_ciphertext IS NULL AND a.plan_key_version IS NULL AND a.settled_at IS NOT NULL
	 AND o.create_attempt_id=a.attempt_id AND o.owner_id=a.buyer_owner_id AND o.creator_kind='order_attempt'
	 AND o.creator_session_id IS NULL AND o.source='merchant_manual' AND o.commercial_state='AWAITING_TRANSFER'
	 AND p.create_attempt_id=a.attempt_id AND p.owner_id=o.owner_id AND p.link_version=1 AND p.revoked_at IS NULL
	 AND octet_length(p.token_hash)=32 AND octet_length(p.credential_ciphertext)=71 AND p.expires_at>clock_timestamp()
	 AND r.create_attempt_id=a.attempt_id AND r.state='HELD' AND r.generation=o.generation
	 AND (SELECT sum(quantity)=2 FROM inventory.reservation_lines WHERE reservation_id=o.id)
	 AND (SELECT sum(delta_reserved)=2 FROM inventory.ledger WHERE create_attempt_id=a.attempt_id AND actor_kind='ORDER_ATTEMPT')
	 AND n.state='READY' AND n.order_id=o.id AND n.owner_id=o.owner_id AND n.conversation_id=$5 AND n.binding_id=$6
	 AND n.link_version=p.link_version AND n.worker_generation=0 AND n.dispatch_authorized_at IS NULL
	 AND a.receipt->'send'->>'notification_id'=n.id::text AND a.receipt->'send'->>'operation_id'=n.operation_id::text
	 AND x.state='READY' AND x.action='meta.dm_send' AND x.binding_id=n.binding_id
	 AND x.request->>'policy'='order-attempt-notification/v1' AND x.request->>'notification_id'=n.id::text
	 AND x.request->>'order_id'=o.id::text AND j.args=jsonb_build_object('operation_id',x.id::text,'version',1)
	 AND j.kind='external_operation_v1' AND b.id=n.outbound_id AND b.operation_id=x.id
	 AND b.conversation_id=n.conversation_id AND b.template_id='order-pay-link/v1' AND b.template_version=1
	 AND octet_length(b.nonce)=12 AND octet_length(b.ciphertext)>16 AND octet_length(z.enc)=32 AND octet_length(z.sealed)>16
	 AND expiry.kind='checkout_expiry_v1' AND expiry.args->>'order_id'=o.id::text
	 AND (SELECT count(*)=1 FROM storefront.carts WHERE create_attempt_id=$1)
	 AND (SELECT count(*)=1 FROM storefront.quotes WHERE create_attempt_id=$1)
	 AND (SELECT count(*)=1 FROM storefront.destination_snapshots WHERE create_attempt_id=$1)
	 AND (SELECT count(*)=1 FROM checkout.command_results WHERE create_attempt_id=$1)
	 AND (SELECT count(*)=1 FROM checkout.bank_transfers WHERE order_id=$1)
	 AND (SELECT count(*)=1 FROM inbox.order_for_buyer WHERE create_attempt_id=$1 AND bundle_id=$3 AND state='placed' AND order_id=$1)
	 AND (SELECT count(*)=1 FROM claims.order_origins WHERE order_id=$1 AND bundle_id=$3 AND offer_id=$4)
	 AND (SELECT count(*)=1 FROM claims.live_price_uses WHERE order_id=$1 AND bundle_id=$3 AND offer_id=$4 AND quantity=2)
	 AND (SELECT count(*)=1 FROM claims.merchant_origin_grants WHERE create_attempt_id=$1 AND bundle_id=$3 AND quote_id=o.quote_id AND consumed_at IS NOT NULL)
	 AND (SELECT count(*)=1 FROM inbox.order_payment_notifications WHERE create_attempt_id=$1)
	 AND (SELECT count(*)=1 FROM integration.operations WHERE request->>'order_id'=($1::uuid)::text AND action='meta.dm_send')
	 AND (SELECT count(*)=1 FROM river.river_job WHERE args->>'operation_id'=x.id::text)
	 AND (SELECT count(*)=1 FROM inbox.outbound_messages WHERE operation_id=x.id)
	 AND (SELECT count(*)=1 FROM inbox.send_secrets WHERE operation_id=x.id)
	 AND (SELECT count(*)=1 FROM ops.audit_events WHERE details->>'attempt_id'=($1::uuid)::text AND action='order.manual_created')
	 AND NOT EXISTS(SELECT 1 FROM ops.order_attempt_subjects WHERE attempt_id=$1),
	 jsonb_build_object('entry',to_jsonb(p),'outbox',to_jsonb(n),'operation',to_jsonb(x),'outbound',to_jsonb(b),'secret',to_jsonb(z))
	 FROM ops.order_attempts a JOIN checkout.orders o ON o.id=a.order_id
	 JOIN checkout.order_pay_entries p ON p.order_id=o.id JOIN inventory.reservations r ON r.id=o.id
	 JOIN inbox.order_payment_notifications n ON n.create_attempt_id=a.attempt_id
	 JOIN integration.operations x ON x.id=n.operation_id JOIN river.river_job j ON j.id=x.job_id
	 JOIN inbox.outbound_messages b ON b.id=n.outbound_id JOIN inbox.send_secrets z ON z.operation_id=x.id
	 JOIN river_expiry.river_job expiry ON expiry.id=o.job_id WHERE a.attempt_id=$1`,
		s.a.AttemptID, string(out.Receipt), s.bundle, s.offer, s.conversation, s.binding).Scan(&exact, &durable)
	if err != nil || !exact {
		t.Fatalf("complete native claim/pay/READY/audit write set: exact=%v err=%v", exact, err)
	}
	return durable
}

type oaCrashBarrier struct{ name, table, timing, event, when, witness string }

// SQL literals below are test-owned constants; UUIDs are generated by the native
// fixture. The trigger's owner can read its uncommitted stage prefix. A blocked
// advisory request proves that the prefix assertion already succeeded.
func oaCrashInstallBarrier(t *testing.T, s oaCrashScenario, b oaCrashBarrier, key string) func() {
	t.Helper()
	name := "zz_oa_crash_" + strings.ReplaceAll(randomUUID(), "-", "")
	fn := pgx.Identifier{"public", name}.Sanitize()
	trigger := pgx.Identifier{name}.Sanitize()
	attempt := "'" + s.a.AttemptID + "'::uuid"
	when := strings.ReplaceAll(b.when, "ATTEMPT", attempt)
	witness := strings.ReplaceAll(b.witness, "ATTEMPT", attempt)
	ddl := fmt.Sprintf(`CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
	 BEGIN
	  IF NOT (%s) THEN RAISE EXCEPTION 'oa_crash_stage_prefix_missing'; END IF;
	  PERFORM pg_advisory_xact_lock(hashtextextended('%s',0));
	  RETURN NEW;
	 END $$`, fn, witness, key)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := s.e.p.f.owner.Exec(ctx, ddl); err != nil {
		t.Fatal("fixture-only barrier function", err)
	}
	cleaned := false
	cleanup := func() {
		if cleaned {
			return
		}
		clean, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		if _, err := s.e.p.f.owner.Exec(clean, "DROP TRIGGER IF EXISTS "+trigger+" ON "+b.table); err != nil {
			t.Error("bounded own trigger cleanup", err)
			return
		}
		if _, err := s.e.p.f.owner.Exec(clean, "DROP FUNCTION IF EXISTS "+fn+"()"); err != nil {
			t.Error("bounded own function cleanup", err)
			return
		}
		cleaned = true
	}
	t.Cleanup(cleanup)
	if _, err := s.e.p.f.owner.Exec(ctx, "REVOKE ALL ON FUNCTION "+fn+"() FROM PUBLIC"); err != nil {
		t.Fatal(err)
	}
	ddl = fmt.Sprintf("CREATE TRIGGER %s %s %s ON %s FOR EACH ROW WHEN (%s) EXECUTE FUNCTION %s()", trigger, b.timing, b.event, b.table, when, fn)
	if _, err := s.e.p.f.owner.Exec(ctx, ddl); err != nil {
		t.Fatal("fixture-only exact attempt barrier", err)
	}
	return cleanup
}

func oaCrashKillAtBarrier(t *testing.T, s oaCrashScenario, b oaCrashBarrier) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	holder, err := s.e.p.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	holderPID := int32(holder.Conn().PgConn().PID())
	t.Cleanup(func() {
		clean, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		_ = holder.Rollback(clean)
	})
	key := "oa.crash|" + randomUUID()
	if _, err = holder.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, key); err != nil {
		_ = holder.Rollback(ctx)
		t.Fatal(err)
	}
	drop := oaCrashInstallBarrier(t, s, b, key)
	done := make(chan oaExecutorOutcome, 1)
	go func() {
		out, callErr := s.executor.Execute(ctx, s.e.token(), s.e.store(), s.a.AttemptID, s.a.Generation)
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
				t.Error("bounded crash executor shutdown")
			}
		}
	}()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	var pid int32
	for pid == 0 {
		err = s.e.p.f.owner.QueryRow(ctx, `SELECT coalesce(max(pid),0) FROM pg_stat_activity
		 WHERE application_name=$1 AND wait_event_type='Lock' AND wait_event='advisory'
		 AND $2::int=ANY(pg_blocking_pids(pid))`, s.application, holderPID).Scan(&pid)
		if err != nil {
			t.Fatal("native advisory blocking readback", err)
		}
		if pid != 0 {
			break
		}
		select {
		case result := <-done:
			finished = true
			t.Fatalf("executor never reached positive %s barrier: %v", b.name, result.err)
		case <-ctx.Done():
			t.Fatalf("no positive native blocking sentinel at %s", b.name)
		case <-ticker.C:
		}
	}
	var killed bool
	if err = s.e.p.f.owner.QueryRow(ctx, `SELECT pg_terminate_backend($1::int)`, pid).Scan(&killed); err != nil || !killed {
		t.Fatalf("terminate positively witnessed executor: killed=%v err=%v", killed, err)
	}
	select {
	case result := <-done:
		finished = true
		if result.err == nil || result.receipt.State != "" || len(result.receipt.Receipt) != 0 {
			t.Fatal("precommit connection termination returned a fabricated terminal receipt")
		}
	case <-ctx.Done():
		t.Fatal("terminated executor did not return within bounded call")
	}
	if err = holder.Rollback(ctx); err != nil {
		t.Fatal("release own barrier", err)
	}
	drop()
	t.Logf("REAL_PG crash witness: stage=%s blocking_pid=%d terminated_pid=%d", b.name, holderPID, pid)
}

func TestOrderAttemptExecutorCrashBarrierMatrix(t *testing.T) {
	// A clean control runs before fault injection, including a discarded-response
	// replay. This reuses the existing lost-response oracle; it does not claim a
	// transport commit-ack injection or a kill after commit acknowledgement.
	if !t.Run("control-and-lost-response-replay", func(t *testing.T) {
		s := oaCrashNativeScenario(t)
		out, err := s.executor.Execute(context.Background(), s.e.token(), s.e.store(), s.a.AttemptID, s.a.Generation)
		if err != nil {
			t.Fatal("full native control prerequisite", err)
		}
		original := oaCrashCommitted(t, s, out)
		defer clear(original)
		again, err := s.executor.Execute(context.Background(), s.e.token(), s.e.store(), s.a.AttemptID, s.a.Generation)
		if err != nil {
			t.Fatal("discarded response replay", err)
		}
		oaExecutorSameReceipt(t, out, again)
		repeated := oaCrashCommitted(t, s, again)
		defer clear(repeated)
		if !bytes.Equal(original, repeated) {
			t.Fatal("lost-response replay rewrote native durable artifacts")
		}
	}) {
		t.Fatal("full native control failed; crash cases cannot supply acceptance evidence")
	}
	quote := `EXISTS(SELECT 1 FROM storefront.quotes q JOIN storefront.carts c ON c.id=q.cart_id WHERE q.create_attempt_id=ATTEMPT AND c.create_attempt_id=ATTEMPT)
	 AND EXISTS(SELECT 1 FROM storefront.cart_lines WHERE create_attempt_id=ATTEMPT)`
	begin := quote + ` AND EXISTS(SELECT 1 FROM checkout.orders o JOIN inventory.reservations r ON r.id=o.id
	 JOIN checkout.command_results c ON c.order_id=o.id JOIN river_expiry.river_job j ON j.id=o.job_id
	 WHERE o.create_attempt_id=ATTEMPT AND r.state='HELD' AND c.create_attempt_id=ATTEMPT AND j.kind='checkout_expiry_v1')`
	pay := begin + ` AND EXISTS(SELECT 1 FROM checkout.order_pay_entries WHERE create_attempt_id=ATTEMPT AND link_version=1)`
	claims := pay + ` AND EXISTS(SELECT 1 FROM inbox.order_for_buyer WHERE create_attempt_id=ATTEMPT AND state='placed' AND order_id=ATTEMPT)
	 AND EXISTS(SELECT 1 FROM claims.order_origins WHERE order_id=ATTEMPT)
	 AND EXISTS(SELECT 1 FROM claims.live_price_uses WHERE order_id=ATTEMPT AND quantity=2)
	 AND EXISTS(SELECT 1 FROM claims.merchant_origin_grants WHERE create_attempt_id=ATTEMPT AND consumed_at IS NOT NULL)`
	ready := claims + ` AND EXISTS(SELECT 1 FROM inbox.order_payment_notifications n JOIN integration.operations x ON x.id=n.operation_id
	 JOIN river.river_job j ON j.id=x.job_id JOIN inbox.outbound_messages b ON b.id=n.outbound_id
	 JOIN inbox.send_secrets z ON z.operation_id=x.id WHERE n.create_attempt_id=ATTEMPT AND n.state='READY'
	 AND j.args=jsonb_build_object('operation_id',x.id::text,'version',1) AND octet_length(b.ciphertext)>16 AND octet_length(z.sealed)>16)`
	for _, b := range []oaCrashBarrier{
		{"quote-cart", "storefront.quotes", "AFTER", "INSERT", "NEW.create_attempt_id=ATTEMPT", quote},
		{"destination", "storefront.destination_snapshots", "AFTER", "INSERT", "NEW.create_attempt_id=ATTEMPT", quote + ` AND EXISTS(SELECT 1 FROM storefront.destination_snapshots WHERE create_attempt_id=ATTEMPT)`},
		{"begin-order-hold", "checkout.command_results", "AFTER", "INSERT", "NEW.create_attempt_id=ATTEMPT", begin},
		{"pay-entry", "checkout.order_pay_entries", "AFTER", "INSERT", "NEW.create_attempt_id=ATTEMPT", pay},
		{"claims-completion", "inbox.order_for_buyer", "AFTER", "UPDATE", "NEW.create_attempt_id=ATTEMPT AND OLD.state='pending' AND NEW.state='placed'", claims},
		{"outbox-job", "inbox.order_payment_notifications", "AFTER", "INSERT", "NEW.create_attempt_id=ATTEMPT AND NEW.state='READY'", ready},
		{"before-terminal-receipt", "ops.order_attempts", "BEFORE", "UPDATE", "NEW.attempt_id=ATTEMPT AND OLD.state='PENDING' AND NEW.state='COMMITTED'", ready + ` AND NEW.receipt->>'state'='COMMITTED' AND OLD.receipt IS NULL`},
		{"after-terminal-update-before-commit", "ops.order_attempts", "AFTER", "UPDATE", "NEW.attempt_id=ATTEMPT AND OLD.state='PENDING' AND NEW.state='COMMITTED'", ready + ` AND EXISTS(SELECT 1 FROM ops.order_attempts WHERE attempt_id=ATTEMPT AND state='COMMITTED' AND receipt IS NOT NULL AND plan_ciphertext IS NULL)`},
	} {
		t.Run(b.name, func(t *testing.T) {
			s := oaCrashNativeScenario(t)
			before := oaCrashCustody(t, s)
			defer clear(before)
			oaCrashPending(t, s, before)
			oaCrashKillAtBarrier(t, s, b)
			oaCrashPending(t, s, before)
			// Exactly one explicit recovery attempt, never a blind retry loop.
			out, err := s.executor.Execute(context.Background(), s.e.token(), s.e.store(), s.a.AttemptID, s.a.Generation)
			if err != nil {
				t.Fatal("explicit same-attempt crash recovery", err)
			}
			original := oaCrashCommitted(t, s, out)
			defer clear(original)
			again, err := s.executor.Execute(context.Background(), s.e.token(), s.e.store(), s.a.AttemptID, s.a.Generation)
			if err != nil {
				t.Fatal("settled crash recovery replay", err)
			}
			oaExecutorSameReceipt(t, out, again)
			repeated := oaCrashCommitted(t, s, again)
			defer clear(repeated)
			if !bytes.Equal(original, repeated) {
				t.Fatal("crash recovery replay mutated native pay/claim/send artifacts")
			}
		})
	}
}
