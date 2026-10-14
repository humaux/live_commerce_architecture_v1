// Purpose: REAL_PG acceptance for metadata-only OA1 abort and its owner prefix.
// Depends on: real C1 Service.Admit, native provenance tables, runtime/claims logins and execution fence.
// Used by: parent CI red/green; owner injections below create isolated partial-effect counterexamples.
// Invariants: I01/I02/I04/I11/I14; exact current-xid fence, zero effects before seal,
// retained immutable receipt, released admission artifacts, no scope/barrier locks or plan decryption.
// PLAN_INVALID is an authenticated trusted-classification protocol fixture for
// receipt/atomicity checks, not proof of an actual codec error or permanent key loss.
package foundation_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"livecommerce/internal/orderattempt"
)

type oaAbortWire struct {
	State   string          `json:"state"`
	Receipt json.RawMessage `json:"receipt"`
}

func oaAbortCleanupTx(t *testing.T, tx pgx.Tx) {
	t.Helper()
	// Register before any assertion: Fatal must release pool connections too.
	// Cleanup outlives the test deadline, but never waits without a bound.
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := tx.Rollback(ctx); err != nil && err != pgx.ErrTxClosed {
			t.Errorf("abort fixture transaction cleanup: %v", err)
		}
	})
}

func oaAbortCall(ctx context.Context, tx pgx.Tx, hash []byte, store any, attempt string, generation int64, code string) ([]byte, error) {
	var raw []byte
	// Calls ops.abort_order_attempt, order-attempt-v1 §§1/3: fence before any domain lock/effect.
	err := tx.QueryRow(ctx, `SELECT ops.abort_order_attempt($1::bytea,$2::uuid,$3::uuid,$4::bigint,$5::text)`, hash, store, attempt, generation, code).Scan(&raw)
	return raw, err
}

func oaAbortReceipt(t *testing.T, raw []byte, a orderattempt.AdmissionReceipt, code string) oaAbortWire {
	t.Helper()
	var out oaAbortWire
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	var receipt map[string]json.RawMessage
	if err := json.Unmarshal(out.Receipt, &receipt); err != nil {
		t.Fatal(err)
	}
	var state, attempt, reason string
	var generation int64
	_ = json.Unmarshal(receipt["state"], &state)
	_ = json.Unmarshal(receipt["attempt_id"], &attempt)
	_ = json.Unmarshal(receipt["code"], &reason)
	_ = json.Unmarshal(receipt["intent_generation"], &generation)
	if out.State != "ABORTED" || state != out.State || attempt != a.AttemptID || generation != a.Generation || reason != code || len(receipt) != 4 {
		t.Fatalf("terminal receipt must have only fixed state/attempt/generation/code: %s", raw)
	}
	return out
}

func oaAbortPending(t *testing.T, e *lbuEnv, attempt string) {
	t.Helper()
	var pending bool
	err := e.p.f.owner.QueryRow(context.Background(), `SELECT state='PENDING' AND order_id IS NULL AND receipt IS NULL
	 AND plan_ciphertext IS NOT NULL AND plan_key_version IS NOT NULL FROM ops.order_attempts WHERE attempt_id=$1`, attempt).Scan(&pending)
	if err != nil || !pending {
		t.Fatalf("refusal changed admitted pending authority: %v/%v", pending, err)
	}
}

func oaAbortAsRuntime(t *testing.T, e *lbuEnv, tx pgx.Tx) {
	t.Helper()
	var login string
	if err := e.p.f.runtime.QueryRow(context.Background(), `SELECT session_user::text`).Scan(&login); err != nil {
		t.Fatal(err)
	}
	oaStateExec(t, tx, `SET LOCAL SESSION AUTHORIZATION `+pgx.Identifier{login}.Sanitize())
	var actual bool
	if err := tx.QueryRow(context.Background(), `SELECT session_user=current_user AND pg_has_role(session_user,'commerce_runtime','MEMBER')`).Scan(&actual); err != nil || !actual {
		t.Fatal("abort action must use actual runtime session, not owner authority")
	}
}

func TestOrderAttemptAbortNoKeyReceiptAndReplay(t *testing.T) {
	e := lbuNew(t)
	a := oaFenceAdmit(t, e)
	ctx := context.Background()
	var before []byte
	if err := e.p.f.owner.QueryRow(ctx, `SELECT to_jsonb(a)-ARRAY['execution_xid','state','receipt','settled_at','plan_ciphertext','plan_key_version']::text[]
	 FROM ops.order_attempts a WHERE attempt_id=$1`, a.AttemptID).Scan(&before); err != nil {
		t.Fatal(err)
	}
	// An unrelated local ring cannot open C1; this is only a decryption-independent
	// settlement sentinel. The inactive owner below supplies the native ERASED reason.
	secret := make([]byte, 32)
	secret[0] = 99
	missing, err := orderattempt.NewPlanKeys("unrelated", map[string][]byte{"unrelated": secret})
	if err != nil {
		t.Fatal(err)
	}
	seed, err := e.p.f.runtime.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	oaAbortCleanupTx(t, seed)
	raw, err := oaFenceCall(ctx, seed, tokenHash(e.token()), e.store(), a.AttemptID, a.Generation)
	if err != nil {
		t.Fatal(err)
	}
	got := oaFenceOpen(t, raw, a)
	if _, err = missing.Open(*got.Binding, *got.Envelope); err == nil {
		t.Fatal("missing-key positive sentinel unexpectedly decrypted C1")
	}
	if err = seed.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	mustExec(t, e.p.f.owner, `UPDATE buyer.owners SET active=false WHERE id=(SELECT buyer_owner_id FROM ops.order_attempts WHERE attempt_id=$1)`, a.AttemptID)
	tx, err := e.p.f.runtime.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	oaAbortCleanupTx(t, tx)
	oaStateExec(t, tx, `SELECT set_config('app.tenant_id','caller-marker',true),set_config('app.store_id','caller-marker',true),
	 set_config('app.principal_id','caller-marker',true),set_config('app.buyer_id','caller-marker',true),set_config('app.buyer_session_id','caller-marker',true)`)
	raw, err = oaAbortCall(ctx, tx, tokenHash(e.token()), e.store(), a.AttemptID, a.Generation, "ERASED")
	if err != nil {
		t.Fatalf("metadata-only erasure abort without local decryption: %v", err)
	}
	first := oaAbortReceipt(t, raw, a, "ERASED")
	var restored bool
	if err = tx.QueryRow(ctx, `SELECT current_setting('app.tenant_id')='caller-marker' AND current_setting('app.store_id')='caller-marker'
	 AND current_setting('app.principal_id')='caller-marker' AND current_setting('app.buyer_id')='caller-marker' AND current_setting('app.buyer_session_id')='caller-marker'`).Scan(&restored); err != nil || !restored {
		t.Fatalf("abort leaked domain context: %v/%v", restored, err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var after []byte
	var subjects, carts, orders, reservations, ledger int
	var shredded bool
	err = e.p.f.owner.QueryRow(ctx, `SELECT to_jsonb(a)-ARRAY['execution_xid','state','receipt','settled_at','plan_ciphertext','plan_key_version']::text[],
	 a.plan_ciphertext IS NULL AND a.plan_key_version IS NULL AND a.order_id IS NULL,
	 (SELECT count(*) FROM ops.order_attempt_subjects WHERE attempt_id=a.attempt_id),
	 (SELECT count(*) FROM storefront.carts WHERE create_attempt_id=a.attempt_id),
	 (SELECT count(*) FROM checkout.orders WHERE create_attempt_id=a.attempt_id),
	 (SELECT count(*) FROM inventory.reservations WHERE create_attempt_id=a.attempt_id),
	 (SELECT count(*) FROM inventory.ledger WHERE create_attempt_id=a.attempt_id)
	 FROM ops.order_attempts a WHERE attempt_id=$1`, a.AttemptID).Scan(&after, &shredded, &subjects, &carts, &orders, &reservations, &ledger)
	if err != nil || !bytes.Equal(before, after) || !shredded || subjects+carts+orders+reservations+ledger != 0 {
		t.Fatalf("abort changed identity or left effects/subject index: shredded=%v counts=%v err=%v", shredded, []int{subjects, carts, orders, reservations, ledger}, err)
	}
	// Exact terminal replay is resolved before validating a new reason; the lost
	// acknowledgement must not rewrite the sealed receipt or need old keys.
	replay, err := e.p.f.runtime.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	oaAbortCleanupTx(t, replay)
	raw, err = oaAbortCall(ctx, replay, tokenHash(e.token()), e.store(), a.AttemptID, a.Generation, "unlisted-new-reason")
	if err != nil || !bytes.Equal(oaAbortReceipt(t, raw, a, "ERASED").Receipt, first.Receipt) {
		t.Fatalf("terminal no-op replay changed receipt: %v", err)
	}
}

func TestOrderAttemptAbortReasonsAndCurrentReadAuthority(t *testing.T) {
	for _, mode := range []string{"unknown", "retryable", "early-expiry", "active-erasure", "wrong-generation", "wrong-store", "foreign-principal", "read-revoked", "runtime-no-auth"} {
		t.Run(mode, func(t *testing.T) {
			e := lbuNew(t)
			a := oaFenceAdmit(t, e)
			ctx := context.Background()
			hash, store, generation, reason, want := tokenHash(e.token()), e.store(), a.Generation, "PLAN_INVALID", "PT400"
			switch mode {
			case "unknown":
				reason = "unlisted"
			case "retryable":
				reason = "INTERNAL_RETRYABLE"
			case "early-expiry":
				reason, want = "EXPIRED", "PT409"
			case "active-erasure":
				reason, want = "ERASED", "PT409"
			case "wrong-generation":
				generation, want = generation+1, "PT409"
			case "wrong-store":
				store, want = e.p.f.storeA2, "PT404"
			case "foreign-principal":
				_, token := lcPrincipal(t, e.p.f, e.tenant(), []string{e.store()}, "orders:read", "store:read")
				hash, want = tokenHash(token), "PT404"
			case "read-revoked":
				mustExec(t, e.p.f.owner, `DELETE FROM identity.store_grants WHERE principal_id=$1 AND store_id=$2 AND permission='orders:read'`, e.p.f.principalA, e.store())
				want = "PT403"
			case "runtime-no-auth":
				hash = nil
			}
			tx, err := e.p.f.runtime.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			oaAbortCleanupTx(t, tx)
			raw, err := oaAbortCall(ctx, tx, hash, store, a.AttemptID, generation, reason)
			oaFenceCode(t, raw, err, want)
			if err = tx.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
			oaAbortPending(t, e, a.AttemptID)
		})
	}
}

func TestOrderAttemptAbortIndexedOwnerPrefixAndWitness(t *testing.T) {
	e := lbuNew(t)
	a := oaFenceAdmit(t, e)
	ctx := context.Background()
	// An explicitly indexed additional contact owner must be fenced too; testing
	// only buyer_owner_id would miss the erasure race this prefix closes.
	other := randomUUID()
	setup, err := e.p.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	oaAbortCleanupTx(t, setup)
	// Native subject guard is a FORCE-RLS definer; owner fixture setup must
	// establish the exact C1 scope for its guarded attempt lookup too.
	oaStateExec(t, setup, `SELECT set_config('app.tenant_id',$1,true),set_config('app.store_id',$2,true)`, e.tenant(), e.store())
	oaStateExec(t, setup, `INSERT INTO buyer.owners(id,tenant_id,store_id) VALUES($1,$2,$3)`, other, e.tenant(), e.store())
	oaStateExec(t, setup, `INSERT INTO ops.order_attempt_subjects(tenant_id,store_id,subject_kind,subject_id,attempt_id) VALUES($1,$2,'customer',$3,$4)`, e.tenant(), e.store(), other, a.AttemptID)
	if err = setup.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	stale, _ := oaFenceDomainTx(t, e, a, true)
	oaAbortCleanupTx(t, stale)
	if err := stale.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	for _, acquire := range []bool{false, true} {
		tx, _ := oaFenceDomainTx(t, e, a, acquire)
		oaAbortCleanupTx(t, tx)
		oaStateExec(t, tx, `SET LOCAL ROLE commerce_order_attempt_owner`)
		if !acquire {
			oaStateReject(t, tx, "PT409", `SELECT buyer.order_attempt_lock_owners($1,$2)`, a.AttemptID, a.Generation)
			oaStateReject(t, tx, "PT409", `SELECT ops.order_attempt_abort_fenced($1,$2,'PLAN_INVALID')`, a.AttemptID, a.Generation)
		} else {
			var status string
			if err := tx.QueryRow(ctx, `SELECT buyer.order_attempt_lock_owners($1,$2)`, a.AttemptID, a.Generation).Scan(&status); err != nil || status != "active" {
				t.Fatalf("exact current-xid owner prefix: %s/%v", status, err)
			}
			oaStateExec(t, tx, `RESET ROLE`)
			probe, err := e.p.f.owner.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			oaAbortCleanupTx(t, probe)
			_, err = probe.Exec(ctx, `SELECT id FROM buyer.owners WHERE id=$1 FOR UPDATE NOWAIT`, other)
			oaJobError(t, err, "55P03")
			_ = probe.Rollback(ctx)
		}
		if err := tx.Rollback(ctx); err != nil {
			t.Fatal(err)
		}
	}
	mustExec(t, e.p.f.owner, `UPDATE buyer.owners SET active=false WHERE id=$1`, other)
	tx, err := e.p.f.runtime.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	oaAbortCleanupTx(t, tx)
	raw, err := oaAbortCall(ctx, tx, tokenHash(e.token()), e.store(), a.AttemptID, a.Generation, "ERASED")
	if err != nil {
		t.Fatalf("inactive indexed contact must permit metadata-only erasure abort: %v", err)
	}
	oaAbortReceipt(t, raw, a, "ERASED")
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestOrderAttemptAbortPartialCartRequiresRollback(t *testing.T) {
	e := lbuNew(t)
	a := oaFenceAdmit(t, e)
	ctx := context.Background()
	tx, _ := oaFenceDomainTx(t, e, a, true)
	oaAbortCleanupTx(t, tx)
	partial, err := tx.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	oaAbortCleanupTx(t, partial)
	// Native same-transaction C2 partial, deliberately injected by fixture owner.
	// No fake quote/Begin implementation or synthetic capability is used.
	oaStateExec(t, partial, `INSERT INTO storefront.carts(tenant_id,store_id,owner_id,currency,version,cart_kind,create_attempt_id)
	 SELECT tenant_id,store_id,buyer_owner_id,'TWD',1,'order_attempt',attempt_id FROM ops.order_attempts WHERE attempt_id=$1`, a.AttemptID)
	oaAbortAsRuntime(t, e, partial)
	probe, err := partial.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	oaAbortCleanupTx(t, probe)
	raw, err := oaAbortCall(ctx, probe, tokenHash(e.token()), e.store(), a.AttemptID, a.Generation, "PLAN_INVALID")
	oaFenceCode(t, raw, err, "PT409")
	if err = probe.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if err = partial.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	oaAbortAsRuntime(t, e, tx)
	raw, err = oaAbortCall(ctx, tx, tokenHash(e.token()), e.store(), a.AttemptID, a.Generation, "PLAN_INVALID")
	if err != nil {
		t.Fatalf("rollback to zero effects must permit sealing same fenced attempt: %v", err)
	}
	oaAbortReceipt(t, raw, a, "PLAN_INVALID")
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var carts int
	if err = e.p.f.owner.QueryRow(ctx, `SELECT count(*) FROM storefront.carts WHERE create_attempt_id=$1`, a.AttemptID).Scan(&carts); err != nil || carts != 0 {
		t.Fatalf("partial cart survived abort: %d/%v", carts, err)
	}
}

func TestOrderAttemptAbortReleasesOnlyOwnAdmission(t *testing.T) {
	e := lbuNew(t)
	a := oaQuoteAdmit(t, e, true)
	ctx := context.Background()
	var ownReservations, ownGrants int
	if err := e.p.f.owner.QueryRow(ctx, `SELECT
	 (SELECT count(*) FROM inbox.order_for_buyer WHERE create_attempt_id=$1 AND state='pending' AND order_id IS NULL),
	 (SELECT count(*) FROM claims.merchant_origin_grants WHERE create_attempt_id=$1 AND quote_id IS NULL AND consumed_at IS NULL)`, a.AttemptID).Scan(&ownReservations, &ownGrants); err != nil || ownReservations != 1 || ownGrants != 1 {
		t.Fatalf("real live C1 reservation/grant sentinel: %d/%d err=%v", ownReservations, ownGrants, err)
	}
	// Another request's unused grant for this same buyer is an independent audit
	// sentinel. The abort must filter exact creator/request rather than owner alone.
	foreign := randomUUID()
	mustExec(t, e.p.f.owner, `INSERT INTO claims.merchant_origin_grants(tenant_id,store_id,request_id,buyer_id,bundle_id,principal_id,expires_at)
	 SELECT tenant_id,store_id,$2,buyer_id,bundle_id,principal_id,expires_at+interval '1 hour' FROM claims.merchant_origin_grants WHERE create_attempt_id=$1`, a.AttemptID, foreign)
	var before []byte
	if err := e.p.f.owner.QueryRow(ctx, `SELECT to_jsonb(g) FROM claims.merchant_origin_grants g WHERE request_id=$1`, foreign).Scan(&before); err != nil {
		t.Fatal(err)
	}
	// Native inactive-owner proof permits ERASED without relying on plan-key availability.
	mustExec(t, e.p.f.owner, `UPDATE buyer.owners SET active=false WHERE id=(SELECT buyer_owner_id FROM ops.order_attempts WHERE attempt_id=$1)`, a.AttemptID)
	tx, err := e.p.f.runtime.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	oaAbortCleanupTx(t, tx)
	raw, err := oaAbortCall(ctx, tx, tokenHash(e.token()), e.store(), a.AttemptID, a.Generation, "ERASED")
	if err != nil {
		t.Fatal(err)
	}
	oaAbortReceipt(t, raw, a, "ERASED")
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var released, expired int
	var after []byte
	err = e.p.f.owner.QueryRow(ctx, `SELECT
	 (SELECT count(*) FROM inbox.order_for_buyer WHERE create_attempt_id=$1 AND request_id=$1 AND state='released' AND order_id IS NULL),
	 (SELECT count(*) FROM claims.merchant_origin_grants WHERE create_attempt_id=$1 AND request_id=$1 AND quote_id IS NULL AND consumed_at IS NULL AND expires_at<=clock_timestamp()),
	 (SELECT to_jsonb(g) FROM claims.merchant_origin_grants g WHERE request_id=$2)`, a.AttemptID, foreign).Scan(&released, &expired, &after)
	if err != nil || released != ownReservations || expired != ownGrants || !bytes.Equal(before, after) {
		t.Fatalf("release/expiry/audit isolation: %d/%d err=%v", released, expired, err)
	}
}

func TestOrderAttemptAbortWorkerNoEarlierLocksAndACL(t *testing.T) {
	e := lbuNew(t)
	a := oaFenceAdmit(t, e)
	mustExec(t, e.p.f.owner, `UPDATE buyer.owners SET active=false WHERE id=(SELECT buyer_owner_id FROM ops.order_attempts WHERE attempt_id=$1)`, a.AttemptID)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	holder, err := e.p.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	oaAbortCleanupTx(t, holder)
	oaStateExec(t, holder, `SELECT scope_id FROM ops.order_attempt_scopes WHERE scope_id=$1 FOR UPDATE`, a.ScopeID)
	oaStateExec(t, holder, `SELECT id FROM identity.principals WHERE id=$1 FOR UPDATE`, e.p.f.principalA)
	oaStateExec(t, holder, `SELECT pg_advisory_xact_lock(hashtextextended('order-attempt-subject:'||tenant_id::text||':'||store_id::text||':'||subject_kind||':'||subject_id::text,0))
	 FROM ops.order_attempt_subjects WHERE attempt_id=$1 ORDER BY subject_kind,subject_id`, a.AttemptID)
	worker := miPool(t, e.p.f, "commerce_claims_worker")
	tx, err := worker.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	oaAbortCleanupTx(t, tx)
	oaStateExec(t, tx, `SET LOCAL lock_timeout='750ms'`)
	var actual bool
	if err = tx.QueryRow(ctx, `SELECT session_user=current_user AND pg_has_role(session_user,'commerce_claims_worker','MEMBER')`).Scan(&actual); err != nil || !actual {
		t.Fatal("worker abort did not use actual claims session")
	}
	raw, err := oaAbortCall(ctx, tx, nil, nil, a.AttemptID, a.Generation, "ERASED")
	if err != nil {
		var native *pgconn.PgError
		if errors.As(err, &native) {
			t.Logf("worker abort native failure: SQLSTATE=%s where=%s", native.Code, native.Where)
		}
		t.Fatalf("worker NULL-store abort must not lock scope/principal/subject barriers: %v", err)
	}
	oaAbortReceipt(t, raw, a, "ERASED")
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if err = holder.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	for _, signature := range []string{"ops.order_attempt_abort_fenced(uuid,bigint,text)", "ops.order_attempt_subject_context(uuid,bigint)", "buyer.order_attempt_lock_owners(uuid,bigint)", "claims.order_attempt_release_admission(uuid,bigint)"} {
		var private bool
		err := e.p.f.owner.QueryRow(ctx, `SELECT p.prosecdef AND p.proconfig=ARRAY['search_path=pg_catalog']::text[]
		 AND NOT EXISTS(SELECT 1 FROM aclexplode(p.proacl) x WHERE x.grantee=0)
		 AND NOT has_function_privilege('commerce_runtime',p.oid,'EXECUTE')
		 AND NOT has_function_privilege('commerce_claims_worker',p.oid,'EXECUTE')
		 FROM pg_proc p WHERE p.oid=to_regprocedure($1)`, signature).Scan(&private)
		if err != nil || !private {
			t.Fatalf("private abort/prefix ACL sentinel %s: %v/%v", signature, private, err)
		}
	}
	for _, role := range []string{"commerce_runtime", "commerce_claims_worker"} {
		pool := e.p.f.runtime
		if role == "commerce_claims_worker" {
			pool = worker
		}
		_, err := pool.Exec(ctx, `SELECT ops.order_attempt_abort_fenced($1,1,'ERASED')`, a.AttemptID)
		oaJobError(t, err, "42501")
	}
	var public bool
	err = e.p.f.owner.QueryRow(ctx, `SELECT p.prosecdef AND p.proconfig=ARRAY['search_path=pg_catalog']::text[] AND p.proowner='commerce_order_attempt_owner'::regrole
	 AND ARRAY(SELECT pg_get_userbyid(x.grantee)::text FROM aclexplode(p.proacl) x WHERE x.grantee<>p.proowner AND x.privilege_type='EXECUTE' ORDER BY 1)
	 =ARRAY['commerce_claims_worker','commerce_runtime']::text[] AND NOT EXISTS(SELECT 1 FROM aclexplode(p.proacl) x WHERE x.grantee=0)
	 FROM pg_proc p WHERE p.oid=to_regprocedure('ops.abort_order_attempt(bytea,uuid,uuid,bigint,text)')`).Scan(&public)
	if err != nil || !public {
		t.Fatalf("public abort exact ACL: %v/%v", public, err)
	}
}

func TestOrderAttemptAbortBoundOrConsumedAdmissionRefused(t *testing.T) {
	for _, anomaly := range []string{"bound", "consumed", "placed"} {
		t.Run(anomaly, func(t *testing.T) {
			e := lbuNew(t)
			a := oaQuoteAdmit(t, e, true)
			ctx := context.Background()
			tx, _ := oaFenceDomainTx(t, e, a, true)
			oaAbortCleanupTx(t, tx)
			partial, err := tx.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			oaAbortCleanupTx(t, partial)
			// Native audit rows intentionally contain a same-Tx partial. Abort
			// must reject it rather than release, erase or compensate its evidence.
			switch anomaly {
			case "bound":
				oaStateExec(t, partial, `UPDATE claims.merchant_origin_grants SET quote_id=$2 WHERE create_attempt_id=$1`, a.AttemptID, randomUUID())
			case "consumed":
				oaStateExec(t, partial, `UPDATE claims.merchant_origin_grants SET quote_id=$2,consumed_at=clock_timestamp() WHERE create_attempt_id=$1`, a.AttemptID, randomUUID())
			case "placed":
				oaStateExec(t, partial, `UPDATE inbox.order_for_buyer SET state='placed',order_id=$2 WHERE create_attempt_id=$1`, a.AttemptID, randomUUID())
			}
			oaAbortAsRuntime(t, e, partial)
			probe, err := partial.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			oaAbortCleanupTx(t, probe)
			raw, err := oaAbortCall(ctx, probe, tokenHash(e.token()), e.store(), a.AttemptID, a.Generation, "PLAN_INVALID")
			oaFenceCode(t, raw, err, "PT409")
			if err = probe.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
			if err = partial.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
			oaAbortAsRuntime(t, e, tx)
			raw, err = oaAbortCall(ctx, tx, tokenHash(e.token()), e.store(), a.AttemptID, a.Generation, "PLAN_INVALID")
			if err != nil {
				t.Fatalf("unchanged pending admission after partial rollback: %v", err)
			}
			oaAbortReceipt(t, raw, a, "PLAN_INVALID")
			if err = tx.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
			oaAbortPending(t, e, a.AttemptID)
		})
	}
}

func TestOrderAttemptAbortExpiredNativeMetadata(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	tx, err := f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	oaAbortCleanupTx(t, tx)
	attempt, scope, owner, principal := randomUUID(), randomUUID(), randomUUID(), randomUUID()
	// Storage-only past-seeded metadata proves the DB deadline boundary without
	// altering immutable C1 rows/triggers or waiting fifteen minutes. Independent
	// tests above use real Service.Admit; this fixture claims no C1/crypto success.
	// The synthetic request digest also pins leading-zero byte decoding: one
	// two-digit byte plus 31 low bytes must remain 32 bytes, never odd hex.
	// This committed synthetic scope belongs to its own principal; an ABORTED
	// attempt correctly leaves its active scope intact for the next explicit CAS.
	oaStateExec(t, tx, `INSERT INTO identity.principals(id) VALUES($1)`, principal)
	oaStateExec(t, tx, `INSERT INTO identity.memberships(tenant_id,principal_id) VALUES($1,$2)`, f.tenantA, principal)
	oaStateExec(t, tx, `SELECT set_config('app.tenant_id',$1,true),set_config('app.store_id',$2,true),set_config('app.principal_id',$3,true)`, f.tenantA, f.storeA1, principal)
	oaStateExec(t, tx, `INSERT INTO buyer.owners(id,tenant_id,store_id) VALUES($1,$2,$3)`, owner, f.tenantA, f.storeA1)
	oaStateExec(t, tx, `INSERT INTO ops.order_attempt_creators(attempt_id,tenant_id,store_id,buyer_owner_id) VALUES($1,$2,$3,$4)`, attempt, f.tenantA, f.storeA1, owner)
	oaStateExec(t, tx, `INSERT INTO ops.order_attempt_scopes(scope_id,tenant_id,store_id,principal_id,current_generation) VALUES($1,$2,$3,$4,1)`, scope, f.tenantA, f.storeA1, principal)
	oaStateExec(t, tx, `INSERT INTO ops.order_attempts(attempt_id,tenant_id,store_id,scope_id,intent_generation,
	 transport_key_digest,request_hash,request_schema_version,buyer_owner_id,admission_provenance,admitted_at,execute_by,state,enqueue_xid)
	 VALUES($1,$2,$3,$4,1,sha256($1::uuid::text::bytea),decode('10'||repeat('01',31),'hex'),1,$5,
	 jsonb_build_object('principal_id',$6::text),statement_timestamp()-interval '16 minutes',statement_timestamp()-interval '1 minute','PENDING',pg_current_xact_id())`, attempt, f.tenantA, f.storeA1, scope, owner, principal)
	oaJobInsert(t, tx, attempt)
	oaJobFinish(t, tx, attempt)
	oaStateExec(t, tx, `INSERT INTO ops.order_attempt_subjects(tenant_id,store_id,subject_kind,subject_id,attempt_id) VALUES($1,$2,'buyer_owner',$3,$4)`, f.tenantA, f.storeA1, owner, attempt)
	oaStateExec(t, tx, `SET CONSTRAINTS ALL IMMEDIATE`)
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	worker := miPool(t, f, "commerce_claims_worker")
	execution, err := worker.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	oaAbortCleanupTx(t, execution)
	raw, err := oaAbortCall(ctx, execution, nil, nil, attempt, 1, "EXPIRED")
	if err != nil {
		t.Fatalf("expired metadata abort must not require plaintext/key or active deadline: %v", err)
	}
	oaAbortReceipt(t, raw, orderattempt.AdmissionReceipt{AttemptID: attempt, ScopeID: scope, Generation: 1}, "EXPIRED")
	if err = execution.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}
