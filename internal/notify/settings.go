package notify

// settings.go is the merchant side of the new-order mail: the opt-out toggle. Route -> Go endpoint: admin BFF /api/stores/{store}/
// notification-settings -> internal/httpapi/notify.go -> these two functions -> notify.read_store_settings / notify.set_store_settings
// (migration 0090; the definers re-authorize with identity.resolve_access, integration:read / integration:manage).

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

// ErrUnavailable is any failure that is not a coded refusal.
var ErrUnavailable = errors.New("notify: unavailable")

// Settings is the merchant's notification preference. No stored row reads as the default, on.
type Settings struct {
	MerchantNewOrderEmail bool `json:"merchant_new_order_email"`
}

func mapError(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "PT400", "22023":
			return command.ErrInvalid
		case "PT401":
			return platform.ErrUnauthorized
		case "PT403":
			return platform.ErrForbidden
		case "PT404":
			return platform.ErrScopeNotFound
		}
	}
	if errors.Is(err, platform.ErrUnauthorized) || errors.Is(err, platform.ErrForbidden) || errors.Is(err, platform.ErrScopeNotFound) {
		return err
	}
	return ErrUnavailable
}

func validAuthority(scope platform.Scope, token string) bool {
	return command.ValidID(scope.TenantID) && command.ValidID(scope.StoreID) && command.ValidID(scope.PrincipalID) &&
		scope.Revision > 0 && len(token) >= 32 && len(token) <= 512
}

func decode(raw []byte) (Settings, error) {
	var s Settings
	if err := json.Unmarshal(raw, &s); err != nil {
		return Settings{}, ErrUnavailable
	}
	return s, nil
}

// ReadSettings reads the store's preference (integration:read).
func ReadSettings(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string) (Settings, error) {
	if tx == nil || !validAuthority(scope, token) {
		return Settings{}, command.ErrInvalid
	}
	hash := sha256.Sum256([]byte(token))
	var raw []byte
	// notify.read_store_settings (0090): integration:read, fresh final fence; no row = on.
	if err := tx.QueryRow(ctx, `SELECT notify.read_store_settings($1,$2::uuid)`, hash[:], scope.StoreID).Scan(&raw); err != nil {
		return Settings{}, mapError(err)
	}
	return decode(raw)
}

// SetSettings writes the preference (integration:manage). It is an idempotent set, so a retry after an unknown outcome is harmless.
func SetSettings(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string, enabled bool) (Settings, error) {
	if tx == nil || !validAuthority(scope, token) {
		return Settings{}, command.ErrInvalid
	}
	hash := sha256.Sum256([]byte(token))
	var raw []byte
	// notify.set_store_settings (0090): integration:manage, upsert of the opt-out flag.
	if err := tx.QueryRow(ctx, `SELECT notify.set_store_settings($1,$2::uuid,$3)`, hash[:], scope.StoreID, enabled).Scan(&raw); err != nil {
		return Settings{}, mapError(err)
	}
	return decode(raw)
}
