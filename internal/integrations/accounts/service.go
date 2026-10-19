// Package accounts owns the custody of merchant-owned provider credentials: sealed key material,
// historical account metadata and the shared Stripe keyring. Configuration never implies the provider
// has approved or verified an account. It never logs or returns a plaintext key, never calls a provider
// (verification lives in the registrars and workers), and never derives merchant scope from a request.
package accounts

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"livecommerce/internal/command"
	"livecommerce/internal/integrations/core"
	"livecommerce/internal/platform"
)

const configuredUnverified = "CONFIGURED_UNVERIFIED"

var accountIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

type Service struct {
	keys     *Keyring
	bindings *core.Service
}

type CreateInput struct {
	Provider    string
	Environment string
	AccountID   string
	Credentials Credentials
}

type RotateInput struct {
	ConnectionID    string
	ExpectedVersion int64
	Credentials     Credentials
}

type Connection struct {
	ID                string    `json:"id"`
	Provider          string    `json:"provider"`
	Environment       string    `json:"environment"`
	AccountID         string    `json:"account_id"`
	BindingID         string    `json:"binding_id"`
	BindingVersion    int64     `json:"binding_version"`
	Enabled           bool      `json:"enabled"`
	CredentialVersion int64     `json:"credential_version"`
	KeyID             string    `json:"key_id"`
	State             string    `json:"state"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func New(keys *Keyring, bindings *core.Service) (*Service, error) {
	if keys == nil || len(keys.keys) == 0 || len(keys.replayKey) != 32 || bindings == nil {
		return nil, command.ErrInvalid
	}
	return &Service{keys: keys, bindings: bindings}, nil
}

// Create and Rotate are retired credential-write APIs. Historical account metadata remains readable via Get/List.
func (s *Service) Create(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key string, in CreateInput) (Connection, error) {
	return Connection{}, command.ErrInvalid
}

func (s *Service) Rotate(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key string, in RotateInput) (Connection, error) {
	return Connection{}, command.ErrInvalid
}

func (s *Service) Get(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, connectionID string) (out Connection, err error) {
	if s == nil || s.keys == nil || s.bindings == nil || !command.ValidID(connectionID) {
		return out, command.ErrInvalid
	}
	if err = authorize(ctx, tx, scope, token, "integration:read"); err != nil {
		return out, err
	}
	err = tx.QueryRow(ctx, `SELECT a.id::text,a.provider,a.environment,a.account_id,a.binding_id::text,
		b.semantic_version,b.enabled,a.credential_version,c.key_id,a.created_at,a.updated_at
		FROM integration.merchant_accounts a
		JOIN integration.bindings b ON b.tenant_id=a.tenant_id AND b.store_id=a.store_id AND b.id=a.binding_id
		JOIN integration.account_credentials c ON c.tenant_id=a.tenant_id AND c.store_id=a.store_id
		 AND c.connection_id=a.id AND c.version=a.credential_version
		WHERE a.tenant_id=$1 AND a.store_id=$2 AND a.id=$3
		FOR SHARE OF a,b`, scope.TenantID, scope.StoreID, connectionID).Scan(&out.ID, &out.Provider,
		&out.Environment, &out.AccountID, &out.BindingID, &out.BindingVersion, &out.Enabled,
		&out.CredentialVersion, &out.KeyID, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return Connection{}, mapError(err)
	}
	if err = authorize(ctx, tx, scope, token, "integration:read"); err != nil {
		return Connection{}, err
	}
	out.State = configuredUnverified
	return out, nil
}

func validEnvironment(value string) bool { return value == "SANDBOX" || value == "LIVE" }
func validAccountID(value string) bool   { return accountIDPattern.MatchString(value) }

// Only the keyed digest reaches command.Run. The independent replay key stays
// stable as encryption keys rotate, preserving the permanent idempotency key.
func (s *Service) fingerprint(scope platform.Scope, c Credentials) string {
	plain, _ := json.Marshal(struct {
		Domain   string `json:"domain"`
		TenantID string `json:"tenant_id"`
		StoreID  string `json:"store_id"`
		HashKey  string `json:"hash_key"`
		HashIV   string `json:"hash_iv"`
	}{"merchant-account-credential-replay-v1", scope.TenantID, scope.StoreID, c.HashKey, c.HashIV})
	mac := hmac.New(sha256.New, s.keys.replayKey)
	_, _ = mac.Write(plain)
	return fmt.Sprintf("%x", mac.Sum(nil))
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
		case "40001", "23505":
			return command.ErrConflict
		case "23503", "P0002":
			return command.ErrNotFound
		case "22001", "22007", "22008", "22023", "22P02", "23514":
			return command.ErrInvalid
		}
	}
	return err
}
