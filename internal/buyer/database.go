// Purpose: validate the retained capability issuer against the process database.
// Depends on: platform physical database proof and the service's private issuer pool.
// Used by: merchanttools startup validation.
// Invariants: no credentials or pool getters; callers retain ownership of both pools.
package buyer

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/platform"
)

// ValidateSameDatabase proves the retained issuer uses the reference database.
func (s *Service) ValidateSameDatabase(ctx context.Context, reference *pgxpool.Pool) error {
	if ctx == nil || s == nil || s.issuerPool == nil || reference == nil {
		return errUnavailable
	}
	if platform.ValidateSameDatabase(ctx, reference, s.issuerPool) != nil {
		return errUnavailable
	}
	return nil
}
