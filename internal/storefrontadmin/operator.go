// operator.go is the platform-operator half: bind, suspend and detach a store's public origin, and read the status.
// Callers are cmd/store-admin only, on the lc_store_registrar login (authority commerce_storefront_registrar, EXECUTE
// on control.operator_* is the authority check). The operator attests the ownership/TLS proof (--evidence,
// --valid-until = certificate notAfter); nothing here can verify DNS or TLS.
//
// Non-goals: no merchant HTTP exposure, no publication write (merchant.go), no wildcard/on-demand TLS, no re-bind of
// a DETACHED origin (SQL refuses: origin is globally UNIQUE and the contract forbids rebind by update).

package storefrontadmin

import (
	"context"
	"errors"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/command"
	"livecommerce/internal/domains"
)

// Operator-visible refusals of the bind lifecycle; the CLI prints a fixed code for each.
var (
	ErrDomainDetached       = errors.New("domain detached")
	ErrDomainOwnedElsewhere = errors.New("domain owned by another store")
	ErrNoOwner              = errors.New("store has no owner principal")
)

// MaxProofLifetime caps --valid-until: a public-CA certificate lives at most 398 days, rounded up to 400, so a
// typo cannot mint an ACTIVE proof that never expires. The SQL enforces the same bound against its own clock.
const MaxProofLifetime = 400 * 24 * time.Hour

// Querier is the one method the operator calls need; *pgxpool.Pool and pgx.Tx satisfy it, tests fake it.
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// BindResult names the domain row; Renewed is true when the origin was already ACTIVE (proof restamped).
type BindResult struct {
	DomainID string `json:"domain_id"`
	Version  int64  `json:"version"`
	State    string `json:"state"`
	Renewed  bool   `json:"renewed"`
}

// MoveResult is the outcome of suspend/detach; Changed is false when the origin already had the target state.
type MoveResult struct {
	DomainID string `json:"domain_id"`
	Version  int64  `json:"version"`
	State    string `json:"state"`
	Changed  bool   `json:"changed"`
}

// DomainStatus is one domain row of Status (no evidence_ref, no proof timestamps).
type DomainStatus struct {
	Origin     string  `json:"origin"`
	State      string  `json:"state"`
	Version    int64   `json:"version"`
	ValidUntil *string `json:"valid_until"`
	Serving    bool    `json:"serving"`
}

// StoreStatus is the operator's read of one store: ids, versions, states and origins only.
type StoreStatus struct {
	StoreID   string         `json:"store_id"`
	Published bool           `json:"published"`
	Version   int64          `json:"version"`
	Domains   []DomainStatus `json:"domains"`
}

// ValidEvidence mirrors the 0020 evidence_ref CHECK: 1..240 characters, at least one non-space, no control characters.
func ValidEvidence(s string) bool {
	if n := utf8.RuneCountInString(s); n < 1 || n > 240 || !utf8.ValidString(s) {
		return false
	}
	visible := false
	for _, r := range s {
		if unicode.IsControl(r) {
			return false
		}
		if !unicode.IsSpace(r) {
			visible = true
		}
	}
	return visible
}

// BindDomain makes origin ACTIVE for the store (creating or advancing/renewing the row).
func BindDomain(ctx context.Context, q Querier, storeID, origin, evidence string, validUntil, now time.Time) (BindResult, error) {
	if ctx == nil || q == nil || !command.ValidID(storeID) || !domains.ValidOrigin(origin) || !ValidEvidence(evidence) ||
		!validUntil.After(now) || validUntil.After(now.Add(MaxProofLifetime)) {
		return BindResult{}, command.ErrInvalid
	}
	var raw []byte
	// control.operator_bind_domain (0081, owner commerce_storefront_writer, EXECUTE commerce_storefront_registrar):
	// one row per origin under an advisory lock; ACTIVE with proof stamped by the DB clock; audit operator.domain_bound.
	if err := q.QueryRow(ctx, `SELECT control.operator_bind_domain($1::uuid,$2,$3,$4)`, storeID, origin, evidence, validUntil.UTC()).Scan(&raw); err != nil {
		return BindResult{}, mapError(err)
	}
	var out BindResult
	if !strictDecode(raw, &out) || !command.ValidID(out.DomainID) || out.Version < 1 || out.State != "ACTIVE" {
		return BindResult{}, ErrUnavailable
	}
	return out, nil
}

// SuspendDomain moves the origin to SUSPENDED: the resolver denies it on the next request.
func SuspendDomain(ctx context.Context, q Querier, origin string) (MoveResult, error) {
	// control.operator_suspend_domain (0081): same owner/authority as BindDomain; audit operator.domain_suspended.
	return move(ctx, q, origin, `SELECT control.operator_suspend_domain($1)`, "SUSPENDED")
}

// DetachDomain moves the origin to DETACHED for good (never re-bound by update).
func DetachDomain(ctx context.Context, q Querier, origin string) (MoveResult, error) {
	// control.operator_detach_domain (0081): same owner/authority as BindDomain; audit operator.domain_detached.
	return move(ctx, q, origin, `SELECT control.operator_detach_domain($1)`, "DETACHED")
}

func move(ctx context.Context, q Querier, origin, sql, want string) (MoveResult, error) {
	if ctx == nil || q == nil || !domains.ValidOrigin(origin) {
		return MoveResult{}, command.ErrInvalid
	}
	var raw []byte
	if err := q.QueryRow(ctx, sql, origin).Scan(&raw); err != nil {
		return MoveResult{}, mapError(err)
	}
	var out MoveResult
	if !strictDecode(raw, &out) || !command.ValidID(out.DomainID) || out.Version < 1 || out.State != want {
		return MoveResult{}, ErrUnavailable
	}
	return out, nil
}

// Status reads one store's publication and domains (read-only).
func Status(ctx context.Context, q Querier, storeID string) (StoreStatus, error) {
	if ctx == nil || q == nil || !command.ValidID(storeID) {
		return StoreStatus{}, command.ErrInvalid
	}
	var raw []byte
	// control.operator_storefront_status (0081): read-only; no evidence_ref, proof timestamps or PII.
	if err := q.QueryRow(ctx, `SELECT control.operator_storefront_status($1::uuid)`, storeID).Scan(&raw); err != nil {
		return StoreStatus{}, mapError(err)
	}
	var out StoreStatus
	if !strictDecode(raw, &out) || out.StoreID != storeID || out.Version < 0 || out.Domains == nil {
		return StoreStatus{}, ErrUnavailable
	}
	for _, d := range out.Domains {
		if !domains.ValidOrigin(d.Origin) || d.State == "" || d.Version < 1 {
			return StoreStatus{}, ErrUnavailable
		}
	}
	return out, nil
}
