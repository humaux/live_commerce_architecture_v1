package foundation_test

// Purpose: real payment-worker runtime suite: River assembly, profile isolation, signal and pool
// cleanup, and a real child-process crash rescued by River's JobRescuer for Stripe.
// Depends on: pw/pq fixtures, sfl Stripe fake (stripe_flow_test.go), payments.NewPaymentWorkerClient,
// River schema river_payment; re-execs the test binary for the crash child.
// Used by: go test ./tests/foundation (CI gate shard g07); pw* helpers used across payment suites.

import (
	"bufio"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"livecommerce/internal/integrations/accounts"
	integration "livecommerce/internal/integrations/core"
	"livecommerce/internal/payments"
	"livecommerce/internal/platform"
	"livecommerce/migrations"
)

// The business baseline provisions cluster-scoped roles, so a second database
// on the suite's server cannot be migrated fresh. Each test owns a labelled,
// loopback-only temporary PG container instead.
func pwIsolatedFixture(t *testing.T) *testFixture {
	t.Helper()
	fixture(t) // enforces the existing explicit local-PG test permission gate
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	name := "lc-payment-worker-" + t04Tag()
	password := hex.EncodeToString(randomBytes(24))
	_, err := exec.CommandContext(ctx, "docker", "run", "-d", "--pull=never", "--name", name,
		"--label", "livecommerce.fixture="+name, "--memory=512m", "--cpus=1", "--pids-limit=128",
		"--tmpfs", "/var/lib/postgresql:rw,size=268435456", "-e", "POSTGRES_PASSWORD="+password,
		"-e", "POSTGRES_DB=lc_foundation_test", "-p", "127.0.0.1::5432",
		"postgres@sha256:4ef4dbc939d61acea57712655ddb4b4ab27419c913f94cca0cd57cb3ea3c2280",
		"-c", "shared_buffers=32MB", "-c", "max_connections=60").CombinedOutput()
	if err != nil {
		t.Fatalf("start labelled local PG container: %v", err)
	}
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 15*time.Second)
		defer stop()
		label, err := exec.CommandContext(cleanup, "docker", "inspect", "-f", `{{index .Config.Labels "livecommerce.fixture"}}`, name).Output()
		if err != nil || strings.TrimSpace(string(label)) != name {
			t.Errorf("refusing unverified PG container cleanup %s: %v", name, err)
			return
		}
		if _, err := exec.CommandContext(cleanup, "docker", "rm", "-f", name).CombinedOutput(); err != nil {
			t.Errorf("remove test container %s: %v", name, err)
		}
	})
	portOutput, err := exec.CommandContext(ctx, "docker", "port", name, "5432/tcp").Output()
	if err != nil || !strings.HasPrefix(strings.TrimSpace(string(portOutput)), "127.0.0.1:") {
		t.Fatal("test PG did not bind loopback")
	}
	port := strings.TrimPrefix(strings.TrimSpace(string(portOutput)), "127.0.0.1:")
	if _, err := strconv.Atoi(port); err != nil {
		t.Fatal("test PG returned invalid port")
	}
	u := &url.URL{Scheme: "postgres", User: url.UserPassword("postgres", password), Host: "127.0.0.1:" + port, Path: "/lc_foundation_test", RawQuery: "sslmode=disable"}
	owner, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(owner.Close)
	ready := false
	for i := 0; i < 50; i++ {
		probe, stop := context.WithTimeout(ctx, 500*time.Millisecond)
		err = owner.Ping(probe)
		stop()
		if err == nil {
			ready = true
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if !ready {
		t.Fatal("test PG did not become ready")
	}
	if err := migrations.Apply(ctx, owner); err != nil {
		t.Fatal(err)
	}
	if err := migrations.Apply(ctx, owner); err != nil {
		t.Fatal(err)
	}
	if err := legacyOpenBankPolicy(ctx, owner, nil); err != nil { // legacy: issue #210
		t.Fatal(err)
	}
	if pgStripeRevoke { // promotions-tests only: disclosed workaround of BASE defect B1, see promotions_gate_test.go / DEFECTS.md
		mustExec(t, owner, `REVOKE EXECUTE ON FUNCTION catalog.products_default_slug(), design.refuse_history_change() FROM PUBLIC`)
	}
	f := &testFixture{owner: owner, databaseURL: u.String(), tenantA: randomUUID(), tenantB: randomUUID(), storeA1: randomUUID(), storeA2: randomUUID(), storeB: randomUUID(), principalA: randomUUID(), tokens: map[string]string{"a": randomToken(), "a2": randomToken(), "b": randomToken(), "expired": randomToken(), "revoked": randomToken(), "buyer": randomToken(), "revoked_grant": randomToken()}}
	if err := f.seed(ctx); err != nil {
		t.Fatal(fmt.Errorf("seed isolated worker database: %w", err))
	}
	runtimeURL := bcRole(t, f, "commerce_runtime")
	runtime, err := platform.OpenPool(ctx, runtimeURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.Close)
	f.runtime = runtime
	return f
}

func pwKeys(t *testing.T) *accounts.Keyring {
	t.Helper()
	keys, err := accounts.NewKeyring("query_test", map[string][]byte{"query_test": randomBytes(32)}, randomBytes(32))
	if err != nil {
		t.Fatal(err)
	}
	return keys
}

func pwWorkerPool(t *testing.T, f *testFixture) *pgxpool.Pool {
	t.Helper()
	return waOpen(t, f, waPayment, platform.WorkerPayment)
}

// Model an archived producer aggregate in the old family. This never resumes a PAYUNi start.
// The real deferred queue guard still normalizes the persisted default queue at commit.
func pwOldQuerySetupOn(t *testing.T, f *testFixture, keys *accounts.Keyring, historical ...string) pqFixture {
	t.Helper()
	p := psSetupItemsOn(t, f, 1, historical...)
	service, err := accounts.New(keys, &integration.Service{})
	if err != nil {
		t.Fatal(err)
	}
	schema := "river_payment"
	if len(historical) == 1 && historical[0] == "river" {
		schema = "river"
	}
	return pqFixture{psHarness: p, keys: keys, accountService: service, schema: schema, result: rm2HistoryIn(t, p, schema, "PROVIDER_MOCK", "default")}
}

func pwDefaultDomainJobs(t *testing.T, q pqFixture) (expiryID, externalID int64) {
	t.Helper()
	ctx := context.Background()
	if err := q.f.owner.QueryRow(ctx, `SELECT job_id FROM checkout.orders WHERE id=$1`, q.hold.OrderID).Scan(&expiryID); err != nil {
		t.Fatal(err)
	}
	jobs, err := river.NewClient(riverpgxv5.New(q.f.runtime), &river.Config{Schema: "river"})
	if err != nil {
		t.Fatal(err)
	}
	service, err := integration.New(jobs)
	if err != nil {
		t.Fatal(err)
	}
	err = platform.WithScope(ctx, q.f.runtime, q.f.tokens["a"], q.f.storeA1, "store:read", func(tx pgx.Tx, scope platform.Scope) error {
		planned, err := service.Plan(ctx, tx, scope, q.f.tokens["a"], t04Key("pw-external"), integration.PlanInput{
			BindingID: q.binding, ExpectedBindingVersion: 1, Purpose: "transactional", Action: "payment.authorize", Request: json.RawMessage(`{"amount":1}`),
		})
		if err == nil {
			externalID = planned.JobID
		}
		return err
	})
	if err != nil || externalID < 1 || pwQueueIn(t, q.f.owner, "river_expiry.river_job", expiryID) != "checkout_expiry_v1" || pwQueueIn(t, q.f.owner, "river.river_job", externalID) != "default" {
		t.Fatalf("actual non-payment domain jobs unavailable: expiry=%d external=%d err=%v", expiryID, externalID, err)
	}
	return expiryID, externalID
}

func pwAwaitJob(t *testing.T, q pqFixture, id int64, state string) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		var got string
		if err := q.f.owner.QueryRow(context.Background(), `SELECT state FROM river_payment.river_job WHERE id=$1`, id).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got == state {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("payment job %d did not reach %s", id, state)
}

func pwStopClient(t *testing.T, client *river.Client[pgx.Tx]) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.StopAndCancel(ctx); err != nil {
		t.Fatal(err)
	}
}

func pwQueryTrade(values url.Values) (string, error) {
	wire, err := hex.DecodeString(values.Get("EncryptInfo"))
	if err != nil {
		return "", err
	}
	parts := strings.Split(string(wire), ":::")
	if len(parts) != 2 {
		return "", fmt.Errorf("bad query envelope")
	}
	data, err := base64.StdEncoding.DecodeString(parts[0])
	if err != nil {
		return "", err
	}
	tag, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher([]byte(pqOldSecret.HashKey))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCMWithNonceSize(block, 16)
	if err != nil {
		return "", err
	}
	plain, err := gcm.Open(nil, []byte(pqOldSecret.HashIV), append(data, tag...), nil)
	if err != nil {
		return "", err
	}
	fields, err := url.ParseQuery(string(plain))
	if err != nil {
		return "", err
	}
	return fields.Get("MerTradeNo"), nil
}

func TestBuyerPaymentWorkerRealRiverTwoTenantCaptureAndRestart(t *testing.T) {
	e := sflNew(t, nil)
	f, keys := e.f, e.keys
	stores := []sstStore{e.stripeStore(t), e.stripeStore(t)}
	stopSeed := e.start(t, true)
	queries := make([]pqFixture, 2)
	sessions := make([]string, 2)
	for i, store := range stores {
		result, session := e.pinned(t, store)
		queries[i] = pqFixture{psHarness: store.p, result: result, keys: keys, schema: "river_payment"}
		sessions[i] = session
	}
	stopSeed()
	first, second := queries[0], queries[1]
	for _, session := range sessions {
		if !e.fake.SetState(session, "complete", "paid") {
			t.Fatal("missing Stripe session")
		}
	}
	expiryID, externalID := pwDefaultDomainJobs(t, first)
	expiryBefore, externalBefore := pwJobExceptQueueIn(t, f.owner, "river_expiry.river_job", expiryID), pwJobExceptQueueIn(t, f.owner, "river.river_job", externalID)
	if first.result.AttemptID == second.result.AttemptID || first.f.tenantA == second.f.tenantA {
		t.Fatal("two-tenant worker fixture collapsed")
	}
	var calls atomic.Int32
	opts := e.opts
	transport := e.fake.Transport()
	rt, err := payments.NewStripeRuntime(context.Background(), e.pool, keys, "PROVIDER_MOCK", pqTransport(func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodGet && (strings.HasSuffix(r.URL.Path, "/"+sessions[0]) || strings.HasSuffix(r.URL.Path, "/"+sessions[1])) {
			calls.Add(1)
		}
		return transport.RoundTrip(r)
	}))
	if err != nil {
		t.Fatal(err)
	}
	cfg := payments.WorkerConfig{Profile: "PROVIDER_MOCK", Concurrency: 2, Query: opts, Keys: keys, Stripe: rt}

	for _, q := range []pqFixture{first, second} {
		mustExec(t, q.f.owner, `UPDATE river_payment.river_job SET scheduled_at=clock_timestamp() WHERE id=$1`, q.result.JobID)
	}
	client, err := payments.NewPaymentWorkerClient(context.Background(), e.pool, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	running := true
	defer func() {
		if running {
			pwStopClient(t, client)
		}
	}()
	for _, q := range []pqFixture{first, second} {
		e.awaitFact(t, q.result.AttemptID, "CAPTURED")
		if n := e.count(t, `SELECT count(*) FROM payments.provider_observations WHERE attempt_id=$1 AND report->>'PaymentStatus'='paid'`, q.result.AttemptID); n != 1 {
			t.Fatalf("paid observation count=%d want 1", n)
		}
	}
	// Reconcile jobs become available only after the real query commits.
	var reconcileIDs [2]int64
	for i, q := range []pqFixture{first, second} {
		if err := q.f.owner.QueryRow(context.Background(), `SELECT id FROM river_payment.river_job WHERE kind='payment_reconcile_v1' AND args->>'operation_id'=$1 AND args->>'report_hash' IN (SELECT encode(report_hash,'hex') FROM payments.provider_observations WHERE attempt_id=$2::uuid AND report->>'PaymentStatus'='paid')`, q.result.OperationID, q.result.AttemptID).Scan(&reconcileIDs[i]); err != nil {
			t.Fatal(err)
		}
		pwAwaitJob(t, q, reconcileIDs[i], "completed")
		pcAssertStock(t, q, 0, 2, "CONFIRMED", "COMMITTED")
		pcAssertWork(t, q, "READY")
		if pcCount(t, q, "payments.facts", " AND kind='CAPTURED'") != 1 {
			t.Fatal("capture fact not unique")
		}
	}
	if calls.Load() != 2 {
		t.Fatalf("Stripe retrieve calls=%d want 2", calls.Load())
	}
	pwStopClient(t, client)
	running = false
	// Redeliver only this fixture's durable reconcile jobs across a client
	// restart. The second real worker pass must be observable in River attempts.
	var beforeAttempts [2]int
	for i, id := range reconcileIDs {
		if err := f.owner.QueryRow(context.Background(), `SELECT attempt FROM river_payment.river_job WHERE id=$1`, id).Scan(&beforeAttempts[i]); err != nil {
			t.Fatal(err)
		}
		mustExec(t, f.owner, `UPDATE river_payment.river_job SET state='available',finalized_at=NULL,scheduled_at=clock_timestamp() WHERE id=$1`, id)
	}
	restarted, err := payments.NewPaymentWorkerClient(context.Background(), e.pool, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := restarted.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	for i, q := range []pqFixture{first, second} {
		pwAwaitJob(t, q, reconcileIDs[i], "completed")
		var attempts int
		if err := f.owner.QueryRow(context.Background(), `SELECT attempt FROM river_payment.river_job WHERE id=$1`, reconcileIDs[i]).Scan(&attempts); err != nil || attempts <= beforeAttempts[i] {
			t.Fatalf("restarted client did not replay reconcile: before=%d after=%d err=%v", beforeAttempts[i], attempts, err)
		}
	}
	pwStopClient(t, restarted)
	for _, q := range []pqFixture{first, second} {
		if pcCount(t, q, "payments.facts", " AND kind='CAPTURED'") != 1 || pcCount(t, q, "fulfillment.payment_work_items", "") != 1 {
			t.Fatal("restart duplicated money or fulfillment")
		}
	}
	if pwQueueIn(t, f.owner, "river_expiry.river_job", expiryID) != "checkout_expiry_v1" || pwQueueIn(t, f.owner, "river.river_job", externalID) != "default" || pwJobExceptQueueIn(t, f.owner, "river_expiry.river_job", expiryID) != expiryBefore || pwJobExceptQueueIn(t, f.owner, "river.river_job", externalID) != externalBefore {
		t.Fatal("payment client changed actual checkout expiry or external operation jobs")
	}
}

func TestBuyerPaymentWorkerLateDefaultInsertPollAndProfileIsolation(t *testing.T) {
	rm2AssertRemovedWorker(t, false)
}

func TestBuyerPaymentWorkerProcessSignalAndPoolCleanup(t *testing.T) {
	f := pwIsolatedFixture(t)
	binary := filepath.Join(t.TempDir(), "payment-worker")
	build := exec.Command("go", "build", "-o", binary, "../../cmd/payment-worker")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build real payment-worker binary: %v %s", err, out)
	}
	disabled := exec.Command(binary)
	disabled.Env = []string{"PATH=" + os.Getenv("PATH"), "COMMERCE_PAYMENT_WORKER_ENABLED=0", "COMMERCE_PAYMENT_WORKER_DATABASE_URL=invalid", "COMMERCE_ACCOUNT_KEYS_JSON=invalid"}
	if out, err := disabled.CombinedOutput(); err != nil || len(out) != 0 {
		t.Fatalf("disabled process opened resources or failed: %v %q", err, out)
	}
	invalid := exec.Command(binary)
	invalid.Env = []string{"PATH=" + os.Getenv("PATH"), "COMMERCE_PAYMENT_WORKER_ENABLED=1", "COMMERCE_PAYMENT_WORKER_PROFILE=PROVIDER_MOCK", "COMMERCE_PAYMENT_WORKER_DATABASE_URL=invalid"}
	if out, err := invalid.CombinedOutput(); err == nil || !strings.Contains(string(out), "payment_worker_invalid_config") || strings.Contains(string(out), "invalid\n") {
		t.Fatalf("production binary accepted MOCK or leaked config: %v %q", err, out)
	}
	role := bcRole(t, f, waPayment)
	u, err := url.Parse(role)
	if err != nil {
		t.Fatal(err)
	}
	app := "pw_binary_" + t04Tag()
	params := u.Query()
	params.Set("application_name", app)
	u.RawQuery = params.Encode()
	key := base64.StdEncoding.EncodeToString(randomBytes(32))
	keysJSON, err := json.Marshal([]map[string]string{{"id": "query_test", "key_base64": key}})
	if err != nil {
		t.Fatal(err)
	}
	env := []string{"PATH=" + os.Getenv("PATH"), "COMMERCE_PAYMENT_WORKER_ENABLED=1", "COMMERCE_PAYMENT_WORKER_PROFILE=SANDBOX", "COMMERCE_PAYMENT_WORKER_CONCURRENCY=1", "COMMERCE_PAYMENT_WORKER_DATABASE_URL=" + u.String(), "COMMERCE_ACCOUNT_ACTIVE_KEY_ID=query_test", "COMMERCE_ACCOUNT_KEYS_JSON=" + string(keysJSON), "COMMERCE_ACCOUNT_REPLAY_KEY=" + base64.StdEncoding.EncodeToString(randomBytes(32))}
	cmd := exec.Command(binary)
	cmd.Env = env
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
			if strings.HasSuffix(scan.Text(), " INFO payment_worker_ready") {
				select {
				case ready <- struct{}{}:
				default:
				}
			}
		}
	}()
	waited := false
	defer func() {
		if !waited {
			_ = cmd.Process.Kill()
			select {
			case <-readerDone:
			case <-time.After(3 * time.Second):
			}
			_ = cmd.Wait()
		}
	}()
	select {
	case <-ready:
	case <-readerDone:
		err := cmd.Wait()
		waited = true
		t.Fatalf("payment-worker exited before ready marker: %v", err)
	case <-time.After(8 * time.Second):
		t.Fatal("payment-worker did not report ready after River start")
	}
	deadline := time.Now().Add(8 * time.Second)
	seen := false
	for time.Now().Before(deadline) {
		var count int
		var queueStarted bool
		if err := f.owner.QueryRow(context.Background(), `SELECT
		 (SELECT count(*) FROM pg_stat_activity WHERE datname='lc_foundation_test' AND application_name=$1),
			 EXISTS(SELECT 1 FROM river_payment.river_queue WHERE name='payment_sandbox_v1')`, app).Scan(&count, &queueStarted); err != nil {
			t.Fatal(err)
		}
		if count > 0 && queueStarted {
			seen = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !seen {
		t.Fatal("ready process has no SANDBOX River producer or worker pool")
	}
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case <-readerDone:
		err := cmd.Wait()
		waited = true
		if err != nil {
			t.Fatalf("SIGTERM worker exit: %v", err)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("payment-worker did not stop after SIGTERM with no due jobs")
	}
	waitPoolsGone(t, f, "worker pool leaked connections", app)
}

func TestBuyerPaymentWorkerCrashChild(t *testing.T) {
	rm2AssertRemovedWorker(t, false)
}

func TestBuyerPaymentWorkerRealCrashAndRiverLeaseRescue(t *testing.T) {
	rm2AssertRemovedWorker(t, false)
}

func TestStripeWorkerCrashChild(t *testing.T) {
	if os.Getenv("LC_PW_CHILD") != "1" {
		return
	}
	ctx := context.Background()
	pool, err := platform.OpenWorkerPool(ctx, os.Getenv("LC_PW_CHILD_DSN"), platform.WorkerPayment)
	if err != nil {
		t.Fatal("child worker authority")
	}
	defer pool.Close()
	key, err := base64.StdEncoding.DecodeString(os.Getenv("LC_PW_CHILD_KEY"))
	if err != nil {
		t.Fatal("child key format")
	}
	replay, err := base64.StdEncoding.DecodeString(os.Getenv("LC_PW_CHILD_REPLAY"))
	if err != nil {
		t.Fatal("child replay key format")
	}
	keys, err := accounts.NewKeyring("query_test", map[string][]byte{"query_test": key}, replay)
	if err != nil {
		t.Fatal("child keyring")
	}
	opts := payments.DefaultQueryWorkerOptions()
	opts.MockTransport = pqTransport(func(r *http.Request) (*http.Response, error) {
		req, err := http.NewRequestWithContext(r.Context(), r.Method, os.Getenv("LC_PW_CHILD_MOCK_URL")+r.URL.RequestURI(), r.Body)
		if err != nil {
			return nil, err
		}
		req.Header = r.Header.Clone()
		return http.DefaultClient.Do(req)
	})
	rt, err := payments.NewStripeRuntime(ctx, pool, keys, "PROVIDER_MOCK", opts.MockTransport)
	if err != nil {
		t.Fatal("child Stripe runtime")
	}
	opts.MockTransport = pqNoNetwork()
	client, err := payments.NewPaymentWorkerClient(ctx, pool, payments.WorkerConfig{Profile: "PROVIDER_MOCK", Concurrency: 1, Query: opts, Keys: keys, Stripe: rt})
	if err != nil {
		t.Fatal("child client admission")
	}
	if err := client.Start(ctx); err != nil {
		t.Fatal("child client start")
	}
	select {} // parent kills this process; no Go cleanup runs
}

func TestStripeWorkerRealCrashAndRiverLeaseRescue(t *testing.T) {
	f := pwIsolatedFixture(t)
	key, replay := randomBytes(32), randomBytes(32)
	keys, err := accounts.NewKeyring("query_test", map[string][]byte{"query_test": key}, replay)
	if err != nil {
		t.Fatal(err)
	}
	e := sflNewOn(t, f, nil, keys)
	s := e.stripeStore(t)
	stop := e.start(t, true)
	result, session := e.pinned(t, s)
	stop()
	q := pqFixture{psHarness: s.p, keys: keys, result: result, schema: "river_payment", stripe: e, stripeStore: s}
	beforeReports := q.reportCount(t)
	if !e.fake.SetState(session, "complete", "paid") {
		t.Fatal("missing mock Stripe session")
	}
	q.pending(t)
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case entered <- struct{}{}:
		default:
		}
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	child := exec.Command(executable, "-test.run=^TestStripeWorkerCrashChild$", "-test.timeout=60s")
	child.Env = append(os.Environ(), "LC_PW_CHILD=1", "LC_PW_CHILD_DSN="+q.worker.Config().ConnString(),
		"LC_PW_CHILD_KEY="+base64.StdEncoding.EncodeToString(key), "LC_PW_CHILD_REPLAY="+base64.StdEncoding.EncodeToString(replay), "LC_PW_CHILD_MOCK_URL="+server.URL)
	child.Stdout, child.Stderr = io.Discard, io.Discard
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- child.Wait() }()
	reaped := false
	defer func() {
		if !reaped {
			_ = child.Process.Kill()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Error("crash child cleanup did not reap process")
			}
		}
	}()
	mustExec(t, f.owner, `UPDATE river_payment.river_job SET scheduled_at=clock_timestamp() WHERE id=$1`, q.result.JobID)
	select {
	case <-entered:
	case <-done:
		reaped = true
		t.Fatal("child exited before provider query boundary")
	case <-time.After(10 * time.Second):
		t.Fatal("child did not claim due payment query")
	}
	var state string
	var generation int64
	if err := f.owner.QueryRow(context.Background(), `SELECT j.state,o.generation FROM river_payment.river_job j JOIN integration.operations o ON o.id=$2 WHERE j.id=$1`, q.result.JobID, q.result.OperationID).Scan(&state, &generation); err != nil || state != "running" || generation < 1 {
		t.Fatalf("crash boundary was not leased: state=%s generation=%d err=%v", state, generation, err)
	}
	if err := child.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		reaped = true
		if err == nil {
			t.Fatal("killed child exited successfully")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("killed child not reaped")
	}
	close(release)
	q.pending(t)
	if pcCount(t, q, "payments.facts", "") != 0 {
		t.Fatal("crashed query created money")
	}
	// Age only this owned lease and River running timestamp; River itself must
	// rescue the same row. No state rewrite or duplicate enqueue is used.
	mustExec(t, f.owner, `UPDATE integration.operations SET lease_until=clock_timestamp()-interval '1 second' WHERE id=$1`, q.result.OperationID)
	mustExec(t, f.owner, `UPDATE river_payment.river_job SET attempted_at=clock_timestamp()-interval '2 hours' WHERE id=$1 AND state='running'`, q.result.JobID)
	var calls, retrieves atomic.Int32
	opts := payments.DefaultQueryWorkerOptions()
	opts.RetryDelay = time.Second // bounded fixture rescue, using River's own scheduler
	transport := e.fake.Transport()
	rt, err := payments.NewStripeRuntime(context.Background(), q.worker, keys, "PROVIDER_MOCK", pqTransport(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/"+session) {
			retrieves.Add(1)
		}
		return transport.RoundTrip(r)
	}))
	if err != nil {
		t.Fatal(err)
	}
	opts.MockTransport = pqNoNetwork()
	client, err := payments.NewPaymentWorkerClient(context.Background(), q.worker, payments.WorkerConfig{Profile: "PROVIDER_MOCK", Concurrency: 1, Query: opts, Keys: keys, Stripe: rt})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer pwStopClient(t, client)
	// Observe River's durable rescue marker: retryable is transient and may
	// already have advanced before this poll. No job state or due time is
	// manually rewritten after the crash.
	rescueDeadline := time.Now().Add(40 * time.Second)
	rescued := false
	for time.Now().Before(rescueDeadline) {
		if err := f.owner.QueryRow(context.Background(), `SELECT coalesce(errors::text LIKE '%Stuck job rescued by JobRescuer%',false) FROM river_payment.river_job WHERE id=$1`, q.result.JobID).Scan(&rescued); err != nil {
			t.Fatal(err)
		}
		if rescued {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !rescued {
		t.Fatal("River did not rescue the crashed running job")
	}
	deadline := time.Now().Add(12 * time.Second)
	captured, work := 0, 0
	for time.Now().Before(deadline) {
		if err := f.owner.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM payments.facts WHERE attempt_id=$1 AND kind='CAPTURED'),(SELECT count(*) FROM fulfillment.payment_work_items WHERE attempt_id=$1)`, q.result.AttemptID).Scan(&captured, &work); err != nil {
			t.Fatal(err)
		}
		if captured == 1 && work == 1 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if captured != 1 || work != 1 {
		var jobState string
		var jobAttempt int
		var scheduled time.Time
		var rescued bool
		var finalGeneration int64
		if err := f.owner.QueryRow(context.Background(), `SELECT j.state,j.attempt,j.scheduled_at,j.errors::text LIKE '%Stuck job rescued by JobRescuer%',o.generation FROM river_payment.river_job j JOIN integration.operations o ON o.id=$2 WHERE j.id=$1`, q.result.JobID, q.result.OperationID).Scan(&jobState, &jobAttempt, &scheduled, &rescued, &finalGeneration); err != nil {
			t.Fatal(err)
		}
		t.Fatalf("rescue stopped before capture: job=%s attempt=%d scheduled_in=%s rescued=%t operation_generation=%d calls=%d observations=%d", jobState, jobAttempt, time.Until(scheduled), rescued, finalGeneration, calls.Load(), q.reportCount(t))
	}
	// Freeze the accepted recovery before checking counts; the query job snoozes
	// after its paid observation and must not add a second provider step.
	pwStopClient(t, client)
	pcAssertStock(t, q, 0, 2, "CONFIRMED", "COMMITTED")
	pcAssertWork(t, q, "READY")
	// One Stripe query step is two transport calls: GET /v1/account (clientForClaim)
	// then GET /v1/checkout/sessions/{id} (stripeRetrieve). Exactly one step ran.
	if calls.Load() != 2 || retrieves.Load() != 1 || q.reportCount(t) != beforeReports+1 || pcCount(t, q, "payments.facts", " AND kind='CAPTURED'") != 1 {
		t.Fatalf("rescue transport/retrieve/report/capture counts: %d/%d/%d/%d", calls.Load(), retrieves.Load(), q.reportCount(t), pcCount(t, q, "payments.facts", " AND kind='CAPTURED'"))
	}
	var finalGeneration int64
	var finalAttempt int
	if err := f.owner.QueryRow(context.Background(), `SELECT o.generation,j.attempt FROM river_payment.river_job j JOIN integration.operations o ON o.id=$2 WHERE j.id=$1`, q.result.JobID, q.result.OperationID).Scan(&finalGeneration, &finalAttempt); err != nil || finalGeneration <= generation || finalAttempt < 1 {
		t.Fatalf("same job was not retried after lease recovery: generation=%d→%d attempt=%d err=%v", generation, finalGeneration, finalAttempt, err)
	}
}
