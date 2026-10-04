// operator.go is the platform-operator half: bind, suspend and detach a store's public origin, and read the status.
// Callers are cmd/store-admin only, on the lc_store_registrar login (authority commerce_storefront_registrar, EXECUTE
// on control.operator_* is the authority check). The operator attests the ownership/TLS proof (--evidence,
// --valid-until = certificate notAfter); nothing here can verify DNS or TLS.
//
// Non-goals: no merchant HTTP exposure, no publication write (merchant.go), no wildcard/on-demand TLS. A DETACHED origin
// re-binds only through BindDomain with a different --evidence (integrator ruling; the SQL enforces it).

package storefrontadmin

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"livecommerce/internal/command"
	"livecommerce/internal/domains"
	"livecommerce/internal/storehandles"
)

// Operator-visible refusals of the bind lifecycle; the CLI prints a fixed code for each.
var (
	ErrDomainDetached       = errors.New("domain detached") // detach refused, or a re-bind with the same evidence_ref
	ErrDomainOwnedElsewhere = errors.New("domain owned by another store")
	ErrNoOwner              = errors.New("store has no owner principal")
	ErrStorePublished       = errors.New("store was already published") // handle change refused without --after-publish (Decision 1)
)

// MaxProofLifetime caps --valid-until: a public-CA certificate lives at most 398 days, rounded up to 400, so a
// typo cannot mint an ACTIVE proof that never expires. The SQL enforces the same bound against its own clock.
const MaxProofLifetime = 400 * 24 * time.Hour

// Querier is the one method the operator calls need; *pgxpool.Pool and pgx.Tx satisfy it, tests fake it.
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Tx is the transaction surface HandleSet needs: set the base-domain GUC (SET LOCAL) and run the definer in one
// transaction. pgx.Tx satisfies it.
type Tx interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// Beginner opens a Tx for HandleSet. The CLI adapts its *pgxpool.Pool (whose Begin returns pgx.Tx, a Tx) to this
// interface; tests fake it with a bare in-memory transaction.
type Beginner interface {
	Begin(ctx context.Context) (Tx, error)
}

// BindResult names the domain row; Renewed is true when the origin was already ACTIVE (proof restamped).
type BindResult struct {
	DomainID string `json:"domain_id"`
	Version  int64  `json:"version"`
	State    string `json:"state"`
	Renewed  bool   `json:"renewed"`
	Rebound  bool   `json:"rebound"` // true: a DETACHED origin re-bound with renewed proof (any store)
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

// BindDomain makes origin ACTIVE for the store (creating, advancing/renewing, or re-binding a DETACHED row with new evidence).
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

// HandleSetResult names the outcome of an operator handle change; Changed is false when the store already had the
// requested handle (a no-op that writes nothing).
type HandleSetResult struct {
	StoreID      string `json:"store_id"`
	Handle       string `json:"handle"`
	Origin       string `json:"origin"`
	Changed      bool   `json:"changed"`
	AfterPublish bool   `json:"after_publish"`
}

// handleSetGUC sets the lc.store_base_domain GUC with SET LOCAL semantics. It runs as its own statement inside the
// HandleSet transaction: PostgreSQL applies a set_config change only once the current statement finishes, so a value
// set earlier in the SAME statement is not yet visible to current_setting — the definer must read it from a later
// statement in the same transaction (the codebase-wide idiom: tx.Exec(set_config(...true)) then tx.QueryRow).
const handleSetGUC = `SELECT set_config('lc.store_base_domain', $1, true)`

// HandleSet changes a store's handle (cmd/store-admin handle-set; Decision 1, P1-4). baseDomain is the platform base
// zone from LC_STORE_BASE_DOMAIN, lower-cased here and re-validated by the definer. The definer refuses when the store
// was ever published unless afterPublish is set; changing detaches the old platform origin and writes the new ACTIVE
// row in the same transaction.
func HandleSet(ctx context.Context, b Beginner, storeID, handle string, afterPublish bool, baseDomain string) (HandleSetResult, error) {
	base := strings.ToLower(strings.TrimSpace(baseDomain))
	if ctx == nil || b == nil || !command.ValidID(storeID) || !storehandles.Valid(handle) || base == "" {
		return HandleSetResult{}, command.ErrInvalid
	}
	tx, err := b.Begin(ctx)
	if err != nil {
		return HandleSetResult{}, mapError(err)
	}
	defer tx.Rollback(ctx)
	// control.operator_set_store_handle (0106, owner commerce_storefront_writer, EXECUTE commerce_storefront_registrar):
	// validate exactly like assignment, refuse store_published without the override, change the handle, detach the old
	// platform origin and write the new ACTIVE row in one transaction, audit operator.handle_set. The definer reads the
	// base zone from the lc.store_base_domain GUC set transaction-locally here (unset = PT409 base domain not configured).
	if _, err := tx.Exec(ctx, handleSetGUC, base); err != nil {
		return HandleSetResult{}, mapError(err)
	}
	var raw []byte
	if err := tx.QueryRow(ctx, `SELECT control.operator_set_store_handle($1::uuid, $2, $3)`, storeID, handle, afterPublish).Scan(&raw); err != nil {
		return HandleSetResult{}, mapError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return HandleSetResult{}, mapError(err)
	}
	var out HandleSetResult
	if !strictDecode(raw, &out) || out.StoreID != storeID || out.Handle != handle || (out.Changed && !domains.ValidOrigin(out.Origin)) {
		return HandleSetResult{}, ErrUnavailable
	}
	return out, nil
}
