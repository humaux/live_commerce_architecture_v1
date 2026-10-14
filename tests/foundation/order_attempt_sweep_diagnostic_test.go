// Purpose: expose a failed global sweep's first current candidate without private data.
// Depends on: the actual claims login and public native sweep/fence/abort entries.
// Used by: failure-only REAL_PG diagnostics; this is not an alternative consumer.
// Invariants: every probe rolls back, fresh transactions preserve fence order,
// fixed output only, no assertion changes, at most fourteen seconds total.
package foundation_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func oaSweepDiagnosticError(t *testing.T, phase, attempt string, generation int64, err error) {
	t.Helper()
	state, message := "non_sql", "unclassified_native"
	var native *pgconn.PgError
	if errors.As(err, &native) {
		if len(native.Code) == 5 {
			valid := true
			for _, c := range native.Code {
				if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z')) {
					valid = false
				}
			}
			if valid {
				state = native.Code
			}
		}
		switch native.Message {
		case "attempt_repair_unavailable", "attempt_repair_already_prepared", "attempt_expired",
			"intent_conflict", "receipt_expired", "attempt_not_fenced", "abort_reason_not_proven",
			"attempt_has_effects", "attempt_not_found", "invalid_execution_authority",
			"invalid_sweep_authority", "invalid_sweep_request", "invalid_abort_reason":
			message = native.Message
		case "order attempt identity is immutable":
			message = "immutable_attempt_identity"
		case "unlinked order attempt job":
			message = "unlinked_attempt_job"
		case "repair must commit before execution":
			message = "repair_requires_separate_transaction"
		}
	} else if errors.Is(err, context.DeadlineExceeded) {
		message = "diagnostic_deadline"
	} else if errors.Is(err, context.Canceled) {
		message = "diagnostic_cancelled"
	}
	// Never print err, PgError.Detail/Hint/Where/InternalQuery or native JSON.
	t.Logf("sweep diagnostic phase=%s attempt=%s generation=%d sqlstate=%s message=%s", phase, attempt, generation, state, message)
}

func oaSweepDiagnoseFirst(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	whole, stop := context.WithTimeout(context.Background(), 14*time.Second)
	defer stop()
	attempt, generation := "", int64(0)
	probe := func(phase, query string, args ...any) ([]byte, bool) {
		ctx, cancel := context.WithTimeout(whole, 5*time.Second)
		defer cancel()
		tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
		if err != nil {
			oaSweepDiagnosticError(t, phase, attempt, generation, err)
			return nil, false
		}
		defer func() {
			cleanup, done := context.WithTimeout(ctx, time.Second)
			defer done()
			if err := tx.Rollback(cleanup); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
				oaSweepDiagnosticError(t, "rollback", attempt, generation, err)
			}
		}()
		_, err = tx.Exec(ctx, `SELECT set_config('statement_timeout','5s',true),
		 set_config('lock_timeout','1s',true),set_config('idle_in_transaction_session_timeout','5s',true)`)
		var raw []byte
		if err == nil {
			err = tx.QueryRow(ctx, query, args...).Scan(&raw)
		}
		if err != nil {
			clear(raw)
			oaSweepDiagnosticError(t, phase, attempt, generation, err)
			return nil, false
		}
		if phase == "expired_abort" {
			if _, err = tx.Exec(ctx, `SET CONSTRAINTS ALL IMMEDIATE`); err != nil {
				clear(raw)
				oaSweepDiagnosticError(t, "deferred_abort", attempt, generation, err)
				return nil, false
			}
		}
		return raw, true
	}
	raw, ok := probe("discover", `SELECT ops.order_attempt_sweep('discover',NULL,NULL)`)
	if !ok {
		return
	}
	var candidates []struct {
		AttemptID  string `json:"attempt_id"`
		Generation int64  `json:"intent_generation"`
	}
	err := json.Unmarshal(raw, &candidates)
	clear(raw)
	if err != nil || len(candidates) > 100 {
		t.Log("sweep diagnostic phase=discover message=invalid_native_shape")
		return
	}
	if len(candidates) == 0 {
		t.Log("sweep diagnostic phase=discover message=no_current_candidate")
		return
	}
	attempt, generation = candidates[0].AttemptID, candidates[0].Generation
	// A current observation may differ from the original failed batch under races.
	t.Logf("sweep diagnostic phase=discover attempt=%s generation=%d message=current_first_candidate", attempt, generation)
	raw, ok = probe("prepare", `SELECT ops.order_attempt_sweep('prepare',$1::uuid,$2::bigint)`, attempt, generation)
	if !ok {
		return
	}
	var prepared struct {
		Action string `json:"action"`
	}
	err = json.Unmarshal(raw, &prepared)
	clear(raw)
	if err != nil || (prepared.Action != "SKIP" && prepared.Action != "REQUEUE" && prepared.Action != "EXPIRED") {
		t.Log("sweep diagnostic phase=prepare message=invalid_native_shape")
		return
	}
	t.Logf("sweep diagnostic phase=prepare attempt=%s generation=%d action=%s", attempt, generation, prepared.Action)
	if prepared.Action != "EXPIRED" {
		return
	}
	// Each completed probe rolled back. Never take an execution suffix in a repair Tx.
	raw, ok = probe("execution_fence", `SELECT ops.lock_order_attempt(NULL,NULL,$1::uuid,$2::bigint)`, attempt, generation)
	clear(raw) // The public fence may return ciphertext/binding; neither is logged.
	if !ok {
		return
	}
	t.Logf("sweep diagnostic phase=execution_fence attempt=%s generation=%d message=accepted", attempt, generation)
	raw, ok = probe("expired_abort", `SELECT ops.abort_order_attempt(NULL,NULL,$1::uuid,$2::bigint,'EXPIRED')`, attempt, generation)
	clear(raw)
	if ok {
		t.Logf("sweep diagnostic phase=expired_abort attempt=%s generation=%d message=accepted_rolled_back", attempt, generation)
	}
}
