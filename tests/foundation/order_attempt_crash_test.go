// Purpose: REAL_PG tests-first crash acceptance at every native C2 write boundary.
// Depends on: real signed ingress, merchant payment settings, C1 claim custody, concrete Executor and native writers.
// Used by: parent-owned OA1 CI; isolated fixture triggers never become product hooks.
// Invariants: a positively witnessed precommit connection kill preserves PENDING and
// zero C2 effects; postcommit process loss preserves the receipt and complete write set.
package foundation_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/orderattempt"
	"livecommerce/internal/platform"
)

type oaCrashScenario struct {
	e                                                 *lbuEnv
	a                                                 orderattempt.AdmissionReceipt
	executor                                          *orderattempt.Executor
	pool                                              *pgxpool.Pool
	payloadKey                                        []byte
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
		// The isolated expiry seed forbids CVS bank transfers. Enable this
		// scenario through the merchant route before asserting native C1/C2.
		status, _, _ := e.mcall(e.token(), "PUT", "/v1/admin/stores/"+e.store()+"/bank-transfer-settings", t04Key("oa-crash-cvs"),
			`{"expected_version":1,"enabled":true,"allow_cvs":true,"bank_name":"Synthetic bank","branch":"","account_name":"Synthetic merchant","account_number":"12345678","window_hours":6}`)
		if status != 200 {
			t.Fatalf("native CVS transfer settings prerequisite: status=%d", status)
		}
		var enabled bool
		if err := e.p.f.owner.QueryRow(context.Background(), `SELECT enabled AND allow_cvs AND version=2
		 AND bank_name='Synthetic bank' AND branch='' AND account_name='Synthetic merchant'
		 AND account_number='12345678' AND window_hours=6 FROM checkout.bank_transfer_settings
		 WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store()).Scan(&enabled); err != nil || !enabled {
			t.Fatal("native CVS transfer settings readback prerequisite", err)
		}
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
		pool: pool, payloadKey: m.key, application: application, bundle: bundle, offer: offer.ID, conversation: conversation, binding: binding}
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
	 AND (SELECT count(*)=1 FROM river_expiry.river_job WHERE kind='checkout_expiry_v1' AND args->>'order_id'=o.id::text)
	 AND (SELECT count(*)=1 FROM checkout.orders WHERE create_attempt_id=$1)
	 AND (SELECT count(*)=1 FROM inventory.reservations WHERE create_attempt_id=$1)
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

// OA-1 has no public HTTP adapter. Both processes use the concrete runtime
// executor; this loopback transport only frames its response and the crash point.
type oaHTTPFixture struct {
	DSN, Token, Store, Attempt string
	Generation                 int64
	PayloadKey                 []byte
	Pause, Early               bool
}

func oaServeCrashChild(t *testing.T) {
	var f oaHTTPFixture
	if err := json.NewDecoder(os.Stdin).Decode(&f); err != nil {
		t.Fatal("invalid private child configuration")
	}
	pool, err := platform.OpenPool(context.Background(), f.DSN)
	if err != nil {
		t.Fatal("child runtime pool authority")
	}
	defer pool.Close()
	executor := oaNotificationExecutorWithPayloadKey(t, pool, f.PayloadKey)
	socket := os.NewFile(3, "fixture listener")
	listener, err := net.FileListener(socket)
	socket.Close()
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	witness := os.NewFile(4, "before response")
	defer witness.Close()
	server := http.Server{ReadHeaderTimeout: 5 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != fmt.Sprintf("/%s/%d", f.Attempt, f.Generation) || r.Header.Get("Authorization") != "Bearer "+f.Token {
			http.Error(w, "wrong fixture intent", http.StatusForbidden)
			return
		}
		pause := func() {
			// Only the parent can release this request, by closing the client
			// or killing this process. No response has been written.
			if _, err := witness.Write([]byte{1}); err != nil {
				t.Error("signal response barrier")
				return
			}
			<-r.Context().Done()
		}
		if f.Pause && f.Early {
			pause()
		}
		out, err := executor.Execute(r.Context(), f.Token, f.Store, f.Attempt, f.Generation)
		if err != nil {
			t.Errorf("child executor: %v", err)
			http.Error(w, "execution failed", 500)
			return
		}
		if f.Pause && !f.Early {
			pause()
		}
		if err := json.NewEncoder(w).Encode(out); err != nil {
			t.Error("write recovery response")
		}
	})}
	t.Fatal(server.Serve(listener))
}

// Hand off an already bound loopback listener. Private DSN/key material travels
// via stdin. Only this exec.Cmd's child is signalled and waited on.
func oaCrashHTTPRequest(t *testing.T, s oaCrashScenario, pause bool) (net.Conn, *os.File, func()) {
	t.Helper()
	listener, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	socket, err := listener.File()
	if err != nil {
		t.Fatal(err)
	}
	defer socket.Close()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reader.Close() })
	defer writer.Close()
	deadline := time.Now().Add(30 * time.Second)
	if err := reader.SetReadDeadline(deadline); err != nil {
		t.Fatal(err)
	}
	config, err := json.Marshal(oaHTTPFixture{
		DSN: s.pool.Config().ConnString(), Token: s.e.token(), Store: s.e.store(),
		Attempt: s.a.AttemptID, Generation: s.a.Generation, PayloadKey: s.payloadKey,
		Pause: pause, Early: os.Getenv("LC_OA_CRASH_CONTROL") == "early-barrier",
	})
	if err != nil {
		t.Fatal("child fixture encoding")
	}

	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestOrderAttemptExecutorCrashBarrierMatrix$")
	cmd.Env = []string{"LC_OA_HTTP_CHILD=1"}
	cmd.ExtraFiles = []*os.File{socket, writer}
	cmd.Stdin = bytes.NewReader(config)
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	kill := func() {
		if cmd.ProcessState != nil {
			return
		}
		err := cmd.Process.Kill()
		waitErr := cmd.Wait()
		var exit *exec.ExitError
		if err != nil || !errors.As(waitErr, &exit) || exit.Sys().(syscall.WaitStatus).Signal() != syscall.SIGKILL || ctx.Err() != nil {
			t.Errorf("expected owned child SIGKILL and reap: kill=%v wait=%v deadline=%v", err, waitErr, ctx.Err())
		}
	}
	t.Cleanup(kill)
	var dial net.Dialer
	conn, err := dial.DialContext(ctx, "tcp4", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if err := conn.SetDeadline(deadline); err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Fprintf(conn, "POST /%s/%d HTTP/1.1\r\nHost: localhost\r\nAuthorization: Bearer %s\r\nContent-Length: 0\r\nConnection: close\r\n\r\n", s.a.AttemptID, s.a.Generation, s.e.token()); err != nil {
		t.Fatal("write fixture request")
	}
	return conn, reader, kill
}

func oaCrashAfterCommitBeforeHTTP(t *testing.T) {
	s := oaCrashNativeScenario(t)
	first, barrier, kill := oaCrashHTTPRequest(t, s, true)
	var one [1]byte
	if _, err := io.ReadFull(barrier, one[:]); err != nil || one[0] != 1 {
		t.Fatalf("missing before-response barrier: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var original orderattempt.ExecutionReceipt
	err := s.e.p.f.owner.QueryRow(ctx, `SELECT a.state, a.receipt FROM ops.order_attempts a
        JOIN checkout.orders o ON o.id=a.order_id AND o.create_attempt_id=a.attempt_id
        WHERE a.attempt_id=$1 AND a.intent_generation=$2 AND a.state='COMMITTED' AND a.receipt IS NOT NULL`, s.a.AttemptID, s.a.Generation).Scan(&original.State, &original.Receipt)
	if err != nil {
		t.Fatalf("independent pre-response reader requires original committed order and receipt: %v", err)
	}
	durable, custody := oaCrashCommitted(t, s, original), oaCrashCustody(t, s)
	kill()
	if n, err := first.Read(one[:]); n != 0 || !(errors.Is(err, io.EOF) || errors.Is(err, syscall.ECONNRESET)) {
		t.Fatalf("expected zero HTTP bytes at child death: n=%d err=%v", n, err)
	}
	recovered, _, stop := oaCrashHTTPRequest(t, s, false)
	response, err := http.ReadResponse(bufio.NewReader(recovered), nil)
	if err != nil {
		t.Fatal("fresh process HTTP recovery", err)
	}
	defer response.Body.Close()
	var replay orderattempt.ExecutionReceipt
	if response.StatusCode != http.StatusOK || json.NewDecoder(response.Body).Decode(&replay) != nil {
		t.Fatalf("invalid fresh process receipt: status=%d", response.StatusCode)
	}
	stop()
	oaExecutorSameReceipt(t, original, replay)
	if !bytes.Equal(durable, oaCrashCommitted(t, s, replay)) || !bytes.Equal(custody, oaCrashCustody(t, s)) {
		t.Fatal("fresh process recovery changed original durable effects or custody")
	}
	t.Log("REAL_PG + test HTTP transport: committed order and receipt witnessed; owned child killed and reaped; zero response bytes; exact-intent recovery preserved receipt, custody and native effects")
}

func TestOrderAttemptExecutorCrashBarrierMatrix(t *testing.T) {
	if os.Getenv("LC_OA_HTTP_CHILD") == "1" {
		oaServeCrashChild(t)
		return
	}
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
	t.Run("post-commit-before-http", oaCrashAfterCommitBeforeHTTP)
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
