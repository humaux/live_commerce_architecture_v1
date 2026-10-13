// Purpose: REAL_PG acceptance of private OA1 erasure authority and retained proof.
// Depends on: actual customer erasure, C1/Executor and exact native function ACLs.
// Used by: parent CI; no owner-injected terminal state or local execution.
// Invariants: privacy has EXECUTE only, terminal commercial proof survives erasure.
package foundation_test

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/customers"
	"livecommerce/internal/orderattempt"
	"livecommerce/internal/platform"
)

func TestOrderAttemptErasurePrefixExactAuthority(t *testing.T) {
	e := lbuNew(t)
	for _, spec := range []struct {
		signature string
		owner     string
		caller    string
	}{
		{"ops.order_attempt_erase_prefix(uuid,uuid,uuid)", "commerce_order_attempt_owner", "commerce_privacy_writer"},
		{"ops.order_attempt_erase_batch_prefix(jsonb)", "commerce_order_attempt_owner", "commerce_privacy_writer"},
		{"buyer.order_attempt_lock_erasure_owners(jsonb,jsonb)", "commerce_buyer_writer", "commerce_order_attempt_owner"},
	} {
		t.Run(spec.signature, func(t *testing.T) {
			var exact bool
			err := e.p.f.owner.QueryRow(context.Background(), `SELECT p.prosecdef AND r.rolname=$2
			 AND NOT r.rolcanlogin AND NOT r.rolbypassrls AND p.proconfig=ARRAY['search_path=pg_catalog']
			 AND obj_description(p.oid,'pg_proc') IS NOT NULL
			 AND has_function_privilege($3,p.oid,'EXECUTE')
			 AND NOT EXISTS(SELECT 1 FROM aclexplode(coalesce(p.proacl,acldefault('f',p.proowner))) a
			  WHERE a.privilege_type='EXECUTE' AND a.grantee<>p.proowner
			   AND a.grantee<>(SELECT oid FROM pg_roles WHERE rolname=$3))
			 FROM pg_proc p JOIN pg_roles r ON r.oid=p.proowner WHERE p.oid=$1::regprocedure`,
				spec.signature, spec.owner, spec.caller).Scan(&exact)
			if err != nil || !exact {
				t.Fatalf("exact private erasure authority: exact=%v err=%v", exact, err)
			}
		})
	}
	var hidden bool
	err := e.p.f.owner.QueryRow(context.Background(), `SELECT NOT EXISTS(
	 SELECT 1 FROM pg_attribute a WHERE a.attrelid IN ('ops.order_attempts'::regclass,
	  'ops.order_attempt_subjects'::regclass,'ops.order_attempt_execution_fences'::regclass)
	 AND a.attnum>0 AND NOT a.attisdropped
	 AND has_column_privilege('commerce_privacy_writer',a.attrelid,a.attnum,'SELECT'))`).Scan(&hidden)
	if err != nil || !hidden {
		t.Fatalf("privacy must not receive ops SELECT: hidden=%v err=%v", hidden, err)
	}
	tx, err := e.p.f.runtime.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	oaAbortCleanupTx(t, tx)
	_, err = tx.Exec(context.Background(), `SELECT ops.order_attempt_erase_prefix($1,$2,$3)`, e.tenant(), e.store(), randomUUID())
	oaFenceCode(t, nil, err, "42501")
	if err = tx.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{
		`SELECT ops.order_attempt_erase_prefix(NULL,NULL,NULL)`,
		`SELECT ops.order_attempt_erase_batch_prefix('[]'::jsonb)`,
	} {
		ownerTx, err := e.p.f.owner.Begin(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		oaAbortCleanupTx(t, ownerTx)
		oaStateExec(t, ownerTx, `SET LOCAL ROLE commerce_privacy_writer`)
		_, err = ownerTx.Exec(context.Background(), query)
		oaFenceCode(t, nil, err, "PT400")
		if err = ownerTx.Rollback(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	ownerTx, err := e.p.f.owner.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	oaAbortCleanupTx(t, ownerTx)
	oaStateExec(t, ownerTx, `SET LOCAL ROLE commerce_privacy_writer`)
	_, err = ownerTx.Exec(context.Background(), `SELECT ops.order_attempt_erase_prefix($1,$2,$3)`, e.tenant(), e.store(), randomUUID())
	oaFenceCode(t, nil, err, "PT403")
	if err = ownerTx.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	batchTx, err := e.p.f.owner.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	oaAbortCleanupTx(t, batchTx)
	oaStateExec(t, batchTx, `SET LOCAL ROLE commerce_privacy_writer`)
	_, err = batchTx.Exec(context.Background(), `SELECT ops.order_attempt_erase_batch_prefix(
	 jsonb_build_array(jsonb_build_object('tenant_id',$1::uuid,'store_id',$2::uuid,'subject_id',$3::uuid)))`,
		e.tenant(), e.store(), randomUUID())
	oaFenceCode(t, nil, err, "PT403")
	if err = batchTx.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestOrderAttemptErasureSerializesConcurrentC1BeforeOwner(t *testing.T) {
	e := lbuNew(t)
	e.grantCreator("customers:privacy", "customers:read")
	owner, conversation := randomUUID(), randomUUID()
	mustExec(t, e.p.f.owner, `INSERT INTO buyer.owners(id,tenant_id,store_id) VALUES($1,$2,$3)`, owner, e.tenant(), e.store())
	mustExec(t, e.p.f.owner, `INSERT INTO social.conversations(id,tenant_id,store_id,app_id,object,asset_id,peer_key)
	 VALUES($1,$2,$3,'123456789','page','123456789',repeat(replace($1::uuid::text,'-',''),2))`, conversation, e.tenant(), e.store())
	mustExec(t, e.p.f.owner, `INSERT INTO inbox.conversation_state(tenant_id,store_id,conversation_id,customer_id)
	 VALUES($1,$2,$3,$4)`, e.tenant(), e.store(), conversation, owner)
	selection := oaFactsSelection(e)
	selection["for"] = map[string]any{"bundle_ids": []string{}, "conversation_id": conversation, "customer_id": owner}
	req := oaMaterialRequest(t, e, selection)
	svc, err := orderattempt.NewService(e.p.f.runtime, oaServiceKeys(t))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ready, erased := make(chan uint32, 1), make(chan error, 1)
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	key := "oa-c1-erasure-" + randomUUID()
	go func() {
		erased <- platform.WithScope(ctx, e.p.f.runtime, e.token(), e.store(), "store:read", func(tx pgx.Tx, s platform.Scope) error {
			if _, err := customers.Erase(ctx, tx, s, e.token(), key, owner); err != nil {
				return err
			}
			ready <- tx.Conn().PgConn().PID()
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	}()
	var erasePID uint32
	select {
	case erasePID = <-ready:
	case err = <-erased:
		t.Fatalf("native erasure prerequisite: %v", err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	admitted := make(chan error, 1)
	go func() {
		_, err := svc.Admit(ctx, e.token(), e.store(), req)
		admitted <- err
	}()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	blocked := false
	var early error
	for !blocked && ctx.Err() == nil {
		err = e.p.f.owner.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity
		 WHERE $1::int=ANY(pg_blocking_pids(pid)) AND wait_event_type='Lock' AND wait_event='advisory')`, erasePID).Scan(&blocked)
		if err != nil {
			t.Fatal(err)
		}
		if !blocked {
			select {
			case early = <-admitted:
				unblock()
				<-erased
				t.Fatalf("C1 reached owner/refusal without the subject barrier: %v", early)
			case <-ticker.C:
			case <-ctx.Done():
			}
		}
	}
	unblock()
	select {
	case err = <-erased:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if !blocked {
		t.Fatal("actual C1 did not wait on the native erasure subject barrier")
	}
	select {
	case err = <-admitted:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	var refusal *orderattempt.AdmissionError
	if !errors.As(err, &refusal) || refusal.Code() != "ERASED" {
		t.Fatalf("post-erasure current C1 subject recheck: %v", err)
	}
	var exact bool
	err = e.p.f.owner.QueryRow(ctx, `SELECT NOT o.active
	 AND EXISTS(SELECT 1 FROM customers.privacy_actions p WHERE p.owner_id=o.id AND p.kind='ERASURE')
	 AND NOT EXISTS(SELECT 1 FROM ops.order_attempts a WHERE a.tenant_id=o.tenant_id AND a.store_id=o.store_id)
	 FROM buyer.owners o WHERE o.id=$1`, owner).Scan(&exact)
	if err != nil || !exact {
		t.Fatalf("erasure/C1 race must leave no attempt identity or private body: exact=%v err=%v", exact, err)
	}
}

func TestOrderAttemptErasureRetainsCommittedCommercialProof(t *testing.T) {
	e := lbuNew(t)
	e.grantCreator("customers:privacy", "customers:read")
	a := oaAllocationAdmit(t, e, oaBeginSelection(t, e, "cash_on_delivery", "duplicate-origins"))
	plan, _ := oaMaterialPlan(t, e, oaServiceKeys(t), a.AttemptID)
	executor := oaExecutorNew(t, e.p.f.runtime, oaServiceKeys(t))
	first, err := executor.Execute(context.Background(), e.token(), e.store(), a.AttemptID, a.Generation)
	if err != nil {
		t.Fatal(err)
	}
	oaExecutorReceipt(t, first, a, "COMMITTED", "")
	before := oaCEAttemptBytes(t, e, a)
	if err = oaCEErase(context.Background(), e, plan.BuyerOwnerID, "oa-retained-"+randomUUID(), nil); err != nil {
		t.Fatalf("native erasure after committed COD: %v", err)
	}
	again, err := executor.Execute(context.Background(), e.token(), e.store(), a.AttemptID, a.Generation)
	if err != nil {
		t.Fatal(err)
	}
	oaExecutorSameReceipt(t, first, again)
	if !bytes.Equal(before, oaCEAttemptBytes(t, e, a)) {
		t.Fatal("erasure changed retained terminal identity/receipt/body metadata")
	}
	var exact bool
	err = e.p.f.owner.QueryRow(context.Background(), `SELECT NOT b.active AND a.state='COMMITTED'
	 AND o.id=a.order_id AND o.owner_id=$2 AND o.commercial_state='AWAITING_COLLECTION'
	 AND o.total_minor=$3 AND o.currency=a.receipt->>'currency' AND r.state='COMMITTED'
	 AND NOT EXISTS(SELECT 1 FROM ops.order_attempt_subjects s WHERE s.attempt_id=a.attempt_id)
	 AND (SELECT sum(delta_allocated) FROM inventory.ledger l WHERE l.checkout_id=a.attempt_id)=5
	 FROM ops.order_attempts a JOIN checkout.orders o ON o.id=a.order_id
	 JOIN inventory.reservations r ON r.id=o.id JOIN buyer.owners b ON b.id=o.owner_id
	 WHERE a.attempt_id=$1`, a.AttemptID, plan.BuyerOwnerID, plan.Calculation.TotalMinor).Scan(&exact)
	if err != nil || !exact {
		t.Fatalf("native retained commercial/stock proof after erasure: exact=%v err=%v", exact, err)
	}
}
