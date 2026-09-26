package foundation_test

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"livecommerce/internal/integrations/accounts"
	"livecommerce/internal/payments"
	"livecommerce/internal/platform"
)

// Public protocol shape, independently declared from the worker's private args.
type pcArgs struct {
	OperationID string `json:"operation_id"`
	ReportHash  string `json:"report_hash"`
	Version     int    `json:"version"`
}

func (pcArgs) Kind() string { return "payment_reconcile_v1" }
func pcFull(q pqFixture) map[string]any {
	r := pqReport(q)
	r["CloseStatus"], r["CloseAmountTWD"] = "2", q.result.AmountMinor/100
	return r
}
func pcHash(t *testing.T, q pqFixture, report any) []byte {
	t.Helper()
	var hash []byte
	if e := q.f.owner.QueryRow(context.Background(), `SELECT sha256(convert_to($1::jsonb::text,'UTF8'))`, pqJSON(report)).Scan(&hash); e != nil {
		t.Fatal(e)
	}
	return hash
}
func pcRecord(t *testing.T, q pqFixture, report any) []byte {
	t.Helper()
	if e := q.record(q.claim(t), report); e != nil {
		t.Fatal(e)
	}
	return pcHash(t, q, report)
}
func pcApply(pool *pgxpool.Pool, attempt string, hash []byte) error {
	_, e := pool.Exec(context.Background(), `SELECT payments.apply_capture($1::uuid,$2::bytea)`, attempt, hash)
	return e
}
func pcCount(t *testing.T, q pqFixture, table, predicate string) int {
	t.Helper()
	var n int
	// Table/predicate are fixed test literals, never external request fields.
	if e := q.f.owner.QueryRow(context.Background(), `SELECT count(*) FROM `+table+` WHERE attempt_id=$1`+predicate, q.result.AttemptID).Scan(&n); e != nil {
		t.Fatal(e)
	}
	return n
}
func pcAssertStock(t *testing.T, q pqFixture, reserved, allocated int64, order, reservation string) {
	t.Helper()
	var actualOrder, actualReservation string
	var r, a, onHand int64
	e := q.f.owner.QueryRow(context.Background(), `SELECT o.commercial_state,r.state,b.reserved,b.allocated,b.on_hand FROM checkout.orders o JOIN inventory.reservations r ON r.id=o.id JOIN inventory.reservation_lines l ON l.reservation_id=r.id JOIN inventory.balances b ON (b.tenant_id,b.store_id,b.warehouse_id,b.sku_id)=(l.tenant_id,l.store_id,l.warehouse_id,l.sku_id) WHERE o.id=$1`, q.hold.OrderID).Scan(&actualOrder, &actualReservation, &r, &a, &onHand)
	if e != nil || actualOrder != order || actualReservation != reservation || r != reserved || a != allocated || onHand != 10 {
		t.Fatalf("stock/order: %s/%s %d/%d/%d error=%v", actualOrder, actualReservation, r, a, onHand, e)
	}
}
func pcAssertWork(t *testing.T, q pqFixture, state string) {
	t.Helper()
	var actual string
	if e := q.f.owner.QueryRow(context.Background(), `SELECT state FROM fulfillment.payment_work_items WHERE attempt_id=$1`, q.result.AttemptID).Scan(&actual); e != nil || actual != state {
		t.Fatalf("work state %s want %s: %v", actual, state, e)
	}
}

func TestBuyerPaymentCaptureAtomicReplayAndFreshSession(t *testing.T) {
	q := pqSetupSession(t, true)
	hash := pcRecord(t, q, pcFull(q))
	if pcCount(t, q, "payments.facts", "") != 0 {
		t.Fatal("query persistence changed financial facts before local reconciliation")
	}
	var wg sync.WaitGroup
	errs := make(chan error, 6)
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- pcApply(q.worker, q.result.AttemptID, hash) }()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	pcAssertStock(t, q, 0, 2, "CONFIRMED", "COMMITTED")
	pcAssertWork(t, q, "READY")
	if pcCount(t, q, "payments.facts", "") != 2 || pcCount(t, q, "payments.facts", " AND kind='CAPTURED'") != 1 || pcCount(t, q, "fulfillment.payment_work_items", "") != 1 {
		t.Fatal("duplicate or missing financial facts/work")
	}
	var n int
	var original, paid, ledger string
	e := q.f.owner.QueryRow(context.Background(), `SELECT o.creator_session_id::text,a.session_id::text,l.buyer_session_id::text,(SELECT count(*) FROM inventory.ledger WHERE checkout_id=o.id AND kind='ALLOCATE') FROM checkout.orders o JOIN checkout.payment_attempts a ON a.order_id=o.id JOIN inventory.ledger l ON l.checkout_id=o.id AND l.kind='ALLOCATE' WHERE o.id=$1`, q.hold.OrderID).Scan(&original, &paid, &ledger, &n)
	if e != nil || original == paid || ledger != original || n != 1 {
		t.Fatalf("original checkout vs payment session provenance: %s %s %s %d %v", original, paid, ledger, n, e)
	}
	var unchanged bool
	if e = q.f.owner.QueryRow(context.Background(), `SELECT state='PAYMENT_PENDING' FROM checkout.payment_attempts WHERE id=$1`, q.result.AttemptID).Scan(&unchanged); e != nil || !unchanged {
		t.Fatal("immutable initiation record overwritten")
	}
}

func TestBuyerPaymentCaptureEvidenceAndStickyReview(t *testing.T) {
	for _, mode := range []string{"authorized", "incomplete", "zero", "partial", "missing", "wrong_trade_status", "refund_before", "refund_with_capture", "neutral_refund_fields", "remain_above_total"} {
		t.Run(mode, func(t *testing.T) {
			q := pqSetup(t)
			r := pcFull(q)
			wantCapture, wantReview, wantReady := 0, 0, false
			switch mode {
			case "authorized":
				r["CloseStatus"] = "9"
				delete(r, "CloseAmountTWD")
			case "incomplete":
				r["DataSource"] = "B"
			case "zero":
				r["CloseAmountTWD"], wantReview = int64(0), 1
			case "partial":
				r["CloseAmountTWD"], wantReview = q.result.AmountMinor/100-1, 1
			case "missing":
				delete(r, "CloseAmountTWD")
				wantReview = 1
			case "wrong_trade_status":
				r["TradeStatus"] = "8"
				wantReview = 1 // Claimed full close with a non-paid trade is conflicting evidence.
			case "refund_before":
				r["DataSource"], r["CardRefundStatus"], wantReview = "B", "2", 1
			case "refund_with_capture":
				r["CardRefundStatus"], r["CardRefundAmountTWD"] = "2", int64(1)
				wantCapture, wantReview = 1, 1
			case "neutral_refund_fields":
				r["CardRefundAmountTWD"], r["CardRemainAmountTWD"] = int64(0), q.result.AmountMinor/100
				wantCapture, wantReady = 1, true
			case "remain_above_total":
				r["CardRemainAmountTWD"] = q.result.AmountMinor/100 + 1
				wantCapture, wantReview = 1, 1
			}
			h := pcRecord(t, q, r)
			if e := pcApply(q.worker, q.result.AttemptID, h); e != nil {
				t.Fatal(e)
			}
			if got := pcCount(t, q, "payments.facts", " AND kind='CAPTURED'"); got != wantCapture {
				t.Fatalf("capture count %d want%d", got, wantCapture)
			}
			if got := pcCount(t, q, "payments.review_cases", ""); (got > 0) != (wantReview > 0) {
				t.Fatalf("review count %d want positive=%v", got, wantReview > 0)
			}
			if wantReady {
				pcAssertStock(t, q, 0, 2, "CONFIRMED", "COMMITTED")
				pcAssertWork(t, q, "READY")
			} else {
				q.pending(t)
				if wantCapture > 0 {
					pcAssertWork(t, q, "REVIEW_REQUIRED")
				}
			}
			// A later clean full observation may record the gross fact, but cannot
			// silently clear review created by an earlier incomplete/refund report.
			if wantReview > 0 {
				h = pcRecord(t, q, pcFull(q))
				if e := pcApply(q.worker, q.result.AttemptID, h); e != nil {
					t.Fatal(e)
				}
				q.pending(t)
				pcAssertWork(t, q, "REVIEW_REQUIRED")
			}
		})
	}
}

func TestBuyerPaymentCaptureLaterRefundAndReverseOrder(t *testing.T) {
	q := pqSetup(t)
	full := pcRecord(t, q, pcFull(q))
	if e := pcApply(q.worker, q.result.AttemptID, full); e != nil {
		t.Fatal(e)
	}
	r := pcFull(q)
	r["DataSource"], r["CardRefundType"], r["CardRefundStatus"] = "B", "2", "8"
	h := pcRecord(t, q, r)
	if e := pcApply(q.worker, q.result.AttemptID, h); e != nil {
		t.Fatal(e)
	}
	if e := pcApply(q.worker, q.result.AttemptID, full); e != nil {
		t.Fatal(e)
	}
	pcAssertWork(t, q, "REVIEW_REQUIRED")
	pcAssertStock(t, q, 0, 2, "CONFIRMED", "COMMITTED")
	if pcCount(t, q, "payments.facts", " AND kind='CAPTURED'") != 1 {
		t.Fatal("refund signal erased or duplicated gross capture")
	}
}

func TestBuyerPaymentCaptureOldAuthorizationIsNotConflict(t *testing.T) {
	q := pqSetup(t)
	oldAuth := pcRecord(t, q, pqReport(q))
	full := pcRecord(t, q, pcFull(q))
	if e := pcApply(q.worker, q.result.AttemptID, full); e != nil {
		t.Fatal(e)
	}
	if e := pcApply(q.worker, q.result.AttemptID, oldAuth); e != nil {
		t.Fatal(e)
	}
	pcAssertWork(t, q, "READY")
	pcAssertStock(t, q, 0, 2, "CONFIRMED", "COMMITTED")
	if pcCount(t, q, "payments.review_cases", "") != 0 {
		t.Fatal("older authorization was mistaken for a contradictory capture")
	}
	// An explicit cancellation is different from absence of newer-stage proof.
	r := pqReport(q)
	r["TradeStatus"], r["CloseStatus"] = "3", "3"
	cancelHash := pcRecord(t, q, r)
	if e := pcApply(q.worker, q.result.AttemptID, cancelHash); e != nil {
		t.Fatal(e)
	}
	pcAssertWork(t, q, "REVIEW_REQUIRED")
	pcAssertStock(t, q, 0, 2, "CONFIRMED", "COMMITTED")
}

func TestBuyerPaymentCaptureACLAndObservationBinding(t *testing.T) {
	q, other := pqSetup(t), pqSetup(t)
	hash := pcRecord(t, q, pcFull(q))
	for _, wrong := range [][]byte{nil, randomBytes(31), randomBytes(32)} {
		if e := pcApply(q.worker, q.result.AttemptID, wrong); e == nil {
			t.Fatal("missing observation accepted")
		}
	}
	if e := pcApply(q.worker, other.result.AttemptID, hash); e == nil {
		t.Fatal("cross-attempt observation accepted")
	}
	for _, role := range []string{"commerce_runtime", "commerce_checkout_runtime", "commerce_worker"} {
		var execute bool
		if e := q.f.owner.QueryRow(context.Background(), `SELECT has_function_privilege($1,'payments.apply_capture(uuid,bytea)','EXECUTE')`, role).Scan(&execute); e != nil || execute != (role == "commerce_worker") {
			t.Fatalf("execute boundary %s %v %v", role, execute, e)
		}
		for _, table := range []string{"payments.facts", "payments.review_cases", "fulfillment.payment_work_items", "inventory.ledger"} {
			if role == "commerce_runtime" && table == "inventory.ledger" {
				continue // Existing merchant ADJUST authority is separately actor-fenced.
			}
			var write bool
			if e := q.f.owner.QueryRow(context.Background(), `SELECT has_table_privilege($1,$2,'INSERT,UPDATE,DELETE')`, role, table).Scan(&write); e != nil || write {
				t.Fatalf("direct business write %s %s %v %v", role, table, write, e)
			}
		}
	}
	if e := pcApply(q.f.runtime, q.result.AttemptID, hash); sqlState(e) != "42501" {
		t.Fatalf("merchant direct apply not denied %v", e)
	}
	// Historical matching evidence remains usable after current credentials,
	// qualification and enablement change; they cannot erase actual money.
	q.rotate(t, 2, accounts.Credentials{HashKey: strings.Repeat("N", 32), HashIV: strings.Repeat("I", 16)})
	mustExec(t, q.f.owner, `UPDATE integration.bindings SET enabled=false,semantic_version=semantic_version+1 WHERE id=$1`, q.binding)
	if e := pcApply(q.worker, q.result.AttemptID, hash); e != nil {
		t.Fatal(e)
	}
	pcAssertWork(t, q, "READY")
}

func TestBuyerPaymentCaptureFaultsAreCausalAndAtomic(t *testing.T) {
	for _, point := range []struct{ table, event, predicate string }{
		{"payments.facts", "INSERT", "NEW.attempt_id=$ATTEMPT AND NEW.kind='CAPTURED'"},
		{"inventory.ledger", "INSERT", "NEW.checkout_id=$ORDER AND NEW.kind='ALLOCATE'"},
		{"inventory.reservations", "UPDATE", "NEW.id=$ORDER AND NEW.state='COMMITTED'"},
		{"checkout.orders", "UPDATE", "NEW.id=$ORDER AND NEW.commercial_state='CONFIRMED'"},
		{"checkout.events", "INSERT", "NEW.order_id=$ORDER AND NEW.action='checkout.payment_captured'"},
		{"fulfillment.payment_work_items", "INSERT", "NEW.order_id=$ORDER"},
	} {
		t.Run(point.table, func(t *testing.T) {
			q := pqSetup(t)
			hash := pcRecord(t, q, pcFull(q))
			name := "pc_fault_" + t04Tag()
			predicate := strings.ReplaceAll(strings.ReplaceAll(point.predicate, "$ORDER", "'"+q.hold.OrderID+"'::uuid"), "$ATTEMPT", "'"+q.result.AttemptID+"'::uuid")
			mustExec(t, q.f.owner, fmt.Sprintf(`CREATE FUNCTION ops.%s() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF %s THEN RAISE EXCEPTION 'injected capture failure' USING ERRCODE='PCT01'; END IF; RETURN NEW; END $$`, name, predicate))
			mustExec(t, q.f.owner, fmt.Sprintf(`CREATE TRIGGER %s AFTER %s ON %s FOR EACH ROW EXECUTE FUNCTION ops.%s()`, name, point.event, point.table, name))
			e := pcApply(q.worker, q.result.AttemptID, hash)
			// Fail if an unrelated ACL/parser error prevented reaching the trigger.
			if sqlState(e) != "PCT01" {
				t.Fatalf("did not reach intended %s injection: %v", point.table, e)
			}
			q.pending(t)
			if pcCount(t, q, "payments.facts", "") != 0 || pcCount(t, q, "fulfillment.payment_work_items", "") != 0 || q.reportCount(t) != 1 {
				t.Fatal("local capture partially committed or durable query evidence lost")
			}
			mustExec(t, q.f.owner, fmt.Sprintf(`DROP TRIGGER %s ON %s`, name, point.table))
			mustExec(t, q.f.owner, fmt.Sprintf(`DROP FUNCTION ops.%s()`, name))
			if e = pcApply(q.worker, q.result.AttemptID, hash); e != nil {
				t.Fatal(e)
			}
			pcAssertWork(t, q, "READY")
		})
	}
}

func TestBuyerPaymentCaptureRealRiverSignedQueryChain(t *testing.T) {
	q := pqSetupWorker(t)
	ctx := context.Background()
	opts := payments.DefaultQueryWorkerOptions()
	body := pqSignedResponse(pcFull(q))
	opts.MockTransport = pqTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "sandbox-api.payuni.com.tw" || r.URL.Path != "/api/trade/query" {
			return nil, errors.New("unexpected query target")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	// First run only the real query worker. No capture worker is online yet:
	// the crash/restart gap must have persistent reconciliation work waiting.
	pqStartWorker(t, q, opts)
	pqAwait(t, q, q.result.JobID, "payment_report_observed", "scheduled")
	q.pending(t)
	var jobID int64
	var jobHash string
	if e := q.f.owner.QueryRow(ctx, `SELECT id,args->>'report_hash' FROM river_payment.river_job WHERE kind='payment_reconcile_v1' AND args->>'operation_id'=$1`, q.result.OperationID).Scan(&jobID, &jobHash); e != nil {
		t.Fatal(e)
	}
	if jobHash != hex.EncodeToString(pcHash(t, q, pcFull(q))) {
		t.Fatal("durable job not bound to canonical persisted projection")
	}
	w, e := payments.NewCaptureWorker(ctx, q.worker)
	if e != nil {
		t.Fatal(e)
	}
	workers := river.NewWorkers()
	river.AddWorker(workers, w)
	queue := "payment_mock_v1"
	client, e := river.NewClient(riverpgxv5.New(q.worker), &river.Config{Schema: "river_payment", Workers: workers, Queues: map[string]river.QueueConfig{queue: {MaxWorkers: 2}}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), JobTimeout: 15 * time.Second, RescueStuckJobsAfter: 30 * time.Second})
	if e != nil {
		t.Fatal(e)
	}
	if e = client.Start(ctx); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if e := client.StopAndCancel(c); e != nil {
			t.Error(e)
		}
	})
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		var state string
		if e = q.f.owner.QueryRow(ctx, `SELECT state FROM river_payment.river_job WHERE id=$1`, jobID).Scan(&state); e != nil {
			t.Fatal(e)
		}
		if state == "completed" {
			pcAssertStock(t, q, 0, 2, "CONFIRMED", "COMMITTED")
			pcAssertWork(t, q, "READY")
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("capture worker did not commit durable work")
}

func TestBuyerPaymentCaptureReleasedAnomalyKeepsRealMoney(t *testing.T) {
	for _, state := range []string{"EXPIRED", "RELEASED"} {
		t.Run(state, func(t *testing.T) {
			q := pqSetup(t)
			hash := pcRecord(t, q, pcFull(q))
			ctx := context.Background()
			// Owner-only anomaly injection in a disposable PG. There is currently
			// no production PAYMENT_PENDING expiry/release endpoint to pretend to use.
			tx, e := q.f.owner.Begin(ctx)
			if e != nil {
				t.Fatal(e)
			}
			defer tx.Rollback(ctx)
			_, e = tx.Exec(ctx, `SELECT set_config('app.tenant_id',$1,true),set_config('app.store_id',$2,true),set_config('app.principal_id','',true),set_config('app.buyer_id',$3,true),set_config('app.buyer_session_id',$4,true)`, q.f.tenantA, q.f.storeA1, q.cap.Scope.OwnerID, q.cap.Scope.SessionID)
			if e != nil {
				t.Fatal(e)
			}
			_, e = tx.Exec(ctx, `INSERT INTO inventory.ledger(tenant_id,store_id,warehouse_id,sku_id,kind,delta_reserved,operation,command_key,reservation_id,principal_id,checkout_id,buyer_owner_id,buyer_session_id,actor_kind) SELECT l.tenant_id,l.store_id,l.warehouse_id,l.sku_id,'RELEASE',-l.quantity,'test.capture.release',$1,r.id,NULL,r.id,r.buyer_owner_id,r.buyer_session_id,'SYSTEM_EXPIRY' FROM inventory.reservations r JOIN inventory.reservation_lines l ON (l.tenant_id,l.store_id,l.reservation_id)=(r.tenant_id,r.store_id,r.id) WHERE r.id=$2`, q.result.AttemptID, q.hold.OrderID)
			if e != nil {
				t.Fatal(e)
			}
			if _, e = tx.Exec(ctx, `UPDATE inventory.reservations SET state=$1 WHERE id=$2`, state, q.hold.OrderID); e != nil {
				t.Fatal(e)
			}
			if _, e = tx.Exec(ctx, `UPDATE checkout.orders SET commercial_state='CANCELLED',fulfillment_state='CANCELLED' WHERE id=$1`, q.hold.OrderID); e != nil {
				t.Fatal(e)
			}
			if e = tx.Commit(ctx); e != nil {
				t.Fatal(e)
			}
			if e = pcApply(q.worker, q.result.AttemptID, hash); e != nil {
				t.Fatal(e)
			}
			pcAssertStock(t, q, 0, 0, "CANCELLED", state)
			pcAssertWork(t, q, "REVIEW_REQUIRED")
			if pcCount(t, q, "payments.facts", " AND kind='CAPTURED'") != 1 || pcCount(t, q, "payments.review_cases", " AND reason='PAID_ALLOCATION_FAILED'") != 1 {
				t.Fatal("actual money/compensation was lost")
			}
			var fulfillment string
			if e = q.f.owner.QueryRow(ctx, `SELECT fulfillment_state FROM checkout.orders WHERE id=$1`, q.hold.OrderID).Scan(&fulfillment); e != nil || fulfillment != "PAID_ALLOCATION_FAILED" {
				t.Fatalf("not visibly held: %s %v", fulfillment, e)
			}
		})
	}
}

func TestBuyerPaymentCaptureBalanceWaitIsAtomic(t *testing.T) {
	q := pqSetup(t)
	hash := pcRecord(t, q, pcFull(q))
	ctx := context.Background()
	name := "pc_wait_" + t04Tag()
	pool, e := platform.OpenWorkerPool(ctx, withApplicationName(t, bcRole(t, q.f, "commerce_worker"), name))
	if e != nil {
		t.Fatal(e)
	}
	defer pool.Close()
	holder, e := q.f.owner.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer holder.Rollback(ctx)
	if _, e = holder.Exec(ctx, `SELECT b.* FROM inventory.balances b JOIN inventory.reservation_lines l ON (b.tenant_id,b.store_id,b.warehouse_id,b.sku_id)=(l.tenant_id,l.store_id,l.warehouse_id,l.sku_id) WHERE l.reservation_id=$1 FOR UPDATE OF b`, q.hold.OrderID); e != nil {
		t.Fatal(e)
	}
	done := make(chan error, 1)
	go func() { done <- pcApply(pool, q.result.AttemptID, hash) }()
	waitForDatabaseLock(t, q.f.owner, name)
	q.pending(t)
	if pcCount(t, q, "payments.facts", "") != 0 || pcCount(t, q, "fulfillment.payment_work_items", "") != 0 {
		t.Fatal("uncommitted capture became visible while stock was locked")
	}
	if e = holder.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	if e = waitError(t, done); e != nil {
		t.Fatal(e)
	}
	pcAssertStock(t, q, 0, 2, "CONFIRMED", "COMMITTED")
	pcAssertWork(t, q, "READY")
}

func TestBuyerPaymentCaptureEmptyReferenceAndMerchantReadScope(t *testing.T) {
	q := pqSetup(t)
	r := pqReport(q)
	r["TradeNo"], r["TradeStatus"], r["DataSource"] = "", "8", "B"
	pendingHash := pcRecord(t, q, r)
	if e := pcApply(q.worker, q.result.AttemptID, pendingHash); e != nil {
		t.Fatal(e)
	}
	if pcCount(t, q, "payments.facts", "") != 0 {
		t.Fatal("empty trade reference produced financial fact")
	}
	fullHash := pcRecord(t, q, pcFull(q))
	if e := pcApply(q.worker, q.result.AttemptID, fullHash); e != nil {
		t.Fatal(e)
	}
	// The older observation remains a valid no-op even after the provider ref pins.
	if e := pcApply(q.worker, q.result.AttemptID, pendingHash); e != nil {
		t.Fatal(e)
	}
	pcAssertWork(t, q, "READY")
	other := pqSetup(t)
	ctx := context.Background()
	for _, sc := range []struct {
		q    pqFixture
		want int
	}{{q, 1}, {other, 0}} {
		e := platform.WithScope(ctx, sc.q.f.runtime, sc.q.f.tokens["a"], sc.q.f.storeA1, "integration:read", func(tx pgx.Tx, s platform.Scope) error {
			var count int
			if e := tx.QueryRow(ctx, `SELECT count(*) FROM fulfillment.payment_work_items WHERE attempt_id=$1`, q.result.AttemptID).Scan(&count); e != nil {
				return e
			}
			if count != sc.want {
				return fmt.Errorf("merchant work read scope got%d want%d", count, sc.want)
			}
			return nil
		})
		if e != nil {
			t.Fatal(e)
		}
	}
}

func TestBuyerPaymentCaptureMultiSKUSingleCommit(t *testing.T) {
	q := pqSetupItems(t, false, 2)
	hash := pcRecord(t, q, pcFull(q))
	for i := 0; i < 2; i++ {
		if e := pcApply(q.worker, q.result.AttemptID, hash); e != nil {
			t.Fatal(e)
		}
	}
	var lines, good int
	var debit, credit int64
	e := q.f.owner.QueryRow(context.Background(), `SELECT count(*),coalesce(sum(-l.delta_reserved),0),coalesce(sum(l.delta_allocated),0),count(*) FILTER(WHERE b.reserved=0 AND b.allocated=2 AND b.on_hand=10) FROM inventory.ledger l JOIN inventory.balances b USING(tenant_id,store_id,warehouse_id,sku_id) WHERE l.checkout_id=$1 AND l.kind='ALLOCATE'`, q.hold.OrderID).Scan(&lines, &debit, &credit, &good)
	if e != nil || lines != 2 || good != 2 || debit != 4 || credit != 4 {
		t.Fatalf("multi SKU conservation %d/%d %d/%d %v", lines, good, debit, credit, e)
	}
	pcAssertWork(t, q, "READY")
}

func TestBuyerPaymentCaptureQueryJobBindingAndRollback(t *testing.T) {
	q := pqSetup(t)
	c := q.claim(t)
	r := pcFull(q)
	hash := hex.EncodeToString(pcHash(t, q, r))
	ctx := context.Background()
	jobs, e := river.NewClient(riverpgxv5.New(q.worker), &river.Config{Schema: "river_payment"})
	if e != nil {
		t.Fatal(e)
	}
	for _, mode := range []string{"wrong_hash", "wrong_attempt", "wrong_version", "terminal_job"} {
		t.Run(mode, func(t *testing.T) {
			// Isolate the worker-side binding fence from the newer immutable
			// INSERT guard using only this disposable fixture owner.
			mustExec(t, q.f.owner, `ALTER TABLE river_payment.river_job DISABLE TRIGGER payment_job_family`)
			defer mustExec(t, q.f.owner, `ALTER TABLE river_payment.river_job ENABLE TRIGGER payment_job_family`)
			tx, e := q.worker.Begin(ctx)
			if e != nil {
				t.Fatal(e)
			}
			defer tx.Rollback(ctx)
			args := pcArgs{q.result.OperationID, hash, 1}
			switch mode {
			case "wrong_hash":
				args.ReportHash = strings.Repeat("0", 64)
			case "wrong_attempt":
				args.OperationID = randomUUID()
			case "wrong_version":
				args.Version = 2
			}
			job, e := jobs.InsertTx(ctx, tx, args, nil)
			if e != nil {
				t.Fatal(e)
			}
			if mode == "terminal_job" {
				if _, e = tx.Exec(ctx, `UPDATE river_payment.river_job SET state='cancelled',finalized_at=clock_timestamp() WHERE id=$1`, job.Job.ID); e != nil {
					t.Fatal(e)
				}
			}
			_, e = tx.Exec(ctx, `SELECT integration.record_payment_query($1,$2,$3,'PROVIDER_MOCK',$4::jsonb,$5)`, q.result.OperationID, c.Generation, c.LeaseToken, pqJSON(r), job.Job.ID)
			if sqlState(e) != "PT409" {
				t.Fatalf("expected exact job binding denial: %v", e)
			}
		})
	}
	if q.reportCount(t) != 0 {
		t.Fatal("mismatched job persisted report")
	}
	if e := q.record(c, r); e != nil {
		t.Fatal(e)
	}
	// A second query gets a valid lease and enqueues inside the same tx, then an
	// observation trigger fails. Neither the new job nor report may survive.
	c = q.claim(t)
	newReport := pcFull(q)
	newReport["CardRefundStatus"] = "1"
	name := "pc_intake_" + t04Tag()
	mustExec(t, q.f.owner, fmt.Sprintf(`CREATE FUNCTION ops.%s() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.attempt_id='%s'::uuid THEN RAISE EXCEPTION 'intake fault' USING ERRCODE='PCT02'; END IF; RETURN NEW; END $$`, name, q.result.AttemptID))
	mustExec(t, q.f.owner, fmt.Sprintf(`CREATE TRIGGER %s AFTER INSERT ON payments.provider_observations FOR EACH ROW EXECUTE FUNCTION ops.%s()`, name, name))
	if e := q.record(c, newReport); sqlState(e) != "PCT02" {
		t.Fatalf("intake fault not reached %v", e)
	}
	var count int
	if e = q.f.owner.QueryRow(ctx, `SELECT count(*) FROM river_payment.river_job WHERE kind='payment_reconcile_v1' AND args->>'operation_id'=$1`, q.result.OperationID).Scan(&count); e != nil || count != 1 || q.reportCount(t) != 1 {
		t.Fatalf("intake not atomic: jobs%d err%v", count, e)
	}
	mustExec(t, q.f.owner, fmt.Sprintf(`DROP TRIGGER %s ON payments.provider_observations`, name))
	mustExec(t, q.f.owner, fmt.Sprintf(`DROP FUNCTION ops.%s()`, name))
}

func TestBuyerPaymentCaptureForgedRiverJobCancelsWithoutMoney(t *testing.T) {
	q := pqSetupWorker(t)
	ctx := context.Background()
	w, e := payments.NewCaptureWorker(ctx, q.worker)
	if e != nil {
		t.Fatal(e)
	}
	workers := river.NewWorkers()
	river.AddWorker(workers, w)
	queue := "payment_mock_v1"
	client, e := river.NewClient(riverpgxv5.New(q.worker), &river.Config{Schema: "river_payment", Workers: workers, Queues: map[string]river.QueueConfig{queue: {MaxWorkers: 2}}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), JobTimeout: 15 * time.Second, RescueStuckJobsAfter: 30 * time.Second})
	if e != nil {
		t.Fatal(e)
	}
	// Begin with valid committed intake, then use fixture-owner corruption to
	// exercise the worker's defense behind the new commit-time insert fence.
	if e = q.record(q.claim(t), pqReport(q)); e != nil {
		t.Fatal(e)
	}
	mustExec(t, q.f.owner, `ALTER TABLE river_payment.river_job DISABLE TRIGGER payment_job_family`)
	defer mustExec(t, q.f.owner, `ALTER TABLE river_payment.river_job ENABLE TRIGGER payment_job_family`)
	var jobID int64
	if e = q.f.owner.QueryRow(ctx, `UPDATE river_payment.river_job SET
		args=jsonb_set(args,'{report_hash}',to_jsonb($2::text))
		WHERE kind='payment_reconcile_v1' AND args->>'operation_id'=$1 RETURNING id`,
		q.result.OperationID, strings.Repeat("a", 64)).Scan(&jobID); e != nil {
		t.Fatal(e)
	}
	mustExec(t, q.f.owner, `ALTER TABLE river_payment.river_job ENABLE TRIGGER payment_job_family`)
	if e = client.Start(ctx); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if e := client.StopAndCancel(c); e != nil {
			t.Error(e)
		}
	})
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var state string
		var attempts int
		if e = q.f.owner.QueryRow(ctx, `SELECT state,attempt FROM river_payment.river_job WHERE id=$1`, jobID).Scan(&state, &attempts); e != nil {
			t.Fatal(e)
		}
		if state == "cancelled" {
			if attempts != 1 || pcCount(t, q, "payments.facts", "") != 0 || q.reportCount(t) != 1 {
				t.Fatal("forged job retried or wrote money")
			}
			q.pending(t)
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("permanently invalid observation job did not cancel")
}
