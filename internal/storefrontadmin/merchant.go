// merchant.go is the merchant half: read the publication state and toggle it. Both run inside the caller's
// platform.WithScope transaction on the commerce_runtime login (Go routes GET/POST
// /v1/admin/stores/{store_id}/storefront[/publication] in internal/httpapi/storefront.go); the definers re-verify the
// bearer token (integration:read / integration:manage) and the scope GUCs themselves.
//
// Non-goals: no domain write (operator.go), no cache, no Idempotency-Key: the write is a compare-and-set on version.

package storefrontadmin

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

// ErrUnavailable is any database class this package does not map: the caller answers 503, never a driver message.
var ErrUnavailable = errors.New("storefront admin unavailable")

// Domain is one ACTIVE origin of the store; Serving is false once valid_until has passed (the resolver then denies).
type Domain struct {
	Origin     string `json:"origin"`
	ValidUntil string `json:"valid_until"`
	Serving    bool   `json:"serving"`
}

// State is the merchant-visible storefront: Version 0 means never published.
type State struct {
	Published bool     `json:"published"`
	Version   int64    `json:"version"`
	Domains   []Domain `json:"domains"`
}

// SetResult is the outcome of SetPublished; Changed is false when the row already had the requested state.
type SetResult struct {
	Published bool  `json:"published"`
	Version   int64 `json:"version"`
	Changed   bool  `json:"changed"`
}

// maxVersion bounds expected_version the same way the SQL does (2^62).
const maxVersion = int64(1) << 62

func validAuthority(tx pgx.Tx, scope platform.Scope, token string) bool {
	return tx != nil && command.ValidID(scope.TenantID) && command.ValidID(scope.StoreID) && command.ValidID(scope.PrincipalID) &&
		scope.Revision > 0 && len(token) >= 32 && len(token) <= 512
}

// Read returns the store's publication state and ACTIVE domains.
func Read(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string) (State, error) {
	if ctx == nil || !validAuthority(tx, scope, token) {
		return State{}, command.ErrInvalid
	}
	hash := sha256.Sum256([]byte(token))
	var raw []byte
	// control.read_storefront (0081, owner commerce_storefront_writer, EXECUTE commerce_runtime): integration:read
	// checked in SQL against the bearer; reads publications + ACTIVE domains of this store only.
	if err := tx.QueryRow(ctx, `SELECT control.read_storefront($1,$2::uuid)`, hash[:], scope.StoreID).Scan(&raw); err != nil {
		return State{}, mapError(err)
	}
	var out State
	if !strictDecode(raw, &out) || out.Version < 0 || out.Domains == nil || (out.Version == 0 && out.Published) {
		return State{}, ErrUnavailable
	}
	for _, d := range out.Domains {
		if d.Origin == "" || d.ValidUntil == "" {
			return State{}, ErrUnavailable
		}
	}
	return out, nil
}

// SetPublished publishes or unpublishes the store. expectedVersion is the version the merchant last read (0 = never
// published); a stale value is command.ErrConflict. Publishing never binds a domain.
func SetPublished(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string, published bool, expectedVersion int64) (SetResult, error) {
	if ctx == nil || !validAuthority(tx, scope, token) || expectedVersion < 0 || expectedVersion >= maxVersion {
		return SetResult{}, command.ErrInvalid
	}
	hash := sha256.Sum256([]byte(token))
	var raw []byte
	// control.set_storefront_published (0081, same owner): integration:manage, compare-and-set on version, row lock,
	// audit row merchant.storefront_published|unpublished per real change, final authority recheck.
	if err := tx.QueryRow(ctx, `SELECT control.set_storefront_published($1,$2::uuid,$3,$4)`,
		hash[:], scope.StoreID, published, expectedVersion).Scan(&raw); err != nil {
		return SetResult{}, mapError(err)
	}
	var out SetResult
	if !strictDecode(raw, &out) || out.Published != published || out.Version < 0 {
		return SetResult{}, ErrUnavailable
	}
	return out, nil
}

// strictDecode rejects unknown fields and trailing values so a changed definer shape fails closed.
func strictDecode(raw []byte, into any) bool {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if dec.Decode(into) != nil {
		return false
	}
	var extra any
	return dec.Decode(&extra) == io.EOF
}

// mapError turns the definers' fixed SQLSTATE classes into sentinels. Only our own PT409 messages are inspected
// (they carry no customer value); anything unknown is ErrUnavailable.
func mapError(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "PT400", "22023", "23514", "22P02":
			return command.ErrInvalid
		case "PT401":
			return platform.ErrUnauthorized
		case "PT403":
			return platform.ErrForbidden
		case "PT404":
			return platform.ErrScopeNotFound
		case "PT409":
			switch pg.Message {
			case "domain_detached":
				return ErrDomainDetached
			case "domain_owned_elsewhere":
				return ErrDomainOwnedElsewhere
			case "store has no owner principal":
				return ErrNoOwner
			}
			return command.ErrConflict
		case "40001", "40P01", "55P03", "57014", "23505":
			return err // the HTTP layer maps deadlocks/lock timeouts to a retry and unique races to conflict
		}
		return ErrUnavailable
	}
	if errors.Is(err, platform.ErrUnauthorized) || errors.Is(err, platform.ErrForbidden) ||
		errors.Is(err, platform.ErrScopeNotFound) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return err
	}
	return ErrUnavailable
}
