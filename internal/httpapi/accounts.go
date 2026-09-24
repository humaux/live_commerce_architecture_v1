package httpapi

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/integrations/accounts"
	"livecommerce/internal/platform"
)

var (
	errAccountUnavailable = errors.New("account service unavailable")
	errAccountCapacity    = errors.New("account admission capacity")
	errAccountRateLimited = errors.New("account admission rate limited")
)

const accountBase = "/v1/admin/stores/{store_id}/provider-accounts"

type accountSecretWire struct {
	HashKey string `json:"hash_key"`
	HashIV  string `json:"hash_iv"`
}

func (accountSecretWire) String() string     { return "[redacted credentials]" }
func (w accountSecretWire) GoString() string { return w.String() }
func (accountSecretWire) MarshalJSON() ([]byte, error) {
	return []byte(`"[redacted credentials]"`), nil
}
func (w accountSecretWire) credentials() accounts.Credentials {
	return accounts.Credentials{HashKey: w.HashKey, HashIV: w.HashIV}
}

type accountCreateWire struct {
	Provider    string            `json:"provider"`
	Environment string            `json:"environment"`
	AccountID   string            `json:"account_id"`
	Credentials accountSecretWire `json:"credentials"`
}

func (accountCreateWire) String() string     { return "[redacted account request]" }
func (w accountCreateWire) GoString() string { return w.String() }
func (accountCreateWire) MarshalJSON() ([]byte, error) {
	return []byte(`"[redacted account request]"`), nil
}

type accountRotateWire struct {
	ExpectedVersion int64             `json:"expected_version"`
	Credentials     accountSecretWire `json:"credentials"`
}

func (accountRotateWire) String() string     { return "[redacted account request]" }
func (w accountRotateWire) GoString() string { return w.String() }
func (accountRotateWire) MarshalJSON() ([]byte, error) {
	return []byte(`"[redacted account request]"`), nil
}

func accountMetadata(value accounts.Connection) accounts.Metadata {
	return accounts.Metadata{ID: value.ID, Provider: value.Provider, Environment: value.Environment,
		AccountID: value.AccountID, BindingID: value.BindingID, BindingVersion: value.BindingVersion,
		Enabled: value.Enabled, CredentialVersion: value.CredentialVersion, State: value.State,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}

type accountScopeKey struct{ tenantID, storeID string }
type accountWindow struct {
	start time.Time
	count int
}
type accountLimiter struct {
	mu      sync.Mutex
	windows map[accountScopeKey]accountWindow
	now     func() time.Time
}

func newAccountLimiter() *accountLimiter {
	return &accountLimiter{windows: make(map[accountScopeKey]accountWindow), now: time.Now}
}

// This is a bounded process admission guard. The first accepted request for a
// scope starts its fixed minute; reads and other routes never use this guard.
func (l *accountLimiter) admit(scope platform.Scope) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	key := accountScopeKey{scope.TenantID, scope.StoreID}
	window, exists := l.windows[key]
	if exists && now.Sub(window.start) >= time.Minute {
		delete(l.windows, key)
		exists = false
	}
	if !exists {
		if len(l.windows) >= 4096 {
			for candidate, window := range l.windows {
				if now.Sub(window.start) >= time.Minute {
					delete(l.windows, candidate)
				}
			}
		}
		if len(l.windows) >= 4096 {
			return errAccountCapacity
		}
		l.windows[key] = accountWindow{start: now, count: 1}
		return nil
	}
	if window.count >= 60 {
		return errAccountRateLimited
	}
	window.count++
	l.windows[key] = window
	return nil
}

func registerAccountRoutes(mux *http.ServeMux, pool *pgxpool.Pool, service *accounts.Service) {
	limiter := newAccountLimiter()
	mux.HandleFunc("GET "+accountBase, scoped(pool, "integration:read", func(ctx context.Context, tx pgx.Tx, scope platform.Scope, r *http.Request) (any, error) {
		if service == nil {
			return nil, errAccountUnavailable
		}
		page, err := parsePage(r.URL.RawQuery)
		if err != nil {
			return nil, err
		}
		return service.List(ctx, tx, scope, bearerToken(r), page)
	}))
	mux.HandleFunc("POST "+accountBase, exactResourceRoute(bodyRoute(pool, "integration:manage", func(ctx context.Context, tx pgx.Tx, scope platform.Scope, r *http.Request, in accountCreateWire) (any, error) {
		if service == nil {
			return nil, errAccountUnavailable
		}
		if err := limiter.admit(scope); err != nil {
			return nil, err
		}
		value, err := service.Create(ctx, tx, scope, bearerToken(r), r.Header.Get("Idempotency-Key"), accounts.CreateInput{
			Provider: in.Provider, Environment: in.Environment, AccountID: in.AccountID, Credentials: in.Credentials.credentials()})
		if err != nil {
			return nil, err
		}
		return accountMetadata(value), nil
	})))
	mux.HandleFunc("GET "+accountBase+"/{connection_id}", exactResourceRoute(scoped(pool, "integration:read", func(ctx context.Context, tx pgx.Tx, scope platform.Scope, r *http.Request) (any, error) {
		if service == nil {
			return nil, errAccountUnavailable
		}
		value, err := service.Get(ctx, tx, scope, bearerToken(r), r.PathValue("connection_id"))
		if err != nil {
			return nil, err
		}
		return accountMetadata(value), nil
	})))
	mux.HandleFunc("POST "+accountBase+"/{connection_id}/rotate", exactResourceRoute(bodyRoute(pool, "integration:manage", func(ctx context.Context, tx pgx.Tx, scope platform.Scope, r *http.Request, in accountRotateWire) (any, error) {
		if service == nil {
			return nil, errAccountUnavailable
		}
		if err := limiter.admit(scope); err != nil {
			return nil, err
		}
		value, err := service.Rotate(ctx, tx, scope, bearerToken(r), r.Header.Get("Idempotency-Key"), accounts.RotateInput{
			ConnectionID: r.PathValue("connection_id"), ExpectedVersion: in.ExpectedVersion,
			Credentials: in.Credentials.credentials()})
		if err != nil {
			return nil, err
		}
		return accountMetadata(value), nil
	})))
}
