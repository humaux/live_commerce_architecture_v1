package foundation_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"livecommerce/migrations"
)

// Each case keeps the same isolated PG through the failed Apply and its retry.
// A failed post-River transaction may leave 0031 and upstream River committed,
// but must never leave a partly copied application lane or ready predicate.
func TestMetaRuntimeIsolationFailureAndResume(t *testing.T) {
	for _, mode := range []string{"running", "poisoned", "destination", "lock_contention"} {
		t.Run(mode, func(t *testing.T) {
			f := mcPre0029Fixture(t)
			ctx := context.Background()
			mcApplyHistorical(t, f, "0029_meta_social_consumer.sql", "0030_meta_runtime.sql")
			m := mcOldSetup(t, f, "page")
			asset := miAsset()
			binding := miBinding(t, m, asset, "facebook", f.tenantA, f.storeA1, f.principalA)
			miRoute(t, m, asset, f.tenantA, f.storeA1, binding)
			e := mcOldPost(t, m, asset, miMessage(asset, "m."+randomUUID(), "cutover-failure"))
			var badID int64
			switch mode {
			case "running":
				mustExec(t, f.owner, `UPDATE river.river_job SET state='running',attempt=1,attempted_at=clock_timestamp() WHERE id=$1`, e.job)
			case "poisoned":
				mustExec(t, f.owner, `ALTER TABLE river.river_job DISABLE TRIGGER meta_job_family`)
				mustExec(t, f.owner, `ALTER TABLE river.river_job DISABLE TRIGGER meta_job_commit`)
				if err := f.owner.QueryRow(ctx, `INSERT INTO river.river_job(kind,queue,args,max_attempts) VALUES('foreign_cutover_probe','meta_inbox','{}',25) RETURNING id`).Scan(&badID); err != nil {
					t.Fatal(err)
				}
				mustExec(t, f.owner, `ALTER TABLE river.river_job ENABLE TRIGGER meta_job_family`)
				mustExec(t, f.owner, `ALTER TABLE river.river_job ENABLE TRIGGER meta_job_commit`)
			case "destination":
				// First fail safely on a running source so 0031 and the pinned
				// upstream destination are installed, then poison that destination.
				mustExec(t, f.owner, `UPDATE river.river_job SET state='running' WHERE id=$1`, e.job)
				if err := migrations.Apply(ctx, f.owner); err == nil {
					t.Fatal("running source passed first cutover")
				}
				mustExec(t, f.owner, `UPDATE river.river_job SET state='available' WHERE id=$1`, e.job)
				if err := f.owner.QueryRow(ctx, `INSERT INTO river_meta.river_job(kind,queue,args,max_attempts) VALUES('foreign_cutover_probe','default','{}',25) RETURNING id`).Scan(&badID); err != nil {
					t.Fatal(err)
				}
			}
			oldJobs := miIsoRows(t, f, "river.river_job", "")
			oldEvents := miIsoRows(t, f, "meta_inbox.events", "")
			destinationBefore := "[]"
			if mode == "destination" {
				destinationBefore = miIsoRows(t, f, "river_meta.river_job", "")
			}
			var heldTx interface{ Rollback(context.Context) error }
			if mode == "lock_contention" {
				tx, err := f.owner.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				heldTx = tx
				if _, err := tx.Exec(ctx, `LOCK TABLE river.river_job IN ACCESS SHARE MODE`); err != nil {
					t.Fatal(err)
				}
			}
			applyCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
			err := migrations.Apply(applyCtx, f.owner)
			cancel()
			if err == nil {
				t.Fatal("unsafe source passed cutover", mode)
			}
			wantState := "22023"
			if mode == "running" {
				wantState = "55000"
			} else if mode == "lock_contention" {
				wantState = "55P03"
			}
			if state := miSQLState(err); state != wantState {
				t.Fatalf("failed Apply %s state=%s want=%s: %v", mode, state, wantState, err)
			}
			if mode == "lock_contention" {
				if err := heldTx.Rollback(ctx); err != nil {
					t.Fatal(err)
				}
			}
			if got := miIsoRows(t, f, "river.river_job", ""); got != oldJobs {
				t.Fatal("failed cutover mutated old jobs", mode)
			}
			if got := miIsoRows(t, f, "meta_inbox.events", ""); got != oldEvents {
				t.Fatal("failed cutover mutated event receipts", mode)
			}
			if got := miIsoRows(t, f, "river_meta.river_job", ""); got != destinationBefore {
				t.Fatal("failed cutover partly copied destination", mode)
			}
			if miCount(t, f.owner, `SELECT count(*) FROM public.lc_schema_migrations WHERE version='post_river/0004_meta_river_isolation.sql'`) != 0 {
				t.Fatal("failed cutover committed post0004 checksum", mode)
			}
			if miCount(t, f.owner, `SELECT count(*) FROM public.lc_schema_migrations WHERE version='0031_meta_river_isolation.sql'`) != 1 {
				t.Fatal("failed post-River cutover did not retain resumable 0031 preparation", mode)
			}
			mrReady(t, miPool(t, f, "commerce_meta_ingress"), false)
			switch mode {
			case "running":
				mustExec(t, f.owner, `UPDATE river.river_job SET state='available' WHERE id=$1`, e.job)
			case "poisoned":
				mustExec(t, f.owner, `DELETE FROM river.river_job WHERE id=$1`, badID)
			case "destination":
				mustExec(t, f.owner, `DELETE FROM river_meta.river_job WHERE id=$1`, badID)
			}
			if err := migrations.Apply(ctx, f.owner); err != nil {
				t.Fatalf("same-fixture resume %s: %v", mode, err)
			}
			if miCount(t, f.owner, `SELECT count(*) FROM river_meta.river_job WHERE id=$1 AND kind='meta_inbox_v1'`, e.job) != 1 ||
				miCount(t, f.owner, `SELECT count(*) FROM river.river_job WHERE kind='meta_inbox_v1' OR queue='meta_inbox'`) != 0 {
				t.Fatal("resume failed to move exactly the valid job", mode)
			}
			mrReady(t, miPool(t, f, "commerce_meta_worker"), true)
			if _, err := miPool(t, f, "commerce_meta_ingress").Exec(ctx,
				`INSERT INTO river.river_job(kind,queue,args,max_attempts) VALUES('meta_inbox_v1','meta_inbox',$1::jsonb,25)`,
				fmt.Sprintf(`{"event_id":%q,"version":1}`, e.id)); miSQLState(err) != "42501" {
				t.Fatalf("old ingress retained old-lane DML after cutover: %s", miSQLState(err))
			}
		})
	}
}
