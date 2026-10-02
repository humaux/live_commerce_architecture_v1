// job_test.go: unit tests for the River worker (job.go). The verify seam is faked so Work never dials DNS/TLS; the
// production VerifyPending wiring and the SQL definers are proven by verify_test.go and the PG gate suite.

package storefrontdomains

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestNewWorker(t *testing.T) {
	if _, err := NewWorker(nil, "example.com"); err == nil {
		t.Error("nil pool accepted")
	}
	w, err := NewWorker(&pgxpool.Pool{}, "example.com")
	if err != nil {
		t.Fatalf("NewWorker: %v", err)
	}
	if w.baseDomain != "example.com" || w.verify == nil {
		t.Fatalf("NewWorker = base %q verify %v, want base example.com and a non-nil verify", w.baseDomain, w.verify)
	}
}

func TestWorkerWork(t *testing.T) {
	ctx := context.Background()
	pool := &pgxpool.Pool{}
	w := &Worker{pool: pool, baseDomain: "example.com"}
	var gotQ Querier
	var gotR Resolver
	var gotP TLSProber
	var gotNow time.Time
	var gotBase string
	w.verify = func(_ context.Context, q Querier, r Resolver, p TLSProber, now time.Time, base string) (int, int, int, error) {
		gotQ, gotR, gotP, gotNow, gotBase = q, r, p, now, base
		return 2, 3, 4, nil
	}
	if err := w.Work(ctx, nil); err != nil {
		t.Fatalf("Work: %v", err)
	}
	if gotQ != pool || gotBase != "example.com" || gotNow.IsZero() {
		t.Fatalf("verify args = q:%p base:%q now:%v", gotQ, gotBase, gotNow)
	}
	// The production seams are the system resolver and prober (never a fake from the job).
	if _, ok := gotR.(SystemResolver); !ok {
		t.Errorf("resolver = %T, want SystemResolver", gotR)
	}
	if _, ok := gotP.(SystemProber); !ok {
		t.Errorf("prober = %T, want SystemProber", gotP)
	}
}

func TestWorkerWorkPropagatesError(t *testing.T) {
	ctx := context.Background()
	w := &Worker{pool: &pgxpool.Pool{}, baseDomain: "example.com"}
	want := errors.New("db down")
	w.verify = func(context.Context, Querier, Resolver, TLSProber, time.Time, string) (int, int, int, error) {
		return 0, 0, 0, want
	}
	if err := w.Work(ctx, nil); !errors.Is(err, want) {
		t.Fatalf("Work = %v, want %v", err, want)
	}
	// A worker whose verify seam is unset is a usage error, not a silent pass.
	nilW := &Worker{pool: &pgxpool.Pool{}, baseDomain: "example.com"}
	if err := nilW.Work(ctx, nil); err == nil {
		t.Error("nil verify accepted")
	}
}

func TestJobShape(t *testing.T) {
	if (JobArgs{}).Kind() != JobKind {
		t.Errorf("JobArgs.Kind() = %q, want %q", (JobArgs{}).Kind(), JobKind)
	}
	if opts := (JobArgs{}).InsertOpts(); opts.UniqueOpts.ByPeriod != time.Minute {
		t.Errorf("InsertOpts ByPeriod = %v, want 1m", opts.UniqueOpts.ByPeriod)
	}
	if got := (&Worker{}).Timeout(nil); got != RescueWindow-5*time.Second {
		t.Errorf("Timeout = %v, want %v", got, RescueWindow-5*time.Second)
	}
	if PeriodicJob() == nil {
		t.Error("PeriodicJob returned nil")
	}
}
