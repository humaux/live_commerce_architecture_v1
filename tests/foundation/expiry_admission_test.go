package foundation_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/checkout"
)

// Each deployment-drift probe uses an owned fresh cluster. Normal workers may
// inspect readiness but cannot repair business linkage or choose another queue.
func TestBuyerCheckoutExpiryRuntimeAdmission(t *testing.T) {
	f := pwIsolatedFixture(t)
	p := psSetupItemsOn(t, f, 1)
	assertReady := func(t *testing.T, want bool) {
		t.Helper()
		client, err := checkout.NewExpiryClient(context.Background(), p.worker, 1)
		if want {
			if client == nil || err != nil {
				t.Fatalf("valid expiry router rejected: %v", err)
			}
		} else if client != nil || err == nil || err.Error() != "expiry_worker_queue_unready" {
			t.Fatalf("invalid expiry routing admitted or unsafe error: %v", err)
		}
	}
	assertReady(t, true)
	var triggerDDL string
	if err := f.owner.QueryRow(context.Background(), `SELECT pg_get_triggerdef(oid) FROM pg_trigger
	 WHERE tgrelid='river.river_job'::regclass AND tgname='checkout_expiry_queue_route_v1'`).Scan(&triggerDDL); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"disabled", "missing", "immediate"} {
		t.Run(mode, func(t *testing.T) {
			if mode == "disabled" {
				mustExec(t, f.owner, `ALTER TABLE river.river_job DISABLE TRIGGER checkout_expiry_queue_route_v1`)
				defer mustExec(t, f.owner, `ALTER TABLE river.river_job ENABLE TRIGGER checkout_expiry_queue_route_v1`)
			} else {
				mustExec(t, f.owner, `DROP TRIGGER checkout_expiry_queue_route_v1 ON river.river_job`)
				defer func() {
					mustExec(t, f.owner, `DROP TRIGGER IF EXISTS checkout_expiry_queue_route_v1 ON river.river_job`)
					mustExec(t, f.owner, triggerDDL)
				}()
				if mode == "immediate" {
					mustExec(t, f.owner, `CREATE CONSTRAINT TRIGGER checkout_expiry_queue_route_v1 AFTER INSERT ON river.river_job
					 DEFERRABLE INITIALLY IMMEDIATE FOR EACH ROW EXECUTE FUNCTION checkout.route_expiry_queue_v1()`)
				}
			}
			assertReady(t, false)
		})
	}
	for _, tc := range []struct{ name, change, restore string }{
		{"default", `queue='default'`, `queue='checkout_expiry_v1'`},
		{"custom queue", `queue='ew_unconsumed'`, `queue='checkout_expiry_v1'`},
		{"foreign kind", `kind='ew_foreign'`, `kind='checkout_expiry_v1'`},
		{"wrong generation", `args=jsonb_set(args,'{generation}','2')`, `args=jsonb_set(args,'{generation}','1')`},
		{"decimal generation", `args=jsonb_set(args,'{generation}','1.0')`, `args=jsonb_set(args,'{generation}','1')`},
		{"decimal version", `args=jsonb_set(args,'{version}','1.0')`, `args=jsonb_set(args,'{version}','1')`},
		{"unique key", `unique_key=decode(repeat('ab',32),'hex')`, `unique_key=NULL`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mustExec(t, f.owner, `UPDATE river.river_job SET `+tc.change+` WHERE id=$1`, p.hold.JobID)
			defer mustExec(t, f.owner, `UPDATE river.river_job SET `+tc.restore+` WHERE id=$1`, p.hold.JobID)
			before, queue := pwJobExceptQueue(t, f.owner, p.hold.JobID), pwQueue(t, f.owner, p.hold.JobID)
			assertReady(t, false)
			if before != pwJobExceptQueue(t, f.owner, p.hold.JobID) || queue != pwQueue(t, f.owner, p.hold.JobID) {
				t.Fatal("startup mutated the job")
			}
		})
	}
	assertReady(t, true)
	for _, role := range []string{"commerce_runtime", "commerce_checkout_runtime", "commerce_hosted_runtime", "commerce_buyer_runtime", "commerce_buyer_issuer", "commerce_identity", "commerce_worker"} {
		t.Run("authority "+role, func(t *testing.T) {
			for _, signature := range []string{"checkout.expiry_job_linked(bigint)", "checkout.route_expiry_queue_v1()", "checkout.expiry_queue_ready()"} {
				var allowed bool
				if err := f.owner.QueryRow(context.Background(), `SELECT has_function_privilege($1,$2,'EXECUTE')`, role, signature).Scan(&allowed); err != nil {
					t.Fatal(err)
				}
				if allowed != (role == "commerce_worker" && signature == "checkout.expiry_queue_ready()") {
					t.Fatalf("unexpected EXECUTE %s %s", role, signature)
				}
			}
			if role == "commerce_worker" {
				return
			}
			pool, err := pgxpool.New(context.Background(), bcRole(t, f, role))
			if err != nil {
				t.Fatal(err)
			}
			defer pool.Close()
			client, err := checkout.NewExpiryClient(context.Background(), pool, 1)
			if client != nil || err == nil || err.Error() != "expiry_worker_database" {
				t.Fatal("non-worker authority admitted")
			}
		})
	}
	if client, err := checkout.NewExpiryClient(context.Background(), f.owner, 1); client != nil || err == nil || err.Error() != "expiry_worker_database" {
		t.Fatal("migration owner admitted as worker")
	}
	if n := countRows(t, f.owner, `SELECT count(*) FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
	 JOIN pg_roles r ON r.oid=p.proowner WHERE n.nspname='checkout'
	 AND p.proname IN ('expiry_job_linked','route_expiry_queue_v1','expiry_queue_ready')
	 AND p.prosecdef AND r.rolname='commerce_checkout_writer' AND NOT r.rolcanlogin
	 AND p.proconfig=ARRAY['search_path=pg_catalog']::text[]`); n != 3 {
		t.Fatal("expiry helper owner/search_path/definer drift")
	}
	t.Run("bounded audit", func(t *testing.T) {
		tx, err := f.owner.Begin(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(context.Background())
		if _, err = tx.Exec(context.Background(), `LOCK TABLE river.river_job IN ACCESS EXCLUSIVE MODE`); err != nil {
			t.Fatal(err)
		}
		start := time.Now()
		assertReady(t, false)
		if elapsed := time.Since(start); elapsed < 4*time.Second || elapsed > 7*time.Second {
			t.Fatalf("queue audit did not use bounded 5s context: %s", elapsed)
		}
	})
}

// A valid new durable link is provisioned inside each owned transaction, so
// failure cannot be explained merely by an orphan. Rollback restores the order.
func TestBuyerCheckoutExpiryRuntimeDeferredAdmission(t *testing.T) {
	f := pwIsolatedFixture(t)
	p := psSetupItemsOn(t, f, 1)
	original := pwJobExceptQueue(t, f.owner, p.hold.JobID)
	for _, name := range []string{"orphan", "wrong generation", "decimal generation", "decimal version", "extra args", "custom queue", "unique key", "deleted", "stale NEW args", "stale NEW kind"} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			tx, err := f.owner.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			var id int64
			err = tx.QueryRow(ctx, `INSERT INTO river.river_job(kind,args,max_attempts,queue)
			 SELECT CASE $2 WHEN 'stale NEW kind' THEN 'ew_unrelated' ELSE kind END,
			 CASE $2 WHEN 'wrong generation' THEN jsonb_set(args,'{generation}','2')
			 WHEN 'decimal generation' THEN jsonb_set(args,'{generation}','1.0')
			 WHEN 'decimal version' THEN jsonb_set(args,'{version}','1.0')
			 WHEN 'extra args' THEN args||'{"extra":true}'::jsonb
			 WHEN 'stale NEW args' THEN args||'{"extra":true}'::jsonb ELSE args END,
			 max_attempts,'default' FROM river.river_job WHERE id=$1 RETURNING id`, p.hold.JobID, name).Scan(&id)
			if err != nil {
				t.Fatal("insert must defer validation to commit", err)
			}
			if name != "orphan" {
				if _, err = tx.Exec(ctx, `UPDATE checkout.orders SET job_id=$1 WHERE id=$2`, id, p.hold.OrderID); err != nil {
					t.Fatal(err)
				}
			}
			changes := map[string]string{
				"custom queue":   `UPDATE river.river_job SET queue='ew_custom' WHERE id=$1`,
				"unique key":     `UPDATE river.river_job SET unique_key=decode(repeat('ab',32),'hex') WHERE id=$1`,
				"deleted":        `DELETE FROM river.river_job WHERE id=$1`,
				"stale NEW args": `UPDATE river.river_job SET args=args-'extra' WHERE id=$1`,
				"stale NEW kind": `UPDATE river.river_job SET kind='checkout_expiry_v1' WHERE id=$1`,
			}
			if sql, ok := changes[name]; ok {
				if _, err = tx.Exec(ctx, sql, id); err != nil {
					t.Fatal(err)
				}
			}
			if name == "stale NEW args" || name == "stale NEW kind" {
				var valid bool
				if err = tx.QueryRow(ctx, `SELECT checkout.expiry_job_linked($1)`, id).Scan(&valid); err != nil || !valid {
					t.Fatal("stale NEW case did not have valid final domain linkage")
				}
			}
			var pgErr *pgconn.PgError
			if err = tx.Commit(ctx); !errors.As(err, &pgErr) || pgErr.Code != "22023" {
				t.Fatalf("expected deferred admission 22023, got %v", err)
			}
			var linked int64
			if err = f.owner.QueryRow(ctx, `SELECT job_id FROM checkout.orders WHERE id=$1`, p.hold.OrderID).Scan(&linked); err != nil || linked != p.hold.JobID {
				t.Fatal("failed commit changed durable order linkage")
			}
			if countRows(t, f.owner, `SELECT count(*) FROM river.river_job WHERE id=$1`, id) != 0 {
				t.Fatal("failed commit left inserted job")
			}
		})
	}
	if original != pwJobExceptQueue(t, f.owner, p.hold.JobID) || pwQueue(t, f.owner, p.hold.JobID) != "checkout_expiry_v1" {
		t.Fatal("invalid inserts changed original job")
	}
}
