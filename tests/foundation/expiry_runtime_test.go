package foundation_test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivertype"
	"livecommerce/internal/checkout"
	"livecommerce/internal/storefront"
	"livecommerce/migrations"
)

const ewQueue = "checkout_expiry_v1"

// Every EW test owns pwIsolatedFixture's labelled, loopback-only PG container.
// The suite database's default River queue is never consumed or mutated here.
func ewFixture(t *testing.T) *testFixture {
	t.Helper()
	return pwIsolatedFixture(t)
}

func ewSetup(t *testing.T, f *testFixture, lines int) psHarness {
	t.Helper()
	p := psSetupItemsOn(t, f, lines)
	if queue := pwQueue(t, f.owner, p.hold.JobID); queue != ewQueue {
		t.Fatalf("checkout producer queue=%s", queue)
	}
	return p
}

func ewDue(t *testing.T, p psHarness) {
	t.Helper()
	bcDue(t, p.bcHarness, p.hold)
	mustExec(t, p.f.owner, `UPDATE river.river_job SET scheduled_at=clock_timestamp() WHERE id=$1`, p.hold.JobID)
}

func ewJob(t *testing.T, pool *pgxpool.Pool, id int64) (state string, attempt int) {
	t.Helper()
	if err := pool.QueryRow(context.Background(), `SELECT state,attempt FROM river.river_job WHERE id=$1`, id).Scan(&state, &attempt); err != nil {
		t.Fatal(err)
	}
	return
}

func ewAwait(t *testing.T, pool *pgxpool.Pool, id int64, want string) int {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		state, attempt := ewJob(t, pool, id)
		if state == want && attempt > 0 {
			return attempt
		}
		time.Sleep(20 * time.Millisecond)
	}
	state, attempt := ewJob(t, pool, id)
	t.Fatalf("expiry job %d state=%s attempt=%d, want %s after actual claim", id, state, attempt, want)
	return 0
}

func ewClient(t *testing.T, pool *pgxpool.Pool, concurrency int) *river.Client[pgx.Tx] {
	t.Helper()
	client, err := checkout.NewExpiryClient(context.Background(), pool, concurrency)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pwStopClient(t, client) })
	return client
}

func ewReady(t *testing.T, worker *pgxpool.Pool) bool {
	t.Helper()
	var ready bool
	if err := worker.QueryRow(context.Background(), `SELECT checkout.expiry_queue_ready()`).Scan(&ready); err != nil {
		t.Fatal(err)
	}
	return ready
}

func ewRemoveRouter(t *testing.T, owner *pgxpool.Pool) {
	t.Helper()
	// Only the new checksummed migration is rewound in this test-owned database.
	mustExec(t, owner, `DROP TRIGGER checkout_expiry_queue_route_v1 ON river.river_job`)
	mustExec(t, owner, `DROP FUNCTION checkout.expiry_queue_ready()`)
	mustExec(t, owner, `DROP FUNCTION checkout.route_expiry_queue_v1()`)
	mustExec(t, owner, `DROP FUNCTION checkout.expiry_job_linked(bigint)`)
	mustExec(t, owner, `DELETE FROM public.lc_schema_migrations WHERE version='post_river/0002_checkout_expiry_queue.sql'`)
}

func ewRouterAbsent(t *testing.T, owner *pgxpool.Pool) {
	t.Helper()
	var ledger, trigger int
	if err := owner.QueryRow(context.Background(), `SELECT
	 (SELECT count(*) FROM public.lc_schema_migrations WHERE version='post_river/0002_checkout_expiry_queue.sql'),
	 (SELECT count(*) FROM pg_trigger WHERE tgrelid='river.river_job'::regclass AND tgname='checkout_expiry_queue_route_v1')`).Scan(&ledger, &trigger); err != nil {
		t.Fatal(err)
	}
	if ledger != 0 || trigger != 0 {
		t.Fatalf("failed migration partly installed: ledger=%d trigger=%d", ledger, trigger)
	}
}

// The real checkout service still inserts its job before begin_hold. Middleware
// models the older producer's requested queue without reproducing business SQL.
func ewOldBegin(t *testing.T, p psHarness) checkout.Result {
	t.Helper()
	b := p.bcHarness
	b.prepare(t, b.cap, []storefront.Item{{SKUID: b.stock.skus[0].ID, Quantity: 1}})
	middleware := river.JobInsertMiddlewareFunc(func(ctx context.Context, params []*rivertype.JobInsertParams, next func(context.Context) ([]*rivertype.JobInsertResult, error)) ([]*rivertype.JobInsertResult, error) {
		for _, row := range params {
			if row.Kind == ewQueue {
				row.Queue = "default"
			}
		}
		return next(ctx)
	})
	jobs, err := river.NewClient(riverpgxv5.New(b.pool), &river.Config{Schema: "river", JobInsertMiddleware: []rivertype.JobInsertMiddleware{middleware}})
	if err != nil {
		t.Fatal(err)
	}
	service, err := checkout.New(context.Background(), b.pool, jobs)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Begin(context.Background(), b.cap.Token, b.f.storeA1, t04Key("ew-legacy"), b.input)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestBuyerCheckoutExpiryRuntimeMigrationUpgradeRollbackAndLedger(t *testing.T) {
	f := ewFixture(t)
	p := ewSetup(t, f, 1)
	worker := p.worker
	if !ewReady(t, worker) {
		t.Fatal("fresh migration not ready")
	}
	ewRemoveRouter(t, f.owner)
	mustExec(t, f.owner, `UPDATE river.river_job SET queue='default' WHERE id=$1`, p.hold.JobID)
	legacy := ewOldBegin(t, p)
	if queue := pwQueue(t, f.owner, legacy.JobID); queue != "default" {
		t.Fatalf("legacy producer queue=%s", queue)
	}
	// The original due time and every other River column must survive the move.
	before := pwJobExceptQueue(t, f.owner, legacy.JobID)
	terminal := p.hold.JobID
	mustExec(t, f.owner, `UPDATE river.river_job SET state='completed',queue='default',finalized_at=clock_timestamp() WHERE id=$1`, terminal)
	terminalBefore := pwJobExceptQueue(t, f.owner, terminal)
	var unrelated int64
	if err := f.owner.QueryRow(context.Background(), `INSERT INTO river.river_job(kind,args,max_attempts,queue) VALUES('external_operation_v1','{}',2,'default') RETURNING id`).Scan(&unrelated); err != nil {
		t.Fatal(err)
	}
	unrelatedBefore := pwJobExceptQueue(t, f.owner, unrelated)
	validArgs := fmt.Sprintf(`{"order_id":"%s","generation":1,"version":1}`, legacy.OrderID)
	for _, tc := range []struct{ name, mutation, restore string }{
		{"running", `UPDATE river.river_job SET state='running' WHERE id=$1`, `UPDATE river.river_job SET state='scheduled' WHERE id=$1`},
		{"orphan", `UPDATE river.river_job SET args=jsonb_build_object('order_id',gen_random_uuid()::text,'generation',1,'version',1) WHERE id=$1`, `UPDATE river.river_job SET args=$2::jsonb WHERE id=$1`},
		{"decimal_generation", `UPDATE river.river_job SET args=jsonb_set(args,'{generation}','1.0'::jsonb) WHERE id=$1`, `UPDATE river.river_job SET args=$2::jsonb WHERE id=$1`},
		{"decimal_version", `UPDATE river.river_job SET args=jsonb_set(args,'{version}','1.0'::jsonb) WHERE id=$1`, `UPDATE river.river_job SET args=$2::jsonb WHERE id=$1`},
		{"extra_key", `UPDATE river.river_job SET args=args||'{"extra":true}'::jsonb WHERE id=$1`, `UPDATE river.river_job SET args=$2::jsonb WHERE id=$1`},
		{"unique_key", `UPDATE river.river_job SET unique_key=decode(repeat('ab',32),'hex') WHERE id=$1`, `UPDATE river.river_job SET unique_key=NULL WHERE id=$1`},
		{"wrong_queue", `UPDATE river.river_job SET queue='other_expiry_queue' WHERE id=$1`, `UPDATE river.river_job SET queue='default' WHERE id=$1`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mustExec(t, f.owner, tc.mutation, legacy.JobID)
			rowBefore := pwJobExceptQueue(t, f.owner, legacy.JobID)
			queueBefore := pwQueue(t, f.owner, legacy.JobID)
			if err := migrations.Apply(context.Background(), f.owner); err == nil {
				t.Fatal("invalid active expiry row migrated")
			}
			ewRouterAbsent(t, f.owner)
			if pwJobExceptQueue(t, f.owner, legacy.JobID) != rowBefore || pwQueue(t, f.owner, legacy.JobID) != queueBefore {
				t.Fatal("failed migration changed legacy row")
			}
			if strings.Contains(tc.restore, "$2") {
				mustExec(t, f.owner, tc.restore, legacy.JobID, validArgs)
			} else {
				mustExec(t, f.owner, tc.restore, legacy.JobID)
			}
		})
	}
	var foreign int64
	if err := f.owner.QueryRow(context.Background(), `INSERT INTO river.river_job(kind,args,max_attempts,queue) VALUES('foreign_v1','{}',2,$1) RETURNING id`, ewQueue).Scan(&foreign); err != nil {
		t.Fatal(err)
	}
	if err := migrations.Apply(context.Background(), f.owner); err == nil {
		t.Fatal("foreign reserved kind passed upgrade")
	}
	ewRouterAbsent(t, f.owner)
	mustExec(t, f.owner, `DELETE FROM river.river_job WHERE id=$1`, foreign)
	if err := migrations.Apply(context.Background(), f.owner); err != nil {
		t.Fatalf("valid upgrade: %v", err)
	}
	if pwQueue(t, f.owner, legacy.JobID) != ewQueue || pwJobExceptQueue(t, f.owner, legacy.JobID) != before || pwQueue(t, f.owner, terminal) != "default" || pwJobExceptQueue(t, f.owner, terminal) != terminalBefore || pwQueue(t, f.owner, unrelated) != "default" || pwJobExceptQueue(t, f.owner, unrelated) != unrelatedBefore || !ewReady(t, worker) {
		t.Fatal("backfill changed non-queue fields/history/unrelated job or left router unready")
	}
	if err := migrations.Apply(context.Background(), f.owner); err != nil {
		t.Fatalf("repeat migration: %v", err)
	}
	mustExec(t, f.owner, `INSERT INTO public.lc_schema_migrations(version,checksum) VALUES('post_river/9999_unknown.sql','test-only')`)
	if err := migrations.Apply(context.Background(), f.owner); err == nil {
		t.Fatal("unknown post-River version accepted")
	}
	mustExec(t, f.owner, `DELETE FROM public.lc_schema_migrations WHERE version='post_river/9999_unknown.sql'`)
	var checksum string
	if err := f.owner.QueryRow(context.Background(), `SELECT checksum FROM public.lc_schema_migrations WHERE version='post_river/0002_checkout_expiry_queue.sql'`).Scan(&checksum); err != nil {
		t.Fatal(err)
	}
	mustExec(t, f.owner, `UPDATE public.lc_schema_migrations SET checksum='test-invalid' WHERE version='post_river/0002_checkout_expiry_queue.sql'`)
	if err := migrations.Apply(context.Background(), f.owner); err == nil {
		t.Fatal("changed checksum accepted")
	}
	mustExec(t, f.owner, `UPDATE public.lc_schema_migrations SET checksum=$1 WHERE version='post_river/0002_checkout_expiry_queue.sql'`, checksum)
	late := ewOldBegin(t, p)
	if pwQueue(t, f.owner, late.JobID) != ewQueue {
		t.Fatal("old job-before-order producer was not routed at commit")
	}
}

func ewAssertOrder(t *testing.T, p psHarness, orderState, reservationState string, releases int) {
	t.Helper()
	var actualOrder, actualReservation string
	var reserved, allocated int64
	err := p.f.owner.QueryRow(context.Background(), `SELECT o.commercial_state,r.state,
	 coalesce(sum(b.reserved),0),coalesce(sum(b.allocated),0)
	 FROM checkout.orders o JOIN inventory.reservations r ON r.id=o.id
	 JOIN inventory.reservation_lines l ON l.reservation_id=r.id
	 JOIN inventory.balances b ON (b.tenant_id,b.store_id,b.warehouse_id,b.sku_id)=(l.tenant_id,l.store_id,l.warehouse_id,l.sku_id)
	 WHERE o.id=$1 GROUP BY o.commercial_state,r.state`, p.hold.OrderID).
		Scan(&actualOrder, &actualReservation, &reserved, &allocated)
	if err != nil {
		t.Fatal(err)
	}
	if actualOrder != orderState || actualReservation != reservationState {
		t.Fatalf("order/reservation=%s/%s want %s/%s", actualOrder, actualReservation, orderState, reservationState)
	}
	wantReserved := int64(2 * len(p.stock.skus))
	if orderState == "CANCELLED" || orderState == "CONFIRMED" {
		wantReserved = 0
	}
	wantAllocated := int64(0)
	if orderState == "CONFIRMED" {
		wantAllocated = int64(2 * len(p.stock.skus))
	}
	if reserved != wantReserved || allocated != wantAllocated {
		t.Fatalf("balance reserved/allocated=%d/%d want %d/%d", reserved, allocated, wantReserved, wantAllocated)
	}
	if n := countRows(t, p.f.owner, `SELECT count(*) FROM inventory.ledger WHERE checkout_id=$1 AND kind='RELEASE'`, p.hold.OrderID); n != releases {
		t.Fatalf("release ledger=%d want %d", n, releases)
	}
	if n := countRows(t, p.f.owner, `SELECT count(*) FROM checkout.events WHERE order_id=$1 AND action='checkout.expired'`, p.hold.OrderID); n != map[bool]int{true: 1, false: 0}[orderState == "CANCELLED"] {
		t.Fatalf("expiry events=%d", n)
	}
}

func TestBuyerCheckoutExpiryRuntimeRiverTwoTenantReplayAndIsolation(t *testing.T) {
	f := ewFixture(t)
	one := ewSetup(t, f, 2)
	two := ewSetup(t, f, 1)
	if one.f.tenantA == two.f.tenantA || one.hold.OrderID == two.hold.OrderID {
		t.Fatal("two-tenant fixture collapsed")
	}
	// These are real payment and external-operation producers. Neither queue
	// belongs to the expiry client, even while due expiry jobs are consumed.
	unrelated := pqSetupItemsOn(t, f, nil, false, 1)
	unrelatedExpiry, external := pwDefaultDomainJobs(t, unrelated)
	var paymentJob int64
	if err := f.owner.QueryRow(context.Background(), `SELECT id FROM river.river_job WHERE kind='payment_query_v1' AND args->>'operation_id'=$1`, unrelated.result.OperationID).Scan(&paymentJob); err != nil {
		t.Fatal(err)
	}
	otherIDs := []int64{unrelatedExpiry, external, paymentJob}
	otherBefore := make([]string, len(otherIDs))
	for i, id := range otherIDs {
		otherBefore[i] = pwJobExceptQueue(t, f.owner, id)
	}
	ewDue(t, one)
	ewDue(t, two)
	client := ewClient(t, one.worker, 2)
	for _, p := range []psHarness{one, two} {
		ewAwait(t, f.owner, p.hold.JobID, "completed")
		ewAssertOrder(t, p, "CANCELLED", "EXPIRED", len(p.stock.skus))
	}
	pwStopClient(t, client)
	// The same committed job ID is redelivered after a client restart. A new
	// duplicate Insert is deliberately rejected by the immutable-link trigger.
	beforeAttempts := []int{0, 0}
	for i, p := range []psHarness{one, two} {
		_, beforeAttempts[i] = ewJob(t, f.owner, p.hold.JobID)
		mustExec(t, f.owner, `UPDATE river.river_job SET state='available',finalized_at=NULL,scheduled_at=clock_timestamp() WHERE id=$1`, p.hold.JobID)
	}
	restarted := ewClient(t, one.worker, 2)
	for i, p := range []psHarness{one, two} {
		attempt := ewAwait(t, f.owner, p.hold.JobID, "completed")
		if attempt <= beforeAttempts[i] {
			t.Fatalf("same-ID redelivery did not execute: %d -> %d", beforeAttempts[i], attempt)
		}
		ewAssertOrder(t, p, "CANCELLED", "EXPIRED", len(p.stock.skus))
	}
	pwStopClient(t, restarted)
	for i, id := range otherIDs {
		if after := pwJobExceptQueue(t, f.owner, id); after != otherBefore[i] {
			t.Fatalf("expiry client changed unrelated job %d", id)
		}
	}
	if pwQueue(t, f.owner, external) != "default" || pwQueue(t, f.owner, paymentJob) == ewQueue {
		t.Fatal("expiry client consumed an unrelated queue")
	}
}

func TestBuyerCheckoutExpiryRuntimeEarlyStalePendingAndConfirmed(t *testing.T) {
	f := ewFixture(t)
	early := ewSetup(t, f, 1)
	stale := ewSetup(t, f, 1)
	pending := ewSetup(t, f, 1)
	confirmed := pqSetupItemsOn(t, f, nil, false, 1)
	mustExec(t, f.owner, `UPDATE river.river_job SET scheduled_at=clock_timestamp() WHERE id=$1`, early.hold.JobID)
	ewDue(t, stale)
	mustExec(t, f.owner, `UPDATE checkout.orders SET generation=2 WHERE id=$1`, stale.hold.OrderID)
	mustExec(t, f.owner, `UPDATE inventory.reservations SET generation=2 WHERE id=$1`, stale.hold.OrderID)
	if _, err := pending.start(t04Key("ew-pending")); err != nil {
		t.Fatal(err)
	}
	ewDue(t, pending)
	hash := pcRecord(t, confirmed, pcFull(confirmed))
	if err := pcApply(confirmed.worker, confirmed.result.AttemptID, hash); err != nil {
		t.Fatal(err)
	}
	ewDue(t, confirmed.psHarness)
	client := ewClient(t, early.worker, 2)
	ewAwait(t, f.owner, early.hold.JobID, "scheduled")
	ewAwait(t, f.owner, stale.hold.JobID, "completed")
	ewAwait(t, f.owner, pending.hold.JobID, "completed")
	ewAwait(t, f.owner, confirmed.hold.JobID, "completed")
	pwStopClient(t, client)
	ewAssertOrder(t, early, "DRAFT", "HELD", 0)
	ewAssertOrder(t, stale, "DRAFT", "HELD", 0)
	ewAssertOrder(t, pending, "AWAITING_PAYMENT", "PAYMENT_PENDING", 0)
	ewAssertOrder(t, confirmed.psHarness, "CONFIRMED", "COMMITTED", 0)
	var scheduled time.Time
	if err := f.owner.QueryRow(context.Background(), `SELECT scheduled_at FROM river.river_job WHERE id=$1`, early.hold.JobID).Scan(&scheduled); err != nil || !scheduled.After(time.Now()) {
		t.Fatalf("early job did not snooze until future DB deadline: %s %v", scheduled, err)
	}
	var currentGeneration int64
	if err := f.owner.QueryRow(context.Background(), `SELECT generation FROM checkout.orders WHERE id=$1`, pending.hold.OrderID).Scan(&currentGeneration); err != nil || currentGeneration != 2 {
		t.Fatalf("payment start did not advance generation: %d %v", currentGeneration, err)
	}
}

func TestBuyerCheckoutExpiryRuntimePaymentStartRace(t *testing.T) {
	f := ewFixture(t)
	p := ewSetup(t, f, 1)
	bcDue(t, p.bcHarness, p.hold)
	client := ewClient(t, p.worker, 1)
	start := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		<-start
		_, err := p.start(t04Key("ew-race-payment"))
		result <- err
	}()
	close(start)
	mustExec(t, f.owner, `UPDATE river.river_job SET scheduled_at=clock_timestamp() WHERE id=$1`, p.hold.JobID)
	select {
	case <-result:
	case <-time.After(8 * time.Second):
		t.Fatal("payment start did not settle")
	}
	ewAwait(t, f.owner, p.hold.JobID, "completed")
	pwStopClient(t, client)
	var order, reservation string
	var attempts, releases int
	if err := f.owner.QueryRow(context.Background(), `SELECT o.commercial_state,r.state,
	 (SELECT count(*) FROM checkout.payment_attempts WHERE order_id=o.id),
	 (SELECT count(*) FROM inventory.ledger WHERE checkout_id=o.id AND kind='RELEASE')
	 FROM checkout.orders o JOIN inventory.reservations r ON r.id=o.id WHERE o.id=$1`, p.hold.OrderID).
		Scan(&order, &reservation, &attempts, &releases); err != nil {
		t.Fatal(err)
	}
	switch {
	case order == "CANCELLED" && reservation == "EXPIRED" && attempts == 0 && releases == 1:
		ewAssertOrder(t, p, "CANCELLED", "EXPIRED", 1)
	case order == "AWAITING_PAYMENT" && reservation == "PAYMENT_PENDING" && attempts == 1 && releases == 0:
		ewAssertOrder(t, p, "AWAITING_PAYMENT", "PAYMENT_PENDING", 0)
	default:
		t.Fatalf("non-serial expiry/payment outcome: %s/%s attempts=%d releases=%d", order, reservation, attempts, releases)
	}
}

func TestBuyerCheckoutExpiryRuntimeLateOldProducerPoll(t *testing.T) {
	f := ewFixture(t)
	p := ewSetup(t, f, 1)
	client := ewClient(t, p.worker, 1)
	late := ewOldBegin(t, p)
	if queue := pwQueue(t, f.owner, late.JobID); queue != ewQueue {
		t.Fatalf("late old producer committed to %s", queue)
	}
	// The INSERT announced the requested default queue before deferred commit
	// routing; this already-running fixed-queue client must find it by polling.
	mustExec(t, f.owner, `UPDATE checkout.orders SET expires_at=clock_timestamp()-interval '1 second',created_at=clock_timestamp()-interval '901 seconds' WHERE id=$1`, late.OrderID)
	mustExec(t, f.owner, `UPDATE inventory.reservations SET expires_at=clock_timestamp()-interval '1 second',created_at=clock_timestamp()-interval '901 seconds' WHERE id=$1`, late.OrderID)
	mustExec(t, f.owner, `UPDATE river.river_job SET scheduled_at=clock_timestamp() WHERE id=$1`, late.JobID)
	ewAwait(t, f.owner, late.JobID, "completed")
	pwStopClient(t, client)
	var order, reservation string
	if err := f.owner.QueryRow(context.Background(), `SELECT o.commercial_state,r.state FROM checkout.orders o JOIN inventory.reservations r ON r.id=o.id WHERE o.id=$1`, late.OrderID).Scan(&order, &reservation); err != nil || order != "CANCELLED" || reservation != "EXPIRED" {
		t.Fatalf("late old producer job was not executed: %s/%s %v", order, reservation, err)
	}
	if n := countRows(t, f.owner, `SELECT count(*) FROM inventory.ledger WHERE checkout_id=$1 AND kind='RELEASE'`, late.OrderID); n != 1 {
		t.Fatalf("late old producer release count=%d", n)
	}
	if state, attempt := ewJob(t, f.owner, p.hold.JobID); state != "scheduled" || attempt != 0 {
		t.Fatalf("unrelated future expiry was consumed: %s/%d", state, attempt)
	}
}

type ewBinary struct {
	cmd  *exec.Cmd
	done chan error
}

func ewBuildBinary(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "expiry-worker")
	build := exec.Command("go", "build", "-o", binary, "../../cmd/expiry-worker")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build real expiry-worker: %v %s", err, out)
	}
	return binary
}

func ewWorkerDSN(t *testing.T, f *testFixture) (string, string) {
	t.Helper()
	u, err := url.Parse(bcRole(t, f, "commerce_worker"))
	if err != nil {
		t.Fatal(err)
	}
	app := "ew_binary_" + t04Tag()
	q := u.Query()
	q.Set("application_name", app)
	u.RawQuery = q.Encode()
	return u.String(), app
}

func ewStartBinary(t *testing.T, binary, dsn string) *ewBinary {
	t.Helper()
	cmd := exec.Command(binary)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "COMMERCE_EXPIRY_WORKER_ENABLED=1",
		"COMMERCE_EXPIRY_WORKER_DATABASE_URL=" + dsn, "COMMERCE_EXPIRY_WORKER_CONCURRENCY=1",
		"COMMERCE_ACCOUNT_KEYS_JSON=invalid", "COMMERCE_PAYMENT_WORKER_DATABASE_URL=invalid"}
	cmd.Stdout = io.Discard
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	ready := make(chan struct{}, 1)
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		scan := bufio.NewScanner(stderr)
		for scan.Scan() {
			if strings.HasSuffix(scan.Text(), " INFO expiry_worker_ready") {
				select {
				case ready <- struct{}{}:
				default:
				}
			}
		}
	}()
	p := &ewBinary{cmd: cmd, done: make(chan error, 1)}
	go func() { <-readerDone; p.done <- cmd.Wait() }()
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	select {
	case <-ready:
	case err := <-p.done:
		t.Fatalf("expiry-worker exited before ready: %v", err)
	case <-time.After(8 * time.Second):
		t.Fatal("expiry-worker did not report ready after River Start")
	}
	return p
}

func ewExit(t *testing.T, p *ewBinary, signal syscall.Signal, wantSuccess bool) {
	t.Helper()
	if err := p.cmd.Process.Signal(signal); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-p.done:
		if (err == nil) != wantSuccess {
			t.Fatalf("expiry-worker signal %v exit=%v want success=%t", signal, err, wantSuccess)
		}
	case <-time.After(8 * time.Second):
		t.Fatalf("expiry-worker did not exit after %v", signal)
	}
}

func ewConnections(t *testing.T, f *testFixture, app string) int {
	t.Helper()
	var n int
	if err := f.owner.QueryRow(context.Background(), `SELECT count(*) FROM pg_stat_activity WHERE datname='lc_foundation_test' AND application_name=$1`, app).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestBuyerCheckoutExpiryRuntimeBinarySignalAndPoolCleanup(t *testing.T) {
	f := ewFixture(t)
	binary := ewBuildBinary(t)
	disabled := exec.Command(binary)
	disabled.Env = []string{"PATH=" + os.Getenv("PATH"), "COMMERCE_EXPIRY_WORKER_ENABLED=0", "COMMERCE_EXPIRY_WORKER_DATABASE_URL=invalid", "COMMERCE_ACCOUNT_KEYS_JSON=invalid"}
	if out, err := disabled.CombinedOutput(); err != nil || len(out) != 0 {
		t.Fatalf("disabled process opened resources: %v %q", err, out)
	}
	dsn, app := ewWorkerDSN(t, f)
	p := ewStartBinary(t, binary, dsn)
	deadline := time.Now().Add(8 * time.Second)
	for ewConnections(t, f, app) == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if ewConnections(t, f, app) == 0 {
		t.Fatal("ready process has no worker DB connection")
	}
	ewExit(t, p, syscall.SIGTERM, true)
	if remaining := ewConnections(t, f, app); remaining != 0 {
		t.Fatalf("SIGTERM left %d worker connections", remaining)
	}
}

func TestBuyerCheckoutExpiryRuntimeBinaryCrashRiverRescue(t *testing.T) {
	f := ewFixture(t)
	p := ewSetup(t, f, 1)
	ewDue(t, p)
	lock, err := f.owner.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Rollback(context.Background())
	var holderPID int
	if err := lock.QueryRow(context.Background(), `SELECT pg_backend_pid()`).Scan(&holderPID); err != nil {
		t.Fatal(err)
	}
	if _, err := lock.Exec(context.Background(), `SELECT 1 FROM checkout.orders WHERE id=$1 FOR UPDATE`, p.hold.OrderID); err != nil {
		t.Fatal(err)
	}
	binary := ewBuildBinary(t)
	dsn, app := ewWorkerDSN(t, f)
	crashed := ewStartBinary(t, binary, dsn)
	deadline := time.Now().Add(8 * time.Second)
	witness := false
	for time.Now().Before(deadline) {
		var state string
		var blocked bool
		err := f.owner.QueryRow(context.Background(), `SELECT j.state,
	 EXISTS(SELECT 1 FROM pg_stat_activity a WHERE a.application_name=$2 AND $3::int=ANY(pg_blocking_pids(a.pid)))
	 FROM river.river_job j WHERE j.id=$1`, p.hold.JobID, app, holderPID).Scan(&state, &blocked)
		if err != nil {
			t.Fatal(err)
		}
		if state == "running" && blocked {
			witness = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !witness {
		t.Fatal("no running River lease blocked on the owned order lock")
	}
	ewExit(t, crashed, syscall.SIGKILL, false)
	if err := lock.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	if state, _ := ewJob(t, f.owner, p.hold.JobID); state != "running" {
		t.Fatalf("SIGKILL did not leave a River running lease: %s", state)
	}
	// Clock-age only the killed process's owned attempted_at. River's default
	// one-hour lease and 30-second rescue scan are unchanged: this is not an SLA.
	var aged int64
	if err := f.owner.QueryRow(context.Background(), `UPDATE river.river_job SET attempted_at=clock_timestamp()-interval '2 hours'
	 WHERE id=$1 AND state='running' RETURNING id`, p.hold.JobID).Scan(&aged); err != nil || aged != p.hold.JobID {
		t.Fatalf("age owned running lease: %d %v", aged, err)
	}
	restarted := ewStartBinary(t, binary, dsn)
	rescueDeadline := time.Now().Add(65 * time.Second)
	var state string
	var attempt int
	var rescued bool
	for time.Now().Before(rescueDeadline) {
		if err := f.owner.QueryRow(context.Background(), `SELECT state,attempt,
	 coalesce(errors::text LIKE '%Stuck job rescued by JobRescuer%',false)
	 FROM river.river_job WHERE id=$1`, p.hold.JobID).Scan(&state, &attempt, &rescued); err != nil {
			t.Fatal(err)
		}
		if state == "completed" && attempt >= 2 && rescued {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if state != "completed" || attempt < 2 || !rescued {
		t.Fatalf("River rescue did not finish same job: state=%s attempt=%d rescued=%t", state, attempt, rescued)
	}
	ewExit(t, restarted, syscall.SIGTERM, true)
	ewAssertOrder(t, p, "CANCELLED", "EXPIRED", 1)
	if remaining := ewConnections(t, f, app); remaining != 0 {
		t.Fatalf("restarted process left %d worker connections", remaining)
	}
}
