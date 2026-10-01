// File: tests/foundation/worker_authority_helpers_test.go
// Purpose: shared names and openers for the five per-process worker authorities of migration 0096 (T21-02/03),
//
//	so every worker-flavoured test logs in as the authority its production process uses instead of the empty
//	legacy commerce_worker role.
//
// Runs as/in: package foundation_test helpers only; no PG state of its own.
// Reads env / secrets: none.
// Used by: every foundation test that opens a worker pool or asserts a worker ACL.
// Status: MOCK-free helper; behaviour is proven by worker_authority_split_test.go (WAS*).
package foundation_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/platform"
)

// The five worker authorities (NOLOGIN roles of migration 0096) and the empty legacy role.
const (
	waPayment = "commerce_payment_worker"
	waLive    = "commerce_payment_live"
	waExpiry  = "commerce_expiry_worker"
	waAds     = "commerce_ads_worker"
	waClaims  = "commerce_claims_worker"
	waLegacy  = "commerce_worker"
)

// waAll lists every worker authority a negative ACL assertion must cover, plus the legacy role (which must stay empty).
var waAll = []string{waPayment, waLive, waExpiry, waAds, waClaims, waLegacy}

// waLegacyWorld reports whether f is a historical-state database from before migration 0096 (no worker authorities yet).
func waLegacyWorld(t *testing.T, f *testFixture) bool {
	t.Helper()
	var legacy bool
	if err := f.owner.QueryRow(context.Background(), `SELECT to_regprocedure('integration.operation_lane(text,text)') IS NULL`).Scan(&legacy); err != nil {
		t.Fatal(err)
	}
	return legacy
}

// waLegacyLogins records, per fixture owner pool, the logins waOpen cloned legacy grants onto, so waResyncLegacy can re-copy them after
// a historical replay created or recreated objects (old SQL grants those to commerce_worker only).
var waLegacyLogins sync.Map // *pgxpool.Pool (owner) -> []string

// waOpen opens a platform worker pool for authority a over a fresh test login in role. On a historical pre-0096 database (the
// "old release" fixtures that replay original migration bytes) the authority role is created bare and the login additionally receives a
// DIRECT copy of everything the old shared commerce_worker holds there (ACL entries and RLS policies), so the pool passes the current
// startup validation and still exercises the old SQL those tests are about; waAfterApply drops the clones after an upgrade.
func waOpen(t *testing.T, f *testFixture, role string, a platform.WorkerAuthority) *pgxpool.Pool {
	t.Helper()
	legacy := waLegacyWorld(t, f)
	if legacy {
		waEnsureRole(t, f.owner, role)
	}
	dsn := bcRole(t, f, role)
	if legacy {
		cfg, err := pgxpool.ParseConfig(dsn)
		if err != nil {
			t.Fatal(err)
		}
		if err := waCloneLegacyGrants(f, cfg.ConnConfig.User); err != nil {
			t.Fatal(err)
		}
		// LIFO: runs before bcRole's DROP ROLE, which would otherwise fail on the cloned grants.
		t.Cleanup(func() {
			_, _ = f.owner.Exec(context.Background(), `DROP OWNED BY `+pgx.Identifier{cfg.ConnConfig.User}.Sanitize())
		})
		prev, _ := waLegacyLogins.Load(f.owner)
		list, _ := prev.([]string)
		waLegacyLogins.Store(f.owner, append(list, cfg.ConnConfig.User))
	}
	pool, err := platform.OpenWorkerPool(context.Background(), dsn, a)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// waCloneLegacyGrants copies every ACL entry and policy role of commerce_worker onto login (direct grants, no membership).
func waCloneLegacyGrants(f *testFixture, login string) error {
	ctx := context.Background()
	to := pgx.Identifier{login}.Sanitize()
	rows, err := f.owner.Query(ctx, `
	 SELECT format('GRANT %s ON SCHEMA %I TO %s', a.privilege_type, n.nspname, $1::text) FROM pg_namespace n, aclexplode(n.nspacl) a WHERE a.grantee='commerce_worker'::regrole
	 UNION ALL SELECT format('GRANT %s ON %s %s TO %s', a.privilege_type, CASE WHEN c.relkind='S' THEN 'SEQUENCE' ELSE 'TABLE' END, c.oid::regclass, $1::text)
	   FROM pg_class c, aclexplode(c.relacl) a WHERE a.grantee='commerce_worker'::regrole AND c.relkind IN ('r','p','v','m','f','S')
	 UNION ALL SELECT format('GRANT %s (%I) ON TABLE %s TO %s', a.privilege_type, t.attname, c.oid::regclass, $1::text)
	   FROM pg_class c JOIN pg_attribute t ON t.attrelid=c.oid, aclexplode(t.attacl) a WHERE a.grantee='commerce_worker'::regrole
	 UNION ALL SELECT format('GRANT EXECUTE ON FUNCTION %s TO %s', p.oid::regprocedure, $1::text) FROM pg_proc p, aclexplode(p.proacl) a WHERE a.grantee='commerce_worker'::regrole
	 UNION ALL SELECT format('ALTER POLICY %I ON %s TO %s', pol.polname, pol.polrelid::regclass,
	   array_to_string(ARRAY(SELECT quote_ident(r.rolname) FROM pg_roles r WHERE r.oid=ANY(pol.polroles)),',')||','||$1::text)
	   FROM pg_policy pol WHERE 'commerce_worker'::regrole::oid=ANY(pol.polroles)`, to)
	if err != nil {
		return err
	}
	var stmts []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			rows.Close()
			return err
		}
		stmts = append(stmts, s)
	}
	rows.Close()
	for _, s := range stmts {
		if _, err := f.owner.Exec(ctx, s); err != nil {
			return fmt.Errorf("%s: %w", s, err)
		}
	}
	return nil
}

// waResyncLegacy re-copies commerce_worker's current grants onto every legacy login of f; the historical-replay helpers call it after
// replaying original migration bytes (old SQL grants new/recreated objects to commerce_worker only). No-op on a current database.
func waResyncLegacy(f *testFixture) error {
	v, ok := waLegacyLogins.Load(f.owner)
	if !ok {
		return nil
	}
	for _, login := range v.([]string) {
		if err := waCloneLegacyGrants(f, login); err != nil {
			return err
		}
	}
	return nil
}

// waOpener adapts platform.OpenWorkerPool to the shared single-argument opener signature of the authority matrices.
func waOpener(a platform.WorkerAuthority) func(context.Context, string) (*pgxpool.Pool, error) {
	return func(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
		return platform.OpenWorkerPool(ctx, dsn, a)
	}
}

// waValidator adapts platform.ValidateWorkerPool to the single-argument validator signature of the authority matrices.
func waValidator(a platform.WorkerAuthority) func(context.Context, *pgxpool.Pool) error {
	return func(ctx context.Context, pool *pgxpool.Pool) error { return platform.ValidateWorkerPool(ctx, pool, a) }
}

// waPrecreateRoles creates the five worker authorities exactly as migration 0096 does (that migration is idempotent). Historical-state
// fixtures hold 0096 back (it needs 0064/0073/0074/0080 objects) but still run the CURRENT migrations.Apply, whose River grants
// target these roles, so they must exist before that Apply.
func waPrecreateRoles(t *testing.T, owner *pgxpool.Pool) {
	t.Helper()
	for _, r := range []string{waPayment, waLive, waExpiry, waAds, waClaims} {
		waEnsureRole(t, owner, r)
	}
}

// waEnsureRole creates authority as a bare NOLOGIN role when it is one of the five worker authorities and does not exist yet (a
// historical-state fixture that upgrades with the current migrations.Apply later: 0096 is idempotent and then grants it its privileges).
func waEnsureRole(t *testing.T, owner *pgxpool.Pool, authority string) {
	t.Helper()
	switch authority {
	case waPayment, waLive, waExpiry, waAds, waClaims:
		mustExec(t, owner, `DO $$ BEGIN IF to_regrole('`+authority+`') IS NULL THEN CREATE ROLE `+authority+` NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION; END IF; END $$`)
	}
}
