// Purpose: resolve server-frozen merchant manual intents without repeating their order pipeline.
// Depends on: shared withAttempt/recoverIntentState, ops.order_intents, fulfillment.order_intent_begin, scoped receipts/audit.
// Used by: POST orders/manual/recover; storefront-v2 G3.
// Invariants: I01/I02/I03/I06/I11; absence is an atomic seal; created requires the original complete manual receipt.
package merchanttools

import (
	"context"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/platform"
)

// Recover resolves the original manual intent from durable facts under its original private Begin lock.
// Partial checkout remains UNKNOWN; absence seals the key and full completion settles the principal's pending slot.
func (m *ManualOrders) Recover(ctx context.Context, token, store, key string, in ManualInput) (OrderRecovery, error) {
	if m == nil {
		return OrderRecovery{}, ErrManualDisabled
	}
	in = normalizeManual(in)
	var out OrderRecovery
	err := m.withAttempt(ctx, token, store, key, manualOperation, in, true, func(callCtx context.Context) error {
		return platform.WithScope(callCtx, m.pool, token, store, manualPermission, func(tx pgx.Tx, scope platform.Scope) error {
			var err error
			out, err = m.recoverIntentState(callCtx, tx, scope, token, key, attemptRequest{Kind: manualOperation, Body: in}, false)
			return err
		})
	})
	if err != nil {
		return OrderRecovery{}, mapReceiptError(err)
	}
	return out, nil
}
