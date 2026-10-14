// Purpose: REAL_PG tests-first acceptance of both C2/erasure lock orders.
// Depends on: isolated native C1, real authenticated erasure, Executor and SQL barriers.
// Used by: OA1 CI and isolated acceptance fixtures in this package.
// Invariants: I01/I03/I08/I14; the first attempt fence wins, ERASED has zero
// execution effects, COMMITTED retains commercial proof, replay never writes twice.
package foundation_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"livecommerce/internal/claims"
	"livecommerce/internal/customers"
	"livecommerce/internal/fulfillment"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/orderattempt"
	"livecommerce/internal/platform"
)

// Keep the existing isolated database, runtime login and its actual session.
// Add only native fixture capabilities required by the claims/notification cases;
// settings use the real merchant handler, with no provider API call.
func oaAreaIsolated(t *testing.T) *lbuEnv {
	t.Helper()
	e := oaExpiryIsolated(t)
	e.grantCreator("live:read", "live:manage", "inbox:read", "inbox:reply", "customers:read")
	labels, err := claims.NewLabelKey(randomBytes(32))
	if err != nil {
		t.Fatal(err)
	}
	e.h = &lcHarness{cqHarness: e.p.cqHarness, ctx: context.Background(), actor: e.p.f.principalA, token: e.token(), labels: labels}
	e.svcs, e.svcVer = map[string]fulfillment.ServiceInput{}, map[string]int64{}
	jobs, err := river.NewClient[pgx.Tx](riverpgxv5.New(e.p.f.runtime), &river.Config{Schema: "river"})
	if err != nil {
		t.Fatal(err)
	}
	e.cvs, err = fulfillment.NewCVS(e.p.f.runtime, jobs, nil, nil, fulfillment.CVSConfig{PaymentEnvironment: "SANDBOX"})
	if err != nil {
		t.Fatal("isolated native settings service", err)
	}
	e.merchant = httpapi.NewHandler(e.p.f.runtime, httpapi.Options{CVS: e.cvs, Studio: true, ClaimLabels: &labels})
	e.origin = "https://oa-area-" + strings.ReplaceAll(randomUUID(), "-", "") + ".example"
	bhPublish(t, e.p.bcHarness, e.origin, e.tenant(), e.store())
	return e
}

func oaAreaRuntime(t *testing.T, e *lbuEnv) (*pgxpool.Pool, int32) {
	t.Helper()
	config := e.p.f.runtime.Config().Copy()
	config.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	var pid int32
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err = pool.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
		t.Fatal("actual isolated runtime backend", err)
	}
	return pool, pid
}

// The ticker only polls an observed PostgreSQL wait edge; it never establishes
// ordering. Release immediately once the edge exists, within C2's 1s lock budget.
func oaAreaWaitEdge(t *testing.T, e *lbuEnv, ctx context.Context, waiter, blocker int32, finished <-chan struct{}, outcomes ...<-chan oaExecutorOutcome) {
	t.Helper()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		var blocked bool
		if err := e.p.f.owner.QueryRow(ctx, `SELECT $2::int=ANY(pg_blocking_pids($1::int))`, waiter, blocker).Scan(&blocked); err != nil {
			t.Fatal("exact PostgreSQL blocking edge", err)
		}
		if blocked {
			return
		}
		select {
		case <-finished:
			if len(outcomes) == 1 {
				out := <-outcomes[0]
				code, sqlState := "", ""
				var admission *orderattempt.AdmissionError
				if errors.As(out.err, &admission) {
					code, sqlState = admission.Code(), admission.SQLState()
				}
				var terminal struct {
					Code string `json:"code"`
				}
				_ = json.Unmarshal(out.receipt.Receipt, &terminal)
				t.Fatalf("operation completed before the required attempt-fence wait: error_type=%T code=%q sqlstate=%q state=%q terminal_code=%q", out.err, code, sqlState, out.receipt.State, terminal.Code)
			}
			t.Fatal("operation completed before the required attempt-fence wait")
		case <-ctx.Done():
			t.Fatal("required attempt-fence wait not observed")
		case <-ticker.C:
		}
	}
}

func oaAreaDrain(t *testing.T, done <-chan struct{}) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	select {
	case <-done:
	case <-ctx.Done():
		t.Error("bounded own race operation shutdown")
	}
}

func oaAreaCommercialSnapshot(t *testing.T, e *lbuEnv, attempt string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var raw []byte
	err := e.p.f.owner.QueryRow(ctx, `SELECT jsonb_build_object(
	 'orders',(SELECT jsonb_agg(to_jsonb(x) ORDER BY id) FROM checkout.orders x WHERE create_attempt_id=$1),
	 'carts',(SELECT jsonb_agg(to_jsonb(x) ORDER BY id) FROM storefront.carts x WHERE create_attempt_id=$1),
	 'lines',(SELECT jsonb_agg(to_jsonb(x) ORDER BY line_ordinal) FROM storefront.cart_lines x WHERE create_attempt_id=$1),
	 'quotes',(SELECT jsonb_agg(to_jsonb(x) ORDER BY id) FROM storefront.quotes x WHERE create_attempt_id=$1),
	 'destinations',(SELECT jsonb_agg(to_jsonb(x) ORDER BY id) FROM storefront.destination_snapshots x WHERE create_attempt_id=$1),
	 'reservations',(SELECT jsonb_agg(to_jsonb(x) ORDER BY id) FROM inventory.reservations x WHERE create_attempt_id=$1),
	 'ledger',(SELECT jsonb_agg(to_jsonb(x) ORDER BY id) FROM inventory.ledger x WHERE checkout_id=$1),
	 'begin',(SELECT jsonb_agg(to_jsonb(x) ORDER BY order_id) FROM checkout.command_results x WHERE create_attempt_id=$1),
	 'pay',(SELECT jsonb_agg(to_jsonb(x) ORDER BY order_id) FROM checkout.order_pay_entries x WHERE create_attempt_id=$1),
	 'expiry',(SELECT jsonb_agg(to_jsonb(x) ORDER BY id) FROM river_expiry.river_job x WHERE args->>'order_id'=($1::uuid)::text),
	 'external',(SELECT jsonb_agg(to_jsonb(x) ORDER BY id) FROM river.river_job x WHERE kind='external_operation_v1'))`, attempt).Scan(&raw)
	if err != nil {
		t.Fatal("exact attempt-owned commercial proof snapshot", err)
	}
	return raw
}

func TestOrderAttemptC2FenceBeforeErasureRetainsCommercialProof(t *testing.T) {
	e := oaAreaIsolated(t)
	e.grantCreator("customers:privacy")
	a := oaAllocationAdmit(t, e, oaBeginSelection(t, e, "cash_on_delivery", "duplicate-origins"))
	plan, _ := oaMaterialPlan(t, e, oaServiceKeys(t), a.AttemptID)
	var indexed bool
	if err := e.p.f.owner.QueryRow(context.Background(), `SELECT state='PENDING' AND plan_ciphertext IS NOT NULL
	 AND EXISTS(SELECT 1 FROM ops.order_attempt_subjects s WHERE s.attempt_id=a.attempt_id)
	 FROM ops.order_attempts a WHERE a.attempt_id=$1`, a.AttemptID).Scan(&indexed); err != nil || !indexed {
		t.Fatal("actual C1 ciphertext and indexed erasure subject", err)
	}
	pool, executorPID := oaAreaRuntime(t, e)
	executor := oaExecutorNew(t, pool, oaServiceKeys(t))
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	holder, err := e.p.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	key := "oa.area.c2|" + randomUUID()
	if _, err = holder.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, key); err != nil {
		_ = holder.Rollback(ctx)
		t.Fatal(err)
	}
	defer func() {
		clean, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		_ = holder.Rollback(clean)
	}()
	// AFTER pay-entry INSERT is after the real Begin, while the same C2 fence
	// remains held. The fixture trigger proves the uncommitted native prefix.
	witness := `EXISTS(SELECT 1 FROM ops.order_attempt_execution_fences WHERE attempt_id=ATTEMPT AND execution_xid=pg_current_xact_id())
	 AND EXISTS(SELECT 1 FROM checkout.orders o JOIN inventory.reservations r ON r.id=o.id
	 JOIN checkout.command_results c ON c.order_id=o.id JOIN river_expiry.river_job j ON j.id=o.job_id
	 WHERE o.create_attempt_id=ATTEMPT AND o.commercial_state='AWAITING_COLLECTION' AND r.state='COMMITTED'
	 AND c.create_attempt_id=ATTEMPT AND j.kind='checkout_expiry_v1')
	 AND EXISTS(SELECT 1 FROM checkout.order_pay_entries WHERE create_attempt_id=ATTEMPT AND link_version=1)`
	oaCrashInstallBarrier(t, oaCrashScenario{e: e, a: a}, oaCrashBarrier{"c2-before-erasure", "checkout.order_pay_entries", "AFTER", "INSERT", "NEW.create_attempt_id=ATTEMPT", witness}, key)
	executed, erased := make(chan oaExecutorOutcome, 1), make(chan error, 1)
	executorDone, eraseDone := make(chan struct{}), make(chan struct{})
	go func() {
		out, callErr := executor.Execute(ctx, e.token(), e.store(), a.AttemptID, a.Generation)
		executed <- oaExecutorOutcome{out, callErr}
		close(executorDone)
	}()
	defer func() {
		cancel()
		clean, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		_ = holder.Rollback(clean)
		oaAreaDrain(t, executorDone)
	}()
	oaAreaWaitEdge(t, e, ctx, executorPID, int32(holder.Conn().PgConn().PID()), executorDone, executed)
	pid := make(chan uint32, 1)
	go func() {
		erased <- oaCEErase(ctx, e, plan.BuyerOwnerID, "oa-area-erase-"+randomUUID(), pid)
		close(eraseDone)
	}()
	defer func() { cancel(); oaAreaDrain(t, eraseDone) }()
	var erasePID uint32
	select {
	case erasePID = <-pid:
	case err = <-erased:
		t.Fatal("native authenticated erasure did not reach its transaction", err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	oaAreaWaitEdge(t, e, ctx, int32(erasePID), executorPID, eraseDone)
	if err = holder.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	first := <-executed
	if first.err != nil {
		t.Fatal("fence winner C2", first.err)
	}
	oaExecutorReceipt(t, first.receipt, a, "COMMITTED", "")
	if err = <-erased; err != nil {
		t.Fatal("erasure after native COD commit", err)
	}
	var exact bool
	err = e.p.f.owner.QueryRow(ctx, `SELECT NOT b.active AND a.state='COMMITTED' AND a.receipt=$2::jsonb
	 AND a.plan_ciphertext IS NULL AND a.plan_key_version IS NULL AND a.settled_at IS NOT NULL
	 AND o.id=a.attempt_id AND o.create_attempt_id=a.attempt_id AND o.owner_id=$3 AND o.commercial_state='AWAITING_COLLECTION'
	 AND o.total_minor=$4 AND o.currency=a.receipt->>'currency' AND r.state='COMMITTED'
	 AND NOT EXISTS(SELECT 1 FROM ops.order_attempt_subjects s WHERE s.attempt_id=a.attempt_id)
	 AND (SELECT sum(delta_allocated) FROM inventory.ledger WHERE checkout_id=a.attempt_id)=5
	 AND (SELECT count(*) FROM checkout.orders WHERE create_attempt_id=a.attempt_id)=1
	 AND (SELECT count(*) FROM storefront.carts WHERE create_attempt_id=a.attempt_id)=1
	 AND (SELECT count(*) FROM storefront.quotes WHERE create_attempt_id=a.attempt_id)=1
	 AND (SELECT count(*) FROM storefront.destination_snapshots WHERE create_attempt_id=a.attempt_id)=1
	 AND (SELECT count(*) FROM inventory.reservations WHERE create_attempt_id=a.attempt_id)=1
	 AND (SELECT count(*) FROM checkout.command_results WHERE create_attempt_id=a.attempt_id)=1
	 AND (SELECT count(*) FROM checkout.order_pay_entries WHERE create_attempt_id=a.attempt_id)=1
	 AND (SELECT count(*) FROM river_expiry.river_job WHERE args->>'order_id'=a.attempt_id::text)=1
	 AND (SELECT count(*) FROM ops.audit_events WHERE tenant_id=a.tenant_id AND store_id=a.store_id AND details->>'attempt_id'=a.attempt_id::text)=1
	 AND NOT EXISTS(SELECT 1 FROM river.river_job WHERE kind='external_operation_v1')
	 FROM ops.order_attempts a JOIN checkout.orders o ON o.id=a.order_id
	 JOIN inventory.reservations r ON r.id=o.id JOIN buyer.owners b ON b.id=o.owner_id
	 WHERE a.attempt_id=$1`, a.AttemptID, string(first.receipt.Receipt), plan.BuyerOwnerID, plan.Calculation.TotalMinor).Scan(&exact)
	if err != nil || !exact {
		t.Fatalf("retained exact native proof after raced erasure: exact=%v err=%v", exact, err)
	}
	before := oaCEAttemptBytes(t, e, a)
	commercial := oaAreaCommercialSnapshot(t, e, a.AttemptID)
	again, err := executor.Execute(ctx, e.token(), e.store(), a.AttemptID, a.Generation)
	if err != nil {
		t.Fatal(err)
	}
	oaExecutorSameReceipt(t, first.receipt, again)
	if !bytes.Equal(before, oaCEAttemptBytes(t, e, a)) {
		t.Fatal("raced erasure terminal replay changed retained attempt proof")
	}
	if !bytes.Equal(commercial, oaAreaCommercialSnapshot(t, e, a.AttemptID)) {
		t.Fatal("terminal replay changed native commercial effects or minted an orphan job")
	}
}

func TestOrderAttemptErasureBeforeC2FenceAbortsAndAllowsNewCAS(t *testing.T) {
	for _, linked := range []bool{false, true} {
		t.Run(map[bool]string{false: "private-owner", true: "linked-customer"}[linked], func(t *testing.T) {
			e := oaAreaIsolated(t)
			a, plan := oaCEPending(t, e, linked)
			pool, executorPID := oaAreaRuntime(t, e)
			executor := oaExecutorNew(t, pool, oaServiceKeys(t))
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			release := make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			ready, erased := make(chan uint32, 1), make(chan error, 1)
			eraseDone, executorDone := make(chan struct{}), make(chan struct{})
			go func() {
				erased <- platform.WithScope(ctx, e.p.f.runtime, e.token(), e.store(), "store:read", func(tx pgx.Tx, s platform.Scope) error {
					if _, err := customers.Erase(ctx, tx, s, e.token(), "oa-area-first-"+randomUUID(), plan.BuyerOwnerID); err != nil {
						return err
					}
					// Runtime cannot SELECT ops private rows. Native erasure success
					// and the exact C2 wait below establish its held attempt prefix;
					// owner readback verifies the durable state after actual commit.
					ready <- tx.Conn().PgConn().PID()
					select {
					case <-release:
						return nil
					case <-ctx.Done():
						return ctx.Err()
					}
				})
				close(eraseDone)
			}()
			defer func() { unblock(); cancel(); oaAreaDrain(t, eraseDone) }()
			var erasePID uint32
			select {
			case erasePID = <-ready:
			case err := <-erased:
				t.Fatal("native erasure/sealed precommit sentinel", err)
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			executed := make(chan oaExecutorOutcome, 1)
			go func() {
				out, err := executor.Execute(ctx, e.token(), e.store(), a.AttemptID, a.Generation)
				executed <- oaExecutorOutcome{out, err}
				close(executorDone)
			}()
			defer func() { unblock(); cancel(); oaAreaDrain(t, executorDone) }()
			oaAreaWaitEdge(t, e, ctx, executorPID, int32(erasePID), executorDone)
			unblock()
			if err := <-erased; err != nil {
				t.Fatal("erasure fence winner commit", err)
			}
			got := <-executed
			if got.err != nil {
				t.Fatal("C2 must replay committed ERASED without executing", got.err)
			}
			oaExecutorReceipt(t, got.receipt, a, "ABORTED", "ERASED")
			oaCEErased(t, e, a, plan) // exact zero effects, body/index shred, replay and explicit next CAS
		})
	}
}
