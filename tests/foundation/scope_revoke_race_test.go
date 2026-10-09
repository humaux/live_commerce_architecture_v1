// Purpose: SCOPE-404 deterministic REAL_PG proof of the mid-request grant-revocation race behind the
// intermittent LCU2_404 403 (tests/admin/live-console.spec.ts "real store grant revoked clears ..."): a
// scoped READ COMMITTED transaction opened BEFORE the revoke commits runs its definer/second-fence
// statement on a fresh snapshot that already sees the revoke. identity.principal_holds (0064) requires
// the store:read grant, so the A13 definer raised PT403 -> 403 where LCN03 (contracts/live-console-v1.md
// §12: cross-tenant/store reads return 404 for A1-A16) requires the non-disclosing 404. The shared
// fences (platform.withScopeContext / platform.RequirePermission) must re-resolve and answer
// ErrScopeNotFound only when the principal provably no longer sees the store; every other outcome keeps
// the denial (fail closed, never an upgrade).
// Depends on: the shared foundation fixture (foundation_integration_test.go), lcPrincipal
// (live_claims_test.go), internal/platform, migrations 0003/0064/0128/0165.
// Used by: scripts/dev/test-focused.sh scope suites; the browser gate re-proves the fix end to end.
package foundation_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/platform"
)

// The exact fault of the Go browser harness (browser_live_console_test.go grant_revoke): the store:read
// grant of this synthetic principal/store is deleted on another connection while a scope transaction is
// open. The A13 definer then runs on a snapshot that sees the revoke and its identity.principal_holds
// guard (which requires store:read) raises PT403. LCN03: scope loss is a 404, never a 403.
func TestScopeRevokeMidTransactionBecomesScopeNotFound(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	actor, token := lcPrincipal(t, f, f.tenantA, []string{f.storeA1}, "store:read", "inbox:read")
	var raw []byte
	err := platform.WithScope(ctx, f.runtime, token, f.storeA1, "inbox:read", func(tx pgx.Tx, _ platform.Scope) error {
		mustExec(t, f.owner, `DELETE FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND permission='store:read'`, f.tenantA, f.storeA1, actor)
		return tx.QueryRow(ctx, `SELECT inbox.buyer_panel($1::uuid,NULL::uuid)::text`, randomUUID()).Scan(&raw)
	})
	if !errors.Is(err, platform.ErrScopeNotFound) {
		t.Fatalf("mid-request store:read revoke error = %v (sqlstate %q), want ErrScopeNotFound (LCN03 404)", err, sqlState(err))
	}
}

// The mirror case: the principal keeps the store (store:read intact) but loses the route permission
// mid-transaction. The denial is genuine and must stay 403 — the re-resolve must never rewrite a
// permission denial into a scope loss nor upgrade it.
func TestScopePermissionLossMidTransactionStaysForbidden(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	actor, token := lcPrincipal(t, f, f.tenantA, []string{f.storeA1}, "store:read", "inbox:read")
	var raw []byte
	err := platform.WithScope(ctx, f.runtime, token, f.storeA1, "inbox:read", func(tx pgx.Tx, _ platform.Scope) error {
		mustExec(t, f.owner, `DELETE FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND permission='inbox:read'`, f.tenantA, f.storeA1, actor)
		return tx.QueryRow(ctx, `SELECT inbox.buyer_panel($1::uuid,NULL::uuid)::text`, randomUUID()).Scan(&raw)
	})
	if errors.Is(err, platform.ErrScopeNotFound) || sqlState(err) != "PT403" {
		t.Fatalf("permission-only loss error = %v (sqlstate %q), want the definer PT403 (403 forbidden)", err, sqlState(err))
	}
}

// RequirePermission's second fence, revision-bump-only (what 0089 staff_set_role does): the principal
// still holds the permission, so the mid-request change fails closed with ErrForbidden — a re-resolve
// never upgrades a denial back to success.
func TestRequirePermissionRevisionBumpFailsClosed(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	actor, token := lcPrincipal(t, f, f.tenantA, []string{f.storeA1}, "store:read", "live:read")
	err := platform.WithScope(ctx, f.runtime, token, f.storeA1, "live:read", func(tx pgx.Tx, scope platform.Scope) error {
		mustExec(t, f.owner, `UPDATE identity.memberships SET authz_revision=authz_revision+1 WHERE tenant_id=$1 AND principal_id=$2`, f.tenantA, actor)
		return platform.RequirePermission(ctx, tx, scope, token, "live:read")
	})
	if !errors.Is(err, platform.ErrForbidden) {
		t.Fatalf("revision bump error = %v, want ErrForbidden (fail closed)", err)
	}
}

// RequirePermission's second fence, full revoke (what 0089 staff_remove does: grants gone and revision
// bumped): the re-resolve sees the store is no longer visible and answers ErrScopeNotFound.
func TestRequirePermissionFullRevokeBecomesScopeNotFound(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	actor, token := lcPrincipal(t, f, f.tenantA, []string{f.storeA1}, "store:read", "live:read")
	err := platform.WithScope(ctx, f.runtime, token, f.storeA1, "live:read", func(tx pgx.Tx, scope platform.Scope) error {
		mustExec(t, f.owner, `DELETE FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3`, f.tenantA, f.storeA1, actor)
		mustExec(t, f.owner, `UPDATE identity.memberships SET authz_revision=authz_revision+1 WHERE tenant_id=$1 AND principal_id=$2`, f.tenantA, actor)
		return platform.RequirePermission(ctx, tx, scope, token, "live:read")
	})
	if !errors.Is(err, platform.ErrScopeNotFound) {
		t.Fatalf("full revoke error = %v (sqlstate %q), want ErrScopeNotFound (LCN03 404)", err, sqlState(err))
	}
}

// No grant at all (a fresh request after the revoke): the scope-open resolve already answers the
// non-disclosing 404. Pins the steady-state counterpart of the race cases above.
func TestScopeOpenWithoutAnyGrantIsScopeNotFound(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	_, token := lcPrincipal(t, f, f.tenantA, []string{f.storeA1})
	err := platform.WithScope(ctx, f.runtime, token, f.storeA1, "store:read", func(pgx.Tx, platform.Scope) error {
		t.Fatal("callback executed without any grant")
		return nil
	})
	if !errors.Is(err, platform.ErrScopeNotFound) {
		t.Fatalf("grantless scope error = %v, want ErrScopeNotFound", err)
	}
}
