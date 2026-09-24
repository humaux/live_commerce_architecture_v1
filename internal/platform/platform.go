// Package platform provides the narrow HTTP and database foundation shared by
// the API process. Domain packages receive a scoped transaction, never a pool.
package platform

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"livecommerce/internal/httperror"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrUnauthorized = errors.New("unauthorized")
var ErrForbidden = errors.New("forbidden")
var ErrScopeNotFound = errors.New("scope not found")

const (
	storeReadPermission = "store:read"
	auditReadPermission = "audit:read"
	requestTimeout      = 5 * time.Second
	startupTimeout      = 2 * time.Second
	lockTimeout         = time.Second
)

// Scope is resolved by identity.resolve_access and is not derived from HTTP input.
type Scope struct {
	TenantID    string
	StoreID     string
	PrincipalID string
	Revision    int64
}

// OpenPool opens the runtime pool and rejects privileged or schema-owning logins.
// A login granted commerce_runtime is valid when it is itself not privileged.
func OpenPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	return openPool(ctx, dsn, "runtime")
}

// OpenIdentityPool is only for the trusted login/onboarding service. Never pass
// this pool into business handlers: identity issuance is a different authority.
func OpenIdentityPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	return openPool(ctx, dsn, "identity")
}

// OpenBuyerPool cannot read merchant tables, even when a buyer has the same
// tenant/store context. Buyer resource grants are separate from merchant RBAC.
func OpenBuyerPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	return openPool(ctx, dsn, "buyer_runtime")
}

// OpenCheckoutPool admits only the dedicated internal checkout login. It is
// never a public buyer SQL credential or a merchant authority.
func OpenCheckoutPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	return openPool(ctx, dsn, "checkout_runtime")
}

// OpenHostedPool is the trusted payment signing authority. Its LOGIN inherits
// commerce_hosted_runtime (GRANT ... WITH INHERIT TRUE, SET FALSE), which in
// turn inherits checkout runtime grants. Admission checks both options.
func OpenHostedPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	return openPool(ctx, dsn, "hosted_runtime")
}

// OpenBuyerIssuerPool is an internal capability authority, never a public
// store-ID-to-token endpoint. See contracts/buyer-capability-v1.md.
func OpenBuyerIssuerPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	return openPool(ctx, dsn, "buyer_issuer")
}

// OpenWorkerPool is a separate non-HTTP authority. River's lifecycle grants
// must never be inherited by a merchant, buyer, or identity service login.
func OpenWorkerPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	return openPool(ctx, dsn, "worker")
}

func openPool(ctx context.Context, dsn string, authority string) (*pgxpool.Pool, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, errors.New("database url required")
	}
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse database config: %w", err)
	}
	config.MaxConns = 8

	startup, cancel := context.WithTimeout(ctx, startupTimeout)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(startup, config)
	if err != nil {
		return nil, fmt.Errorf("open database pool: %w", err)
	}
	if err := pool.Ping(startup); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database unavailable: %w", err)
	}
	if err := validatePoolAuthority(startup, pool, authority); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

// ValidateWorkerPool reuses the startup authority gate when an internal worker
// receives an existing pool. The caller retains ownership of that pool; failure
// never closes it. A nil pool and an owner/mixed-role connection fail closed.
func ValidateWorkerPool(ctx context.Context, pool *pgxpool.Pool) error {
	if pool == nil {
		return errors.New("worker database pool required")
	}
	bounded, cancel := context.WithTimeout(ctx, startupTimeout)
	defer cancel()
	return validatePoolAuthority(bounded, pool, "worker")
}

// ValidateCheckoutPool checks an existing pool without taking ownership of it.
func ValidateCheckoutPool(ctx context.Context, pool *pgxpool.Pool) error {
	if pool == nil {
		return errors.New("checkout database pool required")
	}
	bounded, cancel := context.WithTimeout(ctx, startupTimeout)
	defer cancel()
	return validatePoolAuthority(bounded, pool, "checkout_runtime")
}

func ValidateHostedPool(ctx context.Context, pool *pgxpool.Pool) error {
	if ctx == nil || pool == nil {
		return errors.New("hosted context and database pool required")
	}
	bounded, cancel := context.WithTimeout(ctx, startupTimeout)
	defer cancel()
	return validatePoolAuthority(bounded, pool, "hosted_runtime")
}

// ValidateBuyerPool checks a borrowed buyer pool before it enters buyer.WithScope.
// It must not inherit issuer, merchant, owner, or checkout authority.
func ValidateBuyerPool(ctx context.Context, pool *pgxpool.Pool) error {
	if ctx == nil || pool == nil {
		return errors.New("buyer context and runtime pool required")
	}
	bounded, cancel := context.WithTimeout(ctx, startupTimeout)
	defer cancel()
	return validatePoolAuthority(bounded, pool, "buyer_runtime")
}

// ValidateBuyerIssuerPool checks an existing issuer pool without taking ownership.
func ValidateBuyerIssuerPool(ctx context.Context, pool *pgxpool.Pool) error {
	if ctx == nil {
		return errors.New("buyer issuer context required")
	}
	if pool == nil {
		return errors.New("buyer issuer database pool required")
	}
	bounded, cancel := context.WithTimeout(ctx, startupTimeout)
	defer cancel()
	return validatePoolAuthority(bounded, pool, "buyer_issuer")
}

func validatePoolAuthority(ctx context.Context, pool *pgxpool.Pool, authority string) error {
	var sameLogin, dsnUserMatch, superuser, bypassRLS, roleAdmin, databaseCreator, replication, objectOwner, runtimeMember, authMember, identityMember, buyerRuntimeMember, buyerIssuerMember, workerMember, checkoutMember, hostedMember, hostedUsage, hostedSet, checkoutWriterMember, canSetPrivileged bool
	err := pool.QueryRow(ctx, `
		SELECT session_user=current_user, session_user=$1, r.rolsuper, r.rolbypassrls, r.rolcreaterole, r.rolcreatedb, r.rolreplication,
		       (EXISTS (
			   SELECT 1 FROM pg_namespace n
			   WHERE n.nspowner = r.oid
			     AND n.nspname NOT IN ('pg_catalog', 'information_schema')
		       ) OR EXISTS (
			   SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
			   WHERE c.relowner=r.oid AND n.nspname NOT IN ('pg_catalog','information_schema')
		       ) OR EXISTS (
			   SELECT 1 FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
			   WHERE p.proowner=r.oid AND n.nspname NOT IN ('pg_catalog','information_schema')
		       )),
		       pg_has_role(session_user, 'commerce_runtime', 'MEMBER'),
		       pg_has_role(session_user, 'commerce_auth', 'MEMBER'),
		       pg_has_role(session_user, 'commerce_identity', 'MEMBER'),
		       pg_has_role(session_user, 'commerce_buyer_runtime', 'MEMBER'),
		       pg_has_role(session_user, 'commerce_buyer_issuer', 'MEMBER'),
		       pg_has_role(session_user, 'commerce_worker', 'MEMBER'),
		       coalesce(pg_has_role(session_user, to_regrole('commerce_checkout_runtime'), 'MEMBER'),false),
		       coalesce(pg_has_role(session_user, to_regrole('commerce_hosted_runtime'), 'MEMBER'),false),
		       coalesce(pg_has_role(session_user, to_regrole('commerce_hosted_runtime'), 'USAGE'),false),
		       coalesce(pg_has_role(session_user, to_regrole('commerce_hosted_runtime'), 'SET'),false),
		       coalesce(pg_has_role(session_user, to_regrole('commerce_checkout_writer'), 'MEMBER'),false),
		       EXISTS (
			   SELECT 1 FROM pg_roles candidate
			   WHERE (candidate.rolsuper OR candidate.rolbypassrls OR candidate.rolcreaterole OR candidate.rolcreatedb OR candidate.rolreplication
			       OR EXISTS (
				   SELECT 1 FROM pg_namespace n
				   WHERE n.nspowner = candidate.oid
				     AND n.nspname NOT IN ('pg_catalog', 'information_schema')
			       ) OR EXISTS (
				   SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
				   WHERE c.relowner=candidate.oid AND n.nspname NOT IN ('pg_catalog','information_schema')
			       ) OR EXISTS (
				   SELECT 1 FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
				   WHERE p.proowner=candidate.oid AND n.nspname NOT IN ('pg_catalog','information_schema')
			       ))
			     AND pg_has_role(session_user, candidate.oid, 'SET')
		       )
		FROM pg_roles r WHERE r.rolname = session_user`, pool.Config().ConnConfig.User).
		Scan(&sameLogin, &dsnUserMatch, &superuser, &bypassRLS, &roleAdmin, &databaseCreator, &replication, &objectOwner, &runtimeMember, &authMember, &identityMember, &buyerRuntimeMember, &buyerIssuerMember, &workerMember, &checkoutMember, &hostedMember, &hostedUsage, &hostedSet, &checkoutWriterMember, &canSetPrivileged)
	if err != nil {
		return fmt.Errorf("validate runtime role: %w", err)
	}
	// Exactly one authority, including indirect grants. Checking only the desired
	// role would let a mixed login smuggle merchant privileges into buyer code.
	memberships := map[string]bool{"runtime": runtimeMember, "identity": identityMember,
		"buyer_runtime": buyerRuntimeMember, "buyer_issuer": buyerIssuerMember, "worker": workerMember,
		"checkout_runtime": checkoutMember}
	roleCount := 0
	for _, member := range memberships {
		if member {
			roleCount++
		}
	}
	roleValid := memberships[authority] && roleCount == 1 && !hostedMember
	if authority == "hosted_runtime" {
		roleValid = checkoutMember && hostedMember && hostedUsage && !hostedSet && roleCount == 1
	}
	// A privileged login cannot launder its authority with startup SET ROLE:
	// RESET ROLE would recover the session_user's capabilities after admission.
	if !sameLogin || !dsnUserMatch || superuser || bypassRLS || roleAdmin || databaseCreator || replication || objectOwner || !roleValid || authMember || checkoutWriterMember || canSetPrivileged {
		return errors.New("unsafe runtime database role")
	}
	return nil
}

// WithScope resolves an opaque session inside a transaction, sets transaction-local
// RLS context, and runs fn. Every non-success path rolls back with an independent,
// bounded cleanup context.
func WithScope(ctx context.Context, pool *pgxpool.Pool, token, storeID, permission string, fn func(pgx.Tx, Scope) error) (err error) {
	if fn == nil {
		return ErrUnauthorized
	}
	return withScopeContext(ctx, pool, token, storeID, permission, func(_ context.Context, tx pgx.Tx, scope Scope) error {
		return fn(tx, scope)
	})
}

// RequirePermission checks a second fixed route permission inside the existing
// scoped transaction. It cannot change scope or open a separate auth snapshot.
func RequirePermission(ctx context.Context, tx pgx.Tx, scope Scope, token, permission string) error {
	if tx == nil || len(token) < 32 || len(token) > 512 {
		return ErrUnauthorized
	}
	hash := sha256.Sum256([]byte(token))
	var status, tenant, principal string
	var revision int64
	err := tx.QueryRow(ctx, `SELECT access_status,coalesce(tenant_id::text,''),coalesce(principal_id::text,''),coalesce(authz_revision,0) FROM identity.resolve_access($1,$2::uuid,$3)`, hash[:], scope.StoreID, permission).Scan(&status, &tenant, &principal, &revision)
	if err != nil {
		return err
	}
	switch status {
	case "unauthorized":
		return ErrUnauthorized
	case "not_found":
		return ErrScopeNotFound
	case "forbidden":
		return ErrForbidden
	case "ok":
		if tenant == scope.TenantID && principal == scope.PrincipalID && revision == scope.Revision {
			return nil
		}
		return ErrForbidden
	default:
		return errors.New("invalid access result")
	}
}

func withScopeContext(ctx context.Context, pool *pgxpool.Pool, token, storeID, permission string, fn func(context.Context, pgx.Tx, Scope) error) (err error) {
	if pool == nil || fn == nil || len(token) < 32 || len(token) > 512 || !isCanonicalUUID(storeID) {
		return ErrUnauthorized
	}
	scopeCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	transaction, err := pool.BeginTx(scopeCtx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			rollback(transaction)
			panic(recovered)
		}
		if err != nil {
			rollback(transaction)
		}
	}()

	hash := sha256.Sum256([]byte(token))
	var scope Scope
	_, err = transaction.Exec(scopeCtx, `SELECT
		set_config('statement_timeout', $1, true),
		set_config('lock_timeout', $2, true),
		set_config('idle_in_transaction_session_timeout', $3, true)`, requestTimeout.String(), lockTimeout.String(), requestTimeout.String())
	if err != nil {
		return err
	}
	var access string
	err = transaction.QueryRow(scopeCtx, `SELECT access_status, coalesce(tenant_id::text,''),
        coalesce(principal_id::text,''), coalesce(authz_revision,0)
		FROM identity.resolve_access($1, $2::uuid, $3)`, hash[:], storeID, permission).
		Scan(&access, &scope.TenantID, &scope.PrincipalID, &scope.Revision)
	if err != nil {
		return err
	}
	switch access {
	case "unauthorized":
		return ErrUnauthorized
	case "not_found":
		return ErrScopeNotFound
	case "forbidden":
		return ErrForbidden
	case "ok":
	default:
		return errors.New("invalid access result")
	}
	scope.StoreID = storeID
	_, err = transaction.Exec(scopeCtx, `SELECT
		set_config('app.tenant_id', $1, true),
		set_config('app.store_id', $2, true),
		set_config('app.principal_id', $3, true)`, scope.TenantID, scope.StoreID, scope.PrincipalID)
	if err != nil {
		return err
	}
	if err = fn(scopeCtx, transaction, scope); err != nil {
		return err
	}
	return transaction.Commit(scopeCtx)
}

func rollback(transaction pgx.Tx) {
	cleanup, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = transaction.Rollback(cleanup)
}

func isCanonicalUUID(value string) bool {
	if len(value) != 36 || value != strings.ToLower(value) {
		return false
	}
	for index, character := range value {
		switch index {
		case 8, 13, 18, 23:
			if character != '-' {
				return false
			}
		default:
			if !(character >= '0' && character <= '9' || character >= 'a' && character <= 'f') {
				return false
			}
		}
	}
	return true
}

// NewHandler returns the small API surface. Authorization permissions are fixed
// constants selected by the route, never supplied by a caller.
// HandlerOptions enables only explicitly wired authentication projections.
// The ordinary fixture/business handler must not expand its discovery surface.
type HandlerOptions struct{ SessionStoreList bool }

func NewHandler(pool *pgxpool.Pool, options ...HandlerOptions) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if pool == nil {
			writeError(w, http.StatusServiceUnavailable, "unavailable")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), startupTimeout)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			writeError(w, http.StatusServiceUnavailable, "unavailable")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	if len(options) > 0 && options[0].SessionStoreList {
		mux.HandleFunc("GET /v1/admin/stores", sessionStoresHandler(pool))
	}
	mux.HandleFunc("GET /v1/admin/stores/{store_id}", storeHandler(pool))
	mux.HandleFunc("GET /v1/admin/stores/{store_id}/audit-events", auditHandler(pool))
	return httperror.Middleware(mux)
}

func storeHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		storeID := r.PathValue("store_id")
		var store struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Currency string `json:"currency"`
		}
		err := withRequestScope(r, pool, storeID, storeReadPermission, func(ctx context.Context, tx pgx.Tx, _ Scope) error {
			return tx.QueryRow(ctx, `SELECT id::text, name, currency FROM control.stores WHERE id = $1::uuid`, storeID).
				Scan(&store.ID, &store.Name, &store.Currency)
		})
		switch {
		case errors.Is(err, ErrScopeNotFound):
			writeError(w, http.StatusNotFound, "not_found")
		case errors.Is(err, ErrForbidden):
			writeError(w, http.StatusForbidden, "forbidden")
		case errors.Is(err, ErrUnauthorized):
			writeError(w, http.StatusUnauthorized, "unauthorized")
		case errors.Is(err, pgx.ErrNoRows):
			writeError(w, http.StatusNotFound, "not_found")
		case err != nil:
			writeError(w, http.StatusInternalServerError, "internal")
		default:
			writeJSON(w, http.StatusOK, store)
		}
	}
}

func auditHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		storeID := r.PathValue("store_id")
		events := make([]struct {
			ID        string    `json:"id"`
			Action    string    `json:"action"`
			CreatedAt time.Time `json:"created_at"`
		}, 0)
		err := withRequestScope(r, pool, storeID, auditReadPermission, func(ctx context.Context, tx pgx.Tx, _ Scope) error {
			rows, err := tx.Query(ctx, `SELECT id::text, action, created_at
				FROM ops.audit_events WHERE store_id = $1::uuid ORDER BY created_at DESC LIMIT 50`, storeID)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var event struct {
					ID        string    `json:"id"`
					Action    string    `json:"action"`
					CreatedAt time.Time `json:"created_at"`
				}
				if err := rows.Scan(&event.ID, &event.Action, &event.CreatedAt); err != nil {
					return err
				}
				events = append(events, event)
			}
			return rows.Err()
		})
		if errors.Is(err, ErrScopeNotFound) {
			writeError(w, http.StatusNotFound, "not_found")
			return
		}
		if errors.Is(err, ErrForbidden) {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		if errors.Is(err, ErrUnauthorized) {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal")
			return
		}
		writeJSON(w, http.StatusOK, events)
	}
}

func withRequestScope(r *http.Request, pool *pgxpool.Pool, storeID, permission string, fn func(context.Context, pgx.Tx, Scope) error) error {
	token, ok := bearerToken(r)
	if !ok {
		return ErrUnauthorized
	}
	requestCtx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()
	return withScopeContext(requestCtx, pool, token, storeID, permission, fn)
}

func bearerToken(r *http.Request) (string, bool) {
	value := r.Header.Get("Authorization")
	if !strings.HasPrefix(value, "Bearer ") {
		return "", false
	}
	token := strings.TrimPrefix(value, "Bearer ")
	return token, token != ""
}

func writeError(w http.ResponseWriter, status int, code string) {
	httperror.Write(w, status, code)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
