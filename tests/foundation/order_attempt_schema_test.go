// Purpose: independently pin OA1's frozen durable identities and authority boundary.
// Depends: fixture/newFixture, real migrated PostgreSQL catalogs; order-attempt-v1 §§1–3,6.
// Used: foundation CI; owner connection reads metadata only, never simulates execution.
// Invariants: I01/I02/I14/I18/I21; all-state replay fences and least privilege.
// Scope: schema evidence only; C1/C2 behavior, crash/recovery and actual-login calls
// require separate acceptance tests after the transaction API is frozen.
package foundation_test

import (
	"context"
	"testing"
)

func TestOrderAttemptSchemaDurableIdentities(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	for _, tc := range []struct {
		table   string
		columns []string
	}{
		{"ops.order_attempt_scopes", []string{"tenant_id", "store_id", "principal_id", "operation_family"}},
		{"ops.order_attempts", []string{"tenant_id", "operation_family", "transport_key_digest"}},
		{"ops.order_attempts", []string{"scope_id", "intent_generation"}},
		{"ops.order_attempt_subjects", []string{"tenant_id", "store_id", "subject_kind", "subject_id", "attempt_id"}},
	} {
		t.Run(tc.table+"/"+tc.columns[len(tc.columns)-1], func(t *testing.T) {
			var present bool
			// No state predicate is allowed on any durable identity. In particular,
			// adding body/hash to the key would permit a changed body to reuse it.
			err := f.owner.QueryRow(ctx, `SELECT EXISTS (
			 SELECT 1 FROM pg_index i WHERE i.indrelid=to_regclass($1)
			 AND i.indisunique AND i.indisvalid AND i.indisready AND i.indislive
			 AND i.indpred IS NULL AND i.indexprs IS NULL
			 AND ARRAY(SELECT a.attname::text FROM unnest(i.indkey) WITH ORDINALITY k(attnum,pos)
			   JOIN pg_attribute a ON a.attrelid=i.indrelid AND a.attnum=k.attnum
			   WHERE k.pos<=i.indnkeyatts ORDER BY a.attname::text)
			 = ARRAY(SELECT c FROM unnest($2::text[]) c ORDER BY c)
			 AND NOT EXISTS (SELECT 1 FROM unnest(i.indkey) WITH ORDINALITY k(attnum,pos)
			   JOIN pg_attribute a ON a.attrelid=i.indrelid AND a.attnum=k.attnum
			   WHERE k.pos<=i.indnkeyatts AND NOT a.attnotnull)
			)`, tc.table, tc.columns).Scan(&present)
			if err != nil || !present {
				t.Fatalf("OA1 missing non-null all-state UNIQUE %s %v: present=%v err=%v", tc.table, tc.columns, present, err)
			}
		})
	}

	t.Run("one_pending_per_scope", func(t *testing.T) {
		var present bool
		err := f.owner.QueryRow(ctx, `SELECT EXISTS (
		 SELECT 1 FROM pg_index i WHERE i.indrelid=to_regclass('ops.order_attempts')
		 AND i.indisunique AND i.indisvalid AND i.indisready AND i.indislive
		 AND i.indnkeyatts=1 AND i.indexprs IS NULL
		 AND (SELECT a.attname FROM pg_attribute a
		   WHERE a.attrelid=i.indrelid AND a.attnum=i.indkey[0])='scope_id'
		 AND regexp_replace(pg_get_expr(i.indpred,i.indrelid),
		   '\s|\(|\)|::[a-z_][a-z_0-9.]*','','g')='state=''PENDING'''
		)`).Scan(&present)
		if err != nil || !present {
			t.Fatalf("OA1 missing UNIQUE scope_id WHERE state=PENDING: present=%v err=%v", present, err)
		}
	})

	t.Run("subject_erasure_lookup", func(t *testing.T) {
		var present bool
		err := f.owner.QueryRow(ctx, `SELECT EXISTS (
		 SELECT 1 FROM pg_index i WHERE i.indrelid=to_regclass('ops.order_attempt_subjects')
		 AND i.indisvalid AND i.indisready AND i.indislive AND i.indpred IS NULL
		 AND ARRAY(SELECT a.attname::text FROM unnest(i.indkey) WITH ORDINALITY k(attnum,pos)
		   JOIN pg_attribute a ON a.attrelid=i.indrelid AND a.attnum=k.attnum
		   WHERE k.pos<=5 AND k.pos<=i.indnkeyatts ORDER BY k.pos)
		 = ARRAY['tenant_id','store_id','subject_kind','subject_id','attempt_id']::text[]
		)`).Scan(&present)
		if err != nil || !present {
			t.Fatalf("OA1 missing ordered erasure-subject lookup: present=%v err=%v", present, err)
		}
	})
}

func TestOrderAttemptSchemaTableAuthority(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	for _, table := range []string{"ops.order_attempt_scopes", "ops.order_attempts", "ops.order_attempt_subjects"} {
		t.Run(table, func(t *testing.T) {
			var valid bool
			err := f.owner.QueryRow(ctx, `SELECT EXISTS (
			 SELECT 1 FROM pg_class c JOIN pg_roles r ON r.oid=c.relowner
			 WHERE c.oid=to_regclass($1) AND c.relkind IN ('r','p')
			 AND c.relrowsecurity AND c.relforcerowsecurity
			 AND r.rolname='commerce_order_attempt_owner'
			 AND NOT (r.rolcanlogin OR r.rolsuper OR r.rolbypassrls)
			)`, table).Scan(&valid)
			if err != nil || !valid {
				t.Fatalf("OA1 table absent or missing FORCE RLS/non-login owner: %s valid=%v err=%v", table, valid, err)
			}
			for _, role := range []string{"commerce_runtime", "commerce_claims_worker", "commerce_worker", "commerce_privacy_writer", "commerce_buyer_runtime", "commerce_checkout_runtime"} {
				var exists, allowed bool
				err := f.owner.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname=$1),
				 EXISTS(SELECT 1 FROM pg_roles r WHERE r.rolname=$1 AND (
				 has_table_privilege(r.oid,to_regclass($2),'SELECT,INSERT,UPDATE,DELETE,TRUNCATE,REFERENCES,TRIGGER')
				 OR has_any_column_privilege(r.oid,to_regclass($2),'SELECT,INSERT,UPDATE,REFERENCES')))
				`, role, table).Scan(&exists, &allowed)
				if err != nil || !exists || allowed {
					t.Errorf("OA1 direct table authority %s on %s: role_exists=%v allowed=%v err=%v", role, table, exists, allowed, err)
				}
			}
		})
	}

	t.Run("coordinator_has_no_domain_owner_membership", func(t *testing.T) {
		var safe bool
		err := f.owner.QueryRow(ctx, `SELECT EXISTS (
		 SELECT 1 FROM pg_roles owner WHERE owner.rolname='commerce_order_attempt_owner'
		 AND NOT EXISTS (SELECT 1 FROM pg_auth_members m WHERE m.member=owner.oid)
		 AND NOT (owner.rolcanlogin OR owner.rolsuper OR owner.rolbypassrls
		   OR owner.rolcreatedb OR owner.rolcreaterole OR owner.rolreplication)
		)`).Scan(&safe)
		if err != nil || !safe {
			t.Fatalf("OA1 coordinator role missing or elevated: safe=%v err=%v", safe, err)
		}
	})
}

func TestOrderAttemptSchemaExecutionDefinerACL(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	// Names are frozen by §1; parameter signatures are deliberately not guessed.
	// Every overload must have the same boundary, including future overloads.
	for _, name := range []string{"lock_order_attempt", "execute_order_attempt", "abort_order_attempt"} {
		t.Run(name, func(t *testing.T) {
			var count, unsafe int
			err := f.owner.QueryRow(ctx, `SELECT count(*),count(*) FILTER (WHERE NOT (
			 p.prosecdef AND r.rolname='commerce_order_attempt_owner'
			 AND NOT (r.rolcanlogin OR r.rolsuper OR r.rolbypassrls)
			 AND coalesce(p.proconfig @> ARRAY['search_path=pg_catalog']::text[],false)
			 AND coalesce(length(obj_description(p.oid,'pg_proc')),0)>0
			 AND ARRAY(SELECT pg_get_userbyid(a.grantee)::text
			   FROM aclexplode(coalesce(p.proacl,acldefault('f',p.proowner))) a
			   WHERE a.privilege_type='EXECUTE' AND a.grantee<>p.proowner ORDER BY 1)
			   = ARRAY['commerce_claims_worker','commerce_runtime']::text[]
			 AND NOT EXISTS (SELECT 1 FROM aclexplode(coalesce(p.proacl,acldefault('f',p.proowner))) a
			   WHERE a.grantee<>p.proowner AND a.is_grantable)
			 AND NOT EXISTS (SELECT 1 FROM pg_roles other
			   WHERE other.rolname LIKE 'commerce\_%' AND other.oid<>p.proowner
			   AND other.rolname NOT IN ('commerce_claims_worker','commerce_runtime')
			   AND has_function_privilege(other.oid,p.oid,'EXECUTE'))
			)) FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
			 JOIN pg_roles r ON r.oid=p.proowner WHERE n.nspname='ops' AND p.proname=$1`, name).Scan(&count, &unsafe)
			if err != nil || count == 0 || unsafe != 0 {
				t.Fatalf("OA1 definer ops.%s: overloads=%d unsafe=%d err=%v", name, count, unsafe, err)
			}
		})
	}
}
