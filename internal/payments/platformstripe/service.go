// Purpose: merchant read and enable/disable of platform-Stripe card payments for one store (contract §3.3, §5).
// Depends on: SQL payments.read_platform_stripe, payments.set_platform_stripe (0137; registry-writer definers, EXECUTE
//   commerce_runtime); env COMMERCE_PAYMENT_PROFILE (chosen by cmd/api, passed in; never from the request).
// Used by: internal/httpapi/payment_card.go (GET/PUT /v1/admin/stores/{store_id}/payments/card).
// Invariants: I01 (store/tenant from the token), I05 (no amount from the client), I16 (disable always allowed).
// Status: MOCK + REAL_PG; no Stripe call is made here.

package platformstripe

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

// ErrUnavailable is any unclassified failure (driver error, malformed projection): a retryable 503, never a message.
var ErrUnavailable = errors.New("platformstripe: unavailable")

// Error is a coded refusal of the SQL definers (contract §3.3 codes). Status is the HTTP status the route returns.
type Error struct {
	Status int
	Code   string
}

func (e *Error) Error() string { return e.Code }

var codePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{2,59}$`)

// Summary is payments.read_platform_stripe: merchant-safe, no account id, key or approval data.
type Summary struct {
	PlatformState        string  `json:"platform_state"`
	StoreState           string  `json:"store_state"`
	Allowed              bool    `json:"allowed"`
	TermsVersion         *string `json:"terms_version"`
	AcceptedTermsVersion *string `json:"accepted_terms_version"`
	DisplayName          *string `json:"display_name"`
	DescriptorPreview    *string `json:"descriptor_preview"`
	Currency             *string `json:"currency"`
	MinMinor             *int64  `json:"min_minor"`
	MaxMinor             *int64  `json:"max_minor"`
	Version              int64   `json:"version"`
}

// Input is the PUT body. The terms version is the one the merchant read; the descriptor suffix is optional.
type Input struct {
	Enabled          bool    `json:"enabled"`
	TermsVersion     string  `json:"terms_version"`
	DescriptorSuffix *string `json:"descriptor_suffix"`
	ExpectedVersion  int64   `json:"expected_version"`
}

// Result is payments.set_platform_stripe: the state after the call.
type Result struct {
	State             string  `json:"state"`
	Version           int64   `json:"version"`
	MaxMinor          *int64  `json:"max_minor"`
	Currency          *string `json:"currency"`
	DescriptorPreview *string `json:"descriptor_preview"`
}

func validProfile(p string) bool { return p == "PROVIDER_MOCK" || p == "SANDBOX" || p == "LIVE" }

func tokenHash(token string) ([]byte, bool) {
	if len(token) < 32 || len(token) > 512 {
		return nil, false
	}
	h := sha256.Sum256([]byte(token))
	return h[:], true
}

// Read returns the store's platform-Stripe view. SQL payments.read_platform_stripe (integration:read, fresh final fence).
func Read(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, profile string) (Summary, error) {
	hash, ok := tokenHash(token)
	if tx == nil || !ok || !command.ValidID(scope.StoreID) || !validProfile(profile) {
		return Summary{}, command.ErrInvalid
	}
	var raw []byte
	// payments.read_platform_stripe (0137): the profile only selects SANDBOX vs LIVE; every row comes from the database.
	if err := tx.QueryRow(ctx, `SELECT payments.read_platform_stripe($1,$2::uuid,$3)`, hash, scope.StoreID, profile).Scan(&raw); err != nil {
		return Summary{}, mapError(err)
	}
	var out Summary
	if err := strictDecode(raw, &out); err != nil {
		return Summary{}, ErrUnavailable
	}
	return out, nil
}

// Set enables or disables card payments for the store. SQL payments.set_platform_stripe (billing:manage, operator
// allowlist, one transaction creating binding/derived account/credential copy/qualification/card heads on first enable).
// A nil return of the transaction is the COMMIT acknowledgement the route waits for.
func Set(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, profile string, in Input) (Result, error) {
	hash, ok := tokenHash(token)
	if tx == nil || !ok || !command.ValidID(scope.StoreID) || !validProfile(profile) || in.ExpectedVersion < 0 ||
		in.ExpectedVersion >= 1<<62 || len(in.TermsVersion) > 64 || (in.DescriptorSuffix != nil && len(*in.DescriptorSuffix) > 64) {
		return Result{}, command.ErrInvalid
	}
	var suffix any
	if in.DescriptorSuffix != nil {
		suffix = *in.DescriptorSuffix
	}
	var raw []byte
	// payments.set_platform_stripe (0137): CAS on the enrollment version; replay of the stored state creates nothing.
	if err := tx.QueryRow(ctx, `SELECT payments.set_platform_stripe($1,$2::uuid,$3,$4,$5,$6,$7)`,
		hash, scope.StoreID, profile, in.Enabled, in.TermsVersion, suffix, in.ExpectedVersion).Scan(&raw); err != nil {
		return Result{}, mapError(err)
	}
	var out Result
	if err := strictDecode(raw, &out); err != nil {
		return Result{}, ErrUnavailable
	}
	return out, nil
}

// mapError turns the definers' SQLSTATEs into coded refusals: PT403 (forbidden, or a coded 403), PT409 and PT422 carry
// the contract code in the message; PT400/22023 are invalid input; PT401/PT404 are authority errors.
func mapError(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "PT400", "22023":
			return command.ErrInvalid
		case "PT401":
			return platform.ErrUnauthorized
		case "PT404":
			return platform.ErrScopeNotFound
		case "PT403":
			if pg.Message != "forbidden" && codePattern.MatchString(pg.Message) {
				return &Error{Status: http.StatusForbidden, Code: pg.Message}
			}
			return platform.ErrForbidden
		case "PT409":
			if codePattern.MatchString(pg.Message) {
				return &Error{Status: http.StatusConflict, Code: pg.Message}
			}
			return command.ErrConflict
		case "PT422":
			if codePattern.MatchString(pg.Message) {
				return &Error{Status: http.StatusUnprocessableEntity, Code: pg.Message}
			}
			return command.ErrInvalid
		}
	}
	if errors.Is(err, platform.ErrUnauthorized) || errors.Is(err, platform.ErrForbidden) || errors.Is(err, platform.ErrScopeNotFound) {
		return err
	}
	return ErrUnavailable
}

func strictDecode(raw []byte, into any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	return dec.Decode(into)
}
