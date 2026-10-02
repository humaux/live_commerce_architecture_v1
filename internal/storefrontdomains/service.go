package storefrontdomains

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"livecommerce/internal/command"
	"livecommerce/internal/domains"
	"livecommerce/internal/platform"
)

// ErrUnavailable is any database class this package does not map: the caller answers 503, never a driver message.
var ErrUnavailable = errors.New("storefront domains unavailable")

// Refusals of the merchant domain lifecycle, one sentinel per SQL message (PT409).
var (
	ErrDomainActive         = errors.New("domain already active")
	ErrDomainSuspended      = errors.New("domain suspended")
	ErrDomainDetached       = errors.New("domain detached")
	ErrDomainOwnedElsewhere = errors.New("domain owned by another store")
	ErrReservedHostname     = errors.New("hostname reserved by the platform")
	ErrBaseDomainMissing    = errors.New("store base domain not configured")
	ErrPlatformDomain       = errors.New("platform domain is not merchant-moveable")
)

// DNSInstructions are shown to the merchant once, at request time; nothing here is persisted. EdgeAddresses is the
// platform edge A/AAAA set (stores.<base>) attached for an apex host only, so the merchant points the apex at the
// addresses the verifier accepts; a CNAME host keeps only the CNAME target.
type DNSInstructions struct {
	TXTName       string   `json:"txt_name"`
	TXTValue      string   `json:"txt_value"`
	CNAMETarget   string   `json:"cname_target"`
	Apex          bool     `json:"apex"`
	EdgeAddresses []string `json:"edge_addresses,omitempty"`
}

// RequestResult names the new or refreshed REQUESTED row and its DNS instructions.
type RequestResult struct {
	DomainID string          `json:"domain_id"`
	Version  int64           `json:"version"`
	State    string          `json:"state"`
	Origin   string          `json:"origin"`
	DNS      DNSInstructions `json:"dns"`
}

// DomainRow is one domain of the store (all states); Token is set only for pending rows. Kind is the row's
// authoritative origin class from evidence/origin, never inferred from the suffix: "platform" (the platform
// subdomain) or "custom" (a merchant row).
type DomainRow struct {
	Origin         string  `json:"origin"`
	State          string  `json:"state"`
	Version        int64   `json:"version"`
	Kind           string  `json:"kind"`
	Token          *string `json:"token"`
	VerifyDeadline *string `json:"verify_deadline"`
	Serving        bool    `json:"serving"`
}

// ReadResult lists every domain row of the store, any state.
type ReadResult struct {
	Domains []DomainRow `json:"domains"`
}

// MoveResult is the outcome of a merchant suspend/detach; Changed is false when already in the target state.
type MoveResult struct {
	DomainID string `json:"domain_id"`
	Version  int64  `json:"version"`
	State    string `json:"state"`
	Changed  bool   `json:"changed"`
}

// Querier is the one surface the worker/primary calls need; *pgxpool.Pool and pgx.Tx satisfy it, tests fake it.
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func validAuthority(tx pgx.Tx, scope platform.Scope, token string) bool {
	return tx != nil && command.ValidID(scope.TenantID) && command.ValidID(scope.StoreID) && command.ValidID(scope.PrincipalID) &&
		scope.Revision > 0 && len(token) >= 32 && len(token) <= 512
}

// Request enters (or refreshes) a REQUESTED merchant origin for the store, idempotent per key (command.Run receipt:
// the same key+body replays the first result, so the TXT token/version/audit row are not rotated; a different body
// is ErrConflict). hostname and baseDomain are lower-cased here and re-validated by the definer; the token is
// generated here and echoed back by the definer. edge is the platform edge address set (stores.<base>) resolved by
// the caller outside the transaction; it is attached to the DNS instructions for an apex host only.
func Request(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key, hostname, baseDomain string, edge []string) (RequestResult, error) {
	if ctx == nil || !validAuthority(tx, scope, token) {
		return RequestResult{}, command.ErrInvalid
	}
	host := strings.ToLower(strings.TrimSpace(hostname))
	base := strings.ToLower(strings.TrimSpace(baseDomain))
	if !validHostname(host) || !validHostname(base) {
		return RequestResult{}, command.ErrInvalid
	}
	if underBase(host, base) {
		return RequestResult{}, ErrReservedHostname
	}
	var out RequestResult
	err := command.Run(ctx, tx, scope, "storefront.domain.request", key, struct {
		PrincipalID string `json:"principal_id"`
		Hostname    string `json:"hostname"`
		BaseDomain  string `json:"base_domain"`
	}{scope.PrincipalID, host, base}, &out, func() error {
		return requestEntry(ctx, tx, scope, token, host, base, edge, &out)
	})
	return out, mapError(err)
}

// requestEntry is the pre-command.Run core of Request: it runs the definer and decodes its answer, attaching the
// edge addresses for an apex host. Unit-tested directly (fakeTx has no command.Run receipt surface).
func requestEntry(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, host, base string, edge []string, out *RequestResult) error {
	verification := randomToken()
	hash := sha256.Sum256([]byte(token))
	var raw []byte
	// control.request_merchant_domain (0106, owner commerce_storefront_writer, EXECUTE commerce_runtime): integration:manage
	// checked in SQL against the bearer; REQUESTED row + token + instructions, or a refreshed token on a pending re-request.
	if err := tx.QueryRow(ctx, `SELECT control.request_merchant_domain($1,$2::uuid,$3,$4,$5)`,
		hash[:], scope.StoreID, host, base, verification).Scan(&raw); err != nil {
		return mapError(err)
	}
	if !strictDecode(raw, out) || !command.ValidID(out.DomainID) || out.Version < 1 || out.State != "REQUESTED" ||
		!domains.ValidOrigin(out.Origin) || out.DNS.TXTValue != verification || out.DNS.TXTName != "_lc-verify."+host {
		return ErrUnavailable
	}
	if out.DNS.Apex {
		out.DNS.EdgeAddresses = edge
	}
	return nil
}

// Read returns every domain row of the store (any state). The pending token is the merchant's own; never logged.
func Read(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string) (ReadResult, error) {
	if ctx == nil || !validAuthority(tx, scope, token) {
		return ReadResult{}, command.ErrInvalid
	}
	hash := sha256.Sum256([]byte(token))
	var raw []byte
	// control.read_store_domains (0106): integration:read; every domain row, no proof/evidence.
	if err := tx.QueryRow(ctx, `SELECT control.read_store_domains($1,$2::uuid)`, hash[:], scope.StoreID).Scan(&raw); err != nil {
		return ReadResult{}, mapError(err)
	}
	var out ReadResult
	if !strictDecode(raw, &out) || out.Domains == nil {
		return ReadResult{}, ErrUnavailable
	}
	for _, d := range out.Domains {
		if !domains.ValidOrigin(d.Origin) || d.State == "" || d.Version < 1 || (d.Kind != "platform" && d.Kind != "custom") ||
			(d.Token != nil && !validToken(*d.Token)) {
			return ReadResult{}, ErrUnavailable
		}
	}
	return out, nil
}

// Suspend moves the store's own non-DETACHED origin to SUSPENDED (the resolver denies it on the next request),
// idempotent per key.
func Suspend(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key, origin string) (MoveResult, error) {
	return moveCommand(ctx, tx, scope, token, key, origin, "storefront.domain.suspend",
		`SELECT control.suspend_merchant_domain($1,$2::uuid,$3)`, "SUSPENDED")
}

// Detach finally detaches the store's own origin (never re-bound by update), idempotent per key.
func Detach(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key, origin string) (MoveResult, error) {
	return moveCommand(ctx, tx, scope, token, key, origin, "storefront.domain.detach",
		`SELECT control.detach_merchant_domain($1,$2::uuid,$3)`, "DETACHED")
}

func moveCommand(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key, origin, operation, sql, want string) (MoveResult, error) {
	if ctx == nil || !validAuthority(tx, scope, token) || !domains.ValidOrigin(origin) {
		return MoveResult{}, command.ErrInvalid
	}
	var out MoveResult
	err := command.Run(ctx, tx, scope, operation, key, struct {
		PrincipalID string `json:"principal_id"`
		Origin      string `json:"origin"`
	}{scope.PrincipalID, origin}, &out, func() error {
		return moveEntry(ctx, tx, scope, token, origin, sql, want, &out)
	})
	return out, mapError(err)
}

// moveEntry is the pre-command.Run core of Suspend/Detach: the definer call + strict decode (unit-tested directly).
func moveEntry(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, origin, sql, want string, out *MoveResult) error {
	hash := sha256.Sum256([]byte(token))
	var raw []byte
	if err := tx.QueryRow(ctx, sql, hash[:], scope.StoreID, origin).Scan(&raw); err != nil {
		return mapError(err)
	}
	if !strictDecode(raw, out) || !command.ValidID(out.DomainID) || out.Version < 1 || out.State != want {
		return ErrUnavailable
	}
	return nil
}

// PrimaryOrigin returns the primary ACTIVE origin for the store that origin resolves to (merchant domain when
// ACTIVE, else the platform subdomain), or "" when origin is not ACTIVE or is already primary (no redirect).
func PrimaryOrigin(ctx context.Context, q Querier, origin string) (string, error) {
	if ctx == nil || q == nil || !domains.ValidOrigin(origin) {
		return "", command.ErrInvalid
	}
	var primary *string
	// control.resolve_primary_origin (0106): EXECUTE commerce_buyer_runtime (buyer design read pool).
	if err := q.QueryRow(ctx, `SELECT control.resolve_primary_origin($1)`, origin).Scan(&primary); err != nil {
		return "", mapError(err)
	}
	if primary == nil {
		return "", nil
	}
	if !domains.ValidOrigin(*primary) {
		return "", ErrUnavailable
	}
	return *primary, nil
}

// validHostname is the bare-hostname half of internal/domains.ValidOrigin (lower-case only).
func validHostname(h string) bool {
	if h == "" || len(h) > 253 || h != strings.ToLower(h) || strings.HasSuffix(h, ".localhost") {
		return false
	}
	return domains.ValidOrigin("https://" + h)
}

// underBase mirrors the definer's refusal: a merchant cannot bring a hostname under the platform's own base zone.
func underBase(hostname, baseDomain string) bool {
	return hostname == baseDomain || strings.HasSuffix(hostname, "."+baseDomain)
}

func randomToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func validToken(s string) bool {
	b, err := base64.RawURLEncoding.Strict().DecodeString(s)
	return len(s) == 43 && err == nil && len(b) == 32
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
	if err == nil {
		return nil
	}
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
			case "domain_active":
				return ErrDomainActive
			case "domain_suspended":
				return ErrDomainSuspended
			case "domain_detached":
				return ErrDomainDetached
			case "domain_owned_elsewhere":
				return ErrDomainOwnedElsewhere
			case "reserved hostname":
				return ErrReservedHostname
			case "base domain not configured":
				return ErrBaseDomainMissing
			case "platform_domain":
				return ErrPlatformDomain
			}
			return command.ErrConflict
		case "40001", "40P01", "55P03", "57014", "23505":
			return err // the HTTP layer maps deadlocks/lock timeouts to a retry and unique races to conflict
		}
		return ErrUnavailable
	}
	if errors.Is(err, platform.ErrUnauthorized) || errors.Is(err, platform.ErrForbidden) ||
		errors.Is(err, platform.ErrScopeNotFound) || errors.Is(err, command.ErrInvalid) || errors.Is(err, command.ErrConflict) ||
		errors.Is(err, ErrDomainActive) || errors.Is(err, ErrDomainSuspended) || errors.Is(err, ErrDomainDetached) ||
		errors.Is(err, ErrDomainOwnedElsewhere) || errors.Is(err, ErrReservedHostname) || errors.Is(err, ErrBaseDomainMissing) ||
		errors.Is(err, ErrPlatformDomain) ||
		errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return err
	}
	return ErrUnavailable
}
