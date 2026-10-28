// Purpose: preserve recent-auth PT403 codes from merchant billing writes before shared mapping.
// Depends on: pgconn error inspection and internal/platform's fixed PT403 allowlist.
// Used by: Service.pinCustomer and Service.StartCheckout SQL callbacks; never webhook ingress.
// Invariants: only allowlisted messages become typed errors; other errors retain existing mapping.

package billing

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"

	"livecommerce/internal/platform"
)

// mapAdminWriteError runs before run's mapError can collapse PT403 to ErrForbidden.
func mapAdminWriteError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "PT403" {
		return platform.PT403(pgErr.Message, platform.ErrForbidden)
	}
	return err
}
