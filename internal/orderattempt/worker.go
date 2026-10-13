// Purpose: register native intent workers and repair a bounded pending backlog.
// Depends on: Executor, its exact River/PG pool and the worker-only ops sweep entry.
// Used by: the combined claims-worker composition root and REAL_PG recovery gates.
// Invariants: opaque jobs, one attempt per repair Tx, no scope lock, no partial
// pointer commit, and expiry execution only after the repair Tx has ended.
package orderattempt

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
)

type sweepJob struct{}

func (sweepJob) Kind() string { return "order_attempt_sweep_v1" }

type attemptWorker struct {
	river.WorkerDefaults[admissionJob]
	executor *Executor
}

func (w *attemptWorker) Work(ctx context.Context, job *river.Job[admissionJob]) error {
	if w == nil || w.executor == nil || job == nil {
		return &AdmissionError{code: "invalid_request"}
	}
	_, err := w.executor.Execute(ctx, "", "", job.Args.AttemptID, job.Args.Generation)
	return err
}

func (*attemptWorker) Timeout(*river.Job[admissionJob]) time.Duration { return 10 * time.Second }

type sweepWorker struct {
	river.WorkerDefaults[sweepJob]
	executor *Executor
}

func (w *sweepWorker) Work(ctx context.Context, _ *river.Job[sweepJob]) error {
	if w == nil || w.executor == nil {
		return &AdmissionError{code: "invalid_request"}
	}
	_, err := w.executor.Sweep(ctx)
	return err
}

func (*sweepWorker) Timeout(*river.Job[sweepJob]) time.Duration { return 55 * time.Second }

// AddWorkers preserves the caller's other kinds and reports duplicate kinds.
// The caller must include the claims queue when constructing its River client.
func AddWorkers(workers *river.Workers, executor *Executor) error {
	if workers == nil || executor == nil || executor.pool == nil || executor.jobs == nil {
		return &AdmissionError{code: "invalid_request"}
	}
	if err := river.AddWorkerSafely(workers, &attemptWorker{executor: executor}); err != nil {
		return err
	}
	return river.AddWorkerSafely(workers, &sweepWorker{executor: executor})
}

// PeriodicJob supplies the exact lane and empty payload accepted by the native
// family guard. Uniqueness must remain unset, including for startup insertion.
func PeriodicJob() *river.PeriodicJob {
	return river.NewPeriodicJob(river.PeriodicInterval(30*time.Second),
		func() (river.JobArgs, *river.InsertOpts) {
			return sweepJob{}, &river.InsertOpts{Queue: "claims", Priority: 1}
		}, &river.PeriodicJobOpts{ID: "order_attempt_sweep_v1", RunOnStart: true})
}

// SweepResult contains counts only; it cannot expose plans or contact material.
type SweepResult struct {
	Scanned  int
	Requeued int
	Aborted  int
}

// Sweep discovers at most 100 opaque IDs without holding domain locks. Each
// repair ends before another attempt or the ordinary fenced executor is entered.
func (e *Executor) Sweep(ctx context.Context) (SweepResult, error) {
	var result SweepResult
	if ctx == nil || e == nil || e.pool == nil || e.jobs == nil {
		return result, &AdmissionError{code: "invalid_request"}
	}
	bounded, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	discovery, stop := context.WithTimeout(bounded, 5*time.Second)
	var raw []byte
	// SECURITY DEFINER: this entry checks the actual claims-worker login and
	// discovers only PENDING identities, without locking attempts or River rows.
	err := e.pool.QueryRow(discovery, `SELECT ops.order_attempt_sweep('discover',NULL,NULL)`).Scan(&raw)
	stop()
	if err != nil {
		return result, admissionFailure(err, false)
	}
	var candidates []admissionJob
	if len(raw) > 16384 || json.Unmarshal(raw, &candidates) != nil || len(candidates) > 100 {
		return result, &AdmissionError{code: "order_attempt_invalid_response"}
	}
	for _, candidate := range candidates {
		if !validExecutionTarget("", "", candidate.AttemptID, candidate.Generation) {
			return result, &AdmissionError{code: "order_attempt_invalid_response"}
		}
	}
	for _, candidate := range candidates {
		if err = bounded.Err(); err != nil {
			return result, admissionFailure(err, false)
		}
		result.Scanned++
		action, err := e.repairAttempt(bounded, candidate)
		if err != nil {
			return result, err
		}
		switch action {
		case "REQUEUE":
			result.Requeued++
		case "EXPIRED":
			// repairAttempt has rolled back and released its attempt lock. The
			// executor rechecks the SQL clock under its own fresh attempt fence.
			out, err := e.Execute(bounded, "", "", candidate.AttemptID, candidate.Generation)
			if err != nil {
				return result, err
			}
			if out.State == "ABORTED" {
				result.Aborted++
			}
		}
	}
	return result, nil
}

func (e *Executor) repairAttempt(ctx context.Context, candidate admissionJob) (string, error) {
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := e.pool.BeginTx(bounded, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return "", admissionFailure(err, false)
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 2*time.Second)
		defer stop()
		_ = tx.Rollback(cleanup)
	}()
	if _, err = tx.Exec(bounded, `SELECT set_config('statement_timeout','5s',true),
	 set_config('lock_timeout','1s',true),set_config('idle_in_transaction_session_timeout','5s',true)`); err != nil {
		return "", admissionFailure(err, false)
	}
	var raw []byte
	// SECURITY DEFINER: prepare locks this attempt first with SKIP LOCKED,
	// rechecks the job only afterward and arms its protected current-xid witness.
	err = tx.QueryRow(bounded, `SELECT ops.order_attempt_sweep('prepare',$1::uuid,$2::bigint)`,
		candidate.AttemptID, candidate.Generation).Scan(&raw)
	if err != nil {
		return "", admissionFailure(err, false)
	}
	var prepared struct {
		Action     string `json:"action"`
		AttemptID  string `json:"attempt_id"`
		Generation int64  `json:"intent_generation"`
	}
	if len(raw) > 1024 || json.Unmarshal(raw, &prepared) != nil || prepared.AttemptID != candidate.AttemptID || prepared.Generation != candidate.Generation {
		return "", &AdmissionError{code: "order_attempt_invalid_response"}
	}
	switch prepared.Action {
	case "SKIP", "EXPIRED":
		return prepared.Action, nil
	case "REQUEUE":
	default:
		return "", &AdmissionError{code: "order_attempt_invalid_response"}
	}
	job, err := e.jobs.InsertTx(bounded, tx, candidate, &river.InsertOpts{Queue: "claims", Priority: 1})
	if err != nil {
		return "", admissionFailure(err, false)
	}
	// SECURITY DEFINER: finish consumes the same-Tx job witness in its sole
	// attempt UPDATE. The deferred native constraint refuses partial commits.
	err = tx.QueryRow(bounded, `SELECT ops.order_attempt_sweep('finish',$1::uuid,$2::bigint)`,
		candidate.AttemptID, candidate.Generation).Scan(&raw)
	if err != nil {
		if executionAbortCode(err) == "EXPIRED" {
			return "EXPIRED", nil
		}
		return "", admissionFailure(err, false)
	}
	var finished struct {
		AttemptID  string `json:"attempt_id"`
		Generation int64  `json:"intent_generation"`
		JobID      int64  `json:"execution_job_id"`
	}
	if len(raw) > 1024 || json.Unmarshal(raw, &finished) != nil || finished.AttemptID != candidate.AttemptID ||
		finished.Generation != candidate.Generation || finished.JobID != job.Job.ID {
		return "", &AdmissionError{code: "order_attempt_invalid_response"}
	}
	if err = tx.Commit(bounded); err != nil {
		return "", admissionFailure(err, true)
	}
	return "REQUEUE", nil
}
