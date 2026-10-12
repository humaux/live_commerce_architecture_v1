// Purpose: serialize merchant manual/for-buyer attempts across all pipeline transactions and authoritative A16R recovery.
// Depends on: platform scoped transactions, command.Run append-only ops.command_results, checkout durable Begin receipts.
// Used by: ManualOrders.Place, ForBuyer.Place and ForBuyer.Recover; no external provider calls.
// Invariants: I01/I02/I03/I06/I11; absence is final only after a durable seal, with original placement excluded.
package merchanttools

import (
	"context"
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

func attemptHeld(ctx context.Context, m *ManualOrders, store, key string) bool {
	held, _ := ctx.Value(attemptContextKey{}).(heldAttempt)
	return held.manual == m && held.store == store && held.key == key
}

// withAttempt holds one store/key advisory lock through every pipeline step, including checkout's commit.
// The binding is committed separately before dynamic checks, so a process loss cannot let another body take the key.
func (m *ManualOrders) withAttempt(ctx context.Context, token, store, key, kind string, body any, recovering bool, fn func(context.Context) error) error {
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
		// Whole-attempt lock is distinct from command.Run's short receipt lock; nested merchant transactions must not self-deadlock.
		if _, err := tx.Exec(ctx, `SELECT set_config('lock_timeout','0',true)`); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "merchanttools.attempt|"+scope.TenantID+"|"+scope.StoreID+"|"+key); err != nil {
			return err
		}
		request := attemptRequest{Kind: kind, Body: body}
		var recovery OrderRecovery
		// Revalidate auth after an advisory wait and bind I02 in its own durable transaction.
		err := platform.WithScope(ctx, m.pool, token, store, manualPermission, func(inner pgx.Tx, s platform.Scope) error {
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
			var binding struct {
				Bound bool `json:"bound"`
			}
			if err = command.Run(ctx, inner, s, attemptOperation, key, request, &binding, func() error {
				if !legacyFound {
					// A pre-amendment partial order has no full canonical A16 binding. The key alone cannot
					// prove the target/send/customer body. Never let a newly supplied body claim that legacy order.
					_, found, err := m.lookupAttempt(ctx, store, key)
					if err != nil {
						return err
					}
					if found {
						return ErrUnavailable
					}
				}
				binding.Bound = true
				return nil
			}); err != nil {
				return err
			}
			err = command.Run(ctx, inner, s, recoveryOperation, key, request, &recovery, func() error { return errNeedWork })
			if errors.Is(err, errNeedWork) {
				return nil
			}
			return err
		})
		if err != nil {
			return err
		}
		if !recovering && recovery.State == "not_created" {
			return &Error{Status: http.StatusConflict, Code: "attempt_closed"}
		}
		return fn(context.WithValue(ctx, attemptContextKey{}, heldAttempt{manual: m, store: store, key: key}))
	})
}

// Recover resolves the frozen A16 attempt under its original key, with no order/stock/link/DM creation.
// UNKNOWN (including a capability/receipt lookup failure) stays UNKNOWN; it never writes a not_created receipt.
func (f *ForBuyer) Recover(ctx context.Context, token, store, key string, in ForBuyerInput) (OrderRecovery, error) {
	if f == nil || f.manual == nil {
		return OrderRecovery{}, ErrManualDisabled
	}
	_, _, request, err := normalizeForBuyer(in)
	if err != nil {
		return OrderRecovery{}, err
	}
	m := f.manual
	var out OrderRecovery
	err = m.withAttempt(ctx, token, store, key, forBuyerOperation, request, true, func(callCtx context.Context) error {
		return platform.WithScope(callCtx, m.pool, token, store, manualPermission, func(tx pgx.Tx, scope platform.Scope) error {
			if err := requireCatalog(callCtx, tx, scope, token); err != nil {
				return err
			}
			if err := platform.RequirePermission(callCtx, tx, scope, token, "orders:read"); err != nil {
				return err
			}
			capability := m.capability("order", store, key)
			issued, err := m.issuer.RegisterForTrustedStore(callCtx, store, capability)
			if err != nil {
				return err
			}
			if issued.Scope.TenantID != scope.TenantID || issued.Scope.StoreID != scope.StoreID {
				return platform.ErrScopeNotFound
			}
			beginKey := stepKey(key, "begin")
			// THIS transaction retains the exact Begin lock until both closure and upper recovery receipts COMMIT.
			// The checkout reader uses its own narrow pool without reacquiring this lock; a lost connection rolls this seal back.
			if _, err = tx.Exec(callCtx, `SELECT set_config('lock_timeout','0',true)`); err != nil {
				return err
			}
			if _, err = tx.Exec(callCtx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "checkout.begin|"+scope.TenantID+"|"+store+"|"+issued.Scope.OwnerID+"|"+beginKey); err != nil {
				return err
			}
			check := func() error {
				for _, permission := range []string{manualPermission, "catalog:read", "orders:read"} {
					if err := platform.RequirePermission(callCtx, tx, scope, token, permission); err != nil {
						return err
					}
				}
				return nil
			}
			if err = check(); err != nil {
				return err
			}
			result, found, err := m.checkout.LookupBeginLocked(callCtx, capability, store, beginKey, issued.Scope)
			if err != nil {
				return err
			}
			if err = check(); err != nil {
				return err
			}
			resolved := OrderRecovery{State: "not_created"}
			if found {
				resolved = OrderRecovery{State: "created", OrderID: result.OrderID}
			} else {
				binding := struct {
					OwnerID string `json:"owner_id"`
				}{issued.Scope.OwnerID}
				var saved struct {
					OwnerID string `json:"owner_id"`
				}
				if err = command.Run(callCtx, tx, scope, checkoutClosureOperation, beginKey, binding, &saved, func() error {
					if err := check(); err != nil {
						return err
					}
					saved.OwnerID = issued.Scope.OwnerID
					return nil
				}); err != nil {
					return err
				}
				if saved.OwnerID != issued.Scope.OwnerID {
					return command.ErrConflict
				}
			}
			if err = command.Run(callCtx, tx, scope, recoveryOperation, key, attemptRequest{Kind: forBuyerOperation, Body: request}, &out, func() error {
				if err := check(); err != nil {
					return err
				}
				out = resolved
				return command.Audit(callCtx, tx, scope, "order.attempt_recovered")
			}); err != nil {
				return err
			}
			if out != resolved {
				return command.ErrConflict
			}
			return check()
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
		var saved string
		err := tx.QueryRow(ctx, `SELECT response->>'owner_id' FROM ops.command_results WHERE tenant_id=$1 AND store_id=$2 AND operation=$3 AND idempotency_key=$4`, scope.TenantID, scope.StoreID, checkoutClosureOperation, stepKey(key, "begin")).Scan(&saved)
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

// lookupAttempt derives the private original capability/key and verifies actual durable checkout facts.
func (m *ManualOrders) lookupAttempt(ctx context.Context, store, key string) (checkout.Result, bool, error) {
	capability := m.capability("order", store, key)
	// buyer.register_capability_limited: repeated tokens preserve the original owner and expiry; never replace an expired attempt.
	if _, err := m.issuer.RegisterForTrustedStore(ctx, store, capability); err != nil {
		return checkout.Result{}, false, err
	}
	return m.checkout.LookupBegin(ctx, capability, store, stepKey(key, "begin"))
}
