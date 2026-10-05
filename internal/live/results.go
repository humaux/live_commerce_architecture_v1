// Purpose: A5-1 session-results read model (M14): per-session order count and money over the unified attribution union (live_price_uses⋈bundles ∪ claims.order_origins, integrator ruling 2), served by identity.read_live_session_results (migrations/0114). One database call, then a strict decode + re-verification of the projection (finance.read pattern), so a schema drift fails closed.
// Depends on: draft.go (authorize/mapError/readPermission), platform.RequirePermission, command, identity.read_live_session_results (0114), live.order_session_labels (0110 ownership fence).
// Used by: internal/httpapi/live_flow.go GET /live-sessions/results.
package live

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

// ErrResultsUnavailable is any projection failure (malformed or inconsistent database result); the
// HTTP layer maps it to 503 unavailable, never to a 500 that could echo a row value.
var ErrResultsUnavailable = errors.New("live session results unavailable")

var currencyCode = regexp.MustCompile(`^[A-Z]{3}$`)

// SessionResults is the A5-1 envelope. Items preserves the request order (the SQL orders by
// array_position over the requested ids) so the admin can render columns without a second sort.
type SessionResults struct {
	AsOf  time.Time       `json:"as_of"`
	Items []SessionResult `json:"items"`
}

// SessionResult is one session's totals. Orders/PaidOrders/MultiSessionOrders are counts of
// non-CANCELLED orders attributed to the session; a multi-session order is counted fully in every
// session it spans (ruling 3) and flagged via MultiSessionOrders.
type SessionResult struct {
	SessionID          string        `json:"session_id"`
	Orders             int64         `json:"orders"`
	PaidOrders         int64         `json:"paid_orders"`
	MultiSessionOrders int64         `json:"multi_session_orders"`
	Money              []MoneyBucket `json:"money"`
}

// MoneyBucket is one currency's amounts, split by payment environment (I05): OrderMinor is the
// non-CANCELLED order total (total_minor + cod_surcharge_minor); PaidMinor/SandboxPaidMinor are the
// per-environment collected money (never summed across environments or currencies).
type MoneyBucket struct {
	Currency         string `json:"currency"`
	OrderMinor       int64  `json:"order_minor"`
	PaidMinor        int64  `json:"paid_minor"`
	SandboxPaidMinor int64  `json:"sandbox_paid_minor"`
}

// Results reads the session-results projection for 1..50 distinct store sessions. The SQL definer
// re-authenticates orders:read + live:read; the second fence below re-checks orders:read against the
// scope the HTTP layer opened (finance.read pattern).
func Results(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string, sessionIDs []string) (SessionResults, error) {
	if tx == nil || !command.ValidID(scope.TenantID) || !command.ValidID(scope.StoreID) || !command.ValidID(scope.PrincipalID) ||
		scope.Revision < 1 || len(token) < 32 || len(token) > 512 || len(sessionIDs) < 1 || len(sessionIDs) > 50 {
		return SessionResults{}, command.ErrInvalid
	}
	seen := map[string]bool{}
	for _, id := range sessionIDs {
		if !command.ValidID(id) || seen[id] {
			return SessionResults{}, command.ErrInvalid
		}
		seen[id] = true
	}
	hash := sha256.Sum256([]byte(token))
	var raw []byte
	if err := tx.QueryRow(ctx, `SELECT identity.read_live_session_results($1,$2::uuid,$3::uuid[])`, hash[:], scope.StoreID, sessionIDs).Scan(&raw); err != nil {
		return SessionResults{}, mapReadError(err)
	}
	if err := platform.RequirePermission(ctx, tx, scope, token, "orders:read"); err != nil {
		return SessionResults{}, mapReadError(err)
	}
	return decodeResults(raw, sessionIDs)
}

// decodeResults re-verifies the database projection: exactly the two envelope keys, one item per
// requested id in request order, non-negative counts and amounts, a closed currency vocabulary with
// no duplicate currency per item.
func decodeResults(raw []byte, sessionIDs []string) (SessionResults, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil || len(top) != 2 || top["as_of"] == nil || top["items"] == nil {
		return SessionResults{}, ErrResultsUnavailable
	}
	var out SessionResults
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&out); err != nil {
		return SessionResults{}, ErrResultsUnavailable
	}
	if out.AsOf.IsZero() || len(out.Items) != len(sessionIDs) {
		return SessionResults{}, ErrResultsUnavailable
	}
	for i, item := range out.Items {
		if !command.ValidID(item.SessionID) || item.SessionID != sessionIDs[i] ||
			item.Orders < 0 || item.PaidOrders < 0 || item.MultiSessionOrders < 0 {
			return SessionResults{}, ErrResultsUnavailable
		}
		currencies := map[string]bool{}
		for _, m := range item.Money {
			if !currencyCode.MatchString(m.Currency) || currencies[m.Currency] ||
				m.OrderMinor < 0 || m.PaidMinor < 0 || m.SandboxPaidMinor < 0 {
				return SessionResults{}, ErrResultsUnavailable
			}
			currencies[m.Currency] = true
		}
	}
	return out, nil
}

// mapReadError maps the PT codes the A5-1/A5-3 read definers raise (PT400/PT401/PT403/PT404) to
// stable sentinels and falls back to the write-side mapError for the shared codes (ErrNoRows, 23505,
// 22xxx); retryable serialization codes stay as-is for the HTTP layer's classifier.
func mapReadError(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "PT400":
			return command.ErrInvalid
		case "PT401":
			return platform.ErrUnauthorized
		case "PT403":
			return platform.ErrForbidden
		case "PT404":
			return platform.ErrScopeNotFound
		case "PT503":
			return ErrResultsUnavailable
		case "40001", "40P01", "55P03", "57014":
			return err
		}
	}
	return mapError(err)
}
