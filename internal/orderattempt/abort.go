// Purpose: settle a proved zero-effects attempt and replay its durable receipt.
// Depends on: the fenced ops.abort_order_attempt definer, pgx and command IDs.
// Used by: inline/River execution and authoritative recovery after classification.
// Invariants: order-attempt-v1 §3; SQL owns absence proof, no plan decryption,
// no new key, no business effects and no success before the single commit.

package orderattempt

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/command"
)

// Abort asks the server to prove zero effects and seal a pending attempt. The
// reason is an executor classification, never proof supplied by an HTTP client.
// Terminal replay takes precedence over a new reason. An empty token/store is
// accepted only for the actual worker login, which the SQL entry authenticates.
func (s *Service) Abort(ctx context.Context, merchantToken, store, attempt string, generation int64, reason string) (ExecutionReceipt, error) {
	if ctx == nil || s == nil || s.pool == nil || !validExecutionTarget(merchantToken, store, attempt, generation) || len(reason) > 64 {
		return ExecutionReceipt{}, &AdmissionError{code: "invalid_request"}
	}
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := s.pool.BeginTx(bounded, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return ExecutionReceipt{}, admissionFailure(err, false)
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 2*time.Second)
		defer stop()
		_ = tx.Rollback(cleanup)
	}()
	if _, err = tx.Exec(bounded, `SELECT set_config('statement_timeout','5s',true),
	 set_config('lock_timeout','1s',true),set_config('idle_in_transaction_session_timeout','5s',true)`); err != nil {
		return ExecutionReceipt{}, admissionFailure(err, false)
	}
	var authHash []byte
	if merchantToken != "" {
		digest := sha256.Sum256([]byte(merchantToken))
		authHash = digest[:]
	}
	var storeID *string
	if store != "" {
		storeID = &store
	}
	var raw []byte
	// SECURITY DEFINER: ops.abort_order_attempt (§3) takes the attempt fence first,
	// replays terminal state or proves absence under subject/domain locks and seals.
	err = tx.QueryRow(bounded, `SELECT ops.abort_order_attempt($1::bytea,$2::uuid,$3::uuid,$4::bigint,$5::text)`,
		authHash, storeID, attempt, generation, reason).Scan(&raw)
	if err != nil {
		return ExecutionReceipt{}, admissionFailure(err, false)
	}
	out, err := readExecutionReceipt(raw, attempt, generation)
	if err != nil {
		return ExecutionReceipt{}, err
	}
	if err = tx.Commit(bounded); err != nil {
		return ExecutionReceipt{}, admissionFailure(err, true)
	}
	return out, nil
}

func validExecutionTarget(token, store, attempt string, generation int64) bool {
	return command.ValidID(attempt) && generation > 0 &&
		((token == "" && (store == "" || command.ValidID(store))) ||
			(len(token) >= 32 && len(token) <= 512 && command.ValidID(store)))
}

func readExecutionReceipt(raw []byte, attempt string, generation int64) (ExecutionReceipt, error) {
	invalid := &AdmissionError{code: "order_attempt_invalid_response"}
	var out ExecutionReceipt
	if len(raw) > 16384 || json.Unmarshal(raw, &out) != nil || (out.State != "ABORTED" && out.State != "COMMITTED") {
		return ExecutionReceipt{}, invalid
	}
	var receipt struct {
		State      string `json:"state"`
		AttemptID  string `json:"attempt_id"`
		Generation int64  `json:"intent_generation"`
		OrderID    string `json:"order_id"`
		Code       string `json:"code"`
	}
	if json.Unmarshal(out.Receipt, &receipt) != nil || receipt.State != out.State || receipt.AttemptID != attempt || receipt.Generation != generation {
		return ExecutionReceipt{}, invalid
	}
	if out.State == "COMMITTED" {
		if !command.ValidID(receipt.OrderID) {
			return ExecutionReceipt{}, invalid
		}
	} else {
		var fields map[string]json.RawMessage
		if json.Unmarshal(out.Receipt, &fields) != nil || len(fields) != 4 || fields["order_id"] != nil || receipt.Code == "" {
			return ExecutionReceipt{}, invalid
		}
	}
	return out, nil
}
