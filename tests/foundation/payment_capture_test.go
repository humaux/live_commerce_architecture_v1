package foundation_test

// Purpose: payments.apply_capture / capture worker suite: atomic capture, review, refund
// ordering, ACL, retired-provider rejection and Stripe durable observation across restart.
// Depends on: pq/pc/ps fixtures, rm2StripePending + sfl Stripe fake (payment_fixture_helpers_test.go,
// stripe_flow_test.go), payments worker assembly, River schema river_payment.
// Used by: go test ./tests/foundation (CI gate shard g07).
// Invariants: PAYUNi history never moves money. PAY-RM2 phase 2b (0197, post_river/0026): see output/pay-rm2-p2b/test-contract-map.md.

import (
	"context"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"livecommerce/internal/payments"
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
	// PAY-RM2 phase 2b (0197, post_river/0026) dropped the PAYUNi SQL this test drove; mapped (a)+(b) in
	// output/pay-rm2-p2b/test-contract-map.md: the dropped path is refused and writes nothing, and the
	// surviving invariant is asserted by the Stripe test named there.
	p2bCaptureRetired(t)
}

func TestBuyerPaymentCaptureEvidenceAndStickyReview(t *testing.T) {
	// PAY-RM2 phase 2b (0197, post_river/0026) dropped the PAYUNi SQL this test drove; mapped (a)+(b) in
	// output/pay-rm2-p2b/test-contract-map.md: the dropped path is refused and writes nothing, and the
	// surviving invariant is asserted by the Stripe test named there.
	p2bCaptureRetired(t)
}

func TestBuyerPaymentCaptureLaterRefundAndReverseOrder(t *testing.T) {
	// PAY-RM2 phase 2b (0197, post_river/0026) dropped the PAYUNi SQL this test drove; mapped (a)+(b) in
	// output/pay-rm2-p2b/test-contract-map.md: the dropped path is refused and writes nothing, and the
	// surviving invariant is asserted by the Stripe test named there.
	p2bCaptureRetired(t)
}

func TestBuyerPaymentCaptureOldAuthorizationIsNotConflict(t *testing.T) {
	// PAY-RM2 phase 2b (0197, post_river/0026) dropped the PAYUNi SQL this test drove; mapped (a)+(b) in
	// output/pay-rm2-p2b/test-contract-map.md: the dropped path is refused and writes nothing, and the
	// surviving invariant is asserted by the Stripe test named there.
	p2bCaptureRetired(t)
}

func TestBuyerPaymentCaptureACLAndObservationBinding(t *testing.T) {
	// PAY-RM2 phase 2b (0197, post_river/0026) dropped the PAYUNi SQL this test drove; mapped (a)+(b) in
	// output/pay-rm2-p2b/test-contract-map.md: the dropped path is refused and writes nothing, and the
	// surviving invariant is asserted by the Stripe test named there.
	p2bCaptureRetired(t)
}

func TestBuyerPaymentCaptureFaultsAreCausalAndAtomic(t *testing.T) {
	// PAY-RM2 phase 2b (0197, post_river/0026) dropped the PAYUNi SQL this test drove; mapped (a)+(b) in
	// output/pay-rm2-p2b/test-contract-map.md: the dropped path is refused and writes nothing, and the
	// surviving invariant is asserted by the Stripe test named there.
	p2bCaptureRetired(t)
}

// The old signed query chain is now forbidden at both dispatch stages. Historical
// authenticated observations must survive without financial/stock/work mutation.
func TestBuyerPaymentCaptureRealRiverSignedQueryChain(t *testing.T) {
	q := pqSetupWorker(t)
	rm2SeedReport(t, q, pcFull(q)) // previously persisted evidence (PAY-RM2 2b: seeded, record_payment_query dropped), not provider execution
	before := rm2Snapshot(t, q.psHarness, q.result.AttemptID)
	var calls atomic.Int32
	opts := payments.DefaultQueryWorkerOptions()
	body := pqSignedResponse(pcFull(q))
	opts.MockTransport = pqTransport(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	client, err := payments.NewWorkerClient(context.Background(), q.worker, q.keys, "PROVIDER_MOCK", 1, opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pwStopClient(t, client) })
	var captureJob int64
	var jobHash string
	if err = q.f.owner.QueryRow(context.Background(), `SELECT id,args->>'report_hash' FROM river_payment.river_job WHERE kind='payment_reconcile_v1' AND args->>'operation_id'=$1`, q.result.OperationID).Scan(&captureJob, &jobHash); err != nil {
		t.Fatal(err)
	}
	if jobHash != hex.EncodeToString(pcHash(t, q, pcFull(q))) {
		t.Fatal("archived reconcile job lost canonical report binding")
	}
	mustExec(t, q.f.owner, `UPDATE river_payment.river_job SET scheduled_at=clock_timestamp() WHERE id=$1`, q.result.JobID)
	if err = client.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	pwAwaitJob(t, q, q.result.JobID, "cancelled")
	pwAwaitJob(t, q, captureJob, "cancelled")
	pwStopClient(t, client)
	if calls.Load() != 0 || rm2Snapshot(t, q.psHarness, q.result.AttemptID) != before || pcCount(t, q, "payments.facts", "") != 0 || pcCount(t, q, "fulfillment.payment_work_items", "") != 0 {
		t.Fatal("retired query/capture chain changed money or history")
	}
	pcAssertStock(t, q, 2, 0, "AWAITING_PAYMENT", "PAYMENT_PENDING")
}

// Replaces the active-provider durability half of the former PAYUNi signed query
// chain. The real Stripe query commits its canonical observation and reconcile
// job while River acknowledges a queue pause; capture runs only after restart.
func TestStripeCaptureDurableObservationAcrossRestart(t *testing.T) {
	q := rm2StripePending(t)
	e := q.stripe
	ctx := context.Background()
	session := e.session(t, q.result.AttemptID)
	if !e.fake.SetState(session, "complete", "paid") {
		t.Fatal("fake Stripe session missing")
	}
	entered, release := make(chan struct{}, 1), make(chan struct{})
	var once sync.Once
	transport := e.fake.Transport()
	rt, err := payments.NewStripeRuntime(ctx, e.pool, e.keys, "PROVIDER_MOCK", pqTransport(func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/"+session) {
			select {
			case entered <- struct{}{}:
			default:
			}
			select {
			case <-release:
			case <-r.Context().Done():
				return nil, r.Context().Err()
			}
		}
		return transport.RoundTrip(r)
	}))
	if err != nil {
		t.Fatal(err)
	}
	client, err := payments.NewPaymentWorkerClient(ctx, e.pool, payments.WorkerConfig{Profile: "PROVIDER_MOCK", Concurrency: 2, Query: e.opts, Keys: e.keys, Stripe: rt})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { once.Do(func() { close(release) }); pwStopClient(t, client) })
	// The fixture leaves the query job available but not yet due; sstWake skips that state.
	q.rm2WakeQuery(t)
	if err = client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("Stripe query never entered transport")
	}
	paused, unsubscribe := client.Subscribe(river.EventKindQueuePaused)
	defer unsubscribe()
	if err = client.QueuePause(ctx, "payment_mock_v1", nil); err != nil {
		t.Fatal(err)
	}
	select {
	case event := <-paused:
		if event == nil || event.Queue == nil || event.Queue.Name != "payment_mock_v1" {
			t.Fatalf("wrong pause ACK: %+v", event)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Stripe queue pause ACK missing")
	}
	once.Do(func() { close(release) })
	var jobID int64
	var hash string
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		// Select the paid observation's own reconcile job; an earlier open-session
		// observation from the fixture cannot satisfy the money durability gate.
		err = e.f.owner.QueryRow(ctx, `SELECT j.id,j.args->>'report_hash'
   FROM river_payment.river_job j JOIN payments.provider_observations o
   ON encode(o.report_hash,'hex')=j.args->>'report_hash'
   WHERE j.kind='payment_reconcile_v1' AND j.args->>'operation_id'=$1
   AND o.attempt_id=$2 AND o.report->>'PaymentStatus'='paid'`, q.result.OperationID, q.result.AttemptID).Scan(&jobID, &hash)
		if err == nil {
			break
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if jobID == 0 {
		t.Fatal("paid Stripe observation/reconcile was not durable")
	}
	var canonical string
	if err = e.f.owner.QueryRow(ctx, `SELECT encode(sha256(convert_to(report::text,'UTF8')),'hex') FROM payments.provider_observations WHERE attempt_id=$1 AND encode(report_hash,'hex')=$2`, q.result.AttemptID, hash).Scan(&canonical); err != nil || hash != canonical {
		t.Fatalf("durable canonical report hash: %v", err)
	}
	var state string
	var attempt int
	if err = e.f.owner.QueryRow(ctx, `SELECT state,attempt FROM river_payment.river_job WHERE id=$1`, jobID).Scan(&state, &attempt); err != nil || state != "available" || attempt != 0 {
		t.Fatalf("capture fetched before restart: %s %d %v", state, attempt, err)
	}
	pcAssertStock(t, q, 2, 0, "AWAITING_PAYMENT", "PAYMENT_PENDING")
	if pcCount(t, q, "payments.facts", "") != 0 || pcCount(t, q, "fulfillment.payment_work_items", "") != 0 {
		t.Fatal("query applied money before capture restart")
	}
	pwStopClient(t, client)
	mustExec(t, e.f.owner, `UPDATE river_payment.river_queue SET paused_at=NULL WHERE name='payment_mock_v1'`)
	stop := e.start(t, true)
	defer stop()
	pwAwaitJob(t, q, jobID, "completed")
	pcAssertStock(t, q, 0, 2, "CONFIRMED", "COMMITTED")
	pcAssertWork(t, q, "READY")
	if pcCount(t, q, "payments.facts", " AND kind='CAPTURED'") != 1 {
		t.Fatal("Stripe restart capture not unique")
	}
}

func TestBuyerPaymentCaptureReleasedAnomalyKeepsRealMoney(t *testing.T) {
	// PAY-RM2 phase 2b (0197, post_river/0026) dropped the PAYUNi SQL this test drove; mapped (a)+(b) in
	// output/pay-rm2-p2b/test-contract-map.md: the dropped path is refused and writes nothing, and the
	// surviving invariant is asserted by the Stripe test named there.
	p2bCaptureRetired(t)
}

func TestBuyerPaymentCaptureBalanceWaitIsAtomic(t *testing.T) {
	// PAY-RM2 phase 2b (0197, post_river/0026) dropped the PAYUNi SQL this test drove; mapped (a)+(b) in
	// output/pay-rm2-p2b/test-contract-map.md: the dropped path is refused and writes nothing, and the
	// surviving invariant is asserted by the Stripe test named there.
	p2bCaptureRetired(t)
}

func TestBuyerPaymentCaptureEmptyReferenceAndMerchantReadScope(t *testing.T) {
	// PAY-RM2 phase 2b (0197, post_river/0026) dropped the PAYUNi SQL this test drove; mapped (a)+(b) in
	// output/pay-rm2-p2b/test-contract-map.md: the dropped path is refused and writes nothing, and the
	// surviving invariant is asserted by the Stripe test named there.
	p2bCaptureRetired(t)
}

func TestBuyerPaymentCaptureMultiSKUSingleCommit(t *testing.T) {
	// PAY-RM2 phase 2b (0197, post_river/0026) dropped the PAYUNi SQL this test drove; mapped (a)+(b) in
	// output/pay-rm2-p2b/test-contract-map.md: the dropped path is refused and writes nothing, and the
	// surviving invariant is asserted by the Stripe test named there.
	p2bCaptureRetired(t)
}

func TestBuyerPaymentCaptureQueryJobBindingAndRollback(t *testing.T) {
	// PAY-RM2 phase 2b (0197, post_river/0026) dropped the PAYUNi SQL this test drove; mapped (a)+(b) in
	// output/pay-rm2-p2b/test-contract-map.md: the dropped path is refused and writes nothing, and the
	// surviving invariant is asserted by the Stripe test named there.
	p2bCaptureRetired(t)
}

func TestBuyerPaymentCaptureForgedRiverJobCancelsWithoutMoney(t *testing.T) {
	rm2AssertRemovedWorker(t, true)
}

func TestStripeCaptureForgedRiverJobCancelsWithoutMoney(t *testing.T) {
	q := rm2StripePending(t)
	evidence := &slrEnv{sstEnv: q.stripe.sstEnv, p: q.psHarness}
	session := q.stripe.session(t, q.result.AttemptID)
	if !q.stripe.fake.SetState(session, "complete", "paid") {
		t.Fatal("missing mock session")
	}
	hash := evidence.observe(t, q.result.AttemptID, evidence.sessionReport(t, q.result.AttemptID, "complete", "paid", "succeeded", false))
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
	inserted, e := client.Insert(ctx, pcArgs{OperationID: q.result.AttemptID, ReportHash: hex.EncodeToString(hash), Version: 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	reports := q.reportCount(t)
	mustExec(t, q.f.owner, `ALTER TABLE river_payment.river_job DISABLE TRIGGER payment_job_family`)
	defer mustExec(t, q.f.owner, `ALTER TABLE river_payment.river_job ENABLE TRIGGER payment_job_family`)
	jobID := inserted.Job.ID
	if e = q.f.owner.QueryRow(ctx, `UPDATE river_payment.river_job SET
		args=jsonb_set(args,'{report_hash}',to_jsonb($2::text))
		WHERE id=$1 RETURNING id`,
		jobID, strings.Repeat("a", 64)).Scan(&jobID); e != nil {
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
			if attempts != 1 || pcCount(t, q, "payments.facts", "") != 0 || q.reportCount(t) != reports {
				t.Fatal("forged job retried or wrote money")
			}
			q.pending(t)
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("permanently invalid observation job did not cancel")
}
