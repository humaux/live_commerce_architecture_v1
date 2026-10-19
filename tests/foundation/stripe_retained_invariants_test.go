// Responsibility: live Stripe PROVIDER_MOCK equivalents of retired adapter invariants.
// Depends on sst/sfl registrar, independent Stripe fake and production Go assembly.
// Foundation executes these gates; no real PSP, SANDBOX or LIVE request is allowed.
package foundation_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"livecommerce/internal/buyer"
	"livecommerce/internal/checkout"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

func TestStripeConcurrentKeys(t *testing.T) {
	for _, same := range []bool{true, false} {
		t.Run(fmt.Sprint(same), func(t *testing.T) {
			p := psSetup(t)
			e := sstNewEnv(t, p.f, pwKeys(t))
			s := e.seed(t, p)
			key := t04Key("rm2-stripe-concurrent")
			ready := make(chan struct{})
			type outcome struct {
				out checkout.PaymentResult
				err error
			}
			done := make(chan outcome, 2)
			before := sstFacts(t, p)
			seeded := len(e.fake.Requests())
			for i := 0; i < 2; i++ {
				go func(i int) {
					<-ready
					k := key
					if !same {
						k += fmt.Sprint(i)
					}
					out, err := s.begin(e.svc, k, s.input("en"))
					done <- outcome{out, err}
				}(i)
			}
			close(ready)
			wins := 0
			id := ""
			for range 2 {
				select {
				case got := <-done:
					if got.err == nil {
						wins++
						if id != "" && id != got.out.AttemptID {
							t.Fatal("two attempts")
						}
						id = got.out.AttemptID
					} else if !errors.Is(got.err, command.ErrConflict) {
						t.Fatal(got.err)
					}
				case <-time.After(10 * time.Second):
					t.Fatal("concurrent starts did not return")
				}
			}
			want := 1
			if same {
				want = 2
			}
			if wins != want {
				t.Fatalf("successes=%d want %d", wins, want)
			}
			facts := sstFacts(t, p)
			if facts[0] != 1 || facts[1] != 1 || facts[2] != 1 || facts[3] != 1 || facts[4] != 1 || facts[5] != before[5]+1 || facts[6] != 1 || facts[7] != 1 || facts[8] != 0 || len(e.fake.Requests()) != seeded {
				t.Fatalf("non-atomic contenders: %v", facts)
			}
		})
	}
}

func TestStripeStartFaultRollback(t *testing.T) {
	for _, table := range []string{"river_payment.river_job", "checkout.payment_attempts", "integration.operations", "integration.operation_events", "payments.stripe_sessions", "checkout.events", "checkout.command_results"} {
		t.Run(table, func(t *testing.T) {
			p := psSetup(t)
			e := sstNewEnv(t, p.f, pwKeys(t))
			s := e.seed(t, p)
			name := "rm2_stripe_fault_" + t04Tag()
			mustExec(t, p.f.owner, `CREATE SEQUENCE public.`+name+`_hits`)
			mustExec(t, p.f.owner, `GRANT USAGE ON SEQUENCE public.`+name+`_hits TO commerce_hosted_runtime,commerce_checkout_writer,commerce_integration_writer`)
			mustExec(t, p.f.owner, `CREATE FUNCTION public.`+name+`() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF current_setting('app.buyer_id',true)=`+quoteLiteral(p.cap.Scope.OwnerID)+` THEN PERFORM nextval('public.`+name+`_hits'); RAISE EXCEPTION 'stripe start fault' USING ERRCODE='PCT03'; END IF; RETURN NEW; END $$`)
			mustExec(t, p.f.owner, `CREATE TRIGGER `+name+` BEFORE INSERT ON `+table+` FOR EACH ROW EXECUTE FUNCTION public.`+name+`() `)
			t.Cleanup(func() {
				mustExec(t, p.f.owner, `DROP TRIGGER IF EXISTS `+name+` ON `+table)
				mustExec(t, p.f.owner, `DROP FUNCTION public.`+name+`() `)
				mustExec(t, p.f.owner, `DROP SEQUENCE public.`+name+`_hits`)
			})
			before := rm2ScopeSnapshot(t, p, true)
			seeded := len(e.fake.Requests())
			if _, err := s.begin(e.svc, t04Key("rm2-stripe-fault"), s.input("en")); err == nil {
				t.Fatal("injected start fault missing")
			}
			var fired bool
			if err := p.f.owner.QueryRow(context.Background(), `SELECT is_called FROM public.`+name+`_hits`).Scan(&fired); err != nil || !fired {
				t.Fatal("intended fault not reached", err)
			}
			if rm2ScopeSnapshot(t, p, true) != before || len(e.fake.Requests()) != seeded {
				t.Fatal("start fault committed partial rows or sent provider request")
			}
			mustExec(t, p.f.owner, `DROP TRIGGER `+name+` ON `+table)
			if _, err := s.begin(e.svc, t04Key("rm2-stripe-after-fault"), s.input("en")); err != nil {
				t.Fatal("clean retry after rollback", err)
			}
		})
	}
}

func TestStripeHTTPThreeLocalesAndConcurrentHandoff(t *testing.T) {
	for _, locale := range []string{"zh-CN", "zh-TW", "en"} {
		t.Run(locale, func(t *testing.T) {
			x := sbhNew(t, false)
			initial := bphRaw(t, x.srv.request(t, "GET", x.path, x.h.cap.Token, "", nil, nil), 200, bpViewKeys)
			bpState(t, initial, "payment_state", "NOT_STARTED")
			bpState(t, initial, "handoff_state", "NONE")
			var methods []map[string]json.RawMessage
			if err := json.Unmarshal(initial["methods"], &methods); err != nil || len(methods) != 1 || string(methods[0]["code"]) != `"stripe_checkout"` {
				t.Fatal("Stripe method projection", err)
			}
			if got := bpKeys(methods[0]); fmt.Sprint(got) != fmt.Sprint(bpMethodKeys) {
				t.Fatal("private method fields")
			}
			input := map[string]any{"method_code": "stripe_checkout", "method_version": x.s.method, "locale": locale}
			key := t04Key("rm2-three-locales")
			first := x.srv.request(t, "POST", x.path+"/prepare", x.h.cap.Token, key, input, nil)
			prepared := bphRaw(t, first, 200, []string{"amount_minor", "currency", "order_id", "state"})
			bpState(t, prepared, "state", "PAYMENT_PENDING")
			if string(prepared["amount_minor"]) != "2500" {
				t.Fatal("frozen amount changed")
			}
			before := rm2ScopeSnapshot(t, x.h.psHarness, true)
			replay := x.srv.request(t, "POST", x.path+"/prepare", x.h.cap.Token, key, input, nil)
			if replay.status != 200 || !bytes.Equal(first.body, replay.body) || rm2ScopeSnapshot(t, x.h.psHarness, true) != before {
				t.Fatal("HTTP replay changed rows or bytes")
			}
			if err := x.h.f.owner.QueryRow(context.Background(), `SELECT id::text FROM checkout.payment_attempts WHERE order_id=$1`, x.h.hold.OrderID).Scan(&x.attempt); err != nil {
				t.Fatal(err)
			}
			var frozen, wireLocale string
			if err := x.h.f.owner.QueryRow(context.Background(), `SELECT locale,create_params->>'locale' FROM payments.stripe_sessions WHERE attempt_id=$1`, x.attempt).Scan(&frozen, &wireLocale); err != nil {
				t.Fatal(err)
			}
			want := locale
			if locale == "zh-CN" {
				want = "zh"
			} // Match the installed Stripe locale contract.
			if frozen != locale || wireLocale != want {
				t.Fatalf("locale=%s params=%s want=%s", frozen, wireLocale, want)
			}
			bpState(t, x.view(t), "handoff_state", "CREATING")
			_, redirect := sstPin(t, x.h.f, x.attempt) // disclosed owner pin, as in SP14
			bpState(t, x.view(t), "handoff_state", "READY")
			var wg sync.WaitGroup
			results := make(chan bhResponse, 2)
			errs := make(chan error, 2)
			for range 2 {
				wg.Add(1)
				go func() {
					defer wg.Done()
					r, e := bphConcurrentTake(bphHarness{hpHarness: x.h, http: x.srv})
					results <- r
					errs <- e
				}()
			}
			wg.Wait()
			close(results)
			close(errs)
			for err := range errs {
				if err != nil {
					t.Fatal(err)
				}
			}
			for r := range results {
				fields := bphRaw(t, r, 200, []string{"disposition", "expires_at", "order_id", "redirect_url"})
				bpState(t, fields, "disposition", "REDIRECT")
				if x.str(t, fields, "redirect_url") != redirect {
					t.Fatal("redirect changed")
				}
			}
			var stamp time.Time
			if err := x.h.f.owner.QueryRow(context.Background(), `SELECT first_handed_out_at FROM payments.stripe_sessions WHERE attempt_id=$1`, x.attempt).Scan(&stamp); err != nil || stamp.IsZero() {
				t.Fatal("handoff marker missing", err)
			}
			stable := rm2ScopeSnapshot(t, x.h.psHarness, true)
			bphRaw(t, x.post(t, "/handoff"), 200, []string{"disposition", "expires_at", "order_id", "redirect_url"})
			if rm2ScopeSnapshot(t, x.h.psHarness, true) != stable {
				t.Fatal("repeat handoff changed persisted marker")
			}
			x.h.a.issuer.Close()
			bphFail(t, x.post(t, "/handoff"), 503, "unavailable")
			if rm2ScopeSnapshot(t, x.h.psHarness, true) != stable {
				t.Fatal("early failure erased committed marker")
			}
		})
	}
}

// Exact blocker + still-valid deadline witness; clock expiry happens inside the
// running view call, within the existing 1s lock timeout, as in the SQL v1 test.
func TestStripeViewFinalDatabaseClock(t *testing.T) {
	for _, stage := range []string{"capability", "qualification"} {
		t.Run(stage, func(t *testing.T) {
			p := psSetup(t)
			e := sstNewEnv(t, p.f, pwKeys(t))
			s := e.seed(t, p)
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			name := "rm2-stripe-clock-" + t04Tag()
			pool, err := platform.OpenHostedPool(ctx, withApplicationName(t, hpRole(t, p.f), name))
			if err != nil {
				t.Fatal(err)
			}
			defer pool.Close()
			jobs, err := river.NewClient(riverpgxv5.New(pool), &river.Config{Schema: "river_payment"})
			if err != nil {
				t.Fatal(err)
			}
			svc, err := checkout.NewHostedPaymentService(ctx, pool, jobs, "PROVIDER_MOCK", e.keys, checkout.HostedProviders{Stripe: &e.scfg})
			if err != nil {
				t.Fatal(err)
			}
			holder, err := p.f.owner.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer holder.Rollback(context.Background())
			if _, err = holder.Exec(ctx, `LOCK TABLE checkout.orders IN ACCESS EXCLUSIVE MODE`); err != nil {
				t.Fatal(err)
			}
			var expiry time.Time
			if stage == "capability" {
				err = p.f.owner.QueryRow(ctx, `UPDATE buyer.capability_sessions SET expires_at=clock_timestamp()+interval '450 milliseconds' WHERE id=$1 RETURNING expires_at`, p.cap.Scope.SessionID).Scan(&expiry)
			} else {
				err = qualUpdateScan(ctx, p.f.owner, `UPDATE payments.account_qualifications SET expires_at=clock_timestamp()+interval '450 milliseconds' WHERE id=$1 RETURNING expires_at`, []any{s.qualification}, &expiry)
			}
			if err != nil {
				t.Fatal(err)
			}
			// Snapshot while the relation lock is held must avoid orders SELECT.
			before := sstFactsWithoutOrders(t, p)
			type result struct {
				view checkout.OrderPayment
				err  error
			}
			done := make(chan result, 1)
			go func() { v, e := svc.PaymentView(ctx, p.cap.Token, p.f.storeA1, p.hold.OrderID); done <- result{v, e} }()
			waitForDatabaseLock(t, p.f.owner, name)
			var causal bool
			if err = p.f.owner.QueryRow(ctx, `SELECT clock_timestamp()<$1 AND EXISTS(SELECT 1 FROM pg_stat_activity WHERE application_name=$2 AND state='active' AND query LIKE '%checkout.hosted_payment_view%' AND wait_event_type='Lock' AND $3::int=ANY(pg_blocking_pids(pid)))`, expiry, name, int32(holder.Conn().PgConn().PID())).Scan(&causal); err != nil || !causal {
				t.Fatal("missed exact live-deadline blocker", err)
			}
			mustExec(t, p.f.owner, `SELECT pg_sleep(GREATEST(0,extract(epoch FROM $1::timestamptz-clock_timestamp()))+0.02)`, expiry)
			if err = holder.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			select {
			case got := <-done:
				if stage == "capability" {
					if !errors.Is(got.err, buyer.ErrUnauthorized) || got.view.OrderID != "" || got.view.Methods != nil {
						t.Fatal("expired capability returned view", got.err)
					}
				} else if got.err != nil || len(got.view.Methods) != 0 {
					t.Fatal("expired qualification offered method", got.err)
				}
			case <-ctx.Done():
				t.Fatal("view did not return")
			}
			if sstFactsWithoutOrders(t, p) != before {
				t.Fatal("view changed money/stock/jobs")
			}
		})
	}
}

func sstFactsWithoutOrders(t *testing.T, p psHarness) [5]int64 {
	t.Helper()
	var out [5]int64
	if err := p.f.owner.QueryRow(context.Background(), `SELECT
	 (SELECT count(*) FROM checkout.payment_attempts WHERE tenant_id=$1),
	 (SELECT count(*) FROM integration.operations WHERE tenant_id=$1),
	 (SELECT count(*) FROM inventory.ledger WHERE tenant_id=$1),
	 (SELECT count(*) FROM payments.facts WHERE tenant_id=$1),
	 (SELECT count(*) FROM river_payment.river_job)`, p.f.tenantA).Scan(&out[0], &out[1], &out[2], &out[3], &out[4]); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestStripeWorkerLateDefaultInsertPollAndProfileIsolation(t *testing.T) {
	e := sflNew(t, nil)
	f := e.f
	var unrelated int64
	if err := f.owner.QueryRow(context.Background(), `INSERT INTO river.river_job(kind,args,max_attempts,queue) VALUES('pw_unrelated_v1','{}',2,'default') RETURNING id`).Scan(&unrelated); err != nil {
		t.Fatal(err)
	}
	unrelatedBefore := pwJobExceptQueue(t, f.owner, unrelated)
	s := e.stripeStore(t)
	foreign := e.addPAYUNi(t)
	// Owner prepares a synthetic foreign-profile archive, with the existing
	// immutable family guard restored before the MOCK worker is started.
	mustExec(t, f.owner, `ALTER TABLE river_payment.river_job DISABLE TRIGGER payment_job_family`)
	defer mustExec(t, f.owner, `ALTER TABLE river_payment.river_job ENABLE TRIGGER payment_job_family`)
	mustExec(t, f.owner, `UPDATE checkout.payment_attempts SET execution_profile='SANDBOX' WHERE id=$1`, foreign.result.AttemptID)
	mustExec(t, f.owner, `UPDATE river_payment.river_job SET queue='payment_sandbox_v1',scheduled_at=clock_timestamp() WHERE id=$1`, foreign.result.JobID)
	mustExec(t, f.owner, `ALTER TABLE river_payment.river_job ENABLE TRIGGER payment_job_family`)
	foreignBefore := rm2Snapshot(t, foreign.psHarness, foreign.result.AttemptID)
	stop := e.start(t, true)
	defer stop()
	// A test-only BEFORE INSERT fixture requests the historical default queue.
	// The real family and deferred routing triggers remain enabled throughout.
	name := "a_rm2_default_" + t04Tag()
	mustExec(t, f.owner, `CREATE SEQUENCE public.`+name+`_hits`)
	mustExec(t, f.owner, `GRANT USAGE ON SEQUENCE public.`+name+`_hits TO commerce_hosted_runtime,commerce_payment_worker,commerce_checkout_writer,commerce_integration_writer`)
	mustExec(t, f.owner, `CREATE FUNCTION public.`+name+`() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.kind IN ('payment_query_v1','payment_reconcile_v1') THEN NEW.queue:='default'; PERFORM nextval('public.`+name+`_hits'); END IF; RETURN NEW; END $$`)
	mustExec(t, f.owner, `CREATE TRIGGER `+name+` BEFORE INSERT ON river_payment.river_job FOR EACH ROW EXECUTE FUNCTION public.`+name+`() `)
	t.Cleanup(func() {
		mustExec(t, f.owner, `DROP TRIGGER `+name+` ON river_payment.river_job`)
		mustExec(t, f.owner, `DROP FUNCTION public.`+name+`() `)
		mustExec(t, f.owner, `DROP SEQUENCE public.`+name+`_hits`)
	})
	result, session := e.pinned(t, s)
	if pwQueueIn(t, f.owner, "river_payment.river_job", result.JobID) != "payment_mock_v1" {
		t.Fatal("late default start not routed")
	}
	if !e.fake.SetState(session, "complete", "paid") {
		t.Fatal("missing fake session")
	}
	sstWake(t, f, result.AttemptID)
	e.awaitFact(t, result.AttemptID, "CAPTURED")
	stop()
	var hits int64
	if err := f.owner.QueryRow(context.Background(), `SELECT last_value FROM public.`+name+`_hits`).Scan(&hits); err != nil || hits < 2 {
		t.Fatal("default start/reconcile fixture not reached", err)
	}
	q := pqFixture{psHarness: s.p, result: result, schema: "river_payment"}
	pcAssertStock(t, q, 0, 2, "CONFIRMED", "COMMITTED")
	pcAssertWork(t, q, "READY")
	if rm2Snapshot(t, foreign.psHarness, foreign.result.AttemptID) != foreignBefore || pwJobExceptQueue(t, f.owner, unrelated) != unrelatedBefore || pwQueue(t, f.owner, unrelated) != "default" {
		t.Fatal("MOCK polling changed foreign/default work")
	}
	var attempt int
	var ready bool
	if err := f.owner.QueryRow(context.Background(), `SELECT attempt,integration.payment_queue_ready() FROM river_payment.river_job WHERE id=$1`, foreign.result.JobID).Scan(&attempt, &ready); err != nil || attempt != 0 || !ready || foreign.outbound.Load() != 0 {
		t.Fatal("foreign profile fetched or queue unavailable", err)
	}
}
