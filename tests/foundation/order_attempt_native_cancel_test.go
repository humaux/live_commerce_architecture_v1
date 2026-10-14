// Purpose: native OA1 cancel order-lock, credential revocation and private ACL gates.
// Depends on: real C1/Executor, existing cancellation APIs and PostgreSQL barriers.
// Used by: parent CI alongside unchanged cancel/erasure and legacy fulfillment tests.
// Invariants: no earlier attempt/scope/owner lock, same order advisory, immutable
// credential material/deadline and receipt; only cancellation revocation metadata changes.
package foundation_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestOrderAttemptNativeCancelOrderFenceAndRevocation(t *testing.T) {
	for _, mode := range []string{"pay_at_pickup", "cash_on_delivery", "bank_transfer", "direct_pickup"} {
		t.Run(mode, func(t *testing.T) {
			e := lbuNew(t)
			e.grantCreator("fulfillment:write")
			payment := mode
			if mode == "direct_pickup" {
				payment = "pay_at_pickup"
			}
			a := oaAllocationAdmit(t, e, oaBeginSelection(t, e, payment, "duplicate-origins"))
			executor := oaExecutorNew(t, e.p.f.runtime, oaServiceKeys(t))
			first, err := executor.Execute(context.Background(), e.token(), e.store(), a.AttemptID, a.Generation)
			if err != nil {
				t.Fatal(err)
			}
			oaExecutorReceipt(t, first, a, "COMMITTED", "")
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			var before string
			if err = e.p.f.owner.QueryRow(ctx, `SELECT (to_jsonb(p)-'revoked_at')::text FROM checkout.order_pay_entries p
			 WHERE create_attempt_id=$1 AND revoked_at IS NULL`, a.AttemptID).Scan(&before); err != nil {
				t.Fatal("positive original credential", err)
			}
			earlier, err := e.p.f.owner.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			oaAbortCleanupTx(t, earlier)
			oaStateExec(t, earlier, `SELECT scope_id FROM ops.order_attempt_scopes WHERE scope_id=$1 FOR UPDATE`, a.ScopeID)
			oaStateExec(t, earlier, `SELECT attempt_id FROM ops.order_attempts WHERE attempt_id=$1 FOR UPDATE`, a.AttemptID)
			oaStateExec(t, earlier, `SELECT b.id FROM buyer.owners b JOIN checkout.orders o ON o.owner_id=b.id
			 WHERE o.id=$1 FOR UPDATE OF b`, a.AttemptID)
			holder, err := e.p.f.owner.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			oaAbortCleanupTx(t, holder)
			oaStateExec(t, holder, `SELECT pg_advisory_xact_lock(hashtextextended('checkout.order|'||$1::text||'|'||$2::text||'|'||$3::text,0))`, e.tenant(), e.store(), a.AttemptID)
			tx, err := e.p.f.runtime.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			oaAbortCleanupTx(t, tx)
			oaStateExec(t, tx, `SET LOCAL lock_timeout='1s'`)
			state := "CONFIRMED"
			if payment == "cash_on_delivery" {
				state = "AWAITING_COLLECTION"
			} else if payment == "bank_transfer" {
				state = "AWAITING_TRANSFER"
			}
			done := make(chan error, 1)
			finished := make(chan struct{})
			go func() {
				defer close(finished)
				var raw []byte
				var callErr error
				if mode == "direct_pickup" {
					callErr = tx.QueryRow(ctx, `SELECT inventory.release_pay_at_pickup($1,$2,$3,$4,$5,'cancel','PENDING')`,
						tokenHash(e.token()), e.store(), a.AttemptID, "oa-native-direct-"+randomUUID(), randomBytes(32)).Scan(&raw)
				} else {
					callErr = tx.QueryRow(ctx, `SELECT fulfillment.merchant_cancel_order($1,$2,$3,$4,$5,$6,'synthetic cancellation')`,
						tokenHash(e.token()), e.store(), a.AttemptID, "oa-native-cancel-"+randomUUID(), randomBytes(32), state).Scan(&raw)
				}
				if callErr == nil {
					callErr = tx.Commit(ctx)
				}
				done <- callErr
			}()
			defer func() {
				_ = holder.Rollback(context.Background())
				_ = earlier.Rollback(context.Background())
				cancel()
				select {
				case <-finished:
				case <-time.After(3 * time.Second):
					t.Error("native cancellation failed to stop")
				}
			}()
			tick := time.NewTicker(5 * time.Millisecond)
			defer tick.Stop()
			for {
				var blocked bool
				if err = e.p.f.owner.QueryRow(ctx, `SELECT coalesce($2::int=ANY(pg_blocking_pids(pid)) AND wait_event='advisory',false)
				 FROM pg_stat_activity WHERE pid=$1`, int32(tx.Conn().PgConn().PID()), int32(holder.Conn().PgConn().PID())).Scan(&blocked); err != nil {
					t.Fatal(err)
				}
				if blocked {
					break
				}
				select {
				case err = <-done:
					t.Fatal("cancel did not acquire canonical order advisory first", err)
				case <-ctx.Done():
					t.Fatal("cancel did not reach native advisory barrier")
				case <-tick.C:
				}
			}
			// An independent transaction can still lock the order: the waiting
			// cancellation must not have taken that row before the advisory.
			probe, err := e.p.f.owner.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			oaAbortCleanupTx(t, probe)
			oaStateExec(t, probe, `SELECT id FROM checkout.orders WHERE id=$1 FOR UPDATE NOWAIT`, a.AttemptID)
			if err = probe.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
			if err = holder.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
			select {
			case err = <-done:
				if err != nil {
					t.Fatal("cancel acquired an earlier lock or rejected exact native provenance", err)
				}
			case <-ctx.Done():
				t.Fatal("cancel blocked behind earlier attempt/scope/owner locks")
			}
			var after string
			var revoked bool
			if err = e.p.f.owner.QueryRow(ctx, `SELECT (to_jsonb(p)-'revoked_at')::text,p.revoked_at IS NOT NULL
			 AND o.commercial_state='CANCELLED' FROM checkout.order_pay_entries p JOIN checkout.orders o ON o.id=p.order_id
			 WHERE p.create_attempt_id=$1`, a.AttemptID).Scan(&after, &revoked); err != nil || !revoked || after != before {
				t.Fatal("cancel must revoke without renewing version/material/lifetime", err)
			}
		})
	}
}

func TestOrderAttemptNativeCancelRevokerExactACL(t *testing.T) {
	e := lbuNew(t)
	var exact bool
	err := e.p.f.owner.QueryRow(context.Background(), `SELECT p.prosecdef AND r.rolname='commerce_checkout_writer'
	 AND p.prorettype='trigger'::regtype AND p.proconfig=ARRAY['search_path=pg_catalog']
	 AND NOT has_function_privilege('commerce_runtime',p.oid,'EXECUTE')
	 AND NOT has_function_privilege('commerce_claims_worker',p.oid,'EXECUTE')
	 AND NOT has_function_privilege('commerce_order_attempt_owner',p.oid,'EXECUTE')
	 AND NOT EXISTS(SELECT 1 FROM aclexplode(p.proacl) a WHERE a.grantee=0 AND a.privilege_type='EXECUTE')
	 AND has_column_privilege('commerce_checkout_writer','checkout.order_pay_entries','revoked_at','UPDATE')
	 AND NOT has_column_privilege('commerce_checkout_writer','checkout.order_pay_entries','expires_at','UPDATE')
	 AND NOT has_column_privilege('commerce_runtime','checkout.order_pay_entries','revoked_at','UPDATE')
	 FROM pg_proc p JOIN pg_roles r ON r.oid=p.proowner
	 WHERE p.oid='checkout.revoke_cancelled_order_attempt_pay_entry()'::regprocedure`).Scan(&exact)
	if err != nil || !exact {
		t.Fatal("native cancellation revoker must be trigger-only with one update column", err)
	}
}

func TestOrderAttemptNativeCancelBankProofReviewRefused(t *testing.T) {
	e := lbuNew(t)
	e.grantCreator("fulfillment:write")
	a := oaAllocationAdmit(t, e, oaBeginSelection(t, e, "bank_transfer", "duplicate-origins"))
	executor := oaExecutorNew(t, e.p.f.runtime, oaServiceKeys(t))
	receipt, err := executor.Execute(context.Background(), e.token(), e.store(), a.AttemptID, a.Generation)
	if err != nil {
		t.Fatal(err)
	}
	oaExecutorReceipt(t, receipt, a, "COMMITTED", "")
	// Defensive database fixture: proof submission for native order-only links
	// belongs to OA2. A pending review must still refuse this OA1 cancel path.
	mustExec(t, e.p.f.owner, `UPDATE checkout.bank_transfers SET state='SUBMITTED',proof_last5='12345',
	 proof_amount_minor=1,proof_paid_at=clock_timestamp(),proof_submitted_at=clock_timestamp(),proof_count=1
	 WHERE order_id=$1`, a.AttemptID)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	read := func() string {
		t.Helper()
		var raw string
		if err := e.p.f.owner.QueryRow(ctx, `SELECT jsonb_build_array(to_jsonb(o),to_jsonb(r),to_jsonb(p),to_jsonb(b),
		 (SELECT count(*) FROM inventory.ledger l WHERE l.checkout_id=o.id))::text
		 FROM checkout.orders o JOIN inventory.reservations r ON r.id=o.id
		 JOIN checkout.order_pay_entries p ON p.order_id=o.id JOIN checkout.bank_transfers b ON b.order_id=o.id
		 WHERE o.id=$1 AND b.state='SUBMITTED'`, a.AttemptID).Scan(&raw); err != nil {
			t.Fatal("positive proof-review native readback", err)
		}
		return raw
	}
	before := read()
	var raw []byte
	err = e.p.f.runtime.QueryRow(ctx, `SELECT fulfillment.merchant_cancel_order($1,$2,$3,$4,$5,'AWAITING_TRANSFER','synthetic cancellation')`,
		tokenHash(e.token()), e.store(), a.AttemptID, "oa-native-review-"+randomUUID(), randomBytes(32)).Scan(&raw)
	var pg *pgconn.PgError
	if !errors.As(err, &pg) || pg.Code != "PT409" || pg.Message != "payment_in_flight" || len(raw) != 0 {
		t.Fatal("proof under review must retain financial refusal without a receipt", err)
	}
	if read() != before {
		t.Fatal("refused bank cancellation changed order, hold, credential, proof or ledger")
	}
}
