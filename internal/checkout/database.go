// Purpose: validate the retained checkout authority against the process database.
// Depends on: platform physical database proof and the service's private pool.
// Used by: merchanttools startup validation.
// Invariants: no credentials or pool getters; callers retain ownership of both pools.

package checkout

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/platform"
)

// ValidateSameDatabase proves the retained checkout uses the reference database.
func (s *Service) ValidateSameDatabase(ctx context.Context, reference *pgxpool.Pool) error {
	if ctx == nil || s == nil || s.pool == nil || reference == nil {
		return errCheckoutDatabase
	}
	if platform.ValidateSameDatabase(ctx, reference, s.pool) != nil {
		return errCheckoutDatabase
	}
	return nil
}
