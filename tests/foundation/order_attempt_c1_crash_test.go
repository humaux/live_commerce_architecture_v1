// Purpose: REAL_PG crash acceptance after complete native C1 sealing but before commit.
// Depends on: real claim fixtures, Service.Admit, existing SQL barrier and Executor oracles.
// Used by: parent-owned OA1 CI; the temporary owner trigger is not a production hook.
// Invariants: a witnessed backend kill rolls back private owner, custody, job and CAS;
// the same explicit request then admits once and completes exactly one native order.
package foundation_test

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/orderattempt"
)

// Snapshot native custody independently of an attempt ID that never became durable.
// Include all attempt jobs, so an orphan job cannot hide behind a rolled-back parent.
func oaC1CrashSnapshot(t *testing.T, e *lbuEnv, scope, bundle string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var raw []byte
	err := e.p.f.owner.QueryRow(ctx, `SELECT jsonb_build_object(
	 'scope',(SELECT to_jsonb(s) FROM ops.order_attempt_scopes s WHERE scope_id=$3),
	 'owners',(SELECT coalesce(jsonb_agg(to_jsonb(o) ORDER BY id),'[]'::jsonb) FROM buyer.owners o WHERE tenant_id=$1 AND store_id=$2),
	 'attempts',(SELECT coalesce(jsonb_agg(to_jsonb(a) ORDER BY attempt_id),'[]'::jsonb) FROM ops.order_attempts a WHERE tenant_id=$1 AND store_id=$2),
	 'creators',(SELECT coalesce(jsonb_agg(to_jsonb(a) ORDER BY attempt_id),'[]'::jsonb) FROM ops.order_attempt_creators a WHERE tenant_id=$1 AND store_id=$2),
	 'subjects',(SELECT coalesce(jsonb_agg(to_jsonb(a) ORDER BY attempt_id,subject_kind,subject_id),'[]'::jsonb) FROM ops.order_attempt_subjects a WHERE tenant_id=$1 AND store_id=$2),
	 'reservations',(SELECT coalesce(jsonb_agg(to_jsonb(r) ORDER BY request_id),'[]'::jsonb) FROM inbox.order_for_buyer r WHERE bundle_id=$4),
	 'grants',(SELECT coalesce(jsonb_agg(to_jsonb(g) ORDER BY request_id),'[]'::jsonb) FROM claims.merchant_origin_grants g WHERE bundle_id=$4),
	 'jobs',(SELECT coalesce(jsonb_agg(to_jsonb(j) ORDER BY id),'[]'::jsonb) FROM river.river_job j WHERE kind='order_attempt_v1'),
	 'orders',(SELECT count(*) FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2),
	 'carts',(SELECT count(*) FROM storefront.carts WHERE tenant_id=$1 AND store_id=$2),
	 'stock',(SELECT coalesce(jsonb_agg(to_jsonb(b) ORDER BY warehouse_id,sku_id),'[]'::jsonb) FROM inventory.balances b WHERE tenant_id=$1 AND store_id=$2))`,
		e.tenant(), e.store(), scope, bundle).Scan(&raw)
	if err != nil {
		t.Fatal("native C1 rollback snapshot", err)
	}
	return raw
}

func TestOrderAttemptC1CrashBeforeCommit(t *testing.T) {
	e := lbuNew(t)
	_, offer, bundle, conversation := e.scenario()
	req := oaMaterialRequest(t, e, oaClaimSelection(e, []string{bundle}, []string{offer.ID}, conversation))
	keys := oaServiceKeys(t)
	config := e.p.f.runtime.Config().Copy()
	application := "oa_c1_crash_" + strings.ReplaceAll(randomUUID(), "-", "")
	if config.ConnConfig.RuntimeParams == nil {
		config.ConnConfig.RuntimeParams = map[string]string{}
	}
	config.ConnConfig.RuntimeParams["application_name"] = application
	config.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatal("isolated actual C1 runtime pool", err)
	}
	t.Cleanup(pool.Close)
	svc, err := orderattempt.NewService(pool, keys)
	if err != nil {
		t.Fatal(err)
	}
	before := oaC1CrashSnapshot(t, e, req.ExpectedScopeID, bundle)
	defer clear(before)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	holder, err := e.p.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		clean, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		_ = holder.Rollback(clean)
	})
	holderPID := int32(holder.Conn().PgConn().PID())
	key := "oa.c1.crash|" + randomUUID()
	if _, err = holder.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, key); err != nil {
		t.Fatal(err)
	}
	// The AFTER trigger runs on the real finish UPDATE. No native constraint or
	// trigger is disabled. Blocking proves every uncommitted relational assertion
	// below succeeded, not merely that the client entered Service.Admit.
	barrier := oaCrashBarrier{
		name: "c1-sealed-before-commit", table: "ops.order_attempts", timing: "AFTER", event: "UPDATE",
		when: fmt.Sprintf("NEW.scope_id='%s'::uuid AND NEW.admission_finished AND NOT OLD.admission_finished", req.ExpectedScopeID),
		witness: fmt.Sprintf(`coalesce((NEW.state='PENDING' AND NEW.receipt IS NULL AND NEW.order_id IS NULL
		 AND NEW.enqueue_xid=pg_current_xact_id() AND NEW.execution_job_id=NEW.enqueue_job_id
		 AND NEW.plan_key_version IS NOT NULL AND octet_length(NEW.plan_ciphertext)>28
		 AND octet_length(NEW.plan_digest)=32 AND octet_length(NEW.facts_digest)=32
		 AND NEW.admission_provenance->>'private_owner'='true'
		 AND NEW.execute_by=NEW.admitted_at+interval '15 minutes'
		 AND current_setting('statement_timeout')='5s' AND current_setting('lock_timeout')='1s'
		 AND EXISTS(SELECT 1 FROM ops.order_attempt_scopes s WHERE s.scope_id=NEW.scope_id AND s.current_generation=NEW.intent_generation)
		 AND EXISTS(SELECT 1 FROM buyer.owners o WHERE o.id=NEW.buyer_owner_id AND o.tenant_id=NEW.tenant_id AND o.store_id=NEW.store_id AND o.active)
		 AND EXISTS(SELECT 1 FROM ops.order_attempt_creators c WHERE c.attempt_id=NEW.attempt_id AND c.buyer_owner_id=NEW.buyer_owner_id)
		 AND EXISTS(SELECT 1 FROM ops.order_attempt_subjects s WHERE s.attempt_id=NEW.attempt_id AND s.subject_kind='private_owner' AND s.subject_id=NEW.buyer_owner_id)
		 AND (SELECT count(*)=1 FROM river.river_job j WHERE j.id=NEW.execution_job_id
		  AND j.kind='order_attempt_v1' AND j.queue='claims' AND j.priority=1 AND j.state='available'
		  AND j.args=jsonb_build_object('attempt_id',NEW.attempt_id::text,'intent_generation',NEW.intent_generation))
		 AND (SELECT count(*)=1 FROM inbox.order_for_buyer r WHERE r.create_attempt_id=NEW.attempt_id AND r.bundle_id='%s'::uuid
		  AND r.buyer_id=NEW.buyer_owner_id AND r.state='pending' AND r.order_id IS NULL AND r.expires_at=NEW.execute_by)
		 AND (SELECT count(*)=1 FROM claims.merchant_origin_grants g WHERE g.create_attempt_id=NEW.attempt_id AND g.bundle_id='%s'::uuid
		  AND g.buyer_id=NEW.buyer_owner_id AND g.quote_id IS NULL AND g.consumed_at IS NULL AND g.expires_at=NEW.execute_by)
		 AND NOT EXISTS(SELECT 1 FROM checkout.orders WHERE create_attempt_id=NEW.attempt_id)
		 AND NOT EXISTS(SELECT 1 FROM storefront.carts WHERE create_attempt_id=NEW.attempt_id)),false)`, bundle, bundle),
	}
	drop := oaCrashInstallBarrier(t, oaCrashScenario{e: e}, barrier, key)
	type outcome struct {
		receipt orderattempt.AdmissionReceipt
		err     error
	}
	done := make(chan outcome, 1)
	go func() {
		out, callErr := svc.Admit(ctx, e.token(), e.store(), req)
		done <- outcome{out, callErr}
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
				t.Error("bounded C1 crash caller shutdown")
			}
		}
	}()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	var pid int32
	for pid == 0 {
		err = e.p.f.owner.QueryRow(ctx, `SELECT coalesce(max(pid),0) FROM pg_stat_activity
		 WHERE application_name=$1 AND state='active' AND wait_event_type='Lock' AND wait_event='advisory'
		 AND query LIKE '%ops.finish_order_attempt_admission%' AND $2::int=ANY(pg_blocking_pids(pid))`, application, holderPID).Scan(&pid)
		if err != nil {
			t.Fatal("native sealed C1 blocking witness", err)
		}
		if pid != 0 {
			break
		}
		select {
		case result := <-done:
			finished = true
			t.Fatalf("C1 never reached complete sealed barrier: %v", result.err)
		case <-ctx.Done():
			t.Fatal("no positive C1 precommit backend witness")
		case <-ticker.C:
		}
	}
	var killed bool
	if err = e.p.f.owner.QueryRow(ctx, `SELECT pg_terminate_backend($1::int)`, pid).Scan(&killed); err != nil || !killed {
		t.Fatalf("terminate witnessed C1 backend: killed=%v err=%v", killed, err)
	}
	select {
	case result := <-done:
		finished = true
		if result.err == nil || result.receipt.AttemptID != "" || result.receipt.State != "" || len(result.receipt.Receipt) != 0 {
			t.Fatal("terminated precommit C1 returned durable admission")
		}
	case <-ctx.Done():
		t.Fatal("terminated C1 did not return within bounded call")
	}
	if err = holder.Rollback(ctx); err != nil {
		t.Fatal("release C1 barrier", err)
	}
	drop()
	after := oaC1CrashSnapshot(t, e, req.ExpectedScopeID, bundle)
	defer clear(after)
	if !bytes.Equal(before, after) {
		t.Fatal("C1 crash leaked scope advancement, private owner, attempt, custody, stock or job")
	}
	t.Logf("REAL_PG C1 precommit witness: complete sealed custody, holder_pid=%d terminated_pid=%d", holderPID, pid)
	admitted, err := svc.Admit(context.Background(), e.token(), e.store(), req)
	if err != nil || admitted.Replay || admitted.State != "PENDING" || admitted.ScopeID != req.ExpectedScopeID || admitted.Generation != req.ExpectedGeneration+1 {
		t.Fatal("explicit same-key retry must admit once after real rollback", err)
	}
	plan, sealed := oaMaterialPlan(t, e, keys, admitted.AttemptID)
	defer clear(plan.Facts)
	defer clear(sealed.Ciphertext)
	replay, err := svc.Admit(context.Background(), e.token(), e.store(), req)
	if err != nil || !replay.Replay || replay.AttemptID != admitted.AttemptID || replay.Generation != admitted.Generation {
		t.Fatal("same-key C1 replay after crash recovery", err)
	}
	executor := oaExecutorNew(t, pool, keys)
	first, err := executor.Execute(context.Background(), e.token(), e.store(), admitted.AttemptID, admitted.Generation)
	if err != nil {
		t.Fatal("full native C2 after explicit C1 retry", err)
	}
	entry := oaExecutorCommitted(t, e, admitted, plan, first, 2)
	again, err := executor.Execute(context.Background(), e.token(), e.store(), admitted.AttemptID, admitted.Generation)
	if err != nil {
		t.Fatal("native C2 replay after C1 crash recovery", err)
	}
	oaExecutorSameReceipt(t, first, again)
	if entry != oaExecutorCommitted(t, e, admitted, plan, again, 2) {
		t.Fatal("recovery replay rewrote the original pay credential")
	}
	var exact bool
	err = e.p.f.owner.QueryRow(context.Background(), `SELECT
	 (SELECT count(*)=1 FROM ops.order_attempts WHERE scope_id=$1)
	 AND (SELECT count(*)=1 FROM river.river_job WHERE kind='order_attempt_v1' AND args->>'attempt_id'=$2)
	 AND (SELECT count(*)=1 FROM inbox.order_for_buyer WHERE create_attempt_id=$2::uuid AND bundle_id=$3 AND state='placed' AND order_id=$2::uuid)
	 AND (SELECT count(*)=1 FROM claims.merchant_origin_grants WHERE create_attempt_id=$2::uuid AND bundle_id=$3 AND quote_id IS NOT NULL AND consumed_at IS NOT NULL)
	 AND (SELECT count(*)=1 FROM claims.order_origins WHERE order_id=$2::uuid AND bundle_id=$3 AND offer_id=$4)
	 AND (SELECT count(*)=1 FROM claims.live_price_uses WHERE order_id=$2::uuid AND bundle_id=$3 AND offer_id=$4 AND quantity=2)`,
		req.ExpectedScopeID, admitted.AttemptID, bundle, offer.ID).Scan(&exact)
	if err != nil || !exact {
		t.Fatal("same-key recovery duplicated admission/job or lost native claim completion", err)
	}
}
