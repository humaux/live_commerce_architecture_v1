// Purpose: explicit refusals for removed PAYUNi Go entry points.
// Depends on: existing archive fixtures (rm2HistoryIn, rm2SeedReport) and fake transports.
// Used by: go test ./tests/foundation (CI full suite).
// Invariants: zero outbound calls and byte-identical full scoped business rows;
// only River's independently asserted cancellation acknowledgement may change.
package foundation_test

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/riverqueue/river"
	"livecommerce/internal/checkout"
	"livecommerce/internal/command"
	"livecommerce/internal/integrations/accounts"
	"livecommerce/internal/payments"
)

// Include every scoped row, not just the expected attempt ID: an accidentally
// created second attempt/operation/job must be visible as well as row mutations.
func rm2ScopeSnapshot(t *testing.T, p psHarness, jobs bool) string {
	t.Helper()
	ctx := context.Background()
	var rows []string
	for _, table := range []string{"checkout.orders", "checkout.payment_attempts", "checkout.command_results", "checkout.events",
		"inventory.reservations", "inventory.reservation_lines", "inventory.balances", "inventory.ledger",
		"integration.operations", "integration.operation_events", "integration.bindings", "integration.merchant_accounts", "integration.account_credentials",
		"payments.method_versions", "payments.method_heads", "payments.account_qualifications", "payments.provider_observations", "payments.facts", "payments.review_cases", "fulfillment.payment_work_items"} {
		var raw string
		if err := p.f.owner.QueryRow(ctx, `SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY to_jsonb(x)::text),'[]')::text FROM `+table+` x WHERE tenant_id=$1`, p.f.tenantA).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, table+":"+raw)
	}
	for _, query := range []string{
		`SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY to_jsonb(x)::text),'[]')::text FROM checkout.hosted_payment_pages x JOIN checkout.payment_attempts a ON a.id=x.attempt_id WHERE a.tenant_id=$1`,
		`SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY to_jsonb(x)::text),'[]')::text FROM payments.stripe_sessions x JOIN checkout.payment_attempts a ON a.id=x.attempt_id WHERE a.tenant_id=$1`,
	} {
		var raw string
		if err := p.f.owner.QueryRow(ctx, query, p.f.tenantA).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, raw)
	}
	if jobs {
		var raw string
		if err := p.f.owner.QueryRow(ctx, `SELECT coalesce(jsonb_agg(to_jsonb(j) ORDER BY j.id),'[]')::text FROM river_payment.river_job j`).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, raw)
	}
	return strings.Join(rows, "\n")
}

// The retired constructors have no transport argument. Intercept their default
// HTTP exit; permit only the exact loopback buyer server needed by HTTP probes.
func rm2OutboundCounter(t *testing.T, allowed string) *atomic.Int32 {
	t.Helper()
	var calls atomic.Int32
	previous := http.DefaultTransport
	host := ""
	if allowed != "" {
		u, err := url.Parse(allowed)
		if err != nil {
			t.Fatal(err)
		}
		host = u.Host
	}
	http.DefaultTransport = pqTransport(func(r *http.Request) (*http.Response, error) {
		if host != "" && r.URL.Scheme == "http" && r.URL.Host == host {
			return previous.RoundTrip(r)
		}
		calls.Add(1)
		return nil, errors.New("removed provider outbound forbidden")
	})
	t.Cleanup(func() { http.DefaultTransport = previous })
	return &calls
}

func rm2AssertRemovedHosted(t *testing.T, entry, locale string, wrongKeys bool) {
	t.Helper()
	h := hpSetup(t)
	calls := rm2OutboundCounter(t, "")
	if wrongKeys {
		keys, err := accounts.NewKeyring("missing", map[string][]byte{"missing": randomBytes(32)}, randomBytes(32))
		if err != nil {
			t.Fatal(err)
		}
		h.api = hpStarter(t, h.pool, "PROVIDER_MOCK", keys, h.config)
	}
	h.input.Locale = locale
	_ = rm2HistoricalAttempt(t, h.psHarness)
	before := rm2ScopeSnapshot(t, h.psHarness, true)
	for range 2 {
		if entry == "legacy" {
			out, err := h.starter.StartPayment(context.Background(), h.cap.Token, h.f.storeA1, t04Key("rm2-refuse-start"), h.psHarness.input)
			if !errors.Is(err, command.ErrInvalid) || out != (checkout.PaymentResult{}) {
				t.Fatal("retired StartPayment admitted", err)
			}
		} else if entry == "take" {
			out, err := h.take()
			if !errors.Is(err, command.ErrConflict) || out.Form != nil || out.RedirectURL != "" || out.OrderID != "" {
				t.Fatal("retired TakeHosted released", err)
			}
		} else {
			out, err := h.begin(t04Key("rm2-refuse-hosted"))
			if !errors.Is(err, command.ErrInvalid) || out != (checkout.PaymentResult{}) {
				t.Fatal("retired BeginHosted admitted", err)
			}
		}
	}
	if calls.Load() != 0 || rm2ScopeSnapshot(t, h.psHarness, true) != before {
		t.Fatal("retired entry made outbound call or changed full rows")
	}
}

func TestRemovePAYUNiStartPaymentRefusal(t *testing.T) {
	rm2AssertRemovedHosted(t, "legacy", "en", false)
}
func TestRemovePAYUNiTakeHostedRefusal(t *testing.T) { rm2AssertRemovedHosted(t, "take", "en", false) }

func rm2AssertRemovedHTTP(t *testing.T, entry, locale string) {
	t.Helper()
	h := bphSetup(t)
	_ = rm2HistoricalAttempt(t, h.psHarness)
	calls := rm2OutboundCounter(t, h.http.server.URL)
	before := rm2ScopeSnapshot(t, h.psHarness, true)
	var wg sync.WaitGroup
	if entry == "handoff" {
		results := make(chan bhResponse, 2)
		errs := make(chan error, 2)
		for range 2 {
			wg.Add(1)
			go func() { defer wg.Done(); r, e := bphConcurrentTake(h); results <- r; errs <- e }()
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
			bphFail(t, r, 409, "conflict")
		}
	} else {
		for range 2 {
			bhError(t, h.http.request(t, "POST", bphPath(h, "/prepare"), h.cap.Token, t04Key("rm2-http-refuse"), map[string]any{"method_code": "payuni_credit", "method_version": 1, "locale": locale}, nil), 422, "invalid_request")
		}
	}
	if calls.Load() != 0 || rm2ScopeSnapshot(t, h.psHarness, true) != before {
		t.Fatal("retired HTTP made outbound call or changed full rows")
	}
}

func rm2AssertRemovedWorker(t *testing.T, capture bool) {
	t.Helper()
	q := pqSetupWorker(t)
	if capture {
		rm2SeedReport(t, q, pcFull(q)) // PAY-RM2 2b: record_payment_query dropped, the stored report is seeded (mapping c)
	}
	before := rm2ScopeSnapshot(t, q.psHarness, false)
	var calls atomic.Int32
	opts := payments.DefaultQueryWorkerOptions()
	opts.MockTransport = pqTransport(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return nil, errors.New("removed worker outbound forbidden")
	})
	client, err := payments.NewWorkerClient(context.Background(), q.worker, q.keys, "PROVIDER_MOCK", 1, opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pwStopClient(t, client) })
	mustExec(t, q.f.owner, `UPDATE river_payment.river_job SET scheduled_at=clock_timestamp() WHERE args->>'operation_id'=$1`, q.result.AttemptID)
	if err = client.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	pwAwaitJob(t, q, q.result.JobID, "cancelled")
	if capture {
		var job int64
		if err = q.f.owner.QueryRow(context.Background(), `SELECT id FROM river_payment.river_job WHERE kind='payment_reconcile_v1' AND args->>'operation_id'=$1`, q.result.AttemptID).Scan(&job); err != nil {
			t.Fatal(err)
		}
		pwAwaitJob(t, q, job, "cancelled")
	}
	pwStopClient(t, client)
	if calls.Load() != 0 || rm2ScopeSnapshot(t, q.psHarness, false) != before {
		t.Fatal("retired worker dispatched or changed full business rows")
	}
}

// Keep the job type assertion explicit: refusal still traverses the real River
// worker registration, not a manually cancelled owner fixture.
var _ river.JobArgs = pcArgs{}
