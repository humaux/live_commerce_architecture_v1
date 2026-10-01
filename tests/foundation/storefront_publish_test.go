package foundation_test

// R3 unit storefront-publish, independent PG gate (REAL_PG; evidence label MOCK for the synthetic ownership/TLS
// proof: nothing here verifies DNS or a certificate). Written from docs/delivery/units/storefront-publish.md and
// contracts/published-storefront-resolver-v1.md "Writer (R3)", not from the implementation:
//
//	SPW01 schema/ACL of migration 0081      SPW02 0081 as an upgrade of the 0080 head
//	SPW03 merchant scope + permission       SPW04 merchant compare-and-set, audit, HTTP statuses
//	SPW05 operator lifecycle + resolver     SPW06 operator input boundaries (SQL, not only Go)
//	SPW07 EXECUTE matrix per login          SPW08 lc_store_registrar login shape (provision-logins matrix)
//	SPW09 cmd/store-admin as a process      SPW10 deploy wiring (static)

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/command"
	"livecommerce/internal/domains"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/platform"
	"livecommerce/internal/storefrontadmin"
	"livecommerce/migrations"
)

type spFix struct {
	t        *testing.T
	f        *t06GoFixture
	reg      *pgxpool.Pool
	regURL   string
	resolver *domains.Resolver
	h        http.Handler
	// storeNoOwner has no identity.initial_stores row; storeB belongs to another tenant and has one.
	storeNoOwner, tenantB, storeB, principalB string
}

func spSetup(t *testing.T) *spFix {
	t.Helper()
	f := newT06GoFixture(t)
	b := f.base
	s := &spFix{t: t, f: f, tenantB: b.tenantB, storeB: b.storeB, principalB: randomUUID(), storeNoOwner: randomUUID()}
	wh := func(tenant, store string) string {
		id := randomUUID()
		mustExec(t, b.owner, `INSERT INTO inventory.warehouses(tenant_id,store_id,id,name) VALUES($1,$2,$3,'sp-wh')`, tenant, store, id)
		return id
	}
	initial := func(principal, tenant, store string) {
		mustExec(t, b.owner, `INSERT INTO identity.initial_stores(principal_id,idempotency_key,request_hash,tenant_id,store_id,warehouse_id)
			VALUES($1,$2,decode(repeat('ab',32),'hex'),$3,$4,$5)`, principal, "sp-"+randomUUID()[:16], tenant, store, wh(tenant, store))
	}
	mustExec(t, b.owner, `INSERT INTO control.stores(tenant_id,id,name,currency) VALUES($1,$2,'sp-no-owner','USD')`, f.tenant, s.storeNoOwner)
	mustExec(t, b.owner, `INSERT INTO identity.principals(id) VALUES($1)`, s.principalB)
	mustExec(t, b.owner, `INSERT INTO identity.memberships(tenant_id,principal_id) VALUES($1,$2)`, s.tenantB, s.principalB)
	initial(f.principal, f.tenant, f.store)
	initial(f.otherPrincipal, f.tenant, f.otherStore)
	initial(s.principalB, s.tenantB, s.storeB)
	t.Cleanup(func() { // runs before the t06 fixture cleanup (LIFO): FK order, then the rows other tests must not see
		ts := []string{f.tenant, s.tenantB}
		for _, q := range []string{
			`DELETE FROM control.storefront_domains WHERE tenant_id=ANY($1::uuid[])`,
			`DELETE FROM control.storefront_publications WHERE tenant_id=ANY($1::uuid[])`,
			`DELETE FROM ops.audit_events WHERE tenant_id=ANY($1::uuid[])`,
			`DELETE FROM identity.initial_stores WHERE tenant_id=ANY($1::uuid[])`,
			`DELETE FROM inventory.warehouses WHERE tenant_id=ANY($1::uuid[]) AND name='sp-wh'`,
			`DELETE FROM control.stores WHERE id=` + "'" + s.storeNoOwner + "'",
		} {
			if strings.Contains(q, "$1") {
				_, _ = b.owner.Exec(context.Background(), q, ts)
			} else {
				_, _ = b.owner.Exec(context.Background(), q)
			}
		}
		_, _ = b.owner.Exec(context.Background(), `DELETE FROM identity.memberships WHERE principal_id=$1`, s.principalB)
		_, _ = b.owner.Exec(context.Background(), `DELETE FROM identity.principals WHERE id=$1`, s.principalB)
	})
	s.regURL = miRole(t, b, "commerce_storefront_registrar")
	reg, err := pgxpool.New(context.Background(), s.regURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(reg.Close)
	s.reg = reg
	a := openBuyerTestPools(t, b)
	s.resolver = publishedResolver(t, a.issuer)
	s.h = httpapi.NewHandler(b.runtime)
	return s
}

func spOrigin() string {
	return "https://sp-" + strings.ReplaceAll(randomUUID(), "-", "")[:20] + ".test"
}

func (s *spFix) merchant(perm, token, store string, fn func(pgx.Tx, platform.Scope) error) error {
	return platform.WithScope(context.Background(), s.f.base.runtime, token, store, perm, fn)
}
func (s *spFix) set(token, store string, published bool, expected int64) (out storefrontadmin.SetResult, err error) {
	err = s.merchant("integration:manage", token, store, func(tx pgx.Tx, sc platform.Scope) error {
		var e error
		out, e = storefrontadmin.SetPublished(context.Background(), tx, sc, token, published, expected)
		return e
	})
	return
}
func (s *spFix) read(token, store string) (out storefrontadmin.State, err error) {
	err = s.merchant("integration:read", token, store, func(tx pgx.Tx, sc platform.Scope) error {
		var e error
		out, e = storefrontadmin.Read(context.Background(), tx, sc, token)
		return e
	})
	return
}
func (s *spFix) bind(store, origin, evidence string, until time.Duration) (storefrontadmin.BindResult, error) {
	return storefrontadmin.BindDomain(context.Background(), s.reg, store, origin, evidence, time.Now().Add(until), time.Now())
}
func (s *spFix) status(store string) (storefrontadmin.StoreStatus, error) {
	return storefrontadmin.Status(context.Background(), s.reg, store)
}
func (s *spFix) audits(store string) []string {
	rows, err := s.f.base.owner.Query(context.Background(), `SELECT action FROM ops.audit_events WHERE store_id=$1 AND action LIKE ANY(ARRAY['merchant.storefront%','operator.domain%']) ORDER BY created_at,id`, store)
	if err != nil {
		s.t.Fatal(err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		s.t.Fatal(err)
	}
	return out
}
func (s *spFix) mustResolve(origin, store string) domains.Route {
	s.t.Helper()
	r, err := s.resolver.Resolve(context.Background(), origin)
	if err != nil || r.StoreID != store || r.Origin != origin {
		s.t.Fatalf("resolver must admit %s for %s: route=%+v err=%v", origin, store, r, err)
	}
	return r
}
func (s *spFix) mustDeny(origin, why string) {
	s.t.Helper()
	if r, err := s.resolver.Resolve(context.Background(), origin); !errors.Is(err, domains.ErrUnavailable) || r != (domains.Route{}) {
		s.t.Fatalf("resolver must deny %s (%s): route=%+v err=%v", origin, why, r, err)
	}
}
func pgCode(err error) string {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		return pg.Code
	}
	return ""
}
func eq[T comparable](t *testing.T, label string, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %v, want %v", label, got, want)
	}
}

var spFns = []string{ // merchant first, then operator
	"control.read_storefront(bytea,uuid)", "control.set_storefront_published(bytea,uuid,boolean,bigint)",
	"control.operator_bind_domain(uuid,text,text,timestamptz)", "control.operator_suspend_domain(text)",
	"control.operator_detach_domain(text)", "control.operator_storefront_status(uuid)",
}

// spAssertSchema is shared by the fresh database (SPW01) and the upgraded one (SPW02).
func spAssertSchema(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	for _, fn := range spFns {
		var secdef bool
		var owner, cfg, vol string
		var comment *string
		var acl []string
		err := pool.QueryRow(ctx, `SELECT p.prosecdef,pg_get_userbyid(p.proowner),coalesce(p.proconfig::text,''),p.provolatile::text,
			obj_description(p.oid,'pg_proc'),coalesce((SELECT array_agg(a.grantee::regrole::text) FROM aclexplode(p.proacl) a),'{}')
			FROM pg_proc p WHERE p.oid=$1::regprocedure`, fn).Scan(&secdef, &owner, &cfg, &vol, &comment, &acl)
		if err != nil {
			t.Fatalf("%s missing: %v", fn, err)
		}
		if !secdef || owner != "commerce_storefront_writer" || !strings.Contains(cfg, "search_path=pg_catalog") || vol != "v" || comment == nil || *comment == "" {
			t.Fatalf("%s: secdef=%v owner=%s config=%s volatility=%s comment=%v", fn, secdef, owner, cfg, vol, comment)
		}
		var public bool
		if err := pool.QueryRow(ctx, `SELECT coalesce(bool_or(a.grantee=0),false) FROM pg_proc p, aclexplode(p.proacl) a WHERE p.oid=$1::regprocedure`, fn).Scan(&public); err != nil || public {
			t.Fatalf("%s is executable by PUBLIC (err=%v)", fn, err)
		}
	}
	for _, role := range []string{"commerce_storefront_writer", "commerce_storefront_registrar"} {
		var login, super, bypass bool
		var comment *string
		if err := pool.QueryRow(ctx, `SELECT rolcanlogin,rolsuper,rolbypassrls,shobj_description(oid,'pg_authid') FROM pg_roles WHERE rolname=$1`, role).Scan(&login, &super, &bypass, &comment); err != nil {
			t.Fatalf("role %s: %v", role, err)
		}
		if login || super || bypass || comment == nil {
			t.Fatalf("role %s must be NOLOGIN, unprivileged and commented: login=%v super=%v bypass=%v", role, login, super, bypass)
		}
	}
	if n := countRows(t, pool, `SELECT count(*) FROM pg_auth_members WHERE roleid='commerce_storefront_writer'::regrole OR member='commerce_storefront_writer'::regrole`); n != 0 {
		t.Fatalf("commerce_storefront_writer has %d memberships; it owns definers and must be a dead-end role", n)
	}
	// EXECUTE: exactly runtime -> the two merchant functions, registrar -> the four operator functions, nobody else.
	for i, fn := range spFns {
		wantRuntime, wantRegistrar := i < 2, i >= 2
		var runtime, registrar bool
		if err := pool.QueryRow(ctx, `SELECT has_function_privilege('commerce_runtime',$1::regprocedure,'EXECUTE'),has_function_privilege('commerce_storefront_registrar',$1::regprocedure,'EXECUTE')`, fn).Scan(&runtime, &registrar); err != nil {
			t.Fatal(err)
		}
		if runtime != wantRuntime || registrar != wantRegistrar {
			t.Fatalf("%s: commerce_runtime=%v (want %v) commerce_storefront_registrar=%v (want %v)", fn, runtime, wantRuntime, registrar, wantRegistrar)
		}
		var leaked []string
		rows, err := pool.Query(ctx, `SELECT r.rolname FROM pg_roles r WHERE r.rolname LIKE 'commerce\_%'
			AND r.rolname NOT IN ('commerce_runtime','commerce_storefront_registrar','commerce_storefront_writer')
			AND NOT pg_has_role(r.oid,'commerce_runtime','MEMBER') AND NOT pg_has_role(r.oid,'commerce_storefront_registrar','MEMBER')
			AND has_function_privilege(r.oid,$1::regprocedure,'EXECUTE')`, fn)
		if err == nil {
			leaked, err = pgx.CollectRows(rows, pgx.RowTo[string])
		}
		if err != nil || len(leaked) != 0 {
			t.Fatalf("%s executable by other authorities %v (err=%v)", fn, leaked, err)
		}
	}
	// The registrar and runtime authorities have no direct table access to what the definers write.
	for _, tbl := range []string{"control.storefront_publications", "control.storefront_domains", "ops.audit_events"} {
		var access bool
		if err := pool.QueryRow(ctx, `SELECT has_table_privilege('commerce_storefront_registrar',$1,'SELECT,INSERT,UPDATE,DELETE') OR has_any_column_privilege('commerce_storefront_registrar',$1,'SELECT,INSERT,UPDATE')`, tbl).Scan(&access); err != nil || access {
			t.Fatalf("registrar has direct access to %s (err=%v)", tbl, err)
		}
	}
	if n := countRows(t, pool, `SELECT count(*) FROM pg_policy WHERE polname LIKE 'storefront\_writer\_%'`); n != 8 {
		t.Fatalf("storefront_writer policies = %d, want 8 (publications r/i/u, domains r/i/u, stores read, audit insert)", n)
	}
}

func TestStorefrontPublishSPW01SchemaAndGrants(t *testing.T) {
	s := spSetup(t)
	b := s.f.base
	spAssertSchema(t, b.owner)
	ctx := context.Background()
	// The writer is bounded by RLS to the six fixed audit actions and column-level grants.
	for name, stmt := range map[string]string{
		"foreign audit action": fmt.Sprintf(`INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action) VALUES('%s','%s','%s','store.permissions_granted')`, s.f.tenant, s.f.store, s.f.principal),
		"origin rewrite":       `UPDATE control.storefront_domains SET origin='https://rewritten.test'`,
		"other domain table":   `SELECT 1 FROM catalog.products LIMIT 1`,
	} {
		tx, err := b.owner.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, `SET LOCAL ROLE commerce_storefront_writer`); err != nil {
			t.Fatal(err)
		}
		_, err = tx.Exec(ctx, stmt)
		_ = tx.Rollback(ctx)
		if pgCode(err) != "42501" {
			t.Fatalf("writer %s: err=%v, want 42501", name, err)
		}
	}
}

func TestStorefrontPublishSPW02UpgradeAfter0080(t *testing.T) {
	ctx := context.Background()
	numbered, _ := filepath.Glob("../../migrations/[0-9][0-9][0-9][0-9]_*.sql")
	const v81 = "0081_storefront_publish.sql"
	var found bool
	for _, p := range numbered {
		if filepath.Base(p) == v81 {
			found = true
		}
	}
	if !found {
		t.Fatalf("migrations/%s not present", v81)
	}
	db := mciStartPG(t)
	body, err := os.ReadFile("../../migrations/" + v81)
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `CREATE TABLE public.lc_schema_migrations (version text PRIMARY KEY, checksum text NOT NULL, applied_at timestamptz NOT NULL DEFAULT now())`)
	mustExec(t, db, `INSERT INTO public.lc_schema_migrations(version,checksum) VALUES($1,$2)`, v81, fmt.Sprintf("%x", sha256.Sum256(body)))
	if err := migrations.Apply(ctx, db); err != nil { // every other migration, i.e. the 0080 head
		t.Fatalf("0080 head: %v", err)
	}
	if n := countRows(t, db, `SELECT count(*) FROM pg_proc WHERE proname IN ('set_storefront_published','operator_bind_domain')`) + countRows(t, db, `SELECT count(*) FROM pg_roles WHERE rolname LIKE 'commerce\_storefront\_%'`); n != 0 {
		t.Fatalf("the 0080 head already holds %d 0081 objects: the ledger pre-mark did not isolate 0081", n)
	}
	if n := countRows(t, db, `SELECT count(*) FROM public.lc_schema_migrations WHERE version LIKE '0080\_%'`); n != 1 {
		t.Fatalf("the 0080 head must hold 0080_meta_capi.sql in its ledger, has %d rows", n)
	}
	mustExec(t, db, `DELETE FROM public.lc_schema_migrations WHERE version=$1`, v81)
	for i := 0; i < 2; i++ { // the second Apply must be a no-op
		if err := migrations.Apply(ctx, db); err != nil {
			t.Fatalf("0081 upgrade apply %d: %v", i+1, err)
		}
	}
	if n := countRows(t, db, `SELECT count(*) FROM public.lc_schema_migrations WHERE version=$1`, v81); n != 1 {
		t.Fatalf("ledger rows for 0081 = %d", n)
	}
	spAssertSchema(t, db)
	// The registrar authority can actually run its definer after an upgrade (not only hold the grant).
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SET LOCAL ROLE commerce_storefront_registrar`); err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec(ctx, `SELECT control.operator_storefront_status(gen_random_uuid())`)
	eq(t, "status of an unknown store on the upgraded DB", pgCode(err), "PT404")
}

func TestStorefrontPublishSPW03MerchantScopeAndPermission(t *testing.T) {
	s := spSetup(t)
	f, b := s.f, s.f.base
	ctx := context.Background()
	rows := func() int {
		return countRows(t, b.owner, `SELECT count(*) FROM control.storefront_publications WHERE tenant_id=ANY($1::uuid[])`, []string{f.tenant, s.tenantB})
	}
	audits := func() int {
		return countRows(t, b.owner, `SELECT count(*) FROM ops.audit_events WHERE tenant_id=ANY($1::uuid[]) AND action LIKE 'merchant.storefront%'`, []string{f.tenant, s.tenantB})
	}
	// Reads: own store, never another.
	st, err := s.read(f.token, f.store)
	if err != nil || st.Published || st.Version != 0 || st.Domains == nil || len(st.Domains) != 0 {
		t.Fatalf("fresh store read: %+v %v", st, err)
	}
	for name, c := range map[string]struct {
		token, store string
		write        bool
		want         error
	}{
		"foreign tenant store (read)":  {f.token, s.storeB, false, platform.ErrScopeNotFound},
		"foreign tenant store (write)": {f.token, s.storeB, true, platform.ErrScopeNotFound},
		"store the grantee cannot see": {f.otherToken, f.otherStore, true, platform.ErrScopeNotFound},
		"read-only grantee writes":     {f.otherToken, f.store, true, platform.ErrForbidden},
		"no integration grant at all":  {f.missingPermission, f.store, true, platform.ErrForbidden},
		"no integration grant (read)":  {f.missingPermission, f.store, false, platform.ErrForbidden},
		"unknown bearer":               {randomToken(), f.store, true, platform.ErrUnauthorized},
		"unknown bearer (read)":        {randomToken(), f.store, false, platform.ErrUnauthorized},
	} {
		var err error
		if c.write {
			_, err = s.set(c.token, c.store, true, 0)
		} else {
			_, err = s.read(c.token, c.store)
		}
		if !errors.Is(err, c.want) {
			t.Fatalf("%s: err=%v, want %v", name, err, c.want)
		}
	}
	if rows() != 0 || audits() != 0 {
		t.Fatalf("denied calls left %d publications and %d audit rows", rows(), audits())
	}
	// Defence in depth: the definer re-checks the permission itself when the caller's scope was only store:read.
	lowScope := func(token, store string, fn func(tx pgx.Tx, sc platform.Scope) error) error {
		return s.merchant("store:read", token, store, fn)
	}
	err = lowScope(f.otherToken, f.store, func(tx pgx.Tx, sc platform.Scope) error {
		_, e := storefrontadmin.SetPublished(ctx, tx, sc, f.otherToken, true, 0)
		return e
	})
	if !errors.Is(err, platform.ErrForbidden) {
		t.Fatalf("definer let integration:read publish: %v", err)
	}
	err = lowScope(f.missingPermission, f.store, func(tx pgx.Tx, sc platform.Scope) error {
		_, e := storefrontadmin.Read(ctx, tx, sc, f.missingPermission)
		return e
	})
	if !errors.Is(err, platform.ErrForbidden) {
		t.Fatalf("definer let store:read-only read the storefront card: %v", err)
	}
	// The scope arguments are not trusted: GUCs (server auth) and the SQL arguments must agree.
	for name, mutate := range map[string]func(*platform.Scope){
		"other store of the same tenant": func(sc *platform.Scope) { sc.StoreID = f.otherStore },
	} {
		err = s.merchant("integration:manage", f.token, f.store, func(tx pgx.Tx, sc platform.Scope) error {
			mutate(&sc)
			_, e := storefrontadmin.SetPublished(ctx, tx, sc, f.token, true, 0)
			return e
		})
		if !errors.Is(err, platform.ErrForbidden) {
			t.Fatalf("scope swap (%s): %v, want forbidden", name, err)
		}
	}
	err = s.merchant("integration:manage", f.token, f.store, func(tx pgx.Tx, sc platform.Scope) error {
		sc.StoreID = s.storeB // GUCs name f.store; the argument names a foreign tenant's store
		_, e := storefrontadmin.SetPublished(ctx, tx, sc, f.token, true, 0)
		return e
	})
	if !errors.Is(err, platform.ErrScopeNotFound) {
		t.Fatalf("cross-tenant store argument: %v, want not_found", err)
	}
	// No scope GUCs at all (a caller that skipped platform.WithScope) and an unknown bearer hash, straight to the definer.
	raw := func(sql string, args ...any) error {
		tx, err := b.runtime.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		_, err = tx.Exec(ctx, sql, args...)
		return err
	}
	hash := sha256.Sum256([]byte(f.token))
	eq(t, "set without scope GUCs", pgCode(raw(`SELECT control.set_storefront_published($1,$2::uuid,true,0)`, hash[:], f.store)), "PT403")
	eq(t, "read without scope GUCs", pgCode(raw(`SELECT control.read_storefront($1,$2::uuid)`, hash[:], f.store)), "PT403")
	bad := sha256.Sum256([]byte(randomToken()))
	eq(t, "set with an unknown bearer hash", pgCode(raw(`SELECT control.set_storefront_published($1,$2::uuid,true,0)`, bad[:], f.store)), "PT401")
	eq(t, "short hash", pgCode(raw(`SELECT control.set_storefront_published($1,$2::uuid,true,0)`, []byte("short"), f.store)), "PT400")
	eq(t, "negative expected version", pgCode(raw(`SELECT control.set_storefront_published($1,$2::uuid,true,-1)`, hash[:], f.store)), "PT400")
	if rows() != 0 || audits() != 0 {
		t.Fatalf("scope attacks left %d publications and %d audit rows", rows(), audits())
	}
	// Revocation: the grant is checked per call; a revoked read grant stops the card.
	mustExec(t, b.owner, `DELETE FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND permission='integration:read'`, f.tenant, f.store, f.otherPrincipal)
	if _, err := s.read(f.otherToken, f.store); !errors.Is(err, platform.ErrForbidden) {
		t.Fatalf("read after grant revocation: %v", err)
	}
	// Publication is per store: publishing f.store never touches f.otherStore or the foreign tenant.
	if _, err := s.set(f.token, f.store, true, 0); err != nil {
		t.Fatal(err)
	}
	other, err := s.read(f.token, f.otherStore)
	if err != nil || other.Published || other.Version != 0 {
		t.Fatalf("sibling store state after publishing: %+v %v", other, err)
	}
	if n := countRows(t, b.owner, `SELECT count(*) FROM control.storefront_publications WHERE store_id=$1`, s.storeB); n != 0 {
		t.Fatal("foreign tenant store gained a publication row")
	}
}

func spJSON(t *testing.T, h http.Handler, method, path, token string, body string) (int, map[string]any) {
	t.Helper()
	var rdr []byte
	if body != "" {
		rdr = []byte(body)
	}
	w := adminRequest(h, method, path, token, rdr, map[bool]string{true: "application/json", false: ""}[body != ""], nil)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func TestStorefrontPublishSPW04MerchantCompareAndSetAuditAndHTTP(t *testing.T) {
	s := spSetup(t)
	f, b := s.f, s.f.base
	step := func(label string, published bool, expected int64, wantVersion int64, wantChanged bool) {
		t.Helper()
		out, err := s.set(f.token, f.store, published, expected)
		if err != nil || out.Published != published || out.Version != wantVersion || out.Changed != wantChanged {
			t.Fatalf("%s: %+v err=%v, want published=%v version=%d changed=%v", label, out, err, published, wantVersion, wantChanged)
		}
	}
	// expected_version 0 = never published.
	step("unpublish a never-published store is a no-op", false, 0, 0, false)
	if n := countRows(t, b.owner, `SELECT count(*) FROM control.storefront_publications WHERE store_id=$1`, f.store); n != 0 {
		t.Fatal("a no-op unpublish created a row")
	}
	if _, err := s.set(f.token, f.store, true, 1); !errors.Is(err, command.ErrConflict) {
		t.Fatalf("publish with expected=1 on a never-published store: %v, want conflict", err)
	}
	step("first publish", true, 0, 1, true)
	step("publish again at the current version is a no-op", true, 1, 1, false)
	if _, err := s.set(f.token, f.store, false, 0); !errors.Is(err, command.ErrConflict) {
		t.Fatalf("stale expected=0 after publish: %v, want conflict", err)
	}
	step("unpublish", false, 1, 2, true)
	if _, err := s.set(f.token, f.store, false, 1); !errors.Is(err, command.ErrConflict) {
		t.Fatalf("stale unpublish: %v, want conflict", err)
	}
	step("unpublish at the current version is a no-op", false, 2, 2, false)
	step("republish", true, 2, 3, true)
	st, err := s.read(f.token, f.store)
	if err != nil || !st.Published || st.Version != 3 || len(st.Domains) != 0 {
		t.Fatalf("read after toggles: %+v %v", st, err)
	}
	// One audit row per real change and none for no-ops or refusals, attributed to the acting principal.
	want := []string{"merchant.storefront_published", "merchant.storefront_unpublished", "merchant.storefront_published"}
	if got := s.audits(f.store); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("audit actions = %v, want %v", got, want)
	}
	if n := countRows(t, b.owner, `SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND tenant_id=$2 AND principal_id=$3 AND action LIKE 'merchant.storefront%'`, f.store, f.tenant, f.principal); n != 3 {
		t.Fatalf("audit rows attributed to the merchant principal = %d, want 3", n)
	}
	if n := countRows(t, b.owner, `SELECT count(*) FROM control.storefront_domains WHERE tenant_id=$1`, f.tenant); n != 0 {
		t.Fatal("publishing created a domain row (only the operator binds domains)")
	}

	// Concurrent first publish: exactly one winner, the rest conflict, one version and one audit row.
	var wg sync.WaitGroup
	results := make([]error, 8)
	changed := make([]bool, 8)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out, err := s.set(f.token, f.otherStore, true, 0)
			results[i], changed[i] = err, out.Changed
		}()
	}
	wg.Wait()
	wins := 0
	for i, e := range results {
		switch {
		case e == nil && changed[i]:
			wins++
		case errors.Is(e, command.ErrConflict):
		default:
			t.Fatalf("concurrent first publish %d: %v changed=%v", i, e, changed[i])
		}
	}
	if wins != 1 || countRows(t, b.owner, `SELECT count(*) FROM control.storefront_publications WHERE store_id=$1 AND version=1`, f.otherStore) != 1 || len(s.audits(f.otherStore)) != 1 {
		t.Fatalf("concurrent publish: winners=%d audits=%v", wins, s.audits(f.otherStore))
	}

	// HTTP surface (the Settings card's BFF talks to exactly these routes).
	base := "/v1/admin/stores/" + f.store + "/storefront"
	if code, _ := spJSON(t, s.h, "GET", base, "", ""); code != 401 {
		t.Fatalf("GET without a bearer = %d", code)
	}
	code, got := spJSON(t, s.h, "GET", base, f.token, "")
	if code != 200 || got["published"] != true || got["version"] != float64(3) || fmt.Sprint(got["domains"]) != "[]" || len(got) != 3 {
		t.Fatalf("GET storefront = %d %v", code, got)
	}
	if code, _ := spJSON(t, s.h, "GET", "/v1/admin/stores/"+s.storeB+"/storefront", f.token, ""); code != 404 {
		t.Fatalf("GET foreign tenant store = %d, want 404", code)
	}
	post := func(token, body string) (int, map[string]any) {
		return spJSON(t, s.h, "POST", base+"/publication", token, body)
	}
	if code, _ := post("", `{"published":false,"expected_version":3}`); code != 401 {
		t.Fatalf("POST without a bearer = %d", code)
	}
	if code, _ := post(f.otherToken, `{"published":false,"expected_version":3}`); code != 403 {
		t.Fatalf("POST by integration:read grantee = %d, want 403", code)
	}
	if code, body := post(f.token, `{"published":false,"expected_version":2}`); code != 409 || body["code"] != "conflict" {
		t.Fatalf("POST stale version = %d %v, want 409 conflict", code, body)
	}
	for name, body := range map[string]string{
		"missing expected_version": `{"published":false}`, "missing published": `{"expected_version":3}`,
		"negative version": `{"published":false,"expected_version":-1}`, "version 2^62": `{"published":false,"expected_version":4611686018427387904}`,
		"string published": `{"published":"false","expected_version":3}`,
	} {
		if code, _ := post(f.token, body); code != 422 && code != 400 {
			t.Fatalf("POST %s = %d, want 4xx validation", name, code)
		}
	}
	if code, _ := post(f.token, `{"published":false,"expected_version":3,"tenant_id":"`+s.tenantB+`"}`); code != 400 {
		t.Fatalf("POST with an extra tenant_id field = %d, want 400 (unknown fields refused)", code)
	}
	code, got = post(f.token, `{"published":false,"expected_version":3}`)
	if code != 200 || got["published"] != false || got["version"] != float64(4) || got["changed"] != true {
		t.Fatalf("POST unpublish = %d %v", code, got)
	}
	// No merchant route may bind or touch domains.
	for _, p := range []string{"/domain", "/domains", "/domains/bind", "/domain/bind", "/publication/domains"} {
		if code, _ := spJSON(t, s.h, "POST", base+p, f.token, `{"origin":"https://evil.test"}`); code < 400 || code == 500 {
			t.Fatalf("POST %s = %d: a merchant route exists for domain binding", p, code)
		}
	}
	if n := countRows(t, b.owner, `SELECT count(*) FROM control.storefront_domains WHERE tenant_id=$1`, f.tenant); n != 0 {
		t.Fatal("a merchant HTTP call created a domain row")
	}
	if n := countRows(t, b.owner, `SELECT count(*) FROM control.storefront_publications WHERE store_id=$1 AND version=4 AND NOT published`, f.store); n != 1 {
		t.Fatal("HTTP unpublish did not persist version 4")
	}
}

func TestStorefrontPublishSPW05OperatorLifecycleAndResolver(t *testing.T) {
	s := spSetup(t)
	f, b := s.f, s.f.base
	ctx := context.Background()
	origin, ev1, ev2 := spOrigin(), "cert notAfter proof 1", "cert notAfter proof 2 (renewed)"

	s.mustDeny(origin, "nothing exists")
	if _, err := s.set(f.token, f.store, true, 0); err != nil {
		t.Fatal(err)
	}
	s.mustDeny(origin, "published but no domain: publication alone serves nothing")

	res, err := s.bind(f.store, origin, ev1, 90*24*time.Hour)
	if err != nil || res.State != "ACTIVE" || res.Version != 1 || res.Renewed || res.Rebound || res.DomainID == "" {
		t.Fatalf("bind: %+v %v", res, err)
	}
	route := s.mustResolve(origin, f.store)
	if route.DomainID != res.DomainID || route.DomainVersion != 1 || route.PublicationVersion != 1 {
		t.Fatalf("route after bind: %+v", route)
	}
	// Merchant sees only its own ACTIVE origin (never proof or another store's domains).
	other, err := s.bind(s.storeB, spOrigin(), "other tenant proof", 30*24*time.Hour)
	if err != nil || other.State != "ACTIVE" {
		t.Fatalf("bind for the second tenant: %+v %v", other, err)
	}
	st, err := s.read(f.token, f.store)
	if err != nil || len(st.Domains) != 1 || st.Domains[0].Origin != origin || !st.Domains[0].Serving || !st.Published {
		t.Fatalf("merchant view: %+v %v", st, err)
	}
	// Unpublish / republish: the very next resolve sees it (the resolver has no cache).
	if _, err = s.set(f.token, f.store, false, 1); err != nil {
		t.Fatal(err)
	}
	s.mustDeny(origin, "unpublished")
	if _, err = s.set(f.token, f.store, true, 2); err != nil {
		t.Fatal(err)
	}
	if r := s.mustResolve(origin, f.store); r.PublicationVersion != 3 {
		t.Fatalf("publication version after republish = %d", r.PublicationVersion)
	}
	// Renewal of an ACTIVE origin restamps the proof and bumps the version.
	res, err = s.bind(f.store, origin, ev2, 120*24*time.Hour)
	if err != nil || !res.Renewed || res.Rebound || res.Version != 2 {
		t.Fatalf("renew: %+v %v", res, err)
	}
	if r := s.mustResolve(origin, f.store); r.DomainVersion != 2 {
		t.Fatalf("domain version after renewal = %d", r.DomainVersion)
	}
	// Another store may not take an origin that is not DETACHED.
	if _, err = s.bind(f.otherStore, origin, "stolen", 30*24*time.Hour); !errors.Is(err, storefrontadmin.ErrDomainOwnedElsewhere) {
		t.Fatalf("bind of an ACTIVE origin by another store: %v", err)
	}
	if _, err = s.bind(s.storeB, origin, "stolen across tenants", 30*24*time.Hour); !errors.Is(err, storefrontadmin.ErrDomainOwnedElsewhere) {
		t.Fatalf("bind of an ACTIVE origin by another tenant: %v", err)
	}
	s.mustResolve(origin, f.store)

	// Suspend: the resolver denies at once; repeating is a no-op without a version bump or audit.
	mv, err := storefrontadmin.SuspendDomain(ctx, s.reg, origin)
	if err != nil || mv.State != "SUSPENDED" || !mv.Changed || mv.Version != 3 {
		t.Fatalf("suspend: %+v %v", mv, err)
	}
	s.mustDeny(origin, "suspended")
	again, err := storefrontadmin.SuspendDomain(ctx, s.reg, origin)
	if err != nil || again.Changed || again.Version != 3 {
		t.Fatalf("repeat suspend: %+v %v", again, err)
	}
	if st, _ = s.read(f.token, f.store); len(st.Domains) != 0 {
		t.Fatalf("merchant still sees a SUSPENDED origin: %+v", st.Domains)
	}
	if _, err = s.bind(f.otherStore, origin, "stolen while suspended", 30*24*time.Hour); !errors.Is(err, storefrontadmin.ErrDomainOwnedElsewhere) {
		t.Fatalf("another store binding a SUSPENDED origin: %v", err)
	}
	// SUSPENDED is reversible by binding again with fresh proof.
	res, err = s.bind(f.store, origin, ev2, 100*24*time.Hour)
	if err != nil || res.State != "ACTIVE" || res.Renewed || res.Version != 4 {
		t.Fatalf("reactivate: %+v %v", res, err)
	}
	s.mustResolve(origin, f.store)

	// Detach is final for the evidence in force: re-applying it, suspending it, or re-binding with the SAME evidence refuses.
	dt, err := storefrontadmin.DetachDomain(ctx, s.reg, origin)
	if err != nil || dt.State != "DETACHED" || !dt.Changed || dt.Version != 5 {
		t.Fatalf("detach: %+v %v", dt, err)
	}
	s.mustDeny(origin, "detached")
	if dt, err = storefrontadmin.DetachDomain(ctx, s.reg, origin); err != nil || dt.Changed || dt.Version != 5 {
		t.Fatalf("repeat detach: %+v %v", dt, err)
	}
	if _, err = storefrontadmin.SuspendDomain(ctx, s.reg, origin); !errors.Is(err, storefrontadmin.ErrDomainDetached) {
		t.Fatalf("suspend of a DETACHED origin: %v", err)
	}
	if _, err = s.bind(f.store, origin, ev2, 30*24*time.Hour); !errors.Is(err, storefrontadmin.ErrDomainDetached) {
		t.Fatalf("re-bind with the unchanged evidence: %v, want domain_detached", err)
	}
	s.mustDeny(origin, "refused re-bind must not revive it")
	// Integrator ruling: renewed proof (a different evidence_ref) re-binds a DETACHED origin, audited as a rebind.
	res, err = s.bind(f.store, origin, "renewed after detach: new cert", 60*24*time.Hour)
	if err != nil || !res.Rebound || res.Renewed || res.State != "ACTIVE" || res.Version != 6 || res.DomainID != route.DomainID {
		t.Fatalf("re-bind with renewed proof: %+v %v", res, err)
	}
	if r := s.mustResolve(origin, f.store); r.DomainVersion != 6 || r.DomainID != route.DomainID {
		t.Fatalf("route after re-bind: %+v", r)
	}
	// Bind never publishes: another store with a domain but no publication serves nothing.
	o2 := spOrigin()
	if _, err = s.bind(f.otherStore, o2, "other store proof", 30*24*time.Hour); err != nil {
		t.Fatal(err)
	}
	s.mustDeny(o2, "domain bound but the merchant never published")
	if _, err = s.set(f.token, f.otherStore, true, 0); err != nil {
		t.Fatal(err)
	}
	s.mustResolve(o2, f.otherStore)
	// A DETACHED origin may be re-bound (new evidence) to ANOTHER store per the ruling: the old store stops serving it.
	if _, err = storefrontadmin.DetachDomain(ctx, s.reg, o2); err != nil {
		t.Fatal(err)
	}
	if res, err = s.bind(f.store, o2, "moved to the first store", 30*24*time.Hour); err != nil || !res.Rebound {
		t.Fatalf("move of a DETACHED origin: %+v %v", res, err)
	}
	s.mustResolve(o2, f.store)
	if n := countRows(t, b.owner, `SELECT count(*) FROM control.storefront_domains WHERE origin=$1`, o2); n != 1 {
		t.Fatalf("origin rows = %d, want exactly 1 (UNIQUE origin)", n)
	}

	// Expiry window: valid_until in the past stops serving without any operator action; the merchant sees serving=false.
	mustExec(t, b.owner, `UPDATE control.storefront_domains SET ownership_verified_at=clock_timestamp()-interval '3 hours',tls_verified_at=clock_timestamp()-interval '3 hours',valid_until=clock_timestamp()-interval '1 minute' WHERE origin=$1`, origin)
	s.mustDeny(origin, "certificate window ended")
	if st, _ = s.read(f.token, f.store); !func() bool {
		for _, d := range st.Domains {
			if d.Origin == origin {
				return !d.Serving
			}
		}
		return false
	}() {
		t.Fatalf("merchant view of an expired origin: %+v", st.Domains)
	}
	// Status (operator): every domain row, serving flags, no evidence or proof.
	ss, err := s.status(f.store)
	if err != nil || ss.StoreID != f.store || !ss.Published || len(ss.Domains) != 2 {
		t.Fatalf("status: %+v %v", ss, err)
	}
	for _, d := range ss.Domains {
		if d.Origin == origin && (d.State != "ACTIVE" || d.Serving) {
			t.Fatalf("expired origin status: %+v", d)
		}
	}
	if _, err = s.status(randomUUID()); !errors.Is(err, platform.ErrScopeNotFound) {
		t.Fatalf("status of an unknown store: %v", err)
	}
	var raw []byte
	if err = s.reg.QueryRow(ctx, `SELECT control.operator_storefront_status($1::uuid)`, f.store).Scan(&raw); err != nil || bytes.Contains(raw, []byte("evidence")) || bytes.Contains(raw, []byte("renewed after detach")) {
		t.Fatalf("status JSON exposes evidence: %s %v", raw, err)
	}

	// Stores without an owner principal cannot be bound or moved (audit attribution), and nothing is half-written.
	if _, err = s.bind(s.storeNoOwner, spOrigin(), "no owner", 30*24*time.Hour); !errors.Is(err, storefrontadmin.ErrNoOwner) {
		t.Fatalf("bind for a store without an owner principal: %v", err)
	}
	if n := countRows(t, b.owner, `SELECT count(*) FROM control.storefront_domains WHERE store_id=$1`, s.storeNoOwner); n != 0 {
		t.Fatal("a refused bind left a domain row")
	}
	ghost := spOrigin()
	mustExec(t, b.owner, `INSERT INTO control.storefront_domains(tenant_id,store_id,origin,state,ownership_verified_at,tls_verified_at,valid_until,evidence_ref)
		VALUES($1,$2,$3,'ACTIVE',clock_timestamp(),clock_timestamp(),clock_timestamp()+interval '1 day','owner seeded')`, f.tenant, s.storeNoOwner, ghost)
	if _, err = storefrontadmin.SuspendDomain(ctx, s.reg, ghost); !errors.Is(err, storefrontadmin.ErrNoOwner) {
		t.Fatalf("suspend for a store without an owner principal: %v", err)
	}
	if n := countRows(t, b.owner, `SELECT count(*) FROM control.storefront_domains WHERE origin=$1 AND state='ACTIVE' AND version=1`, ghost); n != 1 {
		t.Fatal("a refused suspend changed the row")
	}
	if _, err = s.bind(randomUUID(), spOrigin(), "unknown store", 30*24*time.Hour); !errors.Is(err, platform.ErrScopeNotFound) {
		t.Fatalf("bind for an unknown store: %v", err)
	}
	if _, err = storefrontadmin.SuspendDomain(ctx, s.reg, spOrigin()); !errors.Is(err, platform.ErrScopeNotFound) {
		t.Fatalf("suspend of an unknown origin: %v", err)
	}

	// Audit trail of the first store: one operator.* row per real change, attributed to the store's creating principal.
	wantAudit := []string{
		"merchant.storefront_published", "operator.domain_bound", "merchant.storefront_unpublished", "merchant.storefront_published",
		"operator.domain_bound", "operator.domain_suspended", "operator.domain_bound", "operator.domain_detached",
		"operator.domain_bound:rebind_from_detached", "operator.domain_bound:rebind_from_detached",
	}
	if got := s.audits(f.store); fmt.Sprint(got) != fmt.Sprint(wantAudit) {
		t.Fatalf("audit actions:\n got %v\nwant %v", got, wantAudit)
	}
	if n := countRows(t, b.owner, `SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND tenant_id=$2 AND principal_id=$3 AND action LIKE 'operator.domain%'`, f.store, f.tenant, f.principal); n != 7 {
		t.Fatalf("operator audit rows attributed to the store's creating principal = %d, want 7", n)
	}
	// Concurrent binds of one new origin from two stores: one row, the loser sees owned-elsewhere, never a raw unique violation.
	race := spOrigin()
	var wg sync.WaitGroup
	errs := make([]error, 8)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			store := f.store
			if i%2 == 1 {
				store = f.otherStore
			}
			_, errs[i] = s.bind(store, race, fmt.Sprintf("race %d", i), 30*24*time.Hour)
		}()
	}
	wg.Wait()
	for i, e := range errs {
		if e != nil && !errors.Is(e, storefrontadmin.ErrDomainOwnedElsewhere) {
			t.Fatalf("concurrent bind %d: %v (23505 would mean the advisory lock is missing)", i, e)
		}
	}
	if n := countRows(t, b.owner, `SELECT count(*) FROM control.storefront_domains WHERE origin=$1`, race); n != 1 {
		t.Fatalf("concurrent binds created %d rows", n)
	}
}

func TestStorefrontPublishSPW06OperatorInputBoundaries(t *testing.T) {
	s := spSetup(t)
	f := s.f
	ctx := context.Background()
	future := time.Now().Add(60 * 24 * time.Hour)
	call := func(store, origin, evidence any, until any) error {
		var raw []byte
		if t, ok := until.(time.Time); ok {
			until = t.UTC().Format(time.RFC3339Nano)
		}
		return s.reg.QueryRow(ctx, `SELECT control.operator_bind_domain($1::uuid,$2,$3,$4::text::timestamptz)`, store, origin, evidence, until).Scan(&raw)
	}
	good := spOrigin()
	for name, origin := range map[string]any{
		"http": "http://x.test", "localhost": "https://localhost", "dot localhost": "https://a.localhost", "uppercase": "https://UPPER.test",
		"path": good + "/", "port": good + ":8443", "leading hyphen": "https://-x.test", "ip": "https://1.2.3.4", "single label": "https://shop",
		"numeric tld": "https://x.123", "userinfo": "https://u@x.test", "empty": "", "null": nil, "space": "https://x .test", "too long": "https://" + strings.Repeat("a", 250) + ".test",
	} {
		if c := pgCode(call(f.store, origin, "evidence", future)); c != "PT400" {
			t.Fatalf("origin %q (%s): SQLSTATE %q, want PT400", fmt.Sprint(origin), name, c)
		}
	}
	for name, ev := range map[string]any{"empty": "", "blank": "   ", "control char": "a\nb", "tab": "a\tb", "241 chars": strings.Repeat("e", 241), "null": nil} {
		if c := pgCode(call(f.store, good, ev, future)); c != "PT400" {
			t.Fatalf("evidence %s: SQLSTATE %q, want PT400", name, c)
		}
	}
	for name, until := range map[string]any{
		"past": time.Now().Add(-time.Hour), "just passed": time.Now().Add(-5 * time.Second), "401 days": time.Now().Add(401 * 24 * time.Hour), "10 years": time.Now().Add(3650 * 24 * time.Hour),
		"infinity": "infinity", "null": nil,
	} {
		if c := pgCode(call(f.store, good, "evidence", until)); c != "PT400" {
			t.Fatalf("valid_until %s: SQLSTATE %q, want PT400 (an ACTIVE proof must expire within a certificate lifetime)", name, c)
		}
	}
	if c := pgCode(call(nil, good, "evidence", future)); c != "PT400" {
		t.Fatalf("null store: %q", c)
	}
	if n := countRows(t, f.base.owner, `SELECT count(*) FROM control.storefront_domains WHERE origin=$1`, good); n != 0 {
		t.Fatal("a refused bind left a row")
	}
	// Boundaries that must pass: 399 days, a 240-character evidence, a 253-octet-class hostname.
	if err := call(f.store, good, strings.Repeat("e", 240), time.Now().Add(399*24*time.Hour)); err != nil {
		t.Fatalf("399 days / 240 chars refused: %v", err)
	}
	// Go-side mirror: the same bounds fail before a connection would open.
	for _, o := range []string{"http://x.test", "https://localhost", "https://X.test"} {
		if _, err := storefrontadmin.BindDomain(ctx, s.reg, f.store, o, "e", future, time.Now()); !errors.Is(err, command.ErrInvalid) {
			t.Fatalf("Go validator accepted %s: %v", o, err)
		}
	}
	for _, o := range []string{"http://x.test", "https://localhost", "https://X.test", ""} {
		if _, err := storefrontadmin.SuspendDomain(ctx, s.reg, o); !errors.Is(err, command.ErrInvalid) {
			t.Fatalf("suspend accepted %q: %v", o, err)
		}
		var raw []byte
		if c := pgCode(s.reg.QueryRow(ctx, `SELECT control.operator_detach_domain($1)`, o).Scan(&raw)); c != "PT400" {
			t.Fatalf("SQL detach accepted %q: %q", o, c)
		}
	}
	var raw []byte
	eq(t, "status of NULL store", pgCode(s.reg.QueryRow(ctx, `SELECT control.operator_storefront_status(NULL)`).Scan(&raw)), "PT400")
}

func TestStorefrontPublishSPW07ExecuteMatrix(t *testing.T) {
	s := spSetup(t)
	f := s.f
	ctx := context.Background()
	hash := sha256.Sum256([]byte(f.token))
	type call struct {
		sql  string
		args []any
	}
	calls := map[string]call{ // fn -> a syntactically valid call
		"control.read_storefront(bytea,uuid)":                         {`SELECT control.read_storefront($1,$2::uuid)`, []any{hash[:], f.store}},
		"control.set_storefront_published(bytea,uuid,boolean,bigint)": {`SELECT control.set_storefront_published($1,$2::uuid,true,0)`, []any{hash[:], f.store}},
		"control.operator_bind_domain(uuid,text,text,timestamptz)":    {`SELECT control.operator_bind_domain($1::uuid,'https://x-matrix.test','e',now()+interval '30 days')`, []any{f.store}},
		"control.operator_suspend_domain(text)":                       {`SELECT control.operator_suspend_domain('https://x-matrix.test')`, nil},
		"control.operator_detach_domain(text)":                        {`SELECT control.operator_detach_domain('https://x-matrix.test')`, nil},
		"control.operator_storefront_status(uuid)":                    {`SELECT control.operator_storefront_status($1::uuid)`, []any{f.store}},
	}
	run := func(pool *pgxpool.Pool, c call) error {
		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx) // the registrar's bind really runs: it is rolled back
		_, err = tx.Exec(ctx, c.sql, c.args...)
		return err
	}
	// Every other service login shape (inherit_noset) of the deployment: none may reach any storefront definer.
	for _, authority := range []string{"commerce_identity", "commerce_buyer_runtime", "commerce_buyer_issuer", "commerce_checkout_runtime",
		"commerce_worker", "commerce_meta_ingress", "commerce_meta_registrar", "commerce_payment_registrar", "commerce_stripe_ingress",
		"commerce_claims_intake", "commerce_auth"} {
		t.Run(authority, func(t *testing.T) {
			pool, err := pgxpool.New(ctx, miRole(t, f.base, authority))
			if err != nil {
				t.Fatal(err)
			}
			defer pool.Close()
			for fn, c0 := range calls {
				if c := pgCode(run(pool, c0)); c != "42501" {
					t.Fatalf("%s as %s: SQLSTATE %q, want 42501 permission denied", fn, authority, c)
				}
			}
		})
	}
	// The merchant runtime login: merchant functions are callable (PT403: no scope), operator functions are not.
	for fn, c0 := range calls {
		c := pgCode(run(f.base.runtime, c0))
		if strings.Contains(fn, "operator_") && c != "42501" {
			t.Fatalf("%s as the merchant runtime login: %q, want 42501", fn, c)
		}
		if !strings.Contains(fn, "operator_") && c != "PT403" {
			t.Fatalf("%s as the merchant runtime login: %q, want PT403 (executable, refused for missing scope)", fn, c)
		}
	}
	// The registrar login: operator functions run, merchant functions are denied.
	for fn, c0 := range calls {
		c := pgCode(run(s.reg, c0))
		if !strings.Contains(fn, "operator_") && c != "42501" {
			t.Fatalf("%s as the registrar login: %q, want 42501", fn, c)
		}
		if strings.Contains(fn, "operator_") && c == "42501" {
			t.Fatalf("%s denied to the registrar login", fn)
		}
	}
}

func TestStorefrontPublishSPW08RegistrarLoginShape(t *testing.T) {
	s := spSetup(t)
	ctx := context.Background()
	// logins.tsv row: authority, grant shape, consumer. The shape is what deploy/postgres/provision-logins.sh applies.
	raw, err := os.ReadFile("../../deploy/postgres/logins.tsv")
	if err != nil {
		t.Fatal(err)
	}
	var row []string
	for _, line := range strings.Split(string(raw), "\n") {
		if f := strings.Split(line, "\t"); !strings.HasPrefix(line, "#") && len(f) >= 5 && f[0] == "lc_store_registrar" {
			row = f
		}
	}
	if len(row) < 6 || row[1] != "commerce_storefront_registrar" || row[2] != "inherit_noset" || row[3] != "core" || row[4] != "store-admin:COMMERCE_STORE_REGISTRAR_DATABASE_URL" {
		t.Fatalf("logins.tsv lc_store_registrar row = %q", row)
	}
	// A login built exactly like provision-logins builds an inherit_noset login (miRole), then the matrix query of that script.
	var members []string
	var canLogin, super, bypass, createRole, createDB, repl, inherit, canUse, canSet, writer, predefined bool
	err = s.f.base.owner.QueryRow(ctx, `
		SELECT ARRAY(SELECT a.rolname::text FROM pg_roles a WHERE a.rolname LIKE 'commerce\_%' AND pg_has_role(l.oid,a.oid,'MEMBER') ORDER BY 1),
		  l.rolcanlogin,l.rolsuper,l.rolbypassrls,l.rolcreaterole,l.rolcreatedb,l.rolreplication,l.rolinherit,
		  pg_has_role(l.oid,'commerce_storefront_registrar','USAGE'),pg_has_role(l.oid,'commerce_storefront_registrar','SET'),
		  EXISTS (SELECT 1 FROM pg_roles w WHERE w.rolname LIKE '%\_writer' AND pg_has_role(l.oid,w.oid,'MEMBER')),
		  EXISTS (SELECT 1 FROM pg_roles p WHERE p.rolname LIKE 'pg\_%' AND p.rolname<>'pg_database_owner' AND pg_has_role(l.oid,p.oid,'MEMBER'))
		FROM pg_roles l WHERE l.rolname=$1`, mustLogin(t, s.regURL)).Scan(&members, &canLogin, &super, &bypass, &createRole, &createDB, &repl, &inherit, &canUse, &canSet, &writer, &predefined)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(members) != "[commerce_storefront_registrar]" || !canLogin || super || bypass || createRole || createDB || repl || !canUse || canSet || writer || predefined {
		t.Fatalf("lc_store_registrar-shaped login: members=%v login=%v super=%v bypass=%v createrole=%v createdb=%v repl=%v inherit=%v use=%v set=%v writer_member=%v predefined_member=%v",
			members, canLogin, super, bypass, createRole, createDB, repl, inherit, canUse, canSet, writer, predefined)
	}
	// SET FALSE: the login cannot become the authority (or the definer owner), so it never holds table rights.
	for _, role := range []string{"commerce_storefront_registrar", "commerce_storefront_writer", "commerce_runtime"} {
		if _, err := s.reg.Exec(ctx, `SET ROLE `+pgx.Identifier{role}.Sanitize()); pgCode(err) != "42501" {
			t.Fatalf("SET ROLE %s as the registrar login: %v, want 42501", role, err)
		}
	}
	for _, q := range []string{`SELECT 1 FROM control.storefront_domains LIMIT 1`, `SELECT 1 FROM control.storefront_publications LIMIT 1`,
		`INSERT INTO control.storefront_domains(tenant_id,store_id,origin) VALUES(gen_random_uuid(),gen_random_uuid(),'https://direct.test')`,
		`INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action) VALUES(gen_random_uuid(),gen_random_uuid(),gen_random_uuid(),'operator.domain_bound')`,
		`SELECT 1 FROM identity.sessions LIMIT 1`} {
		if _, err := s.reg.Exec(ctx, q); pgCode(err) != "42501" {
			t.Fatalf("registrar login ran %q directly: %v", q, err)
		}
	}
}

func mustLogin(t *testing.T, dsn string) string {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	return cfg.ConnConfig.User
}

// SPW09 builds cmd/store-admin and runs it as a process against the real registrar login.
func TestStorefrontPublishSPW09CLIProcess(t *testing.T) {
	s := spSetup(t)
	f := s.f
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(t.TempDir(), "store-admin")
	build := exec.Command("go", "build", "-o", bin, "./cmd/store-admin")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build cmd/store-admin: %v\n%s", err, out)
	}
	cfg, _ := pgxpool.ParseConfig(s.regURL)
	secret := cfg.ConnConfig.Password
	run := func(dsn string, args ...string) (int, string, string) {
		cmd := exec.Command(bin, args...)
		cmd.Env = []string{"PATH=" + os.Getenv("PATH")}
		if dsn != "" {
			cmd.Env = append(cmd.Env, "COMMERCE_STORE_REGISTRAR_DATABASE_URL="+dsn)
		}
		var so, se bytes.Buffer
		cmd.Stdout, cmd.Stderr = &so, &se
		err := cmd.Run()
		code := 0
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			code = ee.ExitCode()
		} else if err != nil {
			t.Fatalf("run %v: %v", args, err)
		}
		if strings.Contains(so.String()+se.String(), secret) || strings.Contains(so.String()+se.String(), "postgres://") {
			t.Fatalf("%v leaked the DSN/password: %q %q", args, so.String(), se.String())
		}
		return code, so.String(), se.String()
	}
	fail := func(want string, dsn string, args ...string) {
		t.Helper()
		code, so, se := run(dsn, args...)
		if code != 1 || so != "" || strings.TrimSpace(se) != want {
			t.Fatalf("%v: exit=%d stdout=%q stderr=%q, want exit 1, empty stdout, stderr %q", args, code, so, se, want)
		}
	}
	ok := func(args ...string) map[string]any {
		t.Helper()
		code, so, se := run(s.regURL, args...)
		var out map[string]any
		if code != 0 || se != "" || strings.Count(so, "\n") != 1 || json.Unmarshal([]byte(so), &out) != nil {
			t.Fatalf("%v: exit=%d stdout=%q stderr=%q, want exit 0 and exactly one JSON line", args, code, so, se)
		}
		return out
	}
	keys := func(m map[string]any) string {
		var ks []string
		for k := range m {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		return strings.Join(ks, ",")
	}
	origin := spOrigin()
	until := time.Now().Add(80 * 24 * time.Hour).UTC().Format(time.RFC3339)
	bind := func(store, ev, until string) []string {
		return []string{"domain-bind", "--store", store, "--origin", origin, "--evidence", ev, "--valid-until", until}
	}
	// usage / config failures never reach the database: they must not depend on a reachable DSN.
	fail("store_admin_usage", "", []string{}...)
	fail("store_admin_usage", s.regURL, "frobnicate")
	fail("store_admin_usage", s.regURL, "status")
	fail("store_admin_usage", s.regURL, "status", "--store", "not-a-uuid")
	fail("store_admin_usage", s.regURL, "status", "--store", f.store, "--extra", "x")
	fail("store_admin_usage", s.regURL, bind(f.store, "e", time.Now().Add(-time.Hour).UTC().Format(time.RFC3339))...)
	fail("store_admin_usage", s.regURL, bind(f.store, "e", time.Now().Add(500*24*time.Hour).UTC().Format(time.RFC3339))...)
	fail("store_admin_usage", s.regURL, bind(f.store, "e", "tomorrow")...)
	fail("store_admin_usage", s.regURL, "domain-bind", "--store", f.store, "--origin", "http://plain.test", "--evidence", "e", "--valid-until", until)
	fail("store_admin_usage", s.regURL, "domain-suspend", "--origin", "https://localhost")
	fail("store_admin_usage", s.regURL, "domain-detach")
	fail("store_admin_config", "", "status", "--store", f.store)
	if code, _, se := run("postgres://nobody@127.0.0.1:1/none?connect_timeout=2", "status", "--store", f.store); code != 1 || !regexp.MustCompile(`^store_admin_(failed|database)\n$`).MatchString(se) {
		t.Fatalf("unreachable database: exit=%d stderr=%q", code, se)
	}
	// Happy path: JSON lines of ids / versions / states only.
	got := ok(bind(f.store, "process proof 1", until)...)
	if keys(got) != "domain_id,rebound,renewed,state,version" || got["state"] != "ACTIVE" || got["version"] != float64(1) || got["renewed"] != false || got["rebound"] != false {
		t.Fatalf("domain-bind JSON: %v", got)
	}
	st := ok("status", "--store", f.store)
	doms, _ := st["domains"].([]any)
	if st["store_id"] != f.store || st["published"] != false || st["version"] != float64(0) || len(st) != 4 || len(doms) != 1 {
		t.Fatalf("status JSON: %v", st)
	}
	d := doms[0].(map[string]any)
	if d["origin"] != origin || d["state"] != "ACTIVE" || d["serving"] != true || d["version"] != float64(1) || len(d) != 5 {
		t.Fatalf("status domain JSON: %v", d)
	}
	if _, err := s.set(f.token, f.store, true, 0); err != nil {
		t.Fatal(err)
	}
	s.mustResolve(origin, f.store) // the process-bound origin serves once the merchant published
	if sus := ok("domain-suspend", "--origin", origin); sus["state"] != "SUSPENDED" || sus["changed"] != true || sus["version"] != float64(2) || len(sus) != 4 {
		t.Fatalf("domain-suspend JSON: %v", sus)
	}
	s.mustDeny(origin, "suspended by the CLI")
	if det := ok("domain-detach", "--origin", origin); det["state"] != "DETACHED" || det["changed"] != true || det["version"] != float64(3) {
		t.Fatalf("domain-detach JSON: %v", det)
	}
	fail("store_admin_domain_detached", s.regURL, "domain-suspend", "--origin", origin)
	fail("store_admin_domain_detached", s.regURL, bind(f.store, "process proof 1", until)...) // same evidence
	re := ok(bind(f.store, "process proof 2", until)...)
	if re["rebound"] != true || re["state"] != "ACTIVE" || re["version"] != float64(4) {
		t.Fatalf("re-bind JSON: %v", re)
	}
	s.mustResolve(origin, f.store)
	fail("store_admin_domain_owned_elsewhere", s.regURL, "domain-bind", "--store", f.otherStore, "--origin", origin, "--evidence", "e", "--valid-until", until)
	fail("store_admin_not_found", s.regURL, "status", "--store", randomUUID())
	fail("store_admin_not_found", s.regURL, "domain-suspend", "--origin", spOrigin())
	fail("store_admin_not_found", s.regURL, "domain-bind", "--store", randomUUID(), "--origin", spOrigin(), "--evidence", "e", "--valid-until", until)
	fail("store_admin_no_owner_principal", s.regURL, "domain-bind", "--store", s.storeNoOwner, "--origin", spOrigin(), "--evidence", "e", "--valid-until", until)
	// A non-registrar DSN is refused by the database, reduced to the fixed code.
	if code, _, se := run(miRole(t, f.base, "commerce_worker"), "status", "--store", f.store); code != 1 || strings.TrimSpace(se) != "store_admin_failed" {
		t.Fatalf("a worker login ran store-admin: exit=%d stderr=%q", code, se)
	}
}

// SPW10: static deploy wiring (no PG): the login exists once, only the one-shot mounts it, the wrapper allowlists exactly
// the four sub-commands, and provisioning knows the authority and checks its EXECUTE grants.
func TestStorefrontPublishSPW10DeployWiring(t *testing.T) {
	read := func(rel string) string {
		b, err := os.ReadFile(filepath.Join("..", "..", rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		return string(b)
	}
	manifest := read("deploy/secrets.manifest.tsv")
	for _, name := range []string{"pw_lc_store_registrar", "dsn_lc_store_registrar"} {
		if n := len(regexp.MustCompile(`(?m)^`+name+`\t`).FindAllString(manifest, -1)); n != 1 {
			t.Fatalf("secrets.manifest.tsv rows for %s = %d, want 1", name, n)
		}
	}
	if !regexp.MustCompile(`(?m)^dsn_lc_store_registrar\tdsn_tcp\tderive\tstore-admin\t`).MatchString(manifest) {
		t.Fatal("dsn_lc_store_registrar must be a derived DSN consumed by store-admin only")
	}
	compose := read("deploy/compose.yml")
	compose = compose[:strings.Index(compose, "\nnetworks:\n")] // top-level secrets declarations are not mounts
	services := regexp.MustCompile(`(?m)^  ([a-z][a-z0-9-]*):\n`).FindAllStringSubmatchIndex(compose, -1)
	mounts := 0
	for i, loc := range services {
		end := len(compose)
		if i+1 < len(services) {
			end = services[i+1][0]
		}
		name, block := compose[loc[2]:loc[3]], compose[loc[0]:end]
		if strings.Contains(block, "dsn_lc_store_registrar") {
			mounts++
			if name != "store-admin" || !strings.Contains(block, "profiles: [ops]") || !strings.Contains(block, `restart: "no"`) ||
				!strings.Contains(block, "COMMERCE_STORE_REGISTRAR_DATABASE_URL_FILE: /run/secrets/dsn_lc_store_registrar") {
				t.Fatalf("service %s mounts dsn_lc_store_registrar but is not the ops one-shot store-admin:\n%s", name, block)
			}
		}
		if name != "provision-logins" && name != "secrets-init" && strings.Contains(block, "pw_lc_store_registrar") {
			t.Fatalf("service %s mounts the registrar password", name)
		}
	}
	if mounts != 1 {
		t.Fatalf("services mounting dsn_lc_store_registrar = %d, want exactly store-admin", mounts)
	}
	ops := read("deploy/scripts/ops-admin.sh")
	if !strings.Contains(ops, "store-admin:domain-bind | store-admin:domain-suspend | store-admin:domain-detach | store-admin:status)") ||
		regexp.MustCompile(`store-admin:(publish|unpublish|domain-attach|\*)`).MatchString(ops) {
		t.Fatal("ops-admin.sh allowlist for store-admin is not exactly domain-bind|domain-suspend|domain-detach|status")
	}
	prov := read("deploy/postgres/provision-logins.sh")
	if !strings.Contains(prov, "('commerce_storefront_registrar')") || !strings.Contains(prov, "operator_bind_domain") ||
		!strings.Contains(prov, `"$store_reg" == 4`) || !strings.Contains(prov, "lc_store_registrar") {
		t.Fatal("provision-logins.sh does not list the authority in its matrix or verify the four EXECUTE grants")
	}
	rb := read("docs/runbooks/merchant-onboarding.md")
	for _, must := range []string{"domain-bind", "domain-suspend", "domain-detach", "--valid-until", "openssl", "网店发布", "store-admin status"} {
		if !strings.Contains(rb, must) {
			t.Fatalf("merchant-onboarding runbook lacks %q", must)
		}
	}
	// No service config may name the registrar DSN variable except the one-shot (the API reads only its own DSNs).
	for _, dir := range []string{"cmd/api", "cmd/admin-fixture", "internal/httpapi"} {
		_ = filepath.WalkDir(filepath.Join("..", "..", dir), func(p string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() && strings.HasSuffix(p, ".go") {
				if b, _ := os.ReadFile(p); strings.Contains(string(b), "COMMERCE_STORE_REGISTRAR_DATABASE_URL") {
					t.Errorf("%s reads the operator registrar DSN", p)
				}
			}
			return nil
		})
	}
}
