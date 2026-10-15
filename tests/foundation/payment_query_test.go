package foundation_test

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"livecommerce/internal/checkout"
	"livecommerce/internal/command"
	"livecommerce/internal/integrations/accounts"
	integration "livecommerce/internal/integrations/core"
	"livecommerce/internal/payments"
	"livecommerce/internal/platform"
	"livecommerce/internal/storefront"
)

type pqFixture struct {
	psHarness
	keys            *accounts.Keyring
	accountService  *accounts.Service
	result          checkout.PaymentResult
	schema          string
	historySnapshot string
	outbound        *atomic.Int32
	stripe          *sflEnv
	stripeStore     sstStore
}

var pqOldSecret = accounts.Credentials{HashKey: strings.Repeat("K", 32), HashIV: strings.Repeat("V", 16)}

func pqSetup(t *testing.T) pqFixture {
	return pqSetupSession(t, false)
}
func pqSetupWorker(t *testing.T) pqFixture {
	// Real River fetch uses its immutable fixed family queue. Give each worker
	// scenario its own cluster so other unit fixtures cannot supply due jobs.
	return pqSetupItemsOn(t, pwIsolatedFixture(t), nil, false, 1)
}
func pqSetupSession(t *testing.T, freshSession bool) pqFixture {
	return pqSetupItems(t, freshSession, 1)
}
func pqSetupItems(t *testing.T, freshSession bool, skuCount int) pqFixture {
	return pqSetupItemsOn(t, fixture(t), nil, freshSession, skuCount)
}

// Optional shared keys model the one runtime keyring serving multiple tenants;
// nil retains the existing single-tenant fixture's independently generated keys.
func pqSetupItemsOn(t *testing.T, base *testFixture, keys *accounts.Keyring, freshSession bool, skuCount int, historical ...string) pqFixture {
	t.Helper()
	p := psSetupItemsOn(t, base, skuCount, historical...)
	var e error
	if keys == nil {
		keys, e = accounts.NewKeyring("query_test", map[string][]byte{"query_test": randomBytes(32)}, randomBytes(32))
		if e != nil {
			t.Fatal(e)
		}
	}
	service, e := accounts.New(keys, &integration.Service{})
	if e != nil {
		t.Fatal(e)
	}
	schema := "river_payment"
	if len(historical) == 1 {
		schema = "river"
	}
	q := pqFixture{psHarness: p, keys: keys, accountService: service, schema: schema}
	if freshSession {
		q.cap.Token, q.cap.Scope.SessionID = randomToken(), randomUUID()
		mustExec(t, p.f.owner, `INSERT INTO buyer.capability_sessions(tenant_id,store_id,owner_id,id,token_hash,expires_at) VALUES($1,$2,$3,$4,$5,clock_timestamp()+interval '1 hour')`, q.cap.Scope.TenantID, q.cap.Scope.StoreID, q.cap.Scope.OwnerID, q.cap.Scope.SessionID, tokenHash(q.cap.Token))
	}
	q.result = rm2HistoryIn(t, q.psHarness, schema, "PROVIDER_MOCK", "payment_mock_v1")
	return q
}
func (q pqFixture) claim(t *testing.T) integration.ClaimResult {
	t.Helper()
	ctx := context.Background()
	tx, e := q.worker.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	c, e := (&integration.Service{}).Claim(ctx, tx, q.result.OperationID, 30)
	if e != nil || c.Disposition != "claimed" {
		t.Fatalf("query claim %s %v", c.Disposition, e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	return c
}
func pqReport(q pqFixture) map[string]any {
	return map[string]any{"MerTradeNo": q.result.MerchantTradeNo, "TradeNo": "test_trade_" + strings.ReplaceAll(q.result.AttemptID, "-", ""), "AmountTWD": q.result.AmountMinor / 100, "PaymentType": "1", "TradeStatus": "1", "Status": "SUCCESS", "AuthType": "1", "CardInst": 0, "DataSource": "A", "CloseStatus": "9"}
}
func pqJSON(v any) []byte { b, _ := json.Marshal(v); return b }
func (q pqFixture) record(c integration.ClaimResult, report any) error {
	return pqRecordIn(q.worker, q.schema, q.result.OperationID, c, "PROVIDER_MOCK", report)
}

// Like the real query worker, these protocol probes persist the observation and
// its reconciliation job in one transaction. A failed SQL fence rolls back both.
func pqRecord(pool *pgxpool.Pool, operation string, c integration.ClaimResult, profile string, report any) error {
	return pqRecordIn(pool, "river_payment", operation, c, profile, report)
}

func pqRecordIn(pool *pgxpool.Pool, schema, operation string, c integration.ClaimResult, profile string, report any) error {
	ctx := context.Background()
	tx, e := pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var hash string
	if e = tx.QueryRow(ctx, `SELECT encode(sha256(convert_to($1::jsonb::text,'UTF8')),'hex')`, pqJSON(report)).Scan(&hash); e != nil {
		return e
	}
	jobs, e := river.NewClient(riverpgxv5.New(pool), &river.Config{Schema: schema})
	if e != nil {
		return e
	}
	job, e := jobs.InsertTx(ctx, tx, pcArgs{OperationID: operation, ReportHash: hash, Version: 1}, nil)
	if e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, `SELECT integration.record_payment_query($1,$2,$3,$4,$5::jsonb,$6)`, operation, c.Generation, c.LeaseToken, profile, pqJSON(report), job.Job.ID); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
func (q pqFixture) reportCount(t *testing.T) int {
	t.Helper()
	var n int
	if e := q.f.owner.QueryRow(context.Background(), `SELECT count(*) FROM payments.provider_observations WHERE attempt_id=$1`, q.result.AttemptID).Scan(&n); e != nil {
		t.Fatal(e)
	}
	return n
}
func (q pqFixture) pending(t *testing.T) {
	t.Helper()
	var state, reservation, attempt string
	var reserved, allocated, onHand int64
	e := q.f.owner.QueryRow(context.Background(), `SELECT o.commercial_state,r.state,a.state,b.reserved,b.allocated,b.on_hand FROM checkout.orders o JOIN inventory.reservations r ON r.id=o.id JOIN checkout.payment_attempts a ON a.order_id=o.id JOIN inventory.reservation_lines l ON l.reservation_id=r.id JOIN inventory.balances b ON b.tenant_id=l.tenant_id AND b.store_id=l.store_id AND b.warehouse_id=l.warehouse_id AND b.sku_id=l.sku_id WHERE o.id=$1`, q.hold.OrderID).Scan(&state, &reservation, &attempt, &reserved, &allocated, &onHand)
	if e != nil || state != "AWAITING_PAYMENT" || reservation != "PAYMENT_PENDING" || attempt != "PAYMENT_PENDING" || reserved != 2 || allocated != 0 || onHand != 10 {
		t.Fatalf("query changed payment/stock: %s %s %s %d/%d/%d %v", state, reservation, attempt, reserved, allocated, onHand, e)
	}
}

type pqTransport func(*http.Request) (*http.Response, error)

func (f pqTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func pqNoNetwork() http.RoundTripper {
	return pqTransport(func(*http.Request) (*http.Response, error) { return nil, errors.New("test_network_forbidden") })
}

func TestBuyerPaymentQueryHistoricalCredentialAndACL(t *testing.T) {
	p := psSetup(t)
	q := pqFixture{psHarness: p, result: rm2HistoricalAttempt(t, p), schema: "river_payment"}
	mustExec(t, q.f.owner, `UPDATE integration.bindings SET enabled=false,semantic_version=semantic_version+1 WHERE id=$1`, q.binding)
	qualExec(t, q.f.owner, `UPDATE payments.account_qualifications SET revoked_at=clock_timestamp() WHERE id=$1`, q.proof)
	ctx := context.Background()
	tx, e := q.worker.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	// The Go material loader was removed. A historical row cannot be restarted or dispatched.
	if _, e = q.starter.StartPayment(ctx, q.cap.Token, q.f.storeA1, t04Key("pq-removed"), q.input); !errors.Is(e, command.ErrInvalid) {
		t.Fatalf("removed historical attempt could restart: %v", e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	var loadOID, guardOID, recordOID, credentialOID, observationOID uint32
	if e = q.f.owner.QueryRow(ctx, `SELECT 'integration.load_payment_query(uuid,bigint,bytea,text)'::regprocedure::oid,'integration.require_payment_query(uuid,bigint,bytea,text)'::regprocedure::oid,'integration.record_payment_query(uuid,bigint,bytea,text,jsonb,bigint)'::regprocedure::oid,'integration.account_credentials'::regclass::oid,'payments.provider_observations'::regclass::oid`).Scan(&loadOID, &guardOID, &recordOID, &credentialOID, &observationOID); e != nil {
		t.Fatal(e)
	}
	for _, pool := range []*pgxpool.Pool{q.f.runtime, q.a.runtime, q.pool, q.worker} {
		for _, oid := range []uint32{loadOID, recordOID, guardOID} {
			var yes bool
			if e = pool.QueryRow(ctx, `SELECT has_function_privilege(current_user,$1::oid,'EXECUTE')`, oid).Scan(&yes); e != nil || yes != (pool == q.worker && oid != guardOID) {
				t.Fatalf("unexpected function privilege %v", e)
			}
		}
		var yes bool
		if e = pool.QueryRow(ctx, `SELECT has_column_privilege(current_user,$1::oid,'ciphertext','SELECT')`, credentialOID).Scan(&yes); e != nil || yes {
			t.Fatalf("ciphertext authority broadened %v", e)
		}
		for _, priv := range []string{"INSERT", "UPDATE", "DELETE"} {
			if e = pool.QueryRow(ctx, `SELECT has_table_privilege(current_user,$1::oid,$2)`, observationOID, priv).Scan(&yes); e != nil || yes {
				t.Fatalf("direct report write %s %v", priv, e)
			}
		}
	}
	q.pending(t)
}

func TestBuyerPaymentQueryReportAtomicDedupAndBinding(t *testing.T) {
	q := pqSQLSetup(t, false, 1)
	c := q.claim(t)
	report := pqReport(q)
	for _, field := range []string{"AmountTWD", "MerTradeNo", "TradeNo", "PaymentType", "AuthType", "DataSource", "CardInst", "extra"} {
		bad := pqReport(q)
		bad[field] = "wrong"
		if field == "TradeNo" {
			bad[field] = "invalid trade number"
		}
		if e := q.record(c, bad); e == nil {
			t.Fatalf("bad field accepted %s", field)
		}
	}
	for _, profile := range []string{"SANDBOX", "LIVE"} {
		if e := pqRecord(q.worker, q.result.OperationID, c, profile, report); e == nil {
			t.Fatal("profile mismatch accepted")
		}
	}
	badClaim := c
	badClaim.LeaseToken = randomBytes(32)
	if e := q.record(badClaim, report); e == nil {
		t.Fatal("forged token")
	}
	if q.reportCount(t) != 0 {
		t.Fatal("invalid report persisted")
	}
	// Pending signed responses may not have a provider reference yet. Preserve
	// them, then pin the first nonempty reference permanently for this attempt.
	pending := pqReport(q)
	pending["TradeNo"], pending["TradeStatus"] = "", "8"
	if e := q.record(c, pending); e != nil {
		t.Fatal(e)
	}
	c = q.claim(t)
	if e := q.record(c, report); e != nil {
		t.Fatal(e)
	}
	if e := q.record(c, report); e == nil {
		t.Fatal("cleared lease replay accepted")
	}
	c = q.claim(t)
	if e := q.record(c, report); e != nil {
		t.Fatal(e)
	}
	if q.reportCount(t) != 2 {
		t.Fatal("duplicate report not deduplicated")
	}
	c = q.claim(t)
	bad := pqReport(q)
	bad["TradeNo"] = "other_trade"
	if e := q.record(c, bad); e == nil {
		t.Fatal("pinned provider reference replaced")
	}
	report["DataSource"] = "B"
	report["TradeStatus"] = "8"
	if e := q.record(c, report); e != nil {
		t.Fatal(e)
	}
	if q.reportCount(t) != 3 {
		t.Fatal("new provider evidence erased prior evidence")
	}
	q.pending(t)
}

func TestBuyerPaymentQueryFaultAndLeaseWaitRollback(t *testing.T) {
	q := pqSQLSetup(t, false, 1)
	c := q.claim(t)
	name := "pq_wait_" + t04Tag()
	ctx := context.Background()
	pool, e := platform.OpenWorkerPool(ctx, withApplicationName(t, bcRole(t, q.f, waPayment), name), platform.WorkerPayment)
	if e != nil {
		t.Fatal(e)
	}
	defer pool.Close()
	// Hold the event table after the report write, so the token expires during
	// Complete's final event insert. The enclosing SQL call must roll back both.
	holder, e := q.f.owner.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer holder.Rollback(ctx)
	if _, e = holder.Exec(ctx, `LOCK TABLE integration.operation_events IN ACCESS EXCLUSIVE MODE`); e != nil {
		t.Fatal(e)
	}
	var expiry time.Time
	if e = q.f.owner.QueryRow(ctx, `UPDATE integration.operations SET lease_until=clock_timestamp()+interval '600 milliseconds' WHERE id=$1 RETURNING lease_until`, q.result.OperationID).Scan(&expiry); e != nil {
		t.Fatal(e)
	}
	done := make(chan error, 1)
	go func() {
		done <- pqRecord(pool, q.result.OperationID, c, "PROVIDER_MOCK", pqReport(q))
	}()
	waitForDatabaseLock(t, q.f.owner, name)
	var before bool
	if e = q.f.owner.QueryRow(ctx, `SELECT clock_timestamp()<$1`, expiry).Scan(&before); e != nil || !before {
		t.Fatal("missed live-lease wait")
	}
	mustExec(t, q.f.owner, `SELECT pg_sleep(GREATEST(0,extract(epoch FROM $1::timestamptz-clock_timestamp()))+0.02)`, expiry)
	if e = holder.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	e = waitError(t, done)
	var pe *pgconn.PgError
	if !errors.As(e, &pe) || pe.Code != "40001" || q.reportCount(t) != 0 {
		t.Fatalf("late lease partial report: %v", e)
	}
	var code string
	var hasLease bool
	if e = q.f.owner.QueryRow(ctx, `SELECT result_code,lease_until IS NOT NULL FROM integration.operations WHERE id=$1`, q.result.OperationID).Scan(&code, &hasLease); e != nil || code == "payment_report_observed" || !hasLease {
		t.Fatal("failed report partially completed operation")
	}
	q.pending(t)
}

// A separate test implementation of the public PAYUNi GCM envelope. It uses
// synthetic keys and verifies the outbound query was encrypted with frozen v2,
// even after merchant rotation creates v3. It never connects to a socket.
func pqSignedResponse(report map[string]any) string {
	row := url.Values{"Status": {"SUCCESS"}}
	for k, v := range report {
		if k != "AmountTWD" && k != "Status" {
			switch k {
			case "CloseAmountTWD":
				k = "CloseAmt"
			case "CardRefundType":
				k = "RefundType"
			case "CardRefundStatus":
				k = "RefundStatus"
			case "CardRefundAmountTWD":
				k = "RefundAmt"
			case "CardRefundDay":
				k = "RefundDay"
			case "CardRemainAmountTWD":
				k = "RemainAmt"
			}
			row.Set("Result[0]["+k+"]", fmt.Sprint(v))
		}
	}
	row.Set("Result[0][TradeAmt]", fmt.Sprint(report["AmountTWD"]))
	row.Set("Result[0][Gateway]", "2")
	block, _ := aes.NewCipher([]byte(pqOldSecret.HashKey))
	gcm, _ := cipher.NewGCMWithNonceSize(block, 16)
	sealed := gcm.Seal(nil, []byte(pqOldSecret.HashIV), []byte(row.Encode()), nil)
	n := len(sealed) - 16
	wire := base64.StdEncoding.EncodeToString(sealed[:n]) + ":::" + base64.StdEncoding.EncodeToString(sealed[n:])
	encrypted := hex.EncodeToString([]byte(wire))
	hash := sha256.Sum256([]byte(pqOldSecret.HashKey + encrypted + pqOldSecret.HashIV))
	return string(pqJSON(map[string]string{"Status": "SUCCESS", "MerID": "mock-account", "Version": "2.0", "EncryptInfo": encrypted, "HashInfo": strings.ToUpper(hex.EncodeToString(hash[:]))}))
}

type pqArgs struct {
	OperationID string `json:"operation_id"`
	Version     int    `json:"version"`
}

func (pqArgs) Kind() string { return "payment_query_v1" }
func pqStartWorker(t *testing.T, q pqFixture, opts payments.QueryWorkerOptions) (*river.Client[pgx.Tx], func() error) {
	t.Helper()
	w, e := payments.NewQueryWorker(context.Background(), q.worker, q.keys, "PROVIDER_MOCK", opts)
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
	mustExec(t, q.f.owner, `UPDATE river_payment.river_job SET scheduled_at=clock_timestamp() WHERE id=$1`, q.result.JobID)
	if e = client.Start(context.Background()); e != nil {
		t.Fatal(e)
	}
	var stopped sync.Once
	var stopErr error
	stop := func() error {
		stopped.Do(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			stopErr = client.StopAndCancel(ctx)
		})
		return stopErr
	}
	t.Cleanup(func() {
		if e := stop(); e != nil {
			t.Errorf("query worker stop failed: %v", e)
		}
	})
	return client, stop
}
func pqAwait(t *testing.T, q pqFixture, jobID int64, wantCode, wantState string) time.Time {
	t.Helper()
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		var code, state string
		var when time.Time
		e := q.f.owner.QueryRow(context.Background(), `SELECT o.result_code,j.state,j.scheduled_at FROM integration.operations o JOIN river_payment.river_job j ON j.id=$2 WHERE o.id=$1`, q.result.OperationID, jobID).Scan(&code, &state, &when)
		if e != nil {
			t.Fatal(e)
		}
		if code == wantCode && state == wantState {
			return when
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("query worker did not persist expected result/queue state")
	return time.Time{}
}

// A formerly valid signed reply is no longer dispatch authority. The transport
// remains armed with that reply so restoring the adapter is a hard failure.
func TestBuyerPaymentQueryRealWorkerSignedHistoricalResponse(t *testing.T) {
	q := pqSetupWorker(t)
	mustExec(t, q.f.owner, `UPDATE integration.bindings SET enabled=false,semantic_version=semantic_version+1 WHERE id=$1`, q.binding)
	report := pqReport(q)
	report["DataSource"] = "B"
	body := pqSignedResponse(report)
	var calls atomic.Int32
	opts := payments.DefaultQueryWorkerOptions()
	opts.MockTransport = pqTransport(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	before := rm2Snapshot(t, q.psHarness, q.result.AttemptID)
	_, stop := pqStartWorker(t, q, opts)
	pwAwaitJob(t, q, q.result.JobID, "cancelled")
	if err := stop(); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 0 || q.reportCount(t) != 0 || rm2Snapshot(t, q.psHarness, q.result.AttemptID) != before {
		t.Fatal("retired signed query dispatched or changed history")
	}
	q.pending(t)
}

func TestBuyerPaymentQueryBudgetDurabilityAndNoSend(t *testing.T) {
	for _, mode := range []string{"age", "age_missing_key", "generation"} {
		t.Run(mode, func(t *testing.T) {
			q := pqSetupWorker(t)
			if mode == "generation" {
				mustExec(t, q.f.owner, `UPDATE integration.operations SET generation=1000 WHERE id=$1`, q.result.OperationID)
			} else {
				mustExec(t, q.f.owner, `UPDATE checkout.payment_attempts SET created_at=clock_timestamp()-interval '2 days' WHERE id=$1`, q.result.AttemptID)
			}
			if mode == "age_missing_key" {
				var e error
				q.keys, e = accounts.NewKeyring("unavailable", map[string][]byte{"unavailable": randomBytes(32)}, randomBytes(32))
				if e != nil {
					t.Fatal(e)
				}
			}
			var calls atomic.Int32
			opts := payments.DefaultQueryWorkerOptions()
			opts.MockTransport = pqTransport(func(*http.Request) (*http.Response, error) { calls.Add(1); return nil, errors.New("must_not_query") })
			before := rm2Snapshot(t, q.psHarness, q.result.AttemptID)
			pqStartWorker(t, q, opts)
			pwAwaitJob(t, q, q.result.JobID, "cancelled")
			// Owner-only redelivery of the same linked job: commit-time routing now
			// rejects a second orphan job ID before the worker could see it.
			mustExec(t, q.f.owner, `UPDATE river_payment.river_job SET state='available',finalized_at=NULL,scheduled_at=clock_timestamp() WHERE id=$1`, q.result.JobID)
			pwAwaitJob(t, q, q.result.JobID, "cancelled")
			if calls.Load() != 0 || q.reportCount(t) != 0 || rm2Snapshot(t, q.psHarness, q.result.AttemptID) != before {
				t.Fatal("budget retry queried provider")
			}
			q.pending(t)
		})
	}
}

func TestBuyerPaymentQueryProviderReferenceCannotFundTwoAttempts(t *testing.T) {
	q := pqSQLSetup(t, false, 1)
	firstReport := pqReport(q)
	if e := q.record(q.claim(t), firstReport); e != nil {
		t.Fatal(e)
	}
	second := q
	second.bcHarness.prepare(t, mustIssue(t, second.cqHarness.service, second.f.storeA1), []storefront.Item{{SKUID: second.stock.skus[0].ID, Quantity: 2}})
	var e error
	second.hold, e = second.bcHarness.begin(t04Key("pq-second-order"))
	if e != nil {
		t.Fatal(e)
	}
	second.input.OrderID = second.hold.OrderID
	second.result, e = psSQLStarter(t, second.pool, "PROVIDER_MOCK").StartPayment(context.Background(), second.cap.Token, second.f.storeA1, t04Key("pq-second-start"), second.input)
	if e != nil {
		t.Fatal(e)
	}
	report := pqReport(second)
	report["TradeNo"] = firstReport["TradeNo"]
	e = second.record(second.claim(t), report)
	var pe *pgconn.PgError
	if !errors.As(e, &pe) || pe.Code != "23505" {
		t.Fatalf("same provider trade reassigned %v", e)
	}
	if second.reportCount(t) != 0 || q.reportCount(t) != 1 {
		t.Fatal("conflict partially persisted report")
	}
}

func TestBuyerPaymentQueryWireFailuresStayPending(t *testing.T) {
	for _, kind := range []string{"amount", "merchant", "signature", "timeout", "panic", "missing_key", "aad"} {
		t.Run(kind, func(t *testing.T) {
			q := pqSetupWorker(t)
			report := pqReport(q)
			if kind == "amount" {
				report["AmountTWD"] = 26
			}
			body := pqSignedResponse(report)
			if kind == "merchant" {
				body = strings.Replace(body, "mock-account", "foreign-account", 1)
			}
			if kind == "signature" {
				var outer map[string]string
				_ = json.Unmarshal([]byte(body), &outer)
				outer["HashInfo"] = strings.Repeat("0", 64)
				body = string(pqJSON(outer))
			}
			if kind == "missing_key" {
				var e error
				q.keys, e = accounts.NewKeyring("other_key", map[string][]byte{"other_key": randomBytes(32)}, randomBytes(32))
				if e != nil {
					t.Fatal(e)
				}
			}
			if kind == "aad" {
				// Corrupt this disposable fixture's encrypted envelope; never real credentials.
				mustExec(t, q.f.owner, `UPDATE integration.account_credentials SET ciphertext=set_byte(ciphertext,0,(get_byte(ciphertext,0)+1)%256) WHERE connection_id=$1 AND version=1`, q.account)
			}
			var calls atomic.Int32
			opts := payments.DefaultQueryWorkerOptions()
			if kind == "timeout" {
				opts.CallTimeout = 100 * time.Millisecond
			}
			opts.MockTransport = pqTransport(func(r *http.Request) (*http.Response, error) {
				calls.Add(1)
				if kind == "timeout" {
					<-r.Context().Done()
					return nil, errors.New("SECRET_provider_sentinel")
				}
				if kind == "panic" {
					panic("SECRET_provider_sentinel")
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			})
			before := rm2Snapshot(t, q.psHarness, q.result.AttemptID)
			_, stop := pqStartWorker(t, q, opts)
			pwAwaitJob(t, q, q.result.JobID, "cancelled")
			if err := stop(); err != nil {
				t.Fatal(err)
			}
			if calls.Load() != 0 || q.reportCount(t) != 0 || rm2Snapshot(t, q.psHarness, q.result.AttemptID) != before {
				t.Fatal("removed wire matrix dispatched or changed history")
			}
			var jobErrors string
			if e := q.f.owner.QueryRow(context.Background(), `SELECT coalesce(errors::text,'') FROM river_payment.river_job WHERE id=$1`, q.result.JobID).Scan(&jobErrors); e != nil || strings.Contains(jobErrors, "SECRET_provider_sentinel") {
				t.Fatalf("unsafe job error %v", e)
			}
			q.pending(t)
		})
	}
}

func TestBuyerPaymentQueryFinishClockAndReferenceFence(t *testing.T) {
	q := pqSQLSetup(t, false, 1)
	c := q.claim(t)
	ctx := context.Background()
	if _, e := q.worker.Exec(ctx, `SELECT integration.finish_payment_query($1,$2,$3,'PROVIDER_MOCK','payment_query_timeout','forged-reference')`, q.result.OperationID, c.Generation, c.LeaseToken); e == nil {
		t.Fatal("error completion pinned provider reference")
	}
	if _, e := q.worker.Exec(ctx, `SELECT integration.finish_payment_query($1,$2,$3,'PROVIDER_MOCK','arbitrary_dynamic_code','')`, q.result.OperationID, c.Generation, c.LeaseToken); e == nil {
		t.Fatal("dynamic completion code accepted")
	}
	name := "pq_finish_" + t04Tag()
	pool, e := platform.OpenWorkerPool(ctx, withApplicationName(t, bcRole(t, q.f, waPayment), name), platform.WorkerPayment)
	if e != nil {
		t.Fatal(e)
	}
	defer pool.Close()
	holder, e := q.f.owner.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer holder.Rollback(ctx)
	if _, e = holder.Exec(ctx, `LOCK TABLE integration.operation_events IN ACCESS EXCLUSIVE MODE`); e != nil {
		t.Fatal(e)
	}
	var expiry time.Time
	if e = q.f.owner.QueryRow(ctx, `UPDATE integration.operations SET lease_until=clock_timestamp()+interval '600 milliseconds' WHERE id=$1 RETURNING lease_until`, q.result.OperationID).Scan(&expiry); e != nil {
		t.Fatal(e)
	}
	done := make(chan error, 1)
	go func() {
		_, e := pool.Exec(ctx, `SELECT integration.finish_payment_query($1,$2,$3,'PROVIDER_MOCK','payment_query_budget_exhausted','')`, q.result.OperationID, c.Generation, c.LeaseToken)
		done <- e
	}()
	waitForDatabaseLock(t, q.f.owner, name)
	var valid bool
	if e = q.f.owner.QueryRow(ctx, `SELECT clock_timestamp()<$1`, expiry).Scan(&valid); e != nil || !valid {
		t.Fatal("missed live finish wait")
	}
	mustExec(t, q.f.owner, `SELECT pg_sleep(GREATEST(0,extract(epoch FROM $1::timestamptz-clock_timestamp()))+0.02)`, expiry)
	if e = holder.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	e = waitError(t, done)
	var pe *pgconn.PgError
	if !errors.As(e, &pe) || pe.Code != "40001" {
		t.Fatalf("late finish accepted %v", e)
	}
	var code string
	if e = q.f.owner.QueryRow(ctx, `SELECT result_code FROM integration.operations WHERE id=$1`, q.result.OperationID).Scan(&code); e != nil || code == "payment_query_budget_exhausted" {
		t.Fatal("budget committed after expired lease")
	}
	q.pending(t)
}

func TestBuyerPaymentQueryCrossAttemptTokenAndMerchantJob(t *testing.T) {
	base := pwIsolatedFixture(t)
	q := pqSQLSetupOn(t, base, false, 1)
	other := pqSQLSetupOn(t, base, false, 1)
	c := q.claim(t)
	foreign := other.claim(t)
	if _, e := q.worker.Exec(context.Background(), `SELECT integration.load_payment_query($1,$2,$3,'PROVIDER_MOCK')`, q.result.OperationID, foreign.Generation, foreign.LeaseToken); e == nil {
		t.Fatal("foreign attempt lease opened credentials")
	}
	if _, e := q.worker.Exec(context.Background(), `SELECT integration.load_payment_query($1,$2,$3,'PROVIDER_MOCK')`, other.result.OperationID, c.Generation, c.LeaseToken); e == nil {
		t.Fatal("token selected other tenant")
	}
	f := newT06GoFixture(t, base)
	b := f.register(t, f.store, t04Key("pq-merchant"))
	op := f.plan(t, t04Key("pq-merchant-plan"), b, `{"amount":100}`)
	var calls atomic.Int32
	opts := payments.DefaultQueryWorkerOptions()
	opts.MockTransport = pqTransport(func(*http.Request) (*http.Response, error) { calls.Add(1); return nil, errors.New("must_not_query") })
	// Corrupt an admitted job only through the isolated fixture owner. The
	// deferred insert fence independently rejects new unlinked merchant jobs.
	// The second valid query stays future while this fixed-queue worker runs;
	// otherwise unrelated same-family work could call the MOCK transport.
	mustExec(t, q.f.owner, `UPDATE river_payment.river_job SET scheduled_at=clock_timestamp()+interval '1 hour' WHERE id=$1`, other.result.JobID)
	mustExec(t, q.f.owner, `ALTER TABLE river_payment.river_job DISABLE TRIGGER payment_job_family`)
	defer mustExec(t, q.f.owner, `ALTER TABLE river_payment.river_job ENABLE TRIGGER payment_job_family`)
	mustExec(t, q.f.owner, `UPDATE river_payment.river_job SET args=jsonb_build_object('operation_id',$2::text,'version',1) WHERE id=$1`, q.result.JobID, op.OperationID)
	mustExec(t, q.f.owner, `ALTER TABLE river_payment.river_job ENABLE TRIGGER payment_job_family`)
	pqStartWorker(t, q, opts)
	var e error
	// Wait for the worker's first fetch of the corrupted job (an eventual condition, not a latency bound). 5 s was too short on a
	// loaded CI runner (trunk 1991a49d, shard B-K: cancelled=false with every other assertion holding); the assertions are unchanged.
	deadline := time.Now().Add(30 * time.Second)
	cancelled := false
	for time.Now().Before(deadline) {
		var state string
		if e = f.base.owner.QueryRow(context.Background(), `SELECT state FROM river_payment.river_job WHERE id=$1`, q.result.JobID).Scan(&state); e != nil {
			t.Fatal(e)
		}
		if state == "cancelled" {
			cancelled = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	var state string
	var generation int64
	if e = f.base.owner.QueryRow(context.Background(), `SELECT state,generation FROM integration.operations WHERE id=$1`, op.OperationID).Scan(&state, &generation); e != nil || !cancelled || state != "READY" || generation != 0 || calls.Load() != 0 {
		t.Fatalf("merchant family affected err=%v cancelled=%v state=%s generation=%d provider_calls=%d", e, cancelled, state, generation, calls.Load())
	}
}

func TestBuyerPaymentQueryConcurrentReportSingleWinner(t *testing.T) {
	q := pqSQLSetup(t, false, 1)
	claim := q.claim(t)
	report := pqReport(q)
	start := make(chan struct{})
	done := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { <-start; done <- q.record(claim, report) }()
	}
	close(start)
	success := 0
	conflict := 0
	for i := 0; i < 2; i++ {
		e := waitError(t, done)
		if e == nil {
			success++
			continue
		}
		var pe *pgconn.PgError
		if errors.As(e, &pe) && pe.Code == "40001" {
			conflict++
		} else {
			t.Fatalf("unexpected competing report error %v", e)
		}
	}
	if success != 1 || conflict != 1 || q.reportCount(t) != 1 {
		t.Fatalf("report race successes%d conflicts%d", success, conflict)
	}
	q.pending(t)
}

// Removal must cancel before any provider IO/lease; Stripe cancellation/recovery
// stays covered by TestStripeQueryCancelledIOKeepsRecoverableLease.
func TestBuyerPaymentQueryCancelledIOKeepsRecoverableLease(t *testing.T) {
	q := pqSetupWorker(t)
	var calls atomic.Int32
	opts := payments.DefaultQueryWorkerOptions()
	opts.MockTransport = pqTransport(func(r *http.Request) (*http.Response, error) { calls.Add(1); return nil, r.Context().Err() })
	before := rm2Snapshot(t, q.psHarness, q.result.AttemptID)
	client, _ := pqStartWorker(t, q, opts)
	pwAwaitJob(t, q, q.result.JobID, "cancelled")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.StopAndCancel(ctx); err != nil {
		t.Fatal("worker cancellation did not stop")
	}
	var state string
	var hasLease bool
	if err := q.f.owner.QueryRow(context.Background(), `SELECT state,lease_until IS NOT NULL FROM integration.operations WHERE id=$1`, q.result.OperationID).Scan(&state, &hasLease); err != nil || state != "UNKNOWN" || hasLease || q.reportCount(t) != 0 || calls.Load() != 0 || rm2Snapshot(t, q.psHarness, q.result.AttemptID) != before {
		t.Fatalf("removed query claimed, sent or mutated: state=%s lease=%v calls=%d err=%v", state, hasLease, calls.Load(), err)
	}
	q.pending(t)
}

// Active-provider replacement for the parent-context/lease assertion above.
// No detached Stripe request may outlive StopAndCancel, and a cancelled query
// must keep recoverable UNKNOWN state without inventing a financial observation.
func TestStripeQueryCancelledIOKeepsRecoverableLease(t *testing.T) {
	q := rm2StripePending(t)
	e := q.stripe
	session := e.session(t, q.result.AttemptID)
	beforeReports := q.reportCount(t)
	entered, stopped := make(chan struct{}, 1), make(chan struct{}, 1)
	transport := e.fake.Transport()
	rt, err := payments.NewStripeRuntime(context.Background(), e.pool, e.keys, "PROVIDER_MOCK", pqTransport(func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/"+session) {
			entered <- struct{}{}
			<-r.Context().Done()
			stopped <- struct{}{}
			return nil, r.Context().Err()
		}
		return transport.RoundTrip(r)
	}))
	if err != nil {
		t.Fatal(err)
	}
	client, err := payments.NewPaymentWorkerClient(context.Background(), e.pool, payments.WorkerConfig{Profile: "PROVIDER_MOCK", Concurrency: 1, Query: e.opts, Keys: e.keys, Stripe: rt})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pwStopClient(t, client) })
	sstWake(t, e.f, q.result.AttemptID)
	if err = client.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("Stripe query never entered wire")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err = client.StopAndCancel(ctx); err != nil {
		t.Fatal("Stripe worker cancellation did not stop", err)
	}
	select {
	case <-stopped:
	default:
		t.Fatal("Stripe IO detached from parent cancellation")
	}
	var state string
	var hasLease bool
	if err = q.f.owner.QueryRow(context.Background(), `SELECT state,lease_until IS NOT NULL FROM integration.operations WHERE id=$1`, q.result.OperationID).Scan(&state, &hasLease); err != nil || state != "UNKNOWN" || !hasLease || q.reportCount(t) != beforeReports {
		t.Fatalf("cancelled Stripe query lost recovery state: %s %v %v", state, hasLease, err)
	}
	if pcCount(t, q, "payments.facts", "") != 0 || pcCount(t, q, "fulfillment.payment_work_items", "") != 0 {
		t.Fatal("cancelled Stripe query invented money/work")
	}
	pcAssertStock(t, q, 2, 0, "AWAITING_PAYMENT", "PAYMENT_PENDING")
}
