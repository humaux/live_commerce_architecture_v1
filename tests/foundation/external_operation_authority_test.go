package foundation_test

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/platform"
)

// These tests intentionally bypass the Go integration service. Fixed SQL and
// ordinary login grants must enforce leases even if a caller sends raw SQL.
func t06AuthorityLogin(t *testing.T, memberships string) (string, *pgxpool.Pool) {
	t.Helper()
	f := fixture(t)
	name := "t06_" + strings.ReplaceAll(randomUUID(), "-", "")
	password := randomToken()
	_, err := f.owner.Exec(context.Background(), fmt.Sprintf(`CREATE ROLE %s LOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION IN ROLE %s PASSWORD '%s'`, pgx.Identifier{name}.Sanitize(), memberships, password))
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(f.databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	u.User = url.UserPassword(name, password)
	p, err := pgxpool.New(context.Background(), u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		p.Close()
		if _, err := f.owner.Exec(context.Background(), "DROP ROLE "+pgx.Identifier{name}.Sanitize()); err != nil {
			t.Error(err)
		}
	})
	return u.String(), p
}

func t06AuthorityOperation(t *testing.T) (string, string) {
	t.Helper()
	f := fixture(t)
	ctx := context.Background()
	tenant, store, principal, binding, operation := randomUUID(), randomUUID(), randomUUID(), randomUUID(), randomUUID()
	tx, err := f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	for _, statement := range []struct {
		q    string
		args []any
	}{
		{`INSERT INTO control.tenants(id,name) VALUES($1,'T06 synthetic authority')`, []any{tenant}},
		{`INSERT INTO control.stores(tenant_id,id,name,currency) VALUES($1,$2,'T06 synthetic store','TWD')`, []any{tenant, store}},
		{`INSERT INTO identity.principals(id) VALUES($1)`, []any{principal}},
		{`INSERT INTO identity.memberships(tenant_id,principal_id) VALUES($1,$2)`, []any{tenant, principal}},
		{`INSERT INTO integration.bindings(id,tenant_id,store_id,principal_id,provider,external_asset_id) VALUES($1,$2,$3,$4,'mock','synthetic-asset')`, []any{binding, tenant, store, principal}},
		{`INSERT INTO integration.operations(id,tenant_id,store_id,principal_id,binding_id,binding_version,provider,external_asset_id,purpose,action,semantic_key,request_hash,request,job_id) VALUES($1,$2,$3,$4,$5,1,'mock','synthetic-asset','transactional','mock.authority','authority:test',decode(repeat('01',32),'hex'),'{}',1)`, []any{operation, tenant, store, principal, binding}},
	} {
		if _, err = tx.Exec(ctx, statement.q, statement.args...); err != nil {
			t.Fatal(err)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return operation, binding
}

type t06AuthorityClaim struct {
	disposition string
	generation  int64
	mode        string
}

func t06AuthorityTake(t *testing.T, p *pgxpool.Pool, id string, token []byte) t06AuthorityClaim {
	t.Helper()
	var c t06AuthorityClaim
	if err := p.QueryRow(context.Background(), `SELECT * FROM integration.claim_operation($1,30,$2)`, id, token).Scan(&c.disposition, &c.generation, &c.mode); err != nil {
		t.Fatal(err)
	}
	return c
}

func t06AuthorityFinish(p *pgxpool.Pool, id string, gen int64, token []byte, state string) error {
	_, err := p.Exec(context.Background(), `SELECT integration.complete_operation($1,$2,$3,$4,'observed','synthetic-ref')`, id, gen, token, state)
	return err
}

func TestT06WorkerAuthorityAndFunctionACL(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	dsn, p := t06AuthorityLogin(t, "commerce_worker")
	checked, err := platform.OpenWorkerPool(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	checked.Close()
	if unsafe, err := platform.OpenPool(ctx, dsn); err == nil {
		unsafe.Close()
		t.Fatal("worker admitted as merchant")
	}
	for _, roles := range []string{"commerce_worker,commerce_runtime", "commerce_worker,commerce_buyer_runtime", "commerce_worker,commerce_integration_writer"} {
		mixed, _ := t06AuthorityLogin(t, roles)
		if unsafe, err := platform.OpenWorkerPool(ctx, mixed); err == nil {
			unsafe.Close()
			t.Fatalf("mixed worker accepted: %s", roles)
		}
	}
	if unsafe, err := platform.OpenWorkerPool(ctx, f.databaseURL); err == nil {
		unsafe.Close()
		t.Fatal("owner admitted as worker")
	}
	for _, q := range []string{
		`SELECT * FROM identity.sessions`, `SELECT * FROM buyer.capability_sessions`,
		`UPDATE integration.operations SET generation=generation+1 WHERE false`,
		`UPDATE integration.operations SET request='{}' WHERE false`,
		`UPDATE integration.bindings SET enabled=false WHERE false`,
		`DELETE FROM integration.operation_events WHERE false`,
		`INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code) SELECT tenant_id,store_id,operation_id,generation,state,mode,reason_code FROM integration.operation_events WHERE false`,
		`SELECT * FROM river.river_migration`,
		`INSERT INTO river.river_migration SELECT * FROM river.river_migration WHERE false`,
		`UPDATE river.river_migration SET version=version WHERE false`,
		`DELETE FROM river.river_migration WHERE false`,
	} {
		if _, err := p.Exec(ctx, q); sqlState(err) != "42501" {
			t.Fatalf("worker privilege unexpectedly allowed: %s: %v", q, err)
		}
	}
	if _, err := p.Exec(ctx, `UPDATE river.river_job SET state=state WHERE false`); err != nil {
		t.Fatalf("queue lifecycle denied: %v", err)
	}
	if _, err := f.runtime.Exec(ctx, `SELECT * FROM integration.claim_operation($1,30,$2)`, randomUUID(), randomBytes(32)); sqlState(err) != "42501" {
		t.Fatalf("merchant claim allowed: %v", err)
	}
	var functions int
	var safe bool
	err = f.owner.QueryRow(ctx, `SELECT count(*),bool_and(p.prosecdef AND p.proconfig @> ARRAY['search_path=pg_catalog']
	 AND pg_get_userbyid(p.proowner)='commerce_integration_writer'
	 AND has_function_privilege('commerce_worker',p.oid,'EXECUTE')
	 AND NOT has_function_privilege('commerce_runtime',p.oid,'EXECUTE')
	 AND NOT has_function_privilege('commerce_buyer_runtime',p.oid,'EXECUTE')
	 AND NOT has_function_privilege('commerce_buyer_issuer',p.oid,'EXECUTE')
	 AND NOT EXISTS(SELECT 1 FROM aclexplode(coalesce(p.proacl,acldefault('f',p.proowner))) a WHERE a.grantee=0 AND a.privilege_type='EXECUTE'))
	 FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname='integration'`).Scan(&functions, &safe)
	if err != nil || functions != 2 || !safe {
		t.Fatalf("fixed function ACL: count=%d safe=%v err=%v", functions, safe, err)
	}
}

func TestT06SQLLeaseOwnershipExpiryAndNoRedispatch(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	_, p := t06AuthorityLogin(t, "commerce_worker")
	id, _ := t06AuthorityOperation(t)
	a, b := randomBytes(32), randomBytes(32)
	c := t06AuthorityTake(t, p, id, a)
	if c.disposition != "claimed" || c.mode != "dispatch" || c.generation != 1 {
		t.Fatalf("initial claim: %+v", c)
	}
	if busy := t06AuthorityTake(t, p, id, b); busy.disposition != "busy" {
		t.Fatalf("duplicate: %+v", busy)
	}
	if err := t06AuthorityFinish(p, id, 1, b, "SUCCEEDED"); sqlState(err) != "40001" {
		t.Fatalf("wrong owner: %v", err)
	}
	if _, err := f.owner.Exec(ctx, `UPDATE integration.operations SET lease_until=clock_timestamp()-interval '1 second' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if err := t06AuthorityFinish(p, id, 1, a, "SUCCEEDED"); sqlState(err) != "40001" {
		t.Fatalf("expired owner: %v", err)
	}
	c = t06AuthorityTake(t, p, id, b)
	if c.disposition != "claimed" || c.mode != "reconcile" || c.generation != 2 {
		t.Fatalf("expired dispatch was resent: %+v", c)
	}
	if err := t06AuthorityFinish(p, id, 1, a, "SUCCEEDED"); sqlState(err) != "40001" {
		t.Fatalf("stale generation: %v", err)
	}
	if err := t06AuthorityFinish(p, id, 2, b, "ACKNOWLEDGED"); err != nil {
		t.Fatal(err)
	}
	c = t06AuthorityTake(t, p, id, a)
	if c.mode != "reconcile" || c.generation != 3 {
		t.Fatalf("ACK was resent: %+v", c)
	}
	if err := t06AuthorityFinish(p, id, 3, a, "UNKNOWN"); err != nil {
		t.Fatal(err)
	}
	c = t06AuthorityTake(t, p, id, b)
	if c.mode != "reconcile" || c.generation != 4 {
		t.Fatalf("UNKNOWN was resent: %+v", c)
	}
	if err := t06AuthorityFinish(p, id, 4, b, "SUCCEEDED"); err != nil {
		t.Fatal(err)
	}
	if c = t06AuthorityTake(t, p, id, a); c.disposition != "terminal" || c.generation != 4 {
		t.Fatalf("terminal re-executed: %+v", c)
	}
}

func TestT06SQLConcurrentClaimAndBindingOutcomeFacts(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	_, p := t06AuthorityLogin(t, "commerce_worker")
	id, binding := t06AuthorityOperation(t)
	tokens := [][]byte{randomBytes(32), randomBytes(32)}
	results := make([]t06AuthorityClaim, 2)
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i := range 2 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = p.QueryRow(ctx, `SELECT * FROM integration.claim_operation($1,30,$2)`, id, tokens[i]).Scan(&results[i].disposition, &results[i].generation, &results[i].mode)
		}(i)
	}
	wg.Wait()
	winner := -1
	for i, c := range results {
		if errs[i] != nil {
			t.Fatal(errs[i])
		}
		if c.disposition == "claimed" {
			if winner != -1 {
				t.Fatal("two dispatch winners")
			}
			winner = i
		} else if c.disposition != "busy" {
			t.Fatalf("unexpected claim: %+v", c)
		}
	}
	if winner < 0 {
		t.Fatal("no dispatch winner")
	}
	if _, err := f.owner.Exec(ctx, `UPDATE integration.bindings SET enabled=false,semantic_version=2 WHERE id=$1`, binding); err != nil {
		t.Fatal(err)
	}
	if err := t06AuthorityFinish(p, id, 1, tokens[winner], "SUCCEEDED"); err != nil {
		t.Fatal(err)
	}
	var state, reason string
	if err := f.owner.QueryRow(ctx, `SELECT state FROM integration.operations WHERE id=$1`, id).Scan(&state); err != nil || state != "SUCCEEDED" {
		t.Fatalf("remote fact lost: state=%s err=%v", state, err)
	}
	if err := f.owner.QueryRow(ctx, `SELECT reason_code FROM integration.operation_events WHERE operation_id=$1 ORDER BY id DESC LIMIT 1`, id).Scan(&reason); err != nil || reason != "completed_binding_changed" {
		t.Fatalf("binding change not recorded: %s %v", reason, err)
	}
	for _, dispatched := range []bool{false, true} {
		id, binding = t06AuthorityOperation(t)
		token := randomBytes(32)
		if dispatched {
			t06AuthorityTake(t, p, id, token)
			if _, err := f.owner.Exec(ctx, `UPDATE integration.operations SET lease_until=clock_timestamp()-interval '1 second' WHERE id=$1`, id); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := f.owner.Exec(ctx, `UPDATE integration.bindings SET enabled=false,semantic_version=2 WHERE id=$1`, binding); err != nil {
			t.Fatal(err)
		}
		c := t06AuthorityTake(t, p, id, randomBytes(32))
		want := "terminal"
		if dispatched {
			want = "blocked_binding"
		}
		if c.disposition != want || c.mode != "" {
			t.Fatalf("binding fence dispatched=%v: %+v", dispatched, c)
		}
		if dispatched {
			if next := t06AuthorityTake(t, p, id, randomBytes(32)); next.generation != c.generation || next.disposition != "blocked_binding" {
				t.Fatalf("unchanged block churn: %+v", next)
			}
		}
	}
}

func TestT06SQLEventFailureRollsBackClaimAndCompletion(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	_, p := t06AuthorityLogin(t, "commerce_worker")
	id, _ := t06AuthorityOperation(t)
	token := randomBytes(32)
	name := "t06_block_" + strings.ReplaceAll(randomUUID(), "-", "")
	function := pgx.Identifier{"integration", name}.Sanitize()
	trigger := pgx.Identifier{name}.Sanitize()
	if _, err := f.owner.Exec(ctx, fmt.Sprintf(`CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'synthetic event failure'; END $$; CREATE TRIGGER %s BEFORE INSERT ON integration.operation_events FOR EACH ROW WHEN (NEW.operation_id='%s') EXECUTE FUNCTION %s()`, function, trigger, id, function)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := f.owner.Exec(ctx, fmt.Sprintf(`DROP TRIGGER %s ON integration.operation_events; DROP FUNCTION %s()`, trigger, function)); err != nil {
			t.Error(err)
		}
	})
	if _, err := p.Exec(ctx, `SELECT * FROM integration.claim_operation($1,30,$2)`, id, token); sqlState(err) != "P0001" {
		t.Fatalf("fault not exercised: %v", err)
	}
	var state string
	var gen int64
	if err := f.owner.QueryRow(ctx, `SELECT state,generation FROM integration.operations WHERE id=$1`, id).Scan(&state, &gen); err != nil || state != "READY" || gen != 0 {
		t.Fatalf("claim partial write %s/%d: %v", state, gen, err)
	}
	if _, err := f.owner.Exec(ctx, fmt.Sprintf(`ALTER TABLE integration.operation_events DISABLE TRIGGER %s`, trigger)); err != nil {
		t.Fatal(err)
	}
	t06AuthorityTake(t, p, id, token)
	if _, err := f.owner.Exec(ctx, fmt.Sprintf(`ALTER TABLE integration.operation_events ENABLE TRIGGER %s`, trigger)); err != nil {
		t.Fatal(err)
	}
	if err := t06AuthorityFinish(p, id, 1, token, "SUCCEEDED"); sqlState(err) != "P0001" {
		t.Fatalf("completion fault not exercised: %v", err)
	}
	if err := f.owner.QueryRow(ctx, `SELECT state,generation FROM integration.operations WHERE id=$1`, id).Scan(&state, &gen); err != nil || state != "DISPATCHING" || gen != 1 {
		t.Fatalf("completion partial write %s/%d: %v", state, gen, err)
	}
}
