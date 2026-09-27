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

// OpenMediaRecoveryPool borrows only the seven observer functions and readiness.
func OpenMediaRecoveryPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	if ctx == nil || len(dsn) > 8192 {
		return nil, errMediaDatabase
	}
	pool, err := openPool(ctx, dsn, "media_recovery")
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

func ValidateMediaRecoveryPool(ctx context.Context, pool *pgxpool.Pool) error {
	return validateMediaPool(ctx, pool, "media_recovery")
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
	 'claim_browser_input_operation','next_media_input_turn','finish_media_input_turn',
	 'reserve_media_input_start','load_media_input_material','reserve_media_input_cleanup','record_media_input_cleanup',
	 'close_media_input_admission','request_media_stop',
     'register_prepared_media','revoke_prepared_media','begin_media_recovery_episode',
     'claim_recovery_observation','record_recovery_observation','finish_recovery_observation',
	 'read_media_recovery_episode','witness_media_recovery_episode',
	 'timeout_media_recovery_episode','media_recovery_ready',
	 'begin_media_recovery_episode_with_input','claim_media_recovery_observation_with_input',
	 'record_browser_input_recovery_observation','record_media_recovery_observation_with_input',
	 'finish_media_recovery_observation_with_input','read_media_recovery_episode_with_input',
	 'witness_media_recovery_episode_with_input','media_browser_input_recovery_ready')
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
	  OR (p.proname IN ('media_worker_ready','media_input_plan_ready','media_browser_input_worker_ready') AND p.pronargs=0))
	 UNION
	 -- BRW adds only these exact executor ABIs. Names alone must not permit an
	 -- overload, registrar authority, private helper, or table access.
	 SELECT unnest(ARRAY[
	  pg_catalog.to_regprocedure('live.claim_browser_input_operation(uuid,bigint,integer,bytea)'),
	  pg_catalog.to_regprocedure('live.next_media_input_turn(uuid,bigint,bytea)'),
	  pg_catalog.to_regprocedure('live.finish_media_input_turn(uuid,bigint,bytea,text)'),
	  pg_catalog.to_regprocedure('live.reserve_media_input_start(uuid,bigint,bytea,text,text,text,text,boolean,boolean,boolean,boolean)'),
	  pg_catalog.to_regprocedure('live.load_media_input_material(uuid,bigint,bytea)'),
	  pg_catalog.to_regprocedure('live.reserve_media_input_cleanup(uuid,bigint,bytea)'),
	  pg_catalog.to_regprocedure('live.record_media_input_cleanup(uuid,bigint,bytea,integer,text,text)')
	 ])::oid
	), recovery_allowed AS (
	 SELECT unnest(ARRAY[
	  pg_catalog.to_regprocedure('live.begin_media_recovery_episode(uuid,bigint,integer,boolean)'),
	  pg_catalog.to_regprocedure('live.claim_recovery_observation(uuid,uuid,bigint,bytea)'),
	  pg_catalog.to_regprocedure('live.record_recovery_observation(uuid,uuid,bigint,bytea,text,text,text,text,bigint,bigint,bigint)'),
	  pg_catalog.to_regprocedure('live.finish_recovery_observation(uuid,uuid,bigint,bytea,text)'),
	  pg_catalog.to_regprocedure('live.read_media_recovery_episode(uuid)'),
	  pg_catalog.to_regprocedure('live.witness_media_recovery_episode(uuid,uuid,uuid,bigint)'),
	  pg_catalog.to_regprocedure('live.timeout_media_recovery_episode(uuid,bigint)'),
	  pg_catalog.to_regprocedure('live.media_recovery_ready()'),
	  pg_catalog.to_regprocedure('live.begin_media_recovery_episode_with_input(uuid,bigint,integer,boolean)'),
	  pg_catalog.to_regprocedure('live.claim_media_recovery_observation_with_input(uuid,uuid,bigint,bytea)'),
	  pg_catalog.to_regprocedure('live.record_browser_input_recovery_observation(uuid,uuid,bigint,bytea,text,text,text,text)'),
	  pg_catalog.to_regprocedure('live.record_media_recovery_observation_with_input(uuid,uuid,bigint,bytea,text,text,text,text,bigint,bigint,bigint)'),
	  pg_catalog.to_regprocedure('live.finish_media_recovery_observation_with_input(uuid,uuid,bigint,bytea,text)'),
	  pg_catalog.to_regprocedure('live.read_media_recovery_episode_with_input(uuid)'),
	  pg_catalog.to_regprocedure('live.witness_media_recovery_episode_with_input(uuid,uuid,uuid,uuid,bigint)'),
	  pg_catalog.to_regprocedure('live.media_browser_input_recovery_ready()')
	 ])::oid AS oid
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
	 OR ($1='media_recovery' AND (SELECT count(*) FROM recovery_allowed a
	  WHERE pg_catalog.has_function_privilege(session_user,a.oid,'EXECUTE'))<>16)
	 OR EXISTS (SELECT 1 FROM reachable r CROSS JOIN pg_catalog.pg_proc p
	 JOIN pg_catalog.pg_namespace n ON n.oid=p.pronamespace
	 WHERE n.nspname<>'information_schema' AND n.nspname NOT LIKE 'pg\_%' ESCAPE '\'
	  AND pg_catalog.has_schema_privilege(r.oid,n.oid,'USAGE')
	  AND p.prorettype<>'trigger'::regtype
	  AND NOT ($1='media_worker' AND (n.nspname='river_media'
	   OR (n.nspname='live' AND p.proname IN ('media_worker_ready','media_input_plan_ready','media_browser_input_worker_ready')
	    AND p.pronargs=0)))
	  -- A missing optional ABI resolves to NULL. Exclude it so SQL's three-valued
	  -- IN/NOT logic cannot hide an unrelated executable function from this guard.
	  AND NOT ($1='media_executor' AND p.oid IN (SELECT oid FROM allowed WHERE oid IS NOT NULL))
	  AND NOT ($1='media_recovery' AND p.oid IN (SELECT oid FROM recovery_allowed WHERE oid IS NOT NULL))
	  AND pg_catalog.has_function_privilege(r.oid,p.oid,'EXECUTE'))`, role).Scan(&forbidden)
	if err != nil || forbidden {
		return errMediaDatabase
	}
	return nil
}
