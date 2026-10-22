// Purpose: issue #91: the shared payment invariants that the PAY-RM2 phase-2b replacement tests (PR #85) only covered
//
//	through retired-path helpers, re-asserted on the surviving Stripe path:
//	PSI01 shared capture ACL (from TestBuyerPaymentCaptureACLAndObservationBinding @2b27a5cae:253-280);
//	PSI02 start final-write fence: qualification expiring while the start waits on checkout.events conflicts and
//	      writes nothing (from TestBuyerPaymentQualificationExpiresDuringFinalWrite; contract stripe-psp-v1 §6.4 start);
//	PSI03 observation final DB-clock lease fence: lease expiring while record_stripe_observation waits on
//	      operation_events gives 40001 with no report, job or Complete (from TestBuyerPaymentQueryFaultAndLeaseWaitRollback);
//	PSI04 capture fault atomicity at six write points (from TestBuyerPaymentCaptureFaultsAreCausalAndAtomic);
//	PSI05 capture of an order with two tracked SKUs: exact once-per-SKU ALLOCATE/balance move and idempotent replay
//	      (issue #107/#116, map row 84: TestBuyerPaymentCaptureMultiSKUSingleCommit had no Stripe equivalent);
//	PSI06 lock-wait invisibility: a capture held mid-transaction on its second balance lock is invisible to other
//	      connections, and a transaction queued on its order lock sees only the complete capture (issue #107/#116,
//	      map row 82: TestBuyerPaymentCaptureBalanceWaitIsAtomic had no Stripe equivalent);
//	PSI05 and PSI07 drive the real Go payments.CaptureWorker (issue #147 PT-2). PSI07 proves, per gate on a fresh
//	      two-SKU order, that a mismatched provider or observation profile cancels the job and that the worker's own 1s
//	      lock_timeout aborts a capture blocked on a held balance lock, each with nothing written, and that a later
//	      retry captures exactly once. PSI06 keeps the direct SQL call: its MVCC probe outlasts the 1s lock_timeout.
//
// Depends on: sucOrder (stripe_untracked_capture_test.go), sstNewEnv/sstStore/sstFacts (stripe_start_test.go),
//
//	rm2Snapshot (payuni_removal_test.go), pcArgs/pcApply (payment_capture_test.go), worker authority constants,
//	waitForDatabaseLock/waitError/withApplicationName, platform pools, integration Claim; PSI05/06 also
//	psSetupItemsOn (payment_start_test.go) and slrEnv pin/observe/apply/sessionReport (stripe_live_registrar_test.go);
//	PSI05/07 payments.NewCaptureWorker (internal/payments/capture_worker.go) and slrReplica (stripe_live_registrar_test.go).
//
// Used by: CI full foundation suite; scripts/dev/test-focused.sh '^TestPaymentSharedInvariant'.
// Invariants: synthetic data only, Stripe MOCK profile, no network. LC_PSI_INJECT (any non-empty value, CI extra_env
// calibration only) injects one fault per gate so each must fail; it is never set on PR runs. An injected trigger and
// its function are dropped by a t.Cleanup registered before either is created (issue #147 PT-6).
package foundation_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"livecommerce/internal/checkout"
	"livecommerce/internal/command"
	integration "livecommerce/internal/integrations/core"
	"livecommerce/internal/payments"
	"livecommerce/internal/platform"
)

func psiInject() bool { return os.Getenv("LC_PSI_INJECT") != "" }

// psiAwaitLock is waitForDatabaseLock that also reports the call's own error when it returns before ever waiting
// (otherwise a precondition failure inside the call reads as "lock wait not observed").
func psiAwaitLock(t *testing.T, owner *pgxpool.Pool, name string, done chan error) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-done:
			t.Fatalf("call returned before waiting on the lock: %v", err)
		default:
		}
		var waiting bool
		if err := owner.QueryRow(context.Background(), `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE application_name=$1 AND wait_event_type='Lock')`, name).Scan(&waiting); err != nil {
			t.Fatalf("observe database lock: %v", err)
		}
		if waiting {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("database lock wait was not observed for %s", name)
}

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
	// The worker's exact §5.7 projection (36 keys): sessionReport is the owner-insert subset, so the remaining keys
	// are added with the values a retrieve of a pinned, paid MOCK session carries.
	full := e.sessionReport(t, attempt, "complete", "paid", "succeeded", false)
	var keyVersion int64
	if err = e.f.owner.QueryRow(ctx, `SELECT credential_version FROM checkout.payment_attempts WHERE id=$1`, attempt).Scan(&keyVersion); err != nil {
		t.Fatal(err)
	}
	for k, v := range map[string]any{"KeyVersion": keyVersion, "RequestID": "", "SendCount": 1, "PresentmentAmount": nil, "Created": nil,
		"PaymentIntentID": "", "ErrorClass": "", "ErrorCode": "", "HTTPStatus": nil, "ListMatchCount": nil, "LocalReason": ""} {
		full[k] = v
	}
	if len(full) != 36 {
		t.Fatalf("fixture: Stripe observation has %d keys, want 36", len(full))
	}
	report := pqJSON(full)
	holder, err := e.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback(ctx)
	// The snapshot and the original lease are read before the lock: rm2Snapshot reads operation_events, which this
	// holder locks ACCESS EXCLUSIVE, so reading it afterwards would wait on the test's own lock.
	before := rm2Snapshot(t, e.p, attempt)
	var origLease time.Time
	if err = e.f.owner.QueryRow(ctx, `SELECT lease_until FROM integration.operations WHERE id=$1`, attempt).Scan(&origLease); err != nil {
		t.Fatal(err)
	}
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
	psiAwaitLock(t, e.f.owner, name, done)
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
	var code string
	var leased bool
	if err = e.f.owner.QueryRow(ctx, `SELECT coalesce(result_code,''),lease_until IS NOT NULL FROM integration.operations WHERE id=$1`, attempt).Scan(&code, &leased); err != nil || code == "stripe_report_observed" || !leased {
		t.Fatalf("failed record partially completed the operation: code=%q leased=%v %v", code, leased, err)
	}
	// Undo only this test's own lease shortening; the whole aggregate must then equal the pre-lock snapshot.
	mustExec(t, e.f.owner, `UPDATE integration.operations SET lease_until=$2 WHERE id=$1`, attempt, origLease)
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
			psiDropTriggerOnCleanup(t, e.f.owner, name, point.table)
			mustExec(t, e.f.owner, fmt.Sprintf(`CREATE FUNCTION ops.%s() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF %s THEN RAISE EXCEPTION 'injected capture failure' USING ERRCODE='PSI01'; END IF; RETURN NEW; END $$`, name, predicate))
			mustExec(t, e.f.owner, fmt.Sprintf(`CREATE TRIGGER %s AFTER %s ON %s FOR EACH ROW EXECUTE FUNCTION ops.%s()`, name, point.event, point.table, name))
			if err := e.apply(attempt, hash); sqlState(err) != "PSI01" {
				t.Fatalf("did not reach the %s injection: %v", point.table, err)
			}
			if after := rm2Snapshot(t, e.p, attempt); after != before {
				t.Fatalf("capture fault at %s partially committed or lost the durable observation:\nbefore=%s\nafter=%s", point.table, before, after)
			}
			if n := countRows(t, e.f.owner, `SELECT count(*) FROM payments.provider_observations WHERE attempt_id=$1 AND report_hash=$2`, attempt, hash); n != 1 {
				t.Fatalf("durable observation rows=%d", n)
			}
			mustExec(t, e.f.owner, fmt.Sprintf(`DROP TRIGGER %s ON %s`, name, point.table))
			mustExec(t, e.f.owner, fmt.Sprintf(`DROP FUNCTION ops.%s()`, name))
			for i := 0; i < 2; i++ { // retry, then replay: exactly one effect
				if err := e.apply(attempt, hash); err != nil {
					t.Fatalf("retry %d after %s fault: %v", i, point.table, err)
				}
			}
			var got [3]int64
			if err := e.f.owner.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM payments.facts WHERE attempt_id=$1 AND kind='CAPTURED'),
				(SELECT count(*) FROM fulfillment.payment_work_items WHERE order_id=$2 AND state='READY'),
				(SELECT count(*) FROM inventory.ledger WHERE checkout_id=$2 AND kind='ALLOCATE')`, attempt, order).Scan(&got[0], &got[1], &got[2]); err != nil || got != [3]int64{1, 1, 1} {
				t.Fatalf("retry after %s fault: CAPTURED/READY/ALLOCATE=%v err=%v, want [1 1 1]", point.table, got, err)
			}
		})
	}
}

// psiMultiOrder is sucOrder with two tracked SKUs and no untracked line: real Begin reserves 2 of each SKU's 10 units,
// then a real MOCK Stripe start and a pinned session (issue #107 carry-over: map rows 82 and 84 had no Stripe test).
func psiMultiOrder(t *testing.T) (*slrEnv, string) {
	t.Helper()
	p := psSetupItemsOn(t, fixture(t), 2)
	e := sstNewEnv(t, p.f, pwKeys(t))
	s := e.seed(t, p)
	result, err := s.begin(e.svc, t04Key("psi-multi-payment"), s.input("zh-TW"))
	if err != nil {
		t.Fatal(err)
	}
	env := &slrEnv{sstEnv: e, p: p}
	env.pin(t, result.AttemptID)
	if n := countRows(t, p.f.owner, `SELECT count(*) FROM inventory.reservation_lines WHERE reservation_id=$1`, p.hold.OrderID); n != 2 {
		t.Fatalf("fixture: real Begin reserved %d tracked lines, want 2", n)
	}
	return env, result.AttemptID
}

type psiLine struct {
	warehouse, sku              string
	quantity                    int64
	reserved, allocated, onHand int64
}

// psiLines returns the order's reservation lines with their current balances, in apply_stripe_observation's lock
// order (warehouse_id, sku_id).
func psiLines(t *testing.T, e *slrEnv) []psiLine {
	t.Helper()
	rows, err := e.f.owner.Query(context.Background(), `SELECT l.warehouse_id::text,l.sku_id::text,l.quantity,b.reserved,b.allocated,b.on_hand
	 FROM inventory.reservation_lines l JOIN inventory.balances b USING(tenant_id,store_id,warehouse_id,sku_id)
	 WHERE l.reservation_id=$1 ORDER BY l.warehouse_id,l.sku_id`, e.p.hold.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []psiLine
	for rows.Next() {
		var l psiLine
		if err = rows.Scan(&l.warehouse, &l.sku, &l.quantity, &l.reserved, &l.allocated, &l.onHand); err != nil {
			t.Fatal(err)
		}
		out = append(out, l)
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// psiAssertCaptured checks the exact once-per-SKU capture effect against the pre-capture lines: one ALLOCATE ledger
// row per SKU moving exactly its reserved quantity to allocated, on_hand unchanged, and exactly one CAPTURED fact,
// COMMITTED reservation, CONFIRMED order, captured event and READY work item.
func psiAssertCaptured(t *testing.T, e *slrEnv, attempt string, before []psiLine) {
	t.Helper()
	ctx := context.Background()
	order := e.p.hold.OrderID
	after := psiLines(t, e)
	if len(after) != len(before) {
		t.Fatalf("reservation lines %d -> %d", len(before), len(after))
	}
	for i, b := range before {
		a := after[i]
		if a.sku != b.sku || a.reserved != b.reserved-b.quantity || a.allocated != b.allocated+b.quantity || a.onHand != b.onHand {
			t.Errorf("SKU %s balance reserved/allocated/on_hand %d/%d/%d -> %d/%d/%d, want reserved-%d allocated+%d on_hand unchanged",
				b.sku, b.reserved, b.allocated, b.onHand, a.reserved, a.allocated, a.onHand, b.quantity, b.quantity)
		}
		var rows int
		var debit, credit int64
		if err := e.f.owner.QueryRow(ctx, `SELECT count(*),coalesce(sum(-delta_reserved),0),coalesce(sum(delta_allocated),0) FROM inventory.ledger
		 WHERE checkout_id=$1 AND warehouse_id=$2 AND sku_id=$3 AND kind='ALLOCATE' AND payment_attempt_id=$4 AND payment_fact_kind='CAPTURED'`,
			order, b.warehouse, b.sku, attempt).Scan(&rows, &debit, &credit); err != nil || rows != 1 || debit != b.quantity || credit != b.quantity {
			t.Errorf("SKU %s ALLOCATE rows=%d reserved-=%d allocated+=%d err=%v, want 1/%d/%d", b.sku, rows, debit, credit, err, b.quantity, b.quantity)
		}
	}
	if n := countRows(t, e.f.owner, `SELECT count(*) FROM inventory.ledger WHERE checkout_id=$1 AND kind='ALLOCATE'`, order); n != len(before) {
		t.Errorf("ALLOCATE rows for the order=%d, want %d", n, len(before))
	}
	var orderState, reservationState string
	if err := e.f.owner.QueryRow(ctx, `SELECT o.commercial_state,r.state FROM checkout.orders o JOIN inventory.reservations r ON r.id=o.id WHERE o.id=$1`, order).Scan(&orderState, &reservationState); err != nil || orderState != "CONFIRMED" || reservationState != "COMMITTED" {
		t.Errorf("order/reservation=%s/%s err=%v, want CONFIRMED/COMMITTED", orderState, reservationState, err)
	}
	var got [3]int64
	if err := e.f.owner.QueryRow(ctx, `SELECT (SELECT count(*) FROM payments.facts WHERE attempt_id=$1 AND kind='CAPTURED'),
		(SELECT count(*) FROM fulfillment.payment_work_items WHERE order_id=$2 AND state='READY'),
		(SELECT count(*) FROM checkout.events WHERE order_id=$2 AND action='checkout.payment_captured')`, attempt, order).Scan(&got[0], &got[1], &got[2]); err != nil || got != [3]int64{1, 1, 1} {
		t.Errorf("CAPTURED/READY/captured-event=%v err=%v, want [1 1 1]", got, err)
	}
}

// psiDropTriggerOnCleanup registers the drop of an injected trigger and its ops function before either is created,
// so a failed CREATE TRIGGER cannot leak the function (issue #147 PT-6). IF EXISTS keeps it valid after a partial
// creation or an explicit earlier drop; it runs before the fixture's pools close because it is registered after them.
func psiDropTriggerOnCleanup(t *testing.T, owner *pgxpool.Pool, name, table string) {
	t.Helper()
	t.Cleanup(func() {
		mustExec(t, owner, fmt.Sprintf(`DROP TRIGGER IF EXISTS %s ON %s`, name, table))
		mustExec(t, owner, fmt.Sprintf(`DROP FUNCTION IF EXISTS ops.%s()`, name))
	})
}

// psiJob binds the real worker to the reconcile job for (attempt, hash). The public job payload is decoded into the
// worker's own argument type (as sucWork does), so each call runs Work exactly as River would: provider check,
// statement/lock timeouts, then payments.apply_capture. No *testing.T is touched, so it may run in a goroutine.
func psiJob[T river.JobArgs](t *testing.T, w river.Worker[T], attempt string, hash []byte) func() error {
	t.Helper()
	raw, err := json.Marshal(pcArgs{OperationID: attempt, ReportHash: hex.EncodeToString(hash), Version: 1})
	if err != nil {
		t.Fatal(err)
	}
	var args T
	if err = json.Unmarshal(raw, &args); err != nil {
		t.Fatal(err)
	}
	return func() error { return w.Work(context.Background(), &river.Job[T]{Args: args}) }
}

// psiCaptureJob is psiJob for the production capture worker on pool.
func psiCaptureJob(t *testing.T, pool *pgxpool.Pool, attempt string, hash []byte) func() error {
	t.Helper()
	w, err := payments.NewCaptureWorker(context.Background(), pool)
	if err != nil {
		t.Fatal(err)
	}
	return psiJob(t, w, attempt, hash)
}

// TestPaymentSharedInvariantMultiSKUCapture (PSI05): a Stripe capture of an order with two tracked SKUs, run through
// the real Go capture worker, allocates each SKU exactly once in one commit, and replaying the job changes nothing.
func TestPaymentSharedInvariantMultiSKUCapture(t *testing.T) {
	e, attempt := psiMultiOrder(t)
	order := e.p.hold.OrderID
	before := psiLines(t, e)
	hash := e.observe(t, attempt, e.sessionReport(t, attempt, "complete", "paid", "succeeded", false))
	work := psiCaptureJob(t, e.p.worker, attempt, hash)
	if psiInject() {
		// calibration: the last SKU's ALLOCATE is silently dropped, so per-SKU conservation must fail
		name := "psi_drop_" + t04Tag()
		psiDropTriggerOnCleanup(t, e.f.owner, name, "inventory.ledger")
		mustExec(t, e.f.owner, fmt.Sprintf(`CREATE FUNCTION ops.%s() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
		 IF NEW.checkout_id='%s'::uuid AND NEW.kind='ALLOCATE' AND NEW.sku_id='%s'::uuid THEN RETURN NULL; END IF; RETURN NEW; END $$`, name, order, before[len(before)-1].sku))
		mustExec(t, e.f.owner, fmt.Sprintf(`CREATE TRIGGER %s BEFORE INSERT ON inventory.ledger FOR EACH ROW EXECUTE FUNCTION ops.%s()`, name, name))
	}
	if err := work(); err != nil {
		t.Fatalf("multi-SKU capture: %v", err)
	}
	psiAssertCaptured(t, e, attempt, before)
	captured := rm2Snapshot(t, e.p, attempt)
	for i := 0; i < 2; i++ {
		if err := work(); err != nil {
			t.Fatalf("replay %d: %v", i, err)
		}
	}
	if after := rm2Snapshot(t, e.p, attempt); after != captured {
		t.Fatalf("replay changed the captured aggregate:\nbefore=%s\nafter=%s", captured, after)
	}
	psiAssertCaptured(t, e, attempt, before)
}

// TestPaymentSharedInvariantCaptureLockWaitInvisible (PSI06): the capture is held mid-transaction (CAPTURED fact
// written, first balance locked, waiting on the second balance). Meanwhile other connections see the exact
// pre-capture aggregate, and a transaction queued on the capture's order lock sees only the complete capture.
func TestPaymentSharedInvariantCaptureLockWaitInvisible(t *testing.T) {
	ctx := context.Background()
	e, attempt := psiMultiOrder(t)
	order := e.p.hold.OrderID
	lines := psiLines(t, e)
	hash := e.observe(t, attempt, e.sessionReport(t, attempt, "complete", "paid", "succeeded", false))
	before := rm2Snapshot(t, e.p, attempt)
	name := "psi_lockwait_" + t04Tag()
	pool, err := platform.OpenWorkerPool(ctx, withApplicationName(t, bcRole(t, e.p.f, waPayment), name), platform.WorkerPayment)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	holder, err := e.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback(ctx)
	lockBalance := `SELECT 1 FROM inventory.balances b WHERE (b.tenant_id,b.store_id,b.warehouse_id,b.sku_id)=($1::uuid,$2::uuid,$3::uuid,$4::uuid) FOR UPDATE`
	if !psiInject() {
		// the second balance in the capture's lock order: the capture gets past its fact insert and first balance lock
		last := lines[len(lines)-1]
		if _, err = holder.Exec(ctx, lockBalance, e.p.f.tenantA, e.p.f.storeA1, last.warehouse, last.sku); err != nil {
			t.Fatal(err)
		}
	}
	done := make(chan error, 1)
	go func() {
		_, err := pool.Exec(ctx, `SELECT payments.apply_capture($1::uuid,$2::bytea)`, attempt, hash)
		done <- err
	}()
	type seen struct {
		order               string
		reserved, allocated int64
		counts              [4]int64 // CAPTURED facts, ALLOCATE rows, READY work items, COMMITTED reservations
		err                 error
	}
	waiterPid, waiterSeen := make(chan uint32, 1), make(chan seen, 1)
	if psiInject() {
		// calibration: nothing holds the capture, so it commits before the "during the wait" read, which must flag it
		if err = waitError(t, done); err != nil {
			t.Fatal(err)
		}
	} else {
		psiAwaitLock(t, e.f.owner, name, done)
		// non-vacuity: the waiting capture already holds the first balance, so it is past its CAPTURED fact insert
		first := lines[0]
		probe, err := e.f.owner.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		_, err = probe.Exec(ctx, lockBalance+` NOWAIT`, e.p.f.tenantA, e.p.f.storeA1, first.warehouse, first.sku)
		probe.Rollback(ctx)
		if sqlState(err) != "55P03" {
			t.Fatalf("capture is not holding the first balance while it waits: %v", err)
		}
		// a READ COMMITTED transaction queued on the capture's aggregate (order) lock
		go func() {
			var s seen
			defer func() { waiterSeen <- s }()
			conn, err := e.f.owner.Acquire(ctx)
			if err != nil {
				s.err = err
				waiterPid <- 0
				return
			}
			defer conn.Release()
			waiterPid <- conn.Conn().PgConn().PID()
			tx, err := conn.Begin(ctx)
			if err != nil {
				s.err = err
				return
			}
			defer tx.Rollback(ctx)
			if s.err = tx.QueryRow(ctx, `SELECT commercial_state FROM checkout.orders WHERE id=$1 FOR UPDATE`, order).Scan(&s.order); s.err != nil {
				return
			}
			s.err = tx.QueryRow(ctx, `SELECT coalesce(sum(b.reserved),0),coalesce(sum(b.allocated),0),
			 (SELECT count(*) FROM payments.facts WHERE attempt_id=$2 AND kind='CAPTURED'),
			 (SELECT count(*) FROM inventory.ledger WHERE checkout_id=$1 AND kind='ALLOCATE'),
			 (SELECT count(*) FROM fulfillment.payment_work_items WHERE order_id=$1 AND state='READY'),
			 (SELECT count(*) FROM inventory.reservations WHERE id=$1 AND state='COMMITTED')
			 FROM inventory.reservation_lines l JOIN inventory.balances b USING(tenant_id,store_id,warehouse_id,sku_id) WHERE l.reservation_id=$1`,
				order, attempt).Scan(&s.reserved, &s.allocated, &s.counts[0], &s.counts[1], &s.counts[2], &s.counts[3])
		}()
		pid := <-waiterPid
		deadline := time.Now().Add(3 * time.Second)
		for waiting := false; !waiting; {
			// a waiter that ended before queueing (no connection, failed Begin or read) reports its own error
			select {
			case s := <-waiterSeen:
				t.Fatalf("order-lock waiter ended before queueing behind the capture: order=%q err=%v", s.order, s.err)
			default:
			}
			if time.Now().After(deadline) {
				t.Fatal("order-lock waiter never queued behind the capture")
			}
			if err = e.f.owner.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE pid=$1 AND wait_event_type='Lock')`, int64(pid)).Scan(&waiting); err != nil {
				t.Fatal(err)
			}
			if !waiting {
				time.Sleep(10 * time.Millisecond)
			}
		}
	}
	if during := rm2Snapshot(t, e.p, attempt); during != before {
		t.Fatalf("uncommitted capture visible during its lock wait:\nbefore=%s\nduring=%s", before, during)
	}
	if err = holder.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err = waitError(t, done); err != nil {
		t.Fatalf("capture after lock release: %v", err)
	}
	var wantReserved, wantAllocated int64
	for _, l := range lines {
		wantReserved += l.reserved - l.quantity
		wantAllocated += l.allocated + l.quantity
	}
	select {
	case s := <-waiterSeen:
		if want := [4]int64{1, 2, 1, 1}; s.err != nil || s.order != "CONFIRMED" || s.reserved != wantReserved || s.allocated != wantAllocated || s.counts != want {
			t.Fatalf("order-lock waiter saw order=%s reserved=%d allocated=%d CAPTURED/ALLOCATE/READY/COMMITTED=%v err=%v, want CONFIRMED %d %d %v",
				s.order, s.reserved, s.allocated, s.counts, s.err, wantReserved, wantAllocated, want)
		}
	case <-time.After(6 * time.Second):
		t.Fatal("order-lock waiter did not return after the capture committed")
	}
	psiAssertCaptured(t, e, attempt, lines)
}

// TestPaymentSharedInvariantCaptureWorkerGates (PSI07): the real Go capture worker on a fresh two-SKU MOCK order per
// gate. A mismatched provider (Go check before apply_capture) or observation profile (apply_capture provenance) is a
// permanent cancel; a capture blocked on a held balance lock is aborted by the worker's own 1s lock_timeout as a
// retryable error well inside its 5s statement/job budget. Each writes nothing, and a later retry captures exactly once.
func TestPaymentSharedInvariantCaptureWorkerGates(t *testing.T) {
	for _, gate := range []string{"provider", "profile", "lock_timeout"} {
		t.Run(gate, func(t *testing.T) {
			ctx := context.Background()
			e, attempt := psiMultiOrder(t)
			lines := psiLines(t, e)
			hash := e.observe(t, attempt, e.sessionReport(t, attempt, "complete", "paid", "succeeded", false))
			before := rm2Snapshot(t, e.p, attempt)
			name := "psi_worker_" + t04Tag()
			pool, err := platform.OpenWorkerPool(ctx, withApplicationName(t, bcRole(t, e.p.f, waPayment), name), platform.WorkerPayment)
			if err != nil {
				t.Fatal(err)
			}
			defer pool.Close()
			work := psiCaptureJob(t, pool, attempt, hash)
			var cancelled *river.JobCancelError
			switch gate {
			case "provider", "profile":
				tamper, restore := `UPDATE integration.operations SET provider='payuni' WHERE id=$1 AND provider='stripe'`,
					`UPDATE integration.operations SET provider='stripe' WHERE id=$1 AND provider='payuni'`
				if gate == "profile" {
					tamper, restore = `UPDATE payments.provider_observations SET execution_profile='SANDBOX' WHERE attempt_id=$1 AND report_hash=$2 AND execution_profile='PROVIDER_MOCK'`,
						`UPDATE payments.provider_observations SET execution_profile='PROVIDER_MOCK' WHERE attempt_id=$1 AND report_hash=$2 AND execution_profile='SANDBOX'`
				}
				args := []any{attempt}
				if gate == "profile" {
					args = append(args, hash)
				}
				if !psiInject() { // calibration: no mismatch, so the worker must (wrongly for this gate) capture
					slrReplica(t, e.f, tamper, args...)
				}
				err = work()
				if !psiInject() {
					slrReplica(t, e.f, restore, args...)
				}
				if !errors.As(err, &cancelled) {
					t.Fatalf("worker did not cancel a job with a mismatched %s: %v", gate, err)
				}
			case "lock_timeout":
				holder, err := e.f.owner.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer holder.Rollback(ctx)
				if !psiInject() { // calibration: nothing holds the balance, so the capture must (wrongly for this gate) commit
					last := lines[len(lines)-1]
					if _, err = holder.Exec(ctx, `SELECT 1 FROM inventory.balances b WHERE (b.tenant_id,b.store_id,b.warehouse_id,b.sku_id)=($1::uuid,$2::uuid,$3::uuid,$4::uuid) FOR UPDATE`,
						e.p.f.tenantA, e.p.f.storeA1, last.warehouse, last.sku); err != nil {
						t.Fatal(err)
					}
				}
				done := make(chan error, 1)
				start := time.Now()
				go func() { done <- work() }()
				if !psiInject() {
					psiAwaitLock(t, e.f.owner, name, done)
				}
				err = waitError(t, done)
				elapsed := time.Since(start)
				// The holder still holds the balance, so only a timeout can end the wait: lock_timeout (1s) and not the
				// 5s statement_timeout/job context, and as a retryable error, not a cancel.
				if err == nil || errors.As(err, &cancelled) || elapsed < time.Second || elapsed >= 4*time.Second {
					t.Fatalf("capture blocked on a held balance was not aborted by the worker's 1s lock_timeout: err=%v after %v", err, elapsed)
				}
				if err = holder.Rollback(ctx); err != nil {
					t.Fatal(err)
				}
			}
			if after := rm2Snapshot(t, e.p, attempt); after != before {
				t.Fatalf("rejected/aborted %s attempt left partial state:\nbefore=%s\nafter=%s", gate, before, after)
			}
			if err = work(); err != nil {
				t.Fatalf("retry after %s: %v", gate, err)
			}
			psiAssertCaptured(t, e, attempt, lines)
			captured := rm2Snapshot(t, e.p, attempt)
			if err = work(); err != nil {
				t.Fatalf("replay after %s: %v", gate, err)
			}
			if after := rm2Snapshot(t, e.p, attempt); after != captured {
				t.Fatalf("replay after %s changed the captured aggregate:\nbefore=%s\nafter=%s", gate, captured, after)
			}
			psiAssertCaptured(t, e, attempt, lines)
		})
	}
}
