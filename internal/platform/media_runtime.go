package platform

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var errMediaDatabase = errors.New("media database unavailable")

// OpenMediaWorkerPool borrows only the native river_media lifecycle authority.
func OpenMediaWorkerPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	if ctx == nil || len(dsn) > 8192 {
		return nil, errMediaDatabase
	}
	pool, err := openPool(ctx, dsn, "media_worker")
	if err != nil {
		return nil, errMediaDatabase
	}
	return pool, nil
}

// OpenMediaExecutorPool borrows only five fixed lease-fenced business functions.
func OpenMediaExecutorPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	if ctx == nil || len(dsn) > 8192 {
		return nil, errMediaDatabase
	}
	pool, err := openPool(ctx, dsn, "media_executor")
	if err != nil {
		return nil, errMediaDatabase
	}
	return pool, nil
}

func ValidateMediaWorkerPool(ctx context.Context, pool *pgxpool.Pool) error {
	return validateMediaPool(ctx, pool, "media_worker")
}

func ValidateMediaExecutorPool(ctx context.Context, pool *pgxpool.Pool) error {
	return validateMediaPool(ctx, pool, "media_executor")
}

func validateMediaPool(ctx context.Context, pool *pgxpool.Pool, role string) error {
	if ctx == nil || pool == nil {
		return errMediaDatabase
	}
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if validatePoolAuthority(bounded, pool, role) != nil {
		return errMediaDatabase
	}
	return nil
}

// Direct grants and reachable non-owner roles are checked as well as membership.
func validateMediaAuthority(ctx context.Context, pool *pgxpool.Pool, role string) error {
	var forbidden bool
	err := pool.QueryRow(ctx, `WITH reachable AS (
	 SELECT oid FROM pg_catalog.pg_roles WHERE rolname=session_user
	 OR pg_catalog.pg_has_role(session_user,oid,'USAGE')
	 OR pg_catalog.pg_has_role(session_user,oid,'SET')
	), fixed AS (
	 SELECT p.oid,p.proname FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_namespace n ON n.oid=p.pronamespace
	 WHERE n.nspname='live' AND p.proname IN ('claim_media_operation','load_media_material',
	 'reserve_media_start','record_media_observation','finish_media_uncertain',
	 'register_prepared_media','revoke_prepared_media')
	)
	SELECT EXISTS (SELECT 1 FROM reachable r CROSS JOIN pg_catalog.pg_class c
	 JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
	 WHERE n.nspname<>'information_schema' AND n.nspname NOT LIKE 'pg\_%' ESCAPE '\'
	 AND CASE WHEN $1='media_worker' AND n.nspname='river_media' THEN
	    c.relname='river_migration' AND
	     (pg_catalog.has_table_privilege(r.oid,c.oid,'INSERT,UPDATE,DELETE,TRUNCATE,REFERENCES,TRIGGER,MAINTAIN')
	      OR pg_catalog.has_any_column_privilege(r.oid,c.oid,'INSERT,UPDATE,REFERENCES'))
	   WHEN c.relkind='S' THEN pg_catalog.has_sequence_privilege(r.oid,c.oid,'USAGE,SELECT,UPDATE')
	   WHEN c.relkind IN ('r','p','v','m','f') THEN
	    pg_catalog.has_table_privilege(r.oid,c.oid,'SELECT,INSERT,UPDATE,DELETE,TRUNCATE,REFERENCES,TRIGGER,MAINTAIN')
	    OR pg_catalog.has_any_column_privilege(r.oid,c.oid,'SELECT,INSERT,UPDATE,REFERENCES')
	   ELSE false END
	) OR EXISTS (SELECT 1 FROM reachable r CROSS JOIN pg_catalog.pg_namespace n
	 WHERE n.nspname<>'information_schema' AND n.nspname NOT LIKE 'pg\_%' ESCAPE '\'
	 AND pg_catalog.has_schema_privilege(r.oid,n.oid,'CREATE'))
	 OR EXISTS (SELECT 1 FROM reachable r WHERE pg_catalog.has_database_privilege(r.oid,current_database(),'CREATE'))
	 OR EXISTS (SELECT 1 FROM reachable r CROSS JOIN fixed f
	 WHERE pg_catalog.has_function_privilege(r.oid,f.oid,'EXECUTE')
	 AND ($1='media_worker' OR f.proname IN ('register_prepared_media','revoke_prepared_media')))
	 OR ($1='media_executor' AND (SELECT count(DISTINCT f.proname) FROM fixed f
	 WHERE f.proname IN ('claim_media_operation','load_media_material','reserve_media_start',
	 'record_media_observation','finish_media_uncertain')
	 AND pg_catalog.has_function_privilege(session_user,f.oid,'EXECUTE'))<>5)`, role).Scan(&forbidden)
	if err != nil || forbidden {
		return errMediaDatabase
	}
	return nil
}
