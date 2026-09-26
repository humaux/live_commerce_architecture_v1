package foundation_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

// Admission audits detect deployment drift; this separate gate proves bad new
// inserts cannot commit in the first place. All mutations use an isolated owner
// fixture, including the stronger final-valid-row / stale-NEW regression.
func TestBuyerPaymentWorkerDeferredRoutingRollback(t *testing.T) {
	f := pwIsolatedFixture(t)
	q := pqSetupItemsOn(t, f, nil, false, 1)
	if err := q.record(q.claim(t), pcFull(q)); err != nil {
		t.Fatal(err)
	}
	var original int64
	if err := f.owner.QueryRow(context.Background(), `SELECT id FROM river_payment.river_job
	 WHERE kind='payment_reconcile_v1' AND args->>'operation_id'=$1`, q.result.OperationID).Scan(&original); err != nil {
		t.Fatal(err)
	}
	before := pwJobExceptQueueIn(t, f.owner, "river_payment.river_job", original)
	for _, tc := range []struct{ name, change, rejectAt string }{
		{"wrong version", `UPDATE river_payment.river_job SET args=args WHERE id=$1`, "insert"},
		{"wrong hash", `UPDATE river_payment.river_job SET args=args WHERE id=$1`, "commit"},
		{"wrong profile queue", `UPDATE river_payment.river_job SET queue='payment_live_v1' WHERE id=$1`, "update"},
		{"unique key", `UPDATE river_payment.river_job SET unique_key=decode(repeat('ab',32),'hex') WHERE id=$1`, "update"},
		{"deleted before commit", `DELETE FROM river_payment.river_job WHERE id=$1`, "commit"},
		{"final valid but changed args", `UPDATE river_payment.river_job SET args=args-'extra' WHERE id=$1`, "commit"},
		{"final valid but changed kind", `UPDATE river_payment.river_job SET kind='payment_reconcile_v1' WHERE id=$1`, "commit"},
		{"query with wrong job ID", `UPDATE river_payment.river_job SET args=args WHERE id=$1`, "commit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			staleNEW := tc.name == "final valid but changed args" || tc.name == "final valid but changed kind"
			if staleNEW {
				// Keep the original deferred captured-NEW gate: owner-only setup
				// bypasses the newer BEFORE guard for this disposable transaction.
				mustExec(t, f.owner, `ALTER TABLE river_payment.river_job DISABLE TRIGGER payment_job_family`)
				defer mustExec(t, f.owner, `ALTER TABLE river_payment.river_job ENABLE TRIGGER payment_job_family`)
			}
			tx, err := f.owner.Begin(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			var id int64
			// A valid QUERY observation permits a reconcile job; vary NEW itself
			// for two cases whose final row would otherwise have valid linkage.
			err = tx.QueryRow(context.Background(), `INSERT INTO river_payment.river_job(kind,args,max_attempts,queue)
			 SELECT CASE $2 WHEN 'final valid but changed kind' THEN 'pw_unrelated_v1'
			 WHEN 'query with wrong job ID' THEN 'payment_query_v1' ELSE kind END,
			 CASE $2 WHEN 'final valid but changed args' THEN args || '{"extra":true}'::jsonb
			 WHEN 'wrong version' THEN jsonb_set(args,'{version}','2')
			 WHEN 'wrong hash' THEN jsonb_set(args,'{report_hash}',to_jsonb(repeat('00',32)))
			 WHEN 'query with wrong job ID' THEN args-'report_hash' ELSE args END,
			 max_attempts,'default' FROM river_payment.river_job WHERE id=$1 RETURNING id`, original, tc.name).Scan(&id)
			var pgErr *pgconn.PgError
			if tc.rejectAt == "insert" {
				if !errors.As(err, &pgErr) || pgErr.Code != "22023" {
					t.Fatalf("family guard must reject INSERT 22023: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal("valid INSERT failed before deferred check", err)
			}
			if _, err = tx.Exec(context.Background(), tc.change, id); err != nil {
				if tc.rejectAt != "update" || !errors.As(err, &pgErr) || pgErr.Code != "22023" {
					t.Fatalf("unexpected UPDATE rejection: %v", err)
				}
				return
			}
			if tc.rejectAt == "update" {
				t.Fatal("family guard admitted invalid UPDATE")
			}
			if staleNEW {
				var expected string
				if err := tx.QueryRow(context.Background(), `SELECT integration.payment_job_queue($1)`, id).Scan(&expected); err != nil || expected != "payment_mock_v1" {
					t.Fatalf("stale NEW case lacks valid final linkage: %s %v", expected, err)
				}
			}
			if err = tx.Commit(context.Background()); !errors.As(err, &pgErr) || pgErr.Code != "22023" {
				t.Fatalf("invalid deferred job must fail linkage at commit: %v", err)
			}
			var count int
			if err := f.owner.QueryRow(context.Background(), `SELECT count(*) FROM river_payment.river_job WHERE id=$1`, id).Scan(&count); err != nil || count != 0 {
				t.Fatal("failed commit left a job behind")
			}
		})
	}
	if pwJobExceptQueueIn(t, f.owner, "river_payment.river_job", original) != before || pwQueueIn(t, f.owner, "river_payment.river_job", original) != "payment_mock_v1" {
		t.Fatal("failed inserts changed the original linked job")
	}
}
