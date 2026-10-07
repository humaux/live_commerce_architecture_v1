// File: tests/foundation/worker_authority_split_test.go
// Purpose: WAS gates of unit worker-authority-split (T21-02 / T21-03): one DB authority per worker process. WAS01 static login
//
//	mapping; WAS02 a compromised non-payment worker holds no payment privilege (the FINDINGS T21-02 test); WAS03 cross-authority
//	claim_operation/complete_operation refused before any mutation; WAS04 RLS lanes (an authority only sees its own operations);
//	WAS05 the execution-profile fence is derived from the caller's authority, not from p_profile (FINDINGS T21-03 test);
//	WAS06 the privilege inventory behind output/worker-authority-split/privileges-*.tsv (evidence generator, skipped without
//	LC_WAS_INVENTORY).
//
// Runs as/in: the shared fixture PG (REAL_PG, MOCK: no provider is called; operations are synthetic rows).
// Reads env / secrets: LC_WAS_INVENTORY (output file path, WAS06 only).
// Used by: bash scripts/dev/test-focused.sh 'TestWAS'.
// Status: REAL_PG once run by the integrator/tester; NOT_RUN in the author's static pass.
package foundation_test

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// WAS01 (static, no PG): every worker login of deploy/postgres/logins.tsv joins its own authority and none joins the legacy role.
func TestWAS01LoginsMapToTheirOwnAuthority(t *testing.T) {
	raw, err := os.ReadFile("../../deploy/postgres/logins.tsv")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"lc_payment_sandbox": waPayment, "lc_payment_live": waLive, "lc_expiry_worker": waExpiry,
		"lc_ads_worker": waAds, "lc_claims_worker": waClaims}
	got := map[string]string{}
	for _, line := range strings.Split(string(raw), "\n") {
		f := strings.Split(line, "\t")
		if strings.HasPrefix(line, "#") || len(f) < 6 {
			continue
		}
		if f[1] == waLegacy {
			t.Errorf("login %s still joins the legacy commerce_worker", f[0])
		}
		got[f[0]] = f[1]
	}
	for login, authority := range want {
		if got[login] != authority {
			t.Errorf("login %s authority=%q want %q", login, got[login], authority)
		}
	}
}

// was lane fixtures: one synthetic operation per lane, owner-inserted (the owner is a superuser, RLS does not apply to it).
type wasOp struct{ provider, actor, id string }

func wasOperation(t *testing.T, f *testFixture, provider, actor string) wasOp {
	t.Helper()
	ctx := context.Background()
	tenant, store, principal, binding, operation := randomUUID(), randomUUID(), randomUUID(), randomUUID(), randomUUID()
	tx, err := f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	// actor_kind other than MERCHANT needs checkout/attempt rows; the lane tests use MERCHANT operations whose provider decides the lane.
	if actor != "MERCHANT" {
		t.Fatalf("wasOperation supports MERCHANT only, got %s", actor)
	}
	for _, s := range []struct {
		q    string
		args []any
	}{
		{`INSERT INTO control.tenants(id,name) VALUES($1,'WAS synthetic')`, []any{tenant}},
		{`INSERT INTO control.stores(tenant_id,id,name,currency) VALUES($1,$2,'WAS store','TWD')`, []any{tenant, store}},
		{`INSERT INTO identity.principals(id) VALUES($1)`, []any{principal}},
		{`INSERT INTO identity.memberships(tenant_id,principal_id) VALUES($1,$2)`, []any{tenant, principal}},
		{`INSERT INTO integration.bindings(id,tenant_id,store_id,principal_id,provider,external_asset_id) VALUES($1,$2,$3,$4,$5,'was-asset')`, []any{binding, tenant, store, principal, provider}},
		{`INSERT INTO integration.operations(id,tenant_id,store_id,principal_id,binding_id,binding_version,provider,external_asset_id,purpose,action,semantic_key,request_hash,request,job_id)
		  VALUES($1,$2,$3,$4,$5,1,$6,'was-asset','transactional','was.lane',$7,decode(repeat('02',32),'hex'),'{}',1)`, []any{operation, tenant, store, principal, binding, provider, "was:" + operation}},
	} {
		if _, err = tx.Exec(ctx, s.q, s.args...); err != nil {
			t.Fatalf("fixture %q: %v", s.q[:40], err)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return wasOp{provider, actor, operation}
}

func wasLogin(t *testing.T, role string) *pgxpool.Pool {
	t.Helper()
	_, p := t06AuthorityLogin(t, role)
	return p
}

// WAS02 is the T21-02 test of output/r3-t21-review/FINDINGS.md: a login of the ads, claims or expiry authority holds no
// payment-queue or Stripe/apply_capture privilege, while the two payment authorities keep exactly what their process uses.
func TestWAS02NonPaymentWorkersHoldNoPaymentPrivilege(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	funcs := []string{
		"integration.record_stripe_observation(uuid,bigint,bytea,text,jsonb,bigint,text)", "payments.apply_capture(uuid,bytea)",
		"integration.load_stripe_credential(uuid,bigint,bytea,text)", "integration.load_stripe_session(uuid,bigint,bytea,text)",
		"integration.finish_stripe_refund(uuid,bigint,bytea,text,text)", "integration.record_stripe_refund_observation(uuid,bigint,bytea,text,jsonb,bigint)",
		"integration.record_stripe_charge_observation(uuid,bigint,bytea,text,jsonb,bigint)", "integration.load_payment_query(uuid,bigint,bytea,text)",
		"integration.payment_queue_ready()",
	}
	for _, role := range []string{waAds, waClaims, waExpiry, waLegacy} {
		t.Run("no payment privilege "+strings.TrimPrefix(role, "commerce_"), func(t *testing.T) {
			for _, rel := range []string{"river_payment.river_job", "river_payment.river_leader"} {
				var any bool
				if err := f.owner.QueryRow(ctx, `SELECT has_table_privilege($1,$2::regclass,'SELECT,INSERT,UPDATE,DELETE') OR has_any_column_privilege($1,$2::regclass,'SELECT,INSERT,UPDATE')`, role, rel).Scan(&any); err != nil || any {
					t.Errorf("%s holds privileges on %s (err=%v)", role, rel, err)
				}
			}
			var seq bool
			if err := f.owner.QueryRow(ctx, `SELECT has_sequence_privilege($1,'river_payment.river_job_id_seq','USAGE,SELECT,UPDATE')`, role).Scan(&seq); err != nil || seq {
				t.Errorf("%s holds river_payment.river_job_id_seq (err=%v)", role, seq)
			}
			for _, fn := range funcs {
				var exec bool
				if err := f.owner.QueryRow(ctx, `SELECT has_function_privilege($1,$2::regprocedure,'EXECUTE')`, role, fn).Scan(&exec); err != nil || exec {
					t.Errorf("%s may EXECUTE %s (err=%v)", role, fn, err)
				}
			}
		})
	}
	// The same non-payment logins are refused at the SQL level (42501 on the table, 42501 on the function), not just by catalog flags.
	for _, role := range []string{waAds, waClaims, waExpiry} {
		p := wasLogin(t, role)
		if _, err := p.Exec(ctx, `SELECT args FROM river_payment.river_job LIMIT 1`); sqlState(err) != "42501" {
			t.Errorf("%s read river_payment.river_job: %v", role, err)
		}
		if _, err := p.Exec(ctx, `SELECT integration.record_stripe_observation($1::uuid,2,$2::bytea,'SANDBOX','{}'::jsonb,1,'')`, randomUUID(), randomBytes(32)); sqlState(err) != "42501" {
			t.Errorf("%s ran record_stripe_observation: %v", role, err)
		}
		if _, err := p.Exec(ctx, `SELECT payments.apply_capture($1::uuid,$2::bytea)`, randomUUID(), randomBytes(32)); sqlState(err) != "42501" {
			t.Errorf("%s ran payments.apply_capture: %v", role, err)
		}
	}
	// And each payment authority holds the queue and the definers its process runs.
	for _, role := range []string{waPayment, waLive} {
		var q, e bool
		if err := f.owner.QueryRow(ctx, `SELECT has_table_privilege($1,'river_payment.river_job','SELECT,INSERT,UPDATE,DELETE'),
			has_function_privilege($1,'payments.apply_capture(uuid,bytea)','EXECUTE')`, role).Scan(&q, &e); err != nil || !q || !e {
			t.Errorf("%s lost its payment queue or apply_capture: queue=%v exec=%v err=%v", role, q, e, err)
		}
		var exp bool
		if err := f.owner.QueryRow(ctx, `SELECT has_table_privilege($1,'river_expiry.river_job','SELECT')`, role).Scan(&exp); err != nil || exp {
			t.Errorf("%s reads river_expiry.river_job (err=%v)", role, err)
		}
	}
	// The expiry authority holds only river_expiry + the checkout expiry definers; the others lost them.
	for _, role := range []string{waPayment, waLive, waAds, waClaims, waLegacy} {
		var any bool
		if err := f.owner.QueryRow(ctx, `SELECT has_table_privilege($1,'river_expiry.river_job','SELECT,INSERT,UPDATE,DELETE') OR has_function_privilege($1,'checkout.expire_held(uuid,bigint)','EXECUTE') OR has_function_privilege($1,'checkout.expiry_queue_ready()','EXECUTE')`, role).Scan(&any); err != nil || any {
			t.Errorf("%s holds expiry privileges (err=%v)", role, err)
		}
	}
	// The ads queue/guard is ads-only: a claims-worker login cannot insert an ads-lane River job.
	claims := wasLogin(t, waClaims)
	if _, err := claims.Exec(ctx, `INSERT INTO river.river_job(kind,queue,args,priority,max_attempts) VALUES('ads_oauth_purge_v1','ads','{}'::jsonb,2,1)`); err == nil {
		t.Error("claims worker inserted an ads periodic job")
		_, _ = f.owner.Exec(ctx, `DELETE FROM river.river_job WHERE kind='ads_oauth_purge_v1' AND queue='ads'`)
	}
	// 0130: the pick-list reader and its session helper are merchant-side only — no worker login (or the
	// empty legacy role) may call either; the refusal is the SQL 42501, not just a catalog flag.
	for _, role := range []string{waPayment, waLive, waExpiry, waAds, waClaims, waLegacy} {
		p := wasLogin(t, role)
		if _, err := p.Exec(ctx, `SELECT fulfillment.read_pick_list($1::bytea,$2::uuid,NULL::uuid[],$3::uuid,false)`, randomBytes(32), randomUUID(), randomUUID()); sqlState(err) != "42501" {
			t.Errorf("%s ran read_pick_list: %v (want 42501)", role, err)
		}
		if _, err := p.Exec(ctx, `SELECT claims.pick_list_session_orders($1::uuid,$2::uuid,$3::uuid)`, randomUUID(), randomUUID(), randomUUID()); sqlState(err) != "42501" {
			t.Errorf("%s ran pick_list_session_orders: %v (want 42501)", role, err)
		}
		// 0146 (W3-07B): the parcel-group definers are merchant-side only as well.
		for _, call := range []string{
			`SELECT fulfillment.read_merge_suggestions($1::bytea,$2::uuid) WHERE $3::uuid IS NOT NULL`,
			`SELECT fulfillment.create_parcel_group($1::bytea,$2::uuid,'was02-parcel-key',$1::bytea,ARRAY[$3::uuid,$3::uuid])`,
			`SELECT fulfillment.dissolve_parcel_group($1::bytea,$2::uuid,$3::uuid,1)`,
			`SELECT fulfillment.begin_parcel_group_shipment($1::bytea,$2::uuid,$3::uuid)`,
			`SELECT fulfillment.mark_parcel_group_shipped($1::bytea,$2::uuid,$3::uuid)`,
			`SELECT fulfillment.guard_parcel_group_orders($1::bytea,$2::uuid,ARRAY[$3::uuid])`,
			`SELECT fulfillment.read_parcel_group_ids($1::bytea,$2::uuid,ARRAY[$3::uuid])`,
			// 0155 (W3-08B): returns and merchant cancel are merchant-side only as well.
			`SELECT returns.register_rma($1::bytea,$2::uuid,$3::uuid,'was02-rma-key',$1::bytea,'x','[]'::jsonb)`,
			`SELECT returns.receive_rma($1::bytea,$2::uuid,$3::uuid,'was02-rma-key',$1::bytea,1,'[]'::jsonb)`,
			`SELECT returns.inspect_rma($1::bytea,$2::uuid,$3::uuid,'was02-rma-key',$1::bytea,1,'[]'::jsonb)`,
			`SELECT returns.close_rma($1::bytea,$2::uuid,$3::uuid,'was02-rma-key',$1::bytea,1,NULL::uuid)`,
			`SELECT returns.cancel_rma($1::bytea,$2::uuid,$3::uuid,'was02-rma-key',$1::bytea,1)`,
			`SELECT returns.read_order_returns($1::bytea,$2::uuid,$3::uuid)`,
			`SELECT returns.list_returns($1::bytea,$2::uuid,NULL::text) WHERE $3::uuid IS NOT NULL`,
			`SELECT returns.list_cancel_refund_gaps($1::bytea,$2::uuid) WHERE $3::uuid IS NOT NULL`,
			`SELECT fulfillment.merchant_cancel_order($1::bytea,$2::uuid,$3::uuid,'was02-cancel-key',$1::bytea,'CONFIRMED','x')`,
		} {
			if _, err := p.Exec(ctx, call, randomBytes(32), randomUUID(), randomUUID()); sqlState(err) != "42501" {
				t.Errorf("%s ran %s: %v (want 42501)", role, call, err)
			}
		}
	}
}

// WAS03: claim_operation/complete_operation answer 'operation not found' (P0002) to an authority that does not own the lane,
// before touching the row; the owning authority claims it. The empty legacy role and the expiry worker have no EXECUTE at all.
func TestWAS03CrossAuthorityClaimRefused(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	// A MERCHANT stripe operation has no payment attempt, so both payment authorities own it (the profile fence needs an
	// attempt and is proven by WAS05); ads and claims operations belong to exactly one authority.
	lanes := []struct {
		name, provider string
		owners         []string
	}{
		{"payment", "stripe", []string{waPayment, waLive}}, {"ads", "meta_ads", []string{waAds}}, {"claims", "mock", []string{waClaims}},
	}
	owners := []string{waPayment, waLive, waAds, waClaims}
	for _, lane := range lanes {
		t.Run(lane.name, func(t *testing.T) {
			op := wasOperation(t, f, lane.provider, "MERCHANT")
			for _, role := range owners {
				p := wasLogin(t, role)
				token := randomBytes(32)
				var disposition, mode string
				var gen int64
				err := p.QueryRow(ctx, `SELECT * FROM integration.claim_operation($1,30,$2)`, op.id, token).Scan(&disposition, &gen, &mode)
				ownsLane := false
				for _, o := range lane.owners {
					ownsLane = ownsLane || o == role
				}
				if ownsLane {
					if err != nil {
						t.Errorf("%s could not claim its own %s operation: %v", role, lane.name, err)
						continue
					}
					if disposition != "claimed" {
						t.Errorf("%s claim disposition %q", role, disposition)
					}
					if err := t06AuthorityFinish(p, op.id, gen, token, "SUCCEEDED"); err != nil {
						t.Errorf("%s could not complete its own %s operation: %v", role, lane.name, err)
					}
					// Re-arm for the next role: the owner flips the row back to READY (fixture-level, not a worker path).
					if _, err := f.owner.Exec(ctx, `UPDATE integration.operations SET state='READY',generation=0,lease_mode='',lease_until=NULL,lease_token_hash=NULL WHERE id=$1`, op.id); err != nil {
						t.Fatal(err)
					}
					continue
				}
				if sqlState(err) != "P0002" {
					t.Errorf("%s claimed or mis-failed on a %s operation: %v", role, lane.name, err)
				}
				// complete_operation is gated the same way, whatever the (here invalid) lease token.
				if _, cerr := p.Exec(ctx, `SELECT integration.complete_operation($1,1,$2,'SUCCEEDED','observed','')`, op.id, token); sqlState(cerr) != "P0002" {
					t.Errorf("%s completed or mis-failed on a %s operation: %v", role, lane.name, cerr)
				}
			}
			// No refused attempt left a trace: still READY, generation 0, no event.
			var state string
			var gen, events int64
			if err := f.owner.QueryRow(ctx, `SELECT state,generation,(SELECT count(*) FROM integration.operation_events WHERE operation_id=$1 AND reason_code='dispatch_claimed' AND generation>1) FROM integration.operations WHERE id=$1`, op.id).Scan(&state, &gen, &events); err != nil {
				t.Fatal(err)
			}
			if state != "READY" || gen != 0 || events != 0 {
				t.Errorf("refused claims mutated the operation: %s gen=%d events=%d", state, gen, events)
			}
		})
	}
	// The expiry worker and the legacy role cannot call either function at all.
	for _, role := range []string{waExpiry, waLegacy} {
		p := wasLogin(t, role)
		op := wasOperation(t, f, "mock", "MERCHANT")
		if _, err := p.Exec(ctx, `SELECT * FROM integration.claim_operation($1,30,$2)`, op.id, randomBytes(32)); sqlState(err) != "42501" {
			t.Errorf("%s claim: %v, want 42501", role, err)
		}
	}
}

// WAS04: RLS lanes. An authority reads only its own lane's operations, bindings and events.
func TestWAS04OperationReadsAreLaneScoped(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	ops := map[string]wasOp{"payment": wasOperation(t, f, "stripe", "MERCHANT"), "ads": wasOperation(t, f, "meta_ads", "MERCHANT"), "claims": wasOperation(t, f, "mock", "MERCHANT")}
	visible := map[string][]string{waPayment: {"payment"}, waLive: {"payment"}, waAds: {"ads"}, waClaims: {"claims"}}
	for role, want := range visible {
		p := wasLogin(t, role)
		var got []string
		for lane, op := range ops {
			var n, b int
			if err := p.QueryRow(ctx, `SELECT (SELECT count(*) FROM integration.operations WHERE id=$1),(SELECT count(*) FROM integration.bindings WHERE id=(SELECT binding_id FROM integration.operations WHERE id=$1))`, op.id).Scan(&n, &b); err != nil {
				t.Fatalf("%s read %s lane: %v", role, lane, err)
			}
			if n > 0 {
				got = append(got, lane)
			}
			if (n > 0) != (b > 0) && lane == want[0] {
				t.Errorf("%s sees operation=%d binding=%d of its own lane", role, n, b)
			}
		}
		sort.Strings(got)
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("%s sees lanes %v, want %v", role, got, want)
		}
	}
}

// WAS05 is the T21-03 test: the allowed execution profile comes from the login's authority. With bogus ids the profile fence is
// the only thing that can answer 42501 (a profile the authority does not own) versus P0002 (profile ok, operation unknown).
func TestWAS05ProfileFenceDerivedFromAuthority(t *testing.T) {
	ctx := context.Background()
	calls := []string{
		`SELECT * FROM integration.load_stripe_credential($1::uuid,2,$2::bytea,$3::text)`,
		`SELECT integration.finish_stripe_query($1::uuid,2,$2::bytea,$3::text,'stripe_terminal_observed')`,
		`SELECT * FROM integration.load_stripe_refund($1::uuid,2,$2::bytea,$3::text)`,
		`SELECT * FROM integration.load_payment_query($1::uuid,2,$2::bytea,$3::text)`,
	}
	cases := []struct{ role, profile, want string }{
		{waPayment, "LIVE", "42501"}, {waPayment, "SANDBOX", "P0002"}, {waPayment, "PROVIDER_MOCK", "P0002"},
		{waLive, "SANDBOX", "42501"}, {waLive, "PROVIDER_MOCK", "42501"}, {waLive, "LIVE", "P0002"},
	}
	for _, c := range cases {
		p := wasLogin(t, c.role)
		for _, q := range calls {
			_, err := p.Exec(ctx, q, randomUUID(), randomBytes(32), c.profile)
			if got := sqlState(err); got != c.want {
				t.Errorf("%s as %s with profile %s: SQLSTATE %s, want %s (%v)", q[:min(70, len(q))], c.role, c.profile, got, c.want, err)
			}
		}
	}
}

// WAS06 builds the effective privilege inventory of the six worker roles (role, kind, object, privilege) and asserts it is
// not empty (the inventory query itself is part of the gate: a catalog change that breaks it fails here, not silently).
// With LC_WAS_INVENTORY=<file> it also writes the TSV for the before/after evidence of the unit (the release gate never
// accepts a bare SKIP, so the generator no longer skips without the variable).
func TestWAS06PrivilegeInventory(t *testing.T) {
	out := os.Getenv("LC_WAS_INVENTORY")
	f := fixture(t)
	ctx := context.Background()
	rows, err := f.owner.Query(ctx, `
	 WITH r AS (SELECT oid,rolname FROM pg_roles WHERE rolname=ANY($1))
	 SELECT rolname,kind,obj,priv FROM (
	  SELECT r.rolname,'schema' AS kind,n.nspname AS obj,a.privilege_type AS priv FROM pg_namespace n CROSS JOIN LATERAL aclexplode(n.nspacl) a JOIN r ON r.oid=a.grantee
	  UNION ALL SELECT r.rolname,CASE c.relkind WHEN 'S' THEN 'sequence' ELSE 'relation' END,c.oid::regclass::text,a.privilege_type
	    FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace CROSS JOIN LATERAL aclexplode(c.relacl) a JOIN r ON r.oid=a.grantee WHERE n.nspname NOT IN ('pg_catalog','information_schema')
	  UNION ALL SELECT r.rolname,'column',c.oid::regclass::text||'.'||t.attname,a.privilege_type
	    FROM pg_class c JOIN pg_attribute t ON t.attrelid=c.oid CROSS JOIN LATERAL aclexplode(t.attacl) a JOIN r ON r.oid=a.grantee
	  UNION ALL SELECT r.rolname,'function',p.oid::regprocedure::text,a.privilege_type
	    FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace CROSS JOIN LATERAL aclexplode(p.proacl) a JOIN r ON r.oid=a.grantee WHERE n.nspname NOT IN ('pg_catalog','information_schema')
	  UNION ALL SELECT r.rolname,'policy',pol.polrelid::regclass::text||' '||pol.polname,pol.polcmd::text FROM pg_policy pol JOIN r ON r.oid=ANY(pol.polroles)
	  UNION ALL SELECT r.rolname,'membership-of',pg_get_userbyid(m.roleid),'MEMBER' FROM pg_auth_members m JOIN r ON r.oid=m.member
	  UNION ALL SELECT r.rolname,'login-members',pg_get_userbyid(m.member),'MEMBER' FROM pg_auth_members m JOIN r ON r.oid=m.roleid
	 ) x ORDER BY 1,2,3,4`, waAll)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var b strings.Builder
	b.WriteString("role\tkind\tobject\tprivilege\n")
	for rows.Next() {
		var role, kind, obj, priv string
		if err := rows.Scan(&role, &kind, &obj, &priv); err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(obj, "t06_") || strings.HasPrefix(obj, "checkout_test_") {
			continue // test logins of the shared fixture
		}
		fmt.Fprintf(&b, "%s\t%s\t%s\t%s\n", role, kind, obj, priv)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if strings.Count(b.String(), "\n") < 2 {
		t.Fatalf("worker privilege inventory is empty for %v", waAll)
	}
	if out == "" {
		return
	}
	if err := os.WriteFile(out, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
}
