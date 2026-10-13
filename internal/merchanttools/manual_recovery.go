// Purpose: resolve frozen merchant manual attempts without repeating their order pipeline.
// Depends on: shared withAttempt, buyer issuer, checkout LookupBeginLocked, scoped command receipts and audit.
// Used by: POST orders/manual/recover; storefront-v2 G3.
// Invariants: I01/I02/I03/I06/I11; absence is an atomic seal; created requires the original complete manual receipt.
package merchanttools

import (
	"context"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

// Recover checks the original Begin and manual completion under their original locks.
// It writes only the absence closure and non-sensitive recovery receipt/audit; partial checkout remains UNKNOWN.
func (m *ManualOrders) Recover(ctx context.Context, token, store, key string, in ManualInput) (OrderRecovery, error) {
	if m == nil {
		return OrderRecovery{}, ErrManualDisabled
	}
	// Placement binds this normalization before validation. Invalid or retired options must remain recoverable.
	in = normalizeManual(in)
	var out OrderRecovery
	err := m.withAttempt(ctx, token, store, key, manualOperation, in, true, func(callCtx context.Context) error {
		return platform.WithScope(callCtx, m.pool, token, store, manualPermission, func(tx pgx.Tx, scope platform.Scope) error {
			check := func() error {
				for _, permission := range []string{manualPermission, "catalog:read", "orders:read"} {
					if err := platform.RequirePermission(callCtx, tx, scope, token, permission); err != nil {
						return err
					}
				}
				return nil
			}
			if err := check(); err != nil {
				return err
			}
			capability := m.capability("order", store, key)
			// buyer.register_capability_limited preserves the original private owner on repeated tokens (G3).
			issued, err := m.issuer.RegisterForTrustedStore(callCtx, store, capability)
			if err != nil {
				return err
			}
			if issued.Scope.TenantID != scope.TenantID || issued.Scope.StoreID != scope.StoreID {
				return platform.ErrScopeNotFound
			}
			beginKey := stepKey(key, "begin")
			// Keep the original Begin lock until closure + recovery + audit COMMIT together.
			if _, err = tx.Exec(callCtx, `SELECT set_config('lock_timeout','0',true)`); err != nil {
				return err
			}
			if _, err = tx.Exec(callCtx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "checkout.begin|"+scope.TenantID+"|"+store+"|"+issued.Scope.OwnerID+"|"+beginKey); err != nil {
				return err
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
				// Manual Place commits source/link/audit with this upper receipt. Begin alone cannot prove completion.
				var completed manualReceipt
				if err = command.Run(callCtx, tx, scope, manualOperation, key, in, &completed, func() error {
					return ErrUnavailable
				}); err != nil {
					return err
				}
				if !command.ValidID(completed.OrderID) || completed.OrderID != result.OrderID {
					return command.ErrConflict
				}
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
			if err = command.Run(callCtx, tx, scope, recoveryOperation, key, attemptRequest{Kind: manualOperation, Body: in}, &out, func() error {
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
		return OrderRecovery{}, mapReceiptError(err)
	}
	return out, nil
}
