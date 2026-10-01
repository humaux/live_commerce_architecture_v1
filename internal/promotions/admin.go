// admin.go is the merchant side of discount codes (storefront-v2 section F): list, create, update (which is also pause). Each call is one SQL
// definer of migration 0091 (promotions.admin_list / admin_create / admin_update) inside the caller's platform.WithScope transaction; the
// definer re-authorizes with identity.resolve_access (pricing:read / pricing:write), takes the per-store advisory lock, writes the
// idempotent receipt and the audit row, and fences the access again before answering.
//
// Non-goals: no buyer data (a redemption row never leaves SQL), no code deletion (a code is paused, never deleted, so its redemptions
// keep a parent), no touch of orders already placed.

package promotions

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

// ErrUnavailable is a definer answer Go could not trust (malformed or unexpected shape): a 503, never a guess.
var ErrUnavailable = errors.New("promotions unavailable")

var adminKey = regexp.MustCompile(`^[A-Za-z0-9_.:-]{8,128}$`)

// Promotion is one code as the merchant sees it. Used is the derived active usage count (orders not CANCELLED).
type Promotion struct {
	ID               string     `json:"id"`
	Code             string     `json:"code"`
	Kind             string     `json:"kind"`
	Percent          *int64     `json:"percent"`
	FixedMinor       *int64     `json:"fixed_minor"`
	MinSubtotalMinor int64      `json:"min_subtotal_minor"`
	StartsAt         *time.Time `json:"starts_at"`
	EndsAt           *time.Time `json:"ends_at"`
	TotalLimit       *int64     `json:"total_limit"`
	PerBuyerLimit    *int64     `json:"per_buyer_limit"`
	Status           string     `json:"status"`
	Version          int64      `json:"version"`
	Used             int64      `json:"used"`
	CreatedAt        time.Time  `json:"created_at"`
}

// Fields are the editable properties of a code (every key present on the wire; nullable ones may be null).
type Fields struct {
	Kind             string     `json:"kind"`
	Percent          *int64     `json:"percent"`
	FixedMinor       *int64     `json:"fixed_minor"`
	MinSubtotalMinor int64      `json:"min_subtotal_minor"`
	StartsAt         *time.Time `json:"starts_at"`
	EndsAt           *time.Time `json:"ends_at"`
	TotalLimit       *int64     `json:"total_limit"`
	PerBuyerLimit    *int64     `json:"per_buyer_limit"`
	Status           string     `json:"status"`
}

// CreateInput is the exact POST body. The code text is immutable afterwards.
type CreateInput struct {
	Code string `json:"code"`
	Fields
}

// UpdateInput is the exact POST .../{id} body: version CAS plus every editable field.
type UpdateInput struct {
	ExpectedVersion int64 `json:"expected_version"`
	Fields
}

// CreateFields / UpdateFields list the wire keys of the two bodies for the transport's strict decoder; Nullable is the subset that may be null.
var (
	CreateFields = []string{"code", "kind", "percent", "fixed_minor", "min_subtotal_minor", "starts_at", "ends_at", "total_limit", "per_buyer_limit", "status"}
	UpdateFields = []string{"expected_version", "kind", "percent", "fixed_minor", "min_subtotal_minor", "starts_at", "ends_at", "total_limit", "per_buyer_limit", "status"}
	Nullable     = []string{"percent", "fixed_minor", "starts_at", "ends_at", "total_limit", "per_buyer_limit"}
)

// adminError maps the definers' SQLSTATEs: coded refusals (409/422) first, authority (PT401/403/404) to the platform sentinels.
func adminError(err error) error {
	var coded *Coded
	if errors.As(err, &coded) {
		return err
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "PT401":
			return platform.ErrUnauthorized
		case "PT403":
			return platform.ErrForbidden
		case "PT404":
			return platform.ErrScopeNotFound
		}
		if mapped := mapError(err); mapped != err {
			return mapped
		}
	}
	if errors.Is(err, platform.ErrUnauthorized) || errors.Is(err, platform.ErrForbidden) || errors.Is(err, platform.ErrScopeNotFound) ||
		errors.Is(err, command.ErrInvalid) {
		return err
	}
	return ErrUnavailable
}

// fitsColumns is a pure range guard so an absurd number is a coded 422 instead of a driver encode error; SQL check_fields owns every rule.
func (f Fields) fitsColumns() bool {
	fits := func(v *int64) bool { return v == nil || (*v >= 0 && *v <= 1<<31-1) }
	return fits(f.Percent) && fits(f.TotalLimit) && fits(f.PerBuyerLimit) && (f.FixedMinor == nil || *f.FixedMinor >= 0) && f.MinSubtotalMinor >= 0
}

func validAdmin(tx pgx.Tx, scope platform.Scope, token string) bool {
	return tx != nil && command.ValidID(scope.TenantID) && command.ValidID(scope.StoreID) && command.ValidID(scope.PrincipalID) &&
		scope.Revision > 0 && len(token) >= 32 && len(token) <= 512
}

func strictDecode(raw []byte, into any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil {
		return ErrUnavailable
	}
	return nil
}

func digestOf(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, command.ErrInvalid
	}
	sum := sha256.Sum256(raw)
	return sum[:], nil
}

// List returns every code of the store with its derived usage (pricing:read).
func List(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string) ([]Promotion, error) {
	if !validAdmin(tx, scope, token) {
		return nil, command.ErrInvalid
	}
	hash := sha256.Sum256([]byte(token))
	var raw []byte
	// promotions.admin_list (0091): pricing:read, GUCs from resolve_access, final access fence.
	if err := tx.QueryRow(ctx, `SELECT promotions.admin_list($1,$2::uuid)`, hash[:], scope.StoreID).Scan(&raw); err != nil {
		return nil, adminError(err)
	}
	var out struct {
		Promotions []Promotion `json:"promotions"`
	}
	if err := strictDecode(raw, &out); err != nil || out.Promotions == nil {
		return nil, ErrUnavailable
	}
	return out.Promotions, nil
}

// Create adds a code (pricing:write). 409 promo_exists on a duplicate code, 422 invalid_promotion on a rule violation.
func Create(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key string, in CreateInput) (Promotion, error) {
	if !validAdmin(tx, scope, token) || !adminKey.MatchString(key) {
		return Promotion{}, command.ErrInvalid
	}
	code, ok := Normalize(in.Code)
	if !ok || !in.Fields.fitsColumns() {
		return Promotion{}, &Coded{Status: http.StatusUnprocessableEntity, Code: "invalid_promotion"}
	}
	in.Code = code // the canonical spelling is what the receipt digest and the row hold
	digest, err := digestOf(struct {
		Op string `json:"op"`
		CreateInput
	}{"promotions.create", in})
	if err != nil {
		return Promotion{}, err
	}
	hash := sha256.Sum256([]byte(token))
	var raw []byte
	// promotions.admin_create (0091): pricing:write, per-store advisory lock, idempotent receipt, audit row.
	if err = tx.QueryRow(ctx, `SELECT promotions.admin_create($1,$2::uuid,$3,$4,$5,$6,$7::integer,$8,$9,$10,$11,$12::integer,$13::integer,$14)`,
		hash[:], scope.StoreID, key, digest, in.Code, in.Kind, in.Percent, in.FixedMinor, in.MinSubtotalMinor, in.StartsAt, in.EndsAt,
		in.TotalLimit, in.PerBuyerLimit, in.Status).Scan(&raw); err != nil {
		return Promotion{}, adminError(err)
	}
	var out Promotion
	if err = strictDecode(raw, &out); err != nil || out.Version < 1 || out.Code != code {
		return Promotion{}, ErrUnavailable
	}
	return out, nil
}

// Update replaces the editable fields of one code with version CAS (pricing:write). Pausing is Status "paused". 409 version_changed on a
// stale version; the version bump makes any in-flight quote of this code answer promo_changed at checkout.
func Update(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key, id string, in UpdateInput) (Promotion, error) {
	if !validAdmin(tx, scope, token) || !adminKey.MatchString(key) || !command.ValidID(id) || in.ExpectedVersion < 1 || in.ExpectedVersion >= 1<<62 {
		return Promotion{}, command.ErrInvalid
	}
	if !in.Fields.fitsColumns() {
		return Promotion{}, &Coded{Status: http.StatusUnprocessableEntity, Code: "invalid_promotion"}
	}
	digest, err := digestOf(struct {
		Op string `json:"op"`
		ID string `json:"id"`
		UpdateInput
	}{"promotions.update", id, in})
	if err != nil {
		return Promotion{}, err
	}
	hash := sha256.Sum256([]byte(token))
	var raw []byte
	// promotions.admin_update (0091): pricing:write, CAS, code row FOR UPDATE (serialises with redeem), audit row, idempotent receipt.
	if err = tx.QueryRow(ctx, `SELECT promotions.admin_update($1,$2::uuid,$3,$4,$5::uuid,$6,$7,$8::integer,$9,$10,$11,$12,$13::integer,$14::integer,$15)`,
		hash[:], scope.StoreID, key, digest, id, in.ExpectedVersion, in.Kind, in.Percent, in.FixedMinor, in.MinSubtotalMinor, in.StartsAt,
		in.EndsAt, in.TotalLimit, in.PerBuyerLimit, in.Status).Scan(&raw); err != nil {
		return Promotion{}, adminError(err)
	}
	var out Promotion
	if err = strictDecode(raw, &out); err != nil || out.ID != id || out.Version != in.ExpectedVersion+1 {
		return Promotion{}, ErrUnavailable
	}
	return out, nil
}
