package platform

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// OpenMetaIngressPool grants callback admission only, never merchant or worker
// authority. Keep parse/connection errors (which may include a DSN) private.
func OpenMetaIngressPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	if ctx == nil || len(dsn) > 8192 {
		return nil, errors.New("meta ingress database unavailable")
	}
	pool, err := openPool(ctx, dsn, "meta_ingress")
	if err != nil {
		return nil, errors.New("meta ingress database unavailable")
	}
	return pool, nil
}

// OpenMetaConsumerPool opens the projection-only authority. River lifecycle
// operations must use the separate Meta lifecycle pool, not a worker authority.
func OpenMetaConsumerPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	if ctx == nil || len(dsn) > 8192 {
		return nil, errors.New("meta consumer database unavailable")
	}
	pool, err := openPool(ctx, dsn, "meta_consumer")
	if err != nil {
		return nil, errors.New("meta consumer database unavailable")
	}
	return pool, nil
}

// OpenMetaWorkerPool confines River maintenance authority to river_meta. Queue
// selection alone does not isolate River's scheduler/rescuer/cleaner.
func OpenMetaWorkerPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	if ctx == nil || len(dsn) > 8192 {
		return nil, errors.New("meta worker database unavailable")
	}
	pool, err := openPool(ctx, dsn, "meta_worker")
	if err != nil {
		return nil, errors.New("meta worker database unavailable")
	}
	return pool, nil
}

// ValidateMetaWorkerPool borrows, but never closes, the caller's lifecycle
// pool. The ordinary worker and projection authorities are both inadmissible.
func ValidateMetaWorkerPool(ctx context.Context, pool *pgxpool.Pool) error {
	if ctx == nil || pool == nil {
		return errors.New("meta worker database unavailable")
	}
	bounded, cancel := context.WithTimeout(ctx, startupTimeout)
	defer cancel()
	if err := validatePoolAuthority(bounded, pool, "meta_worker"); err != nil {
		return errors.New("meta worker database unavailable")
	}
	return nil
}

// ValidateSameDatabase rejects a split ingress or consumer configuration before
// listening/fetching. Names and restored UUIDs cannot distinguish cloned DBs;
// PostgreSQL advisory locks are database-local. Borrowed pools stay caller-owned.
// This proves current trusted routing, not future pool retargeting or a hostile
// server. No permanent row or wider database grant is needed.
func ValidateSameDatabase(ctx context.Context, first, second *pgxpool.Pool) (err error) {
	denied := errors.New("database identity unavailable")
	if ctx == nil || first == nil || second == nil {
		return denied
	}
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var key int64
	for key == 0 {
		var entropy [8]byte
		if _, e := rand.Read(entropy[:]); e != nil {
			return denied
		}
		key = int64(binary.BigEndian.Uint64(entropy[:]))
	}
	// Rollback must work after caller cancellation, release the borrowed pool
	// connection, and never silently turn a cleanup failure into startup success.
	cleanup := func(tx pgx.Tx) {
		rollbackCtx, stop := context.WithTimeout(context.Background(), 2*time.Second)
		defer stop()
		if e := tx.Rollback(rollbackCtx); e != nil {
			err = denied
		}
	}
	a, e := first.Begin(bounded)
	if e != nil {
		return denied
	}
	defer cleanup(a)
	var acquired bool
	if e = a.QueryRow(bounded, "SELECT pg_catalog.pg_try_advisory_xact_lock($1::bigint)", key).Scan(&acquired); e != nil || !acquired {
		return denied
	}
	b, e := second.Begin(bounded)
	if e != nil {
		return denied
	}
	defer cleanup(b)
	if e = b.QueryRow(bounded, "SELECT pg_catalog.pg_try_advisory_xact_lock($1::bigint)", key).Scan(&acquired); e != nil || acquired {
		return denied
	}
	return nil
}
