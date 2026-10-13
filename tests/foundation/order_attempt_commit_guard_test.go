// Purpose: REAL_PG commit-boundary and exact-ACL acceptance for OA1 C2 effects.
// Depends: real C1 admission, public execution fence and frozen quote materializer.
// Used by: parent-owned CI; a native terminal fixture is not a C2 executor.
// Invariants: partial/aborted/stale-xid carts cannot commit, and an opaque order
// pointer cannot substitute for a native order with the exact attempt/cart tuple.
package foundation_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestOrderAttemptCommitGuardRejectsTerminalPartials(t *testing.T) {
	for _, mode := range []string{"aborted-cart", "committed-fake-order", "old-xid"} {
		t.Run(mode, func(t *testing.T) {
			e := lbuNew(t)
			a := oaFenceAdmit(t, e)
			ctx := context.Background()
			previous, _ := oaFenceDomainTx(t, e, a, true)
			var oldXID string
			if err := previous.QueryRow(ctx, `SELECT pg_current_xact_id()::text`).Scan(&oldXID); err != nil {
				t.Fatal(err)
			}
			if err := previous.Commit(ctx); err != nil {
				t.Fatalf("lock-only pending positive sentinel: %v", err)
			}
			tx, plain := oaFenceDomainTx(t, e, a, true)
			oaStateExec(t, tx, `SET LOCAL ROLE commerce_order_attempt_owner`)
			quote, _, err := oaQuoteCall(tx, a.AttemptID, plain)
			if err != nil || quote.CartID == "" || quote.QuoteID == "" {
				t.Fatalf("real materialization positive sentinel: %v", err)
			}
			oaStateExec(t, tx, `RESET ROLE`)
			// Native storage-only terminal mutation inside the doomed transaction.
			// No order/Begin stub, disabled trigger or capability is introduced.
			want := "order attempt execution is unfinished"
			if mode == "aborted-cart" {
				oaStateExec(t, tx, `UPDATE ops.order_attempts SET state='ABORTED',plan_ciphertext=NULL,plan_key_version=NULL,
				 settled_at=clock_timestamp(),receipt=jsonb_build_object('state','ABORTED','attempt_id',attempt_id,
				 'intent_generation',intent_generation,'reason','EXPIRED') WHERE attempt_id=$1`, a.AttemptID)
			} else {
				order := randomUUID()
				oaStateExec(t, tx, `UPDATE ops.order_attempts SET state='COMMITTED',order_id=$2::uuid,
				 plan_ciphertext=NULL,plan_key_version=NULL,settled_at=clock_timestamp(),
				 execution_xid=CASE WHEN $3::boolean THEN $4::xid8 ELSE execution_xid END,
				 receipt=jsonb_build_object('state','COMMITTED','attempt_id',attempt_id,'intent_generation',intent_generation,'order_id',$2::uuid)
				 WHERE attempt_id=$1`, a.AttemptID, order, mode == "old-xid", oldXID)
				if mode == "committed-fake-order" {
					want = "order attempt order is missing"
				}
			}
			// Definers must re-establish trusted NEW scope when the final trigger fires.
			oaStateExec(t, tx, `SELECT set_config('app.tenant_id','caller-marker',true),set_config('app.store_id','caller-marker',true)`)
			err = tx.Commit(ctx)
			oaFenceCode(t, nil, err, "23514")
			var pgerr *pgconn.PgError
			if !errors.As(err, &pgerr) || pgerr.Message != want {
				t.Fatalf("wrong commit-guard branch: want=%s err=%v", want, err)
			}
			var pending bool
			var effects int
			if err = e.p.f.owner.QueryRow(ctx, `SELECT a.state='PENDING' AND a.plan_ciphertext IS NOT NULL,
			 (SELECT count(*) FROM storefront.carts WHERE create_attempt_id=a.attempt_id)
			 +(SELECT count(*) FROM storefront.quotes WHERE create_attempt_id=a.attempt_id)
			 +(SELECT count(*) FROM storefront.cart_lines WHERE create_attempt_id=a.attempt_id)
			 +(SELECT count(*) FROM storefront.events WHERE create_attempt_id=a.attempt_id)
			 +(SELECT count(*) FROM checkout.orders WHERE create_attempt_id=a.attempt_id)
			 FROM ops.order_attempts a WHERE a.attempt_id=$1`, a.AttemptID).Scan(&pending, &effects); err != nil || !pending || effects != 0 {
				t.Fatalf("failed commit left effects: pending=%v effects=%d err=%v", pending, effects, err)
			}
		})
	}
}

func TestOrderAttemptCommitGuardExactACL(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	for _, tc := range []struct {
		signature string
		owner     string
		result    string
		grantees  []string
	}{
		{"ops.check_order_attempt_execution_commit()", "commerce_order_attempt_owner", "trigger", []string{}},
		{"checkout.order_attempt_order_matches(uuid,uuid,uuid,uuid,uuid,uuid)", "commerce_checkout_writer", "bool", []string{"commerce_order_attempt_owner"}},
	} {
		var safe bool
		err := f.owner.QueryRow(ctx, `SELECT p.prosecdef AND p.provolatile='v' AND p.prorettype=$3::regtype
		 AND r.rolname=$2 AND NOT (r.rolcanlogin OR r.rolsuper OR r.rolbypassrls)
		 AND p.proconfig=ARRAY['search_path=pg_catalog']::text[] AND length(obj_description(p.oid,'pg_proc'))>0
		 AND ARRAY(SELECT pg_get_userbyid(a.grantee)::text FROM aclexplode(p.proacl) a WHERE a.grantee<>p.proowner AND a.privilege_type='EXECUTE' ORDER BY 1)=$4::text[]
		 AND NOT EXISTS(SELECT 1 FROM aclexplode(p.proacl) a WHERE a.grantee<>p.proowner AND a.is_grantable)
		 FROM pg_proc p JOIN pg_roles r ON r.oid=p.proowner WHERE p.oid=to_regprocedure($1)`, tc.signature, tc.owner, tc.result, tc.grantees).Scan(&safe)
		if err != nil || !safe {
			t.Fatalf("missing/unsafe commit guard ACL %s: safe=%v err=%v", tc.signature, safe, err)
		}
	}
	var deferred, narrow bool
	if err := f.owner.QueryRow(ctx, `SELECT tgdeferrable AND tginitdeferred AND tgtype=5
	 AND tgfoid='ops.check_order_attempt_execution_commit()'::regprocedure AND tgqual IS NOT NULL
	 FROM pg_trigger WHERE tgrelid='storefront.carts'::regclass AND tgname='order_attempt_execution_commit'`).Scan(&deferred); err != nil || !deferred {
		t.Fatalf("missing deferred INSERT row guard: %v", err)
	}
	if err := f.owner.QueryRow(ctx, `SELECT NOT has_any_column_privilege('commerce_order_attempt_owner','checkout.orders','SELECT,INSERT,UPDATE,REFERENCES')
	 AND NOT has_table_privilege('commerce_order_attempt_owner','checkout.orders','DELETE,TRUNCATE,TRIGGER')`).Scan(&narrow); err != nil || !narrow {
		t.Fatalf("coordinator received broad checkout table authority: %v", err)
	}
	for _, role := range []string{"commerce_runtime", "commerce_claims_worker"} {
		pool := miPool(t, f, role)
		var allowed bool
		err := pool.QueryRow(ctx, `SELECT checkout.order_attempt_order_matches($1,$2,$3,$4,$5,$6)`, randomUUID(), randomUUID(), randomUUID(), randomUUID(), randomUUID(), randomUUID()).Scan(&allowed)
		oaFenceCode(t, nil, err, "42501")
	}
}
