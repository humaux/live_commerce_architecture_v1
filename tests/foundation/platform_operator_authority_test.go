package foundation_test

// OP08 + schema/ACL inventory of migration 0143 (R3 unit OPS-01B), REAL_PG. Positive and negative controls:
//   - the operator login (commerce_platform_operator, INHERIT TRUE / SET FALSE) really runs each of its four definers;
//   - no other authority holds EXECUTE on them (inventory over every commerce_* role, plus real logins that are refused 42501);
//   - the guard predicate store_serving is executable by exactly the two definer owners that use it;
//   - the audit table is reachable by the definer owner only; the new roles are NOLOGIN, non-bypass, member-free.

import (
	"context"
	"strings"
	"testing"
)

var poOperatorFns = []string{
	"control.set_store_active(uuid,boolean,text,text,text)",
	"control.set_tenant_active(uuid,boolean,text,text,text)",
	"control.platform_status(uuid,uuid)",
	"control.read_operator_audit(timestamptz,integer)",
}

const poGuardFn = "control.store_serving(uuid,uuid)"

func TestPlatformOperatorOP08SchemaAndExecuteMatrix(t *testing.T) {
	b := fixture(t)
	ctx := context.Background()
	o := b.owner
	for _, fn := range append(append([]string{}, poOperatorFns...), poGuardFn) {
		var secdef bool
		var owner, cfg, comment string
		var acl []string
		if err := o.QueryRow(ctx, `SELECT p.prosecdef,pg_get_userbyid(p.proowner),coalesce(p.proconfig::text,''),coalesce(obj_description(p.oid,'pg_proc'),''),
			coalesce((SELECT array_agg(a.grantee::regrole::text ORDER BY a.grantee::regrole::text) FROM aclexplode(p.proacl) a WHERE a.privilege_type='EXECUTE'),'{}')
			FROM pg_proc p WHERE p.oid=$1::regprocedure`, fn).Scan(&secdef, &owner, &cfg, &comment, &acl); err != nil {
			t.Fatalf("%s missing: %v", fn, err)
		}
		if !secdef || owner != "commerce_platform_writer" || !strings.Contains(cfg, "search_path=pg_catalog") || comment == "" {
			t.Fatalf("%s: secdef=%v owner=%s config=%s comment=%q", fn, secdef, owner, cfg, comment)
		}
		want := []string{"commerce_platform_operator", "commerce_platform_writer"}
		if fn == poGuardFn {
			want = []string{"commerce_checkout_writer", "commerce_integration_writer", "commerce_platform_writer"}
		}
		lcSameSet(t, fn+" EXECUTE grantees", acl, want)
	}
	// Inventory: every other commerce_* role (workers, runtimes, registrars, ingress...) holds no EXECUTE on the operator definers.
	for _, fn := range poOperatorFns {
		over := lcStrings(t, o, `SELECT r.rolname FROM pg_roles r WHERE r.rolname LIKE 'commerce\_%' AND r.rolname NOT IN ('commerce_platform_operator','commerce_platform_writer')
			AND has_function_privilege(r.oid,$1::regprocedure,'EXECUTE')`, fn)
		if len(over) != 0 {
			t.Fatalf("%s is executable by %v", fn, over)
		}
	}
	// Nobody but the definer owner has any privilege on the audit table; the operator has none on any table it touches.
	acl := lcStrings(t, o, `SELECT a.grantee::regrole::text||':'||a.privilege_type FROM pg_class c, aclexplode(c.relacl) a WHERE c.oid='control.operator_audit'::regclass
		AND a.grantee NOT IN (c.relowner) ORDER BY 1`)
	lcSameSet(t, "operator_audit table ACL", acl, []string{"commerce_platform_writer:INSERT", "commerce_platform_writer:SELECT"})
	for _, tbl := range []string{"control.operator_audit", "control.stores", "control.tenants"} {
		var any bool
		if err := o.QueryRow(ctx, `SELECT has_table_privilege('commerce_platform_operator',$1,'SELECT,INSERT,UPDATE,DELETE') OR has_any_column_privilege('commerce_platform_operator',$1,'SELECT,INSERT,UPDATE')`, tbl).Scan(&any); err != nil || any {
			t.Fatalf("operator authority has direct access to %s (err=%v)", tbl, err)
		}
	}
	// Writer grants are column-narrow: it flips `active` and reads ids only.
	cols := lcStrings(t, o, `SELECT c.relname||'.'||a.attname||':'||x.privilege_type FROM pg_class c JOIN pg_attribute a ON a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped,
		LATERAL aclexplode(a.attacl) x WHERE c.oid IN ('control.stores'::regclass,'control.tenants'::regclass) AND x.grantee='commerce_platform_writer'::regrole ORDER BY 1`)
	lcSameSet(t, "writer column grants", cols, []string{"stores.id:SELECT", "stores.tenant_id:SELECT", "stores.active:SELECT", "stores.active:UPDATE",
		"tenants.id:SELECT", "tenants.active:SELECT", "tenants.active:UPDATE"})
	// Role shape: NOLOGIN, no superuser/bypass/createdb/createrole/replication, no members.
	for _, role := range []string{"commerce_platform_writer", "commerce_platform_operator"} {
		var login, super, bypass, createDB, createRole, repl bool
		if err := o.QueryRow(ctx, `SELECT rolcanlogin,rolsuper,rolbypassrls,rolcreatedb,rolcreaterole,rolreplication FROM pg_roles WHERE rolname=$1`, role).Scan(&login, &super, &bypass, &createDB, &createRole, &repl); err != nil ||
			login || super || bypass || createDB || createRole || repl {
			t.Fatalf("%s role shape login=%v super=%v bypass=%v createdb=%v createrole=%v repl=%v err=%v", role, login, super, bypass, createDB, createRole, repl, err)
		}
		// test logins created by earlier tests are dropped in cleanup; any member left would be a real grant
		if n := countRows(t, o, `SELECT count(*) FROM pg_auth_members m JOIN pg_roles r ON r.oid=m.member WHERE m.roleid=$1::regrole AND r.rolname NOT LIKE 'mi\_test\_%'`, role); n != 0 {
			t.Fatalf("%s has %d unexpected members", role, n)
		}
	}
}

// Real logins of other authorities are refused by the database (the CLI reduces it to platform_admin_denied); the operator
// login runs every definer (positive control: a validator rejecting everyone would pass the negatives).
func TestPlatformOperatorOP08LoginsRefusedAndOperatorRuns(t *testing.T) {
	s := spSetup(t)
	f, b := s.f, s.f.base
	ctx := context.Background()
	calls := []struct {
		sql  string
		args []any
	}{
		{`SELECT control.set_store_active($1::uuid,false,'op','T-1','fraud')`, []any{f.store}},
		{`SELECT control.set_tenant_active($1::uuid,false,'op','T-1','fraud')`, []any{f.tenant}},
		{`SELECT control.platform_status($1::uuid,NULL)`, []any{f.tenant}},
		{`SELECT * FROM control.read_operator_audit(NULL,1)`, nil},
	}
	for _, authority := range []string{"commerce_runtime", "commerce_identity", "commerce_buyer_runtime", "commerce_buyer_issuer", "commerce_checkout_runtime",
		"commerce_worker", "commerce_claims_worker", "commerce_expiry_worker", "commerce_meta_ingress", "commerce_meta_registrar", "commerce_payment_registrar",
		"commerce_stripe_ingress", "commerce_claims_intake", "commerce_auth", "commerce_storefront_registrar"} {
		pool := miPool(t, b, authority)
		for _, c := range calls {
			if _, err := pool.Exec(ctx, c.sql, c.args...); sqlState(err) != "42501" {
				t.Errorf("%s login ran %q: SQLSTATE %q (err %v), want 42501", authority, c.sql, sqlState(err), err)
			}
		}
	}
	// the refused calls changed nothing
	if n := countRows(t, b.owner, `SELECT count(*) FROM control.stores WHERE id=$1 AND active`, f.store); n != 1 {
		t.Fatal("a refused login suspended the store")
	}
	p := poSetup(t, b)
	p.must(p.json(`SELECT control.platform_status($1,NULL)::text`, f.tenant))
	p.must(p.setStore(f.store, false, "other"))
	p.must(p.setStore(f.store, true, nil))
	p.must(p.setTenant(f.tenant, false, "other"))
	p.must(p.setTenant(f.tenant, true, nil))
	if n := countRows(t, p.op, `SELECT count(*) FROM control.read_operator_audit(NULL,500) WHERE tenant_id=$1`, f.tenant); n != 4 {
		t.Fatalf("operator login read %d audit rows, want 4", n)
	}
	// the operator login cannot become the writer / owner
	for _, role := range []string{"commerce_platform_writer", "commerce_runtime"} {
		if _, err := p.op.Exec(ctx, `SET ROLE `+role); sqlState(err) != "42501" {
			t.Errorf("operator login SET ROLE %s: %v, want 42501", role, err)
		}
	}
}
