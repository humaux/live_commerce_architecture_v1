// Purpose: REAL_PG acceptance for order-attempt-v1 A1/A1.1 scope activation and retirement.
// Depends on: runtime scope entry, identity sessions, native scope guards and pgx transactions.
// Used by: OA1 CI before admission/lookup integration; synthetic owner SQL prepares lifecycle only.
// Invariants: fresh scope after retirement, one active scope, principal-lock serialization,
// no pending retirement, immutable retired identity, no runtime/coordinator identity DML.
package foundation_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func oaRetirementScope(t *testing.T, tx pgx.Tx, token, store string) oaScopeReceipt {
	t.Helper()
	var raw []byte
	// Calls ops.read_order_attempt_scope (order-attempt-v1 §2/A1) as the actual runtime login.
	if err := tx.QueryRow(context.Background(), `SELECT ops.read_order_attempt_scope($1,$2)`, tokenHash(token), store).Scan(&raw); err != nil {
		t.Fatalf("runtime scope read: %v", err)
	}
	var out oaScopeReceipt
	if err := json.Unmarshal(raw, &out); err != nil || out.ScopeID == "" || out.Generation != 0 || out.AttemptID != nil {
		t.Fatalf("fresh active scope sentinel: %+v err=%v", out, err)
	}
	return out
}

func TestOrderAttemptScopeRetirementReenable(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	principal, token := lcPrincipal(t, f, f.tenantA, []string{f.storeA1}, "store:read", "orders:read")
	tx, err := f.runtime.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	old := oaRetirementScope(t, tx, token, f.storeA1)
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	// Synthetic lifecycle writer takes the real principal row before the scope row.
	lifecycle, err := f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer lifecycle.Rollback(ctx)
	oaStateExec(t, lifecycle, `UPDATE identity.principals SET active=false WHERE id=$1`, principal)
	oaStateExec(t, lifecycle, `UPDATE ops.order_attempt_scopes SET retired_at=clock_timestamp() WHERE scope_id=$1`, old.ScopeID)
	if err := lifecycle.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	_, err = f.runtime.Exec(ctx, `SELECT ops.read_order_attempt_scope($1,$2)`, tokenHash(token), f.storeA1)
	var pgerr *pgconn.PgError
	if !errors.As(err, &pgerr) || pgerr.Code != "PT401" {
		t.Fatalf("inactive principal disclosed scope: %v", err)
	}
	mustExec(t, f.owner, `UPDATE identity.principals SET active=true WHERE id=$1`, principal)
	tx, err = f.runtime.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	fresh := oaRetirementScope(t, tx, token, f.storeA1)
	if fresh.ScopeID == old.ScopeID {
		t.Fatal("A1 reenable reused retired scope identity")
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var active, retired int
	if err := f.owner.QueryRow(ctx, `SELECT count(*) FILTER(WHERE retired_at IS NULL),count(*) FILTER(WHERE retired_at IS NOT NULL)
	 FROM ops.order_attempt_scopes WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3`, f.tenantA, f.storeA1, principal).Scan(&active, &retired); err != nil || active != 1 || retired != 1 {
		t.Fatalf("A1 active/retired lifecycle: active=%d retired=%d err=%v", active, retired, err)
	}
	probe, err := f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer probe.Rollback(ctx)
	for _, assignment := range []string{"retired_at=NULL", "current_generation=1", "scope_id=gen_random_uuid()"} {
		oaStateReject(t, probe, "PT409", `UPDATE ops.order_attempt_scopes SET `+assignment+` WHERE scope_id=$1`, old.ScopeID)
	}
}

func TestOrderAttemptScopeRetirementPendingRefused(t *testing.T) {
	h := oaStateBegin(t)
	// The native row is the authority even when a lifecycle entry has restored
	// caller GUCs. FORCE RLS must not hide a pending child from the trigger.
	oaStateExec(t, h.tx, `SELECT set_config('app.tenant_id','',true),set_config('app.store_id','',true)`)
	oaStateReject(t, h.tx, "PT409", `UPDATE ops.order_attempt_scopes SET retired_at=clock_timestamp() WHERE scope_id=$1`, h.scope)
	oaStateSettle(t, h.tx, h.attempt, "ABORTED")
	oaStateExec(t, h.tx, `UPDATE ops.order_attempt_scopes SET retired_at=clock_timestamp() WHERE scope_id=$1`, h.scope)
	var retired bool
	if err := h.tx.QueryRow(context.Background(), `SELECT retired_at IS NOT NULL FROM ops.order_attempt_scopes WHERE scope_id=$1`, h.scope).Scan(&retired); err != nil || !retired {
		t.Fatalf("terminal retirement positive sentinel: retired=%v err=%v", retired, err)
	}
}

func TestOrderAttemptScopeRetirementPrincipalLockAndACL(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	principal, token := lcPrincipal(t, f, f.tenantA, []string{f.storeA1}, "store:read", "orders:read")
	tx, err := f.runtime.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	oaRetirementScope(t, tx, token, f.storeA1)
	probe, err := f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer probe.Rollback(ctx)
	oaStateReject(t, probe, "55P03", `SELECT id FROM identity.principals WHERE id=$1 FOR UPDATE NOWAIT`, principal)
	var exact, coordinatorDML, lockColumnOnly bool
	err = f.owner.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_proc p JOIN pg_roles r ON r.oid=p.proowner
	 WHERE p.oid=to_regprocedure('identity.lock_order_attempt_principal(bytea,uuid)')
	 AND p.prosecdef AND r.rolname='commerce_auth'
	 AND p.proconfig @> ARRAY['search_path=pg_catalog']::text[]
	 AND ARRAY(SELECT pg_get_userbyid(a.grantee)::text FROM aclexplode(p.proacl) a
	  WHERE a.privilege_type='EXECUTE' AND a.grantee<>p.proowner ORDER BY 1)
	 = ARRAY['commerce_order_attempt_owner']::text[]),
	 has_table_privilege('commerce_order_attempt_owner','identity.principals','SELECT,INSERT,UPDATE,DELETE')
	 OR has_any_column_privilege('commerce_order_attempt_owner','identity.principals','SELECT,INSERT,UPDATE'),
	 has_column_privilege('commerce_auth','identity.principals','id','UPDATE')
	 AND NOT has_table_privilege('commerce_auth','identity.principals','UPDATE')
	 AND NOT EXISTS(SELECT 1 FROM pg_attribute a WHERE a.attrelid='identity.principals'::regclass
	  AND a.attnum>0 AND NOT a.attisdropped AND a.attname<>'id'
	  AND has_column_privilege('commerce_auth',a.attrelid,a.attnum,'UPDATE'))`).Scan(&exact, &coordinatorDML, &lockColumnOnly)
	if err != nil || !exact || coordinatorDML || !lockColumnOnly {
		t.Fatalf("principal lock authority boundary: exact=%v coordinatorDML=%v lockColumnOnly=%v err=%v", exact, coordinatorDML, lockColumnOnly, err)
	}
	_, err = f.runtime.Exec(ctx, `SELECT identity.lock_order_attempt_principal($1,$2)`, tokenHash(token), f.storeA1)
	var pgerr *pgconn.PgError
	if !errors.As(err, &pgerr) || pgerr.Code != "42501" {
		t.Fatalf("runtime reached internal principal lock: %v", err)
	}
}

func TestOrderAttemptScopeRetirementConcurrentActivation(t *testing.T) {
	f := fixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	principal, token := lcPrincipal(t, f, f.tenantA, []string{f.storeA1}, "store:read", "orders:read")
	first, err := f.runtime.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Rollback(context.Background())
	left := oaRetirementScope(t, first, token, f.storeA1)
	second, err := f.runtime.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Rollback(context.Background())
	var pid int
	if err := second.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
		t.Fatal(err)
	}
	type outcome struct {
		raw []byte
		err error
	}
	done := make(chan outcome, 1)
	go func() {
		var out outcome
		out.err = second.QueryRow(ctx, `SELECT ops.read_order_attempt_scope($1,$2)`, tokenHash(token), f.storeA1).Scan(&out.raw)
		done <- out
	}()
	crWaitBlocked(t, f.owner, pid, "second active-scope discovery")
	if err := first.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	got := <-done
	var right oaScopeReceipt
	if got.err != nil || json.Unmarshal(got.raw, &right) != nil || right.ScopeID != left.ScopeID || right.Generation != 0 {
		t.Fatalf("concurrent activation diverged: first=%+v second=%+v err=%v", left, right, got.err)
	}
	if err := second.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := f.owner.QueryRow(ctx, `SELECT count(*) FROM ops.order_attempt_scopes
	 WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND retired_at IS NULL`, f.tenantA, f.storeA1, principal).Scan(&count); err != nil || count != 1 {
		t.Fatalf("concurrent active uniqueness: count=%d err=%v", count, err)
	}
}

func TestOrderAttemptScopeRetirementRechecksAfterPrincipalWait(t *testing.T) {
	f := fixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	principal, token := lcPrincipal(t, f, f.tenantA, []string{f.storeA1}, "store:read", "orders:read")
	lifecycle, err := f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer lifecycle.Rollback(context.Background())
	oaStateExec(t, lifecycle, `SELECT id FROM identity.principals WHERE id=$1 FOR UPDATE`, principal)
	read, err := f.runtime.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer read.Rollback(context.Background())
	var pid int
	if err := read.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := read.Exec(ctx, `SELECT ops.read_order_attempt_scope($1,$2)`, tokenHash(token), f.storeA1)
		done <- err
	}()
	crWaitBlocked(t, f.owner, pid, "activation must lock the actual principal before creating a scope")
	oaStateExec(t, lifecycle, `UPDATE identity.principals SET active=false WHERE id=$1`, principal)
	if err := lifecycle.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var pgerr *pgconn.PgError
	if err := <-done; !errors.As(err, &pgerr) || pgerr.Code != "PT401" {
		t.Fatalf("activation ignored deactivation during principal wait: %v", err)
	}
	var count int
	if err := f.owner.QueryRow(ctx, `SELECT count(*) FROM ops.order_attempt_scopes WHERE principal_id=$1`, principal).Scan(&count); err != nil || count != 0 {
		t.Fatalf("refused activation persisted scope: count=%d err=%v", count, err)
	}
}
