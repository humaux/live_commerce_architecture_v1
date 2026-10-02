// job.go is the River side of package storefrontdomains: the periodic job store_domain_verify_v1 that
// cmd/claims-worker registers (NewWorker + PeriodicJob) and that runs VerifyPending on the dedicated
// store-domain verify pool (lc_store_domain_verify -> commerce_storefront_registrar). This is the P0-2
// production runner for the Decision 3 lifecycle (REQUESTED -> OWNERSHIP_PENDING -> TLS_PENDING -> ACTIVE):
// there is no operator CLI sweep, only this job. Non-goals: no choice of rows (the SQL batch reads pick),
// no policy, no scheduler of its own.

package storefrontdomains

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
)

// JobKind is the River kind of the periodic verify job (main `river` schema, queue default).
const JobKind = "store_domain_verify_v1"

// JobArgs carries nothing: the pending rows and the base domain live outside the job.
type JobArgs struct{}

// Kind returns JobKind.
func (JobArgs) Kind() string { return JobKind }

// InsertOpts makes the job unique per minute, so a restart within the minute inserts no second one and
// RunOnStart on two replicas still yields one job.
func (JobArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{ByPeriod: time.Minute}}
}

// verifyFunc is the unit-test seam; NewWorker wires it to VerifyPending.
type verifyFunc func(context.Context, Querier, Resolver, TLSProber, time.Time, string) (int, int, error)

// Worker runs one sweep. The DNS attempt backs off per row (BackoffDelay), so the once-a-minute job only
// retries rows whose backoff has elapsed; the 72 h expiry is enforced inside sweepDNS.
type Worker struct {
	river.WorkerDefaults[JobArgs]
	pool       *pgxpool.Pool
	baseDomain string
	verify     verifyFunc
}

// NewWorker returns the job worker over the store-domain verify pool the caller already validated
// (platform.OpenStoreDomainVerifyPool). baseDomain is the platform base zone (LC_STORE_BASE_DOMAIN)
// used to build the CNAME target stores.<base>. A nil pool is an error.
func NewWorker(pool *pgxpool.Pool, baseDomain string) (*Worker, error) {
	if pool == nil {
		return nil, errVerifyUsage
	}
	w := &Worker{pool: pool, baseDomain: baseDomain}
	w.verify = VerifyPending
	return w, nil
}

// Work runs one sweep. A database error is returned so River retries the job; the transitions already
// committed stay committed (the definers are idempotent per row).
func (w *Worker) Work(ctx context.Context, _ *river.Job[JobArgs]) error {
	if w == nil || w.verify == nil {
		return errVerifyUsage
	}
	dnsAttempts, tlsCompleted, err := w.verify(ctx, w.pool, SystemResolver{}, SystemProber{}, time.Now(), w.baseDomain)
	if err != nil {
		return err
	}
	// One fixed line, numbers only.
	slog.Info("store_domain_verify_run", "dns_attempts", dnsAttempts, "tls_completed", tlsCompleted)
	return nil
}

// RescueWindow is River's RescueStuckJobsAfter in cmd/claims-worker. A job running longer is rescued and a
// second runner starts beside the first, so the job bounds itself below it.
const RescueWindow = time.Minute

// Timeout bounds one job below RescueWindow; a sweep mid-probe is aborted by the context deadline and the
// rest waits for the next minute.
func (w *Worker) Timeout(*river.Job[JobArgs]) time.Duration { return RescueWindow - 5*time.Second }

// PeriodicJob is the once-a-minute schedule, also run once at claims-worker start. The constructor returns
// no InsertOpts so JobArgs.InsertOpts (minute uniqueness, queue default) applies.
func PeriodicJob() *river.PeriodicJob {
	return river.NewPeriodicJob(river.PeriodicInterval(time.Minute),
		func() (river.JobArgs, *river.InsertOpts) { return JobArgs{}, nil },
		&river.PeriodicJobOpts{ID: JobKind, RunOnStart: true})
}

var errVerifyUsage = errors.New("store domain verify usage")
