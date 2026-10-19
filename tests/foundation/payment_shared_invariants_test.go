// Purpose: issue #91: the shared payment invariants that the PAY-RM2 phase-2b replacement tests (PR #85) only covered
//
//	through retired-path helpers, re-asserted on the surviving Stripe path:
//	PSI01 shared capture ACL (from TestBuyerPaymentCaptureACLAndObservationBinding @2b27a5cae:253-280);
//	PSI02 start final-write fence: qualification expiring while the start waits on checkout.events conflicts and
//	      writes nothing (from TestBuyerPaymentQualificationExpiresDuringFinalWrite; contract stripe-psp-v1 §6.4 start);
//	PSI03 observation final DB-clock lease fence: lease expiring while record_stripe_observation waits on
//	      operation_events gives 40001 with no report, job or Complete (from TestBuyerPaymentQueryFaultAndLeaseWaitRollback);
//	PSI04 capture fault atomicity at six write points (from TestBuyerPaymentCaptureFaultsAreCausalAndAtomic).
//
// Depends on: sucOrder (stripe_untracked_capture_test.go), sstNewEnv/sstStore/sstFacts (stripe_start_test.go),
//
//	rm2Snapshot (payuni_removal_test.go), pcArgs/pcApply (payment_capture_test.go), worker authority constants,
//	waitForDatabaseLock/waitError/withApplicationName, platform pools, integration Claim.
//
// Used by: CI full foundation suite; scripts/dev/test-focused.sh '^TestPaymentSharedInvariant'.
// Invariants: synthetic data only, Stripe MOCK profile, no network. LC_PSI_INJECT (any non-empty value, CI extra_env
// calibration only) injects one fault per gate so each must fail; it is never set on PR runs.
package foundation_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"livecommerce/internal/checkout"
	"livecommerce/internal/command"
	integration "livecommerce/internal/integrations/core"
	"livecommerce/internal/platform"
)

func psiInject() bool { return os.Getenv("LC_PSI_INJECT") != "" }

// TestPaymentSharedInvariantCaptureACL (PSI01).
func TestPaymentSharedInvariantCaptureACL(t *testing.T) {
	ctx := context.Background()
	f := pwIsolatedFixture(t)
	e, attempt, _ := sucOrder(t, true, f)
	hash := e.observe(t, attempt, e.sessionReport(t, attempt, "complete", "paid", "succeeded", false))
	if psiInject() {
		mustExec(t, f.owner, `GRANT INSERT ON payments.facts TO commerce_checkout_runtime`)
	}
	for _, role := range []string{"commerce_runtime", "commerce_checkout_runtime", waPayment, waLive, waExpiry, waAds, waClaims, waLegacy} {
		var execute bool
		if err := f.owner.QueryRow(ctx, `SELECT has_function_privilege($1,'payments.apply_capture(uuid,bytea)','EXECUTE')`, role).Scan(&execute); err != nil || execute != (role == waPayment || role == waLive) {
			t.Errorf("apply_capture execute boundary %s=%v %v", role, execute, err)
		}
		for _, table := range []string{"payments.facts", "payments.review_cases", "fulfillment.payment_work_items", "inventory.ledger"} {
			if role == "commerce_runtime" && table == "inventory.ledger" {
				continue // merchant ADJUST authority is separately actor-fenced
			}
			var write bool
			if err := f.owner.QueryRow(ctx, `SELECT has_table_privilege($1,$2,'INSERT,UPDATE,DELETE')`, role, table).Scan(&write); err != nil || write {
				t.Errorf("direct business write %s %s=%v %v", role, table, write, err)
			}
		}
	}
	// 0162: only the merchant-cancel definer owner holds DELETE on work items; nobody holds a wider table privilege.
	for priv, want := range map[string]string{"DELETE": "commerce_checkout_writer", "UPDATE": "", "TRUNCATE": "", "REFERENCES": "", "TRIGGER": ""} {
		var got string
		if err := f.owner.QueryRow(ctx, `SELECT coalesce(string_agg(a.grantee::regrole::text,',' ORDER BY a.grantee::regrole::text),'')
			FROM pg_class c, aclexplode(c.relacl) a WHERE c.oid='fulfillment.payment_work_items'::regclass AND a.grantee<>c.relowner AND a.privilege_type=$1`, priv).Scan(&got); err != nil || got != want {
			t.Errorf("fulfillment.payment_work_items table-level %s grantees=%q (err %v), want %q", priv, got, err, want)
		}
	}
	var stateUpdate, orderUpdate bool
	if err := f.owner.QueryRow(ctx, `SELECT has_column_privilege('commerce_checkout_writer','fulfillment.payment_work_items','state','UPDATE'),
		has_column_privilege('commerce_checkout_writer','fulfillment.payment_work_items','order_id','UPDATE')`).Scan(&stateUpdate, &orderUpdate); err != nil || !stateUpdate || orderUpdate {
		t.Errorf("commerce_checkout_writer UPDATE on work items: state=%v order_id=%v err=%v, want state only", stateUpdate, orderUpdate, err)
	}
	if err := pcApply(f.runtime, attempt, hash); sqlState(err) != "42501" {
		t.Errorf("merchant runtime direct apply_capture not denied: %v", err)
	}
	if n := countRows(t, f.owner, `SELECT count(*) FROM payments.facts WHERE attempt_id=$1`, attempt); n != 0 {
		t.Errorf("denied apply wrote %d fact(s)", n)
	}
}

// TestPaymentSharedInvariantStartFinalWriteFence (PSI02).
func TestPaymentSharedInvariantStartFinalWriteFence(t *testing.T) {
	ctx := context.Background()
	p := psSetupItemsOn(t, fixture(t), 1)
	e := sstNewEnv(t, p.f, pwKeys(t))
	s := e.seed(t, p)
	name := "psi-start-" + t04Tag()
	pool, err := platform.OpenHostedPool(ctx, withApplicationName(t, hpRole(t, p.f), name))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	jobs, err := river.NewClient(riverpgxv5.New(pool), &river.Config{Schema: "river_payment"})
	if err != nil {
		t.Fatal(err)
	}
	svc, err := checkout.NewHostedPaymentService(ctx, pool, jobs, "PROVIDER_MOCK", e.keys, checkout.HostedProviders{Stripe: &e.scfg})
	if err != nil {
		t.Fatal(err)
	}
	before := sstFacts(t, p)
	holder, err := p.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback(ctx)
	if _, err = holder.Exec(ctx, `LOCK TABLE checkout.events IN ACCESS EXCLUSIVE MODE`); err != nil {
		t.Fatal(err)
	}
	life := "600 milliseconds"
	if psiInject() {
		life = "1 hour" // calibration: the proof never expires, so the late start must be (wrongly) accepted
	}
	var expiry time.Time
	if err = qualUpdateScan(ctx, p.f.owner, `UPDATE payments.account_qualifications SET expires_at=clock_timestamp()+$2::interval WHERE id=$1 RETURNING expires_at`, []any{s.qualification, life}, &expiry); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := s.begin(svc, t04Key("psi-late-wait"), s.input("zh-TW")); done <- err }()
	waitForDatabaseLock(t, p.f.owner, name)
	var valid bool
	if err = p.f.owner.QueryRow(ctx, `SELECT clock_timestamp()<$1`, expiry).Scan(&valid); err != nil || !valid {
		t.Fatalf("missed qualified-before-wait: %v", err)
	}
	if !psiInject() {
		mustExec(t, p.f.owner, `SELECT pg_sleep(GREATEST(0,extract(epoch FROM $1::timestamptz-clock_timestamp()))+0.02)`, expiry)
	}
	if err = holder.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err = waitError(t, done); !errors.Is(err, command.ErrConflict) {
		t.Fatalf("Stripe start with a proof that expired during the final write accepted: %v", err)
	}
	if after := sstFacts(t, p); after != before {
		t.Fatalf("late expiry left partial start rows: %v -> %v", before, after)
	}
}

// TestPaymentSharedInvariantObservationLeaseFence (PSI03).
func TestPaymentSharedInvariantObservationLeaseFence(t *testing.T) {
	ctx := context.Background()
	e, attempt, _ := sucOrder(t, true)
	tx, err := e.p.worker.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	c, err := (&integration.Service{}).Claim(ctx, tx, attempt, 30)
	if err != nil || c.Disposition != "claimed" {
		t.Fatalf("Stripe query claim %s %v", c.Disposition, err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	name := "psi_wait_" + t04Tag()
	pool, err := platform.OpenWorkerPool(ctx, withApplicationName(t, bcRole(t, e.p.f, waPayment), name), platform.WorkerPayment)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	report := pqJSON(e.sessionReport(t, attempt, "complete", "paid", "succeeded", false))
	holder, err := e.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback(ctx)
	if _, err = holder.Exec(ctx, `LOCK TABLE integration.operation_events IN ACCESS EXCLUSIVE MODE`); err != nil {
		t.Fatal(err)
	}
	life := "600 milliseconds"
	if psiInject() {
		life = "1 hour" // calibration: the lease stays live, so the record must (wrongly for this gate) commit
	}
	var expiry time.Time
	if err = e.f.owner.QueryRow(ctx, `UPDATE integration.operations SET lease_until=clock_timestamp()+$2::interval WHERE id=$1 RETURNING lease_until`, attempt, life).Scan(&expiry); err != nil {
		t.Fatal(err)
	}
	before := rm2Snapshot(t, e.p, attempt)
	done := make(chan error, 1)
	go func() {
		done <- func() error {
			tx, err := pool.Begin(ctx)
			if err != nil {
				return err
			}
			defer tx.Rollback(ctx)
			var hash string
			if err = tx.QueryRow(ctx, `SELECT encode(sha256(convert_to($1::jsonb::text,'UTF8')),'hex')`, report).Scan(&hash); err != nil {
				return err
			}
			jobs, err := river.NewClient(riverpgxv5.New(pool), &river.Config{Schema: "river_payment"})
			if err != nil {
				return err
			}
			job, err := jobs.InsertTx(ctx, tx, pcArgs{OperationID: attempt, ReportHash: hash, Version: 1}, nil)
			if err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `SELECT integration.record_stripe_observation($1,$2,$3,'PROVIDER_MOCK',$4::jsonb,$5,NULL)`, attempt, c.Generation, c.LeaseToken, report, job.Job.ID); err != nil {
				return err
			}
			return tx.Commit(ctx)
		}()
	}()
	waitForDatabaseLock(t, e.f.owner, name)
	var live bool
	if err = e.f.owner.QueryRow(ctx, `SELECT clock_timestamp()<$1`, expiry).Scan(&live); err != nil || !live {
		t.Fatal("missed live-lease wait")
	}
	if !psiInject() {
		mustExec(t, e.f.owner, `SELECT pg_sleep(GREATEST(0,extract(epoch FROM $1::timestamptz-clock_timestamp()))+0.02)`, expiry)
	}
	if err = holder.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	err = waitError(t, done)
	var pe *pgconn.PgError
	if !errors.As(err, &pe) || pe.Code != "40001" {
		t.Fatalf("observation recorded past its lease: %v", err)
	}
	if after := rm2Snapshot(t, e.p, attempt); after != before {
		t.Fatalf("late lease left a partial report/Complete:\nbefore=%s\nafter=%s", before, after)
	}
	if n := countRows(t, e.f.owner, `SELECT count(*) FROM river_payment.river_job WHERE kind='payment_reconcile_v1' AND args->>'operation_id'=$1`, attempt); n != 0 {
		t.Fatalf("late lease left %d reconcile job(s)", n)
	}
}

// TestPaymentSharedInvariantCaptureFaultsAtomic (PSI04).
func TestPaymentSharedInvariantCaptureFaultsAtomic(t *testing.T) {
	for _, point := range []struct{ table, event, predicate string }{
		{"payments.facts", "INSERT", "NEW.attempt_id=$ATTEMPT AND NEW.kind='CAPTURED'"},
		{"inventory.ledger", "INSERT", "NEW.checkout_id=$ORDER AND NEW.kind='ALLOCATE'"},
		{"inventory.reservations", "UPDATE", "NEW.id=$ORDER AND NEW.state='COMMITTED'"},
		{"checkout.orders", "UPDATE", "NEW.id=$ORDER AND NEW.commercial_state='CONFIRMED'"},
		{"checkout.events", "INSERT", "NEW.order_id=$ORDER AND NEW.action='checkout.payment_captured'"},
		{"fulfillment.payment_work_items", "INSERT", "NEW.order_id=$ORDER"},
	} {
		t.Run(point.table, func(t *testing.T) {
			e, attempt, _ := sucOrder(t, true)
			order := e.p.hold.OrderID
			hash := e.observe(t, attempt, e.sessionReport(t, attempt, "complete", "paid", "succeeded", false))
			before := rm2Snapshot(t, e.p, attempt)
			predicate := strings.ReplaceAll(strings.ReplaceAll(point.predicate, "$ORDER", "'"+order+"'::uuid"), "$ATTEMPT", "'"+attempt+"'::uuid")
			if psiInject() {
				predicate = "false" // calibration: the fault never fires, so the capture must commit
			}
			name := "psi_fault_" + t04Tag()
			mustExec(t, e.f.owner, fmt.Sprintf(`CREATE FUNCTION ops.%s() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF %s THEN RAISE EXCEPTION 'injected capture failure' USING ERRCODE='PSI01'; END IF; RETURN NEW; END $$`, name, predicate))
			mustExec(t, e.f.owner, fmt.Sprintf(`CREATE TRIGGER %s AFTER %s ON %s FOR EACH ROW EXECUTE FUNCTION ops.%s()`, name, point.event, point.table, name))
			dropped := false
			drop := func() {
				if !dropped {
					dropped = true
					mustExec(t, e.f.owner, fmt.Sprintf(`DROP TRIGGER %s ON %s`, name, point.table))
					mustExec(t, e.f.owner, fmt.Sprintf(`DROP FUNCTION ops.%s()`, name))
				}
			}
			defer drop()
			if err := e.apply(attempt, hash); sqlState(err) != "PSI01" {
				t.Fatalf("did not reach the %s injection: %v", point.table, err)
			}
			if after := rm2Snapshot(t, e.p, attempt); after != before {
				t.Fatalf("capture fault at %s partially committed or lost the durable observation:\nbefore=%s\nafter=%s", point.table, before, after)
			}
			if n := countRows(t, e.f.owner, `SELECT count(*) FROM payments.provider_observations WHERE attempt_id=$1 AND report_hash=$2`, attempt, hash); n != 1 {
				t.Fatalf("durable observation rows=%d", n)
			}
			drop()
			for i := 0; i < 2; i++ { // retry, then replay: exactly one effect
				if err := e.apply(attempt, hash); err != nil {
					t.Fatalf("retry %d after %s fault: %v", i, point.table, err)
				}
			}
			if n := countRows(t, e.f.owner, `SELECT (SELECT count(*) FROM payments.facts WHERE attempt_id=$1 AND kind='CAPTURED')*100
				+(SELECT count(*) FROM fulfillment.payment_work_items WHERE order_id=$2 AND state='READY')*10
				+(SELECT count(*) FROM inventory.ledger WHERE checkout_id=$2 AND kind='ALLOCATE')`, attempt, order); n != 111 {
				t.Fatalf("retry after %s fault: CAPTURED/READY/ALLOCATE digits=%03d, want 111", point.table, n)
			}
		})
	}
}
