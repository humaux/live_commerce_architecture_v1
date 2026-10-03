package foundation_test

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"livecommerce/internal/platform"
	"livecommerce/migrations"
)

type testFixture struct {
	owner       *pgxpool.Pool
	runtime     *pgxpool.Pool
	databaseURL string
	tenantA     string
	tenantB     string
	storeA1     string
	storeA2     string
	storeB      string
	principalA  string
	tokens      map[string]string
}

var (
	fixtureOnce sync.Once
	shared      *testFixture
	fixtureErr  error
	fixtureSkip string
)

func TestMain(m *testing.M) {
	code := m.Run()
	if shared != nil {
		shared.runtime.Close()
		shared.owner.Close()
	}
	os.Exit(code)
}

func fixture(t *testing.T) *testFixture {
	t.Helper()
	fixtureOnce.Do(func() {
		shared, fixtureSkip, fixtureErr = newFixture()
	})
	if fixtureSkip != "" {
		t.Skip(fixtureSkip)
	}
	if fixtureErr != nil {
		t.Fatalf("set up isolated foundation database: %v", fixtureErr)
	}
	return shared
}

func newFixture() (*testFixture, string, error) {
	if os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" {
		return nil, "LC_TEST_DATABASE_ALLOWED=1 is required; real-PG foundation test NOT_RUN", nil
	}
	databaseURL := os.Getenv("LC_TEST_DATABASE_URL")
	if databaseURL == "" {
		return nil, "LC_TEST_DATABASE_URL is required; real-PG foundation test NOT_RUN", nil
	}
	ownerConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, "", errors.New("LC_TEST_DATABASE_URL is invalid")
	}
	if ownerConfig.ConnConfig.Host != "127.0.0.1" || ownerConfig.ConnConfig.Database != "lc_foundation_test" {
		return nil, "", fmt.Errorf("refusing database outside 127.0.0.1/lc_foundation_test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	owner, err := pgxpool.NewWithConfig(ctx, ownerConfig)
	if err != nil {
		return nil, "", fmt.Errorf("open migration pool: %w", err)
	}
	if err := owner.Ping(ctx); err != nil {
		owner.Close()
		return nil, "", fmt.Errorf("ping isolated database: %w", err)
	}
	if err := migrations.Apply(ctx, owner); err != nil {
		owner.Close()
		return nil, "", fmt.Errorf("first migration apply: %w", err)
	}
	if err := migrations.Apply(ctx, owner); err != nil {
		owner.Close()
		return nil, "", fmt.Errorf("second migration apply: %w", err)
	}

	password := hex.EncodeToString(randomBytes(32))
	if _, err := owner.Exec(ctx, `CREATE ROLE foundation_api LOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE IN ROLE commerce_runtime PASSWORD '`+password+`'`); err != nil {
		owner.Close()
		return nil, "", fmt.Errorf("create runtime login: %w", err)
	}
	runtimeURL, err := url.Parse(databaseURL)
	if err != nil {
		owner.Close()
		return nil, "", errors.New("reparse test database config")
	}
	runtimeURL.User = url.UserPassword("foundation_api", password)
	runtime, err := platform.OpenPool(ctx, runtimeURL.String())
	if err != nil {
		owner.Close()
		return nil, "", fmt.Errorf("open runtime pool: %w", err)
	}

	f := &testFixture{
		owner:       owner,
		runtime:     runtime,
		databaseURL: databaseURL,
		tenantA:     randomUUID(),
		tenantB:     randomUUID(),
		storeA1:     randomUUID(),
		storeA2:     randomUUID(),
		storeB:      randomUUID(),
		principalA:  randomUUID(),
		tokens: map[string]string{
			"a":             randomToken(),
			"a2":            randomToken(),
			"b":             randomToken(),
			"expired":       randomToken(),
			"revoked":       randomToken(),
			"buyer":         randomToken(),
			"revoked_grant": randomToken(),
		},
	}
	if err := f.seed(ctx); err != nil {
		runtime.Close()
		owner.Close()
		return nil, "", fmt.Errorf("seed synthetic fixtures: %w", err)
	}
	return f, "", nil
}

func (f *testFixture) seed(ctx context.Context) error {
	tx, err := f.owner.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	principalA2 := randomUUID()
	principalB := randomUUID()
	principalRevokedGrant := randomUUID()
	if _, err = tx.Exec(ctx, `INSERT INTO control.tenants(id,name) VALUES ($1,'fixture-tenant-a'),($2,'fixture-tenant-b')`, f.tenantA, f.tenantB); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO control.stores(tenant_id,id,name,currency) VALUES
		($1,$2,'fixture-store-a1','USD'),($1,$3,'fixture-store-a2','USD'),($4,$5,'fixture-store-b','TWD')`,
		f.tenantA, f.storeA1, f.storeA2, f.tenantB, f.storeB); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO identity.principals(id) VALUES ($1),($2),($3),($4)`, f.principalA, principalA2, principalB, principalRevokedGrant); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO identity.memberships(tenant_id,principal_id) VALUES ($1,$2),($1,$3),($4,$5),($1,$6)`,
		f.tenantA, f.principalA, principalA2, f.tenantB, principalB, principalRevokedGrant); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES
		($1,$2,$3,'store:read'),($1,$2,$3,'audit:read'),($1,$2,$3,'audit:write'),
		($1,$4,$5,'store:read'),($6,$7,$8,'store:read'),($1,$2,$9,'store:read')`,
		f.tenantA, f.storeA1, f.principalA, f.storeA2, principalA2, f.tenantB, f.storeB, principalB, principalRevokedGrant); err != nil {
		return err
	}
	now := time.Now().UTC()
	if err := insertSession(ctx, tx, f.tokens["a"], f.principalA, "merchant", now.Add(fixtureSessionTTL), nil); err != nil {
		return err
	}
	if err := insertSession(ctx, tx, f.tokens["a2"], principalA2, "merchant", now.Add(fixtureSessionTTL), nil); err != nil {
		return err
	}
	if err := insertSession(ctx, tx, f.tokens["b"], principalB, "merchant", now.Add(fixtureSessionTTL), nil); err != nil {
		return err
	}
	if err := insertSession(ctx, tx, f.tokens["expired"], f.principalA, "merchant", now.Add(-time.Minute), nil); err != nil {
		return err
	}
	if err := insertSession(ctx, tx, f.tokens["revoked"], f.principalA, "merchant", now.Add(fixtureSessionTTL), now); err != nil {
		return err
	}
	if err := insertSession(ctx, tx, f.tokens["buyer"], f.principalA, "buyer", now.Add(fixtureSessionTTL), nil); err != nil {
		return err
	}
	if err := insertSession(ctx, tx, f.tokens["revoked_grant"], principalRevokedGrant, "merchant", now.Add(fixtureSessionTTL), nil); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// fixtureSessionTTL is the lifetime of the shared fixture's live sessions. They are created once per test binary
// (fixtureOnce) and must outlive the whole REAL_PG run: with a 1-hour lifetime a slow release-gate G07 run (>1 h
// before store_design/t04 tests) turned every later fixture request into "unauthorized" (2026-10-03, r5-final-2a12e6c).
const fixtureSessionTTL = 24 * time.Hour

func insertSession(ctx context.Context, tx pgx.Tx, token, principal, audience string, expires time.Time, revoked any) error {
	hash := sha256.Sum256([]byte(token))
	_, err := tx.Exec(ctx, `INSERT INTO identity.sessions(id,token_hash,principal_id,audience,expires_at,revoked_at) VALUES ($1,$2,$3,$4,$5,$6)`,
		randomUUID(), hash[:], principal, audience, expires, revoked)
	return err
}

func TestMigrationIsIdempotentAndRuntimeRoleIsOrdinary(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	var currentUser string
	if err := f.runtime.QueryRow(ctx, `SELECT current_user`).Scan(&currentUser); err != nil {
		t.Fatal(err)
	}
	if currentUser != "foundation_api" {
		t.Fatalf("runtime current_user=%q, want foundation_api", currentUser)
	}
	var migrationCount int
	if err := f.owner.QueryRow(ctx, `SELECT count(*) FROM public.lc_schema_migrations WHERE version='0001_foundation.sql'`).Scan(&migrationCount); err != nil {
		t.Fatal(err)
	}
	if migrationCount != 1 {
		t.Fatalf("migration row count = %d, want 1", migrationCount)
	}
	singleConfig, err := pgxpool.ParseConfig(f.databaseURL)
	if err != nil {
		t.Fatal("reparse isolated database URL")
	}
	singleConfig.MaxConns = 1
	singlePool, err := pgxpool.NewWithConfig(ctx, singleConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer singlePool.Close()
	migrateCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := migrations.Apply(migrateCtx, singlePool); err != nil {
		t.Fatalf("idempotent Apply with MaxConns=1: %v", err)
	}
	var login, super, bypass, createRole, createDB, runtimeMember bool
	if err := f.owner.QueryRow(ctx, `SELECT rolcanlogin,rolsuper,rolbypassrls,rolcreaterole,rolcreatedb,pg_has_role('foundation_api','commerce_runtime','member') FROM pg_roles WHERE rolname='foundation_api'`).
		Scan(&login, &super, &bypass, &createRole, &createDB, &runtimeMember); err != nil {
		t.Fatal(err)
	}
	if !login || super || bypass || createRole || createDB || !runtimeMember {
		t.Fatalf("unsafe foundation_api attributes: login=%t super=%t bypass=%t createRole=%t createDB=%t runtimeMember=%t", login, super, bypass, createRole, createDB, runtimeMember)
	}
	var riverJobTable bool
	if err := f.owner.QueryRow(ctx, `SELECT to_regclass('river.river_job') IS NOT NULL`).Scan(&riverJobTable); err != nil || !riverJobTable {
		t.Fatalf("upstream River schema missing: exists=%t err=%v", riverJobTable, err)
	}
}

func TestMigrationBusyFailsFastWithoutLeakingSingleConnectionPool(t *testing.T) {
	f := fixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	holder, err := f.owner.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	locked := true
	defer func() {
		if locked {
			cleanup, stop := context.WithTimeout(context.Background(), 2*time.Second)
			_, _ = holder.Exec(cleanup, `SELECT pg_advisory_unlock(718020260920)`)
			stop()
		}
		holder.Release()
	}()
	if _, err := holder.Exec(ctx, `SELECT pg_advisory_lock(718020260920)`); err != nil {
		t.Fatal(err)
	}

	singleConfig, err := pgxpool.ParseConfig(f.databaseURL)
	if err != nil {
		t.Fatal("reparse isolated database URL")
	}
	singleConfig.MaxConns = 1
	singlePool, err := pgxpool.NewWithConfig(ctx, singleConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer singlePool.Close()

	started := time.Now()
	err = migrations.Apply(ctx, singlePool)
	if !errors.Is(err, migrations.ErrMigrationBusy) {
		t.Fatalf("Apply error=%v, want ErrMigrationBusy", err)
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("busy migration returned in %s, want <=2s", elapsed)
	}
	var one int
	if err := singlePool.QueryRow(ctx, `SELECT 1`).Scan(&one); err != nil || one != 1 {
		t.Fatalf("single-connection pool leaked after busy result: value=%d err=%v", one, err)
	}

	var unlocked bool
	if err := holder.QueryRow(ctx, `SELECT pg_advisory_unlock(718020260920)`).Scan(&unlocked); err != nil || !unlocked {
		t.Fatalf("release migration advisory lock: unlocked=%t err=%v", unlocked, err)
	}
	locked = false
	if err := migrations.Apply(ctx, singlePool); err != nil {
		t.Fatalf("Apply after lock release: %v", err)
	}
}

func TestOpenPoolRejectsLoginThatInheritsAuthPrivileges(t *testing.T) {
	f := fixture(t)
	password := hex.EncodeToString(randomBytes(32))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := f.owner.Exec(ctx, `CREATE ROLE foundation_unsafe LOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE IN ROLE commerce_runtime,commerce_auth PASSWORD '`+password+`'`); err != nil {
		t.Fatal(err)
	}
	unsafeURL, err := url.Parse(f.databaseURL)
	if err != nil {
		t.Fatal("reparse isolated database URL")
	}
	unsafeURL.User = url.UserPassword("foundation_unsafe", password)
	identityPool, err := pgxpool.New(ctx, unsafeURL.String())
	if err != nil {
		t.Fatal(err)
	}
	var currentUser string
	if err := identityPool.QueryRow(ctx, `SELECT current_user`).Scan(&currentUser); err != nil {
		identityPool.Close()
		t.Fatal(err)
	}
	identityPool.Close()
	if currentUser != "foundation_unsafe" {
		t.Fatalf("unsafe probe current_user=%q, want foundation_unsafe", currentUser)
	}
	pool, err := platform.OpenPool(ctx, unsafeURL.String())
	if pool != nil {
		pool.Close()
	}
	if err == nil {
		t.Fatal("OpenPool accepted a login inheriting commerce_auth")
	}
}

func TestScopeIsolationAndSessionRevocation(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	var got platform.Scope
	var statementTimeout, lockTimeout, idleTimeout string
	if err := platform.WithScope(ctx, f.runtime, f.tokens["a"], f.storeA1, "store:read", func(tx pgx.Tx, scope platform.Scope) error {
		got = scope
		if err := tx.QueryRow(ctx, `SHOW statement_timeout`).Scan(&statementTimeout); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SHOW lock_timeout`).Scan(&lockTimeout); err != nil {
			return err
		}
		return tx.QueryRow(ctx, `SHOW idle_in_transaction_session_timeout`).Scan(&idleTimeout)
	}); err != nil {
		t.Fatalf("valid scope: %v", err)
	}
	if got.TenantID != f.tenantA || got.StoreID != f.storeA1 || got.PrincipalID != f.principalA || got.Revision != 1 {
		t.Fatalf("wrong resolved scope: %+v", got)
	}
	if statementTimeout != "5s" || lockTimeout != "1s" || idleTimeout != "5s" {
		t.Fatalf("scope timeouts statement=%q lock=%q idle-in-tx=%q, want 5s/1s/5s", statementTimeout, lockTimeout, idleTimeout)
	}

	cases := []struct {
		name, token, store, permission string
		want                           error
	}{
		{"no token", "", f.storeA1, "store:read", platform.ErrUnauthorized},
		{"cross tenant", f.tokens["a"], f.storeB, "store:read", platform.ErrScopeNotFound},
		{"same tenant cross store", f.tokens["a"], f.storeA2, "store:read", platform.ErrScopeNotFound},
		{"wrong audience", f.tokens["buyer"], f.storeA1, "store:read", platform.ErrUnauthorized},
		{"expired", f.tokens["expired"], f.storeA1, "store:read", platform.ErrUnauthorized},
		{"revoked", f.tokens["revoked"], f.storeA1, "store:read", platform.ErrUnauthorized},
		{"missing permission", f.tokens["a"], f.storeA1, "grant:write", platform.ErrForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := platform.WithScope(ctx, f.runtime, tc.token, tc.store, tc.permission, func(pgx.Tx, platform.Scope) error {
				t.Fatal("unauthorized callback executed")
				return nil
			})
			if !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}
		})
	}

	if _, err := f.owner.Exec(ctx, `DELETE FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=(SELECT principal_id FROM identity.sessions WHERE token_hash=$3)`,
		f.tenantA, f.storeA1, tokenHash(f.tokens["revoked_grant"])); err != nil {
		t.Fatal(err)
	}
	err := platform.WithScope(ctx, f.runtime, f.tokens["revoked_grant"], f.storeA1, "store:read", func(pgx.Tx, platform.Scope) error { return nil })
	if !errors.Is(err, platform.ErrScopeNotFound) {
		t.Fatalf("revoked grant error = %v, want ErrScopeNotFound", err)
	}
}

func TestDatabaseConstraintsAndRuntimePrivileges(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	_, err := f.owner.Exec(ctx, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES ($1,$2,$3,'store:read')`, f.tenantA, f.storeB, f.principalA)
	if sqlState(err) != "23503" {
		t.Fatalf("cross-tenant composite FK state = %q err=%v, want 23503", sqlState(err), err)
	}

	var count int
	if err := f.runtime.QueryRow(ctx, `SELECT count(*) FROM control.stores`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("no-scope stores count=%d err=%v, want 0", count, err)
	}
	if err := f.runtime.QueryRow(ctx, `SELECT count(*) FROM ops.audit_events`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("no-scope audit count=%d err=%v, want 0", count, err)
	}
	if _, err := f.runtime.Exec(ctx, `SELECT count(*) FROM identity.sessions`); sqlState(err) != "42501" {
		t.Fatalf("runtime auth read state=%q err=%v, want 42501", sqlState(err), err)
	}
	if _, err := f.runtime.Exec(ctx, `UPDATE identity.store_grants SET permission='audit:read'`); sqlState(err) != "42501" {
		t.Fatalf("runtime grant mutation state=%q err=%v, want 42501", sqlState(err), err)
	}
	if _, err := f.runtime.Exec(ctx, `UPDATE river.river_job SET state='completed'`); sqlState(err) != "42501" {
		t.Fatalf("runtime River state mutation state=%q err=%v, want 42501", sqlState(err), err)
	}
	if _, err := f.runtime.Exec(ctx, `DELETE FROM river.river_job`); sqlState(err) != "42501" {
		t.Fatalf("runtime River delete state=%q err=%v, want 42501", sqlState(err), err)
	}
}

func TestAuditIsScopedAndAppendOnly(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	id := randomUUID()
	action := uniqueAction("audit.append")
	var count int
	if err := platform.WithScope(ctx, f.runtime, f.tokens["a"], f.storeA1, "audit:write", func(tx pgx.Tx, scope platform.Scope) error {
		_, err := tx.Exec(ctx, `INSERT INTO ops.audit_events(tenant_id,store_id,id,principal_id,action) VALUES ($1,$2,$3,$4,$5)`, scope.TenantID, scope.StoreID, id, scope.PrincipalID, action)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := platform.WithScope(ctx, f.runtime, f.tokens["a"], f.storeA1, "audit:read", func(tx pgx.Tx, _ platform.Scope) error {
		return tx.QueryRow(ctx, `SELECT count(*) FROM ops.audit_events WHERE id=$1`, id).Scan(&count)
	}); err != nil || count != 1 {
		t.Fatalf("scoped audit count=%d err=%v, want 1", count, err)
	}
	for name, query := range map[string]string{
		"update": `UPDATE ops.audit_events SET action='audit.changed' WHERE id=$1`,
		"delete": `DELETE FROM ops.audit_events WHERE id=$1`,
	} {
		t.Run(name, func(t *testing.T) {
			err := platform.WithScope(ctx, f.runtime, f.tokens["a"], f.storeA1, "audit:write", func(tx pgx.Tx, _ platform.Scope) error {
				_, err := tx.Exec(ctx, query, id)
				return err
			})
			if sqlState(err) != "42501" {
				t.Fatalf("append-only mutation state=%q err=%v, want 42501", sqlState(err), err)
			}
		})
	}
}

type probeArgs struct {
	AuditID string `json:"audit_id"`
}

func (probeArgs) Kind() string { return "foundation_probe" }

func TestAuditAndRiverJobShareTransaction(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	client, err := river.NewClient(riverpgxv5.New(f.runtime), &river.Config{Schema: "river"})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("commit", func(t *testing.T) {
		id := randomUUID()
		action := uniqueAction("atomic.commit")
		err := platform.WithScope(ctx, f.runtime, f.tokens["a"], f.storeA1, "audit:write", func(tx pgx.Tx, scope platform.Scope) error {
			if _, err := tx.Exec(ctx, `INSERT INTO ops.audit_events(tenant_id,store_id,id,principal_id,action) VALUES ($1,$2,$3,$4,$5)`, scope.TenantID, scope.StoreID, id, scope.PrincipalID, action); err != nil {
				return err
			}
			_, err := client.InsertTx(ctx, tx, probeArgs{AuditID: id}, nil)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		assertAuditAndJobCounts(t, f, id, 1, 1)
	})

	t.Run("rollback", func(t *testing.T) {
		id := randomUUID()
		sentinel := errors.New("rollback fixture")
		err := platform.WithScope(ctx, f.runtime, f.tokens["a"], f.storeA1, "audit:write", func(tx pgx.Tx, scope platform.Scope) error {
			if _, err := tx.Exec(ctx, `INSERT INTO ops.audit_events(tenant_id,store_id,id,principal_id,action) VALUES ($1,$2,$3,$4,$5)`, scope.TenantID, scope.StoreID, id, scope.PrincipalID, uniqueAction("atomic.rollback")); err != nil {
				return err
			}
			if _, err := client.InsertTx(ctx, tx, probeArgs{AuditID: id}, nil); err != nil {
				return err
			}
			return sentinel
		})
		if !errors.Is(err, sentinel) {
			t.Fatalf("error=%v, want sentinel", err)
		}
		assertAuditAndJobCounts(t, f, id, 0, 0)
	})
}

func TestErrorPanicCancelRollbackAndScopeCleanup(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()

	t.Run("callback error", func(t *testing.T) {
		id := randomUUID()
		sentinel := errors.New("stop")
		err := platform.WithScope(ctx, f.runtime, f.tokens["a"], f.storeA1, "audit:write", func(tx pgx.Tx, scope platform.Scope) error {
			_, err := tx.Exec(ctx, `INSERT INTO ops.audit_events(tenant_id,store_id,id,principal_id,action) VALUES ($1,$2,$3,$4,$5)`, scope.TenantID, scope.StoreID, id, scope.PrincipalID, uniqueAction("rollback.error"))
			if err != nil {
				return err
			}
			return sentinel
		})
		if !errors.Is(err, sentinel) {
			t.Fatalf("error=%v, want sentinel", err)
		}
		assertAuditAndJobCounts(t, f, id, 0, 0)
	})

	t.Run("panic", func(t *testing.T) {
		id := randomUUID()
		panicked := func() (panicked bool) {
			defer func() { panicked = recover() != nil }()
			_ = platform.WithScope(ctx, f.runtime, f.tokens["a"], f.storeA1, "audit:write", func(tx pgx.Tx, scope platform.Scope) error {
				if _, err := tx.Exec(ctx, `INSERT INTO ops.audit_events(tenant_id,store_id,id,principal_id,action) VALUES ($1,$2,$3,$4,$5)`, scope.TenantID, scope.StoreID, id, scope.PrincipalID, uniqueAction("rollback.panic")); err != nil {
					return err
				}
				panic("synthetic panic")
			})
			return false
		}()
		if !panicked {
			t.Fatal("callback panic was not propagated")
		}
		assertAuditAndJobCounts(t, f, id, 0, 0)
	})

	t.Run("cancel", func(t *testing.T) {
		id := randomUUID()
		requestCtx, cancel := context.WithCancel(context.Background())
		start := time.Now()
		err := platform.WithScope(requestCtx, f.runtime, f.tokens["a"], f.storeA1, "audit:write", func(tx pgx.Tx, scope platform.Scope) error {
			if _, err := tx.Exec(requestCtx, `INSERT INTO ops.audit_events(tenant_id,store_id,id,principal_id,action) VALUES ($1,$2,$3,$4,$5)`, scope.TenantID, scope.StoreID, id, scope.PrincipalID, uniqueAction("rollback.cancel")); err != nil {
				return err
			}
			cancel()
			return requestCtx.Err()
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error=%v, want context.Canceled", err)
		}
		if elapsed := time.Since(start); elapsed > 3*time.Second {
			t.Fatalf("cancel rollback took %s, want <=3s", elapsed)
		}
		assertAuditAndJobCounts(t, f, id, 0, 0)
	})

	t.Run("pool reuse clears local scope", func(t *testing.T) {
		held := make([]*pgxpool.Conn, 0, 7)
		for i := 0; i < 7; i++ {
			conn, err := f.runtime.Acquire(ctx)
			if err != nil {
				t.Fatal(err)
			}
			held = append(held, conn)
		}
		defer func() {
			for _, conn := range held {
				conn.Release()
			}
		}()
		if err := platform.WithScope(ctx, f.runtime, f.tokens["a"], f.storeA1, "store:read", func(pgx.Tx, platform.Scope) error { return nil }); err != nil {
			t.Fatal(err)
		}
		conn, err := f.runtime.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Release()
		var tenant, store, principal string
		if err := conn.QueryRow(ctx, `SELECT current_setting('app.tenant_id',true),current_setting('app.store_id',true),current_setting('app.principal_id',true)`).Scan(&tenant, &store, &principal); err != nil {
			t.Fatal(err)
		}
		if tenant != "" || store != "" || principal != "" {
			t.Fatalf("scope leaked after commit: tenant=%q store=%q principal=%q", tenant, store, principal)
		}
	})
}

func TestHTTPBearerScopeAndUntrustedHeaders(t *testing.T) {
	f := fixture(t)
	handler := platform.NewHandler(f.runtime)

	t.Run("cookie host and tenant headers cannot replace bearer", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/admin/stores/"+f.storeA1+"?permission=store:read", nil)
		req.Host = "attacker.example"
		req.Header.Set("Cookie", "staff_session="+f.tokens["a"])
		req.Header.Set("X-Tenant-ID", f.tenantA)
		req.Header.Set("X-Forwarded-Host", "trusted.example")
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != http.StatusUnauthorized {
			t.Fatalf("status=%d body=%s, want 401", res.Code, res.Body.String())
		}
		assertSafeHTTPResponse(t, res, f.tokens["a"])
	})

	t.Run("authorized response ignores host and tenant headers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/admin/stores/"+f.storeA1, nil)
		req.Host = "attacker.example"
		req.Header.Set("Authorization", "Bearer "+f.tokens["a"])
		req.Header.Set("X-Tenant-ID", f.tenantB)
		req.Header.Set("X-Forwarded-Host", "attacker.example")
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s, want 200", res.Code, res.Body.String())
		}
		var body struct {
			ID, Name, Currency string
		}
		if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.ID != f.storeA1 || body.Name != "fixture-store-a1" || body.Currency != "USD" {
			t.Fatalf("unexpected store response: %+v", body)
		}
		assertSafeHTTPResponse(t, res, f.tokens["a"])
	})

	t.Run("cross store remains non-disclosing not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/admin/stores/"+f.storeB, nil)
		req.Header.Set("Authorization", "Bearer "+f.tokens["a"])
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		var envelope struct {
			Code string `json:"code"`
		}
		if err := json.Unmarshal(res.Body.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		if res.Code != http.StatusNotFound || envelope.Code != "not_found" {
			t.Fatalf("status=%d body=%q, want non-disclosing 404", res.Code, res.Body.String())
		}
		assertSafeHTTPResponse(t, res, f.tokens["a"])
	})
}

func assertAuditAndJobCounts(t *testing.T, f *testFixture, id string, wantAudit, wantJob int) {
	t.Helper()
	var auditCount, jobCount int
	if err := f.owner.QueryRow(context.Background(), `SELECT count(*) FROM ops.audit_events WHERE id=$1`, id).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if err := f.owner.QueryRow(context.Background(), `SELECT count(*) FROM river.river_job WHERE kind='foundation_probe' AND args->>'audit_id'=$1`, id).Scan(&jobCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != wantAudit || jobCount != wantJob {
		t.Fatalf("audit/job counts=%d/%d, want %d/%d", auditCount, jobCount, wantAudit, wantJob)
	}
}

func assertSafeHTTPResponse(t *testing.T, res *httptest.ResponseRecorder, token string) {
	t.Helper()
	if res.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control=%q, want no-store", res.Header().Get("Cache-Control"))
	}
	body := strings.ToLower(res.Body.String())
	for _, forbidden := range []string{strings.ToLower(token), "postgres://", "sqlstate", "identity.sessions"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("response leaked forbidden detail %q", forbidden)
		}
	}
}

func tokenHash(token string) []byte {
	hash := sha256.Sum256([]byte(token))
	return hash[:]
}

func sqlState(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

func uniqueAction(prefix string) string {
	return prefix + "." + strings.ReplaceAll(randomUUID(), "-", "")[:12]
}

func randomToken() string {
	return base64.RawURLEncoding.EncodeToString(randomBytes(32))
}

func randomUUID() string {
	b := randomBytes(16)
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func randomBytes(size int) []byte {
	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand unavailable")
	}
	return b
}
