// Purpose: serialize merchant manual/for-buyer attempts across all pipeline transactions and authoritative A16R recovery.
// Depends on: platform scoped transactions, ops.order_intents, command.Run receipts, fulfillment.order_intent_begin,
//
//	claims.for_buyer_request / for_buyer_release for A16R's original reservation cleanup.
//
// Used by: ManualOrders.Place, ForBuyer.Place and ForBuyer.Recover; no external provider calls.
// Invariants: I01/I02/I03/I06/I11; absence requires a durable seal; created requires post-DM A16 completion.
package merchanttools

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/checkout"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

const attemptOperation = "merchanttools.order.attempt"
const recoveryOperation = "merchanttools.order.attempt.recovery"
const checkoutClosureOperation = "merchanttools.order.checkout.closed"
const forBuyerCompleteOperation = "merchanttools.order.for_buyer.completed"
const attemptBudget = 14 * time.Second

type attemptContextKey struct{}
type heldAttempt struct {
	manual     *ManualOrders
	store, key string
}
type attemptRequest struct {
	Kind string `json:"kind"`
	Body any    `json:"body"`
}

// OrderRecovery is A16R's non-sensitive result. It never contains a buyer link, capability or customer data.
type OrderRecovery struct {
	State   string `json:"state"`
	OrderID string `json:"order_id,omitempty"`
}

type forBuyerCompletion struct {
	OrderID string `json:"order_id"`
}

// complete records only the final pipeline boundary, after sendPayLink has returned a durable or definitive result.
// Failure retains UNKNOWN; a same-key Place replay completes the marker without redoing committed side effects.
func (f *ForBuyer) complete(ctx context.Context, token, store, key string, request any, orderID string) error {
	return platform.WithScope(ctx, f.manual.pool, token, store, manualPermission, func(tx pgx.Tx, scope platform.Scope) error {
		if err := requireCatalog(ctx, tx, scope, token); err != nil {
			return err
		}
		var saved forBuyerCompletion
		if err := command.Run(ctx, tx, scope, forBuyerCompleteOperation, key, request, &saved, func() error {
			saved.OrderID = orderID
			return nil
		}); err != nil {
			return err
		}
		if !command.ValidID(saved.OrderID) || saved.OrderID != orderID {
			return command.ErrConflict
		}
		return settleIntent(ctx, tx, scope, key, "created", orderID)
	})
}

func attemptHeld(ctx context.Context, m *ManualOrders, store, key string) bool {
	held, _ := ctx.Value(attemptContextKey{}).(heldAttempt)
	return held.manual == m && held.store == store && held.key == key
}

// withAttempt holds one store/key advisory lock through every pipeline step, including checkout's commit.
// The binding is committed separately before dynamic checks, so a process loss cannot let another body take the key.
func (m *ManualOrders) withAttempt(ctx context.Context, token, store, key, kind string, body any, recovering bool, fn func(context.Context) error) error {
	if attemptHeld(ctx, m, store, key) {
		return fn(ctx)
	}
	ctx, cancel := context.WithTimeout(ctx, attemptBudget)
	defer cancel()
	// Reserve at most half this service's merchant pool for outer locks. Inner pipeline transactions always have room;
	// without admission, unrelated simultaneous keys could consume every connection while waiting for their own inner work.
	select {
	case m.attempts <- struct{}{}:
		defer func() { <-m.attempts }()
	case <-ctx.Done():
		return ErrUnavailable
	}
	return platform.WithScopeBudget(ctx, m.pool, token, store, manualPermission, attemptBudget, func(tx pgx.Tx, scope platform.Scope) error {
		if err := requireCatalog(ctx, tx, scope, token); err != nil {
			return err
		}
		if recovering {
			if err := platform.RequirePermission(ctx, tx, scope, token, "orders:read"); err != nil {
				return err
			}
		}
		// Principal/store comes first: other tabs and fresh sessions share this authority, irrespective of their key.
		if _, err := tx.Exec(ctx, `SELECT set_config('lock_timeout','0',true)`); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "merchanttools.intent|"+scope.TenantID+"|"+scope.StoreID+"|"+scope.PrincipalID); err != nil {
			return err
		}
		// Whole-attempt lock is distinct from command.Run's short receipt lock; nested merchant transactions must not self-deadlock.
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "merchanttools.attempt|"+scope.TenantID+"|"+scope.StoreID+"|"+key); err != nil {
			return err
		}
		request := attemptRequest{Kind: kind, Body: body}
		var recovery OrderRecovery
		var intent orderIntent
		// Revalidate auth after an advisory wait and bind I02 in its own durable transaction.
		err := platform.WithScope(ctx, m.pool, token, store, manualPermission, func(inner pgx.Tx, s platform.Scope) error {
			if s.TenantID != scope.TenantID || s.StoreID != scope.StoreID || s.PrincipalID != scope.PrincipalID {
				return platform.ErrUnauthorized
			}
			if err := requireCatalog(ctx, inner, s, token); err != nil {
				return err
			}
			if recovering {
				if err := platform.RequirePermission(ctx, inner, s, token, "orders:read"); err != nil {
					return err
				}
			}
			// Preserve pre-amendment receipt conflicts before introducing the new binding. Never poison a legacy successful key.
			var legacy map[string]any
			err := command.Run(ctx, inner, s, kind, key, body, &legacy, func() error { return errNeedWork })
			if err != nil && !errors.Is(err, errNeedWork) {
				return err
			}
			legacyFound := err == nil
			var legacyOwner string
			_, intentErr := readIntent(ctx, inner, s, key)
			if errors.Is(intentErr, pgx.ErrNoRows) {
				var previouslyBound bool
				if err = inner.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM ops.command_results WHERE tenant_id=$1 AND store_id=$2 AND operation=$3 AND idempotency_key=$4)`,
					s.TenantID, s.StoreID, attemptOperation, key).Scan(&previouslyBound); err != nil {
					return err
				}
				// Calls fulfillment.order_intent_legacy_owner (0172) for EVERY missing intent before admission or capability creation.
				// An exact old Begin without a canonical binding is UNKNOWN, even when today's secret would derive another owner.
				hash := sha256.Sum256([]byte(token))
				if err = inner.QueryRow(ctx, `SELECT coalesce(fulfillment.order_intent_legacy_owner($1,$2::uuid,$3,$4)::text,'')`, hash[:], store, key, stepKey(key, "begin")).Scan(&legacyOwner); err != nil {
					return err
				}
				if previouslyBound {
					if !command.ValidID(legacyOwner) {
						return ErrUnavailable
					}
				} else if legacyFound {
					return ErrUnavailable
				}
			} else if intentErr != nil {
				return intentErr
			}
			var binding struct {
				Bound bool `json:"bound"`
			}
			if err = command.Run(ctx, inner, s, attemptOperation, key, request, &binding, func() error {
				// The definer already excluded every unbound exact old Begin; no current-secret buyer lookup can prove absence.
				binding.Bound = true
				return nil
			}); err != nil {
				return err
			}
			if intent, err = m.bindIntent(ctx, inner, s, store, key, request, legacyOwner); err != nil {
				return err
			}
			err = command.Run(ctx, inner, s, recoveryOperation, key, request, &recovery, func() error { return errNeedWork })
			if errors.Is(err, errNeedWork) {
				return nil
			}
			if err != nil {
				return err
			}
			if recovery.State == "not_created" {
				return settleIntent(ctx, inner, s, key, "sealed", "")
			}
			return nil
		})
		if err != nil {
			return err
		}
		if !recovering && (recovery.State == "not_created" || intent.State == "sealed") {
			return &Error{Status: http.StatusConflict, Code: "attempt_closed"}
		}
		callCtx := context.WithValue(ctx, attemptContextKey{}, heldAttempt{manual: m, store: store, key: key})
		if err = fn(callCtx); err != nil {
			if !recovering && definitiveIntentRefusal(err) {
				// The refusal itself proves nothing. Only a fresh locked durable absence check may release this slot.
				_ = m.sealRefusedIntent(callCtx, token, store, key, request)
			}
			return err
		}
		if !recovering && kind == manualOperation {
			return m.settleManualSuccess(callCtx, token, store, key, body)
		}
		return nil
	})
}

// Recover resolves the original A16 intent without recreating its pipeline.
// A Begin-only order stays UNKNOWN until an explicit same-key retry completes its upper receipt.
func (f *ForBuyer) Recover(ctx context.Context, token, store, key string, in ForBuyerInput) (OrderRecovery, error) {
	if f == nil || f.manual == nil {
		return OrderRecovery{}, ErrManualDisabled
	}
	request := canonicalForBuyer(in)
	var out OrderRecovery
	err := f.manual.withAttempt(ctx, token, store, key, forBuyerOperation, request, true, func(callCtx context.Context) error {
		return platform.WithScope(callCtx, f.manual.pool, token, store, manualPermission, func(tx pgx.Tx, scope platform.Scope) error {
			var err error
			out, err = f.manual.recoverIntentState(callCtx, tx, scope, token, key, attemptRequest{Kind: forBuyerOperation, Body: request}, false)
			return err
		})
	})
	if err != nil {
		return OrderRecovery{}, mapForBuyerError(err)
	}
	return out, nil
}

// checkoutGuard consumes one merchant connection while Begin holds its checkout transaction lock.
// It never reacquires the outer attempt lock or writes anything; all original private-owner money paths call it.
func (m *ManualOrders) checkoutGuard(ctx context.Context, token, store, key, owner string) error {
	return platform.WithScope(ctx, m.pool, token, store, manualPermission, func(tx pgx.Tx, scope platform.Scope) error {
		if err := requireCatalog(ctx, tx, scope, token); err != nil {
			return err
		}
		intent, err := readIntent(ctx, tx, scope, key)
		if err != nil {
			return err
		}
		if intent.Owner != owner {
			// A rotated signing secret must never place another checkout for this frozen intent.
			return command.ErrConflict
		}
		if intent.State == "sealed" {
			return checkout.ErrAttemptClosed
		}
		var saved string
		err = tx.QueryRow(ctx, `SELECT response->>'owner_id' FROM ops.command_results WHERE tenant_id=$1 AND store_id=$2 AND operation=$3 AND idempotency_key=$4`, scope.TenantID, scope.StoreID, checkoutClosureOperation, stepKey(key, "begin")).Scan(&saved)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if saved != owner {
			return command.ErrConflict
		}
		return checkout.ErrAttemptClosed
	})
}
