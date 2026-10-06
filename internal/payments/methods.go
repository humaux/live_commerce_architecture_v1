// Purpose: merchant payment-method configuration (immutable revisions, diagnostics) and, since w4-02b, the qualification-gated
//   enable of payuni_credit; never payment attempts, credentials or provider calls.
// Depends on: internal/command, internal/platform, internal/pricing; SQL payments.method_versions/method_heads (0015/0016),
//   payments.enable_payuni_method and payments.method_qualification_state (0137, SECURITY DEFINER); PlatformEnabled gate (LC_PAYUNI_ENABLED).
// Used by: internal/httpapi/settings.go (PUT/GET/inspect payment-methods), tests/foundation payment_methods/payuni_activation tests.
// Invariants: enabled=true only through payments.enable_payuni_method with a current, unexpired, unrevoked qualification and the
//   platform switch on (contracts/payment-methods-v1.md Amendment W4-02B); Reasons order is fixed.
// Status: MOCK/SANDBOX evidence only.

// methods.go owns merchant payment-method configuration, not payment attempts.

package payments

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"livecommerce/internal/command"
	"livecommerce/internal/platform"
	"livecommerce/internal/pricing"
)

const provider = "payuni"

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
	// QualificationID is set only on an enabled revision: the qualification payments.enable_payuni_method found and pinned.
	QualificationID string `json:"qualification_id,omitempty"`
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
	if in.Enabled && (in.Code != "payuni_credit" || in.ConnectionID == "") {
		return out, command.ErrConflict // only payuni_credit with a connection can ever be admitted (v1)
	}
	if err = authorize(ctx, tx, scope, token, "integration:manage"); err != nil {
		return out, err
	}
	if in.Enabled && !PayuniEnabled() {
		return out, ErrPlatformDisabled // LC_PAYUNI_ENABLED=0: refused before any write, even with a valid qualification
	}
	request := struct {
		PrincipalID string `json:"principal_id"`
		MethodInput
	}{scope.PrincipalID, in}
	err = command.Run(ctx, tx, scope, "payment.method.set", key, request, &out, func() error {
		market, lockErr := pricing.LockMarket(ctx, tx, scope.TenantID, scope.StoreID, in.MarketID)
		if lockErr != nil {
			return lockErr
		}
		if market.Currency != "TWD" {
			return command.ErrConflict
		}
		if lockErr = advisoryLock(ctx, tx, methodLockKey(scope, in.MarketID, in.Country, in.Code)); lockErr != nil {
			return lockErr
		}
		current, exists, lockErr := lockCurrent(ctx, tx, scope, in.MarketID, in.Country, in.Code, true)
		if lockErr != nil {
			return lockErr
		}
		if exists != (in.ExpectedVersion > 0) || (exists && current.Version != in.ExpectedVersion) {
			return command.ErrConflict
		}
		if in.ConnectionID != "" {
			account, accountErr := lockAccount(ctx, tx, scope, in.ConnectionID)
			if accountErr != nil {
				return accountErr
			}
			if account.Provider != provider || account.Environment != in.Environment {
				return command.ErrConflict
			}
			bindingVersion, _, bindingErr := lockBinding(ctx, tx, scope, account.BindingID)
			if bindingErr != nil {
				return bindingErr
			}
			unchanged := exists && current.ConnectionID == in.ConnectionID &&
				current.Environment == in.Environment && current.BindingVersion == in.BindingVersion
			if !unchanged && bindingVersion != in.BindingVersion {
				return command.ErrConflict
			}
		}
		version := int64(1)
		if exists {
			version = current.Version + 1
		}
		out = Method{MarketID: in.MarketID, Country: in.Country, Code: in.Code,
			Version: version, Provider: provider, Environment: in.Environment,
			ConnectionID: in.ConnectionID, BindingVersion: in.BindingVersion, Currency: market.Currency,
			NameHans: in.NameHans, NameHant: in.NameHant, NameEN: in.NameEN,
			Enabled: in.Enabled, Visible: in.Visible, SortOrder: in.SortOrder,
			MinAmountMinor: in.MinAmountMinor, MaxAmountMinor: in.MaxAmountMinor}
		var insertErr error
		if out.Enabled {
			// payments.enable_payuni_method (0137, payments-methods Amendment W4-02B): the only writer of an enabled revision.
			// It finds the current qualification itself (PT412 -> ErrNotQualified); merchants hold no INSERT right for enabled=true.
			insertErr = tx.QueryRow(ctx, `SELECT payments.enable_payuni_method($1::uuid,$2,$3,$4,$5,$6::uuid,$7,$8,$9,$10,$11,$12,$13,$14)::text`,
				out.MarketID, out.Country, out.Code, out.Version, out.Environment, out.ConnectionID, out.BindingVersion,
				out.NameHans, out.NameHant, out.NameEN, out.Visible, out.SortOrder, out.MinAmountMinor, out.MaxAmountMinor).
				Scan(&out.QualificationID)
		} else {
			_, insertErr = tx.Exec(ctx, `INSERT INTO payments.method_versions(
			tenant_id,store_id,market_id,country,code,version,provider,environment,
			connection_id,binding_version,currency,name_hans,name_hant,name_en,
			enabled,visible,sort_order,min_amount_minor,max_amount_minor,principal_id)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,nullif($9::text,'')::uuid,
			nullif($10::bigint,0),$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)`,
				scope.TenantID, scope.StoreID, out.MarketID, out.Country, out.Code, out.Version,
				out.Provider, out.Environment, out.ConnectionID, out.BindingVersion, out.Currency,
				out.NameHans, out.NameHant, out.NameEN, out.Enabled, out.Visible, out.SortOrder,
				out.MinAmountMinor, out.MaxAmountMinor, scope.PrincipalID)
		}
		if insertErr != nil {
			return mapError(insertErr)
		}
		if exists {
			tag, updateErr := tx.Exec(ctx, `UPDATE payments.method_heads SET current_version=$6
				WHERE tenant_id=$1 AND store_id=$2 AND market_id=$3 AND country=$4 AND code=$5
				AND current_version=$7`, scope.TenantID, scope.StoreID, in.MarketID, in.Country,
				in.Code, out.Version, current.Version)
			if updateErr != nil {
				return mapError(updateErr)
			}
			if tag.RowsAffected() != 1 {
				return command.ErrConflict
			}
		} else {
			_, insertErr = tx.Exec(ctx, `INSERT INTO payments.method_heads(
				tenant_id,store_id,market_id,country,code,current_version)
				VALUES($1,$2,$3,$4,$5,$6)`, scope.TenantID, scope.StoreID, in.MarketID,
				in.Country, in.Code, out.Version)
			if insertErr != nil {
				return mapError(insertErr)
			}
		}
		return command.Audit(ctx, tx, scope, "payment.method.set")
	})
	if err != nil {
		return Method{}, mapError(err)
	}
	if err = authorize(ctx, tx, scope, token, "integration:manage"); err != nil {
		return Method{}, err
	}
	return out, nil
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
	var qual *qualState
	if method.QualificationID != "" {
		qual, err = readQualification(ctx, tx, method.QualificationID)
		if err != nil {
			return Availability{}, err
		}
	}
	out = diagnose(method, market.Active, in, out.BindingVersion, out.CredentialVersion, bindingEnabled, qual, time.Now(), PayuniEnabled())
	if err = authorize(ctx, tx, scope, token, "integration:read"); err != nil {
		return Availability{}, err
	}
	return out, nil
}

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

// qualState is the own-store fact set of one qualification (payments.method_qualification_state).
type qualState struct {
	ConnectionID      string
	CredentialVersion int64
	Environment       string
	Code              string
	ProofClass        string
	ExpiresAt         time.Time
	Revoked           bool
}

// readQualification reads one qualification through payments.method_qualification_state (0137): commerce_runtime has no table grant.
func readQualification(ctx context.Context, tx pgx.Tx, id string) (*qualState, error) {
	var q qualState
	var revoked *time.Time
	var observed time.Time
	err := tx.QueryRow(ctx, `SELECT connection_id::text,credential_version,environment,code,proof_class,observed_at,expires_at,revoked_at
		FROM payments.method_qualification_state($1::uuid)`, id).Scan(&q.ConnectionID, &q.CredentialVersion, &q.Environment,
		&q.Code, &q.ProofClass, &observed, &q.ExpiresAt, &revoked)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil // referenced row not visible: treated as NOT_QUALIFIED
	}
	if err != nil {
		return nil, mapError(err)
	}
	q.Revoked = revoked != nil
	if observed.After(time.Now().Add(time.Minute)) {
		q.Revoked = true // an observation from the future is never a valid proof
	}
	return &q, nil
}

func diagnose(method Method, marketActive bool, in CheckInput, bindingVersion, credentialVersion int64, bindingEnabled bool,
	q *qualState, now time.Time, platformEnabled bool) Availability {
	out := Availability{MethodVersion: method.Version, BindingVersion: bindingVersion,
		CredentialVersion: credentialVersion, Reasons: make([]string, 0, 14)}
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
		// A proof class admits an environment: LIVE needs REAL_LIVE; SANDBOX takes REAL_SANDBOX or PROVIDER_MOCK (the mock proof
		// is fenced by the payment profile at payment start, not here).
		proofOK := q != nil && q.ConnectionID == method.ConnectionID && q.Environment == method.Environment && q.Code == method.Code &&
			!q.Revoked && ((method.Environment == "LIVE" && q.ProofClass == "REAL_LIVE") ||
			(method.Environment == "SANDBOX" && (q.ProofClass == "REAL_SANDBOX" || q.ProofClass == "PROVIDER_MOCK")))
		rotated := q != nil && q.CredentialVersion != credentialVersion
		expired := q != nil && !now.Before(q.ExpiresAt)
		add(!proofOK || rotated || expired, "CREDENTIALS_UNVERIFIED")
		add(!proofOK, "NOT_QUALIFIED")
		add(proofOK && rotated, "CREDENTIAL_ROTATED")
		add(proofOK && expired, "QUALIFICATION_EXPIRED")
	}
	add(!platformEnabled, "PLATFORM_DISABLED")
	// ponytail: only payuni_credit has an adapter in v1; the other four codes stay disabled drafts (payment-methods-v1 amendment).
	add(method.Code != "payuni_credit", "ADAPTER_UNAVAILABLE")
	out.Available = len(out.Reasons) == 0
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
		name_hans,name_hant,name_en,enabled,visible,sort_order,min_amount_minor,max_amount_minor,
		coalesce(qualification_id::text,'')
		FROM payments.method_versions WHERE tenant_id=$1 AND store_id=$2 AND market_id=$3
		AND country=$4 AND code=$5 AND version=$6`, scope.TenantID, scope.StoreID,
		marketID, country, code, version).Scan(&out.MarketID, &out.Country, &out.Code,
		&out.Version, &out.Provider, &out.Environment, &out.ConnectionID, &out.BindingVersion,
		&out.Currency, &out.NameHans, &out.NameHant, &out.NameEN, &out.Enabled, &out.Visible,
		&out.SortOrder, &out.MinAmountMinor, &out.MaxAmountMinor, &out.QualificationID)
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

func methodLockKey(scope platform.Scope, marketID, country, code string) string {
	return "payment.method|" + scope.TenantID + "|" + scope.StoreID + "|" + marketID + "|" + country + "|" + code
}

func advisoryLock(ctx context.Context, tx pgx.Tx, key string) error {
	var timeout string
	if err := tx.QueryRow(ctx, `SHOW lock_timeout`).Scan(&timeout); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('lock_timeout','0',true)`); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, key); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `SELECT set_config('lock_timeout',$1,true)`, timeout)
	return err
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
		errors.Is(err, command.ErrNotFound) ||
		errors.Is(err, ErrNotQualified) || errors.Is(err, ErrPlatformDisabled) {
		return err
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return command.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "PT412":
			return ErrNotQualified
		case "PT409", "23505", "40001":
			return command.ErrConflict
		case "23503", "P0002":
			return command.ErrNotFound
		case "22001", "22007", "22008", "22023", "22P02", "23514":
			return command.ErrInvalid
		}
	}
	return err
}
