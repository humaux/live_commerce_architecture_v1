// Purpose: REAL_PG acceptance of private OA1 erasure authority and retained proof.
// Depends on: actual customer erasure, C1/Executor and exact native function ACLs.
// Used by: parent CI; no owner-injected terminal state or local execution.
// Invariants: privacy has EXECUTE only, terminal commercial proof survives erasure.
package foundation_test

import (
	"bytes"
	"context"
	"testing"
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
	oaFenceCode(t, tx, "42501", `SELECT ops.order_attempt_erase_prefix($1,$2,$3)`, e.tenant(), e.store(), randomUUID())
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
		oaFenceCode(t, ownerTx, "PT400", query)
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
	oaFenceCode(t, ownerTx, "PT403", `SELECT ops.order_attempt_erase_prefix($1,$2,$3)`, e.tenant(), e.store(), randomUUID())
	if err = ownerTx.Rollback(context.Background()); err != nil {
		t.Fatal(err)
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
	 AND o.total_minor=$3 AND o.currency=$4 AND r.state='COMMITTED'
	 AND NOT EXISTS(SELECT 1 FROM ops.order_attempt_subjects s WHERE s.attempt_id=a.attempt_id)
	 AND (SELECT sum(delta_allocated) FROM inventory.ledger l WHERE l.checkout_id=a.attempt_id)=5
	 FROM ops.order_attempts a JOIN checkout.orders o ON o.id=a.order_id
	 JOIN inventory.reservations r ON r.id=o.id JOIN buyer.owners b ON b.id=o.owner_id
	 WHERE a.attempt_id=$1`, a.AttemptID, plan.BuyerOwnerID, plan.Calculation.TotalMinor, plan.Calculation.Currency).Scan(&exact)
	if err != nil || !exact {
		t.Fatalf("native retained commercial/stock proof after erasure: exact=%v err=%v", exact, err)
	}
}
