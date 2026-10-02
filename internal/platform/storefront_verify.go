package platform

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

// R5 store-domains (migration 0106): the DNS/TLS verify sweep runs as a River periodic job in cmd/claims-worker on a
// dedicated login lc_store_domain_verify (member of the NOLOGIN authority commerce_storefront_registrar, inherit_noset).
// That authority also serves the operator one-shot lc_store_registrar (cmd/store-admin), but each login is a separate
// member and the exactly-one rule in validatePoolAuthority rejects a login that mixes this authority with any other
// (or can reach the definer owner commerce_storefront_writer). Parse/connection errors, which can echo a DSN, stay private.

// OpenStoreDomainVerifyPool opens the store-domain verify job pool (cmd/claims-worker only). It admits exactly the
// commerce_storefront_registrar authority, EXECUTE on the worker transition definers of 0106, and nothing else.
func OpenStoreDomainVerifyPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	return openStoreDomainVerifyPool(ctx, dsn)
}

func openStoreDomainVerifyPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	if ctx == nil || len(dsn) > 8192 {
		return nil, errors.New("store domain verify database unavailable")
	}
	pool, err := openPool(ctx, dsn, "storefront_registrar")
	if err != nil {
		return nil, errors.New("store domain verify database unavailable")
	}
	return pool, nil
}

// ValidateStoreDomainVerifyPool borrows, but never closes, the caller's pool and applies the same admission as
// OpenStoreDomainVerifyPool.
func ValidateStoreDomainVerifyPool(ctx context.Context, pool *pgxpool.Pool) error {
	if ctx == nil || pool == nil {
		return errors.New("store domain verify database unavailable")
	}
	bounded, cancel := context.WithTimeout(ctx, startupTimeout)
	defer cancel()
	if err := validatePoolAuthority(bounded, pool, "storefront_registrar"); err != nil {
		return errors.New("store domain verify database unavailable")
	}
	return nil
}
