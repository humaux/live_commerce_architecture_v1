// methods.go owns historical merchant payment-method reads and permanent unavailable diagnostics.
// Depends on scoped pricing/account metadata; retired configuration writes never reach PG.

package payments

import (
	"context"
	"errors"
	"math"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"livecommerce/internal/command"
	"livecommerce/internal/platform"
	"livecommerce/internal/pricing"
)

const provider = "payuni" // removed provider — read-only, never dispatched

type MethodInput struct {
	MarketID        string `json:"market_id"`
	Country         string `json:"country"`
	Code            string `json:"code"`
	Environment     string `json:"environment"`
	ConnectionID    string `json:"connection_id"`
	BindingVersion  int64  `json:"binding_version"`
	ExpectedVersion int64  `json:"expected_version"`
	NameHans        string `json:"name_hans"`
	NameHant        string `json:"name_hant"`
	NameEN          string `json:"name_en"`
	Enabled         bool   `json:"enabled"`
	Visible         bool   `json:"visible"`
	SortOrder       int    `json:"sort_order"`
	MinAmountMinor  int64  `json:"min_amount_minor"`
	MaxAmountMinor  int64  `json:"max_amount_minor"`
}

type Method struct {
	MarketID       string `json:"market_id"`
	Country        string `json:"country"`
	Code           string `json:"code"`
	Version        int64  `json:"version"`
	Provider       string `json:"provider"`
	Environment    string `json:"environment"`
	ConnectionID   string `json:"connection_id"`
	BindingVersion int64  `json:"binding_version"`
	Currency       string `json:"currency"`
	NameHans       string `json:"name_hans"`
	NameHant       string `json:"name_hant"`
	NameEN         string `json:"name_en"`
	Enabled        bool   `json:"enabled"`
	Visible        bool   `json:"visible"`
	SortOrder      int    `json:"sort_order"`
	MinAmountMinor int64  `json:"min_amount_minor"`
	MaxAmountMinor int64  `json:"max_amount_minor"`
}

type CheckInput struct {
	MarketID        string `json:"market_id"`
	Country         string `json:"country"`
	Code            string `json:"code"`
	ExpectedVersion int64  `json:"expected_version"`
	Environment     string `json:"environment"`
	Currency        string `json:"currency"`
	AmountMinor     int64  `json:"amount_minor"`
}

type Availability struct {
	Available         bool     `json:"available"`
	MethodVersion     int64    `json:"method_version"`
	BindingVersion    int64    `json:"binding_version"`
	CredentialVersion int64    `json:"credential_version"`
	Reasons           []string `json:"reasons"`
}

func SetMethod(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key string, in MethodInput) (out Method, err error) {
	if !validMethodInput(in) {
		return out, command.ErrInvalid
	}
	if in.Enabled {
		return out, command.ErrConflict
	}
	return out, command.ErrInvalid
}

func GetMethod(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, marketID, country, code string) (out Method, err error) {
	if !validTarget(marketID, country, code) {
		return out, command.ErrInvalid
	}
	if err = authorize(ctx, tx, scope, token, "integration:read"); err != nil {
		return out, err
	}
	out, err = readCurrent(ctx, tx, scope, marketID, country, code, 0)
	if err != nil {
		return Method{}, mapError(err)
	}
	if err = authorize(ctx, tx, scope, token, "integration:read"); err != nil {
		return Method{}, err
	}
	return out, nil
}

func InspectMethod(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string, in CheckInput) (out Availability, err error) {
	if !validCheckInput(in) {
		return out, command.ErrInvalid
	}
	if err = authorize(ctx, tx, scope, token, "integration:read"); err != nil {
		return out, err
	}
	market, err := pricing.LockMarket(ctx, tx, scope.TenantID, scope.StoreID, in.MarketID)
	if err != nil {
		return Availability{}, err
	}
	method, exists, err := lockCurrent(ctx, tx, scope, in.MarketID, in.Country, in.Code, false)
	if err != nil {
		return Availability{}, err
	}
	if !exists {
		return Availability{}, command.ErrNotFound
	}
	var bindingEnabled bool
	if method.ConnectionID != "" {
		account, accountErr := lockAccount(ctx, tx, scope, method.ConnectionID)
		if accountErr != nil {
			return Availability{}, accountErr
		}
		out.CredentialVersion = account.CredentialVersion
		out.BindingVersion, bindingEnabled, err = lockBinding(ctx, tx, scope, account.BindingID)
		if err != nil {
			return Availability{}, err
		}
	}
	out = diagnose(method, market.Active, in, out.BindingVersion, out.CredentialVersion, bindingEnabled)
	if err = authorize(ctx, tx, scope, token, "integration:read"); err != nil {
		return Availability{}, err
	}
	return out, nil
}

// validTarget parses the stored method vocabulary for historical reads and diagnostics only.
func validTarget(marketID, country, code string) bool {
	if !command.ValidID(marketID) || country != "TW" {
		return false
	}
	switch code {
	case "payuni_credit", "payuni_installment", "payuni_atm", "payuni_cvs", "payuni_linepay":
		return true
	default:
		return false
	}
}

func validMethodInput(in MethodInput) bool {
	if !validTarget(in.MarketID, in.Country, in.Code) ||
		(in.Environment != "SANDBOX" && in.Environment != "LIVE") ||
		in.ExpectedVersion < 0 || in.ExpectedVersion == math.MaxInt64 ||
		!printable(in.NameHans) || !printable(in.NameHant) || !printable(in.NameEN) ||
		in.SortOrder < 0 || in.SortOrder > 1000 || in.MinAmountMinor < 1 ||
		in.MaxAmountMinor < in.MinAmountMinor || in.MaxAmountMinor > command.MaxMoney {
		return false
	}
	return (in.ConnectionID == "" && in.BindingVersion == 0) ||
		(command.ValidID(in.ConnectionID) && in.BindingVersion > 0)
}

func validCheckInput(in CheckInput) bool {
	return validTarget(in.MarketID, in.Country, in.Code) && in.ExpectedVersion > 0 &&
		(in.Environment == "SANDBOX" || in.Environment == "LIVE") &&
		len(in.Currency) == 3 && in.Currency[0] >= 'A' && in.Currency[0] <= 'Z' &&
		in.Currency[1] >= 'A' && in.Currency[1] <= 'Z' &&
		in.Currency[2] >= 'A' && in.Currency[2] <= 'Z' &&
		in.AmountMinor >= 1 && in.AmountMinor <= command.MaxMoney
}

func printable(s string) bool {
	if !utf8.ValidString(s) || strings.TrimSpace(s) == "" || utf8.RuneCountInString(s) > 120 {
		return false
	}
	for _, r := range s {
		if !unicode.IsPrint(r) {
			return false
		}
	}
	return true
}

func diagnose(method Method, marketActive bool, in CheckInput, bindingVersion, credentialVersion int64, bindingEnabled bool) Availability {
	out := Availability{MethodVersion: method.Version, BindingVersion: bindingVersion,
		CredentialVersion: credentialVersion, Reasons: make([]string, 0, 12)}
	add := func(failed bool, reason string) {
		if failed {
			out.Reasons = append(out.Reasons, reason)
		}
	}
	add(method.Version != in.ExpectedVersion, "METHOD_VERSION_CHANGED")
	add(!method.Enabled, "METHOD_DISABLED")
	add(!method.Visible, "METHOD_HIDDEN")
	add(!marketActive, "MARKET_INACTIVE")
	add(method.Environment != in.Environment, "ENVIRONMENT_MISMATCH")
	add(method.Currency != in.Currency, "CURRENCY_MISMATCH")
	add(in.AmountMinor < method.MinAmountMinor || in.AmountMinor > method.MaxAmountMinor, "AMOUNT_OUT_OF_RANGE")
	add(method.ConnectionID == "", "CONNECTION_MISSING")
	if method.ConnectionID != "" {
		add(method.BindingVersion != bindingVersion, "BINDING_VERSION_CHANGED")
		add(!bindingEnabled, "BINDING_DISABLED")
		add(true, "CREDENTIALS_UNVERIFIED")
	}
	// The removed adapter is permanently unavailable, including historically enabled rows.
	out.Reasons = append(out.Reasons, "ADAPTER_UNAVAILABLE")
	return out
}

func readCurrent(ctx context.Context, tx pgx.Tx, scope platform.Scope, marketID, country, code string, version int64) (Method, error) {
	var out Method
	if version == 0 {
		err := tx.QueryRow(ctx, `SELECT current_version FROM payments.method_heads
			WHERE tenant_id=$1 AND store_id=$2 AND market_id=$3 AND country=$4 AND code=$5`,
			scope.TenantID, scope.StoreID, marketID, country, code).Scan(&version)
		if err != nil {
			return out, err
		}
	}
	err := tx.QueryRow(ctx, `SELECT market_id::text,country,code,version,provider,environment,
		coalesce(connection_id::text,''),coalesce(binding_version,0),currency,
		name_hans,name_hant,name_en,enabled,visible,sort_order,min_amount_minor,max_amount_minor
		FROM payments.method_versions WHERE tenant_id=$1 AND store_id=$2 AND market_id=$3
		AND country=$4 AND code=$5 AND version=$6`, scope.TenantID, scope.StoreID,
		marketID, country, code, version).Scan(&out.MarketID, &out.Country, &out.Code,
		&out.Version, &out.Provider, &out.Environment, &out.ConnectionID, &out.BindingVersion,
		&out.Currency, &out.NameHans, &out.NameHant, &out.NameEN, &out.Enabled, &out.Visible,
		&out.SortOrder, &out.MinAmountMinor, &out.MaxAmountMinor)
	return out, err
}

func lockCurrent(ctx context.Context, tx pgx.Tx, scope platform.Scope, marketID, country, code string, update bool) (Method, bool, error) {
	lock := "FOR SHARE"
	if update {
		lock = "FOR UPDATE"
	}
	var version int64
	err := tx.QueryRow(ctx, `SELECT current_version FROM payments.method_heads
		WHERE tenant_id=$1 AND store_id=$2 AND market_id=$3 AND country=$4 AND code=$5 `+lock,
		scope.TenantID, scope.StoreID, marketID, country, code).Scan(&version)
	if errors.Is(err, pgx.ErrNoRows) {
		return Method{}, false, nil
	}
	if err != nil {
		return Method{}, false, mapError(err)
	}
	out, err := readCurrent(ctx, tx, scope, marketID, country, code, version)
	return out, err == nil, mapError(err)
}

type accountMetadata struct {
	Provider          string
	Environment       string
	BindingID         string
	CredentialVersion int64
}

func lockAccount(ctx context.Context, tx pgx.Tx, scope platform.Scope, connectionID string) (accountMetadata, error) {
	var out accountMetadata
	err := tx.QueryRow(ctx, `SELECT provider,environment,binding_id::text,credential_version
		FROM integration.merchant_accounts WHERE tenant_id=$1 AND store_id=$2 AND id=$3 FOR SHARE`,
		scope.TenantID, scope.StoreID, connectionID).Scan(&out.Provider, &out.Environment,
		&out.BindingID, &out.CredentialVersion)
	return out, mapError(err)
}

func lockBinding(ctx context.Context, tx pgx.Tx, scope platform.Scope, bindingID string) (int64, bool, error) {
	var version int64
	var enabled bool
	err := tx.QueryRow(ctx, `SELECT semantic_version,enabled FROM integration.bindings
		WHERE tenant_id=$1 AND store_id=$2 AND id=$3 FOR SHARE`, scope.TenantID,
		scope.StoreID, bindingID).Scan(&version, &enabled)
	return version, enabled, mapError(err)
}

func authorize(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, permission string) error {
	if tx == nil || !command.ValidID(scope.TenantID) || !command.ValidID(scope.StoreID) ||
		!command.ValidID(scope.PrincipalID) || scope.Revision < 1 {
		return command.ErrInvalid
	}
	var tenantID, storeID, principalID string
	err := tx.QueryRow(ctx, `SELECT coalesce(current_setting('app.tenant_id',true),''),
		coalesce(current_setting('app.store_id',true),''),coalesce(current_setting('app.principal_id',true),'')`).
		Scan(&tenantID, &storeID, &principalID)
	if err != nil {
		return err
	}
	if tenantID != scope.TenantID || storeID != scope.StoreID || principalID != scope.PrincipalID {
		return command.ErrInvalid
	}
	return platform.RequirePermission(ctx, tx, scope, token, permission)
}

func mapError(err error) error {
	if err == nil || errors.Is(err, command.ErrInvalid) || errors.Is(err, command.ErrConflict) ||
		errors.Is(err, command.ErrNotFound) {
		return err
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return command.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505", "40001":
			return command.ErrConflict
		case "23503", "P0002":
			return command.ErrNotFound
		case "22001", "22007", "22008", "22023", "22P02", "23514":
			return command.ErrInvalid
		}
	}
	return err
}
