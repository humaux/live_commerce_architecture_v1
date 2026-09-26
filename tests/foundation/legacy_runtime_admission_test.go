package foundation_test

import (
	"context"
	"fmt"
	"testing"

	"livecommerce/internal/checkout"
	"livecommerce/internal/payments"
)

func TestLegacyRuntimeIsolationAdmissionAndReadiness(t *testing.T) {
	f := pwIsolatedFixture(t)
	ctx := context.Background()
	ready := func() (bool, bool) {
		t.Helper()
		var payment, expiry bool
		if err := f.owner.QueryRow(ctx, `SELECT integration.payment_queue_ready(),checkout.expiry_queue_ready()`).Scan(&payment, &expiry); err != nil {
			t.Fatal(err)
		}
		return payment, expiry
	}
	if payment, expiry := ready(); !payment || !expiry {
		t.Fatal("empty but correctly guarded family schemas are not ready")
	}
	for _, tc := range []struct {
		name, table, trigger    string
		wantPayment, wantExpiry bool
	}{
		{"payment family", "river_payment.river_job", "payment_job_family", false, true},
		{"payment deferred", "river_payment.river_job", "payment_queue_route_v1", false, true},
		{"expiry family", "river_expiry.river_job", "expiry_job_family", true, false},
		{"expiry deferred", "river_expiry.river_job", "checkout_expiry_queue_route_v1", true, false},
		{"old lane", "river.river_job", "legacy_family_exclusion", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mustExec(t, f.owner, `ALTER TABLE `+tc.table+` DISABLE TRIGGER `+tc.trigger)
			defer mustExec(t, f.owner, `ALTER TABLE `+tc.table+` ENABLE TRIGGER `+tc.trigger)
			if payment, expiry := ready(); payment != tc.wantPayment || expiry != tc.wantExpiry {
				t.Fatalf("disabled %s guard left runtime ready payment=%v expiry=%v", tc.name, payment, expiry)
			}
		})
	}
	for _, tc := range []struct {
		table, trigger          string
		wantPayment, wantExpiry bool
	}{
		{"river_payment.river_job", "payment_job_family", false, true},
		{"river_expiry.river_job", "expiry_job_family", true, false},
		{"river.river_job", "legacy_family_exclusion", false, false},
	} {
		t.Run("missing "+tc.trigger, func(t *testing.T) {
			var ddl string
			if err := f.owner.QueryRow(ctx, `SELECT pg_get_triggerdef(oid) FROM pg_trigger WHERE tgrelid=$1::regclass AND tgname=$2`, tc.table, tc.trigger).Scan(&ddl); err != nil {
				t.Fatal(err)
			}
			mustExec(t, f.owner, `DROP TRIGGER `+tc.trigger+` ON `+tc.table)
			defer mustExec(t, f.owner, ddl)
			if payment, expiry := ready(); payment != tc.wantPayment || expiry != tc.wantExpiry {
				t.Fatalf("missing %s guard left wrong readiness payment=%v expiry=%v", tc.trigger, payment, expiry)
			}
		})
	}
	for _, tc := range []struct {
		name, function, owner   string
		wantPayment, wantExpiry bool
	}{
		{"payment owner", "integration.guard_payment_job_family()", "commerce_integration_writer", false, true},
		{"expiry owner", "checkout.guard_expiry_job_family()", "commerce_checkout_writer", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mustExec(t, f.owner, `ALTER FUNCTION `+tc.function+` OWNER TO postgres`)
			defer mustExec(t, f.owner, `ALTER FUNCTION `+tc.function+` OWNER TO `+tc.owner)
			if payment, expiry := ready(); payment != tc.wantPayment || expiry != tc.wantExpiry {
				t.Fatalf("wrong %s left runtime ready", tc.name)
			}
		})
		t.Run(tc.name+" search path", func(t *testing.T) {
			mustExec(t, f.owner, `ALTER FUNCTION `+tc.function+` SET search_path=public`)
			defer mustExec(t, f.owner, `ALTER FUNCTION `+tc.function+` SET search_path=pg_catalog`)
			if payment, expiry := ready(); payment != tc.wantPayment || expiry != tc.wantExpiry {
				t.Fatalf("unsafe %s search_path left runtime ready", tc.name)
			}
		})
	}
	if payment, expiry := ready(); !payment || !expiry {
		t.Fatal("restored guards did not restore readiness")
	}
	p := psSetupItemsOn(t, f, 1)
	before := p.facts(t)
	beforePaymentJobs := miIsoRows(t, f, "river_payment.river_job", "")
	beforeExpiryJobs := miIsoRows(t, f, "river_expiry.river_job", "")
	badPayment := psStarterIn(t, p.pool, "PROVIDER_MOCK", "river_expiry")
	p.starter = badPayment
	if _, err := p.start(t04Key("wrong-payment-schema")); err == nil || err.Error() != "checkout database unavailable" {
		t.Fatalf("wrong payment River schema leaked error or succeeded: %v", err)
	}
	if got := p.facts(t); got != before || miIsoRows(t, f, "river_payment.river_job", "") != beforePaymentJobs || miIsoRows(t, f, "river_expiry.river_job", "") != beforeExpiryJobs {
		t.Fatal("wrong payment schema wrote business facts or either family job lane")
	}
	badExpiry := bcSetup(t)
	expiryBefore := badExpiry.facts(t)
	beforePaymentJobs = miIsoRows(t, badExpiry.f, "river_payment.river_job", "")
	beforeExpiryJobs = miIsoRows(t, badExpiry.f, "river_expiry.river_job", "")
	badExpiry.service = bcServiceIn(t, badExpiry.pool, "river_payment")
	if _, err := badExpiry.begin(t04Key("wrong-expiry-schema")); err == nil || err.Error() != "checkout database unavailable" {
		t.Fatalf("wrong expiry River schema leaked error or succeeded: %v", err)
	}
	if got := badExpiry.facts(t); got != expiryBefore || miIsoRows(t, badExpiry.f, "river_payment.river_job", "") != beforePaymentJobs || miIsoRows(t, badExpiry.f, "river_expiry.river_job", "") != beforeExpiryJobs {
		t.Fatal("wrong expiry schema wrote business facts or either family job lane")
	}
	p.starter = psStarter(t, p.pool, "PROVIDER_MOCK")
	goodPayment, err := p.start(t04Key("correct-payment-schema"))
	if err != nil || goodPayment.JobID < 1 {
		t.Fatalf("correct payment producer not admitted: %v", err)
	}
	rowBefore := miIsoRows(t, f, "river_payment.river_job", "")
	domainBefore := miIsoRows(t, f, "checkout.payment_attempts", "")
	for _, tc := range []struct{ name, statement string }{
		{"old payment kind", `INSERT INTO river.river_job(kind,args,queue,max_attempts) VALUES('payment_query_v1','{}','default',25)`},
		{"old expiry queue", `INSERT INTO river.river_job(kind,args,queue,max_attempts) VALUES('external_operation_v1','{}','checkout_expiry_v1',25)`},
		{"payment foreign kind", `INSERT INTO river_payment.river_job(kind,args,queue,max_attempts) VALUES('external_operation_v1','{}','payment_mock_v1',25)`},
		{"expiry foreign kind", `INSERT INTO river_expiry.river_job(kind,args,queue,max_attempts) VALUES('payment_query_v1','{}','checkout_expiry_v1',25)`},
		{"payment malformed", `INSERT INTO river_payment.river_job(kind,args,queue,max_attempts) VALUES('payment_query_v1','{}','payment_mock_v1',25)`},
		{"expiry malformed", `INSERT INTO river_expiry.river_job(kind,args,queue,max_attempts) VALUES('checkout_expiry_v1','{}','checkout_expiry_v1',25)`},
		{"payment wrong queue", `INSERT INTO river_payment.river_job(kind,args,queue,max_attempts) SELECT kind,args,'checkout_expiry_v1',max_attempts FROM river_payment.river_job WHERE id=` + fmt.Sprint(goodPayment.JobID)},
		{"expiry wrong queue", `INSERT INTO river_expiry.river_job(kind,args,queue,max_attempts) SELECT kind,args,'payment_mock_v1',max_attempts FROM river_expiry.river_job WHERE id=` + fmt.Sprint(p.hold.JobID)},
		{"payment orphan", `INSERT INTO river_payment.river_job(kind,args,queue,max_attempts) SELECT kind,args,queue,max_attempts FROM river_payment.river_job WHERE id=` + fmt.Sprint(goodPayment.JobID)},
		{"expiry orphan", `INSERT INTO river_expiry.river_job(kind,args,queue,max_attempts) SELECT kind,args,queue,max_attempts FROM river_expiry.river_job WHERE id=` + fmt.Sprint(p.hold.JobID)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := f.owner.Exec(ctx, tc.statement); sqlState(err) != "22023" {
				t.Fatalf("invalid family insertion SQLSTATE=%s err=%v", sqlState(err), err)
			}
		})
	}
	for _, tc := range []struct {
		table string
		id    int64
	}{
		{"river_payment.river_job", goodPayment.JobID},
		{"river_expiry.river_job", p.hold.JobID},
	} {
		if _, err := f.owner.Exec(ctx, `UPDATE `+tc.table+` SET args=args||'{"extra":true}'::jsonb WHERE id=$1`, tc.id); sqlState(err) != "22023" {
			t.Fatalf("job identity rewrite SQLSTATE=%s err=%v", sqlState(err), err)
		}
	}
	if miIsoRows(t, f, "river_payment.river_job", "") != rowBefore || miIsoRows(t, f, "checkout.payment_attempts", "") != domainBefore {
		t.Fatal("rejected family mutations changed a valid job or payment attempt")
	}
	// New native lanes remain inaccessible to Meta's distinct lifecycle roles;
	// checkout producers also lose the old River lane after cutover.
	expiryRowsBefore := miIsoRows(t, f, "river_expiry.river_job", "")
	oldBefore := miIsoRows(t, f, "river.river_job", "")
	for _, role := range []string{"commerce_meta_worker", "commerce_meta_consumer", "commerce_meta_ingress"} {
		t.Run("meta ACL "+role, func(t *testing.T) {
			pool := miPool(t, f, role)
			defer pool.Close()
			for _, schema := range []string{"river_payment", "river_expiry"} {
				for _, statement := range []string{
					`SELECT count(*) FROM ` + schema + `.river_job`,
					`INSERT INTO ` + schema + `.river_job(kind,args,queue,max_attempts) VALUES('foreign_probe','{}','default',1)`,
					`UPDATE ` + schema + `.river_job SET queue=queue WHERE id=0`,
					`SELECT nextval('` + schema + `.river_job_id_seq')`,
					`CREATE TABLE ` + schema + `.acl_probe(id bigint)`,
				} {
					if _, err := pool.Exec(ctx, statement); sqlState(err) != "42501" {
						t.Fatalf("%s accessed %s: SQLSTATE=%s err=%v", role, schema, sqlState(err), err)
					}
				}
			}
		})
	}
	t.Run("checkout producer old River ACL", func(t *testing.T) {
		pool := miPool(t, f, "commerce_checkout_runtime")
		defer pool.Close()
		if _, err := pool.Exec(ctx, `INSERT INTO river.river_job(kind,args,queue,max_attempts) VALUES('checkout_expiry_v1','{}','default',1)`); sqlState(err) != "42501" {
			t.Fatalf("checkout producer retained old River admission: SQLSTATE=%s err=%v", sqlState(err), err)
		}
	})
	if miIsoRows(t, f, "river_payment.river_job", "") != rowBefore || miIsoRows(t, f, "river_expiry.river_job", "") != expiryRowsBefore || miIsoRows(t, f, "river.river_job", "") != oldBefore {
		t.Fatal("rejected cross-family ACL attempts changed a job lane")
	}
	// A test owner can bypass a trigger temporarily; readiness must still reject
	// the resulting active poison after every guard is restored.
	mustExec(t, f.owner, `ALTER TABLE river_payment.river_job DISABLE TRIGGER payment_job_family`)
	mustExec(t, f.owner, `ALTER TABLE river_payment.river_job DISABLE TRIGGER payment_queue_route_v1`)
	var poisoned int64
	if err := f.owner.QueryRow(ctx, `INSERT INTO river_payment.river_job(kind,args,queue,max_attempts) VALUES('foreign_family_probe','{}','payment_mock_v1',25) RETURNING id`).Scan(&poisoned); err != nil {
		t.Fatal(err)
	}
	mustExec(t, f.owner, `ALTER TABLE river_payment.river_job ENABLE TRIGGER payment_job_family`)
	mustExec(t, f.owner, `ALTER TABLE river_payment.river_job ENABLE TRIGGER payment_queue_route_v1`)
	if payment, expiry := ready(); payment || !expiry {
		t.Fatal("active payment poison did not fail closed")
	}
	opts := payments.DefaultQueryWorkerOptions()
	opts.MockTransport = pqNoNetwork()
	if client, err := payments.NewWorkerClient(ctx, p.worker, pwKeys(t), "PROVIDER_MOCK", 1, opts); client != nil || err == nil || err.Error() != "payment_worker_queue_unready" {
		t.Fatalf("poisoned payment startup accepted: %v", err)
	}
	mustExec(t, f.owner, `DELETE FROM river_payment.river_job WHERE id=$1`, poisoned)
	if payment, expiry := ready(); !payment || !expiry {
		t.Fatal("rejected writes degraded readiness")
	}
	if client, err := checkout.NewExpiryClient(ctx, p.worker, 1); client == nil || err != nil {
		t.Fatalf("valid expiry constructor rejected: %v", err)
	}
	if client, err := payments.NewWorkerClient(ctx, p.worker, pwKeys(t), "PROVIDER_MOCK", 1, opts); client == nil || err != nil {
		t.Fatalf("valid payment constructor rejected: %v", err)
	}
	q := pqSetupItemsOn(t, f, nil, false, 1)
	pcRecord(t, q, pcFull(q)) // local evidence; InsertTx and report commit together
	if q.reportCount(t) != 1 || miCount(t, f.owner, `SELECT count(*) FROM river_payment.river_job j
	 JOIN integration.operations o ON o.id::text=j.args->>'operation_id'
	 WHERE o.id=$1 AND j.kind='payment_reconcile_v1' AND j.queue='payment_mock_v1'`, q.result.OperationID) != 1 {
		t.Fatal("correct same-transaction query-to-reconcile path missing payment-family job")
	}
}
