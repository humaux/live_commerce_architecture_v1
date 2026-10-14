// Purpose: validate the actual retained manual-order participants before serving.
// Depends on: buyer, checkout and platform physical database proof.
// Used by: API startup after unchanged buyer factory assembly.
// Invariants: all participants share main's physical database; validation owns no pools.

package merchanttools

import (
	"context"

	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

// ValidateSameDatabase proves all retained participants use the main database.
func (m *ManualOrders) ValidateSameDatabase(ctx context.Context) error {
	if ctx == nil || m == nil || m.pool == nil || m.issuer == nil || m.buyerPool == nil || m.checkout == nil {
		return command.ErrInvalid
	}
	if m.issuer.ValidateSameDatabase(ctx, m.pool) != nil ||
		platform.ValidateSameDatabase(ctx, m.pool, m.buyerPool) != nil ||
		m.checkout.ValidateSameDatabase(ctx, m.pool) != nil {
		return command.ErrInvalid
	}
	return nil
}
