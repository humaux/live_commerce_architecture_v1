// Purpose: refuse API startup when buyer participants use a different physical database.
// Depends on: captured buyerConfig DSNs, platform role-validated pools and ValidateSameDatabase.
// Used by: run before buyer handler assembly and by isolated startup acceptance tests.
// Invariants: no handler or business write; temporary pools close on every return;
// the main reference pool remains owned by run (order-attempt-v1 section 1).

package main

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/platform"
)

func buyerDatabasePreflight(ctx context.Context, c buyerConfig, mainPool *pgxpool.Pool) error {
	if !c.enabled {
		return nil
	}
	if ctx == nil || mainPool == nil {
		return errBuyerConfig
	}
	// Use the same captured DSNs and login admission as the factory. Database
	// names or DSN equality cannot distinguish a restored cluster from this one.
	for _, participant := range []struct {
		dsn  string
		open func(context.Context, string) (*pgxpool.Pool, error)
	}{
		{c.issuerDSN, platform.OpenBuyerIssuerPool},
		{c.buyerDSN, platform.OpenBuyerPool},
		{c.checkoutDSN, platform.OpenCheckoutPool},
	} {
		pool, err := participant.open(ctx, participant.dsn)
		if err != nil {
			return errBuyerConfig
		}
		defer pool.Close()
		if err = platform.ValidateSameDatabase(ctx, mainPool, pool); err != nil {
			return errBuyerConfig
		}
	}
	return nil
}
