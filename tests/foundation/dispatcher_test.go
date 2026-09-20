package foundation_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	integration "livecommerce/internal/integrations/core"
	"livecommerce/internal/platform"
)

func t06DispatchOptions() integration.DispatcherOptions {
	o := integration.DefaultDispatcherOptions()
	o.RetryDelay = 100 * time.Millisecond
	return o
}

type t06DuplicateOperationArgs struct {
	OperationID string `json:"operation_id"`
	Version     int    `json:"version"`
}

func (t06DuplicateOperationArgs) Kind() string { return "external_operation_v1" }

func t06Route() integration.DispatchRoute {
	return integration.DispatchRoute{
		Provider: "mock_provider", Action: "payment.authorize", Purpose: "transactional",
		Check: func(context.Context, integration.DispatchRequest) error { return nil },
		Dispatch: func(context.Context, integration.DispatchRequest) (integration.Outcome, error) {
			return integration.Outcome{State: "SUCCEEDED", Code: "mock_observed", ProviderReference: "mock-1"}, nil
		},
		Reconcile: func(context.Context, integration.DispatchRequest) (integration.Outcome, error) {
			return integration.Outcome{State: "UNKNOWN", Code: "mock_not_found"}, nil
		},
	}
}

func t06Queue(t *testing.T, f *t06GoFixture, p integration.PlanResult) string {
	t.Helper()
	queue := "dispatch_" + strings.ReplaceAll(randomUUID(), "-", "")
	// Isolate this Plan-created job from other fixtures before any worker starts.
	result, err := f.base.owner.Exec(context.Background(), `UPDATE river.river_job SET queue=$1 WHERE id=$2 AND state='available'`, queue, p.JobID)
	if err != nil || result.RowsAffected() != 1 {
		t.Fatal("isolate dispatcher fixture job")
	}
	return queue
}

func t06StartDispatcher(t *testing.T, pool *pgxpool.Pool, queue string, routes []integration.DispatchRoute, opts integration.DispatcherOptions) *river.Client[pgx.Tx] {
	t.Helper()
	worker, err := integration.NewDispatcher(context.Background(), pool, routes, opts)
	if err != nil {
		t.Fatal(err)
	}
	workers := river.NewWorkers()
	river.AddWorker(workers, worker)
	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Schema: "river", Workers: workers, Queues: map[string]river.QueueConfig{queue: {MaxWorkers: 2}},
		JobTimeout: 20 * time.Second, RescueStuckJobsAfter: 30 * time.Second,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := client.StopAndCancel(ctx); err != nil {
			t.Error("dispatcher stop did not finish")
		}
	})
	return client
}

func t06Await(t *testing.T, f *t06GoFixture, p integration.PlanResult, want, jobWant string, budget time.Duration) (code string) {
	t.Helper()
	deadline := time.Now().Add(budget)
	var state, jobState string
	for time.Now().Before(deadline) {
		err := f.worker.QueryRow(context.Background(), `SELECT o.state,o.result_code,j.state FROM integration.operations o JOIN river.river_job j ON j.id=$2 WHERE o.id=$1`, p.OperationID, p.JobID).Scan(&state, &code, &jobState)
		if err != nil {
			t.Fatal("read dispatcher fixture")
		}
		if state == want && jobState == jobWant {
			return code
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("dispatcher persisted %s/%s/%s, want %s/%s", state, code, jobState, want, jobWant)
	return code
}

func TestT06DispatcherCommittedFrozenIntentAndTerminalReplay(t *testing.T) {
	f := newT06GoFixture(t)
	b := f.register(t, f.store, "dispatch-binding")
	p := f.plan(t, "dispatch-success", b, `{"amount":9007199254740993}`)
	queue := t06Queue(t, f, p)
	var calls atomic.Int64
	route := t06Route()
	route.Check = func(ctx context.Context, in integration.DispatchRequest) error {
		// Independent connection must see the committed claim, with no row lock
		// held across policy/provider I/O. A dirty read or same transaction cannot pass.
		tx, err := f.base.owner.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(context.Background())
		var state string
		if err := tx.QueryRow(ctx, `SELECT state FROM integration.operations WHERE id=$1 FOR UPDATE NOWAIT`, in.OperationID).Scan(&state); err != nil {
			return err
		}
		if state != "DISPATCHING" {
			return errors.New("claim not committed")
		}
		for i := range in.Request {
			in.Request[i] = 'x'
		}
		return nil
	}
	route.Dispatch = func(_ context.Context, in integration.DispatchRequest) (integration.Outcome, error) {
		calls.Add(1)
		if in.OperationID != p.OperationID || in.BindingID != b.ID || in.ExternalAssetID != b.ExternalAssetID || in.PrincipalID != f.principal || in.IdempotencyKey != "lc:"+p.OperationID || !bytes.Contains(in.Request, []byte("9007199254740993")) {
			return integration.Outcome{}, errors.New("frozen request changed")
		}
		return integration.Outcome{State: "SUCCEEDED", Code: "mock_observed", ProviderReference: "mock-1"}, nil
	}
	client := t06StartDispatcher(t, f.worker, queue, []integration.DispatchRoute{route}, t06DispatchOptions())
	t06Await(t, f, p, "SUCCEEDED", "completed", 5*time.Second)
	// River administrative retry of a terminal job cannot authorize another effect.
	if _, err := client.JobRetry(context.Background(), p.JobID); err != nil {
		t.Fatal(err)
	}
	t06Await(t, f, p, "SUCCEEDED", "completed", 5*time.Second)
	if calls.Load() != 1 {
		t.Fatalf("terminal replay dispatched %d times", calls.Load())
	}
}

func TestT06DispatcherPolicyAndBindingChangesBlockFirstEffect(t *testing.T) {
	for _, revoke := range []bool{false, true} {
		name := "denied"
		if revoke {
			name = "binding_changed"
		}
		t.Run(name, func(t *testing.T) {
			f := newT06GoFixture(t)
			b := f.register(t, f.store, "dispatch-policy-binding")
			p := f.plan(t, "dispatch-policy-op", b, `{"amount":1}`)
			queue := t06Queue(t, f, p)
			var calls atomic.Int64
			route := t06Route()
			route.Check = func(ctx context.Context, _ integration.DispatchRequest) error {
				if !revoke {
					return integration.ErrPolicyDenied
				}
				_, err := f.base.owner.Exec(ctx, `UPDATE integration.bindings SET enabled=false,semantic_version=semantic_version+1 WHERE id=$1`, b.ID)
				return err
			}
			route.Dispatch = func(context.Context, integration.DispatchRequest) (integration.Outcome, error) {
				calls.Add(1)
				return integration.Outcome{State: "SUCCEEDED", Code: "unexpected"}, nil
			}
			t06StartDispatcher(t, f.worker, queue, []integration.DispatchRoute{route}, t06DispatchOptions())
			t06Await(t, f, p, "BLOCKED_POLICY", "completed", 5*time.Second)
			if calls.Load() != 0 {
				t.Fatal("denied dispatch emitted effect")
			}
		})
	}
}

func TestT06DispatcherBusyDuplicateSnoozesWithoutEffect(t *testing.T) {
	f := newT06GoFixture(t)
	b := f.register(t, f.store, "dispatch-busy-binding")
	p := f.plan(t, "dispatch-busy-op", b, `{"amount":1}`)
	queue := t06Queue(t, f, p)
	var sends atomic.Int64
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	route := t06Route()
	route.Dispatch = func(ctx context.Context, _ integration.DispatchRequest) (integration.Outcome, error) {
		sends.Add(1)
		select {
		case started <- struct{}{}:
		default:
		}
		select {
		case <-release:
			return integration.Outcome{State: "SUCCEEDED", Code: "mock_observed"}, nil
		case <-ctx.Done():
			return integration.Outcome{}, ctx.Err()
		}
	}
	client := t06StartDispatcher(t, f.worker, queue, []integration.DispatchRoute{route}, t06DispatchOptions())
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("first callback not started")
	}
	duplicate, err := client.Insert(context.Background(), t06DuplicateOperationArgs{OperationID: p.OperationID, Version: 1}, &river.InsertOpts{Queue: queue})
	if err != nil {
		t.Fatal(err)
	}
	var snoozes, generation int64
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err := f.worker.QueryRow(context.Background(), `SELECT COALESCE((j.metadata->>'snoozes')::bigint,0),o.generation FROM river.river_job j JOIN integration.operations o ON o.id=$2 WHERE j.id=$1`, duplicate.Job.ID, p.OperationID).Scan(&snoozes, &generation); err != nil {
			t.Fatal(err)
		}
		if snoozes > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if snoozes == 0 || generation != 1 || sends.Load() != 1 {
		t.Fatalf("busy duplicate not safely snoozed: snoozes=%d gen=%d sends=%d", snoozes, generation, sends.Load())
	}
	close(release)
	t06Await(t, f, p, "SUCCEEDED", "completed", 5*time.Second)
	t06Await(t, f, integration.PlanResult{OperationID: p.OperationID, JobID: duplicate.Job.ID}, "SUCCEEDED", "completed", 5*time.Second)
	if sends.Load() != 1 {
		t.Fatal("duplicate eventually repeated effect")
	}
}

func TestT06DispatcherAmbiguityReconcilesWithoutRepeating(t *testing.T) {
	for _, mode := range []string{"error", "panic", "malformed", "ack"} {
		t.Run(mode, func(t *testing.T) {
			f := newT06GoFixture(t)
			b := f.register(t, f.store, "dispatch-unknown-binding")
			p := f.plan(t, "dispatch-unknown-op", b, `{"amount":1}`)
			queue := t06Queue(t, f, p)
			var dispatches, queries atomic.Int64
			route := t06Route()
			route.Dispatch = func(context.Context, integration.DispatchRequest) (integration.Outcome, error) {
				dispatches.Add(1)
				switch mode {
				case "error":
					return integration.Outcome{}, errors.New("PRIVATE_PROVIDER_DETAIL")
				case "panic":
					panic("PRIVATE_PROVIDER_DETAIL")
				case "ack":
					return integration.Outcome{State: "ACKNOWLEDGED", Code: "accepted", ProviderReference: "mock-ack"}, nil
				default:
					return integration.Outcome{State: "SUCCEEDED", Code: "bad code PRIVATE_PROVIDER_DETAIL"}, nil
				}
			}
			route.Reconcile = func(_ context.Context, in integration.DispatchRequest) (integration.Outcome, error) {
				queries.Add(1)
				if in.IdempotencyKey != "lc:"+p.OperationID {
					return integration.Outcome{}, errors.New("key changed")
				}
				if mode == "ack" && in.ProviderReference != "mock-ack" {
					return integration.Outcome{}, errors.New("reference lost")
				}
				return integration.Outcome{State: "SUCCEEDED", Code: "queried", ProviderReference: "mock-final"}, nil
			}
			t06StartDispatcher(t, f.worker, queue, []integration.DispatchRoute{route}, t06DispatchOptions())
			t06Await(t, f, p, "SUCCEEDED", "completed", 8*time.Second)
			if dispatches.Load() != 1 || queries.Load() != 1 {
				t.Fatalf("effect/query counts=%d/%d", dispatches.Load(), queries.Load())
			}
			var private bool
			if err := f.worker.QueryRow(context.Background(), `SELECT EXISTS(SELECT 1 FROM river.river_job WHERE id=$1 AND errors::text LIKE '%PRIVATE_PROVIDER_DETAIL%') OR EXISTS(SELECT 1 FROM integration.operation_events WHERE operation_id=$2 AND reason_code LIKE '%PRIVATE_PROVIDER_DETAIL%')`, p.JobID, p.OperationID).Scan(&private); err != nil {
				t.Fatal(err)
			}
			if private {
				t.Fatal("raw provider error leaked to persistent records")
			}
		})
	}
}

func TestT06DispatcherOneDeadlineAndFreshCompletion(t *testing.T) {
	f := newT06GoFixture(t)
	b := f.register(t, f.store, "dispatch-timeout-binding")
	p := f.plan(t, "dispatch-timeout-op", b, `{"amount":1}`)
	queue := t06Queue(t, f, p)
	var deadline atomic.Int64
	var sends, queries atomic.Int64
	route := t06Route()
	route.Check = func(ctx context.Context, _ integration.DispatchRequest) error {
		d, ok := ctx.Deadline()
		if !ok {
			return errors.New("missing callback deadline")
		}
		deadline.Store(d.UnixNano())
		return nil
	}
	route.Dispatch = func(ctx context.Context, _ integration.DispatchRequest) (integration.Outcome, error) {
		sends.Add(1)
		d, ok := ctx.Deadline()
		if !ok || d.UnixNano() != deadline.Load() {
			return integration.Outcome{State: "FAILED_FINAL", Code: "deadline_reset"}, nil
		}
		<-ctx.Done()
		return integration.Outcome{}, ctx.Err()
	}
	route.Reconcile = func(context.Context, integration.DispatchRequest) (integration.Outcome, error) {
		queries.Add(1)
		return integration.Outcome{State: "SUCCEEDED", Code: "queried_after_timeout"}, nil
	}
	opts := t06DispatchOptions()
	opts.LeaseSeconds = 5
	opts.CallTimeout = 100 * time.Millisecond
	opts.DBTimeout = 200 * time.Millisecond
	t06StartDispatcher(t, f.worker, queue, []integration.DispatchRoute{route}, opts)
	t06Await(t, f, p, "SUCCEEDED", "completed", 8*time.Second)
	if sends.Load() != 1 || queries.Load() != 1 {
		t.Fatalf("timeout repeated send: %d/%d", sends.Load(), queries.Load())
	}
	var recorded bool
	if err := f.worker.QueryRow(context.Background(), `SELECT EXISTS(SELECT 1 FROM integration.operation_events WHERE operation_id=$1 AND state='UNKNOWN' AND reason_code='callback_timeout')`, p.OperationID).Scan(&recorded); err != nil {
		t.Fatal(err)
	}
	if !recorded {
		t.Fatal("callback timeout did not commit UNKNOWN with fresh completion context")
	}
}

func TestT06DispatcherUnknownRouteFailsClosedWithoutClaim(t *testing.T) {
	f := newT06GoFixture(t)
	b := f.register(t, f.store, "dispatch-route-binding")
	p := f.plan(t, "dispatch-route-op", b, `{"amount":1}`)
	queue := t06Queue(t, f, p)
	if _, err := f.base.owner.Exec(context.Background(), `UPDATE river.river_job SET max_attempts=1 WHERE id=$1`, p.JobID); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int64
	route := t06Route()
	route.Provider = "not_the_bound_provider"
	route.Check = func(context.Context, integration.DispatchRequest) error { calls.Add(1); return nil }
	t06StartDispatcher(t, f.worker, queue, []integration.DispatchRoute{route}, t06DispatchOptions())
	t06Await(t, f, p, "READY", "discarded", 5*time.Second)
	var generation int64
	if err := f.worker.QueryRow(context.Background(), `SELECT generation FROM integration.operations WHERE id=$1`, p.OperationID).Scan(&generation); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 0 || generation != 0 {
		t.Fatal("unknown provider used fallback or claimed")
	}
}

func TestT06DispatcherPolicyDeniedAfterPossibleEffectStaysUnknown(t *testing.T) {
	f := newT06GoFixture(t)
	b := f.register(t, f.store, "dispatch-late-policy-binding")
	p := f.plan(t, "dispatch-late-policy-op", b, `{"amount":1}`)
	queue := t06Queue(t, f, p)
	var checks, sends, queries atomic.Int64
	route := t06Route()
	route.Check = func(context.Context, integration.DispatchRequest) error {
		if checks.Add(1) > 1 {
			return integration.ErrPolicyDenied
		}
		return nil
	}
	route.Dispatch = func(context.Context, integration.DispatchRequest) (integration.Outcome, error) {
		sends.Add(1)
		return integration.Outcome{State: "ACKNOWLEDGED", Code: "accepted", ProviderReference: "accepted-before-revoke"}, nil
	}
	route.Reconcile = func(context.Context, integration.DispatchRequest) (integration.Outcome, error) {
		queries.Add(1)
		return integration.Outcome{State: "FAILED_FINAL", Code: "must_not_query"}, nil
	}
	opts := t06DispatchOptions()
	opts.MaxGenerations = 3
	t06StartDispatcher(t, f.worker, queue, []integration.DispatchRoute{route}, opts)
	t06Await(t, f, p, "UNKNOWN", "cancelled", 8*time.Second)
	var ref string
	var kept, erased bool
	if err := f.worker.QueryRow(context.Background(), `SELECT provider_reference,
		EXISTS(SELECT 1 FROM integration.operation_events WHERE operation_id=$1 AND generation=2 AND state='UNKNOWN' AND reason_code='policy_check_failed'),
		EXISTS(SELECT 1 FROM integration.operation_events WHERE operation_id=$1 AND state='BLOCKED_POLICY')
		FROM integration.operations WHERE id=$1`, p.OperationID).Scan(&ref, &kept, &erased); err != nil {
		t.Fatal(err)
	}
	if ref != "accepted-before-revoke" || !kept || erased || sends.Load() != 1 || queries.Load() != 0 {
		t.Fatal("late policy denial erased possible effect, reference or query gate")
	}
}

func TestT06DispatcherBudgetPersistsManualRequirement(t *testing.T) {
	f := newT06GoFixture(t)
	b := f.register(t, f.store, "dispatch-budget-binding")
	p := f.plan(t, "dispatch-budget-op", b, `{"amount":1}`)
	queue := t06Queue(t, f, p)
	var sends, queries atomic.Int64
	route := t06Route()
	route.Dispatch = func(context.Context, integration.DispatchRequest) (integration.Outcome, error) {
		sends.Add(1)
		return integration.Outcome{State: "ACKNOWLEDGED", Code: "accepted", ProviderReference: "keep-ref"}, nil
	}
	route.Reconcile = func(context.Context, integration.DispatchRequest) (integration.Outcome, error) {
		queries.Add(1)
		return integration.Outcome{}, errors.New("PRIVATE_PROVIDER_DETAIL")
	}
	opts := t06DispatchOptions()
	opts.MaxGenerations = 2
	t06StartDispatcher(t, f.worker, queue, []integration.DispatchRoute{route}, opts)
	code := t06Await(t, f, p, "UNKNOWN", "cancelled", 8*time.Second)
	if code != "reconcile_budget_exhausted" || sends.Load() != 1 || queries.Load() != 1 {
		t.Fatalf("bad bounded outcome code=%s sends=%d queries=%d", code, sends.Load(), queries.Load())
	}
	var ref string
	var count int
	if err := f.worker.QueryRow(context.Background(), `SELECT provider_reference,(SELECT count(*) FROM integration.operation_events WHERE operation_id=$1 AND reason_code='reconcile_budget_exhausted') FROM integration.operations WHERE id=$1`, p.OperationID).Scan(&ref, &count); err != nil {
		t.Fatal(err)
	}
	if ref != "keep-ref" || count != 1 {
		t.Fatal("manual requirement/reference not durable")
	}
}

func TestT06DispatcherFailedExhaustionCommitDoesNotCancelJob(t *testing.T) {
	f := newT06GoFixture(t)
	b := f.register(t, f.store, "dispatch-fault-binding")
	p := f.plan(t, "dispatch-fault-op", b, `{"amount":1}`)
	queue := t06Queue(t, f, p)
	var sends, queries atomic.Int64
	route := t06Route()
	route.Dispatch = func(context.Context, integration.DispatchRequest) (integration.Outcome, error) {
		sends.Add(1)
		return integration.Outcome{State: "UNKNOWN", Code: "mock_uncertain"}, nil
	}
	route.Reconcile = func(context.Context, integration.DispatchRequest) (integration.Outcome, error) {
		queries.Add(1)
		return integration.Outcome{State: "UNKNOWN", Code: "mock_uncertain"}, nil
	}
	// Local fixture only: reject the final event, so the SECURITY DEFINER
	// transaction must roll back state+event before River sees the failure.
	name := "t06_exhaust_" + strings.ReplaceAll(randomUUID(), "-", "")
	constraint := pgx.Identifier{name}.Sanitize()
	if _, err := f.base.owner.Exec(context.Background(), `ALTER TABLE integration.operation_events ADD CONSTRAINT `+constraint+` CHECK (reason_code <> 'reconcile_budget_exhausted') NOT VALID`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = f.base.owner.Exec(context.Background(), `ALTER TABLE integration.operation_events DROP CONSTRAINT IF EXISTS `+constraint)
	})
	opts := t06DispatchOptions()
	opts.MaxGenerations = 2
	opts.RetryDelay = 500 * time.Millisecond
	t06StartDispatcher(t, f.worker, queue, []integration.DispatchRoute{route}, opts)
	deadline := time.Now().Add(8 * time.Second)
	var jobState, opState, mode string
	var gen int64
	var observed, errorPersisted, partialCommit bool
	for time.Now().Before(deadline) {
		if err := f.worker.QueryRow(context.Background(), `SELECT o.state,o.generation,o.lease_mode,j.state,
			COALESCE(j.errors::text LIKE '%external_operation_completion_uncertain%',false),
			o.result_code='reconcile_budget_exhausted' OR EXISTS(SELECT 1 FROM integration.operation_events WHERE operation_id=o.id AND reason_code='reconcile_budget_exhausted')
			FROM integration.operations o JOIN river.river_job j ON j.id=$2 WHERE o.id=$1`, p.OperationID, p.JobID).Scan(&opState, &gen, &mode, &jobState, &errorPersisted, &partialCommit); err != nil {
			t.Fatal(err)
		}
		if jobState == "cancelled" || jobState == "completed" || jobState == "discarded" || partialCommit {
			t.Fatal("exhaustion fault lost retry or partially committed its fact")
		}
		// River v0.40 deliberately uses available (not retryable) when
		// retry delay is shorter than its scheduler interval. Assert the
		// persisted error+atomic ledger facts, not a transient queue spelling.
		if gen == 2 && mode == "reconcile" && errorPersisted && (jobState == "available" || jobState == "retryable" || jobState == "scheduled") {
			observed = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !observed {
		t.Fatalf("did not observe failed Complete boundary: %s/%d/%s/%s", opState, gen, mode, jobState)
	}
	if _, err := f.base.owner.Exec(context.Background(), `ALTER TABLE integration.operation_events DROP CONSTRAINT `+constraint); err != nil {
		t.Fatal(err)
	}
	if _, err := f.base.owner.Exec(context.Background(), `UPDATE integration.operations SET lease_until=clock_timestamp()-interval '1 second' WHERE id=$1`, p.OperationID); err != nil {
		t.Fatal(err)
	}
	if code := t06Await(t, f, p, "UNKNOWN", "cancelled", 8*time.Second); code != "reconcile_budget_exhausted" {
		t.Fatal("cleanup lost manual requirement")
	}
	if sends.Load() != 1 || queries.Load() != 1 {
		t.Fatalf("cleanup dispatched or queried again: %d/%d", sends.Load(), queries.Load())
	}
}

func TestT06DispatcherForcedStopPreservesClaimForReconcile(t *testing.T) {
	f := newT06GoFixture(t)
	b := f.register(t, f.store, "dispatch-stop-binding")
	p := f.plan(t, "dispatch-stop-op", b, `{"amount":1}`)
	queue := t06Queue(t, f, p)
	var sends, queries atomic.Int64
	started := make(chan struct{}, 1)
	route := t06Route()
	route.Dispatch = func(ctx context.Context, _ integration.DispatchRequest) (integration.Outcome, error) {
		sends.Add(1)
		started <- struct{}{}
		<-ctx.Done()
		return integration.Outcome{}, ctx.Err()
	}
	client := t06StartDispatcher(t, f.worker, queue, []integration.DispatchRoute{route}, t06DispatchOptions())
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("dispatch never started")
	}
	stop, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.StopAndCancel(stop); err != nil {
		t.Fatal("forced stop failed")
	}
	var state string
	if err := f.worker.QueryRow(context.Background(), `SELECT state FROM integration.operations WHERE id=$1`, p.OperationID).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != "DISPATCHING" {
		t.Fatalf("shutdown invented a completion: %s", state)
	}
	if _, err := f.base.owner.Exec(context.Background(), `UPDATE integration.operations SET lease_until=clock_timestamp()-interval '1 second' WHERE id=$1`, p.OperationID); err != nil {
		t.Fatal(err)
	}
	route.Dispatch = func(context.Context, integration.DispatchRequest) (integration.Outcome, error) {
		sends.Add(1)
		return integration.Outcome{}, errors.New("shutdown repeated effect")
	}
	route.Reconcile = func(context.Context, integration.DispatchRequest) (integration.Outcome, error) {
		queries.Add(1)
		return integration.Outcome{State: "SUCCEEDED", Code: "queried_after_shutdown"}, nil
	}
	t06StartDispatcher(t, f.worker, queue, []integration.DispatchRoute{route}, t06DispatchOptions())
	t06Await(t, f, p, "SUCCEEDED", "completed", 8*time.Second)
	if sends.Load() != 1 || queries.Load() != 1 {
		t.Fatalf("wrong shutdown recovery: %d/%d", sends.Load(), queries.Load())
	}
}

// This helper is only executable by this test binary under an explicit marker.
// No production cmd/worker or runtime fixture/provider switch is introduced.
func TestT06DispatcherCrashChild(t *testing.T) {
	if os.Getenv("LC_T06_CHILD") != "1" {
		return
	}
	ctx := context.Background()
	pool, err := platform.OpenWorkerPool(ctx, os.Getenv("LC_T06_WORKER_DSN"))
	if err != nil {
		t.Fatal("child worker authority failed")
	}
	defer pool.Close()
	endpoint := os.Getenv("LC_T06_MOCK_URL")
	mode := os.Getenv("LC_T06_CRASH_MODE")
	route := t06Route()
	request := func(ctx context.Context, path string, in integration.DispatchRequest) (integration.Outcome, error) {
		method := http.MethodGet
		if path == "/effect" {
			method = http.MethodPost
		}
		req, err := http.NewRequestWithContext(ctx, method, endpoint+path, nil)
		if err != nil {
			return integration.Outcome{}, err
		}
		req.Header.Set("Idempotency-Key", in.IdempotencyKey)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return integration.Outcome{}, err
		}
		defer res.Body.Close()
		var out integration.Outcome
		err = json.NewDecoder(res.Body).Decode(&out)
		return out, err
	}
	route.Check = func(ctx context.Context, in integration.DispatchRequest) error {
		if mode == "before" {
			_, err := request(ctx, "/claimed", in)
			if err != nil {
				return err
			}
			select {}
		}
		return nil
	}
	route.Dispatch = func(ctx context.Context, in integration.DispatchRequest) (integration.Outcome, error) {
		out, err := request(ctx, "/effect", in)
		if err == nil && mode == "after" {
			select {}
		}
		return out, err
	}
	route.Reconcile = func(ctx context.Context, in integration.DispatchRequest) (integration.Outcome, error) {
		return request(ctx, "/query", in)
	}
	t06StartDispatcher(t, pool, os.Getenv("LC_T06_QUEUE"), []integration.DispatchRoute{route}, t06DispatchOptions())
	// The parent deliberately kills this process, proving no Go cleanup runs.
	select {}
}

func TestT06DispatcherRealProcessCrashRescue(t *testing.T) {
	for _, mode := range []string{"before", "after"} {
		t.Run(mode, func(t *testing.T) {
			f := newT06GoFixture(t)
			b := f.register(t, f.store, "dispatch-crash-binding")
			p := f.plan(t, "dispatch-crash-op", b, `{"amount":1}`)
			queue := t06Queue(t, f, p)
			var sends, queries atomic.Int64
			ready := make(chan struct{}, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Idempotency-Key") != "lc:"+p.OperationID {
					http.Error(w, "wrong key", 400)
					return
				}
				if r.URL.Path == "/effect" {
					sends.Add(1)
				}
				if r.URL.Path == "/query" {
					queries.Add(1)
				}
				if r.URL.Path == "/claimed" || r.URL.Path == "/effect" {
					select {
					case ready <- struct{}{}:
					default:
					}
				}
				out := integration.Outcome{State: "UNKNOWN", Code: "mock_absent"}
				if sends.Load() > 0 {
					out = integration.Outcome{State: "SUCCEEDED", Code: "mock_observed", ProviderReference: "survives-child"}
				}
				_ = json.NewEncoder(w).Encode(out)
			}))
			defer server.Close()
			binary, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(binary, "-test.run=^TestT06DispatcherCrashChild$", "-test.timeout=50s")
			cmd.Env = append(os.Environ(), "LC_T06_CHILD=1", "LC_T06_WORKER_DSN="+f.worker.Config().ConnString(), "LC_T06_QUEUE="+queue, "LC_T06_MOCK_URL="+server.URL, "LC_T06_CRASH_MODE="+mode)
			cmd.Stdout = io.Discard
			cmd.Stderr = io.Discard
			if err := cmd.Start(); err != nil {
				t.Fatal("start isolated child")
			}
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			t.Cleanup(func() { _ = cmd.Process.Kill() })
			select {
			case <-ready:
			case <-done:
				t.Fatal("child stopped before injected crash")
			case <-time.After(10 * time.Second):
				t.Fatal("child did not reach crash boundary")
			}
			if err := cmd.Process.Kill(); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("child unexpectedly completed normally")
				}
			case <-time.After(5 * time.Second):
				t.Fatal("killed child not reaped")
			}
			// Only age this owned fixture: no wall-clock claim. River itself must
			// rescue the running job; we do NOT rewrite its state or enqueue a duplicate.
			if _, err := f.base.owner.Exec(context.Background(), `UPDATE integration.operations SET lease_until=clock_timestamp()-interval '1 second' WHERE id=$1`, p.OperationID); err != nil {
				t.Fatal(err)
			}
			if _, err := f.base.owner.Exec(context.Background(), `UPDATE river.river_job SET attempted_at=clock_timestamp()-interval '2 hours' WHERE id=$1 AND state='running'`, p.JobID); err != nil {
				t.Fatal(err)
			}
			route := t06Route()
			route.Dispatch = func(context.Context, integration.DispatchRequest) (integration.Outcome, error) {
				sends.Add(1)
				return integration.Outcome{}, errors.New("must never dispatch after crash")
			}
			route.Reconcile = func(ctx context.Context, in integration.DispatchRequest) (integration.Outcome, error) {
				if in.IdempotencyKey != "lc:"+p.OperationID {
					return integration.Outcome{}, errors.New("recovery key changed")
				}
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/query", nil)
				if err != nil {
					return integration.Outcome{}, err
				}
				req.Header.Set("Idempotency-Key", in.IdempotencyKey)
				res, err := http.DefaultClient.Do(req)
				if err != nil {
					return integration.Outcome{}, err
				}
				defer res.Body.Close()
				if res.StatusCode != http.StatusOK {
					return integration.Outcome{}, errors.New("mock query failed")
				}
				var out integration.Outcome
				if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
					return integration.Outcome{}, err
				}
				return out, nil
			}
			opts := t06DispatchOptions()
			opts.MaxGenerations = 2
			t06StartDispatcher(t, f.worker, queue, []integration.DispatchRoute{route}, opts)
			want, jobWant := "SUCCEEDED", "completed"
			if mode == "before" {
				want, jobWant = "UNKNOWN", "cancelled"
			}
			t06Await(t, f, p, want, jobWant, 40*time.Second)
			wantSends := int64(1)
			if mode == "before" {
				wantSends = 0
			}
			if sends.Load() != wantSends || queries.Load() != 1 {
				t.Fatalf("crash repeated effect or lost query: sends=%d queries=%d", sends.Load(), queries.Load())
			}
			var rescued bool
			if err := f.worker.QueryRow(context.Background(), `SELECT errors::text LIKE '%Stuck job rescued by JobRescuer%' FROM river.river_job WHERE id=$1`, p.JobID).Scan(&rescued); err != nil {
				t.Fatal(err)
			}
			if !rescued {
				t.Fatal("test did not exercise real River rescuer")
			}
		})
	}
}
