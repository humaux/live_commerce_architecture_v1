package foundation_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func pwQueue(t *testing.T, pool *pgxpool.Pool, id int64) string {
	return pwQueueIn(t, pool, "river.river_job", id)
}

// Job IDs are independent sequences in the payment, expiry and legacy schemas.
func pwQueueIn(t *testing.T, pool *pgxpool.Pool, table string, id int64) string {
	t.Helper()
	var queue string
	if err := pool.QueryRow(context.Background(), `SELECT queue FROM `+table+` WHERE id=$1`, id).Scan(&queue); err != nil {
		t.Fatal(err)
	}
	return queue
}

func pwJobExceptQueue(t *testing.T, pool *pgxpool.Pool, id int64) string {
	return pwJobExceptQueueIn(t, pool, "river.river_job", id)
}

func pwJobExceptQueueIn(t *testing.T, pool *pgxpool.Pool, table string, id int64) string {
	t.Helper()
	var row string
	if err := pool.QueryRow(context.Background(), `SELECT (to_jsonb(j)-'queue')::text FROM `+table+` j WHERE id=$1`, id).Scan(&row); err != nil {
		t.Fatal(err)
	}
	return row
}

// This runs only in pwIsolatedFixture's test-created database. It models the
// pre-router version without mutating the shared suite or any outside database.
func pwRemoveRouter(t *testing.T, owner *pgxpool.Pool) {
	t.Helper()
	mustExec(t, owner, `DROP TRIGGER payment_queue_route_v1 ON river.river_job`)
	mustExec(t, owner, `DROP FUNCTION integration.payment_queue_ready()`)
	mustExec(t, owner, `DROP FUNCTION integration.route_payment_queue_v1()`)
	mustExec(t, owner, `DROP FUNCTION integration.payment_job_queue(bigint)`)
	mustExec(t, owner, `DELETE FROM public.lc_schema_migrations WHERE version='post_river/0001_payment_queue.sql'`)
}

func pwPostMigrationAbsent(t *testing.T, owner *pgxpool.Pool) {
	t.Helper()
	var ledger, trigger int
	if err := owner.QueryRow(context.Background(), `SELECT
	 (SELECT count(*) FROM public.lc_schema_migrations WHERE version='post_river/0001_payment_queue.sql'),
	 (SELECT count(*) FROM pg_trigger WHERE tgrelid='river.river_job'::regclass AND tgname='payment_queue_route_v1')`).Scan(&ledger, &trigger); err != nil {
		t.Fatal(err)
	}
	if ledger != 0 || trigger != 0 {
		t.Fatalf("failed upgrade partially installed router: ledger=%d trigger=%d", ledger, trigger)
	}
}

func TestBuyerPaymentWorkerQueueMigrationRollbackAndBackfill(t *testing.T) {
	f := lriPre0032Fixture(t)
	q := pqSetupItemsOn(t, f, nil, false, 1, "river")
	if got := pwQueue(t, f.owner, q.result.JobID); got != "payment_mock_v1" {
		t.Fatalf("fresh migration did not route producer: %s", got)
	}
	pwRemoveRouter(t, f.owner)
	mustExec(t, f.owner, `UPDATE river.river_job SET queue='default' WHERE id=$1`, q.result.JobID)
	terminal := pwOldQuerySetupOn(t, f, q.keys, "river")
	mustExec(t, f.owner, `UPDATE river.river_job SET state='completed',finalized_at=clock_timestamp() WHERE id=$1`, terminal.result.JobID)
	terminalBefore := pwJobExceptQueue(t, f.owner, terminal.result.JobID)
	if err := q.record(q.claim(t), pcFull(q)); err != nil {
		t.Fatal(err)
	}
	var reconcile int64
	var reconcileArgs string
	if err := f.owner.QueryRow(context.Background(), `SELECT id,args::text FROM river.river_job WHERE kind='payment_reconcile_v1' AND args->>'operation_id'=$1`, q.result.OperationID).Scan(&reconcile, &reconcileArgs); err != nil {
		t.Fatal(err)
	}
	var unrelated int64
	if err := f.owner.QueryRow(context.Background(), `INSERT INTO river.river_job(kind,args,max_attempts,queue) VALUES('pw_unrelated_v1','{}',2,'default') RETURNING id`).Scan(&unrelated); err != nil {
		t.Fatal(err)
	}
	queryArgs := fmt.Sprintf(`{"operation_id":"%s","version":1}`, q.result.AttemptID)
	for _, tc := range []struct {
		name, mutate, restore string
		id                    int64
	}{
		{"running", `UPDATE river.river_job SET state='running' WHERE id=$1`, `UPDATE river.river_job SET state='scheduled' WHERE id=$1`, q.result.JobID},
		{"orphan", `UPDATE river.river_job SET args=jsonb_build_object('operation_id',gen_random_uuid()::text,'version',1) WHERE id=$1`, `UPDATE river.river_job SET args=$2::jsonb WHERE id=$1`, q.result.JobID},
		{"wrong_hash", `UPDATE river.river_job SET args=jsonb_set(args,'{report_hash}',to_jsonb(repeat('00',32))) WHERE id=$1`, `UPDATE river.river_job SET args=$2::jsonb WHERE id=$1`, reconcile},
		{"unique_key", `UPDATE river.river_job SET unique_key=decode(repeat('ab',32),'hex') WHERE id=$1`, `UPDATE river.river_job SET unique_key=NULL WHERE id=$1`, q.result.JobID},
		{"wrong_queue", `UPDATE river.river_job SET queue='not_payment' WHERE id=$1`, `UPDATE river.river_job SET queue='default' WHERE id=$1`, q.result.JobID},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mustExec(t, f.owner, tc.mutate, tc.id)
			before := pwJobExceptQueue(t, f.owner, tc.id)
			if err := lriApplyHistoricalPost(f, "post_river/0001_payment_queue.sql"); err == nil {
				t.Fatal("invalid legacy job was migrated")
			}
			pwPostMigrationAbsent(t, f.owner)
			if after := pwJobExceptQueue(t, f.owner, tc.id); after != before || pwQueue(t, f.owner, tc.id) != map[bool]string{true: "not_payment", false: "default"}[tc.name == "wrong_queue"] {
				t.Fatal("failed migration changed legacy job")
			}
			if tc.name == "orphan" {
				mustExec(t, f.owner, tc.restore, tc.id, queryArgs)
			} else if tc.name == "wrong_hash" {
				mustExec(t, f.owner, tc.restore, tc.id, reconcileArgs)
			} else {
				mustExec(t, f.owner, tc.restore, tc.id)
			}
		})
	}
	var foreign int64
	if err := f.owner.QueryRow(context.Background(), `INSERT INTO river.river_job(kind,args,max_attempts,queue) VALUES('pw_unrelated_v1','{}',2,'payment_mock_v1') RETURNING id`).Scan(&foreign); err != nil {
		t.Fatal(err)
	}
	if err := lriApplyHistoricalPost(f, "post_river/0001_payment_queue.sql"); err == nil {
		t.Fatal("foreign reserved-queue kind passed upgrade")
	}
	pwPostMigrationAbsent(t, f.owner)
	mustExec(t, f.owner, `DELETE FROM river.river_job WHERE id=$1`, foreign)
	queryBefore := pwJobExceptQueue(t, f.owner, q.result.JobID)
	reconcileBefore := pwJobExceptQueue(t, f.owner, reconcile)
	unrelatedBefore := pwJobExceptQueue(t, f.owner, unrelated)
	if err := lriApplyHistoricalPost(f, "post_river/0001_payment_queue.sql"); err != nil {
		t.Fatalf("valid upgrade: %v", err)
	}
	if got := pwQueue(t, f.owner, q.result.JobID); got != "payment_mock_v1" {
		t.Fatalf("valid legacy job stayed on %s", got)
	}
	if pwJobExceptQueue(t, f.owner, q.result.JobID) != queryBefore || pwJobExceptQueue(t, f.owner, reconcile) != reconcileBefore || pwQueue(t, f.owner, reconcile) != "payment_mock_v1" || pwJobExceptQueue(t, f.owner, terminal.result.JobID) != terminalBefore || pwQueue(t, f.owner, terminal.result.JobID) != "default" || pwJobExceptQueue(t, f.owner, unrelated) != unrelatedBefore || pwQueue(t, f.owner, unrelated) != "default" {
		t.Fatal("backfill changed non-queue fields or unrelated default job")
	}
	if err := lriApplyHistoricalPost(f, "post_river/0001_payment_queue.sql"); err != nil {
		t.Fatalf("repeat post-River migration: %v", err)
	}
	mustExec(t, f.owner, `INSERT INTO public.lc_schema_migrations(version,checksum) VALUES('post_river/9999_unknown.sql','test-only')`)
	if err := lriApplyHistoricalPost(f, "post_river/0001_payment_queue.sql"); err == nil {
		t.Fatal("unknown post-River version was accepted")
	}
	mustExec(t, f.owner, `DELETE FROM public.lc_schema_migrations WHERE version='post_river/9999_unknown.sql'`)
	var checksum string
	if err := f.owner.QueryRow(context.Background(), `SELECT checksum FROM public.lc_schema_migrations WHERE version='post_river/0001_payment_queue.sql'`).Scan(&checksum); err != nil {
		t.Fatal(err)
	}
	mustExec(t, f.owner, `UPDATE public.lc_schema_migrations SET checksum='test-invalid' WHERE version='post_river/0001_payment_queue.sql'`)
	if err := lriApplyHistoricalPost(f, "post_river/0001_payment_queue.sql"); err == nil {
		t.Fatal("changed post-River checksum was accepted")
	}
	// Recover the original checksum from the accepted ledger's pre-mutation
	// snapshot, rather than editing any migration source or accepting drift.
	mustExec(t, f.owner, `UPDATE public.lc_schema_migrations SET checksum=$1 WHERE version='post_river/0001_payment_queue.sql'`, checksum)
	var count int
	if err := f.owner.QueryRow(context.Background(), `SELECT count(*) FROM public.lc_schema_migrations WHERE version='post_river/0001_payment_queue.sql'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("post migration ledger count=%d err=%v", count, err)
	}
	// A pre-upgrade River producer requests default before the attempt exists;
	// the deferred trigger must see the final linkage at commit.
	late := pwOldQuerySetupOn(t, f, q.keys, "river")
	if pwQueue(t, f.owner, late.result.JobID) != "payment_mock_v1" {
		t.Fatal("old producer was not routed at commit")
	}
	if _, err := f.owner.Exec(context.Background(), `INSERT INTO river.river_job(kind,args,max_attempts,queue) VALUES('pw_unrelated_v1','{}',2,'payment_mock_v1')`); err == nil {
		t.Fatal("foreign kind entered reserved payment queue")
	}
	if pwQueue(t, f.owner, unrelated) != "default" {
		t.Fatal("ordinary River job was changed")
	}
}
