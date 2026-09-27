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

// OpenMediaExecutorPool borrows only the fixed lease-fenced business functions.
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
	 'reserve_media_start','record_media_observation','record_media_cleanup_query',
	 'finish_media_uncertain','claim_media_input_operation','load_media_input_custody',
	 'close_media_input_admission','request_media_stop',
	 'register_prepared_media','revoke_prepared_media')
	), allowed AS (
	 SELECT p.oid FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_namespace n ON n.oid=p.pronamespace
	 WHERE n.nspname='live' AND (
	  (p.proname IN ('claim_media_operation','claim_media_input_operation') AND p.pronargs=4
	   AND p.proargtypes[0]='uuid'::regtype AND p.proargtypes[1]='bigint'::regtype
	   AND p.proargtypes[2]='integer'::regtype AND p.proargtypes[3]='bytea'::regtype)
	  OR (p.proname IN ('load_media_material','reserve_media_start','load_media_input_custody') AND p.pronargs=3
	   AND p.proargtypes[0]='uuid'::regtype AND p.proargtypes[1]='bigint'::regtype
	   AND p.proargtypes[2]='bytea'::regtype)
	  OR (p.proname='close_media_input_admission' AND p.pronargs=4
	   AND p.proargtypes[0]='uuid'::regtype AND p.proargtypes[1]='bigint'::regtype
	   AND p.proargtypes[2]='bytea'::regtype AND p.proargtypes[3]='text'::regtype)
	  OR (p.proname='record_media_observation' AND p.pronargs=10
	   AND p.proargtypes[0]='uuid'::regtype AND p.proargtypes[1]='bigint'::regtype
	   AND p.proargtypes[2]='bytea'::regtype
	   AND p.proargtypes[3]='text'::regtype AND p.proargtypes[4]='text'::regtype
	   AND p.proargtypes[5]='text'::regtype AND p.proargtypes[6]='text'::regtype
	   AND p.proargtypes[7]='bigint'::regtype AND p.proargtypes[8]='bigint'::regtype
	   AND p.proargtypes[9]='bigint'::regtype)
	  OR (p.proname='record_media_cleanup_query' AND p.pronargs=9
	   AND p.proargtypes[0]='uuid'::regtype AND p.proargtypes[1]='bigint'::regtype
	   AND p.proargtypes[2]='bytea'::regtype
	   AND p.proargtypes[3]='text'::regtype AND p.proargtypes[4]='text'::regtype
	   AND p.proargtypes[5]='text'::regtype AND p.proargtypes[6]='bigint'::regtype
	   AND p.proargtypes[7]='bigint'::regtype AND p.proargtypes[8]='bigint'::regtype)
	  OR (p.proname='finish_media_uncertain' AND p.pronargs=4
	   AND p.proargtypes[0]='uuid'::regtype AND p.proargtypes[1]='bigint'::regtype
	   AND p.proargtypes[2]='bytea'::regtype AND p.proargtypes[3]='text'::regtype)
	  OR (p.proname IN ('media_worker_ready','media_input_plan_ready') AND p.pronargs=0))
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
	 AND ($1='media_worker' OR f.proname IN
	  ('register_prepared_media','revoke_prepared_media','request_media_stop')))
	 OR ($1='media_executor' AND (SELECT count(DISTINCT f.proname) FROM fixed f
	 WHERE f.proname IN ('claim_media_operation','load_media_material','reserve_media_start',
	 'record_media_observation','record_media_cleanup_query','finish_media_uncertain',
	 'claim_media_input_operation','load_media_input_custody','close_media_input_admission')
	 AND pg_catalog.has_function_privilege(session_user,f.oid,'EXECUTE'))<>9)
	 OR EXISTS (SELECT 1 FROM reachable r CROSS JOIN pg_catalog.pg_proc p
	 JOIN pg_catalog.pg_namespace n ON n.oid=p.pronamespace
	 WHERE n.nspname<>'information_schema' AND n.nspname NOT LIKE 'pg\_%' ESCAPE '\'
	  AND pg_catalog.has_schema_privilege(r.oid,n.oid,'USAGE')
	  AND p.prorettype<>'trigger'::regtype
	  AND NOT ($1='media_worker' AND (n.nspname='river_media'
	   OR (n.nspname='live' AND p.proname IN ('media_worker_ready','media_input_plan_ready')
	    AND p.pronargs=0)))
	  AND NOT ($1='media_executor' AND p.oid IN (SELECT oid FROM allowed))
	  AND pg_catalog.has_function_privilege(r.oid,p.oid,'EXECUTE'))`, role).Scan(&forbidden)
	if err != nil || forbidden {
		return errMediaDatabase
	}
	return nil
}
